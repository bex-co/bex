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
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

func TestStaticRouteDestinationPersistenceAcrossAdapters(t *testing.T) {
	routes := []StaticRouteView{
		{Type: "redirect", Source: "/jump/*", Destination: "/docs/:splat?from=rule#frag"},
		{Type: "redirect", Source: "/star/*", Destination: "/docs/*"},
		{Type: "rewrite", Source: "/query", Destination: "/render.yaml?from=rule#frag"},
		{Type: "rewrite", Source: "/encoded", Destination: "/%72ender.yaml"},
		{Type: "rewrite", Source: "/copy/*", Destination: "/:splat"},
	}

	assertRoutes := func(t *testing.T, value any) {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var got []StaticRouteView
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, routes) {
			t.Fatalf("routes=%+v, want %+v", got, routes)
		}
	}
	for _, surface := range []string{"REST", "GraphQL", "MCP", "create", "Blueprint"} {
		t.Run(surface, func(t *testing.T) {
			svc, cl := newService(nil, sampleStaticApp("site"))
			before := getApp(t, cl, "site").Spec.RestartedAt
			switch surface {
			case "REST":
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				body, _ := json.Marshal(routes)
				for _, method := range []string{http.MethodPut, http.MethodGet} {
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, httptest.NewRequest(method, "/v1/services/site/routes", strings.NewReader(string(body))))
					if rec.Code != http.StatusOK {
						t.Fatalf("%s: %d %s", method, rec.Code, rec.Body.String())
					}
					var got any
					if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
						t.Fatal(err)
					}
					assertRoutes(t, unwrapRESTRules(t, method, got, "route"))
				}
			case "GraphQL":
				schema := blueprintSchema(t, svc)
				var inputs []string
				for _, h := range routes {
					inputs = append(inputs, fmt.Sprintf(`{type:%q,source:%q,destination:%q}`, h.Type, h.Source, h.Destination))
				}
				query := fmt.Sprintf(`mutation { setStaticRoutes(id:"site",routes:[%s]) { routes { type source destination } } }`, strings.Join(inputs, ","))
				res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
				if len(res.Errors) > 0 {
					t.Fatal(res.Errors)
				}
				assertRoutes(t, res.Data.(map[string]any)["setStaticRoutes"].(map[string]any)["routes"])
				for _, alias := range []string{"service", "server"} {
					res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: fmt.Sprintf(`{ %s(id:"site") { routes { type source destination } } }`, alias)})
					if len(res.Errors) > 0 {
						t.Fatal(res.Errors)
					}
					assertRoutes(t, res.Data.(map[string]any)[alias].(map[string]any)["routes"])
				}
			case "MCP":
				call, cleanup := appsMCPClient(t, svc)
				defer cleanup()
				result := call("update_static_routes", map[string]any{"serviceId": "site", "routes": routes})
				assertRoutes(t, result["routes"])
				assertRoutes(t, call("list_static_routes", map[string]any{"serviceId": "site"})["routes"])
			case "create":
				created, err := svc.Create(context.Background(), CreateRequest{Name: "new-site", Type: "static_site", Repo: "https://github.com/acme/site", PublishPath: "dist", Routes: routes})
				if err != nil {
					t.Fatal(err)
				}
				assertRoutes(t, created.Routes)
				assertRoutes(t, staticRouteViews(getApp(t, cl, "new-site").Spec.Routes))
				return
			case "Blueprint":
				manifest := "services:\n  - name: site\n    type: web\n    runtime: static\n    repo: https://github.com/acme/site\n    staticPublishPath: dist\n    routes:\n"
				for _, h := range routes {
					manifest += fmt.Sprintf("      - type: %q\n        source: %q\n        destination: %q\n", h.Type, h.Source, h.Destination)
				}
				stack := parseBlueprintStackForTest(t, manifest)
				parsed := stack.services[0]
				if _, err := applyServiceForTest(context.Background(), t, svc, parsed.req, parsed.fields); err != nil {
					t.Fatal(err)
				}
				assertRoutes(t, staticRouteViews(getApp(t, cl, "site").Spec.Routes))
				return // Other declared Blueprint fields may legitimately redeploy.
			}
			saved := getApp(t, cl, "site")
			assertRoutes(t, staticRouteViews(saved.Spec.Routes))
			if saved.Spec.RestartedAt != before {
				t.Fatal("route-only update triggered republish")
			}
		})
	}
}

// unwrapRESTRules strips Render's list envelope ({route|header, cursor}) from a
// REST GET so a cross-adapter assertion compares the rules themselves; PUT
// already answers bare id-bearing objects. Each cursor must equal its rule id.
func unwrapRESTRules(t *testing.T, method string, got any, key string) any {
	t.Helper()
	if method != http.MethodGet {
		return got
	}
	items, _ := got.([]any)
	out := make([]any, len(items))
	for i, item := range items {
		envelope, _ := item.(map[string]any)
		rule, _ := envelope[key].(map[string]any)
		if rule == nil || envelope["cursor"] == nil || envelope["cursor"] != rule["id"] {
			t.Fatalf("item %d = %v, want {%s: {id…}, cursor: <id>}", i, item, key)
		}
		out[i] = rule
	}
	return out
}
