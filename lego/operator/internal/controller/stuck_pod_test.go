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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestStuckPodMessage pins the w9/011 stuck-rollout diagnosis: a crash-looping
// tenant pod yields an actionable Ready-condition message (with the $PORT hint
// only when the App has one), an image-pull failure surfaces the kubelet's
// message, and a merely progressing rollout yields nothing.
func TestStuckPodMessage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
	}
	pod := func(name string, cs corev1.ContainerStatus) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: "default", Labels: map[string]string{"app": "web"},
			},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{cs}},
		}
	}

	crashing := pod("web-1", corev1.ContainerStatus{
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		LastTerminationState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{ExitCode: 1},
		},
	})
	pulling := pod("web-1", corev1.ContainerStatus{
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
			Reason: "ImagePullBackOff", Message: `Back-off pulling image "zot/web:gen-1"`,
		}},
	})
	starting := pod("web-1", corev1.ContainerStatus{
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
	})

	tests := []struct {
		name       string
		pod        *corev1.Pod
		port       int
		wantReason string
		wantIn     []string
		wantNotIn  []string
	}{
		{
			name: "crash loop with port", pod: crashing, port: 3000,
			wantReason: "CrashLoopBackOff",
			wantIn:     []string{"last exit code 1", "$PORT (3000)", "below 1024", "service logs"},
		},
		{
			name: "crash loop without port (worker)", pod: crashing, port: 0,
			wantReason: "CrashLoopBackOff",
			wantIn:     []string{"last exit code 1", "service logs"},
			wantNotIn:  []string{"$PORT"},
		},
		{
			name: "image pull backoff", pod: pulling, port: 3000,
			wantReason: "ImagePullBackOff",
			wantIn:     []string{"image pull is failing", "Back-off pulling image"},
		},
		{
			name: "merely progressing", pod: starting, port: 3000,
			wantReason: "", wantIn: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.pod).Build()
			r := &AppReconciler{Client: cl}
			reason, msg := r.stuckPodMessage(context.Background(), dep, tc.port)
			if reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q (msg %q)", reason, tc.wantReason, msg)
			}
			if tc.wantReason == "" && msg != "" {
				t.Fatalf("msg = %q, want empty for a progressing rollout", msg)
			}
			for _, s := range tc.wantIn {
				if !strings.Contains(msg, s) {
					t.Errorf("message %q missing %q", msg, s)
				}
			}
			for _, s := range tc.wantNotIn {
				if strings.Contains(msg, s) {
					t.Errorf("message %q must not contain %q", msg, s)
				}
			}
		})
	}
}

// TestProbeStallMessage is w4/m112: the one stall class stuckPodMessage used
// to miss entirely. A pod whose health-check probe keeps failing is RUNNING,
// never Waiting, so every case above `continue`d past it and the App's Ready
// condition carried only the generic "waiting for … pods to become ready".
// Live on 2026-09-17 that left a 404ing Health Check Path indistinguishable
// from a slow image pull for the full 900s rollout budget.
func TestProbeStallMessage(t *testing.T) {
	httpProbe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
			Path: "/qa-bogus-health", Port: intstr.FromInt(3000),
		}},
		PeriodSeconds: 10, TimeoutSeconds: 5,
	}
	tcpProbe := &corev1.Probe{
		ProbeHandler:  corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(3000)}},
		PeriodSeconds: 10, TimeoutSeconds: 5,
	}
	// Old enough that a 10s-period / 5s-timeout probe has had its chance.
	longAgo := metav1.NewTime(time.Now().Add(-5 * time.Minute))
	justNow := metav1.NewTime(time.Now())

	probePod := func(startup, readiness *corev1.Probe, cs corev1.ContainerStatus) corev1.Pod {
		cs.Name = "app"
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "default"},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name: "app", StartupProbe: startup, ReadinessProbe: readiness,
			}}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{cs}},
		}
	}
	running := func(started *bool, ready bool, at metav1.Time, restarts int32) corev1.ContainerStatus {
		return corev1.ContainerStatus{
			Ready: ready, Started: started, RestartCount: restarts,
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: at}},
		}
	}
	yes, no := true, false

	tests := []struct {
		name       string
		pod        corev1.Pod
		wantReason string
		wantIn     []string
		wantNotIn  []string
	}{
		{
			// The live case: startup probe still failing on a 404ing path.
			name:       "startup probe failing names the path",
			pod:        probePod(httpProbe, httpProbe, running(&no, false, longAgo, 0)),
			wantReason: reasonHealthCheckFailing,
			wantIn:     []string{"startup health check", "GET /qa-bogus-health on port 3000", "2xx or 3xx"},
		},
		{
			// Boot succeeded, so kubelet has handed over to readiness.
			name:       "readiness probe failing after startup passed",
			pod:        probePod(httpProbe, httpProbe, running(&yes, false, longAgo, 0)),
			wantReason: reasonHealthCheckFailing,
			wantIn:     []string{"readiness health check", "GET /qa-bogus-health"},
			wantNotIn:  []string{"startup health check"},
		},
		{
			// Restarts prove the check is failing, not merely slow.
			name:       "liveness restarts are reported",
			pod:        probePod(httpProbe, httpProbe, running(&yes, false, longAgo, 3)),
			wantReason: reasonHealthCheckFailing,
			wantIn:     []string{"restarted the container 3 time(s)"},
		},
		{
			// No Health Check Path set: kubelet only asks whether the process
			// is listening, and the message must say so rather than invent a path.
			name:       "tcp probe reports the port, not a path",
			pod:        probePod(tcpProbe, tcpProbe, running(&no, false, longAgo, 0)),
			wantReason: reasonHealthCheckFailing,
			wantIn:     []string{"a TCP connect to port 3000"},
			wantNotIn:  []string{"GET "},
		},
		{
			// A pod two seconds old has not had a probe period yet. Users act
			// on this message, so a progressing rollout must stay silent.
			name: "too young to have failed a probe",
			pod:  probePod(httpProbe, httpProbe, running(&no, false, justNow, 0)),
		},
		{
			name: "ready container is not a stall",
			pod:  probePod(httpProbe, httpProbe, running(&yes, true, longAgo, 0)),
		},
		{
			name: "no probe configured says nothing",
			pod:  probePod(nil, nil, running(nil, false, longAgo, 0)),
		},
		{
			// A pod on its way out is not the rollout's problem.
			name: "terminating pod is ignored",
			pod: func() corev1.Pod {
				p := probePod(httpProbe, httpProbe, running(&no, false, longAgo, 0))
				now := metav1.NewTime(time.Now())
				p.DeletionTimestamp = &now
				return p
			}(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reason, msg := probeStallMessage([]corev1.Pod{tc.pod})
			if reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q (msg %q)", reason, tc.wantReason, msg)
			}
			if tc.wantReason == "" && msg != "" {
				t.Fatalf("msg = %q, want silence", msg)
			}
			for _, want := range tc.wantIn {
				if !strings.Contains(msg, want) {
					t.Errorf("message %q missing %q", msg, want)
				}
			}
			for _, unwanted := range tc.wantNotIn {
				if strings.Contains(msg, unwanted) {
					t.Errorf("message %q must not contain %q", msg, unwanted)
				}
			}
		})
	}
}

// A crash-looping container still reports the crash: it is the stronger
// signal, and probeStallMessage runs only after every Waiting case misses.
func TestCrashLoopWinsOverProbeStall(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
		},
	}
	probe := &corev1.Probe{
		ProbeHandler:  corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt(3000)}},
		PeriodSeconds: 10, TimeoutSeconds: 5,
	}
	startedAt := metav1.NewTime(time.Now().Add(-5 * time.Minute))
	pods := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "web-unready", Namespace: "default", Labels: map[string]string{"app": "web"}},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", ReadinessProbe: probe}}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: startedAt}},
			}}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "web-crashing", Namespace: "default", Labels: map[string]string{"app": "web"}},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "app",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}}},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pods[0], pods[1]).Build()
	r := &AppReconciler{Client: cl}
	reason, _ := r.stuckPodMessage(context.Background(), dep, 3000)
	if reason != "CrashLoopBackOff" {
		t.Fatalf("reason = %q, want CrashLoopBackOff to win over the probe stall", reason)
	}
}

func TestDeploymentRolloutReadyRejectsReadyOldReplicaSet(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 2,
			Replicas:           3,
			UpdatedReplicas:    1,
			ReadyReplicas:      2,
			AvailableReplicas:  2,
		},
	}
	if deploymentRolloutReady(dep, 2) {
		t.Fatal("rollout with two ready old replicas and one failing new replica reported complete")
	}
	dep.Status.Replicas = 2
	dep.Status.UpdatedReplicas = 2
	if !deploymentRolloutReady(dep, 2) {
		t.Fatal("fully updated and available rollout did not report complete")
	}
}

// TestStuckPodMessageSurfacesUnresolvableConfig pins w7/m79/t001: the class the
// w9/011 diagnosis originally missed.
//
// A pod whose Secret/ConfigMap reference cannot be resolved never starts, so it
// never crash-loops and never fails an image pull — the two cases w9/011 covers.
// It simply waits in CreateContainerConfigError while the deploy times out as an
// unexplained health-check failure. That is the shape of the 2026-08-08
// incident's first failure leg, where the operator had no signal to give and the
// cause was visible only in pod state no tenant surface shows.
func TestStuckPodMessageSurfacesUnresolvableConfig(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "forum", Namespace: "tea-a"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "forum"}},
		},
	}
	configErrPod := func(kubeletMessage string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "forum-1", Namespace: "tea-a", Labels: map[string]string{"app": "forum"},
			},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
					Reason: "CreateContainerConfigError", Message: kubeletMessage,
				}},
			}}},
		}
	}

	for _, tc := range []struct {
		name     string
		kubelet  string
		mustName string
	}{
		{
			name:     "missing datastore Secret",
			kubelet:  `secret "dpg-d9nqg9dcavls73fp8m2g-app" not found`,
			mustName: "dpg-d9nqg9dcavls73fp8m2g-app",
		},
		{
			name:     "Secret present but the key is absent",
			kubelet:  `couldn't find key uri in Secret tea-a/red-d9x`,
			mustName: "red-d9x",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configErrPod(tc.kubelet)).Build()
			r := &AppReconciler{Client: cl, Scheme: scheme}

			reason, msg := r.stuckPodMessage(context.Background(), dep, 3000)
			if reason != "CreateContainerConfigError" {
				t.Fatalf("reason = %q, want CreateContainerConfigError — the deploy would time out unexplained", reason)
			}
			// The kubelet already names the exact object; a message that dropped
			// it would leave the user no better off than the silence being fixed.
			if !strings.Contains(msg, tc.mustName) {
				t.Errorf("message does not name the unresolvable object %q: %s", tc.mustName, msg)
			}
			if !strings.Contains(msg, "own namespace") {
				t.Errorf("message gives no actionable cause: %s", msg)
			}
		})
	}
}

// TestStuckPodMessageStaysQuietOnAHealthyRollout is the negative half. A
// condition that fires while a normal rollout is merely progressing is worse
// than none, because users learn to ignore it.
func TestStuckPodMessageStaysQuietOnAHealthyRollout(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "forum", Namespace: "tea-a"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "forum"}},
		},
	}
	for _, waiting := range []*corev1.ContainerStateWaiting{
		nil,                           // running
		{Reason: "ContainerCreating"}, // normal startup
		{Reason: "PodInitializing"},   // normal startup
	} {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "forum-1", Namespace: "tea-a", Labels: map[string]string{"app": "forum"},
			},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
				State: corev1.ContainerState{Waiting: waiting},
			}}},
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()
		r := &AppReconciler{Client: cl, Scheme: scheme}
		if reason, msg := r.stuckPodMessage(context.Background(), dep, 3000); msg != "" {
			t.Errorf("a progressing rollout reported %q/%q", reason, msg)
		}
	}
}
