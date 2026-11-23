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
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// vsChecker grants only can_view_sensitive, and only to the subjects in allow —
// modelling the developer/admin ladder the sandbox SSH sink requires (ADR054 D2).
// A contributor (can_operate only) is absent from allow, so it is refused.
type vsChecker struct{ allow map[string]bool }

func (c vsChecker) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	if relation != core.RelCanViewSensitive {
		return false, nil
	}
	return c.allow[strings.TrimPrefix(subject, "user:")], nil
}

func sshResolver(allow map[string]bool, st *fakeStore) *SSHResolver {
	return &SSHResolver{Base: &core.Base{Authz: vsChecker{allow: allow}}, Store: st}
}

// freshDenyingChecker allows on the cached path and denies on the uncached
// one — a member revoked on another replica whose positive is still warm on
// this gateway process (codex round-7 F7's model).
type freshDenyingChecker struct{ vsChecker }

func (c freshDenyingChecker) CheckFresh(_ context.Context, _, _, _ string) (bool, error) {
	return false, nil
}

// TestSSHResolverReassertsFreshAuthorization pins codex round-7 F7: the
// gateway's agent-session SSH handshake converts a verified key into a NEW
// hours-long pods/exec session, so the relation is consulted uncached — a
// just-revoked member riding a stale cached positive is refused.
func TestSSHResolverReassertsFreshAuthorization(t *testing.T) {
	st := newFakeStore()
	id := liveSession(st, "tea-a", "os-abc", PhaseRunning)
	r := &SSHResolver{
		Base:  &core.Base{Authz: freshDenyingChecker{vsChecker{allow: map[string]bool{"alice": true}}}},
		Store: st,
	}
	if _, err := r.ResolveSSHSession(caller("alice"), id); !errors.Is(err, core.ErrForbidden) {
		t.Fatal("revoked-on-fresh-path subject must be refused at the SSH handshake")
	}
}

func liveSession(st *fakeStore, workspace, sandboxID, phase string) string {
	id := ids.New(ids.AgentSession)
	st.rows[id] = store.AgentSession{ID: id, WorkspaceID: workspace, SandboxID: sandboxID, Phase: phase}
	return id
}

func TestSSHResolverResolvesLiveSandbox(t *testing.T) {
	st := newFakeStore()
	id := liveSession(st, "tea-a", "os-abc", PhaseRunning)
	r := sshResolver(map[string]bool{"alice": true}, st)

	target, err := r.ResolveSSHSession(caller("alice"), id)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if target.PodName != "os-abc-0" {
		t.Errorf("pod = %q, want os-abc-0", target.PodName)
	}
	if target.Namespace != "tea-a-sandbox" {
		t.Errorf("namespace = %q, want tea-a-sandbox", target.Namespace)
	}
	if target.Container != "sandbox" {
		t.Errorf("container = %q, want sandbox", target.Container)
	}
	if !target.Sandbox {
		t.Error("target.Sandbox must be true so the transport permits multi-channel + sftp")
	}
	if target.ID != id || target.ServiceID != id {
		t.Errorf("ID/ServiceID = %q/%q, want %q", target.ID, target.ServiceID, id)
	}
}

func TestSSHResolverAllLivePhasesResolve(t *testing.T) {
	for _, phase := range []string{PhaseCreating, PhaseRunning, PhaseRedispatching} {
		st := newFakeStore()
		id := liveSession(st, "tea-a", "os-live", phase)
		if _, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("alice"), id); err != nil {
			t.Errorf("phase %s should resolve, got %v", phase, err)
		}
	}
}

// A sandbox resume is still starting for an editor opens once its checkout
// is ready and the session settles, not while /workspace is being written
// (ADR054 D8).
func TestSSHResolverWaitsForTheEditorStartToSettle(t *testing.T) {
	st := newFakeStore()
	id := liveSession(st, "tea-a", "os-starting", PhaseResuming)
	if _, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("alice"), id); !isCode(err, "AGENT_SESSION_NOT_ATTACHABLE") {
		t.Fatalf("resuming = %v, want AGENT_SESSION_NOT_ATTACHABLE until it settles", err)
	}
}

func TestSSHResolverContributorRefusedBeforeExistence(t *testing.T) {
	st := newFakeStore()
	id := liveSession(st, "tea-a", "os-abc", PhaseRunning)
	// bob is not in the can_view_sensitive allow set (a contributor holds only
	// can_operate). The refusal must be Forbidden, and it must happen before any
	// store read so a denial never leaks whether the session exists.
	_, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("bob"), id)
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if st.getCalls != 0 {
		t.Errorf("store was read (%d GetAgentSession calls) before authorization refused the caller", st.getCalls)
	}
}

func TestSSHResolverAuthorizeRunsBeforeIDValidation(t *testing.T) {
	st := newFakeStore()
	// A denied caller with a MALFORMED username still gets Forbidden (403 before
	// 400), matching apps.ResolveSSHSession and the repo-wide ordering rule.
	_, err := sshResolver(map[string]bool{}, st).ResolveSSHSession(caller("mallory"), "not-an-id")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden (authorize precedes id validation)", err)
	}
}

func TestSSHResolverDeadPhaseNotAttachable(t *testing.T) {
	for _, phase := range []string{PhaseCanceled, PhaseCanceling} {
		st := newFakeStore()
		id := liveSession(st, "tea-a", "os-dead", phase)
		_, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("alice"), id)
		if !errors.Is(err, core.ErrConflict) {
			t.Errorf("phase %s: err = %v, want ErrConflict (no live sandbox)", phase, err)
		}
	}
}

// A finished session's sandbox waiting out its idle close, its turn's or the
// one resume started, opens in an editor (ADR054 D8); once it is closed, or
// the session archived, it does not.
func TestSSHResolverFinishedSessionOpensWhileItsSandboxIsUp(t *testing.T) {
	for _, phase := range []string{PhaseCompleted, PhaseFailed} {
		st := newFakeStore()
		id := liveSession(st, "tea-a", "os-idle", phase)
		r := sshResolver(map[string]bool{"alice": true}, st)
		if target, err := r.ResolveSSHSession(caller("alice"), id); err != nil || target.PodName != "os-idle-0" {
			t.Errorf("phase %s with its sandbox up: target=%+v err=%v, want os-idle-0", phase, target, err)
		}
		row := st.rows[id]
		row.SandboxID = ""
		st.rows[id] = row
		if _, err := r.ResolveSSHSession(caller("alice"), id); !isCode(err, "AGENT_SESSION_NOT_ATTACHABLE") {
			t.Errorf("phase %s after its sandbox closed: err = %v, want AGENT_SESSION_NOT_ATTACHABLE", phase, err)
		}
		row.SandboxID, row.ArchivedAt = "os-idle", new(time.Now())
		st.rows[id] = row
		if _, err := r.ResolveSSHSession(caller("alice"), id); !isCode(err, "AGENT_SESSION_NOT_ATTACHABLE") {
			t.Errorf("phase %s archived: err = %v, want AGENT_SESSION_NOT_ATTACHABLE", phase, err)
		}
	}
}

func TestSSHResolverEmptySandboxNotAttachable(t *testing.T) {
	st := newFakeStore()
	id := liveSession(st, "tea-a", "", PhaseCreating) // creating, sandbox not yet assigned
	_, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("alice"), id)
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict (no sandbox id yet)", err)
	}
}

func TestSSHResolverMissingSessionNotFound(t *testing.T) {
	st := newFakeStore()
	id := ids.New(ids.AgentSession) // authorized subject, but no row
	_, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("alice"), id)
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// The refusal of a session with no sandbox to open is the banner an SSH client
// prints (w10/043), so each phase says what is wrong and what, if anything,
// brings the sandbox back.
func TestSSHRefusalSaysWhatBringsTheSandboxBack(t *testing.T) {
	archived := new(time.Now())
	for _, tc := range []struct {
		name      string
		row       store.AgentSession
		want      []string
		forbidden []string
	}{
		{"completed, sandbox closed", store.AgentSession{Phase: PhaseCompleted},
			[]string{"is not running", "sandbox has closed", "send the session a message", "connect again"}, []string{"Start the sandbox"}},
		{"failed, sandbox closed", store.AgentSession{Phase: PhaseFailed},
			[]string{"is not running", "send the session a message", "connect again"}, nil},
		{"starting for an editor", store.AgentSession{Phase: PhaseResuming, SandboxID: "os-starting"},
			[]string{"still starting", "Connect again in a moment"}, []string{"message"}},
		{"first turn before its sandbox", store.AgentSession{Phase: PhaseCreating},
			[]string{"still starting"}, []string{"message"}},
		{"canceled", store.AgentSession{Phase: PhaseCanceled, SandboxID: "os-dead"},
			[]string{"was canceled", "takes no more messages"}, []string{"connect again", "Start the sandbox"}},
		{"canceling", store.AgentSession{Phase: PhaseCanceling, SandboxID: "os-dying"},
			[]string{"was canceled"}, []string{"connect again"}},
		{"archived", store.AgentSession{Phase: PhaseCompleted, SandboxID: "os-idle", ArchivedAt: archived},
			[]string{"is archived", "Unarchive it", "connect again"}, []string{"Start the sandbox"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			id := liveSession(st, "tea-a", tc.row.SandboxID, tc.row.Phase)
			row := st.rows[id]
			row.ArchivedAt = tc.row.ArchivedAt
			st.rows[id] = row
			_, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("alice"), id)
			if !isCode(err, core.CodeAgentSessionNotAttachable) {
				t.Fatalf("err = %v, want %s", err, core.CodeAgentSessionNotAttachable)
			}
			for _, want := range append([]string{"agent session " + id}, tc.want...) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message %q lacks %q", err, want)
				}
			}
			for _, unwanted := range tc.forbidden {
				if strings.Contains(err.Error(), unwanted) {
					t.Errorf("message %q says %q, which does not apply", err, unwanted)
				}
			}
		})
	}
}

// The SSH gateway wakes a closed session and waits for a starting one
// (w10/m597) by reading the refusal's params: which of the two it is, and
// the workspace its held login's audit row names. A canceled or archived
// session carries neither, so the gateway keeps its banner.
func TestSSHRefusalTellsTheGatewayWhatAWakeCanDo(t *testing.T) {
	archived := new(time.Now())
	for _, tc := range []struct {
		name    string
		row     store.AgentSession
		sandbox string
	}{
		{"completed, sandbox closed", store.AgentSession{Phase: PhaseCompleted}, SSHSandboxClosed},
		{"failed, sandbox closed", store.AgentSession{Phase: PhaseFailed}, SSHSandboxClosed},
		{"resuming", store.AgentSession{Phase: PhaseResuming, SandboxID: "os-starting"}, SSHSandboxStarting},
		{"creating", store.AgentSession{Phase: PhaseCreating}, SSHSandboxStarting},
		{"redispatching", store.AgentSession{Phase: PhaseRedispatching}, SSHSandboxStarting},
		{"canceled", store.AgentSession{Phase: PhaseCanceled}, ""},
		{"canceling", store.AgentSession{Phase: PhaseCanceling, SandboxID: "os-dying"}, ""},
		{"archived", store.AgentSession{Phase: PhaseCompleted, ArchivedAt: archived}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			id := liveSession(st, "tea-a", tc.row.SandboxID, tc.row.Phase)
			row := st.rows[id]
			row.ArchivedAt = tc.row.ArchivedAt
			st.rows[id] = row
			_, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("alice"), id)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != core.CodeAgentSessionNotAttachable {
				t.Fatalf("err = %v, want %s", err, core.CodeAgentSessionNotAttachable)
			}
			sandbox, _ := coded.Params[SSHSandboxParam].(string)
			workspace, _ := coded.Params[SSHWorkspaceParam].(string)
			if sandbox != tc.sandbox {
				t.Errorf("sandbox param = %q, want %q", sandbox, tc.sandbox)
			}
			if wantWorkspace := map[bool]string{true: "tea-a", false: ""}[tc.sandbox != ""]; workspace != wantWorkspace {
				t.Errorf("workspace param = %q, want %q", workspace, wantWorkspace)
			}
		})
	}
}

// --- sshAddress presenter (ADR054 D5) ---------------------------------------

func viewFor(t *testing.T, host string, record store.AgentSession) View {
	t.Helper()
	svc := &Service{SSHHost: host}
	v, err := svc.toView(record)
	if err != nil {
		t.Fatalf("toView: %v", err)
	}
	return v
}

func liveRecord() store.AgentSession {
	return store.AgentSession{
		ID: "ags-d9example0000000000", WorkspaceID: "tea-a", SandboxID: "os-x",
		Phase: PhaseRunning, AgentConfig: []byte(`{}`), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
}

func TestSSHAddressPresentWhenLiveAndHostSet(t *testing.T) {
	v := viewFor(t, "ssh.bex.co", liveRecord())
	if v.SSHAddress != "ags-d9example0000000000@ssh.bex.co" {
		t.Fatalf("sshAddress = %q", v.SSHAddress)
	}
}

func TestSSHAddressAbsentWhenHostUnset(t *testing.T) {
	if v := viewFor(t, "", liveRecord()); v.SSHAddress != "" {
		t.Fatalf("sshAddress = %q, want empty when BEX_SSH_HOST unset", v.SSHAddress)
	}
}

func TestSSHAddressAbsentWhenHostMalformed(t *testing.T) {
	if v := viewFor(t, "not a host", liveRecord()); v.SSHAddress != "" {
		t.Fatalf("sshAddress = %q, want empty for a non-DNS host", v.SSHAddress)
	}
}

func TestSSHAddressFollowsTheSandboxOfAFinishedSession(t *testing.T) {
	finished := liveRecord()
	finished.Phase = PhaseCompleted
	if v := viewFor(t, "ssh.bex.co", finished); v.SSHAddress != "ags-d9example0000000000@ssh.bex.co" {
		t.Errorf("sshAddress = %q, want the address while a completed session's sandbox is up", v.SSHAddress)
	}
	for name, record := range map[string]store.AgentSession{"canceled": {Phase: PhaseCanceled}, "archived": {Phase: PhaseCompleted, ArchivedAt: new(time.Now())}} {
		gone := liveRecord()
		gone.Phase, gone.ArchivedAt = record.Phase, record.ArchivedAt
		if v := viewFor(t, "ssh.bex.co", gone); v.SSHAddress != "" {
			t.Errorf("sshAddress = %q, want empty for a %s session", v.SSHAddress, name)
		}
	}
}

// An SSH login opens a sandbox that is up, waits for one that is starting
// and wakes one that closed (w10/m597), so the address shows whenever the
// session is neither canceled nor archived, sandbox or not, as Devin offers
// SSH into a sleeping session.
func TestSSHAddressShowsWheneverALoginGetsAShell(t *testing.T) {
	for _, phase := range []string{PhaseCreating, PhaseRunning, PhaseResuming, PhaseRedispatching, PhaseCompleted, PhaseFailed} {
		sandboxless := liveRecord()
		sandboxless.SandboxID, sandboxless.Phase = "", phase
		if v := viewFor(t, "ssh.bex.co", sandboxless); v.SSHAddress != "ags-d9example0000000000@ssh.bex.co" {
			t.Errorf("%s with no sandbox: sshAddress = %q, want the address a login wakes or waits on", phase, v.SSHAddress)
		}
	}
	for _, phase := range []string{PhaseCanceling, PhaseCanceled} {
		gone := liveRecord()
		gone.SandboxID, gone.Phase = "", phase
		if v := viewFor(t, "ssh.bex.co", gone); v.SSHAddress != "" {
			t.Errorf("%s: sshAddress = %q, want none", phase, v.SSHAddress)
		}
	}
	archived := liveRecord()
	archived.SandboxID, archived.Phase, archived.ArchivedAt = "", PhaseCompleted, new(time.Now())
	if v := viewFor(t, "ssh.bex.co", archived); v.SSHAddress != "" {
		t.Errorf("archived: sshAddress = %q, want none", v.SSHAddress)
	}
}
