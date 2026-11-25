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
	"strings"
	"testing"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m628: invoices bill the workspace by name. Stripe prints a Customer's
// name as an invoice's Bill-to, so every Customer bex creates is named after
// its workspace, and a rename follows while the name is still bex's.

func labels(names map[string]string) func(context.Context, string) (string, error) {
	return func(_ context.Context, id string) (string, error) {
		name, ok := names[id]
		if !ok {
			return "", store.ErrNotFound
		}
		return name, nil
	}
}

func formOf(t *testing.T, body string) url.Values {
	t.Helper()
	form, err := url.ParseQuery(body)
	if err != nil {
		t.Fatalf("parse %q: %v", body, err)
	}
	return form
}

func TestEnsureCustomerNamesTheCustomerAfterItsWorkspace(t *testing.T) {
	for name, tc := range map[string]struct {
		labels   map[string]string
		wantName string
	}{
		"its display name":                  {map[string]string{"tea-abc": "Acme Prod"}, "Acme Prod"},
		"a workspace bex cannot find bills": {map[string]string{}, "tea-abc"},
	} {
		t.Run(name, func(t *testing.T) {
			c, stub := newStripeTest(t, func(method, path string) (int, string) {
				if path == "/v1/customers/search" {
					return 200, emptySearch
				}
				return 200, `{"id":"cus_123","object":"customer"}`
			})
			c.workspaceLabel = labels(tc.labels)
			if err := c.EnsureCustomer(context.Background(), "tea-abc"); err != nil {
				t.Fatal(err)
			}
			creates := stub.requests("/v1/customers")
			if len(creates) != 1 {
				t.Fatalf("creates = %d, want 1", len(creates))
			}
			if form := formOf(t, creates[0].body); form.Get("name") != tc.wantName || form.Get("metadata[bex_workspace]") != "tea-abc" {
				t.Fatalf("create body = %v, want name %q tagged tea-abc", form, tc.wantName)
			}
		})
	}
}

// A name that cannot be read is not guessed: EnsureCustomer fails before it
// creates anything, and its caller (the emitter) retries.
func TestEnsureCustomerCreatesNothingWhenTheNameCannotBeRead(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if path == "/v1/customers/search" {
			return 200, emptySearch
		}
		return 200, `{"id":"cus_123","object":"customer"}`
	})
	c.workspaceLabel = func(context.Context, string) (string, error) { return "", errors.New("db down") }
	if err := c.EnsureCustomer(context.Background(), "tea-abc"); err == nil {
		t.Fatal("EnsureCustomer succeeded without the workspace's name")
	}
	if n := len(stub.requests("/v1/customers")); n != 0 {
		t.Fatalf("creates = %d, want none", n)
	}
}

func TestFollowWorkspaceRenameRenamesOnlyANameThatIsStillBexs(t *testing.T) {
	for name, tc := range map[string]struct {
		customer   string // the Customer's JSON fields after id/object
		wantRename bool
	}{
		"named by its id (before w6/m628)": {`"name":"tea-abc"`, true},
		"named after its old name":         {`"name":"Old Name"`, true},
		"unnamed":                          {`"name":""`, true},
		"a company name from Billing info": {`"name":"Old Name","metadata":{"bex_company_name":"true"}`, false},
		"a name typed into the portal":     {`"name":"Jane Doe"`, false},
		"already the new name":             {`"name":"New Name"`, false},
	} {
		t.Run(name, func(t *testing.T) {
			c, stub := newStripeTest(t, func(method, path string) (int, string) {
				if path == "/v1/customers/cus_1" && method == http.MethodGet {
					return 200, `{"id":"cus_1","object":"customer",` + tc.customer + `}`
				}
				if path == "/v1/customers/cus_1" && method == http.MethodPost {
					return 200, `{"id":"cus_1","object":"customer","name":"New Name"}`
				}
				return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
			})
			c.storeCustomer("tea-abc", "cus_1")
			if err := c.FollowWorkspaceRename(context.Background(), "tea-abc", "Old Name", "New Name"); err != nil {
				t.Fatal(err)
			}
			posts := countPosts(stub.requests("/v1/customers/cus_1"))
			if (posts == 1) != tc.wantRename || posts > 1 {
				t.Fatalf("renames = %d, want rename %v", posts, tc.wantRename)
			}
			if tc.wantRename {
				reqs := stub.requests("/v1/customers/cus_1")
				if form := formOf(t, reqs[len(reqs)-1].body); form.Get("name") != "New Name" || len(form) != 1 {
					t.Fatalf("rename body = %v, want only the new name", form)
				}
			}
		})
	}
}

func TestFollowWorkspaceRenameWithoutACustomerChangesNothing(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if path == "/v1/customers/search" {
			return 200, emptySearch
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	if err := c.FollowWorkspaceRename(context.Background(), "tea-abc", "Old", "New"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stub.hits, []string{"/v1/customers/search"}) {
		t.Fatalf("hits = %v, want only the search", stub.hits)
	}
}

// A Customer Checkout created bills the workspace's name too; a business
// name Checkout collected with a tax ID is the member's company name and
// stays, marked as one.
func TestCompletedCheckoutNamesTheCustomerItCreated(t *testing.T) {
	for name, tc := range map[string]struct {
		businessName string
		wantName     string
		wantMarked   bool
	}{
		"after the workspace":     {"", "Acme Prod", false},
		"keeping a business name": {"Acme GmbH", "", true},
	} {
		t.Run(name, func(t *testing.T) {
			adoption := &adoptionStub{businessName: tc.businessName}
			c, stub := newStripeTest(t, adoption.route)
			c.state = &billingStateStoreFake{}
			c.priceIDs = []string{"price_a"}
			c.workspaceLabel = labels(map[string]string{"tea-new": "Acme Prod"})
			if err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_new"}); err != nil {
				t.Fatal(err)
			}
			var named, marked int
			for _, r := range stub.requests("/v1/customers/cus_new") {
				form := formOf(t, r.body)
				if form.Has("name") {
					named++
					if form.Get("name") != tc.wantName {
						t.Fatalf("named %q, want %q", form.Get("name"), tc.wantName)
					}
				}
				if form.Get("metadata[bex_company_name]") == "true" {
					marked++
				}
			}
			if (named == 1) != (tc.wantName != "") || named > 1 || (marked == 1) != tc.wantMarked {
				t.Fatalf("name writes = %d, company marks = %d; want name %q, marked %v", named, marked, tc.wantName, tc.wantMarked)
			}
		})
	}
}

// billingInfoStripe is a Stripe stand-in for one Customer and its tax IDs.
type billingInfoStripe struct {
	customer string // the Customer's JSON fields after id/object
	taxIDs   string // the tax_ids list's data entries
	order    []string
}

func (s *billingInfoStripe) route(method, path string) (int, string) {
	s.order = append(s.order, method+" "+path)
	switch {
	case method == http.MethodGet && path == "/v1/customers/search":
		return 200, `{"object":"search_result","data":[{"id":"cus_1","object":"customer"}],"has_more":false,"url":"/v1/customers/search"}`
	case method == http.MethodGet && path == "/v1/customers/cus_1":
		return 200, `{"id":"cus_1","object":"customer",` + s.customer + `,"tax_ids":{"object":"list","data":[` + s.taxIDs + `],"has_more":false}}`
	case method == http.MethodPost && path == "/v1/customers/cus_1/tax_ids":
		return 200, `{"id":"txi_new","object":"tax_id","type":"eu_vat","value":"DE123456789"}`
	case method == http.MethodPost && path == "/v1/customers/cus_1":
		return 200, `{"id":"cus_1","object":"customer"}`
	case method == http.MethodDelete && strings.HasPrefix(path, "/v1/customers/cus_1/tax_ids/"):
		return 200, `{"id":"` + strings.TrimPrefix(path, "/v1/customers/cus_1/tax_ids/") + `","object":"tax_id","deleted":true}`
	}
	return 500, `{"error":{"type":"api_error","message":"unexpected route ` + method + ` ` + path + `"}}`
}

func newBillingInfoTest(t *testing.T, s *billingInfoStripe) (*StripeClient, *stripeStub) {
	t.Helper()
	c, stub := newStripeTest(t, s.route)
	c.workspaceLabel = labels(map[string]string{"tea-abc": "Acme Prod"})
	return c, stub
}

func TestCustomerBillingInfoReadsWhatInvoicesPrint(t *testing.T) {
	for name, tc := range map[string]struct {
		customer, taxIDs string
		want             BillingInfo
	}{
		"bills the workspace": {
			customer: `"name":"Acme Prod"`,
			want:     BillingInfo{WorkspaceID: "tea-abc", CustomerReady: true, BillTo: "Acme Prod"},
		},
		"bills its id, before the backfill": {
			customer: `"name":"tea-abc"`,
			want:     BillingInfo{WorkspaceID: "tea-abc", CustomerReady: true, BillTo: "tea-abc"},
		},
		"a company, its address and the newest tax ID": {
			customer: `"name":"Acme GmbH","metadata":{"bex_company_name":"true"},"address":{"line1":"Hauptstr. 1","city":"Berlin","postal_code":"10115","country":"DE"}`,
			taxIDs:   `{"id":"txi_2","object":"tax_id","type":"eu_vat","value":"DE123456789"},{"id":"txi_1","object":"tax_id","type":"de_stn","value":"1234567890"}`,
			want: BillingInfo{
				WorkspaceID: "tea-abc", CustomerReady: true, BillTo: "Acme GmbH", CompanyName: "Acme GmbH",
				Address: &BillingAddress{Line1: "Hauptstr. 1", City: "Berlin", PostalCode: "10115", Country: "DE"},
				TaxID:   &BillingTaxID{Type: "eu_vat", Value: "DE123456789"},
			},
		},
		"a name typed into the portal is a company name": {
			customer: `"name":"Jane Doe"`,
			want:     BillingInfo{WorkspaceID: "tea-abc", CustomerReady: true, BillTo: "Jane Doe", CompanyName: "Jane Doe"},
		},
		"a retired Customer is none": {
			customer: `"name":"","metadata":{"bex_deleted_at":"2026-11-01T00:00:00Z"}`,
			want:     BillingInfo{WorkspaceID: "tea-abc"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			c, stub := newBillingInfoTest(t, &billingInfoStripe{customer: tc.customer, taxIDs: tc.taxIDs})
			got, err := c.CustomerBillingInfo(context.Background(), "tea-abc")
			if err != nil {
				t.Fatal(err)
			}
			if !billingInfoEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			if q := stub.queriesFor("/v1/customers/cus_1"); len(q) != 1 || !strings.Contains(q[0], "expand") {
				t.Fatalf("Customer reads = %v, want one with its tax IDs expanded", q)
			}
		})
	}
}

func TestCustomerBillingInfoWithoutACustomerCreatesNone(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		if path == "/v1/customers/search" {
			return 200, emptySearch
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	got, err := c.CustomerBillingInfo(context.Background(), "tea-abc")
	if err != nil || got.CustomerReady {
		t.Fatalf("got %+v, %v; want not ready", got, err)
	}
	if _, err := c.SetCustomerBillingInfo(context.Background(), "tea-abc", BillingInfoInput{CompanyName: "Acme"}); !isStateError(err) {
		t.Fatalf("write without a Customer = %v, want a state error", err)
	}
	if !slices.Equal(stub.hits, []string{"/v1/customers/search", "/v1/customers/search"}) {
		t.Fatalf("hits = %v, want only the searches", stub.hits)
	}
}

// A new tax ID is added before anything else changes, so one Stripe refuses
// changes nothing; the one it replaces is removed last.
func TestSetCustomerBillingInfoReplacesNameAddressAndTaxID(t *testing.T) {
	s := &billingInfoStripe{customer: `"name":"tea-abc"`, taxIDs: `{"id":"txi_old","object":"tax_id","type":"gb_vat","value":"GB123456789"}`}
	c, stub := newBillingInfoTest(t, s)
	got, err := c.SetCustomerBillingInfo(context.Background(), "tea-abc", BillingInfoInput{
		CompanyName: "Acme GmbH",
		Address:     &BillingAddress{Line1: "Hauptstr. 1", City: "Berlin", PostalCode: "10115", Country: "DE"},
		TaxID:       &BillingTaxID{Type: "eu_vat", Value: "de 123456789"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{
		"GET /v1/customers/search", "GET /v1/customers/cus_1",
		"POST /v1/customers/cus_1/tax_ids", "POST /v1/customers/cus_1", "DELETE /v1/customers/cus_1/tax_ids/txi_old",
	}
	if !slices.Equal(s.order, wantOrder) {
		t.Fatalf("calls = %v, want %v", s.order, wantOrder)
	}
	if form := formOf(t, stub.requests("/v1/customers/cus_1/tax_ids")[0].body); form.Get("type") != "eu_vat" || form.Get("value") != "de 123456789" {
		t.Fatalf("tax ID create = %v", form)
	}
	update := formOf(t, countPostBodies(stub.requests("/v1/customers/cus_1"))[0])
	for key, want := range map[string]string{
		"name": "Acme GmbH", "metadata[bex_company_name]": "true",
		"address[line1]": "Hauptstr. 1", "address[city]": "Berlin", "address[postal_code]": "10115", "address[country]": "DE",
	} {
		if update.Get(key) != want {
			t.Fatalf("update %s = %q, want %q (%v)", key, update.Get(key), want, update)
		}
	}
	// The value reads back as Stripe stored it.
	if got.BillTo != "Acme GmbH" || got.TaxID == nil || got.TaxID.Value != "DE123456789" || got.Address.Country != "DE" {
		t.Fatalf("got %+v", got)
	}
}

// Clearing the company name bills the workspace's name again and drops the
// mark, a nil address is removed, and a nil tax ID removes the one there.
func TestSetCustomerBillingInfoClearsBackToTheWorkspace(t *testing.T) {
	s := &billingInfoStripe{
		customer: `"name":"Acme GmbH","metadata":{"bex_company_name":"true"},"address":{"country":"DE"}`,
		taxIDs:   `{"id":"txi_old","object":"tax_id","type":"eu_vat","value":"DE123456789"}`,
	}
	c, stub := newBillingInfoTest(t, s)
	got, err := c.SetCustomerBillingInfo(context.Background(), "tea-abc", BillingInfoInput{})
	if err != nil {
		t.Fatal(err)
	}
	update := formOf(t, countPostBodies(stub.requests("/v1/customers/cus_1"))[0])
	if update.Get("name") != "Acme Prod" || !update.Has("metadata[bex_company_name]") || update.Get("metadata[bex_company_name]") != "" || !update.Has("address") || update.Get("address") != "" {
		t.Fatalf("update = %v, want the workspace's name, the mark deleted and the address unset", update)
	}
	if !slices.Contains(s.order, "DELETE /v1/customers/cus_1/tax_ids/txi_old") || slices.Contains(s.order, "POST /v1/customers/cus_1/tax_ids") {
		t.Fatalf("calls = %v, want the old tax ID removed and none added", s.order)
	}
	if got.BillTo != "Acme Prod" || got.CompanyName != "" || got.Address != nil || got.TaxID != nil {
		t.Fatalf("got %+v", got)
	}
}

// Saving the tax ID already there keeps it: nothing is added or removed.
func TestSetCustomerBillingInfoKeepsTheSameTaxID(t *testing.T) {
	s := &billingInfoStripe{customer: `"name":"Acme Prod"`, taxIDs: `{"id":"txi_1","object":"tax_id","type":"eu_vat","value":"DE123456789"}`}
	c, _ := newBillingInfoTest(t, s)
	if _, err := c.SetCustomerBillingInfo(context.Background(), "tea-abc", BillingInfoInput{TaxID: &BillingTaxID{Type: "eu_vat", Value: "DE 123 456 789"}}); err != nil {
		t.Fatal(err)
	}
	for _, call := range s.order {
		if strings.Contains(call, "/tax_ids") {
			t.Fatalf("calls = %v, want the tax ID kept as it is", s.order)
		}
	}
}

// A tax ID Stripe refuses is a field error and changes nothing.
func TestSetCustomerBillingInfoReportsARefusedTaxIDAsAFieldError(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case path == "/v1/customers/search":
			return 200, `{"object":"search_result","data":[{"id":"cus_1","object":"customer"}],"has_more":false,"url":"/v1/customers/search"}`
		case method == http.MethodGet && path == "/v1/customers/cus_1":
			return 200, `{"id":"cus_1","object":"customer","name":"Acme Prod"}`
		case path == "/v1/customers/cus_1/tax_ids":
			return 400, `{"error":{"type":"invalid_request_error","code":"tax_id_invalid","param":"value","message":"Invalid value for eu_vat."}}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	_, err := c.SetCustomerBillingInfo(context.Background(), "tea-abc", BillingInfoInput{TaxID: &BillingTaxID{Type: "eu_vat", Value: "nope"}})
	var field *fieldError
	if !errors.As(err, &field) || field.field != "taxId.value" || !strings.Contains(field.message, "Invalid value for eu_vat") {
		t.Fatalf("err = %v, want a taxId.value field error carrying Stripe's reason", err)
	}
	if n := countPosts(stub.requests("/v1/customers/cus_1")); n != 0 {
		t.Fatalf("Customer updates = %d, want none after the refusal", n)
	}
}

func TestNormalizeBillingInfoNamesTheRefusedField(t *testing.T) {
	long := strings.Repeat("x", 257)
	for name, tc := range map[string]struct {
		in        BillingInfoInput
		want      BillingInfoInput
		wantField string
	}{
		"trims, uppercases the country, lowercases the type": {
			in:   BillingInfoInput{CompanyName: "  Acme ", Address: &BillingAddress{Line1: " 1 Main ", Country: "de"}, TaxID: &BillingTaxID{Type: " EU_VAT ", Value: " DE1 "}},
			want: BillingInfoInput{CompanyName: "Acme", Address: &BillingAddress{Line1: "1 Main", Country: "DE"}, TaxID: &BillingTaxID{Type: "eu_vat", Value: "DE1"}},
		},
		"blank address and tax ID are none": {
			in:   BillingInfoInput{Address: &BillingAddress{City: "  "}, TaxID: &BillingTaxID{Value: " "}},
			want: BillingInfoInput{},
		},
		"an address needs a country":   {in: BillingInfoInput{Address: &BillingAddress{Line1: "1 Main"}}, wantField: "address.country"},
		"a country is two letters":     {in: BillingInfoInput{Address: &BillingAddress{Country: "Germany"}}, wantField: "address.country"},
		"a type Stripe does not know":  {in: BillingInfoInput{TaxID: &BillingTaxID{Type: "xx_vat", Value: "1"}}, wantField: "taxId.type"},
		"a tax ID needs its value":     {in: BillingInfoInput{TaxID: &BillingTaxID{Type: "eu_vat"}}, wantField: "taxId.value"},
		"a company name over 256":      {in: BillingInfoInput{CompanyName: long}, wantField: "companyName"},
		"a line separator in the city": {in: BillingInfoInput{Address: &BillingAddress{City: "a\u2028b", Country: "DE"}}, wantField: "address.city"},
		"a newline in the company":     {in: BillingInfoInput{CompanyName: "a\nb"}, wantField: "companyName"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := normalizeBillingInfo(tc.in)
			if tc.wantField != "" {
				var field *fieldError
				if !errors.As(err, &field) || field.field != tc.wantField {
					t.Fatalf("err = %v, want a %s field error", err, tc.wantField)
				}
				if coded := field.coded(); coded.Code != BillingInfoInvalidCode || coded.Params["field"] != tc.wantField {
					t.Fatalf("coded = %+v", coded)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !billingInfoEqual(BillingInfo{CompanyName: got.CompanyName, Address: got.Address, TaxID: got.TaxID}, BillingInfo{CompanyName: tc.want.CompanyName, Address: tc.want.Address, TaxID: tc.want.TaxID}) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func billingInfoEqual(a, b BillingInfo) bool {
	eq := func(x, y any, nilX, nilY bool) bool { return nilX == nilY && (nilX || x == y) }
	return a.WorkspaceID == b.WorkspaceID && a.CustomerReady == b.CustomerReady && a.BillTo == b.BillTo && a.CompanyName == b.CompanyName &&
		eq(deref(a.Address), deref(b.Address), a.Address == nil, b.Address == nil) &&
		eq(deref(a.TaxID), deref(b.TaxID), a.TaxID == nil, b.TaxID == nil)
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

// countPostBodies is the bodies of the form-bodied requests, in order.
func countPostBodies(reqs []struct {
	body   string
	header http.Header
}) []string {
	var out []string
	for _, r := range reqs {
		if r.body != "" {
			out = append(out, r.body)
		}
	}
	return out
}
