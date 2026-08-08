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

package staticserver

import "golang.org/x/sync/semaphore"

// Default origin-fetch admission bounds (finding 12). A cache miss buffers an
// object up to maxOriginObjectBytes in memory; without a gate a burst of distinct
// misses can buffer an unbounded amount at once and OOM the shared single-replica
// server. defaultMaxInflightBytes is the concurrency ceiling × the worst-case
// per-object size, so the two bounds agree in the common case while the byte
// budget stays an independent hard ceiling.
const (
	defaultMaxConcurrentFetches = 32
	defaultMaxInflightBytes     = int64(defaultMaxConcurrentFetches) * maxOriginObjectBytes // 1 GiB
)

// fetchGate bounds concurrent origin fetches two ways: a slot count caps how many
// misses hit the origin at once, and a weighted byte budget caps the total memory
// those in-flight fetches may buffer (each reserves the worst-case object size up
// front, since the true size is unknown until the read completes). Both are
// non-blocking: on overload acquire fails and the caller sheds with 503 rather
// than parking unbounded goroutines/memory.
type fetchGate struct {
	slots  chan struct{}
	bytes  *semaphore.Weighted
	weight int64
}

// newFetchGate builds a gate. maxConcurrent <= 0 disables the gate entirely
// (unbounded, the pre-fix behavior); maxInflightBytes <= 0 keeps the count bound
// but drops the byte budget.
func newFetchGate(maxConcurrent int, maxInflightBytes int64) *fetchGate {
	if maxConcurrent <= 0 {
		return nil
	}
	g := &fetchGate{slots: make(chan struct{}, maxConcurrent), weight: maxOriginObjectBytes}
	if maxInflightBytes > 0 {
		g.bytes = semaphore.NewWeighted(maxInflightBytes)
	}
	return g
}

// acquire reserves one fetch slot and its byte reservation. It returns false
// immediately when either bound is at capacity (the caller must not fetch).
func (g *fetchGate) acquire() bool {
	if g == nil {
		return true
	}
	select {
	case g.slots <- struct{}{}:
	default:
		return false
	}
	if g.bytes != nil && !g.bytes.TryAcquire(g.weight) {
		<-g.slots
		return false
	}
	return true
}

// release returns a slot and its byte reservation acquired by acquire.
func (g *fetchGate) release() {
	if g == nil {
		return
	}
	if g.bytes != nil {
		g.bytes.Release(g.weight)
	}
	<-g.slots
}
