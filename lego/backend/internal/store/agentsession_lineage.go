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
	"slices"

	"github.com/jackc/pgx/v5"
)

// Parent and child sessions (ADR102). The limits live here, in the insert
// transaction, so no caller can exceed them and a concurrent pair cannot both
// pass.
const (
	// MaxAgentSessionDepth is the deepest a session may sit below its root.
	MaxAgentSessionDepth = 3
	// MaxLiveAgentSessionChildren is how many live children one session may have.
	MaxLiveAgentSessionChildren = 5
)

// Refusals of a child create. The parent being in another workspace reads as
// not found, so a probe learns nothing about foreign sessions.
var (
	ErrAgentSessionParentNotFound = errors.New("agent session parent not found")
	ErrAgentSessionParentInactive = errors.New("agent session parent is not active")
	ErrAgentSessionDepthLimit     = errors.New("agent session depth limit reached")
	ErrAgentSessionFanOutLimit    = errors.New("agent session child limit reached")
)

func isAgentSessionLineageRefusal(err error) bool {
	return errors.Is(err, ErrAgentSessionParentNotFound) || errors.Is(err, ErrAgentSessionParentInactive) ||
		errors.Is(err, ErrAgentSessionDepthLimit) || errors.Is(err, ErrAgentSessionFanOutLimit)
}

type parentLineage struct {
	root  string
	depth int
}

// lockParentLineage locks the parent row (FOR NO KEY UPDATE: it conflicts with
// itself, so concurrent child inserts under one parent run one at a time, but
// not with the foreign-key share lock the insert itself takes), checks that it
// is a live session of the workspace and that one more child fits, and returns
// the lineage the child gets. The same lock makes a cancel that is moving the
// parent out of its active phases wait for, or be seen by, the insert.
func lockParentLineage(ctx context.Context, tx pgx.Tx, workspaceID, parentID string) (parentLineage, error) {
	var (
		phase, root string
		depth       int
	)
	err := tx.QueryRow(ctx, `
		SELECT phase, coalesce(root_session_id, ''), depth FROM agent_sessions
		WHERE id = $1 AND workspace_id = $2 FOR NO KEY UPDATE`, parentID, workspaceID).Scan(&phase, &root, &depth)
	if errors.Is(err, pgx.ErrNoRows) {
		return parentLineage{}, ErrAgentSessionParentNotFound
	}
	if err != nil {
		return parentLineage{}, err
	}
	if !isActiveAgentSessionPhase(phase) {
		return parentLineage{}, ErrAgentSessionParentInactive
	}
	if depth+1 > MaxAgentSessionDepth {
		return parentLineage{}, ErrAgentSessionDepthLimit
	}
	var live int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM agent_sessions WHERE parent_session_id = $1 AND `+activeAgentSessionPhasesSQL,
		parentID).Scan(&live); err != nil {
		return parentLineage{}, err
	}
	if live >= MaxLiveAgentSessionChildren {
		return parentLineage{}, ErrAgentSessionFanOutLimit
	}
	if root == "" {
		root = parentID
	}
	return parentLineage{root: root, depth: depth + 1}, nil
}

func isActiveAgentSessionPhase(phase string) bool {
	return slices.Contains(ActiveAgentSessionPhases, phase)
}

func (s *PGStore) queryAgentSessions(ctx context.Context, sql string, args ...any) ([]AgentSession, error) {
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSession, 0)
	for rows.Next() {
		v, err := scanAgentSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// AgentSessionSubtree returns rootID and every session below it, shallowest
// first and oldest first within a depth, each with its phase and status. It
// follows root_session_id, so the tree stays whole when a middle session was
// deleted. Empty when rootID is unknown in the workspace.
func (s *PGStore) AgentSessionSubtree(ctx context.Context, workspaceID, rootID string) ([]AgentSession, error) {
	if !textBindable(rootID) {
		return []AgentSession{}, nil
	}
	return s.queryAgentSessions(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions WHERE workspace_id = $1 AND (id = $2 OR root_session_id = $2)
		ORDER BY depth, created_at, id`, workspaceID, rootID)
}

// ListUnfinishedAgentSessionDescendants returns every descendant of sessionID
// (children, their children, and so on) that is still active or canceling,
// deepest first: the sessions a cancel of sessionID must cancel. A canceling
// one is included so a retry converges a cascade that failed half way.
func (s *PGStore) ListUnfinishedAgentSessionDescendants(ctx context.Context, workspaceID, sessionID string) ([]AgentSession, error) {
	if !textBindable(sessionID) {
		return []AgentSession{}, nil
	}
	return s.queryAgentSessions(ctx, `
		WITH RECURSIVE below AS (
			SELECT id FROM agent_sessions WHERE workspace_id = $1 AND parent_session_id = $2
			UNION
			SELECT c.id FROM agent_sessions c JOIN below b ON c.parent_session_id = b.id WHERE c.workspace_id = $1
		)
		SELECT `+agentSessionColumns+`
		FROM agent_sessions WHERE id IN (SELECT id FROM below)
		  AND (`+activeAgentSessionPhasesSQL+` OR phase = 'canceling')
		ORDER BY depth DESC, created_at, id`, workspaceID, sessionID)
}

// SetAgentSessionCancelCause records why a canceled session was canceled, in
// failure_reason and its code, once: a user's cancel leaves both empty, and a
// cause already written is kept.
func (s *PGStore) SetAgentSessionCancelCause(ctx context.Context, id, reason, code string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_sessions SET failure_reason = $2, failure_reason_code = $3
		WHERE id = $1 AND phase = 'canceled' AND failure_reason = ''`, id, reason, code)
	return err
}

// AgentSessionCeilingHeadroom is how much more model spend sessionID's tree
// may record before a ceiling stops it (ADR102 D3): the smallest, over the
// session and each of its ancestors that has a maxSpendUSD ceiling, of that
// ceiling minus the recorded platform-route price of the session and
// everything below it. bounded is false when no ceiling applies. A result at
// or below zero means a ceiling is reached; the value is not clamped.
func (s *PGStore) AgentSessionCeilingHeadroom(ctx context.Context, workspaceID, sessionID string) (headroom int64, bounded bool, err error) {
	err = s.Pool.QueryRow(ctx, `
		WITH RECURSIVE chain AS (
			SELECT id, parent_session_id, max_spend_microusd FROM agent_sessions
			WHERE id = $2 AND workspace_id = $1
			UNION
			SELECT p.id, p.parent_session_id, p.max_spend_microusd
			FROM agent_sessions p JOIN chain c ON p.id = c.parent_session_id WHERE p.workspace_id = $1
		), capped AS (
			SELECT id, max_spend_microusd FROM chain WHERE max_spend_microusd IS NOT NULL
		), below AS (
			SELECT c.id AS capped_id, c.id AS id FROM capped c
			UNION
			SELECT b.capped_id, s.id FROM agent_sessions s JOIN below b ON s.parent_session_id = b.id
			WHERE s.workspace_id = $1
		), spent AS (
			SELECT b.capped_id, COALESCE(SUM(e.price_microusd), 0)::bigint AS spent
			FROM below b LEFT JOIN agent_model_usage_events e ON e.session_id = b.id AND e.workspace_id = $1
			GROUP BY b.capped_id
		)
		SELECT COALESCE(MIN(c.max_spend_microusd - sp.spent), 0)::bigint, count(*) > 0
		FROM capped c JOIN spent sp ON sp.capped_id = c.id`, workspaceID, sessionID).Scan(&headroom, &bounded)
	return headroom, bounded, err
}
