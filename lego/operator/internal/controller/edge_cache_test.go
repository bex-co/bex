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

var _ = Describe("Edge caching (w5/m151)", func() {
	ctx := context.Background()
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, BackupHelperImage: "ghcr.io/bex-co/bex:test"}
	})
	reconcileN := func(nn types.NamespacedName) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	create := func(name, tier, profile string) types.NamespacedName {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Tier: tier, CacheProfile: profile},
		}
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
	containers := func(nn types.NamespacedName) []corev1.Container {
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		return dep.Spec.Template.Spec.Containers
	}

	It("fronts a paid web service with the cache and points its Service at it", func() {
		nn := create("edgecache-pro", "pro", appv1alpha1.CacheProfileStatic)
		cs := containers(nn)
		Expect(cs).To(HaveLen(2))
		Expect(cs[1].Name).To(Equal(edgeCacheContainerName))
		Expect(envMap(cs[1].Env)).To(HaveKeyWithValue("BEX_EDGE_CACHE_UPSTREAM", "http://127.0.0.1:3000"))
		Expect(envMap(cs[1].Env)).To(HaveKeyWithValue("BEX_EDGE_CACHE_PROFILE", "origin-controlled"))
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports[0].TargetPort.StrVal).To(Equal(edgeCachePortName))
		Expect(cs[0].ReadinessProbe).NotTo(BeNil(), "the app keeps its own health check")

		By("a manual purge rolls the pods, emptying the cache")
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: "2026-10-21T00:00:00Z"}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(dep.Spec.Template.Annotations).To(HaveKeyWithValue(appv1alpha1.CachePurgedAtAnnotation, "2026-10-21T00:00:00Z"))

		By("turning caching off removes the proxy and targets the app again")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Spec.CacheProfile = appv1alpha1.CacheProfileNone
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		Expect(containers(nn)).To(HaveLen(1))
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports[0].TargetPort.StrVal).To(Equal(servingPortName))
	})

	It("never caches a free web service, as on Render", func() {
		nn := create("edgecache-free", "free", appv1alpha1.CacheProfileAll)
		Expect(containers(nn)).To(HaveLen(1))
	})
})
