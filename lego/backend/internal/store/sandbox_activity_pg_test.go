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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestSandboxActivityPG proves the idle-suspension record (w2/m278) against
// Postgres: activity only moves forward, exactly one replica claims an idle
// sandbox, a touch after the cutoff wins over the sweeper, and a stale claim
// can be retaken.
func TestSandboxActivityPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "sandbox-activity", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	ws := tenant.ID
	t0 := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	idle := 15 * time.Minute
	grace := 30 * time.Second

	// A never-touched sandbox is seeded from its create time: claimable once
	// its window has passed, never before.
	if ok, err := s.ClaimSandboxIdleSuspend(ctx, ws, "os-new", t0, t0.Add(10*time.Minute).Add(-idle), t0.Add(10*time.Minute), t0.Add(10*time.Minute).Add(-grace)); err != nil || ok {
		t.Fatalf("claim inside the window = %v %v, want false", ok, err)
	}

	// Activity only moves forward: a late heartbeat cannot rewind it.
	if _, err := s.TouchSandboxActivity(ctx, ws, "os-1", t0.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TouchSandboxActivity(ctx, ws, "os-1", t0); err != nil {
		t.Fatal(err)
	}
	now := t0.Add(19 * time.Minute) // 14 min after the newest touch
	if ok, err := s.ClaimSandboxIdleSuspend(ctx, ws, "os-1", t0, now.Add(-idle), now, now.Add(-grace)); err != nil || ok {
		t.Fatalf("claim 14 min after activity = %v %v, want false (the late touch rewound it)", ok, err)
	}

	// Past the window, concurrent sweepers on every replica: exactly one wins.
	now = t0.Add(21 * time.Minute)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			ok, err := s.ClaimSandboxIdleSuspend(ctx, ws, "os-1", t0, now.Add(-idle), now, now.Add(-grace))
			if err != nil {
				t.Errorf("claim: %v", err)
			}
			if ok {
				wins.Add(1)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("%d sweepers claimed the idle sandbox, want exactly 1", wins.Load())
	}

	// An exec that arrives after the claim sees it, so it waits for the pause
	// and resumes instead of racing it.
	claimed, err := s.TouchSandboxActivity(ctx, ws, "os-1", now.Add(time.Second))
	if err != nil || claimed == nil || !claimed.Equal(now) {
		t.Fatalf("touch after claim returned claim %v err %v, want %v", claimed, err, now)
	}
	if err := s.ClearSandboxSuspendClaim(ctx, ws, "os-1"); err != nil {
		t.Fatal(err)
	}
	if claimed, err := s.TouchSandboxActivity(ctx, ws, "os-1", now.Add(2*time.Second)); err != nil || claimed != nil {
		t.Fatalf("claim after clear = %v %v, want none", claimed, err)
	}

	// A claim whose sweeper never paused is retaken once stale, and only
	// when the sandbox is still idle.
	later := now.Add(30 * time.Minute)
	if ok, _ := s.ClaimSandboxIdleSuspend(ctx, ws, "os-1", t0, later.Add(-idle), later, later.Add(-grace)); !ok {
		t.Fatal("idle sandbox not claimable after its claim was cleared")
	}
	if ok, _ := s.ClaimSandboxIdleSuspend(ctx, ws, "os-1", t0, later.Add(-idle), later.Add(time.Second), later.Add(time.Second).Add(-grace)); ok {
		t.Fatal("a fresh claim was taken twice")
	}
	stale := later.Add(time.Minute)
	if ok, _ := s.ClaimSandboxIdleSuspend(ctx, ws, "os-1", t0, stale.Add(-idle), stale, stale.Add(-grace)); !ok {
		t.Fatal("a stale claim was not retaken")
	}

	// Prune keeps only the sandboxes a complete list still shows.
	if err := s.PruneSandboxActivity(ctx, ws, []string{"os-1"}); err != nil {
		t.Fatal(err)
	}
	var ids []string
	rows, err := pool.Query(ctx, `SELECT sandbox_id FROM sandbox_activity WHERE workspace_id = $1 ORDER BY sandbox_id`, ws)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) != 1 || ids[0] != "os-1" {
		t.Fatalf("after prune rows = %v, want [os-1]", ids)
	}
	if err := s.PruneSandboxActivity(ctx, ws, nil); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sandbox_activity WHERE workspace_id = $1`, ws).Scan(&n); err != nil || n != 0 {
		t.Fatalf("after empty prune %d rows (err %v), want 0", n, err)
	}
}

// TestSandboxMeterStopsAtSuspendingPG: the idle sweeper records `suspending`
// the moment it pauses a sandbox, and compute stops accruing there, through
// suspended and until the sandbox runs again (w2/m278).
func TestSandboxMeterStopsAtSuspendingPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "sandbox-idle-meter", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	for _, step := range []struct {
		phase string
		at    time.Duration
	}{
		{"running", 0},
		{"suspending", 15 * time.Minute}, // idle sweeper pauses: 900 s charged
		{"suspended", 16 * time.Minute},
		{"suspended", 50 * time.Minute},
		{"resuming", 55 * time.Minute},
		{"running", 56 * time.Minute},
		{"suspending", 58 * time.Minute}, // 120 s more
	} {
		if err := s.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: tenant.ID, SandboxID: "os-idle", Phase: step.phase,
			Tier: "1c-2g", WeightMilli: 1000, ObservedAt: t0.Add(step.at),
		}); err != nil {
			t.Fatal(err)
		}
	}
	var total int64
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0) FROM usage_hourly
		WHERE workspace_id = $1 AND service_id = 'os-idle' AND kind = $2`,
		tenant.ID, UsageKindSandboxComputeSeconds).Scan(&total); err != nil {
		t.Fatal(err)
	}
	// weight 1000 milli = 1 unit per second: 900 s + 120 s of running.
	if want := int64((900 + 120) * 1000); total != want {
		t.Fatalf("sandbox_compute_seconds = %d, want %d (only running time bills)", total, want)
	}
}
