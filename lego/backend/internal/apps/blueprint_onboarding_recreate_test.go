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
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// apiServerDryRuns gives the fake client's dry-run create the API server's
// last check, which the fake skips: storage refuses a name another object
// still holds — one whose deletion is still finishing included ("object is
// being deleted: … already exists") — after admission passed (w2/m256 t014).
type apiServerDryRuns struct{ client.Client }

func (c apiServerDryRuns) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	var o client.CreateOptions
	o.ApplyOptions(opts)
	if slices.Contains(o.DryRun, metav1.DryRunAll) && obj.GetName() != "" {
		held := obj.DeepCopyObject().(client.Object)
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), held); err == nil {
			gr := schema.GroupResource{Group: "app.bex.co", Resource: "apps"}
			if !held.GetDeletionTimestamp().IsZero() {
				return apierrors.NewAlreadyExists(gr, "object is being deleted: "+obj.GetName())
			}
			return apierrors.NewAlreadyExists(gr, obj.GetName())
		}
	}
	return c.Client.Create(ctx, obj, opts...)
}

// failingAppLists fails every App list while failing is set: the workspace's
// current resources cannot be read, so a plan cannot be made.
type failingAppLists struct {
	client.Client
	failing *atomic.Bool
}

var errAppListDown = errors.New("apps list unavailable")

func (c failingAppLists) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, apps := list.(*appv1alpha1.AppList); apps && c.failing.Load() {
		return errAppListDown
	}
	return c.Client.List(ctx, list, opts...)
}

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
// recreatedDuringTeardown connects a Blueprint, applies it, disconnects it,
// deletes its service with the App's finalizer held, and connects the
// Blueprint again: the new plan creates the service while its old App is
// still being torn down. It returns the new Blueprint's id and the App key.
func recreatedDuringTeardown(t *testing.T) (*Service, *fakeBlueprintStore, context.Context, string, client.ObjectKey) {
	t.Helper()
	svc, st, ctx := newOnboardingService(t, recreateManifest)
	svc.NameReleaseWait = 100 * time.Millisecond
	// The production 500: the plan's dry-run create of the re-created App
	// met the deleted App's name, still held by its teardown.
	svc.Client = apiServerDryRuns{svc.Client}
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
	// A new Blueprint with its own id: the deleted one's row is not revived
	// (w8/m153).
	if again.Blueprint.ID == first.Blueprint.ID {
		t.Fatalf("re-create reused the deleted Blueprint's row %s", first.Blueprint.ID)
	}
	if again.PlanningOutcome != PlanningPlanned || plannedActions(again) != "create service api" {
		t.Fatalf("re-create plan = %s %q, want planned with create service api", again.PlanningOutcome, plannedActions(again))
	}
	if again.Blueprint.Status != store.BlueprintStatusCreated {
		t.Fatalf("re-created status = %q, want created", again.Blueprint.Status)
	}
	return svc, st, ctx, again.Blueprint.ID, key
}

func TestRecreatedBlueprintPlansADeletedServiceAsCreate(t *testing.T) {
	svc, st, ctx, againID, key := recreatedDuringTeardown(t)
	// While the deleted App's teardown still holds its name past the apply's
	// wait (w2/m361), approving refuses by name before writing anything, and
	// nothing is left stuck.
	if _, err := svc.ApproveBlueprint(ctx, againID, "tea-a", nil); err == nil || coded(t, err) != ServiceStillDeletingCode {
		t.Fatalf("approve during the teardown: err = %v, want %s", err, ServiceStillDeletingCode)
	}

	releaseAppFinalizer(t, ctx, svc.Client, key)
	plan, err := svc.PlanBlueprint(ctx, againID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if plan.PlanningOutcome != PlanningPlanned || plannedActions(plan) != "create service api" {
		t.Fatalf("plan after the teardown = %s %q, want planned with create service api", plan.PlanningOutcome, plannedActions(plan))
	}
	approved, err := svc.ApproveBlueprint(ctx, againID, "tea-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	if approved.ID != plan.Sync.ID || approved.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approved = %+v, want the plan %s applied", approved, plan.Sync.ID)
	}
	if err := svc.Client.Get(ctx, appKey(t, svc, "api"), &appv1alpha1.App{}); err != nil {
		t.Fatalf("approval did not re-create api: %v", err)
	}
	if b := st.blueprints[ids.BlueprintStorageID(againID)]; b.Status != store.BlueprintStatusInSync {
		t.Errorf("status after approval = %q, want in_sync", b.Status)
	}
}

// w2/m361: Render re-creates at once. A teardown that finishes while the
// approval waits lets the approval itself re-create the service.
func TestApproveWaitsForATeardownToReleaseTheName(t *testing.T) {
	svc, _, ctx, againID, key := recreatedDuringTeardown(t)
	svc.NameReleaseWait = 10 * time.Second
	released := make(chan error, 1)
	go func() {
		time.Sleep(700 * time.Millisecond)
		var a appv1alpha1.App
		err := svc.Client.Get(ctx, key, &a)
		if err == nil {
			a.Finalizers = nil // the operator's teardown finishes
			err = svc.Client.Update(ctx, &a)
		}
		released <- err
	}()
	approved, err := svc.ApproveBlueprint(ctx, againID, "tea-a", nil)
	if rerr := <-released; rerr != nil {
		t.Fatal(rerr)
	}
	if err != nil {
		t.Fatalf("approve while the old service tears down = %v, want it to wait and apply", err)
	}
	if approved.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approved = %+v, want success", approved)
	}
	if err := svc.Client.Get(ctx, appKey(t, svc, "api"), &appv1alpha1.App{}); err != nil {
		t.Fatalf("approval did not re-create api: %v", err)
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

// w2/m256 t014: a create whose plan cannot be made is all-or-nothing, also
// when it follows a deleted Blueprint on the same source: the deleted row is
// left as it was, the failed create leaves no row of its own, and the retry is
// a new Blueprint (w8/m153; the deleted row used to be revived and restored).
func TestCreateAfterADeleteWhosePlanFailsLeavesOnlyTheDeletedRow(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, recreateManifest)
	failing := &atomic.Bool{}
	svc.Client = failingAppLists{Client: svc.Client, failing: failing}
	req := RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}}
	first, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DisconnectBlueprint(ctx, first.Blueprint.ID, "tea-a"); err != nil {
		t.Fatal(err)
	}
	id := ids.BlueprintStorageID(first.Blueprint.ID)
	before := st.blueprints[id]

	failing.Store(true)
	req.Name = "qa-bp-2"
	if _, err := svc.CreatePlannedBlueprint(ctx, req); !errors.Is(err, errAppListDown) {
		t.Fatalf("re-create while the workspace is unreadable: err = %v, want %v", err, errAppListDown)
	}
	if got := st.blueprints[id]; got.Status != "disconnected" || got.Name != before.Name || len(st.blueprints) != 1 {
		t.Fatalf("after the failed re-create: %d rows, the deleted one %q %q; want only it, still disconnected %q",
			len(st.blueprints), got.Status, got.Name, before.Name)
	}

	failing.Store(false)
	again, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatalf("retrying the re-create: %v", err)
	}
	if again.Blueprint.ID == first.Blueprint.ID || again.PlanningOutcome != PlanningPlanned || again.Blueprint.Name != "qa-bp-2" {
		t.Fatalf("retry = %s %s %q, want a new Blueprint planned as qa-bp-2", again.Blueprint.ID, again.PlanningOutcome, again.Blueprint.Name)
	}
	if got := st.blueprints[id]; got.Status != "disconnected" || got.Name != before.Name {
		t.Fatalf("the retry changed the deleted row: %q %q", got.Status, got.Name)
	}
}

// A first create whose plan cannot be made leaves no row at all.
func TestCreateWhosePlanFailsLeavesNoRow(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, recreateManifest)
	failing := &atomic.Bool{}
	failing.Store(true)
	svc.Client = failingAppLists{Client: svc.Client, failing: failing}
	req := RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}}
	if _, err := svc.CreatePlannedBlueprint(ctx, req); !errors.Is(err, errAppListDown) {
		t.Fatalf("create: err = %v, want %v", err, errAppListDown)
	}
	if len(st.blueprints) != 0 {
		t.Fatalf("the failed create left %d Blueprint rows", len(st.blueprints))
	}
	failing.Store(false)
	if res, err := svc.CreatePlannedBlueprint(ctx, req); err != nil || res.PlanningOutcome != PlanningPlanned {
		t.Fatalf("retry = %+v err=%v, want planned", res, err)
	}
}
