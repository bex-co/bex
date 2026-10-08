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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fixedOwnerResolver is a minimal resourcemeta.OwnerResolver a test can seed
// without pulling in workspaces.Service.
type fixedOwnerResolver map[string]resourcemeta.Owner

func (f fixedOwnerResolver) ResolveResourceOwners(_ context.Context, ids []string) map[string]resourcemeta.Owner {
	out := map[string]resourcemeta.Owner{}
	for _, id := range ids {
		if o, ok := f[id]; ok {
			out[id] = o
		}
	}
	return out
}

type recordingOwnerResolver struct {
	fixedOwnerResolver
	calls int
}

func (r *recordingOwnerResolver) ResolveResourceOwners(ctx context.Context, ids []string) map[string]resourcemeta.Owner {
	r.calls++
	return r.fixedOwnerResolver.ResolveResourceOwners(ctx, ids)
}

func TestToRenderProjectIncludesEnvironmentMembership(t *testing.T) {
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	owner := resourcemeta.Owner{ID: "tea-1", Name: "Acme", Email: "a@acme.test", Type: "team"}
	got := toRenderProject(ProjectView{
		ID: "prj-1", Name: "platform", OwnerID: "tea-1",
		CreatedAt: created, UpdatedAt: updated,
	}, []string{"env-1"}, owner)
	if got.ID != "prj-1" || got.Owner == nil || got.Owner.ID != "tea-1" || got.Owner.Name != "Acme" || got.Owner.Type != "team" || len(got.EnvironmentIDs) != 1 || got.EnvironmentIDs[0] != "env-1" {
		t.Fatalf("Render project = %+v", got)
	}
	if !got.UpdatedAt.Equal(updated) {
		t.Fatalf("updatedAt = %v, want %v", got.UpdatedAt, updated)
	}
}

func TestToRenderProjectOmitsUnresolvedOwner(t *testing.T) {
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	got := toRenderProject(ProjectView{ID: "prj-1", Name: "platform", OwnerID: "tea-1", CreatedAt: created}, nil, resourcemeta.Owner{ID: "tea-1"})
	if got.Owner != nil {
		t.Fatalf("owner = %+v, want omitted (Available requires name)", got.Owner)
	}
	if !got.UpdatedAt.Equal(created) {
		t.Fatalf("zero UpdatedAt should fall back to createdAt, got %v", got.UpdatedAt)
	}
}

func TestProjectsRESTListNamesMissingOwnerID(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore()}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /v1/projects without ownerId = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		ID      string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	// The opaque "bad request" the bug reported must be replaced by a body that
	// names ownerId, matching the validator-gated siblings (w4/038).
	if !strings.Contains(body.Error, "ownerId") {
		t.Fatalf("error body %q does not name the missing ownerId parameter", body.Error)
	}
	if body.Message != body.Error {
		t.Fatalf("message %q and error %q disagree", body.Message, body.Error)
	}
}

func TestProjectsRESTPaginationWalkTerminatesWithoutDuplicates(t *testing.T) {
	const total = 2*core.DefaultPageLimit + 1
	seeded := make([]store.Project, 0, total)
	for i := 1; i <= total; i++ {
		seeded = append(seeded, store.Project{
			ID:       fmt.Sprintf("prj-%03d", i),
			TenantID: "tea-1",
			Name:     fmt.Sprintf("project-%03d", i),
		})
	}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore(seeded...)}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	requestPage := func(cursor string, includeLimit bool) []renderProjectWithCursor {
		t.Helper()
		query := url.Values{"ownerId": {"tea-1"}}
		if includeLimit {
			query.Set("limit", fmt.Sprint(core.DefaultPageLimit))
		}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects?"+query.Encode(), nil)
		mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET projects = %d: %s", rec.Code, rec.Body.String())
		}
		var page []renderProjectWithCursor
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode page: %v", err)
		}
		return page
	}

	if first := requestPage("", false); len(first) != core.DefaultPageLimit {
		t.Fatalf("absent limit returned %d projects, want default %d", len(first), core.DefaultPageLimit)
	}

	seen := make(map[string]struct{}, total)
	cursor := ""
	pages := 0
	for {
		page := requestPage(cursor, true)
		pages++
		if len(page) == 0 {
			break
		}
		for _, item := range page {
			if _, duplicate := seen[item.Project.ID]; duplicate {
				t.Fatalf("duplicate project %q after cursor %q", item.Project.ID, cursor)
			}
			seen[item.Project.ID] = struct{}{}
		}
		cursor = page[len(page)-1].Cursor
		if len(page) < core.DefaultPageLimit {
			if tail := requestPage(cursor, true); len(tail) != 0 {
				t.Fatalf("tail after final cursor = %d projects, want empty", len(tail))
			}
			break
		}
		if pages > 4 {
			t.Fatal("pagination did not terminate")
		}
	}
	if pages != 3 || len(seen) != total {
		t.Fatalf("walk = %d pages, %d unique projects; want 3 pages, %d projects", pages, len(seen), total)
	}
}

func decodeRenderProject(t *testing.T, body []byte) renderProject {
	t.Helper()
	var got renderProject
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	return got
}

func projectRESTFixture(t *testing.T, owners resourcemeta.OwnerResolver) (*Service, *http.ServeMux, *fakeProjectStore) {
	t.Helper()
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	st := newFakeProjectStore(store.Project{
		ID: "prj-1", TenantID: "tea-1", Name: "platform",
		CreatedAt: created, UpdatedAt: created,
	})
	svc := &Service{
		Base:      &core.Base{Authz: allowChecker{}},
		Store:     st,
		Databases: newFakeResourceIndex("tea-1"),
		KeyValues: newFakeResourceIndex("tea-1"),
		Owners:    owners,
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return svc, mux, st
}

func TestProjectMutationsAdvanceUpdatedAt(t *testing.T) {
	owners := fixedOwnerResolver{"tea-1": {ID: "tea-1", Name: "Acme", Email: "a@acme.test", Type: "team"}}
	_, mux, _ := projectRESTFixture(t, owners)

	get := func() renderProject {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/prj-1", nil)
		mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET = %d: %s", rec.Code, rec.Body.String())
		}
		return decodeRenderProject(t, rec.Body.Bytes())
	}

	before := get()
	if !before.UpdatedAt.Equal(before.CreatedAt) {
		t.Fatalf("seed updatedAt=%v createdAt=%v", before.UpdatedAt, before.CreatedAt)
	}

	mutations := []struct {
		name, method, path, body string
	}{
		{"rename", http.MethodPatch, "/v1/projects/prj-1", `{"name":"renamed"}`},
		{"service-links", http.MethodPut, "/v1/projects/prj-1/service-links", `{"serviceIds":[]}`},
		{"database-links", http.MethodPut, "/v1/projects/prj-1/database-links", `{"databaseIds":[]}`},
		{"keyvalue-links", http.MethodPut, "/v1/projects/prj-1/keyvalue-links", `{"keyValueIds":[]}`},
	}
	prev := before.UpdatedAt
	for _, m := range mutations {
		t.Run(m.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(m.method, m.path, strings.NewReader(m.body))
			mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s = %d: %s", m.name, rec.Code, rec.Body.String())
			}
			got := decodeRenderProject(t, rec.Body.Bytes())
			if !got.UpdatedAt.After(prev) {
				t.Fatalf("%s updatedAt=%v did not advance past %v", m.name, got.UpdatedAt, prev)
			}
			prev = got.UpdatedAt
			stable := get()
			if !stable.UpdatedAt.Equal(got.UpdatedAt) {
				t.Fatalf("read after %s moved updatedAt: %v -> %v", m.name, got.UpdatedAt, stable.UpdatedAt)
			}
		})
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/projects", strings.NewReader(`{"name":"fresh","ownerId":"tea-1"}`))
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d: %s", rec.Code, rec.Body.String())
	}
	created := decodeRenderProject(t, rec.Body.Bytes())
	if !created.CreatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("POST createdAt=%v updatedAt=%v, want equal", created.CreatedAt, created.UpdatedAt)
	}
}

func TestProjectOwnerResolutionBatchAndOmit(t *testing.T) {
	recorder := &recordingOwnerResolver{fixedOwnerResolver: fixedOwnerResolver{
		"tea-1": {ID: "tea-1", Name: "Acme", Email: "a@acme.test", Type: "team"},
	}}
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	st := newFakeProjectStore(
		store.Project{ID: "prj-1", TenantID: "tea-1", Name: "a", CreatedAt: created, UpdatedAt: created},
		store.Project{ID: "prj-2", TenantID: "tea-1", Name: "b", CreatedAt: created, UpdatedAt: created},
	)
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, Owners: recorder}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects?ownerId=tea-1", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d: %s", rec.Code, rec.Body.String())
	}
	var page []renderProjectWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page len = %d, want 2", len(page))
	}
	if recorder.calls != 1 {
		t.Fatalf("owner resolution called %d times for a page, want 1 (N+1 guard)", recorder.calls)
	}
	for _, item := range page {
		if item.Project.Owner == nil || item.Project.Owner.Name != "Acme" {
			t.Fatalf("project %s owner = %+v, want populated Acme", item.Project.ID, item.Project.Owner)
		}
	}

	// Unresolvable: empty map → omit.
	svc.Owners = fixedOwnerResolver{}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/projects/prj-1", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	got := decodeRenderProject(t, rec.Body.Bytes())
	if got.Owner != nil {
		t.Fatalf("unresolvable owner = %+v, want omitted", got.Owner)
	}

	// Nil Owners (control-plane unavailable) still renders the project.
	svc.Owners = nil
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/projects/prj-1", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusOK {
		t.Fatalf("nil Owners GET = %d: %s", rec.Code, rec.Body.String())
	}
	got = decodeRenderProject(t, rec.Body.Bytes())
	if got.ID != "prj-1" || got.Owner != nil {
		t.Fatalf("nil Owners response = %+v", got)
	}
}

// recordingEnvironmentStore serves projects' environments, recording the
// project ids each read asks for.
type recordingEnvironmentStore struct {
	*fakeProjectStore
	envs  []store.Environment
	reads [][]string
}

func (r *recordingEnvironmentStore) ListEnvironmentsForProjects(_ context.Context, projectIDs []string) ([]store.Environment, error) {
	r.reads = append(r.reads, slices.Clone(projectIDs))
	return core.Filter(r.envs, func(e store.Environment) bool { return slices.Contains(projectIDs, e.ProjectID) }), nil
}

// TestAProjectPageReadsItsEnvironmentsOnce (w5/146): the list read each
// project's environments separately, a store read per project. One read of
// the page's projects now serves it, each project keeps its own
// environmentIds, an empty list included, and an empty page reads nothing.
func TestAProjectPageReadsItsEnvironmentsOnce(t *testing.T) {
	created := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	env1a, env1b, env3a := ids.EnvironmentStorageID(ids.New(ids.Environment)), ids.EnvironmentStorageID(ids.New(ids.Environment)), ids.EnvironmentStorageID(ids.New(ids.Environment))
	st := &recordingEnvironmentStore{
		fakeProjectStore: newFakeProjectStore(
			store.Project{ID: "prj-1", TenantID: "tea-1", Name: "a", CreatedAt: created, UpdatedAt: created},
			store.Project{ID: "prj-2", TenantID: "tea-1", Name: "b", CreatedAt: created, UpdatedAt: created},
			store.Project{ID: "prj-3", TenantID: "tea-1", Name: "c", CreatedAt: created, UpdatedAt: created},
		),
		envs: []store.Environment{
			{ID: env1a, ProjectID: "prj-1", TenantID: "tea-1"},
			{ID: env3a, ProjectID: "prj-3", TenantID: "tea-1"},
			{ID: env1b, ProjectID: "prj-1", TenantID: "tea-1"},
		},
	}
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st, Owners: fixedOwnerResolver{}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	list := func(query string) (page []renderProjectWithCursor, projectIDs []string) {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/projects?ownerId=tea-1"+query, nil).WithContext(ctxAs("user-a")))
		if rec.Code != http.StatusOK {
			t.Fatalf("list %q = %d: %s", query, rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, item := range page {
			projectIDs = append(projectIDs, item.Project.ID)
		}
		return page, projectIDs
	}

	page, projectIDs := list("")
	if len(st.reads) != 1 || !slices.Equal(st.reads[0], projectIDs) {
		t.Errorf("environment reads = %v, want one for the page's %v", st.reads, projectIDs)
	}
	want := map[string][]string{
		"prj-1": {ids.EnvironmentPublicID(env1a), ids.EnvironmentPublicID(env1b)},
		"prj-2": {},
		"prj-3": {ids.EnvironmentPublicID(env3a)},
	}
	if len(page) != len(want) {
		t.Fatalf("page len = %d, want %d", len(page), len(want))
	}
	for _, item := range page {
		if got := item.Project.EnvironmentIDs; !slices.Equal(got, want[item.Project.ID]) || got == nil {
			t.Errorf("project %s environmentIds = %#v, want %#v", item.Project.ID, got, want[item.Project.ID])
		}
	}

	if page, projectIDs = list("&limit=2"); len(page) != 2 || len(st.reads) != 2 || !slices.Equal(st.reads[1], projectIDs) {
		t.Errorf("a page of %d read %v, want one read of its own %v", len(page), st.reads[1:], projectIDs)
	}
	if page, _ = list("&name=none"); len(page) != 0 || len(st.reads) != 2 {
		t.Errorf("an empty page of %d read %v, want no read", len(page), st.reads[2:])
	}
}

// TestAProjectPageReadsMembershipForThatPageOnly (w5/155): every list surface
// read every project's services, one query per project, before it paged.
// REST, whose Render shape carries no membership, now reads none; GraphQL and
// MCP read the services of their page alone, in one query.
func TestAProjectPageReadsMembershipForThatPageOnly(t *testing.T) {
	newSvc := func() (*Service, *fakeProjectStore, *fakeResourceIndex) {
		var ps []store.Project
		for _, id := range []string{"prj-1", "prj-2", "prj-3", "prj-4", "prj-5"} {
			ps = append(ps, store.Project{ID: id, TenantID: "tea-a", Name: id})
		}
		st := newFakeProjectStore(ps...)
		for _, id := range []string{"prj-1", "prj-2", "prj-3", "prj-4", "prj-5"} {
			st.services[id] = []string{"srv-" + id}
		}
		idx := newFakeResourceIndex("tea-a")
		svc := &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: defaultProjectWorkspace{}}, Store: st, Databases: idx, KeyValues: idx}
		return svc, st, idx
	}
	wantPage := []string{"prj-1", "prj-2"}
	ctx := ctxAs("user-a")

	t.Run("REST", func(t *testing.T) {
		svc, st, idx := newSvc()
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/projects?ownerId=tea-a&limit=2", nil).WithContext(ctx))
		if rec.Code != http.StatusOK {
			t.Fatalf("list = %d: %s", rec.Code, rec.Body.String())
		}
		var page []renderProjectWithCursor
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if len(page) != 2 || st.serviceReads != 0 || idx.lists != 0 {
			t.Errorf("a page of %d read services %d times and listed resources %d times, want 2 and no reads", len(page), st.serviceReads, idx.lists)
		}
	})

	check := func(t *testing.T, st *fakeProjectStore, got []ProjectView) {
		t.Helper()
		var gotIDs []string
		for _, v := range got {
			gotIDs = append(gotIDs, v.ID)
			if !slices.Equal(v.ServiceIDs, []string{"srv-" + v.ID}) {
				t.Errorf("%s serviceIds = %v, want [srv-%s]", v.ID, v.ServiceIDs, v.ID)
			}
		}
		if !slices.Equal(gotIDs, wantPage) || st.serviceReads != 1 {
			t.Errorf("page %v read services %d times, want %v in one read", gotIDs, st.serviceReads, wantPage)
		}
	}

	t.Run("GraphQL", func(t *testing.T) {
		svc, st, _ := newSvc()
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ projects(ownerId: "tea-a", limit: 2) { id serviceIds } }`})
		if len(result.Errors) != 0 {
			t.Fatalf("errors = %v", result.Errors)
		}
		raw, _ := json.Marshal(result.Data)
		var data struct{ Projects []ProjectView }
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Fatal(err)
		}
		check(t, st, data.Projects)
	})

	t.Run("MCP", func(t *testing.T) {
		svc, st, _ := newSvc()
		res, err := newMCPClient(t, ctx, svc).CallTool(ctx, &mcp.CallToolParams{Name: "list_projects", Arguments: map[string]any{"limit": 2}})
		if err != nil || res.IsError {
			t.Fatalf("list_projects: %+v %v", res, err)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var out projectsResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		check(t, st, out.Projects)
	})
}
