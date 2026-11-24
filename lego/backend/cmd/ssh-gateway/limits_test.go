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

package main

import (
	"fmt"
	"testing"
)

// w22/m623 at the process's wiring: one subject's sandbox exec streams and
// file transfers run past 5 at once on one gateway, while that subject's
// sixth native SSH session or web shell (the session limiter they share) is
// still refused, and sandbox streams leave the session limiter's 100 slots to
// those transports.
func TestSandboxStreamsDoNotSpendSSHSessionSlots(t *testing.T) {
	for _, name := range []string{
		"BEX_SSH_MAX_SESSIONS", "BEX_SSH_MAX_SESSIONS_PER_IDENTITY",
		"BEX_SANDBOX_MAX_STREAMS", "BEX_SANDBOX_MAX_STREAMS_PER_IDENTITY",
		"BEX_SANDBOX_MAX_PLATFORM_STREAMS", "BEX_SANDBOX_MAX_PLATFORM_STREAMS_PER_KEY",
	} {
		t.Setenv(name, "")
	}
	sessions, sandbox := gatewayLimits()
	for i := range 100 {
		if _, ok := sandbox.Admit(nil, "id-a", false); !ok {
			t.Fatalf("sandbox stream %d of one subject refused within the default pool", i+1)
		}
	}
	for i := range 5 {
		if ok, scope := sessions.Acquire("id-a"); !ok {
			t.Fatalf("SSH session %d refused behind 100 sandbox streams (%s)", i+1, scope)
		}
	}
	if ok, scope := sessions.Acquire("id-a"); ok || scope != "identity" {
		t.Fatalf("sixth SSH session of one subject: ok=%v scope=%q, want refused by identity", ok, scope)
	}
	for i := range 95 {
		if ok, scope := sessions.Acquire(fmt.Sprintf("id-%d", i)); !ok {
			t.Fatalf("SSH session %d of 100 refused (%s): sandbox streams took the slot", i+6, scope)
		}
	}
	if ok, scope := sessions.Acquire("id-z"); ok || scope != "global" {
		t.Fatalf("SSH session past 100: ok=%v scope=%q, want refused globally", ok, scope)
	}
}

// The bounds are the gateway's knobs: a per-identity bound on sandbox
// streams is opt-in, below the pool it binds first.
func TestSandboxStreamLimitsReadTheirEnvironment(t *testing.T) {
	t.Setenv("BEX_SANDBOX_MAX_STREAMS", "3")
	t.Setenv("BEX_SANDBOX_MAX_STREAMS_PER_IDENTITY", "2")
	t.Setenv("BEX_SANDBOX_MAX_PLATFORM_STREAMS", "2")
	t.Setenv("BEX_SANDBOX_MAX_PLATFORM_STREAMS_PER_KEY", "1")
	_, sandbox := gatewayLimits()
	for range 2 {
		if _, ok := sandbox.Admit(nil, "id-a", false); !ok {
			t.Fatal("a stream within BEX_SANDBOX_MAX_STREAMS_PER_IDENTITY was refused")
		}
	}
	if _, ok := sandbox.Admit(nil, "id-a", false); ok {
		t.Fatal("BEX_SANDBOX_MAX_STREAMS_PER_IDENTITY was not read")
	}
	if _, ok := sandbox.Admit(nil, "id-b", false); !ok {
		t.Fatal("another subject was refused with room in the pool")
	}
	if _, ok := sandbox.Admit(nil, "id-c", false); ok {
		t.Fatal("BEX_SANDBOX_MAX_STREAMS was not read")
	}
	if _, ok := sandbox.Admit(nil, "system:a", true); !ok {
		t.Fatal("the platform pool was refused")
	}
	if _, ok := sandbox.Admit(nil, "system:a", true); ok {
		t.Fatal("BEX_SANDBOX_MAX_PLATFORM_STREAMS_PER_KEY was not read")
	}
	if _, ok := sandbox.Admit(nil, "system:b", true); !ok {
		t.Fatal("another platform key was refused with room in its pool")
	}
	if _, ok := sandbox.Admit(nil, "system:c", true); ok {
		t.Fatal("BEX_SANDBOX_MAX_PLATFORM_STREAMS was not read")
	}
}
