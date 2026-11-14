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
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Kinds of PR feedback (ADR047 D11).
const (
	FeedbackPRComment = "pr_comment"
	FeedbackPRReview  = "pr_review"
	FeedbackCIFailure = "ci_failure"
)

// AgentSessionFeedback is one queued piece of PR feedback for a session.
type AgentSessionFeedback struct {
	SessionID string
	// SourceKey identifies the GitHub object it came from ("review:<id>",
	// "comment:<id>", "suite:<id>"), unique per session.
	SourceKey string
	Kind      string
	// Actor is the bex subject the steer runs as.
	Actor string
	// Author is the commenter's GitHub login, empty for a CI failure.
	Author string
	URL    string
	Body   string
	// HeadSHA is the commit a CI failure ran on; empty otherwise.
	HeadSHA string
	// BaseFailingRef is the pull request's base branch when at least one of
	// a CI failure's checks also failed on its latest commit; empty otherwise.
	BaseFailingRef string
	CreatedAt      time.Time
	DeliveredTurn  *int
}

// FeedbackBatch is one session's pending feedback, leased to one caller.
type FeedbackBatch struct {
	Session AgentSession
	Items   []AgentSessionFeedback
}

// FeedbackTurn is one turn PR feedback started, and whether CI failures were
// all it delivered.
type FeedbackTurn struct {
	Turn   int
	CIOnly bool
}

// FindAgentSessionForPullRequest returns the session in one of workspaceIDs
// whose repository is repo (owner/name; sessions store it lowercase) and
// whose PR is prNumber: the newest live one, else the newest of any state, so
// an ended session never shadows one that still works on the pull request.
// ErrNotFound when none.
func (s *PGStore) FindAgentSessionForPullRequest(ctx context.Context, workspaceIDs []string, repo string, prNumber int) (AgentSession, error) {
	if !textBindable(repo) || prNumber <= 0 || len(workspaceIDs) == 0 {
		return AgentSession{}, ErrNotFound
	}
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions
		WHERE repo = lower($2) AND pr_number = $3 AND workspace_id = ANY($1)
		ORDER BY (`+liveAgentSessionSQL+`) DESC, created_at DESC LIMIT 1`, workspaceIDs, repo, prNumber))
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentSession{}, ErrNotFound
	}
	return out, err
}

// FindAgentSessionsForHead returns the sessions in workspaceIDs with a pull
// request whose repository is repo and whose recorded head commit is sha.
func (s *PGStore) FindAgentSessionsForHead(ctx context.Context, workspaceIDs []string, repo, sha string) ([]AgentSession, error) {
	if !textBindable(repo) || !textBindable(sha) || sha == "" || len(workspaceIDs) == 0 {
		return nil, nil
	}
	return s.queryAgentSessions(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions
		WHERE head_sha = $3 AND pr_number > 0 AND repo = lower($2) AND workspace_id = ANY($1)`,
		workspaceIDs, repo, sha)
}

// QueueAgentSessionFeedback adds one piece of feedback; inserted is false when
// the same source was already queued for the session (a redelivery).
func (s *PGStore) QueueAgentSessionFeedback(ctx context.Context, f AgentSessionFeedback) (bool, error) {
	inserted, err := insertAgentSessionFeedback(ctx, s.Pool, f, 0)
	if err != nil {
		return false, classify("agent session feedback", err)
	}
	return inserted, nil
}

// insertAgentSessionFeedback writes one feedback row, pending or, with
// deliveredTurn > 0, already delivered by that turn (a takeover's starting
// comment). A row for the same source is left as it is.
func insertAgentSessionFeedback(ctx context.Context, q interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}, f AgentSessionFeedback, deliveredTurn int) (bool, error) {
	tag, err := q.Exec(ctx, `
		INSERT INTO agent_session_feedback (session_id, source_key, kind, actor, author, url, body, head_sha, base_failing_ref, delivered_turn, delivered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, 0), CASE WHEN $10 > 0 THEN now() END)
		ON CONFLICT (session_id, source_key) DO NOTHING`,
		f.SessionID, f.SourceKey, f.Kind, f.Actor, f.Author, f.URL, f.Body, f.HeadSHA, f.BaseFailingRef, deliveredTurn)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// HasAgentSessionFeedback reports whether feedback from sourceKey is already
// stored for the session, pending, delivered or dropped.
func (s *PGStore) HasAgentSessionFeedback(ctx context.Context, sessionID, sourceKey string) (bool, error) {
	var found bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_session_feedback WHERE session_id=$1 AND source_key=$2)`,
		sessionID, sourceKey).Scan(&found); err != nil {
		return false, classify("agent session feedback", err)
	}
	return found, nil
}

// ClaimDeliverableFeedback leases the pending feedback of up to limit sessions
// that can take a turn now: in one of phases, not archived, and with rows no
// other caller holds. Each session's rows are leased until now+lease and
// returned oldest first, with the session as it stands.
func (s *PGStore) ClaimDeliverableFeedback(ctx context.Context, now time.Time, lease time.Duration, limit int, phases []string) ([]FeedbackBatch, error) {
	var out []FeedbackBatch
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		out = nil
		sessions, err := collectWith(scanAgentSession)(tx.Query(ctx, `
			SELECT `+agentSessionColumns+` FROM agent_sessions
			WHERE id IN (SELECT DISTINCT session_id FROM agent_session_feedback
			             WHERE delivered_at IS NULL AND dropped_reason IS NULL
			               AND (claimed_until IS NULL OR claimed_until < $1))
			  AND phase = ANY($3) AND archived_at IS NULL
			ORDER BY updated_at
			FOR UPDATE SKIP LOCKED
			LIMIT $2`, now, clampLimit(limit), phases))
		if err != nil || len(sessions) == 0 {
			return err
		}
		ids := make([]string, 0, len(sessions))
		for _, session := range sessions {
			ids = append(ids, session.ID)
		}
		items, err := collectWith(scanAgentSessionFeedback)(tx.Query(ctx, `
			UPDATE agent_session_feedback SET claimed_until = $2
			WHERE session_id = ANY($1) AND delivered_at IS NULL AND dropped_reason IS NULL
			  AND (claimed_until IS NULL OR claimed_until < $3)
			RETURNING `+agentSessionFeedbackColumns, ids, now.Add(lease), now))
		if err != nil {
			return err
		}
		slices.SortFunc(items, func(a, b AgentSessionFeedback) int {
			return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), strings.Compare(a.SourceKey, b.SourceKey))
		})
		for _, session := range sessions {
			batch := FeedbackBatch{Session: session}
			for _, item := range items {
				if item.SessionID == session.ID {
					batch.Items = append(batch.Items, item)
				}
			}
			out = append(out, batch)
		}
		return nil
	})
	if err != nil {
		return nil, classify("agent session feedback", err)
	}
	return out, nil
}

const agentSessionFeedbackColumns = `session_id, source_key, kind, actor, author, url, body, head_sha, base_failing_ref, created_at, delivered_turn`

func scanAgentSessionFeedback(row pgx.Row) (AgentSessionFeedback, error) {
	var f AgentSessionFeedback
	err := row.Scan(&f.SessionID, &f.SourceKey, &f.Kind, &f.Actor, &f.Author, &f.URL, &f.Body, &f.HeadSHA, &f.BaseFailingRef, &f.CreatedAt, &f.DeliveredTurn)
	return f, err
}

// MarkFeedbackDelivered records that turn delivered the session's feedback
// keyed by sourceKeys.
func (s *PGStore) MarkFeedbackDelivered(ctx context.Context, sessionID string, sourceKeys []string, turn int) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_feedback SET delivered_turn=$3, delivered_at=now(), claimed_until=NULL
		WHERE session_id=$1 AND source_key = ANY($2)`, sessionID, sourceKeys, turn)
	return err
}

// DropFeedback records why the session's feedback keyed by sourceKeys will
// never be delivered.
func (s *PGStore) DropFeedback(ctx context.Context, sessionID string, sourceKeys []string, reason string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_feedback SET dropped_reason=$3, claimed_until=NULL
		WHERE session_id=$1 AND source_key = ANY($2)`, sessionID, sourceKeys, reason)
	return err
}

// ReleaseFeedback ends a lease early so the next pass retries the rows.
func (s *PGStore) ReleaseFeedback(ctx context.Context, sessionID string, sourceKeys []string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_feedback SET claimed_until=NULL
		WHERE session_id=$1 AND source_key = ANY($2) AND delivered_at IS NULL`, sessionID, sourceKeys)
	return err
}

// DropEndedSessionsFeedback drops the pending feedback of sessions that will
// take no more turns: archived ones and those in one of endedPhases.
func (s *PGStore) DropEndedSessionsFeedback(ctx context.Context, endedPhases []string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_feedback f SET dropped_reason='the session ended', claimed_until=NULL
		FROM agent_sessions g
		WHERE f.session_id = g.id AND f.delivered_at IS NULL AND f.dropped_reason IS NULL
		  AND (g.archived_at IS NOT NULL OR g.phase = ANY($1))`, endedPhases)
	return err
}

// FeedbackTurns lists the turns PR feedback started for the session, newest
// first, each with whether it delivered only CI failures.
func (s *PGStore) FeedbackTurns(ctx context.Context, sessionID string) ([]FeedbackTurn, error) {
	return collectWith(func(row pgx.Row) (FeedbackTurn, error) {
		var t FeedbackTurn
		return t, row.Scan(&t.Turn, &t.CIOnly)
	})(s.Pool.Query(ctx, `
		SELECT delivered_turn, bool_and(kind = $2) FROM agent_session_feedback
		WHERE session_id=$1 AND delivered_turn IS NOT NULL
		GROUP BY delivered_turn ORDER BY delivered_turn DESC`, sessionID, FeedbackCIFailure))
}

// ListDeliveredFeedback returns the session's delivered feedback by turn for
// the turn-origin view, without bodies.
func (s *PGStore) ListDeliveredFeedback(ctx context.Context, sessionID string) ([]AgentSessionFeedback, error) {
	if !textBindable(sessionID) {
		return nil, nil
	}
	return collectWith(func(row pgx.Row) (AgentSessionFeedback, error) {
		var f AgentSessionFeedback
		return f, row.Scan(&f.Kind, &f.Author, &f.URL, &f.BaseFailingRef, &f.DeliveredTurn)
	})(s.Pool.Query(ctx, `
		SELECT kind, author, url, base_failing_ref, delivered_turn FROM agent_session_feedback
		WHERE session_id=$1 AND delivered_turn IS NOT NULL
		ORDER BY delivered_turn, created_at, source_key`, sessionID))
}
