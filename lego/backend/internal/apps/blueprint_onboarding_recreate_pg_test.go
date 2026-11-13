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

package apps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// newRecreatePGService wires the onboarding verbs to real Postgres (the
// Blueprint rows, plans and sync history) over a fake cluster.
func newRecreatePGService(t *testing.T) (*Service, *store.PGStore, *pgxpool.Pool, context.Context, string) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	ws, err := st.CreateWorkspace(context.Background(), "bp-recreate-"+stamp, store.PlanHobby, "bp-recreate-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	svc := &Service{
		Base:       &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"recreate-owner": ws.ID}},
		Blueprints: st,
		GitFetcher: fakeBlueprintFetcher{contents: recreateManifest},
		EnvSeeder:  &fakeSeeder{},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "recreate-owner", Method: "oauth2"})
	return svc, st, pool, ctx, ws.ID
}

// TestRecreatedBlueprintRecreatesDeletedServicePG replays the t007 production
// walk (2026-11-08) on real Postgres: create → approve, DELETE the Blueprint,
// DELETE its service (still Terminating behind the operator's finalizer),
// create again. The revived row plans `create service api` rather than
// no_changes, its first plan is its own connection's (not the earlier
// connection's applied sync), and approve re-creates the service at once,
// beside the old App's teardown (w2/m256 t014, w2/m365).
func TestRecreatedBlueprintRecreatesDeletedServicePG(t *testing.T) {
	svc, st, pool, ctx, wsID := newRecreatePGService(t)
	// As the API server does, the plan's dry-run create meets the deleted
	// App's name while its teardown holds it: the production 500.
	svc.Client = apiServerDryRuns{svc.Client}
	repo := "https://github.com/acme/recreate-" + wsID
	req := RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo}}

	first, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.PlanningOutcome != PlanningPlanned || plannedActions(first) != "create service api" {
		t.Fatalf("create = %s %q", first.PlanningOutcome, plannedActions(first))
	}
	firstApproved, err := svc.ApproveBlueprint(ctx, first.Blueprint.ID, wsID, nil)
	if err != nil || firstApproved.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %+v err=%v", firstApproved, err)
	}
	key := appKeyIn(t, svc, wsID, "api")
	holdAppFinalizer(t, ctx, svc.Client, key)

	if err := svc.DisconnectBlueprint(ctx, first.Blueprint.ID, wsID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, "api"); err != nil {
		t.Fatalf("delete the service: %v", err)
	}

	again, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	// A new Blueprint with its own id: the deleted one's row is not revived
	// (w8/m153).
	if again.Blueprint.ID == first.Blueprint.ID {
		t.Fatalf("re-create reused the deleted Blueprint's row %s", first.Blueprint.ID)
	}
	if again.PlanningOutcome != PlanningPlanned || plannedActions(again) != "create service api" || again.Blueprint.Status != store.BlueprintStatusCreated {
		t.Fatalf("re-create = %s %q status %q, want planned create service api, created", again.PlanningOutcome, plannedActions(again), again.Blueprint.Status)
	}
	if again.Sync.ID == firstApproved.ID {
		t.Fatalf("the re-create's plan is the earlier connection's sync %s", firstApproved.ID)
	}
	// The manual sync still waits for approval, and approval has a plan.
	if _, err := svc.SyncBlueprint(ctx, again.Blueprint.ID, wsID, "", "", nil); err == nil || coded(t, err) != BlueprintAwaitingApprovalCode {
		t.Fatalf("manual sync of the created Blueprint: err = %v", err)
	}
	// Approving while the deleted App still tears down re-creates the service
	// at once, as on Render, under its own App object beside the old one
	// (w2/m365); the old App finishes on its own.
	approved, err := svc.ApproveBlueprint(ctx, again.Blueprint.ID, wsID, nil)
	if err != nil || approved.ID != again.Sync.ID || approved.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve during the teardown = %+v err=%v, want the plan %s applied", approved, err, again.Sync.ID)
	}
	deleting, live := liveAndDeleting(t, svc.Client, "api")
	if len(live) != 1 || deleting != 1 || !strings.HasPrefix(live[0].Name, key.Name+"-") {
		t.Fatalf("api Apps: live %v, %d deleting; want a new %s-xxxx beside the old %s", live, deleting, key.Name, key.Name)
	}
	releaseAppFinalizer(t, ctx, svc.Client, key)
	b, err := st.GetBlueprint(ctx, again.Blueprint.ID, wsID)
	if err != nil || b.Status != store.BlueprintStatusInSync {
		t.Fatalf("blueprint after approval = %+v err=%v, want in_sync", b, err)
	}
	var waiting int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blueprint_syncs WHERE blueprint_id = $1 AND state = 'created'`, b.ID).Scan(&waiting); err != nil || waiting != 0 {
		t.Fatalf("%d plans still waiting (err %v), want none", waiting, err)
	}
}

// TestNoChangesFirstPlanLeavesCreatedPG: a re-created Blueprint whose
// resources still match its file plans no_changes, leaves `created` for
// in_sync, and syncs; approve says there is nothing to approve on this
// connection (w2/m256 t014).
func TestNoChangesFirstPlanLeavesCreatedPG(t *testing.T) {
	svc, st, _, ctx, wsID := newRecreatePGService(t)
	repo := "https://github.com/acme/nochanges-" + wsID
	req := RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo}}
	first, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, first.Blueprint.ID, wsID, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.DisconnectBlueprint(ctx, first.Blueprint.ID, wsID); err != nil {
		t.Fatal(err)
	}

	again, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if again.PlanningOutcome != PlanningNoChanges || again.Sync != nil || again.Blueprint.Status != store.BlueprintStatusInSync {
		t.Fatalf("re-create = %s sync=%v status %q, want no_changes, no sync, in_sync", again.PlanningOutcome, again.Sync, again.Blueprint.Status)
	}
	b, err := st.GetBlueprint(ctx, again.Blueprint.ID, wsID)
	if err != nil || b.Status != store.BlueprintStatusInSync || b.LastSyncAt == nil || b.Manifest != recreateManifest {
		t.Fatalf("stored = %+v err=%v, want in_sync with last_sync_at and the planned file", b, err)
	}
	if _, err := svc.ApproveBlueprint(ctx, again.Blueprint.ID, wsID, nil); err == nil || coded(t, err) != "BLUEPRINT_NOTHING_TO_APPROVE" {
		t.Fatalf("approve: err = %v, want BLUEPRINT_NOTHING_TO_APPROVE", err)
	}
	res, err := svc.SyncBlueprint(ctx, again.Blueprint.ID, wsID, "", "", nil)
	if err != nil || res.Blueprint.Status != store.BlueprintStatusInSync {
		t.Fatalf("manual sync = %+v err=%v, want in_sync", res.Blueprint, err)
	}
}

// TestCreateAfterADeleteWhosePlanFailsLeavesOnlyTheDeletedRowPG: on real
// Postgres, a create that follows a deleted Blueprint on the same source and
// whose plan cannot be made leaves the deleted row exactly as it was — name,
// file, creation time and waiting plan — and no row of its own; the retry is a
// new Blueprint (w2/m256 t014 as amended by w8/m153, which no longer revives
// the deleted row).
func TestCreateAfterADeleteWhosePlanFailsLeavesOnlyTheDeletedRowPG(t *testing.T) {
	svc, _, pool, ctx, wsID := newRecreatePGService(t)
	failing := &atomic.Bool{}
	svc.Client = failingAppLists{Client: svc.Client, failing: failing}
	repo := "https://github.com/acme/undo-" + wsID
	req := RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo}}
	first, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil || first.PlanningOutcome != PlanningPlanned {
		t.Fatalf("create = %+v err=%v", first, err)
	}
	// Disconnected with its plan still waiting.
	if err := svc.DisconnectBlueprint(ctx, first.Blueprint.ID, wsID); err != nil {
		t.Fatal(err)
	}
	type row struct {
		name, status, path string
		createdAt          time.Time
		waiting            int
	}
	read := func(publicID string) (r row) {
		t.Helper()
		if err := pool.QueryRow(ctx, `
			SELECT b.name, b.status, b.path, b.created_at,
			       (SELECT count(*) FROM blueprint_syncs s WHERE s.blueprint_id = b.id AND s.state = 'created')
			FROM blueprints b WHERE b.tenant_id = $1 AND b.id = $2`, wsID, ids.BlueprintStorageID(publicID),
		).Scan(&r.name, &r.status, &r.path, &r.createdAt, &r.waiting); err != nil {
			t.Fatal(err)
		}
		return r
	}
	rows := func() (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM blueprints WHERE tenant_id = $1 AND repo = $2`, wsID, repo).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := read(first.Blueprint.ID)
	if before.status != "disconnected" || before.waiting != 1 {
		t.Fatalf("before = %+v, want disconnected with its plan waiting", before)
	}

	failing.Store(true)
	req.Name = "qa-bp-2"
	if _, err := svc.CreatePlannedBlueprint(ctx, req); !errors.Is(err, errAppListDown) {
		t.Fatalf("re-create while the workspace is unreadable: err = %v, want %v", err, errAppListDown)
	}
	if after := read(first.Blueprint.ID); after != before || rows() != 1 {
		t.Fatalf("after the failed re-create: %d rows, the deleted one %+v; want only it, as it was: %+v", rows(), after, before)
	}

	failing.Store(false)
	again, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatalf("retrying the re-create: %v", err)
	}
	if again.Blueprint.ID == first.Blueprint.ID || again.PlanningOutcome != PlanningPlanned || again.Blueprint.Name != "qa-bp-2" {
		t.Fatalf("retry = %s %s %q, want a new Blueprint planned as qa-bp-2", again.Blueprint.ID, again.PlanningOutcome, again.Blueprint.Name)
	}
	if r := read(again.Blueprint.ID); r.status != store.BlueprintStatusCreated || r.waiting != 1 {
		t.Fatalf("the new Blueprint = %+v, want created with its own plan waiting", r)
	}
	if after := read(first.Blueprint.ID); after != before || rows() != 2 {
		t.Fatalf("after the retry: %d rows, the deleted one %+v; want 2 with the deleted one untouched: %+v", rows(), after, before)
	}
}

// TestCreateWhosePlanFailsLeavesNoRowPG: a first create whose plan cannot be
// made leaves no row.
func TestCreateWhosePlanFailsLeavesNoRowPG(t *testing.T) {
	svc, _, pool, ctx, wsID := newRecreatePGService(t)
	failing := &atomic.Bool{}
	failing.Store(true)
	svc.Client = failingAppLists{Client: svc.Client, failing: failing}
	repo := "https://github.com/acme/undo-new-" + wsID
	req := RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo}}
	if _, err := svc.CreatePlannedBlueprint(ctx, req); !errors.Is(err, errAppListDown) {
		t.Fatalf("create: err = %v, want %v", err, errAppListDown)
	}
	var rows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blueprints WHERE tenant_id = $1 AND repo = $2`, wsID, repo).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("the failed create left %d rows (err %v)", rows, err)
	}
	failing.Store(false)
	if res, err := svc.CreatePlannedBlueprint(ctx, req); err != nil || res.PlanningOutcome != PlanningPlanned {
		t.Fatalf("retry = %+v err=%v, want planned", res, err)
	}
}

// appKeyIn is appKey for a workspace other than tea-a.
func appKeyIn(t *testing.T, svc *Service, tenantID, service string) client.ObjectKey {
	t.Helper()
	var apps appv1alpha1.AppList
	if err := svc.Client.List(context.Background(), &apps); err != nil {
		t.Fatal(err)
	}
	for _, a := range apps.Items {
		if a.DeletionTimestamp.IsZero() && (a.Name == service || a.Name == core.CRName(tenantID, service)) {
			return client.ObjectKey{Namespace: a.Namespace, Name: a.Name}
		}
	}
	t.Fatalf("no App for service %q", service)
	return client.ObjectKey{}
}
