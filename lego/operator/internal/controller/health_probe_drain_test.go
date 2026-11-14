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
	"strconv"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m353: the 60 s drain applies only to an instance that still serves. The
// preStop runs the instance's own liveness check through the helper and waits
// only when it passes, so a liveness restart lands at Render's 60 s instead of
// 60 s plus the drain.

// drainCheckArgs is the check a health-checked preStop drain runs, failing
// unless the preStop is the helper's drain of drainSeconds.
func drainCheckArgs(t *testing.T, c corev1.Container) []string {
	t.Helper()
	if c.Lifecycle == nil || c.Lifecycle.PreStop == nil || c.Lifecycle.PreStop.Exec == nil {
		t.Fatalf("lifecycle = %+v, want the health-checked drain", c.Lifecycle)
	}
	command := c.Lifecycle.PreStop.Exec.Command
	prefix := []string{healthProbeBinary, "drain", "--seconds", strconv.FormatInt(drainSeconds, 10)}
	if len(command) <= len(prefix) || !slices.Equal(command[:len(prefix)], prefix) {
		t.Fatalf("preStop = %v, want %v followed by the check", command, prefix)
	}
	return command[len(prefix):]
}

// The drain's check is the liveness probe's own, for a Health Check Path (path,
// port and Host) and for a TCP-checked service alike, so the drain can never
// disagree with the check that restarts the instance. Every drained pod
// carries the helper, and the grace period is still drain + shutdown delay.
func TestDrainRunsTheLivenessCheckFirst(t *testing.T) {
	timeout := "5s"
	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
		want []string
	}{
		{"health check path", httpHealthApp(),
			[]string{"check", "--port", "8080", "--path", "/cgi-bin/health", "--timeout", timeout, "--host", "shop-x1." + probeTestBaseDomain}},
		{"tcp-checked web service", projectionApp(), []string{"tcp", "--port", "8080", "--timeout", timeout}},
		{"tcp-checked private service", projectionApp(func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypePrivateService }),
			[]string{"tcp", "--port", "8080", "--timeout", timeout}},
		{"private service with a path", httpHealthApp(func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypePrivateService; a.Spec.Expose = false }),
			[]string{"check", "--port", "8080", "--path", "/cgi-bin/health", "--timeout", timeout}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dep := project(tc.app, execParams())
			app := appContainerOf(t, dep)
			got := drainCheckArgs(t, app)
			if !slices.Equal(got, tc.want) {
				t.Errorf("drain check = %v, want %v", got, tc.want)
			}
			if live := healthProbeCheckArgs(app.LivenessProbe.ProbeHandler); !slices.Equal(got, live) {
				t.Errorf("drain check %v disagrees with the liveness probe's %v", got, live)
			}
			if storedHealthProbeImage(dep.Spec.Template.Spec) != probeTestImage ||
				!slices.ContainsFunc(app.VolumeMounts, func(m corev1.VolumeMount) bool { return m.Name == healthProbeVolumeName && m.ReadOnly }) {
				t.Errorf("the drained pod does not carry the helper (init %v, mounts %v)", dep.Spec.Template.Spec.InitContainers, app.VolumeMounts)
			}
			if grace := dep.Spec.Template.Spec.TerminationGracePeriodSeconds; grace == nil || *grace != drainSeconds+defaultTerminationGracePeriodSeconds {
				t.Errorf("grace = %v, want %d + %d", grace, drainSeconds, defaultTerminationGracePeriodSeconds)
			}
		})
	}
}

// healthProbeCheckArgs reads every probe shape the projection writes: the
// helper's own exec as is, a TCP probe as "tcp", and a kubelet HTTP probe (the
// w2/078 guard's "/") as the no-redirect check with the same path, port and
// Host. Anything it cannot run keeps the plain sleep.
func TestHealthProbeCheckArgsFollowTheProbe(t *testing.T) {
	get := healthCheckHandler("/", 8080, "app.onbex.test")
	path, host, port, ok := healthProbeExecArgs(&corev1.ExecAction{Command: append([]string{healthProbeBinary}, healthProbeCheckArgs(get)...)})
	if !ok || path != "/" || host != "app.onbex.test" || port != 8080 {
		t.Errorf("HTTP probe read as path %q host %q port %d (%v)", path, host, port, ok)
	}
	if got := healthProbeCheckArgs(healthProbeOpenPortHandler()); len(got) == 0 || got[0] != "open-port" {
		t.Errorf("open-port probe read as %v", got)
	}
	for name, handler := range map[string]corev1.ProbeHandler{
		"foreign exec": {Exec: &corev1.ExecAction{Command: []string{"/bin/true", "check"}}},
		"named port":   {TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString("http")}},
		"none":         {},
	} {
		if got := healthProbeCheckArgs(handler); got != nil {
			t.Errorf("%s: check = %v, want none", name, got)
		}
		container := corev1.Container{LivenessProbe: &corev1.Probe{ProbeHandler: handler}}
		applyDrain(&corev1.PodSpec{}, &container, nil, drainSeconds, true)
		if container.Lifecycle.PreStop.Sleep == nil || container.Lifecycle.PreStop.Sleep.Seconds != drainSeconds {
			t.Errorf("%s: preStop = %+v, want the plain sleep", name, container.Lifecycle.PreStop)
		}
	}
}

// Workers and disk-attached services carry no preStop with the helper either,
// and a worker gets no installer for a drain it does not have. An operator
// that does not know its own image keeps the plain sleep.
func TestHealthCheckedDrainOnlyWhereADrainApplies(t *testing.T) {
	worker := execParams()
	worker.worker = true
	workerDep := project(projectionApp(), worker)
	if c := appContainerOf(t, workerDep); c.Lifecycle != nil || len(workerDep.Spec.Template.Spec.InitContainers) != 0 {
		t.Errorf("worker: lifecycle %+v init %v, want neither", c.Lifecycle, workerDep.Spec.Template.Spec.InitContainers)
	}
	disk := projectionApp(func(a *appv1alpha1.App) {
		a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	})
	diskDep := project(disk, execParams())
	if c := appContainerOf(t, diskDep); c.Lifecycle != nil {
		t.Errorf("disk-attached: lifecycle %+v, want none", c.Lifecycle)
	}
	if grace := diskDep.Spec.Template.Spec.TerminationGracePeriodSeconds; grace == nil || *grace != defaultTerminationGracePeriodSeconds {
		t.Errorf("disk-attached grace = %v, want the Kubernetes default", grace)
	}
	noImage := appContainerOf(t, project(httpHealthApp(), probeParams()))
	if noImage.Lifecycle == nil || noImage.Lifecycle.PreStop.Sleep == nil || noImage.Lifecycle.PreStop.Sleep.Seconds != drainSeconds {
		t.Errorf("no helper image: lifecycle %+v, want the plain 60 s sleep", noImage.Lifecycle)
	}
}

// A stored template keeps its plain sleep, and a TCP-checked service its
// helper-free pod, until the template changes for another reason: an operator
// upgrade must not roll every web and private service at once (ADR004 §
// Rollout headroom). The next roll adopts the health-checked drain, which then
// re-projects unchanged.
func TestHealthCheckedDrainIsAdoptedOnTheNextRollNotOnUpgrade(t *testing.T) {
	sleepingPathService := project(httpHealthApp(), execParams())
	appContainerOf(t, sleepingPathService) // fails the test if absent
	sleepingPathService.Spec.Template.Spec.Containers[0].Lifecycle = &corev1.Lifecycle{
		PreStop: &corev1.LifecycleHandler{Sleep: &corev1.SleepAction{Seconds: drainSeconds}},
	}
	for _, tc := range []struct {
		name   string
		app    func(...func(*appv1alpha1.App)) *appv1alpha1.App
		stored corev1.PodTemplateSpec
	}{
		// What w8/m133's operator stored: the helper probe, the plain sleep.
		{"health check path", httpHealthApp, sleepingPathService.Spec.Template},
		// What it stored without a path: TCP probes, no helper, the plain sleep.
		{"tcp-checked", projectionApp, project(projectionApp(), probeParams()).Spec.Template},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stored := project(tc.app(), execParams())
			stored.Spec.Template = *tc.stored.DeepCopy()
			if seconds, checked := storedDrain(stored.Spec.Template.Spec); seconds != drainSeconds || checked {
				t.Fatalf("setup: stored drain %d checked %v, want the plain sleep", seconds, checked)
			}

			upgraded := stored.DeepCopy()
			applyDeploymentSpec(upgraded, tc.app(), execParams())
			if !equalPodTemplate(upgraded.Spec.Template, stored.Spec.Template) {
				t.Fatalf("an operator upgrade changed an unchanged service's template (lifecycle %+v)", appContainerOf(t, upgraded).Lifecycle)
			}

			restarted := tc.app(func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-11-14T00:00:00Z" })
			next := stored.DeepCopy()
			applyDeploymentSpec(next, restarted, execParams())
			drainCheckArgs(t, appContainerOf(t, next))
			if seconds, checked := storedDrain(next.Spec.Template.Spec); seconds != drainSeconds || !checked {
				t.Errorf("the next roll's drain = %d checked %v, want the health-checked 60 s", seconds, checked)
			}
			if storedHealthProbeImage(next.Spec.Template.Spec) != probeTestImage {
				t.Errorf("the next roll does not install the helper")
			}

			again := next.DeepCopy()
			applyDeploymentSpec(again, restarted, execParams())
			if !equalPodTemplate(again.Spec.Template, next.Spec.Template) {
				t.Error("once adopted, the health-checked drain must re-project unchanged")
			}
		})
	}
}
