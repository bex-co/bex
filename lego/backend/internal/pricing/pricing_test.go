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

func TestWorkspacePlanFeesAreThirtyPercentOffRender(t *testing.T) {
	hobby, hobbyOK := Default.WorkspaceUSDPerMonth(store.PlanHobby)
	pro, proOK := Default.WorkspaceUSDPerMonth(store.PlanPro)
	scale, scaleOK := Default.WorkspaceUSDPerMonth(store.PlanScale)
	_, enterpriseOK := Default.WorkspaceUSDPerMonth(store.PlanEnterprise)
	if !hobbyOK || !proOK || !scaleOK {
		t.Fatalf("listed = hobby %v pro %v scale %v, want all true", hobbyOK, proOK, scaleOK)
	}
	if enterpriseOK {
		t.Fatal("enterprise must not have a catalog monthly SKU")
	}
	if hobby != 0 {
		t.Errorf("hobby = %v, want 0", hobby)
	}
	if formatUSD(pro) != "17.50" { // Render $25 × 0.70
		t.Errorf("pro = %v, want 17.50", pro)
	}
	if formatUSD(scale) != "349.30" { // Render $499 × 0.70
		t.Errorf("scale = %v, want 349.30", scale)
	}
}

func TestParseSheetRejectsEnterpriseWorkspaceFee(t *testing.T) {
	yml := []byte(`
version: "4"
workspace:
  - plan: hobby
    usdPerMonth: 0
  - plan: pro
    usdPerMonth: 17.50
  - plan: scale
    usdPerMonth: 349.30
  - plan: enterprise
    usdPerMonth: 1
`)
	if _, err := parseSheet(yml); err == nil {
		t.Fatal("expected error for enterprise workspace SKU")
	}
}

func TestBillableMeterNamesMatchesStripeCatalog(t *testing.T) {
	want := []string{
		"build_seconds",
		"disk_gb_hours",
		"egress_gib",
		"instance_seconds.key_value.standard",
		"instance_seconds.key_value.starter",
		"instance_seconds.postgres.basic-1gb",
		"instance_seconds.postgres.basic-256mb",
		"instance_seconds.service.pro",
		"instance_seconds.service.pro-max",
		"instance_seconds.service.pro-plus",
		"instance_seconds.service.pro-ultra",
		"instance_seconds.service.standard",
		"instance_seconds.service.starter",
		"sandbox_compute_seconds",
		"storage_gb_hours",
	}
	got := Default.BillableMeterNames()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("BillableMeterNames = %v, want %v", got, want)
	}
}

func TestEstimateSandboxComputeUsesWeightedSecondRate(t *testing.T) {
	// 553 units/second is the default 500m/512Mi sandbox shape. One hour
	// should rate at the AgentCore reference cost for 0.5 vCPU + 0.5 GiB.
	est := Default.Estimate([]store.UsageSummaryRow{{
		Kind: store.UsageKindSandboxComputeSeconds, ResourceKind: store.ResourceKindSandbox,
		Total: 553 * 3600,
	}})
	if est.TotalUSD != "0.05" || len(est.Meters) != 1 || est.Meters[0].Kind != store.UsageKindSandboxComputeSeconds {
		t.Fatalf("sandbox estimate = %+v, want one $0.05 meter", est)
	}
}

// A 10 GB disk for one 730-hour pricing month is Render's $2.50 at 30% off.
// This is the one SKU whose estimate equals its invoice, since both sides bill
// provisioned capacity (ADR082 D8) — so the arithmetic is worth pinning.
func TestEstimateDiskUsesProvisionedGBMonthRate(t *testing.T) {
	const gbSecondsIn730Hours = 10 * 730 * 3600 // 10 GB held for a pricing month
	est := Default.Estimate([]store.UsageSummaryRow{{
		Kind: store.UsageKindDiskGBSeconds, ResourceKind: store.ResourceKindService,
		Total: gbSecondsIn730Hours,
	}})
	if est.TotalUSD != "1.75" || len(est.Meters) != 1 || est.Meters[0].Kind != store.UsageKindDiskGBSeconds {
		t.Fatalf("disk estimate = %+v, want one $1.75 meter (10GB × $0.175/GB-month)", est)
	}
}

// The datastore storage meter and the disk meter must not collapse into one
// rate: they bill different things (used bytes vs provisioned capacity) at
// different prices, and folding them would make one of the two invoice lines
// impossible to re-derive.
func TestDiskAndStorageRatesStayDistinct(t *testing.T) {
	const gbMonth = 730 * 3600
	disk := Default.Estimate([]store.UsageSummaryRow{{
		Kind: store.UsageKindDiskGBSeconds, ResourceKind: store.ResourceKindService, Total: gbMonth,
	}})
	storage := Default.Estimate([]store.UsageSummaryRow{{
		Kind: store.UsageKindStorageGBSeconds, ResourceKind: store.ResourceKindPostgres, Total: gbMonth,
	}})
	if disk.TotalUSD == storage.TotalUSD {
		t.Fatalf("disk and storage both priced %s; they must stay separate rates", disk.TotalUSD)
	}
	if disk.TotalUSD != "0.18" || storage.TotalUSD != "0.21" {
		t.Fatalf("disk=%s storage=%s, want 0.18 ($0.175 rounded to the cent) and 0.21", disk.TotalUSD, storage.TotalUSD)
	}
}

// A disk meter attributed to anything but a service prices at zero rather than
// silently charging the service rate for an unknown resource kind.
func TestDiskRateAppliesOnlyToServices(t *testing.T) {
	est := Default.Estimate([]store.UsageSummaryRow{{
		Kind: store.UsageKindDiskGBSeconds, ResourceKind: store.ResourceKindPostgres, Total: 10 * 730 * 3600,
	}})
	if est.TotalUSD != "0.00" {
		t.Fatalf("disk on a postgres resource priced %s, want 0.00", est.TotalUSD)
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
	// 1 byte of egress at $0.015/GiB rounds to $0.00.
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

func TestEstimatePaidTierMonthlyPricesMatchCatalog(t *testing.T) {
	tests := []struct {
		name         string
		tier         string
		resourceKind string
		want         string
	}{
		{"service starter", "starter", store.ResourceKindService, "4.90"},
		{"service standard", "standard", store.ResourceKindService, "17.50"},
		{"service pro", "pro", store.ResourceKindService, "59.50"},
		{"service pro plus", "pro-plus", store.ResourceKindService, "122.50"},
		{"service pro max", "pro-max", store.ResourceKindService, "157.50"},
		{"service pro ultra", "pro-ultra", store.ResourceKindService, "315.00"},
		{"postgres basic 256mb", "basic-256mb", store.ResourceKindPostgres, "4.90"},
		{"postgres basic 1gb", "basic-1gb", store.ResourceKindPostgres, "14.00"},
		{"key value starter", "starter", store.ResourceKindKeyValue, "7.00"},
		{"key value standard", "standard", store.ResourceKindKeyValue, "21.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			est := Default.Estimate([]store.UsageSummaryRow{{
				Kind:         store.UsageKindInstanceSeconds,
				Tier:         tt.tier,
				ResourceKind: tt.resourceKind,
				Total:        2628000,
			}})
			if est.TotalUSD != tt.want {
				t.Fatalf("730-hour monthly price = %s, want %s", est.TotalUSD, tt.want)
			}
		})
	}
}

func TestEstimateOneBandwidthGigabyte(t *testing.T) {
	// 1 GiB at $0.015/GiB rounds to $0.02.
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindEgressBytes, Tier: "", ResourceKind: store.ResourceKindService, Total: 1073741824},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-0.02) > 0.001 {
		t.Errorf("1 GiB bandwidth: want rounded $0.02, got %s", est.TotalUSD)
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

func TestEstimateOneGBMonthStorage(t *testing.T) {
	// 1 GB used continuously for a 730-hour pricing month is 2,628,000
	// GB-seconds. bex applies 30% off Render Postgres's $0.30/GB-month.
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindStorageGBSeconds, ResourceKind: store.ResourceKindPostgres, Total: 2628000},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-0.21) > 0.01 {
		t.Errorf("1 GB-month storage: want ~$0.21, got %s", est.TotalUSD)
	}
	if len(est.Meters) != 1 || est.Meters[0].Kind != store.UsageKindStorageGBSeconds {
		t.Fatalf("storage meter missing from estimate: %+v", est.Meters)
	}
}

func TestEstimateStorageOnlyForDatastores(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindStorageGBSeconds, ResourceKind: store.ResourceKindService, Total: 2628000},
	}
	if got := Default.Estimate(rows).TotalUSD; got != "0.00" {
		t.Errorf("service storage is not a managed-datastore meter: got %s", got)
	}
}

func TestEstimateMixedMeters(t *testing.T) {
	// compute starter + 1 GiB bandwidth + 17 min build
	// ≈ $4.90 + $0.02 + $0.06 = $4.98
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 2628000},
		{Kind: store.UsageKindEgressBytes, Tier: "", ResourceKind: store.ResourceKindService, Total: 1073741824},
		{Kind: store.UsageKindBuildSeconds, Tier: "", ResourceKind: store.ResourceKindService, Total: 1020},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-4.98) > 0.05 {
		t.Errorf("mixed meters: want ~$4.98, got %s", est.TotalUSD)
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
		{1.005, "1.00"}, // float64: 1.005*100 = 100.4999... rounds down
		{1.004, "1.00"},
		{4.9004878, "4.90"},
		{0.0049, "0.00"}, // sub-cent
		{0.005, "0.01"},  // exactly half a cent rounds up
	}
	for _, c := range cases {
		if got := formatUSD(c.input); got != c.want {
			t.Errorf("formatUSD(%v) = %q, want %q", c.input, got, c.want)
		}
	}
}

// --- per-resource breakdown (Resources) ---

// findResource returns the estimate for one service id, failing the test when
// the breakdown does not carry it.
func findResource(t *testing.T, est EstimatedCost, serviceID string) ResourceEstimate {
	t.Helper()
	for _, r := range est.Resources {
		if r.ServiceID == serviceID {
			return r
		}
	}
	t.Fatalf("no resource estimate for %q in %+v", serviceID, est.Resources)
	return ResourceEstimate{}
}

func TestEstimateResourcesSplitCostPerService(t *testing.T) {
	// Two services on the same tier: Meters aggregates them into one entry,
	// Resources must keep them apart — that separation is the whole point.
	rows := []store.UsageSummaryRow{
		{ServiceID: "srv-a", Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 2628000},
		{ServiceID: "srv-b", Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 1314000},
	}
	est := Default.Estimate(rows)
	if len(est.Meters) != 1 {
		t.Fatalf("want 1 aggregated meter, got %d", len(est.Meters))
	}
	if len(est.Resources) != 2 {
		t.Fatalf("want 2 resources, got %d: %+v", len(est.Resources), est.Resources)
	}
	if got := parseUSD(t, findResource(t, est, "srv-a").CostUSD); math.Abs(got-4.90) > 0.02 {
		t.Errorf("srv-a: want ~$4.90, got %s", findResource(t, est, "srv-a").CostUSD)
	}
	if got := parseUSD(t, findResource(t, est, "srv-b").CostUSD); math.Abs(got-2.45) > 0.02 {
		t.Errorf("srv-b: want ~$2.45, got %s", findResource(t, est, "srv-b").CostUSD)
	}
}

func TestEstimateResourceChargeArithmeticIsCheckable(t *testing.T) {
	// The page shows "rate/unit × quantity = cost"; the three must agree, or
	// the bill invites a support ticket.
	rows := []store.UsageSummaryRow{
		{ServiceID: "srv-a", Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 2628000},
	}
	est := Default.Estimate(rows)
	r := findResource(t, est, "srv-a")
	if len(r.Charges) != 1 {
		t.Fatalf("want 1 charge line, got %d", len(r.Charges))
	}
	c := r.Charges[0]
	if c.Unit != "hr" {
		t.Errorf("unit = %q, want hr", c.Unit)
	}
	if got := parseUSD(t, c.Quantity); math.Abs(got-730) > 0.01 {
		t.Errorf("quantity = %s, want 730 hr", c.Quantity)
	}
	product := parseUSD(t, c.RateUSD) * parseUSD(t, c.Quantity)
	if math.Abs(product-parseUSD(t, c.CostUSD)) > 0.01 {
		t.Errorf("rate %s × qty %s = %.4f, but cost = %s", c.RateUSD, c.Quantity, product, c.CostUSD)
	}
}

func TestEstimateResourcesKeepFreeAndSubCentLines(t *testing.T) {
	// Resources doubles as the only per-service *usage* view, so unlike
	// Meters it must not drop a line just because it rounds to zero dollars.
	rows := []store.UsageSummaryRow{
		{ServiceID: "srv-free", Kind: store.UsageKindInstanceSeconds, Tier: "free", ResourceKind: store.ResourceKindService, Total: 2628000},
		{ServiceID: "srv-free", Kind: store.UsageKindEgressBytes, ResourceKind: store.ResourceKindService, Total: 1048576},
	}
	est := Default.Estimate(rows)
	if len(est.Meters) != 0 {
		t.Errorf("want no meters (all sub-cent/free), got %+v", est.Meters)
	}
	r := findResource(t, est, "srv-free")
	if len(r.Charges) != 2 {
		t.Fatalf("want both charge lines retained, got %+v", r.Charges)
	}
	if r.CostUSD != "0.00" {
		t.Errorf("cost = %s, want 0.00", r.CostUSD)
	}
	// The rate must still be legible even where the cost rounds away.
	for _, c := range r.Charges {
		if c.Kind == store.UsageKindEgressBytes && c.RateUSD == "0.00" {
			t.Errorf("bandwidth rate collapsed to 0.00; want a visible rate")
		}
	}
}

func TestEstimateResourcesSeparateKindsSharingAnID(t *testing.T) {
	// A Database and an App may legally share a service_id; they are
	// different resources and must not merge.
	rows := []store.UsageSummaryRow{
		{ServiceID: "dup", Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 2628000},
		{ServiceID: "dup", Kind: store.UsageKindInstanceSeconds, Tier: "basic-256mb", ResourceKind: store.ResourceKindPostgres, Total: 2628000},
	}
	est := Default.Estimate(rows)
	if len(est.Resources) != 2 {
		t.Fatalf("want 2 resources for one shared id, got %+v", est.Resources)
	}
}

func TestEstimateResourcesIsNonNilAndSkipsZeroRows(t *testing.T) {
	if est := Default.Estimate(nil); est.Resources == nil {
		t.Error("Resources must be non-nil for nil rows")
	}
	rows := []store.UsageSummaryRow{
		{ServiceID: "srv-a", Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 0},
	}
	if est := Default.Estimate(rows); len(est.Resources) != 0 {
		t.Errorf("zero-quantity row produced a resource: %+v", est.Resources)
	}
}

func TestUnitLabelsCoverEveryMeterKind(t *testing.T) {
	// A kind with no unit would render "436.26 " with a bare rate — the guard
	// is here so a new meter kind cannot ship without a display unit.
	for _, kind := range []string{
		store.UsageKindInstanceSeconds,
		store.UsageKindEgressBytes,
		store.UsageKindBuildSeconds,
		store.UsageKindStorageGBSeconds,
		store.UsageKindSandboxComputeSeconds,
	} {
		if u := unitFor(kind); u.label == "" || u.per <= 0 {
			t.Errorf("unitFor(%q) = %+v, want a label and positive divisor", kind, u)
		}
	}
}

func TestFormatRateWidensUntilVisible(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{0.21, "0.21"},
		{0.015, "0.015"},
		{0.0035, "0.0035"},
		{0.0067126, "0.006713"}, // starter compute, per hour
		{0.000000123, "0.000000123"},
	}
	for _, tc := range tests {
		if got := formatRate(tc.in); got != tc.want {
			t.Errorf("formatRate(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Render's Blueprint panel shows a Disks group; bex's absence of one was a
// recorded divergence only while disks were a non-goal (ADR018 → ADR082).
func TestMonthlyEstimateIncludesAServicesDisk(t *testing.T) {
	est := Default.MonthlyEstimate([]MonthlyResource{{
		Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter", StorageGB: 10,
	}})
	if len(est.Lines) != 1 {
		t.Fatalf("lines = %+v, want one", est.Lines)
	}
	line := est.Lines[0]
	// $4.90 starter + 10GB x $0.175 = $6.65
	if line.MonthlyUSD != "6.65" || line.StorageUSD != "1.75" || line.StorageGB != 10 {
		t.Fatalf("line = %+v, want $6.65 total with a $1.75 disk line", line)
	}
}

// A disk on a free service contributes nothing, because the whole free line is
// filtered out — and disks are refused on the free tier anyway.
func TestMonthlyEstimateSkipsAFreeServicesDisk(t *testing.T) {
	est := Default.MonthlyEstimate([]MonthlyResource{{
		Name: "web", ResourceKind: store.ResourceKindService, Tier: "free", StorageGB: 10,
	}})
	if est.TotalUSD != "0.00" || len(est.Lines) != 0 {
		t.Fatalf("estimate = %+v, want nothing priced", est)
	}
}

// The two storage rates must not be confused: a service's disk prices at
// $0.175/GB-mo while a datastore's volume prices at $0.21.
func TestMonthlyEstimateUsesTheDiskRateForServicesAndStorageForDatastores(t *testing.T) {
	service := Default.MonthlyEstimate([]MonthlyResource{{
		Name: "web", ResourceKind: store.ResourceKindService, Tier: "starter", StorageGB: 100,
	}})
	datastore := Default.MonthlyEstimate([]MonthlyResource{{
		Name: "db", ResourceKind: store.ResourceKindPostgres, Tier: "basic-256mb", StorageGB: 100,
	}})
	if service.Lines[0].StorageUSD == datastore.Lines[0].StorageUSD {
		t.Fatalf("disk and datastore storage both priced %s; the rates differ", service.Lines[0].StorageUSD)
	}
	if service.Lines[0].StorageUSD != "17.50" || datastore.Lines[0].StorageUSD != "21.00" {
		t.Fatalf("disk=%s datastore=%s, want 17.50 and 21.00",
			service.Lines[0].StorageUSD, datastore.Lines[0].StorageUSD)
	}
}
