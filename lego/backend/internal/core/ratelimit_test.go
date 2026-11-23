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
	"net/http"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestKeyedRateLimiterDisabledWhenZeroOrNegative(t *testing.T) {
	if NewKeyedRateLimiter[string](0, 0, time.Minute, time.Minute) != nil {
		t.Error("zero rpm should return nil (disabled)")
	}
	if NewKeyedRateLimiter[int](-1, 0, time.Minute, time.Minute) != nil {
		t.Error("negative rpm should return nil (disabled)")
	}
}

func TestKeyedRateLimiterDefaultsBurstToCeilRPM(t *testing.T) {
	rl := NewKeyedRateLimiter[string](10, 0, time.Minute, time.Minute)
	if rl == nil {
		t.Fatal("want non-nil limiter")
	}
	// burst=ceil(10)=10, so the first 10 requests for a key should pass.
	lim := rl.Bucket("a")
	for i := 0; i < 10; i++ {
		if d := lim.Reserve().Delay(); d > 0 {
			t.Fatalf("request %d should pass immediately, got delay %v", i+1, d)
		}
	}
	// 11th request should be delayed.
	if d := lim.Reserve().Delay(); d == 0 {
		t.Error("11th request should be rate-limited")
	}
}

func TestKeyedRateLimiterPerKeyIsolation(t *testing.T) {
	rl := NewKeyedRateLimiter[string](60, 1, time.Minute, time.Minute) // burst=1
	if rl == nil {
		t.Fatal("want non-nil limiter")
	}

	// Key "a": first passes, second is denied.
	if d := rl.Bucket("a").Reserve().Delay(); d > 0 {
		t.Fatalf("a first request: want no delay, got %v", d)
	}
	if d := rl.Bucket("a").Reserve().Delay(); d == 0 {
		t.Fatal("a second request: want delay, got none")
	}

	// Key "b": first passes independently.
	if d := rl.Bucket("b").Reserve().Delay(); d > 0 {
		t.Fatalf("b first request: want no delay, got %v", d)
	}
}

func TestKeyedRateLimiterNilSafe(t *testing.T) {
	var rl *KeyedRateLimiter[string]
	if lim := rl.Bucket("any"); lim != nil {
		t.Fatalf("nil limiter Bucket should return nil, got %v", lim)
	}
	if h := rl.Handle("any"); !h.IsZero() {
		t.Fatalf("nil limiter Handle should be zero, got %+v", h)
	}
}

func TestKeyedRateLimiterSweepsIdleEntries(t *testing.T) {
	idle := 10 * time.Millisecond
	sweep := 5 * time.Millisecond
	rl := NewKeyedRateLimiter[string](60, 1, idle, sweep)
	if rl == nil {
		t.Fatal("want non-nil limiter")
	}

	// Create an entry.
	_ = rl.Bucket("x")
	if len(rl.entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(rl.entries))
	}

	// Wait long enough for the entry to be considered idle plus a sweep window.
	time.Sleep(idle + sweep + 10*time.Millisecond)

	// A new Bucket call should trigger the sweep and remove the idle entry.
	_ = rl.Bucket("y")
	if _, ok := rl.entries["x"]; ok {
		t.Error("idle entry 'x' should have been swept")
	}
	if _, ok := rl.entries["y"]; !ok {
		t.Error("entry 'y' should exist")
	}
}

func TestKeyedRateLimiterAcceptsComparableKeyTypes(t *testing.T) {
	// [32]byte is comparable and matches deploys' SHA-256 key shape.
	rl := NewKeyedRateLimiter[[32]byte](60, 1, time.Minute, time.Minute)
	if rl == nil {
		t.Fatal("want non-nil limiter")
	}
	var k [32]byte
	k[0] = 1
	if lim := rl.Bucket(k); lim == nil {
		t.Fatal("want non-nil limiter for [32]byte key")
	}
}

func TestKeyedRateLimiterPreservesRateAndBurst(t *testing.T) {
	rl := NewKeyedRateLimiter[string](120, 5, time.Minute, time.Minute)
	if rl == nil {
		t.Fatal("want non-nil limiter")
	}
	if rl.rps != rate.Limit(2) {
		t.Errorf("rps = %v, want 2", rl.rps)
	}
	if rl.burst != 5 {
		t.Errorf("burst = %d, want 5", rl.burst)
	}
}

// TestKeyedRateLimiterCapsEntries is the codex-security 2026-08 F1 bound: a
// key-space flood (one distinct key per request — the shape of an anonymous
// caller rotating bearer tokens) must never grow the map past
// maxKeyedRateLimitEntries. At the cap the idle are swept first, then the map
// is reset wholesale, mirroring TTLCache's CacheMax discipline.
func TestKeyedRateLimiterCapsEntries(t *testing.T) {
	rl := NewKeyedRateLimiter[string](60, 1, time.Hour, time.Hour) // idle entries never swept mid-test
	if rl == nil {
		t.Fatal("want non-nil limiter")
	}
	for i := 0; i < maxKeyedRateLimitEntries*2; i++ {
		_ = rl.Bucket(fmt.Sprintf("flood-key-%d", i))
	}
	if got := rl.Entries(); got > maxKeyedRateLimitEntries {
		t.Errorf("entries = %d after %d distinct keys; want ≤ %d — the map must be capped, not unbounded",
			got, maxKeyedRateLimitEntries*2, maxKeyedRateLimitEntries)
	}
}

func TestSetRateLimitHeadersReportsTheTightestBucket(t *testing.T) {
	roomy := RateLimitBucketState{Tokens: 10, RPS: 1, Burst: 10}
	tight := RateLimitBucketState{Tokens: 1, RPS: 0.5, Burst: 4} // 1 token left, 3 to refill at 0.5/s
	h := http.Header{}
	SetRateLimitHeaders(h, false, roomy, tight)
	if h.Get("RateLimit-Limit") != "4" || h.Get("RateLimit-Remaining") != "1" || h.Get("RateLimit-Reset") != "6" {
		t.Errorf("headers = %v, want the tight bucket's 4/1/6", h)
	}
	full := http.Header{}
	SetRateLimitHeaders(full, false, roomy)
	if full.Get("RateLimit-Remaining") != "10" || full.Get("RateLimit-Reset") != "0" {
		t.Errorf("full bucket = %v, want remaining 10 and reset 0", full)
	}
	refused := http.Header{}
	SetRateLimitHeaders(refused, true, roomy)
	if refused.Get("RateLimit-Remaining") != "0" {
		t.Errorf("refused remaining = %q, want 0", refused.Get("RateLimit-Remaining"))
	}
}

// ReserveEach takes from every bucket or from none: a refusal by a later
// bucket gives the earlier bucket's token back, and reports that bucket's wait.
func TestReserveEachTakesFromAllOrNone(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	wide := rate.NewLimiter(rate.Limit(1), 5)
	narrow := rate.NewLimiter(rate.Limit(1.0/60), 1)
	taken, wait := ReserveEach(now, 1, wide, narrow)
	if wait != 0 || len(taken) != 2 {
		t.Fatalf("first reserve = %d reservations, wait %v", len(taken), wait)
	}
	taken, wait = ReserveEach(now, 1, wide, narrow)
	if len(taken) != 2 || wait != time.Minute {
		t.Fatalf("refused reserve = %d reservations, wait %v, want 2 and the narrow bucket's minute", len(taken), wait)
	}
	if got := wide.TokensAt(now); got != 4 {
		t.Fatalf("wide bucket holds %v tokens after a refusal, want 4 (the refused request's token given back)", got)
	}
	if got := narrow.TokensAt(now); got != 0 {
		t.Fatalf("narrow bucket holds %v tokens, want 0 (no debt left by the refusal)", got)
	}
	// A refusal by the first bucket never reaches the second.
	taken, wait = ReserveEach(now, 1, narrow, wide)
	if len(taken) != 1 || wait != time.Minute || wide.TokensAt(now) != 4 {
		t.Fatalf("first-bucket refusal = %d reservations, wait %v, wide %v", len(taken), wait, wide.TokensAt(now))
	}
}

func TestSetRateLimitRefusalWritesRetryAfterAndRemainingZero(t *testing.T) {
	bucket := RateLimitBucketState{Tokens: 0, RPS: 10.0 / 60, Burst: 10}
	h := http.Header{}
	if got := SetRateLimitRefusal(h, 5500*time.Millisecond, bucket); got != 6 {
		t.Fatalf("seconds = %d, want 5.5 s rounded up", got)
	}
	if h.Get("Retry-After") != "6" || h.Get("RateLimit-Limit") != "10" || h.Get("RateLimit-Remaining") != "0" {
		t.Fatalf("headers = %v", h)
	}
	if got := SetRateLimitRefusal(nil, time.Millisecond, bucket); got != 1 {
		t.Fatalf("seconds without a header = %d, want the 1 s floor", got)
	}
}

// The scope's refund puts back exactly the token an admitted request took,
// once, long after the reservation's own Cancel would restore nothing, and
// never fills a bucket past its burst.
func TestRateLimitScopeRefundsOnceAndNeverPastTheBurst(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	subject := rate.NewLimiter(rate.Limit(1.0/60), 3)
	workspace := rate.NewLimiter(rate.Limit(1.0/60), 3)
	// Two requests were admitted; the scope is the second one's.
	ReserveEach(now, 1, subject, workspace)
	taken, _ := ReserveEach(now, 1, subject, workspace)
	later := now.Add(time.Second)
	taken[0].CancelAt(later)
	if got := subject.TokensAt(later); got >= 1.5 {
		t.Fatalf("Reservation.CancelAt restored a spent token (%v): Refund no longer needs its own arithmetic", got)
	}
	h := http.Header{}
	scope := NewRateLimitScope("workspace:tea-a", h, []RateLimitBucket{LocalRateLimitBucket(subject), LocalRateLimitBucket(workspace)}, false)
	ctx := WithRateLimitScope(t.Context(), scope)
	if got := RateLimitScopeFrom(ctx); got != scope || got.Workspace() != "workspace:tea-a" || got.Header() == nil {
		t.Fatalf("scope from context = %+v", got)
	}
	scope.Refund(ctx, later)
	scope.Refund(ctx, later)
	for name, bucket := range map[string]*rate.Limiter{"subject": subject, "workspace": workspace} {
		if n := drain(bucket, later); n != 2 {
			t.Errorf("%s bucket admitted %d after two refunds of one request, want 2: its own token back, once", name, n)
		}
	}
	// A full bucket refunded stays at its burst.
	full := rate.NewLimiter(rate.Limit(1.0/60), 3)
	NewRateLimitScope("", nil, []RateLimitBucket{LocalRateLimitBucket(full)}, false).Refund(t.Context(), now)
	if n := drain(full, now); n != 3 {
		t.Fatalf("a refund filled a full bucket to %d, want its burst of 3", n)
	}
	// No scope: every method is a no-op.
	var none *RateLimitScope
	none.Refund(t.Context(), now)
	if RateLimitScopeFrom(t.Context()) != nil || none.Workspace() != "" || none.Header() != nil {
		t.Fatal("a missing scope is not empty")
	}
}

type failingRateLimitStore struct {
	err error
}

func (f failingRateLimitStore) TakeRateLimitBuckets(context.Context, time.Time, int, []RateLimitBucketSpec) ([]RateLimitBucketState, time.Duration, error) {
	return nil, 0, f.err
}

func (f failingRateLimitStore) RefundRateLimitBuckets(context.Context, time.Time, int, []RateLimitBucketSpec) error {
	return f.err
}

func (f failingRateLimitStore) PeekRateLimitBucket(context.Context, time.Time, RateLimitBucketSpec) (RateLimitBucketState, error) {
	return RateLimitBucketState{}, f.err
}

// When the shared store returns an error, TakeRateLimitBuckets and
// PeekRateLimitBucket fall back fail-open to the local in-process bucket and
// continue enforcing the local bound.
func TestTakeRateLimitBucketsFallsBackToLocalOnStoreError(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	rl := NewKeyedRateLimiter[string](60, 1, time.Minute, time.Minute)
	rl.SetSharedStore(failingRateLimitStore{err: errors.New("postgres down")}, "general:caller", nil)

	bucket := rl.Handle("id:u1")
	if st := PeekRateLimitBucket(t.Context(), now, bucket); st.Tokens < 1 {
		t.Fatalf("peek fallback tokens = %v, want 1", st.Tokens)
	}
	states, wait, usedStore := TakeRateLimitBuckets(t.Context(), now, 1, bucket)
	if usedStore || wait != 0 || len(states) != 1 {
		t.Fatalf("first fallback take: usedStore=%v wait=%v states=%+v", usedStore, wait, states)
	}
	_, wait, usedStore = TakeRateLimitBuckets(t.Context(), now, 1, bucket)
	if usedStore || wait == 0 {
		t.Fatalf("second fallback take should be locally rate-limited: usedStore=%v wait=%v", usedStore, wait)
	}
}

func drain(bucket *rate.Limiter, now time.Time) int {
	n := 0
	for bucket.AllowN(now, 1) {
		n++
	}
	return n
}
