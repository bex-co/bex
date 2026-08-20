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

// Package envgroups is the environment-groups feature (Render's /v1/env-groups):
// a named, reusable set of env vars + secret files that can be linked to many
// services at once. It reuses the same OpenBao-backed core.SecretKV store as the
// per-service secrets feature (no new store) — a group's contents live at
// "env-groups/<id>/{meta,env,files}" — and materializes each group into two
// Kubernetes Secrets ("<id>-env", "<id>-files"). Linking a group to a service
// appends those Secret names to the service's App spec (spec.envFromSecrets /
// spec.filesFromSecrets); the operator wires them into the container's envFrom and
// the shared /etc/secrets volume. One implementation, three surfaces + dashboard.
package envgroups

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// maxHydratedEnvGroups bounds the number of groups fully hydrated (env + file
// maps) per list request. Without this, a tenant with many groups triggers two
// OpenBao reads per group on every list, unbounded by the requested page size
// (codex #10). 200 is well above any reasonable page limit.
const maxHydratedEnvGroups = 200

// Round-11 #3 knob: the per-workspace create quota (default; 0 disables via
// the env wiring).
const defaultMaxEnvGroups = 100

// Service manages environment groups over the shared core.SecretKV store and
// projects them into linked services. Embeds *core.Base for the client, clock, and
// authorization gate.
type Service struct {
	*core.Base
	Store core.SecretKV
	// EnvironmentWorkspace resolves an Environment id to its owning workspace.
	// The composition root wires this to environments.Service.Get. It is only
	// required when a caller supplies environmentId; nil keeps standalone env
	// groups available while honestly refusing an association the process cannot
	// validate.
	EnvironmentWorkspace func(ctx context.Context, environmentID string) (string, error)
	// RebuildService starts one full rebuild for a linked service. The API
	// composition root wires deploys.Service.Trigger; nil makes only the explicit
	// rebuild save mode unavailable while save_only/deploy continue to work.
	RebuildService func(ctx context.Context, serviceID string) error

	// MaxEnvGroups caps how many groups ONE workspace may own (round-11 #3,
	// BEX_MAX_ENV_GROUPS_PER_WORKSPACE, default 100; 0 disables). w2/m80: the
	// count is now a prefix-scoped list (listGroupIDs) rather than a global
	// metadata sweep, so this quota is what bounds the walk — no cache needed.
	MaxEnvGroups int
}

// EnvVarView is a group env var ({key, value}); value is empty in list/get
// responses (fetched per key), present only on the sensitive single-var read.
type EnvVarView struct {
	Key           string `json:"key"`
	Value         string `json:"value,omitempty"`
	ValueSet      bool   `json:"-"`
	GenerateValue bool   `json:"generateValue,omitempty"`
}

func (v EnvVarView) MarshalJSON() ([]byte, error) {
	type wire struct {
		Key           string  `json:"key"`
		Value         *string `json:"value,omitempty"`
		GenerateValue bool    `json:"generateValue,omitempty"`
	}
	out := wire{Key: v.Key, GenerateValue: v.GenerateValue}
	if v.ValueSet {
		out.Value = &v.Value
	}
	return json.Marshal(out)
}

func (v *EnvVarView) UnmarshalJSON(data []byte) error {
	type wire EnvVarView
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*v = EnvVarView(decoded)
	_, v.ValueSet = fields["value"]
	return nil
}

// SecretFileView is a group secret file ({name, content}); content follows the
// same names-first discipline as EnvVarView.
type SecretFileView struct {
	Name    string `json:"name"`
	Content string `json:"content,omitempty"`
}

// CreateEnvVarInput is Render's create-time value-or-generate env-var shape.
// GenerateValue is resolved before the group id is minted, so a generation or
// validation failure cannot leave a partially created group behind.
type CreateEnvVarInput struct {
	Key           string `json:"key"`
	Value         string `json:"value,omitempty"`
	ValueSet      bool   `json:"-"`
	GenerateValue bool   `json:"generateValue,omitempty"`
}

func (v *CreateEnvVarInput) UnmarshalJSON(data []byte) error {
	type wire CreateEnvVarInput
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*v = CreateEnvVarInput(decoded)
	_, v.ValueSet = fields["value"]
	return nil
}

// CreateEnvGroupRequest is the one neutral create input shared by REST,
// GraphQL, and MCP. All optional initial contents and links are validated before
// any store, Secret, or App mutation is attempted.
type CreateEnvGroupRequest struct {
	Name          string              `json:"name"`
	OwnerID       string              `json:"ownerId,omitempty"`
	EnvironmentID string              `json:"environmentId,omitempty"`
	EnvVars       []CreateEnvVarInput `json:"envVars,omitempty"`
	SecretFiles   []SecretFileView    `json:"secretFiles,omitempty"`
	ServiceIDs    []string            `json:"serviceIds,omitempty"`
}

// EnvGroupView is the Render-shaped env-group object. EnvVars/SecretFiles carry
// keys/names only (no secret material) — a list/get never leaks values; the
// per-var / per-file reveal verbs return them under the sensitive scope.
// OwnerID/CreatedAt/UpdatedAt (w6/m24, Render's `ownerId`/timestamps) are sourced
// from the group's own stored meta, never faked when unknown (omitempty) —
// AppView.OwnerID's own convention. EnvironmentID is the optional Render-shaped
// Environment membership added by w6/m24/t011.
type EnvGroupView struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	OwnerID       string           `json:"ownerId,omitempty"`
	EnvironmentID string           `json:"environmentId,omitempty"`
	ServiceLinks  []string         `json:"serviceLinks"`
	EnvVars       []EnvVarView     `json:"envVars"`
	SecretFiles   []SecretFileView `json:"secretFiles"`
	CreatedAt     string           `json:"createdAt,omitempty"`
	UpdatedAt     string           `json:"updatedAt,omitempty"`
	Revision      string           `json:"revision,omitempty"`
}

// --- store paths + materialized Secret names ----------------------------------

func metaPath(gid string) string  { return "env-groups/" + gid + "/meta" }
func envPath(gid string) string   { return "env-groups/" + gid + "/env" }
func filesPath(gid string) string { return "env-groups/" + gid + "/files" }

// envSecretName / filesSecretName are the per-group projection Secrets linked
// services consume (envFrom + /etc/secrets projected volume).
func envSecretName(gid string) string   { return gid + "-env" }
func filesSecretName(gid string) string { return gid + "-files" }

// meta is a group's non-secret metadata, stored as a string map in the KV
// store. workspace (w6/m24) is the owning workspace (tenant) id — "" only for a
// group created (or never re-read) while the control-plane store is off; see
// readMeta's migration for how a pre-attribution group gets one.
type meta struct {
	name        string
	links       []string
	workspace   string
	environment string
	createdAt   string
	updatedAt   string
}

// --- group lifecycle ----------------------------------------------------------

// ListEnvGroups returns ownerID's environment groups (no secret material —
// keys/names and links only); "" means the caller's default workspace (w6/m24,
// mirroring apikeys.ListAPIKeys). With the control-plane store off (Workspace
// nil), groups aren't attributed and the list stays unfiltered, byte-identical
// to before. View scope.
func (s *Service) ListEnvGroups(ctx context.Context, ownerID string) ([]EnvGroupView, error) {
	filter := EnvGroupListFilter{}
	if ownerID != "" {
		filter.OwnerIDs = []string{ownerID}
	}
	return s.ListEnvGroupsFiltered(ctx, filter)
}

// EnvGroupListFilter is Render's GET /v1/env-groups filter contract. Values
// within one slice are OR alternatives; different fields compose with AND.
// Empty OwnerIDs targets the caller's default workspace.
type EnvGroupListFilter struct {
	Names          []string
	OwnerIDs       []string
	EnvironmentIDs []string
	CreatedBefore  time.Time
	CreatedAfter   time.Time
	UpdatedBefore  time.Time
	UpdatedAfter   time.Time
}

// ListEnvGroupsFiltered narrows metadata before loading group contents and
// before any caller paginates the result. Multiple owner ids are each resolved
// through the existing membership-checked workspace seam, then unioned.
func (s *Service) ListEnvGroupsFiltered(ctx context.Context, filter EnvGroupListFilter) ([]EnvGroupView, error) {
	owners := filter.OwnerIDs
	if len(owners) == 0 {
		owners = []string{""}
	}
	seen := make(map[string]struct{})
	var matched []scopedMeta
	for _, ownerID := range owners {
		groups, err := s.listScopedMeta(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		for _, group := range groups {
			if _, ok := seen[group.id]; ok || !matchesEnvGroupFilter(group, filter) {
				continue
			}
			seen[group.id] = struct{}{}
			matched = append(matched, group)
		}
	}
	// codex #10: bound the backend work — without this cap, each matched group
	// triggers two OpenBao reads (env map + file map), so a tenant with many
	// groups could exhaust API/OpenBao capacity on every list request. The cap is
	// well above any reasonable page limit, so ordinary pagination is unaffected.
	if len(matched) > maxHydratedEnvGroups {
		matched = matched[:maxHydratedEnvGroups]
	}
	out := make([]EnvGroupView, 0, len(matched))
	for _, group := range matched {
		view, err := s.viewFromMeta(ctx, group.id, group.meta)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func matchesEnvGroupFilter(group scopedMeta, filter EnvGroupListFilter) bool {
	return (len(filter.Names) == 0 || slices.Contains(filter.Names, group.name)) &&
		(len(filter.EnvironmentIDs) == 0 || slices.Contains(filter.EnvironmentIDs, group.environment)) &&
		core.TimeWindow{Before: filter.CreatedBefore, After: filter.CreatedAfter}.Contains(group.createdAt) &&
		core.TimeWindow{Before: filter.UpdatedBefore, After: filter.UpdatedAfter}.Contains(group.updatedAt)
}

// pageEnvGroups is the shared GraphQL/MCP/REST paging rule. Callers pass
// requested=false when neither cursor nor limit was supplied, preserving the
// historical complete-list behavior; an explicit page is stable by group id.
func pageEnvGroups(groups []EnvGroupView, cursor string, limit int, requested bool) []EnvGroupView {
	return core.StablePage(groups, cursor, core.PageLimitOrDefault(limit), requested, func(group EnvGroupView) string { return group.ID })
}

// EnvironmentMembership is the non-secret, narrow projection the Environments
// feature needs to assemble Environment.envGroupIds without reading every
// group's env-var and secret-file maps.
type EnvironmentMembership struct {
	ID            string
	EnvironmentID string
}

// ListEnvironmentMemberships returns one workspace's group-to-Environment
// assignments without loading secret contents. It shares ListEnvGroups' exact
// workspace binding and authorization path.
func (s *Service) ListEnvironmentMemberships(ctx context.Context, ownerID string) ([]EnvironmentMembership, error) {
	groups, err := s.listScopedMeta(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]EnvironmentMembership, 0, len(groups))
	for _, group := range groups {
		out = append(out, EnvironmentMembership{ID: group.id, EnvironmentID: group.meta.environment})
	}
	return out, nil
}

type scopedMeta struct {
	id string
	meta
}

// listScopedMeta is the shared metadata-only list core. Keeping it below both
// public reads prevents Environment membership lookups from fetching secret
// maps while preserving one scoping implementation. w2/m80: the sweep behind
// it is prefix-scoped (listGroupIDs) to the bound workspace's own OpenBao
// tenant, unioned with the shared legacy tenant only for the dual-read
// migration window — replacing the pre-m80 unscoped global-index sweep (which
// a short-TTL cache, since retired, bounded instead).
func (s *Service) listScopedMeta(ctx context.Context, ownerID string) ([]scopedMeta, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
	}
	scopeTo, scoped := s.boundWorkspace(ctx)
	if scoped && scopeTo == "" {
		return []scopedMeta{}, nil
	}
	ids, err := s.listGroupIDs(ctx, scopeTo, true)
	if err != nil {
		return nil, err
	}
	sort.Strings(ids)
	out := make([]scopedMeta, 0, len(ids))
	for _, gid := range ids {
		m, err := s.readMeta(ctx, gid)
		if errors.Is(err, core.ErrNotFound) {
			continue // create publishes metadata last; ignore an in-flight id
		}
		if err != nil {
			return nil, err
		}
		if scoped && m.workspace != scopeTo {
			continue // a legacy-union entry that turned out to belong elsewhere
		}
		out = append(out, scopedMeta{id: gid, meta: m})
	}
	return out, nil
}

// GetEnvGroup returns one group (keys/names + links, no values); ErrForbidden
// for a group belonging to a workspace the caller can't reach. View scope.
func (s *Service) GetEnvGroup(ctx context.Context, gid string) (EnvGroupView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanView, gid)
	if err != nil {
		return EnvGroupView{}, err
	}
	return s.viewFromMeta(ctx, gid, m)
}

// CreateEnvGroup atomically creates a group in req.OwnerID's workspace ("" =>
// the caller's default), optionally populated with vars/files and linked to
// services. Every value, filename, Environment, and service link is validated
// before the id is minted or any state is written. Runtime write failures are
// compensated best-effort across OpenBao, projection Secrets, and App patches;
// validation failures therefore have exactly zero side effects. Manage scope.
func (s *Service) CreateEnvGroup(ctx context.Context, req CreateEnvGroupRequest) (EnvGroupView, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return EnvGroupView{}, err
	}
	return s.createEnvGroupAuthorized(ctx, req)
}

// createEnvGroupAuthorized is the target-side half shared with CloneEnvGroup.
// Its caller has already bound req.OwnerID and authorized create on that exact
// workspace, so source cloning never needs to reveal values to a client or call
// a second public mutation.
func (s *Service) createEnvGroupAuthorized(ctx context.Context, req CreateEnvGroupRequest) (EnvGroupView, error) {
	if s.Store == nil {
		return EnvGroupView{}, core.ErrSecretsUnavailable
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return EnvGroupView{}, fmt.Errorf("%w: env group name is required", core.ErrBadRequest)
	}
	workspace, _ := s.Tenant(ctx) // "" with the store off, matching AppView.OwnerID
	if err := s.validateEnvironment(ctx, req.EnvironmentID, workspace); err != nil {
		return EnvGroupView{}, err
	}
	// Round-11 #3: per-workspace create quota, checked before the id is minted
	// or any state written. Counted from the metadata snapshot, so it is
	// best-effort under concurrent creates — an abuse bound, not a transaction.
	if err := s.envGroupQuota(ctx, workspace); err != nil {
		return EnvGroupView{}, err
	}
	env, err := prepareCreateEnv(req.EnvVars)
	if err != nil {
		return EnvGroupView{}, err
	}
	files, err := prepareCreateFiles(req.SecretFiles)
	if err != nil {
		return EnvGroupView{}, err
	}
	services, links, err := s.prepareCreateServices(ctx, req.ServiceIDs, workspace, req.EnvironmentID)
	if err != nil {
		return EnvGroupView{}, err
	}

	gid := id.New(id.EnvGroup)
	if err := s.claimGroupName(ctx, workspace, name, gid); err != nil {
		return EnvGroupView{}, err
	}
	now := s.now()
	m := meta{
		name: name, links: links, workspace: workspace, environment: req.EnvironmentID,
		createdAt: now, updatedAt: now,
	}
	if err := s.persistCreate(ctx, gid, m, env, files, services); err != nil {
		return EnvGroupView{}, errors.Join(err, s.releaseGroupName(context.WithoutCancel(ctx), workspace, name, gid))
	}
	return s.viewFromMeta(ctx, gid, m)
}

// prepareCreateEnv validates and resolves Render's literal-or-generated input
// before CreateEnvGroup mints an id. Duplicate keys follow the existing
// replace-all semantics: the last declaration wins.
func prepareCreateEnv(vars []CreateEnvVarInput) (map[string]string, error) {
	env := make(map[string]string, len(vars))
	for _, input := range vars {
		key := strings.TrimSpace(input.Key)
		if !core.ValidEnvKey(key) {
			return nil, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		hasValue := input.ValueSet || input.Value != ""
		if input.GenerateValue == hasValue {
			return nil, core.InvalidEnvVarValueInput(key)
		}
		value := input.Value
		if input.GenerateValue {
			var err error
			value, err = core.GenerateValue()
			if err != nil {
				return nil, err
			}
		}
		env[key] = value
	}
	return env, nil
}

// prepareCreateFiles validates every filename without ever including its
// content in an error. Duplicate names use the final supplied content.
func prepareCreateFiles(files []SecretFileView) (map[string]string, error) {
	out := make(map[string]string, len(files))
	for _, input := range files {
		name := strings.TrimSpace(input.Name)
		if !core.ValidSecretFileName(name) {
			return nil, fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, name)
		}
		out[name] = input.Content
	}
	return out, nil
}

// prepareCreateServices resolves every serviceId inside the create workspace
// without calling another public verb (and therefore without a second audit
// event). It is read-only: all links are proven before any group state exists.
func (s *Service) prepareCreateServices(
	ctx context.Context,
	serviceIDs []string,
	workspace string,
	environmentID string,
) ([]*appv1alpha1.App, []string, error) {
	apps := make([]*appv1alpha1.App, 0, len(serviceIDs))
	links := make([]string, 0, len(serviceIDs))
	seen := make(map[string]struct{}, len(serviceIDs))
	for _, raw := range serviceIDs {
		serviceID := strings.TrimSpace(raw)
		if serviceID == "" {
			return nil, nil, fmt.Errorf("%w: serviceId is required", core.ErrBadRequest)
		}
		if _, ok := seen[serviceID]; ok {
			continue
		}
		a, err := s.findCreateService(ctx, serviceID, workspace)
		if err != nil {
			return nil, nil, fmt.Errorf("serviceId %q: %w", serviceID, err)
		}
		if err := validateGroupServiceEnvironment(environmentID, serviceID, a.Labels); err != nil {
			return nil, nil, err
		}
		seen[serviceID] = struct{}{}
		apps = append(apps, a)
		links = append(links, serviceID)
	}
	return apps, links, nil
}

// findCreateService reuses Base's id/name-compatible App resolution, then
// narrows the result to the group workspace. GetApp's resource gate has no
// audit side effect, so the composite create still emits exactly one event.
func (s *Service) findCreateService(ctx context.Context, serviceID, workspace string) (*appv1alpha1.App, error) {
	a, err := s.GetApp(ctx, core.RelCanCreate, serviceID)
	if err != nil {
		return nil, err
	}
	if !s.createWorkspaceMatches(a.Labels, workspace) {
		return nil, core.ErrForbidden
	}
	return a, nil
}

func (s *Service) createWorkspaceMatches(labels map[string]string, workspace string) bool {
	if s.Workspace == nil {
		return true // store-off compatibility: one unscoped workspace
	}
	return labels[core.LabelTenant] == workspace
}

// persistCreate applies the already-validated create plan. The metadata write
// is last, so the group becomes discoverable only after its contents,
// projection Secrets, and service refs exist. Any error triggers compensation.
func (s *Service) persistCreate(
	ctx context.Context,
	gid string,
	m meta,
	env, files map[string]string,
	services []*appv1alpha1.App,
) error {
	patched := make([]*appv1alpha1.App, 0, len(services))
	rollback := func(cause error) error {
		cleanupCtx := context.WithoutCancel(ctx)
		var cleanup []error
		for i := len(patched) - 1; i >= 0; i-- {
			before := patched[i]
			var current appv1alpha1.App
			key := client.ObjectKeyFromObject(before)
			if err := s.Client.Get(cleanupCtx, key, &current); err != nil {
				cleanup = append(cleanup, fmt.Errorf("restore service %q: %w", before.Name, err))
				continue
			}
			restored := before.DeepCopy()
			restored.ResourceVersion = current.ResourceVersion
			if err := s.Client.Patch(cleanupCtx, restored, client.MergeFrom(current.DeepCopy())); err != nil {
				cleanup = append(cleanup, fmt.Errorf("restore service %q: %w", before.Name, err))
			}
		}
		for _, name := range []string{envSecretName(gid), filesSecretName(gid)} {
			if err := s.deleteSecret(cleanupCtx, m.workspace, name); err != nil {
				cleanup = append(cleanup, fmt.Errorf("delete projection Secret %q: %w", name, err))
			}
		}
		if err := s.deleteGroupArtifacts(cleanupCtx, m.workspace, gid); err != nil {
			cleanup = append(cleanup, fmt.Errorf("delete group %q artifacts: %w", gid, err))
		}
		if len(cleanup) == 0 {
			return cause
		}
		return errors.Join(append([]error{cause}, cleanup...)...)
	}

	if err := s.storeMap(ctx, m.workspace, envPath(gid), env); err != nil {
		return rollback(err)
	}
	if err := s.storeMap(ctx, m.workspace, filesPath(gid), files); err != nil {
		return rollback(err)
	}
	if versioned, ok := s.Store.(core.VersionedSecretKV); ok {
		if _, err := versioned.PutCAS(groupCtx(ctx, m.workspace), revisionPath(gid), map[string]string{"state": "idle", "generation": "1"}, 0); err != nil {
			return rollback(err)
		}
	}
	if err := s.upsertSecret(ctx, m.workspace, envSecretName(gid), env); err != nil {
		return rollback(err)
	}
	if err := s.upsertSecret(ctx, m.workspace, filesSecretName(gid), files); err != nil {
		return rollback(err)
	}
	for _, a := range services {
		before := a.DeepCopy()
		base := client.MergeFrom(before)
		a.Spec.EnvFromSecrets = addString(a.Spec.EnvFromSecrets, envSecretName(gid))
		a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, filesSecretName(gid))
		a.Spec.RestartedAt = m.updatedAt
		if err := s.Client.Patch(ctx, a, base); err != nil {
			return rollback(err)
		}
		patched = append(patched, before)
	}
	if err := s.writeMeta(ctx, gid, m); err != nil {
		return rollback(err)
	}
	return nil
}

// SetEnvironmentID assigns or unassigns one group from an Environment. A
// non-empty Environment must exist in the group's own workspace; moving a group
// between Environments is a single metadata update. Manage scope.
func (s *Service) SetEnvironmentID(ctx context.Context, gid, environmentID string) error {
	_, err := s.MoveEnvGroup(ctx, gid, environmentID)
	return err
}

// MoveEnvGroup is SetEnvironmentID's view-returning public contract for the
// REST/GraphQL/MCP move workflow. The group id stays pinned throughout and the
// metadata write happens only after every linked service has been validated.
func (s *Service) MoveEnvGroup(ctx context.Context, gid, environmentID string) (EnvGroupView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return EnvGroupView{}, err
	}
	if err := s.validateEnvironment(ctx, environmentID, m.workspace); err != nil {
		return EnvGroupView{}, err
	}
	if m.environment == environmentID {
		return s.viewFromMeta(ctx, gid, m)
	}
	if err := s.validateLinkedServiceEnvironments(ctx, m.links, environmentID); err != nil {
		return EnvGroupView{}, err
	}
	m.environment = environmentID
	if m, err = s.touch(ctx, gid, m); err != nil {
		return EnvGroupView{}, err
	}
	return s.viewFromMeta(ctx, gid, m)
}

// SetGroupEnvironment assigns the named env group to an Environment, satisfying
// the apps.EnvGroupApplier seam called by the Blueprint apply path when a group is
// declared under projects[].environments[].envVarGroups. The lookup-by-name is
// workspace-scoped (the caller's tenant context), matching ApplyEnvGroup's contract.
func (s *Service) SetGroupEnvironment(ctx context.Context, name, environmentID string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	gid, _, found, err := s.findGroupByName(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: env group %q not found", core.ErrNotFound, name)
	}
	return s.SetEnvironmentID(ctx, gid, environmentID)
}

// RenameEnvGroup updates a group's display name without changing its id,
// contents, links, or materialized Secrets. The name is metadata only, so this
// does not roll linked services. Manage scope.
func (s *Service) RenameEnvGroup(ctx context.Context, gid, name string) (EnvGroupView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return EnvGroupView{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return EnvGroupView{}, fmt.Errorf("%w: env group name is required", core.ErrBadRequest)
	}
	if m.name == name {
		return s.viewFromMeta(ctx, gid, m)
	}
	if err := s.claimGroupName(ctx, m.workspace, name, gid); err != nil {
		return EnvGroupView{}, err
	}
	old := m
	cleanupCtx := context.WithoutCancel(ctx)
	if err := s.releaseGroupName(cleanupCtx, old.workspace, old.name, gid); err != nil {
		return EnvGroupView{}, errors.Join(err, s.releaseGroupName(cleanupCtx, old.workspace, name, gid))
	}
	m.name = name
	if m, err = s.touch(ctx, gid, m); err != nil {
		return EnvGroupView{}, errors.Join(
			err,
			s.claimGroupName(cleanupCtx, old.workspace, old.name, gid),
			s.releaseGroupName(cleanupCtx, old.workspace, name, gid),
		)
	}
	return s.viewFromMeta(ctx, gid, m)
}

// DeleteEnvGroup unlinks the group from every service, deletes its projection
// Secrets, and removes its store paths. Manage scope.
func (s *Service) DeleteEnvGroup(ctx context.Context, gid string) error {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	// Release the claim before deleting metadata. Until metadata is removed, the
	// fresh legacy sweep still blocks reuse; after it is removed there is no stale
	// claim. Stopping here on failure keeps a retry possible and fail-closed.
	if err := s.releaseGroupName(context.WithoutCancel(ctx), m.workspace, m.name, gid); err != nil {
		return err
	}
	// Detach from linked services first (drop the spec refs + roll) so no pod is
	// left referencing a Secret about to be deleted.
	for _, svc := range m.links {
		if err := s.detach(ctx, gid, svc); err != nil {
			return err
		}
	}
	if err := s.deleteSecret(ctx, m.workspace, envSecretName(gid)); err != nil {
		return err
	}
	if err := s.deleteSecret(ctx, m.workspace, filesSecretName(gid)); err != nil {
		return err
	}
	return s.deleteGroupArtifacts(ctx, m.workspace, gid)
}

// --- group contents (env vars) ------------------------------------------------

// SetEnvGroupVars replaces the group's whole env set, re-materializes the group's
// env Secret, and rolls every linked service. Manage scope.
func (s *Service) SetEnvGroupVars(ctx context.Context, gid string, vars []EnvVarView) ([]EnvVarView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return nil, err
	}
	current, err := s.getGroupMap(ctx, m.workspace, envPath(gid))
	if err != nil {
		return nil, err
	}
	desired := make(map[string]EnvVarView, len(vars))
	for _, v := range vars {
		key := strings.TrimSpace(v.Key)
		if !core.ValidEnvKey(key) {
			return nil, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		v.Key = key
		desired[key] = v // retain replace-all's historical last-declaration-wins rule
	}
	writes := make([]EnvVarPatch, 0, len(current)+len(desired))
	for _, key := range slices.Sorted(maps.Keys(current)) {
		if _, keep := desired[key]; !keep {
			writes = append(writes, EnvVarPatch{Key: key, Delete: true})
		}
	}
	for _, key := range slices.Sorted(maps.Keys(desired)) {
		v := desired[key]
		writes = append(writes, EnvVarPatch{Key: key, Value: v.Value, ValueSet: v.ValueSet, GenerateValue: v.GenerateValue})
	}
	result, err := s.patchEnvironmentAuthorized(ctx, gid, m, EnvironmentPatch{EnvVars: writes, SaveMode: SaveModeDeploy})
	if err != nil {
		return nil, err
	}
	return envKeyViewsFromKeys(result.EnvVarKeys), nil
}

// GetEnvGroupVar reveals one variable's value (sensitive read).
func (s *Service) GetEnvGroupVar(ctx context.Context, gid, key string) (EnvVarView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanViewSensitive, gid)
	if err != nil {
		return EnvVarView{}, err
	}
	// codex round-8 #8: per-key reveal — re-assert uncached against the group's
	// owning workspace (the object authorizeGroup's AuthorizeLabeled effectively
	// checked) so a revocation inside PositiveTTL cannot reveal one last value.
	// A legacy group with no recorded workspace re-asserts against the acting
	// workspace, mirroring the cached path's fallback.
	if m.workspace != "" {
		if err := s.AuthorizeFreshOn(ctx, core.RelCanViewSensitive, core.WorkspaceObject(m.workspace)); err != nil {
			return EnvVarView{}, err
		}
	} else if err := s.AuthorizeFresh(ctx, core.RelCanViewSensitive); err != nil {
		return EnvVarView{}, err
	}
	env, err := s.getGroupMap(ctx, m.workspace, envPath(gid))
	if err != nil {
		return EnvVarView{}, err
	}
	v, ok := env[key]
	if !ok {
		return EnvVarView{}, core.ErrNotFound
	}
	return EnvVarView{Key: key, Value: v, ValueSet: true}, nil
}

// SetEnvGroupVar adds or updates one variable while preserving every other key,
// then re-materializes the group's env Secret and rolls linked services. Manage
// scope. This is Render's per-key PUT semantics and means clients never need to
// reveal and resubmit the group's other values.
func (s *Service) SetEnvGroupVar(ctx context.Context, gid, key, value string) (EnvVarView, error) {
	return s.SetEnvGroupVarInput(ctx, gid, EnvVarView{Key: key, Value: value, ValueSet: true})
}

// SetEnvGroupVarInput is the literal-or-generated form of SetEnvGroupVar used
// by every public adapter. SetEnvGroupVar remains as the literal compatibility
// seam for existing internal callers.
func (s *Service) SetEnvGroupVarInput(ctx context.Context, gid string, input EnvVarView) (EnvVarView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return EnvVarView{}, err
	}
	key := strings.TrimSpace(input.Key)
	if !core.ValidEnvKey(key) {
		return EnvVarView{}, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
	}
	_, err = s.patchEnvironmentAuthorized(ctx, gid, m, EnvironmentPatch{
		EnvVars:  []EnvVarPatch{{Key: key, Value: input.Value, ValueSet: input.ValueSet, GenerateValue: input.GenerateValue}},
		SaveMode: SaveModeDeploy,
	})
	if err != nil {
		return EnvVarView{}, err
	}
	return EnvVarView{Key: key}, nil
}

// DeleteEnvGroupVar removes one variable and rolls every linked service. Manage
// scope.
func (s *Service) DeleteEnvGroupVar(ctx context.Context, gid, key string) error {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	env, err := s.getGroupMap(ctx, m.workspace, envPath(gid))
	if err != nil {
		return err
	}
	if _, ok := env[key]; !ok {
		return core.ErrNotFound
	}
	_, err = s.patchEnvironmentAuthorized(ctx, gid, m, EnvironmentPatch{
		EnvVars: []EnvVarPatch{{Key: key, Delete: true}}, SaveMode: SaveModeDeploy,
	})
	return err
}

// --- group contents (secret files) --------------------------------------------

// SetEnvGroupFile adds or updates one group secret file (merged), re-materializes
// the group's files Secret, and rolls every linked service. Manage scope.
func (s *Service) SetEnvGroupFile(ctx context.Context, gid, name, content string) (SecretFileView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return SecretFileView{}, err
	}
	name = strings.TrimSpace(name)
	if !core.ValidSecretFileName(name) {
		return SecretFileView{}, fmt.Errorf("%w: invalid secret file name %q", core.ErrBadRequest, name)
	}
	_, err = s.patchEnvironmentAuthorized(ctx, gid, m, EnvironmentPatch{
		SecretFiles: []SecretFilePatch{{Name: name, Content: content}}, SaveMode: SaveModeDeploy,
	})
	if err != nil {
		return SecretFileView{}, err
	}
	return SecretFileView{Name: name}, nil
}

// DeleteEnvGroupFile removes one group secret file (re-materializing the reduced
// set) and rolls linked services. Manage scope.
func (s *Service) DeleteEnvGroupFile(ctx context.Context, gid, name string) error {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	files, err := s.getGroupMap(ctx, m.workspace, filesPath(gid))
	if err != nil {
		return err
	}
	if _, ok := files[name]; !ok {
		return core.ErrNotFound
	}
	_, err = s.patchEnvironmentAuthorized(ctx, gid, m, EnvironmentPatch{
		SecretFiles: []SecretFilePatch{{Name: name, Delete: true}}, SaveMode: SaveModeDeploy,
	})
	return err
}

// GetEnvGroupFile reveals one file's content (sensitive read).
func (s *Service) GetEnvGroupFile(ctx context.Context, gid, name string) (SecretFileView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanViewSensitive, gid)
	if err != nil {
		return SecretFileView{}, err
	}
	// codex round-9 #7: file content is secret reveal — the same uncached
	// reassertion GetEnvGroupValue (round-8 #8) applies, so a revocation
	// inside PositiveTTL cannot ride a cached positive to one last read.
	// A legacy group with no recorded workspace re-asserts against the acting
	// workspace, mirroring the cached path's fallback.
	if m.workspace != "" {
		if err := s.AuthorizeFreshOn(ctx, core.RelCanViewSensitive, core.WorkspaceObject(m.workspace)); err != nil {
			return SecretFileView{}, err
		}
	} else if err := s.AuthorizeFresh(ctx, core.RelCanViewSensitive); err != nil {
		return SecretFileView{}, err
	}
	files, err := s.getGroupMap(ctx, m.workspace, filesPath(gid))
	if err != nil {
		return SecretFileView{}, err
	}
	content, ok := files[name]
	if !ok {
		return SecretFileView{}, core.ErrNotFound
	}
	return SecretFileView{Name: name, Content: content}, nil
}

// --- linking ------------------------------------------------------------------

// LinkService links the group to a service: the service's App spec gains the
// group's env + files Secret refs, its pods roll, and the group's link set records
// the service. Idempotent. Manage scope. The group must belong to the SAME
// workspace as the service (w6/m24) — otherwise a caller who can create in their
// own workspace could inject another workspace's group (and its secret values)
// into their own service's Secrets, a write-side variant of the read leak this
// milestone closes; refused with ErrForbidden, never a silent cross-workspace
// materialization.
func (s *Service) LinkService(ctx context.Context, gid, service string) error {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	return s.linkFetched(ctx, gid, service, a)
}

// linkFetched is LinkService's post-authorize body, shared with LinkEnvGroup (the
// blueprint seam) so the two link paths authorize + audit exactly once each. It
// appends the group's Secret refs to the already-fetched App, rolls it, and
// records the service in the group's link set.
func (s *Service) linkFetched(ctx context.Context, gid, service string, a *appv1alpha1.App) error {
	m, err := s.fetchGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	if a.Labels[core.LabelTenant] != m.workspace {
		return core.ErrForbidden
	}
	if err := validateGroupServiceEnvironment(m.environment, service, a.Labels); err != nil {
		return err
	}
	base := client.MergeFrom(a.DeepCopy())
	a.Spec.EnvFromSecrets = addString(a.Spec.EnvFromSecrets, envSecretName(gid))
	a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, filesSecretName(gid))
	a.Spec.RestartedAt = s.now()
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return err
	}
	m.links = addString(m.links, service)
	_, err = s.touch(ctx, gid, m)
	return err
}

// UnlinkService reverses LinkService: drop the group's Secret refs from the
// service, roll it, and remove it from the group's link set. Idempotent.
func (s *Service) UnlinkService(ctx context.Context, gid, service string) error {
	// Authorize+fetch against the service's OWN workspace (w6/m17) — reused
	// below via detachFetched, so this is the only fetch of `service` UnlinkService
	// makes. detach (DeleteEnvGroup's bulk-unlink path over every linked service,
	// which authorizes once for the GROUP, not per service) still does its own
	// bare GetApp: it must not fan out into one audit event per linked service.
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	m, err := s.fetchGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	if err := s.detachFetched(ctx, gid, a); err != nil {
		return err
	}
	m.links = removeString(m.links, service)
	_, err = s.touch(ctx, gid, m)
	return err
}

// detach removes the group's Secret refs from a service and rolls it, tolerating a
// service that no longer exists (a deleted service simply drops from the group).
func (s *Service) detach(ctx context.Context, gid, service string) error {
	a, err := s.GetApp(ctx, core.RelCanCreate, service)
	if errors.Is(err, core.ErrNotFound) {
		return nil // a since-deleted service just drops from the group
	}
	if err != nil {
		return err
	}
	return s.detachFetched(ctx, gid, a)
}

// detachFetched is detach's second half, for a caller (UnlinkService) that
// already holds the App it authorized — reusing it rather than fetching (and
// authorizing, and auditing) a second time.
func (s *Service) detachFetched(ctx context.Context, gid string, a *appv1alpha1.App) error {
	base := client.MergeFrom(a.DeepCopy())
	a.Spec.EnvFromSecrets = removeString(a.Spec.EnvFromSecrets, envSecretName(gid))
	a.Spec.FilesFromSecrets = removeString(a.Spec.FilesFromSecrets, filesSecretName(gid))
	a.Spec.RestartedAt = s.now()
	return s.Client.Patch(ctx, a, base)
}

// rollLinked bumps spec.restartedAt on every linked service so it picks up the
// group's changed Secret data (the Secret refs are already on the spec from the
// link). A since-deleted linked service is skipped.
func (s *Service) rollLinked(ctx context.Context, links []string) error {
	stamp := s.now()
	for _, svc := range links {
		if err := s.rollOne(ctx, svc, stamp); err != nil {
			if errors.Is(err, core.ErrNotFound) {
				continue // a since-deleted linked service is skipped
			}
			return err
		}
	}
	return nil
}

func (s *Service) rollOne(ctx context.Context, service, stamp string) error {
	a, err := s.GetApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return err
	}
	base := client.MergeFrom(a.DeepCopy())
	a.Spec.RestartedAt = stamp
	return s.Client.Patch(ctx, a, base)
}

func validateGroupServiceEnvironment(groupEnvironment, serviceID string, labels map[string]string) error {
	serviceEnvironment := labels[core.LabelEnvironment]
	if serviceEnvironment == groupEnvironment {
		return nil
	}
	return core.NewConflictError(
		"ENV_GROUP_SERVICE_ENVIRONMENT_MISMATCH",
		"linked services must have the same Environment scope as the environment group",
		map[string]any{
			"serviceId": serviceID, "serviceEnvironmentId": serviceEnvironment,
			"targetEnvironmentId": groupEnvironment,
		},
	)
}

func (s *Service) validateLinkedServiceEnvironments(ctx context.Context, links []string, environmentID string) error {
	var incompatible []string
	for _, serviceID := range links {
		a, err := s.GetApp(ctx, core.RelCanCreate, serviceID)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if a.Labels[core.LabelEnvironment] != environmentID {
			incompatible = append(incompatible, serviceID)
		}
	}
	if len(incompatible) == 0 {
		return nil
	}
	return core.NewConflictError(
		"ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES",
		"move the linked services to the target Environment or unlink them before moving this group",
		map[string]any{"serviceIds": incompatible, "targetEnvironmentId": environmentID},
	)
}

// --- blueprint apply seam (w1/m35) --------------------------------------------

// GroupNames returns the names of the ACTING workspace's existing env groups
// (w6/m24: scoped the same way ListEnvGroups is — a blueprint deploy must not
// see, let alone match against, another workspace's group names). The blueprint
// apply path uses it to pre-flight `fromGroup` references (an unknown group name
// is a per-entry validation error before any resource is written). View scope.
func (s *Service) GroupNames(ctx context.Context) ([]string, error) {
	groups, err := s.GroupIDsByName(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(groups))
	for name := range groups {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// GroupIDsByName returns the acting workspace's non-secret env-group ids keyed
// by name. It is the narrow read seam Blueprint resource inventories and
// current-state plans need; secret values stay behind the reveal verbs.
func (s *Service) GroupIDsByName(ctx context.Context) (map[string]string, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
	}
	scopeTo, scoped := s.boundWorkspace(ctx)
	if scoped && scopeTo == "" {
		return map[string]string{}, nil
	}
	ids, err := s.listGroupIDs(ctx, scopeTo, true)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(ids))
	for _, gid := range ids {
		m, err := s.readMeta(ctx, gid)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if scoped && m.workspace != scopeTo {
			continue // another workspace's group — not this deploy's to see
		}
		if prior, duplicate := out[m.name]; duplicate {
			return nil, envGroupNameAmbiguous(m.name, scopeTo, []string{prior, gid})
		}
		out[m.name] = gid
	}
	return out, nil
}

// ApplyEnvGroup materializes one blueprint `envVarGroups:` entry (w1/m35),
// keyed by NAME (Render's env groups are name-addressed): create the group if no
// group of that name exists IN THE ACTING WORKSPACE (w6/m24 — findGroupByName is
// workspace-scoped, so a same-named group in another workspace is never matched,
// reused, or overwritten), else reuse it, then reconcile its vars — literals set
// to their declared value (re-synced each apply), generates minted once
// (core.GenerateValue) and thereafter preserved, so a re-sync never re-mints. Keys
// already in the group but absent from the blueprint are retained (Render's
// preservation rule). Idempotent: when the reconciled set already matches, no
// Secret write and no roll of linked services. Manage scope.
func (s *Service) ApplyEnvGroup(ctx context.Context, name string, literals map[string]string, generates []string) error {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: env group name is required", core.ErrBadRequest)
	}
	gid, m, found, err := s.findGroupByName(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		gid = id.New(id.EnvGroup)
		workspace, _ := s.Tenant(ctx) // "" with the store off, matching CreateEnvGroup
		if err := s.envGroupQuota(ctx, workspace); err != nil {
			return err
		}
		if err := s.claimGroupName(ctx, workspace, name, gid); err != nil {
			return err
		}
		now := s.now()
		m = meta{name: name, workspace: workspace, createdAt: now, updatedAt: now}
		inputs := make([]CreateEnvVarInput, 0, len(literals)+len(generates))
		for _, key := range core.SortedKeys(literals) {
			inputs = append(inputs, CreateEnvVarInput{Key: key, Value: literals[key], ValueSet: true})
		}
		for _, key := range generates {
			inputs = append(inputs, CreateEnvVarInput{Key: key, GenerateValue: true})
		}
		env, prepareErr := prepareCreateEnv(inputs)
		if prepareErr != nil {
			_ = s.releaseGroupName(context.WithoutCancel(ctx), workspace, name, gid)
			return prepareErr
		}
		if err := s.persistCreate(ctx, gid, m, env, map[string]string{}, nil); err != nil {
			return errors.Join(err, s.releaseGroupName(context.WithoutCancel(ctx), workspace, name, gid))
		}
		return nil
	}
	env, err := s.getGroupMap(ctx, m.workspace, envPath(gid))
	if err != nil {
		return err
	}
	writes := make([]EnvVarPatch, 0, len(literals)+len(generates))
	for _, key := range core.SortedKeys(literals) {
		if !core.ValidEnvKey(key) {
			return fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		if env[key] != literals[key] {
			writes = append(writes, EnvVarPatch{Key: key, Value: literals[key], ValueSet: true})
		}
	}
	for _, key := range generates {
		if !core.ValidEnvKey(key) {
			return fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		if _, ok := env[key]; ok {
			continue // generate-once: an existing value persists across syncs
		}
		writes = append(writes, EnvVarPatch{Key: key, GenerateValue: true})
	}
	if len(writes) == 0 {
		return nil
	}
	_, err = s.patchEnvironmentAuthorized(ctx, gid, m, EnvironmentPatch{EnvVars: writes, SaveMode: SaveModeDeploy})
	return err
}

// LinkEnvGroup links the named group to a service for the blueprint apply path
// (w1/m35's `fromGroup`). Unknown group name is an error (the apply pre-flights
// existence, but this guards the direct call too) — findGroupByName's workspace
// scoping (w6/m24) means a same-named group in another workspace reports as
// unknown here, not linked. Idempotent: an already-linked service is not
// re-patched, so a stack re-apply neither churns the spec nor rolls the pod.
// Manage scope (via LinkService's AuthorizeApp).
func (s *Service) LinkEnvGroup(ctx context.Context, name, service string) error {
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service) // authorize (+ audit) FIRST
	if err != nil {
		return err
	}
	if s.Store == nil {
		return core.ErrSecretsUnavailable
	}
	gid, m, found, err := s.findGroupByName(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: env group %q does not exist", core.ErrBadRequest, name)
	}
	// Skip the patch (and the roll) when the service is already linked — a re-link
	// would bump restartedAt and break stack re-apply idempotency.
	for _, svc := range m.links {
		if svc == service {
			return nil
		}
	}
	return s.linkFetched(ctx, gid, service, a)
}

// findGroupByName resolves an env group by its display name WITHIN THE ACTING
// WORKSPACE (Render addresses groups by name; bex stores them by id). w6/m24:
// scoped the same way ListEnvGroups is — a name search must never match, and so
// never reuse or overwrite, another workspace's same-named group; with the
// control-plane store off there is only one workspace, so the search stays
// unscoped (byte-identical to before). Returns found=false when no group of that
// name exists in scope. Legacy duplicates fail closed: choosing one by sorted id
// can bind a Blueprint to the wrong secret set.
func (s *Service) findGroupByName(ctx context.Context, name string) (string, meta, bool, error) {
	scopeTo, scoped := s.boundWorkspace(ctx)
	if scoped && scopeTo == "" {
		return "", meta{}, false, nil
	}
	ids, err := s.listGroupIDs(ctx, scopeTo, true)
	if err != nil {
		return "", meta{}, false, err
	}
	sort.Strings(ids)
	var matches []scopedMeta
	for _, gid := range ids {
		m, err := s.readMeta(ctx, gid)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", meta{}, false, err
		}
		if scoped && m.workspace != scopeTo {
			continue // another workspace's group — never matched by name search
		}
		if m.name == name {
			matches = append(matches, scopedMeta{id: gid, meta: m})
		}
	}
	if len(matches) == 0 {
		return "", meta{}, false, nil
	}
	if len(matches) > 1 {
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			ids = append(ids, match.id)
		}
		return "", meta{}, false, envGroupNameAmbiguous(name, scopeTo, ids)
	}
	return matches[0].id, matches[0].meta, true, nil
}

const envGroupNameClaimAttempts = 4

func envGroupNameClaimPath(workspace, name string) string {
	digest := sha256.Sum256([]byte(workspace + "\x00" + name))
	return fmt.Sprintf("env-group-name-claims/%x", digest[:])
}

func envGroupNameExists(name, workspace string) error {
	return core.NewConflictError(
		"ENV_GROUP_NAME_EXISTS",
		"Environment group name already exists.",
		map[string]any{"name": name, "workspaceId": workspace},
	)
}

func envGroupNameAmbiguous(name, workspace string, ids []string) error {
	return core.NewConflictError(
		"ENV_GROUP_NAME_AMBIGUOUS",
		"Multiple environment groups use this name; rename the duplicates by id before retrying.",
		map[string]any{"name": name, "workspaceId": workspace, "ids": ids},
	)
}

// claimGroupName combines a fresh legacy-metadata sweep with a CAS-backed name
// index. The sweep catches pre-index groups; CAS makes concurrent creates and
// renames across bex-api replicas resolve to exactly one winner.
func (s *Service) claimGroupName(ctx context.Context, workspace, name, gid string) error {
	matches, err := s.groupsNamed(ctx, workspace, name, gid)
	if err != nil {
		return err
	}
	if len(matches) > 0 {
		return envGroupNameExists(name, workspace)
	}
	path := envGroupNameClaimPath(workspace, name)
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return core.ErrSecretsUnavailable
	}
	for range envGroupNameClaimAttempts {
		snapshot, err := versioned.GetVersioned(groupCtx(ctx, workspace), path)
		if err != nil {
			return err
		}
		if owner := snapshot.Data["id"]; owner != "" && owner != gid {
			return envGroupNameExists(name, workspace)
		}
		if _, err := versioned.PutCAS(groupCtx(ctx, workspace), path, map[string]string{
			"id": gid, "name": name, "workspace": workspace,
		}, snapshot.Version); err == nil {
			return nil
		} else if !errors.Is(err, core.ErrConflict) {
			return err
		}
	}
	return envGroupNameExists(name, workspace)
}

func (s *Service) releaseGroupName(ctx context.Context, workspace, name, gid string) error {
	path := envGroupNameClaimPath(workspace, name)
	versioned, ok := s.Store.(core.VersionedSecretKV)
	if !ok {
		return core.ErrSecretsUnavailable
	}
	for range envGroupNameClaimAttempts {
		snapshot, err := versioned.GetVersioned(groupCtx(ctx, workspace), path)
		if err != nil || snapshot.Data["id"] == "" || snapshot.Data["id"] != gid {
			return err
		}
		if _, err := versioned.PutCAS(groupCtx(ctx, workspace), path, map[string]string{}, snapshot.Version); err == nil {
			return nil
		} else if !errors.Is(err, core.ErrConflict) {
			return err
		}
	}
	return core.ErrConflict
}

// groupsNamed combines a fresh workspace-prefix-scoped sweep (unioned with the
// legacy tenant for the dual-read migration window) with the CAS-backed name
// index above: the sweep catches pre-index groups; CAS makes concurrent
// creates and renames across bex-api replicas resolve to exactly one winner.
func (s *Service) groupsNamed(ctx context.Context, workspace, name, excludeID string) ([]string, error) {
	ids, err := s.listGroupIDs(ctx, workspace, true)
	if err != nil {
		return nil, err
	}
	sort.Strings(ids)
	matches := make([]string, 0, 1)
	for _, gid := range ids {
		if gid == excludeID {
			continue
		}
		m, err := s.readMeta(ctx, gid)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if m.name == name && (s.Workspace == nil || m.workspace == workspace) {
			matches = append(matches, gid)
		}
	}
	return matches, nil
}

// --- authorization helpers -----------------------------------------------------

// authorizeGroup gates a verb scoped to ONE named group: the caller's own
// workspace first (the single audit point every bare-Authorize verb has always
// had), then the group's OWN workspace via fetchGroup — the AuthorizeApp/
// AuthorizeLabeled pattern (w6/m17) applied to a KV-backed resource that has no
// CRD to fetch through a single seam. A caller who is admin of their own
// workspace but merely a viewer of (or not a member of) the group's actual
// owner may not read/reveal/mutate it just because they know its id.
func (s *Service) authorizeGroup(ctx context.Context, relation, gid string) (meta, error) {
	if err := s.Authorize(ctx, relation); err != nil {
		return meta{}, err
	}
	return s.fetchGroup(ctx, relation, gid)
}

// fetchGroup is authorizeGroup's second half, split out for LinkService/
// UnlinkService: they already authorized (and audited) their primary resource —
// the App, via AuthorizeApp — and only need the group-side cross-workspace check
// added, never a second bare Authorize call (which would double the audit event
// TestAuditCoversEveryWriteVerbExactlyOnce guards against).
func (s *Service) fetchGroup(ctx context.Context, relation, gid string) (meta, error) {
	if s.Store == nil {
		return meta{}, core.ErrSecretsUnavailable
	}
	m, err := s.requireGroup(ctx, gid)
	if err != nil {
		return meta{}, err
	}
	if err := s.AuthorizeLabeled(ctx, relation, map[string]string{core.LabelTenant: m.workspace}); err != nil {
		return meta{}, err
	}
	return m, nil
}

// boundWorkspace resolves the workspace ListEnvGroups/GroupNames/findGroupByName
// scope to. scoped=false means the control-plane store is off (Workspace nil) —
// groups aren't attributed to distinct real workspaces, so these stay unfiltered;
// scoped=true with an empty workspace means the caller has no resolvable tenant —
// scoped to nothing. Mirrors apikeys.boundTenant / core.Base.Tenant's own "check
// Workspace != nil first" contract.
func (s *Service) boundWorkspace(ctx context.Context) (workspace string, scoped bool) {
	if s.Workspace == nil {
		return "", false
	}
	workspace, _ = s.Tenant(ctx)
	return workspace, true
}

// --- store + secret helpers ---------------------------------------------------

// touch bumps a group's updatedAt and persists its meta — called by every write
// verb (identity changes AND content changes alike), so a client polling
// updatedAt observes every mutation, not only renames.
func (s *Service) touch(ctx context.Context, gid string, m meta) (meta, error) {
	m.updatedAt = s.now()
	if err := s.writeMeta(ctx, gid, m); err != nil {
		return meta{}, err
	}
	return m, nil
}

// viewFromMeta builds the (secret-free) view of a group from its already-fetched
// meta plus contents — the second half of view lookups that already hold meta
// (authorizeGroup/ListEnvGroups), so it's never re-read.
func (s *Service) viewFromMeta(ctx context.Context, gid string, m meta) (EnvGroupView, error) {
	env, err := s.getGroupMap(ctx, m.workspace, envPath(gid))
	if err != nil {
		return EnvGroupView{}, err
	}
	files, err := s.getGroupMap(ctx, m.workspace, filesPath(gid))
	if err != nil {
		return EnvGroupView{}, err
	}
	links := m.links
	if links == nil {
		links = []string{}
	}
	revision := ""
	if versioned, ok := s.Store.(core.VersionedSecretKV); ok {
		snapshot, revisionErr := s.getRevisionSnapshot(ctx, versioned, m.workspace, gid)
		if revisionErr != nil {
			return EnvGroupView{}, core.ErrSecretsUnavailable
		}
		if snapshot.Data["state"] == "repair_required" {
			return EnvGroupView{}, envGroupRestorationFailed()
		}
		if snapshot.Data["state"] == "busy" {
			return EnvGroupView{}, envGroupRevisionConflict()
		}
		revision = encodeEnvGroupRevision(revisionGeneration(snapshot.Data))
	}
	return EnvGroupView{
		ID:            gid,
		Name:          m.name,
		OwnerID:       m.workspace,
		EnvironmentID: m.environment,
		ServiceLinks:  links,
		EnvVars:       envKeyViews(env),
		SecretFiles:   fileNameViews(files),
		CreatedAt:     m.createdAt,
		UpdatedAt:     m.updatedAt,
		Revision:      revision,
	}, nil
}

// requireGroup returns a group's meta or core.ErrNotFound when it doesn't exist.
func (s *Service) requireGroup(ctx context.Context, gid string) (meta, error) {
	if !id.WellFormed(gid) {
		return meta{}, core.ErrNotFound
	}
	return s.readMeta(ctx, gid)
}

// readMeta migrates a pre-attribution group in place (w6/m24), unlike
// secrets/github's DefaultTenant fallback (a per-call key derivation for an
// unresolved CALLER, never persisted): a group with no stored workspace, read
// back once the control-plane store (real multi-tenancy) is live, is assigned
// core.DefaultTenant as its OWNER and that assignment is written back — a
// genuine one-time data migration, lazy and idempotent (it only fires once per
// group, since the write leaves workspace non-empty), so it happens
// deterministically and is never a silently stranded, ownerless group. In
// store-off single-tenant mode (Workspace == nil) there is only one workspace
// regardless, so this is left alone — matching AppView.OwnerID's own "never
// fake it" convention.
//
// w2/m80: a group's meta lives at the legacy tenant only until it is either
// created directly under a workspace tenant or explicitly moved there
// (MigratePaths); a bare-gid lookup has no workspace to scope by, so it always
// starts at the legacy tenant. What it finds there is one of three shapes:
// the group's still-full legacy meta (unmigrated — used directly, the
// dual-read case), a thin locator (writeMetaLocator/MigratePaths — the real
// meta is fetched from its workspace tenant), or nothing (never existed, or a
// create's meta write is still in flight — core.ErrNotFound either way).
func (s *Service) readMeta(ctx context.Context, gid string) (meta, error) {
	raw, err := s.Store.Get(legacyCtx(ctx), metaPath(gid))
	if err != nil {
		return meta{}, err
	}
	if len(raw) == 0 {
		// Rare: a group's full meta lives ONLY at its workspace tenant with no
		// legacy locator (a locator write that never landed). The caller's own
		// bound workspace is the one hint available for a direct retry.
		if hint, scoped := s.boundWorkspace(ctx); scoped && hint != "" {
			raw, err = s.Store.Get(groupCtx(ctx, hint), metaPath(gid))
			if err != nil {
				return meta{}, err
			}
		}
		if len(raw) == 0 {
			return meta{}, core.ErrNotFound
		}
		return decodeMeta(raw), nil
	}
	if isLocator(raw) {
		workspace := raw["workspace"]
		if workspace == "" {
			return meta{}, core.ErrNotFound
		}
		full, err := s.Store.Get(groupCtx(ctx, workspace), metaPath(gid))
		if err != nil {
			return meta{}, err
		}
		if len(full) == 0 {
			return meta{}, core.ErrNotFound
		}
		return decodeMeta(full), nil
	}
	m := decodeMeta(raw)
	if m.workspace == "" && s.Workspace != nil {
		m.workspace = core.DefaultTenant
		if err := s.writeMeta(ctx, gid, m); err != nil {
			return meta{}, err
		}
		s.RecordEnvGroupOwnershipMigration(ctx, gid, m.workspace)
	}
	return m, nil
}

func decodeMeta(raw map[string]string) meta {
	m := meta{
		name:        raw["name"],
		workspace:   raw["workspace"],
		environment: raw["environment"],
		createdAt:   raw["createdAt"],
		updatedAt:   raw["updatedAt"],
	}
	if l := strings.TrimSpace(raw["links"]); l != "" {
		m.links = strings.Split(l, ",")
	}
	return m
}

// writeMeta persists a group's full metadata under its OWN workspace tenant
// (w2/m80) — never the shared legacy tenant, once that workspace is a real,
// non-legacy one — and, for that case, republishes the thin legacy-tenant
// locator so a bare-gid lookup (readMeta, GetEnvGroup, the SSH/Blueprint
// seams) keeps finding it. A store-off/unattributed group (m.workspace == "")
// and the lazily-attributed core.DefaultTenant both already ARE the legacy
// tenant (groupCtx(ctx, "") == legacyCtx(ctx)); writing a locator on top of
// that single copy would clobber the meta it names, so the locator write is
// skipped precisely in that case.
func (s *Service) writeMeta(ctx context.Context, gid string, m meta) error {
	data := map[string]string{
		"name":        m.name,
		"links":       strings.Join(m.links, ","),
		"workspace":   m.workspace,
		"environment": m.environment,
		"createdAt":   m.createdAt,
		"updatedAt":   m.updatedAt,
	}
	if err := s.Store.Put(groupCtx(ctx, m.workspace), metaPath(gid), data); err != nil {
		return err
	}
	if m.workspace != "" && m.workspace != secrets.LegacyTenant {
		return s.writeMetaLocator(ctx, gid, m.workspace)
	}
	return nil
}

// envGroupQuota refuses a create that would push workspace past its env-group
// cap. MaxEnvGroups > 0 enables (production wires the env default of 100; 0
// leaves tests and store-off mode uncapped).
func (s *Service) envGroupQuota(ctx context.Context, workspace string) error {
	if s.MaxEnvGroups <= 0 {
		return nil
	}
	ids, err := s.listGroupIDs(ctx, workspace, true)
	if err != nil {
		return err
	}
	count := 0
	for _, gid := range ids {
		m, err := s.readMeta(ctx, gid)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if m.workspace == workspace {
			count++
		}
	}
	if count >= s.MaxEnvGroups {
		return core.NewConflictError("ENV_GROUP_LIMIT",
			fmt.Sprintf("workspace already owns %d environment groups (limit %d); delete unused groups or raise the limit", count, s.MaxEnvGroups),
			map[string]any{"count": count, "limit": s.MaxEnvGroups})
	}
	return nil
}

// validateEnvironment proves that an optional Environment belongs to the same
// workspace as the group. Cross-workspace association is always forbidden; an
// unwired Environments service reports the control-plane dependency honestly.
func (s *Service) validateEnvironment(ctx context.Context, environmentID, workspace string) error {
	if environmentID == "" {
		return nil
	}
	if s.EnvironmentWorkspace == nil {
		return core.ErrWorkspacesUnavailable
	}
	owner, err := s.EnvironmentWorkspace(ctx, environmentID)
	if err != nil {
		return err
	}
	if owner != workspace {
		return core.ErrForbidden
	}
	return nil
}

// upsertSecret creates or replaces a group projection Secret with exactly the
// given data. Group Secrets have no App owner (a group outlives any one service),
// so DeleteEnvGroup removes them explicitly. The workspace is the group's OWNER
// (m.workspace from fetchGroup/authorizeGroup), never the caller's resolved
// tenant: an ID-scoped verb authorizes against the owning workspace but leaves
// the caller's named-or-default workspace in ctx, so deriving the namespace from
// ctx would write the owner's secret material into the caller's namespace.
func (s *Service) upsertSecret(ctx context.Context, workspace, name string, data map[string]string) error {
	bytesData := make(map[string][]byte, len(data))
	for k, v := range data {
		bytesData[k] = []byte(v)
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.envGroupNamespace(workspace)}}
	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, sec, func() error {
		sec.Type = corev1.SecretTypeOpaque
		sec.Data = bytesData
		return nil
	})
	return err
}

// deleteSecret removes one projection Secret from the group's OWNING workspace
// (same owner-workspace rule as upsertSecret).
func (s *Service) deleteSecret(ctx context.Context, workspace, name string) error {
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.envGroupNamespace(workspace)}}
	return client.IgnoreNotFound(s.Client.Delete(ctx, sec))
}

// envGroupNamespace is where a group's projection Secrets live: the OWNING
// workspace's own namespace under per-tenant isolation (ADR043), so the linked
// services' pods (all in that one namespace) resolve them via envFrom /
// projected volumes. Empty workspace (store off / unattributed) => the shared
// s.Namespace.
func (s *Service) envGroupNamespace(workspace string) string {
	return s.AppNamespace(workspace)
}

// now is the rolling-restart stamp (RFC3339Nano so back-to-back edits differ).
func (s *Service) now() string { return s.Now().UTC().Format(time.RFC3339Nano) }

// --- view + validation helpers ------------------------------------------------

func envViews(env map[string]string) []EnvVarView {
	out := make([]EnvVarView, 0, len(env))
	for k, v := range env {
		out = append(out, EnvVarView{Key: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func envKeyViews(env map[string]string) []EnvVarView {
	return envKeyViewsFromKeys(slices.Sorted(maps.Keys(env)))
}

func envKeyViewsFromKeys(keys []string) []EnvVarView {
	out := make([]EnvVarView, 0, len(keys))
	for _, key := range keys {
		out = append(out, EnvVarView{Key: key})
	}
	return out
}

func fileNameViews(files map[string]string) []SecretFileView {
	out := make([]SecretFileView, 0, len(files))
	for name := range files {
		out = append(out, SecretFileView{Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func addString(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// removeString returns list without any occurrence of s. It clones rather than
// compacting in place so no caller's retained slice is mutated behind its back
// (the same contract secrets.removeString keeps).
func removeString(list []string, s string) []string {
	return slices.DeleteFunc(slices.Clone(list), func(v string) bool { return v == s })
}
