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

// Service is the projects feature service. Store nil => ErrProjectsUnavailable.
type Service struct {
	*core.Base
	Store      ProjectStore
	Selections core.WorkspaceSelectionReader
}

// ErrProjectsUnavailable is returned when the control-plane store is not wired
// (BEX_CP_DB_URI unset). Projects have no CR-only equivalent.
var ErrProjectsUnavailable = errors.New("projects store not configured")

// ProjectView is the API shape for a project — all three surfaces return this.
type ProjectView struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	OwnerID    string    `json:"ownerId"`
	CreatedAt  time.Time `json:"createdAt"`
	ServiceIDs []string  `json:"serviceIds"`
}

// List returns all projects in a workspace, each with its current service list.
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
		out = append(out, toView(p, sids))
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
	return toView(p, sids), nil
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
	return toView(p, nil), nil
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
	return toView(p, sids), nil
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
	return toView(p, sids), nil
}

func toView(p store.Project, serviceIDs []string) ProjectView {
	if serviceIDs == nil {
		serviceIDs = []string{}
	}
	return ProjectView{
		ID:         p.ID,
		Name:       p.Name,
		OwnerID:    p.TenantID,
		CreatedAt:  p.CreatedAt,
		ServiceIDs: serviceIDs,
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
