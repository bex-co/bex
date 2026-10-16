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

	"k8s.io/apimachinery/pkg/api/meta"
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
	if image != cnpgExportImages["17"] {
		t.Errorf("cluster image = %q, want pinned PostgreSQL 17", image)
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
	// Ready marks the database as having served, so a later restart reads
	// config_restart rather than creating (w4/m137).
	if !db.Status.Provisioned {
		t.Fatal("a Ready database must record status.provisioned")
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

// TestAFailedMajorUpgradeRecordsTheClustersVersion (w5/110): reverting a failed
// major upgrade's spec.version patched the Database itself, and the server's
// object replaced the status the pass had read from the cluster but not yet
// written. The Failed status then saved the version stored before the pass.
func TestAFailedMajorUpgradeRecordsTheClustersVersion(t *testing.T) {
	ctx := context.Background()
	r, cl, req := newMajorUpgradeReconciler(t, "Failed to upgrade Postgres major version")
	var db appv1alpha1.Database
	if err := cl.Get(ctx, req.NamespacedName, &db); err != nil {
		t.Fatal(err)
	}
	db.Status.CurrentVersion = "15" // stored before the cluster reached 16
	if err := cl.Status().Update(ctx, &db); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, req.NamespacedName, &db); err != nil {
		t.Fatal(err)
	}
	if db.Spec.Version != "16" || db.Status.Phase != appv1alpha1.DBPhaseFailed || db.Status.CurrentVersion != "16" {
		t.Fatalf("failed upgrade = spec %q, phase %q, current %q; want the revert to the cluster's 16, Failed, recording 16",
			db.Spec.Version, db.Status.Phase, db.Status.CurrentVersion)
	}
}

// setClusterStatus rewrites the fake CNPG Cluster's status for the next pass;
// major 0 removes pgDataImageInfo (nothing observed yet).
func setClusterStatus(t *testing.T, cl client.Client, req reconcile.Request, phase string, ready, major int64) {
	t.Helper()
	ctx := context.Background()
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	if err := cl.Get(ctx, req.NamespacedName, cluster); err != nil {
		t.Fatal(err)
	}
	status := map[string]any{"phase": phase, "readyInstances": ready}
	if major > 0 {
		status["pgDataImageInfo"] = map[string]any{"majorVersion": major}
	}
	if err := unstructured.SetNestedMap(cluster.Object, status, "status"); err != nil {
		t.Fatal(err)
	}
	if err := cl.Update(ctx, cluster); err != nil {
		t.Fatal(err)
	}
}

func reconcileDB(t *testing.T, r *DatabaseReconciler, cl client.Client, req reconcile.Request) appv1alpha1.Database {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), req.NamespacedName, &db); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestAcceptedMajorUpgradeStaysNonReadyUntilTheTargetIsHealthy is w4/m215's
// a713 sequence. Right after an accepted 16 → 17 write, CNPG has not yet
// patched its upgrade phase and the old major's instance is still healthy;
// that frame used to mark the new generation Ready (Available mid-upgrade).
// Every frame until the target major is healthy is now Upgrading with the
// source version recorded — including CNPG reporting the 17 PGDATA before its
// instance is ready, which used to skip the post-upgrade base backup and
// point archiving back at the source generation. Completion is Ready, records
// 17, takes that backup and archives into the pg17 generation.
func TestAcceptedMajorUpgradeStaysNonReadyUntilTheTargetIsHealthy(t *testing.T) {
	r, cl, req := newMajorUpgradeReconciler(t, "Cluster in healthy state")
	for _, frame := range []struct {
		name      string
		phase     string
		ready     int64
		major     int64
		wantPhase appv1alpha1.DatabasePhase
		wantSeen  string
	}{
		{"old major healthy, upgrade not yet announced", "Cluster in healthy state", 1, 16, appv1alpha1.DBPhaseUpgrading, "16"},
		{"CNPG upgrading", "Upgrading Postgres major version", 0, 16, appv1alpha1.DBPhaseUpgrading, "16"},
		{"target observed, instances not ready", "Cluster in healthy state", 0, 17, appv1alpha1.DBPhaseUpgrading, "16"},
		{"target healthy", "Cluster in healthy state", 1, 17, appv1alpha1.DBPhaseReady, "17"},
	} {
		setClusterStatus(t, cl, req, frame.phase, frame.ready, frame.major)
		db := reconcileDB(t, r, cl, req)
		ready := meta.IsStatusConditionTrue(db.Status.Conditions, appv1alpha1.ConditionReady)
		if db.Status.Phase != frame.wantPhase || db.Status.CurrentVersion != frame.wantSeen ||
			ready != (frame.wantPhase == appv1alpha1.DBPhaseReady) {
			t.Fatalf("%s: phase %q current %q Ready=%v, want phase %q current %q",
				frame.name, db.Status.Phase, db.Status.CurrentVersion, ready, frame.wantPhase, frame.wantSeen)
		}
	}
	backup := &unstructured.Unstructured{}
	backup.SetGroupVersionKind(cnpgBackupGVK)
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "upgrade-db-post-upgrade-pg17", Namespace: "default"}, backup); err != nil {
		t.Fatalf("post-upgrade base backup: %v", err)
	}
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), req.NamespacedName, &db); err != nil {
		t.Fatal(err)
	}
	if db.Status.BackupServerName != "upgrade-db-pg17" {
		t.Fatalf("backup server = %q, want the pg17 generation", db.Status.BackupServerName)
	}
}

// The pending hold must not change what other states mean: an unchanged
// healthy database is Ready, a first provision with nothing observed yet is
// Ready once CNPG is, and a failed upgrade's revert returns to Ready on the
// source major rather than sticking at Upgrading.
func TestPendingMajorUpgradeLeavesOtherReadinessAlone(t *testing.T) {
	t.Run("unchanged major", func(t *testing.T) {
		r, cl, req := newMajorUpgradeReconciler(t, "Cluster in healthy state")
		var db appv1alpha1.Database
		if err := cl.Get(context.Background(), req.NamespacedName, &db); err != nil {
			t.Fatal(err)
		}
		db.Spec.Version = "16"
		if err := cl.Update(context.Background(), &db); err != nil {
			t.Fatal(err)
		}
		setClusterStatus(t, cl, req, "Cluster in healthy state", 1, 16)
		if got := reconcileDB(t, r, cl, req); got.Status.Phase != appv1alpha1.DBPhaseReady {
			t.Fatalf("unchanged major phase = %q, want Ready", got.Status.Phase)
		}
	})
	t.Run("nothing observed yet", func(t *testing.T) {
		r, cl, req := newMajorUpgradeReconciler(t, "Cluster in healthy state")
		var db appv1alpha1.Database
		if err := cl.Get(context.Background(), req.NamespacedName, &db); err != nil {
			t.Fatal(err)
		}
		db.Status = appv1alpha1.DatabaseStatus{}
		if err := cl.Status().Update(context.Background(), &db); err != nil {
			t.Fatal(err)
		}
		setClusterStatus(t, cl, req, "Cluster in healthy state", 1, 0)
		if got := reconcileDB(t, r, cl, req); got.Status.Phase != appv1alpha1.DBPhaseReady {
			t.Fatalf("first provision phase = %q, want Ready", got.Status.Phase)
		}
	})
	t.Run("failed upgrade reverted", func(t *testing.T) {
		r, cl, req := newMajorUpgradeReconciler(t, "Failed to upgrade Postgres major version")
		if got := reconcileDB(t, r, cl, req); got.Spec.Version != "16" || got.Status.Phase != appv1alpha1.DBPhaseFailed {
			t.Fatalf("failed upgrade = spec %q phase %q", got.Spec.Version, got.Status.Phase)
		}
		setClusterStatus(t, cl, req, "Cluster in healthy state", 1, 16)
		if got := reconcileDB(t, r, cl, req); got.Status.Phase != appv1alpha1.DBPhaseReady {
			t.Fatalf("after revert phase = %q, want Ready on the source major", got.Status.Phase)
		}
	})
}

func TestMajorUpgradePending(t *testing.T) {
	for _, tc := range []struct {
		desired, observed string
		want              bool
	}{
		{"17", "16", true},
		{"18", "16", true},
		{"16", "16", false},
		{"16", "17", false}, // never a downgrade
		{"", "16", false},   // operator default
		{"17", "", false},   // nothing observed yet
		{"17", "0", false},
		{"latest", "16", false},
	} {
		if got := majorUpgradePending(tc.desired, tc.observed); got != tc.want {
			t.Errorf("majorUpgradePending(%q, %q) = %v, want %v", tc.desired, tc.observed, got, tc.want)
		}
	}
}
