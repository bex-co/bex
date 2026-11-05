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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestMicroUSDDecimalIsExactToTheMicroDollar(t *testing.T) {
	for micro, want := range map[int64]string{
		0: "0.00", 1: "0.000001", 1234: "0.001234", 420_000: "0.42", 5_500_000: "5.50",
		1_555_000: "1.555", 12_000_000: "12.00", 100_000_000_000: "100000.00",
	} {
		if got := microUSDDecimal(micro); got != want {
			t.Errorf("microUSDDecimal(%d) = %q, want %q", micro, got, want)
		}
	}
}

// ADR094 D8: maxSpendUSD is refused with the named 400 on a session bex cannot
// meter, never silently ignored, and refused as invalid when it is not an amount.
func TestCreateMaxSpendUSDIsRefusedWhereItCannotBeEnforced(t *testing.T) {
	svc, st, _, lifecycle := fixture()
	input := createInput()
	input.MaxSpendUSD = ptr(5.0)
	_, err := svc.Create(caller("alice"), input)
	if codeOf(err) != SpendLimitUnavailableCode {
		t.Fatalf("outside the platform route: err = %v, want %s", err, SpendLimitUnavailableCode)
	}
	if len(st.rows) != 0 || lifecycle.modelEndpoint != "" {
		t.Fatalf("a refused create persisted %d rows / provisioned %q", len(st.rows), lifecycle.modelEndpoint)
	}

	enablePlatformRoute(t, "tea-a")
	for _, usd := range []float64{0, -1, 100_001, 1e-9} {
		input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
		input.MaxSpendUSD = ptr(usd)
		if _, err := svc.Create(caller("alice"), input); codeOf(err) != SpendLimitInvalidCode {
			t.Errorf("maxSpendUSD %v: err = %v, want %s", usd, err, SpendLimitInvalidCode)
		}
	}
	// A non-claude profile is not served by the platform route either.
	input.AgentConfig = AgentConfig{Agent: "codex", Model: "gpt-5", ModelEndpoint: "https://api.openai.com/v1", Task: "fix"}
	input.MaxSpendUSD = ptr(5.0)
	if _, err := svc.Create(caller("alice"), input); codeOf(err) != SpendLimitUnavailableCode {
		t.Errorf("codex on a platform workspace: err = %v, want %s", err, SpendLimitUnavailableCode)
	}
}

// With the platform route the ceiling is stored in micro-USD, read back as a
// decimal string, and part of the idempotent create's identity.
func TestCreateMaxSpendUSDIsStoredAndIdempotent(t *testing.T) {
	enablePlatformRoute(t, "tea-a")
	svc, st, _, _ := fixture()
	input := createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	input.MaxSpendUSD = ptr(5.5)
	input.IdempotencyKey = "k-1"
	created, err := svc.Create(caller("alice"), input)
	if err != nil {
		t.Fatal(err)
	}
	if row := st.rows[created.ID]; row.MaxSpendMicroUSD == nil || *row.MaxSpendMicroUSD != 5_500_000 {
		t.Fatalf("stored ceiling = %v, want 5500000", row.MaxSpendMicroUSD)
	}
	if created.MaxSpendUSD != "5.50" {
		t.Fatalf("view maxSpendUSD = %q, want 5.50", created.MaxSpendUSD)
	}

	again, err := svc.Create(caller("alice"), input)
	if err != nil || again.ID != created.ID || again.Created == nil || *again.Created {
		t.Fatalf("same request = %+v / %v, want the first session replayed", again, err)
	}
	input.MaxSpendUSD = ptr(6.0)
	if _, err := svc.Create(caller("alice"), input); codeOf(err) != "AGENT_SESSION_IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("same key, other ceiling: err = %v, want AGENT_SESSION_IDEMPOTENCY_KEY_REUSED", err)
	}
	// A session without a ceiling renders none.
	input = createInput()
	input.AgentConfig = AgentConfig{Agent: "claude", Task: "fix the tests"}
	plain, err := svc.Create(caller("alice"), input)
	if err != nil || plain.MaxSpendUSD != "" || st.rows[plain.ID].MaxSpendMicroUSD != nil {
		t.Fatalf("no ceiling = %+v / %v", plain, err)
	}
}

// ADR094 D7: outcome.cost is the sum of the session's recorded customer prices,
// to the micro-dollar, as a string; absent when nothing was priced.
func TestOutcomeCostMatchesTheMeterToTheMicroDollar(t *testing.T) {
	svc, st, fga, _ := fixture()
	id := seedSession(st, fga, func(r *store.AgentSession) { r.Phase, r.Turns = PhaseCompleted, 2 })

	if got, err := svc.Get(caller("alice"), id); err != nil || got.Outcome.Cost != nil {
		t.Fatalf("no cost store: cost = %+v / %v, want none", got.Outcome.Cost, err)
	}
	costs := &fakeModelCosts{read: store.AgentSessionModelCost{Turns: []store.AgentTurnModelUsage{
		{Turn: 1, Requests: 3, PriceMicroUSD: 1_300_001},
		{Turn: 2, Requests: 1, PriceMicroUSD: 254_999},
	}}}
	svc.ModelCosts = costs
	got, err := svc.Get(caller("alice"), id)
	if err != nil || got.Outcome.Cost == nil || got.Outcome.Cost.ModelUSD != "1.555" {
		t.Fatalf("cost = %+v / %v, want modelUSD 1.555 (1300001 + 254999 µUSD)", got.Outcome.Cost, err)
	}
	raw, _ := json.Marshal(got.Outcome)
	var wire map[string]map[string]any
	_ = json.Unmarshal(raw, &wire)
	if wire["cost"]["modelUSD"] != "1.555" {
		t.Fatalf("wire cost = %s, want a decimal string", raw)
	}

	costs.read = store.AgentSessionModelCost{}
	if got, err := svc.Get(caller("alice"), id); err != nil || got.Outcome.Cost != nil {
		t.Fatalf("unpriced session: cost = %+v / %v, want none", got.Outcome.Cost, err)
	}
}

// ADR094 D9: the webhook payload is the session's state and outcome without
// the summary and the file list; a session that moved on to another turn
// reports only its state and turn.
func TestAgentSessionTurnOutcomeOmitsTheSummary(t *testing.T) {
	svc, st, fga, _ := fixture()
	id := seedSession(st, fga, func(r *store.AgentSession) {
		r.Phase, r.Turns, r.PRURL, r.PRNumber = PhaseCompleted, 2, "https://github.com/o/r/pull/412", 412
	})
	seedCompletedTurn(st, id, 2, `{"type":"text","text":"secret summary text"}`)
	svc.ModelCosts = &fakeModelCosts{read: store.AgentSessionModelCost{Turns: []store.AgentTurnModelUsage{{Turn: 2, Requests: 1, PriceMicroUSD: 420_000}}}}

	state, raw, err := (TurnOutcomes{svc}).AgentSessionTurnOutcome(context.Background(), id, 2, PhaseCompleted)
	if err != nil || state != StateSucceeded {
		t.Fatalf("outcome = %q / %v", state, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["summary"]; ok {
		t.Fatalf("outcome carries a summary: %s", raw)
	}
	if out["state"] != StateSucceeded || out["turn"] != float64(2) || out["pullRequest"] == nil || out["cost"] == nil {
		t.Fatalf("outcome = %s", raw)
	}

	// The event was for turn 1 but the session is on turn 2 now.
	_, raw, err = (TurnOutcomes{svc}).AgentSessionTurnOutcome(context.Background(), id, 1, PhaseFailed)
	if err != nil || string(raw) != `{"state":"failed","steerable":false,"turn":1}` {
		t.Fatalf("stale-turn outcome = %s / %v", raw, err)
	}
	if _, _, err := (TurnOutcomes{svc}).AgentSessionTurnOutcome(context.Background(), "ags-gone", 1, PhaseFailed); err == nil {
		t.Fatal("a missing session must report store.ErrNotFound")
	}
}

// The ceiling is one input on REST, GraphQL and MCP: each refuses it with the
// same named 400 off the platform route, and each stores and reads it back on it.
func TestMaxSpendUSDIsOneInputOnEverySurface(t *testing.T) {
	svc, _, _, _ := fixture()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rest := func(usd string) *httptest.ResponseRecorder {
		body := `{"ownerId":"tea-a","repo":"bex-co/example","agentConfig":{"agent":"claude","task":"t"},"maxSpendUSD":` + usd + `}`
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/agent-sessions", bytes.NewBufferString(body)).WithContext(caller("alice")))
		return rec
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}), Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()})})
	if err != nil {
		t.Fatal(err)
	}
	gql := func() *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: `mutation { createAgentSession(ownerId:"tea-a", repo:"bex-co/example", agentConfig:{agent:"claude",task:"t"}, maxSpendUSD: 5.5) { id maxSpendUSD } }`})
	}
	client := newMCPClient(t, caller("alice"), svc)
	mcpCall := func() *mcp.CallToolResult {
		res, err := client.CallTool(caller("alice"), &mcp.CallToolParams{Name: "create_agent_session", Arguments: map[string]any{
			"ownerId": "tea-a", "agentConfig": map[string]any{"agent": "claude", "task": "t"}, "maxSpendUSD": 5.5}})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	// Off the platform route: the same named 400 everywhere.
	if rec := rest("5.5"); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), SpendLimitUnavailableCode) {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
	}
	if r := gql(); len(r.Errors) != 1 || r.Errors[0].Extensions["code"] != SpendLimitUnavailableCode {
		t.Fatalf("GraphQL = %+v", r.Errors)
	}
	if res := mcpCall(); !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, SpendLimitUnavailableCode) {
		t.Fatalf("MCP = %+v", res)
	}

	// On it: stored and read back the same way.
	enablePlatformRoute(t, "tea-a")
	if rec := rest("5.5"); rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"maxSpendUSD":"5.50"`) {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
	}
	if r := gql(); len(r.Errors) != 0 || !strings.Contains(asJSONString(r.Data), `"maxSpendUSD":"5.50"`) {
		t.Fatalf("GraphQL = %+v %v", r.Errors, r.Data)
	}
	if res := mcpCall(); res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"maxSpendUSD":"5.50"`) {
		t.Fatalf("MCP = %+v", res.Content)
	}
}
