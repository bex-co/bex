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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w22/m603: Render's CLI v2.38.0 and SDK v1.10.0 put a sandbox group in a
// project environment, move it and take it out with POST and DELETE
// /environments/{id}/resources alone. The client sequences themselves are
// replayed over REST in internal/sandbox (group_environment_contract_test.go);
// this file holds the membership rules.

// fakeSandboxGroupIndex holds sandbox groups as their store rows; its move is
// the store's compare-and-set: a group no longer where the caller saw it stays
// where it is, and the answer says where that is.
type fakeSandboxGroupIndex struct {
	groups []store.SandboxGroup
	// refuse fails every move of that group.
	refuse string
}

func (f *fakeSandboxGroupIndex) add(workspace string) string {
	g := store.SandboxGroup{ID: ids.New(ids.SandboxGroup), WorkspaceID: workspace}
	f.groups = append(f.groups, g)
	return g.ID
}

func (f *fakeSandboxGroupIndex) SandboxGroups(_ context.Context, workspaceID string) ([]store.SandboxGroup, error) {
	var out []store.SandboxGroup
	for _, g := range f.groups {
		if g.WorkspaceID == workspaceID {
			out = append(out, g)
		}
	}
	return out, nil
}

func (f *fakeSandboxGroupIndex) SetSandboxGroupEnvironment(_ context.Context, workspaceID, id, from, to string) (string, error) {
	if id == f.refuse {
		return "", errors.New("store unavailable")
	}
	for i := range f.groups {
		if g := &f.groups[i]; g.ID == id && g.WorkspaceID == workspaceID {
			if g.EnvironmentID == from {
				g.EnvironmentID = to
			}
			return g.EnvironmentID, nil
		}
	}
	return "", fmt.Errorf("sandbox group: %w", store.ErrNotFound)
}

// in is where group id is: its environment's public id, or "" for none.
func (f *fakeSandboxGroupIndex) in(id string) string {
	for _, g := range f.groups {
		if g.ID == id && g.EnvironmentID != "" {
			return ids.EnvironmentPublicID(g.EnvironmentID)
		}
	}
	return ""
}

// sandboxGroupFixture is a workspace with two environments and the sandbox
// group index wired.
type sandboxGroupFixture struct {
	st                  *fakeStore
	svc                 *Service
	groups              *fakeSandboxGroupIndex
	staging, production string
}

func newSandboxGroupFixture(t *testing.T) sandboxGroupFixture {
	t.Helper()
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	svc := newService(st)
	groups := &fakeSandboxGroupIndex{}
	svc.SandboxGroups = groups
	staging, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatal(err)
	}
	production, err := svc.Create(ctxAs("user-a"), "prj-1", "production")
	if err != nil {
		t.Fatal(err)
	}
	return sandboxGroupFixture{st: st, svc: svc, groups: groups, staging: staging.ID, production: production.ID}
}

// The clients' three calls: an add places a group in no environment, an add
// to another environment moves it there (the CLI "relies on the API to move
// the group out of any environment it was in"), and a removal leaves it in
// none. Removing a group that is not in the environment changes nothing and
// refuses nothing, as for every other kind; the environment's own view lists
// no groups, as Render's environment object does not.
func TestChangeResourcesPlacesMovesAndClearsASandboxGroup(t *testing.T) {
	f := newSandboxGroupFixture(t)
	ctx := ctxAs("user-a")
	g := f.groups.add("tea-a")
	other := f.groups.add("tea-a")

	view, err := f.svc.ChangeResources(ctx, f.staging, []string{g}, true)
	if err != nil || f.groups.in(g) != f.staging {
		t.Fatalf("add to staging = %v; group in %q, want %q", err, f.groups.in(g), f.staging)
	}
	if view.ID != f.staging || EnvironmentOccupied(view) {
		t.Fatalf("add answered %+v, want staging with no member listed", view)
	}
	if _, err := f.svc.ChangeResources(ctx, f.production, []string{g}, true); err != nil || f.groups.in(g) != f.production {
		t.Fatalf("add to production = %v; group in %q, want it moved to %q", err, f.groups.in(g), f.production)
	}
	// Adding it again changes nothing.
	if _, err := f.svc.ChangeResources(ctx, f.production, []string{g}, true); err != nil || f.groups.in(g) != f.production {
		t.Fatalf("second add = %v; group in %q", err, f.groups.in(g))
	}
	// A removal from an environment the group is not in is no change.
	if _, err := f.svc.ChangeResources(ctx, f.staging, []string{g}, false); err != nil || f.groups.in(g) != f.production {
		t.Fatalf("removal from staging = %v; group in %q, want it left in production", err, f.groups.in(g))
	}
	if _, err := f.svc.ChangeResources(ctx, f.production, []string{g}, false); err != nil || f.groups.in(g) != "" {
		t.Fatalf("removal from production = %v; group in %q, want none", err, f.groups.in(g))
	}
	if _, err := f.svc.ChangeResources(ctx, f.production, []string{g}, false); err != nil {
		t.Fatalf("removing a group in no environment = %v, want no change", err)
	}
	if f.groups.in(other) != "" {
		t.Fatalf("a group no request named moved to %q", f.groups.in(other))
	}
}

// One request moves groups out of different environments into a third, and
// each one's source is checked: all land, and none is left behind.
func TestChangeResourcesMovesSandboxGroupsFromSeveralEnvironments(t *testing.T) {
	f := newSandboxGroupFixture(t)
	ctx := ctxAs("user-a")
	third, err := f.svc.Create(ctx, "prj-1", "qa")
	if err != nil {
		t.Fatal(err)
	}
	a, b, loose := f.groups.add("tea-a"), f.groups.add("tea-a"), f.groups.add("tea-a")
	if _, err := f.svc.ChangeResources(ctx, f.staging, []string{a}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChangeResources(ctx, f.production, []string{b}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ChangeResources(ctx, third.ID, []string{a, b, loose}, true); err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{a, b, loose} {
		if f.groups.in(g) != third.ID {
			t.Errorf("%s is in %q, want %s", g, f.groups.in(g), third.ID)
		}
	}
}

// An sbg- id that names no group of the environment's workspace answers 404
// "sandbox group not found", the CLI's own reading of it, on an add and on a
// removal, and identically for another workspace's group and for one that
// does not exist. It refuses the whole request before any member changes: a
// service named beside it is not added.
func TestChangeResourcesRefusesAnUnknownOrForeignSandboxGroupAsNotFound(t *testing.T) {
	f := newSandboxGroupFixture(t)
	ctx := ctxAs("user-a")
	foreign := f.groups.add("tea-b")
	unknown := ids.New(ids.SandboxGroup)
	web := ids.New(ids.Service)

	var first string
	for _, add := range []bool{true, false} {
		for name, id := range map[string]string{"another workspace's group": foreign, "an unknown group": unknown} {
			_, err := f.svc.ChangeResources(ctx, f.staging, []string{web, id}, add)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != core.SandboxGroupNotFoundCode || !errors.Is(err, core.ErrNotFound) {
				t.Fatalf("%s (add %v) = %v, want 404 %s", name, add, err, core.SandboxGroupNotFoundCode)
			}
			if first == "" {
				first = coded.Error()
			} else if coded.Error() != first {
				t.Fatalf("%s answered %q where another not-found answered %q", name, coded.Error(), first)
			}
		}
	}
	if got, _ := f.svc.Get(ctx, f.staging); len(got.ServiceIDs) != 0 {
		t.Fatalf("a refused request added services %v", got.ServiceIDs)
	}
	if f.groups.in(foreign) != "" {
		t.Fatalf("another workspace's group moved to %q", f.groups.in(foreign))
	}

	// A deployment that keeps no groups cannot move one.
	f.svc.SandboxGroups = nil
	if _, err := f.svc.ChangeResources(ctx, f.staging, []string{ids.New(ids.SandboxGroup)}, true); !errors.Is(err, ErrEnvironmentsUnavailable) {
		t.Fatalf("an sbg- id with no group store = %v, want unavailable", err)
	}
}

// A request that fails after its sandbox group moved returns the group to
// where it was: to the environment an add took it from, to none when it was
// in none, and back into the environment a removal took it out of. The group
// moves first, so the failure is a later kind's.
func TestChangeResourcesFailureReturnsASandboxGroupToWhereItWas(t *testing.T) {
	f := newSandboxGroupFixture(t)
	ctx := ctxAs("user-a")
	fromStaging, fromNone := f.groups.add("tea-a"), f.groups.add("tea-a")
	envGroup := ids.New(ids.EnvGroup)
	failing := &failingEnvGroupIndex{refuse: envGroup}
	failing.groups = []envgroups.EnvGroupView{{ID: envGroup, OwnerID: "tea-a"}}
	f.svc.EnvGroups = failing
	if _, err := f.svc.ChangeResources(ctx, f.staging, []string{fromStaging}, true); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.ChangeResources(ctx, f.production, []string{fromStaging, fromNone, envGroup}, true); err == nil {
		t.Fatal("an add whose env group fails succeeded")
	}
	if f.groups.in(fromStaging) != f.staging || f.groups.in(fromNone) != "" {
		t.Fatalf("after the failed add: %s in %q (want staging), %s in %q (want none)",
			fromStaging, f.groups.in(fromStaging), fromNone, f.groups.in(fromNone))
	}

	failing.groups[0].EnvironmentID = ids.EnvironmentStorageID(f.staging)
	if _, err := f.svc.ChangeResources(ctx, f.staging, []string{fromStaging, envGroup}, false); err == nil {
		t.Fatal("a removal whose env group fails succeeded")
	}
	if f.groups.in(fromStaging) != f.staging {
		t.Fatalf("after the failed removal the group is in %q, want it back in staging", f.groups.in(fromStaging))
	}
}

// A group's move that cannot be returned is named in the partial-membership
// conflict, like any member's.
func TestChangeResourcesNamesASandboxGroupItCouldNotReturn(t *testing.T) {
	f := newSandboxGroupFixture(t)
	ctx := ctxAs("user-a")
	g := f.groups.add("tea-a")
	envGroup := ids.New(ids.EnvGroup)
	failing := &failingEnvGroupIndex{refuse: envGroup}
	failing.groups = []envgroups.EnvGroupView{{ID: envGroup, OwnerID: "tea-a"}}
	f.svc.EnvGroups = failing
	f.svc.SandboxGroups = &oneWayGroupIndex{f.groups}

	_, err := f.svc.ChangeResources(ctx, f.staging, []string{g, envGroup}, true)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_MEMBERSHIP_PARTIAL" {
		t.Fatalf("failed add with a failed return = %v, want the partial-membership conflict", err)
	}
	if changed, _ := coded.Params["changedIds"].([]string); !slices.Equal(changed, []string{g}) || coded.Params["failedId"] != envGroup {
		t.Fatalf("partial outcome = %v, want %s still changed and %s failed", coded.Params, g, envGroup)
	}
}

// oneWayGroupIndex places groups but cannot take one out of an environment.
type oneWayGroupIndex struct{ *fakeSandboxGroupIndex }

func (o *oneWayGroupIndex) SetSandboxGroupEnvironment(ctx context.Context, workspaceID, id, from, to string) (string, error) {
	if to == "" {
		return "", errors.New("store unavailable")
	}
	return o.fakeSandboxGroupIndex.SetSandboxGroupEnvironment(ctx, workspaceID, id, from, to)
}

// A group another request moved after this one listed it is not taken from
// where that request put it: the add is refused as a placement change.
func TestChangeResourcesRefusesASandboxGroupMovedMeanwhile(t *testing.T) {
	f := newSandboxGroupFixture(t)
	g := f.groups.add("tea-a")
	racing := &racingGroupIndex{fakeSandboxGroupIndex: f.groups, to: ids.EnvironmentStorageID(f.staging)}
	f.svc.SandboxGroups = racing

	_, err := f.svc.ChangeResources(ctxAs("user-a"), f.production, []string{g}, true)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "RESOURCE_PLACEMENT_CHANGED" {
		t.Fatalf("add of a group moved meanwhile = %v, want RESOURCE_PLACEMENT_CHANGED", err)
	}
	if f.groups.in(g) != f.staging {
		t.Fatalf("the group is in %q, want it left where the other request put it (%s)", f.groups.in(g), f.staging)
	}
}

// racingGroupIndex moves every group to `to` just before the first move it
// is asked for, as a concurrent request would.
type racingGroupIndex struct {
	*fakeSandboxGroupIndex
	to    string
	raced bool
}

func (r *racingGroupIndex) SetSandboxGroupEnvironment(ctx context.Context, workspaceID, id, from, to string) (string, error) {
	if !r.raced {
		r.raced = true
		for i := range r.groups {
			r.groups[i].EnvironmentID = r.to
		}
	}
	return r.fakeSandboxGroupIndex.SetSandboxGroupEnvironment(ctx, workspaceID, id, from, to)
}
