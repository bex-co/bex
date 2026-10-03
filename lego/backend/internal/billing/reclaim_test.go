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
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// reclaimFixture is the Stripe state around one unbound Customer cus_r of
// tea-r. Each field flips one reason the reclaimer must keep the Customer.
type reclaimFixture struct {
	created        time.Time
	owner          string
	deletedMarker  bool
	defaultPM      bool
	attachedPM     bool
	moneyInvoice   bool
	openCheckout   bool
	accruedPreview bool
	missing        bool
}

func (f reclaimFixture) route(method, path string) (int, string) {
	list := func(object, url, data string) string {
		return fmt.Sprintf(`{"object":"list","data":[%s],"has_more":false,"url":"%s"}`, data, url)
	}
	switch {
	case method == http.MethodGet && path == "/v1/customers/cus_r":
		if f.missing {
			return 404, `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"No such customer"}}`
		}
		meta := fmt.Sprintf(`{"bex_workspace":%q}`, f.owner)
		if f.deletedMarker {
			meta = fmt.Sprintf(`{"bex_workspace":%q,"bex_deleted_at":"2026-09-01T00:00:00Z"}`, f.owner)
		}
		pm := `null`
		if f.defaultPM {
			pm = `"pm_default"`
		}
		return 200, fmt.Sprintf(`{"id":"cus_r","object":"customer","livemode":false,"created":%d,"metadata":%s,"invoice_settings":{"default_payment_method":%s}}`, f.created.Unix(), meta, pm)
	case method == http.MethodGet && path == "/v1/payment_methods":
		data := ""
		if f.attachedPM {
			data = `{"id":"pm_1","object":"payment_method","type":"card"}`
		}
		return 200, list("list", "/v1/payment_methods", data)
	case method == http.MethodGet && path == "/v1/invoices":
		total := 0
		if f.moneyInvoice {
			total = 1200
		}
		return 200, list("list", "/v1/invoices", fmt.Sprintf(`{"id":"in_1","object":"invoice","total":%d,"amount_paid":0,"amount_due":%d}`, total, total))
	case method == http.MethodGet && path == "/v1/checkout/sessions":
		data := ""
		if f.openCheckout {
			data = `{"id":"cs_open","object":"checkout.session","status":"open"}`
		}
		return 200, list("list", "/v1/checkout/sessions", data)
	case method == http.MethodGet && path == "/v1/subscriptions":
		return 200, list("list", "/v1/subscriptions", `{"id":"sub_r","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-r"}}`)
	case method == http.MethodPost && path == "/v1/invoices/create_preview":
		total := 0
		if f.accruedPreview {
			total = 345
		}
		return 200, fmt.Sprintf(`{"id":"upcoming","object":"invoice","total":%d,"amount_due":%d}`, total, total)
	case method == http.MethodDelete && path == "/v1/subscriptions/sub_r":
		return 200, `{"id":"sub_r","object":"subscription","status":"canceled"}`
	case method == http.MethodDelete && path == "/v1/customers/cus_r":
		return 200, `{"id":"cus_r","object":"customer","deleted":true}`
	default:
		return 500, fmt.Sprintf(`{"error":{"type":"api_error","message":"unexpected route %s %s"}}`, method, path)
	}
}

func TestReclaimUnboundCustomerDeletesOnlyWhenNothingIsWorthKeeping(t *testing.T) {
	now := time.Now()
	old := now.Add(-72 * time.Hour)
	cutoff := now.Add(-DefaultReclaimHorizon)
	cases := []struct {
		name        string
		fixture     reclaimFixture
		wantReclaim bool
		wantDelete  bool
	}{
		{"abandoned cardless customer", reclaimFixture{created: old, owner: "tea-r"}, true, true},
		{"already gone in Stripe", reclaimFixture{missing: true}, true, false},
		{"younger than the horizon", reclaimFixture{created: now.Add(-time.Hour), owner: "tea-r"}, false, false},
		{"tagged for another workspace", reclaimFixture{created: old, owner: "tea-other"}, false, false},
		{"retired deleted-workspace tombstone", reclaimFixture{created: old, owner: "tea-r", deletedMarker: true}, false, false},
		{"default payment method set", reclaimFixture{created: old, owner: "tea-r", defaultPM: true}, false, false},
		{"payment method attached", reclaimFixture{created: old, owner: "tea-r", attachedPM: true}, false, false},
		{"invoice that carried money", reclaimFixture{created: old, owner: "tea-r", moneyInvoice: true}, false, false},
		{"Checkout open right now", reclaimFixture{created: old, owner: "tea-r", openCheckout: true}, false, false},
		{"usage accrued this period", reclaimFixture{created: old, owner: "tea-r", accruedPreview: true}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, stub := newStripeTest(t, tc.fixture.route)
			c.storeCustomer("tea-r", "cus_r")
			reclaimed, err := c.ReclaimUnboundCustomer(context.Background(), "tea-r", "cus_r", cutoff)
			if err != nil {
				t.Fatalf("ReclaimUnboundCustomer: %v", err)
			}
			if reclaimed != tc.wantReclaim {
				t.Fatalf("reclaimed = %t, want %t (calls %v)", reclaimed, tc.wantReclaim, stub.hits)
			}
			deletes := len(stub.requests("/v1/customers/cus_r")) - 1 // minus the GET
			if (deletes > 0) != tc.wantDelete {
				t.Fatalf("Customer deleted = %t, want %t (calls %v)", deletes > 0, tc.wantDelete, stub.hits)
			}
			if tc.wantDelete && stub.count("/v1/subscriptions/sub_r") != 1 {
				t.Fatalf("Subscription was not cancelled before the Customer was deleted: %v", stub.hits)
			}
			if !tc.wantDelete && stub.count("/v1/subscriptions/sub_r") != 0 {
				t.Fatalf("kept Customer had its Subscription cancelled: %v", stub.hits)
			}
			if _, cached := c.lookupCustomer("tea-r"); cached == tc.wantReclaim {
				t.Fatalf("process cache still holds=%t after reclaimed=%t", cached, tc.wantReclaim)
			}
		})
	}
}

type reclaimStoreFake struct {
	claimed []store.BillingProviderMapping
	dropped []string
}

func (f *reclaimStoreFake) ClaimUnboundBillingCustomers(context.Context, bool, time.Duration, int) ([]store.BillingProviderMapping, error) {
	out := f.claimed
	f.claimed = nil
	return out, nil
}

func (f *reclaimStoreFake) DeleteUnboundBillingProviderMapping(_ context.Context, workspaceID, _ string) error {
	f.dropped = append(f.dropped, workspaceID)
	return nil
}

// The pass also sweeps Customers left by expired customer_creation sessions:
// the unadopted one is deleted, the adopted one spared, and a session that
// ran on a pre-existing Customer ignored.
func TestCheckoutReclaimerSweepsExpiredSessionCustomers(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/checkout/sessions":
			return 200, `{"object":"list","data":[` +
				`{"id":"cs_x1","object":"checkout.session","status":"expired","livemode":false,"customer":{"id":"cus_orphan","object":"customer","metadata":{}},"customer_creation":"always","metadata":{"bex_workspace":"tea-x"}},` +
				`{"id":"cs_x2","object":"checkout.session","status":"expired","livemode":false,"customer":{"id":"cus_adopted","object":"customer","metadata":{"bex_workspace":"tea-y"}},"customer_creation":"always","metadata":{"bex_workspace":"tea-y"}},` +
				`{"id":"cs_x4","object":"checkout.session","status":"expired","livemode":false,"customer":{"id":"cus_gone","object":"customer","deleted":true},"customer_creation":"always","metadata":{"bex_workspace":"tea-w"}},` +
				`{"id":"cs_x3","object":"checkout.session","status":"expired","livemode":false,"customer":"cus_existing","metadata":{"bex_workspace":"tea-z"}}` +
				`],"has_more":false,"url":"/v1/checkout/sessions"}`
		case method == http.MethodDelete && path == "/v1/customers/cus_orphan":
			return 200, `{"id":"cus_orphan","object":"customer","deleted":true}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	st := &reclaimStoreFake{}
	(&CheckoutReclaimer{Store: st, Provider: c}).runOnce(context.Background())
	// The list expands each Customer, so the sweep reads nothing per session.
	if stub.count("/v1/customers/cus_orphan") != 1 {
		t.Fatalf("unadopted expired-session Customer not deleted: %v", stub.hits)
	}
	if stub.count("/v1/customers/cus_adopted") != 0 || stub.count("/v1/customers/cus_gone") != 0 {
		t.Fatalf("adopted or already-deleted Customer was touched: %v", stub.hits)
	}
	if body := stub.requests("/v1/checkout/sessions"); len(body) != 1 {
		t.Fatalf("expired-session list calls = %d", len(body))
	}
	if stub.count("/v1/customers/cus_existing") != 0 {
		t.Fatalf("a session on a pre-existing Customer must be ignored: %v", stub.hits)
	}
}

func TestCheckoutReclaimerDropsMappingOnlyAfterReclaim(t *testing.T) {
	fixture := reclaimFixture{created: time.Now().Add(-72 * time.Hour), owner: "tea-r"}
	c, _ := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/checkout/sessions" {
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/checkout/sessions"}`
		}
		return fixture.route(method, path)
	})
	st := &reclaimStoreFake{claimed: []store.BillingProviderMapping{{WorkspaceID: "tea-r", CustomerID: "cus_r"}}}
	(&CheckoutReclaimer{Store: st, Provider: c}).runOnce(context.Background())
	if len(st.dropped) != 1 || st.dropped[0] != "tea-r" {
		t.Fatalf("dropped mappings = %v, want [tea-r]", st.dropped)
	}
}

// A workspace-create attempt whose Customer id never reached the database
// still has its Customer found (by attempt metadata) and deleted.
func TestCleanupWorkspaceSetupFindsUnpersistedCustomer(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/customers/search":
			return 200, `{"object":"search_result","data":[` +
				`{"id":"cus_leak","object":"customer","livemode":false,"metadata":{"bex_workspace":"tea-w","bex_workspace_creation_attempt":"wca-1"}},` +
				`{"id":"cus_wrongws","object":"customer","livemode":false,"metadata":{"bex_workspace":"tea-other","bex_workspace_creation_attempt":"wca-1"}}` +
				`],"has_more":false,"url":"/v1/customers/search"}`
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, emptySubscriptions
		case method == http.MethodDelete && path == "/v1/customers/cus_leak":
			return 200, `{"id":"cus_leak","object":"customer","deleted":true}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	if err := c.CleanupWorkspaceSetup(context.Background(), "wca-1", "tea-w", "", ""); err != nil {
		t.Fatalf("CleanupWorkspaceSetup: %v", err)
	}
	if stub.count("/v1/customers/cus_leak") != 1 || stub.count("/v1/customers/cus_wrongws") != 0 {
		t.Fatalf("cleanup calls = %v", stub.hits)
	}
}
