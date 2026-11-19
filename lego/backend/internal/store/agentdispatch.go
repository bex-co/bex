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
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// AgentDispatch survives the process that accepted a turn, including after
// abandonment. Tombstones are revisited to catch a late remote create response.
type AgentDispatch struct {
	SessionDeleted    bool
	PreviousSandboxID string
	SessionID         string
	Turn              int
	WorkspaceID       string
	Legacy            bool
}

func insertAgentDispatch(ctx context.Context, tx pgx.Tx, row AgentSession, previousSandboxID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO agent_session_dispatches (session_id, turn, workspace_id, previous_sandbox_id) VALUES ($1,$2,$3,$4)`, row.ID, row.Turns, row.WorkspaceID, previousSandboxID)
	return err
}

// ListAgentDispatchesDue bounds each recovery tick. next_check_at rotates clean
// tombstones out of the hot set without forgetting a possibly late sandbox.
func (s *PGStore) ListAgentDispatchesDue(ctx context.Context, now time.Time) ([]AgentDispatch, error) {
	// During a rolling upgrade an old replica can still accept without the new
	// transaction. Repair those missing intents on every sweep, not only once
	// in the migration. New transactions publish session+intent atomically.
	_, err := s.Pool.Exec(ctx, `INSERT INTO agent_session_dispatches
		(session_id, turn, workspace_id, created_at, next_check_at, legacy)
		SELECT id, turns, workspace_id, updated_at, updated_at + interval '15 minutes', true
		FROM agent_sessions WHERE sandbox_id='' AND phase IN ('creating','redispatching')
		AND updated_at + interval '15 minutes' <= $1
		ON CONFLICT (session_id,turn) DO NOTHING`, now)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT session_id, turn, workspace_id, legacy, previous_sandbox_id, NOT EXISTS (SELECT 1 FROM agent_sessions WHERE id=session_id) FROM agent_session_dispatches WHERE next_check_at <= $1 ORDER BY abandoned, next_check_at, session_id, turn LIMIT 100`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentDispatch
	for rows.Next() {
		var d AgentDispatch
		if err := rows.Scan(&d.SessionID, &d.Turn, &d.WorkspaceID, &d.Legacy, &d.PreviousSandboxID, &d.SessionDeleted); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// AgentSessionFailure is why a dispatch failed: a sentence for the reader and,
// when a client acts on the cause, a stable code (w5/m132), and beside
// SANDBOX_START_FAILED the start's cause (ADR047 § w10/m497).
type AgentSessionFailure struct {
	Reason string
	Code   string
	Cause  string
}

// AbandonAgentDispatch serializes against binding and atomically settles only
// the accepted turn that owns the intent. A canceled/deleted/newer session
// still leaves a cleanup tombstone, but its lifecycle is never overwritten.
// When the session row is terminalized (still unbound creating/redispatching),
// the returned TerminalTurnFact carries timing for metrics (w5/m88); a zero
// Turn means no session terminalization occurred (tombstone-only or already
// bound — the latter returns ErrNotFound).
func (s *PGStore) AbandonAgentDispatch(ctx context.Context, d AgentDispatch, now time.Time, failure AgentSessionFailure) (TerminalTurnFact, error) {
	bound := false
	terminalized := false
	var fact TerminalTurnFact
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var turn int
		var sandboxID string
		err := tx.QueryRow(ctx, `SELECT turns, sandbox_id FROM agent_sessions WHERE id=$1 FOR UPDATE`, d.SessionID).Scan(&turn, &sandboxID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// An old API replica can finish a pre-migration dispatch without removing
		// its backfilled intent. Its durable binding wins; never sweep that pod.
		// A non-empty previous_sandbox_id stays as an abandoned cleanup tombstone
		// so the predecessor is still retried (w5/m99 t004).
		if turn == d.Turn && sandboxID != "" {
			bound = true
			var previous string
			if err := tx.QueryRow(ctx, `SELECT previous_sandbox_id FROM agent_session_dispatches WHERE session_id=$1 AND turn=$2`, d.SessionID, d.Turn).Scan(&previous); err != nil {
				// No dispatch row: the binding replica won the race and already
				// removed the intent, so there is no predecessor to tombstone.
				// That is the bound contract below (ErrNotFound for the loser),
				// not a query failure — returning pgx.ErrNoRows raw skipped it.
				if !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
				return nil
			}
			if previous != "" {
				_, err := tx.Exec(ctx, `UPDATE agent_session_dispatches SET abandoned=true, next_check_at=$3 WHERE session_id=$1 AND turn=$2`, d.SessionID, d.Turn, now)
				return err
			}
			_, err := tx.Exec(ctx, `DELETE FROM agent_session_dispatches WHERE session_id=$1 AND turn=$2`, d.SessionID, d.Turn)
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE agent_session_dispatches SET abandoned=true WHERE session_id=$1 AND turn=$2`, d.SessionID, d.Turn)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		tag, err = tx.Exec(ctx, `UPDATE agent_sessions SET phase='failed', status='failed', failure_reason=$3, failure_reason_code=$4, failure_cause=$6, sandbox_wait_since=NULL, updated_at=$5
    WHERE id=$1 AND turns=$2 AND sandbox_id='' AND phase IN ('creating','redispatching')`, d.SessionID, d.Turn, failure.Reason, failure.Code, now, failure.Cause)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		terminalized = true
		fact.Turn = d.Turn
		fact.TerminalAt = now
		err = tx.QueryRow(ctx, `UPDATE agent_session_turns SET completed_at=COALESCE(completed_at,$3), transcript_complete=false, truncation_reason=$4
    WHERE session_id=$1 AND turn=$2 AND completed_at IS NULL
    RETURNING created_at, started_at, completed_at`, d.SessionID, d.Turn, now, failure.Reason).
			Scan(&fact.AcceptedAt, &fact.StartedAt, &fact.TerminalAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Turn already completed (settleDispatchTurn raced) — still a
				// successful session terminalization; load timing without rewriting.
				err = tx.QueryRow(ctx, `SELECT created_at, started_at, completed_at FROM agent_session_turns
					WHERE session_id=$1 AND turn=$2`, d.SessionID, d.Turn).
					Scan(&fact.AcceptedAt, &fact.StartedAt, &fact.TerminalAt)
				if errors.Is(err, pgx.ErrNoRows) {
					return nil
				}
			}
			return err
		}
		return nil
	})
	if err == nil && bound {
		return TerminalTurnFact{}, ErrNotFound
	}
	if err != nil {
		return TerminalTurnFact{}, err
	}
	if !terminalized {
		return TerminalTurnFact{}, nil
	}
	return fact, nil
}

// MarkAgentDispatchWaiting records that d's dispatch waits for room on the
// sandbox pool and keeps it alive (ADR047 § w10/m497): it stamps the
// session's sandbox_wait_since once and moves the dispatch's next_check_at to
// next, so neither recovery nor the legacy repair abandons a dispatch that is
// still waiting. It returns when the wait began. ok is false, and nothing is
// written, when the wait must end: the session is no longer unbound on d's
// turn in creating or redispatching (a cancel, a delete, a newer turn), it was
// archived, or the dispatch was abandoned.
func (s *PGStore) MarkAgentDispatchWaiting(ctx context.Context, d AgentDispatch, now, next time.Time) (since time.Time, ok bool, err error) {
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		// Lock in the same order as binding and abandonment: session, then intent.
		var current *time.Time
		err := tx.QueryRow(ctx, `SELECT sandbox_wait_since FROM agent_sessions
			WHERE id=$1 AND turns=$2 AND sandbox_id='' AND phase IN ('creating','redispatching') AND archived_at IS NULL
			FOR UPDATE`, d.SessionID, d.Turn).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE agent_session_dispatches SET next_check_at=$3
			WHERE session_id=$1 AND turn=$2 AND NOT abandoned`, d.SessionID, d.Turn, next)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		if current != nil {
			since, ok = *current, true
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_sessions SET sandbox_wait_since=$2, updated_at=$2 WHERE id=$1`, d.SessionID, now); err != nil {
			return err
		}
		since, ok = now, true
		return nil
	})
	if err != nil {
		return time.Time{}, false, err
	}
	return since, ok, nil
}

func (s *PGStore) DeferAgentDispatchCleanup(ctx context.Context, d AgentDispatch, next time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE agent_session_dispatches SET next_check_at=$3 WHERE session_id=$1 AND turn=$2 AND abandoned`, d.SessionID, d.Turn, next)
	return err
}
