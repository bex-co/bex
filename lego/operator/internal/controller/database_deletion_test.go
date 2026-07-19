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
	"errors"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestDatabaseDeletionWaitsForBackupPurgeCompletionAndJobAbsence(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(cnpgClusterGVK, &unstructured.Unstructured{})
	now := metav1.Now()
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: "backed-up-db", Namespace: "default", UID: "uid-backed-up-db",
		Finalizers: []string{dbFinalizer}, DeletionTimestamp: &now,
	}, Status: appv1alpha1.DatabaseStatus{BackupsEnabled: true}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).
		WithStatusSubresource(&appv1alpha1.Database{}, &batchv1.Job{}).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme, Backup: BackupStore{
		DestinationPath: "s3://backups/postgres", EndpointURL: "https://s3.example", S3Secret: "backup-creds",
	}}
	nn := types.NamespacedName{Name: db.Name, Namespace: db.Namespace}
	req := reconcile.Request{NamespacedName: nn}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	desired := r.dbBackupPurgeJob(db)
	var job batchv1.Job
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(desired), &job); err != nil {
		t.Fatalf("purge Job not persisted: %v", err)
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.Database{}); err != nil {
		t.Fatal("Database disappeared before purge completed")
	}
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := cl.Status().Update(context.Background(), &job); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.Database{}); !apierrors.IsNotFound(err) {
		t.Fatalf("Database survived proven purge completion: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(desired), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("purge Job survived finalization: %v", err)
	}
}

// TestDatabaseDeletionDeletesClusterAndReleasesFinalizer pins the w7/m12
// finalizer against its deadlock: owner-ref garbage collection only removes the
// CNPG Cluster AFTER the Database object is gone from storage, while the
// finalizer keeps the Database around until the Cluster is gone. If the
// teardown merely waits for the Cluster to disappear, deletion never completes
// (observed live: a dashboard delete answered success but the CR, Cluster, pod
// and PVC all survived). The reconciler must therefore delete the Cluster
// itself, then release the finalizer once it is gone.
func TestDatabaseDeletionDeletesClusterAndReleasesFinalizer(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(cnpgClusterGVK, &unstructured.Unstructured{})

	now := metav1.Now()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "doomed-db",
			Namespace:         "default",
			Finalizers:        []string{dbFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: appv1alpha1.DatabaseSpec{Plan: "free"},
	}
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cluster.SetName("doomed-db")
	cluster.SetNamespace("default")

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, cluster).
		WithStatusSubresource(&appv1alpha1.Database{}).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "doomed-db", Namespace: "default"}
	req := reconcile.Request{NamespacedName: nn}

	// First reconcile: the Cluster still exists, so the teardown must delete it
	// (not just wait for it) and requeue for the cascade to finish.
	res, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("reconcile with live cluster: %v", err)
	}
	if res.RequeueAfter != 5*time.Second {
		t.Errorf("expected 5s requeue while the cluster winds down, got %+v", res)
	}
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(cnpgClusterGVK)
	if err := cl.Get(ctx, nn, got); !apierrors.IsNotFound(err) {
		t.Fatalf("cluster should have been deleted by the finalizer teardown, got err=%v", err)
	}

	// Second reconcile: the Cluster is gone, so the finalizer is released and
	// the Database object itself finally leaves storage.
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile after cluster gone: %v", err)
	}
	var left appv1alpha1.Database
	if err := cl.Get(ctx, nn, &left); !apierrors.IsNotFound(err) {
		t.Fatalf("database should be fully deleted once the finalizer is released, got err=%v (finalizers=%v)", err, left.Finalizers)
	}
}

func TestDatabaseDeletionRetainsFinalizerAndRetriesCleanupError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(cnpgClusterGVK, &unstructured.Unstructured{})

	now := metav1.Now()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: "retry-delete-db", Namespace: "default", Finalizers: []string{dbFinalizer}, DeletionTimestamp: &now,
		},
	}
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(cnpgClusterGVK)
	cluster.SetName(db.Name)
	cluster.SetNamespace(db.Namespace)
	failOnce := true
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db, cluster).
		WithStatusSubresource(&appv1alpha1.Database{}).
		WithInterceptorFuncs(interceptor.Funcs{Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if failOnce && obj.GetObjectKind().GroupVersionKind() == cnpgClusterGVK {
				failOnce = false
				return errors.New("transient cluster delete failure")
			}
			return c.Delete(ctx, obj, opts...)
		}}).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	nn := types.NamespacedName{Name: db.Name, Namespace: db.Namespace}
	req := reconcile.Request{NamespacedName: nn}

	if _, err := r.Reconcile(context.Background(), req); err == nil || !strings.Contains(err.Error(), "transient") {
		t.Fatalf("first cleanup error = %v", err)
	}
	current := &appv1alpha1.Database{}
	if err := cl.Get(context.Background(), nn, current); err != nil {
		t.Fatal(err)
	}
	if !controllerutil.ContainsFinalizer(current, dbFinalizer) {
		t.Fatal("cleanup error released the Database finalizer")
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("cleanup retry: %v", err)
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("release finalizer: %v", err)
	}
	if err := cl.Get(context.Background(), nn, current); !apierrors.IsNotFound(err) {
		t.Fatalf("database remained after successful retry: %v", err)
	}
}
