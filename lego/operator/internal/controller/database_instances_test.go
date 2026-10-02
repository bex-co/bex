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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestCNPGNamedReaderCapacity(t *testing.T) {
	for _, tc := range []struct {
		name, plan string
		storage    int32
		readers    int
		ha         bool
		current    int64
		want       int64
	}{
		{"one reader without HA", "basic-1gb", 10, 1, false, 1, 2},
		{"five readers without HA", "basic-1gb", 10, 5, false, 1, 6},
		{"reader plus HA standby", "basic-1gb", 10, 1, true, 2, 3},
		{"free does not gain readers", "free", 10, 1, false, 1, 1},
		{"small paid does not gain readers", "basic-256mb", 10, 1, false, 1, 1},
		{"insufficient storage", "basic-1gb", 9, 1, false, 1, 1},
		{"excess count", "basic-1gb", 10, 6, false, 1, 1},
		{"over storage maximum", "basic-1gb", 16385, 1, false, 1, 1},
		{"legacy free capacity preserved", "free", 10, 1, false, 3, 3},
		{"legacy count capacity preserved", "basic-1gb", 10, 6, false, 4, 4},
		{"legacy HA base preserved", "free", 10, 1, true, 1, 2},
		{"explicit reader removal", "basic-1gb", 10, 0, false, 6, 1},
		{"explicit removal keeps HA", "basic-1gb", 10, 0, true, 3, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, _ := resolvePlan(appv1alpha1.DatabaseSpec{Plan: tc.plan})
			spec := cnpgClusterSpec(clusterParams{plan: plan, storageGB: tc.storage,
				highAvailability: tc.ha, readReplicas: tc.readers, currentInstances: tc.current})
			if got := spec["instances"]; got != tc.want {
				t.Fatalf("projected instances = %v, want %d", got, tc.want)
			}
			if _, has := spec["affinity"]; has != tc.ha {
				t.Fatal("named readers changed the HA affinity policy")
			}
			if tc.want > 1 && spec["enablePDB"] != nil {
				t.Fatal("multi-instance cluster disabled CNPG disruption protection")
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

func TestDatabaseReaderReconcilePreservesLegacyCapacityAndStorage(t *testing.T) {
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "legacy-readers", Namespace: "default", Finalizers: []string{dbFinalizer}},
		Spec:   appv1alpha1.DatabaseSpec{Plan: "free", ReadReplicas: namedReaders(1)},
		Status: appv1alpha1.DatabaseStatus{AllocatedStorageGB: 15}}
	r := databaseStorageFixture(t, db, 15)
	key := client.ObjectKeyFromObject(db)
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	if err := r.Get(context.Background(), key, cluster); err != nil {
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
	if instances, _, _ := unstructured.NestedInt64(cluster.Object, "spec", "instances"); instances != 3 || cnpgStorageGB(cluster) != 15 {
		t.Fatalf("legacy cluster lost capacity: %+v", cluster.Object["spec"])
	}
	if err := r.Get(context.Background(), key, db); err != nil {
		t.Fatal(err)
	}
	if db.Spec.Plan != "free" || len(db.Spec.ReadReplicas) != 1 || db.Spec.StorageGB != 0 || db.Status.AllocatedStorageGB != 15 {
		t.Fatalf("legacy intent changed: spec=%+v status=%+v", db.Spec, db.Status)
	}
	db.Spec.ReadReplicas = nil
	if err := r.Update(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: key}); err != nil {
		t.Fatal(err)
	}
	if err := r.Get(context.Background(), key, cluster); err != nil {
		t.Fatal(err)
	}
	if instances, _, _ := unstructured.NestedInt64(cluster.Object, "spec", "instances"); instances != 1 || cnpgStorageGB(cluster) != 15 {
		t.Fatalf("explicit removal must return to the base without shrinking storage: %+v", cluster.Object["spec"])
	}
}

func TestDatabaseReaderAutoscalingAccountsForEveryPVC(t *testing.T) {
	for _, tc := range []struct {
		name                string
		current, next       int32
		declared, observed  int64
		used, blocked, fits string
	}{
		{"existing reader", 10, 15, 2, 2, "20Gi", "25Gi", "30Gi"},
		{"previous alias gains reader", 10, 15, 1, 1, "10Gi", "20Gi", "30Gi"},
		{"reader projection in progress", 10, 15, 2, 1, "10Gi", "20Gi", "30Gi"},
		{"missing observed count", 10, 15, 2, 0, "10Gi", "20Gi", "30Gi"},
		{"crosses reader minimum", 5, 10, 1, 1, "5Gi", "15Gi", "20Gi"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "reader-quota", Namespace: "default"},
				Spec: appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: tc.current, DiskAutoscaling: true, ReadReplicas: namedReaders(1)}}
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
			if err := unstructured.SetNestedField(cluster.Object, tc.declared, "spec", "instances"); err != nil {
				t.Fatal(err)
			}
			if err := unstructured.SetNestedField(cluster.Object, tc.observed, "status", "instances"); err != nil {
				t.Fatal(err)
			}
			if err := r.Update(context.Background(), cluster); err != nil {
				t.Fatal(err)
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
				t.Fatalf("insufficient reader capacity was not blocked: %+v", db)
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
				t.Fatalf("reader growth did not resume with sufficient quota: %+v", db)
			}
		})
	}
}
