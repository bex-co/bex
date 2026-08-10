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
	for _, phase := range []string{PhaseCreating, PhaseRunning, PhaseResuming, PhaseRedispatching} {
		st := newFakeStore()
		id := liveSession(st, "tea-a", "os-live", phase)
		if _, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("alice"), id); err != nil {
			t.Errorf("phase %s should resolve, got %v", phase, err)
		}
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
	for _, phase := range []string{PhaseCompleted, PhaseFailed, PhaseCanceled, PhaseCanceling} {
		st := newFakeStore()
		id := liveSession(st, "tea-a", "os-dead", phase)
		_, err := sshResolver(map[string]bool{"alice": true}, st).ResolveSSHSession(caller("alice"), id)
		if !errors.Is(err, core.ErrConflict) {
			t.Errorf("phase %s: err = %v, want ErrConflict (no live sandbox)", phase, err)
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

func TestSSHAddressAbsentWhenTerminalOrSandboxless(t *testing.T) {
	terminal := liveRecord()
	terminal.Phase = PhaseCompleted
	if v := viewFor(t, "ssh.bex.co", terminal); v.SSHAddress != "" {
		t.Errorf("sshAddress = %q, want empty for a completed session", v.SSHAddress)
	}
	sandboxless := liveRecord()
	sandboxless.SandboxID = ""
	if v := viewFor(t, "ssh.bex.co", sandboxless); v.SSHAddress != "" {
		t.Errorf("sshAddress = %q, want empty when no sandbox id", v.SSHAddress)
	}
}
