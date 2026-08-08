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
	"slices"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

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
	SetEnvironmentACL(ctx context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) error
}

// DatabaseIndex is the narrow contract environments needs from managed
// Postgres to group Database CRs into an environment (w6/m20 extension) —
// mirroring internal/projects.DatabaseIndex. Unlike services, Databases have
// no control-plane row, so membership is a label (core.LabelEnvironment),
// read via ListPostgres and written via SetEnvironmentID.
// SetEnvironmentIPAllowList (w4/m28) is the ACL fan-out target: it projects
// the ENVIRONMENT rule layer onto a member, never touching the Database's own
// ipAllowList (the pre-m28 fan-out full-replaced it — that clobber is
// retired). *postgres.Service satisfies this structurally.
type DatabaseIndex interface {
	ListPostgres(ctx context.Context, ownerID string) ([]postgres.PostgresView, error)
	SetEnvironmentID(ctx context.Context, name, environmentID string) error
	// SetProjectID joins a newly-assigned Database to the environment's
	// project (mirroring SetEnvironmentServices' apps.project_id stamp — see
	// SetDatabases).
	SetProjectID(ctx context.Context, name, projectID string) error
	SetEnvironmentIPAllowList(ctx context.Context, name string, cidrs []string) error
}

// KeyValueIndex is DatabaseIndex's KeyValue-CR counterpart. *keyvalue.Service
// satisfies this structurally.
type KeyValueIndex interface {
	ListKeyValues(ctx context.Context, ownerID string) ([]keyvalue.KeyValueView, error)
	SetEnvironmentID(ctx context.Context, name, environmentID string) error
	SetProjectID(ctx context.Context, name, projectID string) error
	SetEnvironmentIPAllowList(ctx context.Context, name string, cidrs []string) error
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

// ResolveForCreate is the shared environment-assignment seam used by service,
// Postgres, and Key Value creates. Each create has already bound ownerId to its
// context and authorized can_create against that workspace; this helper owns
// the remaining lookup + same-workspace rule once so the three resource
// packages cannot drift. A foreign existing id is 403, an unknown id is 404.
func (s *Service) resolveForCreate(ctx context.Context, environmentID, workspaceID string) (core.EnvironmentAssignment, error) {
	if s.Store == nil {
		return core.EnvironmentAssignment{}, core.ErrWorkspacesUnavailable
	}
	e, err := s.Store.GetEnvironment(ctx, environmentID)
	if err != nil {
		return core.EnvironmentAssignment{}, mapStoreErr(err)
	}
	if workspaceID == "" || e.TenantID != workspaceID {
		return core.EnvironmentAssignment{}, fmt.Errorf("%w: environment %q does not belong to the target workspace", core.ErrForbidden, environmentID)
	}
	return core.EnvironmentAssignment{
		ID:                      e.ID,
		ProjectID:               e.ProjectID,
		WorkspaceID:             e.TenantID,
		NetworkIsolationEnabled: e.NetworkIsolationEnabled,
		IPAllowList:             append([]core.IPAllowListEntry(nil), e.IPAllowList...),
	}, nil
}

type createResolver struct{ service *Service }

func (r createResolver) ResolveForCreate(ctx context.Context, environmentID, workspaceID string) (core.EnvironmentAssignment, error) {
	return r.service.resolveForCreate(ctx, environmentID, workspaceID)
}

// NewCreateResolver returns the shared resource-create assignment seam. Its
// method is kept off Service because it is plumbing after the parent create
// verb's authorization, not an independently callable API verb.
func NewCreateResolver(service *Service) core.EnvironmentResolver {
	return createResolver{service: service}
}

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
	// IPAllowList is Render's [{cidrBlock, description}] objects — descriptions
	// persist in the store row since w4/m24.
	IPAllowList []core.IPAllowListEntry `json:"ipAllowList"`
}

// CreateEnvironmentRequest is the neutral create input shared by REST,
// GraphQL, and MCP. The ACL triple is optional as a unit: omitted fields use
// the store defaults (unprotected, unisolated, open).
type CreateEnvironmentRequest struct {
	Name                    string                  `json:"name"`
	ProjectID               string                  `json:"projectId"`
	ProtectedStatus         string                  `json:"protectedStatus,omitempty"`
	NetworkIsolationEnabled bool                    `json:"networkIsolationEnabled,omitempty"`
	IPAllowList             []core.IPAllowListEntry `json:"ipAllowList,omitempty"`
}

func (r CreateEnvironmentRequest) hasACL() bool {
	// A non-nil empty IPAllowList is an EXPLICIT deny-all (w4/m28) — it must
	// reach applyACL to overwrite the store's seeded allow-all default; only
	// an absent (nil) list keeps the seed.
	return r.ProtectedStatus != "" || r.NetworkIsolationEnabled || r.IPAllowList != nil
}

// EnvironmentPatch is the neutral partial-update input shared by REST PATCH,
// GraphQL updateEnvironment, and MCP update_environment (w4/m30) — pointer
// fields distinguish "absent" (nil, leave unchanged) from a zero value
// (networkIsolationEnabled: false must be appliable), mirroring
// postgres.PostgresPatch. The ACL fields merge into the environment's current
// triple rather than replacing it wholesale — SetACL/applyACL remain the
// full-replace verb the bex-native /acl route uses.
type EnvironmentPatch struct {
	Name                    *string
	ProtectedStatus         *string
	NetworkIsolationEnabled *bool
	IPAllowList             *[]core.IPAllowListEntry
}

func (p EnvironmentPatch) hasACL() bool {
	return p.ProtectedStatus != nil || p.NetworkIsolationEnabled != nil || p.IPAllowList != nil
}

// environmentResource is the only thing this feature needs to know about a
// member kind whose environment membership is a property of the member itself
// — a CR label (core.LabelEnvironment) or an env group's own metadata — rather
// than a control-plane join row: its id, and the environment it currently
// belongs to. Reducing PostgresView/KeyValueView/EnvironmentMembership to this
// pair is what lets one grouping and one membership-diff serve all three kinds.
type environmentResource struct {
	id            string
	environmentID string
}

// resourceIndex is the shared shape of DatabaseIndex, KeyValueIndex, and
// EnvGroupIndex: a member kind that is listed and re-stamped instead of joined.
// join/leave carry the per-kind side effects of a membership change, so the
// diff loop below is identical for every kind. The adapters embed the
// feature-typed index, so only the listing and the two membership verbs need
// writing.
type resourceIndex interface {
	list(ctx context.Context, workspaceID string) ([]environmentResource, error)
	// join adds id to e — and applies whatever else membership implies for
	// this kind.
	join(ctx context.Context, e store.Environment, id string) error
	// leave removes id from whatever environment it is in, undoing join.
	leave(ctx context.Context, id string) error
}

// layeredIndex is a resourceIndex whose members also carry the environment's
// inbound-IP layer (w4/m28) — Databases and KeyValues have a network surface of
// their own to project onto; env groups do not, which is exactly why SetACL's
// fan-out ranges over this narrower set.
type layeredIndex interface {
	resourceIndex
	setIPLayer(ctx context.Context, id string, cidrs []string) error
}

type databaseResources struct{ DatabaseIndex }

func (d databaseResources) list(ctx context.Context, workspaceID string) ([]environmentResource, error) {
	dbs, err := d.ListPostgres(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]environmentResource, len(dbs))
	for i, db := range dbs {
		out[i] = environmentResource{id: db.ID, environmentID: db.EnvironmentID}
	}
	return out, nil
}

// join stamps the three things membership means for a Database: the
// environment label itself, the environment's project (mirroring
// store.SetEnvironmentServices' apps.project_id stamp — Render's model treats
// "in an environment" as "in that project"), and the environment's inbound-IP
// layer, which joiners inherit (w4/m28).
func (d databaseResources) join(ctx context.Context, e store.Environment, id string) error {
	if err := d.SetEnvironmentID(ctx, id, e.ID); err != nil {
		return err
	}
	if err := d.SetProjectID(ctx, id, e.ProjectID); err != nil {
		return err
	}
	return d.SetEnvironmentIPAllowList(ctx, id, core.EnvironmentLayerCIDRs(e.IPAllowList))
}

// leave drops the environment label and the environment's inbound-IP layer,
// but deliberately NOT the project: leaving an environment doesn't unjoin its
// project, matching store.SetEnvironmentServices' own asymmetry for services.
func (d databaseResources) leave(ctx context.Context, id string) error {
	if err := d.SetEnvironmentID(ctx, id, ""); err != nil {
		return err
	}
	return d.SetEnvironmentIPAllowList(ctx, id, nil)
}

func (d databaseResources) setIPLayer(ctx context.Context, id string, cidrs []string) error {
	return d.SetEnvironmentIPAllowList(ctx, id, cidrs)
}

// keyValueResources is databaseResources' KeyValue-CR counterpart.
type keyValueResources struct{ KeyValueIndex }

func (k keyValueResources) list(ctx context.Context, workspaceID string) ([]environmentResource, error) {
	kvs, err := k.ListKeyValues(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]environmentResource, len(kvs))
	for i, kv := range kvs {
		out[i] = environmentResource{id: kv.ID, environmentID: kv.EnvironmentID}
	}
	return out, nil
}

func (k keyValueResources) join(ctx context.Context, e store.Environment, id string) error {
	if err := k.SetEnvironmentID(ctx, id, e.ID); err != nil {
		return err
	}
	if err := k.SetProjectID(ctx, id, e.ProjectID); err != nil {
		return err
	}
	return k.SetEnvironmentIPAllowList(ctx, id, core.EnvironmentLayerCIDRs(e.IPAllowList))
}

func (k keyValueResources) leave(ctx context.Context, id string) error {
	if err := k.SetEnvironmentID(ctx, id, ""); err != nil {
		return err
	}
	return k.SetEnvironmentIPAllowList(ctx, id, nil)
}

func (k keyValueResources) setIPLayer(ctx context.Context, id string, cidrs []string) error {
	return k.SetEnvironmentIPAllowList(ctx, id, cidrs)
}

// envGroupResources is the env-group member kind: membership is the whole of
// it — no project join, and no inbound-IP layer (hence resourceIndex, not
// layeredIndex).
type envGroupResources struct{ EnvGroupIndex }

func (g envGroupResources) list(ctx context.Context, workspaceID string) ([]environmentResource, error) {
	groups, err := g.ListEnvironmentMemberships(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]environmentResource, len(groups))
	for i, group := range groups {
		out[i] = environmentResource{id: group.ID, environmentID: group.EnvironmentID}
	}
	return out, nil
}

func (g envGroupResources) join(ctx context.Context, e store.Environment, id string) error {
	return g.SetEnvironmentID(ctx, id, e.ID)
}

func (g envGroupResources) leave(ctx context.Context, id string) error {
	return g.SetEnvironmentID(ctx, id, "")
}

// databases/keyValues/envGroups adapt the optional feature indexes, returning
// nil when that kind is unwired — read paths then resolve that kind's
// membership empty (rather than failing the whole view), write paths report
// ErrEnvironmentsUnavailable, matching internal/projects' own nil-degrades
// pattern.
func (s *Service) databases() layeredIndex {
	if s.Databases == nil {
		return nil
	}
	return databaseResources{s.Databases}
}

func (s *Service) keyValues() layeredIndex {
	if s.KeyValues == nil {
		return nil
	}
	return keyValueResources{s.KeyValues}
}

func (s *Service) envGroups() resourceIndex {
	if s.EnvGroups == nil {
		return nil
	}
	return envGroupResources{s.EnvGroups}
}

// idsByEnvironment lists every member idx knows about in workspaceID once and
// buckets the ids by the environment each currently belongs to. One listing
// serves EVERY environment in the workspace, which is what lets List enrich N
// environments with a single call per member kind instead of one per
// environment. Members belonging to no environment (empty environmentID) are
// dropped.
func idsByEnvironment(ctx context.Context, idx resourceIndex, workspaceID string) (map[string][]string, error) {
	if idx == nil {
		return nil, nil
	}
	rs, err := idx.list(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	byEnv := map[string][]string{}
	for _, r := range rs {
		if r.environmentID != "" {
			byEnv[r.environmentID] = append(byEnv[r.environmentID], r.id)
		}
	}
	return byEnv, nil
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
	dbsByEnv, kvsByEnv, groupsByEnv, err := s.membersByEnvironment(ctx, p.TenantID)
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
	e, err := s.create(ctx, projectID, name)
	if err != nil {
		return EnvironmentView{}, err
	}
	return newView(e), nil
}

// CreateWithACL creates an environment and its optional protected-environment
// ACL in one cross-surface verb. Validation happens before the row is written,
// so malformed status/CIDR input cannot leave an orphan environment.
func (s *Service) CreateWithACL(ctx context.Context, req CreateEnvironmentRequest) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	name, err := validateEnvironmentName(req.Name)
	if err != nil {
		return EnvironmentView{}, err
	}
	req.Name = name
	if req.ProjectID == "" {
		return EnvironmentView{}, core.ErrBadRequest
	}
	hasACL := req.hasACL()
	if req.ProtectedStatus == "" {
		req.ProtectedStatus = ProtectedStatusUnprotected
	}
	if hasACL {
		if err := validateACL(req.ProtectedStatus, req.IPAllowList); err != nil {
			return EnvironmentView{}, err
		}
	}
	e, err := s.create(ctx, req.ProjectID, req.Name)
	if err != nil {
		return EnvironmentView{}, err
	}
	if !hasACL {
		return newView(e), nil
	}
	return s.applyACL(ctx, e, req.ProtectedStatus, req.NetworkIsolationEnabled, req.IPAllowList)
}

// create is the post-authorization create body shared by Create and
// CreateWithACL. It returns the store row rather than a view because
// CreateWithACL's second phase (applyACL) needs the row, and because a
// brand-new environment id cannot yet be referenced by any
// service/database/key-value/env group — so Create composes its view with
// newView instead of paying for view's membership fetches (matching
// projects.Service.Create's own toView(p, nil, nil, nil)).
func (s *Service) create(ctx context.Context, projectID, name string) (store.Environment, error) {
	p, err := s.requireProject(ctx, core.RelCanCreate, projectID)
	if err != nil {
		return store.Environment{}, err
	}
	e, err := s.Store.CreateEnvironment(ctx, p.ID, p.TenantID, name)
	if err != nil {
		return store.Environment{}, mapStoreErr(err)
	}
	return e, nil
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

// Update applies a partial update (Render's PATCH /environments/{id}
// semantics — only fields set in patch change; everything else is left
// alone) in one authorize + one fetch, replacing the REST adapter's former
// sequence of independently-authorized Rename/Get/SetACL calls. A rename and
// an ACL merge in the same patch apply together against the one fetched row;
// the ACL half reuses applyACL (SetACL's post-authorization body) so the
// merge-then-full-replace semantics stay identical to the standalone /acl
// route.
func (s *Service) Update(ctx context.Context, id string, patch EnvironmentPatch) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	hasACL := patch.hasACL()
	if patch.Name == nil && !hasACL {
		return EnvironmentView{}, core.ErrBadRequest
	}
	name := ""
	if patch.Name != nil {
		trimmed, err := validateEnvironmentName(*patch.Name)
		if err != nil {
			return EnvironmentView{}, err
		}
		name = trimmed
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	if patch.Name != nil {
		if err := s.Store.RenameEnvironment(ctx, id, name); err != nil {
			return EnvironmentView{}, mapStoreErr(err)
		}
		e.Name = name
	}
	if !hasACL {
		return s.toFullView(ctx, e)
	}
	status, isolated, allowList := e.ProtectedStatus, e.NetworkIsolationEnabled, e.IPAllowList
	if status == "" { // pre-ACL-migration rows surface as empty
		status = ProtectedStatusUnprotected
	}
	if patch.ProtectedStatus != nil {
		status = *patch.ProtectedStatus
	}
	if patch.NetworkIsolationEnabled != nil {
		isolated = *patch.NetworkIsolationEnabled
	}
	if patch.IPAllowList != nil {
		allowList = *patch.IPAllowList
	}
	if err := validateACL(status, allowList); err != nil {
		return EnvironmentView{}, err
	}
	return s.applyACL(ctx, e, status, isolated, allowList)
}

// ProjectMemberClearer adapts Service to the projects feature's
// EnvironmentIndex seam (satisfied structurally — this package never imports
// projects), mirroring apps.WorkspacePurger's shape: an internal system
// operation invoked after the calling verb (projects.Service.SetServices/
// Delete) has already authorized, with no separate caller identity of its
// own to check against — never call s.Authorize/AuthorizeOn in these methods
// (w4/m32).
type ProjectMemberClearer struct{ *Service }

// ClearServiceEnvironmentLayer clears the environment-projected fields
// (core.LabelNetworkIsolation, spec.environmentIPAllowList) on every named
// App CR, with no environment row involved — the seam projects.Service uses
// when a service leaving its project also carries a stale
// apps.environment_id: the store NULLs the column, but nothing else re-syncs
// the already-existing k8s CR after that raw write, and environments' own
// verbs never see the departure (the app already dropped out of
// ListEnvironmentServices by the time this runs).
func (c *ProjectMemberClearer) ClearServiceEnvironmentLayer(ctx context.Context, serviceNames []string) error {
	return c.clearServiceEnvironmentLayer(ctx, serviceNames)
}

func (s *Service) clearServiceEnvironmentLayer(ctx context.Context, serviceNames []string) error {
	if err := s.applyAppEnvironmentLabels(ctx, serviceNames, "", false); err != nil {
		return err
	}
	return s.applyAppAllowLists(ctx, serviceNames, nil)
}

// ClearMembersForProject clears the environment-projected layer from every
// member (App, Database, KeyValue, environment group) of every environment
// under projectID — the seam projects.Service.Delete uses BEFORE the project
// row is deleted, while the environment rows this needs to enumerate still
// exist. The DB cascade (environments.project_id ON DELETE CASCADE, then
// apps.environment_id ON DELETE SET NULL transitively) already leaves the
// store rows consistent on its own; only the k8s CRs need this explicit
// fan-out.
func (c *ProjectMemberClearer) ClearMembersForProject(ctx context.Context, projectID string) error {
	return c.clearMembersForProject(ctx, projectID)
}

func (s *Service) clearMembersForProject(ctx context.Context, projectID string) error {
	if s.Store == nil {
		return nil
	}
	envs, err := s.Store.ListEnvironments(ctx, projectID)
	if err != nil {
		return err
	}
	for _, e := range envs {
		if err := s.clearEnvironmentMembers(ctx, e); err != nil {
			return err
		}
	}
	return nil
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
	if err := s.clearEnvironmentMembers(ctx, e); err != nil {
		return err
	}
	return mapStoreErr(s.Store.DeleteEnvironment(ctx, e.ID))
}

// clearEnvironmentMembers clears the environment-projected layer — the
// isolation label, the inbound-IP layer (Apps/Databases/KeyValues), and env
// group membership — from every current member of e, without touching the
// environment row itself. Shared by Delete (the row disappears right after)
// and clearMembersForProject (w4/m32; the row survives here — only
// projects.Service.Delete's cascade removes it, after every member across
// every child environment has already been cleared).
func (s *Service) clearEnvironmentMembers(ctx context.Context, e store.Environment) error {
	names, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
	if err != nil {
		return err
	}
	if err := s.applyAppEnvironmentLabels(ctx, names, "", false); err != nil {
		return err
	}
	// Clear the inbound-IP layer on every member (w4/m28) — an environment
	// leaving service must not leave its rules enforced on orphaned members.
	if err := s.propagateIPAllowList(ctx, e.TenantID, e.ID, names, nil); err != nil {
		return err
	}
	idx := s.envGroups()
	if idx == nil {
		return nil
	}
	// Env group writes authorize against the CONTEXT workspace (unlike the
	// Database/KeyValue seams, which resolve it from the resource's own
	// labels), so bind the environment's own workspace before touching them.
	ctx = core.WithWorkspace(ctx, e.TenantID)
	groups, err := idx.list(ctx, e.TenantID)
	if err != nil {
		return err
	}
	for _, g := range groups {
		if g.environmentID == e.ID {
			if err := idx.leave(ctx, g.id); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetServices replaces the full list of services in an environment.
// serviceIDs are the public ids shown by list_services (normally srv-...),
// matching internal/projects.Service.SetServices exactly. Legacy service names
// remain accepted by the store during the stable-id transition. The
// assigned services also join the environment's project (see
// store.SetEnvironmentServices). core.LabelNetworkIsolation is synced on
// every affected App CR: cleared on services leaving, set (if the
// environment's networkIsolationEnabled) on services now in it — the
// operator's signal for environment-scoped NetworkPolicy (t004).
func (s *Service) SetServices(ctx context.Context, id string, serviceIDs []string) (EnvironmentView, error) {
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
	if err := s.Store.SetEnvironmentServices(ctx, e.ID, e.ProjectID, e.TenantID, serviceIDs); err != nil {
		return EnvironmentView{}, mapStoreErr(err)
	}
	sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
	if err != nil {
		return EnvironmentView{}, err
	}
	leaving := namesLeaving(before, sids)
	if err := s.applyAppEnvironmentLabels(ctx, leaving, "", false); err != nil {
		return EnvironmentView{}, err
	}
	if err := s.applyAppEnvironmentLabels(ctx, sids, e.ID, e.NetworkIsolationEnabled); err != nil {
		return EnvironmentView{}, err
	}
	// Inbound-IP layer sync (w4/m28): leavers lose the environment layer,
	// members carry the environment's current rules.
	if err := s.applyAppAllowLists(ctx, leaving, nil); err != nil {
		return EnvironmentView{}, err
	}
	if err := s.applyAppAllowLists(ctx, sids, core.EnvironmentLayerCIDRs(e.IPAllowList)); err != nil {
		return EnvironmentView{}, err
	}
	return s.view(ctx, e, sids)
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
	return s.setResourceMembers(ctx, s.databases(), id, databaseIDs, ignoreUnknownIDs)
}

// SetKeyValues is SetDatabases' KeyValue-CR counterpart.
func (s *Service) SetKeyValues(ctx context.Context, id string, keyValueIDs []string) (EnvironmentView, error) {
	return s.setResourceMembers(ctx, s.keyValues(), id, keyValueIDs, ignoreUnknownIDs)
}

// SetEnvGroups replaces the full list of environment groups assigned to an
// Environment. Every requested group must belong to the Environment's own
// workspace; an unknown or foreign id is refused instead of silently ignored.
// Groups already assigned to another Environment in the same workspace move in
// one update, while groups omitted from this Environment are unassigned.
func (s *Service) SetEnvGroups(ctx context.Context, id string, envGroupIDs []string) (EnvironmentView, error) {
	return s.setResourceMembers(ctx, s.envGroups(), id, envGroupIDs, rejectUnknownIDs)
}

// unknownIDPolicy decides what a wanted id that the workspace listing doesn't
// know about means — the one behavior the three Set verbs genuinely differ on.
type unknownIDPolicy int

const (
	// ignoreUnknownIDs drops the id: the listing is scoped to the
	// environment's own workspace, so an id naming a Database/KeyValue
	// elsewhere is simply absent from the diff and never adopted across the
	// tenant boundary.
	ignoreUnknownIDs unknownIDPolicy = iota
	// rejectUnknownIDs refuses the whole call instead, so a typo'd or foreign
	// env group id is reported rather than silently dropped. Nothing has been
	// written by the time the check runs, so a refusal never leaves a partial
	// membership change behind.
	rejectUnknownIDs
)

// setResourceMembers replaces the full membership of one member kind in an
// environment: it diffs the wanted set against the workspace's current
// membership and re-stamps only what actually changed, so an unrelated member
// is never rewritten. Every per-kind difference lives in idx (what join/leave
// imply) or in unknown (what a foreign id means); the authorize-fetch-diff
// shape itself exists once, so it cannot drift between the three Set verbs.
func (s *Service) setResourceMembers(ctx context.Context, idx resourceIndex, id string, wantIDs []string, unknown unknownIDPolicy) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	if idx == nil {
		return EnvironmentView{}, ErrEnvironmentsUnavailable
	}
	// Act inside the environment's OWN workspace: env group writes authorize
	// against the context workspace, while the Database/KeyValue seams resolve
	// it from the resource's own labels and are unaffected — binding it here
	// puts all three kinds on the same resource-scoped rule.
	ctx = core.WithWorkspace(ctx, e.TenantID)
	existing, err := idx.list(ctx, e.TenantID)
	if err != nil {
		return EnvironmentView{}, err
	}
	known := make(map[string]bool, len(existing))
	for _, r := range existing {
		known[r.id] = true
	}
	want := make(map[string]bool, len(wantIDs))
	for _, wid := range wantIDs {
		if unknown == rejectUnknownIDs && !known[wid] {
			// The typed id prefix (ADR020) already names the kind, so one
			// message serves whichever kind opted into rejection.
			return EnvironmentView{}, fmt.Errorf("%w: %q does not belong to workspace %q", core.ErrForbidden, wid, e.TenantID)
		}
		want[wid] = true
	}
	for _, r := range existing {
		switch {
		case r.environmentID == e.ID && !want[r.id]:
			if err := idx.leave(ctx, r.id); err != nil {
				return EnvironmentView{}, err
			}
		case r.environmentID != e.ID && want[r.id]:
			if err := idx.join(ctx, e, r.id); err != nil {
				return EnvironmentView{}, err
			}
		}
	}
	return s.toFullView(ctx, e)
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
func (s *Service) SetACL(ctx context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) (EnvironmentView, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvironmentView{}, err
	}
	if err := validateACL(protectedStatus, ipAllowList); err != nil {
		return EnvironmentView{}, err
	}
	e, err := s.requireEnvironment(ctx, core.RelCanCreate, id)
	if err != nil {
		return EnvironmentView{}, err
	}
	return s.applyACL(ctx, e, protectedStatus, networkIsolationEnabled, ipAllowList)
}

// applyACL is SetACL's post-authorization body and CreateWithACL's second
// phase. e may be a freshly created row or one loaded from the store; the ACL
// triple is written, folded into e, and then projected onto the membership
// this re-reads.
func (s *Service) applyACL(ctx context.Context, e store.Environment, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) (EnvironmentView, error) {
	if ipAllowList == nil {
		ipAllowList = []core.IPAllowListEntry{}
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
	if err := s.propagateIPAllowList(ctx, e.TenantID, e.ID, sids, core.EnvironmentLayerCIDRs(ipAllowList)); err != nil {
		return EnvironmentView{}, err
	}
	return s.view(ctx, e, sids)
}

// validateEnvironmentName trims and rejects an empty environment name — the
// one check CreateWithACL and Update (w4/m30) both need before writing,
// factored out so the two verbs cannot drift on it.
func validateEnvironmentName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", core.ErrBadRequest
	}
	return name, nil
}

// validateACL is SetACL's input check, factored out so the Render-shaped
// REST create can reject a bad ACL BEFORE creating the environment (w4/017)
// — one source of truth, no orphan row on a 400.
func validateACL(protectedStatus string, ipAllowList []core.IPAllowListEntry) error {
	if protectedStatus != ProtectedStatusProtected && protectedStatus != ProtectedStatusUnprotected {
		return fmt.Errorf("%w: protectedStatus must be %q or %q", core.ErrBadRequest, ProtectedStatusProtected, ProtectedStatusUnprotected)
	}
	return core.ValidateAllowList(ipAllowList)
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

// membersByEnvironment resolves all three label-backed member kinds for a whole
// workspace in one call per kind — the fetch List shares across every row, and
// the one view (below) needs for a single environment. Kinds left unwired
// resolve to a nil map, so their membership reads empty rather than failing the
// view.
func (s *Service) membersByEnvironment(ctx context.Context, workspaceID string) (databases, keyValues, envGroups map[string][]string, err error) {
	if databases, err = idsByEnvironment(ctx, s.databases(), workspaceID); err != nil {
		return nil, nil, nil, err
	}
	if keyValues, err = idsByEnvironment(ctx, s.keyValues(), workspaceID); err != nil {
		return nil, nil, nil, err
	}
	if envGroups, err = idsByEnvironment(ctx, s.envGroups(), workspaceID); err != nil {
		return nil, nil, nil, err
	}
	return databases, keyValues, envGroups, nil
}

// view composes the API view of e from the service membership the caller has
// already read plus a fresh read of the three label-backed member kinds. Every
// verb returning one environment ends here, so a mutating verb always reports
// the same membership a plain Get would.
func (s *Service) view(ctx context.Context, e store.Environment, serviceIDs []string) (EnvironmentView, error) {
	dids, kids, gids, err := s.membersByEnvironment(ctx, e.TenantID)
	if err != nil {
		return EnvironmentView{}, err
	}
	return toView(e, serviceIDs, dids[e.ID], kids[e.ID], gids[e.ID]), nil
}

// toFullView is view for a caller that hasn't already read the service
// membership.
func (s *Service) toFullView(ctx context.Context, e store.Environment) (EnvironmentView, error) {
	sids, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
	if err != nil {
		return EnvironmentView{}, err
	}
	return s.view(ctx, e, sids)
}

// newView is the memberless view of e — the shape a brand-new environment id
// has, before anything can possibly reference it.
func newView(e store.Environment) EnvironmentView {
	return toView(e, nil, nil, nil, nil)
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
		ipAllowList = []core.IPAllowListEntry{}
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

// patchApps applies mutate to every named App CR, one merge-patch each. Every
// App is fetched via AuthorizeApp — the same authorize-and-audit seam
// apps.Service itself uses — so each fan-out write is individually authorized
// (and its target individually audited), not just once at the environment
// level. mutate reports whether it actually changed anything; an App already in
// the wanted state is not patched. Client nil (no k8s cluster wired — unit
// tests, or a DB-less deploy) is a no-op: the same degrade every other
// CR-touching feature uses, never a panic on a nil client.
func (s *Service) patchApps(ctx context.Context, names []string, mutate func(a *appv1alpha1.App) bool) error {
	if s.Client == nil {
		return nil
	}
	for _, name := range names {
		a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
		if err != nil {
			// A member name with no matching App CR (a stale row, or a race
			// with a concurrent delete) is skipped, not fatal — mirroring
			// store.SetEnvironmentServices' own "names not found are silently
			// skipped" convention rather than failing the whole ACL/membership
			// change over one dangling name.
			if errors.Is(err, core.ErrNotFound) {
				continue
			}
			return err
		}
		before := a.DeepCopy()
		if !mutate(a) {
			continue
		}
		if err := s.Client.Patch(ctx, a, client.MergeFrom(before)); err != nil {
			return err
		}
	}
	return nil
}

// applyAppEnvironmentLabels reconciles core.LabelNetworkIsolation on every
// named App CR to (envID, isolated): present (= envID) when isolated, absent
// otherwise. Callers pass ("", false) to clear unconditionally — a service
// leaving its environment (SetServices) or the environment itself being
// deleted (Delete) — or (e.ID, e.NetworkIsolationEnabled) to sync to an
// environment's current state (SetServices' new members, SetACL's toggle).
func (s *Service) applyAppEnvironmentLabels(ctx context.Context, names []string, envID string, isolated bool) error {
	want := isolated && envID != ""
	return s.patchApps(ctx, names, func(a *appv1alpha1.App) bool {
		if !want {
			if _, ok := a.Labels[core.LabelNetworkIsolation]; !ok {
				return false
			}
			delete(a.Labels, core.LabelNetworkIsolation)
			return true
		}
		if a.Labels[core.LabelNetworkIsolation] == envID {
			return false
		}
		if a.Labels == nil {
			a.Labels = map[string]string{}
		}
		a.Labels[core.LabelNetworkIsolation] = envID
		return true
	})
}

// propagateIPAllowList fans an environment's inbound-IP layer out to every
// member: Databases/KeyValues whose core.LabelEnvironment (w6/m20) names this
// environment, and (w4/m28) the member Apps' spec.environmentIPAllowList —
// each member keeps its OWN spec.ipAllowList untouched (pre-m28 this verb
// full-replaced the datastores' own lists, clobbering service-level rules);
// the operator chains one middleware per layer so a source must pass both.
// cidrs is the projected layer (core.EnvironmentLayerCIDRs — allow-list, or
// the deny-all placeholder for an explicitly empty rule set), or nil to CLEAR
// the layer (environment deleted / member leaving). serviceNames are the
// member Apps. Databases/KeyValues nil (unwired) => datastore half no-ops,
// matching internal/projects' own degrade.
func (s *Service) propagateIPAllowList(ctx context.Context, tenantID, environmentID string, serviceNames, cidrs []string) error {
	if err := s.applyAppAllowLists(ctx, serviceNames, cidrs); err != nil {
		return err
	}
	// Only the kinds with a network surface of their own carry the layer —
	// env groups are members but have nothing to project onto.
	for _, idx := range []layeredIndex{s.databases(), s.keyValues()} {
		if idx == nil {
			continue
		}
		members, err := idx.list(ctx, tenantID)
		if err != nil {
			return err
		}
		for _, m := range members {
			if m.environmentID != environmentID {
				continue
			}
			if err := idx.setIPLayer(ctx, m.id, cidrs); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyAppAllowLists fans one projected layer over a set of member Apps —
// applyAppEnvironmentLabels' inbound-IP sibling (w4/m28), patching
// spec.environmentIPAllowList to the projected layer (nil clears it). Shared by
// SetServices and propagateIPAllowList so the per-App loop exists once.
func (s *Service) applyAppAllowLists(ctx context.Context, names []string, cidrs []string) error {
	return s.patchApps(ctx, names, func(a *appv1alpha1.App) bool {
		if slices.Equal(a.Spec.EnvironmentIPAllowList, cidrs) {
			return false
		}
		a.Spec.EnvironmentIPAllowList = cidrs
		return true
	})
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
