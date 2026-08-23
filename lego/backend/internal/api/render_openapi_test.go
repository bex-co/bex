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

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRenderOpenAPIPinGuards(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(renderOpenAPISource); got != 331391 {
		t.Fatalf("snapshot size = %d, want 331391", got)
	}
	if got := contract.document.OpenAPI; got != "3.0.2" {
		t.Fatalf("OpenAPI = %q, want 3.0.2", got)
	}
	if got := contract.document.Paths.Len(); got != 130 {
		t.Fatalf("paths = %d, want 130", got)
	}
	if got := len(contract.document.Components.Schemas); got != 163 {
		t.Fatalf("schemas = %d, want 163", got)
	}
	operations, requestBodies := 0, 0
	for _, item := range contract.document.Paths.Map() {
		for _, operation := range item.Operations() {
			operations++
			if operation.RequestBody != nil {
				requestBodies++
			}
		}
	}
	if operations != 207 || requestBodies != 58 {
		t.Fatalf("operations/request bodies = %d/%d, want 207/58", operations, requestBodies)
	}
	if len(contract.document.Servers) != 0 {
		t.Fatalf("runtime contract retained upstream servers: %#v", contract.document.Servers)
	}

	tampered := append([]byte(nil), renderOpenAPISource...)
	tampered[len(tampered)-1] ^= 1
	if _, err := loadRenderOpenAPIContractData(tampered, renderOpenAPISHA256); err == nil || !strings.Contains(err.Error(), "integrity mismatch") {
		t.Fatalf("tampered snapshot error = %v", err)
	}
}

func TestRenderOpenAPIRefPolicyGuard(t *testing.T) {
	var raw any
	if err := json.Unmarshal(renderOpenAPISource, &raw); err != nil {
		t.Fatal(err)
	}
	var siblings []string
	var external []string
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case []any:
			for _, child := range value {
				walk(child)
			}
		case map[string]any:
			if ref, ok := value["$ref"].(string); ok {
				if !strings.HasPrefix(ref, "#/") {
					external = append(external, ref)
				}
				var names []string
				for name := range value {
					if name != "$ref" && !strings.HasPrefix(name, "x-") {
						names = append(names, name)
					}
				}
				if len(names) > 0 {
					sort.Strings(names)
					siblings = append(siblings, ref+"|"+strings.Join(names, ","))
				}
			}
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(raw)
	sort.Strings(siblings)
	want := []string{
		"#/components/schemas/DeployMode|description",
		"#/components/schemas/plan|default,description",
		"#/components/schemas/serviceEventWithCursor/properties/event/properties/details/oneOf/41/properties/newTrigger|description",
	}
	if fmt.Sprint(siblings) != fmt.Sprint(want) {
		t.Fatalf("non-extension $ref siblings = %v, want %v", siblings, want)
	}
	if len(external) != 0 {
		t.Fatalf("external references in pinned contract: %v", external)
	}
}

func TestRenderOpenAPICompatibilityIsOperationScoped(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	for operationID, fields := range renderRequiredCompatibility {
		_, operation := findRenderOperation(t, contract.document, operationID)
		media := operation.RequestBody.Value.Content.Get("application/json")
		for _, field := range fields {
			if slices.Contains(media.Schema.Value.Required, field) {
				t.Errorf("%s still requires compatibility field %q", operationID, field)
			}
		}
	}
	for operationID, names := range renderOptionalParameterCompatibility {
		item, operation := findRenderOperation(t, contract.document, operationID)
		for _, name := range names {
			found := false
			for _, parameters := range []openapi3.Parameters{item.Parameters, operation.Parameters} {
				for _, ref := range parameters {
					if ref.Value != nil && ref.Value.Name == name {
						found = true
						if ref.Value.Required {
							t.Errorf("%s parameter %s is still required", operationID, name)
						}
					}
				}
			}
			if !found {
				t.Errorf("%s compatibility parameter %s is absent", operationID, name)
			}
		}
	}
	for operationID, extensions := range renderQueryExtensions {
		item, operation := findRenderOperation(t, contract.document, operationID)
		route := &routers.Route{PathItem: item, Operation: operation}
		for extension := range extensions {
			req := httptest.NewRequest(http.MethodGet, "/?"+extension+"=true", nil)
			if contract.hasUnknownRenderQuery(route, req) {
				t.Errorf("%s rejects documented query extension %s", operationID, extension)
			}
		}
	}
}

func TestRenderRouteIntersectionInventory(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	mux := NewServer(&core.Base{}, Deps{}).restHandler()
	parameter := regexp.MustCompile(`\{[^}]+\}`)
	var operationIDs []string
	for path, item := range contract.document.Paths.Map() {
		for method, operation := range item.Operations() {
			target := "/v1" + parameter.ReplaceAllString(path, "fixture")
			req := httptest.NewRequest(strings.ToUpper(method), target, nil)
			if _, pattern := mux.Handler(req); pattern != "" {
				operationIDs = append(operationIDs, operation.OperationID)
			}
		}
	}
	sort.Strings(operationIDs)
	const expected = "add-disk,add-or-update-secret-file,autoscale-service,cancel-cron-job-run,cancel-deploy,cancel-job,create-custom-domain,create-deploy,create-env-group,create-environment,create-key-value,create-postgres,create-postgres-export,create-project,create-registry-credential,create-service,create-webhook,delete-autoscaling-config,delete-custom-domain,delete-disk,delete-env-group,delete-env-group-env-var,delete-env-group-secret-file,delete-env-var,delete-environment,delete-key-value,delete-postgres,delete-project,delete-registry-credential,delete-secret-file,delete-service,delete-webhook,disconnect-blueprint,failover-postgres,get-bandwidth,get-cpu,get-cpu-target,get-disk-capacity,get-env-vars-for-service,get-http-latency,get-http-requests,get-instance-count,get-memory,get-memory-target,get-replication-lag,get-user,link-service-to-env-group,list-blueprint-syncs,list-blueprints,list-custom-domains,list-deploys,list-disks,list-env-groups,list-environments,list-events,list-headers,list-instances,list-job,list-key-value,list-logs,list-logs-values,list-owners,list-postgres,list-postgres-export,list-projects,list-registry-credentials,list-routes,list-secret-files-for-service,list-services,list-webhook-events,list-webhooks,listWorkflows,patch-service-notification-overrides,post-job,put-routes,refresh-custom-domain,restart-postgres,restart-service,resume-key-value,resume-postgres,resume-service,retrieve-blueprint,retrieve-custom-domain,retrieve-deploy,retrieve-disk,retrieve-env-group,retrieve-env-group-env-var,retrieve-env-group-secret-file,retrieve-env-var,retrieve-environment,retrieve-event,retrieve-job,retrieve-key-value,retrieve-key-value-connection-info,retrieve-owner,retrieve-owner-members,retrieve-postgres,retrieve-postgres-connection-info,retrieve-project,retrieve-registry-credential,retrieve-secret-file,retrieve-service,retrieve-service-notification-overrides,retrieve-webhook,rollback-deploy,run-cron-job,scale-service,subscribe-logs,suspend-key-value,suspend-postgres,suspend-service,unlink-service-from-env-group,update-blueprint,update-disk,update-env-group,update-env-group-env-var,update-env-group-secret-file,update-env-var,update-env-vars-for-service,update-environment,update-headers,update-key-value,update-postgres,update-project,update-registry-credential,update-service,update-webhook,validate-blueprint"
	if got := strings.Join(operationIDs, ","); got != expected {
		t.Fatalf("Render route intersection changed (got %d operations).\n got: %s\nwant: %s", len(operationIDs), got, expected)
	}
}

func findRenderOperation(t *testing.T, doc *openapi3.T, operationID string) (*openapi3.PathItem, *openapi3.Operation) {
	t.Helper()
	for _, item := range doc.Paths.Map() {
		for _, operation := range item.Operations() {
			if operation.OperationID == operationID {
				return item, operation
			}
		}
	}
	t.Fatalf("operation %q is missing from pinned Render contract", operationID)
	return nil, nil
}

type openAPIHandlerBody struct {
	Name             string   `json:"name"`
	Repo             string   `json:"repo"`
	Type             string   `json:"type"`
	Builder          string   `json:"builder"`
	Port             int32    `json:"port"`
	Plan             string   `json:"plan"`
	Domains          []string `json:"domains"`
	PublishPath      string   `json:"publishPath"`
	PreDeployCommand string   `json:"preDeployCommand"`
	DryRun           bool     `json:"dryRun"`
	Routes           []struct {
		Type        string `json:"type"`
		Source      string `json:"source"`
		Destination string `json:"destination"`
	} `json:"routes"`
	Headers []struct {
		Path  string `json:"path"`
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	ServiceDetails *struct {
		Plan           string `json:"plan"`
		IdleTTLSeconds *int32 `json:"idleTTLSeconds"`
	} `json:"serviceDetails"`
}

func strictMutationHandler(mutations *int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body openAPIHandlerBody
		if err := core.DecodeJSON(r, &body); err != nil {
			core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
			return
		}
		(*mutations)++
		w.WriteHeader(http.StatusNoContent)
	}
}

func newOpenAPITestHandler(t *testing.T, mutations *int) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/services", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /v1/events/{eventId}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /v1/webhooks/{webhookId}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /v1/webhooks/{webhookId}/events", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("POST /v1/services", strictMutationHandler(mutations))
	mux.HandleFunc("PATCH /v1/services/{serviceId}", strictMutationHandler(mutations))
	mux.HandleFunc("PATCH /v1/webhooks/{webhookId}", strictMutationHandler(mutations))
	mux.HandleFunc("DELETE /v1/services/{serviceId}", func(w http.ResponseWriter, _ *http.Request) {
		(*mutations)++
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /v1/webhooks/{webhookId}", func(w http.ResponseWriter, _ *http.Request) {
		(*mutations)++
		w.WriteHeader(http.StatusNoContent)
	})
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func requestOpenAPITest(t *testing.T, h http.Handler, method, target, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestRenderRequestValidatorRejectsBeforeSideEffects(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		target      string
		contentType string
		body        string
		secret      string
	}{
		{name: "required", method: http.MethodPost, target: "/v1/services", body: `{}`},
		{name: "body type", method: http.MethodPost, target: "/v1/services", body: `{"name":{"private":"secret-type-marker"}}`, secret: "secret-type-marker"},
		{name: "enum", method: http.MethodPost, target: "/v1/services", body: `{"name":"web","type":"secret-enum-marker"}`, secret: "secret-enum-marker"},
		{name: "format", method: http.MethodGet, target: "/v1/services?createdAfter=secret-format-marker", secret: "secret-format-marker"},
		{name: "range", method: http.MethodGet, target: "/v1/services?limit=9999", secret: "9999"},
		{name: "path pattern", method: http.MethodDelete, target: "/v1/webhooks/secret-pattern-marker", secret: "secret-pattern-marker"},
		{name: "event path pattern", method: http.MethodGet, target: "/v1/events/secret-event-marker", secret: "secret-event-marker"},
		{name: "media type", method: http.MethodPost, target: "/v1/services", contentType: "text/plain", body: `{"name":"secret-media-marker"}`, secret: "secret-media-marker"},
		{name: "unknown query", method: http.MethodGet, target: "/v1/services?secret_query=secret-query-marker", secret: "secret-query-marker"},
		{name: "unknown body", method: http.MethodPost, target: "/v1/services", body: `{"name":"web","mystery":"secret-body-marker"}`, secret: "secret-body-marker"},
		{name: "unknown nested body", method: http.MethodPost, target: "/v1/services", body: `{"name":"web","serviceDetails":{"plan":"starter","mystery":"secret-nested-marker"}}`, secret: "secret-nested-marker"},
		{name: "trailing JSON", method: http.MethodPost, target: "/v1/services", body: `{"name":"web"} {"secret":"secret-trailing-marker"}`, secret: "secret-trailing-marker"},
		{name: "stray body", method: http.MethodGet, target: "/v1/services", body: `{"secret":"secret-stray-marker"}`, secret: "secret-stray-marker"},
		{name: "malformed no panic", method: http.MethodPost, target: "/v1/services", body: `{"name":"secret-malformed-marker"`, secret: "secret-malformed-marker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutations := 0
			h := newOpenAPITestHandler(t, &mutations)
			w := requestOpenAPITest(t, h, tt.method, tt.target, tt.contentType, tt.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
			}
			if mutations != 0 {
				t.Fatalf("rejected request performed %d mutations", mutations)
			}
			if tt.secret != "" && strings.Contains(w.Body.String(), tt.secret) {
				t.Fatalf("response leaked submitted value %q: %s", tt.secret, w.Body.String())
			}
			if w.Body.Len() > 600 || strings.Contains(w.Body.String(), "Schema:") || strings.Contains(w.Body.String(), "Value:") {
				t.Fatalf("unbounded validator response: %s", w.Body.String())
			}
			var envelope struct {
				ID      string `json:"id"`
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.ID != "bad_request" || envelope.Error == "" || envelope.Message != envelope.Error {
				t.Fatalf("non-Render error envelope %#v, err %v", envelope, err)
			}
		})
	}
}

func TestRenderRequestValidatorAcceptsExtensionsAndPreservesInput(t *testing.T) {
	mutations := 0
	h := newOpenAPITestHandler(t, &mutations)
	body := `{"name":"web","repo":"https://github.com/acme/web","builder":"buildkit","port":8080,"plan":"starter","domains":["api.example.com"],"publishPath":"dist","preDeployCommand":"go migrate","routes":[{"type":"rewrite","source":"/a","destination":"/b"}],"headers":[{"path":"/*","name":"X-Test","value":"yes"}],"dryRun":true}`
	w := requestOpenAPITest(t, h, http.MethodPost, "/v1/services?dryRun=true", "", body)
	if w.Code != http.StatusNoContent || mutations != 1 {
		t.Fatalf("extensions: status=%d mutations=%d body=%s", w.Code, mutations, w.Body.String())
	}

	w = requestOpenAPITest(t, h, http.MethodPatch, "/v1/services/srv-test", "application/json", `{"serviceDetails":{"idleTTLSeconds":300}}`)
	if w.Code != http.StatusNoContent || mutations != 2 {
		t.Fatalf("idleTTLSeconds: status=%d mutations=%d body=%s", w.Code, mutations, w.Body.String())
	}

	w = requestOpenAPITest(t, h, http.MethodDelete, "/v1/services/srv-test?confirm=true", "", "")
	if w.Code != http.StatusNoContent || mutations != 3 {
		t.Fatalf("confirm: status=%d mutations=%d body=%s", w.Code, mutations, w.Body.String())
	}

	w = requestOpenAPITest(t, h, http.MethodGet, "/v1/webhooks/whk-00000000000000000000?ownerId=tea-bravo", "", "")
	if w.Code != http.StatusNoContent || mutations != 3 {
		t.Fatalf("webhook ownerId extension: status=%d mutations=%d body=%s", w.Code, mutations, w.Body.String())
	}

	w = requestOpenAPITest(t, h, http.MethodPatch, "/v1/webhooks/whk-00000000000000000000?ownerId=tea-bravo", "application/json", `{"name":"hook"}`)
	if w.Code != http.StatusNoContent || mutations != 4 {
		t.Fatalf("update webhook ownerId extension: status=%d mutations=%d body=%s", w.Code, mutations, w.Body.String())
	}

	w = requestOpenAPITest(t, h, http.MethodGet, "/v1/webhooks/whk-00000000000000000000/events?ownerId=tea-bravo&status=failed", "", "")
	if w.Code != http.StatusNoContent || mutations != 4 {
		t.Fatalf("webhook status extension: status=%d mutations=%d body=%s", w.Code, mutations, w.Body.String())
	}

	w = requestOpenAPITest(t, h, http.MethodDelete, "/v1/webhooks/whk-00000000000000000000?ownerId=tea-bravo", "", "")
	if w.Code != http.StatusNoContent || mutations != 5 {
		t.Fatalf("delete webhook ownerId extension: status=%d mutations=%d body=%s", w.Code, mutations, w.Body.String())
	}

	w = requestOpenAPITest(t, h, http.MethodGet, "/v1/events/evt-00000000000000000000?ownerId=tea-bravo", "", "")
	if w.Code != http.StatusNoContent || mutations != 5 {
		t.Fatalf("event ownerId extension: status=%d mutations=%d body=%s", w.Code, mutations, w.Body.String())
	}
}

func TestRenderRequestValidatorBodyIsBytePreservedAndDefaultsAreNotInjected(t *testing.T) {
	var gotBody, gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/services", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /v1/services", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	})
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	body := " {\n  \"name\" : \"web\", \"repo\" : \"https://github.com/acme/web\"\n } "
	if w := requestOpenAPITest(t, h, http.MethodPost, "/v1/services", "", body); w.Code != http.StatusNoContent {
		t.Fatalf("body preservation status=%d: %s", w.Code, w.Body.String())
	}
	if gotBody != body {
		t.Fatalf("handler body changed:\n got %q\nwant %q", gotBody, body)
	}
	if w := requestOpenAPITest(t, h, http.MethodGet, "/v1/services", "", ""); w.Code != http.StatusNoContent {
		t.Fatalf("default status=%d: %s", w.Code, w.Body.String())
	}
	if gotQuery != "" {
		t.Fatalf("validator injected query defaults: %q", gotQuery)
	}
}

func TestRenderRequestValidatorPassThroughBoundaries(t *testing.T) {
	mux := http.NewServeMux()
	var nativeBody string
	mux.HandleFunc("POST /v1/apps", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("POST /v1/native", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		nativeBody = string(data)
		w.WriteHeader(http.StatusAccepted)
	})
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"unknown":"kept"} {"trailing":"kept"}`
	w := requestOpenAPITest(t, h, http.MethodPost, "/v1/apps?unknown=kept", "text/plain", body)
	if w.Code != http.StatusNotFound {
		t.Fatalf("retired alias status=%d, want 404", w.Code)
	}
	w = requestOpenAPITest(t, h, http.MethodPost, "/v1/native?unknown=kept", "text/plain", body)
	if w.Code != http.StatusAccepted || nativeBody != body {
		t.Fatalf("native route changed: status=%d body=%q", w.Code, nativeBody)
	}
	w = requestOpenAPITest(t, h, http.MethodGet, "/v1/disks", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unsupported Render route status=%d, want 404", w.Code)
	}
}

func TestRenderValidationRunsAfterAuthentication(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{APIKeys: newFakeKeyStore()})
	w := do(t, h, http.MethodPost, "/v1/services", "", `{}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated malformed request status=%d, want 401: %s", w.Code, w.Body.String())
	}
}

func TestRenderValidationRunsAfterAndConsumesRateLimit(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{APIKeys: newFakeKeyStore()})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.RateLimiter = NewRateLimiter(0.01, 1)
	h := buildHandler(t, srv)
	if w := do(t, h, http.MethodPost, "/v1/services", testToken, `{}`); w.Code != http.StatusBadRequest {
		t.Fatalf("first malformed request status=%d, want 400: %s", w.Code, w.Body.String())
	}
	if w := do(t, h, http.MethodGet, "/v1/services", testToken, ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("validation failure did not consume rate budget: status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestRenderCLIImageOwnerContractThroughComposedServer exercises the exact
// generated Render CLI payload through auth, the pinned OpenAPI validator, and
// the real services adapter. The CLI's Image.OwnerId field has no omitempty,
// so both create and update serialize image.ownerId:"". Create also emits the
// selected region inside serviceDetails; bex validates and normalizes it to its
// configured single-region placement.
func TestRenderCLIImageOwnerContractThroughComposedServer(t *testing.T) {
	cl := fakeClient()
	base := &core.Base{
		Client:    cl,
		Namespace: "default",
		Workspace: fakeWorkspace{"client-1": "tea-cli"},
	}
	h, _ := serverWith(t, base, Deps{APIKeys: newFakeKeyStore()})

	createBody := `{"name":"cli-image","ownerId":"tea-cli","type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":""},"serviceDetails":{"plan":"starter","region":"frankfurt","runtime":"image"}}`
	w := do(t, h, http.MethodPost, "/v1/services", testToken, createBody)
	if w.Code != http.StatusCreated {
		t.Fatalf("exact CLI image create = %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		Service struct {
			ID string `json:"id"`
		} `json:"service"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.Service.ID == "" {
		t.Fatalf("decode create response: id=%q err=%v body=%s", created.Service.ID, err, w.Body.String())
	}

	patchBody := `{"image":{"imagePath":"nginx:stable","ownerId":""}}`
	w = do(t, h, http.MethodPatch, "/v1/services/"+created.Service.ID, testToken, patchBody)
	if w.Code != http.StatusOK {
		t.Fatalf("exact CLI image update = %d: %s", w.Code, w.Body.String())
	}
	var apps appv1alpha1.AppList
	if err := cl.List(context.Background(), &apps); err != nil || len(apps.Items) != 1 {
		t.Fatalf("created Apps = %d, err=%v", len(apps.Items), err)
	}
	if got := apps.Items[0].Spec.Image; got != "nginx:stable" {
		t.Fatalf("updated image = %q, want nginx:stable", got)
	}

	w = do(t, h, http.MethodPatch, "/v1/services/"+created.Service.ID, testToken,
		`{"image":{"imagePath":"nginx:rejected","ownerId":"tea-other"}}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "image.ownerId") {
		t.Fatalf("conflicting image owner = %d, want named 400: %s", w.Code, w.Body.String())
	}
	apps = appv1alpha1.AppList{}
	if err := cl.List(context.Background(), &apps); err != nil || len(apps.Items) != 1 || apps.Items[0].Spec.Image != "nginx:stable" {
		t.Fatalf("rejected conflict mutated App: apps=%#v err=%v", apps.Items, err)
	}

	w = do(t, h, http.MethodPatch, "/v1/services/"+created.Service.ID, testToken,
		`{"image":{"imagePath":"nginx:rejected","ownerId":"","mystery":true}}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown nested image field = %d, want 400: %s", w.Code, w.Body.String())
	}
	apps = appv1alpha1.AppList{}
	if err := cl.List(context.Background(), &apps); err != nil || len(apps.Items) != 1 || apps.Items[0].Spec.Image != "nginx:stable" {
		t.Fatalf("unknown field mutated App: apps=%#v err=%v", apps.Items, err)
	}
}
