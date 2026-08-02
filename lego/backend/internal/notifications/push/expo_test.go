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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "ExpoPushToken[abcdefghijklmnop]"

func TestNewDisabledCreatesNothing(t *testing.T) {
	optionCalls := 0
	transport, err := New(Config{
		AccessToken: "malformed\nsecret",
		Endpoint:    "http://not-loopback.example",
	}, func(*expoOptions) { optionCalls++ })
	if err != nil {
		t.Fatalf("New(disabled): %v", err)
	}
	if transport != nil {
		t.Fatal("disabled transport must be nil")
	}
	if optionCalls != 0 {
		t.Fatalf("disabled configuration constructed dependencies: option calls = %d", optionCalls)
	}
}

func TestNewRejectsMalformedConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{"unknown provider", Config{Provider: "apns", AccessToken: "secret"}},
		{"missing credential", Config{Provider: ProviderExpo}},
		{"credential newline", Config{Provider: ProviderExpo, AccessToken: "secret\nleak"}},
		{"non tls endpoint", Config{Provider: ProviderExpo, AccessToken: "secret", Endpoint: "http://example.com/push"}},
		{"endpoint user info", Config{Provider: ProviderExpo, AccessToken: "secret", Endpoint: "https://user@example.com/push"}},
		{"timeout too large", Config{Provider: ProviderExpo, AccessToken: "secret", Timeout: 2 * time.Minute}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport, err := New(test.config)
			if err == nil || transport != nil {
				t.Fatalf("New() = (%v, %v), want (nil, error)", transport, err)
			}
			if strings.Contains(err.Error(), test.config.AccessToken) && test.config.AccessToken != "" {
				t.Fatalf("configuration error leaked credential: %q", err)
			}
		})
	}
}

func TestExpoSendSuccessMapsBoundedPayload(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/send" {
			t.Errorf("path = %q, want /send", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer rotation-key-a" {
			t.Errorf("Authorization = %q", got)
		}
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if got["to"] != testToken || got["collapseId"] != "deploy:srv-1" || got["tag"] != "deploy" || got["priority"] != "high" {
			t.Errorf("unexpected provider payload: %#v", got)
		}
		data, ok := got["data"].(map[string]any)
		if !ok || data["schema"] != "bex.notification.v1" || data["route"] != "/services/srv-1" || data["notificationId"] != "evt-1" || data["event"] != "deploy_failed" {
			t.Errorf("unexpected provider data: %#v", got["data"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"status": "ok", "id": "ticket-1"},
		})
	}))
	defer server.Close()

	transport := testTransport(t, server, "rotation-key-a")
	ticket, err := transport.Send(context.Background(), Message{
		Token: testToken,
		Title: "Deploy failed",
		Body:  "Open the service for evidence.",
		Data: EnvelopeData{
			Schema: "bex.notification.v1", NotificationID: "evt-1",
			Event: "deploy_failed", Route: "/services/srv-1",
		},
		CollapseKey: "deploy:srv-1",
		Tag:         "deploy",
		Priority:    PriorityHigh,
	})
	if err != nil {
		t.Fatalf("Send(): %v", err)
	}
	if ticket.ID != "ticket-1" {
		t.Fatalf("ticket = %#v", ticket)
	}
}

func TestExpoSendTypedFailuresAndRedaction(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		retryAfter string
		response   string
		want       func(error) bool
	}{
		{"throttled http", http.StatusTooManyRequests, "9", `{}`, isError[*RateLimitedError]},
		{"outage", http.StatusServiceUnavailable, "", `provider says ` + testToken, isError[*TransientError]},
		{"invalid token ticket", http.StatusOK, "", `{"data":{"status":"error","message":"` + testToken + `","details":{"error":"DeviceNotRegistered"}}}`, isError[*InvalidTokenError]},
		{"rate limited ticket", http.StatusOK, "", `{"data":{"status":"error","details":{"error":"MessageRateExceeded"}}}`, isError[*RateLimitedError]},
		{"permanent ticket", http.StatusOK, "", `{"data":{"status":"error","details":{"error":"InvalidCredentials"}}}`, isError[*PermanentError]},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if test.retryAfter != "" {
					w.Header().Set("Retry-After", test.retryAfter)
				}
				w.WriteHeader(test.statusCode)
				_, _ = w.Write([]byte(test.response))
			}))
			defer server.Close()
			_, err := testTransport(t, server, "provider-secret").Send(context.Background(), Message{Token: testToken, Data: validTestData()})
			if !test.want(err) {
				t.Fatalf("Send() error = %T %v, want typed error", err, err)
			}
			if strings.Contains(err.Error(), testToken) || strings.Contains(err.Error(), "provider-secret") {
				t.Fatalf("error leaked token or credential: %q", err)
			}
			if limited, ok := err.(*RateLimitedError); ok && test.retryAfter != "" && limited.RetryAfter != 9*time.Second {
				t.Fatalf("RetryAfter = %v, want 9s", limited.RetryAfter)
			}
		})
	}
}

func TestExpoPayloadValidationIsPermanentAndMakesNoRequest(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	transport := testTransport(t, server, "secret")

	tests := []Message{
		{Token: "not-a-token", Data: validTestData()},
		{Token: testToken, Data: EnvelopeData{Schema: "bex.notification.v1", NotificationID: "evt-1", Event: "deploy_failed", Route: "https://attacker.example"}},
		{Token: testToken, Body: strings.Repeat("x", MaxBodyBytes+1), Data: validTestData()},
		// JSON escaping makes the encoded payload exceed 4096 bytes even though
		// the raw body stays within its field cap.
		{Token: testToken, Body: strings.Repeat("\x01", MaxBodyBytes), Data: validTestData()},
	}
	for _, message := range tests {
		_, err := transport.Send(context.Background(), message)
		var invalidToken *InvalidTokenError
		var invalidPayload *PayloadError
		if !errors.As(err, &invalidToken) && !errors.As(err, &invalidPayload) {
			t.Fatalf("Send(%#v) error = %T %v, want permanent input error", message, err, err)
		}
	}
	if requests != 0 {
		t.Fatalf("invalid payloads made %d provider requests", requests)
	}
}

func TestExpoCheckReceiptsMapsPerTicketOutcomes(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getReceipts" {
			t.Errorf("path = %q, want /getReceipts", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"ok":      map[string]any{"status": "ok"},
			"invalid": map[string]any{"status": "error", "details": map[string]any{"error": "DeviceNotRegistered"}},
			"limited": map[string]any{"status": "error", "details": map[string]any{"error": "MessageRateExceeded"}},
			"payload": map[string]any{"status": "error", "details": map[string]any{"error": "MessageTooBig"}},
			"unknown": map[string]any{"status": "error", "details": map[string]any{"error": "UnknownProviderCode"}},
			"foreign": map[string]any{"status": "ok"},
		}})
	}))
	defer server.Close()

	receipts, err := testTransport(t, server, "secret").CheckReceipts(context.Background(), []string{"ok", "invalid", "limited", "payload", "unknown", "pending"})
	if err != nil {
		t.Fatalf("CheckReceipts(): %v", err)
	}
	if len(receipts) != 5 || receipts["ok"].Err != nil {
		t.Fatalf("receipts = %#v", receipts)
	}
	assertErrorType[*InvalidTokenError](t, receipts["invalid"].Err)
	assertErrorType[*RateLimitedError](t, receipts["limited"].Err)
	assertErrorType[*PayloadError](t, receipts["payload"].Err)
	assertErrorType[*PermanentError](t, receipts["unknown"].Err)
	if strings.Contains(receipts["unknown"].Err.Error(), "UnknownProviderCode") {
		t.Fatalf("unknown provider detail escaped redaction: %q", receipts["unknown"].Err)
	}
	if _, found := receipts["pending"]; found {
		t.Fatal("provider-omitted pending receipt must remain omitted")
	}
	if _, found := receipts["foreign"]; found {
		t.Fatal("unrequested provider receipt must be ignored")
	}
}

func TestExpoCheckReceiptsOutageAndMalformedResponseAreTransient(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{"outage", http.StatusBadGateway, `{}`},
		{"missing data", http.StatusOK, `{}`},
		{"invalid json", http.StatusOK, `{`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			_, err := testTransport(t, server, "secret").CheckReceipts(context.Background(), []string{"ticket"})
			assertErrorType[*TransientError](t, err)
		})
	}
}

func TestExpoCredentialRotationUsesOnlyEachConstructedCredential(t *testing.T) {
	var mu sync.Mutex
	var authorizations []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "ok", "id": "ticket"}})
	}))
	defer server.Close()

	oldTransport := testTransport(t, server, "old-credential")
	newTransport := testTransport(t, server, "new-credential")
	if _, err := oldTransport.Send(context.Background(), Message{Token: testToken, Data: validTestData()}); err != nil {
		t.Fatal(err)
	}
	if _, err := newTransport.Send(context.Background(), Message{Token: testToken, Data: validTestData()}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if got := strings.Join(authorizations, ","); got != "Bearer old-credential,Bearer new-credential" {
		t.Fatalf("authorization sequence = %q", got)
	}
}

func validTestData() EnvelopeData {
	return EnvelopeData{
		Schema: "bex.notification.v1", NotificationID: "evt-test",
		Event: "deploy_failed", Route: "/services/srv-test",
	}
}

func testTransport(t *testing.T, server *httptest.Server, accessToken string) Transport {
	t.Helper()
	transport, err := New(Config{
		Provider:    ProviderExpo,
		AccessToken: accessToken,
		Endpoint:    server.URL,
	}, WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return transport
}

func assertErrorType[T error](t *testing.T, err error) {
	t.Helper()
	var target T
	if !errors.As(err, &target) {
		t.Fatalf("error = %T %v, want %T", err, err, target)
	}
}

func isError[T error](err error) bool {
	var target T
	return errors.As(err, &target)
}
