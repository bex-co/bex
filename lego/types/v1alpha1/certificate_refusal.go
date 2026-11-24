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

import (
	"regexp"
	"strings"
)

// caaWord is "CAA" as a word: Let's Encrypt's refusal reads "CAA record for
// example.com prevents issuance", and cert-manager keeps only that detail on
// the Challenge.
var caaWord = regexp.MustCompile(`\bCAA\b`)

// CertificateRefusedOnCAA reports whether cert-manager's text for a failed
// certificate attempt says the CA refused the host because the domain's CAA
// records do not allow it: the ACME `caa` problem type, or a Challenge detail
// naming CAA. A DNS failure while looking the records up ("DNS problem:
// SERVFAIL looking up CAA") is not a refusal: adding a record would not fix
// it. The operator moves such a host to the other ClusterIssuer and bex-api
// names the values to add, so both read the text with this one predicate
// (ADR005 § Two certificate authorities, w21/m618).
func CertificateRefusedOnCAA(text string) bool {
	if strings.Contains(text, "urn:ietf:params:acme:error:caa") {
		return true
	}
	return caaWord.MatchString(text) && !strings.Contains(text, "DNS problem")
}
