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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m235: a static site's host answered Traefik's 404, then the
// static-server's "no static site for host", until its first publish served.
// It now answers the activator's first-deploy response from creation, and the
// static-server takes the host one pass after the first revision is recorded.
var _ = Describe("a static site's first publish (w4/m235)", func() {
	const ns = "default"

	reconciler := func() *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			StaticStore: publish.Store{
				Bucket: "bex-static", Endpoint: "https://s3.example.com", Secret: "static-s3",
			},
			StaticServerService: "bex-static-server", StaticServerPort: 8080,
			ActivatorService: "bex-activator", ActivatorNamespace: "bex-system", ActivatorPort: 8888,
		}
	}
	backend := func(nn types.NamespacedName) string {
		var ing networkingv1.Ingress
		Expect(k8sClient.Get(ctx, nn, &ing)).To(Succeed())
		return ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name
	}
	create := func(name string) types.NamespacedName {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/bex",
				RootDir: "examples/static-site", PublishPath: ".", Expose: true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, app) })
		credential := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "static-s3", Namespace: ns},
			Data:       map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("a"), "AWS_SECRET_ACCESS_KEY": []byte("s")},
		}
		Expect(k8sClient.Create(ctx, credential)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, credential) })
		return types.NamespacedName{Name: name, Namespace: ns}
	}
	// settle finishes the first publish Job by hand (envtest has no kubelet).
	settle := func(nn types.NamespacedName, failed bool) {
		jobNN := types.NamespacedName{Name: publish.JobName(nn.Name, releaseRevision(envtestApp(nn))), Namespace: ns}
		var job batchv1.Job
		Eventually(func() error { return k8sClient.Get(ctx, jobNN, &job) }, "30s", "250ms").Should(Succeed())
		if failed {
			failEnvtestJob(&job, "BackoffLimitExceeded", "the publish directory does not exist")
		} else {
			completeEnvtestJob(&job, metav1.Now())
		}
	}

	It("answers from the activator until the static-server can serve it", func() {
		nn := create("site-first-route")
		r := reconciler()
		for range 3 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
		By("routing the host to the activator while the first publish runs")
		Expect(envtestApp(nn).Status.ActiveRevision).To(BeEmpty())
		Expect(backend(nn)).To(Equal(activatorAliasName(nn.Name)))

		By("recording the revision first, still on the activator")
		settle(nn, false)
		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.RequeueAfter).To(Equal(staticHandoverDelay))
		Expect(envtestApp(nn).Status.ActiveRevision).NotTo(BeEmpty())
		Expect(backend(nn)).To(Equal(activatorAliasName(nn.Name)))

		By("handing the host to the static-server on the next pass")
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		Expect(backend(nn)).To(Equal(staticServerAliasName(nn.Name)))
		Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
	})

	It("moves a served site to the static-server when a newer deploy follows the handover at once", func() {
		nn := create("site-first-then-second")
		r := reconciler()
		for range 3 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
		settle(nn, false)
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		Expect(backend(nn)).To(Equal(activatorAliasName(nn.Name)))

		By("a second deploy arriving before the handover pass")
		app := envtestApp(nn)
		app.Spec.Branch = "second"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		for range 2 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
		Expect(releaseRevision(envtestApp(nn))).NotTo(Equal(envtestApp(nn).Status.ActiveRevision), "the second publish is in flight")
		Expect(backend(nn)).To(Equal(staticServerAliasName(nn.Name)), "the first revision serves through the second publish")
	})

	It("keeps a failed first publish on the activator's failed answer", func() {
		nn := create("site-first-fail")
		r := reconciler()
		for range 3 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
		settle(nn, true)
		for range 2 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		Expect(app.Status.ActiveRevision).To(BeEmpty())
		Expect(backend(nn)).To(Equal(activatorAliasName(nn.Name)))
	})
})
