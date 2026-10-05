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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// failedCronJob is a failed run of app whose Pod (when withPod) terminated as
// given. It is labeled like a real run so cronRuns lists it.
func failedCronJob(t *testing.T, r *AppReconciler, app *appv1alpha1.App, name, jobReason string, pod *corev1.Pod) {
	t.Helper()
	start := metav1.NewTime(time.Date(2026, time.October, 5, 5, 51, 0, 0, time.UTC))
	failed := metav1.NewTime(start.Add(25 * time.Second))
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace, UID: types.UID(name + "-uid"), Labels: map[string]string{labelApp: app.Name}},
		Spec:       batchv1.JobSpec{BackoffLimit: new(int32(0))},
		Status: batchv1.JobStatus{StartTime: &start, Failed: 1, Conditions: []batchv1.JobCondition{{
			Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: jobReason, LastTransitionTime: failed,
		}}},
	}
	if err := r.Create(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	if pod != nil {
		pod.Name, pod.Namespace = name+"-pod", app.Namespace
		pod.Labels = map[string]string{batchv1.ControllerUidLabel: name + "-uid"}
		if err := r.Create(t.Context(), pod); err != nil {
			t.Fatal(err)
		}
	}
}

func terminatedPod(reason string, exit int32) *corev1.Pod {
	return &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
		Name:  appContainerName,
		State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: reason, ExitCode: exit}},
	}}}}
}

func runNamed(t *testing.T, app *appv1alpha1.App, name string) appv1alpha1.CronRun {
	t.Helper()
	for _, run := range app.Status.Runs {
		if run.Name == name {
			return run
		}
	}
	t.Fatalf("run %s missing from status %+v", name, app.Status.Runs)
	return appv1alpha1.CronRun{}
}

// w4/m114 live closeout 2026-10-05: 19 failing runs (`exit 3`) reached
// `unsuccessful` within seconds, but no surface said why. A failed run now
// records its cause from the Job and its Pod.
func TestFailedCronRunRecordsWhyItFailed(t *testing.T) {
	cases := []struct {
		name       string
		jobReason  string
		pod        *corev1.Pod
		wantReason string
		wantExit   *int32
	}{
		{name: "non-zero exit", jobReason: batchv1.JobReasonBackoffLimitExceeded, pod: terminatedPod("Error", 3),
			wantReason: appv1alpha1.CronRunReasonNonZeroExit, wantExit: new(int32(3))},
		{name: "out of memory", jobReason: batchv1.JobReasonBackoffLimitExceeded, pod: terminatedPod("OOMKilled", 137),
			wantReason: appv1alpha1.CronRunReasonOOMKilled, wantExit: new(int32(137))},
		{name: "evicted", jobReason: batchv1.JobReasonBackoffLimitExceeded, pod: &corev1.Pod{Status: corev1.PodStatus{Reason: "Evicted"}},
			wantReason: appv1alpha1.CronRunReasonEvicted},
		{name: "twelve-hour deadline", jobReason: batchv1.JobReasonDeadlineExceeded,
			wantReason: appv1alpha1.CronRunReasonDeadlineExceeded},
		{name: "pod already gone", jobReason: batchv1.JobReasonBackoffLimitExceeded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, app, template := manualIntentFixture(t)
			failedCronJob(t, r, app, app.Name+"-29313831", tc.jobReason, tc.pod)
			reconcileManualIntent(t, r, app, template)
			run := runNamed(t, app, app.Name+"-29313831")
			if run.Status != appv1alpha1.CronRunFailed || run.FailureReason != tc.wantReason {
				t.Fatalf("run = %q reason %q, want Failed with %q", run.Status, run.FailureReason, tc.wantReason)
			}
			if (run.ExitCode == nil) != (tc.wantExit == nil) || (run.ExitCode != nil && *run.ExitCode != *tc.wantExit) {
				t.Fatalf("exit code = %v, want %v", run.ExitCode, tc.wantExit)
			}
		})
	}
}

// Kubernetes garbage-collects a terminal Job's Pod; the cause observed while it
// existed must survive in status rather than vanish on the next pass.
func TestFailedCronRunKeepsItsReasonAfterThePodIsGone(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	name := app.Name + "-29313831"
	failedCronJob(t, r, app, name, batchv1.JobReasonBackoffLimitExceeded, terminatedPod("Error", 3))
	reconcileManualIntent(t, r, app, template)

	if err := r.Delete(t.Context(), &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name + "-pod", Namespace: app.Namespace}}); err != nil {
		t.Fatal(err)
	}
	reconcileManualIntent(t, r, app, template)
	run := runNamed(t, app, name)
	if run.FailureReason != appv1alpha1.CronRunReasonNonZeroExit || run.ExitCode == nil || *run.ExitCode != 3 {
		t.Fatalf("after the Pod is gone: reason %q exit %v, want NonZeroExit 3", run.FailureReason, run.ExitCode)
	}
}

// A run that succeeded, or one a user canceled, carries no failure reason even
// if its Pod exited non-zero on the way down.
func TestOnlyFailedCronRunsCarryAReason(t *testing.T) {
	if reason, exit := cronPodFailure(terminatedPod("Completed", 0)); reason != "" || exit != nil {
		t.Fatalf("exit 0 = (%q, %v), want no failure", reason, exit)
	}
	run := appv1alpha1.CronRun{Name: "x", Status: appv1alpha1.CronRunCanceled}
	r, _, _ := manualIntentFixture(t)
	r.observeCronRunFailure(t.Context(), &batchv1.Job{}, &run)
	if run.FailureReason != "" || run.ExitCode != nil {
		t.Fatalf("canceled run = %+v, want no failure reason", run)
	}
}
