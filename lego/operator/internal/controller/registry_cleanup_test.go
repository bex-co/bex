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
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRegistryCleanupDeletesThenProvesRepositoryEmpty(t *testing.T) {
	var deleted atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/tags/list"):
			w.Header().Set("Content-Type", "application/json")
			if deleted.Load() {
				_, _ = w.Write([]byte(`{"tags":[]}`))
			} else {
				_, _ = w.Write([]byte(`{"tags":["gen-1","gen-2"]}`))
			}
		case r.Method == http.MethodHead:
			w.Header().Set("Docker-Content-Digest", "sha256:abc")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete:
			deleted.Store(true)
			w.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	r := &AppReconciler{Registry: server.URL, HTTPClient: server.Client()}
	app := &appv1alpha1.App{}
	app.Name = "web"
	if done, err := r.deleteRegistryRepo(context.Background(), app); err != nil || done {
		t.Fatalf("delete pass = done %v err %v", done, err)
	}
	if done, err := r.deleteRegistryRepo(context.Background(), app); err != nil || !done {
		t.Fatalf("absence pass = done %v err %v", done, err)
	}
}

func TestRegistryCleanupHonorsCallerCancellationDuringBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	r := &AppReconciler{Registry: server.URL, HTTPClient: server.Client()}
	app := &appv1alpha1.App{}
	app.Name = "web"
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := r.deleteRegistryRepo(ctx, app)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("stalled registry body err=%v elapsed=%s", err, time.Since(started))
	}
}

func TestRegistryCleanupConfiguredMissingPushSecretIsNotAnonymous(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).Build()
	r := &AppReconciler{
		Client: cl, Registry: server.URL, RegistryPushSecret: "configured-but-missing",
		HTTPClient: server.Client(),
	}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "apps"}}
	if _, err := r.deleteRegistryRepo(context.Background(), app); err == nil || !strings.Contains(err.Error(), "configured-but-missing") {
		t.Fatalf("missing configured auth error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("missing configured auth sent %d anonymous registry request(s)", requests.Load())
	}
}

func TestRegistryCleanupFailurePreservesPerAppCredentialAndFinalizer(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := r.BasicAuth()
		if !ok || user != registry.ZotUsername("web") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if requests.Add(1) == 1 {
			// EnsureActive's least-privilege probe succeeds; the subsequent real
			// cleanup request is the transient failure under test.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":[]}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	app := deletionApp("web")
	app.Spec.Repo = "https://github.com/acme/web"
	htpasswd := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "zot"}, Data: map[string][]byte{"htpasswd": {}}}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, htpasswd).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	creds := &registry.Creds{
		Client: cl, ZotNamespace: "zot", HTPasswdName: "zot-htpasswd", ConfigName: "zot-config",
		Registry: server.URL, HTTPClient: server.Client(),
	}
	if err := creds.EnsureCreds(context.Background(), app.Name, app.Namespace); err != nil {
		t.Fatal(err)
	}
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, Registry: server.URL, PerAppRegistry: creds, HTTPClient: server.Client()}
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); err == nil {
		t.Fatal("transient registry cleanup failure was swallowed")
	}

	var current appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &current); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(current.Finalizers, finalizer) {
		t.Fatal("registry failure released the App finalizer")
	}
	var pull corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: app.Namespace, Name: registry.PullSecretName(app.Name)}, &pull); err != nil {
		t.Fatalf("registry failure revoked the retry credential: %v", err)
	}
}

func TestRegistryCleanupRestoresRevokedCredentialThenConverges(t *testing.T) {
	var deleted atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := r.BasicAuth()
		if !ok || user != registry.ZotUsername("web") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/tags/list"):
			w.Header().Set("Content-Type", "application/json")
			if deleted.Load() {
				_, _ = w.Write([]byte(`{"tags":[]}`))
			} else {
				_, _ = w.Write([]byte(`{"tags":["gen-1"]}`))
			}
		case r.Method == http.MethodHead:
			w.Header().Set("Docker-Content-Digest", "sha256:abc")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete:
			deleted.Store(true)
			w.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := deletionApp("web")
	app.Spec.Repo = "https://github.com/acme/web"
	htpasswd := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "zot"}, Data: map[string][]byte{"htpasswd": {}}}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app, htpasswd).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	creds := &registry.Creds{
		Client: cl, ZotNamespace: "zot", HTPasswdName: "zot-htpasswd", ConfigName: "zot-config",
		Registry: server.URL, HTTPClient: server.Client(),
	}
	if err := creds.EnsureCreds(context.Background(), app.Name, app.Namespace); err != nil {
		t.Fatal(err)
	}
	// Reproduce the old finalizer's destructive ordering: Zot ACL/htpasswd and
	// the App pull Secret were revoked before repository absence was proven.
	if err := creds.RevokeCreds(context.Background(), app.Name); err != nil {
		t.Fatal(err)
	}
	var oldPull corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: app.Namespace, Name: registry.PullSecretName(app.Name)}, &oldPull); err != nil {
		t.Fatal(err)
	}
	if err := cl.Delete(context.Background(), &oldPull); err != nil {
		t.Fatal(err)
	}

	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, Registry: server.URL, PerAppRegistry: creds, HTTPClient: server.Client()}
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	req := reconcile.Request{NamespacedName: nn}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if !deleted.Load() {
		t.Fatal("authenticated finalizer did not delete the registry manifest")
	}
	var restored corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: app.Namespace, Name: registry.PullSecretName(app.Name)}, &restored); err != nil {
		t.Fatalf("credential was not restored or was revoked while registry cleanup remained pending: %v", err)
	}
	// Lose all reconciler/credential-manager memory before the absence pass.
	// Durable Kubernetes/registry state, not memoized activation, must converge.
	restartedCreds := &registry.Creds{
		Client: cl, ZotNamespace: "zot", HTPasswdName: "zot-htpasswd", ConfigName: "zot-config",
		Registry: server.URL, HTTPClient: server.Client(),
	}
	r = &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, Registry: server.URL, PerAppRegistry: restartedCreds, HTTPClient: server.Client()}

	for range 4 {
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		var current appv1alpha1.App
		if err := cl.Get(context.Background(), nn, &current); apierrors.IsNotFound(err) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if err := cl.Get(context.Background(), nn, &appv1alpha1.App{}); !apierrors.IsNotFound(err) {
		t.Fatalf("App finalizer did not converge after registry absence proof: %v", err)
	}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: app.Namespace, Name: registry.PullSecretName(app.Name)}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("restored credential survived finalization: %v", err)
	}
}
