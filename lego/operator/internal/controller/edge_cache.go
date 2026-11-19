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
	"context"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/edgecache"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Edge caching (w5/m151): a web service whose spec.cacheProfile asks for
// Render's edge caching gets bex's edge-cache proxy (internal/edgecache) as a
// sidecar, and its Service targets the proxy, which forwards to the app over
// the pod's loopback. The cache lives in the proxy's memory, so a deploy starts
// it empty, as Render purges on deploy. A manual purge and a profile switch
// restart nothing (w3/m486): the operator patches each live pod's annotations
// (annotateEdgeCachePods) and the proxy reads them from a downward-API file, as
// Render's CDN purges without touching an instance. The sidecar fronts the
// port the pod's release was projected on, which for an image-port service is
// its image network's primary (docs/ADR004 § Edge caching covers discovery and
// port changes).
const (
	edgeCacheContainerName = appv1alpha1.EdgeCacheContainer
	edgeCachePortName      = "edge-cache"
	edgeCacheUpstreamEnv   = "BEX_EDGE_CACHE_UPSTREAM"
	// edgeCacheProfileEnv is the profile the proxy starts with; a pod's
	// edgecache.ProfileAnnotation overrides it while the pod runs.
	edgeCacheProfileEnv = "BEX_EDGE_CACHE_PROFILE"
	// edgeCacheControlEnv names the proxy's control file: the pod's
	// annotations, which the kubelet keeps current in a downward-API volume.
	// A sidecar without it was projected before w3/m486 and is rolled for a
	// purge or a profile switch instead.
	edgeCacheControlEnv        = "BEX_EDGE_CACHE_CONTROL"
	edgeCacheControlVolumeName = "edge-cache-control"
	edgeCacheControlDir        = "/etc/bex/edge-cache"
	edgeCacheControlFile       = "annotations"
	// edgeCacheUpstreamPrefix precedes the app port in edgeCacheUpstreamEnv.
	edgeCacheUpstreamPrefix = "http://127.0.0.1:"
	// edgeCachePort is one of the three ports Render reserves
	// (IsReservedImagePort), so a service's own PORT can never collide with it
	// and the refused set stays Render's: 18014, which the sidecar used to take,
	// is an ordinary port again. The Service reaches it by name
	// (edgeCachePortName), so a rollout from the old port is seamless.
	edgeCachePort = 19099
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

// edgeCacheContainer is the sidecar, starting on profile. drain is the app's
// preStop sleep: the proxy keeps serving through it, so a rollout never drops
// requests the endpoints still route to the pod. GOMEMLIMIT keeps the Go heap
// well inside the container limit with the cache full. Alone it is the sidecar
// a template stored before w3/m486 runs; withEdgeCacheControl gives it the
// control file.
func edgeCacheContainer(image, profile string, appPort int, drain int64) corev1.Container {
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
			{Name: edgeCacheProfileEnv, Value: profile},
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

// withEdgeCacheControl gives spec's sidecar its control file: the pod's own
// annotations in a downward-API volume, which the kubelet rewrites when the
// operator patches them, so a purge and a profile switch reach the running
// proxy.
func withEdgeCacheControl(spec *corev1.PodSpec, sidecar *corev1.Container) {
	sidecar.Env = append(sidecar.Env, corev1.EnvVar{Name: edgeCacheControlEnv, Value: edgeCacheControlDir + "/" + edgeCacheControlFile})
	sidecar.VolumeMounts = append(sidecar.VolumeMounts, corev1.VolumeMount{Name: edgeCacheControlVolumeName, MountPath: edgeCacheControlDir, ReadOnly: true})
	spec.Volumes = append(spec.Volumes, edgeCacheControlVolume())
}

// edgeCacheControlVolume is the volume withEdgeCacheControl mounts.
func edgeCacheControlVolume() corev1.Volume {
	return corev1.Volume{Name: edgeCacheControlVolumeName, VolumeSource: corev1.VolumeSource{
		DownwardAPI: &corev1.DownwardAPIVolumeSource{Items: []corev1.DownwardAPIVolumeFile{{
			Path:     edgeCacheControlFile,
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.annotations"},
		}}},
	}}
}

// edgeCacheSidecar is a pod spec's edge-cache container, nil without one.
func edgeCacheSidecar(spec corev1.PodSpec) *corev1.Container {
	for i := range spec.Containers {
		if spec.Containers[i].Name == edgeCacheContainerName {
			return &spec.Containers[i]
		}
	}
	return nil
}

func containerEnv(c *corev1.Container, name string) (string, bool) {
	for _, e := range c.Env {
		if e.Name == name {
			return e.Value, true
		}
	}
	return "", false
}

// keptEdgeCache is the sidecar p projects, as a stored pod spec already runs
// it: what applyDeploymentSpec keeps when nothing else about the template
// changes. A sidecar with its control file keeps the profile it started with,
// since the live profile reaches it through the pod's annotations, and stays,
// as a pass-through, when caching is turned off. One from before w3/m486 has
// no control file, so it keeps its own shape, which carries the purge stamp on
// the template: a purge, a switch or turning caching off still rolls it.
func keptEdgeCache(stored corev1.PodSpec, p deploymentParams) (profile string, legacy bool) {
	sidecar := edgeCacheSidecar(stored)
	if sidecar == nil || p.edgeCacheImage == "" {
		return p.edgeCacheProfile, false
	}
	if _, control := containerEnv(sidecar, edgeCacheControlEnv); !control {
		return p.edgeCacheProfile, p.edgeCacheProfile != ""
	}
	profile, _ = containerEnv(sidecar, edgeCacheProfileEnv)
	return profile, false
}

// projectEdgeCache appends the sidecar to dep's pod template. Its purge stamp
// and live profile reach it through the pod's annotations, so the template
// carries neither: a stamp a template from before w3/m486 carried leaves with
// the roll that adds the control file. A legacy sidecar (keptEdgeCache) keeps
// the stamp on the template, so a purge still rolls it.
func projectEdgeCache(dep *appsv1.Deployment, app *appv1alpha1.App, p deploymentParams, drain int64) {
	tmpl := &dep.Spec.Template
	sidecar := edgeCacheContainer(p.edgeCacheImage, p.edgeCacheProfile, p.port, drain)
	switch purged := app.Annotations[appv1alpha1.CachePurgedAtAnnotation]; {
	case !p.edgeCacheLegacy:
		withEdgeCacheControl(&tmpl.Spec, &sidecar)
		delete(tmpl.Annotations, appv1alpha1.CachePurgedAtAnnotation)
	case purged != "":
		metav1.SetMetaDataAnnotation(&tmpl.ObjectMeta, appv1alpha1.CachePurgedAtAnnotation, purged)
	}
	tmpl.Spec.Containers = append(tmpl.Spec.Containers, sidecar)
}

// edgeCacheProfileFor is the profile app's proxies cache by: its saved one
// while the cache runs, and no-cache, a pass-through, once it does not.
func edgeCacheProfileFor(app *appv1alpha1.App) string {
	if app.Spec.ServesEdgeCache() {
		return app.Spec.CacheProfile
	}
	return appv1alpha1.CacheProfileNone
}

// updateAppPodsInPlace moves what the App's running pods carry outside their
// template, rolling none: the environment labels (relabelAppPods), then the
// edge-cache control annotations (annotateEdgeCachePods).
func (r *AppReconciler) updateAppPodsInPlace(ctx context.Context, app *appv1alpha1.App) error {
	if err := r.relabelAppPods(ctx, app); err != nil {
		return err
	}
	return r.annotateEdgeCachePods(ctx, app)
}

// annotateEdgeCachePods hands each live pod's edge-cache proxy the App's last
// purge and current profile through the pod's own annotations (w3/m486), on
// every pass, ahead of any build, hold or gate, like relabelAppPods. The pod
// template never carries either, so neither a purge nor a switch rolls an
// instance; a pod created later starts empty and on its template's profile,
// and is patched here on the pass its creation triggers. A pod whose sidecar
// has no control file (a template from before w3/m486) is left alone: its
// template rolls instead.
func (r *AppReconciler) annotateEdgeCachePods(ctx context.Context, app *appv1alpha1.App) error {
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(app.Namespace), client.MatchingLabels{labelApp: app.Name}); err != nil {
		return err
	}
	profile, purged := edgeCacheProfileFor(app), app.Annotations[appv1alpha1.CachePurgedAtAnnotation]
	purgedAt, err := time.Parse(time.RFC3339Nano, purged)
	for i := range pods.Items {
		pod := &pods.Items[i]
		sidecar := edgeCacheSidecar(pod.Spec)
		if sidecar == nil || !pod.DeletionTimestamp.IsZero() {
			continue
		}
		if _, control := containerEnv(sidecar, edgeCacheControlEnv); !control {
			continue
		}
		current, annotated := pod.Annotations[edgecache.ProfileAnnotation]
		if !annotated {
			current, _ = containerEnv(sidecar, edgeCacheProfileEnv)
		}
		// A pod created after the purge started empty: handing it the
		// stamp would only purge it again once its proxy is up.
		stamp := purged
		if err == nil && pod.CreationTimestamp.After(purgedAt) {
			stamp = ""
		}
		if current == profile && (stamp == "" || pod.Annotations[edgecache.PurgedAtAnnotation] == stamp) {
			continue
		}
		patch := client.MergeFrom(pod.DeepCopy())
		metav1.SetMetaDataAnnotation(&pod.ObjectMeta, edgecache.ProfileAnnotation, profile)
		if stamp != "" {
			metav1.SetMetaDataAnnotation(&pod.ObjectMeta, edgecache.PurgedAtAnnotation, stamp)
		}
		if err := client.IgnoreNotFound(r.Patch(ctx, pod, patch)); err != nil {
			return err
		}
	}
	return nil
}

// sidecarUpstreamPort is the app port a pod's edge-cache sidecar forwards to
// (edgeCacheContainer writes it); ok is false for a pod without a sidecar.
func sidecarUpstreamPort(p *corev1.Pod) (int32, bool) {
	sidecar := edgeCacheSidecar(p.Spec)
	if sidecar == nil || !podHasPortName(p, edgeCachePortName) {
		return 0, false
	}
	upstream, ok := containerEnv(sidecar, edgeCacheUpstreamEnv)
	number, found := strings.CutPrefix(upstream, edgeCacheUpstreamPrefix)
	port, err := strconv.ParseInt(number, 10, 32)
	return int32(port), ok && found && err == nil
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
