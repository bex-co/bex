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
	"cmp"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Encrypted volumes for managed Postgres and Key Value (w4/m714, ADR082 D3).
//
// A volume on an encrypted class (deploy/gitops/base/disk-storageclass.yaml)
// is unlocked by the passphrase in Secret "<pvc>-luks", key
// "encryption-passphrase", in the claim's own namespace: the class templates
// its node-publish secret as ${pvc.name}-luks, and the kubelet reads it under
// the node authorizer when it mounts the volume. Service disks, Postgres and
// Key Value all mint that Secret through ensureLUKSPassphrase, one random
// passphrase per volume.
const (
	// defaultDatastoreStorageClass is the class Postgres and Key Value volumes
	// claim when BEX_DATASTORE_STORAGE_CLASS is unset: the hcloud CSI chart's
	// plain class, which scripts/dev-env aliases onto local-path for dev-N.
	// Production names the encrypted class (config/prod/kustomization.yaml).
	defaultDatastoreStorageClass = "hcloud-volumes"
	// luksPassphraseKey is the data key the hcloud CSI driver reads a volume's
	// encryption passphrase from.
	luksPassphraseKey = "encryption-passphrase"
	// luksPassphraseBytes is the entropy behind a volume's LUKS passphrase.
	luksPassphraseBytes = 32
)

// datastoreStorageClass is the class a Postgres or Key Value volume claims.
func datastoreStorageClass(configured string) string {
	return cmp.Or(configured, defaultDatastoreStorageClass)
}

// volumeLUKSSecretName is the passphrase Secret the encrypted class's
// ${pvc.name}-luks template resolves to for a claim. scripts/gitops-validate.sh
// pins this derivation against the class.
func volumeLUKSSecretName(claim string) string {
	return claim + appv1alpha1.DiskLUKSSecretSuffix
}

// ensureLUKSPassphrase returns a volume's passphrase Secret, minting it if it
// does not exist yet. A Secret that exists is never rewritten: its passphrase
// is the only thing that can unlock the volume, so replacing it would lock the
// tenant out of their own data. One that exists without a passphrase fails
// closed instead of being repaired, so a volume never mounts on a key nobody
// chose. own sets a new Secret's owner references.
//
// secrets must be an uncached client: the Secret lives in the tenant's own
// namespace, which the manager's single-namespace Secret informer does not
// cover (NamespacedSecretCacheOptions).
func ensureLUKSPassphrase(
	ctx context.Context,
	secrets client.Client,
	key client.ObjectKey,
	labels map[string]string,
	own func(*corev1.Secret) error,
) (*corev1.Secret, error) {
	secret, err := liveLUKSPassphrase(ctx, secrets, key)
	if !apierrors.IsNotFound(err) {
		return secret, err
	}
	passphrase := make([]byte, luksPassphraseBytes)
	if _, err := rand.Read(passphrase); err != nil {
		return nil, fmt.Errorf("generate volume passphrase: %w", err)
	}
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Labels: labels},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{luksPassphraseKey: base64.RawStdEncoding.EncodeToString(passphrase)},
	}
	if err := own(secret); err != nil {
		return nil, err
	}
	switch err := secrets.Create(ctx, secret); {
	case apierrors.IsAlreadyExists(err):
		// Minted concurrently: keep the winner's passphrase.
		return liveLUKSPassphrase(ctx, secrets, key)
	case err != nil:
		return nil, err
	}
	return secret, nil
}

// liveLUKSPassphrase reads a volume's passphrase Secret, refusing one that
// carries no passphrase.
func liveLUKSPassphrase(ctx context.Context, secrets client.Client, key client.ObjectKey) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := secrets.Get(ctx, key, secret); err != nil {
		return nil, err
	}
	if len(secret.Data[luksPassphraseKey]) == 0 && secret.StringData[luksPassphraseKey] == "" {
		return nil, fmt.Errorf("volume passphrase Secret %s is missing its encryption key; refusing to replace or use it", key)
	}
	return secret, nil
}

// claimOwnerReference makes a passphrase Secret a dependent of its claim, so
// the garbage collector deletes it only once the claim itself is gone (a claim
// in use is held by kubernetes.io/pvc-protection until its last pod ends).
// It neither controls nor blocks: the claim belongs to CNPG or the
// StatefulSet, and blocking would need the claims' finalizers permission.
func claimOwnerReference(pvc *corev1.PersistentVolumeClaim) metav1.OwnerReference {
	return metav1.OwnerReference{APIVersion: "v1", Kind: "PersistentVolumeClaim", Name: pvc.Name, UID: pvc.UID}
}

// DatastoreVolumeKeyReconciler mints the passphrase of every Postgres and Key
// Value claim on the datastore class (w4/m714). It is keyed on the claim
// because CloudNativePG names an instance's claim "<cluster>-<serial>" and
// the serial grows when it adds a standby or replaces an instance, so no
// reconcile of the Database can know every name in advance. The kubelet
// retries a mount whose Secret is missing, and the Secret is written as soon
// as the claim exists, well before a new volume is provisioned and attached.
type DatastoreVolumeKeyReconciler struct {
	client.Client
	// SecretClient reads and writes the passphrase Secrets. It must be
	// uncached (see ensureLUKSPassphrase). Nil falls back to the cached client.
	SecretClient client.Client
	// StorageClass is BEX_DATASTORE_STORAGE_CLASS (datastoreStorageClass).
	StorageClass string
}

// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// Secrets access is namespace-scoped via the tenant namespace RoleBinding (bex-tenant-operator).

func (r *DatastoreVolumeKeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, req.NamespacedName, pvc); err != nil {
		// A deleted claim's Secret follows it through its owner reference.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !r.encrypts(pvc) || !pvc.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	secrets := cmp.Or(r.SecretClient, r.Client)
	owner := claimOwnerReference(pvc)
	key := client.ObjectKey{Namespace: pvc.Namespace, Name: volumeLUKSSecretName(pvc.Name)}
	secret, err := ensureLUKSPassphrase(ctx, secrets, key, nil, func(s *corev1.Secret) error {
		s.OwnerReferences = []metav1.OwnerReference{owner}
		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	// A Key Value's Secret is minted before its claim exists, owned by the
	// KeyValue alone (ensureKeyValueVolumeKey). Adding the claim keeps the
	// Secret until both are gone.
	if slices.ContainsFunc(secret.OwnerReferences, func(ref metav1.OwnerReference) bool { return ref.UID == pvc.UID }) {
		return ctrl.Result{}, nil
	}
	before := secret.DeepCopy()
	secret.OwnerReferences = append(secret.OwnerReferences, owner)
	return ctrl.Result{}, secrets.Patch(ctx, secret, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}

// encrypts reports whether a claim is a Postgres or Key Value volume on the
// datastore class. Postgres claims carry the component label the Database
// controller writes into its Clusters' inheritedMetadata, which the
// platform's own CNPG clusters never set; Key Value claims carry the
// StatefulSet claim template's KeyValue label.
func (r *DatastoreVolumeKeyReconciler) encrypts(obj client.Object) bool {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok || pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != datastoreStorageClass(r.StorageClass) {
		return false
	}
	labels := pvc.GetLabels()
	postgres := labels[cnpgClusterLabel] != "" && labels[appv1alpha1.LabelComponent] == componentDatabase
	return postgres || labels[labelKeyValue] != ""
}

// SetupWithManager registers the reconciler on datastore claims only.
func (r *DatastoreVolumeKeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}, builder.WithPredicates(predicate.NewPredicateFuncs(r.encrypts))).
		Named("datastorevolumekey").
		Complete(r)
}
