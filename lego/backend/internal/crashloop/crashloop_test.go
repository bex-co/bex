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

	corev1 "k8s.io/api/core/v1"
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

// failing is the status the operator writes for a served release whose pods
// keep crashing: phase Deploying (reportRolloutProgress handles every
// not-ready Deployment alike), Ready=False with the diagnosis, and the
// current release (rev-1) already active.
func failing(reason string, since time.Time) appOpt {
	return func(a *appv1alpha1.App) {
		a.Status.Phase = appv1alpha1.PhaseDeploying
		a.Status.ActiveRevision = "rev-1"
		a.Status.ReleaseGeneration = 1
		a.Status.Conditions = []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason,
			LastTransitionTime: metav1.NewTime(since),
		}}
	}
}

// rollingOut makes the current release a newer one (rev-2) that has not served
// yet: a deploy in progress over the served rev-1.
func rollingOut(a *appv1alpha1.App) {
	a.Status.ReleaseGeneration = 2
}

// failedFirstRelease is the operator's stopped first release (w1/m179):
// nothing served, the release's rollout verdict is in, phase Failed with the
// diagnosis on Ready, and the Deployment parked at zero (no pods).
func failedFirstRelease(a *appv1alpha1.App) {
	a.Status.Phase = appv1alpha1.PhaseFailed
	a.Status.ActiveRevision = ""
	a.Status.Conditions = append(a.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonCrashLoopBackOff,
		ObservedGeneration: 1, LastTransitionTime: a.Status.Conditions[0].LastTransitionTime,
	})
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

// pod is one of app's instances. restarts and waiting describe its app
// container; ready its Ready condition.
func pod(app, suffix string, restarts int32, waiting string, ready bool) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: app + "-" + suffix, Namespace: "tea-a", Labels: map[string]string{
			core.PodLabelApp: app, core.LabelAppID: "srv-" + app,
		}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	cs := corev1.ContainerStatus{Name: core.AppContainer, RestartCount: restarts}
	if waiting != "" {
		cs.State.Waiting = &corev1.ContainerStateWaiting{Reason: waiting}
	}
	p.Status.ContainerStatuses = []corev1.ContainerStatus{cs}
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: status}}
	return p
}

// crashing is an instance of app in CrashLoopBackOff.
func crashing(app string) *corev1.Pod {
	return pod(app, "crash", 12, appv1alpha1.ReasonCrashLoopBackOff, false)
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
		// A once-healthy service now crash-looping: the main real-world case
		// (w2/m253 dev-2 walk finding B).
		app("crashing", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day))), crashing("crashing"),
		app("unhealthy", failing(appv1alpha1.ReasonHealthCheckFailing, now.Add(-3*day))),
		pod("unhealthy", "probe", 4, "", false),
		// A newer release's rollout failed and settled; the served release's
		// pods are the ones crash-looping.
		app("regressed", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day)), rollingOut, func(a *appv1alpha1.App) {
			a.Status.Conditions = append(a.Status.Conditions, metav1.Condition{
				Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: "ProgressDeadlineExceeded",
				ObservedGeneration: 2, LastTransitionTime: metav1.NewTime(now.Add(-3 * day)),
			})
		}), crashing("regressed"),
		app("young", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(time.Hour))), crashing("young"),
		app("stalled", failing("Unschedulable", now.Add(-3*day))),
		app("healthy", func(a *appv1alpha1.App) {
			a.Status.Phase = appv1alpha1.PhaseRunning
			a.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Ready", LastTransitionTime: metav1.NewTime(now.Add(-3 * day))}}
		}),
		// A genuine rollout of a newer release whose new pods crash: a deploy
		// in progress, settled by its own deadline.
		app("deploying", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day)), rollingOut), crashing("deploying"),
		app("building", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day)), func(a *appv1alpha1.App) { a.Status.Phase = appv1alpha1.PhaseBuilding }),
		crashing("building"),
		// A failed first release parked at zero (finding A).
		app("stopped", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day)), failedFirstRelease),
		// One instance of three still serves.
		app("degraded", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day))), crashing("degraded"),
		pod("degraded", "ok", 0, "", true),
		// The status still reads crash-looping but no instance is left.
		app("gone", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-3*day))),
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
	for _, name := range []string{"crashing", "unhealthy", "regressed"} {
		a := h.get(name)
		if !a.Spec.Suspended || a.Annotations[core.AnnotCrashLoopSuspended] != h.now.Format(time.RFC3339) || a.Annotations[core.AnnotCrashLoopObserved] != "" {
			t.Errorf("%s: suspended=%v annotations=%v", name, a.Spec.Suspended, a.Annotations)
		}
	}
	for _, name := range []string{"young", "stalled", "healthy", "deploying", "building", "stopped", "degraded", "gone", "cron"} {
		if h.get(name).Spec.Suspended {
			t.Errorf("%s must keep running", name)
		}
	}
	for _, name := range []string{"stalled", "healthy", "deploying", "building", "stopped", "degraded", "gone", "cron"} {
		if got := h.get(name).Annotations[core.AnnotCrashLoopObserved]; got != "" {
			t.Errorf("%s is not crash-looping but was sighted: %q", name, got)
		}
	}
	for _, name := range []string{"parked", "capped"} {
		if a := h.get(name); a.Annotations[core.AnnotCrashLoopSuspended] != "" || a.Annotations[core.AnnotCrashLoopObserved] != "" {
			t.Errorf("%s: an existing suspension must not be touched: %v", name, a.Annotations)
		}
	}
	if len(h.notes.notices) != 3 {
		t.Fatalf("notices = %+v, want one per suspended service", h.notes.notices)
	}
	for _, n := range h.notes.notices {
		if n.tenant != "tea-a" || (n.service != "srv-crashing" && n.service != "srv-unhealthy" && n.service != "srv-regressed") || !n.since.Equal(now) {
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
	}), crashing("resumed"))
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
	}), crashing("looping"))
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
	}), crashing("dev"))
	h.sw.Window = 5 * time.Minute
	h.sweep()
	if !h.get("dev").Spec.Suspended {
		t.Error("a 5-minute window (BEX_CRASHLOOP_SUSPEND_AFTER) must apply")
	}
}

// w2/m253 finding A: a first release whose rollout failed is parked at zero
// replicas by the operator; its Failed phase and CrashLoopBackOff Ready
// condition are the failed deploy's record, not restarts. It is never sighted
// or suspended, and a resume (which the operator answers by parking it again)
// does not make it so either.
func TestStoppedFirstReleaseIsNeverSuspended(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, now, app("stopped", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-72*time.Hour)), failedFirstRelease, func(a *appv1alpha1.App) {
		a.Generation = 3 // resumed after an earlier suspension
		a.Annotations = map[string]string{core.AnnotCrashLoopObserved: apps.CrashLoopObservation(3, now.Add(-48*time.Hour))}
	}))
	h.sweep()
	h.now = now.Add(48 * time.Hour)
	h.sweep()
	a := h.get("stopped")
	if a.Spec.Suspended || a.Annotations[core.AnnotCrashLoopSuspended] != "" || len(h.notes.notices) != 0 {
		t.Fatalf("a stopped first release was suspended: suspended=%v annotations=%v notices=%v", a.Spec.Suspended, a.Annotations, h.notes.notices)
	}
	if got := a.Annotations[core.AnnotCrashLoopObserved]; got != "" {
		t.Errorf("stale sighting kept: %q", got)
	}
}

// w2/m253 finding B: a service that served and then started crashing reads
// Deploying (with Ready CrashLoopBackOff) and is suspended after the window;
// a moment with an instance Ready restarts the window.
func TestOnceHealthyServiceThatCrashLoopsIsSuspended(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	h := newHarness(t, now, app("api", failing(appv1alpha1.ReasonCrashLoopBackOff, now.Add(-time.Hour))), crashing("api"))
	h.sweep()
	if got := h.get("api").Annotations[core.AnnotCrashLoopObserved]; got != apps.CrashLoopObservation(0, now) {
		t.Fatalf("sighting = %q, want one at %s", got, now)
	}

	// Twelve hours in, an instance comes up Ready: the sighting is cleared.
	h.now = now.Add(12 * time.Hour)
	ready := pod("api", "up", 0, "", true)
	if err := h.cl.Create(context.Background(), ready); err != nil {
		t.Fatal(err)
	}
	h.sweep()
	if got := h.get("api").Annotations[core.AnnotCrashLoopObserved]; got != "" {
		t.Fatalf("a Ready instance must clear the sighting, got %q", got)
	}
	if err := h.cl.Delete(context.Background(), ready); err != nil {
		t.Fatal(err)
	}

	// It crashes again: a fresh window from the next sighting.
	h.now = now.Add(13 * time.Hour)
	h.sweep()
	h.now = now.Add(13*time.Hour + 24*time.Hour - time.Second)
	h.sweep()
	if h.get("api").Spec.Suspended {
		t.Fatal("suspended before a full window after the healthy moment")
	}
	h.now = now.Add(13*time.Hour + 24*time.Hour)
	h.sweep()
	a := h.get("api")
	if !a.Spec.Suspended || a.Annotations[core.AnnotCrashLoopSuspended] == "" || len(h.notes.notices) != 1 {
		t.Fatalf("suspended=%v annotations=%v notices=%v", a.Spec.Suspended, a.Annotations, h.notes.notices)
	}
	if n := h.notes.notices[0]; n.service != "srv-api" || !n.since.Equal(now.Add(13*time.Hour)) {
		t.Errorf("notice %+v", n)
	}
}
