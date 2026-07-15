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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
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
	// SetEnvironmentACL replaces the protected-environment ACL triple (w6/m19).
	SetEnvironmentACL(ctx context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []string) error
}

// DatabaseIndex is the narrow contract environments needs from managed
// Postgres to group Database CRs into an environment (w6/m20 extension) —
// mirroring internal/projects.DatabaseIndex. Unlike services, Databases have
// no control-plane row, so membership is a label (core.LabelEnvironment),
// read via ListPostgres and written via SetEnvironmentID. SetIPAllowList
// (w6/m19) is the same feature's protected-environment ACL fan-out target —
// environments needs it too since a member Database's ipAllowList follows its
// environment's (SetACL's propagateIPAllowList). *postgres.Service satisfies
// this structurally.
type DatabaseIndex interface {
	ListPostgres(ctx context.Context, ownerID string) ([]postgres.PostgresView, error)
	SetEnvironmentID(ctx context.Context, name, environmentID string) error
	// SetProjectID joins a newly-assigned Database to the environment's
	// project (mirroring SetEnvironmentServices' apps.project_id stamp — see
	// SetDatabases).
	SetProjectID(ctx context.Context, name, projectID string) error
	SetIPAllowList(ctx context.Context, name string, cidrs []string) (postgres.PostgresView, error)
}

// KeyValueIndex is DatabaseIndex's KeyValue-CR counterpart. *keyvalue.Service
// satisfies this structurally.
type KeyValueIndex interface {
	ListKeyValues(ctx context.Context, ownerID string) ([]keyvalue.KeyValueView, error)
	SetEnvironmentID(ctx context.Context, name, environmentID string) error
	SetProjectID(ctx context.Context, name, projectID string) error
	SetIPAllowList(ctx context.Context, name string, cidrs []string) (keyvalue.KeyValueView, error)
}

// EnvGroupIndex is the narrow cross-feature contract environments needs to
// expose Render's Environment.envGroupIds membership. Environment membership
// lives in each env group's KV metadata, so reads list the owning workspace's
// groups and writes update only groups proven to belong to that workspace.
// *envgroups.Service satisfies it structurally.
type EnvGroupIndex interface {
	ListEnvironmentMemberships(ctx context.Context, ownerID string) ([]envgroups.EnvironmentMembership, error)
	SetEnvironmentID(ctx context.Context, id, environmentID string) error
}

// Service is the environments feature service. Store nil =>
// ErrEnvironmentsUnavailable. Databases/KeyValues nil => the corresponding
// *Ids field resolves empty, SetDatabases/SetKeyValues report
// ErrEnvironmentsUnavailable, and SetACL's ipAllowList change is not
// propagated to any Database/KeyValue (matching internal/projects' own
// nil-degrades pattern) — everything else about environments still works.
type Service struct {
	*core.Base
	Store     EnvironmentStore
	Databases DatabaseIndex
	KeyValues KeyValueIndex
	EnvGroups EnvGroupIndex
}

// ErrEnvironmentsUnavailable is returned when the control-plane store is not
// wired (BEX_CP_DB_URI unset). Environments have no CR-only equivalent.
var ErrEnvironmentsUnavailable = errors.New("environments store not configured")

// Render's protectedStatus enum (w6/m19) — aliased from core (the shared leaf
// both this feature and apps.Service's protection guard, apps/protection.go,
// import) rather than defined here, so the two never drift and neither
// feature needs to import the other's package for two string constants.
// Unprotected is the default; on protected, the guard blocks unguarded
// delete/suspend/direct-deploy-override verbs on member Apps unless the
// caller echoes back apps.ProtectedConfirmation.
const (
	ProtectedStatusProtected   = core.ProtectedStatusProtected
	ProtectedStatusUnprotected = core.ProtectedStatusUnprotected
)

// EnvironmentView is the API shape for an environment — all three surfaces
// return this.
type EnvironmentView struct {
	ID                      string    `json:"id"`
	ProjectID               string    `json:"projectId"`
	Name                    string    `json:"name"`
	OwnerID                 string    `json:"ownerId"`
	CreatedAt               time.Time `json:"createdAt"`
	ServiceIDs              []string  `json:"serviceIds"`
	DatabaseIDs             []string  `json:"databaseIds"`
	KeyValueIDs             []string  `json:"keyValueIds"`
	EnvGroupIDs             []string  `json:"envGroupIds"`
	ProtectedStatus         string    `json:"protectedStatus"`
	NetworkIsolationEnabled bool      `json:"networkIsolationEnabled"`
	IPAllowList             []string  `json:"ipAllowList"`
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

// envGroupIDsByEnvironment is the env-group counterpart to the Database and
// KeyValue membership indexes above. A nil EnvGroups seam degrades to empty
// membership on reads, matching the other optional cross-feature indexes.
func (s *Service) envGroupIDsByEnvironment(ctx context.Context, workspaceID string) (map[string][]string, error) {
	if s.EnvGroups == nil {
		return nil, nil
	}
	groups, err := s.EnvGroups.ListEnvironmentMemberships(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	byEnv := map[string][]string{}
	for _, g := range groups {
		if g.EnvironmentID != "" {
			byEnv[g.EnvironmentID] = append(byEnv[g.EnvironmentID], g.ID)
		}
	}
	return byEnv, nil
}

// databaseIDsForEnvironment returns the one environment's member Database
// ids — mirroring internal/projects.Service.databaseIDsForProject, built on
// top of databaseIDsByEnvironment so a single-environment call site (Get,
// Rename, SetServices, SetACL, SetDatabases, SetKeyValues) doesn't need its
// own tenant-wide fetch.
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
	groupsByEnv, err := s.envGroupIDsByEnvironment(ctx, p.TenantID)
	if err != nil {
		return nil, err
	}
	out := make([]EnvironmentView, 0, len(rows))
	for _, e := range rows {
		sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
		if err != nil {
			return nil, err
		}
		out = append(out, toView(e, sids, dbsByEnv[e.ID], kvsByEnv[e.ID], groupsByEnv[e.ID]))
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
	return toView(e, nil, nil, nil, nil), nil
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
// (e.g. joining a different environment) overwrites it. Member Apps'
// core.LabelNetworkIsolation IS cleared first, though — before the row
// disappears — so a delete never leaves an App CR pointing at an environment
// id that no longer exists (Apps have no store-row staleness tolerance the
// way Database/KeyValue's label does, since the operator actively acts on
// this label for NetworkPolicy scoping).
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return err
	}
	names, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
	if err != nil {
		return err
	}
	if err := s.applyAppEnvironmentLabels(ctx, names, "", false); err != nil {
		return err
	}
	if s.EnvGroups != nil {
		workspaceCtx := core.WithWorkspace(ctx, e.TenantID)
		groups, err := s.EnvGroups.ListEnvironmentMemberships(workspaceCtx, e.TenantID)
		if err != nil {
			return err
		}
		for _, g := range groups {
			if g.EnvironmentID == e.ID {
				if err := s.EnvGroups.SetEnvironmentID(workspaceCtx, g.ID, ""); err != nil {
					return err
				}
			}
		}
	}
	return mapStoreErr(s.Store.DeleteEnvironment(ctx, e.ID))
}

// SetServices replaces the full list of services in an environment.
// serviceNames are App CR names (e.g. "whoami"), the same id shown by
// list_services — matching internal/projects.Service.SetServices exactly. The
// assigned services also join the environment's project (see
// store.SetEnvironmentServices). core.LabelNetworkIsolation is synced on
// every affected App CR: cleared on services leaving, set (if the
// environment's networkIsolationEnabled) on services now in it — the
// operator's signal for environment-scoped NetworkPolicy (t004).
func (s *Service) SetServices(ctx context.Context, id string, serviceNames []string) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	before, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
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
	if err := s.applyAppEnvironmentLabels(ctx, namesLeaving(before, sids), "", false); err != nil {
		return EnvironmentView{}, err
	}
	if err := s.applyAppEnvironmentLabels(ctx, sids, e.ID, e.NetworkIsolationEnabled); err != nil {
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
	gidsByEnv, err := s.envGroupIDsByEnvironment(ctx, e.TenantID)
	if err != nil {
		return EnvironmentView{}, err
	}
	return toView(e, sids, dids, kids, gidsByEnv[e.ID]), nil
}

// SetDatabases replaces the full list of managed Postgres databases in an
// environment. databaseIDs are immutable Database CR names (normally dpg-...), not store
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

// SetEnvGroups replaces the full list of environment groups assigned to an
// Environment. Every requested group must belong to the Environment's own
// workspace; an unknown or foreign id is refused instead of silently ignored.
// Groups already assigned to another Environment in the same workspace move in
// one update, while groups omitted from this Environment are unassigned.
func (s *Service) SetEnvGroups(ctx context.Context, id string, envGroupIDs []string) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	if s.EnvGroups == nil {
		return EnvironmentView{}, ErrEnvironmentsUnavailable
	}
	workspaceCtx := core.WithWorkspace(ctx, e.TenantID)
	groups, err := s.EnvGroups.ListEnvironmentMemberships(workspaceCtx, e.TenantID)
	if err != nil {
		return EnvironmentView{}, err
	}
	byID := make(map[string]envgroups.EnvironmentMembership, len(groups))
	for _, g := range groups {
		byID[g.ID] = g
	}
	want := make(map[string]bool, len(envGroupIDs))
	for _, gid := range envGroupIDs {
		if _, ok := byID[gid]; !ok {
			return EnvironmentView{}, fmt.Errorf("%w: environment group %q does not belong to workspace %q", core.ErrForbidden, gid, e.TenantID)
		}
		want[gid] = true
	}
	for _, g := range groups {
		switch {
		case g.EnvironmentID == e.ID && !want[g.ID]:
			if err := s.EnvGroups.SetEnvironmentID(workspaceCtx, g.ID, ""); err != nil {
				return EnvironmentView{}, err
			}
		case g.EnvironmentID != e.ID && want[g.ID]:
			if err := s.EnvGroups.SetEnvironmentID(workspaceCtx, g.ID, e.ID); err != nil {
				return EnvironmentView{}, err
			}
		}
	}
	return s.toFullView(workspaceCtx, e)
}

// SetACL replaces an environment's full protected-environment ACL triple
// (w6/m19) — full-replace, not a merge, matching every other Set verb in this
// codebase (SetServices, postgres/keyvalue's SetIPAllowList): a caller
// changing one field sends the current value of the other two.
//
//   - protectedStatus arms/disarms apps.Service's destructive-verb guard on
//     every member App (checked at guard time via the store, not projected
//     onto any CR — see apps/protection.go).
//   - networkIsolationEnabled is projected onto every member App CR as
//     core.LabelNetworkIsolation (present) or its absence — the operator's
//     signal to scope that App's NetworkPolicy to same-environment peers
//     (t004).
//   - ipAllowList is fanned out to every Database/KeyValue whose
//     core.LabelEnvironment (w6/m20) names this environment
//     (propagateIPAllowList).
func (s *Service) SetACL(ctx context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []string) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	if protectedStatus != ProtectedStatusProtected && protectedStatus != ProtectedStatusUnprotected {
		return EnvironmentView{}, fmt.Errorf("%w: protectedStatus must be %q or %q", core.ErrBadRequest, ProtectedStatusProtected, ProtectedStatusUnprotected)
	}
	if err := core.ValidateCIDRs(ipAllowList); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	if ipAllowList == nil {
		ipAllowList = []string{}
	}
	if err := s.Store.SetEnvironmentACL(ctx, e.ID, protectedStatus, networkIsolationEnabled, ipAllowList); err != nil {
		return EnvironmentView{}, mapStoreErr(err)
	}
	e.ProtectedStatus, e.NetworkIsolationEnabled, e.IPAllowList = protectedStatus, networkIsolationEnabled, ipAllowList
	sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
	if err != nil {
		return EnvironmentView{}, err
	}
	// Always resync, regardless of which direction networkIsolationEnabled
	// moved: applyAppEnvironmentLabels is a no-op patch for an App whose label
	// already matches the target state, so this correctly (and cheaply)
	// handles on->off, off->on, and unchanged alike without needing the
	// pre-update value.
	if err := s.applyAppEnvironmentLabels(ctx, sids, e.ID, e.NetworkIsolationEnabled); err != nil {
		return EnvironmentView{}, err
	}
	if err := s.propagateIPAllowList(ctx, e.TenantID, e.ID, ipAllowList); err != nil {
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
	gidsByEnv, err := s.envGroupIDsByEnvironment(ctx, e.TenantID)
	if err != nil {
		return EnvironmentView{}, err
	}
	return toView(e, sids, dids, kids, gidsByEnv[e.ID]), nil
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
	gidsByEnv, err := s.envGroupIDsByEnvironment(ctx, e.TenantID)
	if err != nil {
		return EnvironmentView{}, err
	}
	return toView(e, sids, dids, kids, gidsByEnv[e.ID]), nil
}

func toView(e store.Environment, serviceIDs, databaseIDs, keyValueIDs, envGroupIDs []string) EnvironmentView {
	if serviceIDs == nil {
		serviceIDs = []string{}
	}
	if databaseIDs == nil {
		databaseIDs = []string{}
	}
	if keyValueIDs == nil {
		keyValueIDs = []string{}
	}
	if envGroupIDs == nil {
		envGroupIDs = []string{}
	}
	ipAllowList := e.IPAllowList
	if ipAllowList == nil {
		ipAllowList = []string{}
	}
	return EnvironmentView{
		ID:                      e.ID,
		ProjectID:               e.ProjectID,
		Name:                    e.Name,
		OwnerID:                 e.TenantID,
		CreatedAt:               e.CreatedAt,
		ServiceIDs:              serviceIDs,
		DatabaseIDs:             databaseIDs,
		KeyValueIDs:             keyValueIDs,
		EnvGroupIDs:             envGroupIDs,
		ProtectedStatus:         e.ProtectedStatus,
		NetworkIsolationEnabled: e.NetworkIsolationEnabled,
		IPAllowList:             ipAllowList,
	}
}

// namesLeaving returns the entries of before that are absent from after —
// the App CR names that just stopped being members of an environment
// (SetServices), so their core.LabelNetworkIsolation can be cleared.
func namesLeaving(before, after []string) []string {
	keep := make(map[string]bool, len(after))
	for _, n := range after {
		keep[n] = true
	}
	var out []string
	for _, n := range before {
		if !keep[n] {
			out = append(out, n)
		}
	}
	return out
}

// applyAppEnvironmentLabels reconciles core.LabelNetworkIsolation on every
// named App CR to (envID, isolated): present (= envID) when isolated, absent
// otherwise. Callers pass ("", false) to clear unconditionally — a service
// leaving its environment (SetServices) or the environment itself being
// deleted (Delete) — or (e.ID, e.NetworkIsolationEnabled) to sync to an
// environment's current state (SetServices' new members, SetACL's toggle).
func (s *Service) applyAppEnvironmentLabels(ctx context.Context, names []string, envID string, isolated bool) error {
	for _, name := range names {
		if err := s.setAppEnvironmentLabel(ctx, name, envID, isolated); err != nil {
			return err
		}
	}
	return nil
}

// setAppEnvironmentLabel fetches the named App via AuthorizeApp — the same
// authorize-and-audit seam apps.Service itself uses, so this fan-out write is
// individually authorized (and its target individually audited) per App, not
// just once at the environment level — then patches core.LabelNetworkIsolation
// to match (isolated, envID) with a merge-patch. A no-op patch (label already
// matches) is skipped. Client nil (no k8s cluster wired — unit tests, or a
// DB-less deploy) is a no-op: the same degrade every other CR-touching
// feature uses, never a panic on a nil client.
func (s *Service) setAppEnvironmentLabel(ctx context.Context, name, envID string, isolated bool) error {
	if s.Client == nil {
		return nil
	}
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		// A member name with no matching App CR (a stale row, or a race with a
		// concurrent delete) is skipped, not fatal — mirroring
		// store.SetEnvironmentServices' own "names not found are silently
		// skipped" convention rather than failing the whole ACL/membership
		// change over one dangling name.
		if errors.Is(err, core.ErrNotFound) {
			return nil
		}
		return err
	}
	want := isolated && envID != ""
	if want && a.Labels[core.LabelNetworkIsolation] == envID {
		return nil
	}
	if !want {
		if _, ok := a.Labels[core.LabelNetworkIsolation]; !ok {
			return nil
		}
	}
	before := a.DeepCopy()
	if want {
		if a.Labels == nil {
			a.Labels = map[string]string{}
		}
		a.Labels[core.LabelNetworkIsolation] = envID
	} else {
		delete(a.Labels, core.LabelNetworkIsolation)
	}
	return s.Client.Patch(ctx, a, client.MergeFrom(before))
}

// propagateIPAllowList fans an environment's ipAllowList out to every
// Database/KeyValue whose core.LabelEnvironment (w6/m20) names this
// environment — the same membership SetDatabases/SetKeyValues manage.
// Databases/KeyValues nil (unwired) => no-op, matching internal/projects' own
// degrade.
func (s *Service) propagateIPAllowList(ctx context.Context, tenantID, environmentID string, cidrs []string) error {
	if s.Databases != nil {
		dbs, err := s.Databases.ListPostgres(ctx, tenantID)
		if err != nil {
			return err
		}
		for _, d := range dbs {
			if d.EnvironmentID != environmentID {
				continue
			}
			if _, err := s.Databases.SetIPAllowList(ctx, d.ID, cidrs); err != nil {
				return err
			}
		}
	}
	if s.KeyValues != nil {
		kvs, err := s.KeyValues.ListKeyValues(ctx, tenantID)
		if err != nil {
			return err
		}
		for _, kv := range kvs {
			if kv.EnvironmentID != environmentID {
				continue
			}
			if _, err := s.KeyValues.SetIPAllowList(ctx, kv.ID, cidrs); err != nil {
				return err
			}
		}
	}
	return nil
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
