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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m511 t002: DatabaseReconciler probes ready read replicas, holds Syncing=true
// during hysteresis (initial sync and failover never trip), flips Syncing=false
// after the stale threshold with SyncStoppedAt pinned to the episode's start,
// and clears both on recovery so a second break opens a new episode.
var _ = Describe("Database read replica stale status", func() {
	It("tracks stale episodes with hysteresis and ignores initial sync and failover", func(ctx SpecContext) {
		const name = "dpg-replica-sync-env"
		db := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Finalizers: []string{dbFinalizer}},
			Spec: appv1alpha1.DatabaseSpec{
				Name: name, Plan: "1c-2g", StorageGB: 10, HighAvailability: true,
				ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "a"}, {Name: "b"}},
			},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		key := client.ObjectKeyFromObject(db)
		primary := &unstructured.Unstructured{}
		primary.SetGroupVersionKind(cnpgClusterGVK)
		replicaObj := func(ctx SpecContext, replica string) *unstructured.Unstructured {
			live := &unstructured.Unstructured{}
			live.SetGroupVersionKind(cnpgClusterGVK)
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: appv1alpha1.ReadReplicaClusterName(name, replica)}, live)).To(Succeed())
			return live
		}
		DeferCleanup(func(ctx SpecContext) {
			for _, rep := range []string{"a", "b"} {
				live := &unstructured.Unstructured{}
				live.SetGroupVersionKind(cnpgClusterGVK)
				if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: appv1alpha1.ReadReplicaClusterName(name, rep)}, live); err == nil {
					_ = k8sClient.Delete(ctx, live)
				}
			}
			_ = k8sClient.Delete(ctx, primary)
			if err := k8sClient.Get(ctx, key, db); err == nil {
				db.Finalizers = nil
				Expect(k8sClient.Update(ctx, db)).To(Succeed())
				Expect(k8sClient.Delete(ctx, db)).To(Succeed())
			}
		})

		now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
		samples := map[string]ReadReplicaSyncSample{
			"a": {IsWalReceiverActive: true},
			"b": {IsWalReceiverActive: true},
		}
		r := &DatabaseReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Now:    func() time.Time { return now },
			ReadReplicaSyncReader: func(_ context.Context, _ *appv1alpha1.Database, replicaName string, _ *unstructured.Unstructured) (ReadReplicaSyncSample, error) {
				return samples[replicaName], nil
			},
		}
		run := func() reconcile.Result {
			res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, key, db)).To(Succeed())
			Expect(k8sClient.Get(ctx, key, primary)).To(Succeed())
			return res
		}
		statusOf := func(rep string) appv1alpha1.DatabaseReadReplicaStatus {
			for _, s := range db.Status.ReadReplicaStatuses {
				if s.Name == rep {
					return s
				}
			}
			Fail("missing status for replica " + rep)
			return appv1alpha1.DatabaseReadReplicaStatus{}
		}

		// Initial creation: primary ready, replicas created but still bootstrapping (readyInstances=0).
		run()
		Expect(unstructured.SetNestedField(primary.Object, int64(2), "status", "readyInstances")).To(Succeed())
		Expect(unstructured.SetNestedField(primary.Object, int64(17), "status", "pgDataImageInfo", "majorVersion")).To(Succeed())
		Expect(unstructured.SetNestedField(primary.Object, name+"-1", "status", "currentPrimary")).To(Succeed())
		Expect(k8sClient.Status().Update(ctx, primary)).To(Succeed())
		Expect(markApplicationDatabaseApplied(ctx, k8sClient, db)).To(Succeed())
		samples["a"] = ReadReplicaSyncSample{IsWalReceiverActive: false} // even if not streaming during initial sync
		run()
		Expect(statusOf("a").Syncing).To(BeNil(), "initial sync must never mark a replica stale")
		Expect(statusOf("a").SyncStoppedAt).To(BeEmpty())

		// Both replicas finish initial sync and stream.
		samples["a"] = ReadReplicaSyncSample{IsWalReceiverActive: true}
		for _, rep := range []string{"a", "b"} {
			live := replicaObj(ctx, rep)
			Expect(unstructured.SetNestedField(live.Object, int64(1), "status", "readyInstances")).To(Succeed())
			Expect(k8sClient.Status().Update(ctx, live)).To(Succeed())
		}
		res := run()
		Expect(db.Status.Phase).To(Equal(appv1alpha1.DBPhaseReady))
		Expect(res.RequeueAfter).To(Equal(readReplicaSyncProbeInterval))
		Expect(statusOf("a").Syncing).To(HaveValue(BeTrue()))
		Expect(statusOf("b").Syncing).To(HaveValue(BeTrue()))

		// Primary failover: walreceiver dips briefly (~15s), primary moves to -2, then resumes.
		samples["a"] = ReadReplicaSyncSample{IsWalReceiverActive: false}
		samples["b"] = ReadReplicaSyncSample{IsWalReceiverActive: false}
		Expect(unstructured.SetNestedField(primary.Object, name+"-2", "status", "currentPrimary")).To(Succeed())
		Expect(k8sClient.Status().Update(ctx, primary)).To(Succeed())
		now = now.Add(30 * time.Second)
		run()
		Expect(statusOf("a").Syncing).To(HaveValue(BeTrue()), "failover dip within hysteresis must stay syncing=true")
		Expect(statusOf("a").SyncStoppedAt).To(Equal(now.Format(time.RFC3339)))

		samples["a"] = ReadReplicaSyncSample{IsWalReceiverActive: true}
		samples["b"] = ReadReplicaSyncSample{IsWalReceiverActive: true}
		now = now.Add(15 * time.Second)
		run()
		Expect(statusOf("a").Syncing).To(HaveValue(BeTrue()))
		Expect(statusOf("a").SyncStoppedAt).To(BeEmpty())
		Expect(statusOf("b").SyncStoppedAt).To(BeEmpty())

		// Real break on replica "a" only: hysteresis holds before 2m, flips false at 2m.
		samples["a"] = ReadReplicaSyncSample{IsWalReceiverActive: false}
		episode1Start := now.Add(30 * time.Second)
		now = episode1Start
		run()
		Expect(statusOf("a").Syncing).To(HaveValue(BeTrue()))
		Expect(statusOf("a").SyncStoppedAt).To(Equal(episode1Start.Format(time.RFC3339)))
		Expect(statusOf("b").Syncing).To(HaveValue(BeTrue()))
		Expect(statusOf("b").SyncStoppedAt).To(BeEmpty())

		now = episode1Start.Add(90 * time.Second)
		run()
		Expect(statusOf("a").Syncing).To(HaveValue(BeTrue()), "still under 2m threshold")

		now = episode1Start.Add(2 * time.Minute)
		run()
		Expect(statusOf("a").Syncing).To(HaveValue(BeFalse()), "stale once 2m threshold elapsed")
		Expect(statusOf("a").SyncStoppedAt).To(Equal(episode1Start.Format(time.RFC3339)))
		Expect(statusOf("b").Syncing).To(HaveValue(BeTrue()))

		// Recovery clears Syncing back to true and clears SyncStoppedAt.
		samples["a"] = ReadReplicaSyncSample{IsWalReceiverActive: true}
		now = now.Add(30 * time.Second)
		run()
		Expect(statusOf("a").Syncing).To(HaveValue(BeTrue()))
		Expect(statusOf("a").SyncStoppedAt).To(BeEmpty())

		// Second break records a distinct SyncStoppedAt once it crosses 2m.
		samples["a"] = ReadReplicaSyncSample{IsWalReceiverActive: false}
		episode2Start := now.Add(30 * time.Second)
		now = episode2Start
		run()
		now = episode2Start.Add(2 * time.Minute)
		run()
		Expect(statusOf("a").Syncing).To(HaveValue(BeFalse()))
		Expect(statusOf("a").SyncStoppedAt).To(Equal(episode2Start.Format(time.RFC3339)))
		Expect(statusOf("a").SyncStoppedAt).NotTo(Equal(episode1Start.Format(time.RFC3339)))
	})
})
