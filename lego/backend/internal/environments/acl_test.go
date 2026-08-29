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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// acl_test.go exercises w6/m19's ACL side effects: core.LabelNetworkIsolation
// projection onto member App CRs (enforcement the operator's NetworkPolicy
// depends on, not just the storage round-trip environments_test.go covers)
// and ipAllowList propagation to Database/KeyValue.

func sampleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
}

func newServiceWithClient(st EnvironmentStore, apps ...*appv1alpha1.App) (*Service, client.Client) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Service{Base: &core.Base{Authz: allowChecker{}, Client: cl, Namespace: "default"}, Store: st}, cl
}

func getApp(t *testing.T, cl client.Client, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &a); err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return &a
}

func TestSetServices_ProjectsLabelWhenIsolationEnabled(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web"), sampleApp("worker"))

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, true, nil); err != nil {
		t.Fatalf("SetACL: %v", err)
	}
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	if got := getApp(t, cl, "web").Labels[core.LabelNetworkIsolation]; got != e.ID {
		t.Errorf("web's core.LabelNetworkIsolation = %q, want %q", got, e.ID)
	}
	if got := getApp(t, cl, "worker").Labels[core.LabelNetworkIsolation]; got != "" {
		t.Errorf("worker (never assigned) should carry no core.LabelNetworkIsolation, got %q", got)
	}
}

func TestSetServices_ClearsLabelWhenServiceLeaves(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web"))

	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, true, nil)
	svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"})
	if got := getApp(t, cl, "web").Labels[core.LabelNetworkIsolation]; got != e.ID {
		t.Fatalf("precondition: web should carry the label, got %q", got)
	}

	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, nil); err != nil {
		t.Fatalf("SetServices (empty): %v", err)
	}
	if got := getApp(t, cl, "web").Labels[core.LabelNetworkIsolation]; got != "" {
		t.Errorf("web should have core.LabelNetworkIsolation cleared after leaving, got %q", got)
	}
}

func TestSetServices_NoLabelWhenIsolationDisabled(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web"))

	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	if got := getApp(t, cl, "web").Labels[core.LabelNetworkIsolation]; got != "" {
		t.Errorf("networkIsolationEnabled=false (default) should never label the App, got %q", got)
	}
}

func TestSetACL_TogglingIsolationSyncsExistingMembers(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web"))

	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"})
	if got := getApp(t, cl, "web").Labels[core.LabelNetworkIsolation]; got != "" {
		t.Fatalf("precondition: no label before isolation is enabled, got %q", got)
	}

	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, true, nil); err != nil {
		t.Fatalf("SetACL (enable isolation): %v", err)
	}
	if got := getApp(t, cl, "web").Labels[core.LabelNetworkIsolation]; got != e.ID {
		t.Errorf("after enabling isolation, want label %q, got %q", e.ID, got)
	}

	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, false, nil); err != nil {
		t.Fatalf("SetACL (disable isolation): %v", err)
	}
	if got := getApp(t, cl, "web").Labels[core.LabelNetworkIsolation]; got != "" {
		t.Errorf("after disabling isolation, want no label, got %q", got)
	}
}

func TestDelete_ClearsLabelsFromMembers(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web"))

	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, true, nil)
	svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"})

	if err := svc.Delete(ctxAs("user-a"), e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := getApp(t, cl, "web").Labels[core.LabelNetworkIsolation]; got != "" {
		t.Errorf("web should have core.LabelNetworkIsolation cleared after its environment is deleted, got %q", got)
	}
}

func TestSetACL_RejectsBadProtectedStatus(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st)
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, "quarantined", false, nil); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("SetACL with a bad protectedStatus: got %v, want ErrBadRequest", err)
	}
}

func TestSetACL_RejectsBadCIDR(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st)
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, false, []core.IPAllowListEntry{{CIDRBlock: "not-a-cidr"}}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("SetACL with a bad CIDR: got %v, want ErrBadRequest", err)
	}
}

// developerChecker allows every relation EXCEPT can_manage — a workspace
// developer below admin. round-5 finding 12: protected-environment ACL controls
// (protectedStatus / networkIsolationEnabled / ipAllowList) are admin-classified,
// so a developer must be refused on the ACL-bearing verbs while still creating
// and renaming ordinary environments.
type developerChecker struct{}

func (developerChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != core.RelCanManage, nil
}

func TestACLMutationsRequireCanManage(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc := &Service{Base: &core.Base{Authz: developerChecker{}}, Store: st}

	// A plain create (no ACL) is allowed for a developer (can_create).
	e, err := svc.CreateWithACL(ctxAs("user-a"), CreateEnvironmentRequest{ProjectID: "prj-1", Name: "staging"})
	if err != nil {
		t.Fatalf("plain create must be allowed for a developer: %v", err)
	}
	// SetACL is admin-only.
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusProtected, true, nil); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("SetACL as developer = %v, want ErrForbidden", err)
	}
	// A create that ALSO arms an ACL is admin-only (and leaves no environment).
	if _, err := svc.CreateWithACL(ctxAs("user-a"), CreateEnvironmentRequest{ProjectID: "prj-1", Name: "prod", ProtectedStatus: ProtectedStatusProtected}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("CreateWithACL(protected) as developer = %v, want ErrForbidden", err)
	}
	// The ACL half of Update is admin-only; a rename-only patch stays allowed.
	protected := ProtectedStatusProtected
	if _, err := svc.Update(ctxAs("user-a"), e.ID, EnvironmentPatch{ProtectedStatus: &protected}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("Update(ACL) as developer = %v, want ErrForbidden", err)
	}
	newName := "staging2"
	if _, err := svc.Update(ctxAs("user-a"), e.ID, EnvironmentPatch{Name: &newName}); err != nil {
		t.Errorf("rename-only Update as developer must be allowed, got %v", err)
	}
}

// codex round-9 #5: deleting an ACL-bearing environment removes exactly the
// administrator boundary SetACL guards — member protection, network isolation,
// and the inbound-IP layer are cleared on the way to the row delete — so the
// developer's can_create must not be enough, and the refusal must leave the
// environment and its member controls untouched.
func TestDeleteACLBearerEnvironmentRequiresCanManage(t *testing.T) {
	cases := []struct {
		name string
		acl  CreateEnvironmentRequest
	}{
		{
			name: "protected status",
			acl:  CreateEnvironmentRequest{ProtectedStatus: ProtectedStatusProtected},
		},
		{
			name: "network isolation",
			acl:  CreateEnvironmentRequest{NetworkIsolationEnabled: true},
		},
		{
			name: "inbound ip allowlist",
			acl:  CreateEnvironmentRequest{IPAllowList: []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
			admin, cl := newServiceWithClient(st, sampleApp("web"))
			dev := &Service{Base: &core.Base{Authz: developerChecker{}}, Store: st}
			ctx := ctxAs("user-a")

			req := CreateEnvironmentRequest{ProjectID: "prj-1", Name: "prod"}
			req.ProtectedStatus = tc.acl.ProtectedStatus
			req.NetworkIsolationEnabled = tc.acl.NetworkIsolationEnabled
			req.IPAllowList = tc.acl.IPAllowList
			e, err := admin.CreateWithACL(ctx, req)
			if err != nil {
				t.Fatalf("admin CreateWithACL: %v", err)
			}
			if _, err := admin.SetServices(ctx, e.ID, []string{"web"}); err != nil {
				t.Fatalf("admin SetServices: %v", err)
			}

			// A developer (can_create, not can_manage) is refused the delete…
			if err := dev.Delete(ctx, e.ID); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("developer Delete of an ACL-bearing environment = %v, want ErrForbidden", err)
			}
			// …and the refusal changed nothing: the row survives…
			if _, err := st.GetEnvironment(ctx, e.ID); err != nil {
				t.Fatalf("refused delete must leave the environment row: %v", err)
			}
			// …and, when isolation is armed, the member's isolation label
			// (the environment id) survives with it.
			if tc.name == "network isolation" {
				if got := getApp(t, cl, "web").Labels[core.LabelNetworkIsolation]; got != e.ID {
					t.Fatalf("refused delete must leave member labels intact: got %q, want %q", got, e.ID)
				}
			}

			// An admin (can_manage) deletes the same environment cleanly.
			if err := admin.Delete(ctx, e.ID); err != nil {
				t.Fatalf("admin Delete of an ACL-bearing environment: %v", err)
			}
		})
	}
}

// A bare environment (no ACL state) keeps the historical developer-level
// delete — round-9 #5 elevates only ACL-bearing environments.
func TestDeletePlainEnvironmentStaysDeveloperLevel(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	dev := &Service{Base: &core.Base{Authz: developerChecker{}}, Store: st}

	e, err := dev.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("developer Create: %v", err)
	}
	if err := dev.Delete(ctxAs("user-a"), e.ID); err != nil {
		t.Fatalf("developer Delete of a plain environment: %v", err)
	}
}

// codex security round 22 #3: every path that can dismantle an ACL-bearing
// environment must inherit the same fresh can_manage boundary as Delete.
// Keeping these cases together pins the shared helper across service links,
// label-backed resource links, and the project-delete cascade seam.
func TestACLBearingMembershipTeardownRequiresCanManage(t *testing.T) {
	ctx := ctxAs("user-a")
	newFixture := func(t *testing.T) (*fakeStore, *Service, store.Environment) {
		t.Helper()
		st := newFakeStore()
		st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
		admin, _ := newServiceWithClient(st, sampleApp("web"))
		view, err := admin.CreateWithACL(ctx, CreateEnvironmentRequest{
			ProjectID:               "prj-1",
			Name:                    "prod",
			NetworkIsolationEnabled: true,
		})
		if err != nil {
			t.Fatalf("admin CreateWithACL: %v", err)
		}
		e, err := st.GetEnvironment(ctx, view.ID)
		if err != nil {
			t.Fatalf("GetEnvironment: %v", err)
		}
		return st, admin, e
	}

	t.Run("service links", func(t *testing.T) {
		st, admin, e := newFixture(t)
		if _, err := admin.SetServices(ctx, e.ID, []string{"web"}); err != nil {
			t.Fatalf("admin SetServices: %v", err)
		}
		dev := &Service{Base: &core.Base{Authz: developerChecker{}}, Store: st}
		if _, err := dev.SetServices(ctx, e.ID, nil); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("developer SetServices on ACL-bearing environment = %v, want ErrForbidden", err)
		}
		members, err := st.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
		if err != nil || len(members) != 1 || members[0] != "web" {
			t.Fatalf("refused service-link mutation changed members: members=%v err=%v", members, err)
		}
	})

	t.Run("database links", func(t *testing.T) {
		st, admin, e := newFixture(t)
		dbs := newDatabaseIndex()
		dbs.add(postgres.PostgresView{ID: "dpg-one", OwnerID: "tea-a", EnvironmentID: e.ID})
		admin.Databases = dbs
		dev := &Service{Base: &core.Base{Authz: developerChecker{}}, Store: st, Databases: dbs}
		if _, err := dev.SetDatabases(ctx, e.ID, nil); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("developer SetDatabases on ACL-bearing environment = %v, want ErrForbidden", err)
		}
		if got := dbs.dbs["dpg-one"].EnvironmentID; got != e.ID {
			t.Fatalf("refused database-link mutation changed environmentID: got %q, want %q", got, e.ID)
		}
	})

	t.Run("project cascade", func(t *testing.T) {
		st, _, e := newFixture(t)
		dev := &Service{Base: &core.Base{Authz: developerChecker{}}, Store: st}
		if err := dev.clearMembersForProject(ctx, e.ProjectID); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("developer project cascade on ACL-bearing environment = %v, want ErrForbidden", err)
		}
		if _, err := st.GetEnvironment(ctx, e.ID); err != nil {
			t.Fatalf("refused project cascade removed environment: %v", err)
		}
	})
}

// --- ipAllowList propagation (t006) ---
//
// Reuses fakeDatabaseIndex/fakeKeyValueIndex (databases_test.go, w6/m20) —
// the same fakes SetDatabases/SetKeyValues are tested against — rather than a
// second, narrower pair of fakes: since w6/m20 gave Database/KeyValue real
// EnvironmentID membership, propagateIPAllowList targets by EnvironmentID
// (not ProjectID, an earlier draft's assumption before m20 landed).

func TestSetACL_PropagatesIPAllowListToEnvironmentMembers(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st)
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "indb", OwnerID: "tea-a", EnvironmentID: e.ID})
	dbs.add(postgres.PostgresView{ID: "outdb", OwnerID: "tea-a", EnvironmentID: "env-other"})
	kvs := newKeyValueIndex()
	kvs.add(keyvalue.KeyValueView{ID: "inkv", OwnerID: "tea-a", EnvironmentID: e.ID})
	kvs.add(keyvalue.KeyValueView{ID: "outkv", OwnerID: "tea-a", EnvironmentID: "env-other"})
	svc.Databases, svc.KeyValues = dbs, kvs

	entries := []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/24", Description: "office"}}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, false, entries); err != nil {
		t.Fatalf("SetACL: %v", err)
	}
	// w4/m28: the fan-out projects the ENVIRONMENT layer (CIDRs only) —
	// the member's own IPAllowList is never touched (pre-m28 it was
	// full-replaced, clobbering service-level rules).
	if got := dbs.envLayers["indb"]; len(got) != 1 || got[0] != "10.0.0.0/24" {
		t.Errorf("indb (member of this environment) should get the projected environment layer, got %v", got)
	}
	if got := dbs.dbs["indb"].IPAllowList; got != nil {
		t.Errorf("indb's own IPAllowList must stay untouched (the pre-m28 clobber is retired), got %v", got)
	}
	if _, touched := dbs.envLayers["outdb"]; touched {
		t.Errorf("outdb (member of a different environment) must not be touched")
	}
	if got := kvs.envLayers["inkv"]; len(got) != 1 || got[0] != "10.0.0.0/24" {
		t.Errorf("inkv (member of this environment) should get the projected environment layer, got %v", got)
	}
	if _, touched := kvs.envLayers["outkv"]; touched {
		t.Errorf("outkv (member of a different environment) must not be touched")
	}
}

// TestSetACL_EmptyListProjectsDenyAll pins w4/m28's empty-means-deny-all: an
// explicitly empty rule set reaches members as the unmatchable placeholder,
// never as a cleared (open) layer.
func TestSetACL_EmptyListProjectsDenyAll(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st)
	e, _ := svc.Create(ctxAs("user-a"), "prj-1", "staging")

	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "indb", OwnerID: "tea-a", EnvironmentID: e.ID})
	svc.Databases = dbs

	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, false, []core.IPAllowListEntry{}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}
	if got := dbs.envLayers["indb"]; len(got) != 1 || got[0] != core.DenyAllCIDR {
		t.Errorf("empty rule set should project the deny-all placeholder, got %v", got)
	}
}

// TestClearServiceEnvironmentLayer_ClearsLabelAndAllowList is w4/m32/t001's
// CR-level proof: projects.Service calls this when a departing service
// carried a stale environment_id, and the App CR — not just a DB row — must
// actually lose the projected fields.
func TestClearServiceEnvironmentLayer_ClearsLabelAndAllowList(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web"), sampleApp("worker"))

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, true, []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	if got := getApp(t, cl, "web"); got.Labels[core.LabelNetworkIsolation] != e.ID || len(got.Spec.EnvironmentIPAllowList) != 1 {
		t.Fatalf("precondition: web should carry the environment layer, got labels=%v spec=%v", got.Labels, got.Spec.EnvironmentIPAllowList)
	}

	if err := svc.clearServiceEnvironmentLayer(ctxAs("user-a"), []string{"web"}); err != nil {
		t.Fatalf("ClearServiceEnvironmentLayer: %v", err)
	}
	got := getApp(t, cl, "web")
	if _, ok := got.Labels[core.LabelNetworkIsolation]; ok {
		t.Errorf("web's core.LabelNetworkIsolation should be cleared, got %v", got.Labels)
	}
	if got.Spec.EnvironmentIPAllowList != nil {
		t.Errorf("web's spec.EnvironmentIPAllowList should be cleared, got %v", got.Spec.EnvironmentIPAllowList)
	}
	if got := getApp(t, cl, "worker").Labels[core.LabelNetworkIsolation]; got != "" {
		t.Errorf("worker (never named) must not be touched")
	}
	// The environment row itself is untouched — this clears a departed
	// service's CR, not the environment.
	if _, err := svc.Get(ctxAs("user-a"), e.ID); err != nil {
		t.Errorf("environment row should still exist: %v", err)
	}
}

// TestClearMembersForProject_ClearsEveryChildEnvironmentsMembers is
// w4/m32/t002's CR-level proof: projects.Service.Delete calls this before
// the project row (and its cascaded environment rows) disappear, and every
// member CR kind — App and Database — must lose the projected layer.
func TestClearMembersForProject_ClearsEveryChildEnvironmentsMembers(t *testing.T) {
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, cl := newServiceWithClient(st, sampleApp("web"))
	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "indb", OwnerID: "tea-a", EnvironmentID: ""})
	svc.Databases = dbs
	kvs := newKeyValueIndex()
	kvs.add(keyvalue.KeyValueView{ID: "inkv", OwnerID: "tea-a", EnvironmentID: ""})
	svc.KeyValues = kvs

	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, false, []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}
	if _, err := svc.SetServices(ctxAs("user-a"), e.ID, []string{"web"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	dbs.dbs["indb"] = postgres.PostgresView{ID: "indb", OwnerID: "tea-a", EnvironmentID: e.ID}
	if err := dbs.SetEnvironmentIPAllowList(context.Background(), "indb", []string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("seed db layer: %v", err)
	}
	kvs.kvs["inkv"] = keyvalue.KeyValueView{ID: "inkv", OwnerID: "tea-a", EnvironmentID: e.ID}
	if err := kvs.SetEnvironmentIPAllowList(context.Background(), "inkv", []string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("seed kv layer: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.EnvironmentIPAllowList; len(got) != 1 {
		t.Fatalf("precondition: web should carry the environment layer, got %v", got)
	}

	if err := svc.clearMembersForProject(ctxAs("user-a"), "prj-1"); err != nil {
		t.Fatalf("ClearMembersForProject: %v", err)
	}
	if got := getApp(t, cl, "web"); got.Spec.EnvironmentIPAllowList != nil {
		t.Errorf("web's spec.EnvironmentIPAllowList should be cleared, got %v", got.Spec.EnvironmentIPAllowList)
	} else if _, ok := got.Labels[core.LabelNetworkIsolation]; ok {
		t.Errorf("web's core.LabelNetworkIsolation should be cleared, got %v", got.Labels)
	}
	if got := dbs.envLayers["indb"]; got != nil {
		t.Errorf("indb's projected environment layer should be cleared, got %v", got)
	}
	if got := kvs.envLayers["inkv"]; got != nil {
		t.Errorf("inkv's projected environment layer should be cleared, got %v", got)
	}
	// The environment row itself is untouched — projects.Service.Delete calls
	// this BEFORE deleting the project row, which is what actually removes it.
	if _, err := svc.Get(ctxAs("user-a"), e.ID); err != nil {
		t.Errorf("environment row should still exist: %v", err)
	}
}

// TestClearMembersForProjectClearsEnvGroupMembership is a regression test for
// a real bug the w4/m32 /simplify pass found and fixed: the first-cut
// clearMembersForProject was written independently of Delete and, unlike
// Delete (TestDeleteEnvironmentClearsEnvGroupMembership, envgroups_test.go),
// never cleared env group membership — silently leaving a group pointing at
// an environment about to be cascade-deleted. Both now share
// clearEnvironmentMembers, so this must hold for the project-cascade path
// exactly as it already does for direct environment deletion.
func TestClearMembersForProjectClearsEnvGroupMembership(t *testing.T) {
	svc, idx, e := envGroupFixture(t)
	if err := svc.clearMembersForProject(ctxAs("user-a"), e.ProjectID); err != nil {
		t.Fatalf("clearMembersForProject: %v", err)
	}
	if idx.groups[1].EnvironmentID != "" {
		t.Fatalf("clearMembersForProject left dangling membership %q", idx.groups[1].EnvironmentID)
	}
}
