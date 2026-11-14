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

// ErrAgentRepoEnvironmentLimit is an upsert of a new repository into a
// workspace already at its limit.
var ErrAgentRepoEnvironmentLimit = errors.New("agent repo environment limit reached")

// AgentRepoEnvironment is one repository's environment (ADR109): setup steps
// and checks as JSON arrays of {name, run}.
type AgentRepoEnvironment struct {
	WorkspaceID string
	Repo        string
	Setup       json.RawMessage
	Checks      json.RawMessage
	UpdatedAt   time.Time
}

const agentRepoEnvironmentColumns = `workspace_id, repo, setup, checks, updated_at`

func scanAgentRepoEnvironment(row pgx.Row) (AgentRepoEnvironment, error) {
	var out AgentRepoEnvironment
	err := row.Scan(&out.WorkspaceID, &out.Repo, &out.Setup, &out.Checks, &out.UpdatedAt)
	return out, err
}

// ListAgentRepoEnvironments returns the workspace's environments by repository.
func (s *PGStore) ListAgentRepoEnvironments(ctx context.Context, workspaceID string) ([]AgentRepoEnvironment, error) {
	rows, err := collectWith(scanAgentRepoEnvironment)(s.Pool.Query(ctx, `SELECT `+agentRepoEnvironmentColumns+`
		FROM agent_repo_environments WHERE workspace_id = $1 ORDER BY repo`, workspaceID))
	if err != nil {
		return nil, fmt.Errorf("list agent repo environments: %w", err)
	}
	return rows, nil
}

// GetAgentRepoEnvironment returns one repository's environment, or ErrNotFound.
func (s *PGStore) GetAgentRepoEnvironment(ctx context.Context, workspaceID, repo string) (AgentRepoEnvironment, error) {
	out, err := scanAgentRepoEnvironment(s.Pool.QueryRow(ctx, `SELECT `+agentRepoEnvironmentColumns+`
		FROM agent_repo_environments WHERE workspace_id = $1 AND repo = $2`, workspaceID, repo))
	if err != nil {
		return AgentRepoEnvironment{}, classify("agent repo environment", err)
	}
	return out, nil
}

// UpsertAgentRepoEnvironment writes one repository's environment. A new
// repository in a workspace that already has maxRepos is
// ErrAgentRepoEnvironmentLimit; the count and the insert share a transaction
// under the workspace's advisory lock, so concurrent writes cannot pass it.
func (s *PGStore) UpsertAgentRepoEnvironment(ctx context.Context, workspaceID, repo string, setup, checks json.RawMessage, maxRepos int) (AgentRepoEnvironment, error) {
	var out AgentRepoEnvironment
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockAgentRepoEnvironments(ctx, tx, workspaceID); err != nil {
			return err
		}
		var existing, total int
		if err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE repo = $2), count(*)
			FROM agent_repo_environments WHERE workspace_id = $1`, workspaceID, repo).Scan(&existing, &total); err != nil {
			return err
		}
		if existing == 0 && total >= maxRepos {
			return ErrAgentRepoEnvironmentLimit
		}
		var err error
		out, err = scanAgentRepoEnvironment(tx.QueryRow(ctx, `INSERT INTO agent_repo_environments (workspace_id, repo, setup, checks)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (workspace_id, repo) DO UPDATE SET setup = EXCLUDED.setup, checks = EXCLUDED.checks, updated_at = now()
			RETURNING `+agentRepoEnvironmentColumns, workspaceID, repo, setup, checks))
		return err
	})
	if errors.Is(err, ErrAgentRepoEnvironmentLimit) {
		return AgentRepoEnvironment{}, err
	}
	if err != nil {
		return AgentRepoEnvironment{}, fmt.Errorf("upsert agent repo environment: %w", err)
	}
	return out, nil
}

// DeleteAgentRepoEnvironment removes one repository's environment, or
// returns ErrNotFound.
func (s *PGStore) DeleteAgentRepoEnvironment(ctx context.Context, workspaceID, repo string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM agent_repo_environments WHERE workspace_id = $1 AND repo = $2`, workspaceID, repo)
	if err != nil {
		return fmt.Errorf("delete agent repo environment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
