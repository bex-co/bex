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
	// never a fresh re-fetch (see buildJobName). commit is the resolved commit
	// this deploy runs (w9/001), zero when unresolvable.
	CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo) (store.Deploy, error)
	// CreateRollbackDeploy opens a "rollback"-triggered deploy row (w2/m10)
	// restoring image, provenance-tagged with the source deploy id and the
	// target's own commit metadata (w9/001).
	CreateRollbackDeploy(ctx context.Context, appID, image, rollbackOf string, generation int64, commit store.CommitInfo) (store.Deploy, error)
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

// CommitResolver resolves a repo ref (branch, tag, or SHA) to the exact
// commit it points at, via workspaceID's GitHub App connection —
// github.Service's DeployCommitSource satisfies it (w9/001). nil on the
// Service ⇒ deploy rows carry no commit metadata (omitted, not faked).
// ok=false (nil err) means "nothing to resolve" (no connection, repo not in
// the grant, unknown ref); commit metadata is provenance, so callers treat a
// non-nil err the same way rather than failing the deploy.
type CommitResolver interface {
	ResolveCommit(ctx context.Context, workspaceID, repoURL, ref string) (store.CommitInfo, bool, error)
}

// DeployStartedNotifier is the request-time notification seam. The
// notifications service satisfies it structurally; keeping the interface here
// avoids a feature-package dependency while letting Trigger fire only after
// the deploy row was opened successfully.
type DeployStartedNotifier interface {
	NotifyDeployStarted(ctx context.Context, tenantID, appName, notificationsToSend string)
}

// DeployView is the neutral projection of a store.Deploy the adapters render
// in Render's deploy shape. CommitID/CommitMessage (w9/001) are the resolved
// commit a build-from-git deploy ran, "" when unresolved — the adapters omit
// rather than fake them. CommitAuthorAt (w2/m42) is the git author timestamp
// captured from the same GitHub commit object — nil when unavailable.
// RollbackOf is a bex extra (w2/m10): empty for every deploy except one
// Rollback created, naming the source deploy it restores.
type DeployView struct {
	ID             string
	ServiceID      string
	Status         string
	Image          string
	Trigger        string
	RollbackOf     string
	CommitID       string
	CommitMessage  string
	CommitAuthorAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	// PreDeployStatus is the pre-deploy command's outcome for this deploy (w1/m33):
	// "" (no step) | "running" | "succeeded" | "failed". A deploy that fails its
	// migration is update_failed with PreDeployStatus "failed"; one that fails its
	// health check is update_failed with PreDeployStatus "" — the field is how a
	// client tells the two apart. Its logs are retrievable via the logs surface
	// (`type=predeploy`).
	PreDeployStatus string
	// FailureReason is the actionable cause of a failed deploy (w9/011) — the
	// operator's diagnosis (crash loop with the $PORT hint, image-pull failure,
	// build error) or a health-gate-timeout line. Empty unless Status is a
	// failure. A bex extra beyond Render's deploy shape, like RollbackOf.
	FailureReason string
}

func view(d store.Deploy) DeployView {
	return DeployView{
		ID:              d.ID,
		ServiceID:       d.AppID,
		Status:          d.Status,
		Image:           d.Image,
		Trigger:         d.Trigger,
		RollbackOf:      d.RollbackOf,
		CommitID:        d.Commit,
		CommitMessage:   d.CommitMessage,
		CommitAuthorAt:  d.CommitAuthorAt,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
		StartedAt:       d.StartedAt,
		FinishedAt:      d.FinishedAt,
		PreDeployStatus: d.PreDeployStatus,
		FailureReason:   d.FailureReason,
	}
}

// Service lists and triggers deploys for store-managed Apps. Embeds
// *core.Base for the auth gate and GetApp (App-name lookup + tenant gate) —
// the same fetch every other feature service shares.
type Service struct {
	*core.Base
	Store DeployStore
	// StartedNotifier is invoked asynchronously after a trigger opens its deploy
	// row. nil keeps notifications disabled without changing trigger behavior.
	StartedNotifier DeployStartedNotifier
	// Commits resolves the triggering ref to the exact commit a
	// build-from-git deploy runs (w9/001) — github.Service's
	// DeployCommitSource. nil ⇒ deploy rows open with no commit metadata.
	Commits CommitResolver
	// DeployHookBaseURL is BEX_API_PUBLIC_URL (for example
	// "https://api.bex.co"). It prefixes the secret trigger path returned by the
	// authenticated REST/GraphQL/MCP management surfaces. Empty keeps the path
	// relative, which is useful for local tests but not a copy-ready CI URL.
	DeployHookBaseURL string
	// DeployHookLimiter is the token-keyed limiter for the unauthenticated deploy
	// hook endpoint. It is deliberately separate from api.RateLimiter, whose
	// buckets are keyed by authenticated caller. Nil uses the fixed v1 default.
	DeployHookLimiter *DeployHookRateLimiter
	// BuildNamespace is BEX_BUILD_NAMESPACE — the namespace Cancel looks for a
	// repo-backed App's in-flight build Job in (lego/operator's own build
	// namespace, must match so the Job identity resolves); empty falls back to
	// the App's own namespace, the operator's own default (w2/m10).
	BuildNamespace string
	// CloneSecrets refreshes a repo-backed App's private-clone credential at
	// trigger time (apps.Service's reconciler bridge, wired in the composition
	// root). GitHub App installation tokens live ONE HOUR — a manual trigger or
	// deploy hook changes the release identity and makes the operator rebuild
	// with the PREVIOUS deploy's token, which git surfaces as the misleading
	// "could not read Username" (its 401-then-prompt fallback). Found live on
	// prod 2026-07-17 (agentmarketcap-1: every trigger=api build failed at
	// clone while webhook-triggered siblings built fine). nil ⇒ no refresh
	// (GitHub integration off), prior behavior.
	CloneSecrets store.CloneSecreter
}

// deployWorkspace resolves the workspace whose GitHub connection owns this
// App's clone token — the App's own tenant label first (the deploy hook
// carries no caller identity), then the caller's tenant, then the
// single-workspace default; the same precedence as apps.Service's
// deployWorkspace, duplicated because deploys must not import apps.
func (s *Service) deployWorkspace(ctx context.Context, a *appv1alpha1.App) string {
	if t := a.Labels[core.LabelTenant]; t != "" {
		return t
	}
	if t, ok := s.Tenant(ctx); ok && t != "" {
		return t
	}
	return core.DefaultTenant
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

// ListFilter is the neutral shape the REST/GraphQL/MCP adapters translate
// Render's status, exclusive created/updated/finished time bounds, keyset
// cursor, and limit params into. A zero limit preserves the pre-m31 full-history
// contract; the store clamps values above core.MaxPageLimit.
type ListFilter struct {
	Statuses       []string
	CreatedBefore  time.Time
	CreatedAfter   time.Time
	UpdatedBefore  time.Time
	UpdatedAfter   time.Time
	FinishedBefore time.Time
	FinishedAfter  time.Time
	Cursor         string
	Limit          int
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
func FilterOf(statuses []string, createdBefore, createdAfter, updatedBefore, updatedAfter, finishedBefore, finishedAfter, cursor string, limit int) (ListFilter, error) {
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
	if f.UpdatedBefore, err = parseTime("updatedBefore", updatedBefore); err != nil {
		return ListFilter{}, err
	}
	if f.UpdatedAfter, err = parseTime("updatedAfter", updatedAfter); err != nil {
		return ListFilter{}, err
	}
	if f.FinishedBefore, err = parseTime("finishedBefore", finishedBefore); err != nil {
		return ListFilter{}, err
	}
	if f.FinishedAfter, err = parseTime("finishedAfter", finishedAfter); err != nil {
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
		Statuses:       filter.Statuses,
		CreatedBefore:  filter.CreatedBefore,
		CreatedAfter:   filter.CreatedAfter,
		UpdatedBefore:  filter.UpdatedBefore,
		UpdatedAfter:   filter.UpdatedAfter,
		FinishedBefore: filter.FinishedBefore,
		FinishedAfter:  filter.FinishedAfter,
		Cursor:         filter.Cursor,
		Limit:          filter.Limit,
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
// request (commitId, clearCache, deployMode, imageUrl) that bex can honestly honor.
// Zero value = default behavior (Branch HEAD, full build-and-deploy).
type TriggerParams struct {
	// CommitID pins the build to a specific Git ref instead of Branch HEAD.
	// Rejected for cron_job services (they run on a schedule, not per-commit).
	// Only meaningful for repo-backed services; silently ignored for image-backed.
	CommitID string
	// DeployMode selects the deploy strategy. "deploy_only" skips the build
	// step — valid for image-backed services (nothing to build anyway), but
	// returns ErrBadRequest for repo-backed ones (the public trigger does not
	// expose cached-artifact deployment; use build_and_deploy). Empty
	// or "build_and_deploy" is the normal full-rebuild path.
	DeployMode string
	// ImageURL overrides the image for this deploy (Render's imageUrl). Only
	// accepted for image-backed services; rejected with ErrBadRequest for
	// repo-backed ones (the origin-safety rule: a git-sourced service must be
	// rebuilt from source — swapping its image at trigger time would silently
	// divorce the running container from the committed source and is always a
	// mistake, not an oversight). Any valid image reference is accepted for an
	// image-backed service (the authenticated caller chooses the image).
	ImageURL string
}

// Trigger starts a fresh deploy (Render's POST .../deploys): bumps
// spec.RestartedAt to create a new release identity/generation — triggering the
// operator to rebuild/restart — then opens a dep-… row (trigger "api") stamped
// with that generation so Cancel can later find the right build Job.
//
// p.CommitID, if non-empty, sets spec.BuildCommit so the operator checks out
// that ref instead of Branch HEAD; the field is explicitly reset to "" on
// every trigger without a commitId so Branch HEAD is always the default.
//
// p.DeployMode "deploy_only" is an explicit request NOT to rebuild:
//   - repo-backed service: rejected with ErrBadRequest (this public trigger does
//     not expose cached-artifact deployment; use build_and_deploy).
//   - image-backed service: accepted (nothing to build regardless of mode).
//
// Suspended services refuse the trigger: there is nothing to roll.
func (s *Service) Trigger(ctx context.Context, service string, p TriggerParams) (DeployView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, service)
	if err != nil {
		return DeployView{}, err
	}
	if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		return DeployView{}, err
	}
	return s.triggerFetched(ctx, service, a, p, store.TriggerAPI)
}

// triggerFetched is the one deploy-trigger implementation shared by the
// authenticated Trigger verb and the secret-URL deploy hook. The caller owns
// the authentication boundary and supplies an already-resolved App; everything
// after that boundary (validation, CR patch, deploy-history row) is identical.
func (s *Service) triggerFetched(ctx context.Context, service string, a *appv1alpha1.App, p TriggerParams, trigger string) (DeployView, error) {
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
	// deploy_only for a repo-backed service is rejected: this public trigger does
	// not expose cached-artifact deployment. Operational spec changes reuse the
	// active artifact, but a deploy trigger deliberately rebuilds from source.
	if p.DeployMode == "deploy_only" && a.Spec.Repo != "" {
		return DeployView{}, fmt.Errorf("%w: deployMode \"deploy_only\" is not supported for repo-backed services — "+
			"use \"build_and_deploy\" (or omit deployMode) to rebuild from source", core.ErrBadRequest)
	}
	// imageUrl is rejected for repo-backed services: bex rebuilds from source on
	// every trigger; swapping the image would silently divorce the running
	// container from the committed code. Image-backed services can deploy any
	// valid image ref the caller supplies (origin-safety rule: gate on service
	// kind, not registry domain — the caller is authenticated and chooses the
	// image; that is the point of the verb). NOTE: for an image-backed service the
	// deploy-hook URL (an unguessable token, minted behind RelCanViewSensitive) is
	// therefore effectively an arbitrary-image deploy credential — treat it like a
	// secret (docs/ADR006-bex-api.md § Deploy hooks).
	if p.ImageURL != "" && a.Spec.Repo != "" {
		return DeployView{}, fmt.Errorf("%w: imageUrl is not supported for repo-backed services — "+
			"bex rebuilds from source on every trigger; use commitId to pin a ref instead", core.ErrBadRequest)
	}
	// Validate the supplied image ref at the boundary (w1/m53): reject whitespace/
	// control/shell-meta characters so a malformed reference can't reach the App
	// CR spec (where the CRD schema would reject it with a less legible error).
	if p.ImageURL != "" && !store.ValidImage(p.ImageURL) {
		return DeployView{}, fmt.Errorf("%w: imageUrl must be an OCI reference (no whitespace or shell metacharacters)", core.ErrBadRequest)
	}
	// commitId is meaningless for a cron_job: a cron runs on a schedule, not
	// per-commit. Reject early rather than silently ignoring the field.
	if p.CommitID != "" && a.Spec.Type == appv1alpha1.TypeCronJob {
		return DeployView{}, fmt.Errorf("%w: commitId is not supported for cron_job services", core.ErrBadRequest)
	}
	// Refresh the private-repo clone credential BEFORE the generation bump: the
	// build this trigger starts must never run with the previous deploy's
	// expired installation token. A mint failure fails the trigger loudly —
	// same rule as the create path (clonesecret.go): a private repo must never
	// silently fall back to a stale or absent credential.
	cloneSecret := ""
	if s.CloneSecrets != nil && a.Spec.Repo != "" {
		name, err := s.CloneSecrets.EnsureCloneSecret(ctx, a.Namespace, a.Name, s.deployWorkspace(ctx, a), a.Spec.Repo)
		if err != nil {
			return DeployView{}, err
		}
		cloneSecret = name
	}
	// Resolve the triggering ref to its exact commit BEFORE the CR patch that
	// starts the rollout (w9/001) — best-effort provenance: the deploy row
	// opens either way, so a GitHub hiccup can never block a deploy.
	commit := s.resolveCommit(ctx, a, p.CommitID)
	// Rollback temporarily points a repo-backed service at the selected deploy's
	// resolved image so that exact artifact can run without rebuilding it. A
	// subsequent source deploy must leave that override behind; otherwise
	// spec.image wins over the freshly built artifact forever and, once registry
	// retention removes the old tag, every later deploy fails ErrImagePull.
	// Clear the row first because the control-plane projector owns spec.image.
	if a.Spec.Repo != "" {
		if err := s.Store.SetAppImage(ctx, appID, ""); err != nil {
			return DeployView{}, fmt.Errorf("clear rollback image override: %w", err)
		}
	}
	previousGeneration := a.Generation
	releaseGeneration := previousGeneration + 1
	if err := s.patchApp(ctx, a, func(a *appv1alpha1.App) {
		stampReleaseGeneration(a, releaseGeneration)
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
		if a.Spec.Repo != "" {
			a.Spec.Image = ""
		}
		// A freshly minted clone credential rides the same patch as the bump;
		// "" (public/unconnected repo, or GitHub off) leaves the field alone.
		if cloneSecret != "" {
			a.Spec.CloneSecret = cloneSecret
		}
		// Always write BuildCommit — set to the requested ref, or "" to reset to
		// Branch HEAD. This ensures a trigger without commitId never inherits the
		// commit a previous trigger pinned.
		a.Spec.BuildCommit = p.CommitID
		// imageUrl overrides the running image for this deploy; the operator
		// picks up the new spec.image on its next reconcile.
		if p.ImageURL != "" {
			a.Spec.Image = p.ImageURL
		}
	}); err != nil {
		return DeployView{}, err
	}
	releaseGeneration = patchedGeneration(previousGeneration, a.Generation)
	d, err := s.Store.CreateDeploy(ctx, appID, trigger, a.Spec.Image, releaseGeneration, commit)
	if err != nil {
		return DeployView{}, err
	}
	if store.IsOpenDeployStatus(d.Status) {
		s.notifyDeployStarted(ctx, a, service)
	}
	return view(d), nil
}

// patchedGeneration returns the generation the spec patch initiated. A real
// API server returns the incremented metadata.generation on Patch; controller-
// runtime's fake client does not, so the monotonic fallback keeps unit tests
// and off-cluster adapters aligned with Kubernetes semantics.
func patchedGeneration(before, after int64) int64 {
	return max(after, before+1)
}

func stampReleaseGeneration(a *appv1alpha1.App, generation int64) {
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(generation, 10)
}

// notifyDeployStarted detaches delivery from the request hot path, matching the
// close-time notifier's best-effort pattern. The App's owner label is
// authoritative: using the caller's default workspace here would misroute a
// notification when they operate a service in another workspace they joined.
func (s *Service) notifyDeployStarted(ctx context.Context, a *appv1alpha1.App, service string) {
	if s.StartedNotifier == nil {
		return
	}
	tenantID := a.Labels[core.LabelTenant]
	if tenantID == "" {
		return
	}
	go s.StartedNotifier.NotifyDeployStarted(context.WithoutCancel(ctx), tenantID, service, a.Spec.NotificationsToSend)
}

// resolveCommit resolves the ref a trigger will build — the explicit
// commitId, else the App's branch (the operator's own default-"main"
// fallback, app_controller.go) — to its exact commit via the App's OWN
// workspace's GitHub connection (its core.LabelTenant label, the
// apps.deployWorkspace precedent, so the identity-less deploy hook resolves
// the right connection too). Best-effort by design (w9/001): an image-backed
// App, a missing resolver, or any resolution failure returns the zero
// CommitInfo — the deploy row simply carries no commit metadata, mirroring
// the "omitted, not faked" contract the views hold.
func (s *Service) resolveCommit(ctx context.Context, a *appv1alpha1.App, ref string) store.CommitInfo {
	if s.Commits == nil || a.Spec.Repo == "" {
		return store.CommitInfo{}
	}
	if ref == "" {
		ref = a.Spec.Branch
	}
	if ref == "" {
		ref = "main"
	}
	workspace := a.Labels[core.LabelTenant]
	if workspace == "" {
		if t, ok := s.Tenant(ctx); ok && t != "" {
			workspace = t
		} else {
			workspace = core.DefaultTenant
		}
	}
	commit, ok, err := s.Commits.ResolveCommit(ctx, workspace, a.Spec.Repo, ref)
	if err != nil || !ok {
		return store.CommitInfo{}
	}
	return commit
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
// first place). A deploy that already reached any terminal status is past the
// cancelable window: Render's 409, never a silent no-op.
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
		// Mark the release before deleting its artifact. The operator is
		// level-triggered and would otherwise recreate the deterministic build
		// Job/kpack Image on its next pass because the App generation still asks
		// for this release. A newer deploy stamps a newer release-generation and
		// naturally supersedes this marker.
		base := a.DeepCopy()
		if a.Annotations == nil {
			a.Annotations = map[string]string{}
		}
		a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = strconv.FormatInt(d.Generation, 10)
		if err := s.Client.Patch(ctx, a, client.MergeFrom(base)); err != nil {
			return DeployView{}, fmt.Errorf("mark canceled release: %w", err)
		}
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
	d, err = s.Store.GetDeploy(ctx, appID, deployID)
	if err != nil {
		return DeployView{}, err
	}
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
	if (target.Status != store.DeployLive && target.Status != store.DeployDeactivated) || target.ResolvedImage == "" {
		return DeployView{}, fmt.Errorf("%w: deploy %q never went live — nothing to roll back to", core.ErrConflict, deployID)
	}
	// Row-first: the projector owns spec.image for store-managed Apps, so the
	// row updates before the CR patch below — the same writeThroughStore
	// discipline apps.Service's suspend/plan/scale verbs follow, applied here
	// since Rollback is deploys' first verb that changes a row-owned field.
	if err := s.Store.SetAppImage(ctx, appID, target.ResolvedImage); err != nil {
		return DeployView{}, fmt.Errorf("update source of truth: %w", err)
	}
	previousGeneration := a.Generation
	releaseGeneration := previousGeneration + 1
	if err := s.patchApp(ctx, a, func(a *appv1alpha1.App) {
		stampReleaseGeneration(a, releaseGeneration)
		a.Spec.Image = target.ResolvedImage
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
	}); err != nil {
		return DeployView{}, err
	}
	releaseGeneration = patchedGeneration(previousGeneration, a.Generation)
	d, err := s.Store.CreateRollbackDeploy(ctx, appID, target.ResolvedImage, target.ID, releaseGeneration,
		store.CommitInfo{Hash: target.Commit, Message: target.CommitMessage})
	if err != nil {
		return DeployView{}, err
	}
	return view(d), nil
}
