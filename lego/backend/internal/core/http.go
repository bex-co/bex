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

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

// WriteJSON writes body as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteErr maps a domain error sentinel onto its HTTP status and writes a
// {"error": msg} body. One mapper for every feature's REST fragment, so the
// surfaces answer identically.
func WriteErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, ErrLogsUnavailable), errors.Is(err, ErrLogStoreUnavailable),
		errors.Is(err, ErrAPIKeysUnavailable),
		errors.Is(err, ErrMetricsUnavailable), errors.Is(err, ErrAuthzUnavailable),
		errors.Is(err, ErrSecretsUnavailable), errors.Is(err, ErrWorkspacesUnavailable),
		errors.Is(err, ErrUsageUnavailable), errors.Is(err, ErrDeploysUnavailable),
		errors.Is(err, ErrAuditUnavailable), errors.Is(err, ErrGitHubUnavailable),
		errors.Is(err, ErrEventsUnavailable), errors.Is(err, ErrRegistryCredentialsUnavailable):
		code = http.StatusServiceUnavailable
	case errors.Is(err, ErrBadRequest):
		code = http.StatusBadRequest
	case errors.Is(err, ErrForbidden):
		code = http.StatusForbidden
	case errors.Is(err, ErrConflict):
		code = http.StatusConflict
	}
	var ce *CodedError
	if errors.As(err, &ce) {
		WriteJSON(w, code, map[string]any{"error": err.Error(), "code": ce.Code, "params": ce.Params})
		return
	}
	WriteJSON(w, code, map[string]string{"error": err.Error()})
}

const (
	// PositiveTTL is how long a positive introspection/authz answer may be reused.
	PositiveTTL = 30 * time.Second
	// CacheMax bounds a TTLCache; the expired are swept before a wholesale reset.
	CacheMax = 4096
)

// OryTransport is the one connection pool shared by every Ory/FGA-bound client
// (introspection, whoami, key management, authz checks): the traffic all targets
// one or two hosts, where the default transport's 2 idle conns would re-dial.
var OryTransport = func() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = 32
	return tr
}()

// DrainClose drains and closes a response body so its connection returns to the
// pool for reuse.
func DrainClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// HTTPStatusError is DoJSON's unexpected-status error; callers may map specific
// codes (e.g. 404 -> ErrNotFound) via errors.As.
type HTTPStatusError struct {
	Code    int
	Summary string // "METHOD path returned 404 Not Found"
}

func (e *HTTPStatusError) Error() string { return e.Summary }

// DoJSON runs one JSON-over-HTTP call against an Ory/FGA-style API: optional JSON
// body, optional bearer, drain-close for connection reuse, unexpected status ->
// *HTTPStatusError, response decoded into out when non-nil. Shared by the Hydra
// admin store and the OpenFGA checker so request mechanics can't drift.
func DoJSON(ctx context.Context, client *http.Client, method, endpoint, bearer string, body []byte, want int, out any) error {
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
	defer DrainClose(resp)
	if resp.StatusCode != want {
		return &HTTPStatusError{Code: resp.StatusCode, Summary: method + " " + endpoint + " returned " + resp.Status}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// TTLCache is the positive-result cache the auth gate and OpenFGA checker share:
// entries expire after their expiry, expired entries are dropped on lookup, and
// at CacheMax the expired are swept before a wholesale reset.
type TTLCache[V any] struct {
	mu sync.Mutex
	m  map[string]ttlEntry[V]
}

type ttlEntry[V any] struct {
	v       V
	expires time.Time
}

// NewTTLCache returns an empty TTLCache.
func NewTTLCache[V any]() *TTLCache[V] { return &TTLCache[V]{m: map[string]ttlEntry[V]{}} }

// Get returns the live value for key, or false if absent/expired.
func (c *TTLCache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		var zero V
		return zero, false
	}
	if !time.Now().Before(e.expires) {
		delete(c.m, key) // dead entry — don't let it linger until CacheMax
		var zero V
		return zero, false
	}
	return e.v, true
}

// Delete evicts key immediately, regardless of its expiry — the counterpart to
// Put for a cached positive answer that must not survive a state change no
// timer alone can observe (e.g. the resource the key resolves to is deleted
// mid-TTL). A no-op if key isn't cached.
func (c *TTLCache[V]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

// Put caches v until the given expiry (callers may clamp below PositiveTTL, e.g.
// to a token's own exp).
func (c *TTLCache[V]) Put(key string, v V, expires time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= CacheMax {
		now := time.Now()
		for k, e := range c.m { // sweep the expired before nuking the live
			if !now.Before(e.expires) {
				delete(c.m, k)
			}
		}
		if len(c.m) >= CacheMax {
			c.m = map[string]ttlEntry[V]{}
		}
	}
	c.m[key] = ttlEntry[V]{v: v, expires: expires}
}
