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
	// SetProjectServices returns the departing names that carried a non-null
	// environment_id (w4/m32) — Service.SetServices' cue to also clear their
	// App CR's environment-projected fields via EnvironmentIndex.
	SetProjectServices(ctx context.Context, projectID, tenantID string, serviceNames []string) ([]string, error)
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

// EnvironmentIndex is the narrow contract projects needs from the
// environments feature so a service's project departure (SetServices) or a
// project's deletion (Delete) never leaves a member App/Database/KeyValue CR
// frozen with a stale environment inbound-IP layer (w4/m32). The store-row
// side already clears correctly on its own — FK ON DELETE SET NULL/CASCADE
// (SetServices' own environment_id NULL, DeleteProject's cascade) — but
// nothing else re-syncs an already-existing k8s CR after a raw store write,
// which is exactly what these two calls do. Optional: nil (environments
// unwired) => the clear is skipped, matching every other optional
// cross-feature index in this codebase. *environments.Service satisfies this
// structurally.
type EnvironmentIndex interface {
	// ClearServiceEnvironmentLayer clears the environment-projected fields on
	// every named App CR, independent of any specific environment row.
	ClearServiceEnvironmentLayer(ctx context.Context, serviceNames []string) error
	// ClearMembersForProject clears the environment-projected layer from every
	// member of every environment under projectID — called BEFORE the project
	// row is deleted, while the environment rows this needs to enumerate still
	// exist. No separate tenantID: every store.Environment already carries its
	// own TenantID (denormalized from its project at creation, always correct).
	ClearMembersForProject(ctx context.Context, projectID string) error
}

// Service is the projects feature service. Store nil => ErrProjectsUnavailable.
// Databases/KeyValues/Environments nil => the corresponding functionality
// degrades (Databases/KeyValues: the *Ids field resolves empty and
// SetDatabases/SetKeyValues report ErrProjectsUnavailable; Environments: the
// w4/m32 member-clear fan-out is skipped) — Store is the only hard
// dependency (matching every other verb here).
type Service struct {
	*core.Base
	Store        ProjectStore
	Databases    DatabaseIndex
	KeyValues    KeyValueIndex
	Environments EnvironmentIndex
}

// ErrProjectsUnavailable is returned when the control-plane store is not wired
// (BEX_CP_DB_URI unset). Projects have no CR-only equivalent.
var ErrProjectsUnavailable = errors.New("projects store not configured")

// projectEnvironmentLister is the optional store capability for reading a
// project's environments; *store.PGStore satisfies it. Kept out of the narrow
// ProjectStore contract so a store without environments still works.
type projectEnvironmentLister interface {
	ListEnvironments(ctx context.Context, projectID string) ([]store.Environment, error)
}

// environmentIDs returns the environment ids belonging to a project (or nil when
// the store does not implement environment listing). It is a read-enrichment
// helper over an ALREADY-authorized project (List/Get gate the caller), not a
// standalone verb — kept in the service (w1/m53) so the store access lives in the
// domain layer instead of a REST fragment and any surface can reuse it.
func (s *Service) environmentIDs(ctx context.Context, projectID string) ([]string, error) {
	lister, ok := s.Store.(projectEnvironmentLister)
	if !ok {
		return nil, nil
	}
	environments, err := lister.ListEnvironments(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(environments))
	for i := range environments {
		ids[i] = environments[i].ID
	}
	return ids, nil
}

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

// projectResource is the only thing this feature needs to know about a
// label-backed member CR: its id and the project it currently belongs to.
// Reducing PostgresView/KeyValueView to this pair is what lets one grouping and
// one membership-diff serve both kinds.
type projectResource struct {
	id        string
	projectID string
}

// resourceIndex is the shared shape of DatabaseIndex and KeyValueIndex — a CR
// kind whose project membership is the core.LabelProject label rather than a
// control-plane row (w1/m31 extension), so it is listed and re-labeled instead
// of joined. The adapters below embed the feature-typed index, which promotes
// SetProjectID; only the listing needs flattening.
type resourceIndex interface {
	list(ctx context.Context, workspaceID string) ([]projectResource, error)
	SetProjectID(ctx context.Context, name, projectID string) error
}

type databaseResources struct{ DatabaseIndex }

func (d databaseResources) list(ctx context.Context, workspaceID string) ([]projectResource, error) {
	dbs, err := d.ListPostgres(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]projectResource, len(dbs))
	for i, db := range dbs {
		out[i] = projectResource{id: db.ID, projectID: db.ProjectID}
	}
	return out, nil
}

type keyValueResources struct{ KeyValueIndex }

func (k keyValueResources) list(ctx context.Context, workspaceID string) ([]projectResource, error) {
	kvs, err := k.ListKeyValues(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]projectResource, len(kvs))
	for i, kv := range kvs {
		out[i] = projectResource{id: kv.ID, projectID: kv.ProjectID}
	}
	return out, nil
}

// databases/keyValues adapt the optional feature indexes, returning a nil
// resourceIndex when that kind is unwired — read paths then resolve empty
// (rather than failing the whole view), write paths report
// ErrProjectsUnavailable, exactly as before.
func (s *Service) databases() resourceIndex {
	if s.Databases == nil {
		return nil
	}
	return databaseResources{s.Databases}
}

func (s *Service) keyValues() resourceIndex {
	if s.KeyValues == nil {
		return nil
	}
	return keyValueResources{s.KeyValues}
}

// idsByProject lists every resource idx knows about in workspaceID once and
// buckets the ids by the project each is currently labeled with. One listing
// serves EVERY project in the workspace, which is what lets List enrich N
// projects with a single call per resource kind instead of one per project.
// Unlabeled resources (empty projectID) belong to no project and are dropped.
func idsByProject(ctx context.Context, idx resourceIndex, workspaceID string) (map[string][]string, error) {
	if idx == nil {
		return nil, nil
	}
	rs, err := idx.list(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	byProject := map[string][]string{}
	for _, r := range rs {
		if r.projectID != "" {
			byProject[r.projectID] = append(byProject[r.projectID], r.id)
		}
	}
	return byProject, nil
}

// authorizedProject fetches the project named by id and authorizes the caller
// against the PROJECT'S OWN workspace, never the caller's default one — the
// resource-scoped rule core.Base.AuthorizeApp applies to Apps
// (../../docs/ADR012-auth.md). Every id-scoped verb here starts with it, so the
// fetch-then-gate order — and the forbidden-not-found distinction it produces
// for an id in another workspace — cannot drift between verbs.
func (s *Service) authorizedProject(ctx context.Context, relation, id string) (store.Project, error) {
	if s.Store == nil {
		return store.Project{}, ErrProjectsUnavailable
	}
	p, err := s.Store.GetProject(ctx, id)
	if err != nil {
		return store.Project{}, mapStoreErr(err)
	}
	if err := s.AuthorizeOn(ctx, relation, core.WorkspaceObject(p.TenantID)); err != nil {
		return store.Project{}, err
	}
	return p, nil
}

// views composes the API view of ps — projects that all belong to workspaceID —
// resolving each one's service, database, and key-value membership. Callers
// have already authorized; this is read enrichment only.
func (s *Service) views(ctx context.Context, workspaceID string, ps []store.Project) ([]ProjectView, error) {
	if len(ps) == 0 {
		return []ProjectView{}, nil
	}
	dids, err := idsByProject(ctx, s.databases(), workspaceID)
	if err != nil {
		return nil, err
	}
	kids, err := idsByProject(ctx, s.keyValues(), workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectView, 0, len(ps))
	for _, p := range ps {
		sids, err := s.Store.ListProjectServices(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, toView(p, sids, dids[p.ID], kids[p.ID]))
	}
	return out, nil
}

// view is views for the single project p — the shared tail of every verb that
// returns one project, so a mutating verb always reports the same freshly read
// membership a plain Get would.
func (s *Service) view(ctx context.Context, p store.Project) (ProjectView, error) {
	vs, err := s.views(ctx, p.TenantID, []store.Project{p})
	if err != nil {
		return ProjectView{}, err
	}
	return vs[0], nil
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
	return s.views(ctx, workspaceID, ps)
}

// Get returns a single project by id.
func (s *Service) Get(ctx context.Context, id string) (ProjectView, error) {
	p, err := s.authorizedProject(ctx, core.RelCanView, id)
	if err != nil {
		return ProjectView{}, err
	}
	return s.view(ctx, p)
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
	p, err := s.authorizedProject(ctx, core.RelCanCreate, id)
	if err != nil {
		return ProjectView{}, err
	}
	if err := s.Store.RenameProject(ctx, id, name); err != nil {
		return ProjectView{}, mapStoreErr(err)
	}
	p.Name = name
	return s.view(ctx, p)
}

// Delete removes a project (its services' project_id is set to NULL by the DB
// cascade, and — since the same FK behavior nulls apps.environment_id for
// members of every child environment, which the cascade also deletes — the
// store rows end up consistent on their own). What the DB cascade can't do is
// touch the already-existing k8s CRs: before the row disappears, fan the
// environment-projected layer clear out to every member of every child
// environment (w4/m32), so a deleted project's protected/isolated
// environments don't leave their Apps/Databases/KeyValues silently blocked.
func (s *Service) Delete(ctx context.Context, id string) error {
	p, err := s.authorizedProject(ctx, core.RelCanCreate, id)
	if err != nil {
		return err
	}
	if s.Environments != nil {
		if err := s.Environments.ClearMembersForProject(ctx, p.ID); err != nil {
			return err
		}
	}
	return mapStoreErr(s.Store.DeleteProject(ctx, id))
}

// SetServices replaces the full list of services in a project.
// serviceIDs are the public ids returned by list_services (normally srv-...).
// Legacy public names remain accepted by the store during the stable-id
// transition. A service leaving the project that also carried a stale
// environment membership has its App CR's environment-projected layer cleared
// too (w4/m32) — the store
// already NULLs apps.environment_id for it, but only a k8s Patch can clear
// the CR spec fields that projection stamped.
func (s *Service) SetServices(ctx context.Context, id string, serviceIDs []string) (ProjectView, error) {
	p, err := s.authorizedProject(ctx, core.RelCanCreate, id)
	if err != nil {
		return ProjectView{}, err
	}
	departedWithEnv, err := s.Store.SetProjectServices(ctx, id, p.TenantID, serviceIDs)
	if err != nil {
		return ProjectView{}, err
	}
	if len(departedWithEnv) > 0 && s.Environments != nil {
		if err := s.Environments.ClearServiceEnvironmentLayer(ctx, departedWithEnv); err != nil {
			return ProjectView{}, err
		}
	}
	return s.view(ctx, p)
}

// SetDatabases replaces the full list of managed Postgres databases in a
// project. databaseIDs are immutable Database CR names (normally dpg-...), not store rows —
// unlike SetServices' apps.project_id column, a Database's membership is
// purely the core.LabelProject label (w1/m31 extension), so this diffs the
// current label state against the wanted set and calls SetProjectID per
// change instead of a single bulk store UPDATE.
func (s *Service) SetDatabases(ctx context.Context, id string, databaseIDs []string) (ProjectView, error) {
	return s.setResourceMembers(ctx, s.databases(), id, databaseIDs)
}

// SetKeyValues is SetDatabases' KeyValue-CR counterpart.
func (s *Service) SetKeyValues(ctx context.Context, id string, keyValueIDs []string) (ProjectView, error) {
	return s.setResourceMembers(ctx, s.keyValues(), id, keyValueIDs)
}

// setResourceMembers replaces the full membership of one label-backed resource
// kind in a project: it diffs the wanted set against the workspace's current
// labels and re-labels only what actually changed, so an unrelated resource is
// never rewritten. Only relabels within the project's OWN workspace are
// considered — the listing is scoped to p.TenantID, so an id naming a resource
// in another workspace is simply absent from the diff and silently ignored,
// never adopted across the tenant boundary.
func (s *Service) setResourceMembers(ctx context.Context, idx resourceIndex, id string, wantIDs []string) (ProjectView, error) {
	if s.Store == nil || idx == nil {
		return ProjectView{}, ErrProjectsUnavailable
	}
	p, err := s.authorizedProject(ctx, core.RelCanCreate, id)
	if err != nil {
		return ProjectView{}, err
	}
	existing, err := idx.list(ctx, p.TenantID)
	if err != nil {
		return ProjectView{}, err
	}
	want := make(map[string]bool, len(wantIDs))
	for _, wid := range wantIDs {
		want[wid] = true
	}
	for _, r := range existing {
		switch {
		case r.projectID == p.ID && !want[r.id]:
			if err := idx.SetProjectID(ctx, r.id, ""); err != nil {
				return ProjectView{}, err
			}
		case r.projectID != p.ID && want[r.id]:
			if err := idx.SetProjectID(ctx, r.id, p.ID); err != nil {
				return ProjectView{}, err
			}
		}
	}
	return s.view(ctx, p)
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
