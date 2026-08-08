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

// GitConnection is a row of `git_connections`: the GitHub App installation a
// workspace has connected (docs/ADR026-github-integration.md). One per workspace — the
// workspace id is the primary key, so recording again upserts.
type GitConnection struct {
	WorkspaceID    string    `json:"workspaceId"`
	InstallationID int64     `json:"installationId"`
	AccountLogin   string    `json:"accountLogin"`
	CreatedAt      time.Time `json:"createdAt"`
}

// UpsertGitConnection records (or replaces) a workspace's GitHub connection.
func (s *PGStore) UpsertGitConnection(ctx context.Context, c GitConnection) (GitConnection, error) {
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO git_connections (workspace_id, installation_id, account_login)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (workspace_id) DO UPDATE
		   SET installation_id = EXCLUDED.installation_id,
		       account_login = EXCLUDED.account_login,
		       created_at = now()
		 RETURNING created_at`,
		c.WorkspaceID, c.InstallationID, c.AccountLogin,
	).Scan(&c.CreatedAt)
	if err != nil {
		return GitConnection{}, classify("git connection", err)
	}
	return c, nil
}

// GitConnectionByInstallation returns the connection bound to installationID, or
// ErrNotFound when no workspace has connected it. It backs the unique
// installation->workspace binding (w1/m65 F2): because the App JWT can look up
// EVERY installation of itself, a GetInstallation success is existence proof
// only — this lookup is what lets the service reject a second workspace trying to
// claim an installation another already owns.
func (s *PGStore) GitConnectionByInstallation(ctx context.Context, installationID int64) (GitConnection, error) {
	c := GitConnection{InstallationID: installationID}
	err := s.Pool.QueryRow(ctx,
		`SELECT workspace_id, account_login, created_at FROM git_connections WHERE installation_id = $1`,
		installationID,
	).Scan(&c.WorkspaceID, &c.AccountLogin, &c.CreatedAt)
	if err != nil {
		return GitConnection{}, classify("git connection", err)
	}
	return c, nil
}

// GetGitConnection returns a workspace's connection, or ErrNotFound.
func (s *PGStore) GetGitConnection(ctx context.Context, workspaceID string) (GitConnection, error) {
	c := GitConnection{WorkspaceID: workspaceID}
	err := s.Pool.QueryRow(ctx,
		`SELECT installation_id, account_login, created_at FROM git_connections WHERE workspace_id = $1`,
		workspaceID,
	).Scan(&c.InstallationID, &c.AccountLogin, &c.CreatedAt)
	if err != nil {
		return GitConnection{}, classify("git connection", err)
	}
	return c, nil
}

// DeleteGitConnection removes a workspace's connection. Not-found is ErrNotFound.
func (s *PGStore) DeleteGitConnection(ctx context.Context, workspaceID string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM git_connections WHERE workspace_id = $1`, workspaceID)
	if err != nil {
		return classify("git connection", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
