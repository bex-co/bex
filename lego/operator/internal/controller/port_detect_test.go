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
	"strconv"
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

func detectFixture(t *testing.T, started time.Duration, scanner portscan.Scanner, mutate func(*appv1alpha1.App, *corev1.Pod)) (*AppReconciler, *appv1alpha1.App, *appsv1.Deployment) {
	t.Helper()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"},
		Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/example/app:1", Port: appv1alpha1.DefaultPort},
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
		"an explicit non-default port":                 {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Spec.Port = 8080 }},
		"a default PORT variable":                      {open: []int{8000}, started: time.Minute, mutate: func(a *appv1alpha1.App, _ *corev1.Pod) { a.Status.EnvPort = appv1alpha1.DefaultPort }},
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

// w8/259: an instance with nothing listening where the platform can reach it
// (no server, or one bound to 127.0.0.1) is told so in Render's terms, whether
// the scan came back empty or unreachable. On a tenant pod a closed port drops
// the connect, so both are the same case; the stall text used to blame port
// detection for the second.
func TestDetectBoundPortNamesAnInstanceWithNoOpenPort(t *testing.T) {
	for name, scanner := range map[string]*cannedScanner{
		"the scan found nothing":   {},
		"the scan was unreachable": {err: portscan.ErrUnreachable},
	} {
		r, app, dep := detectFixture(t, time.Minute, scanner, nil)
		got := r.detectBoundPort(t.Context(), app, dep, 10000)
		for _, want := range []string{"No open port was detected", "0.0.0.0", "PORT (10000)", "background worker"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: stall text = %q, want %q in it", name, got, want)
			}
		}
		if strings.Contains(got, "could not reach the instance") {
			t.Errorf("%s: stall text still blames port detection: %q", name, got)
		}
		if app.Status.DetectedPort != 0 {
			t.Errorf("%s: DetectedPort = %d, want none", name, app.Status.DetectedPort)
		}
	}
}

func TestDetectBoundPortWithoutAPersistedDefault(t *testing.T) {
	r, app, dep := detectFixture(t, time.Minute, &cannedScanner{open: []int{3000}}, func(a *appv1alpha1.App, _ *corev1.Pod) { a.Spec.Port = 0 })
	r.detectBoundPort(t.Context(), app, dep, 10000)
	r.detectBoundPort(t.Context(), app, dep, 10000)
	if app.Status.DetectedPort != 3000 || app.ServingPort() != 3000 {
		t.Fatalf("detected=%d serving=%d", app.Status.DetectedPort, app.ServingPort())
	}
}

// w8/267: a detected port decides where the platform routes, never what PORT
// the process is given. Render leaves PORT at its default; bex handed the
// detected port back as PORT, so the instance was restarted with a different
// environment than the one it was found in, and an app with a public listener
// on 8080 and an internal one on $PORT then bound 8080 twice and crash-looped.
func TestADetectedPortRoutesButIsNotGivenBackAsPort(t *testing.T) {
	portOf := func(app *appv1alpha1.App, serving int) string {
		env := appEnv(app, serving)
		last := env[len(env)-1]
		if last.Name != portEnvName {
			t.Fatalf("PORT is not last: %+v", env)
		}
		return last.Value
	}
	detected := &appv1alpha1.App{Status: appv1alpha1.AppStatus{DetectedPort: 8080}}
	if got := portOf(detected, 8080); got != "10000" {
		t.Errorf("PORT for a detected port = %s, want the default 10000", got)
	}
	if hint := crashHintPort(detected, 8080); hint != 0 {
		t.Errorf("crash hint port = %d, want none: the process is not expected on $PORT", hint)
	}
	if msg := crashLoopMessage(corev1.ContainerStatus{}, crashHintPort(detected, 8080)); strings.Contains(msg, "$PORT") {
		t.Errorf("the crash message still says the process must listen on $PORT: %q", msg)
	}

	// A port the service names is both routed and handed to the process.
	for name, tc := range map[string]struct {
		app     *appv1alpha1.App
		serving int
	}{
		"an explicit port":          {&appv1alpha1.App{Spec: appv1alpha1.AppSpec{Port: 3000}}, 3000},
		"the service's own PORT":    {&appv1alpha1.App{Status: appv1alpha1.AppStatus{EnvPort: 4000}}, 4000},
		"the default, undetected":   {&appv1alpha1.App{}, 10000},
		"a stale detection, unused": {&appv1alpha1.App{Spec: appv1alpha1.AppSpec{Port: 3000}, Status: appv1alpha1.AppStatus{DetectedPort: 8080}}, 3000},
	} {
		if got := portOf(tc.app, tc.serving); got != strconv.Itoa(tc.serving) {
			t.Errorf("%s: PORT = %s, want %d", name, got, tc.serving)
		}
		if hint := crashHintPort(tc.app, tc.serving); hint != tc.serving {
			t.Errorf("%s: crash hint port = %d, want %d", name, hint, tc.serving)
		}
	}
}
