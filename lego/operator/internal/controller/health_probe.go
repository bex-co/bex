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
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The no-redirect health probe (w8/m133). Render counts a "2xx or 3xx status
// code within five seconds" as a passing health check. The kubelet's HTTP
// prober instead follows every same-host redirect (every relative Location),
// up to ten hops, and scores the final response, and HTTPGetAction has no way
// to stop it: a health path that answers 302 never became Ready. A service
// with a Health Check Path therefore probes by exec of bex's own static
// health-probe binary (internal/healthprobe), which makes one request, follows
// nothing and exits 0 for 2xx/3xx. The binary comes from bex's own image, so it
// works for any tenant image, shell-less and distroless ones included: an
// init container copies it into a shared volume, or, where the cluster serves
// image volumes, the app container mounts it straight out of bex's image
// (healthProbeImageVolume). A service with no path keeps the TCP probe, except
// a first web image release still discovering its HTTP port, which checks any
// open port with the same helper (w4/m350).
const (
	healthProbeVolumeName = "bex-health"
	healthProbeMountPath  = "/bex-health"
	healthProbeBinary     = healthProbeMountPath + "/health-probe"
	healthProbeInitName   = "bex-health-probe-install"
	// healthProbeVolumeBytes bounds the shared volume; the binary is ~5 MiB.
	healthProbeVolumeBytes = 32 << 20
	// healthProbeImageDir is the directory of bex's image that holds the
	// helper alone (lego/Dockerfile), relative to the image root an image
	// volume mounts. containerd mounts only a directory subPath from one.
	healthProbeImageDir = "bex-health"
)

// healthProbeExecHandler is healthCheckHandler's exec form for an HTTP check.
// The Host header and path travel as arguments, so the three probes still
// cannot disagree. A zero host sends the pod address, as the HTTP probe did.
func healthProbeExecHandler(path string, port int, host string) corev1.ProbeHandler {
	command := []string{healthProbeBinary, "check",
		"--port", strconv.Itoa(port), "--path", path,
		"--timeout", (time.Duration(healthCheckTimeoutSeconds) * time.Second).String()}
	if host != "" {
		command = append(command, "--host", host)
	}
	return corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: command}}
}

// healthProbeOpenPortHandler is Render's default check, "TCP socket probes to
// one of your service's open ports", as an exec of the same helper: it passes
// when the pod accepts a connection on any port it listens on, bex's own
// sidecar ports aside. It names no port, so a first release's template stays
// the same whichever port HTTP discovery chooses (w4/m350).
func healthProbeOpenPortHandler() corev1.ProbeHandler {
	reserved := appv1alpha1.ReservedImagePorts()
	skip := make([]string, len(reserved))
	for i, port := range reserved {
		skip[i] = strconv.Itoa(int(port))
	}
	return corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{healthProbeBinary, "open-port",
		"--skip", strings.Join(skip, ","),
		"--timeout", (time.Duration(healthCheckTimeoutSeconds) * time.Second).String()}}}
}

// healthProbeDrainHandler is the preStop drain of an instance that may still
// serve (w4/m353): an exec of the helper's "drain", which runs check once and
// waits seconds only when it passes. A deleted instance that is healthy keeps
// serving through the drain as before (w1/m154, w8/180); one restarted by its
// liveness probe, which readiness took out of rotation long before, gets
// SIGTERM at once, so the restart lands at Render's 60 s. check is the liveness
// probe's own handler, so the drain can never disagree with the probes. nil
// when the helper cannot run that check.
func healthProbeDrainHandler(check corev1.ProbeHandler, seconds int64) *corev1.LifecycleHandler {
	args := healthProbeCheckArgs(check)
	if args == nil {
		return nil
	}
	command := append([]string{healthProbeBinary, "drain", "--seconds", strconv.FormatInt(seconds, 10)}, args...)
	return &corev1.LifecycleHandler{Exec: &corev1.ExecAction{Command: command}}
}

// healthProbeCheckArgs is the helper subcommand and flags that run the check h
// describes: an exec of the helper as is, a TCP probe as "tcp", an HTTP probe
// as the no-redirect "check" with its path, port and Host. nil for anything
// else.
func healthProbeCheckArgs(h corev1.ProbeHandler) []string {
	timeout := (time.Duration(healthCheckTimeoutSeconds) * time.Second).String()
	switch {
	case h.Exec != nil:
		if len(h.Exec.Command) >= 2 && h.Exec.Command[0] == healthProbeBinary {
			return h.Exec.Command[1:]
		}
	case h.TCPSocket != nil && h.TCPSocket.Port.IntValue() > 0:
		return []string{"tcp", "--port", h.TCPSocket.Port.String(), "--timeout", timeout}
	case h.HTTPGet != nil && h.HTTPGet.Port.IntValue() > 0:
		host := ""
		for _, header := range h.HTTPGet.HTTPHeaders {
			if header.Name == probeHostHeader {
				host = header.Value
			}
		}
		return healthProbeExecHandler(h.HTTPGet.Path, h.HTTPGet.Port.IntValue(), host).Exec.Command[1:]
	}
	return nil
}

// healthProbeDrainSeconds reads the drain length off a preStop exec of the
// helper (healthProbeDrainHandler's inverse), 0 for any other command.
func healthProbeDrainSeconds(exec *corev1.ExecAction) int64 {
	if exec == nil || len(exec.Command) < 4 || exec.Command[0] != healthProbeBinary || exec.Command[1] != "drain" || exec.Command[2] != "--seconds" {
		return 0
	}
	seconds, _ := strconv.ParseInt(exec.Command[3], 10, 64)
	return seconds
}

// healthProbeChecksOpenPort reports an exec probe that is the any-open-port
// check (healthProbeOpenPortHandler).
func healthProbeChecksOpenPort(exec *corev1.ExecAction) bool {
	return exec != nil && len(exec.Command) >= 2 && exec.Command[0] == healthProbeBinary && exec.Command[1] == "open-port"
}

// healthProbeExecArgs reads the path, port and Host back off an exec health
// probe (healthProbeExecHandler's inverse); ok is false for any other command.
func healthProbeExecArgs(exec *corev1.ExecAction) (path, host string, port int, ok bool) {
	if exec == nil || len(exec.Command) < 2 || exec.Command[0] != healthProbeBinary || exec.Command[1] != "check" {
		return "", "", 0, false
	}
	args := exec.Command[2:]
	for i := 0; i+1 < len(args); i += 2 {
		switch args[i] {
		case "--path":
			path = args[i+1]
		case "--host":
			host = args[i+1]
		case "--port":
			port, _ = strconv.Atoi(args[i+1])
		}
	}
	return path, host, port, true
}

// healthProbeInstallMemory bounds the installer, which copies a ~5 MiB binary.
const healthProbeInstallMemory = "32Mi"

// healthProbeInstallResources is the installer's allocation: the app
// container's CPU, as requests and limits, and healthProbeInstallMemory.
//
// The CPU limit decides how fast the installer starts. runc starts a container
// inside that container's cgroup, so at the 20m first shipped containerd's
// StartContainer took 2.05–2.22 s and the copy another 0.59 s, which a disk
// service under Recreate paid as downtime on every deploy and restart (w6/170,
// after the disk holder's w6/m559). The app container's CPU costs nothing: a
// pod's request is the larger of its largest init container's and its
// containers' sum, so the pod's request, its QoS class and the plan's quota
// stay as they were. An App with no tier names no CPU on either container, and
// the namespace's LimitRange gives both the same default.
func healthProbeInstallResources(app corev1.ResourceRequirements) corev1.ResourceRequirements {
	requests := corev1.ResourceList{corev1.ResourceMemory: resource.MustParse(healthProbeInstallMemory)}
	limits := corev1.ResourceList{corev1.ResourceMemory: resource.MustParse(healthProbeInstallMemory)}
	if cpu, ok := app.Requests[corev1.ResourceCPU]; ok {
		requests[corev1.ResourceCPU] = cpu
	}
	if cpu, ok := app.Limits[corev1.ResourceCPU]; ok {
		limits[corev1.ResourceCPU] = cpu
	}
	return corev1.ResourceRequirements{Requests: requests, Limits: limits}
}

// healthProbeInstallContainer copies the health-probe binary from bex's image
// into the shared volume before the app starts. bex's own image, pinned by the
// operator: never re-pulled per pod.
func healthProbeInstallContainer(image string, resources corev1.ResourceRequirements) corev1.Container {
	sec := execution.RestrictedContainerSecurityContext()
	sec.RunAsNonRoot, sec.RunAsUser, sec.ReadOnlyRootFilesystem = new(true), new(int64(65532)), new(true)
	return corev1.Container{
		Name:            healthProbeInitName,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/health-probe", "install", healthProbeBinary},
		Resources:       resources,
		VolumeMounts:    []corev1.VolumeMount{{Name: healthProbeVolumeName, MountPath: healthProbeMountPath}},
		SecurityContext: sec,
	}
}

// healthProbeVolume is the shared volume the binary is installed into.
func healthProbeVolume() corev1.Volume {
	limit := resource.NewQuantity(healthProbeVolumeBytes, resource.BinarySI)
	return corev1.Volume{Name: healthProbeVolumeName, VolumeSource: corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: limit},
	}}
}

// healthProbeImageVolume is the helper's volume on a cluster that serves image
// volumes (BEX_HEALTH_PROBE_IMAGE_VOLUME, w6/m624): bex's image itself, which
// the kubelet pulls like a container image and mounts read-only before it
// starts the app. With no init container to wait for, the kubelet starts the
// app in the pass that starts the pod. The installer's exit was noticed only
// on the next relist, up to a second later, which a disk service under
// Recreate paid as downtime on every deploy and restart (w6/176).
func healthProbeImageVolume(image string) corev1.Volume {
	return corev1.Volume{Name: healthProbeVolumeName, VolumeSource: corev1.VolumeSource{
		Image: &corev1.ImageVolumeSource{Reference: image, PullPolicy: corev1.PullIfNotPresent},
	}}
}

// healthProbeMount is the app container's read-only mount of the helper at
// healthProbeMountPath: the installer's volume, or the directory of bex's
// image that holds the helper alone, so the rest of bex's image stays out of
// the container. Either way the probes exec the same healthProbeBinary.
func healthProbeMount(imageVolume bool) corev1.VolumeMount {
	mount := corev1.VolumeMount{Name: healthProbeVolumeName, MountPath: healthProbeMountPath, ReadOnly: true}
	if imageVolume {
		mount.SubPath = healthProbeImageDir
	}
	return mount
}

// storedHealthProbeInstaller is the health-probe install container a stored
// pod spec carries, nil when it probes another way.
func storedHealthProbeInstaller(spec corev1.PodSpec) *corev1.Container {
	for i := range spec.InitContainers {
		if spec.InitContainers[i].Name == healthProbeInitName {
			return &spec.InitContainers[i]
		}
	}
	return nil
}

// storedHealthProbeImageVolume is the image volume a stored pod spec mounts
// the helper from, nil when it installs it or probes another way.
func storedHealthProbeImageVolume(spec corev1.PodSpec) *corev1.ImageVolumeSource {
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == healthProbeVolumeName {
			return spec.Volumes[i].Image
		}
	}
	return nil
}

// storedHealthProbeImage is the image a stored pod spec takes the helper
// from, its installer's or its image volume's, "" when it probes another way.
func storedHealthProbeImage(spec corev1.PodSpec) string {
	if c := storedHealthProbeInstaller(spec); c != nil {
		return c.Image
	}
	if v := storedHealthProbeImageVolume(spec); v != nil {
		return v.Reference
	}
	return ""
}
