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

	"github.com/bex-co/bex/lego/backend/internal/testenv"
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
	if _, err := st.BlueprintCreateRetry(ctx, tenant.ID, bp.Repo, bp.Branch, bp.Path, BlueprintExistingAdopt); !errors.Is(err, ErrNotFound) {
		t.Errorf("create-retry lookup of a disconnected row = %v, want not-found", err)
	}
	inSync := BlueprintStatusInSync
	if _, err := st.UpdateBlueprint(ctx, bp.ID, tenant.ID, nil, nil, nil, nil, &inSync, nil); !errors.Is(err, ErrNotFound) {
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
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
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
			}, BlueprintSync{State: BlueprintSyncStateRunning, StartedAt: now}, "")
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

// AssertBlueprintExecution holds only while (generation, runID) own the claim
// (w8/m39): abandon and successor admission both fence it.
func TestPGAssertBlueprintExecution(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "assert")
	bp := lifecycleBlueprint(t, st, tenant, "assert")
	ctx := context.Background()
	_, run, err := st.AdmitBlueprintSyncRun(ctx, bp.ID, tenant.ID, BlueprintSync{
		State: BlueprintSyncStateRunning, StartedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AssertBlueprintExecution(ctx, bp.ID, tenant.ID, run.ExecutionGeneration, run.ID); err != nil {
		t.Fatalf("assert while owning: %v", err)
	}
	if err := st.AssertBlueprintExecution(ctx, bp.ID, tenant.ID, run.ExecutionGeneration, "bsr-other"); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("wrong run = %v, want busy", err)
	}
	if err := st.AssertBlueprintExecution(ctx, bp.ID, tenant.ID, run.ExecutionGeneration+1, run.ID); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("wrong gen = %v, want busy", err)
	}
	now := time.Now().UTC()
	if _, err := st.Pool.Exec(ctx, `UPDATE blueprint_syncs SET started_at = $2 WHERE id = $1`, run.ID, now.Add(-2*BlueprintRunRecoveryBound)); err != nil {
		t.Fatal(err)
	}
	settled, err := st.AbandonBlueprintSync(ctx, run.ID, now, BlueprintRunInterruptedReason)
	if err != nil || !settled {
		t.Fatalf("abandon = (%v, %v)", settled, err)
	}
	if err := st.AssertBlueprintExecution(ctx, bp.ID, tenant.ID, run.ExecutionGeneration, run.ID); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("assert after abandon = %v, want busy", err)
	}
	var gen int64
	var active *string
	if err := st.Pool.QueryRow(ctx, `SELECT execution_generation, active_run_id FROM blueprints WHERE id = $1`, bp.ID).Scan(&gen, &active); err != nil {
		t.Fatal(err)
	}
	if active != nil {
		t.Fatalf("active_run_id still set after abandon: %v", active)
	}
	if gen != run.ExecutionGeneration+1 {
		t.Fatalf("generation after abandon = %d, want %d (bumped)", gen, run.ExecutionGeneration+1)
	}
}

// TestPGCreateIsANewBlueprintExceptARetry is w8/m153's storage rule. Render:
// "You can create multiple Blueprints from a single YAML file. Each Blueprint
// creates and manages a completely independent set of resources." So a
// one-step create of a source another Blueprint tracks is a new row with a new
// id, and a deleted Blueprint's row is never brought back (it used to be
// revived under its old id, w8/m37 and w4/120). The one reuse is a retry: a
// create whose apply failed leaves its row, and repeating the create continues
// on it instead of leaving a second half-made Blueprint.
func TestPGCreateIsANewBlueprintExceptARetry(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "persource")
	ctx := context.Background()
	now := time.Now().UTC()
	const repo, branch, path = "example/persource", "main", "render.yaml"

	admit := func(name, mode, suffix, retryOf string) (Blueprint, BlueprintSync, error) {
		t.Helper()
		return st.AdmitBlueprintCreate(ctx, Blueprint{
			TenantID: tenant.ID, Name: name, Repo: repo, Branch: branch, Path: path,
			Manifest: "services: []", AutoSync: true, ExistingResources: mode, ResourceSuffix: suffix,
		}, BlueprintSync{State: BlueprintSyncStateRunning, StartedAt: now}, retryOf)
	}
	settle := func(bp Blueprint, run BlueprintSync, state string) {
		t.Helper()
		if _, err := st.CompleteBlueprintSync(ctx, bp.ID, tenant.ID, run.ID, run.ExecutionGeneration, state, time.Now().UTC(), nil); err != nil {
			t.Fatalf("settle %s as %s: %v", bp.Name, state, err)
		}
	}
	retryTarget := func(mode string) string {
		t.Helper()
		b, err := st.BlueprintCreateRetry(ctx, tenant.ID, repo, branch, path, mode)
		if errors.Is(err, ErrNotFound) {
			return ""
		}
		if err != nil {
			t.Fatalf("create-retry lookup: %v", err)
		}
		return b.ID
	}

	first, firstRun, err := admit("first", "", "", "")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if first.ExistingResources != BlueprintExistingAdopt || first.ResourceSuffix != "" || first.ExecutionGeneration != 1 {
		t.Fatalf("first = mode %q suffix %q generation %d, want adopt, no suffix, generation 1", first.ExistingResources, first.ResourceSuffix, first.ExecutionGeneration)
	}

	// While the first create applies, a second create of the same source is
	// the same request sent twice: busy, whether or not it names the row.
	if retryTarget(BlueprintExistingAdopt) != first.ID {
		t.Fatalf("a create still applying is not the retry target")
	}
	if _, _, err := admit("double", "", "", ""); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("second create during the first's apply = %v, want ErrBlueprintSyncBusy", err)
	}
	if _, _, err := admit("double", "", "", first.ID); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("retry of a row still applying = %v, want ErrBlueprintSyncBusy", err)
	}

	// The apply fails: repeating the create continues on the same Blueprint.
	settle(first, firstRun, BlueprintSyncStateError)
	if got := retryTarget(BlueprintExistingAdopt); got != first.ID {
		t.Fatalf("retry target after a failed create = %q, want %s", got, first.ID)
	}
	retried, retriedRun, err := admit("first again", "", "", first.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	// A retry writes what a sync writes: the row keeps its own name, which
	// may have been changed since the failed attempt.
	if retried.ID != first.ID || retried.ExecutionGeneration != first.ExecutionGeneration+1 || retried.Name != "first" ||
		!retried.AutoSync || !retried.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("retry = id %s generation %d name %q autoSync %v created %v, want %s, a bumped generation and the row's own name, setting and created_at",
			retried.ID, retried.ExecutionGeneration, retried.Name, retried.AutoSync, retried.CreatedAt, first.ID)
	}
	// A caller that did not look (or looked before the failure) is told to
	// retry rather than given a second Blueprint beside the failed one.
	settle(retried, retriedRun, BlueprintSyncStateError)
	if _, _, err := admit("blind", "", "", ""); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("a create that ignores the retry target = %v, want ErrBlueprintSyncBusy", err)
	}

	// Once a sync has succeeded the Blueprint is established, and the same
	// create is a second, independent Blueprint.
	established, establishedRun, err := admit("first", "", "", first.ID)
	if err != nil {
		t.Fatalf("third attempt: %v", err)
	}
	settle(established, establishedRun, BlueprintSyncStateSuccess)
	if got := retryTarget(BlueprintExistingAdopt); got != "" {
		t.Fatalf("retry target after a successful sync = %q, want none", got)
	}
	if _, _, err := admit("stale", "", "", first.ID); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("retry naming a Blueprint that has synced = %v, want ErrBlueprintSyncBusy", err)
	}
	second, secondRun, err := admit("second", "", "", "")
	if err != nil {
		t.Fatalf("second Blueprint from the same source: %v", err)
	}
	if second.ID == first.ID || second.ExecutionGeneration != 1 {
		t.Fatalf("second = id %s generation %d, want a new row at generation 1", second.ID, second.ExecutionGeneration)
	}
	settle(second, secondRun, BlueprintSyncStateSuccess)

	// A create_new copy carries its own suffix and never retries an adopt row.
	copyBP, copyRun, err := admit("copy", BlueprintExistingCreateNew, "ab12", "")
	if err != nil {
		t.Fatalf("create_new copy: %v", err)
	}
	if copyBP.ExistingResources != BlueprintExistingCreateNew || copyBP.ResourceSuffix != "ab12" {
		t.Fatalf("copy = mode %q suffix %q, want create_new ab12", copyBP.ExistingResources, copyBP.ResourceSuffix)
	}
	settle(copyBP, copyRun, BlueprintSyncStateError)
	if got := retryTarget(BlueprintExistingCreateNew); got != copyBP.ID {
		t.Fatalf("create_new retry target = %q, want %s", got, copyBP.ID)
	}
	if got := retryTarget(BlueprintExistingAdopt); got != "" {
		t.Fatalf("an adopt create would retry the create_new copy %q", got)
	}
	again, _, err := admit("copy", BlueprintExistingCreateNew, "zz99", copyBP.ID)
	if err != nil {
		t.Fatalf("create_new retry: %v", err)
	}
	if again.ID != copyBP.ID || again.ResourceSuffix != "ab12" {
		t.Fatalf("create_new retry = id %s suffix %q, want %s keeping its suffix ab12", again.ID, again.ResourceSuffix, copyBP.ID)
	}

	// Deleting a Blueprint leaves its row disconnected; a later create is new.
	if err := st.DisconnectBlueprint(ctx, first.ID, tenant.ID); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	// A retry that names the deleted row cannot bring it back.
	if _, _, err := admit("zombie", "", "", first.ID); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("retry naming a deleted Blueprint = %v, want ErrBlueprintSyncBusy", err)
	}
	third, _, err := admit("third", "", "", "")
	if err != nil {
		t.Fatalf("create after a delete: %v", err)
	}
	if third.ID == first.ID {
		t.Fatalf("a create after a delete reused the deleted row %s", first.ID)
	}
	var status string
	if err := st.Pool.QueryRow(ctx, `SELECT status FROM blueprints WHERE id = $1`, first.ID).Scan(&status); err != nil || status != "disconnected" {
		t.Fatalf("deleted row status = %q err=%v, want disconnected", status, err)
	}
	listed, err := st.ListBlueprints(ctx, tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	live := map[string]bool{}
	for _, b := range listed {
		live[b.ID] = true
	}
	if len(live) != 3 || !live[second.ID] || !live[copyBP.ID] || !live[third.ID] {
		t.Fatalf("live Blueprints for one source = %v, want the second, the copy and the third", live)
	}
}

// A Blueprint that has ever been planned came from Render's create. Its failed
// first sync is retried by planning again; a one-step create of the same file
// must not take its row over.
func TestPGPlannedBlueprintIsNeverACreateRetryTarget(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "plannedretry")
	ctx := context.Background()
	now := time.Now().UTC()
	const repo, commit = "example/plannedretry", "0123456789abcdef0123456789abcdef01234567"
	planned, err := st.CreatePlannedBlueprint(ctx, Blueprint{TenantID: tenant.ID, Name: "planned", Repo: repo, Branch: "main", Path: "render.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := st.InsertPlannedBlueprintSync(ctx, PlannedBlueprintSync{
		BlueprintSync: BlueprintSync{BlueprintID: planned.ID, CommitID: commit, State: BlueprintSyncStateCreated, StartedAt: now},
		Path:          "render.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	admitted, run, err := st.AdmitBlueprintSyncRun(ctx, planned.ID, tenant.ID, BlueprintSync{ID: plan.ID, CommitID: commit, State: BlueprintSyncStateRunning, StartedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	failed, err := st.CompleteBlueprintSync(ctx, admitted.ID, tenant.ID, run.ID, run.ExecutionGeneration, BlueprintSyncStateError, time.Now().UTC(), nil)
	if err != nil || failed.Status != BlueprintStatusPaused && failed.Status != BlueprintStatusError {
		t.Fatalf("the approved plan's failed sync left %+v (err %v), want the Blueprint failed", failed, err)
	}
	if got, err := st.BlueprintCreateRetry(ctx, tenant.ID, repo, "main", "render.yaml", BlueprintExistingAdopt); !errors.Is(err, ErrNotFound) {
		t.Fatalf("create-retry lookup = %s (err %v), want ErrNotFound for a planned Blueprint", got.ID, err)
	}
	created, _, err := st.AdmitBlueprintCreate(ctx, Blueprint{TenantID: tenant.ID, Name: "one-step", Repo: repo, Branch: "main", Path: "render.yaml",
		Manifest: "services: []", AutoSync: true}, BlueprintSync{State: BlueprintSyncStateRunning, StartedAt: now}, "")
	if err != nil || created.ID == planned.ID {
		t.Fatalf("one-step create beside the failed planned Blueprint = %s (err %v), want a new Blueprint", created.ID, err)
	}
	var name string
	if err := st.Pool.QueryRow(ctx, `SELECT name FROM blueprints WHERE id = $1`, planned.ID).Scan(&name); err != nil || name != "planned" {
		t.Fatalf("the planned Blueprint's name = %q (err %v), want it untouched", name, err)
	}
}

// TestPGDeployRegistrationPicksTheUnsuffixedBlueprint: a direct deploy applies
// the manifest's own names, so with several Blueprints on one source it
// refreshes the oldest live one that manages unsuffixed resources and leaves
// create_new copies alone (w8/m153; the unique key used to make the choice).
func TestPGDeployRegistrationPicksTheUnsuffixedBlueprint(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "register")
	ctx := context.Background()
	const repo = "example/register"
	register := func(manifest string) (Blueprint, error) {
		return st.UpsertBlueprint(ctx, Blueprint{TenantID: tenant.ID, Name: "register", Repo: repo, Branch: "main",
			AutoSync: true, Manifest: manifest, Status: BlueprintStatusInSync})
	}
	manifestOf := func(id string) string {
		t.Helper()
		var m string
		if err := st.Pool.QueryRow(ctx, `SELECT manifest FROM blueprints WHERE id = $1`, id).Scan(&m); err != nil {
			t.Fatal(err)
		}
		return m
	}

	copyBP, err := st.CreatePlannedBlueprint(ctx, Blueprint{TenantID: tenant.ID, Name: "copy", Repo: repo, Branch: "main", Path: "render.yaml",
		ExistingResources: BlueprintExistingCreateNew})
	if err != nil {
		t.Fatal(err)
	}
	// Only a create_new copy exists: the deploy registers its own Blueprint.
	first, err := register("v1")
	if err != nil || first.ID == copyBP.ID || first.ResourceSuffix != "" {
		t.Fatalf("first registration = %+v err=%v, want a new unsuffixed row beside the copy", first, err)
	}
	second, err := register("v2")
	if err != nil || second.ID != first.ID || second.Manifest != "v2" {
		t.Fatalf("second registration = %+v err=%v, want %s refreshed to v2", second, err, first.ID)
	}
	if got := manifestOf(copyBP.ID); got != "" {
		t.Fatalf("the create_new copy's manifest = %q, want it untouched", got)
	}
	// Racing registrations of a new source insert one row, not one each.
	const racers = 8
	errs := make(chan error, racers)
	for i := 0; i < racers; i++ {
		go func() {
			_, err := st.UpsertBlueprint(ctx, Blueprint{TenantID: tenant.ID, Name: "race", Repo: repo + "-race", Branch: "main",
				AutoSync: true, Manifest: "race", Status: BlueprintStatusInSync})
			errs <- err
		}()
	}
	for i := 0; i < racers; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("racing registration: %v", err)
		}
	}
	var rows int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM blueprints WHERE tenant_id = $1 AND repo = $2`, tenant.ID, repo+"-race").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("racing registrations left %d rows (err %v), want 1", rows, err)
	}
	// The deploy-registered Blueprint is deleted: a deploy does not bring a
	// Blueprint for the source back.
	if err := st.DisconnectBlueprint(ctx, first.ID, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := register("v3"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("registration after the delete = %v, want ErrNotFound", err)
	}
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM blueprints WHERE tenant_id = $1 AND repo = $2 AND status != 'disconnected'`, tenant.ID, repo).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("live rows after the refused registration = %d (err %v), want only the copy", rows, err)
	}
}

// TestPGBlueprintsPerSourceMigration: 0229 drops the one-per-source key.
func TestPGBlueprintsPerSourceMigration(t *testing.T) {
	st := openLifecyclePG(t)
	ctx := context.Background()
	var constraints, indexes int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint
		WHERE conrelid = 'blueprints'::regclass AND contype = 'u'`).Scan(&constraints); err != nil {
		t.Fatal(err)
	}
	if constraints != 0 {
		t.Fatalf("blueprints still has %d unique constraint(s); 0229 removes the (tenant_id, repo, branch) key", constraints)
	}
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes
		WHERE tablename = 'blueprints' AND indexname = 'blueprints_tenant_repo_branch'`).Scan(&indexes); err != nil {
		t.Fatal(err)
	}
	if indexes != 1 {
		t.Fatalf("the source lookup index is missing (found %d)", indexes)
	}
}
