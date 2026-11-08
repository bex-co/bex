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
// disconnected one is reconnected under a fresh generation, keeping its id and
// history, which revived reports (w4/120, as AdmitBlueprintCreate does).
func (s *PGStore) CreatePlannedBlueprint(ctx context.Context, b Blueprint) (out Blueprint, revived bool, err error) {
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
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Blueprint{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// prior reads the row's status before the upsert, the only way to tell a
	// revive from a fresh insert once the conflict arm has run.
	err = tx.QueryRow(ctx, `
		WITH prior AS (
			SELECT status FROM blueprints WHERE tenant_id = $2 AND repo = $4 AND branch = $5
		)
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
		          last_sync_at, created_at, updated_at, existing_resources, resource_suffix,
		          COALESCE((SELECT status FROM prior), '') = 'disconnected'`,
		b.ID, b.TenantID, b.Name, b.Repo, b.Branch, b.Path, b.AutoSync, b.ExistingResources, b.ResourceSuffix,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch, &out.Path, &out.AutoSync,
		&out.Manifest, &out.Status, &out.LastSyncAt, &out.CreatedAt, &out.UpdatedAt, &out.ExistingResources, &out.ResourceSuffix,
		&revived)
	if errors.Is(err, pgx.ErrNoRows) {
		return Blueprint{}, false, fmt.Errorf("blueprint: %w", ErrConflict)
	}
	if err != nil {
		return Blueprint{}, false, classify("blueprint", err)
	}
	// A reconnected Blueprint keeps its id and history; a plan left waiting by
	// the earlier connection must not be approvable under the new one.
	if _, err := tx.Exec(ctx, `
		UPDATE blueprint_syncs SET state = 'error', completed_at = now(),
			error_message = 'superseded: the Blueprint was reconnected'
		WHERE blueprint_id = $1 AND state = 'created'`, out.ID); err != nil {
		return Blueprint{}, false, err
	}
	return out, revived, tx.Commit(ctx)
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
			plan_path, required_manual_env_vars, plan_actions, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at`,
		p.ID, p.BlueprintID, p.CommitID, p.State, p.StartedAt, p.CompletedAt, p.ErrorMessage,
		p.Path, required, nullJSON(p.PlanActions), p.Note,
	).Scan(&p.CreatedAt)
	if err != nil {
		return PlannedBlueprintSync{}, classify("blueprint_sync", err)
	}
	p.RequiredManualEnvVars = required
	return p, nil
}

// LatestBlueprintPlan returns the newest planning attempt of the Blueprint's
// current connection — planned, failed, or since approved — or ErrNotFound
// when it has none. Ordinary sync runs (and busy refusals) are not planning
// attempts, and neither are an earlier connection's: a revived row keeps its
// history, but the connection that wrote it ended at the disconnect (its
// created_at is reset at every reconnect), so its applied plan must not
// answer for the new one (w2/m256 t014).
func (s *PGStore) LatestBlueprintPlan(ctx context.Context, blueprintID string) (PlannedBlueprintSync, error) {
	return s.scanPlannedSync(ctx, `WHERE blueprint_id = $1 AND plan_path IS NOT NULL
		AND created_at >= (SELECT b.created_at FROM blueprints b WHERE b.id = $1)
		ORDER BY created_at DESC, id DESC LIMIT 1`, blueprintID)
}

// SettleNoChangesPlan records a plan that found nothing to change: a waiting
// plan is superseded (approval applies the newest plan, and the newest needs
// nothing), and a Blueprint still `created` is in sync with manifest — its
// resources already match the file, which on Render needs no approval — so it
// leaves `created`, takes manifest as its stored file and stamps last_sync_at.
// A Blueprint in any other status keeps it. It returns the Blueprint as it
// now stands.
func (s *PGStore) SettleNoChangesPlan(ctx context.Context, blueprintID, tenantID, manifest string) (Blueprint, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Blueprint{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		UPDATE blueprint_syncs SET state = 'error', completed_at = now(),
			error_message = 'superseded: a newer plan found no changes to apply'
		WHERE blueprint_id = $1 AND state = 'created' AND plan_path IS NOT NULL
		  AND EXISTS (SELECT 1 FROM blueprints WHERE id = $1 AND tenant_id = $2)`, blueprintID, tenantID); err != nil {
		return Blueprint{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE blueprints SET status = 'in_sync', manifest = $3, last_sync_at = now(), updated_at = now()
		WHERE id = $1 AND tenant_id = $2 AND status = 'created' AND active_run_id IS NULL`, blueprintID, tenantID, manifest); err != nil {
		return Blueprint{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Blueprint{}, err
	}
	return s.GetBlueprint(ctx, blueprintID, tenantID)
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
