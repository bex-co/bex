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
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeBlueprintIRPreservesPresenceAndGrouping(t *testing.T) {
	source, problems := CompileBlueprintSource(`
envVarGroups:
  - name: shared
    envVars: []
services:
  - type: keyvalue
    name: cache
    ipAllowList: []
ungrouped:
  services:
    - type: web
      name: public
      runtime: image
      image: {url: nginx:1.27}
      autoDeploy: false
projects:
  - name: app
    environments:
      - name: prod
        services:
          - type: web
            name: api
            runtime: image
            image: {url: nginx:1.27}
        databases:
          - name: data
`)
	if len(problems) != 0 {
		t.Fatalf("CompileBlueprintSource() problems = %+v", problems)
	}
	ir, problems := NormalizeBlueprintIR(source)
	if len(problems) != 0 {
		t.Fatalf("NormalizeBlueprintIR() problems = %+v", problems)
	}
	if got, want := len(ir.Resources), 5; got != want {
		t.Fatalf("resource count = %d, want %d (%+v)", got, want, ir.Resources)
	}

	var public, api BlueprintResourceIR
	for _, resource := range ir.Resources {
		switch resource.Name {
		case "public":
			public = resource
		case "api":
			api = resource
		}
	}
	if !public.Ungrouped || public.Project != "" || public.Environment != "" {
		t.Fatalf("ungrouped public resource = %+v", public)
	}
	if public.Fields["autoDeploy"].Value != false {
		t.Fatalf("explicit false autoDeploy field = %+v", public.Fields["autoDeploy"])
	}
	if _, present := public.Fields["plan"]; present {
		t.Fatalf("omitted plan was represented as present: %+v", public.Fields["plan"])
	}
	if api.Project != "app" || api.Environment != "prod" || api.Ungrouped {
		t.Fatalf("environment-scoped api resource = %+v", api)
	}
}

func TestNormalizeBlueprintIRRejectsDuplicateResourceAcrossLocations(t *testing.T) {
	source, problems := CompileBlueprintSource(`
services:
  - type: web
    name: api
    runtime: image
    image: {url: nginx:1.27}
ungrouped:
  services:
    - type: web
      name: api
      runtime: image
      image: {url: nginx:1.27}
`)
	if len(problems) != 0 {
		t.Fatalf("CompileBlueprintSource() problems = %+v", problems)
	}
	_, problems = NormalizeBlueprintIR(source)
	if findBlueprintProblem(problems, "BLUEPRINT_DUPLICATE_RESOURCE") == nil {
		t.Fatalf("duplicate resource problems = %+v", problems)
	}
}

func TestPlanBlueprintIRIsDeterministicAndSecretSafe(t *testing.T) {
	ir := BlueprintIR{Resources: []BlueprintResourceIR{
		{
			Kind: BlueprintResourceService, Name: "api", SourcePath: "#/services/0",
			Fields: map[string]BlueprintField{
				"name": {Value: "api"}, "plan": {Value: "starter"}, "envVars": {Value: []any{map[string]any{"key": "TOKEN", "value": "secret-not-in-plan"}}},
			},
		},
		{
			Kind: BlueprintResourcePostgres, Name: "data", SourcePath: "#/databases/0",
			Fields: map[string]BlueprintField{"name": {Value: "data"}, "plan": {Value: "basic-256mb"}},
		},
		{
			Kind: BlueprintResourceKeyValue, Name: "cache", SourcePath: "#/services/1",
			Fields: map[string]BlueprintField{"name": {Value: "cache"}},
		},
	}}
	resolver := fakeBlueprintStateResolver{rows: map[string]BlueprintCurrentResource{
		string(BlueprintResourceService) + ":api": {
			ID: "srv-api",
			Fields: map[string]any{
				"name": "api", "plan": "free", "envVars": []any{map[string]any{"key": "TOKEN", "value": "different-secret"}},
			},
		},
		string(BlueprintResourcePostgres) + ":data": {ID: "dpg-data", Fields: map[string]any{"name": "data", "plan": "basic-256mb"}},
	}}
	plan, err := PlanBlueprintIR(context.Background(), ir, resolver)
	if err != nil {
		t.Fatalf("PlanBlueprintIR(): %v", err)
	}
	if got, want := len(plan.Actions), 3; got != want {
		t.Fatalf("action count = %d, want %d", got, want)
	}
	if got := plan.Actions[0]; got.Kind != BlueprintResourcePostgres || got.Operation != BlueprintPlanNoop {
		t.Fatalf("first action = %+v, want postgres noop", got)
	}
	if got := plan.Actions[1]; got.Kind != BlueprintResourceKeyValue || got.Operation != BlueprintPlanCreate {
		t.Fatalf("second action = %+v, want key value create", got)
	}
	api := plan.Actions[2]
	if api.Kind != BlueprintResourceService || api.Operation != BlueprintPlanUpdate || api.ResourceID != "srv-api" {
		t.Fatalf("service action = %+v, want service update", api)
	}
	if got, want := api.ChangedFields, []BlueprintFieldChange{{Path: "envVars"}, {Path: "plan"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("changed fields = %+v, want %+v", got, want)
	}
	if strings.Contains(fmt.Sprintf("%+v", plan), "secret-not-in-plan") || strings.Contains(fmt.Sprintf("%+v", plan), "different-secret") {
		t.Fatalf("plan leaked a secret: %+v", plan)
	}
}

type fakeBlueprintStateResolver struct {
	rows map[string]BlueprintCurrentResource
}

func (f fakeBlueprintStateResolver) ResolveBlueprintResource(_ context.Context, kind BlueprintResourceKind, name string) (BlueprintCurrentResource, bool, error) {
	row, ok := f.rows[string(kind)+":"+name]
	return row, ok, nil
}
