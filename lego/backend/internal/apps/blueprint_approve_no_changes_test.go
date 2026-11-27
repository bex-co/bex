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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// noChangesToApprove is the refusal for a Blueprint whose newest plan found
// no changes (w32/071).
const noChangesToApprove = "the newest plan of this Blueprint found no changes; there is nothing to approve"

// assertNoChangesToApprove checks err is that refusal: 409
// BLUEPRINT_NOTHING_TO_APPROVE in the no-changes wording, naming no sync,
// because none is pending.
func assertNoChangesToApprove(t *testing.T, err error) {
	t.Helper()
	var c *core.CodedError
	if !errors.As(err, &c) || c.Code != "BLUEPRINT_NOTHING_TO_APPROVE" || c.Error() != noChangesToApprove ||
		!errors.Is(err, core.ErrConflict) || len(c.Params) != 0 {
		t.Fatalf("approve: err = %#v, want 409 BLUEPRINT_NOTHING_TO_APPROVE %q with no params", err, noChangesToApprove)
	}
}

// w32/071, the cli-qa run 0cd3ca repro over REST: a plan fails on a missing
// file, the fixed path plans no_changes (no sync row, as Render answers
// `sync: null`), and approve answers that there is nothing to approve. It used
// to answer BLUEPRINT_SYNC_PLANNING_FAILED for the stale failed plan, telling
// the user to fix the file and re-plan, which they had just done.
func TestApproveAfterAFailedPlanFixedWithNoChangesFindsNothingToApprove(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, recreateManifest)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	call := func(method, path, body string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		if body != "" {
			req.ContentLength = int64(len(body))
		}
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	syncCount := func(id string) int {
		t.Helper()
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/blueprints/"+id+"/syncs", nil).WithContext(ctx))
		var rows []any
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); rec.Code != http.StatusOK || err != nil {
			t.Fatalf("list syncs = %d %s (%v)", rec.Code, rec.Body, err)
		}
		return len(rows)
	}

	code, created := call(http.MethodPost, "/v1/blueprints", `{"name":"stack","ownerId":"tea-a","source":{"repo":"https://github.com/a/app","branch":"main"}}`)
	if code != http.StatusCreated || created["planningOutcome"] != PlanningPlanned {
		t.Fatalf("create = %d %v", code, created)
	}
	id := created["blueprint"].(map[string]any)["id"].(string)
	if code, approved := call(http.MethodPost, "/v1/blueprints/"+id+"/approve", `{}`); code != http.StatusOK || approved["state"] != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %d %v", code, approved)
	}

	// The plan at a missing path fails, and approving it is refused as a failed
	// plan, with the advice to fix the file and re-plan. An update refuses a
	// path with no file (w8/418), so the file goes after the update.
	withFileAt(svc, "nope.yaml", func() {
		if code, out := call(http.MethodPatch, "/v1/blueprints/"+id, `{"path":"nope.yaml"}`); code != http.StatusOK {
			t.Fatalf("update path = %d %v", code, out)
		}
	})
	code, failed := call(http.MethodPost, "/v1/blueprints/"+id+"/syncs", "")
	if code != http.StatusCreated || failed["planningOutcome"] != PlanningFailed {
		t.Fatalf("plan at a missing path = %d %v, want 201 failed", code, failed)
	}
	failedID := failed["sync"].(map[string]any)["id"].(string)
	code, refused := call(http.MethodPost, "/v1/blueprints/"+id+"/approve", `{}`)
	if code != http.StatusConflict || refused["code"] != "BLUEPRINT_SYNC_PLANNING_FAILED" ||
		refused["params"].(map[string]any)["syncId"] != failedID || !strings.Contains(refused["message"].(string), "fix the Blueprint file and re-plan") {
		t.Fatalf("approve after the failed plan = %d %v, want 409 BLUEPRINT_SYNC_PLANNING_FAILED for %s", code, refused, failedID)
	}

	// The fixed path finds no changes and records no sync.
	before := syncCount(id)
	if code, out := call(http.MethodPatch, "/v1/blueprints/"+id, `{"path":"render.yaml"}`); code != http.StatusOK {
		t.Fatalf("update path = %d %v", code, out)
	}
	code, fixed := call(http.MethodPost, "/v1/blueprints/"+id+"/syncs", "")
	if code != http.StatusOK || fixed["planningOutcome"] != PlanningNoChanges || fixed["sync"] != nil {
		t.Fatalf("plan at the fixed path = %d %v, want 200 no_changes with a null sync", code, fixed)
	}
	if after := syncCount(id); after != before {
		t.Fatalf("the no_changes plan listed %d syncs, %d before: it must add none", after, before)
	}
	code, refused = call(http.MethodPost, "/v1/blueprints/"+id+"/approve", `{}`)
	if code != http.StatusConflict || refused["code"] != "BLUEPRINT_NOTHING_TO_APPROVE" || refused["message"] != noChangesToApprove {
		t.Fatalf("approve after the no_changes plan = %d %v, want 409 BLUEPRINT_NOTHING_TO_APPROVE %q", code, refused, noChangesToApprove)
	}
	if params, _ := refused["params"].(map[string]any); params["syncId"] != nil {
		t.Fatalf("params = %v, want no syncId: no sync is pending", params)
	}
	// Binding the approval to the failed plan the caller saw changes nothing:
	// the newest plan found no changes either way.
	_, err := svc.ApproveBlueprint(ctx, id, "tea-a", failedID, nil)
	assertNoChangesToApprove(t, err)

	// A plan that fails after the no_changes one is the newest again.
	withFileAt(svc, "nope.yaml", func() {
		if _, err := svc.UpdateBlueprint(ctx, id, "tea-a", UpdateBlueprintRequest{Path: new("nope.yaml")}); err != nil {
			t.Fatal(err)
		}
	})
	again, err := svc.PlanBlueprint(ctx, id, "tea-a")
	if err != nil || again.PlanningOutcome != PlanningFailed {
		t.Fatalf("plan = %+v err=%v, want failed", again, err)
	}
	if _, err := svc.ApproveBlueprint(ctx, id, "tea-a", "", nil); err == nil || coded(t, err) != "BLUEPRINT_SYNC_PLANNING_FAILED" ||
		!strings.Contains(err.Error(), again.Sync.ID) {
		t.Fatalf("approve after a newer failed plan: err = %v, want BLUEPRINT_SYNC_PLANNING_FAILED for %s", err, again.Sync.ID)
	}
}

// A plan with changes made after a no_changes plan is approvable, and once
// applied, approving again names that applied sync, not the older no_changes
// plan (w32/071).
func TestAPlanAfterANoChangesPlanIsTheNewest(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, recreateManifest)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "s", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", "", nil); err != nil {
		t.Fatal(err)
	}
	if plan, err := svc.PlanBlueprint(ctx, res.Blueprint.ID, "tea-a"); err != nil || plan.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("plan = %+v err=%v, want no_changes", plan, err)
	}
	_, err = svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", "", nil)
	assertNoChangesToApprove(t, err)

	svc.GitFetcher = fakeBlueprintFetcher{contents: recreateManifest + `  - type: web
    name: worker
    runtime: image
    image:
      url: docker.io/library/nginx:1.27
`}
	planned, err := svc.PlanBlueprint(ctx, res.Blueprint.ID, "tea-a")
	if err != nil || planned.PlanningOutcome != PlanningPlanned || plannedActions(planned) != "noop service api, create service worker" {
		t.Fatalf("plan = %s %q err=%v, want planned noop service api, create service worker", planned.PlanningOutcome, plannedActions(planned), err)
	}
	applied, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", "", nil)
	if err != nil || applied.ID != planned.Sync.ID || applied.State != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %+v err=%v, want the plan %s applied", applied, err, planned.Sync.ID)
	}
	_, err = svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", "", nil)
	if err == nil || coded(t, err) != "BLUEPRINT_NOTHING_TO_APPROVE" || err.Error() == noChangesToApprove || !strings.Contains(err.Error(), applied.ID) {
		t.Fatalf("approve again: err = %v, want BLUEPRINT_NOTHING_TO_APPROVE naming the applied sync %s", err, applied.ID)
	}
}

// TestApproveAfterAFailedPlanFixedWithNoChangesPG replays the w32/071 repro on
// real Postgres, where the order of a no_changes plan against the planning
// attempts around it is the column it stamps against each sync's created_at.
func TestApproveAfterAFailedPlanFixedWithNoChangesPG(t *testing.T) {
	svc, st, pool, ctx, wsID := newRecreatePGService(t)
	repo := "https://github.com/acme/nochanges-approve-" + wsID
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "qa-bp", OwnerID: wsID, Source: RenderBlueprintSource{Repo: repo}})
	if err != nil {
		t.Fatal(err)
	}
	id := res.Blueprint.ID
	if _, err := svc.ApproveBlueprint(ctx, id, wsID, "", nil); err != nil {
		t.Fatal(err)
	}
	planAt := func(path, want string) BlueprintPlanResult {
		t.Helper()
		withFileAt(svc, path, func() {
			if _, err := svc.UpdateBlueprint(ctx, id, wsID, UpdateBlueprintRequest{Path: new(path)}); err != nil {
				t.Fatal(err)
			}
		})
		plan, err := svc.PlanBlueprint(ctx, id, wsID)
		if err != nil || plan.PlanningOutcome != want {
			t.Fatalf("plan at %s = %+v err=%v, want %s", path, plan, err, want)
		}
		return plan
	}
	failed := planAt("nope.yaml", PlanningFailed)
	if _, err := svc.ApproveBlueprint(ctx, id, wsID, "", nil); err == nil || coded(t, err) != "BLUEPRINT_SYNC_PLANNING_FAILED" {
		t.Fatalf("approve after the failed plan: err = %v, want BLUEPRINT_SYNC_PLANNING_FAILED", err)
	}

	var before, after int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blueprint_syncs WHERE blueprint_id = $1`, ids.BlueprintStorageID(id)).Scan(&before); err != nil {
		t.Fatal(err)
	}
	planAt("render.yaml", PlanningNoChanges)
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM blueprint_syncs WHERE blueprint_id = $1`, ids.BlueprintStorageID(id)).Scan(&after); err != nil || after != before {
		t.Fatalf("the no_changes plan wrote %d sync rows (err %v), want none", after-before, err)
	}
	b, err := st.GetBlueprint(ctx, id, wsID)
	if err != nil || b.NoChangesPlannedAt == nil {
		t.Fatalf("blueprint = %+v err=%v, want the no-changes plan stamped", b, err)
	}
	_, err = svc.ApproveBlueprint(ctx, id, wsID, "", nil)
	assertNoChangesToApprove(t, err)
	_, err = svc.ApproveBlueprint(ctx, id, wsID, failed.Sync.ID, nil)
	assertNoChangesToApprove(t, err)

	again := planAt("nope.yaml", PlanningFailed)
	if _, err := svc.ApproveBlueprint(ctx, id, wsID, "", nil); err == nil || coded(t, err) != "BLUEPRINT_SYNC_PLANNING_FAILED" ||
		!strings.Contains(err.Error(), again.Sync.ID) {
		t.Fatalf("approve after a newer failed plan: err = %v, want BLUEPRINT_SYNC_PLANNING_FAILED for %s", err, again.Sync.ID)
	}
}
