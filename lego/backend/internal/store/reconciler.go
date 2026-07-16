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
	// defaultDeployGateTimeout bounds how long a deploy may sit open
	// (update_in_progress) before recordDeploy gives up on it and calls it
	// failed. Needed because a bad image (ImagePullBackOff) never makes the
	// App CR's own phase machine reach PhaseFailed — it polls PhaseDeploying
	// forever (lego/operator/internal/controller/app_controller.go) — so
	// health gating (docs/ADR004-deployment.md) needs its own timeout, not just the
	// CR's phase, to ever report a deploy as failed.
	defaultDeployGateTimeout = 3 * time.Minute
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
	Client    client.Client
	Store     Store
	Namespace string        // namespace the App CRs are projected into
	Resync    time.Duration // full-resync interval
	// DeployGateTimeout bounds how long a deploy may stay open before
	// recordDeploy closes it as failed even though the CR's phase never
	// reached Failed on its own (see defaultDeployGateTimeout).
	DeployGateTimeout time.Duration
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

	kick chan struct{}
}

func NewReconciler(cl client.Client, store Store, namespace string) *Reconciler {
	return &Reconciler{
		Client:            cl,
		Store:             store,
		Namespace:         namespace,
		Resync:            defaultResyncPeriod,
		DeployGateTimeout: defaultDeployGateTimeout,
		kick:              make(chan struct{}, 1),
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
	var existing appv1alpha1.AppList
	if err := r.Client.List(ctx, &existing,
		client.InNamespace(r.Namespace),
		client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		return fmt.Errorf("list App CRs: %w", err)
	}
	byID := make(map[string]*appv1alpha1.App, len(existing.Items))
	for i := range existing.Items {
		if id := existing.Items[i].Labels[LabelAppID]; id != "" {
			byID[id] = &existing.Items[i]
		}
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
		if open, ok := openByApp[d.ID]; ok {
			r.recordDeploy(ctx, d, open, cur)
		}
	}
	// Rows deleted from Postgres → delete their projected CR.
	for id, cur := range byID {
		if seen[id] {
			continue
		}
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

	status := observedDeployStatus(open, cur, time.Since(open.CreatedAt) > r.DeployGateTimeout)
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
	ok, err := r.Store.TransitionDeploy(ctx, open.ID, status, resolvedImage)
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
			NotifyOnFail:        cur.Spec.NotifyOnFail,
			NotificationsToSend: cur.Spec.NotificationsToSend,
		})
	}
}

// observedDeployStatus maps only current-generation evidence to a Render
// lifecycle state. Empty means the sample proves no new state. The Ready
// condition is generation-scoped by the operator's setPhase/fail helpers; that
// guard prevents a freshly opened row from inheriting the prior revision's
// Building/Deploying/Failed status before the operator has touched it.
func observedDeployStatus(open Deploy, app *appv1alpha1.App, timedOut bool) string {
	if app.Generation != 0 && open.Generation != 0 && app.Generation != open.Generation {
		return ""
	}
	pds := preDeployStatusFor(app)
	switch pds {
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
		if conditionCurrent && timedOut {
			return DeployBuildFailed
		}
		if conditionCurrent && reason == "BuildQueued" {
			return DeployQueued
		}
		if conditionCurrent {
			return DeployBuildInProgress
		}
	case appv1alpha1.PhaseDeploying:
		if conditionCurrent {
			if timedOut {
				return DeployUpdateFailed
			}
			return DeployUpdateInProgress
		}
	case appv1alpha1.PhaseRunning:
		if app.Status.ObservedGeneration == app.Generation {
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
	switch open.Status {
	case DeployQueued, DeployBuildInProgress:
		return DeployBuildFailed
	case DeployPreDeployInProgress:
		return DeployPreDeployFailed
	default:
		return DeployUpdateFailed
	}
}

func readyReasonForGeneration(app *appv1alpha1.App) (string, bool) {
	for i := range app.Status.Conditions {
		ready := &app.Status.Conditions[i]
		if ready.Type == "Ready" && ready.ObservedGeneration == app.Generation {
			return ready.Reason, true
		}
	}
	return "", false
}

// preDeployStatusFor maps the App CR's pre-deploy step status (status.preDeploy,
// set by the operator) to the deploy row's lowercase pre_deploy_status
// vocabulary, but only for the CR's CURRENT generation — a status left over from
// a superseded revision must not be projected onto the open deploy. Empty means
// no pre-deploy step applies to this rollout.
func preDeployStatusFor(app *appv1alpha1.App) string {
	pd := app.Status.PreDeploy
	if pd == nil || pd.Generation != app.Generation {
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

// CRName is the projected CR's name, "<tenant>-<app>". Both parts are
// API-validated DNS labels of ≤30 chars, so the result always fits the
// 63-char object-name limit. Delegates to core.CRName — the apps feature's
// create path (w4/m19) computes the identical name for the same row, so
// whichever side (bex-api's direct Create or this reconciler's own
// fallback-create) gets there first, the other recognizes the CR by its
// LabelAppID rather than re-creating it under a different name.
func CRName(tenant, app string) string { return core.CRName(tenant, app) }

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
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRName(d.TenantName, d.Name),
			Namespace: r.Namespace,
		},
		Spec: projectSpec(d),
	}
	stampLabels(a, d)
	if r.CloneSecrets != nil && d.Repo != "" {
		if secretName, err := r.CloneSecrets.EnsureCloneSecret(ctx, r.Namespace, a.Name, d.TenantID, d.Repo); err != nil {
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
	if d.RegistryCredentialID != nil && *d.RegistryCredentialID != "" {
		// Explicit credentials materialize to a deterministic Secret name. Keep
		// this in the desired spec (not only projectApp) so every later resync
		// preserves the reference instead of treating it as stale owned state.
		s.ExternalRegistryPullSecret = CRName(d.TenantName, d.Name) + "-registry-pull"
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
