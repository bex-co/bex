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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func databaseStorageFixture(t *testing.T, db *appv1alpha1.Database, clusterGB int32) *DatabaseReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	for _, gvk := range []schema.GroupVersionKind{
		cnpgClusterGVK, cnpgScheduledBackupGVK, cnpgBackupGVK, cnpgPoolerGVK,
		traefikIngressRouteTCPGVK, traefikMiddlewareTCPGVK,
	} {
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1", "kind": "Cluster",
		"metadata": map[string]any{"name": db.Name, "namespace": db.Namespace},
		"spec":     map[string]any{"storage": map[string]any{"size": storageGi(clusterGB)}},
	}}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, cluster).
		WithStatusSubresource(&appv1alpha1.Database{}).Build()
	return &DatabaseReconciler{Client: cl, Scheme: scheme}
}

func storageGi(gb int32) string { return fmt.Sprintf("%dGi", gb) }

func TestDatabaseStorageIsMonotonicAcrossPlanChanges(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "grow-only-db", Namespace: "default", Finalizers: []string{dbFinalizer}},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free"},
		Status: appv1alpha1.DatabaseStatus{
			AllocatedStorageGB: 5, ObservedStorageGB: 0,
		},
	}
	r := databaseStorageFixture(t, db, 5)
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(cnpgClusterGVK)
	if err := r.Get(context.Background(), req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	if size, _, _ := unstructured.NestedString(got.Object, "spec", "storage", "size"); size != "5Gi" {
		t.Fatalf("plan downgrade projected %q, want retained 5Gi", size)
	}

	current := &appv1alpha1.Database{}
	if err := r.Get(context.Background(), req.NamespacedName, current); err != nil {
		t.Fatal(err)
	}
	current.Spec.StorageGB = 10
	if err := r.Update(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := r.Get(context.Background(), req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	if size, _, _ := unstructured.NestedString(got.Object, "spec", "storage", "size"); size != "10Gi" {
		t.Fatalf("explicit grow projected %q, want 10Gi", size)
	}
}

func TestDatabaseStorageShrinkIsRejectedBeforeCNPGMutation(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "no-shrink-db", Namespace: "default", Finalizers: []string{dbFinalizer}},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free", StorageGB: 5},
		Status: appv1alpha1.DatabaseStatus{
			AllocatedStorageGB: 10, ObservedStorageGB: 10,
		},
	}
	r := databaseStorageFixture(t, db, 10)
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: db.Name, Namespace: db.Namespace}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(cnpgClusterGVK)
	if err := r.Get(context.Background(), req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	if size, _, _ := unstructured.NestedString(got.Object, "spec", "storage", "size"); size != "10Gi" {
		t.Fatalf("rejected shrink mutated CNPG storage to %q", size)
	}
	current := &appv1alpha1.Database{}
	if err := r.Get(context.Background(), req.NamespacedName, current); err != nil {
		t.Fatal(err)
	}
	condition := metaReadyCondition(current.Status.Conditions)
	if current.Status.Phase != appv1alpha1.DBPhaseFailed || condition == nil || condition.Reason != "StorageShrinkRejected" {
		t.Fatalf("shrink status = phase %q condition %+v", current.Status.Phase, condition)
	}
}
