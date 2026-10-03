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
	"net/http"
	"strings"
	"sync"
	"testing"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const emptySearch = `{"object":"search_result","data":[],"has_more":false,"url":"/v1/customers/search"}`
const emptySubscriptions = `{"object":"list","data":[],"has_more":false,"url":"/v1/subscriptions"}`

// A workspace with no Customer opens Checkout without minting anything: no
// Customer, no Subscription — Stripe creates the Customer only on submit.
func TestStripeCreateCheckoutSessionMintsNothingForNewWorkspace(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/customers/search":
			return 200, emptySearch
		case method == http.MethodPost && path == "/v1/checkout/sessions":
			return 200, `{"id":"cs_test_new","object":"checkout.session","mode":"setup","livemode":false,"expires_at":1785200000,"url":"https://checkout.stripe.com/c/pay/cs_test_new"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	c.dashboardURL = "https://dashboard.bex.co"
	state := &billingStateStoreFake{}
	c.state = state
	if _, err := c.CreateCheckoutSession(context.Background(), "tea-new", CheckoutRequest{
		SuccessURL: "https://dashboard.bex.co/usage?billing=success",
		CancelURL:  "https://dashboard.bex.co/usage?billing=cancelled",
	}); err != nil {
		t.Fatalf("CreateCheckoutSession: %v", err)
	}
	if got := stub.requests("/v1/customers"); len(got) != 0 {
		t.Fatalf("Customer created before Checkout: %+v", got)
	}
	if got := stub.requests("/v1/subscriptions"); len(got) != 0 {
		t.Fatalf("Subscription touched before Checkout: %+v", got)
	}
	if len(state.mappings) != 0 {
		t.Fatalf("mapping persisted before Checkout: %+v", state.mappings)
	}
	body := stub.requests("/v1/checkout/sessions")[0].body
	if !strings.Contains(body, "customer_creation=always") || strings.Contains(body, "customer=") || strings.Contains(body, "customer_update") {
		t.Fatalf("Checkout body = %s, want customer_creation=always and no customer/customer_update", body)
	}
}

// adoptionStub models the Stripe objects of a completed customer_creation
// session: Stripe created cus_new on submit, and the Subscription exists only
// once bex creates it.
type adoptionStub struct {
	mu            sync.Mutex
	customerOwner string // bex_workspace metadata on cus_new
	subscription  bool
}

func (a *adoptionStub) route(method, path string) (int, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case method == http.MethodGet && path == "/v1/checkout/sessions/cs_new":
		return 200, `{"id":"cs_new","object":"checkout.session","mode":"setup","status":"complete","livemode":false,"customer":"cus_new","customer_creation":"always","setup_intent":"seti_new","metadata":{"bex_workspace":"tea-new"}}`
	case method == http.MethodGet && path == "/v1/customers/search":
		return 200, emptySearch
	case method == http.MethodGet && path == "/v1/customers/cus_new":
		meta := `{}`
		if a.customerOwner != "" {
			meta = `{"bex_workspace":"` + a.customerOwner + `"}`
		}
		return 200, `{"id":"cus_new","object":"customer","livemode":false,"metadata":` + meta + `}`
	case method == http.MethodPost && path == "/v1/customers/cus_new":
		a.customerOwner = "tea-new"
		return 200, `{"id":"cus_new","object":"customer","livemode":false,"metadata":{"bex_workspace":"tea-new"}}`
	case method == http.MethodGet && path == "/v1/subscriptions":
		if !a.subscription {
			return 200, emptySubscriptions
		}
		return 200, `{"object":"list","data":[{"id":"sub_new","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-new","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
	case method == http.MethodPost && path == "/v1/subscriptions":
		a.subscription = true
		return 200, `{"id":"sub_new","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-new","bex_billing_contract":"true"}}`
	case method == http.MethodGet && path == "/v1/setup_intents/seti_new":
		return 200, `{"id":"seti_new","object":"setup_intent","status":"succeeded","customer":"cus_new","payment_method":"pm_new","metadata":{"bex_workspace":"tea-new"}}`
	case method == http.MethodGet && path == "/v1/payment_methods/pm_new":
		return 200, `{"id":"pm_new","object":"payment_method","customer":"cus_new","livemode":false,"type":"card","billing_details":{"address":{"country":"US","postal_code":"94107","line1":"1 Main St","city":"SF","state":"CA"}}}`
	case method == http.MethodPost && path == "/v1/subscriptions/sub_new":
		return 200, `{"id":"sub_new","object":"subscription","status":"active","default_payment_method":"pm_new"}`
	default:
		return 500, `{"error":{"type":"api_error","message":"unexpected route ` + method + " " + path + `"}}`
	}
}

func TestStripeCompleteCheckoutSessionAdoptsCheckoutCustomerAndCreatesContract(t *testing.T) {
	adoption := &adoptionStub{}
	c, stub := newStripeTest(t, adoption.route)
	state := &billingStateStoreFake{}
	c.state = state
	c.priceIDs = []string{"price_a", "price_b"}

	// A webhook retry must converge: one adoption, one Subscription, two
	// idempotent bind stamps.
	for i := 0; i < 2; i++ {
		if err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_new"}); err != nil {
			t.Fatalf("CompleteCheckoutSession #%d: %v", i+1, err)
		}
	}
	if creates := stub.requests("/v1/subscriptions"); countPosts(creates) != 1 {
		t.Fatalf("Subscription creates = %d, want exactly 1", countPosts(creates))
	}
	if got, ok, _ := state.BillingCustomerID(context.Background(), "tea-new", false); !ok || got != "cus_new" {
		t.Fatalf("mapping = %q/%t, want cus_new", got, ok)
	}
	customerPosts := stub.requests("/v1/customers/cus_new")
	adopts := 0
	var binds []string
	for _, r := range customerPosts {
		switch {
		case strings.Contains(r.body, "metadata[bex_workspace]=tea-new"):
			adopts++
		case strings.Contains(r.body, "invoice_settings[default_payment_method]=pm_new"):
			binds = append(binds, r.body)
		}
	}
	if adopts != 1 {
		t.Fatalf("Customer adoptions = %d, want 1: %+v", adopts, customerPosts)
	}
	// Same idempotency key ⇒ Stripe requires identical parameters on retry.
	if len(binds) != 2 || binds[0] != binds[1] {
		t.Fatalf("bind requests differ across webhook retries: %q", binds)
	}
	if !strings.Contains(binds[0], "address[country]=US") || !strings.Contains(binds[0], "address[postal_code]=94107") {
		t.Fatalf("bind did not seed the adopted Customer's address from the card: %s", binds[0])
	}
	if len(state.boundWorkspaces) != 2 || state.boundWorkspaces[0] != "tea-new" {
		t.Fatalf("payment marker stamps = %#v", state.boundWorkspaces)
	}
}

// countPosts counts form-bodied requests — Stripe mutations; reads are GETs
// with an empty body.
func countPosts(reqs []struct {
	body   string
	header http.Header
}) int {
	n := 0
	for _, r := range reqs {
		if r.body != "" {
			n++
		}
	}
	return n
}

// Two tabs, both completed: the workspace already adopted the first tab's
// Customer, so the second tab's Checkout-created Customer is deleted and the
// event acknowledged — never bound, never retried.
func TestStripeCompleteCheckoutSessionDiscardsSupersededCheckoutCustomer(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/checkout/sessions/cs_second":
			return 200, `{"id":"cs_second","object":"checkout.session","mode":"setup","status":"complete","livemode":false,"customer":"cus_second","customer_creation":"always","setup_intent":"seti_2","metadata":{"bex_workspace":"tea-a"}}`
		case method == http.MethodDelete && path == "/v1/customers/cus_second":
			return 200, `{"id":"cus_second","object":"customer","deleted":true}`
		default:
			return 500, `{"error":{"type":"api_error","message":"must not bind"}}`
		}
	})
	state := &billingStateStoreFake{}
	_ = state.UpsertBillingProviderMapping(context.Background(), store.BillingProviderMapping{WorkspaceID: "tea-a", CustomerID: "cus_first"})
	c.state = state
	if err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_second"}); err != nil {
		t.Fatalf("superseded completion = %v, want acknowledged", err)
	}
	if stub.count("/v1/customers/cus_second") != 1 || stub.count("/v1/customers/cus_first") != 0 || stub.count("/v1/subscriptions") != 0 {
		t.Fatalf("superseded completion calls = %v", stub.hits)
	}
	if len(state.boundWorkspaces) != 0 {
		t.Fatalf("superseded completion stamped a binding: %v", state.boundWorkspaces)
	}
}

// A Checkout-created Customer already tagged for another workspace is never
// adopted, even when the session names this workspace.
func TestStripeCompleteCheckoutSessionRefusesForeignTaggedCustomer(t *testing.T) {
	adoption := &adoptionStub{customerOwner: "tea-other"}
	c, stub := newStripeTest(t, adoption.route)
	err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_new"})
	if err == nil || !isInputError(err) || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("foreign-tagged adoption error = %v", err)
	}
	if countPosts(stub.requests("/v1/customers/cus_new")) != 0 || countPosts(stub.requests("/v1/subscriptions")) != 0 {
		t.Fatalf("foreign-tagged adoption mutated Stripe: %v", stub.hits)
	}
}

// Before any card is bound, readiness reports no Customer and no Subscription —
// and reading it never mints either (the dashboard polls it every 15s).
func TestStripeReadinessForNewWorkspaceIsEmptyAndMintsNothing(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/customers/search" {
			return 200, emptySearch
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.state = &billingStateStoreFake{}
	got, err := c.Readiness(context.Background(), "tea-new")
	if err != nil {
		t.Fatalf("Readiness: %v", err)
	}
	if got.CustomerReady || got.SubscriptionReady || got.PaymentMethodReady {
		t.Fatalf("readiness before Checkout = %+v, want nothing ready", got)
	}
	if countPosts(stub.requests("/v1/customers")) != 0 || stub.count("/v1/subscriptions") != 0 {
		t.Fatalf("Readiness touched Stripe objects: %v", stub.hits)
	}
}

// A second replica must see a Customer another replica just adopted even
// though Customer Search has not indexed the new tag yet; otherwise the usage
// emitter would mint a duplicate for the freshly bound workspace.
func TestStripeEnsureCustomerReadsPersistedMappingBeforeSearch(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/customers/search" {
			return 200, emptySearch // the index lags the adoption
		}
		return 500, `{"error":{"type":"api_error","message":"must not create"}}`
	})
	state := &billingStateStoreFake{}
	_ = state.UpsertBillingProviderMapping(context.Background(), store.BillingProviderMapping{WorkspaceID: "tea-a", CustomerID: "cus_adopted"})
	c.state = state
	if err := c.EnsureCustomer(context.Background(), "tea-a"); err != nil {
		t.Fatalf("EnsureCustomer: %v", err)
	}
	if id, _ := c.lookupCustomer("tea-a"); id != "cus_adopted" {
		t.Fatalf("resolved Customer = %q, want the persisted cus_adopted", id)
	}
	if len(stub.hits) != 0 {
		t.Fatalf("EnsureCustomer reached Stripe despite a persisted mapping: %v", stub.hits)
	}
}

// Another replica reclaimed the workspace's Customer and dropped the mapping.
// This replica's cache still holds the old id; the mapping wins, so a
// completion's fresh Checkout Customer is adopted — not discarded as if the
// workspace still owned the reclaimed one (which would delete the new card).
func TestStripeCompleteCheckoutSessionIgnoresCustomerReclaimedByAnotherReplica(t *testing.T) {
	adoption := &adoptionStub{}
	c, stub := newStripeTest(t, adoption.route)
	c.state = &billingStateStoreFake{}
	c.priceIDs = []string{"price_a"}
	c.storeCustomer("tea-new", "cus_reclaimed")
	if err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_new"}); err != nil {
		t.Fatalf("CompleteCheckoutSession: %v", err)
	}
	if stub.count("/v1/customers/cus_reclaimed") != 0 {
		t.Fatalf("stale cached Customer was used: %v", stub.hits)
	}
	if id, _ := c.lookupCustomer("tea-new"); id != "cus_new" {
		t.Fatalf("workspace Customer = %q, want the adopted cus_new", id)
	}
}
