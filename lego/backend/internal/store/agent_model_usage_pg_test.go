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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// Platform-route token usage (w10/m14 t003) is recorded once per request id
// into usage_hourly, read back by the generic month-to-date verb and the
// quota total, and its ledger is purged by compaction.
func TestAgentModelUsageIsIdempotentAndMetered(t *testing.T) {
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
	t.Cleanup(pool.Close) // registered first, so it runs after the tenant cleanup
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "agent-model-usage", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	at := time.Date(2026, 10, 27, 9, 40, 0, 0, time.UTC)
	const model = "deepseek/deepseek-v4.1-flash-1017"
	first := AgentModelUsage{
		RequestID: "amr-" + tenant.ID, WorkspaceID: tenant.ID, SessionID: "ags-usage", Model: model, At: at,
		InputTokens: 100, OutputTokens: 20, CacheReadTokens: 300, CostSource: AgentModelCostUnpriced,
	}
	for attempt, want := range []bool{true, false} {
		recorded, err := st.RecordAgentModelUsage(ctx, first)
		if err != nil || recorded != want {
			t.Fatalf("attempt %d: recorded=%v err=%v, want %v", attempt, recorded, err, want)
		}
	}
	second := first
	second.RequestID += "-2"
	second.At = at.Add(10 * time.Minute)
	second.InputTokens, second.OutputTokens, second.CacheReadTokens, second.CacheWriteTokens = 1, 2, 0, 4
	if recorded, err := st.RecordAgentModelUsage(ctx, second); err != nil || !recorded {
		t.Fatalf("second request: recorded=%v err=%v", recorded, err)
	}

	rows, err := st.UsageMonthToDate(ctx, tenant.ID, at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, r := range rows {
		if r.ResourceKind != ResourceKindAgentSession || r.ServiceID != "ags-usage" || r.Tier != model {
			t.Fatalf("unexpected row %+v", r)
		}
		got[r.Kind] = r.Total
	}
	want := map[string]int64{
		UsageKindAgentModelInputTokens: 101, UsageKindAgentModelOutputTokens: 22,
		UsageKindAgentModelCacheReadTokens: 300, UsageKindAgentModelCacheWriteTokens: 4,
	}
	for kind, total := range want {
		if got[kind] != total {
			t.Errorf("%s = %d, want %d (rows %+v)", kind, got[kind], total, rows)
		}
	}
	total, err := st.agentSessionMonthToDate(ctx, tenant.ID, at, AgentModelUsageKinds)
	if err != nil || total != 427 {
		t.Fatalf("month-to-date tokens = %d, %v, want 427", total, err)
	}
	if next, err := st.agentSessionMonthToDate(ctx, tenant.ID, at.AddDate(0, 1, 0), AgentModelUsageKinds); err != nil || next != 0 {
		t.Fatalf("next month's tokens = %d, %v, want 0", next, err)
	}

	if _, err := st.CompactUsage(ctx, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	var ledger int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_model_usage_events WHERE workspace_id = $1`, tenant.ID).Scan(&ledger); err != nil || ledger != 0 {
		t.Fatalf("ledger rows after compaction = %d, %v", ledger, err)
	}
	if after, err := st.agentSessionMonthToDate(ctx, tenant.ID, at, AgentModelUsageKinds); err != nil || after != 427 {
		t.Fatalf("month-to-date after compaction = %d, %v, want 427 from usage_monthly", after, err)
	}
}

// ADR091 D1–D3: a priced exchange records its cost columns and meters its
// price into agent_model_cost_microusd once per request id; the cost row
// seals for export about an hour after its window starts, not after 48h.
func TestAgentModelCostIsMeteredOnceAndSealsFast(t *testing.T) {
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
	t.Cleanup(pool.Close) // registered first, so it runs after the tenant cleanup
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "agent-model-cost", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	at := time.Now().UTC().Add(-90 * time.Minute) // its window started ≥90 min ago, so it closed ≥30 min ago
	window := at.Truncate(time.Hour)
	const model = "deepseek/deepseek-v4.1-flash-1017"
	priced := AgentModelUsage{
		RequestID: "amr-cost-" + tenant.ID, WorkspaceID: tenant.ID, SessionID: "ags-cost", Model: model, At: at,
		InputTokens: 10, OutputTokens: 2,
		UpstreamCostMicroUSD: 1000, CostSource: AgentModelCostReported, PriceMicroUSD: 1300,
	}
	for attempt, want := range []bool{true, false} {
		if recorded, err := st.RecordAgentModelUsage(ctx, priced); err != nil || recorded != want {
			t.Fatalf("attempt %d: recorded=%v err=%v, want %v", attempt, recorded, err, want)
		}
	}
	unpriced := priced
	unpriced.RequestID += "-unpriced"
	unpriced.UpstreamCostMicroUSD, unpriced.CostSource, unpriced.PriceMicroUSD = 0, AgentModelCostUnpriced, 0
	if recorded, err := st.RecordAgentModelUsage(ctx, unpriced); err != nil || !recorded {
		t.Fatalf("unpriced: recorded=%v err=%v", recorded, err)
	}

	var upstream, price int64
	var source string
	if err := pool.QueryRow(ctx, `SELECT upstream_cost_microusd, cost_source, price_microusd
		FROM agent_model_usage_events WHERE request_id = $1`, priced.RequestID).Scan(&upstream, &source, &price); err != nil {
		t.Fatal(err)
	}
	if upstream != 1000 || source != AgentModelCostReported || price != 1300 {
		t.Fatalf("event = %d/%s/%d, want 1000/reported/1300", upstream, source, price)
	}
	if err := pool.QueryRow(ctx, `SELECT cost_source FROM agent_model_usage_events WHERE request_id = $1`, unpriced.RequestID).Scan(&source); err != nil || source != AgentModelCostUnpriced {
		t.Fatalf("unpriced event source = %q, %v", source, err)
	}
	var quantity int64
	if err := pool.QueryRow(ctx, `SELECT quantity FROM usage_hourly
		WHERE workspace_id = $1 AND kind = $2 AND tier = $3 AND resource_kind = $4 AND service_id = 'ags-cost' AND window_start = $5`,
		tenant.ID, UsageKindAgentModelCostMicroUSD, model, ResourceKindAgentSession, window).Scan(&quantity); err != nil || quantity != 1300 {
		t.Fatalf("cost meter row = %d, %v, want 1300 (replay and unpriced add nothing)", quantity, err)
	}
	if in, err := st.AgentSpendInputs(ctx, tenant.ID, at); err != nil || in.MonthToDateMicroUSD != 1300 {
		t.Fatalf("month-to-date cost = %d, %v, want 1300", in.MonthToDateMicroUSD, err)
	}
	if tokens, err := st.agentSessionMonthToDate(ctx, tenant.ID, at, AgentModelUsageKinds); err != nil || tokens != 24 {
		t.Fatalf("month-to-date tokens = %d, %v, want 24 (cost is not a token)", tokens, err)
	}

	// Outbox: with only the 48h seal the fresh rows wait; the fast seal
	// releases the cost row alone once its hour has closed.
	now := time.Now().UTC()
	floor := now.Add(-34 * 24 * time.Hour)
	slow := UsageSeal{Before: now.Add(-48 * time.Hour)}
	mine := func(rows []HourlyRow) []HourlyRow {
		var out []HourlyRow
		for _, r := range rows {
			if r.WorkspaceID == tenant.ID {
				out = append(out, r)
			}
		}
		return out
	}
	rows, err := st.SelectUnemittedUsage(ctx, floor, slow, 1000, false)
	if err != nil || len(mine(rows)) != 0 {
		t.Fatalf("48h seal selected %+v, %v, want none", mine(rows), err)
	}
	fast := UsageSeal{FastKinds: []string{UsageKindAgentModelCostMicroUSD}, FastBefore: now.Add(-65 * time.Minute)}
	rows, err = st.SelectUnemittedUsage(ctx, floor, fast, 1000, false)
	if got := mine(rows); err != nil || len(got) != 1 || got[0].Kind != UsageKindAgentModelCostMicroUSD || got[0].Quantity != 1300 {
		t.Fatalf("fast seal selected %+v, %v, want only the cost row", got, err)
	}
	stats, err := st.BillingExportStats(ctx, floor, fast, now, false)
	if err != nil || stats.PendingRows < 1 {
		t.Fatalf("export stats with the fast seal = %+v, %v, want the cost row pending", stats, err)
	}

	// Held (no Stripe catalog Price yet): the cost row is never selected and
	// not counted, but stays pending, so releasing the hold ships it.
	held := fast
	held.HoldKinds = []string{UsageKindAgentModelCostMicroUSD}
	rows, err = st.SelectUnemittedUsage(ctx, floor, held, 1000, false)
	if got := mine(rows); err != nil || len(got) != 0 {
		t.Fatalf("held cost kind selected %+v, %v, want none", got, err)
	}
	heldStats, err := st.BillingExportStats(ctx, floor, held, now, false)
	if err != nil || heldStats.PendingRows != stats.PendingRows-1 {
		t.Fatalf("held export stats pending = %d, %v, want %d (cost row excluded)", heldStats.PendingRows, err, stats.PendingRows-1)
	}
	rows, err = st.SelectUnemittedUsage(ctx, floor, fast, 1000, false)
	if got := mine(rows); err != nil || len(got) != 1 {
		t.Fatalf("released hold selected %+v, %v, want the still-pending cost row", got, err)
	}
}

// A record committing after its hour's row was exported must not add to that
// row (the export is final and would never bill it): its quantity lands in a
// later, still-pending window instead.
func TestAgentModelLateUsageSkipsAnExportedRow(t *testing.T) {
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
	t.Cleanup(pool.Close) // registered first, so it runs after the tenant cleanup
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "agent-model-late", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	at := time.Now().UTC().Add(-3 * time.Hour)
	window := at.Truncate(time.Hour)
	const model = "deepseek/deepseek-v4.1-flash-1017"
	first := AgentModelUsage{
		RequestID: "amr-late-" + tenant.ID, WorkspaceID: tenant.ID, SessionID: "ags-late", Model: model, At: at,
		InputTokens: 10, UpstreamCostMicroUSD: 1000, CostSource: AgentModelCostReported, PriceMicroUSD: 1300,
	}
	if recorded, err := st.RecordAgentModelUsage(ctx, first); err != nil || !recorded {
		t.Fatalf("first: recorded=%v err=%v", recorded, err)
	}
	costRow := HourlyRow{
		WorkspaceID: tenant.ID, ServiceID: "ags-late", Kind: UsageKindAgentModelCostMicroUSD, Tier: model,
		ResourceKind: ResourceKindAgentSession, WindowStart: window, Quantity: 1300,
	}
	attempt := UsageExportAttempt{Row: costRow, TransactionID: "late-" + tenant.ID, EventName: UsageKindAgentModelCostMicroUSD}
	if err := st.MarkUsageAttempted(ctx, []UsageExportAttempt{attempt}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordUsageExportResult(ctx, []UsageExportAttempt{attempt}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	late := first
	late.RequestID += "-2"
	late.InputTokens, late.PriceMicroUSD = 5, 650
	if recorded, err := st.RecordAgentModelUsage(ctx, late); err != nil || !recorded {
		t.Fatalf("late: recorded=%v err=%v", recorded, err)
	}

	type hourRow struct {
		window   time.Time
		quantity int64
		state    string
	}
	read := func(kind string) []hourRow {
		t.Helper()
		rows, err := pool.Query(ctx, `SELECT window_start, quantity, billing_export_state FROM usage_hourly
			WHERE workspace_id = $1 AND kind = $2 ORDER BY window_start`, tenant.ID, kind)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []hourRow
		for rows.Next() {
			var r hourRow
			if err := rows.Scan(&r.window, &r.quantity, &r.state); err != nil {
				t.Fatal(err)
			}
			out = append(out, r)
		}
		return out
	}
	cost := read(UsageKindAgentModelCostMicroUSD)
	if len(cost) != 2 || !cost[0].window.Equal(window) || cost[0].quantity != 1300 || cost[0].state != "emitted" {
		t.Fatalf("cost rows = %+v, want the exported %s row unchanged at 1300", cost, window)
	}
	if !cost[1].window.After(window) || cost[1].quantity != 650 || cost[1].state != "pending" {
		t.Fatalf("late cost row = %+v, want 650 pending in a later window", cost[1])
	}
	// The input-token row of that hour is still pending, so it grows in place.
	if tokens := read(UsageKindAgentModelInputTokens); len(tokens) != 1 || tokens[0].quantity != 15 {
		t.Fatalf("input token rows = %+v, want one row of 15", tokens)
	}
}

// agentSessionMonthToDate runs the spend query's month-to-date sum for any
// meter kinds, so the tests can check the token meters through the same SQL.
func (s *PGStore) agentSessionMonthToDate(ctx context.Context, workspaceID string, now time.Time, kinds []string) (int64, error) {
	now = now.UTC()
	var total int64
	err := s.Pool.QueryRow(ctx, `SELECT `+agentSessionMonthToDateSQL,
		workspaceID, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), kinds).Scan(&total)
	return total, err
}
