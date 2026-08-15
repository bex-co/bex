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

package members

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestRESTUnconfiguredStoreIs503 is the REST-surface companion to
// TestVerbsUnavailableWithoutStore, which only proved the SERVICE returns
// ErrMembersUnavailable. The adapter then mistranslated it: the sentinel was a
// plain errors.New that core.WriteErr's hand-maintained unavailable list never
// named, so a bex-api running without BEX_CP_DB_URI answered 500 — "something
// broke, retry" — for what is a stable configuration state. Declaring it with
// core.Unavailable makes the surface agree with the verb.
func TestRESTUnconfiguredStoreIs503(t *testing.T) {
	svc := &Service{Base: &core.Base{}} // Store nil, authz nil (allow)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces/tea-1/members", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxWith("user-a")))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode REST body: %v", err)
	}
	if body["id"] != "unavailable" || body["message"] != ErrMembersUnavailable.Error() {
		t.Fatalf("REST body = %#v, want the Render unavailable envelope", body)
	}
}
