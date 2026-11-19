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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// TestEverySurfaceServesTheWaitAndTheCause (ADR047 § w10/m497): REST, GraphQL
// and MCP serve a waiting session's waitingFor and waitingSince, and a failed
// start's failureCause, with the same names and values. A capacity-limit
// failure keeps its answer unchanged, with no cause.
func TestEverySurfaceServesTheWaitAndTheCause(t *testing.T) {
	svc, st, _, lc := fixture()
	create := func(err error) string {
		t.Helper()
		lc.createErr = err
		view, createErr := svc.Create(caller("alice"), createInput())
		if createErr != nil {
			t.Fatal(createErr)
		}
		return view.ID
	}
	since := time.Date(2026, 11, 18, 10, 27, 39, 0, time.UTC)
	waiting := create(nil)
	row := st.rows[waiting]
	row.Phase, row.SandboxID, row.SandboxWaitSince = PhaseCreating, "", &since
	st.rows[waiting] = row

	type want struct{ waitingFor, waitingSince, failureCause any }
	cases := map[string]want{
		waiting: {WaitingForSandboxCapacity, since.Format(time.RFC3339), nil},
		create(fmt.Errorf("create: %w", core.NewUnavailableError(sandbox.CodeSandboxStartFailed, "no pull", map[string]any{"cause": "image_pull"}))): {nil, nil, "image_pull"},
		create(core.NewConflictError(sandbox.CodeSandboxCapacityLimit, "the workspace is at its sandbox limit", nil)):                                {nil, nil, nil},
		create(nil): {nil, nil, nil},
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	ctx := caller("alice")
	client := newMCPClient(t, ctx, svc)
	check := func(surface, id string, view map[string]any, w want) {
		t.Helper()
		for key, value := range map[string]any{"waitingFor": w.waitingFor, "waitingSince": w.waitingSince, "failureCause": w.failureCause} {
			got, present := view[key]
			if surface == "GraphQL" {
				present = got != nil // GraphQL answers null where REST and MCP omit the key
			}
			if got != value || present != (value != nil) {
				t.Errorf("%s %s %s = %v (present %v), want %v", surface, id, key, got, present, value)
			}
		}
	}
	for id, w := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-sessions/"+id, nil).WithContext(ctx))
		var rest map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("REST %s = %d %s", id, rec.Code, rec.Body.String())
		}
		check("REST", id, rest, w)
		if w.waitingFor != nil && rest["state"] != StateWorking {
			t.Errorf("a waiting session's state = %v, want working", rest["state"])
		}

		result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ agentSession(id: "` + id + `") { waitingFor waitingSince failureCause } }`})
		if len(result.Errors) != 0 {
			t.Fatalf("GraphQL %s errors = %v", id, result.Errors)
		}
		check("GraphQL", id, result.Data.(map[string]any)["agentSession"].(map[string]any), w)

		res, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "get_agent_session", Arguments: map[string]any{"id": id}})
		if err != nil || res.IsError {
			t.Fatalf("MCP %s = %+v err=%v", id, res, err)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		var viaMCP map[string]any
		_ = json.Unmarshal(raw, &viaMCP)
		check("MCP", id, viaMCP, w)
	}
}

// A session stops reading waiting the moment its turn is bound, and a start
// failure from before causes were read reads unknown.
func TestTheWaitShowsOnlyWhileTheTurnStarts(t *testing.T) {
	since := time.Now()
	for _, tc := range []struct {
		phase, sandboxID string
		waits            bool
	}{
		{PhaseCreating, "", true},
		{PhaseRedispatching, "", true},
		{PhaseRunning, "sbx-1", false},
		{PhaseCanceled, "", false},
		{PhaseFailed, "", false},
	} {
		waitingFor, at, _ := startState(tc.phase, tc.sandboxID, &since, "", "")
		if (waitingFor != "") != tc.waits || (at != nil) != tc.waits {
			t.Errorf("%s/%q: waitingFor=%q since=%v, want waiting=%v", tc.phase, tc.sandboxID, waitingFor, at, tc.waits)
		}
	}
	if _, _, cause := startState(PhaseFailed, "", nil, sandbox.CodeSandboxStartFailed, ""); cause != "unknown" {
		t.Errorf("an older start failure's cause = %q, want unknown", cause)
	}
	if _, _, cause := startState(PhaseFailed, "", nil, sandbox.CodeSandboxCapacityLimit, "pool_full"); cause != "" {
		t.Errorf("a cause beside another code = %q, want none", cause)
	}
}
