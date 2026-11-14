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
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The queue dispatcher (ADR047 D12) is the sibling of the D11 feedback
// worker: every replica polls; a session whose turn has settled (completed or
// failed, not archived) and whose queue has a head nobody holds is claimed
// with FOR UPDATE SKIP LOCKED, the head is leased, and it is delivered through
// Service.Steer as the member who queued it, so the spend and live-sandbox
// limits, the branch rules and the prompt-history quota apply when it runs,
// and its author must still be allowed to steer. The turn's own transaction
// records the delivery, so a crash between the turn and its record cannot run a
// message twice. One message starts per turn: the next waits for it to settle.
//
// A refusal at dispatch ends one of two ways. It is FINAL, and the message is
// dropped with the refusal's text as its reason, when no later attempt can
// start it: the author lost can_create (a real authorization denial), the
// session is gone, archived, canceled or otherwise not steerable, the message
// itself is refused (its prompt, attachments, schema or egress list), or the
// session's prompt history is full (AGENT_SESSION_PROMPT_QUOTA). It is BLOCKED,
// and the message is released with the refusal's text as blockedReason so the
// queue shows why and the next pass tries again, when the cause is
// configuration or enrollment a person can fix: the workspace has no model key
// for the agent (ModelKeyRequiredCode), the agent is not available there
// (AgentUnavailableCode), the author has no second factor in a workspace that
// requires one (WORKSPACE_2FA_REQUIRED, a 403 but not a lost permission), a
// skill or secret the message names no longer exists, or a limit that clears on
// its own (the live-sandbox cap, the trust tier's session limit, billing). The
// same goes for every refusal not listed as final.
const (
	queuePollInterval = 5 * time.Second
	queueLease        = 2 * time.Minute
	queueBatch        = 20
	// maxBlockedReason bounds the reason shown on a blocked message.
	maxBlockedReason = 500

	codePromptQuota = "AGENT_SESSION_PROMPT_QUOTA"
	codeLiveLimit   = "AGENT_SESSION_LIVE_LIMIT"
)

// QueueSessions starts a turn from a queued message (*Service).
type QueueSessions interface {
	Steer(ctx context.Context, req SteerRequest) (View, error)
}

// QueueDispatchStore is the dispatcher's durable seam (*store.PGStore).
type QueueDispatchStore interface {
	ClaimQueuedAgentMessages(ctx context.Context, now time.Time, lease time.Duration, limit int, phases []string) ([]store.QueuedMessageClaim, error)
	ReleaseQueuedAgentMessage(ctx context.Context, messageID, blockedReason string) error
	DropQueuedAgentMessage(ctx context.Context, messageID, reason string) error
	CountAllQueuedAgentMessages(ctx context.Context) (int, error)
}

// MessageQueue delivers queued messages as steering turns.
type MessageQueue struct {
	Sessions QueueSessions
	Store    QueueDispatchStore
	Metrics  *QueueMetrics
	Now      func() time.Time
}

func (q *MessageQueue) now() time.Time {
	if q.Now != nil {
		return q.Now()
	}
	return time.Now()
}

// Run delivers queued messages until cancellation.
func (q *MessageQueue) Run(ctx context.Context) {
	core.Poll(ctx, "agent-session message queue", queuePollInterval, q.RunOnce)
}

// RunOnce leases the head of each settled session's queue and starts its turn.
func (q *MessageQueue) RunOnce(ctx context.Context) error {
	// A pass over an empty queue is the common one: the cheap count sets the
	// gauge and ends it without opening the claim transaction.
	depth, err := q.Store.CountAllQueuedAgentMessages(ctx)
	if err == nil {
		q.Metrics.setDepth(depth)
		if depth == 0 {
			return nil
		}
	}
	claims, err := q.Store.ClaimQueuedAgentMessages(ctx, q.now(), queueLease, queueBatch, []string{PhaseCompleted, PhaseFailed})
	if err != nil {
		return err
	}
	for _, claim := range claims {
		q.deliver(ctx, claim)
	}
	if len(claims) > 0 {
		if depth, err := q.Store.CountAllQueuedAgentMessages(ctx); err == nil {
			q.Metrics.setDepth(depth)
		}
	}
	return nil
}

// deliver steers the session with the head of its queue, as its author, and
// settles the message: started (its turn recorded it), dropped when no later
// attempt can start it, or released, blocked with its reason, to try again.
func (q *MessageQueue) deliver(ctx context.Context, claim store.QueuedMessageClaim) {
	session, message := claim.Session, claim.Message
	req, err := steerRequestOf(message)
	if err != nil {
		q.drop(ctx, message, "its stored request could not be read")
		return
	}
	steerCtx := core.WithWorkspace(core.WithIdentity(ctx, core.Identity{Subject: message.Author, Method: "session"}), session.WorkspaceID)
	view, err := q.Sessions.Steer(steerCtx, req)
	switch {
	case err == nil:
		q.Metrics.observeDispatch(q.now().Sub(latest(session.UpdatedAt, message.CreatedAt)))
		log.Printf("agent-session queue: %s message %s started turn %d", session.ID, message.ID, view.Turns)
	case errors.Is(err, store.ErrQueuedMessageNotPending) || hasCode(err, codeQueuedMessageGone):
		// Dropped or delivered since the claim (a cancel, a delete): nothing to do.
	case hasCode(err, codeTurnInFlight):
		// A turn started since the claim: the next pass waits for it to settle.
		q.release(ctx, message, "")
	case finalQueueRefusal(err):
		q.drop(ctx, message, truncateReason(err.Error()))
	default:
		// A live-sandbox, trust or spend limit, billing, or a transient
		// failure: the message stays queued and says why.
		q.Metrics.observeBlocked(blockedReasonLabel(err))
		q.release(ctx, message, truncateReason(err.Error()))
	}
}

func latest(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func hasCode(err error, code string) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == code
}

// finalQueueRefusal is a refusal no later attempt can turn into a turn (the
// file comment lists them). A refusal a person can fix is not final, though
// it is a 400, 403 or 404 like the final ones.
func finalQueueRefusal(err error) bool {
	if fixableRefusal(err) {
		return false
	}
	return finalSteerRefusal(err) || hasCode(err, codePromptQuota)
}

// fixableRefusal is a refusal whose cause is the workspace's configuration or
// the author's enrollment, not the message or the session.
func fixableRefusal(err error) bool {
	return hasCode(err, ModelKeyRequiredCode) || hasCode(err, AgentUnavailableCode) ||
		hasCode(err, core.CodeWorkspaceTwoFactorRequired) || hasCode(err, codeSkillNotFound) || hasCode(err, codeSecretNotFound)
}

// blockedReasonLabel is the bounded metric label of a blocked dispatch: never
// a workspace, a session or the error's text.
func blockedReasonLabel(err error) string {
	switch {
	case hasCode(err, codeLiveLimit):
		return blockedLiveLimit
	case hasCode(err, TrustLimitCode):
		return blockedTrustLimit
	case hasCode(err, MetaRouteCapacityCode):
		return blockedRouteCapacity
	case errors.Is(err, core.ErrPaymentRequired), errors.Is(err, core.ErrBillingEnforced):
		return blockedBilling
	}
	return blockedOther
}

func truncateReason(text string) string {
	runes := []rune(text)
	if len(runes) <= maxBlockedReason {
		return text
	}
	return string(runes[:maxBlockedReason-1]) + "…"
}

func (q *MessageQueue) drop(ctx context.Context, message store.QueuedAgentMessage, reason string) {
	if err := q.Store.DropQueuedAgentMessage(ctx, message.ID, reason); err != nil {
		log.Printf("agent-session queue: drop %s: %v", message.ID, err)
		return
	}
	q.Metrics.observeDropped()
	log.Printf("agent-session queue: %s message %s dropped: %s", message.SessionID, message.ID, reason)
}

func (q *MessageQueue) release(ctx context.Context, message store.QueuedAgentMessage, blockedReason string) {
	if err := q.Store.ReleaseQueuedAgentMessage(ctx, message.ID, blockedReason); err != nil {
		log.Printf("agent-session queue: release %s: %v", message.ID, err)
	}
}
