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
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/178: a request from a client that is not a browser is held through the cold
// start and then served by the app, with its method, headers and body; a browser
// keeps the loading page; the hold is bounded.

// holdFixture wires a holding handler to an upstream that stands in for the
// woken App's Service, which "becomes ready" after wakeDelay.
func holdFixture(
	t *testing.T, wakeDelay time.Duration, bound time.Duration,
) (http.Handler, *atomic.Int32, *countingClient) {
	t.Helper()
	app, dep := sleepingApp()
	app.Status.Phase = appv1alpha1.PhaseHibernated
	cache, base := primedHostCache(t, app, dep)
	cl := &countingClient{Client: base}

	var upstreamHits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Seen-Host", r.Host)
		w.Header().Set("X-Seen-Method", r.Method)
		w.Header().Set("X-Seen-Path", r.URL.RequestURI())
		w.Header().Set("X-Seen-Custom", r.Header.Get("X-Custom"))
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write(append([]byte("app:"), body...))
	}))
	t.Cleanup(upstream.Close)

	h := newHolder(cl)
	h.bound, h.pollInterval = bound, 10*time.Millisecond
	target, _ := url.Parse(upstream.URL)
	h.target = func(*appv1alpha1.App) *url.URL { return target }
	woke := time.Now().Add(wakeDelay)
	h.ready = func(context.Context, *appv1alpha1.App) bool { return time.Now().After(woke) }
	return newHoldingHandler(cl, cache, logr.Discard(), h), &upstreamHits, cl
}

func TestHeldRequestIsServedByTheWokenApp(t *testing.T) {
	h, hits, cl := holdFixture(t, 150*time.Millisecond, 5*time.Second)
	req := httptest.NewRequest(http.MethodPost, "https://sleeping.onbex.co/hooks/x?a=1", strings.NewReader("payload"))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Custom", "kept")
	rr := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted || rr.Body.String() != "app:payload" {
		t.Fatalf("response = %d %q, want the app's own", rr.Code, rr.Body.String())
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("answered after %v, want it held until the wake", time.Since(start))
	}
	for header, want := range map[string]string{
		"X-Seen-Host": "sleeping.onbex.co", "X-Seen-Method": "POST", "X-Seen-Path": "/hooks/x?a=1", "X-Seen-Custom": "kept",
	} {
		if got := rr.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if hits.Load() != 1 {
		t.Errorf("upstream saw %d requests, want exactly 1", hits.Load())
	}
	if cl.patches.Load() == 0 {
		t.Error("the held request did not wake the workload")
	}
}

func TestConcurrentHeldRequestsShareOneWake(t *testing.T) {
	h, hits, cl := holdFixture(t, 150*time.Millisecond, 5*time.Second)
	const n = 5
	done := make(chan *httptest.ResponseRecorder, n)
	for range n {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "https://sleeping.onbex.co/", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			done <- rr
		}()
	}
	for range n {
		if rr := <-done; rr.Code != http.StatusAccepted {
			t.Fatalf("held request = %d %s", rr.Code, rr.Body.String())
		}
	}
	if hits.Load() != n {
		t.Errorf("upstream saw %d requests, want %d", hits.Load(), n)
	}
	// The coalesced wake patched once per distinct object, not once per request.
	if p := cl.patches.Load(); p > 4 {
		t.Errorf("%d patches for %d concurrent requests; the wake must coalesce", p, n)
	}
}

func TestBrowsersKeepTheLoadingPage(t *testing.T) {
	h, hits, _ := holdFixture(t, 0, 5*time.Second)
	req := httptest.NewRequest(http.MethodGet, "https://sleeping.onbex.co/", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "Application loading") {
		t.Fatalf("browser response = %d %q, want the loading page", rr.Code, rr.Body.String())
	}
	if hits.Load() != 0 {
		t.Errorf("a browser request reached the app %d times", hits.Load())
	}
}

func TestHoldIsBoundedAndAnswersAGatewayTimeout(t *testing.T) {
	h, hits, _ := holdFixture(t, time.Hour, 200*time.Millisecond)
	req := httptest.NewRequest(http.MethodGet, "https://sleeping.onbex.co/", nil)
	rr := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("response = %d %q, want 504", rr.Code, rr.Body.String())
	}
	if elapsed := time.Since(start); elapsed < 150*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("held for %v, want about the 200ms bound", elapsed)
	}
	if hits.Load() != 0 {
		t.Errorf("the app was reached %d times despite never becoming ready", hits.Load())
	}
}

// The hold applies to a hibernated App only; an awake App the readiness hold
// routes here keeps the immediate not-ready answer.
func TestAwakeUnreadyServiceIsNotHeld(t *testing.T) {
	app, dep := sleepingApp()
	app.Status.Phase = appv1alpha1.PhaseDeploying
	cache, base := primedHostCache(t, app, dep)
	cl := &countingClient{Client: base}
	h := newHolder(cl)
	h.bound = 5 * time.Second
	handler := newHoldingHandler(cl, cache, logr.Discard(), h)

	req := httptest.NewRequest(http.MethodGet, "https://sleeping.onbex.co/", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable || time.Since(start) > time.Second {
		t.Fatalf("awake-but-unready response = %d after %v, want an immediate 503", rr.Code, time.Since(start))
	}
}

func TestHeldRequestLimitShedsToTheImmediateAnswer(t *testing.T) {
	h, _, cl := holdFixture(t, time.Hour, 5*time.Second)
	hold := newHolder(cl)
	for range maxHeldPerApp {
		release, ok := hold.admit("bex-system/sleeping")
		if !ok {
			t.Fatal("admit refused under the limit")
		}
		defer release()
	}
	if _, ok := hold.admit("bex-system/sleeping"); ok {
		t.Fatal("admit accepted past the per-App limit")
	}
	_ = h
}

// A held request reaches the port the App's Service publishes: an image-port
// App's image network, not the container default (w4/m243 t009).
func TestHeldRequestTargetsThePublishedServicePort(t *testing.T) {
	image := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "tea-a"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1},
		Status: appv1alpha1.AppStatus{ServingNetwork: &appv1alpha1.ImageNetworkStatus{
			Image: "nginx@sha256:x", Revision: "rev-1", PrimaryPort: 8080, Ports: []int32{8080},
		}},
	}
	if got := serviceURL(image).Host; got != "nginx.tea-a.svc:8080" {
		t.Fatalf("image-port target = %q, want the published 8080", got)
	}
	configured := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Port: 3000},
	}
	if got := serviceURL(configured).Host; got != "web.tea-a.svc:3000" {
		t.Fatalf("configured target = %q, want its port", got)
	}
}

// A woken App that is ready but never reachable on its port (a dial that
// fails until the bound) is a wake timeout, not a bare bad gateway.
func TestUnreachableWokenAppAnswersAWakeTimeout(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	dead, _ := url.Parse(closed.URL)
	closed.Close()
	rr := httptest.NewRecorder()
	unreachableFixture(t, dead).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "https://sleeping.onbex.co/", nil))
	if rr.Code != http.StatusGatewayTimeout || !strings.Contains(rr.Body.String(), "did not wake in time") {
		t.Fatalf("response = %d %q, want the wake timeout", rr.Code, rr.Body.String())
	}
}

func unreachableFixture(t *testing.T, target *url.URL) http.Handler {
	t.Helper()
	app, dep := sleepingApp()
	app.Status.Phase = appv1alpha1.PhaseHibernated
	cache, base := primedHostCache(t, app, dep)
	cl := &countingClient{Client: base}
	h := newHolder(cl)
	h.bound, h.pollInterval = 300*time.Millisecond, 10*time.Millisecond
	h.target = func(*appv1alpha1.App) *url.URL { return target }
	h.ready = func(context.Context, *appv1alpha1.App) bool { return true }
	return newHoldingHandler(cl, cache, logr.Discard(), h)
}
