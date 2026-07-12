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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// TenantMember is a row of `tenant_members` — a subject's membership of a
// workspace and their role. Subject is the OpenFGA user (Kratos identity id or
// Hydra client id) granted the matching relation on workspace:<tenant-id>.
type TenantMember struct {
	TenantID  string    `json:"tenantId"`
	Subject   string    `json:"subject"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreateWorkspace inserts a tenant row and the owner's `admin` membership in one
// transaction — the atomic create the workspaces feature writes through, so a
// workspace never exists without an owner (the OpenFGA grant is a separate,
// caller-driven step, see workspaces.Service.Create). Name collisions and the
// plan CHECK surface as ErrConflict / ErrInvalid via classify.
func (s *PGStore) CreateWorkspace(ctx context.Context, name, plan, ownerSubject string) (Tenant, error) {
	t := Tenant{ID: ids.New(ids.Workspace), Name: name, Plan: plan}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO tenants (id, name, plan) VALUES ($1, $2, $3) RETURNING created_at`,
			t.ID, name, plan,
		).Scan(&t.CreatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, $2, 'admin')`,
			t.ID, ownerSubject)
		return err
	})
	if err != nil {
		return Tenant{}, classify("workspace", err)
	}
	return t, nil
}

// TenantForOwner returns the personal tenant an identity OWNS (by
// owner_identity_id), or ErrNotFound — the cheap read the onboarding path tries
// before the upsert, so a returning caller's login is a single SELECT, not a
// write transaction. Unlike TenantForIdentity (a membership JOIN that can now
// return a workspace the caller was merely invited to), this returns only the
// workspace the caller actually owns (w4/m12).
func (s *PGStore) TenantForOwner(ctx context.Context, identityID string) (Tenant, error) {
	t, err := scanTenant(s.Pool.QueryRow(ctx,
		`SELECT id, name, plan, created_at FROM tenants WHERE owner_identity_id = $1`, identityID))
	if err != nil {
		return Tenant{}, classify("tenant", err)
	}
	return t, nil
}

// GetTenant reads one tenant by id (ErrNotFound when absent).
func (s *PGStore) GetTenant(ctx context.Context, id string) (Tenant, error) {
	var t Tenant
	err := s.Pool.QueryRow(ctx,
		`SELECT id, name, plan, created_at FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Plan, &t.CreatedAt)
	if err != nil {
		return Tenant{}, classify("workspace", err)
	}
	return t, nil
}

// RenameTenant updates a workspace's display name (ErrConflict on collision,
// ErrNotFound when the id doesn't exist).
func (s *PGStore) RenameTenant(ctx context.Context, id, name string) (Tenant, error) {
	var t Tenant
	err := s.Pool.QueryRow(ctx,
		`UPDATE tenants SET name = $2, updated_at = now() WHERE id = $1
		 RETURNING id, name, plan, created_at`,
		id, name,
	).Scan(&t.ID, &t.Name, &t.Plan, &t.CreatedAt)
	if err != nil {
		return Tenant{}, classify("workspace", err)
	}
	return t, nil
}

// DeleteTenant removes a workspace row. The FK cascades (ON DELETE CASCADE) drop
// its apps, their domains, and its tenant_members in the same statement; the
// projector then prunes the orphaned App CRs on its next pass. Returns
// ErrNotFound when the id doesn't exist.
func (s *PGStore) DeleteTenant(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, id)
	if err != nil {
		return classify("workspace", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("workspace %s: %w", id, ErrNotFound)
	}
	return nil
}

// ListTenantsForSubject returns the workspaces the subject belongs to, oldest
// first — the caller's workspace list (the dashboard switcher / owners list).
func (s *PGStore) ListTenantsForSubject(ctx context.Context, subject string) ([]Tenant, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT t.id, t.name, t.plan, t.created_at
		 FROM tenants t JOIN tenant_members m ON m.tenant_id = t.id
		 WHERE m.subject = $1 ORDER BY t.created_at`, subject)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Plan, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CountWorkspacesForSubjectPlan counts how many workspaces of a given plan the
// subject owns — the per-user plan cap check (Render's five-Hobby-workspace
// limit). Membership, not just ownership, is the unit Render caps on.
func (s *PGStore) CountWorkspacesForSubjectPlan(ctx context.Context, subject, plan string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM tenants t JOIN tenant_members m ON m.tenant_id = t.id
		 WHERE m.subject = $1 AND t.plan = $2`, subject, plan,
	).Scan(&n)
	return n, err
}

// CountAppsForTenant counts a workspace's apps (all of them, including suspended
// — the `apps` row exists whether or not spec.suspended) for the per-plan
// service cap.
func (s *PGStore) CountAppsForTenant(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM apps WHERE tenant_id = $1`, tenantID).Scan(&n)
	return n, err
}

// CountTenantMembers counts a workspace's members — the guard w4/m12's
// invite verb consults before adding one (Hobby's single-member cap).
func (s *PGStore) CountTenantMembers(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT count(*) FROM tenant_members WHERE tenant_id = $1`, tenantID).Scan(&n)
	return n, err
}

// ListTenantMembers returns a workspace's members, oldest first.
func (s *PGStore) ListTenantMembers(ctx context.Context, tenantID string) ([]TenantMember, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT tenant_id, subject, role, created_at FROM tenant_members
		 WHERE tenant_id = $1 ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TenantMember
	for rows.Next() {
		var m TenantMember
		if err := rows.Scan(&m.TenantID, &m.Subject, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// OwnerIDForSubject returns the stable opaque "own-" id for a subject, minting
// and persisting one on first sight (w6/m7). The Render owners members surface
// reports userId as this id instead of leaking the raw Kratos/Hydra subject.
// The upsert is race-safe and idempotent: a concurrent first-sight for the same
// subject yields exactly one row (the unique PK is the gate), and DO UPDATE is a
// no-op that only lets RETURNING surface the EXISTING own_id rather than the
// freshly-minted candidate.
func (s *PGStore) OwnerIDForSubject(ctx context.Context, subject string) (string, error) {
	var ownID string
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO owner_ids (subject, own_id) VALUES ($1, $2)
		 ON CONFLICT (subject) DO UPDATE SET subject = EXCLUDED.subject
		 RETURNING own_id`,
		subject, ids.New(ids.Owner),
	).Scan(&ownID)
	if err != nil {
		return "", classify("owner_id", err)
	}
	return ownID, nil
}
