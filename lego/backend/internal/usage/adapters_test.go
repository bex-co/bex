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
		RequestString: `{ usage { workspaceId services { serviceId rows { kind tier total } } } }`,
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
