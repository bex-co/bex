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
)

// organization_api_keys.go is the org side of an organization's audit-log
// API keys (w1/m657, migration 0349): which Hydra clients are an org's keys,
// with their label, type and creation, and whether each is still live.

// OrganizationAPIKey is one org key's row.
type OrganizationAPIKey struct {
	ClientID       string
	OrganizationID string
	Label          string
	Type           string
	CreatedBy      string
	CreatedAt      time.Time
}

// ErrOrganizationAPIKeyLimit refuses a key past the org's live-key quota.
var ErrOrganizationAPIKeyLimit = errors.New("organization has the maximum number of live API keys")

// InsertOrganizationAPIKey records clientID as orgID's audit-log key, unless
// the org already holds limit live keys (ErrOrganizationAPIKeyLimit). The
// count and the insert run under one org lock, so replicas minting at once
// cannot pass the quota together.
func (s *PGStore) InsertOrganizationAPIKey(ctx context.Context, k OrganizationAPIKey, limit int) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		if err := lockOrganizationAPIKeys(ctx, tx, k.OrganizationID); err != nil {
			return err
		}
		var live int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM organization_api_keys WHERE organization_id = $1 AND revoked_at IS NULL`,
			k.OrganizationID).Scan(&live); err != nil {
			return err
		}
		if live >= limit {
			return ErrOrganizationAPIKeyLimit
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO organization_api_keys (client_id, organization_id, label, created_by)
			VALUES ($1, $2, $3, $4)`,
			k.ClientID, k.OrganizationID, k.Label, k.CreatedBy)
		return err
	})
}

// ListOrganizationAPIKeys is orgID's live keys, oldest first.
func (s *PGStore) ListOrganizationAPIKeys(ctx context.Context, orgID string) ([]OrganizationAPIKey, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT client_id, organization_id, label, key_type, created_by, created_at
		FROM organization_api_keys
		WHERE organization_id = $1 AND revoked_at IS NULL
		ORDER BY created_at, client_id`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (OrganizationAPIKey, error) {
		var k OrganizationAPIKey
		err := row.Scan(&k.ClientID, &k.OrganizationID, &k.Label, &k.Type, &k.CreatedBy, &k.CreatedAt)
		return k, err
	})
}

// RevokeOrganizationAPIKey marks orgID's live key clientID revoked.
// ErrNotFound when orgID has no such live key, another org's key included.
func (s *PGStore) RevokeOrganizationAPIKey(ctx context.Context, orgID, clientID string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE organization_api_keys SET revoked_at = now()
		WHERE client_id = $1 AND organization_id = $2 AND revoked_at IS NULL`, clientID, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeOrganizationAPIKeys marks every live key of orgID revoked and returns
// their client ids: the org's end (ADR115 D11).
func (s *PGStore) RevokeOrganizationAPIKeys(ctx context.Context, orgID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		UPDATE organization_api_keys SET revoked_at = now()
		WHERE organization_id = $1 AND revoked_at IS NULL
		RETURNING client_id`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// LiveOrganizationAPIKey reports whether clientID is a live key of an org
// that still exists, and which: the authority behind an org key's reads,
// read uncached so a revoke or the org's end stops the key at once.
func (s *PGStore) LiveOrganizationAPIKey(ctx context.Context, clientID string) (orgID string, live bool, err error) {
	err = s.Pool.QueryRow(ctx, `
		SELECT k.organization_id
		FROM organization_api_keys k JOIN organizations o ON o.id = k.organization_id
		WHERE k.client_id = $1 AND k.revoked_at IS NULL`, clientID).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return orgID, true, nil
}

// WorkspaceOrganizationID is the org workspaceID belongs to now, "" for a
// standalone workspace or one that does not exist.
func (s *PGStore) WorkspaceOrganizationID(ctx context.Context, workspaceID string) (string, error) {
	var orgID *string
	err := s.Pool.QueryRow(ctx, `SELECT organization_id FROM tenants WHERE id = $1`, workspaceID).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && orgID == nil) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return *orgID, nil
}
