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
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

func TestMalformedStaticRouteRefusedAcrossAdapters(t *testing.T) {
	for _, adapter := range []string{"REST", "GraphQL", "MCP"} {
		t.Run(adapter, func(t *testing.T) {
			app := sampleStaticApp("site")
			svc, cl := newService(nil, app)
			switch adapter {
			case "REST":
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/v1/services/site/routes", strings.NewReader(`[{"type":"rewrite","source":"/old","destination":"/bad%"}]`)))
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status=%d %s", rec.Code, rec.Body)
				}
			case "GraphQL":
				res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: context.Background(), RequestString: `mutation { setStaticRoutes(id:"site", routes:[{type:"rewrite",source:"/old",destination:"/bad%"}]){id} }`})
				if len(res.Errors) == 0 {
					t.Fatal("malformed route accepted")
				}
			case "MCP":
				message := callAppsMCPError(t, svc, "update_static_routes", map[string]any{"serviceId": "site", "routes": []StaticRouteView{{Type: "rewrite", Source: "/old", Destination: "/bad%"}}})
				if !strings.Contains(message, "valid local URL") {
					t.Fatalf("unexpected error:%s", message)
				}
			}
			if !reflect.DeepEqual(getApp(t, cl, "site").Spec.Routes, app.Spec.Routes) {
				t.Fatal("refused mutation changed routes")
			}
		})
	}
}

func TestStaticRouteURLBlueprintReapply(t *testing.T) {
	svc, cl := newService(nil)
	apply := func(destination string) error {
		raw, err := json.Marshal([]StaticRouteView{{Type: "rewrite", Source: "/old", Destination: destination}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.DeployStack(context.Background(), DeployRequest{Manifest: "services:\n- name: site\n  type: web\n  runtime: static\n  repo: https://github.com/acme/site\n  staticPublishPath: dist\n  routes: " + string(raw) + "\n"})
		return err
	}
	for _, destination := range []string{"/initial.yaml", "/%72ender.yaml?from=updated#fragment"} {
		if err := apply(destination); err != nil {
			t.Fatal(err)
		}
		want := []StaticRouteView{{Type: "rewrite", Source: "/old", Destination: destination}}
		if got := staticRouteViews(getApp(t, cl, "site").Spec.Routes); !reflect.DeepEqual(got, want) {
			t.Fatalf("Blueprint routes=%#v, want %#v", got, want)
		}
	}
	before := getApp(t, cl, "site")
	if err := apply("/broken?escape=%"); err == nil {
		t.Fatal("malformed Blueprint update accepted")
	}
	after := getApp(t, cl, "site")
	if !reflect.DeepEqual(after.Spec, before.Spec) || after.ResourceVersion != before.ResourceVersion {
		t.Fatal("refused Blueprint update mutated the persisted App")
	}
}
