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
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A build hold (w6/m306, docs/ADR060 §D5/§D6) is a release whose source build
// has not been dispatched: the cluster ceiling is full, or a linked datastore is
// still being created. The tenant sees a deploy sitting `queued`, but no build
// Job exists yet, so status.buildHold is the only record of the wait.

// holdBuild records that the current release's build is held for cause and
// reports whether status changed. A newer release generation starts a new hold:
// its deploy is a new wait, and Render's queued deploy is replaced, not merged.
func holdBuild(app *appv1alpha1.App, cause string, now time.Time) bool {
	generation := releaseGeneration(app)
	if hold := app.Status.BuildHold; hold != nil && hold.Generation == generation {
		if hold.Cause == cause {
			return false
		}
		hold.Cause = cause
		return true
	}
	app.Status.BuildHold = &appv1alpha1.BuildHoldStatus{Generation: generation, Since: metav1.NewTime(now), Cause: cause}
	return true
}

// endBuildHold clears the hold once the release's build exists, observing the
// whole wait for the current generation, and reports whether status changed.
func endBuildHold(app *appv1alpha1.App, now time.Time) bool {
	hold := app.Status.BuildHold
	if hold == nil {
		return false
	}
	if hold.Generation == releaseGeneration(app) {
		recordBuildHoldSeconds(hold.Cause, now.Sub(hold.Since.Time).Seconds())
	}
	app.Status.BuildHold = nil
	return true
}

// heldBuild reports the live hold of app: the current release's, while the
// App still waits as BuildQueued. A hold left behind by a release that failed,
// was canceled or moved on no longer describes a waiting deploy.
func heldBuild(app *appv1alpha1.App) *appv1alpha1.BuildHoldStatus {
	hold := app.Status.BuildHold
	if hold == nil || hold.Generation != releaseGeneration(app) || app.DeletionTimestamp != nil ||
		app.Status.Phase != appv1alpha1.PhaseBuilding {
		return nil
	}
	ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Reason != reasonBuildQueued {
		return nil
	}
	return hold
}

// heldBuilds lists the Apps with a live hold, of cause when cause is non-empty.
// The listing is read-only, so it skips the cache's per-object deep copy: the
// admission gate calls it on every new dispatch, and a datastore wait re-enters
// the gate every 10 s. Never mutate the returned Apps.
func heldBuilds(ctx context.Context, apps client.Reader, cause string) ([]*appv1alpha1.App, error) {
	var list appv1alpha1.AppList
	if err := apps.List(ctx, &list, client.UnsafeDisableDeepCopy); err != nil {
		return nil, err
	}
	var held []*appv1alpha1.App
	for i := range list.Items {
		if hold := heldBuild(&list.Items[i]); hold != nil && (cause == "" || hold.Cause == cause) {
			held = append(held, &list.Items[i])
		}
	}
	return held, nil
}

// fairOrderAhead counts the releases held by admission that rank ahead of app
// for a free cluster slot (docs/ADR060 §D6): first the workspace running fewer
// builds, then the longer wait, then the name. app ranks with its own hold, or
// as a newcomer arriving now.
//
// A release held by its own workspace's cap (BEX_MAX_CONCURRENT_BUILDS,
// self-hosters only) is counted here too. That is harmless while the cap is the
// same for every workspace: a workspace at its cap runs at least as many builds
// as any workspace below it, so its release never ranks ahead of one that could
// take the slot.
func (r *AppReconciler) fairOrderAhead(ctx context.Context, app *appv1alpha1.App, byWorkspace map[string]int, now time.Time) (int, error) {
	held, err := heldBuilds(ctx, r.Client, appv1alpha1.BuildHoldAdmission)
	if err != nil {
		return 0, err
	}
	self := heldEntryFor(app, now)
	ahead := 0
	for _, other := range held {
		if other.UID != app.UID && heldEntryFor(other, now).ranksBefore(self, byWorkspace) {
			ahead++
		}
	}
	return ahead, nil
}

type heldEntry struct {
	workspace string
	since     time.Time
	key       string
}

// heldEntryFor places app in the fair order: at its hold's start, or at now
// when it has none yet.
func heldEntryFor(app *appv1alpha1.App, now time.Time) heldEntry {
	entry := heldEntry{workspace: app.Labels[labelWorkspace], since: now, key: app.Namespace + "/" + app.Name}
	if hold := heldBuild(app); hold != nil {
		entry.since = hold.Since.Time
	}
	return entry
}

func (e heldEntry) ranksBefore(o heldEntry, byWorkspace map[string]int) bool {
	if a, b := byWorkspace[e.workspace], byWorkspace[o.workspace]; a != b {
		return a < b
	}
	if !e.since.Equal(o.since) {
		return e.since.Before(o.since)
	}
	return e.key < o.key
}

// buildSlotWakeBuffer bounds the wake channel. A dropped wake costs one held
// release a 30-second requeue, never its build.
const buildSlotWakeBuffer = 256

// wakeHeldBuilds enqueues the held releases first in line for the slots free
// now, so the next build starts within seconds of one ending instead of on its
// 30-second requeue (docs/ADR060 §D6). Only those: waking every held release
// would have each re-list the build plane for a slot just one can take. One
// extra is woken in case the first in line has meanwhile moved on. Best
// effort: the requeue remains the fallback.
func (r *AppReconciler) wakeHeldBuilds(ctx context.Context, buildNs string) {
	if r.buildSlotFreed == nil || r.MaxActiveBuilds <= 0 {
		return
	}
	held, err := heldBuilds(ctx, r.Client, appv1alpha1.BuildHoldAdmission)
	if err != nil || len(held) == 0 {
		return
	}
	active, byWorkspace, err := build.ActiveBuildsByWorkspace(ctx, r.buildPlaneClient(), buildNs)
	if err != nil {
		logf.FromContext(ctx).Error(err, "count builds to wake held releases; they retry on their requeue")
		return
	}
	now := time.Now()
	sort.Slice(held, func(a, b int) bool {
		return heldEntryFor(held[a], now).ranksBefore(heldEntryFor(held[b], now), byWorkspace)
	})
	for _, app := range held[:min(len(held), max(r.MaxActiveBuilds-active, 0)+1)] {
		wake := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
		select {
		case r.buildSlotFreed <- event.GenericEvent{Object: wake}:
		default:
			return
		}
	}
}
