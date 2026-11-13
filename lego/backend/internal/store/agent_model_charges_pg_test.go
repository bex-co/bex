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

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func agentModelChargeTestStore(t *testing.T) (*PGStore, *pgxpool.Pool, Tenant) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "model-charges-"+time.Now().Format("150405.000000000"), PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })
	return st, pool, tenant
}

// ADR091 D10.2a: concurrent begins (two replicas' mints at one crossing) start
// exactly one charge and the rest join it; a begin whose spend read predates
// the month's latest charge joins that charge instead of charging again; seq
// counts per month; and the transitions are pending → paid|failed and
// failed → paid only.
func TestAgentModelChargeLedger(t *testing.T) {
	st, _, tenant := agentModelChargeTestStore(t)
	ctx := context.Background()
	ws := tenant.ID
	nov := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

	const racers = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		started int
		keys    = map[AgentModelChargeKey]bool{}
	)
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			charge, ok, err := st.BeginAgentModelCharge(ctx, ws, nov, 0, 10_000_000, 1000)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			keys[charge.AgentModelChargeKey] = true
			if ok {
				started++
			}
		}()
	}
	wg.Wait()
	first := AgentModelChargeKey{WorkspaceID: ws, Month: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Seq: 1}
	if started != 1 || len(keys) != 1 || !keys[first] {
		t.Fatalf("%d racers started %d charges, keys %v; want exactly one, seq 1", racers, started, keys)
	}
	charge, err := st.GetAgentModelCharge(ctx, first)
	if err != nil || charge.Status != AgentModelChargePending || charge.AmountMicroUSD != 10_000_000 || charge.AmountCents != 1000 ||
		charge.Attempts != 0 || !charge.CreatedAt.Equal(nov) || !charge.SettledAt.IsZero() {
		t.Fatalf("charge = %+v, %v", charge, err)
	}

	in, err := st.AgentSpendInputs(ctx, ws, nov)
	if err != nil || !in.ChargePending || in.ChargeSeq != 1 || in.ChargedMicroUSD != 0 || in.Chargeable {
		t.Fatalf("pending inputs = %+v, %v (no Subscription: not chargeable)", in, err)
	}

	// The invoice is recorded while pending, an undecided try counted, and
	// the sweep sees the charge only once it is older than the cutoff.
	if err := st.SetAgentModelChargeInvoice(ctx, first, "in_"+ws+"_1"); err != nil {
		t.Fatal(err)
	}
	if err := st.CountAgentModelChargeAttempt(ctx, first); err != nil {
		t.Fatal(err)
	}
	if retry := mineCharges(ctx, t, st, nov, ws); len(retry) != 0 {
		t.Fatalf("a fresh pending charge is retried: %+v", retry)
	}
	if retry := mineCharges(ctx, t, st, nov.Add(time.Minute), ws); len(retry) != 1 || retry[0].Attempts != 1 || retry[0].InvoiceID != "in_"+ws+"_1" {
		t.Fatalf("retry = %+v, want the pending charge with its invoice", retry)
	}

	// Paid: final, counted as charged, and due for its net item.
	paid, err := st.SettleAgentModelCharge(ctx, first, AgentModelChargeSettlement{Status: AgentModelChargePaid, HostedInvoiceURL: "https://pay/1", At: nov.Add(time.Second)})
	if err != nil || paid.Status != AgentModelChargePaid || paid.HostedInvoiceURL != "https://pay/1" || !paid.SettledAt.Equal(nov.Add(time.Second)) {
		t.Fatalf("paid = %+v, %v", paid, err)
	}
	if again, err := st.SettleAgentModelCharge(ctx, first, AgentModelChargeSettlement{Status: AgentModelChargeFailed, Failure: "card_declined", At: nov.Add(time.Hour)}); err != nil ||
		again.Status != AgentModelChargePaid || !again.SettledAt.Equal(paid.SettledAt) {
		t.Fatalf("a paid charge was re-settled: %+v, %v", again, err)
	}
	if err := st.SetAgentModelChargeInvoice(ctx, first, "in_other"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invoice of a settled charge = %v, want ErrNotFound", err)
	}
	if retry := mineCharges(ctx, t, st, nov.Add(time.Minute), ws); len(retry) != 1 || retry[0].Status != AgentModelChargePaid {
		t.Fatalf("retry = %+v, want the paid charge awaiting its net item", retry)
	}
	if err := st.SetAgentModelChargeNetItem(ctx, first, "ii_net_1"); err != nil {
		t.Fatal(err)
	}
	if retry := mineCharges(ctx, t, st, nov.Add(time.Minute), ws); len(retry) != 0 {
		t.Fatalf("a netted charge is retried: %+v", retry)
	}

	// A begin from a read that saw no charge joins the paid one; one that
	// saw it starts seq 2.
	if joined, ok, err := st.BeginAgentModelCharge(ctx, ws, nov, 0, 10_000_000, 1000); err != nil || ok || joined.AgentModelChargeKey != first || joined.Status != AgentModelChargePaid {
		t.Fatalf("stale begin = %+v started=%t, %v; want the paid seq 1 joined", joined, ok, err)
	}
	second, ok, err := st.BeginAgentModelCharge(ctx, ws, nov, 1, 25_000_000, 2500)
	if err != nil || !ok || second.Seq != 2 {
		t.Fatalf("second = %+v started=%t, %v", second, ok, err)
	}
	if err := st.SetAgentModelChargeInvoice(ctx, second.AgentModelChargeKey, "in_"+ws+"_2"); err != nil {
		t.Fatal(err)
	}
	// Declined, then paid later on its hosted page (the webhook names only
	// the invoice).
	failed, err := st.SettleAgentModelCharge(ctx, second.AgentModelChargeKey, AgentModelChargeSettlement{Status: AgentModelChargeFailed, Failure: "card_declined", HostedInvoiceURL: "https://pay/2", At: nov})
	if err != nil || failed.Status != AgentModelChargeFailed || failed.Failure != "card_declined" {
		t.Fatalf("failed = %+v, %v", failed, err)
	}
	in, err = st.AgentSpendInputs(ctx, ws, nov)
	if err != nil || in.ChargePending || in.ChargeSeq != 2 || in.ChargedMicroUSD != 10_000_000 {
		t.Fatalf("inputs after a decline = %+v, %v (a failed charge collected nothing)", in, err)
	}
	later, found, err := st.SettleAgentModelChargeInvoice(ctx, "in_"+ws+"_2", AgentModelChargeSettlement{Status: AgentModelChargePaid, At: nov.Add(time.Hour)})
	if err != nil || !found || later.Status != AgentModelChargePaid || later.Failure != "" || later.HostedInvoiceURL != "https://pay/2" {
		t.Fatalf("paid later = %+v found=%t, %v", later, found, err)
	}
	if _, found, err := st.SettleAgentModelChargeInvoice(ctx, "in_unknown_"+ws, AgentModelChargeSettlement{Status: AgentModelChargePaid, At: nov}); err != nil || found {
		t.Fatalf("unknown invoice found=%t, %v", found, err)
	}

	// The month's paid charges, a Subscription mapping, and a new month.
	if err := st.UpsertBillingProviderMapping(ctx, BillingProviderMapping{
		WorkspaceID: ws, CustomerID: "cus_charges_" + strings.ReplaceAll(ws, "-", ""), SubscriptionID: "sub_charges_" + strings.ReplaceAll(ws, "-", ""),
	}); err != nil {
		t.Fatal(err)
	}
	in, err = st.AgentSpendInputs(ctx, ws, nov)
	if err != nil || in.ChargedMicroUSD != 35_000_000 || in.ChargeSeq != 2 || in.ChargePending || !in.Chargeable {
		t.Fatalf("inputs = %+v, %v; want $35 charged over 2 charges, chargeable", in, err)
	}
	dec := time.Date(2026, 12, 1, 0, 0, 1, 0, time.UTC)
	if in, err = st.AgentSpendInputs(ctx, ws, dec); err != nil || in.ChargedMicroUSD != 0 || in.ChargeSeq != 0 {
		t.Fatalf("December inputs = %+v, %v; want nothing charged yet", in, err)
	}
	decCharge, ok, err := st.BeginAgentModelCharge(ctx, ws, dec, 0, 10_000_000, 1000)
	if err != nil || !ok || decCharge.Seq != 1 || !decCharge.Month.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("December charge = %+v started=%t, %v; want seq 1 of December", decCharge, ok, err)
	}
	// While it is pending, a begin in any month joins it.
	if joined, ok, err := st.BeginAgentModelCharge(ctx, ws, nov, 2, 1_000_000, 100); err != nil || ok || joined.AgentModelChargeKey != decCharge.AgentModelChargeKey {
		t.Fatalf("begin beside a pending charge = %+v started=%t, %v", joined, ok, err)
	}
	all, err := st.ListAgentModelCharges(ctx, ws, nov.Add(-time.Hour))
	if err != nil || len(all) != 3 || all[0].Seq != 1 || all[1].Seq != 2 || all[2].AgentModelChargeKey != decCharge.AgentModelChargeKey {
		t.Fatalf("charges = %+v, %v", all, err)
	}

	// Excluded or comped workspaces are never charged.
	for _, flag := range []string{"billing_excluded", "billing_comped"} {
		if _, err := st.Pool.Exec(ctx, `UPDATE tenants SET `+flag+` = true WHERE id = $1`, ws); err != nil {
			t.Fatal(err)
		}
		if in, err := st.AgentSpendInputs(ctx, ws, nov); err != nil || in.Chargeable {
			t.Fatalf("%s: chargeable = %t, %v", flag, in.Chargeable, err)
		}
		if _, err := st.Pool.Exec(ctx, `UPDATE tenants SET `+flag+` = false WHERE id = $1`, ws); err != nil {
			t.Fatal(err)
		}
	}

	if _, _, err := st.BeginAgentModelCharge(ctx, id.New(id.Workspace), nov, 0, 1, 50); !errors.Is(err, ErrWorkspaceGone) {
		t.Fatalf("begin for a missing workspace = %v, want ErrWorkspaceGone", err)
	}
}

// mineCharges is AgentModelChargesToRetry at cutoff, limited to ws.
func mineCharges(ctx context.Context, t *testing.T, st *PGStore, cutoff time.Time, ws string) []AgentModelCharge {
	t.Helper()
	all, err := st.AgentModelChargesToRetry(ctx, cutoff, 100_000)
	if err != nil {
		t.Fatal(err)
	}
	var out []AgentModelCharge
	for _, c := range all {
		if c.WorkspaceID == ws {
			out = append(out, c)
		}
	}
	return out
}

// ADR091 D10.2a: the charge window opens where a subscription invoice last
// billed the meter. A boundary at 08:39 counts the month's hours
// that start at or after it (09:00 on) and only the paid charges made since;
// the month-to-date total the workspace's own cap reads is unchanged. The
// boundary only moves forward, and a workspace with no mapping ignores it.
func TestAgentSpendInputsChargeWindow(t *testing.T) {
	st, _, tenant := agentModelChargeTestStore(t)
	ctx := context.Background()
	ws := tenant.ID
	month := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 11, 13, 10, 30, 0, 0, time.UTC)
	for i, u := range []struct {
		at    time.Time
		price int64
	}{
		{month.Add(5 * time.Hour), 3_000_000},
		{time.Date(2026, 11, 13, 8, 20, 0, 0, time.UTC), 2_000_000},
		{time.Date(2026, 11, 13, 9, 10, 0, 0, time.UTC), 4_000_000},
	} {
		if _, err := st.RecordAgentModelUsage(ctx, AgentModelUsage{
			RequestID: fmt.Sprintf("amr-window-%d-%s", i, ws), WorkspaceID: ws, SessionID: "ags-window", Model: "deepseek/deepseek-v4.1-flash-1017",
			At: u.at, InputTokens: 1, UpstreamCostMicroUSD: u.price, CostSource: AgentModelCostReported, PriceMicroUSD: u.price,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if advanced, err := st.RecordAgentModelBilledUntil(ctx, ws, now); err != nil || advanced {
		t.Fatalf("billed-until with no mapping: advanced=%t, %v; want a no-op", advanced, err)
	}
	if err := st.UpsertBillingProviderMapping(ctx, BillingProviderMapping{
		WorkspaceID: ws, CustomerID: "cus_window_" + strings.ReplaceAll(ws, "-", ""), SubscriptionID: "sub_window_" + strings.ReplaceAll(ws, "-", ""),
	}); err != nil {
		t.Fatal(err)
	}
	early, _, err := st.BeginAgentModelCharge(ctx, ws, time.Date(2026, 11, 13, 8, 30, 0, 0, time.UTC), 0, 5_000_000, 500)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SettleAgentModelCharge(ctx, early.AgentModelChargeKey, AgentModelChargeSettlement{Status: AgentModelChargePaid, At: now}); err != nil {
		t.Fatal(err)
	}
	in, err := st.AgentSpendInputs(ctx, ws, now)
	if err != nil || !in.UnbilledFrom.Equal(month) || in.MonthToDateMicroUSD != 9_000_000 || in.WindowSpendMicroUSD != 9_000_000 || in.ChargedMicroUSD != 5_000_000 {
		t.Fatalf("no boundary: %+v, %v; want the whole month, $5 charged", in, err)
	}

	boundary := time.Date(2026, 11, 13, 8, 39, 52, 0, time.UTC)
	if advanced, err := st.RecordAgentModelBilledUntil(ctx, ws, boundary); err != nil || !advanced {
		t.Fatalf("billed-until: advanced=%t, %v", advanced, err)
	}
	if advanced, err := st.RecordAgentModelBilledUntil(ctx, ws, boundary.Add(-time.Hour)); err != nil || advanced {
		t.Fatalf("an earlier billed-until moved the boundary back: advanced=%t, %v", advanced, err)
	}
	late, ok, err := st.BeginAgentModelCharge(ctx, ws, time.Date(2026, 11, 13, 9, 50, 0, 0, time.UTC), 1, 1_000_000, 100)
	if err != nil || !ok {
		t.Fatalf("late charge started=%t, %v", ok, err)
	}
	if _, err := st.SettleAgentModelCharge(ctx, late.AgentModelChargeKey, AgentModelChargeSettlement{Status: AgentModelChargePaid, At: now}); err != nil {
		t.Fatal(err)
	}
	in, err = st.AgentSpendInputs(ctx, ws, now)
	if err != nil || !in.UnbilledFrom.Equal(boundary) || in.MonthToDateMicroUSD != 9_000_000 || in.WindowSpendMicroUSD != 4_000_000 ||
		in.ChargedMicroUSD != 1_000_000 || in.ChargeSeq != 2 {
		t.Fatalf("after the 08:39 invoice: %+v, %v; want the 09:00 hour's $4 and the 09:50 charge's $1, month at $9", in, err)
	}

	// A boundary in an earlier month leaves the window at this month's start.
	if in, err = st.AgentSpendInputs(ctx, ws, time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)); err != nil ||
		!in.UnbilledFrom.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) || in.WindowSpendMicroUSD != 0 {
		t.Fatalf("December: %+v, %v; want December's window, nothing spent", in, err)
	}
}
