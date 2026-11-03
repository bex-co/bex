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

package crashloop

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

type notice struct {
	tenant, service, name string
	since                 time.Time
}

type recordingNotifier struct{ notices []notice }

func (n *recordingNotifier) NotifyCrashLoopSuspended(_ context.Context, tenant, service, name string, since, _ time.Time) error {
	n.notices = append(n.notices, notice{tenant, service, name, since})
	return nil
}

type appOpt func(*appv1alpha1.App)

func failing(reason string, since time.Time) appOpt {
	return func(a *appv1alpha1.App) {
		a.Status.Phase = appv1alpha1.PhaseRunning
		a.Status.Conditions = []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason,
			LastTransitionTime: metav1.NewTime(since),
		}}
	}
}

func app(name string, opts ...appOpt) *appv1alpha1.App {
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "tea-a", Labels: map[string]string{
			core.LabelTenant: "tea-a", core.LabelAppID: "srv-" + name,
		}},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Tier: "starter", Image: "nginx:1"},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

type harness struct {
	t     *testing.T
	cl    client.Client
	sw    *Sweeper
	notes *recordingNotifier
	now   time.Time
}

func newHarness(t *testing.T, now time.Time, objs ...client.Object) *harness {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, now: now, notes: &recordingNotifier{}}
	h.cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	base := &core.Base{Client: h.cl, Namespace: "default", Clock: func() time.Time { return h.now }}
	h.sw = &Sweeper{Apps: apps.CrashLoopWorker{Svc: &apps.Service{Base: base}}, Notifier: h.notes}
	return h
}

func (h *harness) sweep() {
	h.t.Helper()
	if err := h.sw.SweepOnce(context.Background()); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) get(name string) *appv1alpha1.App {
	h.t.Helper()
	var a appv1alpha1.App
	if err := h.cl.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: name}, &a); err != nil {
		h.t.Fatal(err)
	}
	return &a
}

func TestSweepSuspendsOnlyServicesFailingForTheWholeWindow(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	h := newHarness(t, now,
		app("crashing", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day))),
		app("unhealthy", failing(appv1alpha1.ReasonHealthCheckFailing, now.Add(-3*day))),
		app("young", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(time.Hour))),
		app("stalled", failing("Unschedulable", now.Add(-3*day))),
		app("healthy", func(a *appv1alpha1.App) {
			a.Status.Phase = appv1alpha1.PhaseRunning
			a.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Ready", LastTransitionTime: metav1.NewTime(now.Add(-3 * day))}}
		}),
		app("deploying", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day)), func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseDeploying }),
		app("cron", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day)), func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeCronJob }),
		app("parked", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day)), func(a *appv1alpha1.App) { a.Spec.Suspended = true }),
		app("capped", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day)), func(a *appv1alpha1.App) {
			a.Annotations = map[string]string{core.AnnotFreeHoursSuspended: "2026-11"}
		}),
	)

	// First sweep: a long-failing service the sweeper has never seen gets a
	// sighting, not a suspension — the window is the sweeper's own, never a
	// transition time it did not witness.
	h.sweep()
	if h.get("crashing").Spec.Suspended || len(h.notes.notices) != 0 {
		t.Fatal("suspended on the first sighting")
	}
	if got := h.get("crashing").Annotations[core.AnnotCrashLoopObserved]; got != apps.CrashLoopObservation(0, now) {
		t.Fatalf("sighting = %q", got)
	}

	h.now = now.Add(day - time.Second)
	h.sweep()
	if h.get("crashing").Spec.Suspended {
		t.Fatal("suspended a second before the window closed")
	}

	h.now = now.Add(day)
	h.sweep()
	h.sweep() // a second sweep suspends and mails nothing more
	for _, name := range []string{"crashing", "unhealthy"} {
		a := h.get(name)
		if !a.Spec.Suspended || a.Annotations[core.AnnotCrashLoopSuspended] != h.now.Format(time.RFC3339) || a.Annotations[core.AnnotCrashLoopObserved] != "" {
			t.Errorf("%s: suspended=%v annotations=%v", name, a.Spec.Suspended, a.Annotations)
		}
	}
	for _, name := range []string{"young", "stalled", "healthy", "deploying", "cron"} {
		if h.get(name).Spec.Suspended {
			t.Errorf("%s must keep running", name)
		}
	}
	for _, name := range []string{"parked", "capped"} {
		if a := h.get(name); a.Annotations[core.AnnotCrashLoopSuspended] != "" || a.Annotations[core.AnnotCrashLoopObserved] != "" {
			t.Errorf("%s: an existing suspension must not be touched: %v", name, a.Annotations)
		}
	}
	if len(h.notes.notices) != 2 {
		t.Fatalf("notices = %+v, want one per suspended service", h.notes.notices)
	}
	for _, n := range h.notes.notices {
		if n.tenant != "tea-a" || (n.service != "srv-crashing" && n.service != "srv-unhealthy") || !n.since.Equal(now) {
			t.Errorf("notice %+v", n)
		}
	}
}

// A resume or deploy changes the spec generation; a sighting from an older
// generation restarts the window even though Ready never left False.
func TestNewGenerationRestartsTheWindow(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, now, app("resumed", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-72*time.Hour)), func(a *appv1alpha1.App) {
		a.Generation = 7
		a.Annotations = map[string]string{core.AnnotCrashLoopObserved: apps.CrashLoopObservation(6, now.Add(-48*time.Hour))}
	}))
	h.sweep()
	a := h.get("resumed")
	if a.Spec.Suspended || a.Annotations[core.AnnotCrashLoopObserved] != apps.CrashLoopObservation(7, now) {
		t.Fatalf("suspended=%v sighting=%q, want a fresh sighting at generation 7", a.Spec.Suspended, a.Annotations[core.AnnotCrashLoopObserved])
	}
}

func TestRecoveryClearsTheSighting(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, now, app("recovered", func(a *appv1alpha1.App) {
		a.Status.Phase = appv1alpha1.PhaseRunning
		a.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Ready", LastTransitionTime: metav1.NewTime(now)}}
		a.Annotations = map[string]string{core.AnnotCrashLoopObserved: apps.CrashLoopObservation(0, now.Add(-time.Hour))}
	}))
	h.sweep()
	if got := h.get("recovered").Annotations[core.AnnotCrashLoopObserved]; got != "" {
		t.Fatalf("sighting survived recovery: %q", got)
	}
}

type failingNotifier struct{ calls int }

func (n *failingNotifier) NotifyCrashLoopSuspended(context.Context, string, string, string, time.Time, time.Time) error {
	n.calls++
	return errors.New("smtp queue unavailable")
}

// The owners are told before the suspension is written; if they cannot be,
// nothing is suspended and the next sweep tries again.
func TestUnnotifiedServiceIsNotSuspended(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, now, app("looping", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-48*time.Hour)), func(a *appv1alpha1.App) {
		a.Annotations = map[string]string{core.AnnotCrashLoopObserved: apps.CrashLoopObservation(0, now.Add(-25*time.Hour))}
	}))
	n := &failingNotifier{}
	h.sw.Notifier = n
	if err := h.sw.SweepOnce(context.Background()); err == nil {
		t.Fatal("a failed notification must surface")
	}
	if h.get("looping").Spec.Suspended || n.calls != 1 {
		t.Fatalf("suspended=%v calls=%d", h.get("looping").Spec.Suspended, n.calls)
	}
}

func TestSweepHonorsAShorterWindow(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, now, app("dev", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-time.Hour)), func(a *appv1alpha1.App) {
		a.Annotations = map[string]string{core.AnnotCrashLoopObserved: apps.CrashLoopObservation(0, now.Add(-6*time.Minute))}
	}))
	h.sw.Window = 5 * time.Minute
	h.sweep()
	if !h.get("dev").Spec.Suspended {
		t.Error("a 5-minute window (BEX_CRASHLOOP_SUSPEND_AFTER) must apply")
	}
}
