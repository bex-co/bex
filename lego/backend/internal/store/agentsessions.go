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
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// MaxAgentSessionPromptBytes caps all durable user intent for one session.
// Individual prompts are already limited to 100 KiB at the API and schema;
// this aggregate bound prevents an unbounded number of valid turns from
// growing Postgres independently of the assistant-transcript quota.
const MaxAgentSessionPromptBytes = 8 << 20

var (
	ErrAgentSessionPromptQuota = fmt.Errorf("agent session prompt byte quota reached: %w", ErrConflict)
	ErrAgentSessionTurnState   = fmt.Errorf("agent session cannot accept another turn in its current state: %w", ErrConflict)
)

// AgentSessionTranscriptPart is one durable UI-message-stream part (ADR047 D9,
// w3/m43, amended by w5/m71). Seq is the store-allocated monotonic replay
// cursor; PartIndex is the driver's turn-local emission ordinal. Part is the
// verbatim `data:` payload forwarded to a Vercel AI SDK client.
type AgentSessionTranscriptPart struct {
	Seq       int64
	PartIndex int64
	Turn      int
	Part      json.RawMessage
	CreatedAt time.Time
}

// AgentSessionTurn is the durable user intent for one accepted agent turn.
// Assistant output stays in AgentSessionTranscriptPart so the gateway can replay
// its exact wire bytes; this row supplies the role and completeness facts the
// byte stream itself cannot encode.
type AgentSessionTurn struct {
	SessionID           string
	Turn                int
	Prompt              string
	DeliveryMode        string
	TranscriptComplete  bool
	TranscriptTruncated bool
	TruncationReason    string
	CreatedAt           time.Time
	CompletedAt         *time.Time
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
	// Hibernation (ADR059 D2/D3, w2/m68). Pinned removes the retention delete
	// edge; SnapshotRef is the object-storage key (empty ⇒ no durable snapshot);
	// SnapshotBytes is the storage-metering + quota dimension; HibernatedAt/
	// RetainUntil drive the retention sweep (NULL while Active).
	Pinned        bool
	SnapshotRef   string
	SnapshotBytes int64
	SnapshotSHA   string
	HibernatedAt  *time.Time
	RetainUntil   *time.Time
	// ArchivedAt is the ADR065 D1 archive flag: LIST-state, orthogonal to phase.
	// NULL ⇒ the session is in the working set; set ⇒ it is archived (hidden from
	// the default list, mutation verbs refused) but still viewable.
	ArchivedAt *time.Time
	// InitialPrompt is create-only input used to atomically insert turn 1 with
	// the session row. It is not an agent_sessions column and is never scanned.
	InitialPrompt string
}

const agentSessionColumns = `id, workspace_id, repo, branch, agent_config, sandbox_id,
	phase, status, head_sha, pr_url, pr_number, evidence, turns, delivery_mode,
	failure_reason, created_at, updated_at, canceled_at,
	pinned, snapshot_ref, snapshot_bytes, snapshot_sha, hibernated_at, retain_until,
	archived_at`

func scanAgentSession(row pgx.Row) (AgentSession, error) {
	var s AgentSession
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.Repo, &s.Branch, &s.AgentConfig,
		&s.SandboxID, &s.Phase, &s.Status, &s.HeadSHA, &s.PRURL, &s.PRNumber,
		&s.Evidence, &s.Turns, &s.DeliveryMode, &s.FailureReason,
		&s.CreatedAt, &s.UpdatedAt, &s.CanceledAt,
		&s.Pinned, &s.SnapshotRef, &s.SnapshotBytes, &s.SnapshotSHA,
		&s.HibernatedAt, &s.RetainUntil, &s.ArchivedAt)
	return s, err
}

// CreateAgentSession mints the only valid agent-session id kind and persists
// the row in creating phase before any external OpenFGA/OpenSandbox mutation.
func (s *PGStore) CreateAgentSession(ctx context.Context, in AgentSession) (AgentSession, error) {
	in.ID = ids.New(ids.AgentSession)
	if len(in.AgentConfig) == 0 {
		in.AgentConfig = json.RawMessage(`{}`)
	}
	var row AgentSession
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		row, err = scanAgentSession(tx.QueryRow(ctx, `
			INSERT INTO agent_sessions (id, workspace_id, repo, branch, agent_config, phase, status, turns)
			VALUES ($1, $2, $3, $4, $5, 'creating', '', CASE WHEN $6 <> '' THEN 1 ELSE 0 END)
			RETURNING `+agentSessionColumns,
			in.ID, in.WorkspaceID, in.Repo, in.Branch, in.AgentConfig, in.InitialPrompt))
		if err != nil {
			return err
		}
		if in.InitialPrompt != "" {
			_, err = tx.Exec(ctx, `
				INSERT INTO agent_session_turns (session_id, turn, prompt, delivery_mode)
				VALUES ($1, 1, $2, '')`, in.ID, in.InitialPrompt)
		}
		return err
	})
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

// BeginAgentSessionTurn atomically records an accepted follow-up prompt and
// moves the session into its asynchronous dispatch phase. The public turn count
// advances here, with prompt acceptance, so provisioning failure cannot erase
// or renumber what the user submitted.
func (s *PGStore) BeginAgentSessionTurn(ctx context.Context, id, prompt, deliveryMode, phase, status string) (AgentSession, error) {
	var out AgentSession
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var turns int
		var currentPhase string
		var archivedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT turns, phase, archived_at FROM agent_sessions WHERE id=$1 FOR UPDATE`, id).Scan(&turns, &currentPhase, &archivedAt); err != nil {
			return err
		}
		// Recheck after taking the row lock. Two concurrent Steers can both pass
		// the service's optimistic Get; only the first may persist a prompt.
		if archivedAt != nil || (currentPhase != "completed" && currentPhase != "failed") {
			return ErrAgentSessionTurnState
		}
		if err := enforceAgentSessionPromptQuota(ctx, tx, id, prompt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_session_turns (session_id, turn, prompt, delivery_mode)
			VALUES ($1, $2, $3, $4)`, id, turns+1, prompt, deliveryMode); err != nil {
			return err
		}
		var err error
		out, err = scanAgentSession(tx.QueryRow(ctx, `
			UPDATE agent_sessions
			SET sandbox_id='', phase=$2, status=$3, turns=turns+1,
			    failure_reason='', updated_at=now()
			WHERE id=$1
			RETURNING `+agentSessionColumns, id, phase, status))
		return err
	})
	if err != nil {
		if errors.Is(err, ErrAgentSessionPromptQuota) || errors.Is(err, ErrAgentSessionTurnState) {
			return AgentSession{}, err
		}
		return AgentSession{}, classify("agent session turn", err)
	}
	return out, nil
}

func enforceAgentSessionPromptQuota(ctx context.Context, tx pgx.Tx, sessionID, prompt string) error {
	var stored int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(sum(octet_length(prompt)), 0)
		FROM agent_session_turns WHERE session_id=$1`, sessionID).Scan(&stored); err != nil {
		return err
	}
	if stored+int64(len(prompt)) > MaxAgentSessionPromptBytes {
		return ErrAgentSessionPromptQuota
	}
	return nil
}

// AgentSessionTurns returns the durable user intents in chronological order.
// Prompt input is capped individually and by MaxAgentSessionPromptBytes across
// the session; callers still stream assistant parts page-wise.
func (s *PGStore) AgentSessionTurns(ctx context.Context, sessionID string) ([]AgentSessionTurn, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT session_id, turn, prompt, delivery_mode,
		       transcript_complete, transcript_truncated, truncation_reason,
		       created_at, completed_at
		FROM agent_session_turns WHERE session_id=$1 ORDER BY turn`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSessionTurn, 0)
	for rows.Next() {
		var turn AgentSessionTurn
		if err := rows.Scan(&turn.SessionID, &turn.Turn, &turn.Prompt, &turn.DeliveryMode,
			&turn.TranscriptComplete, &turn.TranscriptTruncated, &turn.TruncationReason,
			&turn.CreatedAt, &turn.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, turn)
	}
	return out, rows.Err()
}

// CompleteAgentSessionTurn records whether the persisted assistant transcript
// is complete. A false complete value is intentional: the row still preserves
// the accepted user prompt while making loss/truncation visible to consumers.
func (s *PGStore) CompleteAgentSessionTurn(ctx context.Context, sessionID string, turn int, complete, truncated bool, reason string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_turns
		SET transcript_complete=$3, transcript_truncated=$4,
		    truncation_reason=$5, completed_at=now()
		WHERE session_id=$1 AND turn=$2`, sessionID, turn, complete, truncated, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent session turn: %w", ErrNotFound)
	}
	return nil
}

// Archived-membership selector for ListAgentSessions (ADR065 D3). The zero
// value excludes archived rows — the default working set.
type ArchivedFilter int

const (
	ArchivedExclude ArchivedFilter = iota // archived_at IS NULL (default)
	ArchivedOnly                          // archived_at IS NOT NULL
	ArchivedInclude                       // no predicate — the full history
)

// AgentSessionListQuery is the ADR065 D3 filter + keyset-page contract for the
// workspace session list. Cursor is the id of the prior page's last item
// (Render's cursor idiom); an unknown or foreign cursor yields an empty page.
// Limit must be normalized (>0) by the caller — the store never substitutes a
// default, so an unbounded read is impossible by construction.
type AgentSessionListQuery struct {
	Archived      ArchivedFilter
	Phases        []string
	Repo          string
	CreatedBefore time.Time
	CreatedAfter  time.Time
	Cursor        string
	Limit         int
}

// ListAgentSessions returns one filtered page of a workspace's sessions,
// newest first (created_at DESC, id DESC — the order the keyset cursor pages).
// The default query (zero filters) is the unarchived working set.
func (s *PGStore) ListAgentSessions(ctx context.Context, workspaceID string, q AgentSessionListQuery) ([]AgentSession, error) {
	if q.Limit <= 0 {
		return nil, fmt.Errorf("agent session list: limit must be positive")
	}
	sql := `SELECT ` + agentSessionColumns + ` FROM agent_sessions WHERE workspace_id=$1`
	args := []any{workspaceID}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	switch q.Archived {
	case ArchivedOnly:
		sql += ` AND archived_at IS NOT NULL`
	case ArchivedInclude:
	default:
		sql += ` AND archived_at IS NULL`
	}
	if len(q.Phases) > 0 {
		sql += ` AND phase = ANY(` + arg(q.Phases) + `)`
	}
	if q.Repo != "" {
		sql += ` AND repo = ` + arg(q.Repo)
	}
	if !q.CreatedBefore.IsZero() {
		sql += ` AND created_at < ` + arg(q.CreatedBefore)
	}
	if !q.CreatedAfter.IsZero() {
		sql += ` AND created_at > ` + arg(q.CreatedAfter)
	}
	if q.Cursor != "" {
		// Keyset: strictly after the cursor row in (created_at DESC, id DESC)
		// order. The subquery is workspace-scoped so a foreign/unknown cursor is
		// an empty page (Render's end-of-list behavior), never an ordering oracle.
		c := arg(q.Cursor)
		sql += ` AND (created_at, id) < (SELECT created_at, id FROM agent_sessions WHERE id = ` + c + ` AND workspace_id = $1)`
	}
	sql += ` ORDER BY created_at DESC, id DESC LIMIT ` + arg(q.Limit)
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

// SetAgentSessionArchived flips the ADR065 D1 archive flag. Idempotent both
// ways: re-archiving keeps the original archived_at (the first archive time is
// the honest one), and unarchiving a working-set row is a no-op. It never
// touches phase/status — archive is list-state, not lifecycle-state.
func (s *PGStore) SetAgentSessionArchived(ctx context.Context, id string, archived bool) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET archived_at = CASE WHEN $2 THEN COALESCE(archived_at, now()) ELSE NULL END,
		    updated_at = now()
		WHERE id=$1
		RETURNING `+agentSessionColumns, id, archived))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
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

// SetAgentSessionFailure terminalizes a still-active session from a background
// dispatch/resume/steer failure with a named reason. Unlike
// SetAgentSessionLifecycle it records the reason in failure_reason (the field the
// dashboard's failed-session callout reads) and sets status to the terminal
// 'failed', so a provisioning failure surfaces identically to a driver-reported
// failure via FinalizeAgentSession (w5/m80 t005, closes w5/048). It preserves
// SetAgentSessionLifecycle's conditional sandbox_id write so an egress-transition
// failure still records the just-canceled sandbox for the reaper. The caller
// (setLifecycleIfActive) guards against clobbering a settled/canceling session.
func (s *PGStore) SetAgentSessionFailure(ctx context.Context, id, sandboxID, reason string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET sandbox_id = CASE WHEN $2 <> '' THEN $2 ELSE sandbox_id END,
		    phase='failed', status='failed', failure_reason=$3, updated_at=now()
		WHERE id=$1
		RETURNING `+agentSessionColumns, id, sandboxID, reason))
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

// ListTerminalAgentSessionsForPush returns completed/failed sessions updated at
// or after `since`, across all workspaces, for the push worker's agent-terminal
// projection (w11/m6 t005). Bounded by the recency window so the scan stays
// cheap; dedup to one push per (session, phase) is the notification's
// source_event_key ON CONFLICT, not this query. Like ListAgentSessionsByPhases
// it performs no authorization and is a trusted background read only.
func (s *PGStore) ListTerminalAgentSessionsForPush(ctx context.Context, since time.Time) ([]AgentSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions
		WHERE phase = ANY($1) AND updated_at >= $2
		ORDER BY updated_at ASC, id ASC`, []string{"completed", "failed"}, since)
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

// ListTerminalAgentSessionsWithSandbox returns terminal sessions (completed/
// failed/canceled) that still carry a sandbox id and were updated at or after
// `since`, oldest first (ADR054 D6). It drives the Completer's deferred-teardown
// reaper: a terminal session keeps its sandbox_id only while its editor SSH is
// held open, so this is the small working set of sandboxes awaiting reclamation.
// The recency bound keeps the scan cheap and skips pre-feature history (whose
// sandboxes are long gone). Trusted background read — no authorization.
func (s *PGStore) ListTerminalAgentSessionsWithSandbox(ctx context.Context, since time.Time) ([]AgentSession, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+agentSessionColumns+`
		FROM agent_sessions
		WHERE phase = ANY($1) AND sandbox_id <> '' AND updated_at >= $2
		ORDER BY updated_at ASC, id ASC`,
		[]string{"completed", "failed", "canceled"}, since)
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

// CountLiveAgentSessionSandboxes counts the workspace's sessions currently in a
// live phase (`phases`) that hold a sandbox id — the concurrent live-sandbox
// working set the ADR059 D6 per-workspace cap bounds. A terminal session in its
// idle grace still carries a sandbox_id but is not in a live phase, so it does
// not count against the create/steer/resume cap. Trusted read used by the
// dispatch-time admission check; a small best-effort overshoot is acceptable
// (the dispatch CAS and reconcile bound it), so it takes no lock.
func (s *PGStore) CountLiveAgentSessionSandboxes(ctx context.Context, workspaceID string, phases []string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM agent_sessions
		WHERE workspace_id = $1 AND phase = ANY($2) AND sandbox_id <> ''`,
		workspaceID, phases).Scan(&n)
	return n, err
}

// ClearAgentSessionSandbox blanks a session's sandbox_id after its sandbox has
// been torn down (ADR054 D6), dropping it from the deferred-teardown reaper's
// working set and releasing the live-sandbox unique index. It never changes
// phase/status — the session stays terminal — and is a no-op if already blank.
func (s *PGStore) ClearAgentSessionSandbox(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE agent_sessions SET sandbox_id = '', updated_at = now() WHERE id = $1 AND sandbox_id <> ''`, id)
	return err
}

// ClaimAgentSessionForHibernation atomically claims a finished session for the
// hibernation reclaim (ADR059 D3), moving it from a terminal phase holding a
// sandbox into the transient `hibernating` phase so exactly one Completer
// replica runs the snapshot. It is CAS-guarded like the reap path: only a
// completed/failed row with a live sandbox_id matches, so a concurrent Cancel or
// a second replica finds no row (ErrNotFound) and backs off. Returns the claimed
// row (its sandbox_id still set — the snapshot exec needs it).
func (s *PGStore) ClaimAgentSessionForHibernation(ctx context.Context, id string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET phase='hibernating', status='hibernating', updated_at=now()
		WHERE id=$1 AND sandbox_id <> '' AND phase IN ('completed','failed')
		RETURNING `+agentSessionColumns, id))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// HibernateAgentSession records a durable snapshot and moves the session to the
// `hibernated` state (ADR059 D3): it stores the object-storage ref + size +
// digest, stamps hibernated_at, and sets the retention deadline (NULL for a
// pinned row). It deliberately KEEPS sandbox_id so the Completer can terminate
// the (still-live) pod after this durable write; the Completer clears it once the
// pod is gone (ClearAgentSessionSandbox). Guarded to the `hibernating` claim so a
// stale writer can't hibernate a resurrected session.
func (s *PGStore) HibernateAgentSession(ctx context.Context, id, snapshotRef string, snapshotBytes int64, snapshotSHA string, retainUntil time.Time) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET phase='hibernated', status='hibernated',
		    snapshot_ref=$2, snapshot_bytes=$3, snapshot_sha=$4,
		    hibernated_at=now(),
		    retain_until = CASE WHEN pinned THEN NULL ELSE $5::timestamptz END,
		    updated_at=now()
		WHERE id=$1 AND phase='hibernating'
		RETURNING `+agentSessionColumns, id, snapshotRef, snapshotBytes, snapshotSHA, retainUntil))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// BeginRehydrate claims a hibernated session for rehydration on Resume/Steer
// (ADR059 D4), CAS-moving it hibernated → resuming while KEEPING the snapshot
// fields (the fresh sandbox is provisioned in the background; the snapshot is
// only consumed once it is up). A concurrent retention delete or double resume
// finds no `hibernated` row (ErrNotFound). The row's sandbox_id stays empty
// until RehydrateAgentSession adopts the new one.
func (s *PGStore) BeginRehydrate(ctx context.Context, id, prompt, deliveryMode string) (AgentSession, error) {
	var out AgentSession
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var turns int
		if err := tx.QueryRow(ctx, `
			SELECT turns FROM agent_sessions
			WHERE id=$1 AND phase='hibernated' AND snapshot_ref <> ''
			FOR UPDATE`, id).Scan(&turns); err != nil {
			return err
		}
		if prompt != "" {
			if err := enforceAgentSessionPromptQuota(ctx, tx, id, prompt); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO agent_session_turns (session_id, turn, prompt, delivery_mode)
				VALUES ($1, $2, $3, $4)`, id, turns+1, prompt, deliveryMode); err != nil {
				return err
			}
		}
		var err error
		out, err = scanAgentSession(tx.QueryRow(ctx, `
			UPDATE agent_sessions
			SET phase='resuming', status='rehydrating',
			    turns=turns + CASE WHEN $2 <> '' THEN 1 ELSE 0 END,
			    updated_at=now()
			WHERE id=$1
			RETURNING `+agentSessionColumns, id, prompt))
		return err
	})
	if err != nil {
		if errors.Is(err, ErrAgentSessionPromptQuota) {
			return AgentSession{}, err
		}
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// AbortRehydrate reverts a rehydrate whose background sandbox provisioning failed
// (ADR059 D4), CAS-moving resuming → hibernated so the snapshot (still present)
// can be retried by a later Resume. Guarded to a still-snapshot-bearing resuming
// row so it can't clobber a session that already rehydrated or was canceled.
func (s *PGStore) AbortRehydrate(ctx context.Context, id string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET phase='hibernated', status='hibernated', updated_at=now()
		WHERE id=$1 AND phase='resuming' AND snapshot_ref <> '' AND sandbox_id=''
		RETURNING `+agentSessionColumns, id))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// RehydrateAgentSession completes a rehydration (ADR059 D4): it adopts the fresh
// sandbox id and advances to the live phase. A Steer's turn counter already
// advanced atomically in BeginRehydrate; a prompt-less Resume does not advance.
// clears the snapshot fields (the blob has been consumed into the new pod — a
// re-hibernation writes a fresh one). Guarded to the `resuming` claim
// BeginRehydrate set, so it can't race a retention delete or a double resume.
func (s *PGStore) RehydrateAgentSession(ctx context.Context, id, sandboxID, phase, status, deliveryMode string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET sandbox_id=$2, phase=$3, status=$4, delivery_mode=$5,
		    snapshot_ref='', snapshot_bytes=0, snapshot_sha='',
		    hibernated_at=NULL, retain_until=NULL, updated_at=now()
		WHERE id=$1 AND phase='resuming' AND sandbox_id=''
		RETURNING `+agentSessionColumns, id, sandboxID, phase, status, deliveryMode))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// SetAgentSessionPinned toggles the never-expire pin (ADR059 D5). Pinning clears
// retain_until (removes the delete edge); unpinning puts a hibernated row back
// on the clock with the supplied deadline (retainUntil is ignored — left NULL —
// while the row is not hibernated). It authorizes nothing; the caller gates it.
func (s *PGStore) SetAgentSessionPinned(ctx context.Context, id string, pinned bool, retainUntil *time.Time) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET pinned=$2,
		    retain_until = CASE
		        WHEN $2 THEN NULL
		        WHEN phase='hibernated' THEN $3::timestamptz
		        ELSE retain_until END,
		    updated_at=now()
		WHERE id=$1
		RETURNING `+agentSessionColumns, id, pinned, retainUntil))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// ListHibernatedForRetention returns unpinned hibernated sessions whose
// retention deadline has passed (ADR059 D5), oldest deadline first — the working
// set the retention sweep reclaims. The sweep deletes ONLY the snapshot object;
// the row and its transcript are kept (ExpireHibernatedAgentSession moves the
// row to canceled + archived). Pinned rows (retain_until NULL) never match.
// Trusted background read; the partial index backs it.
func (s *PGStore) ListHibernatedForRetention(ctx context.Context, now time.Time, limit int) ([]AgentSession, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT `+agentSessionColumns+` FROM agent_sessions
		WHERE phase='hibernated' AND NOT pinned AND snapshot_ref <> ''
		      AND retain_until IS NOT NULL AND retain_until <= $1
		ORDER BY retain_until ASC, id ASC LIMIT $2`, now, limit)
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

// ExpireHibernatedAgentSession finalizes a retention-expired session after its
// snapshot has been deleted from object storage (ADR059 D5): it clears the
// snapshot fields and moves the row to the terminal `canceled` state with a
// reason, keeping the conversation history. It also stamps archived_at (ADR065
// D5): a session whose snapshot aged out is one nobody came back for, so it
// leaves the working set by itself. Guarded to the exact snapshot_ref so it
// can't clobber a row a concurrent Resume already rehydrated.
func (s *PGStore) ExpireHibernatedAgentSession(ctx context.Context, id, snapshotRef string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET phase='canceled', status='canceled', canceled_at=now(),
		    failure_reason='hibernation retention window elapsed',
		    snapshot_ref='', snapshot_bytes=0, snapshot_sha='',
		    retain_until=NULL, archived_at=COALESCE(archived_at, now()), updated_at=now()
		WHERE id=$1 AND phase='hibernated' AND snapshot_ref=$2
		RETURNING `+agentSessionColumns, id, snapshotRef))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// CountPinnedAgentSessions counts a workspace's pinned sessions — the ADR059 D5
// pin-quota dimension. Best-effort like the live cap; no lock.
func (s *PGStore) CountPinnedAgentSessions(ctx context.Context, workspaceID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM agent_sessions WHERE workspace_id=$1 AND pinned`, workspaceID).Scan(&n)
	return n, err
}

// RecordAgentSessionDispatch binds a newly dispatched accepted turn: it advances the
// phase/status, adopts a fresh sandbox id when re-dispatched, records the
// delivery mode ("resume" or "redispatch"). Prompt acceptance already advanced
// the turn counter transactionally.
//
// It is CAS-guarded against a session a concurrent Cancel already took terminal
// (w2/m64): sandbox provisioning runs in the background after the create/steer
// verb has returned, so a Cancel can land while a sandbox is still coming up. The
// `phase NOT IN ('canceling','canceled')` guard makes the record a no-op in that
// race — the update matches no row and returns ErrNotFound — so the caller tears
// the just-created sandbox back down instead of resurrecting the session or
// orphaning the sandbox.
func (s *PGStore) RecordAgentSessionDispatch(ctx context.Context, id, sandboxID, phase, status, deliveryMode string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET sandbox_id = CASE WHEN $2 <> '' THEN $2 ELSE sandbox_id END,
		    phase=$3, status=$4, delivery_mode=$5, failure_reason='', updated_at=now()
		WHERE id=$1 AND phase NOT IN ('canceling', 'canceled')
		RETURNING `+agentSessionColumns, id, sandboxID, phase, status, deliveryMode))
	if err != nil {
		return AgentSession{}, classify("agent session", err)
	}
	return out, nil
}

// FinalizeAgentSession records a terminal turn outcome. On success it stores the
// pushed head SHA, the opened draft PR, and the bounded evidence extract; on
// failure it stores a named reason. Evidence is left untouched when nil so a
// failure does not erase a prior successful turn's evidence. The phase guard is
// the cross-replica CAS: two Completers may observe the same running row, but
// only one can own its terminal transition. `hibernating` is admitted solely so
// the snapshot reaper can unclaim its own transient phase.
func (s *PGStore) FinalizeAgentSession(ctx context.Context, id, phase, headSHA, prURL string, prNumber int, evidence json.RawMessage, failureReason string) (AgentSession, error) {
	out, err := scanAgentSession(s.Pool.QueryRow(ctx, `
		UPDATE agent_sessions
		SET phase=$2, status=$3,
		    head_sha = CASE WHEN $4 <> '' THEN $4 ELSE head_sha END,
		    pr_url   = CASE WHEN $5 <> '' THEN $5 ELSE pr_url END,
		    pr_number = CASE WHEN $6 <> 0 THEN $6 ELSE pr_number END,
		    evidence = CASE WHEN $7::jsonb IS NOT NULL THEN $7::jsonb ELSE evidence END,
		    failure_reason=$8, updated_at=now()
		WHERE id=$1 AND phase IN ('creating','running','resuming','redispatching','hibernating')
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

// MaxAgentSessionTranscriptBytes caps the total payload bytes one session's
// transcript may hold (w1/m65 F10), bounding Postgres growth from
// tenant-controlled agent output and replay memory. Every write path (gateway
// live splice, gateway prompt turn, Completer harvest) seeds its byte counter
// from the already-stored total (AgentSessionTranscriptBytes) and stops
// appending at this cap; the replay read (AgentSessionTranscript) is budgeted
// by the same bound, so it never materializes more than the cap.
const MaxAgentSessionTranscriptBytes = 64 << 20

// AppendAgentSessionTranscript idempotently persists teed transcript parts.
// Driver ordinals restart at zero for every sandbox, so identity is the
// turn-local (session_id, turn, part_index). seq remains a monotonic per-session
// cursor allocated while holding a transaction-scoped advisory lock; concurrent
// gateway replicas therefore cannot collide or reorder the replay cursor.
func (s *PGStore) AppendAgentSessionTranscript(ctx context.Context, sessionID string, parts []AgentSessionTranscriptPart) error {
	if len(parts) == 0 {
		return nil
	}
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// Stable 64-bit hash lock, scoped to this transaction and session only.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, sessionID); err != nil {
			return err
		}
		var seq int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(max(seq), -1) FROM agent_session_transcripts WHERE session_id=$1`, sessionID).Scan(&seq); err != nil {
			return err
		}
		for _, p := range parts {
			partIndex := p.PartIndex
			// Compatibility for internal callers/tests written before part_index:
			// their per-turn Seq was the driver ordinal. New writers set PartIndex.
			if partIndex == 0 && p.Seq > 0 {
				partIndex = p.Seq
			}
			part := string(p.Part)
			if part == "" {
				part = "null"
			}
			tag, err := tx.Exec(ctx, `
				INSERT INTO agent_session_transcripts (session_id, seq, turn, part_index, part)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (session_id, turn, part_index) DO NOTHING`,
				sessionID, seq+1, p.Turn, partIndex, part)
			if err != nil {
				return err
			}
			if tag.RowsAffected() > 0 {
				seq++
			}
		}
		return nil
	})
	return classify("agent session transcript", err)
}

// AgentSessionTranscript returns a session's stored parts in emission order,
// strictly after afterSeq (pass -1 for the whole transcript), capped at
// maxBytes of cumulative payload: rows are scanned in seq order and the read
// stops before the part that would exceed the budget, so a replay never
// materializes more than maxBytes even if the stored transcript ever outgrew
// it. limit additionally bounds the ROW count in SQL (ADR065 D2's poll-shaped
// page; <=0 ⇒ unbounded, the gateway replay's paging is byte-budgeted). It is
// the replay source for reattach and for terminal-session history (ADR047 D9):
// it reads the durable store, never the sandbox, so it works after the sandbox
// is gone.
func (s *PGStore) AgentSessionTranscript(ctx context.Context, sessionID string, afterSeq int64, maxBytes int64, limit int) ([]AgentSessionTranscriptPart, error) {
	sql := `
		SELECT seq, part_index, turn, part, created_at
		FROM agent_session_transcripts
		WHERE session_id=$1 AND seq > $2
		ORDER BY seq ASC`
	args := []any{sessionID, afterSeq}
	if limit > 0 {
		sql += ` LIMIT $3`
		args = append(args, limit)
	}
	rows, err := s.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AgentSessionTranscriptPart, 0)
	var total int64
	for rows.Next() {
		var p AgentSessionTranscriptPart
		if err := rows.Scan(&p.Seq, &p.PartIndex, &p.Turn, &p.Part, &p.CreatedAt); err != nil {
			return nil, err
		}
		if total+int64(len(p.Part)) > maxBytes {
			break
		}
		total += int64(len(p.Part))
		out = append(out, p)
	}
	return out, rows.Err()
}

// AgentSessionTranscriptBytes returns the total stored payload bytes of a
// session's transcript. Write paths seed their cumulative quota counter from
// it so the stored transcript can never exceed MaxAgentSessionTranscriptBytes
// regardless of how many turns or attaches append.
func (s *PGStore) AgentSessionTranscriptBytes(ctx context.Context, sessionID string) (int64, error) {
	var total int64
	if err := s.Pool.QueryRow(ctx,
		`SELECT COALESCE(sum(octet_length(part)), 0) FROM agent_session_transcripts WHERE session_id=$1`,
		sessionID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
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

// AgentSessionTranscriptTurnMaxIndex returns the largest stored local driver
// ordinal for one turn. A reconnect uses it to skip the prefix already replayed
// from Postgres while still accepting a fresh sandbox's ordinal zero.
func (s *PGStore) AgentSessionTranscriptTurnMaxIndex(ctx context.Context, sessionID string, turn int) (int64, bool, error) {
	var maxIndex *int64
	if err := s.Pool.QueryRow(ctx, `
		SELECT max(part_index) FROM agent_session_transcripts WHERE session_id=$1 AND turn=$2`,
		sessionID, turn).Scan(&maxIndex); err != nil {
		return 0, false, err
	}
	if maxIndex == nil {
		return -1, false, nil
	}
	return *maxIndex, true, nil
}

// Transcript retention is deliberate cascade-only (ADR065 D2): a session's
// conversation is PRODUCT data that lives exactly as long as its row — deleted
// only by the D4 delete verb or the workspace cascade, never by an age sweep.
// (An unwired age-based prune previously sat here; removed rather than wired.)

// DeleteAgentSession removes a session row; its transcript parts cascade with
// it. Two callers: the create-path compensation (a row whose OpenFGA parent
// tuple could not be established) and the ADR065 D4 public delete verb — the
// service layer owns the D4 guardrails (fresh authorization, no-live-sandbox
// phase gate, snapshot-blob-before-row ordering). Cancel still preserves the
// audit/history row.
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
