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

// Package environments implements named environments — a subset of a
// project's services (e.g. staging/production) — layered on top of w1/m31's
// projects feature (internal/projects). An environment is a project-scoped
// label; services opt-in by being assigned to one, which also assigns them to
// the environment's project. Three surfaces: REST, GraphQL, MCP — mirroring
// internal/projects' own shape so the two grouping features read as one
// consistent system rather than two independently-designed ones.
package environments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// EnvironmentStore is the narrow persistence contract this feature needs.
// *store.PGStore satisfies it structurally — GetProject is the one method
// borrowed from the sibling projects feature's own store surface (needed to
// resolve a project id to its tenant for authorization and to create/list
// environments under it), not a new capability.
type EnvironmentStore interface {
	GetProject(ctx context.Context, id string) (store.Project, error)

	CreateEnvironment(ctx context.Context, projectID, tenantID, name string) (store.Environment, error)
	GetEnvironment(ctx context.Context, id string) (store.Environment, error)
	ListEnvironments(ctx context.Context, projectID string) ([]store.Environment, error)
	RenameEnvironment(ctx context.Context, id, name string) error
	DeleteEnvironment(ctx context.Context, id string) error
	SetEnvironmentServices(ctx context.Context, environmentID, projectID, tenantID string, serviceNames []string) error
	ListEnvironmentServices(ctx context.Context, environmentID, projectID string) ([]string, error)
}

// Service is the environments feature service. Store nil =>
// ErrEnvironmentsUnavailable.
type Service struct {
	*core.Base
	Store EnvironmentStore
}

// ErrEnvironmentsUnavailable is returned when the control-plane store is not
// wired (BEX_CP_DB_URI unset). Environments have no CR-only equivalent.
var ErrEnvironmentsUnavailable = errors.New("environments store not configured")

// EnvironmentView is the API shape for an environment — all three surfaces
// return this.
type EnvironmentView struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"projectId"`
	Name       string    `json:"name"`
	OwnerID    string    `json:"ownerId"`
	CreatedAt  time.Time `json:"createdAt"`
	ServiceIDs []string  `json:"serviceIds"`
}

// List returns every environment under a project, each with its current
// service list.
func (s *Service) List(ctx context.Context, projectID string) ([]EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	p, err := s.requireProject(ctx, core.RelCanView, projectID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListEnvironments(ctx, p.ID)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	out := make([]EnvironmentView, 0, len(rows))
	for _, e := range rows {
		sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
		if err != nil {
			return nil, err
		}
		out = append(out, toView(e, sids))
	}
	return out, nil
}

// Get returns a single environment by id.
func (s *Service) Get(ctx context.Context, id string) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanView, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
	if err != nil {
		return EnvironmentView{}, err
	}
	return toView(e, sids), nil
}

// Create creates a new environment under the named project.
func (s *Service) Create(ctx context.Context, projectID, name string) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	p, err := s.requireProject(ctx, core.RelCanCreate, projectID)
	if err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.Store.CreateEnvironment(ctx, p.ID, p.TenantID, name)
	if err != nil {
		return EnvironmentView{}, mapStoreErr(err)
	}
	return toView(e, nil), nil
}

// Rename renames an environment.
func (s *Service) Rename(ctx context.Context, id, name string) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	if err := s.Store.RenameEnvironment(ctx, id, name); err != nil {
		return EnvironmentView{}, mapStoreErr(err)
	}
	e.Name = name
	sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
	if err != nil {
		return EnvironmentView{}, err
	}
	return toView(e, sids), nil
}

// Delete removes an environment (its services' environment_id is set to NULL
// by the DB cascade; their project_id is untouched — deleting an environment
// doesn't remove a service from the project it also belongs to).
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return err
	}
	return mapStoreErr(s.Store.DeleteEnvironment(ctx, e.ID))
}

// SetServices replaces the full list of services in an environment.
// serviceNames are App CR names (e.g. "whoami"), the same id shown by
// list_services — matching internal/projects.Service.SetServices exactly. The
// assigned services also join the environment's project (see
// store.SetEnvironmentServices).
func (s *Service) SetServices(ctx context.Context, id string, serviceNames []string) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	if err := s.Store.SetEnvironmentServices(ctx, e.ID, e.ProjectID, e.TenantID, serviceNames); err != nil {
		return EnvironmentView{}, mapStoreErr(err)
	}
	sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
	if err != nil {
		return EnvironmentView{}, err
	}
	return toView(e, sids), nil
}

// requireProject fetches a project and authorizes it against the workspace it
// belongs to — the id-scoped counterpart to List/Create's direct projectID
// param, needed everywhere an environment verb is reached by a project id
// rather than a workspace id.
func (s *Service) requireProject(ctx context.Context, relation, projectID string) (store.Project, error) {
	if s.Store == nil {
		return store.Project{}, ErrEnvironmentsUnavailable
	}
	p, err := s.Store.GetProject(ctx, projectID)
	if err != nil {
		return store.Project{}, mapStoreErr(err)
	}
	if err := s.AuthorizeOn(ctx, relation, core.WorkspaceObject(p.TenantID)); err != nil {
		return store.Project{}, err
	}
	return p, nil
}

// requireEnvironment fetches an environment and authorizes it against the
// workspace it belongs to (see requireProject).
func (s *Service) requireEnvironment(ctx context.Context, relation, id string) (store.Environment, error) {
	if s.Store == nil {
		return store.Environment{}, ErrEnvironmentsUnavailable
	}
	e, err := s.Store.GetEnvironment(ctx, id)
	if err != nil {
		return store.Environment{}, mapStoreErr(err)
	}
	if err := s.AuthorizeOn(ctx, relation, core.WorkspaceObject(e.TenantID)); err != nil {
		return store.Environment{}, err
	}
	return e, nil
}

func toView(e store.Environment, serviceIDs []string) EnvironmentView {
	if serviceIDs == nil {
		serviceIDs = []string{}
	}
	return EnvironmentView{
		ID:         e.ID,
		ProjectID:  e.ProjectID,
		Name:       e.Name,
		OwnerID:    e.TenantID,
		CreatedAt:  e.CreatedAt,
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
