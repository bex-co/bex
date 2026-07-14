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

// blueprint_test.go covers the /blueprints surface (w2/m15):
// validate (stateless) · list · sync — each verb over all three adapters.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// --- fake store ---

type fakeBlueprintStore struct {
	blueprints map[string]store.Blueprint // key: id
}

func newFakeBlueprintStore(bs ...store.Blueprint) *fakeBlueprintStore {
	f := &fakeBlueprintStore{blueprints: make(map[string]store.Blueprint)}
	for _, b := range bs {
		f.blueprints[b.ID] = b
	}
	return f
}

func (f *fakeBlueprintStore) UpsertBlueprint(_ context.Context, b store.Blueprint) (store.Blueprint, error) {
	for id, existing := range f.blueprints {
		if existing.TenantID == b.TenantID && existing.Repo == b.Repo && existing.Branch == b.Branch {
			existing.Name = b.Name
			existing.Manifest = b.Manifest
			existing.Status = "active"
			f.blueprints[id] = existing
			return existing, nil
		}
	}
	if b.ID == "" {
		b.ID = fmt.Sprintf("blp-fake-%d", len(f.blueprints))
	}
	f.blueprints[b.ID] = b
	return b, nil
}

func (f *fakeBlueprintStore) GetBlueprint(_ context.Context, id, tenantID string) (store.Blueprint, error) {
	b, ok := f.blueprints[id]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, fmt.Errorf("blueprint: %w", store.ErrNotFound)
	}
	return b, nil
}

func (f *fakeBlueprintStore) ListBlueprints(_ context.Context, tenantID string) ([]store.Blueprint, error) {
	var out []store.Blueprint
	for _, b := range f.blueprints {
		if b.TenantID == tenantID && b.Status == "active" {
			out = append(out, b)
		}
	}
	return out, nil
}

// --- helpers ---

func newBlueprintService(fs *fakeBlueprintStore, ws core.WorkspaceResolver) *Service {
	cl := fakeClient()
	svc := &Service{
		Base:       &core.Base{Client: cl, Namespace: "default", Workspace: ws},
		Blueprints: fs,
	}
	return svc
}

func blueprintSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return schema
}

// --- ValidateBlueprint ---

func TestValidateBlueprintValidYAML(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), stackManifest)
	if err != nil {
		t.Fatalf("ValidateBlueprint(valid): %v", err)
	}
	if !v.Valid || len(v.Errors) != 0 {
		t.Errorf("valid manifest: want Valid=true no errors, got %+v", v)
	}
}

func TestValidateBlueprintBadYAML(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	const bad = `services:
  - name: ""
    type: web
`
	v, err := svc.ValidateBlueprint(context.Background(), bad)
	if err != nil {
		t.Fatalf("ValidateBlueprint(bad): unexpected error %v", err)
	}
	if v.Valid || len(v.Errors) == 0 {
		t.Errorf("invalid manifest: want Valid=false with errors, got %+v", v)
	}
}

func TestValidateBlueprintStateless(t *testing.T) {
	// validate must not touch the store — Blueprints=nil must not panic.
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: nil}
	if _, err := svc.ValidateBlueprint(context.Background(), stackManifest); err != nil {
		t.Fatalf("ValidateBlueprint with nil store: %v", err)
	}
}

// --- ListBlueprints ---

func TestListBlueprintsNoStore(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	if _, err := svc.ListBlueprints(context.Background(), "tea-a"); !errors.Is(err, ErrBlueprintsUnavailable) {
		t.Errorf("ListBlueprints no store: want ErrBlueprintsUnavailable, got %v", err)
	}
}

func TestListBlueprintsScopedToTenant(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(
		store.Blueprint{ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app", Branch: "main", Status: "active", Name: "app"},
		store.Blueprint{ID: "blp-2", TenantID: "tea-b", Repo: "https://github.com/b/other", Branch: "main", Status: "active", Name: "other"},
	)
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	views, err := svc.ListBlueprints(ctx, "tea-a")
	if err != nil {
		t.Fatalf("ListBlueprints: %v", err)
	}
	if len(views) != 1 || views[0].ID != "blp-1" {
		t.Errorf("ListBlueprints: want [blp-1], got %+v", views)
	}
}

func TestListBlueprintsEmpty(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: newFakeBlueprintStore()}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	views, err := svc.ListBlueprints(ctx, "tea-a")
	if err != nil {
		t.Fatalf("ListBlueprints empty: %v", err)
	}
	if len(views) != 0 {
		t.Errorf("ListBlueprints empty: want [], got %+v", views)
	}
}

// --- SyncBlueprint ---

func TestSyncBlueprintNoStore(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	if _, err := svc.SyncBlueprint(context.Background(), "blp-1", "tea-a", ""); !errors.Is(err, ErrBlueprintsUnavailable) {
		t.Errorf("SyncBlueprint no store: want ErrBlueprintsUnavailable, got %v", err)
	}
}

func TestSyncBlueprintNotFound(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore()
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	if _, err := svc.SyncBlueprint(ctx, "blp-missing", "tea-a", ""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SyncBlueprint missing id: want ErrNotFound, got %v", err)
	}
}

func TestSyncBlueprintReappliesManifest(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Manifest: stackManifest,
		Status:   "active",
		Name:     "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	res, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", "")
	if err != nil {
		t.Fatalf("SyncBlueprint: %v", err)
	}
	if res.Blueprint.ID != "blp-1" {
		t.Errorf("SyncBlueprint: blueprint id = %q, want blp-1", res.Blueprint.ID)
	}
	if len(res.Stack.Services) == 0 {
		t.Errorf("SyncBlueprint: stack.services empty, expected stack to be deployed")
	}
}

func TestSyncBlueprintReplacesManifest(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Manifest: stackManifest,
		Status:   "active",
		Name:     "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	const newManifest = `
services:
  - name: freshsvc
    type: web
    image: {url: "nginx:latest"}
`
	res, err := svc.SyncBlueprint(ctx, "blp-1", "tea-a", newManifest)
	if err != nil {
		t.Fatalf("SyncBlueprint replace: %v", err)
	}
	// Manifest should be updated in the store.
	stored := fs.blueprints["blp-1"]
	if stored.Manifest != newManifest {
		t.Errorf("SyncBlueprint: stored manifest not updated")
	}
	if len(res.Stack.Services) == 0 {
		t.Errorf("SyncBlueprint replace: stack.services empty")
	}
}

// --- upsertBlueprint (auto-registration) ---

func TestUpsertBlueprintAutoRegistersOnDeploy(t *testing.T) {
	fs := newFakeBlueprintStore()
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: fs}
	ctx := core.WithWorkspace(context.Background(), "tea-a")

	svc.upsertBlueprint(ctx, DeployRequest{
		Repo:     "https://github.com/bex/myapp.git",
		Branch:   "main",
		Manifest: stackManifest,
	})
	if len(fs.blueprints) != 1 {
		t.Fatalf("upsertBlueprint: want 1 blueprint, got %d", len(fs.blueprints))
	}
	for _, b := range fs.blueprints {
		if b.Name != "myapp" {
			t.Errorf("upsertBlueprint: name = %q, want myapp", b.Name)
		}
		if b.TenantID != "tea-a" {
			t.Errorf("upsertBlueprint: tenantID = %q, want tea-a", b.TenantID)
		}
	}
}

func TestUpsertBlueprintNoopWithoutRepo(t *testing.T) {
	fs := newFakeBlueprintStore()
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Blueprints: fs}

	svc.upsertBlueprint(context.Background(), DeployRequest{Manifest: stackManifest})
	if len(fs.blueprints) != 0 {
		t.Errorf("upsertBlueprint without repo: should not store anything")
	}
}

// --- repoName ---

func TestRepoName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://github.com/bex/myapp", "myapp"},
		{"https://github.com/bex/myapp.git", "myapp"},
		{"git@github.com:bex/repo.git", "repo"},
		{"https://github.com/bex/stack/", "stack"},
	}
	for _, c := range cases {
		if got := repoName(c.in); got != c.want {
			t.Errorf("repoName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- REST adapter ---

func TestRESTValidateBlueprint(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := fmt.Sprintf(`{"bexYaml":%q}`, stackManifest)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("validate valid => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Valid {
		t.Errorf("validate valid manifest: want valid=true, got %+v", out)
	}
}

func TestRESTValidateBlueprintBadYAML(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"bexYaml":"services:\n  - name: \"\"\n    type: web\n"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("validate bad => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Valid {
		t.Errorf("validate bad manifest: want valid=false, got %+v", out)
	}
}

func TestRESTListBlueprints(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Status: "active", Name: "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest("GET", "/v1/blueprints?ownerId=tea-a", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user-a", Method: "oauth2"}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list blueprints => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out []BlueprintView
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 1 || out[0].ID != "blp-1" {
		t.Errorf("list blueprints: want [blp-1], got %+v", out)
	}
}

// --- GraphQL adapter ---

func TestGraphQLValidateBlueprint(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	schema := blueprintSchema(t, svc)

	q := fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid errors } }`, stackManifest)
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q})
	if len(res.Errors) > 0 {
		t.Fatalf("validateBlueprint: %v", res.Errors)
	}
	v := res.Data.(map[string]any)["validateBlueprint"].(map[string]any)
	if v["valid"] != true {
		t.Errorf("validateBlueprint valid manifest: want valid=true, got %v", v)
	}
}

func TestGraphQLListBlueprints(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID: "blp-1", TenantID: "tea-a", Repo: "https://github.com/a/app",
		Branch: "main", Status: "active", Name: "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	schema := blueprintSchema(t, svc)

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ blueprints(ownerId: "tea-a") { id name repo branch status } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("blueprints query: %v", res.Errors)
	}
	list := res.Data.(map[string]any)["blueprints"].([]any)
	if len(list) != 1 {
		t.Errorf("blueprints query: want 1, got %d", len(list))
	}
	b := list[0].(map[string]any)
	if b["id"] != "blp-1" {
		t.Errorf("blueprints[0].id = %v, want blp-1", b["id"])
	}
}

func TestGraphQLSyncBlueprint(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Manifest: stackManifest,
		Status:   "active",
		Name:     "app",
	})
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws}, Blueprints: fs}
	schema := blueprintSchema(t, svc)

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `mutation { syncBlueprint(id: "blp-1", ownerId: "tea-a") { blueprint { id } services { id } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("syncBlueprint: %v", res.Errors)
	}
	data := res.Data.(map[string]any)["syncBlueprint"].(map[string]any)
	bp := data["blueprint"].(map[string]any)
	if bp["id"] != "blp-1" {
		t.Errorf("syncBlueprint.blueprint.id = %v, want blp-1", bp["id"])
	}
}
