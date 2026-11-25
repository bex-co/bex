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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// overdueStripe is a stateful Stripe stand-in for paying overdue invoices at
// binding (w6/m626). It completes setup Checkouts (session cs_<card> on
// payment method pm_<card>), lists the Subscription's invoices by status, and
// pays them, declining any card whose fingerprint is in declines.
type overdueStripe struct {
	ws, customer, sub string

	mu sync.Mutex
	// invoices in creation order; fingerprints maps a card name to its
	// fingerprint (cards entered twice share one).
	invoices     []*overdueInvoiceFixture
	fingerprints map[string]string
	declines     map[string]string
	// subDefault is the Subscription's own default payment method,
	// customerDefault the Customer's.
	subDefault, customerDefault string
	pays                        []overduePay
	subUpdates                  []string
	invoiceLists                int
}

type overdueInvoiceFixture struct {
	id        string
	created   int64
	status    string
	remaining int64
}

type overduePay struct {
	invoice, paymentMethod, offSession, key string
}

func newOverdueStripe(ws string) *overdueStripe {
	suffix := strings.ReplaceAll(ws, "-", "")
	return &overdueStripe{
		ws: ws, customer: "cus_od" + suffix, sub: "sub_od" + suffix,
		fingerprints: map[string]string{}, declines: map[string]string{},
	}
}

func (s *overdueStripe) client(t *testing.T) *StripeClient {
	t.Helper()
	c := NewStripe(StripeConfig{
		SecretKey: "sk_test_x", HTTPClient: &http.Client{Transport: s},
		BaseURL: "https://stub.stripe.test", MaxNetworkRetries: stripe.Int64(0),
	})
	if c == nil {
		t.Fatal("NewStripe returned nil")
	}
	return c
}

func (s *overdueStripe) addInvoice(id string, created, remaining int64, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invoices = append(s.invoices, &overdueInvoiceFixture{id: id, created: created, status: status, remaining: remaining})
}

func (s *overdueStripe) invoiceJSON(inv *overdueInvoiceFixture) string {
	return fmt.Sprintf(`{"id":%q,"object":"invoice","livemode":false,"customer":%q,"status":%q,"created":%d,"amount_due":%d,"amount_remaining":%d,"currency":"usd",`+
		`"hosted_invoice_url":"https://invoice.stripe.test/%s","status_transitions":{},`+
		`"parent":{"type":"subscription_details","subscription_details":{"subscription":%q,"metadata":{"bex_workspace":%q,"bex_billing_contract":"true"}}}}`,
		inv.id, s.customer, inv.status, inv.created, inv.remaining, inv.remaining, inv.id, s.sub, s.ws)
}

func (s *overdueStripe) paysOf(invoiceID string) []overduePay {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []overduePay
	for _, p := range s.pays {
		if p.invoice == invoiceID {
			out = append(out, p)
		}
	}
	return out
}

func (s *overdueStripe) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}
	form, _ := url.ParseQuery(string(body))
	s.mu.Lock()
	status, out := s.route(req, form)
	s.mu.Unlock()
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: h, Body: io.NopCloser(strings.NewReader(out)), Request: req}, nil
}

func (s *overdueStripe) route(req *http.Request, form url.Values) (int, string) {
	path, get := req.URL.Path, req.Method == http.MethodGet
	switch {
	case get && strings.HasPrefix(path, "/v1/checkout/sessions/cs_"):
		card := strings.TrimPrefix(path, "/v1/checkout/sessions/cs_")
		return 200, fmt.Sprintf(`{"id":"cs_%s","object":"checkout.session","mode":"setup","status":"complete","livemode":false,"customer":%q,"setup_intent":"seti_%s","metadata":{"bex_workspace":%q}}`, card, s.customer, card, s.ws)
	case get && strings.HasPrefix(path, "/v1/setup_intents/seti_"):
		card := strings.TrimPrefix(path, "/v1/setup_intents/seti_")
		return 200, fmt.Sprintf(`{"id":"seti_%s","object":"setup_intent","status":"succeeded","customer":%q,"payment_method":"pm_%s","metadata":{"bex_workspace":%q}}`, card, s.customer, card, s.ws)
	case get && strings.HasPrefix(path, "/v1/payment_methods/pm_"):
		card := strings.TrimPrefix(path, "/v1/payment_methods/pm_")
		return 200, fmt.Sprintf(`{"id":"pm_%s","object":"payment_method","customer":%q,"livemode":false,"type":"card","card":{"brand":"visa","last4":"4242","funding":"credit","fingerprint":%q}}`, card, s.customer, s.fingerprints[card])
	case get && path == "/v1/subscriptions":
		return 200, fmt.Sprintf(`{"object":"list","data":[{"id":%q,"object":"subscription","status":"past_due","livemode":false,"customer":%q,"default_payment_method":%s,"metadata":{"bex_workspace":%q,"bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`, s.sub, s.customer, idOrNull(s.subDefault), s.ws)
	case get && path == "/v1/customers/"+s.customer:
		return 200, fmt.Sprintf(`{"id":%q,"object":"customer","livemode":false,"metadata":{"bex_workspace":%q},"invoice_settings":{"default_payment_method":%s}}`, s.customer, s.ws, idOrNull(s.customerDefault))
	case req.Method == http.MethodPost && path == "/v1/customers/"+s.customer:
		if pm := form.Get("invoice_settings[default_payment_method]"); pm != "" {
			s.customerDefault = pm
		}
		return 200, fmt.Sprintf(`{"id":%q,"object":"customer"}`, s.customer)
	case req.Method == http.MethodPost && path == "/v1/subscriptions/"+s.sub:
		if pm := form.Get("default_payment_method"); pm != "" {
			s.subDefault = pm
			s.subUpdates = append(s.subUpdates, pm)
		}
		return 200, fmt.Sprintf(`{"id":%q,"object":"subscription","status":"past_due"}`, s.sub)
	case get && path == "/v1/invoices":
		s.invoiceLists++
		q := req.URL.Query()
		if q.Get("subscription") != s.sub {
			return 400, `{"error":{"type":"invalid_request_error","message":"wrong subscription"}}`
		}
		var data []string
		for i := len(s.invoices) - 1; i >= 0; i-- { // Stripe lists newest first
			if inv := s.invoices[i]; inv.status == q.Get("status") {
				data = append(data, s.invoiceJSON(inv))
			}
		}
		return 200, `{"object":"list","data":[` + strings.Join(data, ",") + `],"has_more":false,"url":"/v1/invoices"}`
	case req.Method == http.MethodPost && strings.HasPrefix(path, "/v1/invoices/") && strings.HasSuffix(path, "/pay"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/invoices/"), "/pay")
		pm := form.Get("payment_method")
		s.pays = append(s.pays, overduePay{invoice: id, paymentMethod: pm, offSession: form.Get("off_session"), key: req.Header.Get("Idempotency-Key")})
		var inv *overdueInvoiceFixture
		for _, candidate := range s.invoices {
			if candidate.id == id {
				inv = candidate
			}
		}
		if inv == nil || inv.status == "paid" {
			return 400, `{"error":{"type":"invalid_request_error","code":"invoice_unexpected_state","message":"not payable"}}`
		}
		if code, ok := s.declines[s.fingerprints[strings.TrimPrefix(pm, "pm_")]]; ok {
			return 402, fmt.Sprintf(`{"error":{"type":"card_error","code":"card_declined","decline_code":%q,"message":"Your card was declined."}}`, code)
		}
		inv.status, inv.remaining = "paid", 0
		return 200, s.invoiceJSON(inv)
	}
	return 500, `{"error":{"type":"api_error","message":"unexpected route ` + req.Method + ` ` + path + `"}}`
}

// idOrNull is a JSON expandable-id field: the quoted id, or null when unset.
func idOrNull(id string) string {
	if id == "" {
		return "null"
	}
	return fmt.Sprintf("%q", id)
}

// overdueTestStore is a fresh tenant on the real store, mapped to the stub's
// Customer and Subscription, with its lifecycle in status.
func overdueTestStore(t *testing.T, status string) (*store.PGStore, *pgxpool.Pool, *overdueStripe) {
	t.Helper()
	pg, pool := writeOffTestStore(t)
	tenant := writeOffTestTenant(t, pg, "overdue")
	stripeFake := newOverdueStripe(tenant.ID)
	ctx := context.Background()
	if err := pg.UpsertBillingProviderMapping(ctx, store.BillingProviderMapping{WorkspaceID: tenant.ID, CustomerID: stripeFake.customer, SubscriptionID: stripeFake.sub}); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.EnsureBillingLifecycle(ctx, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if status != store.BillingHealthy {
		// The failed invoice opens grace, as its webhook would; enforcement
		// is then forced, as the worker leaves it after the deadline.
		failed := store.StripeBillingEvent{
			EventID: "evt_failed_" + tenant.ID, EventType: "invoice.payment_failed", WorkspaceID: tenant.ID,
			CustomerID: stripeFake.customer, SubscriptionID: stripeFake.sub, ObjectID: "in_failed",
			ProviderCreatedAt: time.Now().Add(-time.Hour), Outcome: store.BillingOutcomeFailure, Reason: "payment_failed",
		}
		if _, _, _, err := pg.RecordStripeBillingEvent(ctx, failed, 7*24*time.Hour); err != nil {
			t.Fatal(err)
		}
		if status != store.BillingGrace {
			if _, err := pool.Exec(ctx, `UPDATE billing_lifecycles SET status=$2, enforced_at=now() WHERE workspace_id=$1`, tenant.ID, status); err != nil {
				t.Fatal(err)
			}
		}
	}
	return pg, pool, stripeFake
}

func overdueClient(t *testing.T, pg *store.PGStore, s *overdueStripe) *StripeClient {
	t.Helper()
	c := s.client(t)
	c.state = pg
	c.overdue = &OverduePayments{Store: pg, GracePeriod: 7 * 24 * time.Hour}
	return c
}

func lifecycleStatus(t *testing.T, pg *store.PGStore, workspaceID string) string {
	t.Helper()
	state, err := pg.GetBillingLifecycle(context.Background(), workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	return state.Status
}

// attemptRows reads the workspace's recorded attempts, oldest first, as
// "invoice card outcome reason attempts".
func attemptRows(t *testing.T, pool *pgxpool.Pool, workspaceID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT invoice_id, card_key, outcome, reason, attempts FROM billing_overdue_payments WHERE workspace_id=$1 ORDER BY attempted_at, invoice_id`, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var invoice, card, outcome, reason string
		var attempts int
		if err := rows.Scan(&invoice, &card, &outcome, &reason, &attempts); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s %s %s %s %d", invoice, card, outcome, reason, attempts))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func bindCard(t *testing.T, c *StripeClient, card string) {
	t.Helper()
	if err := c.CompleteCheckoutSession(context.Background(), &stripe.CheckoutSession{ID: "cs_" + card}); err != nil {
		t.Fatalf("bind %s: %v", card, err)
	}
}

// A card bound through Checkout while billing enforcement holds the workspace
// pays its open invoice once, off-session with that card, and the lifecycle
// moves to recovering at once; a redelivered event pays nothing more.
func TestBindInEnforcedPaysTheOpenInvoiceAndRecoversAgainstPostgres(t *testing.T) {
	pg, pool, fake := overdueTestStore(t, store.BillingEnforced)
	fake.fingerprints["good"] = "fp_good"
	fake.addInvoice("in_overdue", 100, 1200, "open")
	c := overdueClient(t, pg, fake)

	bindCard(t, c, "good")
	pays := fake.paysOf("in_overdue")
	if len(pays) != 1 || pays[0].paymentMethod != "pm_good" || pays[0].offSession != "true" || pays[0].key != "bex-overdue-pay-in_overdue-pm_good-1" {
		t.Fatalf("pays = %+v, want one off-session payment with pm_good under its attempt key", pays)
	}
	if got := lifecycleStatus(t, pg, fake.ws); got != store.BillingRecovering {
		t.Fatalf("lifecycle = %q, want recovering", got)
	}
	if got := attemptRows(t, pool, fake.ws); !slices.Equal(got, []string{"in_overdue card:fp_good paid  1"}) {
		t.Fatalf("attempts = %q", got)
	}
	bindCard(t, c, "good") // the same event, redelivered
	if n := len(fake.paysOf("in_overdue")); n != 1 {
		t.Fatalf("pays after redelivery = %d, want 1", n)
	}
}

// A declined card leaves the lifecycle where it was and records the issuer's
// reason. The same card entered again (a new Checkout, a new PaymentMethod,
// the same fingerprint) is not charged again for that invoice; a different
// card pays it, and grace ends at once.
func TestADeclinedCardIsNotRetriedOnTheSameInvoiceAgainstPostgres(t *testing.T) {
	pg, pool, fake := overdueTestStore(t, store.BillingGrace)
	fake.fingerprints["bad1"], fake.fingerprints["bad2"], fake.fingerprints["good"] = "fp_bad", "fp_bad", "fp_good"
	fake.declines["fp_bad"] = "insufficient_funds"
	fake.addInvoice("in_overdue", 100, 1200, "open")
	c := overdueClient(t, pg, fake)

	bindCard(t, c, "bad1")
	if got := lifecycleStatus(t, pg, fake.ws); got != store.BillingGrace {
		t.Fatalf("lifecycle after a decline = %q, want grace unchanged", got)
	}
	bindCard(t, c, "bad2")
	if pays := fake.paysOf("in_overdue"); len(pays) != 1 || pays[0].paymentMethod != "pm_bad1" {
		t.Fatalf("pays = %+v, want only the first attempt with the declined card", pays)
	}
	if got := attemptRows(t, pool, fake.ws); !slices.Equal(got, []string{"in_overdue card:fp_bad declined insufficient_funds 1"}) {
		t.Fatalf("attempts = %q", got)
	}
	bindCard(t, c, "good")
	if pays := fake.paysOf("in_overdue"); len(pays) != 2 || pays[1].paymentMethod != "pm_good" {
		t.Fatalf("pays = %+v, want the new card's attempt", pays)
	}
	if got := lifecycleStatus(t, pg, fake.ws); got != store.BillingHealthy {
		t.Fatalf("lifecycle = %q, want healthy (paid during grace)", got)
	}
}

// A healthy workspace's binding pays nothing and reads no invoice.
func TestBindInHealthyPaysNothingAgainstPostgres(t *testing.T) {
	pg, pool, fake := overdueTestStore(t, store.BillingHealthy)
	fake.fingerprints["good"] = "fp_good"
	fake.addInvoice("in_open", 100, 1200, "open")
	c := overdueClient(t, pg, fake)

	bindCard(t, c, "good")
	if len(fake.pays) != 0 || fake.invoiceLists != 0 {
		t.Fatalf("pays = %+v, invoice lists = %d, want none", fake.pays, fake.invoiceLists)
	}
	if got := attemptRows(t, pool, fake.ws); len(got) != 0 {
		t.Fatalf("attempts = %q, want none", got)
	}
}

// Several overdue invoices are paid oldest first, uncollectible ones too, and
// recovery is recorded only once every one is paid. A decline stops the run:
// the card is not offered the next invoice, and the lifecycle stays.
func TestOverdueInvoicesArePaidOldestFirstUntilADeclineAgainstPostgres(t *testing.T) {
	t.Run("all paid", func(t *testing.T) {
		pg, _, fake := overdueTestStore(t, store.BillingEnforced)
		fake.fingerprints["good"] = "fp_good"
		fake.addInvoice("in_march", 100, 900, "uncollectible")
		fake.addInvoice("in_april", 200, 1200, "open")
		c := overdueClient(t, pg, fake)
		bindCard(t, c, "good")
		var order []string
		for _, p := range fake.pays {
			order = append(order, p.invoice)
		}
		if !slices.Equal(order, []string{"in_march", "in_april"}) {
			t.Fatalf("pay order = %v, want oldest first", order)
		}
		if got := lifecycleStatus(t, pg, fake.ws); got != store.BillingRecovering {
			t.Fatalf("lifecycle = %q, want recovering", got)
		}
	})
	t.Run("declined on the first", func(t *testing.T) {
		pg, _, fake := overdueTestStore(t, store.BillingEnforced)
		fake.fingerprints["bad"] = "fp_bad"
		fake.declines["fp_bad"] = "do_not_honor"
		fake.addInvoice("in_march", 100, 900, "open")
		fake.addInvoice("in_april", 200, 1200, "open")
		c := overdueClient(t, pg, fake)
		bindCard(t, c, "bad")
		if len(fake.pays) != 1 || fake.pays[0].invoice != "in_march" {
			t.Fatalf("pays = %+v, want one attempt on the oldest", fake.pays)
		}
		if got := lifecycleStatus(t, pg, fake.ws); got != store.BillingEnforced {
			t.Fatalf("lifecycle = %q, want enforced unchanged", got)
		}
	})
}

// A claim survives only an attempt Stripe decided: an attempt that failed for
// another reason, or one abandoned pending, is claimed again under the next
// number; a paid or declined one never is.
func TestOverduePaymentClaimsAgainstPostgres(t *testing.T) {
	pg, _, fake := overdueTestStore(t, store.BillingEnforced)
	ctx := context.Background()
	at := time.Now().UTC()
	claim := func(invoice string, at time.Time) int {
		t.Helper()
		n, err := pg.ClaimOverdueInvoicePayment(ctx, fake.ws, invoice, "card:fp", at)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	record := func(invoice, outcome string) {
		t.Helper()
		if err := pg.RecordOverdueInvoicePayment(ctx, fake.ws, invoice, "card:fp", outcome, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if n := claim("in_a", at); n != 1 {
		t.Fatalf("first claim = %d, want 1", n)
	}
	if n := claim("in_a", at.Add(time.Minute)); n != 0 {
		t.Fatalf("claim while pending = %d, want 0", n)
	}
	if n := claim("in_a", at.Add(store.OverduePaymentReclaimAfter+time.Minute)); n != 2 {
		t.Fatalf("claim of an abandoned attempt = %d, want 2", n)
	}
	record("in_a", store.OverduePaymentError)
	if n := claim("in_a", at.Add(store.OverduePaymentReclaimAfter+2*time.Minute)); n != 3 {
		t.Fatalf("claim after an error = %d, want 3", n)
	}
	for _, outcome := range []string{store.OverduePaymentPaid, store.OverduePaymentDeclined} {
		invoice := "in_" + outcome
		claim(invoice, at)
		record(invoice, outcome)
		if n := claim(invoice, at.Add(time.Hour)); n != 0 {
			t.Fatalf("claim after %s = %d, want 0", outcome, n)
		}
	}
}

// A card chosen in the Billing Portal becomes the Subscription's own default
// too, so Stripe stops retrying the failed card, and pays the overdue invoice
// like a Checkout-bound card.
func TestPortalCardMovesTheSubscriptionDefaultAndPaysAgainstPostgres(t *testing.T) {
	pg, _, fake := overdueTestStore(t, store.BillingEnforced)
	fake.fingerprints["failed"], fake.fingerprints["portal"] = "fp_failed", "fp_portal"
	fake.subDefault, fake.customerDefault = "pm_failed", "pm_portal"
	fake.addInvoice("in_overdue", 100, 1200, "open")
	c := overdueClient(t, pg, fake)

	var event stripe.Event
	raw := fmt.Sprintf(`{"id":"evt_portal_card","object":"event","type":"customer.updated","created":%d,"data":{"object":{"id":%q,"object":"customer","metadata":{"bex_workspace":%q},"invoice_settings":{"default_payment_method":"pm_portal"}},"previous_attributes":{"invoice_settings":{"default_payment_method":"pm_failed"}}}}`,
		time.Now().Unix(), fake.customer, fake.ws)
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordPortalPaymentMethodChange(context.Background(), &event); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fake.subUpdates, []string{"pm_portal"}) {
		t.Fatalf("Subscription default updates = %v, want pm_portal", fake.subUpdates)
	}
	if pays := fake.paysOf("in_overdue"); len(pays) != 1 || pays[0].paymentMethod != "pm_portal" {
		t.Fatalf("pays = %+v, want one with the portal's card", pays)
	}
	if got := lifecycleStatus(t, pg, fake.ws); got != store.BillingRecovering {
		t.Fatalf("lifecycle = %q, want recovering", got)
	}
	// Redelivered: the Subscription already charges the card, nothing moves.
	if err := c.RecordPortalPaymentMethodChange(context.Background(), &event); err != nil {
		t.Fatal(err)
	}
	if len(fake.subUpdates) != 1 || len(fake.pays) != 1 {
		t.Fatalf("after redelivery: updates %v pays %+v", fake.subUpdates, fake.pays)
	}
}

// The overdue invoice is the oldest that still owes; none is nil.
func TestOverdueInvoiceIsTheOldestUnpaid(t *testing.T) {
	fake := newOverdueStripe("tea-a")
	c := fake.client(t)
	c.storeCustomer("tea-a", fake.customer)
	if got, err := c.OverdueInvoice(context.Background(), "tea-a"); err != nil || got != nil {
		t.Fatalf("no invoices: %+v, %v, want nil", got, err)
	}
	fake.addInvoice("in_paid", 50, 0, "paid")
	fake.addInvoice("in_april", 200, 1200, "open")
	fake.addInvoice("in_march", 100, 900, "uncollectible")
	got, err := c.OverdueInvoice(context.Background(), "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != "in_march" || got.HostedInvoiceURL != "https://invoice.stripe.test/in_march" {
		t.Fatalf("overdue = %+v, want in_march with its hosted page", got)
	}
}

func TestOverduePaymentOutcome(t *testing.T) {
	cardErr := &stripe.Error{Type: stripe.ErrorTypeCard, Code: stripe.ErrorCodeCardDeclined, DeclineCode: "insufficient_funds", HTTPStatusCode: 402}
	authErr := &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Code: "invoice_payment_intent_requires_action", HTTPStatusCode: 400}
	stateErr := &stripe.Error{Type: stripe.ErrorTypeInvalidRequest, Code: "invoice_unexpected_state", HTTPStatusCode: 400}
	for _, tc := range []struct {
		name            string
		invoice         *stripe.Invoice
		err             error
		outcome, reason string
	}{
		{"paid", &stripe.Invoice{Status: stripe.InvoiceStatusPaid}, nil, store.OverduePaymentPaid, ""},
		{"processing", &stripe.Invoice{Status: stripe.InvoiceStatusOpen, AmountRemaining: 100}, nil, store.OverduePaymentError, "invoice_open"},
		{"declined", nil, cardErr, store.OverduePaymentDeclined, "insufficient_funds"},
		{"needs the customer", nil, authErr, store.OverduePaymentDeclined, "invoice_payment_intent_requires_action"},
		{"not payable", nil, stateErr, store.OverduePaymentError, "invoice_unexpected_state"},
		{"transport", nil, errors.New("connection reset"), store.OverduePaymentError, "provider_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome, reason := overduePaymentOutcome(tc.invoice, tc.err)
			if outcome != tc.outcome || reason != tc.reason {
				t.Fatalf("outcome = %q %q, want %q %q", outcome, reason, tc.outcome, tc.reason)
			}
		})
	}
}
