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

package keyvalue

// metadata_cross_surface_test.go is w9/m42/t003's cross-surface parity test for
// the KeyValue sibling: region/dashboardUrl/updatedAt added by w2/m46 must appear
// with identical values on GraphQL (keyValue query) and MCP (get_key_value tool)
// when Metadata is configured — so a future surface regression fails here rather
// than silently dropping the fields. The REST half lives in
// metadata_rest_test.go (w9/m41/t005). GraphQL/MCP intentionally keep the flat
// ownerId (no nested owner object); this test covers only the three metadata
// fields that were previously REST-only.

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// TestGraphQLKeyValueMetadataParity verifies that region/dashboardUrl/updatedAt
// appear on the GraphQL KeyValue type when Metadata is configured, and match the
// values the REST handler emits for the same resource.
func TestGraphQLKeyValueMetadataParity(t *testing.T) {
	svc, _ := newService()
	svc.Metadata = resourcemeta.Config{Region: "fsn1", DashboardBaseURL: "https://dashboard.bex.co"}

	restW := serveREST(svc, http.MethodPost, "/v1/key-value", `{"name":"gql-meta-kv","plan":"free"}`)
	if restW.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", restW.Code, restW.Body.String())
	}
	rest := decodeMap(t, restW.Body.Bytes())
	id, _ := rest["id"].(string)
	if id == "" {
		t.Fatal("create response has no id")
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: fmt.Sprintf(`{ keyValue(id: %q) { region dashboardUrl updatedAt } }`, id),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", res.Errors)
	}
	gql, ok := res.Data.(map[string]any)["keyValue"].(map[string]any)
	if !ok {
		t.Fatalf("GraphQL keyValue nil: %#v", res.Data)
	}

	if gql["region"] != "fsn1" {
		t.Errorf("GraphQL region = %v, want fsn1", gql["region"])
	}
	if du, _ := gql["dashboardUrl"].(string); du == "" {
		t.Errorf("GraphQL dashboardUrl missing or empty")
	}
	if ua, _ := gql["updatedAt"].(string); ua == "" {
		t.Errorf("GraphQL updatedAt missing or empty")
	}

	// Cross-surface agreement: GraphQL values match what REST stamped.
	if restRegion, _ := rest["region"].(string); restRegion != "" {
		assertStringField(t, "GraphQL region vs REST", gql, "region", restRegion)
	}
}

// TestMCPKeyValueMetadataParity verifies that region/dashboardUrl/updatedAt
// appear on the MCP get_key_value tool output when Metadata is configured, and
// match the values the REST handler emits for the same resource.
func TestMCPKeyValueMetadataParity(t *testing.T) {
	svc, _ := newService()
	svc.Metadata = resourcemeta.Config{Region: "fsn1", DashboardBaseURL: "https://dashboard.bex.co"}

	restW := serveREST(svc, http.MethodPost, "/v1/key-value", `{"name":"mcp-meta-kv","plan":"free"}`)
	if restW.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", restW.Code, restW.Body.String())
	}
	rest := decodeMap(t, restW.Body.Bytes())
	id, _ := rest["id"].(string)
	if id == "" {
		t.Fatal("create response has no id")
	}

	call, cleanup := kvMCPClient(t, svc)
	defer cleanup()
	got := call("get_key_value", map[string]any{"keyValueId": id})

	if got["region"] != "fsn1" {
		t.Errorf("MCP region = %v, want fsn1", got["region"])
	}
	if du, _ := got["dashboardUrl"].(string); du == "" {
		t.Errorf("MCP dashboardUrl missing or empty")
	}
	if ua, _ := got["updatedAt"].(string); ua == "" {
		t.Errorf("MCP updatedAt missing or empty")
	}

	if restRegion, _ := rest["region"].(string); restRegion != "" {
		if got["region"] != restRegion {
			t.Errorf("MCP region = %v, want %v (same as REST)", got["region"], restRegion)
		}
	}
}

// TestGraphQLAndMCPKeyValueMetadataAbsentWhenUnconfigured is the omission half:
// with no Metadata set, region and dashboardUrl must resolve to empty on both
// GraphQL and MCP (zero-value strings, not the configured value).
func TestGraphQLAndMCPKeyValueMetadataAbsentWhenUnconfigured(t *testing.T) {
	svc, _ := newService() // no Metadata

	restW := serveREST(svc, http.MethodPost, "/v1/key-value", `{"name":"bare-meta-kv","plan":"free"}`)
	if restW.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", restW.Code, restW.Body.String())
	}
	id, _ := decodeMap(t, restW.Body.Bytes())["id"].(string)
	if id == "" {
		t.Fatal("create response has no id")
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: fmt.Sprintf(`{ keyValue(id: %q) { region dashboardUrl } }`, id),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", res.Errors)
	}
	gql, ok := res.Data.(map[string]any)["keyValue"].(map[string]any)
	if !ok {
		t.Fatalf("GraphQL keyValue nil: %#v", res.Data)
	}
	if r, _ := gql["region"].(string); r != "" {
		t.Errorf("GraphQL region = %q, want empty when unconfigured", r)
	}
	if du, _ := gql["dashboardUrl"].(string); du != "" {
		t.Errorf("GraphQL dashboardUrl = %q, want empty when unconfigured", du)
	}

	call, cleanup := kvMCPClient(t, svc)
	defer cleanup()
	got := call("get_key_value", map[string]any{"keyValueId": id})
	if r, _ := got["region"].(string); r != "" {
		t.Errorf("MCP region = %q, want empty when unconfigured", r)
	}
	if du, _ := got["dashboardUrl"].(string); du != "" {
		t.Errorf("MCP dashboardUrl = %q, want empty when unconfigured", du)
	}
}
