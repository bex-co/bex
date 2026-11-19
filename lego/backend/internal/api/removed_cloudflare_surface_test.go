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

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ADR112 D3 removed the Cloudflare-shaped `wrangler r2` surface (w21/m500):
// objects.<domain> is the S3 gateway's alone, and bex-api serves Render's
// /v1/objects routes and the bex-only /v1/object-storage routes. A route that
// brings a /cloudflare path back would be a second internet-facing object
// surface that nobody tests, so the root mux must register none and must not
// answer one.
func TestNoRouteServesTheRemovedCloudflareSurface(t *testing.T) {
	srv := matrixServer(t)
	root, err := srv.rootMux()
	if err != nil {
		t.Fatalf("rootMux: %v", err)
	}
	for _, pattern := range serveMuxPatterns(root) {
		if strings.Contains(strings.ToLower(pattern), "cloudflare") {
			t.Errorf("the root mux registers %q: the Cloudflare-shaped surface was removed (ADR112 D3)", pattern)
		}
	}
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/cloudflare/client/v4/accounts/tea-a/r2/buckets"},
		{http.MethodPut, "/cloudflare/client/v4/accounts/tea-a/r2/buckets/b/objects/k"},
		{http.MethodPost, "/cloudflare/client/v4/graphql"},
	} {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer rnd_not_a_key")
		w := httptest.NewRecorder()
		root.ServeHTTP(w, req)
		if w.Code == http.StatusOK || strings.Contains(w.Body.String(), `"success"`) {
			t.Errorf("%s %s = %d %s, want it unserved", c.method, c.path, w.Code, w.Body.String())
		}
	}
}
