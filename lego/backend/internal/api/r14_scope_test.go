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

package api

// r14_scope_test.go guards codex-security round 14 finding #1
// (docs/ADR069-security-review-round14.md): a dynamically registered client
// could request the API resource AUDIENCE with identity-only SCOPES, walk the
// ordinary consent flow, and exercise every action the victim's OpenFGA
// subject may perform — because introspection discarded scope and
// authorization checked only the subject. The gate now requires the
// control-plane capability scope (DefaultAPIScope "bex.api") on an
// API-audience HUMAN token, exempting the machine (API-key) class and
// bex-provisioned platform clients exactly as the narrowed empty-audience
// exception (w1/m67 F1) does.

import (
	"net/http"
	"testing"
)

const identityScopes = "openid offline_access"

// TestIdentityScopedAudienceTokenFromThirdPartyClientIsRefused is the round-14
// #1 regression: the token passes the audience check (it asked for this
// resource) but its granted scopes are identity-only — the consent screen a
// dynamic client shows while obtaining the victim's full authority.
func TestIdentityScopedAudienceTokenFromThirdPartyClientIsRefused(t *testing.T) {
	h := newClassHydraScoped(t, "identity-1", "dcr-client", identityScopes,
		[]string{bexResource}, map[string]bool{"dcr-client": false})

	if got := authStatusScoped(t, h, bexResource, true, DefaultAPIScope); got != http.StatusUnauthorized {
		t.Errorf("identity-scoped audience token from a third-party client = %d, want 401", got)
	}
	if h.clientLookups.Load() == 0 {
		t.Error("the decision must consult Hydra's client record — the scope exemption is proven, not inferred from the token")
	}
}

// The caller shapes that must keep working with the scope rule armed.
func TestScopeRuleKeepsLegitimateCallers(t *testing.T) {
	for _, tc := range []struct {
		name             string
		sub, clientID    string
		scope            string
		aud              []string
		platform         map[string]bool
		wantClientLookup bool
	}{
		{
			// An MCP/agent client that requested the resource AND the advertised
			// capability scope — the ordinary compliant connect.
			name:     "third-party client with the API scope",
			sub:      "identity-1", clientID: "dcr-client",
			scope:    "openid offline_access " + DefaultAPIScope,
			aud:      []string{bexResource},
			platform: map[string]bool{"dcr-client": false},
		},
		{
			// bex-mobile: requests the API audience with identity-only scopes
			// today; admitted because bex provisioned it (the marker), exactly
			// like the audience-less device-flow client.
			name:             "bex-provisioned client without the scope yet",
			sub:              "identity-1", clientID: "bex-mobile",
			scope:            identityScopes,
			aud:              []string{bexResource},
			platform:         map[string]bool{"bex-mobile": true},
			wantClientLookup: true,
		},
		{
			// API keys are client_credentials: their authority comes from the
			// workspace binding, not an OAuth consent, so an audience-bearing
			// machine token needs no scope.
			name:     "api key (client_credentials) with an audience",
			sub:      "key-1", clientID: "key-1",
			aud:      []string{bexResource},
			platform: map[string]bool{},
		},
		{
			// The official Render CLI's device flow carries no audience at all,
			// so the scope rule never arms for it.
			name:             "audience-less device-flow token",
			sub:              "identity-1", clientID: "render-cli",
			scope:            identityScopes,
			platform:         map[string]bool{"render-cli": true},
			wantClientLookup: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newClassHydraScoped(t, tc.sub, tc.clientID, tc.scope, tc.aud, tc.platform)
			if got := authStatusScoped(t, h, bexResource, true, DefaultAPIScope); got != http.StatusOK {
				t.Errorf("status = %d, want 200", got)
			}
			if lookups := h.clientLookups.Load(); tc.wantClientLookup != (lookups > 0) {
				t.Errorf("client lookups = %d, wantLookup = %v — the extra Hydra call must be paid only where the exemption is actually needed",
					lookups, tc.wantClientLookup)
			}
		})
	}
}

// The rule is inert without a configured resource or scope, so a deployment
// with discovery off keeps byte-identical behavior.
func TestScopeRuleIsInertUnarmed(t *testing.T) {
	h := newClassHydraScoped(t, "identity-1", "dcr-client", identityScopes,
		[]string{bexResource}, map[string]bool{"dcr-client": false})
	if got := authStatusScoped(t, h, "", true, DefaultAPIScope); got != http.StatusOK {
		t.Errorf("no configured resource: status = %d, want 200", got)
	}
	if got := authStatusScoped(t, h, bexResource, true, ""); got != http.StatusOK {
		t.Errorf("no configured scope: status = %d, want 200", got)
	}
	if h.clientLookups.Load() != 0 {
		t.Error("no client lookup may happen while the rule is inert")
	}
}

// TestScopeGrantedParsesHydrasSpaceSeparatedScope pins the tiny parser:
// substring matches and empty strings must not read as granted.
func TestScopeGrantedParsesHydrasSpaceSeparatedScope(t *testing.T) {
	if scopeGranted("openid "+DefaultAPIScope, DefaultAPIScope) != true {
		t.Error("scope present in the granted set must read granted")
	}
	for _, not := range []string{"", "openid offline_access", "openid not" + DefaultAPIScope + " x", DefaultAPIScope + "-readonly"} {
		if scopeGranted(not, DefaultAPIScope) {
			t.Errorf("scope %q must not read as granted", not)
		}
	}
}
