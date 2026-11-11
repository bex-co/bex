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

package apps

import (
	"context"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Crash-loop suspension (w2/m253, docs/ADR096-crashloop-suspension.md):
// "Render suspends a service that repeatedly fails to start and become healthy
// over a 24-hour period" (render.com/docs/notifications), reported with the
// suspender `stuck_crashlooping`.

// crashLoopReasons are the operator's Ready=False reasons for an instance that
// keeps failing to start or runs but never passes its health check.
var crashLoopReasons = map[string]bool{
	appv1alpha1.ReasonCrashLoopBackOff:   true,
	appv1alpha1.ReasonHealthCheckFailing: true,
}

// CrashLoopFailingSince reports whether a's own status says it is failing to
// start and become healthy now, and since when its Ready condition has been
// False. The rule (docs/ADR096-crashloop-suspension.md § Rule):
//
//   - Ready is False with a start-failure reason (crashLoopReasons);
//   - the phase is Running, Deploying or Failed. A served release whose pods
//     start crashing reads Running with Ready CrashLoopBackOff since w4/m327
//     (Deploying before, and still after a wake), so the phase alone does
//     not decide;
//   - the release those pods run has finished rolling out
//     (crashLoopReleaseSettled): a newer release still rolling out is a
//     deploy in progress, and a first release that never served is a failed
//     deploy the operator has stopped at zero replicas, with no restarts.
//
// Services the owner or the platform already parked, services mid-build, and
// cron jobs and static sites (no long-running instance) are never failing in
// this sense. The status alone cannot tell whether restarts continue or
// whether one instance still serves; the sweeper asks the pods for that
// (CrashLoopRestarting).
func CrashLoopFailingSince(a *appv1alpha1.App) (time.Time, bool) {
	if a.Spec.Suspended || a.Annotations[core.AnnotFreeHoursSuspended] != "" || a.Annotations[core.AnnotCrashLoopSuspended] != "" {
		return time.Time{}, false
	}
	switch effectiveType(a.Spec.Type) {
	case appv1alpha1.TypeCronJob, appv1alpha1.TypeStaticSite:
		return time.Time{}, false
	}
	switch a.Status.Phase {
	case appv1alpha1.PhaseRunning, appv1alpha1.PhaseDeploying, appv1alpha1.PhaseFailed:
	default:
		return time.Time{}, false
	}
	ready := meta.FindStatusCondition(a.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || !crashLoopReasons[ready.Reason] {
		return time.Time{}, false
	}
	if !crashLoopReleaseSettled(a) {
		return time.Time{}, false
	}
	return ready.LastTransitionTime.Time, true
}

// crashLoopReleaseSettled reports whether no rollout is in progress for a:
// its current release (status.releaseGeneration) has served — the active
// revision is that release's — or that release's rollout already settled
// failed (a Rollout=False verdict for it) over an earlier release that served.
// Either way the pods that are restarting are a steady state, not a deploy
// that may still succeed.
//
// A release that never served (no active revision) is excluded even when its
// verdict is in: the operator parks a failed first release at zero replicas
// (w1/m179), so its CrashLoopBackOff Ready condition and Failed phase are a
// record of the failed deploy, not restarts. A status written before release
// generations existed has only the active revision, which is then the
// release.
func crashLoopReleaseSettled(a *appv1alpha1.App) bool {
	active := a.Status.ActiveRevision
	if active == "" {
		return false
	}
	release := a.Status.ReleaseGeneration
	if release <= 0 {
		return true
	}
	if served, ok := appv1alpha1.RevisionGeneration(active); ok && served == release {
		return true
	}
	verdict := meta.FindStatusCondition(a.Status.Conditions, appv1alpha1.ConditionRollout)
	return verdict != nil && verdict.Status == metav1.ConditionFalse && verdict.ObservedGeneration == release
}

// CrashLoopRestarting reports whether a service's pods show a crash loop that
// is still turning: none of its live instances is Ready, and at least one
// app container has restarted or is waiting out a CrashLoopBackOff. No live
// pod at all means nothing is restarting (a service parked at zero), and one
// Ready instance means the service still serves; neither is suspended.
func CrashLoopRestarting(pods []corev1.Pod) bool {
	restarting := false
	for i := range pods {
		p := &pods[i]
		if p.DeletionTimestamp != nil || p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				return false
			}
		}
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Name != core.AppContainer {
				continue
			}
			if cs.RestartCount > 0 || (cs.State.Waiting != nil && cs.State.Waiting.Reason == appv1alpha1.ReasonCrashLoopBackOff) {
				restarting = true
			}
		}
	}
	return restarting
}

// CrashLoopObservation is the AnnotCrashLoopObserved value for a first
// sighting at generation and at.
func CrashLoopObservation(generation int64, at time.Time) string {
	return strconv.FormatInt(generation, 10) + "@" + at.UTC().Format(time.RFC3339)
}

// CrashLoopObservedAt is when the sweeper first saw a failing at its current
// generation, or false when it has no sighting for this generation.
func CrashLoopObservedAt(a *appv1alpha1.App) (time.Time, bool) {
	gen, at, ok := strings.Cut(a.Annotations[core.AnnotCrashLoopObserved], "@")
	if !ok || gen != strconv.FormatInt(a.Generation, 10) {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, at)
	return t, err == nil
}

// suspendForCrashLoop suspends a for a crash loop. It is an internal worker
// verb: no caller identity, no audit row (the observed service_suspended event
// records it). A service already parked is left alone.
func (s *Service) suspendForCrashLoop(ctx context.Context, a *appv1alpha1.App, at time.Time) error {
	if a.Spec.Suspended || a.Annotations[core.AnnotCrashLoopSuspended] != "" {
		return nil
	}
	_, err := s.writeThroughStoreFetched(ctx, a,
		func(ctx context.Context, id string) error { return s.Store.SetAppSuspended(ctx, id, true) },
		func(a *appv1alpha1.App) {
			a.Spec.Suspended = true
			if a.Annotations == nil {
				a.Annotations = map[string]string{}
			}
			a.Annotations[core.AnnotCrashLoopSuspended] = at.UTC().Format(time.RFC3339)
			delete(a.Annotations, core.AnnotCrashLoopObserved)
		})
	return err
}

// observeCrashLoop sets (or, with value "", removes) a's sighting annotation.
// A metadata-only merge patch: nothing the operator reconciles changes.
func (s *Service) observeCrashLoop(ctx context.Context, a *appv1alpha1.App, value string) error {
	if a.Annotations[core.AnnotCrashLoopObserved] == value {
		return nil
	}
	patched := a.DeepCopy()
	if value == "" {
		delete(patched.Annotations, core.AnnotCrashLoopObserved)
	} else {
		if patched.Annotations == nil {
			patched.Annotations = map[string]string{}
		}
		patched.Annotations[core.AnnotCrashLoopObserved] = value
	}
	return s.Client.Patch(ctx, patched, client.MergeFrom(a))
}

// CrashLoopWorker exposes the crash-loop suspension to its sweeper, a worker
// with no caller identity, like BillingSuspensionWorker.
type CrashLoopWorker struct{ Svc *Service }

// Now is the service clock.
func (w CrashLoopWorker) Now() time.Time { return w.Svc.Now() }

// Suspend suspends a for a crash loop.
func (w CrashLoopWorker) Suspend(ctx context.Context, a *appv1alpha1.App, at time.Time) error {
	return w.Svc.suspendForCrashLoop(ctx, a, at)
}

// Restarting reports whether a's pods are crash-looping now
// (CrashLoopRestarting).
func (w CrashLoopWorker) Restarting(ctx context.Context, a *appv1alpha1.App) (bool, error) {
	pods, err := w.Svc.OwnAppPods(ctx, a)
	if err != nil {
		return false, err
	}
	return CrashLoopRestarting(pods), nil
}

// Observe records (or clears, with "") the sweeper's first sighting of a.
func (w CrashLoopWorker) Observe(ctx context.Context, a *appv1alpha1.App, value string) error {
	return w.Svc.observeCrashLoop(ctx, a, value)
}

// List returns every App in the cluster.
func (w CrashLoopWorker) List(ctx context.Context) ([]appv1alpha1.App, error) {
	var list appv1alpha1.AppList
	if err := w.Svc.Client.List(ctx, &list); err != nil {
		return nil, err
	}
	return core.ServiceApps(list.Items), nil
}
