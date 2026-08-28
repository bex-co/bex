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

// name_contract_test.go pins the settled service-name contract (w6/m115):
//
//   - `name` is Render's MUTABLE service.name — the display label when set,
//     else the immutable name — and REST, GraphQL and MCP all return the SAME
//     string for it. Before this milestone GraphQL alone returned the raw
//     immutable name, so the four read surfaces disagreed for any renamed
//     service (found live, 39th /qa-find-bugs run).
//   - `immutableName` carries the immutable, workspace-unique name on every read
//     surface, so the stable address is always recoverable from a read response.
//   - The immutable name (and the id) still address the service through
//     core.Base.GetApp; the mutable display label is deliberately NOT an address
//     (displayName is unvalidated and non-unique — only trimmed), matching
//     Render, which addresses by id alone.
//
// All five App-CR-backed types share AppView, so each is probed rather than
// sampled.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// renamedApp is a sampleApp of the given type whose display label differs from
// its immutable name — the exact shape the bug needs (create, then rename).
func renamedApp(name, svcType, displayName string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Type = svcType
	a.Spec.DisplayName = displayName
	return a
}

// readSurfaceName returns the `name` and `immutableName` each read surface
// reports for the service addressed by immutableName, over REST, GraphQL and
// MCP — the three read APIs that share renderService/AppView.
func readSurfaceNames(t *testing.T, svc *Service, immutableName string) (restName, restImmutable, gqlName, gqlImmutable, mcpName, mcpImmutable string) {
	t.Helper()
	ctx := context.Background()

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/"+immutableName, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST GET %s: %d %s", immutableName, rec.Code, rec.Body)
	}
	var restBody struct {
		Name          string `json:"name"`
		ImmutableName string `json:"immutableName"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
		t.Fatalf("decode REST body: %v", err)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ server(id: "` + immutableName + `") { name immutableName } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	server := res.Data.(map[string]any)["server"].(map[string]any)

	handler := svc.serviceTool(svc.Get)
	_, mcpService, err := handler(ctx, nil, serviceArgs{ServiceID: immutableName})
	if err != nil {
		t.Fatalf("MCP get_service: %v", err)
	}

	return restBody.Name, restBody.ImmutableName,
		str(server["name"]), str(server["immutableName"]),
		mcpService.Name, mcpService.ImmutableName
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// TestServiceNameAgreesAcrossReadSurfaces is the headline check: for a renamed
// service of EVERY App-CR-backed type, REST/GraphQL/MCP return the same `name`
// (the display label) and the same `immutableName` (the immutable name). This
// is exactly the cross-surface agreement that never had a test before — the
// drift survived w6/m101's parity pass because that pass checked no-regression,
// not agreement.
func TestServiceNameAgreesAcrossReadSurfaces(t *testing.T) {
	for _, svcType := range []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
		appv1alpha1.TypeStaticSite,
	} {
		t.Run(svcType, func(t *testing.T) {
			const immutable, label = "web", "renamed-label"
			svc, _ := newService(nil, renamedApp(immutable, svcType, label))

			restName, restImmutable, gqlName, gqlImmutable, mcpName, mcpImmutable :=
				readSurfaceNames(t, svc, immutable)

			// name == the mutable label on all three surfaces.
			for surface, got := range map[string]string{"REST": restName, "GraphQL": gqlName, "MCP": mcpName} {
				if got != label {
					t.Errorf("%s name = %q, want the display label %q", surface, got, label)
				}
			}
			// immutableName == the immutable name on all three surfaces.
			for surface, got := range map[string]string{"REST": restImmutable, "GraphQL": gqlImmutable, "MCP": mcpImmutable} {
				if got != immutable {
					t.Errorf("%s immutableName = %q, want %q", surface, got, immutable)
				}
			}
		})
	}
}

// TestControlServiceWithoutDisplayNameAgrees is the load-bearing control from
// the hunt: with no displayName, name == immutableName on every surface, so the
// milestone's fix does not change the common case.
func TestControlServiceWithoutDisplayNameAgrees(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	restName, restImmutable, gqlName, gqlImmutable, mcpName, mcpImmutable := readSurfaceNames(t, svc, "web")
	for _, got := range []string{restName, restImmutable, gqlName, gqlImmutable, mcpName, mcpImmutable} {
		if got != "web" {
			t.Errorf("no-displayName control: got %q, want every name/immutableName = web", got)
		}
	}
}

// TestImmutableNameAddressesButDisplayLabelDoesNot pins the addressing contract
// t004 settled: the immutable name a read surface hands back resolves through
// core.Base.GetApp; the mutable display label does not (it is unvalidated and
// non-unique, so it is not a stable address — the recoverable address is
// `immutableName`/`id`, both in the same read response).
func TestImmutableNameAddressesButDisplayLabelDoesNot(t *testing.T) {
	svc, _ := newService(nil, renamedApp("web", appv1alpha1.TypeWebService, "renamed-label"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// The immutable name round-trips: GET /v1/services/{immutableName} -> 200.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET by immutable name => 200, got %d: %s", rec.Code, rec.Body)
	}
	// The display label is not an address: GET /v1/services/{displayName} -> 404.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/renamed-label", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET by display label => 404 (not a stable address), got %d: %s", rec.Code, rec.Body)
	}

	// GraphQL server(id: immutableName) resolves; the value REST/GraphQL now
	// return as `name` is recoverable to an address via the immutableName field.
	v, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get by immutable name: %v", err)
	}
	if v.Name != "web" || renderServiceName(v) != "renamed-label" {
		t.Fatalf("Get returned name=%q label=%q, want web / renamed-label", v.Name, renderServiceName(v))
	}
}

// TestListFilterMatchesBothNameSpellings guards DoD bullet 4: the list `name=`
// filter keeps matching a service by EITHER its immutable name or its display
// label (rest.go accepts a.Name or renderServiceName(a)); the name-contract
// change must not narrow it.
func TestListFilterMatchesBothNameSpellings(t *testing.T) {
	svc, _ := newService(nil, renamedApp("web", appv1alpha1.TypeWebService, "renamed-label"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, spelling := range []string{"web", "renamed-label"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services?name="+spelling, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("list ?name=%s => 200, got %d: %s", spelling, rec.Code, rec.Body)
		}
		var page []struct {
			Service struct {
				ImmutableName string `json:"immutableName"`
			} `json:"service"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode list: %v", err)
		}
		if len(page) != 1 || page[0].Service.ImmutableName != "web" {
			t.Errorf("list ?name=%s matched %d services, want the one immutable=web", spelling, len(page))
		}
	}
}
