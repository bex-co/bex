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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Decision kinds, states and approval scopes (ADR100 D2). A decision is open
// while pending; every other state is final.
const (
	AgentDecisionApproval = "approval"
	AgentDecisionQuestion = "question"

	AgentDecisionPending  = "pending"
	AgentDecisionAllowed  = "allowed"
	AgentDecisionDenied   = "denied"
	AgentDecisionAnswered = "answered"
	AgentDecisionExpired  = "expired"
	AgentDecisionCanceled = "canceled"

	// AgentDecisionScopeOnce allows one call; AgentDecisionScopeSession also
	// allows the same rule for the rest of the session.
	AgentDecisionScopeOnce    = "once"
	AgentDecisionScopeSession = "session"

	// AgentSessionNeedsInput is the coarse state (ADR094 D7) of a running
	// turn paused on a decision (ADR100 D1).
	AgentSessionNeedsInput = "needs_input"
)

var (
	// ErrAgentDecisionAnswered: the decision already has its final answer.
	ErrAgentDecisionAnswered = fmt.Errorf("agent session decision already answered: %w", ErrConflict)
	// ErrAgentDecisionExpired: the decision timed out (or is past its expiry
	// and the sweep has not stamped it yet).
	ErrAgentDecisionExpired = fmt.Errorf("agent session decision expired: %w", ErrConflict)
	// ErrAgentDecisionNotOpen: the session is not running the named turn, so it
	// cannot pause on a decision.
	ErrAgentDecisionNotOpen = fmt.Errorf("agent session is not running this turn: %w", ErrConflict)
)

// AgentDecision is one approval or question a running turn paused on.
type AgentDecision struct {
	ID          string
	SessionID   string
	WorkspaceID string
	Turn        int
	RequestID   string
	Kind        string
	Title       string
	Detail      json.RawMessage
	State       string
	// Answer is the final answer ({allow, scope} or {text}); nil while pending.
	Answer     json.RawMessage
	AnsweredBy string
	CreatedAt  time.Time
	AnsweredAt *time.Time
	ExpiresAt  time.Time
}

// AgentDecisionInput opens a decision. RequestID is the driver's own key, so a
// retried request returns the decision it already opened.
type AgentDecisionInput struct {
	SessionID   string
	WorkspaceID string
	Turn        int
	RequestID   string
	Kind        string
	Title       string
	Detail      json.RawMessage
	TTL         time.Duration
}

// agentDecisionColumnList is the one column list every read scans; qualified
// renders it for an UPDATE … FROM, where the bare names would be ambiguous.
var agentDecisionColumnList = []string{"id", "session_id", "workspace_id", "turn", "request_id", "kind", "title",
	"detail", "state", "answer", "answered_by", "created_at", "answered_at", "expires_at"}

func decisionColumns(prefix string) string {
	cols := make([]string, len(agentDecisionColumnList))
	for i, c := range agentDecisionColumnList {
		cols[i] = prefix + c
		if c == "answered_by" {
			cols[i] = "COALESCE(" + prefix + c + ", '')"
		}
	}
	return strings.Join(cols, ", ")
}

var (
	agentDecisionColumns     = decisionColumns("")
	qualifiedDecisionColumns = decisionColumns("d.")
)

func scanAgentDecision(row pgx.Row) (AgentDecision, error) {
	var d AgentDecision
	err := row.Scan(&d.ID, &d.SessionID, &d.WorkspaceID, &d.Turn, &d.RequestID, &d.Kind, &d.Title, &d.Detail,
		&d.State, &d.Answer, &d.AnsweredBy, &d.CreatedAt, &d.AnsweredAt, &d.ExpiresAt)
	return d, err
}

// OpenAgentDecision records a decision the session's current turn is paused
// on and refreshes the session's mirror, in one transaction holding the
// session row lock. An engine may ask several at once (parallel tool calls),
// so other open decisions stay open. created is false when the request id
// already opened one (its row is returned as it stands).
// ErrAgentDecisionNotOpen when the session is not running in.Turn.
func (s *PGStore) OpenAgentDecision(ctx context.Context, in AgentDecisionInput) (AgentDecision, bool, error) {
	var out AgentDecision
	created := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var phase string
		var turns int
		if err := tx.QueryRow(ctx, `
			SELECT phase, turns FROM agent_sessions
			WHERE id = $1 AND workspace_id = $2 FOR UPDATE`, in.SessionID, in.WorkspaceID,
		).Scan(&phase, &turns); err != nil {
			return classify("agent session", err)
		}
		existing, err := scanAgentDecision(tx.QueryRow(ctx, `SELECT `+agentDecisionColumns+`
			FROM agent_session_decisions WHERE session_id = $1 AND request_id = $2`, in.SessionID, in.RequestID))
		if err == nil {
			out = existing
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if phase != "running" || turns != in.Turn {
			return ErrAgentDecisionNotOpen
		}
		detail := in.Detail
		if len(detail) == 0 {
			detail = json.RawMessage(`{}`)
		}
		out, err = scanAgentDecision(tx.QueryRow(ctx, `
			INSERT INTO agent_session_decisions (id, session_id, workspace_id, turn, request_id, kind, title, detail, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, clock_timestamp() + $9::interval)
			RETURNING `+agentDecisionColumns,
			ids.New(ids.AgentDecision), in.SessionID, in.WorkspaceID, in.Turn, in.RequestID, in.Kind, in.Title, detail, in.TTL.String()))
		if err != nil {
			return err
		}
		created = true
		return refreshPendingDecisions(ctx, tx, []string{in.SessionID})
	})
	if err != nil {
		if errors.Is(err, ErrAgentDecisionNotOpen) || errors.Is(err, ErrNotFound) {
			return AgentDecision{}, false, err
		}
		return AgentDecision{}, false, classify("agent session decision", err)
	}
	return out, created, nil
}

// GetAgentDecision reads one decision; ErrNotFound when it does not exist.
func (s *PGStore) GetAgentDecision(ctx context.Context, id string) (AgentDecision, error) {
	d, err := scanAgentDecision(s.Pool.QueryRow(ctx, `SELECT `+agentDecisionColumns+`
		FROM agent_session_decisions WHERE id = $1`, id))
	if err != nil {
		return AgentDecision{}, classify("agent session decision", err)
	}
	return d, nil
}

// ResolveAgentDecision records the final answer of an open decision of the
// session and refreshes the session's mirror, atomically: of two concurrent
// answers exactly one wins. It locks the session row before the decision row,
// the order the phase trigger and OpenAgentDecision use, so an answer racing a
// cancel or turn end cannot deadlock. ErrNotFound when the session has no such decision,
// ErrAgentDecisionExpired when it is past its expiry, ErrAgentDecisionAnswered
// when it already has a final state.
func (s *PGStore) ResolveAgentDecision(ctx context.Context, sessionID, id, state string, answer json.RawMessage, answeredBy string) (AgentDecision, error) {
	var out AgentDecision
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM agent_sessions WHERE id = $1 FOR UPDATE`, sessionID); err != nil {
			return err
		}
		var err error
		out, err = scanAgentDecision(tx.QueryRow(ctx, `
			UPDATE agent_session_decisions
			   SET state = $3, answer = $4, answered_by = NULLIF($5, ''), answered_at = clock_timestamp()
			 WHERE id = $1 AND session_id = $2 AND state = 'pending' AND expires_at > clock_timestamp()
			RETURNING `+agentDecisionColumns, id, sessionID, state, answer, answeredBy))
		if errors.Is(err, pgx.ErrNoRows) {
			current, readErr := scanAgentDecision(tx.QueryRow(ctx, `SELECT `+agentDecisionColumns+`
				FROM agent_session_decisions WHERE id = $1 AND session_id = $2`, id, sessionID))
			switch {
			case errors.Is(readErr, pgx.ErrNoRows):
				return ErrNotFound
			case readErr != nil:
				return readErr
			case current.State == AgentDecisionPending || current.State == AgentDecisionExpired:
				return ErrAgentDecisionExpired
			default:
				return ErrAgentDecisionAnswered
			}
		}
		if err != nil {
			return err
		}
		return refreshPendingDecisions(ctx, tx, []string{sessionID})
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAgentDecisionExpired) || errors.Is(err, ErrAgentDecisionAnswered) {
			return AgentDecision{}, err
		}
		return AgentDecision{}, classify("agent session decision", err)
	}
	return out, nil
}

// ExpireAgentDecisions resolves the open decisions past their expiry of up to
// limit sessions (ADR100 D2): an approval as denied ({"allow":false}), a
// question with no answer ({"text":""}), state expired, and refreshes those
// sessions' mirrors by primary key. It locks the sessions first (skipping any
// another sweep or writer holds), then their decisions: the order every
// decision write follows. Trusted background write.
func (s *PGStore) ExpireAgentDecisions(ctx context.Context, limit int) ([]AgentDecision, error) {
	var out []AgentDecision
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id FROM agent_sessions
			WHERE id IN (
				SELECT session_id FROM agent_session_decisions
				WHERE state = 'pending' AND expires_at <= clock_timestamp()
				GROUP BY session_id ORDER BY min(expires_at) LIMIT $1)
			ORDER BY id FOR UPDATE SKIP LOCKED`, limit)
		if err != nil {
			return err
		}
		sessions, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil || len(sessions) == 0 {
			return err
		}
		rows, err = tx.Query(ctx, `
			UPDATE agent_session_decisions d
			   SET state = 'expired', answered_at = clock_timestamp(),
			       answer = CASE d.kind WHEN 'approval' THEN '{"allow":false}'::jsonb ELSE '{"text":""}'::jsonb END
			 WHERE d.session_id = ANY($1) AND d.state = 'pending' AND d.expires_at <= clock_timestamp()
			RETURNING `+qualifiedDecisionColumns, sessions)
		if err != nil {
			return err
		}
		if out, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (AgentDecision, error) { return scanAgentDecision(row) }); err != nil {
			return err
		}
		return refreshPendingDecisions(ctx, tx, sessions)
	})
	if err != nil {
		return nil, classify("agent session decision", err)
	}
	return out, nil
}

// refreshPendingDecisions recomputes the sessions' decision mirror from their
// open decisions: the oldest one (what a read shows) and how many are open.
// Callers hold the session rows' locks.
func refreshPendingDecisions(ctx context.Context, tx pgx.Tx, sessionIDs []string) error {
	_, err := tx.Exec(ctx, `
		UPDATE agent_sessions s SET
		       pending_decision_id = oldest.id, pending_decision_kind = oldest.kind,
		       pending_decision_count = COALESCE(open.n, 0), updated_at = now()
		  FROM unnest($1::text[]) AS target(id)
		  LEFT JOIN LATERAL (
		       SELECT d.id, d.kind FROM agent_session_decisions d
		        WHERE d.session_id = target.id AND d.state = 'pending'
		        ORDER BY d.created_at, d.id LIMIT 1) oldest ON true
		  LEFT JOIN LATERAL (
		       SELECT count(*)::int AS n FROM agent_session_decisions d
		        WHERE d.session_id = target.id AND d.state = 'pending') open ON true
		 WHERE s.id = target.id`, sessionIDs)
	return err
}
