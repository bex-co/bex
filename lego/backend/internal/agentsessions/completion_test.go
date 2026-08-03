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
	if !strings.Contains(pr.last.body, "go test ./...") || !strings.Contains(pr.last.body, "fix.txt") {
		t.Fatalf("PR body missing evidence: %s", pr.last.body)
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
	if view.Phase != PhaseRunning || view.DeliveryMode != DeliveryRedispatch || view.Ticket == "" {
		t.Fatalf("steer view = %+v", view)
	}
	if lc.created != beforeCreated+1 || lc.canceled != beforeCanceled+1 {
		t.Fatalf("re-dispatch not clean: created=%d canceled=%d", lc.created, lc.canceled)
	}
	if lc.driverEnv["BEX_AGENT_PROMPT"] != "also update the docs" || lc.branch != "bex-agent/session-test" {
		t.Fatalf("steer env/branch = %v %q", lc.driverEnv, lc.branch)
	}
	if st.rows[id].Turns != 2 {
		t.Fatalf("turns = %d, want 2", st.rows[id].Turns)
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
