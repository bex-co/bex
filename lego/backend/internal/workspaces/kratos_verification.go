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

	"github.com/google/uuid"
)

// EmailVerification answers the SSH gateway's verification question (w2/m168)
// for a stored key owner. Unlike Lookup, which flattens every failure into
// ok=false, it keeps "not a Kratos identity" (404: a machine API-key client
// that registered a key, so human=false and exempt) apart from "Kratos could
// not answer" (an error, which the caller fails closed on).
func (k *KratosIdentities) EmailVerification(ctx context.Context, subject string) (human, verified bool, err error) {
	id, found, err := k.adminIdentity(ctx, subject, "")
	if err != nil || !found {
		return false, false, err
	}
	return true, id.traitEmailVerified(), nil
}

// MFAEnrolled answers the workspace two-factor gate's question (w22/m402,
// docs/ADR012-auth.md §9a): does this Kratos identity have an enrolled second
// factor? The rule is attrs()'s — a TOTP credential, or a WebAuthn credential
// that carries a registered key — not "this session is AAL2", so a session
// minted before enrollment is not locked out by a test stricter than Render's.
// Like EmailVerification it keeps "not a Kratos identity" (a 404, or a subject
// that is not a UUID, such as a named platform client: human=false, exempt)
// apart from "Kratos could not answer" (an error, which the caller fails
// closed on). Only webauthn is included: TOTP is presence-based, and asking for
// its config would load its secret (w5/127).
func (k *KratosIdentities) MFAEnrolled(ctx context.Context, subject string) (enrolled, human bool, err error) {
	if _, perr := uuid.Parse(subject); perr != nil {
		return false, false, nil
	}
	id, found, err := k.adminIdentity(ctx, subject, "?include_credential=webauthn")
	if err != nil || !found {
		return false, false, err
	}
	return id.attrs().MFAEnabled, true, nil
}

// adminIdentity reads one identity from the admin API: found=false is Kratos'
// 404 (the subject is not an identity), an error is anything else that kept it
// from answering.
func (k *KratosIdentities) adminIdentity(ctx context.Context, subject, query string) (id kratosIdentity, found bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.AdminURL+"/admin/identities/"+url.PathEscape(subject)+query, nil)
	if err != nil {
		return kratosIdentity{}, false, err
	}
	resp, err := k.client().Do(req)
	if err != nil {
		return kratosIdentity{}, false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return kratosIdentity{}, false, nil
	default:
		return kratosIdentity{}, false, fmt.Errorf("kratos admin identity lookup returned %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&id); err != nil {
		return kratosIdentity{}, false, err
	}
	return id, true, nil
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
