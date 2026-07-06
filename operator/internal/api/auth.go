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
	"bytes"
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
	// for one token are coalesced into a single Hydra call (group), which also
	// writes the cache exactly once per upstream call.
	cache *ttlCache[Identity]
	group singleflight.Group
}

const (
	positiveTTL = 30 * time.Second // how long a positive introspection/authz answer may be reused
	cacheMax    = 4096             // hard bound; sweep expired then reset beyond it (negatives are never cached)
)

// ttlCache is the package's positive-result cache (introspections, authz
// checks): entries expire after positiveTTL, expired entries are dropped on
// lookup, and at cacheMax the expired are swept before a wholesale reset.
type ttlCache[V any] struct {
	mu sync.Mutex
	m  map[string]ttlEntry[V]
}

type ttlEntry[V any] struct {
	v       V
	expires time.Time
}

func newTTLCache[V any]() *ttlCache[V] { return &ttlCache[V]{m: map[string]ttlEntry[V]{}} }

func (c *ttlCache[V]) get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		var zero V
		return zero, false
	}
	if !time.Now().Before(e.expires) {
		delete(c.m, key) // dead entry — don't let it linger until cacheMax
		var zero V
		return zero, false
	}
	return e.v, true
}

// put caches v until the given expiry (callers may clamp below positiveTTL,
// e.g. to a token's own exp).
func (c *ttlCache[V]) put(key string, v V, expires time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= cacheMax {
		now := time.Now()
		for k, e := range c.m { // sweep the expired before nuking the live
			if !now.Before(e.expires) {
				delete(c.m, k)
			}
		}
		if len(c.m) >= cacheMax {
			c.m = map[string]ttlEntry[V]{}
		}
	}
	c.m[key] = ttlEntry[V]{v: v, expires: expires}
}

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

// httpStatusError is doJSON's unexpected-status error; callers may map
// specific codes (e.g. 404 -> ErrNotFound) via errors.As.
type httpStatusError struct {
	code    int
	summary string // "METHOD path returned 404 Not Found"
}

func (e *httpStatusError) Error() string { return e.summary }

// doJSON runs one JSON-over-HTTP call against an Ory/FGA-style API: optional
// JSON body, optional bearer, drain-close for connection reuse, unexpected
// status -> *httpStatusError, response decoded into out when non-nil. Shared
// by the Hydra admin store and the OpenFGA checker so request mechanics can't
// drift between them.
func doJSON(ctx context.Context, client *http.Client, method, endpoint, bearer string, body []byte, want int, out any) error {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, rdr)
	if err != nil {
		return err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer drainClose(resp)
	if resp.StatusCode != want {
		return &httpStatusError{code: resp.StatusCode, summary: method + " " + endpoint + " returned " + resp.Status}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func newOryAuth(hydraAdminURL, kratosURL string) *oryAuth {
	return &oryAuth{
		hydraAdminURL: strings.TrimSuffix(hydraAdminURL, "/"),
		kratosURL:     strings.TrimSuffix(kratosURL, "/"),
		client:        &http.Client{Timeout: 5 * time.Second, Transport: oryTransport},
		cache:         newTTLCache[Identity](),
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
	if id, ok := a.cache.get(token); ok {
		return id, nil
	}
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

	expires := time.Now().Add(positiveTTL)
	if exp := time.Unix(int64(out.Exp), 0); out.Exp > 0 && exp.Before(expires) {
		expires = exp
	}
	a.cache.put(token, id, expires)
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
