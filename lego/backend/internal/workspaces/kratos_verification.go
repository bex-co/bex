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
	"encoding/base64"
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
	id, human, err := k.humanIdentity(ctx, subject, "?include_credential=webauthn")
	return human && id.attrs().MFAEnabled, human, err
}

// GoogleAccountMatches answers the workspace Google-login gate's account
// question (w7/m465, docs/ADR012-auth.md §9b): has this Kratos identity linked
// a Google account whose email is its own email trait? Render: "Team members
// must sign in with the Google account that matches the invited email
// address", and an invite is redeemed against that trait. The Google email is
// the one in the ID token Kratos stored when the account was linked (the
// oidc credential's initial_id_token; Kratos verified it then, so only its
// payload is read). The tokens Kratos returns beside it are never kept. Like
// MFAEnrolled it keeps "not a Kratos identity" (human=false) apart from
// "Kratos could not answer" (an error).
func (k *KratosIdentities) GoogleAccountMatches(ctx context.Context, subject string) (matches, human bool, err error) {
	id, human, err := k.humanIdentity(ctx, subject, "?include_credential=oidc")
	return human && id.googleAccountMatches(), human, err
}

// SettingsFacts is what the Kratos settings guard (internal/settingsguard,
// w7/m465) compares a settings change against: the stored email trait, and
// whether a Google account is linked. The oidc identifiers are listed without
// including the credential's tokens.
func (k *KratosIdentities) SettingsFacts(ctx context.Context, subject string) (email string, googleLinked, found bool, err error) {
	id, found, err := k.humanIdentity(ctx, subject, "")
	if !found {
		return "", false, false, err
	}
	for _, identifier := range id.Credentials["oidc"].Identifiers {
		if strings.HasPrefix(identifier, googleOIDCProvider+":") {
			googleLinked = true
		}
	}
	return id.Traits.Email, googleLinked, true, nil
}

// googleAccountMatches reports whether one of the identity's linked Google
// accounts has the identity's email: the ID token's email equal to the trait
// (case-insensitively), and not marked unverified by Google.
func (id kratosIdentity) googleAccountMatches() bool {
	if id.Traits.Email == "" {
		return false
	}
	for _, p := range id.Credentials["oidc"].Config.Providers {
		if p.Provider != googleOIDCProvider {
			continue
		}
		claims, ok := idTokenClaims(p.InitialIDToken)
		if ok && claims.Email != "" && strings.EqualFold(claims.Email, id.Traits.Email) &&
			(claims.EmailVerified == nil || *claims.EmailVerified) {
			return true
		}
	}
	return false
}

// idTokenClaims reads the email claims of a JWT's payload without checking its
// signature: the token comes from Kratos's admin API, which verified it when
// the account was linked.
func idTokenClaims(token string) (claims struct {
	Email         string `json:"email"`
	EmailVerified *bool  `json:"email_verified"`
}, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return claims, false
	}
	return claims, json.Unmarshal(payload, &claims) == nil
}

// humanIdentity is adminIdentity for a subject that may not be a Kratos
// identity at all: a subject that is not a UUID (a named platform client) is
// "not human" without asking Kratos, like a 404. human is false whenever err
// is set.
func (k *KratosIdentities) humanIdentity(ctx context.Context, subject, query string) (id kratosIdentity, human bool, err error) {
	if _, perr := uuid.Parse(subject); perr != nil {
		return kratosIdentity{}, false, nil
	}
	id, human, err = k.adminIdentity(ctx, subject, query)
	return id, human && err == nil, err
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
