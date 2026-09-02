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

// r14_scope_test.go guards the round-14 #1 hole as tightened by w8/m27:
// a dynamically registered client that requests the API audience without a
// granular capability (bex.read / bex.write / bex.sensitive) introspects
// inactive. Legacy bex.api is not an umbrella grant for a third-party client.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const identityScopes = "openid offline_access"

func TestIdentityScopedAudienceTokenFromThirdPartyClientIsRefused(t *testing.T) {
	h := newClassHydraScoped(t, "identity-1", "dcr-client", identityScopes,
		[]string{bexResource}, map[string]bool{"dcr-client": false})

	if got := authStatusScoped(t, h, bexResource, true, DefaultAPIScope); got != http.StatusUnauthorized {
		t.Errorf("identity-scoped audience token from a third-party client = %d, want 401", got)
	}
	if h.clientLookups.Load() != 0 {
		t.Error("the decision must not consult attacker-writable Hydra client metadata")
	}
}

func TestLegacyAPIScopeDoesNotAdmitThirdPartyHuman(t *testing.T) {
	h := newClassHydraScoped(t, "identity-1", "dcr-client", "openid offline_access "+core.ScopeAPICompatibility,
		[]string{bexResource}, map[string]bool{"dcr-client": false})
	if got := authStatusScoped(t, h, bexResource, true, DefaultAPIScope); got != http.StatusUnauthorized {
		t.Errorf("third-party bex.api token = %d, want 401", got)
	}
}

func TestScopeRuleKeepsLegitimateCallers(t *testing.T) {
	for _, tc := range []struct {
		name          string
		sub, clientID string
		scope         string
		aud           []string
		platform      map[string]bool
	}{
		{
			name: "third-party client with a granular capability",
			sub:  "identity-1", clientID: "dcr-client",
			scope:    "openid offline_access " + core.ScopeRead,
			aud:      []string{bexResource},
			platform: map[string]bool{"dcr-client": false},
		},
		{
			name: "bex-provisioned client without a granular scope yet",
			sub:  "identity-1", clientID: "bex-mobile",
			scope:    identityScopes,
			aud:      []string{bexResource},
			platform: map[string]bool{"bex-mobile": true},
		},
		{
			name: "api key (client_credentials) with an audience",
			sub:  "key-1", clientID: "key-1",
			aud:      []string{bexResource},
			platform: map[string]bool{},
		},
		{
			name: "audience-less device-flow token",
			sub:  "identity-1", clientID: "render-cli",
			scope:    identityScopes,
			platform: map[string]bool{"render-cli": true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newClassHydraScoped(t, tc.sub, tc.clientID, tc.scope, tc.aud, tc.platform)
			if got := authStatusScoped(t, h, bexResource, true, DefaultAPIScope); got != http.StatusOK {
				t.Errorf("status = %d, want 200", got)
			}
			if lookups := h.clientLookups.Load(); lookups != 0 {
				t.Errorf("Hydra client metadata lookups = %d, want 0", lookups)
			}
		})
	}
}

func TestCachedIdentityCarriesNormalizedGrant(t *testing.T) {
	h := newClassHydraScoped(t, "identity-1", "dcr-client", "openid bex.write bex.read bex.write",
		[]string{bexResource, "https://other.example"}, map[string]bool{"dcr-client": false})
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := core.IdentityFrom(r.Context())
		fmt.Fprintf(w, "%s|%s|%t", id.CanonicalScopes, id.AcceptedAudience, id.PlatformClient)
	})
	mw := newOryAuth(h.url, "", bexResource, "", "", true, nil, nil, nil, DefaultAPIScope).middleware(probe)
	var bodies [2]string
	for i := range bodies {
		r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
		r.Header.Set("Authorization", "Bearer "+testToken)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("pass %d status = %d", i, w.Code)
		}
		bodies[i] = w.Body.String()
	}
	want := "bex.read bex.write|" + bexResource + "|false"
	if bodies[0] != want || bodies[1] != want {
		t.Errorf("cache miss/hit = %q / %q, want %q", bodies[0], bodies[1], want)
	}
}

func TestMalformedGrantIntrospectsInactive(t *testing.T) {
	oversize := make([]byte, core.MaxOAuthScopeLen+1)
	for i := range oversize {
		oversize[i] = 'a'
	}
	h := newClassHydraScoped(t, "identity-1", "dcr-client", string(oversize),
		[]string{bexResource}, map[string]bool{"dcr-client": false})
	if got := authStatusScoped(t, h, bexResource, true, DefaultAPIScope); got != http.StatusUnauthorized {
		t.Errorf("oversized grant = %d, want 401", got)
	}
}

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
