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
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// databases_test.go covers w6/m20: SetDatabases/SetKeyValues, the
// Database/KeyValue-CR counterparts to SetServices, mirroring
// internal/projects' own (untested) SetDatabases/SetKeyValues shape with
// fakes standing in for *postgres.Service/*keyvalue.Service.

// fakeDatabaseIndex is an in-memory DatabaseIndex — a map of Database name to
// its current view, mutated in place by SetEnvironmentID/SetProjectID so a
// test can assert on the resulting label state after a verb runs.
type fakeDatabaseIndex struct {
	envLayers map[string][]string
	mu        sync.Mutex
	dbs       map[string]postgres.PostgresView
	listCalls int
	// setEnvCalls counts membership WRITES, so a test can prove the diff
	// re-stamps only what actually changed rather than rewriting every member.
	setEnvCalls int
}

func newDatabaseIndex() *fakeDatabaseIndex {
	return &fakeDatabaseIndex{dbs: map[string]postgres.PostgresView{}}
}

func (f *fakeDatabaseIndex) add(v postgres.PostgresView) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dbs[v.ID] = v
}

func (f *fakeDatabaseIndex) ListPostgres(_ context.Context, ownerID string) ([]postgres.PostgresView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	var out []postgres.PostgresView
	for _, d := range f.dbs {
		if ownerID == "" || d.OwnerID == ownerID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeDatabaseIndex) SetEnvironmentID(_ context.Context, name, environmentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setEnvCalls++
	d := f.dbs[name]
	d.EnvironmentID = environmentID
	f.dbs[name] = d
	return nil
}

func (f *fakeDatabaseIndex) SetProjectID(_ context.Context, name, projectID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.dbs[name]
	d.ProjectID = projectID
	f.dbs[name] = d
	return nil
}

// SetEnvironmentIPAllowList (w4/m28) records the projected environment layer
// so a test can assert the fan-out reached this member.
func (f *fakeDatabaseIndex) SetEnvironmentIPAllowList(_ context.Context, name string, cidrs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.envLayers == nil {
		f.envLayers = map[string][]string{}
	}
	f.envLayers[name] = cidrs
	return nil
}

// fakeKeyValueIndex is fakeDatabaseIndex's KeyValue-CR counterpart.
type fakeKeyValueIndex struct {
	envLayers   map[string][]string
	mu          sync.Mutex
	kvs         map[string]keyvalue.KeyValueView
	listCalls   int
	setEnvCalls int
}

func newKeyValueIndex() *fakeKeyValueIndex {
	return &fakeKeyValueIndex{kvs: map[string]keyvalue.KeyValueView{}}
}

func (f *fakeKeyValueIndex) add(v keyvalue.KeyValueView) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kvs[v.ID] = v
}

func (f *fakeKeyValueIndex) ListKeyValues(_ context.Context, ownerID string) ([]keyvalue.KeyValueView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	var out []keyvalue.KeyValueView
	for _, kv := range f.kvs {
		if ownerID == "" || kv.OwnerID == ownerID {
			out = append(out, kv)
		}
	}
	return out, nil
}

func (f *fakeKeyValueIndex) SetEnvironmentID(_ context.Context, name, environmentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setEnvCalls++
	kv := f.kvs[name]
	kv.EnvironmentID = environmentID
	f.kvs[name] = kv
	return nil
}

func (f *fakeKeyValueIndex) SetProjectID(_ context.Context, name, projectID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	kv := f.kvs[name]
	kv.ProjectID = projectID
	f.kvs[name] = kv
	return nil
}

// SetIPAllowList (w6/m19) is fakeDatabaseIndex.SetIPAllowList's KeyValue-CR
// counterpart.
// SetEnvironmentIPAllowList (w4/m28) — fakeDatabaseIndex's counterpart.
func (f *fakeKeyValueIndex) SetEnvironmentIPAllowList(_ context.Context, name string, cidrs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.envLayers == nil {
		f.envLayers = map[string][]string{}
	}
	f.envLayers[name] = cidrs
	return nil
}

func TestSetDatabases_ReplacesFullListAndJoinsProject(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "db-alpha", OwnerID: "tea-a"})
	dbs.add(postgres.PostgresView{ID: "db-beta", OwnerID: "tea-a"})
	dbs.add(postgres.PostgresView{ID: "db-gamma", OwnerID: "tea-a"})
	svc := newService(st)
	svc.Databases = dbs
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	got, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"db-alpha", "db-beta"})
	if err != nil {
		t.Fatalf("SetDatabases: %v", err)
	}
	if len(got.DatabaseIDs) != 2 {
		t.Fatalf("DatabaseIDs = %+v, want 2", got.DatabaseIDs)
	}
	// Assigning to the environment also joins its project (mirroring
	// store.SetEnvironmentServices' apps.project_id stamp).
	if dbs.dbs["db-alpha"].ProjectID != "prj-1" || dbs.dbs["db-beta"].ProjectID != "prj-1" {
		t.Fatalf("assigned databases should join project prj-1: %+v", dbs.dbs)
	}

	got, err = svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"db-gamma"})
	if err != nil {
		t.Fatalf("SetDatabases (replace): %v", err)
	}
	if len(got.DatabaseIDs) != 1 || got.DatabaseIDs[0] != "db-gamma" {
		t.Fatalf("DatabaseIDs after replace = %+v, want [db-gamma]", got.DatabaseIDs)
	}
	// db-alpha/beta left the environment but keep their project — removing
	// from an environment doesn't unjoin its project (matching
	// store.SetEnvironmentServices' own asymmetry for services).
	if dbs.dbs["db-alpha"].EnvironmentID != "" || dbs.dbs["db-beta"].EnvironmentID != "" {
		t.Fatalf("db-alpha/beta should be unassigned from the environment: %+v", dbs.dbs)
	}
	if dbs.dbs["db-alpha"].ProjectID != "prj-1" || dbs.dbs["db-beta"].ProjectID != "prj-1" {
		t.Fatalf("db-alpha/beta should keep project prj-1 after leaving the environment: %+v", dbs.dbs)
	}
}

func TestSetKeyValues_ReplacesFullListAndJoinsProject(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	kvs := newKeyValueIndex()
	kvs.add(keyvalue.KeyValueView{ID: "kv-alpha", OwnerID: "tea-a"})
	kvs.add(keyvalue.KeyValueView{ID: "kv-beta", OwnerID: "tea-a"})
	svc := newService(st)
	svc.KeyValues = kvs
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	got, err := svc.SetKeyValues(ctxAs("user-a"), e.ID, []string{"kv-alpha"})
	if err != nil {
		t.Fatalf("SetKeyValues: %v", err)
	}
	if len(got.KeyValueIDs) != 1 || got.KeyValueIDs[0] != "kv-alpha" {
		t.Fatalf("KeyValueIDs = %+v, want [kv-alpha]", got.KeyValueIDs)
	}
	if kvs.kvs["kv-alpha"].ProjectID != "prj-1" {
		t.Fatalf("kv-alpha should join project prj-1: %+v", kvs.kvs["kv-alpha"])
	}

	got, err = svc.SetKeyValues(ctxAs("user-a"), e.ID, []string{"kv-beta"})
	if err != nil {
		t.Fatalf("SetKeyValues (replace): %v", err)
	}
	if len(got.KeyValueIDs) != 1 || got.KeyValueIDs[0] != "kv-beta" {
		t.Fatalf("KeyValueIDs after replace = %+v, want [kv-beta]", got.KeyValueIDs)
	}
	if kvs.kvs["kv-alpha"].EnvironmentID != "" {
		t.Fatalf("kv-alpha should be unassigned from the environment: %+v", kvs.kvs["kv-alpha"])
	}
}

func TestSetDatabases_UnavailableWhenDatabasesUnwired(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st) // Databases left nil
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	if _, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"db-a"}); !errors.Is(err, ErrEnvironmentsUnavailable) {
		t.Fatalf("SetDatabases with Databases unwired: want ErrEnvironmentsUnavailable, got %v", err)
	}
}

func TestSetKeyValues_UnavailableWhenKeyValuesUnwired(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st) // KeyValues left nil
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	if _, err := svc.SetKeyValues(ctxAs("user-a"), e.ID, []string{"kv-a"}); !errors.Is(err, ErrEnvironmentsUnavailable) {
		t.Fatalf("SetKeyValues with KeyValues unwired: want ErrEnvironmentsUnavailable, got %v", err)
	}
}

// TestGetAndListEnvironment_RoundTripDatabaseAndKeyValueIDs is w6/m20's
// definition-of-done regression test: assigning a Database/KeyValue to an
// Environment persists as a CR label (via the fakes' SetEnvironmentID) and
// round-trips back out through both Get and List.
func TestGetAndListEnvironment_RoundTripDatabaseAndKeyValueIDs(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "db-a", OwnerID: "tea-a"})
	kvs := newKeyValueIndex()
	kvs.add(keyvalue.KeyValueView{ID: "kv-a", OwnerID: "tea-a"})
	svc := newService(st)
	svc.Databases = dbs
	svc.KeyValues = kvs
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	if _, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"db-a"}); err != nil {
		t.Fatalf("SetDatabases: %v", err)
	}
	if _, err := svc.SetKeyValues(ctxAs("user-a"), e.ID, []string{"kv-a"}); err != nil {
		t.Fatalf("SetKeyValues: %v", err)
	}

	got, err := svc.Get(ctxAs("user-a"), e.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.DatabaseIDs) != 1 || got.DatabaseIDs[0] != "db-a" {
		t.Fatalf("Get DatabaseIDs = %+v, want [db-a]", got.DatabaseIDs)
	}
	if len(got.KeyValueIDs) != 1 || got.KeyValueIDs[0] != "kv-a" {
		t.Fatalf("Get KeyValueIDs = %+v, want [kv-a]", got.KeyValueIDs)
	}

	list, err := svc.List(ctxAs("user-a"), "prj-1")
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %+v, err=%v; want exactly one environment", list, err)
	}
	if len(list[0].DatabaseIDs) != 1 || list[0].DatabaseIDs[0] != "db-a" {
		t.Fatalf("List()[0].DatabaseIDs = %+v, want [db-a]", list[0].DatabaseIDs)
	}
	if len(list[0].KeyValueIDs) != 1 || list[0].KeyValueIDs[0] != "kv-a" {
		t.Fatalf("List()[0].KeyValueIDs = %+v, want [kv-a]", list[0].KeyValueIDs)
	}
}

// TestListEnvironments_FetchesDatabasesAndKeyValuesOncePerCall proves List
// shares one tenant-wide ListPostgres/ListKeyValues scan across every
// environment row instead of re-issuing it per environment (a real N+1
// regression the first version of this code had — each row called toFullView,
// which re-fetched the whole tenant's Databases/KeyValues from scratch).
func TestListEnvironments_FetchesDatabasesAndKeyValuesOncePerCall(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "db-a", OwnerID: "tea-a"})
	kvs := newKeyValueIndex()
	kvs.add(keyvalue.KeyValueView{ID: "kv-a", OwnerID: "tea-a"})
	svc := newService(st)
	svc.Databases = dbs
	svc.KeyValues = kvs

	svc.Create(ctxAs("user-a"), "prj-1", "staging")
	svc.Create(ctxAs("user-a"), "prj-1", "production")
	svc.Create(ctxAs("user-a"), "prj-1", "preview")

	dbs.listCalls, kvs.listCalls = 0, 0 // Create no longer fetches membership at all; reset for a clean baseline.
	if _, err := svc.List(ctxAs("user-a"), "prj-1"); err != nil {
		t.Fatalf("List: %v", err)
	}
	if dbs.listCalls != 1 {
		t.Errorf("ListPostgres called %d times across 3 environments, want exactly 1", dbs.listCalls)
	}
	if kvs.listCalls != 1 {
		t.Errorf("ListKeyValues called %d times across 3 environments, want exactly 1", kvs.listCalls)
	}
}

// TestCreateEnvironment_DoesNotFetchDatabasesOrKeyValues proves Create skips
// the membership fetch entirely — a brand-new environment id cannot yet be
// referenced by any Database/KeyValue, so there is nothing to look up.
func TestCreateEnvironment_DoesNotFetchDatabasesOrKeyValues(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	dbs := newDatabaseIndex()
	kvs := newKeyValueIndex()
	svc := newService(st)
	svc.Databases = dbs
	svc.KeyValues = kvs

	if _, err := svc.Create(ctxAs("user-a"), "prj-1", "staging"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if dbs.listCalls != 0 {
		t.Errorf("ListPostgres called %d times by Create, want 0", dbs.listCalls)
	}
	if kvs.listCalls != 0 {
		t.Errorf("ListKeyValues called %d times by Create, want 0", kvs.listCalls)
	}
}
