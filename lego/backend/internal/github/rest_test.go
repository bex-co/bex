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

package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func mux(s *Service) *http.ServeMux {
	m := http.NewServeMux()
	s.RegisterREST(m)
	return m
}

func do(t *testing.T, m *http.ServeMux, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)
	return rec
}

func callbackPath(t *testing.T, s *Service, installationID, workspaceID string) string {
	t.Helper()
	state, err := s.mintConnectState(workspaceID)
	if err != nil {
		t.Fatalf("mint callback state: %v", err)
	}
	return "/v1/git/callback?installation_id=" + url.QueryEscape(installationID) + "&state=" + url.QueryEscape(state) + "&code=oauth-code"
}

func TestRESTUnconfigured503(t *testing.T) {
	// No client, no store => every git-connect route 503.
	// DashboardURL must not turn the optional-feature contract into a redirect.
	m := mux(&Service{Base: &core.Base{Namespace: "default"}, DashboardURL: "https://dash.bex.co"})
	for _, tc := range []struct{ method, path string }{
		{"POST", "/v1/git/connect"},
		{"GET", "/v1/git/callback?installation_id=1"},
		{"GET", "/v1/git/connection"},
		{"DELETE", "/v1/git/connection"},
		{"GET", "/v1/repos"},
	} {
		rec := do(t, m, tc.method, tc.path)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s => %d, want 503", tc.method, tc.path, rec.Code)
		}
	}
}

func TestRESTHappyPath(t *testing.T) {
	svc := &Service{
		Base:         &core.Base{Namespace: "default"},
		GitHub:       &fakeClient{login: "octo", repos: []Repo{{ID: 1, FullName: "octo/pub"}, {ID: 2, FullName: "octo/priv", Private: true}}},
		Store:        newFakeStore(),
		Verifier:     &fakeVerifier{ok: true},
		StateSecret:  []byte("test-only-high-entropy-state-secret"),
		DashboardURL: "https://dash.bex.co/",
	}
	m := mux(svc)

	// connect returns the install URL.
	rec := do(t, m, "POST", "/v1/git/connect")
	if rec.Code != http.StatusOK {
		t.Fatalf("connect => %d", rec.Code)
	}
	var conn Connection
	json.Unmarshal(rec.Body.Bytes(), &conn)
	if conn.InstallURL == "" {
		t.Error("connect should return install url")
	}

	// callback records + redirects to the dashboard settings page.
	installURL, err := url.Parse(conn.InstallURL)
	if err != nil {
		t.Fatal(err)
	}
	state := installURL.Query().Get("state")
	if state == "" {
		t.Fatal("connect install URL has no state")
	}
	rec = do(t, m, "GET", "/v1/git/callback?installation_id=42&state="+url.QueryEscape(state)+"&code=oauth-code")
	if rec.Code != http.StatusFound {
		t.Fatalf("callback => %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://dash.bex.co/settings" {
		t.Errorf("callback redirect = %q", loc)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("callback Referrer-Policy = %q, want no-referrer", got)
	}

	// connection now shows connected with the account login.
	rec = do(t, m, "GET", "/v1/git/connection")
	json.Unmarshal(rec.Body.Bytes(), &conn)
	if !conn.Connected || conn.AccountLogin != "octo" {
		t.Errorf("connection = %+v", conn)
	}

	// repos include the private one.
	rec = do(t, m, "GET", "/v1/repos")
	var repos []Repo
	json.Unmarshal(rec.Body.Bytes(), &repos)
	if len(repos) != 2 || !repos[1].Private {
		t.Errorf("repos = %+v", repos)
	}

	// disconnect => 204, then repos empty.
	rec = do(t, m, "DELETE", "/v1/git/connection")
	if rec.Code != http.StatusNoContent {
		t.Errorf("disconnect => %d, want 204", rec.Code)
	}
	rec = do(t, m, "GET", "/v1/repos")
	json.Unmarshal(rec.Body.Bytes(), &repos)
	if len(repos) != 0 {
		t.Errorf("repos after disconnect = %+v", repos)
	}
}

func TestRESTCallbackBadInstallationID(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{login: "octo"}, Store: newFakeStore(), StateSecret: []byte("test-only-high-entropy-state-secret")}
	m := mux(svc)
	state, err := svc.mintConnectState(core.DefaultTenant)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/v1/git/callback?state=" + url.QueryEscape(state),
		"/v1/git/callback?installation_id=abc&state=" + url.QueryEscape(state),
		"/v1/git/callback?installation_id=0&state=" + url.QueryEscape(state),
		"/v1/git/callback?installation_id=-5&state=" + url.QueryEscape(state),
	} {
		if rec := do(t, m, "GET", path); rec.Code != http.StatusBadRequest {
			t.Errorf("callback %s => %d, want 400", path, rec.Code)
		}
	}
}

func TestRESTCallbackForgedInstallationRejected(t *testing.T) {
	// A forged installation_id GitHub can't authenticate => 400, nothing recorded.
	st := newFakeStore()
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{installErr: &APIError{Status: 404, Body: "Not Found"}}, Store: st, Verifier: &fakeVerifier{ok: true}, StateSecret: []byte("test-only-high-entropy-state-secret")}
	m := mux(svc)
	if rec := do(t, m, "GET", callbackPath(t, svc, "777", core.DefaultTenant)); rec.Code != http.StatusBadRequest {
		t.Fatalf("forged callback => %d, want 400", rec.Code)
	}
	if len(st.conns) != 0 {
		t.Error("forged installation must not be recorded")
	}
}

func TestRESTCallbackNoDashboardReturnsJSON(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{login: "octo"}, Store: newFakeStore(), Verifier: &fakeVerifier{ok: true}, StateSecret: []byte("test-only-high-entropy-state-secret")}
	m := mux(svc)
	rec := do(t, m, "GET", callbackPath(t, svc, "42", core.DefaultTenant))
	if rec.Code != http.StatusOK {
		t.Fatalf("callback without dashboard => %d, want 200", rec.Code)
	}
	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "connected" {
		t.Errorf("body = %v", body)
	}
}

func TestRESTCallbackStateFailuresRedirectToDashboard(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	st := newFakeStore()
	svc := &Service{
		Base:         &core.Base{Namespace: "default", Clock: func() time.Time { return now }},
		GitHub:       &fakeClient{login: "octo"},
		Store:        st,
		StateSecret:  []byte("test-only-high-entropy-state-secret"),
		DashboardURL: "https://dash.bex.co",
	}
	expired, err := svc.mintConnectState(core.DefaultTenant)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(connectStateTTL)

	for _, tc := range []struct {
		name, path, code string
	}{
		{name: "missing", path: "/v1/git/callback?installation_id=42", code: "missing_state"},
		{name: "invalid", path: "/v1/git/callback?installation_id=42&state=not-a-token", code: "invalid_state"},
		{name: "expired", path: "/v1/git/callback?installation_id=42&state=" + url.QueryEscape(expired), code: "expired_state"},
		{name: "github", path: "/v1/git/callback?error=access_denied", code: "github_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, mux(svc), "GET", tc.path)
			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302; body=%s", rec.Code, rec.Body.String())
			}
			want := "https://dash.bex.co/settings?git_error=" + tc.code
			if got := rec.Header().Get("Location"); got != want {
				t.Fatalf("Location = %q, want %q", got, want)
			}
			if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
				t.Fatalf("Referrer-Policy = %q, want no-referrer", got)
			}
		})
	}
	if len(st.conns) != 0 {
		t.Fatalf("failed callbacks recorded connections: %+v", st.conns)
	}
}

func TestRESTCallbackStateFailuresReturnClearJSONWithoutDashboard(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	svc := &Service{
		Base:        &core.Base{Namespace: "default", Clock: func() time.Time { return now }},
		GitHub:      &fakeClient{login: "octo"},
		Store:       newFakeStore(),
		StateSecret: []byte("test-only-high-entropy-state-secret"),
	}
	expired, err := svc.mintConnectState(core.DefaultTenant)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(connectStateTTL)

	for _, tc := range []struct {
		name, path, code string
	}{
		{name: "missing", path: "/v1/git/callback?installation_id=42", code: "missing_state"},
		{name: "invalid", path: "/v1/git/callback?installation_id=42&state=not-a-token", code: "invalid_state"},
		{name: "expired", path: "/v1/git/callback?installation_id=42&state=" + url.QueryEscape(expired), code: "expired_state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, mux(svc), "GET", tc.path)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["code"] != tc.code || body["error"] == "" {
				t.Fatalf("body = %v, want code %q and a clear error", body, tc.code)
			}
		})
	}
}

func TestRESTCallbackRecordsStateWorkspace(t *testing.T) {
	st := newFakeStore()
	svc := &Service{
		Base:        &core.Base{Namespace: "default"},
		GitHub:      &fakeClient{login: "octo"},
		Store:       st,
		Verifier:    &fakeVerifier{ok: true},
		StateSecret: []byte("test-only-high-entropy-state-secret"),
	}
	rec := do(t, mux(svc), "GET", callbackPath(t, svc, "42", "tea-target"))
	if rec.Code != http.StatusOK {
		t.Fatalf("callback status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := st.conns["tea-target"]; got.InstallationID != 42 || got.AccountLogin != "octo" {
		t.Fatalf("target workspace connection = %+v", got)
	}
	if _, ok := st.conns[core.DefaultTenant]; ok {
		t.Fatal("state callback recorded against the default workspace")
	}
}

func TestRESTCallbackRejectsAuthenticatedRequestWithoutState(t *testing.T) {
	st := newFakeStore()
	svc := &Service{
		Base:         &core.Base{Namespace: "default"},
		GitHub:       &fakeClient{login: "octo"},
		Store:        st,
		DashboardURL: "https://dash.bex.co",
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/git/callback?installation_id=42", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "agent", Method: "oauth2"}))
	rec := httptest.NewRecorder()
	mux(svc).ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("authenticated callback status = %d, want 302; body=%s", rec.Code, rec.Body.String())
	}
	if len(st.conns) != 0 {
		t.Fatalf("authenticated no-state callback recorded a connection: %+v", st.conns)
	}
	if got := rec.Header().Get("Location"); got != "https://dash.bex.co/settings?git_error=missing_state" {
		t.Fatalf("authenticated callback redirect = %q", got)
	}
}
