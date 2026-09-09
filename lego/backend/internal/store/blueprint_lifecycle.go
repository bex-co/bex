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
	"fmt"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/jackc/pgx/v5"
)

// blueprint_lifecycle.go (w8/m37) is the persisted execution boundary for
// Blueprint syncs: at most one admitted apply per Blueprint across API
// replicas, fenced by a generation counter so a stale worker cannot complete,
// restamp ownership, or resurrect a disconnected row.
//
// A process-local mutex cannot coordinate production bex-api replicas, and a
// database transaction must never stay open across Git or Kubernetes network
// work — so admission, staging, completion, disconnect, and recovery are each
// short conditional transactions over two columns (migration 0111):
// blueprints.execution_generation (bumped by every admission, disconnect, and
// explicit re-creation) and blueprints.active_run_id (the one admitted but
// uncompleted run). Every lifecycle write re-checks both; zero matched rows
// means another writer won, and the loser takes the busy path instead of
// overwriting.

// ErrBlueprintSyncBusy reports that a Blueprint lifecycle verb lost (or never
// held) execution authority: another apply owns the active claim, or the
// caller's generation was fenced by a newer admission or a disconnect. It
// wraps core.ErrConflict through MapError so every adapter answers 409; the
// apps layer re-codes it as BLUEPRINT_SYNC_BUSY. Callers retry after the
// recorded run settles, or start an explicit new sync.
var ErrBlueprintSyncBusy = fmt.Errorf("blueprint execution busy: %w", ErrConflict)

// BlueprintRunRecoveryBound is the documented staleness bound: a running sync
// with no terminal state older than this is abandoned as interrupted, and a
// disconnect may reap an active claim older than this inline. Healthy applies
// are expected to finish well inside it; a live run past the bound is settled
// as interrupted anyway (its late completion is then fenced, never applied as
// success), which bounds the busy window after process loss.
const BlueprintRunRecoveryBound = 30 * time.Minute

// BlueprintRunInterruptedReason is the stored error_message for an abandoned
// run: an actionable retry instruction, never an automatic replay.
const BlueprintRunInterruptedReason = "Sync was interrupted before it could finish (process loss or deadline). Send a new sync to retry; partial work is never replayed automatically."

// AbandonedBlueprintSync is one stale running run the recovery sweep returns.
type AbandonedBlueprintSync struct {
	RunID         string
	BlueprintID   string
	Generation    int64
	StartedAt     time.Time
	Owned         bool // blueprint still names this run as its active claim
	BlueprintGone bool // no live blueprint row to project onto (defensive)
}

const blueprintColumns = `id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
	last_sync_at, created_at, updated_at, execution_generation, COALESCE(active_run_id, '')`

const blueprintSyncColumns = `id, blueprint_id, commit_id, state, started_at, completed_at, created_at, error_message, execution_generation`

// insertAdmittedRun records the running run of an admission inside the claim
// transaction, so claim and run always commit together.
func insertAdmittedRun(ctx context.Context, tx pgx.Tx, run BlueprintSync) (BlueprintSync, error) {
	var out BlueprintSync
	err := tx.QueryRow(ctx, `
		INSERT INTO blueprint_syncs (id, blueprint_id, commit_id, state, started_at, execution_generation)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+blueprintSyncColumns,
		run.ID, run.BlueprintID, run.CommitID, run.State, run.StartedAt, run.ExecutionGeneration,
	).Scan(&out.ID, &out.BlueprintID, &out.CommitID, &out.State,
		&out.StartedAt, &out.CompletedAt, &out.CreatedAt, &out.ErrorMessage, &out.ExecutionGeneration)
	if err != nil {
		return BlueprintSync{}, err
	}
	return out, nil
}

func scanBlueprint(out *Blueprint, active *string) []any {
	return []any{&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch, &out.Path,
		&out.AutoSync, &out.Manifest, &out.Status, &out.LastSyncAt, &out.CreatedAt, &out.UpdatedAt,
		&out.ExecutionGeneration, active}
}

// AdmitBlueprintSyncRun atomically claims the active apply for an existing
// Blueprint and records its running run: generation bump, active_run_id claim,
// syncing status, and the run insert commit together. The caller must have
// resolved the row through the active lookup first, so zero claimed rows means
// a competing admission won the race → ErrBlueprintSyncBusy, never a silent
// second apply.
func (s *PGStore) AdmitBlueprintSyncRun(ctx context.Context, blueprintID, tenantID string, run BlueprintSync) (Blueprint, BlueprintSync, error) {
	if run.ID == "" {
		run.ID = ids.New(ids.BlueprintSync)
	}
	var b Blueprint
	var out BlueprintSync
	var active string
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// The claim (generation + active_run_id) is taken without touching
		// status: a preflight failure must leave current status and settings
		// exactly as they were (w8/m37 t005). Staging flips to syncing.
		if err := tx.QueryRow(ctx, `
			UPDATE blueprints SET execution_generation = execution_generation + 1,
				active_run_id = $3, updated_at = now()
			WHERE id = $1 AND tenant_id = $2
			  AND status != 'disconnected' AND active_run_id IS NULL
			RETURNING `+blueprintColumns,
			blueprintID, tenantID, run.ID,
		).Scan(scanBlueprint(&b, &active)...); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBlueprintSyncBusy
			}
			return err
		}
		b.ActiveRunID = active
		run.BlueprintID = b.ID
		run.ExecutionGeneration = b.ExecutionGeneration
		var err error
		out, err = insertAdmittedRun(ctx, tx, run)
		return err
	})
	if err != nil {
		return Blueprint{}, BlueprintSync{}, classify("blueprint_sync", err)
	}
	return b, out, nil
}

// AdmitBlueprintCreate atomically upserts the Blueprint row for explicit
// creation and claims its initial sync: a conflicting active claim loses with
// ErrBlueprintSyncBusy, while a disconnected row is deliberately
// re-established under a fresh (bumped) generation so old queued execution
// cannot regain authority (w8/m37 t001). The run insert commits in the same
// transaction — admission failure means zero workload mutations.
func (s *PGStore) AdmitBlueprintCreate(ctx context.Context, b Blueprint, run BlueprintSync) (Blueprint, BlueprintSync, error) {
	if b.ID == "" {
		b.ID = ids.New(ids.Blueprint)
	}
	if b.Path == "" {
		b.Path = "render.yaml"
	}
	if run.ID == "" {
		run.ID = ids.New(ids.BlueprintSync)
	}
	var out Blueprint
	var outRun BlueprintSync
	var active string
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO blueprints (id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
				execution_generation, active_run_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'syncing', 1, $9)
			ON CONFLICT (tenant_id, repo, branch) DO UPDATE SET
				name                 = EXCLUDED.name,
				path                 = EXCLUDED.path,
				auto_sync            = EXCLUDED.auto_sync,
				manifest             = EXCLUDED.manifest,
				status               = 'syncing',
				execution_generation = blueprints.execution_generation + 1,
				active_run_id        = EXCLUDED.active_run_id,
				updated_at           = now()
			WHERE blueprints.active_run_id IS NULL
			RETURNING `+blueprintColumns,
			b.ID, b.TenantID, b.Name, b.Repo, b.Branch, b.Path, b.AutoSync, b.Manifest, run.ID,
		).Scan(scanBlueprint(&out, &active)...); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrBlueprintSyncBusy
			}
			return err
		}
		out.ActiveRunID = active
		run.BlueprintID = out.ID
		run.ExecutionGeneration = out.ExecutionGeneration
		var err error
		outRun, err = insertAdmittedRun(ctx, tx, run)
		return err
	})
	if err != nil {
		return Blueprint{}, BlueprintSync{}, classify("blueprint_sync", err)
	}
	return out, outRun, nil
}

// StageBlueprintManifest stores the admitted sync's preflighted manifest. Only
// the manifest (the sync-owned field) is written — current name/path/autoSync
// settings are retained (w8/m37 t005). Zero rows means the generation was
// fenced by a newer admission or a disconnect → ErrBlueprintSyncBusy.
func (s *PGStore) StageBlueprintManifest(ctx context.Context, id, tenantID string, generation int64, runID, manifest string) (Blueprint, error) {
	var out Blueprint
	var active string
	err := s.Pool.QueryRow(ctx, `
		UPDATE blueprints SET manifest = $4, status = 'syncing', updated_at = now()
		WHERE id = $1 AND tenant_id = $2
		  AND execution_generation = $3 AND active_run_id = $5
		RETURNING `+blueprintColumns,
		id, tenantID, generation, manifest, runID,
	).Scan(scanBlueprint(&out, &active)...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Blueprint{}, fmt.Errorf("blueprint manifest stage: %w", ErrBlueprintSyncBusy)
		}
		return Blueprint{}, classify("blueprint", err)
	}
	out.ActiveRunID = active
	return out, nil
}

// CompleteBlueprintSync commits a run's terminal state together with the
// Blueprint status projection, computed from the CURRENT row (w8/m37 t005):
// success lands in_sync unless auto-sync is currently off (paused); failure
// lands error unless auto-sync is currently off (paused). The run settles only
// from running under the admitted generation, and the Blueprint updates only
// while the admitted generation still owns the claim on a connected row — a
// stale completion cannot overwrite a newer run or a disconnected row, and a
// failed persistence never reports success.
func (s *PGStore) CompleteBlueprintSync(ctx context.Context, id, tenantID, runID string, generation int64, state string, completedAt time.Time, errMsg *string) (Blueprint, error) {
	var out Blueprint
	var active string
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE blueprint_syncs SET state = $2, completed_at = $3, error_message = $4
			WHERE id = $1 AND state = 'running' AND execution_generation = $5`,
			runID, state, completedAt, errMsg, generation)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrBlueprintSyncBusy
		}
		var autoSync bool
		var status string
		if err := tx.QueryRow(ctx, `
			SELECT auto_sync, status FROM blueprints WHERE id = $1 AND tenant_id = $2 FOR UPDATE`,
			id, tenantID).Scan(&autoSync, &status); err != nil {
			return err
		}
		finalStatus := status
		if state == BlueprintSyncStateSuccess {
			finalStatus = BlueprintStatusInSync
			if !autoSync {
				finalStatus = BlueprintStatusPaused
			}
		} else {
			finalStatus = BlueprintStatusError
			if !autoSync {
				finalStatus = BlueprintStatusPaused
			}
		}
		tag, err = tx.Exec(ctx, `
			UPDATE blueprints SET status = $3, last_sync_at = $4, active_run_id = NULL, updated_at = now()
			WHERE id = $1 AND tenant_id = $2
			  AND execution_generation = $5 AND active_run_id = $6 AND status != 'disconnected'`,
			id, tenantID, finalStatus, completedAt, generation, runID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrBlueprintSyncBusy
		}
		if err := tx.QueryRow(ctx,
			`SELECT `+blueprintColumns+` FROM blueprints WHERE id = $1 AND tenant_id = $2`,
			id, tenantID,
		).Scan(scanBlueprint(&out, &active)...); err != nil {
			return err
		}
		out.ActiveRunID = active
		return nil
	})
	if err != nil {
		return Blueprint{}, classify("blueprint_sync", err)
	}
	return out, nil
}

// FailAdmittedSync settles an admitted run in error without projecting
// Blueprint status (w8/m37 t005): a preflight or stage failure leaves current
// name/path/autoSync settings and status untouched — the sync never owned an
// apply — while releasing the active claim. The run settles only from running
// under the admitted generation, and the claim clears only while still owned,
// so a disconnect or newer admission that already took authority is never
// overwritten. Zero settled rows (already settled elsewhere) report
// ErrBlueprintSyncBusy.
func (s *PGStore) FailAdmittedSync(ctx context.Context, id, tenantID, runID string, generation int64, completedAt time.Time, errMsg *string) error {
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE blueprint_syncs SET state = 'error', completed_at = $2, error_message = $3
			WHERE id = $1 AND state = 'running' AND execution_generation = $4`,
			runID, completedAt, errMsg, generation)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrBlueprintSyncBusy
		}
		_, err = tx.Exec(ctx, `
			UPDATE blueprints SET active_run_id = NULL, updated_at = now()
			WHERE id = $1 AND tenant_id = $2
			  AND execution_generation = $3 AND active_run_id = $4`,
			id, tenantID, generation, runID)
		return err
	})
	if err != nil {
		return classify("blueprint_sync", err)
	}
	return nil
}

// DisconnectBlueprint marks the blueprint disconnected (hidden from list and
// ordinary by-ID reads; resources remain untouched). Already-disconnected and
// foreign IDs report ErrNotFound (w8/m37 t001 uniform absence). While a fresh
// apply still owns the active claim the verb refuses with ErrBlueprintSyncBusy
// so the caller can retry after it settles; a stale claim is settled inline as
// interrupted and the disconnect proceeds, which bounds the busy window after
// process loss without a separate recovery tick (w8/m37 t003).
func (s *PGStore) DisconnectBlueprint(ctx context.Context, id, tenantID string) error {
	now := time.Now().UTC()
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var status string
		var generation int64
		var active string
		if err := tx.QueryRow(ctx, `
			SELECT status, execution_generation, COALESCE(active_run_id, '')
			FROM blueprints WHERE id = $1 AND tenant_id = $2 FOR UPDATE`,
			id, tenantID).Scan(&status, &generation, &active); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status == "disconnected" {
			return ErrNotFound
		}
		if active != "" {
			var runState string
			var started time.Time
			err := tx.QueryRow(ctx, `
				SELECT state, started_at FROM blueprint_syncs WHERE id = $1`, active,
			).Scan(&runState, &started)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil && runState == BlueprintSyncStateRunning && now.Sub(started) < BlueprintRunRecoveryBound {
				return ErrBlueprintSyncBusy
			}
			msg := BlueprintRunInterruptedReason
			if _, err := tx.Exec(ctx, `
				UPDATE blueprint_syncs SET state = 'error', completed_at = $2, error_message = $3
				WHERE id = $1 AND state = 'running'`, active, now, msg); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
			UPDATE blueprints SET status = 'disconnected', auto_sync = false,
				execution_generation = execution_generation + 1, active_run_id = NULL, updated_at = now()
			WHERE id = $1 AND tenant_id = $2`, id, tenantID)
		return err
	})
	if err != nil {
		return classify("blueprint", err)
	}
	return nil
}

// ListAbandonedBlueprintSyncs returns stale running runs for the recovery
// sweep, oldest first, bounded per tick (w8/m37 t004). A run qualifies only
// past BlueprintRunRecoveryBound — a demonstrably live run is never listed.
func (s *PGStore) ListAbandonedBlueprintSyncs(ctx context.Context, before time.Time, limit int) ([]AbandonedBlueprintSync, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT bs.id, bs.blueprint_id, bs.execution_generation, bs.started_at,
			COALESCE(b.active_run_id = bs.id, false),
			b.id IS NULL
		FROM blueprint_syncs bs
		LEFT JOIN blueprints b ON b.id = bs.blueprint_id
		WHERE bs.state = 'running' AND bs.started_at < $1
		ORDER BY bs.started_at LIMIT $2`,
		before, limit)
	if err != nil {
		return nil, classify("blueprint_sync", err)
	}
	defer rows.Close()
	var out []AbandonedBlueprintSync
	for rows.Next() {
		var d AbandonedBlueprintSync
		if err := rows.Scan(&d.RunID, &d.BlueprintID, &d.Generation, &d.StartedAt, &d.Owned, &d.BlueprintGone); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// AbandonBlueprintSync settles one stale running run as interrupted (w8/m37
// t004). The run settles only from running; the Blueprint flips to error only
// while the abandoned generation still owns the claim (or for a legacy row
// stuck syncing with no claim). Disconnected rows and newer generations are
// never overwritten. It returns false when another writer settled first — an
// idempotent no-op, not an error.
func (s *PGStore) AbandonBlueprintSync(ctx context.Context, runID string, now time.Time, reason string) (bool, error) {
	settled := false
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var bpID string
		var generation int64
		var started time.Time
		if err := tx.QueryRow(ctx, `
			SELECT blueprint_id, execution_generation, started_at
			FROM blueprint_syncs WHERE id = $1 FOR UPDATE`, runID,
		).Scan(&bpID, &generation, &started); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if !started.Before(now.Add(-BlueprintRunRecoveryBound)) {
			return nil
		}
		tag, err := tx.Exec(ctx, `
			UPDATE blueprint_syncs SET state = 'error', completed_at = $2, error_message = $3
			WHERE id = $1 AND state = 'running'`, runID, now, reason)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		settled = true
		var status string
		var bpGen int64
		var active string
		if err := tx.QueryRow(ctx, `
			SELECT status, execution_generation, COALESCE(active_run_id, '')
			FROM blueprints WHERE id = $1 FOR UPDATE`, bpID,
		).Scan(&status, &bpGen, &active); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if status == "disconnected" {
			return nil
		}
		if (active == runID && bpGen == generation) || (active == "" && status == BlueprintStatusSyncing) {
			_, err = tx.Exec(ctx, `
				UPDATE blueprints SET status = 'error', active_run_id = NULL, updated_at = now()
				WHERE id = $1 AND execution_generation = $2`, bpID, bpGen)
			return err
		}
		return nil
	})
	if err != nil {
		return false, classify("blueprint_sync", err)
	}
	return settled, nil
}
