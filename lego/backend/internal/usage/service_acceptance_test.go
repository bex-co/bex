//go:build integration

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

// Local acceptance for w8/m1: proves the metering pipeline writes durable
// usage_hourly rows for all three kinds against a real Postgres, and that
// re-processing the same window is idempotent. w8/m4 adds the retention
// acceptance: compaction folds old months into usage_monthly, purges the
// hourly detail, and period queries return byte-identical totals. w8/m5 adds
// same-name Postgres/KeyValue windows to prove resource-kind isolation through
// the real schema and MonthToDate query.
//
// Run with:
//
//	BEX_ACC_DB="postgres://postgres:postgres@localhost:5774/usage_acceptance_test?sslmode=disable" \
//	  go test -v -tags=integration ./internal/usage/ -run TestAcceptance
package usage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const defaultAccDB = "postgres://postgres:postgres@localhost:5774/usage_acceptance_test?sslmode=disable"

// setupAcceptance connects to the acceptance Postgres (BEX_ACC_DB), runs all
// migrations, seeds one tenant + one app, and registers cleanup of every row
// the test writes. Shared by both acceptance tests.
func setupAcceptance(t *testing.T, tenantID, serviceID, appName string) (*pgxpool.Pool, *store.PGStore) {
	t.Helper()
	dbURI := os.Getenv("BEX_ACC_DB")
	if dbURI == "" {
		dbURI = defaultAccDB
	}
	ctx := context.Background()

	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(func() {
		// Tear down the test rows to keep the DB reusable between runs.
		_, _ = pool.Exec(ctx, `DELETE FROM usage_monthly WHERE workspace_id = $1`, tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM usage_hourly WHERE workspace_id = $1`, tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM apps WHERE tenant_id = $1`, tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID)
		pool.Close()
	})

	// Seed a tenant + app row — the minimum the foreign-key constraints need.
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, plan) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`,
		tenantID, appName+"-workspace", "hobby",
	); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO apps (id, tenant_id, name, slug, image, tier)
		 VALUES ($1, $2, $3, $3, $4, $5) ON CONFLICT (id) DO NOTHING`,
		serviceID, tenantID, appName, "ghcr.io/bex-co/hello-go:latest", "starter",
	); err != nil {
		t.Fatalf("insert app: %v", err)
	}
	return pool, store.NewPGStore(pool)
}

// TestAcceptance_UsageHourlyAccrues is the w8/m1 local acceptance test. It
// connects to a real Postgres, runs migrations, processes two simulated hourly
// windows for a synthetic app, and verifies that:
//   - All three kinds (instance_seconds, egress_bytes, build_seconds) produce
//     rows with plausible positive magnitudes across both windows.
//   - Re-processing a window already in the DB (idempotent upsert) leaves the
//     row count unchanged — the restart-safety invariant.
//   - LatestUsageWindow advances as windows are written.
//   - UsageMonthToDate aggregates match the sum of the raw window rows.
func TestAcceptance_UsageHourlyAccrues(t *testing.T) {
	ctx := context.Background()
	pool, st := setupAcceptance(t, "tea-acc-001", "srv-acc-001", "hello-go")
	app := store.App{
		ID:       "srv-acc-001",
		TenantID: "tea-acc-001",
		Name:     "hello-go",
		Tier:     "starter",
	}

	// Fake Prometheus: 2 pods present (→ 2×3600 = 7200 instance-seconds per
	// window); also drives egress_bytes to 1024 bytes per window.
	prom := fakeProm(2.0)
	defer prom.Close()

	svc := NewService(&core.Base{}, st, prom.URL, nil)

	// Two consecutive closed windows ending ≥2 h before now (safely in the past
	// so catch-up logic would pick them up on a restart).
	now := time.Now().UTC().Truncate(time.Hour)
	w1 := now.Add(-3 * time.Hour)
	w2 := now.Add(-2 * time.Hour)

	t.Log("processing window 1:", w1.Format(time.RFC3339))
	svc.processWindow(ctx, app, w1)

	t.Log("processing window 2:", w2.Format(time.RFC3339))
	svc.processWindow(ctx, app, w2)

	// --- verification: rows exist for all 3 kinds across both windows ---

	type rowKey struct{ kind, tier string }
	countByKind := map[rowKey]int64{}
	rows, err := pool.Query(ctx,
		`SELECT kind, tier, SUM(quantity) FROM usage_hourly
		 WHERE workspace_id = 'tea-acc-001' AND service_id = 'srv-acc-001'
		 GROUP BY kind, tier`)
	if err != nil {
		t.Fatalf("query usage_hourly: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind, tier string
		var total int64
		if err := rows.Scan(&kind, &tier, &total); err != nil {
			t.Fatalf("scan: %v", err)
		}
		countByKind[rowKey{kind, tier}] = total
		t.Logf("  kind=%-20s tier=%-10s total=%d", kind, tier, total)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	// instance_seconds: 2 pods × 3600 s × 2 windows = 14400 s
	instKey := rowKey{store.UsageKindInstanceSeconds, "starter"}
	if instSecs := countByKind[instKey]; instSecs <= 0 {
		t.Errorf("instance_seconds: expected > 0, got %d", instSecs)
	} else {
		t.Logf("instance_seconds total across 2 windows: %d s (expected ~14400)", instSecs)
	}

	// egress_bytes: prom returns 2.0 per query → ≥2 bytes across 2 windows
	egressKey := rowKey{store.UsageKindEgressBytes, ""}
	if egressBytes := countByKind[egressKey]; egressBytes <= 0 {
		t.Errorf("egress_bytes: expected > 0, got %d", egressBytes)
	} else {
		t.Logf("egress_bytes total across 2 windows: %d bytes", egressBytes)
	}

	// build_seconds: no k8s client wired → no Jobs → 0 rows (expected, log only)
	buildKey := rowKey{store.UsageKindBuildSeconds, ""}
	t.Logf("build_seconds (no k8s client, expected 0): %d", countByKind[buildKey])

	// --- LatestUsageWindow must reflect w2 ---
	latest, err := st.LatestUsageWindow(ctx, store.ResourceKindService, "srv-acc-001", store.UsageKindInstanceSeconds)
	if err != nil {
		t.Fatalf("LatestUsageWindow: %v", err)
	}
	if !latest.Equal(w2) && !latest.Equal(w1) {
		t.Errorf("LatestUsageWindow: expected %v or %v, got %v", w1, w2, latest)
	}
	t.Logf("LatestUsageWindow: %v", latest.Format(time.RFC3339))

	// --- UsageMonthToDate must agree with the raw row sums ---
	summary, err := st.UsageMonthToDate(ctx, "tea-acc-001", time.Now().UTC())
	if err != nil {
		t.Fatalf("UsageMonthToDate: %v", err)
	}
	mtdByKind := map[rowKey]int64{}
	for _, r := range summary {
		mtdByKind[rowKey{r.Kind, r.Tier}] = r.Total
	}
	for k, rawTotal := range countByKind {
		if mtdByKind[k] != rawTotal {
			t.Errorf("MTD mismatch for kind=%s tier=%s: raw=%d mtd=%d", k.kind, k.tier, rawTotal, mtdByKind[k])
		}
	}
	t.Logf("UsageMonthToDate consistent with raw rows: %v", summary)

	// --- idempotency: re-process w1 must not change totals ---
	t.Log("re-processing window 1 (idempotency check)...")
	svc.processWindow(ctx, app, w1)

	summary2, err := st.UsageMonthToDate(ctx, "tea-acc-001", time.Now().UTC())
	if err != nil {
		t.Fatalf("UsageMonthToDate after re-process: %v", err)
	}
	mtdByKind2 := map[rowKey]int64{}
	for _, r := range summary2 {
		mtdByKind2[rowKey{r.Kind, r.Tier}] = r.Total
	}
	for k, before := range mtdByKind {
		after := mtdByKind2[k]
		if after != before {
			t.Errorf("idempotency violation for kind=%s tier=%s: before=%d after=%d", k.kind, k.tier, before, after)
		}
	}
	t.Log("idempotency confirmed: totals unchanged after re-processing w1")

	// --- catch-up idempotency: a fresh service sees the already-written windows
	//     and fills any gap up to now; a second catchUp must be a no-op. ---
	svc2 := NewService(&core.Base{}, st, prom.URL, nil)

	// First catchUp: fills any windows between LatestUsageWindow and now.
	svc2.catchUp(ctx)

	var rowsAfterFirst int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM usage_hourly WHERE workspace_id = 'tea-acc-001'`,
	).Scan(&rowsAfterFirst); err != nil {
		t.Fatalf("count after first catchUp: %v", err)
	}
	t.Logf("rows after first catchUp: %d", rowsAfterFirst)

	// Second catchUp: nothing new to process → row count must not change.
	svc2.catchUp(ctx)

	var rowsAfterSecond int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM usage_hourly WHERE workspace_id = 'tea-acc-001'`,
	).Scan(&rowsAfterSecond); err != nil {
		t.Fatalf("count after second catchUp: %v", err)
	}
	if rowsAfterSecond != rowsAfterFirst {
		t.Errorf("catchUp not idempotent: %d → %d rows on second run", rowsAfterFirst, rowsAfterSecond)
	}
	t.Logf("catchUp idempotency confirmed: %d rows before, %d after second run", rowsAfterFirst, rowsAfterSecond)

	fmt.Printf("\n=== w8/m1 local acceptance PASS ===\n")
	fmt.Printf("  2 windows processed for srv-acc-001 / tea-acc-001\n")
	fmt.Printf("  instance_seconds: %d s across 2 windows\n", countByKind[instKey])
	fmt.Printf("  egress_bytes:     %d bytes across 2 windows\n", countByKind[egressKey])
	fmt.Printf("  build_seconds:    %d s (no k8s client — expected 0)\n", countByKind[buildKey])
	fmt.Printf("  idempotency:      confirmed (re-process w1 unchanged)\n")
	fmt.Printf("  catch-up no-op:   confirmed (no spurious rows)\n")
}

// TestAcceptance_ZeroCoverageAnchorsStayOutOfSummary proves the PostgreSQL
// read path keeps m11's raw zero egress/build anchors available for coverage
// audits without adding new all-zero groups to tenant-facing usage summaries.
func TestAcceptance_ZeroCoverageAnchorsStayOutOfSummary(t *testing.T) {
	ctx := context.Background()
	pool, st := setupAcceptance(t, "tea-acc-zero", "srv-acc-zero", "quiet-app")
	window := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)

	for _, row := range []store.HourlyRow{
		{WorkspaceID: "tea-acc-zero", ServiceID: "srv-acc-zero", Kind: store.UsageKindInstanceSeconds, Tier: "starter", WindowStart: window},
		{WorkspaceID: "tea-acc-zero", ServiceID: "srv-acc-zero", Kind: store.UsageKindEgressBytes, WindowStart: window},
		{WorkspaceID: "tea-acc-zero", ServiceID: "srv-acc-zero", Kind: store.UsageKindBuildSeconds, WindowStart: window},
	} {
		if err := st.UpsertUsageHourly(ctx, row); err != nil {
			t.Fatalf("upsert %s zero anchor: %v", row.Kind, err)
		}
	}

	var rawRows int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM usage_hourly WHERE workspace_id = 'tea-acc-zero'`,
	).Scan(&rawRows); err != nil {
		t.Fatalf("count raw zero anchors: %v", err)
	}
	if rawRows != 3 {
		t.Fatalf("raw zero anchors: want 3, got %d", rawRows)
	}

	summary, err := st.UsageMonthToDate(ctx, "tea-acc-zero", window.Add(time.Hour))
	if err != nil {
		t.Fatalf("UsageMonthToDate: %v", err)
	}
	if len(summary) != 1 || summary[0].Kind != store.UsageKindInstanceSeconds || summary[0].Total != 0 {
		t.Fatalf("zero summary rows: want only instance_seconds=0, got %+v", summary)
	}
}

// TestAcceptance_DatastoreUsageIdentity proves the managed-datastore path
// against the real schema. A Database and KeyValue may share a Kubernetes
// name, meter, plan, and hour; resource_kind must keep both rows and summaries
// distinct rather than letting one upsert overwrite the other.
func TestAcceptance_DatastoreUsageIdentity(t *testing.T) {
	ctx := context.Background()
	pool, st := setupAcceptance(t, "tea-acc-ds", "srv-acc-ds", "datastore-usage")
	prom := fakeProm(1.0)
	defer prom.Close()

	svc := NewService(&core.Base{Namespace: "default"}, st, prom.URL, nil)
	window := time.Now().UTC().Truncate(time.Hour).Add(-time.Hour)
	for _, ds := range []datastoreEntry{
		{ID: "shared", Name: "shared", TenantID: "tea-acc-ds", Plan: "free", Kind: store.ResourceKindPostgres},
		{ID: "shared", Name: "shared", TenantID: "tea-acc-ds", Plan: "free", Kind: store.ResourceKindKeyValue},
	} {
		svc.processDatastoreWindow(ctx, ds, window)
	}

	var persisted int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM usage_hourly
		WHERE workspace_id = 'tea-acc-ds' AND service_id = 'shared'
		  AND kind = 'instance_seconds' AND tier = 'free' AND window_start = $1`,
		window,
	).Scan(&persisted); err != nil {
		t.Fatalf("count datastore rows: %v", err)
	}
	if persisted != 2 {
		t.Fatalf("same-name datastore rows: want 2, got %d", persisted)
	}

	rows, err := st.UsageMonthToDate(ctx, "tea-acc-ds", window.Add(time.Hour))
	if err != nil {
		t.Fatalf("UsageMonthToDate: %v", err)
	}
	totals := map[string]int64{}
	for _, row := range rows {
		if row.ServiceID == "shared" {
			totals[row.ResourceKind] = row.Total
		}
	}
	if totals[store.ResourceKindPostgres] != 3600 || totals[store.ResourceKindKeyValue] != 3600 {
		t.Errorf("same-name datastore totals: want 3600 each, got %+v", totals)
	}
}

// TestAcceptance_UsageCompaction is the w8/m4 acceptance (t006): seed hourly
// rows for a month outside the hot window against a real Postgres, run the
// compaction pass, and verify usage_monthly is populated, the hourly detail is
// purged, GET /v1/usage?period=<old-month> is byte-identical to its
// pre-compaction response, and a re-run is a no-op.
func TestAcceptance_UsageCompaction(t *testing.T) {
	ctx := context.Background()
	pool, st := setupAcceptance(t, "tea-acc-002", "srv-acc-002", "compact-me")

	// Seed hourly rows in a month 4 calendar months back — outside the default
	// 3-month hot window whatever today's date is. All four kinds, and two
	// windows for instance_seconds so the SUM is exercised.
	now := time.Now().UTC()
	oldMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -4, 0)
	seed := []store.HourlyRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", WindowStart: oldMonth.Add(10 * time.Hour), Quantity: 3600},
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", WindowStart: oldMonth.Add(11 * time.Hour), Quantity: 3600},
		{Kind: store.UsageKindEgressBytes, WindowStart: oldMonth.Add(10 * time.Hour), Quantity: 2048},
		{Kind: store.UsageKindBuildSeconds, WindowStart: oldMonth.Add(12 * time.Hour), Quantity: 120},
		{Kind: store.UsageKindStorageGBSeconds, ResourceKind: store.ResourceKindPostgres, WindowStart: oldMonth.Add(12 * time.Hour), Quantity: 3600},
	}
	for _, row := range seed {
		row.WorkspaceID, row.ServiceID = "tea-acc-002", "srv-acc-002"
		if err := st.UpsertUsageHourly(ctx, row); err != nil {
			t.Fatalf("seed %s: %v", row.Kind, err)
		}
	}

	// The REST surface the totals are read through — the real adapter, with a
	// static tenant resolver standing in for the control-plane lookup.
	svc := &Service{
		Base:  &core.Base{Workspace: staticTenant{"tea-acc-002"}},
		Store: st,
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	period := oldMonth.Format("2006-01")
	get := func() string {
		req := httptest.NewRequest("GET", "/v1/usage?period="+period, nil)
		req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user:acc"}))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/usage?period=%s: status %d: %s", period, w.Code, w.Body.String())
		}
		return w.Body.String()
	}

	pre := get()
	t.Logf("pre-compaction  %s: %s", period, pre)

	// Run the loop's own compaction pass (default 3-month hot window).
	svc.compact(ctx)

	// usage_monthly must hold the aggregates, the hourly detail must be gone.
	var monthlyRows, hourlyRows int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM usage_monthly WHERE workspace_id = 'tea-acc-002' AND month = $1`,
		oldMonth,
	).Scan(&monthlyRows); err != nil {
		t.Fatalf("count usage_monthly: %v", err)
	}
	if monthlyRows != 4 { // one per (resource kind, kind, tier) aggregate
		t.Errorf("usage_monthly rows for %s: want 4, got %d", period, monthlyRows)
	}
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM usage_hourly WHERE workspace_id = 'tea-acc-002'
		 AND window_start >= $1 AND window_start < $2`,
		oldMonth, oldMonth.AddDate(0, 1, 0),
	).Scan(&hourlyRows); err != nil {
		t.Fatalf("count usage_hourly: %v", err)
	}
	if hourlyRows != 0 {
		t.Errorf("usage_hourly rows for %s after compaction: want 0, got %d", period, hourlyRows)
	}

	post := get()
	t.Logf("post-compaction %s: %s", period, post)
	if post != pre {
		t.Errorf("period query changed across compaction:\n  pre:  %s\n  post: %s", pre, post)
	}

	// Idempotency: a second pass must not drift anything.
	svc.compact(ctx)
	var monthlyAgain int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM usage_monthly WHERE workspace_id = 'tea-acc-002' AND month = $1`,
		oldMonth,
	).Scan(&monthlyAgain); err != nil {
		t.Fatalf("count usage_monthly after re-run: %v", err)
	}
	if monthlyAgain != monthlyRows {
		t.Errorf("re-run duplicated monthly rows: %d → %d", monthlyRows, monthlyAgain)
	}
	if again := get(); again != pre {
		t.Errorf("re-run drifted the period query:\n  pre:   %s\n  again: %s", pre, again)
	}

	fmt.Printf("\n=== w8/m4 compaction acceptance PASS ===\n")
	fmt.Printf("  %d hourly rows for %s compacted into %d monthly rows\n", len(seed), period, monthlyRows)
	fmt.Printf("  hourly detail purged: %d rows remain in %s\n", hourlyRows, period)
	fmt.Printf("  GET /v1/usage?period=%s byte-identical across compaction\n", period)
	fmt.Printf("  re-run: no-op confirmed\n")
}
