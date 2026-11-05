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

// AgentEnvironment is a workspace's agent-session environment row
// (w10/m17): the sandbox network tier, the custom hostnames the `custom` tier
// opens, and the default model. A workspace with no row has the defaults.
type AgentEnvironment struct {
	WorkspaceID  string
	NetworkTier  string
	CustomHosts  []string
	DefaultModel string
	// SpendCapOverrideMicroUSD is the admin's monthly agent-model spend cap
	// (ADR091 D5); nil means the plan default.
	SpendCapOverrideMicroUSD *int64
	UpdatedAt                time.Time
}

// AgentEnvironmentPatch is a partial update: a nil field keeps the stored
// value (or the column default on first write).
type AgentEnvironmentPatch struct {
	NetworkTier  *string
	CustomHosts  *[]string
	DefaultModel *string
	// SpendCapOverrideMicroUSD sets the override; ClearSpendCapOverride returns
	// to the plan default (and wins over a set value).
	SpendCapOverrideMicroUSD *int64
	ClearSpendCapOverride    bool
}

// GetAgentEnvironment returns the workspace's explicit row — ErrNotFound when
// it never customized the environment (the caller applies the defaults).
func (s *PGStore) GetAgentEnvironment(ctx context.Context, workspaceID string) (AgentEnvironment, error) {
	e := AgentEnvironment{WorkspaceID: workspaceID}
	err := s.Pool.QueryRow(ctx, `
		SELECT network_tier, custom_hosts, default_model, agent_model_spend_cap_override_microusd, updated_at
		FROM agent_environments WHERE workspace_id = $1`, workspaceID,
	).Scan(&e.NetworkTier, &e.CustomHosts, &e.DefaultModel, &e.SpendCapOverrideMicroUSD, &e.UpdatedAt)
	if err != nil {
		return AgentEnvironment{}, classify("agent environment", err)
	}
	return e, nil
}

// UpsertAgentEnvironment applies patch in one statement, so two concurrent
// partial updates of different fields never lose each other's change.
func (s *PGStore) UpsertAgentEnvironment(ctx context.Context, workspaceID string, patch AgentEnvironmentPatch) (AgentEnvironment, error) {
	// nil keeps the stored hosts (SQL NULL); an explicit empty list clears them.
	var hosts any
	if patch.CustomHosts != nil {
		hosts = nonNil(*patch.CustomHosts)
	}
	e := AgentEnvironment{WorkspaceID: workspaceID}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO agent_environments (workspace_id, network_tier, custom_hosts, default_model, agent_model_spend_cap_override_microusd)
		VALUES ($1, COALESCE($2, 'registries'), COALESCE($3::text[], '{}'), COALESCE($4, ''),
		        CASE WHEN $6::boolean THEN NULL ELSE $5::bigint END)
		ON CONFLICT (workspace_id) DO UPDATE
			SET network_tier  = COALESCE($2, agent_environments.network_tier),
			    custom_hosts  = COALESCE($3::text[], agent_environments.custom_hosts),
			    default_model = COALESCE($4, agent_environments.default_model),
			    agent_model_spend_cap_override_microusd = CASE
			        WHEN $6::boolean THEN NULL
			        ELSE COALESCE($5::bigint, agent_environments.agent_model_spend_cap_override_microusd) END,
			    updated_at    = now()
		RETURNING network_tier, custom_hosts, default_model, agent_model_spend_cap_override_microusd, updated_at`,
		workspaceID, patch.NetworkTier, hosts, patch.DefaultModel, patch.SpendCapOverrideMicroUSD, patch.ClearSpendCapOverride,
	).Scan(&e.NetworkTier, &e.CustomHosts, &e.DefaultModel, &e.SpendCapOverrideMicroUSD, &e.UpdatedAt)
	if err != nil {
		return AgentEnvironment{}, classify("agent environment", err)
	}
	return e, nil
}
