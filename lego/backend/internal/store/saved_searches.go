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
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// SavedSearch is a row of `saved_searches` (w3/m505, ADR032): one of a
// project's observability views. TenantID is the project's workspace, read
// with the row so a caller can be authorized against it.
type SavedSearch struct {
	ID        string
	ProjectID string
	TenantID  string
	Name      string
	Type      string
	Filters   json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

// savedSearchOrder lists a project's views as Render's sidebar does: oldest
// first, the defaults (created together) in Render's order.
const savedSearchOrder = `ORDER BY s.created_at, array_position(ARRAY['APPLICATION_LOGS', 'REQUEST_LOGS', 'METRICS'], s.type), s.name, s.id`

const savedSearchColumns = `s.id, s.project_id, p.tenant_id, s.name, s.type, s.filters, s.created_at, s.updated_at`

// ListSavedSearches returns projectID's saved searches, only those of
// searchType when it is not empty.
func (s *PGStore) ListSavedSearches(ctx context.Context, projectID, searchType string) ([]SavedSearch, error) {
	if !textBindable(projectID) || !textBindable(searchType) {
		return []SavedSearch{}, nil
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT `+savedSearchColumns+` FROM saved_searches s JOIN projects p ON p.id = s.project_id
		 WHERE s.project_id = $1 AND ($2 = '' OR s.type = $2) `+savedSearchOrder, projectID, searchType)
	if err != nil {
		return nil, classify("saved search", err)
	}
	defer rows.Close()
	out := []SavedSearch{}
	for rows.Next() {
		v, err := scanSavedSearch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanSavedSearch(row pgx.Row) (SavedSearch, error) {
	var v SavedSearch
	err := row.Scan(&v.ID, &v.ProjectID, &v.TenantID, &v.Name, &v.Type, &v.Filters, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

// GetSavedSearch reads one saved search by id.
func (s *PGStore) GetSavedSearch(ctx context.Context, id string) (SavedSearch, error) {
	if !textBindable(id) {
		return SavedSearch{}, classify("saved search", pgx.ErrNoRows)
	}
	v, err := scanSavedSearch(s.Pool.QueryRow(ctx,
		`SELECT `+savedSearchColumns+` FROM saved_searches s JOIN projects p ON p.id = s.project_id WHERE s.id = $1`, id))
	if err != nil {
		return SavedSearch{}, classify("saved search", err)
	}
	return v, nil
}

// MaxSavedSearchesPerProject bounds one project's views (w3/m507), as the
// filter document's own bounds bound one view: a project editor cannot grow
// its rows without limit. Render's limit, if it has one, was not captured.
const MaxSavedSearchesPerProject = 100

// ErrSavedSearchLimit is CreateSavedSearch's refusal once the project holds
// MaxSavedSearchesPerProject views.
var ErrSavedSearchLimit = errors.New("saved search limit reached")

// savedSearchWriteColumns is what a write returns: the row's own columns.
// The caller names the project's workspace (TenantID), which it authorized.
const savedSearchWriteColumns = `id, project_id, name, type, filters, created_at, updated_at`

func scanWrittenSavedSearch(row pgx.Row, tenantID string) (SavedSearch, error) {
	v := SavedSearch{TenantID: tenantID}
	err := row.Scan(&v.ID, &v.ProjectID, &v.Name, &v.Type, &v.Filters, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

// CreateSavedSearch stores a new view of v.ProjectID (in workspace
// v.TenantID) under a fresh svs- id and returns it. A name the project
// already uses is ErrConflict (Render's duplicate_saved_search_name); a
// project already at MaxSavedSearchesPerProject views is ErrSavedSearchLimit.
// The count and the insert are one statement, but two concurrent creates can
// both see room, so the bound is an abuse bound that may be passed by a few.
func (s *PGStore) CreateSavedSearch(ctx context.Context, v SavedSearch) (SavedSearch, error) {
	if !textBindable(v.ProjectID) || !textBindable(v.Name) {
		return SavedSearch{}, fmt.Errorf("saved search: %w: not valid text", ErrInvalid)
	}
	created, err := scanWrittenSavedSearch(s.Pool.QueryRow(ctx,
		`INSERT INTO saved_searches (id, project_id, name, type, filters)
		 SELECT $1::text, $2::text, $3::text, $4::text, $5::jsonb
		 WHERE (SELECT count(*) FROM saved_searches WHERE project_id = $2::text) < $6::bigint
		 RETURNING `+savedSearchWriteColumns,
		ids.New(ids.SavedSearch), v.ProjectID, v.Name, v.Type, v.Filters, MaxSavedSearchesPerProject), v.TenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return SavedSearch{}, ErrSavedSearchLimit
	}
	if err != nil {
		return SavedSearch{}, classify("saved search", err)
	}
	return created, nil
}

// UpdateSavedSearch sets view v.ID's name and filter document and returns it.
// Its project and type never change. A name another view of the project has
// is ErrConflict.
func (s *PGStore) UpdateSavedSearch(ctx context.Context, v SavedSearch) (SavedSearch, error) {
	if !textBindable(v.ID) {
		return SavedSearch{}, classify("saved search", pgx.ErrNoRows)
	}
	if !textBindable(v.Name) {
		return SavedSearch{}, fmt.Errorf("saved search: %w: not valid text", ErrInvalid)
	}
	updated, err := scanWrittenSavedSearch(s.Pool.QueryRow(ctx,
		`UPDATE saved_searches SET name = $2, filters = $3::jsonb, updated_at = now() WHERE id = $1
		 RETURNING `+savedSearchWriteColumns, v.ID, v.Name, v.Filters), v.TenantID)
	if err != nil {
		return SavedSearch{}, classify("saved search", err)
	}
	return updated, nil
}

// DeleteSavedSearch removes one view, a default one too: a project with no
// views is a project like any other.
func (s *PGStore) DeleteSavedSearch(ctx context.Context, id string) error {
	if !textBindable(id) {
		return classify("saved search", pgx.ErrNoRows)
	}
	tag, err := s.Pool.Exec(ctx, `DELETE FROM saved_searches WHERE id = $1`, id)
	if err != nil {
		return classify("saved search", err)
	}
	if tag.RowsAffected() == 0 {
		return classify("saved search", pgx.ErrNoRows)
	}
	return nil
}

// ProjectService is what a project log view needs to know about one of the
// project's services (w3/m505): its id, shown name, type and environment.
type ProjectService struct {
	ID            string
	Name          string
	Type          string
	EnvironmentID string // public evm- id; empty outside an environment
}

// ListProjectServices returns projectID's services, build source carriers
// excluded (they are never project members, ADR097), ordered by name.
func (s *PGStore) ListProjectServices(ctx context.Context, projectID string) ([]ProjectService, error) {
	if !textBindable(projectID) {
		return []ProjectService{}, nil
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT a.id, `+appDisplayLabel+`, a.type, COALESCE(a.environment_id, '')
		 FROM apps a WHERE a.project_id = $1 AND a.type <> $2 ORDER BY a.name, a.id`, projectID, TypeBuildSource)
	if err != nil {
		return nil, classify("project services", err)
	}
	defer rows.Close()
	out := []ProjectService{}
	for rows.Next() {
		var v ProjectService
		if err := rows.Scan(&v.ID, &v.Name, &v.Type, &v.EnvironmentID); err != nil {
			return nil, err
		}
		if v.EnvironmentID != "" {
			v.EnvironmentID = ids.EnvironmentPublicID(v.EnvironmentID)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
