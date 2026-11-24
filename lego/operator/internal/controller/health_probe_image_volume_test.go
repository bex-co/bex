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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// imageVolumeParams is execParams on an operator with
// BEX_HEALTH_PROBE_IMAGE_VOLUME on.
func imageVolumeParams() deploymentParams {
	p := execParams()
	p.healthProbeImageVolume = true
	return p
}

// helperForm is how dep's pods get the health-probe helper: "image volume",
// "installer", or "none". It fails t on a template that mixes the two or
// mounts the helper anywhere the probes do not exec it from.
func helperForm(t *testing.T, dep *appsv1.Deployment) string {
	t.Helper()
	spec := dep.Spec.Template.Spec
	var volumes []corev1.Volume
	for _, v := range spec.Volumes {
		if v.Name == healthProbeVolumeName {
			volumes = append(volumes, v)
		}
	}
	var mounts []corev1.VolumeMount
	for _, m := range appContainerOf(t, dep).VolumeMounts {
		if m.Name == healthProbeVolumeName {
			mounts = append(mounts, m)
		}
	}
	switch {
	case len(volumes) == 0 && len(mounts) == 0 && len(spec.InitContainers) == 0:
		return "none"
	case len(volumes) != 1 || len(mounts) != 1:
		t.Fatalf("helper volumes %+v, app mounts %+v: want exactly one of each", volumes, mounts)
	}
	volume, mount := volumes[0], mounts[0]
	if image := volume.Image; image != nil {
		want := corev1.VolumeMount{Name: healthProbeVolumeName, MountPath: healthProbeMountPath, SubPath: "bex-health", ReadOnly: true}
		if volume.EmptyDir != nil || len(spec.InitContainers) != 0 || image.PullPolicy != corev1.PullIfNotPresent || !apiequality.Semantic.DeepEqual(mount, want) {
			t.Fatalf("image-volume helper: volume %+v, mount %+v, init %+v; want only bex's image, pulled if absent, its bex-health directory read-only at %s",
				volume, mount, spec.InitContainers, healthProbeMountPath)
		}
		return "image volume"
	}
	installer := storedHealthProbeInstaller(spec)
	if volume.EmptyDir == nil || installer == nil || len(spec.InitContainers) != 1 ||
		!apiequality.Semantic.DeepEqual(mount, corev1.VolumeMount{Name: healthProbeVolumeName, MountPath: healthProbeMountPath, ReadOnly: true}) {
		t.Fatalf("installed helper: volume %+v, mount %+v, init %+v; want the emptyDir the installer fills, read-only at %s",
			volume, mount, spec.InitContainers, healthProbeMountPath)
	}
	return "installer"
}

// execsTheHelper reports a probe or preStop command that runs the helper.
func execsTheHelper(exec *corev1.ExecAction) bool {
	return exec != nil && len(exec.Command) > 0 && exec.Command[0] == healthProbeBinary
}

// Every pod that carries the helper (w6/m624) mounts it out of bex's image
// when the setting is on: a Health Check Path's no-redirect check, a first
// web image release's any-open-port check, and a TCP-checked service's
// health-checked drain. None has an init container, so the kubelet starts the
// app in the pass that starts the pod. With the setting off each keeps the
// installer, and a worker carries no helper either way.
func TestHealthProbeImageVolumeReplacesTheInstaller(t *testing.T) {
	firstWebImage := func() *appv1alpha1.App {
		app := webImageApp(1)
		app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "img@sha256:1", Revision: releaseRevision(app), PrimaryPort: 80, Ports: []int32{80}, ProcessPort: 10000}
		return app
	}
	cases := []struct {
		name  string
		app   *appv1alpha1.App
		port  int
		check func(corev1.Container) bool
	}{
		{"health check path", httpHealthApp(), 8080, func(c corev1.Container) bool {
			_, _, _, ok := healthProbeExecArgs(c.StartupProbe.Exec)
			return ok
		}},
		{"any open port", firstWebImage(), 80, func(c corev1.Container) bool { return healthProbeChecksOpenPort(c.StartupProbe.Exec) }},
		{"drain only", projectionApp(), 8080, func(c corev1.Container) bool {
			return c.StartupProbe.TCPSocket != nil && healthProbeDrainSeconds(c.Lifecycle.PreStop.Exec) > 0
		}},
	}
	for _, tc := range cases {
		for want, on := range map[string]bool{"image volume": true, "installer": false} {
			p := execParams()
			p.port, p.healthProbeImageVolume = tc.port, on
			dep := project(tc.app, p)
			if got := helperForm(t, dep); got != want {
				t.Fatalf("%s, setting %v: helper form = %s, want %s", tc.name, on, got, want)
			}
			if got := storedHealthProbeImage(dep.Spec.Template.Spec); got != probeTestImage {
				t.Errorf("%s, setting %v: helper image = %q, want bex's %q", tc.name, on, got, probeTestImage)
			}
			app := appContainerOf(t, dep)
			if !tc.check(app) {
				t.Errorf("%s, setting %v: startup %+v, preStop %+v: not the helper's check", tc.name, on, app.StartupProbe.ProbeHandler, app.Lifecycle)
			}
			for _, exec := range []*corev1.ExecAction{app.StartupProbe.Exec, app.ReadinessProbe.Exec, app.LivenessProbe.Exec, app.Lifecycle.PreStop.Exec} {
				if exec != nil && !execsTheHelper(exec) {
					t.Errorf("%s, setting %v: %v does not run the mounted helper", tc.name, on, exec.Command)
				}
			}
		}
	}
	worker := imageVolumeParams()
	worker.worker = true
	if got := helperForm(t, project(httpHealthApp(), worker)); got != "none" {
		t.Errorf("worker: helper form = %s, want none", got)
	}
	if got := helperForm(t, project(httpHealthApp(), probeParams())); got != "none" {
		t.Errorf("operator without its own image: helper form = %s, want none", got)
	}
}

// Turning the setting on or off must not roll every service that carries the
// helper, nor Recreate every disk service: a stored template keeps its own
// form, and the image it took the helper from, until a deploy, restart or
// settings change rolls it. That roll adopts the current form.
func TestHealthProbeImageVolumeIsAdoptedOnTheNextRollNotOnUpgrade(t *testing.T) {
	starter := func(a *appv1alpha1.App) { a.Spec.Tier = "starter" }
	disk := func(a *appv1alpha1.App) {
		a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1}
	}
	form := map[bool]string{true: "image volume", false: "installer"}
	for name, mutate := range map[string][]func(*appv1alpha1.App){
		"web service":     {starter},
		"disk service":    {starter, disk},
		"TCP-check drain": {starter, func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "" }},
	} {
		for _, on := range []bool{true, false} {
			app := httpHealthApp(mutate...)
			before, after := execParams(), execParams()
			before.healthProbeImageVolume, after.healthProbeImageVolume = !on, on
			stored := project(app, before)

			upgraded := stored.DeepCopy()
			applyDeploymentSpec(upgraded, app, after)
			if !equalPodTemplate(upgraded.Spec.Template, stored.Spec.Template) {
				t.Fatalf("%s: setting the image volume %v changed an unchanged template (now %s); that rolls every such service at once",
					name, on, helperForm(t, upgraded))
			}

			// A new operator image alone keeps the stored form and image too.
			newer := after
			newer.healthProbeImage = "ghcr.io/bex-co/bex@sha256:2222"
			again := stored.DeepCopy()
			applyDeploymentSpec(again, app, newer)
			if !equalPodTemplate(again.Spec.Template, stored.Spec.Template) {
				t.Fatalf("%s: a new operator image with the setting %v rolled the template (now %s from %q)",
					name, on, helperForm(t, again), storedHealthProbeImage(again.Spec.Template.Spec))
			}

			next := stored.DeepCopy()
			applyDeploymentSpec(next, httpHealthApp(append(mutate, restartedAt("2026-11-24T00:00:00Z"))...), newer)
			if got := helperForm(t, next); got != form[on] {
				t.Errorf("%s: the next roll with the setting %v gives the %s, want the %s", name, on, got, form[on])
			}
			if got := storedHealthProbeImage(next.Spec.Template.Spec); got != newer.healthProbeImage {
				t.Errorf("%s: the next roll takes the helper from %q, want the new operator image %q", name, got, newer.healthProbeImage)
			}
		}
	}
}

// A disk service's code deploy pre-pulls every image its new instance needs
// on the volume's node before Recreate stops the old one (w6/m595). The
// helper's image volume is pulled like a container's image, so a deploy that
// also picks up a new operator image pulls it there first, as the installer's
// image was, and with the volume's own pull policy.
func TestDiskPrePullPullsTheHelpersImageVolume(t *testing.T) {
	disk := func(a *appv1alpha1.App) {
		a.Spec.Tier = "starter"
		a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1}
	}
	stored := project(httpHealthApp(disk), imageVolumeParams())
	newer := imageVolumeParams()
	newer.image, newer.healthProbeImage = "zot.local:5000/hello:rev-2", "ghcr.io/bex-co/bex@sha256:2222"
	projected := project(httpHealthApp(disk, restartedAt("2026-11-24T00:00:00Z")), newer)
	if got := helperForm(t, projected); got != "image volume" {
		t.Fatalf("projected helper form = %s, want the image volume", got)
	}
	images := newTemplateImages(&stored.Spec.Template.Spec, &projected.Spec.Template.Spec)
	if want := []string{newer.healthProbeImage, newer.image}; !slices.Equal(images, want) {
		t.Fatalf("pre-pull images = %v, want %v: the new instance would pull the helper's image inside its Recreate window", images, want)
	}
	if got := prePullPolicy(&projected.Spec.Template, newer.healthProbeImage); got != corev1.PullIfNotPresent {
		t.Errorf("helper image pre-pull policy = %s, want the volume's IfNotPresent", got)
	}
	if got := prePullPolicy(&projected.Spec.Template, newer.image); got != corev1.PullAlways {
		t.Errorf("app image pre-pull policy = %s, want Always", got)
	}
}
