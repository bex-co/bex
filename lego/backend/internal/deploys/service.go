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
	"log"
	"strconv"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	// never a fresh re-fetch (store.CancelRelease). commit is the resolved commit
	// this deploy runs (w9/001), zero when unresolvable.
	CreateDeploy(ctx context.Context, appID, trigger, image string, generation int64, commit store.CommitInfo, triggeredBy string) (store.Deploy, error)
	// LatestDeployCommit returns the newest non-empty commit for the app, or
	// zero CommitInfo — rollout.Tracker carries it onto config_change rows.
	LatestDeployCommit(ctx context.Context, appID string) (store.CommitInfo, error)
	// CreateRollbackDeploy opens a "rollback"-triggered deploy row (w2/m10)
	// restoring image, provenance-tagged with the source deploy id and the
	// target's own commit metadata (w9/001).
	CreateRollbackDeploy(ctx context.Context, appID, image, rollbackOf string, generation int64, commit store.CommitInfo, triggeredBy string) (store.Deploy, error)
	ListDeploys(ctx context.Context, appID string, filter store.DeployFilter) ([]store.Deploy, error)
	GetDeploy(ctx context.Context, appID, deployID string) (store.Deploy, error)
	// CloseDeploy transitions a still-open deploy row terminal, CAS-guarded
	// (see store.Store.CloseDeploy) — Cancel's write path, and the same method
	// the reconciler's write-back uses.
	CloseDeploy(ctx context.Context, id, status, resolvedImage string) (bool, error)
	// SetAppImage clears a legacy row image when deploying from a repository.
	// Image overrides and rollbacks select a release without changing this row.
	SetAppImage(ctx context.Context, id string, image string) error
	// InsertServiceEventFact appends a closed lifecycle fact exactly once
	// (store.PGStore's idempotent-by-source-key insert) — Cancel's route to the
	// build_started/build_ended pair the reconciler itself can never emit for a
	// deploy it closes directly (w6/m128).
	InsertServiceEventFact(ctx context.Context, fact store.ServiceEventFact) (bool, error)
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

// PullSecretPreparer materializes the source credential selected in Settings
// only when a deploy actually starts. apps.Service's bridge satisfies it.
type PullSecretPreparer interface {
	EnsurePullSecret(ctx context.Context, app *appv1alpha1.App) (string, error)
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
	// "" (no step) | "running" | "succeeded" | "failed" | "canceled" (closed while
	// still running, w4/187). A deploy that fails its
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
	// CancelReason is the neutral cause of a non-user cancel (w4/089) — today
	// "Superseded by dep-…" when a newer release replaced this row. Empty for
	// deploys.Cancel and every non-canceled status.
	CancelReason string
	// StallReason is why an OPEN deploy is not progressing (w4/m112) — the
	// operator's live diagnosis of the current revision's pods (a failing
	// health check naming its path, a crash loop, an image pull, a quota
	// block). Empty while a rollout progresses normally, and cleared the
	// moment the deploy goes terminal, when FailureReason takes over. An
	// observation, never a verdict: a deploy carrying one may still go live.
	// A bex extra beyond Render's deploy shape, like FailureReason.
	StallReason string
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
		CancelReason:    d.CancelReason,
		StallReason:     d.StallReason,
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
	// triggerLocks is withTriggerLock's in-process fallback (app id →
	// *sync.Mutex) for a store without advisory locks.
	triggerLocks sync.Map
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
	// PullSecrets resolves the pending image/Dockerfile credential at trigger
	// time. Source-save validation is deliberately read-only so it cannot replace
	// the active release's deterministic pull Secret before this point.
	PullSecrets PullSecretPreparer
}

// deployWorkspace resolves the workspace whose GitHub connection owns this
// App's clone token — the App's own tenant label first (the deploy hook
// carries no caller identity), then the caller's tenant, then the
// single-workspace default; the same precedence as apps.Service's
// deployWorkspace, duplicated because deploys must not import apps.

// openRelease is the one critical section every deploy trigger shares —
// Trigger (API, deploy hook, restart) and Rollback (w8/m46). Near-simultaneous
// triggers used to interleave: each wrote the row image, computed its release
// generation from its OWN earlier read, and merge-patched the CR without a
// resourceVersion, so the newest trigger could lose, both could close
// canceled, and a deploy row could record another caller's image. Now, per
// app:
//
//   - one trigger at a time (a Postgres advisory lock when the store provides
//     one, so bex-api replicas agree; an in-process lock otherwise);
//   - the App is re-read under the lock, so the release generation derives
//     from the newest accepted trigger, never a stale pre-lock copy;
//   - rowWrite (the projector-owned row image) runs before the CR patch, and
//     the patch carries an optimistic lock, re-read and re-applied on conflict;
//   - the deploy row opens inside the same section with the generation the
//     patch produced, so rows are ordered like releases and the newest one is
//     the release the CR names — the reconciler's superseded-row close then
//     cancels every earlier open row by that ordering.
//
// a is updated in place with the patched CR.
func (s *Service) openRelease(ctx context.Context, a *appv1alpha1.App, appID string, rowWrite func() error,
	mutate func(a *appv1alpha1.App, release int64), open func(release int64) (store.Deploy, error)) (store.Deploy, error) {
	var d store.Deploy
	err := s.withTriggerLock(ctx, appID, func() error {
		if err := s.Client.Get(ctx, client.ObjectKeyFromObject(a), a); err != nil {
			return err
		}
		// Deleted while the trigger waited for the lock (w8/023).
		if err := core.NotFoundIfDeleting(a); err != nil {
			return err
		}
		if rowWrite != nil {
			if err := rowWrite(); err != nil {
				return err
			}
		}
		for range 5 {
			previous := a.Generation
			base := client.MergeFromWithOptions(a.DeepCopy(), client.MergeFromWithOptimisticLock{})
			mutate(a, previous+1)
			err := s.Client.Patch(ctx, a, base)
			if err == nil {
				d, err = open(patchedGeneration(previous, a.Generation))
				return err
			}
			if !apierrors.IsConflict(err) {
				return err
			}
			if err := s.Client.Get(ctx, client.ObjectKeyFromObject(a), a); err != nil {
				return err
			}
		}
		return fmt.Errorf("%w: too many concurrent updates to service %q; retry the deploy", core.ErrConflict, a.Name)
	})
	return d, err
}

var _ AppLocker = (*store.PGStore)(nil)

// AppLocker is the optional store capability openRelease serializes on across
// replicas; *store.PGStore provides it.
type AppLocker interface {
	WithAppAdvisoryLock(ctx context.Context, appID string, fn func() error) error
}

// withTriggerLock runs fn holding the per-app trigger lock. A hand-applied App
// (no row id) still serializes in-process on its object name.
func (s *Service) withTriggerLock(ctx context.Context, appID string, fn func() error) error {
	if locker, ok := s.Store.(AppLocker); ok && appID != "" {
		return locker.WithAppAdvisoryLock(ctx, appID, fn)
	}
	mu, _ := s.triggerLocks.LoadOrStore(appID, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	return fn()
}

// appStoreID resolves an already-fetched App CR to its control-plane row id
// (the bex.co/app-id label store.Reconciler stamps) — the key deploy rows are
// stored under. Empty for a hand-applied App: it never had a row, so it never
// has deploy history.
func appStoreID(a *appv1alpha1.App) string { return a.Labels[store.LabelAppID] }

// ListFilter is the neutral shape the REST/GraphQL/MCP adapters translate
// Render's status, exclusive created/updated/finished time bounds, keyset
// cursor, and limit params into. A zero limit (absent) is bounded by the
// store at core.MaxPageLimit, as are values above it (codex-security
// round-6 #7).
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
// silently dropping a bound (or turning a negative limit into "absent",
// which the store reads <=0 as) would return the default page as if it were
// the filtered one. Both limit bounds are the store's invariant
// (store.DeployFilter), not re-clamped here.
func FilterOf(statuses []string, createdBefore, createdAfter, updatedBefore, updatedAfter, finishedBefore, finishedAfter, cursor string, limit int) (ListFilter, error) {
	if limit < 0 {
		return ListFilter{}, core.ErrLimitNotPositive
	}
	f := ListFilter{Statuses: statuses, Cursor: cursor, Limit: limit}
	var err error
	if f.CreatedBefore, err = core.ParseTime("createdBefore", createdBefore); err != nil {
		return ListFilter{}, err
	}
	if f.CreatedAfter, err = core.ParseTime("createdAfter", createdAfter); err != nil {
		return ListFilter{}, err
	}
	if f.UpdatedBefore, err = core.ParseTime("updatedBefore", updatedBefore); err != nil {
		return ListFilter{}, err
	}
	if f.UpdatedAfter, err = core.ParseTime("updatedAfter", updatedAfter); err != nil {
		return ListFilter{}, err
	}
	if f.FinishedBefore, err = core.ParseTime("finishedBefore", finishedBefore); err != nil {
		return ListFilter{}, err
	}
	if f.FinishedAfter, err = core.ParseTime("finishedAfter", finishedAfter); err != nil {
		return ListFilter{}, err
	}
	return f, nil
}

// List returns a service's deploy history, newest first (Render's
// list_deploys / GET .../deploys), narrowed by filter (w2/m31) — a zero
// ListFilter returns the newest core.MaxPageLimit page (cursor for the
// rest). A hand-applied App has no history: an empty list, not an error.
func (s *Service) List(ctx context.Context, service string, filter ListFilter) ([]DeployView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, service)
	if err != nil {
		return nil, err
	}
	// A deleting service is absent from every by-id surface, its deploy history
	// included (w3/m81): reads agree with List and Render's GET 404 rather than
	// serving the history of a resource that is being torn down.
	if err := core.NotFoundIfDeleting(a); err != nil {
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
	// Absent once deletion is accepted, same by-id contract as the list (w3/m81).
	if err := core.NotFoundIfDeleting(a); err != nil {
		return DeployView{}, err
	}
	if s.Store == nil {
		return DeployView{}, core.ErrDeploysUnavailable
	}
	appID := appStoreID(a)
	if appID == "" {
		return DeployView{}, core.NotFound("deploy")
	}
	d, err := s.Store.GetDeploy(ctx, appID, deployID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return DeployView{}, core.NotFound("deploy")
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
	// Rejected for image-backed and cron_job services. Internal restarts retain
	// the live release's commit without accepting a caller-chosen ref.
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
	// mistake, not an oversight). For an image-backed service it must keep the
	// configured registry host, repository and image name — only the tag or
	// digest changes (Render's contract, w8/042).
	ImageURL string
	// ClearCache is Render's "clear" | "do_not_clear" string enum (its
	// dashboard's "Clear build cache and deploy"). When per-App registry
	// caching is enabled (BEX_BUILD_CACHE=registry), "clear" stamps a
	// release-scoped reset on the App so that generation rebuilds without
	// importing prior layers and still exports a fresh cache (w7/m88). With
	// the gate off, both values remain behavioral no-ops (ephemeral BuildKit
	// Jobs already start empty). Empty = omitted = do_not_clear.
	ClearCache string
	// restart marks a Restart's trigger: CommitID is the live deploy's commit,
	// not a caller-chosen ref. Unexported, so no surface can set it.
	restart bool
	// rollbackOf marks Rollback re-publishing a static site's earlier commit
	// (w4/m141): the new deploy row records it as a rollback of this deploy.
	// Unexported, so no surface can set it.
	rollbackOf *store.Deploy
	// disableAutoDeploy carries the dashboard option for static republishing.
	disableAutoDeploy bool
}

// Trigger starts a fresh deploy (Render's POST .../deploys): bumps
// spec.RestartedAt to create a new release identity/generation — triggering the
// operator to rebuild/restart — then opens a dep-… row (trigger "api") stamped
// with that generation so Cancel can later find the right build Job.
//
// p.CommitID, if non-empty, sets spec.BuildCommit so the operator checks out
// that ref instead of Branch HEAD. A trigger without a commitId resolves the
// branch head and pins that SHA (or "" when it cannot be resolved), so each
// trigger builds exactly one commit. Pinning does not turn autoDeploy off:
// Render's API says commitId "does not disable autodeploys"; its dashboard's
// "Deploy a specific commit" does that as a separate step. Restart pins the
// live commit instead of the branch head.
//
// p.DeployMode "deploy_only" is an explicit request NOT to rebuild:
//   - repo-backed service: rejected with ErrBadRequest (this public trigger does
//     not expose cached-artifact deployment; use build_and_deploy).
//   - image-backed service: accepted (nothing to build regardless of mode).
//
// Suspended services refuse the trigger: there is nothing to roll.
func (s *Service) Trigger(ctx context.Context, service string, p TriggerParams) (DeployView, error) {
	// SECURITY (codex round-5 F2): imageUrl and commitId SELECT the executable
	// content this deploy runs — a different image, or a different commit to
	// build — so supplying either is create-like (developer and up). A
	// parameter-free trigger redeploys the artifact the service is already
	// configured for and stays lifecycle, available to contributors.
	a, err := s.AuthorizeApp(ctx, core.LifecycleOrCreate(p.ImageURL != "" || p.CommitID != ""), service)
	if err != nil {
		return DeployView{}, err
	}
	if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		return DeployView{}, err
	}
	return s.triggerFetched(ctx, service, a, p, store.TriggerAPI)
}

// Restart selects the live release's artifact and configuration without changing
// saved settings. For a legacy repo deploy with no resolved artifact, it retains
// the existing commit-pinned rebuild fallback; this cannot promise historical
// configuration that was never recorded.
//
// Authorization is lifecycle, like a parameter-free trigger: the commit is the
// one already running, not content the caller selects, so a contributor can
// restart without create rights. REST, GraphQL (restartServer), MCP and the
// dashboard all restart through this verb.
func (s *Service) Restart(ctx context.Context, service string) (DeployView, error) {
	a, err := s.AuthorizeApp(ctx, core.LifecycleOrCreate(false), service)
	if err != nil {
		return DeployView{}, err
	}
	if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		return DeployView{}, err
	}
	if err := core.NotFoundIfDeleting(a); err != nil {
		return DeployView{}, err
	}
	// Render: "Restart the service with the provided ID. Not supported for
	// cron jobs." A cron job has no running instance to restart; this used to
	// answer 200 and open a live deploy row while nothing ran (w8/040).
	if a.Spec.Type == appv1alpha1.TypeCronJob {
		return DeployView{}, fmt.Errorf("%w: restart is not supported for cron jobs; trigger a run instead", core.ErrBadRequest)
	}
	if selected, err := s.restartSelectedRelease(ctx, a); selected != nil || err != nil {
		if err != nil {
			return DeployView{}, err
		}
		return *selected, nil
	}
	p := TriggerParams{restart: true}
	if a.Spec.Repo != "" {
		if p.CommitID, err = s.liveCommit(ctx, service, a); err != nil {
			return DeployView{}, err
		}
	}
	return s.triggerFetched(ctx, service, a, p, store.TriggerAPI)
}

// liveCommit is the commit a repo-backed service's live release was built
// from. A service with no live deploy — its first deploy is still building, or
// every deploy failed — has no running release to restart, so it is refused
// rather than silently deploying the branch head. A live deploy whose commit
// was never resolved (no GitHub connection) falls back to spec.buildCommit,
// the ref the last build was pinned to; empty means that build used the
// branch head, which is then all a restart can rebuild.
func (s *Service) liveCommit(ctx context.Context, service string, a *appv1alpha1.App) (string, error) {
	if s.Store == nil {
		return "", core.ErrDeploysUnavailable
	}
	appID := appStoreID(a)
	if appID == "" {
		return "", fmt.Errorf("%w: service %q is not store-managed", core.ErrBadRequest, service)
	}
	live, err := s.Store.ListDeploys(ctx, appID, store.DeployFilter{Statuses: []string{store.DeployLive}, Limit: 1})
	if err != nil {
		return "", err
	}
	if len(live) == 0 {
		return "", fmt.Errorf("%w: service %q has no live deploy to restart; deploy it first", core.ErrConflict, service)
	}
	if live[0].Commit != "" {
		return live[0].Commit, nil
	}
	return a.Spec.BuildCommit, nil
}

// validateTrigger holds every reason a trigger is refused before anything is
// mutated. The checks are order-insensitive and read only the App spec and the
// caller's params, so a rejection can never leave a half-applied deploy.
func (s *Service) validateTrigger(service string, a *appv1alpha1.App, p TriggerParams) error {
	if s.Store == nil {
		return core.ErrDeploysUnavailable
	}
	if a.Spec.Suspended {
		return fmt.Errorf("%w: service %q is suspended", core.ErrConflict, service)
	}
	if appStoreID(a) == "" {
		return fmt.Errorf("%w: service %q is not store-managed", core.ErrBadRequest, service)
	}
	// deploy_only for a repo-backed service is rejected: this public trigger does
	// not expose cached-artifact deployment. Operational spec changes reuse the
	// active artifact, but a deploy trigger deliberately rebuilds from source.
	if p.DeployMode == "deploy_only" && a.Spec.Repo != "" {
		return fmt.Errorf("%w: deployMode \"deploy_only\" is not supported for repo-backed services — "+
			"use \"build_and_deploy\" (or omit deployMode) to rebuild from source", core.ErrBadRequest)
	}
	// imageUrl is rejected for repo-backed services: bex rebuilds from source on
	// every trigger; swapping the image would silently divorce the running
	// container from the committed code. For an image-backed service it may only
	// pick another tag or digest of the configured image (below).
	if p.ImageURL != "" && a.Spec.Repo != "" {
		return fmt.Errorf("%w: imageUrl is not supported for repo-backed services — "+
			"bex rebuilds from source on every trigger; use commitId to pin a ref instead", core.ErrBadRequest)
	}
	if p.CommitID != "" && !p.restart && a.Spec.Repo == "" && a.Spec.Image != "" {
		return fmt.Errorf("%w: commitId is not supported for image-backed services — "+
			"deploy an image tag or digest with imageUrl", core.ErrBadRequest)
	}
	// Validate grammar and registry policy before creating a deploy or changing
	// the App, retaining the specific reason so the caller can correct the input.
	if p.ImageURL != "" {
		if err := store.ValidateImage(p.ImageURL); err != nil {
			return fmt.Errorf("%w: imageUrl: %v", core.ErrBadRequest, err)
		}
		if err := sameImageRepository(a.Spec.Image, p.ImageURL); err != nil {
			return err
		}
	}
	// commitId is the second caller field that becomes a git ref: when commit
	// resolution fails (guaranteed for an option-shaped value — GitHub cannot
	// resolve "--upload-pack=..." to a commit), the raw value is written to
	// spec.buildCommit and reaches the clone phase's `git fetch origin "$REF"`
	// argv, where a leading dash is parsed as a git OPTION (codex-security
	// 2026-08 F5). Branch already passes this exact validator; commitId must
	// too, so a ref can never be read as a git flag — the invariant refRE
	// states verbatim.
	if p.CommitID != "" && !store.ValidGitRef(p.CommitID) {
		return fmt.Errorf("%w: commitId must be a git ref (no whitespace, shell metacharacters, or leading dash)", core.ErrBadRequest)
	}
	// commitId is meaningless for a cron_job: a cron runs on a schedule, not
	// per-commit. Reject early rather than silently ignoring the field. A
	// restart's commit is the running one, not caller input, so it is exempt.
	if p.CommitID != "" && !p.restart && a.Spec.Type == appv1alpha1.TypeCronJob {
		return fmt.Errorf("%w: commitId is not supported for cron_job services", core.ErrBadRequest)
	}
	// clearCache is enum-validated here — shared by REST/GraphQL/MCP — so a typo
	// never reaches the App. "clear" stamps a release-scoped reset; anything
	// else (including omission) leaves prior-cache import intact for that
	// generation (w7/m88).
	if p.ClearCache != "" && p.ClearCache != "clear" && p.ClearCache != "do_not_clear" {
		return fmt.Errorf("%w: unknown clearCache %q (valid: clear, do_not_clear)", core.ErrBadRequest, p.ClearCache)
	}
	return nil
}

// sameImageRepository is Render's imageUrl origin rule (w8/042): "host,
// repository, and image name all must match the currently configured image
// for the service". A deploy may change only the tag or digest; switching
// repositories is a saved-settings change (PATCH image.imagePath), which is
// authenticated and audited. This also bounds the unauthenticated deploy hook
// to new versions of the user's own image instead of any public image run
// with the service's environment and secret files.
func sameImageRepository(configured, requested string) error {
	if configured == "" {
		return nil
	}
	want, err := store.ImageRepository(configured)
	if err != nil {
		// A saved image the parser rejects predates validation; refuse rather
		// than let it disable the rule.
		return fmt.Errorf("%w: imageUrl cannot be checked against the service's configured image; fix the image in settings first", core.ErrBadRequest)
	}
	got, err := store.ImageRepository(requested)
	if err != nil {
		return fmt.Errorf("%w: imageUrl: %v", core.ErrBadRequest, err)
	}
	if got != want {
		return fmt.Errorf("%w: imageUrl must use the service's configured image %s (got %s); "+
			"only the tag or digest can change per deploy — change the image in settings to switch repositories",
			core.ErrBadRequest, want, got)
	}
	return nil
}

// triggerFetched is the one deploy-trigger implementation shared by the
// authenticated Trigger verb and the secret-URL deploy hook. The caller owns
// the authentication boundary and supplies an already-resolved App; everything
// after that boundary (validation, CR patch, deploy-history row) is identical.
func (s *Service) triggerFetched(ctx context.Context, service string, a *appv1alpha1.App, p TriggerParams, trigger string) (DeployView, error) {
	// A service being deleted is absent to writes as it is to reads (w8/023):
	// the trigger used to reach its secrets, CR and store row mid-teardown and
	// surface whatever failed first as a 500.
	if err := core.NotFoundIfDeleting(a); err != nil {
		return DeployView{}, err
	}
	if err := s.validateTrigger(service, a, p); err != nil {
		return DeployView{}, err
	}
	appID := appStoreID(a)
	// Refresh the private-repo clone credential BEFORE the generation bump: the
	// build this trigger starts must never run with the previous deploy's
	// expired installation token. A mint failure fails the trigger loudly —
	// same rule as the create path (clonesecret.go): a private repo must never
	// silently fall back to a stale or absent credential.
	cloneSecret := ""
	if s.CloneSecrets != nil && a.Spec.Repo != "" {
		name, err := s.CloneSecrets.EnsureCloneSecret(ctx, a.Namespace, a.Name, s.AppWorkspace(ctx, a), a.Spec.Repo)
		if err != nil {
			return DeployView{}, err
		}
		cloneSecret = name
	}
	pullSecret := a.Spec.ExternalRegistryPullSecret
	if s.PullSecrets != nil {
		name, err := s.PullSecrets.EnsurePullSecret(ctx, a)
		if err != nil {
			return DeployView{}, err
		}
		pullSecret = name
	}
	// Resolve the triggering ref to its exact commit BEFORE the CR patch that
	// starts the rollout (w9/001) — best-effort provenance: the deploy row
	// opens either way, so a GitHub hiccup can never block a deploy.
	commit := s.resolveCommit(ctx, a, p.CommitID)
	// Deploy-hook URLs are unauthenticated; git paths use TriggerNewCommit
	// elsewhere. Manual/API (and any other authenticated Trigger) stamps the
	// request subject — including API keys — the same form audit uses (w4/072).
	triggeredBy := ""
	if trigger != store.TriggerDeployHook {
		triggeredBy = core.SubjectFrom(ctx)
	}
	rowWrite := func() error {
		// Rollback temporarily points a repo-backed service at the selected
		// deploy's resolved image so that exact artifact can run without
		// rebuilding it. A subsequent source deploy must leave that override
		// behind; otherwise spec.image wins over the freshly built artifact
		// forever and, once registry retention removes the old tag, every later
		// deploy fails ErrImagePull. Clear the row first because the
		// control-plane projector owns spec.image.
		if a.Spec.Repo != "" {
			if err := s.Store.SetAppImage(ctx, appID, ""); err != nil {
				return fmt.Errorf("clear rollback image override: %w", err)
			}
		}
		return nil
	}
	disablesAutoDeploy := p.disableAutoDeploy && a.Spec.AutoDeploy
	d, err := s.openRelease(ctx, a, appID, rowWrite, func(a *appv1alpha1.App, release int64) {
		stampReleaseGeneration(a, release)
		stampClearCacheRelease(a, release, p.ClearCache)
		a.Spec.ReleaseConfig = nil
		if p.disableAutoDeploy {
			a.Spec.AutoDeploy = false
		}
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
		if a.Spec.Repo != "" {
			a.Spec.Image = ""
		}
		// A freshly minted clone credential rides the same patch as the bump;
		// "" (public/unconnected repo, or GitHub off) leaves the field alone.
		if cloneSecret != "" {
			a.Spec.CloneSecret = cloneSecret
		}
		a.Spec.ExternalRegistryPullSecret = pullSecret
		// Always write BuildCommit — when the commit resolver succeeds the
		// immutable SHA becomes the build input so the clone job can verify it
		// via EXPECTED_COMMIT (finding-1 TOCTOU). Otherwise the original ref
		// is preserved for best-effort provenance (w9/001) and the operator
		// fetches HEAD without verification. A trigger without commitId resets
		// to "" so Branch HEAD is always the default.
		if commit.Hash != "" {
			a.Spec.BuildCommit = commit.Hash
		} else {
			a.Spec.BuildCommit = p.CommitID
		}
		// A deploy override selects runtime input without changing the saved
		// image that the projector and subsequent config changes use.
		if p.ImageURL != "" {
			a.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: release, Image: p.ImageURL}
		}
	}, func(release int64) (store.Deploy, error) {
		image := a.Spec.Image
		if p.ImageURL != "" {
			image = p.ImageURL
		}
		if p.rollbackOf != nil {
			// Provenance is the target's, whether or not the ref resolved again.
			if commit.Hash == "" {
				commit = store.CommitInfo{Hash: p.rollbackOf.Commit, Message: p.rollbackOf.CommitMessage}
			}
			return s.Store.CreateRollbackDeploy(ctx, appID, image, p.rollbackOf.ID, release, commit, triggeredBy)
		}
		return s.Store.CreateDeploy(ctx, appID, trigger, image, release, commit, triggeredBy)
	})
	if err != nil {
		return DeployView{}, err
	}
	if disablesAutoDeploy {
		s.RecordAutoDeployChanged(ctx, a, false)
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

// stampClearCacheRelease binds Render's clearCache=clear to the release being
// opened. Omission and do_not_clear remove any prior marker so a later normal
// deploy cannot inherit a stale reset (w7/m88).
func stampClearCacheRelease(a *appv1alpha1.App, generation int64, clearCache string) {
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	if clearCache == "clear" {
		a.Annotations[appv1alpha1.AnnotationClearCacheReleaseGeneration] = strconv.FormatInt(generation, 10)
		return
	}
	delete(a.Annotations, appv1alpha1.AnnotationClearCacheReleaseGeneration)
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
		ref = appv1alpha1.DefaultBranch
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
// w2/m10). store.CancelRelease ends its release — the canceled-release stamp
// the operator settles from, then a best-effort stop of a repo-backed build —
// keyed by the deploy row's OWN stored Generation rather than the App's
// current one: a later, unrelated spec write (a scale, an env change, another
// trigger) bumps metadata.generation independently of this deploy, and would
// otherwise name the wrong release and build. It then closes the row canceled
// with the same CAS-guarded CloseDeploy the reconciler's write-back uses —
// whichever of Cancel and a genuinely-converging rollout gets there first
// wins, so a race can never leave the row half-canceled. A deploy that already
// reached any terminal status is past the cancelable window: Render's 409,
// never a silent no-op.
func (s *Service) Cancel(ctx context.Context, service, deployID string) (DeployView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, service)
	if err != nil {
		return DeployView{}, err
	}
	if err := core.NotFoundIfDeleting(a); err != nil {
		return DeployView{}, err
	}
	if s.Store == nil {
		return DeployView{}, core.ErrDeploysUnavailable
	}
	appID := appStoreID(a)
	if appID == "" {
		return DeployView{}, core.NotFound("deploy")
	}
	d, err := s.Store.GetDeploy(ctx, appID, deployID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return DeployView{}, core.NotFound("deploy")
		}
		return DeployView{}, err
	}
	if d.FinishedAt != nil {
		return DeployView{}, fmt.Errorf("%w: deploy %q is already %s", core.ErrConflict, deployID, d.Status)
	}
	if err := store.CancelRelease(ctx, s.Client, a, d.Generation, s.BuildNamespace); err != nil {
		return DeployView{}, err
	}
	won, err := s.Store.CloseDeploy(ctx, deployID, store.DeployCanceled, "")
	if err != nil {
		return DeployView{}, err
	}
	if !won {
		return DeployView{}, fmt.Errorf("%w: deploy %q is already terminal", core.ErrConflict, deployID)
	}
	if a.Spec.Repo != "" {
		// CloseDeploy just made this row terminal, so the reconciler's own
		// recordDeploy pass will never observe it open again and its usual
		// build_started/build_ended emission (recordLifecycleFacts) is now
		// unreachable for this deploy. Derive the same facts directly from d,
		// the row exactly as it stood before the cancel (w6/m128), so a build
		// that started also ends in the feed. Best-effort like every other
		// lifecycle fact write: logged, not fatal — Cancel's own terminal state
		// already landed regardless.
		for _, fact := range store.CanceledBuildLifecycleFacts(d) {
			if _, err := s.Store.InsertServiceEventFact(ctx, fact); err != nil {
				log.Printf("deploys: record cancel build lifecycle fact %s: %v", fact.SourceKey, err)
			}
		}
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
// canceled deploy never has one). Selects historical runtime configuration
// without overwriting the saved settings used by subsequent standard deploys.
//
// SECURITY (codex round-16 #2/#5): deployID SELECTs the executable image that
// becomes the runtime selection, so this is create-like (can_create), not lifecycle —
// the same executable-selection class as Trigger(imageUrl). It also produces a
// deploy write, so it shares Trigger's RequireBillingMutation gate.
//
// opts carries the per-surface differences (at most one is read). It is variadic
// so there stays exactly ONE exported rollback verb: the audit verb is derived
// from the first exported method on the stack (core.callerVerb), so a second
// entry point would rename every rollback's audit event and un-map it from the
// events feed — the regression ADR018 records for apps.writeThroughStore.
func (s *Service) Rollback(ctx context.Context, service, deployID string, opts ...RollbackOptions) (DeployView, error) {
	var o RollbackOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	a, err := s.AuthorizeApp(ctx, core.RelCanCreate, service)
	if err != nil {
		return DeployView{}, err
	}
	if err := core.NotFoundIfDeleting(a); err != nil {
		return DeployView{}, err
	}
	if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
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
			return DeployView{}, core.NotFound("deploy")
		}
		return DeployView{}, err
	}
	if !RollbackEligible(a, target) {
		return DeployView{}, fmt.Errorf("%w: deploy %q never went live — nothing to roll back to", core.ErrConflict, deployID)
	}
	// Rolling back to the currently-live deploy WHEN it is already what is
	// running is a no-op: it changes nothing but still patches RestartedAt + a
	// new generation, so it silently restarts the service and creates a
	// redundant deploy (w4/051 — the dashboard detail page offered it, the list
	// did not). Reject it on every surface. This is narrower than "reject any
	// live target": a deploy stays DeployLive until a NEWER one goes live (a
	// failed deploy never deactivates it), so after a failed rollout the spec
	// can drift off the still-live last-good deploy — rolling back to it then
	// restores it and is a legitimate recovery
	// (TestRollbackRestoresPreviousLiveImage), which the comparison preserves.
	// Shared with the capability projection (RollbackActionable) so the answer
	// deployActions gives and the answer this verb gives cannot drift (w4/110).
	if !RollbackActionable(a, target) {
		return DeployView{}, fmt.Errorf("%w: deploy %q is already live — nothing to roll back to", core.ErrConflict, deployID)
	}
	// A static site published straight from its repository has no image to
	// restore; re-publishing the target's commit restores its files (w4/m141).
	// It is the commit-pinned trigger path, recorded as a rollback of target.
	if rollbackRepublishes(a, target) {
		return s.triggerFetched(ctx, service, a, TriggerParams{CommitID: target.Commit, rollbackOf: &target, disableAutoDeploy: o.DisableAutoDeploy}, store.TriggerRollback)
	}
	selected, err := s.targetReleaseConfig(ctx, a, target, false)
	if err != nil {
		return DeployView{}, err
	}
	disablesAutoDeploy := o.DisableAutoDeploy && a.Spec.AutoDeploy
	d, err := s.openSelectedRelease(ctx, a, target, selected, true, o.DisableAutoDeploy)
	if err != nil {
		return DeployView{}, err
	}
	if disablesAutoDeploy {
		s.RecordAutoDeployChanged(ctx, a, false)
	}
	return view(d), nil
}
