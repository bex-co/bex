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
			mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
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
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/image", nil))
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

func TestDeployStackDisabledImageAutoDeploy(t *testing.T) {
	for _, field := range []string{"autoDeploy: false", "autoDeployTrigger: off"} {
		svc, cl := newService(nil)
		raw := "services:\n  - name: image\n    type: web\n    runtime: image\n    image: {url: nginx:1.27}\n    " + field + "\n"
		if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: raw}); err != nil {
			t.Fatal(err)
		}
		if getApp(t, cl, "image").Spec.AutoDeploy {
			t.Fatal("Blueprint enabled image automation")
		}
		validation, err := svc.ValidateBlueprint(context.Background(), "", raw)
		if err != nil || !validation.Valid || validation.Plan == nil || len(validation.Plan.Actions) != 1 {
			t.Fatalf("existing-image plan: %+v, %v", validation, err)
		}
		if validation.Plan.Actions[0].Operation != BlueprintPlanNoop {
			t.Fatalf("disabled image plan was not a no-op: %+v", validation.Plan)
		}
		if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: raw}); err != nil {
			t.Fatalf("reapply disabled image: %v", err)
		}
	}
}
