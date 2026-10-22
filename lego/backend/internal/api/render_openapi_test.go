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
	"github.com/bex-co/bex/lego/backend/internal/registrycreds"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRenderOpenAPIPinGuards(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(renderOpenAPISource); got != 345074 {
		t.Fatalf("snapshot size = %d, want 345074", got)
	}
	if got := contract.document.OpenAPI; got != "3.0.2" {
		t.Fatalf("OpenAPI = %q, want 3.0.2", got)
	}
	if got := contract.document.Paths.Len(); got != 131 {
		t.Fatalf("paths = %d, want 131", got)
	}
	if got := len(contract.document.Components.Schemas); got != 164 {
		t.Fatalf("schemas = %d, want 164", got)
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
	if operations != 208 || requestBodies != 58 {
		t.Fatalf("operations/request bodies = %d/%d, want 208/58", operations, requestBodies)
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
		"#/paths/~1events~1{eventId}/get/responses/200/content/application~1json/schema/properties/details/oneOf/0/oneOf/45/properties/newTrigger|description",
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
			if len(contract.unknownRenderQuery(route, req)) > 0 {
				t.Errorf("%s rejects documented query extension %s", operationID, extension)
			}
		}
	}
	for operationID, extensions := range renderBodyPropertyExtensions {
		_, operation := findRenderOperation(t, contract.document, operationID)
		schema := operation.RequestBody.Value.Content.Get("application/json").Schema.Value
		for extension := range extensions {
			if schema.Properties[extension] == nil {
				t.Errorf("%s rejects documented body extension %s", operationID, extension)
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
	// w2/m100 t002 added list-notification-overrides: bex now serves Render's
	// workspace-wide override list, so it is a real intersection. Every other
	// entry is unchanged — t001's validator pass-through deliberately does not
	// alter this set, it only stops a bex-native literal being claimed by a
	// Render template. w5/113 added list-owner-audit-logs: the audit log now
	// mounts without the control-plane store, as production always served it.
	// w8/137 added Render's five single-rule static-site edge operations.
	// w8/188 added add-resources-to-environment and remove-resources-from-environment.
	const expected = "add-disk,add-headers,add-or-update-secret-file,add-resources-to-environment,add-route,autoscale-service,cancel-cron-job-run,cancel-deploy,cancel-job,create-custom-domain,create-deploy,create-env-group,create-environment,create-key-value,create-postgres,create-postgres-export,create-project,create-registry-credential,create-service,create-webhook,delete-autoscaling-config,delete-custom-domain,delete-disk,delete-env-group,delete-env-group-env-var,delete-env-group-secret-file,delete-env-var,delete-environment,delete-header,delete-key-value,delete-postgres,delete-postgres-user,delete-project,delete-registry-credential,delete-route,delete-secret-file,delete-service,delete-webhook,disconnect-blueprint,failover-postgres,get-active-connections,get-bandwidth,get-bandwidth-sources,get-cpu,get-cpu-limit,get-cpu-target,get-disk-capacity,get-disk-usage,get-env-vars-for-service,get-http-latency,get-http-requests,get-instance-count,get-memory,get-memory-limit,get-memory-target,get-replication-lag,get-user,link-service-to-env-group,list-application-filter-values,list-blueprint-syncs,list-blueprints,list-custom-domains,list-deploys,list-disks,list-env-groups,list-environments,list-events,list-headers,list-http-filter-values,list-instances,list-job,list-key-value,list-logs,list-logs-values,list-notification-overrides,list-owner-audit-logs,list-owners,list-path-filter-values,list-postgres,list-postgres-export,list-postgres-processes,list-postgres-sizes,list-postgres-table-scans,list-postgres-top-queries,list-postgres-users,list-projects,list-registry-credentials,list-routes,list-secret-files-for-service,list-services,list-snapshots,list-webhook-events,list-webhooks,listWorkflows,patch-route,patch-service-notification-overrides,post-job,purge-cache,put-routes,recover-postgres,refresh-custom-domain,remove-resources-from-environment,restart-postgres,restart-service,restore-snapshot,resume-key-value,resume-postgres,resume-service,retrieve-blueprint,retrieve-custom-domain,retrieve-deploy,retrieve-disk,retrieve-env-group,retrieve-env-group-env-var,retrieve-env-group-secret-file,retrieve-env-var,retrieve-environment,retrieve-event,retrieve-job,retrieve-key-value,retrieve-key-value-connection-info,retrieve-owner,retrieve-owner-members,retrieve-postgres,retrieve-postgres-connection-info,retrieve-postgres-recovery-info,retrieve-project,retrieve-registry-credential,retrieve-secret-file,retrieve-service,retrieve-service-notification-overrides,retrieve-service-outbound-ips,retrieve-webhook,rollback-deploy,run-cron-job,scale-service,subscribe-logs,suspend-key-value,suspend-postgres,suspend-service,unlink-service-from-env-group,update-blueprint,update-disk,update-env-group,update-env-group-env-var,update-env-group-secret-file,update-env-var,update-env-vars-for-service,update-environment,update-headers,update-key-value,update-postgres,update-project,update-registry-credential,update-secret-files-for-service,update-service,update-webhook,validate-blueprint"
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

// TestMetricsPercentageQueryThroughComposedServer guards bex's
// replica-percentage query extensions (w5/m89 aggregateAllMethod + w5/m90
// percentage) against the strict Render-query gate. The metrics adapter tests
// drive a bare mux that never sees the gate, so without the extension entries
// the exact DoD call (?percentage=true&aggregateAllMethod=AVG) 400s on the
// real server while GraphQL and MCP serve it — found by the w5/m90 t008 live
// walkthrough. An unknown app must 404 (gate passed, store miss), while a
// genuinely unknown parameter must still 400.

// TestKeyValueSuspendedQueryThroughComposedServer admits bex's list-key-value
// ?suspended= extension past the Render-query gate (w5/060). Bare keyvalue
// handler tests never see the gate, so production 400'd while unit tests
// passed. Unknown values must reach the handler's named enum 400.
func TestKeyValueSuspendedQueryThroughComposedServer(t *testing.T) {
	cl := fakeClient()
	base := &core.Base{
		Client:    cl,
		Namespace: "default",
		Workspace: fakeWorkspace{"client-1": "tea-cli"},
	}
	h, _ := serverWith(t, base, Deps{APIKeys: newFakeKeyStore()})

	w := do(t, h, http.MethodGet, "/v1/key-value?ownerId=tea-cli&suspended=suspended", testToken, "")
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "unsupported query parameter") {
		t.Fatalf("suspended extension still blocked at the gate: %s", w.Body.String())
	}
	if w.Code != http.StatusOK {
		t.Fatalf("suspended=suspended = %d, want 200 after gate: %s", w.Code, w.Body.String())
	}

	w = do(t, h, http.MethodGet, "/v1/key-value?ownerId=tea-cli&suspended=bogus", testToken, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("suspended=bogus = %d, want 400: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "unsupported query parameter") {
		t.Fatalf("bogus value must reach handler enum check, not the gate: %s", body)
	}
	if !strings.Contains(body, "suspended") || !strings.Contains(body, "not_suspended") {
		t.Fatalf("bogus value must name accepted enum values: %s", body)
	}
}

func TestMetricsPercentageQueryThroughComposedServer(t *testing.T) {
	cl := fakeClient()
	base := &core.Base{
		Client:    cl,
		Namespace: "default",
		Workspace: fakeWorkspace{"client-1": "tea-cli"},
	}
	h, _ := serverWith(t, base, Deps{APIKeys: newFakeKeyStore()})

	for _, path := range []string{
		"/v1/metrics/cpu",
		"/v1/metrics/memory",
		"/v1/metrics/cpu-target",
		"/v1/metrics/memory-target",
		"/v1/metrics/instance-count",
		"/v1/metrics/http-requests",
		"/v1/metrics/http-latency",
		"/v1/metrics/bandwidth",
	} {
		w := do(t, h, http.MethodGet, path+"?resource=srv-x&percentage=true&aggregateAllMethod=AVG", testToken, "")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s percentage+aggregate = %d, want 404 (gate passed, unknown app): %s", path, w.Code, w.Body.String())
		}
	}
	w := do(t, h, http.MethodGet, "/v1/metrics/cpu?resource=srv-x&bogusParam=1", testToken, "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unsupported query parameter") {
		t.Fatalf("unknown param = %d, want 400 unsupported query parameter: %s", w.Code, w.Body.String())
	}
}

// TestOutboundIPsThroughComposedServer is the w6/073 evidence that the
// 2026-09-11 pin refresh admits Render's retrieve-service-outbound-ips
// (GET /services/{serviceId}/outbound-ips) through the composed request
// validator. Under the 2026-07-20 pin the operation was absent, so the route
// was an ungated bex extension: a valid read passed, but so did any stray
// query. Now the read still reaches the existing handler (w2/023) and its body
// conforms to the pinned outboundIps response schema, while the strict-query
// gate rejects an undeclared parameter — the same shape every other enforced
// operation has.
func TestOutboundIPsThroughComposedServer(t *testing.T) {
	base := &core.Base{
		Client:    fakeClient(),
		Namespace: "default",
		Workspace: fakeWorkspace{"client-1": "tea-cli"},
	}
	h, _ := serverWith(t, base, Deps{APIKeys: newFakeKeyStore()})

	w := do(t, h, http.MethodPost, "/v1/services", testToken,
		`{"name":"egress","ownerId":"tea-cli","type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":""}}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create service = %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		Service struct {
			ID string `json:"id"`
		} `json:"service"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.Service.ID == "" {
		t.Fatalf("decode create response: id=%q err=%v body=%s", created.Service.ID, err, w.Body.String())
	}

	w = do(t, h, http.MethodGet, "/v1/services/"+created.Service.ID+"/outbound-ips", testToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("outbound-ips through the gate = %d, want 200: %s", w.Code, w.Body.String())
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"type":"shared","ips":[]}` {
		t.Fatalf("outbound-ips body = %s, want the shared empty-pool answer", got)
	}
	if divergences := loadRenderSpec(t).validate("retrieve-service-outbound-ips", w.Body.Bytes()); len(divergences) != 0 {
		t.Fatalf("outbound-ips response diverges from the pinned outboundIps schema: %v", divergences)
	}

	w = do(t, h, http.MethodGet, "/v1/services/"+created.Service.ID+"/outbound-ips?bogusParam=1", testToken, "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unsupported query parameter") {
		t.Fatalf("undeclared query on outbound-ips = %d, want 400 unsupported query parameter: %s", w.Code, w.Body.String())
	}

	w = do(t, h, http.MethodGet, "/v1/services/srv-00000000000000000000/outbound-ips", testToken, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown service outbound-ips = %d, want 404 (gate passed, store miss): %s", w.Code, w.Body.String())
	}
}

// TestRenderRequestValidatorAdmitsRefreshedPlanSpellings locks in the other
// documented delta of the 2026-09-11 pin: Render's spec-based plan spellings
// joined the plan/paidPlan/keyValuePlan/postgres enums (pure additions — no
// legacy name was dropped). The w8/011 input aliases (0.1c-256mb, 0.5c-1g,
// 256mb, 1g) were therefore rejected at this gate over REST under the old pin
// even though the datastore handlers accepted them. An unknown plan is still a
// 400 that never echoes the submitted value.
func TestRenderRequestValidatorAdmitsRefreshedPlanSpellings(t *testing.T) {
	reached := 0
	mux := http.NewServeMux()
	for _, pattern := range []string{"POST /v1/postgres", "POST /v1/key-value", "POST /v1/services"} {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) {
			reached++
			w.WriteHeader(http.StatusNoContent)
		})
	}
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	admitted := []struct{ target, body string }{
		{"/v1/postgres", `{"name":"db","plan":"0.1c-256mb"}`},
		{"/v1/postgres", `{"name":"db","plan":"0.5c-1g"}`},
		{"/v1/postgres", `{"name":"db","plan":"basic-256mb"}`},
		{"/v1/key-value", `{"name":"kv","plan":"256mb"}`},
		{"/v1/key-value", `{"name":"kv","plan":"1g"}`},
		{"/v1/services", `{"name":"web","type":"web_service","serviceDetails":{"plan":"0.5c-512mb"}}`},
	}
	for i, tt := range admitted {
		w := requestOpenAPITest(t, h, http.MethodPost, tt.target, "", tt.body)
		if w.Code != http.StatusNoContent || reached != i+1 {
			t.Fatalf("%s %s: status=%d reached=%d body=%s", tt.target, tt.body, w.Code, reached, w.Body.String())
		}
	}
	for _, tt := range []struct{ target, body string }{
		{"/v1/postgres", `{"name":"db","plan":"secret-plan-marker"}`},
		{"/v1/key-value", `{"name":"kv","plan":"secret-plan-marker"}`},
	} {
		w := requestOpenAPITest(t, h, http.MethodPost, tt.target, "", tt.body)
		if w.Code != http.StatusBadRequest || reached != len(admitted) || strings.Contains(w.Body.String(), "secret-plan-marker") {
			t.Fatalf("%s unknown plan: status=%d reached=%d body=%s", tt.target, w.Code, reached, w.Body.String())
		}
	}
}

// TestRenderRequestValidatorNamesParameterConstraints (w5/063): schema
// violations for query parameters must name the bound or enum from the pin,
// not the bare `invalid query parameter "x"` that hid ParseEnum's vocabulary.
func TestRenderRequestValidatorNamesParameterConstraints(t *testing.T) {
	mux := http.NewServeMux()
	for _, pattern := range []string{
		"GET /v1/logs",
		"GET /v1/services",
		"GET /v1/postgres",
	} {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
	}
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		path string
		want []string // every substring must appear; none of the "bare" message
	}{
		{
			path: "/v1/logs?resource=srv-x&limit=0",
			want: []string{"limit", "1", "100"},
		},
		{
			path: "/v1/logs?resource=srv-x&limit=150",
			want: []string{"limit", "1", "100"},
		},
		{
			path: "/v1/logs?resource=srv-x&direction=sideways",
			want: []string{"direction", "forward", "backward"},
		},
		{
			path: "/v1/services?limit=500",
			want: []string{"limit", "1", "100"},
		},
		{
			path: "/v1/postgres?suspended=bogus",
			want: []string{"suspended", "not_suspended"},
		},
	}
	for _, tc := range cases {
		w := requestOpenAPITest(t, h, http.MethodGet, tc.path, "", "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400: %s", tc.path, w.Code, w.Body.String())
		}
		body := w.Body.String()
		if strings.Contains(body, `invalid query parameter`) {
			t.Fatalf("%s still uses the bare invalid-query message: %s", tc.path, body)
		}
		for _, s := range tc.want {
			if !strings.Contains(body, s) {
				t.Fatalf("%s body %q missing %q", tc.path, body, s)
			}
		}
	}

	// Distinct violations of opposite bounds must not collapse to one opaque string.
	lo := requestOpenAPITest(t, h, http.MethodGet, "/v1/logs?resource=srv-x&limit=0", "", "")
	hi := requestOpenAPITest(t, h, http.MethodGet, "/v1/logs?resource=srv-x&limit=150", "", "")
	if lo.Body.String() == hi.Body.String() {
		// Both may still say "between 1 and 100" — that's fine and distinct from
		// the old bare message. Require at least that they name the constraint.
		if !strings.Contains(lo.Body.String(), "between") {
			t.Fatalf("limit=0 and limit=150 both opaque: %s", lo.Body.String())
		}
	}

	// Unsupported (undeclared) params keep their own wording (w5/060).
	w := requestOpenAPITest(t, h, http.MethodGet, "/v1/logs?resource=srv-x&notARealParam=1", "", "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unsupported query parameter") {
		t.Fatalf("undeclared param = %d %s, want unsupported query parameter", w.Code, w.Body.String())
	}
}

// TestRenderRequestValidatorNamesUnsupportedQuery is w8/059: an undeclared
// query parameter is named in the refusal (sorted, quoted, capped), never the
// anonymous "request contains an unsupported query parameter".
func TestRenderRequestValidatorNamesUnsupportedQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/postgres", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("x", 200)
	cases := []struct {
		query string
		want  string
	}{
		{"ownerId=tea-x&limt=5", `unsupported query parameter "limt" for list-postgres`},
		{"zeta=1&ownerId=tea-x&alpha=2", `unsupported query parameters "alpha", "zeta" for list-postgres`},
		{"a=1&b=1&c=1&d=1&e=1&f=1&g=1", `unsupported query parameters "a", "b", "c", "d", "e" and 2 more for list-postgres`},
		{"bad%0Aname=1", `unsupported query parameter "bad\nname" for list-postgres`},
		{long + "=1", `unsupported query parameter "` + strings.Repeat("x", 64) + `…" for list-postgres`},
	}
	for _, tc := range cases {
		w := requestOpenAPITest(t, h, http.MethodGet, "/v1/postgres?"+tc.query, "", "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d, want 400: %s", tc.query, w.Code, w.Body.String())
		}
		var envelope struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || envelope.Message != tc.want {
			t.Fatalf("%s message = %q (err %v), want %q", tc.query, envelope.Message, err, tc.want)
		}
	}
	if w := requestOpenAPITest(t, h, http.MethodGet, "/v1/postgres?ownerId=tea-x&limit=5", "", ""); w.Code != http.StatusNoContent {
		t.Fatalf("declared params = %d, want pass-through: %s", w.Code, w.Body.String())
	}
}

// protectedConfirmRoutes is every Render-shaped route whose handler reads
// ?confirm= (a protected-environment guard or a data-loss phrase), with its
// Render operation. Kept by hand from the handlers, not derived from
// renderQueryExtensions, so a missing allowlist entry fails (w4/362).
var protectedConfirmRoutes = []struct{ operationID, pattern, path, body string }{
	{"update-service", "PATCH /v1/services/{id}", "/v1/services/srv-c185th5c2rvvnhbfiltg", `{"name":"web"}`},
	{"delete-service", "DELETE /v1/services/{id}", "/v1/services/srv-c185th5c2rvvnhbfiltg", ""},
	{"create-deploy", "POST /v1/services/{id}/deploys", "/v1/services/srv-c185th5c2rvvnhbfiltg/deploys", `{"imageUrl":"docker.io/traefik/whoami:v1.10.1"}`},
	{"rollback-deploy", "POST /v1/services/{id}/rollback", "/v1/services/srv-c185th5c2rvvnhbfiltg/rollback", `{"deployId":"dep-c185th5c2rvvnhbfiltg"}`},
	{"suspend-service", "POST /v1/services/{id}/suspend", "/v1/services/srv-c185th5c2rvvnhbfiltg/suspend", ""},
	{"resume-service", "POST /v1/services/{id}/resume", "/v1/services/srv-c185th5c2rvvnhbfiltg/resume", ""},
	{"restart-service", "POST /v1/services/{id}/restart", "/v1/services/srv-c185th5c2rvvnhbfiltg/restart", ""},
	{"update-key-value", "PATCH /v1/key-value/{id}", "/v1/key-value/red-c185th5c2rvvnhbfiltg", `{"name":"cache"}`},
	{"delete-key-value", "DELETE /v1/key-value/{id}", "/v1/key-value/red-c185th5c2rvvnhbfiltg", ""},
	{"suspend-key-value", "POST /v1/key-value/{id}/suspend", "/v1/key-value/red-c185th5c2rvvnhbfiltg/suspend", ""},
	{"resume-key-value", "POST /v1/key-value/{id}/resume", "/v1/key-value/red-c185th5c2rvvnhbfiltg/resume", ""},
	{"update-postgres", "PATCH /v1/postgres/{id}", "/v1/postgres/dpg-c185th5c2rvvnhbfiltg", `{"name":"db"}`},
	{"delete-postgres", "DELETE /v1/postgres/{id}", "/v1/postgres/dpg-c185th5c2rvvnhbfiltg", ""},
	{"suspend-postgres", "POST /v1/postgres/{id}/suspend", "/v1/postgres/dpg-c185th5c2rvvnhbfiltg/suspend", ""},
	{"resume-postgres", "POST /v1/postgres/{id}/resume", "/v1/postgres/dpg-c185th5c2rvvnhbfiltg/resume", ""},
	{"restart-postgres", "POST /v1/postgres/{id}/restart", "/v1/postgres/dpg-c185th5c2rvvnhbfiltg/restart", ""},
	{"failover-postgres", "POST /v1/postgres/{id}/failover", "/v1/postgres/dpg-c185th5c2rvvnhbfiltg/failover", ""},
}

// TestRenderValidatorAdmitsProtectedConfirm: every guarded Render route's
// refusal says "retry with confirm=…", so the validator must let ?confirm=
// reach the handler. PATCH /services/{id} (w4/m176) and the Key Value and
// Postgres update and failover routes (w4/362) were each rejected as an
// unsupported query parameter before.
func TestRenderValidatorAdmitsProtectedConfirm(t *testing.T) {
	mux := http.NewServeMux()
	for _, route := range protectedConfirmRoutes {
		mux.HandleFunc(route.pattern, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	}
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, route := range protectedConfirmRoutes {
		listed[route.operationID] = true
		method, _, _ := strings.Cut(route.pattern, " ")
		w := requestOpenAPITest(t, h, method, route.path+"?confirm=sudo+repoint+service+web", "", route.body)
		if w.Code != http.StatusNoContent {
			t.Errorf("%s ?confirm= = %d %s, want it to reach the handler", route.pattern, w.Code, w.Body.String())
		}
	}
	// The allowlist and the table name the same operations: confirm allowed
	// where no handler reads it is an undocumented extension.
	for operationID, extensions := range renderQueryExtensions {
		if _, ok := extensions["confirm"]; ok && !listed[operationID] {
			t.Errorf("%s allows confirm but no route in protectedConfirmRoutes reads it", operationID)
		}
	}
}

// TestEveryConfirmReaderIsListed counts the call sites in REST handlers that
// read ?confirm=. A new one must add its route to protectedConfirmRoutes (and
// confirm to its Render operation in renderQueryExtensions), then this count.
// Its limit: a new route served by an existing call site (one more verb on
// restVerb, the Postgres verb or the Key Value lifecycle helper) adds none, so
// a route like that must be listed by whoever adds it. Two bex-only routes
// read confirm and are never validated: PUT /v1/key-value/{id}/ip-allow-list
// and POST /v1/postgres/{id}/query.
func TestEveryConfirmReaderIsListed(t *testing.T) {
	const callSites = 14
	reader := regexp.MustCompile(`(?:\.Get|FormValue)\("confirm"\)`)
	got := 0
	walkInternalSources(t, func(source string) {
		for _, line := range strings.Split(source, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "//") {
				got += len(reader.FindAllString(line, -1))
			}
		}
	})
	if got != callSites {
		t.Fatalf("%d call sites read ?confirm=, want %d: list the new route in protectedConfirmRoutes, allow confirm on its Render operation in renderQueryExtensions, then update this count", got, callSites)
	}
}

func TestRegistryCredentialWorkspaceQueryPassesRenderValidator(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			var seen string
			mux := http.NewServeMux()
			mux.HandleFunc(method+" /v1/registrycredentials/{registryCredentialId}", func(w http.ResponseWriter, r *http.Request) {
				seen = r.URL.Query().Get("ownerId")
				w.WriteHeader(http.StatusNoContent)
			})
			handler, err := newRenderRequestValidator(mux)
			if err != nil {
				t.Fatal(err)
			}
			body, contentType := "", ""
			if method == http.MethodPatch {
				body, contentType = `{"name":"renamed","registry":"DOCKER","username":"user","authToken":"synthetic"}`, "application/json"
			}
			result := requestOpenAPITest(t, handler, method, "/v1/registrycredentials/rcr-00000000000000000000?ownerId=tea-bravo", contentType, body)
			if result.Code != http.StatusNoContent || seen != "tea-bravo" {
				t.Fatalf("workspace selector: status=%d seen=%q body=%s", result.Code, seen, result.Body.String())
			}
		})
	}
}

// POST and PATCH /v1/registrycredentials reach the real handler through the
// Render request validator in bex's own dialect (w8/135). The pinned schema
// demanded Render's `registry` enum and every field on a PATCH, while the
// strict handler refuses `registry` as unknown, so no REST body could create
// or rename a credential.
func TestRegistryCredentialRESTWritesPassTheRenderValidator(t *testing.T) {
	rc := &registrycreds.Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}, Store: newFakeRCStore(), Secret: newFakeRCSecretKV()}
	mux := http.NewServeMux()
	rc.RegisterREST(mux)
	handler, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	created := requestOpenAPITest(t, handler, http.MethodPost, "/v1/registrycredentials", "application/json",
		`{"name":"ghcr","host":"ghcr.io","username":"alice","authToken":"hunter2","expiresAt":"2099-01-01T00:00:00Z"}`)
	var credential struct{ ID, Name, Host string }
	if err := json.Unmarshal(created.Body.Bytes(), &credential); created.Code != http.StatusCreated || err != nil || credential.Host != "ghcr.io" {
		t.Fatalf("create = %d %s, want 201", created.Code, created.Body.String())
	}
	if strings.Contains(created.Body.String(), "hunter2") {
		t.Fatalf("create echoed the secret: %s", created.Body.String())
	}
	renamed := requestOpenAPITest(t, handler, http.MethodPatch, "/v1/registrycredentials/"+credential.ID, "application/json", `{"name":"renamed"}`)
	if renamed.Code != http.StatusOK || !strings.Contains(renamed.Body.String(), `"name":"renamed"`) {
		t.Fatalf("rename = %d %s, want 200 with the new name", renamed.Code, renamed.Body.String())
	}
	// Render's `registry` enum is not bex's wire; it is refused by name, not
	// with a bare bad request.
	refused := requestOpenAPITest(t, handler, http.MethodPost, "/v1/registrycredentials", "application/json",
		`{"name":"x","registry":"DOCKER","username":"u","authToken":"t","ownerId":"tea-a"}`)
	if refused.Code != http.StatusBadRequest || !strings.Contains(refused.Body.String(), "registry") {
		t.Fatalf("registry body = %d %s, want a 400 naming the field", refused.Code, refused.Body.String())
	}
}
