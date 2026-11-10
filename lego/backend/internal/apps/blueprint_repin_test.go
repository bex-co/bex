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
	"slices"
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
		tier, err := normalizeTierForType(effectiveType(svc.req.Type), svc.req.Plan, "")
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
// plan field, never rounded to a neighbouring rung (w8/011's rule). Every
// keyValuePlan value has a rung since w5/m266 (TestBlueprintKeyValueLargePlans).
func TestBlueprintRejectsComputePlanSizesBexDoesNotOffer(t *testing.T) {
	for _, tc := range []struct{ name, manifest, path string }{
		{"web", "services:\n  - type: web\n    name: web\n    runtime: image\n    image: {url: nginx:1.27}\n    plan: 2c-8g\n", "#/services/0/plan"},
		{"cron", "services:\n  - type: cron\n    name: c\n    runtime: image\n    image: {url: nginx:1.27}\n    schedule: '* * * * *'\n    plan: 8c-64g\n", "#/services/0/plan"},
		{"postgres", "databases:\n  - name: db\n    plan: 2c-8g\n", "#/databases/0/plan"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireOneProblem(t, tc.manifest, "BLUEPRINT_CAPABILITY_UNSUPPORTED", tc.path, "compute plan size that bex does not offer")
		})
	}
}

// Render sells no free cron, worker or private service (w8/048, w1/111). A
// Blueprint refuses each at its own services[N].plan in Blueprint spelling,
// every hinted plan validates when written back (w6/093, w8/241), and a file
// with all three reports all three in one run.
func TestBlueprintPaidOnlyTypesRefuseFreeInBlueprintSpelling(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	validate := func(t *testing.T, manifest string) BlueprintValidation {
		t.Helper()
		v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
		if err != nil {
			t.Fatalf("ValidateBlueprint: %v", err)
		}
		return v
	}
	for _, tc := range []struct{ entry, svcType, hint string }{
		{"  - type: cron\n    name: c\n    runtime: image\n    image: {url: nginx:1.27}\n    schedule: '* * * * *'\n    plan: free\n", "cron_job", "starter|standard|pro|pro plus"},
		{"  - type: worker\n    name: w\n    runtime: image\n    image: {url: nginx:1.27}\n    plan: free\n", "background_worker", ""},
		{"  - type: pserv\n    name: p\n    runtime: image\n    image: {url: nginx:1.27}\n    plan: free\n", "private_service", ""},
	} {
		t.Run(tc.svcType, func(t *testing.T) {
			manifest := "services:\n" + tc.entry
			v := validate(t, manifest)
			if v.Valid || len(v.Errors) != 1 || v.Errors[0].Path == nil || *v.Errors[0].Path != "services[0].plan" ||
				!strings.Contains(v.Errors[0].Error, "a "+tc.svcType+" requires a paid plan; plan must be one of ") {
				t.Fatalf("plan: free = %+v, want the named paid-plan refusal at services[0].plan", v.Errors)
			}
			message := v.Errors[0].Error
			hinted := strings.Split(message[strings.Index(message, "one of ")+len("one of "):], "|")
			if tc.hint != "" && strings.Join(hinted, "|") != tc.hint {
				t.Errorf("hint = %v, want %s", hinted, tc.hint)
			}
			if slices.ContainsFunc(hinted, func(plan string) bool { return strings.Contains(plan, "_") }) {
				t.Errorf("hint %v uses the API's underscore spelling", hinted)
			}
			for _, plan := range hinted {
				if v := validate(t, strings.Replace(manifest, "plan: free", "plan: "+plan, 1)); !v.Valid {
					t.Errorf("hinted plan %q is refused: %+v", plan, v.Errors)
				}
			}
		})
	}

	three := "services:\n" +
		"  - type: cron\n    name: c\n    runtime: image\n    image: {url: nginx:1.27}\n    schedule: '* * * * *'\n    plan: free\n" +
		"  - type: worker\n    name: w\n    runtime: image\n    image: {url: nginx:1.27}\n    plan: free\n" +
		"  - type: pserv\n    name: p\n    runtime: image\n    image: {url: nginx:1.27}\n    plan: free\n"
	var paths []string
	for _, e := range validate(t, three).Errors {
		if e.Path != nil {
			paths = append(paths, *e.Path)
		}
	}
	slices.Sort(paths)
	if want := []string{"services[0].plan", "services[1].plan", "services[2].plan"}; !slices.Equal(paths, want) {
		t.Fatalf("three free paid-only services report %v, want %v", paths, want)
	}
}

// Render Build Sources build once and deploy the same artifact to every
// linked service (w2/m254's carriers, ADR097). Every place the schema admits a
// build source compiles (w3/m270). A linked service carries no runtime/repo
// (the schema no longer requires them there), and the conditional branch that
// forbids source fields beside buildSource raises no schema noise.
func TestBlueprintAcceptsBuildSourcesWhereverTheSchemaAdmitsThem(t *testing.T) {
	const source = "  - name: shared\n    git:\n      repo: https://github.com/bex-co/bex\n      runtime: docker\n"
	for _, tc := range []struct{ name, manifest string }{
		{"root", "buildSources:\n" + source},
		{"ungrouped", "ungrouped:\n  buildSources:\n  " + strings.ReplaceAll(source, "\n  ", "\n    ")},
		{"project", "projects:\n  - name: p\n    buildSources:\n    " + strings.ReplaceAll(source, "\n  ", "\n      ") + "    environments:\n      - name: prod\n"},
		{"service link", "services:\n  - type: web\n    name: web\n    buildSource: shared\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, problems := CompileBlueprintSource(tc.manifest); len(problems) > 0 {
				t.Fatalf("problems = %+v, want none", problems)
			}
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
