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
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w6/m564 (ADR054 D8): resume gives a session whose sandbox closed a sandbox
// an editor can open, running no turn, and the Completer brings the session
// back to its finished phase once the checkout is ready.

const readyStatus = `{"state":"ready"}`

// closedFixture is a session whose turn completed and whose sandbox the idle
// close has ended, on a deployment that serves editors over SSH.
func closedFixture(t *testing.T) (*Service, *fakeStore, *fakeLifecycle, string) {
	t.Helper()
	svc, st, lc, id := steerableFixture(t)
	svc.SSHHost = "ssh.bex.co"
	row := st.rows[id]
	row.SandboxID = ""
	st.rows[id] = row
	return svc, st, lc, id
}

func TestResumeStartsAnEditorSandboxForAClosedSession(t *testing.T) {
	svc, st, lc, id := closedFixture(t)
	turns, created := st.rows[id].Turns, lc.created
	view, err := svc.Resume(caller("alice"), id)
	if err != nil {
		t.Fatal(err)
	}
	if view.Phase != PhaseResuming || view.SSHAddress != "" || view.Ticket != "" {
		t.Fatalf("resume = phase %s address %q, want resuming with no address yet", view.Phase, view.SSHAddress)
	}
	row := st.rows[id]
	if lc.created != created+1 || row.SandboxID == "" || row.EditorSandboxID != row.SandboxID || row.Phase != PhaseResuming {
		t.Fatalf("after the start: created %d row %+v, want one sandbox bound as the editor's while resuming", lc.created-created, row)
	}
	// It runs no turn: no prompt, no turn label, no new turn.
	if _, ok := lc.driverEnv["BEX_AGENT_PROMPT"]; ok {
		t.Errorf("the editor's driver got a prompt: %q", lc.driverEnv["BEX_AGENT_PROMPT"])
	}
	if _, ok := lc.driverEnv["BEX_AGENT_TURN"]; ok || lc.driverEnv[envEditor] != "1" {
		t.Errorf("driver env turn=%q editor=%q, want no turn and the editor flag", lc.driverEnv["BEX_AGENT_TURN"], lc.driverEnv[envEditor])
	}
	if lc.driverEnv["BEX_AGENT_BRANCH"] != row.Branch || row.Turns != turns {
		t.Errorf("branch %q turns %d, want the session's branch %q and still %d turns", lc.driverEnv["BEX_AGENT_BRANCH"], row.Turns, row.Branch, turns)
	}
	// Until the checkout is ready the session waits, then it is completed
	// again with its sandbox, which an editor can open.
	c := &Completer{Store: st, Sandbox: lc, IdleTTL: DefaultIdleTimeout, Now: func() time.Time { return st.now }}
	c.Reconcile(context.Background())
	if st.rows[id].Phase != PhaseResuming {
		t.Fatalf("phase %s before the driver reported, want resuming", st.rows[id].Phase)
	}
	lc.status = readyStatus
	c.Reconcile(context.Background())
	got, err := svc.Get(caller("alice"), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != PhaseCompleted || got.SandboxID != row.SandboxID || got.SSHAddress != id+"@ssh.bex.co" {
		t.Fatalf("settled = phase %s sandbox %q address %q, want completed with the sandbox and its address", got.Phase, got.SandboxID, got.SSHAddress)
	}
}

func TestAFailedSessionResumesBackToFailed(t *testing.T) {
	svc, st, lc, id := closedFixture(t)
	row := st.rows[id]
	row.Phase, row.FailureReason = PhaseFailed, "agent turn failed"
	st.rows[id] = row
	if _, err := svc.Resume(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	lc.status = readyStatus
	(&Completer{Store: st, Sandbox: lc, IdleTTL: DefaultIdleTimeout, Now: func() time.Time { return st.now }}).Reconcile(context.Background())
	if got := st.rows[id]; got.Phase != PhaseFailed || got.FailureReason != "agent turn failed" || !got.EditorSandbox() {
		t.Fatalf("settled = %+v, want failed with its reason and the editor's sandbox", got)
	}
}

func TestResumeOfALiveSandboxKeepsItForTheEditor(t *testing.T) {
	svc, st, lc, id := steerableFixture(t)
	before := st.rows[id]
	created := lc.created
	view, err := svc.Resume(caller("alice"), id)
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[id]
	if view.Phase != PhaseCompleted || lc.created != created || row.EditorSandboxID != before.SandboxID || !row.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("resume of a sandbox still up = %+v (created %d), want it kept for the editor with its idle clock restarted", row, lc.created-created)
	}
}

func TestResumeRefusals(t *testing.T) {
	for _, phase := range []string{PhaseCanceled, PhaseCanceling} {
		t.Run(phase, func(t *testing.T) {
			svc, st, _, id := closedFixture(t)
			row := st.rows[id]
			row.Phase, row.SandboxID = phase, "sandbox-going" // a canceling one may still name its sandbox
			st.rows[id] = row
			if _, err := svc.Resume(caller("alice"), id); !isCode(err, "AGENT_SESSION_NOT_RESUMABLE") {
				t.Fatalf("resume %s = %v, want AGENT_SESSION_NOT_RESUMABLE", phase, err)
			}
		})
	}
	t.Run("archived", func(t *testing.T) {
		svc, st, lc, id := closedFixture(t)
		row := st.rows[id]
		row.ArchivedAt = new(time.Now())
		st.rows[id] = row
		created := lc.created
		if _, err := svc.Resume(caller("alice"), id); !isCode(err, "AGENT_SESSION_ARCHIVED") || lc.created != created {
			t.Fatalf("resume archived = %v (created %d), want AGENT_SESSION_ARCHIVED and no sandbox", err, lc.created-created)
		}
	})
	t.Run("another workspace", func(t *testing.T) {
		svc, _, _, id := closedFixture(t)
		if _, err := svc.Resume(caller("bob"), id); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("resume as bob = %v, want forbidden", err)
		}
	})
}

// A message sent while the editor's sandbox starts cannot race it for the
// session's kept disk: it waits (here, with no queue, it is refused as a
// running turn's would be). Stop has no turn to end.
func TestTheEditorStartHoldsMessagesAndStop(t *testing.T) {
	svc, st, _, id := closedFixture(t)
	var pending []func()
	svc.dispatchRunner = func(fn func()) { pending = append(pending, fn) }
	if _, err := svc.Resume(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: id, Prompt: "go on"}); !isCode(err, codeTurnInFlight) {
		t.Fatalf("steer while starting = %v, want %s", err, codeTurnInFlight)
	}
	if _, err := svc.Interrupt(caller("alice"), id); !isCode(err, codeNoTurnRunning) {
		t.Fatalf("stop while starting = %v, want %s", err, codeNoTurnRunning)
	}
	// A second resume while it starts changes nothing.
	if view, err := svc.Resume(caller("alice"), id); err != nil || view.Phase != PhaseResuming {
		t.Fatalf("second resume = %+v / %v, want the start in progress", view, err)
	}
	if len(pending) != 1 || st.rows[id].Phase != PhaseResuming {
		t.Fatalf("starts %d phase %s, want one start", len(pending), st.rows[id].Phase)
	}
}

func TestAFailedEditorStartReturnsTheSessionClosed(t *testing.T) {
	svc, st, lc, id := closedFixture(t)
	lc.createErr = errors.New("pod schedule timeout")
	if _, err := svc.Resume(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	if got := st.rows[id]; got.Phase != PhaseCompleted || got.SandboxID != "" {
		t.Fatalf("after a failed start = %+v, want completed with no sandbox", got)
	}
}

// A cancel during the start wins: the start ends the sandbox it made.
func TestACancelDuringTheEditorStartEndsItsSandbox(t *testing.T) {
	svc, st, lc, id := closedFixture(t)
	var pending []func()
	svc.dispatchRunner = func(fn func()) { pending = append(pending, fn) }
	if _, err := svc.Resume(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Cancel(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	canceled := lc.canceled
	for _, fn := range pending {
		fn()
	}
	if got := st.rows[id]; got.Phase != PhaseCanceled || got.SandboxID != "" || lc.canceled != canceled+1 {
		t.Fatalf("after the start = %+v (sandboxes ended %d), want canceled with the new sandbox ended", got, lc.canceled-canceled)
	}
}

// A bex-api restart mid-start leaves no session resuming: a start that bound
// no sandbox is given up after its own ceiling, a bound one settles from the
// driver's status, at the latest after the turn ceiling, and a lost sandbox is
// dropped.
func TestARestartDuringTheEditorStartConverges(t *testing.T) {
	resuming := func(t *testing.T, sandboxID string) (*Completer, *fakeStore, *fakeLifecycle, string) {
		t.Helper()
		_, st, lc, id := closedFixture(t)
		row := st.rows[id]
		row.Phase, row.Status, row.SandboxID, row.EditorSandboxID = PhaseResuming, "resuming", sandboxID, sandboxID
		st.rows[id] = row
		return &Completer{Store: st, Sandbox: lc}, st, lc, id
	}
	at := func(c *Completer, st *fakeStore, id string, after time.Duration) string {
		c.Now = func() time.Time { return st.rows[id].UpdatedAt.Add(after) }
		c.finalize(context.Background(), st.rows[id])
		return st.rows[id].Phase
	}
	t.Run("unbound", func(t *testing.T) {
		c, st, _, id := resuming(t, "")
		if got := at(c, st, id, editorStartBound-time.Second); got != PhaseResuming {
			t.Fatalf("phase %s inside the start's ceiling, want resuming", got)
		}
		if got := at(c, st, id, editorStartBound); got != PhaseCompleted || st.rows[id].SandboxID != "" {
			t.Fatalf("phase %s past it, want completed with no sandbox", got)
		}
	})
	t.Run("bound, no word", func(t *testing.T) {
		c, st, _, id := resuming(t, "sandbox-9")
		if got := at(c, st, id, turnCeiling(0)-time.Second); got != PhaseResuming {
			t.Fatalf("phase %s while the checkout may still run, want resuming", got)
		}
		if got := at(c, st, id, turnCeiling(0)); got != PhaseCompleted || st.rows[id].SandboxID != "sandbox-9" {
			t.Fatalf("phase %s past the ceiling, want completed with its sandbox for the idle close", got)
		}
	})
	t.Run("bound, setup failed", func(t *testing.T) {
		c, st, lc, id := resuming(t, "sandbox-9")
		lc.status = `{"state":"failed","error":"git fetch failed"}`
		if got := at(c, st, id, time.Second); got != PhaseCompleted || st.rows[id].SandboxID != "sandbox-9" {
			t.Fatalf("phase %s, want completed with the sandbox left to the idle close", got)
		}
	})
	t.Run("bound, sandbox gone", func(t *testing.T) {
		c, st, lc, id := resuming(t, "sandbox-9")
		lc.statusErr = core.ErrNotFound
		if got := at(c, st, id, time.Second); got != PhaseCompleted || st.rows[id].SandboxID != "" {
			t.Fatalf("phase %s sandbox %q, want completed with the lost sandbox dropped", got, st.rows[id].SandboxID)
		}
	})
}

// The editor's sandbox follows a person's idle rule whoever started the
// session: an automation's closes at the harvest, but not one resume started
// for a person. Its close reads no transcript: it ran no turn.
func TestAnEditorSandboxClosesOnAPersonsIdleTimeout(t *testing.T) {
	_, st, lc, id := steerableFixture(t)
	row := st.rows[id]
	row.AutomationID = "aut-nightly"
	st.rows[id] = row
	c := &Completer{Store: st, Sandbox: lc, IdleTTL: DefaultIdleTimeout}
	if grace, _ := c.idleGrace(context.Background(), row, 0); grace != 0 {
		t.Fatalf("an automation's turn sandbox has grace %s, want none", grace)
	}
	row.EditorSandboxID = row.SandboxID
	st.rows[id] = row
	if grace, _ := c.idleGrace(context.Background(), row, 0); grace != DefaultIdleTimeout {
		t.Fatalf("the editor's sandbox has grace %s, want %s", grace, DefaultIdleTimeout)
	}
	c.Now = func() time.Time { return row.UpdatedAt.Add(DefaultIdleTimeout - time.Second) }
	c.reapIdleSandboxes(context.Background())
	if st.rows[id].SandboxID == "" {
		t.Fatal("closed inside the idle timeout")
	}
	c.Now = func() time.Time { return row.UpdatedAt.Add(DefaultIdleTimeout + time.Second) }
	c.reapIdleSandboxes(context.Background())
	if st.rows[id].SandboxID != "" || lc.readTranscript != 0 {
		t.Fatalf("sandbox %q transcript reads %d, want closed without a harvest", st.rows[id].SandboxID, lc.readTranscript)
	}
}

func TestCapabilitiesSayWhetherEditorsCanOpenSessions(t *testing.T) {
	svc, _, _, _ := fixture()
	if caps, err := svc.Capabilities(caller("alice"), "tea-a"); err != nil || caps.SSHEnabled {
		t.Fatalf("caps without BEX_SSH_HOST = %+v / %v, want sshEnabled false", caps, err)
	}
	svc.SSHHost = "ssh.bex.co"
	if caps, err := svc.Capabilities(caller("alice"), "tea-a"); err != nil || !caps.SSHEnabled {
		t.Fatalf("caps with BEX_SSH_HOST = %+v / %v, want sshEnabled", caps, err)
	}
}
