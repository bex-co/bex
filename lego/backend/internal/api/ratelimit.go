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
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// RateLimiter is a per-caller token-bucket limiter shared across all three API
// surfaces (REST, GraphQL, MCP). Each caller gets its own bucket; idle buckets
// are evicted after limiterIdle to prevent unbounded map growth.
//
// Callers are keyed by their authenticated identity Subject (populated by the
// auth middleware), falling back to remote IP for unauthenticated paths (webhooks
// run outside the auth gate and are exempt by design).
//
// NOTE: This is a single-replica implementation. In a multi-replica deployment
// each replica maintains its own per-caller map, so the effective per-caller
// budget is rpm×replicas. Acceptable today; a distributed counter (Redis token
// bucket) is the follow-up when bex-api scales out.
type RateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*limEntry
	rps       rate.Limit
	burst     int
	lastSweep time.Time
}

type limEntry struct {
	lim  *rate.Limiter
	last time.Time
}

const (
	limiterIdle       = 10 * time.Minute
	limiterSweepEvery = 5 * time.Minute
)

// NewRateLimiter returns a per-caller token-bucket limiter at rpm
// requests/minute with the given burst depth. Zero or negative rpm returns nil
// (disabled). burst ≤ 0 defaults to ceil(rpm).
func NewRateLimiter(rpm float64, burst int) *RateLimiter {
	if rpm <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = int(math.Ceil(rpm))
	}
	return &RateLimiter{
		entries: make(map[string]*limEntry),
		rps:     rate.Limit(rpm / 60),
		burst:   burst,
	}
}

// Middleware returns an http.Handler wrapper that enforces the rate limit.
// It must run inside the auth middleware so the caller Identity is in ctx.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := callerKey(r)
		lim := rl.bucket(key)
		res := lim.Reserve()
		if d := res.Delay(); d > 0 {
			res.Cancel()
			retryAfter := int(math.Ceil(d.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			writeTooManyRequests(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bucket returns (and lazily creates) the per-caller limiter, sweeping idle
// entries periodically to bound memory use.
func (rl *RateLimiter) bucket(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.lastSweep) > limiterSweepEvery {
		for k, e := range rl.entries {
			if now.Sub(e.last) > limiterIdle {
				delete(rl.entries, k)
			}
		}
		rl.lastSweep = now
	}
	e, ok := rl.entries[key]
	if !ok {
		e = &limEntry{lim: rate.NewLimiter(rl.rps, rl.burst)}
		rl.entries[key] = e
	}
	e.last = now
	return e.lim
}

// callerKey derives the rate-limit key from the request: the authenticated
// identity Subject when available, else the remote IP.
func callerKey(r *http.Request) string {
	if id, ok := core.IdentityFrom(r.Context()); ok && id.Subject != "" {
		return "id:" + id.Subject
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	return "ip:" + ip
}

// writeTooManyRequests writes a Render-shaped 429 for the request's surface:
// a GraphQL error envelope for POST /graphql, the standard Render error body
// (REST + MCP both speak plain JSON 429).
func writeTooManyRequests(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/graphql" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": nil,
			"errors": []map[string]any{{
				"message":    "rate limit exceeded",
				"extensions": map[string]string{"code": "RATE_LIMITED"},
			}},
		})
		return
	}
	core.WriteJSON(w, http.StatusTooManyRequests, map[string]any{
		"id":      "rate_limited",
		"message": "rate limit exceeded; see Retry-After header",
	})
}

// withBodyLimit returns a middleware that rejects non-GET requests whose body
// exceeds max bytes with 413. Zero or negative max disables the check.
func withBodyLimit(max int64) func(http.Handler) http.Handler {
	if max <= 0 {
		return func(h http.Handler) http.Handler { return h }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Body != nil {
				// Read at most max+1 bytes; if we get max+1 the body is too large.
				buf, err := io.ReadAll(io.LimitReader(r.Body, max+1))
				_ = r.Body.Close()
				if err != nil {
					core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
					return
				}
				if int64(len(buf)) > max {
					core.WriteJSON(w, http.StatusRequestEntityTooLarge,
						map[string]string{"error": fmt.Sprintf("request body exceeds %d bytes", max)})
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(buf))
			}
			next.ServeHTTP(w, r)
		})
	}
}
