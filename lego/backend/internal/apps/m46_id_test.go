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

// m46_id_test.go pins the w1/m46 flip: GraphQL Service.id is the minted
// srv-… id (what REST/MCP already serve), verbs accept BOTH the srv- id and
// the App name (core.Base's LabelAppID-first / LabelServiceName-fallback
// resolution), and a legacy hand-applied CR without LabelAppID keeps its
// name as id.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func gqlDo(t *testing.T, schema graphql.Schema, query string) map[string]any {
	t.Helper()
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql errors for %q: %v", query, res.Errors)
	}
	return res.Data.(map[string]any)
}

// A store-managed App: carries the minted srv- id (LabelAppID) and its public
// name (LabelServiceName), like every App the create path stamps.
func mintedApp(name, srvID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{
		core.LabelAppID:       srvID,
		core.LabelServiceName: name,
	}
	return a
}

func TestGraphQLServiceIDIsMintedSrvID(t *testing.T) {
	svc, _ := newService(nil, mintedApp("web", "srv-m46test0000000000a"))
	schema := mustSchema(t, svc)

	// Fetch by the srv- id: id round-trips, name stays the display name.
	data := gqlDo(t, schema, `{ service(id:"srv-m46test0000000000a"){ id name } }`)
	got := data["service"].(map[string]any)
	if got["id"] != "srv-m46test0000000000a" || got["name"] != "web" {
		t.Errorf("service by srv- id = %+v, want id=srv-m46test0000000000a name=web", got)
	}

	// Fetch by NAME (pre-flip URLs, bookmarks): the fallback resolves the same
	// App and STILL returns the srv- id.
	data = gqlDo(t, schema, `{ service(id:"web"){ id name } }`)
	got = data["service"].(map[string]any)
	if got["id"] != "srv-m46test0000000000a" || got["name"] != "web" {
		t.Errorf("service by name = %+v, want the same srv- id", got)
	}
}

func TestGraphQLMutationAcceptsSrvID(t *testing.T) {
	svc, cl := newService(nil, mintedApp("web", "srv-m46test0000000000b"))
	schema := mustSchema(t, svc)

	gqlDo(t, schema, `mutation { suspendService(id:"srv-m46test0000000000b"){ id suspended } }`)

	a := getApp(t, cl, "web")
	if !a.Spec.Suspended {
		t.Error("suspend via srv- id did not reach the App CR")
	}
}

func TestGraphQLLegacyAppKeepsNameAsID(t *testing.T) {
	// A hand-applied CR: no LabelAppID, no LabelServiceName — publicID falls
	// back to the name, and name resolution finds it by metadata.name.
	svc, _ := newService(nil, sampleApp("legacy"))
	schema := mustSchema(t, svc)

	data := gqlDo(t, schema, `{ service(id:"legacy"){ id name } }`)
	got := data["service"].(map[string]any)
	if got["id"] != "legacy" || got["name"] != "legacy" {
		t.Errorf("legacy service = %+v, want name-as-id fallback", got)
	}
}

// TestServiceIDAgreesAcrossRESTGraphQLAndMCP is the w1/m46 parity pin: the
// three surfaces must emit the SAME id for the same service (REST/MCP already
// spoke srv-; the GraphQL flip must have joined them, not drifted them).
func TestServiceIDAgreesAcrossRESTGraphQLAndMCP(t *testing.T) {
	svc, _ := newService(nil, mintedApp("web", "srv-m46test0000000000d"))

	// REST
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/srv-m46test0000000000d", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST status = %d, body %s", rec.Code, rec.Body)
	}
	// The single-get returns a bare renderService (the {service, cursor}
	// wrapper is list-only) — decode it directly so a wrapper leaking into the
	// single-get would fail this pin instead of being silently tolerated.
	var out renderService
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("REST decode: %v (%s)", err, rec.Body)
	}
	restID := out.ID

	// GraphQL
	schema := mustSchema(t, svc)
	data := gqlDo(t, schema, `{ service(id:"srv-m46test0000000000d"){ id } }`)
	gqlID := data["service"].(map[string]any)["id"].(string)

	// MCP get_service round-trips the same view struct GraphQL/REST project.
	view, err := svc.Get(context.Background(), "srv-m46test0000000000d")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if restID != "srv-m46test0000000000d" || gqlID != restID || view.ID != restID {
		t.Errorf("id drift: rest=%q graphql=%q core=%q", restID, gqlID, view.ID)
	}
}

func TestGraphQLListEmitsSrvIDs(t *testing.T) {
	svc, _ := newService(nil, mintedApp("web", "srv-m46test0000000000c"), sampleApp("legacy"))
	schema := mustSchema(t, svc)

	data := gqlDo(t, schema, `{ services { id name } }`)
	byName := map[string]string{}
	for _, item := range data["services"].([]any) {
		m := item.(map[string]any)
		byName[m["name"].(string)] = m["id"].(string)
	}
	if byName["web"] != "srv-m46test0000000000c" {
		t.Errorf("minted app list id = %q, want srv-m46test0000000000c", byName["web"])
	}
	if byName["legacy"] != "legacy" {
		t.Errorf("legacy app list id = %q, want name fallback", byName["legacy"])
	}
}
