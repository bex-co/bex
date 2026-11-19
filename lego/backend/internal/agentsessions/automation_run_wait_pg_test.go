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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// An automation run whose session waits for a sandbox machine reads as
// queued, not dropped, and one whose session failed to start reads apart from
// a failed create, on REST, GraphQL and MCP alike (ADR047 § w10/m497).
func TestARunShowsItsSessionWaitingAndWhyItDidNotStart(t *testing.T) {
	e := newSchedEnv(t, "aut-wait")
	alice := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
	a := e.automation("waits", "alice", e.clock.Now().Add(time.Hour))
	run, err := e.svc.RunAgentAutomationNow(alice, e.ws, a.ID)
	if err != nil || run.Outcome != store.AutomationRunStarted {
		t.Fatalf("run now = %+v err=%v", run, err)
	}
	ctx := context.Background()
	since := time.Date(2026, 11, 18, 10, 27, 39, 0, time.UTC)
	if _, err := e.pool.Exec(ctx, `UPDATE agent_sessions SET sandbox_wait_since=$2 WHERE id=$1`, run.SessionID, since); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	e.svc.RegisterREST(mux)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: e.svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	client := newMCPClient(t, alice, e.svc)
	// sessions reads the latest run's session on every surface.
	sessions := func() map[string]map[string]any {
		t.Helper()
		out := map[string]map[string]any{}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-automations/"+a.ID+"/runs?ownerId="+e.ws, nil).WithContext(alice))
		var rest struct{ Runs []map[string]any }
		if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || rec.Code != http.StatusOK || len(rest.Runs) == 0 {
			t.Fatalf("REST runs = %d %s", rec.Code, rec.Body.String())
		}
		out["REST"], _ = rest.Runs[0]["session"].(map[string]any)

		result := graphql.Do(graphql.Params{Schema: schema, Context: alice, RequestString: `{ agentAutomationRuns(ownerId: "` + e.ws + `", id: "` + a.ID + `") { runs { session { state waitingFor waitingSince failureReasonCode failureCause } } } }`})
		if len(result.Errors) != 0 {
			t.Fatalf("GraphQL runs errors = %v", result.Errors)
		}
		runs := result.Data.(map[string]any)["agentAutomationRuns"].(map[string]any)["runs"].([]any)
		out["GraphQL"], _ = runs[0].(map[string]any)["session"].(map[string]any)

		res, err := client.CallTool(alice, &mcp.CallToolParams{Name: "list_agent_automation_runs", Arguments: map[string]any{"ownerId": e.ws, "automation": a.ID}})
		if err != nil || res.IsError {
			t.Fatalf("MCP runs = %+v err=%v", res, err)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var viaMCP struct{ Runs []map[string]any }
		_ = json.Unmarshal(raw, &viaMCP)
		out["MCP"], _ = viaMCP.Runs[0]["session"].(map[string]any)
		return out
	}
	expect := func(want map[string]any) {
		t.Helper()
		for surface, session := range sessions() {
			if session == nil {
				t.Fatalf("%s: the run carries no session", surface)
			}
			for key, value := range want {
				if got := session[key]; got != value {
					t.Errorf("%s run session %s = %v, want %v", surface, key, got, value)
				}
			}
		}
	}
	expect(map[string]any{"state": StateWorking, "waitingFor": WaitingForSandboxCapacity, "waitingSince": since.Format(time.RFC3339), "failureCause": nil})

	if _, err := e.pool.Exec(ctx, `UPDATE agent_sessions SET phase='failed', status='failed', sandbox_wait_since=NULL,
		failure_reason_code=$2, failure_cause='pool_full' WHERE id=$1`, run.SessionID, sandbox.CodeSandboxStartFailed); err != nil {
		t.Fatal(err)
	}
	expect(map[string]any{"state": StateFailed, "waitingFor": nil, "failureReasonCode": sandbox.CodeSandboxStartFailed, "failureCause": "pool_full"})
	if got := e.latestRun(a.ID).Outcome; got != store.AutomationRunStarted {
		t.Fatalf("the run reads %s, want started: a session that did not start is not a failed create", got)
	}
}

// A run whose session failed to start for a platform cause is a started run,
// so it never counts toward the repeated_failures pause (ADR101 D7): only
// failed creates do.
func TestAPlatformStartFailureNeverPausesTheAutomation(t *testing.T) {
	e := newSchedEnv(t, "aut-start-failed")
	a := e.automation("busy-pool", "alice", e.clock.Now().Add(-time.Minute))
	for i := 1; i <= automationFailureLimit+1; i++ {
		e.tick()
		run := e.latestRun(a.ID)
		if run.Outcome != store.AutomationRunStarted {
			t.Fatalf("fire %d = %s, want started", i, run.Outcome)
		}
		if _, err := e.pool.Exec(context.Background(), `UPDATE agent_sessions SET phase='failed', status='failed',
			failure_reason_code=$2, failure_cause='pool_full' WHERE id=$1`, run.SessionID, sandbox.CodeSandboxStartFailed); err != nil {
			t.Fatal(err)
		}
		e.clock.Advance(5 * time.Minute)
	}
	if row := e.reload(a.ID); !row.Enabled || row.PausedReason != "" || row.ConsecutiveFailures != 0 {
		t.Fatalf("after %d start failures the automation is enabled=%v paused=%q failures=%d, want it still running",
			automationFailureLimit+1, row.Enabled, row.PausedReason, row.ConsecutiveFailures)
	}
}
