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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/m170 re-pinned Render's Blueprint schema to the 2026-10-02 snapshot.
// These tests pin what each upstream addition does on bex: accepted where bex
// has the same thing, refused with a named error where it does not, and never
// silently dropped.

// requireOneProblem asserts the compiler returned exactly one diagnostic, with
// this code at this path, so a refusal can never hide behind schema noise.
func requireOneProblem(t *testing.T, manifest, code, path, messageFragment string) {
	t.Helper()
	_, problems := CompileBlueprintSource(manifest)
	if len(problems) != 1 {
		t.Fatalf("problems = %+v, want exactly one %s at %s", problems, code, path)
	}
	got := problems[0]
	if got.Code != code || got.Path != path || !strings.Contains(got.Message, messageFragment) {
		t.Fatalf("problem = %+v, want %s at %s containing %q", got, code, path, messageFragment)
	}
	if got.Line == 0 {
		t.Fatalf("problem %+v has no source line", got)
	}
}

// Render's blueprint-spec now names plans by compute plan ID (a new service
// defaults to 0.5c-512mb). An ID whose CPU/RAM equals a bex rung must compile
// and land on that rung; before the re-pin the schema refused all of them.
func TestBlueprintRenderComputePlanIDsResolveToTheSameRung(t *testing.T) {
	manifest := `
services:
  - type: web
    name: web
    runtime: image
    image: {url: nginx:1.27}
    plan: 0.5c-512mb
  - type: worker
    name: worker
    runtime: image
    image: {url: nginx:1.27}
    plan: 4c-16g
  - type: cron
    name: cron
    runtime: image
    image: {url: nginx:1.27}
    schedule: "*/5 * * * *"
    plan: 1c-2g
  - type: keyvalue
    name: cache
    plan: 256mb
    ipAllowList: []
databases:
  - name: db
    plan: 0.1c-256mb
`
	st := parseBlueprintStackForTest(t, manifest)
	wantTier := map[string]string{"web": "starter", "worker": "pro-max", "cron": "standard"}
	for _, svc := range st.services {
		tier, err := normalizeTierForType(effectiveType(svc.req.Type), svc.req.Plan)
		if err != nil || tier != wantTier[svc.req.Name] {
			t.Errorf("service %s plan %q => tier %q, %v; want %q", svc.req.Name, svc.req.Plan, tier, err, wantTier[svc.req.Name])
		}
	}
	if len(st.keyValues) != 1 || st.keyValues[0].spec.Plan != "starter" {
		t.Errorf("key value plan 256mb => %+v, want starter", st.keyValues)
	}
	if len(st.databases) != 1 || st.databases[0].spec.Plan != "basic-256mb" {
		t.Errorf("database plan 0.1c-256mb => %+v, want basic-256mb", st.databases)
	}

	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !v.Valid {
		t.Fatalf("ValidateBlueprint(compute plan IDs) = %+v, %v; want valid", v, err)
	}
}

// The compute aliases live in the tier catalog's ByRenderPlan, so the direct
// create and plan-change paths (REST, GraphQL, MCP all funnel here) accept the
// same exact-size IDs the Blueprint does, and still refuse the rest.
func TestDirectPlanWritesAcceptRenderComputePlanIDs(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{
		Name: "web", Type: appv1alpha1.TypeWebService, Image: "nginx:1", Plan: "0.5c-512mb",
	}); err != nil {
		t.Fatalf("Create plan 0.5c-512mb: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "starter" {
		t.Fatalf("plan 0.5c-512mb tier = %q, want starter", got)
	}
	if _, err := svc.SetPlan(context.Background(), "web", "2c-4g"); err != nil {
		t.Fatalf("SetPlan 2c-4g: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "pro" {
		t.Fatalf("plan 2c-4g tier = %q, want pro", got)
	}
	if _, err := svc.SetPlan(context.Background(), "web", "2c-8g"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("SetPlan 2c-8g = %v, want a bad-request refusal (no same-size rung)", err)
	}
}

// A compute plan ID with no same-size bex rung is refused by name at its
// plan field, never rounded to a neighbouring rung (w8/011's rule).
func TestBlueprintRejectsComputePlanSizesBexDoesNotOffer(t *testing.T) {
	for _, tc := range []struct{ name, manifest, path string }{
		{"web", "services:\n  - type: web\n    name: web\n    runtime: image\n    image: {url: nginx:1.27}\n    plan: 2c-8g\n", "#/services/0/plan"},
		{"cron", "services:\n  - type: cron\n    name: c\n    runtime: image\n    image: {url: nginx:1.27}\n    schedule: '* * * * *'\n    plan: 8c-64g\n", "#/services/0/plan"},
		{"keyvalue", "services:\n  - type: keyvalue\n    name: kv\n    plan: 10g\n    ipAllowList: []\n", "#/services/0/plan"},
		{"postgres", "databases:\n  - name: db\n    plan: 2c-8g\n", "#/databases/0/plan"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireOneProblem(t, tc.manifest, "BLUEPRINT_CAPABILITY_UNSUPPORTED", tc.path, "compute plan size that bex does not offer")
		})
	}
}

// Render's cronPlan has no free rung, but bex keeps a free cron tier (it is
// also the omitted-plan default), and tenant Blueprints already say plan: free
// on cron jobs. That stays valid — a documented bex divergence (ADR049) — and
// lands on the free tier; the rest of cronPlan is unchanged.
func TestBlueprintCronPlanFreeStaysValidOnBex(t *testing.T) {
	manifest := "services:\n  - type: cron\n    name: c\n    runtime: image\n    image: {url: nginx:1.27}\n    schedule: '* * * * *'\n    plan: free\n"
	st := parseBlueprintStackForTest(t, manifest)
	if len(st.services) != 1 {
		t.Fatalf("services = %+v, want one cron job", st.services)
	}
	tier, err := normalizeTierForType(effectiveType(st.services[0].req.Type), st.services[0].req.Plan)
	if err != nil || tier != "free" {
		t.Fatalf("cron plan: free => tier %q, %v; want free", tier, err)
	}
	_, problems := CompileBlueprintSource(strings.Replace(manifest, "plan: free", "plan: nonsense", 1))
	if len(problems) != 1 || problems[0].Code != "BLUEPRINT_SCHEMA_INVALID" || problems[0].Path != "#/services/0/plan" {
		t.Fatalf("cron plan: nonsense problems = %+v, want one schema error at #/services/0/plan", problems)
	}
}

// Render Build Sources (private beta) build once and deploy the same artifact
// to every linked service, with build-only env vars. bex has neither, so every
// place the schema admits a build source is refused by name. A linked service
// carries no runtime/repo (the schema no longer requires them there): the one
// diagnostic must be the build source, not schema noise from the conditional
// branch that forbids source fields beside it.
func TestBlueprintRejectsBuildSourcesWithNamedError(t *testing.T) {
	const source = "  - name: shared\n    git:\n      repo: https://github.com/bex-co/bex\n      runtime: docker\n"
	for _, tc := range []struct{ name, manifest, path string }{
		{"root", "buildSources:\n" + source, "#/buildSources"},
		{"ungrouped", "ungrouped:\n  buildSources:\n  " + strings.ReplaceAll(source, "\n  ", "\n    "), "#/ungrouped/buildSources"},
		{"project", "projects:\n  - name: p\n    buildSources:\n    " + strings.ReplaceAll(source, "\n  ", "\n      ") + "    environments:\n      - name: prod\n", "#/projects/0/buildSources"},
		{"service link", "services:\n  - type: web\n    name: web\n    buildSource: shared\n", "#/services/0/buildSource"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireOneProblem(t, tc.manifest, "BLUEPRINT_CAPABILITY_UNSUPPORTED", tc.path, "Render Build Sources")
		})
	}
}

// Render Workflows are a recorded non-goal: a workflow service is refused with
// one named error on its type (not one per field), and so are the two
// reference forms that only make sense against a workflow.
func TestBlueprintRejectsWorkflowServicesWithNamedError(t *testing.T) {
	requireOneProblem(t, `
services:
  - type: workflow
    name: tasks
    runtime: python
    region: oregon
    startCommand: python main.py
    repo: https://github.com/bex-co/bex
`, "BLUEPRINT_CAPABILITY_UNSUPPORTED", "#/services/0/type", "Render Workflows")

	for _, tc := range []struct{ name, fromService, path string }{
		{"type", "{type: workflow, name: tasks, property: host}", "#/services/0/envVars/0/fromService/type"},
		{"slug", "{type: web, name: other, property: slug}", "#/services/0/envVars/0/fromService/property"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireOneProblem(t, "services:\n  - type: web\n    name: web\n    runtime: image\n    image: {url: nginx:1.27}\n    envVars:\n      - key: X\n        fromService: "+tc.fromService+"\n",
				"BLUEPRINT_CAPABILITY_UNSUPPORTED", tc.path, "Render Workflows")
		})
	}
}

// The re-pinned schema lets a project omit environments (so it can hold only
// buildSources). bex builds a project from its environments, so an
// environment-less project would apply as a silent no-op; it is refused.
func TestBlueprintProjectWithoutEnvironmentsIsRefused(t *testing.T) {
	requireOneProblem(t, "projects:\n  - name: p\n", "BLUEPRINT_PROJECT_ENVIRONMENTS_REQUIRED", "#/projects/0", "at least one environment")
}
