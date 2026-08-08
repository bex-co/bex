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
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// resources_test.go covers the label-backed membership plumbing shared by
// Databases and KeyValues — the resourceIndex seam, the one-listing-per-kind
// grouping every view is composed from, and the diff that decides which
// resources get re-labeled. SetDatabases/SetKeyValues run the SAME code now, so
// each contract is asserted once per kind to prove the shared path really does
// serve both, and once more for the parts (grouping, workspace scoping) that
// have no per-kind behavior at all.

type relabel struct{ id, projectID string }

// fakeResourceIndex satisfies BOTH DatabaseIndex and KeyValueIndex, mirroring
// how one double stands in for both feature services in the api sweep. Tests
// that count listings wire a separate instance per kind so the counters stay
// independent.
type fakeResourceIndex struct {
	// byWorkspace is the simulated core.LabelProject state: workspace id =>
	// the resources living in it, in listing order.
	byWorkspace map[string][]projectResource
	lists       int
	relabels    []relabel
	listErr     error
}

func newFakeResourceIndex(workspaceID string, members ...projectResource) *fakeResourceIndex {
	return &fakeResourceIndex{byWorkspace: map[string][]projectResource{workspaceID: members}}
}

func (f *fakeResourceIndex) in(workspaceID string, members ...projectResource) *fakeResourceIndex {
	f.byWorkspace[workspaceID] = members
	return f
}

func (f *fakeResourceIndex) list(workspaceID string) ([]projectResource, error) {
	f.lists++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.byWorkspace[workspaceID], nil
}

func (f *fakeResourceIndex) ListPostgres(_ context.Context, ownerID string) ([]postgres.PostgresView, error) {
	rs, err := f.list(ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]postgres.PostgresView, len(rs))
	for i, r := range rs {
		out[i] = postgres.PostgresView{ID: r.id, ProjectID: r.projectID}
	}
	return out, nil
}

func (f *fakeResourceIndex) ListKeyValues(_ context.Context, ownerID string) ([]keyvalue.KeyValueView, error) {
	rs, err := f.list(ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]keyvalue.KeyValueView, len(rs))
	for i, r := range rs {
		out[i] = keyvalue.KeyValueView{ID: r.id, ProjectID: r.projectID}
	}
	return out, nil
}

func (f *fakeResourceIndex) SetProjectID(_ context.Context, name, projectID string) error {
	f.relabels = append(f.relabels, relabel{id: name, projectID: projectID})
	for ws := range f.byWorkspace {
		for i := range f.byWorkspace[ws] {
			if f.byWorkspace[ws][i].id == name {
				f.byWorkspace[ws][i].projectID = projectID
			}
		}
	}
	return nil
}

func (f *fakeResourceIndex) relabelPairs() []string {
	out := make([]string, len(f.relabels))
	for i, r := range f.relabels {
		out[i] = r.id + "=>" + r.projectID
	}
	return out
}

// setKind names one of the two label-backed resource kinds so a contract that
// is genuinely kind-independent can be asserted for both without duplicating
// the test body — the test-side mirror of the single setResourceMembers the
// two verbs now share.
type setKind struct {
	name string
	// wire attaches idx to the service as this kind.
	wire func(svc *Service, idx *fakeResourceIndex)
	// set calls the kind's public verb.
	set func(svc *Service, ctx context.Context, id string, ids []string) (ProjectView, error)
	// ids reads back the kind's membership from a view.
	ids func(v ProjectView) []string
}

var setKinds = []setKind{
	{
		name: "Databases",
		wire: func(svc *Service, idx *fakeResourceIndex) { svc.Databases = idx },
		set: func(svc *Service, ctx context.Context, id string, ids []string) (ProjectView, error) {
			return svc.SetDatabases(ctx, id, ids)
		},
		ids: func(v ProjectView) []string { return v.DatabaseIDs },
	},
	{
		name: "KeyValues",
		wire: func(svc *Service, idx *fakeResourceIndex) { svc.KeyValues = idx },
		set: func(svc *Service, ctx context.Context, id string, ids []string) (ProjectView, error) {
			return svc.SetKeyValues(ctx, id, ids)
		},
		ids: func(v ProjectView) []string { return v.KeyValueIDs },
	},
}

func projectServiceWith(st ProjectStore, checker core.Checker) *Service {
	return &Service{Base: &core.Base{Authz: checker}, Store: st}
}

func join(ids []string) string { return strings.Join(ids, ",") }

// TestSetResourceMembersRelabelsOnlyTheDifference is the core contract of the
// diff both link verbs now share: a resource already in the project stays put,
// a departing one is cleared, a joining one is claimed, and a resource owned by
// a DIFFERENT project that nobody asked for is never touched.
func TestSetResourceMembersRelabelsOnlyTheDifference(t *testing.T) {
	for _, kind := range setKinds {
		t.Run(kind.name, func(t *testing.T) {
			st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
			idx := newFakeResourceIndex("tea-a",
				projectResource{id: "res-keep", projectID: "prj-1"},
				projectResource{id: "res-leave", projectID: "prj-1"},
				projectResource{id: "res-join", projectID: ""},
				projectResource{id: "res-other", projectID: "prj-2"},
				projectResource{id: "res-idle", projectID: ""},
			)
			svc := projectServiceWith(st, allowChecker{})
			kind.wire(svc, idx)

			v, err := kind.set(svc, ctxAs("user-a"), "prj-1", []string{"res-keep", "res-join"})
			if err != nil {
				t.Fatalf("set %s: %v", kind.name, err)
			}
			if got := join(idx.relabelPairs()); got != "res-leave=>,res-join=>prj-1" {
				t.Errorf("relabels = %q, want only the departure and the join", got)
			}
			if got := join(kind.ids(v)); got != "res-keep,res-join" {
				t.Errorf("membership in returned view = %q, want res-keep,res-join", got)
			}
		})
	}
}

// TestSetResourceMembersNeverAdoptsAcrossWorkspaces: the listing the diff runs
// against is scoped to the PROJECT'S workspace, so an id naming a resource in
// another workspace is simply absent from the diff — ignored, never re-labeled
// into this project.
func TestSetResourceMembersNeverAdoptsAcrossWorkspaces(t *testing.T) {
	for _, kind := range setKinds {
		t.Run(kind.name, func(t *testing.T) {
			st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
			idx := newFakeResourceIndex("tea-a", projectResource{id: "res-mine"}).
				in("tea-b", projectResource{id: "res-theirs"})
			svc := projectServiceWith(st, allowChecker{})
			kind.wire(svc, idx)

			v, err := kind.set(svc, ctxAs("user-a"), "prj-1", []string{"res-mine", "res-theirs"})
			if err != nil {
				t.Fatalf("set %s: %v", kind.name, err)
			}
			if got := join(idx.relabelPairs()); got != "res-mine=>prj-1" {
				t.Errorf("relabels = %q, want only the same-workspace resource", got)
			}
			if got := join(kind.ids(v)); got != "res-mine" {
				t.Errorf("membership = %q, want only res-mine", got)
			}
		})
	}
}

// TestSetResourceMembersUnavailableWhenIndexUnwired: the kind's own index nil
// (managed Postgres / KeyValue not wired) makes its link verb report
// ErrProjectsUnavailable — the store-unwired degrade, applied per kind — and it
// must do so without re-labeling anything.
func TestSetResourceMembersUnavailableWhenIndexUnwired(t *testing.T) {
	for _, kind := range setKinds {
		t.Run(kind.name, func(t *testing.T) {
			st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
			// Wire the OTHER kind only, so the failure is specific to this one.
			other := newFakeResourceIndex("tea-a", projectResource{id: "res-a"})
			svc := projectServiceWith(st, allowChecker{})
			for _, k := range setKinds {
				if k.name != kind.name {
					k.wire(svc, other)
				}
			}

			if _, err := kind.set(svc, ctxAs("user-a"), "prj-1", []string{"res-a"}); !errors.Is(err, ErrProjectsUnavailable) {
				t.Fatalf("set %s with the index unwired = %v, want ErrProjectsUnavailable", kind.name, err)
			}
			if len(other.relabels) != 0 {
				t.Errorf("the other kind was re-labeled anyway: %v", other.relabelPairs())
			}
		})
	}
}

// TestSetResourceMembersCrossWorkspaceIsForbidden: the link verbs authorize
// against the PROJECT'S workspace, and a denial has to land before any resource
// is re-labeled — the fetch-then-gate order authorizedProject fixes in one
// place for every id-scoped verb.
func TestSetResourceMembersCrossWorkspaceIsForbidden(t *testing.T) {
	for _, kind := range setKinds {
		t.Run(kind.name, func(t *testing.T) {
			st := newFakeProjectStore(store.Project{ID: "prj-other", TenantID: "tea-other", Name: "theirs"})
			idx := newFakeResourceIndex("tea-other", projectResource{id: "res-theirs"})
			svc := projectServiceWith(st, denyObjectChecker(core.WorkspaceObject("tea-other")))
			kind.wire(svc, idx)

			if _, err := kind.set(svc, ctxAs("user-a"), "prj-other", []string{"res-theirs"}); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("set %s cross-workspace = %v, want ErrForbidden", kind.name, err)
			}
			if len(idx.relabels) != 0 {
				t.Errorf("re-labeled despite the denial: %v", idx.relabelPairs())
			}
		})
	}
}

// TestListEnrichesEveryProjectFromOneListingPerKind pins the reason the
// grouping exists: enriching N projects costs ONE listing per resource kind for
// the whole workspace, not one per project (the N+1 the per-project helpers
// used to do), while each project still gets exactly its own members.
func TestListEnrichesEveryProjectFromOneListingPerKind(t *testing.T) {
	st := newFakeProjectStore(
		store.Project{ID: "prj-1", TenantID: "tea-a", Name: "one"},
		store.Project{ID: "prj-2", TenantID: "tea-a", Name: "two"},
		store.Project{ID: "prj-3", TenantID: "tea-a", Name: "three"},
	)
	dbs := newFakeResourceIndex("tea-a",
		projectResource{id: "dpg-1", projectID: "prj-1"},
		projectResource{id: "dpg-2", projectID: "prj-2"},
		projectResource{id: "dpg-loose", projectID: ""},
	)
	kvs := newFakeResourceIndex("tea-a",
		projectResource{id: "kv-2", projectID: "prj-2"},
	)
	svc := projectServiceWith(st, allowChecker{})
	svc.Databases, svc.KeyValues = dbs, kvs

	vs, err := svc.List(ctxAs("user-a"), "tea-a")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(vs) != 3 {
		t.Fatalf("List returned %d projects, want 3", len(vs))
	}
	if dbs.lists != 1 || kvs.lists != 1 {
		t.Errorf("listings = %d databases / %d key-values for 3 projects, want 1 each", dbs.lists, kvs.lists)
	}
	byID := map[string]ProjectView{}
	for _, v := range vs {
		byID[v.ID] = v
	}
	if got := join(byID["prj-1"].DatabaseIDs); got != "dpg-1" {
		t.Errorf("prj-1 databases = %q, want dpg-1", got)
	}
	if got := join(byID["prj-2"].DatabaseIDs); got != "dpg-2" {
		t.Errorf("prj-2 databases = %q, want dpg-2", got)
	}
	if got := join(byID["prj-2"].KeyValueIDs); got != "kv-2" {
		t.Errorf("prj-2 key-values = %q, want kv-2", got)
	}
	// prj-3 owns nothing, and the unlabeled dpg-loose belongs to no project at
	// all — neither may leak into a project's membership.
	if len(byID["prj-3"].DatabaseIDs) != 0 || len(byID["prj-3"].KeyValueIDs) != 0 {
		t.Errorf("prj-3 = %+v, want no members", byID["prj-3"])
	}
	if len(byID["prj-1"].KeyValueIDs) != 0 {
		t.Errorf("prj-1 key-values = %v, want none", byID["prj-1"].KeyValueIDs)
	}
}

// TestListWithNoProjectsSkipsResourceListings: an empty workspace resolves
// without listing any CRs at all, and still answers with an empty (never nil)
// slice so the REST/GraphQL surfaces render [] rather than null.
func TestListWithNoProjectsSkipsResourceListings(t *testing.T) {
	dbs := newFakeResourceIndex("tea-a", projectResource{id: "dpg-1", projectID: "prj-x"})
	kvs := newFakeResourceIndex("tea-a")
	svc := projectServiceWith(newFakeProjectStore(), allowChecker{})
	svc.Databases, svc.KeyValues = dbs, kvs

	vs, err := svc.List(ctxAs("user-a"), "tea-a")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if vs == nil || len(vs) != 0 {
		t.Fatalf("List of an empty workspace = %#v, want a non-nil empty slice", vs)
	}
	if dbs.lists != 0 || kvs.lists != 0 {
		t.Errorf("listings = %d databases / %d key-values, want none when there are no projects", dbs.lists, kvs.lists)
	}
}

// TestGetResolvesEmptyMembershipWhenIndexesUnwired: a read must degrade to
// empty lists — not an error — when the optional Database/KeyValue indexes are
// absent, matching how every other optional cross-feature index behaves.
func TestGetResolvesEmptyMembershipWhenIndexesUnwired(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
	st.services["prj-1"] = []string{"web"}
	svc := projectServiceWith(st, allowChecker{})

	v, err := svc.Get(ctxAs("user-a"), "prj-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if join(v.ServiceIDs) != "web" {
		t.Errorf("services = %v, want [web]", v.ServiceIDs)
	}
	if v.DatabaseIDs == nil || len(v.DatabaseIDs) != 0 || v.KeyValueIDs == nil || len(v.KeyValueIDs) != 0 {
		t.Errorf("membership = %+v, want non-nil empty database/key-value lists", v)
	}
}

// TestResourceListingErrorSurfacesFromEveryReader: a failing CR listing is a
// hard error on every verb that composes a view — the shared tail must not
// swallow it into a silently incomplete membership list.
func TestResourceListingErrorSurfacesFromEveryReader(t *testing.T) {
	boom := errors.New("listing the CRs failed")
	verbs := map[string]func(*Service) error{
		"List":         func(s *Service) error { _, err := s.List(ctxAs("user-a"), "tea-a"); return err },
		"Get":          func(s *Service) error { _, err := s.Get(ctxAs("user-a"), "prj-1"); return err },
		"Rename":       func(s *Service) error { _, err := s.Rename(ctxAs("user-a"), "prj-1", "next"); return err },
		"SetServices":  func(s *Service) error { _, err := s.SetServices(ctxAs("user-a"), "prj-1", nil); return err },
		"SetDatabases": func(s *Service) error { _, err := s.SetDatabases(ctxAs("user-a"), "prj-1", nil); return err },
		"SetKeyValues": func(s *Service) error { _, err := s.SetKeyValues(ctxAs("user-a"), "prj-1", nil); return err },
	}
	for name, call := range verbs {
		t.Run(name, func(t *testing.T) {
			st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "stack"})
			idx := newFakeResourceIndex("tea-a")
			idx.listErr = boom
			svc := projectServiceWith(st, allowChecker{})
			svc.Databases, svc.KeyValues = idx, idx

			if err := call(svc); !errors.Is(err, boom) {
				t.Fatalf("%s = %v, want the listing error", name, err)
			}
		})
	}
}

// TestRenameReturnsTheNewNameWithCurrentMembership: Rename's view tail is the
// same one Get uses, so the renamed project comes back fully populated rather
// than with the empty lists a create returns.
func TestRenameReturnsTheNewNameWithCurrentMembership(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "before"})
	st.services["prj-1"] = []string{"web", "worker"}
	svc := projectServiceWith(st, allowChecker{})
	svc.Databases = newFakeResourceIndex("tea-a", projectResource{id: "dpg-1", projectID: "prj-1"})

	v, err := svc.Rename(ctxAs("user-a"), "prj-1", "after")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if v.Name != "after" || v.OwnerID != "tea-a" {
		t.Errorf("renamed view = %+v, want name after in tea-a", v)
	}
	if join(v.ServiceIDs) != "web,worker" || join(v.DatabaseIDs) != "dpg-1" {
		t.Errorf("membership after rename = %+v, want the pre-existing members", v)
	}
	stored, err := st.GetProject(ctxAs("user-a"), "prj-1")
	if err != nil || stored.Name != "after" {
		t.Errorf("stored project = %+v (err %v), want the rename persisted", stored, err)
	}
}

// TestRenameCrossWorkspaceIsForbiddenAndDoesNotWrite: Rename authorizes against
// the project's own workspace before touching the row.
func TestRenameCrossWorkspaceIsForbiddenAndDoesNotWrite(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-other", TenantID: "tea-other", Name: "theirs"})
	svc := projectServiceWith(st, denyObjectChecker(core.WorkspaceObject("tea-other")))

	if _, err := svc.Rename(ctxAs("user-a"), "prj-other", "mine"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-workspace Rename = %v, want ErrForbidden", err)
	}
	stored, err := st.GetProject(ctxAs("user-a"), "prj-other")
	if err != nil || stored.Name != "theirs" {
		t.Errorf("stored project = %+v (err %v), want the name untouched", stored, err)
	}
}

// TestCreateReturnsEmptyMembershipSlices: a fresh project has no members yet,
// and the three lists must serialize as [] rather than null.
func TestCreateReturnsEmptyMembershipSlices(t *testing.T) {
	svc := projectServiceWith(newFakeProjectStore(), allowChecker{})
	svc.Databases = newFakeResourceIndex("tea-a", projectResource{id: "dpg-1", projectID: "prj-old"})

	v, err := svc.Create(ctxAs("user-a"), "tea-a", "fresh")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.Name != "fresh" || v.OwnerID != "tea-a" {
		t.Errorf("created view = %+v, want name fresh in tea-a", v)
	}
	if v.ServiceIDs == nil || v.DatabaseIDs == nil || v.KeyValueIDs == nil {
		t.Errorf("created view = %+v, want non-nil empty membership lists", v)
	}
	if len(v.ServiceIDs)+len(v.DatabaseIDs)+len(v.KeyValueIDs) != 0 {
		t.Errorf("created view = %+v, want no members", v)
	}
}

// TestUnavailableStoreShortCircuitsEveryIDScopedVerb: with the control-plane
// store unwired there is nothing to fetch, so every id-scoped verb reports
// ErrProjectsUnavailable through the one preamble they share.
func TestUnavailableStoreShortCircuitsEveryIDScopedVerb(t *testing.T) {
	verbs := map[string]func(*Service) error{
		"Get":          func(s *Service) error { _, err := s.Get(ctxAs("user-a"), "prj-1"); return err },
		"Rename":       func(s *Service) error { _, err := s.Rename(ctxAs("user-a"), "prj-1", "x"); return err },
		"Delete":       func(s *Service) error { return s.Delete(ctxAs("user-a"), "prj-1") },
		"SetServices":  func(s *Service) error { _, err := s.SetServices(ctxAs("user-a"), "prj-1", nil); return err },
		"SetDatabases": func(s *Service) error { _, err := s.SetDatabases(ctxAs("user-a"), "prj-1", nil); return err },
		"SetKeyValues": func(s *Service) error { _, err := s.SetKeyValues(ctxAs("user-a"), "prj-1", nil); return err },
	}
	for name, call := range verbs {
		t.Run(name, func(t *testing.T) {
			idx := newFakeResourceIndex("tea-a", projectResource{id: "res-a"})
			svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Databases: idx, KeyValues: idx}
			if err := call(svc); !errors.Is(err, ErrProjectsUnavailable) {
				t.Fatalf("%s with no store = %v, want ErrProjectsUnavailable", name, err)
			}
			if len(idx.relabels) != 0 {
				t.Errorf("%s re-labeled resources with no store: %v", name, idx.relabelPairs())
			}
		})
	}
}
