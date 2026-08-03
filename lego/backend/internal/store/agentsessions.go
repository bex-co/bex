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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// AgentSessionTranscriptPart is one durable UI-message-stream part (ADR047 D9,
// w3/m43). Seq is the driver's emission ordinal on the /stream the gateway
// proxies (0-based, stable across replays and replicas); Turn is the session
// turn the part belongs to; Part is the verbatim `data:` payload — the exact
// bytes forwarded to a Vercel AI SDK client.
type AgentSessionTranscriptPart struct {
	Seq       int64
	Turn      int
	Part      json.RawMessage
	CreatedAt time.Time
}

// AgentSession is the durable control-plane record for one cloud coding-agent
// task (ADR047 D3). AgentConfig is kept as JSON because the driver-specific
// knobs evolve independently of the lifecycle contract; callers validate its
// public shape before persistence.
type AgentSession struct {
	ID            string
	WorkspaceID   string
	Repo          string
	Branch        string
	AgentConfig   json.RawMessage
	SandboxID     string
	Phase         string
	Status        string
	HeadSHA       string
	PRURL         string
	PRNumber      int
	Evidence      json.RawMessage
	Turns         int
	DeliveryMode  string
	FailureReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CanceledAt    *time.Time
}

const agentSessionColumns = `id, workspace_id, repo, branch, agent_config, sandbox_id,
	phase, status, head_sha, pr_url, pr_number, evidence, turns, delivery_mode,
	failure_reason, created_at, updated_at, canceled_at`

func scanAgentSession(row pgx.Row) (AgentSession, error) {
	var s AgentSession
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.Repo, &s.Branch, &s.AgentConfig,
		&s.SandboxID, &s.Phase, &s.Status, &s.HeadSHA, &s.PRURL, &s.PRNumber,
		&s.Evidence, &s.Turns, &s.DeliveryMode, &s.FailureReason,
		&s.CreatedAt, &s.UpdatedAt, &s.CanceledAt)
	return s, err
}

// CreateAgentSession mints the only valid agent-session id kind and persists
// the row in creating phase before any external OpenFGA/OpenSandbox mutation.
func (s *PGStore) CreateAgentSession(ctx context.Context, in AgentSession) (AgentSession, error) {
	in.ID = ids.New(ids.AgentSession)
	if len(in.AgentConfig) == 0 {
		in.AgentConfig = json.RawMessage(`{}`)
	}
	row, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		INSERT INTO agent_sessions (id, workspace_id, repo, branch, agent_config, phase, status)
		VALUES ($1, $2, $3, $4, $5, 'creating', '')
		RETURNING `+agentSessionColumns,
		in.ID, in.WorkspaceID, in.Repo, in.Branch, in.AgentConfig))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return row, nil
}

func (s *PGStore) GetAgentSession(ctx context.Context, id string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx,
		`SELECT `+agentSessionColumns+` FROM agent_sessions WHERE id=$1`, id))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

func (s *PGStore) ListAgentSessions(ctx context.Context, workspaceID string) ([]AgentSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions WHERE workspace_id=$1 ORDER BY created_at DESC, id DESC`, workspaceID)
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

// SetAgentSessionLifecycle advances the durable session state. canceled=true
// stamps canceled_at exactly once; every transition advances updated_at.
func (s *PGStore) SetAgentSessionLifecycle(ctx context.Context, id, sandboxID, phase, status string, canceled bool) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET sandbox_id = CASE WHEN $2 <> '' THEN $2 ELSE sandbox_id END,
		    phase=$3, status=$4, updated_at=now(),
		    canceled_at=CASE WHEN $5 THEN COALESCE(canceled_at, now()) ELSE canceled_at END
		WHERE id=$1
		RETURNING `+agentSessionColumns, id, sandboxID, phase, status, canceled))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// ListAgentSessionsByPhases returns every session across all workspaces in any
// of the given phases, oldest first. It powers the trusted background Completer
// loop (ADR047 D4) that finalizes running sessions; it performs no authorization
// and must never be reachable from a tenant-facing verb.
func (s *PGStore) ListAgentSessionsByPhases(ctx context.Context, phases []string) ([]AgentSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions WHERE phase = ANY($1) ORDER BY updated_at ASC, id ASC`, phases)
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

// RecordAgentSessionDispatch stamps a newly dispatched turn: it advances the
// phase/status, adopts a fresh sandbox id when re-dispatched, records the
// delivery mode ("resume" or "redispatch"), and increments the turn counter.
func (s *PGStore) RecordAgentSessionDispatch(ctx context.Context, id, sandboxID, phase, status, deliveryMode string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET sandbox_id = CASE WHEN $2 <> '' THEN $2 ELSE sandbox_id END,
		    phase=$3, status=$4, delivery_mode=$5, turns=turns+1, updated_at=now()
		WHERE id=$1
		RETURNING `+agentSessionColumns, id, sandboxID, phase, status, deliveryMode))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// FinalizeAgentSession records a terminal turn outcome. On success it stores the
// pushed head SHA, the opened draft PR, and the bounded evidence extract; on
// failure it stores a named reason. Evidence is left untouched when nil so a
// failure does not erase a prior successful turn's evidence.
func (s *PGStore) FinalizeAgentSession(ctx context.Context, id, phase, headSHA, prURL string, prNumber int, evidence json.RawMessage, failureReason string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET phase=$2, status=$3,
		    head_sha = CASE WHEN $4 <> '' THEN $4 ELSE head_sha END,
		    pr_url   = CASE WHEN $5 <> '' THEN $5 ELSE pr_url END,
		    pr_number = CASE WHEN $6 <> 0 THEN $6 ELSE pr_number END,
		    evidence = CASE WHEN $7::jsonb IS NOT NULL THEN $7::jsonb ELSE evidence END,
		    failure_reason=$8, updated_at=now()
		WHERE id=$1
		RETURNING `+agentSessionColumns,
		id, phase, phase, headSHA, prURL, prNumber, nullableJSON(evidence), failureReason))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

func nullableJSON(v json.RawMessage) any {
	if len(v) == 0 {
		return nil
	}
	return []byte(v)
}

// AppendAgentSessionTranscript idempotently persists teed transcript parts
// (ADR047 D9). The append is keyed by the driver's emission ordinal
// (session_id, seq), so re-teeing the same part — from another gateway replica
// or a re-attach that re-reads the driver's replayed history — is a no-op via
// ON CONFLICT DO NOTHING, never a duplicate. A missing session (its row cascaded
// away) surfaces as ErrNotFound rather than a silent FK error.
func (s *PGStore) AppendAgentSessionTranscript(ctx context.Context, sessionID string, parts []AgentSessionTranscriptPart) error {
	if len(parts) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, p := range parts {
		// part is a text column (verbatim bytes, not canonicalized jsonb): bind it
		// as a string so pgx never re-encodes it as bytea.
		part := string(p.Part)
		if part == "" {
			part = "null"
		}
		batch.Queue(`
			INSERT INTO agent_session_transcripts (session_id, seq, turn, part)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (session_id, seq) DO NOTHING`, sessionID, p.Seq, p.Turn, part)
	}
	results := s.Pool.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()
	for range parts {
		if _, err := results.Exec(); err != nil {
			return classify("agent session transcript", err)
		}
	}
	return nil
}

// AgentSessionTranscript returns a session's stored parts in emission order,
// strictly after afterSeq (pass -1 for the whole transcript). It is the replay
// source for reattach and for terminal-session history (ADR047 D9): it reads
// the durable store, never the sandbox, so it works after the sandbox is gone.
func (s *PGStore) AgentSessionTranscript(ctx context.Context, sessionID string, afterSeq int64) ([]AgentSessionTranscriptPart, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT seq, turn, part, created_at
		FROM agent_session_transcripts
		WHERE session_id=$1 AND seq > $2
		ORDER BY seq ASC`, sessionID, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSessionTranscriptPart, 0)
	for rows.Next() {
		var p AgentSessionTranscriptPart
		if err := rows.Scan(&p.Seq, &p.Turn, &p.Part, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AgentSessionTranscriptMaxSeq returns the highest stored seq for a session and
// whether any part exists. The gateway uses it to skip driver-replayed parts it
// has already persisted, so its live tee resumes exactly where the store ends.
func (s *PGStore) AgentSessionTranscriptMaxSeq(ctx context.Context, sessionID string) (int64, bool, error) {
	var maxSeq *int64
	if err := s.Pool.QueryRow(ctx,
		`SELECT max(seq) FROM agent_session_transcripts WHERE session_id=$1`, sessionID).Scan(&maxSeq); err != nil {
		return 0, false, err
	}
	if maxSeq == nil {
		return -1, false, nil
	}
	return *maxSeq, true, nil
}

// PruneAgentSessionTranscripts deletes parts older than the cutoff, returning
// the number removed. It rides the same daily retention sweep as audit data
// (BEX_AUDIT_RETENTION_DAYS lineage); the ON DELETE CASCADE already clears a
// deleted session's parts, so this only bounds long-lived sessions' history.
func (s *PGStore) PruneAgentSessionTranscripts(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM agent_session_transcripts WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteAgentSession removes a row whose first-class OpenFGA parent tuple could
// not be established. It is compensation for the pre-sandbox create path, not
// a public lifecycle verb (Cancel preserves the audit/history row).
func (s *PGStore) DeleteAgentSession(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM agent_sessions WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent session: %w", ErrNotFound)
	}
	return nil
}
