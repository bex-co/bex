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

// TestServerFailedUnhealthyAgreesOnEverySurface is w6/m634: one server_failed
// event whose failing health check recorded a result reads it, in Render's
// words, as failureReason.unhealthy on REST and MCP and as unhealthy on
// GraphQL. With none recorded, all three read the generic sentence. A
// webhook's receiver reads the same event by id, through the same lookup.
func TestServerFailedUnhealthyAgreesOnEverySurface(t *testing.T) {
	for _, tc := range []struct {
		name, result, want string
	}{
		{"a 503", "HTTP health check failed with status code 503", "HTTP health check failed with status code 503"},
		{"a timeout", "HTTP health check failed (timed out after 5 seconds)", "HTTP health check failed (timed out after 5 seconds)"},
		{"no listener", "HTTP health check failed: dial tcp 10.0.0.7:8080: connect: connection refused",
			"HTTP health check failed: dial tcp 10.0.0.7:8080: connect: connection refused"},
		{"no result recorded", "", unhealthyMessage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := store.ServiceEventRow{
				Key: "fact:observed:app-1:failed:" + tc.name, At: now, Source: store.EventSourceFact, FactType: TypeServerFailed,
				ReasonCode: store.EventReasonReadinessFailed, InstanceID: "web-7d9f5c-abcde", HealthCheckResult: tc.result,
			}
			eventID := ids.Derive(ids.Event, row.Key)
			newSvc := func() *Service {
				svc := newService(&hydrationStore{fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: ids.New(ids.Service)}}})
				svc.Workspace = hydrationWorkspaces{}
				return svc
			}
			ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "user", Method: "session"})
			type failure struct {
				Details struct {
					Reason struct {
						Unhealthy string `json:"unhealthy"`
					} `json:"reason"`
				} `json:"details"`
			}

			mux := http.NewServeMux()
			newSvc().RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/events/"+eventID, nil).WithContext(ctx))
			var rest failure
			if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("REST status %d: %s (%v)", rec.Code, rec.Body, err)
			}
			if rest.Details.Reason.Unhealthy != tc.want {
				t.Errorf("REST unhealthy = %q, want %q", rest.Details.Reason.Unhealthy, tc.want)
			}

			schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: newSvc().GraphQLQuery()})})
			if err != nil {
				t.Fatal(err)
			}
			result := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: fmt.Sprintf(`{serviceEvent(id:%q){details{reasonCode unhealthy}}}`, eventID)})
			if len(result.Errors) > 0 {
				t.Fatal(result.Errors)
			}
			details := result.Data.(map[string]any)["serviceEvent"].(map[string]any)["details"].(map[string]any)
			if details["unhealthy"] != tc.want || details["reasonCode"] != store.EventReasonReadinessFailed {
				t.Errorf("GraphQL details = %v, want unhealthy %q", details, tc.want)
			}

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
			called, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_service_event", Arguments: map[string]any{"id": eventID}})
			if err != nil || called.IsError {
				t.Fatalf("MCP call: %v %+v", err, called)
			}
			payload, _ := json.Marshal(called.StructuredContent)
			var out struct {
				Event failure `json:"event"`
			}
			if err := json.Unmarshal(payload, &out); err != nil || out.Event.Details.Reason.Unhealthy != tc.want {
				t.Errorf("MCP = %s (%v), want unhealthy %q", payload, err, tc.want)
			}
		})
	}
}

// Only server_failed's readiness failure carries a result: a GraphQL read of
// any other event's unhealthy is null.
func TestUnhealthyIsNullOffAFailedHealthCheck(t *testing.T) {
	exit := int32(3)
	row := store.ServiceEventRow{Key: "fact:exit:crash", At: now, Source: store.EventSourceFact, FactType: TypeServerFailed,
		ReasonCode: store.EventReasonNonZeroExit, FactExitCode: &exit}
	eventID := ids.Derive(ids.Event, row.Key)
	svc := newService(&hydrationStore{fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: ids.New(ids.Service)}}})
	svc.Workspace = hydrationWorkspaces{}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "user", Method: "session"})
	result := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: fmt.Sprintf(`{serviceEvent(id:%q){details{unhealthy exitCode}}}`, eventID)})
	if len(result.Errors) > 0 {
		t.Fatal(result.Errors)
	}
	details := result.Data.(map[string]any)["serviceEvent"].(map[string]any)["details"].(map[string]any)
	if details["unhealthy"] != nil {
		t.Fatalf("a crash's unhealthy = %v, want null", details["unhealthy"])
	}
}
