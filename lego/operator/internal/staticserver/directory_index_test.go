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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/384: a directory requested without its trailing slash used to fall into
// the SPA fallback and serve the homepage.
func directoryIndexObjects() map[string]Object {
	return map[string]Object{
		key("index.html"):      {Body: []byte("HOME"), ContentType: "text/html"},
		key("blog/index.html"): {Body: []byte("BLOG"), ContentType: "text/html"},
	}
}

func TestDirectoryWithoutSlashRedirectsToTheSlashForm(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Headers: []appv1alpha1.StaticHeader{{Path: "/blog/*", Name: "X-Blog", Value: "yes"}, {Path: "/blog", Name: "X-Blog-Bare", Value: "yes"}},
		// A catch-all SPA rewrite must not claim a published directory.
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
	}, directoryIndexObjects())
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := do(h, method, "/blog?x=1&y=2")
		if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/blog/?x=1&y=2" {
			t.Fatalf("%s /blog => %d Location=%q, want 301 /blog/?x=1&y=2", method, rec.Code, rec.Header().Get("Location"))
		}
		if rec.Header().Get("X-Blog-Bare") != "yes" {
			t.Fatalf("%s redirect lost the request-path headers: %v", method, rec.Header())
		}
	}
	if rec := do(h, http.MethodGet, "/blog/"); rec.Code != http.StatusOK || rec.Body.String() != "BLOG" {
		t.Fatalf("/blog/ => %d %q, want 200 BLOG", rec.Code, rec.Body)
	}
}

func TestRewriteToADirectoryServesItsIndexInPlace(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/x", Destination: "/blog"}},
	}, directoryIndexObjects())
	if rec := do(h, http.MethodGet, "/x"); rec.Code != http.StatusOK || rec.Body.String() != "BLOG" {
		t.Fatalf("rewrite /x -> /blog => %d %q, want 200 BLOG", rec.Code, rec.Body)
	}
}

func TestDirectoryResolutionLeavesMissesAlone(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, directoryIndexObjects())
	if rec := do(h, http.MethodGet, "/docs"); rec.Code != http.StatusOK || rec.Body.String() != "HOME" {
		t.Errorf("/docs with no docs/index.html => %d %q, want the SPA fallback", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/missing.css"); rec.Code != http.StatusNotFound {
		t.Errorf("/missing.css => %d, want 404", rec.Code)
	}
}
