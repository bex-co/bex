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
	"net/http"
	"slices"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
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
// registry API host, repository and reference, for an anonymous metadata
// read (w4/m243). No platform credential is ever attached to it.
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
		host = "registry-1.docker.io"
		if !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}
	}
	if repo == "" || reference == "" {
		return "", "", "", "", false
	}
	return host, repo, name, reference, true
}

// resolveImageNetwork is deliberately scoped to the App's own built repository.
// An arbitrary image host can never receive platform registry credentials; a
// web service's prebuilt image is read anonymously instead (w4/m243).
func (r *AppReconciler) resolveImageNetwork(ctx context.Context, app *appv1alpha1.App, image string) (string, int, error) {
	if !app.Spec.HasImagePorts() {
		return image, int(app.Spec.EffectivePort()), nil
	}
	// The configured primary: an explicit port, or a web service's own PORT
	// env (EnvPort), which wins over the image's choice (w4/m243).
	configured := configuredPortMode(app.Spec.PortMode)
	configuredPrimary := app.Spec.Port
	if app.Spec.UsesWebImagePorts() && app.Status.EnvPort > 0 {
		configured, configuredPrimary = true, app.Status.EnvPort
	}
	// A prebuilt image arrives as its tag, which the cache records as Source:
	// the tag is re-resolved once per release, as Render re-pulls it on each
	// deploy, never once per reconcile (w4/m243 t008).
	if cached := app.Status.ImageNetwork; cached != nil &&
		(cached.Image == image || cached.Source == image && cached.Revision == releaseRevision(app)) &&
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
	} else if app.Spec.UsesWebImagePorts() {
		host, repo, repoPath, reference, ok := parseExternalImageReference(image)
		if !ok {
			return "", 0, fmt.Errorf("image port discovery cannot parse image %q", image)
		}
		var err error
		metadata, err = registry.ReadImageConfig(ctx, r.HTTPClient, host, repo, reference, "")
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
		NoDeclaredPorts: len(metadata.TCPPorts) == 0,
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
		ports = make([]corev1.ServicePort, 0, len(network.Ports)+1)
		for _, number := range network.Ports {
			ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("tcp-%d", number), Port: number, TargetPort: intstr.FromInt32(number)})
		}
		// The public route keeps naming the replaced primary until the rollout
		// completes (ingressBackend). A Service without that port makes Traefik
		// drop the route, and the host falls through to the activator's "service
		// not ready" (w4/430), so the old number stays as an alias of the new
		// listener while the old pods drain.
		if old, ok := aliasedPriorPrimary(app); ok {
			ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("tcp-%d", old), Port: old, TargetPort: intstr.FromInt32(network.PrimaryPort)})
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

// replacedImagePorts reports a port change routed early (w4/428) whose old
// pods may still run: until the rollout completes they lack the new
// listener, so the Services keep pinning the now-serving revision.
func replacedImagePorts(app *appv1alpha1.App) bool {
	prior, serving := app.Status.PriorServingNetwork, app.Status.ServingNetwork
	return prior != nil && serving != nil &&
		(prior.PrimaryPort != serving.PrimaryPort || !slices.Equal(prior.Ports, serving.Ports))
}

// aliasedPriorPrimary is the replaced primary port of a port change routed
// early, while the new network no longer declares it: the Services alias it to
// the new listener and the public route keeps naming it until the rollout
// completes (w4/430). A replaced primary the new image still declares is a
// real listener of another role, so the route moves at once instead.
func aliasedPriorPrimary(app *appv1alpha1.App) (int32, bool) {
	if !replacedImagePorts(app) {
		return 0, false
	}
	old := app.Status.PriorServingNetwork.PrimaryPort
	return old, !slices.Contains(app.Status.ServingNetwork.Ports, old)
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
func (r *AppReconciler) promotePortChangeEarly(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, image string, port int) (moved bool, err error) {
	if !app.Spec.HasImagePorts() || !changesImagePorts(app) || !r.currentRevisionFullyReady(ctx, dep, 1) {
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
