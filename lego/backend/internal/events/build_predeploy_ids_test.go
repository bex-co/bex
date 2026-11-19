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

// w4/710: build_started/build_ended send Render's required buildId (the
// deploy's dep- id for a service build, or its bld- id for a build-source
// build), deploy_started/deploy_ended send buildId when the deploy shipped a
// build-source build and omit it otherwise, and pre_deploy_started/_ended send
// Render's required deployCommandExecutionId as the deploy id — identically on
// REST, GraphQL and MCP.
func TestBuildAndPreDeployEventsCarryTheirIDs(t *testing.T) {
	serviceID := ids.New(ids.Service)
	carrierID := ids.New(ids.BuildSource)
	deployID := ids.New(ids.Deploy)
	supersededBy := ids.New(ids.Deploy)
	carrierBuildID, _ := ids.Rekind(ids.Build, deployID)
	supersededBuildID, _ := ids.Rekind(ids.Build, supersededBy)
	shippedBuildID := ids.New(ids.Build)

	for _, tc := range []struct {
		name              string
		serviceID         string
		row               store.ServiceEventRow
		wantBuildID       string
		wantPreDeployExec string
		wantNewBuildID    string
	}{
		{
			name:        "service build_started",
			serviceID:   serviceID,
			row:         store.ServiceEventRow{Key: "fact:deploy:" + deployID + ":build_started", At: now, Source: store.EventSourceFact, FactType: TypeBuildStarted, DeployID: deployID, Trigger: store.TriggerManual},
			wantBuildID: deployID,
		},
		{
			name:        "service build_ended",
			serviceID:   serviceID,
			row:         store.ServiceEventRow{Key: "fact:deploy:" + deployID + ":build_ended", At: now, Source: store.EventSourceFact, FactType: TypeBuildEnded, DeployID: deployID, FactStatus: store.EventStatusSucceeded},
			wantBuildID: deployID,
		},
		{
			name:        "build-source build_started",
			serviceID:   carrierID,
			row:         store.ServiceEventRow{Key: "fact:deploy:" + deployID + ":build_started", At: now, Source: store.EventSourceFact, FactType: TypeBuildStarted, DeployID: deployID, Trigger: store.TriggerAPI},
			wantBuildID: carrierBuildID,
		},
		{
			name:           "build-source build_ended superseded",
			serviceID:      carrierID,
			row:            store.ServiceEventRow{Key: "fact:deploy:" + deployID + ":build_ended", At: now, Source: store.EventSourceFact, FactType: TypeBuildEnded, DeployID: deployID, FactStatus: store.EventStatusCanceled, CancelReason: "Superseded by " + supersededBy},
			wantBuildID:    carrierBuildID,
			wantNewBuildID: supersededBuildID,
		},
		{
			name:        "build-source pipeline_minutes_exhausted",
			serviceID:   carrierID,
			row:         store.ServiceEventRow{Key: "fact:deploy:" + deployID + ":pipeline_minutes_exhausted", At: now, Source: store.EventSourceFact, FactType: TypePipelineMinutesExhausted, DeployID: deployID, Trigger: store.TriggerAPI},
			wantBuildID: carrierBuildID,
		},
		{
			name:        "build-source deploy_started",
			serviceID:   serviceID,
			row:         store.ServiceEventRow{Key: deployID + ":started", At: now, Source: store.EventSourceDeploy, Phase: store.EventPhaseStarted, DeployID: deployID, DeployBuildID: shippedBuildID, Trigger: store.TriggerAPI},
			wantBuildID: shippedBuildID,
		},
		{
			name:        "build-source deploy_ended",
			serviceID:   serviceID,
			row:         store.ServiceEventRow{Key: deployID + ":ended", At: now, Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, DeployID: deployID, DeployBuildID: shippedBuildID, Status: store.DeployLive},
			wantBuildID: shippedBuildID,
		},
		{
			name:      "non-build-source deploy_ended omits buildId",
			serviceID: serviceID,
			row:       store.ServiceEventRow{Key: deployID + ":ended-plain", At: now, Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded, DeployID: deployID, Status: store.DeployLive},
		},
		{
			name:              "pre_deploy_started",
			serviceID:         serviceID,
			row:               store.ServiceEventRow{Key: "fact:deploy:" + deployID + ":pre_deploy_started", At: now, Source: store.EventSourceFact, FactType: TypePreDeployStarted, DeployID: deployID},
			wantPreDeployExec: deployID,
		},
		{
			name:              "pre_deploy_ended",
			serviceID:         serviceID,
			row:               store.ServiceEventRow{Key: "fact:deploy:" + deployID + ":pre_deploy_ended", At: now, Source: store.EventSourceFact, FactType: TypePreDeployEnded, DeployID: deployID, FactStatus: store.EventStatusSucceeded},
			wantPreDeployExec: deployID,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eventID := ids.Derive(ids.Event, tc.row.Key)
			newSvc := func() *Service {
				svc := newService(&hydrationStore{fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: tc.row, ServiceID: tc.serviceID}}},
					sampleApp("web", serviceID, "tea-b"))
				svc.Workspace = hydrationWorkspaces{}
				return svc
			}
			ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "user", Method: "session"})

			checkMap := func(t *testing.T, surface string, details map[string]any, strictAbsence bool) {
				t.Helper()
				if tc.wantBuildID == "" {
					if v, ok := details["buildId"]; strictAbsence && ok {
						t.Errorf("%s buildId = %v, want key omitted", surface, v)
					} else if v != nil && v != "" {
						t.Errorf("%s buildId = %v, want empty", surface, v)
					}
				} else if details["buildId"] != tc.wantBuildID {
					t.Errorf("%s buildId = %v, want %q", surface, details["buildId"], tc.wantBuildID)
				}
				if tc.wantPreDeployExec == "" {
					if v, ok := details["deployCommandExecutionId"]; strictAbsence && ok {
						t.Errorf("%s deployCommandExecutionId = %v, want key omitted", surface, v)
					} else if v != nil && v != "" {
						t.Errorf("%s deployCommandExecutionId = %v, want empty", surface, v)
					}
				} else if details["deployCommandExecutionId"] != tc.wantPreDeployExec {
					t.Errorf("%s deployCommandExecutionId = %v, want %q", surface, details["deployCommandExecutionId"], tc.wantPreDeployExec)
				}
				if tc.wantNewBuildID != "" {
					reason, _ := details["reason"].(map[string]any)
					nb, _ := reason["newBuild"].(map[string]any)
					if nb["id"] != tc.wantNewBuildID {
						t.Errorf("%s reason.newBuild.id = %v, want %q", surface, nb["id"], tc.wantNewBuildID)
					}
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
					Details map[string]any `json:"details"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				checkMap(t, "REST", body.Details, true)
			})

			t.Run("GraphQL", func(t *testing.T) {
				schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: newSvc().GraphQLQuery()})})
				if err != nil {
					t.Fatal(err)
				}
				result := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
					RequestString: fmt.Sprintf(`{serviceEvent(id:%q){details{deployId buildId deployCommandExecutionId reason{newBuild{id}}}}}`, eventID)})
				if len(result.Errors) > 0 {
					t.Fatal(result.Errors)
				}
				raw, _ := json.Marshal(result.Data)
				var out struct {
					ServiceEvent struct {
						Details map[string]any `json:"details"`
					} `json:"serviceEvent"`
				}
				if err := json.Unmarshal(raw, &out); err != nil {
					t.Fatal(err)
				}
				checkMap(t, "GraphQL", out.ServiceEvent.Details, false)
			})

			t.Run("MCP", func(t *testing.T) {
				server := mcp.NewServer(&mcp.Implementation{Name: "events", Version: "0"}, nil)
				newSvc().RegisterMCP(server)
				a, b := mcp.NewInMemoryTransports()
				if _, err := server.Connect(ctx, a, nil); err != nil {
					t.Fatal(err)
				}
				session, err := mcp.NewClient(&mcp.Implementation{Name: "events", Version: "0"}, nil).Connect(ctx, b, nil)
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
						Details map[string]any `json:"details"`
					} `json:"event"`
				}
				if err := json.Unmarshal(payload, &out); err != nil {
					t.Fatal(err)
				}
				checkMap(t, "MCP", out.Event.Details, true)
			})
		})
	}
}
