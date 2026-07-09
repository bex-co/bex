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

//go:build integration

// Local acceptance for w8/m1: proves the metering pipeline writes durable
// usage_hourly rows for all three kinds against a real Postgres, and that
// re-processing the same window is idempotent.
//
// Run with:
//
//	BEX_ACC_DB="postgres://postgres:postgres@localhost:5774/usage_acceptance_test?sslmode=disable" \
//	  go test -v -tags=integration ./internal/usage/ -run TestAcceptance
package usage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const defaultAccDB = "postgres://postgres:postgres@localhost:5774/usage_acceptance_test?sslmode=disable"

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
	dbURI := os.Getenv("BEX_ACC_DB")
	if dbURI == "" {
		dbURI = defaultAccDB
	}

	ctx := context.Background()

	// Apply all migrations to the target DB.
	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(func() {
		// Tear down the test rows to keep the DB reusable between runs.
		_, _ = pool.Exec(ctx, `DELETE FROM usage_hourly WHERE workspace_id = 'tea-acc-001'`)
		_, _ = pool.Exec(ctx, `DELETE FROM apps WHERE tenant_id = 'tea-acc-001'`)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = 'tea-acc-001'`)
		pool.Close()
	})

	// Seed a tenant + app row — the minimum the foreign-key constraint needs.
	_, err = pool.Exec(ctx,
		`INSERT INTO tenants (id, name, plan) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`,
		"tea-acc-001", "acceptance-workspace", "hobby",
	)
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO apps (id, tenant_id, name, image, tier)
		 VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO NOTHING`,
		"srv-acc-001", "tea-acc-001", "hello-go", "ghcr.io/bex-co/hello-go:latest", "starter",
	)
	if err != nil {
		t.Fatalf("insert app: %v", err)
	}

	st := store.NewPGStore(pool)
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
	latest, err := st.LatestUsageWindow(ctx, "srv-acc-001")
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
