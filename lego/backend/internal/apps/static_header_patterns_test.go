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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/graphql-go/graphql"
)

func TestStaticHeaderWildcardPersistenceAcrossAdapters(t *testing.T) {
	var headers []StaticHeaderView
	for i, path := range []string{"/*", "/blog/*", "/**/*", "/*.css", "/**/*.css"} {
		headers = append(headers, StaticHeaderView{Path: path, Name: fmt.Sprintf("X-Pattern-%d", i), Value: "marker"})
	}
	assertHeaders := func(t *testing.T, value any) {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var got []StaticHeaderView
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, headers) {
			t.Fatalf("headers=%+v, want %+v", got, headers)
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
				body, _ := json.Marshal(headers)
				for _, method := range []string{http.MethodPut, http.MethodGet} {
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, httptest.NewRequest(method, "/v1/services/site/headers", strings.NewReader(string(body))))
					if rec.Code != http.StatusOK {
						t.Fatalf("%s: %d %s", method, rec.Code, rec.Body.String())
					}
					var got any
					if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
						t.Fatal(err)
					}
					assertHeaders(t, unwrapRESTRules(t, method, got, "header"))
				}
			case "GraphQL":
				schema := blueprintSchema(t, svc)
				var inputs []string
				for _, h := range headers {
					inputs = append(inputs, fmt.Sprintf(`{path:%q,name:%q,value:%q}`, h.Path, h.Name, h.Value))
				}
				query := fmt.Sprintf(`mutation { setStaticHeaders(id:"site",headers:[%s]) { headers { path name value } } }`, strings.Join(inputs, ","))
				res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
				if len(res.Errors) > 0 {
					t.Fatal(res.Errors)
				}
				assertHeaders(t, res.Data.(map[string]any)["setStaticHeaders"].(map[string]any)["headers"])
				for _, alias := range []string{"service", "server"} {
					res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: fmt.Sprintf(`{ %s(id:"site") { headers { path name value } } }`, alias)})
					if len(res.Errors) > 0 {
						t.Fatal(res.Errors)
					}
					assertHeaders(t, res.Data.(map[string]any)[alias].(map[string]any)["headers"])
				}
			case "MCP":
				call, cleanup := appsMCPClient(t, svc)
				defer cleanup()
				result := call("update_static_headers", map[string]any{"serviceId": "site", "headers": headers})
				assertHeaders(t, result["headers"])
				assertHeaders(t, call("list_static_headers", map[string]any{"serviceId": "site"})["headers"])
			case "create":
				created, err := svc.Create(context.Background(), CreateRequest{Name: "new-site", Type: "static_site", Repo: "https://github.com/acme/site", PublishPath: "dist", Headers: headers})
				if err != nil {
					t.Fatal(err)
				}
				assertHeaders(t, created.Headers)
				assertHeaders(t, staticHeaderViews(getApp(t, cl, "new-site").Spec.Headers))
				return
			case "Blueprint":
				manifest := "services:\n  - name: site\n    type: web\n    runtime: static\n    repo: https://github.com/acme/site\n    staticPublishPath: dist\n    headers:\n"
				for _, h := range headers {
					manifest += fmt.Sprintf("      - path: %q\n        name: %q\n        value: %q\n", h.Path, h.Name, h.Value)
				}
				stack := parseBlueprintStackForTest(t, manifest)
				parsed := stack.services[0]
				if _, err := svc.applyBlueprintCreate(context.Background(), parsed.req, parsed.fields); err != nil {
					t.Fatal(err)
				}
				assertHeaders(t, staticHeaderViews(getApp(t, cl, "site").Spec.Headers))
				return // Other declared Blueprint fields may legitimately redeploy.
			}
			saved := getApp(t, cl, "site")
			assertHeaders(t, staticHeaderViews(saved.Spec.Headers))
			if saved.Spec.RestartedAt != before {
				t.Fatal("header-only update triggered republish")
			}
		})
	}
}

func TestStaticHeaderOperationsRejectOtherAppTypes(t *testing.T) {
	for _, kind := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob} {
		t.Run(kind, func(t *testing.T) {
			app := sampleApp("service")
			app.Spec.Type = kind
			svc, cl := newService(nil, app)
			before := getApp(t, cl, "service").Spec
			if _, err := svc.SetHeaders(context.Background(), "service", []StaticHeaderView{{Path: "/**/*.css", Name: "X-Pattern", Value: "marker"}}); !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("SetHeaders=%v, want bad request", err)
			}
			if _, err := svc.ListHeaders(context.Background(), "service"); !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("ListHeaders=%v, want bad request", err)
			}
			if after := getApp(t, cl, "service").Spec; !reflect.DeepEqual(before, after) {
				t.Fatal("rejected header operation mutated service")
			}
		})
	}
}
