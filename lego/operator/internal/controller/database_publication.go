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
	"context"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Logical replication as a publisher (w34/m541; docs/ADR009 § Logical
// replication). Render enables it through its support team on request
// (render.com/docs/postgresql-logical-replication, read 2026-11-20); on bex an
// operator runs bex-api's internal verb, which writes
// Database.spec.logicalReplication. This file projects that intent: the
// publisher role's REPLICATION attribute, the retained-WAL bound, and the
// `FOR ALL TABLES` publication Render's support creates, since only a
// superuser can.

// cnpgPublicationGVK is CloudNativePG's declarative publication.
var cnpgPublicationGVK = schema.GroupVersionKind{Group: cnpgGroup, Version: "v1", Kind: "Publication"}

// paramMaxSlotWALKeepSize bounds the WAL one replication slot may hold back.
// PostgreSQL's default, which CNPG keeps, is -1 (unbounded): an abandoned
// subscriber would keep every WAL segment until the volume fills, or until
// disk autoscaling grows it.
const paramMaxSlotWALKeepSize = "max_slot_wal_keep_size"

// slotWALKeepPercent is the share of the database's storage one slot may
// retain: a fifth, so retained WAL alone never reaches the 90% disk
// autoscaling trigger (databaseDiskAutoscaleThreshold). At Render's 10 GB
// floor that is 2048 MB. bex's choice until w34/001 captures Render's.
const slotWALKeepPercent = 20

// slotWALKeepSize is max_slot_wal_keep_size for a database of storageGB, in
// megabytes: it follows the disk as it grows.
func slotWALKeepSize(storageGB int32) string {
	return fmt.Sprintf("%dMB", int64(storageGB)*1024*slotWALKeepPercent/100)
}

// markPublisherRole gives the publisher role CNPG's `replication: true` in the
// projected managed roles. Every other role keeps PostgreSQL's
// NOREPLICATION, so removing the field or naming another role takes the
// attribute back off.
func markPublisherRole(roles []any, publisher string) {
	withActiveLogin(roles, publisher, func(role map[string]any) { role["replication"] = true })
}

// withActiveLogin applies mark to the projected managed role name, when it
// is there and an active login: a retired one (login false) never
// replicates or subscribes, and a role managedRoles left out (reserved) is
// not there to mark.
func withActiveLogin(roles []any, name string, mark func(map[string]any)) {
	for _, item := range roles {
		role, ok := item.(map[string]any)
		if ok && role["name"] == name && role["login"] == true {
			mark(role)
		}
	}
}

// allTablesPublicationName names the CNPG Publication child of db.
func allTablesPublicationName(db *appv1alpha1.Database) string { return db.Name + "-all-tables" }

// allTablesPublicationSpec declares appv1alpha1.LogicalReplicationAllTablesPublication
// in dbname, created by CNPG as the superuser. Reclaim policy delete drops it
// when the child goes: disabling the publisher removes the publication.
func allTablesPublicationSpec(clusterName, dbname string) map[string]any {
	return map[string]any{
		cnpgClusterRef:             map[string]any{"name": clusterName},
		"dbname":                   dbname,
		"name":                     appv1alpha1.LogicalReplicationAllTablesPublication,
		"target":                   map[string]any{"allTables": true},
		"publicationReclaimPolicy": "delete",
	}
}

// ConditionLogicalReplication reasons.
const (
	reasonLogicalReplicationEnabled     = "Enabled"
	reasonLogicalReplicationApplied     = "PublicationApplied"
	reasonLogicalReplicationPending     = "PublicationPending"
	reasonLogicalReplicationApplyFailed = "PublicationApplyFailed"
	reasonLogicalReplicationApplyError  = "PublicationApplyError"
	reasonLogicalReplicationRemoveError = "PublicationRemoveError"
)

var allTablesPublicationVerdict = cnpgChildVerdict{
	conditionType: appv1alpha1.ConditionLogicalReplication,
	applied:       reasonLogicalReplicationApplied, pending: reasonLogicalReplicationPending,
	applyFailed: reasonLogicalReplicationApplyFailed, applyError: reasonLogicalReplicationApplyError,
	appliedMessage: "publication " + appv1alpha1.LogicalReplicationAllTablesPublication + " is applied",
	pendingMessage: "waiting for CloudNativePG to apply the all-tables publication",
	retryMessage:   "retrying the all-tables publication",
}

// reconcileLogicalReplication converges db's CloudNativePG children, the
// all-tables publication and the 13-15 Subscriptions, and reports them on
// ConditionLogicalReplication. The condition's presence is also the memory
// that a child may exist, so a database that was never enabled costs no read
// here. A problem never fails the reconcile: it degrades only the condition,
// as the application database's does.
func (r *DatabaseReconciler) reconcileLogicalReplication(ctx context.Context, db *appv1alpha1.Database, dbname string) {
	lr := db.Spec.LogicalReplication
	present := meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionLogicalReplication) != nil
	if lr == nil && !present {
		return
	}
	publication, publicationAsked := r.reconcileAllTablesPublication(ctx, db, dbname)
	subscriptions, subscriptionsAsked := r.reconcileLogicalSubscriptions(ctx, db, dbname)
	if db.Spec.Suspended && present && (publicationAsked || subscriptionsAsked) {
		// Hibernated: CNPG cannot reach the instance, so keep the verdict.
		return
	}
	condition := worstCondition(publication, subscriptions)
	switch {
	case condition != nil:
	case lr == nil:
		meta.RemoveStatusCondition(&db.Status.Conditions, appv1alpha1.ConditionLogicalReplication)
		return
	default:
		condition = &metav1.Condition{Status: metav1.ConditionTrue, Reason: reasonLogicalReplicationEnabled, Message: enabledMessage(lr)}
	}
	condition.Type, condition.ObservedGeneration = appv1alpha1.ConditionLogicalReplication, db.Generation
	meta.SetStatusCondition(&db.Status.Conditions, *condition)
}

// enabledMessage describes an enabled database with no CloudNativePG child.
func enabledMessage(lr *appv1alpha1.DatabaseLogicalReplication) string {
	var parts []string
	if lr.Publishes() {
		parts = append(parts, lr.PublisherRole+" may start a walsender; no all-tables publication was requested")
	}
	if lr.SubscriberRole != "" {
		parts = append(parts, lr.SubscriberRole+" may create subscriptions on PostgreSQL 16 and later")
	}
	return strings.Join(parts, "; ")
}

// reconcileAllTablesPublication converges the all-tables publication and
// returns its verdict (nil: none requested and none left), and whether CNPG
// was asked for one.
func (r *DatabaseReconciler) reconcileAllTablesPublication(ctx context.Context, db *appv1alpha1.Database, dbname string) (*metav1.Condition, bool) {
	lr := db.Spec.LogicalReplication
	child := &unstructured.Unstructured{}
	child.SetGroupVersionKind(cnpgPublicationGVK)
	child.SetName(allTablesPublicationName(db))
	child.SetNamespace(db.Namespace)
	switch {
	case !lr.Publishes() || !lr.AllTables:
		if err := deleteOptionalObject(ctx, r.Client, child); err != nil {
			logf.FromContext(ctx).Error(err, "remove the all-tables publication", "publication", child.GetName())
			return &metav1.Condition{Status: metav1.ConditionFalse, Reason: reasonLogicalReplicationRemoveError, Message: err.Error()}, false
		}
		return nil, false
	case appv1alpha1.ReservedPostgresDatabaseName(dbname):
		// CNPG refuses to manage a PostgreSQL-owned database.
		return &metav1.Condition{Status: metav1.ConditionFalse, Reason: reasonReservedDatabaseName,
			Message: fmt.Sprintf("the all-tables publication is not managed in the PostgreSQL-owned database %q", dbname)}, false
	}
	err := upsertOwnedObject(ctx, r.Client, r.Scheme, db, child, allTablesPublicationSpec(db.Name, dbname))
	if err != nil {
		logf.FromContext(ctx).Error(err, "apply the all-tables publication", "publication", child.GetName())
	}
	condition := allTablesPublicationVerdict.condition(child, db.Generation, err)
	return &condition, true
}

// worstCondition folds verdicts into one: the first False, else the first
// Unknown, else True with every message. nil when there is none.
func worstCondition(verdicts ...*metav1.Condition) *metav1.Condition {
	verdicts = slices.DeleteFunc(verdicts, func(c *metav1.Condition) bool { return c == nil })
	for _, status := range []metav1.ConditionStatus{metav1.ConditionFalse, metav1.ConditionUnknown} {
		if i := slices.IndexFunc(verdicts, func(c *metav1.Condition) bool { return c.Status == status }); i >= 0 {
			return verdicts[i]
		}
	}
	if len(verdicts) == 0 {
		return nil
	}
	messages := make([]string, len(verdicts))
	for i, c := range verdicts {
		messages[i] = c.Message
	}
	folded := *verdicts[0]
	folded.Message = strings.Join(messages, "; ")
	return &folded
}
