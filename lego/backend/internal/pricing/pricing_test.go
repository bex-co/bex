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
	"fmt"
	"math"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// parseUSD parses a "$X.YY"-style string to float64.
func parseUSD(t *testing.T, s string) float64 {
	t.Helper()
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		t.Fatalf("parseUSD(%q): %v", s, err)
	}
	return v
}

// --- sheet load tests ---

func TestDefaultSheetLoadsWithoutPanic(t *testing.T) {
	if Default == nil {
		t.Fatal("Default sheet is nil after init")
	}
}

func TestParseSheetRejectsEmptyVersion(t *testing.T) {
	if _, err := parseSheet([]byte(`{}`)); err == nil {
		t.Error("expected error for missing version field")
	}
}

func TestParseSheetRejectsEmptyTier(t *testing.T) {
	yml := []byte("version: \"1\"\ncompute:\n  - tier: \"\"\n    usdPerSecond: 0.0\n")
	if _, err := parseSheet(yml); err == nil {
		t.Error("expected error for empty tier in compute entry")
	}
}

// --- zero-usage tests ---

func TestEstimateNilRowsIsZero(t *testing.T) {
	est := Default.Estimate(nil)
	if est.TotalUSD != "0.00" {
		t.Errorf("nil rows: want totalUsd=0.00, got %q", est.TotalUSD)
	}
	if est.Meters == nil {
		t.Error("Meters must be non-nil even for zero usage")
	}
	if len(est.Meters) != 0 {
		t.Errorf("nil rows: want 0 meters, got %d", len(est.Meters))
	}
}

func TestEstimateZeroQuantityRow(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 0},
	}
	est := Default.Estimate(rows)
	if est.TotalUSD != "0.00" {
		t.Errorf("zero quantity: want 0.00, got %q", est.TotalUSD)
	}
	if len(est.Meters) != 0 {
		t.Errorf("zero quantity: want 0 meters, got %d", len(est.Meters))
	}
}

// --- unknown / unpriced tier ---

func TestEstimateUnknownTierIsZero(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "galaxy-brain-tier", ResourceKind: store.ResourceKindService, Total: 2628000},
	}
	est := Default.Estimate(rows)
	if est.TotalUSD != "0.00" {
		t.Errorf("unknown tier: want 0.00, got %q", est.TotalUSD)
	}
	// zero-cost entries are omitted from meters
	if len(est.Meters) != 0 {
		t.Errorf("unknown tier: want 0 meters (zero omitted), got %d", len(est.Meters))
	}
}

func TestEstimateFreeTierIsZero(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "free", ResourceKind: store.ResourceKindService, Total: 2628000},
	}
	est := Default.Estimate(rows)
	if est.TotalUSD != "0.00" {
		t.Errorf("free tier: want 0.00, got %q", est.TotalUSD)
	}
}

// --- rounding ---

func TestEstimateSubCentBandwidthRoundsToZero(t *testing.T) {
	// 1 byte of egress at $0.01/GB ≈ 9.31e-12 → rounds to $0.00
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindEgressBytes, Tier: "", ResourceKind: store.ResourceKindService, Total: 1},
	}
	est := Default.Estimate(rows)
	if est.TotalUSD != "0.00" {
		t.Errorf("1-byte bandwidth: want 0.00, got %q", est.TotalUSD)
	}
	// sub-cent meter omitted
	if len(est.Meters) != 0 {
		t.Errorf("sub-cent bandwidth: want 0 meter entries, got %d", len(est.Meters))
	}
}

func TestEstimateSubCentBuildRoundsToZero(t *testing.T) {
	// 1 second of build at $0.0035/min = $0.000058.../s → rounds to $0.00
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindBuildSeconds, Tier: "", ResourceKind: store.ResourceKindService, Total: 1},
	}
	est := Default.Estimate(rows)
	if est.TotalUSD != "0.00" {
		t.Errorf("1-second build: want 0.00, got %q", est.TotalUSD)
	}
}

// --- known tiers ---

func TestEstimateStarterOneMonth(t *testing.T) {
	// 1 month = 2,628,000 seconds → ~$4.90 (bex starter = Render $7 × 0.70)
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 2628000},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-4.90) > 0.02 {
		t.Errorf("starter 1 month: want ~$4.90, got %s", est.TotalUSD)
	}
	if len(est.Meters) != 1 {
		t.Fatalf("want 1 meter, got %d", len(est.Meters))
	}
	m := est.Meters[0]
	if m.Kind != store.UsageKindInstanceSeconds || m.Tier != "starter" || m.ResourceKind != store.ResourceKindService {
		t.Errorf("meter fields: %+v", m)
	}
}

func TestEstimateOneBandwidthGigabyte(t *testing.T) {
	// 1 GB (1,073,741,824 bytes) at $0.01/GB → $0.01
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindEgressBytes, Tier: "", ResourceKind: store.ResourceKindService, Total: 1073741824},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-0.01) > 0.001 {
		t.Errorf("1 GB bandwidth: want ~$0.01, got %s", est.TotalUSD)
	}
}

func TestEstimateOneBuildMinute(t *testing.T) {
	// 1 minute = 60 seconds at $0.0035/min → $0.0035 → rounds to $0.00
	// but at 17 minutes = 1020 seconds → $0.0595 → rounds to $0.06
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindBuildSeconds, Tier: "", ResourceKind: store.ResourceKindService, Total: 1020},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-0.06) > 0.01 {
		t.Errorf("17-min build: want ~$0.06, got %s", est.TotalUSD)
	}
}

// --- Postgres and KeyValue ---

func TestEstimatePostgresBasic256mb(t *testing.T) {
	// Same per-second rate as compute starter ($4.90/mo)
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "basic-256mb", ResourceKind: store.ResourceKindPostgres, Total: 2628000},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-4.90) > 0.02 {
		t.Errorf("postgres basic-256mb 1 month: want ~$4.90, got %s", est.TotalUSD)
	}
	if len(est.Meters) != 1 {
		t.Fatalf("want 1 meter, got %d", len(est.Meters))
	}
	if est.Meters[0].ResourceKind != store.ResourceKindPostgres {
		t.Errorf("resourceKind: want postgres, got %q", est.Meters[0].ResourceKind)
	}
}

func TestEstimateKeyValueStarterOneMonth(t *testing.T) {
	// starter KV = Render $10 × 0.70 = $7.00/mo
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindKeyValue, Total: 2628000},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-7.00) > 0.02 {
		t.Errorf("kv starter 1 month: want ~$7.00, got %s", est.TotalUSD)
	}
}

func TestEstimateMixedMeters(t *testing.T) {
	// compute starter + 1 GB bandwidth + 17 min build
	// ≈ $4.90 + $0.01 + $0.06 = $4.97
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 2628000},
		{Kind: store.UsageKindEgressBytes, Tier: "", ResourceKind: store.ResourceKindService, Total: 1073741824},
		{Kind: store.UsageKindBuildSeconds, Tier: "", ResourceKind: store.ResourceKindService, Total: 1020},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-4.97) > 0.05 {
		t.Errorf("mixed meters: want ~$4.97, got %s", est.TotalUSD)
	}
	// All three meters should appear (each ≥ $0.01 when rounded)
	if len(est.Meters) != 3 {
		t.Errorf("want 3 meters, got %d: %+v", len(est.Meters), est.Meters)
	}
}

func TestEstimateTotalIsSumOfAggregatedRows(t *testing.T) {
	// Two rows for the same service/kind/tier should aggregate before pricing.
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 1314000},
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 1314000},
	}
	// 1314000 + 1314000 = 2628000 → same as 1 month of starter
	est := Default.Estimate(rows)
	split := Default.Estimate([]store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 2628000},
	})
	if est.TotalUSD != split.TotalUSD {
		t.Errorf("split vs single: %s != %s (want same after aggregation)", est.TotalUSD, split.TotalUSD)
	}
}

func TestEstimateMixedResourceKinds(t *testing.T) {
	// Service compute + postgres + keyvalue — all at starter-equivalent rates
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 2628000},
		{Kind: store.UsageKindInstanceSeconds, Tier: "basic-256mb", ResourceKind: store.ResourceKindPostgres, Total: 2628000},
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindKeyValue, Total: 2628000},
	}
	// ≈ $4.90 (compute) + $4.90 (postgres) + $7.00 (kv) = $16.80
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-16.80) > 0.10 {
		t.Errorf("mixed resource kinds: want ~$16.80, got %s", est.TotalUSD)
	}
	if len(est.Meters) != 3 {
		t.Fatalf("want 3 meters, got %d", len(est.Meters))
	}
}

func TestFormatUSD(t *testing.T) {
	cases := []struct {
		input float64
		want  string
	}{
		{0, "0.00"},
		{1.005, "1.00"},   // float64: 1.005*100 = 100.4999... rounds down
		{1.004, "1.00"},
		{4.9004878, "4.90"},
		{0.0049, "0.00"},  // sub-cent
		{0.005, "0.01"},   // exactly half a cent rounds up
	}
	for _, c := range cases {
		if got := formatUSD(c.input); got != c.want {
			t.Errorf("formatUSD(%v) = %q, want %q", c.input, got, c.want)
		}
	}
}
