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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
)

// podReady reports the pod's PodReady condition. Readiness is what the App and
// KeyValue reconcilers count to decide a rollout has converged — a pod that
// exists but is not PodReady is not yet serving.
func podReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// currentRevisionPodsReady closes the window where a Pod has already crashed
// but the workload controller has not yet lowered its availability counters. A
// missing pod list is treated as inconclusive so startup/fake clients still
// rely on the authoritative Deployment/StatefulSet status; once
// current-revision pods are visible, every desired replica must have
// PodReady=true. Pods carrying no matching revision label are skipped when
// revision is set; an empty revision counts every selected pod. Shared by
// deploymentPodsReady and keyValuePodsReady, which differ only in selector and
// revision-label key.
func currentRevisionPodsReady(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	selector map[string]string,
	revisionKey, revision string,
	replicas int32,
) bool {
	var pods corev1.PodList
	if err := reader.List(ctx, &pods, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return true
	}
	var current, ready int32
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !pod.DeletionTimestamp.IsZero() || (revision != "" && pod.Labels[revisionKey] != revision) {
			continue
		}
		current++
		if podReady(pod) {
			ready++
		}
	}
	return current == 0 || ready >= replicas
}

// Health-check timing. Every value here mirrors a number Render documents, so
// an app Render deploys without complaint also deploys here.
//
// The one deliberate divergence is the *period*: Render pulls an instance from
// rotation after 15s of consecutive failures, which would need a 5s period, and
// we keep Kubernetes' 10s (so 10s × the default failureThreshold of 3 = 30s).
// The probe targets a tenant-supplied path that is frequently an expensive
// server-side render, so halving the interval doubles that cost for every
// service on the platform; and the thing a faster removal buys — shifting
// traffic to a healthy peer — does not exist at replicas: 1, which is the
// common shape. Leave periodSeconds/failureThreshold at Kubernetes' defaults
// unless that trade-off changes.
const (
	// healthCheckTimeoutSeconds is Render's budget: "Render considers a check
	// successful if the instance responds with a 2xx or 3xx status code within
	// five seconds." Kubernetes would otherwise default this to 1s, which no
	// server-side-rendered page reliably meets — it produces a permanently
	// flapping pod rather than an honest failure.
	healthCheckTimeoutSeconds int32 = 5

	// healthCheckPeriodSeconds is how often a probe runs. It is set on the
	// startup and liveness probes; the readiness probe takes the identical
	// Kubernetes default from applyPodSpecServerDefaults.
	healthCheckPeriodSeconds int32 = 10

	// rolloutBudgetSeconds is how long a new pod may take to first report
	// healthy, matching Render: "If this condition is not met within 15
	// minutes, Render cancels the deploy and continues routing traffic to your
	// service's existing instances."
	//
	// THREE timers bound a rollout, in two roles:
	//
	//   mechanism — the startupProbe budget below (periodSeconds ×
	//               failureThreshold) and Deployment.spec.progressDeadlineSeconds,
	//               both derived from this constant so they are one number.
	//   observer  — bex-api's DeployGateTimeout
	//               (lego/backend/internal/store/reconciler.go), which watches
	//               the rollout from the control plane and is deliberately
	//               LONGER (18m), so the mechanism's own specific verdict lands
	//               before the generic health-gate message. That follows the
	//               rule its sibling gates already use (build Jobs 30m → gate
	//               35m, pre-deploy 10m → 12m).
	//
	// The observer lives in another module and cannot share this constant; its
	// comment names this one and a test on each side pins the relationship.
	//
	// Before m80 the three read ∞ / 600s (the Kubernetes default, never set) /
	// 180s, so the real budget was 3 minutes — the accidental minimum of three
	// unrelated choices, none of them Render's.
	rolloutBudgetSeconds int32 = 900

	// healthCheckStartupFailureThreshold spends rolloutBudgetSeconds at
	// healthCheckPeriodSeconds. Derived, never written as a literal, so the
	// budget cannot drift away from progressDeadlineSeconds.
	healthCheckStartupFailureThreshold = rolloutBudgetSeconds / healthCheckPeriodSeconds

	// livenessRestartWindowSeconds is Render's restart stage: a check that
	// keeps failing for 60 seconds gets the instance restarted.
	livenessRestartWindowSeconds int32 = 60

	// healthCheckLivenessFailureThreshold spends livenessRestartWindowSeconds
	// at healthCheckPeriodSeconds. Derived, like the startup threshold, so the
	// window cannot silently drift if the period ever changes. Set on the
	// probe explicitly — unlike readiness, liveness cannot inherit
	// Kubernetes' default threshold of 3 (30s), half the documented window.
	healthCheckLivenessFailureThreshold = livenessRestartWindowSeconds / healthCheckPeriodSeconds
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
	// spec.healthCheckPath — Render's health check. A failure pulls the pod out
	// of the Service until it recovers.
	//
	// Render's ONE health check carries three consequences (gate the deploy,
	// drop from rotation, restart) where Kubernetes has three probes. Loading
	// all of it onto readiness — as this did before m80 — welds boot behavior
	// to steady-state behavior, so neither can be tuned without wrecking the
	// other. Split them:
	//
	//   startupProbe  — owns boot, with Render's full 15-minute budget. While
	//                   it runs kubelet suspends readiness (and liveness),
	//                   which is what makes that budget real.
	//   readinessProbe — owns steady state, strict, once boot has succeeded.
	//   livenessProbe  — Render's restart stage: 60s of consecutive failures
	//                    restarts the container in place.
	//
	// The liveness contract (decided in m81/t001) is unconditional parity —
	// the same rule Render applies, with the same absence of per-service
	// control. The restart-spiral hazard that deferred it from m80 (a service
	// that merely slows under load gets restarted into a cold start, making
	// the next probe slower still) is bounded two ways: kubelet suspends
	// liveness while the startupProbe runs, so a slow boot gets the full 15
	// minutes rather than a kill chain; and in TCP mode the probe is a
	// kernel-level connect that never invokes the tenant's handler, so an
	// expensive SSR route cannot fail it — in HTTP mode the hazard is
	// Render's own documented behavior, carried as parity. The rejected
	// alternative — liveness only on an explicit healthCheckPath — would deny
	// TCP-mode services the restart Render gives them to avoid a hazard that
	// mode does not have, at the price of a mode distinction Render users
	// never have to learn.
	if !p.worker {
		handler := healthCheckHandler(app.Spec.HealthCheckPath, p.port)
		container.StartupProbe = &corev1.Probe{
			ProbeHandler:     handler,
			TimeoutSeconds:   healthCheckTimeoutSeconds,
			PeriodSeconds:    healthCheckPeriodSeconds,
			FailureThreshold: healthCheckStartupFailureThreshold,
		}
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler:   handler,
			TimeoutSeconds: healthCheckTimeoutSeconds,
		}
		container.LivenessProbe = &corev1.Probe{
			ProbeHandler:     handler,
			TimeoutSeconds:   healthCheckTimeoutSeconds,
			PeriodSeconds:    healthCheckPeriodSeconds,
			FailureThreshold: healthCheckLivenessFailureThreshold,
		}
	}
	return container
}

// healthCheckHandler builds the probe handler all three probes share, so they
// can never disagree about what "healthy" means for a given App.
//
// The two modes are Render's. An unset healthCheckPath means the platform has
// not been told what healthy looks like, so it asks only the question it can
// answer honestly — is the process listening — exactly as Render does: "By
// default, health checks are TCP socket probes to one of your service's open
// ports." Setting a path is the opt-in to the stricter HTTP check.
//
// Defaulting the empty case to GET / instead (bex's behavior before m80) is not
// a harmless guess: Kubernetes scores an HTTP probe healthy only on
// 200 <= code < 400, so a service whose root is a legitimate 404 — an API with
// no root route — could never become Ready, and could never deploy, while the
// same service deploys on Render untouched.
func healthCheckHandler(path string, port int) corev1.ProbeHandler {
	if path == "" {
		return corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(port)},
		}
	}
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromInt(port)},
	}
}

// applyDeploymentSpec projects the App onto dep's spec. It is the body of the
// reconciler's CreateOrUpdate mutation, kept separate and free of client calls
// so the whole pod shape is unit-testable.
//
// Every field it sets is rebuilt from the App on each pass, so removing a
// source (a secret file, a start command) drops cleanly out of the pod template
// rather than lingering from a previous revision.
func applyDeploymentSpec(dep *appsv1.Deployment, app *appv1alpha1.App, p deploymentParams) {
	replicas := clampReplicas(p.replicas)
	dep.Spec.Replicas = &replicas
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: app.Name}}
	dep.Spec.Template.Labels = appPodLabels(app, p.verifyImage)
	// Timer (2) of the three named on rolloutBudgetSeconds. Unset, Kubernetes
	// defaults this to 600s, which would cut Render's 15-minute window to 10
	// for reasons nobody chose.
	dep.Spec.ProgressDeadlineSeconds = ptr.To(rolloutBudgetSeconds)

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
		container.VolumeMounts = append(container.VolumeMounts, *mount)
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, *vol)
	}
	// The persistent disk, when one is attached: its volume and mount, plus the
	// Recreate strategy and eviction guard a shared volume forces (see disk.go).
	applyDiskProjection(dep, &container, app)
	dep.Spec.Template.Spec.Containers = []corev1.Container{container}
	// Render's maxShutdownDelaySeconds is Kubernetes' native pod termination
	// grace period. Left nil when unset, so applyPodSpecServerDefaults below
	// writes Kubernetes' own 30-second default — which is the value every
	// existing pod template already carries, so writing it rolls nothing.
	dep.Spec.Template.Spec.TerminationGracePeriodSeconds = terminationGracePeriodSeconds(app.Spec.MaxShutdownDelaySeconds)
	dep.Spec.Template.Spec.ImagePullSecrets = p.pullSecrets
	dep.Spec.Template.Spec.AutomountServiceAccountToken = ptr.To(false)
	// Last, always: above is what bex chooses, this is what Kubernetes would have
	// chosen for the rest. See server_defaults.go.
	applyPodSpecServerDefaults(&dep.Spec.Template.Spec)
}
