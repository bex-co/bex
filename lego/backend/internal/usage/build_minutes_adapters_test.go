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

package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m308 (docs/ADR030 §13): every usage surface reports the month's build
// minutes against the plan's included minutes, and the estimate charges only
// those above.
func TestBuildMinutesAgreeAcrossSurfacesAndCutTheEstimate(t *testing.T) {
	st := newMemUsageStore(store.App{ID: "srv-001", TenantID: "tea-001", Name: "myapp", Tier: "starter"})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-001", ServiceID: "srv-001", Kind: store.UsageKindBuildSeconds, ResourceKind: store.ResourceKindService,
		WindowStart: time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC), Quantity: 600 * 60,
	})
	svc := svcWithTenant(st, "tea-001")
	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return store.PlanHobby, nil })
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/v1/usage", nil).WithContext(ctx))
	if w.Code != http.StatusOK {
		t.Fatalf("REST: %d %s", w.Code, w.Body.String())
	}
	var rest usageResponse
	if err := json.NewDecoder(w.Body).Decode(&rest); err != nil {
		t.Fatal(err)
	}
	if rest.BuildMinutes.Used != 600 || rest.BuildMinutes.Included != 500 {
		t.Errorf("REST buildMinutes = %+v, want 600 used of 500 included", rest.BuildMinutes)
	}
	if rest.EstimatedCost.TotalUSD != "0.35" {
		t.Errorf("REST estimate = %s, want 0.35 (100 minutes above the 500 at $0.0035)", rest.EstimatedCost.TotalUSD)
	}

	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
		RequestString: `{ usage { buildMinutes { used included } estimatedCost { totalUsd resources { charges { kind quantity included costUsd } } } } }`})
	if len(gql.Errors) > 0 {
		t.Fatalf("GraphQL: %v", gql.Errors)
	}
	usage := gql.Data.(map[string]any)["usage"].(map[string]any)
	bm := usage["buildMinutes"].(map[string]any)
	if bm["used"] != rest.BuildMinutes.Used || bm["included"] != rest.BuildMinutes.Included {
		t.Errorf("GraphQL buildMinutes = %v, want REST's %+v", bm, rest.BuildMinutes)
	}
	charge := usage["estimatedCost"].(map[string]any)["resources"].([]any)[0].(map[string]any)["charges"].([]any)[0].(map[string]any)
	if charge["included"] != "500.00" || charge["quantity"] != "600.00" {
		t.Errorf("GraphQL build charge line = %v, want 600 used with 500 included", charge)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "usage-test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "usage-test-client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_usage"})
	if err != nil || result.IsError {
		t.Fatalf("MCP get_usage: err=%v result=%+v", err, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	var viaMCP usageResponse
	if err := json.Unmarshal(raw, &viaMCP); err != nil {
		t.Fatal(err)
	}
	if viaMCP.BuildMinutes != rest.BuildMinutes || viaMCP.EstimatedCost.TotalUSD != rest.EstimatedCost.TotalUSD {
		t.Errorf("MCP = %+v / %s, want REST's %+v / %s", viaMCP.BuildMinutes, viaMCP.EstimatedCost.TotalUSD, rest.BuildMinutes, rest.EstimatedCost.TotalUSD)
	}
}
