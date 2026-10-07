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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
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

	"github.com/bex-co/bex/lego/operator/internal/identity"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPerAppRegistryCleanupCompletesAcrossTheCacheRename (w5/m135): the build
// cache moved from <repo>-cache, which an App named <name>-cache owns as its
// image repository, to <repo>_cache, and each App's credential pass gives its
// claim on the old name back. Zot honors a config only after it restarts and
// answers 403 for a repository the user holds no grant on, so App deletion
// must neither reach for the old name nor wait on a cache grant Zot has not
// honored yet. This fake Zot enforces the stored grants, as the real one does.
func TestPerAppRegistryCleanupCompletesAcrossTheCacheRename(t *testing.T) {
	const ws = "tea-aaaaaaaaaaaaaaaaaaaa"
	for _, tc := range []struct {
		name          string
		sibling       bool
		beforeRestart bool // Zot still runs the config from before this App's first pass since the rename
	}{
		{name: "after a zot restart, alone"},
		{name: "after a zot restart, beside an App named web-cache", sibling: true},
		{name: "before zot restarts", beforeRestart: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			var (
				mu      sync.Mutex
				cl      client.Client
				running map[string]any // nil: Zot runs the stored config
				denied  []string
				touched []string
			)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, _, ok := r.BasicAuth()
				if !ok {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				mu.Lock()
				defer mu.Unlock()
				config := running
				if config == nil {
					config = storedZotConfig(t, cl)
				}
				repo := requestRepo(r.URL.Path)
				touched = append(touched, repo)
				if !zotAllows(config, user, repo, map[bool]string{true: "delete", false: "read"}[r.Method == http.MethodDelete]) {
					denied = append(denied, repo)
					w.WriteHeader(http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusNotFound) // nothing was pushed
			}))
			defer server.Close()

			app := deletionApp("web")
			app.Labels = map[string]string{labelWorkspace: ws}
			app.Spec.Repo = "https://example.invalid/acme/web.git"
			objs := []client.Object{app, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: "zot"}, Data: map[string][]byte{"htpasswd": {}}}}
			if tc.sibling {
				objs = append(objs, &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web-cache", Namespace: "default", Labels: map[string]string{labelWorkspace: ws}}})
			}
			cl = fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(objs...).WithStatusSubresource(&appv1alpha1.App{}).Build()
			creds := &registry.Creds{Client: cl, ZotNamespace: "zot", HTPasswdName: "zot-htpasswd", ConfigName: "zot-config", Registry: server.URL, HTTPClient: server.Client()}
			id := identity.ForApp("web", ws)
			if err := creds.EnsureCredsFor(ctx, id, app.Namespace); err != nil {
				t.Fatal(err)
			}
			if tc.beforeRestart {
				// What Zot loaded before the rename: the App's grant on the old
				// name, none yet on the new one.
				running = storedZotConfig(t, cl)
				repos := running["http"].(map[string]any)["accessControl"].(map[string]any)["repositories"].(map[string]any)
				repos[id.PriorCacheRepo()] = repos[id.CacheRepo()]
				delete(repos, id.CacheRepo())
			}
			r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, Registry: server.URL, PerAppRegistry: creds, HTTPClient: server.Client()}
			if done, err := r.deleteRegistryRepo(ctx, app); err != nil || !done {
				t.Fatalf("cleanup = done %v err %v, want it to complete (denied: %v)", done, err, denied)
			}
			if slices.Contains(touched, id.PriorCacheRepo()) {
				t.Errorf("cleanup reached %s, the cache's prior name, which web gave back and web-cache may own", id.PriorCacheRepo())
			}
		})
	}
}

// storedZotConfig is the zot-config document Creds keeps.
func storedZotConfig(t *testing.T, cl client.Client) map[string]any {
	t.Helper()
	var secret corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "zot", Name: "zot-config"}, &secret); err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(secret.Data["config.json"], &config); err != nil {
		t.Fatal(err)
	}
	return config
}

// zotAllows decides as Zot does: the admin policy, then the longest matching
// repository rule ("**" or the exact name), its policies, then its default.
func zotAllows(config map[string]any, user, repo, action string) bool {
	has := func(list any, s string) bool { items, _ := list.([]any); return slices.Contains(items, any(s)) }
	access, _ := config["http"].(map[string]any)["accessControl"].(map[string]any)
	if admin, ok := access["adminPolicy"].(map[string]any); ok && has(admin["users"], user) && has(admin["actions"], action) {
		return true
	}
	rules, _ := access["repositories"].(map[string]any)
	match := ""
	for pattern := range rules {
		if (pattern == "**" || pattern == repo) && len(pattern) > len(match) {
			match = pattern
		}
	}
	rule, _ := rules[match].(map[string]any)
	for _, p := range rule["policies"].([]any) {
		policy, _ := p.(map[string]any)
		if has(policy["users"], user) && has(policy["actions"], action) {
			return true
		}
	}
	return has(rule["defaultPolicy"], action)
}

// requestRepo is the repository a registry API path names.
func requestRepo(path string) string {
	repo := strings.TrimPrefix(path, "/v2/")
	for _, suffix := range []string{"/tags/list", "/manifests/"} {
		if before, _, found := strings.Cut(repo, suffix); found {
			return before
		}
	}
	return repo
}

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

func TestRegistryCleanupLabeledAppDoesNotDeleteLegacyRepo(t *testing.T) {
	var hits []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	r := &AppReconciler{Registry: server.URL, HTTPClient: server.Client()}
	app := &appv1alpha1.App{}
	app.Name = "web"
	app.Labels = map[string]string{labelWorkspace: "tea-aaaaaaaaaaaaaaaaaaaa"}
	if done, err := r.deleteRegistryRepo(context.Background(), app); err != nil || !done {
		t.Fatalf("scoped empty-repo pass = done %v err %v", done, err)
	}
	joined := strings.Join(hits, "\n")
	if !strings.Contains(joined, "/v2/tea-aaaaaaaaaaaaaaaaaaaa/web/tags/list") {
		t.Fatalf("labeled App missed scoped repo: %s", joined)
	}
	if strings.Contains(joined, "/v2/web/tags/list") {
		t.Fatalf("labeled non-tombstoned App hit a legacy same-named sibling repo: %s", joined)
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

func TestRegistryCleanupDeletesTheBuildCacheRepository(t *testing.T) {
	// The cache is a separate repository holding several times the image's bytes
	// (docs/ADR060 D3), and this same teardown revokes its ACL entry — so a cache
	// left behind is both unowned and unreachable, which is a storage leak nobody
	// can find later. It also has to be reclaimed for an App whose cache was
	// built while BEX_BUILD_CACHE was on and deleted after it was turned off,
	// which is why deletion is unconditional rather than gated.
	var hits []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	r := &AppReconciler{Registry: server.URL, HTTPClient: server.Client()}
	app := &appv1alpha1.App{}
	app.Name = "web"
	app.Labels = map[string]string{labelWorkspace: "tea-aaaaaaaaaaaaaaaaaaaa"}
	if done, err := r.deleteRegistryRepo(context.Background(), app); err != nil || !done {
		t.Fatalf("cleanup pass = done %v err %v", done, err)
	}
	joined := strings.Join(hits, "\n")
	for _, want := range []string{
		"/v2/tea-aaaaaaaaaaaaaaaaaaaa/web/tags/list",
		"/v2/tea-aaaaaaaaaaaaaaaaaaaa/web_cache/tags/list",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("teardown never touched %s:\n%s", want, joined)
		}
	}
}

func TestRegistryCleanupAbsentCacheRepositoryStillCompletes(t *testing.T) {
	// The common case once the gate is off: the image repository exists, the
	// cache never did. A 404 there must read as "nothing to reclaim" rather than
	// stalling the finalizer forever on an App that has no cache.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v2/web/tags/list" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tags":[]}`))
			return
		}
		http.NotFound(w, r) // every cache request 404s
	}))
	defer server.Close()
	r := &AppReconciler{Registry: server.URL, HTTPClient: server.Client()}
	app := &appv1alpha1.App{}
	app.Name = "web"
	if done, err := r.deleteRegistryRepo(context.Background(), app); err != nil || !done {
		t.Fatalf("absent-cache pass = done %v err %v; the finalizer would never clear", done, err)
	}
}
