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

package usage

import (
	"cmp"
	"context"
	"log"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// build_seconds counts what Render counts as pipeline minutes (w6/m307,
// docs/ADR023): the build, Dockerfile or buildpack, and the pre-deploy command,
// whether it succeeded or failed, from when its pod was placed. Two exceptions,
// both the platform's own time: a build that failed for a platform reason, and
// a platform-opened release (w1/m184). A run is metered in the hour it ended.

var kpackBuildListGVK = appv1alpha1.KpackGroupVersion.WithKind("BuildList")

// kpackSucceeded is the condition a kpack Build reports its verdict on.
const kpackSucceeded = "Succeeded"

// queryBuildSeconds sums one App's metered build and pre-deploy runs that
// ended in [window, end). Build Jobs and kpack Builds run in
// BEX_BUILD_NAMESPACE (or the App's own namespace when it is unset);
// pre-deploy Jobs always run in the App's namespace (ADR043 D8). crName is the
// App CR's name, which the operator stamps on all three; the workspace-scoped
// store name never matches. exempt names the platform-opened release
// revisions.
func (s *Service) queryBuildSeconds(ctx context.Context, crName, appNs string, window, end time.Time, exempt map[string]bool) (int64, bool) {
	if s.Client == nil {
		return 0, false
	}
	buildNs := cmp.Or(s.BuildNamespace, appNs)
	var total int64
	for _, q := range []struct {
		namespace string
		labels    client.MatchingLabels
		// buildJob: a build Job's failure reason names its fault; a
		// pre-deploy Job's does not (it has no pod failure policy, and a failed
		// migration is the tenant's).
		buildJob bool
	}{
		{buildNs, client.MatchingLabels{appv1alpha1.LabelBuild: crName}, true},
		{appNs, client.MatchingLabels{core.PodLabelPreDeploy: crName}, false},
	} {
		var jobs batchv1.JobList
		if err := s.Client.List(ctx, &jobs, client.InNamespace(q.namespace), q.labels); err != nil {
			log.Printf("usage: build_seconds list jobs for %s in %s: %v", crName, q.namespace, err)
			return 0, false
		}
		for i := range jobs.Items {
			if !exempt[jobs.Items[i].Labels[appv1alpha1.LabelBuildRevision]] {
				total += jobBilledSeconds(&jobs.Items[i], q.buildJob, window, end)
			}
		}
	}
	kpack, ok := s.kpackBuildSeconds(ctx, crName, buildNs, window, end, exempt)
	if !ok {
		return 0, false
	}
	return total + kpack, true
}

// jobBilledSeconds is what a finished Job bills in [window, end): from its
// pod's first placement (the operator's app.bex.co/build-placed-at, else the
// Job's own startTime) to its completion or failure. A build that failed for
// a platform reason bills nothing: the operator's verdict when it stamped one,
// else the Job's failure reason.
func jobBilledSeconds(job *batchv1.Job, buildJob bool, window, end time.Time) int64 {
	finished, failureReason, ok := jobEnd(job)
	if !ok {
		return 0
	}
	if buildJob && cmp.Or(job.Annotations[appv1alpha1.AnnotationBuildFault], appv1alpha1.BuildJobFault(failureReason)) == appv1alpha1.BuildFaultInfra {
		return 0
	}
	var started time.Time
	if job.Status.StartTime != nil {
		started = job.Status.StartTime.Time
	}
	if at, err := time.Parse(time.RFC3339, job.Annotations[appv1alpha1.AnnotationBuildPlacedAt]); err == nil && !at.After(finished) {
		started = at
	}
	return billedSeconds(started, finished, window, end)
}

// jobEnd is when a Job finished and, for a failed one, why. Kubernetes sets
// completionTime on success only, so a failed Job ends at its Failed condition
// (the operator's execution.JobFailedAt, which the backend cannot import).
func jobEnd(job *batchv1.Job) (time.Time, string, bool) {
	if job.Status.CompletionTime != nil {
		return job.Status.CompletionTime.Time, "", true
	}
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return c.LastTransitionTime.Time, c.Reason, true
		}
	}
	return time.Time{}, "", false
}

// kpackBuildSeconds sums an App's buildpack builds (builder: buildpack) that
// ended in [window, end). kpack runs them as Builds, not Jobs, and copies the
// operator's labels from the release's Image onto each Build and its pod. A
// cluster without kpack has none.
func (s *Service) kpackBuildSeconds(ctx context.Context, crName, buildNs string, window, end time.Time, exempt map[string]bool) (int64, bool) {
	builds := &unstructured.UnstructuredList{}
	builds.SetGroupVersionKind(kpackBuildListGVK)
	if err := s.Client.List(ctx, builds, client.InNamespace(buildNs), client.MatchingLabels{appv1alpha1.LabelBuild: crName}); err != nil {
		if meta.IsNoMatchError(err) {
			return 0, true
		}
		log.Printf("usage: build_seconds list kpack builds for %s: %v", crName, err)
		return 0, false
	}
	var podStarts map[string]time.Time
	var total int64
	for i := range builds.Items {
		b := &builds.Items[i]
		finished, ok := kpackBuildEnd(b)
		if !ok || exempt[b.GetLabels()[appv1alpha1.LabelBuildRevision]] || finished.Before(window) || !finished.Before(end) {
			continue
		}
		if podStarts == nil {
			podStarts = s.buildPodStarts(ctx, crName, buildNs)
		}
		started := b.GetCreationTimestamp().Time
		if name, _, _ := unstructured.NestedString(b.Object, "status", "podName"); !podStarts[name].IsZero() {
			started = podStarts[name]
		}
		total += billedSeconds(started, finished, window, end)
	}
	return total, true
}

// buildPodStarts maps an App's build pods to when each started. Missing pods
// (reaped, or the list failed) leave a Build to its creation time.
func (s *Service) buildPodStarts(ctx context.Context, crName, buildNs string) map[string]time.Time {
	starts := map[string]time.Time{}
	pods, err := s.BuildPods(ctx, crName, buildNs)
	if err != nil {
		log.Printf("usage: build_seconds list build pods for %s: %v", crName, err)
		return starts
	}
	for _, pod := range pods {
		if pod.Status.StartTime != nil {
			starts[pod.Name] = pod.Status.StartTime.Time
		}
	}
	return starts
}

// kpackBuildEnd is when a kpack Build reached its terminal Succeeded verdict.
func kpackBuildEnd(b *unstructured.Unstructured) (time.Time, bool) {
	conditions, _, _ := unstructured.NestedSlice(b.Object, "status", "conditions")
	for _, raw := range conditions {
		c, _ := raw.(map[string]any)
		status, _ := c["status"].(string)
		if c["type"] != kpackSucceeded || (status != string(corev1.ConditionTrue) && status != string(corev1.ConditionFalse)) {
			continue
		}
		at, _ := c["lastTransitionTime"].(string)
		if t, err := time.Parse(time.RFC3339, at); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// billedSeconds is the whole seconds from started to finished when the run
// ended in [window, end); zero otherwise, or when either time is unknown or
// they are out of order.
func billedSeconds(started, finished, window, end time.Time) int64 {
	if started.IsZero() || finished.Before(window) || !finished.Before(end) || !finished.After(started) {
		return 0
	}
	return int64(finished.Sub(started) / time.Second)
}
