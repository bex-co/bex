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
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// buildSourceBlueprint is Render's Build Sources Blueprint shape: one git
// source feeding two web services (w3/m270).
const buildSourceBlueprint = `
buildSources:
  - name: mono
    git:
      repo: https://github.com/acme/mono
      branch: main
      runtime: node
      buildCommand: npm ci && npm run build
      envVars:
        - key: NODE_ENV
          value: production
services:
  - type: web
    name: api
    buildSource: mono
    startCommand: node api.js
  - type: web
    name: site
    buildSource: mono
    startCommand: node site.js
`

// TestBlueprintBuildSourcesCompile: every place Render's schema admits a build
// source compiles into a build_source declaration, scoped to its project, and
// a linked service compiles with no source of its own (ADR097 § Blueprints).
func TestBlueprintBuildSourcesCompile(t *testing.T) {
	const entry = "  - name: shared\n    git:\n      repo: https://github.com/bex-co/bex\n      runtime: docker\n"
	for _, tc := range []struct{ name, manifest, project string }{
		{"root", "buildSources:\n" + entry, ""},
		{"ungrouped", "ungrouped:\n  buildSources:\n  " + strings.ReplaceAll(entry, "\n  ", "\n    "), ""},
		{"project", "projects:\n  - name: p\n    buildSources:\n    " + strings.ReplaceAll(entry, "\n  ", "\n      ") + "    environments:\n      - name: prod\n", "p"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := parseBlueprintStackForTest(t, tc.manifest)
			if len(st.buildSources) != 1 {
				t.Fatalf("build sources = %+v, want one", st.buildSources)
			}
			b := st.buildSources[0]
			if b.name != "shared" || b.project != tc.project || b.native || b.svc.req.Repo != "https://github.com/bex-co/bex" ||
				b.svc.req.Type != appv1alpha1.TypeBackgroundWorker || b.svc.req.buildSource == nil {
				t.Fatalf("build source = %+v (req %+v)", b, b.svc.req)
			}
			if _, ok := b.svc.fields["repo"]; !ok {
				t.Fatalf("fields = %v, want git.repo lifted to repo", b.svc.fields)
			}
		})
	}

	st := parseBlueprintStackForTest(t, buildSourceBlueprint)
	if len(st.services) != 2 || len(st.buildSources) != 1 || !st.buildSources[0].native {
		t.Fatalf("stack = %+v", st)
	}
	for _, svc := range st.services {
		if svc.req.blueprintBuildSource != "mono" || svc.req.Repo != "" || svc.req.Runtime != "" {
			t.Fatalf("linked service req = %+v, want buildSource mono and no source of its own", svc.req)
		}
		if _, err := specFromCreate(svc.req); err != nil {
			t.Fatalf("linked service spec: %v", err)
		}
	}
	if env := st.buildSources[0].svc.req.Env; len(env) != 1 || env[0].Name != "NODE_ENV" || env[0].Value != "production" {
		t.Fatalf("build-time env = %+v", env)
	}
}

// The pinned schema refuses a source field beside buildSource, as Render's
// does, and Render's start-command rule holds against a declared source.
func TestBlueprintBuildSourceRefusals(t *testing.T) {
	_, problems := CompileBlueprintSource("services:\n  - type: web\n    name: web\n    buildSource: mono\n    repo: https://github.com/acme/web\n")
	if len(problems) == 0 {
		t.Fatal("repo beside buildSource compiled")
	}
	for name, manifest := range map[string]string{
		"native without start": strings.Replace(buildSourceBlueprint, "    startCommand: node api.js\n", "", 1),
		"docker with start":    strings.Replace(buildSourceBlueprint, "runtime: node", "runtime: docker", 1),
	} {
		t.Run(name, func(t *testing.T) {
			source, ir, problems := CompileBlueprintIR(manifest)
			if len(problems) > 0 {
				t.Fatalf("compile: %+v", problems)
			}
			if _, err := parseCompiledStack(blueprintParseOverrides{}, source, ir); !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("parse = %v, want a bad request", err)
			}
		})
	}
}

// One build source feeds two services: the apply creates one carrier and
// links both services to it, and an unchanged re-apply writes nothing.
func TestBlueprintBuildSourceApply(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	res, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: buildSourceBlueprint})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(res.BuildSources) != 1 || res.BuildSources[0].Name != "mono" || !strings.HasPrefix(res.BuildSources[0].ID, "bds-") {
		t.Fatalf("applied build sources = %+v", res.BuildSources)
	}
	id := res.BuildSources[0].ID
	carrier := getTenantApp(t, svc.Client, "tea-a", id)
	if !core.IsBuildSource(carrier) || carrier.Spec.DisplayName != "mono" || carrier.Spec.Repo != "https://github.com/acme/mono" ||
		carrier.Spec.Runtime != "node" || len(carrier.Spec.Env) != 1 {
		t.Fatalf("carrier spec = %+v", carrier.Spec)
	}
	for _, name := range []string{"api", "site"} {
		app := getTenantApp(t, svc.Client, "tea-a", name)
		if app.Spec.BuildSource == nil || app.Spec.BuildSource.Name != carrier.Name || app.Spec.Repo != "" || app.Spec.Image != "" {
			t.Fatalf("%s spec = %+v, want linked to %s with no source of its own", name, app.Spec, carrier.Name)
		}
	}
	deploys := len(st.deployCalls)
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: buildSourceBlueprint}); err != nil {
		t.Fatalf("re-apply: %v", err)
	}
	if len(st.deployCalls) != deploys {
		t.Fatalf("an unchanged re-apply opened %d deploys", len(st.deployCalls)-deploys)
	}
	var list appv1alpha1.AppList
	if err := svc.Client.List(ctx, &list); err != nil {
		t.Fatal(err)
	}
	carriers := 0
	for i := range list.Items {
		if core.IsBuildSource(&list.Items[i]) {
			carriers++
		}
	}
	if carriers != 1 {
		t.Fatalf("carriers = %d after a re-apply, want 1", carriers)
	}

	// A changed source rebuilds the carrier; the services keep their link.
	changed := strings.Replace(buildSourceBlueprint, "branch: main", "branch: release", 1)
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: changed}); err != nil {
		t.Fatalf("changed apply: %v", err)
	}
	if got := getTenantApp(t, svc.Client, "tea-a", id); got.Spec.Branch != "release" {
		t.Fatalf("carrier branch = %q, want release", got.Spec.Branch)
	}
	if len(st.deployCalls) != deploys+1 || st.deployCalls[len(st.deployCalls)-1].AppID != id {
		t.Fatalf("deploys after a source change = %+v, want one build of %s", st.deployCalls[deploys:], id)
	}
}

// A service may link a build source the workspace already has, by name; a
// name that is neither declared nor existing is refused before any write.
func TestBlueprintBuildSourceLinksAnExistingSource(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	source, err := svc.CreateBuildSource(ctx, monorepoSource) // named "Mono"
	if err != nil {
		t.Fatal(err)
	}
	manifest := "services:\n  - type: web\n    name: api\n    buildSource: Mono\n    startCommand: node api.js\n"
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if app := getTenantApp(t, svc.Client, "tea-a", "api"); app.Spec.BuildSource == nil || app.Spec.BuildSource.Name != core.CRName("tea-a", source.ID) {
		t.Fatalf("api spec = %+v, want linked to %s", app.Spec, source.ID)
	}

	missing := "services:\n  - type: web\n    name: web\n    buildSource: nope\n    startCommand: node web.js\n"
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: missing}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "names no build source") {
		t.Fatalf("unknown build source = %v, want a bad request naming it", err)
	}
	var list appv1alpha1.AppList
	if err := svc.Client.List(ctx, &list); err != nil {
		t.Fatal(err)
	}
	for i := range list.Items {
		if core.AppPublicName(&list.Items[i]) == "web" {
			t.Fatal("the refused apply created the service")
		}
	}
}

// An existing service a Blueprint now declares with buildSource is linked by
// the sync and rolls out; giving it its own source again unlinks it.
func TestBlueprintBuildSourceLinksAndUnlinksAnExistingService(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	own := "services:\n  - type: web\n    name: api\n    runtime: image\n    image: {url: nginx:1}\n"
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: own}); err != nil {
		t.Fatalf("own source: %v", err)
	}
	linked := strings.Replace(strings.Replace(buildSourceBlueprint, "runtime: node", "runtime: docker", 1), "    startCommand: node api.js\n", "", 1)
	linked = linked[:strings.Index(linked, "  - type: web\n    name: site")]
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: linked}); err != nil {
		t.Fatalf("link: %v", err)
	}
	app := getTenantApp(t, svc.Client, "tea-a", "api")
	if app.Spec.BuildSource == nil || app.Spec.Image != "" || app.Annotations[appv1alpha1.AnnotationReleaseGeneration] == "" {
		t.Fatalf("api after linking = %+v (annotations %v), want linked and rolled out", app.Spec, app.Annotations)
	}
	if len(st.linked) != 1 {
		t.Fatalf("store links = %v, want one", st.linked)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: own}); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if app := getTenantApp(t, svc.Client, "tea-a", "api"); app.Spec.BuildSource != nil || app.Spec.Image != "nginx:1" {
		t.Fatalf("api after its own source again = %+v, want unlinked", app.Spec)
	}
}

// The plan names the build source and both links before the apply, and
// nothing after it.
func TestBlueprintBuildSourcePlan(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	ctx = core.WithWorkspace(ctx, "tea-a")
	plan := func() map[string]BlueprintPlanOperation {
		t.Helper()
		v, err := svc.ValidateBlueprint(ctx, "tea-a", buildSourceBlueprint, "")
		if err != nil || !v.Valid || v.Plan == nil {
			t.Fatalf("validate = %+v, %v", v, err)
		}
		ops := map[string]BlueprintPlanOperation{}
		for _, a := range v.Plan.Actions {
			ops[string(a.Kind)+"/"+a.Name] = a.Operation
		}
		return ops
	}
	before := plan()
	if before["build_source/mono"] != BlueprintPlanCreate || before["service/api"] != BlueprintPlanCreate || before["service/site"] != BlueprintPlanCreate {
		t.Fatalf("plan before = %v", before)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: buildSourceBlueprint}); err != nil {
		t.Fatal(err)
	}
	for key, op := range plan() {
		if op != BlueprintPlanNoop {
			t.Errorf("after apply %s = %s, want noop", key, op)
		}
	}
	changed := strings.Replace(buildSourceBlueprint, "branch: main", "branch: release", 1)
	v, err := svc.ValidateBlueprint(ctx, "tea-a", changed, "")
	if err != nil || v.Plan == nil {
		t.Fatalf("validate changed = %+v, %v", v, err)
	}
	for _, a := range v.Plan.Actions {
		if a.Kind == BlueprintResourceBuildSource && (a.Operation != BlueprintPlanUpdate || len(a.ChangedFields) != 1 || a.ChangedFields[0].Path != "git.branch") {
			t.Fatalf("changed build source action = %+v, want an update of git.branch", a)
		}
	}
}

// Generate Blueprint exports a linked service as buildSource and its build
// source under buildSources, and the export re-applies as a no-op.
func TestGenerateBlueprintExportsBuildSources(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	ctx = core.WithWorkspace(ctx, "tea-a")
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: buildSourceBlueprint}); err != nil {
		t.Fatal(err)
	}
	out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{OwnerID: "tea-a", ServiceIDs: []string{"api", "site"}})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	for _, want := range []string{"buildSources:", "name: mono", "repo: https://github.com/acme/mono", "runtime: node", "buildSource: mono", "NODE_ENV"} {
		if !strings.Contains(out.Manifest, want) {
			t.Errorf("export lacks %q:\n%s", want, out.Manifest)
		}
	}
	if strings.Count(out.Manifest, "name: mono") != 1 {
		t.Errorf("the shared build source is exported more than once:\n%s", out.Manifest)
	}
	deploys := len(st.deployCalls)
	v, err := svc.ValidateBlueprint(ctx, "tea-a", out.Manifest, "")
	if err != nil || !v.Valid || v.Plan == nil {
		t.Fatalf("validate export = %+v, %v", v, err)
	}
	for _, a := range v.Plan.Actions {
		if a.Operation != BlueprintPlanNoop {
			t.Errorf("export re-plans %s %s as %s %v", a.Kind, a.Name, a.Operation, a.ChangedFields)
		}
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: out.Manifest}); err != nil {
		t.Fatalf("re-apply export: %v", err)
	}
	if len(st.deployCalls) != deploys {
		t.Fatalf("re-applying the export opened %d deploys", len(st.deployCalls)-deploys)
	}
}

// A Blueprint claims the build sources it declares: another Blueprint naming
// the same one is refused with the takeover phrase, and a file that stops
// declaring it releases the claim (the carrier keeps running).
func TestBlueprintBuildSourceOwnership(t *testing.T) {
	const manifest = `buildSources:
  - name: shared
    image: {url: nginx:1}
services:
  - name: web
    type: web
    buildSource: shared
`
	svc, fs := ownershipService(t)
	svc.GitFetcher = fakeBlueprintFetcher{contents: manifest, sha: "abc1234"}
	ctx := ownershipCtx()
	a, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	carrierOwner := func() string {
		t.Helper()
		var apps appv1alpha1.AppList
		if err := svc.Client.List(ctx, &apps); err != nil {
			t.Fatal(err)
		}
		for i := range apps.Items {
			if core.IsBuildSource(&apps.Items[i]) {
				return apps.Items[i].Labels[core.LabelBlueprint]
			}
		}
		t.Fatal("no carrier")
		return ""
	}
	if owner := carrierOwner(); owner != a.ID {
		t.Fatalf("carrier owner = %q, want %q", owner, a.ID)
	}
	if owner, _ := fs.GetBlueprintResourceOwner(ctx, "tea-a", "build_source", "shared"); owner != a.ID {
		t.Fatalf("claim owner = %q, want %q", owner, a.ID)
	}
	view, err := svc.GetBlueprintByID(ctx, a.ID, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	listed := false
	for _, r := range view.Resources {
		listed = listed || (r.Type == "build_source" && r.Name == "shared" && strings.HasPrefix(r.ID, "bds-"))
	}
	if !listed {
		t.Fatalf("resources = %+v, want the build source listed as build_source", view.Resources)
	}

	_, err = svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/b", Branch: "main"})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "BLUEPRINT_RESOURCE_CONFLICT" {
		t.Fatalf("B create = %v, want BLUEPRINT_RESOURCE_CONFLICT", err)
	}

	svc.GitFetcher = fakeBlueprintFetcher{contents: ownershipManifest, sha: "def5678"}
	if _, err := svc.SyncBlueprint(ctx, a.ID, "tea-a", "", "", nil); err != nil {
		t.Fatalf("sync without the build source: %v", err)
	}
	if owner, _ := fs.GetBlueprintResourceOwner(ctx, "tea-a", "build_source", "shared"); owner != "" {
		t.Fatalf("claim after the file stopped declaring it = %q, want released", owner)
	}
	if owner := carrierOwner(); owner != "" {
		t.Fatalf("carrier marker after release = %q, want none", owner)
	}
}

// A field the file stops declaring is planned too: an omitted buildFilter
// clears, as on a service, and the plan names it.
func TestBlueprintBuildSourcePlanNamesAnOmittedField(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	ctx = core.WithWorkspace(ctx, "tea-a")
	filtered := strings.Replace(buildSourceBlueprint, "      envVars:\n", "      buildFilter:\n        paths: [\"api/**\"]\n      envVars:\n", 1)
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: filtered}); err != nil {
		t.Fatal(err)
	}
	v, err := svc.ValidateBlueprint(ctx, "tea-a", buildSourceBlueprint, "")
	if err != nil || v.Plan == nil {
		t.Fatalf("validate = %+v, %v", v, err)
	}
	for _, a := range v.Plan.Actions {
		if a.Kind == BlueprintResourceBuildSource && (a.Operation != BlueprintPlanUpdate || len(a.ChangedFields) != 1 || a.ChangedFields[0].Path != "git.buildFilter") {
			t.Fatalf("build source action = %+v, want an update of git.buildFilter", a)
		}
	}
}

// A project that declares only build sources applies (Render's 2026-10
// schema): its build sources are scoped to it, and a missing project is
// created for them. A project with neither is still refused (w3/m275).
func TestBlueprintBuildSourcesOnlyProject(t *testing.T) {
	const manifest = `projects:
  - name: p
    buildSources:
      - name: mono
        image: {url: nginx:1}
`
	requireOneProblem(t, "projects:\n  - name: p\n", "BLUEPRINT_PROJECT_ENVIRONMENTS_REQUIRED", "#/projects/0", "at least one environment")

	svc, st, ctx := newBuildSourceService(t)
	groups := &blueprintGroupingTestStore{recordingStore: st.recordingStore, projects: []store.Project{{ID: "prj-a", TenantID: "tea-a", Name: "p"}}}
	svc.BlueprintGroups = groups
	res, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(res.BuildSources) != 1 || st.rows[res.BuildSources[0].ID].ScopeProjectID != "prj-a" {
		t.Fatalf("build sources = %+v, want mono scoped to prj-a", res.BuildSources)
	}

	assignments := map[string]core.EnvironmentAssignment{}
	if err := svc.applyBlueprintSourceProjects(core.WithWorkspace(ctx, "tea-a"), []string{"fresh"}, assignments); err != nil {
		t.Fatal(err)
	}
	created := assignments[blueprintGroupingKey("fresh", "")].ProjectID
	if created == "" || len(groups.projects) != 2 || groups.projects[1].Name != "fresh" || groups.projects[1].ID != created {
		t.Fatalf("projects = %+v, assignment %q; want fresh created and assigned", groups.projects, created)
	}
}

// A build source's envVars take a service's forms (w3/m275): generateValue is
// seeded once into the carrier's store, fromGroup links the group to the
// carrier, fromDatabase becomes a secret-ref build env var; only a reference
// to a service, which builds after it, is refused.
func TestBlueprintBuildSourceEnvForms(t *testing.T) {
	const manifest = `envVarGroups:
  - name: shared
    envVars:
      - key: FLAG
        value: "1"
databases:
  - name: db
    plan: free
buildSources:
  - name: mono
    git:
      repo: https://github.com/acme/mono
      runtime: docker
      envVars:
        - key: TOKEN
          generateValue: true
        - fromGroup: shared
        - key: DATABASE_URL
          fromDatabase: {name: db, property: connectionString}
`
	svc, _, ctx := newBuildSourceService(t)
	groups, seeder := newFakeEnvGroups(), &fakeSeeder{}
	svc.EnvGroups, svc.EnvSeeder = groups, seeder
	res, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	id := res.BuildSources[0].ID
	carrier := getTenantApp(t, svc.Client, "tea-a", id)
	ref := stackServiceRef(carrier)
	if !slices.Contains(groups.links, "shared->"+ref) {
		t.Errorf("group links = %v, want shared linked to %s", groups.links, ref)
	}
	if len(seeder.seeds) != 1 || seeder.seeds[0].service != ref || !slices.Equal(seeder.seeds[0].generates, []string{"TOKEN"}) {
		t.Errorf("seeds = %+v, want TOKEN generated once on %s", seeder.seeds, ref)
	}
	var dbEnv *appv1alpha1.EnvVar
	for i := range carrier.Spec.Env {
		if carrier.Spec.Env[i].Name == "DATABASE_URL" {
			dbEnv = &carrier.Spec.Env[i]
		}
	}
	if dbEnv == nil || dbEnv.ValueFrom == nil || dbEnv.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("carrier env = %+v, want DATABASE_URL from the database's Secret", carrier.Spec.Env)
	}

	refused := strings.Replace(manifest, "fromDatabase: {name: db, property: connectionString}", "fromService: {type: web, name: api, property: host}", 1)
	source, ir, problems := CompileBlueprintIR(refused)
	if len(problems) > 0 {
		t.Fatalf("compile: %+v", problems)
	}
	if _, err := parseCompiledStack(blueprintParseOverrides{}, source, ir); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "builds before the services") {
		t.Fatalf("fromService a service = %v, want a named refusal", err)
	}
}
