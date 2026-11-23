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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cnpgGroup is CloudNativePG's API group; cnpgClusterRef, cnpgEnsure and
// cnpgPresent are the spec vocabulary its declarative objects share.
const (
	cnpgGroup      = "postgresql.cnpg.io"
	cnpgClusterRef = "cluster"
	cnpgEnsure     = "ensure"
	cnpgPresent    = "present"
)

// cnpgDatabaseGVK is CloudNativePG's declarative database (postgresql.cnpg.io
// Database), distinct from bex's own app.bex.co Database parent.
var cnpgDatabaseGVK = schema.GroupVersionKind{Group: cnpgGroup, Version: "v1", Kind: "Database"}

// conditionExtensionsReady reports whether the application database's managed
// extensions are installed. Only a database the operator still preinstalls
// vector into carries it (reconcileApplicationDatabase).
const conditionExtensionsReady = "ExtensionsReady"

const (
	// reasonReservedDatabaseName marks the PostgreSQL-owned database CNPG
	// cannot manage, which keeps its legacy no-extensions treatment.
	reasonReservedDatabaseName = "ReservedDatabaseName"
	// reasonInitializingExtensions keeps a new database creating while its
	// managed extensions install (w4/m223).
	reasonInitializingExtensions = "InitializingExtensions"

	// ExtensionsReady reasons from CNPG's verdict on the child.
	reasonExtensionsApplied     = "Applied"
	reasonExtensionsApplyFailed = "ApplyFailed"
	reasonExtensionsApplyError  = "ApplyError"
	reasonExtensionsPending     = "Pending"
)

// applicationDatabaseName names the CNPG Database child for db's application
// database.
func applicationDatabaseName(db *appv1alpha1.Database) string { return db.Name + "-app" }

// applicationDatabaseSpec declares the application database with pgvector
// installed (w8/m84), for a cluster whose image has no allowlist that admits
// vector. The CNPG images package vector, but CREATE EXTENSION needs a
// superuser, which the application owner is not; installed by CNPG, a
// migration's `CREATE EXTENSION IF NOT EXISTS vector` is a no-op and the type,
// operators and indexes work for the owner. dbname and owner must be the
// database's own (create and restore record them on the spec), since CNPG
// creates a missing database; retain keeps the database and its data if this
// child is deleted, and no pinned version means CNPG never alters an installed
// extension.
func applicationDatabaseSpec(clusterName, dbname, owner string) map[string]any {
	return map[string]any{
		cnpgClusterRef:          map[string]any{"name": clusterName},
		"name":                  dbname,
		"owner":                 owner,
		cnpgEnsure:              cnpgPresent,
		"databaseReclaimPolicy": "retain",
		"extensions":            []any{map[string]any{"name": "vector", cnpgEnsure: cnpgPresent}},
	}
}

// reconcileApplicationDatabase preinstalls vector only where the owner cannot
// create it. On a cluster whose allowlist admits vector, the owner runs
// Render's `CREATE EXTENSION vector;` on demand (w21/m586), so the operator
// installs nothing and releases an existing database's child. CloudNativePG
// 1.30.1 drops neither the database (retain) nor an extension when a child is
// deleted, so a vector it installed stays, with its rows and indexes, for the
// owner to keep or drop through the same allowlist.
//
// Anywhere else (an upstream image: the adoption's rollback, or an
// unversioned cluster whose major is not yet observed) it applies db's CNPG
// Database child and projects CNPG's verdict onto the ExtensionsReady
// condition. Once a database has served, the child is optional to serving and
// a problem with it degrades only that condition; before that it gates the
// first Ready (w4/m223). A suspended database keeps its last verdict: its
// cluster is hibernated, so CNPG cannot reach it.
func (r *DatabaseReconciler) reconcileApplicationDatabase(ctx context.Context, db *appv1alpha1.Database, cluster *unstructured.Unstructured, dbname, owner string) {
	if allowlistAdmits(cluster, "vector") {
		name := applicationDatabaseName(db)
		if err := deleteOwned(ctx, r.Client, db, cnpgDatabaseGVK, name); err != nil {
			logf.FromContext(ctx).Error(err, "release the application database", "database", name)
		}
		meta.RemoveStatusCondition(&db.Status.Conditions, conditionExtensionsReady)
		return
	}
	if appv1alpha1.ReservedPostgresDatabaseName(dbname) {
		// CNPG refuses to manage a PostgreSQL-owned database, and a refused
		// child would fail every reconcile (as a reserved role did, w4/m170).
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type: conditionExtensionsReady, Status: metav1.ConditionFalse, Reason: reasonReservedDatabaseName, ObservedGeneration: db.Generation,
			Message: fmt.Sprintf("extensions are not managed in the PostgreSQL-owned database %q", dbname),
		})
		return
	}
	child := &unstructured.Unstructured{}
	child.SetGroupVersionKind(cnpgDatabaseGVK)
	child.SetName(applicationDatabaseName(db))
	child.SetNamespace(db.Namespace)
	// The live object the write leaves behind carries the verdict, so no
	// second read.
	err := upsertOwnedObject(ctx, r.Client, r.Scheme, db, child, applicationDatabaseSpec(db.Name, dbname, owner))
	if err != nil {
		logf.FromContext(ctx).Error(err, "apply the application database", "database", child.GetName())
	}
	if db.Spec.Suspended {
		return
	}
	meta.SetStatusCondition(&db.Status.Conditions, extensionsVerdict.condition(child, db.Generation, err))
}

// allowlistAdmits reports whether the extension allowlist projected onto
// cluster (pgExtensionParameters) lets the database owner create extension.
func allowlistAdmits(cluster *unstructured.Unstructured, extension string) bool {
	list, _, _ := unstructured.NestedString(cluster.Object, "spec", "postgresql", "parameters", "extwlist.extensions")
	return slices.Contains(strings.Split(list, ","), extension)
}

// cnpgChildVerdict is how a condition reads CloudNativePG's verdict on one of
// its declarative children (the application database, the all-tables
// publication): its type, its reasons and its fixed messages.
type cnpgChildVerdict struct {
	conditionType                                string
	applied, pending, applyFailed, applyError    string
	appliedMessage, pendingMessage, retryMessage string
}

var extensionsVerdict = cnpgChildVerdict{
	conditionType: conditionExtensionsReady,
	applied:       reasonExtensionsApplied, pending: reasonExtensionsPending,
	applyFailed: reasonExtensionsApplyFailed, applyError: reasonExtensionsApplyError,
	appliedMessage: "vector is installed in the application database",
	pendingMessage: "waiting for CloudNativePG to apply the application database",
	retryMessage:   "retrying the application database",
}

// condition reads the verdict on child after a write that returned err. A
// conflict or a busy API server is retried on the next pass, not reported as
// a failure that would mark a new database unavailable. CNPG advances
// observedGeneration only on success, so a failure (applied false with a
// message) is reported whatever generation it names; a success counts only
// for the current one.
func (v cnpgChildVerdict) condition(child *unstructured.Unstructured, generation int64, err error) metav1.Condition {
	condition := metav1.Condition{Type: v.conditionType, ObservedGeneration: generation}
	applied, found, _ := unstructured.NestedBool(child.Object, "status", "applied")
	observed, _, _ := unstructured.NestedInt64(child.Object, "status", "observedGeneration")
	message, _, _ := unstructured.NestedString(child.Object, "status", "message")
	switch {
	case err != nil && transientAPIError(err):
		condition.Status, condition.Reason, condition.Message = metav1.ConditionUnknown, v.pending, v.retryMessage
	case err != nil:
		condition.Status, condition.Reason, condition.Message = metav1.ConditionFalse, v.applyError, err.Error()
	case found && !applied && message != "":
		condition.Status, condition.Reason, condition.Message = metav1.ConditionFalse, v.applyFailed, message
	case found && applied && observed == child.GetGeneration():
		condition.Status, condition.Reason, condition.Message = metav1.ConditionTrue, v.applied, v.appliedMessage
	default:
		condition.Status, condition.Reason, condition.Message = metav1.ConditionUnknown, v.pending, v.pendingMessage
	}
	return condition
}

// transientAPIError is a write refusal the API server expects a retry to
// clear.
func transientAPIError(err error) bool {
	return apierrors.IsConflict(err) || apierrors.IsServerTimeout(err) || apierrors.IsTimeout(err) ||
		apierrors.IsTooManyRequests(err) || apierrors.IsServiceUnavailable(err)
}
