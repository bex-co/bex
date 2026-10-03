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
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestStaticHeaderLiteralOrderedContractAcrossAdapters(t *testing.T) {
	headers := []StaticHeaderView{
		{Path: "/*.css", Name: "X-Pattern", Value: " root "},
		{Path: "/**/*.css", Name: "X-Pattern", Value: "nested"},
		{Path: "/**/*", Name: "X-Nested", Value: "all"},
		{Path: "/blog/*", Name: "X-Prefix", Value: "prefix"},
		{Path: "/*", Name: "X-All", Value: "all"},
		{Path: "/literal[?]é.css", Name: "X-Literal", Value: "literal"},
	}
	assertHeaders := func(t *testing.T, got any) {
		t.Helper()
		raw, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var actual []StaticHeaderView
		if err := json.Unmarshal(raw, &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, headers) {
			t.Fatalf("headers changed: got %#v want %#v", actual, headers)
		}
	}
	for _, adapter := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(adapter, func(t *testing.T) {
			app := sampleStaticApp("site")
			app.Spec.RestartedAt = "original-release"
			svc, cl := newService(nil, app)
			raw, _ := json.Marshal(headers)
			switch adapter {
			case "REST":
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				for _, method := range []string{http.MethodPut, http.MethodGet} {
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, httptest.NewRequest(method, "/v1/services/site/headers", strings.NewReader(string(raw))))
					if rec.Code != http.StatusOK {
						t.Fatalf("%s: %d %s", method, rec.Code, rec.Body)
					}
					var got any
					if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
						t.Fatal(err)
					}
					assertHeaders(t, got)
				}
			case "GraphQL":
				schema := mustSchema(t, svc)
				var inputs any
				_ = json.Unmarshal(raw, &inputs)
				result := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
					RequestString:  `mutation($headers:[StaticHeaderInput]) { setStaticHeaders(id:"site",headers:$headers) { headers { path name value } } }`,
					VariableValues: map[string]any{"headers": inputs}})
				if len(result.Errors) > 0 {
					t.Fatal(result.Errors)
				}
				assertHeaders(t, result.Data.(map[string]any)["setStaticHeaders"].(map[string]any)["headers"])
				result = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{ service(id:"site"){headers{path name value}} server(id:"site"){headers{path name value}} }`})
				if len(result.Errors) > 0 {
					t.Fatal(result.Errors)
				}
				for _, alias := range []string{"service", "server"} {
					assertHeaders(t, result.Data.(map[string]any)[alias].(map[string]any)["headers"])
				}
			case "MCP":
				call, closeClient := appsMCPClient(t, svc)
				defer closeClient()
				assertHeaders(t, call("update_static_headers", map[string]any{"serviceId": "site", "headers": headers})["headers"])
				assertHeaders(t, call("list_static_headers", map[string]any{"serviceId": "site"})["headers"])
			}
			persisted := getApp(t, cl, "site")
			assertHeaders(t, staticHeaderViews(persisted.Spec.Headers))
			if persisted.Spec.RestartedAt != "original-release" {
				t.Fatal("header update requested republish")
			}
		})
	}
}

func TestStaticHeaderAuthorizationAndDeletionPreserveRules(t *testing.T) {
	for _, kind := range []string{"denied", "deleting"} {
		t.Run(kind, func(t *testing.T) {
			app := sampleStaticApp("site")
			want := core.ErrForbidden
			switch kind {
			case "denied":
				want = core.ErrForbidden
			case "deleting":
				app = staticSite(deletingApp("site"))
				want = core.ErrNotFound
			}
			svc, cl := newService(nil, app)
			if kind == "denied" {
				svc.Authz = &sshAuthzRecorder{allow: false}
			}
			ctx := ctxAs("user-a")
			if _, err := svc.SetHeaders(ctx, "site", []StaticHeaderView{{Path: "/*.css", Name: "X-New", Value: "new"}}); !errors.Is(err, want) {
				t.Fatalf("SetHeaders=%v want %v", err, want)
			}
			if _, err := svc.ListHeaders(ctx, "site"); !errors.Is(err, want) {
				t.Fatalf("ListHeaders=%v want %v", err, want)
			}
			if !reflect.DeepEqual(getApp(t, cl, "site").Spec.Headers, app.Spec.Headers) {
				t.Fatal("refused mutation changed headers")
			}
		})
	}
}
