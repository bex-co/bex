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
	"crypto/x509"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The real cert-manager and CNPG CRD schemas prune unknown fields silently, so
// this proves the projected Certificate (secretTemplate, issuerRef) and the
// Cluster's serverTLSSecret/serverCASecret survive admission as written.
var _ = Describe("Database publicly trusted certificate", func() {
	It("issues through cert-manager and wires the issued Secret into CNPG", func(ctx SpecContext) {
		name := "dpg-public-tls"
		names := []string{name + ".db.example"}
		root, tlsCrt := issuedChain(GinkgoTB(), names, publicTLSNow.Add(90*24*time.Hour))
		roots := x509.NewCertPool()
		roots.AddCert(root)

		db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Finalizers: []string{dbFinalizer}},
			Spec: appv1alpha1.DatabaseSpec{Name: name, Plan: "basic-1gb", Public: true}}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		key := client.ObjectKeyFromObject(db)
		cluster := &unstructured.Unstructured{}
		cluster.SetGroupVersionKind(cnpgClusterGVK)
		cert := &unstructured.Unstructured{}
		cert.SetGroupVersionKind(certManagerCertificateGVK)
		certKey := client.ObjectKey{Namespace: "default", Name: name + dbPublicTLSSuffix}
		DeferCleanup(func(ctx SpecContext) {
			_ = k8sClient.Delete(ctx, cluster)
			_ = k8sClient.Delete(ctx, cert)
			for _, n := range []string{name + dbPublicTLSSuffix, name + dbPublicCASuffix} {
				_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: n, Namespace: "default"}})
			}
			if err := k8sClient.Get(ctx, key, db); err == nil {
				db.Finalizers = nil
				Expect(k8sClient.Update(ctx, db)).To(Succeed())
				Expect(k8sClient.Delete(ctx, db)).To(Succeed())
			}
		})
		r := &DatabaseReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), DBDomain: "db.example",
			PublicCertIssuer: "letsencrypt-prod", PublicCertRoots: roots,
			Now: func() time.Time { return publicTLSNow },
		}
		run := func() {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, key, db)).To(Succeed())
			Expect(k8sClient.Get(ctx, key, cluster)).To(Succeed())
		}

		run()
		Expect(k8sClient.Get(ctx, certKey, cert)).To(Succeed())
		labels, _, _ := unstructured.NestedStringMap(cert.Object, "spec", "secretTemplate", "labels")
		Expect(labels).To(HaveKeyWithValue("cnpg.io/reload", ""))
		Expect(labels).To(HaveKeyWithValue(labelDatabaseUID, string(db.UID)))
		dnsNames, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames")
		Expect(dnsNames).To(Equal(names))
		issuer, _, _ := unstructured.NestedString(cert.Object, "spec", "issuerRef", "name")
		Expect(issuer).To(Equal("letsencrypt-prod"))
		Expect(clusterCertificates(cluster)).To(HaveKey("serverAltDNSNames"))
		Expect(db.Status.ExternalCertificate).To(Equal(appv1alpha1.DBExternalCertificatePending))

		// cert-manager issues.
		Expect(unstructured.SetNestedSlice(cert.Object, []any{map[string]any{
			"type": "Ready", "status": "True", "reason": "Ready", "message": "issued",
			"observedGeneration": cert.GetGeneration(),
		}}, "status", "conditions")).To(Succeed())
		Expect(k8sClient.Status().Update(ctx, cert)).To(Succeed())
		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: certKey.Name, Namespace: "default", Labels: labels},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{corev1.TLSCertKey: tlsCrt, corev1.TLSPrivateKeyKey: []byte("key")},
		})).To(Succeed())
		run()
		Expect(clusterCertificates(cluster)).To(Equal(map[string]any{
			"serverTLSSecret": name + dbPublicTLSSuffix, "serverCASecret": name + dbPublicCASuffix,
		}))

		// CNPG serves it.
		Expect(unstructured.SetNestedField(cluster.Object, name+dbPublicTLSSuffix, "status", "certificates", "serverTLSSecret")).To(Succeed())
		Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
		run()
		Expect(db.Status.ExternalCertificate).To(Equal(appv1alpha1.DBExternalCertificatePubliclyTrusted))
	})
})
