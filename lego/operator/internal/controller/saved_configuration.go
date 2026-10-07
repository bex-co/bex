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
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Saved configuration has its own App-only queue. A Save-only notification must
// never dispatch runtime work, and Secret watches would require a new informer
// and broader access. Startup replay and serving-revision changes also compare
// the last served record; a candidate's snapshots are not proof it is live.
func (r *AppReconciler) setupSavedConfigurationStatus(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("app-config-status").
		For(&appv1alpha1.App{}, builder.WithPredicates(savedConfigurationPredicate{})).
		Complete(reconcile.Func(r.reconcileSavedConfigurationStatus))
}

type savedConfigurationPredicate struct{ predicate.Funcs }

func (savedConfigurationPredicate) Update(e event.UpdateEvent) bool {
	old, oldOK := e.ObjectOld.(*appv1alpha1.App)
	next, nextOK := e.ObjectNew.(*appv1alpha1.App)
	if !oldOK || !nextOK {
		return false
	}
	if old.Generation != next.Generation || old.Status.ActiveRevision != next.Status.ActiveRevision ||
		old.Status.ConfigSnapshotGeneration != next.Status.ConfigSnapshotGeneration ||
		old.Status.UnscopedSnapshotGeneration != next.Status.UnscopedSnapshotGeneration {
		return true
	}
	for _, key := range []string{appv1alpha1.AnnotationSavedConfigRevision,
		appv1alpha1.PendingEnvSecretAnnotation, appv1alpha1.PendingFilesSecretAnnotation,
		appv1alpha1.AnnotationCanceledReleaseGeneration, appv1alpha1.AnnotationReleaseConfigScaled} {
		if old.Annotations[key] != next.Annotations[key] {
			return true
		}
	}
	return false
}

func (r *AppReconciler) reconcileSavedConfigurationStatus(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	app := &appv1alpha1.App{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Static publications do not retain build-input configuration records. They
	// cannot use pod-release comparison to claim their artifact's inputs match.
	if !app.DeletionTimestamp.IsZero() || !r.canonicalNamespace(app) ||
		r.Mode != ModeKubernetes || app.Spec.Type == appv1alpha1.TypeStaticSite {
		return ctrl.Result{}, nil
	}
	different, compareErr := r.servingConfigurationDiffers(ctx, app)
	if app.Status.UndeployedChanges != different {
		// A concurrent save or serving-revision change invalidates the comparison.
		// Patch only this field and retry the complete read on a conflict.
		if err := r.patchAppStatus(ctx, app, func(status *appv1alpha1.AppStatus) {
			status.UndeployedChanges = different
		}); err != nil {
			return ctrl.Result{}, err
		}
	}
	// A failed save can restore projected Secrets without ever changing the App
	// resourceVersion. Repair a comparison that raced that compensation even
	// when no metadata notification was committed. No workload work is queued.
	result := ctrl.Result{}
	if len(releaseConfigSources(app)) > 0 || different || app.Annotations[appv1alpha1.AnnotationSavedConfigRevision] != "" {
		result.RequeueAfter = time.Minute
	}
	return result, compareErr
}

func (r *AppReconciler) servingConfigurationDiffers(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	gen := successfulReleaseGeneration(app)
	if gen <= 0 || !releaseHasServed(app) {
		return app.Status.UndeployedChanges || app.Annotations[appv1alpha1.AnnotationSavedConfigRevision] != "", nil
	}
	rec, err := r.readRuntimeConfigRecord(ctx, app, gen)
	if apierrors.IsNotFound(err) {
		// A legacy/missing record cannot prove equality. Do not snapshot today's
		// mutable values and mislabel them as the values the old release consumed.
		return app.Status.UndeployedChanges || canceledOverServed(app) || app.ActiveReleaseConfig() != nil ||
			app.Annotations[appv1alpha1.AnnotationSavedConfigRevision] != "", nil
	}
	if err != nil {
		return true, err
	}
	if app.Status.ConfigSnapshotGeneration == 0 && len(releaseConfigSources(app)) > 0 {
		return app.Status.UndeployedChanges || app.Annotations[appv1alpha1.AnnotationSavedConfigRevision] != "", nil
	}
	runtime := rec.projectRuntimeApp(app, &appv1alpha1.ReleaseConfigReference{Generation: gen})
	runtime.Status.ConfigSnapshotGeneration = gen
	different, err := r.savedConfigurationDiffers(ctx, app, runtime)
	if err != nil {
		return true, err // unavailable evidence is never evidence of equality
	}
	if app.Spec.Repo == "" {
		for _, container := range rec.template.Spec.Containers {
			if container.Name == appContainerName && container.Image != app.Spec.Image {
				different = true
			}
		}
	}
	// Cancellation also retains plan/pre-deploy settings outside the rollback
	// record's selected fields. The served record's settings fingerprint says
	// whether any of them differ; a cancel of an identical redeploy leaves
	// nothing pending (w4/m165). A record without one cannot prove equality.
	if !different && canceledOverServed(app) {
		fp := rec.spec.SettingsFingerprint
		different = fp == "" || fp != releaseSettingsFingerprint(app.Spec)
	}
	return different, nil
}

// retainReleaseSources prevents a newly staged first source from entering the
// current pod template during an unrelated health/scale/restart reconciliation.
// Only membership is frozen: operational settings still converge normally.
func (r *AppReconciler) retainReleaseSources(ctx context.Context, app *appv1alpha1.App) (*appv1alpha1.App, error) {
	if app.Annotations[appv1alpha1.PendingEnvSecretAnnotation] == "" && app.Annotations[appv1alpha1.PendingFilesSecretAnnotation] == "" {
		return app, nil
	}
	gen := app.Status.ReleaseGeneration
	rec, err := r.readRuntimeConfigRecord(ctx, app, gen)
	if apierrors.IsNotFound(err) {
		rec, err = r.liveConfigurationRecord(ctx, app, gen)
	}
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return app, nil // the new release has not projected its inputs yet
	}
	runtime := app.DeepCopy()
	rec.applySources(runtime)
	return runtime, nil
}

// Missing release records may recover source NAMES from the owned live template,
// never historical values from mutable Secrets. This keeps lazy migration from
// rolling a legacy service, including a previously consumed pending reference.
func (r *AppReconciler) liveConfigurationRecord(ctx context.Context, app *appv1alpha1.App, gen int64) (*runtimeConfigRecord, error) {
	var template corev1.PodTemplateSpec
	var workload client.Object
	if app.Spec.Type == appv1alpha1.TypeCronJob {
		// New cron releases persist their record before writing the template.
		if !releaseHasServed(app) || gen != successfulReleaseGeneration(app) {
			return nil, nil
		}
		cj := &batchv1.CronJob{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.CronJobName(app.Name)}, cj); err != nil {
			return nil, err
		}
		template, workload = cj.Spec.JobTemplate.Spec.Template, cj
	} else {
		dep := &appsv1.Deployment{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(app), dep); err != nil {
			if apierrors.IsNotFound(err) && (!releaseHasServed(app) || gen != successfulReleaseGeneration(app)) {
				return nil, nil
			}
			return nil, err
		}
		template, workload = dep.Spec.Template, dep
	}
	if !metav1.IsControlledBy(workload, app) {
		return nil, fmt.Errorf("serving configuration has no owned workload")
	}
	if app.Spec.Type != appv1alpha1.TypeCronJob && template.Labels[labelRevision] != releaseRevision(app) {
		if gen != successfulReleaseGeneration(app) {
			return nil, nil // prior template, before the new release is applied
		}
		if template.Labels[labelRevision] != "" {
			return nil, fmt.Errorf("serving configuration record is missing and workload belongs to another release")
		}
	}
	rec := &runtimeConfigRecord{template: template}
	rec.adoptLegacySpec(app, gen)
	return rec, nil
}
