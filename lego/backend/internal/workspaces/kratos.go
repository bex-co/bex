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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// kratos.go is the IdentityReader implementation over Kratos' admin API
// (BEX_KRATOS_ADMIN_URL) — Kratos' own service, on its admin port, distinct
// from the public port api/auth.go's session whoami hits (BEX_KRATOS_URL). The
// admin API is required here because a workspace's owners/members endpoints
// look up OTHER members' identities, not just the caller's own session.

// KratosIdentities looks up identity attributes at Kratos' admin API
// (GET /admin/identities/{id}; LookupMany uses ?ids=…). Satisfies
// IdentityReader.
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
	return defaultKratosClient
}

// defaultKratosClient bounds every admin lookup: the auth gate now calls one on
// the request path (w2/m168), where http.DefaultClient's missing timeout would
// let a hung Kratos hold the introspection singleflight indefinitely.
var defaultKratosClient = &http.Client{Timeout: 5 * time.Second, Transport: core.OryTransport}

// kratosIdentity is the subset of Kratos' identity schema this reader needs:
// the `email` trait plus the optional `name` display-name trait (w4/m25; ""
// for identities that never set it). Credentials are requested via
// include_credential so mfaEnabled can be derived without a second call.
type kratosIdentity struct {
	ID     string `json:"id"`
	Traits struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"traits"`
	// VerifiableAddresses carries Kratos' own verification state per address;
	// only the entry matching the email trait decides EmailVerified.
	VerifiableAddresses []struct {
		Value    string `json:"value"`
		Verified bool   `json:"verified"`
	} `json:"verifiable_addresses"`
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
	var id kratosIdentity
	if !k.get(ctx, "/admin/identities/"+url.PathEscape(subject)+"?include_credential=totp&include_credential=webauthn", &id) {
		return IdentityAttrs{}, false
	}
	return id.attrs(), true
}

// kratosBatchSize keeps one LookupMany request inside Kratos' limits (at most
// 500 ids, a default page of 250); 100 ids is about 4 KB of query string.
const kratosBatchSize = 100

// LookupMany resolves many identities with one admin request per
// kratosBatchSize subjects (w5/116), with the same credential includes as
// Lookup. It returns the subjects that resolved, keyed as asked. A subject
// Kratos does not know is absent, and so is one that is not a UUID, such as a
// named platform client (bex-bootstrap), which is never sent: Kratos refuses a
// whole batch over one malformed id. A failed batch leaves its subjects absent
// too, so the caller omits their fields, as with Lookup.
func (k *KratosIdentities) LookupMany(ctx context.Context, subjects []string) map[string]IdentityAttrs {
	asked := map[string][]string{} // Kratos' canonical id → the subjects spelling it
	var ids []string
	for _, subject := range subjects {
		id, err := uuid.Parse(subject)
		if err != nil {
			continue
		}
		canonical := id.String()
		if asked[canonical] == nil {
			ids = append(ids, canonical)
		}
		asked[canonical] = append(asked[canonical], subject)
	}
	out := make(map[string]IdentityAttrs, len(subjects))
	for batch := range slices.Chunk(ids, kratosBatchSize) {
		query := url.Values{
			"ids":                batch,
			"include_credential": {"totp", "webauthn"},
			"page_size":          {strconv.Itoa(len(batch))},
		}
		var found []kratosIdentity
		if !k.get(ctx, "/admin/identities?"+query.Encode(), &found) {
			continue
		}
		for _, id := range found {
			for _, subject := range asked[id.ID] {
				out[subject] = id.attrs()
			}
		}
	}
	return out
}

// get decodes the admin API's answer to path, query included, into v; false on
// any failure.
func (k *KratosIdentities) get(ctx context.Context, path string, v any) bool {
	return core.DoJSON(ctx, k.client(), http.MethodGet, k.AdminURL+path, "", nil, http.StatusOK, v) == nil
}

func (id kratosIdentity) attrs() IdentityAttrs {
	// totp is presence-based: Kratos only writes the entry on enrollment.
	// webauthn needs its config inspected: Kratos mints a stub entry
	// (config.user_handle only, no registered keys) at password registration
	// because webauthn is an enabled method with identifier: true — so only a
	// non-empty config.credentials list counts as an enrolled second factor
	// (w4/020).
	_, totp := id.Credentials["totp"]
	webauthn := len(id.Credentials["webauthn"].Config.Credentials) > 0
	return IdentityAttrs{Email: id.Traits.Email, EmailVerified: id.traitEmailVerified(), Name: id.Traits.Name, MFAEnabled: totp || webauthn}
}
