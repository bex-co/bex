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

// Package projects implements named project groupings for services within a
// workspace (w1/m31). A project is a workspace-scoped label; services opt-in by
// being assigned to one. Three surfaces: REST, GraphQL, MCP — all behind the
// same authorization gate as every other feature.
package projects

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ProjectStore is the narrow persistence contract the projects feature needs.
// *store.PGStore satisfies it structurally.
type ProjectStore interface {
	CreateProject(ctx context.Context, tenantID, name string) (store.Project, error)
	GetProject(ctx context.Context, id string) (store.Project, error)
	ListProjects(ctx context.Context, tenantID string) ([]store.Project, error)
	RenameProject(ctx context.Context, id, name string) error
	DeleteProject(ctx context.Context, id string) error
	SetProjectServices(ctx context.Context, projectID, tenantID string, serviceNames []string) error
	ListProjectServices(ctx context.Context, projectID string) ([]string, error)
}

// DatabaseIndex is the narrow contract projects needs from managed Postgres to
// group Database CRs into a project (w1/m31 extension) — unlike services,
// Databases have no control-plane row, so membership is a label
// (core.LabelProject), read via ListPostgres and written via SetProjectID.
// *postgres.Service satisfies this structurally.
type DatabaseIndex interface {
	ListPostgres(ctx context.Context, ownerID string) ([]postgres.PostgresView, error)
	SetProjectID(ctx context.Context, name, projectID string) error
}

// KeyValueIndex is DatabaseIndex's KeyValue-CR counterpart. *keyvalue.Service
// satisfies this structurally.
type KeyValueIndex interface {
	ListKeyValues(ctx context.Context, ownerID string) ([]keyvalue.KeyValueView, error)
	SetProjectID(ctx context.Context, name, projectID string) error
}

// Service is the projects feature service. Store nil => ErrProjectsUnavailable.
// Databases/KeyValues nil => the corresponding *Ids field resolves empty and
// SetDatabases/SetKeyValues report ErrProjectsUnavailable — Store is the only
// hard dependency (matching every other verb here).
type Service struct {
	*core.Base
	Store      ProjectStore
	Databases  DatabaseIndex
	KeyValues  KeyValueIndex
	Selections core.WorkspaceSelectionReader
}

// ErrProjectsUnavailable is returned when the control-plane store is not wired
// (BEX_CP_DB_URI unset). Projects have no CR-only equivalent.
var ErrProjectsUnavailable = errors.New("projects store not configured")

// ProjectView is the API shape for a project — all three surfaces return this.
type ProjectView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	OwnerID     string    `json:"ownerId"`
	CreatedAt   time.Time `json:"createdAt"`
	ServiceIDs  []string  `json:"serviceIds"`
	DatabaseIDs []string  `json:"databaseIds"`
	KeyValueIDs []string  `json:"keyValueIds"`
}

// databaseIDsForProject lists workspaceID's Databases and returns the ids of
// the ones currently labeled with projectID. Databases unwired (s.Databases
// nil) => empty, matching Store-unwired's ErrProjectsUnavailable degrade for
// the feature as a whole rather than failing just this piece.
func (s *Service) databaseIDsForProject(ctx context.Context, workspaceID, projectID string) ([]string, error) {
	if s.Databases == nil {
		return nil, nil
	}
	dbs, err := s.Databases.ListPostgres(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, d := range dbs {
		if d.ProjectID == projectID {
			ids = append(ids, d.ID)
		}
	}
	return ids, nil
}

// keyValueIDsForProject is databaseIDsForProject's KeyValue-CR counterpart.
func (s *Service) keyValueIDsForProject(ctx context.Context, workspaceID, projectID string) ([]string, error) {
	if s.KeyValues == nil {
		return nil, nil
	}
	kvs, err := s.KeyValues.ListKeyValues(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, kv := range kvs {
		if kv.ProjectID == projectID {
			ids = append(ids, kv.ID)
		}
	}
	return ids, nil
}

// List returns all projects in a workspace, each with its current resource lists.
func (s *Service) List(ctx context.Context, workspaceID string) ([]ProjectView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanView, core.WorkspaceObject(workspaceID)); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, ErrProjectsUnavailable
	}
	ps, err := s.Store.ListProjects(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectView, 0, len(ps))
	for _, p := range ps {
		sids, err := s.Store.ListProjectServices(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		dids, err := s.databaseIDsForProject(ctx, workspaceID, p.ID)
		if err != nil {
			return nil, err
		}
		kids, err := s.keyValueIDsForProject(ctx, workspaceID, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, toView(p, sids, dids, kids))
	}
	return out, nil
}

// Get returns a single project by id.
func (s *Service) Get(ctx context.Context, id string) (ProjectView, error) {
	if s.Store == nil {
		return ProjectView{}, ErrProjectsUnavailable
	}
	p, err := s.Store.GetProject(ctx, id)
	if err != nil {
		return ProjectView{}, mapStoreErr(err)
	}
	if err := s.AuthorizeOn(ctx, core.RelCanView, core.WorkspaceObject(p.TenantID)); err != nil {
		return ProjectView{}, err
	}
	sids, err := s.Store.ListProjectServices(ctx, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	dids, err := s.databaseIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	kids, err := s.keyValueIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	return toView(p, sids, dids, kids), nil
}

// Create creates a new project in the workspace named by workspaceID.
func (s *Service) Create(ctx context.Context, workspaceID, name string) (ProjectView, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanCreate, core.WorkspaceObject(workspaceID)); err != nil {
		return ProjectView{}, err
	}
	if s.Store == nil {
		return ProjectView{}, ErrProjectsUnavailable
	}
	p, err := s.Store.CreateProject(ctx, workspaceID, name)
	if err != nil {
		return ProjectView{}, mapStoreErr(err)
	}
	return toView(p, nil, nil, nil), nil
}

// Rename renames a project.
func (s *Service) Rename(ctx context.Context, id, name string) (ProjectView, error) {
	if s.Store == nil {
		return ProjectView{}, ErrProjectsUnavailable
	}
	p, err := s.Store.GetProject(ctx, id)
	if err != nil {
		return ProjectView{}, mapStoreErr(err)
	}
	if err := s.AuthorizeOn(ctx, core.RelCanCreate, core.WorkspaceObject(p.TenantID)); err != nil {
		return ProjectView{}, err
	}
	if err := s.Store.RenameProject(ctx, id, name); err != nil {
		return ProjectView{}, mapStoreErr(err)
	}
	p.Name = name
	sids, err := s.Store.ListProjectServices(ctx, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	dids, err := s.databaseIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	kids, err := s.keyValueIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	return toView(p, sids, dids, kids), nil
}

// Delete removes a project (its services' project_id is set to NULL by the DB cascade).
func (s *Service) Delete(ctx context.Context, id string) error {
	if s.Store == nil {
		return ErrProjectsUnavailable
	}
	p, err := s.Store.GetProject(ctx, id)
	if err != nil {
		return mapStoreErr(err)
	}
	if err := s.AuthorizeOn(ctx, core.RelCanCreate, core.WorkspaceObject(p.TenantID)); err != nil {
		return err
	}
	return mapStoreErr(s.Store.DeleteProject(ctx, id))
}

// SetServices replaces the full list of services in a project.
// serviceNames are App CR names (e.g. "whoami"), not store UUIDs.
func (s *Service) SetServices(ctx context.Context, id string, serviceNames []string) (ProjectView, error) {
	if s.Store == nil {
		return ProjectView{}, ErrProjectsUnavailable
	}
	p, err := s.Store.GetProject(ctx, id)
	if err != nil {
		return ProjectView{}, mapStoreErr(err)
	}
	if err := s.AuthorizeOn(ctx, core.RelCanCreate, core.WorkspaceObject(p.TenantID)); err != nil {
		return ProjectView{}, err
	}
	if err := s.Store.SetProjectServices(ctx, id, p.TenantID, serviceNames); err != nil {
		return ProjectView{}, err
	}
	sids, err := s.Store.ListProjectServices(ctx, id)
	if err != nil {
		return ProjectView{}, err
	}
	dids, err := s.databaseIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	kids, err := s.keyValueIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	return toView(p, sids, dids, kids), nil
}

// SetDatabases replaces the full list of managed Postgres databases in a
// project. databaseIDs are immutable Database CR names (normally dpg-...), not store rows —
// unlike SetServices' apps.project_id column, a Database's membership is
// purely the core.LabelProject label (w1/m31 extension), so this diffs the
// current label state against the wanted set and calls SetProjectID per
// change instead of a single bulk store UPDATE.
func (s *Service) SetDatabases(ctx context.Context, id string, databaseIDs []string) (ProjectView, error) {
	if s.Store == nil {
		return ProjectView{}, ErrProjectsUnavailable
	}
	if s.Databases == nil {
		return ProjectView{}, ErrProjectsUnavailable
	}
	p, err := s.Store.GetProject(ctx, id)
	if err != nil {
		return ProjectView{}, mapStoreErr(err)
	}
	if err := s.AuthorizeOn(ctx, core.RelCanCreate, core.WorkspaceObject(p.TenantID)); err != nil {
		return ProjectView{}, err
	}
	existing, err := s.Databases.ListPostgres(ctx, p.TenantID)
	if err != nil {
		return ProjectView{}, err
	}
	want := make(map[string]bool, len(databaseIDs))
	for _, did := range databaseIDs {
		want[did] = true
	}
	for _, d := range existing {
		switch {
		case d.ProjectID == p.ID && !want[d.ID]:
			if err := s.Databases.SetProjectID(ctx, d.ID, ""); err != nil {
				return ProjectView{}, err
			}
		case d.ProjectID != p.ID && want[d.ID]:
			if err := s.Databases.SetProjectID(ctx, d.ID, p.ID); err != nil {
				return ProjectView{}, err
			}
		}
	}
	sids, err := s.Store.ListProjectServices(ctx, id)
	if err != nil {
		return ProjectView{}, err
	}
	dids, err := s.databaseIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	kids, err := s.keyValueIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	return toView(p, sids, dids, kids), nil
}

// SetKeyValues is SetDatabases' KeyValue-CR counterpart.
func (s *Service) SetKeyValues(ctx context.Context, id string, keyValueIDs []string) (ProjectView, error) {
	if s.Store == nil {
		return ProjectView{}, ErrProjectsUnavailable
	}
	if s.KeyValues == nil {
		return ProjectView{}, ErrProjectsUnavailable
	}
	p, err := s.Store.GetProject(ctx, id)
	if err != nil {
		return ProjectView{}, mapStoreErr(err)
	}
	if err := s.AuthorizeOn(ctx, core.RelCanCreate, core.WorkspaceObject(p.TenantID)); err != nil {
		return ProjectView{}, err
	}
	existing, err := s.KeyValues.ListKeyValues(ctx, p.TenantID)
	if err != nil {
		return ProjectView{}, err
	}
	want := make(map[string]bool, len(keyValueIDs))
	for _, kid := range keyValueIDs {
		want[kid] = true
	}
	for _, kv := range existing {
		switch {
		case kv.ProjectID == p.ID && !want[kv.ID]:
			if err := s.KeyValues.SetProjectID(ctx, kv.ID, ""); err != nil {
				return ProjectView{}, err
			}
		case kv.ProjectID != p.ID && want[kv.ID]:
			if err := s.KeyValues.SetProjectID(ctx, kv.ID, p.ID); err != nil {
				return ProjectView{}, err
			}
		}
	}
	sids, err := s.Store.ListProjectServices(ctx, id)
	if err != nil {
		return ProjectView{}, err
	}
	dids, err := s.databaseIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	kids, err := s.keyValueIDsForProject(ctx, p.TenantID, p.ID)
	if err != nil {
		return ProjectView{}, err
	}
	return toView(p, sids, dids, kids), nil
}

func toView(p store.Project, serviceIDs, databaseIDs, keyValueIDs []string) ProjectView {
	if serviceIDs == nil {
		serviceIDs = []string{}
	}
	if databaseIDs == nil {
		databaseIDs = []string{}
	}
	if keyValueIDs == nil {
		keyValueIDs = []string{}
	}
	return ProjectView{
		ID:          p.ID,
		Name:        p.Name,
		OwnerID:     p.TenantID,
		CreatedAt:   p.CreatedAt,
		ServiceIDs:  serviceIDs,
		DatabaseIDs: databaseIDs,
		KeyValueIDs: keyValueIDs,
	}
}

func mapStoreErr(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("%w: %v", core.ErrNotFound, err)
	case errors.Is(err, store.ErrConflict):
		return fmt.Errorf("%w: %v", core.ErrConflict, err)
	case errors.Is(err, store.ErrInvalid):
		return fmt.Errorf("%w: %v", core.ErrBadRequest, err)
	default:
		return err
	}
}
