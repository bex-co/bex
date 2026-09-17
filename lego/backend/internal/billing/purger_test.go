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
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	stripe "github.com/stripe/stripe-go/v86"
)

// urlRecorder is a RoundTripper that captures full request URLs (path + query),
// which the shared stripeStub does not — the SDK encodes DELETE params in the
// query string, so that is where CancelContract's invoice_now flag lands.
type urlRecorder struct {
	mu    sync.Mutex
	urls  []string
	route func(method, path string) (int, string)
}

func (r *urlRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}
	r.mu.Lock()
	full := req.URL.Path
	if req.URL.RawQuery != "" {
		full += "?" + req.URL.RawQuery
	}
	r.urls = append(r.urls, req.Method+" "+full)
	r.mu.Unlock()
	status, body := r.route(req.Method, req.URL.Path)
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

// TestCancelContractCancelsLiveSubscription proves CancelContract finds the
// workspace's live subscription and issues an immediate cancel (DELETE) that
// invoices final metered usage, and never deletes the Customer.
func TestCancelContractCancelsLiveSubscription(t *testing.T) {
	rec := &urlRecorder{route: func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodDelete && path == "/v1/subscriptions/sub_1":
			return 200, `{"id":"sub_1","object":"subscription","status":"canceled"}`
		case method == http.MethodGet && path == "/v1/payment_methods":
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/payment_methods"}`
		case method == http.MethodPost && path == "/v1/customers/cus_1":
			return 200, `{"id":"cus_1","object":"customer"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route ` + method + ` ` + path + `"}}`
		}
	}}
	c := NewStripe(StripeConfig{
		SecretKey:         "sk_test_x",
		HTTPClient:        &http.Client{Transport: rec},
		BaseURL:           "https://stub.stripe.test",
		MaxNetworkRetries: stripe.Int64(0),
	})
	c.storeCustomer("tea-a", "cus_1")

	if err := c.CancelContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("CancelContract: %v", err)
	}
	var cancels, customerHits int
	var cancelURL string
	for _, u := range rec.urls {
		if strings.HasPrefix(u, "DELETE /v1/subscriptions/sub_1") {
			cancels++
			cancelURL = u
		}
		if strings.Contains(u, "/v1/customers/cus_1") && !strings.HasSuffix(u, "/customers/cus_1?") && strings.HasPrefix(u, "DELETE") {
			customerHits++
		}
	}
	if cancels != 1 {
		t.Fatalf("immediate cancel (DELETE) calls = %d, want exactly 1 (urls=%v)", cancels, rec.urls)
	}
	if !strings.Contains(cancelURL, "invoice_now=true") {
		t.Errorf("cancel must request a final invoice (invoice_now=true): %q", cancelURL)
	}
	if customerHits != 0 {
		t.Errorf("CancelContract must never delete the Customer, saw %d customer deletes", customerHits)
	}
}

// TestCancelContractNoCustomerIsNoop proves a workspace that never became a
// Stripe Customer (Mode A excluded, or never billable) cancels nothing and makes
// no subscription call.
func TestCancelContractNoCustomerIsNoop(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && strings.Contains(path, "/customers/search") {
			return 200, `{"object":"search_result","data":[],"has_more":false,"url":"/v1/customers/search"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	if err := c.CancelContract(context.Background(), "tea-none"); err != nil {
		t.Fatalf("CancelContract no-customer: %v", err)
	}
	if got := stub.count("/subscriptions"); got != 0 {
		t.Fatalf("subscription calls = %d, want 0 when no Customer exists", got)
	}
}

// TestCancelContractNoLiveSubscriptionStillRetiresTheCustomer proves that an
// already-cancelled subscription issues no second cancel (idempotent retry) —
// and that the Customer is still retired anyway. A workspace can hold a card
// without a live subscription, so payment-instrument teardown must not hang off
// the cancel path.
func TestCancelContractNoLiveSubscriptionStillRetiresTheCustomer(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			// Only a cancelled subscription exists — findSubscriptionObject skips it.
			return 200, `{"object":"list","data":[{"id":"sub_old","object":"subscription","status":"canceled","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodGet && path == "/v1/payment_methods":
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/payment_methods"}`
		case method == http.MethodPost && path == "/v1/customers/cus_1":
			return 200, `{"id":"cus_1","object":"customer"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.storeCustomer("tea-a", "cus_1")

	if err := c.CancelContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("CancelContract already-cancelled: %v", err)
	}

	for _, hit := range stub.hits {
		if strings.HasPrefix(hit, "/v1/subscriptions/") {
			t.Errorf("no cancel should be issued for an already-cancelled subscription, saw %s", hit)
		}
	}
	if got := stub.count("/v1/customers/cus_1"); got != 1 {
		t.Fatalf("customer retire calls = %d, want 1 even without a live subscription", got)
	}
}

// TestCancelContractRetiresPaymentInstrumentsAndPersonalData is the defect this
// closes: deleting a workspace used to cancel the subscription and stop, leaving
// the card attached and the customer's email readable in Stripe indefinitely
// (observed on cus_VCV5qtrtlYh0N0 and cus_VGzT7XpXc4z57t).
func TestCancelContractRetiresPaymentInstrumentsAndPersonalData(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, `{"object":"list","data":[{"id":"sub_live","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
		case method == http.MethodDelete && path == "/v1/subscriptions/sub_live":
			return 200, `{"id":"sub_live","object":"subscription","status":"canceled"}`
		case method == http.MethodGet && path == "/v1/payment_methods":
			return 200, `{"object":"list","data":[{"id":"pm_1","object":"payment_method","type":"card"},{"id":"pm_2","object":"payment_method","type":"card"}],"has_more":false,"url":"/v1/payment_methods"}`
		case method == http.MethodPost && strings.HasSuffix(path, "/detach"):
			return 200, `{"id":"pm_x","object":"payment_method"}`
		case method == http.MethodPost && path == "/v1/customers/cus_1":
			return 200, `{"id":"cus_1","object":"customer"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.storeCustomer("tea-a", "cus_1")

	if err := c.CancelContract(context.Background(), "tea-a"); err != nil {
		t.Fatalf("CancelContract: %v", err)
	}

	if got := stub.count("/detach"); got != 2 {
		t.Errorf("detach calls = %d, want 2 (every attached payment method)", got)
	}
	retires := stub.requests("/v1/customers/cus_1")
	if len(retires) != 1 {
		t.Fatalf("customer retire calls = %d, want 1", len(retires))
	}
	body := retires[0].body
	for _, want := range []string{"email=", "name=", "invoice_settings[default_payment_method]=", "metadata[bex_deleted_at]="} {
		if !strings.Contains(body, want) {
			t.Errorf("retire body missing %q: %s", want, body)
		}
	}
	// Cleared, not rewritten: the values must be empty.
	for _, cleared := range []string{"email=&", "name=&", "invoice_settings[default_payment_method]=&"} {
		if !strings.Contains(body+"&", cleared) {
			t.Errorf("retire body does not clear %q: %s", cleared, body)
		}
	}
	// The Customer survives so its invoices stay linked.
	for _, hit := range stub.hits {
		if hit == "/v1/customers/cus_1" && retires[0].header.Get("X-Stripe-Mock-Delete") != "" {
			t.Errorf("customer must never be deleted, saw %s", hit)
		}
	}
}

// fakeCanceller records the workspace it was asked to cancel and can fail.
type fakeCanceller struct {
	cancelled []string
	err       error
}

func (f *fakeCanceller) CancelContract(_ context.Context, workspaceID string) error {
	if f.err != nil {
		return f.err
	}
	f.cancelled = append(f.cancelled, workspaceID)
	return nil
}

// TestWorkspacePurgerCancels proves the purger delegates to the canceller, and
// that a nil canceller (Stripe off) is a byte-identical no-op.
func TestWorkspacePurgerCancels(t *testing.T) {
	f := &fakeCanceller{}
	p := &WorkspacePurger{Canceller: f}
	if err := p.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if len(f.cancelled) != 1 || f.cancelled[0] != "tea-a" {
		t.Fatalf("cancelled = %v, want [tea-a]", f.cancelled)
	}

	// Stripe disabled: nil canceller, no-op, no panic.
	if err := (&WorkspacePurger{}).PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("nil-canceller purge: %v", err)
	}
}

// TestWorkspacePurgerSurfacesError proves a cancel failure surfaces so
// workspaces.Delete leaves the row intact and the delete is retryable.
func TestWorkspacePurgerSurfacesError(t *testing.T) {
	p := &WorkspacePurger{Canceller: &fakeCanceller{err: errors.New("stripe down")}}
	if err := p.PurgeWorkspace(context.Background(), "tea-a"); err == nil {
		t.Fatal("want the cancel error surfaced")
	}
}
