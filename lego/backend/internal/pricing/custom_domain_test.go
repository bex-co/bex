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
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w7/m429 (docs/ADR030 §16): a custom domain above the plan's included count
// is Render's $0.25 a month × 0.70 = $0.175 a domain-month, prorated over the
// 730-hour pricing month, and it bills on the custom_domain_hours meter.
func TestCustomDomainRateIsRendersAtTheStandingDiscount(t *testing.T) {
	const domainMonth = 730 * 3600
	monthly := Default.customDomain * domainMonth
	if diff := monthly - 0.25*0.70; diff > 1e-12 || diff < -1e-12 {
		t.Fatalf("custom domain = $%v a domain-month, want $0.175", monthly)
	}
	// An hour of one domain is 1/730 of a month.
	hour := Default.Estimate([]store.UsageSummaryRow{{ServiceID: "tea-1", ResourceKind: store.ResourceKindCustomDomains,
		Kind: store.UsageKindCustomDomainSeconds, Total: 3600 * 730 * 4}})
	if hour.TotalUSD != "0.70" {
		t.Fatalf("four domain-months = $%s, want $0.70", hour.TotalUSD)
	}
	// Only the custom domain resource is priced at this rate.
	other := Default.Estimate([]store.UsageSummaryRow{{ServiceID: "srv-1", ResourceKind: store.ResourceKindService,
		Kind: store.UsageKindCustomDomainSeconds, Total: domainMonth * 10}})
	if other.TotalUSD != "0.00" {
		t.Fatalf("custom domain seconds on a service resource priced $%s", other.TotalUSD)
	}
	if !slices.Contains(Default.BillableMeterNames(), CustomDomainMeter) {
		t.Fatalf("BillableMeterNames lacks %s", CustomDomainMeter)
	}
}

func TestCustomDomainRateMustBeNonNegative(t *testing.T) {
	yml := strings.Replace(string(sheetYAML), "usdPerDomainSecond: 0.000000066590563165905631659056", "usdPerDomainSecond: -1", 1)
	if yml == string(sheetYAML) {
		t.Fatal("pricing.yaml no longer carries the custom domain rate this test rewrites")
	}
	if _, err := parseSheet([]byte(yml)); err == nil || !strings.Contains(err.Error(), "customDomains") {
		t.Fatalf("negative custom domain rate parsed: %v", err)
	}
}
