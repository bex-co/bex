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
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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
	CreateSavedSearch(ctx context.Context, v store.SavedSearch) (store.SavedSearch, error)
	UpdateSavedSearch(ctx context.Context, v store.SavedSearch) (store.SavedSearch, error)
	DeleteSavedSearch(ctx context.Context, id string) error
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
	if searchType != "" {
		if err := checkViewType(searchType); err != nil {
			return nil, err
		}
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
	_, row, err := s.authorizedSavedSearch(ctx, core.RelCanView, id)
	if err != nil {
		return SavedSearchView{}, err
	}
	return savedSearchView(row), nil
}

// authorizedSavedSearch reads the view named by id and authorizes the caller
// on its project's workspace, as authorizedProject does a project: every
// by-id view verb starts with it. A write relation's allowed row is deferred
// to the verb, which records Render's event once its write lands (or none).
func (s *Service) authorizedSavedSearch(ctx context.Context, relation, id string) (savedSearchStore, store.SavedSearch, error) {
	st, err := s.savedSearches()
	if err != nil {
		return nil, store.SavedSearch{}, err
	}
	row, err := st.GetSavedSearch(ctx, id)
	if err != nil {
		return nil, store.SavedSearch{}, store.MapError(err)
	}
	if err := s.AuthorizeOnOwner(core.WithDeferredAllowedWriteAudit(ctx), relation, row.TenantID, "saved search"); err != nil {
		return nil, store.SavedSearch{}, err
	}
	return st, row, nil
}

// checkViewType refuses a type that is not a view type bex serves.
func checkViewType(searchType string) error {
	if savedsearch.ValidType(searchType) {
		return nil
	}
	return fmt.Errorf("%w: unknown saved search type %q (want one of %v)", core.ErrBadRequest, searchType, savedsearch.Types)
}

// savedSearchView is a stored view as the API serves it, its document in
// Render's shape (w3/083): one an older replica wrote in bex's earlier
// spellings while migration 0316 rolled out reads as Render's too. A
// document that no longer parses is served as stored.
func savedSearchView(r store.SavedSearch) SavedSearchView {
	filters := r.Filters
	if len(filters) == 0 {
		filters, _ = json.Marshal(savedsearch.Default())
	} else if doc, err := savedsearch.Parse(filters, r.Type); err == nil {
		filters, _ = json.Marshal(doc)
	}
	return SavedSearchView{ID: r.ID, Name: r.Name, Type: r.Type, Filters: filters}
}

// DuplicateSavedSearchNameCode is Render's error code for a view name the
// project already uses (its errorCode enum, w3/m507).
const DuplicateSavedSearchNameCode = "duplicate_saved_search_name"

// SavedSearchLimitCode refuses a view past store.MaxSavedSearchesPerProject,
// bex's own bound (Render's was not captured).
const SavedSearchLimitCode = "SAVED_SEARCH_LIMIT"

// SavedSearchEdit is what UpdateSavedSearch changes. A nil Name or Filters
// keeps the view's own. Type is taken only to refuse a change: a view keeps
// the type it was created with.
type SavedSearchEdit struct {
	Name    *string
	Filters any
	Type    string
}

// CreateSavedSearch adds a view of searchType named name to the project, with
// filters as its document (nil: Render's empty one), for a caller who may
// edit the project (its rename's relation, can_create). It records Render's
// CreateSavedSearchEvent once the view exists.
func (s *Service) CreateSavedSearch(ctx context.Context, projectID, searchType, name string, filters any) (SavedSearchView, error) {
	p, err := s.authorizedProject(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, projectID)
	if err != nil {
		return SavedSearchView{}, err
	}
	if err := checkViewType(searchType); err != nil {
		return SavedSearchView{}, err
	}
	doc := savedsearch.Default()
	if filters != nil {
		if doc, err = savedsearch.ParseValue(filters, searchType); err != nil {
			return SavedSearchView{}, err
		}
	}
	v, err := s.createSavedSearch(ctx, p.ID, p.TenantID, searchType, name, doc)
	if err != nil {
		return SavedSearchView{}, err
	}
	s.RecordAllowedWrite(ctx, "projects.CreateSavedSearch", core.RelCanCreate, p.TenantID, core.SavedSearchTarget(v.ID), v.Name)
	return v, nil
}

// createSavedSearch stores a view of a validated document under its
// required name, in projectID of workspace tenantID.
func (s *Service) createSavedSearch(ctx context.Context, projectID, tenantID, searchType, name string, doc savedsearch.Document) (SavedSearchView, error) {
	st, err := s.savedSearches()
	if err != nil {
		return SavedSearchView{}, err
	}
	if name, err = requiredName(name); err != nil {
		return SavedSearchView{}, err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return SavedSearchView{}, err
	}
	row, err := st.CreateSavedSearch(ctx, store.SavedSearch{ProjectID: projectID, TenantID: tenantID, Name: name, Type: searchType, Filters: raw})
	if err != nil {
		return SavedSearchView{}, savedSearchWriteError(err, name)
	}
	return savedSearchView(row), nil
}

// UpdateSavedSearch renames a view and replaces its filter document, for a
// caller who may edit its project; the change is what everyone on the project
// sees. Render's audit log has no update event, so an edit records none.
func (s *Service) UpdateSavedSearch(ctx context.Context, id string, edit SavedSearchEdit) (SavedSearchView, error) {
	st, row, err := s.authorizedSavedSearch(ctx, core.RelCanCreate, id)
	if err != nil {
		return SavedSearchView{}, err
	}
	if edit.Type != "" && edit.Type != row.Type {
		return SavedSearchView{}, fmt.Errorf("%w: a view's type cannot change (it is %s); create a %s view instead", core.ErrBadRequest, row.Type, edit.Type)
	}
	name := row.Name
	if edit.Name != nil {
		if name, err = requiredName(*edit.Name); err != nil {
			return SavedSearchView{}, err
		}
	}
	raw := row.Filters
	if edit.Filters != nil {
		doc, err := savedsearch.ParseValue(edit.Filters, row.Type)
		if err != nil {
			return SavedSearchView{}, err
		}
		if raw, err = json.Marshal(doc); err != nil {
			return SavedSearchView{}, err
		}
	}
	updated, err := st.UpdateSavedSearch(ctx, store.SavedSearch{ID: row.ID, TenantID: row.TenantID, Name: name, Filters: raw})
	if err != nil {
		return SavedSearchView{}, savedSearchWriteError(err, name)
	}
	return savedSearchView(updated), nil
}

// DuplicateSavedSearch copies a view, a default one too, under the first
// free name of "<name> (copy)", "<name> (copy 2)", …, for a caller who may
// edit its project, and records CreateSavedSearchEvent for the copy.
func (s *Service) DuplicateSavedSearch(ctx context.Context, id string) (SavedSearchView, error) {
	st, row, err := s.authorizedSavedSearch(ctx, core.RelCanCreate, id)
	if err != nil {
		return SavedSearchView{}, err
	}
	views, err := st.ListSavedSearches(ctx, row.ProjectID, "")
	if err != nil {
		return SavedSearchView{}, store.MapError(err)
	}
	taken := make(map[string]bool, len(views))
	for _, v := range views {
		taken[v.Name] = true
	}
	doc, err := savedsearch.Parse(savedSearchView(row).Filters, row.Type)
	if err != nil {
		return SavedSearchView{}, err
	}
	v, err := s.createSavedSearch(ctx, row.ProjectID, row.TenantID, row.Type, copyName(row.Name, taken), doc)
	if err != nil {
		return SavedSearchView{}, err
	}
	s.RecordAllowedWrite(ctx, "projects.DuplicateSavedSearch", core.RelCanCreate, row.TenantID, core.SavedSearchTarget(v.ID), v.Name)
	return v, nil
}

// DeleteSavedSearch deletes a view, a default one too, for a caller who may
// edit its project, and records Render's DeleteSavedSearchEvent.
func (s *Service) DeleteSavedSearch(ctx context.Context, id string) error {
	st, row, err := s.authorizedSavedSearch(ctx, core.RelCanCreate, id)
	if err != nil {
		return err
	}
	if err := st.DeleteSavedSearch(ctx, row.ID); err != nil {
		return store.MapError(err)
	}
	s.RecordAllowedWrite(ctx, "projects.DeleteSavedSearch", core.RelCanCreate, row.TenantID, core.SavedSearchTarget(row.ID), row.Name)
	return nil
}

// requiredName is a project's or a view's name as stored: trimmed, a label
// (core.CheckLabel), never blank.
func requiredName(name string) (string, error) {
	name, err := core.TrimmedLabel(name)
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", fmt.Errorf("%w: name: must not be blank", core.ErrBadRequest)
	}
	return name, nil
}

// copyName is the first of "<name> (copy)", "<name> (copy 2)", … that taken
// does not hold, the name cut so the whole stays within a label's length. A
// project holds at most store.MaxSavedSearchesPerProject views, so one more
// candidate than that always finds a free name.
func copyName(name string, taken map[string]bool) string {
	var candidate string
	for n := 1; n <= store.MaxSavedSearchesPerProject+1; n++ {
		suffix := " (copy)"
		if n > 1 {
			suffix = fmt.Sprintf(" (copy %d)", n)
		}
		base := []rune(name)
		if room := core.MaxLabelLength - utf8.RuneCountInString(suffix); len(base) > room {
			base = base[:room]
		}
		candidate = strings.TrimSpace(string(base)) + suffix
		if !taken[candidate] {
			break
		}
	}
	return candidate
}

// savedSearchWriteError is a store refusal of a view write as the API answers
// it: Render's duplicate_saved_search_name for a name the project already
// uses, and the per-project bound by name.
func savedSearchWriteError(err error, name string) error {
	switch {
	case errors.Is(err, store.ErrSavedSearchLimit):
		return core.NewConflictError(SavedSearchLimitCode,
			fmt.Sprintf("a project keeps at most %d views; delete one first", store.MaxSavedSearchesPerProject),
			map[string]any{"limit": store.MaxSavedSearchesPerProject})
	case errors.Is(err, store.ErrConflict):
		return core.NewConflictError(DuplicateSavedSearchNameCode,
			fmt.Sprintf("a view named %q already exists in this project", name), map[string]any{"name": name})
	}
	return store.MapError(err)
}

// projectEnvironmentGetter is the optional store capability for resolving a
// standalone environment ID for project/environment observability reads
// (w4/m596); *store.PGStore satisfies it.
type projectEnvironmentGetter interface {
	GetEnvironment(ctx context.Context, id string) (store.Environment, error)
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
	out, err := s.projectResources(ctx, p.TenantID, p.ID)
	if err != nil {
		return "", nil, err
	}
	return p.TenantID, out, nil
}

// ProjectScopeResources resolves the resources of a project, an environment,
// or an environment within a project for Render's project-scoped logs and
// metrics reads (w4/m596). An unknown or cross-workspace project or
// environment returns 404; an environment that exists and is viewable but
// belongs to a different project returns 400.
func (s *Service) ProjectScopeResources(ctx context.Context, projectID, environmentID string) (string, []savedsearch.Resource, error) {
	if projectID == "" && environmentID == "" {
		if err := s.Authorize(ctx, core.RelCanView); err != nil {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("%w: project or environment is required", core.ErrBadRequest)
	}
	if projectID == "" {
		getter, ok := s.Store.(projectEnvironmentGetter)
		if !ok {
			return "", nil, ErrProjectsUnavailable
		}
		env, err := getter.GetEnvironment(ctx, environmentID)
		if err != nil {
			return "", nil, store.MapError(err)
		}
		if err := s.AuthorizeOnOwner(ctx, core.RelCanView, env.TenantID, "environment"); err != nil {
			return "", nil, err
		}
		all, err := s.projectResources(ctx, env.TenantID, env.ProjectID)
		if err != nil {
			return "", nil, err
		}
		return env.TenantID, filterResourcesByEnvironment(all, ids.EnvironmentPublicID(env.ID)), nil
	}

	p, err := s.authorizedProject(ctx, core.RelCanView, projectID)
	if err != nil {
		return "", nil, err
	}
	pubEnv := ""
	if environmentID != "" {
		pubEnv = ids.EnvironmentPublicID(environmentID)
		if getter, ok := s.Store.(projectEnvironmentGetter); ok {
			env, err := getter.GetEnvironment(ctx, environmentID)
			if err != nil {
				return "", nil, store.MapError(err)
			}
			if err := s.AuthorizeOnOwner(ctx, core.RelCanView, env.TenantID, "environment"); err != nil {
				return "", nil, err
			}
			if env.ProjectID != p.ID {
				return "", nil, fmt.Errorf("%w: environment %q does not belong to project %q", core.ErrBadRequest, pubEnv, p.ID)
			}
		} else if lister, ok := s.Store.(projectEnvironmentLister); ok {
			envs, err := lister.ListEnvironmentsForProjects(ctx, []string{p.ID})
			if err != nil {
				return "", nil, store.MapError(err)
			}
			found := false
			for _, e := range envs {
				if ids.EnvironmentPublicID(e.ID) == pubEnv {
					found = true
					break
				}
			}
			if !found {
				return "", nil, fmt.Errorf("%w: environment %q does not belong to project %q", core.ErrBadRequest, pubEnv, p.ID)
			}
		}
	}
	all, err := s.projectResources(ctx, p.TenantID, p.ID)
	if err != nil {
		return "", nil, err
	}
	if pubEnv != "" {
		all = filterResourcesByEnvironment(all, pubEnv)
	}
	return p.TenantID, all, nil
}

func filterResourcesByEnvironment(resources []savedsearch.Resource, envID string) []savedsearch.Resource {
	out := make([]savedsearch.Resource, 0, len(resources))
	for _, r := range resources {
		if r.EnvironmentID == envID {
			out = append(out, r)
		}
	}
	return out
}

func (s *Service) projectResources(ctx context.Context, tenantID, projectID string) ([]savedsearch.Resource, error) {
	st, err := s.savedSearches()
	if err != nil {
		return nil, err
	}
	services, err := st.ListProjectServices(ctx, projectID)
	if err != nil {
		return nil, store.MapError(err)
	}
	out := make([]savedsearch.Resource, 0, len(services))
	for _, svc := range services {
		out = append(out, savedsearch.Resource{ID: svc.ID, Name: svc.Name, Type: svc.Type, EnvironmentID: svc.EnvironmentID})
	}
	if s.Databases != nil {
		dbs, err := s.Databases.ListPostgres(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		for _, db := range dbs {
			if db.ProjectID == projectID {
				out = append(out, savedsearch.Resource{ID: db.ID, Name: db.Name, Type: savedsearch.KindPostgres, EnvironmentID: ids.EnvironmentPublicID(db.EnvironmentID)})
			}
		}
	}
	if s.KeyValues != nil {
		kvs, err := s.KeyValues.ListKeyValues(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		for _, kv := range kvs {
			if kv.ProjectID == projectID {
				out = append(out, savedsearch.Resource{ID: kv.ID, Name: kv.Name, Type: savedsearch.KindKeyValue, EnvironmentID: ids.EnvironmentPublicID(kv.EnvironmentID)})
			}
		}
	}
	return out, nil
}
