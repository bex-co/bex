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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	censusBuildNS      = "bex-build"
	overshootWorkspace = "tea-aaa"
)

func overshootApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", UID: types.UID("uid-" + name),
		Labels: map[string]string{labelWorkspace: overshootWorkspace},
	}}
}

func overshootJob(name string, age time.Duration) *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:              name,
		Namespace:         censusBuildNS,
		CreationTimestamp: metav1.NewTime(time.Now().Add(-age)),
		Labels: map[string]string{
			"app.bex.co/component": "build",
			"app.bex.co/workspace": overshootWorkspace,
		},
	}}
}

// TestShedBuildOvershootBacksOffTheRacerThatJustDispatched is the correction
// ADR060 D6 asks for in place of a distributed lock: the gate is list-then-create,
// so two reconciles can each see the same free slot. The one whose build is
// newest must be the one that yields — and it must learn that it yielded, or it
// would go on to observe a Job it no longer has.
func TestShedBuildOvershootBacksOffTheRacerThatJustDispatched(t *testing.T) {
	app := overshootApp("loser")
	mine := build.JobName(app.Name, releaseBuildRevision(app))
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(
		app,
		overshootJob("bld-incumbent", 5*time.Minute),
		overshootJob(mine, time.Second),
	).Build()
	r := &AppReconciler{Client: cl, MaxActiveBuilds: 1}

	shed, err := r.shedBuildOvershoot(context.Background(), app, censusBuildNS, cl)
	if err != nil {
		t.Fatalf("shedBuildOvershoot: %v", err)
	}
	if !shed {
		t.Fatal("shed = false; the App must learn its own build was shed, or it observes a deleted Job")
	}
	var gone batchv1.Job
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: censusBuildNS, Name: mine}, &gone); err == nil {
		t.Error("the just-created build survived the overshoot correction")
	}
	var incumbent batchv1.Job
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: censusBuildNS, Name: "bld-incumbent"}, &incumbent); err != nil {
		t.Errorf("the longer-running build was shed instead of the newest: %v", err)
	}
}

// TestShedBuildOvershootKeepsTheWinnersBuild: when someone else dispatched more
// recently, this App keeps its build and reports nothing shed — the correction
// must be silent for the racer that won.
func TestShedBuildOvershootKeepsTheWinnersBuild(t *testing.T) {
	app := overshootApp("winner")
	mine := build.JobName(app.Name, releaseBuildRevision(app))
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(
		app,
		overshootJob(mine, 5*time.Minute),
		overshootJob("bld-latecomer", time.Second),
	).Build()
	r := &AppReconciler{Client: cl, MaxActiveBuilds: 1}

	shed, err := r.shedBuildOvershoot(context.Background(), app, censusBuildNS, cl)
	if err != nil {
		t.Fatalf("shedBuildOvershoot: %v", err)
	}
	if shed {
		t.Error("shed = true; this App's build was not the newest and must survive")
	}
	var kept batchv1.Job
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: censusBuildNS, Name: mine}, &kept); err != nil {
		t.Errorf("this App's build was deleted despite being the incumbent: %v", err)
	}
}

// TestShedBuildOvershootIsInertWithNoCaps pins the byte-identical default: with
// neither cap configured the correction must not list, delete, or decide
// anything.
func TestShedBuildOvershootIsInertWithNoCaps(t *testing.T) {
	app := overshootApp("uncapped")
	mine := build.JobName(app.Name, releaseBuildRevision(app))
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(
		app,
		overshootJob(mine, time.Second),
		overshootJob("bld-other", 5*time.Minute),
	).Build()
	r := &AppReconciler{Client: cl} // both caps zero

	shed, err := r.shedBuildOvershoot(context.Background(), app, censusBuildNS, cl)
	if err != nil {
		t.Fatalf("shedBuildOvershoot: %v", err)
	}
	if shed {
		t.Error("shed = true with no caps configured")
	}
	var jobs batchv1.JobList
	if err := cl.List(context.Background(), &jobs, client.InNamespace(censusBuildNS)); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 2 {
		t.Errorf("%d build Jobs left, want 2 — an unset cap must delete nothing", len(jobs.Items))
	}
}

// TestBuildQueueSeriesRecordTheMeasuredValues asserts the metrics actually MOVE,
// not merely that they exist: the histogram must gain the observed wait, the
// retry counter must land on the classified label, and a build with nothing to
// report (kpack, which has no Job pods) must leave every series untouched rather
// than observing an unmeasured zero.
func TestBuildQueueSeriesRecordTheMeasuredValues(t *testing.T) {
	baseQueue := histogramSampleCount(t, "bex_build_queue_seconds")
	basePush := histogramSampleCount(t, "bex_build_push_seconds")
	baseDisruption := testutil.ToFloat64(buildRetriesTotal.WithLabelValues(build.RetryDisruption))
	basePushErrors := testutil.ToFloat64(buildPushErrorsTotal)

	recordBuildSignals(build.Signals{
		QueueSeconds: 240, QueueMeasured: true,
		PushSeconds: 12,
		PushFailed:  true,
		Retries:     []string{build.RetryDisruption, build.RetryDisruption},
	})

	if got := histogramSampleCount(t, "bex_build_queue_seconds"); got != baseQueue+1 {
		t.Errorf("queue observations = %d, want %d", got, baseQueue+1)
	}
	if got := histogramSampleCount(t, "bex_build_push_seconds"); got != basePush+1 {
		t.Errorf("push observations = %d, want %d", got, basePush+1)
	}
	if got := testutil.ToFloat64(buildRetriesTotal.WithLabelValues(build.RetryDisruption)); got != baseDisruption+2 {
		t.Errorf("disruption retries = %v, want %v", got, baseDisruption+2)
	}
	if got := testutil.ToFloat64(buildPushErrorsTotal); got != basePushErrors+1 {
		t.Errorf("push errors = %v, want %v", got, basePushErrors+1)
	}

	// A kpack build (no Job pods) must be invisible to every series.
	recordBuildSignals(build.Signals{})
	if got := histogramSampleCount(t, "bex_build_queue_seconds"); got != baseQueue+1 {
		t.Errorf("an unmeasured build was observed as 0s: count = %d, want %d", got, baseQueue+1)
	}

	// …but a build that genuinely waited under a second IS an observation. This is
	// the warm-node regime the low buckets exist to resolve, and metav1.Time's
	// second granularity reports it as exactly 0 — dropping it would leave the
	// histogram populated only by slow builds and inflate the p95 the capacity
	// alert pages on.
	recordBuildSignals(build.Signals{QueueMeasured: true})
	if got := histogramSampleCount(t, "bex_build_queue_seconds"); got != baseQueue+2 {
		t.Errorf("a sub-second (warm-node) queue wait was dropped: count = %d, want %d", got, baseQueue+2)
	}
	if got := testutil.ToFloat64(buildPushErrorsTotal); got != basePushErrors+1 {
		t.Errorf("an unmeasured build incremented the push error counter: %v", got)
	}
}

// TestBuildCensusGaugesRiseAndReturnToZero is the restart-safety property: the
// gauges are SET from a full recount, so republishing an idle census clears them
// instead of stranding a value that would page forever about a finished build.
func TestBuildCensusGaugesRiseAndReturnToZero(t *testing.T) {
	publishBuildCensus(build.Census{Active: 4, Queued: 2, OldestQueued: 9 * time.Minute})
	if got := testutil.ToFloat64(buildsActive); got != 4 {
		t.Errorf("bex_builds_active = %v, want 4", got)
	}
	if got := testutil.ToFloat64(buildsQueued); got != 2 {
		t.Errorf("bex_builds_queued = %v, want 2", got)
	}
	if got := testutil.ToFloat64(buildQueueOldestSeconds); got != 540 {
		t.Errorf("bex_build_queue_oldest_seconds = %v, want 540", got)
	}

	publishBuildCensus(build.Census{})
	for name, gauge := range map[string]prometheus.Gauge{
		"bex_builds_active":              buildsActive,
		"bex_builds_queued":              buildsQueued,
		"bex_build_queue_oldest_seconds": buildQueueOldestSeconds,
	} {
		if got := testutil.ToFloat64(gauge); got != 0 {
			t.Errorf("%s = %v after an idle recount, want 0", name, got)
		}
	}
}

// histogramSampleCount reads the observation count of a registered histogram off
// the controller-runtime registry — the same registry the manager serves — so
// the test proves the series is reachable where Prometheus will scrape it, not
// merely that a local variable moved.
func histogramSampleCount(t *testing.T, name string) uint64 {
	t.Helper()
	families, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		if len(f.GetMetric()) == 0 {
			return 0
		}
		return f.GetMetric()[0].GetHistogram().GetSampleCount()
	}
	t.Fatalf("%s is not registered on the manager's metrics registry", name)
	return 0
}
