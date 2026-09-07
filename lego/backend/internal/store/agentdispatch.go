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

// AbandonAgentDispatch serializes against binding and atomically settles only
// the accepted turn that owns the intent. A canceled/deleted/newer session
// still leaves a cleanup tombstone, but its lifecycle is never overwritten.
func (s *PGStore) AbandonAgentDispatch(ctx context.Context, d AgentDispatch, now time.Time, reason string) error {
	bound := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var turn int
		var sandboxID string
		err := tx.QueryRow(ctx, `SELECT turns, sandbox_id FROM agent_sessions WHERE id=$1 FOR UPDATE`, d.SessionID).Scan(&turn, &sandboxID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// An old API replica can finish a pre-migration dispatch without removing
		// its backfilled intent. Its durable binding wins; never sweep that pod.
		if turn == d.Turn && sandboxID != "" {
			bound = true
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
		if _, err = tx.Exec(ctx, `UPDATE agent_sessions SET phase='failed', status='failed', failure_reason=$3, updated_at=$4
    WHERE id=$1 AND turns=$2 AND sandbox_id='' AND phase IN ('creating','redispatching')`, d.SessionID, d.Turn, reason, now); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE agent_session_turns SET completed_at=COALESCE(completed_at,$3), transcript_complete=false, truncation_reason=$4
    WHERE session_id=$1 AND turn=$2 AND completed_at IS NULL`, d.SessionID, d.Turn, now, reason)
		return err
	})
	if err == nil && bound {
		return ErrNotFound
	}
	return err
}

func (s *PGStore) DeferAgentDispatchCleanup(ctx context.Context, d AgentDispatch, next time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE agent_session_dispatches SET next_check_at=$3 WHERE session_id=$1 AND turn=$2 AND abandoned`, d.SessionID, d.Turn, next)
	return err
}
