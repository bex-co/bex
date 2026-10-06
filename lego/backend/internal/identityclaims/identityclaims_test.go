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

package identityclaims

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testToken = "ops-secret"

func serve(h *Handler, target, auth string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	Register(mux, h)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestIdentityClaimsAnswersClaims(t *testing.T) {
	var gotSubject string
	h := &Handler{Token: testToken, Identity: func(_ context.Context, s string) (Claims, bool) {
		gotSubject = s
		return Claims{Email: "ada@example.com", EmailVerified: true, Name: "Ada"}, true
	}}
	rec := serve(h, Path+"?subject=id-1", "Bearer "+testToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if gotSubject != "id-1" {
		t.Fatalf("lookup subject = %q", gotSubject)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"email": "ada@example.com", "email_verified": true, "name": "Ada"}
	if len(body) != len(want) {
		t.Fatalf("body = %v, want exactly %v", body, want)
	}
	for k, v := range want {
		if body[k] != v {
			t.Fatalf("body[%q] = %v, want %v", k, body[k], v)
		}
	}
}

func TestIdentityClaimsBearerGateRunsFirst(t *testing.T) {
	calls := 0
	h := &Handler{Token: testToken, Identity: func(context.Context, string) (Claims, bool) {
		calls++
		return Claims{Email: "a@example.com"}, true
	}}
	for _, auth := range []string{"", "Bearer wrong", testToken} {
		if rec := serve(h, Path+"?subject=id-1", auth); rec.Code != http.StatusUnauthorized {
			t.Fatalf("auth %q: status = %d, want 401", auth, rec.Code)
		}
	}
	if calls != 0 {
		t.Fatalf("identity looked up %d times before the bearer gate", calls)
	}
}

func TestIdentityClaimsFailsClosed(t *testing.T) {
	cases := map[string]Lookup{
		"unwired": nil,
		"miss":    func(context.Context, string) (Claims, bool) { return Claims{}, false },
		"noemail": func(context.Context, string) (Claims, bool) { return Claims{Name: "x"}, true },
	}
	for name, lookup := range cases {
		h := &Handler{Token: testToken, Identity: lookup}
		if rec := serve(h, Path+"?subject=id-1", "Bearer "+testToken); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status = %d, want 503", name, rec.Code)
		}
	}
	h := &Handler{Token: testToken, Identity: cases["miss"]}
	if rec := serve(h, Path, "Bearer "+testToken); rec.Code != http.StatusBadRequest {
		t.Errorf("missing subject: status = %d, want 400", rec.Code)
	}
}

func TestIdentityClaimsUnregisteredWithoutToken(t *testing.T) {
	h := &Handler{Identity: func(context.Context, string) (Claims, bool) { return Claims{Email: "a@example.com"}, true }}
	if rec := serve(h, Path+"?subject=id-1", "Bearer "); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (route absent)", rec.Code)
	}
	if rec := serve(nil, Path+"?subject=id-1", "Bearer "); rec.Code != http.StatusNotFound {
		t.Fatalf("nil handler: status = %d, want 404", rec.Code)
	}
}
