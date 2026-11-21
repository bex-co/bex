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
	"k8s.io/apimachinery/pkg/api/meta"
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

// TestDatabaseDeletionRemovesAddedUsersSecrets is w34/004: the password Secret
// of a user added before bex-api owner-referenced it has nothing for garbage
// collection to follow, so the finalizer deletes every spec.users Secret,
// holding the Database until it can. Production had 18 left behind by deleted
// databases. A spec.users entry naming a Secret that is not a user Secret of
// this database is never followed.
func TestDatabaseDeletionRemovesAddedUsersSecrets(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(cnpgClusterGVK, &unstructured.Unstructured{})

	now := metav1.Now()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dpg-gone", Namespace: "default", Finalizers: []string{dbFinalizer}, DeletionTimestamp: &now,
		},
		Spec: appv1alpha1.DatabaseSpec{Users: []appv1alpha1.DatabaseUser{
			{Name: "reporting", SecretName: "dpg-gone-user-0610cb9fb5aae04d3842844735b6c3af"},
			{Name: "legacy", SecretName: "dpg-gone-user-legacy"},
			{Name: "stray", SecretName: "dpg-other-user-0610cb9fb5aae04d3842844735b6c3af"},
			{Name: "app", SecretName: "dpg-gone-app"},
		}},
	}
	secret := func(name string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: db.Namespace}}
	}
	failOnce := true
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(db, secret(db.Spec.Users[0].SecretName), secret(db.Spec.Users[1].SecretName),
			secret(db.Spec.Users[2].SecretName), secret(db.Spec.Users[3].SecretName)).
		WithStatusSubresource(&appv1alpha1.Database{}).
		WithInterceptorFuncs(interceptor.Funcs{Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, isSecret := obj.(*corev1.Secret); isSecret && failOnce {
				failOnce = false
				return errors.New("transient secret delete failure")
			}
			return c.Delete(ctx, obj, opts...)
		}}).Build()
	r := &DatabaseReconciler{Client: cl, Scheme: scheme}
	ctx := context.Background()
	nn := types.NamespacedName{Name: db.Name, Namespace: db.Namespace}
	req := reconcile.Request{NamespacedName: nn}

	if _, err := r.Reconcile(ctx, req); err == nil || !strings.Contains(err.Error(), "transient") {
		t.Fatalf("first secret delete error = %v", err)
	}
	current := &appv1alpha1.Database{}
	if err := cl.Get(ctx, nn, current); err != nil || !controllerutil.ContainsFinalizer(current, dbFinalizer) {
		t.Fatalf("a failed user Secret delete released the Database (err=%v): its Secret would be orphaned", err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if err := cl.Get(ctx, nn, current); !apierrors.IsNotFound(err) {
		t.Fatalf("database remained after its user Secrets were deleted: %v", err)
	}
	for _, u := range db.Spec.Users[:2] {
		if err := cl.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: u.SecretName}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
			t.Errorf("user %q Secret %s outlived its database: %v", u.Name, u.SecretName, err)
		}
	}
	for _, u := range db.Spec.Users[2:] {
		if err := cl.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: u.SecretName}, &corev1.Secret{}); err != nil {
			t.Errorf("Secret %s is not a user Secret of %s and must survive: %v", u.SecretName, db.Name, err)
		}
	}
}

// codex round-9 #13: the backup-purge Job runs in a tenant hosting namespace,
// where the extended workload-admission grammar now denies added capabilities,
// privilege escalation, and non-RuntimeDefault seccomp — the generated Job must
// carry the same execution.RestrictedContainerSecurityContext the export Jobs
// already stamp.
func TestDatabaseBackupPurgeJobCarriesTenantHardening(t *testing.T) {
	r := &DatabaseReconciler{Backup: BackupStore{
		DestinationPath: "s3://backups/postgres", EndpointURL: "https://s3.example", S3Secret: "backup-creds",
	}}
	job := r.dbBackupPurgeJob(&appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: "purge-hardening", Namespace: "default", UID: "uid-purge-hardening",
	}})
	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("purge Job containers = %d, want 1", len(job.Spec.Template.Spec.Containers))
	}
	sc := job.Spec.Template.Spec.Containers[0].SecurityContext
	if sc == nil {
		t.Fatal("purge container carries no SecurityContext — admission would deny the Job")
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Fatal("purge container must not allow privilege escalation")
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Drop) == 0 || sc.Capabilities.Drop[0] != "ALL" {
		t.Fatal("purge container must drop ALL capabilities")
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatal("purge container must pin RuntimeDefault seccomp")
	}
	if job.Spec.Template.Spec.AutomountServiceAccountToken == nil || *job.Spec.Template.Spec.AutomountServiceAccountToken {
		t.Fatal("purge pod must disable ServiceAccount token automount")
	}
	if job.Spec.Template.Spec.NodeName != "" {
		t.Fatal("purge pod must not pin a node directly")
	}
}

// TestDatabaseDeletionSurfacesStalledConditionOnOverrun is w8/012: a Database
// finalization that has not converged past the finalization window stamps
// DeletionStalled, backs the requeue off to childHealthRequeue, and retains the
// finalizer — same bound Apps got in w3/m81.
func TestDatabaseDeletionSurfacesStalledConditionOnOverrun(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(cnpgClusterGVK, &unstructured.Unstructured{})
	now := metav1.Now()
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: "stalled-db", Namespace: "default", UID: "uid-stalled-db",
		Finalizers: []string{dbFinalizer}, DeletionTimestamp: &now,
	}, Status: appv1alpha1.DatabaseStatus{BackupsEnabled: true}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).
		WithStatusSubresource(&appv1alpha1.Database{}, &batchv1.Job{}).Build()
	r := &DatabaseReconciler{
		Client: cl, Scheme: scheme, FinalizerOverrunAfter: time.Nanosecond,
		Backup: BackupStore{
			DestinationPath: "s3://backups/postgres", EndpointURL: "https://s3.example", S3Secret: "backup-creds",
		},
	}
	nn := types.NamespacedName{Name: db.Name, Namespace: db.Namespace}
	result, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != childHealthRequeue {
		t.Fatalf("stalled requeue = %v, want childHealthRequeue %v", result.RequeueAfter, childHealthRequeue)
	}
	var current appv1alpha1.Database
	if err := cl.Get(context.Background(), nn, &current); err != nil {
		t.Fatal(err)
	}
	if !controllerutil.ContainsFinalizer(&current, dbFinalizer) {
		t.Fatal("finalizer must be retained while stalled")
	}
	cond := meta.FindStatusCondition(current.Status.Conditions, conditionDeletionStalled)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("DeletionStalled condition = %+v, want present and True", cond)
	}
	if cond.Reason != reasonCleanupExceededDeadline {
		t.Fatalf("DeletionStalled reason = %q, want %q", cond.Reason, reasonCleanupExceededDeadline)
	}
}

// TestDatabaseDeletionDoesNotStampStalledInWindow guards the healthy path:
// an in-progress purge within the bound keeps settleRequeue and stamps no
// DeletionStalled condition.
func TestDatabaseDeletionDoesNotStampStalledInWindow(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(cnpgClusterGVK, &unstructured.Unstructured{})
	now := metav1.Now()
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: "in-window-db", Namespace: "default", UID: "uid-in-window-db",
		Finalizers: []string{dbFinalizer}, DeletionTimestamp: &now,
	}, Status: appv1alpha1.DatabaseStatus{BackupsEnabled: true}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(db).
		WithStatusSubresource(&appv1alpha1.Database{}, &batchv1.Job{}).Build()
	r := &DatabaseReconciler{
		Client: cl, Scheme: scheme, // default 15m window
		Backup: BackupStore{
			DestinationPath: "s3://backups/postgres", EndpointURL: "https://s3.example", S3Secret: "backup-creds",
		},
	}
	nn := types.NamespacedName{Name: db.Name, Namespace: db.Namespace}
	result, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != settleRequeue {
		t.Fatalf("in-window requeue = %v, want settleRequeue %v", result.RequeueAfter, settleRequeue)
	}
	var current appv1alpha1.Database
	if err := cl.Get(context.Background(), nn, &current); err != nil {
		t.Fatal(err)
	}
	if cond := meta.FindStatusCondition(current.Status.Conditions, conditionDeletionStalled); cond != nil {
		t.Fatalf("healthy in-progress deletion stamped DeletionStalled: %+v", cond)
	}
}
