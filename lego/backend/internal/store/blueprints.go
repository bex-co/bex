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

// Blueprint is a row of `blueprints` — a workspace-scoped bex.yml stack source.
// Created automatically when deploy is called with a repo+manifest; sync
// re-applies the stored manifest (optionally replacing it first).
type Blueprint struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	Name      string    `json:"name"`
	Repo      string    `json:"repo"`
	Branch    string    `json:"branch"`
	Manifest  string    `json:"manifest"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UpsertBlueprint creates a blueprint or updates its manifest when (tenant_id,
// repo, branch) already exists. The id field is ignored on conflict — the
// existing row's id is preserved. Returns the current row.
func (s *PGStore) UpsertBlueprint(ctx context.Context, b Blueprint) (Blueprint, error) {
	if b.ID == "" {
		b.ID = ids.New(ids.Blueprint)
	}
	var out Blueprint
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO blueprints (id, tenant_id, name, repo, branch, manifest, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (tenant_id, repo, branch)
		DO UPDATE SET
			name     = EXCLUDED.name,
			manifest = EXCLUDED.manifest,
			status   = 'active',
			updated_at = now()
		RETURNING id, tenant_id, name, repo, branch, manifest, status, created_at, updated_at`,
		b.ID, b.TenantID, b.Name, b.Repo, b.Branch, b.Manifest, b.Status,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch,
		&out.Manifest, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	return out, nil
}

// GetBlueprint fetches a blueprint by id, scoped to tenantID (returns ErrNotFound
// if the id belongs to a different workspace).
func (s *PGStore) GetBlueprint(ctx context.Context, id, tenantID string) (Blueprint, error) {
	var out Blueprint
	err := s.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, repo, branch, manifest, status, created_at, updated_at
		 FROM blueprints WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	).Scan(&out.ID, &out.TenantID, &out.Name, &out.Repo, &out.Branch,
		&out.Manifest, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Blueprint{}, classify("blueprint", err)
	}
	return out, nil
}

// ListBlueprints returns all active blueprints for a tenant, newest first.
func (s *PGStore) ListBlueprints(ctx context.Context, tenantID string) ([]Blueprint, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, tenant_id, name, repo, branch, manifest, status, created_at, updated_at
		 FROM blueprints WHERE tenant_id = $1 AND status = 'active'
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
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Name, &b.Repo, &b.Branch,
			&b.Manifest, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
