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

package cliauth

import (
	"math"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// DeviceRateLimiter is an IP-keyed token-bucket limiter guarding the three
// credential-less device-flow routes (w4/m31/t002) — every hit costs a full
// Hydra round trip with no metering, and these routes mount outside the auth
// gate by design (internal/api/CLAUDE.md's always-public inventory), so the
// identity-keyed internal/api.RateLimiter never sees them and would key every
// anonymous caller into one shared bucket even if it did. This mirrors that
// limiter's shape (map + mutex + idle sweep over golang.org/x/time/rate) but
// stays IP-only (there is never an identity here) and lives in this package
// rather than internal/api to avoid a cliauth->api->cliauth import cycle
// (internal/api/server.go already imports cliauth to wire it in).
//
// NOTE: single-replica, like internal/api.RateLimiter — the same
// FUTURE-MAYBE boundary (a distributed counter is the follow-up if bex-api
// scales out).
type DeviceRateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*rateEntry
	rps       rate.Limit
	burst     int
	lastSweep time.Time
}

type rateEntry struct {
	lim  *rate.Limiter
	last time.Time
}

const (
	deviceLimiterIdle       = 10 * time.Minute
	deviceLimiterSweepEvery = 5 * time.Minute
)

// NewDeviceRateLimiter returns an IP-keyed limiter at rpm requests/minute
// with the given burst depth. Zero or negative rpm returns nil (disabled).
// burst <= 0 defaults to ceil(rpm) — the same convention
// internal/api.NewRateLimiter uses.
func NewDeviceRateLimiter(rpm float64, burst int) *DeviceRateLimiter {
	if rpm <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = int(math.Ceil(rpm))
	}
	return &DeviceRateLimiter{
		entries: make(map[string]*rateEntry),
		rps:     rate.Limit(rpm / 60),
		burst:   burst,
	}
}

// allow reports whether the request's source IP may proceed now; if not, the
// seconds the caller should wait before retrying.
func (rl *DeviceRateLimiter) allow(r *http.Request) (bool, int) {
	lim := rl.bucket(clientIP(r))
	res := lim.Reserve()
	if d := res.Delay(); d > 0 {
		res.Cancel()
		wait := int(math.Ceil(d.Seconds()))
		if wait < 1 {
			wait = 1
		}
		return false, wait
	}
	return true, 0
}

// bucket returns (and lazily creates) the per-IP limiter, sweeping idle
// entries periodically to bound memory use.
func (rl *DeviceRateLimiter) bucket(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.lastSweep) > deviceLimiterSweepEvery {
		for k, e := range rl.entries {
			if now.Sub(e.last) > deviceLimiterIdle {
				delete(rl.entries, k)
			}
		}
		rl.lastSweep = now
	}
	e, ok := rl.entries[key]
	if !ok {
		e = &rateEntry{lim: rate.NewLimiter(rl.rps, rl.burst)}
		rl.entries[key] = e
	}
	e.last = now
	return e.lim
}

// clientIP extracts the request's source IP, falling back to the raw
// RemoteAddr if it isn't a host:port pair.
func clientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
