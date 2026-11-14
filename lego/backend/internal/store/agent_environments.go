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

	"github.com/jackc/pgx/v5"
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
	// ModelKeySetAt is when the workspace admin last set its own model
	// provider key (ADR091 D7); nil means no BYO key. The key itself lives
	// only in OpenBao.
	ModelKeySetAt *time.Time
	// DisabledAgents are the profile ids the workspace's admins turned off;
	// DataUseAcks is each data-use acknowledgement an admin gave, by profile
	// id (w7/m392).
	DisabledAgents []string
	DataUseAcks    map[string]AgentDataUseAck
	UpdatedAt      time.Time
}

// AgentDataUseAck is one admin's acknowledgement that a profile's provider
// may train on what its sessions send.
type AgentDataUseAck struct {
	At time.Time `json:"at"`
	By string    `json:"by"`
}

// ModelKeySet reports whether the workspace has its own model provider key,
// which wins over the platform model route.
func (e AgentEnvironment) ModelKeySet() bool { return e.ModelKeySetAt != nil }

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
	// ModelKeySet records (true) or clears (false) the BYO model key flag;
	// nil keeps it.
	ModelKeySet *bool
	// DisabledAgents replaces the turned-off profile ids; nil keeps them.
	DisabledAgents *[]string
	// AcknowledgeDataUse records an acknowledgement by AcknowledgedBy for each
	// profile id that has none yet; an existing one is kept.
	AcknowledgeDataUse []string
	AcknowledgedBy     string
}

// GetAgentEnvironment returns the workspace's explicit row — ErrNotFound when
// it never customized the environment (the caller applies the defaults).
func (s *PGStore) GetAgentEnvironment(ctx context.Context, workspaceID string) (AgentEnvironment, error) {
	e := AgentEnvironment{WorkspaceID: workspaceID}
	err := s.Pool.QueryRow(ctx, `
		SELECT `+agentEnvironmentColumns+`
		FROM agent_environments WHERE workspace_id = $1`, workspaceID,
	).Scan(e.dest()...)
	if err != nil {
		return AgentEnvironment{}, classify("agent environment", err)
	}
	return e, nil
}

// UpsertAgentEnvironment applies patch in one statement, so two concurrent
// partial updates of different fields never lose each other's change.
func (s *PGStore) UpsertAgentEnvironment(ctx context.Context, workspaceID string, patch AgentEnvironmentPatch) (AgentEnvironment, error) {
	return upsertAgentEnvironment(ctx, s.Pool, workspaceID, patch)
}

// UpsertAgentEnvironmentLocked applies patch in one transaction that first
// locks workspaceID's row (inserting the defaults when it has none) and hands
// it to write. write's error rolls everything back, so the patch commits only
// after write succeeded, and concurrent callers serialize on the row lock: the
// write-only model key's OpenBao write and its flag never
// interleave with another admin's.
func (s *PGStore) UpsertAgentEnvironmentLocked(ctx context.Context, workspaceID string, patch AgentEnvironmentPatch, write func(ctx context.Context, current AgentEnvironment) error) (AgentEnvironment, error) {
	var out AgentEnvironment
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO agent_environments (workspace_id) VALUES ($1) ON CONFLICT (workspace_id) DO NOTHING`, workspaceID); err != nil {
			return classify("agent environment", err)
		}
		current := AgentEnvironment{WorkspaceID: workspaceID}
		if err := tx.QueryRow(ctx, `
			SELECT `+agentEnvironmentColumns+`
			FROM agent_environments WHERE workspace_id = $1 FOR UPDATE`, workspaceID,
		).Scan(current.dest()...); err != nil {
			return classify("agent environment", err)
		}
		if err := write(ctx, current); err != nil {
			return err
		}
		var err error
		out, err = upsertAgentEnvironment(ctx, tx, workspaceID, patch)
		return err
	})
	if err != nil {
		return AgentEnvironment{}, err
	}
	return out, nil
}

// agentEnvironmentColumns are the columns AgentEnvironment.dest scans.
const agentEnvironmentColumns = `network_tier, custom_hosts, default_model, agent_model_spend_cap_override_microusd, model_key_set_at, disabled_agents, agent_data_use_acks, updated_at`

func (e *AgentEnvironment) dest() []any {
	return []any{&e.NetworkTier, &e.CustomHosts, &e.DefaultModel, &e.SpendCapOverrideMicroUSD, &e.ModelKeySetAt, &e.DisabledAgents, &e.DataUseAcks, &e.UpdatedAt}
}

func upsertAgentEnvironment(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, workspaceID string, patch AgentEnvironmentPatch) (AgentEnvironment, error) {
	// nil keeps the stored hosts (SQL NULL); an explicit empty list clears them.
	var hosts any
	if patch.CustomHosts != nil {
		hosts = nonNil(*patch.CustomHosts)
	}
	var disabled any
	if patch.DisabledAgents != nil {
		disabled = nonNil(*patch.DisabledAgents)
	}
	acks := map[string]AgentDataUseAck{}
	for _, agent := range patch.AcknowledgeDataUse {
		acks[agent] = AgentDataUseAck{At: time.Now().UTC(), By: patch.AcknowledgedBy}
	}
	e := AgentEnvironment{WorkspaceID: workspaceID}
	err := q.QueryRow(ctx, `
		INSERT INTO agent_environments (workspace_id, network_tier, custom_hosts, default_model, agent_model_spend_cap_override_microusd, model_key_set_at, disabled_agents, agent_data_use_acks)
		VALUES ($1, COALESCE($2, 'registries'), COALESCE($3::text[], '{}'), COALESCE($4, ''),
		        CASE WHEN $6::boolean THEN NULL ELSE $5::bigint END,
		        CASE WHEN $7::boolean THEN now() END,
		        COALESCE($8::text[], '{}'), $9::jsonb)
		ON CONFLICT (workspace_id) DO UPDATE
			SET network_tier  = COALESCE($2, agent_environments.network_tier),
			    custom_hosts  = COALESCE($3::text[], agent_environments.custom_hosts),
			    default_model = COALESCE($4, agent_environments.default_model),
			    agent_model_spend_cap_override_microusd = CASE
			        WHEN $6::boolean THEN NULL
			        ELSE COALESCE($5::bigint, agent_environments.agent_model_spend_cap_override_microusd) END,
			    model_key_set_at = CASE
			        WHEN $7::boolean IS NULL THEN agent_environments.model_key_set_at
			        WHEN $7::boolean THEN now() END,
			    disabled_agents = COALESCE($8::text[], agent_environments.disabled_agents),
			    -- The stored acknowledgement wins: the first admin's is kept.
			    agent_data_use_acks = $9::jsonb || agent_environments.agent_data_use_acks,
			    updated_at    = now()
		RETURNING `+agentEnvironmentColumns,
		workspaceID, patch.NetworkTier, hosts, patch.DefaultModel, patch.SpendCapOverrideMicroUSD, patch.ClearSpendCapOverride, patch.ModelKeySet,
		disabled, acks,
	).Scan(e.dest()...)
	if err != nil {
		return AgentEnvironment{}, classify("agent environment", err)
	}
	return e, nil
}

// MarkAgentModelKeySet records that workspaceID has its own model key when
// no flag is recorded yet, and never clears one: the startup backfill of keys
// put in OpenBao by hand before the flag existed. It reports
// whether it set the flag; a workspace that no longer exists is ErrNotFound.
func (s *PGStore) MarkAgentModelKeySet(ctx context.Context, workspaceID string) (bool, error) {
	var set bool
	err := s.Pool.QueryRow(ctx, `
		WITH marked AS (
			INSERT INTO agent_environments (workspace_id, model_key_set_at) VALUES ($1, now())
			ON CONFLICT (workspace_id) DO UPDATE SET model_key_set_at = now(), updated_at = now()
				WHERE agent_environments.model_key_set_at IS NULL
			RETURNING 1)
		SELECT EXISTS (SELECT 1 FROM marked)`, workspaceID).Scan(&set)
	if err != nil {
		return false, classify("agent environment", err)
	}
	return set, nil
}
