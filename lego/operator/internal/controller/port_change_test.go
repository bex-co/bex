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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPortChangeMovesEverySurfaceTogether is w4/m121's blast-radius assertion.
//
// The port became editable after create (it had been create-only on every
// surface), so the thing that matters is that ONE number still drives all four
// places it appears — the injected PORT, the container port, the health probe,
// and release identity. If any of them kept the old value, a port change would
// leave the container listening on one port while the platform targeted
// another: a service that reports Live and answers nothing, which is strictly
// worse than the create-only state this replaced.
//
// The operator recomputes each of these from spec.EffectivePort() on every
// reconcile, so this pins that they cannot drift apart rather than that any one
// of them is correct.
func TestPortChangeMovesEverySurfaceTogether(t *testing.T) {
	const newPort = 8081

	app := projectionApp(func(a *appv1alpha1.App) {
		a.Spec.Port = newPort
		a.Spec.HealthCheckPath = "/healthz"
	})
	// deploymentParams carries the port the controller derived from the spec;
	// EffectivePort is that derivation, so reading it here is the same number
	// app_controller.go passes in.
	p := webParams()
	p.port = int(app.Spec.EffectivePort())

	container := appContainerOf(t, project(app, p))

	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != newPort {
		t.Fatalf("container ports = %v, want the new port %d", container.Ports, newPort)
	}
	if container.ReadinessProbe == nil || container.ReadinessProbe.HTTPGet == nil {
		t.Fatal("a health-checked web service must keep its HTTP probe")
	}
	if got := container.ReadinessProbe.HTTPGet.Port.IntValue(); got != newPort {
		t.Errorf("readiness probe port = %d, want %d — a probe left on the old port fails a healthy service", got, newPort)
	}
	var injected string
	for _, e := range container.Env {
		if e.Name == "PORT" {
			injected = e.Value
		}
	}
	if injected != "8081" {
		t.Errorf("injected PORT = %q, want 8081 — the container would bind the old port", injected)
	}

	// Release identity must change, or the operator reuses the running release
	// and the pods never roll onto the new port.
	before := projectionApp(func(a *appv1alpha1.App) {
		a.Spec.Port = 8080
		a.Spec.HealthCheckPath = "/healthz"
	})
	if desiredAppReleaseIdentity(before.Spec).release == desiredAppReleaseIdentity(app.Spec).release {
		t.Error("a port change must change release identity, or the rollout is skipped and the pods keep the old PORT")
	}
}

// TestPortIsMeaninglessForTheTypesWithoutAListener pins the other half: the
// types the new update verb refuses are the types whose projections have no
// port to move. A worker or cron job runs no server, and a static site is
// served by the shared static-server on its own port.
func TestPortIsMeaninglessForTheTypesWithoutAListener(t *testing.T) {
	for _, serviceType := range []string{
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
		appv1alpha1.TypeStaticSite,
	} {
		t.Run(serviceType, func(t *testing.T) {
			spec := appv1alpha1.AppSpec{Type: serviceType, Port: 8081}
			if spec.InternallyAddressable() {
				t.Fatalf("%s must not be internally addressable — the port verb gates on this", serviceType)
			}
			if addr := spec.InternalAddress("svc"); addr != "" {
				t.Errorf("%s reported an internal address %q; it has no listening port to publish", serviceType, addr)
			}
		})
	}

	// And the two that DO have one stay addressable with an explicit port.
	for _, serviceType := range []string{"", appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService} {
		spec := appv1alpha1.AppSpec{Type: serviceType, Port: 8081}
		if !spec.InternallyAddressable() {
			t.Errorf("service type %q must stay addressable", serviceType)
		}
	}
}
