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

package billing_test

import (
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/usage"
)

// TestClampSealHoursKeepsExportedRowsFinal is w7/m57's regression guard for the
// seal-window/rollup-catch-up decoupling. A seal horizon shorter than the usage
// rollup's rewrite window would let a row be exported to Stripe and then rewritten
// by a later catch-up rollup (silent over/under-bill). ClampSealHours makes the
// two windows non-overlapping by construction.
func TestClampSealHoursKeepsExportedRowsFinal(t *testing.T) {
	cw := usage.CatchupWindow
	for _, tc := range []struct {
		name       string
		configured time.Duration
		want       time.Duration
	}{
		{"below-window-is-raised", cw - time.Hour, cw},
		{"far-below-window-is-raised", time.Hour, cw},
		{"at-window-is-kept", cw, cw},
		{"above-window-is-kept", cw + 24*time.Hour, cw + 24*time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := billing.ClampSealHours(tc.configured, cw); got != tc.want {
				t.Errorf("ClampSealHours(%s, %s) = %s, want %s", tc.configured, cw, got, tc.want)
			}
			// The invariant the fix guarantees: the effective seal is never shorter
			// than the rewrite window, so an exported row is provably un-rewritable.
			if billing.ClampSealHours(tc.configured, cw) < cw {
				t.Errorf("effective seal horizon fell inside the rollup rewrite window")
			}
		})
	}
}

// TestDefaultSealHoursCoversCatchupWindow pins the safe-by-default relationship so
// a future change to either constant that reintroduced the overlap fails CI.
func TestDefaultSealHoursCoversCatchupWindow(t *testing.T) {
	if billing.DefaultSealHours < usage.CatchupWindow {
		t.Fatalf("DefaultSealHours (%s) is shorter than usage.CatchupWindow (%s) — an exported row could be rewritten",
			billing.DefaultSealHours, usage.CatchupWindow)
	}
}
