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
	"fmt"
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
