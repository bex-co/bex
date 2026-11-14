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
	"net/http"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/324: Render's redirect rule (wc3v.onrender.com, 2026-11-14) carries the
// visitor's query to the destination, answers every method with the same 301,
// and sends only Content-Type and Location: none of the site's rules, HSTS or
// the cache policy, although the site's files carry all three.
func redirectSite(t *testing.T) *Handler {
	t.Helper()
	h, _ := newTestHandler(t, Site{
		Headers: []appv1alpha1.StaticHeader{{Path: "/*", Name: "X-Frame-Options", Value: "SAMEORIGIN"}},
		Routes: []appv1alpha1.StaticRoute{
			{Type: "redirect", Source: "/ext/*", Destination: "https://cdn.example.com/ext/*"},
			{Type: "redirect", Source: "/loc/*", Destination: "/target/*"},
			{Type: "redirect", Source: "/tagged/*", Destination: "https://cdn.example.com/t/*?src=rule#top"},
			{Type: "redirect", Source: "/broken/*", Destination: "/" + strings.Repeat(":splat/", 250)},
			{Type: "rewrite", Source: "/spa/*", Destination: "/index.html"},
		},
	}, map[string]Object{key("index.html"): {Body: []byte("HOME"), ContentType: "text/html"}})
	return h
}

func TestRedirectCarriesTheRequestQuery(t *testing.T) {
	h := redirectSite(t)
	for _, tc := range []struct{ path, location string }{
		{"/ext/foo.jpg?v=1&x=2", "https://cdn.example.com/ext/foo.jpg?v=1&x=2"},
		{"/ext/foo.jpg", "https://cdn.example.com/ext/foo.jpg"},
		{"/ext/foo.jpg?", "https://cdn.example.com/ext/foo.jpg"},
		{"/loc/x.txt?q=1", "/target/x.txt?q=1"},
		// The destination's own query stays first; the visitor's follows it,
		// and the fragment stays last.
		{"/tagged/x.png?v=2", "https://cdn.example.com/t/x.png?src=rule&v=2#top"},
		// A raw "#" a client sent stays query text, never the fragment.
		{"/tagged/x.png?v=2#a", "https://cdn.example.com/t/x.png?src=rule&v=2%23a#top"},
		// Past the "?" a reverse solidus cannot move the Location's host.
		{`/ext/x?q=a\b`, `https://cdn.example.com/ext/x?q=a\b`},
	} {
		rec := do(h, http.MethodGet, tc.path)
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != tc.location {
			t.Errorf("GET %s => %d Location=%q, want 301 %q", tc.path, rec.Code, rec.Header().Get("Location"), tc.location)
		}
	}
}

func TestRedirectAnswersEveryMethod(t *testing.T) {
	h := redirectSite(t)
	methods := []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions}
	for _, method := range methods {
		rec := do(h, method, "/ext/map.jpg?v=1")
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "https://cdn.example.com/ext/map.jpg?v=1" {
			t.Fatalf("%s => %d Location=%q, want Render's 301 for every method", method, rec.Code, rec.Header().Get("Location"))
		}
		if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("%s Content-Type=%q, want text/html; charset=utf-8", method, got)
		}
		for _, name := range []string{"X-Frame-Options", "Strict-Transport-Security", "Cache-Control", "X-Content-Type-Options"} {
			if got := rec.Header().Get(name); got != "" {
				t.Errorf("%s 301 carried %s=%q; Render's carries only Content-Type and Location", method, name, got)
			}
		}
		if method != http.MethodGet && rec.Body.Len() != 0 {
			t.Errorf("%s 301 body %q, want none", method, rec.Body)
		}
	}
	// A target that fails to expand is refused as on GET, rules included.
	for _, method := range methods {
		rec := do(h, method, "/broken/"+strings.Repeat("a", 100))
		if rec.Code != http.StatusBadRequest || rec.Header().Get("Location") != "" || rec.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
			t.Errorf("%s broken => %d Location=%q headers %v, want the 400 GET gets", method, rec.Code, rec.Header().Get("Location"), rec.Header())
		}
	}
	// A local rewrite, and git metadata (never part of a site), keep w8/319's
	// bare 200 for another method.
	for _, p := range []string{"/spa/x", "/ext/.git/config"} {
		if rec := do(h, http.MethodPost, p); rec.Code != http.StatusOK || rec.Body.Len() != 0 || rec.Header().Get("Location") != "" {
			t.Errorf("POST %s => %d Location=%q, want a bare 200", p, rec.Code, rec.Header().Get("Location"))
		}
	}
}
