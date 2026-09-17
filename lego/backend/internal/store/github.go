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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// GitConnection is a row of `git_connections`: a GitHub App installation a
// workspace has connected (docs/ADR026-github-integration.md, ADR078). Since
// w2/m162 the key is composite (workspace_id, installation_id) — the model is
// N:N. A workspace holds many connections (one per GitHub account/org it has
// installed the App on), and an installation may serve many workspaces, each
// binding carrying its own complete three-proof sequence (ADR078 §2). A
// re-connect of the same (workspace, installation) pair upserts; anything else
// adds a row.
type GitConnection struct {
	WorkspaceID    string    `json:"workspaceId"`
	InstallationID int64     `json:"installationId"`
	AccountLogin   string    `json:"accountLogin"`
	CreatedAt      time.Time `json:"createdAt"`
}

// GitConnectionLimitError reports an atomic refusal of the per-workspace quota
// (BEX_MAX_GIT_CONNECTIONS_PER_WORKSPACE). It is typed so the GitHub service can
// preserve its public GIT_CONNECTION_LIMIT dialect without making this storage
// package depend on API errors.
type GitConnectionLimitError struct {
	Count int
	Limit int
}

func (e *GitConnectionLimitError) Error() string {
	return fmt.Sprintf("git connection limit reached: %d of %d", e.Count, e.Limit)
}

// GitInstallationWorkspaceLimitError is GitConnectionLimitError's mirror: an
// atomic refusal of the per-installation quota
// (BEX_MAX_WORKSPACES_PER_GIT_INSTALLATION, ADR078 §2). It bounds how many
// workspaces one installation may serve, and therefore how wide a single push
// delivery can fan out (ADR078 §4a).
type GitInstallationWorkspaceLimitError struct {
	Count int
	Limit int
}

func (e *GitInstallationWorkspaceLimitError) Error() string {
	return fmt.Sprintf("git installation workspace limit reached: %d of %d", e.Count, e.Limit)
}

// BindGitConnection performs the same-pair check, both quota checks, and the
// write in one transaction. The transaction-scoped advisory locks work across API
// replicas and distinct pool connections.
//
// SECURITY: the counts and the insert MUST share this transaction. A standalone
// count beforehand lets two concurrent callbacks at limit-1 both pass. Two locks
// are taken because N:N has two independent caps — the workspace's fan-in and the
// installation's fan-out — and they are always acquired workspace-first so
// concurrent binds cannot deadlock on opposite orders. A same-pair reconnect is
// detected before either count and remains exempt from both.
func (s *PGStore) BindGitConnection(ctx context.Context, c GitConnection, maxConnections, maxWorkspaces int) (GitConnection, error) {
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		// Consistent lock order: workspace, then installation. Never reverse it.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, c.WorkspaceID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, c.InstallationID); err != nil {
			return err
		}

		// Idempotent reconnect of a pair this workspace already holds: refresh the
		// account login (so a GitHub account rename converges) and stop. Neither
		// quota applies — the row count does not change.
		err := tx.QueryRow(ctx,
			`UPDATE git_connections SET account_login = $3, created_at = now()
			  WHERE workspace_id = $1 AND installation_id = $2
			  RETURNING created_at`,
			c.WorkspaceID, c.InstallationID, c.AccountLogin,
		).Scan(&c.CreatedAt)
		switch {
		case err == nil:
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		if maxConnections > 0 {
			var count int
			if err := tx.QueryRow(ctx,
				`SELECT count(*) FROM git_connections WHERE workspace_id = $1`, c.WorkspaceID,
			).Scan(&count); err != nil {
				return err
			}
			if count >= maxConnections {
				return &GitConnectionLimitError{Count: count, Limit: maxConnections}
			}
		}

		if maxWorkspaces > 0 {
			var count int
			if err := tx.QueryRow(ctx,
				`SELECT count(*) FROM git_connections WHERE installation_id = $1`, c.InstallationID,
			).Scan(&count); err != nil {
				return err
			}
			if count >= maxWorkspaces {
				return &GitInstallationWorkspaceLimitError{Count: count, Limit: maxWorkspaces}
			}
		}

		return tx.QueryRow(ctx,
			`INSERT INTO git_connections (workspace_id, installation_id, account_login)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (workspace_id, installation_id) DO UPDATE
			   SET account_login = EXCLUDED.account_login, created_at = now()
			 RETURNING created_at`,
			c.WorkspaceID, c.InstallationID, c.AccountLogin,
		).Scan(&c.CreatedAt)
	})
	if err != nil {
		var connLimit *GitConnectionLimitError
		if errors.As(err, &connLimit) {
			return GitConnection{}, connLimit
		}
		var wsLimit *GitInstallationWorkspaceLimitError
		if errors.As(err, &wsLimit) {
			return GitConnection{}, wsLimit
		}
		return GitConnection{}, classify("git connection", err)
	}
	return c, nil
}

// GitConnectionsByInstallation returns every workspace binding for
// installationID, oldest first (an empty slice when none — not ErrNotFound).
//
// It is the reverse lookup, and the ONLY direction in which an installation
// resolves to workspaces: every other consumer goes workspace -> connections.
// The push webhook reads it to confine an app-signed delivery to the proved
// binding set (ADR078 §4a), where an EMPTY result must mean "act on nothing" —
// never a widening to a global match.
func (s *PGStore) GitConnectionsByInstallation(ctx context.Context, installationID int64) ([]GitConnection, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT workspace_id, account_login, created_at FROM git_connections
		  WHERE installation_id = $1 ORDER BY created_at, workspace_id`,
		installationID)
	if err != nil {
		return nil, classify("git connection", err)
	}
	defer rows.Close()
	out := []GitConnection{}
	for rows.Next() {
		c := GitConnection{InstallationID: installationID}
		if err := rows.Scan(&c.WorkspaceID, &c.AccountLogin, &c.CreatedAt); err != nil {
			return nil, classify("git connection", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, classify("git connection", err)
	}
	return out, nil
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
	Nonce    string
	TenantID string
	Subject  string
	// InstallationID is the claim flow's optional start-time selector (ADR078
	// §3a): 0 = unspecified. When set it NARROWS the server-proved candidate set
	// at the callback; it can never add to it, because an installation the OAuth
	// user does not administer is not a candidate regardless of what was named
	// here. Unused by the install flow, which gets its id from GitHub.
	InstallationID int64
	ExpiresAt      time.Time
}

// CreateGitHubConnectTransaction records a connect attempt. Expired rows are
// pruned here rather than by a janitor: the flow is human-driven and rare, so the
// piggybacked DELETE keeps the table at "attempts started in the last few
// minutes" for free.
func (s *PGStore) CreateGitHubConnectTransaction(ctx context.Context, t GitHubConnectTransaction) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM github_connect_transactions WHERE expires_at < now()`); err != nil {
		return err
	}
	var installation *int64
	if t.InstallationID > 0 {
		installation = &t.InstallationID
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO github_connect_transactions (nonce, tenant_id, subject, installation_id, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		t.Nonce, t.TenantID, t.Subject, installation, t.ExpiresAt)
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
	var installation *int64
	err := s.Pool.QueryRow(ctx,
		`DELETE FROM github_connect_transactions
		  WHERE nonce = $1 AND expires_at > now()
		  RETURNING nonce, tenant_id, subject, installation_id, expires_at`, nonce,
	).Scan(&t.Nonce, &t.TenantID, &t.Subject, &installation, &t.ExpiresAt)
	if err != nil {
		return GitHubConnectTransaction{}, classify("github connect transaction", err)
	}
	if installation != nil {
		t.InstallationID = *installation
	}
	return t, nil
}

// GitHubClaimCandidate is one proved member of a claim selection: an
// installation the authorizing user was shown to administer.
type GitHubClaimCandidate struct {
	InstallationID int64  `json:"installationId"`
	AccountLogin   string `json:"accountLogin"`
}

// GitHubClaimSelection is an ambiguous claim's deferred choice (ADR078 §3a): the
// candidate set the callback ALREADY proved, held for the few minutes the human
// needs to pick one. Subject- and workspace-bound, single-use, and closed — the
// selection can only ever resolve to a member of Candidates.
type GitHubClaimSelection struct {
	ID          string
	WorkspaceID string
	Subject     string
	Candidates  []GitHubClaimCandidate
	ExpiresAt   time.Time
}

// CreateGitHubClaimSelection records a proved candidate set. Expired rows are
// pruned here rather than by a janitor, matching the connect transaction: the
// flow is human-driven and rare, so the piggybacked DELETE keeps the table at
// "selections offered in the last few minutes" for free.
func (s *PGStore) CreateGitHubClaimSelection(ctx context.Context, sel GitHubClaimSelection) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM github_claim_selections WHERE expires_at < now()`); err != nil {
		return err
	}
	candidates, err := json.Marshal(sel.Candidates)
	if err != nil {
		return err
	}
	if _, err := s.Pool.Exec(ctx,
		`INSERT INTO github_claim_selections (id, workspace_id, subject, candidates, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		sel.ID, sel.WorkspaceID, sel.Subject, candidates, sel.ExpiresAt,
	); err != nil {
		return classify("github claim selection", err)
	}
	return nil
}

// GetGitHubClaimSelection reads an unexpired selection WITHOUT consuming it — the
// picker's render. ErrNotFound covers unknown, already-consumed, and expired
// alike, so a caller cannot distinguish them (and neither can an attacker
// probing). Subject matching is the service's job, on the fresh caller identity.
func (s *PGStore) GetGitHubClaimSelection(ctx context.Context, id string) (GitHubClaimSelection, error) {
	sel := GitHubClaimSelection{ID: id}
	var candidates []byte
	err := s.Pool.QueryRow(ctx,
		`SELECT workspace_id, subject, candidates, expires_at FROM github_claim_selections
		  WHERE id = $1 AND expires_at > now()`, id,
	).Scan(&sel.WorkspaceID, &sel.Subject, &candidates, &sel.ExpiresAt)
	if err != nil {
		return GitHubClaimSelection{}, classify("github claim selection", err)
	}
	if err := json.Unmarshal(candidates, &sel.Candidates); err != nil {
		return GitHubClaimSelection{}, err
	}
	return sel, nil
}

// ConsumeGitHubClaimSelection atomically claims an unexpired selection and
// returns it. Single-use by construction: the row is DELETEd in the same
// statement that reads it, so a replayed selection — on any replica — finds
// nothing.
func (s *PGStore) ConsumeGitHubClaimSelection(ctx context.Context, id string) (GitHubClaimSelection, error) {
	sel := GitHubClaimSelection{ID: id}
	var candidates []byte
	err := s.Pool.QueryRow(ctx,
		`DELETE FROM github_claim_selections
		  WHERE id = $1 AND expires_at > now()
		  RETURNING workspace_id, subject, candidates, expires_at`, id,
	).Scan(&sel.WorkspaceID, &sel.Subject, &candidates, &sel.ExpiresAt)
	if err != nil {
		return GitHubClaimSelection{}, classify("github claim selection", err)
	}
	if err := json.Unmarshal(candidates, &sel.Candidates); err != nil {
		return GitHubClaimSelection{}, err
	}
	return sel, nil
}
