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

// NewSandboxBuild is an import to record: the resolved source and the name.
type NewSandboxBuild struct {
	WorkspaceID      string
	SandboxGroupID   string
	Name             string
	SourceRepository string
	SourceImage      string
	SourceReference  string
	SourceDigest     string
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
		b, err = scanSandboxBuild(tx.QueryRow(ctx, `
			INSERT INTO sandbox_builds (id, workspace_id, sandbox_group_id, name, source_repository, source_image, source_reference, source_digest)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING `+sandboxBuildColumns,
			ids.New(ids.SandboxBuild), in.WorkspaceID, in.SandboxGroupID, in.Name,
			in.SourceRepository, in.SourceImage, in.SourceReference, in.SourceDigest))
		created = err == nil
		return err
	})
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
		SET status = 'failed', failure_code = $2, failure_reason = $3, updated_at = now(), finished_at = now()
		WHERE id = $1 AND status IN ('pending', 'running')`, id, code, reason)
	return err
}

// FailGroupSandboxBuilds fails a group's pending and running builds, whose
// imports then stop (their heartbeat finds the build settled), when the group
// is deleted.
func (s *PGStore) FailGroupSandboxBuilds(ctx context.Context, workspaceID, groupID, reason string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE sandbox_builds
		SET status = 'failed', failure_code = 'internal_error', failure_reason = $3, updated_at = now(), finished_at = now()
		WHERE workspace_id = $1 AND sandbox_group_id = $2 AND status IN ('pending', 'running')`, workspaceID, groupID, reason)
	return err
}

// FailStaleSandboxBuilds fails every pending or running build not advanced
// for staleAfter (by the database's clock, which also stamps updated_at),
// whose import died with its bex-api replica: the registry credential it may
// have needed was never stored, so it cannot be resumed.
func (s *PGStore) FailStaleSandboxBuilds(ctx context.Context, staleAfter time.Duration, reason string) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE sandbox_builds
		SET status = 'failed', failure_code = 'internal_error', failure_reason = $2, updated_at = now(), finished_at = now()
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
