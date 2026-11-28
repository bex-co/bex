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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	testLUKSClass = "hcloud-volumes-luks"
	volumeKeyNS   = "tea-volumekey"
)

func volumeKeyScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

// cnpgClaim is a claim as CloudNativePG makes it for a tenant Cluster: named
// <cluster>-<serial>, labelled with its cluster and, through the Database
// controller's inheritedMetadata, the database component.
func cnpgClaim(cluster, serial, class string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster + "-" + serial, Namespace: volumeKeyNS, UID: types.UID("uid-" + cluster + "-" + serial),
			Labels: map[string]string{cnpgClusterLabel: cluster, appv1alpha1.LabelComponent: componentDatabase},
		},
		Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: new(class)},
	}
}

func keyValueClaim(kv, class string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: keyValuePVCName(kv), Namespace: volumeKeyNS, UID: types.UID("uid-" + kv),
			Labels: map[string]string{labelKeyValue: kv},
		},
		Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: new(class)},
	}
}

func reconcileVolumeKey(t *testing.T, r *DatastoreVolumeKeyReconciler, pvc *corev1.PersistentVolumeClaim) error {
	t.Helper()
	_, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(pvc)})
	return err
}

func volumeKeySecret(t *testing.T, c client.Client, claim string) (*corev1.Secret, bool) {
	t.Helper()
	secret := &corev1.Secret{}
	err := c.Get(context.Background(), client.ObjectKey{Namespace: volumeKeyNS, Name: claim + "-luks"}, secret)
	if apierrors.IsNotFound(err) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return secret, true
}

// Every claim CNPG makes for a Database, including the ones it adds later
// with a higher serial (an HA standby, a replacement, a read replica's
// instance), gets its own passphrase, owned by the claim alone, through the
// uncached client: the tenant namespace is outside the Secret informer.
func TestVolumeKeyMintsOnePassphrasePerPostgresClaim(t *testing.T) {
	primary := cnpgClaim("dpg-a", "1", testLUKSClass)
	standby := cnpgClaim("dpg-a", "2", testLUKSClass)
	replacement := cnpgClaim("dpg-a", "3", testLUKSClass)
	replica := cnpgClaim(appv1alpha1.ReadReplicaClusterName("dpg-a", "r1"), "1", testLUKSClass)
	real := fake.NewClientBuilder().WithScheme(volumeKeyScheme(t)).WithObjects(primary, standby, replacement, replica).Build()
	r := &DatastoreVolumeKeyReconciler{
		Client:       namespaceScopedClient{Client: real, allowed: "default"},
		SecretClient: real,
		StorageClass: testLUKSClass,
	}
	seen := map[string]bool{}
	for _, pvc := range []*corev1.PersistentVolumeClaim{primary, standby, replacement, replica} {
		if err := reconcileVolumeKey(t, r, pvc); err != nil {
			t.Fatalf("%s: %v", pvc.Name, err)
		}
		secret, ok := volumeKeySecret(t, real, pvc.Name)
		if !ok {
			t.Fatalf("%s has no passphrase Secret; its instance could never mount", pvc.Name)
		}
		passphrase := secret.StringData[luksPassphraseKey]
		if len(passphrase) < 40 {
			t.Fatalf("%s passphrase is %d characters, want 32 random bytes", pvc.Name, len(passphrase))
		}
		if seen[passphrase] {
			t.Fatalf("%s shares a passphrase with another volume", pvc.Name)
		}
		seen[passphrase] = true
		if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].UID != pvc.UID ||
			secret.OwnerReferences[0].Kind != "PersistentVolumeClaim" {
			t.Fatalf("%s Secret owners = %+v, want only its claim, so it is collected after the claim and never before", pvc.Name, secret.OwnerReferences)
		}
		if ref := secret.OwnerReferences[0]; ref.Controller != nil || ref.BlockOwnerDeletion != nil {
			t.Fatalf("%s Secret claims to control or block its claim: %+v", pvc.Name, ref)
		}
	}
}

// The passphrase is the only thing that unlocks the volume: a later
// reconcile, and one after the Secret lost its owner, never replaces it.
func TestVolumeKeyNeverRewritesAPassphrase(t *testing.T) {
	pvc := cnpgClaim("dpg-b", "1", testLUKSClass)
	real := fake.NewClientBuilder().WithScheme(volumeKeyScheme(t)).WithObjects(pvc).Build()
	r := &DatastoreVolumeKeyReconciler{Client: real, SecretClient: real, StorageClass: testLUKSClass}
	if err := reconcileVolumeKey(t, r, pvc); err != nil {
		t.Fatal(err)
	}
	first, _ := volumeKeySecret(t, real, pvc.Name)
	passphrase := first.StringData[luksPassphraseKey]
	first.OwnerReferences = nil
	if err := real.Update(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := reconcileVolumeKey(t, r, pvc); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := volumeKeySecret(t, real, pvc.Name)
	if after.StringData[luksPassphraseKey] != passphrase {
		t.Fatal("a later reconcile rewrote the passphrase; that locks the tenant out of their own database")
	}
	if len(after.OwnerReferences) != 1 || after.OwnerReferences[0].UID != pvc.UID {
		t.Fatalf("owners = %+v, want the claim restored", after.OwnerReferences)
	}
}

// A Secret that exists without a passphrase fails closed: the volume must
// not be formatted on a key nobody chose, and the Secret is not overwritten.
func TestVolumeKeyRefusesASecretWithoutPassphrase(t *testing.T) {
	pvc := cnpgClaim("dpg-c", "1", testLUKSClass)
	broken := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: volumeLUKSSecretName(pvc.Name), Namespace: volumeKeyNS}}
	real := fake.NewClientBuilder().WithScheme(volumeKeyScheme(t)).WithObjects(pvc, broken).Build()
	r := &DatastoreVolumeKeyReconciler{Client: real, SecretClient: real, StorageClass: testLUKSClass}
	if err := reconcileVolumeKey(t, r, pvc); err == nil {
		t.Fatal("accepted a passphrase Secret with no passphrase")
	}
	after, _ := volumeKeySecret(t, real, pvc.Name)
	if len(after.Data) != 0 || len(after.StringData) != 0 || len(after.OwnerReferences) != 0 {
		t.Fatalf("the damaged Secret was changed: %+v", after)
	}
}

// Only datastore claims on the datastore class get a Secret: a live volume
// left on the plain class, the platform's own CNPG clusters, a service disk
// and a claim being deleted are not this reconciler's.
func TestVolumeKeyLeavesOtherClaimsAlone(t *testing.T) {
	plain := cnpgClaim("dpg-d", "1", defaultDatastoreStorageClass)
	platform := cnpgClaim("bex-postgres", "1", testLUKSClass)
	delete(platform.Labels, appv1alpha1.LabelComponent)
	disk := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: diskPVCName("srv-x"), Namespace: volumeKeyNS, Labels: map[string]string{labelApp: "srv-x"}},
		Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: new(testLUKSClass)},
	}
	noClass := cnpgClaim("dpg-e", "1", "")
	noClass.Spec.StorageClassName = nil
	deleting := cnpgClaim("dpg-f", "1", testLUKSClass)
	deleting.Finalizers = []string{"kubernetes.io/pvc-protection"}
	deleting.DeletionTimestamp = new(metav1.Now())
	claims := []*corev1.PersistentVolumeClaim{plain, platform, disk, noClass, deleting}
	builder := fake.NewClientBuilder().WithScheme(volumeKeyScheme(t))
	for _, pvc := range claims {
		builder = builder.WithObjects(pvc)
	}
	real := builder.Build()
	r := &DatastoreVolumeKeyReconciler{Client: real, SecretClient: real, StorageClass: testLUKSClass}
	for _, pvc := range claims {
		if err := reconcileVolumeKey(t, r, pvc); err != nil {
			t.Fatalf("%s: %v", pvc.Name, err)
		}
		if _, ok := volumeKeySecret(t, real, pvc.Name); ok {
			t.Fatalf("%s got a datastore passphrase Secret", pvc.Name)
		}
	}
}

// With BEX_DATASTORE_STORAGE_CLASS unset (dev-N, the CAPD mock), claims on
// the default class are the datastore's, so the path runs there too.
func TestVolumeKeyDefaultClass(t *testing.T) {
	pvc := keyValueClaim("red-a", defaultDatastoreStorageClass)
	real := fake.NewClientBuilder().WithScheme(volumeKeyScheme(t)).WithObjects(pvc).Build()
	r := &DatastoreVolumeKeyReconciler{Client: real, SecretClient: real}
	if err := reconcileVolumeKey(t, r, pvc); err != nil {
		t.Fatal(err)
	}
	if _, ok := volumeKeySecret(t, real, pvc.Name); !ok {
		t.Fatal("a Key Value claim on the default datastore class got no passphrase")
	}
}

// A Key Value's Secret is minted before its claim, owned by the KeyValue.
// Once the claim exists it becomes an owner too, so the Secret outlives the
// claim even when the KeyValue is deleted first, and its passphrase stays.
func TestVolumeKeyAddsTheClaimToAPreMintedKeyValueSecret(t *testing.T) {
	scheme := volumeKeyScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-b", Namespace: volumeKeyNS, UID: "uid-kv"}}
	real := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).Build()
	kr := &KeyValueReconciler{Client: real, SecretClient: real, Scheme: scheme}
	existing := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "red-old", Namespace: volumeKeyNS, CreationTimestamp: metav1.Now()}}
	if err := kr.ensureKeyValueVolumeKey(context.Background(), kv, existing); err != nil {
		t.Fatal(err)
	}
	if _, ok := volumeKeySecret(t, real, keyValuePVCName(kv.Name)); ok {
		t.Fatal("minted a passphrase for an existing StatefulSet, whose claim template keeps its class")
	}
	stub := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
	if err := kr.ensureKeyValueVolumeKey(context.Background(), kv, stub); err != nil {
		t.Fatal(err)
	}
	minted, ok := volumeKeySecret(t, real, keyValuePVCName(kv.Name))
	if !ok {
		t.Fatal("no passphrase before the StatefulSet exists; its pod would mount before the Secret")
	}
	if !metav1.IsControlledBy(minted, kv) {
		t.Fatalf("pre-minted Secret owners = %+v, want the KeyValue", minted.OwnerReferences)
	}
	passphrase := minted.StringData[luksPassphraseKey]

	pvc := keyValueClaim(kv.Name, testLUKSClass)
	if err := real.Create(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	r := &DatastoreVolumeKeyReconciler{Client: real, SecretClient: real, StorageClass: testLUKSClass}
	if err := reconcileVolumeKey(t, r, pvc); err != nil {
		t.Fatal(err)
	}
	after, _ := volumeKeySecret(t, real, pvc.Name)
	if after.StringData[luksPassphraseKey] != passphrase {
		t.Fatal("the claim's reconcile replaced the Key Value's passphrase")
	}
	uids := map[types.UID]bool{}
	for _, ref := range after.OwnerReferences {
		uids[ref.UID] = true
	}
	if len(after.OwnerReferences) != 2 || !uids[kv.UID] || !uids[pvc.UID] {
		t.Fatalf("owners = %+v, want the KeyValue and the claim", after.OwnerReferences)
	}
}

// Every Cluster a Database projects names the datastore class: a new
// database, an HA pair, a restore to a new database, a point-in-time
// recovery, and the read replicas, which copy the primary's storage. Unset,
// the class is hcloud-volumes, as before.
func TestEveryClusterSpecNamesTheDatastoreClass(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "basic-4gb"})
	base := clusterParams{name: "dpg-g", plan: plan, storageGB: gb, dbname: "d", owner: "d_user", namespace: volumeKeyNS, storageClass: testLUKSClass}
	ha, restore, pitr, unset := base, base, base, base
	ha.highAvailability = true
	restore.store, restore.recovery = &testStore, &appv1alpha1.DatabaseRecovery{SourceDatabase: "dpg-src"}
	pitr.store, pitr.recovery = &testStore, &appv1alpha1.DatabaseRecovery{SourceDatabase: "dpg-src", TargetTime: "2026-11-27T10:00:00Z"}
	unset.storageClass = ""
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-g", Namespace: volumeKeyNS}}
	for _, tc := range []struct {
		name     string
		params   clusterParams
		recovers bool
		want     string
	}{
		{"new", base, false, testLUKSClass},
		{"high availability", ha, false, testLUKSClass},
		{"restore to a new database", restore, true, testLUKSClass},
		{"point-in-time recovery", pitr, true, testLUKSClass},
		{"class unset", unset, false, defaultDatastoreStorageClass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := cnpgClusterSpec(tc.params)
			if _, recovers := spec["bootstrap"].(map[string]any)["recovery"]; recovers != tc.recovers {
				t.Fatalf("bootstrap = %v, want recovery %t", spec["bootstrap"], tc.recovers)
			}
			if got := spec["storage"].(map[string]any)["storageClass"]; got != tc.want {
				t.Fatalf("primary Cluster storageClass = %v, want %s", got, tc.want)
			}
			replica := readReplicaClusterSpec(db, "r1", spec, nil)
			if got := replica["storage"].(map[string]any)["storageClass"]; got != tc.want {
				t.Fatalf("read replica storageClass = %v, want %s", got, tc.want)
			}
		})
	}
}
