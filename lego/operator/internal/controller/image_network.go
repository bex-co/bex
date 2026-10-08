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

// resolveImageNetwork is deliberately scoped to the App's own built repository.
// An arbitrary image host can never receive platform registry credentials.
func (r *AppReconciler) resolveImageNetwork(ctx context.Context, app *appv1alpha1.App, image string) (string, int, error) {
	if !app.Spec.UsesImagePorts() {
		return image, int(app.Spec.EffectivePort()), nil
	}
	if cached := app.Status.ImageNetwork; cached != nil && cached.Image == image &&
		(app.Spec.PortMode != appv1alpha1.PortModeImageConfiguredV1 || cached.PrimaryPort == app.Spec.Port) {
		cached.Revision = releaseRevision(app)
		return image, int(cached.PrimaryPort), nil
	}
	prefix := r.Registry + "/" + appIdentity(app).Repo()
	if r.Registry == "" || !strings.HasPrefix(image, prefix+":") && !strings.HasPrefix(image, prefix+"@") {
		return "", 0, fmt.Errorf("image port discovery requires this service's built image repository")
	}
	reference := strings.TrimPrefix(image, prefix)
	if _, digest, ok := strings.Cut(reference, "@"); ok {
		reference = digest
	} else {
		reference = strings.TrimPrefix(reference, ":")
	}
	authorization, ready, err := r.registryAuthHeader(ctx, app)
	if err != nil {
		return "", 0, err
	}
	if !ready {
		return "", 0, fmt.Errorf("image registry credential is not active yet")
	}
	metadata, err := registry.ReadImageConfig(ctx, r.HTTPClient, r.Registry, appIdentity(app).Repo(), reference, authorization)
	if err != nil {
		return "", 0, err
	}
	ports := metadata.TCPPorts
	if len(ports) == 0 {
		ports = []int32{appv1alpha1.DefaultPort}
		if app.Spec.PortMode == appv1alpha1.PortModeImageConfiguredV1 {
			ports = []int32{app.Spec.Port}
		}
	}
	primary := ports[0]
	if slices.Contains(ports, int32(appv1alpha1.DefaultPort)) {
		primary = appv1alpha1.DefaultPort
	}
	if app.Spec.PortMode == appv1alpha1.PortModeImageConfiguredV1 {
		primary = app.Spec.Port
		if primary < 1024 || primary > 65535 || appv1alpha1.IsReservedImagePort(primary) {
			return "", 0, fmt.Errorf("configured private primary port is unsupported")
		}
		if !slices.Contains(ports, primary) {
			ports = append(slices.Clone(ports), primary)
			slices.Sort(ports)
		}
		if len(ports) > appv1alpha1.MaxImagePorts {
			return "", 0, fmt.Errorf("private services support at most %d TCP ports including the configured primary", appv1alpha1.MaxImagePorts)
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
	if app.Spec.UsesImagePorts() {
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
	if !app.Spec.UsesImagePorts() {
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
