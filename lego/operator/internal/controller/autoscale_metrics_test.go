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

	"github.com/prometheus/client_golang/prometheus/testutil"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// freshlyAutoscaledApp is a starter-tier App whose owner just turned
// autoscaling on (min 1, max 3, CPU 70%): nothing recorded yet, and a stored
// manual count of replicas.
func freshlyAutoscaledApp(name string, replicas int32) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appv1alpha1.AppSpec{Tier: "starter", Replicas: replicas, Autoscaling: &appv1alpha1.AutoscalingSpec{
			Enabled: true, MinReplicas: 1, MaxReplicas: 3, TargetCPUPercent: new(int32(70)),
		}},
	}
}

// runningDeployment is the Deployment of app running n replicas.
func runningDeployment(app *appv1alpha1.App, n int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace},
		Spec:       appsv1.DeploymentSpec{Replicas: &n},
	}
}

// w4/m364: production refused every metrics read (the operator had no
// metrics.k8s.io grant) and applyAutoscaling held the count without a word for
// months. A failed read now sets AutoscalingMetricsAvailable=False with a
// tenant-safe message, counts the failure, and logs the error with the App
// once per window. The first good read clears the condition.
func TestAFailedMetricsReadIsVisibleAndClearsOnRecovery(t *testing.T) {
	app := freshlyAutoscaledApp("metrics-refused", 3)
	readErr := errors.New(`pods.metrics.k8s.io is forbidden: User "system:serviceaccount:bex-system:bex-controller-manager" cannot list resource "pods"`)
	var failing = true
	r := &AppReconciler{
		Client: fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(app).Build(),
		MetricsReader: func(context.Context, string, string) ([]PodUsage, error) {
			if failing {
				return nil, readErr
			}
			return threePodsAt(0.2), nil // wants 2 of 3: within the window, held
		},
	}
	var logged capturedLog
	ctx := logged.into(context.Background())
	failures := func() float64 {
		return testutil.ToFloat64(autoscalingMetricsReadFailures.WithLabelValues(app.Namespace, app.Name))
	}
	before := failures()

	for range 3 {
		if got, requeue := r.applyAutoscaling(ctx, app, 3); got != 3 || !requeue {
			t.Fatalf("a failed read returned %d requeue %v, want the count held at 3 and a retry", got, requeue)
		}
	}
	cond := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionAutoscalingMetricsAvailable)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != appv1alpha1.AutoscalingMetricsUnavailableReason {
		t.Fatalf("condition = %+v, want AutoscalingMetricsAvailable=False/MetricsUnavailable", cond)
	}
	if strings.Contains(cond.Message, "serviceaccount") || strings.Contains(cond.Message, "forbidden") {
		t.Fatalf("the tenant-visible message leaks the read error: %q", cond.Message)
	}
	if got := failures() - before; got != 3 {
		t.Fatalf("counted %v failed reads, want 3", got)
	}
	if len(logged.lines) != 1 || !strings.Contains(logged.lines[0], `"app"="metrics-refused"`) || !strings.Contains(logged.lines[0], "forbidden") {
		t.Fatalf("logged %q, want the error once, naming the App", logged.lines)
	}

	// Past the window a still-failing read is logged again.
	r.metricsFailureLogged.Store(app.Namespace+"/"+app.Name, time.Now().Add(-metricsFailureLogInterval-time.Second))
	r.applyAutoscaling(ctx, app, 3)
	if len(logged.lines) != 2 {
		t.Fatalf("logged %d lines after the window, want a second", len(logged.lines))
	}

	failing = false
	r.applyAutoscaling(ctx, app, 3)
	if cond := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionAutoscalingMetricsAvailable); cond != nil {
		t.Fatalf("a good read left the condition %+v", cond)
	}
	if _, logged := r.metricsFailureLogged.Load(app.Namespace + "/" + app.Name); logged {
		t.Fatal("a good read left the failure log's rate limit armed")
	}
}

// Turning autoscaling off (or suspending) reads no metrics, so a condition
// about failing reads must not outlive it.
func TestSkippingAutoscalingClearsTheMetricsCondition(t *testing.T) {
	app := freshlyAutoscaledApp("metrics-off", 2)
	app.Spec.Autoscaling.Enabled = false
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionAutoscalingMetricsAvailable, Status: metav1.ConditionFalse,
		Reason: appv1alpha1.AutoscalingMetricsUnavailableReason, LastTransitionTime: metav1.Now(),
	}}
	r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(app).Build()}
	r.desiredReplicas(context.Background(), app, effectiveReplicas(app), releaseObservation{})
	if cond := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionAutoscalingMetricsAvailable); cond != nil {
		t.Fatalf("autoscaling off kept %+v", cond)
	}
}

// w4/m364, Render's "Render ignores this value as long as autoscaling is
// enabled for the service": the first autoscaled pass records the count that
// runs, so a manual scale (spec.replicas) right after autoscaling is turned on
// no longer changes the running count, even before the autoscaler's first
// decision and while metrics fail. Turning autoscaling off runs the stored
// count.
func TestAManualScaleUnderAutoscalingLeavesTheRunningCount(t *testing.T) {
	ctx := context.Background()
	app := freshlyAutoscaledApp("manual-under-autoscaling", 2)
	r := &AppReconciler{
		Client: fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(app).WithStatusSubresource(app).Build(),
		MetricsReader: func(context.Context, string, string) ([]PodUsage, error) {
			return nil, errors.New("metrics unavailable")
		},
	}
	obs := releaseObservation{dep: runningDeployment(app, 2)}

	if got, _, _ := r.desiredReplicas(ctx, app, effectiveReplicas(app), obs); got != 2 {
		t.Fatalf("the first autoscaled pass runs %d, want the 2 that ran", got)
	}
	if got := app.Annotations[annotAutoscaleReplicas]; got != "2" {
		t.Fatalf("recorded %q, want the running 2 seeded", got)
	}

	// The owner scales to 3 while autoscaling is on.
	app.Spec.Replicas = 3
	if got, _, _ := r.desiredReplicas(ctx, app, effectiveReplicas(app), obs); got != 2 {
		t.Fatalf("the manual scale changed the running count to %d, want the autoscaler's 2", got)
	}
	if app.Spec.Replicas != 3 {
		t.Fatalf("spec.replicas = %d, want the stored 3", app.Spec.Replicas)
	}

	app.Spec.Autoscaling.Enabled = false
	if got, _, _ := r.desiredReplicas(ctx, app, effectiveReplicas(app), obs); got != 3 {
		t.Fatalf("autoscaling off runs %d, want the stored 3", got)
	}
	stored := &appv1alpha1.App{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(app), stored); err != nil {
		t.Fatal(err)
	}
	if _, recorded := stored.Annotations[annotAutoscaleReplicas]; recorded {
		t.Fatalf("autoscaling off kept the autoscaler's count: %v", stored.Annotations)
	}
}

// The seed is clamped to the range, and a count the range moves is a scaling
// the autoscaler made, so it records autoscaling_started's transition.
func TestTheSeedIsClampedToTheRangeAndRecordsTheMove(t *testing.T) {
	app := freshlyAutoscaledApp("seed-clamped", 5)
	r := &AppReconciler{
		Client: fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(app).Build(),
		MetricsReader: func(context.Context, string, string) ([]PodUsage, error) {
			return nil, errors.New("metrics unavailable")
		},
	}
	got, _, _ := r.desiredReplicas(context.Background(), app, effectiveReplicas(app), releaseObservation{dep: runningDeployment(app, 5)})
	if got != 3 {
		t.Fatalf("running 5 under max 3 seeds %d, want 3", got)
	}
	if transition := app.Status.Autoscaling; transition == nil || transition.State != appv1alpha1.AutoscalingTransitionStarted ||
		transition.FromReplicas != 5 || transition.ToReplicas != 3 {
		t.Fatalf("transition = %+v, want Started 5→3", transition)
	}
}

// DoD 1 of w4/m364: an idle service turned to autoscaling at 3 instances
// (min 1) holds 3 through the scale-down window, then scales to 1 and records
// the transition the backend reports as autoscaling_started.
func TestAnIdleServiceScalesDownToMinAfterTheWindow(t *testing.T) {
	ctx := context.Background()
	app := freshlyAutoscaledApp("idle-scale-down", 3)
	r := &AppReconciler{
		Client:        fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(app).Build(),
		MetricsReader: func(context.Context, string, string) ([]PodUsage, error) { return threePodsAt(0.001), nil },
	}
	obs := releaseObservation{dep: runningDeployment(app, 3)}

	if got, _, _ := r.desiredReplicas(ctx, app, effectiveReplicas(app), obs); got != 3 {
		t.Fatalf("the first idle reading scaled to %d, want it held for the window", got)
	}
	if !scaleDownPending(app) {
		t.Fatalf("the first idle reading stamped no window: %v", app.Annotations)
	}
	app.Annotations[annotAutoscaleScaleDown] = time.Now().Add(-scaleDownStabilizationWindow - time.Second).UTC().Format(time.RFC3339)
	if err := r.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := r.desiredReplicas(ctx, app, effectiveReplicas(app), obs); got != 1 {
		t.Fatalf("past the window the idle service runs %d, want min 1", got)
	}
	if transition := app.Status.Autoscaling; transition == nil || transition.FromReplicas != 3 || transition.ToReplicas != 1 {
		t.Fatalf("transition = %+v, want Started 3→1", transition)
	}
}

// A seed that lands while a transition is still under way (its record lost)
// keeps that transition, whose end the backend still has to report.
func TestTheSeedKeepsATransitionUnderWay(t *testing.T) {
	app := freshlyAutoscaledApp("seed-mid-transition", 5)
	app.Status.Autoscaling = &appv1alpha1.AutoscalingStatus{
		TransitionID: "to-2", FromReplicas: 1, ToReplicas: 2,
		State: appv1alpha1.AutoscalingTransitionStarted, StartedAt: time.Now().Format(time.RFC3339Nano),
	}
	r := &AppReconciler{
		Client: fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(app).Build(),
		MetricsReader: func(context.Context, string, string) ([]PodUsage, error) {
			return nil, errors.New("metrics unavailable")
		},
	}
	got, _ := r.applyAutoscaling(context.Background(), app, 5)
	if transition := app.Status.Autoscaling; transition.TransitionID != "to-2" {
		t.Fatalf("the seed replaced the transition under way with %+v", transition)
	}
	if got != 2 {
		t.Fatalf("the pass runs %d, want the transition's target 2 held", got)
	}
}
