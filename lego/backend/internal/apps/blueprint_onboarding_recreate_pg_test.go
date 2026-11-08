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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
// connection's applied sync), and approve re-creates the service once the
// teardown is over (w2/m256 t014).
func TestRecreatedBlueprintRecreatesDeletedServicePG(t *testing.T) {
	svc, st, pool, ctx, wsID := newRecreatePGService(t)
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
	if again.Blueprint.ID != first.Blueprint.ID {
		t.Fatalf("re-create = %s, want the revived row %s", again.Blueprint.ID, first.Blueprint.ID)
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
	if _, err := svc.ApproveBlueprint(ctx, again.Blueprint.ID, wsID, nil); err == nil || coded(t, err) != ServiceStillDeletingCode {
		t.Fatalf("approve during the teardown: err = %v, want %s", err, ServiceStillDeletingCode)
	}

	releaseAppFinalizer(t, ctx, svc.Client, key)
	plan, err := svc.PlanBlueprint(ctx, again.Blueprint.ID, wsID)
	if err != nil || plan.PlanningOutcome != PlanningPlanned || plannedActions(plan) != "create service api" {
		t.Fatalf("plan after the teardown = %+v (%q) err=%v", plan, plannedActions(plan), err)
	}
	approved, err := svc.ApproveBlueprint(ctx, again.Blueprint.ID, wsID, nil)
	if err != nil || approved.ID != plan.Sync.ID || approved.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %+v err=%v, want the plan %s applied", approved, err, plan.Sync.ID)
	}
	if err := svc.Client.Get(ctx, appKeyIn(t, svc, wsID, "api"), &appv1alpha1.App{}); err != nil {
		t.Fatalf("approval did not re-create api: %v", err)
	}
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
