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
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func uncollectibleInvoice(workspaceID, invoiceID string, total, amountRemaining int64, periodStart time.Time) string {
	return fmt.Sprintf(`{"id":%q,"object":"invoice","livemode":false,"customer":"cus_x","total":%d,"amount_remaining":%d,"status":"uncollectible",`+
		`"period_start":%d,"parent":{"type":"subscription_details","subscription_details":`+
		`{"subscription":"sub_x","metadata":{"bex_workspace":%q,"bex_billing_contract":"true"}}}}`,
		invoiceID, total, amountRemaining, periodStart.Unix(), workspaceID)
}

// trustInvoicesFake answers AgentModelInvoiceLines from a map and counts
// finalizations.
type trustInvoicesFake map[string]AgentModelInvoice

func (f trustInvoicesFake) AgentModelInvoiceLines(_ context.Context, invoiceID string) (AgentModelInvoice, error) {
	return f[invoiceID], nil
}

func (f trustInvoicesFake) FinalizeThresholdInvoice(context.Context, string) error { return nil }

func (f trustInvoicesFake) WrittenOffCardFingerprint(context.Context, string, string) (string, error) {
	return "", nil
}

type writeOffsFake struct{ got []store.AgentModelWriteOff }

func (f *writeOffsFake) RecordAgentModelWriteOff(_ context.Context, w store.AgentModelWriteOff) (bool, error) {
	f.got = append(f.got, w)
	return true, nil
}

// invoice.marked_uncollectible records the agent-model lines' net amount,
// prorated by the unpaid share of the invoice, against the month the usage
// was billed for; compute-only, zero-total and foreign invoices record
// nothing, and the tier is not touched.
func TestTrustRecordsAgentModelWriteOff(t *testing.T) {
	at := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	oct := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	period := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	writeOffs := &writeOffsFake{}
	tierChanges := 0
	h := &Trust{Store: &trustStoreFake{}, WriteOffs: writeOffs,
		Invoices: trustInvoicesFake{
			"in_model":    {NetCents: 1234, UsageMonth: oct},
			"in_partial":  {NetCents: 5000, UsageMonth: oct},
			"in_noperiod": {NetCents: 700},
			"in_compute":  {},
			"in_zero":     {NetCents: 100, UsageMonth: oct},
		},
		OnTierChanged: func(context.Context, string, pricing.Tier) { tierChanges++ }}
	for _, e := range []*stripe.Event{
		trustEvent("evt_1", stripe.EventTypeInvoiceMarkedUncollectible, at, uncollectibleInvoice("tea-a", "in_model", 2000, 2000, period)),
		trustEvent("evt_2", stripe.EventTypeInvoiceMarkedUncollectible, at, uncollectibleInvoice("tea-a", "in_partial", 6000, 300, period)),
		trustEvent("evt_3", stripe.EventTypeInvoiceMarkedUncollectible, at, uncollectibleInvoice("tea-a", "in_noperiod", 700, 700, period)),
		trustEvent("evt_4", stripe.EventTypeInvoiceMarkedUncollectible, at, uncollectibleInvoice("tea-a", "in_compute", 900, 900, period)),
		trustEvent("evt_5", stripe.EventTypeInvoiceMarkedUncollectible, at, uncollectibleInvoice("tea-a", "in_zero", 0, 0, period)),
		trustEvent("evt_6", stripe.EventTypeInvoiceMarkedUncollectible, at, `{"id":"in_foreign","object":"invoice","customer":"cus_x","total":100,"amount_remaining":100}`),
	} {
		if err := h.HandleStripeEvent(context.Background(), e); err != nil {
			t.Fatalf("%s: %v", e.ID, err)
		}
	}
	want := []store.AgentModelWriteOff{
		{InvoiceID: "in_model", WorkspaceID: "tea-a", CustomerID: "cus_x", UsageMonth: oct,
			AgentModelNetMicroUSD: 12_340_000, AmountMicroUSD: 12_340_000, WrittenOffAt: at},
		// 5000 × 300 / 6000 = 250 cents.
		{InvoiceID: "in_partial", WorkspaceID: "tea-a", CustomerID: "cus_x", UsageMonth: oct,
			AgentModelNetMicroUSD: 50_000_000, AmountMicroUSD: 2_500_000, WrittenOffAt: at},
		// No line period: the invoice's period_start.
		{InvoiceID: "in_noperiod", WorkspaceID: "tea-a", CustomerID: "cus_x", UsageMonth: period,
			AgentModelNetMicroUSD: 7_000_000, AmountMicroUSD: 7_000_000, WrittenOffAt: at},
	}
	if fmt.Sprint(writeOffs.got) != fmt.Sprint(want) {
		t.Fatalf("write-offs = %+v, want %+v", writeOffs.got, want)
	}
	if tierChanges != 0 {
		t.Fatalf("a write-off changed the tier %d times", tierChanges)
	}
	if !isTrustEvent(stripe.EventTypeInvoiceMarkedUncollectible) {
		t.Fatal("invoice.marked_uncollectible is not dispatched to Trust")
	}
}

// AgentModelInvoiceLines sums only the lines that rate the agent-model meter,
// net of their discounts, and reports the first one's period month.
func TestAgentModelInvoiceLinesNetsDiscounts(t *testing.T) {
	names := pricing.Default.BillableMeterNames()
	catalog := stripePriceCatalogJSON(t, names)
	i := agentModelIndex(t)
	other := (i + 1) % len(names)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	lines := fmt.Sprintf(`{"object":"list","has_more":false,"url":"/v1/invoices/in_1/lines","data":[`+
		`{"id":"il_1","object":"line_item","amount":1500,"discount_amounts":[{"amount":500}],"period":{"start":%d,"end":%d},"pricing":{"type":"price_details","price_details":{"price":{"id":"price_%02d","object":"price","recurring":{"meter":"mtr_%02d"}}}}},`+
		`{"id":"il_2","object":"line_item","amount":700,"discount_amounts":[{"amount":700}],"pricing":{"type":"price_details","price_details":{"price":{"id":"price_%02d","object":"price","recurring":{"meter":"mtr_%02d"}}}}},`+
		`{"id":"il_3","object":"line_item","amount":20,"pricing":{"type":"price_details","price_details":{"price":{"id":"price_old","object":"price","recurring":{"meter":"mtr_%02d"}}}}}]}`,
		start.Unix(), start.AddDate(0, 1, 0).Unix(), i, i, other, other, i)
	c, stub := newStripeTest(t, func(method, path string) (int, string) {
		switch {
		case method == http.MethodGet && path == "/v1/prices":
			return 200, catalog
		case method == http.MethodGet && path == "/v1/invoices/in_1/lines":
			return 200, lines
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
	got, err := c.AgentModelInvoiceLines(context.Background(), "in_1")
	if err != nil {
		t.Fatal(err)
	}
	if got.NetCents != 1020 || !got.UsageMonth.Equal(start) {
		t.Fatalf("agent-model lines = %+v, want 1020 cents net (current and retired Prices, after discounts) in %s", got, start)
	}
	if got := stub.count("/v1/invoices/in_1/lines"); got != 1 {
		t.Fatalf("lines calls = %d", got)
	}
}

func writeOffTestStore(t *testing.T) (*store.PGStore, *pgxpool.Pool) {
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
	return store.NewPGStore(pool), pool
}

func writeOffTestTenant(t *testing.T, pg *store.PGStore, prefix string) store.Tenant {
	t.Helper()
	tenant, err := pg.CreateTenant(context.Background(), prefix+"-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pg.DeleteTenant(context.Background(), tenant.ID) })
	return tenant
}

// Real DB: a write-off counts in the month its usage was billed for, against
// that month's revenue snapshot; both survive the workspace's deletion; the
// write-off refuses the mint until paid, and paying the invoice deletes it
// even after the workspace is gone. Old write-offs are purged.
func TestAgentModelWriteOffsAgainstPostgres(t *testing.T) {
	pg, pool := writeOffTestStore(t)
	ctx := context.Background()
	tenant := writeOffTestTenant(t, pg, "writeoff")
	suffix := strings.ReplaceAll(tenant.ID, "-", "")
	// A month no other test writes; this test owns its snapshot and sums.
	month := time.Date(2031, 3, 1, 0, 0, 0, 0, time.UTC)
	next := month.AddDate(0, 1, 0)
	for _, q := range []string{
		`DELETE FROM billing_agent_model_revenue_months WHERE month IN ($1, $2)`,
		`DELETE FROM billing_agent_model_writeoffs WHERE usage_month IN ($1, $2)`,
	} {
		if _, err := pool.Exec(ctx, q, month, next); err != nil {
			t.Fatal(err)
		}
	}

	session, err := pg.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID: tenant.ID, Repo: "octo/repo", Branch: "bex-agent/task-1",
		AgentConfig: []byte(`{"agent":"claude","task":"x"}`), InitialPrompt: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{month.Add(36 * time.Hour), next.Add(time.Hour)} {
		if _, err := pg.RecordAgentModelUsage(ctx, store.AgentModelUsage{
			RequestID: id.New(id.AgentModelRequest), WorkspaceID: tenant.ID, SessionID: session.ID,
			Model: "anthropic/claude-sonnet-4.5", At: at, InputTokens: 1, CostSource: store.AgentModelCostReported,
			UpstreamCostMicroUSD: 400_000_000, PriceMicroUSD: 400_000_000,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := pg.RefreshAgentModelRevenue(ctx, month.Add(5*24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	// Billed for March, written off in April: it is March's bad debt.
	writeOff := store.AgentModelWriteOff{InvoiceID: "in_wo_" + suffix, WorkspaceID: tenant.ID, CustomerID: "cus_wo_" + suffix,
		UsageMonth: month.Add(24 * time.Hour), AgentModelNetMicroUSD: 20_000_000, AmountMicroUSD: 10_000_000, WrittenOffAt: next.Add(10 * 24 * time.Hour)}
	for n, want := range []bool{true, false} {
		recorded, err := pg.RecordAgentModelWriteOff(ctx, writeOff)
		if err != nil || recorded != want {
			t.Fatalf("record %d: recorded=%v err=%v, want %v", n, recorded, err, want)
		}
	}
	bad := writeOff
	bad.InvoiceID, bad.AmountMicroUSD = "in_wo_bad_"+suffix, 30_000_000
	if _, err := pg.RecordAgentModelWriteOff(ctx, bad); !errors.Is(err, store.ErrInvalidAgentModelWriteOff) {
		t.Fatalf("write-off above its net = %v, want ErrInvalidAgentModelWriteOff", err)
	}
	if debt, err := pg.AgentModelBadDebtForMonth(ctx, month); err != nil {
		t.Fatal(err)
	} else if debt != (store.AgentModelBadDebt{RevenueMicroUSD: 400_000_000, WrittenOffMicroUSD: 10_000_000}) {
		t.Fatalf("March bad debt = %+v, want $400 revenue and $10 written off", debt)
	}
	if debt, err := pg.AgentModelBadDebtForMonth(ctx, next); err != nil || debt.WrittenOffMicroUSD != 0 {
		t.Fatalf("April bad debt = %+v, %v: the write-off belongs to March", debt, err)
	}
	if in, err := pg.AgentSpendInputs(ctx, tenant.ID, time.Now()); err != nil || !in.BalanceDue {
		t.Fatalf("an unpaid write-off: BalanceDue=%v err=%v, want refused", in.BalanceDue, err)
	}

	// The workspace is deleted with the debt unpaid: the debt and the
	// month's revenue stay; a later payment of the invoice still clears it.
	if err := pg.DeleteTenant(ctx, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if err := pg.RefreshAgentModelRevenue(ctx, month); err != nil {
		t.Fatal(err)
	}
	if debt, err := pg.AgentModelBadDebtForMonth(ctx, month); err != nil || debt.RevenueMicroUSD != 400_000_000 || debt.WrittenOffMicroUSD != 10_000_000 {
		t.Fatalf("after deletion = %+v, %v, want both kept", debt, err)
	}
	h := &Trust{Store: pg}
	if err := h.HandleStripeEvent(ctx, trustEvent("evt_paid_"+suffix, stripe.EventTypeInvoicePaid, time.Now(),
		trustInvoice(tenant.ID, writeOff.InvoiceID, 1000, "subscription_cycle", time.Now()))); err != nil {
		t.Fatalf("paid after deletion: %v", err)
	}
	if debt, err := pg.AgentModelBadDebtForMonth(ctx, month); err != nil || debt.WrittenOffMicroUSD != 0 {
		t.Fatalf("after payment = %+v, %v, want the write-off gone", debt, err)
	}

	purged := writeOff
	purged.InvoiceID = "in_wo_old_" + suffix
	if _, err := pg.RecordAgentModelWriteOff(ctx, purged); err != nil {
		t.Fatal(err)
	}
	if n, err := pg.PurgeAgentModelWriteOffs(ctx, purged.WrittenOffAt.Add(time.Second)); err != nil || n != 1 {
		t.Fatalf("purge = %d, %v, want the one old write-off", n, err)
	}
}

// Real DB (ADR091 D10.3): every failed invoice refuses the mint until it is
// itself paid or voided; a written-off one keeps refusing until paid. The
// T0 lock still follows the newest failure only.
func TestBalanceDueTracksEveryFailedInvoiceAgainstPostgres(t *testing.T) {
	pg, _ := writeOffTestStore(t)
	ctx := context.Background()
	tenant := writeOffTestTenant(t, pg, "balance-due")
	ws := tenant.ID
	now := time.Now().UTC().Truncate(time.Second)
	h := &Trust{Store: pg, WriteOffs: pg, Invoices: trustInvoicesFake{"in_u_" + ws: {NetCents: 500, UsageMonth: now}}}
	send := func(id string, eventType stripe.EventType, at time.Time, object string) {
		t.Helper()
		if err := h.HandleStripeEvent(ctx, trustEvent(id, eventType, at, object)); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	invoice := func(invoiceID string, paid int64) string {
		return trustInvoice(ws, invoiceID, paid, "subscription_threshold", now)
	}
	due := func(want bool, step string) {
		t.Helper()
		in, err := pg.AgentSpendInputs(ctx, ws, now)
		if err != nil || in.BalanceDue != want {
			t.Fatalf("%s: BalanceDue=%v err=%v, want %v", step, in.BalanceDue, err, want)
		}
	}
	oldInv, newInv := "in_old_"+ws, "in_new_"+ws
	send("e1", stripe.EventTypeInvoicePaymentFailed, now.Add(-2*time.Hour), invoice(oldInv, 0))
	send("e2", stripe.EventTypeInvoicePaymentFailed, now.Add(-time.Hour), invoice(newInv, 0))
	due(true, "two failures")
	send("e3", stripe.EventTypeInvoicePaid, now, invoice(newInv, 1000))
	due(true, "the newer paid, the older still open")
	if trust, err := pg.GetBillingTrust(ctx, ws); err != nil || trust.FailedInvoiceID != "" || trust.LockedUntil.Equal(store.LockedIndefinitely) {
		t.Fatalf("lock after the newest failure is paid = %+v, %v, want settled (t005)", trust, err)
	}
	send("e4", stripe.EventTypeInvoiceVoided, now, invoice(oldInv, 0))
	due(false, "the older voided")
	send("e5", stripe.EventTypeInvoicePaymentFailed, now.Add(-2*time.Hour), invoice(oldInv, 0))
	due(false, "a redelivered failure of a voided invoice")
	if trust, err := pg.GetBillingTrust(ctx, ws); err != nil || trust.LockedUntil.Equal(store.LockedIndefinitely) {
		t.Fatalf("a voided invoice's late failure re-locked: %+v, %v", trust, err)
	}

	// Voided before its failure arrives; a $0 payment closes too.
	send("e6", stripe.EventTypeInvoiceVoided, now, invoice("in_v_"+ws, 0))
	send("e7", stripe.EventTypeInvoicePaymentFailed, now, invoice("in_v_"+ws, 0))
	send("e8", stripe.EventTypeInvoicePaymentFailed, now, invoice("in_z_"+ws, 0))
	due(true, "a new failure")
	send("e9", stripe.EventTypeInvoicePaid, now, invoice("in_z_"+ws, 0))
	due(false, "voided-first and $0-paid invoices")

	// Written off: still owed, still refused, until paid.
	uInv := "in_u_" + ws
	send("e10", stripe.EventTypeInvoicePaymentFailed, now, invoice(uInv, 0))
	send("e11", stripe.EventTypeInvoiceMarkedUncollectible, now, uncollectibleInvoice(ws, uInv, 500, 500, now))
	due(true, "written off")
	send("e12", stripe.EventTypeInvoicePaid, now, invoice(uInv, 500))
	due(false, "the written-off invoice paid")
}

// Real DB: the threshold sweep syncs a workspace once per trust change,
// skips Stripe when the recorded value already matches, backs a failure off
// without blocking, keeps a workspace due when its trust changes during a
// sync, and resyncs when a payment lock expires.
func TestAgentModelThresholdSweepAgainstPostgres(t *testing.T) {
	pg, _ := writeOffTestStore(t)
	ctx := context.Background()
	tenant := writeOffTestTenant(t, pg, "threshold-sweep")
	ws := tenant.ID
	now := time.Now().UTC()
	setter := &thresholdSetterFake{fail: map[string]bool{}}
	th := &AgentModelThresholds{Store: pg, Stripe: setter, Clock: func() time.Time { return now }}
	calls := func(want ...string) {
		t.Helper()
		if got := thresholdValues(setter.calls[ws]); !slices.Equal(got, want) {
			t.Fatalf("Stripe syncs = %v, want %v", got, want)
		}
	}
	isDue := func() bool {
		t.Helper()
		due, err := pg.AgentModelThresholdsDue(ctx, now, 100_000)
		if err != nil {
			t.Fatal(err)
		}
		return slices.Contains(due, ws)
	}

	if _, _, _, err := pg.RecordBillingTrustPayment(ctx, store.BillingTrustPayment{
		InvoiceID: "in_t1_" + ws, WorkspaceID: ws, AmountCents: 1000, PaidAt: now.Add(-48 * time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if !isDue() {
		t.Fatal("a never-synced trust record is not due")
	}
	_ = th.RunOnce(ctx)
	_ = th.RunOnce(ctx)
	calls("25000000")
	if isDue() {
		t.Fatal("a synced workspace is still due")
	}

	// A failed charge: T0, $10. Stripe refuses; the retry waits for its
	// backoff, then succeeds.
	failed := "in_failed_" + ws
	// Failed 31 days ago, so a payment can settle it into a lock ending soon.
	if _, _, _, err := pg.RecordBillingTrustFailure(ctx, ws, failed, now.Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	setter.fail[ws] = true
	_ = th.RunOnce(ctx)
	_ = th.RunOnce(ctx)
	calls("25000000", "10000000")
	setter.fail[ws] = false
	now = now.Add(2 * time.Minute)
	_ = th.RunOnce(ctx)
	calls("25000000", "10000000", "10000000")

	// A trust change committed while a sync is in flight leaves it due.
	if err := pg.WithAgentModelThresholdSync(ctx, ws, now, func(state store.AgentModelThresholdState) (*int64, error) {
		if _, _, _, err := pg.RecordBillingTrustPayment(ctx, store.BillingTrustPayment{
			InvoiceID: "in_race_" + ws, WorkspaceID: ws, AmountCents: 100, PaidAt: now,
		}); err != nil {
			return nil, err
		}
		return state.SyncedMicroUSD, nil
	}); err != nil {
		t.Fatal(err)
	}
	if !isDue() {
		t.Fatal("a trust change during a sync was lost")
	}

	// Paying the failed invoice starts the 30-day lock, which ends a minute
	// from now: still T0 ($10, already applied, so no Stripe call) until it
	// expires, then T1 again.
	if _, _, _, err := pg.RecordBillingTrustPayment(ctx, store.BillingTrustPayment{
		InvoiceID: failed, WorkspaceID: ws, AmountCents: 1000, PaidAt: now.Add(-store.TrustLockAfterSettlement + time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	_ = th.RunOnce(ctx)
	calls("25000000", "10000000", "10000000")
	if isDue() {
		t.Fatal("due before the lock expires")
	}
	now = now.Add(2 * time.Minute)
	if !isDue() {
		t.Fatal("not due after the lock expired")
	}
	_ = th.RunOnce(ctx)
	calls("25000000", "10000000", "10000000", "25000000")
}
