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
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/graphql-go/graphql"
)

// Save only, cancellation and historical selection can leave saved configuration
// different from the serving release. The operator owns the fact; the API must
// carry it without treating the notification metadata itself as divergence.

func TestViewCarriesUndeployedChanges(t *testing.T) {
	a := sampleApp("web")
	if view(a).UndeployedChanges {
		t.Fatal("an ordinary service reports undeployed changes")
	}
	a.Status.UndeployedChanges = true
	if !view(a).UndeployedChanges {
		t.Fatal("the operator's undeployedChanges did not reach the view")
	}
}

// REST and MCP share renderService. The field is a bex extension, so it must be
// absent — not `false` — on an ordinary service: a Render client then sees nothing
// it did not ask for.
func TestRenderServiceOmitsUndeployedChangesUnlessSet(t *testing.T) {
	a := sampleApp("web")
	raw, err := json.Marshal(toRenderService(view(a)))
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	_ = json.Unmarshal(raw, &wire)
	if _, present := wire["undeployedChanges"]; present {
		t.Errorf("ordinary service carries undeployedChanges on the wire: %s", raw)
	}

	a.Status.UndeployedChanges = true
	raw, _ = json.Marshal(toRenderService(view(a)))
	wire = map[string]any{}
	_ = json.Unmarshal(raw, &wire)
	if wire["undeployedChanges"] != true {
		t.Errorf("undeployedChanges = %v on the wire, want true: %s", wire["undeployedChanges"], raw)
	}
}

func TestServiceReadsCarryAuthoritativeUndeployedChanges(t *testing.T) {
	for _, pending := range []bool{false, true} {
		name := "saved matches serving release"
		if pending {
			name = "saved differs from serving release"
		}
		t.Run(name, func(t *testing.T) {
			a := sampleApp("web")
			a.Status.UndeployedChanges = pending
			a.Annotations = map[string]string{appv1alpha1.AnnotationSavedConfigRevision: "opaque-notification"}
			svc, _ := newService(nil, a)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/services/web", nil))
			if response.Code != http.StatusOK {
				t.Fatalf("REST read: %d %s", response.Code, response.Body.String())
			}
			var rest map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &rest); err != nil {
				t.Fatal(err)
			}
			call, cleanup := appsMCPClient(t, svc)
			defer cleanup()
			mcp := call("get_service", map[string]any{"serviceId": "web"})
			for surface, result := range map[string]map[string]any{"REST": rest, "MCP": mcp} {
				got, exists := result["undeployedChanges"]
				if pending && got != true || !pending && exists {
					t.Errorf("%s undeployedChanges = %v (present %t), pending %t", surface, got, exists, pending)
				}
			}
			res := graphql.Do(graphql.Params{
				Schema:        mustSchema(t, svc),
				Context:       context.Background(),
				RequestString: `{ service(id:"web") { undeployedChanges } server(id:"web") { undeployedChanges } }`,
			})
			if len(res.Errors) > 0 {
				t.Fatalf("GraphQL errors: %v", res.Errors)
			}
			for _, alias := range []string{"service", "server"} {
				got := res.Data.(map[string]any)[alias].(map[string]any)["undeployedChanges"]
				if got != pending {
					t.Errorf("GraphQL %s undeployedChanges = %v, want %t", alias, got, pending)
				}
			}
		})
	}
}
