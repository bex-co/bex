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

package v1alpha1

import "testing"

func TestCertificateRefusedOnCAA(t *testing.T) {
	for text, want := range map[string]bool{
		// The Challenge's reason: cert-manager keeps the ACME detail only.
		"CAA record for shop.example.com prevents issuance": true,
		// A finalize-time refusal reaches the Certificate with the problem type.
		`The certificate request has failed to complete and will be retried: Failed to wait for order resource "o" to become ready: order is in "errored" state: Failed to finalize Order: 403 urn:ietf:params:acme:error:caa: CAA record for shop.example.com prevents issuance`: true,
		// A lookup failure is DNS, not policy: no record to add fixes it.
		"While processing CAA for shop.example.com: DNS problem: SERVFAIL looking up CAA for shop.example.com - the domain's nameservers may be malfunctioning": false,
		"429 urn:ietf:params:acme:error:rateLimited: too many certificates already issued":                                                                      false,
		"Waiting for HTTP-01 challenge propagation: wrong status code '404', expected '200'":                                                                    false,
		// "CAA" inside another word is not the record type.
		"VACAANT":       false,
		"caa lowercase": false,
		"":              false,
	} {
		if got := CertificateRefusedOnCAA(text); got != want {
			t.Errorf("CertificateRefusedOnCAA(%q) = %v, want %v", text, got, want)
		}
	}
}
