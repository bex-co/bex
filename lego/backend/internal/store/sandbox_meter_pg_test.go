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
)

// TestSandboxMeterPG proves the money-path invariants against Postgres: each
// phase sample advances one durable cursor, retries do not double count,
// suspended time is free, intervals split across hours, a disappeared sandbox
// closes its final interval, and monthly compaction remains transparent.
func TestSandboxMeterPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
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
