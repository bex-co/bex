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

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// SSHSessionReader is the resolver's minimal database dependency: read one
// session row by id. *store.PGStore satisfies it. Deliberately narrower than the
// feature Store — the gateway resolver needs no lifecycle or transcript surface,
// matching the SELECT-only agent_sessions grant (ADR054 D7, dbrole.sql).
type SSHSessionReader interface {
	GetAgentSession(ctx context.Context, id string) (store.AgentSession, error)
}

// SSHResolver resolves an `ags-<xid>@ssh.bex.co` SSH username to the session's
// live sandbox pod (ADR054 D1). It is the gateway-side counterpart to
// AttachTicket: the SAME authorize seam (can_view_sensitive on the agent_session
// object) and the SAME deterministic pod derivation (<sandbox_id>-0 in
// <workspace_id>-sandbox), but driven by the caller's proven SSH-key identity
// instead of a minted ticket. It satisfies sshgateway.TargetResolver, so the
// gateway's composite resolver can route ags-… usernames to it while srv-…
// usernames stay on apps.Service.ResolveSSHSession, byte-identically.
//
// It holds only the authorization kernel and a read-only session reader; it can
// neither mint tickets nor mutate a session, so a gateway process compromise
// gains nothing beyond a can_view_sensitive-gated shell it already had.
type SSHResolver struct {
	*core.Base
	Store SSHSessionReader
}

// ResolveSSHSession authorizes can_view_sensitive on the agent_session object,
// then — only if the session's sandbox is up (openableSandbox) — returns its
// pod as a sandbox-marked target. Authorization runs FIRST (403 before 400/404), mirroring
// apps.ResolveSSHSession and the repository-wide rule that a refusal never leaks
// existence. The relation is then re-asserted UNCACHED (codex round-7 F7): the
// gateway converts a verified SSH key into a new hours-long pods/exec session,
// so a member revoked on another replica within PositiveTTL must not open one
// off this gateway's stale cached positive (the attach-revalidator pattern,
// ADR057 #11). The gateway has already attached the SSH-key identity to ctx.
func (r *SSHResolver) ResolveSSHSession(ctx context.Context, username string) (apps.SSHInstanceTarget, error) {
	id := username
	if err := r.AuthorizeOn(ctx, core.RelCanViewSensitive, sessionObject(id)); err != nil {
		return apps.SSHInstanceTarget{}, err
	}
	if err := r.AuthorizeFreshOn(ctx, core.RelCanViewSensitive, sessionObject(id)); err != nil {
		return apps.SSHInstanceTarget{}, err
	}
	if err := validateSessionID(id); err != nil {
		return apps.SSHInstanceTarget{}, err
	}
	record, err := r.Store.GetAgentSession(ctx, id)
	if err != nil {
		return apps.SSHInstanceTarget{}, mapStoreError(id, err)
	}
	if !openableSandbox(record) {
		return apps.SSHInstanceTarget{}, errNoSandboxToOpen(record)
	}
	return apps.SSHInstanceTarget{
		ID:        record.ID,
		ServiceID: record.ID,
		OwnerID:   record.WorkspaceID,
		Namespace: record.WorkspaceID + "-sandbox",
		PodName:   record.SandboxID + "-0",
		Container: sandboxexec.SandboxContainer,
		Sandbox:   true,
	}, nil
}

// The SSH gateway reads what an SSH login into a session with no sandbox to
// open can do from its refusal's params (w10/m597): SSHSandboxParam is
// SSHSandboxClosed for a finished session the caller may wake, or
// SSHSandboxStarting for one whose sandbox is on its way, and
// SSHWorkspaceParam names the session's workspace for the held login's audit
// row. A canceled or archived session carries neither.
const (
	SSHSandboxParam    = "sandbox"
	SSHWorkspaceParam  = "workspaceId"
	SSHSandboxClosed   = "closed"
	SSHSandboxStarting = "starting"
	// SSHWakeTimeout bounds how long the gateway holds a login for a sandbox
	// that is starting: the editor start's own bound, past which the
	// Completer has settled the session either way.
	SSHWakeTimeout = editorStartBound
)

// SSHNoSandbox reads err as the resolver's refusal of a session with no
// sandbox to open (errNoSandboxToOpen): the refusal itself, and the sandbox
// state and workspace its params name, both "" for a canceled or archived
// session. refusal is nil for any other error. The gateway's held SSH login
// (w10/m597) and its waiting Terminal (w10/m615) both read it.
func SSHNoSandbox(err error) (refusal *core.CodedError, state, workspace string) {
	if !errors.As(err, &refusal) || refusal.Code != core.CodeAgentSessionNotAttachable {
		return nil, "", ""
	}
	state, _ = refusal.Params[SSHSandboxParam].(string)
	workspace, _ = refusal.Params[SSHWorkspaceParam].(string)
	return refusal, state, workspace
}

// errNoSandboxToOpen is the refusal of a session that openableSandbox turns
// down. The gateway wakes a closed session for a caller who may start it,
// and waits for one that is starting (w10/m597), as SSH into a sleeping
// Devin session wakes the machine and completes the request
// (docs.devin.ai/release-notes/2026, 2026-09-25). Otherwise it prints the
// message in the SSH auth banner (w10/043), so the message says what is
// wrong and what brings the sandbox back. It is reached only after
// can_view_sensitive passed, so it says no more than the caller's own read
// of the session would.
func errNoSandboxToOpen(record store.AgentSession) error {
	session := "agent session " + record.ID
	params := map[string]any{"phase": record.Phase}
	var msg string
	switch {
	case record.Phase == PhaseCanceling || record.Phase == PhaseCanceled:
		msg = session + " was canceled, so it has no sandbox to open and takes no more messages"
	case record.ArchivedAt != nil:
		msg = session + " is archived, and its sandbox has closed. Unarchive it on its page in the dashboard, then connect again"
	case steerable(record.Phase, false):
		msg = session + " is not running, and its sandbox has closed. To start it again, send the session a message, then connect again"
		params[SSHSandboxParam], params[SSHWorkspaceParam] = SSHSandboxClosed, record.WorkspaceID
	case liveSandboxPhase(record.Phase):
		msg = session + " is still starting its sandbox. Connect again in a moment"
		params[SSHSandboxParam], params[SSHWorkspaceParam] = SSHSandboxStarting, record.WorkspaceID
	default:
		msg = session + " has no sandbox to open now"
	}
	return core.NewConflictError(core.CodeAgentSessionNotAttachable, msg, params)
}

// sshReachable reports whether an SSH login into the session gets a shell,
// at once or once its sandbox is up (w10/m597): a turn's live phase or a
// start under way waits for the sandbox, and a finished, unarchived session
// is woken. sshAddress shows exactly then. A canceled or archived session
// has nothing to open.
func sshReachable(record store.AgentSession) bool {
	return liveSandboxPhase(record.Phase) || steerable(record.Phase, record.ArchivedAt != nil)
}

// openableSandbox reports whether an editor may open the session's sandbox now
// (ADR054 D1, D8): the session names one and is either in a turn's live
// phase or finished (completed or failed) and not archived, its sandbox
// waiting out the idle close, whether its turn left it or resume started it.
// A sandbox resume is still starting (resuming) opens once its checkout is
// ready and the session settles. A canceling or canceled session's pod is
// gone or going, and an archived finished session's is reclaimed at the next
// reaper tick, so opening a shell into either fails closed at handshake.
func openableSandbox(record store.AgentSession) bool {
	if record.SandboxID == "" || record.Phase == PhaseResuming {
		return false
	}
	return liveSandboxPhase(record.Phase) || steerable(record.Phase, record.ArchivedAt != nil)
}

// liveSandboxPhase reports whether a session's phase implies a sandbox running
// or starting: a turn's, or the one resume starts for an editor (resuming). It
// is exactly the set the Completer's status watch treats as active
// (activePhases).
func liveSandboxPhase(phase string) bool {
	switch phase {
	case PhaseCreating, PhaseRunning, PhaseResuming, PhaseRedispatching:
		return true
	}
	return false
}
