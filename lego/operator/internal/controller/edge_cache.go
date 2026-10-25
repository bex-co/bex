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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Edge caching (w5/m151): a web service whose spec.cacheProfile asks for
// Render's edge caching gets bex's edge-cache proxy (internal/edgecache) as a
// sidecar, and its Service targets the proxy, which forwards to the app over
// the pod's loopback. The cache lives in the proxy's memory, so a deploy, or a
// manual purge (which rolls the pods), starts it empty, as Render's does.
const (
	edgeCacheContainerName = "edge-cache"
	edgeCachePortName      = "edge-cache"
	// edgeCachePort is reserved (IsReservedImagePort), so a service's own
	// PORT can never collide with it.
	edgeCachePort = 18014
	// edgeCacheMaxBytes bounds the cache's memory; its container limit leaves
	// room for the proxy itself.
	edgeCacheMaxBytes = 48 << 20
)

// edgeCaches reports whether the App gets the edge cache: a paid web service
// with a caching profile, on an operator that knows its own image. Render
// offers no edge caching on free web services.
func (r *AppReconciler) edgeCaches(app *appv1alpha1.App) bool {
	return app.Spec.EdgeCaching() && !isFreeApp(app) && !app.Spec.HasImagePorts() && r.BackupHelperImage != ""
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
	sec := tenantSecCtx()
	sec.RunAsNonRoot, sec.RunAsUser, sec.ReadOnlyRootFilesystem = new(true), new(int64(65532)), new(true)
	return corev1.Container{
		Name:  edgeCacheContainerName,
		Image: image,
		// bex's own image, pinned by the operator: never re-pulled per pod.
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/edge-cache"},
		Env: []corev1.EnvVar{
			{Name: "BEX_EDGE_CACHE_LISTEN", Value: ":" + strconv.Itoa(edgeCachePort)},
			{Name: "BEX_EDGE_CACHE_UPSTREAM", Value: "http://127.0.0.1:" + strconv.Itoa(appPort)},
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
