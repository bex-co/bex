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
	"net/http"
	"testing"
)

// Render's Terraform provider (render-oss/terraform-provider-render 06af456)
// serializes an unset optional list as JSON null, and its go-vcr recordings
// show Render answering each of these requests 2xx (w9/m472 t002, t004). The
// webhook update has no recording: the provider sends it on every update of a
// webhook with no event_filter, and bex reads it as every event (w9/103). The
// last rows are controls: a wrong list or item type keeps its 400.
func TestRenderValidatorAcceptsTerraformProviderNulls(t *testing.T) {
	const service = `"name":"web","ownerId":"tea-x","repo":"https://github.com/render-examples/express-hello-world"`
	tests := []struct {
		name, method, target, body string
		want                       int
	}{
		{"service envVars and secretFiles", http.MethodPost, "/v1/services",
			`{` + service + `,"type":"web_service","envVars":null,"secretFiles":null}`, http.StatusNoContent},
		{"static site routes", http.MethodPost, "/v1/services",
			`{` + service + `,"type":"static_site","serviceDetails":{"buildCommand":"","publishPath":"public","routes":null}}`, http.StatusNoContent},
		{"postgres create readReplicas", http.MethodPost, "/v1/postgres",
			`{"name":"db","ownerId":"tea-x","plan":"basic_256mb","version":"16","readReplicas":null}`, http.StatusNoContent},
		{"postgres update readReplicas", http.MethodPatch, "/v1/postgres/dpg-db49ak3jrdls73co04p0-a",
			`{"name":"db","readReplicas":null}`, http.StatusNoContent},
		{"webhook create eventFilter", http.MethodPost, "/v1/webhooks",
			`{"enabled":true,"eventFilter":null,"name":"test-tf-webhook","ownerId":"tea-x","url":"https://example.com/h"}`, http.StatusNoContent},
		{"webhook update eventFilter", http.MethodPatch, "/v1/webhooks/whk-d04m9b1r0fns73ckp94f",
			`{"enabled":true,"eventFilter":null,"name":"renamed","url":"https://example.com/h"}`, http.StatusNoContent},

		{"envVars still a list", http.MethodPost, "/v1/services",
			`{` + service + `,"type":"web_service","envVars":{"key":"A"}}`, http.StatusBadRequest},
		{"readReplicas items still checked", http.MethodPost, "/v1/postgres",
			`{"name":"db","ownerId":"tea-x","plan":"basic_256mb","version":"16","readReplicas":[7]}`, http.StatusBadRequest},
		{"eventFilter items still checked", http.MethodPost, "/v1/webhooks",
			`{"enabled":true,"eventFilter":["not_an_event"],"name":"n","ownerId":"tea-x","url":"https://example.com/h"}`, http.StatusBadRequest},
		{"update eventFilter still a list", http.MethodPatch, "/v1/webhooks/whk-d04m9b1r0fns73ckp94f",
			`{"eventFilter":"deploy_ended"}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			mux := http.NewServeMux()
			count := func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(http.StatusNoContent)
			}
			for _, pattern := range []string{
				"POST /v1/services", "POST /v1/postgres", "PATCH /v1/postgres/{postgresId}", "POST /v1/webhooks",
				"PATCH /v1/webhooks/{webhookId}",
			} {
				mux.HandleFunc(pattern, count)
			}
			h, err := newRenderRequestValidator(mux)
			if err != nil {
				t.Fatal(err)
			}
			w := requestOpenAPITest(t, h, tt.method, tt.target, "application/json", tt.body)
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.want, w.Body.String())
			}
			if wantCalls := map[bool]int{true: 1, false: 0}[tt.want == http.StatusNoContent]; calls != wantCalls {
				t.Errorf("handler calls = %d, want %d", calls, wantCalls)
			}
		})
	}
}

// The nulls are request-only: every component the widened properties point at
// is shared with another request or a response (envVars is PUT /env-vars' own
// body) and stays non-null. update-webhook's eventFilter is create-webhook's
// by $ref, and the webhook object read back must keep a non-null list.
func TestTerraformProviderNullsLeaveSharedSchemasStrict(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	doc := contract.document
	body := func(operationID string) map[string]bool {
		t.Helper()
		_, operation := findRenderOperation(t, doc, operationID)
		nullable := map[string]bool{}
		for name, ref := range operation.RequestBody.Value.Content.Get("application/json").Schema.Value.Properties {
			nullable[name] = ref.Value.Nullable
		}
		return nullable
	}
	for operationID, names := range map[string][]string{
		"create-service":  {"envVars", "secretFiles"},
		"create-postgres": {"readReplicas"},
		"update-postgres": {"readReplicas"},
		"create-webhook":  {"eventFilter"},
		"update-webhook":  {"eventFilter"},
	} {
		got := body(operationID)
		for _, name := range names {
			if !got[name] {
				t.Errorf("%s %s is not nullable", operationID, name)
			}
		}
	}
	_, retrieveWebhook := findRenderOperation(t, doc, "retrieve-webhook")
	if retrieveWebhook.Responses.Status(http.StatusOK).Value.Content.Get("application/json").Schema.Value.Properties["eventFilter"].Value.Nullable {
		t.Error("the webhook object's eventFilter became nullable")
	}
	_, putEnv := findRenderOperation(t, doc, "update-env-vars-for-service")
	if putEnv.RequestBody.Value.Content.Get("application/json").Schema.Value.Nullable {
		t.Error("PUT /env-vars body became nullable")
	}
	if doc.Components.Schemas["readReplicasInput"].Value.Nullable {
		t.Error("shared readReplicasInput became nullable")
	}
	if doc.Components.Schemas["staticSiteDetailsPOST"].Value.Properties["routes"].Value.Nullable {
		t.Error("shared staticSiteDetailsPOST.routes became nullable")
	}
}
