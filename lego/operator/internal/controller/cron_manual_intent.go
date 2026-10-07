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

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Binds manual Jobs to their exact trigger even before App status observes them.
const annotationManualRunAt = "app.bex.co/manual-run-at"

func manualRunHandled(app *appv1alpha1.App) bool {
	return app.Spec.RunAt != "" && app.Status.ManualRunHandledAt == app.Spec.RunAt
}

// acknowledgeManualRun retains one token independently of bounded run history.
// Persist only after materialization or accepted cancellation, before discarding
// their evidence. Optimistic locking prevents an old reconcile from replacing a
// newer intent's acknowledgement; its caller must retry conflicts from scratch.
func (r *AppReconciler) acknowledgeManualRun(ctx context.Context, app *appv1alpha1.App) error {
	if app.Spec.RunAt == "" || manualRunHandled(app) {
		return nil
	}
	return r.patchAppStatus(ctx, app, func(status *appv1alpha1.AppStatus) {
		status.ManualRunHandledAt = app.Spec.RunAt
	})
}

// adoptManualRun upgrades existing intent evidence before history eviction or
// Job deletion. A running history entry also proves materialization. A legacy
// App with no matching Job, history, or cancellation is indistinguishable from
// a fresh pending trigger and stays eligible to run; age is not evidence.
func (r *AppReconciler) adoptManualRun(ctx context.Context, app *appv1alpha1.App) error {
	if app.Spec.RunAt == "" || manualRunHandled(app) {
		return nil
	}
	name := manualRunJobName(app.Name, app.Spec.RunAt)
	if app.Spec.CancelRun != nil && app.Spec.CancelRun.Name == name {
		return r.acknowledgeManualRun(ctx, app)
	}
	for _, run := range app.Status.Runs {
		if run.Name == name {
			return r.acknowledgeManualRun(ctx, app)
		}
	}
	// Read the current manual Job directly: a terminal Job may be older than all
	// ten entries cronRuns retains and never appear in the resulting history.
	job := &batchv1.Job{}
	if err := r.buildPlaneClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, job); err != nil {
		return client.IgnoreNotFound(err)
	}
	// A legacy current Job can be identified exactly from RunAt. Bind it before
	// acknowledgement, so a newer trigger arriving before history persistence
	// can still preempt it without guessing the identity of unrelated Jobs.
	if job.Annotations[annotationManualRunAt] != app.Spec.RunAt && metav1.IsControlledBy(job, app) {
		before := job.DeepCopy()
		metav1.SetMetaDataAnnotation(&job.ObjectMeta, annotationManualRunAt, app.Spec.RunAt)
		if err := r.buildPlaneClient().Patch(ctx, job, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
			return err
		}
	}
	return r.acknowledgeManualRun(ctx, app)
}
