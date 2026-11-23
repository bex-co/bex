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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m564 (ADR054 D8) against the real store: resume on a session whose
// sandbox closed starts one from its kept disk, else from its branch, runs no
// turn, and the sandbox then follows a person's idle close, keeping the disk
// again at it.

// editorEnv is a person's session whose first turn finished with the given
// status and whose sandbox then closed at the idle timeout.
func editorEnv(t *testing.T, status string) *retainEnv {
	t.Helper()
	e := newRetainEnv(t)
	e.c.IdleTTL = time.Hour
	e.svc.SSHHost = "ssh.bex.co"
	e.finished(t, status)
	e.idleOut(t)
	if row := e.session(t); row.SandboxID != "" || row.Phase != PhaseCompleted {
		t.Fatalf("before resume = %+v, want a completed session with its sandbox closed", row)
	}
	return e
}

// resumeReady resumes the closed session, which starts its sandbox inline,
// and has the driver report its checkout ready.
func (e *retainEnv) resumeReady(t *testing.T) store.AgentSession {
	t.Helper()
	view, err := e.svc.Resume(caller("alice"), e.id)
	if err != nil {
		t.Fatal(err)
	}
	// The address stays while it starts: an SSH login to it waits (w10/m597).
	if view.Phase != PhaseResuming || view.SSHAddress != e.id+"@ssh.bex.co" {
		t.Fatalf("resume = phase %s address %q, want resuming with the address", view.Phase, view.SSHAddress)
	}
	started := e.session(t)
	if started.Phase != PhaseResuming || !started.EditorSandbox() {
		t.Fatalf("after the start = %+v, want resuming on the editor's sandbox", started)
	}
	if _, ok := e.lc.driverEnv["BEX_AGENT_PROMPT"]; ok || e.lc.driverEnv[envEditor] != "1" {
		t.Fatalf("driver env = %v, want the editor flag and no prompt", e.lc.driverEnv)
	}
	e.lc.status = readyStatus
	e.settle(t)
	settled := e.session(t)
	if settled.Phase != PhaseCompleted || settled.SandboxID != started.SandboxID {
		t.Fatalf("settled = %+v, want completed with the editor's sandbox", settled)
	}
	got, err := e.svc.Get(caller("alice"), e.id)
	if err != nil || got.SSHAddress != e.id+"@ssh.bex.co" || got.Turns != 1 {
		t.Fatalf("view = address %q turns %d / %v, want the address and no new turn", got.SSHAddress, got.Turns, err)
	}
	if turns, err := e.pg.AgentSessionTurns(context.Background(), e.id); err != nil || len(turns) != 1 {
		t.Fatalf("turns = %d / %v, want the one turn: resume runs none", len(turns), err)
	}
	return settled
}

func TestResumeStartsAClosedSessionFromItsKeptDiskPG(t *testing.T) {
	e := editorEnv(t, gitStatus(undeliveredGit))
	kept := e.live(t)
	e.resumeReady(t)
	if e.lc.image.ID != kept.ID || e.lc.image.Digest != testImageDigest {
		t.Fatalf("the editor's sandbox booted %+v, want the kept disk %s", e.lc.image, kept.ID)
	}
	if turn := e.turn(t, 1); turn.Retention != store.TurnRetentionContinued {
		t.Fatalf("turn 1 retention = %q, want continued into the editor's sandbox", turn.Retention)
	}
	// The editor's close reads the checkout in its sandbox and keeps the disk
	// again, edits included; the next message continues from that one.
	e.rs.setCheckout(store.TurnCheckout{Dirty: true})
	reads := e.rs.checkoutReads
	e.idleOut(t)
	again := e.live(t)
	if again.ID == kept.ID || again.State != store.RetainedImageKept || e.rs.checkoutReads != reads+1 || e.session(t).SandboxID != "" {
		t.Fatalf("after the editor's close: image %+v reads %d sandbox %q, want a new kept disk read from the sandbox", again, e.rs.checkoutReads-reads, e.session(t).SandboxID)
	}
	e.send(t, "alice", "carry on")
	if e.lc.image.ID != again.ID {
		t.Fatalf("the next turn booted %+v, want the editor's kept disk %s", e.lc.image, again.ID)
	}
}

func TestResumeStartsAClosedSessionFromItsBranchPG(t *testing.T) {
	e := editorEnv(t, gitStatus(statusGit{}))
	e.noImage(t)
	settled := e.resumeReady(t)
	if e.lc.image != (sandbox.AgentImage{}) || e.lc.branch != settled.Branch {
		t.Fatalf("the editor's sandbox booted %+v on %q, want a fresh checkout of %q", e.lc.image, e.lc.branch, settled.Branch)
	}
	// Clean at its close: it is ended, nothing kept.
	canceled := e.lc.canceled
	e.idleOut(t)
	if e.session(t).SandboxID != "" || e.lc.canceled != canceled+1 || e.handles(t) != 0 {
		t.Fatalf("after the close: sandbox %q ended %d handles %d, want it ended and nothing kept", e.session(t).SandboxID, e.lc.canceled-canceled, e.handles(t))
	}
}

// A message sent while the editor's sandbox starts waits in the queue rather
// than racing the start for the kept disk.
func TestAMessageWaitsForTheEditorStartPG(t *testing.T) {
	e := editorEnv(t, gitStatus(undeliveredGit))
	var pending []func()
	e.svc.dispatchRunner = func(fn func()) { pending = append(pending, fn) }
	if _, err := e.svc.Resume(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	view := e.send(t, "alice", "carry on")
	if view.Queued == nil || view.Phase != PhaseResuming || view.Turns != 1 {
		t.Fatalf("send while starting = queued %+v phase %s turns %d, want it queued", view.Queued, view.Phase, view.Turns)
	}
	if len(pending) != 1 {
		t.Fatalf("starts = %d, want the one editor start", len(pending))
	}
}

// A bex-api restart after the session went resuming but before its start
// bound a sandbox: the Completer returns it to completed once the start's own
// ceiling has passed, never leaving it resuming.
func TestARestartDuringTheEditorStartLeavesNoResumingSessionPG(t *testing.T) {
	e := editorEnv(t, gitStatus(statusGit{}))
	e.svc.dispatchRunner = func(func()) {} // the replica died before the start ran
	if _, err := e.svc.Resume(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	started := e.session(t)
	e.c.Now = func() time.Time { return started.UpdatedAt.Add(editorStartBound - time.Second) }
	e.c.Reconcile(context.Background())
	if got := e.session(t).Phase; got != PhaseResuming {
		t.Fatalf("phase %s inside the start's ceiling, want resuming", got)
	}
	e.c.Now = func() time.Time { return started.UpdatedAt.Add(editorStartBound) }
	e.c.Reconcile(context.Background())
	if got := e.session(t); got.Phase != PhaseCompleted || got.SandboxID != "" {
		t.Fatalf("after the start's ceiling = %+v, want completed with no sandbox", got)
	}
}
