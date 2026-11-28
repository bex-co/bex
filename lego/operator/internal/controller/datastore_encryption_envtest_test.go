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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m714: managed Postgres and Key Value volumes are encrypted at rest, as
// Render's are. Every Cluster a Database projects (its primary and HA standby,
// and each read replica's) names the datastore class, every CNPG claim on it
// gets its own passphrase, and a new Key Value's claim template names it with
// the passphrase minted before the StatefulSet exists.
var _ = Describe("Datastore volume encryption", func() {
	const luks = "hcloud-volumes-luks"

	storageClassOf := func(cluster *unstructured.Unstructured) string {
		class, _, _ := unstructured.NestedString(cluster.Object, "spec", "storage", "storageClass")
		return class
	}

	DescribeTable("puts every instance of a Database on the datastore class",
		func(ctx SpecContext, ha bool) {
			name := fmt.Sprintf("dpg-luks-ha-%t", ha)
			db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Finalizers: []string{dbFinalizer}},
				Spec: appv1alpha1.DatabaseSpec{Name: name, Plan: "basic-4gb", StorageGB: 10, HighAvailability: ha,
					ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "a"}}}}
			Expect(k8sClient.Create(ctx, db)).To(Succeed())
			key := client.ObjectKeyFromObject(db)
			cluster := &unstructured.Unstructured{}
			cluster.SetGroupVersionKind(cnpgClusterGVK)
			replica := &unstructured.Unstructured{}
			replica.SetGroupVersionKind(cnpgClusterGVK)
			replicaKey := client.ObjectKey{Namespace: "default", Name: appv1alpha1.ReadReplicaClusterName(name, "a")}
			DeferCleanup(func(ctx SpecContext) {
				_ = k8sClient.Delete(ctx, replica)
				_ = k8sClient.Delete(ctx, cluster)
				if err := k8sClient.Get(ctx, key, db); err == nil {
					db.Finalizers = nil
					Expect(k8sClient.Update(ctx, db)).To(Succeed())
					Expect(k8sClient.Delete(ctx, db)).To(Succeed())
				}
			})
			r := &DatabaseReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), StorageClass: luks}
			run := func() {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				Expect(err).NotTo(HaveOccurred())
				Expect(k8sClient.Get(ctx, key, cluster)).To(Succeed())
			}
			run()
			Expect(storageClassOf(cluster)).To(Equal(luks), "the primary Cluster (primary and HA standby)")

			// The primary serves, so the replica is cloned: on the same class.
			Expect(unstructured.SetNestedField(cluster.Object, int64(1), "status", "readyInstances")).To(Succeed())
			Expect(unstructured.SetNestedField(cluster.Object, int64(18), "status", "pgDataImageInfo", "majorVersion")).To(Succeed())
			Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
			run()
			Expect(k8sClient.Get(ctx, replicaKey, replica)).To(Succeed())
			Expect(storageClassOf(replica)).To(Equal(luks), "the read replica's Cluster")
		},
		Entry("single instance", false),
		Entry("high availability", true),
	)

	// CloudNativePG 1.30 takes a changed class on a live Cluster and builds
	// it into new claims only, so an existing database's next standby or
	// replacement is encrypted while its live volumes stay where they are.
	It("moves an existing Cluster's spec onto the datastore class", func(ctx SpecContext) {
		name := "dpg-luks-existing"
		db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Finalizers: []string{dbFinalizer}},
			Spec: appv1alpha1.DatabaseSpec{Name: name, Plan: "basic-256mb"}}
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
		before := &DatabaseReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := before.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, key, cluster)).To(Succeed())
		Expect(storageClassOf(cluster)).To(Equal(defaultDatastoreStorageClass))

		after := &DatabaseReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), StorageClass: luks}
		_, err = after.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Get(ctx, key, cluster)).To(Succeed())
		Expect(storageClassOf(cluster)).To(Equal(luks))
	})

	// The API server turns the minted stringData into data: the passphrase a
	// second pass reads back, and must keep, is the stored one.
	It("mints a stored passphrase for each CNPG claim and never rewrites it", func(ctx SpecContext) {
		r := &DatastoreVolumeKeyReconciler{Client: k8sClient, StorageClass: luks}
		claims := make([]*corev1.PersistentVolumeClaim, 0, 2)
		for _, serial := range []string{"1", "2"} {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Name: "dpg-luks-claims-" + serial, Namespace: "default",
					Labels: map[string]string{cnpgClusterLabel: "dpg-luks-claims", appv1alpha1.LabelComponent: componentDatabase}},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					StorageClassName: new(luks),
					Resources:        corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")}},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())
			claims = append(claims, pvc)
		}
		DeferCleanup(func(ctx SpecContext) {
			for _, pvc := range claims {
				_ = k8sClient.Delete(ctx, pvc)
				_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: volumeLUKSSecretName(pvc.Name), Namespace: "default"}})
			}
		})
		passphrases := map[string]string{}
		for range 3 {
			for _, pvc := range claims {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(pvc)})
				Expect(err).NotTo(HaveOccurred())
				secret := &corev1.Secret{}
				Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: volumeLUKSSecretName(pvc.Name)}, secret)).To(Succeed())
				stored := string(secret.Data[luksPassphraseKey])
				Expect(stored).NotTo(BeEmpty())
				if first, ok := passphrases[pvc.Name]; ok {
					Expect(stored).To(Equal(first), "a later reconcile rewrote %s's passphrase", pvc.Name)
				}
				passphrases[pvc.Name] = stored
				Expect(secret.OwnerReferences).To(HaveLen(1))
				Expect(secret.OwnerReferences[0].UID).To(Equal(pvc.UID))
			}
		}
		Expect(passphrases[claims[0].Name]).NotTo(Equal(passphrases[claims[1].Name]), "two volumes share a passphrase")
	})

	It("gives a new Key Value an encrypted claim whose passphrase exists first", func(ctx SpecContext) {
		name := "red-luks"
		kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.KeyValueSpec{Name: name, Plan: "starter"}}
		Expect(k8sClient.Create(ctx, kv)).To(Succeed())
		nn := client.ObjectKeyFromObject(kv)
		secretKey := client.ObjectKey{Namespace: "default", Name: volumeLUKSSecretName(keyValuePVCName(name))}
		DeferCleanup(func(ctx SpecContext) {
			_ = k8sClient.Delete(ctx, &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}})
			_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretKey.Name, Namespace: "default"}})
			if live := (&appv1alpha1.KeyValue{}); k8sClient.Get(ctx, nn, live) == nil {
				live.Finalizers = nil
				_ = k8sClient.Update(ctx, live)
				_ = k8sClient.Delete(ctx, live)
			}
		})
		r := &KeyValueReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), StorageClass: luks}
		reconcileOnce := func() {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
		reconcileOnce()
		sts := &appsv1.StatefulSet{}
		Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
		Expect(sts.Spec.VolumeClaimTemplates).To(HaveLen(1))
		Expect(*sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal(luks))
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())
		first := string(secret.Data[luksPassphraseKey])
		Expect(first).NotTo(BeEmpty())
		Expect(metav1.IsControlledBy(secret, kv)).To(BeTrue())

		// A later reconcile keeps the passphrase, and the existing
		// StatefulSet's immutable claim template keeps its class even when
		// the configured class moves.
		r.StorageClass = "another-class"
		reconcileOnce()
		reconcileOnce()
		Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())
		Expect(string(secret.Data[luksPassphraseKey])).To(Equal(first))
		Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
		Expect(*sts.Spec.VolumeClaimTemplates[0].Spec.StorageClassName).To(Equal(luks))
	})
})
