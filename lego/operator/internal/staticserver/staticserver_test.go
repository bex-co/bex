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

package staticserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeOrigin is an in-memory object store keyed by full object key; it counts
// gets so tests can assert caching.
type fakeOrigin struct {
	objs map[string]Object
	gets map[string]int
}

func newFakeOrigin(objs map[string]Object) *fakeOrigin {
	return &fakeOrigin{objs: objs, gets: map[string]int{}}
}

func (f *fakeOrigin) Get(_ context.Context, key string) (Object, error) {
	f.gets[key]++
	obj, ok := f.objs[key]
	if !ok {
		return Object{}, ErrNotFound
	}
	return obj, nil
}

// staticResolver maps a fixed host to a fixed Site.
type staticResolver struct {
	host string
	site Site
}

func (s staticResolver) Resolve(host string) (Site, bool) {
	if host == s.host {
		return s.site, true
	}
	return Site{}, false
}

const (
	testHost = "site.onbex.co"
	appID    = "mysite"
	rev      = "rev-3"
)

// key builds the object key the handler will look up for a site-relative path.
func key(p string) string { return appID + "/" + rev + "/" + p }

func newTestHandler(t *testing.T, site Site, objs map[string]Object) (*Handler, *fakeOrigin) {
	t.Helper()
	origin := newFakeOrigin(objs)
	site.AppID, site.Revision = appID, rev
	return New(staticResolver{host: testHost, site: site}, origin, 1<<20), origin
}

func do(h *Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://"+testHost+path, nil)
	req.Host = testHost
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServesIndexAtRoot(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("index.html"): {Body: []byte("<h1>home</h1>"), ContentType: "text/html"},
	})
	rec := do(h, http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / => %d, want 200 (body %q)", rec.Code, rec.Body)
	}
	if got := rec.Body.String(); got != "<h1>home</h1>" {
		t.Errorf("body = %q, want index.html contents", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html" {
		t.Errorf("content-type = %q, want text/html", ct)
	}
}

func TestServesNestedAsset(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("assets/app.js"): {Body: []byte("console.log(1)")},
	})
	rec := do(h, http.MethodGet, "/assets/app.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js => %d, want 200", rec.Code)
	}
	// Content-Type inferred from extension when the origin gives none.
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Errorf("content-type not set; want inferred from .js")
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("cache-control = %q, want immutable for a hashed asset", cc)
	}
}

func TestRedirectRoute(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/old", Destination: "/new"}},
	}, map[string]Object{})
	rec := do(h, http.MethodGet, "/old")
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /old => %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/new" {
		t.Errorf("Location = %q, want /new", loc)
	}
}

func TestRewriteRoute(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/app", Destination: "/app.html"}},
	}, map[string]Object{
		key("app.html"): {Body: []byte("APP"), ContentType: "text/html"},
	})
	rec := do(h, http.MethodGet, "/app")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /app (rewrite) => %d, want 200", rec.Code)
	}
	if rec.Body.String() != "APP" {
		t.Errorf("body = %q, want rewritten content APP", rec.Body.String())
	}
}

func TestRewriteNeverFetchesAnUpstreamURL(t *testing.T) {
	// The API accepts only destinations beginning with "/". Even if a tenant
	// makes the remainder look like an internal URL, the static server cleans it
	// as a path and asks the S3 Origin seam for that object key. It never creates
	// an HTTP request to the apparent host.
	h, origin := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{
			Type:        "rewrite",
			Source:      "/internal",
			Destination: "/http://bex-api.bex-system.svc:8091/private",
		}},
	}, map[string]Object{})
	rec := do(h, http.MethodGet, "/internal")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("internal-looking rewrite => %d, want object-store 404", rec.Code)
	}
	if got := origin.gets[key("http:/bex-api.bex-system.svc:8091/private")]; got != 1 {
		t.Fatalf("origin object lookup count = %d, want 1", got)
	}
}

func TestSplatRewrite(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/docs/*", Destination: "/help/:splat"}},
	}, map[string]Object{
		key("help/intro.html"): {Body: []byte("INTRO"), ContentType: "text/html"},
	})
	rec := do(h, http.MethodGet, "/docs/intro.html")
	if rec.Code != http.StatusOK || rec.Body.String() != "INTRO" {
		t.Fatalf("GET /docs/intro.html => %d %q, want 200 INTRO", rec.Code, rec.Body)
	}
}

func TestSPAFallbackRoute(t *testing.T) {
	// The canonical SPA rule: rewrite everything to /index.html.
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
	}, map[string]Object{
		key("index.html"): {Body: []byte("SPA"), ContentType: "text/html"},
	})
	rec := do(h, http.MethodGet, "/some/client/route")
	if rec.Code != http.StatusOK || rec.Body.String() != "SPA" {
		t.Fatalf("SPA route => %d %q, want 200 SPA", rec.Code, rec.Body)
	}
}

func TestImplicitSPAFallback(t *testing.T) {
	// No explicit route, but an extension-less miss falls back to index.html so
	// client-side routing works; an asset miss (has extension) stays 404.
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("index.html"): {Body: []byte("SPA"), ContentType: "text/html"},
	})
	if rec := do(h, http.MethodGet, "/dashboard"); rec.Code != http.StatusOK || rec.Body.String() != "SPA" {
		t.Errorf("extension-less miss => %d %q, want 200 SPA", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/missing.png"); rec.Code != http.StatusNotFound {
		t.Errorf("asset miss => %d, want 404", rec.Code)
	}
}

func TestCustomHeaderApplied(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Headers: []appv1alpha1.StaticHeader{
			{Path: "/*", Name: "X-Frame-Options", Value: "DENY"},
			{Path: "/assets/*", Name: "X-Asset", Value: "yes"},
		},
	}, map[string]Object{
		key("index.html"):   {Body: []byte("H"), ContentType: "text/html"},
		key("assets/a.css"): {Body: []byte("C")},
	})
	rec := do(h, http.MethodGet, "/")
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options = %q, want DENY", got)
	}
	if got := rec.Header().Get("X-Asset"); got != "" {
		t.Errorf("X-Asset should not apply to /, got %q", got)
	}
	rec = do(h, http.MethodGet, "/assets/a.css")
	if got := rec.Header().Get("X-Asset"); got != "yes" {
		t.Errorf("X-Asset = %q on /assets/a.css, want yes", got)
	}
}

func TestUnknownHost404(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{})
	req := httptest.NewRequest(http.MethodGet, "http://other.example/", nil)
	req.Host = "other.example"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown host => %d, want 404", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{})
	rec := do(h, http.MethodPost, "/")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST / => %d, want 405", rec.Code)
	}
}

func TestCacheServesFromMemory(t *testing.T) {
	h, origin := newTestHandler(t, Site{}, map[string]Object{
		key("index.html"): {Body: []byte("home"), ContentType: "text/html"},
	})
	for range 3 {
		if rec := do(h, http.MethodGet, "/"); rec.Code != http.StatusOK {
			t.Fatalf("GET / => %d, want 200", rec.Code)
		}
	}
	if n := origin.gets[key("index.html")]; n != 1 {
		t.Errorf("origin gets = %d, want 1 (subsequent served from cache)", n)
	}
}

func TestRedirectFirstMatchWins(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{
			{Type: "redirect", Source: "/a", Destination: "/first"},
			{Type: "redirect", Source: "/a", Destination: "/second"},
		},
	}, map[string]Object{})
	rec := do(h, http.MethodGet, "/a")
	if loc := rec.Header().Get("Location"); loc != "/first" {
		t.Errorf("Location = %q, want /first (first match wins)", loc)
	}
}

func TestHeadRequestNoBody(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("index.html"): {Body: []byte("home"), ContentType: "text/html"},
	})
	rec := do(h, http.MethodHead, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD / => %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("HEAD body = %q, want empty", rec.Body)
	}
}
