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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakePR struct {
	calls int
	last  struct{ owner, repo, head, base, title, body string }
	pr    github.PullRequest
	err   error
}

func (f *fakePR) OpenDraftPullRequest(_ context.Context, _ int64, owner, repo, head, base, title, body string) (github.PullRequest, error) {
	f.calls++
	f.last.owner, f.last.repo, f.last.head, f.last.base, f.last.title, f.last.body = owner, repo, head, base, title, body
	if f.err != nil {
		return github.PullRequest{}, f.err
	}
	return f.pr, nil
}

type fakeConn struct {
	conn store.GitConnection
	err  error
}

func (f fakeConn) GetGitConnection(context.Context, string) (store.GitConnection, error) {
	return f.conn, f.err
}

// completerFixture builds a Completer over a fake store holding one running
// session with a live sandbox id, ready for a status-driven finalize.
func completerFixture(status string, statusErr error) (*Completer, *fakeStore, *fakeLifecycle, *fakePR, string) {
	st := newFakeStore()
	config, _ := json.Marshal(AgentConfig{Agent: "codex", Task: "fix the tests"})
	row, _ := st.CreateAgentSession(context.Background(), store.AgentSession{WorkspaceID: "tea-a", Repo: "bex-co/example", Branch: "bex-agent/s1", AgentConfig: config})
	row, _ = st.RecordAgentSessionDispatch(context.Background(), row.ID, "sandbox-1", PhaseRunning, "running", "")
	lc := &fakeLifecycle{status: status, statusErr: statusErr}
	pr := &fakePR{pr: github.PullRequest{Number: 7, HTMLURL: "https://github.com/bex-co/example/pull/7", State: "open", Draft: true}}
	c := &Completer{Store: st, Sandbox: lc, GitHub: pr, Connections: fakeConn{conn: store.GitConnection{InstallationID: 42, AccountLogin: "bex-co"}}, APIPublicURL: "https://api.bex.co"}
	return c, st, lc, pr, row.ID
}

func succeededStatus(pushed bool) string {
	d := statusDelivery{Branch: "bex-agent/s1", BaseBranch: "main", HeadSHA: "abc123", Pushed: pushed, Commits: 1, ChangedFiles: []string{"fix.txt"}}
	if !pushed {
		d = statusDelivery{Branch: "bex-agent/s1", BaseBranch: "main"}
	}
	raw, _ := json.Marshal(statusReport{State: "succeeded", Delivery: d, Evidence: statusEvidence{CommandLog: []string{"go test ./..."}, TestOutput: []string{"ok pkg"}, OutputTail: "done"}})
	return string(raw)
}

func TestCompleterOpensDraftPRAndRecordsEvidence(t *testing.T) {
	c, st, lc, pr, id := completerFixture(succeededStatus(true), nil)
	c.Reconcile(context.Background())

	if pr.calls != 1 || pr.last.head != "bex-agent/s1" || pr.last.base != "main" || pr.last.owner != "bex-co" || pr.last.repo != "example" {
		t.Fatalf("PR open = %+v (calls=%d)", pr.last, pr.calls)
	}
	// The body is session metadata only (w5/m65) — the diff and commit list are
	// what GitHub already renders, and the "test output" the digest used to inline
	// was any tool call that happened to write stdout.
	if !strings.Contains(pr.last.body, id) || !strings.Contains(pr.last.body, "bex-agent/s1") || !strings.Contains(pr.last.body, "abc123") {
		t.Fatalf("PR body missing session metadata: %s", pr.last.body)
	}
	for _, banned := range []string{"go test ./...", "fix.txt", "Changed files", "Commands run", "Test output", "truncated"} {
		if strings.Contains(pr.last.body, banned) {
			t.Fatalf("PR body still carries the evidence digest (%q): %s", banned, pr.last.body)
		}
	}
	row := st.rows[id]
	if row.Phase != PhaseCompleted || row.HeadSHA != "abc123" || row.PRURL == "" || row.PRNumber != 7 {
		t.Fatalf("completed row = %+v", row)
	}
	var ev Evidence
	if err := json.Unmarshal(row.Evidence, &ev); err != nil || len(ev.CommandLog) == 0 || len(ev.ChangedFiles) == 0 {
		t.Fatalf("evidence = %s err=%v", row.Evidence, err)
	}
	if lc.canceled != 1 {
		t.Fatalf("sandbox not torn down: canceled=%d", lc.canceled)
	}
}

// The ADR051 fix: the Completer harvests the driver's log and persists the
// transcript BEFORE tearing the sandbox down (the log dies with the pod), scoped
// to the session's current turn.
func TestCompleterHarvestsTranscriptBeforeTeardown(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	lc.transcriptLog = `{"at":"t","type":"ui-message","part":{"type":"text","text":"hello"}}
{"at":"t","type":"ui-message","part":{"type":"data-acp","data":{"type":"plan"}}}`
	c.Reconcile(context.Background())

	if lc.readTranscript != 1 {
		t.Fatalf("transcript not harvested: readTranscript=%d", lc.readTranscript)
	}
	if !lc.readTranscriptBeforeCancel {
		t.Fatal("transcript harvested AFTER teardown; the log is gone by then")
	}
	if lc.canceled != 1 {
		t.Fatalf("sandbox not torn down: canceled=%d", lc.canceled)
	}
	// The parsed parts landed in the durable store at turn 1, seq 0..1.
	parts, err := st.AgentSessionTranscript(context.Background(), id, -1, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 {
		t.Fatalf("stored %d parts, want 2 (the two log lines)", len(parts))
	}
	if parts[0].Seq != 0 || parts[1].Seq != 1 || parts[0].Turn != 1 {
		t.Fatalf("parts = %+v, want seq 0,1 turn 1", parts)
	}
	if !strings.Contains(string(parts[0].Part), `"text":"hello"`) {
		t.Fatalf("first part = %s, want the driver's `.part` payload", parts[0].Part)
	}
}

// A redispatched (steered) turn's harvest concatenates after the prior turn's
// parts rather than colliding.
func TestCompleterHarvestConcatenatesTurns(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	// Pretend turn 1 was already recorded (seq 0,1).
	_ = st.AppendAgentSessionTranscript(context.Background(), id, []store.AgentSessionTranscriptPart{
		{Seq: 0, Turn: 1, Part: []byte(`{"type":"text","text":"t1"}`)},
		{Seq: 1, Turn: 1, Part: []byte(`{"type":"text","text":"t1b"}`)},
	})
	row := st.rows[id] // this reconcile finalizes turn 2
	row.Turns = 2
	st.rows[id] = row
	lc.transcriptLog = `{"part":{"type":"text","text":"t2"}}`
	c.Reconcile(context.Background())

	parts, _ := st.AgentSessionTranscript(context.Background(), id, -1, 1<<30)
	if len(parts) != 3 {
		t.Fatalf("stored %d parts, want 3 (turn 2 appended)", len(parts))
	}
	if parts[2].Seq != 2 || parts[2].Turn != 2 {
		t.Fatalf("turn-2 part = %+v, want seq 2 turn 2", parts[2])
	}
}

// The harvest enforces the SAME cumulative session cap as the gateway's live
// tee (w1/m65 F10): seeded near the cap, only the parts that still fit are
// stored — never a per-harvest fresh budget.
func TestCompleterHarvestRespectsCumulativeQuota(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	c.MaxTranscriptBytes = 100 // small quota so the test needs no 64 MiB payloads
	// Turn 1 already stored 80 bytes; this reconcile finalizes turn 2.
	_ = st.AppendAgentSessionTranscript(context.Background(), id, []store.AgentSessionTranscriptPart{
		{Seq: 0, Turn: 1, Part: []byte(strings.Repeat("x", 80))},
	})
	row := st.rows[id]
	row.Turns = 2
	st.rows[id] = row
	// The harvested part (~56 bytes with its JSON envelope) exceeds the 20
	// bytes left under the cap, so nothing is appended.
	lc.transcriptLog = `{"part":{"type":"text","text":"` + strings.Repeat("y", 30) + `"}}`
	c.Reconcile(context.Background())

	parts, _ := st.AgentSessionTranscript(context.Background(), id, -1, 1<<30)
	var total int64
	for _, p := range parts {
		total += int64(len(p.Part))
	}
	if total > c.MaxTranscriptBytes {
		t.Fatalf("stored transcript = %d bytes, exceeds the %d-byte session cap (harvest quota not cumulative)", total, c.MaxTranscriptBytes)
	}
	if len(parts) != 1 {
		t.Fatalf("stored %d parts, want just the seed (oversized harvest dropped)", len(parts))
	}
	if st.rows[id].Phase != PhaseCompleted || lc.canceled != 1 {
		t.Fatalf("quota truncation blocked finalize/teardown: phase=%s canceled=%d", st.rows[id].Phase, lc.canceled)
	}
}

// Harvest is best-effort: a read failure must not strand the session or block
// teardown.
func TestCompleterHarvestFailureDoesNotBlockTeardown(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	lc.transcriptErr = errors.New("exec unreachable")
	c.Reconcile(context.Background())
	if st.rows[id].Phase != PhaseCompleted || lc.canceled != 1 {
		t.Fatalf("harvest failure blocked finalize/teardown: phase=%s canceled=%d", st.rows[id].Phase, lc.canceled)
	}
}

func TestCompleterNoChangeCompletesWithoutPR(t *testing.T) {
	c, st, lc, pr, id := completerFixture(succeededStatus(false), nil)
	c.Reconcile(context.Background())
	if pr.calls != 0 {
		t.Fatalf("no-op turn opened a PR: %d", pr.calls)
	}
	if st.rows[id].Phase != PhaseCompleted || st.rows[id].PRURL != "" || lc.canceled != 1 {
		t.Fatalf("no-op completion = %+v", st.rows[id])
	}
}

// An unreadable config must not block completion: the branch is already pushed,
// so the Completer still opens the draft PR — with an untitled fallback title.
func TestCompleterUnreadableConfigStillOpensPR(t *testing.T) {
	c, st, _, pr, id := completerFixture(succeededStatus(true), nil)
	row := st.rows[id]
	row.AgentConfig = []byte("{not json")
	st.rows[id] = row
	c.Reconcile(context.Background())

	if pr.calls != 1 {
		t.Fatalf("unreadable config did not open a PR: calls=%d", pr.calls)
	}
	if pr.last.title != "bex agent session "+id {
		t.Fatalf("unreadable config PR title = %q, want the untitled fallback", pr.last.title)
	}
	if st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("unreadable config blocked completion: %+v", st.rows[id])
	}
}

func TestCompleterFailedTurnRecordsReason(t *testing.T) {
	raw, _ := json.Marshal(statusReport{State: "failed", Error: "agent crashed"})
	c, st, lc, pr, id := completerFixture(string(raw), nil)
	c.Reconcile(context.Background())
	if pr.calls != 0 || st.rows[id].Phase != PhaseFailed || st.rows[id].FailureReason != "agent crashed" || lc.canceled != 1 {
		t.Fatalf("failed row = %+v pr=%d", st.rows[id], pr.calls)
	}
}

func TestCompleterPROpenFailureFailsSession(t *testing.T) {
	c, st, lc, pr, id := completerFixture(succeededStatus(true), nil)
	pr.err = &github.APIError{Status: 500}
	c.Reconcile(context.Background())
	if st.rows[id].Phase != PhaseFailed || st.rows[id].FailureReason == "" || lc.canceled != 1 {
		t.Fatalf("pr-failure row = %+v", st.rows[id])
	}
}

func TestCompleterLostSandboxFailsSession(t *testing.T) {
	c, st, _, _, id := completerFixture("", core.ErrNotFound)
	c.Reconcile(context.Background())
	if st.rows[id].Phase != PhaseFailed || !strings.Contains(st.rows[id].FailureReason, "terminated") {
		t.Fatalf("lost-sandbox row = %+v", st.rows[id])
	}
}

func TestCompleterStillRunningIsLeftAlone(t *testing.T) {
	c, st, pr, _, id := completerFixtureRunning()
	_ = pr
	c.Reconcile(context.Background())
	if st.rows[id].Phase != PhaseRunning {
		t.Fatalf("running session was finalized prematurely: %+v", st.rows[id])
	}
}

func completerFixtureRunning() (*Completer, *fakeStore, *fakeLifecycle, *fakePR, string) {
	// Empty status file (turn still starting) and a running-state report both mean
	// "not ready"; use the running-state report here.
	raw, _ := json.Marshal(statusReport{State: "running"})
	return completerFixture(string(raw), nil)
}

func TestBuildEvidenceBoundsAndMarksTruncation(t *testing.T) {
	commands := make([]string, maxEvidenceCommands+5)
	for i := range commands {
		commands[i] = fmt.Sprintf("cmd-%d", i)
	}
	ev := buildEvidence(statusReport{
		Evidence: statusEvidence{CommandLog: commands, OutputTail: strings.Repeat("x", maxEvidenceOutputTail+100)},
		Delivery: statusDelivery{Commits: 2},
	})
	if len(ev.CommandLog) != maxEvidenceCommands || len(ev.OutputTail) != maxEvidenceOutputTail || !ev.Truncated {
		t.Fatalf("evidence not bounded: cmds=%d tail=%d trunc=%v", len(ev.CommandLog), len(ev.OutputTail), ev.Truncated)
	}
}

// --- Steering ---

func steerableFixture(t *testing.T) (*Service, *fakeStore, *fakeLifecycle, string) {
	t.Helper()
	svc, st, _, lc := fixture()
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	// Drive it to a completed terminal state so it is steerable.
	row := st.rows[created.ID]
	row.Phase = PhaseCompleted
	st.rows[created.ID] = row
	return svc, st, lc, created.ID
}

func TestSteerRedispatchesFreshSandboxOnSameBranch(t *testing.T) {
	svc, st, lc, id := steerableFixture(t)
	beforeCreated, beforeCanceled := lc.created, lc.canceled
	view, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "also update the docs"})
	if err != nil {
		t.Fatal(err)
	}
	// Steer returns FAST (w2/m64): the redispatching phase with no ticket; the
	// fresh sandbox is provisioned in the background (the sync runner completes it).
	if view.Phase != PhaseRedispatching || view.Ticket != "" || view.URL != "" {
		t.Fatalf("steer should return a fast, ticketless redispatching view: %+v", view)
	}
	if lc.created != beforeCreated+1 || lc.canceled != beforeCanceled+1 {
		t.Fatalf("re-dispatch not clean: created=%d canceled=%d", lc.created, lc.canceled)
	}
	if lc.driverEnv["BEX_AGENT_PROMPT"] != "also update the docs" || lc.branch != "bex-agent/session-test" {
		t.Fatalf("steer env/branch = %v %q", lc.driverEnv, lc.branch)
	}
	// The background re-dispatch converged the store row: a second turn recorded on
	// the redispatch delivery path, adopting the fresh sandbox.
	settled := st.rows[id]
	if settled.Turns != 2 || settled.DeliveryMode != DeliveryRedispatch || settled.Phase != PhaseRunning {
		t.Fatalf("converged row = %+v, want turns=2 redispatch running", settled)
	}
}

func TestSteerRejectsCanceledAndInFlightSessions(t *testing.T) {
	svc, st, _, id := steerableFixture(t)

	row := st.rows[id]
	row.Phase = PhaseCanceled
	st.rows[id] = row
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "go"}); !isCode(err, "AGENT_SESSION_NOT_STEERABLE") {
		t.Fatalf("steer canceled = %v", err)
	}

	row = st.rows[id]
	row.Phase = PhaseRunning
	st.rows[id] = row
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "go"}); !isCode(err, "AGENT_SESSION_TURN_IN_FLIGHT") {
		t.Fatalf("steer in-flight = %v", err)
	}
}

func TestSteerRequiresPromptAndAuthorization(t *testing.T) {
	svc, _, _, id := steerableFixture(t)
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "   "}); !isCode(err, "AGENT_SESSION_INPUT_INVALID") {
		t.Fatalf("empty prompt = %v", err)
	}
	if _, err := svc.Steer(caller("bob"), SteerRequest{SessionID: id, Prompt: "go"}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("foreign steer = %v, want forbidden", err)
	}
}

func isCode(err error, code string) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == code
}

// --- Open in Zed: deferred teardown (ADR054 D6) -----------------------------

// While an editor SSH session is open, a finished turn finalizes and captures
// its transcript but leaves the sandbox alive; once the connection closes, the
// reaper reclaims it and clears the sandbox id.
func TestCompleterDefersTeardownWhileEditorSSHOpen(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	lc.transcriptLog = `{"at":"t","type":"ui-message","part":{"type":"text","text":"hi"}}`
	st.openSSH[id] = time.Now() // a live "Open in Zed" session

	c.Reconcile(context.Background())

	if st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("session should still finalize while SSH is open, phase=%s", st.rows[id].Phase)
	}
	if lc.canceled != 0 {
		t.Fatalf("sandbox torn down despite open editor SSH (canceled=%d)", lc.canceled)
	}
	if st.rows[id].SandboxID == "" {
		t.Fatal("sandbox id cleared while teardown is deferred; the reaper would lose the sandbox")
	}
	if len(st.transcript[id]) == 0 {
		t.Fatal("transcript should be captured even while teardown is deferred")
	}

	// The editor disconnects; the next tick reaps the sandbox.
	delete(st.openSSH, id)
	c.Reconcile(context.Background())

	if lc.canceled != 1 {
		t.Fatalf("sandbox not reaped after SSH closed (canceled=%d)", lc.canceled)
	}
	if st.rows[id].SandboxID != "" {
		t.Fatalf("sandbox id not cleared after reap: %q", st.rows[id].SandboxID)
	}
}

// A leaked ssh_sessions row (older than the SSH session cap — a crashed gateway
// replica) must not pin a sandbox forever: teardown proceeds normally.
func TestCompleterStaleSSHRowDoesNotPinSandbox(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	st.openSSH[id] = time.Now().Add(-5 * time.Hour) // older than the 4h grace

	c.Reconcile(context.Background())

	if lc.canceled != 1 {
		t.Fatalf("stale SSH row pinned the sandbox (canceled=%d)", lc.canceled)
	}
	if st.rows[id].SandboxID != "" {
		t.Fatalf("sandbox id not cleared: %q", st.rows[id].SandboxID)
	}
}

// --- ADR059 D2: last-interaction idle grace ---------------------------------

// With a positive idle grace and no editor connected, a finished turn keeps its
// sandbox until the idle window elapses since the turn end, then the reaper
// reclaims it. This is the Active-tier generalization of the D6 immediate reap.
func TestCompleterIdleGraceDefersUntilElapsed(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	c.IdleTTL = 30 * time.Minute
	base := st.now
	c.Now = func() time.Time { return base }

	c.Reconcile(context.Background()) // finalize; idle ≈ 0 < 30m ⇒ defer
	if st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("turn should finalize while grace holds, phase=%s", st.rows[id].Phase)
	}
	if lc.canceled != 0 || st.rows[id].SandboxID == "" {
		t.Fatalf("sandbox reaped inside the idle grace (canceled=%d, sandbox=%q)", lc.canceled, st.rows[id].SandboxID)
	}

	c.Now = func() time.Time { return base.Add(20 * time.Minute) } // still inside
	c.Reconcile(context.Background())
	if lc.canceled != 0 {
		t.Fatalf("sandbox reaped before grace elapsed (canceled=%d)", lc.canceled)
	}

	c.Now = func() time.Time { return base.Add(40 * time.Minute) } // past the window
	c.Reconcile(context.Background())
	if lc.canceled != 1 {
		t.Fatalf("sandbox not reaped after grace elapsed (canceled=%d)", lc.canceled)
	}
	if st.rows[id].SandboxID != "" {
		t.Fatalf("sandbox id not cleared after reap: %q", st.rows[id].SandboxID)
	}
}

// The idle clock is measured from the LATEST interaction: a later editor SSH
// disconnect restarts it past the turn-end, so a reconnect-then-disconnect keeps
// the sandbox alive well after the turn itself finished.
func TestCompleterIdleGraceMeasuredFromLastSSHDisconnect(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	c.IdleTTL = 30 * time.Minute
	base := st.now
	c.Now = func() time.Time { return base }
	c.Reconcile(context.Background()) // finalize + defer

	// An editor connected then disconnected 35m after the turn end — a fresh
	// interaction that resets the idle clock.
	end := base.Add(35 * time.Minute)
	st.lastSSHEnd[id] = end

	// 40m after the turn end is only 5m after the disconnect: still deferred.
	c.Now = func() time.Time { return base.Add(40 * time.Minute) }
	c.Reconcile(context.Background())
	if lc.canceled != 0 {
		t.Fatalf("reaped 5m after SSH disconnect, grace is 30m (canceled=%d)", lc.canceled)
	}

	// 31m past the disconnect: reaped.
	c.Now = func() time.Time { return end.Add(31 * time.Minute) }
	c.Reconcile(context.Background())
	if lc.canceled != 1 {
		t.Fatalf("not reaped 31m after the last disconnect (canceled=%d)", lc.canceled)
	}
}

// An OPEN editor session pins idle at zero regardless of how far past the grace
// the clock has moved: a live edit is never killed by the idle reaper.
func TestCompleterOpenSSHPinsIdleRegardlessOfGrace(t *testing.T) {
	c, st, lc, _, id := completerFixture(succeededStatus(true), nil)
	c.IdleTTL = 30 * time.Minute
	base := st.now
	st.openSSH[id] = base.Add(time.Minute) // a live editor within the SSH cap window
	c.Now = func() time.Time { return base.Add(2 * time.Hour) }

	c.Reconcile(context.Background())
	if lc.canceled != 0 {
		t.Fatalf("sandbox reaped despite an open editor SSH, 2h past the grace (canceled=%d)", lc.canceled)
	}
	if st.rows[id].SandboxID == "" {
		t.Fatal("sandbox id cleared while an editor is connected; the reaper would lose it")
	}
}
