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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// observeCronRunFailure records why a Failed run failed (w4/m114). cronRuns
// calls it once, on the pass that first sees the run Failed, while the Job's
// Pod still exists, and carries the result forward from status: Kubernetes
// garbage-collects terminal Jobs and their Pods, so a later probe can only miss.
func (r *AppReconciler) observeCronRunFailure(ctx context.Context, job *batchv1.Job, run *appv1alpha1.CronRun) {
	if run.Status != appv1alpha1.CronRunFailed || run.FailureReason != "" {
		return
	}
	if execution.JobFailedReason(job) == batchv1.JobReasonDeadlineExceeded {
		run.FailureReason = appv1alpha1.CronRunReasonDeadlineExceeded
		return
	}
	var pods corev1.PodList
	// The controller-uid label, not the Job name: a Job recreated under the
	// same name must never borrow an earlier run's Pod.
	if err := r.buildPlaneClient().List(ctx, &pods, client.InNamespace(job.Namespace),
		client.MatchingLabels{batchv1.ControllerUidLabel: string(job.UID)}); err != nil {
		return
	}
	for i := range pods.Items {
		if reason, exit := cronPodFailure(&pods.Items[i]); reason != "" {
			run.FailureReason, run.ExitCode = reason, exit
			return
		}
	}
}

// cronPodFailure maps one run Pod onto the failure vocabulary, or "" when the
// Pod says nothing about a failure (still running, or exited 0).
func cronPodFailure(pod *corev1.Pod) (string, *int32) {
	if pod.Status.Reason == "Evicted" {
		return appv1alpha1.CronRunReasonEvicted, nil
	}
	for _, cs := range pod.Status.ContainerStatuses {
		t := cs.State.Terminated
		if cs.Name != appContainerName || t == nil {
			continue
		}
		exit := t.ExitCode
		if t.Reason == "OOMKilled" {
			return appv1alpha1.CronRunReasonOOMKilled, &exit
		}
		if exit != 0 {
			return appv1alpha1.CronRunReasonNonZeroExit, &exit
		}
	}
	return "", nil
}
