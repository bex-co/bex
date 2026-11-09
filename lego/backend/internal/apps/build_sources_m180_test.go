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
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// refusingEnvGroups refuses to link one group, as the env-group service
// refuses a group scoped to an environment on a build source.
type refusingEnvGroups struct {
	*fakeEnvGroups
	refuse string
}

func (r refusingEnvGroups) LinkEnvGroup(ctx context.Context, name, service string) error {
	if name == r.refuse {
		return fmt.Errorf("%w: env group %q is scoped to an environment", core.ErrBadRequest, name)
	}
	return r.fakeEnvGroups.LinkEnvGroup(ctx, name, service)
}

// w9/m180 t001: Render's create accepts serviceIds and envGroupIds. Each
// service is linked as a PATCH buildSourceId links it (no deploy), and a
// refused link writes nothing.
func TestBuildSourceCreateLinksServices(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	// The test store gives every service the same id, so the links below name
	// services the way AuthorizeApp also resolves them: by name.
	if _, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "web", Type: appv1alpha1.TypeWebService,
		Repo: "https://github.com/acme/web", Runtime: "node", BuildCommand: "npm ci", StartCommand: "node server.js"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "worker", Type: appv1alpha1.TypeBackgroundWorker, Image: "nginx:1"}); err != nil {
		t.Fatal(err)
	}
	web, worker := struct{ ID string }{"web"}, struct{ ID string }{"worker"}

	// A native source needs every linked service's start command: the worker
	// has none, so nothing is created and nothing is linked.
	before := len(st.rows)
	req := monorepoSource
	req.ServiceIDs = []string{web.ID, worker.ID}
	if _, err := svc.CreateBuildSource(ctx, req); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("native source linking a service without a start command: err = %v, want bad request", err)
	}
	if len(st.rows) != before || len(st.linked) != 0 {
		t.Fatalf("a refused link wrote %d carrier(s) and links %v", len(st.rows)-before, st.linked)
	}
	req.ServiceIDs = []string{web.ID, web.ID}
	if _, err := svc.CreateBuildSource(ctx, req); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a service listed twice: err = %v, want bad request", err)
	}

	req.ServiceIDs = []string{web.ID}
	created, err := svc.CreateBuildSource(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.linked) != 1 || !strings.HasSuffix(st.linked[0], "→"+created.ID) {
		t.Fatalf("store links = %v, want web linked to %s", st.linked, created.ID)
	}
	got := getTenantApp(t, svc.Client, "tea-a", "web")
	if got.Spec.BuildSource == nil || got.Spec.Repo != "" || got.Spec.StartCommand != "node server.js" {
		t.Fatalf("linked web spec = %+v, want the link and its own start command kept", got.Spec)
	}
	if got.Annotations[appv1alpha1.AnnotationPendingSourceGeneration] == "" {
		t.Fatalf("linked web annotations = %v, want the link saved without deploying", got.Annotations)
	}

	// An image source takes no start command: the worker links and keeps none.
	image, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{OwnerID: "tea-a", Name: "Img",
		Image: &BuildSourceImage{ImageURL: "nginx:1"}, ServiceIDs: []string{worker.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if w := getTenantApp(t, svc.Client, "tea-a", "worker"); core.BuildSourceIDOf(w) != image.ID || w.Spec.Image != "" {
		t.Fatalf("worker spec = %+v, want it linked to %s", w.Spec, image.ID)
	}
}

func TestBuildSourceCreateLinksEnvGroups(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	groups := newFakeEnvGroups("shared", "staging-only")
	svc.EnvGroups = refusingEnvGroups{fakeEnvGroups: groups, refuse: "staging-only"}

	req := monorepoSource
	req.EnvGroupIDs = []string{"evg-missing"}
	if _, err := svc.CreateBuildSource(ctx, req); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("an env group outside the workspace: err = %v, want bad request", err)
	}
	if len(st.rows) != 0 {
		t.Fatalf("an unknown env group still created a carrier: %+v", st.rows)
	}

	// The env-group service refuses a group scoped to an environment; the
	// create then deletes the carrier it wrote.
	req.EnvGroupIDs = []string{"evg-staging-only"}
	if _, err := svc.CreateBuildSource(ctx, req); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a refused env group link: err = %v, want bad request", err)
	}
	if left, err := svc.ListBuildSources(ctx, "tea-a", BuildSourceFilter{}); err != nil || len(left) != 0 {
		t.Fatalf("after a refused group link: build sources = %+v (err %v), want none", left, err)
	}

	req.EnvGroupIDs = []string{"evg-shared", "evg-shared"}
	created, err := svc.CreateBuildSource(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	carrierLink := "shared->" + created.ID
	if !slices.Contains(groups.links, carrierLink) || len(groups.links) != 1 {
		t.Fatalf("group links = %v, want exactly %s", groups.links, carrierLink)
	}
}

// w9/m180 t004: a build source's name is unique in the workspace, as Render's
// Blueprint schema says and a Blueprint sync relies on; create and rename
// refuse a taken name by code.
func TestBuildSourceNamesAreUnique(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	first, err := svc.CreateBuildSource(ctx, monorepoSource)
	if err != nil {
		t.Fatal(err)
	}
	var coded *core.CodedError
	if _, err := svc.CreateBuildSource(ctx, monorepoSource); !errors.As(err, &coded) || coded.Code != BuildSourceNameTakenCode {
		t.Fatalf("a second %q: err = %v, want %s", monorepoSource.Name, err, BuildSourceNameTakenCode)
	}
	other := monorepoSource
	other.Name = "Other"
	second, err := svc.CreateBuildSource(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	taken := monorepoSource.Name
	if _, err := svc.UpdateBuildSource(ctx, second.ID, UpdateBuildSourceRequest{Name: &taken}); !errors.As(err, &coded) || coded.Code != BuildSourceNameTakenCode {
		t.Fatalf("renaming onto %q: err = %v, want %s", taken, err, BuildSourceNameTakenCode)
	}
	same := monorepoSource.Name
	if _, err := svc.UpdateBuildSource(ctx, first.ID, UpdateBuildSourceRequest{Name: &same}); err != nil {
		t.Fatalf("keeping its own name: %v", err)
	}
	if err := svc.DeleteBuildSource(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateBuildSource(ctx, monorepoSource); err != nil {
		t.Fatalf("reusing a deleted build source's name: %v", err)
	}
}

// w9/m180 t003: a service linked to a Docker or image build source serves on
// its image's port with the render-v1 policy, as one built from that image
// itself does; a service linked to a native source keeps the native rules.
func TestServiceLinkedToImageSourceGetsImageRules(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	svc.ImageCompatibilityWorkspaces = map[string]bool{"*": true}
	image, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{OwnerID: "tea-a", Name: "Img", Image: &BuildSourceImage{ImageURL: "nginx:1"}})
	if err != nil {
		t.Fatal(err)
	}
	native, err := svc.CreateBuildSource(ctx, monorepoSource)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		req        CreateRequest
		portMode   string
		policy     string
		migratable bool
	}{
		"web on an image source": {
			req:      CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, BuildSourceID: image.ID},
			portMode: appv1alpha1.PortModeWebImageV1, policy: appv1alpha1.ContainerPolicyRenderV1, migratable: true,
		},
		"web on an image source, explicit port": {
			req:      CreateRequest{Name: "web-port", Type: appv1alpha1.TypeWebService, BuildSourceID: image.ID, Port: 8080},
			portMode: appv1alpha1.PortModeWebImageConfiguredV1, policy: appv1alpha1.ContainerPolicyRenderV1, migratable: true,
		},
		"private on an image source": {
			req:      CreateRequest{Name: "pserv", Type: appv1alpha1.TypePrivateService, BuildSourceID: image.ID},
			portMode: appv1alpha1.PortModeImageV1, policy: appv1alpha1.ContainerPolicyRenderV1, migratable: true,
		},
		"web on a native source": {
			req: CreateRequest{Name: "native-web", Type: appv1alpha1.TypeWebService, BuildSourceID: native.ID, StartCommand: "node server.js"},
		},
	} {
		tc.req.OwnerID = "tea-a"
		if _, err := svc.Create(ctx, tc.req); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := getTenantApp(t, svc.Client, "tea-a", tc.req.Name)
		if got.Spec.PortMode != tc.portMode || got.Spec.ContainerPolicy != tc.policy {
			t.Errorf("%s: portMode %q policy %q, want %q %q", name, got.Spec.PortMode, got.Spec.ContainerPolicy, tc.portMode, tc.policy)
		}
		carrier, err := svc.linkedCarrier(ctx, got)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := containerPolicyMigratable(got, carrier); (err == nil) != tc.migratable {
			t.Errorf("%s: migratable = %v (err %v), want %v", name, err == nil, err, tc.migratable)
		}
	}
}

// projectBuildSource declares mono under project p, with no environments
// (Render's 2026-10 schema), and a root service linking it.
const projectBuildSource = `projects:
  - name: p
    buildSources:
      - name: mono
        image: {url: nginx:1}
services:
  - type: worker
    name: worker
    buildSource: mono
`

func projectBuildSourceService(t *testing.T) (*Service, *buildSourceStore, context.Context) {
	t.Helper()
	svc, st, ctx := newBuildSourceService(t)
	svc.BlueprintGroups = &blueprintGroupingTestStore{recordingStore: st.recordingStore,
		projects: []store.Project{{ID: "prj-a", TenantID: "tea-a", Name: "p"}}}
	ctx = core.WithWorkspace(ctx, "tea-a")
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: projectBuildSource}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	return svc, st, ctx
}

// w9/m180 t004: a build source's project cannot change, so a root declaration
// of a project-scoped one is refused as a projects[] entry naming an unscoped
// one already is (ADR097 Blueprints §2).
func TestBlueprintRootDeclarationOfScopedBuildSourceIsRefused(t *testing.T) {
	svc, _, ctx := projectBuildSourceService(t)
	const root = `buildSources:
  - name: mono
    image: {url: nginx:1}
`
	_, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: root})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "scoped to a project") {
		t.Fatalf("root declaration of a scoped build source: err = %v, want a named bad request", err)
	}
}

// w9/m180 t004: Generate Blueprint writes a project-scoped build source under
// its project's buildSources, the only place a sync accepts it, and the
// export re-plans as a no-op.
func TestGenerateBlueprintExportsScopedBuildSourceUnderItsProject(t *testing.T) {
	svc, _, ctx := projectBuildSourceService(t)
	out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{OwnerID: "tea-a", ServiceIDs: []string{"worker"}})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	var doc struct {
		BuildSources []map[string]any `json:"buildSources"`
		Projects     []struct {
			Name         string           `json:"name"`
			BuildSources []map[string]any `json:"buildSources"`
		} `json:"projects"`
	}
	if err := yaml.Unmarshal([]byte(out.Manifest), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.BuildSources) != 0 || len(doc.Projects) != 1 || doc.Projects[0].Name != "p" ||
		len(doc.Projects[0].BuildSources) != 1 || doc.Projects[0].BuildSources[0]["name"] != "mono" {
		t.Fatalf("export placed mono wrong:\n%s", out.Manifest)
	}
	v, err := svc.ValidateBlueprint(ctx, "tea-a", out.Manifest, "")
	if err != nil || !v.Valid || v.Plan == nil {
		t.Fatalf("validate export = %+v, %v\n%s", v, err, out.Manifest)
	}
	for _, a := range v.Plan.Actions {
		if a.Operation != BlueprintPlanNoop {
			t.Errorf("export re-plans %s %s as %s %v", a.Kind, a.Name, a.Operation, a.ChangedFields)
		}
	}
}

// w9/m180 t004: a build source's export keeps every env form a service's
// export writes: a selected linked env group as fromGroup, an unselected
// one's keys as sync: false, and a stored value as sync: false.
func TestBuildSourceExportKeepsEnvGroupAndStoredForms(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	src := monorepoSource
	src.EnvVars = []envVarInput{{Key: "NODE_ENV", Value: "production"}}
	created, err := svc.CreateBuildSource(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	carrier := getTenantApp(t, svc.Client, "tea-a", created.ID)
	carrier.Spec.EnvFromSecrets = []string{"evg-shared-env", "evg-other-env"}
	svc.EnvGroupExport = fakeEnvGroupExport{groups: map[string]struct {
		name string
		keys []string
	}{
		"evg-shared": {name: "shared-config", keys: []string{"API_TOKEN"}},
		"evg-other":  {name: "other", keys: []string{"OTHER_KEY"}},
	}}
	selected := map[string]exportedEnvGroup{"evg-shared": {name: "shared-config"}}
	entry, err := svc.generateBuildSourceEntry(ctx, carrier, nil, nil, selected, map[string]exportedEnvGroup{})
	if err != nil {
		t.Fatal(err)
	}
	env, _ := entry["git"].(map[string]any)["envVars"].([]map[string]any)
	has := func(want map[string]any) bool {
		return slices.ContainsFunc(env, func(e map[string]any) bool { return maps.Equal(e, want) })
	}
	for _, want := range []map[string]any{
		{"key": "NODE_ENV", "value": "production"},
		{"fromGroup": "shared-config"},
		{"key": "OTHER_KEY", "sync": false},
	} {
		if !has(want) {
			t.Errorf("build source envVars = %v, want %v", env, want)
		}
	}
}

// w9/m180 t001 + t004 on GraphQL: createBuildSource takes serviceIds, and a
// taken name is refused by code, as on REST.
func TestBuildSourceGraphQLCreateLinksAndRefusesATakenName(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	if _, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "worker", Type: appv1alpha1.TypeBackgroundWorker, Image: "nginx:1"}); err != nil {
		t.Fatal(err)
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	const create = `mutation { createBuildSource(ownerId:"tea-a", name:"Img", image:{imageUrl:"nginx:1"}, serviceIds:["worker"]) { id } }`
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: create})
	if res.HasErrors() {
		t.Fatalf("create with serviceIds: %v", res.Errors)
	}
	if len(st.linked) != 1 {
		t.Fatalf("store links = %v, want the worker linked", st.linked)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: create})
	if !res.HasErrors() || !strings.Contains(res.Errors[0].Message, "already exists") {
		t.Fatalf("a second Img = %+v, want the taken-name refusal", res.Errors)
	}
}

// leasingStore records whether the workspace create lease is held, as
// store.PGStore.WithWorkspaceCreateLease holds one row per workspace.
type leasingStore struct {
	*buildSourceStore
	held   map[string]bool
	leased []string
}

func (l *leasingStore) WithWorkspaceCreateLease(_ context.Context, tenantID string, fn func() error) error {
	l.leased = append(l.leased, tenantID)
	l.held[tenantID] = true
	defer delete(l.held, tenantID)
	return fn()
}

// w9/m180 t005: a build source widens the namespace's App quota and then
// writes its App; a service create in between could take that slot (ADR097
// quota residual). Both creates hold the workspace create lease, and the
// carrier's quota widening happens inside it.
func TestCreatesHoldTheWorkspaceLeaseThroughTheQuotaWidening(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	lease := &leasingStore{buildSourceStore: st, held: map[string]bool{}}
	svc.Store = lease
	var widenedUnderLease []bool
	svc.EnsureNamespaces = func(_ context.Context, tenantID string) error {
		widenedUnderLease = append(widenedUnderLease, lease.held[tenantID])
		return nil
	}

	if _, err := svc.CreateBuildSource(ctx, monorepoSource); err != nil {
		t.Fatal(err)
	}
	if len(widenedUnderLease) == 0 || !widenedUnderLease[0] {
		t.Fatalf("the carrier's quota widening ran outside the workspace lease (calls: %v)", widenedUnderLease)
	}
	if _, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: "web", Type: appv1alpha1.TypeWebService,
		Repo: "https://github.com/acme/web", Runtime: "node", BuildCommand: "npm ci", StartCommand: "node server.js"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lease.leased, []string{"tea-a", "tea-a"}) {
		t.Fatalf("workspace lease taken for %v, want once per create in tea-a", lease.leased)
	}
}

// w9/m180: a create whose first service link fails transiently deletes its
// build source again, so a retry is not refused as a taken name; one whose
// later link fails keeps the source (a link rewrote a service's own source,
// which no undo restores) and says which services linked.
func TestBuildSourceCreateUndoesOrReportsAFailedServiceLink(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	for _, name := range []string{"web", "api"} {
		if _, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: name, Type: appv1alpha1.TypeWebService,
			Repo: "https://github.com/acme/" + name, Runtime: "node", BuildCommand: "npm ci", StartCommand: "node server.js"}); err != nil {
			t.Fatal(err)
		}
	}
	req := monorepoSource
	req.ServiceIDs = []string{"web", "api"}

	st.failLinkCall = 1
	if _, err := svc.CreateBuildSource(ctx, req); err == nil {
		t.Fatal("a failed first link: want an error")
	}
	for id, row := range st.rows {
		if row.Type == store.TypeBuildSource {
			t.Fatalf("the failed create left build source %s behind", id)
		}
	}

	st.failLinkCall = st.linkCalls + 2
	_, err := svc.CreateBuildSource(ctx, req)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != BuildSourcePartiallyLinkedCode || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a failed second link on retry: err = %v, want %s (not a taken name)", err, BuildSourcePartiallyLinkedCode)
	}
	if got := coded.Params["linkedServiceIds"]; !slices.Equal(got.([]string), []string{"web"}) || coded.Params["failedServiceId"] != "api" {
		t.Fatalf("params = %v, want web linked and api failed", coded.Params)
	}
}

// w9/m180: an export reads each scope project once, however many of its build
// sources it writes.
func TestBuildSourceExportReadsEachProjectOnce(t *testing.T) {
	svc, st, ctx := projectBuildSourceService(t)
	var list appv1alpha1.AppList
	if err := svc.ListByTenant(ctx, &list, "tea-a"); err != nil {
		t.Fatal(err)
	}
	var carrier *appv1alpha1.App
	for i := range list.Items {
		if core.IsBuildSource(&list.Items[i]) {
			carrier = &list.Items[i]
		}
	}
	if carrier == nil {
		t.Fatal("no build source in the fixture")
	}
	names := map[string]string{}
	before := st.projectReads
	for range 3 {
		if name, err := svc.buildSourceProjectName(ctx, carrier, names); err != nil || name != "p" {
			t.Fatalf("project name = %q, %v; want p", name, err)
		}
	}
	if reads := st.projectReads - before; reads != 1 {
		t.Fatalf("three lookups read the project %d times, want 1", reads)
	}
}

// w9/m180 production check: a build source's env-group link is staged until
// its next build (Render gives the group "at build time"), so Spec carries no
// ref yet. Export reads the links the owner asked for, staged ones included,
// and keeps the fromGroup.
func TestBuildSourceExportKeepsAStagedGroupLink(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	created, err := svc.CreateBuildSource(ctx, monorepoSource)
	if err != nil {
		t.Fatal(err)
	}
	carrier := getTenantApp(t, svc.Client, "tea-a", created.ID)
	carrier.Spec.EnvFromSecrets = nil
	rollout.StageGroupLink(carrier, "evg-shared-env", "evg-shared-files", true)
	svc.EnvGroupExport = fakeEnvGroupExport{groups: map[string]struct {
		name string
		keys []string
	}{"evg-shared": {name: "shared-config", keys: []string{"API_TOKEN"}}}}
	selected := map[string]exportedEnvGroup{"evg-shared": {name: "shared-config"}}
	entry, err := svc.generateBuildSourceEntry(ctx, carrier, nil, nil, selected, map[string]exportedEnvGroup{})
	if err != nil {
		t.Fatal(err)
	}
	git, _ := entry["git"].(map[string]any)
	env, _ := git["envVars"].([]map[string]any)
	if !slices.ContainsFunc(env, func(e map[string]any) bool { return maps.Equal(e, map[string]any{"fromGroup": "shared-config"}) }) {
		t.Fatalf("build source envVars = %v, want the staged group's fromGroup", env)
	}
}
