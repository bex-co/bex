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

import "testing"

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
