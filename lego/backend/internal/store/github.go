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

// GitConnection is a row of `git_connections`: a GitHub App installation a
// workspace has connected (docs/ADR026-github-integration.md, ADR075). Since
// w5/m74 the installation id is the primary key, so a workspace may hold many
// connections (one per GitHub account/org it has installed the App on) while an
// installation still belongs to at most one workspace. A re-connect of the same
// installation upserts; a different installation adds a row.
type GitConnection struct {
	WorkspaceID    string    `json:"workspaceId"`
	InstallationID int64     `json:"installationId"`
	AccountLogin   string    `json:"accountLogin"`
	CreatedAt      time.Time `json:"createdAt"`
}

// UpsertGitConnection records a connection, keyed by installation (ADR075). A
// re-connect of the same installation refreshes its workspace binding and
// account login; a new installation adds a row to the workspace's set. The
// caller (internal/github) enforces the one-workspace-per-installation and
// per-workspace-count invariants before this write.
// SECURITY (finding-4): the ON CONFLICT update is conditional on workspace_id
// matching so concurrent claims by two workspaces cannot silently transfer the
// installation. A cross-workspace conflict returns ErrConflict instead of
// overwriting the row.
func (s *PGStore) UpsertGitConnection(ctx context.Context, c GitConnection) (GitConnection, error) {
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO git_connections (workspace_id, installation_id, account_login)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (installation_id) DO UPDATE
		   SET account_login = EXCLUDED.account_login,
		       created_at = now()
		   WHERE git_connections.workspace_id = EXCLUDED.workspace_id
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

// GetGitConnection returns a workspace's oldest connection, or ErrNotFound. Since
// ADR075 a workspace may hold several; this backs the singular
// GET/POST/DELETE /v1/git/connection compatibility aliases, which act on the sole
// (or, ambiguously, the first) connection. Prefer ListGitConnections /
// GetGitConnectionByOwner for the multi-connection paths.
func (s *PGStore) GetGitConnection(ctx context.Context, workspaceID string) (GitConnection, error) {
	c := GitConnection{WorkspaceID: workspaceID}
	err := s.Pool.QueryRow(ctx,
		`SELECT installation_id, account_login, created_at FROM git_connections
		  WHERE workspace_id = $1 ORDER BY created_at, installation_id LIMIT 1`,
		workspaceID,
	).Scan(&c.InstallationID, &c.AccountLogin, &c.CreatedAt)
	if err != nil {
		return GitConnection{}, classify("git connection", err)
	}
	return c, nil
}

// ListGitConnections returns all of a workspace's connections, oldest first (an
// empty slice when it has none — not ErrNotFound). This is the aggregate the
// multi-account repo picker and the connections surface read (ADR075).
func (s *PGStore) ListGitConnections(ctx context.Context, workspaceID string) ([]GitConnection, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT installation_id, account_login, created_at FROM git_connections
		  WHERE workspace_id = $1 ORDER BY created_at, installation_id`,
		workspaceID)
	if err != nil {
		return nil, classify("git connection", err)
	}
	defer rows.Close()
	out := []GitConnection{}
	for rows.Next() {
		c := GitConnection{WorkspaceID: workspaceID}
		if err := rows.Scan(&c.InstallationID, &c.AccountLogin, &c.CreatedAt); err != nil {
			return nil, classify("git connection", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, classify("git connection", err)
	}
	return out, nil
}

// GetGitConnectionByOwner resolves the workspace connection whose GitHub account
// login matches accountLogin (case-insensitive), or ErrNotFound. Within a
// workspace account_login is unique by construction (GitHub allows one
// installation of a given App per account), so this is the exact connection to
// mint a token from for a repo owned by that account (ADR075 §4).
func (s *PGStore) GetGitConnectionByOwner(ctx context.Context, workspaceID, accountLogin string) (GitConnection, error) {
	c := GitConnection{WorkspaceID: workspaceID, AccountLogin: accountLogin}
	err := s.Pool.QueryRow(ctx,
		`SELECT installation_id, account_login, created_at FROM git_connections
		  WHERE workspace_id = $1 AND lower(account_login) = lower($2)
		  ORDER BY created_at, installation_id LIMIT 1`,
		workspaceID, accountLogin,
	).Scan(&c.InstallationID, &c.AccountLogin, &c.CreatedAt)
	if err != nil {
		return GitConnection{}, classify("git connection", err)
	}
	return c, nil
}

// DeleteGitConnection removes one connection of a workspace by installation id.
// Not-found (wrong workspace or unknown installation) is ErrNotFound. Scoping the
// delete to workspaceID keeps one workspace from disconnecting another's
// installation even if it learns the id.
func (s *PGStore) DeleteGitConnection(ctx context.Context, workspaceID string, installationID int64) error {
	tag, err := s.Pool.Exec(ctx,
		`DELETE FROM git_connections WHERE workspace_id = $1 AND installation_id = $2`,
		workspaceID, installationID)
	if err != nil {
		return classify("git connection", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountGitConnections returns how many connections a workspace holds — the
// per-workspace quota check (BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE, ADR075 §2).
func (s *PGStore) CountGitConnections(ctx context.Context, workspaceID string) (int, error) {
	var n int
	if err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM git_connections WHERE workspace_id = $1`, workspaceID,
	).Scan(&n); err != nil {
		return 0, classify("git connection", err)
	}
	return n, nil
}

// GitHubConnectTransaction is one in-flight connect attempt: who started it, for
// which workspace, and until when (w1/m67 F3). It exists because the GitHub
// install redirect returns to an anonymous callback, so without a server-side
// record the flow could only ever prove that SOMEONE authorized SOME workspace —
// never that the human completing the installation is the one who asked.
type GitHubConnectTransaction struct {
	Nonce     string
	TenantID  string
	Subject   string
	ExpiresAt time.Time
}

// CreateGitHubConnectTransaction records a connect attempt. Expired rows are
// pruned here rather than by a janitor: the flow is human-driven and rare, so the
// piggybacked DELETE keeps the table at "attempts started in the last few
// minutes" for free.
func (s *PGStore) CreateGitHubConnectTransaction(ctx context.Context, t GitHubConnectTransaction) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM github_connect_transactions WHERE expires_at < now()`); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO github_connect_transactions (nonce, tenant_id, subject, expires_at)
		 VALUES ($1, $2, $3, $4)`,
		t.Nonce, t.TenantID, t.Subject, t.ExpiresAt)
	if err != nil {
		return classify("github connect transaction", err)
	}
	return nil
}

// ConsumeGitHubConnectTransaction atomically claims an unexpired attempt and
// returns it. Single-use by construction: the row is DELETEd in the same
// statement that reads it, so a replayed callback — on any replica — finds
// nothing. ErrNotFound covers unknown, already-consumed, and expired alike, so a
// caller cannot distinguish them (and neither can an attacker probing).
func (s *PGStore) ConsumeGitHubConnectTransaction(ctx context.Context, nonce string) (GitHubConnectTransaction, error) {
	var t GitHubConnectTransaction
	err := s.Pool.QueryRow(ctx,
		`DELETE FROM github_connect_transactions
		  WHERE nonce = $1 AND expires_at > now()
		  RETURNING nonce, tenant_id, subject, expires_at`, nonce,
	).Scan(&t.Nonce, &t.TenantID, &t.Subject, &t.ExpiresAt)
	if err != nil {
		return GitHubConnectTransaction{}, classify("github connect transaction", err)
	}
	return t, nil
}
