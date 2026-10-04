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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
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
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: "uid-dep"},
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

// servedReplicaSet is the ReplicaSet dep still retains for a served release: what
// a failed rollout over it is restored from when the release has no record.
func servedReplicaSet(dep *appsv1.Deployment, revision string) *appsv1.ReplicaSet {
	controller := true
	labels := map[string]string{"app": dep.Name, labelRevision: revision}
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: dep.Name + "-served", Namespace: dep.Namespace, Labels: labels,
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: dep.Name, UID: dep.UID, Controller: &controller}},
		},
		Spec: appsv1.ReplicaSetSpec{Selector: dep.Spec.Selector, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}}},
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

// w8/m44: over a release that had served, the failed rollout's diagnosis was
// dropped — Ready (rightly) describes the serving release, so the deploy row
// waited out bex-api's gate and closed "did not become healthy … check the
// service logs" 18 minutes in, for an image that was never pulled (and, sweep
// 39, for w4/m112's failing health check). The diagnosis now rides
// ConditionRollout, attributed to the failed release generation, and the Ready
// message names it.
func TestRolloutDeadlineOverPriorReleaseKeepsTheDiagnosis(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		state      corev1.ContainerState
		ready      bool
		wantReason string
		wantMsg    string
	}{
		"image pull": {
			state: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason: "ImagePullBackOff", Message: `Back-off pulling image "docker.io/x/y:does-not-exist-999"`,
			}},
			wantReason: "ImagePullBackOff", wantMsg: "image pull is failing: Back-off pulling image",
		},
		"nothing diagnosed": {
			state:      corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			ready:      true,
			wantReason: "ProgressDeadlineExceeded", wantMsg: "rollout did not become healthy within the progress deadline",
		},
	} {
		t.Run(name, func(t *testing.T) {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
				Spec:       appv1alpha1.AppSpec{Image: "docker.io/x/y:does-not-exist-999"},
				Status: appv1alpha1.AppStatus{
					Phase: appv1alpha1.PhaseDeploying, ActiveRevision: "rev-2", ObservedGeneration: 2,
					ReleaseGeneration: 3,
				},
			}
			dep := progressDeadlineDep(app.Name)
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "web-3", Namespace: "default",
					Labels: map[string]string{"app": "web", labelRevision: "rev-3"}},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{Name: "app", State: tc.state, Ready: tc.ready}},
				},
			}
			if tc.ready {
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			}
			cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
				WithObjects(app, dep, pod, servedReplicaSet(dep, "rev-2")).WithStatusSubresource(&appv1alpha1.App{}).Build()
			r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

			if _, err := r.reportRolloutProgress(ctx, app, dep, 1, 3000, "waiting"); err != nil {
				t.Fatalf("reportRolloutProgress: %v", err)
			}
			var stored appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
				t.Fatal(err)
			}
			if stored.Status.Phase != appv1alpha1.PhaseRunning {
				t.Fatalf("phase = %q, want Running — the prior release keeps serving", stored.Status.Phase)
			}
			rollout := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionRollout)
			if rollout == nil || rollout.Status != metav1.ConditionFalse || rollout.Reason != tc.wantReason ||
				!strings.HasPrefix(rollout.Message, tc.wantMsg) || rollout.ObservedGeneration != 3 {
				t.Fatalf("Rollout = %+v, want False/%s %q at release generation 3", rollout, tc.wantReason, tc.wantMsg)
			}
			ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
			if ready == nil || ready.Reason != appv1alpha1.ReasonPriorReleaseServing ||
				!strings.HasPrefix(ready.Message, "the latest rollout failed: "+tc.wantMsg) {
				t.Fatalf("Ready = %+v, want the diagnosis in the prior-release message", ready)
			}
		})
	}
}

func TestPermanentPullFailureSettlesBeforeRolloutDeadline(t *testing.T) {
	for _, tc := range []struct {
		name, reason, message, revision string
		age                             time.Duration
		failed                          bool
	}{
		{"missing tag", "ErrImagePull", "rpc error: code = NotFound desc = manifest missing", "rev-1", 91 * time.Second, true},
		{"manifest unknown", "ImagePullBackOff", "manifest unknown", "rev-1", 91 * time.Second, true},
		{"unauthorized", "ErrImagePull", "unauthorized: authentication required", "rev-1", 91 * time.Second, true},
		{"invalid image name", "InvalidImageName", "invalid reference format: repository name must be lowercase", "rev-1", 0, true},
		{"grace", "ErrImagePull", "rpc error: code = NotFound", "rev-1", 60 * time.Second, false}, // well inside the grace: a loaded runner adds seconds
		{"timeout", "ErrImagePull", "dial tcp: i/o timeout", "rev-1", 10 * time.Minute, false},
		{"registry unavailable", "ErrImagePull", "503 Service Unavailable", "rev-1", 10 * time.Minute, false},
		{"rate limit", "ImagePullBackOff", "429 Too Many Requests", "rev-1", 10 * time.Minute, false},
		{"slow pull", "ContainerCreating", "", "rev-1", 10 * time.Minute, false},
		{"old revision", "ErrImagePull", "rpc error: code = NotFound", "old", 10 * time.Minute, false},
		{"old invalid image", "InvalidImageName", "invalid reference format", "old", 10 * time.Minute, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
				Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying, ActiveRevision: "old", ReleaseGeneration: 3}}
			dep := progressDeadlineDep("web")
			dep.Status.Conditions = nil
			image := "example.org/web:missing"
			if tc.reason == "InvalidImageName" {
				image = "example.org/Web:latest"
			}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-new", Namespace: "default",
				Labels: map[string]string{"app": "web", labelRevision: tc.revision}, CreationTimestamp: metav1.NewTime(time.Now().Add(-tc.age))},
				Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "app", Image: image,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: tc.reason, Message: tc.message}}}}}}
			cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(app, dep, pod, servedReplicaSet(dep, "old")).WithStatusSubresource(&appv1alpha1.App{}).Build()
			r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
			result, err := r.reportRolloutProgress(ctx, app, dep, 1, 3000, "waiting")
			if err != nil {
				t.Fatal(err)
			}
			var stored appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
				t.Fatal(err)
			}
			rollout := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionRollout)
			if !tc.failed {
				if rollout != nil && rollout.Status == metav1.ConditionFalse {
					t.Fatalf("transient/irrelevant pull failed rollout: %+v", rollout)
				}
				if result.RequeueAfter == 0 || stored.Status.Phase != appv1alpha1.PhaseDeploying {
					t.Fatalf("did not preserve rollout budget: %+v %+v", result, stored.Status)
				}
				if ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady); ready != nil && ready.Reason == "InvalidImageName" {
					t.Fatalf("stale malformed image diagnosed the current revision: %+v", ready)
				}
				return
			}
			wantReason := "ImagePullBackOff"
			if tc.reason == "InvalidImageName" {
				wantReason = tc.reason
			}
			if rollout == nil || rollout.Status != metav1.ConditionFalse || rollout.Reason != wantReason || rollout.ObservedGeneration != 3 || !strings.Contains(rollout.Message, image) {
				t.Fatalf("missing image failure: %+v", rollout)
			}
			if stored.Status.Phase != appv1alpha1.PhaseRunning || stored.Status.ActiveRevision != "old" {
				t.Fatalf("prior release changed: %+v", stored.Status)
			}
			var unchanged appsv1.Deployment
			if err := cl.Get(ctx, client.ObjectKeyFromObject(dep), &unchanged); err != nil {
				t.Fatal(err)
			}
			if *unchanged.Spec.Replicas != 1 {
				t.Fatal("scaled down serving release")
			}
		})
	}
}

// w8/039: kubelet's startup-probe kill lands seconds before the progress
// deadline, so the settle-time scan sees a freshly restarted container that
// has not had a probe period to fail. The diagnosis Ready already carried for
// this generation must survive to the terminal, not the generic line.
func TestRolloutDeadlineKeepsProbeDiagnosisAcrossLateRestart(t *testing.T) {
	const probeMsg = "the container is running but its startup health check has not succeeded, so the rollout is waiting: a TCP connect to port 3000."
	restartedPod := func() *corev1.Pod {
		notStarted := false
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "web-0", Namespace: "default",
				Labels: map[string]string{"app": "web", labelRevision: "rev-1"},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "app",
				StartupProbe: &corev1.Probe{
					ProbeHandler:  corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(3000)}},
					PeriodSeconds: 10, TimeoutSeconds: 1, FailureThreshold: 90,
				},
			}}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app", Started: &notStarted, RestartCount: 1,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.NewTime(time.Now().Add(-3 * time.Second)),
				}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 137}},
			}}},
		}
	}
	settle := func(t *testing.T, app *appv1alpha1.App) appv1alpha1.App {
		t.Helper()
		ctx := context.Background()
		cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
			WithObjects(app, progressDeadlineDep(app.Name), restartedPod()).WithStatusSubresource(&appv1alpha1.App{}).Build()
		r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
		if _, err := r.reportRolloutProgress(ctx, app, progressDeadlineDep(app.Name), 1, 3000, "waiting"); err != nil {
			t.Fatalf("reportRolloutProgress: %v", err)
		}
		var stored appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
			t.Fatal(err)
		}
		return stored
	}
	diagnosed := func(gen int64) []metav1.Condition {
		return []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reasonHealthCheckFailing,
			Message: probeMsg, ObservedGeneration: gen, LastTransitionTime: metav1.Now(),
		}}
	}

	t.Run("first release", func(t *testing.T) {
		stored := settle(t, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 1},
			Spec:       appv1alpha1.AppSpec{Image: "docker.io/traefik/whoami:v1.10"},
			Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying, Conditions: diagnosed(1)},
		})
		ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
		if stored.Status.Phase != appv1alpha1.PhaseFailed || ready == nil || ready.Reason != reasonHealthCheckFailing || ready.Message != probeMsg {
			t.Fatalf("phase = %q, Ready = %+v; want Failed with the probe diagnosis", stored.Status.Phase, ready)
		}
	})
	t.Run("over prior release", func(t *testing.T) {
		stored := settle(t, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
			Spec:       appv1alpha1.AppSpec{Image: "docker.io/traefik/whoami:v1.10"},
			Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying, ActiveRevision: "rev-0",
				ObservedGeneration: 2, Conditions: diagnosed(3)},
		})
		rollout := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionRollout)
		if rollout == nil || rollout.Reason != reasonHealthCheckFailing || rollout.Message != probeMsg {
			t.Fatalf("Rollout = %+v; want the probe diagnosis carried to the failed deploy", rollout)
		}
	})
	t.Run("stale generation diagnosis is not reused", func(t *testing.T) {
		stored := settle(t, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 2},
			Spec:       appv1alpha1.AppSpec{Image: "docker.io/traefik/whoami:v1.10"},
			Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying, Conditions: diagnosed(1)},
		})
		ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
		if ready == nil || ready.Reason != "ProgressDeadlineExceeded" {
			t.Fatalf("Ready = %+v; want the generic deadline line", ready)
		}
	})
}
