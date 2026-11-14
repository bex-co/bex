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
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m357: cert-manager's own words for a Let's Encrypt refusal, as read from
// production's errored onbex.co Certificates on 2026-11-14.
const rateLimitedIssuing = `The certificate request has failed to complete and will be retried: Failed to wait for order resource "x-1-1" to become ready: order is in "errored" state: Failed to create Order: 429 urn:ietf:params:acme:error:rateLimited: too many certificates (50) already issued for "bex.co" in the last 168h0m0s, retry after 2026-11-13 05:33:55 UTC`

// setIssuing plays cert-manager's issuing controller on a Certificate.
func setIssuing(cert *unstructured.Unstructured, status, reason, message string, generation int64) {
	_ = unstructured.SetNestedSlice(cert.Object, []any{
		map[string]any{"type": "Issuing", "status": status, "reason": reason, "message": message, "observedGeneration": generation},
		map[string]any{"type": "Ready", "status": "False", "reason": "DoesNotExist", "observedGeneration": generation},
	}, "status", "conditions")
}

func TestCertificateIssuanceNamesTheFailure(t *testing.T) {
	cert := &unstructured.Unstructured{Object: map[string]any{}}
	cert.SetGeneration(2)
	for _, tc := range []struct {
		name                    string
		status, reason, message string
		generation              int64
		want                    string
	}{
		{"no conditions yet", "", "", "", 0, appv1alpha1.ReasonPublicCertificateIssuing},
		{"issuing", "True", "Issuing", "Issuing certificate as Secret does not exist", 2, appv1alpha1.ReasonPublicCertificateIssuing},
		{"rate limited", "False", "Failed", rateLimitedIssuing, 2, appv1alpha1.ReasonPublicCertificateRateLimited},
		{"other failure", "False", "Failed", "The certificate request has failed to complete and will be retried: challenge failed", 2, appv1alpha1.ReasonPublicCertificateIssuanceFailed},
		{"failure from an older name set", "False", "Failed", rateLimitedIssuing, 1, appv1alpha1.ReasonPublicCertificateIssuing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.status == "" {
				unstructured.RemoveNestedField(cert.Object, "status")
			} else {
				setIssuing(cert, tc.status, tc.reason, tc.message, tc.generation)
			}
			reason, message := certificateIssuance(cert)
			if reason != tc.want {
				t.Fatalf("reason = %q, want %q", reason, tc.want)
			}
			if tc.want != appv1alpha1.ReasonPublicCertificateIssuing && message != tc.message {
				t.Fatalf("message = %q, want cert-manager's %q", message, tc.message)
			}
		})
	}
}

// TestRateLimitedPublicDatabaseSaysSo is the w4/m357 regression: on
// 2026-11-13 every new public database read "Issuing" for hours while Let's
// Encrypt refused its order. The condition now names the refusal (which
// bex-api reports as externalCertificate "failed"), the private-CA contract
// stays, and a later issuance still flips it to PubliclyTrusted.
func TestRateLimitedPublicDatabaseSaysSo(t *testing.T) {
	root, tlsCrt := issuedChain(t, publicNames, publicTLSNow.Add(90*24*time.Hour))
	h := newPublicTLSHarness(t, "letsencrypt-prod", publicTLSRoots(root))
	h.reconcile()
	h.requireStatus(appv1alpha1.DBExternalCertificatePending, appv1alpha1.ReasonPublicCertificateIssuing)

	cert, err := h.certificate()
	if err != nil {
		t.Fatal(err)
	}
	setIssuing(cert, "False", "Failed", rateLimitedIssuing, cert.GetGeneration())
	if err := h.cl.Update(h.ctx, cert); err != nil {
		t.Fatal(err)
	}
	h.reconcile()
	h.requireStatus(appv1alpha1.DBExternalCertificatePending, appv1alpha1.ReasonPublicCertificateRateLimited)
	if certs := clusterCertificates(h.cluster()); certs["serverTLSSecret"] != nil {
		t.Fatalf("a rate-limited certificate was wired into CNPG: %v", certs)
	}
	if c := apimeta.FindStatusCondition(h.database().Status.Conditions, appv1alpha1.ConditionPublicCertificateReady); !strings.Contains(c.Message, "rateLimited") {
		t.Fatalf("condition message %q dropped cert-manager's error", c.Message)
	}

	h.markIssued(tlsCrt)
	h.reconcile()
	h.cnpgObserves("dpg-orders-public-tls")
	h.reconcile()
	h.requireStatus(appv1alpha1.DBExternalCertificatePubliclyTrusted, appv1alpha1.ReasonPublicCertificateIssued)
}

func TestKeyValuePublicCertificateCondition(t *testing.T) {
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-cache", Namespace: "tea-ws", Generation: 1}}
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certManagerCertificateGVK)
	cert.SetName("red-cache-kv-tls")
	cert.SetNamespace("tea-ws")
	cert.SetGeneration(1)
	setIssuing(cert, "False", "Failed", rateLimitedIssuing, 1)
	r := &KeyValueReconciler{Client: fake.NewClientBuilder().WithScheme(publicTLSScheme(t)).WithObjects(cert).Build()}
	reason := func() string {
		c := apimeta.FindStatusCondition(kv.Status.Conditions, appv1alpha1.ConditionPublicCertificateReady)
		if c == nil {
			return ""
		}
		return c.Reason
	}

	// Round 254/304: public, but no listener while issuance is refused.
	r.setKeyValuePublicCertificate(context.Background(), kv, true, false)
	if got := reason(); got != appv1alpha1.ReasonPublicCertificateRateLimited {
		t.Fatalf("rate-limited store reason = %q", got)
	}
	r.setKeyValuePublicCertificate(context.Background(), kv, true, true)
	if got := reason(); got != appv1alpha1.ReasonPublicCertificateIssued {
		t.Fatalf("listening store reason = %q", got)
	}
	r.setKeyValuePublicCertificate(context.Background(), kv, false, false)
	if got := reason(); got != "" {
		t.Fatalf("private store kept the condition (%q)", got)
	}
}

// TestDatastoreMetricsSurfaceAPendingPublicCertificate: the series behind
// PublicCertificatePendingTooLong measures since the condition turned False,
// and nothing is exported for an issued or private datastore.
func TestDatastoreMetricsSurfaceAPendingPublicCertificate(t *testing.T) {
	pending := func(reason string, since time.Duration) []metav1.Condition {
		return []metav1.Condition{{
			Type: appv1alpha1.ConditionPublicCertificateReady, Status: metav1.ConditionFalse,
			Reason: reason, LastTransitionTime: metav1.NewTime(time.Now().Add(-since)),
		}}
	}
	cl := fake.NewClientBuilder().WithScheme(newTestScheme(t)).WithObjects(
		&appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-limited", Namespace: "tea-ws"},
			Status:     appv1alpha1.DatabaseStatus{Conditions: pending(appv1alpha1.ReasonPublicCertificateRateLimited, 20*time.Minute)},
		},
		&appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "dpg-trusted", Namespace: "tea-ws"},
			Status: appv1alpha1.DatabaseStatus{Conditions: []metav1.Condition{{
				Type: appv1alpha1.ConditionPublicCertificateReady, Status: metav1.ConditionTrue,
				Reason: appv1alpha1.ReasonPublicCertificateIssued, LastTransitionTime: metav1.Now(),
			}}},
		},
		&appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: "red-waiting", Namespace: "tea-ws"},
			Status:     appv1alpha1.KeyValueStatus{Conditions: pending(appv1alpha1.ReasonPublicCertificateIssuing, 2*time.Minute)},
		},
	).Build()

	got := renderMetric(t, NewDatastoreCollector(cl), "bex_datastore_public_certificate_pending_seconds")
	for _, want := range []string{
		`bex_datastore_public_certificate_pending_seconds{kind="database",name="dpg-limited",namespace="tea-ws",reason="RateLimited"} 120`,
		`bex_datastore_public_certificate_pending_seconds{kind="keyvalue",name="red-waiting",namespace="tea-ws",reason="Issuing"} 12`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing series:\n  want %s…\n  got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "dpg-trusted") {
		t.Errorf("an issued certificate exported a pending series:\n%s", got)
	}
}
