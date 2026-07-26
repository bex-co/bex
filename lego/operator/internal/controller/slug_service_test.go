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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The slug-named Service gives a store-managed App its Render-shaped internal
// address `<slug>:<port>` (docs/ADR041-service-addresses.md D2, w9/m57).
var _ = Describe("Slug-named Service (ADR041 D2)", func() {
	ctx := context.Background()

	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	reconcileN := func(nn types.NamespacedName) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	// cleanup deletes the App (running its finalizer) and any Services envtest's
	// missing GC controller would otherwise leak.
	cleanup := func(nn types.NamespacedName, svcNames ...string) {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			reconcileN(nn)
		}
		for _, name := range svcNames {
			_ = k8sClient.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace}})
		}
	}
	svcAbsent := func(name string) {
		Expect(errors.IsNotFound(k8sClient.Get(ctx,
			types.NamespacedName{Name: name, Namespace: "default"}, &corev1.Service{}))).To(BeTrue(),
			"Service %s should be absent", name)
	}

	Context("store-managed App (spec.subdomain minted)", func() {
		const name = "tea-x1-api" // the w4/m19 <tenant>-<name> CR-name shape
		const slug = "api-x1y2"   // the globally-unique slug
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		AfterEach(func() { cleanup(nn, name, slug) })

		It("creates the slug Service beside the CR-named one, identically shaped", func() {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Type: appv1alpha1.TypePrivateService, Subdomain: slug,
					Image: "traefik/whoami", Port: 8080, Replicas: 1,
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN(nn)

			By("both Services exist over the same pods, byte-equal in shape")
			primary := &corev1.Service{}
			Expect(k8sClient.Get(ctx, nn, primary)).To(Succeed())
			slugSvc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: slug, Namespace: "default"}, slugSvc)).To(Succeed())
			Expect(slugSvc.Spec.Selector).To(Equal(primary.Spec.Selector))
			Expect(slugSvc.Spec.Ports).To(Equal(primary.Spec.Ports))
			Expect(slugSvc.Spec.Ports[0].Port).To(Equal(int32(8080)))

			By("the slug Service is controller-owned by the App (GC on App delete)")
			got := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
			Expect(metav1.IsControlledBy(slugSvc, got)).To(BeTrue())
		})
	})

	Context("non-addressable App with a leftover slug alias", func() {
		// spec.type is CRD-immutable (app_types.go XValidation), so a live type
		// flip cannot create this state — but a CR predating the rule, object
		// surgery, or a projector edge can. The delete half of the bidirectional
		// reconcile converges it away regardless of which type path runs.
		const name = "tea-c1-job"
		const slug = "job-a1b2"
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		AfterEach(func() { cleanup(nn, name, slug) })

		It("removes an owned slug alias from a cron_job App", func() {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Type: appv1alpha1.TypeCronJob, Subdomain: slug, Schedule: "*/5 * * * *",
					Image: "busybox:1.36", Command: "true", Port: 3000, Replicas: 1,
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			got := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())

			By("hand-planting the leftover alias, controller-owned by the App")
			leftover := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: slug, Namespace: "default"},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{labelApp: name},
					Ports:    []corev1.ServicePort{{Port: 3000}},
				},
			}
			Expect(controllerutil.SetControllerReference(got, leftover, k8sClient.Scheme())).To(Succeed())
			Expect(k8sClient.Create(ctx, leftover)).To(Succeed())

			By("reconciling the cron_job converges the alias away")
			reconcileN(nn)
			svcAbsent(slug)
		})
	})

	Context("legacy slug-less App", func() {
		const name = "legacy-api"
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		AfterEach(func() { cleanup(nn, name) })

		It("creates exactly one Service (the slug falls back to the CR name)", func() {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Replicas: 1},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN(nn)

			got := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
			owned := 0
			list := &corev1.ServiceList{}
			Expect(k8sClient.List(ctx, list)).To(Succeed())
			for i := range list.Items {
				if metav1.IsControlledBy(&list.Items[i], got) {
					owned++
				}
			}
			Expect(owned).To(Equal(1), "a slug-less App must own exactly its CR-named Service")
		})
	})

	Context("slug squatted by another App's Service", func() {
		victimNN := types.NamespacedName{Name: "victim", Namespace: "default"}
		claimNN := types.NamespacedName{Name: "tea-z9-web", Namespace: "default"}
		AfterEach(func() {
			cleanup(claimNN, "tea-z9-web")
			cleanup(victimNN, "victim")
		})

		It("never adopts or rewrites a Service owned by a different App", func() {
			By("a legacy App owns the Service its name claims")
			victim := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "victim", Namespace: "default"},
				Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Replicas: 1},
			}
			Expect(k8sClient.Create(ctx, victim)).To(Succeed())
			reconcileN(victimNN)
			Expect(k8sClient.Get(ctx, victimNN, &corev1.Service{})).To(Succeed())

			By("another App whose slug collides with that name reconciles without failing")
			claimant := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "tea-z9-web", Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Subdomain: "victim", // collides with the legacy CR-named Service
					Image:     "traefik/whoami", Port: 9090, Replicas: 1,
				},
			}
			Expect(k8sClient.Create(ctx, claimant)).To(Succeed())
			reconcileN(claimNN)

			By("the victim's Service is untouched — still victim-owned, still port 3000")
			svc := &corev1.Service{}
			Expect(k8sClient.Get(ctx, victimNN, svc)).To(Succeed())
			gotVictim := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, victimNN, gotVictim)).To(Succeed())
			Expect(metav1.IsControlledBy(svc, gotVictim)).To(BeTrue())
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(3000)))
			Expect(svc.Spec.Selector).To(HaveKeyWithValue(labelApp, "victim"))

			By("deleting the claimant leaves the victim's Service alone")
			gotClaim := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, claimNN, gotClaim)).To(Succeed())
			Expect(k8sClient.Delete(ctx, gotClaim)).To(Succeed())
			reconcileN(claimNN)
			Expect(k8sClient.Get(ctx, victimNN, &corev1.Service{})).To(Succeed())
		})
	})
})

// internalURL is the surfaced no-public-host address (docs/ADR041 D1, w9/m57
// t005): Render-shaped `http://<slug>:<port>` when the slug differs from the
// CR name (so the slug Service exists), the legacy k8s FQDN otherwise — the
// same predicate reconcileSlugService uses, so the URL and the Service that
// answers it cannot disagree.
func TestInternalURL(t *testing.T) {
	storeManaged := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "tea-x1-api", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Subdomain: "api-x1y2"},
	}
	if got, want := internalURL(storeManaged, 8080), "http://api-x1y2:8080"; got != want {
		t.Errorf("store-managed internalURL = %q, want %q", got, want)
	}

	legacy := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-api", Namespace: "default"},
	}
	if got, want := internalURL(legacy, 3000), "http://legacy-api.default.svc:3000"; got != want {
		t.Errorf("legacy internalURL = %q, want %q", got, want)
	}

	// Adopted-legacy corner (eden-cms-v2 class): the store minted slug == CR
	// name, so no slug Service exists — the URL must stay the FQDN, matching
	// reconcileSlugService's no-op instead of flipping to a bare short name.
	adopted := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "eden-cms-v2", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Subdomain: "eden-cms-v2"},
	}
	if got, want := internalURL(adopted, 3000), "http://eden-cms-v2.default.svc:3000"; got != want {
		t.Errorf("adopted internalURL = %q, want %q", got, want)
	}
}
