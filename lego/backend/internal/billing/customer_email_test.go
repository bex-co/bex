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
	"net/http"
	"net/url"
	"slices"
	"testing"

	stripe "github.com/stripe/stripe-go/v86"
)

// w6/m627: a workspace's billing email becomes its Stripe Customer's email.
func TestSetCustomerEmailUpdatesTheWorkspacesCustomer(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodPost && path == "/v1/customers/cus_1" {
			return 200, `{"id":"cus_1","object":"customer","email":"finance@example.com"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	if err := c.SetCustomerEmail(context.Background(), "tea-a", "finance@example.com"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stub.hits, []string{"/v1/customers/cus_1"}) {
		t.Fatalf("hits = %v, want the one Customer update", stub.hits)
	}
	form, err := url.ParseQuery(stub.bodies[0])
	if err != nil || form.Get("email") != "finance@example.com" || len(form) != 1 {
		t.Fatalf("update body = %q, want only the email", stub.bodies[0])
	}
}

// A workspace with no Customer has nothing to update, and none is created:
// a Customer made later takes the billing email in effect then (w6/179).
func TestSetCustomerEmailWithoutACustomerChangesNothing(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/customers/search" {
			return 200, `{"object":"search_result","data":[],"has_more":false,"url":"/v1/customers/search"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	if err := c.SetCustomerEmail(context.Background(), "tea-a", "finance@example.com"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stub.hits, []string{"/v1/customers/search"}) {
		t.Fatalf("hits = %v, want only the search", stub.hits)
	}
}

// A failed update is returned, so the caller leaves bex's record unchanged;
// a Customer deleted under bex is not an error.
func TestSetCustomerEmailReportsAFailedUpdate(t *testing.T) {
	for name, tc := range map[string]struct {
		status  int
		body    string
		wantErr bool
	}{
		"stripe down":   {500, `{"error":{"type":"api_error","message":"down"}}`, true},
		"customer gone": {404, `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"No such customer"}}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := newStripeTest(t, func(string, string) (int, string) { return tc.status, tc.body })
			c.storeCustomer("tea-a", "cus_1")
			err := c.SetCustomerEmail(context.Background(), "tea-a", "finance@example.com")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
		})
	}
}

// w6/179: every Customer bex creates takes the workspace's billing email in
// effect, through the BillingEmail seam cmd/api wires from
// workspaces.BillingEmailsInEffect: the owner's account email on
// Hobby, the workspace's own address on Pro and higher (that choice is
// tested there, and composed in api.TestNewCustomersTakeTheBillingEmailE2E).

// billingEmails answers each workspace's billing email in effect, or err.
func billingEmails(emails map[string]string, err error) func(context.Context, string) (string, error) {
	return func(_ context.Context, id string) (string, error) {
		if err != nil {
			return "", err
		}
		return emails[id], nil
	}
}

// A Checkout that will create the Customer sends the billing email as
// customer_email. One bex cannot name, or cannot read, is not sent, and
// the Checkout still opens: Stripe asks for the email, as before.
func TestCheckoutForAWorkspaceWithNoCustomerSendsItsBillingEmail(t *testing.T) {
	for name, tc := range map[string]struct {
		email string
		err   error
		want  string // customer_email; "" when none is sent
	}{
		"Hobby: the owner's account email":    {"owner@example.com", nil, "owner@example.com"},
		"Pro: its own address":                {"finance@example.com", nil, "finance@example.com"},
		"an owner whose email cannot be read": {"", nil, ""},
		"a failed read":                       {"", errors.New("db down"), ""},
	} {
		t.Run(name, func(t *testing.T) {
			c, stub := newStripeTest(t, func(method, path string) (int, string) {
				switch {
				case method == http.MethodGet && path == "/v1/customers/search":
					return 200, emptySearch
				case method == http.MethodPost && path == "/v1/checkout/sessions":
					return 200, `{"id":"cs_test_new","object":"checkout.session","mode":"setup","livemode":false,"expires_at":1785200000,"url":"https://checkout.stripe.com/c/pay/cs_test_new"}`
				}
				return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
			})
			c.dashboardURL = "https://dashboard.bex.co"
			c.billingEmail = billingEmails(map[string]string{"tea-new": tc.email}, tc.err)
			if _, err := c.CreateCheckoutSession(context.Background(), "tea-new", CheckoutRequest{
				SuccessURL: "https://dashboard.bex.co/usage?billing=success",
				CancelURL:  "https://dashboard.bex.co/usage?billing=cancelled",
			}); err != nil {
				t.Fatalf("CreateCheckoutSession: %v", err)
			}
			form := formOf(t, stub.requests("/v1/checkout/sessions")[0].body)
			if form.Get("customer_creation") != "always" || form.Has("customer_email") != (tc.want != "") || form.Get("customer_email") != tc.want {
				t.Fatalf("Checkout body = %v, want customer_creation=always and customer_email %q", form, tc.want)
			}
		})
	}
}

// A workspace that has a Customer opens Checkout on it: Stripe refuses
// customer_email beside customer, and the Customer already has its email.
func TestCheckoutForAnExistingCustomerSendsNoEmail(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, emptySubscriptions
		case method == http.MethodPost && path == "/v1/checkout/sessions":
			return 200, `{"id":"cs_test_1","object":"checkout.session","mode":"setup","livemode":false,"expires_at":1785200000,"url":"https://checkout.stripe.com/c/pay/cs_test_1"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.dashboardURL = "https://dashboard.bex.co"
	c.storeCustomer("tea-a", "cus_1")
	c.billingEmail = func(context.Context, string) (string, error) {
		t.Fatal("read the billing email for a workspace that has a Customer")
		return "", nil
	}
	if _, err := c.CreateCheckoutSession(context.Background(), "tea-a", CheckoutRequest{
		SuccessURL: "https://dashboard.bex.co/usage?billing=success",
		CancelURL:  "https://dashboard.bex.co/usage?billing=cancelled",
	}); err != nil {
		t.Fatalf("CreateCheckoutSession: %v", err)
	}
	if form := formOf(t, stub.requests("/v1/checkout/sessions")[0].body); form.Get("customer") != "cus_1" || form.Has("customer_email") {
		t.Fatalf("Checkout body = %v, want customer cus_1 and no customer_email", form)
	}
}

// The Customer the emitter or the admin path creates carries the billing
// email; with none bex can name it carries none, and a failed read creates
// nothing, so the emitter retries rather than mint a Customer without it.
func TestEnsureCustomerCreatesTheCustomerWithItsBillingEmail(t *testing.T) {
	for name, tc := range map[string]struct {
		email   string
		err     error
		creates int
	}{
		"its billing email":          {"finance@example.com", nil, 1},
		"none bex can name":          {"", nil, 1},
		"a failed read creates none": {"", errors.New("db down"), 0},
	} {
		t.Run(name, func(t *testing.T) {
			c, stub := newStripeTest(t, func(method, path string) (int, string) {
				if path == "/v1/customers/search" {
					return 200, emptySearch
				}
				return 200, `{"id":"cus_123","object":"customer"}`
			})
			c.billingEmail = billingEmails(map[string]string{"tea-abc": tc.email}, tc.err)
			if err := c.EnsureCustomer(context.Background(), "tea-abc"); (err != nil) != (tc.err != nil) {
				t.Fatalf("EnsureCustomer = %v, want error %v", err, tc.err != nil)
			}
			creates := stub.requests("/v1/customers")
			if len(creates) != tc.creates {
				t.Fatalf("creates = %d, want %d", len(creates), tc.creates)
			}
			if tc.creates == 1 {
				if form := formOf(t, creates[0].body); form.Has("email") != (tc.email != "") || form.Get("email") != tc.email {
					t.Fatalf("create body = %v, want email %q", form, tc.email)
				}
			}
		})
	}
}

// Once its card binds, a Customer Checkout created takes the billing email
// in effect: an address the member typed because bex could not name one when
// the session opened, or one a Pro workspace changed while it was open, is
// replaced. One already right, or none bex can name, changes nothing.
func TestCompletedCheckoutGivesItsCustomerTheBillingEmail(t *testing.T) {
	for name, tc := range map[string]struct {
		typed, inEffect string
		want            string // the email written; "" when none is
	}{
		"a typed address is replaced":       {"typed@example.com", "owner@example.com", "owner@example.com"},
		"the address Checkout was given":    {"finance@example.com", "finance@example.com", ""},
		"none bex can name keeps the typed": {"typed@example.com", "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			adoption := &adoptionStub{email: tc.typed}
			c, stub := newStripeTest(t, adoption.route)
			c.state = &billingStateStoreFake{}
			c.priceIDs = []string{"price_a"}
			c.workspaceLabel = labels(map[string]string{"tea-new": "Acme Prod"})
			c.billingEmail = billingEmails(map[string]string{"tea-new": tc.inEffect}, nil)
			// A retried delivery runs it again with the same answer.
			for i := 0; i < 2; i++ {
				if err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_new"}); err != nil {
					t.Fatal(err)
				}
			}
			var emailed []string
			for _, r := range stub.requests("/v1/customers/cus_new") {
				if form := formOf(t, r.body); form.Has("email") {
					emailed = append(emailed, form.Get("email"))
				}
			}
			if tc.want == "" && len(emailed) != 0 {
				t.Fatalf("email writes = %v, want none", emailed)
			}
			// The stand-in does not keep the write, so each delivery repeats it.
			if tc.want != "" && !slices.Equal(emailed, []string{tc.want, tc.want}) {
				t.Fatalf("email writes = %v, want %q on each delivery", emailed, tc.want)
			}
		})
	}
}
