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
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func deletionApp(name string) *appv1alpha1.App {
	now := metav1.Now()
	return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", UID: types.UID("uid-" + name),
		Finalizers: []string{finalizer}, DeletionTimestamp: &now,
	}}
}

func deletionScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func TestAppDeletionRetainsFinalizerAcrossTransientInventoryFailure(t *testing.T) {
	app := deletionApp("retry-app")
	failOnce := true
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).
		WithInterceptorFuncs(interceptor.Funcs{List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*batchv1.JobList); ok && failOnce {
				failOnce = false
				return errors.New("transient Job inventory failure")
			}
			return c.List(ctx, list, opts...)
		}}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); err == nil {
		t.Fatal("transient inventory failure was swallowed")
	}
	var current appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &current); err != nil {
		t.Fatal(err)
	}
	if !controllerutil.ContainsFinalizer(&current, finalizer) {
		t.Fatal("inventory failure released the App finalizer")
	}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), nn, &current); !apierrors.IsNotFound(err) {
		t.Fatalf("successful retry did not finish deletion: %v", err)
	}
}

func TestAppDeletionRemovesHistoricalTLSSecretBeforeFinalizer(t *testing.T) {
	app := deletionApp("tls-history")
	app.Annotations = map[string]string{annotTLSSecretHistory: `["tls-history-tls-old.example.com"]`}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls-history-tls-old.example.com", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, secret).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(secret), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("historical TLS Secret survived: %v", err)
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); err != nil {
		t.Fatal("finalizer released before absence verification")
	}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); !apierrors.IsNotFound(err) {
		t.Fatalf("App remained after TLS absence was proven: %v", err)
	}
}

func TestStaticAppDeletionWaitsForPurgeCompletionAndJobAbsence(t *testing.T) {
	app := deletionApp("static-delete")
	app.Spec.Type = appv1alpha1.TypeStaticSite
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
	store := publish.Store{Bucket: "static", Endpoint: "https://s3.example", Secret: "static-creds"}
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, StaticStore: store}
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	req := reconcile.Request{NamespacedName: nn}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	desired := publish.PurgeJob(app.Name, string(app.UID), "", app.Namespace, store, app.Namespace, "", "")
	var job batchv1.Job
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(desired), &job); err != nil {
		t.Fatalf("static purge Job not persisted: %v", err)
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
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); !apierrors.IsNotFound(err) {
		t.Fatalf("static App survived proven purge completion: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(desired), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("static purge Job survived finalization: %v", err)
	}
}
