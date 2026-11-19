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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/priorityqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// inFlightFirstAtStart lifts an App that was mid-change when the manager
// started ahead of the rest of the fleet (w5/m370 t003).
//
// A starting manager queues every App once (informer replay), and
// controller-runtime queues that replay at handler.LowPriority, so a change
// made after the start jumps the sweep. A change made while no manager led
// (the rollout handover) arrives inside the replay, though, and waited for
// its turn: on production two workers took about 75 s to sweep 29 Apps, at
// 0.5 to 2.5 s a pass, and the sweep grows with the fleet. This handler,
// registered beside For's, re-adds such an App at the default priority; the
// queue only ever raises an item's priority, so the order the two handlers
// run in does not matter. Every other event is For's alone.
func inFlightFirstAtStart() handler.TypedEventHandler[*appv1alpha1.App, reconcile.Request] {
	return handler.TypedFuncs[*appv1alpha1.App, reconcile.Request]{
		CreateFunc: func(_ context.Context, e event.TypedCreateEvent[*appv1alpha1.App], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			if !e.IsInInitialList || !appInFlight(e.Object) {
				return
			}
			// Without a priority queue the replay is plain FIFO and For's
			// handler has already queued the App. The priority is explicit
			// because TypedFuncs hands a replay Create a queue that defaults
			// to LowPriority.
			if pq, ok := q.(priorityqueue.PriorityQueue[reconcile.Request]); ok {
				pq.AddWithOpts(priorityqueue.AddOpts{Priority: new(0)},
					reconcile.Request{NamespacedName: client.ObjectKeyFromObject(e.Object)})
			}
		},
	}
}

// appInFlight reports whether an App is mid-change: being deleted, carrying a
// spec generation the operator has not acted on, or with a release still
// building or rolling out. A settled App (serving and Ready, hibernated,
// failed or canceled, with its generation observed) waits for the sweep.
func appInFlight(app *appv1alpha1.App) bool {
	if !app.DeletionTimestamp.IsZero() || app.Generation != app.Status.ObservedGeneration {
		return true
	}
	switch app.Status.Phase {
	case "", appv1alpha1.PhasePending, appv1alpha1.PhaseBuilding, appv1alpha1.PhaseDeploying:
		return true
	case appv1alpha1.PhaseRunning:
		return !meta.IsStatusConditionTrue(app.Status.Conditions, appv1alpha1.ConditionReady)
	}
	return false
}
