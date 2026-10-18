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

package envgroups

import (
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// apiApp is a service as the API creates it: a public srv- id beside its name.
func apiApp(name, appID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{core.LabelAppID: appID, core.LabelServiceName: name, core.LabelTenant: core.DefaultTenant}
	return a
}

// setLinks overwrites group gid's stored link set.
func setLinks(t *testing.T, store *fakeStore, gid string, links ...string) {
	t.Helper()
	row, ok := groupsOf(store).Row(gid)
	if !ok {
		t.Fatalf("no group %s", gid)
	}
	row.ServiceLinks = links
	groupsOf(store).Set(row)
}

func TestEnvGroup_PatchSkipsAServiceThatDoesNotMountTheGroup(t *testing.T) {
	store := newFakeStore()
	svc := newService(store, apiApp("web", id.New(id.Service)))
	ctx := testCtx()
	if err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"A": "1"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.LinkEnvGroup(ctx, "shared", "web"); err != nil {
		t.Fatal(err)
	}
	gid := idNamed(ctx, svc, "shared")
	setLinks(t, store, gid, "web")
	if err := svc.Client.Delete(ctx, getApp(t, svc.Client, "web")); err != nil {
		t.Fatal(err)
	}
	if err := svc.Client.Create(ctx, apiApp("web", id.New(id.Service))); err != nil {
		t.Fatal(err)
	}

	if err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"A": "2"}, nil); err != nil {
		t.Fatalf("Blueprint sync: %v", err)
	}
	if got := getApp(t, svc.Client, "web").Spec.RestartedAt; got != "" {
		t.Errorf("the sync restarted the unrelated new service (restartedAt %q)", got)
	}
	if row, _ := groupsOf(store).Row(gid); len(row.ServiceLinks) != 0 {
		t.Errorf("links after the sync = %v, want none", row.ServiceLinks)
	}
}

// Linking stores the service id whichever identifier the caller passes, so a
// service linked by name and then by id is linked once.
func TestEnvGroup_LinkingStoresTheServiceID(t *testing.T) {
	webID := id.New(id.Service)
	svc := newService(newFakeStore(), apiApp("web", webID))
	ctx := testCtx()
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"web", webID} {
		if err := svc.LinkService(ctx, g.ID, ref); err != nil {
			t.Fatalf("LinkService(%s): %v", ref, err)
		}
	}
	if got, _ := svc.GetEnvGroup(ctx, g.ID); !slices.Equal(got.ServiceLinks, []string{webID}) {
		t.Fatalf("links = %v, want web's id once", got.ServiceLinks)
	}
	// Unlinking by name removes the id entry.
	if err := svc.UnlinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.GetEnvGroup(ctx, g.ID); len(got.ServiceLinks) != 0 {
		t.Fatalf("links after unlinking by name = %v, want none", got.ServiceLinks)
	}
}

// A service being deleted leaves every group it mounts, whether the group
// stored its id or, before w5/m120, its name. A group already deleted is
// skipped.
func TestWorkspacePurger_UnlinkAppLeavesNoGroupListingIt(t *testing.T) {
	store := newFakeStore()
	webID := id.New(id.Service)
	svc := newService(store, apiApp("web", webID), apiApp("api", id.New(id.Service)))
	ctx := testCtx()
	var gids []string
	for _, name := range []string{"by-id", "deleted"} {
		g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		for _, service := range []string{"web", "api"} {
			if err := svc.LinkService(ctx, g.ID, service); err != nil {
				t.Fatal(err)
			}
		}
		gids = append(gids, g.ID)
	}
	apiID := core.AppPublicID(getApp(t, svc.Client, "api"))
	web := getApp(t, svc.Client, "web")
	if err := svc.DeleteEnvGroup(ctx, gids[1]); err != nil {
		t.Fatal(err)
	}

	if err := (&WorkspacePurger{Service: svc}).UnlinkApp(ctx, web); err != nil {
		t.Fatalf("UnlinkApp: %v", err)
	}
	if err := svc.Client.Delete(ctx, web); err != nil { // the rest of the service delete
		t.Fatal(err)
	}
	for _, gid := range gids[:1] {
		got, err := svc.GetEnvGroup(ctx, gid)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.ServiceLinks, []string{apiID}) {
			t.Errorf("group %s lists %v, want only api %s", got.Name, got.ServiceLinks, apiID)
		}
	}
}

// A link survives a rename. It is the service's id: a link stored as the
// display name the caller used went stale once the service was renamed again,
// and the next group patch pruned it as a deleted service.
func TestEnvGroup_ALinkSurvivesARename(t *testing.T) {
	ctx := testCtx()
	webID := id.New(id.Service)
	web := apiApp("web", webID)
	web.Namespace, web.Labels[core.LabelTenant] = "tea-a", "tea-a"
	web.Spec.DisplayName = "api"
	svc := withGroups(&Service{
		Base:  &core.Base{Client: fakeClient(web), Namespace: "default", Workspace: coretest.Members{"dana": {"tea-a"}}},
		Store: newFakeStore(),
	})
	ctx = core.WithIdentity(ctx, core.Identity{Subject: "dana", Method: "session"})
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LinkService(ctx, g.ID, "api"); err != nil {
		t.Fatalf("LinkService by display name: %v", err)
	}
	var renamed appv1alpha1.App
	if err := svc.Client.Get(ctx, client.ObjectKeyFromObject(web), &renamed); err != nil {
		t.Fatal(err)
	}
	renamed.Spec.DisplayName = "backend"
	if err := svc.Client.Update(ctx, &renamed); err != nil {
		t.Fatal(err)
	}

	result, err := svc.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
		ExpectedRevision: &g.Revision, SaveMode: SaveModeDeploy,
		EnvVars: []EnvVarPatch{{Key: "A", Value: "v"}},
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if !slices.Equal(result.AffectedServiceIDs, []string{webID}) {
		t.Fatalf("patch reached %v, want the renamed service %s", result.AffectedServiceIDs, webID)
	}
	if got, _ := svc.GetEnvGroup(ctx, g.ID); !slices.Equal(got.ServiceLinks, []string{webID}) {
		t.Fatalf("links after the rename = %v, want %s", got.ServiceLinks, webID)
	}
}

// A patch prunes, and does not restart, a listed service that no longer mounts
// the group: whatever the link named, the group's values do not reach it.
func TestEnvGroup_PatchPrunesAListedServiceThatDoesNotMountTheGroup(t *testing.T) {
	svc := newService(newFakeStore(), apiApp("web", id.New(id.Service)))
	ctx := testCtx()
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	unmount(t, svc, "web")
	current, err := svc.GetEnvGroup(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}

	result, err := svc.PatchEnvironment(ctx, g.ID, EnvironmentPatch{
		ExpectedRevision: &current.Revision, SaveMode: SaveModeDeploy,
		EnvVars: []EnvVarPatch{{Key: "A", Value: "v"}},
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if len(result.AffectedServiceIDs) != 0 || getApp(t, svc.Client, "web").Spec.RestartedAt != "" {
		t.Fatalf("patch acted on a service that does not mount the group: %+v", result)
	}
	if got, _ := svc.GetEnvGroup(ctx, g.ID); len(got.ServiceLinks) != 0 {
		t.Fatalf("links after the patch = %v, want the stale link pruned", got.ServiceLinks)
	}
}

// Deleting a group detaches only the services that mount it: one it lists
// without mounting is not restarted.
func TestEnvGroup_DeleteDetachesOnlyServicesThatMountTheGroup(t *testing.T) {
	svc := newService(newFakeStore(), apiApp("web", id.New(id.Service)))
	ctx := testCtx()
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	unmount(t, svc, "web")

	if err := svc.DeleteEnvGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, svc.Client, "web").Spec.RestartedAt; got != "" {
		t.Fatalf("deleting the group restarted a service that does not mount it (restartedAt %q)", got)
	}
}

// unmount removes every group's refs from service, as if behind the group's
// back, and clears its last restart.
func unmount(t *testing.T, svc *Service, service string) {
	t.Helper()
	a := getApp(t, svc.Client, service)
	a.Spec.EnvFromSecrets, a.Spec.FilesFromSecrets, a.Spec.RestartedAt = nil, nil, ""
	if err := svc.Client.Update(testCtx(), a); err != nil {
		t.Fatal(err)
	}
}

// UnlinkApp leaves another workspace's group alone, even one the service's
// spec names: links never cross workspaces.
func TestWorkspacePurger_UnlinkAppSkipsAnotherWorkspacesGroup(t *testing.T) {
	store := newFakeStore()
	bWeb := apiApp("web", id.New(id.Service))
	bWeb.Namespace, bWeb.Labels[core.LabelTenant] = "tea-b", "tea-b"
	svc := withGroups(&Service{
		Base:  &core.Base{Client: fakeClient(bWeb), Namespace: "default", Workspace: coretest.Members{"erin": {"tea-b"}}},
		Store: store,
	})
	ctx := core.WithIdentity(testCtx(), core.Identity{Subject: "erin", Method: "session"})
	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LinkService(ctx, g.ID, "web"); err != nil {
		t.Fatal(err)
	}
	setLinks(t, store, g.ID, "web")
	aWeb := apiApp("web", id.New(id.Service))
	aWeb.Labels[core.LabelTenant] = "tea-a"
	aWeb.Spec.EnvFromSecrets = []string{envSecretName(g.ID)}

	if err := (&WorkspacePurger{Service: svc}).UnlinkApp(ctx, aWeb); err != nil {
		t.Fatalf("UnlinkApp: %v", err)
	}
	if row, _ := groupsOf(store).Row(g.ID); !slices.Equal(row.ServiceLinks, []string{"web"}) {
		t.Fatalf("tea-b's group links = %v, want its own link kept", row.ServiceLinks)
	}
}
