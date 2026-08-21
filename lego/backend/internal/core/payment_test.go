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

package core

import (
	"context"
	"errors"
	"testing"
)

type recordingPaymentGate struct{ calls int }

func (g *recordingPaymentGate) RequirePaymentMethod(context.Context, string) error {
	g.calls++
	return NewPaymentRequiredError()
}

// TestRequirePlanBillingModeMatrix pins the ONE seam every billable
// create/plan change runs (ADR046 §2 + ADR075 D7 w6/m42): the payment marker
// is consulted for a paid plan always, for a free plan only under
// PaymentAllPlans, and never when no gate is wired — the exact contract the
// cmd/api mode parse maps BEX_REQUIRE_PAYMENT_METHOD onto.
func TestRequirePlanBillingModeMatrix(t *testing.T) {
	for _, tc := range []struct {
		name     string
		allPlans bool
		plan     string
		want402  bool
	}{
		{"paid plan, paid-intent mode", false, "starter", true},
		{"free plan, paid-intent mode", false, "free", false},
		{"paid plan, all mode", true, "starter", true},
		{"free plan, all mode", true, "free", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := &recordingPaymentGate{}
			b := &Base{Payment: gate, PaymentAllPlans: tc.allPlans}
			err := b.RequirePlanBilling(context.Background(), "tea-x", tc.plan)
			if tc.want402 {
				if !errors.Is(err, ErrPaymentRequired) || gate.calls != 1 {
					t.Fatalf("err=%v calls=%d, want 402 with one gate consult", err, gate.calls)
				}
			} else if err != nil || gate.calls != 0 {
				t.Fatalf("err=%v calls=%d, want pass with no gate consult", err, gate.calls)
			}
		})
	}

	// No gate wired (BEX_REQUIRE_PAYMENT_METHOD unset): PaymentAllPlans is
	// meaningless and every plan passes — the store-off compatibility path.
	b := &Base{PaymentAllPlans: true}
	if err := b.RequirePlanBilling(context.Background(), "tea-x", "starter"); err != nil {
		t.Fatalf("nil-gate paid plan: %v", err)
	}
}
