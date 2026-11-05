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
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// checkoutCardStub answers a completed setup Checkout for workspace ws whose
// session cs_<card> succeeded on payment method pm_<card>; cards maps each
// card name to its funding and fingerprint. Detached methods read back
// without a Customer.
func checkoutCardStub(ws, customer string, cards map[string][2]string) func(method, path string) (int, string) {
	detached := map[string]bool{}
	return func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && strings.HasPrefix(path, "/v1/checkout/sessions/cs_"):
			card := strings.TrimPrefix(path, "/v1/checkout/sessions/cs_")
			return 200, fmt.Sprintf(`{"id":"cs_%s","object":"checkout.session","mode":"setup","status":"complete","livemode":false,"customer":%q,"setup_intent":"seti_%s","metadata":{"bex_workspace":%q}}`, card, customer, card, ws)
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, fmt.Sprintf(`{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active","metadata":{"bex_workspace":%q,"bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`, ws)
		case method == http.MethodGet && strings.HasPrefix(path, "/v1/setup_intents/seti_"):
			card := strings.TrimPrefix(path, "/v1/setup_intents/seti_")
			return 200, fmt.Sprintf(`{"id":"seti_%s","object":"setup_intent","status":"succeeded","customer":%q,"payment_method":"pm_%s","metadata":{"bex_workspace":%q}}`, card, customer, card, ws)
		case method == http.MethodGet && strings.HasPrefix(path, "/v1/payment_methods/pm_"):
			card := strings.TrimPrefix(path, "/v1/payment_methods/pm_")
			owner := fmt.Sprintf("%q", customer)
			if detached[card] {
				owner = "null"
			}
			return 200, fmt.Sprintf(`{"id":"pm_%s","object":"payment_method","customer":%s,"livemode":false,"type":"card","card":{"brand":"visa","last4":"4242","funding":%q,"fingerprint":%q}}`, card, owner, cards[card][0], cards[card][1])
		case method == http.MethodPost && strings.HasSuffix(path, "/detach"):
			card := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/payment_methods/pm_"), "/detach")
			detached[card] = true
			return 200, fmt.Sprintf(`{"id":"pm_%s","object":"payment_method","customer":null}`, card)
		case method == http.MethodGet && path == "/v1/tax/registrations":
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/tax/registrations"}`
		case method == http.MethodPost && path == "/v1/customers/"+customer:
			return 200, fmt.Sprintf(`{"id":%q,"object":"customer"}`, customer)
		case method == http.MethodPost && path == "/v1/subscriptions/sub_1":
			return 200, `{"id":"sub_1","object":"subscription","status":"active"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route ` + method + ` ` + path + `"}}`
		}
	}
}

type cardScreenFake struct {
	flagged    map[string]bool
	rejections map[string]store.PaymentMethodRejection
}

func (f *cardScreenFake) RecordPaymentMethodRejection(_ context.Context, workspaceID string, reason store.PaymentMethodRejection) error {
	if f.rejections == nil {
		f.rejections = map[string]store.PaymentMethodRejection{}
	}
	f.rejections[workspaceID] = reason
	return nil
}

func (f *cardScreenFake) UnpaidWriteOffForCardFingerprint(_ context.Context, fingerprint string) (bool, error) {
	return f.flagged[fingerprint], nil
}

// ADR091 D10.5–D10.6 at Checkout completion: a prepaid card and a card
// whose fingerprint carries an unpaid write-off are detached and never bound,
// with the reason recorded; a clean credit card binds.
func TestCompleteCheckoutSessionScreensCards(t *testing.T) {
	c, stub := newStripeTest(t, checkoutCardStub("tea-a", "cus_1", map[string][2]string{
		"prepaid": {"prepaid", "fp_prepaid"}, "flagged": {"debit", "fp_debt"}, "clean": {"credit", "fp_clean"},
	}))
	state := &billingStateStoreFake{}
	_ = state.UpsertBillingProviderMapping(context.Background(), store.BillingProviderMapping{WorkspaceID: "tea-a", CustomerID: "cus_1"})
	c.state = state
	cards := &cardScreenFake{flagged: map[string]bool{"fp_debt": true}}
	c.cards = cards
	for _, tc := range []struct {
		card   string
		reason store.PaymentMethodRejection
	}{
		{"prepaid", store.PaymentMethodRejectedPrepaid},
		{"flagged", store.PaymentMethodRejectedUnpaidBalance},
	} {
		for range 2 { // a redelivered event converges
			if err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_" + tc.card}); err != nil {
				t.Fatalf("%s: %v", tc.card, err)
			}
		}
		if got := stub.count("/v1/payment_methods/pm_" + tc.card + "/detach"); got != 1 {
			t.Fatalf("%s: detach calls = %d, want 1 (a redelivery finds it detached)", tc.card, got)
		}
		if cards.rejections["tea-a"] != tc.reason {
			t.Fatalf("%s: rejection = %q, want %q", tc.card, cards.rejections["tea-a"], tc.reason)
		}
		if len(state.boundWorkspaces) != 0 || stub.count("/v1/subscriptions/sub_1") != 0 {
			t.Fatalf("%s: a refused card was bound (stamps %v)", tc.card, state.boundWorkspaces)
		}
	}
	if err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_clean"}); err != nil {
		t.Fatal(err)
	}
	if len(state.boundWorkspaces) != 1 || stub.count("/v1/payment_methods/pm_clean/detach") != 0 {
		t.Fatalf("clean card: stamps %v, detaches %d", state.boundWorkspaces, stub.count("/pm_clean/detach"))
	}
}

// Workspace creation refuses a prepaid card with the typed reason, after
// detaching it; a nil card store still refuses prepaid cards.
func TestVerifyWorkspaceSetupRefusesPrepaidCard(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/setup_intents/seti_1":
			return 200, `{"id":"seti_1","object":"setup_intent","status":"succeeded","customer":"cus_1","payment_method":"pm_1","livemode":false,"metadata":{"bex_workspace":"tea-new","bex_workspace_creation_attempt":"wca-1"}}`
		case method == http.MethodGet && path == "/v1/payment_methods/pm_1":
			return 200, `{"id":"pm_1","object":"payment_method","customer":"cus_1","type":"card","card":{"funding":"prepaid","fingerprint":"fp_1"}}`
		case method == http.MethodPost && path == "/v1/payment_methods/pm_1/detach":
			return 200, `{"id":"pm_1","object":"payment_method","customer":null}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	_, err := c.VerifyWorkspaceSetup(context.Background(), "wca-1", "tea-new", "cus_1", "seti_1")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != CodePaymentMethodRejected || coded.Params["reason"] != string(store.PaymentMethodRejectedPrepaid) || !IsPaymentMethodRejected(err) ||
		!strings.Contains(coded.Error(), "Prepaid cards aren't accepted") {
		t.Fatalf("VerifyWorkspaceSetup = %v, want PAYMENT_METHOD_REJECTED prepaid_card", err)
	}
	if stub.count("/v1/payment_methods/pm_1/detach") != 1 {
		t.Fatal("the prepaid card was not detached")
	}
}

// Real DB: a card whose fingerprint is on an unpaid write-off, in a deleted
// workspace, is refused in another workspace and the reason is readable; a
// clean card then binds and clears it. Paying the written-off invoice frees
// the fingerprint.
func TestCardFingerprintDedupeAgainstPostgres(t *testing.T) {
	pg, _ := writeOffTestStore(t)
	ctx := context.Background()
	debtor := writeOffTestTenant(t, pg, "card-debtor")
	tenant := writeOffTestTenant(t, pg, "card-new")
	suffix := strings.ReplaceAll(tenant.ID, "-", "")
	fingerprint := "fp_debt_" + suffix
	writeOff := store.AgentModelWriteOff{InvoiceID: "in_card_" + suffix, WorkspaceID: debtor.ID, CustomerID: "cus_debtor_" + suffix,
		UsageMonth: time.Now(), AgentModelNetMicroUSD: 5_000_000, AmountMicroUSD: 5_000_000, WrittenOffAt: time.Now(), CardFingerprint: fingerprint}
	if _, err := pg.RecordAgentModelWriteOff(ctx, writeOff); err != nil {
		t.Fatal(err)
	}
	if err := pg.DeleteTenant(ctx, debtor.ID); err != nil {
		t.Fatal(err)
	}
	customer := "cus_new_" + suffix
	if err := pg.UpsertBillingProviderMapping(ctx, store.BillingProviderMapping{WorkspaceID: tenant.ID, CustomerID: customer}); err != nil {
		t.Fatal(err)
	}
	c, stub := newStripeTest(t, checkoutCardStub(tenant.ID, customer, map[string][2]string{
		"reused": {"credit", fingerprint}, "clean": {"credit", "fp_clean_" + suffix},
	}))
	c.state, c.cards = pg, pg

	if err := c.CompleteCheckoutSession(ctx, &stripe.CheckoutSession{ID: "cs_reused"}); err != nil {
		t.Fatal(err)
	}
	if bound, err := pg.PaymentMethodBound(ctx, tenant.ID); err != nil || bound {
		t.Fatalf("a card with unpaid debt bound: %v %v", bound, err)
	}
	if eligibility, err := pg.PaymentEligibility(ctx, tenant.ID); err != nil || eligibility.Rejection != store.PaymentMethodRejectedUnpaidBalance {
		t.Fatalf("rejection = %q, %v, want unpaid_balance", eligibility.Rejection, err)
	}
	if stub.count("/pm_reused/detach") != 1 {
		t.Fatal("the reused card was not detached")
	}
	if err := c.CompleteCheckoutSession(ctx, &stripe.CheckoutSession{ID: "cs_clean"}); err != nil {
		t.Fatal(err)
	}
	if bound, err := pg.PaymentMethodBound(ctx, tenant.ID); err != nil || !bound {
		t.Fatalf("the clean card did not bind: %v %v", bound, err)
	}
	if eligibility, err := pg.PaymentEligibility(ctx, tenant.ID); err != nil || eligibility.Rejection != "" {
		t.Fatalf("rejection after a bind = %q, %v, want cleared", eligibility.Rejection, err)
	}
	h := &Trust{Store: pg}
	if err := h.HandleStripeEvent(ctx, trustEvent("evt_card_paid_"+suffix, stripe.EventTypeInvoicePaid, time.Now(),
		trustInvoice(debtor.ID, writeOff.InvoiceID, 500, "subscription_threshold", time.Now()))); err != nil {
		t.Fatal(err)
	}
	if flagged, err := pg.UnpaidWriteOffForCardFingerprint(ctx, fingerprint); err != nil || flagged {
		t.Fatalf("after the debt is paid the card is still flagged: %v %v", flagged, err)
	}
}

type fingerprintInvoices struct{ trustInvoicesFake }

func (fingerprintInvoices) WrittenOffCardFingerprint(_ context.Context, invoiceID, customerID string) (string, error) {
	return "fp_of_" + invoiceID + "_" + customerID, nil
}

// A write-off keeps the fingerprint of the card that owes the invoice.
func TestWriteOffRecordsCardFingerprint(t *testing.T) {
	writeOffs := &writeOffsFake{}
	h := &Trust{Store: &trustStoreFake{}, WriteOffs: writeOffs,
		Invoices: fingerprintInvoices{trustInvoicesFake{"in_fp": {NetCents: 100, UsageMonth: time.Now()}}}}
	if err := h.HandleStripeEvent(context.Background(), trustEvent("evt_fp", stripe.EventTypeInvoiceMarkedUncollectible, time.Now(),
		uncollectibleInvoice("tea-a", "in_fp", 100, 100, time.Now()))); err != nil {
		t.Fatal(err)
	}
	if len(writeOffs.got) != 1 || writeOffs.got[0].CardFingerprint != "fp_of_in_fp_cus_x" {
		t.Fatalf("write-offs = %+v, want the owing card's fingerprint", writeOffs.got)
	}
}

// ADR091 D10.6: the written-off fingerprint is the card whose charge of the
// invoice failed, not whatever card is default now; only an invoice with no
// card charge falls back to the Customer's default card.
func TestWrittenOffCardFingerprintPrefersFailedCharge(t *testing.T) {
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/invoice_payments":
			return 200, `{"object":"list","data":[{"id":"inpay_1","object":"invoice_payment","status":"open","payment":{"type":"payment_intent","payment_intent":` +
				`{"id":"pi_1","object":"payment_intent","latest_charge":{"id":"ch_1","object":"charge","status":"failed","payment_method_details":{"type":"card","card":{"fingerprint":"fp_failed"}}}}}}],"has_more":false,"url":"/v1/invoice_payments"}`
		case method == http.MethodGet && path == "/v1/customers/cus_1":
			return 200, `{"id":"cus_1","object":"customer","invoice_settings":{"default_payment_method":{"id":"pm_new","object":"payment_method","card":{"fingerprint":"fp_new_default"}}}}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	got, err := c.WrittenOffCardFingerprint(context.Background(), "in_1", "cus_1")
	if err != nil || got != "fp_failed" {
		t.Fatalf("fingerprint = %q, %v, want the failed charge's card", got, err)
	}
	if stub.count("/v1/customers/cus_1") != 0 {
		t.Fatal("the default card was read although the failed charge named the card")
	}

	c, _ = newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/invoice_payments":
			return 200, `{"object":"list","data":[{"id":"inpay_1","object":"invoice_payment","status":"open","payment":{"type":"payment_intent","payment_intent":` +
				`{"id":"pi_1","object":"payment_intent","last_payment_error":{"type":"card_error","code":"card_declined","payment_method":{"id":"pm_old","object":"payment_method","card":{"fingerprint":"fp_error"}}}}}}],"has_more":false,"url":"/v1/invoice_payments"}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	if got, err := c.WrittenOffCardFingerprint(context.Background(), "in_1", "cus_1"); err != nil || got != "fp_error" {
		t.Fatalf("fingerprint from last_payment_error = %q, %v", got, err)
	}

	c, _ = newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/invoice_payments":
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/invoice_payments"}`
		case method == http.MethodGet && path == "/v1/customers/cus_1":
			return 200, `{"id":"cus_1","object":"customer","invoice_settings":{"default_payment_method":{"id":"pm_new","object":"payment_method","card":{"fingerprint":"fp_default"}}}}`
		default:
			return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
		}
	})
	if got, err := c.WrittenOffCardFingerprint(context.Background(), "in_1", "cus_1"); err != nil || got != "fp_default" {
		t.Fatalf("fallback fingerprint = %q, %v, want the default card", got, err)
	}
}
