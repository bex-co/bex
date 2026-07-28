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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
)

// stripeStub is a path-routed http.RoundTripper returning canned Stripe JSON.
type stripeStub struct {
	mu      sync.Mutex
	hits    []string // request paths, in order
	bodies  []string // POST bodies, in order
	headers []http.Header
	route   func(method, path string) (status int, body string)
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
	s.headers = append(s.headers, req.Header.Clone())
	s.mu.Unlock()
	status, respBody := s.route(req.Method, req.URL.Path)
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: h, Body: io.NopCloser(strings.NewReader(respBody)), Request: req}, nil
}

func (s *stripeStub) requests(path string) []struct {
	body   string
	header http.Header
} {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]struct {
		body   string
		header http.Header
	}, 0)
	for i, hit := range s.hits {
		if hit == path {
			out = append(out, struct {
				body   string
				header http.Header
			}{body: s.bodies[i], header: s.headers[i]})
		}
	}
	return out
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
	if id, ok := c.lookupCustomer("tea-abc"); !ok || id != "cus_123" {
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
	if id, _ := c.lookupCustomer("tea-abc"); id != "cus_existing" {
		t.Fatalf("recovered customer = %q, want cus_existing (no create)", id)
	}
	if stub.count("/customers/search") != 1 {
		t.Fatalf("search calls = %d, want 1", stub.count("/customers/search"))
	}
}

func TestStripeEnsureCustomerRejectsDuplicateMetadataMatches(t *testing.T) {
	c, _ := newStripeTest(t, func(_ string, path string) (int, string) {
		if strings.Contains(path, "/customers/search") {
			return 200, `{"object":"search_result","data":[{"id":"cus_one","object":"customer"},{"id":"cus_two","object":"customer"}],"has_more":false,"url":"/v1/customers/search"}`
		}
		return 500, `{"error":{"type":"api_error","message":"must not create"}}`
	})
	err := c.EnsureCustomer(context.Background(), "tea-abc")
	if err == nil || !strings.Contains(err.Error(), "has 2 Customers") {
		t.Fatalf("EnsureCustomer duplicate metadata matches = %v, want explicit ambiguity error", err)
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
	result := c.IngestBatch(ctx, events)
	if len(result.Failed) != 0 || len(result.Accepted) != len(events) {
		t.Fatalf("IngestBatch result = %+v", result)
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
		{"egress rounded to Stripe precision", Event{EventType: "egress_bytes", Properties: map[string]string{"value": "1528842059"}}, "egress_gib", "1.423845122568", false},
		{"single byte retained", Event{EventType: "egress_bytes", Properties: map[string]string{"value": "1"}}, "egress_gib", "0.000000000931", false},
		{"storage re-based to GB-hours", Event{EventType: "storage_gb_seconds", Properties: map[string]string{"value": "7200"}}, "storage_gb_hours", "2", false},
		{"storage rounded to Stripe precision", Event{EventType: "storage_gb_seconds", Properties: map[string]string{"value": "1"}}, "storage_gb_hours", "0.000277777778", false},
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
	c.storeCustomer("tea-a", "cus_1") // pre-seed the customer cache
	// A permanent 400 is returned as a per-event durable-reject candidate.
	result := c.IngestBatch(context.Background(), []Event{
		{TransactionID: "tx1", CustomerID: "tea-a", EventType: "instance_seconds", Timestamp: time.Now(), Properties: map[string]string{"tier": "starter", "resource_kind": "service", "value": "1"}},
	})
	if len(result.Accepted) != 0 || len(result.Failed) != 1 || !result.Failed[0].Permanent {
		t.Fatalf("IngestBatch on 400 = %+v, want one permanent failure", result)
	}
}

func TestStripeIngestBatchTreatsDuplicateIdentifierAsAccepted(t *testing.T) {
	c, _ := newStripeTest(t, func(_ string, path string) (int, string) {
		if strings.Contains(path, "/billing/meter_events") {
			return 400, `{"error":{"type":"invalid_request_error","code":"duplicate_meter_event","message":"already submitted"}}`
		}
		return 200, `{"id":"cus_1","object":"customer"}`
	})
	c.storeCustomer("tea-a", "cus_1")
	result := c.IngestBatch(context.Background(), []Event{
		{TransactionID: "tx1", CustomerID: "tea-a", EventType: "instance_seconds", Timestamp: time.Now(), Properties: map[string]string{"tier": "starter", "resource_kind": "service", "value": "1"}},
	})
	if len(result.Accepted) != 1 || result.Accepted[0] != "tx1" || len(result.Failed) != 0 {
		t.Fatalf("IngestBatch duplicate result = %+v, want accepted tx1", result)
	}
}

func TestStripeIngestBatchReturnsRetryableResultOnTransient(t *testing.T) {
	c, _ := newStripeTest(t, func(_ string, path string) (int, string) {
		if strings.Contains(path, "/billing/meter_events") {
			return 500, `{"error":{"type":"api_error","message":"boom"}}`
		}
		return 200, `{"id":"cus_1","object":"customer"}`
	})
	c.storeCustomer("tea-a", "cus_1")
	result := c.IngestBatch(context.Background(), []Event{
		{TransactionID: "tx1", CustomerID: "tea-a", EventType: "instance_seconds", Timestamp: time.Now(), Properties: map[string]string{"tier": "starter", "resource_kind": "service", "value": "1"}},
	})
	if len(result.Accepted) != 0 || len(result.Failed) != 1 || result.Failed[0].Permanent {
		t.Fatalf("IngestBatch on 5xx = %+v, want one retryable failure", result)
	}
}

func TestStripeEnsureContractCreatesCompleteSubscription(t *testing.T) {
	catalog := stripePriceCatalogJSON(t, pricing.Default.BillableMeterNames())
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && strings.Contains(path, "/subscriptions"):
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && strings.Contains(path, "/prices"):
			return 200, catalog
		case method == http.MethodPost && strings.Contains(path, "/subscriptions"):
			return 200, `{"id":"sub_123","object":"subscription","status":"active"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.storeCustomer("tea-a", "cus_1")
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("EnsureContract: %v", err)
	}
	stub.mu.Lock()
	bodies := strings.Join(stub.bodies, "\n")
	stub.mu.Unlock()
	for _, want := range []string{"customer=cus_1", "metadata[bex_workspace]=tea-a", "metadata[bex_billing_contract]=true"} {
		if !strings.Contains(bodies, want) {
			t.Fatalf("subscription body missing %q: %s", want, bodies)
		}
	}
	if got := strings.Count(bodies, "items["); got != len(pricing.Default.BillableMeterNames()) {
		t.Fatalf("subscription item fields = %d, want %d: %s", got, len(pricing.Default.BillableMeterNames()), bodies)
	}
}

func TestStripeEnsureContractReusesExistingAfterRestart(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && strings.Contains(path, "/subscriptions") {
			return 200, `{"object":"list","data":[{"id":"sub_existing","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		}
		return 500, `{"error":{"type":"api_error","message":"should not create or list prices"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	if err := c.EnsureContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("EnsureContract: %v", err)
	}
	if got := stub.count("/prices"); got != 0 {
		t.Fatalf("price calls = %d, want 0 for existing subscription", got)
	}
	if got := stub.count("/subscriptions"); got != 1 {
		t.Fatalf("subscription calls = %d, want list only", got)
	}
}

func TestStripeEnsureContractRejectsIncompleteCatalog(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	catalog := stripePriceCatalogJSON(t, names[:len(names)-1])
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && strings.Contains(path, "/subscriptions"):
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && strings.Contains(path, "/prices"):
			return 200, catalog
		default:
			return 500, `{"error":{"type":"api_error","message":"subscription must not be created"}}`
		}
	})
	c.storeCustomer("tea-a", "cus_1")
	if err := c.EnsureContract(context.Background(), "tea-a"); err == nil || !strings.Contains(err.Error(), "is missing") {
		t.Fatalf("EnsureContract incomplete catalog = %v, want missing-price error", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	for i, path := range stub.hits {
		if strings.Contains(path, "/subscriptions") && i < len(stub.bodies) && stub.bodies[i] != "" {
			t.Fatalf("unexpected subscription create body: %s", stub.bodies[i])
		}
	}
}

func TestStripeCompCustomerAppliesCouponIdempotently(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/subscriptions" {
			return 200, `{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		}
		if method == http.MethodPost && path == "/v1/subscriptions/sub_1" {
			return 200, `{"id":"sub_1","object":"subscription","status":"active"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	if err := c.CompCustomer(context.Background(), "tea-a"); err != nil {
		t.Fatalf("CompCustomer: %v", err)
	}
	stub.mu.Lock()
	bodies := strings.Join(stub.bodies, "\n")
	stub.mu.Unlock()
	for _, want := range []string{"discounts[0][coupon]=bex-comp-100", "proration_behavior=none"} {
		if !strings.Contains(bodies, want) {
			t.Fatalf("comp body missing %q: %s", want, bodies)
		}
	}
}

func TestStripeBillingForReadsPreviewAndFinalizedInvoices(t *testing.T) {
	c, _ := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodPost && path == "/v1/invoices/create_preview":
			return 200, `{"id":"upcoming_in_1","object":"invoice","currency":"usd","total":1234,"period_start":1782864000,"period_end":1785542400,"status":"draft"}`
		case method == http.MethodGet && path == "/v1/invoices":
			return 200, `{"object":"list","data":[{"id":"in_draft","object":"invoice","currency":"usd","total":999,"period_start":1780272000,"period_end":1782864000,"status":"draft"},{"id":"in_paid","object":"invoice","currency":"usd","total":4000,"period_start":1780272000,"period_end":1782864000,"status":"paid"}],"has_more":false,"url":"/v1/invoices"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.storeCustomer("tea-a", "cus_1")
	b, err := c.BillingFor(context.Background(), "tea-a", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("BillingFor: %v", err)
	}
	if b == nil || b.CurrentCost == nil || b.CurrentCost.AmountUSD != "12.34" || b.CurrentCost.Currency != "USD" {
		t.Fatalf("current billing = %+v", b)
	}
	if len(b.Invoices) != 1 || b.Invoices[0].ID != "in_paid" || b.Invoices[0].Status != "PAID" || b.Invoices[0].AmountUSD != "40.00" {
		t.Fatalf("invoices = %+v", b.Invoices)
	}
}

func TestStripeBillingForMissingCustomerDoesNotCreate(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && strings.Contains(path, "/customers/search") {
			return 200, `{"object":"search_result","data":[],"has_more":false,"url":"/v1/customers/search"}`
		}
		return 500, `{"error":{"type":"api_error","message":"must not create"}}`
	})
	b, err := c.BillingFor(context.Background(), "tea-excluded", time.Time{}, time.Time{})
	if err != nil || b != nil {
		t.Fatalf("BillingFor missing customer = %+v,%v, want nil,nil", b, err)
	}
	if got := stub.count("/customers"); got != 1 {
		t.Fatalf("customer calls = %d, want search only", got)
	}
}

func TestStripeWebhookVerifiesSignatureAndDispatchesPaymentFailure(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(fmt.Sprintf(`{"id":"evt_1","object":"event","api_version":%q,"created":1785196800,"livemode":false,"type":"invoice.payment_failed","data":{"object":{"id":"in_1","object":"invoice","livemode":false,"customer":"cus_1","parent":{"type":"subscription_details","subscription_details":{"subscription":"sub_1","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}}}}}`, stripe.APIVersion))
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: time.Now()})
	var got string
	h := &StripeWebhook{Secret: secret, OnLifecycle: func(_ context.Context, event *stripe.Event) error {
		got = event.ID
		return nil
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", signed.Header)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent || got != "evt_1" {
		t.Fatalf("webhook status=%d event=%q body=%s", w.Code, got, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", "bad")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad signature status=%d, want 400", w.Code)
	}
}

func TestStripeWebhookRejectsIncompatibleVersionAndMode(t *testing.T) {
	secret := "whsec_test"
	for _, tc := range []struct {
		name, version string
		livemode      bool
	}{
		{name: "version", version: "2025-01-01", livemode: false},
		{name: "mode", version: stripe.APIVersion, livemode: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(fmt.Sprintf(`{"id":"evt_guard","object":"event","api_version":%q,"created":1785196800,"livemode":%t,"type":"invoice.payment_failed","data":{"object":{}}}`, tc.version, tc.livemode))
			signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: time.Now()})
			req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe", strings.NewReader(string(payload)))
			req.Header.Set("Stripe-Signature", signed.Header)
			w := httptest.NewRecorder()
			(&StripeWebhook{Secret: secret, ExpectedLivemode: false}).ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestStripeCreateCheckoutSessionUsesExistingContractAndDynamicPaymentMethods(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodPost && path == "/v1/checkout/sessions":
			return 200, `{"id":"cs_test_1","object":"checkout.session","mode":"setup","livemode":false,"expires_at":1785200000,"url":"https://checkout.stripe.com/c/pay/cs_test_1"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.dashboardURL = "https://dashboard.bex.co"
	c.storeCustomer("tea-a", "cus_1")

	session, err := c.CreateCheckoutSession(context.Background(), "tea-a", CheckoutRequest{
		SuccessURL: "https://dashboard.bex.co/usage?billing=success",
		CancelURL:  "https://dashboard.bex.co/usage?billing=cancelled",
	})
	if err != nil {
		t.Fatalf("CreateCheckoutSession: %v", err)
	}
	if session.URL == "" || session.ExpiresAt == "" {
		t.Fatalf("hosted session = %+v", session)
	}
	requests := stub.requests("/v1/checkout/sessions")
	if len(requests) != 1 {
		t.Fatalf("Checkout create calls = %d, want 1", len(requests))
	}
	body := requests[0].body
	for _, want := range []string{"mode=setup", "currency=usd", "customer=cus_1", "client_reference_id=tea-a", "metadata[bex_workspace]=tea-a", "metadata[bex_subscription]=sub_1", "setup_intent_data[metadata][bex_workspace]=tea-a"} {
		if !strings.Contains(body, want) {
			t.Errorf("Checkout body missing %q: %s", want, body)
		}
	}
	for _, forbidden := range []string{"payment_method_types", "line_items", "subscription_data"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("Checkout body unexpectedly contains %q: %s", forbidden, body)
		}
	}
	if got := requests[0].header.Get("Idempotency-Key"); !strings.HasPrefix(got, "bex-checkout-tea-a-") {
		t.Errorf("Idempotency-Key = %q", got)
	}
	if !strings.Contains(body, "integration_identifier=bex_billing_") {
		t.Errorf("Checkout body missing randomized integration_identifier: %s", body)
	}
	// EnsureContract and the postcondition read only listed the one existing
	// Subscription; setup mode never created a second contract.
	for _, req := range stub.requests("/v1/subscriptions") {
		if req.body != "" {
			t.Fatalf("unexpected Subscription mutation: %s", req.body)
		}
	}
}

func TestStripeCreateCheckoutSessionRejectsUntrustedReturnOrigin(t *testing.T) {
	c, stub := newStripeTest(t, func(_, _ string) (int, string) {
		return 500, `{"error":{"type":"api_error","message":"must not call Stripe"}}`
	})
	c.dashboardURL = "https://dashboard.bex.co"
	_, err := c.CreateCheckoutSession(context.Background(), "tea-a", CheckoutRequest{
		SuccessURL: "https://evil.example/steal",
		CancelURL:  "https://dashboard.bex.co/usage",
	})
	if err == nil || !isInputError(err) {
		t.Fatalf("untrusted Checkout success URL error = %v, want inputError", err)
	}
	if len(stub.hits) != 0 {
		t.Fatalf("Stripe calls after invalid redirect = %v", stub.hits)
	}
}

func TestStripeCreatePortalSessionIsCustomerScoped(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodPost && path == "/v1/billing_portal/sessions":
			return 200, `{"id":"bps_1","object":"billing_portal.session","livemode":false,"url":"https://billing.stripe.com/p/session/test"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.dashboardURL = "https://dashboard.bex.co"
	c.portalConfigurationID = "bpc_test"
	c.storeCustomer("tea-a", "cus_1")
	out, err := c.CreatePortalSession(context.Background(), "tea-a", PortalRequest{ReturnURL: "https://dashboard.bex.co/usage"})
	if err != nil {
		t.Fatalf("CreatePortalSession: %v", err)
	}
	if out.URL == "" {
		t.Fatal("Portal URL is empty")
	}
	requests := stub.requests("/v1/billing_portal/sessions")
	if len(requests) != 1 || !strings.Contains(requests[0].body, "customer=cus_1") || !strings.Contains(requests[0].body, "configuration=bpc_test") {
		t.Fatalf("Portal request = %+v", requests)
	}
}

func TestStripeCompleteCheckoutSessionBindsDefaultsIdempotently(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/checkout/sessions/cs_test_1":
			return 200, `{"id":"cs_test_1","object":"checkout.session","mode":"setup","status":"complete","livemode":false,"customer":"cus_1","setup_intent":"seti_1","metadata":{"bex_workspace":"tea-a","bex_subscription":"sub_1"}}`
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/setup_intents/seti_1":
			return 200, `{"id":"seti_1","object":"setup_intent","status":"succeeded","customer":"cus_1","payment_method":"pm_1","metadata":{"bex_workspace":"tea-a","bex_subscription":"sub_1"}}`
		case method == http.MethodGet && path == "/v1/payment_methods/pm_1":
			return 200, `{"id":"pm_1","object":"payment_method","customer":"cus_1","livemode":false,"type":"card"}`
		case method == http.MethodGet && path == "/v1/tax/registrations":
			return 200, `{"object":"list","data":[{"id":"taxreg_test","object":"tax.registration","livemode":false,"status":"active","country":"US"}],"has_more":false,"url":"/v1/tax/registrations"}`
		case method == http.MethodPost && path == "/v1/customers/cus_1":
			return 200, `{"id":"cus_1","object":"customer","invoice_settings":{"default_payment_method":"pm_1"}}`
		case method == http.MethodPost && path == "/v1/subscriptions/sub_1":
			return 200, `{"id":"sub_1","object":"subscription","status":"active","default_payment_method":"pm_1"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.storeCustomer("tea-a", "cus_1")
	c.taxCode = "txcd_confirmed"
	c.taxBehavior = "exclusive"
	c.priceIDs = []string{"price_tax_ready"}
	for i := 0; i < 2; i++ {
		if err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_test_1"}); err != nil {
			t.Fatalf("CompleteCheckoutSession #%d: %v", i+1, err)
		}
	}
	customerUpdates := stub.requests("/v1/customers/cus_1")
	subscriptionUpdates := stub.requests("/v1/subscriptions/sub_1")
	if len(customerUpdates) != 2 || len(subscriptionUpdates) != 2 {
		t.Fatalf("updates customer=%d subscription=%d", len(customerUpdates), len(subscriptionUpdates))
	}
	for _, req := range customerUpdates {
		if !strings.Contains(req.body, "invoice_settings[default_payment_method]=pm_1") || req.header.Get("Idempotency-Key") != "bex-payment-customer-cs_test_1" {
			t.Errorf("Customer update = body:%s idempotency:%s", req.body, req.header.Get("Idempotency-Key"))
		}
	}
	for _, req := range subscriptionUpdates {
		if !strings.Contains(req.body, "default_payment_method=pm_1") || !strings.Contains(req.body, "automatic_tax[enabled]=true") || req.header.Get("Idempotency-Key") != "bex-payment-subscription-cs_test_1" {
			t.Errorf("Subscription update = body:%s idempotency:%s", req.body, req.header.Get("Idempotency-Key"))
		}
	}
}

func TestStripeTaxGateFailsClosedWithoutActiveTestRegistration(t *testing.T) {
	c, _ := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/tax/registrations" {
			return 200, `{"object":"list","data":[{"id":"taxreg_live","object":"tax.registration","livemode":true,"status":"active","country":"US"}],"has_more":false,"url":"/v1/tax/registrations"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.taxCode = "txcd_operator_confirmed"
	c.taxBehavior = "exclusive"
	c.priceIDs = []string{"price_tax_ready"}
	tax := c.taxReadiness(context.Background(), &stripe.Subscription{AutomaticTax: &stripe.SubscriptionAutomaticTax{Enabled: true}})
	if tax.Configured || tax.Enabled || tax.Reason != "active_registration_missing" || tax.RegistrationCount != 0 {
		t.Fatalf("tax readiness with only live registration = %+v", tax)
	}
}

func TestStripeTaxGateReportsConfiguredOnlyAfterCatalogAndRegistration(t *testing.T) {
	c, _ := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/tax/registrations" {
			return 200, `{"object":"list","data":[{"id":"taxreg_test","object":"tax.registration","livemode":false,"status":"active","country":"US"}],"has_more":false,"url":"/v1/tax/registrations"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.taxCode = "txcd_operator_confirmed"
	c.taxBehavior = "exclusive"
	c.priceIDs = []string{"price_tax_ready"}
	tax := c.taxReadiness(context.Background(), &stripe.Subscription{AutomaticTax: &stripe.SubscriptionAutomaticTax{Enabled: true}})
	if !tax.Configured || !tax.Enabled || tax.Reason != "" || tax.RegistrationCount != 1 || tax.ProductTaxCode != c.taxCode || tax.TaxBehavior != "exclusive" {
		t.Fatalf("tax readiness = %+v", tax)
	}
}

func TestStripeCompleteCheckoutSessionRejectsCrossWorkspaceBinding(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/checkout/sessions/cs_test_bad":
			return 200, `{"id":"cs_test_bad","object":"checkout.session","mode":"setup","status":"complete","livemode":false,"customer":"cus_other","setup_intent":"seti_1","metadata":{"bex_workspace":"tea-a","bex_subscription":"sub_other"}}`
		default:
			return 500, `{"error":{"type":"api_error","message":"must not mutate"}}`
		}
	})
	c.storeCustomer("tea-a", "cus_1")
	err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_test_bad"})
	if err == nil || !isInputError(err) || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("cross-workspace completion error = %v", err)
	}
	if stub.count("/customers/cus_1") != 0 || stub.count("/subscriptions/sub_other") != 0 {
		t.Fatalf("cross-workspace completion mutated Stripe: %v", stub.hits)
	}
}

func TestStripeCompleteCheckoutSessionRejectsAuthoritativeOpenSession(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/checkout/sessions/cs_test_open" {
			return 200, `{"id":"cs_test_open","object":"checkout.session","mode":"setup","status":"open","livemode":false,"customer":"cus_1","setup_intent":"seti_1","metadata":{"bex_workspace":"tea-a","bex_subscription":"sub_1"}}`
		}
		return 500, `{"error":{"type":"api_error","message":"must not mutate"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_test_open"})
	if err == nil || !isInputError(err) || !strings.Contains(err.Error(), "not a completed setup session") {
		t.Fatalf("open Checkout completion error = %v", err)
	}
	if len(stub.hits) != 1 {
		t.Fatalf("open Checkout caused additional Stripe calls: %v", stub.hits)
	}
}

func TestStripeWebhookDispatchesCheckoutCompletion(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(fmt.Sprintf(`{"id":"evt_checkout","object":"event","api_version":%q,"type":"checkout.session.completed","data":{"object":{"id":"cs_test_1","object":"checkout.session","livemode":false}}}`, stripe.APIVersion))
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: time.Now()})
	var got string
	h := &StripeWebhook{Secret: secret, OnCheckoutCompleted: func(_ context.Context, session *stripe.CheckoutSession) error {
		got = session.ID
		return nil
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe", strings.NewReader(string(payload)))
	req.Header.Set("Stripe-Signature", signed.Header)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent || got != "cs_test_1" {
		t.Fatalf("webhook status=%d session=%q body=%s", w.Code, got, w.Body.String())
	}
}

func stripePriceCatalogJSON(t *testing.T, names []string) string {
	t.Helper()
	data := make([]map[string]any, 0, len(names))
	for i, name := range names {
		data = append(data, map[string]any{
			"id":         fmt.Sprintf("price_%02d", i),
			"object":     "price",
			"active":     true,
			"currency":   "usd",
			"lookup_key": name,
			"recurring": map[string]any{
				"interval":       "month",
				"interval_count": 1,
				"meter":          fmt.Sprintf("mtr_%02d", i),
				"usage_type":     "metered",
			},
		})
	}
	b, err := json.Marshal(map[string]any{"object": "list", "data": data, "has_more": false, "url": "/v1/prices"})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
