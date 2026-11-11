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
	"cmp"
	"context"
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Exit statuses a container gets when it is stopped from outside: SIGKILL and
// SIGTERM. The kubelet ends a container that way for a failed liveness check,
// a rollout or a drain, none of which is the application exiting, so they are
// not recorded as its exits. An out-of-memory kill also reports 137 and is
// told apart by its reason.
const (
	exitStatusSIGKILL = 137
	exitStatusSIGTERM = 143
)

// instanceExitMaxAge is how old an exit may be when it is first seen. A pod
// keeps its last termination for as long as it lives, so without the bound
// the first pass of an operator that learns to read it would record crashes
// from days ago as if they had just happened. A pod's status change wakes
// the reconcile within seconds, so a fresh exit is never this old.
const instanceExitMaxAge = 10 * time.Minute

// recordInstanceExits notes how the serving release's instances last exited
// (status.instanceExits, w8/m158). Render: "An Instance Failed event occurs
// when your service's application exits unexpectedly… the event might include
// an exit code." A crashed or memory-killed process restarts in place within
// seconds, so bex-api's sampling of the Ready condition usually never saw it,
// and when it did it could only say a health check failed. The pod carries the
// cause: the app container's last termination.
//
// Only the release that is serving is read, and only what happened after its
// rollout settled. A release whose rollout is failing reports its exit through
// status.rolloutFailure (recordRolloutExit) and its deploy, and one that
// crashed once while starting and then came up never took the service down:
// the previous release served until it was Ready.
func (r *AppReconciler) recordInstanceExits(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment) {
	if dep == nil || dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 || app.Status.ActiveRevision == "" {
		return
	}
	settled, ok := rolloutSettledAt(dep)
	if !ok {
		return
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return
	}
	app.Status.InstanceExits = withInstanceExits(app.Status.InstanceExits, pods.Items, app.Status.ActiveRevision, settled, time.Now())
}

// rolloutSettledAt is when dep's current rollout completed: the time
// Kubernetes marked its new ReplicaSet available. ok is false while a rollout
// is still in progress, when no exit is the served release's to report.
func rolloutSettledAt(dep *appsv1.Deployment) (time.Time, bool) {
	c := deploymentCondition(dep, appsv1.DeploymentProgressing)
	if c == nil || c.Reason != reasonNewReplicaSetAvailable {
		return time.Time{}, false
	}
	if !c.LastUpdateTime.IsZero() {
		return c.LastUpdateTime.Time, true
	}
	return c.LastTransitionTime.Time, true
}

// instanceExitUnreadySlack is how much earlier than its process's exit an
// instance must have stopped being ready for the exit to count as following
// the outage rather than causing it. A process that dies takes readiness with
// it in the same instant, and the two times are recorded to the second.
const instanceExitUnreadySlack = 3 * time.Second

// unreadyBefore reports whether pod had already stopped being ready before
// its process exited at finished (InstanceExit.AfterUnready). A pod that is
// ready again no longer says when it was not, so it answers false: the exit
// is then taken as the cause. The answer is recorded with the exit the first
// time it is seen, which is normally while the instance is still down.
func unreadyBefore(pod *corev1.Pod, finished time.Time) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status != corev1.ConditionTrue && c.LastTransitionTime.Time.Before(finished.Add(-instanceExitUnreadySlack))
		}
	}
	return false
}

// withInstanceExits adds the exits the pods of revision show to recorded and
// returns the newest appv1alpha1.MaxInstanceExits, newest first. An exit from
// before since, the time the release's rollout settled, is a start-up crash
// of the rollout and is left out. It returns recorded itself when the pods
// show nothing new, so an unchanged status is not rewritten.
func withInstanceExits(recorded []appv1alpha1.InstanceExit, pods []corev1.Pod, revision string, since, now time.Time) []appv1alpha1.InstanceExit {
	var found []appv1alpha1.InstanceExit
	for i := range pods {
		pod := &pods[i]
		if pod.Labels[labelRevision] != revision {
			continue
		}
		for _, cs := range pod.Status.ContainerStatuses {
			t := cs.LastTerminationState.Terminated
			if cs.Name != appContainerName || t == nil || cs.RestartCount <= 0 ||
				t.FinishedAt.Time.Before(since) || now.Sub(t.FinishedAt.Time) > instanceExitMaxAge {
				continue
			}
			oom := t.Reason == "OOMKilled"
			if !oom && (t.ExitCode == exitStatusSIGKILL || t.ExitCode == exitStatusSIGTERM) {
				continue
			}
			exit := appv1alpha1.InstanceExit{
				Instance: pod.Name, RestartCount: cs.RestartCount, ExitCode: t.ExitCode, OOMKilled: oom, FinishedAt: t.FinishedAt,
				AfterUnready: unreadyBefore(pod, t.FinishedAt.Time),
			}
			known := slices.ContainsFunc(recorded, func(e appv1alpha1.InstanceExit) bool {
				return e.Instance == exit.Instance && e.RestartCount == exit.RestartCount
			})
			if !known {
				found = append(found, exit)
			}
		}
	}
	if len(found) == 0 {
		return recorded
	}
	all := append(found, recorded...)
	// A total order, so exits that finished in the same second keep the same
	// places from pass to pass and the kept set does not flip at the bound.
	slices.SortFunc(all, func(a, b appv1alpha1.InstanceExit) int {
		return cmp.Or(
			b.FinishedAt.Compare(a.FinishedAt.Time),
			cmp.Compare(a.Instance, b.Instance),
			cmp.Compare(b.RestartCount, a.RestartCount),
		)
	})
	if len(all) > appv1alpha1.MaxInstanceExits {
		all = all[:appv1alpha1.MaxInstanceExits]
	}
	return all
}
