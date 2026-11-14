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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Edge caching (w5/m151): a web service whose spec.cacheProfile asks for
// Render's edge caching gets bex's edge-cache proxy (internal/edgecache) as a
// sidecar, and its Service targets the proxy, which forwards to the app over
// the pod's loopback. The cache lives in the proxy's memory, so a deploy, or a
// manual purge (which rolls the pods), starts it empty, as Render's does. The
// sidecar fronts the port the pod's release was projected on, which for an
// image-port service is its image network's primary (docs/ADR004 § Edge
// caching covers discovery and port changes).
const (
	edgeCacheContainerName = "edge-cache"
	edgeCachePortName      = "edge-cache"
	edgeCacheUpstreamEnv   = "BEX_EDGE_CACHE_UPSTREAM"
	// edgeCacheUpstreamPrefix precedes the app port in edgeCacheUpstreamEnv.
	edgeCacheUpstreamPrefix = "http://127.0.0.1:"
	// edgeCachePort is reserved (IsReservedImagePort), so a service's own
	// PORT can never collide with it.
	edgeCachePort = 18014
	// edgeCacheMaxBytes bounds the cache's memory; its container limit leaves
	// room for the proxy itself.
	edgeCacheMaxBytes = 48 << 20
)

// edgeCaches reports whether the App gets the edge cache: the spec serves one
// (AppSpec.ServesEdgeCache, which bex-api's edge-cache events and purge read
// too), on an operator that knows its own image.
func (r *AppReconciler) edgeCaches(app *appv1alpha1.App) bool {
	return app.Spec.ServesEdgeCache() && r.BackupHelperImage != ""
}

// edgeCacheContainer is the sidecar. drain is the app's preStop sleep: the
// proxy keeps serving through it, so a rollout or purge never drops requests
// the endpoints still route to the pod. GOMEMLIMIT keeps the Go heap well
// inside the container limit with the cache full.
func edgeCacheContainer(app *appv1alpha1.App, image string, appPort int, drain int64) corev1.Container {
	resources := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("100m"),
		corev1.ResourceMemory: resource.MustParse("160Mi"),
	}
	sec := execution.RestrictedContainerSecurityContext()
	sec.RunAsNonRoot, sec.RunAsUser, sec.ReadOnlyRootFilesystem = new(true), new(int64(65532)), new(true)
	return corev1.Container{
		Name:  edgeCacheContainerName,
		Image: image,
		// bex's own image, pinned by the operator: never re-pulled per pod.
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/edge-cache"},
		Env: []corev1.EnvVar{
			{Name: "BEX_EDGE_CACHE_LISTEN", Value: ":" + strconv.Itoa(edgeCachePort)},
			{Name: edgeCacheUpstreamEnv, Value: edgeCacheUpstreamPrefix + strconv.Itoa(appPort)},
			{Name: "BEX_EDGE_CACHE_PROFILE", Value: app.Spec.CacheProfile},
			{Name: "BEX_EDGE_CACHE_MAX_BYTES", Value: strconv.Itoa(edgeCacheMaxBytes)},
			{Name: "BEX_EDGE_CACHE_DRAIN_SECONDS", Value: strconv.FormatInt(drain, 10)},
			{Name: "GOMEMLIMIT", Value: "120MiB"},
		},
		Ports:     []corev1.ContainerPort{{Name: edgeCachePortName, ContainerPort: edgeCachePort, Protocol: corev1.ProtocolTCP}},
		Resources: corev1.ResourceRequirements{Requests: resources, Limits: resources},
		ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(edgeCachePort)},
		}},
		SecurityContext: sec,
	}
}

// sidecarUpstreamPort is the app port a pod's edge-cache sidecar forwards to
// (edgeCacheContainer writes it); ok is false for a pod without a sidecar.
func sidecarUpstreamPort(p *corev1.Pod) (int32, bool) {
	if !podHasPortName(p, edgeCachePortName) {
		return 0, false
	}
	for _, c := range p.Spec.Containers {
		if c.Name != edgeCacheContainerName {
			continue
		}
		for _, e := range c.Env {
			if e.Name != edgeCacheUpstreamEnv {
				continue
			}
			number, found := strings.CutPrefix(e.Value, edgeCacheUpstreamPrefix)
			port, err := strconv.ParseInt(number, 10, 32)
			return int32(port), found && err == nil
		}
	}
	return 0, false
}

// podsCachePort reports whether every routable pod of pods (those a Service
// selects) runs a sidecar in front of port. An image-port Service targets the
// cache only then. A pod without the sidecar would drop out of a Service naming
// the cache's listener, so turning caching on would empty it mid-rollout (the
// hold servingTargetPort keeps for a configured port). A pod whose sidecar
// fronts another port was projected on another primary, as a first release's
// pods are after its HTTP port choice moves the primary, and its cache would
// forward to a port the app may not serve. Either keeps the direct target
// until it is gone.
func podsCachePort(pods []corev1.Pod, port int32) bool {
	for i := range pods {
		if !routablePod(&pods[i]) {
			continue
		}
		if upstream, ok := sidecarUpstreamPort(&pods[i]); !ok || upstream != port {
			return false
		}
	}
	return true
}

// targetImageEdgeCache points every Service port that targets an image-port
// web service's primary at the edge cache: the primary's own port and a
// replaced primary aliased to it through a port change (aliasedPriorPrimary).
// withPrivateNetworkPorts then copies the primary's target onto 10000. Other
// declared ports stay direct to the app.
func targetImageEdgeCache(ports []corev1.ServicePort, primary int32) {
	for i := range ports {
		if ports[i].TargetPort == intstr.FromInt32(primary) {
			ports[i].TargetPort = intstr.FromString(edgeCachePortName)
		}
	}
}
