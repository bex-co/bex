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

package postgres

// dryrun_test.go — zero-side-effect assertions for w2/m29.
//
// Every test asserts that a dry-run call on REST, GraphQL, or MCP returns the
// resolved spec preview without creating or modifying any Database CR in k8s.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// countDatabases returns the number of Database CRs in the fake client's namespace.
func countDatabases(t *testing.T, cl client.Client) int {
	t.Helper()
	var list appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &list, client.InNamespace("default")); err != nil {
		t.Fatalf("list databases: %v", err)
	}
	return len(list.Items)
}

// getDatabase fetches a single Database CR by name.
func getDatabase(t *testing.T, cl client.Client, name string) *appv1alpha1.Database {
	t.Helper()
	var d appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &d); err != nil {
		t.Fatalf("get database %s: %v", name, err)
	}
	return &d
}

// ---- REST ----------------------------------------------------------------

func TestRESTDryRunCreatePostgres(t *testing.T) {
	svc, cl := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// dryRun in request body => 200, no CR created.
	body := `{"name":"preview-db","databaseName":"preview_data","databaseUser":"preview_owner","plan":"basic-1gb","dryRun":true}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/postgres", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run create => 200, got %d: %s", rec.Code, rec.Body)
	}
	var got PostgresView
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if !strings.HasPrefix(got.ID, "dpg-") || got.Name != "preview-db" || got.DatabaseName != "preview_data" || got.DatabaseUser != "preview_owner" || got.Plan != "basic-1gb" {
		t.Fatalf("preview wrong: %+v", got)
	}
	if n := countDatabases(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d Database(s)", n)
	}
}

func TestRESTDryRunCreatePostgresQueryParam(t *testing.T) {
	svc, cl := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// dryRun via query param => 200, no CR created.
	body := `{"name":"preview-qp","plan":"free"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/postgres?dryRun=true", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run (query param) => 200, got %d: %s", rec.Code, rec.Body)
	}
	if n := countDatabases(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d Database(s)", n)
	}
}

func TestRESTDryRunPatchPostgresPlan(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "patch-db")
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// dryRun PATCH => 200, plan not written.
	body := `{"plan":"basic-1gb","dryRun":true}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/postgres/patch-db", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run patch => 200, got %d: %s", rec.Code, rec.Body)
	}
	// The preview should reflect the new plan.
	var preview PostgresView
	_ = json.Unmarshal(rec.Body.Bytes(), &preview)
	if preview.Plan != "basic-1gb" {
		t.Fatalf("preview plan = %v, want basic-1gb", preview.Plan)
	}
	// CR must still carry the original plan.
	got := getDatabase(t, cl, "patch-db")
	if got.Spec.Plan != "free" {
		t.Fatalf("dry-run must not modify CR, got plan=%q", got.Spec.Plan)
	}
}

// ---- GraphQL -------------------------------------------------------------

func pgGQLSchema(svc *Service) (graphql.Schema, error) {
	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
}

func TestGraphQLDryRunCreateDatabase(t *testing.T) {
	svc, cl := pgGQLNewService()
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createDatabase(name:"gql-preview", plan:"basic-1gb", dryRun:true) { id name plan } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql createDatabase dryRun: %v", res.Errors)
	}
	data := res.Data.(map[string]any)["createDatabase"].(map[string]any)
	if !strings.HasPrefix(data["id"].(string), "dpg-") || data["name"] != "gql-preview" || data["plan"] != "basic-1gb" {
		t.Fatalf("preview wrong: %+v", data)
	}
	if n := countDatabases(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d Database(s)", n)
	}
}

func TestGraphQLDryRunUpdateDatabasePlan(t *testing.T) {
	svc, cl := pgGQLNewService()
	seedDatabase(t, cl, "gql-update")
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { updateDatabasePlan(id:"gql-update", plan:"basic-1gb", dryRun:true) { id plan } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql updateDatabasePlan dryRun: %v", res.Errors)
	}
	preview := res.Data.(map[string]any)["updateDatabasePlan"].(map[string]any)
	if preview["plan"] != "basic-1gb" {
		t.Fatalf("preview plan = %v, want basic-1gb", preview["plan"])
	}
	// CR must be unchanged.
	got := getDatabase(t, cl, "gql-update")
	if got.Spec.Plan != "free" {
		t.Fatalf("dry-run must not modify CR, got plan=%q", got.Spec.Plan)
	}
}

// ---- MCP -----------------------------------------------------------------

func pgMCPClient(t *testing.T, svc *Service) (func(string, map[string]any) map[string]any, func()) {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	call := func(name string, args map[string]any) map[string]any {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v isErr=%v", name, err, res.IsError)
		}
		out := map[string]any{}
		if res.StructuredContent != nil {
			b, _ := json.Marshal(res.StructuredContent)
			_ = json.Unmarshal(b, &out)
		}
		return out
	}
	return call, func() { cs.Close() }
}

func TestMCPDryRunCreatePostgres(t *testing.T) {
	svc, cl := pgGQLNewService()
	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()

	got := call("create_postgres", map[string]any{
		"name":   "mcp-preview",
		"plan":   "basic-1gb",
		"dryRun": true,
	})
	if !strings.HasPrefix(got["id"].(string), "dpg-") || got["name"] != "mcp-preview" || got["plan"] != "basic-1gb" {
		t.Fatalf("preview wrong: %+v", got)
	}
	if n := countDatabases(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d Database(s)", n)
	}
}

func TestMCPDryRunUpdatePostgresPlan(t *testing.T) {
	svc, cl := pgGQLNewService()
	seedDatabase(t, cl, "mcp-update")
	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()

	got := call("update_postgres_plan", map[string]any{
		"postgresId": "mcp-update",
		"plan":       "basic-1gb",
		"dryRun":     true,
	})
	if got["plan"] != "basic-1gb" {
		t.Fatalf("preview plan = %v, want basic-1gb", got["plan"])
	}
	// CR must be unchanged.
	cr := getDatabase(t, cl, "mcp-update")
	if cr.Spec.Plan != "free" {
		t.Fatalf("dry-run must not modify CR, got plan=%q", cr.Spec.Plan)
	}
}

// pgGQLNewService creates a fresh service + client for GraphQL/MCP tests —
// the local newService() helper in postgres_test.go takes variadic client.Object
// which requires the scheme to be built the same way.
func pgGQLNewService() (*Service, client.Client) {
	svc, cl := newService()
	return svc, cl
}
