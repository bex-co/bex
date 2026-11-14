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
// and checks as JSON arrays of {name, run}, and how many setup steps it has.
// Version is the SHA-256 of the setup steps' canonical JSON (ADR111 D7),
// which a build records and a session's build must match.
type AgentRepoEnvironment struct {
	WorkspaceID string
	Repo        string
	Setup       json.RawMessage
	SetupSteps  int
	Checks      json.RawMessage
	Version     string
	UpdatedAt   time.Time
}

const agentRepoEnvironmentColumns = `workspace_id, repo, setup, jsonb_array_length(setup), checks, version, updated_at`

func scanAgentRepoEnvironment(row pgx.Row) (AgentRepoEnvironment, error) {
	var out AgentRepoEnvironment
	err := row.Scan(&out.WorkspaceID, &out.Repo, &out.Setup, &out.SetupSteps, &out.Checks, &out.Version, &out.UpdatedAt)
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

// UpsertAgentRepoEnvironment writes one repository's environment and reports
// whether the write changed its setup steps' version (ADR111 D3). A new
// repository in a workspace that already has maxRepos is
// ErrAgentRepoEnvironmentLimit; the count, the previous version and the write
// share a transaction under the workspace's advisory lock, so concurrent
// writes cannot pass the limit or both see the same change.
func (s *PGStore) UpsertAgentRepoEnvironment(ctx context.Context, workspaceID, repo string, setup, checks json.RawMessage, maxRepos int) (AgentRepoEnvironment, bool, error) {
	var out AgentRepoEnvironment
	var changed bool
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
		// The version hashes the setup's jsonb text, whose key order and
		// spacing Postgres fixes, as migration 0269's backfill does.
		return tx.QueryRow(ctx, `WITH previous AS (SELECT version FROM agent_repo_environments WHERE workspace_id = $1 AND repo = $2)
			INSERT INTO agent_repo_environments (workspace_id, repo, setup, checks, version)
			VALUES ($1, $2, $3, $4, encode(sha256(convert_to($3::jsonb::text, 'UTF8')), 'hex'))
			ON CONFLICT (workspace_id, repo) DO UPDATE
			SET setup = EXCLUDED.setup, checks = EXCLUDED.checks, version = EXCLUDED.version, updated_at = now()
			RETURNING `+agentRepoEnvironmentColumns+`, version IS DISTINCT FROM (SELECT version FROM previous)`,
			workspaceID, repo, setup, checks).Scan(&out.WorkspaceID, &out.Repo, &out.Setup, &out.SetupSteps, &out.Checks, &out.Version, &out.UpdatedAt, &changed)
	})
	if errors.Is(err, ErrAgentRepoEnvironmentLimit) {
		return AgentRepoEnvironment{}, false, err
	}
	if err != nil {
		return AgentRepoEnvironment{}, false, fmt.Errorf("upsert agent repo environment: %w", err)
	}
	return out, changed, nil
}

// DeleteAgentRepoEnvironment removes one repository's environment, or
// returns ErrNotFound. In the same transaction its queued and running builds
// are cancelled and every image its builds hold is retired (ADR111 D8), so no
// session boots from one afterwards.
func (s *PGStore) DeleteAgentRepoEnvironment(ctx context.Context, workspaceID, repo string) error {
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM agent_repo_environments WHERE workspace_id = $1 AND repo = $2`, workspaceID, repo)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_repo_environment_builds
			SET status = `+buildCancelledSQL+`, failure_reason = 'its environment was deleted', size_bytes = NULL, finished_at = now()
			WHERE workspace_id = $1 AND repo = $2 AND status IN (`+buildQueuedSQL+`, `+buildBuildingSQL+`)`, workspaceID, repo); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE agent_repo_environment_builds SET image_deleted_at = now()
			WHERE workspace_id = $1 AND repo = $2 AND image_digest IS NOT NULL AND image_deleted_at IS NULL`, workspaceID, repo)
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return err
	}
	if err != nil {
		return fmt.Errorf("delete agent repo environment: %w", err)
	}
	return nil
}
