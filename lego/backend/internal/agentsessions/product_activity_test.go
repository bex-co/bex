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
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ADR113 D7 server facts: agent sessions and automations record one created
// product activity row with the surface that started them, so the Product
// adoption board can answer "does the CLI start real work?" without the
// client collecting anything more.

// activityCapture is the recorder's sink. The surface it receives is the one
// core.ObserveProductActivity derived from the request context.
type activityCapture struct {
	mu     sync.Mutex
	events []core.ProductActivity
	err    error
}

func (c *activityCapture) record(_ context.Context, e core.ProductActivity) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	return c.err
}

func (c *activityCapture) recorded() []core.ProductActivity {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]core.ProductActivity(nil), c.events...)
}

// only asserts exactly one created fact for resource and returns it.
func (c *activityCapture) only(t *testing.T, resource string) core.ProductActivity {
	t.Helper()
	events := c.recorded()
	if len(events) != 1 || events[0].ResourceID != resource || events[0].EventType != "created" {
		t.Fatalf("product activity = %+v, want one created fact for %s", events, resource)
	}
	return events[0]
}

func capturing(svc *Service) *activityCapture {
	capture := &activityCapture{}
	svc.ProductActivity = capture.record
	return capture
}

// origin is what api.RequestOriginMiddleware puts on every public request.
func origin(ctx context.Context, transport, userAgent string) context.Context {
	return core.WithRequestOrigin(ctx, core.RequestOrigin{Transport: transport, UserAgent: userAgent})
}

const cliUserAgent = "render-cli/2.36.0 (macOS - 26.1)"

func TestSessionCreateRecordsTheSurfaceThatStartedIt(t *testing.T) {
	t.Run("rest from the cli", func(t *testing.T) {
		svc, _, _, _ := fixture()
		capture := capturing(svc)
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		body, _ := json.Marshal(createInput())
		req := httptest.NewRequest(http.MethodPost, "/v1/agent-sessions", strings.NewReader(string(body))).
			WithContext(origin(caller("alice"), core.TransportREST, cliUserAgent))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
		}
		var view View
		if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		got := capture.only(t, view.ID)
		if got.WorkspaceID != "tea-a" || got.ResourceType != "agent_session" || got.Surface != core.SurfaceCLI || got.ActorID != "alice" {
			t.Fatalf("session fact = %+v, want an agent_session in tea-a from the cli by alice", got)
		}
	})

	t.Run("graphql from the dashboard", func(t *testing.T) {
		svc, _, _, _ := fixture()
		capture := capturing(svc)
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		result := graphql.Do(graphql.Params{Schema: schema, Context: origin(caller("alice"), core.TransportGraphQL, "Mozilla/5.0"),
			RequestString: `mutation { createAgentSession(ownerId:"tea-a", repo:"bex-co/example", agentConfig:{agent:"codex",model:"gpt-5",task:"fix the tests"}) { id } }`})
		if len(result.Errors) != 0 {
			t.Fatalf("GraphQL errors = %#v", result.Errors)
		}
		id, _ := result.Data.(map[string]any)["createAgentSession"].(map[string]any)["id"].(string)
		if got := capture.only(t, id); got.ResourceType != "agent_session" || got.Surface != core.SurfaceDashboard {
			t.Fatalf("session fact = %+v, want an agent_session from the dashboard", got)
		}
	})

	t.Run("mcp", func(t *testing.T) {
		svc, _, _, _ := fixture()
		capture := capturing(svc)
		ctx := origin(caller("alice"), core.TransportMCP, cliUserAgent)
		client := newMCPClient(t, ctx, svc)
		result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "spawn_agent_session", Arguments: map[string]any{
			"ownerId": "tea-a", "repo": "bex-co/example",
			"agentConfig": map[string]any{"agent": "codex", "model": "gpt-5", "task": "fix the tests"},
		}})
		if err != nil || result.IsError {
			t.Fatalf("MCP = %+v err=%v", result, err)
		}
		raw, _ := json.Marshal(result.StructuredContent)
		var view View
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		// MCP outranks a CLI-shaped client: an agent's tool call is agent traffic.
		if got := capture.only(t, view.ID); got.ResourceType != "agent_session" || got.Surface != core.SurfaceMCP {
			t.Fatalf("session fact = %+v, want an agent_session over mcp", got)
		}
	})
}

// A create that did not make a reachable session records nothing, and a
// replay of one that did records nothing again: one session, one fact.
func TestSessionCreateRecordsOnlyANewReachableSession(t *testing.T) {
	svc, _, fga, _ := fixture()
	capture := capturing(svc)
	ctx := origin(caller("alice"), core.TransportREST, cliUserAgent)
	in := createInput()
	in.IdempotencyKey = "retry-1"
	first, err := svc.Create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := svc.Create(ctx, in)
	if err != nil || replay.ID != first.ID || replay.Created == nil || *replay.Created {
		t.Fatalf("replay = %+v err=%v, want the first session replayed", replay, err)
	}
	capture.only(t, first.ID)

	// The parent tuple cannot be written: the row is compensated and the
	// create refused, so there is no session to count.
	fga.fail = true
	svc.Base.Authz = checkerFunc(func(context.Context, string, string, string) (bool, error) { return true, nil })
	if _, err := svc.Create(ctx, createInput()); !isCode(err, core.AgentSessionDependencyUnavailableCode) {
		t.Fatalf("create = %v, want %s", err, core.AgentSessionDependencyUnavailableCode)
	}
	capture.only(t, first.ID)
}

// Analytics never turns a completed create into an error.
func TestProductActivityFailureNeverFailsACreate(t *testing.T) {
	svc, st, _, _ := fixture()
	capture := capturing(svc)
	capture.err = errors.New("product analytics unavailable")
	view, err := svc.Create(origin(caller("alice"), core.TransportREST, cliUserAgent), createInput())
	if err != nil {
		t.Fatalf("session create = %v, want success despite the recorder", err)
	}
	if _, ok := st.rows[view.ID]; !ok {
		t.Fatal("the session row is missing")
	}
	capture.only(t, view.ID)

	auto, _ := automationTestService("tea-a", newFakeAutomations())
	autoCapture := capturing(auto)
	autoCapture.err = capture.err
	created, err := auto.CreateAgentAutomation(caller("dev"), "tea-a", automationInput("nightly"))
	if err != nil {
		t.Fatalf("automation create = %v, want success despite the recorder", err)
	}
	autoCapture.only(t, created.ID)
}

func TestAutomationCreateRecordsTheSurfaceThatCreatedIt(t *testing.T) {
	svc, _ := automationTestService("tea-a", newFakeAutomations())
	capture := capturing(svc)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	body, _ := json.Marshal(automationInput("nightly"))
	req := httptest.NewRequest(http.MethodPost, "/v1/agent-automations?ownerId=tea-a", strings.NewReader(string(body))).
		WithContext(origin(caller("dev"), core.TransportREST, cliUserAgent))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
	}
	var created AgentAutomation
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	got := capture.only(t, created.ID)
	if got.WorkspaceID != "tea-a" || got.ResourceType != "agent_automation" || got.Surface != core.SurfaceCLI {
		t.Fatalf("automation fact = %+v, want an agent_automation in tea-a from the cli", got)
	}

	// A refused create (the name is taken) records nothing more.
	if _, err := svc.CreateAgentAutomation(caller("dev"), "tea-a", automationInput("nightly")); !isCode(err, CodeAutomationConflict) {
		t.Fatalf("duplicate name = %v, want %s", err, CodeAutomationConflict)
	}
	capture.only(t, created.ID)
}

// The scheduler has no request at all, and a run-now arrives on whatever
// transport the member used: either way the session is the automation's work.
func TestAutomationFiredSessionsAreAttributedToTheAutomation(t *testing.T) {
	template, _ := json.Marshal(AutomationTemplate{Task: "update the dependencies", Repo: "bex-co/example",
		AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5"}})

	t.Run("scheduled fire", func(t *testing.T) {
		svc, _, _, _ := fixture()
		capture := capturing(svc)
		now := time.Date(2026, time.November, 16, 9, 0, 0, 0, time.UTC)
		table := &claimingAutomations{fakeAutomations: newFakeAutomations(), settled: map[string]string{},
			claims: []store.ClaimedRun{{
				Run:        store.AgentAutomationRun{AutomationID: "aut-nightly", FireAt: now},
				Automation: store.AgentAutomation{ID: "aut-nightly", WorkspaceID: "tea-a", CreatedBy: "alice", Template: template},
			}}}
		svc.Automations = table
		if err := (&AutomationScheduler{Service: svc, Now: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got := table.settled["aut-nightly"]; got != store.AutomationRunStarted {
			t.Fatalf("run settled %q, want %q", got, store.AutomationRunStarted)
		}
		events := capture.recorded()
		if len(events) != 1 || events[0].ResourceType != "agent_session" || events[0].Surface != core.SurfaceAutomation ||
			events[0].WorkspaceID != "tea-a" || events[0].ActorID != "alice" {
			t.Fatalf("product activity = %+v, want one agent_session from the automation, as its creator", events)
		}
	})

	t.Run("run-now over mcp", func(t *testing.T) {
		svc, _, _, _ := fixture()
		capture := capturing(svc)
		table := &claimingAutomations{fakeAutomations: newFakeAutomations(), settled: map[string]string{}}
		svc.Automations = table
		row, err := table.CreateAgentAutomation(context.Background(), store.AgentAutomation{
			ID: ids.New(ids.AgentAutomation), WorkspaceID: "tea-a", Name: "nightly", CreatedBy: "alice", Template: template})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.RunAgentAutomationNow(origin(caller("alice"), core.TransportMCP, ""), "tea-a", row.ID); err != nil {
			t.Fatal(err)
		}
		if got := table.settled[row.ID]; got != store.AutomationRunStarted {
			t.Fatalf("run settled %q, want %q", got, store.AutomationRunStarted)
		}
		events := capture.recorded()
		if len(events) != 1 || events[0].ResourceType != "agent_session" || events[0].Surface != core.SurfaceAutomation {
			t.Fatalf("product activity = %+v, want one agent_session from the automation, not the run-now's transport", events)
		}
	})
}

// A child an agent starts from inside its sandbox goes through the sandbox's
// bex MCP server (spawn_session) and reaches bex-api on the internal listener,
// which records no request origin. It is agent work over MCP, and its
// automation-started parent does not make it the automation's.
func TestSpawnedChildIsAgentWorkOverMCP(t *testing.T) {
	svc, st, fga, _ := fixture()
	capture := capturing(svc)
	config, _ := json.Marshal(AgentConfig{Agent: "codex", Model: "gpt-5", Task: "split the work"})
	parent := store.AgentSession{
		ID: ids.New(ids.AgentSession), WorkspaceID: "tea-a", Repo: "bex-co/example", Phase: PhaseRunning,
		SandboxID: "sbx-parent", AgentConfig: config, CreatedBy: "alice", AutomationID: "aut-nightly", UpdatedAt: st.now,
	}
	st.rows[parent.ID] = parent
	fga.parents[parent.ID] = "tea-a"
	response, err := svc.SpawnSession(context.Background(), agentsession.SpawnEnvelope{
		Kind: agentsession.SpawnKind, Op: agentsession.SpawnOpCreate, Workspace: "tea-a", SessionID: parent.ID,
		PodName: "sbx-parent-0", PodUID: "uid-parent", Spawn: &agentsession.SpawnContent{Task: "write the tests"},
	})
	if err != nil {
		t.Fatalf("spawn = %v", err)
	}
	if got := capture.only(t, response.SessionID); got.ResourceType != "agent_session" || got.Surface != core.SurfaceMCP || got.ActorID != "alice" {
		t.Fatalf("child fact = %+v, want an agent_session over mcp as the parent's creator", got)
	}
}
