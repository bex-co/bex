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
	"testing"

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

func TestRESTUnconfigured503(t *testing.T) {
	// No client, no store => every git-connect route 503.
	m := mux(&Service{Base: &core.Base{Namespace: "default"}})
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
	rec = do(t, m, "GET", "/v1/git/callback?installation_id=42")
	if rec.Code != http.StatusFound {
		t.Fatalf("callback => %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://dash.bex.co/settings" {
		t.Errorf("callback redirect = %q", loc)
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
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{login: "octo"}, Store: newFakeStore()}
	m := mux(svc)
	for _, path := range []string{"/v1/git/callback", "/v1/git/callback?installation_id=abc", "/v1/git/callback?installation_id=0", "/v1/git/callback?installation_id=-5"} {
		if rec := do(t, m, "GET", path); rec.Code != http.StatusBadRequest {
			t.Errorf("callback %s => %d, want 400", path, rec.Code)
		}
	}
}

func TestRESTCallbackForgedInstallationRejected(t *testing.T) {
	// A forged installation_id GitHub can't authenticate => 400, nothing recorded.
	st := newFakeStore()
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{installErr: &APIError{Status: 404, Body: "Not Found"}}, Store: st}
	m := mux(svc)
	if rec := do(t, m, "GET", "/v1/git/callback?installation_id=777"); rec.Code != http.StatusBadRequest {
		t.Fatalf("forged callback => %d, want 400", rec.Code)
	}
	if len(st.conns) != 0 {
		t.Error("forged installation must not be recorded")
	}
}

func TestRESTCallbackNoDashboardReturnsJSON(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default"}, GitHub: &fakeClient{login: "octo"}, Store: newFakeStore()}
	m := mux(svc)
	rec := do(t, m, "GET", "/v1/git/callback?installation_id=42")
	if rec.Code != http.StatusOK {
		t.Fatalf("callback without dashboard => %d, want 200", rec.Code)
	}
	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "connected" {
		t.Errorf("body = %v", body)
	}
}
