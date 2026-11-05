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

package agentsessions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeSpend is the SpendStore over the fake environments and sessions.
type fakeSpend struct {
	*fakeEnvironments
	plan  string
	trust store.BillingTrust
	used  int64
}

func (f *fakeSpend) AgentSpendInputs(_ context.Context, ws string, _ time.Time) (store.AgentSpendInputs, error) {
	return store.AgentSpendInputs{Plan: f.plan, Trust: f.trust, OverrideMicroUSD: f.rows[ws].SpendCapOverrideMicroUSD, MonthToDateMicroUSD: f.used}, nil
}

func spendEnvironmentService(plan string, trust store.BillingTrust) (*Service, *fakeSpend) {
	envs := newFakeEnvironments()
	svc := environmentService("tea-a", envs)
	spend := &fakeSpend{fakeEnvironments: envs, plan: plan, trust: trust}
	svc.Spend = spend
	return svc, spend
}

var paidT1 = store.BillingTrust{LifetimePaidCents: 1000}

// ADR091 D5/D10.1: the environment view carries the spend controls; a T0
// workspace is capped at $10 even on Pro, and a paid one gets its plan cap.
func TestEnvironmentReportsSpendControls(t *testing.T) {
	svc, spend := spendEnvironmentService(store.PlanPro, store.BillingTrust{})
	spend.used = 1_234_567
	got, err := svc.Environment(caller("viewer"), "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	if *got.AgentModelMonthlySpendCapUSD != 10 || *got.AgentModelPlanSpendCapUSD != 200 || *got.TrustTier != "T0" ||
		*got.NextChargeThresholdUSD != 10 || *got.AgentModelSpendMonthToDateUSD != 1.234567 ||
		*got.AgentModelMaxSpendCapUSD != 2000 || got.AgentModelSpendCapOverrideUSD != nil {
		t.Fatalf("T0 Pro view = %+v", got)
	}
	spend.trust = store.BillingTrust{LifetimePaidCents: 200000, PaidCycleInvoices: 6}
	got, _ = svc.Environment(caller("viewer"), "tea-a")
	if *got.AgentModelMonthlySpendCapUSD != 200 || *got.TrustTier != "T4" || got.NextChargeThresholdUSD != nil {
		t.Fatalf("T4 Pro view = %+v", got)
	}
}

// Only an admin writes the cap; above 10× the plan default it is a typed 400
// on every surface; an explicit null clears it back to the plan default.
func TestSpendCapOverrideWrites(t *testing.T) {
	svc, _ := spendEnvironmentService(store.PlanHobby, paidT1)
	for _, subject := range []string{"dev", "viewer", "stranger"} {
		if _, err := svc.UpdateEnvironment(caller(subject), "tea-a", EnvironmentInput{AgentModelMonthlySpendCapUSD: ptr(50.0)}); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s cap write = %v, want forbidden", subject, err)
		}
	}
	for _, bad := range []float64{0, -1, 200.01, 1e9} {
		if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{AgentModelMonthlySpendCapUSD: ptr(bad)}); !errors.Is(err, core.ErrBadRequest) || !isCode(err, SpendCapInvalidCode) {
			t.Errorf("cap %v = %v, want %s", bad, err, SpendCapInvalidCode)
		}
	}
	got, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{AgentModelMonthlySpendCapUSD: ptr(200.0)})
	if err != nil || *got.AgentModelMonthlySpendCapUSD != 200 || *got.AgentModelSpendCapOverrideUSD != 200 {
		t.Fatalf("10x cap = %+v err=%v", got, err)
	}
	// A partial update of another field keeps the override.
	if got, err = svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{NetworkTier: ptr(NetworkTierCustom)}); err != nil || *got.AgentModelMonthlySpendCapUSD != 200 {
		t.Fatalf("other-field update = %+v err=%v", got, err)
	}

	// REST: above 10× is the typed 400; null clears.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	patch := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/agent-environment?ownerId=tea-a", strings.NewReader(body)).WithContext(caller("admin")))
		return rec
	}
	if rec := patch(`{"agentModelMonthlySpendCapUsd":201}`); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), SpendCapInvalidCode) {
		t.Fatalf("REST over 10x = %d %s", rec.Code, rec.Body.String())
	}
	rec := patch(`{"agentModelMonthlySpendCapUsd":null}`)
	var view Environment
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &view) != nil || view.AgentModelSpendCapOverrideUSD != nil || *view.AgentModelMonthlySpendCapUSD != 20 {
		t.Fatalf("REST null = %d %s", rec.Code, rec.Body.String())
	}

	// MCP: set, then null clears; over 10× is the typed error.
	client := newMCPClient(t, caller("admin"), svc)
	call := func(args map[string]any) (*mcp.CallToolResult, Environment) {
		args["ownerId"] = "tea-a"
		res, err := client.CallTool(caller("admin"), &mcp.CallToolParams{Name: "update_agent_environment", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		var out Environment
		if !res.IsError {
			raw, _ := json.Marshal(res.StructuredContent)
			_ = json.Unmarshal(raw, &out)
		}
		return res, out
	}
	if res, _ := call(map[string]any{"agentModelMonthlySpendCapUsd": 500}); !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, SpendCapInvalidCode) {
		t.Fatalf("MCP over 10x = %+v", res)
	}
	if res, out := call(map[string]any{"agentModelMonthlySpendCapUsd": 15.5}); res.IsError || out.AgentModelSpendCapOverrideUSD == nil || *out.AgentModelSpendCapOverrideUSD != 15.5 {
		t.Fatalf("MCP set = %+v %+v", res, out)
	}
	if res, out := call(map[string]any{"agentModelMonthlySpendCapUsd": nil}); res.IsError || out.AgentModelSpendCapOverrideUSD != nil || *out.AgentModelMonthlySpendCapUSD != 20 {
		t.Fatalf("MCP null = %+v %+v", res, out)
	}
	call(map[string]any{"agentModelMonthlySpendCapUsd": 15.5})
	if res, out := call(map[string]any{"resetAgentModelMonthlySpendCap": true}); res.IsError || out.AgentModelSpendCapOverrideUSD != nil {
		t.Fatalf("MCP reset = %+v %+v", res, out)
	}

	// GraphQL: set, over 10× typed, null clears.
	schema := environmentSchema(t, svc)
	do := func(query string) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, Context: caller("admin"), RequestString: query})
	}
	if r := do(`mutation { updateAgentEnvironment(ownerId: "tea-a", input: {agentModelMonthlySpendCapUsd: 999}) { trustTier } }`); len(r.Errors) != 1 || r.Errors[0].Extensions["code"] != SpendCapInvalidCode {
		t.Fatalf("GraphQL over 10x = %#v", r.Errors)
	}
	var data map[string]any
	r := do(`mutation { updateAgentEnvironment(ownerId: "tea-a", input: {agentModelMonthlySpendCapUsd: 12}) { agentModelMonthlySpendCapUsd agentModelSpendCapOverrideUsd trustTier nextChargeThresholdUsd } }`)

	data, _ = r.Data.(map[string]any)
	if env, _ := data["updateAgentEnvironment"].(map[string]any); len(r.Errors) != 0 || env["agentModelMonthlySpendCapUsd"] != 12.0 || env["trustTier"] != "T1" || env["nextChargeThresholdUsd"] != 25.0 {
		t.Fatalf("GraphQL set = %#v %#v", r.Data, r.Errors)
	}
	// graphql-go cannot carry an explicit null, so GraphQL resets with the flag.
	r = do(`mutation { updateAgentEnvironment(ownerId: "tea-a", input: {resetAgentModelMonthlySpendCap: true}) { agentModelMonthlySpendCapUsd agentModelSpendCapOverrideUsd } }`)
	data, _ = r.Data.(map[string]any)
	if env, _ := data["updateAgentEnvironment"].(map[string]any); len(r.Errors) != 0 || env["agentModelSpendCapOverrideUsd"] != nil || env["agentModelMonthlySpendCapUsd"] != 20.0 {
		t.Fatalf("GraphQL null = %#v %#v", r.Data, r.Errors)
	}
}

// ADR091 D10.1 acceptance: a T0 workspace on the platform route cannot start
// a third concurrent session (typed AGENT_SESSION_TRUST_LIMIT); at T1 it can.
// BYO-key profiles are not limited.
func TestTrustTierLimitsConcurrentPlatformSessions(t *testing.T) {
	enablePlatformRoute(t, "tea-a")
	svc, st, _ := sessionWithEnvironment(t, nil)
	spend := &fakeSpend{fakeEnvironments: newFakeEnvironments(), plan: store.PlanPro}
	svc.Spend = spend
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	for i := range 2 {
		if _, err := svc.Create(caller("alice"), input); err != nil {
			t.Fatalf("T0 session %d: %v", i+1, err)
		}
	}
	_, err := svc.Create(caller("alice"), input)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != TrustLimitCode || !errors.Is(err, core.ErrConflict) || coded.Params["limit"] != 2 {
		t.Fatalf("third T0 session = %v, want %s (409)", err, TrustLimitCode)
	}
	if !strings.Contains(err.Error(), "first successful payment") {
		t.Fatalf("new-workspace refusal = %q", err)
	}
	// A workspace held at T0 by a failed payment is told to settle it.
	spend.trust = store.BillingTrust{LifetimePaidCents: 200000, PaidCycleInvoices: 6, LockedUntil: store.LockedIndefinitely}
	if _, err := svc.Create(caller("alice"), input); !errors.As(err, &coded) || coded.Code != TrustLimitCode ||
		!strings.Contains(err.Error(), "settle the outstanding payment") || coded.Params["locked"] != true {
		t.Fatalf("locked refusal = %v", err)
	}
	// Resuming one of the two live sessions does not count it against itself.
	spend.trust = store.BillingTrust{}
	var live string
	for id, row := range st.rows {
		if slices.Contains(activePhases, row.Phase) {
			live = id
			break
		}
	}
	if _, err := svc.Resume(caller("alice"), live); err != nil {
		t.Fatalf("resuming a live session at the limit: %v", err)
	}
	byo := input
	byo.AgentConfig = AgentConfig{Agent: "codex", Task: "fix the tests"}
	if _, err := svc.Create(caller("alice"), byo); err != nil {
		t.Fatalf("a BYO-key session is not tier-limited: %v", err)
	}
	spend.trust = paidT1
	if _, err := svc.Create(caller("alice"), input); err != nil {
		t.Fatalf("third session at T1: %v", err)
	}
}

// The store's active-phase set (the partial index predicate and every
// session count) is this package's activePhases.
func TestStoreActivePhasesMatch(t *testing.T) {
	got, want := slices.Sorted(slices.Values(store.ActiveAgentSessionPhases)), slices.Sorted(slices.Values(activePhases))
	if !slices.Equal(got, want) {
		t.Fatalf("store.ActiveAgentSessionPhases = %v, want activePhases %v", got, want)
	}
}
