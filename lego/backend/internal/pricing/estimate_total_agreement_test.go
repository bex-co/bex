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

// estimate_total_agreement_test.go pins the usage estimate's agreement
// invariant: TotalUSD is exactly the sum of the per-resource figures, so the
// bill always adds up. Rounding the raw float sum instead drifts a cent from
// what the resource lines show whenever the fractional cents straddle a
// rounding boundary (the same drift MonthlyEstimate already sums away).

import (
	"math/rand"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A four-line stack whose fractional cents round the raw grand sum up while
// the per-resource figures round down: the old total read $4.49 for $4.48 of
// lines.
func TestEstimateTotalEqualsSumOfResources(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{ServiceID: "b", Kind: store.UsageKindCronMinimumMicroUSD, ResourceKind: store.ResourceKindService, Total: 1934401},
		{ServiceID: "a", Kind: store.UsageKindAgentModelCostMicroUSD, ResourceKind: store.ResourceKindAgentSession, Total: 714612},
		{ServiceID: "c", Kind: store.UsageKindAgentModelCostMicroUSD, ResourceKind: store.ResourceKindAgentSession, Total: 1191896},
		{ServiceID: "c", Kind: store.UsageKindBuildSeconds, ResourceKind: store.ResourceKindService, Total: 11145},
	}
	got := Default.Estimate(rows)
	if got.TotalUSD != "4.48" {
		t.Errorf("TotalUSD = %s, want 4.48", got.TotalUSD)
	}
	var sum int64
	for _, r := range got.Resources {
		sum += monthlyCents(t, r.CostUSD)
	}
	if total := monthlyCents(t, got.TotalUSD); total != sum {
		t.Errorf("TotalUSD %s != sum of resource figures (%d cents): %+v", got.TotalUSD, sum, got.Resources)
	}
}

// The meter breakdown shows the same total: it is the other reader-visible
// decomposition of TotalUSD, so its lines must add to it exactly like the
// resource figures do. Rounding each meter's aggregate can drift a cent from
// the total the resource lines sum to: the stack below meters at $4.49 for a
// $4.48 total (the two agent-model rows meter $1.91 together but $0.71 and
// $1.19 apart).
func TestEstimateMetersEqualTotal(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{ServiceID: "b", Kind: store.UsageKindCronMinimumMicroUSD, ResourceKind: store.ResourceKindService, Total: 1934401},
		{ServiceID: "a", Kind: store.UsageKindAgentModelCostMicroUSD, ResourceKind: store.ResourceKindAgentSession, Total: 714612},
		{ServiceID: "c", Kind: store.UsageKindAgentModelCostMicroUSD, ResourceKind: store.ResourceKindAgentSession, Total: 1191896},
		{ServiceID: "c", Kind: store.UsageKindBuildSeconds, ResourceKind: store.ResourceKindService, Total: 11145},
	}
	got := Default.Estimate(rows)
	var sum int64
	for _, m := range got.Meters {
		sum += monthlyCents(t, m.CostUSD)
	}
	if total := monthlyCents(t, got.TotalUSD); total != sum {
		t.Errorf("TotalUSD %s != sum of meter lines (%d cents): %+v", got.TotalUSD, sum, got.Meters)
	}
}

// The total is the sum of the displayed resource figures, to the cent, with
// and without plan allowances: a fixed-seed sweep over meter kinds,
// resources, and quantities.
func TestEstimateTotalEqualsSumOfResourcesProperty(t *testing.T) {
	kinds := []struct {
		kind, tier, rk string
		maxQty         int64
	}{
		{store.UsageKindInstanceSeconds, "starter", store.ResourceKindService, 2_628_000},
		{store.UsageKindInstanceSeconds, "standard", store.ResourceKindService, 2_628_000},
		{store.UsageKindEgressBytes, "", store.ResourceKindService, 50 * 1 << 30},
		{store.UsageKindBuildSeconds, "", store.ResourceKindService, 36000},
		{store.UsageKindStorageGBSeconds, "", store.ResourceKindPostgres, 100 * MonthSeconds},
		{store.UsageKindDiskGBSeconds, "", store.ResourceKindService, 100 * MonthSeconds},
		{store.UsageKindAgentModelCostMicroUSD, "", store.ResourceKindAgentSession, 2000000},
		{store.UsageKindCronMinimumMicroUSD, "", store.ResourceKindService, 2000000},
		{store.UsageKindCustomDomainSeconds, "", store.ResourceKindCustomDomains, 3 * MonthSeconds},
	}
	allowance := Included{
		store.UsageKindBuildSeconds: Default.IncludedFor(store.PlanHobby)[store.UsageKindBuildSeconds],
		store.UsageKindEgressBytes:  Default.IncludedFor(store.PlanHobby)[store.UsageKindEgressBytes],
	}
	rng := rand.New(rand.NewSource(1))
	for iter := 0; iter < 5000; iter++ {
		var rows []store.UsageSummaryRow
		for i, n := 0, 1+rng.Intn(4); i < n; i++ {
			k := kinds[rng.Intn(len(kinds))]
			rows = append(rows, store.UsageSummaryRow{
				ServiceID:    string(rune('a' + rng.Intn(3))),
				Kind:         k.kind,
				Tier:         k.tier,
				ResourceKind: k.rk,
				Total:        rng.Int63n(k.maxQty + 1),
			})
		}
		included := Included(nil)
		if iter%2 == 0 {
			included = allowance
		}
		got := Default.EstimateIncluding(rows, included)
		var sum int64
		for _, r := range got.Resources {
			sum += monthlyCents(t, r.CostUSD)
		}
		if total := monthlyCents(t, got.TotalUSD); total != sum {
			t.Fatalf("iter %d: total %s != resources sum %d cents\nrows=%+v\nres=%+v", iter, got.TotalUSD, sum, rows, got.Resources)
		}
	}
}

// The meter lines add to the same total, to the cent, with and without plan
// allowances: the same fixed-seed sweep, asserting the second decomposition.
func TestEstimateMetersEqualTotalProperty(t *testing.T) {
	kinds := []struct {
		kind, tier, rk string
		maxQty         int64
	}{
		{store.UsageKindInstanceSeconds, "starter", store.ResourceKindService, 2_628_000},
		{store.UsageKindInstanceSeconds, "standard", store.ResourceKindService, 2_628_000},
		{store.UsageKindEgressBytes, "", store.ResourceKindService, 50 * 1 << 30},
		{store.UsageKindBuildSeconds, "", store.ResourceKindService, 36000},
		{store.UsageKindStorageGBSeconds, "", store.ResourceKindPostgres, 100 * MonthSeconds},
		{store.UsageKindDiskGBSeconds, "", store.ResourceKindService, 100 * MonthSeconds},
		{store.UsageKindAgentModelCostMicroUSD, "", store.ResourceKindAgentSession, 2000000},
		{store.UsageKindCronMinimumMicroUSD, "", store.ResourceKindService, 2000000},
		{store.UsageKindCustomDomainSeconds, "", store.ResourceKindCustomDomains, 3 * MonthSeconds},
	}
	allowance := Included{
		store.UsageKindBuildSeconds: Default.IncludedFor(store.PlanHobby)[store.UsageKindBuildSeconds],
		store.UsageKindEgressBytes:  Default.IncludedFor(store.PlanHobby)[store.UsageKindEgressBytes],
	}
	rng := rand.New(rand.NewSource(2))
	for iter := 0; iter < 5000; iter++ {
		var rows []store.UsageSummaryRow
		for i, n := 0, 1+rng.Intn(4); i < n; i++ {
			k := kinds[rng.Intn(len(kinds))]
			rows = append(rows, store.UsageSummaryRow{
				ServiceID:    string(rune('a' + rng.Intn(3))),
				Kind:         k.kind,
				Tier:         k.tier,
				ResourceKind: k.rk,
				Total:        rng.Int63n(k.maxQty + 1),
			})
		}
		included := Included(nil)
		if iter%2 == 0 {
			included = allowance
		}
		got := Default.EstimateIncluding(rows, included)
		var sum int64
		for _, m := range got.Meters {
			sum += monthlyCents(t, m.CostUSD)
		}
		if total := monthlyCents(t, got.TotalUSD); total != sum {
			t.Fatalf("iter %d: total %s != meters sum %d cents\nrows=%+v\nmeters=%+v", iter, got.TotalUSD, sum, rows, got.Meters)
		}
	}
}
