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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func maxShutdownSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return schema
}

func TestRESTMaxShutdownDelayCreatePatchAndReadPlacement(t *testing.T) {
	for _, serviceType := range []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
	} {
		t.Run(serviceType, func(t *testing.T) {
			svc, cl := newService(nil)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)

			body := fmt.Sprintf(`{"name":"app","type":%q,"image":{"imagePath":"nginx:v1"},"serviceDetails":{"maxShutdownDelaySeconds":75}}`, serviceType)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
			if rec.Code != http.StatusCreated {
				t.Fatalf("create => %d: %s", rec.Code, rec.Body)
			}
			app := getApp(t, cl, "app")
			if app.Spec.MaxShutdownDelaySeconds == nil || *app.Spec.MaxShutdownDelaySeconds != 75 {
				t.Fatalf("spec.maxShutdownDelaySeconds = %v, want 75", app.Spec.MaxShutdownDelaySeconds)
			}
			assertMaxShutdownDelayPlacement(t, rec.Body.Bytes(), 75)

			rec = httptest.NewRecorder()
			patch := `{"serviceDetails":{"maxShutdownDelaySeconds":120}}`
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/app", strings.NewReader(patch)))
			if rec.Code != http.StatusOK {
				t.Fatalf("PATCH => %d: %s", rec.Code, rec.Body)
			}
			app = getApp(t, cl, "app")
			if app.Spec.MaxShutdownDelaySeconds == nil || *app.Spec.MaxShutdownDelaySeconds != 120 {
				t.Fatalf("patched spec.maxShutdownDelaySeconds = %v, want 120", app.Spec.MaxShutdownDelaySeconds)
			}
			assertMaxShutdownDelayPlacement(t, rec.Body.Bytes(), 120)

			rec = httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/app", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET => %d: %s", rec.Code, rec.Body)
			}
			assertMaxShutdownDelayPlacement(t, rec.Body.Bytes(), 120)
		})
	}
}

func assertMaxShutdownDelayPlacement(t *testing.T, body []byte, want float64) {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// Render's create response wraps the service, while PATCH/GET return the
	// service object directly. Validate the field's placement within either
	// operation's service object.
	if service, ok := out["service"].(map[string]any); ok {
		out = service
	}
	if _, exists := out["maxShutdownDelaySeconds"]; exists {
		t.Fatalf("maxShutdownDelaySeconds must be nested under serviceDetails: %s", body)
	}
	details, ok := out["serviceDetails"].(map[string]any)
	if !ok || details["maxShutdownDelaySeconds"] != want {
		t.Fatalf("serviceDetails.maxShutdownDelaySeconds = %v, want %v: %s", details["maxShutdownDelaySeconds"], want, body)
	}
}

func TestRESTMaxShutdownDelayDefaultsToThirtyOnReadWithoutMutatingSpec(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET => %d: %s", rec.Code, rec.Body)
	}
	assertMaxShutdownDelayPlacement(t, rec.Body.Bytes(), 30)
	if got := getApp(t, cl, "web").Spec.MaxShutdownDelaySeconds; got != nil {
		t.Fatalf("read mutated spec.maxShutdownDelaySeconds to %v", *got)
	}
}

func TestRESTMaxShutdownDelayRejectsRangeAndWrongTypesWithNamed400(t *testing.T) {
	badValues := []string{"0", "301", `"30"`, "1.5", "true", "null", "[]"}
	for _, value := range badValues {
		t.Run(value, func(t *testing.T) {
			svc, _ := newService(nil)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			body := fmt.Sprintf(`{"name":"web","image":{"imagePath":"nginx:v1"},"serviceDetails":{"maxShutdownDelaySeconds":%s}}`, value)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(body)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("value %s => %d, want 400: %s", value, rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), "maxShutdownDelaySeconds") {
				t.Fatalf("value %s returned unnamed error: %s", value, rec.Body)
			}
		})
	}

	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	for _, value := range []string{"0", "301", `"30"`, "false", "null"} {
		rec := httptest.NewRecorder()
		body := fmt.Sprintf(`{"serviceDetails":{"maxShutdownDelaySeconds":%s}}`, value)
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "maxShutdownDelaySeconds") {
			t.Errorf("PATCH value %s => %d %s, want named 400", value, rec.Code, rec.Body)
		}
	}
}

func TestMaxShutdownDelayRejectsCronAndStaticServices(t *testing.T) {
	for _, serviceType := range []string{appv1alpha1.TypeCronJob, appv1alpha1.TypeStaticSite} {
		t.Run(serviceType, func(t *testing.T) {
			seconds := int32(60)
			req := CreateRequest{Name: "app", Type: serviceType, Image: "nginx:v1", MaxShutdownDelaySeconds: &seconds}
			if serviceType == appv1alpha1.TypeCronJob {
				req.Schedule = "0 * * * *"
			} else {
				req.PublishPath = "dist"
			}
			svc, _ := newService(nil)
			if _, err := svc.Create(context.Background(), req); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "maxShutdownDelaySeconds") {
				t.Fatalf("create %s should return named ErrBadRequest, got %v", serviceType, err)
			}

			app := sampleApp("existing")
			app.Spec.Type = serviceType
			svc, _ = newService(nil, app)
			if _, err := svc.SetMaxShutdownDelay(context.Background(), "existing", 60); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "maxShutdownDelaySeconds") {
				t.Fatalf("set %s should return named ErrBadRequest, got %v", serviceType, err)
			}
		})
	}
}

func TestGraphQLMaxShutdownDelayCreateSetReadAndValidation(t *testing.T) {
	svc, cl := newService(nil)
	schema := maxShutdownSchema(t, svc)

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `
		mutation { createService(name: "worker", type: "background_worker", image: "nginx:v1", maxShutdownDelaySeconds: 80) { maxShutdownDelaySeconds } }
	`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	if got := getApp(t, cl, "worker").Spec.MaxShutdownDelaySeconds; got == nil || *got != 80 {
		t.Fatalf("createService did not thread maxShutdownDelaySeconds: %v", got)
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `
		mutation { setMaxShutdownDelay(id: "worker", seconds: 150) { maxShutdownDelaySeconds } }
	`})
	if len(res.Errors) > 0 {
		t.Fatalf("setMaxShutdownDelay: %v", res.Errors)
	}
	set := res.Data.(map[string]any)["setMaxShutdownDelay"].(map[string]any)
	if set["maxShutdownDelaySeconds"] != 150 {
		t.Fatalf("set response = %v, want 150", set["maxShutdownDelaySeconds"])
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{ server(id: "worker") { maxShutdownDelaySeconds } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server: %v", res.Errors)
	}
	server := res.Data.(map[string]any)["server"].(map[string]any)
	if server["maxShutdownDelaySeconds"] != 150 {
		t.Fatalf("server.maxShutdownDelaySeconds = %v, want 150", server["maxShutdownDelaySeconds"])
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { setMaxShutdownDelay(id: "worker", seconds: 301) { id } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "maxShutdownDelaySeconds") {
		t.Fatalf("out-of-range GraphQL error = %v, want named error", res.Errors)
	}
}

func TestMCPMaxShutdownDelayMirrorsSharedCreateSetAndRead(t *testing.T) {
	seconds := int32(65)
	req := createWebServiceArgs{
		Name: "worker", Type: appv1alpha1.TypeBackgroundWorker, Image: "nginx:v1",
		MaxShutdownDelaySeconds: &seconds,
	}.toCreateRequest()
	if req.MaxShutdownDelaySeconds == nil || *req.MaxShutdownDelaySeconds != seconds {
		t.Fatalf("create_web_service did not thread maxShutdownDelaySeconds: %+v", req)
	}

	svc, _ := newService(nil, sampleApp("worker"))
	app := getApp(t, svc.Client, "worker")
	app.Spec.Type = appv1alpha1.TypeBackgroundWorker
	if err := svc.Client.Update(context.Background(), app); err != nil {
		t.Fatalf("set worker type: %v", err)
	}
	view, err := svc.SetMaxShutdownDelay(context.Background(), "worker", 95)
	if err != nil {
		t.Fatalf("set_max_shutdown_delay: %v", err)
	}
	rendered := toRenderService(view)
	if rendered.ServiceDetails["maxShutdownDelaySeconds"] != int32(95) {
		t.Fatalf("MCP render serviceDetails = %+v", rendered.ServiceDetails)
	}
}

// TestBlueprintMaxShutdownDelayRoundTripsAndValidates is w10/m2/t003+t006: the
// bex.yml Blueprint parser threads maxShutdownDelaySeconds onto CreateRequest
// (previously dropped — ADR006's recorded drift) and gets Create's ordinary
// validation for free, so an out-of-range Blueprint value fails exactly like
// REST/GraphQL/MCP (same named core.ErrBadRequest), not silently or differently.
func TestBlueprintMaxShutdownDelayRoundTripsAndValidates(t *testing.T) {
	stack, err := parseStack(DeployRequest{Manifest: `services:
  - type: web
    name: web
    image: {url: nginx:1}
    maxShutdownDelaySeconds: 75
`})
	if err != nil {
		t.Fatal(err)
	}
	req := stack.services[0].req
	if req.MaxShutdownDelaySeconds == nil || *req.MaxShutdownDelaySeconds != 75 {
		t.Fatalf("Blueprint request MaxShutdownDelaySeconds = %v, want 75", req.MaxShutdownDelaySeconds)
	}

	svc, cl := newService(nil)
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: `services:
  - type: web
    name: web
    image: {url: nginx:1}
    maxShutdownDelaySeconds: 75
`}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	app := getApp(t, cl, "web")
	if app.Spec.MaxShutdownDelaySeconds == nil || *app.Spec.MaxShutdownDelaySeconds != 75 {
		t.Fatalf("spec.maxShutdownDelaySeconds = %v, want 75", app.Spec.MaxShutdownDelaySeconds)
	}

	svc, _ = newService(nil)
	_, err = svc.DeployStack(context.Background(), DeployRequest{Manifest: `services:
  - type: web
    name: web2
    image: {url: nginx:1}
    maxShutdownDelaySeconds: 301
`})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "maxShutdownDelaySeconds") {
		t.Fatalf("out-of-range Blueprint value should return the same named ErrBadRequest as REST, got %v", err)
	}
}
