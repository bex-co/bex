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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/staticserver"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	siteHost   = "site.onbex.co"
	sitePrefix = "ws/site/rev-1/"
	indexHTML  = "<!doctype html><title>site</title>"
	renderYAML = "services: []\n"
)

type hostResolver map[string]staticserver.Site

func (r hostResolver) Resolve(host string) (staticserver.Site, bool) {
	s, ok := r[host]
	return s, ok
}

// mapOrigin serves objects by key; a key in errs fails with that error.
type mapOrigin struct {
	objs map[string]string
	errs map[string]error
}

func (o mapOrigin) Get(_ context.Context, key string) (staticserver.Object, error) {
	if err, ok := o.errs[key]; ok {
		return staticserver.Object{}, err
	}
	body, ok := o.objs[key]
	if !ok {
		return staticserver.Object{}, staticserver.ErrNotFound
	}
	return staticserver.Object{Body: []byte(body)}, nil
}

// serveSite dispatches one request through the startup server construction
// around the real site handler — the layer where a reserved /healthz used to
// shadow the tenant (w4/m162).
func serveSite(site staticserver.Site, origin mapOrigin, method, target string) *httptest.ResponseRecorder {
	site.Prefix = sitePrefix
	h := staticserver.New(hostResolver{siteHost: site}, origin, 1<<20)
	rr := httptest.NewRecorder()
	newServer(":0", h).Handler.ServeHTTP(rr, httptest.NewRequest(method, "https://"+siteHost+target, nil))
	return rr
}

func publishedSite(extra map[string]string) mapOrigin {
	objs := map[string]string{
		sitePrefix + "index.html":  indexHTML,
		sitePrefix + "render.yaml": renderYAML,
	}
	for k, v := range extra {
		objs[sitePrefix+k] = v
	}
	return mapOrigin{objs: objs}
}

// TestHealthPathsReachTheSite: with no rules every health spelling takes the
// documented extensionless SPA fallback, exactly like the trailing-slash
// control; HEAD carries headers only.
func TestHealthPathsReachTheSite(t *testing.T) {
	for _, target := range []string{"/healthz", "/healthz?qa=r52", "/health%7a", "/healthz/"} {
		rr := serveSite(staticserver.Site{}, publishedSite(nil), http.MethodGet, target)
		if rr.Code != http.StatusOK || rr.Body.String() != indexHTML {
			t.Fatalf("GET %s = %d %q, want the site's index", target, rr.Code, rr.Body.String())
		}
		if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Fatalf("GET %s Content-Type = %q", target, got)
		}
	}
	rr := serveSite(staticserver.Site{}, publishedSite(nil), http.MethodHead, "/healthz")
	if rr.Code != http.StatusOK || rr.Body.Len() != 0 || rr.Header().Get("Content-Type") == "" {
		t.Fatalf("HEAD /healthz = %d %q %v", rr.Code, rr.Body.String(), rr.Header())
	}
}

// TestHealthPathRulesObjectsAndHeaders: a saved rewrite serves its target with
// the target's normal headers, a published /healthz object wins over it, and a
// custom header rule for the path applies.
func TestHealthPathRulesObjectsAndHeaders(t *testing.T) {
	rewrite := staticserver.Site{Routes: []appv1alpha1.StaticRoute{
		{Type: "rewrite", Source: "/healthz", Destination: "/render.yaml"},
	}}
	direct := serveSite(rewrite, publishedSite(nil), http.MethodGet, "/render.yaml")
	rr := serveSite(rewrite, publishedSite(nil), http.MethodGet, "/healthz")
	if rr.Code != http.StatusOK || rr.Body.String() != renderYAML {
		t.Fatalf("rewritten /healthz = %d %q, want the target bytes", rr.Code, rr.Body.String())
	}
	for _, h := range []string{"Content-Type", "Cache-Control"} {
		if rr.Header().Get(h) != direct.Header().Get(h) {
			t.Fatalf("rewritten /healthz %s = %q, direct target %q", h, rr.Header().Get(h), direct.Header().Get(h))
		}
	}

	rr = serveSite(rewrite, publishedSite(map[string]string{"healthz": "own"}), http.MethodGet, "/healthz")
	if rr.Code != http.StatusOK || rr.Body.String() != "own" {
		t.Fatalf("published /healthz = %d %q, want the object over the rule", rr.Code, rr.Body.String())
	}

	headers := staticserver.Site{Headers: []appv1alpha1.StaticHeader{{Path: "/healthz", Name: "X-Probe", Value: "tenant"}}}
	rr = serveSite(headers, publishedSite(nil), http.MethodGet, "/healthz")
	if got := rr.Header().Get("X-Probe"); got != "tenant" {
		t.Fatalf("custom header on /healthz = %q, want tenant", got)
	}
}

// TestHealthPathErrorsKeepTheSiteContract: origin failures, unsupported
// methods, unknown hosts and degraded mode answer /healthz as any other path.
func TestHealthPathErrorsKeepTheSiteContract(t *testing.T) {
	failing := publishedSite(nil)
	failing.errs = map[string]error{sitePrefix + "healthz": errors.New("boom")}
	if rr := serveSite(staticserver.Site{}, failing, http.MethodGet, "/healthz"); rr.Code != http.StatusBadGateway {
		t.Fatalf("origin error on /healthz = %d, want 502", rr.Code)
	}

	rr := serveSite(staticserver.Site{}, publishedSite(nil), http.MethodPost, "/healthz")
	if rr.Code != http.StatusMethodNotAllowed || rr.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST /healthz = %d Allow %q, want 405 GET, HEAD", rr.Code, rr.Header().Get("Allow"))
	}

	h := staticserver.New(hostResolver{}, publishedSite(nil), 1<<20)
	rr = httptest.NewRecorder()
	newServer(":0", h).Handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "https://unknown.onbex.co/healthz", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown host /healthz = %d, want 404", rr.Code)
	}

	rr = httptest.NewRecorder()
	newServer(":0", http.HandlerFunc(originNotConfigured)).Handler.ServeHTTP(rr,
		httptest.NewRequest(http.MethodGet, "https://"+siteHost+"/healthz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("degraded /healthz = %d, want 503", rr.Code)
	}
}

// TestServerTimeoutsConfigured asserts the static server sets bounding timeouts
// so a slow-header/slow-read/idle client cannot pin a goroutine + fd open on the
// shared single-replica server (finding 12).
func TestServerTimeoutsConfigured(t *testing.T) {
	srv := newServer(":8080", http.NewServeMux())
	if srv.ReadHeaderTimeout <= 0 {
		t.Errorf("ReadHeaderTimeout = %v, want > 0", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout <= 0 {
		t.Errorf("ReadTimeout = %v, want > 0", srv.ReadTimeout)
	}
	if srv.WriteTimeout <= 0 {
		t.Errorf("WriteTimeout = %v, want > 0", srv.WriteTimeout)
	}
	if srv.IdleTimeout <= 0 {
		t.Errorf("IdleTimeout = %v, want > 0", srv.IdleTimeout)
	}
}
