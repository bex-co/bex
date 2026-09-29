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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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

	// releaseSnapshotSuffix marks a snapshot and carries its generation:
	// `<source>-r<gen>`. Deriving the name from the generation is what lets the
	// pod-template projection stay a pure function of the App.
	releaseSnapshotSuffix = "-r"

	// snapshotOfLabel records which source a snapshot copies, and
	// snapshotGenerationLabel its release generation, so garbage collection can
	// select an App's snapshots without parsing names.
	snapshotOfLabel         = "app.bex.co/config-snapshot-of"
	snapshotGenerationLabel = "app.bex.co/config-snapshot-generation"
)

// releaseSnapshotName is the immutable copy of source for generation gen.
func releaseSnapshotName(source string, gen int64) string {
	return source + releaseSnapshotSuffix + strconv.FormatInt(gen, 10)
}

// configSnapshotActive reports whether the projection should reference snapshots
// rather than the mutable sources. It keys on ConfigSnapshotGeneration ALONE, not
// on that field matching ReleaseGeneration, which is what lets a cancel settle
// back onto an EARLIER release's configuration (w1/m152 t002): the canceled branch
// points this at the last served generation, and the template reverts image and
// config together.
//
// Zero keeps the migration lazy — an App that has not dispatched a release since
// this shipped references its mutable Secrets and keeps its current pod template,
// so nothing rolls on operator upgrade.
func configSnapshotActive(app *appv1alpha1.App) bool {
	return app.Status.ConfigSnapshotGeneration > 0
}

// snapshotOrSource maps a mutable source name to the snapshot of whichever
// generation the App is currently projecting, and returns it unchanged otherwise.
// Every projection site goes through here so a single App can never mix snapshot
// and mutable references — which would make precedence depend on which source
// happened to be copied.
func snapshotOrSource(app *appv1alpha1.App, source string) string {
	if source == "" || !configSnapshotActive(app) {
		return source
	}
	return releaseSnapshotName(source, app.Status.ConfigSnapshotGeneration)
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
		name := releaseSnapshotName(source, gen)
		err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, &corev1.Secret{})
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
		if name == "" || isReleaseSnapshotName(name) {
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

// isReleaseSnapshotName reports whether name is itself a snapshot, by requiring
// the suffix to be followed by digits only. A tenant-named Secret ending in
// "-r12" is therefore not mistaken for one.
func isReleaseSnapshotName(name string) bool {
	idx := strings.LastIndex(name, releaseSnapshotSuffix)
	if idx <= 0 {
		return false
	}
	digits := name[idx+len(releaseSnapshotSuffix):]
	if digits == "" {
		return false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
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
	gen := app.Status.ReleaseGeneration
	if gen <= 0 || app.Status.ConfigSnapshotGeneration == gen {
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

// copyConfigSecret writes `<source>-r<gen>` from source's current contents.
func (r *AppReconciler) copyConfigSecret(ctx context.Context, app *appv1alpha1.App, source string, gen int64) error {
	name := releaseSnapshotName(source, gen)
	existing := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, existing)
	if err == nil {
		// Copy-once: the snapshot already holds what this release ran with.
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	from := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: source}, from); err != nil {
		if apierrors.IsNotFound(err) {
			// The source is gone (an unlinked or deleted group). The release runs
			// without it; wedging the deploy would be worse than a missing copy,
			// and the projection marks it optional exactly as before.
			return nil
		}
		return err
	}

	snapshot := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, snapshot, func() error {
		if snapshot.Labels == nil {
			snapshot.Labels = map[string]string{}
		}
		snapshot.Labels[snapshotOfLabel] = source
		snapshot.Labels[snapshotGenerationLabel] = strconv.FormatInt(gen, 10)
		snapshot.Type = from.Type
		snapshot.Data = make(map[string][]byte, len(from.Data))
		for k, v := range from.Data {
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
	if err := r.List(ctx, &owned, client.InNamespace(app.Namespace),
		client.HasLabels{snapshotGenerationLabel}); err != nil {
		return err
	}
	for i := range owned.Items {
		s := &owned.Items[i]
		if !metav1.IsControlledBy(s, app) {
			continue
		}
		at, err := strconv.ParseInt(s.Labels[snapshotGenerationLabel], 10, 64)
		if err != nil || at > cutoff {
			continue
		}
		if err := r.Delete(ctx, s); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// podTemplateSnapshotKey holds the JSON-encoded PodTemplateSpec in a recorded
// release template.
const podTemplateSnapshotKey = "podTemplate"

// podTemplateSnapshotName is the recorded pod template of release generation gen.
// It shares releaseSnapshotName's suffix and the snapshot labels, so it rides the
// same 20-generation GC and the same App ownership as the configuration copies.
func podTemplateSnapshotName(app *appv1alpha1.App, gen int64) string {
	return releaseSnapshotName(app.Name+"-podtemplate", gen)
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
	raw, err := json.Marshal(tmpl)
	if err != nil {
		return err
	}
	rec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: podTemplateSnapshotName(app, gen), Namespace: app.Namespace}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, rec, func() error {
		if rec.Labels == nil {
			rec.Labels = map[string]string{}
		}
		rec.Labels[snapshotOfLabel] = app.Name + "-podtemplate"
		rec.Labels[snapshotGenerationLabel] = strconv.FormatInt(gen, 10)
		rec.Data = map[string][]byte{podTemplateSnapshotKey: raw}
		return controllerutil.SetControllerReference(app, rec, r.Scheme)
	})
	return err
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
	err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: podTemplateSnapshotName(app, served)}, rec)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tmpl corev1.PodTemplateSpec
	if err := json.Unmarshal(rec.Data[podTemplateSnapshotKey], &tmpl); err != nil {
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
