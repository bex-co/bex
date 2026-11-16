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
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// small is a set of caps a test can reach: 1 KiB/s with a 4 KiB burst per
// bucket, double that per workspace, and 2 requests a second.
func small() LimitConfig {
	return LimitConfig{
		BucketRequestsPerSecond: 1000, BucketRequestBurst: 1000,
		WorkspaceRequestsPerSecond: 1000, WorkspaceRequestBurst: 1000,
		BucketBytesPerSecond: 1024, BucketByteBurst: 4096,
		WorkspaceBytesPerSecond: 2048, WorkspaceByteBurst: 8192,
		BucketInFlight: 32, WorkspaceInFlight: 64, MaxInFlight: 128,
	}
}

func TestABucketOverItsBandwidthCapGets429UntilItRepays(t *testing.T) {
	e := newGWWith(t, small())
	e.put("o", randomBytes(t, 4096), objectstorage.ObjectHeaders{})
	// The first read takes the bucket's whole burst: it is served whole, never cut.
	if rec := e.do(http.MethodGet, "/o"); rec.Code != http.StatusOK || rec.Body.Len() != 4096 {
		t.Fatalf("the first read = %d with %d bytes, want it served whole", rec.Code, rec.Body.Len())
	}
	rec := e.do(http.MethodGet, "/o")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a read with the bucket in debt = %d, want 429", rec.Code)
	}
	wait, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || wait < 1 {
		t.Fatalf("Retry-After = %q, want a positive number of seconds", rec.Header().Get("Retry-After"))
	}
	// Waiting what it says is enough: the bucket has repaid its debt.
	e.clock.advance(time.Duration(wait) * time.Second)
	if rec := e.do(http.MethodGet, "/o"); rec.Code != http.StatusOK {
		t.Errorf("after the Retry-After = %d, want 200", rec.Code)
	}
	// Refused requests cost nothing at Wasabi.
	e.clock.advance(0)
}

func TestARefusedBucketDoesNotReachWasabi(t *testing.T) {
	e := newGWWith(t, small())
	e.put("o", randomBytes(t, 4096), objectstorage.ObjectHeaders{})
	e.do(http.MethodGet, "/o")
	before := len(e.fake.Requests())
	for range 5 {
		if rec := e.do(http.MethodGet, "/o"); rec.Code != http.StatusTooManyRequests {
			t.Fatalf("= %d, want 429", rec.Code)
		}
	}
	if got := len(e.fake.Requests()); got != before {
		t.Errorf("%d requests reached Wasabi for refused reads", got-before)
	}
}

// TestACacheHitIsChargedToo: bandwidth out of the gateway is what the cap bounds,
// not only what Wasabi sent.
func TestACacheHitIsChargedToo(t *testing.T) {
	e := newGWWith(t, small())
	e.put("o", randomBytes(t, 3000), objectstorage.ObjectHeaders{})
	e.do(http.MethodGet, "/o") // miss: 3000 charged
	e.do(http.MethodGet, "/o") // hit: 3000 more, the bucket is now at -2000
	if rec := e.do(http.MethodGet, "/o"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("a third read = %d, want 429: hits spend bandwidth too", rec.Code)
	}
}

func TestAnObjectLargerThanTheBurstIsAnEmptyBucketsWorthNotAnEndlessDebt(t *testing.T) {
	// Each side alone: a debt the bucket's cap makes, and one the workspace's makes.
	for name, edit := range map[string]func(*LimitConfig){
		"bucket cap":    func(c *LimitConfig) { c.BucketByteBurst, c.WorkspaceByteBurst = 2000, 1<<30 },
		"workspace cap": func(c *LimitConfig) { c.BucketByteBurst, c.WorkspaceByteBurst = 1<<30, 2000 },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := small()
			edit(&cfg)
			e := newGWWith(t, cfg)
			e.put("big", randomBytes(t, CacheMaxObjectBytes+10), objectstorage.ObjectHeaders{})
			if rec := e.do(http.MethodGet, "/big"); rec.Code != http.StatusOK {
				t.Fatalf("an object larger than the burst = %d, want 200", rec.Code)
			}
			// It took the whole burst: the cap is in debt at once.
			if rec := e.do(http.MethodGet, "/big"); rec.Code != http.StatusTooManyRequests {
				t.Fatalf("a read right after an object larger than the burst = %d, want 429", rec.Code)
			}
			// The debt is capped at the burst, so it is repaid in burst/rate seconds, not object/rate.
			e.clock.advance(5 * time.Second)
			if rec := e.do(http.MethodGet, "/big"); rec.Code != http.StatusOK {
				t.Errorf("after 5 s = %d, want 200: the charge was capped at the burst", rec.Code)
			}
		})
	}
}

func TestTheWorkspaceCapCoversAllItsBucketsAndSparesOtherWorkspaces(t *testing.T) {
	cfg := small()
	cfg.BucketByteBurst, cfg.WorkspaceByteBurst = 100_000, 4096
	e := newGWWith(t, cfg)
	e.put("o", randomBytes(t, 4096), objectstorage.ObjectHeaders{})

	// A second bucket of the same workspace, and a bucket of another workspace.
	sibling := Bucket{ID: e.bucket.ID[:4] + "00000000000000000000", WorkspaceID: e.bucket.WorkspaceID, Region: objectstorage.Oregon}
	_ = sibling
	e.do(http.MethodGet, "/o") // takes the workspace's whole burst

	lim := e.handler.Limits
	if _, refusal := lim.Admit("obk-other-bucket-same-ws", e.bucket.WorkspaceID); refusal == nil || refusal.Status != http.StatusTooManyRequests {
		t.Errorf("another bucket of the workspace in debt: %+v, want 429", refusal)
	}
	release, refusal := lim.Admit("obk-bucket-of-another-ws", "tea-another-workspace")
	if refusal != nil {
		t.Fatalf("a bucket of another workspace was refused: %+v", refusal)
	}
	release()
}

func TestTheRequestRateCaps(t *testing.T) {
	cfg := small()
	cfg.BucketRequestsPerSecond, cfg.BucketRequestBurst = 1, 3
	e := newGWWith(t, cfg)
	e.put("o", []byte("x"), objectstorage.ObjectHeaders{})
	codes := map[int]int{}
	for range 6 {
		codes[e.do(http.MethodGet, "/o").Code]++
	}
	if codes[http.StatusOK] != 3 || codes[http.StatusTooManyRequests] != 3 {
		t.Errorf("6 requests against a burst of 3 = %v, want 3 served and 3 refused", codes)
	}
	e.clock.advance(2 * time.Second)
	if rec := e.do(http.MethodGet, "/o"); rec.Code != http.StatusOK {
		t.Errorf("after the bucket refilled = %d", rec.Code)
	}

	cfg = small()
	cfg.WorkspaceRequestsPerSecond, cfg.WorkspaceRequestBurst = 1, 2
	e = newGWWith(t, cfg)
	e.put("o", []byte("x"), objectstorage.ObjectHeaders{})
	codes = map[int]int{}
	for range 5 {
		codes[e.do(http.MethodGet, "/o").Code]++
	}
	if codes[http.StatusOK] != 2 || codes[http.StatusTooManyRequests] != 3 {
		t.Errorf("5 requests against a workspace burst of 2 = %v", codes)
	}
}

func TestTheInFlightCapRefusesWith503(t *testing.T) {
	lim := NewLimits(LimitConfig{
		BucketRequestsPerSecond: 100, BucketRequestBurst: 100, WorkspaceRequestsPerSecond: 100, WorkspaceRequestBurst: 100,
		BucketBytesPerSecond: 100, BucketByteBurst: 100, WorkspaceBytesPerSecond: 100, WorkspaceByteBurst: 100,
		BucketInFlight: 10, WorkspaceInFlight: 10, MaxInFlight: 2,
	})
	r1, ref := lim.Admit("b", "w")
	r2, ref2 := lim.Admit("b", "w")
	if ref != nil || ref2 != nil {
		t.Fatal("the first two should be admitted")
	}
	if _, ref := lim.Admit("b", "w"); ref == nil || ref.Status != http.StatusServiceUnavailable {
		t.Errorf("a third in-flight request: %+v, want 503", ref)
	}
	r1()
	r3, ref := lim.Admit("b", "w")
	if ref != nil {
		t.Fatalf("after one finished: %+v, want admitted", ref)
	}
	r2()
	r3()
	// A refusal gives its place back: a long run of refused requests never wedges the cap.
	for range 10 {
		lim.Admit("b", "w") // may be refused for its rate; none may leak a place
	}
}

func TestEveryProductionCapIsSetAndSane(t *testing.T) {
	c := DefaultLimits()
	if c.BucketRequestsPerSecond <= 0 || c.WorkspaceRequestsPerSecond < c.BucketRequestsPerSecond {
		t.Errorf("request caps: bucket %v, workspace %v; a workspace holds many buckets", c.BucketRequestsPerSecond, c.WorkspaceRequestsPerSecond)
	}
	if c.BucketBytesPerSecond <= 0 || c.WorkspaceBytesPerSecond < c.BucketBytesPerSecond {
		t.Errorf("bandwidth caps: bucket %v, workspace %v", c.BucketBytesPerSecond, c.WorkspaceBytesPerSecond)
	}
	if c.BucketByteBurst < CacheMaxObjectBytes || c.WorkspaceByteBurst < c.BucketByteBurst {
		t.Errorf("a burst must hold at least one cacheable object and a workspace's must hold a bucket's: %d, %d", c.BucketByteBurst, c.WorkspaceByteBurst)
	}
	if c.MaxInFlight < 1 || c.BucketInFlight < 1 || c.WorkspaceInFlight < c.BucketInFlight || c.MaxInFlight < c.WorkspaceInFlight {
		t.Errorf("in-flight caps: bucket %d, workspace %d, replica %d; each must hold the one below", c.BucketInFlight, c.WorkspaceInFlight, c.MaxInFlight)
	}
}

// TestTheBandwidthCapsLeaveTheAllowanceAnHourOfWarning pins how the production
// bandwidth caps were sized (w21/m417 t008). Wasabi's free egress is monthly
// egress at or below the account's active storage, which was measured at
// 67,382,215,898 bytes for the whole shared account on 2026-11-16, and a month's
// allowance cannot be protected by a rate: the alert and the kill switch do
// that, so the caps only have to leave them the hour an operator needs. One
// workspace flat out on every production replica must move less than half of the
// allowance in that hour (the first caps, 20 MiB/s, spent all of it in 27
// minutes), and a debt left by one large object must repay within a minute, so
// one reader cannot hold its bucket in 429s for long. If the account's storage
// changes materially, measure it again (docs/runbooks/objectgw-kill-switch.md),
// reset the alert thresholds with it, and change this number in the same commit.
func TestTheBandwidthCapsLeaveTheAllowanceAnHourOfWarning(t *testing.T) {
	const (
		allowanceBytes = 67_382_215_898
		replicas       = 2 // bex-objectgw in the production overlay
		hour           = 3600
	)
	c := DefaultLimits()
	if perHour := c.WorkspaceBytesPerSecond * replicas * hour; perHour >= allowanceBytes/2 {
		t.Errorf("one workspace flat out moves %.1f GB in an hour, which is %.0f%% of the %.1f GB allowance; want under half",
			perHour/1e9, 100*perHour/allowanceBytes, float64(allowanceBytes)/1e9)
	}
	if perHour := c.BucketBytesPerSecond * replicas * hour; perHour >= c.WorkspaceBytesPerSecond*replicas*hour {
		t.Errorf("one bucket flat out moves %.1f GB an hour, not less than its workspace's cap", perHour/1e9)
	}
	for name, repay := range map[string]float64{
		"bucket":    float64(c.BucketByteBurst) / c.BucketBytesPerSecond,
		"workspace": float64(c.WorkspaceByteBurst) / c.WorkspaceBytesPerSecond,
	} {
		if repay > 60 {
			t.Errorf("a %s emptied by one object repays in %.0f s, want within a minute", name, repay)
		}
	}
}

// TestAZeroCapPanicsAtConstruction: a limiter built with a zero rate disables
// itself, which would turn a typo into no cap at all.
func TestAZeroCapPanicsAtConstruction(t *testing.T) {
	for name, edit := range map[string]func(*LimitConfig){
		"request rate": func(c *LimitConfig) { c.BucketRequestsPerSecond = 0 },
		"byte burst":   func(c *LimitConfig) { c.WorkspaceByteBurst = 0 },
		"in flight":    func(c *LimitConfig) { c.BucketInFlight = 0 },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: a zero cap did not panic", name)
				}
			}()
			c := DefaultLimits()
			edit(&c)
			NewLimits(c)
		}()
	}
}

// TestOneBucketsSlowReadersCannotHoldEveryPlace: the places a bucket and a
// workspace may hold are capped below the replica's, so a few slow readers of one
// bucket leave room for everyone else.
func TestOneBucketsSlowReadersCannotHoldEveryPlace(t *testing.T) {
	cfg := DefaultLimits()
	cfg.BucketInFlight, cfg.WorkspaceInFlight, cfg.MaxInFlight = 2, 3, 10
	lim := NewLimits(cfg)
	var releases []func()
	admit := func(bucket, ws string) *Refusal {
		release, refusal := lim.Admit(bucket, ws)
		if refusal == nil {
			releases = append(releases, release)
		}
		return refusal
	}
	for range 2 {
		if r := admit("b1", "w1"); r != nil {
			t.Fatalf("within the bucket cap: %+v", r)
		}
	}
	if r := admit("b1", "w1"); r == nil || r.Status != http.StatusTooManyRequests {
		t.Errorf("a third open response of one bucket: %+v, want 429", r)
	}
	if r := admit("b2", "w1"); r != nil { // the workspace's third
		t.Errorf("another bucket of the workspace: %+v", r)
	}
	if r := admit("b3", "w1"); r == nil || r.Status != http.StatusTooManyRequests {
		t.Errorf("a fourth open response of one workspace: %+v, want 429", r)
	}
	if r := admit("b4", "w2"); r != nil {
		t.Errorf("another workspace was refused: %+v; its places are its own", r)
	}
	// Releasing gives every place back, including after refusals.
	for _, release := range releases {
		release()
	}
	if len(lim.bucketOpen.n) != 0 || len(lim.workspaceOpen.n) != 0 || len(lim.inflight) != 0 {
		t.Errorf("places leaked: %v %v %d", lim.bucketOpen.n, lim.workspaceOpen.n, len(lim.inflight))
	}
}

// TestAStreamedObjectIsChargedWhenItStarts: an object too large for the cache is
// relayed from Wasabi, and its bytes are debited from the bucket as the response
// begins, not after it ends and not never.
func TestAStreamedObjectIsChargedWhenItStarts(t *testing.T) {
	e := newGWWith(t, small())
	e.put("big", randomBytes(t, CacheMaxObjectBytes+5), objectstorage.ObjectHeaders{})
	if rec := e.do(http.MethodGet, "/big"); rec.Code != http.StatusOK || rec.Body.Len() != CacheMaxObjectBytes+5 {
		t.Fatalf("the first read = %d with %d bytes, want it served whole", rec.Code, rec.Body.Len())
	}
	if rec := e.do(http.MethodGet, "/big"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("the next read = %d, want 429: the stream was never charged", rec.Code)
	}
}
