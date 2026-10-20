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
	"strings"
	"time"
	"unicode/utf8"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/jackc/pgx/v5"
)

// BlueprintAutoSyncIntent is a durable accepted automatic Blueprint sync
// (w8/m38): persisted before the git-webhook acknowledges the delivery so a
// process restart cannot lose work the HMAC already authorized.
type BlueprintAutoSyncIntent struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenantId"`
	BlueprintID    string     `json:"blueprintId"`
	DeliveryDigest string     `json:"deliveryDigest"`
	CommitSHA      string     `json:"commitSha"`
	Path           string     `json:"path"`
	State          string     `json:"state"`
	ErrorMessage   *string    `json:"errorMessage,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	ClaimedAt      *time.Time `json:"claimedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	// ClaimGeneration advances on every claim or reclaim. It is execution
	// authority, never a source-order or Git-ancestry comparison.
	ClaimGeneration int64 `json:"-"`
}

// Intent state constants for blueprint_auto_sync_intents.
const (
	BlueprintAutoSyncIntentPending   = "pending"
	BlueprintAutoSyncIntentClaimed   = "claimed"
	BlueprintAutoSyncIntentCompleted = "completed"
	BlueprintAutoSyncIntentFailed    = "failed"
	BlueprintAutoSyncReclaimedReason = "Automatic sync claim was reclaimed by another worker; retry uses the current branch source."
)

const blueprintAutoSyncIntentColumns = `id, tenant_id, blueprint_id, delivery_digest, commit_sha, path, state,
 error_message, created_at, claimed_at, completed_at, claim_generation`

func scanBlueprintAutoSyncIntent(scan interface {
	Scan(dest ...any) error
}) (BlueprintAutoSyncIntent, error) {
	var out BlueprintAutoSyncIntent
	err := scan.Scan(
		&out.ID, &out.TenantID, &out.BlueprintID, &out.DeliveryDigest, &out.CommitSHA, &out.Path, &out.State,
		&out.ErrorMessage, &out.CreatedAt, &out.ClaimedAt, &out.CompletedAt, &out.ClaimGeneration,
	)
	return out, err
}

// ListAutoSyncBlueprints returns every auto-sync-enabled, non-disconnected
// Blueprint on branch. When tenantScope is non-empty, results are confined to
// that workspace (GitHub App installation binding). Repo URL matching is left
// to the caller (repoURLsMatch) because stored repo strings vary by form.
func (s *PGStore) ListAutoSyncBlueprints(ctx context.Context, branch, tenantScope string) ([]Blueprint, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, tenant_id, name, repo, branch, path, auto_sync, manifest, status,
		       last_sync_at, created_at, updated_at
		  FROM blueprints
		 WHERE auto_sync = true
		   AND status != 'disconnected'
		   AND branch = $1
		   AND ($2 = '' OR tenant_id = $2)
		 ORDER BY created_at ASC`,
		branch, tenantScope,
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

// EnqueueBlueprintAutoSyncIntent inserts a pending intent. ON CONFLICT
// (delivery_digest, blueprint_id) DO NOTHING — duplicate delivery is one
// logical intent. Returns whether a new row was inserted.
func (s *PGStore) EnqueueBlueprintAutoSyncIntent(ctx context.Context, intent BlueprintAutoSyncIntent) (bool, error) {
	if intent.ID == "" {
		intent.ID = ids.New(ids.BlueprintAutoSyncIntent)
	}
	if intent.State == "" {
		intent.State = BlueprintAutoSyncIntentPending
	}
	tag, err := s.Pool.Exec(ctx, `
		INSERT INTO blueprint_auto_sync_intents
			(id, tenant_id, blueprint_id, delivery_digest, commit_sha, path, state)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (delivery_digest, blueprint_id) DO NOTHING`,
		intent.ID, intent.TenantID, intent.BlueprintID, intent.DeliveryDigest,
		intent.CommitSHA, intent.Path, intent.State,
	)
	if err != nil {
		return false, classify("blueprint_auto_sync_intent", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ClaimBlueprintAutoSyncIntents leases a bounded batch of pending (or
// stale-claimed) intents for the worker. lease is how long a claimed row stays
// exclusive before another replica may reclaim it after process loss.
func (s *PGStore) ClaimBlueprintAutoSyncIntents(ctx context.Context, limit int, lease time.Duration) ([]BlueprintAutoSyncIntent, error) {
	if limit <= 0 {
		limit = 50
	}
	if lease <= 0 {
		lease = 5 * time.Minute
	}
	staleBefore := time.Now().UTC().Add(-lease)
	var out []BlueprintAutoSyncIntent
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		// Lock the Blueprint, not just one intent: two replicas must not each
		// lease a different delivery for the same Blueprint. A second statement
		// below rechecks fresh claims after the lock under READ COMMITTED.
		rows, err := tx.Query(ctx, `
			SELECT b.id FROM blueprints b
			JOIN (SELECT blueprint_id, min(created_at) AS oldest FROM blueprint_auto_sync_intents
			 WHERE state = 'pending' OR (state = 'claimed' AND (claimed_at IS NULL OR claimed_at < $1))
			 GROUP BY blueprint_id) due ON due.blueprint_id = b.id
			WHERE NOT EXISTS (SELECT 1 FROM blueprint_auto_sync_intents i WHERE i.blueprint_id = b.id
			 AND i.state = 'claimed' AND i.claimed_at >= $1)
			ORDER BY due.oldest, b.id
			LIMIT $2 FOR UPDATE OF b SKIP LOCKED`, staleBefore, limit)
		if err != nil {
			return err
		}
		var blueprintIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			blueprintIDs = append(blueprintIDs, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, blueprintID := range blueprintIDs {
			intent, err := scanBlueprintAutoSyncIntent(tx.QueryRow(ctx, `
				WITH candidate AS (
				 SELECT id FROM blueprint_auto_sync_intents i
				 WHERE blueprint_id = $1
				  AND (state = 'pending' OR (state = 'claimed' AND (claimed_at IS NULL OR claimed_at < $2)))
				  AND NOT EXISTS (SELECT 1 FROM blueprint_auto_sync_intents fresh
				   WHERE fresh.blueprint_id = $1 AND fresh.state = 'claimed' AND fresh.claimed_at >= $2)
				 ORDER BY CASE WHEN state = 'claimed' THEN 0 ELSE 1 END, created_at, id
				 LIMIT 1 FOR UPDATE OF i SKIP LOCKED
				)
				UPDATE blueprint_auto_sync_intents SET state = 'claimed', claimed_at = now(),
				 claim_generation = claim_generation + 1
				WHERE id = (SELECT id FROM candidate) RETURNING `+blueprintAutoSyncIntentColumns, blueprintID, staleBefore))
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if err := retireReclaimedBlueprintRun(ctx, tx, blueprintID); err != nil {
				return err
			}
			out = append(out, intent)
		}
		return nil
	})
	if err != nil {
		return nil, classify("blueprint_auto_sync_intent", err)
	}
	return out, nil
}

// CompleteBlueprintAutoSyncIntent marks a claimed intent successfully processed.
func (s *PGStore) CompleteBlueprintAutoSyncIntent(ctx context.Context, id string, claimGeneration int64, reason string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE blueprint_auto_sync_intents
		   SET state = $2, completed_at = now(), error_message = NULLIF($5, '')
		 WHERE id = $1 AND state = $3 AND claim_generation = $4`,
		id, BlueprintAutoSyncIntentCompleted, BlueprintAutoSyncIntentClaimed, claimGeneration, boundedAutoSyncReason(reason),
	)
	if err != nil {
		return classify("blueprint_auto_sync_intent", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBlueprintSyncBusy
	}
	return nil
}

// FailBlueprintAutoSyncIntent marks a claimed intent terminal-failed with a
// sanitized reason retained for diagnosis (never credentials / bodies).
func (s *PGStore) FailBlueprintAutoSyncIntent(ctx context.Context, id string, claimGeneration int64, errMsg string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE blueprint_auto_sync_intents
		   SET state = $2, completed_at = now(), error_message = $3
		 WHERE id = $1 AND state = $4 AND claim_generation = $5`,
		id, BlueprintAutoSyncIntentFailed, boundedAutoSyncReason(errMsg), BlueprintAutoSyncIntentClaimed, claimGeneration,
	)
	if err != nil {
		return classify("blueprint_auto_sync_intent", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBlueprintSyncBusy
	}
	return nil
}

func boundedAutoSyncReason(reason string) string {
	reason = strings.ToValidUTF8(reason, "")
	if len(reason) > 500 {
		reason = reason[:500]
		for !utf8.ValidString(reason) {
			reason = reason[:len(reason)-1]
		}
	}
	return reason
}

// AssertBlueprintAutoSyncIntent rejects a worker whose lease was reclaimed or
// settled. Expiry by itself does not revoke authority; a new claim does.
func (s *PGStore) AssertBlueprintAutoSyncIntent(ctx context.Context, id string, claimGeneration int64) error {
	var one int
	err := s.Pool.QueryRow(ctx, `SELECT 1 FROM blueprint_auto_sync_intents
	 WHERE id = $1 AND state = 'claimed' AND claim_generation = $2`, id, claimGeneration).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBlueprintSyncBusy
	}
	return classify("blueprint_auto_sync_intent", err)
}

// retireReclaimedBlueprintRun frees a stale automatic execution immediately.
// The caller holds the Blueprint row. Do not wait for its run lock: an older
// completion may hold that lock while waiting on the Blueprint. Bumping the
// execution generation fences it; recovery settles any run we cannot lock.
func retireReclaimedBlueprintRun(ctx context.Context, tx pgx.Tx, blueprintID string) error {
	var runID string
	err := tx.QueryRow(ctx, `SELECT b.active_run_id FROM blueprints b
	 JOIN blueprint_syncs r ON r.id = b.active_run_id
	 LEFT JOIN blueprint_auto_sync_intents i ON i.id = r.auto_intent_id
	 WHERE b.id = $1 AND r.auto_intent_id IS NOT NULL
	 AND (i.id IS NULL OR i.state != 'claimed' OR i.claim_generation != r.auto_claim_generation)`, blueprintID).Scan(&runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE blueprint_syncs SET state = 'error', completed_at = now(), error_message = $2
	 WHERE id IN (SELECT id FROM blueprint_syncs WHERE id = $1 AND state = 'running' FOR UPDATE SKIP LOCKED)`,
		runID, BlueprintAutoSyncReclaimedReason); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE blueprints SET active_run_id = NULL,
	 execution_generation = execution_generation + 1,
	 status = CASE WHEN status = 'syncing' THEN CASE WHEN auto_sync THEN 'error' ELSE 'paused' END ELSE status END,
	 updated_at = now() WHERE id = $1 AND active_run_id = $2`, blueprintID, runID)
	return err
}
