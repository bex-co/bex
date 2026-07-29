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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("reconciling a static_site App", func() {
	const ns = "default"

	// A reconciler with the static-site plane configured.
	staticReconciler := func() *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			StaticStore: publish.Store{
				Bucket: "bex-static", Endpoint: "https://s3.example.com", Secret: "static-s3",
			},
			StaticServerService: "bex-static-server", StaticServerPort: 8080,
		}
	}

	reconcileN := func(r *AppReconciler, nn types.NamespacedName) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	// reconcileFailing drives the reconcile for an invalid spec: the finalizer pass
	// requeues cleanly, then the reconcile that reaches the static-site path calls
	// r.fail — which sets Phase=Failed and returns the error (so controller-runtime
	// requeues). We tolerate that error and assert on the recorded phase instead.
	reconcileFailing := func(r *AppReconciler, nn types.NamespacedName) {
		for range 3 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
	}

	It("serves from the static-server without a Deployment/Service", func() {
		const name = "site-serve"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Image:       "site:v1", // prebuilt => no build path
				PublishPath: "dist",
				Expose:      true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())

		// Pre-mark this generation as already published so the reconcile skips the
		// publish Job (which can't complete under envtest — no kubelet) and takes
		// the serving path.
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Status.ActiveRevision = revFor(app)
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())

		reconcileN(staticReconciler(), nn)

		By("creating no Deployment and no Service for the served content")
		var dep appsv1.Deployment
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, nn, &dep))).To(BeTrue())

		By("routing the host Ingress at the shared static-server")
		var ing networkingv1.Ingress
		Expect(k8sClient.Get(ctx, nn, &ing)).To(Succeed())
		Expect(ing.Spec.Rules).To(HaveLen(1))
		Expect(ing.Spec.Rules[0].Host).To(Equal(name + ".onbex.co"))
		backend := ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service
		aliasName := staticServerAliasName(name)
		Expect(backend.Name).To(Equal(aliasName))
		Expect(backend.Port.Number).To(Equal(int32(8080)))
		var alias corev1.Service
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: aliasName, Namespace: ns}, &alias)).To(Succeed())
		Expect(alias.Spec.Type).To(Equal(corev1.ServiceTypeExternalName))
		Expect(alias.Spec.ExternalName).To(Equal("bex-static-server.bex-system.svc.cluster.local"))
		Expect(alias.Spec.Ports).To(HaveLen(1))
		Expect(alias.Spec.Ports[0].Name).To(Equal("http"))
		Expect(alias.Spec.Ports[0].Port).To(Equal(int32(8080)))
		Expect(alias.OwnerReferences).To(HaveLen(1))
		Expect(alias.OwnerReferences[0].UID).To(Equal(app.UID))

		By("reporting Running with the served URL")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.URL).To(Equal("https://" + name + ".onbex.co"))
	})

	It("direct-publishes a no-build static_site by cloning the repo (w9/010)", func() {
		const name = "site-direct"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Repo:        "https://github.com/bex-co/bex",
				RootDir:     "examples/static-site",
				PublishPath: ".",
				Expose:      true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())

		// The reconcile blocks inside publish.Publish waiting for the Job, so
		// drive it from a goroutine and complete the Job by hand (envtest has no
		// kubelet to run it).
		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			r := staticReconciler()
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
		}()

		By("dispatching a clone-mode publish Job and no build Job")
		var job batchv1.Job
		jobNN := types.NamespacedName{Name: "pub-" + name + "-rev-1", Namespace: ns}
		Eventually(func() error { return k8sClient.Get(ctx, jobNN, &job) }, "30s", "250ms").Should(Succeed())
		Expect(job.Spec.Template.Spec.InitContainers).To(HaveLen(1))
		clone := job.Spec.Template.Spec.InitContainers[0]
		Expect(clone.Name).To(Equal("clone"))
		Expect(clone.Image).To(Equal(publish.DefaultGitImage))
		env := map[string]string{}
		for _, e := range clone.Env {
			env[e.Name] = e.Value
		}
		Expect(env["REPO"]).To(Equal("https://github.com/bex-co/bex"))
		Expect(env["SRC_DIR"]).To(Equal("examples/static-site"))
		var bld batchv1.Job
		Expect(apierrors.IsNotFound(
			k8sClient.Get(ctx, types.NamespacedName{Name: "bld-" + name + "-gen-1", Namespace: ns}, &bld),
		)).To(BeTrue())

		By("completing the publish Job by hand and letting the reconcile finish")
		now := metav1.Now()
		job.Status.StartTime = &now
		job.Status.CompletionTime = &now
		job.Status.Conditions = append(job.Status.Conditions,
			batchv1.JobCondition{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
			batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		)
		Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())
		Eventually(done, "30s").Should(BeClosed())

		By("serving from the static-server with no image recorded")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.Image).To(BeEmpty())
		var ing networkingv1.Ingress
		Expect(k8sClient.Get(ctx, nn, &ing)).To(Succeed())
		Expect(ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name).To(Equal(staticServerAliasName(name)))
	})

	It("fails a static_site with no publishPath", func() {
		const name = "site-nopublish"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Image: "site:v1", Expose: true},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		reconcileFailing(staticReconciler(), nn)

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
	})

	It("fails a static_site when the object store is unconfigured", func() {
		const name = "site-nostore"
		nn := types.NamespacedName{Name: name, Namespace: ns}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeStaticSite, Image: "site:v1", PublishPath: "dist", Expose: true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())

		// A reconciler with no StaticStore/StaticServerService configured.
		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
		}
		reconcileFailing(r, nn)

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
	})
})

// revFor is the revision string reconcileStaticSite computes for an App.
func revFor(app *appv1alpha1.App) string {
	return fmt.Sprintf("rev-%d", app.Generation)
}
