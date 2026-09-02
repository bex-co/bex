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

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// classHydra serves token introspection with a caller-chosen
// subject/client/audience. Its admin client endpoint deliberately claims every
// client is platform-marked; authorization must never consult it.
type classHydra struct {
	url               string
	platformClientIDs []string
	clientLookups     atomic.Int32
}

func newClassHydra(t *testing.T, sub, clientID string, aud []string, platformClients map[string]bool) *classHydra {
	return newClassHydraScoped(t, sub, clientID, "", aud, platformClients)
}

// newClassHydraScoped additionally pins the introspected scope string — the
// round-14 #1 dimension ("openid offline_access" vs one carrying the API
// scope).
func newClassHydraScoped(t *testing.T, sub, clientID, scope string, aud []string, platformClients map[string]bool) *classHydra {
	t.Helper()
	h := &classHydra{}
	for clientID, platform := range platformClients {
		if platform {
			h.platformClientIDs = append(h.platformClientIDs, clientID)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/admin/oauth2/introspect" && r.Method == http.MethodPost:
			_ = r.ParseForm()
			if r.PostFormValue("token") != testToken {
				_, _ = fmt.Fprint(w, `{"active":false}`)
				return
			}
			body := map[string]any{"active": true, "sub": sub, "client_id": clientID}
			if scope != "" {
				body["scope"] = scope
			}
			if len(aud) > 0 {
				body["aud"] = aud
			}
			_ = json.NewEncoder(w).Encode(body)
		case strings.HasPrefix(r.URL.Path, "/admin/clients/") && r.Method == http.MethodGet:
			h.clientLookups.Add(1)
			id := strings.TrimPrefix(r.URL.Path, "/admin/clients/")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client_id": id,
				"metadata":  map[string]any{"bex.co/platform-client": true},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	h.url = srv.URL
	return h
}

func authStatus(t *testing.T, h *classHydra, resource string, requireAudience bool) int {
	return authStatusScoped(t, h, resource, requireAudience, "")
}

// authStatusScoped drives the gate with the API scope dimension armed
// (round-14 #1); "" keeps the scope rule inert (the m67 expectations).
func authStatusScoped(t *testing.T, h *classHydra, resource string, requireAudience bool, apiScope string) int {
	t.Helper()
	auth := newOryAuth(h.url, "", resource, "", "", requireAudience, nil, nil, nil, apiScope)
	auth.setPlatformClientIDs(h.platformClientIDs)
	mw := auth.middleware(echoIdentity)
	r := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	r.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)
	return w.Code
}

const bexResource = "https://api.bex.co/mcp"

// TestAudienceLessHumanTokenFromThirdPartyClientIsRefused is the w1/m67 F1
// regression. The empty-audience exception exists for client_credentials API
// keys, but written as "any active token with an empty aud" it also admitted a
// HUMAN token minted for a self-registered (DCR) third-party client that never
// requested this resource — handing that client the user's full workspace rights.
func TestAudienceLessHumanTokenFromThirdPartyClientIsRefused(t *testing.T) {
	// A human subject (sub != client_id), no audience, client not provisioned by bex.
	h := newClassHydra(t, "identity-1", "dcr-client", nil, map[string]bool{"dcr-client": false})

	if got := authStatus(t, h, bexResource, true); got != http.StatusUnauthorized {
		t.Errorf("audience-less human token from a third-party client = %d, want 401", got)
	}
	if h.clientLookups.Load() != 0 {
		t.Error("attacker-writable Hydra client metadata must not be consulted")
	}
}

// The three shapes that must keep working, because each is a legitimate caller
// that carries no audience or a correct one.
func TestNarrowedAudienceRuleKeepsLegitimateCallers(t *testing.T) {
	for _, tc := range []struct {
		name          string
		sub, clientID string
		scope         string
		aud           []string
		platform      map[string]bool
	}{
		{
			// client_credentials API key: Hydra returns sub == client_id, no audience.
			name: "api key (client_credentials)",
			sub:  "key-1", clientID: "key-1",
			platform: map[string]bool{},
		},
		{
			// The official Render CLI's device flow: a human subject, no audience —
			// admitted because the operator registry names the fixed client ID.
			name: "human token from a bex-provisioned client",
			sub:  "identity-1", clientID: "render-cli",
			platform: map[string]bool{"render-cli": true},
		},
		{
			// An MCP/agent client that requested the resource AND a granular
			// capability — the ordinary compliant connect (w8/m27).
			name: "human token that carries the resource audience",
			sub:  "identity-1", clientID: "dcr-client",
			scope:    core.ScopeRead,
			aud:      []string{bexResource},
			platform: map[string]bool{"dcr-client": false},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newClassHydraScoped(t, tc.sub, tc.clientID, tc.scope, tc.aud, tc.platform)
			if got := authStatus(t, h, bexResource, true); got != http.StatusOK {
				t.Errorf("status = %d, want 200", got)
			}
			if lookups := h.clientLookups.Load(); lookups != 0 {
				t.Errorf("Hydra client metadata lookups = %d, want 0", lookups)
			}
		})
	}
}

// The rule is inert unless both a resource is configured and the operator has
// enabled it, preserving the existing rollout gate.
func TestNarrowedAudienceRuleIsOffByDefault(t *testing.T) {
	h := newClassHydra(t, "identity-1", "dcr-client", nil, map[string]bool{"dcr-client": false})

	// w8/m27: a non-platform human token without a granular capability is
	// inactive even when the empty-audience flag is off. The audience rule
	// staying inert is not an exemption from the capability vocabulary.
	if got := authStatus(t, h, bexResource, false); got != http.StatusUnauthorized {
		t.Errorf("identity-only third-party token = %d, want 401", got)
	}
}

func TestPlatformClientTrustComesOnlyFromOperatorRegistry(t *testing.T) {
	auth := newOryAuth("http://hydra.invalid", "", "", "", "", true, nil, nil, nil, "")
	auth.setPlatformClientIDs([]string{"render-cli"})

	for clientID, want := range map[string]bool{
		"render-cli": true,
		"dcr-client": false,
		"":           false,
	} {
		got, err := auth.IsPlatformClientFresh(context.Background(), clientID)
		if err != nil || got != want {
			t.Errorf("IsPlatformClientFresh(%q) = %v, %v; want %v, nil", clientID, got, err, want)
		}
	}
}

func TestOperatorRegistryIsTheMintClassAuthority(t *testing.T) {
	auth := newOryAuth("http://hydra.invalid", "", "", "", "", true, nil, nil, nil, "")
	auth.setPlatformClientIDs([]string{"render-cli"})
	base := &core.Base{PlatformClients: auth}

	for _, tc := range []struct {
		name     string
		clientID string
		want     error
	}{
		{name: "operator-listed client", clientID: "render-cli"},
		{name: "self-registered client with asserted marker", clientID: "dcr-client", want: core.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := core.WithIdentity(context.Background(), core.Identity{
				Subject: "user-a", Method: "oauth2", ClientID: tc.clientID,
				Human: true, CanonicalScopes: core.ScopeWrite,
			})
			err := base.AuthorizeMintClass(ctx)
			if !errors.Is(err, tc.want) {
				t.Fatalf("AuthorizeMintClass() = %v, want %v", err, tc.want)
			}
		})
	}
}
