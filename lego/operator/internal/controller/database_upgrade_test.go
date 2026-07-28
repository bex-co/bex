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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestCNPGMajorUpgradeState(t *testing.T) {
	tests := []struct {
		phase       string
		wantRunning bool
		wantFailed  bool
	}{
		{phase: "Cluster in healthy state"},
		{phase: "Upgrading Postgres major version", wantRunning: true},
		{phase: "Failed to upgrade Postgres major version", wantFailed: true},
	}
	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			state := cnpgClusterState{phase: tt.phase}
			if got := state.majorUpgradeRunning(); got != tt.wantRunning {
				t.Errorf("majorUpgradeRunning() = %v, want %v", got, tt.wantRunning)
			}
			if got := state.majorUpgradeFailed(); got != tt.wantFailed {
				t.Errorf("majorUpgradeFailed() = %v, want %v", got, tt.wantFailed)
			}
		})
	}
}

func TestCNPGConditionMessagePreservesUpgradeFailure(t *testing.T) {
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"conditions": []any{map[string]any{
				"type": "Ready", "status": "False",
				"message": "pg_upgrade --check found an incompatible extension",
			}},
		},
	}}
	if got := cnpgConditionMessage(cluster, "fallback"); got != "pg_upgrade --check found an incompatible extension" {
		t.Errorf("condition message = %q", got)
	}
}

func newMajorUpgradeReconciler(t *testing.T, phase string) (*DatabaseReconciler, client.Client, reconcile.Request) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	for _, gvk := range []struct {
		group, version, kind string
	}{
		{cnpgClusterGVK.Group, cnpgClusterGVK.Version, cnpgClusterGVK.Kind},
		{cnpgBackupGVK.Group, cnpgBackupGVK.Version, cnpgBackupGVK.Kind},
		{cnpgScheduledBackupGVK.Group, cnpgScheduledBackupGVK.Version, cnpgScheduledBackupGVK.Kind},
		{cnpgPoolerGVK.Group, cnpgPoolerGVK.Version, cnpgPoolerGVK.Kind},
		{traefikIngressRouteTCPGVK.Group, traefikIngressRouteTCPGVK.Version, traefikIngressRouteTCPGVK.Kind},
		{traefikMiddlewareTCPGVK.Group, traefikMiddlewareTCPGVK.Version, traefikMiddlewareTCPGVK.Kind},
	} {
		scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: gvk.group, Version: gvk.version, Kind: gvk.kind}, &unstructured.Unstructured{})
	}

	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "upgrade-db", Namespace: "default", Finalizers: []string{dbFinalizer}},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "basic-1gb", Version: "17"},
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, CurrentVersion: "16", BackupsEnabled: true,
			BackupServerName: "upgrade-db",
		},
	}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]any{
			"name": "upgrade-db", "namespace": "default",
		},
		"spec": map[string]any{"imageName": "ghcr.io/cloudnative-pg/postgresql:16"},
		"status": map[string]any{
			"phase": phase, "readyInstances": int64(0),
			"pgDataImageInfo": map[string]any{"majorVersion": int64(16)},
		},
	}}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, cluster).
		WithStatusSubresource(&appv1alpha1.Database{}).Build()
	return &DatabaseReconciler{Client: cl, Scheme: scheme, Backup: testStore}, cl, reconcile.Request{
		NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace},
	}
}

func TestReconcileMajorVersionUpgradeLifecycle(t *testing.T) {
	ctx := context.Background()
	r, cl, req := newMajorUpgradeReconciler(t, "Upgrading Postgres major version")

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	if err := cl.Get(ctx, req.NamespacedName, cluster); err != nil {
		t.Fatal(err)
	}
	image, _, _ := unstructured.NestedString(cluster.Object, "spec", "imageName")
	if image != "ghcr.io/cloudnative-pg/postgresql:17" {
		t.Errorf("cluster image = %q, want PostgreSQL 17", image)
	}
	plugins, _, _ := unstructured.NestedSlice(cluster.Object, "spec", "plugins")
	if len(plugins) != 1 {
		t.Fatalf("upgrade plugins = %v, want one Barman Cloud WAL archiver", plugins)
	}
	serverName, _, _ := unstructured.NestedString(plugins[0].(map[string]any), "parameters", "serverName")
	if serverName != "upgrade-db-pg17" {
		t.Errorf("upgrade backup serverName = %q, want target generation", serverName)
	}
	var db appv1alpha1.Database
	if err := cl.Get(ctx, req.NamespacedName, &db); err != nil {
		t.Fatal(err)
	}
	if db.Status.Phase != appv1alpha1.DBPhaseUpgrading || db.Status.CurrentVersion != "16" {
		t.Fatalf("mid-upgrade status = phase %q current %q", db.Status.Phase, db.Status.CurrentVersion)
	}

	if err := unstructured.SetNestedField(cluster.Object, "Cluster in healthy state", "status", "phase"); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(cluster.Object, int64(1), "status", "readyInstances"); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(cluster.Object, int64(17), "status", "pgDataImageInfo", "majorVersion"); err != nil {
		t.Fatal(err)
	}
	if err := cl.Update(ctx, cluster); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, req.NamespacedName, &db); err != nil {
		t.Fatal(err)
	}
	if db.Status.Phase != appv1alpha1.DBPhaseReady || db.Status.CurrentVersion != "17" || db.Status.BackupServerName != "upgrade-db-pg17" {
		t.Fatalf("completed status = phase %q current %q backup server %q", db.Status.Phase, db.Status.CurrentVersion, db.Status.BackupServerName)
	}
	backup := &unstructured.Unstructured{}
	backup.SetGroupVersionKind(cnpgBackupGVK)
	if err := cl.Get(ctx, types.NamespacedName{Name: "upgrade-db-post-upgrade-pg17", Namespace: "default"}, backup); err != nil {
		t.Fatalf("post-upgrade base backup: %v", err)
	}
}

func TestReconcileFailedMajorVersionUpgradeRollsBackIntent(t *testing.T) {
	ctx := context.Background()
	r, cl, req := newMajorUpgradeReconciler(t, "Failed to upgrade Postgres major version")
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	var db appv1alpha1.Database
	if err := cl.Get(ctx, req.NamespacedName, &db); err != nil {
		t.Fatal(err)
	}
	if db.Spec.Version != "16" {
		t.Errorf("failed upgrade spec.version = %q, want source 16", db.Spec.Version)
	}
	if db.Status.Phase != appv1alpha1.DBPhaseFailed {
		t.Errorf("failed upgrade phase = %q", db.Status.Phase)
	}
}
