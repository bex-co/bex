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

package sshgateway

import (
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

func TestSessionLimits(t *testing.T) {
	limits := NewSessionLimiter(2, 1)
	a, _ := limits.Acquire("a")
	aAgain, identityScope := limits.Acquire("a")
	b, _ := limits.Acquire("b")
	c, globalScope := limits.Acquire("c")
	if !a || aAgain || identityScope != "identity" || !b || c || globalScope != "global" {
		t.Fatal("global/per-identity limits not enforced")
	}
	limits.Release("a")
	c, _ = limits.Acquire("c")
	if !c {
		t.Fatal("released capacity was not reusable")
	}
}

// codex round-8 #7: channel (exec-stream) caps are enforced on both scopes and
// released when a stream ends, independently of the transport-count session
// limiter.
func TestChannelLimits(t *testing.T) {
	channels := NewChannelLimiter(2, 1)
	a, _ := channels.AcquireChannel("a")
	aAgain, identityScope := channels.AcquireChannel("a")
	b, _ := channels.AcquireChannel("b")
	c, globalScope := channels.AcquireChannel("c")
	if !a || aAgain || identityScope != "channel_identity" || !b || c || globalScope != "channel_global" {
		t.Fatal("global/per-identity channel limits not enforced")
	}
	channels.ReleaseChannel("a")
	c, _ = channels.AcquireChannel("c")
	if !c {
		t.Fatal("released channel capacity was not reusable")
	}
}

func TestChannelLimitsDefaults(t *testing.T) {
	channels := NewChannelLimiter(0, 0)
	// Defaults (512 global, 32 per identity) apply for non-positive values.
	for i := 0; i < 32; i++ {
		if ok, scope := channels.AcquireChannel("a"); !ok {
			t.Fatalf("channel %d within the per-identity default must pass (scope %q)", i+1, scope)
		}
	}
	if ok, scope := channels.AcquireChannel("a"); ok || scope != "channel_identity" {
		t.Fatalf("33rd stream of one identity must be shed at the default, got ok=%v scope=%q", ok, scope)
	}
	if ok, _ := channels.AcquireChannel("b"); !ok {
		t.Fatal("a second identity must not be affected by the first identity's channel budget")
	}
}

// round-11 #2: the per-source pre-auth limiter caps one resolved source's
// share of the global pre-auth pool, releases slots when handshakes resolve,
// and can be disabled with a negative cap (global-only, pre-round-11).
func TestSourceLimiterPreAuthFairness(t *testing.T) {
	sources := NewSourceLimiter(2)
	if !sources.Acquire("203.0.113.9") || !sources.Acquire("203.0.113.9") {
		t.Fatal("first two connections from one source must be admitted")
	}
	if sources.Acquire("203.0.113.9") {
		t.Fatal("third concurrent connection from the same source must be shed")
	}
	if !sources.Acquire("198.51.100.7") {
		t.Fatal("an unrelated source must stay admissible — the cap is per-source, not global")
	}
	sources.Release("203.0.113.9")
	if !sources.Acquire("203.0.113.9") {
		t.Fatal("released per-source capacity was not reusable")
	}
	sources.Release("203.0.113.9")
	sources.Release("203.0.113.9")
	if len(sources.count) != 1 { // only 198.51.100.7 remains
		t.Fatalf("fully-released sources must leave no map entries: %+v", sources.count)
	}

	disabled := NewSourceLimiter(-1)
	for i := 0; i < 64; i++ {
		if !disabled.Acquire("203.0.113.9") {
			t.Fatal("negative cap disables per-source fairness")
		}
	}
}

// A stray Release must not drive the counts negative and lift either cap.
func TestUnmatchedReleaseKeepsCaps(t *testing.T) {
	sessions := NewSessionLimiter(2, 1)
	channels := NewChannelLimiter(2, 1)
	for _, tc := range []struct {
		name                 string
		acquire              func(string) (bool, string)
		release              func(string)
		globalScope, idScope string
	}{
		{"session", sessions.Acquire, sessions.Release, "global", "identity"},
		{"channel", channels.AcquireChannel, channels.ReleaseChannel, "channel_global", "channel_identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.release("a") // never acquired
			if ok, _ := tc.acquire("a"); !ok {
				t.Fatal("first slot refused")
			}
			if ok, scope := tc.acquire("a"); ok || scope != tc.idScope {
				t.Fatalf("identity cap lifted by an unmatched release (ok=%v scope=%q)", ok, scope)
			}
			if ok, _ := tc.acquire("b"); !ok {
				t.Fatal("second global slot refused")
			}
			if ok, scope := tc.acquire("c"); ok || scope != tc.globalScope {
				t.Fatalf("global cap lifted by an unmatched release (ok=%v scope=%q)", ok, scope)
			}
			tc.release("a")
			tc.release("a") // double release
			if ok, _ := tc.acquire("c"); !ok {
				t.Fatal("released slot not reusable")
			}
			if ok, scope := tc.acquire("d"); ok || scope != tc.globalScope {
				t.Fatalf("global cap lifted by a double release (ok=%v scope=%q)", ok, scope)
			}
		})
	}
}

// w22/m623: sandbox streams hold two pools of their own. By default a caller
// is bounded only by the caller pool, bex-api's own streams by theirs, 5 per
// key, and neither pool's streams take or free the other's slots.
func TestSandboxStreamPools(t *testing.T) {
	limits := NewSandboxStreamLimits(0, 0, 0, 0)
	var ends []func(string)
	for i := range 100 {
		end, ok := limits.Admit(nil, "a", false)
		if !ok {
			t.Fatalf("caller stream %d refused within the default pool of 100", i+1)
		}
		ends = append(ends, end)
	}
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	if _, ok := limits.Admit(metrics, "b", false); ok {
		t.Fatal("a caller stream past the pool was admitted")
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_limit_rejections_total", map[string]string{"scope": "sandbox_global"}); got != 1 {
		t.Fatalf("limit_rejections_total{sandbox_global} = %v, want 1", got)
	}
	// A full caller pool leaves the platform pool whole: 5 per key, 32 in all.
	for n := range 32 {
		if n == 5 {
			if _, ok := limits.Admit(metrics, "system:0", true); ok {
				t.Fatal("a sixth stream for one platform key was admitted")
			}
		}
		end, ok := limits.Admit(nil, fmt.Sprintf("system:%d", n/5), true)
		if !ok {
			t.Fatalf("platform stream %d of key %d refused", n%5+1, n/5)
		}
		ends = append(ends, end)
	}
	if _, ok := limits.Admit(metrics, "system:9", true); ok {
		t.Fatal("a platform stream past its pool of 32 was admitted")
	}
	for _, scope := range []string{"sandbox_platform_identity", "sandbox_platform_global"} {
		if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_limit_rejections_total", map[string]string{"scope": scope}); got != 1 {
			t.Fatalf("limit_rejections_total{%s} = %v, want 1", scope, got)
		}
	}
	// Ending a platform stream, even twice, frees one platform slot and no
	// caller slot.
	platformEnd := ends[len(ends)-1]
	platformEnd("closed")
	platformEnd("closed")
	if _, ok := limits.Admit(nil, "b", false); ok {
		t.Fatal("ending a platform stream freed a caller slot")
	}
	if _, ok := limits.Admit(nil, "system:9", true); !ok {
		t.Fatal("the ended platform stream's slot was not freed")
	}
	if _, ok := limits.Admit(nil, "system:8", true); ok {
		t.Fatal("an end called twice freed two platform slots")
	}
	// And the reverse.
	ends[0]("closed")
	if _, ok := limits.Admit(nil, "system:8", true); ok {
		t.Fatal("ending a caller stream freed a platform slot")
	}
	if _, ok := limits.Admit(nil, "b", false); !ok {
		t.Fatal("the ended caller stream's slot was not freed")
	}
}

// A per-identity bound set below the caller pool binds first, and a caller
// at it leaves the pool to everyone else.
func TestSandboxStreamPerIdentityBound(t *testing.T) {
	limits := NewSandboxStreamLimits(4, 2, 0, 0)
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	for range 2 {
		if _, ok := limits.Admit(metrics, "a", false); !ok {
			t.Fatal("a stream within the per-identity bound was refused")
		}
	}
	if _, ok := limits.Admit(metrics, "a", false); ok {
		t.Fatal("a third stream for one identity was admitted past its bound of 2")
	}
	if got := gatewaytest.MetricValue(t, registry, "bex_ssh_gateway_limit_rejections_total", map[string]string{"scope": "sandbox_identity"}); got != 1 {
		t.Fatalf("limit_rejections_total{sandbox_identity} = %v, want 1", got)
	}
	for range 2 {
		if _, ok := limits.Admit(metrics, "b", false); !ok {
			t.Fatal("another caller was refused while the pool had room")
		}
	}
}
