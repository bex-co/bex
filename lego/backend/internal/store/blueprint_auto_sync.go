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

	ids "github.com/bex-co/bex/lego/backend/internal/id"
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
}

// Intent state constants for blueprint_auto_sync_intents.
const (
	BlueprintAutoSyncIntentPending   = "pending"
	BlueprintAutoSyncIntentClaimed   = "claimed"
	BlueprintAutoSyncIntentCompleted = "completed"
	BlueprintAutoSyncIntentFailed    = "failed"
)

const blueprintAutoSyncIntentColumns = `id, tenant_id, blueprint_id, delivery_digest, commit_sha, path, state,
		        error_message, created_at, claimed_at, completed_at`

func scanBlueprintAutoSyncIntent(scan interface {
	Scan(dest ...any) error
}) (BlueprintAutoSyncIntent, error) {
	var out BlueprintAutoSyncIntent
	err := scan.Scan(
		&out.ID, &out.TenantID, &out.BlueprintID, &out.DeliveryDigest, &out.CommitSHA, &out.Path, &out.State,
		&out.ErrorMessage, &out.CreatedAt, &out.ClaimedAt, &out.CompletedAt,
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
	rows, err := s.Pool.Query(ctx, `
		WITH due AS (
			SELECT id FROM blueprint_auto_sync_intents
			 WHERE state = $1
			    OR (state = $2 AND (claimed_at IS NULL OR claimed_at < $3))
			 ORDER BY created_at ASC
			 LIMIT $4
			 FOR UPDATE SKIP LOCKED
		)
		UPDATE blueprint_auto_sync_intents i
		   SET state = $2, claimed_at = now()
		  FROM due
		 WHERE i.id = due.id
		 RETURNING i.id, i.tenant_id, i.blueprint_id, i.delivery_digest, i.commit_sha, i.path, i.state,
		           i.error_message, i.created_at, i.claimed_at, i.completed_at`,
		BlueprintAutoSyncIntentPending, BlueprintAutoSyncIntentClaimed, staleBefore, limit,
	)
	if err != nil {
		return nil, classify("blueprint_auto_sync_intent", err)
	}
	defer rows.Close()
	var out []BlueprintAutoSyncIntent
	for rows.Next() {
		intent, err := scanBlueprintAutoSyncIntent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, intent)
	}
	return out, rows.Err()
}

// CompleteBlueprintAutoSyncIntent marks a claimed intent successfully processed.
func (s *PGStore) CompleteBlueprintAutoSyncIntent(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE blueprint_auto_sync_intents
		   SET state = $2, completed_at = now(), error_message = NULL
		 WHERE id = $1 AND state = $3`,
		id, BlueprintAutoSyncIntentCompleted, BlueprintAutoSyncIntentClaimed,
	)
	if err != nil {
		return classify("blueprint_auto_sync_intent", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// FailBlueprintAutoSyncIntent marks a claimed intent terminal-failed with a
// sanitized reason retained for diagnosis (never credentials / bodies).
func (s *PGStore) FailBlueprintAutoSyncIntent(ctx context.Context, id, errMsg string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE blueprint_auto_sync_intents
		   SET state = $2, completed_at = now(), error_message = $3
		 WHERE id = $1 AND state = $4`,
		id, BlueprintAutoSyncIntentFailed, errMsg, BlueprintAutoSyncIntentClaimed,
	)
	if err != nil {
		return classify("blueprint_auto_sync_intent", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
