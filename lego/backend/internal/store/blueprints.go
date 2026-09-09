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

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Blueprint is a row of `blueprints` — a workspace-scoped render.yaml stack source.
// Created automatically when deploy is called with a repo+manifest; sync
// re-applies the stored manifest (optionally replacing it first).
// w2/m62: path/AutoSync/LastSyncAt added for Git-connected instance semantics.
type Blueprint struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenantId"`
	Name       string     `json:"name"`
	Repo       string     `json:"repo"`
	Branch     string     `json:"branch"`
	Path       string     `json:"path"`
	AutoSync   bool       `json:"autoSync"`
	Manifest   string     `json:"manifest"`
	Status     string     `json:"status"`
	LastSyncAt *time.Time `json:"lastSyncAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	// ExecutionGeneration is the lifecycle fencing counter (w8/m37, migration
	// 0111): bumped by every admission, disconnect, and explicit
	// re-creation. Only the Admit/Complete paths populate it; the remaining
	// readers leave it zero rather than pay for columns they never use.
	ExecutionGeneration int64 `json:"-"`
	// ActiveRunID names the one admitted-but-uncompleted run, "" when idle.
	// Internal to lifecycle coordination; never rendered on API views.
	ActiveRunID string `json:"-"`
}

// BlueprintSync is a row of `blueprint_syncs` — one recorded sync run.
// State machine: created → running → success | error.
type BlueprintSync struct {
	ID           string     `json:"id"`
	BlueprintID  string     `json:"blueprintId"`
	CommitID     string     `json:"commitId"`
	State        string     `json:"state"`
	StartedAt    time.Time  `json:"startedAt"`
	CompletedAt  *time.Time `json:"completedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	ErrorMessage *string    `json:"errorMessage"`
	// ExecutionGeneration records which admission a run belongs to (w8/m37,
	// migration 0111) so recovery and completion never settle a run against a
	// newer generation. Internal; never rendered on API views.
	ExecutionGeneration int64 `json:"-"`
}

// Blueprint sync state constants (Render's vocabulary).
const (
	BlueprintSyncStateCreated = "created"
	BlueprintSyncStateRunning = "running"
	BlueprintSyncStateSuccess = "success"
	BlueprintSyncStateError   = "error"
)

// Blueprint status constants (Render's vocabulary). The legacy 'active' value
// stored in pre-m62 rows is mapped to 'in_sync' at the service layer.
const (
	BlueprintStatusCreated = "created"
	BlueprintStatusPaused  = "paused"
	BlueprintStatusInSync  = "in_sync"
	BlueprintStatusSyncing = "syncing"
	BlueprintStatusError   = "error"
)

// UpsertBlueprint creates a blueprint or refreshes its manifest when
// (tenant_id, repo, branch) already exists. The id field is ignored on
// conflict — the existing row's id is preserved. Returns the current row.
//
// The conflict arm writes the manifest ONLY (w8/m37 t005): deploy-time
// auto-registration must never clobber current name/path/autoSync settings or
// lifecycle status with deploy-request defaults — a direct deploy racing an
// admitted sync (or a mid-run settings change) keeps the row's settings.
// It also refuses disconnected rows (re-establishment is confined to explicit
// creation through AdmitBlueprintCreate, which bumps the execution
// generation). Generation and active-run ownership are never written here —
// an auto-register racing a live sync must not fence its own admission.
func (s *PGStore) UpsertBlueprint(ctx context.Context, b Blueprint) (Blueprint, error) {
	if b.ID == "" {
		b.ID = ids.New(ids.Blueprint)
	}
	if b.Path == "" {
		b.Path = "render.yaml"
	}
	var out Blueprint
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO blueprints (id, tenant_id, name, repo, branch, path, auto_sync, manifest, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, repo, branch)
		DO UPDATE SET
			manifest   = EXCLUDED.manifest,
			updated_at = now()
		WHERE blueprints.status != 'disconnected'
		RETURNING id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		          last_sync_at, created_at, updated_at`,
		b.ID, b.TenantID, b.Name, b.Repo, b.Branch, b.Path, b.AutoSync, b.Manifest, b.Status,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch, &out.Path,
		&out.AutoSync, &out.Manifest, &out.Status, &out.LastSyncAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	return out, nil
}

// GetBlueprint fetches an active blueprint by id, scoped to tenantID (returns
// ErrNotFound if the id belongs to a different workspace — or names a
// disconnected row, which ordinary reads treat as absent, w8/m37 t001).
func (s *PGStore) GetBlueprint(ctx context.Context, id, tenantID string) (Blueprint, error) {
	var out Blueprint
	err := s.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		        last_sync_at, created_at, updated_at
		 FROM blueprints WHERE id = $1 AND tenant_id = $2 AND status != 'disconnected'`,
		id, tenantID,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch, &out.Path,
		&out.AutoSync, &out.Manifest, &out.Status, &out.LastSyncAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	return out, nil
}

// GetBlueprintByRepo fetches the active blueprint for a tenant+repo+branch, used
// by the push-webhook auto-sync path. Returns ErrNotFound when unregistered —
// or when the row is disconnected, so webhooks cannot resurrect one (w8/m37).
func (s *PGStore) GetBlueprintByRepo(ctx context.Context, tenantID, repo, branch string) (Blueprint, error) {
	var out Blueprint
	err := s.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		        last_sync_at, created_at, updated_at
		 FROM blueprints WHERE tenant_id = $1 AND repo = $2 AND branch = $3
		   AND status != 'disconnected'
		 LIMIT 1`,
		tenantID, repo, branch,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch, &out.Path,
		&out.AutoSync, &out.Manifest, &out.Status, &out.LastSyncAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	return out, nil
}

// ListBlueprints returns all non-disconnected blueprints for a tenant, newest first.
func (s *PGStore) ListBlueprints(ctx context.Context, tenantID string) ([]Blueprint, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		        last_sync_at, created_at, updated_at
		 FROM blueprints WHERE tenant_id = $1 AND status != 'disconnected'
		 ORDER BY created_at DESC`,
		tenantID,
	)
	if err != nil {
		return nil, classify("blueprint", err)
	}
	defer rows.Close()
	var out []Blueprint
	for rows.Next() {
		var b Blueprint
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Name, &b.Repo, &b.Branch, &b.Path,
			&b.AutoSync, &b.Manifest, &b.Status, &b.LastSyncAt, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateBlueprint applies a partial update (only non-zero pointer fields are
// changed). Returns the updated row. Disconnected rows are untouched —
// ordinary updates cannot reactivate one (w8/m37 t001).
func (s *PGStore) UpdateBlueprint(ctx context.Context, id, tenantID string, name *string, autoSync *bool, path *string, status *string, lastSyncAt *time.Time) (Blueprint, error) {
	var out Blueprint
	err := s.Pool.QueryRow(ctx,
		`UPDATE blueprints SET
			name        = COALESCE($3, name),
			auto_sync   = COALESCE($4, auto_sync),
			path        = COALESCE($5, path),
			status      = COALESCE($6, status),
			last_sync_at = COALESCE($7, last_sync_at),
			updated_at  = now()
		 WHERE id = $1 AND tenant_id = $2 AND status != 'disconnected'
		 RETURNING id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		           last_sync_at, created_at, updated_at`,
		id, tenantID, name, autoSync, path, status, lastSyncAt,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch, &out.Path,
		&out.AutoSync, &out.Manifest, &out.Status, &out.LastSyncAt, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	return out, nil
}

// InsertBlueprintSync inserts a new sync run row. Terminal fields travel with
// the insert so a pre-admission failure lands directly in its final error
// state — one statement, no stranded running row (w8/m37 t002 failSync).
func (s *PGStore) InsertBlueprintSync(ctx context.Context, run BlueprintSync) (BlueprintSync, error) {
	if run.ID == "" {
		run.ID = ids.New(ids.BlueprintSync)
	}
	var out BlueprintSync
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO blueprint_syncs (id, blueprint_id, commit_id, state, started_at, completed_at, error_message, execution_generation)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, blueprint_id, commit_id, state, started_at, completed_at, created_at, error_message, execution_generation`,
		run.ID, run.BlueprintID, run.CommitID, run.State, run.StartedAt, run.CompletedAt, run.ErrorMessage, run.ExecutionGeneration,
	).Scan(&out.ID, &out.BlueprintID, &out.CommitID, &out.State,
		&out.StartedAt, &out.CompletedAt, &out.CreatedAt, &out.ErrorMessage, &out.ExecutionGeneration)
	if err != nil {
		return BlueprintSync{}, classify("blueprint_sync", err)
	}
	return out, nil
}

// UpdateBlueprintSync updates a sync run's state, completion timestamp, and
// error reason (nil on success, the failure's message on error).
func (s *PGStore) UpdateBlueprintSync(ctx context.Context, id, state string, completedAt *time.Time, errMsg *string) (BlueprintSync, error) {
	var out BlueprintSync
	err := s.Pool.QueryRow(ctx,
		`UPDATE blueprint_syncs SET state = $2, completed_at = $3, error_message = $4
		 WHERE id = $1
		 RETURNING id, blueprint_id, commit_id, state, started_at, completed_at, created_at, error_message`,
		id, state, completedAt, errMsg,
	).Scan(&out.ID, &out.BlueprintID, &out.CommitID, &out.State,
		&out.StartedAt, &out.CompletedAt, &out.CreatedAt, &out.ErrorMessage)
	if err != nil {
		return BlueprintSync{}, classify("blueprint_sync", err)
	}
	return out, nil
}

// ListBlueprintSyncs returns sync runs for a blueprint, newest first, with
// cursor-based paging (exclusive, by started_at DESC + id).
func (s *PGStore) ListBlueprintSyncs(ctx context.Context, blueprintID, cursor string, limit int) ([]BlueprintSync, error) {
	// core's shared bounds, not hand-spelled literals. The previous form
	// (`<= 0 || > 100 ⇒ 20`) folded the oversized arm in with the absent one,
	// so an oversized limit got the default page instead of the max. The
	// service layer clamps before calling and can no longer pass an oversized
	// value down, so this was the same defect one layer deeper — latent rather
	// than live, and the copy the service's own version was written from.
	limit = core.PageLimitOrAbsent(limit)
	var rows interface {
		Next() bool
		Scan(...any) error
		Close()
		Err() error
	}
	var err error
	if cursor == "" {
		rows, err = s.Pool.Query(ctx,
			`SELECT id, blueprint_id, commit_id, state, started_at, completed_at, created_at, error_message
			 FROM blueprint_syncs WHERE blueprint_id = $1
			 ORDER BY started_at DESC, id DESC LIMIT $2`,
			blueprintID, limit)
	} else {
		// Resume after cursor: find the cursor row's (started_at, id) then page past it.
		rows, err = s.Pool.Query(ctx,
			`SELECT bs.id, bs.blueprint_id, bs.commit_id, bs.state, bs.started_at, bs.completed_at, bs.created_at, bs.error_message
			 FROM blueprint_syncs bs
			 WHERE bs.blueprint_id = $1
			   AND (bs.started_at, bs.id) < (
			       SELECT started_at, id FROM blueprint_syncs WHERE id = $2
			   )
			 ORDER BY bs.started_at DESC, bs.id DESC LIMIT $3`,
			blueprintID, cursor, limit)
	}
	if err != nil {
		return nil, classify("blueprint_sync", err)
	}
	defer rows.Close()
	var out []BlueprintSync
	for rows.Next() {
		var r BlueprintSync
		if err := rows.Scan(&r.ID, &r.BlueprintID, &r.CommitID, &r.State,
			&r.StartedAt, &r.CompletedAt, &r.CreatedAt, &r.ErrorMessage); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
