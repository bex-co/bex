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

// Project is a row of `projects` — a named grouping of services within a
// workspace. Services opt-in to a project by having project_id set in `apps`.
type Project struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenantId"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *PGStore) CreateProject(ctx context.Context, tenantID, name string) (Project, error) {
	p := Project{ID: ids.New(ids.Project), TenantID: tenantID, Name: name}
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO projects (id, tenant_id, name) VALUES ($1, $2, $3) RETURNING created_at`,
		p.ID, tenantID, name,
	).Scan(&p.CreatedAt)
	if err != nil {
		return Project{}, classify("project", err)
	}
	return p, nil
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
	rows, err := s.Pool.Query(ctx,
		`SELECT id, tenant_id, name, created_at FROM projects WHERE tenant_id = $1 ORDER BY created_at`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
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

// SetProjectServices replaces the full list of services in a project
// (within tenantID): clears any apps currently assigned to it, then assigns
// only the identified apps. Public srv- ids are canonical; names remain
// accepted for backward compatibility with clients from before stable service
// ids shipped. Also NULLs environment_id on departing rows in the
// same transaction (w4/m32) — a service leaving its project must not keep a
// stale apps.environment_id (and the App CR's frozen spec.environmentIPAllowList
// that implies): ListEnvironmentServices already filters on project_id too,
// so the row silently drops out of every future environment fan-out while
// its k8s-projected rules stay stuck. Returns the departing names that
// carried a non-null environment_id — the store layer's cue for the service
// layer's k8s-side clear, since a raw SQL UPDATE can't itself patch a CR.
// Service ids/names not found in tenantID are silently skipped (the UPDATE
// affects 0 rows for them).
func (s *PGStore) SetProjectServices(ctx context.Context, projectID, tenantID string, serviceIDs []string) ([]string, error) {
	var departedWithEnv []string
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT name FROM apps WHERE project_id = $1 AND tenant_id = $2 AND environment_id IS NOT NULL`,
			projectID, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			departedWithEnv = append(departedWithEnv, name)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE apps SET project_id = NULL, environment_id = NULL, updated_at = now()
			 WHERE project_id = $1 AND tenant_id = $2`,
			projectID, tenantID); err != nil {
			return err
		}
		if len(serviceIDs) == 0 {
			return nil
		}
		_, err = tx.Exec(ctx,
			`UPDATE apps SET project_id = $1, updated_at = now()
			 WHERE (id = ANY($2) OR name = ANY($2)) AND tenant_id = $3`,
			projectID, serviceIDs, tenantID)
		return err
	})
	return departedWithEnv, err
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
