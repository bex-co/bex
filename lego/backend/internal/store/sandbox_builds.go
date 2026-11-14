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

// Sandbox builds (w2/m358, docs/ADR104-sandbox-render-parity.md § Images): an
// OCI image imported into a sandbox group, which a sandbox create names.

// The build statuses, Render's vocabulary.
const (
	SandboxBuildPending   = "pending"
	SandboxBuildRunning   = "running"
	SandboxBuildSucceeded = "succeeded"
	SandboxBuildFailed    = "failed"
)

// ErrSandboxBuildLimit is an import into a group that already has its limit of
// builds pending or running (Render answers 429).
var ErrSandboxBuildLimit = errors.New("sandbox build limit reached")

// SandboxBuild is one row of sandbox_builds.
type SandboxBuild struct {
	ID               string
	WorkspaceID      string
	SandboxGroupID   string
	Name             string
	SourceRepository string
	SourceImage      string
	SourceReference  string
	SourceDigest     string
	Status           string
	FailureReason    *string
	// FailureCode is Render's machine-readable failureCode (w3/m367); nil
	// unless failed, and for a build that failed before codes were recorded.
	FailureCode *string
	ImageDigest *string
	SizeBytes   *int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	FinishedAt  *time.Time
}

// ErrSandboxImportBudget is a new import refused because the imported image
// bytes of every workspace together would pass the platform budget
// (NewSandboxBuild.PlatformBudgetBytes, w5/m383).
var ErrSandboxImportBudget = errors.New("sandbox import platform budget reached")

// ErrSandboxImportShare is a new import refused because the workspace's own
// imported images would pass its share of the platform budget
// (NewSandboxBuild.WorkspaceShareBytes), so one workspace cannot hold the
// whole budget against everyone else (w5/m383).
var ErrSandboxImportShare = errors.New("sandbox import workspace share reached")

// NewSandboxBuild is an import to record: the resolved source and the name.
type NewSandboxBuild struct {
	WorkspaceID      string
	SandboxGroupID   string
	Name             string
	SourceRepository string
	SourceImage      string
	SourceReference  string
	SourceDigest     string
	// SizeBytes is the compressed size the source manifest declares, stored at
	// once so an import still copying counts against the workspace's storage
	// cap (round 24, S-3). A failed import clears it.
	SizeBytes int64
	// MaxStorageBytes is the workspace's sandbox storage cap (0: none). A new
	// build whose size would take the workspace past it is
	// *SandboxStorageLimitError, checked under a workspace-wide lock so
	// concurrent imports into different groups cannot all pass it.
	MaxStorageBytes int64
	// PlatformBudgetBytes bounds the imported image bytes of every workspace
	// together, stored and in flight (0: none), so imports cannot fill the
	// shared sandbox registry (w5/m383). Past it a new build is
	// ErrSandboxImportBudget, checked under a platform-wide lock taken after
	// the workspace's.
	PlatformBudgetBytes int64
	// WorkspaceShareBytes bounds one workspace's imported image bytes within
	// that budget (0: none); past it a new build is ErrSandboxImportShare.
	WorkspaceShareBytes int64
}

// SandboxStorageLimitError refuses a new build that would take its workspace
// past its sandbox storage cap.
type SandboxStorageLimitError struct{ UsedBytes int64 }

func (e *SandboxStorageLimitError) Error() string {
	return fmt.Sprintf("sandbox storage limit: %d bytes already used", e.UsedBytes)
}

const sandboxBuildColumns = `id, workspace_id, sandbox_group_id, name, source_repository, source_image,
	source_reference, source_digest, status, failure_reason, failure_code, image_digest, size_bytes, created_at, updated_at, finished_at`

func scanSandboxBuild(row pgx.Row) (SandboxBuild, error) {
	var b SandboxBuild
	err := row.Scan(&b.ID, &b.WorkspaceID, &b.SandboxGroupID, &b.Name, &b.SourceRepository, &b.SourceImage,
		&b.SourceReference, &b.SourceDigest, &b.Status, &b.FailureReason, &b.FailureCode, &b.ImageDigest, &b.SizeBytes,
		&b.CreatedAt, &b.UpdatedAt, &b.FinishedAt)
	return b, err
}

// CreateSandboxBuild records an import, or returns the group's existing build
// of the same name and digest that has not failed (created false), as Render
// does. A new build past maxInFlight builds pending or running in the group (0
// disables the limit) is ErrSandboxBuildLimit.
func (s *PGStore) CreateSandboxBuild(ctx context.Context, in NewSandboxBuild, maxInFlight int) (b SandboxBuild, created bool, err error) {
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := lockSandboxBuilds(ctx, tx, in.SandboxGroupID); err != nil {
			return err
		}
		existing, err := scanSandboxBuild(tx.QueryRow(ctx, `SELECT `+sandboxBuildColumns+` FROM sandbox_builds
			WHERE sandbox_group_id = $1 AND name = $2 AND source_digest = $3 AND status <> 'failed' AND deleted_at IS NULL`,
			in.SandboxGroupID, in.Name, in.SourceDigest))
		if err == nil {
			b = existing
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if maxInFlight > 0 {
			var n int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM sandbox_builds
				WHERE sandbox_group_id = $1 AND status IN ('pending', 'running') AND deleted_at IS NULL`, in.SandboxGroupID).Scan(&n); err != nil {
				return err
			}
			if n >= maxInFlight {
				return ErrSandboxBuildLimit
			}
		}
		if (in.MaxStorageBytes > 0 || in.WorkspaceShareBytes > 0) && in.SizeBytes > 0 {
			if err := lockSandboxStorage(ctx, tx, in.WorkspaceID); err != nil {
				return err
			}
		}
		if in.MaxStorageBytes > 0 && in.SizeBytes > 0 {
			var used int64
			if err := tx.QueryRow(ctx, sandboxStorageBytesSQL, in.WorkspaceID).Scan(&used); err != nil {
				return err
			}
			if used+in.SizeBytes > in.MaxStorageBytes {
				return &SandboxStorageLimitError{UsedBytes: used}
			}
		}
		if in.WorkspaceShareBytes > 0 && in.SizeBytes > 0 {
			var own int64
			if err := tx.QueryRow(ctx, `SELECT COALESCE(sum(size_bytes), 0) FROM sandbox_builds
				WHERE workspace_id = $1 AND `+importedBuildSQL, in.WorkspaceID).Scan(&own); err != nil {
				return err
			}
			if own+in.SizeBytes > in.WorkspaceShareBytes {
				return ErrSandboxImportShare
			}
		}
		if in.PlatformBudgetBytes > 0 && in.SizeBytes > 0 {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "sandbox-import-platform"); err != nil {
				return err
			}
			var imported int64
			if err := tx.QueryRow(ctx, platformImportedBytesSQL).Scan(&imported); err != nil {
				return err
			}
			if imported+in.SizeBytes > in.PlatformBudgetBytes {
				return ErrSandboxImportBudget
			}
		}
		b, err = scanSandboxBuild(tx.QueryRow(ctx, `
			INSERT INTO sandbox_builds (id, workspace_id, sandbox_group_id, name, source_repository, source_image, source_reference, source_digest, size_bytes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, 0))
			RETURNING `+sandboxBuildColumns,
			ids.New(ids.SandboxBuild), in.WorkspaceID, in.SandboxGroupID, in.Name,
			in.SourceRepository, in.SourceImage, in.SourceReference, in.SourceDigest, in.SizeBytes))
		created = err == nil
		return err
	})
	if limit := (*SandboxStorageLimitError)(nil); errors.As(err, &limit) || errors.Is(err, ErrSandboxImportBudget) || errors.Is(err, ErrSandboxImportShare) {
		return SandboxBuild{}, false, err
	}
	if err != nil {
		// A group deleted meanwhile (23503) classifies as ErrNotFound.
		return SandboxBuild{}, false, classify("sandbox build", err)
	}
	return b, created, nil
}

// lockSandboxBuilds serializes a group's imports, so the dedupe read and the
// in-flight count see no concurrent insert.
func lockSandboxBuilds(ctx context.Context, tx pgx.Tx, groupID string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "sandbox-builds:"+groupID)
	return err
}

// SandboxBuild returns one build of a group, or ErrNotFound (a build of
// another group or workspace is not found either).
func (s *PGStore) SandboxBuild(ctx context.Context, workspaceID, groupID, id string) (SandboxBuild, error) {
	b, err := scanSandboxBuild(s.Pool.QueryRow(ctx, `SELECT `+sandboxBuildColumns+` FROM sandbox_builds
		WHERE id = $1 AND sandbox_group_id = $2 AND workspace_id = $3 AND deleted_at IS NULL`, id, groupID, workspaceID))
	if err != nil {
		return SandboxBuild{}, classify("sandbox build", err)
	}
	return b, nil
}

// SandboxBuilds lists a group's builds newest first, optionally only those of
// one name and of the given statuses.
func (s *PGStore) SandboxBuilds(ctx context.Context, workspaceID, groupID, name string, statuses []string) ([]SandboxBuild, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+sandboxBuildColumns+` FROM sandbox_builds
		WHERE sandbox_group_id = $1 AND workspace_id = $2 AND deleted_at IS NULL
		  AND ($3 = '' OR name = $3)
		  AND (COALESCE(cardinality($4::text[]), 0) = 0 OR status = ANY($4))
		ORDER BY created_at DESC, id DESC`, groupID, workspaceID, name, statuses)
	if err != nil {
		return nil, classify("sandbox builds", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (SandboxBuild, error) { return scanSandboxBuild(r) })
}

// NewestSucceededSandboxBuild is the group's newest succeeded build of name,
// the one a create by name starts from, or ErrNotFound.
func (s *PGStore) NewestSucceededSandboxBuild(ctx context.Context, workspaceID, groupID, name string) (SandboxBuild, error) {
	b, err := scanSandboxBuild(s.Pool.QueryRow(ctx, `SELECT `+sandboxBuildColumns+` FROM sandbox_builds
		WHERE sandbox_group_id = $1 AND workspace_id = $2 AND name = $3 AND status = 'succeeded' AND deleted_at IS NULL
		ORDER BY created_at DESC, id DESC LIMIT 1`, groupID, workspaceID, name))
	if err != nil {
		return SandboxBuild{}, classify("sandbox build", err)
	}
	return b, nil
}

// StartSandboxBuild moves a pending build to running; false when it is no
// longer pending (another worker took it, or it was failed as stale).
func (s *PGStore) StartSandboxBuild(ctx context.Context, id string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE sandbox_builds SET status = 'running', updated_at = now()
		WHERE id = $1 AND status = 'pending' AND deleted_at IS NULL`, id)
	return tag.RowsAffected() == 1, err
}

// TouchSandboxBuild records that a queued or running import is still alive;
// false once the build has settled (the sweeper failed it, say), which tells
// the import to stop.
func (s *PGStore) TouchSandboxBuild(ctx context.Context, id string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE sandbox_builds SET updated_at = now()
		WHERE id = $1 AND status IN ('pending', 'running') AND deleted_at IS NULL`, id)
	return tag.RowsAffected() == 1, err
}

// SucceedSandboxBuild settles a running build with its stored image; false
// when the build was settled or deleted meanwhile (its group was deleted,
// say), so the caller deletes the image nothing names.
func (s *PGStore) SucceedSandboxBuild(ctx context.Context, id, imageDigest string, sizeBytes int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE sandbox_builds
		SET status = 'succeeded', image_digest = $2, size_bytes = $3, updated_at = now(), finished_at = now()
		WHERE id = $1 AND status IN ('pending', 'running') AND deleted_at IS NULL`, id, imageDigest, sizeBytes)
	return tag.RowsAffected() == 1, err
}

// FailSandboxBuild settles an unfinished build as failed with Render's
// failureCode and a reason.
func (s *PGStore) FailSandboxBuild(ctx context.Context, id, code, reason string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sandbox_builds
		SET status = 'failed', size_bytes = NULL, failure_code = $2, failure_reason = $3, updated_at = now(), finished_at = now()
		WHERE id = $1 AND status IN ('pending', 'running')`, id, code, reason)
	return err
}

// FailGroupSandboxBuilds fails a group's pending and running builds, whose
// imports then stop (their heartbeat finds the build settled), when the group
// is deleted.
func (s *PGStore) FailGroupSandboxBuilds(ctx context.Context, workspaceID, groupID, reason string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sandbox_builds
		SET status = 'failed', size_bytes = NULL, failure_code = 'internal_error', failure_reason = $3, updated_at = now(), finished_at = now()
		WHERE workspace_id = $1 AND sandbox_group_id = $2 AND status IN ('pending', 'running')`, workspaceID, groupID, reason)
	return err
}

// FailStaleSandboxBuilds fails every pending or running build not advanced
// for staleAfter (by the database's clock, which also stamps updated_at),
// whose import died with its bex-api replica: the registry credential it may
// have needed was never stored, so it cannot be resumed.
func (s *PGStore) FailStaleSandboxBuilds(ctx context.Context, staleAfter time.Duration, reason string) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE sandbox_builds
		SET status = 'failed', size_bytes = NULL, failure_code = 'internal_error', failure_reason = $2, updated_at = now(), finished_at = now()
		WHERE status IN ('pending', 'running') AND updated_at < now() - make_interval(secs => $1)`,
		staleAfter.Seconds(), reason)
	return tag.RowsAffected(), err
}

// SandboxBuildIDs are the builds of one group, or of the whole workspace when
// groupID is empty, that may have an image in the registry (every status but
// failed): what a group or workspace delete removes from the registry. A
// deleted build is included, so an image its own delete failed to remove is
// retried there (a tag already gone is not an error).
func (s *PGStore) SandboxBuildIDs(ctx context.Context, workspaceID, groupID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id FROM sandbox_builds
		WHERE workspace_id = $1 AND ($2 = '' OR sandbox_group_id = $2) AND status <> 'failed'`, workspaceID, groupID)
	if err != nil {
		return nil, classify("sandbox builds", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// DeleteSandboxBuild marks one build of a group deleted (w3/m367), as Render's
// delete-sandbox-build: it leaves every read, and an import still running
// stops (its heartbeat and settle find no live build). deleted is false for a
// build already deleted, which Render answers 204 too; a build of another
// group or workspace, or none, is ErrNotFound.
func (s *PGStore) DeleteSandboxBuild(ctx context.Context, workspaceID, groupID, id string) (deleted bool, err error) {
	var deletedAt *time.Time
	err = s.Pool.QueryRow(ctx, `
		WITH marked AS (
			UPDATE sandbox_builds SET deleted_at = now(), updated_at = now()
			WHERE id = $1 AND sandbox_group_id = $2 AND workspace_id = $3 AND deleted_at IS NULL
			RETURNING deleted_at)
		SELECT (SELECT deleted_at FROM marked)
		WHERE EXISTS (SELECT 1 FROM sandbox_builds WHERE id = $1 AND sandbox_group_id = $2 AND workspace_id = $3)`,
		id, groupID, workspaceID).Scan(&deletedAt)
	if err != nil {
		return false, classify("sandbox build", err)
	}
	return deletedAt != nil, nil
}

// RestoreSandboxBuild undoes a DeleteSandboxBuild that a dependent sandbox
// refused (w3/m367): the delete marks first, so no new create can start from
// the build while its dependents are checked.
func (s *PGStore) RestoreSandboxBuild(ctx context.Context, workspaceID, groupID, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sandbox_builds SET deleted_at = NULL, updated_at = now()
		WHERE id = $1 AND sandbox_group_id = $2 AND workspace_id = $3 AND deleted_at IS NOT NULL`, id, groupID, workspaceID)
	return classify("sandbox build", err)
}
