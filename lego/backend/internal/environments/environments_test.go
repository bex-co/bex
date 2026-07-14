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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func (f *fakeStore) SetEnvironmentServices(_ context.Context, environmentID, _, _ string, serviceNames []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := map[string]bool{}
	for _, n := range serviceNames {
		m[n] = true
	}
	f.assign[environmentID] = m
	return nil
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

func (f *fakeStore) SetEnvironmentACL(_ context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []string) error {
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

func newService(st EnvironmentStore) *Service {
	return &Service{Base: &core.Base{Authz: allowChecker{}}, Store: st}
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
	if _, err := svc.Create(ctxAs("user-a"), "prj-1", "staging"); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("want ErrConflict on name collision, got %v", err)
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
// package-local sentinel core.WriteErr's switch doesn't recognize — still
// surfaces as 503, not the silent 500 fallthrough it would get without
// rest.go's own writeErr wrapper.
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
