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

const corsTestOrigin = "https://dashboard.bex.co"

func composedCORSFixture(t *testing.T) (http.Handler, *http.ServeMux) {
	t.Helper()
	srv := matrixServer(t)
	srv.CORSOrigin = corsTestOrigin
	muxes, err := srv.composedMuxes()
	if err != nil {
		t.Fatalf("composedMuxes: %v", err)
	}
	return srv.wrapMuxes(muxes), muxes.rest
}

func corsPreflight(handler http.Handler, path, method, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodOptions, path, nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", method)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func commaFoldedTokens(value string) map[string]bool {
	tokens := map[string]bool{}
	for _, token := range strings.Split(value, ",") {
		if token = strings.TrimSpace(token); token != "" {
			tokens[strings.ToLower(token)] = true
		}
	}
	return tokens
}

// corsCoverageGaps is the structural drift guard. The route inventory comes
// from the populated mux itself; no verb list is restated here.
func corsCoverageGaps(handler http.Handler, rest *http.ServeMux) (gaps []string, counts map[string]int) {
	counts = map[string]int{}
	for _, pattern := range serveMuxPatterns(rest) {
		method, path := splitPattern(pattern)
		if method == "" {
			continue // A method-less pattern already accepts every requested verb.
		}
		counts[method]++
		path = fillWildcards(path, "cors-fixture")
		rec := corsPreflight(handler, path, method, corsTestOrigin)
		methods := commaFoldedTokens(rec.Header().Get("Access-Control-Allow-Methods"))
		if rec.Code != http.StatusNoContent || !methods[strings.ToLower(method)] || !methods[strings.ToLower(http.MethodOptions)] {
			gaps = append(gaps, pattern)
		}
	}
	return gaps, counts
}

func TestCORSPreflightCoversEveryRegisteredRESTMethod(t *testing.T) {
	handler, rest := composedCORSFixture(t)
	patterns := serveMuxPatterns(rest)
	if len(patterns) < 150 {
		t.Fatalf("route enumeration found only %d patterns; ServeMux tree walk may have broken", len(patterns))
	}
	gaps, counts := corsCoverageGaps(handler, rest)
	if len(gaps) > 0 {
		t.Fatalf("preflight does not advertise registered routes: %s", strings.Join(gaps, ", "))
	}
	for _, method := range []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	} {
		if counts[method] == 0 {
			t.Errorf("assembled REST mux has no %s routes; coverage guard is incomplete", method)
		}
	}

	// net/http routes HEAD through a registered GET pattern; CORS must consult
	// that same matcher instead of accidentally treating HEAD as unregistered.
	rec := corsPreflight(handler, "/v1/services/cors-fixture", http.MethodHead, corsTestOrigin)
	if methods := commaFoldedTokens(rec.Header().Get("Access-Control-Allow-Methods")); !methods[strings.ToLower(http.MethodHead)] {
		t.Errorf("HEAD preflight methods = %q, want HEAD from GET's implicit route", rec.Header().Get("Access-Control-Allow-Methods"))
	}
}

// Guard the guard: if the handler serves a newly registered verb but CORS is
// accidentally wired to an older router, the structural check names that route.
func TestCORSCoverageGuardDetectsAStaleRouter(t *testing.T) {
	servedRest := http.NewServeMux()
	servedRest.HandleFunc("GET /v1/probe", func(http.ResponseWriter, *http.Request) {})
	servedRest.HandleFunc("BREW /v1/probe", func(http.ResponseWriter, *http.Request) {})
	staleRest := http.NewServeMux()
	staleRest.HandleFunc("GET /v1/probe", func(http.ResponseWriter, *http.Request) {})
	staleRoot := http.NewServeMux()
	staleRoot.Handle("/v1/", staleRest)
	routes := corsRoutes{
		root: staleRoot,
		delegated: map[string]*http.ServeMux{
			"/v1/": staleRest,
		},
	}
	handler := withCORS(corsTestOrigin, routes)

	gaps, _ := corsCoverageGaps(handler, servedRest)
	if len(gaps) != 1 || gaps[0] != "BREW /v1/probe" {
		t.Fatalf("coverage gaps = %v, want the newly registered BREW route", gaps)
	}
}

func TestCORSPreflightPolicyAndPreviouslyBlockedMethods(t *testing.T) {
	rest := http.NewServeMux()
	root := http.NewServeMux()
	root.Handle("/v1/", rest)
	routes := corsRoutes{
		root: root,
		delegated: map[string]*http.ServeMux{
			"/v1/": rest,
		},
	}

	tests := []struct {
		method  string
		pattern string
		path    string
	}{
		{http.MethodGet, "GET /v1/services/{id}", "/v1/services/srv-cors"},
		{http.MethodPost, "POST /v1/services", "/v1/services"},
		{http.MethodDelete, "DELETE /v1/services/{id}/custom-domains/{name}", "/v1/services/srv-cors/custom-domains/example.test"},
		{http.MethodDelete, "DELETE /v1/postgres/{id}", "/v1/postgres/dpg-cors"},
		{http.MethodPatch, "PATCH /v1/services/{id}", "/v1/services/srv-cors"},
		{http.MethodPut, "PUT /v1/projects/{id}/service-links", "/v1/projects/prj-cors/service-links"},
	}
	for _, tc := range tests {
		rest.HandleFunc(tc.pattern, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != tc.method {
				t.Errorf("handler method = %s, want %s", r.Method, tc.method)
			}
			w.WriteHeader(http.StatusAccepted)
		})
	}

	var idempotencyKey string
	rest.HandleFunc("POST /v1/webhooks/{id}/events/{attemptId}/resend", func(w http.ResponseWriter, r *http.Request) {
		idempotencyKey = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusAccepted)
	})
	handler := withCORS(corsTestOrigin, routes)

	for _, tc := range tests {
		rec := corsPreflight(handler, tc.path, tc.method, corsTestOrigin)
		methods := commaFoldedTokens(rec.Header().Get("Access-Control-Allow-Methods"))
		if rec.Code != http.StatusNoContent || !methods[strings.ToLower(tc.method)] || !methods[strings.ToLower(http.MethodOptions)] {
			t.Errorf("%s %s preflight = %d methods %q", tc.method, tc.path, rec.Code, rec.Header().Get("Access-Control-Allow-Methods"))
			continue
		}
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.Header.Set("Origin", corsTestOrigin)
		actual := httptest.NewRecorder()
		handler.ServeHTTP(actual, req)
		if actual.Code != http.StatusAccepted {
			t.Errorf("%s %s actual status = %d, want %d", tc.method, tc.path, actual.Code, http.StatusAccepted)
		}
	}

	resendPath := "/v1/webhooks/wh-cors/events/attempt-cors/resend"
	preflight := corsPreflight(handler, resendPath, http.MethodPost, corsTestOrigin)
	headers := commaFoldedTokens(preflight.Header().Get("Access-Control-Allow-Headers"))
	for _, required := range []string{"Authorization", "Content-Type", "Idempotency-Key", "X-Session-Token"} {
		if !headers[strings.ToLower(required)] {
			t.Errorf("Allow-Headers = %q, missing %s", preflight.Header().Get("Access-Control-Allow-Headers"), required)
		}
	}
	if len(headers) != 4 {
		t.Errorf("Allow-Headers = %q, want exactly the four reviewed browser inputs", preflight.Header().Get("Access-Control-Allow-Headers"))
	}
	for _, excluded := range []string{"Last-Event-ID", "X-Api-Key"} {
		if headers[strings.ToLower(excluded)] {
			t.Errorf("Allow-Headers = %q, unexpectedly includes %s", preflight.Header().Get("Access-Control-Allow-Headers"), excluded)
		}
	}
	if got := preflight.Header().Get("Access-Control-Max-Age"); got != corsMaxAge {
		t.Errorf("Max-Age = %q, want %s", got, corsMaxAge)
	}
	vary := commaFoldedTokens(strings.Join(preflight.Header().Values("Vary"), ","))
	for _, required := range []string{"Origin", "Access-Control-Request-Method"} {
		if !vary[strings.ToLower(required)] {
			t.Errorf("Vary = %q, missing %s", preflight.Header().Values("Vary"), required)
		}
	}

	req := httptest.NewRequest(http.MethodPost, resendPath, nil)
	req.Header.Set("Origin", corsTestOrigin)
	req.Header.Set("Idempotency-Key", "resend-cors-0001")
	actual := httptest.NewRecorder()
	handler.ServeHTTP(actual, req)
	if actual.Code != http.StatusAccepted || idempotencyKey != "resend-cors-0001" {
		t.Errorf("idempotent resend status=%d handler key=%q", actual.Code, idempotencyKey)
	}

	unknown := corsPreflight(handler, "/v1/services/srv-cors", "CORS-NOT-ROUTED", corsTestOrigin)
	if methods := commaFoldedTokens(unknown.Header().Get("Access-Control-Allow-Methods")); methods["cors-not-routed"] {
		t.Errorf("unrouted method advertised: %q", unknown.Header().Get("Access-Control-Allow-Methods"))
	}
}
