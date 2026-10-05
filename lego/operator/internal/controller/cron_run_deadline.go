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
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Kubernetes counts appv1alpha1.CronRunActiveDeadlineSeconds from Job startTime,
// including pending scheduling and image pulls, and honors Pod termination grace.
const cronRunActiveDeadlineSeconds = appv1alpha1.CronRunActiveDeadlineSeconds

// CronJob template updates apply only to future Jobs. Adopt active owned runs
// while observing history so an upgrade also bounds their existing lifetime.
// Preserve shorter deadlines and startTime; never restart a run's clock.
func (r *AppReconciler) enforceCronRunDeadline(ctx context.Context, app *appv1alpha1.App, cron *batchv1.CronJob, job *batchv1.Job) error {
	if jobSettled(job) || !job.DeletionTimestamp.IsZero() || !ownedCronRun(app, cron, job) {
		return nil
	}
	if deadline := job.Spec.ActiveDeadlineSeconds; deadline != nil && *deadline <= cronRunActiveDeadlineSeconds {
		return nil
	}
	before := job.DeepCopy()
	job.Spec.ActiveDeadlineSeconds = new(cronRunActiveDeadlineSeconds)
	return client.IgnoreNotFound(r.buildPlaneClient().Patch(ctx, job, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})))
}
