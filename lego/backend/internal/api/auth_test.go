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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeHydra serves POST /admin/oauth2/introspect: testToken is active (sub
// "client-1", with the given aud list, if any), everything else inactive. hits
// counts real introspections.
func fakeHydra(t *testing.T, hits *atomic.Int32, aud ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/oauth2/introspect" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		hits.Add(1)
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.PostFormValue("token") == testToken {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active": true, "sub": "client-1", "client_id": "client-1", "aud": aud,
			})
			return
		}
		_, _ = fmt.Fprint(w, `{"active":false}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakeHydraURL is the shorthand for tests that only need an introspection backend
// accepting testToken.
func fakeHydraURL(t *testing.T) string {
	var hits atomic.Int32
	return fakeHydra(t, &hits).URL
}

// fakeKratos serves GET /sessions/whoami: session token "live-session" or a
// cookie containing ory_kratos_session=live is a valid session for identity
// "identity-1"; anything else is 401.
func fakeKratos(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/whoami" {
			http.NotFound(w, r)
			return
		}
		ok := r.Header.Get("X-Session-Token") == "live-session" ||
			strings.Contains(r.Header.Get("Cookie"), "ory_kratos_session=live")
		if !ok {
			http.Error(w, `{"error":{"code":401}}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"identity":{"id":"identity-1"}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func brokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func deadServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

// echoIdentity is the probe behind the middleware: it writes what IdentityFrom
// sees so tests assert both the status and the resolved caller.
var echoIdentity = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if id, ok := core.IdentityFrom(r.Context()); ok {
		_, _ = fmt.Fprintf(w, "%s/%s", id.Method, id.Subject)
		return
	}
	_, _ = fmt.Fprint(w, "anonymous")
})

func TestAuthGate(t *testing.T) {
	var hits atomic.Int32
	hydra := fakeHydra(t, &hits)
	kratos := fakeKratos(t)

	cases := []struct {
		name       string
		hydraURL   string
		kratosURL  string
		bearer     string
		rawAuth    string
		sessionTok string
		cookie     string
		wantStatus int
		wantBody   string
	}{
		{name: "bearer-active", bearer: testToken, wantStatus: 200, wantBody: "oauth2/client-1"},
		{name: "bearer-inactive", bearer: "revoked", wantStatus: 401},
		{name: "no-credentials", wantStatus: 401},
		{name: "prefixless-authorization-header", rawAuth: testToken, wantStatus: 401},
		{name: "session-token", sessionTok: "live-session", wantStatus: 200, wantBody: "session/identity-1"},
		{name: "session-cookie", cookie: "ory_kratos_session=live", wantStatus: 200, wantBody: "session/identity-1"},
		{name: "session-expired", cookie: "ory_kratos_session=stale", wantStatus: 401},
		{name: "sessions-disabled", kratosURL: "off", cookie: "ory_kratos_session=live", wantStatus: 401},
		{name: "inactive-bearer-beats-valid-session", bearer: "revoked", cookie: "ory_kratos_session=live", wantStatus: 401},
		{name: "hydra-500", hydraURL: "broken", bearer: testToken, wantStatus: 503},
		{name: "hydra-down", hydraURL: "dead", bearer: testToken, wantStatus: 503},
		{name: "kratos-500", kratosURL: "broken", cookie: "ory_kratos_session=live", wantStatus: 503},
		{name: "kratos-down", kratosURL: "dead", cookie: "ory_kratos_session=live", wantStatus: 503},
	}

	upstream := func(t *testing.T, spec, healthy string) string {
		switch spec {
		case "off":
			return ""
		case "broken":
			return brokenServer(t).URL
		case "dead":
			return deadServer(t)
		default:
			return healthy
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{
				HydraAdminURL: upstream(t, tc.hydraURL, hydra.URL),
				KratosURL:     upstream(t, tc.kratosURL, kratos.URL),
			}
			auth, err := s.authMiddleware()
			if err != nil {
				t.Fatalf("authMiddleware: %v", err)
			}
			r := httptest.NewRequest(http.MethodGet, "/probe", nil)
			if tc.bearer != "" {
				r.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			if tc.rawAuth != "" {
				r.Header.Set("Authorization", tc.rawAuth)
			}
			if tc.sessionTok != "" {
				r.Header.Set("X-Session-Token", tc.sessionTok)
			}
			if tc.cookie != "" {
				r.Header.Set("Cookie", tc.cookie)
			}
			w := httptest.NewRecorder()
			auth(echoIdentity).ServeHTTP(w, r)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantBody != "" && w.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", w.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestIntrospectionCache(t *testing.T) {
	var hits atomic.Int32
	hydra := fakeHydra(t, &hits)
	mw := newOryAuth(hydra.URL, "", "", "", nil).middleware(echoIdentity)

	req := func(token string) int {
		r := httptest.NewRequest(http.MethodGet, "/probe", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, r)
		return w.Code
	}

	for i := range 2 {
		if code := req(testToken); code != 200 {
			t.Fatalf("live token request %d: status %d, want 200", i, code)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("introspections = %d, want 1 (positive cached)", got)
	}
	for i := range 2 {
		if code := req("revoked"); code != 401 {
			t.Fatalf("revoked token request %d: status %d, want 401", i, code)
		}
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("introspections = %d, want 3 (negatives not cached)", got)
	}
}

func TestWithCORS(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	do := func(origins, reqOrigin, method string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/graphql", nil)
		if reqOrigin != "" {
			req.Header.Set("Origin", reqOrigin)
		}
		rec := httptest.NewRecorder()
		withCORS(origins, ok).ServeHTTP(rec, req)
		return rec
	}
	const list = "https://dashboard.bex.co, http://localhost:5173"

	for _, origin := range []string{"https://dashboard.bex.co", "http://localhost:5173"} {
		rec := do(list, origin, http.MethodGet)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("Allow-Origin for %s = %q, want the origin echoed", origin, got)
		}
		if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Errorf("Allow-Credentials missing for %s", origin)
		}
	}
	// Unlisted origin: Vary only, no allow headers.
	rec := do(list, "https://evil.example", http.MethodGet)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin for unlisted origin = %q, want empty", got)
	}
	// Empty config: pure pass-through.
	rec = do("", "http://localhost:5173", http.MethodGet)
	for _, hd := range []string{"Access-Control-Allow-Origin", "Vary"} {
		if got := rec.Header().Get(hd); got != "" {
			t.Errorf("empty config set %s = %q, want unset", hd, got)
		}
	}
	// Preflight short-circuits with 204 and the echoed origin.
	rec = do(list, "http://localhost:5173", http.MethodOptions)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("preflight: code %d origin %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
