/*
Copyright 2026 The bex Authors.

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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestErrorDialectIsRenderShaped pins w9/m38: the two highest-traffic error
// paths in the composition root — the auth gate's unauthenticated 401 and the
// GraphQL body-decode 400 — answer in the one Render-shaped dialect
// (Content-Type application/json + a `message` key), not text/plain bare
// `{"error"}`. The deploy-hook 405's counterpart lives in the deploys package.
func TestErrorDialectIsRenderShaped(t *testing.T) {
	assertRenderError := func(t *testing.T, w *httptest.ResponseRecorder, wantStatus int) {
		t.Helper()
		if w.Code != wantStatus {
			t.Fatalf("status = %d, want %d; body=%q", w.Code, wantStatus, w.Body.String())
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("body not JSON: %v (%q)", err, w.Body.String())
		}
		if _, ok := body["message"].(string); !ok || body["message"] == "" {
			t.Errorf("body missing non-empty string `message`: %v", body)
		}
	}

	t.Run("unauthenticated 401", func(t *testing.T) {
		mw := newOryAuth(fakeHydraURL(t), "", "", "", "", false, nil, nil, nil, "").middleware(echoIdentity)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/services", nil))
		assertRenderError(t, w, http.StatusUnauthorized)
		if w.Header().Get("WWW-Authenticate") == "" {
			t.Error("401 dropped its WWW-Authenticate challenge")
		}
	})

	t.Run("insufficient scope 403", func(t *testing.T) {
		h, _, _ := scopedAPI(t, "identity-1", "dcr-client", "openid "+core.ScopeRead,
			[]string{bexResource}, map[string]bool{"dcr-client": false})
		w := do(t, h, http.MethodPost, "/v1/services/web/suspend", testToken, "")
		assertRenderError(t, w, http.StatusForbidden)
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["code"] != core.InsufficientScopeCode {
			t.Errorf("code = %v, want %s", body["code"], core.InsufficientScopeCode)
		}
	})

	t.Run("graphql decode 400", func(t *testing.T) {
		srv := NewServer(&core.Base{Namespace: "default"}, Deps{})
		srv.HydraAdminURL = fakeHydraURL(t)
		h, err := srv.Handler()
		if err != nil {
			t.Fatalf("Handler: %v", err)
		}
		r := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader("{not json"))
		r.Header.Set("Authorization", "Bearer "+testToken)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assertRenderError(t, w, http.StatusBadRequest)
	})
}
