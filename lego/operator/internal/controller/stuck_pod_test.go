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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
