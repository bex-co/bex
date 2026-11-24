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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/types/tiers"
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
		init.SecurityContext.RunAsNonRoot == nil || !*init.SecurityContext.RunAsNonRoot || init.Resources.Limits.Memory().IsZero() {
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
	if path, port, ok := probeGET(app.ReadinessProbe); !ok || path != "/cgi-bin/health" || port != "8080" {
		t.Errorf("probeGET = %q %q %v, want the exec probe's GET /cgi-bin/health on port 8080", path, port, ok)
	}
}

// Without a path the TCP probe stays; without the operator knowing its image,
// or for a worker, the kubelet's HTTP probe does. Neither of the last two adds
// an init container; a TCP-checked service carries the helper only for its
// drain (w4/m353).
func TestNoRedirectProbeOnlyWhereItApplies(t *testing.T) {
	noPath := project(projectionApp(), execParams())
	for _, probe := range probesOf(appContainerOf(t, noPath)) {
		if probe.TCPSocket == nil || probe.Exec != nil {
			t.Fatalf("no health path: probe = %+v, want the TCP check", probe.ProbeHandler)
		}
	}
	if got := storedHealthProbeImage(noPath.Spec.Template.Spec); got != probeTestImage {
		t.Fatalf("no health path: installer image = %q, want the helper for the drain", got)
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
		"no image": noImage.Spec.Template.Spec,
		"worker":   project(httpHealthApp(), worker).Spec.Template.Spec,
	} {
		if len(spec.InitContainers) != 0 || slices.ContainsFunc(spec.Volumes, func(v corev1.Volume) bool { return v.Name == healthProbeVolumeName }) {
			t.Errorf("%s: pod carries the installer (init %v, volumes %v)", name, spec.InitContainers, spec.Volumes)
		}
	}
}

// Render: "Private services only support default TCP checks"
// (render.com/docs/health-checks). A private service whose stored Health Check
// Path no route answers gets the TCP check on all three probes, with or without
// the helper image; a web service with the same path keeps its HTTP check
// (w8/406).
func TestPrivateServiceHealthPathGetsTheTCPCheck(t *testing.T) {
	missing := func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/missing" }
	private := func(a *appv1alpha1.App) { a.Spec.Type, a.Spec.Expose = appv1alpha1.TypePrivateService, false }
	for name, p := range map[string]deploymentParams{"helper image": execParams(), "no helper image": probeParams()} {
		for i, probe := range probesOf(appContainerOf(t, project(httpHealthApp(missing, private), p))) {
			if probe.TCPSocket == nil || probe.TCPSocket.Port.IntValue() != 8080 || probe.HTTPGet != nil || probe.Exec != nil {
				t.Fatalf("%s: private service probe %d = %+v, want the TCP check on 8080", name, i, probe.ProbeHandler)
			}
		}
		web := appContainerOf(t, project(httpHealthApp(missing), p))
		if path, port, ok := probeGET(web.ReadinessProbe); !ok || path != "/missing" || port != "8080" {
			t.Fatalf("%s: web service readiness = %+v, want its HTTP check of /missing", name, web.ReadinessProbe.ProbeHandler)
		}
	}
}

// A private service projected before w8/406 keeps probing its path until its
// template changes for another reason, so the upgrade rolls none; the next
// roll gives it the TCP check.
func TestPrivateServiceTCPCheckIsAdoptedOnTheNextRollNotOnUpgrade(t *testing.T) {
	private := func(a *appv1alpha1.App) {
		a.Spec.Type, a.Spec.Expose, a.Spec.HealthCheckPath = appv1alpha1.TypePrivateService, false, "/missing"
	}
	for name, p := range map[string]deploymentParams{"helper image": execParams(), "no helper image": probeParams()} {
		app := httpHealthApp(private)
		before := p
		before.keptHealthPath = "/missing" // what an operator before w8/406 projected
		legacy := project(app, before)
		if path, _, ok := probeGET(appContainerOf(t, legacy).ReadinessProbe); !ok || path != "/missing" {
			t.Fatalf("%s: the legacy template does not probe /missing", name)
		}
		upgraded := legacy.DeepCopy()
		applyDeploymentSpec(upgraded, app, p)
		if !equalPodTemplate(upgraded.Spec.Template, legacy.Spec.Template) {
			t.Fatalf("%s: an operator upgrade rolled a private service", name)
		}
		restarted := httpHealthApp(private, func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-11-22T00:00:00Z" })
		next := legacy.DeepCopy()
		applyDeploymentSpec(next, restarted, p)
		if probe := appContainerOf(t, next).ReadinessProbe; probe.TCPSocket == nil {
			t.Fatalf("%s: the next roll must adopt the TCP check; readiness %+v", name, probe.ProbeHandler)
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

// healthProbeInstallCPU is the CPU time the installer spends being started and
// copying the helper, measured on a production tenant node (w6/m547 t011,
// 2026-11-23): containerd's StartContainer took 2.05 s and the copy 0.59 s
// under its 20m limit, 2.64 s at 1/50 of a CPU.
const healthProbeInstallCPU = 53 * time.Millisecond

// healthProbeInstallBound is the most the installer may add to an instance's
// start on a plan a disk can run on. Under Recreate it is downtime, and the
// kubelet's next pass after the installer exits (about 0.7 s, its PLEG relist)
// already uses most of the 1 s a probing disk service may take over one that
// names its port.
const healthProbeInstallBound = 200 * time.Millisecond

// The installer runs at the app container's CPU (w6/170). Its CPU limit is the
// CFS quota runc's init and the copy run under, so at the 20m first shipped
// every pod that probes by exec started 2.6 s later, and a disk service paid
// that as downtime on each deploy and restart. Taking the app container's CPU
// must not move what the pod requests: the larger of its largest init
// container and its containers' sum is what the scheduler places and the
// workspace quota charges, and every container's requests equal its limits
// for Guaranteed QoS.
func TestHealthProbeInstallerStartsAtTheAppContainersCPU(t *testing.T) {
	sides := map[string]func(corev1.ResourceRequirements) corev1.ResourceList{
		"requests": func(r corev1.ResourceRequirements) corev1.ResourceList { return r.Requests },
		"limits":   func(r corev1.ResourceRequirements) corev1.ResourceList { return r.Limits },
	}
	for _, tier := range append(tiers.Compute.IDs(), "") {
		dep := project(httpHealthApp(func(a *appv1alpha1.App) { a.Spec.Tier = tier }), execParams())
		spec := dep.Spec.Template.Spec
		if len(spec.InitContainers) != 1 {
			t.Fatalf("tier %q: init containers = %v, want the installer", tier, spec.InitContainers)
		}
		installer := spec.InitContainers[0].Resources
		for side, of := range sides {
			got, gotSet := of(installer)[corev1.ResourceCPU]
			want, wantSet := of(appContainerOf(t, dep).Resources)[corev1.ResourceCPU]
			if gotSet != wantSet || got.Cmp(want) != 0 {
				t.Errorf("tier %q: installer cpu %s = %q (set %v), want the app container's %q (set %v)", tier, side, got.String(), gotSet, want.String(), wantSet)
			}
		}
		if tier == "" {
			continue // no tier: the LimitRange defaults both containers alike
		}
		for side, of := range sides {
			for kind, quantity := range of(installer) {
				var containers resource.Quantity
				for _, c := range spec.Containers {
					containers.Add(of(c.Resources)[kind])
				}
				if quantity.Cmp(containers) > 0 {
					t.Errorf("tier %q: installer %s %s = %s, above the containers' %s: the pod's %s would grow", tier, kind, side, quantity.String(), containers.String(), side)
				}
			}
		}
		for _, kind := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
			if got, want := installer.Requests[kind], installer.Limits[kind]; got.Cmp(want) != 0 {
				t.Errorf("tier %q: installer %s request %s != limit %s: the pod leaves Guaranteed QoS", tier, kind, got.String(), want.String())
			}
		}
		if tier == tierFree {
			continue // spec.disk requires a paid tier
		}
		cpu := installer.Limits[corev1.ResourceCPU]
		if start := time.Duration(float64(healthProbeInstallCPU) / cpu.AsApproximateFloat64()); start > healthProbeInstallBound {
			t.Errorf("tier %q: at a %s CPU limit the installer takes %v, want at most %v: a disk service is down for it on every deploy", tier, cpu.String(), start.Round(10*time.Millisecond), healthProbeInstallBound)
		}
	}
}

// The upgrade that gives the installer the app container's CPU must not roll
// every service that probes by exec at once, nor Recreate every disk service:
// a template whose installer runs at 20m keeps it until the next roll.
func TestHealthProbeInstallerCPUIsAdoptedOnTheNextRollNotOnUpgrade(t *testing.T) {
	starter := func(a *appv1alpha1.App) { a.Spec.Tier = "starter" }
	disk := func(a *appv1alpha1.App) {
		a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1}
	}
	for name, mutate := range map[string][]func(*appv1alpha1.App){"web service": {starter}, "disk service": {starter, disk}} {
		app := httpHealthApp(mutate...)
		p := execParams()
		stored := project(app, p)
		legacy := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("20m"), corev1.ResourceMemory: resource.MustParse("32Mi")}
		stored.Spec.Template.Spec.InitContainers[0].Resources = corev1.ResourceRequirements{Requests: legacy, Limits: legacy}

		upgraded := stored.DeepCopy()
		applyDeploymentSpec(upgraded, app, p)
		if !equalPodTemplate(upgraded.Spec.Template, stored.Spec.Template) {
			t.Fatalf("%s: an operator upgrade changed an unchanged template (installer %v); that rolls every service at once",
				name, upgraded.Spec.Template.Spec.InitContainers[0].Resources)
		}

		// A new operator image alone keeps both the image and the 20m.
		newer := p
		newer.healthProbeImage = "ghcr.io/bex-co/bex@sha256:2222"
		again := stored.DeepCopy()
		applyDeploymentSpec(again, app, newer)
		if !equalPodTemplate(again.Spec.Template, stored.Spec.Template) {
			t.Fatalf("%s: a new operator image rolled a template whose installer runs at 20m", name)
		}

		next := stored.DeepCopy()
		applyDeploymentSpec(next, httpHealthApp(append(mutate, restartedAt("2026-11-23T00:00:00Z"))...), p)
		if got, want := next.Spec.Template.Spec.InitContainers[0].Resources.Limits[corev1.ResourceCPU], resource.MustParse("500m"); got.Cmp(want) != 0 {
			t.Errorf("%s: the next roll's installer cpu limit = %s, want the Starter instance's %s", name, got.String(), want.String())
		}
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
