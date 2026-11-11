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
	// Render: "You can create multiple Blueprints from a single YAML file"
	// (w8/m153). The second is its own row, not a conflict with the first.
	again, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "Again", Repo: repo, Branch: "main", Path: "render.yaml"})
	if err != nil || again.ID == b.ID || again.Status != store.BlueprintStatusCreated {
		t.Fatalf("a second Blueprint from the same repo, branch and file = %+v err=%v, want a new created row beside %s", again, err, b.ID)
	}
	if err := st.UndoPlannedBlueprintCreate(ctx, again); err != nil {
		t.Fatalf("undo the second create: %v", err)
	}
	if _, err := st.GetBlueprint(ctx, again.ID, ws.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("an undone create is still readable: err = %v", err)
	}
	if _, err := st.GetBlueprint(ctx, b.ID, ws.ID); err != nil {
		t.Fatalf("undoing the second create touched the first: %v", err)
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
		PlanActions:           json.RawMessage(`[{"operation":"create","kind":"service","name":"api"}]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	latest, err := st.LatestBlueprintPlan(ctx, b.ID)
	if err != nil || latest.ID != planned.ID || latest.Path != "render.yaml" || string(latest.RequiredManualEnvVars) == "[]" ||
		!strings.Contains(string(latest.PlanActions), `"operation": "create"`) && !strings.Contains(string(latest.PlanActions), `"operation":"create"`) {
		t.Fatalf("latest = %+v (actions %s) err=%v", latest, latest.PlanActions, err)
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

// A deleted Blueprint is not brought back: a create after it is a new
// Blueprint with a new id (w8/m153; the row used to be revived, w4/120), and
// the deleted one's waiting plan answers busy, not not-found (w2/m256 review).
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
	again, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "s", Repo: repo, Branch: "main", Path: "render.yaml"})
	if err != nil {
		t.Fatalf("create after the delete: %v", err)
	}
	if again.ID == b.ID {
		t.Fatalf("a create after a delete reused the deleted row %s", b.ID)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM blueprints WHERE id = $1`, b.ID).Scan(&status); err != nil || status != "disconnected" {
		t.Fatalf("the deleted Blueprint's row = %q err=%v, want it left disconnected", status, err)
	}
	_, _, err = st.AdmitBlueprintSyncRun(ctx, b.ID, ws.ID, store.BlueprintSync{ID: old.ID, CommitID: commit, State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()})
	if !errors.Is(err, store.ErrBlueprintSyncBusy) {
		t.Fatalf("admitting the deleted Blueprint's plan: err = %v, want ErrBlueprintSyncBusy", err)
	}
	// The plan belongs to the Blueprint it was made for, never to a later one
	// on the same source.
	if _, _, err := st.AdmitBlueprintSyncRun(ctx, again.ID, ws.ID, store.BlueprintSync{ID: old.ID, CommitID: commit, State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()}); err == nil {
		t.Fatal("the new Blueprint admitted the deleted Blueprint's plan")
	}

	// A planned sync's note survives an approval that brings none (w2/m256 t013).
	const note = "a note recorded with the plan"
	fresh, err := st.InsertPlannedBlueprintSync(ctx, store.PlannedBlueprintSync{
		BlueprintSync: store.BlueprintSync{BlueprintID: again.ID, CommitID: commit, State: store.BlueprintSyncStateCreated, StartedAt: time.Now().UTC(), Note: note},
		Path:          "render.yaml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, run, err := st.AdmitBlueprintSyncRun(ctx, again.ID, ws.ID, store.BlueprintSync{ID: fresh.ID, CommitID: commit, State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()}); err != nil || run.Note != note {
		t.Fatalf("approved run note = %q, err=%v; want the planned sync's note kept", run.Note, err)
	}
}

// TestCreateNewBlueprintSuffixPG pins w2/m261's storage: a create_new
// Blueprint gets one Render-shaped suffix at create, and every read that feeds
// a plan or a sync returns it (an empty suffix there would compile the
// unsuffixed names and touch another Blueprint's resources). A one-step create
// of the same source is a separate Blueprint and leaves it alone (w8/m153).
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

	// A one-step create of the same source, in the default adopt mode, is its
	// own Blueprint: the create_new one keeps its id, mode and suffix.
	second, _, err := st.AdmitBlueprintCreate(ctx, store.Blueprint{TenantID: ws.ID, Name: "Copy", Repo: repo, Branch: "main", Path: "render.yaml"},
		store.BlueprintSync{CommitID: "0123456789abcdef0123456789abcdef01234567", State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()}, "")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == b.ID || second.ResourceSuffix != "" || second.ExistingResources != store.BlueprintExistingAdopt {
		t.Fatalf("one-step create = %s mode %q suffix %q, want a new adopt Blueprint with no suffix", second.ID, second.ExistingResources, second.ResourceSuffix)
	}
	got, err = st.GetBlueprint(ctx, b.ID, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	check("GetBlueprint after a second create of the source", got)
}

// TestBlueprintPlanConnectionAndNoChangesPG pins w2/m256 t014's storage: a
// Blueprint created after another on the same source was deleted has no plan
// of its own yet (the deleted one's applied plan does not answer), and a
// no_changes plan
// supersedes a waiting plan and takes a created Blueprint to in_sync — while
// a Blueprint in any other status keeps it.
func TestBlueprintPlanConnectionAndNoChangesPG(t *testing.T) {
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
	ws, err := st.CreateWorkspace(ctx, "bp-conn-"+stamp, store.PlanHobby, "bp-conn-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	repo := "https://github.com/acme/conn-" + stamp
	commit := "0123456789abcdef0123456789abcdef01234567"
	plan := func(bpID string) store.PlannedBlueprintSync {
		t.Helper()
		p, err := st.InsertPlannedBlueprintSync(ctx, store.PlannedBlueprintSync{
			BlueprintSync: store.BlueprintSync{BlueprintID: bpID, CommitID: commit, State: store.BlueprintSyncStateCreated, StartedAt: time.Now().UTC()},
			Path:          "render.yaml", PlanActions: json.RawMessage(`[{"operation":"create","kind":"service","name":"api"}]`),
		})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}

	b, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "s", Repo: repo, Branch: "main", Path: "render.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	applied := plan(b.ID)
	if _, _, err := st.AdmitBlueprintSyncRun(ctx, b.ID, ws.ID, store.BlueprintSync{ID: applied.ID, CommitID: commit, State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE blueprint_syncs SET state = 'success', completed_at = now() WHERE id = $1`, applied.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE blueprints SET active_run_id = NULL, status = 'in_sync', last_sync_at = now() WHERE id = $1`, b.ID); err != nil {
		t.Fatal(err)
	}
	if latest, err := st.LatestBlueprintPlan(ctx, b.ID); err != nil || latest.ID != applied.ID {
		t.Fatalf("latest before the reconnect = %+v err=%v, want %s", latest, err, applied.ID)
	}

	// An in_sync Blueprint keeps its status through a no_changes plan, which
	// still supersedes its waiting plan.
	waiting := plan(b.ID)
	settled, err := st.SettleNoChangesPlan(ctx, b.ID, ws.ID, "changed: manifest")
	if err != nil || settled.Status != store.BlueprintStatusInSync || settled.Manifest == "changed: manifest" {
		t.Fatalf("settle on in_sync = %+v err=%v, want in_sync with its manifest kept", settled, err)
	}
	if got, err := st.BlueprintPlannedSync(ctx, b.ID, waiting.ID); err != nil || got.State != store.BlueprintSyncStateError ||
		got.ErrorMessage == nil || !strings.Contains(*got.ErrorMessage, "superseded") {
		t.Fatalf("waiting plan after a no_changes plan = %+v err=%v, want superseded", got, err)
	}

	if err := st.DisconnectBlueprint(ctx, b.ID, ws.ID); err != nil {
		t.Fatal(err)
	}
	deleted := b
	b, err = st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "s", Repo: repo, Branch: "main", Path: "render.yaml"})
	if err != nil || b.ID == deleted.ID {
		t.Fatalf("create after the delete = %+v err=%v, want a new Blueprint, not %s", b, err, deleted.ID)
	}
	if latest, err := st.LatestBlueprintPlan(ctx, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("latest plan of the new Blueprint = %+v err=%v, want ErrNotFound (the deleted Blueprint's do not count)", latest, err)
	}
	const manifest = "services: []\n"
	settled, err = st.SettleNoChangesPlan(ctx, b.ID, ws.ID, manifest)
	if err != nil || settled.Status != store.BlueprintStatusInSync || settled.LastSyncAt == nil || settled.Manifest != manifest {
		t.Fatalf("settle on created = %+v err=%v, want in_sync with last_sync_at and the manifest", settled, err)
	}
	own := plan(b.ID)
	if latest, err := st.LatestBlueprintPlan(ctx, b.ID); err != nil || latest.ID != own.ID {
		t.Fatalf("latest plan = %+v err=%v, want this connection's %s", latest, err, own.ID)
	}
}
