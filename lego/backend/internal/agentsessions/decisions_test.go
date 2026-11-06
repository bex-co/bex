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
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeDecisions is the ADR100 decision table over a fakeStore: it keeps the
// session mirror (pending_decision_*) in step and pokes the state listeners,
// like the PG rows and NOTIFY trigger.
type fakeDecisions struct {
	mu   sync.Mutex
	st   *fakeStore
	rows map[string]store.AgentDecision
	now  func() time.Time
	// expireErr fails every sweep; expireCalls counts them.
	expireErr   error
	expireCalls int
}

func newFakeDecisions(st *fakeStore) *fakeDecisions {
	return &fakeDecisions{st: st, rows: map[string]store.AgentDecision{}, now: time.Now}
}

// mirror recomputes the session's mirror from its open decisions, like
// store.refreshPendingDecisions: the oldest open one, or none.
func (f *fakeDecisions) mirror(sessionID string) {
	f.mu.Lock()
	var oldest *store.AgentDecision
	for _, d := range f.rows {
		if d.SessionID != sessionID || d.State != store.AgentDecisionPending {
			continue
		}
		if oldest == nil || d.CreatedAt.Before(oldest.CreatedAt) || (d.CreatedAt.Equal(oldest.CreatedAt) && d.ID < oldest.ID) {
			d := d
			oldest = &d
		}
	}
	f.mu.Unlock()
	f.st.rowMu.Lock()
	row := f.st.rows[sessionID]
	row.PendingDecisionID, row.PendingDecisionKind = "", ""
	if oldest != nil {
		row.PendingDecisionID, row.PendingDecisionKind = oldest.ID, oldest.Kind
	}
	f.st.rows[sessionID] = row
	f.st.rowMu.Unlock()
	f.st.notifyState(sessionID)
}

func (f *fakeDecisions) OpenAgentDecision(_ context.Context, in store.AgentDecisionInput) (store.AgentDecision, bool, error) {
	f.mu.Lock()
	for _, d := range f.rows {
		if d.SessionID == in.SessionID && d.RequestID == in.RequestID {
			f.mu.Unlock()
			return d, false, nil
		}
	}
	f.st.rowMu.Lock()
	session, ok := f.st.rows[in.SessionID]
	f.st.rowMu.Unlock()
	if !ok || session.WorkspaceID != in.WorkspaceID {
		f.mu.Unlock()
		return store.AgentDecision{}, false, store.ErrNotFound
	}
	if session.Phase != PhaseRunning || session.Turns != in.Turn {
		f.mu.Unlock()
		return store.AgentDecision{}, false, store.ErrAgentDecisionNotOpen
	}
	d := store.AgentDecision{ID: ids.New(ids.AgentDecision), SessionID: in.SessionID, WorkspaceID: in.WorkspaceID, Turn: in.Turn,
		RequestID: in.RequestID, Kind: in.Kind, Title: in.Title, Detail: in.Detail, State: store.AgentDecisionPending,
		CreatedAt: f.now(), ExpiresAt: f.now().Add(in.TTL)}
	f.rows[d.ID] = d
	f.mu.Unlock()
	f.mirror(in.SessionID)
	return d, true, nil
}

func (f *fakeDecisions) GetAgentDecision(_ context.Context, id string) (store.AgentDecision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.rows[id]
	if !ok {
		return store.AgentDecision{}, store.ErrNotFound
	}
	return d, nil
}

func (f *fakeDecisions) ResolveAgentDecision(_ context.Context, sessionID, id, state string, answer json.RawMessage, by string) (store.AgentDecision, error) {
	f.mu.Lock()
	d, ok := f.rows[id]
	switch {
	case !ok || d.SessionID != sessionID:
		f.mu.Unlock()
		return store.AgentDecision{}, store.ErrNotFound
	case d.State == store.AgentDecisionExpired || (d.State == store.AgentDecisionPending && !d.ExpiresAt.After(f.now())):
		f.mu.Unlock()
		return store.AgentDecision{}, store.ErrAgentDecisionExpired
	case d.State != store.AgentDecisionPending:
		f.mu.Unlock()
		return store.AgentDecision{}, store.ErrAgentDecisionAnswered
	}
	at := f.now()
	d.State, d.Answer, d.AnsweredBy, d.AnsweredAt = state, answer, by, &at
	f.rows[id] = d
	f.mu.Unlock()
	f.mirror(sessionID)
	return d, nil
}

func (f *fakeDecisions) ExpireAgentDecisions(_ context.Context, limit int) ([]store.AgentDecision, error) {
	f.mu.Lock()
	f.expireCalls++
	if f.expireErr != nil {
		f.mu.Unlock()
		return nil, f.expireErr
	}
	var out []store.AgentDecision
	for id, d := range f.rows {
		if len(out) == limit {
			break
		}
		if d.State == store.AgentDecisionPending && !d.ExpiresAt.After(f.now()) {
			d.State = store.AgentDecisionExpired
			d.Answer = json.RawMessage(`{"allow":false}`)
			if d.Kind == store.AgentDecisionQuestion {
				d.Answer = json.RawMessage(`{"text":""}`)
			}
			f.rows[id] = d
			out = append(out, d)
		}
	}
	f.mu.Unlock()
	for _, d := range out {
		f.mirror(d.SessionID)
	}
	return out, nil
}

// decisionSession is a running session on sandbox sbx-live in tea-a, with the
// decision store wired.
func decisionSession(t *testing.T) (*Service, *fakeStore, *fakeDecisions, string) {
	t.Helper()
	svc, st, _, _ := fixture()
	decisions := newFakeDecisions(st)
	svc.Decisions = decisions
	svc.DecisionAwaitHold = 200 * time.Millisecond
	view, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "deploy it"}, ApprovalMode: ApprovalRisky})
	if err != nil {
		t.Fatal(err)
	}
	setSession(st, view.ID, func(r *store.AgentSession) { r.SandboxID, r.Turns, r.Phase = "sbx-live", 1, PhaseRunning })
	return svc, st, decisions, view.ID
}

func decisionEnvelope(sessionID string, content agentsession.DecisionContent) agentsession.DecisionEnvelope {
	return agentsession.DecisionEnvelope{Kind: agentsession.DecisionKind, Op: agentsession.DecisionOpOpen, Workspace: "tea-a",
		SessionID: sessionID, PodName: "sbx-live-0", PodUID: "uid-live", Content: content}
}

func approvalContent(requestID string) agentsession.DecisionContent {
	return agentsession.DecisionContent{RequestID: requestID, Kind: agentsession.DecisionApproval,
		Title: "Run: git push --force origin main", Detail: json.RawMessage(`{"toolKind":"execute","command":"git push --force origin main","rule":"git-force-push"}`)}
}

func TestDecisionPausesTheSessionAndAnAnswerResumesIt(t *testing.T) {
	svc, st, _, id := decisionSession(t)
	ctx := context.Background()
	opened, err := svc.OpenDecision(ctx, decisionEnvelope(id, approvalContent("perm-1")))
	if err != nil || opened.State != store.AgentDecisionPending {
		t.Fatalf("open = %+v, %v", opened, err)
	}
	// A retried open with the same request id is the same decision.
	again, err := svc.OpenDecision(ctx, decisionEnvelope(id, approvalContent("perm-1")))
	if err != nil || again.ID != opened.ID {
		t.Fatalf("retried open = %+v, %v (want %s)", again, err, opened.ID)
	}

	view, err := svc.Get(caller("alice"), id)
	if err != nil {
		t.Fatal(err)
	}
	if view.State != StateNeedsInput || view.WaitingFor != WaitingForApproval || view.Steerable {
		t.Fatalf("paused view state=%s waitingFor=%s steerable=%v", view.State, view.WaitingFor, view.Steerable)
	}
	if view.Outcome == nil || view.Outcome.PendingDecision == nil || view.Outcome.PendingDecision.ID != opened.ID || view.Outcome.State != StateNeedsInput {
		t.Fatalf("outcome = %+v", view.Outcome)
	}
	// The list filter sees the same state.
	listed, err := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a", States: []string{StateNeedsInput}})
	if err != nil || len(listed) != 1 || listed[0].ID != id {
		t.Fatalf("needs_input list = %+v, %v", listed, err)
	}
	if working, _ := svc.List(caller("alice"), ListRequest{OwnerID: "tea-a", States: []string{StateWorking}}); len(working) != 0 {
		t.Fatalf("a paused session still lists as working: %+v", working)
	}
	// wait settles on needs_input.
	waited, err := svc.Wait(caller("alice"), WaitRequest{IDs: []string{id}, TimeoutSeconds: 1}, nil)
	if err != nil || waited.TimedOut || len(waited.Settled) != 1 || waited.Sessions[0].Outcome.PendingDecision == nil {
		t.Fatalf("wait = %+v, %v", waited, err)
	}

	// The sandbox's await returns the answer once a member gives it.
	answered := make(chan agentsession.DecisionResponse, 1)
	go func() {
		env := decisionEnvelope(id, agentsession.DecisionContent{})
		env.Op, env.DecisionID = agentsession.DecisionOpAwait, opened.ID
		for {
			out, err := svc.AwaitDecision(ctx, env)
			if err != nil {
				t.Error(err)
				close(answered)
				return
			}
			if out.State != store.AgentDecisionPending {
				answered <- out
				return
			}
		}
	}()
	allow := true
	decision, err := svc.AnswerDecision(caller("alice"), AnswerDecisionRequest{SessionID: id, DecisionID: opened.ID, Allow: &allow, Scope: "session"})
	if err != nil || decision.State != store.AgentDecisionAllowed {
		t.Fatalf("answer = %+v, %v", decision, err)
	}
	select {
	case out := <-answered:
		if out.State != store.AgentDecisionAllowed || string(out.Answer) != `{"allow":true,"scope":"session"}` {
			t.Fatalf("await = %+v (%s)", out, out.Answer)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the await never saw the answer")
	}
	view, _ = svc.Get(caller("alice"), id)
	if view.State != StateWorking || view.WaitingFor != "" || st.rows[id].PendingDecisionID != "" {
		t.Fatalf("after the answer state=%s waitingFor=%s", view.State, view.WaitingFor)
	}

	// A decision has one answer.
	deny := false
	if _, err := svc.AnswerDecision(caller("alice"), AnswerDecisionRequest{SessionID: id, DecisionID: opened.ID, Allow: &deny}); codeOf(err) != "AGENT_SESSION_DECISION_ANSWERED" {
		t.Fatalf("second answer = %v", err)
	}
}

// An engine may ask several things at once (parallel tool calls): every
// decision stays open until it is answered, the session shows the oldest,
// and it is needs_input until the last is answered.
func TestParallelDecisionsStayOpenUntilEachIsAnswered(t *testing.T) {
	svc, st, decisions, id := decisionSession(t)
	ctx := context.Background()
	first, err := svc.OpenDecision(ctx, decisionEnvelope(id, approvalContent("perm-1")))
	if err != nil {
		t.Fatal(err)
	}
	decisions.now = func() time.Time { return time.Now().Add(time.Second) }
	second, err := svc.OpenDecision(ctx, decisionEnvelope(id, agentsession.DecisionContent{RequestID: "q-1", Kind: agentsession.DecisionQuestion, Title: "eu or us?"}))
	if err != nil || second.ID == first.ID {
		t.Fatalf("second open = %+v, %v", second, err)
	}
	if d, _ := decisions.GetAgentDecision(ctx, first.ID); d.State != store.AgentDecisionPending {
		t.Fatalf("opening a second decision changed the first: %s", d.State)
	}
	view, _ := svc.Get(caller("alice"), id)
	if view.State != StateNeedsInput || view.WaitingFor != WaitingForApproval || view.Outcome.PendingDecision.ID != first.ID {
		t.Fatalf("with two open: state=%s waitingFor=%s pending=%+v", view.State, view.WaitingFor, view.Outcome.PendingDecision)
	}
	answer := "eu"
	if _, err := svc.AnswerDecision(caller("alice"), AnswerDecisionRequest{SessionID: id, DecisionID: second.ID, Answer: &answer}); err != nil {
		t.Fatal(err)
	}
	if view, _ = svc.Get(caller("alice"), id); view.State != StateNeedsInput || st.rows[id].PendingDecisionID != first.ID {
		t.Fatalf("after the newer answer: state=%s pending=%s", view.State, st.rows[id].PendingDecisionID)
	}
	allow := true
	if _, err := svc.AnswerDecision(caller("alice"), AnswerDecisionRequest{SessionID: id, DecisionID: first.ID, Allow: &allow}); err != nil {
		t.Fatal(err)
	}
	if view, _ = svc.Get(caller("alice"), id); view.State != StateWorking || view.WaitingFor != "" {
		t.Fatalf("after both answers: state=%s waitingFor=%s", view.State, view.WaitingFor)
	}
}

// An await on an expired decision whose sweep fails sweeps once and then
// waits out its hold, rather than re-reading in a tight loop.
func TestAwaitOnAnExpiredDecisionDoesNotSpin(t *testing.T) {
	svc, _, decisions, id := decisionSession(t)
	ctx := context.Background()
	opened, err := svc.OpenDecision(ctx, decisionEnvelope(id, approvalContent("perm-1")))
	if err != nil {
		t.Fatal(err)
	}
	decisions.now = func() time.Time { return time.Now().Add(48 * time.Hour) }
	decisions.mu.Lock()
	d := decisions.rows[opened.ID]
	d.ExpiresAt = time.Now().Add(-time.Minute)
	decisions.rows[opened.ID] = d
	decisions.expireErr = errors.New("sweep failed")
	decisions.mu.Unlock()
	svc.DecisionAwaitPoll = time.Hour
	env := decisionEnvelope(id, agentsession.DecisionContent{})
	env.Op, env.DecisionID = agentsession.DecisionOpAwait, opened.ID
	start := time.Now()
	out, err := svc.AwaitDecision(ctx, env)
	if err != nil || out.State != store.AgentDecisionPending {
		t.Fatalf("await = %+v, %v", out, err)
	}
	if held := time.Since(start); held < 150*time.Millisecond {
		t.Fatalf("the await returned after %s instead of holding", held)
	}
	decisions.mu.Lock()
	calls := decisions.expireCalls
	decisions.mu.Unlock()
	if calls != 1 {
		t.Fatalf("the await swept %d times, want 1", calls)
	}
}

func TestDecisionAnswerShapesAndRefusals(t *testing.T) {
	svc, _, decisions, id := decisionSession(t)
	ctx := context.Background()
	question, err := svc.OpenDecision(ctx, decisionEnvelope(id, agentsession.DecisionContent{RequestID: "ask-1", Kind: agentsession.DecisionQuestion, Title: "Which region?",
		Detail: json.RawMessage(`{"question":"Deploy to eu or us?","options":["eu","us"]}`)}))
	if err != nil {
		t.Fatal(err)
	}
	if view, _ := svc.Get(caller("alice"), id); view.WaitingFor != WaitingForUser {
		t.Fatalf("question waitingFor = %q", view.WaitingFor)
	}
	allow := true
	text := "  eu  "
	for _, c := range []struct {
		name string
		req  AnswerDecisionRequest
		code string
	}{
		{"allow on a question", AnswerDecisionRequest{Allow: &allow}, "AGENT_SESSION_DECISION_INVALID"},
		{"empty answer", AnswerDecisionRequest{Answer: new(string)}, "AGENT_SESSION_DECISION_INVALID"},
		{"bad decision id", AnswerDecisionRequest{DecisionID: "att-x"}, "AGENT_SESSION_DECISION_INVALID"},
		{"unknown decision", AnswerDecisionRequest{DecisionID: ids.New(ids.AgentDecision), Answer: &text}, "AGENT_SESSION_DECISION_NOT_FOUND"},
	} {
		req := c.req
		req.SessionID = id
		if req.DecisionID == "" {
			req.DecisionID = question.ID
		}
		if _, err := svc.AnswerDecision(caller("alice"), req); codeOf(err) != c.code {
			t.Errorf("%s: %v, want %s", c.name, err, c.code)
		}
	}
	// Another workspace's member cannot answer (the session's own can_create).
	if _, err := svc.AnswerDecision(caller("bob"), AnswerDecisionRequest{SessionID: id, DecisionID: question.ID, Answer: &text}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-workspace answer = %v", err)
	}
	got, err := svc.AnswerDecision(caller("alice"), AnswerDecisionRequest{SessionID: id, DecisionID: question.ID, Answer: &text})
	if err != nil || got.State != store.AgentDecisionAnswered || got.Answer.(map[string]any)["text"] != "eu" {
		t.Fatalf("answer = %+v, %v", got, err)
	}

	// An unanswered decision expires: an approval as denied, and a late answer is refused.
	late, err := svc.OpenDecision(ctx, decisionEnvelope(id, approvalContent("perm-2")))
	if err != nil {
		t.Fatal(err)
	}
	decisions.now = func() time.Time { return time.Now().Add(decisionTTL + time.Minute) }
	svc.expireDecisions(ctx)
	if d, _ := decisions.GetAgentDecision(ctx, late.ID); d.State != store.AgentDecisionExpired || string(d.Answer) != `{"allow":false}` {
		t.Fatalf("expired = %+v (%s)", d, d.Answer)
	}
	if _, err := svc.AnswerDecision(caller("alice"), AnswerDecisionRequest{SessionID: id, DecisionID: late.ID, Allow: &allow}); codeOf(err) != "AGENT_SESSION_DECISION_EXPIRED" {
		t.Fatalf("late answer = %v", err)
	}
}

func TestDecisionVerbsHoldTheSandboxToTheSession(t *testing.T) {
	svc, st, _, id := decisionSession(t)
	ctx := context.Background()
	stranger := decisionEnvelope(id, approvalContent("perm-1"))
	stranger.PodName = "sbx-other-0"
	if _, err := svc.OpenDecision(ctx, stranger); resultCode(err) != agentsession.ResultForbidden {
		t.Fatalf("another Pod opened a decision: %v", err)
	}
	opened, err := svc.OpenDecision(ctx, decisionEnvelope(id, approvalContent("perm-1")))
	if err != nil {
		t.Fatal(err)
	}
	await := decisionEnvelope(id, agentsession.DecisionContent{})
	await.Op, await.DecisionID = agentsession.DecisionOpAwait, ids.New(ids.AgentDecision)
	if _, err := svc.AwaitDecision(ctx, await); resultCode(err) != agentsession.DecisionNotFound {
		t.Fatalf("await of an unknown decision = %v", err)
	}
	// The turn ended: a new open is refused and the pending await times out pending.
	setSession(st, id, func(r *store.AgentSession) {
		r.Phase, r.PendingDecisionID, r.PendingDecisionKind = PhaseCompleted, "", ""
	})
	if _, err := svc.OpenDecision(ctx, decisionEnvelope(id, approvalContent("perm-9"))); resultCode(err) != agentsession.ResultForbidden {
		t.Fatalf("open after the turn ended = %v", err)
	}
	await.DecisionID = opened.ID
	if _, err := svc.AwaitDecision(ctx, await); resultCode(err) != agentsession.ResultForbidden {
		t.Fatalf("await after the turn ended = %v", err)
	}
	// Without the decision store the verbs are unavailable.
	svc.Decisions = nil
	if _, err := svc.OpenDecision(ctx, decisionEnvelope(id, approvalContent("perm-3"))); resultCode(err) != agentsession.ResultUnavailable {
		t.Fatalf("open without a store = %v", err)
	}
}

func TestSessionApprovalModeFollowsTheWorkspacePolicy(t *testing.T) {
	svc, st, _, _ := fixture()
	envs := newFakeEnvironments()
	svc.Environments = envs
	svc.Decisions = newFakeDecisions(st)
	set := func(mode string, bypass bool) {
		envs.rows["tea-a"] = store.AgentEnvironment{WorkspaceID: "tea-a", NetworkTier: NetworkTierRegistries, ApprovalMode: mode, ApprovalBypassAllowed: bypass}
	}
	mode := func(requested string, bypass bool) (string, error) {
		view, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}, ApprovalMode: requested, BypassApproval: bypass})
		return view.ApprovalMode, err
	}
	set(ApprovalRisky, true)
	if got, err := mode("", false); err != nil || got != ApprovalRisky {
		t.Fatalf("default = %q, %v; want the workspace's risky", got, err)
	}
	if got, err := mode(ApprovalAll, false); err != nil || got != ApprovalAll {
		t.Fatalf("raised = %q, %v", got, err)
	}
	if got, err := mode("", true); err != nil || got != ApprovalAutonomous {
		t.Fatalf("bypass = %q, %v", got, err)
	}
	set(ApprovalRisky, false)
	if _, err := mode("", true); codeOf(err) != "AGENT_SESSION_APPROVAL_BYPASS_FORBIDDEN" {
		t.Fatalf("bypass on a strict workspace = %v", err)
	}
	if _, err := mode(ApprovalAutonomous, false); codeOf(err) != "AGENT_SESSION_APPROVAL_BYPASS_FORBIDDEN" {
		t.Fatalf("lowering on a strict workspace = %v", err)
	}
	if _, err := mode("sometimes", false); codeOf(err) != "AGENT_SESSION_INPUT_INVALID" {
		t.Fatalf("unknown mode = %v", err)
	}
	if _, err := mode(ApprovalAll, true); codeOf(err) != "AGENT_SESSION_INPUT_INVALID" {
		t.Fatalf("bypass with a stricter mode = %v", err)
	}
	// Without the decision store nothing can pause, so asking to is refused.
	svc.Decisions = nil
	if _, err := mode(ApprovalRisky, false); codeOf(err) != "AGENT_SESSION_APPROVAL_UNAVAILABLE" {
		t.Fatalf("risky without a store = %v", err)
	}
	if got, err := mode("", false); err != nil || got != ApprovalAutonomous {
		t.Fatalf("default without a store = %q, %v", got, err)
	}
}

func TestDriverEnvCarriesTheDecisionContractOnlyWhenTheSessionCanPause(t *testing.T) {
	svc, st, _, _ := fixture()
	svc.Decisions = newFakeDecisions(st)
	record := store.AgentSession{ID: ids.New(ids.AgentSession), WorkspaceID: "tea-a", ApprovalMode: ApprovalAutonomous}
	env := svc.driverEnv(AgentConfig{Agent: "claude"}, record, false)
	if _, ok := env["BEX_AGENT_DECISION_URL"]; ok {
		t.Fatalf("an autonomous session got a decision URL: %v", env)
	}
	record.ApprovalMode = ApprovalRisky
	env = svc.driverEnv(AgentConfig{Agent: "claude"}, record, false)
	if env["BEX_AGENT_APPROVAL_MODE"] != ApprovalRisky || env["BEX_AGENT_DECISION_URL"] == "" {
		t.Fatalf("risky env = %v", env)
	}
	ns, session, decisionID, err := agentsession.ParseDecisionPath(env["BEX_AGENT_DECISION_URL"][len(svc.gitProxyURL()):])
	if err != nil || ns != store.SandboxNamespace("tea-a") || session != record.ID || decisionID != "" {
		t.Fatalf("decision URL %q → %q %q %q %v", env["BEX_AGENT_DECISION_URL"], ns, session, decisionID, err)
	}
}

func TestApprovalModeIsAnEnvironmentSetting(t *testing.T) {
	svc := environmentService("tea-a", newFakeEnvironments())
	got, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ApprovalMode: ptr(ApprovalAll), ApprovalBypassAllowed: ptr(false)})
	if err != nil || got.ApprovalMode != ApprovalAll || got.ApprovalBypassAllowed {
		t.Fatalf("update = %+v, %v", got, err)
	}
	if _, err := svc.UpdateEnvironment(caller("admin"), "tea-a", EnvironmentInput{ApprovalMode: ptr("never")}); codeOf(err) != "AGENT_ENVIRONMENT_APPROVAL_MODE_INVALID" {
		t.Fatalf("invalid mode = %v", err)
	}
	if _, err := svc.UpdateEnvironment(caller("dev"), "tea-a", EnvironmentInput{ApprovalMode: ptr(ApprovalAutonomous)}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a developer changed the approval policy: %v", err)
	}
}
