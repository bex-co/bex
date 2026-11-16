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

package objectgw

import (
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// The caps that bound what public reads can cost the shared Wasabi account and
// the nodes' egress (ADR112 D4). They are bex's own: neither Render nor
// Cloudflare publishes a value (Cloudflare calls r2.dev non-production and
// rate-limited without saying by how much), so each is a dated, unverified entry
// in ADR018, and changing one is a reviewed code change. None is an environment
// variable (ADR112 D9).
//
// Every limit is per gateway replica: with N replicas the platform allows N
// times these, as core.KeyedRateLimiter documents. A bandwidth cap bounds a
// rate, not a month: the month-to-date egress alert and the kill switch bound
// the volume (docs/runbooks/objectgw-kill-switch.md).
//
// The values were reset from the production walk of 2026-11-16 (w21/m417 t008,
// ADR112 § Not verified), which replaced the first values (50 and 100 requests
// a second, 10 and 20 MiB a second):
//
//   - What a replica can move is not the constraint. Relaying from Wasabi's
//     Frankfurt region, 24 parallel streams of a 20 MiB object reached 216 MiB/s
//     over the two replicas, and each replica spent about 2 ms of CPU per MiB
//     (0.55 and 0.36 CPU-seconds for 260 and 220 MiB), so one replica's 0.5 core
//     limit is worth roughly 240 MiB/s; from Oregon a stream runs at 8 MiB/s and 8
//     streams at 68 MiB/s. The nodes' egress is not the constraint either: 22 TB
//     a month included per node against 24 GB used by all 15 servers.
//   - What a workspace can spend of Wasabi's allowance is. Wasabi's free egress
//     is monthly egress at or below the account's active storage, measured at
//     67 GB for the whole shared account, so 20 MiB/s from each of two replicas
//     (a workspace's first cap) spent all of it in 27 minutes, before the first
//     alert's hour of waiting had passed. The bandwidth caps are now sized so one
//     workspace flat out on both replicas moves less than half of that allowance
//     in the hour an operator needs to answer an alert. The bursts are about 32
//     seconds of the rate, so a large object does not hold a bucket in debt for
//     minutes.
//   - The request caps held exactly: 20 s and 30 s of load at 9,000 and 12,000
//     requests a second were admitted at 2178 and 3192, the 2 x (burst + rate x
//     seconds) the cap allows. A refused request cost about 80 microseconds of
//     CPU, so a flood of about 6,000 requests a second to one replica uses its
//     whole 0.5 core whatever the caps say; they protect bex-api's lookups and
//     Wasabi's request volume, and no measurement asked for a change.
const (
	// BucketRequestsPerSecond and BucketRequestBurst bound one bucket's requests.
	BucketRequestsPerSecond = 50
	BucketRequestBurst      = 100
	// WorkspaceRequestsPerSecond and WorkspaceRequestBurst bound all of one
	// workspace's public buckets together.
	WorkspaceRequestsPerSecond = 100
	WorkspaceRequestBurst      = 200

	// BucketBytesPerSecond and BucketByteBurst bound the bytes one bucket sends:
	// 2 MiB/s sustained, which is 16 Mbit/s, with room for 64 MiB.
	BucketBytesPerSecond = 2 << 20
	BucketByteBurst      = 64 << 20
	// WorkspaceBytesPerSecond and WorkspaceByteBurst bound all of one
	// workspace's public buckets together.
	WorkspaceBytesPerSecond = 4 << 20
	WorkspaceByteBurst      = 128 << 20

	// BucketInFlight and WorkspaceInFlight bound the responses one bucket or
	// workspace has open at once, so a few slow readers of one bucket cannot hold
	// every place the replica has.
	BucketInFlight    = 32
	WorkspaceInFlight = 64
	// MaxInFlight bounds the requests one replica serves at once.
	MaxInFlight = 256
)

// LimitConfig is a set of caps. Production uses DefaultLimits; tests inject
// small ones, as the object store's own tests inject smaller caps through its
// config struct. Every value must be positive.
type LimitConfig struct {
	BucketRequestsPerSecond, WorkspaceRequestsPerSecond float64
	BucketRequestBurst, WorkspaceRequestBurst           int
	BucketBytesPerSecond, WorkspaceBytesPerSecond       float64
	BucketByteBurst, WorkspaceByteBurst                 int
	BucketInFlight, WorkspaceInFlight, MaxInFlight      int
}

// DefaultLimits are the production caps.
func DefaultLimits() LimitConfig {
	return LimitConfig{
		BucketRequestsPerSecond: BucketRequestsPerSecond, BucketRequestBurst: BucketRequestBurst,
		WorkspaceRequestsPerSecond: WorkspaceRequestsPerSecond, WorkspaceRequestBurst: WorkspaceRequestBurst,
		BucketBytesPerSecond: BucketBytesPerSecond, BucketByteBurst: BucketByteBurst,
		WorkspaceBytesPerSecond: WorkspaceBytesPerSecond, WorkspaceByteBurst: WorkspaceByteBurst,
		BucketInFlight: BucketInFlight, WorkspaceInFlight: WorkspaceInFlight, MaxInFlight: MaxInFlight,
	}
}

// Refusal is a request the caps hold back: its status (429 for a cap of one
// bucket or workspace, 503 for the replica's own), the result it counts as, and
// how many seconds to wait.
type Refusal struct {
	Status            int
	Result            requestResult
	RetryAfterSeconds int
}

var (
	replicaBusy = &Refusal{Status: http.StatusServiceUnavailable, Result: resultUnavailable, RetryAfterSeconds: 1}
	tooMany     = &Refusal{Status: http.StatusTooManyRequests, Result: resultRateLimited, RetryAfterSeconds: 1}
)

// Limits applies the caps. A request is admitted by Admit, which refuses it when
// it is over a request rate, a place limit, or when its bucket or workspace has
// spent its bandwidth, and Charge debits the bytes a response will send. A
// response is charged as it starts, so one large object can take a bucket into
// debt and is served whole; the next requests wait until the debt is repaid at
// the sustained rate. The first reader is never cut off mid-body, and a bucket
// that streams more than its cap gets 429.
type Limits struct {
	cfg                               LimitConfig
	bucketRequests, workspaceRequests *core.KeyedRateLimiter[string]
	bucketBytes, workspaceBytes       *core.KeyedRateLimiter[string]
	inflight                          chan struct{}
	bucketOpen, workspaceOpen         *openCounter
	now                               func() time.Time
}

// NewLimits builds the limiters for c. A zero cap is a programming error (the
// limiters would disable themselves), so it panics at construction.
func NewLimits(c LimitConfig) *Limits {
	for _, v := range []float64{
		c.BucketRequestsPerSecond, c.WorkspaceRequestsPerSecond, float64(c.BucketRequestBurst), float64(c.WorkspaceRequestBurst),
		c.BucketBytesPerSecond, c.WorkspaceBytesPerSecond, float64(c.BucketByteBurst), float64(c.WorkspaceByteBurst),
		float64(c.BucketInFlight), float64(c.WorkspaceInFlight), float64(c.MaxInFlight),
	} {
		if v <= 0 {
			panic("objectgw: every cap in a LimitConfig must be positive")
		}
	}
	const idle = 10 * time.Minute
	return &Limits{
		cfg:               c,
		bucketRequests:    core.NewKeyedRateLimiter[string](c.BucketRequestsPerSecond*60, c.BucketRequestBurst, idle, time.Minute),
		workspaceRequests: core.NewKeyedRateLimiter[string](c.WorkspaceRequestsPerSecond*60, c.WorkspaceRequestBurst, idle, time.Minute),
		bucketBytes:       core.NewKeyedRateLimiter[string](c.BucketBytesPerSecond*60, c.BucketByteBurst, idle, time.Minute),
		workspaceBytes:    core.NewKeyedRateLimiter[string](c.WorkspaceBytesPerSecond*60, c.WorkspaceByteBurst, idle, time.Minute),
		inflight:          make(chan struct{}, c.MaxInFlight),
		bucketOpen:        newOpenCounter(c.BucketInFlight), workspaceOpen: newOpenCounter(c.WorkspaceInFlight),
		now: time.Now,
	}
}

// openCounter counts the responses open per key and refuses one past its limit.
// A key with none open has no entry, so it holds nothing for idle keys.
type openCounter struct {
	max int
	mu  sync.Mutex
	n   map[string]int
}

func newOpenCounter(max int) *openCounter { return &openCounter{max: max, n: map[string]int{}} }

func (c *openCounter) acquire(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n[key] >= c.max {
		return false
	}
	c.n[key]++
	return true
}

func (c *openCounter) release(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n[key] <= 1 {
		delete(c.n, key)
		return
	}
	c.n[key]--
}

// Admit decides whether a request for a bucket of a workspace may proceed. The
// release it returns frees the request's places and must be called once the
// response is finished.
func (l *Limits) Admit(bucketID, workspaceID string) (release func(), refusal *Refusal) {
	select {
	case l.inflight <- struct{}{}:
	default:
		return nil, replicaBusy
	}
	releaseAll := func() { <-l.inflight }
	now := l.now()
	if !l.bucketRequests.Bucket(bucketID).AllowN(now, 1) || !l.workspaceRequests.Bucket(workspaceID).AllowN(now, 1) {
		releaseAll()
		return nil, tooMany
	}
	for _, b := range []struct {
		tokens float64
		rate   float64
	}{
		{l.bucketBytes.Bucket(bucketID).TokensAt(now), l.cfg.BucketBytesPerSecond},
		{l.workspaceBytes.Bucket(workspaceID).TokensAt(now), l.cfg.WorkspaceBytesPerSecond},
	} {
		if b.tokens <= 0 {
			releaseAll()
			// Wait until the debt is repaid and one more byte is allowed.
			return nil, &Refusal{Status: http.StatusTooManyRequests, Result: resultRateLimited, RetryAfterSeconds: max(1, int(math.Ceil((1-b.tokens)/b.rate)))}
		}
	}
	if !l.bucketOpen.acquire(bucketID) {
		releaseAll()
		return nil, tooMany
	}
	if !l.workspaceOpen.acquire(workspaceID) {
		l.bucketOpen.release(bucketID)
		releaseAll()
		return nil, tooMany
	}
	return func() {
		l.workspaceOpen.release(workspaceID)
		l.bucketOpen.release(bucketID)
		releaseAll()
	}, nil
}

// Charge debits n bytes from the bucket's and the workspace's bandwidth. It never
// refuses: a response already admitted is sent whole, and the debt is what the
// next Admit sees. A charge is capped at the burst so one object larger than it
// is an empty bucket's worth, not a debt that never repays.
func (l *Limits) Charge(bucketID, workspaceID string, n int64) {
	if n <= 0 {
		return
	}
	now := l.now()
	l.bucketBytes.Bucket(bucketID).ReserveN(now, int(min(n, int64(l.cfg.BucketByteBurst))))
	l.workspaceBytes.Bucket(workspaceID).ReserveN(now, int(min(n, int64(l.cfg.WorkspaceByteBurst))))
}
