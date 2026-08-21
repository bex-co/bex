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

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWriteErrRedactsUnclassifiedInternalError proves an unclassified failure —
// a raw pgx/Kubernetes error carrying no domain-error sentinel — is returned as
// a generic 500 body, never spilling its internal text (constraint names,
// connection strings, paths). This is the security-audit run-1 hardening fix.
func TestWriteErrRedactsUnclassifiedInternalError(t *testing.T) {
	leaky := errors.New(`pq: duplicate key value violates unique constraint "tenants_email_key" DETAIL: host=10.0.0.5 dbname=bex`)

	rec := httptest.NewRecorder()
	WriteErr(rec, leaky)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["message"] != "internal error" || body["error"] != "internal error" {
		t.Fatalf("500 body leaked internal detail: %v", body)
	}
	if raw := rec.Body.String(); containsAny(raw, "constraint", "10.0.0.5", "dbname", "duplicate key") {
		t.Fatalf("500 body leaked internal error text: %s", raw)
	}
}

// TestWriteErrKeepsClassifiedAndCodedMessages proves the redaction does not
// swallow developer-authored messages: sentinel-classified errors keep their
// status and text, and CodedError keeps its full envelope.
func TestWriteErrKeepsClassifiedAndCodedMessages(t *testing.T) {
	t.Run("sentinel", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteErr(rec, fmt.Errorf("service %q: %w", "web", ErrNotFound))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["message"] == "internal error" {
			t.Fatalf("classified 404 was wrongly redacted: %v", body)
		}
	})

	t.Run("coded", func(t *testing.T) {
		rec := httptest.NewRecorder()
		WriteErr(rec, NewPaymentRequiredError())
		if rec.Code != http.StatusPaymentRequired {
			t.Fatalf("status = %d, want 402", rec.Code)
		}
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["message"] == "internal error" || body["message"] != PaymentRequiredMessage {
			t.Fatalf("coded 402 message wrong: %v", body)
		}
	})
}

func TestIsPublicError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"raw internal", errors.New("pq: boom"), false},
		{"wrapped raw internal", fmt.Errorf("query users: %w", errors.New("pq: boom")), false},
		{"not found", ErrNotFound, true},
		{"wrapped not found", fmt.Errorf("x: %w", ErrNotFound), true},
		{"bad request", ErrBadRequest, true},
		{"forbidden", ErrForbidden, true},
		{"conflict", ErrConflict, true},
		{"billing enforced", ErrBillingEnforced, true},
		{"payment required", ErrPaymentRequired, true},
		{"coded", NewPaymentRequiredError(), true},
	}
	for _, tc := range cases {
		if got := IsPublicError(tc.err); got != tc.want {
			t.Errorf("IsPublicError(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestMCPErrorRedaction proves the MCP surface shares the redaction policy: a
// nil error stays nil (success path), a coded error keeps its code+message, a
// sentinel keeps its message, and an unclassified internal error is replaced
// with a generic one.
func TestMCPErrorRedaction(t *testing.T) {
	if MCPError(nil) != nil {
		t.Fatal("MCPError(nil) must stay nil (success path)")
	}
	leaky := errors.New(`pq: constraint "x" host=10.0.0.5`)
	if got := MCPError(leaky); got == nil || got.Error() != "internal error" {
		t.Fatalf("unclassified MCP error not redacted: %v", got)
	}
	if got := MCPError(NewPaymentRequiredError()); got == nil || got.Error() == "internal error" {
		t.Fatalf("coded MCP error wrongly redacted: %v", got)
	}
	if got := MCPError(fmt.Errorf("svc: %w", ErrNotFound)); got == nil || got.Error() == "internal error" {
		t.Fatalf("sentinel MCP error wrongly redacted: %v", got)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
