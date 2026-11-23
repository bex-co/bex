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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// The states of a session's pull request (Devin's pr_state). A draft is open.
const (
	PRStateOpen   = "open"
	PRStateMerged = "merged"
	PRStateClosed = "closed"
)

// PRAutoMerge is GitHub auto-merge armed on a session's pull request
// (w10/047), as GitHub's autoMergeRequest reports it: its merge method in
// lower case, and the GitHub login that armed it and when (either absent
// when GitHub does not say).
type PRAutoMerge struct {
	Method    string     `json:"method"`
	EnabledBy string     `json:"enabledBy,omitempty"`
	EnabledAt *time.Time `json:"enabledAt,omitempty"`
}

// AgentSessionPRCheck is one claimed pull request read.
type AgentSessionPRCheck struct {
	SessionID   string
	WorkspaceID string
	Repo        string
	PRNumber    int
	PRState     string
	PRStateAt   *time.Time
}

// ClaimAgentSessionPRChecks leases up to limit sessions whose pull request is
// due a read (pr_next_check_at <= now), moving each one's next check to
// now+retry so a read that fails is retried then (FOR UPDATE SKIP LOCKED, so
// replicas never read the same one).
func (s *PGStore) ClaimAgentSessionPRChecks(ctx context.Context, now time.Time, retry time.Duration, limit int) ([]AgentSessionPRCheck, error) {
	rows, err := collectWith(scanAgentSessionPRCheck)(s.Pool.Query(ctx, `
		UPDATE agent_sessions SET pr_next_check_at = $1::timestamptz + $2 * interval '1 second'
		WHERE id IN (
			SELECT id FROM agent_sessions
			WHERE pr_next_check_at <= $1
			ORDER BY pr_next_check_at
			LIMIT $3
			FOR UPDATE SKIP LOCKED)
		RETURNING id, workspace_id, repo, pr_number, pr_state, pr_state_at`,
		now, retry.Seconds(), clampLimit(limit)))
	if err != nil {
		return nil, fmt.Errorf("claim agent session pr checks: %w", err)
	}
	return rows, nil
}

func scanAgentSessionPRCheck(row pgx.Row) (AgentSessionPRCheck, error) {
	var out AgentSessionPRCheck
	err := row.Scan(&out.SessionID, &out.WorkspaceID, &out.Repo, &out.PRNumber, &out.PRState, &out.PRStateAt)
	return out, err
}

// RecordAgentSessionPRState sets a session's pull request state as read at
// at, and when to read it next (nil: never). pr_state_at moves only when the
// state changes, and a merged or closed one drops its auto-merge and merge
// queue marks, as GitHub does. It reports a change.
func (s *PGStore) RecordAgentSessionPRState(ctx context.Context, sessionID, state string, at time.Time, next *time.Time) (bool, error) {
	var changed bool
	err := s.Pool.QueryRow(ctx, `
		WITH prior AS (SELECT pr_state FROM agent_sessions WHERE id = $1 AND pr_number > 0 FOR UPDATE)
		UPDATE agent_sessions a SET pr_state = $2, pr_next_check_at = $4,
		       pr_state_at = CASE WHEN prior.pr_state <> $2 THEN $3 ELSE a.pr_state_at END,
		       pr_auto_merge = CASE WHEN $2 = 'open' THEN a.pr_auto_merge END,
		       pr_merge_queued = a.pr_merge_queued AND $2 = 'open'
		FROM prior WHERE a.id = $1
		RETURNING prior.pr_state <> $2`, sessionID, state, at, next).Scan(&changed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("record agent session pr state: %w", err)
	}
	return changed, nil
}

// RecordPullRequestState applies a GitHub pull_request delivery: every session
// in workspaceIDs whose pull request is repo#number and whose state differs
// takes state, changed at at, next read at next (nil: never), dropping its
// auto-merge and merge queue marks when it is merged or closed. It returns how
// many changed.
func (s *PGStore) RecordPullRequestState(ctx context.Context, workspaceIDs []string, repo string, number int, state string, at time.Time, next *time.Time) (int, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET pr_state = $4, pr_state_at = $5, pr_next_check_at = $6,
		       pr_auto_merge = CASE WHEN $4 = 'open' THEN pr_auto_merge END,
		       pr_merge_queued = pr_merge_queued AND $4 = 'open'
		WHERE workspace_id = ANY($1) AND repo = $2 AND pr_number = $3 AND pr_number > 0 AND pr_state <> $4`,
		workspaceIDs, repo, number, state, at, next)
	if err != nil {
		return 0, fmt.Errorf("record pull request state: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// RecordPullRequestMergeMarks records what GitHub says of pull request
// repo#number's auto-merge and merge queue (w10/047) on every session in
// workspaceIDs that recorded it and has not read it merged or closed:
// autoMerge is the auto-merge armed on it, nil for none, and queued is set
// while it is in its base branch's merge queue. It returns how many changed.
func (s *PGStore) RecordPullRequestMergeMarks(ctx context.Context, workspaceIDs []string, repo string, number int, autoMerge *PRAutoMerge, queued bool) (int, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET pr_auto_merge = $4, pr_merge_queued = $5
		WHERE workspace_id = ANY($1) AND repo = lower($2) AND pr_number = $3 AND pr_number > 0
		  AND pr_state IN ('', 'open')
		  AND (pr_auto_merge IS DISTINCT FROM $4::jsonb OR pr_merge_queued <> $5)`,
		workspaceIDs, repo, number, autoMerge, queued)
	if err != nil {
		return 0, fmt.Errorf("record pull request merge marks: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
