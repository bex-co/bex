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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeOrigin is an in-memory object store keyed by full object key; it counts
// gets so tests can assert caching, and can serve injected per-key errors so
// tests can assert failure classes.
type fakeOrigin struct {
	objs map[string]Object
	gets map[string]int
	errs map[string]error
}

func newFakeOrigin(objs map[string]Object) *fakeOrigin {
	return &fakeOrigin{objs: objs, gets: map[string]int{}, errs: map[string]error{}}
}

func (f *fakeOrigin) Get(_ context.Context, key string) (Object, error) {
	f.gets[key]++
	if err, ok := f.errs[key]; ok {
		return Object{}, err
	}
	// Emulate the store's key cap: real S3 answers a key past 1024 bytes with a
	// KeyTooLongError client error, not a clean not-found (w6/047).
	if len(key) > maxObjectKeyBytes {
		return Object{}, errors.New("api error KeyTooLongError: your key is too long")
	}
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

func TestWildcardRedirectRejectsUnsafeExpandedLocation(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/old/*", Destination: "/:splat"}},
	}, map[string]Object{})
	for _, path := range []string{"/old/%5Cevil.example", "/old/%5C%5Cevil.example"} {
		rec := do(h, http.MethodGet, path)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s => %d Location=%q, want 400 with no redirect", path, rec.Code, rec.Header().Get("Location"))
		}
		if loc := rec.Header().Get("Location"); loc != "" {
			t.Errorf("unsafe expanded redirect emitted Location %q", loc)
		}
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

func TestOverlongPathIs404NotOriginError(t *testing.T) {
	// The store caps object keys at maxObjectKeyBytes; a request path whose
	// derived key is longer can never match an object, so it must be a hard 404
	// with no origin round trip — not even the SPA fallback — rather than a 502
	// "origin error" from the store's key-length complaint (w6/047).
	pad := maxObjectKeyBytes - len(key("")) // longest legal site-relative key
	deep := strings.Repeat("a", pad-len(".html")) + ".html"
	h, origin := newTestHandler(t, Site{}, map[string]Object{
		key(deep):         {Body: []byte("deep"), ContentType: "text/html"},
		key("index.html"): {Body: []byte("SPA"), ContentType: "text/html"},
	})

	// Control: a long-but-legal path (key exactly at the cap) still serves.
	if rec := do(h, http.MethodGet, "/"+deep); rec.Code != http.StatusOK || rec.Body.String() != "deep" {
		t.Fatalf("GET key at cap => %d %q, want 200 deep", rec.Code, rec.Body)
	}

	for _, p := range []string{
		"/" + strings.Repeat("a", pad+1-len(".html")) + ".html",   // asset-like, one byte past the cap
		"/" + strings.Repeat("a", pad+1),                          // extension-less: still no SPA fallback
		"/" + strings.Repeat("a", pad+1-len("/index.html")) + "/", // legal raw key, but index.html defaulting crosses the cap
	} {
		if rec := do(h, http.MethodGet, p); rec.Code != http.StatusNotFound {
			t.Errorf("GET overlong %d-byte path => %d (body %q), want 404", len(p), rec.Code, rec.Body)
		}
	}
	if n := origin.gets[key("index.html")]; n != 0 {
		t.Errorf("SPA fallback fetched index.html %d times for impossible keys, want 0", n)
	}
	for k, n := range origin.gets {
		if len(k) > maxObjectKeyBytes {
			t.Errorf("origin saw impossible %d-byte key (%d gets)", len(k), n)
		}
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

func TestKeyForPrefersRecordedPrefix(t *testing.T) {
	s := Site{AppID: "web", Revision: "rev-1", Prefix: "tea-aaaaaaaaaaaaaaaaaaaa/web/rev-1/"}
	if got, want := s.keyFor("/index.html"), "tea-aaaaaaaaaaaaaaaaaaaa/web/rev-1/index.html"; got != want {
		t.Errorf("keyFor = %q, want %q", got, want)
	}
}

func TestKeyForLegacyFallback(t *testing.T) {
	s := Site{AppID: "web", Revision: "rev-1"}
	if got, want := s.keyFor("/index.html"), "web/rev-1/index.html"; got != want {
		t.Errorf("keyFor = %q, want %q", got, want)
	}
}

// The w4/m94 t001 regression: an existing published object must win over a
// catch-all rewrite (the UI's own SPA example), preserving its exact body,
// content type, and cache policy — while a genuine miss still rewrites.
func TestExistingObjectWinsOverCatchAllRewrite(t *testing.T) {
	yaml := "services:\n  - type: web\n"
	h, origin := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
	}, map[string]Object{
		key("render.yaml"): {Body: []byte(yaml), ContentType: "binary/octet-stream"},
		key("index.html"):  {Body: []byte("<h1>SPA</h1>"), ContentType: "text/html"},
	})

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := do(h, method, "/render.yaml")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s /render.yaml => %d, want 200", method, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "binary/octet-stream" {
			t.Errorf("%s content-type = %q, want the object's own", method, ct)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
			t.Errorf("%s cache-control = %q, want the asset policy", method, cc)
		}
		if method == http.MethodGet && rec.Body.String() != yaml {
			t.Errorf("body = %q, want the original object bytes", rec.Body.String())
		}
	}

	// A genuine miss still follows the rewrite.
	rec := do(h, http.MethodGet, "/qa-route")
	if rec.Code != http.StatusOK || rec.Body.String() != "<h1>SPA</h1>" {
		t.Fatalf("GET /qa-route => %d %q, want the rewritten index.html", rec.Code, rec.Body)
	}

	// The existing-object win costs exactly one origin get; a repeat is cached.
	if rec := do(h, http.MethodGet, "/render.yaml"); rec.Code != http.StatusOK {
		t.Fatal("cached repeat failed")
	}
	if n := origin.gets[key("render.yaml")]; n != 1 {
		t.Errorf("render.yaml origin gets = %d, want 1 (cached repeat)", n)
	}
}

func TestExistingObjectWinsOverMatchingRedirect(t *testing.T) {
	yaml := "services: []\n"
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{
			{Type: "redirect", Source: "/render.yaml", Destination: "/index.html"},
			{Type: "redirect", Source: "/qa-old", Destination: "/index.html"},
		},
	}, map[string]Object{
		key("render.yaml"): {Body: []byte(yaml)},
		key("index.html"):  {Body: []byte("<h1>SPA</h1>"), ContentType: "text/html"},
	})

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := do(h, method, "/render.yaml")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s /render.yaml => %d, want 200 (existing file wins)", method, rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "" {
			t.Errorf("%s emitted Location %q for an existing file", method, loc)
		}
	}

	// The missing path still redirects.
	rec := do(h, http.MethodGet, "/qa-old")
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/index.html" {
		t.Fatalf("GET /qa-old => %d Location=%q, want 301 /index.html", rec.Code, rec.Header().Get("Location"))
	}
}

// The w4/m94 t002 regression: custom headers match the visitor's request path,
// not the rewrite destination — and a destination-scoped header must not leak
// onto rewritten requests.
func TestHeadersMatchOriginalRequestPathUnderRewrite(t *testing.T) {
	site := Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
		Headers: []appv1alpha1.StaticHeader{
			{Path: "/qa-route", Name: "X-QA-Path", Value: "request-path"},
			{Path: "/*", Name: "X-QA-All", Value: "all-paths"},
			{Path: "/index.html", Name: "X-QA-Dest", Value: "destination-only"},
		},
	}
	h, _ := newTestHandler(t, site, map[string]Object{
		key("index.html"): {Body: []byte("<h1>SPA</h1>"), ContentType: "text/html"},
	})

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := do(h, method, "/qa-route")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s /qa-route => %d, want 200", method, rec.Code)
		}
		if got := rec.Header().Get("X-QA-Path"); got != "request-path" {
			t.Errorf("%s X-QA-Path = %q, want request-path (matched on the request path)", method, got)
		}
		if got := rec.Header().Get("X-QA-All"); got != "all-paths" {
			t.Errorf("%s X-QA-All = %q, want all-paths", method, got)
		}
		if got := rec.Header().Get("X-QA-Dest"); got != "" {
			t.Errorf("%s X-QA-Dest = %q; a destination-scoped header must not apply to /qa-route", method, got)
		}
	}

	// A direct request for the destination still gets its scoped header.
	rec := do(h, http.MethodGet, "/index.html")
	if got := rec.Header().Get("X-QA-Dest"); got != "destination-only" {
		t.Errorf("X-QA-Dest = %q on /index.html itself, want destination-only", got)
	}
}

// Pin the previously-working control: with no explicit rule, the implicit SPA
// fallback already matched headers on the request path. Keep it that way.
func TestHeadersMatchRequestPathOnImplicitFallback(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Headers: []appv1alpha1.StaticHeader{
			{Path: "/qa-route", Name: "X-QA-Path", Value: "request-path"},
			{Path: "/index.html", Name: "X-QA-Dest", Value: "destination-only"},
		},
	}, map[string]Object{
		key("index.html"): {Body: []byte("<h1>SPA</h1>"), ContentType: "text/html"},
	})
	rec := do(h, http.MethodGet, "/qa-route")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /qa-route => %d, want 200 (implicit fallback)", rec.Code)
	}
	if got := rec.Header().Get("X-QA-Path"); got != "request-path" {
		t.Errorf("X-QA-Path = %q, want request-path", got)
	}
	if got := rec.Header().Get("X-QA-Dest"); got != "" {
		t.Errorf("X-QA-Dest = %q; must not leak onto /qa-route", got)
	}
}

// The first lookup must not turn origin failures into rules or fallbacks:
// permission denial stays 502, overload stays 503 + Retry-After, an oversized
// object stays 413 — even with a catch-all rewrite and a matching redirect.
func TestOriginFailureDoesNotFallThroughToRules(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantCode   int
		wantHeader string
	}{
		{"denial", errors.New("api error AccessDenied"), http.StatusBadGateway, ""},
		{"overload", ErrOverloaded, http.StatusServiceUnavailable, "Retry-After"},
		{"oversize", ErrObjectTooLarge, http.StatusRequestEntityTooLarge, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, origin := newTestHandler(t, Site{
				Routes: []appv1alpha1.StaticRoute{
					{Type: "redirect", Source: "/render.yaml", Destination: "/index.html"},
					{Type: "rewrite", Source: "/*", Destination: "/index.html"},
				},
			}, map[string]Object{
				key("index.html"): {Body: []byte("<h1>SPA</h1>"), ContentType: "text/html"},
			})
			origin.errs[key("render.yaml")] = tc.err
			rec := do(h, http.MethodGet, "/render.yaml")
			if rec.Code != tc.wantCode {
				t.Fatalf("GET /render.yaml => %d, want %d (no rule/fallback on %s)", rec.Code, tc.wantCode, tc.name)
			}
			if loc := rec.Header().Get("Location"); loc != "" {
				t.Errorf("origin %s produced a redirect Location %q", tc.name, loc)
			}
			if tc.wantHeader != "" && rec.Header().Get(tc.wantHeader) == "" {
				t.Errorf("missing %s header", tc.wantHeader)
			}
		})
	}
}

// A rewrite target that misses still degrades exactly as before: origin errors
// on the rewritten lookup keep their classes, and a missing rewritten asset
// (with extension) is a 404, not a silent fallback.
func TestRewriteTargetFailureClasses(t *testing.T) {
	h, origin := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/app", Destination: "/app.html"}},
	}, map[string]Object{})
	origin.errs[key("app.html")] = ErrOverloaded
	if rec := do(h, http.MethodGet, "/app"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("rewrite-target overload => %d, want 503", rec.Code)
	}
	delete(origin.errs, key("app.html"))
	if rec := do(h, http.MethodGet, "/app"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing rewrite target => %d, want 404", rec.Code)
	}
}

// Directory-style requests keep index.html defaulting even with a catch-all
// rewrite saved: /docs/ serves docs/index.html, not the rewrite destination.
func TestDirectoryIndexWinsOverCatchAllRewrite(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
	}, map[string]Object{
		key("docs/index.html"): {Body: []byte("DOCS"), ContentType: "text/html"},
		key("index.html"):      {Body: []byte("ROOT"), ContentType: "text/html"},
	})
	rec := do(h, http.MethodGet, "/docs/")
	if rec.Code != http.StatusOK || rec.Body.String() != "DOCS" {
		t.Fatalf("GET /docs/ => %d %q, want the directory index", rec.Code, rec.Body)
	}
}

// A confirmed miss is immutable within a revision, so it is negatively cached:
// repeated requests to the same SPA deep link or redirect path cost exactly one
// origin round trip and stop consuming fetch-gate slots (w4/m94 follow-through
// for the lookup-before-rules order).
func TestConfirmedMissIsNegativelyCached(t *testing.T) {
	h, origin := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{
			{Type: "redirect", Source: "/qa-old", Destination: "/index.html"},
			{Type: "rewrite", Source: "/*", Destination: "/index.html"},
		},
	}, map[string]Object{
		key("index.html"): {Body: []byte("<h1>SPA</h1>"), ContentType: "text/html"},
	})

	for range 3 {
		if rec := do(h, http.MethodGet, "/qa-old"); rec.Code != http.StatusMovedPermanently {
			t.Fatalf("GET /qa-old => %d, want 301", rec.Code)
		}
		if rec := do(h, http.MethodGet, "/deep/client/route"); rec.Code != http.StatusOK {
			t.Fatalf("GET /deep/client/route => %d, want 200", rec.Code)
		}
	}
	if n := origin.gets[key("qa-old")]; n != 1 {
		t.Errorf("redirect-path miss origin gets = %d, want 1 (negatively cached)", n)
	}
	if n := origin.gets[key("deep/client/route")]; n != 1 {
		t.Errorf("SPA deep-link miss origin gets = %d, want 1 (negatively cached)", n)
	}
	if n := origin.gets[key("index.html")]; n != 1 {
		t.Errorf("rewrite target origin gets = %d, want 1 (positively cached)", n)
	}
}

// Only a confirmed miss is cached: transient failures (denial, overload) must
// be re-asked so recovery is immediate.
func TestTransientOriginFailureIsNotNegativelyCached(t *testing.T) {
	h, origin := newTestHandler(t, Site{}, map[string]Object{})
	origin.errs[key("flaky.txt")] = errors.New("api error AccessDenied")
	if rec := do(h, http.MethodGet, "/flaky.txt"); rec.Code != http.StatusBadGateway {
		t.Fatalf("denied => %d, want 502", rec.Code)
	}
	delete(origin.errs, key("flaky.txt"))
	origin.objs[key("flaky.txt")] = Object{Body: []byte("ok")}
	if rec := do(h, http.MethodGet, "/flaky.txt"); rec.Code != http.StatusOK {
		t.Fatalf("recovered origin => %d, want 200 (failure must not be cached)", rec.Code)
	}
}
