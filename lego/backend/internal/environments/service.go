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
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
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

// DatabaseIndex is the narrow contract environments needs from managed
// Postgres to group Database CRs into an environment (w6/m20 extension) —
// mirroring internal/projects.DatabaseIndex. Unlike services, Databases have
// no control-plane row, so membership is a label (core.LabelEnvironment),
// read via ListPostgres and written via SetEnvironmentID.
// *postgres.Service satisfies this structurally.
type DatabaseIndex interface {
	ListPostgres(ctx context.Context, ownerID string) ([]postgres.PostgresView, error)
	SetEnvironmentID(ctx context.Context, name, environmentID string) error
	// SetProjectID joins a newly-assigned Database to the environment's
	// project (mirroring SetEnvironmentServices' apps.project_id stamp — see
	// SetDatabases).
	SetProjectID(ctx context.Context, name, projectID string) error
}

// KeyValueIndex is DatabaseIndex's KeyValue-CR counterpart. *keyvalue.Service
// satisfies this structurally.
type KeyValueIndex interface {
	ListKeyValues(ctx context.Context, ownerID string) ([]keyvalue.KeyValueView, error)
	SetEnvironmentID(ctx context.Context, name, environmentID string) error
	SetProjectID(ctx context.Context, name, projectID string) error
}

// Service is the environments feature service. Store nil =>
// ErrEnvironmentsUnavailable. Databases/KeyValues nil => the corresponding
// *Ids field resolves empty and SetDatabases/SetKeyValues report
// ErrEnvironmentsUnavailable — Store is the only hard dependency (matching
// internal/projects.Service).
type Service struct {
	*core.Base
	Store     EnvironmentStore
	Databases DatabaseIndex
	KeyValues KeyValueIndex
}

// ErrEnvironmentsUnavailable is returned when the control-plane store is not
// wired (BEX_CP_DB_URI unset). Environments have no CR-only equivalent.
var ErrEnvironmentsUnavailable = errors.New("environments store not configured")

// EnvironmentView is the API shape for an environment — all three surfaces
// return this.
type EnvironmentView struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Name        string    `json:"name"`
	OwnerID     string    `json:"ownerId"`
	CreatedAt   time.Time `json:"createdAt"`
	ServiceIDs  []string  `json:"serviceIds"`
	DatabaseIDs []string  `json:"databaseIds"`
	KeyValueIDs []string  `json:"keyValueIds"`
}

// databaseIDsByEnvironment lists workspaceID's Databases once and groups
// their ids by current environment label — the single fetch List's loop
// shares across every row instead of re-issuing a tenant-wide ListPostgres
// per environment. databaseIDsForEnvironment (below) derives the
// single-environment case from this. Databases unwired (s.Databases nil) =>
// nil map, matching Store-unwired's ErrEnvironmentsUnavailable degrade for
// the feature as a whole rather than failing just this piece.
func (s *Service) databaseIDsByEnvironment(ctx context.Context, workspaceID string) (map[string][]string, error) {
	if s.Databases == nil {
		return nil, nil
	}
	dbs, err := s.Databases.ListPostgres(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	byEnv := map[string][]string{}
	for _, d := range dbs {
		if d.EnvironmentID != "" {
			byEnv[d.EnvironmentID] = append(byEnv[d.EnvironmentID], d.ID)
		}
	}
	return byEnv, nil
}

// keyValueIDsByEnvironment is databaseIDsByEnvironment's KeyValue-CR
// counterpart.
func (s *Service) keyValueIDsByEnvironment(ctx context.Context, workspaceID string) (map[string][]string, error) {
	if s.KeyValues == nil {
		return nil, nil
	}
	kvs, err := s.KeyValues.ListKeyValues(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	byEnv := map[string][]string{}
	for _, kv := range kvs {
		if kv.EnvironmentID != "" {
			byEnv[kv.EnvironmentID] = append(byEnv[kv.EnvironmentID], kv.ID)
		}
	}
	return byEnv, nil
}

// databaseIDsForEnvironment returns the one environment's member Database
// ids — mirroring internal/projects.Service.databaseIDsForProject, built on
// top of databaseIDsByEnvironment so a single-environment call site (Get,
// Rename, SetServices, SetDatabases, SetKeyValues) doesn't need its own
// tenant-wide fetch.
func (s *Service) databaseIDsForEnvironment(ctx context.Context, workspaceID, environmentID string) ([]string, error) {
	byEnv, err := s.databaseIDsByEnvironment(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return byEnv[environmentID], nil
}

// keyValueIDsForEnvironment is databaseIDsForEnvironment's KeyValue-CR
// counterpart.
func (s *Service) keyValueIDsForEnvironment(ctx context.Context, workspaceID, environmentID string) ([]string, error) {
	byEnv, err := s.keyValueIDsByEnvironment(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return byEnv[environmentID], nil
}

// List returns every environment under a project, each with its current
// service/database/key-value membership.
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
	// Fetch each tenant-wide membership scan once and index by environment,
	// rather than re-issuing ListPostgres/ListKeyValues per row below — every
	// environment in this project shares the same underlying Database/KeyValue
	// population.
	dbsByEnv, err := s.databaseIDsByEnvironment(ctx, p.TenantID)
	if err != nil {
		return nil, err
	}
	kvsByEnv, err := s.keyValueIDsByEnvironment(ctx, p.TenantID)
	if err != nil {
		return nil, err
	}
	out := make([]EnvironmentView, 0, len(rows))
	for _, e := range rows {
		sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
		if err != nil {
			return nil, err
		}
		out = append(out, toView(e, sids, dbsByEnv[e.ID], kvsByEnv[e.ID]))
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
	return s.toFullView(ctx, e)
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
	// A brand-new environment id cannot yet be referenced by any
	// service/database/key-value, so skip toFullView's membership fetches
	// (matching projects.Service.Create's own toView(p, nil, nil, nil)).
	return toView(e, nil, nil, nil), nil
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
	return s.toFullView(ctx, e)
}

// Delete removes an environment (its services' environment_id is set to NULL
// by the DB cascade; their project_id is untouched — deleting an environment
// doesn't remove a service from the project it also belongs to). Member
// Databases/KeyValues keep their core.LabelEnvironment label pointing at the
// now-deleted id — the same staleness projects.Service.Delete already
// tolerates for core.LabelProject; a member's next SetEnvironmentID call
// (e.g. joining a different environment) overwrites it.
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
	return s.toFullView(ctx, e)
}

// SetDatabases replaces the full list of managed Postgres databases in an
// environment. databaseIDs are Database CR names (e.g. "mydb"), not store
// rows — unlike SetServices' apps.environment_id column, a Database's
// membership is purely the core.LabelEnvironment label (w6/m20 extension), so
// this diffs the current label state against the wanted set and calls
// SetEnvironmentID per change instead of a single bulk store UPDATE
// (mirroring internal/projects.Service.SetDatabases). A newly-assigned
// Database also joins the environment's project (SetProjectID), mirroring
// SetEnvironmentServices' apps.project_id stamp — Render's model treats "in
// an environment" as "in that project." Removing a Database from the
// environment does NOT clear its project membership, matching
// store.SetEnvironmentServices' own asymmetry for services.
func (s *Service) SetDatabases(ctx context.Context, id string, databaseIDs []string) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	if s.Databases == nil {
		return EnvironmentView{}, ErrEnvironmentsUnavailable
	}
	existing, err := s.Databases.ListPostgres(ctx, e.TenantID)
	if err != nil {
		return EnvironmentView{}, err
	}
	want := make(map[string]bool, len(databaseIDs))
	for _, did := range databaseIDs {
		want[did] = true
	}
	for _, d := range existing {
		switch {
		case d.EnvironmentID == e.ID && !want[d.ID]:
			if err := s.Databases.SetEnvironmentID(ctx, d.ID, ""); err != nil {
				return EnvironmentView{}, err
			}
		case d.EnvironmentID != e.ID && want[d.ID]:
			if err := s.Databases.SetEnvironmentID(ctx, d.ID, e.ID); err != nil {
				return EnvironmentView{}, err
			}
			if err := s.Databases.SetProjectID(ctx, d.ID, e.ProjectID); err != nil {
				return EnvironmentView{}, err
			}
		}
	}
	return s.toFullView(ctx, e)
}

// SetKeyValues is SetDatabases' KeyValue-CR counterpart.
func (s *Service) SetKeyValues(ctx context.Context, id string, keyValueIDs []string) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	if s.KeyValues == nil {
		return EnvironmentView{}, ErrEnvironmentsUnavailable
	}
	existing, err := s.KeyValues.ListKeyValues(ctx, e.TenantID)
	if err != nil {
		return EnvironmentView{}, err
	}
	want := make(map[string]bool, len(keyValueIDs))
	for _, kid := range keyValueIDs {
		want[kid] = true
	}
	for _, kv := range existing {
		switch {
		case kv.EnvironmentID == e.ID && !want[kv.ID]:
			if err := s.KeyValues.SetEnvironmentID(ctx, kv.ID, ""); err != nil {
				return EnvironmentView{}, err
			}
		case kv.EnvironmentID != e.ID && want[kv.ID]:
			if err := s.KeyValues.SetEnvironmentID(ctx, kv.ID, e.ID); err != nil {
				return EnvironmentView{}, err
			}
			if err := s.KeyValues.SetProjectID(ctx, kv.ID, e.ProjectID); err != nil {
				return EnvironmentView{}, err
			}
		}
	}
	return s.toFullView(ctx, e)
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

// toFullView assembles the full member snapshot of an environment — its
// current service, database, and key-value ids — the shape every read and
// write verb returns (mirroring internal/projects' inline sids/dids/kids
// fetch at each of its verbs, factored out here since environments has more
// call sites doing the identical three-fetch).
func (s *Service) toFullView(ctx context.Context, e store.Environment) (EnvironmentView, error) {
	sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
	if err != nil {
		return EnvironmentView{}, err
	}
	dids, err := s.databaseIDsForEnvironment(ctx, e.TenantID, e.ID)
	if err != nil {
		return EnvironmentView{}, err
	}
	kids, err := s.keyValueIDsForEnvironment(ctx, e.TenantID, e.ID)
	if err != nil {
		return EnvironmentView{}, err
	}
	return toView(e, sids, dids, kids), nil
}

func toView(e store.Environment, serviceIDs, databaseIDs, keyValueIDs []string) EnvironmentView {
	if serviceIDs == nil {
		serviceIDs = []string{}
	}
	if databaseIDs == nil {
		databaseIDs = []string{}
	}
	if keyValueIDs == nil {
		keyValueIDs = []string{}
	}
	return EnvironmentView{
		ID:          e.ID,
		ProjectID:   e.ProjectID,
		Name:        e.Name,
		OwnerID:     e.TenantID,
		CreatedAt:   e.CreatedAt,
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
