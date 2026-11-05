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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// reportSchema is the {tests_passed, files_changed} shape of the milestone's
// motivating example.
var reportSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"tests_passed":  map[string]any{"type": "boolean"},
		"files_changed": map[string]any{"type": "integer", "minimum": 0},
	},
	"required":             []any{"tests_passed", "files_changed"},
	"additionalProperties": false,
}

func yes() *bool { f := true; return &f }
func no() *bool  { f := false; return &f }

func TestOutputSchemaLimitsAndExternalRefs(t *testing.T) {
	huge := map[string]any{"description": strings.Repeat("x", maxOutputSchemaBytes)}
	for name, schema := range map[string]map[string]any{
		"oversize":             huge,
		"http ref":             {"$ref": "https://evil.example/schema.json"},
		"file ref":             {"properties": map[string]any{"a": map[string]any{"$ref": "other.json"}}},
		"ref to a remote defs": {"$ref": "https://evil.example/s.json#/$defs/x"},
		"another dialect":      {"$schema": "http://json-schema.org/draft-07/schema#", "type": "object"},
		"unknown type":         {"type": "frobnicate"},
		"minimum not a number": {"minimum": "high"},
		"NUL in the schema":    {"description": "a\x00b"},
		"dangling local ref":   {"$ref": "#/$defs/missing"},
	} {
		svc, st, _, _ := fixture()
		req := createInput()
		req.OutputSchema = schema
		_, err := svc.Create(caller("alice"), req)
		if !isCode(err, "AGENT_SESSION_OUTPUT_SCHEMA_INVALID") {
			t.Errorf("%s: err = %v, want AGENT_SESSION_OUTPUT_SCHEMA_INVALID", name, err)
		}
		if len(st.rows) != 0 {
			t.Errorf("%s: a refused schema still created a session", name)
		}
	}
}

func TestValidOutputSchemasAreStoredOnTheFirstTurn(t *testing.T) {
	for name, schema := range map[string]map[string]any{
		"typed object":      reportSchema,
		"empty schema":      {},
		"explicit dialect":  {"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "string"},
		"local $defs ref":   {"$defs": map[string]any{"n": map[string]any{"type": "integer"}}, "properties": map[string]any{"a": map[string]any{"$ref": "#/$defs/n"}}},
		"just under 64 KiB": {"description": strings.Repeat("x", maxOutputSchemaBytes-64)},
	} {
		svc, st, _, _ := fixture()
		req := createInput()
		req.OutputSchema = schema
		view, err := svc.Create(caller("alice"), req)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		turn := st.turns[view.ID][1]
		var stored map[string]any
		if json.Unmarshal(turn.OutputSchema, &stored) != nil || len(stored) != len(schema) {
			t.Errorf("%s: stored schema = %s", name, turn.OutputSchema)
		}
		if !turn.OutputRequired {
			t.Errorf("%s: outputRequired should default to true once a schema is set", name)
		}
	}

	svc, st, _, _ := fixture()
	req := createInput()
	req.OutputSchema, req.OutputRequired = reportSchema, no()
	view, err := svc.Create(caller("alice"), req)
	if err != nil || st.turns[view.ID][1].OutputRequired {
		t.Fatalf("explicit outputRequired=false = %+v err=%v", st.turns[view.ID][1], err)
	}

	// A free-form turn stores nothing.
	svc, st, _, _ = fixture()
	view, err = svc.Create(caller("alice"), createInput())
	if turn := st.turns[view.ID][1]; err != nil || turn.OutputSchema != nil || turn.OutputRequired {
		t.Fatalf("free-form turn = %+v err=%v", turn, err)
	}
}

func TestOutputRequiredNeedsASchema(t *testing.T) {
	svc, _, _, _ := fixture()
	req := createInput()
	req.OutputRequired = yes()
	if _, err := svc.Create(caller("alice"), req); !isCode(err, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("outputRequired without a schema = %v, want AGENT_SESSION_INPUT_INVALID", err)
	}
	req.OutputRequired = no() // harmless
	if _, err := svc.Create(caller("alice"), req); err != nil {
		t.Fatalf("outputRequired=false without a schema = %v", err)
	}
}

func TestSteerStoresThisTurnsSchemaAndFreeFormTurnsStoreNone(t *testing.T) {
	svc, st, lc, id := steerableFixture(t)
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "report", OutputSchema: reportSchema}); err != nil {
		t.Fatal(err)
	}
	turn := st.turns[id][2]
	if len(turn.OutputSchema) == 0 || !turn.OutputRequired {
		t.Fatalf("turn 2 = %+v, want the schema stored and required", turn)
	}
	if lc.driverEnv["BEX_AGENT_OUTPUT_SCHEMA"] != string(turn.OutputSchema) {
		t.Fatalf("the sandbox got schema %q, the turn stored %q", lc.driverEnv["BEX_AGENT_OUTPUT_SCHEMA"], turn.OutputSchema)
	}

	setSession(st, id, func(r *store.AgentSession) { r.Phase = PhaseCompleted })
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "and now freely"}); err != nil {
		t.Fatal(err)
	}
	if turn := st.turns[id][3]; turn.OutputSchema != nil || turn.OutputRequired {
		t.Fatalf("turn 3 inherited a contract: %+v", turn)
	}
	for _, name := range []string{"BEX_AGENT_OUTPUT_SCHEMA", "BEX_AGENT_RESULT_URL", "BEX_AGENT_OUTPUT_REQUIRED"} {
		if _, ok := lc.driverEnv[name]; ok {
			t.Errorf("free-form turn env carries %s", name)
		}
	}

	setSession(st, id, func(r *store.AgentSession) { r.Phase = PhaseCompleted })
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "bad", OutputSchema: map[string]any{"$ref": "https://evil.example/x"}}); !isCode(err, "AGENT_SESSION_OUTPUT_SCHEMA_INVALID") {
		t.Fatalf("steer with an external ref = %v", err)
	}
	if st.rows[id].Turns != 3 {
		t.Fatalf("a refused steer advanced the turn counter to %d", st.rows[id].Turns)
	}
}

func TestSchemaTurnsGetTheResultEndpointInTheSandboxEnv(t *testing.T) {
	svc, _, _, lc := fixture()
	req := createInput()
	req.Repo, req.Branch = "", "" // repo-less: no Git proxy URL exists, the result URL must not need one
	req.OutputSchema = reportSchema
	view, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	env := lc.driverEnv
	wantURL, _ := agentsession.ResultURL(svc.gitProxyURL(), store.SandboxNamespace("tea-a"), view.ID)
	if env["BEX_AGENT_RESULT_URL"] != wantURL || env["BEX_AGENT_OUTPUT_REQUIRED"] != "1" || env["BEX_AGENT_REPO_URL"] != "" {
		t.Fatalf("env = %v, want the Pod-bound result URL %s on a repo-less session", env, wantURL)
	}
	if !strings.Contains(wantURL, "/agent-results/") {
		t.Fatalf("result URL %q is not on the result route", wantURL)
	}
	req = createInput()
	req.OutputSchema, req.OutputRequired = reportSchema, no()
	if _, err := svc.Create(caller("alice"), req); err != nil || lc.driverEnv["BEX_AGENT_OUTPUT_REQUIRED"] != "0" {
		t.Fatalf("optional result env = %v err=%v", lc.driverEnv, err)
	}
}

func TestIdempotencyDigestCoversTheOutputContract(t *testing.T) {
	svc, st, _, _ := fixture()
	req := createInput()
	req.IdempotencyKey, req.OutputSchema = "k", reportSchema
	first, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	// Same intent, key order irrelevant: replayed.
	again, err := svc.Create(caller("alice"), req)
	if err != nil || again.ID != first.ID || *again.Created {
		t.Fatalf("retry = %+v err=%v", again, err)
	}
	other := req
	other.OutputSchema = map[string]any{"type": "string"}
	if _, err := svc.Create(caller("alice"), other); !isCode(err, "AGENT_SESSION_IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("different schema, same key = %v", err)
	}
	other = req
	other.OutputRequired = no()
	if _, err := svc.Create(caller("alice"), other); !isCode(err, "AGENT_SESSION_IDEMPOTENCY_KEY_REUSED") {
		t.Fatalf("different outputRequired, same key = %v", err)
	}
	if len(st.rows) != 1 {
		t.Fatalf("rows = %d", len(st.rows))
	}
}

// A bad schema is the same named 400 on REST, GraphQL and MCP, and a good one
// is accepted on all three.
func TestOutputSchemaIsValidatedOnEveryTransport(t *testing.T) {
	bad := map[string]any{"$ref": "https://evil.example/schema.json"}
	code := "AGENT_SESSION_OUTPUT_SCHEMA_INVALID"

	svc, _, _, _ := fixture()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	post := func(schema map[string]any) *httptest.ResponseRecorder {
		req := createInput()
		req.OutputSchema = schema
		body, _ := json.Marshal(req)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/agent-sessions", strings.NewReader(string(body))).WithContext(caller("alice")))
		return rec
	}
	if rec := post(bad); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), code) {
		t.Fatalf("REST bad schema = %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(reportSchema); rec.Code != http.StatusCreated {
		t.Fatalf("REST good schema = %d %s", rec.Code, rec.Body.String())
	}

	gqlSvc, _, _, _ := fixture()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: gqlSvc.GraphQLQuery()}), Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: gqlSvc.GraphQLMutation()})})
	if err != nil {
		t.Fatal(err)
	}
	create := func(outputSchema string) *graphql.Result {
		return graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: `mutation($s: String) { createAgentSession(ownerId:"tea-a", repo:"bex-co/example", agentConfig:{agent:"codex",task:"t"}, outputSchema: $s) { id } }`,
			VariableValues: map[string]any{"s": outputSchema}})
	}
	rawBad, _ := json.Marshal(bad)
	for name, text := range map[string]string{"external ref": string(rawBad), "not an object": `[1]`, "not JSON": `{`} {
		if r := create(text); len(r.Errors) != 1 || r.Errors[0].Extensions["code"] != code {
			t.Errorf("GraphQL %s = %+v, want %s", name, r.Errors, code)
		}
	}
	rawGood, _ := json.Marshal(reportSchema)
	if r := create(string(rawGood)); len(r.Errors) != 0 {
		t.Fatalf("GraphQL good schema = %+v", r.Errors)
	}

	mcpSvc, _, _, _ := fixture()
	ctx := caller("alice")
	client := newMCPClient(t, ctx, mcpSvc)
	call := func(schema map[string]any) *mcp.CallToolResult {
		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "create_agent_session", Arguments: map[string]any{
			"ownerId": "tea-a", "agentConfig": map[string]any{"agent": "codex", "task": "t"}, "outputSchema": schema}})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := call(bad); !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, code) {
		t.Fatalf("MCP bad schema = %+v", res)
	}
	if res := call(reportSchema); res.IsError {
		t.Fatalf("MCP good schema = %+v", res.Content)
	}
}

// --- the result verb ---------------------------------------------------------

func resultEnvelope(sessionID string, result any, final bool) agentsession.ResultEnvelope {
	raw, _ := json.Marshal(result)
	return agentsession.ResultEnvelope{Kind: agentsession.ResultKind, Workspace: "tea-a", SessionID: sessionID,
		PodName: "sbx-live-0", PodUID: "uid-live", Result: raw, Final: final}
}

func resultCode(err error) string {
	var typed *agentsession.ResultError
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

// schemaSession is a running session whose current turn carries reportSchema.
func schemaSession(t *testing.T, svc *Service, st *fakeStore, repo string, required bool) string {
	t.Helper()
	req := CreateRequest{OwnerID: "tea-a", Repo: repo, AgentConfig: AgentConfig{Agent: "claude", Task: "report"}, OutputSchema: reportSchema, OutputRequired: &required}
	view, err := svc.Create(caller("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	setSession(st, view.ID, func(r *store.AgentSession) { r.SandboxID, r.Turns, r.Phase = "sbx-live", 1, PhaseRunning })
	return view.ID
}

func TestSubmitResultKeepsTheLastValidSubmissionAndOutcomeShowsOnlyAFinalOne(t *testing.T) {
	svc, st, _, _ := fixture()
	id := schemaSession(t, svc, st, "", true)
	ctx := context.Background()
	good := map[string]any{"tests_passed": true, "files_changed": 3}

	// A partial value is stored and readable on the turn, but it is not the outcome.
	if got, err := svc.SubmitResult(ctx, resultEnvelope(id, map[string]any{"tests_passed": false, "files_changed": 0}, false)); err != nil || !got.Accepted || got.Final {
		t.Fatalf("partial = %+v err=%v", got, err)
	}
	view, _ := svc.Get(caller("alice"), id)
	if view.Outcome.Result != nil {
		t.Fatalf("a partial result became the outcome: %v", view.Outcome.Result)
	}
	page, _ := svc.Transcript(caller("alice"), id, -1, 10)
	if turn := page.Turns[0]; turn.Result == nil || turn.ResultFinal || turn.OutputSchema == nil || !turn.OutputRequired {
		t.Fatalf("turn view = %+v, want the partial result, not final, with its schema", turn)
	}

	if _, err := svc.SubmitResult(ctx, resultEnvelope(id, good, true)); err != nil {
		t.Fatal(err)
	}
	view, _ = svc.Get(caller("alice"), id)
	if want := `{"files_changed":3,"tests_passed":true}`; mustJSON(view.Outcome.Result) != want {
		t.Fatalf("outcome.result = %s, want %s", mustJSON(view.Outcome.Result), want)
	}

	// An invalid submission is refused and leaves the last valid one alone.
	_, err := svc.SubmitResult(ctx, resultEnvelope(id, map[string]any{"tests_passed": "maybe", "files_changed": -2}, true))
	if resultCode(err) != agentsession.ResultSchemaMismatch || !strings.Contains(err.Error(), "/tests_passed") || !strings.Contains(err.Error(), "/files_changed") {
		t.Fatalf("invalid result = %v, want schema_mismatch naming both fields", err)
	}
	// A later partial never downgrades a final result.
	if _, err := svc.SubmitResult(ctx, resultEnvelope(id, map[string]any{"tests_passed": false, "files_changed": 9}, false)); err != nil {
		t.Fatal(err)
	}
	view, _ = svc.Get(caller("alice"), id)
	if mustJSON(view.Outcome.Result) != `{"files_changed":3,"tests_passed":true}` {
		t.Fatalf("a later partial replaced the final result: %s", mustJSON(view.Outcome.Result))
	}
	// A later valid final replaces it: the last valid submission wins.
	if _, err := svc.SubmitResult(ctx, resultEnvelope(id, map[string]any{"tests_passed": true, "files_changed": 4}, true)); err != nil {
		t.Fatal(err)
	}
	view, _ = svc.Get(caller("alice"), id)
	if mustJSON(view.Outcome.Result) != `{"files_changed":4,"tests_passed":true}` {
		t.Fatalf("outcome.result = %s, want the last valid final", mustJSON(view.Outcome.Result))
	}
}

func mustJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func TestSubmitResultOnlyFromTheSessionsOwnLiveSandbox(t *testing.T) {
	svc, st, _, _ := fixture()
	id := schemaSession(t, svc, st, "bex-co/example", true)
	good := map[string]any{"tests_passed": true, "files_changed": 1}
	for name, mutate := range map[string]func(*agentsession.ResultEnvelope){
		"another workspace":  func(e *agentsession.ResultEnvelope) { e.Workspace = "tea-b" },
		"a stale pod":        func(e *agentsession.ResultEnvelope) { e.PodName = "sbx-old-0" },
		"no pod uid":         func(e *agentsession.ResultEnvelope) { e.PodUID = "" },
		"an unknown session": func(e *agentsession.ResultEnvelope) { e.SessionID = "ags-doesnotexist" },
	} {
		envelope := resultEnvelope(id, good, true)
		mutate(&envelope)
		if _, err := svc.SubmitResult(context.Background(), envelope); resultCode(err) != agentsession.ResultForbidden {
			t.Errorf("%s: err = %v, want forbidden", name, err)
		}
	}
	// A finished turn takes no more results, even from its own (grace-period) pod.
	setSession(st, id, func(r *store.AgentSession) { r.Phase = PhaseCompleted })
	if _, err := svc.SubmitResult(context.Background(), resultEnvelope(id, good, true)); resultCode(err) != agentsession.ResultForbidden {
		t.Errorf("after the turn ended: err = %v, want forbidden", err)
	}
	if turn := st.turns[id][1]; turn.Result != nil {
		t.Fatalf("a refused submission stored %s", turn.Result)
	}
}

func TestSubmitResultOnAFreeFormTurnIsNotExpected(t *testing.T) {
	svc, st, _, _ := fixture()
	id := runningSession(t, svc, st, 1)
	_, err := svc.SubmitResult(context.Background(), resultEnvelope(id, map[string]any{"a": 1}, true))
	if resultCode(err) != agentsession.ResultNotExpected {
		t.Fatalf("err = %v, want not_expected", err)
	}
}

// Structured output works without a repository (ADR094 D5): the chat-only
// "research this and answer in this shape" case.
func TestRepoLessSessionsSupportStructuredOutput(t *testing.T) {
	svc, st, _, _ := fixture()
	id := schemaSession(t, svc, st, "", true)
	if st.rows[id].Repo != "" || st.rows[id].Branch != "" {
		t.Fatalf("session = %+v, want repo-less", st.rows[id])
	}
	if _, err := svc.SubmitResult(context.Background(), resultEnvelope(id, map[string]any{"tests_passed": true, "files_changed": 0}, true)); err != nil {
		t.Fatal(err)
	}
	view, _ := svc.Get(caller("alice"), id)
	if mustJSON(view.Outcome.Result) != `{"files_changed":0,"tests_passed":true}` {
		t.Fatalf("outcome.result = %s", mustJSON(view.Outcome.Result))
	}
}

func TestOutcomeResultIsNullWithoutAFinalSubmissionAndIsPerTurn(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	view, _ := svc.Get(caller("alice"), id)
	if view.Outcome.Result != nil {
		t.Fatalf("free-form session outcome.result = %v", view.Outcome.Result)
	}
	// Turn 1 had a final result; turn 2 (new contract, no submission yet) shows none.
	st.turns[id][1] = withResult(st.turns[id][1], `{"tests_passed":true,"files_changed":1}`, true)
	if view, _ = svc.Get(caller("alice"), id); mustJSON(view.Outcome.Result) != `{"files_changed":1,"tests_passed":true}` {
		t.Fatalf("turn 1 outcome.result = %s", mustJSON(view.Outcome.Result))
	}
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "again", OutputSchema: reportSchema}); err != nil {
		t.Fatal(err)
	}
	if view, _ = svc.Get(caller("alice"), id); view.Outcome.Result != nil {
		t.Fatalf("turn 2 inherited turn 1's result: %s", mustJSON(view.Outcome.Result))
	}
}

func withResult(turn store.AgentSessionTurn, result string, final bool) store.AgentSessionTurn {
	turn.Result, turn.ResultFinal = json.RawMessage(result), final
	return turn
}

// outcome.result, the transcript turn's result and the failure code read the
// same on REST, GraphQL and MCP.
func TestResultReadsTheSameOnEverySurface(t *testing.T) {
	svc, st, _, _ := fixture()
	id := schemaSession(t, svc, st, "", true)
	if _, err := svc.SubmitResult(context.Background(), resultEnvelope(id, map[string]any{"tests_passed": true, "files_changed": 2}, true)); err != nil {
		t.Fatal(err)
	}
	setSession(st, id, func(r *store.AgentSession) { r.Phase = PhaseCompleted })
	want := `{"files_changed":2,"tests_passed":true}`
	ctx := caller("alice")

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-sessions/"+id, nil).WithContext(ctx))
	var rest struct {
		Outcome struct {
			Result json.RawMessage `json:"result"`
		} `json:"outcome"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || string(rest.Outcome.Result) != want {
		t.Fatalf("REST outcome.result = %s (%v)", rest.Outcome.Result, err)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-sessions/"+id+"/transcript", nil).WithContext(ctx))
	var restPage struct {
		Turns []struct {
			Result      json.RawMessage `json:"result"`
			ResultFinal bool            `json:"resultFinal"`
		} `json:"turns"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &restPage); err != nil || len(restPage.Turns) != 1 || string(restPage.Turns[0].Result) != want || !restPage.Turns[0].ResultFinal {
		t.Fatalf("REST transcript turn = %s (%v)", rec.Body.String(), err)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	r := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ agentSession(id: "` + id + `") { outcome { result } } agentSessionTranscript(id: "` + id + `") { turns { result resultFinal outputRequired } } }`})
	if len(r.Errors) != 0 {
		t.Fatalf("GraphQL = %+v", r.Errors)
	}
	data := r.Data.(map[string]any)
	if got := data["agentSession"].(map[string]any)["outcome"].(map[string]any)["result"]; got != want {
		t.Fatalf("GraphQL outcome.result = %v, want %s", got, want)
	}
	if turn := data["agentSessionTranscript"].(map[string]any)["turns"].([]any)[0].(map[string]any); turn["result"] != want || turn["resultFinal"] != true || turn["outputRequired"] != true {
		t.Fatalf("GraphQL turn = %v", turn)
	}

	client := newMCPClient(t, ctx, svc)
	res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "get_agent_session", Arguments: map[string]any{"id": id}})
	if err != nil || res.IsError {
		t.Fatalf("MCP = %+v err=%v", res, err)
	}
	if got := mustJSON(res.StructuredContent.(map[string]any)["outcome"].(map[string]any)["result"]); got != want {
		t.Fatalf("MCP outcome.result = %s, want %s", got, want)
	}
	waited, err := svc.Wait(ctx, WaitRequest{IDs: []string{id}, TimeoutSeconds: 1}, nil)
	if err != nil || mustJSON(waited.Sessions[0].Outcome.Result) != want {
		t.Fatalf("wait outcome = %+v err=%v", waited.Sessions, err)
	}
}

// --- missing result ----------------------------------------------------------

func structuredCompleter(t *testing.T, required, withSchema, submitFinal bool) (*Completer, *fakeStore, string) {
	t.Helper()
	c, st, _, id := completerFixture(succeededStatus(true), nil)
	recordAgentPR(st, id)
	if withSchema {
		turn := st.turns[id][1]
		raw, _ := json.Marshal(reportSchema)
		turn.OutputSchema, turn.OutputRequired = raw, required
		if submitFinal {
			turn.Result, turn.ResultFinal = json.RawMessage(`{"tests_passed":true,"files_changed":1}`), true
		}
		st.turns[id][1] = turn
	}
	return c, st, id
}

func TestRequiredResultMissingFailsTheTurnAndKeepsThePullRequest(t *testing.T) {
	c, st, id := structuredCompleter(t, true, true, false)
	c.Reconcile(context.Background())
	row := st.rows[id]
	if row.Phase != PhaseFailed || row.FailureReasonCode != FailureCodeStructuredOutputMissing || row.FailureReason != structuredOutputMissingReason {
		t.Fatalf("row = %+v, want failed STRUCTURED_OUTPUT_MISSING", row)
	}
	if row.PRURL == "" || row.PRNumber != 7 {
		t.Fatalf("the failed turn dropped the agent's pull request: %+v", row)
	}
	out := outcomeOf(row)
	if out.State != StateFailed || out.FailureReasonCode != FailureCodeStructuredOutputMissing || out.PullRequest == nil || out.Result != nil {
		t.Fatalf("outcome = %+v", out)
	}
}

func TestResultPresentOptionalOrAbsentContractsComplete(t *testing.T) {
	for name, tc := range map[string]struct{ required, schema, final bool }{
		"final result submitted":      {true, true, true},
		"result optional and missing": {false, true, false},
		"no schema at all":            {false, false, false},
	} {
		c, st, id := structuredCompleter(t, tc.required, tc.schema, tc.final)
		c.Reconcile(context.Background())
		if row := st.rows[id]; row.Phase != PhaseCompleted || row.FailureReasonCode != "" {
			t.Errorf("%s: row = %+v, want completed", name, row)
		}
	}
}

// The driver's own status is not believed: a clean "succeeded" report with no
// stored result still fails a required turn, and a driver failure keeps its reason.
func TestMissingResultIsDecidedFromWhatBexAPIStoredNotFromTheDriver(t *testing.T) {
	c, st, id := structuredCompleter(t, true, true, false)
	raw, _ := json.Marshal(statusReport{State: "succeeded", Git: statusGit{}})
	c.Sandbox.(*fakeLifecycle).status = string(raw)
	c.Reconcile(context.Background())
	if st.rows[id].FailureReasonCode != FailureCodeStructuredOutputMissing {
		t.Fatalf("row = %+v", st.rows[id])
	}

	c, st, id = structuredCompleter(t, true, true, false)
	c.Sandbox.(*fakeLifecycle).status = `{"state":"failed","error":"model refused"}`
	c.Reconcile(context.Background())
	if row := st.rows[id]; row.FailureReason != "model refused" || row.FailureReasonCode != "" {
		t.Fatalf("a driver failure was rewritten: %+v", row)
	}
}

func TestOnlyTheCurrentTurnsContractCounts(t *testing.T) {
	c, st, id := structuredCompleter(t, true, true, false)
	// Turn 2 is free-form: turn 1's unmet contract must not fail it.
	st.recordTurn(id, 2, "free", DeliveryRedispatch)
	setSession(st, id, func(r *store.AgentSession) { r.Turns = 2 })
	c.Reconcile(context.Background())
	if row := st.rows[id]; row.Phase != PhaseCompleted {
		t.Fatalf("row = %+v, want completed", row)
	}
}

func TestOutputArgsReadsTheGraphQLArguments(t *testing.T) {
	schema, required, err := outputArgs(map[string]any{"outputSchema": `{"type":"string"}`, "outputRequired": false})
	if err != nil || schema["type"] != "string" || required == nil || *required {
		t.Fatalf("schema=%v required=%v err=%v", schema, required, err)
	}
	if schema, required, err = outputArgs(map[string]any{}); err != nil || schema != nil || required != nil {
		t.Fatalf("absent args = %v %v %v", schema, required, err)
	}
	if _, _, err = outputArgs(map[string]any{"outputSchema": strings.Repeat("a", 2*maxOutputSchemaBytes+1)}); !isCode(err, "AGENT_SESSION_OUTPUT_SCHEMA_INVALID") {
		t.Fatalf("oversize text = %v", err)
	}
}

func TestCheckResultReportsBoundedProblems(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"type": "array", "items": map[string]any{"type": "integer"}})
	problems := checkResult(raw, []byte(`["a","b","c","d","e","f","g","h","i","j"]`))
	if len(problems) != maxResultProblems || !strings.HasPrefix(problems[0], "/0: ") {
		t.Fatalf("problems = %q, want %d located problems", problems, maxResultProblems)
	}
	if problems := checkResult(raw, []byte(`[1,2]`)); problems != nil {
		t.Fatalf("a conforming result reported %q", problems)
	}
	if problems := checkResult(raw, []byte(`not json`)); len(problems) != 1 {
		t.Fatalf("garbage result = %q", problems)
	}
}
