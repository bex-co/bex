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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// recoveredCopy is a copy as bex-api's Recover writes it: the source's
// database and owner, and the source's logical-replication credential
// retired (w34/m543 t005).
func recoveredCopy() (*appv1alpha1.Database, clusterParams) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-copy", Namespace: "default", UID: "uid-dpg-copy"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "copy", Plan: "basic-256mb", DatabaseName: "dpg_source", DatabaseUser: "dpg_source_user",
			DeletedUsers: []string{"cdc_reader"},
			Recovery:     &appv1alpha1.DatabaseRecovery{SourceDatabase: "dpg-source", TargetTime: "2026-11-21T16:16:00Z"},
		},
	}
	plan, gb := resolvePlan(db.Spec)
	return db, clusterParams{
		name: db.Name, namespace: db.Namespace, plan: plan, storageGB: gb, store: &testStore,
		dbname:   db.Spec.EffectiveDatabaseName(db.Name),
		owner:    db.Spec.EffectiveDatabaseUser(db.Name),
		recovery: db.Spec.Recovery, deletedUsers: db.Spec.DeletedUsers,
	}
}

// reconcileRecoveredCluster runs one Cluster write and returns the stored
// spec and its bootstrap.recovery.
func reconcileRecoveredCluster(t *testing.T, r *DatabaseReconciler, db *appv1alpha1.Database, p clusterParams) (spec, recovery map[string]any) {
	t.Helper()
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cluster.SetName(db.Name)
	cluster.SetNamespace(db.Namespace)
	if _, err := r.reconcileCluster(context.Background(), db, cluster, p); err != nil {
		t.Fatal(err)
	}
	live := &unstructured.Unstructured{}
	live.SetGroupVersionKind(cnpgClusterGVK)
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(db), live); err != nil {
		t.Fatal(err)
	}
	spec, _, _ = unstructured.NestedMap(live.Object, "spec")
	recovery, _, _ = unstructured.NestedMap(spec, "bootstrap", "recovery")
	return spec, recovery
}

// A recovered copy bootstraps with the source's database and owner, the pair
// its view names, so CNPG writes the "-app" Secret connection info reads for
// the restored data (w34/007). Without them CNPG defaults both to `app` and
// creates an empty database the connection strings then opened. The copy's
// retirements still reach its managed roles.
func TestARecoveredCopyBootstrapsTheRestoredApplicationDatabase(t *testing.T) {
	db, p := recoveredCopy()
	scheme := publicationScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}

	var spec map[string]any
	for pass := 1; pass <= 2; pass++ {
		var recovery map[string]any
		spec, recovery = reconcileRecoveredCluster(t, r, db, p)
		if recovery["database"] != "dpg_source" || recovery["owner"] != "dpg_source_user" || recovery["source"] != recoverySource {
			t.Fatalf("pass %d: bootstrap.recovery = %v, want the source's database and owner", pass, recovery)
		}
	}
	roles := managedRoleIndex(t, spec)
	if owner := roles["dpg_source_user"]; owner["login"] != true {
		t.Errorf("owner role = %v, want a login", owner)
	}
	if retired := roles["cdc_reader"]; retired["login"] != false || retired["disablePassword"] != true {
		t.Errorf("retired credential = %v, want NOLOGIN with no password", retired)
	}
}

// A Cluster CNPG already restored on another application database keeps it:
// CNPG never rewrites its "-app" Secret, and its instance manager refuses one
// naming a different owner, so changing the pair would fail every password
// refresh on the copy.
func TestARestoredClusterKeepsTheApplicationDatabaseCNPGAccepted(t *testing.T) {
	db, p := recoveredCopy()
	existing := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
		"metadata": map[string]any{"name": db.Name, "namespace": db.Namespace},
		"spec": map[string]any{"bootstrap": map[string]any{"recovery": map[string]any{
			"source": recoverySource, "database": "app", "owner": "app",
		}}},
	}}
	existing.SetGroupVersionKind(cnpgClusterGVK)
	scheme := publicationScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, existing).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}

	_, recovery := reconcileRecoveredCluster(t, r, db, p)
	if recovery["database"] != "app" || recovery["owner"] != "app" {
		t.Fatalf("bootstrap.recovery = %v, want the accepted app/app kept", recovery)
	}
}
