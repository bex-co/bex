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

package pricing

import (
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ADR091 D5: the shipped plan defaults and the 10× override bound;
// enterprise (no row) takes the top cap.
func TestAgentModelPlanSpendCaps(t *testing.T) {
	for plan, want := range map[string]float64{
		store.PlanHobby: 20, store.PlanPro: 200, store.PlanScale: 2000,
		store.PlanEnterprise: 2000, "future-plan": 2000,
	} {
		if got := Default.AgentModelPlanSpendCapMicroUSD(plan); got != USDToMicro(want) {
			t.Errorf("%s cap = %v, want $%v", plan, got, want)
		}
	}
	if got := Default.AgentModelMaxCapMicroUSD(store.PlanHobby); got != USDToMicro(200) || MicroToUSD(got) != 200 {
		t.Fatalf("hobby max override = %v, want $200", got)
	}
}

// ADR091 D10.1 acceptance: every tier boundary (lifetime paid and paid cycle
// invoices) lands in the right tier, and a lock pins T0 until it expires.
func TestTrustTierBoundaries(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		cents    int64
		invoices int
		want     string
	}{
		{0, 0, "T0"},
		{999, 0, "T0"},
		{999, 10, "T0"},
		{1000, 0, "T1"},
		{9999, 5, "T1"},
		{10000, 0, "T1"}, // $100 but no paid cycle invoice yet
		{10000, 1, "T2"},
		{49999, 3, "T2"},
		{50000, 2, "T2"},
		{50000, 3, "T3"},
		{199999, 6, "T3"},
		{200000, 5, "T3"},
		{200000, 6, "T4"},
		{1 << 40, 100, "T4"},
	} {
		if got := TrustTier(tc.cents, tc.invoices, time.Time{}, now); got.Name != tc.want {
			t.Errorf("TrustTier(%d¢, %d invoices) = %s, want %s", tc.cents, tc.invoices, got.Name, tc.want)
		}
	}
	// Locked: T0 until lockedUntil, the earned tier from that instant on.
	until := now.Add(time.Hour)
	if got := TrustTier(200000, 6, until, now); got.Name != "T0" {
		t.Fatalf("locked = %s, want T0", got.Name)
	}
	if got := TrustTier(200000, 6, until, until); got.Name != "T4" {
		t.Fatalf("at lock expiry = %s, want T4", got.Name)
	}
}

// The tier table carries the D10.1 limits: T0's $10 threshold, $10 cap
// ceiling and 2 sessions; thresholds rising 25/50/250; none at T4.
func TestTrustTierLimits(t *testing.T) {
	tiers := Default.TrustTiers()
	if len(tiers) != 5 {
		t.Fatalf("tiers = %d, want T0..T4", len(tiers))
	}
	wantThreshold := []float64{10, 25, 50, 250, 0}
	for i, tier := range tiers {
		got := 0.0
		if tier.ChargeThresholdUSD != nil {
			got = *tier.ChargeThresholdUSD
		}
		if got != wantThreshold[i] || tier.Level != i {
			t.Errorf("%s threshold = %v level %d, want %v level %d", tier.Name, got, tier.Level, wantThreshold[i], i)
		}
		if i > 0 && (tier.CapCeilingMicroUSD != nil || tier.MaxConcurrentSessions != 0) {
			t.Errorf("%s must not lower the plan cap or concurrency", tier.Name)
		}
	}
	t0 := tiers[0]
	// T0 is charged at $10 in the request that reaches it, and has no
	// in-flight allowance: paying lifts the tier at once, and unpaid it stops
	// at $10 (ADR091 D10.2a).
	if t0.CapCeilingMicroUSD == nil || *t0.CapCeilingMicroUSD != USDToMicro(10) || t0.MaxConcurrentSessions != 2 {
		t.Fatalf("T0 = %+v, want a $10 ceiling ($10 threshold, no allowance) and 2 sessions", t0)
	}
	pro := Default.AgentModelPlanSpendCapMicroUSD(store.PlanPro)
	if got := t0.EffectiveCapMicroUSD(pro); got != USDToMicro(10) {
		t.Fatalf("T0 Pro cap = %v, want $10", got)
	}
	if got := tiers[1].EffectiveCapMicroUSD(pro); got != pro {
		t.Fatalf("T1 Pro cap = %v, want the plan's %v", got, pro)
	}
}
