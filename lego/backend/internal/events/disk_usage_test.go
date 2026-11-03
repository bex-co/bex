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
	"reflect"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestDiskUsageEventRendersRenderDetails (w2/m255): a service_disk_usage_high
// event reads on REST, GraphQL and MCP with exactly Render's details —
// diskId, diskName, mountPath, usagePercent, thresholdPercent.
func TestDiskUsageEventRendersRenderDetails(t *testing.T) {
	row := store.ServiceEventRow{
		Key: "fact:disk_usage:dsk-1:high:1", At: now, Source: store.EventSourceFact,
		FactType: TypeServiceDiskUsageHigh,
		DiskUsage: &store.DiskUsageDetails{DiskID: "dsk-1", DiskName: "data", MountPath: "/data", UsagePercent: 85.5, ThresholdPercent: 80},
	}
	eventID := ids.Derive(ids.Event, row.Key)
	newSvc := func() *Service {
		svc := newService(&hydrationStore{fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: ids.New(ids.Service)}}})
		svc.Workspace = hydrationWorkspaces{}
		return svc
	}
	ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "user", Method: "session"})
	want := map[string]any{"diskId": "dsk-1", "diskName": "data", "mountPath": "/data", "usagePercent": 85.5, "thresholdPercent": float64(80)}

	t.Run("REST", func(t *testing.T) {
		mux := http.NewServeMux()
		newSvc().RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/events/"+eventID, nil).WithContext(ctx))
		var body struct {
			Type    string         `json:"type"`
			Details map[string]any `json:"details"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s (%v)", rec.Code, rec.Body, err)
		}
		if body.Type != TypeServiceDiskUsageHigh || !reflect.DeepEqual(body.Details, want) {
			t.Fatalf("event = %s, want details exactly %v", rec.Body, want)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: newSvc().GraphQLQuery()})})
		if err != nil {
			t.Fatal(err)
		}
		result := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
			RequestString: fmt.Sprintf(`{serviceEvent(id:%q){type details{diskId diskName mountPath usagePercent thresholdPercent}}}`, eventID)})
		if len(result.Errors) > 0 {
			t.Fatal(result.Errors)
		}
		got, _ := json.Marshal(result.Data)
		wantJSON := `{"serviceEvent":{"details":{"diskId":"dsk-1","diskName":"data","mountPath":"/data","thresholdPercent":80,"usagePercent":85.5},"type":"service_disk_usage_high"}}`
		if string(got) != wantJSON {
			t.Fatalf("graphql = %s, want %s", got, wantJSON)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		server := mcp.NewServer(&mcp.Implementation{Name: "disk", Version: "0"}, nil)
		newSvc().RegisterMCP(server)
		a, b := mcp.NewInMemoryTransports()
		if _, err := server.Connect(ctx, a, nil); err != nil {
			t.Fatal(err)
		}
		session, err := mcp.NewClient(&mcp.Implementation{Name: "disk", Version: "0"}, nil).Connect(ctx, b, nil)
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
		if err := json.Unmarshal(payload, &out); err != nil || !reflect.DeepEqual(out.Event.Details, want) {
			t.Fatalf("mcp = %s (%v)", payload, err)
		}
	})
}
