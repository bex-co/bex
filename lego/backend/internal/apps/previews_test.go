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
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestBlueprintPreviewOptOut(t *testing.T) {
	base := "services:\n  - name: api\n    type: web\n    runtime: image\n    image: {url: nginx}\n"
	for _, tc := range []struct {
		name, root, service string
		accepted            bool
	}{
		{"root off", "previews: {generation: off}\n", "", true},
		{"root off with expiry", "previews: {generation: off, expireAfterDays: 3}\n", "", true},
		{"root false", "previewsEnabled: false\n", "", true},
		{"service off", "", "previews: {generation: off}", true},
		{"service false", "", "pullRequestPreviewsEnabled: false", true},
		{"root automatic", "previews: {generation: automatic}\n", "", false},
		{"root manual", "previews: {generation: manual}\n", "", false},
		{"root enabled", "previewsEnabled: true\n", "", false},
		{"service enabled", "", "pullRequestPreviewsEnabled: true", false},
		{"service automatic", "", "previews: {generation: automatic}", false},
		{"preview plan", "", "previewPlan: free", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := tc.root + base
			if tc.service != "" {
				manifest += "    " + tc.service + "\n"
			}
			_, problems := CompileBlueprintSource(manifest)
			if tc.accepted {
				if len(problems) != 0 {
					t.Fatalf("opt-out refused: %+v", problems)
				}
				return
			}
			problem := findBlueprintProblem(problems, "BLUEPRINT_CAPABILITY_UNSUPPORTED")
			if problem == nil || problem.Line == 0 || problem.Column == 0 || !strings.Contains(problem.Message, "preview") {
				t.Fatalf("missing located refusal: %+v", problems)
			}
		})
	}
}

func TestRESTPreviewOptOut(t *testing.T) {
	for _, tc := range []struct {
		details  string
		accepted bool
	}{
		{`"previews":{"generation":"off"}`, true},
		{`"previews":{"generation":"off","expireAfterDays":3}`, true},
		{`"pullRequestPreviewsEnabled":"no"`, true},
		{`"previews":{"generation":"automatic"}`, false},
		{`"previews":{"generation":"manual"}`, false},
		{`"pullRequestPreviewsEnabled":"yes"`, false},
		{`"previews":false`, false},
		{`"previews":"no"`, false},
		{`"previews":{"generation":"off","numInstances":2}`, false},
		{`"previews":{"generation":"off","expireAfterDays":0}`, false},
	} {
		for _, method := range []string{http.MethodPost, http.MethodPatch} {
			t.Run(method+tc.details, func(t *testing.T) {
				original := sampleApp("web")
				svc, cl := newService(nil, original)
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				path := "/v1/services/web"
				body := fmt.Sprintf(`{"serviceDetails":{%s}}`, tc.details)
				if method == http.MethodPost {
					path = "/v1/services"
					body = fmt.Sprintf(`{"name":"new","type":"web_service","image":{"imagePath":"nginx"},"serviceDetails":{%s}}`, tc.details)
				}
				rec := httptest.NewRecorder()
				validated(t, mux).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
				if tc.accepted {
					if rec.Code < 200 || rec.Code >= 300 {
						t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
					}
					if method == http.MethodPatch && !reflect.DeepEqual(original.Spec, getApp(t, cl, "web").Spec) {
						t.Fatal("opt-out changed service spec")
					}
				} else if rec.Code != http.StatusBadRequest {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
				}
			})
		}
	}
}

// previewValueManifest declares a previewValue on an env group entry and on a
// service entry; {{root}} and {{service}} take the preview settings under test.
const previewValueManifest = `{{root}}envVarGroups:
  - name: shared
    envVars:
      - key: MODE
        value: prod
        previewValue: preview
services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx}
{{service}}    envVars:
      - fromGroup: shared
      - key: API_URL
        value: https://api.example.com
        previewValue: https://preview-api.example.com
`

func previewValueBlueprint(root, service string) string {
	if service != "" {
		service = "    " + service + "\n"
	}
	return strings.NewReplacer("{{root}}", root, "{{service}}", service).Replace(previewValueManifest)
}

// Render applies previewValue only in preview environments, so a Blueprint
// that enables none accepts it with no effect, recorded for a warning notice;
// one that enables previews refuses it beside the toggle, so preview intent
// is never silently dropped (w32/032).
func TestBlueprintPreviewValueWithoutPreviews(t *testing.T) {
	inert := []string{"#/envVarGroups/0/envVars/0/previewValue", "#/services/0/envVars/1/previewValue"}
	for _, tc := range []struct {
		name, root, service string
		enabled             bool
	}{
		{"no preview settings", "", "", false},
		{"root off", "previews: {generation: off}\n", "", false},
		{"root off with expiry", "previews: {generation: off, expireAfterDays: 3}\n", "", false},
		{"root false", "previewsEnabled: false\n", "", false},
		{"service off", "", "previews: {generation: off}", false},
		{"service false", "", "pullRequestPreviewsEnabled: false", false},
		{"root automatic", "previews: {generation: automatic}\n", "", true},
		{"root manual", "previews: {generation: manual}\n", "", true},
		{"root enabled", "previewsEnabled: true\n", "", true},
		{"service enabled", "", "pullRequestPreviewsEnabled: true", true},
		{"service automatic", "", "previews: {generation: automatic}", true},
		{"root off but service enabled", "previews: {generation: off}\n", "pullRequestPreviewsEnabled: true", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, problems := CompileBlueprintSource(previewValueBlueprint(tc.root, tc.service))
			if !tc.enabled {
				if len(problems) != 0 {
					t.Fatalf("previewValue refused without previews: %+v", problems)
				}
				if !reflect.DeepEqual(source.inertPreviewFields, inert) {
					t.Fatalf("inert preview fields = %v, want %v", source.inertPreviewFields, inert)
				}
				return
			}
			for _, path := range inert {
				problem := findBlueprintProblemAtPath(problems, path)
				if problem == nil || problem.Code != "BLUEPRINT_CAPABILITY_UNSUPPORTED" || problem.Line == 0 || problem.Column == 0 ||
					!strings.Contains(problem.Message, "previewValue is not available on bex") {
					t.Errorf("previewValue at %s = %+v, want a located refusal; all = %+v", path, problem, problems)
				}
			}
			if len(source.inertPreviewFields) != 0 {
				t.Errorf("inert preview fields = %v, want none in a file that enables previews", source.inertPreviewFields)
			}
		})
	}
}

// An accepted previewValue changes nothing the apply writes: each entry keeps
// its own value. Validate stays valid and holds one warning per previewValue,
// in source order, which a plan reports as Render's warning notices (w32/032).
func TestBlueprintPreviewValueAppliesTheEntryValue(t *testing.T) {
	manifest := previewValueBlueprint("", "")
	source, ir, problems := CompileBlueprintIR(manifest)
	if len(problems) != 0 {
		t.Fatalf("compile: %+v", problems)
	}
	st, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(st.envGroups) != 1 || !reflect.DeepEqual(st.envGroups[0].literals, map[string]string{"MODE": "prod"}) {
		t.Errorf("env groups = %+v, want MODE=prod", st.envGroups)
	}
	if len(st.services) != 1 || len(st.services[0].req.Env) != 1 || st.services[0].req.Env[0].Name != "API_URL" ||
		st.services[0].req.Env[0].Value != "https://api.example.com" {
		t.Errorf("service env = %+v, want API_URL=https://api.example.com", st.services)
	}

	want := []struct {
		path string
		line int
	}{{"envVarGroups[0].envVars[0].previewValue", 6}, {"services[0].envVars[1].previewValue", 16}}
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	svc.EnvGroups = newFakeEnvGroups()
	validation, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !validation.Valid || len(validation.warnings) != len(want) {
		t.Fatalf("validation = %+v, %v; want valid with %d warnings", validation, err, len(want))
	}
	for i, w := range want {
		got := validation.warnings[i]
		if got.Path == nil || *got.Path != w.path || got.Line == nil || *got.Line != w.line ||
			got.Error != "previewValue has no effect: this Blueprint creates no preview environments" {
			t.Errorf("warning %d = %+v, want %s at line %d", i, got, w.path, w.line)
		}
	}

	planner, _, ctx := newOnboardingService(t, manifest)
	planner.EnvGroups = newFakeEnvGroups()
	res, err := planner.CreatePlannedBlueprint(ctx, RenderCreateBlueprintRequest{Name: "stack", OwnerID: "tea-a", Source: RenderBlueprintSource{Repo: "https://github.com/a/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.PlanningOutcome != PlanningPlanned || len(res.Notices) != len(want) {
		t.Fatalf("plan = %+v, want planned with %d notices", res, len(want))
	}
	for i, w := range want {
		n := res.Notices[i]
		if n.Severity != "warning" || n.Path == nil || !strings.HasSuffix(*n.Path, w.path) || n.Line == nil || *n.Line != w.line {
			t.Errorf("notice %d = %+v, want a warning at %s, line %d", i, n, w.path, w.line)
		}
	}
}
