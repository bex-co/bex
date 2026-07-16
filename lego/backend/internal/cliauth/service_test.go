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

package cliauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func renderRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	// The official CLI labels its JSON body as form data; the adapter must honor
	// the actual bytes, not trust this misleading header.
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestRenderProtocolAdapters(t *testing.T) {
	t.Helper()
	var calls []url.Values
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse upstream form: %v", err)
		}
		calls = append(calls, r.Form)
		switch {
		case r.URL.Path == "/oauth2/device/auth":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"device_code": "device-1", "user_code": "ABCDEF",
				"verification_uri":          "https://dashboard.bex.co/auth/device",
				"verification_uri_complete": "https://dashboard.bex.co/auth/device?user_code=ABCDEF",
				"expires_in":                600, "interval": 5,
			})
		case r.URL.Path == "/oauth2/token" && r.Form.Get("grant_type") == deviceGrantType:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-1", "refresh_token": "refresh-1",
				"token_type": "bearer", "expires_in": 900,
			})
		case r.URL.Path == "/oauth2/token" && r.Form.Get("grant_type") == "refresh_token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-2", "refresh_token": "refresh-2",
				"token_type": "bearer", "expires_in": 900,
			})
		default:
			http.Error(w, "unexpected upstream request", http.StatusBadRequest)
		}
	}))
	defer upstream.Close()

	svc := New(upstream.URL, "", nil)
	mux := http.NewServeMux()
	svc.RegisterPublic(mux)

	cases := []struct {
		path string
		body string
		want string
	}{
		{"/v1/device-grant", `{"client_id":"` + RenderCLIClientID + `"}`, `"device_code":"device-1"`},
		{"/v1/device-token", `{"grant_type":"` + deviceGrantType + `","client_id":"` + RenderCLIClientID + `","device_code":"device-1"}`, `"refresh_token":"refresh-1"`},
		{"/v1/token/refresh/", `{"grant_type":"refresh_token","refresh_token":"refresh-1"}`, `"refresh_token":"refresh-2"`},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, renderRequest(http.MethodPost, tc.path, tc.body))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s => %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if len(calls) != 3 {
		t.Fatalf("upstream calls = %d, want 3", len(calls))
	}
	if calls[0].Get("client_id") != RenderCLIClientID || calls[0].Get("scope") != "openid offline_access" {
		t.Errorf("device grant form = %v", calls[0])
	}
	if calls[1].Get("device_code") != "device-1" || calls[2].Get("client_id") != RenderCLIClientID {
		t.Errorf("token forms = %v / %v", calls[1], calls[2])
	}
}

func TestRenderProtocolRejectsWrongClientBeforeHydra(t *testing.T) {
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer upstream.Close()
	svc := New(upstream.URL, "", nil)
	mux := http.NewServeMux()
	svc.RegisterPublic(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, renderRequest(http.MethodPost, "/v1/device-grant", `{"client_id":"attacker"}`))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid_client") || called {
		t.Fatalf("wrong client => %d %q called=%v", rec.Code, rec.Body.String(), called)
	}
}

type fakeRevoker struct {
	id  string
	err error
}

func (f *fakeRevoker) RevokeAPIKey(_ context.Context, _ string, id string) error {
	f.id = id
	return f.err
}

func TestLogoutRevokesHumanConsentChainAndKeepsSharedClient(t *testing.T) {
	var subject, client string
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/admin/oauth2/auth/sessions/consent" {
			t.Fatalf("unexpected admin request %s %s", r.Method, r.URL.Path)
		}
		subject, client = r.URL.Query().Get("subject"), r.URL.Query().Get("client")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer admin.Close()

	svc := New("", admin.URL, nil)
	invalidated := ""
	var invalidatedIdentity core.Identity
	h := svc.RevokeHandler(func(token string, identity core.Identity) {
		invalidated = token
		invalidatedIdentity = identity
	})
	id := core.Identity{Subject: "kratos-user-a", Method: "oauth2", ClientID: RenderCLIClientID, Human: true}
	req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
	req.Header.Set("Authorization", "Bearer access-a")
	req = req.WithContext(core.WithIdentity(req.Context(), id))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke => %d %s", rec.Code, rec.Body.String())
	}
	if subject != "kratos-user-a" || client != RenderCLIClientID || invalidated != "access-a" || invalidatedIdentity != id {
		t.Fatalf("subject=%q client=%q invalidated=%q identity=%+v", subject, client, invalidated, invalidatedIdentity)
	}
}

func TestLogoutRetainsAPIKeySelfRevoke(t *testing.T) {
	revoker := &fakeRevoker{}
	svc := New("", "", revoker)
	h := svc.RevokeHandler(nil)
	id := core.Identity{Subject: "api-key-1", Method: "oauth2", ClientID: "api-key-1"}
	req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), id))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || revoker.id != "api-key-1" {
		t.Fatalf("revoke => %d id=%q body=%s", rec.Code, revoker.id, rec.Body.String())
	}
}

func TestLogoutFailsClosed(t *testing.T) {
	t.Run("no identity", func(t *testing.T) {
		rec := httptest.NewRecorder()
		New("", "", nil).RevokeHandler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("admin unavailable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
		id := core.Identity{Subject: "user", Method: "oauth2", ClientID: RenderCLIClientID, Human: true}
		req = req.WithContext(core.WithIdentity(req.Context(), id))
		New("", "", nil).RevokeHandler(nil).ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
		}
	})
	t.Run("api key error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
		req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "key", Method: "oauth2", ClientID: "key"}))
		New("", "", &fakeRevoker{err: errors.New("boom")}).RevokeHandler(nil).ServeHTTP(rec, req)
		if rec.Code == http.StatusNoContent {
			t.Fatal("unexpected success")
		}
	})
	t.Run("other human oauth client is not treated as an api key", func(t *testing.T) {
		revoker := &fakeRevoker{}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/oauth/revoke", nil)
		req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{
			Subject: "human", Method: "oauth2", ClientID: "other-public-client", Human: true,
		}))
		New("", "", revoker).RevokeHandler(nil).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || revoker.id != "" {
			t.Fatalf("status = %d revoked=%q body=%s", rec.Code, revoker.id, rec.Body.String())
		}
	})
}
