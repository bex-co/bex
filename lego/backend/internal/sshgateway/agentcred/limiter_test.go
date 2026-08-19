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

package agentcred

import "testing"

// TestDefaultLimiterIsShared is the regression for a cap that silently counted
// nothing: limiter() minted a FRESH SessionLimiter per call, so acquire and
// release landed on different instances and no request ever observed another's
// slot.
func TestDefaultLimiterIsShared(t *testing.T) {
	b := &Broker{}
	if b.limiter() != b.limiter() {
		t.Fatal("limiter() returned a different instance per call: acquire and release cannot pair, so the default cap never counts")
	}
}

// TestDefaultLimiterCapsPerSource proves the consequence, not just the
// identity: one source holding its share must shed, and it must shed on the
// per-source dimension rather than by exhausting the global pool — a regression
// that dropped the per-source cap would still shed eventually, just far too
// late and for every other tenant too.
func TestDefaultLimiterCapsPerSource(t *testing.T) {
	b := &Broker{}
	const source = "10.1.2.3"

	for i := range defaultMaxConnsPerPod {
		if ok, scope := b.limiter().Acquire(source); !ok {
			t.Fatalf("acquire %d of %d refused (scope %q)", i+1, defaultMaxConnsPerPod, scope)
		}
	}
	ok, scope := b.limiter().Acquire(source)
	if ok {
		t.Fatalf("a %dth concurrent request from one source was admitted", defaultMaxConnsPerPod+1)
	}
	if scope != "identity" {
		t.Fatalf("shed on scope %q, want the per-source dimension", scope)
	}

	// Releasing one slot must let exactly one more request in — the pairing the
	// fresh-instance bug broke.
	b.limiter().Release(source)
	if ok, _ := b.limiter().Acquire(source); !ok {
		t.Fatal("a released slot was not reusable: release landed on a different limiter than acquire")
	}
}
