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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRESTImageCreateAutoDeploy(t *testing.T) {
	for _, tc := range []struct {
		field  string
		status int
	}{
		{"", 201}, {`,"autoDeploy":"no"`, 201}, {`,"autoDeploy":false`, 400},
		{`,"autoDeploy":"yes"`, 400}, {`,"autoDeploy":true`, 400},
		{`,"autoDeploy":"invalid"`, 400}, {`,"autoDeployTrigger":"checksPass"`, 400},
	} {
		t.Run(tc.field, func(t *testing.T) {
			svc, cl := newService(nil)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			body := `{"name":"image","ownerId":"default","type":"web_service","envVars":[{"key":"WHOAMI_PORT_NUMBER","value":"3000"},{"key":"QA_MARKER","value":"m99"}],"image":{"imagePath":"docker.io/traefik/whoami:v1.11.0","ownerId":""},"serviceDetails":{"healthCheckPath":"/","numInstances":1,"plan":"free","region":"frankfurt","runtime":"image"}` + tc.field + `}`
			rec := httptest.NewRecorder()
			validated(t, mux).ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
			if rec.Code != tc.status {
				t.Fatalf("status=%d want=%d: %s", rec.Code, tc.status, rec.Body)
			}
			if tc.status != 201 {
				return
			}
			app := getApp(t, cl, "image")
			if app.Spec.AutoDeploy {
				t.Fatal("image automation enabled")
			}
			rec = httptest.NewRecorder()
			validated(t, mux).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/image", nil))
			var result map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result["autoDeploy"] != "no" || result["autoDeployTrigger"] != "off" {
				t.Fatalf("disabled state lost: %s", rec.Body)
			}
		})
	}
}

func TestGraphQLImageCreateAutoDeploy(t *testing.T) {
	for _, value := range []string{"false", "true"} {
		svc, cl := newService(nil)
		schema := mustSchema(t, svc)
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { createService(name:"image",image:"nginx:1.27",autoDeploy:` + value + `) { autoDeploy autoDeployTrigger } }`})
		if value == "true" {
			if len(res.Errors) == 0 {
				t.Fatal("enabled image automation accepted")
			}
			continue
		}
		if len(res.Errors) != 0 {
			t.Fatalf("createService: %v", res.Errors)
		}
		if getApp(t, cl, "image").Spec.AutoDeploy {
			t.Fatal("image automation enabled")
		}
		got := res.Data.(map[string]any)["createService"].(map[string]any)
		if got["autoDeploy"] != false || got["autoDeployTrigger"] != "off" {
			t.Fatalf("disabled state lost: %+v", got)
		}
	}
}

func TestMCPImageCreateAutoDeploy(t *testing.T) {
	for _, tool := range []string{"create_web_service", "create_cron_job"} {
		t.Run(tool, func(t *testing.T) {
			svc, cl := newService(nil)
			call, cleanup := appsMCPClient(t, svc)
			defer cleanup()
			args := map[string]any{"name": "image", "image": "nginx:1.27", "autoDeploy": "no", "runtime": "image", "buildCommand": "", "startCommand": ""}
			if tool == "create_cron_job" {
				args["schedule"] = "* * * * *"
				args["command"] = "echo ok"
			}
			got := call(tool, args)
			if got["autoDeploy"] != "no" || got["autoDeployTrigger"] != "off" {
				t.Fatalf("disabled state lost: %+v", got)
			}
			name, _ := got["immutableName"].(string)
			if name == "" {
				name = "image"
			}
			if getApp(t, cl, name).Spec.AutoDeploy {
				t.Fatal("image automation enabled")
			}
		})
	}
}

func TestMCPImageCreateRejectsEnabledAndInvalidAutoDeploy(t *testing.T) {
	for _, tool := range []string{"create_web_service", "create_cron_job"} {
		for _, value := range []string{"yes", "invalid"} {
			t.Run(tool+"/"+value, func(t *testing.T) {
				svc, _ := newService(nil)
				srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
				svc.RegisterMCP(srv)
				serverT, clientT := mcp.NewInMemoryTransports()
				serverSession, err := srv.Connect(context.Background(), serverT, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer serverSession.Close()
				session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), clientT, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer session.Close()
				args := map[string]any{"name": "image", "image": "nginx:1.27", "autoDeploy": value, "runtime": "image", "buildCommand": "", "startCommand": ""}
				if tool == "create_cron_job" {
					args["schedule"] = "* * * * *"
				}
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
				if err != nil {
					t.Fatal(err)
				}
				if !result.IsError {
					t.Fatalf("%s was accepted: %+v", value, result)
				}
				body, _ := json.Marshal(result.Content)
				if !strings.Contains(string(body), "autoDeploy") {
					t.Fatalf("wrong failure: %s", body)
				}
			})
		}
	}
}

// TestDeployStackImageAutoDeployHasNoEffect: Render's Blueprint spec says
// autoDeployTrigger "has no effect for services that deploy a prebuilt Docker
// image", and commit is "Equivalent to the deprecated setting autoDeploy:
// true". Every spelling plans a create, applies with Auto-Deploy off, and a
// re-plan under any of them is a no-op (w8/405).
func TestDeployStackImageAutoDeployHasNoEffect(t *testing.T) {
	fields := []string{"autoDeploy: false", "autoDeployTrigger: off", "autoDeploy: true",
		"autoDeployTrigger: commit", "autoDeployTrigger: checksPass"}
	manifest := func(field string) string {
		return "services:\n  - name: image\n    type: web\n    runtime: image\n    image: {url: nginx:1.27}\n    " + field + "\n"
	}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			svc, cl := newService(nil)
			raw := manifest(field)
			validation, err := svc.ValidateBlueprint(context.Background(), "", raw, "")
			if err != nil || !validation.Valid || validation.Plan == nil || len(validation.Plan.Actions) != 1 ||
				validation.Plan.Actions[0].Operation != BlueprintPlanCreate {
				t.Fatalf("new-image plan: %+v, %v", validation, err)
			}
			if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: raw}); err != nil {
				t.Fatal(err)
			}
			if spec := getApp(t, cl, "image").Spec; spec.AutoDeploy || spec.AutoDeployTrigger != "" {
				t.Fatalf("Blueprint gave the image Auto-Deploy: autoDeploy=%v trigger=%q", spec.AutoDeploy, spec.AutoDeployTrigger)
			}
			for _, next := range fields {
				validation, err := svc.ValidateBlueprint(context.Background(), "", manifest(next), "")
				if err != nil || !validation.Valid || validation.Plan == nil || len(validation.Plan.Actions) != 1 {
					t.Fatalf("existing-image plan with %q: %+v, %v", next, validation, err)
				}
				if validation.Plan.Actions[0].Operation != BlueprintPlanNoop {
					t.Fatalf("image plan with %q was not a no-op: %+v", next, validation.Plan)
				}
			}
			if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: raw}); err != nil {
				t.Fatalf("reapply image: %v", err)
			}
			if getApp(t, cl, "image").Spec.AutoDeploy {
				t.Fatal("reapply gave the image Auto-Deploy")
			}
		})
	}
}

// TestValidateBlueprintImageAutoDeployOnEverySurface is w8/405's repro on
// REST (what `blueprints validate` posts), GraphQL and MCP: every enabled
// Auto-Deploy spelling beside an image validates, while an invalid enum and
// the build inputs Render does not call no-ops stay refused at their paths.
func TestValidateBlueprintImageAutoDeployOnEverySurface(t *testing.T) {
	for _, tc := range []struct{ field, refusedAt string }{
		{"autoDeployTrigger: checksPass", ""},
		{"autoDeployTrigger: commit", ""},
		{"autoDeploy: true", ""},
		{"autoDeployTrigger: bogus", "services[0].autoDeployTrigger"},
		{"buildCommand: make", "services[0].buildCommand"},
		{"repo: https://github.com/render-examples/express-hello-world", "services[0].repo"},
		{"dockerfilePath: ./Dockerfile", "services[0].dockerfilePath"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			manifest := "services:\n  - type: web\n    name: qa-w\n    runtime: image\n    plan: free\n    image:\n      url: docker.io/library/nginx:1.27\n    " + tc.field + "\n"
			check := func(surface string, valid any, errors []any) {
				t.Helper()
				if tc.refusedAt == "" {
					if valid != true || len(errors) != 0 {
						t.Fatalf("%s: valid=%v errors=%+v", surface, valid, errors)
					}
					return
				}
				for _, e := range errors {
					if e.(map[string]any)["path"] == tc.refusedAt {
						return
					}
				}
				t.Fatalf("%s: no refusal at %s: valid=%v errors=%+v", surface, tc.refusedAt, valid, errors)
			}

			svc, _ := newService(nil)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			body, _ := json.Marshal(map[string]string{"bexYaml": manifest})
			validated(t, mux).ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/validate", strings.NewReader(string(body))))
			var rest struct {
				Valid  any   `json:"valid"`
				Errors []any `json:"errors"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
				t.Fatalf("REST unmarshal: %v (%s)", err, rec.Body)
			}
			check("REST", rest.Valid, rest.Errors)

			res := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: context.Background(),
				RequestString:  `query($y: String!) { validateBlueprint(bexYaml: $y) { valid errorDetails { path } } }`,
				VariableValues: map[string]any{"y": manifest}})
			if len(res.Errors) > 0 {
				t.Fatalf("GraphQL: %v", res.Errors)
			}
			got := res.Data.(map[string]any)["validateBlueprint"].(map[string]any)
			details, _ := got["errorDetails"].([]any)
			check("GraphQL", got["valid"], details)

			call, cleanup := appsMCPClient(t, svc)
			defer cleanup()
			out := call("validate_bex_yml", map[string]any{"bexYaml": manifest})
			mcpErrors, _ := out["errors"].([]any)
			check("MCP", out["valid"], mcpErrors)
		})
	}
}
