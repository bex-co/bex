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
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// KeyedRateLimiter is a generic per-key token-bucket rate limiter. It is the
// shared implementation behind internal/api.RateLimiter,
// internal/cliauth.DeviceRateLimiter, and internal/deploys.DeployHookRateLimiter
// (w4/028): each of those packages previously hand-rolled the same mutex + map
// + sweep + golang.org/x/time/rate shape. The generic lives in core because all
// three already import core and have no import-cycle reason to duplicate the
// code.
//
// Keys must be comparable. Callers keep their own keying strategy (identity
// subject, IP address, SHA-256 digest of a credential) and their own response
// dialect (REST error, OAuth slow_down, deploy-hook JSON).
//
// Bounded memory (codex-security 2026-08 F1): keys on the unauthenticated path
// include an HMAC of whatever bearer the caller presented, so an anonymous
// flood of distinct tokens maps one-to-one onto map entries. The idle sweep
// alone cannot bound that — sustained traffic keeps every entry live — so
// Bucket enforces maxKeyedRateLimitEntries exactly like TTLCache enforces
// CacheMax: at the cap the idle are swept first, then the map is reset
// wholesale. An attacker who drives the limiter to the cap trades their
// budgets for a fresh burst, never for unbounded heap.
//
// NOTE: This is a single-replica implementation. In a multi-replica deployment
// each replica maintains its own per-key map, so the effective per-key budget
// is rpm×replicas. Acceptable today; a distributed counter (Redis token bucket)
// is the follow-up when bex-api scales out.
type KeyedRateLimiter[K comparable] struct {
	mu         sync.Mutex
	entries    map[K]*keyedRateLimitEntry
	rps        rate.Limit
	burst      int
	idle       time.Duration
	sweepEvery time.Duration
	lastSweep  time.Time
}

type keyedRateLimitEntry struct {
	lim  *rate.Limiter
	last time.Time
}

// maxKeyedRateLimitEntries bounds the per-key map of every KeyedRateLimiter,
// mirroring TTLCache's CacheMax. Sized so a full reset costs a legitimate
// deployment nothing (real callers number in the thousands) while an
// unauthenticated key-space flood stalls at a fixed heap cost.
const maxKeyedRateLimitEntries = 65536

// NewKeyedRateLimiter returns a per-key token-bucket limiter at rpm
// requests/minute with the given burst depth. Zero or negative rpm returns nil
// (disabled). burst ≤ 0 defaults to ceil(rpm). Idle entries are swept every
// sweepEvery after idle has elapsed.
func NewKeyedRateLimiter[K comparable](rpm float64, burst int, idle, sweepEvery time.Duration) *KeyedRateLimiter[K] {
	if rpm <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = int(math.Ceil(rpm))
	}
	return &KeyedRateLimiter[K]{
		entries:    make(map[K]*keyedRateLimitEntry),
		rps:        rate.Limit(rpm / 60),
		burst:      burst,
		idle:       idle,
		sweepEvery: sweepEvery,
	}
}

// Bucket returns (and lazily creates) the per-key limiter, sweeping idle
// entries periodically and capping the entry count to bound memory use. A nil
// limiter returns nil so callers that disable the limiter (rpm=0) can still
// call Bucket without a guard.
func (rl *KeyedRateLimiter[K]) Bucket(key K) *rate.Limiter {
	if rl == nil {
		return nil
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.lastSweep) > rl.sweepEvery {
		for k, e := range rl.entries {
			if now.Sub(e.last) > rl.idle {
				delete(rl.entries, k)
			}
		}
		rl.lastSweep = now
	}
	e, ok := rl.entries[key]
	if !ok {
		// TTLCache's CacheMax discipline: at the cap sweep the idle first, then
		// reset wholesale. A key-space flood (distinct random bearers) pays for
		// a fresh burst per reset, never for unbounded resident entries.
		if len(rl.entries) >= maxKeyedRateLimitEntries {
			for k, e := range rl.entries {
				if now.Sub(e.last) > rl.idle {
					delete(rl.entries, k)
				}
			}
			if len(rl.entries) >= maxKeyedRateLimitEntries {
				rl.entries = make(map[K]*keyedRateLimitEntry)
			}
		}
		e = &keyedRateLimitEntry{lim: rate.NewLimiter(rl.rps, rl.burst)}
		rl.entries[key] = e
	}
	e.last = now
	return e.lim
}

// Entries reports the number of live per-key buckets. Test-only observation of
// the bound enforced in Bucket.
func (rl *KeyedRateLimiter[K]) Entries() int {
	if rl == nil {
		return 0
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.entries)
}

// RateLimitUser is the bucket key of the person behind a request, for a limit
// Render applies "for your user": the subject of a session or an OAuth grant,
// or the creator of an API key, so a user's keys share that user's buckets.
// keys is the cached lookup that caps a key at its creator's role
// (Base.KeyCreators); nil, or a key with no recorded creator, leaves the key
// a caller of its own. A request with no identity is "local".
func RateLimitUser(ctx context.Context, keys KeyCreatorResolver) string {
	id, ok := IdentityFrom(ctx)
	if !ok || id.Subject == "" {
		return "local"
	}
	if !id.IsAPIKey() {
		return "user:" + id.Subject
	}
	clientID := id.KeyClientID()
	if keys != nil {
		if creator, _, err := keys.APIKeyCreator(ctx, clientID); err == nil && creator != "" {
			return "user:" + creator
		}
	}
	return "key:" + clientID
}

// SetRateLimitHeaders writes Render's RateLimit-Limit, RateLimit-Remaining and
// RateLimit-Reset for the tightest of buckets, after this request's token is
// taken (w8/147). On a token bucket the limit is its burst, remaining is the
// whole tokens left, and reset is the seconds until it is full again. A
// refused request reports remaining 0, as Render's 429 does.
func SetRateLimitHeaders(h http.Header, now time.Time, refused bool, buckets ...*rate.Limiter) {
	var tight *rate.Limiter
	var tokens float64
	for _, b := range buckets {
		if t := b.TokensAt(now); tight == nil || t < tokens {
			tight, tokens = b, t
		}
	}
	if tight == nil {
		return
	}
	limit := tight.Burst()
	remaining := max(0, int(math.Floor(tokens)))
	if refused {
		remaining = 0
	}
	reset := 0
	if perSecond := float64(tight.Limit()); perSecond > 0 && tokens < float64(limit) {
		reset = int(math.Ceil((float64(limit) - max(tokens, 0)) / perSecond))
	}
	h.Set("RateLimit-Limit", strconv.Itoa(limit))
	h.Set("RateLimit-Remaining", strconv.Itoa(remaining))
	h.Set("RateLimit-Reset", strconv.Itoa(reset))
}

// ReserveEach takes n tokens from each bucket at now, in order, or none at
// all. n is capped at each bucket's burst, the most a full bucket holds. It
// returns one reservation per bucket it reached. A wait above zero is a
// refusal: the last bucket reached could not give them, everything taken was
// given back, and wait is how long until that bucket can. Every limiter that
// charges more than one bucket for a request reserves through it (the general
// limiter's subject and workspace buckets, a sandbox operation's class and
// workspace buckets), so a refusal never spends part of a request.
func ReserveEach(now time.Time, n int, buckets ...*rate.Limiter) (taken []*rate.Reservation, wait time.Duration) {
	taken = make([]*rate.Reservation, 0, len(buckets))
	for _, bucket := range buckets {
		res := bucket.ReserveN(now, min(n, bucket.Burst()))
		taken = append(taken, res)
		if wait = res.DelayFrom(now); wait > 0 {
			for _, reservation := range taken {
				reservation.CancelAt(now)
			}
			return taken, wait
		}
	}
	return taken, 0
}

// ChargeRateLimit takes one token from every bucket at now, or none. On a
// refusal it writes the refusal's Retry-After and RateLimit headers to h (nil
// writes nothing) and returns the surface's 429 coded code: what a GraphQL
// field or an MCP tool with a budget of its own answers.
func ChargeRateLimit(h http.Header, now time.Time, code string, buckets ...*rate.Limiter) error {
	taken, wait := ReserveEach(now, 1, buckets...)
	if wait == 0 {
		return nil
	}
	seconds := SetRateLimitRefusal(h, now, wait, buckets[:len(taken)]...)
	return NewTooManyRequestsError(code, fmt.Sprintf("rate limit exceeded; retry in %d seconds", seconds),
		map[string]any{"retryAfterSeconds": seconds})
}

// SetRateLimitRefusal writes a refused request's Retry-After (whole seconds,
// at least 1) and Render's RateLimit headers for the buckets it reached, and
// returns the seconds. A nil header writes nothing.
func SetRateLimitRefusal(h http.Header, now time.Time, wait time.Duration, buckets ...*rate.Limiter) int {
	seconds := max(1, int(math.Ceil(wait.Seconds())))
	if h != nil {
		h.Set("Retry-After", strconv.Itoa(seconds))
		SetRateLimitHeaders(h, now, true, buckets...)
	}
	return seconds
}

// RateLimitScope is what the general limiter leaves in the context of a
// request it admitted, for an operation that has a budget of its own and is
// only known after the request is read (a GraphQL field, an MCP tool). Such an
// operation charges its own buckets and gives the general ones back, so it is
// counted once, as the same operation is on a REST route the limiter can
// classify by its pattern. A nil scope is a request no limiter admitted
// (a test, the stdio transport): every method is a no-op.
type RateLimitScope struct {
	workspace string
	header    http.Header
	general   []*rate.Limiter
	mu        sync.Mutex
	refunded  bool
}

// NewRateLimitScope records the caller's workspace bucket key ("" for none),
// the response header a refusal may still write to, and the general buckets
// the request took one token from.
func NewRateLimitScope(workspace string, header http.Header, general []*rate.Limiter) *RateLimitScope {
	return &RateLimitScope{workspace: workspace, header: header, general: general}
}

type rateLimitScopeKey struct{}

// WithRateLimitScope attaches scope to ctx.
func WithRateLimitScope(ctx context.Context, scope *RateLimitScope) context.Context {
	return context.WithValue(ctx, rateLimitScopeKey{}, scope)
}

// RateLimitScopeFrom returns the scope the general limiter attached, or nil.
func RateLimitScopeFrom(ctx context.Context) *RateLimitScope {
	scope, _ := ctx.Value(rateLimitScopeKey{}).(*RateLimitScope)
	return scope
}

// Workspace is the caller's workspace bucket key, "" when there is none.
func (s *RateLimitScope) Workspace() string {
	if s == nil {
		return ""
	}
	return s.workspace
}

// Header is the response header of the request, nil when there is none.
func (s *RateLimitScope) Header() http.Header {
	if s == nil {
		return nil
	}
	return s.header
}

// Another is a scope for one more unit the same request paid the same
// general buckets for: each further member of a GraphQL batch costs a token
// of its own, and gives its own back when it is a sandbox operation.
func (s *RateLimitScope) Another() *RateLimitScope {
	if s == nil {
		return nil
	}
	return NewRateLimitScope(s.workspace, s.header, s.general)
}

// Refund gives the scope's general tokens back at now, once. A reservation
// cannot do it: Reservation.Cancel restores nothing once the reservation's
// time to act has passed, which for an admitted request is the instant it was
// taken. Reserving minus one token is the limiter's own arithmetic run
// backwards, and a bucket never holds more than its burst.
func (s *RateLimitScope) Refund(now time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refunded {
		return
	}
	s.refunded = true
	for _, bucket := range s.general {
		bucket.ReserveN(now, -1)
	}
}
