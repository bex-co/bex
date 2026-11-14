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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateKeyValueTLSSecret(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: "cache", Namespace: "ws", UID: "kv-uid", CreationTimestamp: created,
	}}
	identity := keyValueTLSIdentity{Host: "cache.kv.example", Issuer: "letsencrypt", CertificateUID: "cert-uid"}

	// validLegacy is the ownerless cert-manager output the legacy path accepts.
	validLegacy := func() *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cache-kv-tls", Namespace: "ws", CreationTimestamp: metav1.NewTime(created.Add(time.Minute)),
				Annotations: map[string]string{
					"cert-manager.io/certificate-name": "cache-kv-tls",
					"cert-manager.io/alt-names":        "cache.kv.example",
					"cert-manager.io/issuer-name":      "letsencrypt",
					"cert-manager.io/issuer-kind":      "ClusterIssuer",
				},
			},
			Type: corev1.SecretTypeTLS,
		}
	}
	kvOwner := metav1.OwnerReference{APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "KeyValue", Name: "cache", UID: "kv-uid"}
	certOwner := metav1.OwnerReference{APIVersion: "cert-manager.io/v1", Kind: "Certificate", Name: "cache-kv-tls", UID: "cert-uid"}

	for _, test := range []struct {
		name   string
		secret func() *corev1.Secret
		wantOK bool
	}{
		{name: "ownerless legacy issuance", secret: validLegacy, wantOK: true},
		{name: "this KeyValue owns it", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.OwnerReferences = []metav1.OwnerReference{kvOwner}
			return secret
		}, wantOK: true},
		{name: "the issuing Certificate owns it", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.OwnerReferences = []metav1.OwnerReference{certOwner}
			return secret
		}, wantOK: true},
		{name: "the Certificate owner must carry its UID", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.OwnerReferences = []metav1.OwnerReference{{APIVersion: "cert-manager.io/v1", Kind: "Certificate", Name: "cache-kv-tls"}}
			return secret
		}},
		{name: "a foreign owner refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Secret", Name: "other", UID: "other-uid"}}
			return secret
		}},
		{name: "the lifetime label proves it", secret: func() *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cache-kv-tls", Namespace: "ws", Labels: map[string]string{execution.LabelKeyValueUID: "kv-uid"}},
				Type:       corev1.SecretTypeTLS,
			}
		}, wantOK: true},
		{name: "a label from another lifetime refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Labels = map[string]string{execution.LabelKeyValueUID: "old-kv"}
			return secret
		}},
		{name: "an empty lifetime label refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Labels = map[string]string{execution.LabelKeyValueUID: ""}
			return secret
		}},
		{name: "another Secret name refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Name = "other"
			return secret
		}},
		{name: "another namespace refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Namespace = "other"
			return secret
		}},
		{name: "another Secret type refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Type = corev1.SecretTypeOpaque
			return secret
		}},
		{name: "a Secret from before this lifetime refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.CreationTimestamp = metav1.NewTime(created.Add(-time.Minute))
			return secret
		}},
		{name: "a Secret with no creation time refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.CreationTimestamp = metav1.Time{}
			return secret
		}},
		{name: "a mismatched certificate name refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Annotations["cert-manager.io/certificate-name"] = "other"
			return secret
		}},
		{name: "a mismatched host refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Annotations["cert-manager.io/alt-names"] = "other.kv.example"
			return secret
		}},
		{name: "a mismatched issuer refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Annotations["cert-manager.io/issuer-name"] = "other"
			return secret
		}},
		{name: "a non-ClusterIssuer refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Annotations["cert-manager.io/issuer-kind"] = "Issuer"
			return secret
		}},
		{name: "an empty issuer group is cert-manager's default", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Annotations["cert-manager.io/issuer-group"] = ""
			return secret
		}, wantOK: true},
		{name: "the cert-manager issuer group passes", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Annotations["cert-manager.io/issuer-group"] = "cert-manager.io"
			return secret
		}, wantOK: true},
		{name: "another issuer group refuses", secret: func() *corev1.Secret {
			secret := validLegacy()
			secret.Annotations["cert-manager.io/issuer-group"] = "other.example"
			return secret
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateKeyValueTLSSecret(kv, test.secret(), identity)
			if test.wantOK && err != nil {
				t.Fatalf("want the Secret proven, got %v", err)
			}
			if !test.wantOK && err == nil {
				t.Fatal("want the Secret refused")
			}
		})
	}
}

func TestValidateKeyValueTLSSecretNeedsIssuanceIdentity(t *testing.T) {
	created := metav1.NewTime(time.Now().Add(-time.Hour))
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: "cache", Namespace: "ws", UID: "kv-uid", CreationTimestamp: created,
	}}
	// issuedSecret carries exactly the cert-manager annotations identity
	// implies: an empty Host or Issuer leaves that annotation absent, which
	// reads back as "" and so matches the empty field. Only the empty-identity
	// guard can refuse such a pair; the annotation comparison cannot.
	issuedSecret := func(identity keyValueTLSIdentity) *corev1.Secret {
		annotations := map[string]string{
			"cert-manager.io/certificate-name": "cache-kv-tls",
			"cert-manager.io/issuer-kind":      "ClusterIssuer",
		}
		if identity.Host != "" {
			annotations["cert-manager.io/alt-names"] = identity.Host
		}
		if identity.Issuer != "" {
			annotations["cert-manager.io/issuer-name"] = identity.Issuer
		}
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cache-kv-tls", Namespace: "ws", Annotations: annotations,
				CreationTimestamp: metav1.NewTime(created.Add(time.Minute)),
			},
			Type: corev1.SecretTypeTLS,
		}
	}
	complete := keyValueTLSIdentity{Host: "cache.kv.example", Issuer: "letsencrypt"}
	if err := validateKeyValueTLSSecret(kv, issuedSecret(complete), complete); err != nil {
		t.Fatalf("control: a Secret issued for a complete identity must be accepted: %v", err)
	}
	for _, identity := range []keyValueTLSIdentity{
		{Issuer: "letsencrypt"},
		{Host: "cache.kv.example"},
		{},
	} {
		if err := validateKeyValueTLSSecret(kv, issuedSecret(identity), identity); err == nil {
			t.Fatalf("want refusal for incomplete identity %+v", identity)
		}
	}
	noLifetime := kv.DeepCopy()
	noLifetime.CreationTimestamp = metav1.Time{}
	if err := validateKeyValueTLSSecret(noLifetime, issuedSecret(complete), complete); err == nil {
		t.Fatal("want refusal for a KeyValue with no lifetime start")
	}
}
