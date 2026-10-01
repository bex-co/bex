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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutingErrorsAreStructured(t *testing.T) {
	srv := matrixServer(t)
	srv.CORSOrigin = corsTestOrigin
	h, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, path       string
		status             int
		id, message, allow string
	}{
		{"GET", "/v1/nonexistent", 404, "not_found", "not found", ""},
		{"GET", "/v1/services/web/nonexistent", 404, "not_found", "not found", ""},
		{"GET", "/v2/services", 404, "not_found", "not found", ""},
		{"PUT", "/v1/services", 405, "method_not_allowed", "method not allowed", "GET, HEAD, POST"},
		{"DELETE", "/v1/owners", 405, "method_not_allowed", "method not allowed", "GET, HEAD"},
		{"GET", "/v1/services/nope", 404, "not_found", "not found", ""},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+testToken)
			req.Header.Set("Origin", corsTestOrigin)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get("Allow"); got != tc.allow {
				t.Errorf("Allow = %q, want %q", got, tc.allow)
			}
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Errorf("Content-Type = %q", got)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != corsTestOrigin {
				t.Errorf("Allow-Origin = %q", got)
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error envelope: %v; body=%q", err, rec.Body.String())
			}
			if body["id"] != tc.id || body["message"] != tc.message || body["error"] != tc.message {
				t.Errorf("envelope = %#v", body)
			}
		})
	}
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/v1/nonexistent", 401}, {"PUT", "/v1/services", 401}, {"GET", "/v2/services", 404},
	} {
		if got := do(t, h, tc.method, tc.path, "", "").Code; got != tc.status {
			t.Errorf("unauthenticated %s %s = %d, want %d", tc.method, tc.path, got, tc.status)
		}
	}
	health := do(t, h, "GET", "/healthz", "", "")
	if health.Code != 200 || health.Body.String() != "ok" || strings.HasPrefix(health.Header().Get("Content-Type"), "application/json") {
		t.Errorf("health changed: %d %q", health.Code, health.Body.String())
	}
}

func TestRoutingErrorsPreserveMatchedHandlersAndRedirects(t *testing.T) {
	srv := matrixServer(t)
	muxes, err := srv.composedMuxes()
	if err != nil {
		t.Fatal(err)
	}
	muxes.rest.HandleFunc("GET /v1/routing-control/{value}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Matched-Pattern", r.Pattern)
		http.Error(w, "resource "+r.PathValue("value")+" missing", http.StatusNotFound)
	})
	muxes.rest.HandleFunc("GET /v1/routing-subtree/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := srv.wrapMuxes(muxes)
	matched := do(t, h, "GET", "/v1/routing-control/example", testToken, "")
	if matched.Code != 404 || matched.Body.String() != "resource example missing\n" || matched.Header().Get("X-Matched-Pattern") != "GET /v1/routing-control/{value}" || !strings.HasPrefix(matched.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("matched handler altered: status=%d body=%q headers=%v", matched.Code, matched.Body.String(), matched.Header())
	}
	for _, tc := range []struct{ path, location string }{
		{"/v1/routing-subtree", "/v1/routing-subtree/"},
		{"/v1//routing-subtree/", "/v1/routing-subtree/"},
	} {
		rec := do(t, h, "GET", tc.path, testToken, "")
		if rec.Code != http.StatusTemporaryRedirect || rec.Header().Get("Location") != tc.location {
			t.Errorf("redirect %s: status=%d Location=%q", tc.path, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestRoutingErrorCORSPreflight(t *testing.T) {
	h, _ := composedCORSFixture(t)
	for _, tc := range []struct{ path, method string }{
		{"/v1/nonexistent", "GET"}, {"/v1/nonexistent", "POST"}, {"/v2/services", "GET"},
		{"/v1/services", "PUT"}, {"/v1/owners", "DELETE"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			rec := corsPreflight(h, tc.path, tc.method, corsTestOrigin)
			methods := commaFoldedTokens(rec.Header().Get("Access-Control-Allow-Methods"))
			if rec.Code != 204 || !methods[strings.ToLower(tc.method)] || !methods["options"] || rec.Header().Get("Access-Control-Allow-Origin") != corsTestOrigin {
				t.Errorf("preflight cannot read routing error: status=%d headers=%v", rec.Code, rec.Header())
			}
			if headers := commaFoldedTokens(rec.Header().Get("Access-Control-Allow-Headers")); !headers["content-type"] || !headers["authorization"] || len(headers) != 4 {
				t.Errorf("allowed headers changed: %v", headers)
			}
		})
	}
	extension := corsPreflight(h, "/v1/nonexistent", "CORS-NOT-ROUTED", corsTestOrigin)
	if commaFoldedTokens(extension.Header().Get("Access-Control-Allow-Methods"))["cors-not-routed"] {
		t.Error("unregistered extension method allowed")
	}
	untrusted := corsPreflight(h, "/v1/nonexistent", "POST", "https://untrusted.example")
	if got := untrusted.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("untrusted origin allowed: %q", got)
	}
}
