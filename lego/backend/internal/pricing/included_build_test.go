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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w6/m308 (docs/ADR030 §13): each plan includes Render's pipeline minutes, and
// only the minutes above them are priced.

func TestIncludedBuildSecondsFollowsRendersPlans(t *testing.T) {
	for plan, minutes := range map[string]int64{
		store.PlanHobby: 500, store.PlanPro: 1000, store.PlanScale: 5000, store.PlanEnterprise: 5000, "": 0, "legacy": 0,
	} {
		if got := Default.IncludedBuildSeconds(plan); got != minutes*60 {
			t.Errorf("IncludedBuildSeconds(%q) = %d, want %d", plan, got, minutes*60)
		}
	}
}

func buildRow(service string, minutes int64) store.UsageSummaryRow {
	return store.UsageSummaryRow{ServiceID: service, Kind: store.UsageKindBuildSeconds, ResourceKind: store.ResourceKindService, Total: minutes * 60}
}

func TestEstimateIncludingPricesOnlyMinutesAboveTheAllowance(t *testing.T) {
	hobby := Default.IncludedBuildSeconds(store.PlanHobby)

	under := Default.EstimateIncluding([]store.UsageSummaryRow{buildRow("srv-a", 400)}, hobby)
	if under.TotalUSD != "0.00" || len(under.Meters) != 0 {
		t.Errorf("400 of 500 included minutes = %+v, want nothing charged", under)
	}
	line := under.Resources[0].Charges[0]
	if line.Quantity != "400.00" || line.Included != "400.00" || line.CostUSD != "0.00" {
		t.Errorf("covered line = %+v, want quantity 400, included 400, $0", line)
	}

	// 600 minutes across two services: the allowance covers the first 500 in
	// row order, and 100 minutes × $0.0035 are charged.
	over := Default.EstimateIncluding([]store.UsageSummaryRow{buildRow("srv-a", 450), buildRow("srv-b", 150)}, hobby)
	if over.TotalUSD != "0.35" {
		t.Errorf("total = %s, want 0.35", over.TotalUSD)
	}
	a, b := over.Resources[0].Charges[0], over.Resources[1].Charges[0]
	if a.Included != "450.00" || a.CostUSD != "0.00" || b.Quantity != "150.00" || b.Included != "50.00" || b.CostUSD != "0.35" {
		t.Errorf("lines = %+v / %+v, want srv-a fully covered, srv-b 50 of 150 covered and $0.35", a, b)
	}

	// No plan, no allowance: the plain estimate.
	if got, want := Default.EstimateIncluding([]store.UsageSummaryRow{buildRow("srv-a", 600)}, 0), Default.Estimate([]store.UsageSummaryRow{buildRow("srv-a", 600)}); got.TotalUSD != want.TotalUSD || got.TotalUSD != "2.10" {
		t.Errorf("no allowance = %s, want %s (600 × $0.0035)", got.TotalUSD, want.TotalUSD)
	}
}

func TestIncludedBuildMinutesCatalogIsValidated(t *testing.T) {
	base := string(sheetYAML)
	for name, edit := range map[string][2]string{
		"unknown plan":     {"    - plan: enterprise\n      minutes: 5000", "    - plan: platinum\n      minutes: 5000"},
		"negative minutes": {"    - plan: hobby\n      minutes: 500", "    - plan: hobby\n      minutes: -1"},
	} {
		raw := strings.Replace(base, edit[0], edit[1], 1)
		if raw == base {
			t.Fatalf("%s: fixture edit did not apply", name)
		}
		if _, err := parseSheet([]byte(raw)); err == nil {
			t.Errorf("%s: parseSheet accepted it", name)
		}
	}
}
