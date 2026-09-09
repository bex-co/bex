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
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"os"
)

// blueprint_lifecycle_pg_test.go (w8/m37 t008) proves the persisted execution
// boundary against real Postgres: absence of disconnected rows, atomic
// cross-connection admission, generation-fenced completion and disconnect,
// and bounded abandoned-run recovery. The fake-backed unit tests in
// internal/apps exercise the service mapping; these prove the SQL itself
// serializes and fences. Requires BEX_TEST_DB_URI.

func openLifecyclePG(t *testing.T) *PGStore {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
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
	return NewPGStore(pool)
}

func lifecycleTenant(t *testing.T, st *PGStore, tag string) Tenant {
	t.Helper()
	stamp := fmt.Sprintf("%s-%d", tag, time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(context.Background(), "bplife-test-"+stamp, PlanHobby, "bplife-owner-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	return tenant
}

func lifecycleBlueprint(t *testing.T, st *PGStore, tenant Tenant, name string) Blueprint {
	t.Helper()
	bp, err := st.UpsertBlueprint(context.Background(), Blueprint{
		TenantID: tenant.ID, Name: name, Repo: "example/" + name, Branch: "main",
		Manifest: "services: []", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	return bp
}

func admitRun(t *testing.T, st *PGStore, bp Blueprint, started time.Time) BlueprintSync {
	t.Helper()
	b, run, err := st.AdmitBlueprintSyncRun(context.Background(), bp.ID, bp.TenantID, BlueprintSync{
		CommitID: "cafef00d", State: BlueprintSyncStateRunning, StartedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	if b.ActiveRunID != run.ID || b.ExecutionGeneration <= 0 {
		t.Fatalf("admit did not claim: %+v / %+v", b, run)
	}
	return run
}

// The fencing columns exist with rollout-safe defaults.
func TestPGLifecycleMigrationColumns(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "cols")
	bp := lifecycleBlueprint(t, st, tenant, "cols")
	var gen int64
	var active *string
	var rgen int64
	ctx := context.Background()
	if err := st.Pool.QueryRow(ctx, `SELECT execution_generation, active_run_id FROM blueprints WHERE id = $1`, bp.ID).Scan(&gen, &active); err != nil {
		t.Fatal(err)
	}
	if gen != 1 || active != nil {
		t.Fatalf("blueprint defaults = (%d, %v), want (1, NULL)", gen, active)
	}
	run, err := st.InsertBlueprintSync(ctx, BlueprintSync{BlueprintID: bp.ID, State: BlueprintSyncStateRunning, StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Pool.QueryRow(ctx, `SELECT execution_generation FROM blueprint_syncs WHERE id = $1`, run.ID).Scan(&rgen); err != nil {
		t.Fatal(err)
	}
	if rgen != 0 && rgen != 1 {
		t.Fatalf("run generation = %d, want an explicit small value", rgen)
	}
}

// Disconnected rows read as absent on every ordinary path, and the deploy
// auto-register upsert cannot revive one.
func TestPGDisconnectedReadsAsAbsent(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "absent")
	bp := lifecycleBlueprint(t, st, tenant, "gone")
	ctx := context.Background()
	if err := st.DisconnectBlueprint(ctx, bp.ID, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetBlueprint(ctx, bp.ID, tenant.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get disconnected = %v, want not-found", err)
	}
	if _, err := st.GetBlueprintByRepo(ctx, tenant.ID, bp.Repo, bp.Branch); !errors.Is(err, ErrNotFound) {
		t.Errorf("get-by-repo disconnected = %v, want not-found", err)
	}
	inSync := BlueprintStatusInSync
	if _, err := st.UpdateBlueprint(ctx, bp.ID, tenant.ID, nil, nil, nil, &inSync, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("update disconnected = %v, want not-found", err)
	}
	if err := st.DisconnectBlueprint(ctx, bp.ID, tenant.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("second disconnect = %v, want not-found", err)
	}
	listed, err := st.ListBlueprints(ctx, tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range listed {
		if b.ID == bp.ID {
			t.Fatalf("disconnected row listed")
		}
	}
	// Auto-register over the same source neither revives nor inserts.
	if _, err := st.UpsertBlueprint(ctx, Blueprint{TenantID: tenant.ID, Name: "x", Repo: bp.Repo, Branch: bp.Branch, Manifest: "services: []", Status: BlueprintStatusInSync}); !errors.Is(err, ErrNotFound) {
		t.Errorf("upsert over disconnected = %v, want refusal", err)
	}
	var status string
	if err := st.Pool.QueryRow(ctx, `SELECT status FROM blueprints WHERE id = $1`, bp.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "disconnected" {
		t.Fatalf("row status = %q after refused upsert, want disconnected", status)
	}
}

// Independent API instances (separate pools) racing one Blueprint admit
// exactly one apply; losers take the busy path without a second run row.
func TestPGAdmissionRaceAdmitsOne(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "race")
	bp := lifecycleBlueprint(t, st, tenant, "race")
	const racers = 8
	var wg sync.WaitGroup
	wins := make(chan error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			pool, err := pgxpool.New(ctx, uri)
			if err != nil {
				wins <- err
				return
			}
			defer pool.Close()
			_, _, err = NewPGStore(pool).AdmitBlueprintSyncRun(ctx, bp.ID, tenant.ID, BlueprintSync{
				CommitID: "race", State: BlueprintSyncStateRunning, StartedAt: time.Now().UTC(),
			})
			wins <- err
		}()
	}
	wg.Wait()
	close(wins)
	var succeeded, busy int
	for err := range wins {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrBlueprintSyncBusy):
			busy++
		default:
			t.Fatalf("racer error = %v, want success or busy", err)
		}
	}
	if succeeded != 1 || busy != racers-1 {
		t.Fatalf("race: %d wins + %d busy, want 1 + %d", succeeded, busy, racers-1)
	}
	var running int
	if err := st.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM blueprint_syncs WHERE blueprint_id = $1 AND state = 'running'`, bp.ID).Scan(&running); err != nil {
		t.Fatal(err)
	}
	if running != 1 {
		t.Fatalf("running runs = %d, want exactly the winner's", running)
	}
}

// A stale completion (wrong generation, terminal run, disconnected row)
// cannot overwrite; the winner's completion projects status honestly.
func TestPGCompletionFencing(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "fence")
	bp := lifecycleBlueprint(t, st, tenant, "fence")
	ctx := context.Background()
	now := time.Now().UTC()

	run := admitRun(t, st, bp, now)
	completed, err := st.CompleteBlueprintSync(ctx, bp.ID, tenant.ID, run.ID, run.ExecutionGeneration, BlueprintSyncStateSuccess, now, nil)
	if err != nil {
		t.Fatalf("winner completion: %v", err)
	}
	// The fixture leaves auto-sync off, so success projects paused (the full
	// matrix lives in TestPGCompleteProjection).
	if completed.Status != BlueprintStatusPaused || completed.ActiveRunID != "" {
		t.Fatalf("completed row = %+v, want paused with released claim", completed)
	}
	// Terminal resurrection: completing the same run again is fenced.
	if _, err := st.CompleteBlueprintSync(ctx, bp.ID, tenant.ID, run.ID, run.ExecutionGeneration, BlueprintSyncStateSuccess, now, nil); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("second completion = %v, want busy", err)
	}
	// A newer admission fences the older generation's late write.
	run2 := admitRun(t, st, bp, now)
	if _, err := st.CompleteBlueprintSync(ctx, bp.ID, tenant.ID, run.ID, 1, BlueprintSyncStateSuccess, now, nil); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("cross-generation completion = %v, want busy", err)
	}
	msg := "boom"
	if _, err := st.CompleteBlueprintSync(ctx, bp.ID, tenant.ID, run2.ID, run2.ExecutionGeneration, BlueprintSyncStateError, now, &msg); err != nil {
		t.Fatalf("error completion: %v", err)
	}
	// Disconnect fences every outstanding generation: age the new claim so the
	// disconnect settles it inline, then prove its late completion is fenced.
	run3 := admitRun(t, st, bp, now)
	if _, err := st.Pool.Exec(ctx, `UPDATE blueprint_syncs SET started_at = $2 WHERE id = $1`, run3.ID, now.Add(-2*BlueprintRunRecoveryBound)); err != nil {
		t.Fatal(err)
	}
	if err := st.DisconnectBlueprint(ctx, bp.ID, tenant.ID); err != nil {
		t.Fatalf("disconnect with stale claim: %v", err)
	}
	if _, err := st.CompleteBlueprintSync(ctx, bp.ID, tenant.ID, run3.ID, run3.ExecutionGeneration, BlueprintSyncStateSuccess, now, nil); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("post-disconnect completion = %v, want busy", err)
	}
}

// Disconnect refuses while a fresh apply owns the claim, and settles a stale
// claim inline so the busy window after process loss is bounded.
func TestPGDisconnectCoordination(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "disc")
	bp := lifecycleBlueprint(t, st, tenant, "disc")
	ctx := context.Background()
	now := time.Now().UTC()

	fresh := admitRun(t, st, bp, now)
	if err := st.DisconnectBlueprint(ctx, bp.ID, tenant.ID); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("disconnect during fresh apply = %v, want busy", err)
	}
	// Age the claim past the bound: the same disconnect now settles it inline.
	if _, err := st.Pool.Exec(ctx, `UPDATE blueprint_syncs SET started_at = $2 WHERE id = $1`, fresh.ID, now.Add(-2*BlueprintRunRecoveryBound)); err != nil {
		t.Fatal(err)
	}
	if err := st.DisconnectBlueprint(ctx, bp.ID, tenant.ID); err != nil {
		t.Fatalf("disconnect with stale claim: %v", err)
	}
	var status, state string
	var gen int64
	var active *string
	if err := st.Pool.QueryRow(ctx, `SELECT status, execution_generation, active_run_id FROM blueprints WHERE id = $1`, bp.ID).Scan(&status, &gen, &active); err != nil {
		t.Fatal(err)
	}
	if status != "disconnected" || active != nil || gen < 2 {
		t.Fatalf("row after disconnect = (%q, gen %d, claim %v), want (disconnected, ≥2, NULL)", status, gen, active)
	}
	if err := st.Pool.QueryRow(ctx, `SELECT state FROM blueprint_syncs WHERE id = $1`, fresh.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != BlueprintSyncStateError {
		t.Fatalf("stale claim run state = %q, want error", state)
	}
}

// Abandonment settles stale owned claims (flipping their Blueprint to error),
// settles orphaned rows without touching foreign state, and never lists a
// demonstrably live run.
func TestPGAbandonment(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "abandon")
	ctx := context.Background()
	now := time.Now().UTC()
	old := now.Add(-2 * BlueprintRunRecoveryBound)
	recent := now.Add(-time.Minute)

	oldBP := lifecycleBlueprint(t, st, tenant, "old")
	oldRun := admitRun(t, st, oldBP, old)
	liveBP := lifecycleBlueprint(t, st, tenant, "live")
	liveRun := admitRun(t, st, liveBP, recent)

	// A stale running run from a newer generation the sweeper must preserve:
	// simulate a newer admission by bumping the row out from under the run.
	newerBP := lifecycleBlueprint(t, st, tenant, "newer")
	newerRun := admitRun(t, st, newerBP, old)
	if _, err := st.Pool.Exec(ctx, `UPDATE blueprints SET execution_generation = execution_generation + 1, active_run_id = 'other' WHERE id = $1`, newerBP.ID); err != nil {
		t.Fatal(err)
	}

	due, err := st.ListAbandonedBlueprintSyncs(ctx, now.Add(-BlueprintRunRecoveryBound), 100)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range due {
		seen[d.RunID] = true
	}
	if !seen[oldRun.ID] || !seen[newerRun.ID] {
		t.Fatalf("sweep missed stale runs: %v", seen)
	}
	if seen[liveRun.ID] {
		t.Fatalf("sweep listed a demonstrably live run")
	}

	settled, err := st.AbandonBlueprintSync(ctx, oldRun.ID, now, BlueprintRunInterruptedReason)
	if err != nil || !settled {
		t.Fatalf("abandon old = (%v, %v), want (true, nil)", settled, err)
	}
	var state string
	var bstatus string
	var active *string
	if err := st.Pool.QueryRow(ctx, `SELECT state FROM blueprint_syncs WHERE id = $1`, oldRun.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err := st.Pool.QueryRow(ctx, `SELECT status, active_run_id FROM blueprints WHERE id = $1`, oldBP.ID).Scan(&bstatus, &active); err != nil {
		t.Fatal(err)
	}
	if state != BlueprintSyncStateError || bstatus != BlueprintStatusError || active != nil {
		t.Fatalf("abandoned pair = (%q, %q, %v), want (error, error, NULL)", state, bstatus, active)
	}
	// The newer generation's row is preserved; only its stale run settles.
	settled, err = st.AbandonBlueprintSync(ctx, newerRun.ID, now, BlueprintRunInterruptedReason)
	if err != nil || !settled {
		t.Fatalf("abandon newer-gen run = (%v, %v), want (true, nil)", settled, err)
	}
	var claim string
	if err := st.Pool.QueryRow(ctx, `SELECT COALESCE(active_run_id, '') FROM blueprints WHERE id = $1`, newerBP.ID).Scan(&claim); err != nil {
		t.Fatal(err)
	}
	if claim != "other" {
		t.Fatalf("newer claim = %q, want preserved 'other'", claim)
	}
	// Settling twice is an idempotent no-op.
	settled, err = st.AbandonBlueprintSync(ctx, oldRun.ID, now, BlueprintRunInterruptedReason)
	if err != nil || settled {
		t.Fatalf("second abandon = (%v, %v), want (false, nil)", settled, err)
	}
}

// Terminal status projects from the CURRENT row: automation disabled at
// completion stays disabled.
func TestPGCompleteProjection(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "proj")
	ctx := context.Background()
	now := time.Now().UTC()
	for _, tc := range []struct {
		name     string
		autoSync bool
		runState string
		want     string
	}{
		{"success with automation", true, BlueprintSyncStateSuccess, BlueprintStatusInSync},
		{"success without automation", false, BlueprintSyncStateSuccess, BlueprintStatusPaused},
		{"failure with automation", true, BlueprintSyncStateError, BlueprintStatusError},
		{"failure without automation", false, BlueprintSyncStateError, BlueprintStatusPaused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// AdmitBlueprintCreate carries the requested automation flag.
			bp, run, err := st.AdmitBlueprintCreate(ctx, Blueprint{
				TenantID: tenant.ID, Name: tc.name, Repo: "example/" + tc.name, Branch: "main",
				Manifest: "services: []", AutoSync: tc.autoSync,
			}, BlueprintSync{State: BlueprintSyncStateRunning, StartedAt: now})
			if err != nil {
				t.Fatal(err)
			}
			msg := "failure detail"
			completed, err := st.CompleteBlueprintSync(ctx, bp.ID, tenant.ID, run.ID, run.ExecutionGeneration, tc.runState, now, &msg)
			if err != nil {
				t.Fatal(err)
			}
			if completed.Status != tc.want {
				t.Fatalf("status = %q, want %q", completed.Status, tc.want)
			}
			if completed.AutoSync != tc.autoSync {
				t.Fatalf("autoSync flipped to %v by completion", completed.AutoSync)
			}
		})
	}
}

// A preflight failure settles the admitted run while leaving status and
// settings exactly as they were.
func TestPGFailAdmittedPreservesStatus(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "failadm")
	bp := lifecycleBlueprint(t, st, tenant, "failadm")
	ctx := context.Background()
	now := time.Now().UTC()
	run := admitRun(t, st, bp, now)
	msg := "unsupported service type"
	if err := st.FailAdmittedSync(ctx, bp.ID, tenant.ID, run.ID, run.ExecutionGeneration, now, &msg); err != nil {
		t.Fatal(err)
	}
	var status, active, name string
	if err := st.Pool.QueryRow(ctx, `SELECT status, COALESCE(active_run_id, ''), name FROM blueprints WHERE id = $1`, bp.ID).Scan(&status, &active, &name); err != nil {
		t.Fatal(err)
	}
	if status != "active" || active != "" || name != "failadm" {
		t.Fatalf("row = (%q, %q, %q), want status/settings preserved with released claim", status, active, name)
	}
	var state string
	if err := st.Pool.QueryRow(ctx, `SELECT state FROM blueprint_syncs WHERE id = $1`, run.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != BlueprintSyncStateError {
		t.Fatalf("run state = %q, want error", state)
	}
}

// A rollout-era row (defaulted generation, unclaimed, stuck syncing with a
// stale running run) is recoverable without touching live work.
func TestPGStarterLegacyRow(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "legacy")
	bp := lifecycleBlueprint(t, st, tenant, "legacy")
	ctx := context.Background()
	old := time.Now().UTC().Add(-2 * BlueprintRunRecoveryBound)
	run, err := st.InsertBlueprintSync(ctx, BlueprintSync{BlueprintID: bp.ID, State: BlueprintSyncStateRunning, StartedAt: old})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE blueprints SET status = 'syncing' WHERE id = $1`, bp.ID); err != nil {
		t.Fatal(err)
	}
	due, err := st.ListAbandonedBlueprintSyncs(ctx, time.Now().UTC().Add(-BlueprintRunRecoveryBound), 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range due {
		if d.RunID == run.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("legacy stranded run not listed")
	}
	settled, err := st.AbandonBlueprintSync(ctx, run.ID, time.Now().UTC(), BlueprintRunInterruptedReason)
	if err != nil || !settled {
		t.Fatalf("abandon legacy = (%v, %v), want (true, nil)", settled, err)
	}
	var status string
	if err := st.Pool.QueryRow(ctx, `SELECT status FROM blueprints WHERE id = $1`, bp.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != BlueprintStatusError {
		t.Fatalf("legacy blueprint status = %q, want error", status)
	}
}

// The recovery sweep is bounded per tick and oldest-first.
func TestPGListAbandonedBounded(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "bounded")
	bp := lifecycleBlueprint(t, st, tenant, "bounded")
	ctx := context.Background()
	base := time.Now().UTC().Add(-2 * BlueprintRunRecoveryBound)
	for i := 0; i < 105; i++ {
		if _, err := st.InsertBlueprintSync(ctx, BlueprintSync{
			BlueprintID: bp.ID, State: BlueprintSyncStateRunning,
			StartedAt: base.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatal(err)
		}
	}
	due, err := st.ListAbandonedBlueprintSyncs(ctx, time.Now().UTC().Add(-BlueprintRunRecoveryBound), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 100 {
		t.Fatalf("sweep returned %d rows, want the bounded 100", len(due))
	}
	for i := 1; i < len(due); i++ {
		if due[i].StartedAt.Before(due[i-1].StartedAt) {
			t.Fatalf("sweep not oldest-first at index %d", i)
		}
	}
}
