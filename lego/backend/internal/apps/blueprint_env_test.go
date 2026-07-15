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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- fake seams ---------------------------------------------------------------

type appliedGroup struct {
	literals  map[string]string
	generates []string
}

type fakeEnvGroups struct {
	preexisting []string                // names that already exist in the workspace
	applied     map[string]appliedGroup // name -> last ApplyEnvGroup args
	links       []string                // "group->service" in call order
}

func newFakeEnvGroups(preexisting ...string) *fakeEnvGroups {
	return &fakeEnvGroups{preexisting: preexisting, applied: map[string]appliedGroup{}}
}

func (f *fakeEnvGroups) GroupNames(context.Context) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, n := range f.preexisting {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for n := range f.applied {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
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
    image: web:1
    envVars:
      - {fromGroup: shared}
      - {key: PLAIN, value: hello}
      - {key: SESSION_SECRET, generateValue: true}
      - {key: PROMPTED, sync: false}
      - {key: SEEDED, value: once, sync: false}
      - {key: DB_PASS, fromService: {name: api, type: pserv, envVarKey: ROOT_PASS}}
  - name: api
    type: pserv
    image: api:1
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

// TestValidateFiveFormsCrossSurface asserts the five w1/m35 forms validate
// identically on every surface (t007): REST, GraphQL, and the core verb the MCP
// validate_bex_yml tool calls all route through the one ValidateBlueprint →
// parseStack core, so no surface can drift on which fields it accepts.
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

func TestDeployStackFromGroupPreexistingGroup(t *testing.T) {
	// A fromGroup referencing a group that is NOT declared in-file but pre-exists in
	// the workspace links fine (no envVarGroups block needed).
	groups := newFakeEnvGroups("platform-config")
	svc, _ := newBlueprintEnvService(groups, &fakeSeeder{})
	m := "services:\n  - {name: web, image: web:1, envVars: [{fromGroup: platform-config}]}\n"
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
	m := "services:\n  - {name: web, image: web:1, envVars: [{fromGroup: ghost}]}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("unknown fromGroup => error naming it, got %v", err)
	}
	assertNoApps(t, cl)
}

func TestDeployStackEnvGroupsWithoutSeamRejected(t *testing.T) {
	// envVarGroups used but the env-groups seam is unavailable (OpenBao off).
	svc, cl := newBlueprintEnvService(nil, &fakeSeeder{})
	m := "envVarGroups:\n  - {name: g, envVars: [{key: K, value: v}]}\nservices:\n  - {name: web, image: web:1}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("envVarGroups without seam => unavailable error, got %v", err)
	}
	assertNoApps(t, cl)
}

func TestDeployStackSeedWithoutSeamRejected(t *testing.T) {
	// generateValue used but the env-vars seam is unavailable (OpenBao off).
	svc, cl := newBlueprintEnvService(newFakeEnvGroups(), nil)
	m := "services:\n  - {name: web, image: web:1, envVars: [{key: S, generateValue: true}]}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("generateValue without seam => unavailable error, got %v", err)
	}
	assertNoApps(t, cl)
}

func TestDeployStackFromGroupWithKeyRejected(t *testing.T) {
	// A fromGroup entry that also carries a key is malformed (it links the whole
	// group). Rejected at parse, before any seam call.
	svc, cl := newBlueprintEnvService(newFakeEnvGroups(), &fakeSeeder{})
	m := "services:\n  - {name: web, image: web:1, envVars: [{key: S, fromGroup: g}]}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: m}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("fromGroup with a key => ErrBadRequest, got %v", err)
	}
	assertNoApps(t, cl)
}

func assertNoApps(t *testing.T, cl client.Client) {
	t.Helper()
	var apps appv1alpha1.AppList
	_ = cl.List(context.Background(), &apps)
	if len(apps.Items) != 0 {
		t.Errorf("all-or-nothing: %d apps created, want zero", len(apps.Items))
	}
}
