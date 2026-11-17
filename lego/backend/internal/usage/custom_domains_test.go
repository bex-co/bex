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
	"slices"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// countedDomain is one domain the PG store's CountedCustomDomains would
// return: already verified, not a generated www/apex sibling (the SQL's
// filter, covered against Postgres in store's custom domain test).
type countedDomain struct {
	workspace, plan string
	verifiedAt      time.Time
}

func (m *memUsageStore) CountedCustomDomains(_ context.Context, before time.Time, workspaceID string) ([]store.CustomDomainUsage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	byWorkspace := map[string]*store.CustomDomainUsage{}
	var order []string
	for _, d := range m.domains {
		if !d.verifiedAt.Before(before) || (workspaceID != "" && d.workspace != workspaceID) {
			continue
		}
		ws := byWorkspace[d.workspace]
		if ws == nil {
			ws = &store.CustomDomainUsage{WorkspaceID: d.workspace, Plan: d.plan}
			byWorkspace[d.workspace] = ws
			order = append(order, d.workspace)
		}
		ws.VerifiedAt = append(ws.VerifiedAt, d.verifiedAt)
	}
	slices.Sort(order)
	out := make([]store.CustomDomainUsage, 0, len(order))
	for _, w := range order {
		out = append(out, *byWorkspace[w])
	}
	return out, nil
}

func (m *memUsageStore) UsageMeterCursor(_ context.Context, kind string) (time.Time, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	window, ok := m.meterCursors[kind]
	return window, ok, nil
}

func (m *memUsageStore) AdvanceUsageMeterCursor(_ context.Context, kind string, window time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.meterCursors == nil {
		m.meterCursors = map[string]time.Time{}
	}
	if window.After(m.meterCursors[kind]) {
		m.meterCursors[kind] = window
	}
	return nil
}

// domainsOn is n domains of workspace on plan, all verified at verified.
func domainsOn(workspace, plan string, n int, verified time.Time) []countedDomain {
	out := make([]countedDomain, n)
	for i := range out {
		out[i] = countedDomain{workspace: workspace, plan: plan, verifiedAt: verified}
	}
	return out
}

func customDomainRow(st *memUsageStore, workspace string, window time.Time) (store.HourlyRow, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	row, ok := st.rows[usageKey{
		resourceKind: store.ResourceKindCustomDomains, serviceID: workspace,
		kind: store.UsageKindCustomDomainSeconds, windowStart: window,
	}]
	return row, ok
}

// Render's plan table (render.com/pricing, read 2026-11-16): 2, 15 and 25
// custom domains included on Hobby, Pro and Scale, custom on Enterprise. The
// default tenant has no plan, as for every allowance.
func TestCustomDomainsIncludedByPlan(t *testing.T) {
	for _, tc := range []struct {
		workspace, plan string
		included        int
		metered         bool
	}{
		{"tea-1", store.PlanHobby, 2, true},
		{"tea-1", store.PlanPro, 15, true},
		{"tea-1", store.PlanScale, 25, true},
		{"tea-1", store.PlanEnterprise, 0, false},
		{core.DefaultTenant, store.PlanHobby, 0, false},
	} {
		included, metered := includedCustomDomainsFor(tc.workspace, tc.plan)
		if included != tc.included || metered != tc.metered {
			t.Errorf("%s on %s: included=%d metered=%v, want %d %v", tc.workspace, tc.plan, included, metered, tc.included, tc.metered)
		}
	}
}

// The domains above the count are the most recently verified ones, each
// charged from its verification to the end of the hour, by the second.
func TestDomainSecondsAboveProrateWithinTheHour(t *testing.T) {
	from := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	old := from.Add(-48 * time.Hour)
	for _, tc := range []struct {
		name     string
		verified []time.Time
		included int
		want     int64
	}{
		{"within the count", []time.Time{old, old}, 2, 0},
		{"one above all hour", []time.Time{old, old, old}, 2, 3600},
		{"one above from 10:20", []time.Time{old, from.Add(20 * time.Minute), old}, 2, 2400},
		{"two above, verified 10:15 and 10:45", []time.Time{from.Add(45 * time.Minute), old, from.Add(15 * time.Minute)}, 1, 2700 + 900},
		{"verified after the hour", []time.Time{old, old, to}, 2, 0},
		{"none included", []time.Time{old}, 0, 3600},
	} {
		if got := domainSecondsAbove(tc.verified, from, to, tc.included); got != tc.want {
			t.Errorf("%s: %d domain-seconds, want %d", tc.name, got, tc.want)
		}
	}
}

func TestCustomDomainMeterWritesOnlyTheDomainsAboveEachPlansCount(t *testing.T) {
	window := hourWindow(-1)
	old := window.Add(-72 * time.Hour)
	st := newMemUsageStore()
	st.domains = slices.Concat(
		domainsOn("tea-hobby", store.PlanHobby, 3, old),
		domainsOn("tea-pro", store.PlanPro, 15, old),
		domainsOn("tea-pro-over", store.PlanPro, 17, old),
		domainsOn("tea-scale", store.PlanScale, 25, old),
		domainsOn("tea-ent", store.PlanEnterprise, 40, old),
		// The third Hobby domain of this workspace was verified at :30.
		domainsOn("tea-half", store.PlanHobby, 2, old),
		domainsOn("tea-half", store.PlanHobby, 1, window.Add(30*time.Minute)),
	)
	svc := diskMeterService(st, "")
	svc.catchUpCustomDomainsThrough(context.Background(), window)

	for ws, want := range map[string]int64{"tea-hobby": 3600, "tea-pro-over": 7200, "tea-half": 1800} {
		row, ok := customDomainRow(st, ws, window)
		if !ok || row.Quantity != want || row.WorkspaceID != ws || row.Tier != "" {
			t.Errorf("%s row = %+v (present %v), want %d domain-seconds", ws, row, ok, want)
		}
	}
	for _, ws := range []string{"tea-pro", "tea-scale", "tea-ent"} {
		if row, ok := customDomainRow(st, ws, window); ok {
			t.Errorf("%s within its included count (or unlimited) wrote %+v", ws, row)
		}
	}
}

// A first run starts at the newest closed hour: the charge never reaches back
// before the meter existed. Later runs meter each hour once, and a restart
// backfills the hours it missed.
func TestCustomDomainMeterCursor(t *testing.T) {
	first, second, third := hourWindow(-3), hourWindow(-2), hourWindow(-1)
	st := newMemUsageStore()
	st.domains = domainsOn("tea-1", store.PlanHobby, 3, first.Add(-24*time.Hour))
	svc := diskMeterService(st, "")

	svc.catchUpCustomDomainsThrough(context.Background(), first)
	if _, ok := customDomainRow(st, "tea-1", first.Add(-time.Hour)); ok {
		t.Fatal("first run metered an hour before it ran")
	}
	if _, ok := customDomainRow(st, "tea-1", first); !ok {
		t.Fatal("first run did not meter the newest closed hour")
	}

	// The plan changes after `first` was metered: that hour stays as metered.
	for i := range st.domains {
		st.domains[i].plan = store.PlanPro
	}
	svc.catchUpCustomDomainsThrough(context.Background(), first)
	if row, ok := customDomainRow(st, "tea-1", first); !ok || row.Quantity != 3600 {
		t.Fatalf("re-run rewrote a metered hour: %+v", row)
	}

	st.domains = domainsOn("tea-1", store.PlanHobby, 4, first.Add(-24*time.Hour))
	svc.catchUpCustomDomainsThrough(context.Background(), third)
	for _, w := range []time.Time{second, third} {
		if row, ok := customDomainRow(st, "tea-1", w); !ok || row.Quantity != 7200 {
			t.Errorf("missed hour %s = %+v (present %v), want 7200", w, row, ok)
		}
	}
	if got := st.meterCursors[store.UsageKindCustomDomainSeconds]; !got.Equal(third) {
		t.Errorf("cursor = %s, want %s", got, third)
	}
}

// A domain-month above the count is $0.175 (Render's $0.25 × 0.70), quoted in
// domain-months on the charge line.
func TestCustomDomainChargeLine(t *testing.T) {
	const domainMonth = 730 * 3600
	est := pricing.Default.Estimate([]store.UsageSummaryRow{{
		ServiceID: "tea-1", ResourceKind: store.ResourceKindCustomDomains,
		Kind: store.UsageKindCustomDomainSeconds, Total: 10 * domainMonth,
	}})
	if est.TotalUSD != "1.75" {
		t.Errorf("10 domain-months = $%s, want $1.75", est.TotalUSD)
	}
	line := est.Resources[0].Charges[0]
	if line.Unit != "domain-mo" || line.RateUSD != "0.175" || line.Quantity != "10.00" || line.CostUSD != "1.75" {
		t.Errorf("charge line = %+v", line)
	}
}

// GET /v1/usage, GraphQL usage and MCP get_usage report the same count and
// the same priced line (w7/m429).
func TestCustomDomainsAgreeAcrossSurfaces(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) // svcWithTenant's clock
	st := newMemUsageStore()
	st.domains = domainsOn("tea-001", store.PlanHobby, 3, now.Add(-30*24*time.Hour))
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-001", ServiceID: "tea-001", ResourceKind: store.ResourceKindCustomDomains,
		Kind: store.UsageKindCustomDomainSeconds, WindowStart: time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC),
		Quantity: 730 * 3600,
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
	if rest.CustomDomains == nil || *rest.CustomDomains != (CustomDomains{Used: 3, Included: 2}) {
		t.Fatalf("REST customDomains = %+v, want 3 used of 2 included", rest.CustomDomains)
	}
	var restLine *pricing.ResourceEstimate
	for i, r := range rest.EstimatedCost.Resources {
		if r.ResourceKind == store.ResourceKindCustomDomains {
			restLine = &rest.EstimatedCost.Resources[i]
		}
	}
	if restLine == nil || restLine.ServiceName != customDomainsResourceName || restLine.CostUSD != "0.18" ||
		restLine.Charges[0].Kind != store.UsageKindCustomDomainSeconds || restLine.Charges[0].Quantity != "1.00" {
		t.Fatalf("REST custom domain line = %+v, want one domain-month at $0.175", restLine)
	}

	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ usage {
		customDomains { used included unlimited }
		estimatedCost { totalUsd resources { serviceId serviceName resourceKind costUsd charges { kind unit rateUsd quantity costUsd } } }
	} }`})
	if len(gql.Errors) > 0 {
		t.Fatalf("GraphQL: %v", gql.Errors)
	}
	usage := gql.Data.(map[string]any)["usage"].(map[string]any)
	if got := usage["customDomains"]; !reflect.DeepEqual(got, map[string]any{"used": 3, "included": 2, "unlimited": false}) {
		t.Errorf("GraphQL customDomains = %v, want REST's %+v", got, rest.CustomDomains)
	}
	gqlCost := usage["estimatedCost"].(map[string]any)
	if gqlCost["totalUsd"] != rest.EstimatedCost.TotalUSD {
		t.Errorf("GraphQL total %v, REST %s", gqlCost["totalUsd"], rest.EstimatedCost.TotalUSD)
	}
	gqlLine := gqlCost["resources"].([]any)[0].(map[string]any)
	charge := gqlLine["charges"].([]any)[0].(map[string]any)
	if gqlLine["serviceName"] != restLine.ServiceName || gqlLine["costUsd"] != restLine.CostUSD ||
		charge["rateUsd"] != restLine.Charges[0].RateUSD || charge["quantity"] != restLine.Charges[0].Quantity ||
		charge["unit"] != restLine.Charges[0].Unit {
		t.Errorf("GraphQL line = %v, want REST's %+v", gqlLine, restLine)
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
	if !reflect.DeepEqual(viaMCP.CustomDomains, rest.CustomDomains) || !reflect.DeepEqual(viaMCP.EstimatedCost, rest.EstimatedCost) {
		t.Errorf("MCP = %+v / %+v, want REST's %+v / %+v", viaMCP.CustomDomains, viaMCP.EstimatedCost, rest.CustomDomains, rest.EstimatedCost)
	}
}

// The count is the present inventory, so a past month carries the charged line
// in its estimate and no count; Enterprise reports unlimited.
func TestCustomDomainsCountIsCurrentAndUnlimitedOnEnterprise(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	st := newMemUsageStore()
	st.domains = domainsOn("tea-001", store.PlanEnterprise, 40, now.Add(-time.Hour))
	svc := svcWithTenant(st, "tea-001")
	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return store.PlanEnterprise, nil })
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})

	current, err := svc.MonthToDate(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if current.CustomDomains == nil || *current.CustomDomains != (CustomDomains{Used: 40, Unlimited: true}) {
		t.Errorf("Enterprise customDomains = %+v, want 40 used, unlimited", current.CustomDomains)
	}
	june, err := ResolvePeriodEnd("2026-06", now)
	if err != nil {
		t.Fatal(err)
	}
	past, err := svc.monthToDateAt(ctx, "", june)
	if err != nil {
		t.Fatal(err)
	}
	if past.CustomDomains != nil {
		t.Errorf("June's summary carries a count %+v", past.CustomDomains)
	}
}
