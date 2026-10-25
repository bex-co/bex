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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/portscan"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type cannedScanner struct {
	open  []int
	err   error
	calls int
}

func (c *cannedScanner) Scan(context.Context, string) ([]int, error) {
	c.calls++
	return c.open, c.err
}

func detectFixture(t *testing.T, started time.Duration, scanner *cannedScanner, mutate func(*appv1alpha1.App, *corev1.Pod)) (*AppReconciler, *appv1alpha1.App, *appsv1.Deployment) {
	t.Helper()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"},
		Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/example/app:1"},
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: "web"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelApp: "web", labelRevision: "rev-1"}}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "tea-a", Labels: map[string]string{labelApp: "web", labelRevision: "rev-1"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning, PodIP: "10.0.0.7",
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: appContainerName, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(time.Now().Add(-started))}},
			}},
		},
	}
	if mutate != nil {
		mutate(app, pod)
	}
	r := &AppReconciler{Client: fake.NewClientBuilder().WithObjects(pod).Build(), PortScanner: scanner, portDetectEvery: time.Nanosecond}
	return r, app, dep
}

// w8/177: a web service whose check keeps failing on the default port and whose
// process listens on exactly one other port moves to that port once two scans
// agree; the serving port then follows it.
func TestDetectBoundPortAdoptsALoneListener(t *testing.T) {
	scanner := &cannedScanner{open: []int{8000}}
	r, app, dep := detectFixture(t, time.Minute, scanner, nil)

	if hint := r.detectBoundPort(context.Background(), app, dep, 10000); app.Status.DetectedPort != 0 || !strings.Contains(hint, "8000") {
		t.Fatalf("first scan: DetectedPort=%d hint=%q, want only a candidate", app.Status.DetectedPort, hint)
	}
	time.Sleep(time.Millisecond)
	hint := r.detectBoundPort(context.Background(), app, dep, 10000)
	if app.Status.DetectedPort != 8000 || !strings.Contains(hint, "now uses") {
		t.Fatalf("second scan: DetectedPort=%d hint=%q, want 8000 adopted", app.Status.DetectedPort, hint)
	}
	if got := detectedServingPort(app, 10000); got != 8000 {
		t.Fatalf("serving port = %d, want the detected 8000", got)
	}
	if got := app.ServingPort(); got != 8000 {
		t.Fatalf("App.ServingPort = %d, want 8000", got)
	}
}

func TestDetectBoundPortLeavesAmbiguousOrConfiguredServicesAlone(t *testing.T) {
	for name, tc := range map[string]struct {
		open    []int
		mutate  func(*appv1alpha1.App, *corev1.Pod)
		started time.Duration
	}{
		"two listeners are named, not chosen":          {open: []int{8000, 9090}, started: time.Minute},
		"the configured port answers (a path problem)": {open: []int{10000, 8000}, started: time.Minute},
		"nothing answers":                              {open: nil, started: time.Minute},
		"an explicit port":                             {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Spec.Port = 3000 }},
		"a PORT variable":                              {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Status.EnvPort = 3000 }},
		"a worker":                                     {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Spec.Type = appv1alpha1.TypeBackgroundWorker }},
		"a booting pod":                                {open: []int{8000}, started: time.Second},
		"a cached service": {open: []int{8000}, started: time.Minute, mutate: func(_ *appv1alpha1.App, p *corev1.Pod) {
			p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: edgeCacheContainerName})
		}},
	} {
		t.Run(name, func(t *testing.T) {
			scanner := &cannedScanner{open: tc.open}
			r, app, dep := detectFixture(t, tc.started, scanner, tc.mutate)
			for range 3 {
				r.detectBoundPort(context.Background(), app, dep, 10000)
				time.Sleep(time.Millisecond)
			}
			if app.Status.DetectedPort != 0 {
				t.Fatalf("DetectedPort = %d, want none", app.Status.DetectedPort)
			}
		})
	}
}

// An explicit port or PORT variable wins over, and clears, a stale detection.
func TestExplicitPortClearsADetectedOne(t *testing.T) {
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Port: 3000}, Status: appv1alpha1.AppStatus{DetectedPort: 8000}}
	if got := detectedServingPort(app, 3000); got != 3000 || app.Status.DetectedPort != 0 {
		t.Fatalf("serving port %d, DetectedPort %d; want 3000 and cleared", got, app.Status.DetectedPort)
	}
}

// w4/393: a failed scan names itself in the stall text instead of reading
// like "no listener found".
func TestDetectBoundPortSurfacesScanFailure(t *testing.T) {
	r, app, dep := detectFixture(t, time.Minute, &cannedScanner{err: portscan.ErrUnreachable}, nil)
	if got := r.detectBoundPort(t.Context(), app, dep, 10000); !strings.Contains(got, "could not reach the instance") {
		t.Fatalf("stall text = %q, want the scan failure named", got)
	}
}
