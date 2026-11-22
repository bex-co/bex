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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestRenderOperationsAnswerOnlyRendersErrorCodes pins w8/411: an error on one
// of Render's REST operations carries a code only from the pinned errorCode
// vocabulary, as Render's own error body does, so the pinned CLI prints
// `received response code 404: not found`, not `… 404 (NOT_FOUND): not found`.
// That holds for a refusal a middleware outside the request validator writes
// too. A code in the vocabulary still answers, and a bex-native route keeps
// bex's code for the bex CLI's own commands.
func TestRenderOperationsAnswerOnlyRendersErrorCodes(t *testing.T) {
	base := &core.Base{Client: (&admission{}).client(), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, _ := serverWith(t, base, Deps{})
	readOnly, _, _ := scopedAPI(t, "identity-1", "dcr-client", "openid "+core.ScopeRead, []string{bexResource}, map[string]bool{"dcr-client": false})
	for _, c := range []struct {
		h                  http.Handler
		name, method, path string
		status             int
		code               string // "" when the body must carry none
	}{
		{h, "an unknown service", http.MethodGet, "/v1/services/srv-doesnotexist000000000", http.StatusNotFound, ""},
		{h, "an unknown custom domain", http.MethodGet, "/v1/services/srv-doesnotexist000000000/custom-domains/shop.example.com", http.StatusNotFound, ""},
		{h, "a restart of an unknown service", http.MethodPost, "/v1/services/srv-doesnotexist000000000/restart", http.StatusNotFound, ""},
		{h, "a NUL in a Render operation's query", http.MethodGet, "/v1/services?name=%00", http.StatusBadRequest, ""},
		{readOnly, "a code in Render's vocabulary", http.MethodPost, "/v1/services/web/suspend", http.StatusForbidden, core.InsufficientScopeCode},
		{h, "a bex-native route", http.MethodGet, "/v1/git/connections?ownerId=default", http.StatusServiceUnavailable, "GITHUB_UNAVAILABLE"},
		{h, "a NUL in a bex-native route's query", http.MethodGet, "/v1/git/connections?ownerId=%00", http.StatusBadRequest, core.InvalidTextCode},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := do(t, c.h, c.method, c.path, testToken, "")
			var body map[string]any
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s %s answered %d %s: %v", c.method, c.path, res.Code, res.Body, err)
			}
			code, hasCode := body["code"]
			if res.Code != c.status || hasCode != (c.code != "") || (hasCode && code != c.code) {
				t.Fatalf("%s %s = %d %s, want %d with code %q", c.method, c.path, res.Code, res.Body, c.status, c.code)
			}
			if id, _ := body["id"].(string); id == "" || body["message"] == "" {
				t.Fatalf("%s %s = %s, want Render's id and message", c.method, c.path, res.Body)
			}
			if _, ok := body["params"]; ok && body["params"] == nil {
				t.Fatalf("%s %s = %s, want no params: null", c.method, c.path, res.Body)
			}
		})
	}
}
