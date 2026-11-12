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

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// --- fakeBlueprintStore: blueprintPlanStore (w2/m256) ---

// CreatePlannedBlueprint mirrors the store (w8/m153): always a new row.
func (f *fakeBlueprintStore) CreatePlannedBlueprint(_ context.Context, b store.Blueprint) (store.Blueprint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if b.ID == "" {
		f.plannedCreates++
		b.ID = fmt.Sprintf("blp-fake%016d", f.plannedCreates)
	}
	if b.ExistingResources == "" {
		b.ExistingResources = store.BlueprintExistingAdopt
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now().UTC()
	}
	b.Status, b.ExecutionGeneration = store.BlueprintStatusCreated, 1
	if b.ExistingResources == store.BlueprintExistingCreateNew && b.ResourceSuffix == "" {
		// The PG store mints a random one; distinct per create here too.
		b.ResourceSuffix = fmt.Sprintf("t%03d", f.plannedCreates%1000)
	}
	f.blueprints[b.ID] = b
	return b, nil
}

func (f *fakeBlueprintStore) UndoPlannedBlueprintCreate(_ context.Context, created store.Blueprint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.blueprints, created.ID)
	return nil
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
		p.ID = ids.New(ids.BlueprintSync)
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

func (f *fakeBlueprintStore) SettleNoChangesPlan(_ context.Context, blueprintID, tenantID, manifest string) (store.Blueprint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.blueprints[blueprintID]
	if !ok || b.TenantID != tenantID || b.Status == "disconnected" {
		return store.Blueprint{}, fmt.Errorf("blueprint: %w", store.ErrNotFound)
	}
	for id, p := range f.planned {
		if run := f.syncs[id]; run.BlueprintID == blueprintID && run.State == store.BlueprintSyncStateCreated {
			msg := "superseded: a newer plan found no changes to apply"
			run.State, run.ErrorMessage = store.BlueprintSyncStateError, &msg
			f.syncs[id], p.BlueprintSync = run, run
			f.planned[id] = p
		}
	}
	if b.Status == store.BlueprintStatusCreated && b.ActiveRunID == "" {
		now := time.Now()
		b.Status, b.Manifest, b.LastSyncAt = store.BlueprintStatusInSync, manifest, &now
		f.blueprints[blueprintID] = b
	}
	return b, nil
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
	svc.Metadata = resourcemeta.Config{DashboardBaseURL: "https://dashboard.example"}

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
	// w2/m269: the create answers with Render's exs- spelling (the pinned CLI
	// refuses anything else) and links the dashboard page by it.
	if !strings.HasPrefix(bpID, "exs-") || !strings.HasSuffix(res.Sync.DashboardURL, "/blueprints/"+bpID) {
		t.Fatalf("blueprint id %q / dashboardUrl %q, want the exs- public id", bpID, res.Sync.DashboardURL)
	}

	if _, err := svc.ApproveBlueprint(ctx, bpID, "tea-a", nil); err == nil || coded(t, err) != "BLUEPRINT_MANUAL_ENV_VARS_MISSING" {
		t.Fatalf("approve without the prompt value: err = %v", err)
	}
	unknown := "x"
	// The durable blp- key addresses the same Blueprint as its exs- alias.
	if _, err := svc.ApproveBlueprint(ctx, ids.BlueprintStorageID(bpID), "tea-a", []ManualEnvVar{{ServiceType: "web", ServiceName: "other", Key: "API_TOKEN", Value: &unknown}}); err == nil || coded(t, err) != "BLUEPRINT_MANUAL_ENV_VAR_UNKNOWN" {
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
	if b := st.blueprints[ids.BlueprintStorageID(bpID)]; b.Status != store.BlueprintStatusInSync {
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
	b := st.blueprints[ids.BlueprintStorageID(res.Blueprint.ID)]
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
	if b := st.blueprints[ids.BlueprintStorageID(res.Blueprint.ID)]; b.Status != store.BlueprintStatusCreated {
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
		validated(t, mux).ServeHTTP(rec, req)
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

// w2/m256 t012: a created Blueprint applies only by approving its plan, so the
// manual sync refuses it by name on every surface and creates nothing.
func TestBlueprintManualSyncRefusesACreatedBlueprint(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			svc, st, ctx := newOnboardingService(t, onboardingManifest)
			res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
			if err != nil {
				t.Fatal(err)
			}
			id := res.Blueprint.ID
			var refusal string
			switch surface {
			case "REST":
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/"+id+"/sync", strings.NewReader(`{"ownerId":"tea-a"}`)).WithContext(ctx))
				if rec.Code != http.StatusConflict {
					t.Fatalf("sync = %d %s, want 409", rec.Code, rec.Body.String())
				}
				refusal = rec.Body.String()
			case "GraphQL":
				out := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: ctx,
					RequestString: fmt.Sprintf(`mutation { syncBlueprint(id:%q,ownerId:"tea-a") { blueprint { id } } }`, id)})
				if len(out.Errors) == 0 {
					t.Fatal("syncBlueprint succeeded on a created Blueprint")
				}
				refusal = fmt.Sprint(out.Errors[0].Extensions["code"], out.Errors[0].Message)
			case "MCP":
				server := mcp.NewServer(&mcp.Implementation{Name: "t012", Version: "0"}, nil)
				svc.RegisterMCP(server)
				a, b := mcp.NewInMemoryTransports()
				if _, err := server.Connect(ctx, a, nil); err != nil {
					t.Fatal(err)
				}
				session, err := mcp.NewClient(&mcp.Implementation{Name: "t012", Version: "0"}, nil).Connect(ctx, b, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = session.Close() }()
				out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "sync_blueprint", Arguments: map[string]any{"id": id}})
				if err != nil {
					t.Fatal(err)
				}
				if !out.IsError {
					t.Fatal("sync_blueprint succeeded on a created Blueprint")
				}
				for _, c := range out.Content {
					if text, ok := c.(*mcp.TextContent); ok {
						refusal += text.Text
					}
				}
			}
			if !strings.Contains(refusal, BlueprintAwaitingApprovalCode) {
				t.Errorf("refusal = %s, want %s", refusal, BlueprintAwaitingApprovalCode)
			}
			var apps appv1alpha1.AppList
			if err := svc.Client.List(ctx, &apps); err != nil || len(apps.Items) != 0 {
				t.Fatalf("the refused sync created %d Apps (err %v)", len(apps.Items), err)
			}
			if b := st.blueprints[ids.BlueprintStorageID(id)]; b.Status != store.BlueprintStatusCreated {
				t.Errorf("status after the refusal = %q, want created", b.Status)
			}
		})
	}
}

// w2/m256 t012: a waiting plan stays readable — actions and required manual
// env vars — without planning again, and is gone once approved.
func TestBlueprintPlannedSyncIsReadableUntilApproved(t *testing.T) {
	svc, _, ctx := newOnboardingService(t, onboardingManifest)
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	id := res.Blueprint.ID
	// The plan mutation's own sync reads through the same type: its fields
	// must not come back null.
	replanned := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: ctx,
		RequestString: fmt.Sprintf(`mutation { planBlueprintSync(id:%q, ownerId:"tea-a") { sync { id state } } }`, id)})
	if len(replanned.Errors) != 0 {
		t.Fatal(replanned.Errors)
	}
	sync, _ := replanned.Data.(map[string]any)["planBlueprintSync"].(map[string]any)["sync"].(map[string]any)
	if sync["id"] == nil || sync["state"] != store.BlueprintSyncStateCreated {
		t.Fatalf("planBlueprintSync sync = %v, want its id and created state", sync)
	}
	res.Sync.ID = sync["id"].(string)
	query := fmt.Sprintf(`{ blueprintPlannedSync(id:%q, ownerId:"tea-a") { id state rawPlan requiredManualEnvVars { serviceName key } } }`, id)
	out := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: ctx, RequestString: query})
	if len(out.Errors) != 0 {
		t.Fatal(out.Errors)
	}
	planned, _ := out.Data.(map[string]any)["blueprintPlannedSync"].(map[string]any)
	if planned["id"] != res.Sync.ID || planned["state"] != store.BlueprintSyncStateCreated {
		t.Fatalf("planned sync = %v, want %s waiting", planned, res.Sync.ID)
	}
	if raw, _ := planned["rawPlan"].(string); !strings.Contains(raw, `"operation":"create"`) || !strings.Contains(raw, `"name":"api"`) {
		t.Errorf("rawPlan = %v, want the create of api", planned["rawPlan"])
	}
	if vars, _ := planned["requiredManualEnvVars"].([]any); len(vars) != 1 || vars[0].(map[string]any)["key"] != "API_TOKEN" {
		t.Errorf("requiredManualEnvVars = %v, want API_TOKEN", planned["requiredManualEnvVars"])
	}

	token := "s3cret"
	if _, err := svc.ApproveBlueprint(ctx, id, "tea-a", []ManualEnvVar{{ServiceType: "web", ServiceName: "api", Key: "API_TOKEN", Value: &token}}); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.PlannedSync(ctx, id, "tea-a"); err != nil || got != nil {
		t.Fatalf("planned sync after approval = %+v err=%v, want none", got, err)
	}
	out = graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: ctx, RequestString: query})
	if len(out.Errors) != 0 || out.Data.(map[string]any)["blueprintPlannedSync"] != nil {
		t.Fatalf("GraphQL after approval = %v %v, want null", out.Data, out.Errors)
	}
}

// w8/m153 (was w2/m256 t013 and w4/120): a Blueprint created after another on
// the same source was deleted is a new Blueprint with its own id. The deleted
// row is not revived, so the new one has no inherited history to mark off.
func TestPlannedCreateAfterADeleteIsANewBlueprint(t *testing.T) {
	svc, st, ctx := newOnboardingService(t, onboardingManifest)
	req := RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}}
	first, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Sync == nil {
		t.Fatal("a first create planned no sync")
	}
	firstID := ids.BlueprintStorageID(first.Blueprint.ID)
	if err := st.DisconnectBlueprint(ctx, firstID, "tea-a"); err != nil {
		t.Fatal(err)
	}
	again, err := svc.CreatePlannedBlueprint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if again.Blueprint.ID == first.Blueprint.ID || again.Sync == nil {
		t.Fatalf("create after a delete = %+v, want a new Blueprint (not %s) with a planned sync", again, first.Blueprint.ID)
	}
	if note := st.syncs[ids.BlueprintSyncStorageID(again.Sync.ID)].Note; note != "" {
		t.Errorf("the new Blueprint's planned sync note = %q, want none", note)
	}
	if got := st.blueprints[firstID].Status; got != "disconnected" {
		t.Errorf("the deleted Blueprint's row status = %q, want it left disconnected", got)
	}
}

// Render ignores a sync:false variable inside an env group instead of refusing
// the Blueprint (w8/293): the plan succeeds, the key is not prompted for, and
// one warning notice locates the entry.
func TestBlueprintPlanWarnsAboutAnIgnoredGroupSecret(t *testing.T) {
	manifest := `
envVarGroups:
  - name: shared
    envVars:
      - {key: API_KEY, sync: false}
      - {key: MODE, value: prod}
services:
  - type: web
    name: api
    runtime: image
    image:
      url: docker.io/library/nginx:1.27
    envVars:
      - fromGroup: shared
`
	svc, _, ctx := newOnboardingService(t, manifest)
	svc.EnvGroups = newFakeEnvGroups()
	res, err := svc.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.PlanningOutcome != PlanningPlanned || len(res.Notices) != 1 {
		t.Fatalf("plan = %+v, want planned with one notice", res)
	}
	n := res.Notices[0]
	if n.Severity != "warning" || n.Line == nil || *n.Line != 5 || n.Path == nil || !strings.HasSuffix(*n.Path, "envVarGroups[0].envVars[0]") ||
		!strings.Contains(n.Message, `envVars["API_KEY"]: sync: false is ignored in an environment group`) {
		t.Errorf("notice = %+v, want a warning at envVarGroups[0].envVars[0], line 5", n)
	}
	if res.Sync == nil || len(res.Sync.RequiredManualEnvVars) != 0 {
		t.Errorf("an ignored group key is not prompted for: %+v", res.Sync)
	}
}

// A group's sync:false entries are skipped, with or without a value, and
// reported by index and key; its other entries parse as before (w8/293).
func TestParseEnvGroupIgnoresSyncFalse(t *testing.T) {
	no := false
	pg, err := parseEnvGroup(bexEnvGroup{Name: "shared", EnvVars: []bexEnvVar{
		{Key: "A", Sync: &no}, {Key: "B", Sync: &no, Value: "v"}, {Key: "C", Value: "c"},
		{Key: "D", Sync: &no, GenerateValue: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(pg.literals) != 1 || pg.literals["C"] != "c" || len(pg.generates) != 1 || pg.generates[0] != "D" {
		t.Errorf("literals = %v, generates = %v; want C, and D generated as a service's generateValue wins over sync:false", pg.literals, pg.generates)
	}
	if len(pg.ignored) != 2 {
		t.Fatalf("ignored = %v, want A and B", pg.ignored)
	}
	for i, key := range []string{"A", "B"} {
		var entry *envEntryError
		if !errors.As(pg.ignored[i], &entry) || entry.index != i || !strings.Contains(pg.ignored[i].Error(), `envVars["`+key+`"]: sync: false is ignored`) {
			t.Errorf("ignored[%d] = %v, want entry %d naming %s", i, pg.ignored[i], i, key)
		}
	}
}
