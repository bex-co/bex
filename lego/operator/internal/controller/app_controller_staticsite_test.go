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
		Expect(backend.Name).To(Equal("bex-static-server"))
		Expect(backend.Port.Number).To(Equal(int32(8080)))

		By("reporting Running with the served URL")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.URL).To(Equal("https://" + name + ".onbex.co"))
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
