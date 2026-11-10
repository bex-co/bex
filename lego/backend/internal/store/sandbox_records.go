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

// SandboxRecordRetention is how long an ended sandbox stays readable (w3/m318).
// Render documents no window; this is bex's own sandbox retention, the same as
// a suspended sandbox's and an execution's (SandboxExecutionRetention).
const SandboxRecordRetention = 30 * 24 * time.Hour

// SandboxRecord is one sandbox's durable record: its owner, its public read
// shape (View, JSON) when last written, and when it ended (nil while live).
type SandboxRecord struct {
	WorkspaceID  string
	SandboxID    string
	Owner        string
	View         []byte
	TerminatedAt *time.Time
}

// RecordSandbox writes a sandbox's record: a live one at create, an ended one
// (TerminatedAt set) at its end. Ending a recorded sandbox only stamps the
// end, keeping the view written at create, since nothing about a sandbox
// changes after it; the view is written when no record exists (a sandbox
// created before records did). The first end wins, and an ended record never
// becomes live again.
func (s *PGStore) RecordSandbox(ctx context.Context, r SandboxRecord) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sandbox_records (workspace_id, sandbox_id, owner, view, terminated_at) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (workspace_id, sandbox_id) DO UPDATE SET terminated_at = EXCLUDED.terminated_at
		WHERE sandbox_records.terminated_at IS NULL`, r.WorkspaceID, r.SandboxID, r.Owner, r.View, r.TerminatedAt)
	return classify("sandbox record", err)
}

// EndAbsentSandboxes ends, at at, the live records of one workspace's
// sandboxes absent from a complete OpenSandbox list: ones OpenSandbox deleted
// itself, at their expireTime. A record younger than ten minutes is kept, as
// for names: its create may not have reached the list yet.
func (s *PGStore) EndAbsentSandboxes(ctx context.Context, workspaceID string, live []string, at time.Time) error {
	if live == nil {
		live = []string{}
	}
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_records SET terminated_at = $3
		WHERE workspace_id = $1 AND terminated_at IS NULL AND NOT (sandbox_id = ANY($2::text[]))
		  AND created_at < now() - interval '10 minutes'`, workspaceID, live, at)
	return err
}

// ExpireSandboxRecords deletes every record ended more than
// SandboxRecordRetention before now.
func (s *PGStore) ExpireSandboxRecords(ctx context.Context, now time.Time) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sandbox_records WHERE terminated_at < $1`, now.Add(-SandboxRecordRetention))
	return err
}

// EndedSandbox is one ended sandbox's record, or ErrNotFound when the sandbox
// has none or has not ended.
func (s *PGStore) EndedSandbox(ctx context.Context, workspaceID, sandboxID string) (SandboxRecord, error) {
	r := SandboxRecord{WorkspaceID: workspaceID, SandboxID: sandboxID}
	err := s.Pool.QueryRow(ctx, `
		SELECT owner, view, terminated_at FROM sandbox_records
		WHERE workspace_id = $1 AND sandbox_id = $2 AND terminated_at IS NOT NULL`, workspaceID, sandboxID).
		Scan(&r.Owner, &r.View, &r.TerminatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SandboxRecord{}, ErrNotFound
	}
	return r, err
}

// EndedSandboxes are one workspace's ended records, most recently ended first.
func (s *PGStore) EndedSandboxes(ctx context.Context, workspaceID string) ([]SandboxRecord, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT sandbox_id, owner, view, terminated_at FROM sandbox_records
		WHERE workspace_id = $1 AND terminated_at IS NOT NULL
		ORDER BY terminated_at DESC, sandbox_id`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SandboxRecord
	for rows.Next() {
		r := SandboxRecord{WorkspaceID: workspaceID}
		if err := rows.Scan(&r.SandboxID, &r.Owner, &r.View, &r.TerminatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// EndedSandboxWorkspaces are the workspaces holding an ended record of
// sandboxID: the routing read that lets a by-id verb find an ended sandbox's
// workspace once OpenSandbox no longer answers for it.
func (s *PGStore) EndedSandboxWorkspaces(ctx context.Context, sandboxID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT workspace_id FROM sandbox_records WHERE sandbox_id = $1 AND terminated_at IS NOT NULL
		ORDER BY workspace_id`, sandboxID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}
