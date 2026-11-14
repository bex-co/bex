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
	"net/http/httptest"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/317: a prerendered site (SvelteKit adapter-static, Next.js export, Astro
// build.format "file") publishes about.html for /about. Render serves it in
// place: its sveltekit-static demo answers /about, /about/ and /index with
// 200, and a Docusaurus trailingSlash:false site gives /markdown-page,
// /markdown-page/ and /markdown-page.html the same ETag (captured 2026-11-13).
func htmlPageObjects() map[string]Object {
	return map[string]Object{
		key("index.html"):  {Body: []byte("HOME"), ContentType: "text/html"},
		key("about.html"):  {Body: []byte("ABOUT"), ContentType: "text/html"},
		key("404.html"):    {Body: []byte("CUSTOM-404"), ContentType: "text/html"},
		key("favicon.png"): {Body: []byte("PNG"), ContentType: "image/png"},
	}
}

func TestAnExtensionlessPathServesItsHTMLPageInPlace(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		NoSPAFallback: true,
		Headers:       []appv1alpha1.StaticHeader{{Path: "/about", Name: "X-About", Value: "yes"}, {Path: "/about.html", Name: "X-File", Value: "leaked"}},
		// A catch-all SPA rewrite must not claim a published page.
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
	}, htmlPageObjects())
	file := do(h, http.MethodGet, "/about.html")
	if file.Code != http.StatusOK || file.Body.String() != "ABOUT" {
		t.Fatalf("/about.html => %d %q, want 200 ABOUT", file.Code, file.Body)
	}
	for _, p := range []string{"/about", "/about/", "/about?x=1"} {
		assertServedInPlace(t, h, p, file, "ABOUT")
	}
	// Custom headers match the visitor's path, never the served object's.
	if rec := do(h, http.MethodGet, "/about"); rec.Header().Get("X-About") != "yes" || rec.Header().Get("X-File") != "" {
		t.Fatalf("/about headers = %v, want the request path's rule only", rec.Header())
	}
	for _, p := range []string{"/index", "/index/"} {
		if rec := do(h, http.MethodGet, p); rec.Code != http.StatusOK || rec.Body.String() != "HOME" {
			t.Fatalf("%s => %d %q, want the root index.html", p, rec.Code, rec.Body)
		}
	}
}

// Only <path>.html is tried for an extensionless path; Render answers each of
// these with 404 on the same demo.
func TestOnlyAnExtensionlessPathTriesItsHTMLPage(t *testing.T) {
	h, _ := newTestHandler(t, Site{NoSPAFallback: true}, htmlPageObjects())
	for _, p := range []string{"/favicon", "/about.html/", "/about.htm", "/ABOUT", "/about/x"} {
		rec := do(h, http.MethodGet, p)
		if rec.Code != http.StatusNotFound || rec.Body.String() != "CUSTOM-404" {
			t.Errorf("%s => %d %q, want 404 with the site's 404.html", p, rec.Code, rec.Body)
		}
	}
}

// A site created before the SPA cutoff serves the page itself, not the root
// index.html its implicit fallback would.
func TestAPreCutoffSiteServesThePageNotTheRoot(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, htmlPageObjects())
	if rec := do(h, http.MethodGet, "/about"); rec.Code != http.StatusOK || rec.Body.String() != "ABOUT" {
		t.Fatalf("/about => %d %q, want about.html", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/client-route"); rec.Code != http.StatusOK || rec.Body.String() != "HOME" {
		t.Fatalf("/client-route => %d %q, want the implicit fallback", rec.Code, rec.Body)
	}
}

// Render's order when a site publishes both about.html and about/index.html
// was not captured. The slashless path prefers the file and the slash path the
// directory, as each was asked.
func TestAPageAndADirectoryEachAnswerTheirOwnForm(t *testing.T) {
	objs := htmlPageObjects()
	objs[key("about/index.html")] = Object{Body: []byte("ABOUT-DIR"), ContentType: "text/html"}
	h, _ := newTestHandler(t, Site{NoSPAFallback: true}, objs)
	if rec := do(h, http.MethodGet, "/about"); rec.Body.String() != "ABOUT" {
		t.Errorf("/about => %q, want about.html", rec.Body)
	}
	if rec := do(h, http.MethodGet, "/about/"); rec.Body.String() != "ABOUT-DIR" {
		t.Errorf("/about/ => %q, want about/index.html", rec.Body)
	}
}

func TestARewriteToAnExtensionlessPathServesItsPage(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		NoSPAFallback: true,
		Routes:        []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/team", Destination: "/about"}},
	}, htmlPageObjects())
	if rec := do(h, http.MethodGet, "/team"); rec.Code != http.StatusOK || rec.Body.String() != "ABOUT" {
		t.Fatalf("rewrite /team -> /about => %d %q, want 200 ABOUT", rec.Code, rec.Body)
	}
}

// A published page wins over a rule as any object does, even one whose
// expansion is refused (the 400 a miss would get).
func TestAPageWinsOverABrokenRule(t *testing.T) {
	name := strings.Repeat("a", 100)
	objs := htmlPageObjects()
	objs[key(name+".html")] = Object{Body: []byte("LONG"), ContentType: "text/html"}
	h, _ := newTestHandler(t, Site{
		NoSPAFallback: true,
		Routes:        []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/*", Destination: "/" + strings.Repeat(":splat/", 250)}},
	}, objs)
	if rec := do(h, http.MethodGet, "/"+name); rec.Code != http.StatusOK || rec.Body.String() != "LONG" {
		t.Fatalf("/%s => %d %q, want the page", name, rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/b"+name); rec.Code != http.StatusBadRequest {
		t.Fatalf("a miss under the broken rule => %d, want 400", rec.Code)
	}
}

// An origin failure on the page answers its own class, directly or behind a
// rewrite; it never degrades into a rule or the root page.
func TestAPageOriginFailureIsNotAMiss(t *testing.T) {
	h, origin := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{
			{Type: "rewrite", Source: "/team", Destination: "/about"},
			{Type: "rewrite", Source: "/*", Destination: "/index.html"},
		},
	}, htmlPageObjects())
	origin.errs[key("about.html")] = errors.New("origin timeout")
	for _, p := range []string{"/about", "/team"} {
		if rec := do(h, http.MethodGet, p); rec.Code != http.StatusBadGateway {
			t.Errorf("%s with a failing origin => %d %q, want 502", p, rec.Code, rec.Body)
		}
	}
}

// assertServedInPlace checks that GET and HEAD of p answer 200 with no
// Location and ref's representation (its Content-Type and ETag), that the GET
// carries wantBody, and that ref's ETag revalidates p with a 304.
func assertServedInPlace(t *testing.T, h *Handler, p string, ref *httptest.ResponseRecorder, wantBody string) {
	t.Helper()
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := do(h, method, p)
		if rec.Code != http.StatusOK || rec.Header().Get("Location") != "" {
			t.Fatalf("%s %s => %d Location=%q, want 200 in place", method, p, rec.Code, rec.Header().Get("Location"))
		}
		if method == http.MethodGet && rec.Body.String() != wantBody {
			t.Fatalf("GET %s => %q, want %q", p, rec.Body, wantBody)
		}
		if rec.Header().Get("Content-Type") != ref.Header().Get("Content-Type") || rec.Header().Get("Etag") != ref.Header().Get("Etag") {
			t.Fatalf("%s %s => type %q etag %q, want %q %q", method, p, rec.Header().Get("Content-Type"), rec.Header().Get("Etag"), ref.Header().Get("Content-Type"), ref.Header().Get("Etag"))
		}
	}
	if rec := doWith(h, http.MethodGet, p, http.Header{"If-None-Match": {ref.Header().Get("Etag")}}); rec.Code != http.StatusNotModified {
		t.Fatalf("conditional %s => %d, want 304", p, rec.Code)
	}
}
