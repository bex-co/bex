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
	"slices"
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m350: a first web image release discovering its HTTP port checks any open
// port with the helper, pins no container port, and keeps that template when
// discovery moves the primary and when the release goes live. A release over
// a serving one, a service with a Health Check Path or an edge cache, and an
// operator without its own image keep the port-bound probe.
func TestFirstWebImageReleaseChecksAnyOpenPort(t *testing.T) {
	first := func() *appv1alpha1.App {
		app := webImageApp(1)
		app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "img@sha256:1", Revision: releaseRevision(app), PrimaryPort: 80, Ports: []int32{80}, ProcessPort: 10000}
		return app
	}
	params := func(port int) deploymentParams {
		p := execParams()
		p.port = port
		return p
	}
	app := first()
	dep := project(app, params(80))
	c := appContainerOf(t, dep)
	if len(c.Ports) != 0 {
		t.Fatalf("container ports = %+v, want none while discovery may move the primary", c.Ports)
	}
	for _, probe := range probesOf(c) {
		if !healthProbeChecksOpenPort(probe.Exec) || !slices.Contains(probe.Exec.Command, "18012,18013,18014,19099") {
			t.Fatalf("probe = %+v, want the any-open-port helper skipping bex's reserved ports", probe.ProbeHandler)
		}
	}
	if got := probedHealthCheckPath(c); got == nil || *got != "" {
		t.Errorf("probedHealthCheckPath = %v, want the default check's empty path", got)
	}
	if len(dep.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("init containers = %+v, want the helper installer", dep.Spec.Template.Spec.InitContainers)
	}
	stored := dep.Spec.Template.DeepCopy()

	// Discovery picks 10000, then the release goes live: one template.
	network := app.Status.ImageNetwork
	network.PrimaryPort, network.Ports, network.HTTPSelection = 10000, []int32{80, 10000}, appv1alpha1.HTTPSelectionDetected
	applyDeploymentSpec(dep, app, params(10000))
	app.Status.ActiveRevision, app.Status.ServingNetwork = releaseRevision(app), network.DeepCopy()
	applyDeploymentSpec(dep, app, params(10000))
	if !apiequality.Semantic.DeepEqual(dep.Spec.Template, *stored) {
		t.Fatalf("the first release's template changed:\n%+v\nwant\n%+v", appContainerOf(t, dep), stored.Spec.Containers[0])
	}
	// An operator upgrade keeps the helper image the release was installed from.
	upgraded := params(10000)
	upgraded.healthProbeImage = "ghcr.io/bex-co/bex@sha256:2222"
	applyDeploymentSpec(dep, app, upgraded)
	if !apiequality.Semantic.DeepEqual(dep.Spec.Template, *stored) {
		t.Fatal("an operator upgrade rolled the served first release")
	}

	// The next release has a known port: the TCP probe on it, as before.
	next := app.DeepCopy()
	next.Status.ReleaseGeneration = 2
	next.Status.ImageNetwork.Revision = releaseRevision(next)
	for _, probe := range probesOf(appContainerOf(t, project(next, params(10000)))) {
		if probe.TCPSocket == nil || probe.TCPSocket.Port.IntValue() != 10000 {
			t.Fatalf("next release probe = %+v, want TCP on 10000", probe.ProbeHandler)
		}
	}

	for name, mutate := range map[string]func(*appv1alpha1.App, *deploymentParams){
		"a release over a serving one": func(a *appv1alpha1.App, _ *deploymentParams) { a.Status.ActiveRevision = "rev-0" },
		"a Health Check Path":          func(a *appv1alpha1.App, _ *deploymentParams) { a.Spec.HealthCheckPath = "/health" },
		"an edge cache":                func(_ *appv1alpha1.App, p *deploymentParams) { p.edgeCacheImage = probeTestImage },
		"no helper image":              func(_ *appv1alpha1.App, p *deploymentParams) { p.healthProbeImage = "" },
		"the service's own PORT":       func(a *appv1alpha1.App, _ *deploymentParams) { a.Status.EnvPort = 8080 },
	} {
		app, p := first(), params(80)
		mutate(app, &p)
		for _, probe := range probesOf(project(app, p).Spec.Template.Spec.Containers[0]) {
			if healthProbeChecksOpenPort(probe.Exec) {
				t.Errorf("%s: probe checks any open port, want the port-bound check", name)
			}
		}
	}
}
