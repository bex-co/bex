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

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// AttachRevalidator is the gateway-side redemption re-check for agent-session
// attach tickets (codex-security round-6 #11). bex-api checks authorization,
// lifecycle, and billing only when MINTING a ticket; without this, a holder
// whose membership was revoked — or whose session was canceled — right after
// mint could still redeem for the remaining TTL+skew window. It mirrors the
// web shell's redemption-time reauthorization: re-run the SAME relation the
// mint used (can_operate for read, can_create for turn) as the ticket's
// subject, fresh (bypassing the positive decision cache — this guards a
// privilege exercise, the round-5 finding-4 class), and re-require a live
// phase for turns (the round-5 finding-13 gate). Like SSHResolver it holds
// only the authorization kernel and the SELECT-only session reader.
type AttachRevalidator struct {
	*core.Base
	Store SSHSessionReader
}

// RevalidateAttach re-authorizes a verified ticket at redemption. subject and
// sessionID come from the verified claims, never the request. Any error means
// refuse the attach.
func (r *AttachRevalidator) RevalidateAttach(ctx context.Context, subject, sessionID, action string) error {
	ctx = core.WithIdentity(ctx, core.Identity{Subject: subject, Method: "agent-attach"})
	relation := core.RelCanOperate
	if action == agentsessionticket.ActionTurn {
		relation = core.RelCanCreate
	}
	if err := r.AuthorizeFreshOn(ctx, relation, sessionObject(sessionID)); err != nil {
		return err
	}
	if action != agentsessionticket.ActionTurn {
		// Read tickets replay terminal sessions by design — no phase gate.
		return nil
	}
	record, err := r.Store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return mapStoreError(sessionID, err)
	}
	// ADR065 D1: an archive landing inside the ticket's TTL window refuses the
	// turn at redemption too, mirroring the mint-time gate.
	if record.ArchivedAt != nil {
		return errArchived(record.Phase)
	}
	if !liveSandboxPhase(record.Phase) {
		return core.NewConflictError("AGENT_SESSION_NOT_LIVE",
			"agent session is not accepting live turns", map[string]any{"phase": record.Phase})
	}
	return nil
}
