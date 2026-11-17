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

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Sandbox snapshots (w32/m285, docs/ADR104-sandbox-render-parity.md D4): a
// running sandbox's root filesystem saved as an image in its sandbox group,
// which a sandbox create names to start from it.

// The snapshot statuses and kinds, Render's vocabulary.
const (
	SandboxSnapshotCreating  = "creating"
	SandboxSnapshotAvailable = "available"
	SandboxSnapshotFailed    = "failed"

	SandboxSnapshotFilesystem = "filesystem"
	SandboxSnapshotRuntime    = "runtime"
)

// The refusals of CreateSandboxSnapshot.
var (
	// ErrSandboxSnapshotRate is a requester past its captures per minute
	// (Render documents 10 and answers 429).
	ErrSandboxSnapshotRate = errors.New("sandbox snapshot capture rate limit reached")
	// ErrSandboxSnapshotInFlight is a workspace with its limit of captures
	// still creating.
	ErrSandboxSnapshotInFlight = errors.New("sandbox snapshot captures in flight at the workspace's limit")
	// ErrSandboxSnapshotGroupLimit is a group holding its limit of snapshots.
	ErrSandboxSnapshotGroupLimit = errors.New("sandbox group snapshot limit reached")
)

// SandboxSnapshotBusyError is a capture refused because the same sandbox
// already has one in flight: every capture freezes the sandbox while its
// filesystem is committed.
type SandboxSnapshotBusyError struct{ ID string }

func (e *SandboxSnapshotBusyError) Error() string {
	return fmt.Sprintf("sandbox snapshot %s is still being captured from this sandbox", e.ID)
}

// SandboxSnapshot is one row of sandbox_snapshots.
type SandboxSnapshot struct {
	ID              string
	WorkspaceID     string
	SandboxGroupID  string
	SourceSandboxID string
	// SourceRef is the source sandbox's substrate id, which names the
	// repository the commit pushed to.
	SourceRef string
	Name      *string
	Kind      string
	Status    string
	// Plan, NetworkPolicy (Render's object, as JSON), TimeoutSeconds and
	// IdleTimeoutSeconds are the source's, which a restore inherits.
	Plan               string
	NetworkPolicy      []byte
	TimeoutSeconds     int
	IdleTimeoutSeconds int
	// TTLSeconds is the lifetime after the last use; 0 never expires, and
	// ExpiresAt is nil then.
	TTLSeconds  int64
	ExpiresAt   *time.Time
	RequestedBy string
	RequestedAt time.Time
	CapturedAt  *time.Time
	ImageDigest *string
	SizeBytes   *int64
	Error       *string
	// RuntimeID is the substrate's id of the capture, until it is cleaned up.
	RuntimeID *string
	DeletedAt *time.Time
	ReapedAt  *time.Time
}

// NewSandboxSnapshot is a capture to record.
type NewSandboxSnapshot struct {
	WorkspaceID        string
	SandboxGroupID     string
	SourceSandboxID    string
	SourceRef          string
	Name               *string
	Kind               string
	Plan               string
	NetworkPolicy      []byte
	TimeoutSeconds     int
	IdleTimeoutSeconds int
	TTLSeconds         int64
	RequestedBy        string
}

// SandboxSnapshotLimits bound captures; a zero field is no limit.
type SandboxSnapshotLimits struct {
	// PerRequesterPerMinute is how many captures one requester may ask for in
	// any minute, across the workspace.
	PerRequesterPerMinute int
	// InFlight is how many captures a workspace may have creating at once.
	InFlight int
	// PerGroup is how many snapshots (creating or available, not expired) a
	// group may hold.
	PerGroup int
}

const sandboxSnapshotColumns = `id, workspace_id, sandbox_group_id, source_sandbox_id, source_ref, name, kind, status,
	plan, network_policy, timeout_seconds, idle_timeout_seconds, ttl_seconds, expires_at, requested_by, requested_at,
	captured_at, image_digest, size_bytes, error, runtime_id, deleted_at, reaped_at`

// sandboxSnapshotLiveSQL is a snapshot a read still returns at $1: not
// deleted, not expired.
var sandboxSnapshotLiveSQL = sandboxSnapshotLiveSQLAt("$1")

// sandboxSnapshotLiveSQLAt is the same predicate with the time bound to param.
func sandboxSnapshotLiveSQLAt(param string) string {
	return `deleted_at IS NULL AND (expires_at IS NULL OR expires_at > ` + param + `)`
}

func scanSandboxSnapshot(row pgx.Row) (SandboxSnapshot, error) {
	var s SandboxSnapshot
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.SandboxGroupID, &s.SourceSandboxID, &s.SourceRef, &s.Name, &s.Kind, &s.Status,
		&s.Plan, &s.NetworkPolicy, &s.TimeoutSeconds, &s.IdleTimeoutSeconds, &s.TTLSeconds, &s.ExpiresAt, &s.RequestedBy,
		&s.RequestedAt, &s.CapturedAt, &s.ImageDigest, &s.SizeBytes, &s.Error, &s.RuntimeID, &s.DeletedAt, &s.ReapedAt)
	return s, err
}

func collectSandboxSnapshots(rows pgx.Rows, err error) ([]SandboxSnapshot, error) {
	if err != nil {
		return nil, classify("sandbox snapshots", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (SandboxSnapshot, error) { return scanSandboxSnapshot(r) })
}

// CreateSandboxSnapshot records a capture requested at now, in `creating`, and
// checks the limits against the workspace's other captures under one lock, so
// parallel requests cannot each fit under them.
func (s *PGStore) CreateSandboxSnapshot(ctx context.Context, in NewSandboxSnapshot, limits SandboxSnapshotLimits, now time.Time) (snap SandboxSnapshot, err error) {
	now = now.UTC()
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "sandbox-snapshots:"+in.WorkspaceID); err != nil {
			return err
		}
		var busy string
		err := tx.QueryRow(ctx, `SELECT id FROM sandbox_snapshots
			WHERE workspace_id = $1 AND source_sandbox_id = $2 AND status = 'creating' AND deleted_at IS NULL
			LIMIT 1`, in.WorkspaceID, in.SourceSandboxID).Scan(&busy)
		if err == nil {
			return &SandboxSnapshotBusyError{ID: busy}
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		count := func(limit int, refusal error, where string, args ...any) error {
			if limit <= 0 {
				return nil
			}
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM sandbox_snapshots WHERE `+where, args...).Scan(&n); err != nil {
				return err
			}
			if n >= limit {
				return refusal
			}
			return nil
		}
		if in.RequestedBy != "" {
			// Every request counts, whatever became of its snapshot.
			if err := count(limits.PerRequesterPerMinute, ErrSandboxSnapshotRate,
				`workspace_id = $1 AND requested_by = $2 AND requested_at > $3`,
				in.WorkspaceID, in.RequestedBy, now.Add(-time.Minute)); err != nil {
				return err
			}
		}
		if err := count(limits.InFlight, ErrSandboxSnapshotInFlight,
			`workspace_id = $1 AND status = 'creating' AND deleted_at IS NULL`, in.WorkspaceID); err != nil {
			return err
		}
		if err := count(limits.PerGroup, ErrSandboxSnapshotGroupLimit,
			`sandbox_group_id = $2 AND status <> 'failed' AND `+sandboxSnapshotLiveSQL, now, in.SandboxGroupID); err != nil {
			return err
		}
		var expiresAt *time.Time
		if in.TTLSeconds > 0 {
			at := now.Add(time.Duration(in.TTLSeconds) * time.Second)
			expiresAt = &at
		}
		snap, err = scanSandboxSnapshot(tx.QueryRow(ctx, `
			INSERT INTO sandbox_snapshots (id, workspace_id, sandbox_group_id, source_sandbox_id, source_ref, name, kind,
				plan, network_policy, timeout_seconds, idle_timeout_seconds, ttl_seconds, expires_at, requested_by, requested_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			RETURNING `+sandboxSnapshotColumns,
			ids.New(ids.SandboxSnapshot), in.WorkspaceID, in.SandboxGroupID, in.SourceSandboxID, in.SourceRef, in.Name, in.Kind,
			in.Plan, in.NetworkPolicy, in.TimeoutSeconds, in.IdleTimeoutSeconds, in.TTLSeconds, expiresAt, in.RequestedBy, now))
		return err
	})
	var busy *SandboxSnapshotBusyError
	switch {
	case err == nil:
		return snap, nil
	case errors.As(err, &busy), errors.Is(err, ErrSandboxSnapshotRate), errors.Is(err, ErrSandboxSnapshotInFlight),
		errors.Is(err, ErrSandboxSnapshotGroupLimit):
		return SandboxSnapshot{}, err
	}
	// A group deleted meanwhile (23503) classifies as ErrNotFound.
	return SandboxSnapshot{}, classify("sandbox snapshot", err)
}

// SandboxSnapshot returns one snapshot of a group that is neither deleted nor
// expired at now, or ErrNotFound (a snapshot of another group or workspace is
// not found either).
func (s *PGStore) SandboxSnapshot(ctx context.Context, workspaceID, groupID, id string, now time.Time) (SandboxSnapshot, error) {
	snap, err := scanSandboxSnapshot(s.Pool.QueryRow(ctx, `SELECT `+sandboxSnapshotColumns+` FROM sandbox_snapshots
		WHERE `+sandboxSnapshotLiveSQL+` AND id = $2 AND sandbox_group_id = $3 AND workspace_id = $4`,
		now.UTC(), id, groupID, workspaceID))
	if err != nil {
		return SandboxSnapshot{}, classify("sandbox snapshot", err)
	}
	return snap, nil
}

// SandboxSnapshots lists a group's snapshots that are neither deleted nor
// expired at now, newest first, optionally only those of one name and of the
// given statuses.
func (s *PGStore) SandboxSnapshots(ctx context.Context, workspaceID, groupID, name string, statuses []string, now time.Time) ([]SandboxSnapshot, error) {
	return collectSandboxSnapshots(s.Pool.Query(ctx, `SELECT `+sandboxSnapshotColumns+` FROM sandbox_snapshots
		WHERE `+sandboxSnapshotLiveSQL+` AND sandbox_group_id = $2 AND workspace_id = $3
		  AND ($4 = '' OR name = $4)
		  AND (COALESCE(cardinality($5::text[]), 0) = 0 OR status = ANY($5))
		ORDER BY requested_at DESC, id DESC`, now.UTC(), groupID, workspaceID, name, statuses))
}

// NewestAvailableSandboxSnapshot is the group's most recently requested
// snapshot of name that is available and not expired at now, the one a
// restore by name starts from, or ErrNotFound.
func (s *PGStore) NewestAvailableSandboxSnapshot(ctx context.Context, workspaceID, groupID, name string, now time.Time) (SandboxSnapshot, error) {
	snap, err := scanSandboxSnapshot(s.Pool.QueryRow(ctx, `SELECT `+sandboxSnapshotColumns+` FROM sandbox_snapshots
		WHERE `+sandboxSnapshotLiveSQL+` AND sandbox_group_id = $2 AND workspace_id = $3 AND name = $4 AND status = 'available'
		ORDER BY requested_at DESC, id DESC LIMIT 1`, now.UTC(), groupID, workspaceID, name))
	if err != nil {
		return SandboxSnapshot{}, classify("sandbox snapshot", err)
	}
	return snap, nil
}

// SetSandboxSnapshotRuntimeID records the substrate's id of a capture in
// flight; false when the snapshot is no longer creating.
func (s *PGStore) SetSandboxSnapshotRuntimeID(ctx context.Context, id, runtimeID string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE sandbox_snapshots SET runtime_id = $2
		WHERE id = $1 AND status = 'creating' AND deleted_at IS NULL`, id, runtimeID)
	return tag.RowsAffected() == 1, err
}

// ClaimCreatingSandboxSnapshots hands this caller up to limit captures in
// flight that no other replica holds at now, oldest first, and holds each for
// hold. A claim left by a replica that died expires by itself.
func (s *PGStore) ClaimCreatingSandboxSnapshots(ctx context.Context, now time.Time, hold time.Duration, limit int) ([]SandboxSnapshot, error) {
	return collectSandboxSnapshots(s.Pool.Query(ctx, `
		UPDATE sandbox_snapshots SET claimed_until = $1::timestamptz + make_interval(secs => $2)
		WHERE id IN (
			SELECT id FROM sandbox_snapshots
			WHERE status = 'creating' AND deleted_at IS NULL AND (claimed_until IS NULL OR claimed_until <= $1)
			ORDER BY requested_at LIMIT $3 FOR UPDATE SKIP LOCKED)
		RETURNING `+sandboxSnapshotColumns, now.UTC(), hold.Seconds(), limit))
}

// ReleaseSandboxSnapshotClaim gives a capture still in flight back to the next
// sweep.
func (s *PGStore) ReleaseSandboxSnapshotClaim(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sandbox_snapshots SET claimed_until = NULL WHERE id = $1 AND status = 'creating'`, id)
	return err
}

// MarkSandboxSnapshotAvailable settles a capture with its stored image; false
// when the snapshot was settled or deleted meanwhile, so the caller deletes
// the image nothing names.
func (s *PGStore) MarkSandboxSnapshotAvailable(ctx context.Context, id, imageDigest string, sizeBytes int64, capturedAt time.Time) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE sandbox_snapshots
		SET status = 'available', image_digest = $2, size_bytes = $3, captured_at = $4, claimed_until = NULL
		WHERE id = $1 AND status = 'creating' AND deleted_at IS NULL`, id, imageDigest, sizeBytes, capturedAt.UTC())
	return tag.RowsAffected() == 1, err
}

// FailSandboxSnapshot settles a capture in flight as failed with the reason
// its `error` reports; false when it was settled meanwhile.
func (s *PGStore) FailSandboxSnapshot(ctx context.Context, id, reason string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE sandbox_snapshots SET status = 'failed', error = $2, claimed_until = NULL
		WHERE id = $1 AND status = 'creating'`, id, reason)
	return tag.RowsAffected() == 1, err
}

// ClearSandboxSnapshotRuntimeID records that the substrate's object of a
// settled capture is deleted.
func (s *PGStore) ClearSandboxSnapshotRuntimeID(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sandbox_snapshots SET runtime_id = NULL WHERE id = $1 AND status <> 'creating'`, id)
	return err
}

// UseSandboxSnapshot restarts an available snapshot's lifetime at at, when a
// sandbox is created from it: Render resets the expiration to ttlSeconds
// after each use. A snapshot that never expires is left alone.
func (s *PGStore) UseSandboxSnapshot(ctx context.Context, id string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sandbox_snapshots SET expires_at = $2::timestamptz + make_interval(secs => ttl_seconds)
		WHERE id = $1 AND status = 'available' AND ttl_seconds > 0 AND `+sandboxSnapshotLiveSQLAt("$2"), id, at.UTC())
	return err
}

// SandboxSnapshotDeletion is what DeleteSandboxSnapshot did.
type SandboxSnapshotDeletion int

const (
	// SandboxSnapshotDeleted: the snapshot was marked deleted now.
	SandboxSnapshotDeleted SandboxSnapshotDeletion = iota
	// SandboxSnapshotGone: it was already deleted, or has expired (Render
	// answers 204 for both).
	SandboxSnapshotGone
	// SandboxSnapshotStillCreating: its capture is in flight, which Render
	// refuses to delete.
	SandboxSnapshotStillCreating
)

// DeleteSandboxSnapshot marks one snapshot of a group deleted at now, as
// Render's delete-sandbox-snapshot: it leaves every read and can start no
// sandbox. A snapshot of another group or workspace, or none, is ErrNotFound.
// The row of a snapshot deleted now is returned for its image and its meter.
func (s *PGStore) DeleteSandboxSnapshot(ctx context.Context, workspaceID, groupID, id string, now time.Time) (snap SandboxSnapshot, did SandboxSnapshotDeletion, err error) {
	now = now.UTC()
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		snap, err = scanSandboxSnapshot(tx.QueryRow(ctx, `SELECT `+sandboxSnapshotColumns+` FROM sandbox_snapshots
			WHERE id = $1 AND sandbox_group_id = $2 AND workspace_id = $3 FOR UPDATE`, id, groupID, workspaceID))
		if err != nil {
			return err
		}
		switch {
		case snap.DeletedAt != nil || snap.ExpiresAt != nil && !snap.ExpiresAt.After(now):
			did = SandboxSnapshotGone
			return nil
		case snap.Status == SandboxSnapshotCreating:
			did = SandboxSnapshotStillCreating
			return nil
		}
		did = SandboxSnapshotDeleted
		snap.DeletedAt = &now
		_, err = tx.Exec(ctx, `UPDATE sandbox_snapshots SET deleted_at = $2 WHERE id = $1`, id, now)
		return err
	})
	if err != nil {
		return SandboxSnapshot{}, 0, classify("sandbox snapshot", err)
	}
	return snap, did, nil
}

// EndGroupSandboxSnapshots marks every snapshot of one group, or of the whole
// workspace when groupID is empty, deleted at now and fails the captures in
// flight, when the group or the workspace is deleted. It returns the rows that
// still hold something outside the database: an image not yet reaped, or a
// substrate object. A snapshot deleted earlier whose image is still in the
// registry is included, so that delete's failure is retried here.
func (s *PGStore) EndGroupSandboxSnapshots(ctx context.Context, workspaceID, groupID, reason string, now time.Time) ([]SandboxSnapshot, error) {
	return collectSandboxSnapshots(s.Pool.Query(ctx, `
		UPDATE sandbox_snapshots
		SET deleted_at = COALESCE(deleted_at, $3),
		    status = CASE WHEN status = 'creating' THEN 'failed' ELSE status END,
		    error = CASE WHEN status = 'creating' THEN $4 ELSE error END,
		    claimed_until = NULL
		WHERE workspace_id = $1 AND ($2 = '' OR sandbox_group_id = $2)
		  AND (deleted_at IS NULL OR runtime_id IS NOT NULL OR (image_digest IS NOT NULL AND reaped_at IS NULL))
		RETURNING `+sandboxSnapshotColumns, workspaceID, groupID, now.UTC(), reason))
}

// ReapableSandboxSnapshots returns up to limit snapshots that are deleted or
// expired at now and still hold something outside the database: an image in
// the registry, or the substrate's object of a capture that is settled.
// Oldest first.
func (s *PGStore) ReapableSandboxSnapshots(ctx context.Context, now time.Time, limit int) ([]SandboxSnapshot, error) {
	return collectSandboxSnapshots(s.Pool.Query(ctx, `SELECT `+sandboxSnapshotColumns+` FROM sandbox_snapshots
		WHERE status <> 'creating'
		  AND ((image_digest IS NOT NULL AND reaped_at IS NULL AND (deleted_at IS NOT NULL OR expires_at <= $1))
		    OR runtime_id IS NOT NULL)
		ORDER BY requested_at LIMIT $2`, now.UTC(), limit))
}

// MarkSandboxSnapshotReaped records that a snapshot's image left the registry.
func (s *PGStore) MarkSandboxSnapshotReaped(ctx context.Context, id string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sandbox_snapshots SET reaped_at = $2 WHERE id = $1 AND reaped_at IS NULL`, id, at.UTC())
	return err
}

// StaleCreatingSandboxSnapshots fails the captures still creating that were
// requested before olderThan and returns them, for their cleanup: the capture
// outlived its bound, or its bex-api replica died before the substrate
// accepted it.
func (s *PGStore) StaleCreatingSandboxSnapshots(ctx context.Context, olderThan time.Time, reason string) ([]SandboxSnapshot, error) {
	return collectSandboxSnapshots(s.Pool.Query(ctx, `
		UPDATE sandbox_snapshots SET status = 'failed', error = $2, claimed_until = NULL
		WHERE status = 'creating' AND requested_at < $1
		RETURNING `+sandboxSnapshotColumns, olderThan.UTC(), reason))
}
