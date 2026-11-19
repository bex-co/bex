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

// TestReadReplicaStaleHydratesRenderDetails verifies w4/m511's read contract:
// a postgres_read_replica_stale event hydrates by its evt- id identically on
// REST, GraphQL and MCP with Render's empty details ({}) and the database's
// dpg- id.
func TestReadReplicaStaleHydratesRenderDetails(t *testing.T) {
	dbID := ids.New(ids.Postgres)
	row := store.ServiceEventRow{
		Key: "fact:" + dbID + ":replica_stale:reader:1795089600000000000", At: now,
		Source: store.EventSourceFact, FactType: TypePostgresReadReplicaStale,
	}
	eventID := ids.Derive(ids.Event, row.Key)
	newSvc := func() *Service {
		svc := newService(&hydrationStore{fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: dbID}}})
		svc.Workspace = hydrationWorkspaces{}
		return svc
	}
	ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "user", Method: "session"})

	t.Run("REST", func(t *testing.T) {
		mux := http.NewServeMux()
		newSvc().RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/events/"+eventID, nil).WithContext(ctx))
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var body struct {
			ID        string         `json:"id"`
			ServiceID string         `json:"serviceId"`
			Type      string         `json:"type"`
			Details   map[string]any `json:"details"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.ID != eventID || body.ServiceID != dbID || body.Type != TypePostgresReadReplicaStale || len(body.Details) != 0 {
			t.Fatalf("event = %s, want id=%s serviceId=%s type=%s details={}", rec.Body, eventID, dbID, TypePostgresReadReplicaStale)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: newSvc().GraphQLQuery()})})
		if err != nil {
			t.Fatal(err)
		}
		result := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
			RequestString: fmt.Sprintf(`{serviceEvent(id:%q){id serviceId type details{leaderId reasonCode}}}`, eventID)})
		if len(result.Errors) > 0 {
			t.Fatal(result.Errors)
		}
		got, _ := json.Marshal(result.Data)
		want := fmt.Sprintf(`{"serviceEvent":{"details":{"leaderId":"","reasonCode":""},"id":%q,"serviceId":%q,"type":"postgres_read_replica_stale"}}`, eventID, dbID)
		if string(got) != want {
			t.Fatalf("graphql = %s, want %s", got, want)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		server := mcp.NewServer(&mcp.Implementation{Name: "replica-stale", Version: "0"}, nil)
		newSvc().RegisterMCP(server)
		a, b := mcp.NewInMemoryTransports()
		if _, err := server.Connect(ctx, a, nil); err != nil {
			t.Fatal(err)
		}
		session, err := mcp.NewClient(&mcp.Implementation{Name: "replica-stale", Version: "0"}, nil).Connect(ctx, b, nil)
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
				ID        string         `json:"id"`
				ServiceID string         `json:"serviceId"`
				Type      string         `json:"type"`
				Details   map[string]any `json:"details"`
			} `json:"event"`
		}
		if err := json.Unmarshal(payload, &out); err != nil {
			t.Fatal(err)
		}
		if out.Event.ID != eventID || out.Event.ServiceID != dbID || out.Event.Type != TypePostgresReadReplicaStale || len(out.Event.Details) != 0 {
			t.Fatalf("mcp = %s, want id=%s serviceId=%s type=%s details={}", payload, eventID, dbID, TypePostgresReadReplicaStale)
		}
	})
}
