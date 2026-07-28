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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Database deletion events", func() {
	It("runs the finalizer from the metadata-only delete update", func() {
		skipNameValidation := true
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: k8sClient.Scheme(), Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: controllerconfig.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&DatabaseReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}).SetupWithManager(mgr)).To(Succeed())

		managerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mgr.Start(managerCtx) }()
		DeferCleanup(func() {
			stop()
			Eventually(done, 10*time.Second).Should(Receive(BeNil()))
		})

		nn := types.NamespacedName{Name: "delete-event-db", Namespace: "default"}
		db := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec:       appv1alpha1.DatabaseSpec{Name: nn.Name, Plan: "free"},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		Eventually(func(g Gomega) {
			current := &appv1alpha1.Database{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			g.Expect(current.Finalizers).To(ContainElement(dbFinalizer))
			cluster := &unstructured.Unstructured{}
			cluster.SetGroupVersionKind(cnpgClusterGVK)
			g.Expect(k8sClient.Get(ctx, nn, cluster)).To(Succeed())
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Delete(ctx, db)).To(Succeed())
		Eventually(func() bool {
			err := k8sClient.Get(ctx, nn, &appv1alpha1.Database{})
			return apierrors.IsNotFound(err)
		}, 15*time.Second, 100*time.Millisecond).Should(BeTrue())
		cluster := &unstructured.Unstructured{}
		cluster.SetGroupVersionKind(cnpgClusterGVK)
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, nn, cluster))).To(BeTrue())
	})
})
