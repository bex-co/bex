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

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// sandboxContainer is the agent-session sandbox workload container the gateway
// execs into — the same container sandboxsse targets from a signed exec ticket.
const sandboxContainer = "sandbox"

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
// then — only if a live sandbox exists — returns its pod as a sandbox-marked
// target. Authorization runs FIRST (403 before 400/404), mirroring
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
	if !liveSandboxPhase(record.Phase) || record.SandboxID == "" {
		return apps.SSHInstanceTarget{}, core.NewConflictError("AGENT_SESSION_NOT_ATTACHABLE",
			"agent session has no live sandbox to open", map[string]any{"phase": record.Phase})
	}
	return apps.SSHInstanceTarget{
		ID:        record.ID,
		ServiceID: record.ID,
		OwnerID:   record.WorkspaceID,
		Namespace: record.WorkspaceID + "-sandbox",
		PodName:   record.SandboxID + "-0",
		Container: sandboxContainer,
		Sandbox:   true,
	}, nil
}

// liveSandboxPhase reports whether a session's phase implies a running sandbox
// pod the resolver can reach. It is exactly the set the Completer's status watch
// treats as active (activePhases) — a terminal or canceling session's pod is
// gone or going, so opening a shell into it must fail closed at handshake.
func liveSandboxPhase(phase string) bool {
	switch phase {
	case PhaseCreating, PhaseRunning, PhaseResuming, PhaseRedispatching:
		return true
	}
	return false
}
