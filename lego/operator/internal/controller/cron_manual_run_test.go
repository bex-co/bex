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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// cron_manual_run_test.go replays w4/m114 t001: a user-canceled manual cron
// run came back to life and executed.
//
// Production, 2026-09-17, cron `qa-20260917-p12-cron`
// (`srv-dam3habs0ils73bgp0v0`, schedule `* * * * *`):
//
//	19:12:1x  runCronJob            -> manual run `…-run-2207ebf1` pending
//	19:14:20  cancelCronJobRun(manual)    -> canceled, its Job deleted
//	19:18:00  cancelCronJobRun(scheduled) -> overwrites spec.cancelRun
//	19:21:05  resume                -> the CANCELED manual run returns to
//	                                   pending and runs two fresh attempts
//
// The guard that should have stopped it consulted only spec.cancelRun — a
// single slot the backend reuses for every cancel — so the unrelated cancel
// four minutes later disarmed it while spec.runAt still stood.

const (
	incidentApp   = "tea-x-qa-20260917-p12-cron"
	incidentRunAt = "2026-09-17T19:12:14Z"
)

// incidentApp builds the cron App at a point in the timeline.
func cronAppAt(runs []appv1alpha1.CronRun, cancel *appv1alpha1.CronRunCancellation) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: incidentApp, Namespace: "tea-x"},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeCronJob, Schedule: "* * * * *",
			RunAt: incidentRunAt, CancelRun: cancel,
		},
		Status: appv1alpha1.AppStatus{Runs: runs},
	}
}

func TestCanceledManualRunStaysDeadAfterAnUnrelatedCancel(t *testing.T) {
	manual := manualRunJobName(incidentApp, incidentRunAt)
	const scheduled = "tea-x-qa-20260917-p12-cron-29827874"

	// 19:12 — triggered, running. Nothing is settled: the Job must be created
	// (and, having been created, must be left alone on re-reconcile).
	running := cronAppAt(
		[]appv1alpha1.CronRun{{Name: manual, Status: appv1alpha1.CronRunRunning, StartedAt: "2026-09-17T19:12:21Z"}},
		nil)
	if manualRunSettled(running) {
		t.Fatal("a running manual run must not be treated as settled — its Job would never be created")
	}

	// 19:14:20 — the user cancels it. The cancel slot names it; its Job is
	// deleted, and cronRuns retains the terminal entry in status.
	canceled := cronAppAt(
		[]appv1alpha1.CronRun{{
			Name: manual, Status: appv1alpha1.CronRunCanceled,
			StartedAt: "2026-09-17T19:12:21Z", FinishedAt: "2026-09-17T19:14:20Z",
		}},
		&appv1alpha1.CronRunCancellation{Name: manual, RequestedAt: "2026-09-17T19:14:20Z"})
	if !manualRunSettled(canceled) {
		t.Fatal("a just-canceled manual run must be settled")
	}

	// 19:18:00 — an UNRELATED cancel overwrites the single-slot intent. This
	// is the step that used to disarm the guard; status.runs still records the
	// manual run as canceled, which is what now keeps it dead.
	overwritten := cronAppAt(canceled.Status.Runs,
		&appv1alpha1.CronRunCancellation{Name: scheduled, RequestedAt: "2026-09-17T19:18:00Z"})
	if !manualRunSettled(overwritten) {
		t.Fatal("canceling an unrelated run resurrected the canceled manual run (w4/m114 t001)")
	}

	// 19:21:05 — resume. Same state, still settled: no recreation, no attempts.
	if !manualRunSettled(overwritten) {
		t.Fatal("resume recreated the canceled manual run")
	}

	// The control that proves the guard is not simply always-on: a NEW trigger
	// changes runAt, so the replacement run is created normally even while the
	// old cancel record is still in history.
	replacement := overwritten.DeepCopy()
	replacement.Spec.RunAt = "2026-09-17T19:25:00Z"
	if manualRunSettled(replacement) {
		t.Fatal("a fresh trigger must create its run — a new runAt is a different Job")
	}
}

func TestManualRunSettledCoversEveryTerminalStatus(t *testing.T) {
	manual := manualRunJobName(incidentApp, incidentRunAt)
	for _, tc := range []struct {
		status string
		want   bool
	}{
		{appv1alpha1.CronRunSucceeded, true},
		{appv1alpha1.CronRunFailed, true},
		{appv1alpha1.CronRunCanceled, true},
		// Still going: recreating is exactly what must happen if the Job is
		// somehow absent mid-run, and pausing the schedule is still correct.
		{appv1alpha1.CronRunRunning, false},
	} {
		t.Run(tc.status, func(t *testing.T) {
			app := cronAppAt([]appv1alpha1.CronRun{{Name: manual, Status: tc.status}}, nil)
			if got := manualRunSettled(app); got != tc.want {
				t.Errorf("manualRunSettled(%s) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// A cron with no manual run pending is never gated, and an App whose history
// knows nothing about the current runAt falls back to "not settled" so a fresh
// trigger on a service with a long run history still runs.
func TestManualRunSettledIsSilentWithoutAMatchingRecord(t *testing.T) {
	if manualRunSettled(cronAppAt(nil, nil)) {
		t.Error("no history and no cancel: nothing is settled")
	}
	noRunAt := cronAppAt(nil, nil)
	noRunAt.Spec.RunAt = ""
	if manualRunSettled(noRunAt) {
		t.Error("no runAt: there is no manual run to settle")
	}
	other := cronAppAt([]appv1alpha1.CronRun{
		{Name: incidentApp + "-29827872", Status: appv1alpha1.CronRunFailed},
	}, nil)
	if manualRunSettled(other) {
		t.Error("another run's terminal record must not gate this manual run")
	}
}

// The schedule-pause gate reads the same predicate, so a canceled manual run
// whose Job is gone cannot hold the recurring schedule paused forever.
func TestSettledManualRunDoesNotPauseTheSchedule(t *testing.T) {
	manual := manualRunJobName(incidentApp, incidentRunAt)
	app := cronAppAt(
		[]appv1alpha1.CronRun{{Name: manual, Status: appv1alpha1.CronRunCanceled}},
		&appv1alpha1.CronRunCancellation{Name: "tea-x-other-29827874", RequestedAt: "2026-09-17T19:18:00Z"})

	// manualCronRunActive short-circuits on manualRunSettled before it reads
	// the (deleted, NotFound) Job — the NotFound branch would otherwise report
	// "about to be created" and pause the schedule indefinitely.
	r := &AppReconciler{}
	active, err := r.manualCronRunActive(t.Context(), app, false)
	if err != nil {
		t.Fatalf("manualCronRunActive: %v", err)
	}
	if active {
		t.Fatal("a settled manual run must not pause the recurring schedule")
	}
}
