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

package projects

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// savedSearchStore is the optional store capability behind a project's
// observability views (w3/m505, ADR032); *store.PGStore satisfies it. Kept
// out of ProjectStore, as projectEnvironmentLister is, so a store without
// views still serves every other project verb.
type savedSearchStore interface {
	ListSavedSearches(ctx context.Context, projectID, searchType string) ([]store.SavedSearch, error)
	GetSavedSearch(ctx context.Context, id string) (store.SavedSearch, error)
	ListProjectServices(ctx context.Context, projectID string) ([]store.ProjectService, error)
}

// SavedSearchView is a saved search as Render's GraphQL returns it: id, name,
// type and the filter document.
type SavedSearchView struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Filters json.RawMessage `json:"filters"`
}

func (s *Service) savedSearches() (savedSearchStore, error) {
	st, ok := s.Store.(savedSearchStore)
	if !ok {
		return nil, ErrProjectsUnavailable
	}
	return st, nil
}

// SavedSearches returns the project's views, only those of searchType when it
// is set, to a caller who may view the project (Render's
// savedSearches(projectId, type)).
func (s *Service) SavedSearches(ctx context.Context, projectID, searchType string) ([]SavedSearchView, error) {
	p, err := s.authorizedProject(ctx, core.RelCanView, projectID)
	if err != nil {
		return nil, err
	}
	if searchType != "" && !savedsearch.ValidType(searchType) {
		return nil, fmt.Errorf("%w: unknown saved search type %q (want one of %v)", core.ErrBadRequest, searchType, savedsearch.Types)
	}
	st, err := s.savedSearches()
	if err != nil {
		return nil, err
	}
	rows, err := st.ListSavedSearches(ctx, p.ID, searchType)
	if err != nil {
		return nil, store.MapError(err)
	}
	out := make([]SavedSearchView, len(rows))
	for i, r := range rows {
		out[i] = savedSearchView(r)
	}
	return out, nil
}

// SavedSearch returns one view to a caller who may view its project (Render's
// savedSearch(id)). An unknown id is not found; a view of a project the
// caller may not view is refused as that project's read is.
func (s *Service) SavedSearch(ctx context.Context, id string) (SavedSearchView, error) {
	st, err := s.savedSearches()
	if err != nil {
		return SavedSearchView{}, err
	}
	row, err := st.GetSavedSearch(ctx, id)
	if err != nil {
		return SavedSearchView{}, store.MapError(err)
	}
	if err := s.AuthorizeOnOwner(ctx, core.RelCanView, row.TenantID, "saved search"); err != nil {
		return SavedSearchView{}, err
	}
	return savedSearchView(row), nil
}

func savedSearchView(r store.SavedSearch) SavedSearchView {
	filters := r.Filters
	if len(filters) == 0 {
		filters, _ = json.Marshal(savedsearch.Default())
	}
	return SavedSearchView{ID: r.ID, Name: r.Name, Type: r.Type, Filters: filters}
}

// ProjectLogResources is what a project log view can read (w3/m505, ADR010 §
// Project views): the project's workspace and every service, cron job,
// Postgres and Key Value in it, with names, types and environments, to a
// caller who may view the project. Which of them the caller may read the
// logs of, and which a view's filters keep, is the logs feature's to decide.
func (s *Service) ProjectLogResources(ctx context.Context, projectID string) (string, []savedsearch.Resource, error) {
	p, err := s.authorizedProject(ctx, core.RelCanView, projectID)
	if err != nil {
		return "", nil, err
	}
	st, err := s.savedSearches()
	if err != nil {
		return "", nil, err
	}
	services, err := st.ListProjectServices(ctx, p.ID)
	if err != nil {
		return "", nil, store.MapError(err)
	}
	out := make([]savedsearch.Resource, 0, len(services))
	for _, svc := range services {
		out = append(out, savedsearch.Resource{ID: svc.ID, Name: svc.Name, Type: svc.Type, EnvironmentID: svc.EnvironmentID})
	}
	if s.Databases != nil {
		dbs, err := s.Databases.ListPostgres(ctx, p.TenantID)
		if err != nil {
			return "", nil, err
		}
		for _, db := range dbs {
			if db.ProjectID == p.ID {
				out = append(out, savedsearch.Resource{ID: db.ID, Name: db.Name, Type: savedsearch.KindPostgres, EnvironmentID: ids.EnvironmentPublicID(db.EnvironmentID)})
			}
		}
	}
	if s.KeyValues != nil {
		kvs, err := s.KeyValues.ListKeyValues(ctx, p.TenantID)
		if err != nil {
			return "", nil, err
		}
		for _, kv := range kvs {
			if kv.ProjectID == p.ID {
				out = append(out, savedsearch.Resource{ID: kv.ID, Name: kv.Name, Type: savedsearch.KindKeyValue, EnvironmentID: ids.EnvironmentPublicID(kv.EnvironmentID)})
			}
		}
	}
	return p.TenantID, out, nil
}
