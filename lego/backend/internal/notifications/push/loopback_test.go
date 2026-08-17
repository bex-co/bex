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

package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestLoopbackFakeProviderRoundTripUsesTheDefaultClient exercises the one path
// the rest of expo_test.go deliberately bypasses: docs/runbooks/mobile-push.md's
// loopback affordance (BEX_EXPO_PUSH_URL=http://127.0.0.1:PORT) driven through
// the transport's OWN http.Client rather than an injected one.
//
// Every other test injects WithHTTPClient(server.Client()) because httptest's
// TLS server needs its self-signed cert trusted — which also substitutes the
// client the production path builds. This is how w3/m78's live crash leg was
// verified against a fake Expo, so the affordance is real operator surface: it
// must keep working without a test-only client, and it must survive the plain
// HTTP that a loopback fake serves. Configuration is read through os.Getenv to
// mirror cmd/api/main.go's construction exactly.
func TestLoopbackFakeProviderRoundTripUsesTheDefaultClient(t *testing.T) {
	var sends, receipts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer loopback-fake-token" {
			t.Errorf("Authorization = %q, want the configured credential", got)
		}
		switch r.URL.Path {
		case "/send":
			sends++
			var got map[string]any
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode send: %v", err)
			}
			if got["to"] != testToken {
				t.Errorf("to = %v, want %v", got["to"], testToken)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"status": "ok", "id": "loopback-ticket-1"},
			})
		case "/getReceipts":
			receipts++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"loopback-ticket-1": map[string]any{"status": "ok"}},
			})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	// httptest.NewServer binds a loopback address, which is exactly what
	// validateEndpoint's http:// carve-out admits; a non-loopback http:// URL
	// would be refused here rather than silently downgrading to cleartext.
	t.Setenv("BEX_PUSH_PROVIDER", ProviderExpo)
	t.Setenv("BEX_EXPO_PUSH_ACCESS_TOKEN", "loopback-fake-token")
	t.Setenv("BEX_EXPO_PUSH_URL", server.URL)

	transport, err := New(Config{
		Provider:    os.Getenv("BEX_PUSH_PROVIDER"),
		AccessToken: os.Getenv("BEX_EXPO_PUSH_ACCESS_TOKEN"),
		Endpoint:    os.Getenv("BEX_EXPO_PUSH_URL"),
	})
	if err != nil {
		t.Fatalf("New() from environment: %v", err)
	}
	// main.go derives Deps.PushAvailable — and therefore the dashboard's
	// "push is not configured" banner — from exactly this nil check.
	if transport == nil {
		t.Fatal("New() = nil transport, want the configured environment to enable push")
	}

	ticket, err := transport.Send(context.Background(), Message{
		Token: testToken,
		Title: "Deploy failed",
		Body:  "Open the service for evidence.",
		Data: EnvelopeData{
			Schema: "bex.notification.v1", NotificationID: "evt-loopback",
			Event: "deploy_failed", Route: "/services/srv-1",
		},
	})
	if err != nil {
		t.Fatalf("Send(): %v", err)
	}
	if ticket.ID != "loopback-ticket-1" {
		t.Fatalf("ticket = %q, want loopback-ticket-1", ticket.ID)
	}

	got, err := transport.CheckReceipts(context.Background(), []string{ticket.ID})
	if err != nil {
		t.Fatalf("CheckReceipts(): %v", err)
	}
	receipt, ok := got[ticket.ID]
	if !ok || receipt.Err != nil {
		t.Fatalf("receipt = %+v (present %v), want a delivered receipt", receipt, ok)
	}
	if sends != 1 || receipts != 1 {
		t.Fatalf("provider calls = %d send / %d receipt, want 1 / 1", sends, receipts)
	}
}

// TestDisabledProviderMakesNoProviderCall pins the honest disabled state the
// dashboard banner reports: with BEX_PUSH_PROVIDER unset, no transport is
// constructed and the configured endpoint is never dialed, even when a
// credential and URL are present in the environment.
func TestDisabledProviderMakesNoProviderCall(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer server.Close()

	t.Setenv("BEX_PUSH_PROVIDER", "")
	t.Setenv("BEX_EXPO_PUSH_ACCESS_TOKEN", "loopback-fake-token")
	t.Setenv("BEX_EXPO_PUSH_URL", server.URL)

	transport, err := New(Config{
		Provider:    os.Getenv("BEX_PUSH_PROVIDER"),
		AccessToken: os.Getenv("BEX_EXPO_PUSH_ACCESS_TOKEN"),
		Endpoint:    os.Getenv("BEX_EXPO_PUSH_URL"),
	})
	if err != nil || transport != nil {
		t.Fatalf("New() = (%v, %v), want (nil, nil) for an unset provider", transport, err)
	}
	if calls != 0 {
		t.Fatalf("provider calls = %d, want 0", calls)
	}
}
