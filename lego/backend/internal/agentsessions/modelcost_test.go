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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeModelCosts struct {
	session string
	read    store.AgentSessionModelCost
}

func (f *fakeModelCosts) AgentSessionModelUsage(_ context.Context, sessionID string) (store.AgentSessionModelCost, error) {
	f.session = sessionID
	return f.read, nil
}

// ADR091 D6: a session's model cost is the same on the service, REST,
// GraphQL and MCP, scoped to the session's own workspace and read access.
func TestModelCostIsTheSameOnEverySurface(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	at := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	costs := &fakeModelCosts{read: store.AgentSessionModelCost{
		WorkspaceID: st.rows[id].WorkspaceID,
		AgentConfig: []byte(`{"agent":"claude","modelEndpoint":"https://openrouter.ai/api"}`),
		Turns: []store.AgentTurnModelUsage{
			{Turn: 1, Requests: 3, InputTokens: 100, OutputTokens: 20, CacheReadTokens: 5, PriceMicroUSD: 1_300_000, Models: []string{"m/a"}, FirstAt: at, LastAt: at.Add(time.Minute)},
			{Turn: 2, Requests: 1, CacheWriteTokens: 7, PriceMicroUSD: 255_000, Models: []string{"m/a", "m/b"}, FirstAt: at.Add(time.Hour), LastAt: at.Add(time.Hour)},
		},
	}}

	// No usage store wired: zero cost on the session's own route, never an
	// error. The fixture session runs on its provider's endpoint (BYO).
	empty, err := svc.ModelCost(caller("alice"), id)
	if err != nil || empty.CostUSD != "0.00" || empty.ModelSource != ModelSourceBYO || empty.Turns == nil || len(empty.Turns) != 0 {
		t.Fatalf("store-off model cost = %+v err=%v", empty, err)
	}

	svc.ModelCosts = costs
	out, err := svc.ModelCost(caller("alice"), id)
	if err != nil {
		t.Fatal(err)
	}
	if costs.session != id {
		t.Fatalf("read %s, want %s", costs.session, id)
	}
	// Cents strings, the usage read's format: 1.555 rounds half-up to 1.56.
	if out.SessionID != id || out.ModelSource != ModelSourcePlatform || out.CostUSD != "1.56" || out.Requests != 4 || len(out.Turns) != 2 ||
		out.Turns[0].CostUSD != "1.30" || out.Turns[1].CostUSD != "0.26" || out.Turns[1].CacheWriteTokens != 7 {
		t.Fatalf("model cost = %+v", out)
	}
	if _, err := svc.ModelCost(caller("bob"), id); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-workspace model cost = %v, want the session read's denial", err)
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-sessions/"+id+"/model-cost", nil).WithContext(caller("alice")))
	var body ModelCost
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || !reflect.DeepEqual(body, out) {
		t.Fatalf("REST model cost = %d %s, want %+v", rec.Code, rec.Body.String(), out)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	r := graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: `{ agentSessionModelCost(id:"` + id +
		`") { sessionId modelSource costUsd requests turns { turn costUsd requests inputTokens outputTokens cacheReadTokens cacheWriteTokens models firstAt lastAt } } }`})
	if len(r.Errors) != 0 {
		t.Fatalf("GraphQL errors = %#v", r.Errors)
	}
	got := asJSONString(r.Data)
	for _, want := range []string{`"costUsd":"1.56"`, `"modelSource":"PLATFORM"`, `"requests":4`, `"models":["m/a","m/b"]`, `"firstAt":"2026-11-05T09:00:00Z"`, `"cacheWriteTokens":7`} {
		if !strings.Contains(got, want) {
			t.Fatalf("GraphQL model cost = %s, missing %s", got, want)
		}
	}

	client := newMCPClient(t, caller("alice"), svc)
	res, err := client.CallTool(caller("alice"), &mcp.CallToolParams{Name: "get_agent_session_model_cost", Arguments: map[string]any{"id": id}})
	if err != nil || res.IsError {
		t.Fatalf("MCP model cost = %+v err=%v", res, err)
	}
	var mcpBody ModelCost
	if text := res.Content[0].(*mcp.TextContent).Text; json.Unmarshal([]byte(text), &mcpBody) != nil || !reflect.DeepEqual(mcpBody, body) {
		t.Fatalf("MCP model cost = %s, want the REST body %+v", text, body)
	}
}

// The session's label follows the route it was created on; billed requests
// always read as platform.
func TestSessionModelSource(t *testing.T) {
	for _, c := range []struct {
		config   string
		requests int
		want     ModelSource
	}{
		{`{"agent":"claude","modelEndpoint":"https://openrouter.ai/api/"}`, 0, ModelSourcePlatform},
		{`{"agent":"claude","modelEndpoint":"https://api.anthropic.com"}`, 0, ModelSourceBYO},
		{`{"agent":"claude","modelEndpoint":"https://api.anthropic.com"}`, 2, ModelSourcePlatform},
		{``, 0, ModelSourceBYO},
	} {
		if got := sessionModelSource([]byte(c.config), c.requests); got != c.want {
			t.Errorf("sessionModelSource(%s, %d) = %s, want %s", c.config, c.requests, got, c.want)
		}
	}
	if got := microUSDString(1_555_000); got != "1.56" {
		t.Errorf("microUSDString(1555000) = %s, want 1.56 (half-up)", got)
	}
}
