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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/406: Render's "Private services only support default TCP checks"
// (render.com/docs/health-checks), its private-service API schemas have no
// healthCheckPath, and its CLI refuses --health-check-path on every type but
// web_service. A Health Check Path is a web-service setting.

const notApplicableToPrivate = "health check path is not applicable to a private_service"

func privateApp(name, path string) *appv1alpha1.App {
	app := sampleApp(name)
	app.Spec.Type, app.Spec.Tier, app.Spec.HealthCheckPath = appv1alpha1.TypePrivateService, "starter", path
	return app
}

// Every direct create and update surface refuses a private service's path
// with the worker's reason, and stores nothing.
func TestPrivateServiceHealthCheckPathIsRefusedOnEveryDirectSurface(t *testing.T) {
	svc, cl := newService(nil, privateApp("pserv", ""))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rest := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}
	for _, body := range []string{
		`{"name":"new","type":"private_service","image":{"imagePath":"nginx:1"},"serviceDetails":{"healthCheckPath":"/healthz"}}`,
		`{"name":"new","type":"private_service","image":{"imagePath":"nginx:1"},"serviceDetails":{"healthCheckPath":"/healthz"},"dryRun":true}`,
	} {
		if rec := rest("POST", "/v1/services", body); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), notApplicableToPrivate) {
			t.Fatalf("REST create %s = %d %s", body, rec.Code, rec.Body)
		}
	}
	if rec := rest("PATCH", "/v1/services/pserv", `{"healthCheckPath":"/ready"}`); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), notApplicableToPrivate) {
		t.Fatalf("REST PATCH = %d %s", rec.Code, rec.Body)
	}

	schema := mustSchema(t, svc)
	for _, query := range []string{
		`mutation { createService(name: "new", type: "private_service", image: "nginx:1", healthCheckPath: "/healthz") { id } }`,
		`mutation { setHealthCheckPath(id: "pserv", path: "/ready") { id } }`,
	} {
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
		if len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, notApplicableToPrivate) {
			t.Fatalf("GraphQL %s: %v", query, res.Errors)
		}
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(context.Background(), serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for tool, args := range map[string]map[string]any{
		"create_web_service": {"name": "new", "type": "private_service", "image": "nginx:1", "runtime": "image", "plan": "starter",
			"buildCommand": "", "startCommand": "", "healthCheckPath": "/healthz"},
		"update_service": {"serviceId": "pserv", "healthCheckPath": "/ready"},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(result.Content)
		if !result.IsError || !strings.Contains(string(body), notApplicableToPrivate) {
			t.Fatalf("MCP %s = %s", tool, body)
		}
	}

	var apps appv1alpha1.AppList
	if err := cl.List(context.Background(), &apps); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 1 || apps.Items[0].Spec.HealthCheckPath != "" {
		t.Fatalf("a refused write stored something: %+v", apps.Items)
	}
}

// A path stored before w8/406 has no effect, is not read back on any surface,
// and can still be cleared.
func TestPrivateServiceStoredHealthCheckPathIsNotReadBackAndClears(t *testing.T) {
	svc, cl := newService(nil, privateApp("pserv", "/legacy"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/pserv", nil))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "healthCheckPath") {
		t.Fatalf("REST read = %d %s, want no healthCheckPath", rec.Code, rec.Body)
	}
	res := graphql.Do(graphql.Params{Schema: mustSchema(t, svc), Context: context.Background(),
		RequestString: `{ server(id: "pserv") { healthCheckPath } }`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	if got := res.Data.(map[string]any)["server"].(map[string]any)["healthCheckPath"]; got != nil && got != "" {
		t.Fatalf("GraphQL healthCheckPath = %v, want none", got)
	}

	rec = httptest.NewRecorder()
	validated(t, mux).ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/pserv", strings.NewReader(`{"healthCheckPath":""}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("clearing PATCH = %d %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "pserv").Spec.HealthCheckPath; got != "" {
		t.Fatalf("spec.healthCheckPath = %q after clearing", got)
	}
}

// Render's Blueprint schema accepts healthCheckPath on every server service,
// so a pserv or worker entry with one validates. The path has no effect: the
// service is created with none, and a re-plan of the same file, also over a
// path stored before w8/406, is a no-op. A cron job's schema has no such field.
func TestBlueprintPrivateServiceAndWorkerHealthCheckPathHasNoEffect(t *testing.T) {
	const manifest = `services:
  - type: pserv
    name: pserv
    runtime: image
    image: {url: nginx:1.27}
    plan: starter
    healthCheckPath: /healthz
  - type: worker
    name: worker
    runtime: image
    image: {url: busybox:1.37}
    plan: starter
    healthCheckPath: /h
`
	svc, cl := newService(nil)
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !v.Valid || len(v.Errors) != 0 {
		t.Fatalf("validate = %+v, %v", v, err)
	}
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pserv", "worker"} {
		if spec := getApp(t, cl, name).Spec; spec.HealthCheckPath != "" || spec.HTTPHealthCheckPath() != "" {
			t.Fatalf("%s spec.healthCheckPath = %q, want none", name, spec.HealthCheckPath)
		}
	}
	stored := getApp(t, cl, "pserv")
	stored.Spec.HealthCheckPath = "/legacy"
	if err := cl.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	v, err = svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !v.Valid || v.Plan == nil || len(v.Plan.Actions) != 2 {
		t.Fatalf("re-plan = %+v, %v", v, err)
	}
	for _, action := range v.Plan.Actions {
		if action.Operation != BlueprintPlanNoop {
			t.Fatalf("re-plan was not a no-op: %+v", v.Plan)
		}
	}
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("reapply: %v", err)
	}
	if got := getApp(t, cl, "pserv").Spec.HealthCheckPath; got != "/legacy" {
		t.Fatalf("reapply rewrote the stored path to %q", got)
	}

	cron := "services:\n  - type: cron\n    name: c\n    runtime: image\n    image: {url: busybox:1.37}\n    schedule: \"0 0 1 1 *\"\n    healthCheckPath: /h\n"
	if v, err := svc.ValidateBlueprint(context.Background(), "", cron, ""); err != nil || v.Valid {
		t.Fatalf("cron with healthCheckPath = %+v, %v, want invalid", v, err)
	}
}
