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

package workspaces

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// EmailVerification answers the SSH gateway's verification question (w2/m168)
// for a stored key owner. Unlike Lookup, which flattens every failure into
// ok=false, it keeps "not a Kratos identity" (404: a machine API-key client
// that registered a key, so human=false and exempt) apart from "Kratos could
// not answer" (an error, which the caller fails closed on).
func (k *KratosIdentities) EmailVerification(ctx context.Context, subject string) (human, verified bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.AdminURL+"/admin/identities/"+url.PathEscape(subject), nil)
	if err != nil {
		return false, false, err
	}
	resp, err := k.client().Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return false, false, nil
	default:
		return false, false, fmt.Errorf("kratos admin identity lookup returned %s", resp.Status)
	}
	var id kratosIdentity
	if err := json.NewDecoder(resp.Body).Decode(&id); err != nil {
		return false, false, err
	}
	return true, id.traitEmailVerified(), nil
}

// traitEmailVerified reports whether Kratos verified the address matching the
// email trait; a verified secondary address does not count.
func (id kratosIdentity) traitEmailVerified() bool {
	for _, a := range id.VerifiableAddresses {
		if a.Verified && strings.EqualFold(a.Value, id.Traits.Email) {
			return true
		}
	}
	return false
}
