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

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// The durable queue of messages sent to a session whose turn is running
// (ADR047 D12). A message is one row: pending, then either delivered by the
// turn it started or dropped with a reason. Every write that decides who goes
// next takes the session's own row lock first, so queueing, reordering,
// claiming and starting a turn never interleave: the lock order is the
// session, then its messages.

var (
	// ErrAgentSessionQueueFull is a queue already holding the most pending
	// messages the caller allows.
	ErrAgentSessionQueueFull = fmt.Errorf("agent session message queue is full: %w", ErrConflict)
	// ErrQueuedMessageNotPending is a message that was dispatched, dropped or
	// is being dispatched now, so it can no longer be changed.
	ErrQueuedMessageNotPending = fmt.Errorf("queued message is no longer pending: %w", ErrConflict)
	// ErrQueuedMessageKeyReused is an idempotency key already used for another
	// message of the session.
	ErrQueuedMessageKeyReused = fmt.Errorf("idempotency key was used for a different queued message: %w", ErrConflict)
	// ErrAgentSessionNotSteerable is a session that took a cancel or an
	// archive between the caller's read and the write, so nothing is queued.
	ErrAgentSessionNotSteerable = fmt.Errorf("agent session takes no more messages: %w", ErrConflict)
	// ErrQueuedMessageAttachmentsOwned is an edit by someone other than the
	// sender of a message that carries attachments: they are the sender's
	// uploads, which only the sender's own turn may attach.
	ErrQueuedMessageAttachmentsOwned = fmt.Errorf("queued message carries its sender's attachments: %w", ErrConflict)
)

// QueuedAgentMessage is one message queued for a session.
type QueuedAgentMessage struct {
	ID        string
	SessionID string
	// Index is the message's 1-based place among the session's pending
	// messages, set by the reads that list them; 0 elsewhere.
	Index int
	// Position orders the queue: first to last by ascending value.
	Position int64
	// Author is the bex subject the steer runs as.
	Author string
	Prompt string
	// Request is the rest of the validated steer request, as JSON.
	Request        json.RawMessage
	IdempotencyKey string
	RequestHash    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// ClaimedUntil is when the dispatcher's lease on it runs out; nil when
	// none holds it.
	ClaimedUntil *time.Time
	// BlockedReason is why the last attempt to start its turn failed.
	BlockedReason string
	DeliveredTurn *int
	DeliveredAt   *time.Time
	DroppedReason string
}

// QueuedMessageClaim is a settled session and the head of its queue, leased to
// one caller. Session carries only what delivering the message needs: its id,
// its workspace and when it last changed.
type QueuedMessageClaim struct {
	Session AgentSession
	Message QueuedAgentMessage
}

const queuedMessageColumns = `id, session_id, position, author, prompt, request, coalesce(idempotency_key, ''), request_hash,
	created_at, updated_at, claimed_until, blocked_reason, delivered_turn, delivered_at, coalesce(dropped_reason, '')`

func scanQueuedMessage(row pgx.Row) (QueuedAgentMessage, error) {
	var m QueuedAgentMessage
	err := row.Scan(&m.ID, &m.SessionID, &m.Position, &m.Author, &m.Prompt, &m.Request, &m.IdempotencyKey, &m.RequestHash,
		&m.CreatedAt, &m.UpdatedAt, &m.ClaimedUntil, &m.BlockedReason, &m.DeliveredTurn, &m.DeliveredAt, &m.DroppedReason)
	return m, err
}

// pendingQueuedSQL selects the rows still waiting: not delivered, not dropped.
const pendingQueuedSQL = `delivered_at IS NULL AND dropped_reason IS NULL`

// lockSessionForQueue takes the session's row lock and returns its phase and
// whether it is archived. ErrNotFound when the session does not exist.
func lockSessionForQueue(ctx context.Context, tx pgx.Tx, sessionID string) (phase string, archived bool, err error) {
	var archivedAt *time.Time
	if err := tx.QueryRow(ctx, `SELECT phase, archived_at FROM agent_sessions WHERE id=$1 FOR UPDATE`, sessionID).Scan(&phase, &archivedAt); err != nil {
		return "", false, err
	}
	return phase, archivedAt != nil, nil
}

func listPendingQueued(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, sessionID string, lock bool) ([]QueuedAgentMessage, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	rows, err := collectWith(scanQueuedMessage)(q.Query(ctx, `SELECT `+queuedMessageColumns+`
		FROM agent_session_queued_messages WHERE session_id=$1 AND `+pendingQueuedSQL+`
		ORDER BY position, id`+suffix, sessionID))
	for i := range rows {
		rows[i].Index = i + 1
	}
	return rows, err
}

// EnqueueAgentSessionMessage appends a message to the session's queue and
// returns it with its place. inserted is false when m's idempotency key was
// already used for an identical message, which is returned as it stands; the
// same key with another RequestHash is ErrQueuedMessageKeyReused. A queue
// already holding limit pending messages is ErrAgentSessionQueueFull, and a
// session that is canceled, canceling or archived takes none
// (ErrAgentSessionNotSteerable). The caller fills Author, Prompt, Request,
// IdempotencyKey and RequestHash.
func (s *PGStore) EnqueueAgentSessionMessage(ctx context.Context, m QueuedAgentMessage, limit int) (QueuedAgentMessage, bool, error) {
	var out QueuedAgentMessage
	inserted := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		inserted = false
		phase, archived, err := lockSessionForQueue(ctx, tx, m.SessionID)
		if err != nil {
			return err
		}
		if archived || phase == "canceled" || phase == "canceling" {
			return ErrAgentSessionNotSteerable
		}
		if m.IdempotencyKey != "" {
			existing, err := scanQueuedMessage(tx.QueryRow(ctx, `SELECT `+queuedMessageColumns+`
				FROM agent_session_queued_messages WHERE session_id=$1 AND idempotency_key=$2`, m.SessionID, m.IdempotencyKey))
			switch {
			case err == nil:
				if existing.RequestHash != m.RequestHash {
					return ErrQueuedMessageKeyReused
				}
				out = existing
				return nil
			case !errors.Is(err, pgx.ErrNoRows):
				return err
			}
		}
		pending, err := listPendingQueued(ctx, tx, m.SessionID, false)
		if err != nil {
			return err
		}
		if len(pending) >= limit {
			return ErrAgentSessionQueueFull
		}
		m.ID = ids.New(ids.AgentQueuedMessage)
		out, err = scanQueuedMessage(tx.QueryRow(ctx, `
			INSERT INTO agent_session_queued_messages (id, session_id, position, author, prompt, request, idempotency_key, request_hash)
			VALUES ($1, $2, (SELECT COALESCE(max(position), 0) + 1 FROM agent_session_queued_messages WHERE session_id=$2),
			        $3, $4, $5, NULLIF($6, ''), $7)
			RETURNING `+queuedMessageColumns, m.ID, m.SessionID, m.Author, m.Prompt, orEmptyJSON(m.Request, "{}"), m.IdempotencyKey, m.RequestHash))
		if err != nil {
			return err
		}
		out.Index = len(pending) + 1
		inserted = true
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrAgentSessionQueueFull) || errors.Is(err, ErrQueuedMessageKeyReused) || errors.Is(err, ErrAgentSessionNotSteerable) {
			return QueuedAgentMessage{}, false, err
		}
		return QueuedAgentMessage{}, false, classify("agent session queued message", err)
	}
	return out, inserted, nil
}

// ListQueuedAgentMessages returns the session's pending messages in queue
// order, each with its place.
func (s *PGStore) ListQueuedAgentMessages(ctx context.Context, sessionID string) ([]QueuedAgentMessage, error) {
	if !textBindable(sessionID) {
		return nil, nil
	}
	out, err := listPendingQueued(ctx, s.Pool, sessionID, false)
	if err != nil {
		return nil, classify("agent session queued message", err)
	}
	return out, nil
}

// CountQueuedAgentMessages is how many pending messages the session holds.
func (s *PGStore) CountQueuedAgentMessages(ctx context.Context, sessionID string) (int, error) {
	if !textBindable(sessionID) {
		return 0, nil
	}
	var n int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM agent_session_queued_messages WHERE session_id=$1 AND `+pendingQueuedSQL, sessionID).Scan(&n); err != nil {
		return 0, classify("agent session queued message", err)
	}
	return n, nil
}

// CountQueuedAgentMessagesBySession is how many pending messages each of the
// sessions holds; a session with none is absent.
func (s *PGStore) CountQueuedAgentMessagesBySession(ctx context.Context, sessionIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(sessionIDs))
	rows, err := s.Pool.Query(ctx, `
		SELECT session_id, count(*) FROM agent_session_queued_messages
		WHERE session_id = ANY($1) AND `+pendingQueuedSQL+` GROUP BY session_id`, sessionIDs)
	if err != nil {
		return nil, classify("agent session queued message", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// CountAllQueuedAgentMessages is how many pending messages every session holds.
func (s *PGStore) CountAllQueuedAgentMessages(ctx context.Context) (int, error) {
	var n int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM agent_session_queued_messages WHERE `+pendingQueuedSQL).Scan(&n); err != nil {
		return 0, classify("agent session queued message", err)
	}
	return n, nil
}

// lockQueuedMessage takes the session's lock, then the message's row, and
// refuses one that is not pending or is leased to the dispatcher now.
// ErrNotFound when the session has no such message.
func lockQueuedMessage(ctx context.Context, tx pgx.Tx, sessionID, messageID string) (QueuedAgentMessage, error) {
	if _, _, err := lockSessionForQueue(ctx, tx, sessionID); err != nil {
		return QueuedAgentMessage{}, err
	}
	m, err := scanQueuedMessage(tx.QueryRow(ctx, `SELECT `+queuedMessageColumns+`
		FROM agent_session_queued_messages WHERE id=$1 AND session_id=$2 FOR UPDATE`, messageID, sessionID))
	if err != nil {
		return QueuedAgentMessage{}, err
	}
	leased := m.ClaimedUntil != nil && m.ClaimedUntil.After(time.Now())
	if m.DeliveredAt != nil || m.DroppedReason != "" || leased {
		return QueuedAgentMessage{}, ErrQueuedMessageNotPending
	}
	return m, nil
}

func queuedMessageError(err error) error {
	if errors.Is(err, ErrQueuedMessageNotPending) || errors.Is(err, ErrQueuedMessageAttachmentsOwned) {
		return err
	}
	return classify("agent session queued message", err)
}

// EditQueuedAgentMessage replaces a pending message's prompt and makes author
// the member it runs as, since the text is now theirs. It also forgets why the
// last attempt was blocked. A message that is not pending, or is being
// dispatched now, is ErrQueuedMessageNotPending.
func (s *PGStore) EditQueuedAgentMessage(ctx context.Context, sessionID, messageID, prompt, author string) (QueuedAgentMessage, error) {
	var out QueuedAgentMessage
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		current, err := lockQueuedMessage(ctx, tx, sessionID, messageID)
		if err != nil {
			return err
		}
		var request struct {
			AttachmentIDs []string `json:"attachmentIds"`
		}
		if json.Unmarshal(current.Request, &request) == nil && len(request.AttachmentIDs) > 0 && current.Author != author {
			return ErrQueuedMessageAttachmentsOwned
		}
		out, err = scanQueuedMessage(tx.QueryRow(ctx, `
			UPDATE agent_session_queued_messages
			SET prompt=$3, author=$4, blocked_reason='', updated_at=now()
			WHERE id=$1 AND session_id=$2 RETURNING `+queuedMessageColumns, messageID, sessionID, prompt, author))
		if err != nil {
			return err
		}
		pending, err := listPendingQueued(ctx, tx, sessionID, false)
		if err != nil {
			return err
		}
		for _, p := range pending {
			if p.ID == out.ID {
				out.Index = p.Index
			}
		}
		return nil
	})
	if err != nil {
		return QueuedAgentMessage{}, queuedMessageError(err)
	}
	return out, nil
}

// DeleteQueuedAgentMessage removes a pending message from the queue; the same
// refusals as EditQueuedAgentMessage.
func (s *PGStore) DeleteQueuedAgentMessage(ctx context.Context, sessionID, messageID string) error {
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := lockQueuedMessage(ctx, tx, sessionID, messageID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM agent_session_queued_messages WHERE id=$1 AND session_id=$2`, messageID, sessionID)
		return err
	})
	if err != nil {
		return queuedMessageError(err)
	}
	return nil
}

// MoveQueuedAgentMessage places a pending message at the 1-based index among
// the session's pending messages (clamped to the queue's length) and returns
// the queue in its new order. The pending rows keep the set of position values
// they had and trade them. The same refusals as EditQueuedAgentMessage.
func (s *PGStore) MoveQueuedAgentMessage(ctx context.Context, sessionID, messageID string, index int) ([]QueuedAgentMessage, error) {
	var out []QueuedAgentMessage
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		target, err := lockQueuedMessage(ctx, tx, sessionID, messageID)
		if err != nil {
			return err
		}
		pending, err := listPendingQueued(ctx, tx, sessionID, true)
		if err != nil {
			return err
		}
		positions := make([]int64, len(pending))
		for i, p := range pending {
			positions[i] = p.Position
		}
		pending = slices.DeleteFunc(pending, func(p QueuedAgentMessage) bool { return p.ID == target.ID })
		at := min(max(index, 1), len(pending)+1) - 1
		pending = slices.Insert(pending, at, target)
		for i := range pending {
			if pending[i].Position != positions[i] {
				if _, err := tx.Exec(ctx, `UPDATE agent_session_queued_messages SET position=$2, updated_at=now() WHERE id=$1`, pending[i].ID, positions[i]); err != nil {
					return err
				}
				pending[i].Position = positions[i]
			}
			pending[i].Index = i + 1
		}
		out = pending
		return nil
	})
	if err != nil {
		return nil, queuedMessageError(err)
	}
	return out, nil
}

// QueuedAgentMessageState reads one message of the session in any state, so a
// caller can tell one that is gone from one that was already dispatched.
// ErrNotFound when the session has no such message.
func (s *PGStore) QueuedAgentMessageState(ctx context.Context, sessionID, messageID string) (QueuedAgentMessage, error) {
	if !textBindable(sessionID) || !textBindable(messageID) {
		return QueuedAgentMessage{}, ErrNotFound
	}
	m, err := scanQueuedMessage(s.Pool.QueryRow(ctx, `SELECT `+queuedMessageColumns+`
		FROM agent_session_queued_messages WHERE id=$1 AND session_id=$2`, messageID, sessionID))
	if err != nil {
		return QueuedAgentMessage{}, classify("agent session queued message", err)
	}
	return m, nil
}

// ClaimQueuedAgentMessages leases the head of the queue of up to limit
// sessions that can take a turn now: in one of phases, not archived, with a
// pending head no other caller holds. The lease runs until now+lease. A session
// whose head a caller holds is skipped whole, so two callers never start the
// same message and a later message never overtakes the head.
func (s *PGStore) ClaimQueuedAgentMessages(ctx context.Context, now time.Time, lease time.Duration, limit int, phases []string) ([]QueuedMessageClaim, error) {
	var out []QueuedMessageClaim
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		out = nil
		sessions, err := collectWith(scanClaimedSession)(tx.Query(ctx, `
			SELECT agent_sessions.id, workspace_id, updated_at FROM agent_sessions
			JOIN (SELECT DISTINCT ON (session_id) session_id, claimed_until, last_attempt_at
			      FROM agent_session_queued_messages WHERE `+pendingQueuedSQL+`
			      ORDER BY session_id, position, id) head ON head.session_id = agent_sessions.id
			WHERE phase = ANY($3) AND archived_at IS NULL
			  AND (head.claimed_until IS NULL OR head.claimed_until < $1)
			ORDER BY head.last_attempt_at NULLS FIRST, updated_at
			FOR UPDATE OF agent_sessions SKIP LOCKED
			LIMIT $2`, now, clampLimit(limit), phases))
		if err != nil || len(sessions) == 0 {
			return err
		}
		for _, session := range sessions {
			head, err := scanQueuedMessage(tx.QueryRow(ctx, `
				UPDATE agent_session_queued_messages SET claimed_until=$2, last_attempt_at=$3
				WHERE id = (SELECT id FROM agent_session_queued_messages WHERE session_id=$1 AND `+pendingQueuedSQL+`
				            ORDER BY position, id LIMIT 1)
				RETURNING `+queuedMessageColumns, session.ID, now.Add(lease), now))
			if err != nil {
				return err
			}
			out = append(out, QueuedMessageClaim{Session: session, Message: head})
		}
		return nil
	})
	if err != nil {
		return nil, classify("agent session queued message", err)
	}
	return out, nil
}

// scanClaimedSession reads the part of a session a claim keeps.
func scanClaimedSession(row pgx.Row) (AgentSession, error) {
	var s AgentSession
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.UpdatedAt)
	return s, err
}

// ReleaseQueuedAgentMessage ends a message's lease early so the next pass
// retries it, recording why this attempt did not start its turn (empty when it
// only lost a race).
func (s *PGStore) ReleaseQueuedAgentMessage(ctx context.Context, messageID, blockedReason string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_queued_messages SET claimed_until=NULL, blocked_reason=$2
		WHERE id=$1 AND delivered_at IS NULL AND dropped_reason IS NULL`, messageID, blockedReason)
	return err
}

// DropQueuedAgentMessage records why a pending message will never run.
func (s *PGStore) DropQueuedAgentMessage(ctx context.Context, messageID, reason string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_queued_messages SET dropped_reason=$2, claimed_until=NULL
		WHERE id=$1 AND delivered_at IS NULL AND dropped_reason IS NULL`, messageID, reason)
	return err
}

// dropSessionQueued drops every pending message of the session in the
// caller's transaction, as part of the transition that ends its queue.
func dropSessionQueued(ctx context.Context, tx pgx.Tx, sessionID, reason string) error {
	_, err := tx.Exec(ctx, `
		UPDATE agent_session_queued_messages SET dropped_reason=$2, claimed_until=NULL
		WHERE session_id=$1 AND `+pendingQueuedSQL, sessionID, reason)
	return err
}

// MarkAgentSessionTurnInterrupted records that a person stopped the turn.
func (s *PGStore) MarkAgentSessionTurnInterrupted(ctx context.Context, sessionID string, turn int) error {
	_, err := s.Pool.Exec(ctx, `UPDATE agent_session_turns SET interrupted=true WHERE session_id=$1 AND turn=$2`, sessionID, turn)
	return err
}
