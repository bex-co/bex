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
	"cmp"
	"context"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
// Render pulls an instance from rotation after 15s of consecutive failed
// checks, so the readiness probe runs every 5s and fails after 3 (w8/189; it
// kept Kubernetes' 10s period, a 30s removal, until then). Startup and liveness
// stay on a 10s period: they spend the 15-minute rollout and 60-second restart
// budgets, which their derived thresholds keep exact. Existing Deployments
// adopt the 5s readiness period on the pass that changes their template for
// another reason, not on operator upgrade (applyDeploymentSpec).
const (
	// healthCheckTimeoutSeconds is Render's budget: "Render considers a check
	// successful if the instance responds with a 2xx or 3xx status code within
	// five seconds." Kubernetes would otherwise default this to 1s, which no
	// server-side-rendered page reliably meets — it produces a permanently
	// flapping pod rather than an honest failure.
	healthCheckTimeoutSeconds int32 = 5

	// healthCheckPeriodSeconds is how often the startup and liveness probes run.
	healthCheckPeriodSeconds int32 = 10

	// healthCheckReadinessPeriodSeconds is how often the readiness probe runs.
	// With Kubernetes' default failureThreshold of 3 an instance leaves rotation
	// after 15s of consecutive failures, as on Render (w8/189).
	healthCheckReadinessPeriodSeconds int32 = 5
	// healthCheckReadinessFailureThreshold is Kubernetes' default, set explicitly
	// so the 15s removal cannot drift with it.
	healthCheckReadinessFailureThreshold int32 = 3

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
	// baseDomain is the reconciler's platform domain: the probe Host's
	// fallback (healthProbeHost).
	baseDomain string
	// launch is the selected or served release's recorded launch, if any.
	launch launchMode
	// webConcurrency is the WEB_CONCURRENCY default, 0 for none
	// (web_concurrency.go).
	webConcurrency int
	// userNamespaces is BEX_RENDER_V1_USER_NAMESPACES (appHostUsers).
	userNamespaces bool
	// platform carries the Render default variables' inputs that do not come
	// from the App (platform_env.go).
	platform platformEnvInputs
	// edgeCacheImage runs the edge-cache sidecar; "" for none (edge_cache.go).
	edgeCacheImage string
	// healthProbeImage is the image the no-redirect health probe is installed
	// from; "" probes with the kubelet's HTTP check instead (health_probe.go).
	healthProbeImage string
	// readinessPeriod overrides the readiness probe period while a stored
	// template still carries its previous one (applyDeploymentSpec); 0 = current.
	readinessPeriod int32
}

// usesHealthProbeBinary reports whether the App's HTTP health check runs as the
// no-redirect exec probe: a web or private service with a Health Check Path,
// on an operator that knows its own image.
func (p deploymentParams) usesHealthProbeBinary(app *appv1alpha1.App) bool {
	return !p.worker && app.Spec.HealthCheckPath != "" && p.healthProbeImage != ""
}

// appPodLabels builds the pod template's label set.
//
// The Deployment *selector* stays labelApp-only and must not gain these: a
// selector is immutable, so adding a label here that also entered the selector
// would break every existing Deployment the moment the label appeared.
func appPodLabels(app *appv1alpha1.App, verifyImage bool) map[string]string {
	labels := map[string]string{
		labelApp:      app.Name,
		labelAppID:    appIDOrName(app),
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
	return appPolicyLabels(app, labels)
}

const appContainerName = appv1alpha1.ReleaseRecordAppContainer

// servingPortName names a configured-port App's listener on its pods and on
// both stable Services.
const servingPortName = "bex-http"

// launchMode is how a release's app container launches the saved StartCommand.
type launchMode string

const (
	launchUnknown launchMode = ""      // no recorded evidence: the saved source decides
	launchShell   launchMode = "shell" // the app container runs it through the shell
	launchImage   launchMode = "image" // the image's own entrypoint, which a native/buildpack build bakes it into
)

// shellRunsStartCommand reports whether the app container running image must
// launch the saved StartCommand itself (the Docker Command override) rather
// than keep the image's own entrypoint. A recorded launch decides first (a
// release whose artifact predates the saved source: selectedRuntime). Then the
// saved prebuilt image actually being run gets the override whatever build
// settings it retains from a native source (w4/m202). Otherwise the builder
// decides: bex has no control over an opaque Dockerfile image's ENTRYPOINT/CMD,
// while a native or buildpack build bakes the command into its image's CMD.
func shellRunsStartCommand(app *appv1alpha1.App, image string, recorded launchMode) bool {
	switch recorded {
	case launchShell:
		return true
	case launchImage:
		return false
	}
	if app.Spec.Repo == "" && app.Spec.Image != "" && image == app.Spec.Image {
		return true
	}
	b := effectiveBuilder(app.Spec)
	return b != build.BuilderNative && b != build.BuilderBuildpack
}

// appContainerEnv is the app container's env: the user's literals, Render's
// platform set, the WEB_CONCURRENCY default, then PORT.
func appContainerEnv(app *appv1alpha1.App, p deploymentParams) []corev1.EnvVar {
	in := p.platform
	in.baseDomain = p.baseDomain
	return withWebConcurrency(withPlatformEnv(appEnv(app, p.port), platformEnv(app, in)), p.webConcurrency)
}

// appContainer projects the App onto its single "app" container.
// probeHost is the Host header its HTTP health probes send ("" for none).
func appContainer(app *appv1alpha1.App, p deploymentParams, probeHost string) corev1.Container {
	var ports []corev1.ContainerPort
	if !p.worker {
		ports = []corev1.ContainerPort{{Name: servingPortName, ContainerPort: int32(p.port)}}
		if app.Spec.HasImagePorts() && app.Status.ImageNetwork != nil {
			ports = make([]corev1.ContainerPort, 0, len(app.Status.ImageNetwork.Ports))
			for _, port := range app.Status.ImageNetwork.Ports {
				ports = append(ports, corev1.ContainerPort{ContainerPort: port, Protocol: corev1.ProtocolTCP})
			}
		}
	}
	container := corev1.Container{
		Name:            appContainerName,
		Image:           p.image,
		ImagePullPolicy: pullPolicyFor(p.image),
		Env:             appContainerEnv(app, p),
		EnvFrom:         envFromSources(app),
		Ports:           ports,
		Resources:       resourcesForTier(app.Spec.Tier),
		SecurityContext: appSecCtx(app.Spec),
	}
	if app.Spec.StartCommand != "" && shellRunsStartCommand(app, p.image, p.launch) {
		container.Command = shellCommand(app.Spec.StartCommand)
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
		handler := healthCheckHandler(app.Spec.HealthCheckPath, p.port, probeHost)
		if p.usesHealthProbeBinary(app) {
			handler = healthProbeExecHandler(app.Spec.HealthCheckPath, p.port, probeHost)
			container.VolumeMounts = append(container.VolumeMounts,
				corev1.VolumeMount{Name: healthProbeVolumeName, MountPath: healthProbeMountPath, ReadOnly: true})
		}
		container.StartupProbe = &corev1.Probe{
			ProbeHandler:     handler,
			TimeoutSeconds:   healthCheckTimeoutSeconds,
			PeriodSeconds:    healthCheckPeriodSeconds,
			FailureThreshold: healthCheckStartupFailureThreshold,
		}
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler:     handler,
			TimeoutSeconds:   healthCheckTimeoutSeconds,
			PeriodSeconds:    cmp.Or(p.readinessPeriod, healthCheckReadinessPeriodSeconds),
			FailureThreshold: healthCheckReadinessFailureThreshold,
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
//
// host is the Host header an HTTP probe sends (healthProbeHost); TCP mode has no
// header to carry it. It deliberately never becomes HTTPGetAction.Host: that
// field moves the connection itself, so the probe would test the public route
// (and the release already serving it) instead of this pod.
func healthCheckHandler(path string, port int, host string) corev1.ProbeHandler {
	if path == "" {
		return corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(port)},
		}
	}
	get := &corev1.HTTPGetAction{Path: path, Port: intstr.FromInt(port)}
	if host != "" {
		get.HTTPHeaders = []corev1.HTTPHeader{{Name: probeHostHeader, Value: host}}
	}
	return corev1.ProbeHandler{HTTPGet: get}
}

const probeHostHeader = "Host"

// healthProbeHost is Render's probe Host: a verified custom domain, else the
// platform hostname. Without one kubelet sends the pod IP, which an app that
// allowlists its own hostnames rejects, so it could never deploy.
//
// Custom-first, unlike EffectiveHosts' public-URL order. bex-api admits only
// verified claims into spec.host/hosts, so a pending domain is never chosen; a
// redirect source is skipped because the edge never forwards it to the app.
// Types with no public route (private service, worker) keep the pod-IP Host.
func healthProbeHost(app *appv1alpha1.App, baseDomain string) string {
	hosts := effectiveHosts(app, baseDomain)
	if len(hosts) == 0 {
		return "" // not publicly routable, so a stray spec.host is not served
	}
	redirects := effectiveHostRedirects(app, hosts)
	custom := func(h string) bool {
		_, redirect := redirects[h]
		return h != "" && !redirect
	}
	if custom(app.Spec.Host) {
		return app.Spec.Host
	}
	for _, h := range app.Spec.Hosts {
		if custom(h) {
			return h
		}
	}
	// Only the platform host can remain unclaimed by the custom fields.
	for _, h := range hosts {
		if h != app.Spec.Host && !slices.Contains(app.Spec.Hosts, h) {
			return h
		}
	}
	return ""
}

// retainedProbeHost is the probe Host the stored template already sends for this
// same release. Domains are operational (AppSpecIdentityClasses): editing one
// opens no deploy, so it must not roll pods either; the freeze also keeps an
// operator upgrade from adding the header to running releases. ok is false for
// a new release or Deployment, which select from current domains.
func retainedProbeHost(stored corev1.PodTemplateSpec, app *appv1alpha1.App) (host string, ok bool) {
	if stored.Labels[labelRevision] != releaseRevision(app) {
		return "", false
	}
	for _, c := range stored.Spec.Containers {
		if c.Name != appContainerName {
			continue
		}
		if c.StartupProbe != nil && c.StartupProbe.HTTPGet != nil {
			for _, h := range c.StartupProbe.HTTPGet.HTTPHeaders {
				if h.Name == probeHostHeader {
					return h.Value, true
				}
			}
		}
		if c.StartupProbe != nil {
			if _, host, _, ok := healthProbeExecArgs(c.StartupProbe.Exec); ok {
				return host, true
			}
		}
		return "", true
	}
	return "", false
}

// drainSeconds is how long a traffic-serving tenant pod keeps serving after it
// is marked for deletion, before it gets SIGTERM (w1/m154). Deleting a pod
// sends SIGTERM at once while its endpoint leaves Traefik (and kube-proxy) a
// moment later, so an app that exits on SIGTERM would fail the requests still
// routed to it. The length is Render's: the original instance keeps serving
// its in-flight requests for 60 s after the switch (w8/180; it was 10 s,
// w1/m154); docs/ADR004-app-deployment.md § Rollout headroom, ADR018.
const drainSeconds int64 = 60

// drainSecondsFor is the preStop drain this App's pods should carry, or 0. A
// worker takes no inbound traffic, so it has nothing to drain. A disk-attached
// service rolls with Recreate, whose documented downtime a drain would only
// lengthen.
func drainSecondsFor(app *appv1alpha1.App, p deploymentParams) int64 {
	if p.worker || app.Spec.Disk != nil {
		return 0
	}
	return drainSeconds
}

// storedDrain is the preStop drain a stored pod spec carries, 0 for none.
func storedDrain(spec corev1.PodSpec) int64 {
	if len(spec.Containers) == 0 {
		return 0
	}
	lifecycle := spec.Containers[0].Lifecycle
	if lifecycle == nil || lifecycle.PreStop == nil || lifecycle.PreStop.Sleep == nil {
		return 0
	}
	return lifecycle.PreStop.Sleep.Seconds
}

// storedReadinessPeriod is the readiness probe period a stored pod spec
// carries, 0 for none or an unset one (which the server defaults to 10s).
func storedReadinessPeriod(spec corev1.PodSpec) int32 {
	if len(spec.Containers) == 0 || spec.Containers[0].ReadinessProbe == nil {
		return 0
	}
	return spec.Containers[0].ReadinessProbe.PeriodSeconds
}

// applyDrain sets the container's preStop drain and the pod's grace period
// together, so they cannot drift apart. The drain is a native preStop.sleep,
// which needs no shell or sleep binary in the tenant's image (scratch and
// distroless images have neither).
func applyDrain(spec *corev1.PodSpec, container *corev1.Container, shutdownDelay *int32, drain int64) {
	container.Lifecycle = nil
	if drain > 0 {
		container.Lifecycle = &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{Sleep: &corev1.SleepAction{Seconds: drain}},
		}
	}
	spec.TerminationGracePeriodSeconds = terminationGracePeriodSeconds(shutdownDelay, drain)
}

// terminationGracePeriodSeconds maps Render's maxShutdownDelaySeconds — the time
// after SIGTERM before SIGKILL — onto the pod grace period. Kubernetes counts
// preStop inside that period, so a drained pod's is the drain plus the delay
// (default 30). nil for an undrained pod with no delay set, so
// applyPodSpecServerDefaults writes Kubernetes' own 30-second default — the
// value its stored template already carries.
func terminationGracePeriodSeconds(seconds *int32, drain int64) *int64 {
	if seconds == nil && drain == 0 {
		return nil
	}
	shutdown := defaultTerminationGracePeriodSeconds
	if seconds != nil {
		shutdown = int64(*seconds)
	}
	value := drain + shutdown
	return &value
}

// applyDeploymentSpec projects the App onto dep's spec. It is the body of the
// reconciler's CreateOrUpdate mutation, kept separate and free of client calls
// so the whole pod shape is unit-testable.
//
// Every field it sets is rebuilt from the App on each pass, so removing a
// source (a secret file, a start command) drops cleanly out of the pod template
// rather than lingering from a previous revision.
//
// The preStop drain (w1/m154) is adopted lazily. A Deployment whose stored
// template carries a different drain (none, or the 10 s of before w8/180) gets
// the current one on the pass that changes its template for any other reason —
// a deploy, a config change, a restart — and not before. Changing it eagerly
// would change every web and private service's template on the same operator
// upgrade and roll them all at once, where the surge pods outrun the tenant
// headroom and mint nodes (ADR004 § Rollout headroom). A new Deployment, or
// one already on the current drain, always gets it.
func applyDeploymentSpec(dep *appsv1.Deployment, app *appv1alpha1.App, p deploymentParams) {
	probeHost, retained := retainedProbeHost(dep.Spec.Template, app)
	if !retained {
		probeHost = healthProbeHost(app, p.baseDomain)
	}
	drain := drainSecondsFor(app, p)
	if stored := dep.Spec.Template; len(stored.Spec.Containers) > 0 {
		keptDrain, keptPeriod := storedDrain(stored.Spec), storedReadinessPeriod(stored.Spec)
		keptProbeImage := storedHealthProbeImage(stored.Spec)
		drainStale := drain > 0 && keptDrain != drain
		periodStale := !p.worker && keptPeriod != healthCheckReadinessPeriodSeconds
		// The no-redirect probe (w8/m133) is adopted the same lazy way: a
		// running release keeps its HTTP probe, or the helper image it was
		// installed from, until its template changes for another reason, so
		// an operator upgrade never rolls the fleet.
		probeStale := !p.worker && app.Spec.HealthCheckPath != "" && keptProbeImage != p.healthProbeImage
		if drainStale || periodStale || probeStale {
			// Keep whatever the stored template carries when nothing else
			// about it changes; any other change adopts the current values.
			unchanged := dep.DeepCopy()
			kept := p
			kept.readinessPeriod = keptPeriod
			kept.healthProbeImage = keptProbeImage
			keptDrainFor := drain
			if drainStale {
				keptDrainFor = keptDrain
			}
			projectDeployment(unchanged, app, kept, keptDrainFor, probeHost)
			if apiequality.Semantic.DeepEqual(unchanged.Spec.Template, stored) {
				drain, p = keptDrainFor, kept
			}
		}
	}
	projectDeployment(dep, app, p, drain, probeHost)
}

// projectDeployment is applyDeploymentSpec with the drain and probe Host decided.
func projectDeployment(dep *appsv1.Deployment, app *appv1alpha1.App, p deploymentParams, drain int64, probeHost string) {
	replicas := clampReplicas(app, p.replicas)
	dep.Spec.Replicas = &replicas
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: app.Name}}
	dep.Spec.Template.Labels = appPodLabels(app, p.verifyImage)
	// Timer (2) of the three named on rolloutBudgetSeconds. Unset, Kubernetes
	// defaults this to 600s, which would cut Render's 15-minute window to 10
	// for reasons nobody chose.
	dep.Spec.ProgressDeadlineSeconds = new(rolloutBudgetSeconds)

	// restart = roll the template (same mechanism as kubectl rollout restart,
	// recorded in the contract). Never removed once set — removal would itself
	// roll the pods again.
	if app.Spec.RestartedAt != "" {
		if dep.Spec.Template.Annotations == nil {
			dep.Spec.Template.Annotations = map[string]string{}
		}
		dep.Spec.Template.Annotations["app.bex.co/restarted-at"] = app.Spec.RestartedAt
	}

	container := appContainer(app, p, probeHost)
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
	applyDrain(&dep.Spec.Template.Spec, &container, app.Spec.MaxShutdownDelaySeconds, drain)
	dep.Spec.Template.Spec.Containers = []corev1.Container{container}
	dep.Spec.Template.Spec.InitContainers = nil
	if p.usesHealthProbeBinary(app) {
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, healthProbeVolume())
		dep.Spec.Template.Spec.InitContainers = []corev1.Container{healthProbeInstallContainer(p.healthProbeImage)}
	}
	if p.edgeCacheImage != "" {
		dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, edgeCacheContainer(app, p.edgeCacheImage, p.port, drain))
		if purged := app.Annotations[appv1alpha1.CachePurgedAtAnnotation]; purged != "" {
			if dep.Spec.Template.Annotations == nil {
				dep.Spec.Template.Annotations = map[string]string{}
			}
			dep.Spec.Template.Annotations[appv1alpha1.CachePurgedAtAnnotation] = purged
		}
	}
	dep.Spec.Template.Spec.ImagePullSecrets = p.pullSecrets
	dep.Spec.Template.Spec.AutomountServiceAccountToken = new(false)
	dep.Spec.Template.Spec.HostUsers = appHostUsers(app.Spec, p.userNamespaces)
	// No legacy Docker-link variables (w4/m123). With Kubernetes' default
	// (true), the kubelet injects a <NAME>_SERVICE_HOST / <NAME>_SERVICE_PORT /
	// <NAME>_PORT_<n>_TCP_* set for EVERY Service in the pod's namespace — live,
	// a tenant web service with four configured variables received 185, of which
	// 174 were these. They named every sibling resource in the workspace,
	// including managed Postgres primaries by id and cert-manager's own ACME
	// solver Service, and — the actual footgun — a <NAME>_PORT variable shadows
	// the same name an application expects to configure itself with, handing it
	// `tcp://10.x.x.x:6379` where it expected its own value. Nothing in bex
	// reads a service link; the supported way to reach a sibling is its private
	// address, `<slug>:<port>` (ADR041 D4).
	//
	// This is a bex CHOICE, so it belongs here and not in
	// applyPodSpecServerDefaults, which exists to mirror what Kubernetes would
	// have chosen. Writing it moves the stored pod template, so every tenant
	// pod rolls once when this ships — see the milestone's t002.
	dep.Spec.Template.Spec.EnableServiceLinks = new(false)
	// Last, always: above is what bex chooses, this is what Kubernetes would have
	// chosen for the rest. See server_defaults.go.
	applyPodSpecServerDefaults(&dep.Spec.Template.Spec)
}
