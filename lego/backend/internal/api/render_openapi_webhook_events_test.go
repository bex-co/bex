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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/webhooks"
)

// Every event type bex advertises (GET /v1/webhooks/event-types, GraphQL
// webhookEventTypes) subscribes over REST too. Render's pinned enum lacks the
// bex agent session events, so the 076/077 live run got "invalid request body
// at /eventFilter/0" on REST while GraphQL accepted the same filter.
func TestRenderValidatorAcceptsBexWebhookEventTypes(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		want   int
	}{
		{"agent turn events", `["agent_session_turn_succeeded","agent_session_turn_failed"]`, http.StatusNoContent},
		{"render event", `["deploy_ended"]`, http.StatusNoContent},
		{"legacy alias", `["disk_attached"]`, http.StatusNoContent},
		{"unknown event", `["not_an_event"]`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		for _, op := range []struct{ method, target, body string }{
			{http.MethodPost, "/v1/webhooks", `{"ownerId":"tea-x","name":"n","url":"https://example.com/h","enabled":true,"eventFilter":` + tt.filter + `}`},
			{http.MethodPatch, "/v1/webhooks/whk-db49ak3jrdls73co04p0", `{"eventFilter":` + tt.filter + `}`},
		} {
			t.Run(op.method+" "+tt.name, func(t *testing.T) {
				calls := 0
				mux := http.NewServeMux()
				count := func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.WriteHeader(http.StatusNoContent)
				}
				mux.HandleFunc("POST /v1/webhooks", count)
				mux.HandleFunc("PATCH /v1/webhooks/{webhookId}", count)
				h, err := newRenderRequestValidator(mux)
				if err != nil {
					t.Fatal(err)
				}
				w := requestOpenAPITest(t, h, op.method, op.target, "application/json", op.body)
				if w.Code != tt.want {
					t.Fatalf("status = %d, want %d: %s", w.Code, tt.want, w.Body.String())
				}
			})
		}
	}
	for _, typ := range webhooks.EventTypes {
		if !slices.Contains(webhooks.AcceptedEventTypes(), typ) {
			t.Errorf("AcceptedEventTypes lacks %s", typ)
		}
	}
}

// The widening is request-only: the event type component every events
// response shares keeps Render's enum.
func TestWebhookEventFilterWideningLeavesEventComponentStrict(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	_, get := findRenderOperation(t, contract.document, "retrieve-event")
	typ := get.Responses.Status(http.StatusOK).Value.Content.Get("application/json").Schema.Value.Properties["type"].Value
	if slices.ContainsFunc(typ.Enum, func(v any) bool { return v == webhooks.TypeAgentSessionTurnSucceeded }) {
		t.Fatal("the shared event type component gained a bex extension")
	}
}
