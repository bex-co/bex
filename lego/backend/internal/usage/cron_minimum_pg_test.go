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

package usage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func newCronMinimumStore(t *testing.T) (*store.PGStore, *pgxpool.Pool, context.Context) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM usage_hourly; DELETE FROM usage_meter_cursors`); err != nil {
		t.Fatal(err)
	}
	return store.NewPGStore(pool), pool, ctx
}

// cronMinimums reads every cron_minimum_microusd row, by cron job.
func cronMinimums(t *testing.T, ctx context.Context, pool *pgxpool.Pool) map[string]store.HourlyRow {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT workspace_id, service_id, resource_kind, tier, window_start, quantity
		FROM usage_hourly WHERE kind = $1`, store.UsageKindCronMinimumMicroUSD)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]store.HourlyRow{}
	for rows.Next() {
		var r store.HourlyRow
		if err := rows.Scan(&r.WorkspaceID, &r.ServiceID, &r.ResourceKind, &r.Tier, &r.WindowStart, &r.Quantity); err != nil {
			t.Fatal(err)
		}
		out[r.ServiceID] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// w9/m448 against Postgres: every cron job active in a calendar month owes the
// cron job minimum less what its runs there cost, once the month's run time is
// final, exactly once.
func TestPGCronMinimum(t *testing.T) {
	st, pool, ctx := newCronMinimumStore(t)
	s := &Service{Store: st}
	minimum := pricing.Default.CronMinimumMicroUSD()
	if minimum != 700_000 {
		t.Fatalf("cron job minimum = %d micro-USD, want 700000 (Render's $1 × 0.70)", minimum)
	}
	tenant, err := st.CreateTenant(ctx, "cron-minimum", store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	app := func(name, typ, tier string) store.App {
		t.Helper()
		a, err := st.CreateApp(ctx, store.App{TenantID: tenant.ID, Name: name, Type: typ, Image: "busybox:1", Branch: "main", Port: 3000, Replicas: 1, Tier: tier})
		if err != nil {
			t.Fatalf("CreateApp(%s): %v", name, err)
		}
		return a
	}
	idle := app("idle", appv1alpha1.TypeCronJob, "starter")        // ran nothing
	busy := app("busy", appv1alpha1.TypeCronJob, "standard")       // runs cost $3
	cheap := app("cheap", appv1alpha1.TypeCronJob, "starter")      // runs cost about $0.40
	deleted := app("deleted", appv1alpha1.TypeCronJob, "starter")  // deleted mid-month
	suspended := app("paused", appv1alpha1.TypeCronJob, "starter") // suspended all month
	web := app("web", appv1alpha1.TypeWebService, "starter")       // not a cron job
	if err := st.SetAppSuspended(ctx, suspended.ID, true); err != nil {
		t.Fatal(err)
	}

	october := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	run := func(a store.App, tier string, seconds int64) {
		t.Helper()
		if err := st.UpsertUsageHourly(ctx, store.HourlyRow{
			WorkspaceID: tenant.ID, ServiceID: a.ID, Kind: store.UsageKindInstanceSeconds, Tier: tier,
			ResourceKind: store.ResourceKindService, WindowStart: october.Add(10 * 24 * time.Hour), Quantity: seconds,
		}); err != nil {
			t.Fatal(err)
		}
	}
	run(busy, "standard", 450_600) // 450,600 s × $17.50/2,628,000 s ≈ $3.00
	run(cheap, "starter", 214_531) // ≈ $0.40
	run(web, "starter", 10)

	// Mid-October the sweep records the active cron jobs; then one is deleted.
	s.chargeCronMinimums(ctx, october.Add(14*24*time.Hour))
	if err := st.DeleteApp(ctx, deleted.ID); err != nil {
		t.Fatal(err)
	}
	if got := cronMinimums(t, ctx, pool); len(got) != 0 {
		t.Fatalf("October charged before it ended: %v", got)
	}

	// October's last hour is still inside the rollup's rewrite window.
	november := october.AddDate(0, 1, 0)
	s.chargeCronMinimums(ctx, november.Add(cronMinimumDelay-time.Minute))
	if got := cronMinimums(t, ctx, pool); len(got) != 0 {
		t.Fatalf("October charged while its run time could still change: %v", got)
	}

	s.chargeCronMinimums(ctx, november.Add(cronMinimumDelay))
	got := cronMinimums(t, ctx, pool)
	cheapRun := cronMinimumTopUp(minimum, pricing.Default.ServiceInstanceMicroUSD("starter", 214_531))
	if cheapRun <= 290_000 || cheapRun >= 310_000 {
		t.Fatalf("cheap cron's top-up = %d, want about 300000 ($0.70 − $0.40)", cheapRun)
	}
	want := map[string]int64{idle.ID: minimum, cheap.ID: cheapRun, deleted.ID: minimum}
	if len(got) != len(want) {
		t.Fatalf("cron minimum rows = %v, want one each for %v", got, want)
	}
	lastHour := november.Add(-time.Hour)
	for id, quantity := range want {
		r, ok := got[id]
		if !ok {
			t.Fatalf("no cron minimum row for %s; rows %v", id, got)
		}
		if r.Quantity != quantity || r.WorkspaceID != tenant.ID || r.ResourceKind != store.ResourceKindService || r.Tier != "" || !r.WindowStart.Equal(lastHour) {
			t.Errorf("row for %s = %+v, want %d micro-USD in %s", id, r, quantity, lastHour)
		}
	}

	// Charging again, from a later tick or with the month re-read, adds nothing
	// and never rewrites a row that may already be on its way to Stripe.
	run(cheap, "starter", 1)
	s.chargeCronMinimums(ctx, november.Add(cronMinimumDelay+time.Hour))
	if err := s.chargeCronMinimumMonth(ctx, october, minimum); err != nil {
		t.Fatal(err)
	}
	again := cronMinimums(t, ctx, pool)
	if len(again) != len(want) || again[cheap.ID].Quantity != cheapRun {
		t.Fatalf("a second charge changed the rows: %v, want %v", again, got)
	}

	// The row is an ordinary usage row: the export picks it up once sealed,
	// except in a workspace excluded from billing.
	seal := store.UsageSeal{Before: november.Add(48 * time.Hour)}
	pending, err := st.SelectUnemittedUsage(ctx, october, seal, 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := countKind(pending, store.UsageKindCronMinimumMicroUSD); n != len(want) {
		t.Fatalf("export selects %d cron minimum rows, want %d", n, len(want))
	}
	if _, err := st.SetTenantBillingExcluded(ctx, tenant.ID, true, "test", november); err != nil {
		t.Fatal(err)
	}
	pending, err = st.SelectUnemittedUsage(ctx, october, seal, 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := countKind(pending, store.UsageKindCronMinimumMicroUSD); n != 0 {
		t.Fatalf("an excluded workspace's cron minimum is exported: %d rows", n)
	}

	// The cron job minimum is priced at $0.000001 a unit, under the cron job.
	est := pricing.Default.Estimate([]store.UsageSummaryRow{{ServiceID: idle.ID, Kind: store.UsageKindCronMinimumMicroUSD, ResourceKind: store.ResourceKindService, Total: minimum}})
	if est.TotalUSD != "0.70" {
		t.Fatalf("estimate of one cron minimum = %s, want 0.70", est.TotalUSD)
	}
}

func countKind(rows []store.HourlyRow, kind string) int {
	n := 0
	for _, r := range rows {
		if r.Kind == kind {
			n++
		}
	}
	return n
}
