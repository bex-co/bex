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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// --- fakeBlueprintStore: blueprintPlanStore (w2/m256) ---

func (f *fakeBlueprintStore) CreatePlannedBlueprint(_ context.Context, b store.Blueprint) (store.Blueprint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.blueprints {
		if existing.TenantID == b.TenantID && existing.Repo == b.Repo && existing.Branch == b.Branch && existing.Status != "disconnected" {
			return store.Blueprint{}, fmt.Errorf("blueprint: %w", store.ErrConflict)
		}
	}
	if b.ID == "" {
		b.ID = fmt.Sprintf("blp-fake%016d", len(f.blueprints))
	}
	b.Status, b.ExecutionGeneration = store.BlueprintStatusCreated, 1
	if b.ExistingResources == store.BlueprintExistingCreateNew && b.ResourceSuffix == "" {
		b.ResourceSuffix = "t3st" // the PG store mints a random one
	}
	f.blueprints[b.ID] = b
	return b, nil
}

func (f *fakeBlueprintStore) BlueprintNameTaken(_ context.Context, tenantID, name, exceptID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, b := range f.blueprints {
		if b.TenantID == tenantID && strings.EqualFold(b.Name, name) && b.ID != exceptID &&
			b.Status != "disconnected" && b.Status != store.BlueprintStatusCreated {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeBlueprintStore) InsertPlannedBlueprintSync(_ context.Context, p store.PlannedBlueprintSync) (store.PlannedBlueprintSync, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p.ID == "" {
		p.ID = fmt.Sprintf("exe-fake%016d", len(f.insertedSyncs))
	}
	if f.planned == nil {
		f.planned = map[string]store.PlannedBlueprintSync{}
	}
	p.CreatedAt = time.Now()
	f.planned[p.ID] = p
	f.syncs[p.ID] = p.BlueprintSync
	f.insertedSyncs = append(f.insertedSyncs, p.BlueprintSync)
	return p, nil
}

func (f *fakeBlueprintStore) LatestBlueprintPlan(_ context.Context, blueprintID string) (store.PlannedBlueprintSync, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.insertedSyncs) - 1; i >= 0; i-- {
		id := f.insertedSyncs[i].ID
		p, planned := f.planned[id]
		run := f.syncs[id]
		if !planned || run.BlueprintID != blueprintID {
			continue
		}
		p.BlueprintSync = run
		return p, nil
	}
	return store.PlannedBlueprintSync{}, fmt.Errorf("blueprint_sync: %w", store.ErrNotFound)
}

func (f *fakeBlueprintStore) BlueprintPlannedSync(_ context.Context, blueprintID, syncID string) (store.PlannedBlueprintSync, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	run, ok := f.syncs[syncID]
	if !ok || run.BlueprintID != blueprintID {
		return store.PlannedBlueprintSync{}, fmt.Errorf("blueprint_sync: %w", store.ErrNotFound)
	}
	p := f.planned[syncID]
	p.BlueprintSync = run
	return p, nil
}

// onboardingManifest declares one web service with one sync:false prompt.
const onboardingManifest = `
services:
  - type: web
    name: api
    runtime: image
    image:
      url: docker.io/library/nginx:1.27
    envVars:
      - {key: API_TOKEN, sync: false}
`

func newOnboardingService(t *testing.T, manifest string) (*Service, *fakeBlueprintStore, context.Context) {
	t.Helper()
	st := newFakeBlueprintStore()
	svc := newBlueprintService(st, fakeWorkspace{"user-a": "tea-a"})
	svc.GitFetcher = fakeBlueprintFetcher{contents: manifest}
	svc.EnvSeeder = &fakeSeeder{}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	return svc, st, ctx
}

func coded(t *testing.T, err error) string {
	t.Helper()
	var c *core.CodedError
	if !errors.As(err, &c) {
		t.Fatalf("err = %v, want a coded error", err)
	}
	return c.Code
}

// w2/m256: Render's onboarding — create plans without applying, approve
// applies the planned sync with its manual env vars, and a sync that changes
// nothing is not saved.
func TestBlueprintOnboardingCreatePlanApprove(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, onboardingManifest)

	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{
		Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.PlanningOutcome != PlanningPlanned || res.Sync == nil || res.Sync.State != store.BlueprintSyncStateCreated {
		t.Fatalf("create = %+v, want a planned sync in the created state", res)
	}
	if res.Blueprint == nil || res.Blueprint.Status != store.BlueprintStatusCreated || res.Blueprint.OwnerID != "tea-a" || res.Blueprint.Branch != DefaultBlueprintBranch {
		t.Fatalf("blueprint = %+v, want created on main owned by tea-a", res.Blueprint)
	}
	want := []ManualEnvVar{{ServiceType: "web", ServiceName: "api", Key: "API_TOKEN"}}
	if got := res.Sync.RequiredManualEnvVars; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("requiredManualEnvVars = %+v, want %+v", got, want)
	}
	var apps appv1alpha1.AppList
	if err := svc.Client.List(ctx, &apps); err != nil || len(apps.Items) != 0 {
		t.Fatalf("create applied resources: %d Apps, err %v", len(apps.Items), err)
	}
	bpID := res.Blueprint.ID

	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", nil); err == nil || coded(t, err) != "BLUEPRINT_MANUAL_ENV_VARS_MISSING" {
		t.Fatalf("approve without the prompt value: err = %v", err)
	}
	unknown := "x"
	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", []ManualEnvVar{{ServiceType: "web", ServiceName: "other", Key: "API_TOKEN", Value: &unknown}}); err == nil || coded(t, err) != "BLUEPRINT_MANUAL_ENV_VAR_UNKNOWN" {
		t.Fatalf("approve with an unknown value: err = %v", err)
	}

	token := "s3cret"
	approved, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", []ManualEnvVar{{ServiceType: "web", ServiceName: "api", Key: "API_TOKEN", Value: &token}})
	if err != nil {
		t.Fatal(err)
	}
	if approved.ID != res.Sync.ID || approved.State != store.BlueprintSyncStateSuccess || approved.Commit.ID != res.Sync.Commit.ID {
		t.Fatalf("approved = %+v, want the planned sync %s applied", approved, res.Sync.ID)
	}
	if err := svc.Client.List(ctx, &apps); err != nil || len(apps.Items) != 1 {
		t.Fatalf("approve created %d Apps (err %v), want 1", len(apps.Items), err)
	}
	if b := st.blueprints[bpID]; b.Status != store.BlueprintStatusInSync {
		t.Errorf("status after approval = %q, want in_sync", b.Status)
	}
	seeds := svc.EnvSeeder.(*fakeSeeder).seeds
	if len(seeds) != 1 || seeds[0].literals["API_TOKEN"] != token {
		t.Errorf("seeded = %+v, want API_TOKEN=%q on api", seeds, token)
	}

	// Re-planning the unchanged file finds nothing to approve.
	again, err := svc.PlanBlueprint(ctx, bpID, "tea-a")
	if err != nil || again.PlanningOutcome != PlanningNoChanges || again.Sync != nil {
		t.Fatalf("re-plan = %+v err=%v, want no_changes with no sync", again, err)
	}
	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", nil); err == nil || coded(t, err) != "BLUEPRINT_NOTHING_TO_APPROVE" {
		t.Fatalf("approve after an applied sync: err = %v", err)
	}
}

// w2/m261: create_new manages a suffixed copy of every resource beside the
// same-named ones another Blueprint already manages, and keeps managing that
// copy on later plans.
func TestBlueprintOnboardingCreateNewManagesASuffixedCopy(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, onboardingManifest)
	token := "s3cret"
	answer := func(service string) []ManualEnvVar {
		return []ManualEnvVar{{ServiceType: "web", ServiceName: service, Key: "API_TOKEN", Value: &token}}
	}
	adopted, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "first", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveBlueprint(ctx, adopted.Blueprint.ID, "tea-a", answer("api")); err != nil {
		t.Fatal(err)
	}
	original := &appv1alpha1.App{}
	if err := svc.Client.Get(ctx, appKey(t, svc, "api"), original); err != nil {
		t.Fatalf("adopt Blueprint's api: %v", err)
	}

	createNew := store.BlueprintExistingCreateNew
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "second", OwnerID: "tea-a",
		Source: RenderBlueprintSource{Repo: "https://github.com/a/copy"}, ExistingResources: &createNew})
	if err != nil {
		t.Fatal(err)
	}
	b := st.blueprints[res.Blueprint.ID]
	if b.ExistingResources != store.BlueprintExistingCreateNew || b.ResourceSuffix == "" {
		t.Fatalf("stored blueprint = %+v, want create_new with a suffix", b)
	}
	copyName := "api-" + b.ResourceSuffix
	want := ManualEnvVar{ServiceType: "web", ServiceName: copyName, Key: "API_TOKEN"}
	if res.PlanningOutcome != PlanningPlanned || len(res.Sync.RequiredManualEnvVars) != 1 || res.Sync.RequiredManualEnvVars[0] != want {
		t.Fatalf("plan = %+v, want the suffixed service's manual env var %+v", res, want)
	}
	if _, err := svc.ApproveBlueprint(ctx, b.ID, "tea-a", answer(copyName)); err != nil {
		t.Fatal(err)
	}
	var apps appv1alpha1.AppList
	if err := svc.Client.List(ctx, &apps); err != nil || len(apps.Items) != 2 {
		t.Fatalf("Apps after the create_new approval = %d (err %v), want api and %s", len(apps.Items), err, copyName)
	}
	if err := svc.Client.Get(ctx, appKey(t, svc, copyName), &appv1alpha1.App{}); err != nil {
		t.Fatalf("suffixed copy: %v", err)
	}
	untouched := &appv1alpha1.App{}
	if err := svc.Client.Get(ctx, appKey(t, svc, "api"), untouched); err != nil || untouched.ResourceVersion != original.ResourceVersion {
		t.Fatalf("the adopt Blueprint's api changed: resourceVersion %s → %s (err %v)", original.ResourceVersion, untouched.ResourceVersion, err)
	}
	// The suffix sticks: re-planning the same file changes nothing.
	again, err := svc.PlanBlueprint(ctx, b.ID, "tea-a")
	if err != nil || again.PlanningOutcome != PlanningNoChanges {
		t.Fatalf("re-plan = %+v err=%v, want no_changes", again, err)
	}
}

func TestBlueprintOnboardingFailedPlanCannotBeApproved(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, onboardingManifest)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	svc.GitFetcher = fakeBlueprintFetcher{contents: "services:\n  - type: web\n    name: api\n    runtime: nope\n"}
	failed, err := svc.PlanBlueprint(ctx, res.Blueprint.ID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if failed.PlanningOutcome != PlanningFailed || len(failed.Notices) == 0 || failed.Notices[0].Severity != "error" ||
		failed.Sync == nil || failed.Sync.State != store.BlueprintSyncStateError {
		t.Fatalf("plan = %+v, want failed with an error notice and an error sync", failed)
	}
	if _, err := svc.ApproveBlueprint(ctx, res.Blueprint.ID, "tea-a", nil); err == nil || coded(t, err) != "BLUEPRINT_SYNC_PLANNING_FAILED" {
		t.Fatalf("approve after a failed plan: err = %v", err)
	}
	if b := st.blueprints[res.Blueprint.ID]; b.Status != store.BlueprintStatusCreated {
		t.Errorf("a refused approval changed the status to %q", b.Status)
	}
}

func TestBlueprintOnboardingRefusals(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, onboardingManifest)
	st.blueprints["blp-existing0000000001"] = store.Blueprint{ID: "blp-existing0000000001", TenantID: "tea-a", Name: "Stack",
		Repo: "https://github.com/a/other", Branch: "main", Status: store.BlueprintStatusInSync}
	_, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err == nil || coded(t, err) != "BLUEPRINT_NAME_TAKEN" {
		t.Fatalf("a name an approved Blueprint uses (any case): err = %v", err)
	}
	// A plan needs the workspace's current resources; without them it is
	// unavailable, never a false "no changes".
	blind, _, blindCtx := newOnboardingService(t, "envVarGroups:\n  - name: shared\n    envVars:\n      - {key: A, value: b}\n"+onboardingManifest)
	blind.EnvGroups = nil
	if _, err := blind.CreatePlannedBlueprint(blindCtx, RenderCreateBlueprintRequest{Name: "blind", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/blind"}}); err == nil || coded(t, err) != "BLUEPRINT_PLAN_UNAVAILABLE" {
		t.Fatalf("plan without current state: err = %v", err)
	}
	bogus := "replace"
	_, err = svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "fresh", OwnerID: "tea-a",
		Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}, ExistingResources: &bogus})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("existingResources outside adopt|create_new: err = %v", err)
	}
	for _, req := range []RenderCreateBlueprintRequest{
		{OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}},
		{Name: "x", OwnerID: "tea-a"},
	} {
		if _, err := svc.CreatePlannedBlueprint(ctx, req); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%+v: err = %v, want bad request", req, err)
		}
	}
}

// The REST face speaks Render's shapes and status codes: create 201, plan 201
// with a sync and 200 without, approve 200.
func TestBlueprintOnboardingREST(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, `
services:
  - type: web
    name: api
    runtime: image
    image:
      url: docker.io/library/nginx:1.27
`)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	call := func(method, path, body string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		if body != "" {
			req.ContentLength = int64(len(body))
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	code, created := call(http.MethodPost, "/v1/blueprints", `{"name":"stack","ownerId":"tea-a","source":{"repo":"https://github.com/a/app","branch":"main"}}`)
	if code != http.StatusCreated || created["planningOutcome"] != PlanningPlanned {
		t.Fatalf("create = %d %v", code, created)
	}
	bp := created["blueprint"].(map[string]any)
	id := bp["id"].(string)
	if bp["status"] != "created" || bp["ownerId"] != "tea-a" {
		t.Fatalf("blueprint = %v", bp)
	}
	code, approved := call(http.MethodPost, "/v1/blueprints/"+id+"/approve", `{}`)
	if code != http.StatusOK || approved["state"] != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %d %v", code, approved)
	}
	code, planned := call(http.MethodPost, "/v1/blueprints/"+id+"/syncs", "")
	if code != http.StatusOK || planned["planningOutcome"] != PlanningNoChanges || planned["sync"] != nil {
		t.Fatalf("plan = %d %v, want 200 no_changes with a null sync", code, planned)
	}
	// bex's earlier create body still creates and applies at once.
	svc.GitFetcher = fakeBlueprintFetcher{contents: "services:\n  - type: web\n    name: legacy\n    runtime: image\n    image:\n      url: docker.io/library/nginx:1.27\n"}
	code, legacy := call(http.MethodPost, "/v1/blueprints", `{"repo":"https://github.com/a/legacy","branch":"main","ownerId":"tea-a"}`)
	if code != http.StatusCreated || legacy["status"] == "created" {
		t.Fatalf("legacy create = %d %v", code, legacy)
	}
}

// appKey finds the App the Blueprint named service in svc's fake cluster.
func appKey(t *testing.T, svc *Service, service string) client.ObjectKey {
	t.Helper()
	var apps appv1alpha1.AppList
	if err := svc.Client.List(context.Background(), &apps); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range apps.Items {
		if a.Name == service || a.Name == core.CRName("tea-a", service) {
			return client.ObjectKeyFromObject(&a)
		}
		names = append(names, a.Namespace+"/"+a.Name)
	}
	t.Fatalf("no App for service %q among %v", service, names)
	return client.ObjectKey{}
}
