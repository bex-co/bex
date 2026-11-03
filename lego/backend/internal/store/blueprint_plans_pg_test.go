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

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPlannedBlueprintSyncsPG pins w2/m256's storage on real Postgres: a
// created Blueprint is excluded from auto-sync, a planned sync is the newest
// sync, admission turns that row into the run (same id), and a second live
// Blueprint on the same repo and branch is a conflict.
func TestPlannedBlueprintSyncsPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	ws, err := st.CreateWorkspace(ctx, "bp-plans-"+stamp, store.PlanHobby, "bp-plans-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	repo := "https://github.com/acme/plans-" + stamp

	b, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "Stack", Repo: repo, Branch: "main", Path: "render.yaml", AutoSync: true})
	if err != nil || b.Status != store.BlueprintStatusCreated || b.ExistingResources != store.BlueprintExistingAdopt {
		t.Fatalf("create = %+v err=%v", b, err)
	}
	if _, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "Again", Repo: repo, Branch: "main", Path: "render.yaml"}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("second live Blueprint on the same repo+branch: err = %v, want ErrConflict", err)
	}
	if taken, err := st.BlueprintNameTaken(ctx, ws.ID, "stack", ""); err != nil || taken {
		t.Fatalf("a created (unapproved) Blueprint's name counted as taken: %v %v", taken, err)
	}
	auto, err := st.ListAutoSyncBlueprints(ctx, "main", ws.ID)
	if err != nil || len(auto) != 0 {
		t.Fatalf("auto-sync listed a created Blueprint: %+v err=%v", auto, err)
	}

	commit := "0123456789abcdef0123456789abcdef01234567"
	planned, err := st.InsertPlannedBlueprintSync(ctx, store.PlannedBlueprintSync{
		BlueprintSync:         store.BlueprintSync{BlueprintID: b.ID, CommitID: commit, State: store.BlueprintSyncStateCreated, StartedAt: time.Now().UTC()},
		Path:                  "render.yaml",
		RequiredManualEnvVars: json.RawMessage(`[{"serviceType":"web","serviceName":"api","key":"K"}]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	latest, err := st.LatestBlueprintPlan(ctx, b.ID)
	if err != nil || latest.ID != planned.ID || latest.Path != "render.yaml" || string(latest.RequiredManualEnvVars) == "[]" {
		t.Fatalf("latest = %+v err=%v", latest, err)
	}

	admitted, run, err := st.AdmitBlueprintSyncRun(ctx, b.ID, ws.ID, store.BlueprintSync{ID: planned.ID, CommitID: commit, State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()})
	if err != nil || run.ID != planned.ID || run.State != store.BlueprintSyncStateRunning || admitted.ActiveRunID != planned.ID {
		t.Fatalf("admitting the planned sync: run=%+v err=%v", run, err)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blueprint_syncs WHERE blueprint_id = $1`, b.ID).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("admission left %d sync rows (err %v), want the planned row converted in place", rows, err)
	}
}

// A reconnected Blueprint's waiting plan is superseded, and a busy admission
// of an already-applied plan answers busy, not not-found (w2/m256 review).
func TestPlannedBlueprintSupersedeAndBusyPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	ws, err := st.CreateWorkspace(ctx, "bp-supersede-"+stamp, store.PlanHobby, "bp-supersede-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	repo := "https://github.com/acme/supersede-" + stamp
	b, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "s", Repo: repo, Branch: "main", Path: "render.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	commit := "89abcdef0123456789abcdef0123456789abcdef"
	old, err := st.InsertPlannedBlueprintSync(ctx, store.PlannedBlueprintSync{
		BlueprintSync: store.BlueprintSync{BlueprintID: b.ID, CommitID: commit, State: store.BlueprintSyncStateCreated, StartedAt: time.Now().UTC()},
		Path:          "render.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DisconnectBlueprint(ctx, b.ID, ws.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "s", Repo: repo, Branch: "main", Path: "render.yaml"}); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	stale, err := st.BlueprintPlannedSync(ctx, b.ID, old.ID)
	if err != nil || stale.State != store.BlueprintSyncStateError {
		t.Fatalf("the earlier connection's plan = %+v err=%v, want superseded (error)", stale, err)
	}
	_, _, err = st.AdmitBlueprintSyncRun(ctx, b.ID, ws.ID, store.BlueprintSync{ID: old.ID, CommitID: commit, State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()})
	if !errors.Is(err, store.ErrBlueprintSyncBusy) {
		t.Fatalf("admitting a superseded plan: err = %v, want ErrBlueprintSyncBusy", err)
	}
}

// TestCreateNewBlueprintSuffixPG pins w2/m261's storage: a create_new
// Blueprint gets one Render-shaped suffix at create, every read that feeds a
// plan or a sync returns it (an empty suffix there would compile the
// unsuffixed names and touch another Blueprint's resources), and the legacy
// connect path — which always adopts — clears it.
func TestCreateNewBlueprintSuffixPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	ws, err := st.CreateWorkspace(ctx, "bp-suffix-"+stamp, store.PlanHobby, "bp-suffix-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	repo := "https://github.com/acme/suffix-" + stamp

	b, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "Copy", Repo: repo, Branch: "main", Path: "render.yaml", AutoSync: true,
		ExistingResources: store.BlueprintExistingCreateNew})
	if err != nil {
		t.Fatal(err)
	}
	suffix := b.ResourceSuffix
	if len(suffix) != 4 || strings.Trim(suffix, "0123456789abcdefghijklmnopqrstuvwxyz") != "" {
		t.Fatalf("suffix = %q, want 4 lowercase base36 characters", suffix)
	}
	check := func(where string, got store.Blueprint) {
		t.Helper()
		if got.ResourceSuffix != suffix || got.ExistingResources != store.BlueprintExistingCreateNew {
			t.Errorf("%s: mode %q suffix %q, want create_new %q", where, got.ExistingResources, got.ResourceSuffix, suffix)
		}
	}
	got, err := st.GetBlueprint(ctx, b.ID, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	check("GetBlueprint", got)
	byRepo, err := st.GetBlueprintByRepo(ctx, ws.ID, repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	check("GetBlueprintByRepo", byRepo)
	listed, err := st.ListBlueprints(ctx, ws.ID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListBlueprints = %+v err=%v", listed, err)
	}
	check("ListBlueprints", listed[0])
	admitted, _, err := st.AdmitBlueprintSyncRun(ctx, b.ID, ws.ID, store.BlueprintSync{CommitID: "0123456789abcdef0123456789abcdef01234567",
		State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	check("AdmitBlueprintSyncRun", admitted)
	if _, err := pool.Exec(ctx, `UPDATE blueprints SET active_run_id = NULL, status = 'in_sync' WHERE id = $1`, b.ID); err != nil {
		t.Fatal(err)
	}
	auto, err := st.ListAutoSyncBlueprints(ctx, "main", ws.ID)
	if err != nil || len(auto) != 1 {
		t.Fatalf("ListAutoSyncBlueprints = %+v err=%v", auto, err)
	}
	check("ListAutoSyncBlueprints", auto[0])

	// The legacy connect replaces the connection and always adopts.
	replaced, _, _, err := st.AdmitBlueprintCreate(ctx, store.Blueprint{TenantID: ws.ID, Name: "Copy", Repo: repo, Branch: "main", Path: "render.yaml"},
		store.BlueprintSync{CommitID: "0123456789abcdef0123456789abcdef01234567", State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if replaced.ResourceSuffix != "" || replaced.ExistingResources != store.BlueprintExistingAdopt {
		t.Fatalf("legacy connect kept mode %q suffix %q, want adopt and no suffix", replaced.ExistingResources, replaced.ResourceSuffix)
	}
}
