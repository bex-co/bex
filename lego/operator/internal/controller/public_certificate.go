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
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Shared by the Database and Key Value public certificates (w4/m357): what a
// not-yet-Ready cert-manager Certificate is doing, and how long each public
// datastore has been waiting for one.

// certificateIssuance names a not-Ready Certificate's progress as a
// PublicCertificateReady reason and message. cert-manager records a failed
// attempt as Issuing=False/Failed with the ACME error in the message, then
// retries with exponential backoff (1h up to 32h); before w4/m357 every such
// failure read as "Issuing", so a rate-limited database looked merely slow.
// A failure left over from a previous generation (an older name set) is
// ignored, as certificateReady ignores a stale Ready.
func certificateIssuance(cert *unstructured.Unstructured) (reason, message string) {
	conditions, _, _ := unstructured.NestedSlice(cert.Object, "status", "conditions")
	for _, raw := range conditions {
		c, ok := raw.(map[string]any)
		if !ok || c["type"] != "Issuing" || c["status"] != "False" || c["reason"] != "Failed" {
			continue
		}
		if observed, found, _ := unstructured.NestedInt64(c, "observedGeneration"); found && observed != cert.GetGeneration() {
			continue
		}
		message, _ = c["message"].(string)
		if strings.Contains(message, "rateLimited") {
			return appv1alpha1.ReasonPublicCertificateRateLimited, message
		}
		return appv1alpha1.ReasonPublicCertificateIssuanceFailed, message
	}
	return appv1alpha1.ReasonPublicCertificateIssuing, "waiting for the publicly trusted certificate to be issued"
}

// setKeyValuePublicCertificate reports a public Key Value's certificate as
// its PublicCertificateReady condition. The external TLS listener opens only
// once cert-manager has written the certificate (ADR021), so until then a
// public store has no external access at all, and bex-api tells the user so
// instead of presenting the store as reachable (w4/m357).
func (r *KeyValueReconciler) setKeyValuePublicCertificate(ctx context.Context, kv *appv1alpha1.KeyValue, public, listening bool) {
	if !public {
		meta.RemoveStatusCondition(&kv.Status.Conditions, appv1alpha1.ConditionPublicCertificateReady)
		return
	}
	cond := metav1.Condition{
		Type: appv1alpha1.ConditionPublicCertificateReady, Status: metav1.ConditionTrue,
		Reason:             appv1alpha1.ReasonPublicCertificateIssued,
		Message:            "the external endpoint serves a publicly trusted certificate",
		ObservedGeneration: kv.Generation,
	}
	if !listening {
		cond.Status = metav1.ConditionFalse
		cert := &unstructured.Unstructured{}
		cert.SetGroupVersionKind(certManagerCertificateGVK)
		// A Certificate not in the cache yet reads as still issuing.
		if err := r.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: keyValueTLSName(kv)}, cert); err == nil && certificateReady(cert) {
			cond.Reason, cond.Message = "ListenerStarting", "the certificate is issued; the external TLS listener is opening"
		} else {
			cond.Reason, cond.Message = certificateIssuance(cert)
		}
	}
	meta.SetStatusCondition(&kv.Status.Conditions, cond)
}
