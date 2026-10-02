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
	"reflect"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestCronRunsHaveTwelveHourDeadline(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	reconcileManualIntent(t, r, app, template)
	var cron batchv1.CronJob
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.CronJobName(app.Name)}, &cron); err != nil {
		t.Fatal(err)
	}
	var manual batchv1.Job
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: manualRunJobName(app.Name, app.Spec.RunAt)}, &manual); err != nil {
		t.Fatal(err)
	}
	for name, spec := range map[string]batchv1.JobSpec{"scheduled": cron.Spec.JobTemplate.Spec, "manual": manual.Spec} {
		if spec.ActiveDeadlineSeconds == nil || *spec.ActiveDeadlineSeconds != 43200 {
			t.Errorf("%s deadline = %v, want 43200 seconds (12 hours)", name, spec.ActiveDeadlineSeconds)
		}
		if spec.BackoffLimit == nil || *spec.BackoffLimit != 0 {
			t.Errorf("%s backoff = %v, want existing zero-retry policy", name, spec.BackoffLimit)
		}
	}
}

func TestExistingCronRunsReceiveDeadlineWithoutResettingExecution(t *testing.T) {
	cases := []struct {
		name     string
		owner    string
		deadline *int64
		want     *int64
		terminal batchv1.JobConditionType
		deleting bool
	}{
		{name: "scheduled missing", owner: "scheduled", want: new(int64(43200))},
		{name: "scheduled overlong", owner: "scheduled", deadline: new(int64(86400)), want: new(int64(43200))},
		{name: "legacy current manual", owner: "current manual", want: new(int64(43200))},
		{name: "marked older manual", owner: "older manual", deadline: new(int64(86400)), want: new(int64(43200))},
		{name: "shorter scheduled preserved", owner: "scheduled", deadline: new(int64(60)), want: new(int64(60))},
		{name: "shorter manual preserved", owner: "current manual", deadline: new(int64(90)), want: new(int64(90))},
		{name: "terminating toward failure", owner: "scheduled", terminal: batchv1.JobFailureTarget, want: new(int64(43200))},
		{name: "completed untouched", owner: "scheduled", terminal: batchv1.JobComplete},
		{name: "failed untouched", owner: "scheduled", terminal: batchv1.JobFailed, deadline: new(int64(86400)), want: new(int64(86400))},
		{name: "deleting untouched", owner: "scheduled", deleting: true},
		{name: "foreign CronJob UID untouched", owner: "foreign scheduled"},
		{name: "foreign App UID untouched", owner: "foreign manual"},
		{name: "unrelated App owned untouched", owner: "unrelated"},
		{name: "unidentified untouched", owner: "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, app, template := manualIntentFixture(t)
			// Suspending the service pauses future scheduling; existing executions
			// still consume their deadline and must receive the compatibility cap.
			app.Spec.Suspended = true
			persistManualIntent(t, r, app)
			cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.CronJobName(app.Name), Namespace: app.Namespace, UID: "deadline-schedule"}}
			if err := r.Create(t.Context(), cron); err != nil {
				t.Fatal(err)
			}
			start := metav1.NewTime(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC))
			job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: app.Name + "-scheduled", Namespace: app.Namespace, Labels: map[string]string{labelApp: app.Name}},
				Spec:   batchv1.JobSpec{ActiveDeadlineSeconds: tc.deadline, BackoffLimit: new(int32(0))},
				Status: batchv1.JobStatus{StartTime: &start, Active: 1}}
			setDeadlineFixtureOwner(app, cron, job, tc.owner)
			if tc.terminal != "" {
				job.Status.Conditions = []batchv1.JobCondition{{Type: tc.terminal, Status: corev1.ConditionTrue}}
			}
			if tc.deleting {
				job.Finalizers = []string{"test.bex.co/pods-running"}
			}
			if err := r.Create(t.Context(), job); err != nil {
				t.Fatal(err)
			}
			if tc.deleting {
				if err := r.Delete(t.Context(), job); err != nil {
					t.Fatal(err)
				}
			}
			reconcileManualIntent(t, r, app, template)
			var first batchv1.Job
			if err := r.Get(t.Context(), client.ObjectKeyFromObject(job), &first); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first.Spec.ActiveDeadlineSeconds, tc.want) {
				t.Errorf("deadline = %v, want %v", first.Spec.ActiveDeadlineSeconds, tc.want)
			}
			if !equality.Semantic.DeepEqual(first.Status, job.Status) {
				t.Errorf("deadline adoption changed execution status/start time: before=%+v after=%+v", job.Status, first.Status)
			}
			if first.Spec.BackoffLimit == nil || *first.Spec.BackoffLimit != 0 {
				t.Error("adoption changed existing retry policy")
			}
			reconcileManualIntent(t, r, app, template)
			var second batchv1.Job
			if err := r.Get(t.Context(), client.ObjectKeyFromObject(job), &second); err != nil {
				t.Fatal(err)
			}
			if second.ResourceVersion != first.ResourceVersion {
				t.Errorf("unchanged deadline caused recurring Job writes: %s -> %s", first.ResourceVersion, second.ResourceVersion)
			}
		})
	}
}

func setDeadlineFixtureOwner(app *appv1alpha1.App, cron *batchv1.CronJob, job *batchv1.Job, owner string) {
	ref := metav1.OwnerReference{APIVersion: "batch/v1", Kind: "CronJob", Name: cron.Name, UID: cron.UID, Controller: new(true)}
	switch owner {
	case "foreign scheduled":
		ref.UID = "foreign-schedule"
	case "current manual", "older manual", "foreign manual", "unrelated":
		ref.APIVersion, ref.Kind, ref.Name, ref.UID = appv1alpha1.SchemeGroupVersion.String(), "App", app.Name, app.UID
		runAt := app.Spec.RunAt
		if owner == "older manual" {
			runAt = "2026-09-17T18:00:00Z"
			job.Annotations = map[string]string{annotationManualRunAt: runAt}
		}
		job.Name = manualRunJobName(app.Name, runAt)
		if owner == "foreign manual" {
			ref.UID = "foreign-app"
		}
		if owner == "unrelated" {
			job.Name = app.Name + "-other-purpose"
		}
	case "none":
		return
	}
	job.OwnerReferences = []metav1.OwnerReference{ref}
}

func TestManualCronDeadlineFailureWaitsForPodsThenReleasesSchedule(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	reconcileManualIntent(t, r, app, template)
	var job batchv1.Job
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: manualRunJobName(app.Name, app.Spec.RunAt)}, &job); err != nil {
		t.Fatal(err)
	}
	start := metav1.NewTime(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC))
	deadline := metav1.NewTime(start.Add(12 * time.Hour))
	job.Status.StartTime = &start
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: "DeadlineExceeded", LastTransitionTime: deadline}}
	if err := r.Status().Update(t.Context(), &job); err != nil {
		t.Fatal(err)
	}
	reconcileManualIntent(t, r, app, template)
	assertManualScheduleSuspended(t, r, app, true)
	assertDeadlineRunState(t, app, job.Name, appv1alpha1.CronRunRunning, "")

	// Kubernetes publishes terminal Failed only once its Pods have terminated.
	finished := metav1.NewTime(deadline.Add(30 * time.Second))
	job.Status.Conditions = append(job.Status.Conditions, batchv1.JobCondition{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "DeadlineExceeded", LastTransitionTime: finished})
	if err := r.Status().Update(t.Context(), &job); err != nil {
		t.Fatal(err)
	}
	reconcileManualIntent(t, r, app, template)
	assertManualScheduleSuspended(t, r, app, false)
	assertDeadlineRunState(t, app, job.Name, appv1alpha1.CronRunFailed, finished.UTC().Format(time.RFC3339))
	if err := r.Delete(t.Context(), &job); err != nil {
		t.Fatal(err)
	}
	reconcileManualIntent(t, r, app, template)
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(&job), &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("timed-out manual run was recreated after GC: %v", err)
	}
	assertDeadlineRunState(t, app, job.Name, appv1alpha1.CronRunFailed, finished.UTC().Format(time.RFC3339))
	app.Spec.RunAt = "2026-10-01T13:00:00Z"
	persistManualIntent(t, r, app)
	reconcileManualIntent(t, r, app, template)
	var next batchv1.Job
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: manualRunJobName(app.Name, app.Spec.RunAt)}, &next); err != nil {
		t.Fatalf("later manual trigger did not execute: %v", err)
	}
	if next.Spec.ActiveDeadlineSeconds == nil || *next.Spec.ActiveDeadlineSeconds != 43200 {
		t.Fatal("later execution did not receive a fresh twelve-hour deadline")
	}
	assertManualScheduleSuspended(t, r, app, true)
}

func assertDeadlineRunState(t *testing.T, app *appv1alpha1.App, name, status, finishedAt string) {
	t.Helper()
	for _, run := range app.Status.Runs {
		if run.Name == name {
			if run.Status != status || run.FinishedAt != finishedAt {
				t.Fatalf("run projection = status %s finished %q, want %s %q", run.Status, run.FinishedAt, status, finishedAt)
			}
			return
		}
	}
	t.Fatalf("run %s missing from history", name)
}
