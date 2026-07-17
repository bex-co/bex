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
	"errors"
	"fmt"
	"reflect"
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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

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
	// Selections is the shared MCP per-session workspace selection (w6/m24,
	// mirroring apps/postgres/keyvalue/apikeys, core.WorkspaceSelectionReader):
	// list_env_groups'/create_env_group's ownerId precedence (explicit arg > the
	// session's select_workspace > the caller's default). nil degrades to
	// explicit-arg-or-default.
	Selections core.WorkspaceSelectionReader
}

// EnvVarView is a group env var ({key, value}); value is empty in list/get
// responses (fetched per key), present only on the sensitive single-var read.
type EnvVarView struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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
	GenerateValue bool   `json:"generateValue,omitempty"`
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
		matchesTimeWindow(group.createdAt, filter.CreatedBefore, filter.CreatedAfter) &&
		matchesTimeWindow(group.updatedAt, filter.UpdatedBefore, filter.UpdatedAfter)
}

func matchesTimeWindow(raw string, before, after time.Time) bool {
	if before.IsZero() && after.IsZero() {
		return true
	}
	// Legacy groups pre-dating w6/m24's timestamp stamping have an empty raw
	// string. We can't place them in a time window, so we include them rather
	// than silently dropping them — omitted data doesn't imply exclusion.
	if raw == "" {
		return true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	return (before.IsZero() || value.Before(before)) && (after.IsZero() || value.After(after))
}

// pageEnvGroups is the shared GraphQL/MCP/REST paging rule. Callers pass
// requested=false when neither cursor nor limit was supplied, preserving the
// historical complete-list behavior; an explicit page is stable by group id.
func pageEnvGroups(groups []EnvGroupView, cursor string, limit int, requested bool) []EnvGroupView {
	if limit == 0 {
		limit = core.DefaultPageLimit
	} else {
		limit = core.PageLimit(limit)
	}
	return core.StablePage(groups, cursor, limit, requested, func(group EnvGroupView) string { return group.ID })
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
// maps while preserving one scoping implementation.
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
	ids, err := s.Store.List(ctx, "env-groups")
	if err != nil {
		return nil, err
	}
	sort.Strings(ids)
	out := make([]scopedMeta, 0, len(ids))
	for _, gid := range ids {
		m, err := s.readMeta(ctx, gid)
		if err != nil {
			return nil, err
		}
		if scoped && m.workspace != scopeTo {
			continue
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
	env, err := prepareCreateEnv(req.EnvVars)
	if err != nil {
		return EnvGroupView{}, err
	}
	files, err := prepareCreateFiles(req.SecretFiles)
	if err != nil {
		return EnvGroupView{}, err
	}
	services, links, err := s.prepareCreateServices(ctx, req.ServiceIDs, workspace)
	if err != nil {
		return EnvGroupView{}, err
	}

	gid := id.New(id.EnvGroup)
	now := s.now()
	m := meta{
		name: name, links: links, workspace: workspace, environment: req.EnvironmentID,
		createdAt: now, updatedAt: now,
	}
	if err := s.persistCreate(ctx, gid, m, env, files, services); err != nil {
		return EnvGroupView{}, err
	}
	return EnvGroupView{
		ID: gid, Name: name, OwnerID: workspace, EnvironmentID: req.EnvironmentID,
		ServiceLinks: links, EnvVars: envKeyViews(env), SecretFiles: fileNameViews(files),
		CreatedAt: now, UpdatedAt: now,
	}, nil
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
		if input.GenerateValue && input.Value != "" {
			return nil, fmt.Errorf("%w: env var %q sets both value and generateValue — pick one", core.ErrBadRequest, key)
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
			if err := s.deleteSecret(cleanupCtx, name); err != nil {
				cleanup = append(cleanup, fmt.Errorf("delete projection Secret %q: %w", name, err))
			}
		}
		for _, path := range []string{envPath(gid), filesPath(gid), metaPath(gid)} {
			if err := s.Store.Delete(cleanupCtx, path); err != nil {
				cleanup = append(cleanup, fmt.Errorf("delete %q: %w", path, err))
			}
		}
		if len(cleanup) == 0 {
			return cause
		}
		return errors.Join(append([]error{cause}, cleanup...)...)
	}

	if err := s.storeMap(ctx, envPath(gid), env); err != nil {
		return rollback(err)
	}
	if err := s.storeMap(ctx, filesPath(gid), files); err != nil {
		return rollback(err)
	}
	if err := s.upsertSecret(ctx, envSecretName(gid), env); err != nil {
		return rollback(err)
	}
	if err := s.upsertSecret(ctx, filesSecretName(gid), files); err != nil {
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
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	if err := s.validateEnvironment(ctx, environmentID, m.workspace); err != nil {
		return err
	}
	if m.environment == environmentID {
		return nil
	}
	m.environment = environmentID
	_, err = s.touch(ctx, gid, m)
	return err
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
	m.name = name
	if m, err = s.touch(ctx, gid, m); err != nil {
		return EnvGroupView{}, err
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
	// Detach from linked services first (drop the spec refs + roll) so no pod is
	// left referencing a Secret about to be deleted.
	for _, svc := range m.links {
		if err := s.detach(ctx, gid, svc); err != nil {
			return err
		}
	}
	if err := s.deleteSecret(ctx, envSecretName(gid)); err != nil {
		return err
	}
	if err := s.deleteSecret(ctx, filesSecretName(gid)); err != nil {
		return err
	}
	for _, p := range []string{envPath(gid), filesPath(gid), metaPath(gid)} {
		if err := s.Store.Delete(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// --- group contents (env vars) ------------------------------------------------

// SetEnvGroupVars replaces the group's whole env set, re-materializes the group's
// env Secret, and rolls every linked service. Manage scope.
func (s *Service) SetEnvGroupVars(ctx context.Context, gid string, vars []EnvVarView) ([]EnvVarView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return nil, err
	}
	env := make(map[string]string, len(vars))
	for _, v := range vars {
		key := strings.TrimSpace(v.Key)
		if !core.ValidEnvKey(key) {
			return nil, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		env[key] = v.Value
	}
	if err := s.storeMap(ctx, envPath(gid), env); err != nil {
		return nil, err
	}
	if err := s.upsertSecret(ctx, envSecretName(gid), env); err != nil {
		return nil, err
	}
	if m, err = s.touch(ctx, gid, m); err != nil {
		return nil, err
	}
	if err := s.rollLinked(ctx, m.links); err != nil {
		return nil, err
	}
	return envViews(env), nil
}

// GetEnvGroupVar reveals one variable's value (sensitive read).
func (s *Service) GetEnvGroupVar(ctx context.Context, gid, key string) (EnvVarView, error) {
	if _, err := s.authorizeGroup(ctx, core.RelCanViewSensitive, gid); err != nil {
		return EnvVarView{}, err
	}
	env, err := s.Store.Get(ctx, envPath(gid))
	if err != nil {
		return EnvVarView{}, err
	}
	v, ok := env[key]
	if !ok {
		return EnvVarView{}, core.ErrNotFound
	}
	return EnvVarView{Key: key, Value: v}, nil
}

// SetEnvGroupVar adds or updates one variable while preserving every other key,
// then re-materializes the group's env Secret and rolls linked services. Manage
// scope. This is Render's per-key PUT semantics and means clients never need to
// reveal and resubmit the group's other values.
func (s *Service) SetEnvGroupVar(ctx context.Context, gid, key, value string) (EnvVarView, error) {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return EnvVarView{}, err
	}
	key = strings.TrimSpace(key)
	if !core.ValidEnvKey(key) {
		return EnvVarView{}, fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
	}
	env, err := s.Store.Get(ctx, envPath(gid))
	if err != nil {
		return EnvVarView{}, err
	}
	env[key] = value
	if err := s.storeMap(ctx, envPath(gid), env); err != nil {
		return EnvVarView{}, err
	}
	if err := s.upsertSecret(ctx, envSecretName(gid), env); err != nil {
		return EnvVarView{}, err
	}
	if m, err = s.touch(ctx, gid, m); err != nil {
		return EnvVarView{}, err
	}
	if err := s.rollLinked(ctx, m.links); err != nil {
		return EnvVarView{}, err
	}
	return EnvVarView{Key: key, Value: value}, nil
}

// DeleteEnvGroupVar removes one variable and rolls every linked service. Manage
// scope.
func (s *Service) DeleteEnvGroupVar(ctx context.Context, gid, key string) error {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	env, err := s.Store.Get(ctx, envPath(gid))
	if err != nil {
		return err
	}
	if _, ok := env[key]; !ok {
		return core.ErrNotFound
	}
	delete(env, key)
	if err := s.storeMap(ctx, envPath(gid), env); err != nil {
		return err
	}
	if err := s.upsertSecret(ctx, envSecretName(gid), env); err != nil {
		return err
	}
	if m, err = s.touch(ctx, gid, m); err != nil {
		return err
	}
	return s.rollLinked(ctx, m.links)
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
	files, err := s.Store.Get(ctx, filesPath(gid))
	if err != nil {
		return SecretFileView{}, err
	}
	files[name] = content
	if err := s.storeMap(ctx, filesPath(gid), files); err != nil {
		return SecretFileView{}, err
	}
	if err := s.upsertSecret(ctx, filesSecretName(gid), files); err != nil {
		return SecretFileView{}, err
	}
	if m, err = s.touch(ctx, gid, m); err != nil {
		return SecretFileView{}, err
	}
	if err := s.rollLinked(ctx, m.links); err != nil {
		return SecretFileView{}, err
	}
	return SecretFileView{Name: name, Content: content}, nil
}

// DeleteEnvGroupFile removes one group secret file (re-materializing the reduced
// set) and rolls linked services. Manage scope.
func (s *Service) DeleteEnvGroupFile(ctx context.Context, gid, name string) error {
	m, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	files, err := s.Store.Get(ctx, filesPath(gid))
	if err != nil {
		return err
	}
	if _, ok := files[name]; !ok {
		return core.ErrNotFound
	}
	delete(files, name)
	if err := s.storeMap(ctx, filesPath(gid), files); err != nil {
		return err
	}
	if err := s.upsertSecret(ctx, filesSecretName(gid), files); err != nil {
		return err
	}
	if m, err = s.touch(ctx, gid, m); err != nil {
		return err
	}
	return s.rollLinked(ctx, m.links)
}

// GetEnvGroupFile reveals one file's content (sensitive read).
func (s *Service) GetEnvGroupFile(ctx context.Context, gid, name string) (SecretFileView, error) {
	if _, err := s.authorizeGroup(ctx, core.RelCanViewSensitive, gid); err != nil {
		return SecretFileView{}, err
	}
	files, err := s.Store.Get(ctx, filesPath(gid))
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
		a, err := s.GetApp(ctx, core.RelCanCreate, svc)
		if errors.Is(err, core.ErrNotFound) {
			continue // a since-deleted linked service is skipped
		}
		if err != nil {
			return err
		}
		base := client.MergeFrom(a.DeepCopy())
		a.Spec.RestartedAt = stamp
		if err := s.Client.Patch(ctx, a, base); err != nil {
			return err
		}
	}
	return nil
}

// --- blueprint apply seam (w1/m35) --------------------------------------------

// GroupNames returns the names of the ACTING workspace's existing env groups
// (w6/m24: scoped the same way ListEnvGroups is — a blueprint deploy must not
// see, let alone match against, another workspace's group names). The blueprint
// apply path uses it to pre-flight `fromGroup` references (an unknown group name
// is a per-entry validation error before any resource is written). View scope.
func (s *Service) GroupNames(ctx context.Context) ([]string, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrSecretsUnavailable
	}
	ids, err := s.Store.List(ctx, "env-groups")
	if err != nil {
		return nil, err
	}
	scopeTo, scoped := s.boundWorkspace(ctx)
	out := make([]string, 0, len(ids))
	for _, gid := range ids {
		m, err := s.readMeta(ctx, gid)
		if err != nil {
			return nil, err
		}
		if scoped && m.workspace != scopeTo {
			continue // another workspace's group — not this deploy's to see
		}
		out = append(out, m.name)
	}
	sort.Strings(out)
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
		now := s.now()
		m = meta{name: name, workspace: workspace, createdAt: now, updatedAt: now}
		if err := s.writeMeta(ctx, gid, m); err != nil {
			return err
		}
	}
	env, err := s.Store.Get(ctx, envPath(gid))
	if err != nil {
		return err
	}
	next := make(map[string]string, len(env))
	for k, v := range env {
		next[k] = v // retain existing keys (preservation rule)
	}
	for _, key := range core.SortedKeys(literals) {
		if !core.ValidEnvKey(key) {
			return fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		next[key] = literals[key] // literals re-sync to the declared value
	}
	for _, key := range generates {
		if !core.ValidEnvKey(key) {
			return fmt.Errorf("%w: invalid environment variable name %q", core.ErrBadRequest, key)
		}
		if _, ok := next[key]; ok {
			continue // generate-once: an existing value persists across syncs
		}
		v, err := core.GenerateValue()
		if err != nil {
			return err
		}
		next[key] = v
	}
	// A freshly created group still needs its (possibly empty) projection Secret so
	// a link can reference it; an existing group with an unchanged set is a no-op.
	if found && reflect.DeepEqual(env, next) {
		return nil
	}
	if err := s.storeMap(ctx, envPath(gid), next); err != nil {
		return err
	}
	if err := s.upsertSecret(ctx, envSecretName(gid), next); err != nil {
		return err
	}
	if !found {
		// A brand-new group also needs its files projection Secret to exist (parity
		// with CreateEnvGroup), so a later files write / delete finds it.
		if err := s.upsertSecret(ctx, filesSecretName(gid), nil); err != nil {
			return err
		}
	} else if _, err := s.touch(ctx, gid, m); err != nil {
		return err
	}
	return s.rollLinked(ctx, m.links)
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
// name exists in scope. The first match wins if names ever collide within one
// workspace (CreateEnvGroup does not enforce uniqueness) — deterministic because
// the id list is sorted.
func (s *Service) findGroupByName(ctx context.Context, name string) (string, meta, bool, error) {
	ids, err := s.Store.List(ctx, "env-groups")
	if err != nil {
		return "", meta{}, false, err
	}
	sort.Strings(ids)
	scopeTo, scoped := s.boundWorkspace(ctx)
	for _, gid := range ids {
		m, err := s.readMeta(ctx, gid)
		if err != nil {
			return "", meta{}, false, err
		}
		if scoped && m.workspace != scopeTo {
			continue // another workspace's group — never matched by name search
		}
		if m.name == name {
			return gid, m, true, nil
		}
	}
	return "", meta{}, false, nil
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
	env, err := s.Store.Get(ctx, envPath(gid))
	if err != nil {
		return EnvGroupView{}, err
	}
	files, err := s.Store.Get(ctx, filesPath(gid))
	if err != nil {
		return EnvGroupView{}, err
	}
	links := m.links
	if links == nil {
		links = []string{}
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
func (s *Service) readMeta(ctx context.Context, gid string) (meta, error) {
	raw, err := s.Store.Get(ctx, metaPath(gid))
	if err != nil {
		return meta{}, err
	}
	if len(raw) == 0 {
		return meta{}, core.ErrNotFound
	}
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
	if m.workspace == "" && s.Workspace != nil {
		m.workspace = core.DefaultTenant
		if err := s.writeMeta(ctx, gid, m); err != nil {
			return meta{}, err
		}
		s.RecordEnvGroupOwnershipMigration(ctx, gid, m.workspace)
	}
	return m, nil
}

func (s *Service) writeMeta(ctx context.Context, gid string, m meta) error {
	return s.Store.Put(ctx, metaPath(gid), map[string]string{
		"name":        m.name,
		"links":       strings.Join(m.links, ","),
		"workspace":   m.workspace,
		"environment": m.environment,
		"createdAt":   m.createdAt,
		"updatedAt":   m.updatedAt,
	})
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

// storeMap writes a map to the source of truth, deleting the path when empty.
func (s *Service) storeMap(ctx context.Context, path string, data map[string]string) error {
	if len(data) == 0 {
		return s.Store.Delete(ctx, path)
	}
	return s.Store.Put(ctx, path, data)
}

// upsertSecret creates or replaces a group projection Secret with exactly the
// given data. Group Secrets have no App owner (a group outlives any one service),
// so DeleteEnvGroup removes them explicitly.
func (s *Service) upsertSecret(ctx context.Context, name string, data map[string]string) error {
	bytesData := make(map[string][]byte, len(data))
	for k, v := range data {
		bytesData[k] = []byte(v)
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, s.Client, sec, func() error {
		sec.Type = corev1.SecretTypeOpaque
		sec.Data = bytesData
		return nil
	})
	return err
}

func (s *Service) deleteSecret(ctx context.Context, name string) error {
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.Namespace}}
	return client.IgnoreNotFound(s.Client.Delete(ctx, sec))
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
	out := envViews(env)
	for i := range out {
		out[i].Value = "" // keys only
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
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

func removeString(list []string, s string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}
