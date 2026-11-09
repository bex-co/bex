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
		Expect(second.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{Name: "PORT", Value: "80"}))
	})
})
