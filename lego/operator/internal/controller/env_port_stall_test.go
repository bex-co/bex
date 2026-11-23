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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/416: a web service that sets PORT is scanned on that port alone, as on
// Render, whose staff answer that setting PORT disables the port detector
// (community.render.com/t/15678). With PORT=8080 and the process listening on
// another port, the stall names PORT and Render's fix, reads as a scan of
// port 8080, and the other listener is never read or adopted. The same
// process with PORT unset is detected on its listener.
func TestAClosedPORTIsAScanOfThatPortAlone(t *testing.T) {
	listener := httpListenerPort(t)
	fixture := func(port int, envPort int32) (*appv1alpha1.App, func() string, *cannedScanner) {
		probe := &corev1.Probe{ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(port)}},
			PeriodSeconds: 10, TimeoutSeconds: 5}
		scanner := &cannedScanner{open: []int{listener}}
		r, app, dep := detectFixture(t, time.Minute, scanner, func(a *appv1alpha1.App, p *corev1.Pod) {
			a.Status.EnvPort = envPort
			p.Status.PodIP = "127.0.0.1"
			p.Spec.Containers[0].StartupProbe, p.Spec.Containers[0].ReadinessProbe = probe, probe
		})
		return app, func() string {
			_, msg := r.rolloutStallDiagnosis(t.Context(), app, dep, port)
			time.Sleep(time.Millisecond)
			return msg
		}, scanner
	}

	app, stall, scanner := fixture(8080, 8080)
	want := appv1alpha1.ProbeStallReason("startup", true, "", "8080", 0) +
		" Nothing accepted a TCP connection on port 8080, set by the service's PORT environment variable." +
		" Bind the server to 0.0.0.0 on port 8080, or update the PORT environment variable to the port it listens on."
	for pass := range 3 {
		got := stall()
		if got != want {
			t.Fatalf("pass %d: stall = %q, want %q", pass, got, want)
		}
		if scan, found := appv1alpha1.ReadPortScan(got); scan != appv1alpha1.PortScanEnvPort || found.EnvPort() != 8080 {
			t.Fatalf("pass %d: ReadPortScan = (%d, %q), want a scan of PORT 8080", pass, scan, found)
		}
	}
	if scanner.calls != 0 || app.Status.DetectedPort != 0 {
		t.Fatalf("PORT set: %d scans, detected port %d; want none, as Render routes only to PORT", scanner.calls, app.Status.DetectedPort)
	}

	// The control: PORT unset, the same listener is found and adopted.
	app, stall, _ = fixture(appv1alpha1.DefaultPort, 0)
	for range 2 {
		stall()
	}
	if app.Status.DetectedPort != int32(listener) {
		t.Fatalf("PORT unset: detected port %d, want the listener %d", app.Status.DetectedPort, listener)
	}

	// A private service and a Health Check Path get no PORT hint: Render
	// documents PORT for web services, and a path check is about the path.
	for name, mutate := range map[string]func(*appv1alpha1.App){
		"private service": func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypePrivateService },
		"health check":    func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/healthz" },
		"another port":    func(a *appv1alpha1.App) { a.Status.EnvPort = 9090 },
	} {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Port, a.Status.EnvPort = 10000, 8080 })
		mutate(app)
		if got := envPortStallHint(app, 8080); got != "" {
			t.Errorf("%s: hint %q, want none", name, got)
		}
	}
}
