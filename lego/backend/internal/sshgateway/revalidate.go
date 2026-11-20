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

package sshgateway

import (
	"context"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
)

// DefaultRevalidateInterval is how often a live gateway stream re-runs its
// authoritative admission checks (codex round-9 #6). The helper itself lives
// in core so live log subscriptions (round 15) share one watchdog.
const DefaultRevalidateInterval = core.DefaultStreamRevalidateInterval

// WithRevalidation is the gateway-facing name of core.WithRevalidation.
func WithRevalidation(parent context.Context, interval time.Duration, check func(context.Context) error) (context.Context, context.CancelFunc) {
	return core.WithRevalidation(parent, interval, check)
}

// Revalidator re-runs a verified ticket's authorization at redemption and on
// every live-stream revalidation tick (round-13 #3). Any error refuses the
// exchange / ends the stream. The claims — never the request — supply the
// subject, workspace, sandbox, and agent-session binding. It lives in the
// kernel so the exec transports (SSE, files) share one revalidator instead
// of importing each other.
type Revalidator interface {
	RevalidateExec(ctx context.Context, claims sandboxexec.Claims) error
}

// ExecRevalidator is the authorization kernel implementing Revalidator: fresh
// can_operate on the ticket's workspace for its subject, plus — when the
// sandbox is agent-session-bound — the session object's stronger
// can_view_sensitive (round-13 #1, the same relation the dedicated ags-… SSH
// path enforces). Platform-internal tickets (the Completer's system sentinel)
// are exempt: their authority is the HMAC itself, minted by trusted system
// loops with no caller identity to re-check. Like agentsessions.AttachRevalidator
// it holds only the kernel — no ticket minting, no store mutation. KeyGrants
// holds a ticket a bex API key minted (its KeyAction) to the key's current
// grant (w9/125); a nil gate refuses every such ticket.
type ExecRevalidator struct {
	*core.Base
	KeyGrants *KeyGrantGate
}

// RevalidateExec re-authorizes a verified exec ticket as its signed subject,
// uncached (a positive decision cached on another replica must not outlive a
// revocation — the round-5 finding-4 class this transport joins late).
// SECURITY (codex round-16 #7 / finding-2): command selection is create-like
// (caller-selected argv reaches pods/exec) so both redemption and live-stream
// revalidation require can_create — not can_operate — matching the mint-time
// gate in sandbox.Service.dialGateway. A demoted contributor who retains
// can_operate must not keep arbitrary exec alive.
func (r *ExecRevalidator) RevalidateExec(ctx context.Context, claims sandboxexec.Claims) error {
	if r == nil || r.Base == nil {
		return nil // authorization kernel not wired: admission-only (pre-round-13)
	}
	if claims.Subject == "" || sandboxexec.PlatformSubject(claims.Subject) {
		return nil
	}
	if claims.Workspace == "" {
		return core.ErrForbidden // a caller ticket always carries its workspace
	}
	ctx = core.WithIdentity(ctx, core.Identity{Subject: claims.Subject, Method: core.MethodSandboxExec})
	if err := r.AuthorizeFreshOn(ctx, core.RelCanCreate, core.WorkspaceObject(claims.Workspace)); err != nil {
		return err
	}
	if claims.AgentSessionID != "" {
		if err := r.AuthorizeFreshOn(ctx, core.RelCanViewSensitive, agentsession.SessionObject(claims.AgentSessionID)); err != nil {
			return err
		}
	}
	return r.KeyGrants.RequireTicket(ctx, claims.Subject, claims.KeyAction)
}
