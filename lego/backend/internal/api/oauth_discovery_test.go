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
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestAudienceValidation(t *testing.T) {
	const resource = "https://api.bex.co/mcp"
	cases := []struct {
		name     string
		aud      []string
		resource string
		want     int
	}{
		{"aud-includes-resource-accepted", []string{"https://other.example", resource}, resource, 200},
		{"aud-mismatch-rejected", []string{"https://someone-else.example"}, resource, 401},
		// Hydra has no RFC 8707 `resource` param; client_credentials API-key
		// tokens carry no audience and must keep working (documented subset).
		{"empty-aud-accepted", nil, resource, 200},
		{"no-resource-configured-no-check", []string{"https://someone-else.example"}, "", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			hydra := fakeHydra(t, &hits, tc.aud...)
			mw := newOryAuth(hydra.URL, "", tc.resource, "", nil, nil).middleware(echoIdentity)
			if w := do(t, mw, http.MethodGet, "/mcp", testToken, ""); w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestWWWAuthenticateResourceMetadata(t *testing.T) {
	t.Run("enriched-when-configured", func(t *testing.T) {
		mw := newOryAuth(fakeHydraURL(t), "", "https://api.bex.co/mcp",
			"https://api.bex.co/.well-known/oauth-protected-resource", nil, nil).middleware(echoIdentity)
		w := do(t, mw, http.MethodGet, "/mcp", "", "") // no credential
		if w.Code != 401 {
			t.Fatalf("status = %d, want 401", w.Code)
		}
		got := w.Header().Get("WWW-Authenticate")
		want := `Bearer resource_metadata="https://api.bex.co/.well-known/oauth-protected-resource"`
		if got != want {
			t.Fatalf("WWW-Authenticate = %q, want %q", got, want)
		}
	})

	t.Run("bare-when-unconfigured", func(t *testing.T) {
		mw := newOryAuth(fakeHydraURL(t), "", "", "", nil, nil).middleware(echoIdentity)
		w := do(t, mw, http.MethodGet, "/mcp", "", "")
		if got := w.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Fatalf("WWW-Authenticate = %q, want bare Bearer (unchanged behavior)", got)
		}
	})
}

func TestProtectedResourceMetadataEndpoint(t *testing.T) {
	newHandler := func(issuer, resource string) http.Handler {
		t.Helper()
		srv := NewServer(nil, Deps{})
		srv.HydraAdminURL = fakeHydraURL(t)
		srv.OAuthIssuer = issuer
		srv.OAuthResource = resource
		h, err := srv.Handler()
		if err != nil {
			t.Fatalf("Handler: %v", err)
		}
		return h
	}

	t.Run("serves-metadata-when-configured", func(t *testing.T) {
		h := newHandler("https://oauth.bex.co", "https://api.bex.co/mcp")
		w := do(t, h, http.MethodGet, "/.well-known/oauth-protected-resource", "", "")
		if w.Code != 200 {
			t.Fatalf("status = %d, want 200 (open, no auth)", w.Code)
		}
		var body struct {
			Resource             string   `json:"resource"`
			AuthorizationServers []string `json:"authorization_servers"`
		}
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Resource != "https://api.bex.co/mcp" {
			t.Fatalf("resource = %q", body.Resource)
		}
		if len(body.AuthorizationServers) != 1 || body.AuthorizationServers[0] != "https://oauth.bex.co" {
			t.Fatalf("authorization_servers = %v", body.AuthorizationServers)
		}
	})

	t.Run("absent-when-unconfigured", func(t *testing.T) {
		h := newHandler("", "")
		w := do(t, h, http.MethodGet, "/.well-known/oauth-protected-resource", "", "")
		if w.Code != 404 {
			t.Fatalf("status = %d, want 404 (byte-identical prior behavior)", w.Code)
		}
	})
}

func TestResourceMetadataURLDerivation(t *testing.T) {
	cases := []struct {
		issuer, resource, want string
	}{
		{"https://oauth.bex.co", "https://api.bex.co/mcp", "https://api.bex.co/.well-known/oauth-protected-resource"},
		{"https://oauth.bex.co", "http://localhost:8090/mcp", "http://localhost:8090/.well-known/oauth-protected-resource"},
		{"", "https://api.bex.co/mcp", ""},   // discovery off
		{"https://oauth.bex.co", "", ""},     // discovery off
		{"https://oauth.bex.co", ":bad", ""}, // unparseable resource — no endpoint, no hint
	}
	for _, tc := range cases {
		s := &Server{OAuthIssuer: tc.issuer, OAuthResource: tc.resource}
		if got := s.resourceMetadataURL(); got != tc.want {
			t.Fatalf("resourceMetadataURL(%q, %q) = %q, want %q", tc.issuer, tc.resource, got, tc.want)
		}
	}
}
