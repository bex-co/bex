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
	"errors"
	"net/http"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// revalidate is the success policy every response without a matching custom
// Cache-Control rule must carry (w4/m160). Pinned literally: it is the wire
// contract browsers act on, not an implementation detail.
const revalidate = "public, max-age=0, must-revalidate"

// publishResolver serves one host whose active revision a test can swap, like
// the real resolver after an atomic publication.
type publishResolver struct{ site *Site }

func (p publishResolver) Resolve(host string) (Site, bool) {
	if host != testHost {
		return Site{}, false
	}
	return *p.site, true
}

// The w4/m160 regression: a republish changes the bytes behind an unchanged
// public URL, so that URL must never be handed to browsers as immutable — the
// live failure kept /render.yaml from rev-1 after rev-2 was serving. The
// revision-keyed origin cache must still be reused across the swap.
func TestRepublishedURLServesNewBytesAndRevalidates(t *testing.T) {
	origin := newFakeOrigin(map[string]Object{
		appID + "/rev-1/render.yaml": {Body: []byte("static-site"), ContentType: "binary/octet-stream"},
		appID + "/rev-2/render.yaml": {Body: []byte("hello-go"), ContentType: "binary/octet-stream"},
	})
	site := Site{AppID: appID, Revision: "rev-1"}
	h := New(publishResolver{site: &site}, origin, 1<<20)

	for _, step := range []struct{ rev, body string }{
		{"rev-1", "static-site"},
		{"rev-2", "hello-go"},
		{"rev-1", "static-site"}, // restore, as the live journey did
	} {
		site.Revision = step.rev
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := do(h, method, "/render.yaml")
			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s => %d, want 200", method, step.rev, rec.Code)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != revalidate {
				t.Errorf("%s %s cache-control = %q, want %q", method, step.rev, cc, revalidate)
			}
			if method == http.MethodGet && rec.Body.String() != step.body {
				t.Errorf("%s body = %q, want %q", step.rev, rec.Body.String(), step.body)
			}
		}
	}

	// Each immutable revision object was fetched once; the restore reused it.
	for _, r := range []string{"rev-1", "rev-2"} {
		if n := origin.gets[appID+"/"+r+"/render.yaml"]; n != 1 {
			t.Errorf("%s origin gets = %d, want 1 (revision cache reused)", r, n)
		}
	}
}

// Every successful delivery path shares the default, whatever the filename:
// no extension or content-hash heuristic may grant long freshness.
func TestSuccessResponsesDefaultToRevalidation(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/app", Destination: "/assets/app.3f9a1c7e.js"}},
	}, map[string]Object{
		key("index.html"):               {Body: []byte("<h1>home</h1>"), ContentType: "text/html"},
		key("docs/index.html"):          {Body: []byte("<h1>docs</h1>"), ContentType: "text/html"},
		key("render.yaml"):              {Body: []byte("services: []\n")},
		key("assets/app.js"):            {Body: []byte("console.log(1)")},
		key("assets/app.3f9a1c7e.js"):   {Body: []byte("console.log(2)")},
		key("assets/site.css"):          {Body: []byte("body{}")},
		key("fonts/inter.woff2"):        {Body: []byte("font")},
		key("LICENSE"):                  {Body: []byte("MIT")},
		key("config/settings.json"):     {Body: []byte("{}")},
		key("images/logo.9b2e41d0.png"): {Body: []byte("png")},
	})

	for name, p := range map[string]string{
		"root index":       "/",
		"directory index":  "/docs/",
		"plain yaml":       "/render.yaml",
		"plain js":         "/assets/app.js",
		"hash-looking js":  "/assets/app.3f9a1c7e.js",
		"css":              "/assets/site.css",
		"font":             "/fonts/inter.woff2",
		"extensionless":    "/LICENSE",
		"json config":      "/config/settings.json",
		"hash-looking png": "/images/logo.9b2e41d0.png",
		"rewrite target":   "/app",
		"SPA fallback":     "/dashboard",
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := do(h, method, p)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s (%s) => %d, want 200", method, p, name, rec.Code)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != revalidate {
				t.Errorf("%s %s (%s) cache-control = %q, want %q", method, p, name, cc, revalidate)
			}
		}
	}
}

// A matching custom Cache-Control rule still replaces the default — including
// an explicit immutable opt-in for content-addressed assets — with the last
// matching rule winning, matched on the visitor's request path.
func TestExplicitCacheControlOverridesDefault(t *testing.T) {
	const immutable = "public, max-age=31536000, immutable"
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/app", Destination: "/assets/app.js"}},
		Headers: []appv1alpha1.StaticHeader{
			{Path: "/*", Name: "Cache-Control", Value: "no-cache"},
			{Path: "/assets/*", Name: "Cache-Control", Value: immutable},
		},
	}, map[string]Object{
		key("index.html"):    {Body: []byte("<h1>home</h1>"), ContentType: "text/html"},
		key("render.yaml"):   {Body: []byte("services: []\n")},
		key("assets/app.js"): {Body: []byte("console.log(1)")},
	})

	for p, want := range map[string]string{
		"/assets/app.js": immutable,  // later rule wins
		"/render.yaml":   "no-cache", // only the catch-all matches
		"/app":           "no-cache", // rewrite: request path, not destination
		"/dashboard":     "no-cache", // SPA fallback
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := do(h, method, p)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s => %d, want 200", method, p, rec.Code)
			}
			if got := rec.Header().Values("Cache-Control"); len(got) != 1 || got[0] != want {
				t.Errorf("%s %s cache-control = %q, want exactly [%q]", method, p, got, want)
			}
		}
	}
}

// The success policy belongs to successful object responses only: misses,
// redirects and origin failures keep their own headers.
func TestNonSuccessResponsesCarryNoDefaultCachePolicy(t *testing.T) {
	h, origin := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/old", Destination: "/new"}},
	}, nil)
	origin.errs[key("denied.bin")] = errors.New("api error AccessDenied")
	origin.errs[key("big.bin")] = ErrObjectTooLarge
	origin.errs[key("busy.bin")] = ErrOverloaded

	for p, code := range map[string]int{
		"/missing.js": http.StatusNotFound,
		"/old":        http.StatusMovedPermanently,
		"/denied.bin": http.StatusBadGateway,
		"/big.bin":    http.StatusRequestEntityTooLarge,
		"/busy.bin":   http.StatusServiceUnavailable,
	} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != code {
			t.Fatalf("GET %s => %d, want %d", p, rec.Code, code)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "" {
			t.Errorf("GET %s (%d) cache-control = %q, want none", p, code, cc)
		}
	}
}
