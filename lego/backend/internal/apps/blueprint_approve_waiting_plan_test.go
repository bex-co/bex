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
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w32/073 pins what approve applies after a re-plan, starting from a plan
// still waiting for approval, to the pinned Render CLI's contract
// (render-oss/cli 4fd34231b637, pkg/blueprint/output.go):
//
//   - replanNoChangesNext: "A plan with no changes is not saved, so an older
//     planned sync can still be the newest one, and approve would apply it."
//   - NewPlanOut / NewUpdateOut: a failed plan without a saved sync leaves the
//     older planned sync as the newest, "so approve could still apply it. A
//     saved failed sync is the newest one instead, and approve refuses it."

// waitingManifest adds a service to recreateManifest, so planning it on a
// Blueprint that applied recreateManifest plans `create service worker`.
const waitingManifest = recreateManifest + `  - type: web
    name: worker
    runtime: image
    image:
      url: docker.io/library/nginx:1.27
`

// blueprintWithAWaitingPlan applies recreateManifest, then plans
// waitingManifest, which waits for approval.
func blueprintWithAWaitingPlan(t *testing.T) (*Service, context.Context, string, BlueprintPlanResult) {
	t.Helper()
	svc, _, ctx := newOnboardingService(t, recreateManifest)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "s", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	svc.GitFetcher = fakeBlueprintFetcher{contents: waitingManifest}
	waiting, err := svc.PlanBlueprint(ctx, res.Blueprint.ID, "tea-a")
	if err != nil || waiting.PlanningOutcome != PlanningPlanned || plannedActions(waiting) != "noop service api, create service worker" {
		t.Fatalf("plan = %s %q err=%v, want planned noop service api, create service worker", waiting.PlanningOutcome, plannedActions(waiting), err)
	}
	return svc, ctx, res.Blueprint.ID, waiting
}

// assertApprovesTheWaitingPlan approves with no reviewed sync and checks that
// the waiting plan is what ran: its id, success, and the service it creates.
func assertApprovesTheWaitingPlan(t *testing.T, ctx context.Context, svc *Service, bpID string, waiting BlueprintPlanResult) {
	t.Helper()
	planned, err := svc.PlannedSync(ctx, bpID, "tea-a")
	if err != nil || planned == nil || planned.ID != waiting.Sync.ID {
		t.Fatalf("planned sync = %+v err=%v, want the waiting plan %s", planned, err, waiting.Sync.ID)
	}
	applied, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", "", nil)
	if err != nil || applied.ID != waiting.Sync.ID || applied.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %+v err=%v, want the waiting plan %s applied", applied, err, waiting.Sync.ID)
	}
	if _, live := liveAndDeleting(t, svc.Client, "worker"); len(live) != 1 {
		t.Fatalf("worker Apps after approve = %v, want the one the waiting plan creates", live)
	}
}

// Sequence 1: a re-plan that finds no changes saves no sync, so the plan
// still waiting is the newest one and approve applies it, as the pinned CLI
// tells the user ("running approve now will approve that earlier plan").
// bex used to supersede the waiting plan (w2/m256 t014) and answer 409
// BLUEPRINT_NOTHING_TO_APPROVE.
func TestANoChangesReplanLeavesTheWaitingPlanForApprove(t *testing.T) {
	svc, ctx, bpID, waiting := blueprintWithAWaitingPlan(t)
	svc.GitFetcher = fakeBlueprintFetcher{contents: recreateManifest}
	again, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil || again.PlanningOutcome != PlanningNoChanges || again.Sync != nil {
		t.Fatalf("re-plan = %+v err=%v, want no_changes with no sync", again, err)
	}
	// The fake fetcher ignores the commit: serve the waiting plan's file for
	// its approval.
	svc.GitFetcher = fakeBlueprintFetcher{contents: waitingManifest}
	assertApprovesTheWaitingPlan(t, ctx, svc, bpID, waiting)
	_, err = svc.ApproveBlueprint(ctx, bpID, "tea-a", "", nil)
	if err == nil || coded(t, err) != "BLUEPRINT_NOTHING_TO_APPROVE" || !strings.Contains(err.Error(), waiting.Sync.ID) {
		t.Fatalf("approve again: err = %v, want BLUEPRINT_NOTHING_TO_APPROVE naming the applied sync %s", err, waiting.Sync.ID)
	}
}

// Sequence 1 on a Blueprint still `created`: its first plan waits, the
// service it creates appears some other way, and the re-plan finds no
// changes. The Blueprint leaves `created` as any no-changes plan takes it
// (w2/m256 t014), and approve still applies the waiting first plan.
func TestANoChangesReplanOfACreatedBlueprintLeavesItsFirstPlanForApprove(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, recreateManifest)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "s", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil || res.PlanningOutcome != PlanningPlanned {
		t.Fatalf("create = %+v err=%v, want planned", res, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: recreateManifest}); err != nil {
		t.Fatal(err)
	}
	plan, err := svc.PlanBlueprint(ctx, res.Blueprint.ID, "tea-a")
	if err != nil || plan.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("plan = %+v err=%v, want no_changes", plan, err)
	}
	if got, err := svc.PlannedSync(ctx, res.Blueprint.ID, "tea-a"); err != nil || got == nil || got.ID != res.Sync.ID {
		t.Fatalf("planned sync = %+v err=%v, want the first plan %s still waiting", got, err, res.Sync.ID)
	}
	applied, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", res.Sync.ID, nil)
	if err != nil || applied.ID != res.Sync.ID || applied.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %+v err=%v, want the first plan %s applied", applied, err, res.Sync.ID)
	}
	if b := st.blueprints[ids.BlueprintStorageID(res.Blueprint.ID)]; b.Status != store.BlueprintStatusInSync {
		t.Errorf("status after approval = %q, want in_sync", b.Status)
	}
	if _, err := svc.SyncBlueprint(ctx, res.Blueprint.ID, "tea-a", "", "", nil); err != nil {
		t.Fatalf("a manual sync after the approval: %v", err)
	}
}

// Sequence 2: a failed plan that saves no sync. Every `failed` outcome bex
// answers carries the sync it saved (sequences 3 and 4), so the only way to
// get there is a plan request that fails outright, for which the CLI reports
// a failed planning outcome with no sync. Nothing was saved, so the waiting
// plan is still the newest and approve applies it, as the CLI warns
// ("approving now could apply that earlier plan").
func TestAPlanRequestThatFailsLeavesTheWaitingPlanForApprove(t *testing.T) {
	svc, ctx, bpID, waiting := blueprintWithAWaitingPlan(t)
	failing := &atomic.Bool{}
	svc.Client = failingAppLists{Client: svc.Client, failing: failing}
	failing.Store(true)
	if res, err := svc.PlanBlueprint(ctx, bpID, "tea-a"); !errors.Is(err, errAppListDown) {
		t.Fatalf("plan with the workspace unreadable = %+v err=%v, want the request to fail", res, err)
	}
	failing.Store(false)
	assertApprovesTheWaitingPlan(t, ctx, svc, bpID, waiting)
}

// Sequence 3: a failed re-plan saves its sync in the `error` state, which is
// the newest one instead, and approve refuses it, as the pinned CLI says. The
// waiting plan is not applied.
func TestAFailedReplanThatSavesItsSyncIsWhatApproveRefuses(t *testing.T) {
	svc, ctx, bpID, waiting := blueprintWithAWaitingPlan(t)
	svc.GitFetcher = fakeBlueprintFetcher{contents: recreateManifest + "  - type: web\n    name: worker\n    runtime: nope\n"}
	failed, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil || failed.PlanningOutcome != PlanningFailed || failed.Sync == nil || failed.Sync.State != store.BlueprintSyncStateError {
		t.Fatalf("re-plan of an invalid file = %+v err=%v, want failed with an error sync saved", failed, err)
	}
	assertRefusesTheFailedPlan(t, ctx, svc, bpID, failed, waiting)
}

// Sequence 4: `update --path` to a file that is not there, whose re-plan
// fails. The failed plan is saved, so it is the newest one and approve
// refuses it, as NewUpdateOut says ("A failed sync saved by this re-plan is
// the newest one instead, and approve refuses it").
func TestAnUpdatedPathWhoseReplanFailsIsWhatApproveRefuses(t *testing.T) {
	svc, ctx, bpID, waiting := blueprintWithAWaitingPlan(t)
	if _, err := svc.UpdateBlueprint(ctx, bpID, "tea-a", UpdateBlueprintRequest{Path: new("nope.yaml")}); err != nil {
		t.Fatal(err)
	}
	failed, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil || failed.PlanningOutcome != PlanningFailed || failed.Sync == nil || failed.Sync.State != store.BlueprintSyncStateError {
		t.Fatalf("re-plan at a missing path = %+v err=%v, want failed with an error sync saved", failed, err)
	}
	assertRefusesTheFailedPlan(t, ctx, svc, bpID, failed, waiting)
}

// assertRefusesTheFailedPlan checks approve answers 409
// BLUEPRINT_SYNC_PLANNING_FAILED for the saved failed plan, and that the
// waiting plan it replaced as the newest is not approvable.
func assertRefusesTheFailedPlan(t *testing.T, ctx context.Context, svc *Service, bpID string, failed, waiting BlueprintPlanResult) {
	t.Helper()
	if got, err := svc.PlannedSync(ctx, bpID, "tea-a"); err != nil || got != nil {
		t.Fatalf("planned sync = %+v err=%v, want none: the newest plan failed", got, err)
	}
	for _, reviewed := range []string{"", waiting.Sync.ID} {
		_, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", reviewed, nil)
		var c *core.CodedError
		if !errors.As(err, &c) || c.Code != "BLUEPRINT_SYNC_PLANNING_FAILED" || c.Params["syncId"] != failed.Sync.ID || !errors.Is(err, core.ErrConflict) {
			t.Fatalf("approve (reviewed %q): err = %#v, want 409 BLUEPRINT_SYNC_PLANNING_FAILED for %s", reviewed, err, failed.Sync.ID)
		}
	}
	if _, live := liveAndDeleting(t, svc.Client, "worker"); len(live) != 0 {
		t.Fatalf("worker Apps = %v, want none: the waiting plan was not applied", live)
	}
}

// TestANoChangesReplanLeavesTheWaitingPlanForApprovePG replays sequence 1 on
// real Postgres, where the no-changes stamp (w32/071) is ordered against the
// waiting plan's created_at.
func TestANoChangesReplanLeavesTheWaitingPlanForApprovePG(t *testing.T) {
	svc, st, _, ctx, wsID := newRecreatePGService(t)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: wsID, Source: RenderBlueprintSource{Repo: "https://github.com/acme/waiting-" + wsID}})
	if err != nil {
		t.Fatal(err)
	}
	id := res.Blueprint.ID
	if _, err := svc.ApproveBlueprint(ctx, id, wsID, "", nil); err != nil {
		t.Fatal(err)
	}
	svc.GitFetcher = fakeBlueprintFetcher{contents: waitingManifest}
	waiting, err := svc.PlanBlueprint(ctx, id, wsID)
	if err != nil || waiting.PlanningOutcome != PlanningPlanned {
		t.Fatalf("plan = %+v err=%v, want planned", waiting, err)
	}
	svc.GitFetcher = fakeBlueprintFetcher{contents: recreateManifest}
	if again, err := svc.PlanBlueprint(ctx, id, wsID); err != nil || again.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("re-plan = %+v err=%v, want no_changes", again, err)
	}
	if b, err := st.GetBlueprint(ctx, id, wsID); err != nil || b.NoChangesPlannedAt == nil {
		t.Fatalf("blueprint = %+v err=%v, want the no-changes plan stamped", b, err)
	}
	if got, err := svc.PlannedSync(ctx, id, wsID); err != nil || got == nil || got.ID != waiting.Sync.ID {
		t.Fatalf("planned sync = %+v err=%v, want the waiting plan %s", got, err, waiting.Sync.ID)
	}
	svc.GitFetcher = fakeBlueprintFetcher{contents: waitingManifest}
	applied, err := svc.ApproveBlueprint(ctx, id, wsID, waiting.Sync.ID, nil)
	if err != nil || applied.ID != waiting.Sync.ID || applied.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %+v err=%v, want the waiting plan %s applied", applied, err, waiting.Sync.ID)
	}
	// Approving it cleared the no-changes stamp, so approving again names the
	// applied plan rather than the no-changes plan before it.
	if b, err := st.GetBlueprint(ctx, id, wsID); err != nil || b.NoChangesPlannedAt != nil {
		t.Fatalf("blueprint = %+v err=%v, want the no-changes stamp cleared by the approval", b, err)
	}
	_, err = svc.ApproveBlueprint(ctx, id, wsID, "", nil)
	if err == nil || coded(t, err) != "BLUEPRINT_NOTHING_TO_APPROVE" || !strings.Contains(err.Error(), waiting.Sync.ID) {
		t.Fatalf("approve again: err = %v, want BLUEPRINT_NOTHING_TO_APPROVE naming the applied sync %s", err, waiting.Sync.ID)
	}
}
