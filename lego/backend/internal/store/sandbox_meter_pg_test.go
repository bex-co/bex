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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestSandboxMeterPG proves the money-path invariants against Postgres: each
// phase sample advances one durable cursor, retries do not double count,
// suspended time is free, intervals split across hours, a disappeared sandbox
// closes its final interval, and monthly compaction remains transparent.
func TestSandboxMeterPG(t *testing.T) {
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
	tenant, err := s.CreateTenant(ctx, "sandbox-meter", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SandboxKeyForWorkspace(ctx, tenant.ID); err != nil {
		t.Fatal(err)
	}
	keys, err := s.ListSandboxTenantKeys(ctx)
	if err != nil || len(keys) != 1 || keys[0].WorkspaceID != tenant.ID || keys[0].APIKey == "" {
		t.Fatalf("tenant keys = %+v err=%v", keys, err)
	}

	t0 := time.Date(2026, 8, 1, 0, 15, 0, 0, time.UTC)
	observe := func(phase string, at time.Time) {
		t.Helper()
		if err := s.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: tenant.ID, SandboxID: "os-metered", Phase: phase,
			Tier: "starter", WeightMilli: 553, ObservedAt: at,
		}); err != nil {
			t.Fatalf("observe %s at %s: %v", phase, at, err)
		}
	}
	// Concurrent first samples serialize even though no cursor row exists yet.
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- s.ObserveSandboxMeter(ctx, SandboxMeterObservation{
				WorkspaceID: tenant.ID, SandboxID: "os-metered", Phase: "running",
				Tier: "starter", WeightMilli: 553, ObservedAt: t0,
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent first observation: %v", err)
		}
	}
	observe("running", t0.Add(30*time.Minute))
	observe("running", t0.Add(30*time.Minute)) // exact retry: no double count
	observe("suspended", t0.Add(time.Hour))
	observe("suspended", t0.Add(105*time.Minute))
	observe("running", t0.Add(2*time.Hour))
	observe("running", t0.Add(150*time.Minute))
	if err := s.TerminateMissingSandboxMeters(ctx, tenant.ID, []string{}, t0.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Re-running the same complete-list reconciliation is also a no-op.
	if err := s.TerminateMissingSandboxMeters(ctx, tenant.ID, []string{}, t0.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}

	rows, err := s.UsageMonthToDate(ctx, tenant.ID, t0.Add(4*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	const want = int64(553 * 2 * 60 * 60) // two running hours; one suspended hour is free
	if len(rows) != 1 || rows[0].Kind != UsageKindSandboxComputeSeconds ||
		rows[0].ResourceKind != ResourceKindSandbox || rows[0].Total != want {
		t.Fatalf("sandbox usage = %+v, want total %d", rows, want)
	}

	if _, err := s.CompactUsage(ctx, t0.Add(4*time.Hour)); err != nil {
		t.Fatal(err)
	}
	rows, err = s.UsageMonthToDate(ctx, tenant.ID, t0.Add(5*time.Hour))
	if err != nil || len(rows) != 1 || rows[0].Total != want {
		t.Fatalf("compacted sandbox usage = %+v err=%v, want total %d", rows, err, want)
	}
}

// TestSandboxMemoryMeterPG proves the shadow memory meter (w22/m313): a 2 GiB
// sandbox running 3,600 s records 7,200 GiB-seconds in its own kind and the
// same weighted total as before in the old one; a terminate mid-interval closes
// it once; a replayed or older sample adds nothing; the sub-unit remainder
// carries across one-second samples; and a sandbox whose memory is unknown is
// metered for compute only.
func TestSandboxMemoryMeterPG(t *testing.T) {
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
	tenant, err := s.CreateTenant(ctx, "sandbox-memory-meter", PlanPro)
	if err != nil {
		t.Fatal(err)
	}

	t0 := time.Date(2026, 8, 1, 0, 15, 0, 0, time.UTC)
	observe := func(id, phase string, memoryMiB int64, at time.Time) {
		t.Helper()
		if err := s.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: tenant.ID, SandboxID: id, Phase: phase,
			Tier: "1c-2g", WeightMilli: 1000, MemoryMiB: memoryMiB, ObservedAt: at,
		}); err != nil {
			t.Fatalf("observe %s %s at %s: %v", id, phase, at, err)
		}
	}
	totals := func(id string) map[string]int64 {
		t.Helper()
		rows, err := s.UsageMonthToDate(ctx, tenant.ID, t0.Add(24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int64{}
		for _, r := range rows {
			if r.ServiceID == id && r.ResourceKind == ResourceKindSandbox {
				got[r.Kind] += r.Total
			}
		}
		return got
	}

	// 2 GiB for one hour, across an hour boundary, with a replay and an older sample.
	observe("os-two-gib", "running", 2048, t0)
	observe("os-two-gib", "running", 2048, t0.Add(30*time.Minute))
	observe("os-two-gib", "running", 2048, t0.Add(30*time.Minute)) // exact replay
	observe("os-two-gib", "running", 2048, t0.Add(10*time.Minute)) // older sample
	observe("os-two-gib", "suspended", 2048, t0.Add(time.Hour))
	observe("os-two-gib", "suspended", 2048, t0.Add(3*time.Hour)) // suspended time is free
	got := totals("os-two-gib")
	if got[UsageKindSandboxMemoryGBSeconds] != 7200 {
		t.Errorf("memory = %d GiB-seconds, want 7200", got[UsageKindSandboxMemoryGBSeconds])
	}
	if got[UsageKindSandboxComputeSeconds] != 1000*3600 {
		t.Errorf("compute = %d, want %d: the weighted meter must not change", got[UsageKindSandboxComputeSeconds], 1000*3600)
	}

	// A terminate half an hour in closes the interval once; replaying it adds nothing.
	observe("os-gone", "running", 2048, t0)
	for range 2 {
		if err := s.TerminateMissingSandboxMeters(ctx, tenant.ID, []string{"os-two-gib"}, t0.Add(30*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if got := totals("os-gone"); got[UsageKindSandboxMemoryGBSeconds] != 2*1800 || got[UsageKindSandboxComputeSeconds] != 1000*1800 {
		t.Errorf("terminated mid-interval = %v, want memory 3600 and compute 1800000", got)
	}

	// 512 MiB sampled every second: 0.5 GiB-second each, so the carry makes two
	// whole units out of four samples instead of rounding every one down to zero.
	for i := range 5 {
		phase := "running"
		if i == 4 {
			phase = "suspended"
		}
		observe("os-carry", phase, 512, t0.Add(time.Duration(i)*time.Second))
	}
	if got := totals("os-carry"); got[UsageKindSandboxMemoryGBSeconds] != 2 {
		t.Errorf("carried memory = %d GiB-seconds, want 2", got[UsageKindSandboxMemoryGBSeconds])
	}

	// Unknown memory (a sandbox that predates the stamp): compute only, no memory row.
	observe("os-unknown", "running", 0, t0)
	observe("os-unknown", "suspended", 0, t0.Add(10*time.Minute))
	if got := totals("os-unknown"); got[UsageKindSandboxComputeSeconds] != 1000*600 || got[UsageKindSandboxMemoryGBSeconds] != 0 {
		t.Errorf("unknown memory = %v, want compute 600000 and no memory", got)
	}
}
