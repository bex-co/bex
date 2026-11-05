package agentsessions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// seedSession adds a workspace-tea-a session row readable by alice.
func seedSession(st *fakeStore, fga *fakeFGA, mutate func(*store.AgentSession)) string {
	id := seedLiveSession(st, "tea-a")
	row := st.rows[id]
	row.AgentConfig = []byte(`{"agent":"codex"}`)
	mutate(&row)
	st.rows[id] = row
	fga.parents[id] = "tea-a"
	return id
}

// seedCompletedTurn gives a session a completed turn whose complete items are
// the given parts (JSON), in part_index order.
func seedCompletedTurn(st *fakeStore, id string, turn int, parts ...string) {
	if st.turns[id] == nil {
		st.turns[id] = map[int]store.AgentSessionTurn{}
	}
	done := st.now
	st.turns[id][turn] = store.AgentSessionTurn{SessionID: id, Turn: turn, CompletedAt: &done}
	if st.transcript[id] == nil {
		st.transcript[id] = map[int64]store.AgentSessionTranscriptPart{}
	}
	for i, part := range parts {
		seq := int64(len(st.transcript[id]) + 1)
		st.transcript[id][seq] = store.AgentSessionTranscriptPart{Seq: seq, PartIndex: int64(i), Turn: turn, Part: []byte(part), Item: true}
	}
}

func TestOutcomeSummaryIsFinalAssistantTextOfLatestCompletedTurn(t *testing.T) {
	svc, st, fga, _ := fixture()
	var row store.AgentSession
	id := seedSession(st, fga, func(r *store.AgentSession) {
		r.Phase, r.Turns, r.PRURL, r.PRNumber, r.HeadSHA = PhaseCompleted, 2, "https://github.com/o/r/pull/412", 412, "9f2c"
		r.DeliveryNote = "branch pushed, no PR"
		row = *r
	})
	seedCompletedTurn(st, id, 1, `{"type":"text","text":"turn one answer"}`)
	seedCompletedTurn(st, id, 2,
		`{"type":"text","text":"early thought"}`,
		`{"type":"dynamic-tool","toolName":"bash"}`,
		`{"type":"text","text":"Added retry with jitter; tests pass."}`,
		`{"type":"reasoning","text":"private"}`)

	got, err := svc.Get(caller("alice"), id)
	if err != nil {
		t.Fatal(err)
	}
	o := got.Outcome
	if o == nil || o.Summary != "Added retry with jitter; tests pass." {
		t.Fatalf("outcome = %+v, want the final text part of turn 2", o)
	}
	if o.State != StateSucceeded || !o.Steerable || o.Turn != 2 {
		t.Fatalf("outcome state = %+v", o)
	}
	if o.PullRequest == nil || o.PullRequest.URL != row.PRURL || o.PullRequest.Number != 412 || o.PullRequest.HeadSHA != "9f2c" {
		t.Fatalf("pullRequest = %+v", o.PullRequest)
	}
	if o.DeliveryNote != "branch pushed, no PR" {
		t.Fatalf("deliveryNote = %q", o.DeliveryNote)
	}
}

func TestOutcomeSummaryClipsHeadAndTail(t *testing.T) {
	svc, st, fga, _ := fixture()
	id := seedSession(st, fga, func(r *store.AgentSession) { r.Phase, r.Turns = PhaseCompleted, 1 })
	long := strings.Repeat("H", 3000) + strings.Repeat("M", 5000) + strings.Repeat("T", 3000)
	seedCompletedTurn(st, id, 1, `{"type":"text","text":"`+long+`"}`)

	got, err := svc.Get(caller("alice"), id)
	if err != nil {
		t.Fatal(err)
	}
	s := got.Outcome.Summary
	if !strings.HasPrefix(s, strings.Repeat("H", 2000)) || !strings.HasSuffix(s, strings.Repeat("T", 2000)) {
		t.Fatalf("summary lost its head or tail: len=%d", len(s))
	}
	if !strings.Contains(s, "[bex: 7000 chars clipped]") || strings.Contains(s, "M") {
		t.Fatalf("summary not clipped in the middle: %q", s[2000:2100])
	}
}

func TestOutcomeWithoutCompletedTurnHasNoSummary(t *testing.T) {
	svc, st, fga, _ := fixture()
	id := seedSession(st, fga, func(*store.AgentSession) {}) // running, no turn completed yet
	got, err := svc.Get(caller("alice"), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Outcome == nil || got.Outcome.Summary != "" || got.Outcome.State != StateWorking || got.Outcome.Steerable || got.Outcome.PullRequest != nil {
		t.Fatalf("outcome = %+v", got.Outcome)
	}
}

func TestOutcomeCarriesFailureCodeAndListSkipsSummary(t *testing.T) {
	svc, st, fga, _ := fixture()
	id := seedSession(st, fga, func(r *store.AgentSession) {
		r.Phase, r.Turns, r.FailureReasonCode = PhaseFailed, 1, "SANDBOX_CAPACITY_LIMIT"
	})
	seedCompletedTurn(st, id, 1, `{"type":"text","text":"could not start"}`)

	views, err := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Outcome == nil {
		t.Fatalf("list views = %+v", views)
	}
	o := views[0].Outcome
	if o.State != StateFailed || o.FailureReasonCode != "SANDBOX_CAPACITY_LIMIT" || o.Summary != "" {
		t.Fatalf("list outcome = %+v (summary must be Get-only)", o)
	}
}

// TestEverySurfaceServesStateAndOutcome (ADR094 D7): REST, GraphQL and MCP
// project one View, so a finished session reads identically on all three.
func TestEverySurfaceServesStateAndOutcome(t *testing.T) {
	svc, st, fga, _ := fixture()
	id := seedSession(st, fga, func(r *store.AgentSession) {
		r.Phase, r.Turns, r.PRURL, r.PRNumber = PhaseCompleted, 1, "https://github.com/o/r/pull/7", 7
	})
	seedCompletedTurn(st, id, 1, `{"type":"text","text":"All done."}`)
	ctx := caller("alice")

	check := func(surface string, state string, steerable bool, summary, prURL any) {
		t.Helper()
		if state != StateSucceeded || !steerable || summary != "All done." || prURL != "https://github.com/o/r/pull/7" {
			t.Errorf("%s: state=%v steerable=%v summary=%v pr=%v", surface, state, steerable, summary, prURL)
		}
	}

	// REST
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/agent-sessions/"+id, nil).WithContext(ctx))
	var rest struct {
		State     string
		Steerable bool
		Outcome   struct {
			Summary     string
			PullRequest struct{ URL string }
		}
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &rest) != nil {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
	}
	check("REST", rest.State, rest.Steerable, rest.Outcome.Summary, rest.Outcome.PullRequest.URL)

	// GraphQL
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ agentSession(id: "` + id + `") { state steerable outcome { summary pullRequest { url } } } }`})
	if len(res.Errors) != 0 {
		t.Fatalf("GraphQL errors = %v", res.Errors)
	}
	g := res.Data.(map[string]any)["agentSession"].(map[string]any)
	o := g["outcome"].(map[string]any)
	check("GraphQL", g["state"].(string), g["steerable"].(bool), o["summary"], o["pullRequest"].(map[string]any)["url"])

	// MCP
	client := newMCPClient(t, ctx, svc)
	call, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "get_agent_session", Arguments: map[string]any{"id": id}})
	if err != nil || call.IsError {
		t.Fatalf("MCP = %+v err=%v", call, err)
	}
	raw, _ := json.Marshal(call.StructuredContent)
	var m struct {
		State     string
		Steerable bool
		Outcome   struct {
			Summary     string
			PullRequest struct{ URL string }
		}
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	check("MCP", m.State, m.Steerable, m.Outcome.Summary, m.Outcome.PullRequest.URL)
}

// outcome.files are the files the agent shared in the summary's turn (ADR095
// D11): outputs of the latest completed turn only — never the user's inputs,
// and never an earlier turn's outputs.
func TestOutcomeFilesAreTheLatestCompletedTurnsSharedFiles(t *testing.T) {
	svc, st, fga, _ := fixture()
	rows := &fakeAttachmentRows{rows: map[string]store.AgentAttachment{}, state: map[string]string{}}
	svc.Attachments, svc.AttachmentRows = &fakeObjects{data: map[string][]byte{}}, rows
	id := seedSession(st, fga, func(r *store.AgentSession) { r.Phase, r.Turns = PhaseCompleted, 2 })
	seedCompletedTurn(st, id, 1, `{"type":"text","text":"one"}`)
	seedCompletedTurn(st, id, 2, `{"type":"text","text":"two"}`)
	for _, a := range []store.AgentAttachment{
		{ID: "att-old", SessionID: id, Turn: 1, Direction: store.AgentAttachmentOutput, Name: "old.csv", MediaType: "text/csv", SizeBytes: 3},
		{ID: "att-in", SessionID: id, Turn: 2, Direction: store.AgentAttachmentInput, Name: "input.log", MediaType: "text/plain", SizeBytes: 4},
		{ID: "att-out", SessionID: id, Turn: 2, Direction: store.AgentAttachmentOutput, Name: "report.csv", MediaType: "text/csv", SizeBytes: 26},
	} {
		rows.rows[a.ID], rows.state[a.ID] = a, "ready"
	}

	got, err := svc.Get(caller("alice"), id)
	if err != nil {
		t.Fatal(err)
	}
	want := []TurnAttachment{{ID: "att-out", Name: "report.csv", MediaType: "text/csv", Size: 26}}
	if got.Outcome == nil || !slices.Equal(got.Outcome.Files, want) {
		t.Fatalf("outcome.files = %+v, want %+v", got.Outcome, want)
	}
}
