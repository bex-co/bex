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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m467: each named replica is a replica cluster of its own, with its own
// internal host; the primary Cluster holds only the primary and the HA
// standby, and adding or removing a replica leaves its spec alone.
var _ = Describe("Database read replica clusters", func() {
	DescribeTable("gives each replica its own reader and waits for every one",
		func(ctx SpecContext, ha bool) {
			name := fmt.Sprintf("dpg-replicas-ha-%t", ha)
			db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Finalizers: []string{dbFinalizer}},
				Spec: appv1alpha1.DatabaseSpec{Name: name, Plan: "basic-4gb", StorageGB: 10, HighAvailability: ha,
					ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "a"}, {Name: "b"}}}}
			Expect(k8sClient.Create(ctx, db)).To(Succeed())
			key := client.ObjectKeyFromObject(db)
			cluster := &unstructured.Unstructured{}
			cluster.SetGroupVersionKind(cnpgClusterGVK)
			replicaIn := func(ctx SpecContext, replica string) (*unstructured.Unstructured, bool) {
				live := &unstructured.Unstructured{}
				live.SetGroupVersionKind(cnpgClusterGVK)
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: appv1alpha1.ReadReplicaClusterName(name, replica)}, live)
				if apierrors.IsNotFound(err) {
					return nil, false
				}
				Expect(err).NotTo(HaveOccurred())
				return live, true
			}
			replica := func(name string) (*unstructured.Unstructured, bool) { return replicaIn(ctx, name) }
			DeferCleanup(func(ctx SpecContext) {
				for _, r := range []string{"a", "b"} {
					if live, ok := replicaIn(ctx, r); ok {
						_ = k8sClient.Delete(ctx, live)
					}
				}
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
			instances, _, _ := unstructured.NestedInt64(cluster.Object, "spec", "instances")
			Expect(instances).To(Equal(base), "the primary Cluster holds the primary and the HA standby only")
			Expect(db.Status.ReadReplicaStatuses).To(HaveLen(2))
			for i, status := range db.Status.ReadReplicaStatuses {
				Expect(status.InternalHost).To(Equal(appv1alpha1.ReadReplicaServiceName(name, db.Spec.ReadReplicas[i].Name) + ".default.svc"))
			}
			Expect(db.Status.ReadReplicaStatuses[0].InternalHost).NotTo(Equal(db.Status.ReadReplicaStatuses[1].InternalHost))
			_, cloned := replica("a")
			Expect(cloned).To(BeFalse(), "a replica was cloned before the primary served")

			// The primary serves: each replica is cloned from it, and the
			// Database waits for every replica's own instance.
			Expect(unstructured.SetNestedField(cluster.Object, base, "status", "readyInstances")).To(Succeed())
			Expect(unstructured.SetNestedField(cluster.Object, int64(18), "status", "pgDataImageInfo", "majorVersion")).To(Succeed())
			Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
			Expect(markApplicationDatabaseApplied(ctx, k8sClient, db)).To(Succeed())
			primarySpec, _, _ := unstructured.NestedMap(cluster.Object, "spec")
			run()
			for _, r := range []string{"a", "b"} {
				live, ok := replica(r)
				Expect(ok).To(BeTrue(), "replica %s has no cluster", r)
				Expect(metav1.IsControlledBy(live, db)).To(BeTrue())
				Expect(live.Object["spec"]).To(HaveKeyWithValue("instances", int64(1)))
				enabled, _, _ := unstructured.NestedBool(live.Object, "spec", "replica", "enabled")
				Expect(enabled).To(BeTrue())
			}
			Expect(db.Status.Phase).NotTo(Equal(appv1alpha1.DBPhaseReady), "Ready before the replicas' instances")
			afterAdd, _, _ := unstructured.NestedMap(cluster.Object, "spec")
			Expect(afterAdd).To(Equal(primarySpec), "adding replicas changed the primary Cluster")

			for _, r := range []string{"a", "b"} {
				live, _ := replica(r)
				Expect(unstructured.SetNestedField(live.Object, int64(1), "status", "readyInstances")).To(Succeed())
				Expect(k8sClient.Status().Update(ctx, live)).To(Succeed())
			}
			run()
			Expect(db.Status.Phase).To(Equal(appv1alpha1.DBPhaseReady))
			Expect(db.Status.HighAvailabilityEnabled).To(Equal(ha))

			// Removing a replica removes its cluster alone.
			db.Spec.ReadReplicas = []appv1alpha1.DatabaseReadReplica{{Name: "b"}}
			Expect(k8sClient.Update(ctx, db)).To(Succeed())
			run()
			_, kept := replica("a")
			Expect(kept).To(BeFalse())
			_, kept = replica("b")
			Expect(kept).To(BeTrue())
			afterRemove, _, _ := unstructured.NestedMap(cluster.Object, "spec")
			Expect(afterRemove).To(Equal(primarySpec), "removing a replica changed the primary Cluster")

			db.Spec.ReadReplicas = nil
			Expect(k8sClient.Update(ctx, db)).To(Succeed())
			run()
			_, kept = replica("b")
			Expect(kept).To(BeFalse())
			Expect(db.Status.ReadReplicaStatuses).To(BeEmpty())
			Expect(cnpgStorageGB(cluster)).To(Equal(int32(10)))
		},
		Entry("without HA", false),
		Entry("in addition to HA", true),
	)
})
