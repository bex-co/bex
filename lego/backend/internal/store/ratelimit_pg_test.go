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

package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func cleanRateLimitTables(t *testing.T, st *PGStore, prefix string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), `DELETE FROM rate_limit_buckets WHERE bucket_key LIKE $1`, prefix+"%")
		_, _ = st.Pool.Exec(context.Background(), `DELETE FROM log_subscription_slots WHERE slot_id LIKE $1`, prefix+"%")
	})
}

func TestPGRateLimitBucketsTakeRefillRefundAndPrune(t *testing.T) {
	st := newReplayTestStore(t)
	prefix := fmt.Sprintf("rl-basic-%d:", time.Now().UnixNano())
	cleanRateLimitTables(t, st, prefix)

	ctx := context.Background()
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	spec := core.RateLimitBucketSpec{
		Key:   prefix + "read:u1",
		RPS:   10.0 / 60.0, // 10/min -> 1 token every 6s
		Burst: 10,
		Idle:  10 * time.Minute,
	}

	// Peek on a missing row reports full burst without inserting.
	peek, err := st.PeekRateLimitBucket(ctx, now, spec)
	if err != nil {
		t.Fatalf("peek missing: %v", err)
	}
	if peek.Tokens != 10 || peek.Burst != 10 {
		t.Fatalf("peek missing = %+v, want 10/10", peek)
	}

	// Spend all 10 tokens at the same timestamp.
	for i := range 10 {
		states, wait, err := st.TakeRateLimitBuckets(ctx, now, 1, []core.RateLimitBucketSpec{spec})
		if err != nil {
			t.Fatalf("take %d: %v", i+1, err)
		}
		if wait != 0 {
			t.Fatalf("take %d refused with wait %v", i+1, wait)
		}
		want := float64(9 - i)
		if math.Abs(states[0].Tokens-want) > 1e-6 {
			t.Fatalf("take %d tokens = %v, want %v", i+1, states[0].Tokens, want)
		}
	}

	// 11th take is refused with ~6s wait.
	states, wait, err := st.TakeRateLimitBuckets(ctx, now, 1, []core.RateLimitBucketSpec{spec})
	if err != nil {
		t.Fatalf("11th take: %v", err)
	}
	if wait != 6*time.Second {
		t.Fatalf("11th take wait = %v, want 6s", wait)
	}
	if states[0].Tokens != 0 {
		t.Fatalf("11th take tokens = %v, want 0", states[0].Tokens)
	}

	// 6 seconds later, exactly 1 token refilled.
	later := now.Add(6 * time.Second)
	states, wait, err = st.TakeRateLimitBuckets(ctx, later, 1, []core.RateLimitBucketSpec{spec})
	if err != nil || wait != 0 {
		t.Fatalf("take after 6s: wait=%v err=%v", wait, err)
	}
	if states[0].Tokens > 1e-6 {
		t.Fatalf("tokens after taking refilled token = %v, want ~0", states[0].Tokens)
	}

	// Refund adds back 1 token, and refunding past burst clamps at burst.
	if err := st.RefundRateLimitBuckets(ctx, later, 1, []core.RateLimitBucketSpec{spec}); err != nil {
		t.Fatalf("refund 1: %v", err)
	}
	peek, err = st.PeekRateLimitBucket(ctx, later, spec)
	if err != nil {
		t.Fatalf("peek after refund: %v", err)
	}
	if math.Abs(peek.Tokens-1) > 1e-6 {
		t.Fatalf("peek after refund 1 = %v, want 1", peek.Tokens)
	}
	if err := st.RefundRateLimitBuckets(ctx, later, 100, []core.RateLimitBucketSpec{spec}); err != nil {
		t.Fatalf("refund 100: %v", err)
	}
	peek, err = st.PeekRateLimitBucket(ctx, later, spec)
	if err != nil {
		t.Fatalf("peek after refund 100: %v", err)
	}
	if peek.Tokens != 10 {
		t.Fatalf("refund exceeded burst: got %v, want 10", peek.Tokens)
	}

	// Prune deletes rows whose expires_at has passed.
	pruned, err := st.PruneRateLimitBuckets(ctx, later.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("prune early: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("early prune deleted %d rows, want 0", pruned)
	}
	pruned, err = st.PruneRateLimitBuckets(ctx, later.Add(11*time.Minute))
	if err != nil {
		t.Fatalf("prune expired: %v", err)
	}
	if pruned < 1 {
		t.Fatalf("expired prune deleted %d rows, want >= 1", pruned)
	}
}

func TestPGRateLimitMultiBucketAllOrNothingAndConcurrentNoDeadlock(t *testing.T) {
	st := newReplayTestStore(t)
	prefix := fmt.Sprintf("rl-multi-%d:", time.Now().UnixNano())
	cleanRateLimitTables(t, st, prefix)

	ctx := context.Background()
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

	wide := core.RateLimitBucketSpec{Key: prefix + "z-wide", RPS: 1, Burst: 5, Idle: time.Minute}
	narrow := core.RateLimitBucketSpec{Key: prefix + "a-narrow", RPS: 1.0 / 60.0, Burst: 1, Idle: time.Minute}

	// First multi-bucket take succeeds (4 left in wide, 0 in narrow).
	if _, wait, err := st.TakeRateLimitBuckets(ctx, now, 1, []core.RateLimitBucketSpec{wide, narrow}); err != nil || wait != 0 {
		t.Fatalf("first take: wait=%v err=%v", wait, err)
	}

	// Second multi-bucket take refuses on narrow and must NOT spend wide.
	_, wait, err := st.TakeRateLimitBuckets(ctx, now, 1, []core.RateLimitBucketSpec{wide, narrow})
	if err != nil {
		t.Fatalf("second take err: %v", err)
	}
	if wait != time.Minute {
		t.Fatalf("second take wait = %v, want 1m", wait)
	}
	peekWide, err := st.PeekRateLimitBucket(ctx, now, wide)
	if err != nil {
		t.Fatalf("peek wide: %v", err)
	}
	if peekWide.Tokens != 4 {
		t.Fatalf("wide tokens after refused multi-bucket take = %v, want 4", peekWide.Tokens)
	}

	// Concurrent multi-bucket takes in opposite caller orders must never deadlock
	// and must admit the exact burst count (20).
	b1 := core.RateLimitBucketSpec{Key: prefix + "conc-1", RPS: 1.0 / 60.0, Burst: 20, Idle: time.Minute}
	b2 := core.RateLimitBucketSpec{Key: prefix + "conc-2", RPS: 1.0 / 60.0, Burst: 20, Idle: time.Minute}
	const workers = 40
	var admitted atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			specs := []core.RateLimitBucketSpec{b1, b2}
			if i%2 == 1 {
				specs = []core.RateLimitBucketSpec{b2, b1}
			}
			_, w, err := st.TakeRateLimitBuckets(ctx, now, 1, specs)
			if err != nil {
				errs <- err
				return
			}
			if w == 0 {
				admitted.Add(1)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent take error: %v", err)
	}
	if got := admitted.Load(); got != 20 {
		t.Fatalf("concurrent multi-bucket admitted = %d, want exactly 20", got)
	}
}

func TestPGLogSubscriptionSlotsCapsRenewalReleaseAndExpiry(t *testing.T) {
	st := newReplayTestStore(t)
	prefix := fmt.Sprintf("lss-test-%d-", time.Now().UnixNano())
	cleanRateLimitTables(t, st, prefix)

	ctx := context.Background()
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	ttl := 60 * time.Second

	// Ensure clean slate for global cap assertions.
	if _, err := st.Pool.Exec(ctx, `DELETE FROM log_subscription_slots`); err != nil {
		t.Fatalf("clean slots: %v", err)
	}

	// Per-subject cap = 2, per-workspace cap = 3, global cap = 4.
	if err := st.AcquireLogSubscriptionSlot(ctx, now, prefix+"s1", "sub-a", "tea-1", ttl, 4, 2, 3); err != nil {
		t.Fatalf("acquire s1: %v", err)
	}
	if err := st.AcquireLogSubscriptionSlot(ctx, now, prefix+"s2", "sub-a", "tea-1", ttl, 4, 2, 3); err != nil {
		t.Fatalf("acquire s2: %v", err)
	}
	// 3rd for sub-a hits per-subject cap.
	if err := st.AcquireLogSubscriptionSlot(ctx, now, prefix+"s3", "sub-a", "tea-1", ttl, 4, 2, 3); !errors.Is(err, ErrLogSubscriptionLimit) {
		t.Fatalf("3rd for sub-a = %v, want ErrLogSubscriptionLimit", err)
	}
	// Different subject in tea-1 succeeds (workspace count = 3).
	if err := st.AcquireLogSubscriptionSlot(ctx, now, prefix+"s3", "sub-b", "tea-1", ttl, 4, 2, 3); err != nil {
		t.Fatalf("acquire s3 for sub-b: %v", err)
	}
	// 4th in tea-1 hits per-workspace cap.
	if err := st.AcquireLogSubscriptionSlot(ctx, now, prefix+"s4", "sub-c", "tea-1", ttl, 4, 2, 3); !errors.Is(err, ErrLogSubscriptionLimit) {
		t.Fatalf("4th in tea-1 = %v, want ErrLogSubscriptionLimit", err)
	}
	// Different workspace succeeds (global count = 4).
	if err := st.AcquireLogSubscriptionSlot(ctx, now, prefix+"s4", "sub-c", "tea-2", ttl, 4, 2, 3); err != nil {
		t.Fatalf("acquire s4 in tea-2: %v", err)
	}
	// 5th globally hits global cap.
	if err := st.AcquireLogSubscriptionSlot(ctx, now, prefix+"s5", "sub-d", "tea-3", ttl, 4, 2, 3); !errors.Is(err, ErrLogSubscriptionLimit) {
		t.Fatalf("5th globally = %v, want ErrLogSubscriptionLimit", err)
	}

	// Releasing s1 immediately frees both global and sub-a/tea-1 capacity.
	if err := st.ReleaseLogSubscriptionSlot(ctx, prefix+"s1"); err != nil {
		t.Fatalf("release s1: %v", err)
	}
	if err := st.AcquireLogSubscriptionSlot(ctx, now, prefix+"s5", "sub-a", "tea-1", ttl, 4, 2, 3); err != nil {
		t.Fatalf("acquire s5 after release: %v", err)
	}

	// Renew s5 past now+60s, then advance past TTL: s2/s3/s4 expire and are
	// reclaimed while renewed s5 remains active.
	if err := st.RenewLogSubscriptionSlot(ctx, prefix+"s5", now.Add(120*time.Second)); err != nil {
		t.Fatalf("renew s5: %v", err)
	}
	afterTTL := now.Add(61 * time.Second)
	// With maxGlobal=2, s5 is still active so 1 more fits and a 3rd refuses.
	if err := st.AcquireLogSubscriptionSlot(ctx, afterTTL, prefix+"s6", "sub-a", "tea-1", ttl, 2, 2, 3); err != nil {
		t.Fatalf("acquire s6 after expiry: %v", err)
	}
	if err := st.AcquireLogSubscriptionSlot(ctx, afterTTL, prefix+"s7", "sub-x", "tea-9", ttl, 2, 2, 3); !errors.Is(err, ErrLogSubscriptionLimit) {
		t.Fatalf("3rd global with renewed s5 + s6 = %v, want ErrLogSubscriptionLimit", err)
	}
}
