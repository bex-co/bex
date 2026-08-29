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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Project is a row of `projects` — a named grouping of services within a
// workspace. Services opt-in to a project by having project_id set in `apps`.
type Project struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *PGStore) CreateProject(ctx context.Context, tenantID, name string) (Project, error) {
	return createProject(ctx, s.Pool, tenantID, name)
}

func (s *PGStore) GetProject(ctx context.Context, id string) (Project, error) {
	var p Project
	err := s.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, name, created_at FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.TenantID, &p.Name, &p.CreatedAt)
	if err != nil {
		return Project{}, classify("project", err)
	}
	return p, nil
}

func (s *PGStore) ListProjects(ctx context.Context, tenantID string) ([]Project, error) {
	return listProjects(ctx, s.Pool, tenantID)
}

func (s *PGStore) RenameProject(ctx context.Context, id, name string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE projects SET name = $2, updated_at = now() WHERE id = $1`,
		id, name)
	if err != nil {
		return classify("project", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project: %w", ErrNotFound)
	}
	return nil
}

func (s *PGStore) DeleteProject(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return classify("project", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project: %w", ErrNotFound)
	}
	return nil
}

// servicePlacement is one apps row's placement snapshot inside the
// SetServices transactions: identity plus nullable project/environment ids.
type servicePlacement struct {
	id            string
	name          string
	projectID     *string
	environmentID *string
}

func queryServicePlacements(ctx context.Context, tx pgx.Tx, query string, args ...any) ([]servicePlacement, error) {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []servicePlacement
	for rows.Next() {
		var p servicePlacement
		if err := rows.Scan(&p.id, &p.name, &p.projectID, &p.environmentID); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// placementChanges diffs the candidate rows' before/after snapshots (matched
// by row id) down to the subset whose placement pair actually changed. A row
// absent from after (deleted mid-transaction) has no truthful after state and
// is dropped rather than guessed.
func placementChanges(before, after []servicePlacement) []core.ServicePlacementChange {
	afterByID := make(map[string]servicePlacement, len(after))
	for _, a := range after {
		afterByID[a.id] = a
	}
	var changes []core.ServicePlacementChange
	for _, b := range before {
		a, ok := afterByID[b.id]
		if !ok || (equalStringPtrs(b.projectID, a.projectID) && equalStringPtrs(b.environmentID, a.environmentID)) {
			continue
		}
		changes = append(changes, core.ServicePlacementChange{
			ServiceID:   b.id,
			ServiceName: b.name,
			ServiceMove: core.ServiceMove{
				ProjectFrom:     b.projectID,
				ProjectTo:       a.projectID,
				EnvironmentFrom: b.environmentID,
				EnvironmentTo:   a.environmentID,
			},
		})
	}
	return changes
}

// placementDiffAround brackets one funnel's membership UPDATEs (mutate) with
// the candidate placement snapshots that turn them into a per-service diff
// (w6/m134). The candidate set — current members of the pivot grouping plus
// every row the caller named — is the correctness-critical predicate, so it
// lives once here for both funnels. pivotColumn is a compile-time literal
// ("project_id" / "environment_id"), never caller input.
func placementDiffAround(ctx context.Context, tx pgx.Tx, pivotColumn, pivotID, tenantID string, serviceIDs []string, mutate func() error) ([]core.ServicePlacementChange, error) {
	before, err := queryServicePlacements(ctx, tx,
		`SELECT id, name, project_id, environment_id FROM apps
		 WHERE tenant_id = $1 AND (`+pivotColumn+` = $2 OR id = ANY($3) OR name = ANY($3))
		 ORDER BY name`,
		tenantID, pivotID, serviceIDs)
	if err != nil {
		return nil, err
	}
	if err := mutate(); err != nil {
		return nil, err
	}
	if len(before) == 0 {
		return nil, nil
	}
	beforeIDs := make([]string, len(before))
	for i, p := range before {
		beforeIDs[i] = p.id
	}
	after, err := queryServicePlacements(ctx, tx,
		`SELECT id, name, project_id, environment_id FROM apps WHERE id = ANY($1)`, beforeIDs)
	if err != nil {
		return nil, err
	}
	return placementChanges(before, after), nil
}

// SetProjectServices replaces the full list of services in a project
// (within tenantID): clears any apps currently assigned to it, then assigns
// only the identified apps. Public srv- ids are canonical; names remain
// accepted for backward compatibility with clients from before stable service
// ids shipped. Also NULLs environment_id on departing rows in the
// same transaction (w4/m32) — a service leaving its project must not keep a
// stale apps.environment_id (and the App CR's frozen spec.environmentIPAllowList
// that implies): ListEnvironmentServices already filters on project_id too,
// so the row silently drops out of every future environment fan-out while
// its k8s-projected rules stay stuck. Returns the per-service placement diff
// (w6/m134): the service layer records move events from it and derives the
// w4/m32 environment-layer clear list (rows whose environment this
// transaction NULLed are exactly the changes with EnvironmentFrom set and
// EnvironmentTo nil, since nothing here ever sets environment_id). Service
// ids/names not found in tenantID are silently skipped (the UPDATE affects 0
// rows for them).
func (s *PGStore) SetProjectServices(ctx context.Context, projectID, tenantID string, serviceIDs []string) ([]core.ServicePlacementChange, error) {
	if serviceIDs == nil {
		serviceIDs = []string{}
	}
	var changes []core.ServicePlacementChange
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var err error
		changes, err = placementDiffAround(ctx, tx, "project_id", projectID, tenantID, serviceIDs, func() error {
			if _, err := tx.Exec(ctx,
				`UPDATE apps SET project_id = NULL, environment_id = NULL, updated_at = now()
				 WHERE project_id = $1 AND tenant_id = $2`,
				projectID, tenantID); err != nil {
				return err
			}
			if len(serviceIDs) == 0 {
				return nil
			}
			_, err := tx.Exec(ctx,
				`UPDATE apps SET project_id = $1, updated_at = now()
				 WHERE (id = ANY($2) OR name = ANY($2)) AND tenant_id = $3`,
				projectID, serviceIDs, tenantID)
			return err
		})
		return err
	})
	return changes, err
}

// ListProjectServices returns the public ids of all services in the project.
func (s *PGStore) ListProjectServices(ctx context.Context, projectID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id FROM apps WHERE project_id = $1 ORDER BY name`, projectID)
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
