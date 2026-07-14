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

// Package deploys is the deploy-history feature (w2/m5): every rollout of a
// store-managed App is a row in lego/backend/internal/store, listable and
// triggerable over REST/GraphQL/MCP under Render's names (list_deploys /
// get_deploy / POST .../deploys) — the poll-loop a Render-trained agent
// already knows how to run. It requires the control-plane store
// (BEX_CP_DB_URI): deploy history has no CR-only equivalent to fall back to,
// so with the store unwired every verb reports core.ErrDeploysUnavailable
// (503) — the env-vars precedent, omitted rather than faked.
package deploys

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DeployStore is the Service's seam to the control-plane store — the narrow
// slice of Store it needs, the same way apps.IntentStore narrows Store to the
// lifecycle verbs' writes. *store.PGStore satisfies it.
type DeployStore interface {
	// CreateDeploy opens a deploy row; generation is the App CR's
	// metadata.generation this deploy runs under, captured once at open time
	// (w2/m10) — Cancel derives its build-Job identity from the stored value,
	// never a fresh re-fetch (see buildJobName).
	CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64) (store.Deploy, error)
	// CreateRollbackDeploy opens a "rollback"-triggered deploy row (w2/m10)
	// restoring image, provenance-tagged with the source deploy id.
	CreateRollbackDeploy(ctx context.Context, appID, image, rollbackOf string) (store.Deploy, error)
	ListDeploys(ctx context.Context, appID string, filter store.DeployFilter) ([]store.Deploy, error)
	GetDeploy(ctx context.Context, appID, deployID string) (store.Deploy, error)
	// CloseDeploy transitions a still-open deploy row terminal, CAS-guarded
	// (see store.Store.CloseDeploy) — Cancel's write path, and the same method
	// the reconciler's write-back uses.
	CloseDeploy(ctx context.Context, id, status, resolvedImage string) (bool, error)
	// SetAppImage writes the row-owned image field — Rollback's row-first
	// write, same discipline as apps.Service.writeThroughStore.
	SetAppImage(ctx context.Context, id string, image string) error
}

// DeployView is the neutral projection of a store.Deploy the adapters render
// in Render's deploy shape. Commit is left out — it stays empty until w1/m5
// tracks build-from-git commits, so there is nothing yet worth surfacing.
// RollbackOf is a bex extra (w2/m10): empty for every deploy except one
// Rollback created, naming the source deploy it restores.
type DeployView struct {
	ID         string
	Status     string
	Image      string
	Trigger    string
	RollbackOf string
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	// PreDeployStatus is the pre-deploy command's outcome for this deploy (w1/m33):
	// "" (no step) | "running" | "succeeded" | "failed". A deploy that fails its
	// migration is update_failed with PreDeployStatus "failed"; one that fails its
	// health check is update_failed with PreDeployStatus "" — the field is how a
	// client tells the two apart. Its logs are retrievable via the logs surface
	// (`type=predeploy`).
	PreDeployStatus string
}

func view(d store.Deploy) DeployView {
	return DeployView{
		ID:              d.ID,
		Status:          d.Status,
		Image:           d.Image,
		Trigger:         d.Trigger,
		RollbackOf:      d.RollbackOf,
		CreatedAt:       d.CreatedAt,
		StartedAt:       d.StartedAt,
		FinishedAt:      d.FinishedAt,
		PreDeployStatus: d.PreDeployStatus,
	}
}

// Service lists and triggers deploys for store-managed Apps. Embeds
// *core.Base for the auth gate and GetApp (App-name lookup + tenant gate) —
// the same fetch every other feature service shares.
type Service struct {
	*core.Base
	Store DeployStore
	// BuildNamespace is BEX_BUILD_NAMESPACE — the namespace Cancel looks for a
	// repo-backed App's in-flight build Job in (lego/operator's own build
	// namespace, must match so the Job identity resolves); empty falls back to
	// the App's own namespace, the operator's own default (w2/m10).
	BuildNamespace string
}

// buildJobName mirrors lego/operator/internal/build.JobName's naming
// convention (bld-<name>-gen-<generation>, lowercased, 63-char k8s name cap)
// exactly — bex-api must never import the operator (operator/backend
// layering, CLAUDE.md), so the convention is duplicated here; keep the two in
// sync by hand (same precedent as core.PodLabelApp/AppContainer).
func buildJobName(name string, generation int64) string {
	n := "bld-" + name + "-gen-" + strconv.FormatInt(generation, 10)
	if len(n) > 63 {
		n = n[:63]
	}
	return strings.ToLower(n)
}

// patchApp merge-patches a's spec via mutate — the small CR-write dance
// (DeepCopy for the patch base, mutate, Patch) Trigger and Rollback both need
// and neither warrants its own copy of; mirrors apps.Service.patchFetched,
// which lives on a different package's receiver and so can't be called
// directly from here. a is updated in place with the server's response
// (including its bumped metadata.generation), which Trigger relies on.
func (s *Service) patchApp(ctx context.Context, a *appv1alpha1.App, mutate func(*appv1alpha1.App)) error {
	base := client.MergeFrom(a.DeepCopy())
	mutate(a)
	return s.Client.Patch(ctx, a, base)
}

// appStoreID resolves an already-fetched App CR to its control-plane row id
// (the bex.co/app-id label store.Reconciler stamps) — the key deploy rows are
// stored under. Empty for a hand-applied App: it never had a row, so it never
// has deploy history.
func appStoreID(a *appv1alpha1.App) string { return a.Labels[store.LabelAppID] }

// ListFilter narrows List (w2/m31) — the neutral shape the REST/GraphQL/MCP
// adapters translate Render's status/createdBefore/createdAfter/cursor/limit
// params into, mirroring store.DeployFilter field-for-field: Statuses (empty
// ⇒ all), exclusive CreatedBefore/CreatedAfter bounds, keyset Cursor (a
// previously returned deploy's id), and Limit (0 ⇒ the full history — the
// pre-m31 contract; the store clamps anything above core.MaxPageLimit).
type ListFilter struct {
	Statuses      []string
	CreatedBefore time.Time
	CreatedAfter  time.Time
	Cursor        string
	Limit         int
}

// FilterOf builds a ListFilter from the params in the string form every
// adapter has them in (a query value, a GraphQL argument, an MCP tool field)
// — one translator for all three surfaces, the events.FilterOf precedent, so
// a REST call and a tool call with the same params cannot page differently.
// Unlike events' permissive reading, a malformed value is core.ErrBadRequest
// (400): events falls back to its default window, but deploys has none —
// silently dropping a bound (or turning a negative limit into "unbounded",
// which is what the store reads <=0 as) would return the unfiltered history
// as if it were the filtered one. The upper limit bound is the store's
// invariant (store.DeployFilter), not re-clamped here.
func FilterOf(statuses []string, createdBefore, createdAfter, cursor string, limit int) (ListFilter, error) {
	if limit < 0 {
		return ListFilter{}, fmt.Errorf("%w: limit must be a positive integer", core.ErrBadRequest)
	}
	f := ListFilter{Statuses: statuses, Cursor: cursor, Limit: limit}
	var err error
	if f.CreatedBefore, err = parseTime("createdBefore", createdBefore); err != nil {
		return ListFilter{}, err
	}
	if f.CreatedAfter, err = parseTime("createdAfter", createdAfter); err != nil {
		return ListFilter{}, err
	}
	return f, nil
}

// parseTime reads one optional RFC3339 param — empty stays the zero time
// (bound unset), anything else must parse.
func parseTime(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s must be RFC3339 (e.g. 2026-01-02T15:04:05Z)", core.ErrBadRequest, name)
	}
	return t, nil
}

// List returns a service's deploy history, newest first (Render's
// list_deploys / GET .../deploys), narrowed by filter (w2/m31) — a zero
// ListFilter returns the full history. A hand-applied App has no history: an
// empty list, not an error.
func (s *Service) List(ctx context.Context, service string, filter ListFilter) ([]DeployView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, service)
	if err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrDeploysUnavailable
	}
	appID := appStoreID(a)
	if appID == "" {
		return []DeployView{}, nil
	}
	deploys, err := s.Store.ListDeploys(ctx, appID, store.DeployFilter{
		Statuses:      filter.Statuses,
		CreatedBefore: filter.CreatedBefore,
		CreatedAfter:  filter.CreatedAfter,
		Cursor:        filter.Cursor,
		Limit:         filter.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]DeployView, len(deploys))
	for i, d := range deploys {
		out[i] = view(d)
	}
	return out, nil
}

// Get fetches one deploy by dep-… id, scoped to service (Render's
// get_deploy / GET .../deploys/{deployId}). A deployId belonging to a
// different service, or a hand-applied service with no history at all, is
// core.ErrNotFound — the same "not yours" shape GetApp's tenant gate uses,
// never a cross-app leak through the id alone.
func (s *Service) Get(ctx context.Context, service, deployID string) (DeployView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, service)
	if err != nil {
		return DeployView{}, err
	}
	if s.Store == nil {
		return DeployView{}, core.ErrDeploysUnavailable
	}
	appID := appStoreID(a)
	if appID == "" {
		return DeployView{}, core.ErrNotFound
	}
	d, err := s.Store.GetDeploy(ctx, appID, deployID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return DeployView{}, core.ErrNotFound
		}
		return DeployView{}, err
	}
	return view(d), nil
}

// TriggerParams carries the optional body fields of Render's CreateDeploy
// request (commitId, clearCache, deployMode) that bex can honestly honor.
// Zero value = default behavior (Branch HEAD, full build-and-deploy).
type TriggerParams struct {
	// CommitID pins the build to a specific Git ref instead of Branch HEAD.
	// Rejected for cron_job services (they run on a schedule, not per-commit).
	// Only meaningful for repo-backed services; silently ignored for image-backed.
	CommitID string
	// DeployMode selects the deploy strategy. "deploy_only" skips the build
	// step — valid for image-backed services (nothing to build anyway), but
	// returns ErrBadRequest for repo-backed ones (bex has no cached build
	// artifact; any spec change unconditionally rebuilds from source). Empty
	// or "build_and_deploy" is the normal full-rebuild path.
	DeployMode string
}

// Trigger starts a fresh deploy (Render's POST .../deploys): bumps
// spec.RestartedAt to create a new generation — triggering the operator to
// rebuild/restart — then opens a dep-… row (trigger "api") stamped with the
// App's bumped generation so Cancel can later find the right build Job.
//
// p.CommitID, if non-empty, sets spec.BuildCommit so the operator checks out
// that ref instead of Branch HEAD; the field is explicitly reset to "" on
// every trigger without a commitId so Branch HEAD is always the default.
//
// p.DeployMode "deploy_only" is an explicit request NOT to rebuild:
//   - repo-backed service: rejected with ErrBadRequest (bex rebuilds on every
//     generation bump; there is no cached artifact to skip the build with).
//   - image-backed service: accepted (nothing to build regardless of mode).
//
// Suspended services refuse the trigger: there is nothing to roll.
func (s *Service) Trigger(ctx context.Context, service string, p TriggerParams) (DeployView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, service)
	if err != nil {
		return DeployView{}, err
	}
	if s.Store == nil {
		return DeployView{}, core.ErrDeploysUnavailable
	}
	if a.Spec.Suspended {
		return DeployView{}, fmt.Errorf("%w: service %q is suspended", core.ErrConflict, service)
	}
	appID := appStoreID(a)
	if appID == "" {
		return DeployView{}, fmt.Errorf("%w: service %q is not store-managed", core.ErrBadRequest, service)
	}
	// deploy_only for a repo-backed service is rejected: bex has no cached build
	// artifact — every generation bump unconditionally rebuilds from source.
	if p.DeployMode == "deploy_only" && a.Spec.Repo != "" {
		return DeployView{}, fmt.Errorf("%w: deployMode \"deploy_only\" is not supported for repo-backed services — "+
			"bex has no cached build artifact; use \"build_and_deploy\" (or omit deployMode) to rebuild from source", core.ErrBadRequest)
	}
	// commitId is meaningless for a cron_job: a cron runs on a schedule, not
	// per-commit. Reject early rather than silently ignoring the field.
	if p.CommitID != "" && a.Spec.Type == appv1alpha1.TypeCronJob {
		return DeployView{}, fmt.Errorf("%w: commitId is not supported for cron_job services", core.ErrBadRequest)
	}
	if err := s.patchApp(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
		// Always write BuildCommit — set to the requested ref, or "" to reset to
		// Branch HEAD. This ensures a trigger without commitId never inherits the
		// commit a previous trigger pinned.
		a.Spec.BuildCommit = p.CommitID
	}); err != nil {
		return DeployView{}, err
	}
	d, err := s.Store.CreateDeploy(ctx, appID, store.TriggerAPI, a.Spec.Image, a.Generation)
	if err != nil {
		return DeployView{}, err
	}
	return view(d), nil
}

// Cancel kills a still-open deploy (Render's POST .../deploys/{id}/cancel,
// w2/m10): best-effort terminates the in-flight build Job for a repo-backed
// service (an image-backed service has no build Job — the delete is a
// harmless not-found no-op for it), computing the Job's identity from the
// deploy row's OWN stored Generation rather than the App's current one — a
// later, unrelated spec write (a scale, an env change, another trigger) bumps
// metadata.generation independently of this deploy, and would otherwise make
// Cancel compute the wrong Job name and silently no-op past the real build.
// It then closes the row canceled with the same CAS-guarded CloseDeploy the
// reconciler's write-back uses — whichever of Cancel and a genuinely-
// converging rollout gets there first wins, so a race can never leave the
// row half-canceled. Canceling the k8s rollout itself is out of scope
// (matches Render: an image-backed deploy has no build to interrupt in the
// first place). A deploy that already reached a terminal status
// (live/update_failed/canceled) is past the cancelable window: Render's 409,
// never a silent no-op.
func (s *Service) Cancel(ctx context.Context, service, deployID string) (DeployView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, service)
	if err != nil {
		return DeployView{}, err
	}
	if s.Store == nil {
		return DeployView{}, core.ErrDeploysUnavailable
	}
	appID := appStoreID(a)
	if appID == "" {
		return DeployView{}, core.ErrNotFound
	}
	d, err := s.Store.GetDeploy(ctx, appID, deployID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return DeployView{}, core.ErrNotFound
		}
		return DeployView{}, err
	}
	if d.FinishedAt != nil {
		return DeployView{}, fmt.Errorf("%w: deploy %q is already %s", core.ErrConflict, deployID, d.Status)
	}
	if a.Spec.Repo != "" {
		buildNS := a.Namespace
		if s.BuildNamespace != "" {
			buildNS = s.BuildNamespace
		}
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: buildJobName(a.Name, d.Generation), Namespace: buildNS}}
		if err := s.Client.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !apierrors.IsNotFound(err) {
			return DeployView{}, fmt.Errorf("cancel build job: %w", err)
		}
		// A buildpack deploy is represented by a kpack Image with the same
		// deterministic name as the legacy BuildKit Job. Delete both shapes:
		// builder=auto may resolve either way and cancellation must not race that
		// resolution. Unstructured keeps the backend independent of operator/kpack
		// implementation modules.
		image := &unstructured.Unstructured{}
		image.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Image"})
		image.SetName(buildJobName(a.Name, d.Generation))
		image.SetNamespace(buildNS)
		if err := s.Client.Delete(ctx, image, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !apierrors.IsNotFound(err) {
			return DeployView{}, fmt.Errorf("cancel kpack image: %w", err)
		}
	}
	won, err := s.Store.CloseDeploy(ctx, deployID, store.DeployCanceled, "")
	if err != nil {
		return DeployView{}, err
	}
	if !won {
		return DeployView{}, fmt.Errorf("%w: deploy %q is already terminal", core.ErrConflict, deployID)
	}
	d.Status = store.DeployCanceled
	now := s.Now()
	d.FinishedAt = &now
	return view(d), nil
}

// Rollback creates a fresh deploy restoring a previously-live deploy's exact
// image (Render's POST .../rollback {deployId}, w2/m10) — never a history
// rewrite: the new row's own lifecycle (open -> live/failed) converges
// through the same reconciler write-back every other deploy uses. Only a
// deploy that itself reached live is a valid target — ResolvedImage is the
// only field trustworthy enough to restore blind (an in-progress, failed, or
// canceled deploy never has one). Restores what ran (the image), not
// workspace config — replicas/tier/idleTTL stay put, keeping this minimal.
func (s *Service) Rollback(ctx context.Context, service, deployID string) (DeployView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, service)
	if err != nil {
		return DeployView{}, err
	}
	if s.Store == nil {
		return DeployView{}, core.ErrDeploysUnavailable
	}
	if a.Spec.Suspended {
		return DeployView{}, fmt.Errorf("%w: service %q is suspended", core.ErrConflict, service)
	}
	appID := appStoreID(a)
	if appID == "" {
		return DeployView{}, fmt.Errorf("%w: service %q is not store-managed", core.ErrBadRequest, service)
	}
	target, err := s.Store.GetDeploy(ctx, appID, deployID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return DeployView{}, core.ErrNotFound
		}
		return DeployView{}, err
	}
	if target.Status != store.DeployLive || target.ResolvedImage == "" {
		return DeployView{}, fmt.Errorf("%w: deploy %q never went live — nothing to roll back to", core.ErrConflict, deployID)
	}
	// Row-first: the projector owns spec.image for store-managed Apps, so the
	// row updates before the CR patch below — the same writeThroughStore
	// discipline apps.Service's suspend/plan/scale verbs follow, applied here
	// since Rollback is deploys' first verb that changes a row-owned field.
	if err := s.Store.SetAppImage(ctx, appID, target.ResolvedImage); err != nil {
		return DeployView{}, fmt.Errorf("update source of truth: %w", err)
	}
	d, err := s.Store.CreateRollbackDeploy(ctx, appID, target.ResolvedImage, target.ID)
	if err != nil {
		return DeployView{}, err
	}
	if err := s.patchApp(ctx, a, func(a *appv1alpha1.App) {
		a.Spec.Image = target.ResolvedImage
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
	}); err != nil {
		return DeployView{}, err
	}
	return view(d), nil
}
