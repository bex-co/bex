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
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

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
	// ExistingResources is Render's existingResources mode (adopt | create_new)
	// and ResourceSuffix the create_new suffix (w2/m256). Read by GetBlueprint;
	// other readers leave them empty, which means adopt with no suffix.
	ExistingResources string `json:"-"`
	ResourceSuffix    string `json:"-"`
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
	// Note annotates what the run did when its state cannot say it — today,
	// the resources a sync detached from the Blueprint (migration 0131).
	// Empty means nothing to report, the same convention
	// deploys.stall_reason uses.
	Note string `json:"note,omitempty"`
	// ExecutionGeneration records which admission a run belongs to (w8/m37,
	// migration 0111) so recovery and completion never settle a run against a
	// newer generation. Internal; never rendered on API views.
	ExecutionGeneration int64 `json:"-"`
	// Automatic runs carry both the accepted intent's lease and the actual
	// source snapshot they resolved. Manual reviewed commits have no intent.
	AutoIntentID        string `json:"-"`
	AutoClaimGeneration int64  `json:"-"`
	AutoSourceRepo      string `json:"-"`
	AutoSourceBranch    string `json:"-"`
	AutoSourcePath      string `json:"-"`
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

// UpsertBlueprint registers the Blueprint that a direct deploy of a repo's
// manifest stands for, or refreshes that Blueprint's manifest. Returns the
// current row.
//
// A deploy applies the manifest's own names, so the Blueprint it stands for is
// the one that manages unsuffixed resources: the oldest live row for
// (tenant_id, repo, branch) with no create_new suffix. Since a workspace may
// hold several Blueprints for one source (w8/m153, migration 0229) the choice
// is made here, under a per-source lock, instead of by a unique key.
//
// The refresh writes the manifest ONLY (w8/m37 t005): deploy-time
// auto-registration must never clobber current name/path/autoSync settings or
// lifecycle status with deploy-request defaults — a direct deploy racing an
// admitted sync (or a mid-run settings change) keeps the row's settings.
// Generation and active-run ownership are never written here — an
// auto-register racing a live sync must not fence its own admission.
//
// A source whose unsuffixed Blueprint was deleted is not registered again: the
// deletion was explicit, and only an explicit create connects the repo again.
// That case answers ErrNotFound, as it did when the deleted row held the key.
func (s *PGStore) UpsertBlueprint(ctx context.Context, b Blueprint) (Blueprint, error) {
	if b.ID == "" {
		b.ID = ids.New(ids.Blueprint)
	}
	if b.Path == "" {
		b.Path = "render.yaml"
	}
	var out Blueprint
	var active string
	scan := func(row pgx.Row) error { return row.Scan(scanBlueprint(&out, &active)...) }
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockBlueprintSource(ctx, tx, b.TenantID, b.Repo, b.Branch); err != nil {
			return err
		}
		err := scan(tx.QueryRow(ctx, `
			UPDATE blueprints SET manifest = $4, updated_at = now()
			WHERE id = (
				SELECT id FROM blueprints
				WHERE tenant_id = $1 AND repo = $2 AND branch = $3
				  AND status != 'disconnected' AND resource_suffix = ''
				ORDER BY created_at, id LIMIT 1)
			RETURNING `+blueprintColumns, b.TenantID, b.Repo, b.Branch, b.Manifest))
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return scan(tx.QueryRow(ctx, `
			INSERT INTO blueprints (id, tenant_id, name, repo, branch, path, auto_sync, manifest, status)
			SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9
			WHERE NOT EXISTS (
				SELECT 1 FROM blueprints
				WHERE tenant_id = $2 AND repo = $4 AND branch = $5
				  AND status = 'disconnected' AND resource_suffix = '')
			RETURNING `+blueprintColumns,
			b.ID, b.TenantID, b.Name, b.Repo, b.Branch, b.Path, b.AutoSync, b.Manifest, b.Status))
	})
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	out.ActiveRunID = active
	return out, nil
}

// lockBlueprintSource serializes, for the rest of tx, the writers that decide
// by source whether a Blueprint row is reused or a new one is inserted. The
// unique key on (tenant_id, repo, branch) used to arbitrate that; since 0229
// two racing writers would otherwise each insert.
func lockBlueprintSource(ctx context.Context, tx pgx.Tx, tenantID, repo, branch string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"blueprint-source|"+tenantID+"|"+repo+"|"+branch)
	return err
}

// GetBlueprint fetches an active blueprint by id, scoped to tenantID (returns
// ErrNotFound if the id belongs to a different workspace — or names a
// disconnected row, which ordinary reads treat as absent, w8/m37 t001).
// The id is normalized to its blp- storage form as a deliberate backstop:
// apps' blueprintScope already canonicalizes, but a caller that reaches the
// store with the public exs- alias still finds the row (w2/m269).
func (s *PGStore) GetBlueprint(ctx context.Context, id, tenantID string) (Blueprint, error) {
	var out Blueprint
	err := s.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		        last_sync_at, created_at, updated_at, existing_resources, resource_suffix
		 FROM blueprints WHERE id = $1 AND tenant_id = $2 AND status != 'disconnected'`,
		ids.BlueprintStorageID(id), tenantID,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch, &out.Path,
		&out.AutoSync, &out.Manifest, &out.Status, &out.LastSyncAt, &out.CreatedAt, &out.UpdatedAt,
		&out.ExistingResources, &out.ResourceSuffix)
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	return out, nil
}

// BlueprintWorkspace returns the workspace that owns a live (non-disconnected)
// Blueprint id, or ErrNotFound. It is a routing read only: the caller must
// still authorize against the returned workspace before revealing anything,
// exactly as a service or env-group id resolves to its own workspace (w4/m169).
// Like GetBlueprint it normalizes an exs- alias to blp- as a backstop.
func (s *PGStore) BlueprintWorkspace(ctx context.Context, id string) (string, error) {
	var tenantID string
	err := s.Pool.QueryRow(ctx,
		`SELECT tenant_id FROM blueprints WHERE id = $1 AND status != 'disconnected'`, ids.BlueprintStorageID(id),
	).Scan(&tenantID)
	if err != nil {
		return "", classify("blueprint", err)
	}
	return tenantID, nil
}

// blueprintCreateRetryMatch selects the Blueprint a repeated one-step create
// retries instead of adding another: the newest live row for the same file, in
// the same existingResources mode, that no sync has ever brought in sync.
// $1..$5 are tenant_id, repo, branch, path and mode.
//
// last_sync_at cannot answer that (a failed run stamps it too), so the test is
// the absence of a successful run on a row that is applying or failed. A
// `created` row is a planned create waiting for approval, not a failed one,
// and `in_sync` (or the pre-m62 `active`) has been in sync even when no run
// says so, as after a plan that found nothing to change. A Blueprint that has
// ever been planned came from Render's create, which undoes its own failures;
// its failed first sync is retried by planning again, never by a one-step
// create taking the row over.
const blueprintCreateRetryMatch = `FROM blueprints
	WHERE tenant_id = $1 AND repo = $2 AND branch = $3 AND path = $4
	  AND existing_resources = $5 AND status IN ('syncing', 'error', 'paused')
	  AND NOT EXISTS (SELECT 1 FROM blueprint_syncs r
		WHERE r.blueprint_id = blueprints.id AND (r.state = 'success' OR r.plan_path IS NOT NULL))
	ORDER BY created_at DESC, id DESC LIMIT 1`

// BlueprintCreateRetry returns the Blueprint a one-step create of this source
// and mode (adopt or create_new) would retry (blueprintCreateRetryMatch), or
// ErrNotFound when the create is a new Blueprint. The caller compiles with the
// returned row's resource suffix and names its id to AdmitBlueprintCreate,
// which re-checks the choice under the source lock.
func (s *PGStore) BlueprintCreateRetry(ctx context.Context, tenantID, repo, branch, path, mode string) (Blueprint, error) {
	var out Blueprint
	var active string
	err := s.Pool.QueryRow(ctx, `SELECT `+blueprintColumns+` `+blueprintCreateRetryMatch,
		tenantID, repo, branch, path, mode,
	).Scan(scanBlueprint(&out, &active)...)
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	out.ActiveRunID = active
	return out, nil
}

// ListBlueprints returns all non-disconnected blueprints for a tenant, newest first.
func (s *PGStore) ListBlueprints(ctx context.Context, tenantID string) ([]Blueprint, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		        last_sync_at, created_at, updated_at, existing_resources, resource_suffix
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
			&b.AutoSync, &b.Manifest, &b.Status, &b.LastSyncAt, &b.CreatedAt, &b.UpdatedAt, &b.ExistingResources, &b.ResourceSuffix); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateBlueprint applies a partial update (only non-zero pointer fields are
// changed). Returns the updated row. Disconnected rows are untouched —
// ordinary updates cannot reactivate one (w8/m37 t001).
func (s *PGStore) UpdateBlueprint(ctx context.Context, id, tenantID string, name *string, autoSync *bool, path, branch *string, status *string, lastSyncAt *time.Time) (Blueprint, error) {
	var out Blueprint
	err := s.Pool.QueryRow(ctx,
		`UPDATE blueprints SET
			name        = COALESCE($3, name),
			auto_sync   = COALESCE($4, auto_sync),
			path        = COALESCE($5, path),
			status      = COALESCE($6, status),
			last_sync_at = COALESCE($7, last_sync_at),
			branch      = COALESCE($8, branch),
			updated_at  = now()
		 WHERE id = $1 AND tenant_id = $2 AND status != 'disconnected'
		 RETURNING id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		           last_sync_at, created_at, updated_at, existing_resources, resource_suffix`,
		id, tenantID, name, autoSync, path, status, lastSyncAt, branch,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch, &out.Path,
		&out.AutoSync, &out.Manifest, &out.Status, &out.LastSyncAt, &out.CreatedAt, &out.UpdatedAt, &out.ExistingResources, &out.ResourceSuffix)
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
		`INSERT INTO blueprint_syncs (id, blueprint_id, commit_id, state, started_at, completed_at, error_message, execution_generation, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, blueprint_id, commit_id, state, started_at, completed_at, created_at, error_message, execution_generation, note`,
		run.ID, run.BlueprintID, run.CommitID, run.State, run.StartedAt, run.CompletedAt, run.ErrorMessage, run.ExecutionGeneration, run.Note,
	).Scan(&out.ID, &out.BlueprintID, &out.CommitID, &out.State,
		&out.StartedAt, &out.CompletedAt, &out.CreatedAt, &out.ErrorMessage, &out.ExecutionGeneration, &out.Note)
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
			`SELECT id, blueprint_id, commit_id, state, started_at, completed_at, created_at, error_message, note
			 FROM blueprint_syncs WHERE blueprint_id = $1
			 ORDER BY started_at DESC, id DESC LIMIT $2`,
			blueprintID, limit)
	} else {
		// Resume after cursor: find the cursor row's (started_at, id) then page past it.
		rows, err = s.Pool.Query(ctx,
			`SELECT bs.id, bs.blueprint_id, bs.commit_id, bs.state, bs.started_at, bs.completed_at, bs.created_at, bs.error_message, bs.note
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
			&r.StartedAt, &r.CompletedAt, &r.CreatedAt, &r.ErrorMessage, &r.Note); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
