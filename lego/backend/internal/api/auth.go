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
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// auth.go is the shared auth gate every HTTP surface sits behind (docs/ADR012-auth.md):
// a bearer token is introspected at Hydra's admin endpoint; otherwise an Ory
// session (cookie or X-Session-Token) is checked via Kratos' whoami. It attaches
// the resolved core.Identity to the request context, which the feature services'
// authorize gate reads. Upstream failures fail closed (401 / 503). The one
// deliberate exception is an otherwise-uncredentialed GET /v1/git/callback:
// GitHub's redirect authenticates with its short-lived HMAC-signed state query
// parameter inside the github feature handler. Exact method + path matching keeps
// that alternate credential from becoming a general auth bypass.

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
	// Issuer pinning (w6/m6): when set (Hydra's public issuer, e.g.
	// https://oauth.bex.co), a token whose introspected `iss` is non-empty must
	// match it. Defense-in-depth on top of introspecting one fixed Hydra admin
	// URL — bounds the residual risk of BEX_HYDRA_ADMIN_URL misconfiguration
	// pointing at the wrong/shared Hydra. Empty iss stays accepted (the same
	// defensive shape as the aud check: client_credentials tokens need not carry it).
	issuer string
	// challenge is the constant WWW-Authenticate value for 401s: bare "Bearer",
	// or — when discovery is configured — enriched with RFC 9728's
	// `resource_metadata="…"` so an MCP client can find the authorization server.
	challenge string

	// onboard, when set (the control-plane store is wired), mints a personal
	// tenant for a human identity on first login. nil => store off: no mint, the
	// legacy default-workspace behavior. Only session callers mint — machine
	// callers resolve via their key's tenant binding, never mint.
	onboard Onboarding

	// Positive introspections are cached briefly so a chatty agent doesn't cost
	// one Hydra round trip per request. Negatives are never cached. Concurrent
	// misses for one token coalesce into a single Hydra call (group), which also
	// writes the cache exactly once per upstream call.
	cache *core.TTLCache[core.Identity]
	group singleflight.Group
	// revocationEpoch + epochMu make an upstream introspection's cache write
	// coherent with a concurrent local invalidation WITHOUT holding any lock
	// across the Hydra round trip (w4/m31 — the RWMutex this replaced blocked
	// every new cache-miss introspection process-wide for up to the full
	// Hydra RTT whenever a logout was in flight, since sync.RWMutex admits no
	// new readers once a writer is pending).
	//
	// introspectUpstream snapshots the epoch BEFORE the RTT (lock-free — a
	// stale snapshot only ever makes the later check stricter, never looser).
	// The RTT itself runs under no lock at all. Only the tail — the epoch
	// re-check immediately before the cache Put — takes epochMu, and
	// invalidate's epoch bump + cache delete also run under epochMu: this is
	// the part that MUST be atomic, because "Load the epoch, then Put" is not
	// one operation — a bare atomic re-check with no lock leaves a window
	// where invalidate's Add()+Delete() can complete strictly between the
	// Load() and the Put(), which would let the stale result land in the
	// cache anyway (found in review; a plain atomic re-check is NOT
	// sufficient). epochMu is held only for that instant — an atomic op plus
	// a map write/delete, no I/O — so invalidate can still never be blocked
	// by an in-flight Hydra RTT (the fix's whole point): it only ever
	// contends with introspectUpstream's tail, never its network wait.
	revocationEpoch atomic.Uint64
	epochMu         sync.Mutex

	// touch records a key's last-used metadata after a successful API-key
	// introspection (w4/m13). Fire-and-forget + self-throttling on the callee
	// side, so it adds no I/O to the request path; nil disables it (no apikeys
	// feature).
	touch func(clientID string)
}

func newOryAuth(hydraAdminURL, kratosURL, resource, issuer, resourceMetadataURL string, onboard Onboarding, touch func(string)) *oryAuth {
	challenge := "Bearer"
	if resourceMetadataURL != "" {
		challenge = `Bearer resource_metadata="` + resourceMetadataURL + `"`
	}
	return &oryAuth{
		hydraAdminURL: strings.TrimSuffix(hydraAdminURL, "/"),
		kratosURL:     strings.TrimSuffix(kratosURL, "/"),
		resource:      resource,
		issuer:        issuer,
		challenge:     challenge,
		onboard:       onboard,
		client:        &http.Client{Timeout: 5 * time.Second, Transport: core.OryTransport},
		cache:         core.NewTTLCache[core.Identity](),
		touch:         touch,
	}
}

// invalidate evicts a token whose upstream state changed. A human CLI logout
// revokes the whole subject+client consent chain in Hydra, so evict every
// positively cached access token in that chain too. The official CLI refreshes
// before logout when its token expires within 24h; deleting only the bearer on
// the revoke request would leave the immediately previous token usable until
// PositiveTTL.
func (a *oryAuth) invalidate(token string, identity core.Identity) {
	// The bump must be serialized against introspectUpstream's epoch-check+Put
	// (epochMu — see its doc comment): a bare atomic Add here, with no lock,
	// would leave a window where introspectUpstream's Load()-then-Put() could
	// straddle this Add(), passing its check a moment before the bump and
	// still Putting a moment after it — resurrecting the very entry this call
	// means to revoke (found in review; this is not a hypothetical). Holding
	// epochMu only around the Add (not the deletes below, which are already
	// independently safe via the cache's own internal lock and only need to
	// run at some point after this Add, not atomically with it) keeps the
	// critical section to a single non-blocking instruction.
	a.epochMu.Lock()
	a.revocationEpoch.Add(1)
	a.epochMu.Unlock()
	a.cache.Delete(token)
	if identity.Method == "oauth2" {
		a.cache.DeleteIf(func(cached core.Identity) bool {
			if cached.Method != "oauth2" || cached.ClientID != identity.ClientID {
				return false
			}
			// A machine client owns the whole client_id, while the shared Render
			// client must evict only this human subject's consent chain.
			return !identity.Human || (cached.Human && cached.Subject == identity.Subject)
		})
	}
}

func (a *oryAuth) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id core.Identity
		var err error
		bearer, hasBearer := bearerToken(r)
		hasSession := a.kratosURL != "" && hasSessionCredential(r)
		if r.Method == http.MethodGet && r.URL.Path == "/v1/git/callback" && !hasBearer && !hasSession {
			// The github handler must verify its state credential before acting.
			// A supplied OAuth/session credential still takes the ordinary path so
			// existing API/agent callback calls retain their Identity and authz.
			next.ServeHTTP(w, r)
			return
		}
		switch {
		case hasBearer:
			id, err = a.introspect(r.Context(), bearer)
		case hasSession:
			id, err = a.whoami(r)
		default:
			a.unauthorized(w)
			return
		}
		switch {
		case err != nil: // Ory unreachable/broken — fail closed, honestly
			core.WriteErrStatus(w, http.StatusServiceUnavailable, "auth upstream unavailable")
		case id == core.Identity{}:
			a.unauthorized(w)
		default:
			// Tenant onboarding (w1/m9): a human's first authenticated call mints
			// it a personal tenant. This includes Kratos sessions and OAuth tokens
			// with a real `sub` (authorization/device flow); client_credentials API
			// keys remain machine callers and resolve through their binding. A
			// broken store fails closed (503), like a broken Ory: a
			// request that can't be tenanted must not be served un-tenanted.
			if a.onboard != nil && id.Human {
				if _, err := a.onboard.EnsureTenant(r.Context(), id.Subject, id.Email, id.EmailVerified); err != nil {
					core.WriteErrStatus(w, http.StatusServiceUnavailable, "tenant onboarding unavailable")
					return
				}
			}
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
	_, err, _ := a.group.Do(token, func() (any, error) {
		return nil, a.introspectUpstream(ctx, token)
	})
	if err != nil {
		return core.Identity{}, err
	}
	// Read the cache instead of returning singleflight's shared value directly.
	// A revocation may have completed between the upstream call and this waiter
	// resuming; in that case invalidate deleted the entry and this request must
	// observe the token as inactive.
	if id, ok := a.cache.Get(token); ok {
		return id, nil
	}
	return core.Identity{}, nil
}

// introspectUpstream performs the Hydra round trip and populates the cache;
// its only output channel is the cache — introspect re-reads it afterward so
// a concurrent revocation is always observed (see introspect's comment). The
// round trip itself runs lock-free (see revocationEpoch's doc comment); only
// the epoch snapshot at entry and the epoch check immediately before Put
// guard against a stale positive result racing a concurrent invalidate.
func (a *oryAuth) introspectUpstream(ctx context.Context, token string) error {
	epoch := a.revocationEpoch.Load()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.hydraAdminURL+"/admin/oauth2/introspect",
		strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer core.DrainClose(resp)
	if resp.StatusCode != http.StatusOK {
		return core.Err("hydra introspection returned " + resp.Status)
	}
	var out struct {
		Active   bool     `json:"active"`
		Sub      string   `json:"sub"`
		ClientID string   `json:"client_id"`
		Iss      string   `json:"iss"`
		Exp      float64  `json:"exp"`
		Aud      []string `json:"aud"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.Active {
		return nil
	}
	// Audience discipline (see the resource field): a token minted for another
	// resource must not authorize this one. Empty aud stays accepted (API keys).
	if a.resource != "" && len(out.Aud) > 0 && !slices.Contains(out.Aud, a.resource) {
		return nil
	}
	// Issuer discipline (see the issuer field): a token from a different issuer
	// must not authorize this resource. Empty iss stays accepted.
	if a.issuer != "" && out.Iss != "" && out.Iss != a.issuer {
		return nil
	}
	subject := out.Sub
	if subject == "" {
		subject = out.ClientID
	}
	// Hydra may return sub=client_id for client_credentials tokens. A human
	// authorization/device token instead carries the end-user subject, distinct
	// from the OAuth client that obtained it.
	human := out.Sub != "" && out.Sub != out.ClientID
	id := core.Identity{
		Subject:  subject,
		Method:   "oauth2",
		ClientID: out.ClientID,
		Human:    human,
	}

	// Record last-used on the key this token was minted for (w4/m13). Keyed on
	// client_id (the API key's own id for client_credentials tokens), not the
	// subject — a user token's subject is the Kratos id, not a key. The callee
	// throttles + runs async and no-ops non-api-key clients, so this is cheap and
	// only fires on cache misses (≤ once per PositiveTTL per token).
	if a.touch != nil && out.ClientID != "" {
		a.touch(out.ClientID)
	}

	expires := time.Now().Add(core.PositiveTTL)
	if exp := time.Unix(int64(out.Exp), 0); out.Exp > 0 && exp.Before(expires) {
		expires = exp
	}
	// If a revocation ran anywhere during the RTT above, this result may be
	// stale (Hydra can still answer active=true for a token whose revoke
	// hasn't finished propagating server-side) — drop it rather than risk
	// resurrecting a just-revoked credential. The token simply misses the
	// cache on the next call and gets a fresh, authoritative introspection.
	//
	// The check and the Put below MUST happen in the same epochMu critical
	// section as one unit — a bare "Load, then Put" (no lock) leaves a gap
	// where invalidate's Add() can complete strictly between them, which
	// would let this stale result land anyway (found in review). epochMu is
	// held only for this instant (an atomic load + a map write, no I/O), so
	// it can never be the thing an in-flight invalidate blocks behind.
	a.epochMu.Lock()
	defer a.epochMu.Unlock()
	if a.revocationEpoch.Load() != epoch {
		return nil
	}
	a.cache.Put(token, id, expires)
	return nil
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
			ID     string `json:"id"`
			Traits struct {
				Email string `json:"email"`
				Name  string `json:"name"`
			} `json:"traits"`
			// verifiable_addresses carries Kratos's own verified-state for each
			// address, so bex can tell a not-yet-verified trait email apart from a
			// proven one (w1/m53) rather than trusting the raw trait.
			VerifiableAddresses []struct {
				Value    string `json:"value"`
				Verified bool   `json:"verified"`
			} `json:"verifiable_addresses"`
		} `json:"identity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return core.Identity{}, err
	}
	if out.Identity.ID == "" {
		return core.Identity{}, nil
	}
	// traits.email is the standard Kratos identity schema's email field — the key
	// a pending workspace invite is redeemed against on this caller's first login.
	// traits.name is bex's optional display-name trait (w4/m25); "" when unset.
	email := strings.ToLower(out.Identity.Traits.Email)
	emailVerified := false
	for _, a := range out.Identity.VerifiableAddresses {
		if a.Verified && strings.EqualFold(a.Value, email) {
			emailVerified = true
			break
		}
	}
	return core.Identity{
		Subject:       out.Identity.ID,
		Method:        "session",
		Human:         true,
		Email:         email,
		EmailVerified: emailVerified,
		Name:          out.Identity.Traits.Name,
	}, nil
}

// unauthorized answers 401 with the precomputed WWW-Authenticate challenge
// (RFC 9728 resource_metadata when discovery is configured — how an MCP client
// that hit the API unauthenticated finds the authorization server).
func (a *oryAuth) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", a.challenge)
	core.WriteErrStatus(w, http.StatusUnauthorized, "unauthorized")
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
