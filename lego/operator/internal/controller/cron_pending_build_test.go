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
	"reflect"
	"testing"

	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestCronControlsOverPendingBuildKeepPriorTemplate(t *testing.T) {
	for _, state := range []string{"failed", "running", "queued"} {
		t.Run(state, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			scheme := wakeScheme()
			app := activeApp("tea-cron-hold")
			app.UID = "cron-hold-uid"
			app.Spec.Type = appv1alpha1.TypeCronJob
			app.Spec.Tier = ""
			app.Spec.Expose = false
			app.Spec.Schedule = "* * * * *"
			app.Spec.Command = "echo prior"
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
				WithStatusSubresource(&appv1alpha1.App{}, &batchv1.Job{}).Build()
			r := wakeReconciler(cl, scheme)
			r.Registry = "zot.test:5000"
			nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
			reconcileTwice(t, r, nn)
			var prior batchv1.CronJob
			g.Expect(cl.Get(ctx, nn, &prior)).To(Succeed())
			var live appv1alpha1.App
			g.Expect(cl.Get(ctx, nn, &live)).To(Succeed())
			priorStatus := live.Status.DeepCopy()
			if priorStatus.ActiveRevision == "" {
				t.Fatal("first cron release never became active")
			}

			releaseTwoFromSource(t, cl, nn)
			g.Expect(cl.Get(ctx, nn, &live)).To(Succeed())
			live.Spec.Command = "echo unbuilt"
			live.Spec.Env = []appv1alpha1.EnvVar{{Name: "RELEASE", Value: "unbuilt"}}
			g.Expect(cl.Update(ctx, &live)).To(Succeed())
			switch state {
			case "failed":
				storeReleaseTwoBuildFailure(t, cl, nn, appv1alpha1.ConditionBuild)
			case "queued":
				r.MaxConcurrentBuilds = 1
				g.Expect(cl.Create(ctx, buildSlotTaken(app.Namespace))).To(Succeed())
			}
			reconcileTwice(t, r, nn)

			check := func(suspended bool, schedule string) {
				t.Helper()
				var cron batchv1.CronJob
				g.Expect(cl.Get(ctx, nn, &cron)).To(Succeed())
				if cron.Spec.Suspend == nil || *cron.Spec.Suspend != suspended || cron.Spec.Schedule != schedule {
					t.Fatalf("cron controls = suspend %v schedule %q, want %v %q", cron.Spec.Suspend, cron.Spec.Schedule, suspended, schedule)
				}
				if !reflect.DeepEqual(cron.Spec.JobTemplate, prior.Spec.JobTemplate) {
					t.Fatal("pending release changed the prior cron pod template")
				}
				g.Expect(cl.Get(ctx, nn, &live)).To(Succeed())
				if live.Status.ActiveRevision != priorStatus.ActiveRevision || live.Status.Image != priorStatus.Image || live.Status.ArtifactImage != priorStatus.ArtifactImage {
					t.Fatalf("pending release became active: %+v", live.Status)
				}
				if live.Status.ReleaseGeneration != 2 {
					t.Fatalf("release generation = %d, want 2", live.Status.ReleaseGeneration)
				}
				if state == "failed" {
					condition := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionBuild)
					if condition == nil || condition.Status != metav1.ConditionFalse || condition.ObservedGeneration != 2 {
						t.Fatalf("failed build verdict lost: %+v", condition)
					}
					assertNoBuildJobs(t, cl, nn)
				} else if live.Status.Phase != appv1alpha1.PhaseBuilding {
					t.Fatalf("phase = %s, want Building while build is pending", live.Status.Phase)
				}
			}
			setSuspendedAt(t, cl, nn, true, 3)
			reconcileTwice(t, r, nn)
			check(true, "* * * * *")
			if state == "failed" && live.Status.Phase != appv1alpha1.PhaseHibernated {
				t.Fatalf("suspended phase = %s", live.Status.Phase)
			}

			setSuspendedAt(t, cl, nn, false, 4)
			g.Expect(cl.Get(ctx, nn, &live)).To(Succeed())
			live.Spec.Schedule = "*/5 * * * *"
			g.Expect(cl.Update(ctx, &live)).To(Succeed())
			reconcileTwice(t, r, nn)
			check(false, "*/5 * * * *")
			if state == "failed" && live.Status.Phase != appv1alpha1.PhaseRunning {
				t.Fatalf("resumed phase = %s", live.Status.Phase)
			}

			// Manual runs must use the same proven template and pause the schedule.
			live.Spec.RunAt = "2026-09-21T12:00:00Z"
			g.Expect(cl.Update(ctx, &live)).To(Succeed())
			reconcileTwice(t, r, nn)
			check(true, "*/5 * * * *")
			var job batchv1.Job
			key := types.NamespacedName{Namespace: nn.Namespace, Name: manualRunJobName(app.Name, live.Spec.RunAt)}
			g.Expect(cl.Get(ctx, key, &job)).To(Succeed())
			if !reflect.DeepEqual(job.Spec.Template, prior.Spec.JobTemplate.Spec.Template) {
				t.Fatal("manual run used the unbuilt release")
			}
			job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
			g.Expect(cl.Status().Update(ctx, &job)).To(Succeed())
			res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			if err != nil {
				t.Fatal(err)
			}
			check(false, "*/5 * * * *")
			if res.RequeueAfter == 0 {
				t.Fatal("cron run history must keep polling while a failed build is held")
			}
		})
	}
}
