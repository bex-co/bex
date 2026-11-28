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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/m713: a paid 10 GB Postgres with about 2 GB used, present all of a
// 730-hour month, pays $2.10 on its capacity line (10 GB × Render's $0.30 ×
// 0.70) and $0 on its used-bytes reading, which the reads mark Unbilled from
// the switch. No hour is priced on both.
func TestCapacityPricesThePostgresStorage(t *testing.T) {
	const hours = 730
	est := Default.Estimate([]store.UsageSummaryRow{
		{ServiceID: "dpg-db", ResourceKind: store.ResourceKindPostgres, Kind: store.UsageKindStorageCapacityGBSeconds, Tier: "basic-1gb", Total: 36000 * hours},
		{ServiceID: "dpg-db", ResourceKind: store.ResourceKindPostgres, Kind: store.UsageKindStorageGBSeconds, Tier: "basic-1gb", Total: 7200 * hours, Unbilled: true},
	})
	if est.TotalUSD != "2.10" {
		t.Fatalf("total = %s, want 2.10", est.TotalUSD)
	}
	if len(est.Resources) != 1 || len(est.Resources[0].Charges) != 2 {
		t.Fatalf("resources = %+v", est.Resources)
	}
	capacity, used := est.Resources[0].Charges[0], est.Resources[0].Charges[1]
	if capacity.Kind != store.UsageKindStorageCapacityGBSeconds || capacity.Unit != "GB-mo" || capacity.RateUSD != "0.21" ||
		capacity.Quantity != "10.00" || capacity.CostUSD != "2.10" || capacity.Unbilled {
		t.Errorf("capacity line = %+v, want 10 GB-mo at $0.21 = $2.10", capacity)
	}
	if used.Kind != store.UsageKindStorageGBSeconds || used.RateUSD != "0" || used.Quantity != "2.00" || used.CostUSD != "0.00" || !used.Unbilled {
		t.Errorf("used line = %+v, want 2 GB-mo shown at $0", used)
	}
	if len(est.Meters) != 1 || est.Meters[0].Kind != store.UsageKindStorageCapacityGBSeconds {
		t.Errorf("meters = %+v, want the capacity alone", est.Meters)
	}
	if est.StorageCapacityBilledFrom != Default.StorageCapacityBilledFrom().Format(time.RFC3339) {
		t.Errorf("storageCapacityBilledFrom = %q, want the sheet's switch", est.StorageCapacityBilledFrom)
	}
}

// Free Postgres capacity is included at $0 (w4/427), a Key Value has no
// storage capacity rate, and a period without datastore storage names no
// switch.
func TestCapacityRates(t *testing.T) {
	const gbMonth = 730 * 3600
	for _, tt := range []struct {
		resourceKind, tier, want string
	}{
		{store.ResourceKindPostgres, "basic-256mb", "0.21"},
		{store.ResourceKindPostgres, "pro-4gb", "0.21"},
		{store.ResourceKindPostgres, "free", "0.00"},
		{store.ResourceKindKeyValue, "starter", "0.00"},
		{store.ResourceKindService, "starter", "0.00"},
	} {
		est := Default.Estimate([]store.UsageSummaryRow{{ServiceID: "x", ResourceKind: tt.resourceKind, Kind: store.UsageKindStorageCapacityGBSeconds, Tier: tt.tier, Total: gbMonth}})
		if est.TotalUSD != tt.want {
			t.Errorf("%s %s capacity GB-month = %s, want %s", tt.resourceKind, tt.tier, est.TotalUSD, tt.want)
		}
	}
	if est := Default.Estimate([]store.UsageSummaryRow{{ServiceID: "srv", ResourceKind: store.ResourceKindService,
		Kind: store.UsageKindInstanceSeconds, Tier: "starter", Total: 3600}}); est.StorageCapacityBilledFrom != "" {
		t.Errorf("a period without datastore storage names the switch %q", est.StorageCapacityBilledFrom)
	}
}

// The switch is a UTC month start, so every month, compacted or not, bills on
// one storage basis; the shipped sheet switches at 2026-12-01.
func TestStorageCapacityBilledFromIsAMonthStart(t *testing.T) {
	if got, want := Default.StorageCapacityBilledFrom(), time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("StorageCapacityBilledFrom = %s, want %s", got, want)
	}
	const line = `billedFrom: "2026-12-01T00:00:00Z"`
	if !strings.Contains(string(sheetYAML), "storageCapacity:\n  "+line) {
		t.Fatal("pricing.yaml no longer carries the switch this test rewrites")
	}
	for _, bad := range []string{"2026-12-01T01:00:00Z", "2026-12-15T00:00:00Z", "2026-12-01T00:00:00+01:00", "December"} {
		yml := strings.Replace(string(sheetYAML), line, `billedFrom: "`+bad+`"`, 1)
		if _, err := parseSheet([]byte(yml)); err == nil || !strings.Contains(err.Error(), "storageCapacity.billedFrom") {
			t.Errorf("billedFrom %q parsed: %v", bad, err)
		}
	}
	never, err := parseSheet([]byte(strings.Replace(string(sheetYAML), "storageCapacity:\n  "+line+"\n", "", 1)))
	if err != nil || !never.StorageCapacityBilledFrom().IsZero() {
		t.Errorf("a sheet without the switch = %v, %v; want it never to switch", never.StorageCapacityBilledFrom(), err)
	}
}
