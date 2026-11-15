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

// A retained image (ADR047 D12, w22/m413) is the disk of an agent turn a
// person stopped, kept as an image so the session's next turn boots from it.
// The row is the image's handle: it is deleted only once the registry no
// longer holds the image, and every transition is a compare-and-set on its
// state, so an expiry, a cancel and a continuation that race leave one winner.
//
//	saving  → kept     the Completer paused the sandbox and copied its disk
//	kept    → in_use   a dispatch claimed it to boot the next turn's sandbox
//	in_use  → kept     the dispatch let it go (the sandbox did not start for
//	                   a reason that is not the image's)
//	any     → released the image is to be deleted from the registry
//
// A released row is deleted from the registry by the purge, then removed.
const (
	RetainedImageSaving   = "saving"
	RetainedImageKept     = "kept"
	RetainedImageInUse    = "in_use"
	RetainedImageReleased = "released"
)

// A turn's retention outcome (agent_session_turns.retention). A stopped turn
// reads "" until its save has ended; TurnRetentionSaving is what a read says
// for it meanwhile, derived from the image's row and never stored.
const (
	TurnRetentionKept      = "kept"
	TurnRetentionContinued = "continued"
	TurnRetentionLost      = "lost"
	TurnRetentionSaving    = "saving"
)

// Why a retained image was released (agent_session_retained_images.release_reason).
const (
	RetainedReleaseContinued   = "continued"
	RetainedReleaseSuperseded  = "superseded"
	RetainedReleaseExpired     = "expired"
	RetainedReleaseCanceled    = "canceled"
	RetainedReleaseArchived    = "archived"
	RetainedReleaseDeleted     = "deleted"
	RetainedReleaseSaveFailed  = "save_failed"
	RetainedReleaseStorageFull = "storage_limit"
	RetainedReleaseUnusable    = "unusable"
	RetainedReleaseAbandoned   = "abandoned"
	RetainedReleaseNewAgent    = "agent_image_changed"
	// RetainedNotSavedEditorOpen is recorded on a stopped turn only, as the
	// reason it kept nothing: an editor was open on its sandbox, which a Stop
	// must not end, so no image was ever begun (agent_session_turns.retention_reason).
	RetainedNotSavedEditorOpen = "editor_open"
)

// AgentRetainedImage is one retained image's handle.
type AgentRetainedImage struct {
	ID          string
	SessionID   string
	WorkspaceID string
	// Turn is the stopped turn whose disk this is.
	Turn  int
	State string
	// BaseImage is the agent template image the disk ran on.
	BaseImage   string
	ImageDigest string
	// SizeBytes is the compressed image's size, reserved against the
	// workspace's storage cap before the copy; 0 until then.
	SizeBytes     int64
	CreatedAt     time.Time
	SaveDeadline  time.Time
	RetainedUntil *time.Time
	ClaimedUntil  *time.Time
	ReleasedAt    *time.Time
	ReleaseReason string
	PurgeAttempts int
}

const agentRetainedImageColumns = `id, session_id, workspace_id, turn, state, base_image, coalesce(image_digest, ''),
	coalesce(size_bytes, 0), created_at, save_deadline, retained_until, claimed_until, released_at,
	coalesce(release_reason, ''), purge_attempts`

func scanAgentRetainedImage(row pgx.Row) (AgentRetainedImage, error) {
	var r AgentRetainedImage
	err := row.Scan(&r.ID, &r.SessionID, &r.WorkspaceID, &r.Turn, &r.State, &r.BaseImage, &r.ImageDigest,
		&r.SizeBytes, &r.CreatedAt, &r.SaveDeadline, &r.RetainedUntil, &r.ClaimedUntil, &r.ReleasedAt,
		&r.ReleaseReason, &r.PurgeAttempts)
	return r, err
}

// NewAgentRetainedImage is a retention to begin for a stopped turn.
type NewAgentRetainedImage struct {
	ID string
	// BaseImage is the agent template image the sandbox runs on.
	BaseImage string
	// SaveDeadline is when the save lapses: past it the next turn stops
	// waiting for the copy and starts fresh.
	SaveDeadline time.Time
}

// FinalizeAgentSessionRetaining is FinalizeAgentSession for a turn a person
// stopped (phase completed), which also begins the retention of its disk in
// the same transaction: the session becomes steerable at the instant its
// retained image exists as a saving row, so the next turn can never slip in
// between and start without waiting for the save. The row is the claim on
// pausing the sandbox, so two replicas never do. It is inserted only for a
// session that is not archived and whose turn is flagged interrupted, and not
// when one is already live; with none the returned row is nil and the session
// is finalized as FinalizeAgentSession would. ErrNotFound, as for that, when
// the session already left its live phase.
func (s *PGStore) FinalizeAgentSessionRetaining(ctx context.Context, id, headSHA, deliveryNote string, in NewAgentRetainedImage) (AgentSession, TerminalTurnFact, *AgentRetainedImage, error) {
	var out AgentSession
	var fact TerminalTurnFact
	var retained *AgentRetainedImage
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var err error
		if out, fact, err = finalizeAgentSessionTx(ctx, tx, id, "completed", headSHA, deliveryNote, ""); err != nil {
			return err
		}
		row, err := scanAgentRetainedImage(tx.QueryRow(ctx, `
			INSERT INTO agent_session_retained_images (id, session_id, workspace_id, turn, base_image, save_deadline)
			SELECT $1, s.id, s.workspace_id, s.turns, $3, $4
			FROM agent_sessions s
			JOIN agent_session_turns t ON t.session_id = s.id AND t.turn = s.turns
			WHERE s.id = $2 AND s.archived_at IS NULL AND t.interrupted
			ON CONFLICT (session_id) WHERE state <> 'released' DO NOTHING
			RETURNING `+agentRetainedImageColumns,
			in.ID, id, in.BaseImage, in.SaveDeadline))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		retained = &row
		return nil
	})
	if err != nil {
		return AgentSession{}, TerminalTurnFact{}, nil, classify("agent session", err)
	}
	return out, fact, retained, nil
}

// ReserveAgentSessionRetainedImage records a saving image's size before it is
// copied, under the workspace's storage lock that imports and repository
// environment builds take too, refused with *SandboxStorageLimitError when the
// workspace's retained images would pass limitBytes (ADR047 D6). ErrNotFound
// when the retention is no longer saving: it was released meanwhile.
func (s *PGStore) ReserveAgentSessionRetainedImage(ctx context.Context, workspaceID, id string, sizeBytes, limitBytes int64) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockSandboxStorage(ctx, tx, workspaceID); err != nil {
			return err
		}
		var used int64
		if err := tx.QueryRow(ctx, sandboxStorageBytesSQL, workspaceID).Scan(&used); err != nil {
			return err
		}
		if used+sizeBytes > limitBytes {
			return &SandboxStorageLimitError{UsedBytes: used}
		}
		tag, err := tx.Exec(ctx, `UPDATE agent_session_retained_images SET size_bytes = $3
			WHERE id = $1 AND workspace_id = $2 AND state = 'saving'`, id, workspaceID, sizeBytes)
		if err == nil && tag.RowsAffected() == 0 {
			return fmt.Errorf("agent session retained image: %w", ErrNotFound)
		}
		return err
	})
}

// KeepAgentSessionRetainedImage settles a saving retention with the image it
// copied, kept until `until`, and records on the turn that its disk is kept.
// false when the retention is no longer saving (it was released while the
// copy ran), and the image it just wrote is then named by nothing.
func (s *PGStore) KeepAgentSessionRetainedImage(ctx context.Context, id, digest string, until time.Time) (bool, error) {
	kept := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var sessionID string
		var turn int
		err := tx.QueryRow(ctx, `UPDATE agent_session_retained_images
			SET state = 'kept', image_digest = $2, retained_until = $3
			WHERE id = $1 AND state = 'saving'
			RETURNING session_id, turn`, id, digest, until).Scan(&sessionID, &turn)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		kept = true
		_, err = tx.Exec(ctx, `UPDATE agent_session_turns SET retention = $4, retained_until = $3
			WHERE session_id = $1 AND turn = $2`, sessionID, turn, until, TurnRetentionKept)
		return err
	})
	return kept, err
}

// AgentSessionRetainedImage is the session's live retained image: saving,
// kept or in use. ErrNotFound when it has none.
func (s *PGStore) AgentSessionRetainedImage(ctx context.Context, sessionID string) (AgentRetainedImage, error) {
	row, err := scanAgentRetainedImage(s.Pool.QueryRow(ctx, `SELECT `+agentRetainedImageColumns+`
		FROM agent_session_retained_images WHERE session_id = $1 AND state <> 'released'`, sessionID))
	if err != nil {
		return AgentRetainedImage{}, classify("agent session retained image", err)
	}
	return row, nil
}

// AgentSessionRetainedImages lists every retained image handle of the session,
// whatever its state: the Delete verb removes each from the registry before
// the session row, which owns the handles, goes.
func (s *PGStore) AgentSessionRetainedImages(ctx context.Context, sessionID string) ([]AgentRetainedImage, error) {
	rows, err := collectWith(scanAgentRetainedImage)(s.Pool.Query(ctx, `SELECT `+agentRetainedImageColumns+`
		FROM agent_session_retained_images WHERE session_id = $1 ORDER BY created_at`, sessionID))
	if err != nil {
		return nil, fmt.Errorf("list agent session retained images: %w", err)
	}
	return rows, nil
}

// ClaimAgentSessionRetainedImage takes the session's kept image for a
// dispatch that will boot from it: kept and not past its time at now, which
// is decided in this one statement, so an expiry and a claim cannot both win.
// The claim lasts lease; an unfinished one is let go by the sweep.
// ErrNotFound when there is nothing to take.
func (s *PGStore) ClaimAgentSessionRetainedImage(ctx context.Context, sessionID string, now time.Time, lease time.Duration) (AgentRetainedImage, error) {
	row, err := scanAgentRetainedImage(s.Pool.QueryRow(ctx, `
		UPDATE agent_session_retained_images
		SET state = 'in_use', claimed_until = $2::timestamptz + make_interval(secs => $3)
		WHERE session_id = $1 AND state = 'kept' AND retained_until > $2
		RETURNING `+agentRetainedImageColumns, sessionID, now, lease.Seconds()))
	if err != nil {
		return AgentRetainedImage{}, classify("agent session retained image", err)
	}
	return row, nil
}

// UnclaimAgentSessionRetainedImage lets go of a claim whose dispatch did not
// use the image, so a later message can. A claim already released, by a
// cancel or the sweep, stays released.
func (s *PGStore) UnclaimAgentSessionRetainedImage(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE agent_session_retained_images
		SET state = 'kept', claimed_until = NULL WHERE id = $1 AND state = 'in_use'`, id)
	return err
}

// FinishAgentSessionRetainedImages settles a session's retained images once a
// turn has started in a sandbox: the one the turn booted from (usedID, or ""
// for a turn that started fresh) is released as continued, and every other
// live one as superseded, since a turn has since changed the session's disk
// and an older image would carry it back. Their turns record the outcome.
func (s *PGStore) FinishAgentSessionRetainedImages(ctx context.Context, sessionID, usedID string, now time.Time) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if usedID != "" {
			if err := releaseRetainedImagesTx(ctx, tx, `id = $1 AND session_id = $2 AND state = 'in_use'`, RetainedReleaseContinued, now, usedID, sessionID); err != nil {
				return err
			}
		}
		return releaseRetainedImagesTx(ctx, tx, `session_id = $1`, RetainedReleaseSuperseded, now, sessionID)
	})
}

// ReleaseAgentSessionRetainedImage releases one live retained image, whatever
// its state, for reason. false when it was already released or is gone.
func (s *PGStore) ReleaseAgentSessionRetainedImage(ctx context.Context, id, reason string, now time.Time) (bool, error) {
	released := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		n, err := releaseRetainedImagesCount(ctx, tx, `id = $1`, reason, now, id)
		released = n > 0
		return err
	})
	return released, err
}

// ReleaseAgentSessionRetainedImages releases the session's live retained
// image for reason: the Delete verb's, which then deletes the image itself
// before it removes the session row that would take the handle with it.
func (s *PGStore) ReleaseAgentSessionRetainedImages(ctx context.Context, sessionID, reason string, now time.Time) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		return releaseRetainedImagesTx(ctx, tx, `session_id = $1`, reason, now, sessionID)
	})
}

// releaseRetainedImagesTx releases the live retained images matching where,
// whose own placeholders are $1 to $len(args), and records the outcome on
// their turns: continued for a continuation, lost, with the reason, for every
// other. A turn that already continued stays so.
func releaseRetainedImagesTx(ctx context.Context, tx pgx.Tx, where, reason string, now time.Time, args ...any) error {
	_, err := releaseRetainedImagesCount(ctx, tx, where, reason, now, args...)
	return err
}

// releaseRetainedImagesCount is releaseRetainedImagesTx reporting how many
// images it released.
func releaseRetainedImagesCount(ctx context.Context, tx pgx.Tx, where, reason string, now time.Time, args ...any) (int64, error) {
	outcome, lostReason := TurnRetentionLost, reason
	if reason == RetainedReleaseContinued {
		outcome, lostReason = TurnRetentionContinued, ""
	}
	n := len(args)
	sql := fmt.Sprintf(`
		WITH released AS (
		  UPDATE agent_session_retained_images
		  SET state = 'released', released_at = $%d, release_reason = $%d, claimed_until = NULL
		  WHERE (%s) AND state <> 'released'
		  RETURNING session_id, turn
		), turns AS (
		  UPDATE agent_session_turns t SET retention = $%d, retention_reason = $%d
		  FROM released r
		  WHERE t.session_id = r.session_id AND t.turn = r.turn AND t.retention <> 'continued'
		)
		SELECT count(*) FROM released`, n+1, n+2, where, n+3, n+4)
	var count int64
	if err := tx.QueryRow(ctx, sql, append(args, now, reason, outcome, lostReason)...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// ExpireAgentSessionRetainedImages applies the clock to the retained images:
// a kept one past its time is released as expired, a saving one past its
// deadline (its Completer is gone) as abandoned, and a claim whose dispatch
// never finished is let go. It reports how many images it released.
func (s *PGStore) ExpireAgentSessionRetainedImages(ctx context.Context, now time.Time) (int64, error) {
	var released int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE agent_session_retained_images
			SET state = 'kept', claimed_until = NULL WHERE state = 'in_use' AND claimed_until <= $1`, now); err != nil {
			return err
		}
		for _, r := range []struct{ where, reason string }{
			{`state = 'kept' AND retained_until <= $1`, RetainedReleaseExpired},
			{`state = 'saving' AND save_deadline <= $1`, RetainedReleaseAbandoned},
		} {
			// $1 is now in the where; the shared placeholders follow it.
			n, err := releaseRetainedImagesCount(ctx, tx, r.where, r.reason, now, now)
			if err != nil {
				return err
			}
			released += n
		}
		return nil
	})
	return released, err
}

// RetainedImagePurge paces the deletion of released images from the registry.
type RetainedImagePurge struct {
	// Backoff is the wait after a claim, doubled per earlier attempt up to
	// MaxBackoff: the next claim of an image whose delete failed.
	Backoff    time.Duration
	MaxBackoff time.Duration
	Limit      int
	// SessionID, when set, claims only that session's images.
	SessionID string
}

// ClaimAgentSessionRetainedImagePurges claims released images whose delete is
// due and defers each one's next attempt, so one replica deletes each and a
// failed delete backs off.
func (s *PGStore) ClaimAgentSessionRetainedImagePurges(ctx context.Context, now time.Time, p RetainedImagePurge) ([]AgentRetainedImage, error) {
	rows, err := collectWith(scanAgentRetainedImage)(s.Pool.Query(ctx, `
		UPDATE agent_session_retained_images
		SET purge_attempts = purge_attempts + 1,
		    purge_after = $1::timestamptz + least(make_interval(secs => $2) * power(2, purge_attempts), make_interval(secs => $3))
		WHERE id IN (SELECT id FROM agent_session_retained_images
		             WHERE state = 'released' AND (purge_after IS NULL OR purge_after <= $1)
		               AND ($5 = '' OR session_id = $5)
		             ORDER BY released_at LIMIT $4 FOR UPDATE SKIP LOCKED)
		RETURNING `+agentRetainedImageColumns,
		now, p.Backoff.Seconds(), p.MaxBackoff.Seconds(), p.Limit, p.SessionID))
	if err != nil {
		return nil, fmt.Errorf("claim agent session retained image purges: %w", err)
	}
	return rows, nil
}

// DeleteAgentSessionRetainedImage removes a released image's handle once the
// registry no longer holds the image.
func (s *PGStore) DeleteAgentSessionRetainedImage(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM agent_session_retained_images WHERE id = $1 AND state = 'released'`, id)
	return err
}

// AgentRetainedImageIDs lists every retained image of a workspace whose handle
// exists, for the workspace delete to remove from the registry before its rows
// cascade away.
func (s *PGStore) AgentRetainedImageIDs(ctx context.Context, workspaceID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id FROM agent_session_retained_images WHERE workspace_id = $1`, workspaceID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// AgentSessionsSavingRetainedImage reports which of the sessions have a
// retention being saved, so the idle reaper leaves their sandboxes alone.
func (s *PGStore) AgentSessionsSavingRetainedImage(ctx context.Context, sessionIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(sessionIDs) == 0 {
		return out, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT session_id FROM agent_session_retained_images
		WHERE state = 'saving' AND session_id = ANY($1)`, sessionIDs)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// SetAgentSessionTurnStartNote records the sentence a turn adds to its
// delivery note when it finishes: it started from a fresh sandbox although
// the stopped turn's disk was kept.
func (s *PGStore) SetAgentSessionTurnStartNote(ctx context.Context, sessionID string, turn int, note string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE agent_session_turns SET start_note = $3 WHERE session_id = $1 AND turn = $2`, sessionID, turn, note)
	return err
}

// SetAgentSessionTurnRetentionLost records on a stopped turn that its disk was
// not kept, and why, when no image was ever begun for it (an editor was open on
// its sandbox). A turn that already records an outcome keeps it.
func (s *PGStore) SetAgentSessionTurnRetentionLost(ctx context.Context, sessionID string, turn int, reason string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE agent_session_turns SET retention = $4, retention_reason = $3
		WHERE session_id = $1 AND turn = $2 AND retention = ''`, sessionID, turn, reason, TurnRetentionLost)
	return err
}

// ClearAgentSessionRetainedSandbox drops the session's sandbox id once a save
// has ended that sandbox, and only while it is still the session's: a session
// whose next turn already bound another sandbox keeps it.
func (s *PGStore) ClearAgentSessionRetainedSandbox(ctx context.Context, sessionID, sandboxID string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET sandbox_id = '', updated_at = now()
		WHERE id = $1 AND sandbox_id = $2`, sessionID, sandboxID)
	return err
}
