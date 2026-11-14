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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A message sent to a session whose turn is running is queued, not refused
// (ADR047 D12, Devin parity), and runs as its own steering turn once the
// session's turn settles (message_queue.go). Stop turn, Interrupt, ends only
// the running turn: the session settles as completed with the turn flagged
// interrupted, so it stays steerable and its queue keeps its place.

const (
	// maxQueuedMessages is how many messages one session may hold pending.
	maxQueuedMessages = 10

	codeQueueFull             = "AGENT_SESSION_QUEUE_FULL"
	codeQueueInlineSecrets    = "AGENT_SESSION_QUEUE_INLINE_SECRETS"
	codeQueuedMessageNotFound = "AGENT_QUEUED_MESSAGE_NOT_FOUND"
	codeQueuedMessageGone     = "AGENT_QUEUED_MESSAGE_DISPATCHED"
	codeNoTurnRunning         = "AGENT_SESSION_NO_TURN_RUNNING"
	codeTurnStarting          = "AGENT_SESSION_TURN_STARTING"
)

// QueuedMessage is one message waiting to run as a turn of its session.
type QueuedMessage struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	// Position is its 1-based place in the queue: 1 runs next.
	Position int    `json:"position"`
	Prompt   string `json:"prompt"`
	// Author is the member the turn runs as: who queued the message, or who
	// last edited it.
	Author          string            `json:"author"`
	AttachmentIDs   []string          `json:"attachmentIds,omitempty"`
	SecretNames     []string          `json:"secretNames,omitempty"`
	Skills          []SkillInvocation `json:"skills,omitempty"`
	EgressAllowlist []string          `json:"egressAllowlist,omitempty"`
	// HasOutputSchema says the turn will be held to a structured-output contract.
	HasOutputSchema bool `json:"hasOutputSchema,omitempty"`
	// BlockedReason is why the last attempt to run it failed (a spend ceiling,
	// the live-sandbox limit); the next attempt follows within seconds.
	BlockedReason string `json:"blockedReason,omitempty"`
	// State is pending, dispatched (Turn started) or dropped (DroppedReason).
	State         string    `json:"state"`
	Turn          int       `json:"turn,omitempty"`
	DroppedReason string    `json:"droppedReason,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Queued message states.
const (
	QueuedPending    = "pending"
	QueuedDispatched = "dispatched"
	QueuedDropped    = "dropped"
)

// QueuedMessageList is a session's pending messages, next to run first.
type QueuedMessageList struct {
	QueuedMessages []QueuedMessage `json:"queuedMessages"`
}

// QueueStore is the durable message queue (*store.PGStore).
type QueueStore interface {
	EnqueueAgentSessionMessage(ctx context.Context, m store.QueuedAgentMessage, limit int) (store.QueuedAgentMessage, bool, error)
	ListQueuedAgentMessages(ctx context.Context, sessionID string) ([]store.QueuedAgentMessage, error)
	CountQueuedAgentMessages(ctx context.Context, sessionID string) (int, error)
	CountQueuedAgentMessagesBySession(ctx context.Context, sessionIDs []string) (map[string]int, error)
	EditQueuedAgentMessage(ctx context.Context, sessionID, messageID, prompt, author string) (store.QueuedAgentMessage, error)
	DeleteQueuedAgentMessage(ctx context.Context, sessionID, messageID string) error
	MoveQueuedAgentMessage(ctx context.Context, sessionID, messageID string, index int) ([]store.QueuedAgentMessage, error)
	BeginQueuedAgentSessionTurn(ctx context.Context, messageID, id, prompt, deliveryMode, phase, status string, attachments store.AgentAttachmentBinding, output store.AgentTurnOutput, secretNames []string, skills []store.TurnSkill) (store.AgentSession, error)
}

// queuedRequest is the part of a validated steer request a queued message
// keeps besides its prompt. An inline secret's value is never in it: it would
// sit in the control-plane database until the message ran.
type queuedRequest struct {
	EgressAllowlist []string          `json:"egressAllowlist,omitempty"`
	AttachmentIDs   []string          `json:"attachmentIds,omitempty"`
	OutputSchema    map[string]any    `json:"outputSchema,omitempty"`
	OutputRequired  *bool             `json:"outputRequired,omitempty"`
	SecretNames     []string          `json:"secretNames,omitempty"`
	Skills          []SkillInvocation `json:"skills,omitempty"`
}

func queuedRequestOf(req SteerRequest) queuedRequest {
	return queuedRequest{EgressAllowlist: req.EgressAllowlist, AttachmentIDs: req.AttachmentIDs, OutputSchema: req.OutputSchema,
		OutputRequired: req.OutputRequired, SecretNames: req.SecretNames, Skills: req.Skills}
}

// steerRequestOf is the steer a queued message stands for, as the dispatcher
// sends it.
func steerRequestOf(m store.QueuedAgentMessage) (SteerRequest, error) {
	var stored queuedRequest
	if len(m.Request) > 0 {
		if err := json.Unmarshal(m.Request, &stored); err != nil {
			return SteerRequest{}, err
		}
	}
	return SteerRequest{SessionID: m.SessionID, Prompt: m.Prompt, EgressAllowlist: stored.EgressAllowlist, AttachmentIDs: stored.AttachmentIDs,
		OutputSchema: stored.OutputSchema, OutputRequired: stored.OutputRequired,
		SessionSecretsInput: SessionSecretsInput{SecretNames: stored.SecretNames}, Skills: stored.Skills,
		queuedMessageID: m.ID}, nil
}

func queuedMessageOf(row store.QueuedAgentMessage) QueuedMessage {
	m := QueuedMessage{ID: row.ID, SessionID: row.SessionID, Position: row.Index, Prompt: row.Prompt, Author: row.Author,
		BlockedReason: row.BlockedReason, State: QueuedPending, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	var stored queuedRequest
	if len(row.Request) > 0 && json.Unmarshal(row.Request, &stored) == nil {
		m.AttachmentIDs, m.SecretNames, m.Skills, m.EgressAllowlist = stored.AttachmentIDs, stored.SecretNames, stored.Skills, stored.EgressAllowlist
		m.HasOutputSchema = len(stored.OutputSchema) > 0
	}
	switch {
	case row.DeliveredAt != nil:
		m.State = QueuedDispatched
		if row.DeliveredTurn != nil {
			m.Turn = *row.DeliveredTurn
		}
	case row.DroppedReason != "":
		m.State, m.DroppedReason = QueuedDropped, row.DroppedReason
	}
	return m
}

func queuedMessagesOf(rows []store.QueuedAgentMessage) []QueuedMessage {
	out := make([]QueuedMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, queuedMessageOf(row))
	}
	return out
}

// queueRequestHash digests what an idempotency key was first used for, so the
// same key with another message is refused.
func queueRequestHash(prompt string, req queuedRequest) string {
	body, _ := json.Marshal(struct {
		Prompt  string        `json:"p"`
		Request queuedRequest `json:"r"`
	}{prompt, req})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// enqueueSteer queues a validated steer behind the session's turn and the
// messages already waiting. The returned View is the session as it stands,
// with Queued set to the message.
func (s *Service) enqueueSteer(ctx context.Context, record store.AgentSession, req SteerRequest) (View, error) {
	if len(req.Secrets) > 0 {
		return View{}, core.NewConflictError(codeQueueInlineSecrets,
			"a message that carries inline secrets cannot wait in the queue: their values are never stored. Send it when the session is idle, or name workspace secrets in secretNames",
			map[string]any{"phase": record.Phase})
	}
	author := core.SubjectFrom(ctx)
	if author == "" {
		return View{}, core.ErrForbidden
	}
	stored := queuedRequestOf(req)
	request, err := json.Marshal(stored)
	if err != nil {
		return View{}, err
	}
	prompt := strings.TrimSpace(req.Prompt)
	row, _, err := s.Queue.EnqueueAgentSessionMessage(ctx, store.QueuedAgentMessage{SessionID: record.ID, Author: author, Prompt: prompt, Request: request,
		IdempotencyKey: req.IdempotencyKey, RequestHash: queueRequestHash(prompt, stored)}, maxQueuedMessages)
	if err != nil {
		return View{}, mapQueueError(record, err)
	}
	view, err := s.toView(record)
	if err != nil {
		return View{}, err
	}
	message := queuedMessageOf(row)
	view.Queued = &message
	view.PendingMessages = message.Position
	return view, nil
}

// mapQueueError is a queue refusal as its coded error; the refusals it does
// not name (the message already dispatched among them) are mapStoreError's.
func mapQueueError(record store.AgentSession, err error) error {
	switch {
	case errors.Is(err, store.ErrAgentSessionQueueFull):
		return core.NewConflictError(codeQueueFull, "the session already has the most messages it can queue; wait for one to run or delete one",
			map[string]any{"id": record.ID, "max": maxQueuedMessages})
	case errors.Is(err, store.ErrQueuedMessageKeyReused):
		return core.NewConflictError(codeIdempotencyKeyReused, "this idempotency key was already used for a different queued message", map[string]any{"id": record.ID})
	case errors.Is(err, store.ErrAgentSessionNotSteerable):
		if record.ArchivedAt != nil {
			return errArchived(record.Phase)
		}
		return errNotSteerable(record.Phase)
	case errors.Is(err, store.ErrQueuedMessageAttachmentsOwned):
		return core.NewForbiddenError("AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED", "this message carries its sender's attachments, so only the sender can edit it",
			map[string]any{"sessionId": record.ID})
	case errors.Is(err, store.ErrNotFound):
		return core.NewNotFoundError(codeQueuedMessageNotFound, "queued message not found", map[string]any{"sessionId": record.ID})
	}
	return mapStoreError(record.ID, err)
}

func validateQueuedMessageID(value string) error {
	if kind, ok := ids.KindOf(value); !ok || kind != ids.AgentQueuedMessage {
		return core.NewBadRequestError("AGENT_QUEUED_MESSAGE_ID_INVALID", "invalid queued message id", map[string]any{"id": value})
	}
	return nil
}

// queueRecord authorizes relation on the session and loads it, for the verbs
// that manage its queue: nothing is revealed about a session the caller cannot
// reach, and an archived or canceled session has no queue to manage.
func (s *Service) queueRecord(ctx context.Context, relation, sessionID string) (store.AgentSession, error) {
	record, err := s.operableRecord(ctx, relation, sessionID)
	if err != nil {
		return store.AgentSession{}, err
	}
	if s.Queue == nil {
		return store.AgentSession{}, core.ErrAgentSessionsUnavailable
	}
	return record, nil
}

// QueuedMessages lists the session's pending messages, the next to run first.
// Anyone who may operate the session may read them.
func (s *Service) QueuedMessages(ctx context.Context, sessionID string) (QueuedMessageList, error) {
	record, err := s.queueRecord(ctx, core.RelCanOperate, sessionID)
	if err != nil {
		return QueuedMessageList{}, err
	}
	rows, err := s.Queue.ListQueuedAgentMessages(ctx, record.ID)
	if err != nil {
		return QueuedMessageList{}, err
	}
	return QueuedMessageList{QueuedMessages: queuedMessagesOf(rows)}, nil
}

// EditQueuedMessage replaces a pending message's prompt. The turn then runs
// as the member who edited it, who must be able to steer the session (the text
// is theirs now), so editing needs the relation a steer needs. Attachments,
// skills and the rest of the request are kept.
func (s *Service) EditQueuedMessage(ctx context.Context, sessionID, messageID, prompt string) (QueuedMessage, error) {
	record, err := s.queueRecord(ctx, core.RelCanCreate, sessionID)
	if err != nil {
		return QueuedMessage{}, err
	}
	if err := validateQueuedMessageID(messageID); err != nil {
		return QueuedMessage{}, err
	}
	prompt, err = validatePrompt(prompt)
	if err != nil {
		return QueuedMessage{}, err
	}
	author := core.SubjectFrom(ctx)
	if author == "" {
		return QueuedMessage{}, core.ErrForbidden
	}
	row, err := s.Queue.EditQueuedAgentMessage(ctx, record.ID, messageID, prompt, author)
	if err != nil {
		return QueuedMessage{}, mapQueueError(record, err)
	}
	return queuedMessageOf(row), nil
}

// DeleteQueuedMessage removes a pending message so it never runs.
func (s *Service) DeleteQueuedMessage(ctx context.Context, sessionID, messageID string) error {
	record, err := s.queueRecord(ctx, core.RelCanOperate, sessionID)
	if err != nil {
		return err
	}
	if err := validateQueuedMessageID(messageID); err != nil {
		return err
	}
	if err := s.Queue.DeleteQueuedAgentMessage(ctx, record.ID, messageID); err != nil {
		return mapQueueError(record, err)
	}
	return nil
}

// MoveQueuedMessage puts a pending message at the 1-based position in the
// queue (1 runs next; past the end is last) and returns the queue. "Send now"
// is Interrupt, then a move to position 1.
func (s *Service) MoveQueuedMessage(ctx context.Context, sessionID, messageID string, position int) (QueuedMessageList, error) {
	record, err := s.queueRecord(ctx, core.RelCanOperate, sessionID)
	if err != nil {
		return QueuedMessageList{}, err
	}
	if err := validateQueuedMessageID(messageID); err != nil {
		return QueuedMessageList{}, err
	}
	if position < 1 {
		return QueuedMessageList{}, core.NewBadRequestError("AGENT_SESSION_INPUT_INVALID", "position is a place in the queue, 1 or more", map[string]any{"field": "position"})
	}
	rows, err := s.Queue.MoveQueuedAgentMessage(ctx, record.ID, messageID, position)
	if err != nil {
		return QueuedMessageList{}, mapQueueError(record, err)
	}
	return QueuedMessageList{QueuedMessages: queuedMessagesOf(rows)}, nil
}

// Interrupt ends the session's running turn and nothing else (ADR047 D12,
// Devin's stop): the driver is asked, through the sandbox, to stop the agent;
// the Completer then settles the turn as completed and interrupted. The
// session stays steerable, what the agent already streamed stays in the
// transcript, commits it already pushed stay on the branch, and queued
// messages keep their place. The call returns once the request is delivered
// and the session is still working; the turn settles within a Completer tick.
// Cancel, which ends the session, is unchanged.
func (s *Service) Interrupt(ctx context.Context, sessionID string) (View, error) {
	record, err := s.operableRecord(ctx, core.RelCanOperate, sessionID)
	if err != nil {
		return View{}, err
	}
	if record.ArchivedAt != nil {
		return View{}, errArchived(record.Phase)
	}
	switch {
	case record.Phase == PhaseCanceled || record.Phase == PhaseCanceling:
		return View{}, errNotSteerable(record.Phase)
	case !liveSandboxPhase(record.Phase):
		return View{}, core.NewConflictError(codeNoTurnRunning, "no turn is running; the session is idle", map[string]any{"phase": record.Phase})
	case record.Phase != PhaseRunning || record.SandboxID == "":
		// The turn is accepted but its sandbox is not up: there is no agent to stop yet.
		return View{}, core.NewConflictError(codeTurnStarting, "the turn is still starting; stop it once it is running", map[string]any{"phase": record.Phase})
	}
	// The sentinel names the turn number the driver was started with (BEX_AGENT_TURN).
	if err := s.Sandbox.InterruptSessionTurn(ctx, record.WorkspaceID, record.ID, record.SandboxID, driverTurn(record)); err != nil {
		return View{}, err
	}
	return s.toView(record)
}

// QueueRunnable reports whether a queued message could start a turn, so the
// dispatcher runs only where a steer could succeed.
func (s *Service) QueueRunnable() bool {
	return s != nil && s.Base != nil && s.Queue != nil && s.enabled() && s.modelProxyEnabled() && s.ticketEnabled()
}

// listQueued lists the session's pending messages, next first; nil when the
// read fails, which is left out rather than failing the read of the session.
func (s *Service) listQueued(ctx context.Context, sessionID string) []QueuedMessage {
	rows, err := s.Queue.ListQueuedAgentMessages(ctx, sessionID)
	if err != nil {
		log.Printf("agent-session read %s: queued messages: %v", sessionID, err)
		return nil
	}
	return queuedMessagesOf(rows)
}

// fillQueueCount sets how many messages wait on a single-session read: one
// count, since the messages carry prompts of up to 100 KB that most reads never
// show. A surface that returns them lists them with fillQueue (REST, MCP) or
// View.queued (GraphQL, only when the field is selected).
func (s *Service) fillQueueCount(ctx context.Context, view *View) {
	if s.Queue == nil {
		return
	}
	pending, err := s.Queue.CountQueuedAgentMessages(ctx, view.ID)
	if err != nil {
		log.Printf("agent-session read %s: queued message count: %v", view.ID, err)
		return
	}
	view.PendingMessages = pending
	id := view.ID
	view.listQueue = func(ctx context.Context) []QueuedMessage { return s.listQueued(ctx, id) }
}

// fillQueue lists the session's pending messages on a view a single-session
// read returned.
func (s *Service) fillQueue(ctx context.Context, view *View) {
	if s.Queue == nil {
		return
	}
	if messages := s.listQueued(ctx, view.ID); messages != nil {
		view.QueuedMessages = messages
		view.PendingMessages = len(messages)
	}
}

// fillPendingMessages sets how many messages wait on each listed session, in
// one read, so a list page can show them.
func (s *Service) fillPendingMessages(ctx context.Context, views []View) {
	if s.Queue == nil || len(views) == 0 {
		return
	}
	sessionIDs := make([]string, 0, len(views))
	for _, v := range views {
		sessionIDs = append(sessionIDs, v.ID)
	}
	counts, err := s.Queue.CountQueuedAgentMessagesBySession(ctx, sessionIDs)
	if err != nil {
		log.Printf("agent-session list: queued message counts: %v", err)
		return
	}
	for i := range views {
		views[i].PendingMessages = counts[views[i].ID]
	}
}
