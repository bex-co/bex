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

// dryrun_test.go — zero-side-effect assertions for w2/m29.
//
// Every test asserts that a dry-run call on REST, GraphQL, or MCP returns the
// resolved spec preview without creating or modifying any App CR in k8s.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// countApps returns the number of App CRs in the fake client's namespace.
func countApps(t *testing.T, cl client.Client) int {
	t.Helper()
	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list, client.InNamespace("default")); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	return len(list.Items)
}

// seedAppWithPlan creates an App CR with the given plan tier for PATCH / update tests.
func seedAppWithPlan(t *testing.T, cl client.Client, name, tierID string) {
	t.Helper()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: name + ":v1", Tier: tierID},
	}
	if err := cl.Create(context.Background(), app); err != nil {
		t.Fatalf("seed app %s: %v", name, err)
	}
}

// ---- REST ----------------------------------------------------------------

func TestRESTDryRunCreateService(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// dryRun in request body => 200, no CR created.
	// Image is Render's nested object: {"imagePath": "..."}.
	body := `{"name":"preview-svc","image":{"imagePath":"img:v1"},"dryRun":true}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run create => 200, got %d: %s", rec.Code, rec.Body)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["id"] != "preview-svc" {
		t.Fatalf("preview id = %v, want preview-svc", got["id"])
	}
	if n := countApps(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d App(s)", n)
	}
}

func TestRESTDryRunCreateServiceQueryParam(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// dryRun via query param => 200, no CR created.
	body := `{"name":"preview-qp","image":{"imagePath":"img:v1"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services?dryRun=true", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run (query param) => 200, got %d: %s", rec.Code, rec.Body)
	}
	if n := countApps(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d App(s)", n)
	}
}

func TestRESTDryRunPatchServicePlan(t *testing.T) {
	svc, cl := newService(nil)
	seedAppWithPlan(t, cl, "patch-svc", "free")
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// dryRun PATCH with plan => 200, plan not written.
	body := `{"serviceDetails":{"plan":"starter"},"dryRun":true}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/patch-svc", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run patch => 200, got %d: %s", rec.Code, rec.Body)
	}
	// The CR must still carry the original plan tier.
	got := getApp(t, cl, "patch-svc")
	if got.Spec.Tier != "free" {
		t.Fatalf("dry-run patch must not modify CR, got tier=%q", got.Spec.Tier)
	}
}

// ---- GraphQL -------------------------------------------------------------

func appsGQLSchema(svc *Service) (graphql.Schema, error) {
	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
}

func TestGraphQLDryRunCreateService(t *testing.T) {
	svc, cl := newService(nil)
	schema, err := appsGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { createService(name:"gql-preview", image:"img:v1", dryRun:true) { id } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql createService dryRun: %v", res.Errors)
	}
	data := res.Data.(map[string]any)
	if id := data["createService"].(map[string]any)["id"]; id != "gql-preview" {
		t.Fatalf("preview id = %v, want gql-preview", id)
	}
	if n := countApps(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d App(s)", n)
	}
}

func TestGraphQLDryRunUpdateServicePlan(t *testing.T) {
	svc, cl := newService(nil)
	seedAppWithPlan(t, cl, "gql-update", "free")
	schema, err := appsGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { updateServicePlan(id:"gql-update", plan:"starter", dryRun:true) { id plan } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql updateServicePlan dryRun: %v", res.Errors)
	}
	// The preview should reflect the new plan without writing it.
	svcData := res.Data.(map[string]any)["updateServicePlan"].(map[string]any)
	if svcData["plan"] != "starter" {
		t.Fatalf("preview plan = %v, want starter", svcData["plan"])
	}
	// CR must be unchanged.
	got := getApp(t, cl, "gql-update")
	if got.Spec.Tier != "free" {
		t.Fatalf("dry-run must not modify CR, got tier=%q", got.Spec.Tier)
	}
}

// ---- MCP -----------------------------------------------------------------

func appsMCPClient(t *testing.T, svc *Service) (func(string, map[string]any) map[string]any, func()) {
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

func TestMCPDryRunCreateWebService(t *testing.T) {
	svc, cl := newService(nil)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	got := call("create_web_service", map[string]any{
		"name":         "mcp-preview",
		"image":        "img:v1",
		"runtime":      "image",
		"buildCommand": "",
		"startCommand": "",
		"dryRun":       true,
	})
	if got["id"] != "mcp-preview" {
		t.Fatalf("preview id = %v, want mcp-preview", got["id"])
	}
	if n := countApps(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d App(s)", n)
	}
}

func TestMCPDryRunUpdateServicePlan(t *testing.T) {
	svc, cl := newService(nil)
	seedAppWithPlan(t, cl, "mcp-update", "free")
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	got := call("update_service", map[string]any{
		"serviceId": "mcp-update",
		"plan":      "starter",
		"dryRun":    true,
	})
	// plan is nested under serviceDetails in the Render service shape.
	sd, _ := got["serviceDetails"].(map[string]any)
	if sd == nil || sd["plan"] != "starter" {
		t.Fatalf("preview serviceDetails.plan = %v, want starter (full: %v)", sd, got)
	}
	// CR must be unchanged.
	cr := getApp(t, cl, "mcp-update")
	if cr.Spec.Tier != "free" {
		t.Fatalf("dry-run must not modify CR, got tier=%q", cr.Spec.Tier)
	}
}
