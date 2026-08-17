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

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/types/netutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func maintenanceApp(uri string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "bex-system"},
		Spec: appv1alpha1.AppSpec{
			MaintenanceMode: &appv1alpha1.MaintenanceModeSpec{Enabled: true, URI: uri},
			Hosts:           []string{"custom.example.com"},
		},
		Status: appv1alpha1.AppStatus{
			URL:  "https://web.onbex.co",
			URLs: []string{"https://web.onbex.co", "https://custom.example.com"},
		},
	}
}

func TestMaintenanceHandlerDoesNotWakeOrMutateWorkload(t *testing.T) {
	app := maintenanceApp("")
	zero := int32(0)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "bex-system"},
		Spec:       appsv1.DeploymentSpec{Replicas: &zero},
	}
	cache, cl := primedHostCache(t, app, dep)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://web.onbex.co/arbitrary/path", strings.NewReader("ignored"))
	newHandler(cl, cache, logr.Discard()).ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "currently under maintenance") {
		t.Fatalf("response = %d %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	var got appsv1.Deployment
	if err := cl.Get(context.Background(), clientKey("web"), &got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.Replicas == nil || *got.Spec.Replicas != 0 {
		t.Fatalf("maintenance request woke Deployment: replicas=%v", got.Spec.Replicas)
	}
	var gotApp appv1alpha1.App
	if err := cl.Get(context.Background(), clientKey("web"), &gotApp); err != nil {
		t.Fatal(err)
	}
	if gotApp.Annotations[annotLastActive] != "" {
		t.Fatalf("maintenance request touched last-active: %#v", gotApp.Annotations)
	}
}

func TestWakeHandlerNegotiatesContentAndAlwaysWakes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		accept   string
		wantType string
		wantBody string
	}{
		{
			name:     "browser gets polling interstitial",
			accept:   "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			wantType: "text/html; charset=utf-8",
			wantBody: "Application loading",
		},
		{
			name:     "API gets retryable JSON",
			accept:   "application/json",
			wantType: "application/json",
			wantBody: `{"error":"service hibernated","retryAfter":5}`,
		},
		{
			name:     "wildcard stays on API default",
			accept:   "*/*",
			wantType: "application/json",
			wantBody: `{"error":"service hibernated","retryAfter":5}`,
		},
		{
			name:     "explicit HTML refusal stays on API default",
			accept:   "text/html;q=0,*/*;q=0.8",
			wantType: "application/json",
			wantBody: `{"error":"service hibernated","retryAfter":5}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zero := int32(0)
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "sleeping", Namespace: "bex-system"},
				Status:     appv1alpha1.AppStatus{URL: "https://sleeping.onbex.co"},
			}
			dep := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "sleeping", Namespace: "bex-system"},
				Spec:       appsv1.DeploymentSpec{Replicas: &zero},
			}
			cache, cl := primedHostCache(t, app, dep)

			req := httptest.NewRequest(http.MethodGet, "https://sleeping.onbex.co/ready", nil)
			req.Header.Set("Accept", tc.accept)
			rr := httptest.NewRecorder()
			newHandler(cl, cache, logr.Discard()).ServeHTTP(rr, req)

			if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") != "5" {
				t.Fatalf("response = %d, Retry-After %q", rr.Code, rr.Header().Get("Retry-After"))
			}
			if got := rr.Header().Get("Content-Type"); got != tc.wantType {
				t.Fatalf("Content-Type = %q, want %q", got, tc.wantType)
			}
			if got := rr.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", got)
			}
			if !strings.Contains(rr.Body.String(), tc.wantBody) {
				t.Fatalf("body = %q, want substring %q", rr.Body.String(), tc.wantBody)
			}
			if tc.wantType == "text/html; charset=utf-8" {
				for _, want := range []string{
					"probeIntervalMs = 5000",
					"fallbackReloadMs = 45000",
					"method: \"HEAD\"",
				} {
					if !strings.Contains(rr.Body.String(), want) {
						t.Errorf("wake page missing %q", want)
					}
				}
			}

			var gotDep appsv1.Deployment
			if err := cl.Get(context.Background(), clientKey("sleeping"), &gotDep); err != nil {
				t.Fatal(err)
			}
			if gotDep.Spec.Replicas == nil || *gotDep.Spec.Replicas != 1 {
				t.Fatalf("wake replicas = %v, want 1", gotDep.Spec.Replicas)
			}
			var gotApp appv1alpha1.App
			if err := cl.Get(context.Background(), clientKey("sleeping"), &gotApp); err != nil {
				t.Fatal(err)
			}
			if gotApp.Annotations[annotLastActive] == "" {
				t.Fatalf("wake did not stamp %s: %#v", annotLastActive, gotApp.Annotations)
			}
		})
	}
}

func TestWakeHTMLHeadHasNoBody(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "https://sleeping.onbex.co/", nil)
	req.Header.Set("Accept", "text/html")
	writeWakeResponse(rr, req)
	if rr.Code != http.StatusServiceUnavailable || rr.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d %q", rr.Code, rr.Body.String())
	}
}

func TestMaintenanceHandlerCoversPlatformAndCustomHostsOnEveryMethod(t *testing.T) {
	app := maintenanceApp("")
	cache, cl := primedHostCache(t, app)
	handler := newHandler(cl, cache, logr.Discard())

	for _, tc := range []struct {
		host   string
		method string
		path   string
	}{
		{host: "web.onbex.co", method: http.MethodGet, path: "/"},
		{host: "custom.example.com", method: http.MethodPost, path: "/api/orders"},
		{host: "web.onbex.co", method: http.MethodPut, path: "/deep/path?x=1"},
		{host: "custom.example.com", method: http.MethodDelete, path: "/anything"},
	} {
		t.Run(tc.method+" "+tc.host+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "https://"+tc.host+tc.path, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "currently under maintenance") {
				t.Fatalf("response = %d %q", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestCustomMaintenancePageSuccessAndOriginError(t *testing.T) {
	app := maintenanceApp("https://status.example.com/page")
	for _, tc := range []struct {
		name       string
		originCode int
		wantCode   int
	}{
		{name: "success becomes maintenance 503", originCode: http.StatusOK, wantCode: http.StatusServiceUnavailable},
		{name: "origin error passes through", originCode: http.StatusTeapot, wantCode: http.StatusTeapot},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fetch := func(context.Context, *appv1alpha1.App, string) (*http.Response, error) {
				return &http.Response{
					StatusCode: tc.originCode,
					Header:     http.Header{"Content-Type": []string{"text/html"}},
					Body:       io.NopCloser(strings.NewReader("custom body")),
				}, nil
			}
			rr := httptest.NewRecorder()
			serveMaintenanceWithFetcher(rr, httptest.NewRequest(http.MethodGet, "/anything", nil), app, fetch)
			if rr.Code != tc.wantCode || rr.Body.String() != "custom body" || rr.Header().Get("Content-Type") != "text/html" {
				t.Fatalf("response = %d %q %#v", rr.Code, rr.Body.String(), rr.Header())
			}
		})
	}

	rr := httptest.NewRecorder()
	serveMaintenanceWithFetcher(rr, httptest.NewRequest(http.MethodGet, "/", nil), app,
		func(context.Context, *appv1alpha1.App, string) (*http.Response, error) {
			return nil, errors.New("timeout")
		})
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("fetch failure status = %d", rr.Code)
	}
}

func TestCustomMaintenancePageHeadAndSizeLimit(t *testing.T) {
	app := maintenanceApp("https://status.example.com/page")
	fetchBody := func(body string) func(context.Context, *appv1alpha1.App, string) (*http.Response, error) {
		return func(context.Context, *appv1alpha1.App, string) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/plain"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}
	}

	head := httptest.NewRecorder()
	headReq := httptest.NewRequest(http.MethodHead, "/any", nil)
	serveMaintenanceWithFetcher(head, headReq, app, fetchBody("must not be written"))
	if head.Code != http.StatusServiceUnavailable || head.Body.Len() != 0 {
		t.Fatalf("HEAD response = %d %q", head.Code, head.Body.String())
	}

	oversized := httptest.NewRecorder()
	oversizedReq := httptest.NewRequest(http.MethodGet, "/", nil)
	oversizedBody := fetchBody(strings.Repeat("x", customPageMaxSize+1))
	serveMaintenanceWithFetcher(oversized, oversizedReq, app, oversizedBody)
	if oversized.Code != http.StatusBadGateway || !strings.Contains(oversized.Body.String(), "unavailable") {
		t.Fatalf("oversized response = %d %q", oversized.Code, oversized.Body.String())
	}
}

func TestMaintenanceURLSafety(t *testing.T) {
	app := maintenanceApp("")
	for _, raw := range []string{
		"/relative",
		"ftp://example.com/page",
		"https://user:password@status.example.com/page",
		"https://web.onbex.co/page",
		"https://custom.example.com/page",
	} {
		if _, err := validateCustomPageURL(raw, app, nil); err == nil {
			t.Errorf("validateCustomPageURL(%q) succeeded", raw)
		}
	}
	for _, raw := range []string{"https://status.example.com/page", "http://status.example.com"} {
		if _, err := validateCustomPageURL(raw, app, nil); err != nil {
			t.Errorf("validateCustomPageURL(%q): %v", raw, err)
		}
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.1.1", "::1"} {
		if !netutil.UnsafeOriginIP(net.ParseIP(raw)) {
			t.Errorf("netutil.UnsafeOriginIP(%s) = false", raw)
		}
	}
	dial := netutil.SafeDialContext(customPageTimeout)
	_, err := dial(context.Background(), "tcp", "127.0.0.1:80")
	if err == nil || !strings.Contains(err.Error(), "blocked address") {
		t.Fatalf("netutil.SafeDialContext(loopback) error = %v", err)
	}
}

func TestMaintenanceRedirectPolicyRejectsSelfAndLongChains(t *testing.T) {
	app := maintenanceApp("")
	policy := customPageRedirectPolicy(app, nil)
	self := httptest.NewRequest(http.MethodGet, "https://web.onbex.co/redirected", nil)
	selfVia := []*http.Request{httptest.NewRequest(http.MethodGet, "https://status.example.com", nil)}
	if err := policy(self, selfVia); err == nil || !strings.Contains(err.Error(), "this service") {
		t.Fatalf("redirect-to-self error = %v", err)
	}

	external := httptest.NewRequest(http.MethodGet, "https://status.example.com/final", nil)
	via := make([]*http.Request, maxRedirects+1)
	for i := range via {
		via[i] = httptest.NewRequest(http.MethodGet, "https://status.example.com/hop", nil)
	}
	if err := policy(external, via); err == nil || !strings.Contains(err.Error(), "too many redirects") {
		t.Fatalf("long redirect chain error = %v", err)
	}
}

// TestMaintenanceURLRejectsOtherAppHosts is the cross-service recursion guard:
// a custom maintenance page URL whose host belongs to ANY App routed through
// the platform — not just the App itself — must be refused, so two services
// cannot point their maintenance pages at each other and close an amplifying
// synchronous-fetch cycle through this shared activator. The denylist is the
// host cache's cluster-wide host map, so it covers Apps in every workspace's
// namespace.
func TestMaintenanceURLRejectsOtherAppHosts(t *testing.T) {
	appA := maintenanceApp("")
	appB := &appv1alpha1.App{ // another workspace's namespace
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "tenant-b"},
		Spec:       appv1alpha1.AppSpec{Hosts: []string{"b.example.com"}},
		Status:     appv1alpha1.AppStatus{URL: "https://other.onbex.co"},
	}
	cache, _ := primedHostCache(t, appA, appB)

	for _, raw := range []string{
		"https://b.example.com/page",  // B's custom host (the A->B half of a cycle)
		"https://B.Example.com/page",  // case variant of the same host
		"https://b.example.com./page", // trailing-dot spelling of the same host
		"https://other.onbex.co/page", // B's platform host
	} {
		if _, err := validateCustomPageURL(raw, appA, cache.claims); err == nil {
			t.Errorf("validateCustomPageURL(%q) succeeded — cross-service cycle admitted", raw)
		}
	}
	// A genuinely external page is still allowed.
	if _, err := validateCustomPageURL("https://status.example.com/page", appA, cache.claims); err != nil {
		t.Errorf("external maintenance page rejected: %v", err)
	}

	// End to end: A in maintenance mode pointing at B's host must answer 502
	// from validation, never a live fetch.
	cyclic := maintenanceApp("https://b.example.com/page")
	cache, cl := primedHostCache(t, cyclic, appB)
	handler := newHandler(cl, cache, logr.Discard())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "https://web.onbex.co/", nil))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("cyclic maintenance page response = %d, want 502 (rejected before fetch)", rr.Code)
	}
}

func TestHostCacheCoversSpecAndStatusHosts(t *testing.T) {
	app := maintenanceApp("")
	app.Spec.Host = "legacy.example.com"
	cache, _ := primedHostCache(t, app)
	for _, host := range []string{"web.onbex.co", "custom.example.com", "legacy.example.com"} {
		got, ok := cache.lookup(host)
		if !ok || got == nil || got.Name != "web" {
			t.Fatalf("cache.lookup(%q) = %+v, ok=%v", host, got, ok)
		}
	}
	// An unknown host does not resolve, and lookup never hits the API.
	if got, ok := cache.lookup("unknown.example.com"); ok || got != nil {
		t.Fatalf("cache.lookup(unknown) = %+v, ok=%v; want not found", got, ok)
	}
}

// sleepingApp is a hibernated (replicas 0) App reachable on several hosts, so a
// lookup for any of them resolves to the same cached object — the exact shape
// that let the pre-fix code hand a shared *App to concurrent wake goroutines.
func sleepingApp() (*appv1alpha1.App, *appsv1.Deployment) {
	zero := int32(0)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "sleeping", Namespace: "bex-system"},
		Spec:       appv1alpha1.AppSpec{Hosts: []string{"a.onbex.co", "b.onbex.co"}},
		Status: appv1alpha1.AppStatus{
			URL:  "https://sleeping.onbex.co",
			URLs: []string{"https://sleeping.onbex.co", "https://a.onbex.co", "https://b.onbex.co"},
		},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "sleeping", Namespace: "bex-system"},
		Spec:       appsv1.DeploymentSpec{Replicas: &zero},
	}
	return app, dep
}

// TestWakeUnderConcurrentRequestsIsRaceFree floods the same sleeping App from
// many goroutines at once. Pre-fix, hostCache.lookup handed every request the
// cache-owned *App and wakeApp mutated its annotations map directly, so this
// raced ("concurrent map writes", a fatal crash of the single replica). It must
// now run clean under -race and still wake the workload.
func TestWakeUnderConcurrentRequestsIsRaceFree(t *testing.T) {
	app, dep := sleepingApp()
	cache, cl := primedHostCache(t, app, dep)
	handler := newHandler(cl, cache, logr.Discard())

	hosts := []string{"sleeping.onbex.co", "a.onbex.co", "b.onbex.co"}
	var wg sync.WaitGroup
	for i := range 300 {
		host := hosts[i%len(hosts)]
		wg.Go(func() {
			req := httptest.NewRequest(http.MethodGet, "https://"+host+"/", nil)
			req.Host = host
			handler.ServeHTTP(httptest.NewRecorder(), req)
		})
	}
	wg.Wait()

	var gotDep appsv1.Deployment
	if err := cl.Get(context.Background(), clientKey("sleeping"), &gotDep); err != nil {
		t.Fatal(err)
	}
	if gotDep.Spec.Replicas == nil || *gotDep.Spec.Replicas != 1 {
		t.Fatalf("wake replicas = %v, want 1", gotDep.Spec.Replicas)
	}
}

// blockingClient counts App/Deployment patches and holds the App patch open until
// released, so a test can force many concurrent wakes to overlap inside one
// singleflight window and prove they coalesce into a single patch.
type blockingClient struct {
	client.Client
	appPatches  atomic.Int64
	depPatches  atomic.Int64
	entered     chan struct{} // closed when the leader is inside the App patch
	release     chan struct{} // the leader blocks here until the test closes it
	enteredOnce sync.Once
}

func (b *blockingClient) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption,
) error {
	switch obj.(type) {
	case *appv1alpha1.App:
		b.appPatches.Add(1)
		b.enteredOnce.Do(func() { close(b.entered) })
		<-b.release
	case *appsv1.Deployment:
		b.depPatches.Add(1)
	}
	return b.Client.Patch(ctx, obj, patch, opts...)
}

// TestWakeCoalescesConcurrentRequestsIntoOnePatch asserts the singleflight seam:
// N simultaneous requests to one sleeping App issue exactly one last-active patch
// and one scale-up, not N (finding 5 — otherwise an unauthenticated flood
// stampedes the API server).
func TestWakeCoalescesConcurrentRequestsIntoOnePatch(t *testing.T) {
	app, dep := sleepingApp()
	cache, base := primedHostCache(t, app, dep)
	bc := &blockingClient{Client: base, entered: make(chan struct{}), release: make(chan struct{})}
	handler := newHandler(bc, cache, logr.Discard())

	const n = 64
	var reached, done sync.WaitGroup
	reached.Add(n)
	done.Add(n)
	for range n {
		go func() {
			defer done.Done()
			req := httptest.NewRequest(http.MethodGet, "https://sleeping.onbex.co/", nil)
			reached.Done()
			handler.ServeHTTP(httptest.NewRecorder(), req)
		}()
	}

	reached.Wait() // every goroutine is about to enter the handler
	<-bc.entered   // the leader is inside the App patch, holding the flight open
	// The leader holds the singleflight entry open, so every other request can
	// only join it as a waiter. A brief settle covers the sub-microsecond gap
	// between reached.Done and singleflight.Do for the stragglers.
	time.Sleep(100 * time.Millisecond)
	close(bc.release) // the leader completes; waiters share its result
	done.Wait()

	if got := bc.appPatches.Load(); got != 1 {
		t.Fatalf("app patches = %d, want exactly 1 (coalesced)", got)
	}
	if got := bc.depPatches.Load(); got != 1 {
		t.Fatalf("deployment patches = %d, want exactly 1 (coalesced)", got)
	}
	var gotApp appv1alpha1.App
	if err := base.Get(context.Background(), clientKey("sleeping"), &gotApp); err != nil {
		t.Fatal(err)
	}
	if gotApp.Annotations[annotLastActive] == "" {
		t.Fatal("coalesced wake did not stamp last-active")
	}
}

// primedHostCache builds a hostCache over a fake client preloaded with objs and
// refreshes it once, mirroring how main primes the cache before serving.
func primedHostCache(t *testing.T, objs ...client.Object) (*hostCache, client.Client) {
	t.Helper()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	cache := newHostCache(cl, logr.Discard())
	cache.refreshOnce(context.Background())
	return cache, cl
}

func clientKey(name string) client.ObjectKey {
	return client.ObjectKey{Name: name, Namespace: "bex-system"}
}
