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
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// recreateManifest declares one service with no manual env vars, so approval
// needs no values.
const recreateManifest = `
services:
  - type: web
    name: api
    runtime: image
    image:
      url: docker.io/library/nginx:1.27
`

// holdAppFinalizer gives the service's App the operator's finalizer, so a
// delete leaves it Terminating while the teardown runs (w3/m81).
func holdAppFinalizer(t *testing.T, ctx context.Context, cl client.Client, key client.ObjectKey) {
	t.Helper()
	a := &appv1alpha1.App{}
	if err := cl.Get(ctx, key, a); err != nil {
		t.Fatal(err)
	}
	controllerutil.AddFinalizer(a, "app.bex.co/finalizer")
	if err := cl.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
}

// releaseAppFinalizer ends the teardown: the App object goes away.
func releaseAppFinalizer(t *testing.T, ctx context.Context, cl client.Client, key client.ObjectKey) {
	t.Helper()
	a := &appv1alpha1.App{}
	if err := cl.Get(ctx, key, a); err != nil {
		t.Fatal(err)
	}
	controllerutil.RemoveFinalizer(a, "app.bex.co/finalizer")
	if err := cl.Update(ctx, a); err != nil {
		t.Fatal(err)
	}
}

func plannedActions(res BlueprintPlanResult) string {
	if res.Sync == nil {
		return ""
	}
	parts := make([]string, 0, len(res.Sync.Actions))
	for _, a := range res.Sync.Actions {
		parts = append(parts, string(a.Operation)+" "+string(a.Kind)+" "+a.Name)
	}
	return strings.Join(parts, ", ")
}

// w2/m256 t014, the production walk: create → approve, delete the Blueprint,
// delete its service, create again. The re-created Blueprint plans the
// service's re-creation (Render: "the next time you sync your Blueprint,
// Render detects that a defined resource is missing and recreates it"), even
// while the deleted App is still Terminating, and approving re-creates it
// once the teardown is over.
func TestRecreatedBlueprintPlansADeletedServiceAsCreate(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, recreateManifest)
	req := RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}}
	first, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, first.Blueprint.ID, "tea-a", nil); err != nil {
		t.Fatal(err)
	}
	key := appKey(t, svc, "api")
	holdAppFinalizer(t, ctx, svc.Client, key)

	if err := svc.DisconnectBlueprint(ctx, first.Blueprint.ID, "tea-a"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, "api"); err != nil {
		t.Fatalf("delete the service: %v", err)
	}
	if _, err := svc.Get(ctx, "api"); err == nil {
		t.Fatal("the deleted service still reads; the test needs it gone from GET")
	}

	again, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if again.Blueprint.ID != first.Blueprint.ID {
		t.Fatalf("re-create = %s, want the revived row %s", again.Blueprint.ID, first.Blueprint.ID)
	}
	if again.PlanningOutcome != PlanningPlanned || plannedActions(again) != "create service api" {
		t.Fatalf("re-create plan = %s %q, want planned with create service api", again.PlanningOutcome, plannedActions(again))
	}
	if again.Blueprint.Status != store.BlueprintStatusCreated {
		t.Fatalf("re-created status = %q, want created", again.Blueprint.Status)
	}
	// While the deleted App's teardown still holds its name, approving
	// refuses by name before writing anything, and nothing is left stuck.
	if _, err := svc.ApproveBlueprint(ctx, again.Blueprint.ID, "tea-a", nil); err == nil || coded(t, err) != ServiceStillDeletingCode {
		t.Fatalf("approve during the teardown: err = %v, want %s", err, ServiceStillDeletingCode)
	}

	releaseAppFinalizer(t, ctx, svc.Client, key)
	plan, err := svc.PlanBlueprint(ctx, again.Blueprint.ID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PlanningOutcome != PlanningPlanned || plannedActions(plan) != "create service api" {
		t.Fatalf("plan after the teardown = %s %q, want planned with create service api", plan.PlanningOutcome, plannedActions(plan))
	}
	approved, err := svc.ApproveBlueprint(ctx, again.Blueprint.ID, "tea-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if approved.ID != plan.Sync.ID || approved.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approved = %+v, want the plan %s applied", approved, plan.Sync.ID)
	}
	if err := svc.Client.Get(ctx, appKey(t, svc, "api"), &appv1alpha1.App{}); err != nil {
		t.Fatalf("approval did not re-create api: %v", err)
	}
	if b := st.blueprints[ids.BlueprintStorageID(again.Blueprint.ID)]; b.Status != store.BlueprintStatusInSync {
		t.Errorf("status after approval = %q, want in_sync", b.Status)
	}
}

// A plain sync of an in-sync Blueprint re-creates a service deleted since its
// last sync, through the same current-state resolution the plan uses.
func TestBlueprintSyncRecreatesADeletedService(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, recreateManifest)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "s", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, "api"); err != nil {
		t.Fatal(err)
	}
	plan, err := svc.PlanBlueprint(ctx, res.Blueprint.ID, "tea-a")
	if err != nil || plannedActions(plan) != "create service api" {
		t.Fatalf("plan after the delete = %q err=%v, want create service api", plannedActions(plan), err)
	}
	if _, err := svc.SyncBlueprint(ctx, res.Blueprint.ID, "tea-a", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.Client.Get(ctx, appKey(t, svc, "api"), &appv1alpha1.App{}); err != nil {
		t.Fatalf("sync did not re-create api: %v", err)
	}
}

// w2/m256 t014: a re-created Blueprint whose first plan finds nothing to
// change (its resources already match the file) is in sync — Render needs no
// approval for no_changes — so a manual sync runs it. The earlier
// connection's applied plan does not answer for the new connection.
func TestNoChangesFirstPlanLeavesCreated(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, recreateManifest)
	req := RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}}
	first, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, first.Blueprint.ID, "tea-a", nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.DisconnectBlueprint(ctx, first.Blueprint.ID, "tea-a"); err != nil {
		t.Fatal(err)
	}

	again, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if again.PlanningOutcome != PlanningNoChanges || again.Sync != nil {
		t.Fatalf("re-create = %s sync=%v, want no_changes with no sync", again.PlanningOutcome, again.Sync)
	}
	if again.Blueprint.Status != store.BlueprintStatusInSync || again.Blueprint.LastSync == nil {
		t.Fatalf("re-created blueprint = %+v, want in_sync with a last sync time", again.Blueprint)
	}
	stored := st.blueprints[ids.BlueprintStorageID(again.Blueprint.ID)]
	if stored.Status != store.BlueprintStatusInSync || stored.Manifest != recreateManifest {
		t.Fatalf("stored = status %q manifest %q, want in_sync with the planned file", stored.Status, stored.Manifest)
	}
	// Nothing to approve on this connection, said without the earlier one's sync.
	_, err = svc.ApproveBlueprint(ctx, again.Blueprint.ID, "tea-a", nil)
	if err == nil || coded(t, err) != "BLUEPRINT_NOTHING_TO_APPROVE" || strings.Contains(err.Error(), first.Sync.ID) {
		t.Fatalf("approve = %v, want BLUEPRINT_NOTHING_TO_APPROVE naming no earlier-connection sync", err)
	}
	if got, err := svc.PlannedSync(ctx, again.Blueprint.ID, "tea-a"); err != nil || got != nil {
		t.Fatalf("planned sync = %+v err=%v, want none", got, err)
	}
	if _, err := svc.SyncBlueprint(ctx, again.Blueprint.ID, "tea-a", "", "", nil); err != nil {
		t.Fatalf("a manual sync of the in-sync Blueprint: %v", err)
	}
}

// A no_changes plan supersedes a plan still waiting for approval: approval
// applies the newest plan, and the newest needs nothing.
func TestNoChangesPlanSupersedesAWaitingPlan(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, recreateManifest)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "s", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	// The service the plan creates appears some other way before approval.
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: recreateManifest}); err != nil {
		t.Fatal(err)
	}
	plan, err := svc.PlanBlueprint(ctx, res.Blueprint.ID, "tea-a")
	if err != nil || plan.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("plan = %+v err=%v, want no_changes", plan, err)
	}
	_, err = svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", nil)
	if err == nil || coded(t, err) != "BLUEPRINT_NOTHING_TO_APPROVE" || !strings.Contains(err.Error(), "superseded") {
		t.Fatalf("approving the superseded plan: err = %v, want BLUEPRINT_NOTHING_TO_APPROVE (superseded)", err)
	}
	if _, err := svc.SyncBlueprint(ctx, res.Blueprint.ID, "tea-a", "", "", nil); err != nil {
		t.Fatalf("sync after the no_changes plan: %v", err)
	}
}
