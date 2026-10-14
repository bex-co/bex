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
	"fmt"
	"slices"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m210 — production 2026-10-14 (a475): one successful manual run, then ten
// failed ones. After run11 the history led with the 17:07 success and dropped
// the second-oldest run instead: a completed Job whose record had been
// evicted was mistaken for a new execution.

var historyT0 = time.Date(2026, time.October, 14, 17, 7, 3, 0, time.UTC)

// settledCronJob creates a terminal run Job of app that started at start.
func settledCronJob(t *testing.T, r *AppReconciler, app *appv1alpha1.App, name string, start time.Time, ok bool) {
	t.Helper()
	started := metav1.NewTime(start)
	cond := batchv1.JobCondition{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(start.Add(5 * time.Second))}
	if ok {
		cond.Type = batchv1.JobComplete
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace, Labels: map[string]string{labelApp: app.Name}},
		Status:     batchv1.JobStatus{StartTime: &started, Conditions: []batchv1.JobCondition{cond}},
	}
	if err := r.Create(t.Context(), job); err != nil {
		t.Fatal(err)
	}
}

// observeHistory runs one history pass and stores it, as a reconcile does.
func observeHistory(t *testing.T, r *AppReconciler, app *appv1alpha1.App) []string {
	t.Helper()
	runs, err := r.cronRuns(t.Context(), app, &batchv1.CronJob{})
	if err != nil {
		t.Fatal(err)
	}
	app.Status.Runs = runs
	names := make([]string, len(runs))
	for i, run := range runs {
		names[i] = run.Name
	}
	return names
}

func runName(app *appv1alpha1.App, n int) string { return fmt.Sprintf("%s-run%02d", app.Name, n) }

// want is runs hi..lo of app, newest first.
func wantRuns(app *appv1alpha1.App, hi, lo int) []string {
	var names []string
	for n := hi; n >= lo; n-- {
		names = append(names, runName(app, n))
	}
	return names
}

func TestCappedCronHistoryKeepsTheNewestTenInOrder(t *testing.T) {
	r, app, _ := manualIntentFixture(t)
	settledCronJob(t, r, app, runName(app, 1), historyT0, true)
	observeHistory(t, r, app)
	for n := 2; n <= 11; n++ {
		settledCronJob(t, r, app, runName(app, n), historyT0.Add(time.Duration(n)*36*time.Second), false)
		observeHistory(t, r, app)
	}
	if got, want := observeHistory(t, r, app), wantRuns(app, 11, 2); !slices.Equal(got, want) {
		t.Fatalf("after run11:\n got %v\nwant %v (the old success must fall outside the ten)", got, want)
	}
	for range 5 {
		if got, want := observeHistory(t, r, app), wantRuns(app, 11, 2); !slices.Equal(got, want) {
			t.Fatalf("unchanged re-read reordered history: %v", got)
		}
	}

	// Recovery: a new success leads and displaces only the oldest retained run.
	settledCronJob(t, r, app, runName(app, 12), historyT0.Add(11*time.Minute), true)
	if got, want := observeHistory(t, r, app), wantRuns(app, 12, 3); !slices.Equal(got, want) {
		t.Fatalf("after run12:\n got %v\nwant %v", got, want)
	}
	if app.Status.Runs[0].Status != appv1alpha1.CronRunSucceeded || app.Status.Runs[1].Status != appv1alpha1.CronRunFailed {
		t.Fatalf("statuses changed: %+v", app.Status.Runs[:2])
	}
}

// A run canceled before it started has no StartedAt, and cancellation
// deletes its Job — its persisted OrderedAt keeps it in place afterwards.
func TestCanceledPreStartRunKeepsItsPlaceAfterItsJobIsDeleted(t *testing.T) {
	r, app, _ := manualIntentFixture(t)
	settledCronJob(t, r, app, runName(app, 1), historyT0, true)
	pending := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: runName(app, 2), Namespace: app.Namespace, Labels: map[string]string{labelApp: app.Name},
		CreationTimestamp: metav1.NewTime(historyT0.Add(time.Minute)),
	}}
	if err := r.Create(t.Context(), pending); err != nil {
		t.Fatal(err)
	}
	app.Spec.CancelRun = &appv1alpha1.CronRunCancellation{Name: pending.Name, RequestedAt: "2026-10-14T17:08:30Z"}
	observeHistory(t, r, app)
	if got := runNamed(t, app, pending.Name); got.Status != appv1alpha1.CronRunCanceled || got.StartedAt != "" || got.OrderedAt == "" {
		t.Fatalf("canceled pre-start run = %+v, want Canceled with an order clock and no start", got)
	}
	if err := r.Delete(t.Context(), pending); err != nil {
		t.Fatal(err)
	}
	settledCronJob(t, r, app, runName(app, 3), historyT0.Add(2*time.Minute), false)
	if got, want := observeHistory(t, r, app), wantRuns(app, 3, 1); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// Ten newer runs push the tombstone (and the success) out, oldest first.
	for n := 4; n <= 12; n++ {
		settledCronJob(t, r, app, runName(app, n), historyT0.Add(time.Duration(n)*time.Minute), false)
		observeHistory(t, r, app)
	}
	if got, want := observeHistory(t, r, app), wantRuns(app, 12, 3); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// A record written before OrderedAt existed, that never started and whose Job
// is gone, has no clock to recover: it keeps its prior neighbors instead of
// sinking to the tail or being given an invented start.
func TestLegacyClocklessRecordKeepsItsPriorPosition(t *testing.T) {
	r, app, _ := manualIntentFixture(t)
	app.Spec.RunAt = ""
	app.Status.Runs = []appv1alpha1.CronRun{
		{Name: runName(app, 3), Status: appv1alpha1.CronRunFailed, StartedAt: "2026-10-14T17:10:00Z"},
		{Name: runName(app, 2), Status: appv1alpha1.CronRunCanceled},
		{Name: runName(app, 1), Status: appv1alpha1.CronRunSucceeded, StartedAt: "2026-10-14T17:07:03Z"},
	}
	settledCronJob(t, r, app, runName(app, 4), historyT0.Add(10*time.Minute), false)
	if got, want := observeHistory(t, r, app), wantRuns(app, 4, 1); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if legacy := runNamed(t, app, runName(app, 2)); legacy.StartedAt != "" || legacy.OrderedAt != "" {
		t.Fatalf("legacy record gained an invented clock: %+v", legacy)
	}
}

// Runs that share a clock (Job timestamps are whole seconds) order the same
// way on every pass, keeping the order the status already showed.
func TestTiedCronRunsOrderDeterministically(t *testing.T) {
	r, app, _ := manualIntentFixture(t)
	app.Spec.RunAt = ""
	for n := 1; n <= 3; n++ {
		settledCronJob(t, r, app, runName(app, n), historyT0, n%2 == 0)
	}
	first := observeHistory(t, r, app)
	if want := wantRuns(app, 3, 1); !slices.Equal(first, want) {
		t.Fatalf("tied runs = %v, want name order %v", first, want)
	}
	app.Status.Runs[0], app.Status.Runs[1] = app.Status.Runs[1], app.Status.Runs[0]
	shown := []string{app.Status.Runs[0].Name, app.Status.Runs[1].Name, app.Status.Runs[2].Name}
	for range 3 {
		if got := observeHistory(t, r, app); !slices.Equal(got, shown) {
			t.Fatalf("tie reshuffled: got %v, want the shown order %v", got, shown)
		}
	}
}
