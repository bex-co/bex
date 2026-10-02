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

package deploys

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Invalid source selectors and image references must fail before allocating a
// release. Every adapter must preserve the useful error rather than flattening
// registry policy failures into a misleading syntax error.
func TestTriggerInputRefusalsAcrossAdapters(t *testing.T) {
	for _, tc := range []struct {
		name, field, value, want string
	}{
		{"commit on image", "commitId", "deadbeef", "commitId is not supported for image-backed services"},
		{"uppercase repository", "imageUrl", "ghcr.io/ACME/web:v1", "must be lowercase"},
		{"short digest", "imageUrl", "nginx@sha256:deadbeef", "image digest is invalid"},
		{"private registry", "imageUrl", "127.0.0.1:5000/web:v1", "private or reserved"},
		{"untrusted registry", "imageUrl", "registry.example.com/web:v1", "not trusted"},
	} {
		for _, adapter := range []string{"REST", "GraphQL", "MCP", "deploy hook"} {
			t.Run(tc.name+"/"+adapter, func(t *testing.T) {
				ds := newFakeStore()
				svc, cl := newService(ds, sampleApp("web", "srv-1"))
				var hookURL string
				if adapter == "deploy hook" {
					hook, err := svc.GetDeployHook(context.Background(), "web")
					if err != nil {
						t.Fatal(err)
					}
					hookURL = hook.URL
				}
				before := getApp(t, cl, "web")
				var message string
				switch adapter {
				case "REST":
					body, err := json.Marshal(map[string]string{tc.field: tc.value})
					if err != nil {
						t.Fatal(err)
					}
					mux := http.NewServeMux()
					svc.RegisterREST(mux)
					req := httptest.NewRequest(http.MethodPost, "/v1/services/web/deploys", strings.NewReader(string(body)))
					req.Header.Set("Content-Type", "application/json")
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, req)
					if rec.Code != http.StatusBadRequest {
						t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
					}
					message = rec.Body.String()
				case "deploy hook":
					field := "imgURL"
					if tc.field == "commitId" {
						field = "ref"
					}
					req := httptest.NewRequest(http.MethodPost, hookURL+"&"+field+"="+url.QueryEscape(tc.value), nil)
					rec := httptest.NewRecorder()
					svc.DeployHookHandler().ServeHTTP(rec, req)
					if rec.Code != http.StatusBadRequest {
						t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
					}
					message = rec.Body.String()
				case "GraphQL":
					schema, err := graphql.NewSchema(graphql.SchemaConfig{
						Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
						Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
					})
					if err != nil {
						t.Fatal(err)
					}
					res := graphql.Do(graphql.Params{
						Schema: schema, Context: context.Background(),
						RequestString:  `mutation($value: String!) { triggerDeploy(serviceId: "web", ` + tc.field + `: $value) { id } }`,
						VariableValues: map[string]any{"value": tc.value},
					})
					if len(res.Errors) != 1 {
						t.Fatalf("errors = %v, data = %v", res.Errors, res.Data)
					}
					message = res.Errors[0].Message
				case "MCP":
					res, err := newMCPSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{
						Name: "trigger_deploy", Arguments: map[string]any{"serviceId": "web", tc.field: tc.value},
					})
					if err != nil {
						t.Fatal(err)
					}
					if !res.IsError {
						t.Fatalf("expected tool error, got %+v", res)
					}
					for _, content := range res.Content {
						if text, ok := content.(*mcp.TextContent); ok {
							message += text.Text
						}
					}
				}
				if !strings.Contains(message, tc.want) {
					t.Fatalf("error = %q, want %q", message, tc.want)
				}
				if after := getApp(t, cl, "web"); !reflect.DeepEqual(before, after) {
					t.Fatal("refused input changed the App")
				}
				if ds.nextID != 0 || len(ds.byApp) != 0 || len(ds.setImage) != 0 || len(ds.facts) != 0 {
					t.Fatal("refused input changed deploy history or saved settings")
				}
			})
		}
	}
}

func TestTriggerAcceptsImageDigestWithoutChangingSavedImage(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, sampleApp("web", "srv-1"))
	image := "ghcr.io/acme/web:v2@sha256:" + strings.Repeat("a", 64)
	deploy, err := svc.Trigger(context.Background(), "web", TriggerParams{ImageURL: image})
	if err != nil {
		t.Fatal(err)
	}
	if deploy.Image != image || ds.nextID != 1 {
		t.Fatalf("deploy image = %q, new deploys = %d", deploy.Image, ds.nextID)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "web:v1" || len(ds.setImage) != 0 {
		t.Fatalf("override changed saved image: spec = %q, store writes = %v", got, ds.setImage)
	}
}
