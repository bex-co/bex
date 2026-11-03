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
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

// Attachment caps (ADR095 D8) enforced where the rows are written. The
// per-session caps cover inputs and outputs together; the per-uploader caps
// bound what one member can park unbound in a workspace before attaching it.
const (
	MaxAgentSessionAttachments     = 50
	MaxAgentSessionAttachmentBytes = 250 << 20
	MaxUnboundAttachmentsPerUser   = 50
	MaxUnboundAttachmentBytes      = 250 << 20
)

// AgentAttachmentDirection says who produced a file: the user ("input") or the
// agent ("output").
const (
	AgentAttachmentInput  = "input"
	AgentAttachmentOutput = "output"
)

var (
	// ErrAgentSessionAttachmentQuota refuses a binding that would take a
	// session past MaxAgentSessionAttachments or MaxAgentSessionAttachmentBytes.
	ErrAgentSessionAttachmentQuota = fmt.Errorf("agent session attachment quota reached: %w", ErrConflict)
	// ErrUnboundAttachmentQuota refuses an upload past the uploader's unbound caps.
	ErrUnboundAttachmentQuota = fmt.Errorf("unattached upload quota reached: %w", ErrConflict)
)

// AgentAttachmentInvalidError names the first attachment id a binding could
// not take: missing, deleted, an output, in another workspace or session, or
// another member's unbound upload.
type AgentAttachmentInvalidError struct{ ID string }

func (e *AgentAttachmentInvalidError) Error() string {
	return "agent attachment cannot be attached: " + e.ID
}

// AgentAttachment is one row of agent_session_attachments (ADR095 D1).
type AgentAttachment struct {
	ID          string
	WorkspaceID string
	SessionID   string // "" while unbound
	Turn        int    // 0 while unbound
	Direction   string
	Name        string
	MediaType   string
	SizeBytes   int64
	SHA256      string
	ObjectKey   string
	CreatedBy   string
	CreatedAt   time.Time
	ExpiresAt   *time.Time
}

// TurnAttachment is the metadata a turn keeps for each file it attached.
type TurnAttachment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
}

// AgentAttachmentBinding is the set of uploads a turn attaches. Caller is the
// identity accepting the turn: an unbound upload binds only for its uploader.
type AgentAttachmentBinding struct {
	IDs    []string
	Caller string
}

// AgentAttachmentListQuery filters a session's attachments; Cursor is the last
// id of the previous page.
type AgentAttachmentListQuery struct {
	Direction string
	Turn      int
	Cursor    string
	Limit     int
}

// attachmentPendingTTL bounds how long an upload may stay unconfirmed before
// the sweep treats its object as abandoned.
const attachmentPendingTTL = time.Hour

const agentAttachmentColumns = `id, workspace_id, COALESCE(session_id, ''), COALESCE(turn, 0),
	direction, name, media_type, size_bytes, sha256, object_key, created_by, created_at, expires_at`

func scanAgentAttachment(row pgx.Row) (AgentAttachment, error) {
	var a AgentAttachment
	err := row.Scan(&a.ID, &a.WorkspaceID, &a.SessionID, &a.Turn, &a.Direction, &a.Name,
		&a.MediaType, &a.SizeBytes, &a.SHA256, &a.ObjectKey, &a.CreatedBy, &a.CreatedAt, &a.ExpiresAt)
	return a, err
}

func collectAgentAttachments(rows pgx.Rows, err error) ([]AgentAttachment, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (AgentAttachment, error) {
		return scanAgentAttachment(row)
	})
}

// BeginAgentAttachment records an upload before its object is stored, so an
// object can never exist without a row the sweep can purge. An unbound input
// counts against its uploader's caps; a row with SessionID set (an agent
// output) counts against the session's.
func (s *PGStore) BeginAgentAttachment(ctx context.Context, in AgentAttachment) error {
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var session, turn any
		if in.SessionID != "" {
			session, turn = in.SessionID, in.Turn
			// Take the session lock before the INSERT, so concurrent shares
			// queue here and each counts the others' committed rows.
			if err := lockAgentSession(ctx, tx, in.SessionID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_session_attachments
				(id, workspace_id, session_id, turn, direction, name, media_type,
				 size_bytes, sha256, object_key, state, created_by, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '', $9, 'pending', $10, now() + $11::interval)`,
			in.ID, in.WorkspaceID, session, turn, in.Direction, in.Name, in.MediaType,
			in.SizeBytes, in.ObjectKey, in.CreatedBy, attachmentPendingTTL.String()); err != nil {
			return err
		}
		if in.SessionID != "" {
			return enforceAgentAttachmentQuota(ctx, tx, in.SessionID)
		}
		var count int
		var total int64
		if err := tx.QueryRow(ctx, `
			SELECT count(*), COALESCE(sum(size_bytes), 0) FROM agent_session_attachments
			WHERE workspace_id=$1 AND created_by=$2 AND session_id IS NULL AND state IN ('pending', 'ready')`,
			in.WorkspaceID, in.CreatedBy).Scan(&count, &total); err != nil {
			return err
		}
		if count > MaxUnboundAttachmentsPerUser || total > MaxUnboundAttachmentBytes {
			return ErrUnboundAttachmentQuota
		}
		return nil
	})
	if err != nil && !isAgentAttachmentRefusal(err) {
		return classify("agent attachment", err)
	}
	return err
}

// ReadyAgentAttachment confirms a stored object. An unbound upload then lives
// until expiresAt; a bound one (an output) has no expiry.
func (s *PGStore) ReadyAgentAttachment(ctx context.Context, id, sha256 string, unboundExpiry time.Time) (AgentAttachment, error) {
	a, err := scanAgentAttachment(s.Pool.QueryRow(ctx, `
		UPDATE agent_session_attachments
		SET state='ready', sha256=$2,
		    expires_at = CASE WHEN session_id IS NULL THEN $3::timestamptz END
		WHERE id=$1 AND state='pending'
		RETURNING `+agentAttachmentColumns, id, sha256, unboundExpiry))
	if err != nil {
		return AgentAttachment{}, classify("agent attachment", err)
	}
	return a, nil
}

// GetAgentAttachment returns a ready attachment.
func (s *PGStore) GetAgentAttachment(ctx context.Context, id string) (AgentAttachment, error) {
	a, err := scanAgentAttachment(s.Pool.QueryRow(ctx,
		`SELECT `+agentAttachmentColumns+` FROM agent_session_attachments WHERE id=$1 AND state='ready'`, id))
	if err != nil {
		return AgentAttachment{}, classify("agent attachment", err)
	}
	return a, nil
}

// ListAgentSessionAttachments pages a session's ready attachments oldest first.
func (s *PGStore) ListAgentSessionAttachments(ctx context.Context, sessionID string, q AgentAttachmentListQuery) ([]AgentAttachment, error) {
	query := `SELECT ` + agentAttachmentColumns + ` FROM agent_session_attachments
		WHERE session_id=$1 AND state='ready'`
	args := []any{sessionID}
	if q.Direction != "" {
		args = append(args, q.Direction)
		query += fmt.Sprintf(" AND direction=$%d", len(args))
	}
	if q.Turn > 0 {
		args = append(args, q.Turn)
		query += fmt.Sprintf(" AND turn=$%d", len(args))
	}
	query, args = pageKeyset(query, args, "agent_session_attachments", "created_at", q.Cursor, clampPageLimit(q.Limit), true)
	return collectAgentAttachments(s.Pool.Query(ctx, query, args...))
}

// ListAgentSessionInputs returns every ready input of a session, in the order
// a dispatch copies them into the sandbox.
func (s *PGStore) ListAgentSessionInputs(ctx context.Context, sessionID string) ([]AgentAttachment, error) {
	return collectAgentAttachments(s.Pool.Query(ctx, `
		SELECT `+agentAttachmentColumns+`
		FROM agent_session_attachments
		WHERE session_id=$1 AND state='ready' AND direction='input'
		ORDER BY turn, created_at, id`, sessionID))
}

// RetireAgentAttachment marks a row deleted so the sweep purges its object.
// unboundOnly refuses an attachment some session already holds.
func (s *PGStore) RetireAgentAttachment(ctx context.Context, id string, unboundOnly bool) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_attachments SET state='deleted'
		WHERE id=$1 AND state <> 'deleted' AND (NOT $2 OR session_id IS NULL)`, id, unboundOnly)
	if err != nil {
		return classify("agent attachment", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent attachment: %w", ErrNotFound)
	}
	return nil
}

// ExpireAgentAttachments retires uploads left unconfirmed or unattached past
// expires_at and reports how many it retired.
func (s *PGStore) ExpireAgentAttachments(ctx context.Context, now time.Time) (int, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE agent_session_attachments SET state='deleted'
		WHERE expires_at <= $1 AND state <> 'deleted'
		  AND (state='pending' OR session_id IS NULL)`, now)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ClaimRetiredAgentAttachments returns up to limit deleted rows whose objects
// still need purging, rows never tried first, and stamps the attempt so a row
// whose object keeps failing to delete waits behind the rest instead of
// starving every batch.
func (s *PGStore) ClaimRetiredAgentAttachments(ctx context.Context, limit int, now time.Time) ([]AgentAttachment, error) {
	return collectAgentAttachments(s.Pool.Query(ctx, `
		UPDATE agent_session_attachments SET purge_attempted_at=$2
		WHERE id IN (
			SELECT id FROM agent_session_attachments WHERE state='deleted'
			ORDER BY purge_attempted_at NULLS FIRST, created_at, id
			LIMIT $1 FOR UPDATE SKIP LOCKED)
		RETURNING `+agentAttachmentColumns, limit, now))
}

// PurgeAgentAttachments drops retired rows whose objects are gone.
func (s *PGStore) PurgeAgentAttachments(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM agent_session_attachments WHERE id = ANY($1) AND state='deleted'`, ids)
	return err
}

// retireSessionAttachments and retireWorkspaceAttachments run inside the
// delete that would otherwise detach the rows from their objects.
func retireSessionAttachments(ctx context.Context, tx pgx.Tx, sessionID string) error {
	_, err := tx.Exec(ctx, `UPDATE agent_session_attachments SET state='deleted'
		WHERE session_id=$1 AND state <> 'deleted'`, sessionID)
	return err
}

func retireWorkspaceAttachments(ctx context.Context, tx pgx.Tx, workspaceID string) error {
	_, err := tx.Exec(ctx, `UPDATE agent_session_attachments SET state='deleted'
		WHERE workspace_id=$1 AND state <> 'deleted'`, workspaceID)
	return err
}

// bindAgentAttachments attaches uploads to a session turn inside the turn's
// own transaction, so a refused binding records no turn. Each id must be a
// ready input in the workspace, either the caller's unbound upload or already
// bound to this session (a later turn may refer to an earlier file again; the
// row keeps the turn that introduced it, the turn's snapshot lists it).
func bindAgentAttachments(ctx context.Context, tx pgx.Tx, workspaceID, sessionID string, turn int, binding AgentAttachmentBinding) error {
	if len(binding.IDs) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `
		UPDATE agent_session_attachments
		SET session_id=$1, turn=COALESCE(turn, $2), expires_at=NULL
		WHERE id = ANY($3) AND workspace_id=$4 AND state='ready' AND direction='input'
		  AND (session_id=$1 OR (session_id IS NULL AND created_by=$5))
		RETURNING id`, sessionID, turn, binding.IDs, workspaceID, binding.Caller)
	if err != nil {
		return err
	}
	bound, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range binding.IDs {
		if !slices.Contains(bound, id) {
			return &AgentAttachmentInvalidError{ID: id}
		}
	}
	if err := enforceAgentAttachmentQuota(ctx, tx, sessionID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE agent_session_turns t
		SET attachments = (
			SELECT jsonb_agg(jsonb_build_object('id', a.id, 'name', a.name,
				'mediaType', a.media_type, 'size', a.size_bytes) ORDER BY o.ord)
			FROM unnest($3::text[]) WITH ORDINALITY AS o(id, ord)
			JOIN agent_session_attachments a ON a.id = o.id)
		WHERE t.session_id=$1 AND t.turn=$2`, sessionID, turn, binding.IDs)
	return err
}

// isAgentAttachmentRefusal reports a refusal the service maps to a coded
// error instead of a store failure.
func isAgentAttachmentRefusal(err error) bool {
	var invalid *AgentAttachmentInvalidError
	return errors.As(err, &invalid) || errors.Is(err, ErrAgentSessionAttachmentQuota) || errors.Is(err, ErrUnboundAttachmentQuota)
}

// enforceAgentAttachmentQuota counts the session's live files. The caller must
// already hold the session row (lockAgentSession, the turn's FOR UPDATE, or the
// session's own uncommitted INSERT): concurrent writers — a turn binding
// inputs, the agent sharing outputs — would otherwise each count only their own
// rows under READ COMMITTED and together pass the cap.
func enforceAgentAttachmentQuota(ctx context.Context, tx pgx.Tx, sessionID string) error {
	var count int
	var bytes int64
	if err := tx.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(size_bytes), 0)
		FROM agent_session_attachments WHERE session_id=$1 AND state IN ('pending', 'ready')`, sessionID).Scan(&count, &bytes); err != nil {
		return err
	}
	if count > MaxAgentSessionAttachments || bytes > MaxAgentSessionAttachmentBytes {
		return ErrAgentSessionAttachmentQuota
	}
	return nil
}

// lockAgentSession serializes attachment quota checks on one session. FOR NO
// KEY UPDATE conflicts with itself and with the turn's FOR UPDATE, so quota
// writers queue, but not with the KEY SHARE lock the attachment row's foreign
// key takes — the writer's own INSERT never waits on its own lock.
func lockAgentSession(ctx context.Context, tx pgx.Tx, sessionID string) error {
	var id string
	return tx.QueryRow(ctx, `SELECT id FROM agent_sessions WHERE id=$1 FOR NO KEY UPDATE`, sessionID).Scan(&id)
}
