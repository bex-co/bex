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

package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeProjectStore struct {
	projects map[string]store.Project
	services map[string][]string
	// envAttached simulates apps.environment_id IS NOT NULL for a service name
	// — w4/m32's SetProjectServices tests use withEnv to seed it, then assert
	// the departure return value/clearing behavior against it.
	envAttached map[string]bool
}

func newFakeProjectStore(projects ...store.Project) *fakeProjectStore {
	f := &fakeProjectStore{projects: map[string]store.Project{}, services: map[string][]string{}, envAttached: map[string]bool{}}
	for _, p := range projects {
		f.projects[p.ID] = p
	}
	return f
}

// withEnv marks names as carrying a (simulated) non-null environment_id —
// see envAttached.
func (f *fakeProjectStore) withEnv(names ...string) *fakeProjectStore {
	for _, n := range names {
		f.envAttached[n] = true
	}
	return f
}

func (f *fakeProjectStore) CreateProject(_ context.Context, tenantID, name string) (store.Project, error) {
	for _, existing := range f.projects {
		if existing.TenantID == tenantID && existing.Name == name {
			// Mirrors the real store's classify() on a UNIQUE(tenant_id, name)
			// violation (w6/m49).
			return store.Project{}, fmt.Errorf("project: %w", store.ErrConflict)
		}
	}
	p := store.Project{ID: "prj-created", TenantID: tenantID, Name: name}
	f.projects[p.ID] = p
	return p, nil
}

func (f *fakeProjectStore) GetProject(_ context.Context, projectID string) (store.Project, error) {
	p, ok := f.projects[projectID]
	if !ok {
		return store.Project{}, fmt.Errorf("project: %w", store.ErrNotFound)
	}
	return p, nil
}

func (f *fakeProjectStore) ListProjects(_ context.Context, tenantID string) ([]store.Project, error) {
	var out []store.Project
	for _, p := range f.projects {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeProjectStore) RenameProject(_ context.Context, projectID, name string) error {
	p, ok := f.projects[projectID]
	if !ok {
		return fmt.Errorf("project: %w", store.ErrNotFound)
	}
	p.Name = name
	f.projects[projectID] = p
	return nil
}

func (f *fakeProjectStore) DeleteProject(_ context.Context, projectID string) error {
	if _, ok := f.projects[projectID]; !ok {
		return fmt.Errorf("project: %w", store.ErrNotFound)
	}
	delete(f.projects, projectID)
	delete(f.services, projectID)
	return nil
}

func (f *fakeProjectStore) SetProjectServices(_ context.Context, projectID, _ string, serviceNames []string) ([]core.ServicePlacementChange, error) {
	want := make(map[string]bool, len(serviceNames))
	for _, n := range serviceNames {
		want[n] = true
	}
	var changes []core.ServicePlacementChange
	for _, n := range f.services[projectID] {
		if want[n] {
			continue
		}
		mv := core.ServiceMove{ProjectFrom: &projectID}
		if f.envAttached[n] {
			env := "env-" + n
			mv.EnvironmentFrom = &env
			delete(f.envAttached, n) // simulates environment_id NULLed in the same transaction
		}
		changes = append(changes, core.ServicePlacementChange{ServiceID: n, ServiceName: n, ServiceMove: mv})
	}
	for _, n := range serviceNames {
		if !slices.Contains(f.services[projectID], n) {
			changes = append(changes, core.ServicePlacementChange{
				ServiceID:   n,
				ServiceName: n,
				ServiceMove: core.ServiceMove{ProjectTo: &projectID},
			})
		}
	}
	f.services[projectID] = append([]string(nil), serviceNames...)
	return changes, nil
}

func (f *fakeProjectStore) ListProjectServices(_ context.Context, projectID string) ([]string, error) {
	return append([]string(nil), f.services[projectID]...), nil
}

type allowChecker struct{}

func (allowChecker) Check(context.Context, string, string, string) (bool, error) { return true, nil }

type denyObjectChecker string

func (d denyObjectChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	return object != string(d), nil
}

func ctxAs(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func newMCPClient(t *testing.T, ctx context.Context, svc *Service) *mcp.ClientSession {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGetCrossTenantIsForbiddenNotNotFound(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-other", TenantID: "tea-other", Name: "other"})
	svc := &Service{
		Base:  &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-other"))},
		Store: st,
	}

	_, err := svc.Get(ctxAs("user-a"), "prj-other")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-tenant Get: got %v, want ErrForbidden", err)
	}
	if errors.Is(err, core.ErrNotFound) {
		t.Fatalf("cross-tenant Get leaked nonexistence semantics: %v", err)
	}
}

func TestGetNonexistentIsNotFound(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore()}
	_, err := svc.Get(ctxAs("user-a"), "prj-missing")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("nonexistent Get: got %v, want ErrNotFound", err)
	}
}

// txProjectStore adds the transactional grouping runner to the plain fake —
// the capability the direct-create quota keys on (the production *store.PGStore
// has it structurally).
type txProjectStore struct{ *fakeProjectStore }

func (t txProjectStore) RunGroupingTx(ctx context.Context, fn func(store.GroupingStore) error) error {
	return fn(fakeProjectGroupings{t.fakeProjectStore})
}

// fakeProjectGroupings adapts the fake to the tx-scoped GroupingStore the
// quota path reads; only the project half is real, the environment half inert.
type fakeProjectGroupings struct{ f *fakeProjectStore }

func (g fakeProjectGroupings) ListProjects(ctx context.Context, tenantID string) ([]store.Project, error) {
	return g.f.ListProjects(ctx, tenantID)
}
func (g fakeProjectGroupings) CreateProject(ctx context.Context, tenantID, name string) (store.Project, error) {
	return g.f.CreateProject(ctx, tenantID, name)
}
func (fakeProjectGroupings) ListEnvironments(context.Context, string) ([]store.Environment, error) {
	return nil, nil
}
func (fakeProjectGroupings) CreateEnvironment(context.Context, string, string, string) (store.Environment, error) {
	return store.Environment{}, nil
}
func (fakeProjectGroupings) SetEnvironmentACL(context.Context, string, string, bool, []core.IPAllowListEntry) error {
	return nil
}
func (g fakeProjectGroupings) CountWorkspaceGroupings(ctx context.Context, tenantID string) (int, int, error) {
	ps, err := g.f.ListProjects(ctx, tenantID)
	return len(ps), 0, err
}

// TestCreateEnforcesGroupingQuota pins codex-security round 12, finding 5: the
// direct project create shares the Blueprint grouping quota — an over-cap
// workspace is refused with the same coded BLUEPRINT_GROUPING_LIMIT error
// across every surface, and 0 disables the bound.
func TestCreateEnforcesGroupingQuota(t *testing.T) {
	st := &txProjectStore{newFakeProjectStore(store.Project{ID: "prj-held", TenantID: "tea-a", Name: "held"})}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, MaxGroupings: 1}
	ctx := ctxAs("user-a")

	_, err := svc.Create(ctx, "tea-a", "next")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_GROUPING_LIMIT" {
		t.Fatalf("over-quota create = %v, want BLUEPRINT_GROUPING_LIMIT", err)
	}
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("quota refusal must be conflict-class, got %v", err)
	}

	// 0 disables the cap (the documented boundary), and one below-cap create
	// still succeeds through the transactional path.
	svc.MaxGroupings = 0
	if _, err := svc.Create(ctx, "tea-a", "next"); err != nil {
		t.Fatalf("uncapped create: %v", err)
	}
	svc.MaxGroupings = 3 // workspace now holds 2 projects — one more fits
	if _, err := svc.Create(ctx, "tea-a", "another"); err != nil {
		t.Fatalf("below-cap create through the tx path: %v", err)
	}
}

// TestCreateDuplicateNameIsConflict is w6/m49's regression: a duplicate
// project name in the same workspace must be a coded 409 that names the
// attempted name — the pre-fix message ("project: already exists") named
// neither.
func TestCreateDuplicateNameIsConflict(t *testing.T) {
	st := newFakeProjectStore()
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}
	ctx := ctxAs("user-a")

	if _, err := svc.Create(ctx, "tea-a", "dup"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctx, "tea-a", "dup")
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate name: got %v, want ErrConflict", err)
	}
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "CONFLICT" {
		t.Fatalf("duplicate name: got %v, want *core.CodedError{Code: CONFLICT}", err)
	}
	if !strings.Contains(err.Error(), `"dup"`) {
		t.Errorf("message = %q, want it to name the attempted name", err.Error())
	}
}

func TestCrossTenantGetIsForbiddenAcrossAdapters(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-other", TenantID: "tea-other", Name: "other"})
	svc := &Service{
		Base:  &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-other"))},
		Store: st,
	}
	ctx := ctxAs("user-a")

	t.Run("REST", func(t *testing.T) {
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/prj-other", nil)
		mux.ServeHTTP(rec, req.WithContext(ctx))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		})
		if err != nil {
			t.Fatalf("schema: %v", err)
		}
		result := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       ctx,
			RequestString: `{ project(id: "prj-other") { id } }`,
		})
		if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, core.ErrForbidden.Error()) {
			t.Fatalf("GraphQL errors = %#v, want forbidden resolver error", result.Errors)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		client := newMCPClient(t, ctx, svc)
		result, err := client.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_project",
			Arguments: map[string]any{"id": "prj-other"},
		})
		if err != nil {
			t.Fatalf("transport error: %v", err)
		}
		raw, _ := json.Marshal(result.Content)
		if !result.IsError || !strings.Contains(string(raw), core.ErrForbidden.Error()) {
			t.Fatalf("MCP result = %#v content=%s, want forbidden tool error", result, raw)
		}
	})
}

func TestUnavailableStoreAcrossAdapters(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}}

	t.Run("REST is 503 with Render error envelope", func(t *testing.T) {
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/prj-missing", nil)
		mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode REST body: %v", err)
		}
		if body["id"] != "unavailable" || body["message"] != ErrProjectsUnavailable.Error() {
			t.Fatalf("REST body = %#v, want Render unavailable envelope", body)
		}
	})

	t.Run("GraphQL is a resolver error", func(t *testing.T) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		})
		if err != nil {
			t.Fatalf("schema: %v", err)
		}
		result := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       ctxAs("user-a"),
			RequestString: `{ project(id: "prj-missing") { id } }`,
		})
		if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, ErrProjectsUnavailable.Error()) {
			t.Fatalf("GraphQL errors = %#v, want projects-unavailable resolver error", result.Errors)
		}
	})

	t.Run("MCP is a tool error", func(t *testing.T) {
		ctx := ctxAs("user-a")
		client := newMCPClient(t, ctx, svc)

		result, err := client.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_project",
			Arguments: map[string]any{"id": "prj-missing"},
		})
		if err != nil {
			t.Fatalf("transport error: %v", err)
		}
		if !result.IsError {
			t.Fatalf("MCP result = %#v, want tool error", result)
		}
		raw, _ := json.Marshal(result.Content)
		if !strings.Contains(string(raw), ErrProjectsUnavailable.Error()) {
			t.Fatalf("MCP error content = %s, want projects-unavailable message", raw)
		}
	})
}

func TestProjectListPaginationAcrossExtensionSurfaces(t *testing.T) {
	seeded := make([]store.Project, 0, 5)
	for i := 1; i <= 5; i++ {
		seeded = append(seeded, store.Project{
			ID:       fmt.Sprintf("prj-%02d", i),
			TenantID: "tea-1",
			Name:     fmt.Sprintf("project-%02d", i),
		})
	}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore(seeded...)}
	ctx := ctxAs("user-a")

	t.Run("GraphQL", func(t *testing.T) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		page := func(cursor string, paged bool) []string {
			t.Helper()
			args := `ownerId: "tea-1"`
			if paged {
				args += `, limit: 2`
				if cursor != "" {
					args += fmt.Sprintf(`, cursor: %q`, cursor)
				}
			}
			result := graphql.Do(graphql.Params{
				Schema:        schema,
				Context:       ctx,
				RequestString: `{ projects(` + args + `) { id } }`,
			})
			if len(result.Errors) > 0 {
				t.Fatalf("GraphQL errors: %#v", result.Errors)
			}
			raw, _ := json.Marshal(result.Data)
			var body struct {
				Projects []struct {
					ID string `json:"id"`
				} `json:"projects"`
			}
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatal(err)
			}
			ids := make([]string, len(body.Projects))
			for i := range body.Projects {
				ids[i] = body.Projects[i].ID
			}
			return ids
		}
		if got := page("", true); strings.Join(got, ",") != "prj-01,prj-02" {
			t.Fatalf("first GraphQL page = %v", got)
		}
		if got := page("prj-02", true); strings.Join(got, ",") != "prj-03,prj-04" {
			t.Fatalf("second GraphQL page = %v", got)
		}
		if got := page("", false); len(got) != len(seeded) {
			t.Fatalf("unpaged GraphQL list = %d, want %d", len(got), len(seeded))
		}
	})

	t.Run("MCP", func(t *testing.T) {
		mcpCtx := core.WithWorkspace(ctx, "tea-1")
		client := newMCPClient(t, mcpCtx, svc)
		call := func(args map[string]any) projectsResult {
			t.Helper()
			result, err := client.CallTool(mcpCtx, &mcp.CallToolParams{Name: "list_projects", Arguments: args})
			if err != nil || result.IsError {
				t.Fatalf("list_projects(%v): err=%v isError=%v", args, err, result.IsError)
			}
			raw, _ := json.Marshal(result.StructuredContent)
			var out projectsResult
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatal(err)
			}
			return out
		}
		first := call(map[string]any{"limit": 2})
		if len(first.Projects) != 2 || first.Projects[0].ID != "prj-01" || first.Cursor != "prj-02" {
			t.Fatalf("first MCP page = %+v", first)
		}
		second := call(map[string]any{"limit": 2, "cursor": first.Cursor})
		if len(second.Projects) != 2 || second.Projects[0].ID != "prj-03" || second.Cursor != "prj-04" {
			t.Fatalf("second MCP page = %+v", second)
		}
		if all := call(nil); len(all.Projects) != len(seeded) || all.Cursor != "" {
			t.Fatalf("unpaged MCP list = %d with cursor %q, want %d with no cursor", len(all.Projects), all.Cursor, len(seeded))
		}
	})
}

// recordingEnvironmentIndex is a call-recording EnvironmentIndex double for
// w4/m32's cross-feature member-clear fan-out — projects_test.go can't import
// environments' own test fixtures (different package), so this stands in for
// *environments.Service structurally.
type recordingEnvironmentIndex struct {
	clearedServices []string
	clearedProjects []string
	err             error
}

func (r *recordingEnvironmentIndex) ClearServiceEnvironmentLayer(_ context.Context, serviceNames []string) error {
	r.clearedServices = append(r.clearedServices, serviceNames...)
	return r.err
}

func (r *recordingEnvironmentIndex) ClearMembersForProject(_ context.Context, projectID string) error {
	r.clearedProjects = append(r.clearedProjects, projectID)
	return r.err
}

// TestSetServicesClearsEnvironmentLayerForDeparting is w4/m32/t001: a service
// leaving its project that also carried a (simulated) non-null
// environment_id must have its App CR's environment-projected layer cleared
// via EnvironmentIndex — the store already NULLs the column, but only this
// fan-out can touch the k8s CR.
func TestSetServicesClearsEnvironmentLayerForDeparting(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"}).withEnv("web")
	st.services["prj-1"] = []string{"web", "worker"}
	envIdx := &recordingEnvironmentIndex{}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, Environments: envIdx}

	if _, err := svc.SetServices(ctxAs("user-a"), "prj-1", []string{"worker"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	if len(envIdx.clearedServices) != 1 || envIdx.clearedServices[0] != "web" {
		t.Errorf("cleared services = %v, want [web] (worker stayed, web departed carrying environment_id)", envIdx.clearedServices)
	}
}

// TestSetServicesSkipsClearForDeparturesWithNoEnvironment: a departing
// service that never carried an environment_id triggers no EnvironmentIndex
// call at all — nothing to clear, and the optional fan-out should not fire
// on every ordinary membership change.
func TestSetServicesSkipsClearForDeparturesWithNoEnvironment(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	st.services["prj-1"] = []string{"web", "worker"}
	envIdx := &recordingEnvironmentIndex{}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, Environments: envIdx}

	if _, err := svc.SetServices(ctxAs("user-a"), "prj-1", []string{"worker"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	if len(envIdx.clearedServices) != 0 {
		t.Errorf("cleared services = %v, want none", envIdx.clearedServices)
	}
}

// TestSetServicesToleratesUnwiredEnvironments: Environments nil (unwired)
// degrades the same way every other optional cross-feature index in this
// codebase does — SetServices still succeeds, the clear is just skipped.
func TestSetServicesToleratesUnwiredEnvironments(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"}).withEnv("web")
	st.services["prj-1"] = []string{"web", "worker"}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}

	if _, err := svc.SetServices(ctxAs("user-a"), "prj-1", []string{"worker"}); err != nil {
		t.Fatalf("SetServices with Environments unwired: %v", err)
	}
}

// TestDeleteClearsEnvironmentMembersBeforeRemovingProject is w4/m32/t002:
// deleting a project fans the environment-layer clear to every child
// environment's members via EnvironmentIndex before the project row (and its
// cascaded environment rows) disappear.
func TestDeleteClearsEnvironmentMembersBeforeRemovingProject(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	envIdx := &recordingEnvironmentIndex{}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, Environments: envIdx}

	if err := svc.Delete(ctxAs("user-a"), "prj-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(envIdx.clearedProjects) != 1 || envIdx.clearedProjects[0] != "prj-1" {
		t.Errorf("cleared projects = %v, want one entry for prj-1", envIdx.clearedProjects)
	}
	if _, err := st.GetProject(ctxAs("user-a"), "prj-1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("project still exists after Delete: %v", err)
	}
}

// TestDeleteAbortsBeforeRemovingProjectIfEnvironmentClearFails: an error
// clearing member CRs must stop the delete — never remove the project row
// (and cascade its environments away) while member CRs might still carry
// stale enforcement, since there would be no environment row left to re-derive
// the correct state from.
func TestDeleteAbortsBeforeRemovingProjectIfEnvironmentClearFails(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	envIdx := &recordingEnvironmentIndex{err: errors.New("clear failed")}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, Environments: envIdx}

	if err := svc.Delete(ctxAs("user-a"), "prj-1"); err == nil {
		t.Fatal("Delete succeeded despite the environment-clear failure")
	}
	if _, err := st.GetProject(ctxAs("user-a"), "prj-1"); err != nil {
		t.Errorf("project should still exist after the aborted delete: %v", err)
	}
}
