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

	"github.com/jackc/pgx/v5"
)

// GetMCPWorkspaceSelection returns the workspace selected by one authenticated
// subject in one MCP session. The subject is part of the key so possession of a
// leaked or reused session id cannot expose another caller's selection.
func (s *PGStore) GetMCPWorkspaceSelection(ctx context.Context, sessionID, subject string) (string, bool, error) {
	var workspaceID string
	err := s.Pool.QueryRow(ctx, `
		SELECT workspace_id
		FROM mcp_workspace_selections
		WHERE session_id = $1 AND subject = $2
	`, sessionID, subject).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return workspaceID, true, nil
}

// SetMCPWorkspaceSelection upserts a subject's selection for an MCP session.
func (s *PGStore) SetMCPWorkspaceSelection(ctx context.Context, sessionID, subject, workspaceID string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO mcp_workspace_selections (session_id, subject, workspace_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (session_id, subject) DO UPDATE
		SET workspace_id = EXCLUDED.workspace_id, updated_at = now()
	`, sessionID, subject, workspaceID)
	return err
}
