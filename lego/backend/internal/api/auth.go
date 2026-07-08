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
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// auth.go is the shared auth gate every HTTP surface sits behind (docs/auth.md):
// a bearer token is introspected at Hydra's admin endpoint; otherwise an Ory
// session (cookie or X-Session-Token) is checked via Kratos' whoami. It attaches
// the resolved core.Identity to the request context, which the feature services'
// authorize gate reads. Upstream failures fail closed (401 / 503).

// bearerToken extracts the RFC 6750 credential from the Authorization header; ok
// is false when the header is absent or not "Bearer "-prefixed.
func bearerToken(r *http.Request) (string, bool) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return tok, ok && tok != ""
}

// oryAuth validates real credentials against the Ory substrate. A bearer, when
// present, is authoritative — an inactive token is rejected without falling
// through to the session.
type oryAuth struct {
	hydraAdminURL string // e.g. http://hydra-admin.auth:4445 (required)
	kratosURL     string // e.g. http://kratos-public.auth:80; empty disables sessions
	client        *http.Client

	// Audience discipline (MCP authorization spec / RFC 8707, w4/m9): when set
	// (the resource's canonical URI, e.g. https://api.bex.co/mcp), a token whose
	// introspected `aud` list is non-empty must include it, or the token is
	// rejected. Tokens with an EMPTY aud are still accepted — Hydra doesn't
	// implement RFC 8707's `resource` parameter (it has its own `audience`
	// request param), so plain API-key (client_credentials) tokens carry no
	// audience and must keep working. A documented subset, not full RFC 8707.
	resource string
	// challenge is the constant WWW-Authenticate value for 401s: bare "Bearer",
	// or — when discovery is configured — enriched with RFC 9728's
	// `resource_metadata="…"` so an MCP client can find the authorization server.
	challenge string

	// Positive introspections are cached briefly so a chatty agent doesn't cost
	// one Hydra round trip per request. Negatives are never cached. Concurrent
	// misses for one token coalesce into a single Hydra call (group), which also
	// writes the cache exactly once per upstream call.
	cache *core.TTLCache[core.Identity]
	group singleflight.Group
}

func newOryAuth(hydraAdminURL, kratosURL, resource, resourceMetadataURL string) *oryAuth {
	challenge := "Bearer"
	if resourceMetadataURL != "" {
		challenge = `Bearer resource_metadata="` + resourceMetadataURL + `"`
	}
	return &oryAuth{
		hydraAdminURL: strings.TrimSuffix(hydraAdminURL, "/"),
		kratosURL:     strings.TrimSuffix(kratosURL, "/"),
		resource:      resource,
		challenge:     challenge,
		client:        &http.Client{Timeout: 5 * time.Second, Transport: core.OryTransport},
		cache:         core.NewTTLCache[core.Identity](),
	}
}

func (a *oryAuth) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id core.Identity
		var err error
		bearer, hasBearer := bearerToken(r)
		switch {
		case hasBearer:
			id, err = a.introspect(r.Context(), bearer)
		case a.kratosURL != "" && hasSessionCredential(r):
			id, err = a.whoami(r)
		default:
			a.unauthorized(w)
			return
		}
		switch {
		case err != nil: // Ory unreachable/broken — fail closed, honestly
			http.Error(w, `{"error":"auth upstream unavailable"}`, http.StatusServiceUnavailable)
		case id == core.Identity{}:
			a.unauthorized(w)
		default:
			next.ServeHTTP(w, r.WithContext(core.WithIdentity(r.Context(), id)))
		}
	})
}

// hasSessionCredential reports whether the request carries something worth a
// Kratos round trip: the session header, or Kratos' session cookie. An unrelated
// cookie (analytics, LB affinity) must not cost an upstream call.
func hasSessionCredential(r *http.Request) bool {
	if r.Header.Get("X-Session-Token") != "" {
		return true
	}
	_, err := r.Cookie("ory_kratos_session")
	return err == nil
}

// introspect validates an OAuth2 token at Hydra's admin API. Returns the zero
// Identity for an inactive/unknown token, an error when Hydra is unreachable.
func (a *oryAuth) introspect(ctx context.Context, token string) (core.Identity, error) {
	if id, ok := a.cache.Get(token); ok {
		return id, nil
	}
	// Coalesce concurrent misses for the same token into one Hydra call.
	v, err, _ := a.group.Do(token, func() (any, error) {
		return a.introspectUpstream(ctx, token)
	})
	if err != nil {
		return core.Identity{}, err
	}
	return v.(core.Identity), nil
}

func (a *oryAuth) introspectUpstream(ctx context.Context, token string) (core.Identity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.hydraAdminURL+"/admin/oauth2/introspect",
		strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return core.Identity{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return core.Identity{}, err
	}
	defer core.DrainClose(resp)
	if resp.StatusCode != http.StatusOK {
		return core.Identity{}, core.Err("hydra introspection returned " + resp.Status)
	}
	var out struct {
		Active   bool     `json:"active"`
		Sub      string   `json:"sub"`
		ClientID string   `json:"client_id"`
		Exp      float64  `json:"exp"`
		Aud      []string `json:"aud"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return core.Identity{}, err
	}
	if !out.Active {
		return core.Identity{}, nil
	}
	// Audience discipline (see the resource field): a token minted for another
	// resource must not authorize this one. Empty aud stays accepted (API keys).
	if a.resource != "" && len(out.Aud) > 0 && !slices.Contains(out.Aud, a.resource) {
		return core.Identity{}, nil
	}
	subject := out.Sub
	if subject == "" {
		subject = out.ClientID
	}
	id := core.Identity{Subject: subject, Method: "oauth2"}

	expires := time.Now().Add(core.PositiveTTL)
	if exp := time.Unix(int64(out.Exp), 0); out.Exp > 0 && exp.Before(expires) {
		expires = exp
	}
	a.cache.Put(token, id, expires)
	return id, nil
}

// whoami validates an Ory session at Kratos' public API, forwarding the caller's
// session credential (cookie or X-Session-Token). Returns the zero Identity for a
// missing/expired session, an error when Kratos is unreachable.
func (a *oryAuth) whoami(r *http.Request) (core.Identity, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, a.kratosURL+"/sessions/whoami", nil)
	if err != nil {
		return core.Identity{}, err
	}
	if c := r.Header.Get("Cookie"); c != "" {
		req.Header.Set("Cookie", c)
	}
	if t := r.Header.Get("X-Session-Token"); t != "" {
		req.Header.Set("X-Session-Token", t)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return core.Identity{}, err
	}
	defer core.DrainClose(resp)
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return core.Identity{}, nil
	case resp.StatusCode != http.StatusOK:
		return core.Identity{}, core.Err("kratos whoami returned " + resp.Status)
	}
	var out struct {
		Identity struct {
			ID string `json:"id"`
		} `json:"identity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return core.Identity{}, err
	}
	if out.Identity.ID == "" {
		return core.Identity{}, nil
	}
	return core.Identity{Subject: out.Identity.ID, Method: "session"}, nil
}

// unauthorized answers 401 with the precomputed WWW-Authenticate challenge
// (RFC 9728 resource_metadata when discovery is configured — how an MCP client
// that hit the API unauthenticated finds the authorization server).
func (a *oryAuth) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", a.challenge)
	http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
}

// withCORS adds CORS for a comma-separated allowlist of origins and answers
// preflight. Empty origins => no CORS headers (same-origin / server-to-server).
// The matched request Origin is echoed back; Allow-Credentials is required for
// the dashboard's Kratos-session cookie to be readable cross-origin.
func withCORS(origins string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(allowed) > 0 {
			w.Header().Set("Vary", "Origin")
			if origin := r.Header.Get("Origin"); allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Session-Token")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
