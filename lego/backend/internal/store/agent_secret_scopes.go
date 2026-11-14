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
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Workspace agent secret scopes (ADR098 D9). A secret with no row is
// AgentSecretScopeExplicit.
const (
	AgentSecretScopeWorkspace = "workspace"
	AgentSecretScopeRepo      = "repo"
	AgentSecretScopeExplicit  = "explicit"
)

// AgentSecretScope is the scope row of one workspace agent secret. Repo is
// the owner/name a repo scope names, empty otherwise.
type AgentSecretScope struct {
	Name  string
	Scope string
	Repo  string
}

// ListAgentSecretScopes returns the workspace's scope rows, ordered by name.
func (s *PGStore) ListAgentSecretScopes(ctx context.Context, workspaceID string) ([]AgentSecretScope, error) {
	rows, err := collectWith(scanAgentSecretScope)(s.Pool.Query(ctx, `SELECT name, scope, repo FROM agent_secret_scopes
		WHERE workspace_id = $1 ORDER BY name`, workspaceID))
	if err != nil {
		return nil, fmt.Errorf("list agent secret scopes: %w", err)
	}
	return rows, nil
}

func scanAgentSecretScope(row pgx.Row) (AgentSecretScope, error) {
	var out AgentSecretScope
	err := row.Scan(&out.Name, &out.Scope, &out.Repo)
	return out, err
}

// UpsertAgentSecretScope writes one secret's scope.
func (s *PGStore) UpsertAgentSecretScope(ctx context.Context, workspaceID string, in AgentSecretScope) error {
	if _, err := s.Pool.Exec(ctx, `INSERT INTO agent_secret_scopes (workspace_id, name, scope, repo)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, name) DO UPDATE SET scope = EXCLUDED.scope, repo = EXCLUDED.repo, updated_at = now()`,
		workspaceID, in.Name, in.Scope, in.Repo); err != nil {
		return fmt.Errorf("upsert agent secret scope: %w", err)
	}
	return nil
}

// DeleteAgentSecretScope removes one secret's scope row; a missing row is not
// an error.
func (s *PGStore) DeleteAgentSecretScope(ctx context.Context, workspaceID, name string) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM agent_secret_scopes WHERE workspace_id = $1 AND name = $2`, workspaceID, name); err != nil {
		return fmt.Errorf("delete agent secret scope: %w", err)
	}
	return nil
}
