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

// SandboxExecution is one recorded command run or file transfer (w32/m298).
type SandboxExecution struct {
	ID          string
	WorkspaceID string
	SandboxID   string
	Type        string // runs | files
	Operation   string // sync | stream | upload | download
	// Command is the truncated command, or a transfer's file path.
	Command string
	// Subject is the identity the execution was minted for.
	Subject   string
	StartedAt time.Time
	StoppedAt *time.Time
	ExitCode  *int
}

const sandboxExecutionColumns = `id, workspace_id, sandbox_id, type, operation, command, subject, started_at, stopped_at, exit_code`

func scanSandboxExecution(row pgx.Row) (SandboxExecution, error) {
	var e SandboxExecution
	err := row.Scan(&e.ID, &e.WorkspaceID, &e.SandboxID, &e.Type, &e.Operation, &e.Command, &e.Subject, &e.StartedAt, &e.StoppedAt, &e.ExitCode)
	return e, err
}

// RecordSandboxExecution writes a started execution.
func (s *PGStore) RecordSandboxExecution(ctx context.Context, e SandboxExecution) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sandbox_executions (id, workspace_id, sandbox_id, type, operation, command, subject, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.ID, e.WorkspaceID, e.SandboxID, e.Type, e.Operation, e.Command, e.Subject, e.StartedAt)
	return classify("sandbox execution", err)
}

// StopSandboxExecution records an execution's end once; a later stop (a
// retried report, the sandbox's finalization) leaves the first one standing.
func (s *PGStore) StopSandboxExecution(ctx context.Context, workspaceID, executionID string, stoppedAt time.Time, exitCode *int) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_executions SET stopped_at = $3, exit_code = $4
		WHERE workspace_id = $1 AND id = $2 AND stopped_at IS NULL`,
		workspaceID, executionID, stoppedAt, exitCode)
	return err
}

// TouchSandboxExecution records that an open execution's stream is still
// running (w8/m151).
func (s *PGStore) TouchSandboxExecution(ctx context.Context, workspaceID, executionID string, seenAt time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_executions SET seen_at = $3
		WHERE workspace_id = $1 AND id = $2 AND stopped_at IS NULL`,
		workspaceID, executionID, seenAt)
	return err
}

// StopStaleSandboxExecutions ends a sandbox's open executions whose stream
// was last seen (or, never seen, started) before staleBefore, at that
// instant: their stream is gone without having stopped them (w8/m151).
func (s *PGStore) StopStaleSandboxExecutions(ctx context.Context, workspaceID, sandboxID string, staleBefore time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_executions SET stopped_at = COALESCE(seen_at, started_at)
		WHERE workspace_id = $1 AND sandbox_id = $2 AND stopped_at IS NULL
		  AND COALESCE(seen_at, started_at) < $3`,
		workspaceID, sandboxID, staleBefore)
	return err
}

// StopOpenSandboxExecutions ends every execution still open in a sandbox,
// for its termination.
func (s *PGStore) StopOpenSandboxExecutions(ctx context.Context, workspaceID, sandboxID string, stoppedAt time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE sandbox_executions SET stopped_at = $3
		WHERE workspace_id = $1 AND sandbox_id = $2 AND stopped_at IS NULL`,
		workspaceID, sandboxID, stoppedAt)
	return err
}

// SandboxExecutions lists one sandbox's executions newest first, as Render
// orders them, resuming after the execution id `after` when it is set. An
// unknown cursor yields an empty page.
func (s *PGStore) SandboxExecutions(ctx context.Context, workspaceID, sandboxID, after string, limit int) ([]SandboxExecution, error) {
	query, args := pageKeyset(`SELECT `+sandboxExecutionColumns+` FROM sandbox_executions
		WHERE workspace_id = $1 AND sandbox_id = $2`, []any{workspaceID, sandboxID}, "sandbox_executions", "started_at", after, limit, false)
	rows, err := s.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SandboxExecution{}
	for rows.Next() {
		e, err := scanSandboxExecution(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SandboxExecution reads one execution of one sandbox; ErrNotFound when the
// sandbox has no such execution.
func (s *PGStore) SandboxExecution(ctx context.Context, workspaceID, sandboxID, executionID string) (SandboxExecution, error) {
	e, err := scanSandboxExecution(s.Pool.QueryRow(ctx, `
		SELECT `+sandboxExecutionColumns+` FROM sandbox_executions
		WHERE workspace_id = $1 AND sandbox_id = $2 AND id = $3`, workspaceID, sandboxID, executionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return SandboxExecution{}, ErrNotFound
	}
	return e, err
}

// SandboxExecutionRetention bounds how long any record is kept, so a
// long-lived sandbox's history cannot grow without end. Render documents no
// retention; 30 days is its longest sandbox timeout.
const SandboxExecutionRetention = 30 * 24 * time.Hour

// PruneSandboxExecutions deletes the executions of one workspace's sandboxes
// absent from a complete OpenSandbox list and with no record (a sandbox that
// is gone and was never recorded has no readable history; an ended one keeps
// its history while its record lasts, w3/m318; a row younger than ten minutes
// is kept, as for names), and any record older than SandboxExecutionRetention.
func (s *PGStore) PruneSandboxExecutions(ctx context.Context, workspaceID string, live []string) error {
	if live == nil {
		live = []string{}
	}
	_, err := s.Pool.Exec(ctx, `
		DELETE FROM sandbox_executions e
		WHERE e.workspace_id = $1 AND (
		    (NOT (e.sandbox_id = ANY($2::text[])) AND e.started_at < now() - interval '10 minutes'
		     AND NOT EXISTS (SELECT 1 FROM sandbox_records r
		                     WHERE r.workspace_id = e.workspace_id AND r.sandbox_id = e.sandbox_id))
		    OR e.started_at < now() - make_interval(secs => $3))`,
		workspaceID, live, SandboxExecutionRetention.Seconds())
	return err
}
