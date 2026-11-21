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
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"golang.org/x/time/rate"
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
	*core.KeyedRateLimiter[string]
	// Workspace resolves authenticated callers to a shared aggregate bucket.
	// When wired, a request must pass both its subject bucket and its workspace
	// bucket, so minting more API keys cannot multiply workspace throughput.
	Workspace core.WorkspaceResolver
	workspace *core.KeyedRateLimiter[string]
	// TrustedProxies, when set (BEX_TRUSTED_PROXY_CIDRS), lets the unauthenticated
	// IP fallback derive the real client IP from X-Forwarded-For/X-Real-IP when
	// the immediate peer is a trusted proxy, instead of keying every anonymous
	// Internet client into the edge proxy's one bucket (.pm/w4/029.md report
	// #10). nil ⇒ headers ignored, peer IP used — byte-identical to before.
	TrustedProxies core.TrustedProxies
	// Operations, when set, names the requests that spend their own buckets
	// instead of the general ones (Render's per-operation sandbox limits,
	// w22/m443). nil charges every request to the general budget.
	Operations OperationLimits
	// now supplies the current time; nil => time.Now.
	now func() time.Time
}

const (
	limiterIdle       = 10 * time.Minute
	limiterSweepEvery = 5 * time.Minute
)

// NewRateLimiter returns a per-caller token-bucket limiter at rpm
// requests/minute with the given burst depth. Zero or negative rpm returns nil
// (disabled). burst ≤ 0 defaults to ceil(rpm).
func NewRateLimiter(rpm float64, burst int) *RateLimiter {
	inner := core.NewKeyedRateLimiter[string](rpm, burst, limiterIdle, limiterSweepEvery)
	if inner == nil {
		return nil
	}
	return &RateLimiter{
		KeyedRateLimiter: inner,
		workspace:        core.NewKeyedRateLimiter[string](rpm, burst, limiterIdle, limiterSweepEvery),
	}
}

// OperationLimits gives some requests buckets of their own. RequestBuckets
// returns the buckets a request spends in place of the general per-subject and
// workspace ones, or nil for a request it does not classify; workspace is the
// caller's workspace bucket key, "" when there is none.
type OperationLimits interface {
	RequestBuckets(r *http.Request, workspace string) []*rate.Limiter
}

// Middleware returns an http.Handler wrapper that enforces the rate limit.
// It must run inside the auth middleware so the caller Identity is in ctx.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workspace := rl.workspaceKey(r)
		var buckets []*rate.Limiter
		if rl.Operations != nil {
			buckets = rl.Operations.RequestBuckets(r, workspace)
		}
		classified := buckets != nil
		if !classified {
			buckets = rl.generalBuckets(r, workspace)
		}
		if !rl.reserve(w, r, 1, buckets) {
			return
		}
		if !classified && rl.Operations != nil {
			// A GraphQL field or an MCP tool with a budget of its own is only
			// known once the body is read: it charges that budget and gives
			// this token back.
			r = r.WithContext(core.WithRateLimitScope(r.Context(), core.NewRateLimitScope(workspace, w.Header(), buckets)))
		}
		next.ServeHTTP(w, r)
	})
}

// admit takes n more tokens from the caller's general bucket and, when it
// has one, the caller's workspace bucket: a GraphQL batch pays one per member
// past the first, which Middleware charged. When either cannot give them now
// it takes none, answers 429 with Retry-After, and returns false. n is capped
// at the burst, the most a full bucket holds.
func (rl *RateLimiter) admit(w http.ResponseWriter, r *http.Request, n int) bool {
	return rl.reserve(w, r, n, rl.generalBuckets(r, rl.workspaceKey(r)))
}

// generalBuckets are the caller's bucket and, when workspace names one, the
// caller's workspace bucket.
func (rl *RateLimiter) generalBuckets(r *http.Request, workspace string) []*rate.Limiter {
	buckets := []*rate.Limiter{rl.Bucket(rl.callerKey(r))}
	if workspace != "" {
		buckets = append(buckets, rl.workspace.Bucket(workspace))
	}
	return buckets
}

// reserve takes n tokens from every bucket or from none, writes the
// RateLimit headers, and on a refusal answers the surface's 429.
func (rl *RateLimiter) reserve(w http.ResponseWriter, r *http.Request, n int, buckets []*rate.Limiter) bool {
	now := rl.clock()
	taken, wait := core.ReserveEach(now, n, buckets...)
	if wait > 0 {
		core.SetRateLimitRefusal(w.Header(), now, wait, buckets[:len(taken)]...)
		writeTooManyRequests(w, r)
		return false
	}
	core.SetRateLimitHeaders(w.Header(), now, false, buckets...)
	return true
}

func (rl *RateLimiter) clock() time.Time {
	if rl.now != nil {
		return rl.now()
	}
	return time.Now()
}

// RefusedCredentialMiddleware bounds, per caller, only the requests next
// refuses as a bad credential (401, 403, 409 or 410): a caller whose budget
// is spent is answered 429 before next runs, and a request next serves costs
// nothing. It is for a route whose credential is a single-use token minted
// under a limit of its own (a sandbox run or file connect token, minted under
// Render's per-user exec and file-transfer rates): the mint already bounds
// the tokens that redeem, so only the guesses, replays and expired tokens
// need a budget here. Counting every redeem instead held a user to this
// budget, far below what the mint allows (w22/m443).
func (rl *RateLimiter) RefusedCredentialMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := rl.clock()
		bucket := rl.Bucket(rl.callerKey(r))
		if tokens := bucket.TokensAt(now); tokens < 1 {
			wait := time.Duration((1 - tokens) / float64(bucket.Limit()) * float64(time.Second))
			core.SetRateLimitRefusal(w.Header(), now, wait, bucket)
			writeTooManyRequests(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		switch sw.statusCode() {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusGone:
			bucket.ReserveN(rl.clock(), 1)
		}
	})
}

func (rl *RateLimiter) workspaceKey(r *http.Request) string {
	if rl.Workspace == nil {
		return ""
	}
	id, ok := core.IdentityFrom(r.Context())
	if !ok || id.Subject == "" {
		return ""
	}
	workspace, ok := rl.Workspace.Tenant(r.Context(), id)
	if !ok || workspace == "" {
		return ""
	}
	return "workspace:" + workspace
}

// callerKey derives the rate-limit key from the request: the authenticated
// identity Subject when available, else the client IP — derived through the
// configured trusted proxies, or the raw peer IP when there are none (or the
// peer is not one), which ignores spoofed forwarding headers.
func (rl *RateLimiter) callerKey(r *http.Request) string {
	if id, ok := core.IdentityFrom(r.Context()); ok && id.Subject != "" {
		return "id:" + id.Subject
	}
	return "ip:" + rl.TrustedProxies.ClientIP(r)
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
	return withBodyLimitMessage(max, fmt.Sprintf("request body exceeds %d bytes", max))
}

func withBodyLimitMessage(max int64, tooLargeMessage string) func(http.Handler) http.Handler {
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
					core.WriteErrStatus(w, http.StatusBadRequest, "failed to read request body")
					return
				}
				if int64(len(buf)) > max {
					core.WriteErrStatus(w, http.StatusRequestEntityTooLarge, tooLargeMessage)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(buf))
			}
			next.ServeHTTP(w, r)
		})
	}
}
