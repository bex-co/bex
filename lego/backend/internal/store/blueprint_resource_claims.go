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

	"github.com/jackc/pgx/v5"
)

// BlueprintResourceClaim is the durable owner of one workspace resource name
// (w8/m40). Kind is service | database | key_value.
type BlueprintResourceClaim struct {
	TenantID    string
	Kind        string
	Name        string
	BlueprintID string
	ClaimedAt   time.Time
}

// ErrBlueprintResourceConflict wraps ErrConflict when another Blueprint already
// owns the (tenant, kind, name) claim. Callers map it to BLUEPRINT_RESOURCE_CONFLICT.
var ErrBlueprintResourceConflict = fmt.Errorf("blueprint resource claim conflict: %w", ErrConflict)

// ClaimBlueprintResource inserts or keeps ownership of (tenant, kind, name).
// expectedOwner "" adopts only when the row is absent or already ours;
// non-empty authorizes transfer only while that owner still holds the claim
// (intervening takeover cannot use a stale confirmation).
func (s *PGStore) ClaimBlueprintResource(ctx context.Context, tenantID, kind, name, blueprintID, expectedOwner string) error {
	if tenantID == "" || kind == "" || name == "" || blueprintID == "" {
		return fmt.Errorf("claim blueprint resource: missing identity")
	}
	var got string
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO blueprint_resource_claims (tenant_id, kind, name, blueprint_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, kind, name) DO UPDATE
		SET blueprint_id = EXCLUDED.blueprint_id, claimed_at = now()
		WHERE blueprint_resource_claims.blueprint_id = EXCLUDED.blueprint_id
		   OR ($5 <> '' AND blueprint_resource_claims.blueprint_id = $5)
		RETURNING blueprint_id`,
		tenantID, kind, name, blueprintID, expectedOwner,
	).Scan(&got)
	if err == nil {
		if got != blueprintID {
			return ErrBlueprintResourceConflict
		}
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrBlueprintResourceConflict
	}
	return err
}

// ReleaseBlueprintResourceClaims drops every claim held by blueprintID in the
// workspace (disconnect). Resources and CR labels are cleared separately.
func (s *PGStore) ReleaseBlueprintResourceClaims(ctx context.Context, tenantID, blueprintID string) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM blueprint_resource_claims WHERE tenant_id = $1 AND blueprint_id = $2`,
		tenantID, blueprintID)
	return err
}

// ListBlueprintResourceClaims returns claims owned by blueprintID.
func (s *PGStore) ListBlueprintResourceClaims(ctx context.Context, tenantID, blueprintID string) ([]BlueprintResourceClaim, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT tenant_id, kind, name, blueprint_id, claimed_at
		FROM blueprint_resource_claims
		WHERE tenant_id = $1 AND blueprint_id = $2
		ORDER BY kind, name`, tenantID, blueprintID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BlueprintResourceClaim
	for rows.Next() {
		var c BlueprintResourceClaim
		if err := rows.Scan(&c.TenantID, &c.Kind, &c.Name, &c.BlueprintID, &c.ClaimedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetBlueprintResourceOwner returns the owning blueprint id, or "" if unclaimed.
func (s *PGStore) GetBlueprintResourceOwner(ctx context.Context, tenantID, kind, name string) (string, error) {
	var owner string
	err := s.Pool.QueryRow(ctx,
		`SELECT blueprint_id FROM blueprint_resource_claims WHERE tenant_id = $1 AND kind = $2 AND name = $3`,
		tenantID, kind, name,
	).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return owner, err
}
