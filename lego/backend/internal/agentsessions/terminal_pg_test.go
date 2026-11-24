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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w10/m615 against the real store: the session page's Terminal ticket starts
// a closed session's sandbox through Resume as the caller, once however many
// tabs ask, and refuses what an SSH login would be refused.

// terminal serves the Terminal on the env, with a member ("sensitive") who
// may view the sandbox but not start a session, beside the env's developer
// (alice), contributor (contrib) and viewer.
func (e *retainEnv) terminal(t *testing.T) {
	t.Helper()
	e.svc.ShellTicketSecret, e.svc.ShellWSURL = []byte("shell-secret"), terminalWS
	e.fga.members["sensitive"] = e.ws
	roles := e.svc.Authz
	e.svc.Authz = checkerFunc(func(ctx context.Context, subject, relation, object string) (bool, error) {
		switch who := strings.TrimPrefix(subject, "user:"); {
		case relation == core.RelCanViewSensitive && (who == "contrib" || who == "viewer"):
			return false, nil
		case relation == core.RelCanViewSensitive:
			return roles.Check(ctx, subject, core.RelCanOperate, object) // a member of its workspace
		case relation == core.RelCanCreate && who == "sensitive":
			return false, nil
		}
		return roles.Check(ctx, subject, relation, object)
	})
}

func (e *retainEnv) mintTerminal(t *testing.T, as string) (ShellSessionView, error) {
	t.Helper()
	return e.svc.CreateShellSession(caller(as), e.id)
}

func TestTerminalStartsAClosedSessionOnceAcrossTabsPG(t *testing.T) {
	e := editorEnv(t, gitStatus(statusGit{}))
	e.noImage(t)
	e.terminal(t)
	other := e.create(t, "") // another live sandbox in the workspace
	var starts []func()
	e.svc.dispatchRunner = func(fn func()) { starts = append(starts, fn) }

	// A contributor and a viewer may not view the sandbox at all; a member
	// who may view it but not start a session is told what SSH tells them.
	for _, who := range []string{"contrib", "viewer"} {
		if _, err := e.mintTerminal(t, who); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("mint as %s = %v, want forbidden", who, err)
		}
	}
	if _, err := e.mintTerminal(t, "sensitive"); !isCode(err, core.CodeAgentSessionNotAttachable) {
		t.Fatalf("mint by a member who may not start it = %v, want the closed sandbox's refusal", err)
	}
	// Admission refuses as it refuses a turn: the live-sandbox cap.
	if e.phase(t, other.ID) != PhaseRunning {
		t.Fatalf("the other session is %s, want running", e.phase(t, other.ID))
	}
	e.svc.MaxLiveSandboxes = 1
	if _, err := e.mintTerminal(t, "alice"); !isCode(err, codeLiveLimit) {
		t.Fatalf("mint past the live-sandbox cap = %v, want %s", err, codeLiveLimit)
	}
	e.svc.MaxLiveSandboxes = 0
	if len(starts) != 0 || e.session(t).Phase != PhaseCompleted {
		t.Fatalf("after the refusals: %d starts, phase %s; want none and completed", len(starts), e.session(t).Phase)
	}

	// Two tabs open it: two tickets, one start.
	for tab := 1; tab <= 2; tab++ {
		view, err := e.mintTerminal(t, "alice")
		if err != nil || view.Sandbox != ShellSandboxStarting || view.Ticket == "" {
			t.Fatalf("tab %d's mint = %+v, %v; want a ticket while it starts", tab, view, err)
		}
	}
	if len(starts) != 1 || e.session(t).Phase != PhaseResuming {
		t.Fatalf("starts = %d phase %s, want one start while resuming", len(starts), e.session(t).Phase)
	}
}

// Once the sandbox is up, the member who could not start it opens its shell.
func TestTerminalOpensAStartedSandboxForItsViewersPG(t *testing.T) {
	e := editorEnv(t, gitStatus(statusGit{}))
	e.noImage(t)
	e.terminal(t)
	e.resumeReady(t)
	view, err := e.mintTerminal(t, "sensitive")
	if err != nil || view.Sandbox != ShellSandboxUp {
		t.Fatalf("mint on the started sandbox = %+v, %v; want it up", view, err)
	}
}

func TestTerminalRefusesArchivedAndCanceledSessionsPG(t *testing.T) {
	e := editorEnv(t, gitStatus(statusGit{}))
	e.terminal(t)
	if _, err := e.svc.Archive(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.mintTerminal(t, "alice"); !isCode(err, core.CodeAgentSessionNotAttachable) || !strings.Contains(err.Error(), "is archived") {
		t.Fatalf("mint on an archived session = %v, want its refusal", err)
	}
	if _, err := e.svc.Unarchive(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Cancel(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.mintTerminal(t, "alice"); !isCode(err, core.CodeAgentSessionNotAttachable) || !strings.Contains(err.Error(), "was canceled") {
		t.Fatalf("mint on a canceled session = %v, want its refusal", err)
	}
	if e.session(t).SandboxID != "" {
		t.Fatalf("a refused mint started a sandbox: %+v", e.session(t))
	}
}
