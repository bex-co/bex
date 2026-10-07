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

package apps

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// webApp is tenant's store-managed web service name, running image.
func webApp(tenant, name, image string) *appv1alpha1.App {
	app := tenantNSApp(name, tenant, "srv-"+tenant+"-"+name)
	app.Spec = appv1alpha1.AppSpec{Image: image, Type: appv1alpha1.TypeWebService, Runtime: "image"}
	return app
}

const webStack = `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:2}
`

// tenantStackService is a Service acting in tea-a over apps, and a caller
// there.
func tenantStackService(apps ...*appv1alpha1.App) (*Service, client.Client, context.Context) {
	svc, cl := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, &recordingStore{}, apps...)
	return svc, cl, ctxAs("id-a")
}

// renamedTwins is a workspace where the names swapped: web was created as
// "web" and is displayed as "frontend", and api was created as "api" and is
// displayed as "web".
func renamedTwins() (web, api *appv1alpha1.App) {
	web = webApp("tea-a", "web", "nginx:1")
	web.Spec.DisplayName, web.Spec.Subdomain = "frontend", "web-slug"
	api = webApp("tea-a", "api", "nginx:1")
	api.Spec.DisplayName, api.Spec.Subdomain = "web", "api-slug"
	return web, api
}

// stored reads app as stored now.
func stored(t *testing.T, cl client.Client, app *appv1alpha1.App) *appv1alpha1.App {
	t.Helper()
	var got appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), &got); err != nil {
		t.Fatal(err)
	}
	return &got
}

// TestAnOwnerlessDeployUpdatesOnlyItsOwnWorkspacesServices (w5/m133): MCP's
// deploy passes no ownerId, so a deploy by a member of two workspaces acts in
// the one its identity resolves to. The apply looked each declared service up
// in every workspace the caller belongs to, and patched the other one's
// same-named service while creating the rest in its own.
func TestAnOwnerlessDeployUpdatesOnlyItsOwnWorkspacesServices(t *testing.T) {
	theirs := webApp("tea-b", "web", "nginx:1")
	svc, cl := newTenantStoreService(coretest.Workspaces{"tea-a", "tea-b"}, &recordingStore{}, theirs)
	svc.MemberWorkspaceIDs = func(context.Context, core.Identity) ([]string, error) { return []string{"tea-a", "tea-b"}, nil }

	if _, err := svc.DeployStack(ctxAs("id-a"), DeployRequest{Manifest: webStack}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if got := stored(t, cl, theirs).Spec.Image; got != "nginx:1" {
		t.Errorf("tea-b's web runs %q, want it untouched at nginx:1", got)
	}
	if got := getTenantApp(t, cl, "tea-a", "web").Spec.Image; got != "nginx:2" {
		t.Errorf("tea-a's web runs %q, want nginx:2", got)
	}
}

// TestAnUnscopedDeployLeavesWorkspacesServicesAlone (w5/m133): with no
// workspace resolved, a deploy counts only the Apps no workspace owns, never
// a workspace's App left in the shared namespace under the same name.
func TestAnUnscopedDeployLeavesWorkspacesServicesAlone(t *testing.T) {
	theirs := webApp("tea-b", "web", "nginx:1")
	theirs.Namespace = "default"
	svc, cl := newService(nil, theirs)

	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: webStack}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if got := stored(t, cl, theirs).Spec.Image; got != "nginx:1" {
		t.Errorf("tea-b's web runs %q, want it untouched at nginx:1", got)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "nginx:2" {
		t.Errorf("the unscoped web runs %q, want nginx:2", got)
	}
}

// TestAManifestNameUpdatesTheServiceCreatedUnderIt (w5/m133): the plan keys a
// workspace's services by the name they were created under, while the apply
// matched the name a service is displayed as. So the plan said it would update
// tea-a-web, and the apply patched tea-a-api, renamed to "web" for display.
func TestAManifestNameUpdatesTheServiceCreatedUnderIt(t *testing.T) {
	web, api := renamedTwins()
	svc, cl, ctx := tenantStackService(web, api)

	actions := planActionsForTest(core.WithWorkspace(ctx, "tea-a"), t, svc, webStack)
	if len(actions) != 1 || actions[0].Operation != BlueprintPlanUpdate || actions[0].ResourceID != web.Name {
		t.Fatalf("plan = %+v, want an update of %s", actions, web.Name)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: webStack}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if got := stored(t, cl, web).Spec.Image; got != "nginx:2" {
		t.Errorf("%s, which the plan names, runs %q, want nginx:2", web.Name, got)
	}
	if got := stored(t, cl, api).Spec.Image; got != "nginx:1" {
		t.Errorf("%s, displayed as web, runs %q, want it untouched at nginx:1", api.Name, got)
	}
}

// TestASharedNamespaceServiceIsPlannedAsTheApplyUpdatesIt (w5/m133): a
// workspace's service still only in the shared namespace (mid ADR043 D8) was
// planned as a create. A deploy naming the workspace then created a second
// copy, or was refused as a name in use; an ownerless one updated it.
func TestASharedNamespaceServiceIsPlannedAsTheApplyUpdatesIt(t *testing.T) {
	shared := webApp("tea-a", "web", "nginx:1")
	shared.Namespace = "default"
	svc, cl, ctx := tenantStackService(shared)

	actions := planActionsForTest(core.WithWorkspace(ctx, "tea-a"), t, svc, webStack)
	if len(actions) != 1 || actions[0].Operation != BlueprintPlanUpdate || actions[0].ResourceID != shared.Name {
		t.Fatalf("plan = %+v, want an update of the shared-namespace %s", actions, shared.Name)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: webStack}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if got := stored(t, cl, shared).Spec.Image; got != "nginx:2" {
		t.Errorf("the shared-namespace web runs %q, want nginx:2", got)
	}
	var second appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-a", Name: shared.Name}, &second); !apierrors.IsNotFound(err) {
		t.Errorf("the apply created a second web in tea-a (%v)", err)
	}
}

// TestTheOwnCopyAnswersWhateverTheListOrder (w5/m133): a name's copies were
// compared in list order, so two stale copies listed before the workspace's
// own were refused as a duplicate. The own copy answers, and only copies in
// the answering namespace are compared.
func TestTheOwnCopyAnswersWhateverTheListOrder(t *testing.T) {
	legacy := sampleApp("web") // pre-w4/m19: bare-named, labelled only with its workspace
	legacy.Labels = map[string]string{core.LabelTenant: "tea-a"}
	staleTwin := webApp("tea-a", "web", "nginx:1")
	staleTwin.Namespace = "default"
	own := webApp("tea-a", "web", "nginx:2")
	svc, _, ctx := tenantStackService(legacy, staleTwin, own)

	actions := planActionsForTest(core.WithWorkspace(ctx, "tea-a"), t, svc, webStack)
	if len(actions) != 1 || actions[0].Operation != BlueprintPlanNoop || actions[0].ResourceID != own.Name {
		t.Fatalf("plan = %+v, want web unchanged against its own copy", actions)
	}
}

// TestAWorkspaceDuplicateIsRefusedForTheWorkspace (w5/m133): two copies of a
// name in one namespace are the workspace's conflict. Validation reported it
// against every declared service with domains, as if each had a problem.
func TestAWorkspaceDuplicateIsRefusedForTheWorkspace(t *testing.T) {
	legacy := sampleApp("web")
	legacy.Namespace, legacy.Labels = "tea-a", map[string]string{core.LabelTenant: "tea-a"}
	svc, _, ctx := tenantStackService(legacy, webApp("tea-a", "web", "nginx:1"))
	svc.BaseDomain = "onbex.co"

	_, err := svc.ValidateBlueprint(ctx, "tea-a", webStack+`  - name: site
    type: web
    runtime: image
    image: {url: nginx:1}
    domains:
      - shop.example.com
`, "")
	if !errors.Is(err, core.ErrConflict) || !strings.Contains(err.Error(), `service name "web"`) || strings.Contains(err.Error(), `service "site"`) {
		t.Fatalf("ValidateBlueprint error = %v, want the workspace's web conflict alone", err)
	}
}

// TestABlueprintApplyListsTheWorkspacesAppsOnce (w5/m133): the apply looked
// each declared service up for itself, two or three App lists apiece. One
// list now serves the plan and every declared service.
func TestABlueprintApplyListsTheWorkspacesAppsOnce(t *testing.T) {
	svc, _, ctx := tenantStackService(webApp("tea-a", "web", "nginx:1"), webApp("tea-a", "api", "nginx:1"))
	lists := &workspaceListCounter{Client: svc.Client}
	svc.Client = lists

	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: webStack + `  - name: api
    type: web
    runtime: image
    image: {url: nginx:2}
  - name: worker
    type: web
    runtime: image
    image: {url: nginx:2}
`}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if got := lists.apps.Load(); got != 1 {
		t.Errorf("a deploy of two existing services and a new one listed Apps %d times, want once", got)
	}
}

// TestAnApplyNeverPatchesTheSharedListing (w5/m133): the request's readers
// share one listing of the workspace, so the apply reads the App it updates
// afresh rather than patching the listed copy in place.
func TestAnApplyNeverPatchesTheSharedListing(t *testing.T) {
	svc, _, ctx := tenantStackService(webApp("tea-a", "web", "nginx:1"))
	appsByName, err := svc.newWorkspaceSnapshot(ctx).services(ctx)
	if err != nil {
		t.Fatal(err)
	}
	listed := appsByName["web"].DeepCopy()
	st := parseBlueprintStackForTest(t, webStack)
	if _, _, err := svc.applyStackService(ctx, st.services[0].req, st.services[0].fields, appsByName); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !reflect.DeepEqual(appsByName["web"], listed) {
		t.Errorf("the apply patched the shared listing: %+v", appsByName["web"].Spec)
	}
}

// resolvingEnvGroups and resolvingSeeder record the App each link and seed
// reaches, through the AuthorizeApp the real verbs resolve their service by.
type resolvingEnvGroups struct {
	*fakeEnvGroups
	svc     *Service
	reached []string
}

func (r *resolvingEnvGroups) LinkEnvGroup(ctx context.Context, _, service string) error {
	app, err := r.svc.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	r.reached = append(r.reached, app.Name)
	return nil
}

type resolvingSeeder struct {
	svc     *Service
	reached []string
}

func (r *resolvingSeeder) SeedEnvVars(ctx context.Context, service string, _ map[string]string, _ []string) error {
	app, err := r.svc.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	r.reached = append(r.reached, app.Name)
	return nil
}

// TestTheApplysLaterVerbsReachTheServiceItWrote (w5/m133): after writing a
// service, the apply links its env groups and seeds its generated values by
// name, and a name resolves the service displayed under it. Both now reach the
// App the apply wrote, whether it updated or created it. A service it creates
// has its groups linked as it is written, so only the update links by name
// (w5/143).
func TestTheApplysLaterVerbsReachTheServiceItWrote(t *testing.T) {
	web, api := renamedTwins()
	svc, _, ctx := tenantStackService(web, api)
	groups, seeder := &resolvingEnvGroups{fakeEnvGroups: newFakeEnvGroups("shared"), svc: svc}, &resolvingSeeder{svc: svc}
	svc.EnvGroups, svc.EnvSeeder = groups, seeder
	const env = `    envVars:
      - {fromGroup: shared}
      - {key: TOKEN, generateValue: true}
`
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: webStack + env + `  - name: worker
    type: web
    runtime: image
    image: {url: nginx:1}
` + env}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if want := []string{web.Name}; !reflect.DeepEqual(groups.reached, want) {
		t.Errorf("links by name reached %v, want %v", groups.reached, want)
	}
	if len(groups.links) != 1 {
		t.Errorf("groups linked as services were created = %v, want the worker's one", groups.links)
	}
	if want := []string{web.Name, core.CRName("tea-a", "worker")}; !reflect.DeepEqual(seeder.reached, want) {
		t.Errorf("seeds reached %v, want %v", seeder.reached, want)
	}
}

// TestAMaintenanceOnlyChangeReachesTheServiceThePlanNames (w5/m133): a re-apply
// that changes only maintenance mode hands it to ConfigureMaintenanceMode, which
// took the manifest name and so reached the service displayed under it.
func TestAMaintenanceOnlyChangeReachesTheServiceThePlanNames(t *testing.T) {
	web, api := renamedTwins()
	svc, cl, ctx := tenantStackService(web, api)
	paid := webStack + "    plan: starter\n"
	for _, manifest := range []string{paid, paid + "    maintenanceMode:\n      enabled: true\n"} {
		if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
			t.Fatalf("deploy: %v", err)
		}
	}
	for _, app := range []*appv1alpha1.App{web, api} {
		got := stored(t, cl, app)
		if on := got.Spec.MaintenanceMode != nil && got.Spec.MaintenanceMode.Enabled; on != (app == web) {
			t.Errorf("%s maintenance mode on = %v, want it on only for %s", app.Name, on, web.Name)
		}
	}
}

// TestAFromServiceReferenceResolvesTheServiceCreatedUnderTheName (w5/m133): a
// fromService reference to an existing service the manifest does not declare
// took the host of the service displayed under that name.
func TestAFromServiceReferenceResolvesTheServiceCreatedUnderTheName(t *testing.T) {
	web, api := renamedTwins()
	svc, cl, ctx := tenantStackService(web, api)
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: `services:
  - name: site
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - {key: WEB_HOST, fromService: {name: web, type: web, property: host}}
`}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	site := getTenantApp(t, cl, "tea-a", "site")
	for _, env := range site.Spec.Env {
		if env.Name == "WEB_HOST" {
			if !strings.Contains(env.Value, "web-slug") {
				t.Errorf("WEB_HOST = %q, want web's host (web-slug)", env.Value)
			}
			return
		}
	}
	t.Fatalf("site has no WEB_HOST: %+v", site.Spec.Env)
}

// TestAHostPreviewExemptsTheServiceCreatedUnderTheName (w5/m133): validation
// exempts the existing service from its own hosts' collision check. It took
// the service displayed under the manifest name, so web re-stating its own
// domain was refused as another site's.
func TestAHostPreviewExemptsTheServiceCreatedUnderTheName(t *testing.T) {
	web, api := renamedTwins()
	web.Spec.Hosts = []string{"shop.example.com"}
	svc, _, ctx := tenantStackService(web, api)
	svc.BaseDomain = "onbex.co"
	validation, err := svc.ValidateBlueprint(ctx, "tea-a", webStack+"    domains:\n      - shop.example.com\n", "")
	if err != nil || !validation.Valid {
		t.Fatalf("ValidateBlueprint = %+v, %v; want web's own domain to validate", validation, err)
	}
}

// TestAFromServiceReferenceToADisplayedNameIsUnknown (w5/m133): validation
// accepted a fromService reference that only a displayed name matched, which
// the apply then cannot resolve.
func TestAFromServiceReferenceToADisplayedNameIsUnknown(t *testing.T) {
	worker := webApp("tea-a", "worker", "nginx:1")
	worker.Spec.DisplayName = "api"
	svc, _, ctx := tenantStackService(worker)
	validation, err := svc.ValidateBlueprint(ctx, "tea-a", webStack+`    envVars:
      - {key: API_HOST, fromService: {name: api, type: web, property: host}}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if validation.Valid || len(validation.Errors) != 1 || !strings.Contains(validation.Errors[0].Error, `unknown service "api"`) {
		t.Fatalf("validation = %+v, want api refused as unknown", validation)
	}
}

// TestALabellessServicesLaterVerbsStayInItsWorkspace (w5/144): an App without
// a service-name label goes by its object name, and an ownerless deploy's
// by-name verbs resolved that through the names displayed in every workspace
// the caller belongs to. Another workspace's service displayed under the name
// then took the verbs' writes, or made them refuse as ambiguous. They now look
// in the App's own workspace: its env groups, seeds and maintenance mode reach
// it, whether maintenance changes alone or with the rest of the service.
func TestALabellessServicesLaterVerbsStayInItsWorkspace(t *testing.T) {
	for name, displayedAsFrontend := range map[string]bool{"sharing its displayed name": false, "displayed under another name": true} {
		t.Run(name, func(t *testing.T) {
			ours := webApp("tea-a", "web", "nginx:1")
			ours.Name = "web" // a bare name, from before w4/m19
			delete(ours.Labels, core.LabelServiceName)
			if displayedAsFrontend {
				ours.Spec.DisplayName = "frontend"
			}
			theirs := webApp("tea-b", "site", "nginx:1")
			theirs.Spec.DisplayName, theirs.Spec.Tier = "web", "starter"
			svc, cl := newTenantStoreService(coretest.Workspaces{"tea-a", "tea-b"}, &recordingStore{}, ours, theirs)
			svc.MemberWorkspaceIDs = func(context.Context, core.Identity) ([]string, error) { return []string{"tea-a", "tea-b"}, nil }
			groups, seeder := &resolvingEnvGroups{fakeEnvGroups: newFakeEnvGroups("shared"), svc: svc}, &resolvingSeeder{svc: svc}
			svc.EnvGroups, svc.EnvSeeder = groups, seeder

			manifest := `services:
  - name: web
    type: web
    runtime: image
    plan: starter
    envVars:
      - {fromGroup: shared}
      - {key: TOKEN, generateValue: true}
`
			for _, m := range []string{
				manifest + "    image: {url: nginx:2}\n",
				manifest + "    image: {url: nginx:2}\n    maintenanceMode:\n      enabled: true\n",
				manifest + "    image: {url: nginx:3}\n    maintenanceMode:\n      enabled: false\n",
			} {
				if _, err := svc.DeployStack(ctxAs("id-a"), DeployRequest{Manifest: m}); err != nil {
					t.Fatalf("deploy: %v", err)
				}
			}
			if len(groups.reached) == 0 || len(seeder.reached) == 0 {
				t.Fatalf("links reached %v and seeds %v, want tea-a's web", groups.reached, seeder.reached)
			}
			for _, reached := range append(append([]string{}, groups.reached...), seeder.reached...) {
				if reached != ours.Name {
					t.Errorf("a link or seed for web reached %q, want only tea-a's %q", reached, ours.Name)
				}
			}
			if got := stored(t, cl, theirs); got.Spec.MaintenanceMode != nil {
				t.Errorf("tea-b's %s got maintenance mode %+v, want it untouched", theirs.Name, got.Spec.MaintenanceMode)
			}
			if got := stored(t, cl, ours); got.Spec.MaintenanceMode == nil || got.Spec.MaintenanceMode.Enabled {
				t.Errorf("tea-a's web maintenance mode = %+v, want it set and then turned off with its image change", got.Spec.MaintenanceMode)
			}
		})
	}
}
