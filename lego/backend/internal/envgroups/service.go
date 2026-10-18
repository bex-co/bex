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
// services at once. A group's non-secret facts — name, workspace, Environment,
// links, timestamps, the names its content holds and its save coordination —
// are a control-plane row (GroupStore, env_groups); its values are two OpenBao
// maps under the group's workspace tenant ("env-groups/<id>/{env,files}"),
// materialized into two Kubernetes Secrets ("<id>-env", "<id>-files"). Linking
// a group to a service appends those Secret names to the service's App spec
// (spec.envFromSecrets / spec.filesFromSecrets); the operator wires them into
// the container's envFrom and the shared /etc/secrets volume. One
// implementation, three surfaces + dashboard.
package envgroups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Service manages environment groups and projects them into linked services.
// Embeds *core.Base for the client, clock, and authorization gate.
type Service struct {
	*core.Base
	// Store holds the groups' env and files maps (OpenBao).
	Store core.SecretKV
	// Groups holds every non-secret fact about a group (the control-plane
	// store). Both are required; either unset answers ErrSecretsUnavailable.
	Groups GroupStore
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

	// Rollout records the deploy-history row every linked-service patch below
	// owes the user: linking, unlinking, and changing a group's values all roll
	// a new release on each linked service, and until w6/m51 that rollout was
	// invisible in the Deploys tab and Events feed. The composition root wires
	// the control-plane store; nil leaves the patches untracked (CR-only mode).
	Rollout *rollout.Tracker

	// MaxEnvGroups caps how many groups ONE workspace may own (round-11 #3,
	// BEX_MAX_ENV_GROUPS_PER_WORKSPACE, default 100; 0 disables).
	MaxEnvGroups int

	// OpLease bounds how long a busy content/clone operation may run before
	// another API instance may reclaim it from durable recovery evidence
	// (w4/m98). Zero uses defaultEnvGroupOpLease.
	OpLease time.Duration
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

// SecretFileContent is a revealed group file: the content-read answer of REST
// and MCP. Unlike SecretFileView — names-only in metadata and write
// acknowledgements, where omitempty is the masking — content is always present,
// so a revealed empty file reads `content:""` rather than looking unrevealed
// (w4/326).
type SecretFileContent struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// revealEnvGroupFile is GetEnvGroupFile in the content-read shape — unexported:
// it is the REST/MCP adapters' projection of that verb, not a verb of its own.
func (s *Service) revealEnvGroupFile(ctx context.Context, gid, name string) (SecretFileContent, error) {
	f, err := s.GetEnvGroupFile(ctx, gid, name)
	if err != nil {
		return SecretFileContent{}, err
	}
	return SecretFileContent{Name: f.Name, Content: f.Content}, nil
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

// EnvGroupView is the env-group object every surface starts from: REST and MCP
// answer it as Render's envGroup (renderEnvGroup), GraphQL field by field.
// EnvVars/SecretFiles carry keys/names only (no secret material) — a list/get
// never leaks values; the per-var / per-file reveal verbs return them under the
// sensitive scope. OwnerID/CreatedAt/UpdatedAt are Render's `ownerId`/
// timestamps; EnvironmentID is the optional Render-shaped Environment
// membership.
type EnvGroupView struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	OwnerID       string `json:"ownerId,omitempty"`
	EnvironmentID string `json:"environmentId,omitempty"`
	// ServiceLinks is the stored link set, linked services' srv- ids. GraphQL
	// lists them; REST and MCP name each service instead (w5/092).
	ServiceLinks []string         `json:"-"`
	EnvVars      []EnvVarView     `json:"envVars"`
	SecretFiles  []SecretFileView `json:"secretFiles"`
	CreatedAt    string           `json:"createdAt,omitempty"`
	UpdatedAt    string           `json:"updatedAt,omitempty"`
	Revision     string           `json:"revision,omitempty"`
	// Availability is empty for a healthy idle group. "busy" means an active
	// content/clone lease; "repair_required" means automatic recovery failed.
	// When set, EnvVars/SecretFiles/Revision describe only what is safe to show
	// (empty) so a stranded group cannot poison the workspace list (w4/m98).
	Availability string `json:"availability,omitempty"`
	// PendingServiceIDs is set ONLY on the create-with-services result, naming
	// the services that were linked without a deploy because their Auto-Deploy
	// is off. It is a write-result field, never part of a group read, so a list
	// or get always omits it. Same meaning as
	// EnvironmentPatchResult.PendingServiceIDs.
	PendingServiceIDs []string `json:"pendingServiceIds,omitempty"`
}

// --- OpenBao paths + materialized Secret names ---------------------------------

func envPath(gid string) string   { return "env-groups/" + gid + "/env" }
func filesPath(gid string) string { return "env-groups/" + gid + "/files" }

// envSecretName / filesSecretName are the per-group projection Secrets linked
// services consume (envFrom + /etc/secrets projected volume).
func envSecretName(gid string) string   { return gid + "-env" }
func filesSecretName(gid string) string { return gid + "-files" }

// --- group lifecycle ----------------------------------------------------------

// ListEnvGroups returns ownerID's environment groups (no secret material —
// keys/names and links only); "" means the caller's default workspace (w6/m24,
// mirroring apikeys.ListAPIKeys). View scope.
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

// ListEnvGroupsFiltered lists each requested workspace's groups — each owner
// resolved through the membership-checked workspace seam — filters them, and
// builds their views from the rows alone: no group's maps are read.
func (s *Service) ListEnvGroupsFiltered(ctx context.Context, filter EnvGroupListFilter) ([]EnvGroupView, error) {
	filter.EnvironmentIDs = slices.Clone(filter.EnvironmentIDs)
	for i := range filter.EnvironmentIDs {
		filter.EnvironmentIDs[i] = id.EnvironmentStorageID(filter.EnvironmentIDs[i])
	}
	owners := filter.OwnerIDs
	if len(owners) == 0 {
		owners = []string{""}
	}
	seen := make(map[string]struct{})
	var out []EnvGroupView
	for _, ownerID := range owners {
		groups, err := s.listGroups(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			if _, ok := seen[g.ID]; ok || !matchesEnvGroupFilter(g, filter) {
				continue
			}
			seen[g.ID] = struct{}{}
			out = append(out, viewOf(g))
		}
	}
	if out == nil {
		out = []EnvGroupView{}
	}
	return out, nil
}

func matchesEnvGroupFilter(g store.EnvGroup, filter EnvGroupListFilter) bool {
	return (len(filter.Names) == 0 || slices.Contains(filter.Names, g.Name)) &&
		(len(filter.EnvironmentIDs) == 0 || slices.Contains(filter.EnvironmentIDs, g.EnvironmentID)) &&
		core.TimeWindow{Before: filter.CreatedBefore, After: filter.CreatedAfter}.ContainsTime(g.CreatedAt) &&
		core.TimeWindow{Before: filter.UpdatedBefore, After: filter.UpdatedAfter}.ContainsTime(g.UpdatedAt)
}

// pageEnvGroups is the shared GraphQL/MCP/REST paging rule. Callers pass
// requested=false when neither cursor nor limit was supplied, preserving the
// historical complete-list behavior; an explicit page is stable by group id.
func pageEnvGroups(groups []EnvGroupView, cursor string, limit int, requested bool) []EnvGroupView {
	return core.StablePage(groups, cursor, core.PageLimitOrDefault(limit), requested, func(group EnvGroupView) string { return group.ID })
}

// EnvironmentMembership is the non-secret, narrow projection the Environments
// feature needs to assemble Environment.envGroupIds.
type EnvironmentMembership struct {
	ID            string
	EnvironmentID string
}

// ListEnvironmentMemberships returns one workspace's group-to-Environment
// assignments. It shares ListEnvGroups' exact workspace binding and
// authorization path.
func (s *Service) ListEnvironmentMemberships(ctx context.Context, ownerID string) ([]EnvironmentMembership, error) {
	groups, err := s.listGroups(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]EnvironmentMembership, 0, len(groups))
	for _, g := range groups {
		out = append(out, EnvironmentMembership{ID: g.ID, EnvironmentID: g.EnvironmentID})
	}
	return out, nil
}

// listGroups is the shared workspace-scoped list core: ownerID ("" = the
// caller's default) is bound and authorized, then that workspace's rows are
// read.
func (s *Service) listGroups(ctx context.Context, ownerID string) ([]store.EnvGroup, error) {
	ctx = core.WithWorkspace(ctx, ownerID)
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	return s.workspaceGroups(ctx)
}

// workspaceGroups reads the acting workspace's rows; its callers have already
// authorized. A caller with no resolvable workspace sees none.
func (s *Service) workspaceGroups(ctx context.Context) ([]store.EnvGroup, error) {
	if err := s.requireGroups(); err != nil {
		return nil, err
	}
	workspace, ok := s.Tenant(ctx)
	if !ok {
		return []store.EnvGroup{}, nil
	}
	return s.Groups.ListEnvGroups(ctx, workspace)
}

// GetEnvGroup returns one group (keys/names + links, no values); ErrForbidden
// for a group belonging to a workspace the caller can't reach. View scope.
func (s *Service) GetEnvGroup(ctx context.Context, gid string) (EnvGroupView, error) {
	g, err := s.authorizeGroup(ctx, core.RelCanView, gid)
	if err != nil {
		return EnvGroupView{}, err
	}
	return viewOf(g), nil
}

// ExportEnvGroup returns a group's display name and env-var key names for
// Blueprint generation (w4/040) — never values. Sensitive scope matches
// GenerateBlueprint's resource gates.
func (s *Service) ExportEnvGroup(ctx context.Context, gid string) (name string, keys []string, err error) {
	g, err := s.authorizeGroup(ctx, core.RelCanViewSensitive, gid)
	if err != nil {
		return "", nil, err
	}
	if g.Content.State == store.EnvGroupRepairRequired {
		return "", nil, envGroupRestorationFailed()
	}
	if g.Content.State != store.EnvGroupIdle {
		return "", nil, envGroupRevisionConflict()
	}
	view := viewOf(g)
	keys = make([]string, 0, len(view.EnvVars))
	for _, v := range view.EnvVars {
		if v.Key != "" {
			keys = append(keys, v.Key)
		}
	}
	return view.Name, keys, nil
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
	req.EnvironmentID = id.EnvironmentStorageID(req.EnvironmentID)
	if err := s.requireGroups(); err != nil {
		return EnvGroupView{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return EnvGroupView{}, fmt.Errorf("%w: env group name is required", core.ErrBadRequest)
	}
	workspace, ok := s.Tenant(ctx)
	if !ok {
		return EnvGroupView{}, core.ErrForbidden
	}
	if err := s.validateEnvironment(ctx, req.EnvironmentID, workspace); err != nil {
		return EnvGroupView{}, err
	}
	// Round-11 #3: per-workspace create quota, checked before the id is minted
	// or any state written — an abuse bound, not a transaction.
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
	// A taken name fails before any side effect; the unique constraint still
	// decides two creates racing for it (persistCreate's insert).
	if taken, err := s.Groups.EnvGroupNameTaken(ctx, workspace, name); err != nil {
		return EnvGroupView{}, err
	} else if taken {
		return EnvGroupView{}, envGroupNameExists(name, workspace)
	}
	now := s.nowTime()
	g := store.EnvGroup{
		ID: id.New(id.EnvGroup), TenantID: workspace, Name: name, EnvironmentID: req.EnvironmentID,
		ServiceLinks: links, CreatedAt: now, UpdatedAt: now,
	}
	pending, g, err := s.persistCreate(ctx, g, env, files, services)
	if err != nil {
		return EnvGroupView{}, err
	}
	view := viewOf(g)
	view.PendingServiceIDs = pending
	return view, nil
}

// prepareCreateEnv validates and resolves Render's literal-or-generated input
// before CreateEnvGroup mints an id. Duplicate keys follow the existing
// replace-all semantics: the last declaration wins.
func prepareCreateEnv(vars []CreateEnvVarInput) (map[string]string, error) {
	env := make(map[string]string, len(vars))
	for _, input := range vars {
		key := strings.TrimSpace(input.Key)
		if err := core.CheckEnvKey(key); err != nil {
			return nil, err
		}
		hasValue := input.ValueSet || input.Value != ""
		if input.GenerateValue == hasValue {
			return nil, core.InvalidEnvVarValueInput(key)
		}
		value := input.Value
		if err := core.CheckEnvValue(key, value); err != nil {
			return nil, err
		}
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
		if err := core.CheckSecretFileName(name); err != nil {
			return nil, err
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
		a, err := s.findCreateService(ctx, serviceID, workspace)
		if err != nil {
			return nil, nil, fmt.Errorf("serviceId %q: %w", serviceID, err)
		}
		key := core.AppPublicID(a)
		if _, ok := seen[key]; ok {
			continue
		}
		if err := validateGroupServiceEnvironment(environmentID, key, a.Labels); err != nil {
			return nil, nil, err
		}
		seen[key] = struct{}{}
		apps = append(apps, a)
		links = append(links, key)
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
	if a.Labels[core.LabelTenant] != workspace {
		return nil, core.ErrForbidden
	}
	return a, nil
}

// persistCreate applies the already-validated create plan. The row insert is
// last — the create's commit point — so the group becomes discoverable only
// after its contents, projection Secrets, and service refs exist. Any error —
// the unique name constraint refusing a create that raced another for its name
// included — triggers compensation. A crash before the insert leaves only
// unreferenced maps, which no read reaches.
//
// It returns the ids of services that were linked without a deploy because
// their Auto-Deploy is off (autoDeployGated), so the caller can name them on
// the create result, and the inserted row.
func (s *Service) persistCreate(
	ctx context.Context,
	g store.EnvGroup,
	env, files map[string]string,
	services []*appv1alpha1.App,
) (pending []string, _ store.EnvGroup, err error) {
	if err := secrets.ValidateEnvMapQuota(env); err != nil {
		return nil, store.EnvGroup{}, err
	}
	if err := secrets.ValidateFilesMapQuota(files); err != nil {
		return nil, store.EnvGroup{}, err
	}
	gid := g.ID
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
			if err := s.deleteSecret(cleanupCtx, g.TenantID, name); err != nil {
				cleanup = append(cleanup, fmt.Errorf("delete projection Secret %q: %w", name, err))
			}
		}
		if err := s.deleteGroupValues(cleanupCtx, g.TenantID, gid); err != nil {
			cleanup = append(cleanup, fmt.Errorf("delete group %q values: %w", gid, err))
		}
		if len(cleanup) == 0 {
			return cause
		}
		return errors.Join(append([]error{cause}, cleanup...)...)
	}

	if err := s.writeSlot(ctx, g.TenantID, gid, envSlot, env); err != nil {
		return nil, store.EnvGroup{}, rollback(err)
	}
	if err := s.writeSlot(ctx, g.TenantID, gid, filesSlot, files); err != nil {
		return nil, store.EnvGroup{}, rollback(err)
	}
	stamp := s.now()
	for _, a := range services {
		before := a.DeepCopy()
		skipped, err := s.rollLinked(ctx, a, stamp, func(a *appv1alpha1.App) {
			a.Spec.EnvFromSecrets = addString(a.Spec.EnvFromSecrets, envSecretName(gid))
			a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, filesSecretName(gid))
		})
		if err != nil {
			return nil, store.EnvGroup{}, rollback(err)
		}
		if skipped {
			pending = append(pending, core.AppPublicID(a))
		}
		patched = append(patched, before)
	}
	g.EnvKeys, g.FileNames = slices.Sorted(maps.Keys(env)), slices.Sorted(maps.Keys(files))
	inserted, err := s.Groups.InsertEnvGroup(ctx, g)
	if err != nil {
		if errors.Is(err, store.ErrEnvGroupNameTaken) {
			err = envGroupNameExists(g.Name, g.TenantID)
		}
		return nil, store.EnvGroup{}, rollback(err)
	}
	return pending, inserted, nil
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
// metadata write happens only after every linked service has been validated
// against the *current* link set on each attempt (w4/m97).
func (s *Service) MoveEnvGroup(ctx context.Context, gid, environmentID string) (EnvGroupView, error) {
	environmentID = id.EnvironmentStorageID(environmentID)
	g, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return EnvGroupView{}, err
	}
	if err := s.validateEnvironment(ctx, environmentID, g.TenantID); err != nil {
		return EnvGroupView{}, err
	}
	if g.EnvironmentID == environmentID {
		return viewOf(g), nil
	}
	g, err = s.mutateGroup(ctx, gid, func(cur *store.EnvGroup) error {
		if cur.EnvironmentID == environmentID {
			return errMetaUnchanged
		}
		if err := s.validateLinkedServiceEnvironments(ctx, cur.ServiceLinks, environmentID); err != nil {
			return err
		}
		cur.EnvironmentID = environmentID
		cur.UpdatedAt = s.nowTime()
		return nil
	})
	if err != nil {
		return EnvGroupView{}, err
	}
	return viewOf(g), nil
}

// DetachEnvironment returns a group still scoped to expectedEnvironmentID to
// workspace scope while that environment is torn down (w4/m187), keeping its
// contents and links. Links to services in the deleted environment are
// accepted — the same teardown removes them from it — while a link into any
// other environment still refuses. The compare against the expected
// environment leaves a concurrently moved group alone and makes retry a no-op.
func (s *Service) DetachEnvironment(ctx context.Context, gid, expectedEnvironmentID string) error {
	expected := id.EnvironmentStorageID(expectedEnvironmentID)
	g, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	if expected == "" || g.EnvironmentID != expected {
		return nil
	}
	_, err = s.mutateGroup(ctx, gid, func(cur *store.EnvGroup) error {
		if cur.EnvironmentID != expected {
			return errMetaUnchanged
		}
		if err := s.validateLinkedServiceEnvironments(ctx, cur.ServiceLinks, "", expected); err != nil {
			return err
		}
		cur.EnvironmentID = ""
		cur.UpdatedAt = s.nowTime()
		return nil
	})
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
	g, found, err := s.findGroupByName(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: env group %q not found", core.ErrNotFound, name)
	}
	return s.SetEnvironmentID(ctx, g.ID, environmentID)
}

// RenameEnvGroup updates a group's display name without changing its id,
// contents, links, or materialized Secrets. The name is metadata only, so this
// does not roll linked services; the unique (workspace, name) constraint
// refuses a taken name. Manage scope.
func (s *Service) RenameEnvGroup(ctx context.Context, gid, name string) (EnvGroupView, error) {
	g, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return EnvGroupView{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return EnvGroupView{}, fmt.Errorf("%w: env group name is required", core.ErrBadRequest)
	}
	if g.Name == name {
		return viewOf(g), nil
	}
	oldName := g.Name
	g, err = s.mutateGroup(ctx, gid, func(cur *store.EnvGroup) error {
		if cur.Name == name {
			return errMetaUnchanged
		}
		if cur.Name != oldName {
			return envGroupMetadataConflict()
		}
		cur.Name = name
		cur.UpdatedAt = s.nowTime()
		return nil
	})
	if err != nil {
		return EnvGroupView{}, err
	}
	return viewOf(g), nil
}

// DeleteEnvGroup removes the group's row, then unlinks it from every service it
// listed, deletes its projection Secrets, and removes its OpenBao maps. Manage
// scope.
//
// The row goes first so a concurrent link or content writer, whose commit is a
// compare-and-set on the row, finds no group and backs out (w4/m97); the links
// to detach come from the deleted row.
func (s *Service) DeleteEnvGroup(ctx context.Context, gid string) error {
	if _, err := s.authorizeGroup(ctx, core.RelCanCreate, gid); err != nil {
		return err
	}
	var deleted store.EnvGroup
	err := s.Groups.WithEnvGroupContentLock(ctx, gid, func(ctx context.Context) error {
		var err error
		deleted, err = s.Groups.DeleteEnvGroup(ctx, gid)
		return err
	})
	if err != nil {
		return contentErr(err, groupErr(err))
	}
	// Detach from linked services first (drop the spec refs + roll) so no pod is
	// left referencing a Secret about to be deleted.
	for _, svc := range deleted.ServiceLinks {
		if err := s.detach(ctx, gid, svc); err != nil {
			return err
		}
	}
	if err := s.deleteSecret(ctx, deleted.TenantID, envSecretName(gid)); err != nil {
		return err
	}
	if err := s.deleteSecret(ctx, deleted.TenantID, filesSecretName(gid)); err != nil {
		return err
	}
	return s.deleteGroupValues(ctx, deleted.TenantID, gid)
}

// --- group contents (env vars) ------------------------------------------------

// SetEnvGroupVars replaces the group's whole env set, re-materializes the group's
// env Secret, and rolls every linked service. Manage scope.
func (s *Service) SetEnvGroupVars(ctx context.Context, gid string, vars []EnvVarView) ([]EnvVarView, error) {
	g, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return nil, err
	}
	desired := make(map[string]EnvVarView, len(vars))
	for _, v := range vars {
		key := strings.TrimSpace(v.Key)
		if err := core.CheckEnvKey(key); err != nil {
			return nil, err
		}
		v.Key = key
		desired[key] = v // retain replace-all's historical last-declaration-wins rule
	}
	var result EnvironmentPatchResult
	err = s.withGroupContent(ctx, gid, func(ctx context.Context, g store.EnvGroup) error {
		prior, err := s.readGroupContents(ctx, g)
		if err != nil {
			return err
		}
		writes := make([]EnvVarPatch, 0, len(prior.env)+len(desired))
		for _, key := range slices.Sorted(maps.Keys(prior.env)) {
			if _, keep := desired[key]; !keep {
				writes = append(writes, EnvVarPatch{Key: key, Delete: true})
			}
		}
		for _, key := range slices.Sorted(maps.Keys(desired)) {
			v := desired[key]
			writes = append(writes, EnvVarPatch{Key: key, Value: v.Value, ValueSet: v.ValueSet, GenerateValue: v.GenerateValue})
		}
		result, err = s.patchContentsFromSnapshot(ctx, g, prior, EnvironmentPatch{EnvVars: writes, SaveMode: SaveModeDeploy})
		return err
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.rollGroupPatch(ctx, g.ID, SaveModeDeploy, result); err != nil {
		return nil, err
	}
	return envKeyViewsFromKeys(result.EnvVarKeys), nil
}

// GetEnvGroupVar reveals one variable's value (sensitive read).
func (s *Service) GetEnvGroupVar(ctx context.Context, gid, key string) (EnvVarView, error) {
	g, err := s.authorizeGroup(ctx, core.RelCanViewSensitive, gid)
	if err != nil {
		return EnvVarView{}, err
	}
	// codex round-8 #8: per-key reveal — re-assert uncached against the group's
	// owning workspace so a revocation inside PositiveTTL cannot reveal one last
	// value.
	if err := s.authorizeGroupSensitiveFresh(ctx, g); err != nil {
		return EnvVarView{}, err
	}
	var env map[string]string
	err = s.withGroupContent(ctx, gid, func(ctx context.Context, g store.EnvGroup) error {
		var err error
		env, err = s.getGroupMap(ctx, g.TenantID, envPath(gid))
		return err
	})
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
	g, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return EnvVarView{}, err
	}
	key := strings.TrimSpace(input.Key)
	if err := core.CheckEnvKey(key); err != nil {
		return EnvVarView{}, err
	}
	_, err = s.patchEnvironmentAuthorized(ctx, g, EnvironmentPatch{
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
	g, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	if !slices.Contains(g.EnvKeys, key) {
		return core.ErrNotFound
	}
	_, err = s.patchEnvironmentAuthorized(ctx, g, EnvironmentPatch{
		EnvVars: []EnvVarPatch{{Key: key, Delete: true}}, SaveMode: SaveModeDeploy,
	})
	return err
}

// --- group contents (secret files) --------------------------------------------

// SetEnvGroupFile adds or updates one group secret file (merged), re-materializes
// the group's files Secret, and rolls every linked service. Manage scope.
func (s *Service) SetEnvGroupFile(ctx context.Context, gid, name, content string) (SecretFileView, error) {
	g, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return SecretFileView{}, err
	}
	name = strings.TrimSpace(name)
	if err := core.CheckSecretFileName(name); err != nil {
		return SecretFileView{}, err
	}
	_, err = s.patchEnvironmentAuthorized(ctx, g, EnvironmentPatch{
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
	g, err := s.authorizeGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	if !slices.Contains(g.FileNames, name) {
		return core.ErrNotFound
	}
	_, err = s.patchEnvironmentAuthorized(ctx, g, EnvironmentPatch{
		SecretFiles: []SecretFilePatch{{Name: name, Delete: true}}, SaveMode: SaveModeDeploy,
	})
	return err
}

// GetEnvGroupFile reveals one file's content (sensitive read).
func (s *Service) GetEnvGroupFile(ctx context.Context, gid, name string) (SecretFileView, error) {
	g, err := s.authorizeGroup(ctx, core.RelCanViewSensitive, gid)
	if err != nil {
		return SecretFileView{}, err
	}
	// codex round-9 #7: file content is secret reveal — the same uncached
	// reassertion GetEnvGroupVar (round-8 #8) applies, so a revocation inside
	// PositiveTTL cannot ride a cached positive to one last read.
	if err := s.authorizeGroupSensitiveFresh(ctx, g); err != nil {
		return SecretFileView{}, err
	}
	var files map[string]string
	err = s.withGroupContent(ctx, gid, func(ctx context.Context, g store.EnvGroup) error {
		var err error
		files, err = s.getGroupMap(ctx, g.TenantID, filesPath(gid))
		return err
	})
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
// into their own service's Secrets; refused with ErrForbidden, never a silent
// cross-workspace materialization.
func (s *Service) LinkService(ctx context.Context, gid, service string) error {
	// Deferred audit (w4/m122): linking is idempotent, so a re-link wrote
	// nothing and still reported "Env group linked" — and the environment-scope
	// check below refuses after authorization. linkFetched records the row once
	// the link is real.
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, service)
	if err != nil {
		return err
	}
	return s.linkFetched(ctx, gid, a)
}

// linkFetched is LinkService's post-authorize body, shared with LinkEnvGroup (the
// blueprint seam) so the two link paths authorize + audit exactly once each. It
// appends the group's Secret refs to the already-fetched App, rolls it, and
// records the service in the group's link set by compare-and-set so a
// concurrent link to another service is preserved (w4/m97).
func (s *Service) linkFetched(ctx context.Context, gid string, a *appv1alpha1.App) error {
	g, err := s.fetchGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	if a.Labels[core.LabelTenant] != g.TenantID {
		return core.ErrForbidden
	}
	// A link materializes every group value inside caller-controlled workload
	// code, so it is a sensitive read even though the API response stays masked.
	// Reassert against the group's own workspace immediately before adding its
	// Secret refs; a write-only OAuth grant or a freshly revoked developer must
	// not turn App control into indirect secret disclosure.
	if err := s.authorizeGroupSensitiveFresh(ctx, g); err != nil {
		return err
	}
	if err = s.withGroupContent(ctx, gid, func(_ context.Context, fresh store.EnvGroup) error {
		g = fresh
		return nil
	}); err != nil {
		return err
	}
	// Keep the idempotency decision behind the sensitive gate. In particular, a
	// Blueprint re-apply may already control an App linked by an earlier caller;
	// returning before this check would let a write-only caller retain the link
	// while replacing the workload with exfiltration code.
	key := core.AppPublicID(a)
	if slices.Contains(g.ServiceLinks, key) {
		return nil
	}
	if err := validateGroupServiceEnvironment(g.EnvironmentID, key, a.Labels); err != nil {
		return err
	}
	if _, err := s.rollLinked(ctx, a, s.now(), func(a *appv1alpha1.App) {
		a.Spec.EnvFromSecrets = addString(a.Spec.EnvFromSecrets, envSecretName(gid))
		a.Spec.FilesFromSecrets = addString(a.Spec.FilesFromSecrets, filesSecretName(gid))
	}); err != nil {
		return err
	}
	_, err = s.mutateGroup(ctx, gid, func(cur *store.EnvGroup) error {
		if err := validateGroupServiceEnvironment(cur.EnvironmentID, key, a.Labels); err != nil {
			return err
		}
		if slices.Contains(cur.ServiceLinks, key) {
			return errMetaUnchanged
		}
		cur.ServiceLinks = addString(cur.ServiceLinks, key)
		cur.UpdatedAt = s.nowTime()
		return nil
	})
	if err != nil {
		// Group deleted between App patch and the link commit: detach so the
		// App is not left referencing a group that no longer exists.
		if errors.Is(err, core.ErrNotFound) {
			_ = s.detachFetched(context.WithoutCancel(ctx), gid, a)
		}
		return err
	}
	// A link that actually happened: past the already-linked return, past the
	// environment-scope check, and both the App patch and the link commit stuck.
	s.RecordAppConfigChanged(ctx, a, core.AuditVerbLinkService)
	return nil
}

// rollLinkedService applies mutate to an already-fetched linked service and
// merge-patches it, opening a deploy row when the change rolls a new release
// (w6/m51). Every env-group write that reaches a service's App CR goes through
// here: the group's Secret refs and spec.restartedAt are release identity, so
// the operator rebuilds and redeploys for them exactly as it does for a
// Settings-page edit — and the user is owed the same visible deploy.
//
// The patch is locked (w5/m131): spec.filesFromSecrets is one list a merge
// patch replaces whole, so a write computed from a stale read could drop the
// service's own files reference a secrets write just set. A write that loses
// re-reads the App and applies mutate again.
func (s *Service) rollLinkedService(ctx context.Context, a *appv1alpha1.App, mutate func(*appv1alpha1.App)) error {
	return s.Rollout.PatchLocked(ctx, s.Client, a, store.TriggerConfigChange, func(a *appv1alpha1.App) error {
		mutate(a)
		return nil
	})
}

// autoDeployGated reports whether a group write must leave this linked
// service's running release alone.
//
// Render: "If you make changes to an environment group (including deleting
// it), Render kicks off a new deploy for every linked service that has
// autodeploys enabled" (render.com/docs/configure-environment-variables,
// fetched 2026-09-14). Turning Auto-Deploy off is an owner opting out of
// releases they did not ask for, and w1/092 found bex shipping one anyway.
//
// The Repo guard is the part Render's sentence does not cover. bex defaults
// spec.autoDeploy to false for an image-backed service purely because there is
// no branch to watch (apps/service.go "off for an image-backed one (no repo to
// rebuild from)"), not because its owner declined anything. Reading that
// default as an opt-out would silently strand every image-backed service on
// stale group values forever — a far worse bug than the one this gate fixes.
// So the gate applies only where Auto-Deploy is a real user-facing toggle with
// Render's on-by-default: a repo-backed service. See the m94 README Decisions.
func autoDeployGated(a *appv1alpha1.App) bool {
	if a == nil {
		return false
	}
	return a.Spec.Repo != "" && !a.Spec.AutoDeploy
}

// rollLinked is rollLinkedService behind the auto-deploy gate. For a gated
// service the group's Secret refs still land — a link the caller asked for is
// not silently dropped — but spec.restartedAt is left untouched and no deploy
// row opens, so the pod keeps serving its current release and picks the values
// up on the owner's next deploy. It reports whether it skipped, so callers can
// name the untouched services back to the API client.
func (s *Service) rollLinked(ctx context.Context, a *appv1alpha1.App, stamp string, mutate func(*appv1alpha1.App)) (bool, error) {
	if autoDeployGated(a) {
		return true, rollout.PatchAppLocked(ctx, s.Client, a, func(a *appv1alpha1.App) error {
			mutate(a)
			return nil
		})
	}
	return false, s.rollLinkedService(ctx, a, func(a *appv1alpha1.App) {
		mutate(a)
		a.Spec.RestartedAt = stamp
	})
}

// UnlinkService reverses LinkService: drop the group's Secret refs from the
// service, roll it, and remove it from the group's link set. Idempotent: a
// service the group neither lists nor mounts is left alone, and not restarted.
func (s *Service) UnlinkService(ctx context.Context, gid, service string) error {
	// Authorize+fetch against the service's OWN workspace (w6/m17) — reused
	// below via detachFetched, so this is the only fetch of `service` UnlinkService
	// makes. detach (DeleteEnvGroup's bulk-unlink path over every linked service,
	// which authorizes once for the GROUP, not per service) still does its own
	// bare GetApp: it must not fan out into one audit event per linked service.
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, service)
	if errors.Is(err, core.ErrNotFound) {
		return s.dropDeletedLink(ctx, gid, service, err)
	}
	if err != nil {
		return err
	}
	g, err := s.fetchGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	// Linking and unlinking share the same resource-pair invariant. Authorizing
	// each object independently is insufficient for a dual-workspace member:
	// bare service names are workspace-local, so a mismatched pair could detach
	// one workspace's App while mutating another workspace's group metadata.
	if a.Labels[core.LabelTenant] != g.TenantID {
		return core.ErrForbidden
	}
	key := core.AppPublicID(a)
	mounted := mountsGroup(a, gid)
	if !mounted && !slices.Contains(g.ServiceLinks, key) {
		return nil
	}
	if mounted {
		if err := s.detachFetched(ctx, gid, a); err != nil {
			return err
		}
	}
	err = s.dropLinks(ctx, gid, key)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return err
	}
	// A group deleted meanwhile leaves the App detach as the durable outcome.
	s.RecordAppConfigChanged(ctx, a, core.AuditVerbUnlinkService)
	return nil
}

// dropDeletedLink drops a link whose service no longer exists (w4/183), on the
// group's own authorization, without which only deleting the whole group could
// clear the row. Ids are never reused, and AuthorizeApp answers a service id
// it finds but the caller may not touch with a refusal, not not-found: a link
// equal to an id no service answers to is a deleted service's. A name keeps
// the not-found answer, since it may be a live service's the caller cannot
// see.
func (s *Service) dropDeletedLink(ctx context.Context, gid, service string, notFound error) error {
	if !isServiceID(service) {
		return notFound
	}
	g, err := s.fetchGroup(ctx, core.RelCanCreate, gid)
	if err != nil {
		return err
	}
	if !slices.Contains(g.ServiceLinks, service) {
		return notFound
	}
	return s.dropLinks(ctx, gid, service)
}

// detach removes the group's Secret refs from a service and rolls it, tolerating a
// service that no longer exists (a deleted service simply drops from the group).
// A service deleted, or deleted and recreated, since its read is judged again:
// the App now at service is detached if it mounts the group (w5/158).
func (s *Service) detach(ctx context.Context, gid, service string) error {
	err := s.detachOnce(ctx, gid, service)
	if errors.Is(err, core.ErrServiceReplaced) || apierrors.IsNotFound(err) {
		err = s.detachOnce(ctx, gid, service)
	}
	return err
}

func (s *Service) detachOnce(ctx context.Context, gid, service string) error {
	a, err := s.GetApp(ctx, core.RelCanCreate, service)
	if errors.Is(err, core.ErrNotFound) {
		return nil // a since-deleted service just drops from the group
	}
	if err != nil {
		return err
	}
	if !mountsGroup(a, gid) {
		return nil // whatever the link named, this service does not mount the group
	}
	return s.detachFetched(ctx, gid, a)
}

// detachFetched is detach's second half, for a caller (UnlinkService) that
// already holds the App it authorized — reusing it rather than fetching (and
// authorizing, and auditing) a second time.
func (s *Service) detachFetched(ctx context.Context, gid string, a *appv1alpha1.App) error {
	_, err := s.rollLinked(ctx, a, s.now(), func(a *appv1alpha1.App) {
		a.Spec.EnvFromSecrets = removeString(a.Spec.EnvFromSecrets, envSecretName(gid))
		a.Spec.FilesFromSecrets = removeString(a.Spec.FilesFromSecrets, filesSecretName(gid))
	})
	return err
}

// rollOne bumps spec.restartedAt on one fetched linked service so it picks up
// the group's changed Secret data (the Secret refs are already on the spec from
// the link).
//
// A service with Auto-Deploy off is left entirely alone: the group's Secret
// already holds the new values, and not touching spec.restartedAt is exactly
// what keeps the running pod on its current release until the owner deploys.
// It reports whether it skipped for that reason.
func (s *Service) rollOne(ctx context.Context, a *appv1alpha1.App, stamp string) (bool, error) {
	if autoDeployGated(a) {
		return true, nil
	}
	return false, s.rollLinkedService(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.RestartedAt = stamp
	})
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
			"serviceId": serviceID, "serviceEnvironmentId": id.EnvironmentPublicID(serviceEnvironment),
			"targetEnvironmentId": id.EnvironmentPublicID(groupEnvironment),
		},
	)
}

// validateLinkedServiceEnvironments refuses a group scope change to
// environmentID while any linked service sits elsewhere; departing names an
// environment being torn down with the group, whose members count as compatible.
func (s *Service) validateLinkedServiceEnvironments(ctx context.Context, links []string, environmentID string, departing ...string) error {
	var incompatible []string
	for _, serviceID := range links {
		a, err := s.GetApp(ctx, core.RelCanCreate, serviceID)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if env := a.Labels[core.LabelEnvironment]; env != environmentID && !slices.Contains(departing, env) {
			incompatible = append(incompatible, serviceID)
		}
	}
	if len(incompatible) == 0 {
		return nil
	}
	return core.NewConflictError(
		"ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES",
		"move the linked services to the target Environment or unlink them before moving this group",
		map[string]any{"serviceIds": incompatible, "targetEnvironmentId": id.EnvironmentPublicID(environmentID)},
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

// LinkedGroupNames is the names of the groups a already links, in its link
// order — the order its env Secret refs project into the workload (w8/m64).
// Group identity only: no value is read. A ref no visible group owns is
// returned as itself, so it never matches a declared group name.
func (s *Service) LinkedGroupNames(ctx context.Context, a *appv1alpha1.App) ([]string, error) {
	ids, err := s.GroupIDsByName(ctx) // authorizes before anything is read
	if err != nil || a == nil {
		return nil, err
	}
	bySecret := make(map[string]string, len(ids))
	for name, gid := range ids {
		bySecret[envSecretName(gid)] = name
	}
	names := make([]string, 0, len(a.Spec.EnvFromSecrets))
	for _, ref := range a.Spec.EnvFromSecrets {
		name, ok := bySecret[ref]
		if !ok {
			name = ref // not a visible group: callers treat it as undeclared
		}
		names = append(names, name)
	}
	return names, nil
}

// GroupIDsByName returns the acting workspace's non-secret env-group ids keyed
// by name. It is the narrow read seam Blueprint resource inventories and
// current-state plans need; secret values stay behind the reveal verbs.
func (s *Service) GroupIDsByName(ctx context.Context) (map[string]string, error) {
	groups, err := s.listGroups(ctx, "")
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(groups))
	for _, g := range groups {
		out[g.Name] = g.ID
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
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: env group name is required", core.ErrBadRequest)
	}
	g, found, err := s.findGroupByName(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		workspace, ok := s.Tenant(ctx)
		if !ok {
			return core.ErrForbidden
		}
		if err := s.envGroupQuota(ctx, workspace); err != nil {
			return err
		}
		inputs := make([]CreateEnvVarInput, 0, len(literals)+len(generates))
		for _, key := range core.SortedKeys(literals) {
			inputs = append(inputs, CreateEnvVarInput{Key: key, Value: literals[key], ValueSet: true})
		}
		for _, key := range generates {
			inputs = append(inputs, CreateEnvVarInput{Key: key, GenerateValue: true})
		}
		env, err := prepareCreateEnv(inputs)
		if err != nil {
			return err
		}
		now := s.nowTime()
		// No services to link on a Blueprint-minted group, so nothing can be
		// auto-deploy-gated here.
		_, _, err = s.persistCreate(ctx, store.EnvGroup{
			ID: id.New(id.EnvGroup), TenantID: workspace, Name: name, CreatedAt: now, UpdatedAt: now,
		}, env, map[string]string{}, nil)
		return err
	}
	var result EnvironmentPatchResult
	changed := false
	err = s.withGroupContent(ctx, g.ID, func(ctx context.Context, g store.EnvGroup) error {
		env, err := s.getGroupMap(ctx, g.TenantID, envPath(g.ID))
		if err != nil {
			return err
		}
		writes := make([]EnvVarPatch, 0, len(literals)+len(generates))
		for _, key := range core.SortedKeys(literals) {
			if err := core.CheckEnvKey(key); err != nil {
				return err
			}
			// An absent key reads as "", so presence decides too: a declared
			// empty variable must be written, not mistaken for current (w8/095).
			if current, ok := env[key]; !ok || current != literals[key] {
				writes = append(writes, EnvVarPatch{Key: key, Value: literals[key], ValueSet: true})
			}
		}
		for _, key := range generates {
			if err := core.CheckEnvKey(key); err != nil {
				return err
			}
			if _, ok := env[key]; ok {
				continue // generate-once: an existing value persists across syncs
			}
			writes = append(writes, EnvVarPatch{Key: key, GenerateValue: true})
		}
		if len(writes) == 0 {
			return nil
		}
		files, err := s.getGroupMap(ctx, g.TenantID, filesPath(g.ID))
		if err != nil {
			return err
		}
		result, err = s.patchContentsFromSnapshot(ctx, g, groupContents{env: env, files: files}, EnvironmentPatch{EnvVars: writes, SaveMode: SaveModeDeploy})
		changed = err == nil
		return err
	})
	if err != nil || !changed {
		return err
	}
	_, err = s.rollGroupPatch(ctx, g.ID, SaveModeDeploy, result)
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
	// Deferred audit (w4/m122) — the comment this replaces said "authorize (+
	// audit) FIRST", which is exactly the shape that reported an unknown group
	// name as a completed link.
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, service)
	if err != nil {
		return err
	}
	g, found, err := s.findGroupByName(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: env group %q does not exist", core.ErrBadRequest, name)
	}
	return s.linkFetched(ctx, g.ID, a)
}

// findGroupByName resolves an env group by its display name WITHIN THE ACTING
// WORKSPACE (Render addresses groups by name; bex stores them by id). w6/m24:
// a name search never matches, and so never reuses or overwrites, another
// workspace's same-named group. Its callers have already authorized.
func (s *Service) findGroupByName(ctx context.Context, name string) (store.EnvGroup, bool, error) {
	if err := s.requireGroups(); err != nil {
		return store.EnvGroup{}, false, err
	}
	workspace, ok := s.Tenant(ctx)
	if !ok {
		return store.EnvGroup{}, false, nil
	}
	g, err := s.Groups.GetEnvGroupByName(ctx, workspace, name)
	if errors.Is(err, store.ErrNotFound) {
		return store.EnvGroup{}, false, nil
	}
	return g, err == nil, err
}

// --- authorization helpers -----------------------------------------------------

// authorizeGroup gates a verb scoped to ONE named group: the group's own
// workspace first (the single audit point every by-id verb has), then the
// group's row via fetchGroup — the AuthorizeApp/AuthorizeLabeled pattern
// (w6/m17). A caller who is admin of their own workspace but merely a viewer of
// (or not a member of) the group's actual owner may not read/reveal/mutate it
// just because they know its id.
func (s *Service) authorizeGroup(ctx context.Context, relation, gid string) (store.EnvGroup, error) {
	ctx, err := s.ScopeByID(ctx, "", s.groupOwner(gid))
	if err != nil {
		return store.EnvGroup{}, err
	}
	if err := s.Authorize(ctx, relation); err != nil {
		return store.EnvGroup{}, err
	}
	return s.fetchGroup(ctx, relation, gid)
}

// groupOwner is authorizeGroup's routing read (w4/m172): the workspace the
// group's row names, so the leading Authorize — the verb's audit point — checks
// the group's own workspace instead of the caller's default. Anything it cannot
// place stays on the default path, where fetchGroup answers.
func (s *Service) groupOwner(gid string) core.ResourceOwner {
	if !id.WellFormed(gid) {
		return nil
	}
	return func(ctx context.Context) ([]string, error) {
		g, err := s.readGroup(ctx, gid)
		if err != nil {
			return nil, nil
		}
		return []string{g.TenantID}, nil
	}
}

// fetchGroup is authorizeGroup's second half, split out for LinkService/
// UnlinkService: they already authorized (and audited) their primary resource —
// the App, via AuthorizeApp — and only need the group-side cross-workspace check
// added, never a second bare Authorize call (which would double the audit event
// TestAuditCoversEveryWriteVerbExactlyOnce guards against).
func (s *Service) fetchGroup(ctx context.Context, relation, gid string) (store.EnvGroup, error) {
	if !id.WellFormed(gid) {
		if err := s.requireGroups(); err != nil {
			return store.EnvGroup{}, err
		}
		return store.EnvGroup{}, core.ErrNotFound
	}
	g, err := s.readGroup(ctx, gid)
	if err != nil {
		return store.EnvGroup{}, err
	}
	if err := s.AuthorizeLabeled(ctx, relation, map[string]string{core.LabelTenant: g.TenantID}); err != nil {
		return store.EnvGroup{}, err
	}
	return g, nil
}

// authorizeGroupSensitiveFresh is the sink-adjacent, uncached authorization
// shared by direct value reveals and indirect workload materialization.
func (s *Service) authorizeGroupSensitiveFresh(ctx context.Context, g store.EnvGroup) error {
	return s.AuthorizeFreshOn(ctx, core.RelCanViewSensitive, core.WorkspaceObject(g.TenantID))
}

// --- store + secret helpers ---------------------------------------------------

// viewOf builds a secret-free view solely from the row. A save that is busy or
// needs repair reports its availability without reading or modifying its maps;
// the next content mutation or clone offers it to recovery.
func viewOf(g store.EnvGroup) EnvGroupView {
	view := baseView(g)
	if g.Content.State != store.EnvGroupIdle {
		view.Availability = g.Content.State
		return view
	}
	view.EnvVars = envKeyViewsFromKeys(g.EnvKeys)
	view.SecretFiles = fileNameViewsFromNames(g.FileNames)
	view.Revision = encodeEnvGroupRevision(g.Content.Generation)
	return view
}

// baseView is a group's view without its content names or revision — all a
// group held by a save may show.
func baseView(g store.EnvGroup) EnvGroupView {
	links := g.ServiceLinks
	if links == nil {
		links = []string{}
	}
	return EnvGroupView{
		ID: g.ID, Name: g.Name, OwnerID: g.TenantID, EnvironmentID: id.EnvironmentPublicID(g.EnvironmentID),
		ServiceLinks: links, CreatedAt: timestamp(g.CreatedAt), UpdatedAt: timestamp(g.UpdatedAt),
		EnvVars: []EnvVarView{}, SecretFiles: []SecretFileView{},
	}
}

// envGroupQuota refuses a create that would push workspace past its env-group
// cap. MaxEnvGroups > 0 enables (production wires the env default of 100; 0
// leaves tests uncapped).
func (s *Service) envGroupQuota(ctx context.Context, workspace string) error {
	if s.MaxEnvGroups <= 0 {
		return nil
	}
	count, err := s.Groups.CountEnvGroups(ctx, workspace)
	if err != nil {
		return err
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
// (its row's tenant), never the caller's resolved tenant: an ID-scoped verb
// authorizes against the owning workspace but leaves the caller's
// named-or-default workspace in ctx, so deriving the namespace from ctx would
// write the owner's secret material into the caller's namespace.
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
// projected volumes.
func (s *Service) envGroupNamespace(workspace string) string {
	return s.AppNamespace(workspace)
}

// now is the rolling-restart stamp (RFC3339Nano so back-to-back edits differ).
func (s *Service) now() string { return timestamp(s.nowTime()) }

// --- view + validation helpers ------------------------------------------------

func envKeyViewsFromKeys(keys []string) []EnvVarView {
	out := make([]EnvVarView, 0, len(keys))
	for _, key := range keys {
		out = append(out, EnvVarView{Key: key})
	}
	return out
}

func fileNameViewsFromNames(names []string) []SecretFileView {
	out := make([]SecretFileView, 0, len(names))
	for _, name := range names {
		out = append(out, SecretFileView{Name: name})
	}
	return out
}

func addString(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// removeString returns list without any occurrence of the drop values. It
// clones rather than compacting in place so no caller's retained slice is
// mutated behind its back (the same contract secrets.removeString keeps).
func removeString(list []string, drop ...string) []string {
	return slices.DeleteFunc(slices.Clone(list), func(v string) bool { return slices.Contains(drop, v) })
}
