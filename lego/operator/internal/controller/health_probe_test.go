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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const probeTestImage = "ghcr.io/bex-co/bex@sha256:1111"

func execParams() deploymentParams {
	p := probeParams()
	p.healthProbeImage = probeTestImage
	return p
}

func execHostOf(c corev1.Container) string {
	_, host, _, _ := healthProbeExecArgs(c.StartupProbe.Exec)
	return host
}

func probesOf(c corev1.Container) []*corev1.Probe {
	return []*corev1.Probe{c.StartupProbe, c.ReadinessProbe, c.LivenessProbe}
}

// A service with a Health Check Path probes by exec of the no-redirect helper
// on all three probes, with the path, port and Host as arguments, an init
// container that installs the helper, and a read-only mount for the app.
func TestHealthPathProjectsTheNoRedirectExecProbe(t *testing.T) {
	dep := project(httpHealthApp(), execParams())
	spec := dep.Spec.Template.Spec
	app := appContainerOf(t, dep)
	for i, probe := range probesOf(app) {
		path, host, port, ok := healthProbeExecArgs(probe.Exec)
		if probe.HTTPGet != nil || !ok || path != "/cgi-bin/health" || port != 8080 || host != "shop-x1."+probeTestBaseDomain {
			t.Fatalf("probe %d = %+v (path %q host %q port %d), want the exec helper with path/host/port", i, probe.ProbeHandler, path, host, port)
		}
		if probe.TimeoutSeconds != healthCheckTimeoutSeconds {
			t.Errorf("probe %d timeout = %d, want %d", i, probe.TimeoutSeconds, healthCheckTimeoutSeconds)
		}
	}
	if len(spec.InitContainers) != 1 || spec.InitContainers[0].Name != healthProbeInitName || spec.InitContainers[0].Image != probeTestImage {
		t.Fatalf("init containers = %+v, want the installer from %s", spec.InitContainers, probeTestImage)
	}
	init := spec.InitContainers[0]
	if !slices.Equal(init.Command, []string{"/health-probe", "install", healthProbeBinary}) {
		t.Errorf("installer command = %v", init.Command)
	}
	if init.SecurityContext == nil || init.SecurityContext.ReadOnlyRootFilesystem == nil || !*init.SecurityContext.ReadOnlyRootFilesystem ||
		init.SecurityContext.RunAsNonRoot == nil || !*init.SecurityContext.RunAsNonRoot || init.Resources.Limits.Cpu().IsZero() {
		t.Errorf("installer is not hardened and bounded: %+v", init)
	}
	if !slices.ContainsFunc(spec.Volumes, func(v corev1.Volume) bool {
		return v.Name == healthProbeVolumeName && v.EmptyDir != nil && v.EmptyDir.SizeLimit != nil
	}) {
		t.Errorf("volumes = %+v, want the bounded %s emptyDir", spec.Volumes, healthProbeVolumeName)
	}
	if !slices.ContainsFunc(app.VolumeMounts, func(m corev1.VolumeMount) bool {
		return m.Name == healthProbeVolumeName && m.MountPath == healthProbeMountPath && m.ReadOnly
	}) {
		t.Errorf("app mounts = %+v, want %s read-only", app.VolumeMounts, healthProbeMountPath)
	}
	if got := probedHealthCheckPath(app); got == nil || *got != "/cgi-bin/health" {
		t.Errorf("probedHealthCheckPath = %v, want the exec probe's path", got)
	}
	if desc := probeTargetDescription(app.ReadinessProbe); desc != "GET /cgi-bin/health on port 8080" {
		t.Errorf("probeTargetDescription = %q", desc)
	}
}

// Without a path the TCP probe stays; without the operator knowing its image,
// or for a worker, the kubelet's HTTP probe does. None adds an init container.
func TestNoRedirectProbeOnlyWhereItApplies(t *testing.T) {
	noPath := project(projectionApp(), execParams())
	for _, probe := range probesOf(appContainerOf(t, noPath)) {
		if probe.TCPSocket == nil || probe.Exec != nil {
			t.Fatalf("no health path: probe = %+v, want the TCP check", probe.ProbeHandler)
		}
	}
	noImage := project(httpHealthApp(), probeParams())
	for _, probe := range probesOf(appContainerOf(t, noImage)) {
		if probe.HTTPGet == nil {
			t.Fatalf("no helper image: probe = %+v, want the kubelet HTTP check", probe.ProbeHandler)
		}
	}
	worker := execParams()
	worker.worker = true
	for name, spec := range map[string]corev1.PodSpec{
		"no path":  noPath.Spec.Template.Spec,
		"no image": noImage.Spec.Template.Spec,
		"worker":   project(httpHealthApp(), worker).Spec.Template.Spec,
	} {
		if len(spec.InitContainers) != 0 || slices.ContainsFunc(spec.Volumes, func(v corev1.Volume) bool { return v.Name == healthProbeVolumeName }) {
			t.Errorf("%s: pod carries the installer (init %v, volumes %v)", name, spec.InitContainers, spec.Volumes)
		}
	}
}

// A running release keeps its kubelet HTTP probe, and the helper image it was
// installed from, until its template changes for another reason: an operator
// upgrade must not roll the fleet.
func TestNoRedirectProbeIsAdoptedOnTheNextRollNotOnUpgrade(t *testing.T) {
	app := httpHealthApp()
	legacy := project(app, probeParams()) // what a pre-m133 operator stored
	upgraded := legacy.DeepCopy()
	applyDeploymentSpec(upgraded, app, execParams())
	if !equalPodTemplate(upgraded.Spec.Template, legacy.Spec.Template) {
		t.Fatal("an operator upgrade changed an unchanged service's template; that would roll every service at once")
	}

	restarted := httpHealthApp(func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-10-23T00:00:00Z" })
	next := legacy.DeepCopy()
	applyDeploymentSpec(next, restarted, execParams())
	if probe := appContainerOf(t, next).ReadinessProbe; probe.Exec == nil || len(next.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("the next roll must adopt the exec probe; readiness %+v", probe.ProbeHandler)
	}

	// A new operator image alone keeps the installed helper's image.
	installed := project(app, execParams())
	newer := execParams()
	newer.healthProbeImage = "ghcr.io/bex-co/bex@sha256:2222"
	again := installed.DeepCopy()
	applyDeploymentSpec(again, app, newer)
	if !equalPodTemplate(again.Spec.Template, installed.Spec.Template) {
		t.Fatalf("a new operator image rolled the pods (installer %q)", storedHealthProbeImage(again.Spec.Template.Spec))
	}
	roll := installed.DeepCopy()
	applyDeploymentSpec(roll, restarted, newer)
	if got := storedHealthProbeImage(roll.Spec.Template.Spec); got != newer.healthProbeImage {
		t.Fatalf("the next roll installer image = %q, want %q", got, newer.healthProbeImage)
	}
}

// The release's Host survives a re-projection of the exec form too.
func TestExecProbeHostIsFrozenForTheRelease(t *testing.T) {
	release := func(gen int64, mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
		return httpHealthApp(append([]func(*appv1alpha1.App){func(a *appv1alpha1.App) { a.Status.ReleaseGeneration = gen }}, mutate...)...)
	}
	p := execParams()
	live := project(release(4), p)
	edited := live.DeepCopy()
	applyDeploymentSpec(edited, release(4, func(a *appv1alpha1.App) { a.Spec.Host = "shop.example.com" }), p)
	if !equalPodTemplate(edited.Spec.Template, live.Spec.Template) {
		t.Errorf("a domain edit rolled the pods (probe Host %q)", execHostOf(appContainerOf(t, edited)))
	}
	next := live.DeepCopy()
	applyDeploymentSpec(next, release(5, func(a *appv1alpha1.App) { a.Spec.Host = "shop.example.com" }), p)
	if got := execHostOf(appContainerOf(t, next)); got != "shop.example.com" {
		t.Errorf("new release probe Host = %q", got)
	}
}

func TestHealthProbeExecArgsRoundTrip(t *testing.T) {
	for _, tc := range []struct{ path, host string }{
		{"/hc", ""}, {"/hc?echo_code=302&echo_header=Location:/x", "app.onbex.co"}, {"/a b", "x.example"},
	} {
		got, host, port, ok := healthProbeExecArgs(healthProbeExecHandler(tc.path, 80, tc.host).Exec)
		if !ok || got != tc.path || host != tc.host || port != 80 {
			t.Errorf("round trip %+v = %q %q %d %v", tc, got, host, port, ok)
		}
	}
	if _, _, _, ok := healthProbeExecArgs(&corev1.ExecAction{Command: []string{"/bin/true"}}); ok {
		t.Error("a foreign exec probe was read as the health helper")
	}
	if _, _, _, ok := healthProbeExecArgs(nil); ok {
		t.Error("a nil exec probe was read as the health helper")
	}
	if !strings.HasPrefix(healthProbeBinary, healthProbeMountPath+"/") {
		t.Error("the helper must live under its mount")
	}
}
