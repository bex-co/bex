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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type hydrationWorkspaces struct{}

func (hydrationWorkspaces) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-a", true
}
func (hydrationWorkspaces) IsMember(_ context.Context, _ core.Identity, workspace string) (bool, error) {
	return workspace == "tea-a" || workspace == "tea-b", nil
}

type hydrationStore struct {
	*fakeStore
	calls int
}

func (s *hydrationStore) GetServiceEvent(ctx context.Context, workspace, eventID string) (store.ServiceEventLookup, error) {
	s.calls++
	if workspace != "tea-b" {
		return store.ServiceEventLookup{}, store.ErrNotFound
	}
	return s.fakeStore.GetServiceEvent(ctx, workspace, eventID)
}

func TestEventHydrationWorkspaceAdapters(t *testing.T) {
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, workspace := range []string{"", "tea-a", "tea-b", "tea-foreign"} {
			t.Run(surface+"/"+workspace, func(t *testing.T) {
				row := store.ServiceEventRow{Key: "fact:workspace-hydration", At: now, Source: store.EventSourceFact, FactType: TypePostgresUnavailable}
				eventID := ids.Derive(ids.Event, row.Key)
				st := &hydrationStore{fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: ids.New(ids.Postgres)}}}
				svc := newService(st)
				svc.Workspace = hydrationWorkspaces{}
				ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "user", Method: "session"})
				accepted := false
				var encoded string
				switch surface {
				case "REST":
					mux := http.NewServeMux()
					svc.RegisterREST(mux)
					target := "/v1/events/" + eventID
					if workspace != "" {
						target += "?ownerId=" + workspace
					}
					req := httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
					rec := httptest.NewRecorder()
					mux.ServeHTTP(rec, req)
					accepted = rec.Code == http.StatusOK
					encoded = rec.Body.String()
					if workspace == "tea-foreign" && rec.Code != http.StatusForbidden {
						t.Fatalf("foreign response:%d %s", rec.Code, encoded)
					}
					if workspace != "tea-foreign" && workspace != "tea-b" && rec.Code != http.StatusNotFound {
						t.Fatalf("wrong/default workspace response:%d %s", rec.Code, encoded)
					}
				case "GraphQL":
					schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
					if err != nil {
						t.Fatal(err)
					}
					ownerArg := ""
					if workspace != "" {
						ownerArg = fmt.Sprintf(",ownerId:%q", workspace)
					}
					query := fmt.Sprintf(`{serviceEvent(id:%q%s){id serviceId type}}`, eventID, ownerArg)
					result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: query})
					accepted = len(result.Errors) == 0
					payload, _ := json.Marshal(result)
					encoded = string(payload)
				case "MCP":
					// The composed MCP middleware binds workspaceId to this context; the tool
					// must preserve that binding instead of replacing it with the default.
					ctx = core.WithWorkspace(ctx, workspace)
					server := mcp.NewServer(&mcp.Implementation{Name: "event-workspace", Version: "0"}, nil)
					svc.RegisterMCP(server)
					a, b := mcp.NewInMemoryTransports()
					if _, err := server.Connect(ctx, a, nil); err != nil {
						t.Fatal(err)
					}
					session, err := mcp.NewClient(&mcp.Implementation{Name: "event-workspace", Version: "0"}, nil).Connect(ctx, b, nil)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = session.Close() })
					result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_service_event", Arguments: map[string]any{"id": eventID}})
					if err != nil {
						t.Fatal(err)
					}
					accepted = !result.IsError
					payload, _ := json.Marshal(result)
					encoded = string(payload)
				}
				if accepted != (workspace == "tea-b") {
					t.Fatalf("accepted=%t workspace=%q:%s", accepted, workspace, encoded)
				}
				if accepted && (!strings.Contains(encoded, eventID) || !strings.Contains(encoded, st.lookup.ServiceID)) {
					t.Fatalf("wrong hydrated resource:%s", encoded)
				}
				if workspace == "tea-foreign" && st.calls != 0 {
					t.Fatal("foreign membership refusal queried eventstore")
				}
				if workspace != "tea-foreign" && st.calls != 1 {
					t.Fatalf("store calls=%d want1", st.calls)
				}
			})
		}
	}
}
