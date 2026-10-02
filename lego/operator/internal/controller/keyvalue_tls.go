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
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	kvTLSFinalizer     = "app.bex.co/kv-tls-cleanup"
	annotKVTLSIdentity = "app.bex.co/kv-tls-identity"
)

type keyValueTLSIdentity struct {
	Host           string `json:"host"`
	Issuer         string `json:"issuer"`
	CertificateUID string `json:"certificateUID,omitempty"`
}

func keyValueTLSName(kv *appv1alpha1.KeyValue) string { return kv.Name + "-kv-tls" }

// Every managed lifetime reserves cleanup, including a legacy store whose
// public flag was already withdrawn. This never grants a backup purge duty.
func (r *KeyValueReconciler) ensureKeyValueTLSFinalizer(ctx context.Context, kv *appv1alpha1.KeyValue) error {
	changed := controllerutil.AddFinalizer(kv, kvTLSFinalizer)
	if kv.Spec.Public && r.KvDomain != "" && r.ClusterIssuer != "" {
		identity, err := r.keyValueTLSIdentity(kv)
		if err != nil {
			return err
		}
		identity.Host, identity.Issuer = kv.Name+"."+r.KvDomain, r.ClusterIssuer
		raw, _ := json.Marshal(identity)
		if kv.Annotations[annotKVTLSIdentity] != string(raw) {
			if kv.Annotations == nil {
				kv.Annotations = map[string]string{}
			}
			kv.Annotations[annotKVTLSIdentity] = string(raw)
			changed = true
		}
	}
	if changed {
		return r.Update(ctx, kv)
	}
	return nil
}

func (r *KeyValueReconciler) keyValueTLSIdentity(kv *appv1alpha1.KeyValue) (keyValueTLSIdentity, error) {
	identity := keyValueTLSIdentity{}
	if raw := kv.Annotations[annotKVTLSIdentity]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &identity); err != nil {
			return identity, fmt.Errorf("invalid KeyValue TLS cleanup identity")
		}
	} else if r.KvDomain != "" && r.ClusterIssuer != "" {
		identity.Host = kv.Name + "." + r.KvDomain
		identity.Issuer = r.ClusterIssuer
	}
	return identity, nil
}

// Cleanup observes the producer absent before touching its output, then
// observes that exact output absent before releasing the lifetime obligation.
func (r *KeyValueReconciler) reconcileKeyValueTLSCleanup(ctx context.Context, kv *appv1alpha1.KeyValue) (bool, error) {
	if !canonicalNamespace(&kv.ObjectMeta) || kv.UID == "" {
		return false, fmt.Errorf("refusing KeyValue TLS cleanup without canonical namespace and lifetime UID")
	}
	terminating, err := namespaceDeletionOwnsLocalCleanup(ctx, r.secretClient(), kv.Namespace)
	if err != nil {
		return false, err
	}
	if terminating {
		return true, nil
	}
	identity, err := r.keyValueTLSIdentity(kv)
	if err != nil {
		return false, err
	}
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	key := client.ObjectKey{Namespace: kv.Namespace, Name: keyValueTLSName(kv)}
	reader := cmp.Or(r.APIReader, client.Reader(r.Client))
	if err := reader.Get(ctx, key, cert); err == nil {
		if changed, err := r.rememberKeyValueTLSCertificate(ctx, kv, cert); err != nil || changed {
			return false, err
		}
		return false, deleteKeyValueTLSObject(ctx, r.Client, cert)
	} else if !apierrors.IsNotFound(err) {
		return false, err
	}
	secret := &corev1.Secret{}
	if err := r.secretClient().Get(ctx, key, secret); apierrors.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}
	if err := validateKeyValueTLSSecret(kv, secret, identity); err != nil {
		return false, err
	}
	return false, deleteKeyValueTLSObject(ctx, r.secretClient(), secret)
}

func (r *KeyValueReconciler) rememberKeyValueTLSCertificate(ctx context.Context, kv *appv1alpha1.KeyValue, cert *unstructured.Unstructured) (bool, error) {
	identity, err := r.keyValueTLSIdentity(kv)
	if err != nil {
		return false, err
	}
	if !metav1.IsControlledBy(cert, kv) {
		return false, fmt.Errorf("refusing foreign KeyValue TLS Certificate")
	}
	secretName, _, _ := unstructured.NestedString(cert.Object, "spec", "secretName")
	if secretName != keyValueTLSName(kv) || cert.GetUID() == "" {
		return false, fmt.Errorf("refusing ambiguous KeyValue TLS Certificate")
	}
	hosts, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames")
	issuer, _, _ := unstructured.NestedString(cert.Object, "spec", "issuerRef", "name")
	kind, _, _ := unstructured.NestedString(cert.Object, "spec", "issuerRef", "kind")
	group, _, _ := unstructured.NestedString(cert.Object, "spec", "issuerRef", "group")
	if len(hosts) != 1 || !strings.HasPrefix(hosts[0], kv.Name+".") || issuer == "" || kind != "ClusterIssuer" || (group != "" && group != "cert-manager.io") {
		return false, fmt.Errorf("refusing ambiguous KeyValue TLS Certificate issuance identity")
	}
	// The owned producer records its actual historical domain/issuer even
	// if operator configuration changed before this legacy deletion.
	observed := keyValueTLSIdentity{Host: hosts[0], Issuer: issuer, CertificateUID: string(cert.GetUID())}
	if identity != observed {
		identity = observed
		raw, _ := json.Marshal(identity)
		if kv.Annotations == nil {
			kv.Annotations = map[string]string{}
		}
		kv.Annotations[annotKVTLSIdentity] = string(raw)
		if err := r.Update(ctx, kv); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func deleteKeyValueTLSObject(ctx context.Context, cl client.Client, obj client.Object) error {
	if !obj.GetDeletionTimestamp().IsZero() {
		return nil
	}
	uid, rv := obj.GetUID(), obj.GetResourceVersion()
	return client.IgnoreNotFound(cl.Delete(ctx, obj, client.Preconditions{UID: &uid, ResourceVersion: &rv}))
}

func validateKeyValueTLSSecret(kv *appv1alpha1.KeyValue, secret *corev1.Secret, identity keyValueTLSIdentity) error {
	refuse := func() error {
		return fmt.Errorf("refusing foreign or unproven KeyValue TLS Secret; establish its lifetime and issuance provenance before cleanup")
	}
	if secret.Name != keyValueTLSName(kv) || secret.Namespace != kv.Namespace || secret.Type != corev1.SecretTypeTLS {
		return refuse()
	}
	for _, owner := range secret.OwnerReferences {
		ownKV := owner.APIVersion == appv1alpha1.SchemeGroupVersion.String() && owner.Kind == "KeyValue" && owner.Name == kv.Name && owner.UID == kv.UID
		ownCert := owner.APIVersion == "cert-manager.io/v1" && owner.Kind == "Certificate" && owner.Name == secret.Name && string(owner.UID) == identity.CertificateUID && identity.CertificateUID != ""
		if !ownKV && !ownCert {
			return refuse()
		}
	}
	if uid, exists := secret.Labels[execution.LabelKeyValueUID]; exists {
		if uid != string(kv.UID) || uid == "" {
			return refuse()
		}
		return nil
	}
	// Legacy cert-manager outputs have no ownerReference by default. Exact
	// issuance metadata plus creation within this KeyValue lifetime is required;
	// a name alone cannot authorize deleting a previous lifetime's private key.
	annotations := secret.Annotations
	if identity.Host == "" || identity.Issuer == "" || kv.CreationTimestamp.IsZero() || secret.CreationTimestamp.IsZero() || secret.CreationTimestamp.Before(&kv.CreationTimestamp) {
		return refuse()
	}
	if annotations["cert-manager.io/certificate-name"] != secret.Name || annotations["cert-manager.io/alt-names"] != identity.Host || annotations["cert-manager.io/issuer-name"] != identity.Issuer || annotations["cert-manager.io/issuer-kind"] != "ClusterIssuer" {
		return refuse()
	}
	if group := annotations["cert-manager.io/issuer-group"]; group != "" && group != "cert-manager.io" {
		return refuse()
	}
	return nil
}
