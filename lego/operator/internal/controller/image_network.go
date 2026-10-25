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
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"

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
	if cached := app.Status.ImageNetwork; cached != nil && cached.Image == image &&
		(!configured || cached.PrimaryPort == configuredPrimary) {
		cached.Revision = releaseRevision(app)
		return image, int(cached.PrimaryPort), nil
	}
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
	app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: image, Revision: releaseRevision(app), PrimaryPort: primary, Ports: ports}
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
		// A port change must not route to old replicas that lack the new listener.
		selector[labelRevision] = network.Revision
		ports = make([]corev1.ServicePort, 0, len(network.Ports))
		for _, number := range network.Ports {
			ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("tcp-%d", number), Port: number, TargetPort: intstr.FromInt32(number)})
		}
	}
	applyServicePortServerDefaults(ports)
	return selector, ports, nil
}

// promoteImageNetwork runs only after readiness of the candidate revision.
// Both stable hostnames must converge before status publishes the new address.
func (r *AppReconciler) promoteImageNetwork(ctx context.Context, app *appv1alpha1.App, image string, port int) error {
	if !app.Spec.HasImagePorts() {
		return nil
	}
	candidate := app.Status.ImageNetwork
	if candidate == nil || candidate.Image != image || candidate.Revision != releaseRevision(app) {
		return fmt.Errorf("ready release has no matching image network metadata")
	}
	// Already serving: this reconcile's earlier passes projected both Services
	// from ServingNetwork, so re-applying them would be a no-op round trip.
	if equality.Semantic.DeepEqual(app.Status.ServingNetwork, candidate) {
		return nil
	}
	// Project against a copy: a failed Service write must not let fail() persist
	// candidate metadata as the active network. Service writes are sequential,
	// so a partial failure is retried until both hostnames have converged.
	projected := app.DeepCopy()
	projected.Status.ServingNetwork = candidate.DeepCopy()
	if err := r.applyClusterIPService(ctx, projected, app.Name, port); err != nil {
		return err
	}
	if err := r.reconcileSlugService(ctx, projected, port); err != nil {
		return err
	}
	app.Status.ServingNetwork = candidate.DeepCopy()
	return nil
}
