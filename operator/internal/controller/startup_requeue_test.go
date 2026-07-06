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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/operator/api/v1alpha1"
)

// Operator restarts must re-reconcile every existing App (t002): operator-level
// config only reaches Apps through reconcile, and config changes always arrive
// as a restart. The mechanism is informer replay (a Create per App on cache
// sync, which GenerationChangedPredicate does not filter) — this test is the
// regression pin for that invariant: Apps are created BEFORE the manager
// starts, so nothing but the startup pass can touch them. If a future event
// filter drops the replay Creates, this test fails.
var _ = Describe("Startup requeue", func() {
	const n = 3
	ctx := context.Background()
	names := make([]string, n)

	BeforeEach(func() {
		for i := range n {
			names[i] = fmt.Sprintf("startup-requeue-%d", i)
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: names[i], Namespace: "default"},
				// Suspended + prebuilt image: reconcile parks the App at Hibernated
				// in one pass — no build, no readiness polling against a kubelet
				// that envtest doesn't have.
				Spec: appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Suspended: true},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
		}
	})

	AfterEach(func() {
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		for _, name := range names {
			nn := types.NamespacedName{Name: name, Namespace: "default"}
			app := &appv1alpha1.App{}
			if err := k8sClient.Get(ctx, nn, app); err == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) // finalizer path
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	It("reconciles all pre-existing Apps when the manager starts", func() {
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:  k8sClient.Scheme(),
			Metrics: metricsserver.Options{BindAddress: "0"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&AppReconciler{
			Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Mode: ModeKubernetes,
		}).SetupWithManager(mgr)).To(Succeed())

		mgrCtx, stop := context.WithCancel(ctx)
		defer stop()
		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrCtx)).To(Succeed())
		}()

		By("waiting for every App to be reconciled with no spec bump")
		for _, name := range names {
			nn := types.NamespacedName{Name: name, Namespace: "default"}
			Eventually(func(g Gomega) {
				app := &appv1alpha1.App{}
				g.Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
				g.Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))
				g.Expect(app.Status.ObservedGeneration).To(Equal(app.Generation))
			}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		}
	})
})
