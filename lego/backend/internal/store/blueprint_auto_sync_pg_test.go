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

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPGStoreBlueprintAutoSyncIntents covers w8/m38 durable enqueue, UNIQUE
// dedupe, claim, and complete/fail against real Postgres when available.
func TestPGStoreBlueprintAutoSyncIntents(t *testing.T) {
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
	defer pool.Close()
	st := NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "asi-test-"+stamp, PlanHobby, "asi-owner-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })

	bp, err := st.UpsertBlueprint(ctx, Blueprint{
		TenantID: tenant.ID, Name: "asi", Repo: "https://github.com/acme/asi",
		Branch: "main", Path: "render.yaml", AutoSync: true,
		Manifest: "services: []", Status: BlueprintStatusInSync,
	})
	if err != nil {
		t.Fatal(err)
	}

	listed, err := st.ListAutoSyncBlueprints(ctx, "main", tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != bp.ID {
		t.Fatalf("ListAutoSyncBlueprints = %+v, want [%s]", listed, bp.ID)
	}
	foreign, err := st.ListAutoSyncBlueprints(ctx, "main", "tea-missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(foreign) != 0 {
		t.Fatalf("scoped list returned %d rows", len(foreign))
	}

	intent := BlueprintAutoSyncIntent{
		TenantID: tenant.ID, BlueprintID: bp.ID,
		DeliveryDigest: "digest-" + stamp,
		CommitSHA:      strings.Repeat("a", 40),
		Path:           "render.yaml",
	}
	inserted, err := st.EnqueueBlueprintAutoSyncIntent(ctx, intent)
	if err != nil || !inserted {
		t.Fatalf("enqueue: inserted=%v err=%v", inserted, err)
	}
	inserted, err = st.EnqueueBlueprintAutoSyncIntent(ctx, intent)
	if err != nil || inserted {
		t.Fatalf("dedupe: inserted=%v err=%v, want false", inserted, err)
	}

	claimed, err := st.ClaimBlueprintAutoSyncIntents(ctx, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].BlueprintID != bp.ID {
		t.Fatalf("claim = %+v", claimed)
	}
	if err := st.CompleteBlueprintAutoSyncIntent(ctx, claimed[0].ID, claimed[0].ClaimGeneration, ""); err != nil {
		t.Fatal(err)
	}
	again, err := st.ClaimBlueprintAutoSyncIntents(ctx, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("claimed completed intent: %+v", again)
	}
}

func autoSyncPGFixture(t *testing.T) (*PGStore, Blueprint) {
	t.Helper()
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "autosync")
	bp, err := st.UpsertBlueprint(context.Background(), Blueprint{
		TenantID: tenant.ID, Name: "automatic", Repo: "https://github.com/acme/automatic", Branch: "main",
		Path: "render.yaml", AutoSync: true, Manifest: "services: []", Status: BlueprintStatusInSync,
	})
	if err != nil {
		t.Fatal(err)
	}
	return st, bp
}

func enqueueAutoSyncPG(t *testing.T, st *PGStore, bp Blueprint, digest, commit string) {
	t.Helper()
	if inserted, err := st.EnqueueBlueprintAutoSyncIntent(context.Background(), BlueprintAutoSyncIntent{
		TenantID: bp.TenantID, BlueprintID: bp.ID, DeliveryDigest: digest, CommitSHA: commit, Path: bp.Path,
	}); err != nil || !inserted {
		t.Fatalf("enqueue: inserted=%v err=%v", inserted, err)
	}
}

func claimAutoSyncPG(t *testing.T, st *PGStore) BlueprintAutoSyncIntent {
	t.Helper()
	claimed, err := st.ClaimBlueprintAutoSyncIntents(context.Background(), 10, time.Minute)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim = %+v err=%v, want one", claimed, err)
	}
	return claimed[0]
}

func autoSyncRunPG() BlueprintSync {
	return BlueprintSync{CommitID: strings.Repeat("b", 40), State: BlueprintSyncStateRunning, StartedAt: time.Now().UTC()}
}

func TestPGBlueprintAutoSyncClaimsSerializePerBlueprint(t *testing.T) {
	st, bp := autoSyncPGFixture(t)
	ctx := context.Background()
	other, err := st.UpsertBlueprint(ctx, Blueprint{
		TenantID: bp.TenantID, Name: "other", Repo: "https://github.com/acme/other", Branch: "main",
		Path: "render.yaml", AutoSync: true, Manifest: "services: []", Status: BlueprintStatusInSync,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []Blueprint{bp, other} {
		for i := range 3 {
			enqueueAutoSyncPG(t, st, b, fmt.Sprintf("delivery-%s-%d", b.ID, i), fmt.Sprintf("commit-%d", i))
		}
	}
	type result struct {
		intents []BlueprintAutoSyncIntent
		err     error
	}
	const workers = 12
	results := make(chan result, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			<-start
			intents, err := st.ClaimBlueprintAutoSyncIntents(ctx, 10, time.Minute)
			results <- result{intents, err}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	byBlueprint := map[string]BlueprintAutoSyncIntent{}
	for r := range results {
		if r.err != nil {
			t.Fatal(r.err)
		}
		for _, intent := range r.intents {
			if prior, exists := byBlueprint[intent.BlueprintID]; exists {
				t.Fatalf("two fresh claims for one Blueprint: %+v / %+v", prior, intent)
			}
			if intent.ClaimGeneration != 1 || intent.CommitSHA != "commit-0" {
				t.Fatalf("first claim = %+v", intent)
			}
			byBlueprint[intent.BlueprintID] = intent
		}
	}
	if len(byBlueprint) != 2 {
		t.Fatalf("independent Blueprints did not each progress: %+v", byBlueprint)
	}
	first := byBlueprint[bp.ID]
	if err := st.CompleteBlueprintAutoSyncIntent(ctx, first.ID, first.ClaimGeneration, strings.Repeat("é", 400)); err != nil {
		t.Fatal(err)
	}
	var reason string
	if err := st.Pool.QueryRow(ctx, `SELECT error_message FROM blueprint_auto_sync_intents WHERE id = $1`, first.ID).Scan(&reason); err != nil || len(reason) != 500 {
		t.Fatalf("bounded completion reason: bytes=%d err=%v", len(reason), err)
	}
	next := claimAutoSyncPG(t, st)
	if next.BlueprintID != bp.ID || next.CommitSHA != "commit-1" || next.ID == first.ID {
		t.Fatalf("next serialized claim = %+v", next)
	}
}

func TestPGBlueprintAutoSyncReclaimFencesEveryExecutionBoundary(t *testing.T) {
	st, bp := autoSyncPGFixture(t)
	ctx := context.Background()
	acceptedCommit := strings.Repeat("a", 40)
	enqueueAutoSyncPG(t, st, bp, "reclaim", acceptedCommit)
	first := claimAutoSyncPG(t, st)
	_, oldRun, err := st.AdmitBlueprintAutoSyncRun(ctx, first, bp, autoSyncRunPG())
	if err != nil {
		t.Fatal(err)
	}
	if oldRun.AutoIntentID != first.ID || oldRun.AutoClaimGeneration != first.ClaimGeneration || oldRun.AutoSourceRepo != bp.Repo || oldRun.CommitID == acceptedCommit {
		t.Fatalf("automatic source attribution = %+v", oldRun)
	}
	if _, err := st.StageBlueprintManifest(ctx, bp.ID, bp.TenantID, oldRun.ExecutionGeneration, oldRun.ID, "services: [old]"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE blueprint_auto_sync_intents SET claimed_at = now() - interval '1 hour' WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	second := claimAutoSyncPG(t, st)
	if second.ID != first.ID || second.ClaimGeneration != first.ClaimGeneration+1 || second.CommitSHA != acceptedCommit {
		t.Fatalf("reclaimed intent lost provenance or token: %+v", second)
	}
	for name, err := range map[string]error{
		"intent assert":    st.AssertBlueprintAutoSyncIntent(ctx, first.ID, first.ClaimGeneration),
		"intent complete":  st.CompleteBlueprintAutoSyncIntent(ctx, first.ID, first.ClaimGeneration, "stale success"),
		"intent fail":      st.FailBlueprintAutoSyncIntent(ctx, first.ID, first.ClaimGeneration, "stale failure"),
		"execution assert": st.AssertBlueprintExecution(ctx, bp.ID, bp.TenantID, oldRun.ExecutionGeneration, oldRun.ID),
	} {
		if !errors.Is(err, ErrBlueprintSyncBusy) {
			t.Errorf("stale %s = %v, want busy", name, err)
		}
	}
	if _, _, err := st.AdmitBlueprintAutoSyncRun(ctx, first, bp, autoSyncRunPG()); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("stale admission = %v", err)
	}
	if _, err := st.StageBlueprintManifest(ctx, bp.ID, bp.TenantID, oldRun.ExecutionGeneration, oldRun.ID, "services: [stale]"); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("stale stage = %v", err)
	}
	if _, err := st.CompleteBlueprintSync(ctx, bp.ID, bp.TenantID, oldRun.ID, oldRun.ExecutionGeneration, BlueprintSyncStateSuccess, time.Now(), nil); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("stale completion = %v", err)
	}
	_, current, err := st.AdmitBlueprintAutoSyncRun(ctx, second, bp, autoSyncRunPG())
	if err != nil {
		t.Fatalf("successor admission waited for stale-run recovery: %v", err)
	}
	if err := st.AssertBlueprintExecution(ctx, bp.ID, bp.TenantID, current.ExecutionGeneration, current.ID); err != nil {
		t.Fatal(err)
	}
	history, err := st.ListBlueprintSyncs(ctx, bp.ID, "", 10)
	if err != nil || len(history) != 2 || history[1].State != BlueprintSyncStateError || history[1].ErrorMessage == nil || *history[1].ErrorMessage != BlueprintAutoSyncReclaimedReason {
		t.Fatalf("reclaimed history = %+v err=%v", history, err)
	}
	if _, err := st.CompleteBlueprintSync(ctx, bp.ID, bp.TenantID, current.ID, current.ExecutionGeneration, BlueprintSyncStateSuccess, time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	if err := st.CompleteBlueprintAutoSyncIntent(ctx, second.ID, second.ClaimGeneration, "Applied current branch commit "+current.CommitID); err != nil {
		t.Fatal(err)
	}
}

func TestPGBlueprintAutoSyncAdmissionAndApplyRecheckPolicyAndSource(t *testing.T) {
	for _, change := range []string{"auto-sync", "path", "repository", "branch"} {
		t.Run(change, func(t *testing.T) {
			st, bp := autoSyncPGFixture(t)
			ctx := context.Background()
			enqueueAutoSyncPG(t, st, bp, "policy", strings.Repeat("a", 40))
			intent := claimAutoSyncPG(t, st)
			_, run, err := st.AdmitBlueprintAutoSyncRun(ctx, intent, bp, autoSyncRunPG())
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "auto-sync":
				disabled := false
				_, err = st.UpdateBlueprint(ctx, bp.ID, bp.TenantID, nil, &disabled, nil, nil, nil)
			case "path":
				path := "other.yaml"
				_, err = st.UpdateBlueprint(ctx, bp.ID, bp.TenantID, nil, nil, &path, nil, nil)
			case "repository":
				_, err = st.Pool.Exec(ctx, `UPDATE blueprints SET repo = 'https://github.com/acme/other' WHERE id = $1`, bp.ID)
			case "branch":
				_, err = st.Pool.Exec(ctx, `UPDATE blueprints SET branch = 'other' WHERE id = $1`, bp.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := st.AssertBlueprintExecution(ctx, bp.ID, bp.TenantID, run.ExecutionGeneration, run.ID); !errors.Is(err, ErrBlueprintSyncBusy) {
				t.Fatalf("policy/source mutation boundary = %v", err)
			}
			if _, err := st.StageBlueprintManifest(ctx, bp.ID, bp.TenantID, run.ExecutionGeneration, run.ID, "services: [forbidden]"); !errors.Is(err, ErrBlueprintSyncBusy) {
				t.Fatalf("policy/source stage = %v", err)
			}
			if _, err := st.CompleteBlueprintSync(ctx, bp.ID, bp.TenantID, run.ID, run.ExecutionGeneration, BlueprintSyncStateSuccess, time.Now(), nil); !errors.Is(err, ErrBlueprintSyncBusy) {
				t.Fatalf("policy/source completion = %v", err)
			}
			message := "source or policy changed"
			if err := st.FailAdmittedSync(ctx, bp.ID, bp.TenantID, run.ID, run.ExecutionGeneration, time.Now(), &message); err != nil {
				t.Fatalf("refused run could not release its own claim: %v", err)
			}
			if _, _, err := st.AdmitBlueprintAutoSyncRun(ctx, intent, bp, autoSyncRunPG()); !errors.Is(err, ErrBlueprintSyncBusy) {
				t.Fatalf("stale source admission = %v", err)
			}
		})
	}
}

func TestPGBlueprintAutoSyncReclaimDoesNotWaitForCompletionRunLock(t *testing.T) {
	st, bp := autoSyncPGFixture(t)
	ctx := context.Background()
	enqueueAutoSyncPG(t, st, bp, "locked-completion", strings.Repeat("a", 40))
	first := claimAutoSyncPG(t, st)
	_, oldRun, err := st.AdmitBlueprintAutoSyncRun(ctx, first, bp, autoSyncRunPG())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE blueprint_auto_sync_intents SET claimed_at = now() - interval '1 hour' WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	// Completion takes its run lock before the Blueprint. Reclaim must not
	// wait on that lock while holding the Blueprint and creating a cycle.
	tx, err := st.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var locked string
	if err := tx.QueryRow(ctx, `SELECT id FROM blueprint_syncs WHERE id = $1 FOR UPDATE`, oldRun.ID).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	claimed, err := st.ClaimBlueprintAutoSyncIntents(bounded, 10, time.Minute)
	if err != nil || len(claimed) != 1 || claimed[0].ClaimGeneration != first.ClaimGeneration+1 {
		t.Fatalf("reclaim waited on completion: claimed=%+v err=%v", claimed, err)
	}
	if _, _, err := st.AdmitBlueprintAutoSyncRun(bounded, claimed[0], bp, autoSyncRunPG()); err != nil {
		t.Fatalf("successor blocked by old completion lock: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CompleteBlueprintSync(ctx, bp.ID, bp.TenantID, oldRun.ID, oldRun.ExecutionGeneration, BlueprintSyncStateSuccess, time.Now(), nil); !errors.Is(err, ErrBlueprintSyncBusy) {
		t.Fatalf("old completion regained authority after unlock: %v", err)
	}
}
