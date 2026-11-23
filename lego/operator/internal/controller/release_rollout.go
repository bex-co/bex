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
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's deploy log narrates the rollout between a build's end and its
// verdict: `==> Deploying...` when the rollout starts, `==> Setting
// WEB_CONCURRENCY=N by default…` for the instance's default, and, on a native
// runtime, `==> Running '<start command>'` each time its command starts
// (w32/m560). bex-api writes those lines from what the operator saw, which it
// records in status.releaseRollout: the operator is database-free. The last
// is the instance's own line: a native image built since w32/m575 writes it
// from native-init, and the record says so, so the log narrates it only for
// an older image.

// recordReleaseRollout records the rollout of the release whose serving
// template this pass wrote to dep (status.releaseRollout). The first pass that
// writes the release's own template stamps its start, the command its app
// container launched through native-init, whether that native-init names it
// itself (announces: the image's label, w32/m575) and its WEB_CONCURRENCY
// default; while the rollout runs, each pass adds the starts of the app
// container its pods show. A cancel's restore writes the served release's
// template and is not recorded: the caller passes only its own projections.
func (r *AppReconciler) recordReleaseRollout(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, command string, announces bool, webConcurrency int) {
	stampReleaseRollout(app, dep.Spec.Template, command, announces, webConcurrency, time.Now())
	if !releaseRolloutRunning(app, dep) || dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return
	}
	rec := app.Status.ReleaseRollout
	rev := releaseRevision(app)
	rec.AppStarts = withAppStarts(rec.AppStarts, pods.Items, rev)
	rec.FirstExit = withFirstAppExit(rec.FirstExit, pods.Items, rev)
}

// stampReleaseRollout starts the record of the release in flight once tmpl is
// its own: the release's revision label is on it. Only a release newer than
// the recorded one is stamped, so a resync keeps the start and whether the
// instance names its command, and neither a cancel's revert to the served
// release nor a survivor's redispatch, which move the release generation back,
// gives a release a second start.
func stampReleaseRollout(app *appv1alpha1.App, tmpl corev1.PodTemplateSpec, command string, announces bool, webConcurrency int, now time.Time) {
	gen := releaseGeneration(app)
	if gen <= 0 || tmpl.Labels[labelRevision] != releaseRevision(app) {
		return
	}
	if rec := app.Status.ReleaseRollout; rec != nil && rec.Generation >= gen {
		return
	}
	app.Status.ReleaseRollout = &appv1alpha1.ReleaseRolloutStatus{
		Generation: gen, StartedAt: metav1.NewMicroTime(now), Command: command, WebConcurrency: int32(webConcurrency),
		InstanceAnnounces: command != "" && announces,
	}
}

// nativeLaunchCommand is the start command app's container runs on image
// through a native image's entrypoint, native-init (runsNativeEntrypoint):
// baked into a native build's CMD, or passed as its arguments. Render quotes
// it in `==> Running '<command>'` on its native runtimes. "" for every other
// launch: a Docker image runs its own program, with or without a Docker
// Command, and Render names neither (HKUDS/nanobot#6607).
func nativeLaunchCommand(app *appv1alpha1.App, image string, launch launchMode) string {
	if !runsNativeEntrypoint(app, image, launch) {
		return ""
	}
	return app.Spec.StartCommand
}

// releaseRolloutRunning reports that the recorded rollout is still running:
// it is the release in flight's, dep carries that release's template, and the
// release has neither served nor been settled failed or canceled. Its app
// starts after that are the service's, not the deploy's.
func releaseRolloutRunning(app *appv1alpha1.App, dep *appsv1.Deployment) bool {
	rec, gen, rev := app.Status.ReleaseRollout, releaseGeneration(app), releaseRevision(app)
	if rec == nil || rec.Generation != gen || dep.Spec.Template.Labels[labelRevision] != rev ||
		app.Status.ActiveRevision == rev || canceledOverServed(app) {
		return false
	}
	if f := app.Status.RolloutFailure; f != nil && f.Generation == gen {
		return false
	}
	verdict := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout)
	return verdict == nil || verdict.Status != metav1.ConditionFalse || verdict.ObservedGeneration != gen
}

// withAppStarts adds the app-container starts revision's pods show to
// recorded, oldest first, keeping the oldest
// appv1alpha1.MaxReleaseRolloutAppStarts. A container shows its current
// run's start and, once it has restarted, the run before; a pass that sees
// each run reads every start. It returns recorded itself when the pods show
// nothing new, so an unchanged status is not rewritten.
func withAppStarts(recorded []metav1.Time, pods []corev1.Pod, revision string) []metav1.Time {
	if len(recorded) >= appv1alpha1.MaxReleaseRolloutAppStarts {
		return recorded
	}
	known := func(list []metav1.Time, t metav1.Time) bool {
		return slices.ContainsFunc(list, func(k metav1.Time) bool { return k.Time.Equal(t.Time) })
	}
	var found []metav1.Time
	add := func(t metav1.Time) {
		if !t.IsZero() && !known(recorded, t) && !known(found, t) {
			found = append(found, t)
		}
	}
	for i := range pods {
		pod := &pods[i]
		if pod.Labels[labelRevision] != revision || !pod.DeletionTimestamp.IsZero() {
			continue
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != appContainerName {
				continue
			}
			if t := cs.LastTerminationState.Terminated; t != nil {
				add(t.StartedAt)
			}
			if run := cs.State.Running; run != nil {
				add(run.StartedAt)
			}
			if t := cs.State.Terminated; t != nil {
				add(t.StartedAt)
			}
		}
	}
	if len(found) == 0 {
		return recorded
	}
	all := append(slices.Clone(recorded), found...)
	slices.SortFunc(all, func(a, b metav1.Time) int { return a.Compare(b.Time) })
	return all[:min(len(all), appv1alpha1.MaxReleaseRolloutAppStarts)]
}

// withFirstAppExit records the earliest non-OOM termination of revision's app
// container across its non-deleting pods (w32/m591). Out-of-memory kills are
// skipped because Render's `==> Exited with status N` verdict does not cover
// OOM kills. It returns recorded itself when the pods show no earlier
// termination, so an unchanged status is not rewritten needlessly.
func withFirstAppExit(recorded *appv1alpha1.ReleaseRolloutExitStatus, pods []corev1.Pod, revision string) *appv1alpha1.ReleaseRolloutExitStatus {
	best := recorded
	consider := func(t *corev1.ContainerStateTerminated) {
		if t == nil || t.FinishedAt.IsZero() || t.Reason == containerReasonOOMKilled {
			return
		}
		if best == nil || t.FinishedAt.Time.Before(best.FinishedAt.Time) {
			best = &appv1alpha1.ReleaseRolloutExitStatus{FinishedAt: t.FinishedAt, ExitCode: t.ExitCode}
		}
	}
	for i := range pods {
		pod := &pods[i]
		if pod.Labels[labelRevision] != revision || !pod.DeletionTimestamp.IsZero() {
			continue
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != appContainerName {
				continue
			}
			consider(cs.LastTerminationState.Terminated)
			consider(cs.State.Terminated)
		}
	}
	return best
}
