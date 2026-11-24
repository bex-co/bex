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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w21/m618 against a real API server: ingress-shim's Certificate for a host
// Let's Encrypt refused on CAA becomes the App's and names Google Trust
// Services, and with no second issuer configured nothing is written.
var _ = Describe("Certificate issuer move (w21/m618)", func() {
	const namespace = "default"

	// shimCertificate writes the Certificate ingress-shim makes for host's TLS
	// entry, then the status cert-manager records once Let's Encrypt refused
	// the order, and the Challenge that names the refusal.
	shimCertificate := func(ctx SpecContext, ing *networkingv1.Ingress, secret, host string) {
		cert := &unstructured.Unstructured{}
		cert.SetGroupVersionKind(certManagerCertificateGVK)
		cert.SetNamespace(namespace)
		cert.SetName(secret)
		cert.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(ing, networkingv1.SchemeGroupVersion.WithKind("Ingress"))})
		cert.Object["spec"] = map[string]any{
			"secretName": secret, "dnsNames": []any{host},
			"issuerRef": map[string]any{"name": "letsencrypt-prod", "kind": "ClusterIssuer", "group": "cert-manager.io"},
		}
		Expect(k8sClient.Create(ctx, cert)).To(Succeed())
		cert.Object["status"] = map[string]any{
			"failedIssuanceAttempts": int64(1),
			"lastFailureTime":        time.Now().UTC().Format(time.RFC3339),
			"conditions": []any{map[string]any{
				"type": "Issuing", "status": "False", "reason": "Failed", "observedGeneration": cert.GetGeneration(),
				"lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
				"message":            `The certificate request has failed to complete and will be retried: Failed to wait for order resource "o" to become ready: order is in "invalid" state: `,
			}},
		}
		Expect(k8sClient.Status().Update(ctx, cert)).To(Succeed())

		challenge := &unstructured.Unstructured{}
		challenge.SetGroupVersionKind(acmeChallengeGVK)
		challenge.SetNamespace(namespace)
		challenge.SetName(secret + "-challenge")
		challenge.Object["spec"] = map[string]any{
			"dnsName": host, "type": "HTTP-01",
			"issuerRef": map[string]any{"name": "letsencrypt-prod", "kind": "ClusterIssuer", "group": "cert-manager.io"},
		}
		Expect(k8sClient.Create(ctx, challenge)).To(Succeed())
		challenge.Object["status"] = map[string]any{"state": "invalid", "reason": "CAA record for " + host + " prevents issuance"}
		Expect(k8sClient.Status().Update(ctx, challenge)).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			_ = k8sClient.Delete(ctx, cert)
			_ = k8sClient.Delete(ctx, challenge)
		})
	}
	readCertificate := func(ctx SpecContext, secret string) (*unstructured.Unstructured, error) {
		cert := &unstructured.Unstructured{}
		cert.SetGroupVersionKind(certManagerCertificateGVK)
		return cert, k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secret}, cert)
	}

	for _, tc := range []struct {
		name, second string
	}{
		{"caa-moves", "google-trust-services"},
		{"caa-stays", ""},
	} {
		It("with second issuer "+cmp.Or(tc.second, "(none)"), func(ctx SpecContext) {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: namespace},
				Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/example/app:v1", Port: 3000},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			ing := &networkingv1.Ingress{}
			DeferCleanup(func(ctx SpecContext) {
				_ = k8sClient.Delete(ctx, ing)
				_ = k8sClient.Delete(ctx, app)
			})
			r := &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(), BaseDomain: "onbex.co",
				ClusterIssuer: "letsencrypt-prod", SecondClusterIssuer: tc.second,
			}
			platform, custom := tc.name+".onbex.co", "shop."+tc.name+".example"
			hosts := []string{platform, custom}
			Expect(r.reconcileIngress(ctx, app, hosts, tc.name, 3000, nil)).To(Succeed())
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: tc.name}, ing)).To(Succeed())
			Expect(ing.Annotations).To(HaveKeyWithValue(certManagerClusterIssuerAnnotation, "letsencrypt-prod"),
				"ingress-shim still orders every host from the first issuer")
			secret := appv1alpha1.TLSSecretName(tc.name, false, custom)
			shimCertificate(ctx, ing, secret, custom)
			before, err := readCertificate(ctx, secret)
			Expect(err).NotTo(HaveOccurred())

			Expect(r.reconcileIngress(ctx, app, hosts, tc.name, 3000, nil)).To(Succeed())
			after, err := readCertificate(ctx, secret)
			Expect(err).NotTo(HaveOccurred())
			issuer, _, _ := unstructured.NestedMap(after.Object, "spec", "issuerRef")
			if tc.second == "" {
				Expect(after.GetResourceVersion()).To(Equal(before.GetResourceVersion()),
					"with no second issuer the operator must not write the Certificate")
				Expect(metav1.IsControlledBy(after, ing)).To(BeTrue())
				return
			}
			Expect(issuer).To(Equal(clusterIssuerRef(tc.second)))
			Expect(metav1.GetControllerOf(after).UID).To(Equal(app.UID),
				"the Certificate becomes the App's, so ingress-shim no longer puts the first issuer back")
			Expect(after.Object["spec"]).To(HaveKeyWithValue("secretName", secret), "the Secret Traefik serves keeps its name")

			By("the earlier generation's failure does not move it again")
			Expect(r.reconcileIngress(ctx, app, hosts, tc.name, 3000, nil)).To(Succeed())
			again, err := readCertificate(ctx, secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(again.GetResourceVersion()).To(Equal(after.GetResourceVersion()))

			By("dropping the host deletes the Certificate ingress-shim would not")
			Expect(r.reconcileIngress(ctx, app, []string{platform}, tc.name, 3000, nil)).To(Succeed())
			Eventually(func() bool {
				_, err := readCertificate(ctx, secret)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())
		})
	}
})
