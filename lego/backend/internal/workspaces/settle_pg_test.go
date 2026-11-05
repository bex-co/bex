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

package workspaces

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// meterFake is the emitter's Stripe meter sink: it accepts every event and
// adds the agent-model cost (micro-USD) to the balance the invoice stub bills
// and to the meter total Stripe aggregates.
type meterFake struct{ stripe *settleStripe }

func (meterFake) EnsureCustomer(context.Context, string) error { return nil }
func (meterFake) EnsureContract(context.Context, string) error { return nil }
func (m meterFake) IngestBatch(_ context.Context, events []billing.Event) billing.IngestResult {
	var out billing.IngestResult
	for _, e := range events {
		if e.EventType == store.UsageKindAgentModelCostMicroUSD {
			n, _ := strconv.ParseInt(e.Properties["value"], 10, 64)
			m.stripe.meter(n)
		}
		out.Accepted = append(out.Accepted, e.TransactionID)
	}
	return out
}

// settleStripe stubs the Stripe objects of one workspace's settle and
// cancel: meter summaries report what was metered (less lag), the preview
// shows the unbilled balance, resetting the billing cycle anchor invoices it,
// and paying charges the card unless it declines. Cancelling marks the
// Subscription canceled and retiring the Customer stamps bex_deleted_at.
type settleStripe struct {
	mu          sync.Mutex
	ws          string
	unbilled    int64 // micro-USD not yet invoiced
	metered     int64 // micro-USD Stripe has aggregated
	lag         int64 // micro-USD still aggregating
	invoices    map[string]*stubInvoice
	order       []string
	declining   bool
	previewZero bool
	canceled    bool
	retired     bool
	charged     int64 // cents
	calls       []string
}

type stubInvoice struct {
	cents int64
	paid  bool
}

func (s *settleStripe) meter(microUSD int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unbilled += microUSD
	s.metered += microUSD
}

func (s *settleStripe) count(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func (s *settleStripe) addInvoice(cents int64) string {
	id := fmt.Sprintf("in_settle_%d", len(s.order)+1)
	s.invoices[id] = &stubInvoice{cents: cents, paid: cents == 0}
	s.order = append(s.order, id)
	return id
}

func (s *settleStripe) invoiceJSON(id string) string {
	inv := s.invoices[id]
	status, remaining := "open", inv.cents
	if inv.paid {
		status, remaining = "paid", 0
	}
	return fmt.Sprintf(`{"id":%q,"object":"invoice","status":%q,"currency":"usd","amount_due":%d,"amount_remaining":%d,`+
		`"hosted_invoice_url":"https://invoice.stripe.com/i/%s","parent":{"type":"subscription_details","subscription_details":`+
		`{"subscription":"sub_1","metadata":{"bex_workspace":%q,"bex_billing_contract":"true"}}}}`, id, status, inv.cents, remaining, id, s.ws)
}

// lookupKeys reads stripe-go's indexed lookup_keys[n] query parameters.
func lookupKeys(q map[string][]string) []string {
	var keys []string
	for k, v := range q {
		if strings.HasPrefix(k, "lookup_keys") {
			keys = append(keys, v...)
		}
	}
	return keys
}

func (s *settleStripe) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, req.Method+" "+req.URL.Path)
	status, body := 200, ""
	path, query := req.URL.Path, req.URL.Query()
	periodStart := time.Now().Add(-72 * time.Hour).Unix()
	switch {
	case req.Method == http.MethodGet && path == "/v1/subscriptions":
		subStatus := "active"
		if s.canceled {
			subStatus = "canceled"
		}
		body = fmt.Sprintf(`{"object":"list","data":[{"id":"sub_1","object":"subscription","status":%q,"metadata":{"bex_workspace":%q,"bex_billing_contract":"true"},`+
			`"items":{"object":"list","data":[{"id":"si_1","object":"subscription_item","current_period_start":%d}],"has_more":false}}],"has_more":false,"url":"/v1/subscriptions"}`,
			subStatus, s.ws, periodStart)
	case req.Method == http.MethodGet && path == "/v1/customers/cus_settle":
		marker := ""
		if s.retired {
			marker = `,"bex_deleted_at":"2026-11-05T00:00:00Z"`
		}
		body = `{"id":"cus_settle","object":"customer","metadata":{"bex_workspace":"` + s.ws + `"` + marker + `}}`
	case req.Method == http.MethodGet && path == "/v1/prices":
		var data []string
		for _, key := range lookupKeys(query) {
			data = append(data, fmt.Sprintf(`{"id":"price_%s","object":"price","active":true,"currency":"usd","lookup_key":%q,"recurring":{"interval":"month","usage_type":"metered","meter":"mtr_%s"}}`, key, key, key))
		}
		body = `{"object":"list","data":[` + strings.Join(data, ",") + `],"has_more":false,"url":"/v1/prices"}`
	case req.Method == http.MethodGet && strings.HasPrefix(path, "/v1/billing/meters/") && strings.HasSuffix(path, "/event_summaries"):
		value := 0.0
		if path == "/v1/billing/meters/mtr_"+store.UsageKindAgentModelCostMicroUSD+"/event_summaries" && query.Get("customer") == "cus_settle" {
			value = float64(s.metered - s.lag)
		}
		body = fmt.Sprintf(`{"object":"list","data":[{"id":"mtrusg_1","object":"billing.meter_event_summary","aggregated_value":%g}],"has_more":false,"url":"/v1/billing/meters/x/event_summaries"}`, value)
	case req.Method == http.MethodPost && path == "/v1/invoices/create_preview":
		due := s.unbilled / 10_000
		if s.previewZero {
			due = 0
		}
		body = fmt.Sprintf(`{"object":"invoice","status":"draft","currency":"usd","amount_due":%d}`, due)
	case req.Method == http.MethodPost && path == "/v1/subscriptions/sub_1":
		id := s.addInvoice(s.unbilled / 10_000)
		s.unbilled = 0
		body = fmt.Sprintf(`{"id":"sub_1","object":"subscription","status":"past_due","latest_invoice":%s}`, s.invoiceJSON(id))
	case req.Method == http.MethodGet && path == "/v1/invoices":
		var data []string
		if query.Get("status") == "open" && query.Get("subscription") == "sub_1" {
			for _, id := range s.order {
				if !s.invoices[id].paid {
					data = append(data, s.invoiceJSON(id))
				}
			}
		}
		body = `{"object":"list","data":[` + strings.Join(data, ",") + `],"has_more":false,"url":"/v1/invoices"}`
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/pay"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/invoices/"), "/pay")
		if s.declining {
			status, body = 402, `{"error":{"type":"card_error","code":"card_declined","message":"Your card was declined."}}`
			break
		}
		s.invoices[id].paid = true
		s.charged += s.invoices[id].cents
		body = s.invoiceJSON(id)
	case req.Method == http.MethodDelete && path == "/v1/subscriptions/sub_1":
		s.canceled = true
		s.addInvoice(s.unbilled / 10_000) // invoice_now
		s.unbilled = 0
		body = `{"id":"sub_1","object":"subscription","status":"canceled"}`
	case req.Method == http.MethodGet && path == "/v1/payment_methods":
		body = `{"object":"list","data":[],"has_more":false,"url":"/v1/payment_methods"}`
	case req.Method == http.MethodPost && path == "/v1/customers/cus_settle":
		s.retired = true
		body = `{"id":"cus_settle","object":"customer"}`
	default:
		status, body = 500, `{"error":{"type":"api_error","message":"unexpected route `+req.Method+` `+path+`"}}`
	}
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: h, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

func settlePG(t *testing.T) *store.PGStore {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return store.NewPGStore(pool)
}

type settleFixture struct {
	pg     *store.PGStore
	tenant store.Tenant
	other  store.Tenant
	stripe *settleStripe
	svc    *Service
}

// newSettleFixture is a bound, billable workspace with $3 of agent-model
// usage not yet exported ($1 in a closed hour, $2 in the open one) and an
// open-hour sandbox row, wired to the settle and the Stripe purger exactly as
// cmd/api wires them.
func newSettleFixture(t *testing.T) settleFixture {
	t.Helper()
	pg := settlePG(t)
	ctx := context.Background()
	tenant, err := pg.CreateTenant(ctx, "settle-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), tenant.ID) })
	if err := pg.UpsertBillingProviderMapping(ctx, store.BillingProviderMapping{WorkspaceID: tenant.ID, CustomerID: "cus_settle", SubscriptionID: "sub_1"}); err != nil {
		t.Fatal(err)
	}
	if err := pg.SetPaymentMethodBound(ctx, tenant.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, row := range []store.HourlyRow{
		{ServiceID: "ags-settle-1", Kind: store.UsageKindAgentModelCostMicroUSD, Tier: "model", ResourceKind: store.ResourceKindAgentSession, WindowStart: now.Truncate(time.Hour).Add(-2 * time.Hour), Quantity: 1_000_000},
		{ServiceID: "ags-settle-1", Kind: store.UsageKindAgentModelCostMicroUSD, Tier: "model", ResourceKind: store.ResourceKindAgentSession, WindowStart: now.Truncate(time.Hour), Quantity: 2_000_000},
		{ServiceID: "sbx-settle-1", Kind: store.UsageKindSandboxComputeSeconds, Tier: "standard", ResourceKind: store.ResourceKindSandbox, WindowStart: now.Truncate(time.Hour), Quantity: 60},
	} {
		row.WorkspaceID = tenant.ID
		if err := pg.UpsertUsageHourly(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	other, err := pg.CreateTenant(ctx, "settle-other-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), other.ID) })
	if err := pg.UpsertUsageHourly(ctx, store.HourlyRow{WorkspaceID: other.ID, ServiceID: "ags-settle-other", Kind: store.UsageKindAgentModelCostMicroUSD, Tier: "model",
		ResourceKind: store.ResourceKindAgentSession, WindowStart: now.Truncate(time.Hour), Quantity: 5_000_000}); err != nil {
		t.Fatal(err)
	}
	fake := &settleStripe{ws: tenant.ID, invoices: map[string]*stubInvoice{}}
	client := billing.NewStripe(billing.StripeConfig{SecretKey: "sk_test_x", HTTPClient: &http.Client{Transport: fake},
		BaseURL: "https://stub.stripe.test", MaxNetworkRetries: stripe.Int64(0), State: pg, SettleWait: 300 * time.Millisecond})
	emitter := billing.NewEmitter(pg, meterFake{fake})
	emitter.RequirePaymentMethod = true
	emitter.FastKinds = []string{store.UsageKindAgentModelCostMicroUSD}
	svc := &Service{Store: pg,
		PreCascadePurgers: []WorkspacePurger{&billing.WorkspacePurger{Canceller: client}},
		Settler:           &billing.Settler{Emitter: emitter, Usage: pg, Invoices: client}}
	return settleFixture{pg: pg, tenant: tenant, other: other, stripe: fake, svc: svc}
}

func (f settleFixture) pendingRows(t *testing.T) (pending, total int) {
	t.Helper()
	err := f.pg.Pool.QueryRow(context.Background(), `
		SELECT count(*) FILTER (WHERE billing_export_state = 'pending'), count(*)
		FROM usage_hourly WHERE workspace_id = $1`, f.tenant.ID).Scan(&pending, &total)
	if err != nil {
		t.Fatal(err)
	}
	return pending, total
}

// ADR091 D10.4: deleting a workspace with $3 of unbilled usage exports it
// past the seal, invoices and charges exactly $3, then deletes.
func TestDeleteSettlesUnbilledUsageFirst(t *testing.T) {
	f := newSettleFixture(t)
	if err := (AccountTeardown{Service: f.svc}).Delete(context.Background(), f.tenant.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if f.stripe.charged != 300 || f.stripe.count("POST /v1/invoices/in_settle_1/pay") != 1 || f.stripe.count("POST /v1/subscriptions/sub_1") != 1 {
		t.Fatalf("charged %d cents with %v, want 300 in one anchor reset and one charge", f.stripe.charged, f.stripe.calls)
	}
	var otherPending int
	if err := f.pg.Pool.QueryRow(context.Background(), `SELECT count(*) FROM usage_hourly WHERE workspace_id = $1 AND billing_export_state = 'pending'`,
		f.other.ID).Scan(&otherPending); err != nil || otherPending != 1 {
		t.Fatalf("another workspace's open-hour row: %d pending (%v), want left for the normal outbox", otherPending, err)
	}
	if _, err := f.pg.GetTenant(context.Background(), f.tenant.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("workspace after a settled delete: %v, want gone", err)
	}
	calls := strings.Join(f.stripe.calls, "\n")
	if strings.Index(calls, "/pay") > strings.Index(calls, "DELETE /v1/subscriptions/sub_1") {
		t.Fatalf("the subscription was cancelled before the charge:\n%s", calls)
	}
}

// A declined charge refuses the delete with BILLING_BALANCE_DUE and the
// payment link; the workspace, its usage rows and its Subscription stay. A
// retry after the invoice is paid proceeds.
func TestDeleteRefusedWhileBalanceDueThenRetried(t *testing.T) {
	f := newSettleFixture(t)
	f.stripe.declining = true
	ctx := context.Background()
	err := (AccountTeardown{Service: f.svc}).Delete(ctx, f.tenant.ID)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeBillingBalanceDue || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("delete with a declining card = %v, want BILLING_BALANCE_DUE", err)
	}
	if coded.Params["hostedInvoiceUrl"] != "https://invoice.stripe.com/i/in_settle_1" || coded.Params["amountDueCents"] != int64(300) ||
		coded.Params["workspaceId"] != f.tenant.ID || !strings.Contains(coded.Error(), "https://invoice.stripe.com/i/in_settle_1") {
		t.Fatalf("refusal params = %+v (%s)", coded.Params, coded.Error())
	}
	if _, err := f.pg.GetTenant(ctx, f.tenant.ID); err != nil {
		t.Fatalf("workspace after a refused delete: %v, want intact", err)
	}
	if pending, total := f.pendingRows(t); total != 3 || pending != 1 {
		t.Fatalf("usage rows after the refusal: %d pending of %d, want the open sandbox hour pending of 3", pending, total)
	}
	if n := f.stripe.count("DELETE /v1/subscriptions"); n != 0 {
		t.Fatalf("the subscription was cancelled %d times on a refused delete", n)
	}

	// The customer pays the invoice on Stripe's hosted page.
	f.stripe.mu.Lock()
	f.stripe.invoices["in_settle_1"].paid = true
	f.stripe.declining = false
	f.stripe.mu.Unlock()
	if err := (AccountTeardown{Service: f.svc}).Delete(ctx, f.tenant.ID); err != nil {
		t.Fatalf("retry after paying: %v", err)
	}
	if _, err := f.pg.GetTenant(ctx, f.tenant.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("workspace after the retry: %v, want gone", err)
	}
	if f.stripe.charged != 0 {
		t.Fatalf("the retry charged %d cents again", f.stripe.charged)
	}
	if n := f.stripe.count("POST /v1/subscriptions/sub_1"); n != 1 {
		t.Fatalf("billing cycle anchor reset %d times, want once (the retry found nothing due)", n)
	}
	if n := f.stripe.count("POST /v1/invoices/in_settle_1/pay"); n != 1 {
		t.Fatalf("the declined invoice was charged %d times by bex, want once", n)
	}
}

// Billing disabled (no Stripe key): no settler is wired and delete is
// unchanged.
func TestDeleteWithoutBillingIsUnchanged(t *testing.T) {
	f := newSettleFixture(t)
	svc := &Service{Store: f.pg}
	if err := (AccountTeardown{Service: svc}).Delete(context.Background(), f.tenant.ID); err != nil {
		t.Fatal(err)
	}
	if len(f.stripe.calls) != 0 {
		t.Fatalf("Stripe was called with billing disabled: %v", f.stripe.calls)
	}
	if _, err := f.pg.GetTenant(context.Background(), f.tenant.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("workspace: %v, want gone", err)
	}
}

// Stripe's meter summaries lag what was exported: the delete is refused with
// the retryable BILLING_SETTLING and the Subscription is not touched (no
// preview, no anchor reset, no charge). Once Stripe catches up, the retry
// charges exactly once and deletes.
func TestDeleteWaitsForMeterAggregation(t *testing.T) {
	f := newSettleFixture(t)
	f.stripe.lag = 1_000_000
	ctx := context.Background()
	err := (AccountTeardown{Service: f.svc}).Delete(ctx, f.tenant.ID)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeBillingSettling || !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("delete while Stripe lags = %v, want BILLING_SETTLING", err)
	}
	for _, call := range []string{"POST /v1/invoices/create_preview", "POST /v1/subscriptions/sub_1", "POST /v1/invoices/", "DELETE /v1/subscriptions"} {
		if n := f.stripe.count(call); n != 0 {
			t.Fatalf("%s called %d times while Stripe was still aggregating", call, n)
		}
	}
	if _, err := f.pg.GetTenant(ctx, f.tenant.ID); err != nil {
		t.Fatalf("workspace after BILLING_SETTLING: %v, want intact", err)
	}
	f.stripe.mu.Lock()
	f.stripe.lag = 0
	f.stripe.mu.Unlock()
	if err := (AccountTeardown{Service: f.svc}).Delete(ctx, f.tenant.ID); err != nil {
		t.Fatalf("retry after Stripe caught up: %v", err)
	}
	if f.stripe.charged != 300 || f.stripe.count("POST /v1/subscriptions/sub_1") != 1 || f.stripe.count("POST /v1/invoices/in_settle_1/pay") != 1 {
		t.Fatalf("charged %d cents with %v", f.stripe.charged, f.stripe.calls)
	}
}

// A retry after a declined settle finds the open invoice and pays only it: the
// billing cycle anchor is not reset again.
func TestDeletePaysOpenInvoiceWithoutResettingAnchor(t *testing.T) {
	f := newSettleFixture(t)
	f.stripe.mu.Lock()
	f.stripe.addInvoice(150)
	f.stripe.mu.Unlock()
	if err := (AccountTeardown{Service: f.svc}).Delete(context.Background(), f.tenant.ID); err != nil {
		t.Fatal(err)
	}
	if f.stripe.charged != 150 {
		t.Fatalf("charged %d cents, want the open invoice's 150", f.stripe.charged)
	}
	if n := f.stripe.count("POST /v1/subscriptions/sub_1") + f.stripe.count("POST /v1/invoices/create_preview"); n != 0 {
		t.Fatalf("previewed or reset the anchor %d times with an open invoice", n)
	}
}

// Nothing due (a comped workspace, or usage already invoiced): the preview
// shows $0, so the customer's billing date is not moved and nothing is charged.
func TestDeleteWithNothingDueKeepsBillingDate(t *testing.T) {
	f := newSettleFixture(t)
	f.stripe.previewZero = true
	if err := (AccountTeardown{Service: f.svc}).Delete(context.Background(), f.tenant.ID); err != nil {
		t.Fatal(err)
	}
	if f.stripe.count("POST /v1/subscriptions/sub_1") != 0 || f.stripe.count("POST /v1/invoices/in_") != 0 {
		t.Fatalf("a $0 preview still reset or charged: %v", f.stripe.calls)
	}
}

type failOncePurger struct{ failed bool }

func (p *failOncePurger) PurgeWorkspace(context.Context, string) error {
	if !p.failed {
		p.failed = true
		return errors.New("purger unavailable")
	}
	return nil
}

// A retry after the Stripe purger already ran (the Subscription is cancelled
// and the Customer retired) settles nothing and the deletion proceeds; so
// does a Customer carrying the bex_deleted_at marker with a Subscription
// still listed.
func TestDeleteRetryAfterStripeTeardownSkipsSettle(t *testing.T) {
	f := newSettleFixture(t)
	ctx := context.Background()
	f.svc.PreCascadePurgers = append(f.svc.PreCascadePurgers, &failOncePurger{})
	if err := (AccountTeardown{Service: f.svc}).Delete(ctx, f.tenant.ID); err == nil {
		t.Fatal("the failing purger did not fail the first delete")
	}
	if f.stripe.charged != 300 || !f.stripe.canceled {
		t.Fatalf("first attempt: charged %d, canceled %v", f.stripe.charged, f.stripe.canceled)
	}
	before := len(f.stripe.calls)
	if err := (AccountTeardown{Service: f.svc}).Delete(ctx, f.tenant.ID); err != nil {
		t.Fatalf("retry after the Stripe teardown: %v", err)
	}
	for _, call := range f.stripe.calls[before:] {
		if strings.Contains(call, "create_preview") || strings.HasSuffix(call, "/pay") || call == "POST /v1/subscriptions/sub_1" {
			t.Fatalf("the retry settled again: %v", f.stripe.calls[before:])
		}
	}
	if _, err := f.pg.GetTenant(ctx, f.tenant.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("workspace after the retry: %v, want gone", err)
	}

	g := newSettleFixture(t)
	g.stripe.retired = true
	if err := g.svc.Settler.SettleWorkspace(ctx, g.tenant.ID); err != nil {
		t.Fatal(err)
	}
	if pending, _ := g.pendingRows(t); pending != 3 || len(g.stripe.calls) != 2 {
		t.Fatalf("a retired Customer was settled: %d rows pending, calls %v", pending, g.stripe.calls)
	}
}
