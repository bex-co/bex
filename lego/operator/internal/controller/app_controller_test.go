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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("App Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		service := &appv1alpha1.App{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind App")
			err := k8sClient.Get(ctx, typeNamespacedName, service)
			if err != nil && errors.IsNotFound(err) {
				resource := &appv1alpha1.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &appv1alpha1.App{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance App")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AppReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("When reconciling an exposed App on the kubernetes runtime", func() {
		const name = "multi-host-app"
		ctx := context.Background()
		nn := types.NamespacedName{Name: name, Namespace: "default"}

		// k8sClient is only set in BeforeSuite — build the reconciler lazily, never
		// in the container body (tree construction runs before the suite starts).
		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			}
		})
		// First pass only adds the finalizer and requeues; later passes stop at
		// Deploying (no kubelet in envtest), which is past the Ingress write.
		reconcileN := func() {
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		getIngress := func() *networkingv1.Ingress {
			ing := &networkingv1.Ingress{}
			Expect(k8sClient.Get(ctx, nn, ing)).To(Succeed())
			return ing
		}

		AfterEach(func() {
			app := &appv1alpha1.App{}
			if err := k8sClient.Get(ctx, nn, app); err == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileN() // let the finalizer path run
			}
		})

		It("keeps the single-host Ingress byte-stable and grows it additively", func() {
			By("creating an App with only the legacy spec.host set")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami",
					Port:  3000,
					Host:  "app.1.2.3.4.sslip.io",
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN()

			By("asserting the live-App invariants: one rule, legacy TLS secret name")
			ing := getIngress()
			Expect(ing.Spec.Rules).To(HaveLen(1))
			Expect(ing.Spec.Rules[0].Host).To(Equal("app.1.2.3.4.sslip.io"))
			Expect(ing.Spec.TLS).To(HaveLen(1))
			Expect(ing.Spec.TLS[0].SecretName).To(Equal(name + "-tls"))
			Expect(ing.Annotations).To(HaveKeyWithValue("cert-manager.io/cluster-issuer", "letsencrypt-prod"))

			By("adding expose + a custom domain")
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			app.Spec.Expose = true
			app.Spec.Hosts = []string{"www.example.com"}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()

			By("asserting hosts grew additively with per-host TLS secrets")
			ing = getIngress()
			Expect(ing.Spec.Rules).To(HaveLen(3))
			Expect(ing.Spec.Rules[0].Host).To(Equal("app.1.2.3.4.sslip.io"))
			Expect(ing.Spec.Rules[1].Host).To(Equal(name + ".onbex.co"))
			Expect(ing.Spec.Rules[2].Host).To(Equal("www.example.com"))
			Expect(ing.Spec.TLS).To(HaveLen(3))
			Expect(ing.Spec.TLS[0].SecretName).To(Equal(name+"-tls"), "first host must keep the legacy secret")
			Expect(ing.Spec.TLS[1].SecretName).To(Equal(name + "-tls-" + name + ".onbex.co"))
			Expect(ing.Spec.TLS[2].SecretName).To(Equal(name + "-tls-www.example.com"))
			for i, tls := range ing.Spec.TLS {
				Expect(tls.Hosts).To(Equal([]string{ing.Spec.Rules[i].Host}), "TLS entries pair 1:1 with rules")
			}

			By("clearing all exposure removes the Ingress")
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			app.Spec.Host = ""
			app.Spec.Expose = false
			app.Spec.Hosts = nil
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			err := k8sClient.Get(ctx, nn, &networkingv1.Ingress{})
			Expect(errors.IsNotFound(err)).To(BeTrue(), "Ingress should be deleted when no hosts remain")
		})
	})

	Context("health-check gating: spec.healthCheckPath → ReadinessProbe (kubernetes runtime)", func() {
		ctx := context.Background()
		reconcileN := func(nn types.NamespacedName) {
			r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		getDep := func(nn types.NamespacedName) *appsv1.Deployment {
			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			return dep
		}
		// create + reconcile a service-shaped App, return its pod template.
		appPodSpec := func(name string, spec appv1alpha1.AppSpec) corev1.PodSpec {
			nn := types.NamespacedName{Name: name, Namespace: "default"}
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Spec: spec}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN(nn)
			podSpec := getDep(nn).Spec.Template.Spec
			Expect(k8sClient.Delete(ctx, app)).To(Succeed()) // finalizer needs a reconcile to release
			reconcileN(nn)
			return podSpec
		}
		appContainer := func(name string, spec appv1alpha1.AppSpec) corev1.Container {
			return appPodSpec(name, spec).Containers[0]
		}

		It("sets an HTTP ReadinessProbe from an explicit healthCheckPath on the container port", func() {
			c := appContainer("healthz-app", appv1alpha1.AppSpec{
				Image: "traefik/whoami", Port: 3000, HealthCheckPath: "/healthz",
			})
			Expect(c.ReadinessProbe).NotTo(BeNil(), "an HTTP service must carry a readiness probe")
			Expect(c.ReadinessProbe.HTTPGet).NotTo(BeNil())
			Expect(c.ReadinessProbe.HTTPGet.Path).To(Equal("/healthz"))
			Expect(c.ReadinessProbe.HTTPGet.Port.IntVal).To(Equal(int32(3000)), "probe targets the app port")
			Expect(c.ReadinessProbe.TCPSocket).To(BeNil(), "an explicit path selects the strict HTTP mode")
		})

		// The counterpart of the case above, and the reason the CRD carries no
		// default: Kubernetes scores an HTTP probe healthy only on
		// 200 <= code < 400. Defaulting an unset path to GET / therefore made a
		// service whose root is a legitimate 404 — an API with no root route —
		// permanently un-Ready and impossible to deploy, while Render (which
		// defaults to a TCP socket probe) deploys it untouched.
		It("probes TCP, not HTTP, when healthCheckPath is unset — a 404 root must still deploy", func() {
			c := appContainer("default-health-app", appv1alpha1.AppSpec{
				Image: "traefik/whoami", Port: 3000,
			})
			Expect(c.ReadinessProbe).NotTo(BeNil())
			Expect(c.ReadinessProbe.TCPSocket).NotTo(BeNil(), "unset path must not impose an HTTP success range")
			Expect(c.ReadinessProbe.TCPSocket.Port.IntVal).To(Equal(int32(3000)))
			Expect(c.ReadinessProbe.HTTPGet).To(BeNil())
			Expect(c.StartupProbe).NotTo(BeNil())
			Expect(c.StartupProbe.TCPSocket).NotTo(BeNil(), "both probes share one handler")
		})

		// Render: "…responds with a 2xx or 3xx status code within five seconds."
		// Unset, Kubernetes defaults this to 1s, which a server-side-rendered
		// page cannot reliably meet — the pod flaps instead of failing honestly.
		It("gives every probe Render's five-second budget", func() {
			c := appContainer("probe-timeout-app", appv1alpha1.AppSpec{
				Image: "traefik/whoami", Port: 3000, HealthCheckPath: "/healthz",
			})
			Expect(c.ReadinessProbe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(c.StartupProbe.TimeoutSeconds).To(Equal(int32(5)))
		})

		// A slow boot must not be killed by whichever of the three rollout
		// timers happens to be tightest. Asserted as an equality against the
		// Deployment's own deadline rather than as a repeated literal — three
		// literals that agree today is exactly how ∞/600s/180s happened.
		It("spends the startup budget over 15 minutes, equal to progressDeadlineSeconds", func() {
			nn := types.NamespacedName{Name: "slow-boot-app", Namespace: "default"}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
				Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN(nn)
			dep := getDep(nn)
			c := dep.Spec.Template.Spec.Containers[0]

			Expect(dep.Spec.ProgressDeadlineSeconds).NotTo(BeNil())
			budget := c.StartupProbe.PeriodSeconds * c.StartupProbe.FailureThreshold
			Expect(budget).To(Equal(*dep.Spec.ProgressDeadlineSeconds),
				"startupProbe budget and progressDeadlineSeconds must be the same number")
			Expect(budget).To(Equal(int32(900)), "Render cancels a deploy after 15 minutes")

			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			reconcileN(nn)
		})

		It("omits every probe on a background_worker (no HTTP port)", func() {
			c := appContainer("worker-no-probe", appv1alpha1.AppSpec{
				Image: "traefik/whoami", Port: 3000, Type: appv1alpha1.TypeBackgroundWorker,
			})
			Expect(c.ReadinessProbe).To(BeNil(), "a worker exposes no HTTP port, so no readiness probe")
			Expect(c.StartupProbe).To(BeNil(), "…and no startup probe either")
			Expect(c.LivenessProbe).To(BeNil(), "…and no liveness probe either")
		})

		// Render's second steady-state stage (m81, unconditional parity — the
		// m81/t001 decision recorded in deployment_projection.go): 60s of
		// consecutive failures restarts the container. Kubelet suspends
		// liveness while the startup probe runs, so this cannot kill a slow
		// boot.
		It("restarts a wedged instance after 60s via a livenessProbe on the shared handler", func() {
			c := appContainer("liveness-app", appv1alpha1.AppSpec{
				Image: "traefik/whoami", Port: 3000, HealthCheckPath: "/healthz",
			})
			Expect(c.LivenessProbe).NotTo(BeNil(), "Render restarts an instance after 60s of failures")
			Expect(c.LivenessProbe.HTTPGet).NotTo(BeNil())
			Expect(c.LivenessProbe.HTTPGet.Path).To(Equal(c.ReadinessProbe.HTTPGet.Path),
				"the three probes share one handler and cannot disagree about what healthy means")
			Expect(c.LivenessProbe.TimeoutSeconds).To(Equal(int32(5)))
			Expect(c.LivenessProbe.PeriodSeconds*c.LivenessProbe.FailureThreshold).To(Equal(int32(60)),
				"Render restarts after 60 seconds of consecutive failures")
			Expect(c.StartupProbe).NotTo(BeNil(),
				"liveness without a startup probe would kill a slow boot")
		})

		It("maps maxShutdownDelaySeconds onto the pod grace period for every long-running type", func() {
			seconds := int32(90)
			for _, tc := range []struct {
				name        string
				serviceType string
			}{
				{name: "shutdown-web", serviceType: appv1alpha1.TypeWebService},
				{name: "shutdown-private", serviceType: appv1alpha1.TypePrivateService},
				{name: "shutdown-worker", serviceType: appv1alpha1.TypeBackgroundWorker},
			} {
				podSpec := appPodSpec(tc.name, appv1alpha1.AppSpec{
					Image: "traefik/whoami", Port: 3000, Type: tc.serviceType,
					MaxShutdownDelaySeconds: &seconds,
				})
				Expect(podSpec.TerminationGracePeriodSeconds).NotTo(BeNil(), tc.name)
				Expect(*podSpec.TerminationGracePeriodSeconds).To(Equal(int64(seconds)), tc.name)
			}
		})

		It("leaves the desired grace period absent when unset and receives Kubernetes' 30-second default", func() {
			Expect(terminationGracePeriodSeconds(nil)).To(BeNil(), "the reconciler must not author a default")
			podSpec := appPodSpec("shutdown-default", appv1alpha1.AppSpec{
				Image: "traefik/whoami", Port: 3000,
			})
			// The API server defaults PodSpec.terminationGracePeriodSeconds on
			// storage, so the retrieved Deployment contains 30 even though the
			// reconciler submitted nil (asserted directly above).
			Expect(podSpec.TerminationGracePeriodSeconds).NotTo(BeNil())
			Expect(*podSpec.TerminationGracePeriodSeconds).To(Equal(int64(30)))
		})

		It("rejects maxShutdownDelaySeconds outside 1-300 at the CRD boundary", func() {
			for i, seconds := range []int32{0, 301} {
				app := &appv1alpha1.App{
					ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("shutdown-invalid-%d", i), Namespace: "default"},
					Spec: appv1alpha1.AppSpec{
						Image: "traefik/whoami", MaxShutdownDelaySeconds: &seconds,
					},
				}
				Expect(k8sClient.Create(ctx, app)).NotTo(Succeed())
			}
		})
	})

	Context("When building an App from git in-cluster", func() {
		const name = "gitbuild-app"
		ctx := context.Background()
		nn := types.NamespacedName{Name: name, Namespace: "default"}

		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: "default",
			}
		})
		reconcileN := func() {
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				savedRegistry := r.Registry
				r.Registry = "" // this unit suite has no registry server; deletion semantics have focused tests
				reconcileN()
				r.Registry = savedRegistry
			}
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
				Name: build.JobName(name, "gen-1"), Namespace: "default"}})
		})

		It("reuses the exact-lifetime completed BuildKit Job and deploys its image", func() {
			By("creating a repo-backed App (no prebuilt image)")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Repo: "https://github.com/bex-co/hello", Branch: "main",
					Builder: "dockerfile", Port: 3000,
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("simulating the finished in-cluster build: a Complete Job named per release")
			// The image tag is the App's release generation (1 at create); Build()
			// validates this already-Complete Job's App UID instead of starting another
			// build, so reconciliation proceeds straight to the built image.
			image := "zot.test:5000/" + name + ":gen-1"
			job := build.BuildJob(build.Options{
				Name: name, AppUID: string(app.UID), Revision: "gen-1", Registry: "zot.test:5000",
				Namespace: "default", Repo: app.Spec.Repo, Ref: "main",
			}, image)
			Expect(k8sClient.Create(ctx, job)).To(Succeed())
			now := metav1.Now()
			job.Status.StartTime = &now
			job.Status.CompletionTime = &now
			job.Status.Conditions = []batchv1.JobCondition{
				{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			}
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			By("reconciling: the Deployment runs the in-cluster-built image (no host docker)")
			reconcileN()
			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal(image))

			By("the App records the built image so a no-op reconcile never rebuilds")
			got := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
			Expect(got.Status.Image).To(Equal(image))
		})
	})

	Context("Lifecycle verbs: restart, suspend, resume (kubernetes runtime)", func() {
		const name = "lifecycle-app"
		ctx := context.Background()
		nn := types.NamespacedName{Name: name, Namespace: "default"}

		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		})
		reconcileN := func() {
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		getDep := func() *appsv1.Deployment {
			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			return dep
		}
		getApp := func() *appv1alpha1.App {
			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			return app
		}

		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileN()
			}
		})

		It("rolls on restartedAt, hibernates on suspend, restores on resume", func() {
			By("creating a running-shaped App with 2 replicas and a host")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{
					labelAppID: "srv-c185th5c2rvvnhbfiltg",
				}},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami", Port: 3000, Replicas: 2,
					Host: "lifecycle.1.2.3.4.sslip.io",
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN()
			dep := getDep()
			Expect(*dep.Spec.Replicas).To(Equal(int32(2)))
			Expect(dep.Spec.Template.Labels[labelAppID]).To(Equal("srv-c185th5c2rvvnhbfiltg"),
				"the node meter must receive immutable resource attribution")
			Expect(dep.Spec.Template.Annotations).NotTo(HaveKey("app.bex.co/restarted-at"))

			By("restart: setting spec.restartedAt stamps the pod template")
			app = getApp()
			app.Spec.RestartedAt = "2026-07-05T00:00:00Z"
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			dep = getDep()
			Expect(dep.Spec.Template.Annotations).To(HaveKeyWithValue("app.bex.co/restarted-at", "2026-07-05T00:00:00Z"))
			Expect(*dep.Spec.Replicas).To(Equal(int32(2)), "restart must not touch scale")

			By("suspend: scales to 0, phase Hibernated, Ingress kept, spec.replicas kept")
			app = getApp()
			app.Spec.Suspended = true
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			Expect(*getDep().Spec.Replicas).To(Equal(int32(0)))
			app = getApp()
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))
			Expect(app.Spec.Replicas).To(Equal(int32(2)), "suspend must not rewrite the stored count")
			Expect(k8sClient.Get(ctx, nn, &networkingv1.Ingress{})).To(Succeed(), "suspend keeps the Ingress (host + certs)")

			By("resume: restores spec.replicas and leaves Hibernated")
			app.Spec.Suspended = false
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			Expect(*getDep().Spec.Replicas).To(Equal(int32(2)))
			// envtest has no kubelet, so readiness never arrives: Deploying, not Running
			Expect(getApp().Status.Phase).To(Equal(appv1alpha1.PhaseDeploying))
		})

		It("projects spec.env (PORT unshadowable) and spec.envFromSecret onto the container", func() {
			By("creating an App with user env, a PORT shadow attempt, and an envFrom secret ref")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami", Port: 3000,
					Env: []appv1alpha1.EnvVar{
						{Name: "GREETING", Value: "hello"},
						{Name: "PORT", Value: "9999"}, // must be ignored
					},
					EnvFromSecret: name + "-env",
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN()

			c := getDep().Spec.Template.Spec.Containers[0]
			By("user env is present and PORT stays operator-owned")
			envVal := func(key string) string {
				v := ""
				for _, e := range c.Env {
					if e.Name == key {
						v = e.Value
					}
				}
				return v
			}
			Expect(envVal("GREETING")).To(Equal("hello"))
			Expect(envVal("PORT")).To(Equal("3000"), "user PORT=9999 must not shadow the injected port")
			Expect(c.Env[len(c.Env)-1].Name).To(Equal("PORT"), "PORT is appended last so it wins")

			By("envFromSecret wires an envFrom SecretRef")
			Expect(c.EnvFrom).To(HaveLen(1))
			Expect(c.EnvFrom[0].SecretRef).NotTo(BeNil())
			Expect(c.EnvFrom[0].SecretRef.Name).To(Equal(name + "-env"))
		})
	})

	Context("Maintenance mode (kubernetes runtime)", func() {
		const name = "maintenance-app"
		ctx := context.Background()
		nn := types.NamespacedName{Name: name, Namespace: "default"}

		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
				ActivatorService: "bex-activator", ActivatorPort: 8888,
				MaintenanceService: "bex-activator", MaintenanceNamespace: "default", MaintenancePort: 8888,
			}
		})
		reconcileN := func() {
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		getDep := func() *appsv1.Deployment {
			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			return dep
		}
		getApp := func() *appv1alpha1.App {
			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			return app
		}
		getIngress := func() *networkingv1.Ingress {
			ing := &networkingv1.Ingress{}
			Expect(k8sClient.Get(ctx, nn, ing)).To(Succeed())
			return ing
		}
		backendOf := func(ing *networkingv1.Ingress, ruleIdx int) (string, int32) {
			b := ing.Spec.Rules[ruleIdx].HTTP.Paths[0].Backend.Service
			return b.Name, b.Port.Number
		}

		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileN()
			}
		})

		It("routes every host to the activator while enabled, restores on disable, pods untouched throughout", func() {
			By("creating a running App with two hosts")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami", Port: 3000, Replicas: 2,
					Host:  "maint.1.2.3.4.sslip.io",
					Hosts: []string{"www.maint-example.com"},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN()
			ing := getIngress()
			Expect(ing.Spec.Rules).To(HaveLen(2))
			for i := range ing.Spec.Rules {
				svc, port := backendOf(ing, i)
				Expect(svc).To(Equal(name))
				Expect(port).To(Equal(int32(3000)))
			}
			Expect(*getDep().Spec.Replicas).To(Equal(int32(2)))

			By("enabling maintenance mode: every host's Ingress backend swaps to the activator")
			app = getApp()
			app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			ing = getIngress()
			Expect(ing.Spec.Rules).To(HaveLen(2))
			for i := range ing.Spec.Rules {
				svc, port := backendOf(ing, i)
				Expect(svc).To(Equal("bex-activator"))
				Expect(port).To(Equal(int32(8888)))
			}
			By("the Deployment is left running at its configured replica count — no scale change")
			Expect(*getDep().Spec.Replicas).To(Equal(int32(2)))
			Expect(getDep().Spec.Template.Spec.Containers).To(HaveLen(1), "pods themselves are untouched")

			By("disabling maintenance mode restores the app's own Service as the backend")
			app = getApp()
			app.Spec.MaintenanceMode.Enabled = false
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			ing = getIngress()
			for i := range ing.Spec.Rules {
				svc, port := backendOf(ing, i)
				Expect(svc).To(Equal(name))
				Expect(port).To(Equal(int32(3000)))
			}
			Expect(*getDep().Spec.Replicas).To(Equal(int32(2)))
		})

		It("maintenance remains public while suspension independently scales the workload to zero", func() {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami", Port: 3000,
					Host:            "maint.1.2.3.4.sslip.io",
					Suspended:       true,
					MaintenanceMode: &appv1alpha1.MaintenanceModeSpec{Enabled: true},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN()

			By("suspend scales to 0 regardless of maintenance mode")
			Expect(*getDep().Spec.Replicas).To(Equal(int32(0)))
			Expect(getApp().Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))

			By("maintenance keeps the Ingress pointed at the responder")
			svc, port := backendOf(getIngress(), 0)
			Expect(svc).To(Equal("bex-activator"))
			Expect(port).To(Equal(int32(8888)))
		})

		It("maintenance takes routing precedence over an already auto-hibernated workload", func() {
			By("creating a free-tier App idle past its TTL — it auto-hibernates")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami", Port: 3000, Replicas: 2,
					Host:           "maint.1.2.3.4.sslip.io",
					IdleTTLSeconds: 300,
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN()
			app = getApp()
			base := app.DeepCopy()
			if app.Annotations == nil {
				app.Annotations = map[string]string{}
			}
			app.Annotations[annotLastActive] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
			Expect(k8sClient.Patch(ctx, app, client.MergeFrom(base))).To(Succeed())
			reconcileN()
			Expect(*getDep().Spec.Replicas).To(Equal(int32(0)), "idle past its TTL, the app auto-hibernates")
			Expect(getApp().Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))

			By("enabling maintenance keeps the legacy free workload asleep")
			app = getApp()
			app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			Expect(*getDep().Spec.Replicas).To(Equal(int32(0)), "maintenance routing must not override auto-sleep replicas")

			By("the Ingress still routes to the activator throughout (auto-hibernate, then maintenance)")
			svc, port := backendOf(getIngress(), 0)
			Expect(svc).To(Equal("bex-activator"))
			Expect(port).To(Equal(int32(8888)))
		})
	})
})
