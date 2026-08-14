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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Unit tests for the reconcileKubernetes phase helpers: replica resolution
// and the shared Running terminal both the web and worker paths write through.

func TestDesiredReplicas(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name                string
		app                 *appv1alpha1.App
		worker              bool
		activator           string
		wantReplicas        int32
		wantAutoHibernating bool
		wantRequeue         bool
	}{
		{
			name:         "default replica count is 1",
			app:          &appv1alpha1.App{},
			wantReplicas: 1,
		},
		{
			name:         "explicit spec.replicas wins",
			app:          &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Replicas: 3}},
			wantReplicas: 3,
		},
		{
			name:         "suspended scales to 0",
			app:          &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Replicas: 3, Suspended: true}},
			wantReplicas: 0,
		},
		{
			name:                "idle free-tier app past TTL auto-hibernates to 0",
			app:                 mkIdleApp("free", 300, now.Add(-10*time.Minute), false),
			activator:           "bex-activator",
			wantReplicas:        0,
			wantAutoHibernating: true,
		},
		{
			name:      "worker never auto-hibernates",
			app:       mkIdleApp("free", 300, now.Add(-10*time.Minute), false),
			worker:    true,
			activator: "bex-activator",
			// mkIdleApp leaves spec.replicas 0 => default 1.
			wantReplicas: 1,
		},
		{
			name:         "no activator configured: auto-sleep disabled",
			app:          mkIdleApp("free", 300, now.Add(-10*time.Minute), false),
			wantReplicas: 1,
		},
		{
			name: "autoscaling seeds from the annotation, not spec.replicas",
			app: &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{annotAutoscaleReplicas: "4"},
				},
				Spec: appv1alpha1.AppSpec{
					Replicas:    2,
					Autoscaling: &appv1alpha1.AutoscalingSpec{Enabled: true},
				},
			},
			// nil MetricsReader: applyAutoscaling holds the seeded count and
			// declines the poll requeue.
			wantReplicas: 4,
		},
		{
			name: "autoscaling annotation ignored for a worker",
			app: &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{annotAutoscaleReplicas: "4"},
				},
				Spec: appv1alpha1.AppSpec{
					Replicas:    2,
					Autoscaling: &appv1alpha1.AutoscalingSpec{Enabled: true},
				},
			},
			worker:       true,
			wantReplicas: 2,
		},
		{
			name: "autoscaling annotation ignored while suspended",
			app: &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{annotAutoscaleReplicas: "4"},
				},
				Spec: appv1alpha1.AppSpec{
					Replicas:    2,
					Suspended:   true,
					Autoscaling: &appv1alpha1.AutoscalingSpec{Enabled: true},
				},
			},
			wantReplicas: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &AppReconciler{ActivatorService: tc.activator}
			replicas, requeue, autoHibernating := r.desiredReplicas(context.Background(), tc.app, tc.worker)
			if replicas != tc.wantReplicas {
				t.Errorf("replicas = %d, want %d", replicas, tc.wantReplicas)
			}
			if autoHibernating != tc.wantAutoHibernating {
				t.Errorf("autoHibernating = %v, want %v", autoHibernating, tc.wantAutoHibernating)
			}
			if requeue != tc.wantRequeue {
				t.Errorf("autoscaleRequeue = %v, want %v", requeue, tc.wantRequeue)
			}
		})
	}
}

// phaseDep builds a Deployment shell plus the stored server-side object with
// the given rollout status, sharing the selector/revision shape the pod
// readiness check keys on.
func phaseDep(status appsv1.DeploymentStatus) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 1},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: "web"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{labelApp: "web", labelRevision: "rev-1"},
			}},
		},
		Status: status,
	}
}

// TestMarkRunningTerminal covers the shared Running terminal both the web and
// worker paths write through, including its statusSettled guard: a settled app
// re-stamping identical status on a steady-state pass must not issue a write.
func TestMarkRunningTerminal(t *testing.T) {
	newApp := func() *appv1alpha1.App {
		return &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "default", Generation: 2,
		}}
	}
	dep := phaseDep(appsv1.DeploymentStatus{ReadyReplicas: 2})

	t.Run("stamps phase, revision, and the Deployed condition", func(t *testing.T) {
		app := newApp()
		cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
			WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
		r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}
		if err := r.markRunning(context.Background(), app, dep, 2, "app running (kubernetes)"); err != nil {
			t.Fatal(err)
		}
		var stored appv1alpha1.App
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), &stored); err != nil {
			t.Fatal(err)
		}
		if stored.Status.Phase != appv1alpha1.PhaseRunning || stored.Status.ActiveRevision != "rev-2" {
			t.Fatalf("unexpected persisted status %+v", stored.Status)
		}
		cond := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
		if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != "Deployed" ||
			cond.Message != "2/2 replicas ready" {
			t.Fatalf("unexpected Ready condition %+v", cond)
		}
	})

	t.Run("settled steady-state pass skips the status write", func(t *testing.T) {
		app := newApp()
		cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
			WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
		r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}
		if err := r.markRunning(context.Background(), app, dep, 2, "app running (kubernetes)"); err != nil {
			t.Fatal(err)
		}
		var settled appv1alpha1.App
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), &settled); err != nil {
			t.Fatal(err)
		}
		before := settled.ResourceVersion
		if err := r.markRunning(context.Background(), &settled, dep, 2, "app running (kubernetes)"); err != nil {
			t.Fatal(err)
		}
		var after appv1alpha1.App
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), &after); err != nil {
			t.Fatal(err)
		}
		if after.ResourceVersion != before {
			t.Fatal("steady-state Running pass issued a status write")
		}
	})
}
