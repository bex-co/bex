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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Database named reader capacity", func() {
	DescribeTable("waits for every reader and removes only explicitly removed capacity",
		func(ctx SpecContext, readers int, ha bool) {
			name := fmt.Sprintf("dpg-readers-%d-ha-%t", readers, ha)
			db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Finalizers: []string{dbFinalizer}},
				Spec: appv1alpha1.DatabaseSpec{Name: name, Plan: "basic-1gb", StorageGB: 10, HighAvailability: ha, ReadReplicas: namedReaders(readers)}}
			Expect(k8sClient.Create(ctx, db)).To(Succeed())
			key := client.ObjectKeyFromObject(db)
			cluster := &unstructured.Unstructured{}
			cluster.SetGroupVersionKind(cnpgClusterGVK)
			DeferCleanup(func(ctx SpecContext) {
				_ = k8sClient.Delete(ctx, cluster)
				if err := k8sClient.Get(ctx, key, db); err == nil {
					db.Finalizers = nil
					Expect(k8sClient.Update(ctx, db)).To(Succeed())
					Expect(k8sClient.Delete(ctx, db)).To(Succeed())
				}
			})
			r := &DatabaseReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			run := func() {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, key, db)).To(Succeed())
				Expect(k8sClient.Get(ctx, key, cluster)).To(Succeed())
			}
			run()
			base := int64(1)
			if ha {
				base++
			}
			want := base + int64(readers)
			instances, _, _ := unstructured.NestedInt64(cluster.Object, "spec", "instances")
			Expect(instances).To(Equal(want))
			Expect(cnpgStorageGB(cluster)).To(Equal(int32(10)))
			Expect(db.Spec.HighAvailability).To(Equal(ha))
			Expect(db.Status.ReadReplicaStatuses).To(HaveLen(readers))
			Expect(db.Status.ReadReplicaStatuses[0].InternalHost).To(Equal(name + "-ro.default.svc"))

			Expect(unstructured.SetNestedField(cluster.Object, want-1, "status", "readyInstances")).To(Succeed())
			Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
			run()
			Expect(db.Status.Phase).NotTo(Equal(appv1alpha1.DBPhaseReady))
			Expect(unstructured.SetNestedField(cluster.Object, want, "status", "readyInstances")).To(Succeed())
			Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
			run()
			Expect(db.Status.Phase).To(Equal(appv1alpha1.DBPhaseReady))
			Expect(db.Status.HighAvailabilityEnabled).To(Equal(ha))

			db.Spec.ReadReplicas = nil
			Expect(k8sClient.Update(ctx, db)).To(Succeed())
			run()
			instances, _, _ = unstructured.NestedInt64(cluster.Object, "spec", "instances")
			Expect(instances).To(Equal(base))
			Expect(db.Status.ReadReplicaStatuses).To(BeEmpty())
			Expect(db.Spec.StorageGB).To(Equal(int32(10)))
			Expect(cnpgStorageGB(cluster)).To(Equal(int32(10)))
		},
		Entry("one reader without HA", 1, false),
		Entry("five readers without HA", 5, false),
		Entry("one reader in addition to HA", 1, true),
	)
})
