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
