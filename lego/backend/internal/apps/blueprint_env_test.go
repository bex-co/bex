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

// blueprint_env_test.go covers the w1/m35 blueprint env forms: envVarGroups +
// fromGroup, sync:false + generateValue seeding, and fromService.envVarKey — the
// parse-time classification, the apply-time routing through the two seams, and the
// all-or-nothing pre-flight.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- fake seams ---------------------------------------------------------------

type appliedGroup struct {
	literals  map[string]string
	generates []string
}

type fakeEnvGroups struct {
	preexisting  []string                // names that already exist in the workspace
	applied      map[string]appliedGroup // name -> last ApplyEnvGroup args
	links        []string                // "group->service" in call order
	environments map[string]string       // name -> environmentID from SetGroupEnvironment
}

func newFakeEnvGroups(preexisting ...string) *fakeEnvGroups {
	return &fakeEnvGroups{preexisting: preexisting, applied: map[string]appliedGroup{}, environments: map[string]string{}}
}

func (f *fakeEnvGroups) GroupNames(ctx context.Context) ([]string, error) {
	groups, err := f.GroupIDsByName(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(groups))
	for name := range groups {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func (f *fakeEnvGroups) GroupIDsByName(context.Context) (map[string]string, error) {
	seen := map[string]bool{}
	out := map[string]string{}
	for _, n := range f.preexisting {
		if !seen[n] {
			seen[n] = true
			out[n] = "evg-" + n
		}
	}
	for n := range f.applied {
		if !seen[n] {
			seen[n] = true
			out[n] = "evg-" + n
		}
	}
	return out, nil
}

func (f *fakeEnvGroups) ApplyEnvGroup(_ context.Context, name string, literals map[string]string, generates []string) error {
	f.applied[name] = appliedGroup{literals: literals, generates: generates}
	return nil
}

func (f *fakeEnvGroups) LinkEnvGroup(_ context.Context, name, service string) error {
	f.links = append(f.links, name+"->"+service)
	return nil
}

func (f *fakeEnvGroups) SetGroupEnvironment(_ context.Context, name, environmentID string) error {
	f.environments[name] = environmentID
	return nil
}

type seedCall struct {
	service   string
	literals  map[string]string
	generates []string
}

type fakeSeeder struct{ seeds []seedCall }

func (f *fakeSeeder) SeedEnvVars(_ context.Context, service string, literals map[string]string, generates []string) error {
	f.seeds = append(f.seeds, seedCall{service: service, literals: literals, generates: generates})
	return nil
}

// newBlueprintEnvService builds an apps.Service wired with the two blueprint env
// seams (both nil unless supplied), over a fake client seeded with objs.
func newBlueprintEnvService(groups EnvGroupApplier, seeder EnvSeeder, objs ...*appv1alpha1.App) (*Service, client.Client) {
	co := make([]client.Object, len(objs))
	for i, a := range objs {
		co[i] = a
	}
	cl := fakeClient(co...)
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}, EnvGroups: groups, EnvSeeder: seeder}, cl
}

// fiveFieldManifest exercises all five w1/m35 forms in one file.
const fiveFieldManifest = `
envVarGroups:
  - name: shared
    envVars:
      - {key: LOG_LEVEL, value: info}
      - {key: GROUP_SECRET, generateValue: true}
services:
  - name: web
    type: web
    runtime: image
    image: {url: web:1}
    envVars:
      - {fromGroup: shared}
      - {key: PLAIN, value: hello}
      - {key: SESSION_SECRET, generateValue: true}
      - {key: PROMPTED, sync: false}
      - {key: SEEDED, value: once, sync: false}
      - {key: DB_PASS, fromService: {name: api, type: pserv, envVarKey: ROOT_PASS}}
  - name: api
    type: pserv
    runtime: image
    image: {url: api:1}
    envVars:
      - {key: ROOT_PASS, value: supersecret}
`

// --- validate (stateless) ------------------------------------------------------

func TestValidateBlueprintAcceptsAllFiveForms(t *testing.T) {
	// validate is stateless (no store, no seams) — a five-field blueprint must
	// validate clean (t006 DoD: the named-error rejection list is empty).
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", fiveFieldManifest)
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if !v.Valid || len(v.Errors) != 0 {
		t.Errorf("five-field blueprint: want valid with no errors, got %+v", v)
	}
	if v.Plan == nil || len(v.Plan.EnvGroups) != 1 || v.Plan.EnvGroups[0] != "shared" || v.Plan.TotalActions != 3 {
		t.Errorf("five-field blueprint: unexpected validation plan %+v", v.Plan)
	}
}

func TestValidateBlueprintPlansEnvGroupCreateAndConservativeUpdate(t *testing.T) {
	const manifest = `
envVarGroups:
  - name: shared
    envVars:
      - {key: LOG_LEVEL, value: info}
`
	groups := newFakeEnvGroups()
	svc, _ := newBlueprintEnvService(groups, &fakeSeeder{})

	created, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !created.Valid || created.Plan == nil {
		t.Fatalf("ValidateBlueprint(create): validation=%+v err=%v", created, err)
	}
	if created.Plan.Mode != "current_state" || len(created.Plan.Actions) != 1 {
		t.Fatalf("create plan = %+v, want one current-state action", created.Plan)
	}
	if action := created.Plan.Actions[0]; action.Kind != BlueprintResourceEnvVarGroup || action.Operation != BlueprintPlanCreate {
		t.Fatalf("create action = %+v, want env-var-group create", action)
	}

	if err := groups.ApplyEnvGroup(context.Background(), "shared", map[string]string{"LOG_LEVEL": "info"}, nil); err != nil {
		t.Fatalf("ApplyEnvGroup: %v", err)
	}
	updated, err := svc.ValidateBlueprint(context.Background(), "", manifest)
	if err != nil || !updated.Valid || updated.Plan == nil || len(updated.Plan.Actions) != 1 {
		t.Fatalf("ValidateBlueprint(update): validation=%+v err=%v", updated, err)
	}
	action := updated.Plan.Actions[0]
	if action.Operation != BlueprintPlanUpdate || action.ResourceID != "evg-shared" {
		t.Fatalf("update action = %+v, want conservative update for evg-shared", action)
	}
	if len(action.ChangedFields) == 0 {
		t.Fatalf("update action = %+v, want value-free changed field paths", action)
	}
}

// TestValidateFiveFormsCrossSurface asserts the five w1/m35 forms validate
// identically on every surface: REST, GraphQL, and the actual MCP tool. Each
// adapter must delegate to the same strict compiler rather than merely share a
// convenient Go helper in a unit test.
func TestValidateFiveFormsCrossSurface(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}

	// REST: POST /v1/blueprints/validate.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	body := fmt.Sprintf(`{"bexYaml":%q}`, fiveFieldManifest)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(body)))
	var rest BlueprintValidation
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatalf("REST unmarshal: %v", err)
	}
	if !rest.Valid || len(rest.Errors) != 0 {
		t.Errorf("REST validate: want valid, got %+v", rest)
	}

	// GraphQL: validateBlueprint(bexYaml).
	schema := blueprintSchema(t, svc)
	q := fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid errors } }`, fiveFieldManifest)
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL: %v", res.Errors)
	}
	gv := res.Data.(map[string]any)["validateBlueprint"].(map[string]any)
	if gv["valid"] != true {
		t.Errorf("GraphQL validate: want valid=true, got %+v", gv)
	}

	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	mcpResult := call("validate_bex_yml", map[string]any{"bexYaml": fiveFieldManifest})
	if mcpResult["valid"] != true {
		t.Errorf("MCP validate: want valid=true, got %+v", mcpResult)
	}

	const unsupported = `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1}
    autoDeployTrigger: checksPass
`
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(fmt.Sprintf(`{"bexYaml":%q}`, unsupported))))
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatalf("REST unsupported unmarshal: %v", err)
	}
	if rest.Valid || len(rest.Errors) != 1 || rest.Errors[0].Code != "BLUEPRINT_CAPABILITY_UNSUPPORTED" || rest.Errors[0].Path == nil || *rest.Errors[0].Path != "services[0].autoDeployTrigger" {
		t.Errorf("REST unsupported result = %+v", rest)
	}
	q = fmt.Sprintf(`{ validateBlueprint(bexYaml: %q) { valid errorDetails { code path } } }`, unsupported)
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL unsupported: %v", res.Errors)
	}
	gv = res.Data.(map[string]any)["validateBlueprint"].(map[string]any)
	details, _ := gv["errorDetails"].([]any)
	if gv["valid"] != false || len(details) != 1 || details[0].(map[string]any)["code"] != "BLUEPRINT_CAPABILITY_UNSUPPORTED" || details[0].(map[string]any)["path"] != "services[0].autoDeployTrigger" {
		t.Errorf("GraphQL unsupported result = %+v", gv)
	}
	mcpResult = call("validate_bex_yml", map[string]any{"bexYaml": unsupported})
	mcpErrors, _ := mcpResult["errors"].([]any)
	if mcpResult["valid"] != false || len(mcpErrors) != 1 {
		t.Errorf("MCP unsupported result = %+v", mcpResult)
	} else if detail, _ := mcpErrors[0].(map[string]any); detail["code"] != "BLUEPRINT_CAPABILITY_UNSUPPORTED" || detail["path"] != "services[0].autoDeployTrigger" {
		t.Errorf("MCP unsupported detail = %+v", detail)
	}
}

// --- parse ---------------------------------------------------------------------

func TestParseStackClassifiesEnvGroupsAndSeeds(t *testing.T) {
	st, err := parseStack(DeployRequest{Manifest: fiveFieldManifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	// envVarGroups → a parsed group with its literal + generate split.
	if len(st.envGroups) != 1 {
		t.Fatalf("envGroups = %d, want 1", len(st.envGroups))
	}
	g := st.envGroups[0]
	if g.name != "shared" || g.literals["LOG_LEVEL"] != "info" || len(g.generates) != 1 || g.generates[0] != "GROUP_SECRET" {
		t.Errorf("parsed group = %+v", g)
	}

	web := findSvc(t, st, "web")
	// fromGroup → a link; a plain literal stays on spec.Env.
	if len(web.groupLinks) != 1 || web.groupLinks[0] != "shared" {
		t.Errorf("groupLinks = %v, want [shared]", web.groupLinks)
	}
	if p := findEnv(t, web.req.Env, "PLAIN"); p.Value != "hello" {
		t.Errorf("plain literal not on spec.Env: %+v", p)
	}
	// generateValue + sync:false-with-value seed; sync:false-without-value seeds
	// nothing (accepted, sync-exempt, user sets it later).
	if len(web.seedGenerates) != 1 || web.seedGenerates[0] != "SESSION_SECRET" {
		t.Errorf("seedGenerates = %v, want [SESSION_SECRET]", web.seedGenerates)
	}
	if web.seedLiterals["SEEDED"] != "once" || len(web.seedLiterals) != 1 {
		t.Errorf("seedLiterals = %v, want {SEEDED:once}", web.seedLiterals)
	}
	// A seeded/generate var must NOT appear on spec.Env (the mutable store owns it).
	for _, e := range web.req.Env {
		if e.Name == "SESSION_SECRET" || e.Name == "SEEDED" || e.Name == "PROMPTED" {
			t.Errorf("%q leaked onto spec.Env (should be seed-only)", e.Name)
		}
	}
	// fromService.envVarKey copied the sibling's declared value at parse time.
	if dp := findEnv(t, web.req.Env, "DB_PASS"); dp.Value != "supersecret" {
		t.Errorf("envVarKey copy = %q, want supersecret", dp.Value)
	}
}

// --- apply ---------------------------------------------------------------------

func TestDeployStackAppliesEnvGroupsLinksAndSeeds(t *testing.T) {
	groups := newFakeEnvGroups()
	seeder := &fakeSeeder{}
	svc, cl := newBlueprintEnvService(groups, seeder)

	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: fiveFieldManifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	// The group was materialized with its literal + generate.
	ag, ok := groups.applied["shared"]
	if !ok || ag.literals["LOG_LEVEL"] != "info" || len(ag.generates) != 1 {
		t.Errorf("ApplyEnvGroup(shared) = %+v", ag)
	}
	// web linked the group.
	if len(groups.links) != 1 || groups.links[0] != "shared->web" {
		t.Errorf("links = %v, want [shared->web]", groups.links)
	}
	// web's sync:false + generateValue vars were seeded (once), api's were not.
	var webSeed *seedCall
	for i := range seeder.seeds {
		if seeder.seeds[i].service == "web" {
			webSeed = &seeder.seeds[i]
		}
	}
	if webSeed == nil {
		t.Fatalf("web was not seeded; seeds=%+v", seeder.seeds)
	}
	if webSeed.literals["SEEDED"] != "once" || len(webSeed.generates) != 1 || webSeed.generates[0] != "SESSION_SECRET" {
		t.Errorf("web seed = %+v", webSeed)
	}
	// Both services exist.
	for _, name := range []string{"web", "api"} {
		if getApp(t, cl, name).Name != name {
			t.Errorf("service %q not created", name)
		}
	}
}

func TestDeployStackFromGroupRequiresSensitiveBeforeWrites(t *testing.T) {
	const manifest = `
services:
  - name: web
    type: web
    runtime: image
    image: {url: attacker:1}
    envVars:
      - {fromGroup: shared}
`
	groups := newFakeEnvGroups("shared")
	seeder := &fakeSeeder{}
	existing := sampleApp("web")
	existing.Spec.Image = "trusted:1"
	existing.Spec.EnvFromSecrets = []string{"evg-shared-env"}
	svc, cl := newBlueprintEnvService(groups, seeder, existing)
	writeOnly := core.WithIdentity(context.Background(), core.Identity{
		Subject: "writer", Method: "oauth2", Human: true,
		CanonicalScopes: core.ScopeRead + " " + core.ScopeWrite,
	})

	if _, err := svc.DeployStack(writeOnly, DeployRequest{Manifest: manifest}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("write-only Blueprint fromGroup: %v, want ErrForbidden", err)
	}
	if len(groups.applied) != 0 || len(groups.links) != 0 || len(seeder.seeds) != 0 {
		t.Fatalf("sensitive denial occurred after writes: groups=%v links=%v seeds=%v", groups.applied, groups.links, seeder.seeds)
	}
	var app appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &app); err != nil {
		t.Fatalf("get unchanged service: %v", err)
	}
	if app.Spec.Image != "trusted:1" {
		t.Fatalf("sensitive denial replaced linked workload image with %q", app.Spec.Image)
	}

	allowed := core.WithIdentity(context.Background(), core.Identity{
		Subject: "developer", Method: "oauth2", Human: true,
		CanonicalScopes: core.ScopeRead + " " + core.ScopeWrite + " " + core.ScopeSensitive,
	})
	if _, err := svc.DeployStack(allowed, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("write+sensitive Blueprint fromGroup: %v", err)
	}
	if len(groups.links) != 1 || groups.links[0] != "shared->web" {
		t.Fatalf("allowed Blueprint did not link group: %v", groups.links)
	}
}

func TestDeployStackSeedsSuppliedSyncFalsePrompt(t *testing.T) {
	groups := newFakeEnvGroups()
	seeder := &fakeSeeder{}
	svc, cl := newBlueprintEnvService(groups, seeder)

	if _, err := svc.DeployStack(context.Background(), DeployRequest{
		Manifest:     fiveFieldManifest,
		EnvVarValues: map[string]string{"PROMPTED": "from-request"},
	}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	var webSeed *seedCall
	for i := range seeder.seeds {
		if seeder.seeds[i].service == "web" {
			webSeed = &seeder.seeds[i]
		}
	}
	if webSeed == nil || webSeed.literals["PROMPTED"] != "from-request" {
		t.Fatalf("prompt value was not seeded: %+v", webSeed)
	}
	for _, env := range getApp(t, cl, "web").Spec.Env {
		if env.Name == "PROMPTED" {
			t.Fatal("prompt value leaked into the App spec instead of the mutable env store")
		}
	}
}

func TestDeployStackRejectsUnknownSuppliedPrompt(t *testing.T) {
	svc, cl := newBlueprintEnvService(newFakeEnvGroups(), &fakeSeeder{})
	_, err := svc.DeployStack(context.Background(), DeployRequest{
		Manifest:     fiveFieldManifest,
		EnvVarValues: map[string]string{"NOT_A_PROMPT": "value"},
	})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "NOT_A_PROMPT") {
		t.Fatalf("unknown supplied prompt error = %v", err)
	}
	assertNoApps(t, cl)
}

func TestDeployStackFromGroupPreexistingGroup(t *testing.T) {
	// A fromGroup referencing a group that is NOT declared in-file but pre-exists in
	// the workspace links fine (no envVarGroups block needed).
	groups := newFakeEnvGroups("platform-config")
	svc, _ := newBlueprintEnvService(groups, &fakeSeeder{})
	m := "services:\n  - {name: web, type: web, runtime: image, image: {url: web:1}, envVars: [{fromGroup: platform-config}]}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(groups.links) != 1 || groups.links[0] != "platform-config->web" {
		t.Errorf("links = %v, want [platform-config->web]", groups.links)
	}
}

// --- all-or-nothing pre-flight -------------------------------------------------

func TestDeployStackUnknownFromGroupCreatesNothing(t *testing.T) {
	groups := newFakeEnvGroups() // no pre-existing groups
	svc, cl := newBlueprintEnvService(groups, &fakeSeeder{})
	m := "services:\n  - {name: web, type: web, runtime: image, image: {url: web:1}, envVars: [{fromGroup: ghost}]}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("unknown fromGroup => error naming it, got %v", err)
	}
	assertNoApps(t, cl)
}

func TestDeployStackEnvGroupsWithoutSeamRejected(t *testing.T) {
	// envVarGroups used but the env-groups seam is unavailable (OpenBao off).
	svc, cl := newBlueprintEnvService(nil, &fakeSeeder{})
	m := "envVarGroups:\n  - {name: g, envVars: [{key: K, value: v}]}\nservices:\n  - {name: web, type: web, runtime: image, image: {url: web:1}}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("envVarGroups without seam => unavailable error, got %v", err)
	}
	assertNoApps(t, cl)
}

func TestDeployStackSeedWithoutSeamRejected(t *testing.T) {
	// generateValue used but the env-vars seam is unavailable (OpenBao off).
	svc, cl := newBlueprintEnvService(newFakeEnvGroups(), nil)
	m := "services:\n  - {name: web, type: web, runtime: image, image: {url: web:1}, envVars: [{key: S, generateValue: true}]}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("generateValue without seam => unavailable error, got %v", err)
	}
	assertNoApps(t, cl)
}

func TestDeployStackFromGroupWithKeyRejected(t *testing.T) {
	// A fromGroup entry that also carries a key is malformed (it links the whole
	// group). Rejected at parse, before any seam call.
	svc, cl := newBlueprintEnvService(newFakeEnvGroups(), &fakeSeeder{})
	m := "services:\n  - {name: web, image: {url: web:1}, envVars: [{key: S, fromGroup: g}]}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("fromGroup with a key => ErrBadRequest, got %v", err)
	}
	assertNoApps(t, cl)
}

// --- environment-scoped env groups (m40) ----------------------------------------

const envScopedGroupManifest = `
projects:
  - name: platform
    environments:
      - name: production
        services:
          - name: web
            type: web
            runtime: image
            image: {url: web:1}
        envVarGroups:
          - name: prod-config
            envVars:
              - {key: LOG_LEVEL, value: warn}
              - {key: SECRET, generateValue: true}
`

func TestParseStackEnvironmentScopedEnvGroup(t *testing.T) {
	st, err := parseStack(DeployRequest{Manifest: envScopedGroupManifest})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	if len(st.envGroups) != 1 {
		t.Fatalf("envGroups = %d, want 1", len(st.envGroups))
	}
	g := st.envGroups[0]
	if g.name != "prod-config" {
		t.Errorf("group name = %q, want prod-config", g.name)
	}
	if g.literals["LOG_LEVEL"] != "warn" {
		t.Errorf("literals = %v", g.literals)
	}
	if len(g.generates) != 1 || g.generates[0] != "SECRET" {
		t.Errorf("generates = %v", g.generates)
	}
	if g.grouping == "" {
		t.Errorf("grouping is empty — environment-scoped group must carry its grouping key")
	}
}

func TestValidateBlueprintAcceptsEnvironmentScopedEnvGroup(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", envScopedGroupManifest)
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if !v.Valid || len(v.Errors) != 0 {
		t.Errorf("environment-scoped group: want valid, got %+v", v)
	}
	if v.Plan == nil || len(v.Plan.EnvGroups) != 1 || v.Plan.EnvGroups[0] != "prod-config" {
		t.Errorf("plan = %+v", v.Plan)
	}
}

func TestDeployStackEnvironmentScopedEnvGroupAssigned(t *testing.T) {
	grpStore := &blueprintGroupingTestStore{recordingStore: &recordingStore{}}
	groups := newFakeEnvGroups()
	svc, _ := newTenantStoreService(fakeWorkspace{"id-a": "tea-a"}, grpStore)
	svc.BlueprintGroups = grpStore
	svc.Environments = grpStore
	svc.EnvGroups = groups
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "oauth2"})

	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: envScopedGroupManifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	// Group must have been applied.
	if _, ok := groups.applied["prod-config"]; !ok {
		t.Errorf("ApplyEnvGroup not called for prod-config")
	}
	// Group must have been assigned to the created environment.
	envID := groups.environments["prod-config"]
	if envID == "" {
		t.Fatalf("SetGroupEnvironment not called for prod-config")
	}
	// Verify the assigned environment ID actually matches what was created.
	if len(grpStore.environments) == 0 {
		t.Fatalf("no environments created")
	}
	if grpStore.environments[0].ID != envID {
		t.Errorf("SetGroupEnvironment envID = %q, want %q", envID, grpStore.environments[0].ID)
	}
}

func assertNoApps(t *testing.T, cl client.Client) {
	t.Helper()
	var apps appv1alpha1.AppList
	_ = cl.List(context.Background(), &apps)
	if len(apps.Items) != 0 {
		t.Errorf("all-or-nothing: %d apps created, want zero", len(apps.Items))
	}
}

// --- apply result reporting (w6/064) --------------------------------------------

// allKindsManifest declares one resource of every kind the validation plan can
// act on: an env group, a database, a key value, and a web service. The apply
// result must report each kind back — the planner counts env_var_group as an
// action, so it is a reportable outcome.
const allKindsManifest = `
envVarGroups:
  - name: settings
    envVars:
      - {key: LOG_LEVEL, value: info}
databases:
  - name: db
    plan: basic-256mb
services:
  - name: cache
    type: redis
    ipAllowList: []
    plan: free
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
`

// TestDeployStackResultReportsEveryKind is the w6/064 regression at the
// StackResult (REST wire shape) level: a manifest declaring an env group used
// to apply it and then report {"services":…,"databases":…,"keyValues":…} with
// no env-group field at all — applyStackEnvGroups never received the result.
func TestDeployStackResultReportsEveryKind(t *testing.T) {
	groups := newFakeEnvGroups()
	svc, _ := newBlueprintEnvService(groups, &fakeSeeder{})

	res, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: allKindsManifest})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(res.EnvGroups) != 1 || res.EnvGroups[0].Name != "settings" || res.EnvGroups[0].ID != "evg-settings" {
		t.Errorf("envGroups = %+v, want [{ID:evg-settings Name:settings}]", res.EnvGroups)
	}
	// The `type: redis` entry is a key value, not a service — so one service (web).
	if len(res.Databases) != 1 || len(res.KeyValues) != 1 || len(res.Services) != 1 {
		t.Errorf("result = %d databases / %d keyValues / %d services, want 1/1/1", len(res.Databases), len(res.KeyValues), len(res.Services))
	}

	// REST is StackResult marshalled as-is: the raw wire object must carry the
	// envGroups key (named after the GET /v1/env-groups vocabulary — Render's
	// public API declares no blueprint apply-result shape to mirror).
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	body := fmt.Sprintf(`{"bexYaml":%q}`, allKindsManifest)
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/blueprints/deploy", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("deploy Blueprint => 200, got %d: %s", rec.Code, rec.Body)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &wire); err != nil {
		t.Fatalf("unmarshal deploy result: %v", err)
	}
	for _, kind := range []string{"services", "databases", "keyValues", "envGroups"} {
		if _, ok := wire[kind]; !ok {
			t.Errorf("REST deploy result missing %q: %s", kind, rec.Body)
		}
	}
	var restGroups []StackEnvGroupView
	if err := json.Unmarshal(wire["envGroups"], &restGroups); err != nil {
		t.Fatalf("unmarshal envGroups: %v", err)
	}
	if len(restGroups) != 1 || restGroups[0].Name != "settings" || restGroups[0].ID != "evg-settings" {
		t.Errorf("REST envGroups = %+v, want the applied group's id+name", restGroups)
	}
}

// TestSyncBlueprintGraphQLReportsKeyValuesAndEnvGroups is w6/064's GraphQL leg:
// syncBlueprint's result used to expose only services + databases, dropping the
// keyValues REST and MCP both report AND the (new) envGroups — both follow the
// databases field's names-only precedent.
func TestSyncBlueprintGraphQLReportsKeyValuesAndEnvGroups(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a"}
	fs := newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		TenantID: "tea-a",
		Repo:     "https://github.com/a/app",
		Branch:   "main",
		Path:     CanonicalBlueprintFilename,
		Manifest: allKindsManifest,
		Status:   "active",
		Name:     "app",
	})
	groups := newFakeEnvGroups()
	svc := &Service{
		Base:            &core.Base{Client: fakeClient(), Namespace: "default", Workspace: ws},
		Blueprints:      fs,
		EnvGroups:       groups,
		EnvSeeder:       &fakeSeeder{},
		DomainOwnership: allowDomainOwnership{},
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})

	schema := blueprintSchema(t, svc)
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       ctx,
		RequestString: `mutation { syncBlueprint(id: "blp-1") { databases keyValues envGroups } }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL syncBlueprint: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["syncBlueprint"].(map[string]any)
	if names, _ := got["databases"].([]any); len(names) != 1 || names[0] != "db" {
		t.Errorf("databases = %v, want [db]", got["databases"])
	}
	if names, _ := got["keyValues"].([]any); len(names) != 1 || names[0] != "cache" {
		t.Errorf("keyValues = %v, want [cache]", got["keyValues"])
	}
	if names, _ := got["envGroups"].([]any); len(names) != 1 || names[0] != "settings" {
		t.Errorf("envGroups = %v, want [settings]", got["envGroups"])
	}
}

// TestMCPDeployReportsEveryKind is w6/064's MCP leg: the deploy tool's
// renderStack maps StackResult field by field, so the env-group views must be
// carried through explicitly — confirmed here rather than assumed.
func TestMCPDeployReportsEveryKind(t *testing.T) {
	groups := newFakeEnvGroups()
	svc, _ := newBlueprintEnvService(groups, &fakeSeeder{})
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	got := call("deploy", map[string]any{"bexYaml": allKindsManifest})
	if services, _ := got["services"].([]any); len(services) != 1 {
		t.Errorf("services = %v, want the single web service", got["services"])
	}
	databases, _ := got["databases"].([]any)
	if len(databases) != 1 || databases[0].(map[string]any)["name"] != "db" {
		t.Errorf("databases = %v, want [{name:db …}]", got["databases"])
	}
	keyValues, _ := got["keyValues"].([]any)
	if len(keyValues) != 1 || keyValues[0].(map[string]any)["name"] != "cache" {
		t.Errorf("keyValues = %v, want [{name:cache …}]", got["keyValues"])
	}
	envGroups, _ := got["envGroups"].([]any)
	if len(envGroups) != 1 || envGroups[0].(map[string]any)["name"] != "settings" || envGroups[0].(map[string]any)["id"] != "evg-settings" {
		t.Errorf("envGroups = %v, want [{id:evg-settings name:settings}]", got["envGroups"])
	}
}
