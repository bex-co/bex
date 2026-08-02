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
	"reflect"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// --- helpers ---

// denyAll is a core.Checker that always denies. A non-nil Authz is required to
// make core.Base.AuthorizeOn enforce identity presence (nil Authz is allow-all).
type denyAll struct{}

func (denyAll) Check(_ context.Context, _, _, _ string) (bool, error) { return false, nil }

// clockAt returns a Clock pinned to the given instant.
func clockAt(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// fixedClock returns a Clock pinned to the middle of July 2026.
func fixedClock() func() time.Time {
	return clockAt(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC))
}

// seedStore creates a memUsageStore pre-populated with two usage rows for
// workspace "tea-001" / service "srv-001" in July 2026.
func seedStore() *memUsageStore {
	app := store.App{ID: "srv-001", TenantID: "tea-001", Name: "myapp", Tier: "starter"}
	st := newMemUsageStore(app)
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-001", ServiceID: "srv-001",
		Kind: store.UsageKindInstanceSeconds, Tier: "starter",
		WindowStart: time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC),
		Quantity:    3600,
	})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-001", ServiceID: "srv-001",
		Kind: store.UsageKindEgressBytes, Tier: "",
		WindowStart: time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC),
		Quantity:    1024,
	})
	return st
}

// staticTenant resolves every caller to a fixed tenant id.
type staticTenant struct{ id string }

func (s staticTenant) Tenant(_ context.Context, _ core.Identity) (string, bool) {
	return s.id, true
}

func (s staticTenant) IsMember(_ context.Context, _ core.Identity, tenantID string) (bool, error) {
	return tenantID == s.id, nil
}

// svcWithTenant builds a Service with a workspace resolver so MonthToDate can
// resolve the caller's tenant without a real control-plane store.
func svcWithTenant(st *memUsageStore, tenant string) *Service {
	base := &core.Base{
		Clock:     fixedClock(),
		Workspace: staticTenant{tenant},
	}
	return &Service{Base: base, Store: st}
}

// --- REST adapter tests ---

func TestRESTAdapterReturnsUsageData(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")

	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Attach a dummy identity so Authorize passes (no Authz wired → allow-all).
	req := httptest.NewRequest("GET", "/v1/usage", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user:alice"}))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp usageResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.WorkspaceID != "tea-001" {
		t.Errorf("workspaceId: want tea-001, got %q", resp.WorkspaceID)
	}
	if resp.Period != "2026-07" {
		t.Errorf("period: want 2026-07, got %q", resp.Period)
	}
	if len(resp.Services) != 1 || resp.Services[0].ServiceID != "srv-001" {
		t.Errorf("services: want [srv-001], got %v", resp.Services)
	}
	if resp.Services[0].ServiceName != "myapp" {
		t.Errorf("serviceName: want myapp, got %q", resp.Services[0].ServiceName)
	}
	if len(resp.Services[0].Rows) != 2 {
		t.Errorf("rows: want 2, got %d", len(resp.Services[0].Rows))
	}
}

func TestRESTAdapterAuthzDenied(t *testing.T) {
	// Non-nil Authz enforces identity; no identity in context → ErrForbidden.
	base := &core.Base{Authz: denyAll{}, Workspace: staticTenant{"tea-001"}}
	svc := &Service{Base: base, Store: seedStore()}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest("GET", "/v1/usage", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRESTAdapterStoreOffReturns503(t *testing.T) {
	svc := &Service{Base: &core.Base{}, Store: nil}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest("GET", "/v1/usage", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestRESTPeriodFilter(t *testing.T) {
	// Seed one row in June 2026 and one in July 2026; query for June only.
	app := store.App{ID: "srv-002", TenantID: "tea-002", Name: "papp", Tier: "free"}
	st := newMemUsageStore(app)
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-002", ServiceID: "srv-002",
		Kind: store.UsageKindInstanceSeconds, Tier: "free",
		WindowStart: time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
		Quantity:    7200,
	})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-002", ServiceID: "srv-002",
		Kind: store.UsageKindInstanceSeconds, Tier: "free",
		WindowStart: time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC),
		Quantity:    3600,
	})

	svc := svcWithTenant(st, "tea-002")

	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest("GET", "/v1/usage?period=2026-06", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user:alice"}))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp usageResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.Period != "2026-06" {
		t.Errorf("period: want 2026-06, got %q", resp.Period)
	}
	// Only the June row should appear.
	if len(resp.Services) != 1 || len(resp.Services[0].Rows) != 1 {
		t.Errorf("expected 1 service with 1 June row, got %v", resp.Services)
	}
	if resp.Services[0].Rows[0].Total != 7200 {
		t.Errorf("total: want 7200, got %d", resp.Services[0].Rows[0].Total)
	}
}

// --- GraphQL adapter tests ---

func buildTestSchema(svc *Service) (graphql.Schema, error) {
	query := svc.GraphQLQuery()
	return graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: query}),
	})
}

func TestGraphQLAdapterReturnsUsageData(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")

	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})
	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ usage { workspaceId services { serviceId serviceName rows { kind tier total } } } }`,
		Context:       ctx,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("graphql errors: %v", result.Errors)
	}

	data, _ := result.Data.(map[string]any)
	usageData, _ := data["usage"].(map[string]any)
	if usageData["workspaceId"] != "tea-001" {
		t.Errorf("workspaceId: want tea-001, got %v", usageData["workspaceId"])
	}
	services, _ := usageData["services"].([]any)
	if len(services) != 1 {
		t.Fatalf("services: want 1, got %d", len(services))
	}
	svcData := services[0].(map[string]any)
	if svcData["serviceId"] != "srv-001" {
		t.Errorf("serviceId: want srv-001, got %v", svcData["serviceId"])
	}
	if svcData["serviceName"] != "myapp" {
		t.Errorf("serviceName: want myapp, got %v", svcData["serviceName"])
	}
	rows, _ := svcData["rows"].([]any)
	if len(rows) != 2 {
		t.Errorf("rows: want 2, got %d", len(rows))
	}
}

func TestGraphQLAdapterAuthzDenied(t *testing.T) {
	// Non-nil Authz enforces identity; no identity in context → ErrForbidden as GraphQL error.
	base := &core.Base{Authz: denyAll{}, Workspace: staticTenant{"tea-001"}}
	svc := &Service{Base: base, Store: seedStore()}

	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ usage { workspaceId } }`,
		Context:       context.Background(),
	})
	if len(result.Errors) == 0 {
		t.Error("expected graphql error for denied caller, got none")
	}
}

// seedMixedStore creates a store pre-populated with one App service row, one
// Database row, one KeyValue row, and one sandbox row — all in workspace
// "tea-mix" in July 2026. Used to verify that every resource kind surfaces
// identically across adapters.
func seedMixedStore() *memUsageStore {
	appRow := store.App{ID: "srv-mix", TenantID: "tea-mix", Name: "webapi", Tier: "starter"}
	st := newMemUsageStore(appRow)
	window := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-mix", ServiceID: "srv-mix",
		ResourceKind: store.ResourceKindService,
		Kind:         store.UsageKindInstanceSeconds, Tier: "starter",
		WindowStart: window, Quantity: 3600,
	})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-mix", ServiceID: "mydb",
		ResourceKind: store.ResourceKindPostgres,
		Kind:         store.UsageKindInstanceSeconds, Tier: "basic-256mb",
		WindowStart: window, Quantity: 3600,
	})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-mix", ServiceID: "mydb",
		ResourceKind: store.ResourceKindPostgres,
		Kind:         store.UsageKindStorageGBSeconds,
		WindowStart:  window, Quantity: 2628000,
	})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-mix", ServiceID: "mykv",
		ResourceKind: store.ResourceKindKeyValue,
		Kind:         store.UsageKindInstanceSeconds, Tier: "starter",
		WindowStart: window, Quantity: 3600,
	})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-mix", ServiceID: "mykv",
		ResourceKind: store.ResourceKindKeyValue,
		Kind:         store.UsageKindStorageGBSeconds,
		WindowStart:  window, Quantity: 2628000,
	})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-mix", ServiceID: "os-1",
		ResourceKind: store.ResourceKindSandbox,
		Kind:         store.UsageKindSandboxComputeSeconds, Tier: "starter",
		WindowStart: window, Quantity: 1990800,
	})
	return st
}

func entriesByResource(entries []usageServiceEntry) map[string]usageServiceEntry {
	out := make(map[string]usageServiceEntry, len(entries))
	for _, entry := range entries {
		out[entry.ResourceKind+"/"+entry.ServiceID] = entry
	}
	return out
}

// TestResourceKindSurfacesAcrossAdapters verifies that REST, GraphQL, and MCP
// return identical App, Database, KeyValue, and sandbox entries — the t004 DoD.
func TestResourceKindSurfacesAcrossAdapters(t *testing.T) {
	svc := svcWithTenant(seedMixedStore(), "tea-mix")
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})

	// REST
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest("GET", "/v1/usage", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("REST: status %d: %s", w.Code, w.Body.String())
	}
	var restResp usageResponse
	if err := json.NewDecoder(w.Body).Decode(&restResp); err != nil {
		t.Fatalf("REST decode: %v", err)
	}
	if len(restResp.Services) != 4 {
		t.Fatalf("REST: expected 4 resources (app+db+kv+sandbox), got %d", len(restResp.Services))
	}
	restKinds := map[string]string{}
	for _, svcEntry := range restResp.Services {
		restKinds[svcEntry.ServiceID] = svcEntry.ResourceKind
	}
	if restKinds["srv-mix"] != store.ResourceKindService {
		t.Errorf("REST: srv-mix resourceKind: want %q, got %q", store.ResourceKindService, restKinds["srv-mix"])
	}
	if restKinds["mydb"] != store.ResourceKindPostgres {
		t.Errorf("REST: mydb resourceKind: want %q, got %q", store.ResourceKindPostgres, restKinds["mydb"])
	}
	if restKinds["mykv"] != store.ResourceKindKeyValue {
		t.Errorf("REST: mykv resourceKind: want %q, got %q", store.ResourceKindKeyValue, restKinds["mykv"])
	}
	if restKinds["os-1"] != store.ResourceKindSandbox {
		t.Errorf("REST: os-1 resourceKind: want %q, got %q", store.ResourceKindSandbox, restKinds["os-1"])
	}
	restNames := map[string]string{}
	for _, svcEntry := range restResp.Services {
		restNames[svcEntry.ServiceID] = svcEntry.ServiceName
	}
	// The App resolves through the store; the datastores have no k8s client
	// here, so their names stay empty (presenters fall back to the id).
	if restNames["srv-mix"] != "webapi" {
		t.Errorf("REST: srv-mix serviceName: want webapi, got %q", restNames["srv-mix"])
	}

	// GraphQL — same complete entries, modulo its envelope.
	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	gql := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ usage { services { serviceId serviceName resourceKind rows { kind tier total } } } }`,
		Context:       ctx,
	})
	if len(gql.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", gql.Errors)
	}
	data := gql.Data.(map[string]any)
	usageData := data["usage"].(map[string]any)
	gqlServices := usageData["services"].([]any)
	if len(gqlServices) != 4 {
		t.Fatalf("GraphQL: expected 4 resources, got %d", len(gqlServices))
	}
	gqlEntries := make([]usageServiceEntry, 0, len(gqlServices))
	for _, raw := range gqlServices {
		s := raw.(map[string]any)
		sid, _ := s["serviceId"].(string)
		sname, _ := s["serviceName"].(string)
		rk, _ := s["resourceKind"].(string)
		entry := usageServiceEntry{ServiceID: sid, ServiceName: sname, ResourceKind: rk}
		for _, rawRow := range s["rows"].([]any) {
			row := rawRow.(map[string]any)
			entry.Rows = append(entry.Rows, usageRow{
				Kind:  row["kind"].(string),
				Tier:  row["tier"].(string),
				Total: int64(row["total"].(float64)),
			})
		}
		gqlEntries = append(gqlEntries, entry)
	}
	wantEntries := entriesByResource(restResp.Services)
	if got := entriesByResource(gqlEntries); !reflect.DeepEqual(got, wantEntries) {
		t.Errorf("GraphQL entries differ from REST:\nREST: %+v\nGraphQL: %+v", wantEntries, got)
	}

	// MCP — invoke the actual tool over an in-memory transport and compare its
	// shared JSON response to REST, including rows, tiers, and totals.
	srv := mcp.NewServer(&mcp.Implementation{Name: "usage-test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("MCP server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "usage-test-client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("MCP client connect: %v", err)
	}
	defer cs.Close()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_usage"})
	if err != nil || result.IsError {
		t.Fatalf("MCP get_usage: err=%v result=%+v", err, result)
	}
	rawMCP, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("MCP marshal: %v", err)
	}
	var mcpResp usageResponse
	if err := json.Unmarshal(rawMCP, &mcpResp); err != nil {
		t.Fatalf("MCP decode: %v (%s)", err, rawMCP)
	}
	if got := entriesByResource(mcpResp.Services); !reflect.DeepEqual(got, wantEntries) {
		t.Errorf("MCP entries differ from REST:\nREST: %+v\nMCP: %+v", wantEntries, got)
	}
}

func TestGraphQLStorageTotalSupportsTerabyteMonth(t *testing.T) {
	st := newMemUsageStore()
	window := time.Date(2026, 7, 12, 8, 0, 0, 0, time.UTC)
	const total = int64(2_628_000_000) // 1 TB-month; exceeds GraphQL Int32.
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-large", ServiceID: "db", ResourceKind: store.ResourceKindPostgres,
		Kind: store.UsageKindStorageGBSeconds, WindowStart: window, Quantity: total,
	})
	svc := svcWithTenant(st, "tea-large")
	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{
		Schema: schema, RequestString: `{ usage { services { rows { kind total } } } }`,
		Context: core.WithIdentity(context.Background(), core.Identity{Subject: "user:large"}),
	})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL errors for large storage total: %v", result.Errors)
	}
	rows := result.Data.(map[string]any)["usage"].(map[string]any)["services"].([]any)[0].(map[string]any)["rows"].([]any)
	if got := int64(rows[0].(map[string]any)["total"].(float64)); got != total {
		t.Fatalf("large storage total: want %d, got %d", total, got)
	}
}

// TestGraphQLPeriodArg verifies that the `period` argument is forwarded to the
// store query and that the echoed `period` field matches what was requested.
func TestGraphQLPeriodArg(t *testing.T) {
	// Seed one row in June 2026 and one in July 2026.
	app := store.App{ID: "srv-003", TenantID: "tea-003", Name: "periodapp", Tier: "free"}
	st := newMemUsageStore(app)
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-003", ServiceID: "srv-003",
		Kind: store.UsageKindInstanceSeconds, Tier: "free",
		WindowStart: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC),
		Quantity:    5400,
	})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-003", ServiceID: "srv-003",
		Kind: store.UsageKindInstanceSeconds, Tier: "free",
		WindowStart: time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC),
		Quantity:    3600,
	})

	svc := svcWithTenant(st, "tea-003")
	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:bob"})

	// Query for June — should return 5400 and echo period "2026-06".
	result := graphql.Do(graphql.Params{
		Schema:         schema,
		RequestString:  `query($period: String) { usage(period: $period) { workspaceId period services { serviceId rows { kind total } } } }`,
		VariableValues: map[string]any{"period": "2026-06"},
		Context:        ctx,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("graphql errors: %v", result.Errors)
	}
	data, _ := result.Data.(map[string]any)
	usageData, _ := data["usage"].(map[string]any)
	if usageData["period"] != "2026-06" {
		t.Errorf("period: want 2026-06, got %v", usageData["period"])
	}
	services, _ := usageData["services"].([]any)
	if len(services) != 1 {
		t.Fatalf("services: want 1, got %d", len(services))
	}
	rows, _ := services[0].(map[string]any)["rows"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows: want 1 (June only), got %d", len(rows))
	}
	if total, _ := rows[0].(map[string]any)["total"].(float64); total != 5400 {
		t.Errorf("total: want 5400, got %v", rows[0].(map[string]any)["total"])
	}
}

// TestGraphQLPeriodInResponse verifies that the `period` field on UsageSummary
// echoes the queried month even without an explicit period argument.
func TestGraphQLPeriodInResponse(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")
	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})
	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ usage { period } }`,
		Context:       ctx,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("graphql errors: %v", result.Errors)
	}
	data, _ := result.Data.(map[string]any)
	usageData, _ := data["usage"].(map[string]any)
	// fixedClock is 2026-07-15 so period should be "2026-07".
	if usageData["period"] != "2026-07" {
		t.Errorf("period: want 2026-07, got %v", usageData["period"])
	}
}

// TestRESTAdapterEstimatedCostPresent verifies that the REST response includes a
// non-zero estimatedCost when there is billable usage (seedMixedStore has
// service + postgres + key_value + sandbox rows with known ResourceKind).
func TestRESTAdapterEstimatedCostPresent(t *testing.T) {
	svc := svcWithTenant(seedMixedStore(), "tea-mix")
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	req := httptest.NewRequest("GET", "/v1/usage", nil)
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user:alice"}))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp usageResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.EstimatedCost.TotalUSD == "" {
		t.Error("estimatedCost.totalUsd is empty")
	}
	if resp.EstimatedCost.TotalUSD == "0.00" {
		t.Errorf("estimatedCost.totalUsd: expected non-zero, got %q", resp.EstimatedCost.TotalUSD)
	}
	if resp.EstimatedCost.Meters == nil {
		t.Error("estimatedCost.meters is nil; want non-nil slice")
	}
	if len(resp.EstimatedCost.Meters) != 6 {
		t.Errorf("estimatedCost.meters: want 6, got %d", len(resp.EstimatedCost.Meters))
	}
}

// TestGraphQLAdapterEstimatedCostPresent verifies that the GraphQL surface
// returns estimatedCost fields when queried.
func TestGraphQLAdapterEstimatedCostPresent(t *testing.T) {
	svc := svcWithTenant(seedMixedStore(), "tea-mix")
	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})
	result := graphql.Do(graphql.Params{
		Schema: schema,
		RequestString: `{ usage { estimatedCost {
			totalUsd
			meters { kind tier resourceKind costUsd }
		} } }`,
		Context: ctx,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("graphql errors: %v", result.Errors)
	}
	data, _ := result.Data.(map[string]any)
	usageData, _ := data["usage"].(map[string]any)
	ec, _ := usageData["estimatedCost"].(map[string]any)
	if ec == nil {
		t.Fatal("estimatedCost is nil in GraphQL response")
	}
	totalUsd, _ := ec["totalUsd"].(string)
	if totalUsd == "" || totalUsd == "0.00" {
		t.Errorf("estimatedCost.totalUsd: expected non-zero, got %q", totalUsd)
	}
	meters, _ := ec["meters"].([]any)
	if len(meters) != 6 {
		t.Errorf("estimatedCost.meters: want 6, got %d", len(meters))
	}
}

// --- cross-adapter consistency ---

// TestAdapterConsistency verifies that the REST and GraphQL adapters return the
// same workspaceId and the same number of services for the same store state —
// the "same quantities" guarantee in the DoD.
func TestAdapterConsistency(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})

	// REST
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest("GET", "/v1/usage", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	var restResp usageResponse
	if err := json.NewDecoder(w.Body).Decode(&restResp); err != nil {
		t.Fatalf("REST decode: %v", err)
	}

	// GraphQL
	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	gql := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ usage { workspaceId services { serviceId } } }`,
		Context:       ctx,
	})
	if len(gql.Errors) > 0 {
		t.Fatalf("graphql errors: %v", gql.Errors)
	}
	data := gql.Data.(map[string]any)
	usageData := data["usage"].(map[string]any)
	gqlWorkspaceID := usageData["workspaceId"].(string)
	gqlServices := usageData["services"].([]any)

	if restResp.WorkspaceID != gqlWorkspaceID {
		t.Errorf("workspaceId: REST=%q GraphQL=%q", restResp.WorkspaceID, gqlWorkspaceID)
	}
	if len(restResp.Services) != len(gqlServices) {
		t.Errorf("services count: REST=%d GraphQL=%d", len(restResp.Services), len(gqlServices))
	}
}
