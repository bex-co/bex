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
	// ListAllEnvironments backs the w4/m32/t003 backfill sweep (Backfill) —
	// every environment, not scoped to one project.
	ListAllEnvironments(ctx context.Context) ([]store.Environment, error)
	RenameEnvironment(ctx context.Context, id, name string) error
	DeleteEnvironment(ctx context.Context, id string) error
	SetEnvironmentServices(ctx context.Context, environmentID, projectID, tenantID string, serviceNames []string) error
	ListEnvironmentServices(ctx context.Context, environmentID, projectID string) ([]string, error)
	// SetEnvironmentACL replaces the protected-environment ACL triple (w6/m19).
	SetEnvironmentACL(ctx context.Context, id, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) error
	// RepairDriftedEnvironmentIDs backs Backfill's t001-residue repair phase.
	RepairDriftedEnvironmentIDs(ctx context.Context) ([]string, error)
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
	return s.create(ctx, projectID, name)
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
	if err != nil || !hasACL {
		return e, err
	}
	return s.applyACL(ctx, e, req.ProtectedStatus, req.NetworkIsolationEnabled, req.IPAllowList)
}

// create is the post-authorization create body shared by Create and
// CreateWithACL.
func (s *Service) create(ctx context.Context, projectID, name string) (EnvironmentView, error) {
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
	return s.applyACL(ctx, toView(e, nil, nil, nil, nil), status, isolated, allowList)
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

// BackfillReport summarizes one Backfill sweep — an operational count, not an
// API-stable shape.
type BackfillReport struct {
	// DriftedAppsRepaired is how many apps had a pre-w4/m32/t001 stale
	// environment_id NULLed (and their App CR cleared) by this run.
	DriftedAppsRepaired int
	// EnvironmentsSwept is how many environments had their member fan-out
	// re-applied — every environment, not just ones this run changed
	// anything for (the per-resource patches are equality-gated, so a
	// steady-state environment costs a read pass and no writes).
	EnvironmentsSwept int
}

// Backfiller adapts Service to the w4/m32/t003 one-shot admin sweep (the
// `api environments-backfill` subcommand), mirroring ProjectMemberClearer's
// shape: an internal system operation with no request-time caller identity
// — never call s.Authorize/AuthorizeOn in Run or backfill.
type Backfiller struct{ *Service }

// Run executes one Backfill sweep. See backfill's doc comment for what it does.
func (b *Backfiller) Run(ctx context.Context) (BackfillReport, error) {
	return b.backfill(ctx)
}

// backfill is the w4/m32/t003 one-shot idempotent repair: it NULLs any
// apps.environment_id left drifted by pre-w4/m32/t001 SetProjectServices
// calls (RepairDriftedEnvironmentIDs) and clears those apps' CRs, then
// re-applies the standard member fan-out (isolation label + inbound-IP
// layer) to every environment's CURRENT members — the fix for environments
// whose non-empty rules predate w4/m28 and so never triggered the fan-out at
// write time, their members carrying no projected layer at all until an
// unrelated write happened to touch them. Every per-App/Database/KeyValue
// patch this reaches is already equality-gated
// (setAppEnvironmentLabel/setAppEnvironmentAllowList and the Database/
// KeyValue SetEnvironmentIPAllowList callers all skip a no-op patch), so a
// second run finds nothing left to do — safe to re-run, not just a one-time
// migration step. Intended to run once per cluster after this milestone
// deploys, mirroring scripts/ipallowlist-normalize.sh's one-shot-sweep shape.
func (s *Service) backfill(ctx context.Context) (BackfillReport, error) {
	if s.Store == nil {
		return BackfillReport{}, ErrEnvironmentsUnavailable
	}
	var report BackfillReport
	repaired, err := s.Store.RepairDriftedEnvironmentIDs(ctx)
	if err != nil {
		return BackfillReport{}, err
	}
	if len(repaired) > 0 {
		if err := s.clearServiceEnvironmentLayer(ctx, repaired); err != nil {
			return BackfillReport{}, err
		}
		report.DriftedAppsRepaired = len(repaired)
	}
	envs, err := s.Store.ListAllEnvironments(ctx)
	if err != nil {
		return BackfillReport{}, err
	}
	for _, e := range envs {
		names, err := s.Store.ListEnvironmentServices(ctx, e.ID, e.ProjectID)
		if err != nil {
			return BackfillReport{}, err
		}
		if err := s.applyAppEnvironmentLabels(ctx, names, e.ID, e.NetworkIsolationEnabled); err != nil {
			return BackfillReport{}, err
		}
		if err := s.propagateIPAllowList(ctx, e.TenantID, e.ID, names, core.EnvironmentLayerCIDRs(e.IPAllowList)); err != nil {
			return BackfillReport{}, err
		}
		report.EnvironmentsSwept++
	}
	return report, nil
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
	if s.EnvGroups == nil {
		return nil
	}
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
	return nil
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
	layer := core.EnvironmentLayerCIDRs(e.IPAllowList)
	for _, d := range existing {
		switch {
		case d.EnvironmentID == e.ID && !want[d.ID]:
			if err := s.Databases.SetEnvironmentID(ctx, d.ID, ""); err != nil {
				return EnvironmentView{}, err
			}
			if err := s.Databases.SetEnvironmentIPAllowList(ctx, d.ID, nil); err != nil {
				return EnvironmentView{}, err
			}
		case d.EnvironmentID != e.ID && want[d.ID]:
			if err := s.Databases.SetEnvironmentID(ctx, d.ID, e.ID); err != nil {
				return EnvironmentView{}, err
			}
			if err := s.Databases.SetProjectID(ctx, d.ID, e.ProjectID); err != nil {
				return EnvironmentView{}, err
			}
			// Joiners inherit the environment's inbound-IP layer (w4/m28).
			if err := s.Databases.SetEnvironmentIPAllowList(ctx, d.ID, layer); err != nil {
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
	layer := core.EnvironmentLayerCIDRs(e.IPAllowList)
	for _, kv := range existing {
		switch {
		case kv.EnvironmentID == e.ID && !want[kv.ID]:
			if err := s.KeyValues.SetEnvironmentID(ctx, kv.ID, ""); err != nil {
				return EnvironmentView{}, err
			}
			if err := s.KeyValues.SetEnvironmentIPAllowList(ctx, kv.ID, nil); err != nil {
				return EnvironmentView{}, err
			}
		case kv.EnvironmentID != e.ID && want[kv.ID]:
			if err := s.KeyValues.SetEnvironmentID(ctx, kv.ID, e.ID); err != nil {
				return EnvironmentView{}, err
			}
			if err := s.KeyValues.SetProjectID(ctx, kv.ID, e.ProjectID); err != nil {
				return EnvironmentView{}, err
			}
			// Joiners inherit the environment's inbound-IP layer (w4/m28).
			if err := s.KeyValues.SetEnvironmentIPAllowList(ctx, kv.ID, layer); err != nil {
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
	return s.applyACL(ctx, toView(e, nil, nil, nil, nil), protectedStatus, networkIsolationEnabled, ipAllowList)
}

// applyACL is SetACL's post-authorization body and CreateWithACL's second
// phase. The supplied view may be freshly created (no memberships) or loaded
// from the store; membership is re-read below before projection/fan-out.
func (s *Service) applyACL(ctx context.Context, view EnvironmentView, protectedStatus string, networkIsolationEnabled bool, ipAllowList []core.IPAllowListEntry) (EnvironmentView, error) {
	if ipAllowList == nil {
		ipAllowList = []core.IPAllowListEntry{}
	}
	if err := s.Store.SetEnvironmentACL(ctx, view.ID, protectedStatus, networkIsolationEnabled, ipAllowList); err != nil {
		return EnvironmentView{}, mapStoreErr(err)
	}
	view.ProtectedStatus, view.NetworkIsolationEnabled, view.IPAllowList = protectedStatus, networkIsolationEnabled, ipAllowList
	sids, err := s.Store.ListEnvironmentServices(ctx, view.ID, view.ProjectID)
	if err != nil {
		return EnvironmentView{}, err
	}
	// Always resync, regardless of which direction networkIsolationEnabled
	// moved: applyAppEnvironmentLabels is a no-op patch for an App whose label
	// already matches the target state, so this correctly (and cheaply)
	// handles on->off, off->on, and unchanged alike without needing the
	// pre-update value.
	if err := s.applyAppEnvironmentLabels(ctx, sids, view.ID, view.NetworkIsolationEnabled); err != nil {
		return EnvironmentView{}, err
	}
	if err := s.propagateIPAllowList(ctx, view.OwnerID, view.ID, sids, core.EnvironmentLayerCIDRs(ipAllowList)); err != nil {
		return EnvironmentView{}, err
	}
	dids, err := s.databaseIDsForEnvironment(ctx, view.OwnerID, view.ID)
	if err != nil {
		return EnvironmentView{}, err
	}
	kids, err := s.keyValueIDsForEnvironment(ctx, view.OwnerID, view.ID)
	if err != nil {
		return EnvironmentView{}, err
	}
	gidsByEnv, err := s.envGroupIDsByEnvironment(ctx, view.OwnerID)
	if err != nil {
		return EnvironmentView{}, err
	}
	view.ServiceIDs, view.DatabaseIDs, view.KeyValueIDs, view.EnvGroupIDs = sids, dids, kids, gidsByEnv[view.ID]
	return view, nil
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
	if s.Databases != nil {
		dbs, err := s.Databases.ListPostgres(ctx, tenantID)
		if err != nil {
			return err
		}
		for _, d := range dbs {
			if d.EnvironmentID != environmentID {
				continue
			}
			if err := s.Databases.SetEnvironmentIPAllowList(ctx, d.ID, cidrs); err != nil {
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
			if err := s.KeyValues.SetEnvironmentIPAllowList(ctx, kv.ID, cidrs); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyAppAllowLists fans one projected layer over a set of member Apps —
// applyAppEnvironmentLabels' inbound-IP sibling, shared by SetServices and
// propagateIPAllowList so the per-App loop exists once.
func (s *Service) applyAppAllowLists(ctx context.Context, names []string, cidrs []string) error {
	for _, name := range names {
		if err := s.setAppEnvironmentAllowList(ctx, name, cidrs); err != nil {
			return err
		}
	}
	return nil
}

// setAppEnvironmentAllowList is setAppEnvironmentLabel's inbound-IP sibling
// (w4/m28): the same individually-authorized AuthorizeApp seam, patching
// spec.environmentIPAllowList to the projected layer (nil clears it). The
// same nil-Client and stale-name degrades apply.
func (s *Service) setAppEnvironmentAllowList(ctx context.Context, name string, cidrs []string) error {
	if s.Client == nil {
		return nil
	}
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, name)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil
		}
		return err
	}
	if slices.Equal(a.Spec.EnvironmentIPAllowList, cidrs) {
		return nil
	}
	before := a.DeepCopy()
	a.Spec.EnvironmentIPAllowList = cidrs
	return s.Client.Patch(ctx, a, client.MergeFrom(before))
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
