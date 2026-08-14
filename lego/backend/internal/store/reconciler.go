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

package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Labels stamped on every projected App CR. LabelAppID ties the CR back to
// its apps row (the reconciler's join key); LabelTenant carries the owning
// tenant's id (tea-<id>) — core.Base.GetApp's single source of truth for
// gating a fetched App against the caller's tenant, so every feature that
// fetches by name (apps/logs/metrics/secrets) inherits the same check; the
// managed-by label scopes list/delete so hand-applied Apps are never touched.
//
// ManagedByValue is PERSISTED DATA — it lives on every projected App CR in
// live clusters. The service was renamed bex-backend, but changing this value
// would blind the projection to existing CRs (List filters on it) and make it
// re-Create them into name conflicts. Flip it only with a relabel migration.
// LabelTenant is re-stamped on every resync (stampLabels), so a value change
// (it once carried the tenant name) migrates existing CRs without a relabel.
const (
	LabelManagedBy = "app.kubernetes.io/managed-by"
	ManagedByValue = "bex-controlplane"
	LabelAppID     = core.LabelAppID
	// LabelTenant aliases core.LabelTenant — one label, one constant, so the
	// stamp (here) and the gate (core.Base.GetApp) can never drift apart.
	LabelTenant = core.LabelTenant
	// LabelWorkspace aliases core.LabelWorkspace so the stamp (here) and the
	// operator's propagation to pod templates share one canonical value.
	LabelWorkspace      = core.LabelWorkspace
	defaultResyncPeriod = 30 * time.Second
	// Phase-specific gate timeouts bound how long a deploy may sit open before
	// recordDeploy gives up on it and calls it failed. BuildKit Jobs have a
	// 30-minute active deadline and pre-deploy Jobs have a 10-minute deadline,
	// so their control-plane budgets must be longer than the mechanism they
	// observe. Each budget starts from the deploy row's last phase transition.
	// The shorter rollout budget is needed because a bad image
	// (ImagePullBackOff) never makes the App CR's own phase machine reach PhaseFailed — it polls PhaseDeploying
	// forever (lego/operator/internal/controller/app_controller.go) — so
	// health gating (docs/ADR004-app-deployment.md) needs its own timeout, not just the
	// CR's phase, to ever report a deploy as failed.
	defaultBuildGateTimeout     = 35 * time.Minute
	defaultPreDeployGateTimeout = 12 * time.Minute
	// defaultDeployGateTimeout observes the rollout the same way the two gates
	// above observe their Jobs, so it follows the same rule: a control-plane
	// budget must be strictly LONGER than the mechanism it watches, or the
	// generic "did not become healthy within the health-gate window" races the
	// mechanism's own specific verdict and can win — re-creating the vague
	// message w7/m79 worked to eliminate.
	//
	// The mechanism here is the rollout budget in
	// lego/operator/internal/controller/deployment_projection.go
	// (rolloutBudgetSeconds = 900s), which is Render's window: "If this
	// condition is not met within 15 minutes, Render cancels the deploy and
	// continues routing traffic to your service's existing instances." Both
	// the startupProbe budget and Deployment.spec.progressDeadlineSeconds are
	// derived from it, so the Deployment reports ProgressDeadlineExceeded at
	// 15m and this gate closes the row at 18m. The margin matches the existing
	// pattern (build 30m→35m, pre-deploy 10m→12m).
	//
	// The constant cannot be shared — separate Go module, and backend must
	// never import operator — so the invariant is held by a test on each side.
	//
	// Before m80 this was 3 minutes against the operator's inherited 600s
	// default, so the real budget was 3 minutes for reasons nobody chose: an
	// image pull alone has been measured at 52s on a 498 MB image, before the
	// container even starts.
	defaultDeployGateTimeout = 18 * time.Minute
)

// CloneSecreter is called by the Reconciler when projecting a new App CR for
// a repo-backed row: it mints a GitHub installation token, writes the
// <app>-clone Secret, and returns the Secret name for spec.cloneSecret —
// so private-repo builds from the first projected deploy authenticate without
// a separate API call. apps.Service satisfies this; cmd/api injects it when
// both the GitHub App and the control-plane store are wired. nil => public-clone
// only (no authentication, unchanged behaviour).
type CloneSecreter interface {
	EnsureCloneSecret(ctx context.Context, namespace, appName, workspaceID, repo string) (string, error)
}

// DeployNotification is what DeployNotifier fans out — the closed deploy's
// identity and outcome, plus the App's per-service notification override
// (NotifyOnFail: w4/m21, docs/render-artifacts/notify-on-fail.md —
// "default"/"notify"/"ignore", or "" for an App created before the field
// existed, equivalent to "default"). A struct, not positional args, matching
// this codebase's convention for a cross-package "fan out an event" boundary
// (core.AuditEvent, webhooks' payload/DueWebhookDelivery) — so the next
// Render field lands as one more struct field, not a sixth positional arg.
type DeployNotification struct {
	TenantID string
	AppName  string
	Status   string
	// DeployID and CommitMessage enrich the notification email (w7/m44): the
	// deploy id builds the "View Logs" deep link, the commit message names what
	// was deploying. Both empty-friendly — an image-backed deploy has no commit.
	DeployID      string
	CommitMessage string
	// CommitSHA and RepoURL build the deploy email's "View commit" link (the
	// repo's web commit URL). Both empty-friendly — an image-backed deploy has
	// no repo, and a repo build without a resolved SHA has no target.
	CommitSHA string
	RepoURL   string
	// FailureReason is the operator's actionable diagnosis for a failed deploy
	// (w7/m79) — the same string the deploy row and every API surface carry.
	// The failure email is the first thing most people read when a deploy
	// breaks, and it named the commit and linked the logs without ever saying
	// what went wrong. Empty for a succeeded deploy and for a failure the
	// operator could not diagnose.
	FailureReason string
	// NotifyOnFail is the legacy failure-only override. NotificationsToSend is
	// the authoritative Render policy when non-empty.
	NotifyOnFail        string
	NotificationsToSend string
}

// DeployNotifier is called by the Reconciler when recordDeploy closes a
// deploy as succeeded or failed (w3/m9) — it fans the outcome out to the
// workspace's members by email, per each member's notification preferences.
// *notifications.Service satisfies it structurally (this package cannot
// import notifications: notifications imports store for NotificationsStore,
// so the dependency must run the other way, same shape as CloneSecreter).
// nil => no notifications (store-off / feature-off mode, byte-identical to
// before this milestone). Best-effort: implementations must not return an
// error — a flaky relay must never block reconciliation.
type DeployNotifier interface {
	NotifyDeploy(ctx context.Context, n DeployNotification)
}

// Reconciler projects the source of truth into the cluster: each apps row
// (+ its domains) becomes an App CR; rows deleted from Postgres get their CR
// deleted; the CR's observed status (phase, url) is written back to the row.
// It is level-triggered — a full resync every Resync plus a Kick after API
// writes — so etcd stays a rebuildable projection of Postgres.
type Reconciler struct {
	Client client.Client
	Store  Store
	Resync time.Duration // full-resync interval
	// DeployGateTimeout bounds how long a deploy may stay open before
	// recordDeploy closes it as failed even though the CR's phase never
	// reached Failed on its own (see defaultDeployGateTimeout).
	DeployGateTimeout    time.Duration
	BuildGateTimeout     time.Duration
	PreDeployGateTimeout time.Duration
	// CloneSecrets, when non-nil, is called for each new projected App CR
	// whose row has a non-empty Repo, to mint and write the per-app
	// clone-credential Secret. Useful for rows created via the internal CP
	// API (store/api.go) where the public-surface create path hasn't already
	// done so. Soft failure: a minting error is logged but the CR is still
	// created (public repos don't need a secret).
	CloneSecrets CloneSecreter

	// DeployNotifier, when non-nil, is called every time recordDeploy closes a
	// deploy as succeeded or failed — see DeployNotifier. nil => no emails
	// (notifications feature off / store off).
	DeployNotifier DeployNotifier

	// unhealthyOnce remembers, per app, that the previous pass observed
	// unhealthy without recording it — the w3/m78 single-tick debounce (see
	// debounceUnhealthy). Per-process memory is deliberate: a restart just
	// costs one extra confirming tick.
	unhealthyOnce map[string]bool

	kick chan struct{}
}

// namespaceFor returns the namespace an App row projects into: its
// workspace's per-tenant hosting namespace (ADR043).
func namespaceFor(d DesiredApp) string {
	return WorkspaceNamespace(d.TenantID)
}

// debounceUnhealthy suppresses a single-tick unhealthy observation: the first
// pass that sees unhealthy is remembered but recorded as availability-unseen
// (the checkpoint keeps its previous value); only a second consecutive
// unhealthy pass records it and can emit server_failed. Found live in the
// w3/m78 crash leg: during old-ReplicaSet reaping after a successful rollout,
// a reconcile can land on a stale Deployment status (kube-controller-manager
// catch-up) and report Ready=False for one pass, which — with the deploy
// already closed — paged a phantom Critical server_failed + server_available
// pair. A real outage persists across resyncs and is merely delayed one tick;
// recovery stays immediate (a delayed all-clear helps nobody).
func debounceUnhealthy(obs ObservedServiceState, unhealthyOnce map[string]bool) ObservedServiceState {
	unhealthy := obs.AvailabilityObserved && obs.Availability == "unhealthy"
	if unhealthy && !unhealthyOnce[obs.AppID] {
		unhealthyOnce[obs.AppID] = true
		obs.AvailabilityObserved = false
		obs.Availability = ""
		obs.ReasonCode = ""
		return obs
	}
	unhealthyOnce[obs.AppID] = unhealthy
	return obs
}

func NewReconciler(cl client.Client, store Store) *Reconciler {
	return &Reconciler{
		Client:               cl,
		Store:                store,
		Resync:               defaultResyncPeriod,
		DeployGateTimeout:    defaultDeployGateTimeout,
		BuildGateTimeout:     defaultBuildGateTimeout,
		PreDeployGateTimeout: defaultPreDeployGateTimeout,
		kick:                 make(chan struct{}, 1),
	}
}

// Kick schedules an immediate reconcile (non-blocking, coalescing). The API
// calls it after every successful write so a POST is projected within
// milliseconds instead of a resync period.
func (r *Reconciler) Kick() {
	select {
	case r.kick <- struct{}{}:
	default:
	}
}

// Run reconciles until ctx is done.
func (r *Reconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.Resync)
	defer ticker.Stop()
	for {
		if err := r.ReconcileOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("controlplane: reconcile: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-r.kick:
		}
	}
}

// ReconcileOnce drives one full pass: desired rows vs. existing managed CRs →
// create / update / delete, then copies each CR's status back to its row.
// Per-app failures are collected, not fatal — one bad row can't block the rest.
func (r *Reconciler) ReconcileOnce(ctx context.Context) error {
	desired, err := r.Store.ListDesiredApps(ctx)
	if err != nil {
		return fmt.Errorf("list desired apps: %w", err)
	}
	// The managed CRs are spread across every `<ws>` namespace (ADR043), so the
	// list must be cluster-wide (still filtered to our managed-by label). The
	// byID index keys on the unique app-id label, so create/update/delete
	// matching is namespace-agnostic.
	var existing appv1alpha1.AppList
	if err := r.Client.List(ctx, &existing, client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		return fmt.Errorf("list App CRs: %w", err)
	}
	byID := make(map[string]*appv1alpha1.App, len(existing.Items))
	for i := range existing.Items {
		id := existing.Items[i].Labels[LabelAppID]
		if id == "" {
			continue
		}
		// Two managed CRs sharing one id means a stray duplicate exists (see
		// core.AppInOwnWorkspaceNamespace). Index the canonical CR so spec
		// updates and status write-back target the live projection, never
		// last-in-list order; log the loser every pass (the nag is the point) —
		// it is invisible to this loop's update/delete matching and needs a
		// manual finalizer-aware removal (its name-keyed finalizer teardown
		// would otherwise purge the LIVE twin's registry repo + credentials).
		if cur, ok := byID[id]; ok {
			keep, drop := &existing.Items[i], cur
			if core.AppInOwnWorkspaceNamespace(cur) {
				keep, drop = cur, &existing.Items[i]
			}
			log.Printf("controlplane: duplicate App CRs for %s: keeping %s/%s, ignoring stray %s/%s",
				id, keep.Namespace, keep.Name, drop.Namespace, drop.Name)
			byID[id] = keep
			continue
		}
		byID[id] = &existing.Items[i]
	}
	// One query for every app's open deploy (not one per app in the loop
	// below) — at most one open deploy per app in practice, so the last
	// write wins if that invariant is ever violated.
	openDeploys, err := r.Store.ListOpenDeploys(ctx)
	if err != nil {
		return fmt.Errorf("list open deploys: %w", err)
	}
	openByApp := make(map[string]Deploy, len(openDeploys))
	for _, d := range openDeploys {
		openByApp[d.AppID] = d
	}

	var errs []error
	seen := make(map[string]bool, len(desired))
	for _, d := range desired {
		seen[d.ID] = true
		cur, ok := byID[d.ID]
		if !ok {
			if err := r.Client.Create(ctx, r.projectApp(ctx, d)); err != nil {
				errs = append(errs, fmt.Errorf("create App %s/%s: %w", d.TenantName, d.Name, err))
			}
			continue
		}
		// Project desired spec only; observed status (phase/url) is read from
		// the CR by bex-api, never copied back into Postgres. Labels are
		// re-stamped so a LabelTenant value change migrates without a relabel.
		specChanged := applyOwnedSpec(&cur.Spec, projectSpec(d))
		labelsChanged := stampLabels(cur, d)
		if specChanged || labelsChanged {
			if err := r.Client.Update(ctx, cur); err != nil {
				errs = append(errs, fmt.Errorf("update App %s: %w", cur.Name, err))
				continue
			}
		}
		// Deploy write-back (w2/m5): cur.Status still holds what this pass's
		// List observed — Update above only patches spec (status is a separate
		// subresource) — so it's the right snapshot to decide the app's open
		// deploy, if any, is done.
		open, hasOpenDeploy := openByApp[d.ID]
		if hasOpenDeploy {
			r.recordDeploy(ctx, d, open, cur)
		}
		if r.unhealthyOnce == nil {
			r.unhealthyOnce = make(map[string]bool)
		}
		obs := debounceUnhealthy(observedServiceStateFor(d.ID, cur, hasOpenDeploy), r.unhealthyOnce)
		if _, err := r.Store.RecordObservedServiceState(ctx, obs); err != nil {
			log.Printf("controlplane: record observed service state %s: %v", d.ID, err)
		}
		r.recordAutoscalingFacts(ctx, d.ID, cur.Status.Autoscaling)
	}
	// Rows deleted from Postgres → delete their projected CR.
	for id, cur := range byID {
		if seen[id] {
			continue
		}
		delete(r.unhealthyOnce, id)
		if err := r.Client.Delete(ctx, cur); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete App %s: %w", cur.Name, err))
		}
	}
	return errors.Join(errs...)
}

// recordDeploy projects the current generation's observable App/Job facts onto
// one legal deploy transition. Sampling can skip a fast phase, but it can never
// invent one: the transition model accepts forward skips and rejects regressions.
// A rollout that stays open past DeployGateTimeout settles to the failure for
// its last observed phase (the health-gate timeout remains the mechanism for an
// ImagePullBackOff that otherwise stays Deploying forever). Store errors are
// logged, not fatal: deploy bookkeeping must never block App CR reconciliation.
func (r *Reconciler) recordDeploy(ctx context.Context, d DesiredApp, open Deploy, cur *appv1alpha1.App) {
	// Project the pre-deploy step's outcome (w1/m33) onto the open row first, so a
	// migration failure is recorded even on the same pass that closes the deploy
	// update_failed — that's what lets a client tell a failed migration apart from
	// a failed health check. SetDeployPreDeployStatus is a no-op when unchanged.
	if pds := preDeployStatusFor(cur); pds != "" {
		if _, err := r.Store.SetDeployPreDeployStatus(ctx, open.ID, pds); err != nil {
			log.Printf("controlplane: set pre-deploy status %s: %v", open.ID, err)
		}
	}
	if fact, ok := observedImagePullFailure(open, cur); ok {
		if _, err := r.Store.InsertServiceEventFact(ctx, fact); err != nil {
			log.Printf("controlplane: record image-pull failure %s: %v", open.ID, err)
		}
	}

	status := observedDeployStatus(open, cur, r.deployTimedOut(d, open))
	// Build and pre-deploy beats surface as distinct timeline events (w7/m66),
	// derived from the same observed state — emitted before the no-transition
	// early return so a still-building or still-pre-deploying deploy is recorded.
	r.recordLifecycleFacts(ctx, d, open, cur, status)
	if status == "" {
		return
	}
	// resolvedImage backfills Deploy.ResolvedImage (Rollback's restore target,
	// w2/m10) only on a genuine live convergence — cur.Status.Image is the
	// image the operator actually ran, whether resolved from a build or taken
	// straight from spec.image. Left "" on a failed/timed-out close: a deploy
	// that never went live is correctly never a valid rollback target.
	resolvedImage := ""
	if status == DeployLive {
		resolvedImage = cur.Status.Image
	}
	// w9/011: a failing close carries its actionable cause — the generation's
	// Ready-condition message when the operator surfaced a concrete defect
	// (crash loop with the $PORT hint, image-pull failure, build error), else a
	// synthesized health-gate-timeout line — so update_failed stops being an
	// opaque terminal state.
	failureReason := ""
	failureCode := ""
	switch status {
	case DeployBuildFailed, DeployPreDeployFailed, DeployUpdateFailed:
		failureReason, failureCode = failureReasonFor(cur)
	}
	ok, err := r.Store.TransitionDeploy(ctx, open.ID, status, resolvedImage, failureReason, failureCode)
	if err != nil {
		log.Printf("controlplane: transition deploy %s to %s: %v", open.ID, status, err)
		return
	}
	// Notify only the pass that actually closed the row (ok) — a race with a
	// concurrent Cancel must fire at most one notification for this deploy,
	// matching CloseDeploy's own idempotency guard. Backgrounded: NotifyDeploy
	// does per-recipient network I/O (an identity lookup + an SMTP send each),
	// which must not block ReconcileOnce's loop over every OTHER app in this
	// pass. context.WithoutCancel detaches from ctx's deadline — ctx is
	// ReconcileOnce's per-pass context and may already be gone by the time a
	// slow relay would otherwise get to send.
	if ok && r.DeployNotifier != nil && IsTerminalDeployStatus(status) && status != DeployCanceled {
		go r.DeployNotifier.NotifyDeploy(context.WithoutCancel(ctx), DeployNotification{
			TenantID:            d.TenantID,
			AppName:             d.Name,
			Status:              status,
			DeployID:            open.ID,
			CommitMessage:       open.CommitMessage,
			CommitSHA:           open.Commit,
			RepoURL:             cur.Spec.Repo,
			FailureReason:       failureReason,
			NotifyOnFail:        cur.Spec.NotifyOnFail,
			NotificationsToSend: cur.Spec.NotificationsToSend,
		})
	}
}

// recordLifecycleFacts emits the build and pre-deploy beats Render shows as
// distinct timeline entries (w7/m66), derived from the same observed App/deploy
// state recordDeploy already reads: build_started/build_ended for a repo-backed
// deploy, pre_deploy_started/pre_deploy_ended for a deploy whose current release
// runs a pre-deploy step. Every fact is idempotent by source_key, so re-observing
// the same phase across resyncs is a no-op; an image-backed deploy (no build
// phase) and a deploy with no pre-deploy step each emit neither. newStatus is the
// deploy status observedDeployStatus resolved this pass ("" when nothing moved).
func (r *Reconciler) recordLifecycleFacts(ctx context.Context, d DesiredApp, open Deploy, cur *appv1alpha1.App, newStatus string) {
	var facts []ServiceEventFact
	if d.Repo != "" {
		facts = append(facts, buildLifecycleFacts(open, newStatus)...)
	}
	facts = append(facts, preDeployLifecycleFacts(open, cur)...)
	for _, fact := range facts {
		if _, err := r.Store.InsertServiceEventFact(ctx, fact); err != nil {
			log.Printf("controlplane: record lifecycle fact %s: %v", fact.SourceKey, err)
		}
	}
}

// buildLifecycleFacts derives a repo-backed deploy's build_started and (once the
// build is over) build_ended facts from the deploy row's phase. The BuildKit Job
// is dispatched when the deploy row opens, so build_started rides open.CreatedAt;
// build_ended's outcome is read from where the deploy has reached — a build phase
// means still building (no ended fact yet), any later phase means the build
// succeeded, build_failed means it failed, and a cancel while still building
// means it was canceled.
func buildLifecycleFacts(open Deploy, newStatus string) []ServiceEventFact {
	facts := []ServiceEventFact{{
		SourceKey: "deploy:" + open.ID + ":build_started",
		AppID:     open.AppID,
		Type:      EventFactBuildStarted,
		At:        open.CreatedAt,
		DeployID:  open.ID,
		Image:     open.Image,
	}}
	if outcome := buildEndedStatus(open.Status, newStatus); outcome != "" {
		facts = append(facts, ServiceEventFact{
			SourceKey: "deploy:" + open.ID + ":build_ended",
			AppID:     open.AppID,
			Type:      EventFactBuildEnded,
			At:        time.Now().UTC(),
			DeployID:  open.ID,
			Image:     open.Image,
			Status:    outcome,
		})
	}
	return facts
}

// buildEndedStatus reports the outcome to stamp on a repo-backed deploy's
// build_ended fact, or "" while the build is still running. eff is where the
// deploy has reached this pass: the just-observed status, or the row's stored
// status when nothing transitioned.
func buildEndedStatus(current, newStatus string) string {
	eff := newStatus
	if eff == "" {
		eff = current
	}
	switch eff {
	case DeployBuildFailed:
		return EventStatusFailed
	case DeployCanceled:
		if isBuildPhase(current) {
			return EventStatusCanceled
		}
		return EventStatusSucceeded // build had finished before the cancel landed
	case DeployPreDeployInProgress, DeployPreDeployFailed,
		DeployUpdateInProgress, DeployUpdateFailed, DeployLive, DeployDeactivated:
		return EventStatusSucceeded
	default: // DeployCreated, DeployQueued, DeployBuildInProgress — still building
		return ""
	}
}

func isBuildPhase(status string) bool {
	return status == DeployCreated || status == DeployQueued || status == DeployBuildInProgress
}

// preDeployLifecycleFacts derives pre_deploy_started and pre_deploy_ended for a
// deploy whose current release runs a pre-deploy step. It reads the same
// status.preDeploy the reconciler projects onto the deploy row (preDeployStatusFor)
// plus its RFC3339 start/finish stamps, so the events line up with the deploy
// record. A deploy with no pre-deploy step (preDeployStatusFor == "") emits
// neither. The started fact is emitted alongside a terminal outcome so the pair
// still appears when a fast resync only ever samples the finished state.
func preDeployLifecycleFacts(open Deploy, cur *appv1alpha1.App) []ServiceEventFact {
	pds := preDeployStatusFor(cur)
	if pds == "" {
		return nil
	}
	started, finished := preDeployTimes(cur)
	facts := []ServiceEventFact{{
		SourceKey: "deploy:" + open.ID + ":pre_deploy_started",
		AppID:     open.AppID,
		Type:      EventFactPreDeployStarted,
		At:        started,
		DeployID:  open.ID,
	}}
	switch pds {
	case PreDeploySucceeded:
		facts = append(facts, preDeployEndedFact(open, finished, EventStatusSucceeded))
	case PreDeployFailed:
		facts = append(facts, preDeployEndedFact(open, finished, EventStatusFailed))
	}
	return facts
}

func preDeployEndedFact(open Deploy, at time.Time, status string) ServiceEventFact {
	return ServiceEventFact{
		SourceKey: "deploy:" + open.ID + ":pre_deploy_ended",
		AppID:     open.AppID,
		Type:      EventFactPreDeployEnded,
		At:        at,
		DeployID:  open.ID,
		Status:    status,
	}
}

// preDeployTimes lifts the CR's status.preDeploy RFC3339 start/finish stamps for
// the pre-deploy facts, falling back to now for an absent/unparseable value so a
// fact always carries a sortable timestamp.
func preDeployTimes(app *appv1alpha1.App) (started, finished time.Time) {
	now := time.Now().UTC()
	started, finished = now, now
	pd := app.Status.PreDeploy
	if pd == nil {
		return started, finished
	}
	if t, err := time.Parse(time.RFC3339Nano, pd.StartedAt); err == nil {
		started = t.UTC()
	}
	if t, err := time.Parse(time.RFC3339Nano, pd.FinishedAt); err == nil {
		finished = t.UTC()
	}
	return started, finished
}

// observedImagePullFailure turns the operator's current-generation diagnosis
// into an event immediately. The deploy itself keeps its existing retry and
// health-gate timeout policy; making the event wait for that timeout would hide
// a known pull failure for minutes. The source key is shared with the terminal
// transition path, so a later close remains idempotent.
func observedImagePullFailure(open Deploy, app *appv1alpha1.App) (ServiceEventFact, bool) {
	if generation := appReleaseGeneration(app); generation != 0 && open.Generation != 0 && generation != open.Generation {
		return ServiceEventFact{}, false
	}
	for i := range app.Status.Conditions {
		condition := &app.Status.Conditions[i]
		if condition.Type != appv1alpha1.ConditionReady || condition.ObservedGeneration != app.Generation || condition.Reason != "ImagePullBackOff" {
			continue
		}
		at := condition.LastTransitionTime.Time
		if at.IsZero() {
			at = time.Now().UTC()
		}
		image := open.Image
		if image == "" {
			image = app.Spec.Image
		}
		return ServiceEventFact{
			SourceKey:  "deploy:" + open.ID + ":image_pull_failed",
			AppID:      open.AppID,
			Type:       EventFactImagePullFailed,
			At:         at,
			DeployID:   open.ID,
			Image:      image,
			ReasonCode: EventReasonImagePullBackoff,
		}, true
	}
	return ServiceEventFact{}, false
}

// deployTimedOut applies the budget for the deploy row's last observed phase.
// Using UpdatedAt resets the clock on each legal transition: a six-minute build
// that has just entered rollout receives the full rollout health-gate window
// instead of inheriting six minutes of elapsed build time.
func (r *Reconciler) deployTimedOut(d DesiredApp, open Deploy) bool {
	timeout := r.DeployGateTimeout
	switch open.Status {
	case DeployCreated:
		if d.Repo != "" {
			timeout = r.BuildGateTimeout
		}
	case DeployQueued, DeployBuildInProgress:
		timeout = r.BuildGateTimeout
	case DeployPreDeployInProgress:
		timeout = r.PreDeployGateTimeout
	}

	phaseStarted := open.UpdatedAt
	if phaseStarted.IsZero() {
		phaseStarted = open.CreatedAt
	}
	return time.Since(phaseStarted) > timeout
}

// observedDeployStatus maps only evidence for the open row's release generation
// to a Render lifecycle state. Kubernetes metadata generation can advance for
// an operational update (for example manual scale) while the same release is
// building or rolling; status.releaseGeneration keeps that harmless churn from
// orphaning the deploy row. Legacy operators fall back to metadata generation.
func observedDeployStatus(open Deploy, app *appv1alpha1.App, timedOut bool) string {
	if status, settled := supersededDeployStatus(open, app, timedOut); settled {
		return status
	}
	switch preDeployStatusFor(app) {
	case PreDeployRunning:
		if timedOut {
			return DeployPreDeployFailed
		}
		return DeployPreDeployInProgress
	case PreDeployFailed:
		return DeployPreDeployFailed
	}

	reason, conditionCurrent := readyReasonForGeneration(app)
	switch app.Status.Phase {
	case appv1alpha1.PhaseBuilding:
		if conditionCurrent {
			switch {
			case timedOut:
				return DeployBuildFailed
			case reason == "BuildQueued":
				return DeployQueued
			default:
				return DeployBuildInProgress
			}
		}
	case appv1alpha1.PhaseDeploying:
		if conditionCurrent {
			if timedOut {
				return DeployUpdateFailed
			}
			return DeployUpdateInProgress
		}
	case appv1alpha1.PhaseRunning:
		if releaseIsActive(open, app) {
			return DeployLive
		}
	case appv1alpha1.PhaseFailed:
		if conditionCurrent {
			switch reason {
			case "BuildFailed":
				return DeployBuildFailed
			case "PreDeployFailed":
				return DeployPreDeployFailed
			default:
				return DeployUpdateFailed
			}
		}
	}

	if !timedOut {
		return ""
	}
	return timedOutDeployStatus(open)
}

// supersededDeployStatus decides the open row's fate when the CR's release
// generation no longer matches it. settled=false means the generations agree and
// the caller should read live phase evidence instead.
func supersededDeployStatus(open Deploy, app *appv1alpha1.App, timedOut bool) (string, bool) {
	// A reported release generation PAST the open row's means this row's release
	// was superseded before it could finish — a git-push redeploy, env-var
	// change, or restart minted a newer release identity without adopting the
	// row. Generations are monotonic, so the row can never advance again; close
	// it canceled, the CR-side mirror of CreateDeploy's newest-wins cancel.
	// Only status.releaseGeneration is trusted here: the legacy metadata-
	// generation fallback below also moves for operational churn (manual
	// scale), which must keep waiting, not cancel a live rollout's row.
	if app.Status.ReleaseGeneration > 0 && open.Generation != 0 && app.Status.ReleaseGeneration > open.Generation {
		return DeployCanceled, true
	}
	generation := appReleaseGeneration(app)
	if generation == 0 || open.Generation == 0 || generation == open.Generation {
		return "", false
	}
	// The CR's release generation no longer matches this open row. Usually a
	// brief transient while the operator catches up to a just-patched spec, so
	// keep waiting. But once the row has sat past its gate timeout the match
	// will never arrive: if the App CR was deleted and recreated — e.g. the
	// ADR043 per-tenant-namespace migration re-applying every CR — its
	// status.releaseGeneration restarts BELOW this row's stored generation, so
	// the release can never be adopted and the row would otherwise report
	// "in progress" forever (a healthy PhaseRunning App never trips the
	// phase-switch timeout below). Close it canceled — the terminal the
	// superseded case above already uses — but only for a release-generation-
	// aware operator (status.releaseGeneration > 0); a legacy operator's
	// metadata generation also moves for operational churn (manual scale), so
	// it keeps the conservative wait rather than risk canceling a live rollout.
	if timedOut && app.Status.ReleaseGeneration > 0 {
		return DeployCanceled, true
	}
	return "", true
}

// timedOutDeployStatus fails an open row out in the phase it was last known to
// be in, once no live evidence has moved it before the gate timeout.
func timedOutDeployStatus(open Deploy) string {
	switch open.Status {
	case DeployQueued, DeployBuildInProgress:
		return DeployBuildFailed
	case DeployPreDeployInProgress:
		return DeployPreDeployFailed
	default:
		return DeployUpdateFailed
	}
}

func appReleaseGeneration(app *appv1alpha1.App) int64 {
	if app.Status.ReleaseGeneration > 0 {
		return app.Status.ReleaseGeneration
	}
	return app.Generation
}

func releaseIsActive(open Deploy, app *appv1alpha1.App) bool {
	if app.Status.ReleaseGeneration > 0 {
		return app.Status.ReleaseGeneration == open.Generation &&
			app.Status.ActiveRevision == fmt.Sprintf("rev-%d", open.Generation)
	}
	return app.Status.ObservedGeneration == app.Generation
}

// failureReasonFor picks what to stamp as a failing deploy's failure_reason
// (w9/011): the current generation's Ready-condition message when the operator
// diagnosed a concrete defect (crash loop, image pull, unresolvable container
// config, build or pre-deploy failure — their messages are written to be
// user-actionable), the raw condition message for any other PhaseFailed, and a
// synthesized timeout line when the condition proves nothing (a pure
// health-gate-timeout close).
//
// CreateContainerConfigError was the gap w7/m79 closed. A pod whose Secret or
// ConfigMap reference cannot be resolved never starts, so its App stays in
// PhaseDeploying with a Ready=False condition the switch below did not
// recognise — and the deploy therefore closed with the generic
// "did not become healthy within the health-gate window" line while the
// operator already knew the exact missing object. That is the 2026-08-08
// incident's first failure leg: the cause existed, in the right place, and was
// thrown away one step before the user could see it.
func failureReasonFor(app *appv1alpha1.App) (string, string) {
	for i := range app.Status.Conditions {
		c := &app.Status.Conditions[i]
		if c.Type != appv1alpha1.ConditionReady || c.ObservedGeneration != app.Generation {
			continue
		}
		switch c.Reason {
		case "ImagePullBackOff":
			return c.Message, EventReasonImagePullBackoff
		case "CrashLoopBackOff", "CreateContainerConfigError", "BuildFailed", "PreDeployFailed":
			return c.Message, ""
		}
		if app.Status.Phase == appv1alpha1.PhaseFailed && c.Message != "" {
			return c.Message, ""
		}
	}
	return "the deploy did not become healthy within the health-gate window; check the service logs", ""
}

// concreteContainerFailure reports whether a Ready=False reason names a defect
// in the current revision's containers, as opposed to a rollout still making
// ordinary progress. Only these count as a truthful failed-instance observation
// while a deploy is still open and retrying.
//
// CreateContainerConfigError belongs here for the same reason the other two do:
// a pod that cannot resolve its Secret/ConfigMap reference will never start, so
// treating it as ordinary progress means the instance is silently unobserved
// for the whole health-gate window. Transience is handled one layer up by
// debounceUnhealthy, which requires two consecutive passes — so a Secret that
// simply has not been materialised yet costs one tick, not a false alarm.
func concreteContainerFailure(reason string) bool {
	switch reason {
	case "ImagePullBackOff", "CrashLoopBackOff", "CreateContainerConfigError":
		return true
	}
	return false
}

func readyReasonForGeneration(app *appv1alpha1.App) (string, bool) {
	for i := range app.Status.Conditions {
		ready := &app.Status.Conditions[i]
		if ready.Type == appv1alpha1.ConditionReady && ready.ObservedGeneration == app.Generation {
			return ready.Reason, true
		}
	}
	return "", false
}

func observedServiceStateFor(appID string, app *appv1alpha1.App, hasOpenDeploy bool) ObservedServiceState {
	obs := ObservedServiceState{
		AppID:        appID,
		At:           time.Now().UTC(),
		ServicePhase: string(app.Status.Phase),
	}
	if app.Status.Phase == appv1alpha1.PhaseHibernated {
		obs.AvailabilityObserved = true
		return obs
	}
	for i := range app.Status.Conditions {
		condition := &app.Status.Conditions[i]
		if condition.Type != appv1alpha1.ConditionReady {
			continue
		}
		switch condition.Status {
		case metav1.ConditionTrue:
			if app.Status.Phase == appv1alpha1.PhaseRunning {
				obs.Availability = "healthy"
				obs.AvailabilityObserved = true
			}
		case metav1.ConditionFalse:
			// Ordinary rollout progress is not a service failure. A concrete
			// current-revision container failure is still a truthful failed
			// instance observation even while its deploy remains open/retrying.
			if hasOpenDeploy && !concreteContainerFailure(condition.Reason) {
				break
			}
			// RolloutSettling = the operator's live pod scan says the full
			// current-revision complement is ready and only the Deployment's
			// status bookkeeping lags (old-pod reap, KCM catch-up) — the
			// service is serving, so it is excluded from availability like
			// Suspended/AutoHibernated (w3/m78 live-leg finding).
			if app.Status.ActiveRevision != "" && condition.Reason != "Suspended" &&
				condition.Reason != "AutoHibernated" && condition.Reason != "RolloutSettling" {
				obs.Availability = "unhealthy"
				obs.AvailabilityObserved = true
				obs.ReasonCode = EventReasonReadinessFailed
			}
		}
		break
	}
	return obs
}

func (r *Reconciler) recordAutoscalingFacts(ctx context.Context, appID string, transition *appv1alpha1.AutoscalingStatus) {
	if transition == nil || transition.TransitionID == "" {
		return
	}
	startedAt, err := time.Parse(time.RFC3339Nano, transition.StartedAt)
	if err != nil {
		return
	}
	from, to := transition.FromReplicas, transition.ToReplicas
	facts := []ServiceEventFact{{
		SourceKey: fmt.Sprintf("autoscaling:%s:%s:started", appID, transition.TransitionID),
		AppID:     appID, Type: EventFactAutoscalingStarted, At: startedAt,
		FromCount: &from, ToCount: &to,
	}}
	if transition.State == appv1alpha1.AutoscalingTransitionEnded {
		if finishedAt, parseErr := time.Parse(time.RFC3339Nano, transition.FinishedAt); parseErr == nil {
			facts = append(facts, ServiceEventFact{
				SourceKey: fmt.Sprintf("autoscaling:%s:%s:ended", appID, transition.TransitionID),
				AppID:     appID, Type: EventFactAutoscalingEnded, At: finishedAt,
				FromCount: &from, ToCount: &to,
			})
		}
	}
	for _, fact := range facts {
		if _, err := r.Store.InsertServiceEventFact(ctx, fact); err != nil {
			log.Printf("controlplane: record autoscaling fact %s: %v", fact.SourceKey, err)
		}
	}
}

// preDeployStatusFor maps the App CR's pre-deploy step status (status.preDeploy,
// set by the operator) to the deploy row's lowercase pre_deploy_status
// vocabulary, but only for the current release generation — a status left over
// from a superseded release must not be projected onto the open deploy. Empty
// means no pre-deploy step applies to this rollout.
func preDeployStatusFor(app *appv1alpha1.App) string {
	pd := app.Status.PreDeploy
	if pd == nil || pd.Generation != appReleaseGeneration(app) {
		return ""
	}
	switch pd.Status {
	case appv1alpha1.PreDeployRunning:
		return PreDeployRunning
	case appv1alpha1.PreDeploySucceeded:
		return PreDeploySucceeded
	case appv1alpha1.PreDeployFailed:
		return PreDeployFailed
	default:
		return ""
	}
}

// CRName is the projected CR's name, "<tenant-id>-<app>". Both parts are
// API-validated DNS labels of ≤30 chars, so the result always fits the
// 63-char object-name limit. Delegates to core.CRName — the apps feature's
// create path (w4/m19) computes the identical name for the same row, so
// whichever side (bex-api's direct Create or this reconciler's own
// fallback-create) gets there first, the other recognizes the CR by its
// LabelAppID rather than re-creating it under a different name.
func CRName(tenant, app string) string { return core.CRName(tenant, app) }

// ManagedAppID returns the control-plane app id a projected App CR's labels
// carry, or "" when the labels don't prove bex-controlplane ownership (a
// hand-applied CR with no Postgres source row to key a fact/row on). One helper
// so every feature that reads the app id off a CR does it the same way.
func ManagedAppID(labels map[string]string) string {
	if labels[LabelManagedBy] != ManagedByValue {
		return ""
	}
	return labels[LabelAppID]
}

// stampLabels ensures cur carries the projection's labels (managed-by, app-id,
// tenant-id, public name), returning whether any changed. Called on both
// Create and Update so a LabelTenant value change (name → id) converges on the
// next resync — and, for any App still on its pre-w4/m19 bare object name,
// this is what backfills LabelServiceName without ever renaming the object
// (core.GetApp's cross-workspace fallback needs the label to find it).
func stampLabels(cur *appv1alpha1.App, d DesiredApp) bool {
	if cur.Labels == nil {
		cur.Labels = map[string]string{}
	}
	changed := false
	set := func(k, v string) {
		if cur.Labels[k] != v {
			cur.Labels[k] = v
			changed = true
		}
	}
	set(LabelManagedBy, ManagedByValue)
	set(LabelAppID, d.ID)
	set(LabelTenant, d.TenantID)       // the tenant id (tea-<id>), what List/Get filter on
	set(LabelWorkspace, d.TenantID)    // workspace identity for NetworkPolicy selectors (t002)
	set(core.LabelServiceName, d.Name) // the public name — see core.GetApp's cross-workspace fallback
	setOptional := func(k, v string) {
		if v == "" {
			if _, ok := cur.Labels[k]; ok {
				delete(cur.Labels, k)
				changed = true
			}
			return
		}
		set(k, v)
	}
	setOptional(core.LabelProject, d.ProjectID)
	setOptional(core.LabelEnvironment, d.EnvironmentID)
	return changed
}

// projectApp builds the App CR for a desired row. When r.CloneSecrets is set
// and the row has a Repo, it mints the clone Secret so the first build from a
// private repo authenticates without a separate API trigger. Minting errors are
// soft (logged, CR still created) so a GitHub outage never blocks projection.
func (r *Reconciler) projectApp(ctx context.Context, d DesiredApp) *appv1alpha1.App {
	ns := namespaceFor(d)
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRName(d.TenantID, d.Name),
			Namespace: ns,
		},
		Spec: projectSpec(d),
	}
	stampLabels(a, d)
	if r.CloneSecrets != nil && d.Repo != "" {
		if secretName, err := r.CloneSecrets.EnsureCloneSecret(ctx, ns, a.Name, d.TenantID, d.Repo); err != nil {
			log.Printf("controlplane: clone secret for %s: %v (proceeding without)", a.Name, err)
		} else {
			a.Spec.CloneSecret = secretName
		}
	}
	return a
}

// projectSpec maps a row to the App.spec fields the control plane owns.
// Expose is always set so the operator serves the app at the platform
// hostname (<name>.<BEX_BASE_DOMAIN>) even before a custom domain exists.
func projectSpec(d DesiredApp) appv1alpha1.AppSpec {
	s := appv1alpha1.AppSpec{
		Repo:                 d.Repo,
		Image:                d.Image,
		RegistryCredentialID: copyStringPtr(d.RegistryCredentialID),
		Branch:               d.Branch,
		Replicas:             d.Replicas,
		Port:                 d.Port,
		Tier:                 d.Tier,
		IdleTTLSeconds:       d.IdleTTLSeconds,
		Suspended:            d.Suspended,
		Expose:               true,
		Subdomain:            d.Slug,
	}
	s.Host = d.PrimaryHost
	s.Hosts = slices.Clone(d.Hosts)
	s.HostRedirects = maps.Clone(d.HostRedirects)
	// The environment inbound-IP layer (w4/m28): stamped on CREATE so a
	// projector-recreated member CR never rejoins its environment without
	// enforcement; deliberately absent from applyOwnedSpec (updates belong to
	// the environments fan-out, which also handles the deny-all projection).
	s.EnvironmentIPAllowList = slices.Clone(d.EnvironmentIPAllowList)
	if d.RegistryCredentialID != nil && *d.RegistryCredentialID != "" {
		// Explicit credentials materialize to a deterministic Secret name. Keep
		// this in the desired spec (not only projectApp) so every later resync
		// preserves the reference instead of treating it as stale owned state.
		s.ExternalRegistryPullSecret = CRName(d.TenantID, d.Name) + "-registry-pull"
	}
	return s
}

// applyOwnedSpec copies the control-plane-owned fields of want onto dst and
// reports whether anything changed. Fields it doesn't own (Builder,
// HealthCheckPath, AutoDeploy, RestartedAt, …) are left untouched so
// bex-api's lifecycle verbs and server-side defaulting survive a resync.
func applyOwnedSpec(dst *appv1alpha1.AppSpec, want appv1alpha1.AppSpec) bool {
	changed := false
	set := func(cur *string, want string) {
		if *cur != want {
			*cur = want
			changed = true
		}
	}
	set(&dst.Repo, want.Repo)
	set(&dst.Image, want.Image)
	if !equalStringPtrs(dst.RegistryCredentialID, want.RegistryCredentialID) {
		dst.RegistryCredentialID = copyStringPtr(want.RegistryCredentialID)
		changed = true
	}
	// A non-nil binding is explicit, so the projector owns the corresponding
	// deterministic Secret reference (including explicit-empty => clear). nil
	// retains legacy host auto-resolution, whose reference is API-managed.
	if want.RegistryCredentialID != nil && dst.ExternalRegistryPullSecret != want.ExternalRegistryPullSecret {
		dst.ExternalRegistryPullSecret = want.ExternalRegistryPullSecret
		changed = true
	}
	set(&dst.Branch, want.Branch)
	set(&dst.Tier, want.Tier)
	set(&dst.Host, want.Host)
	set(&dst.Subdomain, want.Subdomain)
	if dst.Replicas != want.Replicas {
		dst.Replicas, changed = want.Replicas, true
	}
	if dst.Port != want.Port {
		dst.Port, changed = want.Port, true
	}
	if dst.IdleTTLSeconds != want.IdleTTLSeconds {
		dst.IdleTTLSeconds, changed = want.IdleTTLSeconds, true
	}
	if dst.Suspended != want.Suspended {
		dst.Suspended, changed = want.Suspended, true
	}
	if dst.Expose != want.Expose {
		dst.Expose, changed = want.Expose, true
	}
	if !slices.Equal(dst.Hosts, want.Hosts) {
		dst.Hosts, changed = slices.Clone(want.Hosts), true
	}
	if !maps.Equal(dst.HostRedirects, want.HostRedirects) {
		dst.HostRedirects, changed = maps.Clone(want.HostRedirects), true
	}
	return changed
}

func copyStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func equalStringPtrs(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
