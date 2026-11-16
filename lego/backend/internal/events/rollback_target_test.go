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

package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/690: a rollback's deploy_started names the deploy it rolled back to as
// Render's trigger.rollbackTargetDeployId on REST get, GraphQL and MCP (the
// list and every MCP read serialize through the same toRenderEvent), and a
// deploy that is not a rollback omits the key.
func TestRollbackDeployStartedNamesItsTarget(t *testing.T) {
	for _, tc := range []struct {
		name       string
		row        store.ServiceEventRow
		wantTarget string // "" means the key must be absent
	}{
		{"a rollback", store.ServiceEventRow{Key: "dep-rb:started", At: now, Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted,
			DeployID: "dep-rb", Trigger: store.TriggerRollback, RollbackOf: "dep-first", Caller: "user-x"}, "dep-first"},
		{"an API deploy", store.ServiceEventRow{Key: "dep-api:started", At: now, Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted,
			DeployID: "dep-api", Trigger: store.TriggerAPI, Caller: "user-x"}, ""},
		{"the first deploy", store.ServiceEventRow{Key: "dep-first:started", At: now, Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted,
			DeployID: "dep-first", Trigger: store.TriggerCreate}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eventID := ids.Derive(ids.Event, tc.row.Key)
			newSvc := func() *Service {
				svc := newService(&hydrationStore{fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: tc.row, ServiceID: "srv-1"}}},
					sampleApp("web", "srv-1", "tea-b"))
				svc.Workspace = hydrationWorkspaces{}
				return svc
			}
			ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "user", Method: "session"})
			check := func(t *testing.T, surface string, trigger map[string]any) {
				t.Helper()
				if trigger["rollback"] != (tc.wantTarget != "") {
					t.Errorf("%s trigger.rollback = %v, want %v", surface, trigger["rollback"], tc.wantTarget != "")
				}
				got, present := trigger["rollbackTargetDeployId"]
				switch {
				case tc.wantTarget == "" && present && got != nil:
					t.Errorf("%s trigger.rollbackTargetDeployId = %v, want it absent", surface, got)
				case tc.wantTarget != "" && got != tc.wantTarget:
					t.Errorf("%s trigger.rollbackTargetDeployId = %v, want %s", surface, got, tc.wantTarget)
				}
			}

			t.Run("REST", func(t *testing.T) {
				mux := http.NewServeMux()
				newSvc().RegisterREST(mux)
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/events/"+eventID, nil).WithContext(ctx))
				if rec.Code != http.StatusOK {
					t.Fatalf("status %d: %s", rec.Code, rec.Body)
				}
				var body struct {
					Type    string `json:"type"`
					Details struct {
						Trigger map[string]any `json:"trigger"`
					} `json:"details"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Type != TypeDeployStarted {
					t.Fatalf("type = %q, want %s", body.Type, TypeDeployStarted)
				}
				if _, present := body.Details.Trigger["rollbackTargetDeployId"]; tc.wantTarget == "" && present {
					t.Errorf("REST sends rollbackTargetDeployId for a non-rollback: %s", rec.Body)
				}
				check(t, "REST", body.Details.Trigger)
			})

			t.Run("GraphQL", func(t *testing.T) {
				schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: newSvc().GraphQLQuery()})})
				if err != nil {
					t.Fatal(err)
				}
				result := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
					RequestString: fmt.Sprintf(`{serviceEvent(id:%q){details{trigger{rollback rollbackTargetDeployId}}}}`, eventID)})
				if len(result.Errors) > 0 {
					t.Fatal(result.Errors)
				}
				raw, _ := json.Marshal(result.Data)
				var out struct {
					ServiceEvent struct {
						Details struct {
							Trigger map[string]any `json:"trigger"`
						} `json:"details"`
					} `json:"serviceEvent"`
				}
				if err := json.Unmarshal(raw, &out); err != nil {
					t.Fatal(err)
				}
				check(t, "GraphQL", out.ServiceEvent.Details.Trigger)
			})

			t.Run("MCP", func(t *testing.T) {
				server := mcp.NewServer(&mcp.Implementation{Name: "rollback", Version: "0"}, nil)
				newSvc().RegisterMCP(server)
				a, b := mcp.NewInMemoryTransports()
				if _, err := server.Connect(ctx, a, nil); err != nil {
					t.Fatal(err)
				}
				session, err := mcp.NewClient(&mcp.Implementation{Name: "rollback", Version: "0"}, nil).Connect(ctx, b, nil)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = session.Close() })
				result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_service_event", Arguments: map[string]any{"id": eventID}})
				if err != nil || result.IsError {
					t.Fatalf("call: %v %+v", err, result)
				}
				payload, _ := json.Marshal(result.StructuredContent)
				var out struct {
					Event struct {
						Details struct {
							Trigger map[string]any `json:"trigger"`
						} `json:"details"`
					} `json:"event"`
				}
				if err := json.Unmarshal(payload, &out); err != nil {
					t.Fatal(err)
				}
				if _, present := out.Event.Details.Trigger["rollbackTargetDeployId"]; tc.wantTarget == "" && present {
					t.Errorf("MCP sends rollbackTargetDeployId for a non-rollback: %s", payload)
				}
				check(t, "MCP", out.Event.Details.Trigger)
			})
		})
	}
}
