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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
)

// w10/m615 (ADR054 D10): the session page's Terminal ticket. A caller who may
// view the sandbox gets a ticket for it; a closed one is started as the
// caller when the caller may start it, and refused with the SSH path's words
// when not.

const terminalWS = "wss://ssh.bex.co/shell"

// terminalRoles grants each subject the relations its role holds in the
// session's workspace: a developer (alice) everything a Terminal needs, a
// member who may view the sandbox but not start a session (sensitive-only),
// and a contributor (contrib) neither can_view_sensitive nor can_create.
type terminalRoles struct {
	fga   *fakeFGA
	roles map[string][]string
}

func (r terminalRoles) Check(ctx context.Context, subject, relation, object string) (bool, error) {
	if ok, err := r.fga.Check(ctx, subject, core.RelCanOperate, object); err != nil || !ok {
		return false, err // not a member of the object's workspace
	}
	held, ok := r.roles[strings.TrimPrefix(subject, "user:")]
	if !ok {
		held = []string{core.RelCanViewSensitive, core.RelCanOperate, core.RelCanCreate}
	}
	for _, rel := range held {
		if rel == relation {
			return true, nil
		}
	}
	return false, nil
}

// terminalFixture is a session whose turn completed and whose sandbox the
// idle close ended, on a deployment that serves the Terminal.
func terminalFixture(t *testing.T) (*Service, *fakeStore, *fakeLifecycle, string) {
	t.Helper()
	svc, st, lc, id := closedFixture(t)
	fga := svc.Authz.(*fakeFGA)
	for _, subject := range []string{"sensitive-only", "contrib", "key-ro", "key-rw"} {
		fga.members[subject] = "tea-a"
	}
	svc.Authz = terminalRoles{fga: fga, roles: map[string][]string{
		"sensitive-only": {core.RelCanViewSensitive, core.RelCanOperate},
		"contrib":        {core.RelCanOperate},
	}}
	svc.ShellTicketSecret, svc.ShellWSURL = []byte("shell-secret"), terminalWS
	return svc, st, lc, id
}

// apiKey is a bex API key's identity, granted the given permission levels.
func apiKey(svc *Service, subject string, grants ...core.KeyGrantEntry) context.Context {
	svc.KeyGrants = keyGrants(func(_ context.Context, clientID string) (core.KeyGrant, error) {
		if clientID != subject {
			return core.KeyGrant{}, errors.New("unknown key")
		}
		return core.KeyGrant{Grants: grants}, nil
	})
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, ClientID: subject, Method: core.MethodOAuth2})
}

type keyGrants func(context.Context, string) (core.KeyGrant, error)

func (f keyGrants) APIKeyGrant(ctx context.Context, clientID string) (core.KeyGrant, error) {
	return f(ctx, clientID)
}

var (
	sandboxExecution = core.KeyGrantEntry{Permission: "sandbox_execution", Level: core.KeyLevelExecute}
	agentsReadWrite  = core.KeyGrantEntry{Permission: "agents", Level: core.KeyLevelReadWrite}
)

func mintTerminal(t *testing.T, svc *Service, ctx context.Context, id string) (ShellSessionView, shellticket.Claims) {
	t.Helper()
	view, err := svc.CreateShellSession(ctx, id)
	if err != nil {
		t.Fatalf("CreateShellSession: %v", err)
	}
	claims, err := shellticket.Verify(svc.ShellTicketSecret, view.Ticket, svc.Now())
	if err != nil {
		t.Fatalf("the ticket does not verify: %v", err)
	}
	if view.URL != terminalWS || claims.SessionID != id || claims.ServiceID != "" || claims.InstanceID != "" {
		t.Fatalf("ticket = %+v at %q, want one naming only the session at the gateway's WebSocket", claims, view.URL)
	}
	if exp, err := time.Parse(time.RFC3339, view.ExpiresAt); err != nil || exp.Unix() != claims.ExpiresAt || claims.ExpiresAt-claims.IssuedAt != int64(shellTicketTTL/time.Second) {
		t.Fatalf("expiresAt %q (%v), claims %d..%d, want the 90 s TTL", view.ExpiresAt, err, claims.IssuedAt, claims.ExpiresAt)
	}
	return view, claims
}

func TestTerminalOnAClosedSessionStartsItOnceAsTheCaller(t *testing.T) {
	svc, st, lc, id := terminalFixture(t)
	created := lc.created
	view, claims := mintTerminal(t, svc, caller("alice"), id)
	if view.Sandbox != ShellSandboxStarting || claims.Subject != "alice" || claims.KeyAction != "" {
		t.Fatalf("mint = sandbox %q, claims %+v; want starting, alice's, naming no key action", view.Sandbox, claims)
	}
	row := st.rows[id]
	if row.Phase != PhaseResuming || lc.created != created+1 || !row.EditorSandbox() {
		t.Fatalf("after the mint: phase %s, %d sandboxes created; want resuming on one editor's sandbox", row.Phase, lc.created-created)
	}
	// A second tab, or a retry, while it starts: a ticket, the same start.
	row.SandboxID = ""
	st.rows[id] = row
	if again, _ := mintTerminal(t, svc, caller("alice"), id); again.Sandbox != ShellSandboxStarting || lc.created != created+1 {
		t.Fatalf("a second mint = %q with %d starts, want starting and still one", again.Sandbox, lc.created-created)
	}
}

func TestTerminalOnALiveSandboxOpensAtOnceForAViewerOfIt(t *testing.T) {
	svc, st, lc, id := terminalFixture(t)
	row := st.rows[id]
	row.SandboxID = "os-up"
	st.rows[id] = row
	created := lc.created
	// May view the sandbox, may not start a session: the shell opens, nothing starts.
	view, claims := mintTerminal(t, svc, caller("sensitive-only"), id)
	if view.Sandbox != ShellSandboxUp || claims.Subject != "sensitive-only" || lc.created != created || st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("mint = %q for %q, %d starts, phase %s; want up, nothing started", view.Sandbox, claims.Subject, lc.created-created, st.rows[id].Phase)
	}
	// A turn's sandbox still starting: the gateway waits for it.
	row.Phase, row.SandboxID = PhaseCreating, ""
	st.rows[id] = row
	if view, _ := mintTerminal(t, svc, caller("sensitive-only"), id); view.Sandbox != ShellSandboxStarting || lc.created != created {
		t.Fatalf("mint on a starting turn = %q with %d starts, want starting and none", view.Sandbox, lc.created-created)
	}
}

// A caller who may view the sandbox but not start it is told what the SSH
// banner tells them (w10/043), and nothing starts.
func TestTerminalRefusesToStartAClosedSessionForAViewer(t *testing.T) {
	svc, st, lc, id := terminalFixture(t)
	created := lc.created
	_, err := svc.CreateShellSession(caller("sensitive-only"), id)
	if !isCode(err, core.CodeAgentSessionNotAttachable) || !strings.Contains(err.Error(), "send the session a message") {
		t.Fatalf("mint by a member who may not start it = %v, want the closed sandbox's refusal", err)
	}
	if lc.created != created || st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("after the refusal: %d starts, phase %s; want none, completed", lc.created-created, st.rows[id].Phase)
	}
}

// A contributor holds no can_view_sensitive, and a session the caller cannot
// see answers as a missing one does.
func TestTerminalNeedsCanViewSensitive(t *testing.T) {
	svc, _, lc, id := terminalFixture(t)
	created := lc.created
	if _, err := svc.CreateShellSession(caller("contrib"), id); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("contributor's mint = %v, want forbidden", err)
	}
	other, missing := "ags-d4ae2ab0f8cs73a9ejo0", "ags-d4ae2ab0f8cs73a9ejog"
	svc.Authz.(terminalRoles).fga.parents[other] = "tea-b"
	for _, sid := range []string{other, missing} {
		if _, err := svc.CreateShellSession(caller("alice"), sid); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("mint on %s = %v, want the same forbidden as for any session alice cannot see", sid, err)
		}
	}
	if lc.created != created {
		t.Fatalf("%d sandboxes started by refused mints", lc.created-created)
	}
}

func TestTerminalSaysWhyACanceledOrArchivedSessionHasNone(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*Service, string)
		want string
	}{
		"canceled": {func(svc *Service, id string) {
			st := svc.Store.(*fakeStore)
			row := st.rows[id]
			row.Phase = PhaseCanceled
			st.rows[id] = row
		}, "was canceled"},
		"archived": {func(svc *Service, id string) {
			if _, err := svc.Archive(caller("alice"), id); err != nil {
				t.Fatal(err)
			}
		}, "is archived"},
	} {
		t.Run(name, func(t *testing.T) {
			svc, _, lc, id := terminalFixture(t)
			tc.edit(svc, id)
			created := lc.created
			_, err := svc.CreateShellSession(caller("alice"), id)
			if !isCode(err, core.CodeAgentSessionNotAttachable) || !strings.Contains(err.Error(), tc.want) || lc.created != created {
				t.Fatalf("mint = %v with %d starts, want the %s refusal and none", err, lc.created-created, name)
			}
		})
	}
}

// A bex API key opens a shell under Sandbox execution, the action the gateway
// then holds it to, and starts a closed sandbox only when its grant also
// covers starting a session.
func TestTerminalHoldsABexAPIKeyToItsGrant(t *testing.T) {
	svc, st, lc, id := terminalFixture(t)
	created := lc.created
	if _, err := svc.CreateShellSession(apiKey(svc, "key-ro", sandboxExecution), id); !isCode(err, core.CodeAgentSessionNotAttachable) || lc.created != created {
		t.Fatalf("a key without agents.manage = %v with %d starts, want the closed sandbox's refusal and none", err, lc.created-created)
	}
	view, claims := mintTerminal(t, svc, apiKey(svc, "key-rw", sandboxExecution, agentsReadWrite), id)
	if view.Sandbox != ShellSandboxStarting || claims.KeyAction != string(ShellKeyAction) || lc.created != created+1 || st.rows[id].Phase != PhaseResuming {
		t.Fatalf("a key with agents.manage = %q, claims %+v, %d starts; want starting, held to %s", view.Sandbox, claims, lc.created-created, ShellKeyAction)
	}
}

// Resume's own refusals keep their code: here the live-sandbox cap.
func TestTerminalKeepsAdmissionsRefusal(t *testing.T) {
	svc, st, lc, id := terminalFixture(t)
	seedLiveSession(st, "tea-a")
	svc.MaxLiveSandboxes = 1
	created := lc.created
	if _, err := svc.CreateShellSession(caller("alice"), id); !isCode(err, codeLiveLimit) || lc.created != created {
		t.Fatalf("mint past the live-sandbox cap = %v with %d starts, want %s", err, lc.created-created, codeLiveLimit)
	}
}

func TestTerminalUnconfiguredIsUnavailableAndUnadvertised(t *testing.T) {
	svc, _, _, id := terminalFixture(t)
	caps, err := svc.Capabilities(caller("alice"), "tea-a")
	if err != nil || !caps.TerminalEnabled {
		t.Fatalf("capabilities = %+v, %v; want terminalEnabled", caps, err)
	}
	svc.ShellWSURL = ""
	if _, err := svc.CreateShellSession(caller("alice"), id); !errors.Is(err, core.ErrShellUnavailable) {
		t.Fatalf("mint without the WebSocket URL = %v, want SHELL_UNAVAILABLE", err)
	}
	if caps, err := svc.Capabilities(caller("alice"), "tea-a"); err != nil || caps.TerminalEnabled {
		t.Fatalf("capabilities = %+v, %v; want no terminal", caps, err)
	}
}

// A person's OAuth delegation needs bex.sensitive for the shell, and the start
// a closed sandbox needs is a write: a token without bex.write opens a live
// sandbox's shell but is refused the start, with the scope it lacks.
func TestTerminalHoldsAnOAuthDelegationToItsScopes(t *testing.T) {
	svc, st, lc, id := terminalFixture(t)
	delegated := core.WithIdentity(context.Background(), core.Identity{
		Subject: "alice", Method: core.MethodOAuth2, Human: true, ClientID: "cli",
		CanonicalScopes: core.ScopeRead + " " + core.ScopeSensitive,
	})
	created := lc.created
	_, err := svc.CreateShellSession(delegated, id)
	if !isCode(err, core.InsufficientScopeCode) || !strings.Contains(err.Error(), core.ScopeWrite) || lc.created != created {
		t.Fatalf("a start without bex.write = %v with %d starts, want the scope refusal and none", err, lc.created-created)
	}
	row := st.rows[id]
	row.SandboxID = "os-up"
	st.rows[id] = row
	if view, _ := mintTerminal(t, svc, delegated, id); view.Sandbox != ShellSandboxUp {
		t.Fatalf("a live sandbox's shell = %q, want up", view.Sandbox)
	}
}
