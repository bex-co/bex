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
	networkingv1 "k8s.io/api/networking/v1"
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

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
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
	scheme.AddKnownTypeWithName(certManagerCertificateGVK, &unstructured.Unstructured{})
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

func TestAppDeletionDuringFirstBuildConvergesAcrossManagerRestart(t *testing.T) {
	app := deletionApp("first-build")
	app.Namespace = "apps"
	app.Spec.Repo = "https://github.com/acme/first-build"
	app.Labels = map[string]string{labelWorkspace: "tea-test"}
	o := build.Options{
		Repo: app.Spec.Repo, Ref: "main", Name: app.Name, AppUID: string(app.UID),
		Registry: "zot.example", Revision: "gen-1", Namespace: "builds",
		AppNamespace: app.Namespace, Workspace: app.Labels[labelWorkspace],
	}
	job := build.BuildJob(o, o.ImageRef())
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "first-build-pod", Namespace: "builds",
		Labels: execution.PodLabels(app.Name, string(app.UID), "build", app.Labels[labelWorkspace], app.Namespace, false),
	}}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, job, pod).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	req := reconcile.Request{NamespacedName: nn}
	r := &AppReconciler{Client: cl, BuildClient: cl, BuildNamespace: "builds", Scheme: cl.Scheme(), Mode: ModeKubernetes}
	if result, err := r.Reconcile(context.Background(), req); err != nil || result.RequeueAfter == 0 {
		t.Fatalf("first cleanup pass = result %+v err %v, want pending requeue", result, err)
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); err != nil {
		t.Fatalf("finalizer released before fresh absence inventory: %v", err)
	}
	for _, obj := range []client.Object{
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: job.Name, Namespace: job.Namespace}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace}},
	} {
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(obj), obj); !apierrors.IsNotFound(err) {
			t.Fatalf("first-build artifact %T survived delete pass: %v", obj, err)
		}
	}

	// A fresh reconciler models manager restart: no in-memory cleanup state is
	// required; it inventories the build namespace again and releases only after
	// observing the old UID's artifacts absent.
	restarted := &AppReconciler{Client: cl, BuildClient: cl, BuildNamespace: "builds", Scheme: cl.Scheme(), Mode: ModeKubernetes}
	if _, err := restarted.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); !apierrors.IsNotFound(err) {
		t.Fatalf("App survived fresh absence proof after manager restart: %v", err)
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

func TestDeleteTLSSecretsUsesUncachedClientForTenantNamespace(t *testing.T) {
	ctx := context.Background()
	scheme := deletionScheme(t)
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: "tenant-tls", Namespace: "tea-test",
		Annotations: map[string]string{annotTLSSecretHistory: `["tenant-tls-tls"]`},
	}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tenant-tls-tls", Namespace: app.Namespace}}
	cached := fake.NewClientBuilder().WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
			if _, ok := list.(*corev1.SecretList); ok {
				return errors.New("unknown namespace for the cache")
			}
			return nil
		}}).Build()
	direct := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	r := &AppReconciler{Client: cached, BuildClient: direct, Scheme: scheme}

	done, err := r.deleteTLSSecrets(ctx, app)
	if err != nil || done {
		t.Fatalf("first TLS cleanup pass = done %v err %v, want pending delete", done, err)
	}
	done, err = r.deleteTLSSecrets(ctx, app)
	if err != nil || !done {
		t.Fatalf("second TLS cleanup pass = done %v err %v, want observed absence", done, err)
	}
	if err := direct.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("tenant TLS Secret survived direct-client cleanup: %v", err)
	}
}

func TestAppDeletionQuiescesIngressAndCertificateBeforeTLSSecret(t *testing.T) {
	app := deletionApp("tls-live")
	app.Annotations = map[string]string{annotTLSSecretHistory: `["tls-live-tls"]`}
	ingress := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	certificate := &unstructured.Unstructured{}
	certificate.SetGroupVersionKind(certManagerCertificateGVK)
	certificate.SetName("tls-live-tls")
	certificate.SetNamespace(app.Namespace)
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "tls-live-tls", Namespace: app.Namespace}}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, ingress, certificate, secret).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, ClusterIssuer: "letsencrypt-prod"}
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	req := reconcile.Request{NamespacedName: nn}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(ingress), &networkingv1.Ingress{}); !apierrors.IsNotFound(err) {
		t.Fatalf("Ingress survived first cleanup stage: %v", err)
	}
	firstCertificateProbe := &unstructured.Unstructured{}
	firstCertificateProbe.SetGroupVersionKind(certManagerCertificateGVK)
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(certificate), firstCertificateProbe); err != nil {
		t.Fatalf("Certificate removed before Ingress absence was observed: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(secret), &corev1.Secret{}); err != nil {
		t.Fatalf("TLS Secret removed while its producer was live: %v", err)
	}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	probeCertificate := &unstructured.Unstructured{}
	probeCertificate.SetGroupVersionKind(certManagerCertificateGVK)
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(certificate), probeCertificate); !apierrors.IsNotFound(err) {
		t.Fatalf("Certificate survived second cleanup stage: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(secret), &corev1.Secret{}); err != nil {
		t.Fatalf("TLS Secret removed before Certificate absence was observed: %v", err)
	}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(secret), &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("TLS Secret survived producer shutdown: %v", err)
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); err != nil {
		t.Fatal("finalizer released before TLS Secret absence was observed")
	}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); !apierrors.IsNotFound(err) {
		t.Fatalf("App survived ordered TLS teardown: %v", err)
	}
}

func TestStaticAppDeletionWaitsForPurgeCompletionAndJobAbsence(t *testing.T) {
	app := deletionApp("static-delete")
	app.Spec.Type = appv1alpha1.TypeStaticSite
	creds := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "static-creds", Namespace: app.Namespace}, Data: map[string][]byte{
		"AWS_ACCESS_KEY_ID":     []byte("test-access"),
		"AWS_SECRET_ACCESS_KEY": []byte("test-secret"),
	}}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, creds).
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
