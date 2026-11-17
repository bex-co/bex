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

// w33/007: bex-api stores a destination that is neither rooted nor a URL, as
// Render's redwoodjs template ships one ("replace_with_api_url/*"). The server
// never serves it: a GET it matches answers 400, other methods get Render's
// bare 200, and the rules after it still serve every other path. Render's own
// answer is uncaptured (ADR029).
func TestUnrootedDestinationIsNeverServed(t *testing.T) {
	for _, action := range []string{"rewrite", "redirect"} {
		t.Run(action, func(t *testing.T) {
			h, origin := newTestHandler(t, Site{NoSPAFallback: true, Routes: []appv1alpha1.StaticRoute{
				{Type: action, Source: "/.redwood/functions/*", Destination: "replace_with_api_url/*"},
				{Type: "rewrite", Source: "/*", Destination: "/index.html"},
			}}, map[string]Object{key("index.html"): {Body: []byte("SPA"), ContentType: "text/html"}})

			rec := do(h, http.MethodGet, "/.redwood/functions/graphql")
			if rec.Code != http.StatusBadRequest || rec.Header().Get("Location") != "" || rec.Body.String() != "invalid "+action+" target\n" {
				t.Fatalf("GET matched = %d %v %q, want 400 invalid %s target", rec.Code, rec.Header(), rec.Body, action)
			}
			for k := range origin.gets {
				if k == key("replace_with_api_url/graphql") || k == key("index.html") {
					t.Fatalf("the unrooted rule fetched %s", k)
				}
			}

			// A redirect rule answers every method alike (w8/324); a rewrite
			// that is not external leaves other methods Render's bare 200.
			post := do(h, http.MethodPost, "/.redwood/functions/graphql")
			if action == "redirect" && post.Code != http.StatusBadRequest {
				t.Fatalf("POST matched = %d %q, want the redirect's 400", post.Code, post.Body)
			}
			if action == "rewrite" && (post.Code != http.StatusOK || post.Body.Len() != 0) {
				t.Fatalf("POST matched = %d %q, want Render's bare 200", post.Code, post.Body)
			}

			if spa := do(h, http.MethodGet, "/about"); spa.Code != http.StatusOK || spa.Body.String() != "SPA" {
				t.Fatalf("GET /about = %d %q, want the next rule's index.html", spa.Code, spa.Body)
			}
		})
	}
}
