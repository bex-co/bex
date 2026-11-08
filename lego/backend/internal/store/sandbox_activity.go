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
)

// TouchSandboxActivity records an exec or file transfer on one sandbox
// (w2/m278). last_activity_at only moves forward, so a late heartbeat never
// rewinds a newer touch. It returns the idle sweeper's suspend claim, if one
// is set: the caller then knows a pause may be in flight even though the
// sandbox it just read still reports running.
func (s *PGStore) TouchSandboxActivity(ctx context.Context, workspaceID, sandboxID string, at time.Time) (*time.Time, error) {
	var claimed *time.Time
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO sandbox_activity (workspace_id, sandbox_id, last_activity_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, sandbox_id)
		DO UPDATE SET last_activity_at = GREATEST(sandbox_activity.last_activity_at, EXCLUDED.last_activity_at)
		RETURNING suspend_claimed_at`, workspaceID, sandboxID, at.UTC()).Scan(&claimed)
	return claimed, err
}

// ClaimSandboxIdleSuspend is the idle sweeper's compare-and-set. It claims the
// sandbox only when its last activity is at or before cutoff and no other
// claim newer than staleBefore holds it, so exactly one replica pauses an idle
// sandbox and an exec that touched it after the cutoff wins. since seeds a
// sandbox that has never been touched (its create time). One statement: the
// ON CONFLICT arbitration serializes concurrent sweepers on the row.
func (s *PGStore) ClaimSandboxIdleSuspend(ctx context.Context, workspaceID, sandboxID string, since, cutoff, now, staleBefore time.Time) (bool, error) {
	var claimed bool
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO sandbox_activity AS a (workspace_id, sandbox_id, last_activity_at, suspend_claimed_at)
		VALUES ($1, $2, $3::timestamptz, CASE WHEN $3::timestamptz <= $4::timestamptz THEN $5::timestamptz END)
		ON CONFLICT (workspace_id, sandbox_id) DO UPDATE SET suspend_claimed_at = $5::timestamptz
		WHERE a.last_activity_at <= $4::timestamptz
		  AND (a.suspend_claimed_at IS NULL OR a.suspend_claimed_at < $6::timestamptz)
		RETURNING a.suspend_claimed_at IS NOT NULL`,
		workspaceID, sandboxID, since.UTC(), cutoff.UTC(), now.UTC(), staleBefore.UTC()).Scan(&claimed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return claimed, err
}

// ClearSandboxSuspendClaim drops the suspend claim once the pause it guarded
// has been undone by a resume, or failed.
func (s *PGStore) ClearSandboxSuspendClaim(ctx context.Context, workspaceID, sandboxID string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_activity SET suspend_claimed_at = NULL
		WHERE workspace_id = $1 AND sandbox_id = $2`, workspaceID, sandboxID)
	return err
}

// PruneSandboxActivity deletes the activity rows of one workspace's sandboxes
// that are absent from a complete OpenSandbox list (terminated and reaped).
// The caller invokes it only after a successful list.
func (s *PGStore) PruneSandboxActivity(ctx context.Context, workspaceID string, live []string) error {
	if live == nil {
		live = []string{}
	}
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM sandbox_activity
		WHERE workspace_id = $1 AND NOT (sandbox_id = ANY($2::text[]))`, workspaceID, live)
	return err
}
