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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/592: resuming a suspended cron job runs no slot that passed while it was
// suspended. The CronJob controller starts the most recent slot after its last
// schedule time once suspend clears, so the resume moves that time to the
// resume itself. The pause behind a manual run is no user suspension: its
// delayed slot still runs, as Render delays a slot behind an active run.
func TestResumingACronJobSkipsTheSlotsItMissed(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "nightly", Namespace: "tea-x", UID: "cron-resume"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeCronJob, Schedule: "*/2 * * * *", Suspended: true},
	}
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}, &batchv1.CronJob{}).WithObjects(app).Build()
	r := wakeReconciler(cl, scheme)
	template := corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelApp: app.Name}},
		Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "app", Image: "busybox:latest"}}}}
	cronJob := func() *batchv1.CronJob {
		t.Helper()
		cj := &batchv1.CronJob{}
		if err := cl.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.CronJobName(app.Name)}, cj); err != nil {
			t.Fatal(err)
		}
		return cj
	}
	converge := func() {
		t.Helper()
		if _, err := r.convergeCronRuntime(t.Context(), app, template); err != nil {
			t.Fatal(err)
		}
	}

	converge()
	if cj := cronJob(); !*cj.Spec.Suspend || cj.Annotations[annotationCronUserSuspended] != "true" {
		t.Fatalf("suspended: suspend %v annotations %v, want suspended and marked the owner's", *cj.Spec.Suspend, cj.Annotations)
	}

	before := time.Now().Add(-time.Second)
	app.Spec.Suspended = false
	converge()
	cj := cronJob()
	if *cj.Spec.Suspend || cj.Annotations[annotationCronUserSuspended] != "" {
		t.Fatalf("resumed: suspend %v annotations %v, want running and unmarked", *cj.Spec.Suspend, cj.Annotations)
	}
	if last := cj.Status.LastScheduleTime; last == nil || last.Time.Before(before) {
		t.Fatalf("resumed: last schedule time %v, want the resume, so no missed slot runs", last)
	}

	// A pause behind a manual run carries no mark: lifting it leaves the
	// last schedule time alone, so the slot it delayed still runs.
	cj.Spec.Suspend = new(true)
	if err := cl.Update(t.Context(), cj); err != nil {
		t.Fatal(err)
	}
	cj.Status.LastScheduleTime = nil // the write above returned the stored status
	if err := cl.Status().Update(t.Context(), cj); err != nil {
		t.Fatal(err)
	}
	converge()
	if cj := cronJob(); *cj.Spec.Suspend || cj.Status.LastScheduleTime != nil {
		t.Fatalf("after a manual-run pause: suspend %v last schedule %v, want running with the delayed slot kept", *cj.Spec.Suspend, cj.Status.LastScheduleTime)
	}
}

// w4/592 review: a resume while a manual run is still running moves the last
// schedule time too. The run keeps the schedule paused, so the CronJob stays
// suspended, but its mark is gone afterwards; without the move the run's end
// would start the newest slot missed during the owner's suspension.
func TestResumingACronJobDuringAManualRunSkipsTheSlotsItMissed(t *testing.T) {
	const runAt = "2026-11-11T10:00:00Z"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "nightly", Namespace: "tea-x", UID: "cron-resume-run"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeCronJob, Schedule: "*/2 * * * *", Suspended: true, RunAt: runAt},
	}
	run := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: manualRunJobName(app.Name, runAt), Namespace: app.Namespace, Labels: map[string]string{labelApp: app.Name}}}
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}, &batchv1.CronJob{}).WithObjects(app, run).Build()
	r := wakeReconciler(cl, scheme)
	template := corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelApp: app.Name}},
		Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "app", Image: "busybox:latest"}}}}
	if _, err := r.convergeCronRuntime(t.Context(), app, template); err != nil {
		t.Fatal(err)
	}

	before := time.Now().Add(-time.Second)
	app.Spec.Suspended = false
	if _, err := r.convergeCronRuntime(t.Context(), app, template); err != nil {
		t.Fatal(err)
	}
	cj := &batchv1.CronJob{}
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.CronJobName(app.Name)}, cj); err != nil {
		t.Fatal(err)
	}
	if !*cj.Spec.Suspend || cj.Annotations[annotationCronUserSuspended] != "" {
		t.Fatalf("resumed during a run: suspend %v annotations %v, want paused behind the run and unmarked", *cj.Spec.Suspend, cj.Annotations)
	}
	if last := cj.Status.LastScheduleTime; last == nil || last.Time.Before(before) {
		t.Fatalf("resumed during a run: last schedule time %v, want the resume", last)
	}
}
