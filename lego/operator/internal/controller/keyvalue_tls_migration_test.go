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
	"encoding/json"
	"errors"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func keyValueTLSMigrationFixture(t *testing.T, mode string) (*KeyValueReconciler, *appv1alpha1.KeyValue, *unstructured.Unstructured, *corev1.Secret) {
	t.Helper()
	scheme := keyValueBackupTestScheme(t)
	created := metav1.NewTime(time.Now().Add(-time.Hour))
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-migrate", Namespace: defaultAppsNamespace, UID: "kv-current", CreationTimestamp: created}, Spec: appv1alpha1.KeyValueSpec{Plan: "free", Public: true}}
	cert := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"secretName": kv.Name + "-kv-tls", "dnsNames": []any{kv.Name + ".old.example.test"},
		"issuerRef": map[string]any{"name": "old-issuer", "kind": "ClusterIssuer"},
	}}}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	cert.SetName(kv.Name + "-kv-tls")
	cert.SetNamespace(kv.Namespace)
	cert.SetUID("certificate-current")
	cert.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(kv, appv1alpha1.SchemeGroupVersion.WithKind("KeyValue"))})
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: cert.GetName(), Namespace: kv.Namespace, UID: "legacy-secret", CreationTimestamp: metav1.NewTime(created.Add(time.Minute)), Annotations: map[string]string{
		"cert-manager.io/certificate-name": cert.GetName(), "cert-manager.io/alt-names": kv.Name + ".old.example.test",
		"cert-manager.io/issuer-name": "old-issuer", "cert-manager.io/issuer-kind": "ClusterIssuer",
	}}, Type: corev1.SecretTypeTLS, Data: map[string][]byte{"tls.crt": []byte("existing-certificate"), "tls.key": []byte("existing-private-key")}}
	objects := []client.Object{kv, cert}
	switch mode {
	case "certificate-owner":
		secret.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(cert, certManagerCertificateGVK)}
	case "producer-already-absent":
		identity, err := json.Marshal(keyValueTLSIdentity{Host: kv.Name + ".old.example.test", Issuer: "old-issuer", CertificateUID: string(cert.GetUID())})
		if err != nil {
			t.Fatal(err)
		}
		kv.Annotations = map[string]string{annotKVTLSIdentity: string(identity)}
		objects = objects[:1]
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(kv).WithObjects(objects...).Build()
	secrets := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()
	return &KeyValueReconciler{Client: cl, APIReader: cl, SecretClient: secrets, Scheme: scheme, KvDomain: "new.example.test", ClusterIssuer: "new-issuer", Backup: testKeyValueBackupStore}, kv, cert, secret
}

func TestKeyValueTLSMigratesLegacyIssuanceBeforeConfigChange(t *testing.T) {
	for _, mode := range []string{"ownerless", "certificate-owner", "producer-already-absent"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			r, kv, cert, secret := keyValueTLSMigrationFixture(t, mode)
			req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(kv)}
			if _, err := r.Reconcile(ctx, req); err != nil {
				t.Fatalf("legacy public migration after domain/issuer change: %v", err)
			}
			if err := r.Get(ctx, client.ObjectKeyFromObject(cert), cert); err != nil {
				t.Fatal(err)
			}
			// Fake creation does not assign a UID; subsequent deletion requires
			// the producer identity a real API server supplies automatically.
			if cert.GetUID() == "" {
				cert.SetUID("replacement-certificate")
				if err := r.Update(ctx, cert); err != nil {
					t.Fatal(err)
				}
			}
			hosts, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames")
			issuer, _, _ := unstructured.NestedString(cert.Object, "spec", "issuerRef", "name")
			if len(hosts) != 1 || hosts[0] != kv.Name+".new.example.test" || issuer != "new-issuer" {
				t.Fatalf("Certificate did not converge to new domain/issuer: %v, %q", hosts, issuer)
			}
			currentSecret := &corev1.Secret{}
			if err := r.secretClient().Get(ctx, client.ObjectKeyFromObject(secret), currentSecret); err != nil {
				t.Fatal(err)
			}
			if currentSecret.UID != secret.UID || currentSecret.Labels[execution.LabelKeyValueUID] != string(kv.UID) || string(currentSecret.Data["tls.key"]) != string(secret.Data["tls.key"]) {
				t.Fatal("legacy identity was not bound before changing Certificate issuance; key material must remain unchanged")
			}
			// Withdraw public access before cert-manager has updated the Secret.
			// Cleanup must still recognize the old issuance after a restart.
			if err := r.Get(ctx, req.NamespacedName, kv); err != nil {
				t.Fatal(err)
			}
			kv.Spec.Public = false
			if err := r.Update(ctx, kv); err != nil {
				t.Fatal(err)
			}
			r.KvDomain, r.ClusterIssuer = "", ""
			if _, err := r.Reconcile(ctx, req); err != nil {
				t.Fatalf("private transition: %v", err)
			}
			if err := r.Get(ctx, req.NamespacedName, kv); err != nil {
				t.Fatal(err)
			}
			if !controllerutil.ContainsFinalizer(kv, kvTLSFinalizer) || controllerutil.ContainsFinalizer(kv, kvFinalizer) {
				t.Fatal("private Free lifetime lost TLS cleanup or acquired backup purge duty")
			}
			if err := r.secretClient().Get(ctx, client.ObjectKeyFromObject(secret), currentSecret); err != nil {
				t.Fatalf("private transition changed existing live-Secret retention: %v", err)
			}
			if err := r.Delete(ctx, kv); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				restarted := &KeyValueReconciler{Client: r.Client, APIReader: r.APIReader, SecretClient: r.SecretClient, Scheme: r.Scheme, Backup: r.Backup}
				if _, err := restarted.Reconcile(ctx, req); err != nil {
					t.Fatalf("historical public cleanup: %v", err)
				}
			}
			if err := r.secretClient().Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{}); !apierrors.IsNotFound(err) {
				t.Fatalf("historical issued TLS Secret survived: %v", err)
			}
			if err := r.Get(ctx, req.NamespacedName, &appv1alpha1.KeyValue{}); !apierrors.IsNotFound(err) {
				t.Fatalf("TLS cleanup finalizer survived observed absence: %v", err)
			}
			jobs := &batchv1.JobList{}
			if err := r.List(ctx, jobs); err != nil || len(jobs.Items) != 0 {
				t.Fatalf("Free TLS migration/deletion started backup jobs: %v, count=%d", err, len(jobs.Items))
			}
		})
	}
}

func TestKeyValueTLSMigrationRetainsProducerUntilSecretBindingCommits(t *testing.T) {
	ctx := context.Background()
	r, kv, cert, secret := keyValueTLSMigrationFixture(t, "ownerless")
	denied := errors.New("Secret provenance update unavailable")
	r.SecretClient = interceptor.NewClient(r.SecretClient.(client.WithWatch), interceptor.Funcs{
		Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
			return denied
		},
	})
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(kv)}); !errors.Is(err, denied) {
		t.Fatalf("migration error = %v, want retriable Secret binding failure", err)
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(cert), cert); err != nil {
		t.Fatal(err)
	}
	hosts, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames")
	if len(hosts) != 1 || hosts[0] != kv.Name+".old.example.test" {
		t.Fatalf("Certificate changed before Secret provenance committed: %v", hosts)
	}
	if err := r.secretClient().Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		t.Fatal(err)
	}
	if secret.Labels[execution.LabelKeyValueUID] != "" {
		t.Fatal("failed migration rewrote Secret provenance")
	}
}

func TestKeyValueTLSMalformedHistoryPreventsIssuance(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: "red-malformed", Namespace: defaultAppsNamespace, UID: "kv-current",
		Annotations: map[string]string{annotKVTLSIdentity: "{"},
	}, Spec: appv1alpha1.KeyValueSpec{Public: true}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).WithStatusSubresource(kv).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, KvDomain: "kv.example.test", ClusterIssuer: "issuer"}
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(kv)}); err == nil {
		t.Fatal("malformed cleanup history must refuse new issuance")
	}
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	if err := cl.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-kv-tls"}, cert); !apierrors.IsNotFound(err) {
		t.Fatalf("Certificate was published with unreadable cleanup history: %v", err)
	}
}
