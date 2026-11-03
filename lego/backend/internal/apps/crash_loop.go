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

// CrashLoopFailingSince reports whether a is failing to start and become
// healthy now, and since when its Ready condition has been False. Services the
// owner or the platform already parked, services mid-build or mid-deploy (a fix
// may be on its way), and cron jobs and static sites (no long-running instance)
// are never failing in this sense.
func CrashLoopFailingSince(a *appv1alpha1.App) (time.Time, bool) {
	if a.Spec.Suspended || a.Annotations[core.AnnotFreeHoursSuspended] != "" || a.Annotations[core.AnnotCrashLoopSuspended] != "" {
		return time.Time{}, false
	}
	switch effectiveType(a.Spec.Type) {
	case appv1alpha1.TypeCronJob, appv1alpha1.TypeStaticSite:
		return time.Time{}, false
	}
	switch a.Status.Phase {
	case appv1alpha1.PhaseRunning, appv1alpha1.PhaseFailed:
	default:
		return time.Time{}, false
	}
	ready := meta.FindStatusCondition(a.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || !crashLoopReasons[ready.Reason] {
		return time.Time{}, false
	}
	return ready.LastTransitionTime.Time, true
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
// with no caller identity, like FreeHoursWorker.
type CrashLoopWorker struct{ Svc *Service }

// Now is the service clock.
func (w CrashLoopWorker) Now() time.Time { return w.Svc.Now() }

// Suspend suspends a for a crash loop.
func (w CrashLoopWorker) Suspend(ctx context.Context, a *appv1alpha1.App, at time.Time) error {
	return w.Svc.suspendForCrashLoop(ctx, a, at)
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
