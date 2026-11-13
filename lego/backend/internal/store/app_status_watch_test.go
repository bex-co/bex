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

package store

import (
	"context"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// failRollout writes the operator's verdict for a rollout that failed, as
// TestRecordDeployClosesFailedOnCRFailed does.
func failRollout(t *testing.T, ctx context.Context, cl client.Client) {
	t.Helper()
	app := getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseFailed
	app.Status.Conditions = []metav1.Condition{{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: "IngressFailed",
		Message: "ingress rejected host example.com", ObservedGeneration: app.Generation,
	}}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
}

// TestStatusWatchClosesAFailedDeployWithoutWaitingForTheResync is w4/624's
// target behavior (w5/m369): the operator's failed verdict reaches the deploy
// row within seconds, with the resync far away. The control shows the same
// status write waits for the resync when nothing watches.
func TestStatusWatchClosesAFailedDeployWithoutWaitingForTheResync(t *testing.T) {
	watchClosesDeploy(t, failRollout, DeployUpdateFailed)
}

// settlingRollout is the operator's first word that a good rollout serves: the
// release is the active revision while the Deployment's own bookkeeping still
// converges (app_controller.go reportRolloutProgress, w4/405).
func settlingRollout(t *testing.T, ctx context.Context, cl client.Client) {
	t.Helper()
	app := getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseDeploying
	app.Status.ReleaseGeneration = 1
	app.Status.ActiveRevision = "rev-1"
	app.Status.Conditions = []metav1.Condition{{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonRolloutSettling,
		Message: "current revision fully ready; Deployment status still converging", ObservedGeneration: app.Generation,
	}}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
}

// runningRollout is the same release once the rollout has completed.
func runningRollout(t *testing.T, ctx context.Context, cl client.Client) {
	t.Helper()
	app := getApp(t, cl)
	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.ReleaseGeneration = 1
	app.Status.ActiveRevision = "rev-1"
	app.Status.Conditions = []metav1.Condition{{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Deployed",
		Message: "1/1 replicas ready", ObservedGeneration: app.Generation,
	}}
	if err := cl.Status().Update(ctx, app); err != nil {
		t.Fatal(err)
	}
}

// TestStatusWatchClosesAGoodDeployLiveWithoutWaitingForTheResync is the success
// path of the same promise (w21/064): a deploy reads live within seconds of the
// operator saying its release serves, with the resync far away. The failed-build
// test above passes for a watch that kicks on failures and not on this
// transition, so this one pins the transition itself, in both shapes the
// operator writes it (the settling stamp, then Running), each with its
// no-watch control.
func TestStatusWatchClosesAGoodDeployLiveWithoutWaitingForTheResync(t *testing.T) {
	for _, shape := range []struct {
		name  string
		write func(*testing.T, context.Context, client.Client)
	}{
		{"settling rollout with the release active", settlingRollout},
		{"running", runningRollout},
	} {
		t.Run(shape.name, func(t *testing.T) { watchClosesDeploy(t, shape.write, DeployLive) })
	}
}

// watchClosesDeploy writes the operator's status into a reconciled App after
// the projector's first pass, with a resync 10 minutes away, and asserts the
// deploy row reaches want when an App status watch is running and does not when
// none is (the control: the write alone changes nothing the projector reads).
func watchClosesDeploy(t *testing.T, write func(*testing.T, context.Context, client.Client), want string) {
	t.Helper()
	for _, watching := range []bool{true, false} {
		name := "watching"
		if !watching {
			name = "control: no watch"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			rec, s, cl := newTestReconciler(t)
			ten, _ := s.CreateTenant(ctx, "acme", "free")
			row, _ := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
			if err := rec.ReconcileOnce(ctx); err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			rec.Resync = 10 * time.Minute
			rec.Metrics = NewReconcilerMetrics(prometheus.NewRegistry())
			go rec.Run(ctx)
			// Run passes once at start; the verdict must land after that pass,
			// or the control would read it there.
			waitFor(t, "the first pass", func() bool { return passCount(t, rec.Metrics) >= 1 })

			if watching {
				started := make(chan struct{}, 1)
				w := &AppStatusWatch{Client: &signalingWatch{WithWatch: cl.(client.WithWatch), started: started}, Kick: rec.KickFromWatch}
				go w.Run(ctx)
				select {
				case <-started:
				case <-time.After(5 * time.Second):
					t.Fatal("the watch never started")
				}
			}
			write(t, ctx, cl)

			if watching {
				waitFor(t, "the deploy to close "+want+" without a resync",
					func() bool { return onlyDeploy(t, s, row.ID).Status == want })
				return
			}
			time.Sleep(time.Second)
			if onlyDeploy(t, s, row.ID).Status == want {
				t.Fatalf("deploy closed %s with no watch and the resync 10 minutes away; the control does not control", want)
			}
		})
	}
}

// signalingWatch reports when a watch is registered: the fake client replays
// nothing to a new watch, so a status written before then is never seen.
type signalingWatch struct {
	client.WithWatch
	started chan<- struct{}
}

func (s *signalingWatch) Watch(ctx context.Context, list client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
	wi, err := s.WithWatch.Watch(ctx, list, opts...)
	if err == nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	return wi, err
}

// TestStatusFingerprintNamesEveryStatusField keeps statusFingerprint's field
// choice deliberate: a status field added to the CRD must be either
// fingerprinted (a change to it kicks a pass) or listed here with why a change
// to it alone records nothing.
func TestStatusFingerprintNamesEveryStatusField(t *testing.T) {
	fingerprinted := map[string]bool{
		"Phase": true, "Conditions": true, "BuildRun": true, "BuildHold": true, "PreDeploy": true,
		"ReleaseStart": true, "ReleaseGeneration": true, "ActiveRevision": true, "RolloutFailure": true,
		"InstanceExits": true, "Runs": true, "Autoscaling": true, "ManualRunHandledAt": true, "CancelRunRecorded": true,
	}
	ignored := map[string]string{
		"ObservedGeneration":         "a spec write moves it; the conditions' own generation carries the verdict",
		"ImageNetwork":               "port discovery; read by service views, not recorded",
		"ServingNetwork":             "port discovery; read by service views, not recorded",
		"PriorServingNetwork":        "port discovery; read by service views, not recorded",
		"EnvPort":                    "port discovery; read by service views, not recorded",
		"DetectedPort":               "port discovery; read by service views, not recorded",
		"RuntimePorts":               "port scan every 2 minutes; read by service views, not recorded",
		"ImageRevision":              "read at a deploy's close, which a phase or release change kicks",
		"Image":                      "read at a deploy's close, which a phase or release change kicks",
		"ImageDigest":                "read at a deploy's close, which a phase or release change kicks",
		"URL":                        "read by service views, not recorded",
		"URLs":                       "read by service views, not recorded",
		"Disk":                       "disk facts come from the disk verbs and the usage sampler",
		"InitialDeployHookCompleted": "operator bookkeeping",
		"ProtectedImageTag":          "operator bookkeeping",
		"StoredImages":               "operator bookkeeping",
		"ArtifactFingerprint":        "operator bookkeeping",
		"ArtifactImage":              "operator bookkeeping",
		"BuildCacheGeneration":       "operator bookkeeping",
		"StaticPrefix":               "operator bookkeeping",
		"ReleaseFingerprint":         "operator bookkeeping",
		"ReleaseArtifactFingerprint": "operator bookkeeping",
		"ConfigSnapshotGeneration":   "read by service views, not recorded",
		"UnscopedSnapshotGeneration": "read by service views, not recorded",
		"UndeployedChanges":          "read by service views, not recorded",
		"UndeployedChangesApplying":  "read by service views, not recorded",
		"UndeployedChangesReason":    "read by service views, not recorded",
		"PendingReleaseGeneration":   "the pending release's verdict arrives through the phase and conditions",
		"SelectedReleaseGeneration":  "the selected release's verdict arrives through the phase and conditions",
		"LastSuccessfulRunAt":        "moves when a run in Runs succeeds, which already kicks; read by service views",
		"SandboxID":                  "sandbox runtime only",
		"Endpoint":                   "sandbox runtime only",
	}
	fields := reflect.TypeFor[appv1alpha1.AppStatus]()
	for i := range fields.NumField() {
		name := fields.Field(i).Name
		_, skip := ignored[name]
		if fingerprinted[name] == skip {
			t.Errorf("AppStatus.%s must be either fingerprinted or ignored with a reason, not %s", name,
				map[bool]string{true: "both", false: "neither"}[skip])
		}
		delete(ignored, name)
		delete(fingerprinted, name)
	}
	for name := range ignored {
		t.Errorf("ignored field %s is not in AppStatus", name)
	}
	for name := range fingerprinted {
		t.Errorf("fingerprinted field %s is not in AppStatus", name)
	}
}

func TestStatusFingerprintIgnoresWhatRecordsNothing(t *testing.T) {
	base := func() appv1alpha1.AppStatus {
		return appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning,
			Conditions: []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionTrue, Reason: "Available", Message: "1/1 ready",
				ObservedGeneration: 3, LastTransitionTime: metav1.NewTime(time.Unix(100, 0)),
			}},
			ActiveRevision: "rev-1",
		}
	}
	initial := base()
	want := statusFingerprint(&initial)

	same := map[string]func(*appv1alpha1.AppStatus){
		"a reworded condition message": func(s *appv1alpha1.AppStatus) { s.Conditions[0].Message = "ready" },
		"a re-stamped transition time": func(s *appv1alpha1.AppStatus) {
			s.Conditions[0].LastTransitionTime = metav1.NewTime(time.Unix(200, 0))
		},
		"a port scan":                     func(s *appv1alpha1.AppStatus) { s.DetectedPort = 8080 },
		"the generation of a spec change": func(s *appv1alpha1.AppStatus) { s.ObservedGeneration = 9 },
	}
	for name, change := range same {
		s := base()
		change(&s)
		if got := statusFingerprint(&s); got != want {
			t.Errorf("%s changed the fingerprint; it records nothing and must not kick a pass", name)
		}
	}

	differ := map[string]func(*appv1alpha1.AppStatus){
		"a phase":            func(s *appv1alpha1.AppStatus) { s.Phase = appv1alpha1.PhaseFailed },
		"a condition status": func(s *appv1alpha1.AppStatus) { s.Conditions[0].Status = metav1.ConditionFalse },
		"a condition reason": func(s *appv1alpha1.AppStatus) { s.Conditions[0].Reason = "BuildFailed" },
		"a build record": func(s *appv1alpha1.AppStatus) {
			s.BuildRun = &appv1alpha1.BuildRunStatus{Generation: 4, StartedAt: "2026-11-13T01:48:30Z"}
		},
		"a release start": func(s *appv1alpha1.AppStatus) {
			s.ReleaseStart = &appv1alpha1.ReleaseStart{Generation: 4, StartedAt: "2026-11-13T01:48:30Z"}
		},
		"an instance exit": func(s *appv1alpha1.AppStatus) {
			s.InstanceExits = []appv1alpha1.InstanceExit{{Instance: "web-1", ExitCode: 3}}
		},
		"a cron run":         func(s *appv1alpha1.AppStatus) { s.Runs = []appv1alpha1.CronRun{{Name: "run-1", Status: "running"}} },
		"an active revision": func(s *appv1alpha1.AppStatus) { s.ActiveRevision = "rev-2" },
	}
	for name, change := range differ {
		s := base()
		change(&s)
		if got := statusFingerprint(&s); got == want {
			t.Errorf("%s left the fingerprint unchanged; the projector records it, so it must kick a pass", name)
		}
	}
}

func passCount(t *testing.T, m *ReconcilerMetrics) uint64 {
	t.Helper()
	var d dto.Metric
	if err := m.passSeconds.Write(&d); err != nil {
		t.Fatal(err)
	}
	return d.GetHistogram().GetSampleCount()
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// scriptedWatch serves one fake watch per Watch call and records the resource
// version each call asked for.
type scriptedWatch struct {
	client.WithWatch
	mu       sync.Mutex
	watchers []*watch.FakeWatcher
	asked    []string
	calls    chan int
}

func (s *scriptedWatch) Watch(_ context.Context, _ client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
	o := &client.ListOptions{}
	o.ApplyOptions(opts)
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.asked)
	s.asked = append(s.asked, o.Raw.ResourceVersion)
	s.calls <- n
	return s.watchers[n], nil
}

// TestStatusWatchRestartsFreshAfterAnExpiredVersionAndKicksForWhatItMissed: a
// watch whose resource version expired (410) starts over with a fresh listing,
// and an App whose status changed while it was down kicks, because the
// fingerprint it remembers no longer matches.
func TestStatusWatchRestartsFreshAfterAnExpiredVersionAndKicksForWhatItMissed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", UID: "u1", ResourceVersion: "10"}}
	app.Status.Phase = appv1alpha1.PhaseBuilding

	first, second := watch.NewFakeWithChanSize(4, false), watch.NewFakeWithChanSize(4, false)
	sw := &scriptedWatch{watchers: []*watch.FakeWatcher{first, second}, calls: make(chan int, 4)}
	var kicks atomic.Int32
	w := &AppStatusWatch{Client: sw, Kick: func() { kicks.Add(1) }, Backoff: time.Millisecond}
	go w.Run(ctx)

	<-sw.calls
	first.Add(app.DeepCopy())
	first.Error(&metav1.Status{Status: metav1.StatusFailure, Code: http.StatusGone, Reason: metav1.StatusReasonExpired})

	select {
	case <-sw.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch did not restart after an expired resource version")
	}
	changed := app.DeepCopy()
	changed.ResourceVersion = "20"
	changed.Status.Phase = appv1alpha1.PhaseFailed
	second.Add(changed)

	waitFor(t, "a kick", func() bool { return kicks.Load() > 0 })
	if kicks.Load() != 1 {
		t.Fatalf("kicks = %d, want 1: the App changed while the watch was down", kicks.Load())
	}
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.asked[0] != "" || sw.asked[1] != "" {
		t.Fatalf("resource versions asked = %q, want a fresh listing after the 410", sw.asked)
	}
}

// TestWatchKicksAreSpacedByThePassTheyCause: a watch-asked pass waits until
// max(1 s, 3 × the last pass) after the last pass ended, and changes seen while
// one is pending share it, so a busy fleet cannot run passes back to back.
func TestWatchKicksAreSpacedByThePassTheyCause(t *testing.T) {
	var s watchKickSpacing
	now := time.Now()
	s.passEnded(now, 2*time.Second)

	delay, ok := s.schedule(now)
	if !ok || delay < 6*time.Second || delay >= 6*time.Second+watchKickJitter {
		t.Fatalf("delay = %v (scheduled %v), want 6 s after a 2 s pass, plus under %v of jitter", delay, ok, watchKickJitter)
	}
	if _, again := s.schedule(now); again {
		t.Fatal("a second change scheduled its own kick while one was pending")
	}
	s.fired()

	s.passEnded(now, 10*time.Millisecond)
	delay, ok = s.schedule(now.Add(5 * time.Second))
	if !ok || delay >= watchKickJitter {
		t.Fatalf("delay = %v, want only jitter once a short pass is more than 1 s behind", delay)
	}
	s.fired()
	if delay, _ = s.schedule(now); delay < time.Second {
		t.Fatalf("delay = %v right after a short pass, want at least the 1 s floor", delay)
	}
}
