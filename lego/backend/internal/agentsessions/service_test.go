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

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
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
	// openSSH models the ssh_sessions table for the teardown-deferral tests
	// (ADR054 D6): resource id -> the started_at of a still-open editor session.
	openSSH map[string]time.Time
	// lastSSHEnd models the most recent editor SSH disconnect for the idle-grace
	// tests (ADR059 D2 / w2/m67): resource id -> ended_at of the last closed
	// session. Absent ⇒ no prior SSH disconnect for that resource.
	lastSSHEnd map[string]time.Time
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		rows:       map[string]store.AgentSession{},
		now:        time.Unix(1_800_000_000, 0).UTC(),
		transcript: map[string]map[int64]store.AgentSessionTranscriptPart{},
		openSSH:    map[string]time.Time{},
		lastSSHEnd: map[string]time.Time{},
	}
}

func TestAgentSelectorIsClosedAndResolvesOnlyAbsoluteImagePaths(t *testing.T) {
	for id, want := range map[string]string{
		"claude": "/usr/local/bin/claude-code-acp",
		"codex":  "/usr/local/bin/codex-acp",
		"gemini": "/usr/local/bin/gemini",
	} {
		req := CreateRequest{Repo: "bex-co/example", Branch: "bex-agent/test", AgentConfig: AgentConfig{Agent: id, Task: "test"}}
		if err := validateCreate(&req); err != nil {
			t.Fatalf("validate supported agent %q: %v", id, err)
		}
		if got := agentCommand(req.AgentConfig.Agent); got != want || !strings.HasPrefix(got, "/") {
			t.Errorf("agentCommand(%q) = %q, want absolute %q", id, got, want)
		}
	}
	for _, agent := range []string{"./adapter", "/workspace/adapter", "claude-code-acp", "unknown"} {
		req := CreateRequest{Repo: "bex-co/example", Branch: "bex-agent/test", AgentConfig: AgentConfig{Agent: agent, Task: "test"}}
		if err := validateCreate(&req); err == nil {
			t.Errorf("validateCreate accepted unsupported agent %q", agent)
		}
	}
}

func (f *fakeStore) AgentSessionSSHActivity(_ context.Context, resourceID string, freshSince time.Time) (bool, *time.Time, error) {
	started, ok := f.openSSH[resourceID]
	hasFreshOpen := ok && !started.Before(freshSince)
	var lastEnded *time.Time
	if ended, ok := f.lastSSHEnd[resourceID]; ok {
		e := ended
		lastEnded = &e
	}
	return hasFreshOpen, lastEnded, nil
}

func (f *fakeStore) CountLiveAgentSessionSandboxes(_ context.Context, workspaceID string, phases []string) (int, error) {
	live := map[string]bool{}
	for _, p := range phases {
		live[p] = true
	}
	n := 0
	for _, row := range f.rows {
		if row.WorkspaceID == workspaceID && row.SandboxID != "" && live[row.Phase] {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) ClaimAgentSessionForHibernation(_ context.Context, id string) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok || row.SandboxID == "" || (row.Phase != PhaseCompleted && row.Phase != PhaseFailed) {
		return store.AgentSession{}, store.ErrNotFound
	}
	row.Phase, row.Status = PhaseHibernating, "hibernating"
	row.UpdatedAt = row.UpdatedAt.Add(time.Second)
	f.rows[id] = row
	return row, nil
}

func (f *fakeStore) HibernateAgentSession(_ context.Context, id, ref string, bytes int64, sha string, retainUntil time.Time) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok || row.Phase != PhaseHibernating {
		return store.AgentSession{}, store.ErrNotFound
	}
	row.Phase, row.Status = PhaseHibernated, "hibernated"
	row.SnapshotRef, row.SnapshotBytes, row.SnapshotSHA = ref, bytes, sha
	at := row.UpdatedAt.Add(time.Second)
	row.HibernatedAt = &at
	if !row.Pinned {
		ru := retainUntil
		row.RetainUntil = &ru
	}
	row.UpdatedAt = at
	f.rows[id] = row
	return row, nil
}

func (f *fakeStore) BeginRehydrate(_ context.Context, id string) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok || row.Phase != PhaseHibernated || row.SnapshotRef == "" {
		return store.AgentSession{}, store.ErrNotFound
	}
	row.Phase, row.Status = PhaseResuming, "rehydrating"
	row.UpdatedAt = row.UpdatedAt.Add(time.Second)
	f.rows[id] = row
	return row, nil
}

func (f *fakeStore) AbortRehydrate(_ context.Context, id string) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok || row.Phase != PhaseResuming || row.SnapshotRef == "" || row.SandboxID != "" {
		return store.AgentSession{}, store.ErrNotFound
	}
	row.Phase, row.Status = PhaseHibernated, "hibernated"
	row.UpdatedAt = row.UpdatedAt.Add(time.Second)
	f.rows[id] = row
	return row, nil
}

func (f *fakeStore) RehydrateAgentSession(_ context.Context, id, sandboxID, phase, status, delivery string) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok || row.Phase != PhaseResuming || row.SandboxID != "" {
		return store.AgentSession{}, store.ErrNotFound
	}
	row.SandboxID, row.Phase, row.Status, row.DeliveryMode = sandboxID, phase, status, delivery
	row.Turns++
	row.SnapshotRef, row.SnapshotBytes, row.SnapshotSHA = "", 0, ""
	row.HibernatedAt, row.RetainUntil = nil, nil
	row.UpdatedAt = row.UpdatedAt.Add(time.Second)
	f.rows[id] = row
	return row, nil
}

func (f *fakeStore) SetAgentSessionPinned(_ context.Context, id string, pinned bool, retainUntil *time.Time) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok {
		return store.AgentSession{}, store.ErrNotFound
	}
	row.Pinned = pinned
	switch {
	case pinned:
		row.RetainUntil = nil
	case row.Phase == PhaseHibernated:
		row.RetainUntil = retainUntil
	}
	row.UpdatedAt = row.UpdatedAt.Add(time.Second)
	f.rows[id] = row
	return row, nil
}

func (f *fakeStore) ListHibernatedForRetention(_ context.Context, now time.Time, _ int) ([]store.AgentSession, error) {
	out := []store.AgentSession{}
	for _, row := range f.rows {
		if row.Phase == PhaseHibernated && !row.Pinned && row.SnapshotRef != "" &&
			row.RetainUntil != nil && !row.RetainUntil.After(now) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeStore) ExpireHibernatedAgentSession(_ context.Context, id, ref string) (store.AgentSession, error) {
	row, ok := f.rows[id]
	if !ok || row.Phase != PhaseHibernated || row.SnapshotRef != ref {
		return store.AgentSession{}, store.ErrNotFound
	}
	row.Phase, row.Status, row.FailureReason = PhaseCanceled, "canceled", "hibernation retention window elapsed"
	at := row.UpdatedAt.Add(time.Second)
	row.CanceledAt = &at
	row.SnapshotRef, row.SnapshotBytes, row.SnapshotSHA, row.RetainUntil = "", 0, "", nil
	row.UpdatedAt = at
	f.rows[id] = row
	return row, nil
}

func (f *fakeStore) CountPinnedAgentSessions(_ context.Context, workspaceID string) (int, error) {
	n := 0
	for _, row := range f.rows {
		if row.WorkspaceID == workspaceID && row.Pinned {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) ListTerminalAgentSessionsWithSandbox(_ context.Context, since time.Time) ([]store.AgentSession, error) {
	terminal := map[string]bool{PhaseCompleted: true, PhaseFailed: true, PhaseCanceled: true}
	out := []store.AgentSession{}
	for _, row := range f.rows {
		if terminal[row.Phase] && row.SandboxID != "" && !row.UpdatedAt.Before(since) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (f *fakeStore) ClearAgentSessionSandbox(_ context.Context, id string) error {
	if row, ok := f.rows[id]; ok && row.SandboxID != "" {
		row.SandboxID = ""
		f.rows[id] = row
	}
	return nil
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
// returns the session's parts in seq order, capped at maxBytes of payload like
// the real store's bounded read.
func (f *fakeStore) AgentSessionTranscript(_ context.Context, id string, afterSeq int64, maxBytes int64) ([]store.AgentSessionTranscriptPart, error) {
	out := make([]store.AgentSessionTranscriptPart, 0)
	for seq, p := range f.transcript[id] {
		if seq > afterSeq {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	var total int64
	for i, p := range out {
		if total+int64(len(p.Part)) > maxBytes {
			return out[:i], nil
		}
		total += int64(len(p.Part))
	}
	return out, nil
}

func (f *fakeStore) AgentSessionTranscriptBytes(_ context.Context, id string) (int64, error) {
	var total int64
	for _, p := range f.transcript[id] {
		total += int64(len(p.Part))
	}
	return total, nil
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
	// Mirror the store's CAS guard: a session a concurrent Cancel already took
	// terminal matches no row, so a racing background dispatch tears its sandbox
	// back down instead of resurrecting the session (w2/m64).
	if row.Phase == PhaseCanceling || row.Phase == PhaseCanceled {
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
	// Model a developer: holds both can_operate (resume/steer/read) and can_create
	// (session creation, codex #1). Cross-workspace denial still rides the member
	// match below, not the relation, so isolation tests are unaffected.
	granted := relation == core.RelCanOperate || relation == core.RelCanCreate
	return granted && workspace != "" && f.members[subject] == workspace, nil
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
	// Injected background-provisioning failures (w2/m64). createErr aborts the
	// sandbox create (nothing is counted); resumeErr aborts a resume.
	createErr, resumeErr error
	// Hibernation (ADR059 D3, w2/m68). hibernated counts snapshot calls;
	// lastPutURL/lastRestoreURL capture the presigned URLs threaded through;
	// snapshot is the returned result; hibernateErr injects a snapshot failure.
	hibernated     int
	lastPutURL     string
	lastRestoreURL string
	snapshot       sandbox.SnapshotResult
	hibernateErr   error
}

// fakeSnapshots is an in-memory SnapshotStore for the hibernation tests: it
// records refs, hands out deterministic URLs, and tracks deletes.
type fakeSnapshots struct {
	seq       int
	refs      map[string]bool // ref -> present
	deleted   []string
	uploadErr error
	getErr    error
}

func newFakeSnapshots() *fakeSnapshots { return &fakeSnapshots{refs: map[string]bool{}} }

func (f *fakeSnapshots) PrepareUpload(_ context.Context, workspaceID, sessionID string) (string, string, error) {
	if f.uploadErr != nil {
		return "", "", f.uploadErr
	}
	f.seq++
	ref := fmt.Sprintf("agent-snapshots/%s/%s-%d.tgz", workspaceID, sessionID, f.seq)
	f.refs[ref] = true
	return ref, "https://s3.example/put/" + ref, nil
}

func (f *fakeSnapshots) PrepareDownload(_ context.Context, ref string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return "https://s3.example/get/" + ref, nil
}

func (f *fakeSnapshots) Delete(_ context.Context, ref string) error {
	f.deleted = append(f.deleted, ref)
	delete(f.refs, ref)
	return nil
}

func (f *fakeLifecycle) HibernateAgentSessionSandbox(_ context.Context, _, _, _, putURL string) (sandbox.SnapshotResult, error) {
	f.lastPutURL = putURL
	if f.hibernateErr != nil {
		return sandbox.SnapshotResult{}, f.hibernateErr
	}
	f.hibernated++
	res := f.snapshot
	if res.Bytes == 0 {
		res = sandbox.SnapshotResult{Bytes: 4096, SHA256: strings.Repeat("a", 64)}
	}
	return res, nil
}

func (f *fakeLifecycle) CreateAgentSessionSandbox(_ context.Context, _, _, _, repository, branch, modelEndpoint, modelAPIKey string, egressAllowlist []string, driverEnv map[string]string) (sandbox.Sandbox, error) {
	if f.createErr != nil {
		return sandbox.Sandbox{}, f.createErr
	}
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
	if f.resumeErr != nil {
		return f.resumeErr
	}
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
		// Run the background provisioning half synchronously so create/steer/resume
		// side effects (sandbox lifecycle, final phase) are settled by the time the
		// verb returns — deterministic asserts without sleeps (w2/m64).
		dispatchRunner: func(fn func()) { fn() },
	}
	return svc, st, fga, lifecycle
}

func caller(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

// seedLiveSession injects a workspace session already in a live phase holding a
// sandbox id — the state the ADR059 D6 per-workspace cap counts. Used to bring a
// workspace to its cap without driving full dispatch.
func seedLiveSession(st *fakeStore, ws string) string {
	id := ids.New(ids.AgentSession)
	st.rows[id] = store.AgentSession{ID: id, WorkspaceID: ws, Phase: PhaseRunning, SandboxID: "sb-" + id, UpdatedAt: st.now}
	return id
}

// --- ADR059 D6: per-workspace live-sandbox cap ------------------------------

func TestCreateRefusedAtLiveSandboxCap(t *testing.T) {
	svc, st, _, lc := fixture()
	svc.MaxLiveSandboxes = 2
	seedLiveSession(st, "tea-a")
	seedLiveSession(st, "tea-a") // tea-a now at cap

	_, err := svc.Create(caller("alice"), createInput())
	if !isCode(err, "AGENT_SESSION_LIVE_LIMIT") {
		t.Fatalf("create at cap = %v, want AGENT_SESSION_LIVE_LIMIT", err)
	}
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("cap refusal should wrap ErrConflict for a 409, got %v", err)
	}
	if lc.created != 0 {
		t.Fatalf("sandbox provisioned despite cap refusal (created=%d)", lc.created)
	}
	if len(st.rows) != 2 {
		t.Fatalf("a session row was minted despite the pre-provision cap refusal: rows=%d", len(st.rows))
	}
}

func TestCreateUnderLiveSandboxCapAllowed(t *testing.T) {
	svc, st, _, _ := fixture()
	svc.MaxLiveSandboxes = 3
	seedLiveSession(st, "tea-a")
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatalf("create under cap = %v, want success", err)
	}
}

func TestLiveSandboxCapZeroDisabled(t *testing.T) {
	svc, st, _, _ := fixture()
	svc.MaxLiveSandboxes = 0
	for i := 0; i < 5; i++ {
		seedLiveSession(st, "tea-a")
	}
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatalf("create with cap disabled (0) = %v, want success", err)
	}
}

func TestLiveSandboxCapIsPerWorkspace(t *testing.T) {
	svc, st, _, _ := fixture()
	svc.MaxLiveSandboxes = 2
	seedLiveSession(st, "tea-a")
	seedLiveSession(st, "tea-a") // tea-a at cap, tea-b empty

	req := createInput()
	req.OwnerID = "tea-b"
	if _, err := svc.Create(caller("bob"), req); err != nil {
		t.Fatalf("create in an under-cap peer workspace = %v, want success (cap must not leak across workspaces)", err)
	}
}

// A terminal session in its idle grace holds a sandbox_id but is NOT in a live
// phase, so it does not count toward the cap — only actively live turns do.
func TestLiveSandboxCapIgnoresIdleGraceSandboxes(t *testing.T) {
	svc, st, _, _ := fixture()
	svc.MaxLiveSandboxes = 1
	// A completed session still holding its sandbox during the idle grace.
	id := seedLiveSession(st, "tea-a")
	row := st.rows[id]
	row.Phase = PhaseCompleted
	st.rows[id] = row

	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatalf("create with only an idle-grace (terminal) sandbox present = %v, want success", err)
	}
}

func TestSteerRefusedAtLiveSandboxCap(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	svc.MaxLiveSandboxes = 1
	seedLiveSession(st, "tea-a") // tea-a at cap; the steerable session itself is terminal
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "go"}); !isCode(err, "AGENT_SESSION_LIVE_LIMIT") {
		t.Fatalf("steer at cap = %v, want AGENT_SESSION_LIVE_LIMIT", err)
	}
}

func TestResumeRefusedAtLiveSandboxCap(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	svc.MaxLiveSandboxes = 1
	seedLiveSession(st, "tea-a")
	if _, err := svc.Resume(caller("alice"), id); !isCode(err, "AGENT_SESSION_LIVE_LIMIT") {
		t.Fatalf("resume at cap = %v, want AGENT_SESSION_LIVE_LIMIT", err)
	}
}

// --- ADR059 D4/D5: rehydrate + pin -------------------------------------------

// seedHibernated turns the steerable session into a hibernated one holding a
// snapshot, mirroring the state the Completer leaves after reclaiming it.
func seedHibernated(st *fakeStore, id string) {
	row := st.rows[id]
	row.Phase, row.Status = PhaseHibernated, "hibernated"
	row.SandboxID = ""
	row.SnapshotRef, row.SnapshotBytes = "agent-snapshots/tea-a/"+id+".tgz", 4096
	st.rows[id] = row
}

// Resume of a hibernated session rehydrates it: a fresh sandbox is created
// carrying the presigned restore URL, the phase converges to running, and the
// snapshot fields are cleared (consumed into the new pod).
func TestResumeRehydratesHibernatedSession(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	snaps := newFakeSnapshots()
	svc.Snapshots = snaps
	seedHibernated(st, id)

	view, err := svc.Resume(caller("alice"), id)
	if err != nil {
		t.Fatalf("resume hibernated = %v", err)
	}
	_ = view
	row := st.rows[id]
	if row.Phase != PhaseRunning {
		t.Fatalf("rehydrated session phase = %s, want running", row.Phase)
	}
	if row.SnapshotRef != "" {
		t.Fatalf("snapshot ref not cleared after rehydrate: %q", row.SnapshotRef)
	}
	if row.SandboxID == "" {
		t.Fatal("no fresh sandbox adopted on rehydrate")
	}
	if row.DeliveryMode != DeliveryRehydrate {
		t.Fatalf("delivery mode = %q, want rehydrate", row.DeliveryMode)
	}
}

// A hibernated session with no snapshot store wired cannot be rehydrated.
func TestResumeHibernatedWithoutStoreRefused(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	svc.Snapshots = nil
	seedHibernated(st, id)
	if _, err := svc.Resume(caller("alice"), id); !isCode(err, "AGENT_SESSION_NOT_RESUMABLE") {
		t.Fatalf("resume hibernated without store = %v, want AGENT_SESSION_NOT_RESUMABLE", err)
	}
}

// A failed background rehydrate reverts the row to hibernated so the snapshot is
// not lost — the user can retry.
func TestRehydrateFailureRevertsToHibernated(t *testing.T) {
	svc, st, lc, id := steerableFixture(t)
	svc.Snapshots = newFakeSnapshots()
	lc.createErr = errors.New("pod schedule timeout")
	seedHibernated(st, id)

	if _, err := svc.Resume(caller("alice"), id); err != nil {
		t.Fatalf("resume (accept) = %v", err)
	}
	row := st.rows[id]
	if row.Phase != PhaseHibernated {
		t.Fatalf("failed rehydrate left phase=%s, want reverted to hibernated", row.Phase)
	}
	if row.SnapshotRef == "" {
		t.Fatal("snapshot ref lost on failed rehydrate — the workspace would be unrecoverable")
	}
}

// Pinning removes the retention deadline; unpinning puts it back on the clock.
func TestPinUnpinTogglesRetention(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	svc.Snapshots = newFakeSnapshots()
	seedHibernated(st, id)
	deadline := st.now.Add(time.Hour)
	row := st.rows[id]
	row.RetainUntil = &deadline
	st.rows[id] = row

	if _, err := svc.Pin(caller("alice"), id); err != nil {
		t.Fatalf("pin = %v", err)
	}
	if !st.rows[id].Pinned || st.rows[id].RetainUntil != nil {
		t.Fatalf("pin should set pinned + clear deadline: pinned=%v retain=%v", st.rows[id].Pinned, st.rows[id].RetainUntil)
	}
	if _, err := svc.Unpin(caller("alice"), id); err != nil {
		t.Fatalf("unpin = %v", err)
	}
	if st.rows[id].Pinned || st.rows[id].RetainUntil == nil {
		t.Fatalf("unpin should clear pinned + restore a deadline: pinned=%v retain=%v", st.rows[id].Pinned, st.rows[id].RetainUntil)
	}
}

// The pin quota refuses a pin beyond the workspace cap.
func TestPinRefusedAtQuota(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	svc.Snapshots = newFakeSnapshots()
	svc.MaxPinnedSandboxes = 1
	seedHibernated(st, id)
	// A second already-pinned session in the same workspace fills the quota.
	other := "ags-otherpinned000000000"
	st.rows[other] = store.AgentSession{ID: other, WorkspaceID: "tea-a", Phase: PhaseHibernated, Pinned: true, SnapshotRef: "x"}

	if _, err := svc.Pin(caller("alice"), id); !isCode(err, "AGENT_SESSION_PIN_LIMIT") {
		t.Fatalf("pin at quota = %v, want AGENT_SESSION_PIN_LIMIT", err)
	}
}

// Cancel of a hibernated session deletes its durable snapshot immediately.
func TestCancelHibernatedDeletesSnapshot(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	snaps := newFakeSnapshots()
	svc.Snapshots = snaps
	seedHibernated(st, id)
	ref := st.rows[id].SnapshotRef

	if _, err := svc.Cancel(caller("alice"), id); err != nil {
		t.Fatalf("cancel hibernated = %v", err)
	}
	if len(snaps.deleted) != 1 || snaps.deleted[0] != ref {
		t.Fatalf("snapshot not deleted on cancel: %v", snaps.deleted)
	}
	if st.rows[id].Phase != PhaseCanceled {
		t.Fatalf("phase = %s, want canceled", st.rows[id].Phase)
	}
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
	// Create returns FAST (w2/m64): the session is accepted in its creating phase
	// with no sandbox and no ticket yet. The parent tuple, however, must already
	// exist — the row is not a reachable resource without it.
	if created.Phase != PhaseCreating || created.SandboxID != "" || created.Ticket != "" || created.URL != "" {
		t.Fatalf("create should return a fast, ticketless creating view: %+v", created)
	}
	if fga.parents[created.ID] != "tea-a" {
		t.Fatalf("session parent tuple not established: parents=%v", fga.parents)
	}
	// The synchronous test runner has run the background provisioning by now: the
	// sandbox exists, its binding is recorded, and the phase converged.
	if lifecycle.created != 1 || lifecycle.entered != 1 || lifecycle.modelEndpoint != "https://api.openai.com/v1" || !reflect.DeepEqual(lifecycle.egressAllowlist, []string{"docs.example.com"}) {
		t.Fatalf("background dispatch = lifecycle=%+v", lifecycle)
	}
	if lifecycle.repository != "bex-co/example" || lifecycle.branch != "bex-agent/session-test" {
		t.Fatalf("sandbox binding = %q %q", lifecycle.repository, lifecycle.branch)
	}
	provisioned, err := svc.Get(caller("alice"), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if provisioned.SandboxID != "sandbox-1" || provisioned.Phase != PhaseRunning {
		t.Fatalf("provisioned session = %+v", provisioned)
	}

	// The ticket is minted lazily via AttachTicket once a sandbox exists.
	attached, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionRead)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := agentsessionticket.Verify(svc.TicketSecret, attached.Ticket, st.now)
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

	// Resume returns fast in the resuming phase; the sync runner completes the
	// wake so the sandbox is resumed and the phase converges to running.
	resumed, err := svc.Resume(caller("alice"), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Phase != PhaseResuming || resumed.Ticket != "" {
		t.Fatalf("resume should return a fast, ticketless resuming view: %+v", resumed)
	}
	if lifecycle.resumed != 1 {
		t.Fatalf("resume did not run the sandbox: %+v", lifecycle)
	}
	// A fresh attach ticket after resume differs from the earlier one (fresh nonce).
	reattached, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionRead)
	if err != nil {
		t.Fatal(err)
	}
	if reattached.Ticket == "" || reattached.Ticket == attached.Ticket {
		t.Fatalf("attach after resume did not mint a fresh ticket: %+v", reattached)
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

// TestCreateInjectsGitProxyAndTurnGrantConfig pins both sandbox trust
// boundaries: Git uses a Pod-bound proxy (never a raw token helper), and the
// driver gets only the public half of the gateway's turn-grant key.
func TestCreateInjectsGitProxyAndTurnGrantConfig(t *testing.T) {
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
	wantRepoURL, err := agentsession.ProxyRepositoryURL(defaultCredentialURL, env["BEX_SANDBOX_NAMESPACE"], env["BEX_AGENT_SESSION_ID"], env["BEX_AGENT_REPOSITORY"], env["BEX_AGENT_BRANCH"])
	if err != nil {
		t.Fatal(err)
	}
	if env["BEX_AGENT_REPO_URL"] != wantRepoURL {
		t.Fatalf("BEX_AGENT_REPO_URL = %q, want Pod-bound proxy %q", env["BEX_AGENT_REPO_URL"], wantRepoURL)
	}
	if strings.Contains(env["BEX_AGENT_REPO_URL"], "github.com") || env["BEX_AGENT_GRANT_PUBLIC_KEY"] == "" {
		t.Fatalf("driver trust config = %#v", env)
	}
	if env["BEX_AGENT_REPOSITORY"] != "bex-co/example" || env["BEX_AGENT_BRANCH"] != "bex-agent/session-test" {
		t.Fatalf("repo/branch env = %q/%q", env["BEX_AGENT_REPOSITORY"], env["BEX_AGENT_BRANCH"])
	}
}

// codex r7 #3 follow-up — pin AttachTicket's per-action relation split so
// neither branch can silently weaken: reading the transcript is a contributor
// verb (can_operate, the documented ADR054 boundary), while a live prompt
// turn re-drives the agent and stays developer-only (can_create). The
// role-ladder sweep (internal/api) pins only the zero-value read branch.
func TestAttachTicketRelationPerAction(t *testing.T) {
	for action, want := range map[string]string{
		agentsessionticket.ActionRead: core.RelCanOperate,
		"":                            core.RelCanOperate, // unset action defaults to read
		agentsessionticket.ActionTurn: core.RelCanCreate,
	} {
		var asked []string
		svc := &Service{Base: &core.Base{Authz: checkerFunc(func(_ context.Context, _, relation, _ string) (bool, error) {
			asked = append(asked, relation)
			return false, nil // denial keeps the test at the authorization seam
		})}}
		if _, err := svc.AttachTicket(caller("carol"), "ags-x", action); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("action %q err = %v, want ErrForbidden", action, err)
		}
		if len(asked) != 1 || asked[0] != want {
			t.Errorf("action %q asked relations %v, want [%s]", action, asked, want)
		}
	}
}

// TestAttachTicketTurnRequiresLivePhase pins round-5 finding 13: a live-turn
// ticket must be gated on an active phase, not just a retained sandbox. A
// terminal or canceling session keeps its sandbox through the ADR054 editor
// grace window, so a nonempty SandboxID alone would let a developer run
// un-metered, off-lifecycle model turns on a finished session. Read tickets
// (transcript replay) stay available — that is the feature.
func TestAttachTicketTurnRequiresLivePhase(t *testing.T) {
	svc, st, _, _ := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{PhaseCompleted, PhaseFailed, PhaseCanceled, PhaseCanceling} {
		row := st.rows[created.ID]
		row.Phase, row.SandboxID = phase, "sandbox-1"
		st.rows[created.ID] = row

		if _, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionTurn); !isCode(err, "AGENT_SESSION_NOT_LIVE") {
			t.Errorf("phase %q turn ticket = %v, want AGENT_SESSION_NOT_LIVE", phase, err)
		}
		if _, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionRead); err != nil {
			t.Errorf("phase %q read ticket must still succeed, got %v", phase, err)
		}
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

	attached, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionRead)
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
	if _, err := svc.AttachTicket(caller("alice"), pending.ID, agentsessionticket.ActionRead); !isCode(err, "AGENT_SESSION_NOT_ATTACHABLE") {
		t.Fatalf("attach to sandbox-less session = %v, want AGENT_SESSION_NOT_ATTACHABLE", err)
	}

	// Cross-workspace caller is denied by the first-class tuple.
	if _, err := svc.AttachTicket(caller("bob"), created.ID, agentsessionticket.ActionRead); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-workspace attach = %v, want forbidden", err)
	}

	// Gateway unconfigured => 503 on the mint verb, like create/resume/steer.
	svc.GatewayURL = ""
	if _, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionRead); !errors.Is(err, core.ErrAgentSessionsUnavailable) {
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

// TestCreateReturnsFastBeforeProvisioning pins the core w2/m64 contract: Create
// returns the accepted (durable + authorized) session in its creating phase with
// no sandbox and no ticket, WITHOUT having touched the sandbox lifecycle — the
// slow provisioning is deferred to the background.
func TestCreateReturnsFastBeforeProvisioning(t *testing.T) {
	svc, st, fga, lc := fixture()
	var pending []func()
	svc.dispatchRunner = func(fn func()) { pending = append(pending, fn) }

	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if created.Phase != PhaseCreating || created.SandboxID != "" || created.Ticket != "" {
		t.Fatalf("create should return fast in creating with no sandbox/ticket: %+v", created)
	}
	// Durable + authorized already, but the sandbox has NOT been provisioned yet.
	if _, ok := st.rows[created.ID]; !ok || fga.parents[created.ID] != "tea-a" {
		t.Fatalf("session must be persisted + authorized before returning: rows=%v parents=%v", st.rows, fga.parents)
	}
	if lc.created != 0 || lc.entered != 0 {
		t.Fatalf("sandbox lifecycle ran on the request path: %+v", lc)
	}
	if len(pending) != 1 {
		t.Fatalf("provisioning was not deferred to the background: %d pending", len(pending))
	}

	// Running the deferred provisioning brings the sandbox up and converges.
	for _, fn := range pending {
		fn()
	}
	if lc.created != 1 || lc.entered != 1 || st.rows[created.ID].SandboxID != "sandbox-1" {
		t.Fatalf("background provisioning did not converge: lc=%+v row=%+v", lc, st.rows[created.ID])
	}
}

// TestCreateCancelRaceConverges is the resource-safety guarantee: a Cancel that
// lands while provisioning is still in flight must not leave an orphaned sandbox
// and must not be resurrected to running by the racing dispatch (w2/m64).
func TestCreateCancelRaceConverges(t *testing.T) {
	svc, st, _, lc := fixture()
	var pending []func()
	svc.dispatchRunner = func(fn func()) { pending = append(pending, fn) }

	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	// Cancel lands BEFORE the background dispatch records a sandbox: the row has no
	// sandbox yet, so there is nothing to tear down at cancel time.
	canceled, err := svc.Cancel(caller("alice"), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if canceled.Phase != PhaseCanceled || lc.canceled != 0 {
		t.Fatalf("cancel of a not-yet-provisioned session = %+v canceled=%d", canceled, lc.canceled)
	}
	// Now the deferred provisioning runs. It creates a sandbox, then the CAS-guarded
	// record finds the session canceled — so the sandbox is torn back down and the
	// session is NOT resurrected.
	for _, fn := range pending {
		fn()
	}
	if lc.created != 1 || lc.canceled != 1 {
		t.Fatalf("racing dispatch must create then tear down its sandbox: created=%d canceled=%d", lc.created, lc.canceled)
	}
	final := st.rows[created.ID]
	if final.Phase != PhaseCanceled {
		t.Fatalf("session resurrected by a racing dispatch: phase=%q", final.Phase)
	}
	if final.SandboxID != "" {
		t.Fatalf("canceled session adopted an orphaned sandbox id %q", final.SandboxID)
	}
}

// TestBackgroundDispatchFailureSurfacesFailedPhase proves a provisioning failure
// is not a hung create: the session lands in failed with a named reason (w2/m64),
// which is what the dashboard renders as the failure callout + retry.
func TestBackgroundDispatchFailureSurfacesFailedPhase(t *testing.T) {
	svc, st, _, lc := fixture() // synchronous runner
	lc.createErr = errors.New("pod schedule timeout")

	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err) // the create itself still succeeds fast
	}
	failed := st.rows[created.ID]
	if failed.Phase != PhaseFailed || failed.Status != "sandbox create failed" {
		t.Fatalf("dispatch failure should surface a failed phase + reason: %+v", failed)
	}
	if lc.created != 0 {
		t.Fatalf("no sandbox should have been recorded on a create failure: %+v", lc)
	}
}

// TestBackgroundDispatchFailureDoesNotClobberCancel proves setLifecycleIfActive
// preserves a user's cancel: a provisioning failure that lands after the session
// was already canceled must not flip canceled → failed.
func TestBackgroundDispatchFailureDoesNotClobberCancel(t *testing.T) {
	svc, st, _, lc := fixture()
	lc.createErr = errors.New("pod schedule timeout")
	var pending []func()
	svc.dispatchRunner = func(fn func()) { pending = append(pending, fn) }

	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Cancel(caller("alice"), created.ID); err != nil {
		t.Fatal(err)
	}
	for _, fn := range pending {
		fn() // the deferred provisioning fails now, after the cancel
	}
	if got := st.rows[created.ID].Phase; got != PhaseCanceled {
		t.Fatalf("a failing dispatch clobbered the user's cancel: phase=%q", got)
	}
}

// TestResumeReturnsFastAndSurfacesFailure pins the async Resume: it returns fast
// in resuming, and a background wake failure surfaces as failed with a reason.
func TestResumeReturnsFastAndSurfacesFailure(t *testing.T) {
	svc, st, _, lc := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	// Make it idle-but-resumable, then arm a resume failure.
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	st.rows[created.ID] = row
	lc.resumeErr = errors.New("snapshot restore failed")

	resumed, err := svc.Resume(caller("alice"), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Phase != PhaseResuming || resumed.Ticket != "" {
		t.Fatalf("resume should return fast in resuming with no ticket: %+v", resumed)
	}
	if got := st.rows[created.ID]; got.Phase != PhaseFailed || got.Status != "sandbox resume failed" {
		t.Fatalf("resume failure should surface failed + reason: %+v", got)
	}
}

// TestSteerRejectedWhileRedispatchInFlight proves the in-flight guard holds
// across the async boundary: once a steer has flipped the session to
// redispatching, a second steer is rejected until the background turn settles.
func TestSteerRejectedWhileRedispatchInFlight(t *testing.T) {
	svc, st, _, _ := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	st.rows[created.ID] = row

	// Defer provisioning so the session stays in redispatching between the two steers.
	var pending []func()
	svc.dispatchRunner = func(fn func()) { pending = append(pending, fn) }

	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "one"}); err != nil {
		t.Fatal(err)
	}
	if st.rows[created.ID].Phase != PhaseRedispatching {
		t.Fatalf("first steer did not flip to redispatching: %q", st.rows[created.ID].Phase)
	}
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: created.ID, Prompt: "two"}); !isCode(err, "AGENT_SESSION_TURN_IN_FLIGHT") {
		t.Fatalf("second steer while redispatching = %v, want AGENT_SESSION_TURN_IN_FLIGHT", err)
	}
	_ = pending
}

// TestAttachTicketNotReadyDuringProvisioning is the lazy-ticket contract: while a
// session is still provisioning (creating, no sandbox), AttachTicket returns the
// named, retryable NOT_ATTACHABLE — then succeeds once the sandbox is up.
func TestAttachTicketNotReadyDuringProvisioning(t *testing.T) {
	svc, _, _, _ := fixture()
	var pending []func()
	svc.dispatchRunner = func(fn func()) { pending = append(pending, fn) }

	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionRead); !isCode(err, "AGENT_SESSION_NOT_ATTACHABLE") {
		t.Fatalf("attach during provisioning = %v, want AGENT_SESSION_NOT_ATTACHABLE", err)
	}
	for _, fn := range pending {
		fn()
	}
	attached, err := svc.AttachTicket(caller("alice"), created.ID, agentsessionticket.ActionRead)
	if err != nil || attached.Ticket == "" {
		t.Fatalf("attach after provisioning = %+v err=%v, want a ticket", attached, err)
	}
}

func TestRESTGraphQLMCPCreateParity(t *testing.T) {
	want := createInput()
	check := func(t *testing.T, got View) {
		t.Helper()
		// Create returns fast: the creating phase with no sandbox/ticket yet
		// (w2/m64) — identically across REST, GraphQL, and MCP.
		if got.OwnerID != want.OwnerID || got.Repo != want.Repo || got.Branch != want.Branch || got.AgentConfig != want.AgentConfig || got.Phase != PhaseCreating || got.SandboxID != "" || got.Ticket != "" || got.URL != "" {
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

// The evidence digest lost its renderers in w5/m65 but deliberately kept its
// wire field, so API consumers still receive it.
func TestEvidenceRemainsOnTheWire(t *testing.T) {
	view, err := viewOf(store.AgentSession{
		AgentConfig: []byte(`{"agent":"codex","task":"t"}`),
		Evidence:    []byte(`{"commandLog":["go test ./..."],"commits":2}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Evidence == nil || len(view.Evidence.CommandLog) != 1 || view.Evidence.Commits != 2 {
		t.Fatalf("evidence dropped from the wire view: %+v", view.Evidence)
	}
}
