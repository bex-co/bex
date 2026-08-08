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
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// members_test.go covers the shared membership machinery the three Set verbs,
// the ACL fan-out, and every view now run through: resourceIndex/layeredIndex
// and their three adapters, setResourceMembers' diff, membersByEnvironment, and
// patchApps. Before these were one implementation each, the same behavior was
// written out per member kind — so these tests pin the properties that used to
// be free to drift between the copies.

// TestSetMembers_JoinAndLeaveApplyTheSameSideEffectsPerKind pins what
// membership MEANS for a Database and for a KeyValue: a joiner picks up the
// environment label, the environment's project, and the environment's
// inbound-IP layer (w4/m28); a leaver drops the label and the layer but KEEPS
// the project. Databases and KeyValues are asserted side by side because the
// two used to be independently written 45-line twins — the exact place a
// side effect could be added to one and forgotten on the other.
func TestSetMembers_JoinAndLeaveApplyTheSameSideEffectsPerKind(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	dbs, kvs := newDatabaseIndex(), newKeyValueIndex()
	dbs.add(postgres.PostgresView{ID: "dpg-a", OwnerID: "tea-a"})
	kvs.add(keyvalue.KeyValueView{ID: "red-a", OwnerID: "tea-a"})
	svc.Databases, svc.KeyValues = dbs, kvs

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, false,
		[]core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/24", Description: "office"}}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}

	if _, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"dpg-a"}); err != nil {
		t.Fatalf("SetDatabases: %v", err)
	}
	if _, err := svc.SetKeyValues(ctxAs("user-a"), e.ID, []string{"red-a"}); err != nil {
		t.Fatalf("SetKeyValues: %v", err)
	}
	// Joining stamps all three: environment, project, inbound-IP layer.
	if got := dbs.dbs["dpg-a"]; got.EnvironmentID != e.ID || got.ProjectID != "prj-1" {
		t.Errorf("joined Database = %+v, want environment %q and project prj-1", got, e.ID)
	}
	if got := dbs.envLayers["dpg-a"]; !slices.Equal(got, []string{"10.0.0.0/24"}) {
		t.Errorf("joined Database should inherit the environment layer, got %v", got)
	}
	if got := kvs.kvs["red-a"]; got.EnvironmentID != e.ID || got.ProjectID != "prj-1" {
		t.Errorf("joined KeyValue = %+v, want environment %q and project prj-1", got, e.ID)
	}
	if got := kvs.envLayers["red-a"]; !slices.Equal(got, []string{"10.0.0.0/24"}) {
		t.Errorf("joined KeyValue should inherit the environment layer, got %v", got)
	}

	if _, err := svc.SetDatabases(ctxAs("user-a"), e.ID, nil); err != nil {
		t.Fatalf("SetDatabases (clear): %v", err)
	}
	if _, err := svc.SetKeyValues(ctxAs("user-a"), e.ID, nil); err != nil {
		t.Fatalf("SetKeyValues (clear): %v", err)
	}
	// Leaving drops environment + layer, but never the project.
	if got := dbs.dbs["dpg-a"]; got.EnvironmentID != "" || got.ProjectID != "prj-1" {
		t.Errorf("departed Database = %+v, want no environment but project prj-1 kept", got)
	}
	if got := dbs.envLayers["dpg-a"]; got != nil {
		t.Errorf("departed Database should lose the environment layer, got %v", got)
	}
	if got := kvs.kvs["red-a"]; got.EnvironmentID != "" || got.ProjectID != "prj-1" {
		t.Errorf("departed KeyValue = %+v, want no environment but project prj-1 kept", got)
	}
	if got := kvs.envLayers["red-a"]; got != nil {
		t.Errorf("departed KeyValue should lose the environment layer, got %v", got)
	}
}

// TestSetMembers_RestampsOnlyWhatChanged proves the diff is a diff: a member
// already in the environment and a member in a different environment are both
// left alone, so a no-op Set costs no writes at all. Without this, "replace the
// full list" could be implemented as "clear everything, then set everything"
// and still pass every membership assertion.
func TestSetMembers_RestampsOnlyWhatChanged(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	dbs := newDatabaseIndex()
	svc.Databases = dbs
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	dbs.add(postgres.PostgresView{ID: "dpg-in", OwnerID: "tea-a", EnvironmentID: e.ID})
	dbs.add(postgres.PostgresView{ID: "dpg-elsewhere", OwnerID: "tea-a", EnvironmentID: "env-other"})
	dbs.add(postgres.PostgresView{ID: "dpg-free", OwnerID: "tea-a"})

	// Re-assert the membership that already holds: nothing changes, so nothing
	// is written.
	dbs.setEnvCalls = 0
	if _, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"dpg-in"}); err != nil {
		t.Fatalf("SetDatabases (idempotent): %v", err)
	}
	if dbs.setEnvCalls != 0 {
		t.Errorf("re-asserting the current membership wrote %d times, want 0", dbs.setEnvCalls)
	}
	if got := dbs.dbs["dpg-elsewhere"].EnvironmentID; got != "env-other" {
		t.Errorf("a member of another environment must not be touched, got %q", got)
	}

	// Now one joiner and one leaver: exactly two writes, not four.
	dbs.setEnvCalls = 0
	if _, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"dpg-free"}); err != nil {
		t.Fatalf("SetDatabases (swap): %v", err)
	}
	if dbs.setEnvCalls != 2 {
		t.Errorf("one join + one leave wrote %d times, want exactly 2", dbs.setEnvCalls)
	}
	if got := dbs.dbs["dpg-elsewhere"].EnvironmentID; got != "env-other" {
		t.Errorf("a member of another environment must still not be touched, got %q", got)
	}
}

// TestSetMembers_ForeignWorkspaceIDsAreNotAdopted pins the ignoreUnknownIDs
// policy's security property: the listing is scoped to the environment's OWN
// workspace, so naming a Database that lives in another workspace cannot pull
// it across the tenant boundary — it is simply absent from the diff.
func TestSetMembers_ForeignWorkspaceIDsAreNotAdopted(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "dpg-mine", OwnerID: "tea-a"})
	dbs.add(postgres.PostgresView{ID: "dpg-theirs", OwnerID: "tea-b"})
	svc.Databases = dbs
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"dpg-mine", "dpg-theirs"})
	if err != nil {
		t.Fatalf("SetDatabases: %v", err)
	}
	if !slices.Equal(got.DatabaseIDs, []string{"dpg-mine"}) {
		t.Errorf("DatabaseIDs = %v, want only the same-workspace member", got.DatabaseIDs)
	}
	if d := dbs.dbs["dpg-theirs"]; d.EnvironmentID != "" || d.ProjectID != "" {
		t.Errorf("a Database in another workspace must not be adopted, got %+v", d)
	}
}

// TestSetEnvGroups_UnknownIDIsRefusedBeforeAnyWrite pins the other half of the
// policy: env groups reject an id the workspace listing doesn't know, and
// because the check runs before the diff loop a refusal leaves no partial
// membership change behind. (envgroups_test.go covers the FOREIGN-workspace id;
// this covers an id that exists nowhere at all.)
func TestSetEnvGroups_UnknownIDIsRefusedBeforeAnyWrite(t *testing.T) {
	svc, idx, e := envGroupFixture(t)

	_, err := svc.SetEnvGroups(ctxAs("user-a"), e.ID, []string{"evg-alpha", "evg-nonexistent"})
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("unknown env group id: want ErrForbidden, got %v", err)
	}
	// evg-alpha was named first and is legitimate — but the whole call is
	// refused, so it must not have been assigned on the way to the failure.
	if idx.groups[0].EnvironmentID != "" {
		t.Errorf("refused call assigned %q anyway", idx.groups[0].ID)
	}
	if idx.groups[1].EnvironmentID != e.ID {
		t.Errorf("refused call disturbed the existing membership of %q", idx.groups[1].ID)
	}
}

// TestSetEnvGroups_UnavailableWhenEnvGroupsUnwired completes the unwired-index
// trio (Databases and KeyValues are covered in databases_test.go): every write
// verb reports ErrEnvironmentsUnavailable for its own missing index, and the
// nil check must survive the shared setter's nil-interface handling.
func TestSetEnvGroups_UnavailableWhenEnvGroupsUnwired(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st) // EnvGroups left nil
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	if _, err := svc.SetEnvGroups(ctxAs("user-a"), e.ID, []string{"evg-a"}); !errors.Is(err, ErrEnvironmentsUnavailable) {
		t.Fatalf("SetEnvGroups with EnvGroups unwired: want ErrEnvironmentsUnavailable, got %v", err)
	}
}

// TestView_ResolvesEveryMemberKindTogether exercises all three member kinds
// wired at once — the only configuration that proves membersByEnvironment
// composes Database, KeyValue, and env group membership into one view instead
// of three verbs each resolving their own kind.
func TestView_ResolvesEveryMemberKindTogether(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	dbs, kvs := newDatabaseIndex(), newKeyValueIndex()
	dbs.add(postgres.PostgresView{ID: "dpg-a", OwnerID: "tea-a"})
	kvs.add(keyvalue.KeyValueView{ID: "red-a", OwnerID: "tea-a"})
	groups := &fakeEnvGroupIndex{groups: []envgroups.EnvGroupView{{ID: "evg-a", OwnerID: "tea-a"}}}
	svc.Databases, svc.KeyValues, svc.EnvGroups = dbs, kvs, groups

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"dpg-a"}); err != nil {
		t.Fatalf("SetDatabases: %v", err)
	}
	if _, err := svc.SetKeyValues(ctxAs("user-a"), e.ID, []string{"red-a"}); err != nil {
		t.Fatalf("SetKeyValues: %v", err)
	}
	// The last Set verb's own return value must already carry every OTHER
	// kind's membership — each verb ends at the same view, so none of them can
	// report a partial snapshot.
	got, err := svc.SetEnvGroups(ctxAs("user-a"), e.ID, []string{"evg-a"})
	if err != nil {
		t.Fatalf("SetEnvGroups: %v", err)
	}
	if !slices.Equal(got.DatabaseIDs, []string{"dpg-a"}) ||
		!slices.Equal(got.KeyValueIDs, []string{"red-a"}) ||
		!slices.Equal(got.EnvGroupIDs, []string{"evg-a"}) {
		t.Fatalf("SetEnvGroups view = dbs%v kvs%v groups%v, want all three resolved",
			got.DatabaseIDs, got.KeyValueIDs, got.EnvGroupIDs)
	}
	// Get, Rename, and List reach the same composition by different routes.
	if got, err = svc.Get(ctxAs("user-a"), e.ID); err != nil ||
		!slices.Equal(got.DatabaseIDs, []string{"dpg-a"}) ||
		!slices.Equal(got.KeyValueIDs, []string{"red-a"}) ||
		!slices.Equal(got.EnvGroupIDs, []string{"evg-a"}) {
		t.Fatalf("Get view = %+v err=%v", got, err)
	}
	if got, err = svc.Rename(ctxAs("user-a"), e.ID, "production"); err != nil ||
		got.Name != "production" ||
		!slices.Equal(got.EnvGroupIDs, []string{"evg-a"}) {
		t.Fatalf("Rename view = %+v err=%v", got, err)
	}
	list, err := svc.List(ctxAs("user-a"), "prj-1")
	if err != nil || len(list) != 1 || !slices.Equal(list[0].EnvGroupIDs, []string{"evg-a"}) {
		t.Fatalf("List = %+v err=%v", list, err)
	}
}

// TestView_UnwiredKindsResolveEmptyRatherThanFailing pins the read-side
// degrade: with only one of the three indexes wired, a view still composes —
// the unwired kinds report empty membership instead of failing the whole read.
func TestView_UnwiredKindsResolveEmptyRatherThanFailing(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := newService(st)
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "dpg-a", OwnerID: "tea-a"})
	svc.Databases = dbs // KeyValues and EnvGroups deliberately left nil

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"dpg-a"})
	if err != nil {
		t.Fatalf("SetDatabases: %v", err)
	}
	if !slices.Equal(got.DatabaseIDs, []string{"dpg-a"}) {
		t.Errorf("DatabaseIDs = %v, want [dpg-a]", got.DatabaseIDs)
	}
	// Empty, never nil — all three surfaces serialize these as [].
	if got.KeyValueIDs == nil || len(got.KeyValueIDs) != 0 {
		t.Errorf("KeyValueIDs with KeyValues unwired = %v, want []", got.KeyValueIDs)
	}
	if got.EnvGroupIDs == nil || len(got.EnvGroupIDs) != 0 {
		t.Errorf("EnvGroupIDs with EnvGroups unwired = %v, want []", got.EnvGroupIDs)
	}
}

// failingDatabaseIndex fails whichever DatabaseIndex call the test arms,
// delegating the rest to the ordinary fake. It exists to keep "this kind is
// unwired" (degrade to empty) and "this kind is wired but broken" (fail)
// distinguishable — a distinction that lives in one nil check now that all
// three kinds share idsByEnvironment.
type failingDatabaseIndex struct {
	*fakeDatabaseIndex
	listErr, setErr error
}

func (f failingDatabaseIndex) ListPostgres(ctx context.Context, ownerID string) ([]postgres.PostgresView, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.fakeDatabaseIndex.ListPostgres(ctx, ownerID)
}

func (f failingDatabaseIndex) SetEnvironmentID(ctx context.Context, name, environmentID string) error {
	if f.setErr != nil {
		return f.setErr
	}
	return f.fakeDatabaseIndex.SetEnvironmentID(ctx, name, environmentID)
}

// TestMemberIndexErrors_SurfaceRatherThanDegradeToEmpty pins the line between
// the two: a nil index resolves empty membership (the previous test), but a
// wired index that FAILS must fail the verb. Reads and writes are asserted
// separately because they reach the index by different paths — the view's
// grouping and the diff's join respectively.
func TestMemberIndexErrors_SurfaceRatherThanDegradeToEmpty(t *testing.T) {
	boom := errors.New("postgres index unavailable")

	t.Run("read", func(t *testing.T) {
		st := newFakeStore()
		st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
		svc := newService(st)
		e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		svc.Databases = failingDatabaseIndex{fakeDatabaseIndex: newDatabaseIndex(), listErr: boom}

		if _, err := svc.Get(ctxAs("user-a"), e.ID); !errors.Is(err, boom) {
			t.Errorf("Get with a failing Database index = %v, want the index error", err)
		}
		if _, err := svc.List(ctxAs("user-a"), "prj-1"); !errors.Is(err, boom) {
			t.Errorf("List with a failing Database index = %v, want the index error", err)
		}
	})

	t.Run("write", func(t *testing.T) {
		st := newFakeStore()
		st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
		svc := newService(st)
		e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		idx := newDatabaseIndex()
		idx.add(postgres.PostgresView{ID: "dpg-a", OwnerID: "tea-a"})
		svc.Databases = failingDatabaseIndex{fakeDatabaseIndex: idx, setErr: boom}

		if _, err := svc.SetDatabases(ctxAs("user-a"), e.ID, []string{"dpg-a"}); !errors.Is(err, boom) {
			t.Errorf("SetDatabases with a failing write = %v, want the index error", err)
		}
	})
}

// TestPatchApps_SkipsAppsAlreadyInTheWantedState pins the claim applyACL's
// comment rests on — "a no-op patch for an App whose label already matches" —
// which is what lets it resync unconditionally instead of tracking the
// pre-update value. Both projected fields (the isolation label and the
// inbound-IP layer) share one fan-out now, so both are asserted here.
func TestPatchApps_SkipsAppsAlreadyInTheWantedState(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web"))
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	acl := []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/24"}}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, true, acl); err != nil {
		t.Fatalf("SetACL: %v", err)
	}

	before := getApp(t, cl, "web").ResourceVersion
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	joined := getApp(t, cl, "web")
	// Guard against a vacuous test: joining really does bump the version, so
	// an unchanged version below is evidence of a skipped write, not of a
	// version that never moves.
	if joined.ResourceVersion == before {
		t.Fatalf("joining should have patched web (version stayed %q)", before)
	}
	if joined.Labels[core.LabelNetworkIsolation] != e.ID || len(joined.Spec.EnvironmentIPAllowList) != 1 {
		t.Fatalf("precondition: web should carry the full environment layer, got %+v", joined)
	}

	// Re-applying the identical ACL projects the identical label and layer:
	// nothing to write.
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, true, acl); err != nil {
		t.Fatalf("SetACL (resync): %v", err)
	}
	if got := getApp(t, cl, "web").ResourceVersion; got != joined.ResourceVersion {
		t.Errorf("an unchanged ACL resync patched web anyway (version %q -> %q)", joined.ResourceVersion, got)
	}
}

// TestPatchApps_SkipsMemberNamesWithNoAppCR pins the stale-name degrade the two
// former fan-outs each implemented separately: a member name with no matching
// App CR (a stale row, or a race with a concurrent delete) is skipped rather
// than failing the whole membership change and stranding its other members.
func TestPatchApps_SkipsMemberNamesWithNoAppCR(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web")) // "ghost" has no CR
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, true,
		[]core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/24"}}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}

	got, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"ghost", "web"})
	if err != nil {
		t.Fatalf("SetServices with a dangling member name: %v", err)
	}
	sids := slices.Clone(got.ServiceIDs)
	slices.Sort(sids) // the store lists membership unordered
	if !slices.Equal(sids, []string{"ghost", "web"}) {
		t.Errorf("ServiceIDs = %v, want the store's membership unchanged by the CR skip", got.ServiceIDs)
	}
	// The real member still got both projected fields — the dangling name did
	// not abort the fan-out before reaching it.
	if a := getApp(t, cl, "web"); a.Labels[core.LabelNetworkIsolation] != e.ID || len(a.Spec.EnvironmentIPAllowList) != 1 {
		t.Errorf("web should still carry the environment layer, got labels=%v spec=%v", a.Labels, a.Spec.EnvironmentIPAllowList)
	}
}

// TestProjectMemberClearer_IsTheSeamProjectsCalls exercises the two EXPORTED
// adapter methods rather than the unexported bodies the other tests reach for.
// They are the whole contract projects.Service depends on (structurally — that
// package never imports this one), so a wiring mistake in the adapter would
// otherwise be invisible to this package's tests and only surface as a
// project delete that silently leaves its members isolated.
func TestProjectMemberClearer_IsTheSeamProjectsCalls(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web"), sampleApp("worker"))
	dbs := newDatabaseIndex()
	svc.Databases = dbs
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	dbs.add(postgres.PostgresView{ID: "dpg-a", OwnerID: "tea-a", EnvironmentID: e.ID})
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, true,
		[]core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/24"}}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web", "worker"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}

	clearer := &ProjectMemberClearer{Service: svc}

	// The departing-service seam (projects.Service.SetServices): only the
	// named service is cleared.
	if err := clearer.ClearServiceEnvironmentLayer(ctxAs("user-a"), []string{"web"}); err != nil {
		t.Fatalf("ClearServiceEnvironmentLayer: %v", err)
	}
	if a := getApp(t, cl, "web"); a.Labels[core.LabelNetworkIsolation] != "" || a.Spec.EnvironmentIPAllowList != nil {
		t.Errorf("web should have lost the environment layer, got labels=%v spec=%v", a.Labels, a.Spec.EnvironmentIPAllowList)
	}
	if a := getApp(t, cl, "worker"); a.Labels[core.LabelNetworkIsolation] != e.ID {
		t.Errorf("worker was not named and must keep its layer, got labels=%v", a.Labels)
	}

	// The project-delete seam (projects.Service.Delete): every member of every
	// child environment is cleared, including the Database kind.
	if err := clearer.ClearMembersForProject(ctxAs("user-a"), "prj-1"); err != nil {
		t.Fatalf("ClearMembersForProject: %v", err)
	}
	if a := getApp(t, cl, "worker"); a.Labels[core.LabelNetworkIsolation] != "" || a.Spec.EnvironmentIPAllowList != nil {
		t.Errorf("worker should have lost the environment layer, got labels=%v spec=%v", a.Labels, a.Spec.EnvironmentIPAllowList)
	}
	if got := dbs.envLayers["dpg-a"]; got != nil {
		t.Errorf("the member Database's projected layer should be cleared, got %v", got)
	}
}
