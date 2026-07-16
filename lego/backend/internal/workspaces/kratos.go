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
	"net/http"
	"net/url"
	"strings"
)

// kratos.go is the IdentityReader implementation over Kratos' admin API
// (BEX_KRATOS_ADMIN_URL) — Kratos' own service, on its admin port, distinct
// from the public port api/auth.go's session whoami hits (BEX_KRATOS_URL). The
// admin API is required here because a workspace's owners/members endpoints
// look up OTHER members' identities, not just the caller's own session.

// KratosIdentities looks up identity attributes at Kratos' admin API
// (GET /admin/identities/{id}). Satisfies IdentityReader.
type KratosIdentities struct {
	AdminURL string // Kratos admin base URL, e.g. http://kratos-admin.auth.svc:4434
	Client   *http.Client
}

// NewKratosIdentities builds a reader against the given admin base URL.
func NewKratosIdentities(adminURL string) *KratosIdentities {
	return &KratosIdentities{AdminURL: strings.TrimSuffix(adminURL, "/")}
}

func (k *KratosIdentities) client() *http.Client {
	if k.Client != nil {
		return k.Client
	}
	return http.DefaultClient
}

// kratosIdentity is the subset of Kratos' identity schema this reader needs:
// the `email` trait plus the optional `name` display-name trait (w4/m25; ""
// for identities that never set it). Credentials are requested via
// include_credential so mfaEnabled can be derived without a second call.
type kratosIdentity struct {
	Traits struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"traits"`
	Credentials map[string]struct {
		Type   string `json:"type"`
		Config struct {
			Credentials []json.RawMessage `json:"credentials"`
		} `json:"config"`
	} `json:"credentials"`
}

// Lookup fetches one identity's email + name + MFA state. ok=false on any failure
// (not found, admin API unreachable, bad response) — the honest-omit contract
// IdentityReader documents; a Kratos outage degrades the owners/members
// responses (fields missing) rather than failing the request.
func (k *KratosIdentities) Lookup(ctx context.Context, subject string) (IdentityAttrs, bool) {
	u := k.AdminURL + "/admin/identities/" + url.PathEscape(subject) +
		"?include_credential=totp&include_credential=webauthn"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return IdentityAttrs{}, false
	}
	resp, err := k.client().Do(req)
	if err != nil {
		return IdentityAttrs{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return IdentityAttrs{}, false
	}
	var id kratosIdentity
	if err := json.NewDecoder(resp.Body).Decode(&id); err != nil {
		return IdentityAttrs{}, false
	}
	// totp is presence-based: Kratos only writes the entry on enrollment.
	// webauthn needs its config inspected: Kratos mints a stub entry
	// (config.user_handle only, no registered keys) at password registration
	// because webauthn is an enabled method with identifier: true — so only a
	// non-empty config.credentials list counts as an enrolled second factor
	// (w4/020).
	_, totp := id.Credentials["totp"]
	webauthn := len(id.Credentials["webauthn"].Config.Credentials) > 0
	return IdentityAttrs{Email: id.Traits.Email, Name: id.Traits.Name, MFAEnabled: totp || webauthn}, true
}
