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
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/352: a redeploy whose health path never answers. Its startup check times
// out, which the operator cannot tell from a check that answers 500: both are
// a kubelet probe failure. At the progress deadline the settle put the served
// template back and only then wrote the verdict; with that write lost, nothing
// held the Deployment on the served release, so the next pass rolled the failed
// release again on a fresh 15 minutes, its new instance ran beside the served
// one, and the deploy closed at bex-api's 18-minute gate. The verdict is now
// stored first, and a settle that cannot store it leaves the Deployment on its
// deadline for the next pass.
func TestRolloutDeadlineStoresItsVerdictBeforeRestoringTheServedRelease(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
		Spec:       appv1alpha1.AppSpec{Image: "docker.io/library/python:3.12-alpine", HealthCheckPath: "/hang"},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseDeploying, ActiveRevision: "rev-2", ObservedGeneration: 2, ReleaseGeneration: 3,
		},
	}
	dep := progressDeadlineDep(app.Name)
	dep.Spec.Template.Labels[labelRevision] = "rev-3"
	// The failing release's instance: its startup check of GET /hang has timed
	// out on every period for the whole budget.
	hanging := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-3", Namespace: "default",
			Labels: map[string]string{"app": "web", labelRevision: "rev-3"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name: appContainerName,
			StartupProbe: &corev1.Probe{
				ProbeHandler:   healthProbeExecHandler("/hang", 10000, ""),
				TimeoutSeconds: healthCheckTimeoutSeconds, PeriodSeconds: healthCheckPeriodSeconds,
				FailureThreshold: healthCheckStartupFailureThreshold,
			},
		}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
			Name: appContainerName, Started: new(false),
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{
				StartedAt: metav1.NewTime(time.Now().Add(-time.Duration(rolloutBudgetSeconds) * time.Second)),
			}},
		}}},
	}
	writesFail := true
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
		WithObjects(app, dep, hanging, servedReplicaSet(dep, "rev-2")).WithStatusSubresource(&appv1alpha1.App{}).
		WithInterceptorFuncs(interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if writesFail {
				return errors.New("etcdserver: request timed out")
			}
			return c.SubResource(sub).Update(ctx, obj, opts...)
		}}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
	settle := func() error {
		t.Helper()
		var stored appv1alpha1.App
		var current appsv1.Deployment
		if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
			t.Fatal(err)
		}
		if err := cl.Get(ctx, client.ObjectKeyFromObject(dep), &current); err != nil {
			t.Fatal(err)
		}
		_, err := r.reportRolloutProgress(ctx, &stored, &current, 1, 10000, "waiting")
		return err
	}
	template := func() string {
		t.Helper()
		var current appsv1.Deployment
		if err := cl.Get(ctx, client.ObjectKeyFromObject(dep), &current); err != nil {
			t.Fatal(err)
		}
		return current.Spec.Template.Labels[labelRevision]
	}

	if err := settle(); err == nil {
		t.Fatal("a settle whose verdict was not stored reported success")
	}
	if got := template(); got != "rev-3" {
		t.Fatalf("Deployment template = %s after an unstored verdict, want rev-3 left on its deadline", got)
	}

	writesFail = false
	if err := settle(); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got := template(); got != "rev-2" {
		t.Fatalf("Deployment template = %s, want the served rev-2 back", got)
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	rollout := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionRollout)
	if rollout == nil || rollout.Status != metav1.ConditionFalse || rollout.ObservedGeneration != 3 ||
		rollout.Reason != reasonHealthCheckFailing || !strings.Contains(rollout.Message, appv1alpha1.ProbeStallFailed) ||
		!strings.Contains(rollout.Message, "GET /hang on port 10000") || strings.Contains(rollout.Message, appv1alpha1.ProbeStallWaiting) {
		t.Fatalf("Rollout = %+v, want the timed-out check's failed verdict at release generation 3", rollout)
	}
	if stored.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("phase = %q, want Running on the served release", stored.Status.Phase)
	}
}
