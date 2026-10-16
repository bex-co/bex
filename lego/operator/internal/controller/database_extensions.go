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
// extensions are installed.
const conditionExtensionsReady = "ExtensionsReady"

// applicationDatabaseName names the CNPG Database child for db's application
// database.
func applicationDatabaseName(db *appv1alpha1.Database) string { return db.Name + "-app" }

// applicationDatabaseSpec declares the application database with pgvector
// installed (w8/m84). The CNPG images package vector, but CREATE EXTENSION
// needs a superuser, which the application owner is not; installed by CNPG, a
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

// reconcileApplicationDatabase applies db's CNPG Database child and projects
// CNPG's verdict onto the ExtensionsReady condition. The child is optional to
// serving, so a problem with it degrades only that condition. A suspended
// database keeps its last verdict: its cluster is hibernated, so CNPG cannot
// reach it.
func (r *DatabaseReconciler) reconcileApplicationDatabase(ctx context.Context, db *appv1alpha1.Database, dbname, owner string) {
	if appv1alpha1.ReservedPostgresDatabaseName(dbname) {
		// CNPG refuses to manage a PostgreSQL-owned database, and a refused
		// child would fail every reconcile (as a reserved role did, w4/m170).
		meta.SetStatusCondition(&db.Status.Conditions, metav1.Condition{
			Type: conditionExtensionsReady, Status: metav1.ConditionFalse, Reason: "ReservedDatabaseName", ObservedGeneration: db.Generation,
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
	condition := extensionsCondition(child, db.Generation)
	if err != nil {
		condition.Status, condition.Reason, condition.Message = metav1.ConditionFalse, "ApplyError", err.Error()
	}
	meta.SetStatusCondition(&db.Status.Conditions, condition)
}

// extensionsCondition reads CNPG's verdict. CNPG advances observedGeneration
// only on success, so a failure (applied false with a message) is reported
// whatever generation it names; a success counts only for the current one.
func extensionsCondition(child *unstructured.Unstructured, generation int64) metav1.Condition {
	condition := metav1.Condition{Type: conditionExtensionsReady, ObservedGeneration: generation}
	applied, found, _ := unstructured.NestedBool(child.Object, "status", "applied")
	observed, _, _ := unstructured.NestedInt64(child.Object, "status", "observedGeneration")
	message, _, _ := unstructured.NestedString(child.Object, "status", "message")
	switch {
	case found && !applied && message != "":
		condition.Status, condition.Reason, condition.Message = metav1.ConditionFalse, "ApplyFailed", message
	case found && applied && observed == child.GetGeneration():
		condition.Status, condition.Reason, condition.Message = metav1.ConditionTrue, "Applied", "vector is installed in the application database"
	default:
		condition.Status, condition.Reason, condition.Message = metav1.ConditionUnknown, "Pending", "waiting for CloudNativePG to apply the application database"
	}
	return condition
}
