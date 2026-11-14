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
// the SPA fallback and serve the homepage. w8/318: Render serves its index in
// place with 200 (captured 2026-11-13: a Docusaurus site's /docs/about and
// /docs/about/ carry the same ETag, with no Location).
func directoryIndexObjects() map[string]Object {
	return map[string]Object{
		key("index.html"):      {Body: []byte("HOME"), ContentType: "text/html"},
		key("blog/index.html"): {Body: []byte("BLOG"), ContentType: "text/html"},
	}
}

func TestADirectoryWithoutItsSlashServesItsIndexInPlace(t *testing.T) {
	h, _ := newTestHandler(t, Site{
		Headers: []appv1alpha1.StaticHeader{{Path: "/blog", Name: "X-Blog-Bare", Value: "yes"}, {Path: "/blog/index.html", Name: "X-Index", Value: "leaked"}},
		// A catch-all SPA rewrite must not claim a published directory.
		Routes: []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
	}, directoryIndexObjects())
	slash := do(h, http.MethodGet, "/blog/")
	if slash.Code != http.StatusOK || slash.Body.String() != "BLOG" {
		t.Fatalf("/blog/ => %d %q, want 200 BLOG", slash.Code, slash.Body)
	}
	assertServedInPlace(t, h, "/blog?x=1&y=2", slash, "BLOG")
	// Custom headers match the visitor's path, never the served object's.
	if rec := do(h, http.MethodGet, "/blog"); rec.Header().Get("X-Blog-Bare") != "yes" || rec.Header().Get("X-Index") != "" {
		t.Fatalf("/blog headers = %v, want the request path's rule only", rec.Header())
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
