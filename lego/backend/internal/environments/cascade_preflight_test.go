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
	"cmp"
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type orderedCascadeStore struct{ *fakeStore }

func (s *orderedCascadeStore) ListEnvironments(ctx context.Context, projectID string) ([]store.Environment, error) {
	envs, err := s.fakeStore.ListEnvironments(ctx, projectID)
	slices.SortFunc(envs, func(a, b store.Environment) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return envs, err
}

func cascadePreflightFixture(t *testing.T) (*Service, client.Client, *fakeStore, *fakeEnvGroupIndex) {
	t.Helper()
	svc, cl, st := datastorePlacementService(t, interceptor.Funcs{})
	svc.Store = &orderedCascadeStore{fakeStore: st}
	groups := &fakeEnvGroupIndex{}
	svc.EnvGroups = groups
	for _, pair := range []struct{ name, environmentID string }{{"member", "evm-delete"}, {"other", "evm-other"}} {
		e := st.envs[pair.environmentID]
		e.ProtectedStatus = ProtectedStatusProtected
		e.NetworkIsolationEnabled = true
		e.IPAllowList = []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8"}}
		st.envs[e.ID] = e
		app := sampleApp(pair.name)
		app.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelNetworkIsolation: e.ID}
		app.Spec.IPAllowList = []string{"192.0.2.0/24"}
		app.Spec.EnvironmentIPAllowList = []string{"10.0.0.0/8"}
		if err := cl.Create(t.Context(), app); err != nil {
			t.Fatal(err)
		}
		st.assign[e.ID] = map[string]bool{app.Name: true}
		groups.groups = append(groups.groups, envgroups.EnvGroupView{ID: "evg-" + pair.name, OwnerID: "tea-a", EnvironmentID: e.ID})
	}
	return svc, cl, st, groups
}

func cascadeObjects(t *testing.T, cl client.Client, names ...string) []client.Object {
	t.Helper()
	var objects []client.Object
	for _, name := range names {
		for _, obj := range []client.Object{&appv1alpha1.App{}, &appv1alpha1.Database{}, &appv1alpha1.KeyValue{}} {
			if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "default", Name: name}, obj); err != nil {
				t.Fatal(err)
			}
			objects = append(objects, obj)
		}
	}
	return objects
}

func TestProjectCascadePreflightRefusalLeavesEveryChildAndMemberUnchanged(t *testing.T) {
	svc, cl, st, groups := cascadePreflightFixture(t)
	// The ordinary first child does not require can_manage. Only the later
	// protected child refuses this developer's cascade.
	first := st.envs["evm-delete"]
	first.ProtectedStatus, first.NetworkIsolationEnabled, first.IPAllowList = ProtectedStatusUnprotected, false, nil
	st.envs[first.ID] = first
	for _, obj := range cascadeObjects(t, cl, "member") {
		switch obj := obj.(type) {
		case *appv1alpha1.App:
			delete(obj.Labels, core.LabelNetworkIsolation)
			obj.Spec.EnvironmentIPAllowList = nil
		case *appv1alpha1.Database:
			obj.Spec.EnvironmentIPAllowList = nil
		case *appv1alpha1.KeyValue:
			obj.Spec.EnvironmentIPAllowList = nil
		}
		if err := cl.Update(t.Context(), obj); err != nil {
			t.Fatal(err)
		}
	}
	svc.Authz = developerChecker{}
	before := cascadeObjects(t, cl, "member", "other")
	beforeEnvs, beforeGroups := maps.Clone(st.envs), slices.Clone(groups.groups)
	if err := svc.clearMembersForProject(ctxAs("user-a"), "prj-1"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cascade error = %v, want forbidden", err)
	}
	if got := cascadeObjects(t, cl, "member", "other"); !reflect.DeepEqual(got, before) {
		t.Error("preflight-detectable refusal mutated a child member")
	}
	if !reflect.DeepEqual(groups.groups, beforeGroups) {
		t.Error("preflight-detectable refusal changed environment-group membership")
	}
	if !reflect.DeepEqual(st.envs, beforeEnvs) {
		t.Error("refused cascade changed child environments")
	}
}

func TestProjectCascadeAuthorizedClearsEveryChild(t *testing.T) {
	svc, cl, _, groups := cascadePreflightFixture(t)
	if err := svc.clearMembersForProject(ctxAs("user-a"), "prj-1"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"member", "other"} {
		app := getApp(t, cl, name)
		if app.Labels[core.LabelNetworkIsolation] != "" || len(app.Spec.EnvironmentIPAllowList) != 0 || !slices.Equal(app.Spec.IPAllowList, []string{"192.0.2.0/24"}) {
			t.Errorf("App %s retained inherited policy or lost its own rules: %+v", name, app.Spec)
		}
		for _, obj := range []client.Object{&appv1alpha1.Database{}, &appv1alpha1.KeyValue{}} {
			assertDatastorePlacement(t, cl, obj, name, "prj-1", "", nil)
		}
	}
	for _, group := range groups.groups {
		if group.EnvironmentID != "" {
			t.Errorf("environment group retained membership: %+v", group)
		}
	}
}

type cascadeRevocationChecker struct {
	manageChecks int
	allowChecks  int
}

func (*cascadeRevocationChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (c *cascadeRevocationChecker) CheckFresh(_ context.Context, _, relation, _ string) (bool, error) {
	if relation != core.RelCanManage {
		return true, nil
	}
	c.manageChecks++
	return c.manageChecks <= c.allowChecks, nil
}

func TestProjectCascadeRechecksAuthorizationAfterPreflight(t *testing.T) {
	svc, cl, _, groups := cascadePreflightFixture(t)
	// Both preflight checks succeed; authorization is revoked before the first
	// cleanup check. A cached allow decision must not permit that cleanup.
	svc.Authz = &cascadeRevocationChecker{allowChecks: 2}
	before, beforeGroups := cascadeObjects(t, cl, "member", "other"), slices.Clone(groups.groups)
	if err := svc.clearMembersForProject(ctxAs("user-a"), "prj-1"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("revoked cascade error = %v, want forbidden", err)
	}
	if !reflect.DeepEqual(cascadeObjects(t, cl, "member", "other"), before) || !reflect.DeepEqual(groups.groups, beforeGroups) {
		t.Error("cleanup proceeded after authorization was revoked")
	}
}

func TestProjectCascadeRevocationDuringCleanupStopsLaterChildren(t *testing.T) {
	svc, cl, st, groups := cascadePreflightFixture(t)
	// Preflight and the first cleanup are authorized. Revocation before the
	// second cleanup stops there; it cannot roll back the earlier CR writes.
	svc.Authz = &cascadeRevocationChecker{allowChecks: 3}
	laterBefore := cascadeObjects(t, cl, "other")
	if err := svc.clearMembersForProject(ctxAs("user-a"), "prj-1"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("revoked cascade error = %v, want forbidden", err)
	}
	for _, obj := range []client.Object{&appv1alpha1.Database{}, &appv1alpha1.KeyValue{}} {
		assertDatastorePlacement(t, cl, obj, "member", "prj-1", "", nil)
	}
	if !reflect.DeepEqual(cascadeObjects(t, cl, "other"), laterBefore) || groups.groups[0].EnvironmentID != "" || groups.groups[1].EnvironmentID != "evm-other" {
		t.Error("revocation did not stop at the later child")
	}
	if len(st.envs) != 2 {
		t.Fatal("interrupted cleanup removed child rows")
	}
}
