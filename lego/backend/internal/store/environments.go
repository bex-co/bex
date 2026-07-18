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
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Environment is a row of `environments` — a named subset of a Project's
// services (e.g. staging/production). Services opt-in by having
// environment_id set in `apps`, the same shape Project's project_id uses.
//
// ProtectedStatus/NetworkIsolationEnabled/IPAllowList (0023, w6/m19) are
// Render's protected-environment ACLs: ProtectedStatus is "protected" or
// "unprotected" (never empty — the column CHECK + DEFAULT enforce it);
// NetworkIsolationEnabled, when true, is what makes environments/service.go
// stamp core.LabelEnvironment onto member App CRs for the operator's
// environment-scoped NetworkPolicy; IPAllowList is fanned out onto member
// Database/KeyValue CRs' own Spec.IPAllowList (environments/acl.go). Since
// w4/m24 (0034) ip_allow_list is a jsonb array of {cidrBlock, description}
// entries; rows written before the migration hold bare CIDR strings, which
// core.IPAllowListEntry's UnmarshalJSON still decodes (empty description).
type Environment struct {
	ID                      string                  `json:"id"`
	ProjectID               string                  `json:"projectId"`
	TenantID                string                  `json:"tenantId"`
	Name                    string                  `json:"name"`
	CreatedAt               time.Time               `json:"createdAt"`
	ProtectedStatus         string                  `json:"protectedStatus"`
	NetworkIsolationEnabled bool                    `json:"networkIsolationEnabled"`
	IPAllowList             []core.IPAllowListEntry `json:"ipAllowList"`
}

const environmentColumns = `id, project_id, tenant_id, name, created_at, protected_status, network_isolation_enabled, ip_allow_list`

func scanEnvironment(row pgx.Row) (Environment, error) {
	var e Environment
	var allowList []byte
	err := row.Scan(&e.ID, &e.ProjectID, &e.TenantID, &e.Name, &e.CreatedAt,
		&e.ProtectedStatus, &e.NetworkIsolationEnabled, &allowList)
	if err != nil {
		return Environment{}, classify("environment", err)
	}
	if len(allowList) > 0 {
		if err := json.Unmarshal(allowList, &e.IPAllowList); err != nil {
			return Environment{}, fmt.Errorf("environment: decoding ip_allow_list: %w", err)
		}
	}
	return e, nil
}

// CreateEnvironment inserts a new environment seeded with the allow-all
// inbound rule pair (w4/m28): empty means deny-all now, so a fresh
// environment must start explicitly open (Render's dashboard seeds
// 0.0.0.0/0 the same way). CreateWithACL's explicit list — including an
// explicit deny-all [] — overwrites the seed via SetEnvironmentACL.
func (s *PGStore) CreateEnvironment(ctx context.Context, projectID, tenantID, name string) (Environment, error) {
	seed, err := json.Marshal(core.DefaultEnvironmentAllowList())
	if err != nil {
		return Environment{}, err
	}
	return scanEnvironment(s.Pool.QueryRow(ctx,
		`INSERT INTO environments (id, project_id, tenant_id, name, ip_allow_list) VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+environmentColumns,
		ids.New(ids.Environment), projectID, tenantID, name, seed,
	))
}

func (s *PGStore) GetEnvironment(ctx context.Context, id string) (Environment, error) {
	return scanEnvironment(s.Pool.QueryRow(ctx,
		`SELECT `+environmentColumns+` FROM environments WHERE id = $1`, id))
}

func (s *PGStore) ListEnvironments(ctx context.Context, projectID string) ([]Environment, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT `+environmentColumns+` FROM environments WHERE project_id = $1 ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Environment
	for rows.Next() {
		e, err := scanEnvironment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListAllEnvironments returns every environment across every project/tenant —
// unlike ListEnvironments (scoped to one project), this backs the w4/m32/t003
// backfill sweep, which must re-run the member fan-out for every environment
// regardless of owner.
func (s *PGStore) ListAllEnvironments(ctx context.Context) ([]Environment, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+environmentColumns+` FROM environments ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Environment
	for rows.Next() {
		e, err := scanEnvironment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// RepairDriftedEnvironmentIDs is the w4/m32/t003 one-shot repair for rows
// already drifted before SetProjectServices started NULLing environment_id
// on project departure (w4/m32/t001): an app whose environment_id points at
// an environment belonging to a DIFFERENT project than the app's own
// project_id — the exact staleness the pre-fix SetProjectServices could
// leave behind. Idempotent: once repaired, the WHERE clause matches nothing,
// so a second run is a no-op. Returns the repaired app names, the caller's
// cue to also clear their App CR's environment-projected fields.
func (s *PGStore) RepairDriftedEnvironmentIDs(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		UPDATE apps SET environment_id = NULL, updated_at = now()
		WHERE id IN (
			SELECT apps.id FROM apps
			JOIN environments ON apps.environment_id = environments.id
			WHERE apps.project_id IS DISTINCT FROM environments.project_id
		)
		RETURNING name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (s *PGStore) RenameEnvironment(ctx context.Context, id, name string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE environments SET name = $2, updated_at = now() WHERE id = $1`,
		id, name)
	if err != nil {
		return classify("environment", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("environment: %w", ErrNotFound)
	}
	return nil
}

// SetEnvironmentACL replaces the full protected-environment ACL triple —
// full-replace, not a merge, matching every other "Set" verb in this store
// (SetEnvironmentServices, postgres.SetIPAllowList): the caller always
// supplies all three fields, never a partial patch.
func (s *PGStore) SetEnvironmentACL(ctx context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) error {
	if ipAllowList == nil {
		ipAllowList = []core.IPAllowListEntry{}
	}
	allowList, err := json.Marshal(ipAllowList)
	if err != nil {
		return fmt.Errorf("environment: encoding ip_allow_list: %w", err)
	}
	tag, err := s.Pool.Exec(ctx,
		`UPDATE environments SET protected_status = $2, network_isolation_enabled = $3, ip_allow_list = $4, updated_at = now() WHERE id = $1`,
		id, protectedStatus, networkIsolationEnabled, allowList)
	if err != nil {
		return classify("environment", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("environment: %w", ErrNotFound)
	}
	return nil
}

func (s *PGStore) DeleteEnvironment(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM environments WHERE id = $1`, id)
	if err != nil {
		return classify("environment", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("environment: %w", ErrNotFound)
	}
	return nil
}

// SetEnvironmentServices replaces the full list of services in an environment
// (within tenantID): clears any apps currently assigned to it, then assigns
// only the identified apps — mirroring SetProjectServices exactly. Public
// srv- ids are canonical; names remain accepted for backward compatibility
// with clients from before stable service ids shipped. It ALSO stamps
// project_id to projectID on the assigned apps: Render's model treats "in an
// environment" as "in that project," so assigning to an environment is
// sufficient to join its project too (a caller doesn't need two calls).
// Service ids/names not found in tenantID are silently skipped (the UPDATE
// affects 0 rows for them, the same convention SetProjectServices uses).
func (s *PGStore) SetEnvironmentServices(ctx context.Context, environmentID, projectID, tenantID string, serviceIDs []string) error {
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE apps SET environment_id = NULL, updated_at = now()
			 WHERE environment_id = $1 AND tenant_id = $2`,
			environmentID, tenantID); err != nil {
			return err
		}
		if len(serviceIDs) == 0 {
			return nil
		}
		_, err := tx.Exec(ctx,
			`UPDATE apps SET environment_id = $1, project_id = $2, updated_at = now()
			 WHERE (id = ANY($3) OR name = ANY($3)) AND tenant_id = $4`,
			environmentID, projectID, serviceIDs, tenantID)
		return err
	})
}

// ListEnvironmentServices returns the public ids of all services currently in the
// environment. Filters on project_id too (not just environment_id): a
// service's project_id can drift out from under it via the independent
// setProjectServices verb (which knows nothing about environments), so this
// defends against surfacing a service that no longer belongs to the
// environment's own project.
func (s *PGStore) ListEnvironmentServices(ctx context.Context, environmentID, projectID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id FROM apps WHERE environment_id = $1 AND project_id = $2 ORDER BY name`, environmentID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
