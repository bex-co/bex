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

// dryrun_test.go — zero-side-effect assertions for w2/m29.
//
// Every test asserts that a dry-run call on REST, GraphQL, or MCP returns the
// resolved spec preview without creating or modifying any KeyValue CR in k8s.

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

// countKeyValues returns the number of KeyValue CRs in the fake client's namespace.
func countKeyValues(t *testing.T, cl client.Client) int {
	t.Helper()
	var list appv1alpha1.KeyValueList
	if err := cl.List(context.Background(), &list, client.InNamespace("default")); err != nil {
		t.Fatalf("list keyvalues: %v", err)
	}
	return len(list.Items)
}

// getKeyValue fetches a single KeyValue CR by name.
func getKeyValueCR(t *testing.T, cl client.Client, name string) *appv1alpha1.KeyValue {
	t.Helper()
	var kv appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &kv); err != nil {
		t.Fatalf("get keyvalue %s: %v", name, err)
	}
	return &kv
}

// ---- REST ----------------------------------------------------------------

func TestRESTDryRunCreateKeyValue(t *testing.T) {
	svc, cl := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// dryRun in request body => 200, no CR created.
	body := `{"name":"preview-kv","plan":"starter","dryRun":true}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/key-value", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run create => 200, got %d: %s", rec.Code, rec.Body)
	}
	var got KeyValueView
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	// A dry-run mints an opaque red- id (identity split, w9/m6) but writes no CR;
	// the display name is what the caller supplied.
	if !mintedKVID(got.ID) || got.Name != "preview-kv" || got.Plan != "starter" {
		t.Fatalf("preview wrong: %+v", got)
	}
	if n := countKeyValues(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d KeyValue(s)", n)
	}
}

func TestRESTDryRunCreateKeyValueQueryParam(t *testing.T) {
	svc, cl := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// dryRun via query param => 200, no CR created.
	body := `{"name":"preview-qp","plan":"free"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/key-value?dryRun=true", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run (query param) => 200, got %d: %s", rec.Code, rec.Body)
	}
	if n := countKeyValues(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d KeyValue(s)", n)
	}
}

func TestRESTDryRunPatchKeyValuePlan(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "patch-kv")
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// dryRun PATCH => 200, plan not written.
	body := `{"plan":"starter","dryRun":true}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/key-value/patch-kv", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run patch => 200, got %d: %s", rec.Code, rec.Body)
	}
	// The preview should reflect the new plan.
	var preview KeyValueView
	_ = json.Unmarshal(rec.Body.Bytes(), &preview)
	if preview.Plan != "starter" {
		t.Fatalf("preview plan = %v, want starter", preview.Plan)
	}
	// CR must still carry the original plan.
	got := getKeyValueCR(t, cl, "patch-kv")
	if got.Spec.Plan != "free" {
		t.Fatalf("dry-run must not modify CR, got plan=%q", got.Spec.Plan)
	}
}

// ---- GraphQL -------------------------------------------------------------

func kvGQLSchema(svc *Service) (graphql.Schema, error) {
	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
}

func TestGraphQLDryRunCreateKeyValue(t *testing.T) {
	svc, cl := newService()
	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createKeyValue(name:"gql-preview", plan:"starter", dryRun:true) { id name plan } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql createKeyValue dryRun: %v", res.Errors)
	}
	data := res.Data.(map[string]any)["createKeyValue"].(map[string]any)
	if !mintedKVID(data["id"].(string)) || data["name"] != "gql-preview" || data["plan"] != "starter" {
		t.Fatalf("preview wrong: %+v", data)
	}
	if n := countKeyValues(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d KeyValue(s)", n)
	}
}

// renameKeyValue and setKeyValueMaxmemoryPolicy carry the same dryRun contract
// as updateKeyValuePlan but had no test for it: only the applying direction was
// covered, so a dry run that silently WROTE would have gone unnoticed — a
// preview and an apply return identical responses, so nothing else surfaces it.
func TestGraphQLDryRunRenameKeyValue(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "gql-rename-dry")
	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { renameKeyValue(id:"gql-rename-dry", name:"renamed", dryRun:true) { id name } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql renameKeyValue dryRun: %v", res.Errors)
	}
	if preview := res.Data.(map[string]any)["renameKeyValue"].(map[string]any); preview["name"] != "renamed" {
		t.Fatalf("preview name = %v, want renamed", preview["name"])
	}
	if got := getKeyValueCR(t, cl, "gql-rename-dry"); got.Spec.Name != "" {
		t.Fatalf("dry-run must not modify the CR, got spec.name=%q", got.Spec.Name)
	}
}

func TestGraphQLDryRunSetKeyValueMaxmemoryPolicy(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "gql-mm-dry")
	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { setKeyValueMaxmemoryPolicy(id:"gql-mm-dry", maxmemoryPolicy:"noeviction", dryRun:true) { id maxmemoryPolicy } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql setKeyValueMaxmemoryPolicy dryRun: %v", res.Errors)
	}
	preview := res.Data.(map[string]any)["setKeyValueMaxmemoryPolicy"].(map[string]any)
	if preview["maxmemoryPolicy"] != "noeviction" {
		t.Fatalf("preview maxmemoryPolicy = %v, want noeviction", preview["maxmemoryPolicy"])
	}
	if got := getKeyValueCR(t, cl, "gql-mm-dry"); got.Spec.MaxmemoryPolicy != "" {
		t.Fatalf("dry-run must not modify the CR, got maxmemoryPolicy=%q", got.Spec.MaxmemoryPolicy)
	}
}

func TestGraphQLDryRunUpdateKeyValuePlan(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "gql-update")
	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { updateKeyValuePlan(id:"gql-update", plan:"starter", dryRun:true) { id plan } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql updateKeyValuePlan dryRun: %v", res.Errors)
	}
	preview := res.Data.(map[string]any)["updateKeyValuePlan"].(map[string]any)
	if preview["plan"] != "starter" {
		t.Fatalf("preview plan = %v, want starter", preview["plan"])
	}
	// CR must be unchanged.
	got := getKeyValueCR(t, cl, "gql-update")
	if got.Spec.Plan != "free" {
		t.Fatalf("dry-run must not modify CR, got plan=%q", got.Spec.Plan)
	}
}

// ---- MCP -----------------------------------------------------------------

func kvMCPClient(t *testing.T, svc *Service) (func(string, map[string]any) map[string]any, func()) {
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

func TestMCPDryRunCreateKeyValue(t *testing.T) {
	svc, cl := newService()
	call, cleanup := kvMCPClient(t, svc)
	defer cleanup()

	got := call("create_key_value", map[string]any{
		"name":   "mcp-preview",
		"plan":   "starter",
		"dryRun": true,
	})
	if id, _ := got["id"].(string); !mintedKVID(id) || got["name"] != "mcp-preview" || got["plan"] != "starter" {
		t.Fatalf("preview wrong: %+v", got)
	}
	if n := countKeyValues(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d KeyValue(s)", n)
	}
}

func TestMCPDryRunUpdateKeyValuePlan(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "mcp-update")
	call, cleanup := kvMCPClient(t, svc)
	defer cleanup()

	got := call("update_key_value_plan", map[string]any{
		"keyValueId": "mcp-update",
		"plan":       "starter",
		"dryRun":     true,
	})
	if got["plan"] != "starter" {
		t.Fatalf("preview plan = %v, want starter", got["plan"])
	}
	// CR must be unchanged.
	cr := getKeyValueCR(t, cl, "mcp-update")
	if cr.Spec.Plan != "free" {
		t.Fatalf("dry-run must not modify CR, got plan=%q", cr.Spec.Plan)
	}
}
