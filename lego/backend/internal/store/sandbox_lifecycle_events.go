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
	"time"
)

// SandboxLifecycleEventRetention is how long a lifecycle event is kept: past
// an ended sandbox's record (SandboxRecordRetention), so its timeline never
// outlives the sandbox's own read.
const SandboxLifecycleEventRetention = 35 * 24 * time.Hour

// SandboxLifecycleEvent is one transition of a sandbox's lifecycle (w1/m658,
// migration 0350): Render's SandboxLifecycleEvent `lifecycleType` and
// `trigger`, and the person who started it on its created event.
type SandboxLifecycleEvent struct {
	ID            string
	WorkspaceID   string
	SandboxID     string
	LifecycleType string
	Trigger       string
	UserEmail     string
	At            time.Time
}

// RecordSandboxLifecycleEvent writes one event; writing an id again is a no-op.
func (s *PGStore) RecordSandboxLifecycleEvent(ctx context.Context, e SandboxLifecycleEvent) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sandbox_lifecycle_events (id, workspace_id, sandbox_id, lifecycle_type, trigger, user_email, at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING`,
		e.ID, e.WorkspaceID, e.SandboxID, e.LifecycleType, e.Trigger, e.UserEmail, e.At.UTC())
	return classify("sandbox lifecycle event", err)
}

// SandboxLifecycleEvents are one sandbox's lifecycle events, newest first.
func (s *PGStore) SandboxLifecycleEvents(ctx context.Context, workspaceID, sandboxID string) ([]SandboxLifecycleEvent, error) {
	return s.sandboxLifecycleEvents(ctx, `
		SELECT id, workspace_id, sandbox_id, lifecycle_type, trigger, user_email, at FROM sandbox_lifecycle_events
		WHERE workspace_id = $1 AND sandbox_id = $2
		ORDER BY at DESC, id DESC`, workspaceID, sandboxID)
}

// SandboxStartEvents are the created and ready events of the given sandboxes,
// oldest first: who started each one, and when it first ran.
func (s *PGStore) SandboxStartEvents(ctx context.Context, workspaceID string, sandboxIDs []string) ([]SandboxLifecycleEvent, error) {
	if len(sandboxIDs) == 0 {
		return nil, nil
	}
	return s.sandboxLifecycleEvents(ctx, `
		SELECT id, workspace_id, sandbox_id, lifecycle_type, trigger, user_email, at FROM sandbox_lifecycle_events
		WHERE workspace_id = $1 AND sandbox_id = ANY($2::text[]) AND lifecycle_type IN ('CREATED', 'READY')
		ORDER BY at, id`, workspaceID, sandboxIDs)
}

func (s *PGStore) sandboxLifecycleEvents(ctx context.Context, query string, args ...any) ([]SandboxLifecycleEvent, error) {
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SandboxLifecycleEvent
	for rows.Next() {
		var e SandboxLifecycleEvent
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.SandboxID, &e.LifecycleType, &e.Trigger, &e.UserEmail, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ExpireSandboxLifecycleEvents deletes the events older than their retention.
func (s *PGStore) ExpireSandboxLifecycleEvents(ctx context.Context, now time.Time) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sandbox_lifecycle_events WHERE at < $1`, now.Add(-SandboxLifecycleEventRetention))
	return err
}
