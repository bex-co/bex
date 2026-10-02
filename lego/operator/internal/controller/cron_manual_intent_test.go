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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func manualIntentFixture(t *testing.T) (*AppReconciler, *appv1alpha1.App, corev1.PodTemplateSpec) {
	t.Helper()
	app := cronAppAt(nil, nil)
	return manualIntentFixtureForApp(t, app)
}

func manualIntentFixtureForApp(t *testing.T, app *appv1alpha1.App) (*AppReconciler, *appv1alpha1.App, corev1.PodTemplateSpec) {
	t.Helper()
	app.UID = "manual-intent-app"
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).WithObjects(app).Build()
	template := corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelApp: app.Name}}, Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "app", Image: "busybox:latest"}}}}
	return wakeReconciler(cl, scheme), app, template
}

func reconcileManualIntent(t *testing.T, r *AppReconciler, app *appv1alpha1.App, template corev1.PodTemplateSpec) {
	t.Helper()
	if _, err := r.convergeCronRuntime(t.Context(), app, template); err != nil {
		t.Fatal(err)
	}
	if err := updateStatusIfChanged(t.Context(), r.Client, app); err != nil {
		t.Fatal(err)
	}
}

func persistManualIntent(t *testing.T, r *AppReconciler, app *appv1alpha1.App) {
	t.Helper()
	if err := r.Update(t.Context(), app); err != nil {
		t.Fatal(err)
	}
}

func assertManualJobAbsent(t *testing.T, r *AppReconciler, app *appv1alpha1.App, runAt string) {
	t.Helper()
	key := client.ObjectKey{Namespace: app.Namespace, Name: manualRunJobName(app.Name, runAt)}
	if err := r.Get(t.Context(), key, &batchv1.Job{}); !apierrors.IsNotFound(err) {
		t.Fatalf("canceled manual intent was recreated after history eviction: job=%s get=%v", key.Name, err)
	}
}

func TestCanceledManualIntentStaysHandledAfterHistoryEviction(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	reconcileManualIntent(t, r, app, template)
	original := app.Spec.RunAt
	name := manualRunJobName(app.Name, original)
	app.Spec.CancelRun = &appv1alpha1.CronRunCancellation{Name: name, RequestedAt: "2026-09-17T19:14:20Z"}
	persistManualIntent(t, r, app)
	reconcileManualIntent(t, r, app, template)
	reconcileManualIntent(t, r, app, template)
	assertManualJobAbsent(t, r, app, original)
	if app.Status.ManualRunHandledAt != original {
		t.Fatal("history eviction lost durable acknowledgement")
	}
	assertManualScheduleSuspended(t, r, app, false)

	// Model eleven subsequent completed scheduler executions. The history builder
	// sees their actual Job objects and evicts the original canceled run itself.
	for i := range maxCronRuns + 1 {
		started := metav1.NewTime(time.Date(2026, time.September, 17, 20, i, 0, 0, time.UTC))
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-later-%02d", app.Name, i), Namespace: app.Namespace, Labels: map[string]string{labelApp: app.Name}},
			Status: batchv1.JobStatus{StartTime: &started, Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, LastTransitionTime: started}}}}
		if err := r.Create(t.Context(), job); err != nil {
			t.Fatal(err)
		}
	}
	reconcileManualIntent(t, r, app, template)
	for _, run := range app.Status.Runs {
		if run.Name == name {
			t.Fatal("reproduction did not evict original history")
		}
	}
	t.Logf("canceled manual Job absent; history now contains %d newer runs and RunAt remains %s", len(app.Status.Runs), app.Spec.RunAt)
	app.Spec.CancelRun = &appv1alpha1.CronRunCancellation{Name: app.Name + "-later-cancellation", RequestedAt: "2026-09-17T21:00:00Z"}
	app.Spec.Suspended = true
	persistManualIntent(t, r, app)
	reconcileManualIntent(t, r, app, template)
	app.Spec.Suspended = false
	persistManualIntent(t, r, app)
	// A fresh reconciler and API read prevent an in-memory flag from hiding loss.
	r = &AppReconciler{Client: r.Client, Scheme: r.Scheme, Mode: ModeKubernetes}
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(app), app); err != nil {
		t.Fatal(err)
	}
	reconcileManualIntent(t, r, app, template)
	assertManualJobAbsent(t, r, app, original)

	app.Spec.RunAt = "2026-09-17T21:05:00Z"
	persistManualIntent(t, r, app)
	reconcileManualIntent(t, r, app, template)
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: manualRunJobName(app.Name, app.Spec.RunAt)}, &batchv1.Job{}); err != nil {
		t.Fatalf("fresh manual trigger did not execute: %v", err)
	}
	assertManualScheduleSuspended(t, r, app, true)
}

func assertManualScheduleSuspended(t *testing.T, r *AppReconciler, app *appv1alpha1.App, suspended bool) {
	t.Helper()
	var cron batchv1.CronJob
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.CronJobName(app.Name)}, &cron); err != nil {
		t.Fatal(err)
	}
	if cron.Spec.Suspend == nil || *cron.Spec.Suspend != suspended {
		t.Fatalf("schedule suspension = %v, want %v", cron.Spec.Suspend, suspended)
	}
}

func TestLegacyManualIntentAdoptsExistingHistory(t *testing.T) {
	for _, status := range []string{appv1alpha1.CronRunRunning, appv1alpha1.CronRunSucceeded, appv1alpha1.CronRunFailed, appv1alpha1.CronRunCanceled} {
		t.Run(status, func(t *testing.T) {
			r, app, template := manualIntentFixture(t)
			name := manualRunJobName(app.Name, app.Spec.RunAt)
			app.Status.Runs = []appv1alpha1.CronRun{{Name: name, Status: status}}
			if err := r.Status().Update(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			reconcileManualIntent(t, r, app, template)
			assertManualJobAbsent(t, r, app, app.Spec.RunAt)
			assertManualScheduleSuspended(t, r, app, false)
			var stored appv1alpha1.App
			if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), &stored); err != nil {
				t.Fatal(err)
			}
			if stored.Status.ManualRunHandledAt != app.Spec.RunAt {
				t.Fatal("legacy materialization evidence was not acknowledged")
			}
			// A missing previously running Job has an unknown outcome. Handling
			// must never fabricate successful, failed, or canceled run history.
			if status == appv1alpha1.CronRunRunning && len(stored.Status.Runs) != 0 {
				t.Fatalf("fabricated history for missing running Job: %+v", stored.Status.Runs)
			}
		})
	}
}

func TestLegacyManualIntentWithoutEvidenceRemainsPending(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	// A very old RunAt and fully observed generation cannot distinguish lost
	// historical evidence from a valid trigger queued while suspended.
	app.Spec.Suspended = true
	app.Status.ObservedGeneration = app.Generation
	persistManualIntent(t, r, app)
	reconcileManualIntent(t, r, app, template)
	if app.Status.ManualRunHandledAt != "" {
		t.Fatal("unproven legacy intent was silently acknowledged")
	}
	assertManualJobAbsent(t, r, app, app.Spec.RunAt)
	app.Spec.Suspended = false
	persistManualIntent(t, r, app)
	reconcileManualIntent(t, r, app, template)
	if app.Status.ManualRunHandledAt != app.Spec.RunAt {
		t.Fatal("fresh pending intent did not execute on resume")
	}
	assertManualScheduleSuspended(t, r, app, true)
}

func TestLegacyTerminalManualJobAcknowledgedBeforeHistoryTruncation(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	// The old manual Job has never appeared in status, and is older than the
	// entire retained history. Its terminal evidence still must be acknowledged.
	for i := range maxCronRuns + 2 {
		name := manualRunJobName(app.Name, app.Spec.RunAt)
		if i > 0 {
			name = fmt.Sprintf("%s-newer-%d", app.Name, i)
		}
		started := metav1.NewTime(time.Date(2026, time.October, 1, 1, i, 0, 0, time.UTC))
		job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace, Labels: map[string]string{labelApp: app.Name}},
			Status: batchv1.JobStatus{StartTime: &started, Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}}}
		if err := r.Create(t.Context(), job); err != nil {
			t.Fatal(err)
		}
	}
	reconcileManualIntent(t, r, app, template)
	for _, run := range app.Status.Runs {
		if run.Name == manualRunJobName(app.Name, app.Spec.RunAt) {
			t.Fatal("fixture did not exclude old terminal Job from bounded history")
		}
	}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: manualRunJobName(app.Name, app.Spec.RunAt), Namespace: app.Namespace}}
	if err := r.Delete(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	reconcileManualIntent(t, r, app, template)
	assertManualJobAbsent(t, r, app, app.Spec.RunAt)
	assertManualScheduleSuspended(t, r, app, false)
}

func TestManualIntentAcknowledgementKeepsExistingJobActive(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	reconcileManualIntent(t, r, app, template)
	for range 2 {
		reconcileManualIntent(t, r, app, template)
		assertManualScheduleSuspended(t, r, app, true)
	}
	var job batchv1.Job
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: manualRunJobName(app.Name, app.Spec.RunAt)}, &job); err != nil {
		t.Fatal(err)
	}
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := r.Status().Update(t.Context(), &job); err != nil {
		t.Fatal(err)
	}
	reconcileManualIntent(t, r, app, template)
	assertManualScheduleSuspended(t, r, app, false)
}

func TestManualIntentCancellationPersistsBeforeDeletion(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	name := manualRunJobName(app.Name, app.Spec.RunAt)
	app.Spec.CancelRun = &appv1alpha1.CronRunCancellation{Name: name}
	persistManualIntent(t, r, app)
	if err := r.Create(t.Context(), &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace, Labels: map[string]string{labelApp: app.Name}}}); err != nil {
		t.Fatal(err)
	}
	base := r.Client
	rejectAcknowledgement := true
	deletions := 0
	r.Client = interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		SubResourcePatch: func(ctx context.Context, cl client.Client, subresource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if rejectAcknowledgement {
				return errors.New("status temporarily unavailable")
			}
			return cl.SubResource(subresource).Patch(ctx, obj, patch, opts...)
		},
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			var stored appv1alpha1.App
			if err := base.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
				return err
			}
			if stored.Status.ManualRunHandledAt != app.Spec.RunAt {
				t.Error("deleting the only evidence before acknowledgement persisted")
			}
			deletions++
			return cl.Delete(ctx, obj, opts...)
		},
	})
	if _, err := r.convergeCronRuntime(t.Context(), app, template); err == nil {
		t.Fatal("failed acknowledgement unexpectedly succeeded")
	}
	if deletions != 0 || app.Status.ManualRunHandledAt != "" {
		t.Fatal("failed persistence deleted Job or retained an in-memory acknowledgement")
	}
	rejectAcknowledgement = false
	reconcileManualIntent(t, r, app, template)
	if deletions != 1 {
		t.Fatalf("cancellation retries = %d deletions, want 1", deletions)
	}
}

func TestManualIntentCrashAfterCreateAdoptsJobWithoutReexecution(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	base := r.Client
	rejectAcknowledgement := true
	creates := 0
	r.Client = interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*batchv1.Job); ok {
				creates++
			}
			return cl.Create(ctx, obj, opts...)
		},
		SubResourcePatch: func(ctx context.Context, cl client.Client, subresource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if rejectAcknowledgement {
				return errors.New("crash after Job creation")
			}
			return cl.SubResource(subresource).Patch(ctx, obj, patch, opts...)
		},
	})
	if _, err := r.convergeCronRuntime(t.Context(), app, template); err == nil {
		t.Fatal("fixture did not interrupt acknowledgement")
	}
	if app.Status.ManualRunHandledAt != "" || creates != 1 {
		t.Fatalf("crash window not reproduced: marker=%q creates=%d", app.Status.ManualRunHandledAt, creates)
	}
	rejectAcknowledgement = false
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), app); err != nil {
		t.Fatal(err)
	}
	reconcileManualIntent(t, r, app, template)
	if app.Status.ManualRunHandledAt != app.Spec.RunAt || creates != 1 {
		t.Fatalf("retry did not adopt existing Job once: marker=%q creates=%d", app.Status.ManualRunHandledAt, creates)
	}
	assertManualScheduleSuspended(t, r, app, true)
}

func TestManualIntentConflictCannotOverwriteNewerAcknowledgement(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	app.Spec.CancelRun = &appv1alpha1.CronRunCancellation{Name: manualRunJobName(app.Name, app.Spec.RunAt)}
	persistManualIntent(t, r, app)
	stale := app.DeepCopy()
	app.Spec.RunAt = "2026-10-01T23:00:00Z"
	persistManualIntent(t, r, app)
	reconcileManualIntent(t, r, app, template)
	newToken := app.Status.ManualRunHandledAt
	_, err := r.convergeCronRuntime(t.Context(), stale, template)
	if !apierrors.IsConflict(err) {
		t.Fatalf("stale acknowledgement should conflict: %v", err)
	}
	if _, err := r.failStep(t.Context(), stale, err); !apierrors.IsConflict(err) {
		t.Fatalf("conflict should requeue from fresh state: %v", err)
	}
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), app); err != nil {
		t.Fatal(err)
	}
	if app.Status.ManualRunHandledAt != newToken || newToken != app.Spec.RunAt {
		t.Fatalf("older reconcile replaced newer acknowledgement: %q, want %q", app.Status.ManualRunHandledAt, newToken)
	}
	if app.Status.Phase == appv1alpha1.PhaseFailed {
		t.Fatal("status conflict was recorded as service failure")
	}
}

func TestManualIntentNewTriggerDuringAcknowledgementPreemptsUnobservedJob(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	base := r.Client
	original := app.Spec.RunAt
	newRunAt := "2026-10-01T23:01:00Z"
	race := true
	creates := 0
	r.Client = interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if job, ok := obj.(*batchv1.Job); ok {
				creates++
				if job.Name == manualRunJobName(app.Name, original) {
					job.Finalizers = []string{"test.bex.co/pods-running"}
				}
			}
			return cl.Create(ctx, obj, opts...)
		},
		SubResourcePatch: func(ctx context.Context, cl client.Client, subresource string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if race {
				race = false
				var newer appv1alpha1.App
				if err := base.Get(ctx, client.ObjectKeyFromObject(app), &newer); err != nil {
					return err
				}
				// The backend has no run-history row to select for CancelRun yet.
				newer.Spec.RunAt = newRunAt
				if err := base.Update(ctx, &newer); err != nil {
					return err
				}
			}
			return cl.SubResource(subresource).Patch(ctx, obj, patch, opts...)
		},
	})
	_, err := r.convergeCronRuntime(t.Context(), app, template)
	if !apierrors.IsConflict(err) {
		t.Fatalf("new trigger did not race old acknowledgement: %v", err)
	}
	if _, err := r.failStep(t.Context(), app, err); !apierrors.IsConflict(err) {
		t.Fatal(err)
	}
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), app); err != nil {
		t.Fatal(err)
	}
	if len(app.Status.Runs) != 0 || app.Spec.CancelRun != nil || app.Spec.RunAt != newRunAt {
		t.Fatal("fixture did not reproduce an unobserved older manual Job")
	}
	// Neither an unrelated App-owned Job nor another App lifetime may be
	// preempted merely because it shares the app label or a manual annotation.
	foreignRunAt := "2026-10-01T22:00:00Z"
	foreign := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: manualRunJobName(app.Name, foreignRunAt), Namespace: app.Namespace,
		Labels: map[string]string{labelApp: app.Name}, Annotations: map[string]string{annotationManualRunAt: foreignRunAt},
		OwnerReferences: []metav1.OwnerReference{{APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "App", Name: app.Name, UID: "other-app-lifetime", Controller: new(true)}},
	}}
	unrelated := foreign.DeepCopy()
	unrelated.Name = app.Name + "-other-purpose"
	unrelated.OwnerReferences[0].UID = app.UID
	for _, job := range []*batchv1.Job{foreign, unrelated} {
		if err := base.Create(t.Context(), job); err != nil {
			t.Fatal(err)
		}
	}
	reconcileManualIntent(t, r, app, template)
	assertManualJobAbsent(t, r, app, newRunAt)
	assertManualScheduleSuspended(t, r, app, true)
	var originalJob batchv1.Job
	if err := r.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: manualRunJobName(app.Name, original)}, &originalJob); err != nil {
		t.Fatal(err)
	}
	if originalJob.DeletionTimestamp.IsZero() {
		t.Fatal("unobserved older manual execution was not preempted")
	}
	assertCronRunCanceled(t, app.Status.Runs, originalJob.Name)
	for _, job := range []*batchv1.Job{foreign, unrelated} {
		if err := r.Get(t.Context(), client.ObjectKeyFromObject(job), job); err != nil || !job.DeletionTimestamp.IsZero() {
			t.Fatalf("unrelated Job was preempted: %s: %v", job.Name, err)
		}
	}
	originalJob.Finalizers = nil
	if err := r.Update(t.Context(), &originalJob); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		reconcileManualIntent(t, r, app, template)
	}
	if creates != 2 || app.Status.ManualRunHandledAt != newRunAt {
		t.Fatalf("new intent must execute once after old Pods end: creates=%d marker=%q", creates, app.Status.ManualRunHandledAt)
	}
	assertManualScheduleSuspended(t, r, app, true)
}

func TestManualIntentAcknowledgementUsesControlPlaneClient(t *testing.T) {
	r, app, template := manualIntentFixture(t)
	r.BuildClient = fake.NewClientBuilder().WithScheme(r.Scheme).WithStatusSubresource(&batchv1.Job{}).Build()
	if err := r.ensureManualRun(t.Context(), app, template); err != nil {
		t.Fatal(err)
	}
	var stored appv1alpha1.App
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.ManualRunHandledAt != app.Spec.RunAt {
		t.Fatal("acknowledgement did not reach the control-plane App")
	}
}

func TestLegacyActiveManualJobAdoptionAllowsUnobservedPreemption(t *testing.T) {
	app := cronAppAt(nil, nil)
	// Exercise fitted names too: a prefix guess cannot identify long-named Apps.
	app.Name = "tea-" + strings.Repeat("legacy-cron", 5)
	r, app, template := manualIntentFixtureForApp(t, app)
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: manualRunJobName(app.Name, app.Spec.RunAt), Namespace: app.Namespace,
		Labels: map[string]string{labelApp: app.Name}, Finalizers: []string{"test.bex.co/pods-running"},
		OwnerReferences: []metav1.OwnerReference{{APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "App", Name: app.Name, UID: app.UID, Controller: new(true)}},
	}}
	if err := r.Create(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	// Stop after adoption, before cronRuns can write the first history row.
	if err := r.adoptManualRun(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	app.Spec.RunAt = "2026-10-01T23:02:00Z"
	persistManualIntent(t, r, app)
	reconcileManualIntent(t, r, app, template)
	assertManualJobAbsent(t, r, app, app.Spec.RunAt)
	if err := r.Get(t.Context(), client.ObjectKeyFromObject(job), job); err != nil {
		t.Fatal(err)
	}
	if job.DeletionTimestamp.IsZero() {
		t.Fatal("adopted legacy Job was not preempted before replacement")
	}
	assertCronRunCanceled(t, app.Status.Runs, job.Name)
}
