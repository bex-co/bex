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
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w32/075 pins the expiry of a planned sync to the pinned Render CLI
// (render-oss/cli 4fd34231b637): CreateBlueprintSync, "A planned Sync that is
// not approved within 24 hours will be automatically deleted", and `ea
// blueprints plan`, "A planned sync that is not approved within 24 hours is
// deleted". Approve takes "the newest Sync in the `created` state", so once
// the plan is gone the attempt before it answers, and a Blueprint with none
// has nothing to approve.

// planExpiryStart is when each test plans: the service clock starts here.
var planExpiryStart = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

// expiryClock sets svc's clock to planExpiryStart and returns a setter for
// the time elapsed since then.
func expiryClock(svc *Service) func(time.Duration) {
	now := planExpiryStart
	svc.Base.Clock = func() time.Time { return now }
	return func(elapsed time.Duration) { now = planExpiryStart.Add(elapsed) }
}

// plannedSyncGraphQL is GraphQL blueprintPlannedSync's id, "" for null.
func plannedSyncGraphQL(t *testing.T, ctx context.Context, svc *Service, bpID string) string {
	t.Helper()
	out := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: ctx,
		RequestString: fmt.Sprintf(`{ blueprintPlannedSync(id:%q, ownerId:"tea-a") { id } }`, bpID)})
	if len(out.Errors) != 0 {
		t.Fatalf("blueprintPlannedSync: %v", out.Errors)
	}
	planned, _ := out.Data.(map[string]any)["blueprintPlannedSync"].(map[string]any)
	id, _ := planned["id"].(string)
	return id
}

// approveBlueprintMCP calls MCP approve_blueprint and returns its error text,
// "" when it approved.
func approveBlueprintMCP(t *testing.T, ctx context.Context, svc *Service, bpID string) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "w32-075", Version: "0"}, nil)
	svc.RegisterMCP(server)
	a, b := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, a, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "w32-075", Version: "0"}, nil).Connect(ctx, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "approve_blueprint", Arguments: map[string]any{"blueprintId": bpID}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		return ""
	}
	if text, ok := res.Content[0].(*mcp.TextContent); ok && text.Text != "" {
		return text.Text
	}
	return "refused"
}

// assertNothingToApprove checks err is 409 BLUEPRINT_NOTHING_TO_APPROVE
// containing want, with params.syncId naming syncID ("" for none).
func assertNothingToApprove(t *testing.T, err error, want, syncID string) {
	t.Helper()
	var c *core.CodedError
	if !errors.As(err, &c) || c.Code != "BLUEPRINT_NOTHING_TO_APPROVE" || !errors.Is(err, core.ErrConflict) ||
		!strings.Contains(c.Error(), want) {
		t.Fatalf("approve: err = %#v, want 409 BLUEPRINT_NOTHING_TO_APPROVE containing %q", err, want)
	}
	if got, _ := c.Params["syncId"].(string); got != syncID {
		t.Fatalf("approve: params.syncId = %q, want %q", got, syncID)
	}
}

// A `created` Blueprint's first plan, 23 hours old, is still the plan waiting
// and approve applies it.
func TestAPlanStillWaitingAfter23HoursIsApproved(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, recreateManifest)
	elapse := expiryClock(svc)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "s", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil || res.PlanningOutcome != PlanningPlanned {
		t.Fatalf("create = %+v err=%v, want planned", res, err)
	}
	elapse(23 * time.Hour)
	if got := plannedSyncGraphQL(t, ctx, svc, res.Blueprint.ID); got != res.Sync.ID {
		t.Fatalf("blueprintPlannedSync after 23h = %q, want the plan %s", got, res.Sync.ID)
	}
	applied, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", res.Sync.ID, nil)
	if err != nil || applied.ID != res.Sync.ID || applied.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve after 23h = %+v err=%v, want the plan %s applied", applied, err, res.Sync.ID)
	}
	if b := st.blueprints[ids.BlueprintStorageID(res.Blueprint.ID)]; b.Status != store.BlueprintStatusInSync {
		t.Errorf("status after the approval = %q, want in_sync", b.Status)
	}
}

// The same first plan 24 hours old is deleted: GraphQL, the service and MCP
// show no plan, approve finds nothing to approve on every surface, and
// nothing is applied. The Blueprint stays `created`, and planning again is
// the way forward, as on Render.
func TestAFirstPlanNotApprovedWithin24HoursIsGone(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, recreateManifest)
	elapse := expiryClock(svc)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "s", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil || res.PlanningOutcome != PlanningPlanned {
		t.Fatalf("create = %+v err=%v, want planned", res, err)
	}
	bpID := res.Blueprint.ID
	elapse(24 * time.Hour)
	if got, err := svc.PlannedSync(ctx, bpID, "tea-a"); err != nil || got != nil {
		t.Fatalf("planned sync after 24h = %+v err=%v, want none", got, err)
	}
	if got := plannedSyncGraphQL(t, ctx, svc, bpID); got != "" {
		t.Fatalf("blueprintPlannedSync after 24h = %q, want null", got)
	}
	for _, reviewed := range []string{"", res.Sync.ID} {
		_, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", reviewed, nil)
		assertNothingToApprove(t, err, "has no planned sync", "")
	}
	if refusal := approveBlueprintMCP(t, ctx, svc, bpID); !strings.Contains(refusal, "has no planned sync") {
		t.Fatalf("MCP approve_blueprint after 24h = %q, want the no-planned-sync refusal", refusal)
	}
	if _, live := liveAndDeleting(t, svc.Client, "api"); len(live) != 0 {
		t.Fatalf("api Apps = %v, want none: the expired plan was not applied", live)
	}
	if b := st.blueprints[ids.BlueprintStorageID(bpID)]; b.Status != store.BlueprintStatusCreated {
		t.Fatalf("status after the plan expired = %q, want created", b.Status)
	}

	again, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil || again.PlanningOutcome != PlanningPlanned || again.Sync == nil || again.Sync.ID == res.Sync.ID {
		t.Fatalf("re-plan after the expiry = %+v err=%v, want a new planned sync", again, err)
	}
	applied, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", again.Sync.ID, nil)
	if err != nil || applied.ID != again.Sync.ID || applied.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve of the new plan = %+v err=%v, want %s applied", applied, err, again.Sync.ID)
	}
}

// A re-plan of an approved Blueprint left waiting 24 hours is deleted, so the
// applied sync before it is the newest, and approve answers that it is not
// awaiting approval. The service the expired plan would create is not made.
func TestAWaitingReplanNotApprovedWithin24HoursIsGone(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, recreateManifest)
	elapse := expiryClock(svc)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "s", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	bpID := res.Blueprint.ID
	first, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	elapse(time.Hour)
	svc.GitFetcher = fakeBlueprintFetcher{contents: waitingManifest}
	waiting, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil || waiting.PlanningOutcome != PlanningPlanned {
		t.Fatalf("re-plan = %+v err=%v, want planned", waiting, err)
	}
	elapse(time.Hour + 24*time.Hour - time.Second)
	if got := plannedSyncGraphQL(t, ctx, svc, bpID); got != waiting.Sync.ID {
		t.Fatalf("blueprintPlannedSync a second before 24h = %q, want the waiting plan %s", got, waiting.Sync.ID)
	}
	elapse(time.Hour + 24*time.Hour)
	if got := plannedSyncGraphQL(t, ctx, svc, bpID); got != "" {
		t.Fatalf("blueprintPlannedSync after 24h = %q, want null", got)
	}
	_, err = svc.ApproveBlueprint(ctx, bpID, "tea-a", waiting.Sync.ID, nil)
	assertNothingToApprove(t, err, "is success, not awaiting approval", first.ID)
	if refusal := approveBlueprintMCP(t, ctx, svc, bpID); !strings.Contains(refusal, first.ID+" is success, not awaiting approval") {
		t.Fatalf("MCP approve_blueprint after 24h = %q, want the applied sync %s named", refusal, first.ID)
	}
	if _, live := liveAndDeleting(t, svc.Client, "worker"); len(live) != 0 {
		t.Fatalf("worker Apps = %v, want none: the expired plan was not applied", live)
	}
}

// A first plan waiting behind a no-changes re-plan (w32/073) that expires
// leaves the no-changes plan as the newest, so approve gives w32/071's
// sentence.
func TestAWaitingPlanBehindANoChangesPlanExpiresToNoChanges(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, recreateManifest)
	elapse := expiryClock(svc)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "s", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil || res.PlanningOutcome != PlanningPlanned {
		t.Fatalf("create = %+v err=%v, want planned", res, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: recreateManifest}); err != nil {
		t.Fatal(err)
	}
	if plan, err := svc.PlanBlueprint(ctx, res.Blueprint.ID, "tea-a"); err != nil || plan.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("plan = %+v err=%v, want no_changes", plan, err)
	}
	elapse(24 * time.Hour)
	_, err = svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", res.Sync.ID, nil)
	assertNoChangesToApprove(t, err)
}

// The recovery sweep deletes a plan left waiting 24 hours, as Render does,
// and leaves one 23 hours old and every sync that is not waiting.
func TestBlueprintRecovererDeletesExpiredPlans(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, recreateManifest)
	elapse := expiryClock(svc)
	create := func(name string) BlueprintPlanResult {
		t.Helper()
		res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: name, OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/" + name}})
		if err != nil || res.PlanningOutcome != PlanningPlanned {
			t.Fatalf("create %s = %+v err=%v, want planned", name, res, err)
		}
		return res
	}
	expired := create("expired")
	applied := create("applied")
	elapse(time.Hour)
	// Planned before the approval below, which makes api applied's.
	fresh := create("fresh")
	if _, err := svc.ApproveBlueprint(ctx, applied.Blueprint.ID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}

	r := &BlueprintRecoverer{Store: st, Now: func() time.Time { return planExpiryStart.Add(24 * time.Hour) }}
	r.recoverOnce(context.Background())
	if _, ok := st.syncs[ids.BlueprintSyncStorageID(expired.Sync.ID)]; ok {
		t.Errorf("the plan left waiting 24h is still stored")
	}
	if run, ok := st.syncs[ids.BlueprintSyncStorageID(fresh.Sync.ID)]; !ok || run.State != store.BlueprintSyncStateCreated {
		t.Errorf("the plan waiting 23h = %+v (stored %v), want it kept", run, ok)
	}
	if run, ok := st.syncs[ids.BlueprintSyncStorageID(applied.Sync.ID)]; !ok || run.State != store.BlueprintSyncStateSuccess {
		t.Errorf("the applied sync = %+v (stored %v), want it kept", run, ok)
	}
}

// The expiry on real Postgres, on the service clock: the plan is made a day
// back, approve refuses it once 24 hours have passed, the sweep deletes it,
// and planning again is the way forward. The sync history lists the plan at
// no point while it waits, as Render's never lists a `created` sync (w32/077).
func TestAPlanNotApprovedWithin24HoursIsGonePG(t *testing.T) {
	svc, st, pool, ctx, wsID := newRecreatePGService(t)
	// Planned a day back, so the sweep at now finds no other test's plan
	// expired.
	start := time.Now().UTC().Add(-store.BlueprintPlanApprovalWindow)
	now := start
	svc.Base.Clock = func() time.Time { return now }
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: wsID, Source: RenderBlueprintSource{Repo: "https://github.com/acme/expiry-" + wsID}})
	if err != nil || res.PlanningOutcome != PlanningPlanned {
		t.Fatalf("create = %+v err=%v, want planned", res, err)
	}
	id := res.Blueprint.ID
	now = start.Add(23 * time.Hour)
	if got, err := svc.PlannedSync(ctx, id, wsID); err != nil || got == nil || got.ID != res.Sync.ID {
		t.Fatalf("planned sync after 23h = %+v err=%v, want %s", got, err, res.Sync.ID)
	}
	now = start.Add(24 * time.Hour)
	if got, err := svc.PlannedSync(ctx, id, wsID); err != nil || got != nil {
		t.Fatalf("planned sync after 24h = %+v err=%v, want none", got, err)
	}
	_, err = svc.ApproveBlueprint(ctx, id, wsID, res.Sync.ID, nil)
	assertNothingToApprove(t, err, "has no planned sync", "")

	listed := func() []string {
		t.Helper()
		syncs, err := svc.ListBlueprintSyncs(ctx, id, wsID, "", 20)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(syncs))
		for _, s := range syncs {
			out = append(out, s.ID)
		}
		return out
	}
	if got := listed(); len(got) != 0 {
		t.Fatalf("sync history before the sweep = %v, want none: a waiting plan is not listed", got)
	}
	(&BlueprintRecoverer{Store: st, Now: func() time.Time { return now }}).recoverOnce(context.Background())
	var stored int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blueprint_syncs WHERE id = $1`, ids.BlueprintSyncStorageID(res.Sync.ID)).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("the expired plan after the sweep: %d rows (%v), want it deleted", stored, err)
	}
	if b, err := st.GetBlueprint(ctx, id, wsID); err != nil || b.Status != store.BlueprintStatusCreated {
		t.Fatalf("blueprint after the sweep = %+v err=%v, want it still created", b, err)
	}

	again, err := svc.PlanBlueprint(ctx, id, wsID)
	if err != nil || again.PlanningOutcome != PlanningPlanned {
		t.Fatalf("re-plan = %+v err=%v, want planned", again, err)
	}
	if got := listed(); len(got) != 0 {
		t.Fatalf("sync history with the re-plan waiting = %v, want none", got)
	}
	applied, err := svc.ApproveBlueprint(ctx, id, wsID, again.Sync.ID, nil)
	if err != nil || applied.ID != again.Sync.ID || applied.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve of the new plan = %+v err=%v, want %s applied", applied, err, again.Sync.ID)
	}
	if got := listed(); len(got) != 1 || got[0] != again.Sync.ID {
		t.Fatalf("sync history once approved = %v, want the applied plan %s", got, again.Sync.ID)
	}
}
