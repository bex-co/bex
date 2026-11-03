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
	"strings"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Render's Blueprint onboarding (w2/m256): planned syncs and the existing-
// resources mode.
const (
	BlueprintExistingAdopt     = "adopt"
	BlueprintExistingCreateNew = "create_new"
)

// PlannedBlueprintSync is a planning attempt: a sync planned but not applied
// (or a failed plan), the path it was planned at, and the manual env vars
// approval must supply (JSON the apps layer owns).
type PlannedBlueprintSync struct {
	BlueprintSync
	Path                  string
	RequiredManualEnvVars json.RawMessage
	// PlanActions is the plan's actions (JSON the apps layer owns), null for a
	// failed plan.
	PlanActions json.RawMessage
}

// CreatePlannedBlueprint inserts a Blueprint in the `created` status: its
// first sync is planned, not applied, and auto-sync does not run until a sync
// is approved. A live Blueprint on the same repo and branch is a conflict; a
// disconnected one is reconnected under a fresh generation.
func (s *PGStore) CreatePlannedBlueprint(ctx context.Context, b Blueprint) (Blueprint, error) {
	if b.ID == "" {
		b.ID = ids.New(ids.Blueprint)
	}
	if b.ExistingResources == "" {
		b.ExistingResources = BlueprintExistingAdopt
	}
	// create_new's suffix is minted once, here, and kept for the life of the
	// connection: every later plan and sync names the same suffixed resources
	// (w2/m261). Render's shape, like the onrender.com slug suffix.
	if b.ExistingResources == BlueprintExistingCreateNew && b.ResourceSuffix == "" {
		b.ResourceSuffix = randomSlugSuffix()
	}
	var out Blueprint
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Blueprint{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `
		INSERT INTO blueprints (id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
			existing_resources, resource_suffix, execution_generation)
		VALUES ($1, $2, $3, $4, $5, $6, $7, '', 'created', $8, $9, 1)
		ON CONFLICT (tenant_id, repo, branch) DO UPDATE SET
			name = EXCLUDED.name, path = EXCLUDED.path, auto_sync = EXCLUDED.auto_sync,
			manifest = '', status = 'created', last_sync_at = NULL,
			existing_resources = EXCLUDED.existing_resources, resource_suffix = EXCLUDED.resource_suffix,
			execution_generation = blueprints.execution_generation + 1,
			created_at = now(), updated_at = now()
		WHERE blueprints.status = 'disconnected' AND blueprints.active_run_id IS NULL
		RETURNING id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		          last_sync_at, created_at, updated_at, existing_resources, resource_suffix`,
		b.ID, b.TenantID, b.Name, b.Repo, b.Branch, b.Path, b.AutoSync, b.ExistingResources, b.ResourceSuffix,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch, &out.Path, &out.AutoSync,
		&out.Manifest, &out.Status, &out.LastSyncAt, &out.CreatedAt, &out.UpdatedAt, &out.ExistingResources, &out.ResourceSuffix)
	if errors.Is(err, pgx.ErrNoRows) {
		return Blueprint{}, fmt.Errorf("blueprint: %w", ErrConflict)
	}
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	// A reconnected Blueprint keeps its id and history; a plan left waiting by
	// the earlier connection must not be approvable under the new one.
	if _, err := tx.Exec(ctx, `
		UPDATE blueprint_syncs SET state = 'error', completed_at = now(),
			error_message = 'superseded: the Blueprint was reconnected'
		WHERE blueprint_id = $1 AND state = 'created'`, out.ID); err != nil {
		return Blueprint{}, err
	}
	return out, tx.Commit(ctx)
}

// BlueprintNameTaken reports whether another live, approved Blueprint in the
// workspace already uses name, ignoring case (Render: unique among approved
// Blueprints).
func (s *PGStore) BlueprintNameTaken(ctx context.Context, tenantID, name, exceptID string) (bool, error) {
	var taken bool
	err := s.Pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM blueprints
		 WHERE tenant_id = $1 AND lower(name) = lower($2) AND id <> $3
		   AND status NOT IN ('disconnected', 'created'))`, tenantID, strings.TrimSpace(name), exceptID).Scan(&taken)
	return taken, err
}

// InsertPlannedBlueprintSync records a planning attempt: a `created` row for
// a plan awaiting approval, or an `error` row for a planning failure.
func (s *PGStore) InsertPlannedBlueprintSync(ctx context.Context, p PlannedBlueprintSync) (PlannedBlueprintSync, error) {
	if p.ID == "" {
		p.ID = ids.New(ids.BlueprintSync)
	}
	if (p.State != BlueprintSyncStateCreated && p.State != BlueprintSyncStateError) || p.Path == "" {
		return PlannedBlueprintSync{}, fmt.Errorf("planned sync state %q path %q: %w", p.State, p.Path, ErrInvalid)
	}
	required := p.RequiredManualEnvVars
	if len(required) == 0 {
		required = json.RawMessage("[]")
	}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO blueprint_syncs (id, blueprint_id, commit_id, state, started_at, completed_at, error_message,
			plan_path, required_manual_env_vars, plan_actions)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at`,
		p.ID, p.BlueprintID, p.CommitID, p.State, p.StartedAt, p.CompletedAt, p.ErrorMessage,
		p.Path, required, nullJSON(p.PlanActions),
	).Scan(&p.CreatedAt)
	if err != nil {
		return PlannedBlueprintSync{}, classify("blueprint_sync", err)
	}
	p.RequiredManualEnvVars = required
	return p, nil
}

// LatestBlueprintPlan returns the Blueprint's newest planning attempt —
// planned, failed, or since approved — or ErrNotFound when it has none.
// Ordinary sync runs (and busy refusals) are not planning attempts.
func (s *PGStore) LatestBlueprintPlan(ctx context.Context, blueprintID string) (PlannedBlueprintSync, error) {
	return s.scanPlannedSync(ctx, `WHERE blueprint_id = $1 AND plan_path IS NOT NULL
		ORDER BY created_at DESC, id DESC LIMIT 1`, blueprintID)
}

// BlueprintPlannedSync returns one sync by id within its Blueprint.
func (s *PGStore) BlueprintPlannedSync(ctx context.Context, blueprintID, syncID string) (PlannedBlueprintSync, error) {
	return s.scanPlannedSync(ctx, `WHERE blueprint_id = $1 AND id = $2`, blueprintID, syncID)
}

// nullJSON stores an empty document as SQL NULL.
func nullJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func (s *PGStore) scanPlannedSync(ctx context.Context, where string, args ...any) (PlannedBlueprintSync, error) {
	var p PlannedBlueprintSync
	var path *string
	err := s.Pool.QueryRow(ctx, `
		SELECT id, blueprint_id, commit_id, state, started_at, completed_at, created_at, error_message,
		       plan_path, required_manual_env_vars, plan_actions
		FROM blueprint_syncs `+where, args...,
	).Scan(&p.ID, &p.BlueprintID, &p.CommitID, &p.State, &p.StartedAt, &p.CompletedAt, &p.CreatedAt, &p.ErrorMessage,
		&path, &p.RequiredManualEnvVars, &p.PlanActions)
	if err != nil {
		return PlannedBlueprintSync{}, classify("blueprint_sync", err)
	}
	if path != nil {
		p.Path = *path
	}
	return p, nil
}
