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
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// chargeStoreFake is an in-memory AgentModelChargeStore with the PGStore's
// begin and settle rules.
type chargeStoreFake struct {
	mu          sync.Mutex
	charges     map[store.AgentModelChargeKey]store.AgentModelCharge
	billedUntil map[string]time.Time
	credits     map[string]int64
	locked      bool
}

func (f *chargeStoreFake) put(c store.AgentModelCharge) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.charges == nil {
		f.charges = map[store.AgentModelChargeKey]store.AgentModelCharge{}
	}
	f.charges[c.AgentModelChargeKey] = c
}

func (f *chargeStoreFake) get(key store.AgentModelChargeKey) store.AgentModelCharge {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.charges[key]
}

func (f *chargeStoreFake) BeginAgentModelCharge(_ context.Context, ws string, now time.Time, seenSeq int, micro, cents int64) (store.AgentModelCharge, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	month := store.MonthStartUTC(now)
	var latest store.AgentModelCharge
	for _, c := range f.charges {
		if c.WorkspaceID != ws {
			continue
		}
		if c.Status == store.AgentModelChargePending {
			return c, false, nil
		}
		if c.Month.Equal(month) && c.Seq > latest.Seq {
			latest = c
		}
	}
	if latest.Seq > seenSeq {
		return latest, false, nil
	}
	c := store.AgentModelCharge{
		AgentModelChargeKey: store.AgentModelChargeKey{WorkspaceID: ws, Month: month, Seq: latest.Seq + 1},
		AmountMicroUSD:      micro, AmountCents: cents, Status: store.AgentModelChargePending, CreatedAt: now,
	}
	if f.charges == nil {
		f.charges = map[store.AgentModelChargeKey]store.AgentModelCharge{}
	}
	f.charges[c.AgentModelChargeKey] = c
	return c, true, nil
}

func (f *chargeStoreFake) GetAgentModelCharge(_ context.Context, key store.AgentModelChargeKey) (store.AgentModelCharge, error) {
	if c := f.get(key); c.Seq != 0 {
		return c, nil
	}
	return store.AgentModelCharge{}, store.ErrNotFound
}

func (f *chargeStoreFake) update(key store.AgentModelChargeKey, fn func(*store.AgentModelCharge) bool) (store.AgentModelCharge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.charges[key]
	if !ok {
		return store.AgentModelCharge{}, store.ErrNotFound
	}
	if fn(&c) {
		f.charges[key] = c
	}
	return c, nil
}

func (f *chargeStoreFake) SetAgentModelChargeInvoice(_ context.Context, key store.AgentModelChargeKey, invoiceID string) error {
	_, err := f.update(key, func(c *store.AgentModelCharge) bool {
		c.InvoiceID = invoiceID
		return c.Status == store.AgentModelChargePending
	})
	return err
}

func (f *chargeStoreFake) CountAgentModelChargeAttempt(_ context.Context, key store.AgentModelChargeKey) error {
	_, err := f.update(key, func(c *store.AgentModelCharge) bool {
		c.Attempts++
		return c.Status == store.AgentModelChargePending
	})
	return err
}

func (f *chargeStoreFake) SettleAgentModelCharge(_ context.Context, key store.AgentModelChargeKey, s store.AgentModelChargeSettlement) (store.AgentModelCharge, error) {
	return f.update(key, func(c *store.AgentModelCharge) bool {
		if c.Status != store.AgentModelChargePending && (c.Status != store.AgentModelChargeFailed || s.Status != store.AgentModelChargePaid) {
			return false
		}
		c.Status, c.Failure, c.SettledAt = s.Status, s.Failure, s.At
		if s.HostedInvoiceURL != "" {
			c.HostedInvoiceURL = s.HostedInvoiceURL
		}
		return true
	})
}

func (f *chargeStoreFake) SetAgentModelChargeNetItem(_ context.Context, key store.AgentModelChargeKey, itemID string) error {
	_, err := f.update(key, func(c *store.AgentModelCharge) bool {
		c.NetItemID = itemID
		return c.Status == store.AgentModelChargePaid
	})
	return err
}

func (f *chargeStoreFake) AgentModelChargesToRetry(_ context.Context, cutoff time.Time, _ int) ([]store.AgentModelCharge, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.AgentModelCharge
	for _, c := range f.charges {
		if (c.Status == store.AgentModelChargePending && c.CreatedAt.Before(cutoff)) || (c.Status == store.AgentModelChargePaid && c.NetItemID == "") {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b store.AgentModelCharge) int { return a.Seq - b.Seq })
	return out, nil
}

func (f *chargeStoreFake) RecordAgentModelBilledUntil(_ context.Context, ws string, until time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.billedUntil == nil {
		f.billedUntil = map[string]time.Time{}
	}
	if !until.After(f.billedUntil[ws]) {
		return false, nil
	}
	f.billedUntil[ws] = until
	delete(f.credits, ws)
	return true, nil
}

func (f *chargeStoreFake) RecordAgentModelCredits(_ context.Context, ws string, micro int64, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.credits == nil {
		f.credits = map[string]int64{}
	}
	f.credits[ws] = micro
	return nil
}

func (f *chargeStoreFake) RunExclusive(ctx context.Context, _ string, fn func(context.Context) error) (bool, error) {
	if f.locked {
		return false, nil
	}
	return true, fn(ctx)
}

// chargeTrustFake records the trust writes a charge makes.
type chargeTrustFake struct {
	trustStoreFake
	mu       sync.Mutex
	payments []store.BillingTrustPayment
	failures []string
}

func (f *chargeTrustFake) RecordBillingTrustPayment(_ context.Context, p store.BillingTrustPayment) (store.BillingTrust, store.BillingTrust, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.payments = append(f.payments, p)
	return f.before, f.after, true, nil
}

func (f *chargeTrustFake) RecordBillingTrustFailure(_ context.Context, _, invoiceID string, _ time.Time) (store.BillingTrust, store.BillingTrust, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failures = append(f.failures, invoiceID)
	return f.before, f.after, true, nil
}

const (
	chargeTestSubscriptions = `{"object":"list","data":[{"id":"sub_old","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
	chargeTestOpenInvoice   = `{"id":"in_c1","object":"invoice","status":"open","customer":"cus_1","subtotal":1001,"amount_due":1001,"hosted_invoice_url":"https://pay.stripe.test/in_c1"}`
	chargeTestPaidInvoice   = `{"id":"in_c1","object":"invoice","status":"paid","customer":"cus_1","subtotal":1001,"amount_paid":1001,"hosted_invoice_url":"https://pay.stripe.test/in_c1"}`
)

// chargeStub is workspace tea-a (cus_1, live Subscription sub_old) whose
// charge invoice is in_c1; pay answers what payInvoice returns (200 paid by
// default), and a GET of in_c1 answers current.
type chargeStub struct {
	mu      sync.Mutex
	pay     func() (int, string)
	current string
	items   int
	// listed is the Subscription's latest invoices, as JSON objects;
	// creditCents the customer's available metered billing credit.
	listed      []string
	creditCents int64
}

func newChargeStub(t *testing.T) (*StripeClient, *stripeStub, *chargeStub) {
	t.Helper()
	cs := &chargeStub{pay: func() (int, string) { return 200, chargeTestPaidInvoice }, current: chargeTestOpenInvoice}
	c, stub := newStripeTest(t, newChargeStubRoute(cs))
	c.storeCustomer("tea-a", "cus_1")
	return c, stub, cs
}

// newChargeStubRoute is chargeStub's Stripe routes.
func newChargeStubRoute(cs *chargeStub) func(method, path string) (int, string) {
	return func(method, path string) (int, string) {
		cs.mu.Lock()
		defer cs.mu.Unlock()
		switch {
		case method == http.MethodGet && path == "/v1/subscriptions":
			return 200, chargeTestSubscriptions
		case method == http.MethodGet && path == "/v1/invoices":
			return 200, `{"object":"list","has_more":false,"data":[` + strings.Join(cs.listed, ",") + `]}`
		case method == http.MethodGet && path == "/v1/billing/credit_balance_summary":
			return 200, creditBalanceJSON(cs.creditCents)
		case method == http.MethodGet && path == "/v1/billing/credit_grants":
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/billing/credit_grants"}`
		case method == http.MethodPost && path == "/v1/invoices":
			return 200, `{"id":"in_c1","object":"invoice","status":"draft","customer":"cus_1","subtotal":0}`
		case method == http.MethodGet && path == "/v1/invoices/in_c1":
			return 200, cs.current
		case method == http.MethodPost && path == "/v1/invoiceitems":
			cs.items++
			return 200, fmt.Sprintf(`{"id":"ii_%d","object":"invoiceitem"}`, cs.items)
		case method == http.MethodPost && path == "/v1/invoices/in_c1/finalize":
			return 200, chargeTestOpenInvoice
		case method == http.MethodPost && path == "/v1/invoices/in_c1/pay":
			return cs.pay()
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	}
}

// creditBalanceJSON is a credit balance summary of cents available.
func creditBalanceJSON(cents int64) string {
	return fmt.Sprintf(`{"object":"billing.credit_balance_summary","livemode":false,"balances":[{"available_balance":{"type":"monetary","monetary":{"currency":"usd","value":%d}},"ledger_balance":{"type":"monetary","monetary":{"currency":"usd","value":%d}}}]}`, cents, cents)
}

func (cs *chargeStub) setPay(status int, body string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.pay = func() (int, string) { return status, body }
}

func form(t *testing.T, body string) url.Values {
	t.Helper()
	v, err := url.ParseQuery(body)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

var chargeTestNow = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

func newTestCharger(stripeClient AgentModelChargeProvider) (*AgentModelCharger, *chargeStoreFake, *chargeTrustFake) {
	st := &chargeStoreFake{}
	trust := &chargeTrustFake{}
	return &AgentModelCharger{Store: st, Stripe: stripeClient, Trust: &Trust{Store: trust}, Clock: func() time.Time { return chargeTestNow }}, st, trust
}

var chargeTestKey = store.AgentModelChargeKey{WorkspaceID: "tea-a", Month: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Seq: 1}

// ADR091 D10.2a/t012: a crossing is charged on a standalone invoice on the
// Customer (no subscription parameter, so neither anchor nor proration moves
// and hosting's dunning never sees it), with the amount as an item on that
// invoice, finalized without auto-advance and paid off-session at once; the
// paid charge is recorded in trust history and netted by a negative pending
// item on the Subscription. Every write carries a charge-derived idempotency
// key.
func TestAgentModelChargePaidStandaloneAndNetted(t *testing.T) {
	c, stub, _ := newChargeStub(t)
	charger, st, trust := newTestCharger(c)
	result, err := charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 10_005_000})
	if err != nil || result.Outcome != ChargePaid || result.InvoiceID != "in_c1" {
		t.Fatalf("result = %+v, %v; want paid in_c1", result, err)
	}

	// The charge window is read first (the Subscription's latest invoices
	// and the billing credits; with none, no grant is listed), then the charge
	// is made.
	wantPaths := []string{"/v1/subscriptions", "/v1/invoices", "/v1/billing/credit_balance_summary",
		"/v1/subscriptions", "/v1/invoices", "/v1/invoiceitems",
		"/v1/invoices/in_c1/finalize", "/v1/invoices/in_c1/pay", "/v1/subscriptions", "/v1/invoiceitems"}
	if !slices.Equal(stub.hits, wantPaths) {
		t.Fatalf("Stripe calls = %v, want %v", stub.hits, wantPaths)
	}
	invoices := stub.requests("/v1/invoices")
	if invoices[0].body != "" {
		t.Fatalf("the window read is not a list: %q", invoices[0].body)
	}
	create := form(t, invoices[1].body)
	if _, onSubscription := create["subscription"]; onSubscription {
		t.Fatalf("the charge invoice names a subscription: %v", create)
	}
	for k, want := range map[string]string{
		"customer": "cus_1", "collection_method": "charge_automatically", "auto_advance": "false",
		"pending_invoice_items_behavior": "exclude", "metadata[bex_workspace]": "tea-a",
		"metadata[bex_agent_model_charge]": "true", "metadata[bex_agent_model_charge_month]": "2026-11",
		"metadata[bex_agent_model_charge_seq]": "1",
	} {
		if got := create.Get(k); got != want {
			t.Errorf("invoice %s = %q, want %q", k, got, want)
		}
	}
	if got := invoices[1].header.Get("Idempotency-Key"); got != "bex-threshold-tea-a-2026-11-1-invoice-a0" {
		t.Errorf("invoice idempotency key = %q", got)
	}
	items := stub.requests("/v1/invoiceitems")
	item, net := form(t, items[0].body), form(t, items[1].body)
	if item.Get("invoice") != "in_c1" || item.Get("amount") != "1001" || item.Get("currency") != "usd" || item.Has("subscription") {
		t.Errorf("charge item = %v, want 1001¢ (10.005 USD rounded up) on in_c1, not the subscription", item)
	}
	if net.Get("subscription") != "sub_old" || net.Get("amount") != "-1001" || net.Has("invoice") || net.Get("customer") != "cus_1" ||
		net.Get("description") != "Agent model usage already charged on in_c1" || net.Get("metadata[bex_agent_model_charge_invoice]") != "in_c1" {
		t.Errorf("net item = %v, want -1001¢ pending on sub_old", net)
	}
	if got := items[1].header.Get("Idempotency-Key"); got != "bex-threshold-tea-a-2026-11-1-net" {
		t.Errorf("net idempotency key = %q", got)
	}
	if finalize := stub.requests("/v1/invoices/in_c1/finalize"); form(t, finalize[0].body).Get("auto_advance") != "false" {
		t.Errorf("finalize = %v, want auto_advance=false", finalize[0].body)
	}
	pay := stub.requests("/v1/invoices/in_c1/pay")
	if form(t, pay[0].body).Get("off_session") != "true" || pay[0].header.Get("Idempotency-Key") != "bex-threshold-tea-a-2026-11-1-pay-a0" {
		t.Errorf("pay = %v %q, want off_session with the pay key", pay[0].body, pay[0].header.Get("Idempotency-Key"))
	}

	got := st.get(chargeTestKey)
	if got.Status != store.AgentModelChargePaid || got.InvoiceID != "in_c1" || got.NetItemID != "ii_2" || got.AmountCents != 1001 ||
		got.HostedInvoiceURL != "https://pay.stripe.test/in_c1" || !got.SettledAt.Equal(chargeTestNow) {
		t.Fatalf("ledger row = %+v", got)
	}
	if len(trust.payments) != 1 || trust.payments[0] != (store.BillingTrustPayment{InvoiceID: "in_c1", WorkspaceID: "tea-a", AmountCents: 1001, PaidAt: chargeTestNow}) {
		t.Fatalf("trust payments = %+v, want in_c1's 1001¢ (not a cycle invoice)", trust.payments)
	}
}

// A declined card, a card that needs authentication, and a payment Stripe
// says needs the customer's action are all declined: the charge fails with
// its code and hosted page, the failure opens BILLING_BALANCE_DUE, and
// nothing is netted.
func TestAgentModelChargeDeclined(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		code   string
	}{
		"card declined":           {402, `{"error":{"type":"card_error","code":"card_declined","decline_code":"insufficient_funds","message":"declined"}}`, "card_declined"},
		"authentication required": {402, `{"error":{"type":"card_error","code":"authentication_required","message":"needs 3DS"}}`, "authentication_required"},
		"requires action":         {400, `{"error":{"type":"invalid_request_error","code":"invoice_payment_intent_requires_action","message":"needs action"}}`, "invoice_payment_intent_requires_action"},
	} {
		t.Run(name, func(t *testing.T) {
			c, stub, cs := newChargeStub(t)
			cs.setPay(tc.status, tc.body)
			charger, st, trust := newTestCharger(c)
			result, err := charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 10_000_000})
			if err != nil || result.Outcome != ChargeDeclined || result.HostedInvoiceURL != "https://pay.stripe.test/in_c1" {
				t.Fatalf("result = %+v, %v; want declined with the hosted page", result, err)
			}
			got := st.get(chargeTestKey)
			if got.Status != store.AgentModelChargeFailed || got.Failure != tc.code || got.HostedInvoiceURL != "https://pay.stripe.test/in_c1" {
				t.Fatalf("ledger row = %+v", got)
			}
			if !slices.Equal(trust.failures, []string{"in_c1"}) || len(trust.payments) != 0 {
				t.Fatalf("trust failures = %v payments = %v, want in_c1's failure only", trust.failures, trust.payments)
			}
			if n := len(stub.requests("/v1/invoiceitems")); n != 1 {
				t.Fatalf("invoice items = %d, want the charge item only (no net)", n)
			}
		})
	}
}

// Stripe unreachable (a 500 on pay) decides nothing: the charge stays
// pending with its invoice and one counted attempt. The sweep (once the
// charge is old enough) re-reads that invoice and pays it under the next
// attempt's key, records it, and nets it; a paid charge missing its net item
// gets one.
func TestAgentModelChargeUndecidedThenSwept(t *testing.T) {
	c, stub, cs := newChargeStub(t)
	cs.setPay(500, `{"error":{"type":"api_error","message":"down"}}`)
	charger, st, trust := newTestCharger(c)
	result, err := charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 10_000_000})
	if err != nil || result.Outcome != ChargePending {
		t.Fatalf("result = %+v, %v; want pending", result, err)
	}
	got := st.get(chargeTestKey)
	if got.Status != store.AgentModelChargePending || got.InvoiceID != "in_c1" || got.Attempts != 1 {
		t.Fatalf("ledger row = %+v, want pending in_c1 after one attempt", got)
	}
	if len(trust.payments)+len(trust.failures) != 0 {
		t.Fatalf("an undecided charge reached trust: %v %v", trust.payments, trust.failures)
	}
	// A second crossing while it is pending joins it, and does not wait on
	// a charge Stripe already could not decide.
	start := time.Now()
	if joined, err := charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 12_000_000}); err != nil ||
		joined.Outcome != ChargePending || time.Since(start) > time.Second {
		t.Fatalf("join = %+v, %v after %s", joined, err, time.Since(start))
	}
	if n := countPosts(stub.requests("/v1/invoices")); n != 1 {
		t.Fatalf("invoices created = %d, want one", n)
	}

	// The sweep leaves a fresh pending charge to the mint making it.
	if err := charger.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(stub.requests("/v1/invoices/in_c1/pay")); n != 1 {
		t.Fatalf("pays = %d: the sweep retried a fresh charge", n)
	}
	// A paid charge from earlier that never got its net item.
	st.put(store.AgentModelCharge{
		AgentModelChargeKey: store.AgentModelChargeKey{WorkspaceID: "tea-a", Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Seq: 3},
		AmountMicroUSD:      25_000_000, AmountCents: 2500, Status: store.AgentModelChargePaid, InvoiceID: "in_oct3",
		CreatedAt: chargeTestNow.AddDate(0, -1, 0), SettledAt: chargeTestNow.AddDate(0, -1, 0),
	})
	cs.setPay(200, chargeTestPaidInvoice)
	charger.Clock = func() time.Time { return chargeTestNow.Add(agentModelChargeRetryAfter + time.Second) }
	if err := charger.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	pays := stub.requests("/v1/invoices/in_c1/pay")
	if len(pays) != 2 || pays[1].header.Get("Idempotency-Key") != "bex-threshold-tea-a-2026-11-1-pay-a1" {
		t.Fatalf("pays = %d, second key %q; want the re-pay under attempt 1", len(pays), pays[len(pays)-1].header.Get("Idempotency-Key"))
	}
	if n := countPosts(stub.requests("/v1/invoices")); n != 1 {
		t.Fatalf("invoices created = %d: the sweep made another invoice instead of paying in_c1", n)
	}
	if got := st.get(chargeTestKey); got.Status != store.AgentModelChargePaid || got.NetItemID == "" {
		t.Fatalf("swept charge = %+v, want paid and netted", got)
	}
	if len(trust.payments) != 1 || trust.payments[0].InvoiceID != "in_c1" {
		t.Fatalf("trust payments = %+v", trust.payments)
	}
	october := st.get(store.AgentModelChargeKey{WorkspaceID: "tea-a", Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Seq: 3})
	if october.NetItemID == "" {
		t.Fatalf("the paid October charge was not netted: %+v", october)
	}
	var nets []url.Values
	for _, r := range stub.requests("/v1/invoiceitems") {
		if v := form(t, r.body); v.Has("subscription") {
			nets = append(nets, v)
		}
	}
	if len(nets) != 2 || !slices.ContainsFunc(nets, func(v url.Values) bool { return v.Get("amount") == "-2500" }) {
		t.Fatalf("net items = %v, want in_c1's and the October charge's", nets)
	}

	// Only the replica holding the sweep lock runs it.
	st.locked = true
	st.put(store.AgentModelCharge{AgentModelChargeKey: store.AgentModelChargeKey{WorkspaceID: "tea-b", Month: chargeTestKey.Month, Seq: 1},
		AmountMicroUSD: 1, AmountCents: 50, Status: store.AgentModelChargePaid, InvoiceID: "in_b"})
	before := len(stub.hits)
	if err := charger.RunOnce(context.Background()); err != nil || len(stub.hits) != before {
		t.Fatalf("a replica without the lock swept: %v, %d calls", err, len(stub.hits)-before)
	}
}

// A request that finds another request's fresh charge in flight waits for
// it and shares its outcome; Stripe is called by the first request only.
func TestAgentModelChargeJoinWaitsForTheChargeInFlight(t *testing.T) {
	for _, tc := range []struct {
		settle string
		want   ChargeOutcome
	}{{store.AgentModelChargePaid, ChargePaid}, {store.AgentModelChargeFailed, ChargeDeclined}} {
		t.Run(tc.settle, func(t *testing.T) {
			provider := &chargeProviderFake{}
			charger, st, _ := newTestCharger(provider)
			st.put(store.AgentModelCharge{AgentModelChargeKey: chargeTestKey, AmountMicroUSD: 10_000_000, AmountCents: 1000,
				Status: store.AgentModelChargePending, InvoiceID: "in_c1", CreatedAt: chargeTestNow})
			go func() {
				time.Sleep(50 * time.Millisecond)
				_, _ = st.SettleAgentModelCharge(context.Background(), chargeTestKey, store.AgentModelChargeSettlement{Status: tc.settle, At: chargeTestNow})
			}()
			result, err := charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 10_000_000})
			if err != nil || result.Outcome != tc.want || result.InvoiceID != "in_c1" {
				t.Fatalf("join = %+v, %v; want %v", result, err, tc.want)
			}
			if provider.charges != 0 {
				t.Fatalf("the joining request charged Stripe %d times", provider.charges)
			}
		})
	}
}

// chargeProviderFake counts charges; it is never meant to be reached.
// meter is what AgentModelMeter reads from Stripe.
type chargeProviderFake struct {
	charges int
	meter   AgentModelMeter
}

func (f *chargeProviderFake) AgentModelMeter(context.Context, string) (AgentModelMeter, error) {
	return f.meter, nil
}

func (f *chargeProviderFake) ChargeAgentModelUsage(context.Context, store.AgentModelCharge, func(context.Context, string) error) (AgentModelChargeResult, error) {
	f.charges++
	return AgentModelChargeResult{}, errors.New("unexpected charge")
}

func (f *chargeProviderFake) NetAgentModelCharge(context.Context, store.AgentModelCharge) (string, error) {
	return "", errors.New("unexpected net")
}

func TestAgentModelChargeCents(t *testing.T) {
	for micro, want := range map[int64]int64{10_000_000: 1000, 10_005_000: 1001, 10_000_001: 1001, 1: 50, 499_999: 50} {
		if got := agentModelChargeCents(micro); got != want {
			t.Errorf("agentModelChargeCents(%d) = %d, want %d", micro, got, want)
		}
	}
}

// chargeLedgerFake records the charge settlements and charge window
// boundaries the webhook makes.
type chargeLedgerFake struct {
	settled     []string
	billedUntil []string
}

func (f *chargeLedgerFake) RecordAgentModelBilledUntil(_ context.Context, ws string, until time.Time) (bool, error) {
	f.billedUntil = append(f.billedUntil, ws+"@"+until.UTC().Format(time.RFC3339))
	return true, nil
}

func (f *chargeLedgerFake) SettleAgentModelChargeInvoice(_ context.Context, invoiceID string, s store.AgentModelChargeSettlement) (store.AgentModelCharge, bool, error) {
	f.settled = append(f.settled, invoiceID+":"+s.Status+":"+s.Failure)
	return store.AgentModelCharge{}, true, nil
}

func chargeInvoiceJSON(workspaceID, invoiceID string, amountPaid int64, paidAt time.Time) string {
	return fmt.Sprintf(`{"id":%q,"object":"invoice","livemode":false,"customer":"cus_x","amount_paid":%d,"billing_reason":"manual",`+
		`"status_transitions":{"paid_at":%d},"hosted_invoice_url":"https://pay.stripe.test/%s",`+
		`"metadata":{"bex_workspace":%q,"bex_agent_model_charge":"true","bex_agent_model_charge_month":"2026-11","bex_agent_model_charge_seq":"1"}}`,
		invoiceID, amountPaid, paidAt.Unix(), invoiceID, workspaceID)
}

// A charge invoice has no subscription_details: Trust names its workspace by
// its own metadata. A declined charge paid later on its hosted page records
// the payment (settling its failure and so BILLING_BALANCE_DUE) and turns the
// ledger row paid; a failure Stripe decided after the mint stopped waiting
// opens the balance due and fails the row; a void settles both. An invoice
// that is neither a contract nor a charge is ignored.
func TestTrustRecordsAgentModelChargeInvoicesByMetadata(t *testing.T) {
	now := chargeTestNow
	trustStore := &chargeTrustFake{}
	ledger := &chargeLedgerFake{}
	h := &Trust{Store: trustStore, ModelCharges: ledger, Clock: func() time.Time { return now }}
	send := func(id string, eventType stripe.EventType, object string) {
		t.Helper()
		if err := h.HandleStripeEvent(context.Background(), trustEvent(id, eventType, now, object)); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	send("evt_paid", stripe.EventTypeInvoicePaid, chargeInvoiceJSON("tea-a", "in_c1", 1000, now))
	if len(trustStore.payments) != 1 || trustStore.payments[0] != (store.BillingTrustPayment{InvoiceID: "in_c1", WorkspaceID: "tea-a", AmountCents: 1000, PaidAt: now}) {
		t.Fatalf("payments = %+v, want in_c1's 1000¢ for tea-a (not a cycle invoice)", trustStore.payments)
	}
	send("evt_failed", stripe.EventTypeInvoicePaymentFailed, chargeInvoiceJSON("tea-a", "in_c2", 0, now))
	if !slices.Equal(trustStore.failures, []string{"in_c2"}) {
		t.Fatalf("failures = %v, want in_c2", trustStore.failures)
	}
	send("evt_void", stripe.EventTypeInvoiceVoided, chargeInvoiceJSON("tea-a", "in_c3", 0, now))
	if !slices.Equal(trustStore.settled, []string{"in_c3"}) {
		t.Fatalf("settled = %v, want the voided in_c3 closed for the balance-due refusal", trustStore.settled)
	}
	if want := []string{"in_c1:paid:", "in_c2:failed:payment_failed", "in_c3:failed:invoice_void"}; !slices.Equal(ledger.settled, want) {
		t.Fatalf("ledger settlements = %v, want %v", ledger.settled, want)
	}

	// Not a bex invoice: no workspace on it, or the flag without one.
	send("evt_other", stripe.EventTypeInvoicePaid, `{"id":"in_x","object":"invoice","amount_paid":500,"metadata":{"bex_agent_model_charge":"true"}}`)
	if len(trustStore.payments) != 1 || len(ledger.settled) != 3 {
		t.Fatal("an invoice with no workspace was recorded")
	}
	// The backfill counts a paid charge invoice too.
	var invoice stripe.Invoice
	if err := invoice.UnmarshalJSON([]byte(chargeInvoiceJSON("tea-a", "in_c4", 2500, now))); err != nil {
		t.Fatal(err)
	}
	if p, ok := paidInvoicePayment(&invoice); !ok || p.InvoiceID != "in_c4" || p.WorkspaceID != "tea-a" || p.AmountCents != 2500 || p.Cycle {
		t.Fatalf("backfill payment = %+v ok=%t", p, ok)
	}
}

// t013: an agent-model charge invoice never drives hosting's dunning, even
// one that also names a bex contract: its payment_failed (or action
// required) is not a contract event and is acknowledged without a record.
func TestLifecycleIgnoresAgentModelChargeInvoices(t *testing.T) {
	capture := &lifecycleCapture{}
	h := &Lifecycle{Store: capture, GracePeriod: 7 * 24 * time.Hour}
	for _, object := range []string{
		chargeInvoiceJSON("tea-a", "in_c1", 0, chargeTestNow),
		`{"id":"in_c2","object":"invoice","livemode":false,"customer":"cus_1","metadata":{"bex_workspace":"tea-a","bex_agent_model_charge":"true"},` +
			`"parent":{"type":"subscription_details","subscription_details":{"subscription":"sub_1","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}}}`,
	} {
		for _, eventType := range []stripe.EventType{stripe.EventTypeInvoicePaymentFailed, stripe.EventTypeInvoicePaymentActionRequired, stripe.EventTypeInvoicePaid} {
			event := lifecycleEvent("evt_"+string(eventType), eventType, object)
			if _, err := normalizeStripeLifecycleEvent(event, chargeTestNow); !errors.Is(err, errNotBexBillingContract) {
				t.Fatalf("%s: normalize = %v, want errNotBexBillingContract", eventType, err)
			}
			if err := h.HandleStripeEvent(context.Background(), event); err != nil {
				t.Fatalf("%s: %v, want acknowledged", eventType, err)
			}
		}
	}
	if len(capture.events) != 0 {
		t.Fatalf("hosting dunning recorded %d charge-invoice events", len(capture.events))
	}
	// A contract invoice still drives it.
	contract := `{"id":"in_h","object":"invoice","livemode":false,"customer":"cus_1","parent":{"type":"subscription_details","subscription_details":{"subscription":"sub_1","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}}}`
	if err := h.HandleStripeEvent(context.Background(), lifecycleEvent("evt_h", stripe.EventTypeInvoicePaymentFailed, contract)); err != nil || len(capture.events) != 1 ||
		!strings.Contains(capture.events[0].Reason, "payment_failed") {
		t.Fatalf("contract invoice failure: %v, events %+v", err, capture.events)
	}
}

// pgChargeStub is a Stripe stub for one workspace's charges: each created
// invoice gets its own id, pay answers payFor(invoice id) and settles it.
type pgChargeStub struct {
	mu       sync.Mutex
	prefix   string
	created  int
	items    int
	status   map[string]string
	declines map[int]bool // the n-th invoice's pay is declined
}

func (s *pgChargeStub) invoiceJSON(id string) string {
	paid := 0
	if s.status[id] == "paid" {
		paid = 1000
	}
	return fmt.Sprintf(`{"id":%q,"object":"invoice","status":%q,"customer":"cus_pg","subtotal":1000,"amount_paid":%d,"hosted_invoice_url":"https://pay.stripe.test/%s"}`,
		id, s.status[id], paid, id)
}

func (s *pgChargeStub) route(method, path string) (int, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case method == http.MethodGet && path == "/v1/subscriptions":
		return 200, `{"object":"list","data":[{"id":"sub_pg","object":"subscription","status":"active","metadata":{"bex_workspace":"` + s.prefix + `","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
	case method == http.MethodPost && path == "/v1/invoices":
		s.created++
		id := fmt.Sprintf("in_%s_%d", strings.ReplaceAll(s.prefix, "-", ""), s.created)
		s.status[id] = "draft"
		return 200, fmt.Sprintf(`{"id":%q,"object":"invoice","status":"draft","customer":"cus_pg","subtotal":0}`, id)
	case method == http.MethodPost && path == "/v1/invoiceitems":
		s.items++
		return 200, fmt.Sprintf(`{"id":"ii_pg_%d","object":"invoiceitem"}`, s.items)
	case method == http.MethodPost && strings.HasSuffix(path, "/finalize"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/invoices/"), "/finalize")
		s.status[id] = "open"
		return 200, s.invoiceJSON(id)
	case method == http.MethodPost && strings.HasSuffix(path, "/pay"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/invoices/"), "/pay")
		var n int
		_, _ = fmt.Sscanf(id[strings.LastIndex(id, "_")+1:], "%d", &n)
		if s.declines[n] {
			return 402, `{"error":{"type":"card_error","code":"card_declined","message":"declined"}}`
		}
		s.status[id] = "paid"
		return 200, s.invoiceJSON(id)
	}
	return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
}

// ADR091 D10.2a acceptance (real DB): two replicas' mints racing one crossing
// make exactly one paid invoice, and every racer is served; the payment lifts
// the workspace to T1 at once and its invoice.paid webhook adds nothing. A
// later declined charge opens BILLING_BALANCE_DUE; paying it on the hosted
// page (invoice.paid) lifts it, turns the ledger row paid, and the sweep nets
// it.
func TestAgentModelChargeAgainstPostgres(t *testing.T) {
	pg, _ := writeOffTestStore(t)
	ctx := context.Background()
	tenant := writeOffTestTenant(t, pg, "model-charge")
	ws := tenant.ID
	suffix := strings.ReplaceAll(ws, "-", "")
	if err := pg.UpsertBillingProviderMapping(ctx, store.BillingProviderMapping{WorkspaceID: ws, CustomerID: "cus_pg_" + suffix, SubscriptionID: "sub_pg_" + suffix}); err != nil {
		t.Fatal(err)
	}
	stubState := &pgChargeStub{prefix: ws, status: map[string]string{}, declines: map[int]bool{2: true}}
	c, stub := newStripeTest(t, stubState.route)
	c.storeCustomer(ws, "cus_pg")
	now := time.Now().UTC()
	trust := &Trust{Store: pg, ModelCharges: pg, Clock: func() time.Time { return now }}
	replicas := []*AgentModelCharger{
		{Store: pg, Stripe: c, Trust: trust, Clock: func() time.Time { return now }},
		{Store: pg, Stripe: c, Trust: trust, Clock: func() time.Time { return now }},
	}

	var wg sync.WaitGroup
	outcomes := make([]ChargeOutcome, 6)
	for i := range outcomes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := replicas[i%2].ChargeThreshold(ctx, store.AgentModelCrossing{WorkspaceID: ws, At: now, UnbilledMicroUSD: 10_000_000})
			if err != nil {
				t.Error(err)
			}
			outcomes[i] = result.Outcome
		}()
	}
	wg.Wait()
	for i, o := range outcomes {
		if o != ChargePaid {
			t.Fatalf("racer %d = %v, want every racer served by the one paid charge", i, o)
		}
	}
	if n := countPosts(stub.requests("/v1/invoices")); n != 1 {
		t.Fatalf("invoices created = %d, want exactly one for the crossing", n)
	}
	in, err := pg.AgentSpendInputs(ctx, ws, now)
	if err != nil || in.ChargedMicroUSD != 10_000_000 || in.ChargeSeq != 1 || in.ChargePending || !in.Chargeable || in.Trust.LifetimePaidCents != 1000 {
		t.Fatalf("inputs = %+v, %v; want $10 charged and $10 lifetime paid (T1)", in, err)
	}
	first := "in_" + suffix + "_1"
	if err := trust.HandleStripeEvent(ctx, trustEvent("evt_"+first, stripe.EventTypeInvoicePaid, now, chargeInvoiceJSON(ws, first, 1000, now))); err != nil {
		t.Fatal(err)
	}
	charges, err := pg.ListAgentModelCharges(ctx, ws, now.Add(-time.Hour))
	if err != nil || len(charges) != 1 || charges[0].Status != store.AgentModelChargePaid || charges[0].NetItemID == "" {
		t.Fatalf("charges = %+v, %v; want one paid, netted charge", charges, err)
	}
	if bt, _ := pg.GetBillingTrust(ctx, ws); bt.LifetimePaidCents != 1000 {
		t.Fatalf("lifetime paid = %d after the webhook, want 1000 (counted once)", bt.LifetimePaidCents)
	}

	// The next crossing is declined: balance due, nothing netted.
	result, err := replicas[0].ChargeThreshold(ctx, store.AgentModelCrossing{WorkspaceID: ws, At: now, SeenSeq: 1, UnbilledMicroUSD: 25_000_000})
	if err != nil || result.Outcome != ChargeDeclined || result.HostedInvoiceURL == "" {
		t.Fatalf("second charge = %+v, %v; want declined with its hosted page", result, err)
	}
	if in, err = pg.AgentSpendInputs(ctx, ws, now); err != nil || !in.BalanceDue || in.ChargedMicroUSD != 10_000_000 || in.ChargeSeq != 2 {
		t.Fatalf("inputs after the decline = %+v, %v", in, err)
	}
	// Paid later on the hosted page.
	second := "in_" + suffix + "_2"
	if err := trust.HandleStripeEvent(ctx, trustEvent("evt_"+second, stripe.EventTypeInvoicePaid, now.Add(time.Minute), chargeInvoiceJSON(ws, second, 2500, now.Add(time.Minute)))); err != nil {
		t.Fatal(err)
	}
	if in, err = pg.AgentSpendInputs(ctx, ws, now); err != nil || in.BalanceDue || in.ChargedMicroUSD != 35_000_000 {
		t.Fatalf("inputs after the hosted payment = %+v, %v; want no balance due and $35 charged", in, err)
	}
	if err := replicas[1].Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if charges, err = pg.ListAgentModelCharges(ctx, ws, now.Add(-time.Hour)); err != nil || len(charges) != 2 || charges[1].NetItemID == "" {
		t.Fatalf("charges = %+v, %v; want the late-paid charge netted by the sweep", charges, err)
	}
}

// ADR091 D10.2a: a charge collects only usage no subscription invoice has
// billed. Before it charges, the charger reads where the
// Subscription last billed the meter; when that moved past the caller's spend
// read (the webhook has not arrived), it charges nothing and answers not due,
// so the caller re-reads the narrower window. The next attempt, with the
// boundary already recorded, charges.
func TestAgentModelChargeNotDueWhenAnInvoiceBilledTheMeter(t *testing.T) {
	c, stub, cs := newChargeStub(t)
	boundary := time.Date(2026, 11, 13, 8, 39, 52, 0, time.UTC)
	contract := `"parent":{"type":"subscription_details","subscription_details":{"subscription":"sub_old","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}}`
	cs.listed = []string{
		fmt.Sprintf(`{"id":"in_thr","object":"invoice","status":"paid","billing_reason":"subscription_threshold","period_end":%d,%s}`, boundary.Unix(), contract),
		`{"id":"in_man","object":"invoice","status":"paid","billing_reason":"manual","period_end":1894000000,` + contract + `}`,
	}
	charger, st, _ := newTestCharger(c)
	result, err := charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 10_495_014})
	if err != nil || result.Outcome != ChargeNotDue {
		t.Fatalf("result = %+v, %v; want not due", result, err)
	}
	if n := countPosts(stub.requests("/v1/invoices")); n != 0 {
		t.Fatalf("invoices created = %d, want none", n)
	}
	if got := st.billedUntil["tea-a"]; !got.Equal(boundary) {
		t.Fatalf("billed until = %s, want the threshold invoice's period end %s (a manual invoice bills no meter)", got, boundary)
	}
	if result, err = charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 10_000_000}); err != nil || result.Outcome != ChargePaid {
		t.Fatalf("second attempt = %+v, %v; want paid", result, err)
	}
}

// ADR091 D10.2a: billing credits pay before the card. A standalone charge
// invoice cannot spend them, so the charger reads the balance first: when it
// differs from what the caller's read counted, it records it and charges
// nothing (the caller re-reads and finds the credit covers the spend); when
// it matches, the charge collects only what the credit left. A failed
// credit read keeps the recorded credit and the charge goes ahead.
func TestAgentModelChargeCreditsPayFirst(t *testing.T) {
	c, stub, cs := newChargeStub(t)
	cs.creditCents = 2000
	charger, st, _ := newTestCharger(c)
	result, err := charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 10_000_000})
	if err != nil || result.Outcome != ChargeNotDue {
		t.Fatalf("first crossing = %+v, %v; want not due (the credit was not counted)", result, err)
	}
	if n := countPosts(stub.requests("/v1/invoices")); n != 0 || st.credits["tea-a"] != 20_000_000 {
		t.Fatalf("invoices created = %d, credit recorded = %d; want none and $20", n, st.credits["tea-a"])
	}

	result, err = charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 5_000_000, CreditMicroUSD: 20_000_000})
	if err != nil || result.Outcome != ChargePaid {
		t.Fatalf("crossing past the credit = %+v, %v; want paid", result, err)
	}
	if got := st.get(chargeTestKey); got.AmountCents != 500 {
		t.Fatalf("charge = %+v, want the $5 the credit left", got)
	}

	failing, failStub := newStripeTest(t, func(method, path string) (int, string) {
		if path == "/v1/billing/credit_balance_summary" {
			return 403, `{"error":{"type":"invalid_request_error","message":"restricted key"}}`
		}
		return newChargeStubRoute(cs)(method, path)
	})
	failing.storeCustomer("tea-a", "cus_1")
	charger, st, _ = newTestCharger(failing)
	st.credits = map[string]int64{"tea-a": 7_000_000}
	result, err = charger.ChargeThreshold(context.Background(), store.AgentModelCrossing{WorkspaceID: "tea-a", At: chargeTestNow, UnbilledMicroUSD: 10_000_000, CreditMicroUSD: 7_000_000})
	if err != nil || result.Outcome != ChargePaid || countPosts(failStub.requests("/v1/invoices")) != 1 || st.credits["tea-a"] != 7_000_000 {
		t.Fatalf("failed credit read = %+v, %v, credit %d; want the charge made and the recorded credit kept", result, err, st.credits["tea-a"])
	}
}

// The credit the next invoice applies is the available metered balance less
// what grants expiring by the period's end hold (Stripe applies a grant only
// to an invoice whose period ends before it expires); a grant that outlives
// the period, a never-expiring one and a voided one are not read. Before
// any invoice has billed the meter, the window opens at the Subscription's
// start.
func TestAgentModelMeterCredit(t *testing.T) {
	start, periodEnd := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)
	grant := func(id string, expires time.Time, voided bool) string {
		exp, void := "null", "null"
		if !expires.IsZero() {
			exp = fmt.Sprint(expires.Unix())
		}
		if voided {
			void = fmt.Sprint(start.Unix())
		}
		return fmt.Sprintf(`{"id":%q,"object":"billing.credit_grant","effective_at":%d,"expires_at":%s,"voided_at":%s,`+
			`"applicability_config":{"scope":{"price_type":"metered"}}}`, id, start.Unix(), exp, void)
	}
	var stub *stripeStub
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch path {
		case "/v1/subscriptions":
			return 200, fmt.Sprintf(`{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active","start_date":%d,`+
				`"items":{"object":"list","data":[{"id":"si_1","object":"subscription_item","current_period_end":%d}]},`+
				`"metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`, start.Unix(), periodEnd.Unix())
		case "/v1/invoices":
			return 200, `{"object":"list","has_more":false,"data":[]}`
		case "/v1/billing/credit_grants":
			return 200, `{"object":"list","has_more":false,"url":"/v1/billing/credit_grants","data":[` + strings.Join([]string{
				grant("cg_soon", periodEnd.Add(-time.Hour), false), grant("cg_later", periodEnd.Add(time.Hour), false),
				grant("cg_forever", time.Time{}, false), grant("cg_void", periodEnd.Add(-time.Hour), true),
			}, ",") + `]}`
		case "/v1/billing/credit_balance_summary":
			queries := stub.queriesFor(path)
			q, _ := url.ParseQuery(queries[len(queries)-1])
			switch q.Get("filter[credit_grant]") {
			case "":
				return 200, creditBalanceJSON(5000)
			case "cg_soon":
				return 200, creditBalanceJSON(1500)
			}
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	c.storeCustomer("tea-a", "cus_1")
	meter, err := c.AgentModelMeter(context.Background(), "tea-a")
	if err != nil || !meter.CreditsRead || meter.CreditMicroUSD != 35_000_000 || !meter.BilledUntil.Equal(start) {
		t.Fatalf("meter = %+v, %v; want $35 of credit ($50 less cg_soon's $15) and the window from the Subscription's start", meter, err)
	}
	if got := stub.queriesFor("/v1/billing/credit_balance_summary"); len(got) != 2 {
		t.Fatalf("balance reads = %v, want the aggregate and cg_soon's only", got)
	}
}

// Only a subscription invoice that bills the meter moves the window, to its
// period end; a charge invoice never does.
func TestMeterBilledUntil(t *testing.T) {
	end := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	contract := &stripe.InvoiceParent{SubscriptionDetails: &stripe.InvoiceParentSubscriptionDetails{
		Subscription: &stripe.Subscription{ID: "sub_x"},
		Metadata:     map[string]string{subscriptionMetadataKey: "true", workspaceMetadataKey: "tea-a"},
	}}
	for reason, want := range map[stripe.InvoiceBillingReason]time.Time{
		stripe.InvoiceBillingReasonSubscriptionCycle:     end,
		stripe.InvoiceBillingReasonSubscriptionThreshold: end,
		stripe.InvoiceBillingReasonSubscriptionUpdate:    end,
		stripe.InvoiceBillingReasonSubscriptionCreate:    {},
		stripe.InvoiceBillingReasonManual:                {},
	} {
		if got := meterBilledUntil(&stripe.Invoice{BillingReason: reason, PeriodEnd: end.Unix(), Parent: contract}); !got.Equal(want) {
			t.Errorf("%s: %s, want %s", reason, got, want)
		}
	}
	charge := &stripe.Invoice{BillingReason: stripe.InvoiceBillingReasonSubscriptionCycle, PeriodEnd: end.Unix(),
		Metadata: map[string]string{agentModelChargeMetadataKey: "true", workspaceMetadataKey: "tea-a"}}
	if got := meterBilledUntil(charge); !got.IsZero() {
		t.Errorf("a charge invoice moved the window to %s", got)
	}
}

// Trust moves the window when a cycle or threshold invoice is created and
// again when it is paid; a charge invoice's events never move it.
func TestTrustRecordsWhereTheMeterWasBilled(t *testing.T) {
	now := chargeTestNow
	ledger := &chargeLedgerFake{}
	h := &Trust{Store: &chargeTrustFake{}, ModelCharges: ledger, Clock: func() time.Time { return now }}
	end := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	cycle := strings.Replace(trustInvoice("tea-a", "in_cy", 0, "subscription_cycle", now), `"billing_reason"`,
		fmt.Sprintf(`"period_end":%d,"billing_reason"`, end.Unix()), 1)
	for i, eventType := range []stripe.EventType{stripe.EventTypeInvoiceCreated, stripe.EventTypeInvoicePaid} {
		if err := h.HandleStripeEvent(context.Background(), trustEvent(fmt.Sprintf("evt_cy%d", i), eventType, now, cycle)); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.HandleStripeEvent(context.Background(), trustEvent("evt_ch", stripe.EventTypeInvoicePaid, now, chargeInvoiceJSON("tea-a", "in_c1", 1000, now))); err != nil {
		t.Fatal(err)
	}
	want := []string{"tea-a@2026-11-16T00:00:00Z", "tea-a@2026-11-16T00:00:00Z"}
	if !slices.Equal(ledger.billedUntil, want) {
		t.Fatalf("billed until = %v, want %v", ledger.billedUntil, want)
	}
}

// A declined charge whose usage a later subscription invoice bills is voided:
// only the workspace's open charge invoices created before that invoice's
// period end, never a subscription invoice or a newer charge.
func TestVoidAgentModelChargesBilledBy(t *testing.T) {
	until := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	var voided []string
	c, _ := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/invoices":
			charge := `{"id":%q,"object":"invoice","status":"open","created":%d,"metadata":{"bex_workspace":"tea-a","bex_agent_model_charge":"true"}}`
			return 200, `{"object":"list","has_more":false,"data":[` +
				fmt.Sprintf(charge, "in_old", until.Add(-48*time.Hour).Unix()) + `,` +
				fmt.Sprintf(charge, "in_new", until.Add(time.Hour).Unix()) + `,` +
				fmt.Sprintf(`{"id":"in_cycle","object":"invoice","status":"open","created":%d}`, until.Add(-time.Hour).Unix()) + `]}`
		case method == http.MethodPost && strings.HasSuffix(path, "/void"):
			voided = append(voided, path)
			return 200, `{"id":"in_old","object":"invoice","status":"void"}`
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	if err := c.VoidAgentModelChargesBilledBy(context.Background(), "tea-a", "cus_1", until); err != nil || !slices.Equal(voided, []string{"/v1/invoices/in_old/void"}) {
		t.Fatalf("voided %v, %v; want only in_old", voided, err)
	}
}

// Trust voids the declined charges a cycle invoice bills again, from its
// created and paid events; a charge invoice's own events void nothing.
func TestTrustVoidsDeclinedChargesTheCycleInvoiceBills(t *testing.T) {
	now := chargeTestNow
	invoices := &voidRecorder{}
	h := &Trust{Store: &chargeTrustFake{}, ModelCharges: &chargeLedgerFake{}, Invoices: invoices, Clock: func() time.Time { return now }}
	end := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	cycle := strings.Replace(trustInvoice("tea-a", "in_cy", 0, "subscription_cycle", now), `"billing_reason"`,
		fmt.Sprintf(`"period_end":%d,"billing_reason"`, end.Unix()), 1)
	for i, eventType := range []stripe.EventType{stripe.EventTypeInvoiceCreated, stripe.EventTypeInvoicePaid} {
		if err := h.HandleStripeEvent(context.Background(), trustEvent(fmt.Sprintf("evt_v%d", i), eventType, now, cycle)); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.HandleStripeEvent(context.Background(), trustEvent("evt_vc", stripe.EventTypeInvoicePaid, now, chargeInvoiceJSON("tea-a", "in_c1", 1000, now))); err != nil {
		t.Fatal(err)
	}
	want := []string{"tea-a/cus_x@2026-11-16T00:00:00Z", "tea-a/cus_x@2026-11-16T00:00:00Z"}
	if !slices.Equal(invoices.voids, want) {
		t.Fatalf("voids = %v, want %v", invoices.voids, want)
	}
}

// voidRecorder is a TrustInvoices that records the void requests.
type voidRecorder struct {
	trustInvoicesFake
	voids []string
}

func (v *voidRecorder) VoidAgentModelChargesBilledBy(_ context.Context, ws, customerID string, until time.Time) error {
	v.voids = append(v.voids, ws+"/"+customerID+"@"+until.UTC().Format(time.RFC3339))
	return nil
}

// creditRecordingState is a BillingStateStore that records the agent-model
// credit a Billing read saw.
type creditRecordingState struct {
	billingStateStoreFake
	credits map[string]int64
}

func (f *creditRecordingState) RecordAgentModelCredits(_ context.Context, ws string, micro int64, _ time.Time) error {
	if f.credits == nil {
		f.credits = map[string]int64{}
	}
	f.credits[ws] = micro
	return nil
}

// w6/m400: a Billing read records the credit the charger would read (the
// metered balance less grants expiring by the period end), so the mint and the
// agent environment count it before the first crossing.
func TestBillingReadRecordsTheAgentModelCredit(t *testing.T) {
	start, periodEnd := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)
	var stub *stripeStub
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch path {
		case "/v1/subscriptions":
			return 200, fmt.Sprintf(`{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active",`+
				`"items":{"object":"list","data":[{"id":"si_1","object":"subscription_item","current_period_end":%d}]},`+
				`"metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`, periodEnd.Unix())
		case "/v1/invoices/create_preview":
			return 200, `{"id":"upcoming_in_1","object":"invoice","currency":"usd","subtotal":1234,"total":1234,"amount_due":1234,"status":"draft"}`
		case "/v1/invoices":
			return 200, `{"object":"list","data":[],"has_more":false,"url":"/v1/invoices"}`
		case "/v1/billing/credit_grants":
			return 200, fmt.Sprintf(`{"object":"list","has_more":false,"url":"/v1/billing/credit_grants","data":[`+
				`{"id":"cg_soon","object":"billing.credit_grant","effective_at":%d,"expires_at":%d,"applicability_config":{"scope":{"price_type":"metered"}}}]}`,
				start.Unix(), periodEnd.Add(-time.Hour).Unix())
		case "/v1/billing/credit_balance_summary":
			queries := stub.queriesFor(path)
			q, _ := url.ParseQuery(queries[len(queries)-1])
			if q.Get("filter[credit_grant]") == "cg_soon" {
				return 200, creditBalanceJSON(1000)
			}
			return 200, creditBalanceJSON(2500)
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	state := &creditRecordingState{}
	c.state = state
	c.storeCustomer("tea-a", "cus_1")
	_ = state.UpsertBillingProviderMapping(context.Background(), store.BillingProviderMapping{WorkspaceID: "tea-a", CustomerID: "cus_1"})
	b, err := c.BillingFor(context.Background(), "tea-a", start, start.Add(24*time.Hour))
	if err != nil || b == nil || b.Credits == nil {
		t.Fatalf("billing = %+v, %v; want the credits block", b, err)
	}
	if got := state.credits["tea-a"]; got != 15_000_000 {
		t.Fatalf("recorded credit = %d, want $15 ($25 less cg_soon's $10, which expires before the period ends)", got)
	}
}
