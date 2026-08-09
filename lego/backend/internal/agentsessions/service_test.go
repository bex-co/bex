package agentsessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeStore struct {
	rows       map[string]store.AgentSession
	getCalls   int
	now        time.Time
	transcript map[string]map[int64]store.AgentSessionTranscriptPart
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		rows:       map[string]store.AgentSession{},
		now:        time.Unix(1_800_000_000, 0).UTC(),
		transcript: map[string]map[int64]store.AgentSessionTranscriptPart{},
	}
}

func (f *fakeStore) AppendAgentSessionTranscript(_ context.Context, id string, parts []store.AgentSessionTranscriptPart) error {
	if f.transcript[id] == nil {
		f.transcript[id] = map[int64]store.AgentSessionTranscriptPart{}
	}
	for _, p := range parts {
		if _, exists := f.transcript[id][p.Seq]; exists {
			continue // idempotent on (session, seq)
		}
		f.transcript[id][p.Seq] = p
	}
	return nil
}

// AgentSessionTranscript is a test-only read-back (not on the Store interface):
// returns the session's parts in seq order.
func (f *fakeStore) AgentSessionTranscript(_ context.Context, id string, afterSeq int64) ([]store.AgentSessionTranscriptPart, error) {
	out := make([]store.AgentSessionTranscriptPart, 0)
	for seq, p := range f.transcript[id] {
		if seq > afterSeq {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, nil
}

func (f *fakeStore) AgentSessionTranscriptMaxSeq(_ context.Context, id string) (int64, bool, error) {
	max, ok := int64(-1), false
	for seq := range f.transcript[id] {
		if !ok || seq > max {
			max, ok = seq, true
		}
	}
	return max, ok, nil
}

func (f *fakeStore) AgentSessionTranscriptTurnRecorded(_ context.Context, id string, turn int) (bool, error) {
	for _, p := range f.transcript[id] {
		if p.Turn == turn {
			return true, nil
		}
	}
	return false, nil
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
func (f *fakeStore) ListAgentSessionsByPhases(_ context.Context, phases []string) ([]store.AgentSession, error) {
	want := map[string]bool{}
	for _, p := range phases {
		want[p] = true
	}
	out := []store.AgentSession{}
	for _, row := range f.rows {
		if want[row.Phase] {
			out = append(out, row)
		}
	}
	return out, nil
}
func (f *fakeStore) RecordAgentSessionDispatch(_ context.Context, id, sandboxID, phase, status, deliveryMode string) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok {
		return store.AgentSession{}, store.ErrNotFound
	}
	if sandboxID != "" {
		row.SandboxID = sandboxID
	}
	row.Phase, row.Status, row.DeliveryMode = phase, status, deliveryMode
	row.Turns++
	row.UpdatedAt = row.UpdatedAt.Add(time.Second)
	f.rows[id] = row
	return row, nil
}
func (f *fakeStore) FinalizeAgentSession(_ context.Context, id, phase, headSHA, prURL string, prNumber int, evidence json.RawMessage, failureReason string) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok {
		return store.AgentSession{}, store.ErrNotFound
	}
	row.Phase, row.Status, row.FailureReason = phase, phase, failureReason
	if headSHA != "" {
		row.HeadSHA = headSHA
	}
	if prURL != "" {
		row.PRURL = prURL
	}
	if prNumber != 0 {
		row.PRNumber = prNumber
	}
	if evidence != nil {
		row.Evidence = evidence
	}
	row.UpdatedAt = row.UpdatedAt.Add(time.Second)
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
	created, entered, resumed, canceled int
	modelEndpoint, modelAPIKey          string
	egressAllowlist                     []string
	repository, branch                  string
	driverEnv                           map[string]string
	sandboxSeq                          int
	status                              string
	statusErr                           error
	transcriptLog                       string
	transcriptErr                       error
	readTranscript                      int
	readTranscriptBeforeCancel          bool
}

func (f *fakeLifecycle) CreateAgentSessionSandbox(_ context.Context, _, _, _, repository, branch, modelEndpoint, modelAPIKey string, egressAllowlist []string, driverEnv map[string]string) (sandbox.Sandbox, error) {
	f.created++
	f.sandboxSeq++
	f.repository, f.branch = repository, branch
	f.modelEndpoint = modelEndpoint
	f.modelAPIKey = modelAPIKey
	f.egressAllowlist = append([]string(nil), egressAllowlist...)
	f.driverEnv = driverEnv
	return sandbox.Sandbox{ID: fmt.Sprintf("sandbox-%d", f.sandboxSeq), Status: sandbox.StatusRunning}, nil
}
func (f *fakeLifecycle) EnterAgentSessionPhase(_ context.Context, _, _, _, _ string, _ []string) error {
	f.entered++
	return nil
}
func (f *fakeLifecycle) ResumeAgentSessionSandbox(context.Context, string, string, string) error {
	f.resumed++
	return nil
}
func (f *fakeLifecycle) CancelAgentSessionSandbox(context.Context, string, string, string) error {
	f.canceled++
	return nil
}
func (f *fakeLifecycle) ReadSessionStatus(context.Context, string, string, string) (string, error) {
	return f.status, f.statusErr
}
func (f *fakeLifecycle) ReadSessionTranscript(context.Context, string, string, string) (string, error) {
	f.readTranscript++
	f.readTranscriptBeforeCancel = f.canceled == 0
	return f.transcriptLog, f.transcriptErr
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

// fakeModelKeys is a minimal core.SecretKV for pinning the BYO model-key fetch
// (ADR047 D7): Get on a workspace with no stored key returns an empty map (the
// interface's documented not-found contract), never an error.
type fakeModelKeys struct {
	data map[string]map[string]string // path -> KV
	err  error
}

func (f *fakeModelKeys) Get(_ context.Context, path string) (map[string]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.data[path], nil
}
func (f *fakeModelKeys) Put(context.Context, string, map[string]string) error { return nil }
func (f *fakeModelKeys) Delete(context.Context, string) error                 { return nil }
func (f *fakeModelKeys) List(context.Context, string) ([]string, error)       { return nil, nil }

func createInput() CreateRequest {
	return CreateRequest{OwnerID: "tea-a", Repo: "bex-co/example", Branch: "bex-agent/session-test", AgentConfig: AgentConfig{Agent: "codex", Model: "gpt-5", ModelEndpoint: "https://api.openai.com/v1", Task: "fix the tests"}, EgressAllowlist: []string{"docs.example.com"}}
}

func TestLifecycleTicketClaimsAndFirstClassAuthorization(t *testing.T) {
	svc, st, fga, lifecycle := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if created.Phase != PhaseRunning || created.SandboxID != "sandbox-1" || lifecycle.created != 1 || lifecycle.entered != 1 || lifecycle.modelEndpoint != "https://api.openai.com/v1" || !reflect.DeepEqual(lifecycle.egressAllowlist, []string{"docs.example.com"}) || fga.parents[created.ID] != "tea-a" {
		t.Fatalf("create = %+v, parents=%v lifecycle=%+v", created, fga.parents, lifecycle)
	}
	if lifecycle.repository != "bex-co/example" || lifecycle.branch != "bex-agent/session-test" {
		t.Fatalf("sandbox binding = %q %q", lifecycle.repository, lifecycle.branch)
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

// TestCreateInjectsGitCredentialBrokerConfig is a regression guard for a bug
// caught live on prod (w3/m43): the sandbox driver's git-credential-bex helper
// needs BEX_AGENT_CREDENTIAL_URL + BEX_SANDBOX_NAMESPACE + BEX_AGENT_SESSION_ID
// (ADR047 D2) or the setup-phase clone fails with "missing its session broker
// configuration" and every session dies. driverEnv must always inject them.
func TestCreateInjectsGitCredentialBrokerConfig(t *testing.T) {
	svc, _, _, lifecycle := fixture()
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatal(err)
	}
	env := lifecycle.driverEnv
	if env["BEX_AGENT_SESSION_ID"] == "" || env["BEX_AGENT_SESSION_ID"][:4] != "ags-" {
		t.Fatalf("BEX_AGENT_SESSION_ID = %q, want the session id", env["BEX_AGENT_SESSION_ID"])
	}
	if env["BEX_SANDBOX_NAMESPACE"] != "tea-a-sandbox" {
		t.Fatalf("BEX_SANDBOX_NAMESPACE = %q, want tea-a-sandbox", env["BEX_SANDBOX_NAMESPACE"])
	}
	if env["BEX_AGENT_CREDENTIAL_URL"] != defaultCredentialURL {
		t.Fatalf("BEX_AGENT_CREDENTIAL_URL = %q, want the default broker URL", env["BEX_AGENT_CREDENTIAL_URL"])
	}
	if env["BEX_AGENT_REPOSITORY"] != "bex-co/example" || env["BEX_AGENT_BRANCH"] != "bex-agent/session-test" {
		t.Fatalf("repo/branch env = %q/%q", env["BEX_AGENT_REPOSITORY"], env["BEX_AGENT_BRANCH"])
	}
}

// TestAttachTicketMintsWithoutChangingLifecycle pins the w3/m43 reconnect verb
// (ADR047 D9 target API shape): AttachTicket re-mints a fresh, distinct ticket
// bound to the same session/sandbox without advancing the phase, fails closed
// before a sandbox exists, denies cross-workspace callers, and 503s when the
// gateway is unconfigured.
func TestAttachTicketMintsWithoutChangingLifecycle(t *testing.T) {
	svc, st, _, lifecycle := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	beforeCreated, beforeEntered := lifecycle.created, lifecycle.entered

	attached, err := svc.AttachTicket(caller("alice"), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if attached.Phase != PhaseRunning || attached.Ticket == "" || attached.Ticket == created.Ticket {
		t.Fatalf("attach did not mint a fresh ticket without a lifecycle change: %+v", attached)
	}
	if lifecycle.created != beforeCreated || lifecycle.entered != beforeEntered {
		t.Fatalf("attach touched the sandbox lifecycle: %+v", lifecycle)
	}
	claims, err := agentsessionticket.Verify(svc.TicketSecret, attached.Ticket, st.now)
	if err != nil || claims.SessionID != created.ID || claims.SandboxID != "sandbox-1" || claims.Subject != "alice" {
		t.Fatalf("attach ticket claims = %+v err=%v", claims, err)
	}

	// A session with no sandbox yet is not attachable.
	pending, err := st.CreateAgentSession(caller("alice"), store.AgentSession{WorkspaceID: "tea-a", Repo: "bex-co/example", Branch: "bex-agent/x", AgentConfig: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	svc.Tuples.GrantAgentSessionWorkspace(caller("alice"), pending.ID, "tea-a")
	if _, err := svc.AttachTicket(caller("alice"), pending.ID); !isCode(err, "AGENT_SESSION_NOT_ATTACHABLE") {
		t.Fatalf("attach to sandbox-less session = %v, want AGENT_SESSION_NOT_ATTACHABLE", err)
	}

	// Cross-workspace caller is denied by the first-class tuple.
	if _, err := svc.AttachTicket(caller("bob"), created.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-workspace attach = %v, want forbidden", err)
	}

	// Gateway unconfigured => 503 on the mint verb, like create/resume/steer.
	svc.GatewayURL = ""
	if _, err := svc.AttachTicket(caller("alice"), created.ID); !errors.Is(err, core.ErrAgentSessionsUnavailable) {
		t.Fatalf("attach with no gateway = %v, want unavailable", err)
	}
}

// TestCreateInjectsWorkspaceScopedModelKey pins ADR047 D7: a workspace's BYO
// model key is fetched from a workspace-scoped OpenBao path at session-create
// time and threaded through to the sandbox lifecycle — and a DIFFERENT
// workspace's key at the same logical path never leaks across, proving the
// path really is workspace-scoped (internal/sandbox's ModelAPIKeyEnvVar
// convention, service.go's modelKeySecretPath).
func TestCreateInjectsWorkspaceScopedModelKey(t *testing.T) {
	svc, _, _, lifecycle := fixture()
	svc.ModelKeys = &fakeModelKeys{data: map[string]map[string]string{
		modelKeySecretPath("tea-a"): {sandbox.ModelAPIKeyEnvVar: "sk-tea-a-secret"},
		modelKeySecretPath("tea-b"): {sandbox.ModelAPIKeyEnvVar: "sk-tea-b-secret"},
	}}
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatal(err)
	}
	if lifecycle.modelAPIKey != "sk-tea-a-secret" {
		t.Fatalf("modelAPIKey = %q, want tea-a's own key (not tea-b's, not empty)", lifecycle.modelAPIKey)
	}
}

// TestCreateWithNoProvisionedModelKeyStartsAnyway pins the common case: most
// workspaces have not provisioned a BYO key yet, and that must never block
// session creation — only a genuine store error should.
func TestCreateWithNoProvisionedModelKeyStartsAnyway(t *testing.T) {
	svc, _, _, lifecycle := fixture()
	svc.ModelKeys = &fakeModelKeys{data: map[string]map[string]string{}}
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatal(err)
	}
	if lifecycle.modelAPIKey != "" {
		t.Fatalf("modelAPIKey = %q, want empty (nothing provisioned)", lifecycle.modelAPIKey)
	}
}

// TestCreateFailsClosedOnModelKeyStoreError proves a genuine OpenBao failure
// refuses the create rather than silently starting a keyless session the
// agent could never authenticate from.
func TestCreateFailsClosedOnModelKeyStoreError(t *testing.T) {
	svc, _, _, lifecycle := fixture()
	svc.ModelKeys = &fakeModelKeys{err: errors.New("openbao unreachable")}
	if _, err := svc.Create(caller("alice"), createInput()); !errors.Is(err, core.ErrSecretsUnavailable) {
		t.Fatalf("create with a broken model-key store = %v, want ErrSecretsUnavailable", err)
	}
	if lifecycle.created != 0 {
		t.Fatal("sandbox was created despite the model-key store failing")
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
	result := graphql.Do(graphql.Params{Schema: schema, Context: caller("alice"), RequestString: `mutation { createAgentSession(ownerId:"tea-a", repo:"bex-co/example", branch:"bex-agent/session-test", agentConfig:{agent:"codex",model:"gpt-5",modelEndpoint:"https://api.openai.com/v1",task:"fix the tests"}, egressAllowlist:["docs.example.com"]) { id ownerId repo branch agentConfig { agent model modelEndpoint task template } sandboxId phase status ticket url expiresAt } }`})
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
		"ownerId": "tea-a", "repo": "bex-co/example", "branch": "bex-agent/session-test",
		"agentConfig":     map[string]any{"agent": "codex", "model": "gpt-5", "modelEndpoint": "https://api.openai.com/v1", "task": "fix the tests"},
		"egressAllowlist": []any{"docs.example.com"},
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

// fakeGitHub is the mobile-safe readiness source: it returns the neutral
// github.Connection the projection consumes, never installation tokens/ids.
type fakeGitHub struct {
	conn map[string]github.Connection
	err  error
}

func (f *fakeGitHub) GetConnection(_ context.Context, ownerID string) (github.Connection, error) {
	if f.err != nil {
		return github.Connection{}, f.err
	}
	return f.conn[ownerID], nil
}

// TestCapabilitiesProjection pins w11/m6 t001: the mobile composer projection
// reveals only selectable profiles + readiness, computes Ready from both
// provisioning gates, denies cross-workspace callers, and never carries a
// secret field (model endpoint, template, egress, installation id).
func TestCapabilitiesProjection(t *testing.T) {
	svc, _, _, _ := fixture()
	svc.GitHub = &fakeGitHub{conn: map[string]github.Connection{
		"tea-a": {Connected: true, AccountLogin: "octo-a", InstallURL: "https://github.com/apps/bex/installations/new"},
		"tea-b": {Connected: false, InstallURL: "https://github.com/apps/bex/installations/new"},
	}}
	svc.ModelKeys = &fakeModelKeys{data: map[string]map[string]string{
		modelKeySecretPath("tea-a"): {sandbox.ModelAPIKeyEnvVar: "sk-tea-a"},
	}}

	// Fully provisioned workspace: Ready, profiles present, GitHub connected.
	caps, err := svc.Capabilities(caller("alice"), "")
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Enabled || !caps.Ready || !caps.ModelKeyReady || !caps.GitHub.Connected {
		t.Fatalf("fully provisioned tea-a should be Ready: %#v", caps)
	}
	if caps.GitHub.AccountLogin != "octo-a" || len(caps.Agents) == 0 {
		t.Fatalf("expected connected account + selectable agents: %#v", caps)
	}

	// Not-ready workspace: no model key + GitHub not connected => Ready false,
	// but Enabled true and the install URL is offered as remediation.
	capsB, err := svc.Capabilities(caller("bob"), "")
	if err != nil {
		t.Fatal(err)
	}
	if capsB.Ready || capsB.ModelKeyReady || capsB.GitHub.Connected {
		t.Fatalf("tea-b is unprovisioned; must not be Ready: %#v", capsB)
	}
	if !capsB.Enabled || capsB.GitHub.InstallURL == "" {
		t.Fatalf("unready workspace must still be Enabled with an install-URL remediation: %#v", capsB)
	}

	// Cross-workspace denial: bob cannot read tea-a's capabilities.
	if _, err := svc.Capabilities(caller("bob"), "tea-a"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-workspace capabilities = %v, want ErrForbidden", err)
	}

	// No secret field ever crosses the projection.
	raw, err := json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"modelEndpoint", "template", "egress", "installationId", "ticket", "task"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("capabilities JSON leaked a secret-shaped field %q: %s", secret, raw)
		}
	}
}

// TestCapabilitiesUnavailableWhenNotWired: with the feature not wired (no
// gateway), the projection reports Enabled:false rather than erroring, so the
// phone renders a desktop-configuration callout.
func TestCapabilitiesUnavailableWhenNotWired(t *testing.T) {
	svc, _, _, _ := fixture()
	svc.GatewayURL = "" // ticketEnabled() => false => feature not wired
	caps, err := svc.Capabilities(caller("alice"), "")
	if err != nil {
		t.Fatal(err)
	}
	if caps.Enabled || caps.Ready || len(caps.Agents) != 0 {
		t.Fatalf("unwired feature must report Enabled:false with no profiles: %#v", caps)
	}
}
