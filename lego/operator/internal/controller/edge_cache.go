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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/edgecache"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/types/k8sname"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Edge caching (w5/m151, a per-service tier since w3/m489). A web service
// whose spec.cacheProfile asks for Render's edge caching gets its own cache
// tier in its own namespace: a Deployment of bex's edge-cache proxy
// (internal/edgecache) behind a headless Service. Its public route moves to
// the tier once a tier pod is Ready and back to the service's own Service
// whenever none is, and the tier forwards misses to that Service's port 10000,
// which targets the primary listener whatever its port. The cache therefore
// lives outside the instances, as Render's CDN does: a Restart, a scale
// change, an instance crash or a node drain leaves it alone, and every
// instance is behind one fill per key. It empties only on a successful deploy
// and on a manual purge, which reach the running proxies through their pods'
// annotations (annotateEdgeCacheTierPods), so neither restarts anything.
// docs/ADR004 § Edge caching records the design.
const (
	edgeCacheContainerName = appv1alpha1.EdgeCacheContainer
	edgeCachePortName      = "edge-cache"
	// edgeCachePort is the proxy's listener. It was the sidecar's inside the
	// app's own pods, so it is one of the three ports Render reserves
	// (IsReservedImagePort); a pod template stored before w3/m489 may still
	// carry that sidecar.
	edgeCachePort = 19099
	// edgeCacheControlEnv names the proxy's control file: its pod's
	// annotations, which the kubelet keeps current in a downward-API volume.
	edgeCacheControlEnv        = "BEX_EDGE_CACHE_CONTROL"
	edgeCacheControlVolumeName = "edge-cache-control"
	edgeCacheControlDir        = "/etc/bex/edge-cache"
	edgeCacheControlFile       = "annotations"
	edgeCacheStoreVolumeName   = "edge-cache-store"
	edgeCacheStoreDir          = "/var/cache/bex-edge"

	// labelEdgeCacheTier selects an App's tier pods; its value is the App's
	// name. They carry no app.bex.co/app label, so no instance list, metric,
	// log stream, shell, bill or Service of the App's own counts them.
	labelEdgeCacheTier = "app.bex.co/edge-cache"
	// edgeCacheComponent is the tier pods' app.bex.co/component.
	edgeCacheComponent = "edge-cache"

	// The tier's shape. Two replicas so one pod's loss never empties the
	// route: Traefik sends no new request to a terminating endpoint, and a
	// Service with only terminating endpoints answers 503. Each key is filled
	// by the one replica that owns it (edgecache.SetPeers), so a key reaches
	// the app once whatever the replica and instance counts.
	edgeCacheTierReplicas = 2
	edgeCacheTierPort     = 80
	// edgeCacheMaxBytes is one replica's body store on its emptyDir and
	// edgeCacheMaxObjectBytes one response's cap. The emptyDir's size limit,
	// past which the kubelet evicts the pod, holds the store, the
	// edgeCacheFills fills that may stream to temp files at once (the proxy's
	// default), and 256 MiB of headroom for files a purge is still removing.
	edgeCacheMaxBytes        = 1 << 30
	edgeCacheMaxObjectBytes  = 128 << 20
	edgeCacheFills           = 4
	edgeCacheStoreLimitBytes = edgeCacheMaxBytes + edgeCacheFills*edgeCacheMaxObjectBytes + 256<<20
	// edgeCacheDrainSeconds keeps a terminating tier pod serving while
	// Traefik and its peer drop it, before it finishes in-flight requests.
	edgeCacheDrainSeconds = 10
)

// edgeCaches reports whether the App gets the edge-cache tier: a publicly
// routed web service that runs one on the plan its instances run
// (App.RunsEdgeCache, which bex-api's edge-cache events and purge read too),
// on a Kubernetes operator that knows its own image. A plan saved for the
// next deploy moves the cache with that deploy, not the save (w6/m609).
func (r *AppReconciler) edgeCaches(app *appv1alpha1.App) bool {
	return r.Mode == ModeKubernetes && r.BackupHelperImage != "" && app.RunsEdgeCache() && app.Spec.PubliclyRoutable()
}

// edgeCacheTierName names an App's tier Deployment and its headless Service.
func edgeCacheTierName(appName string) string {
	return k8sname.Prefixed("bex-edge-cache-", appName)
}

// edgeCacheProfileFor is the profile app's proxies cache by: its saved one
// while the cache runs, and no-cache, a pass-through, once it does not.
func edgeCacheProfileFor(app *appv1alpha1.App) string {
	if app.RunsEdgeCache() {
		return app.Spec.CacheProfile
	}
	return appv1alpha1.CacheProfileNone
}

// updateAppPodsInPlace moves what the App's running pods carry outside their
// template, rolling none: the environment labels (relabelAppPods); then it
// converges the edge-cache tier, whose pods take the purge and profile the
// same way. A tier failure is logged and retried by the next pass, never
// returned: the route stays on the App's own Service while the tier is not
// Ready, so a broken tier can cost the cache but never a deploy.
func (r *AppReconciler) updateAppPodsInPlace(ctx context.Context, app *appv1alpha1.App) error {
	if err := r.relabelAppPods(ctx, app); err != nil {
		return err
	}
	if err := r.reconcileEdgeCacheTier(ctx, app); err != nil {
		logf.FromContext(ctx).Error(err, "converging the edge-cache tier", "app", app.Name)
	}
	return nil
}

// reconcileEdgeCacheTier converges app's tier: its headless Service and
// Deployment while the App caches, then its pods' environment labels and
// control annotations; once it does not, it removes them, after the route
// has left them (retireEdgeCacheTier).
func (r *AppReconciler) reconcileEdgeCacheTier(ctx context.Context, app *appv1alpha1.App) error {
	if !r.edgeCaches(app) {
		return r.retireEdgeCacheTier(ctx, app)
	}
	name := edgeCacheTierName(app.Name)
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		projectEdgeCacheTierService(svc, app)
		return controllerutil.SetControllerReference(app, svc, r.Scheme)
	}); err != nil {
		return err
	}
	appDep := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, appDep); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		appDep = nil
	}
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		existed := dep.ResourceVersion != ""
		before := dep.Spec.Template.DeepCopy()
		// The tier Deployment's own metadata (never its pod template)
		// records the release whose successful deploy last emptied the cache,
		// under the name its pods are handed it by.
		last := dep.Annotations[edgecache.RevisionAnnotation]
		revision := edgeCacheRevision(app, appDep, last)
		metav1.SetMetaDataAnnotation(&dep.ObjectMeta, edgecache.RevisionAnnotation, revision)
		projectEdgeCacheTier(dep, app, r.edgeCacheTierImage(dep, app, before, existed && revision == last))
		holdIsolationLabel(&dep.Spec.Template, existed, before, inPlaceLabelsOf(app))
		return controllerutil.SetControllerReference(app, dep, r.Scheme)
	}); err != nil {
		return err
	}
	// The tier's environment labels and region move in place, as the App's
	// own pods' do (relabelAppPods): a toggle or a move rolls no tier pod
	// either. The tier dials the App's pods, so it shares their region.
	if err := relabelPodsInPlace(ctx, r.Client, app.Namespace, client.MatchingLabels{labelEdgeCacheTier: app.Name}, inPlaceLabelsOf(app),
		func(_ *corev1.Pod, owner *metav1.OwnerReference) bool { return owner.Kind == kindReplicaSet }); err != nil {
		return err
	}
	return r.annotateEdgeCacheTierPods(ctx, app, dep.Annotations[edgecache.RevisionAnnotation])
}

// edgeCacheTierImage is the image the tier runs: the one it runs already while
// nothing else about its template changes, so an operator release alone
// restarts no tier pod and empties no cache (the rule w6/m484 sets for the
// App's own pods). A successful deploy, which empties the cache anyway
// (held false), and any other template change adopt the operator's own image.
func (r *AppReconciler) edgeCacheTierImage(dep *appsv1.Deployment, app *appv1alpha1.App, before *corev1.PodTemplateSpec, held bool) string {
	proxy := edgeCacheSidecar(before.Spec)
	if !held || proxy == nil || proxy.Image == r.BackupHelperImage {
		return r.BackupHelperImage
	}
	// The environment labels are held apart (holdIsolationLabel).
	kept := dep.DeepCopy()
	projectEdgeCacheTier(kept, app, proxy.Image)
	if templateEqualIgnoringIsolation(&kept.Spec.Template, before) {
		return proxy.Image
	}
	return r.BackupHelperImage
}

// edgeCacheRevision is the release whose successful deploy last emptied app's
// cache, last as the tier Deployment records it. It moves to the active
// release once that is the App's current release, it is no Restart, and the
// App's own rollout has no old instance left, so a fill can no longer reach
// the replaced version. Render purges "each time you successfully deploy a
// new version", and "failed deploys do not trigger a purge": a failed or
// pending release keeps the active one, and a Restart, which bex runs as a
// new release of the serving one (spec.releaseConfig.preserveGroupValues),
// is no new version. A new tier starts empty, on the active release.
//
// A release stamped but not yet adopted holds it too. The tier converges at
// the top of a pass, before the adoption, so the status still names the active
// release while spec.releaseConfig is already the request's, and restartRelease
// can no longer tell a Restart. A deploy requested after a Restart recorded the
// Restart's release and emptied the cache before that deploy began, even one
// that then failed (w3/m489 t007, on production).
func edgeCacheRevision(app *appv1alpha1.App, appDep *appsv1.Deployment, last string) string {
	active := app.Status.ActiveRevision
	switch {
	case last == "":
		return active
	case active == "" || active == last || active != releaseRevision(app) || releaseUnadopted(app) || restartRelease(app):
		return last
	case appDep == nil || appDep.Spec.Template.Labels[labelRevision] != active ||
		appDep.Status.ObservedGeneration < appDep.Generation || appDep.Status.UpdatedReplicas != appDep.Status.Replicas:
		return last
	}
	return active
}

// restartRelease reports a current release that is a Restart: bex-api's
// Restart selects the serving release with its exact configuration
// (ReleaseConfigReference.PreserveGroupValues), where a Rollback or an
// environment save reusing the served build does not.
func restartRelease(app *appv1alpha1.App) bool {
	ref := app.ActiveReleaseConfig()
	return ref != nil && ref.PreserveGroupValues
}

// projectEdgeCacheTierService is the tier's headless Service. Traefik routes
// to its endpoints, never its ClusterIP, and each tier pod resolves its name
// to its Ready peers (BEX_EDGE_CACHE_PEERS).
func projectEdgeCacheTierService(svc *corev1.Service, app *appv1alpha1.App) {
	svc.Labels = map[string]string{labelEdgeCacheTier: app.Name, execution.ManagedByLabel: execution.ManagedByOperator}
	svc.Spec.ClusterIP = corev1.ClusterIPNone
	svc.Spec.Selector = map[string]string{labelEdgeCacheTier: app.Name}
	svc.Spec.Ports = []corev1.ServicePort{{Name: "http", Port: edgeCacheTierPort, TargetPort: intstr.FromString(edgeCachePortName)}}
	applyServicePortServerDefaults(svc.Spec.Ports)
}

// projectEdgeCacheTier writes the tier Deployment's spec for app on image. Its
// template carries neither the profile nor a purge: both reach the running
// proxies through their annotations, so a new pod passes every request
// through until its first pass annotates it, within seconds of its creation.
func projectEdgeCacheTier(dep *appsv1.Deployment, app *appv1alpha1.App, image string) {
	selector := map[string]string{labelEdgeCacheTier: app.Name}
	labels := map[string]string{labelEdgeCacheTier: app.Name, execution.LabelComponent: edgeCacheComponent}
	if ws := app.Labels[labelWorkspace]; ws != "" {
		labels[labelWorkspace] = ws
	}
	dep.Labels = map[string]string{labelEdgeCacheTier: app.Name, execution.ManagedByLabel: execution.ManagedByOperator}
	dep.Spec.Replicas = new(int32(edgeCacheTierReplicas))
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
	// A rollout keeps both replicas serving: the new pod is Ready before an
	// old one leaves.
	dep.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{MaxUnavailable: new(intstr.FromInt32(0)), MaxSurge: new(intstr.FromInt32(1))}}
	dep.Spec.Template.Labels = labels
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("100m"),
			corev1.ResourceMemory:           resource.MustParse("192Mi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("1"),
			corev1.ResourceMemory:           resource.MustParse("192Mi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
		},
	}
	sec := execution.RestrictedContainerSecurityContext()
	sec.RunAsNonRoot, sec.RunAsUser, sec.ReadOnlyRootFilesystem = new(true), new(int64(65532)), new(true)
	storeLimit := resource.NewQuantity(edgeCacheStoreLimitBytes, resource.BinarySI)
	dep.Spec.Template.Spec = corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:  edgeCacheContainerName,
			Image: image,
			// bex's own image, pinned by the operator: never re-pulled per pod.
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         []string{"/edge-cache"},
			Env: []corev1.EnvVar{
				{Name: "BEX_EDGE_CACHE_LISTEN", Value: ":" + strconv.Itoa(edgeCachePort)},
				// Port 10000 of the App's own Service targets its primary
				// listener on every port path and through a port change
				// (withPrivateNetworkPorts), so the tier never follows a port.
				{Name: "BEX_EDGE_CACHE_UPSTREAM", Value: "http://" + app.Name + ":" + strconv.Itoa(int(appv1alpha1.PrivateHTTPPort))},
				{Name: "BEX_EDGE_CACHE_DIR", Value: edgeCacheStoreDir},
				{Name: "BEX_EDGE_CACHE_MAX_BYTES", Value: strconv.Itoa(edgeCacheMaxBytes)},
				{Name: "BEX_EDGE_CACHE_MAX_OBJECT_BYTES", Value: strconv.Itoa(edgeCacheMaxObjectBytes)},
				{Name: edgeCacheControlEnv, Value: edgeCacheControlDir + "/" + edgeCacheControlFile},
				{Name: "BEX_EDGE_CACHE_PEERS", Value: edgeCacheTierName(app.Name)},
				{Name: "BEX_EDGE_CACHE_SELF", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: corev1.SchemeGroupVersion.Version, FieldPath: "status.podIP"}}},
				{Name: "BEX_EDGE_CACHE_DRAIN_SECONDS", Value: strconv.Itoa(edgeCacheDrainSeconds)},
				{Name: "GOMEMLIMIT", Value: "150MiB"},
			},
			Ports:     []corev1.ContainerPort{{Name: edgeCachePortName, ContainerPort: edgeCachePort, Protocol: corev1.ProtocolTCP}},
			Resources: resources,
			ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(edgeCachePort)},
			}},
			VolumeMounts: []corev1.VolumeMount{
				{Name: edgeCacheControlVolumeName, MountPath: edgeCacheControlDir, ReadOnly: true},
				{Name: edgeCacheStoreVolumeName, MountPath: edgeCacheStoreDir},
			},
			SecurityContext: sec,
		}},
		Volumes: []corev1.Volume{
			edgeCacheControlVolume(),
			{Name: edgeCacheStoreVolumeName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: storeLimit}}},
		},
		// Spread the replicas so a node's loss leaves the other serving.
		Affinity: &corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				Weight: 100,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{MatchLabels: selector},
					TopologyKey:   corev1.LabelHostname,
				},
			}},
		}},
		TerminationGracePeriodSeconds: new(int64(edgeCacheDrainSeconds + 30)),
		AutomountServiceAccountToken:  new(false),
		EnableServiceLinks:            new(false),
	}
	applyPodSpecServerDefaults(&dep.Spec.Template.Spec)
}

// edgeCacheControlVolume is the proxy's control file: its pod's own
// annotations in a downward-API volume, which the kubelet rewrites when the
// operator patches them, so a purge and a profile switch reach the running
// proxy.
func edgeCacheControlVolume() corev1.Volume {
	return corev1.Volume{Name: edgeCacheControlVolumeName, VolumeSource: corev1.VolumeSource{
		DownwardAPI: &corev1.DownwardAPIVolumeSource{Items: []corev1.DownwardAPIVolumeFile{{
			Path:     edgeCacheControlFile,
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.annotations"},
		}}},
	}}
}

// edgeCacheTierServing reports whether app's public route goes through its
// tier: the App caches and a tier pod is Ready. Otherwise the route stays on,
// or returns to, the App's own Service, so turning caching on, a tier that
// cannot start, and losing every tier pod at once all serve uncached
// (DYNAMIC) instead of failing.
func (r *AppReconciler) edgeCacheTierServing(ctx context.Context, app *appv1alpha1.App) bool {
	if !r.edgeCaches(app) {
		return false
	}
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: edgeCacheTierName(app.Name)}, dep); err != nil {
		return false
	}
	return dep.Status.ReadyReplicas > 0
}

// retireEdgeCacheTier removes the tier of an App that no longer caches, once
// none of its Ingresses routes to it: the pass that turns caching off hands
// the proxies no-cache and moves the route, and the Ingress write it makes
// brings the next pass, which removes the tier.
func (r *AppReconciler) retireEdgeCacheTier(ctx context.Context, app *appv1alpha1.App) error {
	name := edgeCacheTierName(app.Name)
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, dep); err != nil {
		return client.IgnoreNotFound(err)
	}
	for _, ingName := range append([]string{app.Name}, appv1alpha1.ChildIngressNames(app.Name)...) {
		ing := &networkingv1.Ingress{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: ingName}, ing); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if ingressRoutesTo(ing, name) {
			// Until the route moves, the proxies pass every request through.
			return r.annotateEdgeCacheTierPods(ctx, app, dep.Annotations[edgecache.RevisionAnnotation])
		}
	}
	return r.deleteStaleChildren(ctx, dep, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}})
}

// ingressRoutesTo reports whether any rule or the default backend of ing
// routes to the Service named svc.
func ingressRoutesTo(ing *networkingv1.Ingress, svc string) bool {
	if b := ing.Spec.DefaultBackend; b != nil && b.Service != nil && b.Service.Name == svc {
		return true
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil && path.Backend.Service.Name == svc {
				return true
			}
		}
	}
	return false
}

// annotateEdgeCacheTierPods hands each live tier pod's proxy the App's last
// purge, its current profile and the release whose deploy last emptied the
// cache (revision), through the pod's own annotations, on every pass. The
// proxy empties its cache on a new purge stamp or revision after its first
// read of them, and a pod created after the purge started empty, so it is
// not handed that stamp.
func (r *AppReconciler) annotateEdgeCacheTierPods(ctx context.Context, app *appv1alpha1.App, revision string) error {
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(app.Namespace), client.MatchingLabels{labelEdgeCacheTier: app.Name}); err != nil {
		return err
	}
	profile, purged := edgeCacheProfileFor(app), app.Annotations[appv1alpha1.CachePurgedAtAnnotation]
	purgedAt, err := time.Parse(time.RFC3339Nano, purged)
	for i := range pods.Items {
		pod := &pods.Items[i]
		// Admission admits the control annotations only on a pod that runs
		// the proxy.
		if edgeCacheSidecar(pod.Spec) == nil || !pod.DeletionTimestamp.IsZero() {
			continue
		}
		want := map[string]string{edgecache.ProfileAnnotation: profile}
		if purged != "" && (err != nil || !pod.CreationTimestamp.After(purgedAt)) {
			want[edgecache.PurgedAtAnnotation] = purged
		}
		if revision != "" {
			want[edgecache.RevisionAnnotation] = revision
		}
		current := true
		for k, v := range want {
			if pod.Annotations[k] != v {
				current = false
			}
		}
		if current {
			continue
		}
		patch := client.MergeFrom(pod.DeepCopy())
		for k, v := range want {
			metav1.SetMetaDataAnnotation(&pod.ObjectMeta, k, v)
		}
		if err := client.IgnoreNotFound(r.Patch(ctx, pod, patch)); err != nil {
			return err
		}
	}
	return nil
}

// edgeCacheSidecar is a pod spec's edge-cache container, nil without one: a
// tier pod's proxy, or the sidecar a template stored before w3/m489 carries.
func edgeCacheSidecar(spec corev1.PodSpec) *corev1.Container {
	for i := range spec.Containers {
		if spec.Containers[i].Name == edgeCacheContainerName {
			return &spec.Containers[i]
		}
	}
	return nil
}

// storedEdgeCache is what a pod template stored before w3/m489 carries for the
// edge-cache sidecar it ran: the container and its control-file volume (none
// before w3/m486). A template from before w3/m486 may carry a purge stamp
// too, which stays among its annotations. applyDeploymentSpec keeps them, as
// stored, while nothing else about the template changes, so the operator
// release that moves caching to the tier rolls no instance; the next change
// for another reason (a deploy, a restart) drops them (projectKeptEdgeCache).
// The sidecar receives no request meanwhile: the App's Services target the
// app itself.
type storedEdgeCache struct {
	sidecar *corev1.Container
	control *corev1.Volume
}

// keptEdgeCache reads stored's edge-cache leftovers, nil when it has none.
func keptEdgeCache(stored corev1.PodTemplateSpec) *storedEdgeCache {
	sidecar := edgeCacheSidecar(stored.Spec)
	_, stamped := stored.Annotations[appv1alpha1.CachePurgedAtAnnotation]
	if sidecar == nil && !stamped {
		return nil
	}
	kept := &storedEdgeCache{}
	if sidecar != nil {
		kept.sidecar = sidecar.DeepCopy()
	}
	for i := range stored.Spec.Volumes {
		if stored.Spec.Volumes[i].Name == edgeCacheControlVolumeName {
			kept.control = stored.Spec.Volumes[i].DeepCopy()
		}
	}
	return kept
}

// projectKeptEdgeCache appends kept's sidecar, after its control volume, to
// tmpl, where the projection of w5/m151 placed them; nil drops the purge
// stamp a template from before w3/m486 carried.
func projectKeptEdgeCache(tmpl *corev1.PodTemplateSpec, kept *storedEdgeCache) {
	if kept == nil {
		delete(tmpl.Annotations, appv1alpha1.CachePurgedAtAnnotation)
		return
	}
	if kept.control != nil {
		tmpl.Spec.Volumes = append(tmpl.Spec.Volumes, *kept.control)
	}
	if kept.sidecar != nil {
		tmpl.Spec.Containers = append(tmpl.Spec.Containers, *kept.sidecar)
	}
}
