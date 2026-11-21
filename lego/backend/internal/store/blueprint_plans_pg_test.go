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
		Branch:                "release",
		RequiredManualEnvVars: json.RawMessage(`[{"serviceType":"web","serviceName":"api","key":"K"}]`),
		PlanActions:           json.RawMessage(`[{"operation":"create","kind":"service","name":"api"}]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	latest, err := st.LatestBlueprintPlan(ctx, b.ID, time.Now())
	if err != nil || latest.ID != planned.ID || latest.Path != "render.yaml" || latest.Branch != "release" || string(latest.RequiredManualEnvVars) == "[]" ||
		!strings.Contains(string(latest.PlanActions), `"operation": "create"`) && !strings.Contains(string(latest.PlanActions), `"operation":"create"`) {
		t.Fatalf("latest = %+v (actions %s) err=%v", latest, latest.PlanActions, err)
	}
	// A plan recorded before migration 0321 has no branch; it reads as empty,
	// which approval fills with the Blueprint's (w32/074).
	if _, err := pool.Exec(ctx, `UPDATE blueprint_syncs SET plan_branch = NULL WHERE id = $1`, planned.ID); err != nil {
		t.Fatal(err)
	}
	if older, err := st.BlueprintPlannedSync(ctx, b.ID, planned.ID); err != nil || older.Branch != "" || older.Path != "render.yaml" {
		t.Fatalf("a plan with no recorded branch = %+v err=%v, want branch empty and its path", older, err)
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
// no_changes plan leaves a waiting plan waiting (w32/073) and takes a created
// Blueprint to in_sync, or to paused when its auto-sync is off (w32/069) —
// while a Blueprint in any other status keeps it.
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
	if latest, err := st.LatestBlueprintPlan(ctx, b.ID, time.Now()); err != nil || latest.ID != applied.ID {
		t.Fatalf("latest before the reconnect = %+v err=%v, want %s", latest, err, applied.ID)
	}

	// An in_sync Blueprint keeps its status through a no_changes plan, and its
	// waiting plan stays waiting and the newest: the no_changes plan saves no
	// sync, as on Render (w32/073).
	waiting := plan(b.ID)
	settled, err := st.SettleNoChangesPlan(ctx, b.ID, ws.ID, "changed: manifest")
	if err != nil || settled.Status != store.BlueprintStatusInSync || settled.Manifest == "changed: manifest" {
		t.Fatalf("settle on in_sync = %+v err=%v, want in_sync with its manifest kept", settled, err)
	}
	if got, err := st.BlueprintPlannedSync(ctx, b.ID, waiting.ID); err != nil || got.State != store.BlueprintSyncStateCreated || got.ErrorMessage != nil {
		t.Fatalf("waiting plan after a no_changes plan = %+v err=%v, want it still created", got, err)
	}
	if latest, err := st.LatestBlueprintPlan(ctx, b.ID, time.Now()); err != nil || latest.ID != waiting.ID {
		t.Fatalf("latest plan after a no_changes plan = %+v err=%v, want the waiting plan %s", latest, err, waiting.ID)
	}
	// The plan is stamped whatever the status, after the plan still waiting
	// (w32/071): it writes no sync row, so approval orders by the stamp.
	if settled.NoChangesPlannedAt == nil || !waiting.CreatedAt.Before(*settled.NoChangesPlannedAt) {
		t.Fatalf("no_changes_planned_at = %v, want a time after the waiting plan's %v", settled.NoChangesPlannedAt, waiting.CreatedAt)
	}

	if err := st.DisconnectBlueprint(ctx, b.ID, ws.ID); err != nil {
		t.Fatal(err)
	}
	deleted := b
	b, err = st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "s", Repo: repo, Branch: "main", Path: "render.yaml", AutoSync: true})
	if err != nil || b.ID == deleted.ID {
		t.Fatalf("create after the delete = %+v err=%v, want a new Blueprint, not %s", b, err, deleted.ID)
	}
	if latest, err := st.LatestBlueprintPlan(ctx, b.ID, time.Now()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("latest plan of the new Blueprint = %+v err=%v, want ErrNotFound (the deleted Blueprint's do not count)", latest, err)
	}
	if fresh, err := st.GetBlueprint(ctx, b.ID, ws.ID); err != nil || fresh.NoChangesPlannedAt != nil {
		t.Fatalf("new Blueprint = %+v err=%v, want no no-changes plan (the deleted one's stamp is its own)", fresh, err)
	}
	const manifest = "services: []\n"
	settled, err = st.SettleNoChangesPlan(ctx, b.ID, ws.ID, manifest)
	if err != nil || settled.Status != store.BlueprintStatusInSync || settled.LastSyncAt == nil || settled.Manifest != manifest ||
		settled.NoChangesPlannedAt == nil {
		t.Fatalf("settle on created = %+v err=%v, want in_sync with last_sync_at, the manifest and the stamp", settled, err)
	}
	own := plan(b.ID)
	if latest, err := st.LatestBlueprintPlan(ctx, b.ID, time.Now()); err != nil || latest.ID != own.ID {
		t.Fatalf("latest plan = %+v err=%v, want this connection's %s", latest, err, own.ID)
	}
	// A plan after the no_changes one is newer than its stamp.
	if !settled.NoChangesPlannedAt.Before(own.CreatedAt) {
		t.Fatalf("plan created_at %v, want after the no-changes stamp %v", own.CreatedAt, *settled.NoChangesPlannedAt)
	}

	// Without auto-sync, the same settle leaves created for paused, as a
	// completed sync of such a Blueprint does.
	manual, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "manual", Repo: repo, Branch: "main", Path: "render.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	settled, err = st.SettleNoChangesPlan(ctx, manual.ID, ws.ID, manifest)
	if err != nil || settled.Status != store.BlueprintStatusPaused || settled.AutoSync || settled.LastSyncAt == nil || settled.Manifest != manifest {
		t.Fatalf("settle on created without auto-sync = %+v err=%v, want paused with last_sync_at and the manifest", settled, err)
	}
}

// TestBlueprintPlanApprovalWindowPG pins w32/075 on real Postgres: Render
// deletes "a planned Sync that is not approved within 24 hours". A `created`
// plan planned 24 hours before the caller's clock is passed over, so the
// attempt before it is the newest, while one 23 hours old still answers; only
// a waiting plan expires, never a run or a failed plan; and the sweep deletes
// exactly the expired ones, so the sync history no longer lists them.
func TestBlueprintPlanApprovalWindowPG(t *testing.T) {
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
	ws, err := st.CreateWorkspace(ctx, "bp-expiry-"+stamp, store.PlanHobby, "bp-expiry-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	// The plans are dated a week back, so the sweep below, at now, finds no
	// other test's rows expired.
	now := time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(time.Microsecond)
	commit := "0123456789abcdef0123456789abcdef01234567"
	blueprint := func(name string) store.Blueprint {
		t.Helper()
		b, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: name, Repo: "https://github.com/acme/expiry-" + stamp, Branch: "main", Path: "render.yaml"})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	plan := func(bpID, state string, age time.Duration) store.PlannedBlueprintSync {
		t.Helper()
		p := store.PlannedBlueprintSync{
			BlueprintSync: store.BlueprintSync{BlueprintID: bpID, CommitID: commit, State: state, StartedAt: now.Add(-age)},
			Path:          "render.yaml", PlanActions: json.RawMessage(`[{"operation":"create","kind":"service","name":"api"}]`),
		}
		if state == store.BlueprintSyncStateError {
			msg := "planning failed: boom"
			p.CompletedAt, p.ErrorMessage, p.PlanActions = &p.StartedAt, &msg, nil
		}
		p, err := st.InsertPlannedBlueprintSync(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	latest := func(bpID string, at time.Time) string {
		t.Helper()
		p, err := st.LatestBlueprintPlan(ctx, bpID, at)
		if errors.Is(err, store.ErrNotFound) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return p.ID
	}
	stored := func(syncID string) bool {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM blueprint_syncs WHERE id = $1`, syncID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}

	// A first plan: approvable at 23 hours, gone at 24.
	first := blueprint("first")
	young := plan(first.ID, store.BlueprintSyncStateCreated, 23*time.Hour)
	if got := latest(first.ID, now); got != young.ID {
		t.Fatalf("latest plan at 23h = %q, want %s", got, young.ID)
	}
	if got := latest(first.ID, now.Add(time.Hour)); got != "" {
		t.Fatalf("latest plan at 24h = %q, want none: the plan expired", got)
	}

	// A re-plan behind a failed one: once it expires, the failed plan before
	// it answers again. A failed plan never expires.
	replanned := blueprint("replanned")
	failed := plan(replanned.ID, store.BlueprintSyncStateError, 48*time.Hour)
	expired := plan(replanned.ID, store.BlueprintSyncStateCreated, 24*time.Hour)
	if got := latest(replanned.ID, now.Add(-time.Second)); got != expired.ID {
		t.Fatalf("latest plan a second before 24h = %q, want the waiting plan %s", got, expired.ID)
	}
	if got := latest(replanned.ID, now); got != failed.ID {
		t.Fatalf("latest plan at 24h = %q, want the failed plan before it, %s", got, failed.ID)
	}

	// An approved plan is a run, which never expires.
	approved := blueprint("approved")
	applied := plan(approved.ID, store.BlueprintSyncStateCreated, 2*time.Hour)
	if _, _, err := st.AdmitBlueprintSyncRun(ctx, approved.ID, ws.ID, store.BlueprintSync{ID: applied.ID, CommitID: commit, State: store.BlueprintSyncStateRunning, StartedAt: now.Add(-2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if got := latest(approved.ID, now.Add(30*24*time.Hour)); got != applied.ID {
		t.Fatalf("latest plan of an approved Blueprint a month on = %q, want its run %s", got, applied.ID)
	}

	deleted, err := st.DeleteExpiredBlueprintPlans(ctx, now, 100)
	if err != nil || deleted < 1 {
		t.Fatalf("sweep = %d err=%v, want the expired plan deleted", deleted, err)
	}
	if stored(expired.ID) {
		t.Errorf("the plan waiting 24h is still stored after the sweep")
	}
	for _, kept := range []store.PlannedBlueprintSync{young, failed, applied} {
		if !stored(kept.ID) {
			t.Errorf("the sweep deleted %s (%s), which had not expired", kept.ID, kept.State)
		}
	}
	runs, err := st.ListBlueprintSyncs(ctx, replanned.ID, "", 20)
	if err != nil || len(runs) != 1 || runs[0].ID != failed.ID {
		t.Fatalf("sync history after the sweep = %+v err=%v, want only the failed plan %s", runs, err, failed.ID)
	}
	if again, err := st.DeleteExpiredBlueprintPlans(ctx, now, 100); err != nil || again != 0 {
		t.Fatalf("a second sweep = %d err=%v, want nothing left to delete", again, err)
	}
	// An hour on, the young plan has expired too.
	if deleted, err := st.DeleteExpiredBlueprintPlans(ctx, now.Add(time.Hour), 100); err != nil || deleted < 1 || stored(young.ID) {
		t.Fatalf("sweep an hour later = %d err=%v (young stored %v), want the young plan deleted", deleted, err, stored(young.ID))
	}
}

// w32/077: the sync history leaves out a plan still waiting for approval, as
// Render's list does ("Syncs that are planned but not yet approved (in the
// `created` state) are not included", the pinned CLI's ListBlueprintSyncs). A
// failed plan and an applied run are listed; the filter comes before the page
// limit; a cursor naming the waiting plan pages from where it stands; and
// approving the plan lists it under the same id.
func TestBlueprintSyncHistoryLeavesOutWaitingPlansPG(t *testing.T) {
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
	ws, err := st.CreateWorkspace(ctx, "bp-history-"+stamp, store.PlanHobby, "bp-history-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	b, err := st.CreatePlannedBlueprint(ctx, store.Blueprint{TenantID: ws.ID, Name: "history", Repo: "https://github.com/acme/history-" + stamp, Branch: "main", Path: "render.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	commit := "0123456789abcdef0123456789abcdef01234567"
	done := now.Add(-3 * time.Hour)
	applied, err := st.InsertBlueprintSync(ctx, store.BlueprintSync{BlueprintID: b.ID, CommitID: commit, State: store.BlueprintSyncStateSuccess, StartedAt: done, CompletedAt: &done})
	if err != nil {
		t.Fatal(err)
	}
	planned := func(state string, age time.Duration) string {
		t.Helper()
		p := store.PlannedBlueprintSync{
			BlueprintSync: store.BlueprintSync{BlueprintID: b.ID, CommitID: commit, State: state, StartedAt: now.Add(-age)},
			Path:          "render.yaml", PlanActions: json.RawMessage(`[{"operation":"create","kind":"service","name":"api"}]`),
		}
		if state == store.BlueprintSyncStateError {
			msg := "planning failed: boom"
			p.CompletedAt, p.ErrorMessage, p.PlanActions = &p.StartedAt, &msg, nil
		}
		p, err := st.InsertPlannedBlueprintSync(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		return p.ID
	}
	failed := planned(store.BlueprintSyncStateError, 2*time.Hour)
	waiting := planned(store.BlueprintSyncStateCreated, time.Hour)
	list := func(cursor string, limit int) string {
		t.Helper()
		runs, err := st.ListBlueprintSyncs(ctx, b.ID, cursor, limit)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, len(runs))
		for i, r := range runs {
			out[i] = r.ID + ":" + r.State
		}
		return strings.Join(out, " ")
	}
	failedRow, appliedRow := failed+":"+store.BlueprintSyncStateError, applied.ID+":"+store.BlueprintSyncStateSuccess
	for _, tc := range []struct {
		name, cursor string
		limit        int
		want         string
	}{
		{"the first page", "", 20, failedRow + " " + appliedRow},
		{"a page of one", "", 1, failedRow},
		{"from the waiting plan", waiting, 20, failedRow + " " + appliedRow},
		{"from the failed plan", failed, 20, appliedRow},
	} {
		if got := list(tc.cursor, tc.limit); got != tc.want {
			t.Errorf("%s = %q, want %q: a plan waiting for approval is not listed", tc.name, got, tc.want)
		}
	}

	if _, _, err := st.AdmitBlueprintSyncRun(ctx, b.ID, ws.ID, store.BlueprintSync{ID: waiting, CommitID: commit, State: store.BlueprintSyncStateRunning, StartedAt: now}); err != nil {
		t.Fatal(err)
	}
	if got, want := list("", 20), waiting+":"+store.BlueprintSyncStateRunning+" "+failedRow+" "+appliedRow; got != want {
		t.Fatalf("history once the plan is approved = %q, want %q", got, want)
	}
}
