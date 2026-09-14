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
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m103: a first deploy whose Deployment hits ProgressDeadlineExceeded must
// leave Deploying — the deploy row already reads Failed while the service
// header used to contradict it forever.

func progressDeadlineDep(name string) *appsv1.Deployment {
	one := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name, labelRevision: "rev-1"}},
			},
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentProgressing,
				Status: corev1.ConditionFalse,
				Reason: "ProgressDeadlineExceeded",
			}},
		},
	}
}

func rolloutFailScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return scheme
}

func TestFirstDeployRolloutDeadlineSettlesFailedCrashLoop(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 1},
		Spec:       appv1alpha1.AppSpec{Image: "registry.example/app:bad"},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying},
	}
	dep := progressDeadlineDep(app.Name)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-0", Namespace: "default",
			Labels: map[string]string{"app": "web", labelRevision: "rev-1"},
		},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name: "app",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason: "CrashLoopBackOff", Message: "back-off restarting failed container",
			}},
			LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 127}},
		}}},
	}
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
		WithObjects(app, dep, pod).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	if _, err := r.reportRolloutProgress(ctx, app, dep, 1, 3000, "waiting"); err != nil {
		t.Fatalf("reportRolloutProgress: %v", err)
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want %q — first-deploy crash-loop past the rollout budget", stored.Status.Phase, appv1alpha1.PhaseFailed)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Reason != "CrashLoopBackOff" || !strings.Contains(ready.Message, "restarting") {
		t.Fatalf("Ready = %+v, want CrashLoopBackOff diagnosis preserved for the deploy row", ready)
	}
}

func TestFirstDeployRolloutDeadlineSettlesFailedImagePull(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 1},
		Spec:       appv1alpha1.AppSpec{Image: "docker.io/traefik/whoami:v0-does-not-exist"},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying},
	}
	dep := progressDeadlineDep(app.Name)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web-0", Namespace: "default",
			Labels: map[string]string{"app": "web", labelRevision: "rev-1"},
		},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name: "app",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason: "ImagePullBackOff", Message: `Back-off pulling image "docker.io/traefik/whoami:v0-does-not-exist"`,
			}},
		}}},
	}
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
		WithObjects(app, dep, pod).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	if _, err := r.reportRolloutProgress(ctx, app, dep, 1, 3000, "waiting"); err != nil {
		t.Fatalf("reportRolloutProgress: %v", err)
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want %q — unpullable first image must settle Failed (18m observer class)", stored.Status.Phase, appv1alpha1.PhaseFailed)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Reason != "ImagePullBackOff" || !strings.Contains(ready.Message, "image pull is failing") {
		t.Fatalf("Ready = %+v, want ImagePullBackOff diagnosis unchanged", ready)
	}
}

func TestRolloutDeadlineOverPriorReleaseStaysRunning(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
		Spec:       appv1alpha1.AppSpec{Image: "registry.example/app:bad"},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseDeploying, Image: "registry.example/app:bad",
			ActiveRevision: "rev-2", ObservedGeneration: 2,
		},
	}
	dep := progressDeadlineDep(app.Name)
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
		WithObjects(app, dep).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	if _, err := r.reportRolloutProgress(ctx, app, dep, 1, 3000, "waiting"); err != nil {
		t.Fatalf("reportRolloutProgress: %v", err)
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("phase = %q, want %q — prior release keeps serving (w6/m124 control for rollouts)", stored.Status.Phase, appv1alpha1.PhaseRunning)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue || ready.Reason != appv1alpha1.ReasonPriorReleaseServing {
		t.Fatalf("Ready = %+v, want True/PriorReleaseServing", ready)
	}
}

func TestRolloutDeadlineOverParkedPriorReleaseStaysHibernated(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
		Spec:       appv1alpha1.AppSpec{Image: "registry.example/app:bad"},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseDeploying, ActiveRevision: "rev-2", ObservedGeneration: 2,
		},
	}
	zero := int32(0)
	dep := progressDeadlineDep(app.Name)
	dep.Spec.Replicas = &zero
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
		WithObjects(app, dep).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	if _, err := r.reportRolloutProgress(ctx, app, dep, 0, 3000, "waiting"); err != nil {
		t.Fatalf("reportRolloutProgress: %v", err)
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseHibernated {
		t.Fatalf("phase = %q, want %q — parked prior release must not become Running", stored.Status.Phase, appv1alpha1.PhaseHibernated)
	}
}

func TestDeploymentProgressDeadlineExceededHelper(t *testing.T) {
	if deploymentProgressDeadlineExceeded(nil) {
		t.Fatal("nil dep must be false")
	}
	dep := progressDeadlineDep("x")
	if !deploymentProgressDeadlineExceeded(dep) {
		t.Fatal("want true for ProgressDeadlineExceeded")
	}
	dep.Status.Conditions[0].Status = corev1.ConditionTrue
	if deploymentProgressDeadlineExceeded(dep) {
		t.Fatal("Progressing=True must not settle")
	}
}
