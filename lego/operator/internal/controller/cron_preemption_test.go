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

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A finalizer models foreground deletion waiting for a still-executing Pod.
// The fake client has no CronJob scheduler: the second case explicitly seeds
// the scheduled Job that escaped the API's stale active-run history.
func TestCronPreemptionWaitsForEveryActiveRun(t *testing.T) {
	for _, namedCancellation := range []bool{true, false} {
		name := "unobserved-scheduled-run"
		if namedCancellation {
			name = "requested-cancellation"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			scheme := wakeScheme()
			app := cronAppAt(nil, nil)
			app.UID = "cron-preemption-app"
			active := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
				Name: app.Name + "-scheduled", Namespace: app.Namespace,
				Labels: map[string]string{labelApp: app.Name}, Finalizers: []string{"test.bex.co/pods-running"},
				OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "CronJob", Name: appv1alpha1.CronJobName(app.Name), UID: "cron-scheduler", Controller: new(true)}},
			}}
			target := "already-gone-run"
			if namedCancellation {
				target = active.Name
			}
			app.Spec.CancelRun = &appv1alpha1.CronRunCancellation{Name: target, RequestedAt: "2026-10-01T21:18:49Z"}
			initialCron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.CronJobName(app.Name), Namespace: app.Namespace, UID: "cron-scheduler"}}
			terminal := active.DeepCopy()
			terminal.Name = app.Name + "-completed"
			terminal.Finalizers = nil
			terminal.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
			foreign := active.DeepCopy()
			foreign.Name = app.Name + "-foreign"
			foreign.OwnerReferences[0].UID = "other-cron"
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, active, initialCron, terminal, foreign).WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if _, ok := obj.(*batchv1.Job); ok {
						var cron batchv1.CronJob
						err := cl.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.CronJobName(app.Name)}, &cron)
						if err != nil || cron.Spec.Suspend == nil || !*cron.Spec.Suspend {
							t.Errorf("deleting active Job before pausing schedule: cron read=%v", err)
						}
					}
					return cl.Delete(ctx, obj, opts...)
				},
			}).Build()
			r := wakeReconciler(cl, scheme)
			template := corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelApp: app.Name}}, Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "app", Image: "busybox:latest"}}}}
			manualKey := client.ObjectKey{Namespace: app.Namespace, Name: manualRunJobName(app.Name, app.Spec.RunAt)}
			for range 2 {
				res, err := r.convergeCronRuntime(ctx, app, template)
				if err != nil {
					t.Fatal(err)
				}
				var cron batchv1.CronJob
				if err := cl.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.CronJobName(app.Name)}, &cron); err != nil {
					t.Fatal(err)
				}
				if cron.Spec.Suspend == nil || !*cron.Spec.Suspend {
					t.Error("schedule resumed during preemption; a pending tick can start a second run")
				}
				if err := cl.Get(ctx, manualKey, &batchv1.Job{}); !apierrors.IsNotFound(err) {
					t.Errorf("manual replacement materialized before old Pods were gone: %v", err)
				}
				assertCronRunCanceled(t, app.Status.Runs, active.Name)
				if res.RequeueAfter == 0 {
					t.Error("pending preemption must requeue")
				}
			}
			for _, untouched := range []*batchv1.Job{terminal, foreign} {
				var live batchv1.Job
				if err := cl.Get(ctx, client.ObjectKeyFromObject(untouched), &live); err != nil || !live.DeletionTimestamp.IsZero() {
					t.Errorf("unrelated or terminal Job was deleted: %s %v", untouched.Name, err)
				}
			}
			var stored appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
				t.Fatal(err)
			}
			if !namedCancellation {
				assertCronRunCanceled(t, stored.Status.Runs, active.Name)
			}
			var terminating batchv1.Job
			if err := cl.Get(ctx, client.ObjectKeyFromObject(active), &terminating); err != nil {
				t.Fatal(err)
			}
			if terminating.DeletionTimestamp.IsZero() {
				t.Fatal("active run was not preempted")
			}
			terminating.Finalizers = nil
			if err := cl.Update(ctx, &terminating); err != nil {
				t.Fatal(err)
			}
			// Foreground deletion is now complete. The pending manual trigger must
			// progress, without opening the recurring schedule in between.
			for range 3 {
				if _, err := r.convergeCronRuntime(ctx, app, template); err != nil {
					t.Fatal(err)
				}
			}
			assertCronRunCanceled(t, app.Status.Runs, active.Name)
			if err := cl.Get(ctx, manualKey, &batchv1.Job{}); err != nil {
				t.Fatalf("replacement did not start after preemption: %v", err)
			}
			var cron batchv1.CronJob
			if err := cl.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: appv1alpha1.CronJobName(app.Name)}, &cron); err != nil {
				t.Fatal(err)
			}
			if cron.Spec.Suspend == nil || !*cron.Spec.Suspend {
				t.Error("schedule not paused during manual execution")
			}
		})
	}
}

func assertCronRunCanceled(t *testing.T, runs []appv1alpha1.CronRun, name string) {
	t.Helper()
	for _, run := range runs {
		if run.Name == name && run.Status == appv1alpha1.CronRunCanceled {
			return
		}
	}
	t.Errorf("cancellation for %s not persisted before Job deletion", name)
}
