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
	"math"
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
