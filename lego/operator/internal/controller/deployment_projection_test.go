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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
)

func projectionApp(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService},
	}
	for _, m := range mutate {
		m(app)
	}
	return app
}

func project(app *appv1alpha1.App, p deploymentParams) *appsv1.Deployment {
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}}
	applyDeploymentSpec(dep, app, p)
	return dep
}

func webParams() deploymentParams {
	return deploymentParams{image: "zot.local:5000/hello:rev-1", port: 8080, replicas: 2}
}

func TestApplyDeploymentSpecCapsOversizedReplicas(t *testing.T) {
	dep := project(projectionApp(), deploymentParams{
		image: "img", port: 8080, replicas: 2_000_000_000,
	})
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != appv1alpha1.MaxReplicas {
		t.Fatalf("replicas = %v, want platform ceiling %d", dep.Spec.Replicas, appv1alpha1.MaxReplicas)
	}
}

func appContainerOf(t *testing.T, dep *appsv1.Deployment) corev1.Container {
	t.Helper()
	if n := len(dep.Spec.Template.Spec.Containers); n != 1 {
		t.Fatalf("containers = %d; want exactly the one app container", n)
	}
	return dep.Spec.Template.Spec.Containers[0]
}

// TestSelectorStaysAppOnly pins the invariant the projection's comment warns
// about: a Deployment selector is immutable, so the pod-template labels that
// grow over time (workspace, revision, network isolation) must never leak into
// it — an existing Deployment would become unpatchable the moment one appeared.
func TestSelectorStaysAppOnly(t *testing.T) {
	app := projectionApp(func(a *appv1alpha1.App) {
		a.Labels = map[string]string{
			labelAppID:            "srv-abc",
			labelWorkspace:        "tea-xyz",
			labelNetworkIsolation: "env-1",
		}
	})
	dep := project(app, deploymentParams{image: "img", port: 8080, replicas: 1, verifyImage: true})

	want := map[string]string{labelApp: "hello"}
	got := dep.Spec.Selector.MatchLabels
	if len(got) != len(want) || got[labelApp] != want[labelApp] {
		t.Fatalf("selector = %v; want exactly %v", got, want)
	}
	// The same labels must nevertheless be on the pod template.
	for _, key := range []string{labelWorkspace, labelNetworkIsolation, labelRevision, execution.LabelVerifyImage} {
		if dep.Spec.Template.Labels[key] == "" {
			t.Errorf("pod template is missing label %q", key)
		}
	}
}

func TestAppPodLabels(t *testing.T) {
	t.Run("app id falls back to the name", func(t *testing.T) {
		got := appPodLabels(projectionApp(), false)
		if got[labelAppID] != "hello" {
			t.Errorf("appID label = %q; want the App name as fallback", got[labelAppID])
		}
		if got[labelApp] != "hello" {
			t.Errorf("app label = %q", got[labelApp])
		}
		if got[labelRevision] == "" {
			t.Error("revision label must always be stamped")
		}
	})

	t.Run("app id is propagated when present", func(t *testing.T) {
		app := projectionApp(func(a *appv1alpha1.App) {
			a.Labels = map[string]string{labelAppID: "srv-abc"}
		})
		if got := appPodLabels(app, false); got[labelAppID] != "srv-abc" {
			t.Errorf("appID label = %q; want srv-abc", got[labelAppID])
		}
	})

	t.Run("optional labels are omitted when their source is absent", func(t *testing.T) {
		got := appPodLabels(projectionApp(), false)
		for _, key := range []string{labelWorkspace, labelNetworkIsolation, execution.LabelVerifyImage} {
			if _, ok := got[key]; ok {
				t.Errorf("label %q must not be stamped when unset", key)
			}
		}
	})

	t.Run("verify-image is stamped only when signing is configured", func(t *testing.T) {
		if got := appPodLabels(projectionApp(), true); got[execution.LabelVerifyImage] != execution.VerifyImageEnabled {
			t.Errorf("verify-image = %q; want %q", got[execution.LabelVerifyImage], execution.VerifyImageEnabled)
		}
	})
}

// TestWorkerHasNoHTTPSurface pins the background_worker divergence: no port to
// declare and nothing to health-check, since no traffic is ever routed to it.
func TestWorkerHasNoHTTPSurface(t *testing.T) {
	p := webParams()
	p.worker = true
	container := appContainerOf(t, project(projectionApp(), p))

	if len(container.Ports) != 0 {
		t.Errorf("ports = %v; a worker declares none", container.Ports)
	}
	if container.ReadinessProbe != nil || container.StartupProbe != nil || container.LivenessProbe != nil {
		t.Error("a worker has no HTTP surface to probe")
	}
}

func TestWebServiceReadinessProbe(t *testing.T) {
	t.Run("probes the configured health check path", func(t *testing.T) {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/healthz" })
		container := appContainerOf(t, project(app, webParams()))

		if container.ReadinessProbe == nil || container.ReadinessProbe.HTTPGet == nil {
			t.Fatal("a web service must be health-gated")
		}
		if got := container.ReadinessProbe.HTTPGet.Path; got != "/healthz" {
			t.Errorf("probe path = %q; want /healthz", got)
		}
		if got := container.ReadinessProbe.HTTPGet.Port.IntValue(); got != 8080 {
			t.Errorf("probe port = %d; want the container port", got)
		}
	})

	// An unset path means the platform was never told what healthy looks like,
	// so it asks only what it can answer honestly — is the process listening —
	// which is Render's default too. It must NOT fall back to GET /: Kubernetes
	// scores an HTTP probe healthy only on 200 <= code < 400, so that fallback
	// made a service whose root is a legitimate 404 permanently un-Ready and
	// impossible to deploy.
	t.Run("falls back to a TCP probe, not GET /, when no path is set", func(t *testing.T) {
		container := appContainerOf(t, project(projectionApp(), webParams()))

		if container.ReadinessProbe == nil || container.ReadinessProbe.TCPSocket == nil {
			t.Fatal("an unset health check path must probe TCP")
		}
		if container.ReadinessProbe.HTTPGet != nil {
			t.Error("an unset path must not impose an HTTP success range on the root route")
		}
		if got := container.ReadinessProbe.TCPSocket.Port.IntValue(); got != 8080 {
			t.Errorf("probe port = %d; want the container port", got)
		}
		if container.LivenessProbe == nil || container.LivenessProbe.TCPSocket == nil {
			t.Error("TCP mode carries the restart stage too — the m81/t001 contract is unconditional parity")
		}
	})

	// Both probes must agree about what healthy means; they share one handler
	// so they cannot drift.
	t.Run("startup and readiness share one handler", func(t *testing.T) {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/healthz" })
		container := appContainerOf(t, project(app, webParams()))

		if container.StartupProbe == nil || container.StartupProbe.HTTPGet == nil {
			t.Fatal("a web service must carry a startup probe")
		}
		if container.StartupProbe.HTTPGet.Path != container.ReadinessProbe.HTTPGet.Path {
			t.Errorf("startup probes %q but readiness probes %q",
				container.StartupProbe.HTTPGet.Path, container.ReadinessProbe.HTTPGet.Path)
		}
	})

	// Render: healthy means answering "within five seconds". Kubernetes would
	// default this to 1s, which a server-side-rendered page cannot meet — the
	// pod then flaps NotReady forever instead of failing honestly.
	t.Run("gives every probe Render's five-second budget", func(t *testing.T) {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/healthz" })
		container := appContainerOf(t, project(app, webParams()))

		if got := container.ReadinessProbe.TimeoutSeconds; got != 5 {
			t.Errorf("readiness timeout = %ds; want Render's 5s", got)
		}
		if got := container.StartupProbe.TimeoutSeconds; got != 5 {
			t.Errorf("startup timeout = %ds; want Render's 5s", got)
		}
	})

	// Render restarts an instance after 60s of consecutive failures; m81 ships
	// that stage as a liveness probe on the shared handler. The contract is
	// unconditional parity (the m81/t001 decision, recorded where the probes
	// are built): the spiral hazard that deferred it is bounded by the startup
	// probe below, and in TCP mode the probe never invokes the tenant handler.
	t.Run("restarts a wedged instance after 60s (Render's second stage)", func(t *testing.T) {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/healthz" })
		container := appContainerOf(t, project(app, webParams()))

		if container.LivenessProbe == nil || container.LivenessProbe.HTTPGet == nil {
			t.Fatal("a web service must carry a liveness probe — Render restarts after 60s of failures")
		}
		if container.LivenessProbe.HTTPGet.Path != container.ReadinessProbe.HTTPGet.Path {
			t.Errorf("liveness probes %q but readiness probes %q; the three probes share one handler",
				container.LivenessProbe.HTTPGet.Path, container.ReadinessProbe.HTTPGet.Path)
		}
		if got := container.LivenessProbe.TimeoutSeconds; got != 5 {
			t.Errorf("liveness timeout = %ds; want Render's 5s", got)
		}
		if window := container.LivenessProbe.PeriodSeconds * container.LivenessProbe.FailureThreshold; window != 60 {
			t.Errorf("liveness window = %ds; want Render's 60s restart threshold", window)
		}
	})

	// Kubelet suspends liveness only while the startup probe runs, so a
	// liveness probe without one would kill a slow-booting service — the
	// restart spiral this milestone waited on m80 to make impossible. Pin the
	// coexistence so a future edit cannot reintroduce boot-kill.
	t.Run("liveness never fires during boot", func(t *testing.T) {
		container := appContainerOf(t, project(projectionApp(), webParams()))

		if container.LivenessProbe == nil {
			t.Fatal("liveness missing — the restart stage above is the contract")
		}
		if container.StartupProbe == nil {
			t.Fatal("liveness without a startup probe would kill a slow boot")
		}
		if boot := container.StartupProbe.PeriodSeconds * container.StartupProbe.FailureThreshold; boot != 900 {
			t.Errorf("startup budget = %ds; liveness must stay suspended for the full 15m rollout budget", boot)
		}
	})

	t.Run("declares the container port", func(t *testing.T) {
		container := appContainerOf(t, project(projectionApp(), webParams()))
		if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 8080 {
			t.Errorf("ports = %v; want the single app port", container.Ports)
		}
	})
}

// TestRolloutTimersAgree pins the two mechanism timers to one number. They are
// the operator's half of the three that bound a rollout; bex-api's
// DeployGateTimeout is the third and observes them from another module, so it
// is asserted there (TestDeployGateOutlastsTheRolloutBudgetItObserves).
//
// Asserted as an equality between the two derived values rather than as two
// literals: three literals that happen to agree today is exactly how the
// pre-m80 state (∞ / 600s / 180s) arose, where the effective budget was the
// accidental minimum of three unrelated choices.
func TestRolloutTimersAgree(t *testing.T) {
	dep := project(projectionApp(), webParams())
	container := appContainerOf(t, dep)

	if dep.Spec.ProgressDeadlineSeconds == nil {
		t.Fatal("progressDeadlineSeconds must be set; unset inherits Kubernetes' 600s and silently " +
			"cuts Render's 15-minute window to 10")
	}
	budget := container.StartupProbe.PeriodSeconds * container.StartupProbe.FailureThreshold
	if budget != *dep.Spec.ProgressDeadlineSeconds {
		t.Errorf("startup budget = %ds but progressDeadlineSeconds = %ds; the tightest silently "+
			"decides, so they must be one number", budget, *dep.Spec.ProgressDeadlineSeconds)
	}
	if budget != 900 {
		t.Errorf("rollout budget = %ds; want Render's 15 minutes", budget)
	}
}

// TestStartCommandOnlyOverridesOpaqueImages pins the rule that a native or
// buildpack build bakes its command into the image, so overriding the
// entrypoint there would fight the builder rather than help.
func TestStartCommandOnlyOverridesOpaqueImages(t *testing.T) {
	for _, tc := range []struct {
		name         string
		builder      string
		runtime      string
		wantOverride bool
	}{
		{"dockerfile builder overrides", build.BuilderDockerfile, "", true},
		{"prebuilt image overrides", "", "", true},
		{"buildpack builder does not", build.BuilderBuildpack, "", false},
		{"native runtime does not", "", "go", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := projectionApp(func(a *appv1alpha1.App) {
				a.Spec.Builder = tc.builder
				a.Spec.Runtime = tc.runtime
				a.Spec.StartCommand = "./server --port 8080"
			})
			container := appContainerOf(t, project(app, webParams()))

			if tc.wantOverride {
				want := []string{"/bin/sh", "-c", "./server --port 8080"}
				if len(container.Command) != 3 || container.Command[2] != want[2] {
					t.Errorf("command = %v; want %v", container.Command, want)
				}
			} else if container.Command != nil {
				t.Errorf("command = %v; want the image's own entrypoint left alone", container.Command)
			}
		})
	}
}

// TestStartCommandAbsentIsNoOverride guards the empty case separately: an
// opaque image with no start command must still keep its own entrypoint.
func TestStartCommandAbsentIsNoOverride(t *testing.T) {
	app := projectionApp(func(a *appv1alpha1.App) { a.Spec.Builder = build.BuilderDockerfile })
	if got := appContainerOf(t, project(app, webParams())).Command; got != nil {
		t.Errorf("command = %v; want none", got)
	}
}

func TestRestartAnnotation(t *testing.T) {
	t.Run("stamped when set", func(t *testing.T) {
		app := projectionApp(func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-08-08T00:00:00Z" })
		got := project(app, webParams()).Spec.Template.Annotations["app.bex.co/restarted-at"]
		if got != "2026-08-08T00:00:00Z" {
			t.Errorf("restarted-at = %q", got)
		}
	})

	t.Run("absent when unset", func(t *testing.T) {
		dep := project(projectionApp(), webParams())
		if _, ok := dep.Spec.Template.Annotations["app.bex.co/restarted-at"]; ok {
			t.Error("restarted-at must not be stamped when the App never restarted")
		}
	})

	// Removing the annotation would itself roll the pods, so an existing stamp
	// survives a pass where the App no longer carries one.
	t.Run("an existing stamp is never cleared", func(t *testing.T) {
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "hello", Namespace: "default"}}
		dep.Spec.Template.Annotations = map[string]string{"app.bex.co/restarted-at": "2026-01-01T00:00:00Z"}
		applyDeploymentSpec(dep, projectionApp(), webParams())
		if got := dep.Spec.Template.Annotations["app.bex.co/restarted-at"]; got != "2026-01-01T00:00:00Z" {
			t.Errorf("restarted-at = %q; want the previous stamp preserved", got)
		}
	})
}

// TestSecretFilesRebuildEveryPass covers the projection's rebuild-from-scratch
// contract: a removed file source must drop out of the pod template rather than
// linger from the previous revision.
func TestSecretFilesRebuildEveryPass(t *testing.T) {
	withFiles := projectionApp(func(a *appv1alpha1.App) {
		a.Spec.FilesFromSecrets = []string{"hello-files"}
	})
	dep := project(withFiles, webParams())
	if len(dep.Spec.Template.Spec.Volumes) != 1 {
		t.Fatalf("volumes = %v; want the projected secret volume", dep.Spec.Template.Spec.Volumes)
	}
	if len(appContainerOf(t, dep).VolumeMounts) != 1 {
		t.Fatal("the app container must mount the secret volume")
	}

	// Same Deployment, App no longer declares any file source.
	applyDeploymentSpec(dep, projectionApp(), webParams())
	if len(dep.Spec.Template.Spec.Volumes) != 0 {
		t.Errorf("volumes = %v; want the removed source dropped", dep.Spec.Template.Spec.Volumes)
	}
	if got := appContainerOf(t, dep).VolumeMounts; len(got) != 0 {
		t.Errorf("volumeMounts = %v; want none", got)
	}
}

// TestPodHardeningDefaults pins the two hardening invariants that must hold for
// every tenant pod regardless of type (w7/m2).
func TestPodHardeningDefaults(t *testing.T) {
	for _, worker := range []bool{false, true} {
		p := webParams()
		p.worker = worker
		dep := project(projectionApp(), p)

		automount := dep.Spec.Template.Spec.AutomountServiceAccountToken
		if automount == nil || *automount {
			t.Errorf("worker=%v: AutomountServiceAccountToken = %v; want an explicit false", worker, automount)
		}
		if appContainerOf(t, dep).SecurityContext == nil {
			t.Errorf("worker=%v: tenant containers must carry a securityContext", worker)
		}
	}
}

func TestTerminationGracePeriod(t *testing.T) {
	t.Run("unset writes the kubernetes default explicitly", func(t *testing.T) {
		// 30 is what the API server already stored for every App that never set
		// maxShutdownDelaySeconds, so writing it changes no pod template and
		// rolls nothing — it only lets the projection equal the stored object,
		// which is what keeps a converged reconcile from PUTting (w7/m84).
		dep := project(projectionApp(), webParams())
		got := dep.Spec.Template.Spec.TerminationGracePeriodSeconds
		if got == nil || *got != defaultTerminationGracePeriodSeconds {
			t.Errorf("grace = %v; want the Kubernetes default %d", got, defaultTerminationGracePeriodSeconds)
		}
	})

	t.Run("propagated when set", func(t *testing.T) {
		app := projectionApp(func(a *appv1alpha1.App) {
			seconds := int32(45)
			a.Spec.MaxShutdownDelaySeconds = &seconds
		})
		got := project(app, webParams()).Spec.Template.Spec.TerminationGracePeriodSeconds
		if got == nil || *got != 45 {
			t.Errorf("grace = %v; want 45", got)
		}
	})
}

func TestReplicasAndPullSecretsPassThrough(t *testing.T) {
	p := webParams()
	p.replicas = 4
	p.pullSecrets = []corev1.LocalObjectReference{{Name: "reg-pull-hello"}}
	dep := project(projectionApp(), p)

	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 4 {
		t.Errorf("replicas = %v; want 4", dep.Spec.Replicas)
	}
	secrets := dep.Spec.Template.Spec.ImagePullSecrets
	if len(secrets) != 1 || secrets[0].Name != "reg-pull-hello" {
		t.Errorf("imagePullSecrets = %v; want the per-App pull secret", secrets)
	}
}

// TestPublicImageKeepsNoPullSecret covers the prebuilt-public-image path: with
// no credential resolved the pod template must stay clean rather than reference
// a secret that does not exist.
func TestPublicImageKeepsNoPullSecret(t *testing.T) {
	dep := project(projectionApp(), webParams())
	if got := dep.Spec.Template.Spec.ImagePullSecrets; len(got) != 0 {
		t.Errorf("imagePullSecrets = %v; want none for an unauthenticated image", got)
	}
}

// TestProjectionIsIdempotent pins that re-projecting an unchanged App produces
// an unchanged pod template — otherwise every reconcile would roll the pods.
func TestProjectionIsIdempotent(t *testing.T) {
	app := projectionApp(func(a *appv1alpha1.App) {
		a.Labels = map[string]string{labelAppID: "srv-abc", labelWorkspace: "tea-xyz"}
		a.Spec.FilesFromSecrets = []string{"hello-files"}
		a.Spec.HealthCheckPath = "/healthz"
		a.Spec.RestartedAt = "2026-08-08T00:00:00Z"
	})
	p := webParams()
	p.pullSecrets = []corev1.LocalObjectReference{{Name: "reg-pull-hello"}}

	first := project(app, p)
	second := first.DeepCopy()
	applyDeploymentSpec(second, app, p)

	if !equalPodTemplate(first.Spec.Template, second.Spec.Template) {
		t.Error("re-projecting an unchanged App must not alter the pod template")
	}
}

func equalPodTemplate(a, b corev1.PodTemplateSpec) bool {
	return a.String() == b.String()
}
