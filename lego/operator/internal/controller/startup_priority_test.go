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
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller/priorityqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func settledApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "tea-test", Generation: 3},
		Status: appv1alpha1.AppStatus{
			ObservedGeneration: 3,
			Phase:              appv1alpha1.PhaseRunning,
			Conditions:         []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Deployed"}},
		},
	}
}

// replay delivers a manager start's informer replay the way the App controller
// receives it: For's handler (EnqueueRequestForObject, which queues the replay
// at LowPriority) and inFlightFirstAtStart, each given every App in list order.
// forFirst picks which handler sees an App first; the informer gives no order.
func replay(q workqueue.TypedRateLimitingInterface[reconcile.Request], apps []*appv1alpha1.App, forFirst bool) {
	ctx := context.Background()
	forHandler := &handler.EnqueueRequestForObject{}
	ours := inFlightFirstAtStart()
	for _, app := range apps {
		viaFor := func() { forHandler.Create(ctx, event.CreateEvent{Object: app, IsInInitialList: true}, q) }
		viaOurs := func() {
			ours.Create(ctx, event.TypedCreateEvent[*appv1alpha1.App]{Object: app, IsInInitialList: true}, q)
		}
		if forFirst {
			viaFor()
			viaOurs()
		} else {
			viaOurs()
			viaFor()
		}
	}
}

// w5/m370 t003: a maintenance toggle made during a rollout's handover reached
// the new manager inside its startup replay and waited behind the whole fleet.
// An App mid-change now comes out of the queue before every settled App, even
// when it is listed last.
func TestAnAppMidChangeLeadsTheStartupSweep(t *testing.T) {
	toggled := settledApp("toggled")
	toggled.Generation = 4 // the spec change the previous manager never acted on
	for _, forFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("for-handler-first=%v", forFirst), func(t *testing.T) {
			q := priorityqueue.New[reconcile.Request]("app")
			defer q.ShutDown()
			apps := make([]*appv1alpha1.App, 0, 7)
			for i := range 6 {
				apps = append(apps, settledApp(fmt.Sprintf("settled-%d", i)))
			}
			replay(q, append(apps, toggled), forFirst)

			if got := q.Len(); got != 7 {
				t.Fatalf("queued %d requests, want one per App (7)", got)
			}
			req, priority, _ := q.GetWithPriority()
			if req.Name != "toggled" || priority != 0 {
				t.Fatalf("first out: %s at priority %d, want toggled at 0", req.Name, priority)
			}
			q.Done(req)
			for i := range 6 {
				req, priority, _ := q.GetWithPriority()
				if want := fmt.Sprintf("settled-%d", i); req.Name != want || priority != handler.LowPriority {
					t.Fatalf("out %d: %s at priority %d, want %s at %d", i+2, req.Name, priority, want, handler.LowPriority)
				}
				q.Done(req)
			}
		})
	}
}

// Outside the replay the handler adds nothing: For's handler already queues a
// live Create at the default priority, and a second add would only duplicate it.
func TestInFlightFirstAtStartIgnoresEverythingButTheReplay(t *testing.T) {
	ctx := context.Background()
	pending := settledApp("pending")
	pending.Generation = 4
	ours := inFlightFirstAtStart()

	q := priorityqueue.New[reconcile.Request]("app")
	defer q.ShutDown()
	ours.Create(ctx, event.TypedCreateEvent[*appv1alpha1.App]{Object: pending}, q)
	ours.Create(ctx, event.TypedCreateEvent[*appv1alpha1.App]{Object: settledApp("settled"), IsInInitialList: true}, q)
	ours.Update(ctx, event.TypedUpdateEvent[*appv1alpha1.App]{ObjectOld: settledApp("pending"), ObjectNew: pending}, q)
	ours.Delete(ctx, event.TypedDeleteEvent[*appv1alpha1.App]{Object: pending}, q)
	ours.Generic(ctx, event.TypedGenericEvent[*appv1alpha1.App]{Object: pending}, q)
	if got := q.Len(); got != 0 {
		t.Fatalf("queued %d requests outside the replay, want 0", got)
	}

	// A queue without priorities replays in FIFO order and For has queued the
	// App already: nothing to raise, and no panic.
	plain := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer plain.ShutDown()
	ours.Create(ctx, event.TypedCreateEvent[*appv1alpha1.App]{Object: pending, IsInInitialList: true}, plain)
	if got := plain.Len(); got != 0 {
		t.Fatalf("queued %d requests on a plain queue, want 0", got)
	}
}

func TestAppInFlight(t *testing.T) {
	now := metav1.NewTime(time.Now())
	for _, tc := range []struct {
		name   string
		mutate func(*appv1alpha1.App)
		want   bool
	}{
		{"serving and Ready", func(*appv1alpha1.App) {}, false},
		{"a spec change not yet observed", func(a *appv1alpha1.App) { a.Generation++ }, true},
		{"being deleted", func(a *appv1alpha1.App) { a.DeletionTimestamp = &now }, true},
		{"never reconciled", func(a *appv1alpha1.App) { a.Status = appv1alpha1.AppStatus{ObservedGeneration: a.Generation} }, true},
		{"pending", func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhasePending }, true},
		{"building", func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseBuilding }, true},
		{"deploying", func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseDeploying }, true},
		{"running while a rollout settles", func(a *appv1alpha1.App) {
			a.Status.Conditions[0].Status, a.Status.Conditions[0].Reason = metav1.ConditionFalse, reasonRolloutSettling
		}, true},
		{"hibernated", func(a *appv1alpha1.App) {
			a.Status.Phase, a.Status.Conditions[0].Status = appv1alpha1.PhaseHibernated, metav1.ConditionFalse
		}, false},
		{"failed", func(a *appv1alpha1.App) {
			a.Status.Phase, a.Status.Conditions[0].Status = appv1alpha1.PhaseFailed, metav1.ConditionFalse
		}, false},
		{"canceled", func(a *appv1alpha1.App) {
			a.Status.Phase, a.Status.Conditions[0].Status = appv1alpha1.PhaseCanceled, metav1.ConditionFalse
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := settledApp("a")
			tc.mutate(app)
			if got := appInFlight(app); got != tc.want {
				t.Fatalf("appInFlight = %v, want %v", got, tc.want)
			}
		})
	}
}
