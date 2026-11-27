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

package bex_client_ip_headers

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// w4/m703 property: the static-site redirect memo's lifecycle under arbitrary
// read sequences, as Render's CDN reported it (captured 2026-11-27,
// docs/render-artifacts/edge-headers.md "Every answer's cf-cache-status").
//
// The contract, stated without the implementation: a key reads MISS once,
// then HIT with whole seconds since it was stored while that stays within
// staticRedirectTTL, then EXPIRED once past it and is stored afresh; a read
// with a query always reads BYPASS and stores nothing. The memo never holds
// more than maxStaticRedirects. The point examples in
// httpsredirect_test.go pin single paths; this file pins the temporal
// language (exactly-once MISS per window under interleavings), query purity
// as a state assertion, the exact TTL boundary on both sides, eviction under
// pressure, and concurrent first reads sharing one MISS.

// memoModel is the contract as an independent oracle: per key the time it was
// stored, without any bound, so a divergence is the implementation's, not
// the model's.
type memoModel struct {
	stored map[string]time.Time
}

func (m memoModel) expect(key string, query bool, now time.Time) (string, int64) {
	if query {
		return "BYPASS", -1
	}
	at, ok := m.stored[key]
	switch {
	case ok && now.Sub(at) < staticRedirectTTL:
		return "HIT", int64(now.Sub(at) / time.Second)
	case ok:
		return "EXPIRED", -1
	default:
		return "MISS", -1
	}
}

func (m memoModel) observe(key string, status string, now time.Time) {
	if status == "MISS" || status == "EXPIRED" {
		m.stored[key] = now
	}
}

// TestRedirectMemoLifecycleUnderRandomSequences drives the memo with seeded
// operation sequences and checks every answer against the model, with the
// implementation's own stored times kept in lockstep. Coverage counters prove
// each lifecycle branch was actually exercised.
func TestRedirectMemoLifecycleUnderRandomSequences(t *testing.T) {
	for _, seed := range []int64{20261127, 42} {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			const keys = 7 // small enough that nothing is ever evicted
			target := 2000 + rng.Intn(2000)
			m := &redirectMemo{seen: map[string]time.Time{}}
			model := memoModel{stored: map[string]time.Time{}}
			now := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
			var misses, hits, expired, bypasses int
			lastAge := map[string]int64{}
			for i := 0; i < target; i++ {
				key := string(rune('a'+rng.Intn(keys))) + ".onbex.co/x"
				query := rng.Intn(10) == 0
				switch r := rng.Intn(10); {
				case r < 7:
					now = now.Add(time.Duration(rng.Int63n(int64(10 * time.Second))))
				case r < 9:
					now = now.Add(time.Duration(rng.Int63n(int64(30 * time.Minute))))
				default:
					now = now.Add(staticRedirectTTL + time.Duration(rng.Int63n(int64(time.Hour))))
				}
				before := len(m.seen)
				_, hadKey := m.seen[key]
				status, age := m.status(key, query, now)
				want, wantAge := model.expect(key, query, now)
				if status != want || age != wantAge {
					t.Fatalf("seed %d op %d: %s query=%v at %v: %s age %d, model says %s age %d",
						seed, i, key, query, now, status, age, want, wantAge)
				}
				if query {
					// A query stores nothing: the map is untouched.
					if len(m.seen) != before {
						t.Fatalf("seed %d op %d: a query grew the memo to %d entries", seed, i, len(m.seen))
					}
					if _, ok := m.seen[key]; ok != hadKey {
						t.Fatalf("seed %d op %d: a query changed key %q presence", seed, i, key)
					}
					bypasses++
					continue
				}
				model.observe(key, status, now)
				if got := m.seen[key]; !got.Equal(model.stored[key]) {
					t.Fatalf("seed %d op %d: stored %v, model %v", seed, i, got, model.stored[key])
				}
				switch status {
				case "MISS":
					misses++
					lastAge[key] = -1
				case "HIT":
					hits++
					if age < 0 {
						t.Fatalf("seed %d op %d: HIT with negative age %d", seed, i, age)
					}
					if age < lastAge[key] {
						t.Fatalf("seed %d op %d: HIT age went %d -> %d on %q", seed, i, lastAge[key], age, key)
					}
					lastAge[key] = age
				case "EXPIRED":
					expired++
					lastAge[key] = -1
				}
			}
			if misses == 0 || hits == 0 || expired == 0 || bypasses == 0 {
				t.Fatalf("seed %d: lifecycle branches hit MISS=%d HIT=%d EXPIRED=%d BYPASS=%d, want every branch",
					seed, misses, hits, expired, bypasses)
			}
		})
	}
}

// TestRedirectMemoBoundaryAtTheTTL pins the exact TTL edge: just inside reads
// HIT, exactly at it reads EXPIRED and stores afresh, so the next read is HIT
// again.
func TestRedirectMemoBoundaryAtTheTTL(t *testing.T) {
	m := &redirectMemo{seen: map[string]time.Time{}}
	start := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	if status, _ := m.status("edge.onbex.co/x", false, start); status != "MISS" {
		t.Fatalf("first read: %s, want MISS", status)
	}
	if status, age := m.status("edge.onbex.co/x", false, start.Add(staticRedirectTTL-time.Nanosecond)); status != "HIT" || age != int64((staticRedirectTTL-time.Nanosecond)/time.Second) {
		t.Fatalf("a nanosecond inside the TTL: %s age %d, want HIT age %d", status, age, int64((staticRedirectTTL-time.Nanosecond)/time.Second))
	}
	if status, age := m.status("edge.onbex.co/x", false, start.Add(staticRedirectTTL)); status != "EXPIRED" || age != -1 {
		t.Fatalf("exactly at the TTL: %s age %d, want EXPIRED with no age", status, age)
	}
	if status, age := m.status("edge.onbex.co/x", false, start.Add(staticRedirectTTL)); status != "HIT" || age != 0 {
		t.Fatalf("right after expiry: %s age %d, want HIT age 0 on the stored-afresh copy", status, age)
	}
}

// TestRedirectMemoConcurrentFirstReadsShareOneMiss shows N simultaneous first
// reads of one key collapsing into a single MISS, the rest HIT at age 0.
func TestRedirectMemoConcurrentFirstReadsShareOneMiss(t *testing.T) {
	m := &redirectMemo{seen: map[string]time.Time{}}
	now := time.Date(2026, time.November, 27, 12, 0, 0, 0, time.UTC)
	const n = 32
	var misses, hits atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch status, age := m.status("race.onbex.co/x", false, now); status {
			case "MISS":
				misses.Add(1)
			case "HIT":
				if age != 0 {
					t.Errorf("shared first read: HIT age %d, want 0", age)
					return
				}
				hits.Add(1)
			default:
				t.Errorf("shared first read: %s, want MISS or HIT", status)
			}
		}()
	}
	wg.Wait()
	if misses.Load() != 1 || hits.Load() != n-1 {
		t.Fatalf("%d misses and %d hits from %d first reads, want exactly 1 MISS and %d HIT", misses.Load(), hits.Load(), n, n-1)
	}
}
