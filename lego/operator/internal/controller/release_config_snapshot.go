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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// release_config_snapshot.go makes a release's CONFIGURATION restorable, which
// release identity never did: a fingerprint proves inputs changed, it is not a
// value you can put back (w1/m152).
//
// Before this, every configuration source a pod read was mutable and shared. A
// service's own `<name>-env` / `<name>-files` Secret is rewritten in place on
// every save, and a linked group's `<evg-id>-env` / `<evg-id>-files` is rewritten
// for every service linked to it. The pod template referenced them by NAME and
// kubelet resolved contents at pod creation, so a save changed what a running
// release would serve on its next pod creation, canceling the deploy that carried
// the save had nothing to restore, and a crash-restarted pod of the old release
// came up with the new values.
//
// Each dispatched release now copies every source it reads into an immutable
// per-release Secret, and the pod template references those copies.
//
// Two shape decisions, both load-bearing, recorded in
// docs/ADR004-app-deployment.md §Per-release configuration snapshots:
//
//   - ONE SNAPSHOT PER SOURCE, NEVER FLATTENED. maxSecretMapBytes (512 KiB) is a
//     per-map quota shared by the service and env-group paths, set under
//     Kubernetes' 1 MiB Secret ceiling so "a map that passes the quota can always
//     materialize". Flattening a 512 KiB service map with two 512 KiB group maps
//     reaches ~1.5 MiB while every input passed its own quota — recreating the
//     source/projection divergence that quota exists to prevent. A per-source copy
//     inherits the guarantee by construction.
//   - PRECEDENCE IS NEVER RE-DERIVED. The envFrom list and the projected-volume
//     source list keep their existing order with snapshot names substituted, so
//     "groups first, the service's own last" stays a property of list order and
//     the existing precedence tests keep covering it.
const (
	// releaseSnapshotRetention is how many recent release generations keep their
	// snapshots. It matches deploys.eligibilityScanLimit, which is what the product
	// actually OFFERS as a rollback target — Rollback's verb accepts any named
	// deployId with no age window, so retention cannot match the verb. Every
	// rollback the product offers therefore restores configuration exactly; an
	// explicitly named older target restores the image only, and says so (t009).
	releaseSnapshotRetention = 20

	// snapshotOfLabel records which source a snapshot copies, and
	// snapshotGenerationLabel its release generation, so garbage collection can
	// select an App's snapshots without parsing names.
	snapshotOfLabel         = "app.bex.co/config-snapshot-of"
	snapshotGenerationLabel = "app.bex.co/config-snapshot-generation"
)

// snapshotOrSource maps a mutable source name to the snapshot of whichever
// generation the App is currently projecting, and returns it unchanged otherwise.
// Every projection site goes through here so a single App can never mix snapshot
// and mutable references — which would make precedence depend on which source
// happened to be copied.
//
// It keys on ConfigSnapshotGeneration ALONE, not on that field matching
// ReleaseGeneration, which is what lets a cancel settle back onto an EARLIER
// release's configuration (w1/m152 t002): the canceled branch points it at the
// last served generation, and the template reverts image and config together.
// Zero means the App has not rolled out a release since snapshots shipped; see
// servingWithoutSnapshots for why such an App keeps its mutable names.
func snapshotOrSource(app *appv1alpha1.App, source string) string {
	if source == "" || app.Status.ConfigSnapshotGeneration <= 0 {
		return source
	}
	return snapshotName(app, source, app.Status.ConfigSnapshotGeneration)
}

// snapshotName is app's snapshot of source at gen: the scoped name, except for the
// one generation still projecting the first build's unscoped group copies.
func snapshotName(app *appv1alpha1.App, source string, gen int64) string {
	if gen == app.Status.UnscopedSnapshotGeneration {
		return appv1alpha1.ReleaseSnapshotName(source, gen)
	}
	return appv1alpha1.AppReleaseSnapshotName(app.Name, source, gen)
}

// inFlightRelease is the App as its in-flight release's own steps — the native
// build and the pre-deploy command — must see it: reading the saved configuration
// sources directly. Those steps run before the release is snapshotted, while the
// projection still points at the SERVED generation's copies, so projecting the App
// itself handed release N+1's migration release N's DATABASE_URL (w1/m152 t004).
// Reading the mutable sources is exactly what these steps did before snapshots.
func inFlightRelease(app *appv1alpha1.App) *appv1alpha1.App {
	release := app.DeepCopy()
	release.Status.ConfigSnapshotGeneration = 0
	return release
}

// settleConfigSnapshotTo points the projection at an earlier release's snapshot,
// for the canceled path. It refuses to point at a generation whose snapshot is no
// longer on the cluster — GC keeps only the retained window, and referencing a
// reclaimed copy would leave pods with an unresolvable optional Secret, silently
// dropping the release's environment. Reports whether the projection moved.
func (r *AppReconciler) settleConfigSnapshotTo(ctx context.Context, app *appv1alpha1.App, gen int64) (bool, error) {
	if gen <= 0 || app.Status.ConfigSnapshotGeneration == gen {
		return false, nil
	}
	for _, source := range releaseConfigSources(app) {
		name := snapshotName(app, source, gen)
		err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, &corev1.Secret{})
		if apierrors.IsNotFound(err) {
			// No snapshot for that release: it predates t001 or has been reclaimed.
			// Leave the projection alone — restoring the image only is the honest
			// outcome, and it is what happened before this milestone.
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
	app.Status.ConfigSnapshotGeneration = gen
	return true, nil
}

// releaseConfigSources lists every Secret whose contents a pod of this App reads,
// in no particular order — the projection owns ordering, this owns membership.
// Snapshot names themselves are excluded, so re-reconciling an App whose template
// already references snapshots never snapshots a snapshot.
func releaseConfigSources(app *appv1alpha1.App) []string {
	var out []string
	add := func(name string) {
		if name == "" || appv1alpha1.IsReleaseSnapshotName(name) {
			return
		}
		if slices.Contains(out, name) {
			return
		}
		out = append(out, name)
	}
	for _, name := range app.Spec.EnvFromSecrets {
		add(name)
	}
	add(app.Spec.EnvFromSecret)
	add(app.Annotations[appv1alpha1.PendingEnvSecretAnnotation])
	for _, name := range app.Spec.FilesFromSecrets {
		add(name)
	}
	add(app.Annotations[appv1alpha1.PendingFilesSecretAnnotation])
	return out
}

// ensureReleaseConfigSnapshot copies every configuration source of the release
// being dispatched into an immutable per-release Secret, then records the
// generation on the App's status so the projection starts referencing them.
//
// It is idempotent and copy-once: an existing snapshot for a generation is never
// rewritten, because its whole purpose is to hold the values that release ran
// with. A source that has since been deleted is skipped rather than failing the
// release — the release already ran without it, and failing here would wedge a
// deploy on a group the user removed.
//
// The generation is PERSISTED here rather than left to ride a later status write
// in the same pass. That ordering is load-bearing: the projection flips on this
// field, so if it were only set in memory the Deployment would reference
// `<source>-r<gen>` while the stored status still said 0, and the NEXT reconcile
// would project the mutable name again — flapping the pod template, and every pod
// with it, on every pass. Writing first means the flip is durable before anything
// references it, and a write failure leaves both the status and the template on
// the pre-snapshot names, which is a correct state rather than a mixed one.
func (r *AppReconciler) ensureReleaseConfigSnapshot(ctx context.Context, app *appv1alpha1.App) error {
	if ref := app.ActiveReleaseConfig(); ref != nil && ref.SourceGeneration > 0 && !canceledOverServed(app) {
		return r.ensureSelectedReleaseConfig(ctx, app, ref)
	}
	return r.ensureSavedReleaseConfigSnapshot(ctx, app)
}

// ensureSelectedConfigBeforePlanning validates/materializes a Deployment's
// historical inputs before the target instance count is used. It runs each
// reconcile, so a lost selected Secret is never hidden by persisted status.
func (r *AppReconciler) ensureSelectedConfigBeforePlanning(ctx context.Context, app *appv1alpha1.App) error {
	if ref := app.ActiveReleaseConfig(); ref != nil && ref.SourceGeneration > 0 && !canceledOverServed(app) {
		return r.ensureSelectedReleaseConfig(ctx, app, ref)
	}
	return nil
}

// ensureSavedReleaseConfigSnapshot runs after the pre-deploy gate. Historical
// inputs were checked before planning and must not be copied or checked twice
// in this pass. Cron jobs use ensureReleaseConfigSnapshot once instead.
func (r *AppReconciler) ensureSavedReleaseConfigSnapshot(ctx context.Context, app *appv1alpha1.App) error {
	if ref := app.ActiveReleaseConfig(); ref != nil && ref.SourceGeneration > 0 && !canceledOverServed(app) {
		return nil
	}
	if err := r.adoptUnscopedSnapshots(ctx, app); err != nil {
		return err
	}
	gen := app.Status.ReleaseGeneration
	if gen <= 0 || app.Status.ConfigSnapshotGeneration == gen || servingWithoutSnapshots(app) {
		return nil
	}
	for _, source := range releaseConfigSources(app) {
		if err := r.copyConfigSecret(ctx, app, source, gen); err != nil {
			return fmt.Errorf("snapshot %s for generation %d: %w", source, gen, err)
		}
	}
	app.Status.ConfigSnapshotGeneration = gen
	if err := updateStatusIfChanged(ctx, r.Client, app); err != nil {
		// Roll the in-memory flip back so this pass projects the mutable names it
		// can still resolve, instead of snapshot names the stored status denies.
		app.Status.ConfigSnapshotGeneration = 0
		return fmt.Errorf("record config snapshot generation %d: %w", gen, err)
	}
	return nil
}

// adoptUnscopedSnapshots handles an App left projecting the first build's
// unscoped group copies (`<evg-id>-env-r<gen>`; see AppReleaseSnapshotName). Its
// pods read those names today. Switching them to scoped names would roll every
// group-linked service with no deploy — which the milestone's "no fleet-wide
// roll" decision rules out. So the App keeps projecting them: the generation is
// marked unscoped on status, and the App becomes a co-owner of each copy it reads,
// so the service that created the copy cannot garbage-collect or cascade-delete it
// out from under this one. The App's next release is snapshotted under scoped
// names and leaves this path.
func (r *AppReconciler) adoptUnscopedSnapshots(ctx context.Context, app *appv1alpha1.App) error {
	gen := app.Status.ConfigSnapshotGeneration
	if gen <= 0 || gen == app.Status.UnscopedSnapshotGeneration {
		return nil
	}
	unscoped := false
	for _, source := range releaseConfigSources(app) {
		legacy := appv1alpha1.ReleaseSnapshotName(source, gen)
		if legacy == appv1alpha1.AppReleaseSnapshotName(app.Name, source, gen) {
			continue // the App's own source: never unscoped
		}
		scoped := &corev1.Secret{}
		err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.AppReleaseSnapshotName(app.Name, source, gen)}, scoped)
		if err == nil {
			continue
		}
		if !apierrors.IsNotFound(err) {
			return err
		}
		copyOf := &corev1.Secret{}
		err = r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: legacy}, copyOf)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}
		unscoped = true
		if metav1.IsControlledBy(copyOf, app) {
			continue
		}
		if err := controllerutil.SetOwnerReference(app, copyOf, r.Scheme); err != nil {
			return err
		}
		if err := r.uncachedSecretClient().Update(ctx, copyOf); err != nil {
			return fmt.Errorf("co-own unscoped snapshot %s: %w", legacy, err)
		}
	}
	if !unscoped {
		return nil
	}
	app.Status.UnscopedSnapshotGeneration = gen
	return updateStatusIfChanged(ctx, r.Client, app)
}

// servingWithoutSnapshots reports an App that is serving a release from before
// snapshots and has no NEW release to roll out. It keeps projecting its mutable
// Secrets: flipping it now would change its pod template and roll it with no
// deploy, and doing that to every such App at once is the fleet-wide roll the
// milestone ruled out (decision 2). Its next release snapshots and flips as part
// of a rollout that happens anyway. A cancel over its served release has nothing
// to restore to either, so it stays put too.
func servingWithoutSnapshots(app *appv1alpha1.App) bool {
	if app.Status.ConfigSnapshotGeneration != 0 || !releaseHasServed(app) {
		return false
	}
	return app.Status.ReleaseGeneration <= successfulReleaseGeneration(app) || canceledOverServed(app)
}

// copyConfigSecret writes app's snapshot of source at gen from source's current
// contents.
func (r *AppReconciler) copyConfigSecret(ctx context.Context, app *appv1alpha1.App, source string, gen int64) error {
	name := snapshotName(app, source, gen)
	existing := &corev1.Secret{}
	err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, existing)
	if err == nil {
		// Copy-once: the snapshot already holds what this release ran with.
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	src := &corev1.Secret{}
	if err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: source}, src); err != nil {
		if apierrors.IsNotFound(err) {
			// The source is gone (an unlinked or deleted group). The release runs
			// without it; wedging the deploy would be worse than a missing copy,
			// and the projection marks it optional exactly as before.
			return nil
		}
		return err
	}

	snapshot := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.uncachedSecretClient(), snapshot, func() error {
		if snapshot.Labels == nil {
			snapshot.Labels = map[string]string{}
		}
		snapshot.Labels[snapshotOfLabel] = source
		snapshot.Labels[snapshotGenerationLabel] = strconv.FormatInt(gen, 10)
		snapshot.Type = src.Type
		snapshot.Data = make(map[string][]byte, len(src.Data))
		for k, v := range src.Data {
			snapshot.Data[k] = append([]byte(nil), v...)
		}
		// Owned by the App so deleting the service reclaims every snapshot, and so
		// a snapshot can never outlive the thing it describes.
		return controllerutil.SetControllerReference(app, snapshot, r.Scheme)
	}); err != nil {
		return err
	}
	return nil
}

// gcReleaseConfigSnapshots deletes snapshots older than the retained window.
//
// The window is counted in GENERATIONS BEHIND the current release, not by sorting
// what exists: a fleet that has been running since before snapshots shipped has
// sparse generations, and "keep the newest 20 objects" would retain a snapshot
// from an arbitrarily old release while dropping a recent one. It never deletes
// the current generation's snapshots, so the serving template's references always
// resolve.
func (r *AppReconciler) gcReleaseConfigSnapshots(ctx context.Context, app *appv1alpha1.App) error {
	gen := app.Status.ReleaseGeneration
	if gen <= 0 {
		return nil
	}
	cutoff := gen - releaseSnapshotRetention
	if cutoff <= 0 {
		return nil
	}
	var owned corev1.SecretList
	if err := r.uncachedSecretClient().List(ctx, &owned, client.InNamespace(app.Namespace),
		client.HasLabels{snapshotGenerationLabel}); err != nil {
		return err
	}
	for i := range owned.Items {
		s := &owned.Items[i]
		at, err := strconv.ParseInt(s.Labels[snapshotGenerationLabel], 10, 64)
		if err != nil || at > cutoff {
			continue
		}
		if err := r.releaseSnapshot(ctx, app, s); err != nil {
			return err
		}
	}
	return nil
}

// releaseSnapshot gives up app's hold on an expired snapshot. An unscoped group
// copy can be co-owned (adoptUnscopedSnapshots): a co-owner only drops its
// reference, and the controller deletes it once nobody else still holds it.
// Retention by generation number stays sound for co-owned copies — they collided
// precisely because both services were at that same generation.
func (r *AppReconciler) releaseSnapshot(ctx context.Context, app *appv1alpha1.App, s *corev1.Secret) error {
	controlled := metav1.IsControlledBy(s, app)
	refs := slices.DeleteFunc(slices.Clone(s.OwnerReferences), func(o metav1.OwnerReference) bool { return o.UID == app.UID })
	switch {
	case len(refs) == len(s.OwnerReferences):
		return nil // not ours
	case controlled && len(refs) == 0:
		if err := r.uncachedSecretClient().Delete(ctx, s); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	case controlled:
		return nil // still read by a co-owner; it drops its reference when it expires
	}
	s.OwnerReferences = refs
	if err := r.uncachedSecretClient().Update(ctx, s); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// canceledOverServed reports whether this pass is settling a cancel over a release
// that served — the one case where the pod template must be the SERVED release's,
// not a projection of the current spec. It is the predicate
// prepareAppReleaseDecision uses to take the canceled branch, plus releaseHasServed.
func canceledOverServed(app *appv1alpha1.App) bool {
	gen, ok := canceledReleaseGeneration(app)
	return ok && gen == requestedReleaseGeneration(app) && releaseHasServed(app)
}

// recordReleasePodTemplate stores the pod template just applied for the current
// release generation, overwriting any earlier record for that generation so it is
// always the LAST template that generation ran with.
//
// Why the whole template and not only the Secrets (w1/m152 t002): a config_change
// alters more than configuration Secrets. The save bumps spec.restartedAt, which
// lands on the template as the app.bex.co/restarted-at annotation, and the same
// deploy can carry a start command, health-check path or plan. Re-projecting the
// current spec on cancel therefore changed the template — and rolled a pod — even
// once the Secrets were reverted. Restoring the recorded template verbatim is the
// only way to guarantee "the last successful deploy remains live" with no rollout.
//
// A Secret rather than a ConfigMap because spec.env literals are rendered inline in
// the template.
func (r *AppReconciler) recordReleasePodTemplate(ctx context.Context, app *appv1alpha1.App, tmpl corev1.PodTemplateSpec) error {
	gen := app.Status.ReleaseGeneration
	if gen <= 0 {
		return nil
	}
	runtime, err := r.selectedRuntimeApp(ctx, app)
	if err != nil {
		return err
	}
	spec := releaseRecordSpec(runtime)
	spec.SavedReplicas = new(app.Spec.Replicas)
	return r.writeRuntimeConfigRecord(ctx, app, gen, &runtimeConfigRecord{spec: spec, template: tmpl})
}

// servedPodTemplateForCancel returns the recorded pod template of the last served
// release when this pass is settling a cancel over it, and nil otherwise — including
// when that release predates this change or GC reclaimed its record, in which case
// the caller re-projects as before: the honest fallback, never a guess.
func (r *AppReconciler) servedPodTemplateForCancel(ctx context.Context, app *appv1alpha1.App) (*corev1.PodTemplateSpec, error) {
	if !canceledOverServed(app) {
		return nil, nil
	}
	served := successfulReleaseGeneration(app)
	if served <= 0 {
		return nil, nil
	}
	rec := &corev1.Secret{}
	err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.ReleaseRecordName(app.Name, served)}, rec)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tmpl corev1.PodTemplateSpec
	if err := json.Unmarshal(rec.Data[appv1alpha1.ReleaseRecordPodTemplateKey], &tmpl); err != nil {
		return nil, fmt.Errorf("decode recorded pod template for generation %d: %w", served, err)
	}
	return &tmpl, nil
}

// recordServingTemplate records what this release generation now runs, for a later
// cancel to restore. Never while settling a cancel: that pass is the served
// release's, and writing a re-projection over its record would lose the template it
// actually ran. A failure is logged rather than failing the release — without a
// record a later cancel falls back to re-projecting, which is the pre-m152
// behaviour.
func (r *AppReconciler) recordServingTemplate(ctx context.Context, app *appv1alpha1.App, tmpl corev1.PodTemplateSpec) {
	if canceledOverServed(app) {
		return
	}
	if err := r.recordReleasePodTemplate(ctx, app, tmpl); err != nil {
		logf.FromContext(ctx).Error(err, "recording release pod template", "app", app.Name)
	}
}

const (
	cancelTemplateRestore   = "restore"
	cancelTemplateReproject = "reproject"
)

// cancelTemplateChangesTotal counts pod-template changes made while settling a
// cancel over a served release (w1/m152 t003 step 1). Every such change is a
// rollout the canceled deploy's own row accounts for, so it is not silent — but the
// two kinds mean different things:
//
//   - restore: the served release's recorded template was applied back over a
//     canceled release that had already rolled (the health-gated case). Expected.
//   - reproject: there was no record to restore, so the current spec was
//     re-projected. That is the pre-m152 path, and it can ship saved-but-canceled
//     changes; it should trend to zero as pre-snapshot releases age out. Anything
//     else is a regression.
var cancelTemplateChangesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "bex_cancel_template_changes_total",
	Help: "Pod-template changes made while settling a cancel over a served release, by kind (restore = the served release's recorded template was re-applied; reproject = no record existed and the current spec was re-projected, the pre-snapshot fallback that can ship canceled changes).",
}, []string{"kind"})

func init() {
	ctrlmetrics.Registry.MustRegister(cancelTemplateChangesTotal)
}

// afterServingDeployment is everything that follows applying the Deployment:
// whether the service is running behind its saved spec, the rollout invariant,
// and recording the template for a later cancel. One call from reconcileKubernetes
// keeps that function under the cyclomatic ceiling.
func (r *AppReconciler) afterServingDeployment(ctx context.Context, app *appv1alpha1.App, tmpl corev1.PodTemplateSpec, templateChanged, restored bool) {
	settling := canceledOverServed(app)
	ref := app.ActiveReleaseConfig()
	app.Status.UndeployedChanges = settling || (ref != nil && (ref.SourceGeneration == 0 || app.Status.UndeployedChanges))
	if settling && templateChanged {
		kind := cancelTemplateRestore
		if !restored {
			kind = cancelTemplateReproject
			logf.FromContext(ctx).Info("canceled release re-projected from the saved spec: no recorded template to restore",
				"app", app.Name, "servedGeneration", successfulReleaseGeneration(app))
		}
		cancelTemplateChangesTotal.WithLabelValues(kind).Inc()
	}
	r.recordServingTemplate(ctx, app, tmpl)
}

// applyServingDeployment creates or updates the Deployment from params, applying a
// served release's recorded template verbatim when restore is non-nil, then runs
// afterServingDeployment with whether the pod template actually changed — which is
// the difference between a rollout and a no-op.
func (r *AppReconciler) applyServingDeployment(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, params deploymentParams, restore *corev1.PodTemplateSpec) error {
	runtime, err := r.selectedRuntimeApp(ctx, app)
	if err != nil {
		return err
	}
	var prior *corev1.PodTemplateSpec
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		if !dep.CreationTimestamp.IsZero() {
			prior = dep.Spec.Template.DeepCopy()
		}
		applyDeploymentSpec(dep, runtime, params)
		if restore != nil {
			dep.Spec.Template = *restore.DeepCopy()
		}
		return controllerutil.SetControllerReference(app, dep, r.Scheme)
	}); err != nil {
		return err
	}
	changed := prior != nil && !equality.Semantic.DeepEqual(*prior, dep.Spec.Template)
	r.afterServingDeployment(ctx, app, dep.Spec.Template, changed, restore != nil)
	return nil
}

// applyServingCronJob is applyServingDeployment for a cron job: it converges the
// CronJob onto tmpl and hands afterServingDeployment whether the job template
// actually changed, so the cancel meter and the release record work the same way
// for both runtimes.
func (r *AppReconciler) applyServingCronJob(ctx context.Context, app *appv1alpha1.App, tmpl corev1.PodTemplateSpec, restored bool) (ctrl.Result, error) {
	before, err := r.cronJobTemplate(ctx, app)
	if err != nil {
		return ctrl.Result{}, &stepFailure{reason: "CronJobFailed", err: err}
	}
	res, err := r.convergeCronRuntime(ctx, app, tmpl)
	if err != nil {
		return res, err
	}
	after, err := r.cronJobTemplate(ctx, app)
	if err != nil {
		return ctrl.Result{}, &stepFailure{reason: "CronJobFailed", err: err}
	}
	r.afterServingDeployment(ctx, app, after, !equality.Semantic.DeepEqual(before, after), restored)
	return res, nil
}

// cronJobTemplate is the CronJob's current job pod template, or the zero value
// before the first converge.
func (r *AppReconciler) cronJobTemplate(ctx context.Context, app *appv1alpha1.App) (corev1.PodTemplateSpec, error) {
	cj := &batchv1.CronJob{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.CronJobName(app.Name)}, cj)
	if apierrors.IsNotFound(err) {
		return corev1.PodTemplateSpec{}, nil
	}
	return cj.Spec.JobTemplate.Spec.Template, err
}
