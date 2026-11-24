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
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
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
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
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
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services?dryRun=true", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run (query param) => 200, got %d: %s", rec.Code, rec.Body)
	}
	if n := countApps(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d App(s)", n)
	}
}

// w6/m609: a REST plan PATCH saves the plan for the next deploy. Its dry run
// still previews the saved plan and writes nothing: no plan, no pending
// marker, no deploy row, no store write and no plan_changed row.
func TestRESTDryRunPlanPreviewsTheSaveAndWritesNothing(t *testing.T) {
	st := &recordingStore{}
	a := managedApp("web", "srv-1")
	a.Spec.Tier = "starter"
	a.Annotations = map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}
	svc, cl := newService(st, a)
	sink := &captureAuditSink{}
	svc.Audit = sink
	before := getApp(t, cl, "web")
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	body := `{"serviceDetails":{"plan":"standard"},"dryRun":true}`
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run patch => 200, got %d: %s", rec.Code, rec.Body)
	}
	var got struct {
		ServiceDetails struct {
			Plan string `json:"plan"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.ServiceDetails.Plan != "standard" {
		t.Fatalf("preview plan = %q (%v), want the saved standard: %s", got.ServiceDetails.Plan, err, rec.Body)
	}
	after := getApp(t, cl, "web")
	if after.ResourceVersion != before.ResourceVersion || after.Spec.Tier != "starter" || !maps.Equal(after.Annotations, before.Annotations) {
		t.Fatalf("after the dry run: tier %q, annotations %v, resourceVersion %s→%s; want the App untouched",
			after.Spec.Tier, after.Annotations, before.ResourceVersion, after.ResourceVersion)
	}
	if len(st.deployCalls) != 0 || len(st.tierCalls) != 0 {
		t.Fatalf("deploy rows %+v, store plan writes %+v; want none", st.deployCalls, st.tierCalls)
	}
	if n := sink.countVerb(core.AuditVerbSetPlan); n != 0 {
		t.Fatalf("the dry run recorded %d plan_changed rows, want 0", n)
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

// createsService names the MCP tools that answer Render's ServiceAndDeploy.
var createsService = map[string]bool{"create_web_service": true, "create_cron_job": true, "create_static_site": true}

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
		// The three creates answer Render's {service, deployId} (w4/m378,
		// TestMCPCreatesReturnServiceAndDeploy); these tests read the service.
		if createsService[name] {
			out, _ = out["service"].(map[string]any)
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
