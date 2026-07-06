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
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// bearerToken extracts the RFC 6750 credential from the Authorization header;
// ok is false when the header is absent or not "Bearer "-prefixed.
func bearerToken(r *http.Request) (string, bool) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return tok, ok && tok != ""
}

// Identity is the authenticated caller: an OAuth2 client (API key) validated
// by Hydra introspection or a Kratos identity validated by its session.
// Subject is Hydra's client_id/sub or the Kratos identity id — the future
// tenant-scoping hook.
type Identity struct {
	Subject string
	Method  string // "oauth2" | "session"
}

type ctxKey struct{}

// IdentityFrom returns the authenticated Identity the auth middleware attached
// to the request context.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// oryAuth validates real credentials against the Ory substrate (docs/auth.md):
// a bearer token is introspected at Hydra's admin endpoint; otherwise an Ory
// session (cookie or X-Session-Token) is checked via Kratos' whoami. A bearer,
// when present, is authoritative — an inactive token is rejected without
// falling through to the session. Upstream failures reject the request
// (fail closed): 401 for bad credentials, 503 when Ory itself is unreachable.
type oryAuth struct {
	hydraAdminURL string // e.g. http://hydra-admin.auth:4445 (required)
	kratosURL     string // e.g. http://kratos-public.auth:80; empty disables sessions
	client        *http.Client

	// Positive introspections are cached briefly so a chatty agent doesn't cost
	// one Hydra round trip per request. Negatives are never cached (a token can
	// become valid; a revoked one must die within the TTL). Concurrent misses
	// for one token are coalesced into a single Hydra call (group).
	mu    sync.Mutex
	cache map[string]cacheEntry
	group singleflight.Group
}

type cacheEntry struct {
	id      Identity
	expires time.Time
}

const (
	introspectTTL = 30 * time.Second
	cacheMax      = 4096 // hard bound; wholesale reset beyond it (garbage tokens are never cached)
)

// oryTransport is the one connection pool shared by every Ory-bound client
// (introspection, whoami, key management): the traffic all targets one or two
// hosts, where the default transport's 2 idle conns would re-dial constantly.
var oryTransport = func() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = 32
	return tr
}()

// drainClose drains and closes a response body so its connection returns to
// the pool for reuse.
func drainClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func newOryAuth(hydraAdminURL, kratosURL string) *oryAuth {
	return &oryAuth{
		hydraAdminURL: strings.TrimSuffix(hydraAdminURL, "/"),
		kratosURL:     strings.TrimSuffix(kratosURL, "/"),
		client:        &http.Client{Timeout: 5 * time.Second, Transport: oryTransport},
		cache:         map[string]cacheEntry{},
	}
}

func (a *oryAuth) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id Identity
		var err error
		bearer, hasBearer := bearerToken(r)
		switch {
		case hasBearer:
			id, err = a.introspect(r.Context(), bearer)
		case a.kratosURL != "" && hasSessionCredential(r):
			id, err = a.whoami(r)
		default:
			unauthorized(w)
			return
		}
		switch {
		case err != nil: // Ory unreachable/broken — fail closed, honestly
			http.Error(w, `{"error":"auth upstream unavailable"}`, http.StatusServiceUnavailable)
		case id == Identity{}:
			unauthorized(w)
		default:
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
		}
	})
}

// hasSessionCredential reports whether the request carries something worth a
// Kratos round trip: the session header, or Kratos' session cookie. An
// unrelated cookie (analytics, LB affinity) must not cost an upstream call.
func hasSessionCredential(r *http.Request) bool {
	if r.Header.Get("X-Session-Token") != "" {
		return true
	}
	_, err := r.Cookie("ory_kratos_session")
	return err == nil
}

// introspect validates an OAuth2 token at Hydra's admin API. Returns the zero
// Identity for an inactive/unknown token, an error when Hydra is unreachable.
func (a *oryAuth) introspect(ctx context.Context, token string) (Identity, error) {
	a.mu.Lock()
	if e, ok := a.cache[token]; ok {
		if time.Now().Before(e.expires) {
			a.mu.Unlock()
			return e.id, nil
		}
		delete(a.cache, token) // dead entry — don't let it linger until cacheMax
	}
	a.mu.Unlock()

	// Coalesce concurrent misses for the same token into one Hydra call.
	v, err, _ := a.group.Do(token, func() (any, error) {
		return a.introspectUpstream(ctx, token)
	})
	if err != nil {
		return Identity{}, err
	}
	return v.(Identity), nil
}

func (a *oryAuth) introspectUpstream(ctx context.Context, token string) (Identity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.hydraAdminURL+"/admin/oauth2/introspect",
		strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return Identity{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer drainClose(resp)
	if resp.StatusCode != http.StatusOK {
		return Identity{}, apiError("hydra introspection returned " + resp.Status)
	}
	var out struct {
		Active   bool    `json:"active"`
		Sub      string  `json:"sub"`
		ClientID string  `json:"client_id"`
		Exp      float64 `json:"exp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Identity{}, err
	}
	if !out.Active {
		return Identity{}, nil
	}
	subject := out.Sub
	if subject == "" {
		subject = out.ClientID
	}
	id := Identity{Subject: subject, Method: "oauth2"}

	expires := time.Now().Add(introspectTTL)
	if exp := time.Unix(int64(out.Exp), 0); out.Exp > 0 && exp.Before(expires) {
		expires = exp
	}
	a.mu.Lock()
	if len(a.cache) >= cacheMax {
		now := time.Now()
		for k, e := range a.cache { // sweep the expired before nuking the live
			if !now.Before(e.expires) {
				delete(a.cache, k)
			}
		}
		if len(a.cache) >= cacheMax {
			a.cache = map[string]cacheEntry{}
		}
	}
	a.cache[token] = cacheEntry{id: id, expires: expires}
	a.mu.Unlock()
	return id, nil
}

// whoami validates an Ory session at Kratos' public API, forwarding the
// caller's session credential (cookie or X-Session-Token). Returns the zero
// Identity for a missing/expired session, an error when Kratos is unreachable.
func (a *oryAuth) whoami(r *http.Request) (Identity, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, a.kratosURL+"/sessions/whoami", nil)
	if err != nil {
		return Identity{}, err
	}
	if c := r.Header.Get("Cookie"); c != "" {
		req.Header.Set("Cookie", c)
	}
	if t := r.Header.Get("X-Session-Token"); t != "" {
		req.Header.Set("X-Session-Token", t)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer drainClose(resp)
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return Identity{}, nil
	case resp.StatusCode != http.StatusOK:
		return Identity{}, apiError("kratos whoami returned " + resp.Status)
	}
	var out struct {
		Identity struct {
			ID string `json:"id"`
		} `json:"identity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Identity{}, err
	}
	if out.Identity.ID == "" {
		return Identity{}, nil
	}
	return Identity{Subject: out.Identity.ID, Method: "session"}, nil
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
}

// withCORS adds permissive-for-one-origin CORS when origin is set, and answers
// preflight. Empty origin => no CORS headers (same-origin / server-to-server).
func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
