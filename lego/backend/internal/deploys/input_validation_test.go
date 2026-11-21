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
		{"internal registry", "imageUrl", "registry.default.svc/web:v1", "not a public registry"},
		{"other repository", "imageUrl", "docker.io/library/nginx:1.27", "configured image docker.io/library/web (got docker.io/library/nginx)"},
		{"other registry", "imageUrl", "ghcr.io/library/web:v1", "configured image docker.io/library/web (got ghcr.io/library/web)"},
		{"malformed reference", "imageUrl", "docker.io/library/web:not a tag", "image reference is malformed"},
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
				if adapter == "deploy hook" {
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
				} else {
					var ok bool
					if ok, message = triggerVia(t, svc, adapter, map[string]string{tc.field: tc.value}); ok {
						t.Fatal("refused input was accepted")
					}
				}
				want := tc.want
				if adapter == "deploy hook" {
					// The hook names its own ref/imgURL parameters (w8/141).
					want = hookFieldNames.Replace(want)
					if strings.Contains(message, "commitId") || strings.Contains(message, "imageUrl") {
						t.Fatalf("hook error = %q names a REST body field the caller never sent", message)
					}
				}
				if !strings.Contains(message, want) {
					t.Fatalf("error = %q, want %q", message, want)
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
	app := sampleApp("web", "srv-1")
	app.Spec.Image = "ghcr.io/acme/web:v1"
	svc, cl := newService(ds, app)
	image := "ghcr.io/acme/web:v2@sha256:" + strings.Repeat("a", 64)
	deploy, err := svc.Trigger(context.Background(), "web", TriggerParams{ImageURL: image})
	if err != nil {
		t.Fatal(err)
	}
	if deploy.Image != image || ds.nextID != 1 {
		t.Fatalf("deploy image = %q, new deploys = %d", deploy.Image, ds.nextID)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "ghcr.io/acme/web:v1" || len(ds.setImage) != 0 {
		t.Fatalf("override changed saved image: spec = %q, store writes = %v", got, ds.setImage)
	}
}

// triggerVia fires one trigger with args through REST, GraphQL or MCP and
// reports whether it was accepted, with the refusal's message otherwise. A
// clearCache value reaches MCP as Render's MCP boolean.
func triggerVia(t *testing.T, svc *Service, adapter string, args map[string]string) (bool, string) {
	t.Helper()
	switch adapter {
	case "REST":
		body, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		req := httptest.NewRequest(http.MethodPost, "/v1/services/web/deploys", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		switch rec.Code {
		case http.StatusCreated:
			return true, ""
		case http.StatusBadRequest:
			var refusal struct{ Message string }
			if err := json.Unmarshal(rec.Body.Bytes(), &refusal); err != nil {
				t.Fatalf("REST refusal %s: %v", rec.Body, err)
			}
			return false, refusal.Message
		}
		t.Fatalf("REST status = %d, body = %s", rec.Code, rec.Body)
	case "GraphQL":
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		var params, fields []string
		vars := map[string]any{}
		for k, v := range args {
			params = append(params, "$"+k+": String")
			fields = append(fields, k+": $"+k)
			vars[k] = v
		}
		res := graphql.Do(graphql.Params{
			Schema: schema, Context: context.Background(), VariableValues: vars,
			RequestString: `mutation(` + strings.Join(params, ", ") + `) { triggerDeploy(serviceId: "web", ` + strings.Join(fields, ", ") + `) { id } }`,
		})
		switch len(res.Errors) {
		case 0:
			return true, ""
		case 1:
			return false, res.Errors[0].Message
		}
		t.Fatalf("GraphQL errors = %v", res.Errors)
	case "MCP":
		in := map[string]any{"serviceId": "web"}
		for k, v := range args {
			in[k] = v
		}
		if v, ok := args["clearCache"]; ok {
			in["clearCache"] = v == "clear"
		}
		res, err := newMCPSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "trigger_deploy", Arguments: in})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			return true, ""
		}
		var message string
		for _, content := range res.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				message += text.Text
			}
		}
		return false, message
	}
	t.Fatalf("unknown adapter %q", adapter)
	return false, ""
}

// Render's create-deploy contract: "deploy_only cannot be combined with
// commitId, imageUrl, or clearCache" (w8/377). Each mix is a 400 on every
// adapter that takes deployMode, and opens no deploy or App write. A repo-backed
// service gets the same refusal, ahead of its own deploy_only refusal.
func TestTriggerRefusesDeployOnlyMixesAcrossAdapters(t *testing.T) {
	const want = `deployMode "deploy_only" cannot be combined with commitId, imageUrl, or clearCache`
	for _, tc := range []struct {
		name string
		repo bool
		args map[string]string
	}{
		{"imageUrl", false, map[string]string{"deployMode": "deploy_only", "imageUrl": "docker.io/library/web:v2"}},
		{"clearCache clear", false, map[string]string{"deployMode": "deploy_only", "clearCache": "clear"}},
		{"commitId", false, map[string]string{"deployMode": "deploy_only", "commitId": "deadbeef"}},
		{"commitId on repo-backed", true, map[string]string{"deployMode": "deploy_only", "commitId": "deadbeef"}},
	} {
		for _, adapter := range []string{"REST", "GraphQL", "MCP"} {
			t.Run(tc.name+"/"+adapter, func(t *testing.T) {
				ds := newFakeStore()
				app := sampleApp("web", "srv-1")
				if tc.repo {
					app.Spec.Image = ""
					app.Spec.Repo = "https://github.com/bex-co/hello.git"
				}
				svc, cl := newService(ds, app)
				before := getApp(t, cl, "web")
				ok, message := triggerVia(t, svc, adapter, tc.args)
				if ok {
					t.Fatal("deploy_only mix was accepted")
				}
				if !strings.Contains(message, want) {
					t.Fatalf("error = %q, want %q", message, want)
				}
				if after := getApp(t, cl, "web"); !reflect.DeepEqual(before, after) {
					t.Fatal("refused mix changed the App")
				}
				if ds.nextID != 0 || len(ds.byApp) != 0 {
					t.Fatal("refused mix opened a deploy")
				}
			})
		}
	}
}

// deploy_only alone, or with clearCache's default do_not_clear (which the
// pinned CLI sends on every create), still redeploys the configured image.
func TestTriggerAcceptsBareDeployOnlyAcrossAdapters(t *testing.T) {
	for _, args := range []map[string]string{
		{"deployMode": "deploy_only"},
		{"deployMode": "deploy_only", "clearCache": "do_not_clear"},
	} {
		for _, adapter := range []string{"REST", "GraphQL", "MCP"} {
			t.Run(adapter+"/"+args["clearCache"], func(t *testing.T) {
				ds := newFakeStore()
				svc, cl := newService(ds, sampleApp("web", "srv-1"))
				if ok, message := triggerVia(t, svc, adapter, args); !ok {
					t.Fatalf("bare deploy_only refused: %s", message)
				}
				if ds.nextID != 1 {
					t.Fatalf("deploys opened = %d, want 1", ds.nextID)
				}
				if got := getApp(t, cl, "web").Spec.Image; got != "web:v1" {
					t.Fatalf("image = %q, want the configured web:v1", got)
				}
			})
		}
	}
}
