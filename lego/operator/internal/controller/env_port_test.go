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

var _ = Describe("A service's own PORT (w5/m149)", func() {
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
	secret := func(name string, data map[string]string) {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, StringData: data}
		Expect(k8sClient.Create(ctx, s)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, s) })
	}
	serving := func(nn types.NamespacedName) (container corev1.Container, servicePort int32, envPort int32) {
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		return dep.Spec.Template.Spec.Containers[0], svc.Spec.Ports[0].Port, app.Status.EnvPort
	}
	create := func(name string, mutate func(*appv1alpha1.App)) types.NamespacedName {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, HealthCheckPath: "/healthz"},
		}
		mutate(app)
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		DeferCleanup(func() {
			if got := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, got) == nil {
				Expect(k8sClient.Delete(ctx, got)).To(Succeed())
				reconcileN(nn)
			}
		})
		reconcileN(nn)
		return nn
	}

	It("serves, routes and probes on a linked group's PORT, and the service's own value beats it", func() {
		secret("envport-group-env", map[string]string{"PORT": "7000"})
		nn := create("envport-group", func(a *appv1alpha1.App) { a.Spec.EnvFromSecrets = []string{"envport-group-env"} })
		c, svcPort, envPort := serving(nn)
		Expect(envPort).To(Equal(int32(7000)))
		Expect(svcPort).To(Equal(int32(7000)))
		Expect(c.Ports).To(ContainElement(HaveField("ContainerPort", int32(7000))))
		Expect(c.ReadinessProbe.HTTPGet.Port.String()).To(Or(Equal("7000"), Equal(servingPortName)))
		Expect(envMap(c.Env)).To(HaveKeyWithValue("PORT", "7000"))

		secret("envport-own-env", map[string]string{"PORT": "8000"})
		nn = create("envport-own", func(a *appv1alpha1.App) {
			a.Spec.EnvFromSecrets = []string{"envport-group-env"}
			a.Spec.EnvFromSecret = "envport-own-env"
		})
		_, svcPort, envPort = serving(nn)
		Expect(envPort).To(Equal(int32(8000)), "the service's own set is the last envFrom source, so it wins")
		Expect(svcPort).To(Equal(int32(8000)))
	})

	It("ignores an unusable PORT and reverts when the variable goes away", func() {
		nn := create("envport-bad", func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: "PORT", Value: "not-a-port"}}
		})
		_, svcPort, envPort := serving(nn)
		Expect(envPort).To(BeZero())
		Expect(svcPort).To(Equal(int32(3000)), "an unusable PORT never takes the service off the network")

		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Spec.Env = []appv1alpha1.EnvVar{{Name: "PORT", Value: "9090"}}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		_, svcPort, envPort = serving(nn)
		Expect([]int32{svcPort, envPort}).To(Equal([]int32{9090, 9090}))

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Spec.Env = nil
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		_, svcPort, envPort = serving(nn)
		Expect([]int32{svcPort, envPort}).To(Equal([]int32{3000, 0}), "removing PORT returns to the service port")
	})
})
