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

package billing

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v82"
)

// stripeStub is a path-routed http.RoundTripper returning canned Stripe JSON.
type stripeStub struct {
	mu     sync.Mutex
	hits   []string // request paths, in order
	bodies []string // POST bodies, in order
	route  func(method, path string) (status int, body string)
}

func (s *stripeStub) RoundTrip(req *http.Request) (*http.Response, error) {
	var body string
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()
		body = string(b)
	}
	s.mu.Lock()
	s.hits = append(s.hits, req.URL.Path)
	s.bodies = append(s.bodies, body)
	s.mu.Unlock()
	status, respBody := s.route(req.Method, req.URL.Path)
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: h, Body: io.NopCloser(strings.NewReader(respBody)), Request: req}, nil
}

func (s *stripeStub) count(substr string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, p := range s.hits {
		if strings.Contains(p, substr) {
			n++
		}
	}
	return n
}

func newStripeTest(t *testing.T, route func(method, path string) (int, string)) (*StripeClient, *stripeStub) {
	t.Helper()
	stub := &stripeStub{route: route}
	c := NewStripe(StripeConfig{
		SecretKey:         "sk_test_x",
		HTTPClient:        &http.Client{Transport: stub},
		BaseURL:           "https://stub.stripe.test",
		MaxNetworkRetries: stripe.Int64(0),
	})
	if c == nil {
		t.Fatal("NewStripe returned nil for a non-empty key")
	}
	return c, stub
}

func TestNewStripeDisabledWhenKeyUnset(t *testing.T) {
	if c := NewStripe(StripeConfig{SecretKey: ""}); c != nil {
		t.Fatalf("NewStripe with empty key = %v, want nil (disabled)", c)
	}
}

func TestStripeEnsureCustomerCreatesAndCaches(t *testing.T) {
	c, stub := newStripeTest(t, func(_ string, path string) (int, string) {
		switch {
		case strings.Contains(path, "/customers/search"):
			return 200, `{"object":"search_result","data":[],"has_more":false,"url":"/v1/customers/search"}`
		case strings.Contains(path, "/customers"):
			return 200, `{"id":"cus_123","object":"customer"}`
		default:
			return 200, "{}"
		}
	})
	ctx := context.Background()
	if err := c.EnsureCustomer(ctx, "tea-abc"); err != nil {
		t.Fatalf("EnsureCustomer: %v", err)
	}
	if id, ok := c.lookup("tea-abc"); !ok || id != "cus_123" {
		t.Fatalf("cached customer = %q,%v, want cus_123", id, ok)
	}
	// Second call served from cache — no more search/create.
	if err := c.EnsureCustomer(ctx, "tea-abc"); err != nil {
		t.Fatalf("EnsureCustomer (2nd): %v", err)
	}
	if stub.count("/customers") != 2 { // one search + one create, total, not four
		t.Fatalf("customer API calls = %d, want 2 (cached second time)", stub.count("/customers"))
	}
}

func TestStripeEnsureCustomerRecoversViaSearch(t *testing.T) {
	c, stub := newStripeTest(t, func(_ string, path string) (int, string) {
		if strings.Contains(path, "/customers/search") {
			return 200, `{"object":"search_result","data":[{"id":"cus_existing","object":"customer"}],"has_more":false,"url":"/v1/customers/search"}`
		}
		return 200, `{"id":"cus_new","object":"customer"}`
	})
	if err := c.EnsureCustomer(context.Background(), "tea-abc"); err != nil {
		t.Fatalf("EnsureCustomer: %v", err)
	}
	if id, _ := c.lookup("tea-abc"); id != "cus_existing" {
		t.Fatalf("recovered customer = %q, want cus_existing (no create)", id)
	}
	if stub.count("/customers/search") != 1 {
		t.Fatalf("search calls = %d, want 1", stub.count("/customers/search"))
	}
}

func TestStripeIngestBatchEmitsMeterEvents(t *testing.T) {
	c, stub := newStripeTest(t, func(_ string, path string) (int, string) {
		switch {
		case strings.Contains(path, "/customers/search"):
			return 200, `{"object":"search_result","data":[],"has_more":false,"url":"/v1/customers/search"}`
		case strings.Contains(path, "/customers"):
			return 200, `{"id":"cus_1","object":"customer"}`
		case strings.Contains(path, "/billing/meter_events"):
			return 200, `{"object":"billing.meter_event","identifier":"x"}`
		default:
			return 200, "{}"
		}
	})
	ctx := context.Background()
	if err := c.EnsureCustomer(ctx, "tea-a"); err != nil {
		t.Fatalf("EnsureCustomer: %v", err)
	}
	w := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	events := []Event{
		{TransactionID: "tx1", CustomerID: "tea-a", EventType: "instance_seconds", Timestamp: w, Properties: map[string]string{"tier": "starter", "resource_kind": "service", "value": "3600"}},
		{TransactionID: "tx2", CustomerID: "tea-a", EventType: "egress_bytes", Timestamp: w, Properties: map[string]string{"value": "1073741824"}}, // 1 GiB
	}
	if err := c.IngestBatch(ctx, events); err != nil {
		t.Fatalf("IngestBatch: %v", err)
	}
	if got := stub.count("/billing/meter_events"); got != 2 {
		t.Fatalf("meter events posted = %d, want 2", got)
	}
	// The payload carries the resolved customer, the composed per-tier meter
	// name, and the re-based value (1 GiB → "1").
	stub.mu.Lock()
	joined := strings.Join(stub.bodies, "\n")
	stub.mu.Unlock()
	for _, want := range []string{"cus_1", "instance_seconds.service.starter", "egress_gib", "payload[value]=1&"} {
		if !strings.Contains(joined+"&", want) {
			t.Fatalf("meter-event body missing %q: %s", want, joined)
		}
	}
}

func TestStripeMeterEventMapping(t *testing.T) {
	cases := []struct {
		name              string
		e                 Event
		wantName, wantVal string
		wantSkip          bool
	}{
		{"instance per-tier", Event{EventType: "instance_seconds", Properties: map[string]string{"tier": "pro-plus", "resource_kind": "service", "value": "3600"}}, "instance_seconds.service.pro-plus", "3600", false},
		{"instance postgres", Event{EventType: "instance_seconds", Properties: map[string]string{"tier": "basic-1gb", "resource_kind": "postgres", "value": "60"}}, "instance_seconds.postgres.basic-1gb", "60", false},
		{"free tier skipped", Event{EventType: "instance_seconds", Properties: map[string]string{"tier": "free", "resource_kind": "service", "value": "3600"}}, "", "", true},
		{"egress re-based to GiB", Event{EventType: "egress_bytes", Properties: map[string]string{"value": "2147483648"}}, "egress_gib", "2", false},
		{"storage re-based to GB-hours", Event{EventType: "storage_gb_seconds", Properties: map[string]string{"value": "7200"}}, "storage_gb_hours", "2", false},
		{"build unchanged", Event{EventType: "build_seconds", Properties: map[string]string{"value": "120"}}, "build_seconds", "120", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, val, skip := stripeMeterEvent(tc.e)
			if skip != tc.wantSkip || name != tc.wantName || val != tc.wantVal {
				t.Fatalf("got (%q,%q,%v), want (%q,%q,%v)", name, val, skip, tc.wantName, tc.wantVal, tc.wantSkip)
			}
		})
	}
}

func TestStripeIngestBatchDeadLettersPermanent4xx(t *testing.T) {
	c, _ := newStripeTest(t, func(_ string, path string) (int, string) {
		if strings.Contains(path, "/billing/meter_events") {
			return 400, `{"error":{"type":"invalid_request_error","message":"bad event"}}`
		}
		return 200, `{"id":"cus_1","object":"customer"}`
	})
	c.store("tea-a", "cus_1") // pre-seed the customer cache
	// A permanent 400 is dead-lettered (logged, dropped) — IngestBatch returns nil.
	err := c.IngestBatch(context.Background(), []Event{
		{TransactionID: "tx1", CustomerID: "tea-a", EventType: "instance_seconds", Timestamp: time.Now(), Properties: map[string]string{"tier": "starter", "resource_kind": "service", "value": "1"}},
	})
	if err != nil {
		t.Fatalf("IngestBatch on 400 = %v, want nil (dead-lettered)", err)
	}
}

func TestStripeIngestBatchReturnsErrorOnTransient(t *testing.T) {
	c, _ := newStripeTest(t, func(_ string, path string) (int, string) {
		if strings.Contains(path, "/billing/meter_events") {
			return 500, `{"error":{"type":"api_error","message":"boom"}}`
		}
		return 200, `{"id":"cus_1","object":"customer"}`
	})
	c.store("tea-a", "cus_1")
	err := c.IngestBatch(context.Background(), []Event{
		{TransactionID: "tx1", CustomerID: "tea-a", EventType: "instance_seconds", Timestamp: time.Now(), Properties: map[string]string{"tier": "starter", "resource_kind": "service", "value": "1"}},
	})
	if err == nil {
		t.Fatal("IngestBatch on 5xx = nil, want an error (transient → retry next cycle)")
	}
}
