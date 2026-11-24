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
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// ErrLogSubscriptionLimit is returned when acquiring a log subscription slot
// would exceed the global, per-subject, or per-workspace cap.
var ErrLogSubscriptionLimit = errors.New("log subscription capacity reached")

const (
	// rateLimitStoreTimeout bounds every shared-store round trip so a stalled
	// control-plane Postgres cannot hang HTTP requests past edge deadlines.
	rateLimitStoreTimeout = 2 * time.Second
	// rateLimitWarnInterval rate-limits fallback warnings during a DB outage.
	rateLimitWarnInterval = 30 * time.Second
	// rateLimitFloatEpsilon absorbs IEEE-754 rounding noise (e.g. 6 * (10/60)
	// producing 0.9999999999999999 instead of 1.0) without relaxing any real
	// rate limit (1e-9 tokens is sub-microsecond at every configured rate).
	rateLimitFloatEpsilon = 1e-9
)

var lastRateLimitWarn atomic.Int64

func warnRateLimitStoreFallback(op string, err error) {
	now := time.Now().UnixNano()
	prev := lastRateLimitWarn.Load()
	if now-prev >= int64(rateLimitWarnInterval) && lastRateLimitWarn.CompareAndSwap(prev, now) {
		log.Printf("bex-api: shared rate-limit store %s failed (falling back to local buckets): %v", op, err)
	}
}

// RateLimitBucketSpec describes one token bucket in a shared RateLimitStore.
type RateLimitBucketSpec struct {
	Key   string
	RPS   float64
	Burst int
	Idle  time.Duration
}

// RateLimitBucketState is a bucket's snapshot for header rendering
// (RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset).
type RateLimitBucketState struct {
	Tokens float64
	RPS    float64
	Burst  int
}

// RateLimitStore backs KeyedRateLimiter across bex-api replicas through
// control-plane Postgres (w8/m579). Implemented by *store.PGStore.
type RateLimitStore interface {
	TakeRateLimitBuckets(ctx context.Context, now time.Time, n int, specs []RateLimitBucketSpec) (states []RateLimitBucketState, wait time.Duration, err error)
	RefundRateLimitBuckets(ctx context.Context, now time.Time, n int, specs []RateLimitBucketSpec) error
	PeekRateLimitBucket(ctx context.Context, now time.Time, spec RateLimitBucketSpec) (RateLimitBucketState, error)
}

// RateLimitBucket identifies one token bucket: an in-process *rate.Limiter
// (resolved lazily when a shared store is attached, so the shared happy path
// takes no local map lock) and, when backed by a shared store, the store and
// globally namespaced bucket spec.
type RateLimitBucket struct {
	local func() *rate.Limiter
	Store RateLimitStore
	Spec  RateLimitBucketSpec
}

// LocalRateLimitBucket wraps an in-process *rate.Limiter as a RateLimitBucket.
func LocalRateLimitBucket(lim *rate.Limiter) RateLimitBucket {
	if lim == nil {
		return RateLimitBucket{}
	}
	return RateLimitBucket{
		local: func() *rate.Limiter { return lim },
		Spec: RateLimitBucketSpec{
			RPS:   float64(lim.Limit()),
			Burst: lim.Burst(),
		},
	}
}

// IsZero reports whether the bucket handle refers to no limiter.
func (b RateLimitBucket) IsZero() bool {
	return b.local == nil && b.Store == nil
}

// Burst reports the bucket's maximum burst depth.
func (b RateLimitBucket) Burst() int {
	return b.Spec.Burst
}

func (b RateLimitBucket) localLimiter() *rate.Limiter {
	if b.local == nil {
		return nil
	}
	return b.local()
}

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
// When SetSharedStore is configured (w8/m579), Handle returns buckets backed
// by the shared control-plane Postgres store across replicas, falling back
// fail-open to the local in-process bucket on any store error.
type KeyedRateLimiter[K comparable] struct {
	mu         sync.Mutex
	entries    map[K]*keyedRateLimitEntry
	rps        rate.Limit
	burst      int
	idle       time.Duration
	sweepEvery time.Duration
	lastSweep  time.Time

	store     RateLimitStore
	namespace string
	keyFormat func(K) string
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

// SetSharedStore attaches a cross-replica RateLimitStore under a unique
// namespace prefix (e.g. "general:caller", "render:read"). keyFormat converts
// non-string keys to a canonical string; nil formats via fmt.Sprint.
func (rl *KeyedRateLimiter[K]) SetSharedStore(store RateLimitStore, namespace string, keyFormat func(K) string) {
	if rl == nil {
		return
	}
	rl.mu.Lock()
	rl.store = store
	rl.namespace = namespace
	rl.keyFormat = keyFormat
	rl.mu.Unlock()
}

// Handle returns a RateLimitBucket for key, backed by the shared store when
// configured and always able to fall back to the local in-process bucket.
func (rl *KeyedRateLimiter[K]) Handle(key K) RateLimitBucket {
	if rl == nil {
		return RateLimitBucket{}
	}
	rl.mu.Lock()
	st := rl.store
	ns := rl.namespace
	format := rl.keyFormat
	rl.mu.Unlock()

	spec := RateLimitBucketSpec{
		RPS:   float64(rl.rps),
		Burst: rl.burst,
		Idle:  rl.idle,
	}
	if st != nil {
		var raw string
		if format != nil {
			raw = format(key)
		} else {
			raw = fmt.Sprint(key)
		}
		if ns != "" {
			spec.Key = ns + ":" + raw
		} else {
			spec.Key = raw
		}
	}
	return RateLimitBucket{
		local: func() *rate.Limiter { return rl.Bucket(key) },
		Store: st,
		Spec:  spec,
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

// sharedRateLimitStore returns the single non-nil RateLimitStore shared by
// every bucket in buckets, or nil if any bucket is local-only or stores differ.
func sharedRateLimitStore(buckets []RateLimitBucket) RateLimitStore {
	if len(buckets) == 0 {
		return nil
	}
	st := buckets[0].Store
	if st == nil {
		return nil
	}
	for _, b := range buckets[1:] {
		if b.Store != st {
			return nil
		}
	}
	return st
}

// TakeRateLimitBuckets atomically takes n tokens from every bucket at now, or
// none at all, returning each bucket's post-operation state, the wait duration
// on refusal (0 when admitted), and whether the shared store handled the call.
// If the shared store fails, it logs a rate-limited warning and falls back
// fail-open to the local in-process buckets.
func TakeRateLimitBuckets(ctx context.Context, now time.Time, n int, buckets ...RateLimitBucket) (states []RateLimitBucketState, wait time.Duration, usedStore bool) {
	if len(buckets) == 0 {
		return nil, 0, false
	}
	if st := sharedRateLimitStore(buckets); st != nil {
		specs := make([]RateLimitBucketSpec, len(buckets))
		for i, b := range buckets {
			specs[i] = b.Spec
		}
		storeCtx, cancel := context.WithTimeout(ctx, rateLimitStoreTimeout)
		stStates, stWait, err := st.TakeRateLimitBuckets(storeCtx, now, n, specs)
		cancel()
		if err == nil {
			return stStates, stWait, true
		}
		warnRateLimitStoreFallback("take", err)
	}

	lims := make([]*rate.Limiter, 0, len(buckets))
	for _, b := range buckets {
		if lim := b.localLimiter(); lim != nil {
			lims = append(lims, lim)
		}
	}
	if len(lims) == 0 {
		return nil, 0, false
	}
	taken, localWait := ReserveEach(now, n, lims...)
	report := lims
	if localWait > 0 {
		report = lims[:len(taken)]
	}
	return localBucketStates(now, report), localWait, false
}

// PeekRateLimitBucket returns the bucket's current refilled state without
// deducting tokens, falling back fail-open to the local bucket on store error.
func PeekRateLimitBucket(ctx context.Context, now time.Time, bucket RateLimitBucket) RateLimitBucketState {
	if bucket.Store != nil {
		storeCtx, cancel := context.WithTimeout(ctx, rateLimitStoreTimeout)
		st, err := bucket.Store.PeekRateLimitBucket(storeCtx, now, bucket.Spec)
		cancel()
		if err == nil {
			return st
		}
		warnRateLimitStoreFallback("peek", err)
	}
	lim := bucket.localLimiter()
	if lim == nil {
		return RateLimitBucketState{}
	}
	return RateLimitBucketState{
		Tokens: lim.TokensAt(now),
		RPS:    float64(lim.Limit()),
		Burst:  lim.Burst(),
	}
}

func localBucketStates(now time.Time, buckets []*rate.Limiter) []RateLimitBucketState {
	states := make([]RateLimitBucketState, len(buckets))
	for i, b := range buckets {
		states[i] = RateLimitBucketState{
			Tokens: b.TokensAt(now),
			RPS:    float64(b.Limit()),
			Burst:  b.Burst(),
		}
	}
	return states
}

// SetRateLimitHeaders writes Render's RateLimit-Limit, RateLimit-Remaining and
// RateLimit-Reset for the tightest of states, after this request's token is
// taken (w8/147). On a token bucket the limit is its burst, remaining is the
// whole tokens left, and reset is the seconds until it is full again. A
// refused request reports remaining 0, as Render's 429 does.
func SetRateLimitHeaders(h http.Header, refused bool, states ...RateLimitBucketState) {
	if h == nil || len(states) == 0 {
		return
	}
	tight := states[0]
	for _, s := range states[1:] {
		if s.Tokens < tight.Tokens {
			tight = s
		}
	}
	if tight.Burst <= 0 {
		return
	}
	limit := tight.Burst
	remaining := max(0, int(math.Floor(tight.Tokens+rateLimitFloatEpsilon)))
	if refused {
		remaining = 0
	}
	reset := 0
	if tight.RPS > 0 && tight.Tokens+rateLimitFloatEpsilon < float64(limit) {
		reset = max(0, int(math.Ceil((float64(limit)-max(tight.Tokens, 0))/tight.RPS-rateLimitFloatEpsilon)))
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
// field or an MCP tool with a budget of its own answers. On admission it
// returns the post-take bucket states so callers can refresh response headers.
func ChargeRateLimit(ctx context.Context, h http.Header, now time.Time, code string, buckets ...RateLimitBucket) ([]RateLimitBucketState, error) {
	states, wait, _ := TakeRateLimitBuckets(ctx, now, 1, buckets...)
	if wait == 0 {
		return states, nil
	}
	seconds := SetRateLimitRefusal(h, wait, states...)
	return nil, NewTooManyRequestsError(code, RateLimitExceededMessage(seconds),
		map[string]any{"retryAfterSeconds": seconds})
}

// RateLimitExceededMessage is a rate-limit refusal's message on every surface:
// "rate limit exceeded; retry in 3 seconds". It names the wait, the seconds
// Retry-After carries, because a reader of the message may see no header: the
// Render CLI prints a 429's message since v2.38.0 (w22/m604).
func RateLimitExceededMessage(retryAfterSeconds int) string {
	if retryAfterSeconds == 1 {
		return "rate limit exceeded; retry in 1 second"
	}
	return fmt.Sprintf("rate limit exceeded; retry in %d seconds", retryAfterSeconds)
}

// SetRateLimitRefusal writes a refused request's Retry-After (whole seconds,
// at least 1) and Render's RateLimit headers for the buckets it reached, and
// returns the seconds. A nil header writes nothing.
func SetRateLimitRefusal(h http.Header, wait time.Duration, states ...RateLimitBucketState) int {
	seconds := max(1, int(math.Ceil(wait.Seconds()-rateLimitFloatEpsilon)))
	if h != nil {
		h.Set("Retry-After", strconv.Itoa(seconds))
		SetRateLimitHeaders(h, true, states...)
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
	general   []RateLimitBucket
	usedStore bool
	mu        sync.Mutex
	refunded  bool
}

// NewRateLimitScope records the caller's workspace bucket key ("" for none),
// the response header a refusal may still write to, the general buckets the
// request took one token from, and whether the shared store handled the take.
func NewRateLimitScope(workspace string, header http.Header, general []RateLimitBucket, usedStore bool) *RateLimitScope {
	return &RateLimitScope{workspace: workspace, header: header, general: general, usedStore: usedStore}
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
func (s *RateLimitScope) Another(usedStore bool) *RateLimitScope {
	if s == nil {
		return nil
	}
	return NewRateLimitScope(s.workspace, s.header, s.general, usedStore)
}

// Refund gives the scope's general tokens back at now, once, in the same store
// (shared Postgres or local in-process) that originally charged them.
func (s *RateLimitScope) Refund(ctx context.Context, now time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.refunded {
		s.mu.Unlock()
		return
	}
	s.refunded = true
	s.mu.Unlock()

	if s.usedStore {
		if st := sharedRateLimitStore(s.general); st != nil {
			specs := make([]RateLimitBucketSpec, len(s.general))
			for i, b := range s.general {
				specs[i] = b.Spec
			}
			base := ctx
			if base == nil {
				base = context.Background()
			}
			storeCtx, cancel := context.WithTimeout(context.WithoutCancel(base), rateLimitStoreTimeout)
			err := st.RefundRateLimitBuckets(storeCtx, now, 1, specs)
			cancel()
			if err != nil {
				warnRateLimitStoreFallback("refund", err)
			}
			return
		}
	}
	for _, bucket := range s.general {
		if lim := bucket.localLimiter(); lim != nil {
			lim.ReserveN(now, -1)
		}
	}
}
