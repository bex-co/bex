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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Pod hardening defaults (w7/m2)", func() {
	const namespace = "default"
	ctx := context.Background()

	reconcileApp := func(r *AppReconciler, name string) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
			})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	getDep := func(name string) *appsv1.Deployment {
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, dep)).To(Succeed())
		return dep
	}

	Context("with a standard high-port app (traefik/whoami :3000)", func() {
		const name = "hardening-sec-app"

		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes,
			}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
		})
		AfterEach(func() {
			app := &appv1alpha1.App{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app); err == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileApp(r, name)
			}
		})

		It("stamps the hardened securityContext on the tenant container", func() {
			reconcileApp(r, name)
			c := getDep(name).Spec.Template.Spec.Containers[0]

			By("allowPrivilegeEscalation is false")
			Expect(c.SecurityContext).NotTo(BeNil())
			Expect(c.SecurityContext.AllowPrivilegeEscalation).NotTo(BeNil())
			Expect(*c.SecurityContext.AllowPrivilegeEscalation).To(BeFalse())

			By("all capabilities dropped, none added — high port needs no NET_BIND_SERVICE")
			Expect(c.SecurityContext.Capabilities).NotTo(BeNil())
			Expect(c.SecurityContext.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
			Expect(c.SecurityContext.Capabilities.Add).To(BeEmpty())

			By("RuntimeDefault seccomp profile set")
			Expect(c.SecurityContext.SeccompProfile).NotTo(BeNil())
			Expect(c.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
		})

		It("does NOT set runAsNonRoot — tenant images may run as root", func() {
			reconcileApp(r, name)
			c := getDep(name).Spec.Template.Spec.Containers[0]

			Expect(c.SecurityContext).NotTo(BeNil())
			Expect(c.SecurityContext.RunAsNonRoot).To(BeNil(),
				"runAsNonRoot must not be set: tenant images may legitimately run as root (PSS baseline, not restricted)")
		})

		It("disables ServiceAccount token automounting on the pod", func() {
			reconcileApp(r, name)
			podSpec := getDep(name).Spec.Template.Spec

			Expect(podSpec.AutomountServiceAccountToken).NotTo(BeNil())
			Expect(*podSpec.AutomountServiceAccountToken).To(BeFalse(),
				"tenant pod must not carry a cluster credential (no SA token volume)")
		})
	})
})
