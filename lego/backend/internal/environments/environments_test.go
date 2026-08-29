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

package environments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeStore is an in-memory EnvironmentStore, mirroring the Postgres error
// taxonomy (store.ErrNotFound) so mapStoreErr is exercised too. Also holds
// projects (a minimal stand-in for the sibling projects feature's rows) since
// EnvironmentStore borrows GetProject from it.
type fakeStore struct {
	mu     sync.Mutex
	projs  map[string]store.Project
	envs   map[string]store.Environment
	assign map[string]map[string]bool // environmentID -> serviceName -> true
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		projs:  map[string]store.Project{},
		envs:   map[string]store.Environment{},
		assign: map[string]map[string]bool{},
	}
}

func (f *fakeStore) addProject(p store.Project) { f.projs[p.ID] = p }

func (f *fakeStore) GetProject(_ context.Context, id string) (store.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.projs[id]
	if !ok {
		return store.Project{}, fmt.Errorf("project: %w", store.ErrNotFound)
	}
	return p, nil
}

func (f *fakeStore) CreateEnvironment(_ context.Context, projectID, tenantID, name string) (store.Environment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.envs {
		if e.ProjectID == projectID && e.Name == name {
			return store.Environment{}, fmt.Errorf("environment: %w", store.ErrConflict)
		}
	}
	e := store.Environment{ID: id.New(id.Environment), ProjectID: projectID, TenantID: tenantID, Name: name, ProtectedStatus: ProtectedStatusUnprotected}
	f.envs[e.ID] = e
	return e, nil
}

func (f *fakeStore) GetEnvironment(_ context.Context, id string) (store.Environment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.envs[id]
	if !ok {
		return store.Environment{}, fmt.Errorf("environment: %w", store.ErrNotFound)
	}
	return e, nil
}

func (f *fakeStore) ListEnvironments(_ context.Context, projectID string) ([]store.Environment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.Environment
	for _, e := range f.envs {
		if e.ProjectID == projectID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeStore) ListWorkspaceEnvironments(_ context.Context, tenantID string) ([]store.Environment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.Environment
	for _, e := range f.envs {
		if e.TenantID == tenantID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeStore) RenameEnvironment(_ context.Context, id, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.envs[id]
	if !ok {
		return fmt.Errorf("environment: %w", store.ErrNotFound)
	}
	e.Name = name
	f.envs[id] = e
	return nil
}

func (f *fakeStore) DeleteEnvironment(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.envs[id]; !ok {
		return fmt.Errorf("environment: %w", store.ErrNotFound)
	}
	delete(f.envs, id)
	delete(f.assign, id)
	return nil
}

func (f *fakeStore) SetEnvironmentServices(_ context.Context, environmentID, projectID, _ string, serviceNames []string) ([]core.ServicePlacementChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := map[string]bool{}
	var changes []core.ServicePlacementChange
	for _, n := range serviceNames {
		m[n] = true
		if !f.assign[environmentID][n] {
			changes = append(changes, core.ServicePlacementChange{
				ServiceID:   n,
				ServiceName: n,
				ServiceMove: core.ServiceMove{ProjectTo: &projectID, EnvironmentTo: &environmentID},
			})
		}
	}
	for n := range f.assign[environmentID] {
		if !m[n] {
			changes = append(changes, core.ServicePlacementChange{
				ServiceID:   n,
				ServiceName: n,
				ServiceMove: core.ServiceMove{ProjectFrom: &projectID, EnvironmentFrom: &environmentID},
			})
		}
	}
	f.assign[environmentID] = m
	return changes, nil
}

func (f *fakeStore) ListEnvironmentServices(_ context.Context, environmentID, _ string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for name := range f.assign[environmentID] {
		out = append(out, name)
	}
	return out, nil
}

func (f *fakeStore) ListWorkspaceEnvironmentServices(_ context.Context, tenantID string) (map[string][]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string][]string{}
	for environmentID, services := range f.assign {
		e, ok := f.envs[environmentID]
		if !ok || e.TenantID != tenantID {
			continue
		}
		for serviceID := range services {
			out[environmentID] = append(out[environmentID], serviceID)
		}
	}
	return out, nil
}

func (f *fakeStore) SetEnvironmentACL(_ context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.envs[id]
	if !ok {
		return fmt.Errorf("environment: %w", store.ErrNotFound)
	}
	e.ProtectedStatus, e.NetworkIsolationEnabled, e.IPAllowList = protectedStatus, networkIsolationEnabled, ipAllowList
	f.envs[id] = e
	return nil
}

// allowChecker allows every authz check — exercises the store/view logic
// rather than OpenFGA.
type allowChecker struct{}

func (allowChecker) Check(context.Context, string, string, string) (bool, error) { return true, nil }

// fakeDenyChecker denies every authz check.
type fakeDenyChecker struct{}

func (fakeDenyChecker) Check(context.Context, string, string, string) (bool, error) {
	return false, nil
}

// denyObjectChecker permits the caller-workspace preflight but denies access
// to one resource-owning workspace. That distinction is what lets the tests
// below pin 403-before-404 for an existing cross-tenant id.
type denyObjectChecker string

func (d denyObjectChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	return object != string(d), nil
}

func newService(st EnvironmentStore) *Service {
	return &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}
}

// txEnvironmentStore adds the transactional grouping runner to the plain fake
// — the capability the direct-create quota keys on (the production
// *store.PGStore has it structurally).
type txEnvironmentStore struct{ *fakeStore }

func (t txEnvironmentStore) RunGroupingTx(ctx context.Context, fn func(store.GroupingStore) error) error {
	return fn(fakeEnvironmentGroupings{t.fakeStore})
}

// fakeEnvironmentGroupings adapts the fake to the tx-scoped GroupingStore the
// quota path reads; only the environment half is real, the project half inert.
type fakeEnvironmentGroupings struct{ f *fakeStore }

func (fakeEnvironmentGroupings) ListProjects(context.Context, string) ([]store.Project, error) {
	return nil, nil
}
func (fakeEnvironmentGroupings) CreateProject(context.Context, string, string) (store.Project, error) {
	return store.Project{}, nil
}
func (g fakeEnvironmentGroupings) ListEnvironments(ctx context.Context, projectID string) ([]store.Environment, error) {
	return g.f.ListEnvironments(ctx, projectID)
}
func (g fakeEnvironmentGroupings) CreateEnvironment(ctx context.Context, projectID, tenantID, name string) (store.Environment, error) {
	return g.f.CreateEnvironment(ctx, projectID, tenantID, name)
}
func (fakeEnvironmentGroupings) SetEnvironmentACL(context.Context, string, string, bool, []core.IPAllowListEntry) error {
	return nil
}
func (g fakeEnvironmentGroupings) CountWorkspaceGroupings(_ context.Context, tenantID string) (int, int, error) {
	g.f.mu.Lock()
	defer g.f.mu.Unlock()
	envs := 0
	for _, e := range g.f.envs {
		if e.TenantID == tenantID {
			envs++
		}
	}
	return 0, envs, nil
}

// TestCreateEnvironmentEnforcesGroupingQuota pins codex-security round 12,
// finding 5: the direct environment create shares the Blueprint grouping quota
// — counted against the PROJECT'S OWN workspace — refusing over-cap with the
// coded BLUEPRINT_GROUPING_LIMIT error; 0 disables the bound.
func TestCreateEnvironmentEnforcesGroupingQuota(t *testing.T) {
	st := &txEnvironmentStore{newFakeStore()}
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	if _, err := st.CreateEnvironment(context.Background(), "prj-1", "tea-a", "production"); err != nil {
		t.Fatal(err)
	}
	svc := newService(st)
	svc.MaxGroupings = 1
	ctx := ctxAs("user-a")

	_, err := svc.Create(ctx, "prj-1", "staging")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_GROUPING_LIMIT" {
		t.Fatalf("over-quota create = %v, want BLUEPRINT_GROUPING_LIMIT", err)
	}
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("quota refusal must be conflict-class, got %v", err)
	}

	svc.MaxGroupings = 0
	if _, err := svc.Create(ctx, "prj-1", "staging"); err != nil {
		t.Fatalf("uncapped create: %v", err)
	}
}

// ctxAs attaches a caller identity — every Authorize/AuthorizeOn call
// requires one in context regardless of what the checker would answer.
func ctxAs(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func TestCreateEnvironment_NestedUnderProject(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ProjectID != "prj-1" || e.Name != "staging" || e.OwnerID != "tea-a" {
		t.Fatalf("unexpected view: %+v", e)
	}
	if len(e.ServiceIDs) != 0 {
		t.Fatalf("new environment should have no services, got %+v", e.ServiceIDs)
	}
}

func TestCreateEnvironment_UnknownProjectIsNotFound(t *testing.T) {
	svc := newService(newFakeStore())
	if _, err := svc.Create(ctxAs("user-a"), "prj-missing", "staging"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("want ErrNotFound for an unknown project, got %v", err)
	}
}

func TestCreateEnvironment_DuplicateNameConflict(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	if _, err := svc.Create(ctxAs("user-a"), "prj-1", "staging"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("want ErrConflict on name collision, got %v", err)
	}
	// w6/m49: a stable code and the attempted name, not just "environment:
	// already exists" — a dashboard hook can detect it without matching
	// backend copy, and the message is actually useful.
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "CONFLICT" {
		t.Fatalf("name collision: got %v, want *core.CodedError{Code: CONFLICT}", err)
	}
	if !strings.Contains(err.Error(), `"staging"`) {
		t.Errorf("message = %q, want it to name the attempted name", err.Error())
	}
}

func TestCreateResolverPinsUnknownForeignAndPolicy(t *testing.T) {
	st := newFakeStore()
	st.envs["env-staging"] = store.Environment{
		ID:                      "env-staging",
		ProjectID:               "prj-platform",
		TenantID:                "tea-a",
		NetworkIsolationEnabled: true,
		IPAllowList:             []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}},
	}
	resolver := NewCreateResolver(&Service{Store: st})

	assignment, err := resolver.ResolveForCreate(context.Background(), "env-staging", "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if assignment.ID != "env-staging" || assignment.ProjectID != "prj-platform" || !assignment.NetworkIsolationEnabled || len(assignment.IPAllowList) != 1 {
		t.Fatalf("assignment = %+v", assignment)
	}
	if _, err := resolver.ResolveForCreate(context.Background(), "missing", "tea-a"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown error = %v, want ErrNotFound", err)
	}
	if _, err := resolver.ResolveForCreate(context.Background(), "env-staging", "tea-b"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("foreign error = %v, want ErrForbidden", err)
	}
}

func TestListEnvironments_ScopedToProject(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	st.addProject(store.Project{ID: "prj-2", TenantID: "tea-a", Name: "data-stack"})
	svc := newService(st)
	svc.Create(ctxAs("user-a"), "prj-1", "staging")
	svc.Create(ctxAs("user-a"), "prj-1", "production")
	svc.Create(ctxAs("user-a"), "prj-2", "staging")

	list, err := svc.List(ctxAs("user-a"), "prj-1")
	if err != nil || len(list) != 2 {
		t.Fatalf("List(prj-1) = %+v (err %v), want 2", list, err)
	}
}

func TestListWorkspaceEnvironments_BatchesAcrossProjectsAndScopesOwner(t *testing.T) {
	st := newFakeStore()
	st.envs["env-web"] = store.Environment{ID: "env-web", ProjectID: "prj-web", TenantID: "tea-a", Name: "web"}
	st.envs["env-data"] = store.Environment{ID: "env-data", ProjectID: "prj-data", TenantID: "tea-a", Name: "data"}
	st.envs["env-foreign"] = store.Environment{ID: "env-foreign", ProjectID: "prj-foreign", TenantID: "tea-b", Name: "foreign"}
	st.assign["env-web"] = map[string]bool{"srv-web": true}
	st.assign["env-data"] = map[string]bool{"srv-data": true}
	st.assign["env-foreign"] = map[string]bool{"srv-foreign": true}

	list, err := newService(st).ListWorkspace(ctxAs("user-a"), "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("ListWorkspace(tea-a) = %+v, want 2", list)
	}
	got := map[string][]string{}
	for _, e := range list {
		got[e.ID] = e.ServiceIDs
	}
	if len(got["env-web"]) != 1 || got["env-web"][0] != "srv-web" ||
		len(got["env-data"]) != 1 || got["env-data"][0] != "srv-data" {
		t.Fatalf("workspace service index = %+v", got)
	}
	if _, ok := got["env-foreign"]; ok {
		t.Fatalf("foreign workspace environment leaked: %+v", got)
	}
}

func TestRenameEnvironment(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	renamed, err := svc.Rename(ctxAs("user-a"), e.ID, "staging-v2")
	if err != nil || renamed.Name != "staging-v2" {
		t.Fatalf("Rename: %+v, %v", renamed, err)
	}
}

func TestDeleteEnvironment_NotFoundAfterDelete(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err := svc.Delete(ctxAs("user-a"), e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctxAs("user-a"), e.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
}

func TestGetCrossTenantIsForbiddenNotNotFound(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-other", TenantID: "tea-other", Name: "other"})
	st.envs["env-other"] = store.Environment{
		ID:        "env-other",
		ProjectID: "prj-other",
		TenantID:  "tea-other",
		Name:      "production",
	}
	svc := &Service{
		Base:  &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-other"))},
		Store: st,
	}

	_, err := svc.Get(ctxAs("user-a"), "env-other")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-tenant Get: got %v, want ErrForbidden", err)
	}
	if errors.Is(err, core.ErrNotFound) {
		t.Fatalf("cross-tenant Get leaked nonexistence semantics: %v", err)
	}
}

func TestGetNonexistentIsNotFound(t *testing.T) {
	svc := newService(newFakeStore())
	_, err := svc.Get(ctxAs("user-a"), "env-missing")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("nonexistent Get: got %v, want ErrNotFound", err)
	}
}

func TestSetServices_ReplacesFullList(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	got, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"alpha", "beta"})
	if err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	if len(got.ServiceIDs) != 2 {
		t.Fatalf("ServiceIDs = %+v, want 2", got.ServiceIDs)
	}

	got, err = svc.SetServices(ctxAs("user-a"), e.ID, []string{"gamma"})
	if err != nil {
		t.Fatalf("SetServices (replace): %v", err)
	}
	if len(got.ServiceIDs) != 1 || got.ServiceIDs[0] != "gamma" {
		t.Fatalf("ServiceIDs after replace = %+v, want [gamma]", got.ServiceIDs)
	}
}

func TestVerbsDenyWhenUnauthorized(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: fakeDenyChecker{}}, Store: newFakeStore()}
	if _, err := svc.List(ctxAs("user-a"), "prj-1"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("List: want ErrForbidden, got %v", err)
	}
	if _, err := svc.Create(ctxAs("user-a"), "prj-1", "x"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("Create: want ErrForbidden, got %v", err)
	}
	if err := svc.Delete(ctxAs("user-a"), "env-1"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("Delete: want ErrForbidden, got %v", err)
	}
}

// TestREST_StoreUnavailableIs503 proves ErrEnvironmentsUnavailable — a
// package-local sentinel core.WriteErr cannot name, since core is a leaf —
// surfaces as 503 rather than the silent 500 fallthrough. It used to need
// rest.go's own writeErr wrapper; the sentinel now carries core's
// ErrUnavailable marker instead, so core.WriteErr maps it directly.
func TestREST_StoreUnavailableIs503(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}} // Store nil
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/environments?projectId=prj-1", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /v1/environments with no store: got %d, want 503 (body: %s)", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode REST body: %v", err)
	}
	if body["id"] != "unavailable" || body["message"] != ErrEnvironmentsUnavailable.Error() {
		t.Fatalf("REST body = %#v, want Render unavailable envelope", body)
	}
}

func TestREST_ListUsesRenderCursorEnvelope(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	created, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/environments?projectId=prj-1", nil)
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var body []struct {
		Cursor      string         `json:"cursor"`
		Environment map[string]any `json:"environment"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body: %s)", err, rec.Body.String())
	}
	if len(body) != 1 || body[0].Cursor != created.ID {
		t.Fatalf("list envelope = %#v, want one item cursor %q", body, created.ID)
	}
	got := body[0].Environment
	if got["id"] != created.ID || got["projectId"] != "prj-1" || got["name"] != "staging" {
		t.Fatalf("environment = %#v, want populated Render fields", got)
	}
	if _, ok := got["databasesIds"]; !ok {
		t.Fatalf("environment = %#v, missing Render databasesIds alias", got)
	}
	if _, ok := got["redisIds"]; !ok {
		t.Fatalf("environment = %#v, missing Render redisIds alias", got)
	}
}

func TestVerbs_StoreUnavailable(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}}
	if _, err := svc.List(ctxAs("user-a"), "prj-1"); !errors.Is(err, ErrEnvironmentsUnavailable) {
		t.Errorf("List: want ErrEnvironmentsUnavailable, got %v", err)
	}
	if _, err := svc.Create(ctxAs("user-a"), "prj-1", "x"); !errors.Is(err, ErrEnvironmentsUnavailable) {
		t.Errorf("Create: want ErrEnvironmentsUnavailable, got %v", err)
	}
}
