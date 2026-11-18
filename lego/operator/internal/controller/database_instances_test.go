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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPrimaryClusterHoldsOnlyTheHAStandby (w6/m467): read replicas are
// clusters of their own, so the primary Cluster is the plan's instance plus
// the HA standby whatever the replicas, and CNPG can promote nothing but that
// standby.
func TestPrimaryClusterHoldsOnlyTheHAStandby(t *testing.T) {
	for _, tc := range []struct {
		name, plan string
		ha         bool
		want       int64
	}{
		{"without HA", "basic-1gb", false, 1},
		{"with HA", "basic-4gb", true, 2},
		{"free", "free", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{Plan: tc.plan})
			spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: 10, highAvailability: tc.ha})
			if got := spec["instances"]; got != tc.want {
				t.Fatalf("projected instances = %v, want %d", got, tc.want)
			}
			if _, has := spec["affinity"]; has != tc.ha {
				t.Fatal("the HA affinity policy does not follow HA")
			}
			if tc.want > 1 && (spec["enablePDB"] != nil || spec["probes"] != nil) {
				t.Fatal("an HA cluster lost CNPG's disruption protection or readiness probe")
			}
		})
	}
}

// TestReadReplicasProvisioned keeps admission's replica policy: a plan with
// the CPU for replicas, at most five, and storage within the replica bounds.
func TestReadReplicasProvisioned(t *testing.T) {
	for _, tc := range []struct {
		name, plan string
		storage    int32
		replicas   int
		want       bool
	}{
		{"one replica", "basic-1gb", 10, 1, true},
		{"five replicas", "basic-1gb", 10, 5, true},
		{"none declared", "basic-1gb", 10, 0, false},
		{"free gains none", "free", 10, 1, false},
		{"small paid gains none", "basic-256mb", 10, 1, false},
		{"insufficient storage", "basic-1gb", 9, 1, false},
		{"excess count", "basic-1gb", 10, 6, false},
		{"over storage maximum", "basic-1gb", 16385, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{Plan: tc.plan})
			if got := readReplicasProvisioned(plan, tc.storage, tc.replicas); got != tc.want {
				t.Fatalf("provisioned = %v, want %v", got, tc.want)
			}
		})
	}
}

func namedReaders(n int) []appv1alpha1.DatabaseReadReplica {
	readers := make([]appv1alpha1.DatabaseReadReplica, n)
	for i := range readers {
		readers[i].Name = fmt.Sprintf("reader-%d", i+1)
	}
	return readers
}

// TestDatabaseReplicasMoveOutOfThePrimaryCluster: a database whose readers
// were instances of its one Cluster (before w6/m467) returns that Cluster to
// the plan's instance, and grows no storage back down. CNPG removes the
// highest-serial standbys and never the primary, so no primary Pod rolls.
func TestDatabaseReplicasMoveOutOfThePrimaryCluster(t *testing.T) {
	for _, tc := range []struct {
		name, plan  string
		provisioned bool
	}{
		{"eligible readers become replica clusters", "basic-1gb", true},
		{"a legacy declaration outside policy keeps no reader", "free", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "legacy-readers", Namespace: "default", Finalizers: []string{dbFinalizer}},
				Spec:   appv1alpha1.DatabaseSpec{Plan: tc.plan, ReadReplicas: namedReaders(2)},
				Status: appv1alpha1.DatabaseStatus{AllocatedStorageGB: 15}}
			r := databaseStorageFixture(t, db, 15)
			key := client.ObjectKeyFromObject(db)
			cluster := &unstructured.Unstructured{}
			cluster.SetGroupVersionKind(cnpgClusterGVK)
			if err := r.Get(context.Background(), key, cluster); err != nil {
				t.Fatal(err)
			}
			if err := unstructured.SetNestedField(cluster.Object, int64(3), "spec", "instances"); err != nil {
				t.Fatal(err)
			}
			if err := unstructured.SetNestedField(cluster.Object, int64(3), "status", "instances"); err != nil {
				t.Fatal(err)
			}
			if err := r.Update(context.Background(), cluster); err != nil {
				t.Fatal(err)
			}
			if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: key}); err != nil {
				t.Fatal(err)
			}
			if err := r.Get(context.Background(), key, cluster); err != nil {
				t.Fatal(err)
			}
			if instances, _, _ := unstructured.NestedInt64(cluster.Object, "spec", "instances"); instances != 1 || cnpgStorageGB(cluster) != 15 {
				t.Fatalf("primary Cluster kept its readers or shrank: %+v", cluster.Object["spec"])
			}
			if err := r.Get(context.Background(), key, db); err != nil {
				t.Fatal(err)
			}
			if db.Spec.Plan != tc.plan || len(db.Spec.ReadReplicas) != 2 || db.Spec.StorageGB != 0 || db.Status.AllocatedStorageGB != 15 {
				t.Fatalf("intent changed: spec=%+v status=%+v", db.Spec, db.Status)
			}
			got := db.Status.ReadReplicaStatuses
			if len(got) != 2 {
				t.Fatalf("read replica statuses = %+v", got)
			}
			want := ""
			if tc.provisioned {
				want = appv1alpha1.ReadReplicaServiceName(db.Name, "reader-1") + ".default.svc"
			}
			if got[0].InternalHost != want {
				t.Fatalf("reader-1 internal host = %q, want %q", got[0].InternalHost, want)
			}
		})
	}
}

// TestDatabaseReplicaAutoscalingAccountsForEveryPVC: a disk growth reaches
// every instance's PVC, the primary Cluster's and each replica cluster's,
// and crossing the replica storage minimum provisions each declared replica a
// whole new one.
func TestDatabaseReplicaAutoscalingAccountsForEveryPVC(t *testing.T) {
	for _, tc := range []struct {
		name                string
		ha                  bool
		current, next       int32
		observed            int64
		replicaClusters     int
		used, blocked, fits string
	}{
		// 1 primary + 1 replica: 2×15 − 2×10 = 10.
		{"existing replica", false, 10, 15, 1, 1, "20Gi", "25Gi", "30Gi"},
		// The replica is declared but not cloned yet: 2×15 − 1×10 = 20.
		{"replica not cloned yet", false, 10, 15, 1, 0, "10Gi", "20Gi", "30Gi"},
		// No observed count: the usage sample proves the primary's PVC.
		{"missing observed count", false, 10, 15, 0, 1, "20Gi", "25Gi", "30Gi"},
		// 5 GB holds no replica; 10 GB provisions it: 2×10 − 1×5 = 15.
		{"crosses replica minimum", false, 5, 10, 1, 0, "5Gi", "15Gi", "20Gi"},
		// HA standby and a replica: 3×15 − 3×10 = 15.
		{"HA standby and replica", true, 10, 15, 2, 1, "30Gi", "40Gi", "45Gi"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "reader-quota", Namespace: "default"},
				Spec: appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: tc.current, DiskAutoscaling: true,
					HighAvailability: tc.ha, ReadReplicas: namedReaders(1)}}
			r := databaseStorageFixture(t, db, tc.current)
			r.Recorder = record.NewFakeRecorder(10)
			r.DiskUsageReader = func(context.Context, string, string) (DatabaseDiskUsage, error) {
				return DatabaseDiskUsage{UsedBytes: 90, CapacityBytes: 100}, nil
			}
			key := client.ObjectKeyFromObject(db)
			cluster := &unstructured.Unstructured{}
			cluster.SetGroupVersionKind(cnpgClusterGVK)
			if err := r.Get(context.Background(), key, cluster); err != nil {
				t.Fatal(err)
			}
			if err := unstructured.SetNestedField(cluster.Object, tc.observed, "status", "instances"); err != nil {
				t.Fatal(err)
			}
			if err := r.Update(context.Background(), cluster); err != nil {
				t.Fatal(err)
			}
			for i := range tc.replicaClusters {
				replica := &unstructured.Unstructured{}
				replica.SetGroupVersionKind(cnpgClusterGVK)
				replica.SetName(appv1alpha1.ReadReplicaClusterName(db.Name, db.Spec.ReadReplicas[i].Name))
				replica.SetNamespace(db.Namespace)
				replica.SetLabels(map[string]string{labelReadReplicaOf: db.Name})
				if err := controllerutil.SetControllerReference(db, replica, r.Scheme); err != nil {
					t.Fatal(err)
				}
				if err := r.Create(context.Background(), replica); err != nil {
					t.Fatal(err)
				}
			}
			quota := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "storage", Namespace: db.Namespace},
				Status: corev1.ResourceQuotaStatus{
					Hard: corev1.ResourceList{corev1.ResourceRequestsStorage: resource.MustParse(tc.blocked)},
					Used: corev1.ResourceList{corev1.ResourceRequestsStorage: resource.MustParse(tc.used)},
				}}
			if err := r.Create(context.Background(), quota); err != nil {
				t.Fatal(err)
			}
			if _, err := r.applyDiskAutoscaling(context.Background(), db); err != nil {
				t.Fatal(err)
			}
			if err := r.Get(context.Background(), key, db); err != nil {
				t.Fatal(err)
			}
			if db.Spec.StorageGB != tc.current || !meta.IsStatusConditionTrue(db.Status.Conditions, conditionDiskGrowthBlockedByQuota) {
				t.Fatalf("insufficient replica capacity was not blocked: %+v", db)
			}
			quota.Status.Hard[corev1.ResourceRequestsStorage] = resource.MustParse(tc.fits)
			if err := r.Update(context.Background(), quota); err != nil {
				t.Fatal(err)
			}
			if _, err := r.applyDiskAutoscaling(context.Background(), db); err != nil {
				t.Fatal(err)
			}
			if err := r.Get(context.Background(), key, db); err != nil {
				t.Fatal(err)
			}
			if db.Spec.StorageGB != tc.next || meta.IsStatusConditionTrue(db.Status.Conditions, conditionDiskGrowthBlockedByQuota) {
				t.Fatalf("replica growth did not resume with sufficient quota: %+v", db)
			}
		})
	}
}
