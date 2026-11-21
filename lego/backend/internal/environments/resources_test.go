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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/188: Render's POST/DELETE /environments/{id}/resources add and remove
// members by id (each kind by its prefix), leaving the other members as they
// are, and deleting a non-empty environment is a 409.
func TestChangeResourcesAddsAndRemovesByID(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	dbA, dbB := ids.New(ids.Postgres), ids.New(ids.Postgres)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: dbA, OwnerID: "tea-a"})
	dbs.add(postgres.PostgresView{ID: dbB, OwnerID: "tea-a"})
	svc := newService(st)
	svc.Databases = dbs
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	got, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, true)
	if err != nil || !slices.Equal(got.DatabaseIDs, []string{dbA}) {
		t.Fatalf("add dbA = %+v, %v", got.DatabaseIDs, err)
	}
	// Adding another keeps the first; adding a member again changes nothing.
	got, err = svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbB, dbA}, true)
	if err != nil || len(got.DatabaseIDs) != 2 {
		t.Fatalf("add dbB = %+v, %v", got.DatabaseIDs, err)
	}
	got, err = svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, false)
	if err != nil || !slices.Equal(got.DatabaseIDs, []string{dbB}) {
		t.Fatalf("remove dbA = %+v, %v", got.DatabaseIDs, err)
	}
	// Removing a non-member is a no-op.
	if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, false); err != nil {
		t.Fatalf("remove non-member: %v", err)
	}
}

func TestChangeResourcesRefusalsChangeNothing(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	dbA := ids.New(ids.Postgres)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: dbA, OwnerID: "tea-a"})
	svc := newService(st)
	svc.Databases = dbs
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	for name, resourceIDs := range map[string][]string{
		"empty":                  {},
		"not an id":              {"nope"},
		"an id of another kind":  {ids.New(ids.Project)},
		"one bad among good ids": {dbA, "nope"},
	} {
		if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, resourceIDs, true); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s: err = %v, want a bad request", name, err)
		}
	}
	if got, _ := svc.Get(ctxAs("user-a"), e.ID); len(got.DatabaseIDs) != 0 {
		t.Errorf("a refused request changed membership: %+v", got.DatabaseIDs)
	}
	// A database outside the workspace is refused like any link.
	foreign := ids.New(ids.Postgres)
	if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{foreign}, true); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("foreign id: err = %v, want forbidden", err)
	}
}

// noReversalStore places services in an environment but cannot take them out,
// so reversing an add fails.
type noReversalStore struct{ *fakeStore }

func (s noReversalStore) ChangeEnvironmentServices(ctx context.Context, environmentID, projectID, tenantID string, serviceIDs []string, add bool, exit core.EnvironmentExitGuard) ([]core.ServicePlacementChange, error) {
	if !add {
		return nil, errors.New("store unavailable")
	}
	return s.fakeStore.ChangeEnvironmentServices(ctx, environmentID, projectID, tenantID, serviceIDs, add, exit)
}

func (noReversalStore) ReturnEnvironmentServices(context.Context, string, string, string, string, []string, core.EnvironmentExitGuard) ([]core.ServicePlacementChange, error) {
	return nil, errors.New("store unavailable")
}

// A request that fails partway reverses only the members it changed, never one
// it did not name; when the reversal fails too, the answer is the
// partial-membership conflict naming what may still be changed.
func TestChangeResourcesFailureReversesOnlyWhatItChanged(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	web, api, group := ids.New(ids.Service), ids.New(ids.Service), ids.New(ids.EnvGroup)
	groups := &failingEnvGroupIndex{refuse: group}
	groups.groups = []envgroups.EnvGroupView{{ID: group, OwnerID: "tea-a"}}
	svc := newService(st)
	svc.EnvGroups = groups
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{api}, true); err != nil {
		t.Fatal(err)
	}

	// The group's join fails after web joined: web leaves again, api stays.
	_, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{web, group}, true)
	var coded *core.CodedError
	if err == nil || errors.As(err, &coded) {
		t.Fatalf("failed add = %v, want the group's own failure", err)
	}
	if got, _ := svc.Get(ctxAs("user-a"), e.ID); !slices.Equal(got.ServiceIDs, []string{api}) || len(got.EnvGroupIDs) != 0 {
		t.Fatalf("after the reversal: services %v, groups %v; want only %s", got.ServiceIDs, got.EnvGroupIDs, api)
	}

	// The same failure with a reversal that fails names the member still in.
	svc.Store = noReversalStore{st}
	_, err = svc.ChangeResources(ctxAs("user-a"), e.ID, []string{web, group}, true)
	if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_MEMBERSHIP_PARTIAL" || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("failed add with a failed reversal = %v, want the partial-membership conflict", err)
	}
	if changed, _ := coded.Params["changedIds"].([]string); !slices.Equal(changed, []string{web}) || coded.Params["failedId"] != group {
		t.Fatalf("partial outcome = %v, want %s changed and %s failed", coded.Params, web, group)
	}
}

// A failed add returns each member it moved to where it was: a service and a
// database it took from another environment go back there, with that
// environment's isolation and inbound-IP layer, and a service it placed from
// no environment leaves again. The old reversal left the moved ones in no
// environment.
func TestChangeResourcesFailureReturnsMovedMembersToTheirEnvironment(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	web, worker, db, group := ids.New(ids.Service), ids.New(ids.Service), ids.New(ids.Postgres), ids.New(ids.EnvGroup)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: db, OwnerID: "tea-a"})
	groups := &failingEnvGroupIndex{refuse: group}
	groups.groups = []envgroups.EnvGroupView{{ID: group, OwnerID: "tea-a"}}
	svc, cl := newServiceWithClient(st, sampleApp(web), sampleApp(worker))
	svc.Databases, svc.EnvGroups = dbs, groups
	staging, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	production, _ := svc.Create(ctxAs("user-a"), "prj-1", "production")
	office := []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8", Description: "office"}}
	if err := st.SetEnvironmentACL(context.Background(), ids.EnvironmentStorageID(staging.ID), ProtectedStatusUnprotected, true, office); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ChangeResources(ctxAs("user-a"), staging.ID, []string{web, db}, true); err != nil {
		t.Fatal(err)
	}

	// The group's join fails after web, worker and db joined production.
	_, err := svc.ChangeResources(ctxAs("user-a"), production.ID, []string{web, worker, db, group}, true)
	var coded *core.CodedError
	if err == nil || errors.As(err, &coded) {
		t.Fatalf("failed add = %v, want the group's own failure", err)
	}
	gotStaging, _ := svc.Get(ctxAs("user-a"), staging.ID)
	gotProduction, _ := svc.Get(ctxAs("user-a"), production.ID)
	if !slices.Equal(gotStaging.ServiceIDs, []string{web}) || !slices.Equal(gotStaging.DatabaseIDs, []string{db}) {
		t.Fatalf("staging after the reversal: services %v, databases %v; want %s and %s back", gotStaging.ServiceIDs, gotStaging.DatabaseIDs, web, db)
	}
	if !EnvironmentOccupied(gotStaging) || EnvironmentOccupied(gotProduction) {
		t.Fatalf("production after the reversal = %+v, want it empty", gotProduction)
	}
	stagingID := ids.EnvironmentStorageID(staging.ID)
	cidrs := core.EnvironmentLayerCIDRs(office)
	if app := getApp(t, cl, web); app.Labels[core.LabelNetworkIsolation] != stagingID || !slices.Equal(app.Spec.EnvironmentIPAllowList, cidrs) {
		t.Errorf("returned %s carries isolation %q and IP layer %v, want staging's %q and %v",
			web, app.Labels[core.LabelNetworkIsolation], app.Spec.EnvironmentIPAllowList, stagingID, cidrs)
	}
	if !dbs.isolation[db] || !slices.Equal(dbs.envLayers[db], cidrs) {
		t.Errorf("returned %s carries isolation %v and IP layer %v, want staging's", db, dbs.isolation[db], dbs.envLayers[db])
	}
	if app := getApp(t, cl, worker); app.Labels[core.LabelEnvironment] != "" || len(app.Spec.EnvironmentIPAllowList) != 0 {
		t.Errorf("%s, placed from no environment, kept production's labels %v and IP layer %v", worker, app.Labels, app.Spec.EnvironmentIPAllowList)
	}
}

// A failed add returns a datastore it placed from no environment to the
// project it was in, with none of the target's rules: one from another
// project goes back to it, one in no project leaves the target's, and one
// already in the target's project stays there. The old reversal only left the
// environment, so each kept the target's project.
func TestChangeResourcesFailureReturnsAMemberPlacedFromNoEnvironmentToItsProject(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	st.addProject(store.Project{ID: "prj-0", TenantID: "tea-a", Name: "other"})
	db, kv, kvHere, group := ids.New(ids.Postgres), ids.New(ids.KeyValue), ids.New(ids.KeyValue), ids.New(ids.EnvGroup)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: db, OwnerID: "tea-a", ProjectID: "prj-0"})
	kvs := newKeyValueIndex()
	kvs.add(keyvalue.KeyValueView{ID: kv, OwnerID: "tea-a"})
	kvs.add(keyvalue.KeyValueView{ID: kvHere, OwnerID: "tea-a", ProjectID: "prj-1"})
	groups := &failingEnvGroupIndex{refuse: group}
	groups.groups = []envgroups.EnvGroupView{{ID: group, OwnerID: "tea-a"}}
	svc := newService(st)
	svc.Databases, svc.KeyValues, svc.EnvGroups = dbs, kvs, groups
	production, _ := svc.Create(ctxAs("user-a"), "prj-1", "production")
	office := []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8", Description: "office"}}
	if err := st.SetEnvironmentACL(context.Background(), ids.EnvironmentStorageID(production.ID), ProtectedStatusUnprotected, true, office); err != nil {
		t.Fatal(err)
	}

	// The group's join fails after the three datastores joined production.
	_, err := svc.ChangeResources(ctxAs("user-a"), production.ID, []string{db, kv, kvHere, group}, true)
	var coded *core.CodedError
	if err == nil || errors.As(err, &coded) {
		t.Fatalf("failed add = %v, want the group's own failure", err)
	}
	if got := dbs.dbs[db]; got.EnvironmentID != "" || got.ProjectID != "prj-0" || len(dbs.envLayers[db]) != 0 || dbs.isolation[db] {
		t.Errorf("%s after the reversal: environment %q, project %q, IP layer %v, isolated %v; want no environment, prj-0 and no rules",
			db, got.EnvironmentID, got.ProjectID, dbs.envLayers[db], dbs.isolation[db])
	}
	for name, want := range map[string]string{kv: "", kvHere: "prj-1"} {
		got := kvs.kvs[name]
		if got.EnvironmentID != "" || got.ProjectID != want || len(kvs.envLayers[name]) != 0 || kvs.isolation[name] {
			t.Errorf("%s after the reversal: environment %q, project %q, IP layer %v, isolated %v; want no environment, project %q and no rules",
				name, got.EnvironmentID, got.ProjectID, kvs.envLayers[name], kvs.isolation[name], want)
		}
	}
	if got, _ := svc.Get(ctxAs("user-a"), production.ID); EnvironmentOccupied(got) {
		t.Fatalf("production after the reversal = %+v, want it empty", got)
	}
}

// racingEnvGroupIndex refuses to move one named group, and first lets a
// concurrent request run.
type racingEnvGroupIndex struct {
	fakeEnvGroupIndex
	refuse string
	race   func()
}

func (f *racingEnvGroupIndex) SetEnvironmentID(ctx context.Context, id, expectedEnvironmentID, environmentID string) error {
	if id == f.refuse {
		if race := f.race; race != nil {
			f.race = nil
			race()
		}
		return errors.New("secret store: connection reset by 10.0.0.5")
	}
	return f.fakeEnvGroupIndex.SetEnvironmentID(ctx, id, expectedEnvironmentID, environmentID)
}

// A failed membership change returns each member only while it is still where
// this request put it. Without a concurrent request, env groups go back too:
// one taken from staging returns there, one placed from no environment leaves.
// When another request moves the members first, every one stays where that
// request put it: the groups an add moved (env-group reversal went through the
// public group move, which takes a group from wherever it is), and the service,
// database and group a removal took out (the rejoin listed them again and
// joined them from where it found them).
func TestChangeResourcesFailureLeavesWhatAConcurrentRequestMoved(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	web, db := ids.New(ids.Service), ids.New(ids.Postgres)
	moved, placed, failing := ids.New(ids.EnvGroup), ids.New(ids.EnvGroup), ids.New(ids.EnvGroup)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: db, OwnerID: "tea-a"})
	groups := &racingEnvGroupIndex{refuse: failing}
	groups.groups = []envgroups.EnvGroupView{{ID: moved, OwnerID: "tea-a"}, {ID: placed, OwnerID: "tea-a"}, {ID: failing, OwnerID: "tea-a"}}
	svc, _ := newServiceWithClient(st, sampleApp(web))
	svc.Databases, svc.EnvGroups = dbs, groups
	staging, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	production, _ := svc.Create(ctxAs("user-a"), "prj-1", "production")
	qa, _ := svc.Create(ctxAs("user-a"), "prj-1", "qa")
	in := func(environmentID string) string { return ids.EnvironmentStorageID(environmentID) }
	groupIn := func(gid string) string {
		for _, g := range groups.groups {
			if g.ID == gid {
				return g.EnvironmentID
			}
		}
		return "missing"
	}
	if _, err := svc.ChangeResources(ctxAs("user-a"), staging.ID, []string{moved}, true); err != nil {
		t.Fatal(err)
	}

	// No concurrent request: both groups go back.
	if _, err := svc.ChangeResources(ctxAs("user-a"), production.ID, []string{moved, placed, failing}, true); err == nil {
		t.Fatal("the add with a failing group succeeded")
	}
	if groupIn(moved) != in(staging.ID) || groupIn(placed) != "" {
		t.Fatalf("after the reversal: %s in %q, %s in %q; want staging and none", moved, groupIn(moved), placed, groupIn(placed))
	}

	// Another request moves both groups on to qa before the add fails.
	groups.race = func() {
		if _, err := svc.ChangeResources(ctxAs("user-a"), qa.ID, []string{moved, placed}, true); err != nil {
			t.Errorf("the concurrent move: %v", err)
		}
	}
	if _, err := svc.ChangeResources(ctxAs("user-a"), production.ID, []string{moved, placed, failing}, true); err == nil {
		t.Fatal("the add with a failing group succeeded")
	}
	if groupIn(moved) != in(qa.ID) || groupIn(placed) != in(qa.ID) {
		t.Fatalf("after the reversal: %s in %q, %s in %q; want both in qa, where the concurrent request put them", moved, groupIn(moved), placed, groupIn(placed))
	}

	// A removal: another request places what it took out in qa before it fails.
	if _, err := svc.ChangeResources(ctxAs("user-a"), production.ID, []string{web, db, moved}, true); err != nil {
		t.Fatal(err)
	}
	groups.groups[2].EnvironmentID = in(production.ID)
	groups.race = func() {
		if _, err := svc.ChangeResources(ctxAs("user-a"), qa.ID, []string{web, db, moved}, true); err != nil {
			t.Errorf("the concurrent move: %v", err)
		}
	}
	if _, err := svc.ChangeResources(ctxAs("user-a"), production.ID, []string{web, db, moved, failing}, false); err == nil {
		t.Fatal("the removal with a failing group succeeded")
	}
	gotQA, _ := svc.Get(ctxAs("user-a"), qa.ID)
	if !slices.Equal(gotQA.ServiceIDs, []string{web}) || !slices.Equal(gotQA.DatabaseIDs, []string{db}) || groupIn(moved) != in(qa.ID) {
		t.Fatalf("qa after the reversal: services %v, databases %v, %s in %q; want all three in qa", gotQA.ServiceIDs, gotQA.DatabaseIDs, moved, groupIn(moved))
	}
	if got, _ := svc.Get(ctxAs("user-a"), production.ID); len(got.ServiceIDs)+len(got.DatabaseIDs) != 0 || !slices.Equal(got.EnvGroupIDs, []string{failing}) {
		t.Fatalf("production after the reversal = %+v, want only %s", got, failing)
	}
}

func TestDeleteIfEmptyRefusesAnOccupiedEnvironment(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	dbA := ids.New(ids.Postgres)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: dbA, OwnerID: "tea-a"})
	svc := newService(st)
	svc.Databases = dbs
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, true); err != nil {
		t.Fatal(err)
	}

	err := svc.DeleteIfEmpty(ctxAs("user-a"), e.ID)
	var coded *core.CodedError
	if !errors.Is(err, core.ErrConflict) || !errors.As(err, &coded) || coded.Code != codeEnvironmentNotEmpty {
		t.Fatalf("delete of an occupied environment = %v, want 409 %s", err, codeEnvironmentNotEmpty)
	}
	if _, err := svc.Get(ctxAs("user-a"), e.ID); err != nil {
		t.Fatalf("the refused delete removed the environment: %v", err)
	}
	if _, err := svc.ChangeResources(ctxAs("user-a"), e.ID, []string{dbA}, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteIfEmpty(ctxAs("user-a"), e.ID); err != nil {
		t.Fatalf("delete of the emptied environment: %v", err)
	}
	// The confirmed flow (dashboard, GraphQL) still detaches and deletes.
	e2, _ := svc.Create(ctxAs("user-a"), "prj-1", "prod")
	if _, err := svc.ChangeResources(ctxAs("user-a"), e2.ID, []string{dbA}, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctxAs("user-a"), e2.ID); err != nil {
		t.Fatalf("Delete of an occupied environment: %v", err)
	}
}
