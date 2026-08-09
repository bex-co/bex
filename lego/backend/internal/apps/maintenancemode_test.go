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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// maintenancemode_test.go covers App.spec.maintenanceMode (Render's
// maintenanceMode object, w1/m37) settable + readable identically across
// REST, GraphQL, and MCP, plus its web_service-only rejection and its
// deliberate exclusion from applyCreateToSpec (a redeploy must not clear or
// silently re-enable it). Mirrors buildfilter_test.go's depth.

func TestRESTCreateThreadsMaintenanceModeAndReadsItBack(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","type":"web_service","image":{"imagePath":"nginx"},"serviceDetails":{"plan":"starter","maintenanceMode":{"enabled":true,"uri":"https://status.example.com/maintenance"}}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	a := getApp(t, cl, "web")
	if a.Spec.MaintenanceMode == nil || !a.Spec.MaintenanceMode.Enabled || a.Spec.MaintenanceMode.URI != "https://status.example.com/maintenance" {
		t.Fatalf("spec.maintenanceMode = %+v, want enabled with the custom uri", a.Spec.MaintenanceMode)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		ServiceDetails struct {
			MaintenanceMode struct {
				Enabled bool   `json:"enabled"`
				URI     string `json:"uri"`
			} `json:"maintenanceMode"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode GET: %v (%s)", err, rec.Body)
	}
	if !out.ServiceDetails.MaintenanceMode.Enabled || out.ServiceDetails.MaintenanceMode.URI != "https://status.example.com/maintenance" {
		t.Errorf("GET serviceDetails.maintenanceMode = %+v, want enabled with the custom uri", out.ServiceDetails.MaintenanceMode)
	}
}

// TestRESTGetMaintenanceModeDefaultsToDisabled asserts Render's exact shape:
// the object is always present, never omitted, even when never configured.
func TestRESTGetMaintenanceModeDefaultsToDisabled(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get => 200, got %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"maintenanceMode":{"enabled":false,"uri":""}`) {
		t.Errorf("an unconfigured service must still report maintenanceMode {enabled:false,uri:\"\"}: %s", rec.Body)
	}
}

func TestRESTPatchServiceMaintenanceMode(t *testing.T) {
	svc, cl := newService(nil, paidWebApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":true,"uri":""}}}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH maintenanceMode => 200, got %d: %s", rec.Code, rec.Body)
	}
	a := getApp(t, cl, "web")
	if a.Spec.MaintenanceMode == nil || !a.Spec.MaintenanceMode.Enabled {
		t.Fatalf("spec.maintenanceMode = %+v, want enabled", a.Spec.MaintenanceMode)
	}

	// Disabling round-trips too, and is never blocked (no confirm needed).
	body = strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":false,"uri":""}}}`)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH disable => 200, got %d: %s", rec.Code, rec.Body)
	}
	if a := getApp(t, cl, "web"); a.Spec.MaintenanceMode == nil || a.Spec.MaintenanceMode.Enabled {
		t.Errorf("spec.maintenanceMode = %+v, want disabled", a.Spec.MaintenanceMode)
	}
}

func TestRESTMaintenanceModeRejectedForNonWebService(t *testing.T) {
	createBody := `{"name":"worker","type":"background_worker","image":{"imagePath":"nginx"},"serviceDetails":{"maintenanceMode":{"enabled":true,"uri":""}}}`
	svc, _ := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(createBody)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("create maintenanceMode on background_worker => 400, got %d: %s", rec.Code, rec.Body)
	}

	worker := sampleApp("worker2")
	worker.Spec.Type = appv1alpha1.TypeBackgroundWorker
	svc, cl := newService(nil, worker)
	mux = http.NewServeMux()
	svc.RegisterREST(mux)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/worker2", strings.NewReader(`{"serviceDetails":{"maintenanceMode":{"enabled":true,"uri":""}}}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("PATCH maintenanceMode on background_worker => 400, got %d: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "worker2").Spec.MaintenanceMode != nil {
		t.Error("a rejected PATCH must not touch spec.maintenanceMode")
	}
}

func TestGraphQLCreateAndReadMaintenanceMode(t *testing.T) {
	svc, cl := newService(nil)
	schema := mustSchema(t, svc)

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { createService(name: "web", type: "web_service", image: "nginx", plan: "starter", maintenanceMode: {enabled: true, uri: "https://status.example.com/m"}) { maintenanceMode { enabled uri } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	a := getApp(t, cl, "web")
	if a.Spec.MaintenanceMode == nil || !a.Spec.MaintenanceMode.Enabled || a.Spec.MaintenanceMode.URI != "https://status.example.com/m" {
		t.Fatalf("createService did not thread maintenanceMode: %+v", a.Spec.MaintenanceMode)
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "web") { maintenanceMode { enabled uri } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	srv := res.Data.(map[string]any)["server"].(map[string]any)
	mm, ok := srv["maintenanceMode"].(map[string]any)
	if !ok {
		t.Fatalf("server.maintenanceMode = %+v, want an object", srv["maintenanceMode"])
	}
	if mm["enabled"] != true || mm["uri"] != "https://status.example.com/m" {
		t.Errorf("server.maintenanceMode = %+v, want enabled=true uri=https://status.example.com/m", mm)
	}
}

func TestGraphQLSetMaintenanceMode(t *testing.T) {
	svc, cl := newService(nil, paidWebApp("web"))
	schema := mustSchema(t, svc)

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setMaintenanceMode(id: "web", maintenanceMode: {enabled: true, uri: ""}) { maintenanceMode { enabled uri } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("setMaintenanceMode: %v", res.Errors)
	}
	if a := getApp(t, cl, "web"); a.Spec.MaintenanceMode == nil || !a.Spec.MaintenanceMode.Enabled {
		t.Errorf("setMaintenanceMode did not patch spec.maintenanceMode: %+v", a.Spec.MaintenanceMode)
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setMaintenanceMode(id: "web", maintenanceMode: {enabled: false, uri: ""}) { maintenanceMode { enabled } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("setMaintenanceMode(disable): %v", res.Errors)
	}
	if a := getApp(t, cl, "web"); a.Spec.MaintenanceMode == nil || a.Spec.MaintenanceMode.Enabled {
		t.Errorf("disable did not clear spec.maintenanceMode.enabled: %+v", a.Spec.MaintenanceMode)
	}
}

func TestGraphQLSetMaintenanceModeRejectedForNonWebService(t *testing.T) {
	worker := sampleApp("worker")
	worker.Spec.Type = appv1alpha1.TypeBackgroundWorker
	svc, _ := newService(nil, worker)
	schema := mustSchema(t, svc)

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setMaintenanceMode(id: "worker", maintenanceMode: {enabled: true, uri: ""}) { id } }`})
	if len(res.Errors) == 0 {
		t.Fatal("setMaintenanceMode on a background_worker should error")
	}
}

func TestMCPCreateWebServiceThreadsMaintenanceMode(t *testing.T) {
	req := createWebServiceArgs{
		Name: "w", Image: "nginx",
		MaintenanceMode: &maintenanceModeArg{Enabled: true, URI: "https://status.example.com/m"},
	}.toCreateRequest()
	if req.MaintenanceMode == nil || !req.MaintenanceMode.Enabled || req.MaintenanceMode.URI != "https://status.example.com/m" {
		t.Errorf("create_web_service maintenanceMode not threaded: %+v", req.MaintenanceMode)
	}
	// Omitted arg => nil, not a create-time error (disabled is the default).
	if got := (createWebServiceArgs{Name: "w2", Image: "nginx"}.toCreateRequest()); got.MaintenanceMode != nil {
		t.Errorf("omitted maintenanceMode should thread nil, got %+v", got.MaintenanceMode)
	}
}

func TestMCPSetMaintenanceMode(t *testing.T) {
	svc, cl := newService(nil, paidWebApp("web"))

	v, err := svc.SetMaintenanceMode(context.Background(), "web", MaintenanceModeView{Enabled: true, URI: "https://status.example.com/m"})
	if err != nil {
		t.Fatalf("SetMaintenanceMode: %v", err)
	}
	// MCP projects through the same renderService as REST — parity by construction.
	out := toRenderService(v)
	mm, _ := out.ServiceDetails["maintenanceMode"].(map[string]any)
	if mm["enabled"] != true || mm["uri"] != "https://status.example.com/m" {
		t.Errorf("set_maintenance_mode (via renderService) = %+v, want enabled=true uri=https://status.example.com/m", mm)
	}
	if a := getApp(t, cl, "web"); a.Spec.MaintenanceMode == nil || !a.Spec.MaintenanceMode.Enabled || a.Spec.MaintenanceMode.URI != "https://status.example.com/m" {
		t.Errorf("spec.maintenanceMode = %+v, want enabled with the custom uri", a.Spec.MaintenanceMode)
	}
}

// --- SetMaintenanceMode validation + guards ---

func TestSetMaintenanceModeRejectsNonWebService(t *testing.T) {
	for _, typ := range []string{appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob, appv1alpha1.TypeStaticSite} {
		t.Run(typ, func(t *testing.T) {
			app := sampleApp("app-" + typ)
			app.Spec.Type = typ
			svc, cl := newService(nil, app)

			_, err := svc.SetMaintenanceMode(context.Background(), app.Name, MaintenanceModeView{Enabled: true})
			if !errors.Is(err, core.ErrBadRequest) {
				t.Errorf("SetMaintenanceMode on a %s should be core.ErrBadRequest, got %v", typ, err)
			}
			if getApp(t, cl, app.Name).Spec.MaintenanceMode != nil {
				t.Error("a rejected SetMaintenanceMode must not touch spec.maintenanceMode")
			}
		})
	}
}

func TestSetMaintenanceModeAllowsLegacyEmptyType(t *testing.T) {
	// Empty spec.type defaults to web_service (view()'s own default) — a
	// legacy/hand-applied App with no explicit type must not be wrongly
	// rejected by requireWebService.
	app := sampleApp("legacy")
	app.Spec.Type = ""
	app.Spec.Tier = "starter"
	svc, cl := newService(nil, app)

	if _, err := svc.SetMaintenanceMode(context.Background(), "legacy", MaintenanceModeView{Enabled: true}); err != nil {
		t.Fatalf("SetMaintenanceMode on an empty-type App: %v", err)
	}
	if a := getApp(t, cl, "legacy"); a.Spec.MaintenanceMode == nil || !a.Spec.MaintenanceMode.Enabled {
		t.Error("empty-type App should accept maintenanceMode like an explicit web_service")
	}
}

func TestSetMaintenanceModeValidatesURI(t *testing.T) {
	svc, cl := newService(nil, paidWebApp("web"))

	for _, bad := range []string{"not a url", "ftp://example.com/page", "/relative/path", "javascript:alert(1)"} {
		if _, err := svc.SetMaintenanceMode(context.Background(), "web", MaintenanceModeView{Enabled: true, URI: bad}); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("SetMaintenanceMode(uri=%q) should be core.ErrBadRequest, got %v", bad, err)
		}
	}
	if getApp(t, cl, "web").Spec.MaintenanceMode != nil {
		t.Error("rejected SetMaintenanceMode calls must not mutate spec.maintenanceMode")
	}

	// A valid absolute http(s) URL is accepted.
	if _, err := svc.SetMaintenanceMode(context.Background(), "web", MaintenanceModeView{Enabled: true, URI: "https://status.example.com/maintenance"}); err != nil {
		t.Errorf("SetMaintenanceMode with a valid uri: %v", err)
	}
}

// TestSetMaintenanceModeRejectsPlatformRoutedHosts is the backend layer of the
// cross-service recursion guard: a maintenance page URI whose host belongs to
// ANY App routed through the platform — not just the service itself — is
// refused, so two paid services cannot point maintenance pages at each other
// and close an amplifying synchronous-fetch cycle through the shared
// activator. The claimed-host sweep is cluster-wide, so it reaches across
// workspaces (per-tenant namespaces, ADR043); the activator re-checks the same
// denylist at fetch time (defense in depth).
func TestSetMaintenanceModeRejectsPlatformRoutedHosts(t *testing.T) {
	other := paidWebApp("other")
	other.Namespace = "tenant-b" // another workspace's namespace
	other.Spec.Hosts = []string{"b.example.com"}
	svc, cl := newBaseDomainService("onbex.co", "", paidWebApp("web"), other)

	for _, uri := range []string{
		"https://b.example.com/m",  // another service's custom host (the A->B half of a cycle)
		"https://B.Example.com/m",  // case variant of the same host
		"https://b.example.com./m", // trailing-dot spelling of the same host
		"https://other.onbex.co/m", // another service's platform host
	} {
		if _, err := svc.SetMaintenanceMode(context.Background(), "web", MaintenanceModeView{Enabled: true, URI: uri}); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("SetMaintenanceMode(uri=%q) should be core.ErrBadRequest, got %v", uri, err)
		}
	}
	if getApp(t, cl, "web").Spec.MaintenanceMode != nil {
		t.Error("rejected SetMaintenanceMode calls must not mutate spec.maintenanceMode")
	}

	// A genuinely external page is still accepted.
	if _, err := svc.SetMaintenanceMode(context.Background(), "web", MaintenanceModeView{Enabled: true, URI: "https://status.example.com/m"}); err != nil {
		t.Errorf("SetMaintenanceMode with an external uri: %v", err)
	}
}

// TestRedeployNeverTouchesMaintenanceMode is the interaction docs/render-
// artifacts/maintenance-mode.md commits to: "deploys proceed normally... the
// maintenance page persists... until explicitly disabled." A redeploy
// (blueprint re-apply, the applyCreateToSpec path) must never clear an
// enabled maintenance window nor silently re-enable a disabled one.
func TestRedeployNeverTouchesMaintenanceMode(t *testing.T) {
	app := repoApp("web", "https://github.com/x/mono", "main")
	app.Spec.Tier = "starter"
	app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true, URI: "https://status.example.com/m"}
	svc, cl := newService(nil, app)

	// A manifest that changes a create-owned build field (rootDir), so the
	// idempotent-upsert path actually reaches applyCreateToSpec instead of
	// short-circuiting as a no-op.
	manifest := "services:\n  - name: web\n    type: web\n    runtime: docker\n    repo: https://github.com/x/mono\n    plan: starter\n    rootDir: cmd/web\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack (redeploy): %v", err)
	}
	got := getApp(t, cl, "web")
	if got.Spec.RootDir != "cmd/web" {
		t.Fatalf("redeploy did not apply — rootDir = %q, want the manifest to have actually re-applied", got.Spec.RootDir)
	}
	if got.Spec.MaintenanceMode == nil || !got.Spec.MaintenanceMode.Enabled || got.Spec.MaintenanceMode.URI != "https://status.example.com/m" {
		t.Errorf("redeploy must not touch spec.maintenanceMode, got %+v", got.Spec.MaintenanceMode)
	}
}
