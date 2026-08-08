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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
)

// deploymentParams bundles the inputs the Deployment projection needs beyond
// the App itself — the reconcile-time decisions (which image won, how many
// replicas the autoscaler settled on) and the reconciler's own configuration.
// Kept as a struct, like clusterParams in database_controller.go, so the
// projection stays a pure function the tests can drive without a cluster.
type deploymentParams struct {
	image    string
	port     int
	replicas int32
	// worker is a background_worker: it runs the image with no HTTP port, so it
	// declares no container port and gets no readiness probe.
	worker bool
	// verifyImage stamps the admission webhook's opt-in label
	// (BEX_TENANT_SIGNING_KEY_SECRET).
	verifyImage bool
	pullSecrets []corev1.LocalObjectReference
}

// appPodLabels builds the pod template's label set.
//
// The Deployment *selector* stays labelApp-only and must not gain these: a
// selector is immutable, so adding a label here that also entered the selector
// would break every existing Deployment the moment the label appeared.
func appPodLabels(app *appv1alpha1.App, verifyImage bool) map[string]string {
	appID := app.Labels[labelAppID]
	if appID == "" {
		appID = app.Name
	}
	labels := map[string]string{
		labelApp:      app.Name,
		labelAppID:    appID,
		labelRevision: releaseRevision(app),
	}
	// Propagated so NetworkPolicy selectors can express "allow same-workspace".
	if ws := app.Labels[labelWorkspace]; ws != "" {
		labels[labelWorkspace] = ws
	}
	if verifyImage {
		labels[execution.LabelVerifyImage] = execution.VerifyImageEnabled
	}
	// Only present when the App's Environment has networkIsolationEnabled
	// (w6/m19); reconcileNetworkPolicy's scopeSelector needs it on the pod
	// template to actually select these pods.
	if env := app.Labels[labelNetworkIsolation]; env != "" {
		labels[labelNetworkIsolation] = env
	}
	return labels
}

// appContainer projects the App onto its single "app" container.
func appContainer(app *appv1alpha1.App, p deploymentParams) corev1.Container {
	var ports []corev1.ContainerPort
	if !p.worker {
		ports = []corev1.ContainerPort{{ContainerPort: int32(p.port)}}
	}
	container := corev1.Container{
		Name:            "app",
		Image:           p.image,
		ImagePullPolicy: pullPolicyFor(p.image),
		Env:             appEnv(app, p.port),
		EnvFrom:         envFromSources(app),
		Ports:           ports,
		Resources:       resourcesForTier(app.Spec.Tier),
		SecurityContext: tenantSecCtx(),
	}
	// StartCommand overrides the running container's entrypoint whenever the
	// image comes from an opaque Dockerfile (or a prebuilt image) — bex has no
	// control over that image's own ENTRYPOINT/CMD. A native-runtime or
	// buildpack build instead bakes the command into the image's own CMD at
	// build time, so no Deployment-level override is needed there.
	if b := effectiveBuilder(app.Spec); b != build.BuilderNative && b != build.BuilderBuildpack && app.Spec.StartCommand != "" {
		container.Command = []string{"/bin/sh", "-c", app.Spec.StartCommand}
	}
	// Health-gating: a non-worker service speaks HTTP, so gate pod readiness on
	// GET spec.healthCheckPath — Render's health check. A 2xx/3xx (k8s' default
	// success range) makes the pod ready and routes traffic; a failure pulls it
	// out of the Service until it recovers. Empty defaults to "/", matching the
	// CRD's +kubebuilder:default.
	if !p.worker {
		path := app.Spec.HealthCheckPath
		if path == "" {
			path = "/"
		}
		container.ReadinessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromInt(p.port)},
		}}
	}
	return container
}

// applyDeploymentSpec projects the App onto dep's spec. It is the body of the
// reconciler's CreateOrUpdate mutation, kept separate and free of client calls
// so the whole pod shape is unit-testable.
//
// Every field it sets is rebuilt from the App on each pass, so removing a
// source (a secret file, a start command) drops cleanly out of the pod template
// rather than lingering from a previous revision.
func applyDeploymentSpec(dep *appsv1.Deployment, app *appv1alpha1.App, p deploymentParams) {
	dep.Spec.Replicas = &p.replicas
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: app.Name}}
	dep.Spec.Template.Labels = appPodLabels(app, p.verifyImage)

	// restart = roll the template (same mechanism as kubectl rollout restart,
	// recorded in the contract). Never removed once set — removal would itself
	// roll the pods again.
	if app.Spec.RestartedAt != "" {
		if dep.Spec.Template.Annotations == nil {
			dep.Spec.Template.Annotations = map[string]string{}
		}
		dep.Spec.Template.Annotations["app.bex.co/restarted-at"] = app.Spec.RestartedAt
	}

	container := appContainer(app, p)
	// Secret files: one projected /etc/secrets volume (the service's own
	// "<name>-files" + each linked env group's files).
	dep.Spec.Template.Spec.Volumes = nil
	if vol, mount := secretFileMounts(app); vol != nil {
		container.VolumeMounts = []corev1.VolumeMount{*mount}
		dep.Spec.Template.Spec.Volumes = []corev1.Volume{*vol}
	}
	dep.Spec.Template.Spec.Containers = []corev1.Container{container}
	// Render's maxShutdownDelaySeconds is Kubernetes' native pod termination
	// grace period. Keep nil when unset so existing Apps retain Kubernetes'
	// identical 30-second default without adding a field to their pod template.
	dep.Spec.Template.Spec.TerminationGracePeriodSeconds = terminationGracePeriodSeconds(app.Spec.MaxShutdownDelaySeconds)
	dep.Spec.Template.Spec.ImagePullSecrets = p.pullSecrets
	dep.Spec.Template.Spec.AutomountServiceAccountToken = ptr(false)
}
