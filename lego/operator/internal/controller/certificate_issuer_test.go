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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// w21/m618: when a host's certificate moves to the other CA.

func TestCertManagerRetryDelay(t *testing.T) {
	for attempts, want := range map[int64]time.Duration{
		0: time.Hour, 1: time.Hour, 2: 2 * time.Hour, 3: 4 * time.Hour,
		5: 16 * time.Hour, 6: 32 * time.Hour, 7: 32 * time.Hour, 40: 32 * time.Hour,
	} {
		if got := certManagerRetryDelay(attempts); got != want {
			t.Errorf("certManagerRetryDelay(%d) = %v, want %v (cert-manager's backoff)", attempts, got, want)
		}
	}
}

func TestIssuerMoveDue(t *testing.T) {
	failedAt := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) time.Time { return failedAt.Add(d) }
	for _, tc := range []struct {
		name     string
		attempts int64
		refused  bool
		now      time.Time
		want     bool
	}{
		{"a first refusal moves at once", 1, true, at(time.Second), true},
		{"a first ordinary failure retries the same CA", 1, false, at(48 * time.Hour), false},
		{"a second refusal waits for cert-manager's retry", 2, true, at(time.Hour), false},
		{"a second refusal moves just before that retry", 2, true, at(2*time.Hour - issuerMoveLead), true},
		{"a second ordinary failure never moves", 2, false, at(48 * time.Hour), false},
		{"a third failure moves at its retry", 3, false, at(4*time.Hour - issuerMoveLead), true},
		{"a third failure waits until then", 3, false, at(4*time.Hour - issuerMoveLead - time.Second), false},
		{"a long streak follows the 32-hour cap", 9, true, at(32*time.Hour - issuerMoveLead), true},
	} {
		got := issuerMoveDue(issuanceFailure{attempts: tc.attempts, at: failedAt}, tc.refused, tc.now)
		if got != tc.want {
			t.Errorf("%s: issuerMoveDue(attempts %d, refused %v, %v after the failure) = %v, want %v",
				tc.name, tc.attempts, tc.refused, tc.now.Sub(failedAt), got, tc.want)
		}
	}
}

func TestCARefused(t *testing.T) {
	for text, want := range map[string]bool{
		`order is in "errored" state: Failed to create Order: 429 urn:ietf:params:acme:error:rateLimited: too many certificates (5) already issued for this exact set of identifiers`: true,
		`order is in "errored" state: Failed to finalize Order: 403 urn:ietf:params:acme:error:caa: CAA record for shop.example.com prevents issuance`:                                true,
		`order is in "invalid" state: `: false, // a validation failure names its cause only on the Challenge
		"Waiting for HTTP-01 challenge propagation: wrong status code '404', expected '200'": false,
	} {
		if got := caRefused(text); got != want {
			t.Errorf("caRefused(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestCertificateIssuanceFailure(t *testing.T) {
	cert := func(generation, observed int64, status map[string]any) *unstructured.Unstructured {
		c := &unstructured.Unstructured{Object: map[string]any{"status": status}}
		c.SetGeneration(generation)
		conditions := []any{map[string]any{
			"type": "Issuing", "status": "False", "reason": "Failed", "observedGeneration": observed,
			"message": "The certificate request has failed to complete and will be retried: boom",
		}}
		_ = unstructured.SetNestedSlice(c.Object, conditions, "status", "conditions")
		return c
	}
	f, failed := certificateIssuanceFailure(cert(2, 2, map[string]any{
		"failedIssuanceAttempts": int64(3), "lastFailureTime": "2026-11-24T12:00:00Z",
	}))
	if !failed || f.attempts != 3 || !f.at.Equal(time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("failure = %+v (failed %v), want three attempts, the last at 12:00Z", f, failed)
	}
	if f, failed := certificateIssuanceFailure(cert(1, 1, map[string]any{})); !failed || f.attempts != 1 {
		t.Errorf("a failure with no count is the first attempt: %+v (failed %v)", f, failed)
	}
	if _, failed := certificateIssuanceFailure(cert(3, 2, map[string]any{"failedIssuanceAttempts": int64(1)})); failed {
		t.Error("a failure of an earlier generation (before the issuer changed) is not the current attempt's")
	}
}

func TestOtherClusterIssuer(t *testing.T) {
	r := &AppReconciler{ClusterIssuer: "letsencrypt-prod", SecondClusterIssuer: "google-trust-services"}
	for issuer, want := range map[string]string{
		"letsencrypt-prod": "google-trust-services", "google-trust-services": "letsencrypt-prod",
		"letsencrypt-staging": "", "": "",
	} {
		if got := r.otherClusterIssuer(issuer); got != want {
			t.Errorf("otherClusterIssuer(%q) = %q, want %q", issuer, got, want)
		}
	}
	for _, r := range []*AppReconciler{
		{ClusterIssuer: "letsencrypt-prod"},
		{ClusterIssuer: "letsencrypt-prod", SecondClusterIssuer: "letsencrypt-prod"},
	} {
		if got := r.otherClusterIssuer("letsencrypt-prod"); got != "" {
			t.Errorf("with second issuer %q nothing moves, got %q", r.SecondClusterIssuer, got)
		}
	}
}
