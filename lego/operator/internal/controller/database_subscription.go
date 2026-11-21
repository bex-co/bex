/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"cmp"
	"context"
	"slices"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Logical replication as a subscriber (w34/m542; docs/ADR009 § Logical
// replication › Subscribers). Render enables subscribers on request like
// publishers, and the user then creates "a subscription on each subscriber"
// (render.com/docs/postgresql-logical-replication, read 2026-11-20). On
// PostgreSQL 16 and later that takes membership in pg_create_subscription,
// which this file gives the subscriber role. On 13-15 only a superuser can,
// so the operator declares each requested subscription as a CloudNativePG
// Subscription, which CNPG creates as the superuser, as Render's support can.

// pgCreateSubscriptionRole is PostgreSQL 16's predefined role whose members
// may CREATE SUBSCRIPTION (with CREATE on the database).
const pgCreateSubscriptionRole = "pg_create_subscription"

// labelLogicalSubscriptionOf marks the CNPG Subscriptions a Database
// declares, so the reconcile lists only those.
const labelLogicalSubscriptionOf = "app.bex.co/logical-subscription-of"

// cnpgSubscriptionGVK is CloudNativePG's declarative subscription.
var cnpgSubscriptionGVK = schema.GroupVersionKind{Group: cnpgGroup, Version: "v1", Kind: "Subscription"}

// hasPgCreateSubscription reports whether the cluster runs a PostgreSQL that
// has pg_create_subscription. The running major counts first: a cluster
// still upgrading from 15 does not have the role yet, and CNPG would fail
// every role while one names it.
func hasPgCreateSubscription(p clusterParams) bool {
	major, err := strconv.Atoi(cmp.Or(p.runningVersion, effectivePostgresVersion(p.version, "", p.creating)))
	return err == nil && major >= appv1alpha1.PgCreateSubscriptionMajor
}

// markSubscriberRole adds pg_create_subscription to the subscriber role's
// projected memberships. CNPG revokes a membership inRoles stops listing, so
// removing the field or naming another role takes it back off.
func markSubscriberRole(roles []any, subscriber string) {
	withActiveLogin(roles, subscriber, func(role map[string]any) {
		inRoles, _ := role["inRoles"].([]any)
		role["inRoles"] = append(slices.Clone(inRoles), pgCreateSubscriptionRole)
	})
}

// subscriptionExternalClusters are the externalClusters entries the 13-15
// Subscriptions connect through, one per subscription, named like its
// Subscription. CNPG's instance manager writes each password to a passfile
// the subscription's connection string names, so no password reaches the
// Cluster spec or pg_subscription.
func subscriptionExternalClusters(lr *appv1alpha1.DatabaseLogicalReplication, db string) []any {
	if lr == nil {
		return nil
	}
	entries := make([]any, 0, len(lr.Subscriptions))
	for _, sub := range lr.Subscriptions {
		parameters := libpqParameters(sub.Host, sub.User, sub.DBName, cmp.Or(sub.SSLMode, appv1alpha1.LogicalSubscriptionDefaultSSLMode))
		parameters["port"] = strconv.Itoa(int(cmp.Or(sub.Port, pgPort)))
		entries = append(entries, map[string]any{
			"name":                 appv1alpha1.LogicalSubscriptionObjectName(db, sub.Name),
			"connectionParameters": parameters,
			"password":             map[string]any{"name": sub.PasswordSecret, "key": "password"},
		})
	}
	return entries
}

// subscriptionSpec declares sub in dbname. Reclaim policy delete drops it
// when the child goes, which also drops its slot on the publisher.
func subscriptionSpec(db, dbname string, sub appv1alpha1.DatabaseLogicalSubscription) map[string]any {
	return map[string]any{
		cnpgClusterRef:              map[string]any{"name": db},
		"dbname":                    dbname,
		"name":                      sub.Name,
		"externalClusterName":       appv1alpha1.LogicalSubscriptionObjectName(db, sub.Name),
		"publicationName":           sub.Publication,
		"subscriptionReclaimPolicy": "delete",
	}
}

const (
	reasonLogicalSubscriptionApplied     = "SubscriptionApplied"
	reasonLogicalSubscriptionPending     = "SubscriptionPending"
	reasonLogicalSubscriptionApplyFailed = "SubscriptionApplyFailed"
	reasonLogicalSubscriptionApplyError  = "SubscriptionApplyError"
	reasonLogicalSubscriptionRemoveError = "SubscriptionRemoveError"
)

var logicalSubscriptionVerdict = cnpgChildVerdict{
	conditionType: appv1alpha1.ConditionLogicalReplication,
	applied:       reasonLogicalSubscriptionApplied, pending: reasonLogicalSubscriptionPending,
	applyFailed: reasonLogicalSubscriptionApplyFailed, applyError: reasonLogicalSubscriptionApplyError,
	appliedMessage: "is applied",
	pendingMessage: "is waiting for CloudNativePG",
	retryMessage:   "is being retried",
}

// reconcileLogicalSubscriptions converges db's Subscriptions: each requested
// one is declared, any other it controls is deleted. It returns their
// verdict (nil: none requested and none left) and whether CNPG was asked
// for one.
func (r *DatabaseReconciler) reconcileLogicalSubscriptions(ctx context.Context, db *appv1alpha1.Database, dbname string) (*metav1.Condition, bool) {
	var desired []appv1alpha1.DatabaseLogicalSubscription
	if lr := db.Spec.LogicalReplication; lr != nil {
		desired = lr.Subscriptions
	}
	var verdicts []*metav1.Condition
	owned, err := r.listLogicalSubscriptions(ctx, db)
	if err != nil {
		logf.FromContext(ctx).Error(err, "list the subscriptions")
		verdicts = append(verdicts, &metav1.Condition{Status: metav1.ConditionFalse, Reason: reasonLogicalSubscriptionRemoveError, Message: err.Error()})
	}
	for i := range owned {
		item := &owned[i]
		wanted := slices.ContainsFunc(desired, func(sub appv1alpha1.DatabaseLogicalSubscription) bool {
			return appv1alpha1.LogicalSubscriptionObjectName(db.Name, sub.Name) == item.GetName()
		})
		if wanted || item.GetDeletionTimestamp() != nil {
			continue
		}
		if err := deleteTolerantly(ctx, r.Client, item); err != nil {
			logf.FromContext(ctx).Error(err, "remove a subscription", "subscription", item.GetName())
			verdicts = append(verdicts, &metav1.Condition{Status: metav1.ConditionFalse, Reason: reasonLogicalSubscriptionRemoveError, Message: err.Error()})
		}
	}
	if len(desired) > 0 && appv1alpha1.ReservedPostgresDatabaseName(dbname) {
		return &metav1.Condition{Status: metav1.ConditionFalse, Reason: reasonReservedDatabaseName,
			Message: "subscriptions are not managed in the PostgreSQL-owned database " + strconv.Quote(dbname)}, false
	}
	for _, sub := range desired {
		child := &unstructured.Unstructured{}
		child.SetGroupVersionKind(cnpgSubscriptionGVK)
		child.SetName(appv1alpha1.LogicalSubscriptionObjectName(db.Name, sub.Name))
		child.SetNamespace(db.Namespace)
		_, err := controllerutil.CreateOrUpdate(ctx, r.Client, child, func() error {
			labels := child.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels[labelLogicalSubscriptionOf] = db.Name
			child.SetLabels(labels)
			if err := projectUnstructuredSpec(child, subscriptionSpec(db.Name, dbname, sub)); err != nil {
				return err
			}
			return controllerutil.SetControllerReference(db, child, r.Scheme)
		})
		if err != nil {
			logf.FromContext(ctx).Error(err, "apply a subscription", "subscription", child.GetName())
		}
		// The verdict's own messages are phrases about the subscription; a
		// failure is CNPG's or the API server's message, after a colon.
		verdict := logicalSubscriptionVerdict.condition(child, db.Generation, err)
		separator := " "
		if verdict.Status == metav1.ConditionFalse {
			separator = ": "
		}
		verdict.Message = "subscription " + sub.Name + separator + verdict.Message
		verdicts = append(verdicts, &verdict)
	}
	return worstCondition(verdicts...), len(desired) > 0
}

// listLogicalSubscriptions lists the CNPG Subscriptions db controls. A
// cluster without CNPG's Subscription CRD has none.
func (r *DatabaseReconciler) listLogicalSubscriptions(ctx context.Context, db *appv1alpha1.Database) ([]unstructured.Unstructured, error) {
	return listControlledObjects(ctx, r.Client, db, cnpgSubscriptionGVK, client.MatchingLabels{labelLogicalSubscriptionOf: db.Name})
}
