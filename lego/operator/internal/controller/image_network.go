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
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ownImageReference is image's tag or digest within the App's own built
// repository; ok is false for any other image, which can never receive
// platform registry credentials.
func (r *AppReconciler) ownImageReference(app *appv1alpha1.App, image string) (string, bool) {
	prefix := r.Registry + "/" + appIdentity(app).Repo()
	if r.Registry == "" || !strings.HasPrefix(image, prefix+":") && !strings.HasPrefix(image, prefix+"@") {
		return "", false
	}
	reference := strings.TrimPrefix(image, prefix)
	if _, digest, ok := strings.Cut(reference, "@"); ok {
		return digest, true
	}
	return strings.TrimPrefix(reference, ":"), true
}

// imagePortPreferred is the declared image port chosen as primary when present.
const imagePortPreferred int32 = 3000

func configuredPortMode(mode string) bool {
	return mode == appv1alpha1.PortModeImageConfiguredV1 || mode == appv1alpha1.PortModeWebImageConfiguredV1
}

// parseExternalImageReference splits a prebuilt image reference into the
// registry API host, repository and reference for its metadata read
// (w4/m243). No platform credential is ever attached to it.
func parseExternalImageReference(image string) (host, repo, repoPath, reference string, ok bool) {
	name := image
	reference = "latest"
	if before, digest, found := strings.Cut(name, "@"); found {
		name, reference = before, digest
	} else if i := strings.LastIndex(name, ":"); i > strings.LastIndex(name, "/") {
		name, reference = name[:i], name[i+1:]
	}
	parts := strings.Split(name, "/")
	host = "docker.io"
	if len(parts) > 1 && (strings.ContainsAny(parts[0], ".:") || parts[0] == "localhost") {
		host, parts = parts[0], parts[1:]
	}
	repo = strings.Join(parts, "/")
	if host == "docker.io" {
		host = dockerHubAPIHost
		if !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}
	}
	if repo == "" || reference == "" {
		return "", "", "", "", false
	}
	return host, repo, name, reference, true
}

// externalImageAuthHeader is the Basic header of the workspace registry
// credential bound to this App (spec.externalRegistryPullSecret, the same
// Secret kubelet pulls with) for the image's host, or "" for an anonymous read
// when it holds none for that host, as kubelet would pull. Docker Hub's entry
// may be keyed by any of its aliases rather than the API host.
func (r *AppReconciler) externalImageAuthHeader(ctx context.Context, app *appv1alpha1.App, host string) (string, error) {
	name := app.Spec.ExternalRegistryPullSecret
	if name == "" {
		return "", nil
	}
	var sec corev1.Secret
	if err := r.uncachedSecretClient().Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, &sec); err != nil {
		return "", fmt.Errorf("read registry credential Secret %s: %w", name, err)
	}
	auths, err := dockerConfigAuths(sec.Data)
	if err != nil {
		return "", fmt.Errorf("registry credential Secret %s: %w", name, err)
	}
	keys := []string{host}
	if host == dockerHubAPIHost {
		keys = dockerHubConfigKeys
	}
	for _, key := range keys {
		if _, ok := auths[key]; ok {
			header, err := dockerConfigEntryHeader(auths, key)
			if err != nil {
				return "", fmt.Errorf("registry credential Secret %s: %w", name, err)
			}
			return header, nil
		}
	}
	return "", nil
}

// dockerHubAPIHost is the registry API host parseExternalImageReference
// resolves Docker Hub images to; dockerHubConfigKeys are the docker config
// keys a Docker Hub credential may be stored under.
const dockerHubAPIHost = "registry-1.docker.io"

var dockerHubConfigKeys = []string{dockerHubAPIHost, "docker.io", "index.docker.io", "https://index.docker.io/v1/"}

// resolveImageNetwork is deliberately scoped to the App's own built repository.
// An arbitrary image host can never receive platform registry credentials; a
// prebuilt image (web, w4/m243; private, w6/095) is read anonymously, or with
// the workspace registry credential bound to this App for that image's host.
func (r *AppReconciler) resolveImageNetwork(ctx context.Context, app *appv1alpha1.App, image string) (string, int, error) {
	if !app.Spec.HasImagePorts() {
		return image, int(app.Spec.EffectivePort()), nil
	}
	// The configured primary: an explicit port, or a web service's own PORT
	// env (EnvPort), which wins over the image's choice (w4/m243).
	configuredPrimary, configured := app.ConfiguredImagePort()
	configuredPort := int32(0)
	if configured {
		configuredPort = configuredPrimary
	}
	// A prebuilt image arrives as its tag, which the cache records as Source:
	// the tag is re-resolved once per release, as Render re-pulls it on each
	// deploy, never once per reconcile (w4/m243 t008).
	// A network resolved from a PORT that is now removed is never reused: a
	// rollback or redeploy returns to the image's own port (w8/210).
	if cached := app.Status.ImageNetwork; cached != nil &&
		(cached.Image == image || cached.Source == image && cached.Revision == releaseRevision(app)) &&
		cached.ConfiguredPort != nil && *cached.ConfiguredPort == configuredPort &&
		(!configured || cached.PrimaryPort == configuredPrimary) {
		cached.Revision = releaseRevision(app)
		retainWebHTTPSelection(app, cached, configured)
		return cached.Image, int(cached.PrimaryPort), nil
	}
	source := image
	var metadata registry.ImageConfig
	var prefix string
	if reference, ok := r.ownImageReference(app, image); ok {
		prefix = r.Registry + "/" + appIdentity(app).Repo()
		authorization, ready, err := r.registryAuthHeader(ctx, app)
		if err != nil {
			return "", 0, err
		}
		if !ready {
			return "", 0, fmt.Errorf("image registry credential is not active yet")
		}
		var err2 error
		metadata, err2 = registry.ReadImageConfig(ctx, r.HTTPClient, r.Registry, appIdentity(app).Repo(), reference, authorization)
		if err2 != nil {
			return "", 0, err2
		}
	} else if app.Spec.Image != "" {
		host, repo, repoPath, reference, ok := parseExternalImageReference(image)
		if !ok {
			return "", 0, fmt.Errorf("image port discovery cannot parse image %q", image)
		}
		authorization, err := r.externalImageAuthHeader(ctx, app, host)
		if err != nil {
			return "", 0, err
		}
		metadata, err = registry.ReadImageConfig(ctx, r.HTTPClient, host, repo, reference, authorization)
		if err != nil {
			return "", 0, err
		}
		prefix = repoPath
	} else {
		return "", 0, fmt.Errorf("image port discovery requires this service's built image repository")
	}
	ports := metadata.TCPPorts
	if len(ports) == 0 {
		ports = []int32{app.Spec.EffectivePort()}
		if configured {
			ports = []int32{configuredPrimary}
		}
	}
	primary := ports[0]
	// A declared 3000 stays the preferred primary (ADR089): the services that
	// rely on it predate the Render PORT default.
	if slices.Contains(ports, imagePortPreferred) {
		primary = imagePortPreferred
	}
	if configured {
		primary = configuredPrimary
		if primary < 1 || primary > 65535 || appv1alpha1.IsReservedImagePort(primary) {
			return "", 0, fmt.Errorf("configured primary port is unsupported")
		}
		if !slices.Contains(ports, primary) {
			ports = append(slices.Clone(ports), primary)
			slices.Sort(ports)
		}
		if len(ports) > appv1alpha1.MaxImagePorts {
			return "", 0, fmt.Errorf("services support at most %d TCP ports including the configured primary", appv1alpha1.MaxImagePorts)
		}
	}
	image = prefix + "@" + metadata.Digest
	network := &appv1alpha1.ImageNetworkStatus{
		Image: image, Source: source, Revision: releaseRevision(app), PrimaryPort: primary, Ports: ports,
		NoDeclaredPorts: len(metadata.TCPPorts) == 0, ConfiguredPort: new(configuredPort),
	}
	retainWebHTTPSelection(app, network, configured)
	app.Status.ImageNetwork = network
	primary = network.PrimaryPort
	// Future reconciles reuse exactly the manifest whose metadata was verified.
	app.Status.ArtifactImage = image
	return image, int(primary), nil
}

// privateServiceProjection is the selector and ports of both stable serving
// Services. The selector takes the runtime pod template's labelApp and
// labelAppID: Job pods (pre-deploy, build, disk backup) carry labelApp alone,
// and must never receive service traffic (w4/m181). Unlike the Deployment's,
// a Service selector is mutable.
func privateServiceProjection(app *appv1alpha1.App, port int) (map[string]string, []corev1.ServicePort, error) {
	pod := appPodLabels(app, false)
	selector := map[string]string{labelApp: pod[labelApp], labelAppID: pod[labelAppID]}
	ports := []corev1.ServicePort{{Port: int32(port), TargetPort: intstr.FromInt(port)}}
	if app.Spec.HasImagePorts() {
		network := app.ActiveImageNetwork()
		if network == nil || len(network.Ports) == 0 || network.Revision == "" {
			return nil, nil, fmt.Errorf("image ports have not been resolved")
		}
		// A port change must not route to old replicas that lack the new
		// listener, so the Services pin the serving revision until promotion.
		// Every other rollout (a deploy, an env save, a restart) keeps the same
		// ports, and a pin there strands the Services on pods the Deployment is
		// draining: no ready endpoint until promotion (w4/m243 t012).
		if changesImagePorts(app) || replacedImagePorts(app) {
			selector[labelRevision] = network.Revision
		}
		alias, aliased := aliasedPriorPrimary(app)
		ports = make([]corev1.ServicePort, 0, len(network.Ports)+1)
		for _, number := range network.Ports {
			target := number
			if aliased && number == alias {
				target = network.PrimaryPort
			}
			ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("tcp-%d", number), Port: number, TargetPort: intstr.FromInt32(target)})
		}
		// The public route keeps naming the replaced primary until the rollout
		// completes (ingressBackend). A Service without that port makes Traefik
		// drop the route, and the host falls through to the activator's "service
		// not ready" (w4/430), so the old number stays as an alias of the new
		// listener while the old pods drain.
		if aliased && !slices.Contains(network.Ports, alias) {
			ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("tcp-%d", alias), Port: alias, TargetPort: intstr.FromInt32(network.PrimaryPort)})
		}
	}
	applyServicePortServerDefaults(ports)
	return selector, ports, nil
}

// changesImagePorts reports a candidate image network whose ports differ from
// the serving one's: the only rollout whose new pods may lack a listener the
// old pods serve, or the reverse.
func changesImagePorts(app *appv1alpha1.App) bool {
	serving, candidate := app.Status.ServingNetwork, app.Status.ImageNetwork
	return serving != nil && candidate != nil &&
		(serving.PrimaryPort != candidate.PrimaryPort || !slices.Equal(serving.Ports, candidate.Ports))
}

// portChangeInFlight reports an image-port rollout whose candidate's ports
// differ from the serving ones: the only rollout that routes early.
func portChangeInFlight(app *appv1alpha1.App) bool {
	return app.Spec.HasImagePorts() && changesImagePorts(app)
}

// replacedImagePorts reports a port change routed early (w4/428) whose old
// pods may still run: until the rollout completes they lack the new
// listener, so the Services keep pinning the now-serving revision.
func replacedImagePorts(app *appv1alpha1.App) bool {
	prior, serving := app.Status.PriorServingNetwork, app.Status.ServingNetwork
	return prior != nil && serving != nil &&
		(prior.PrimaryPort != serving.PrimaryPort || !slices.Equal(prior.Ports, serving.Ports))
}

// aliasedPriorPrimary is the replaced primary port of a port change routed
// early: until the rollout completes, the Services point it at the new
// listener and the public route keeps naming it (w4/430). It is aliased even
// when the new image still declares it, because a declared port is not a
// listener: an image that EXPOSEs 8080 and is told PORT=9090 listens on 9090
// alone, and a route left on 8080 reached the new pod's closed port (502).
func aliasedPriorPrimary(app *appv1alpha1.App) (int32, bool) {
	if !replacedImagePorts(app) {
		return 0, false
	}
	old := app.Status.PriorServingNetwork.PrimaryPort
	return old, old != app.Status.ServingNetwork.PrimaryPort
}

// portChangeHoldSeconds keeps a port-changing rollout's old pods serving after
// a new pod is Ready (the Deployment's minReadySeconds), long enough for the
// operator to move the Services and Ingress to the new pod before the
// Deployment drains the old one (w4/428). Every other rollout keeps 0.
const portChangeHoldSeconds int32 = 15

// promoteImageNetwork runs only after readiness of the candidate revision.
// Both stable hostnames must converge before status publishes the new address.
// moved reports that the serving network changed, so a public route that named
// the old primary must follow it.
func (r *AppReconciler) promoteImageNetwork(ctx context.Context, app *appv1alpha1.App, image string, port int) (moved bool, err error) {
	if !app.Spec.HasImagePorts() {
		return false, nil
	}
	candidate := app.Status.ImageNetwork
	if candidate == nil || candidate.Image != image || candidate.Revision != releaseRevision(app) {
		return false, fmt.Errorf("ready release has no matching image network metadata")
	}
	// Already serving: this reconcile's earlier passes projected both Services
	// from ServingNetwork, so re-applying them would be a no-op round trip.
	if equality.Semantic.DeepEqual(app.Status.ServingNetwork, candidate) {
		return false, nil
	}
	// Project against a copy: a failed Service write must not let fail() persist
	// candidate metadata as the active network. Service writes are sequential,
	// so a partial failure is retried until both hostnames have converged.
	projected := app.DeepCopy()
	projected.Status.ServingNetwork = candidate.DeepCopy()
	if err := r.applyClusterIPService(ctx, projected, app.Name, port); err != nil {
		return false, err
	}
	if err := r.reconcileSlugService(ctx, projected, port); err != nil {
		return false, err
	}
	moved = app.Status.ServingNetwork == nil || app.Status.ServingNetwork.PrimaryPort != candidate.PrimaryPort
	app.Status.ServingNetwork = candidate.DeepCopy()
	return moved, nil
}

// promotePortChangeEarly moves a port-changing rollout's routing to its first
// ready candidate pod, while portChangeHoldSeconds keeps the old pods
// serving, so the stable Services always have a ready endpoint on the port
// they publish (w4/428). It records the replaced network for the old pods'
// pin and for a failed rollout's restore. moved is promoteImageNetwork's.
// The caller checks portChangeInFlight first.
func (r *AppReconciler) promotePortChangeEarly(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, tmpl *corev1.PodTemplateSpec, image string, port int) (moved bool, err error) {
	if !r.templatePodReady(ctx, dep, tmpl) {
		return false, nil
	}
	// Record the replaced network before projecting: the very first Service
	// write must already pin the new revision and alias the old primary, or
	// it selects old pods on a port they lack (w4/430).
	held := app.Status.PriorServingNetwork
	app.Status.PriorServingNetwork = app.Status.ServingNetwork.DeepCopy()
	if moved, err = r.promoteImageNetwork(ctx, app, image, port); err != nil {
		app.Status.PriorServingNetwork = held
		return false, err
	}
	return moved, nil
}

// templatePodReady reports a Ready, non-terminating pod of tmpl, the pod
// template this pass wrote, rather than merely of its release. A release can
// roll its template more than once (choosing the HTTP port swaps discovery's
// probe on / for the final TCP probe), and the Deployment controller drains
// the replaced template's pod as soon as the new ReplicaSet exists, leaving
// the release with no endpoint until the new pod is Ready (w8/m142 t006). The
// pods are found through the ReplicaSet created for tmpl; until it exists,
// none is Ready.
func (r *AppReconciler) templatePodReady(ctx context.Context, dep *appsv1.Deployment, tmpl *corev1.PodTemplateSpec) bool {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return false
	}
	var rss appsv1.ReplicaSetList
	if err := r.List(ctx, &rss, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return false
	}
	hash := ""
	for i := range rss.Items {
		rs := &rss.Items[i]
		h := rs.Spec.Template.Labels[appsv1.DefaultDeploymentUniqueLabelKey]
		if h == "" || rs.Spec.Template.Labels[labelRevision] != tmpl.Labels[labelRevision] || !metav1.IsControlledBy(rs, dep) {
			continue
		}
		// The Deployment controller's own match: the template it copied, plus
		// the pod-template-hash label it adds.
		candidate := rs.Spec.Template.DeepCopy()
		delete(candidate.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
		if equality.Semantic.DeepEqual(*candidate, *tmpl) {
			hash = h
			break
		}
	}
	if hash == "" {
		return false
	}
	selector := maps.Clone(dep.Spec.Selector.MatchLabels)
	selector[appsv1.DefaultDeploymentUniqueLabelKey] = hash
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(selector)); err != nil {
		return false
	}
	for i := range pods.Items {
		if pod := &pods.Items[i]; pod.DeletionTimestamp.IsZero() && podReady(pod) {
			return true
		}
	}
	return false
}

// imageRegistryWait keeps a deploy open while the image registry throttles or
// fails a metadata read: a 429, 5xx or network error is transient, and Render
// retries the pull rather than failing the deploy (w4/m243 t008).
func (r *AppReconciler) imageRegistryWait(ctx context.Context, app *appv1alpha1.App, err error) (ctrl.Result, bool) {
	var transient *registry.TransientError
	if !errors.As(err, &transient) {
		return ctrl.Result{}, false
	}
	logf.FromContext(ctx).Info("image registry read failed; retrying", "app", app.Name, "error", err.Error())
	reason, msg := reasonImageRegistryUnavailable, "Waiting for the image registry to answer the image metadata read; bex retries it automatically"
	if transient.Status == http.StatusTooManyRequests {
		reason, msg = reasonImageRegistryRateLimited, "Waiting for the image registry: it rate-limited the image metadata read (HTTP 429); bex retries it automatically"
	}
	r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, reason, msg)
	return ctrl.Result{RequeueAfter: min(max(transient.RetryAfter, imageRegistryRetry), imageRegistryRetryMax)}, true
}

// imageRegistryRetry and imageRegistryRetryMax bound a throttled read's
// requeue, whatever Retry-After the registry names.
const (
	imageRegistryRetry    = 30 * time.Second
	imageRegistryRetryMax = 5 * time.Minute
)
