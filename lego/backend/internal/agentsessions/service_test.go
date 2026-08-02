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

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeStore struct {
	rows     map[string]store.AgentSession
	getCalls int
	now      time.Time
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[string]store.AgentSession{}, now: time.Unix(1_800_000_000, 0).UTC()}
}

func (f *fakeStore) CreateAgentSession(_ context.Context, in store.AgentSession) (store.AgentSession, error) {
	in.ID, in.Phase, in.CreatedAt, in.UpdatedAt = ids.New(ids.AgentSession), PhaseCreating, f.now, f.now
	f.rows[in.ID] = in
	return in, nil
}
func (f *fakeStore) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	f.getCalls++
	row, ok := f.rows[id]
	if !ok {
		return store.AgentSession{}, store.ErrNotFound
	}
	return row, nil
}
func (f *fakeStore) ListAgentSessions(_ context.Context, workspace string) ([]store.AgentSession, error) {
	out := []store.AgentSession{}
	for _, row := range f.rows {
		if row.WorkspaceID == workspace {
			out = append(out, row)
		}
	}
	return out, nil
}
func (f *fakeStore) SetAgentSessionLifecycle(_ context.Context, id, sandboxID, phase, status string, canceled bool) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok {
		return store.AgentSession{}, store.ErrNotFound
	}
	if sandboxID != "" {
		row.SandboxID = sandboxID
	}
	row.Phase, row.Status = phase, status
	row.UpdatedAt = row.UpdatedAt.Add(time.Second)
	if canceled && row.CanceledAt == nil {
		at := row.UpdatedAt
		row.CanceledAt = &at
	}
	f.rows[id] = row
	return row, nil
}
func (f *fakeStore) DeleteAgentSession(_ context.Context, id string) error {
	delete(f.rows, id)
	return nil
}

type fakeFGA struct {
	members map[string]string // subject -> workspace
	parents map[string]string // session id -> workspace
	fail    bool
}

type checkerFunc func(context.Context, string, string, string) (bool, error)

func (f checkerFunc) Check(ctx context.Context, subject, relation, object string) (bool, error) {
	return f(ctx, subject, relation, object)
}

func (f *fakeFGA) Check(_ context.Context, subject, relation, object string) (bool, error) {
	if f.fail {
		return false, errors.New("fga unavailable")
	}
	subject = strings.TrimPrefix(subject, "user:")
	workspace := ""
	switch {
	case strings.HasPrefix(object, "workspace:"):
		workspace = strings.TrimPrefix(object, "workspace:")
	case strings.HasPrefix(object, "agent_session:"):
		workspace = f.parents[strings.TrimPrefix(object, "agent_session:")]
	}
	return relation == core.RelCanOperate && workspace != "" && f.members[subject] == workspace, nil
}
func (f *fakeFGA) GrantAgentSessionWorkspace(_ context.Context, sessionID, workspaceID string) error {
	if f.fail {
		return errors.New("fga write unavailable")
	}
	f.parents[sessionID] = workspaceID
	return nil
}

type resolver struct{ members map[string]string }

func (r resolver) Tenant(_ context.Context, id core.Identity) (string, bool) {
	ws := r.members[id.Subject]
	return ws, ws != ""
}
func (r resolver) IsMember(_ context.Context, id core.Identity, workspace string) (bool, error) {
	return r.members[id.Subject] == workspace, nil
}

type fakeLifecycle struct {
	created, resumed, canceled int
}

func (f *fakeLifecycle) CreateAgentSessionSandbox(_ context.Context, _, _, _ string) (sandbox.Sandbox, error) {
	f.created++
	return sandbox.Sandbox{ID: "sandbox-1", Status: sandbox.StatusRunning}, nil
}
func (f *fakeLifecycle) ResumeAgentSessionSandbox(context.Context, string, string, string) error {
	f.resumed++
	return nil
}
func (f *fakeLifecycle) CancelAgentSessionSandbox(context.Context, string, string, string) error {
	f.canceled++
	return nil
}

func fixture() (*Service, *fakeStore, *fakeFGA, *fakeLifecycle) {
	st, fga, lifecycle := newFakeStore(), &fakeFGA{members: map[string]string{"alice": "tea-a", "bob": "tea-b"}, parents: map[string]string{}}, &fakeLifecycle{}
	svc := &Service{
		Base:  &core.Base{Authz: fga, Workspace: resolver{fga.members}, Clock: func() time.Time { return st.now }},
		Store: st, Tuples: fga, Sandbox: lifecycle, TicketSecret: []byte("session-secret"), GatewayURL: "wss://ssh.bex.co/agent-sessions",
	}
	return svc, st, fga, lifecycle
}

func caller(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func createInput() CreateRequest {
	return CreateRequest{OwnerID: "tea-a", Repo: "bex-co/example", Branch: "main", AgentConfig: AgentConfig{Agent: "codex", Model: "gpt-5", Task: "fix the tests"}}
}

func TestLifecycleTicketClaimsAndFirstClassAuthorization(t *testing.T) {
	svc, st, fga, lifecycle := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if created.Phase != PhaseRunning || created.SandboxID != "sandbox-1" || lifecycle.created != 1 || fga.parents[created.ID] != "tea-a" {
		t.Fatalf("create = %+v, parents=%v lifecycle=%+v", created, fga.parents, lifecycle)
	}
	claims, err := agentsessionticket.Verify(svc.TicketSecret, created.Ticket, st.now)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "alice" || claims.SessionID != created.ID || claims.SandboxID != "sandbox-1" || claims.Pod != "sandbox-1-0" || claims.Workspace != "tea-a" || claims.Namespace != "tea-a-sandbox" || claims.Nonce == "" {
		t.Fatalf("claims = %+v", claims)
	}

	// The DB row alone is deliberately insufficient: removing the first-class
	// parent tuple denies before the store is consulted.
	delete(fga.parents, created.ID)
	before := st.getCalls
	if _, err := svc.Get(caller("alice"), created.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("tuple-less row = %v, want forbidden", err)
	}
	if st.getCalls != before {
		t.Fatal("tuple denial consulted the store and became an existence oracle")
	}
	fga.parents[created.ID] = "tea-a"

	resumed, err := svc.Resume(caller("alice"), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lifecycle.resumed != 1 || resumed.Ticket == created.Ticket {
		t.Fatalf("resume did not run/mint a fresh ticket: %+v", resumed)
	}
	canceled, err := svc.Cancel(caller("alice"), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if canceled.Phase != PhaseCanceled || canceled.CanceledAt == nil || lifecycle.canceled != 1 {
		t.Fatalf("cancel = %+v lifecycle=%+v", canceled, lifecycle)
	}
	again, err := svc.Cancel(caller("alice"), created.ID)
	if err != nil || again.Phase != PhaseCanceled || lifecycle.canceled != 1 {
		t.Fatalf("idempotent cancel = %+v err=%v lifecycle=%+v", again, err, lifecycle)
	}
}

func TestCrossWorkspaceSessionDeniedByTuple(t *testing.T) {
	svc, st, _, lifecycle := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	before := st.getCalls
	if _, err := svc.Get(caller("bob"), created.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("foreign get = %v, want forbidden", err)
	}
	if _, err := svc.Cancel(caller("bob"), created.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("foreign cancel = %v, want forbidden", err)
	}
	if st.getCalls != before || lifecycle.canceled != 0 {
		t.Fatalf("foreign call crossed FGA boundary: gets=%d/%d lifecycle=%+v", st.getCalls, before, lifecycle)
	}
}

func TestTupleWriteFailureCompensatesBeforeSandbox(t *testing.T) {
	svc, st, fga, lifecycle := fixture()
	fga.fail = true
	// Let the workspace check succeed, then fail just the tuple write.
	svc.Base.Authz = checkerFunc(func(context.Context, string, string, string) (bool, error) { return true, nil })
	if _, err := svc.Create(caller("alice"), createInput()); !errors.Is(err, core.ErrAuthzUnavailable) {
		t.Fatalf("create = %v, want authz unavailable", err)
	}
	if len(st.rows) != 0 || lifecycle.created != 0 {
		t.Fatalf("unreachable resource survived: rows=%d created=%d", len(st.rows), lifecycle.created)
	}
}

func TestRESTGraphQLMCPCreateParity(t *testing.T) {
	want := createInput()
	check := func(t *testing.T, got View) {
		t.Helper()
		if got.OwnerID != want.OwnerID || got.Repo != want.Repo || got.Branch != want.Branch || got.AgentConfig != want.AgentConfig || got.Phase != PhaseRunning || got.Ticket == "" || got.URL == "" {
			t.Fatalf("surface view = %+v", got)
		}
	}

	// REST.
	restSvc, _, _, _ := fixture()
	mux := http.NewServeMux()
	restSvc.RegisterREST(mux)
	body, _ := json.Marshal(want)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/agent-sessions", strings.NewReader(string(body))).WithContext(caller("alice"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body.String())
	}
	var rest View
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}
	check(t, rest)

	// GraphQL.
	gqlSvc, _, _, _ := fixture()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: gqlSvc.GraphQLQuery()}), Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: gqlSvc.GraphQLMutation()})})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: `mutation { createAgentSession(ownerId:"tea-a", repo:"bex-co/example", branch:"main", agentConfig:{agent:"codex",model:"gpt-5",task:"fix the tests"}) { id ownerId repo branch agentConfig { agent model task template } sandboxId phase status ticket url expiresAt } }`})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL errors = %#v", result.Errors)
	}
	raw, _ := json.Marshal(result.Data.(map[string]any)["createAgentSession"])
	var gqlView View
	if err := json.Unmarshal(raw, &gqlView); err != nil {
		t.Fatal(err)
	}
	check(t, gqlView)

	// MCP.
	mcpSvc, _, _, _ := fixture()
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-session-test", Version: "0"}, nil)
	mcpSvc.RegisterMCP(server)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := caller("alice")
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	mcpResult, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "spawn_agent_session", Arguments: map[string]any{
		"ownerId": "tea-a", "repo": "bex-co/example", "branch": "main",
		"agentConfig": map[string]any{"agent": "codex", "model": "gpt-5", "task": "fix the tests"},
	}})
	if err != nil || mcpResult.IsError {
		t.Fatalf("MCP = %+v err=%v", mcpResult, err)
	}
	raw, _ = json.Marshal(mcpResult.StructuredContent)
	var mcpView View
	if err := json.Unmarshal(raw, &mcpView); err != nil {
		t.Fatal(err)
	}
	check(t, mcpView)
}

func TestViewFieldsAndCodedErrorsMatchEverySurface(t *testing.T) {
	// REST and MCP serialize View directly. Pin GraphQL to the same field set so
	// adding a field to one public representation cannot silently omit it from
	// the third surface.
	wantFields := map[string]bool{}
	viewType := reflect.TypeFor[View]()
	for i := 0; i < viewType.NumField(); i++ {
		name := strings.Split(viewType.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			wantFields[name] = true
		}
	}
	for name := range wantFields {
		if agentSessionGQLType.Fields()[name] == nil {
			t.Errorf("View JSON field %q is missing from GraphQL AgentSession", name)
		}
	}
	if len(agentSessionGQLType.Fields()) != len(wantFields) {
		t.Errorf("GraphQL fields=%d View JSON fields=%d", len(agentSessionGQLType.Fields()), len(wantFields))
	}

	// The shared domain validation error keeps its code in REST's body,
	// GraphQL extensions, and MCP's tool error text.
	restSvc, _, _, _ := fixture()
	mux := http.NewServeMux()
	restSvc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/agent-sessions", strings.NewReader(`{"ownerId":"tea-a"}`)).WithContext(caller("alice"))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("REST coded error = %d %s", rec.Code, rec.Body.String())
	}

	gqlSvc, _, _, _ := fixture()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: gqlSvc.GraphQLQuery()}), Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: gqlSvc.GraphQLMutation()})})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: `mutation { createAgentSession(ownerId:"tea-a", repo:"", branch:"", agentConfig:{agent:"",task:""}) { id } }`})
	if len(result.Errors) != 1 || result.Errors[0].Extensions["code"] != "AGENT_SESSION_INPUT_INVALID" {
		t.Fatalf("GraphQL coded error = %#v", result.Errors)
	}

	mcpSvc, _, _, _ := fixture()
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-session-errors", Version: "0"}, nil)
	mcpSvc.RegisterMCP(server)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := caller("alice")
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	mcpResult, err := client.CallTool(ctx, &mcp.CallToolParams{Name: "spawn_agent_session", Arguments: map[string]any{"ownerId": "tea-a"}})
	if err != nil || !mcpResult.IsError || !strings.Contains(mcpResult.Content[0].(*mcp.TextContent).Text, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("MCP coded error = %+v err=%v", mcpResult, err)
	}
}
