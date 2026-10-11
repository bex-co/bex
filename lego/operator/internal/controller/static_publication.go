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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/identity"
	"github.com/bex-co/bex/lego/types/k8sname"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Retained static publications (w4/m197; docs/ADR029-static-sites.md
// §Rollback). Every revision a static_site publishes stays in the object store
// under its own immutable prefix until the App is deleted; its record follows
// the release-record retention window (gcReleaseConfigSnapshots). A rollback restores
// the target's prefix by copying it into the new revision, so it neither
// re-extracts the target's image through today's saved publish directory nor
// rebuilds from today's build command. The per-generation record below is what
// lets bex-api name that prefix — and refuse, before dispatch, a target whose
// publication was never recorded.

// staticRestoreSelection is the rollback's retained-publication selection for
// the release being reconciled, or nil when this release publishes normally.
func staticRestoreSelection(app *appv1alpha1.App) *appv1alpha1.ReleaseConfigReference {
	if ref := app.ActiveReleaseConfig(); ref != nil && ref.StaticPrefix != "" {
		return ref
	}
	return nil
}

// recordStaticPublications records where the just-published revision lives,
// and backfills the outgoing served revision when it predates these records:
// its prefix is exactly what the static-server serves, so it is known rather
// than guessed. Records are written before status adopts the new revision; a
// failed write is retried by the next pass, which re-observes the completed
// publish Job without re-uploading.
func (r *AppReconciler) recordStaticPublications(ctx context.Context, app *appv1alpha1.App, rev string) error {
	if served, ok := appv1alpha1.RevisionGeneration(app.Status.ActiveRevision); ok && app.Status.ActiveRevision != rev {
		prefix := identity.ServedStaticPrefix(app.Status.StaticPrefix, app.Name, app.Status.ActiveRevision)
		if err := r.writeStaticPublicationRecord(ctx, app, served, prefix, false); err != nil {
			return err
		}
	}
	if err := r.writeStaticPublicationRecord(ctx, app, releaseGeneration(app), appIdentity(app).StaticPrefix(rev), true); err != nil {
		return err
	}
	// Like the runtime path, an expired-record sweep failure never fails a
	// publish that already succeeded; the next publish sweeps again.
	if err := r.gcReleaseConfigSnapshots(ctx, app); err != nil {
		logf.FromContext(ctx).Error(err, "gc static publication records", "app", app.Name)
	}
	return nil
}

// writeStaticPublicationRecord stores prefix as generation's publication. A
// backfill never replaces an existing record; the release that just published
// always states its own prefix.
func (r *AppReconciler) writeStaticPublicationRecord(ctx context.Context, app *appv1alpha1.App, generation int64, prefix string, overwrite bool) error {
	if !appv1alpha1.ValidStaticPublicationPrefix(prefix) {
		return nil // e.g. a pre-revision "latest" publish: nothing restorable to name
	}
	rec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.StaticPublicationRecordName(app.Name, generation), Namespace: app.Namespace}}
	mutate := func() error {
		if rec.Labels == nil {
			rec.Labels = map[string]string{}
		}
		rec.Labels[snapshotOfLabel] = k8sname.Fit(app.Name + "-static")
		rec.Labels[snapshotGenerationLabel] = strconv.FormatInt(generation, 10)
		rec.Data = map[string][]byte{appv1alpha1.StaticPublicationPrefixKey: []byte(prefix)}
		return controllerutil.SetControllerReference(app, rec, r.Scheme)
	}
	if overwrite {
		_, err := controllerutil.CreateOrUpdate(ctx, r.uncachedSecretClient(), rec, mutate)
		return err
	}
	if err := mutate(); err != nil {
		return err
	}
	if err := r.uncachedSecretClient().Create(ctx, rec); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
