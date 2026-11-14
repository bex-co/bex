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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A brand-new web image service has no release to protect while HTTP port
// discovery runs, so its first deploy writes one pod template — one
// ReplicaSet — and reports its own release serving. Before w2/078 discovery
// probed HTTP on / and the chosen port rewrote the template to TCP, rolling a
// second ReplicaSet, and Serving read PriorReleaseServing on a service that
// never had a prior release. A later deploy must still roll, with the release
// it replaces reported as the prior one.
var _ = Describe("A first web image deploy (w2/078)", func() {
	It("writes one pod template, reports its own release serving, and a later deploy still rolls", func() {
		nn := types.NamespacedName{Name: "first-deploy-one-rollout", Namespace: "default"}
		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			HTTPClient: imageConfigRegistry(GinkgoT(), `"80/tcp":{}`),
		}
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "docker.io/traefik/whoami:v1.10.3", Type: appv1alpha1.TypeWebService,
				PortMode: appv1alpha1.PortModeWebImageV1, Replicas: 1,
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })

		By("discovering the HTTP port under the final TCP probe")
		reconcileToFixedPoint(run)
		app := envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(BeEmpty())
		Expect(webImageHTTPSelectionPending(app)).To(BeTrue(), "the test must exercise a pending discovery")
		first := envtestDeployment(nn).Spec.Template
		assertImagePortRuntime(nn, 80)
		simulateTemplatePod(nn, "first", true)

		By("choosing the port leaves the template, so no second ReplicaSet rolls")
		app.Status.ImageNetwork.HTTPSelection = appv1alpha1.HTTPSelectionDetected
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
		Expect(apiequality.Semantic.DeepEqual(envtestDeployment(nn).Spec.Template, first)).To(BeTrue(),
			"the first deploy's pod template changed after discovery: %+v", envtestDeployment(nn).Spec.Template.Spec.Containers[0].ReadinessProbe)

		app = envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		serving := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionServing)
		Expect(serving).NotTo(BeNil())
		Expect(serving.Status).To(Equal(metav1.ConditionTrue))
		Expect(serving.Reason).To(Equal(appv1alpha1.ReasonReleaseServing), "a first release has no prior release")

		By("a second deploy rolls a new template, while the first release serves as the prior one")
		app.Spec.RestartedAt = "2026-11-09T00:00:00Z"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		second := envtestDeployment(nn).Spec.Template
		Expect(apiequality.Semantic.DeepEqual(second, first)).To(BeFalse(), "a real redeploy must roll")
		Expect(second.Labels[labelRevision]).To(Equal("rev-2"))
		app = envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		serving = meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionServing)
		Expect(serving).NotTo(BeNil())
		Expect(serving.Status).To(Equal(metav1.ConditionTrue))
		Expect(serving.Reason).To(Equal(appv1alpha1.ReasonPriorReleaseServing))
		for _, probe := range probesOf(second.Spec.Containers[0]) {
			Expect(probe.TCPSocket).NotTo(BeNil(), "the retained HTTP choice needs no discovery probe")
		}
		// The process keeps Render's PORT through discovery and the next
		// release, whatever port it is served on (w4/615).
		Expect(first.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PORT", Value: "10000"}))
		Expect(second.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PORT", Value: "10000"}))
	})
})

// A first deploy of a web image whose server listens on a port its image does
// not declare (traefik/whoami, EXPOSE 80, listening on 10000) rolled two
// ReplicaSets: discovery moved the primary from 80 to 10000, and the probes'
// port and the container ports moved with it (w4/m350 t001; PORT already
// stays 10000, w4/615). Render's default health check is a TCP probe to any
// of the service's open ports, so the first release's template checks any
// open port, pins no container port, and discovery moves only the Services.
var _ = Describe("A first web image deploy on an undeclared port (w4/m350)", func() {
	const probeImage = "ghcr.io/bex-co/bex@sha256:1111"
	firstRelease := func(name string, mutate func(*appv1alpha1.App)) (types.NamespacedName, *AppReconciler, func() (ctrl.Result, error)) {
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, BackupHelperImage: probeImage,
			HTTPClient: imageConfigRegistry(GinkgoT(), `"80/tcp":{}`),
		}
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "docker.io/traefik/whoami:v1.10", Type: appv1alpha1.TypeWebService,
				PortMode: appv1alpha1.PortModeWebImageV1, Replicas: 1,
				Env: []appv1alpha1.EnvVar{{Name: "WHOAMI_PORT_NUMBER", Value: "10000"}},
			},
		}
		if mutate != nil {
			mutate(app)
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })
		reconcileToFixedPoint(run)
		Expect(webImageHTTPSelectionPending(envtestApp(nn))).To(BeTrue(), "the test must exercise a pending discovery")
		return nn, r, run
	}
	// detect stands in for selectWebImageHTTP finding HTTP on 10000 after
	// probing the declared 80 (its own unit tests cover the probe and scan).
	detect := func(nn types.NamespacedName, run func() (ctrl.Result, error)) {
		app := envtestApp(nn)
		network := app.Status.ImageNetwork
		network.Ports, network.PrimaryPort, network.HTTPSelection = []int32{80, 10000}, 10000, appv1alpha1.HTTPSelectionDetected
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
	}

	It("rolls one template from dispatch to Running, and its Services target the detected port", func() {
		nn, r, run := firstRelease("first-deploy-undeclared-port", nil)
		first := envtestDeployment(nn).Spec.Template
		c := first.Spec.Containers[0]
		Expect(c.Ports).To(BeEmpty(), "a first release pins no container port while discovery may move it")
		for _, probe := range probesOf(c) {
			Expect(probe).NotTo(BeNil())
			Expect(healthProbeChecksOpenPort(probe.Exec)).To(BeTrue(), "probe %+v: want the any-open-port check", probe.ProbeHandler)
		}
		Expect(c.Env).To(ContainElement(corev1.EnvVar{Name: "PORT", Value: "10000"}))
		simulateTemplatePod(nn, "first", true)

		By("discovery choosing 10000 moves only the Services")
		detect(nn, run)
		Expect(apiequality.Semantic.DeepEqual(envtestDeployment(nn).Spec.Template, first)).To(BeTrue(),
			"the first deploy's pod template changed after discovery: %+v", envtestDeployment(nn).Spec.Template.Spec.Containers[0])
		assertImagePortService(nn, 10000)
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		reconcileToFixedPoint(run)
		Expect(apiequality.Semantic.DeepEqual(envtestDeployment(nn).Spec.Template, first)).To(BeTrue(), "the served first release rolled again")

		By("a new image over the serving release keeps w4/m320's guard and hold")
		r.HTTPClient = imageConfigRegistry(GinkgoT(), `"80/tcp":{},"81/tcp":{}`)
		app.Spec.Image = "docker.io/traefik/whoami:v1.11"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		app = envtestApp(nn)
		Expect(webImageHTTPSelectionGuardsServing(app)).To(BeTrue())
		candidate := envtestDeployment(nn)
		Expect(candidate.Spec.Template.Labels[labelRevision]).To(Equal("rev-2"))
		for _, probe := range probesOf(candidate.Spec.Template.Spec.Containers[0]) {
			Expect(probe.HTTPGet).NotTo(BeNil(), "a candidate over a serving release waits for HTTP, not any open port")
			Expect(probe.HTTPGet.Path).To(Equal("/"))
		}
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Selector).To(HaveKeyWithValue(labelRevision, "rev-1"), "the Services pin the serving release until promotion")
		Expect(candidate.Spec.Strategy.RollingUpdate.MaxUnavailable.IntValue()).To(BeZero(), "the serving pods are held")
	})

	It("keeps the HTTP check on the chosen port for a service with a Health Check Path", func() {
		nn, _, run := firstRelease("first-deploy-undeclared-health", func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/health" })
		detect(nn, run)
		for _, probe := range probesOf(envtestDeployment(nn).Spec.Template.Spec.Containers[0]) {
			path, _, port, ok := healthProbeExecArgs(probe.Exec)
			Expect(ok).To(BeTrue())
			Expect(path).To(Equal("/health"))
			Expect(port).To(Equal(10000))
		}
	})
})
