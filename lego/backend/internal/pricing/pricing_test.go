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
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

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
	if FormatUSD(pro) != "17.50" { // Render $25 × 0.70
		t.Errorf("pro = %v, want 17.50", pro)
	}
	if FormatUSD(scale) != "349.30" { // Render $499 × 0.70
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

// TestBillableMeterNamesArePinned pins the exact set of billable meter lookup
// keys the code will demand from Stripe. It guards the CODE side only — it does
// NOT reach the live Stripe catalog (CI has no Stripe account). That gap is the
// whole point of the pin: EnsureContract's resolvePriceIDs requires an active
// Stripe price for EVERY name here, so adding or removing one and shipping
// without reprovisioning breaks EVERY new subscription (checkout dead-ends on
// "active price with lookup key X is missing"), while existing subscriptions —
// which skip resolvePriceIDs — keep working, so it hides until a fresh sign-up.
// That is exactly how disk_gb_hours shipped unprovisioned into live Stripe.
//
// So if this test fails, updating `want` is only half the change: you MUST also
// run scripts/stripe-billing-setup.py against BOTH test and live Stripe (--live)
// so the catalog gains the price before the new code deploys.
func TestBillableMeterNamesArePinned(t *testing.T) {
	want := []string{
		"agent_model_cost_microusd",
		"build_seconds",
		// Custom domains above the plan's included count (w7/m429). Held off
		// every Subscription until BEX_STRIPE_CUSTOM_DOMAIN_METER=1 says the
		// setup script created its Price (billing.HeldMeters), so new
		// subscriptions keep resolving before the live run.
		"custom_domain_hours",
		"disk_gb_hours",
		"egress_gib",
		"instance_seconds.postgres.1c-2g",
		"instance_seconds.postgres.basic-1gb",
		"instance_seconds.postgres.basic-256mb",
		"instance_seconds.postgres.basic-4gb",
		"instance_seconds.postgres.pro-4gb",
		"instance_seconds.service.pro",
		"instance_seconds.service.pro-max",
		"instance_seconds.service.pro-plus",
		"instance_seconds.service.pro-ultra",
		"instance_seconds.service.standard",
		"instance_seconds.service.starter",
		"key_value_cost_microusd", // every Key Value tier, in micro-USD (w5/m266 D5)
		"sandbox_compute_seconds",
		"sandbox_snapshot_gb_hours", // a suspended sandbox's retained image (w22/m313)
		"storage_gb_hours",
	}
	sort.Strings(want)
	got := Default.BillableMeterNames()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("BillableMeterNames = %v, want %v\n"+
			"If you changed this set, you MUST also run scripts/stripe-billing-setup.py "+
			"against test AND live Stripe (--live) or every new subscription will fail.", got, want)
	}
	// The object storage meters (ADR112 D8) are shadow meters: recorded and shown
	// at $0, with no Stripe Price. Naming them here, beside the pin, makes adding
	// one to the billed set a deliberate edit of this test and of the Stripe catalog.
	for _, name := range got {
		if strings.HasPrefix(name, "object_storage") {
			t.Errorf("%q is billable: the object storage meters are shadow meters until the user sets a price (ADR112 D8)", name)
		}
	}
}

// ADR112 D8: the three object storage meters are recorded and shown at $0 and
// change neither the estimate nor the set of billed meters. The Stripe items are
// BillableMeterNames (pinned above), so this proves the invoice cannot move.
func TestObjectStorageShadowMetersAreShownAtZeroAndNeverBilled(t *testing.T) {
	const (
		tenGBFor730Hours = 10 * 730 * 3600
		hundredGiB       = 100 * BytesPerGiB
	)
	compute := store.UsageSummaryRow{
		ServiceID: "srv-1", Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 730 * 3600,
	}
	shadow := []store.UsageSummaryRow{
		{ServiceID: "tea-1", Kind: store.UsageKindObjectStorageGBSeconds, ResourceKind: store.ResourceKindObjectStorage, Total: tenGBFor730Hours},
		{ServiceID: "tea-1", Kind: store.UsageKindObjectStorageIngressBytes, ResourceKind: store.ResourceKindObjectStorage, Total: hundredGiB},
		{ServiceID: "tea-1", Kind: store.UsageKindObjectStorageEgressBytes, ResourceKind: store.ResourceKindObjectStorage, Total: hundredGiB},
	}
	without := Default.Estimate([]store.UsageSummaryRow{compute})
	with := Default.Estimate(append([]store.UsageSummaryRow{compute}, shadow...))
	if with.TotalUSD != without.TotalUSD || fmt.Sprint(with.Meters) != fmt.Sprint(without.Meters) {
		t.Fatalf("recording the object storage meters changed the bill: %+v vs %+v", with, without)
	}
	alone := Default.Estimate(shadow)
	if alone.TotalUSD != "0.00" || len(alone.Meters) != 0 {
		t.Fatalf("object storage alone = %+v, want $0.00 and no billed meter", alone)
	}

	// Each is listed under its resource at a rate of 0, in a unit a reader can check.
	want := map[string]struct{ unit, quantity string }{
		store.UsageKindObjectStorageGBSeconds:    {"GB-mo", "10.00"},
		store.UsageKindObjectStorageIngressBytes: {"GB", "100.00"},
		store.UsageKindObjectStorageEgressBytes:  {"GB", "100.00"},
	}
	lines := map[string]ChargeLine{}
	for _, r := range alone.Resources {
		for _, c := range r.Charges {
			lines[c.Kind] = c
		}
	}
	for kind, w := range want {
		c, ok := lines[kind]
		if !ok || c.RateUSD != "0" || c.CostUSD != "0.00" || c.Unit != w.unit || c.Quantity != w.quantity {
			t.Errorf("%s line = %+v (listed %v), want %s %s at a rate of 0 costing 0.00", kind, c, ok, w.quantity, w.unit)
		}
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

// w22/m313 (ADR104 D8): a workspace whose only sandbox sat suspended for a
// pricing month, holding a 10 GB image, owes storage and no compute: Render's
// $0.08/GB-month at 30% off is $0.056, so $0.56.
func TestEstimateSuspendedSandboxPaysSnapshotStorageOnly(t *testing.T) {
	const tenGBFor730Hours = 10 * 730 * 3600
	est := Default.Estimate([]store.UsageSummaryRow{{
		ServiceID: "sbx-1", Kind: store.UsageKindSandboxSnapshotGBSeconds, ResourceKind: store.ResourceKindSandbox,
		Total: tenGBFor730Hours,
	}})
	if est.TotalUSD != "0.56" || len(est.Meters) != 1 || est.Meters[0].Kind != store.UsageKindSandboxSnapshotGBSeconds {
		t.Fatalf("suspended sandbox estimate = %+v, want one $0.56 storage meter and no compute", est)
	}
	if len(est.Resources) != 1 || len(est.Resources[0].Charges) != 1 {
		t.Fatalf("resources = %+v", est.Resources)
	}
	if c := est.Resources[0].Charges[0]; c.Unit != "GB-mo" || c.Quantity != "10.00" || c.CostUSD != "0.56" {
		t.Errorf("charge = %+v, want 10 GB-mo costing 0.56", c)
	}
	// The same workspace's storage is not charged to anything but a sandbox.
	if other := Default.Estimate([]store.UsageSummaryRow{{
		Kind: store.UsageKindSandboxSnapshotGBSeconds, ResourceKind: store.ResourceKindService, Total: tenGBFor730Hours,
	}}); other.TotalUSD != "0.00" {
		t.Errorf("a non-sandbox resource was priced for sandbox storage: %+v", other)
	}
}

// Memory recorded before the switch to billing on use is shown and not billed
// (w22/m313, w22/m314): beside the weighted compute charge, which already
// includes memory, it must not change the estimate, and its line reads at a
// rate of 0.
func TestMemoryShadowMeterIsShownAndNeverBilled(t *testing.T) {
	compute := store.UsageSummaryRow{
		ServiceID: "sbx-1", Kind: store.UsageKindSandboxComputeSeconds, ResourceKind: store.ResourceKindSandbox, Total: 1211 * 3600,
	}
	memory := store.UsageSummaryRow{
		ServiceID: "sbx-1", Kind: store.UsageKindSandboxMemoryGBSeconds, ResourceKind: store.ResourceKindSandbox, Total: 2 * 3600,
		Unbilled: true, // recorded before the switch (w22/m314)
	}
	without := Default.Estimate([]store.UsageSummaryRow{compute})
	with := Default.Estimate([]store.UsageSummaryRow{compute, memory})
	if with.TotalUSD != without.TotalUSD || len(with.Meters) != len(without.Meters) {
		t.Fatalf("recording memory changed the bill: %+v vs %+v", with, without)
	}
	var line *ChargeLine
	for _, r := range with.Resources {
		for i, c := range r.Charges {
			if c.Kind == store.UsageKindSandboxMemoryGBSeconds {
				line = &r.Charges[i]
			}
		}
	}
	if line == nil || line.RateUSD != "0" || line.CostUSD != "0.00" || line.Unit != "GiB-hr" || line.Quantity != "2.00" {
		t.Errorf("memory line = %+v, want 2 GiB-hr at a rate of 0 costing 0.00", line)
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
		{"postgres basic 256mb", "basic-256mb", store.ResourceKindPostgres, "4.20"},
		{"postgres basic 1gb", "basic-1gb", store.ResourceKindPostgres, "13.30"},
		{"postgres 1c-2g", "1c-2g", store.ResourceKindPostgres, "28.00"},
		{"postgres pro 4gb", "pro-4gb", store.ResourceKindPostgres, "38.50"},
		{"postgres basic 4gb", "basic-4gb", store.ResourceKindPostgres, "52.50"},
		{"key value starter", "starter", store.ResourceKindKeyValue, "7.00"},
		{"key value standard", "standard", store.ResourceKindKeyValue, "14.00"},
		{"key value pro (Render 5g)", "pro", store.ResourceKindKeyValue, "70.00"},
		{"key value 50g", "50g", store.ResourceKindKeyValue, "525.00"},
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
	// $4.20/mo: Render's $6 x 0.70 (w2/m171 t012)
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "basic-256mb", ResourceKind: store.ResourceKindPostgres, Total: 2628000},
	}
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-4.20) > 0.02 {
		t.Errorf("postgres basic-256mb 1 month: want ~$4.20, got %s", est.TotalUSD)
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

// A resource's CostUSD is the sum of its displayed charge lines, to the
// cent: the bill's arithmetic is checkable by eye (ChargeLine), so the
// resource figure must sum the rounded charges, not round the raw float sum.
// (The workspace TotalUSD intentionally still counts sub-cent meters the
// breakdown omits; only the per-resource figure must add up.)
func TestEstimateResourceCostEqualsSumOfCharges(t *testing.T) {
	kinds := []struct {
		kind, tier, rk string
		unit           int64
	}{
		{store.UsageKindInstanceSeconds, "starter", store.ResourceKindService, MonthSeconds},
		{store.UsageKindDiskGBSeconds, "", store.ResourceKindService, MonthSeconds},
		{store.UsageKindInstanceSeconds, "basic-256mb", store.ResourceKindPostgres, MonthSeconds},
		{store.UsageKindStorageGBSeconds, "basic-256mb", store.ResourceKindPostgres, MonthSeconds},
		{store.UsageKindInstanceSeconds, "starter", store.ResourceKindKeyValue, MonthSeconds},
		{store.UsageKindBuildSeconds, "", store.ResourceKindService, 60},
		{store.UsageKindEgressBytes, "", store.ResourceKindService, BytesPerGiB},
	}
	for i := int64(0); i < 60; i++ {
		var rows []store.UsageSummaryRow
		for j, k := range kinds {
			q := (i*37 + int64(j)*101 + k.unit/7) % (k.unit * 3)
			rows = append(rows, store.UsageSummaryRow{
				ServiceID: "s1", ResourceKind: k.rk, Kind: k.kind, Tier: k.tier, Total: q,
			})
		}
		for _, r := range Default.Estimate(rows).Resources {
			var sum int64
			for _, c := range r.Charges {
				sum += monthlyCents(t, c.CostUSD)
			}
			if cost := monthlyCents(t, r.CostUSD); cost != sum {
				t.Errorf("rows %d: resource cost %s != sum of charges (%d cents vs %d): %+v",
					i, r.CostUSD, cost, sum, r.Charges)
			}
		}
	}
}

func TestEstimateMixedResourceKinds(t *testing.T) {
	// Service compute + postgres + keyvalue at their entry paid rates
	rows := []store.UsageSummaryRow{
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindService, Total: 2628000},
		{Kind: store.UsageKindInstanceSeconds, Tier: "basic-256mb", ResourceKind: store.ResourceKindPostgres, Total: 2628000},
		{Kind: store.UsageKindInstanceSeconds, Tier: "starter", ResourceKind: store.ResourceKindKeyValue, Total: 2628000},
	}
	// ≈ $4.90 (compute) + $4.20 (postgres) + $7.00 (kv) = $16.10
	est := Default.Estimate(rows)
	got := parseUSD(t, est.TotalUSD)
	if math.Abs(got-16.10) > 0.10 {
		t.Errorf("mixed resource kinds: want ~$16.10, got %s", est.TotalUSD)
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
		{-0.42, "-0.42"}, // a credit keeps its sign
		{-0.004, "0.00"}, // rounds to zero: never "-0.00"
		{-1.5, "-1.50"},
	}
	for _, c := range cases {
		if got := FormatUSD(c.input); got != c.want {
			t.Errorf("FormatUSD(%v) = %q, want %q", c.input, got, c.want)
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
		store.UsageKindSandboxMemoryGBSeconds,
		store.UsageKindSandboxSnapshotGBSeconds,
		store.UsageKindObjectStorageGBSeconds,
		store.UsageKindObjectStorageIngressBytes,
		store.UsageKindObjectStorageEgressBytes,
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

// A Free Postgres or Key Value includes its storage at $0, as Render's Free
// Postgres does; a paid one keeps the storage rate (w4/427).
func TestFreeDatastoreStorageIsIncluded(t *testing.T) {
	const gbMonth = 730 * 3600
	for _, rk := range []string{store.ResourceKindPostgres, store.ResourceKindKeyValue} {
		free := Default.Estimate([]store.UsageSummaryRow{{Kind: store.UsageKindStorageGBSeconds, ResourceKind: rk, Tier: "free", Total: gbMonth}})
		paid := Default.Estimate([]store.UsageSummaryRow{{Kind: store.UsageKindStorageGBSeconds, ResourceKind: rk, Tier: "basic-256mb", Total: gbMonth}})
		if free.TotalUSD != "0.00" || paid.TotalUSD != "0.21" {
			t.Errorf("%s storage: free=%s paid=%s, want 0.00 and 0.21", rk, free.TotalUSD, paid.TotalUSD)
		}
	}
}

// w5/m266 D5: a Key Value hour is billed as its tier's price in micro-USD on
// the one Key Value meter; exact for every tier, nothing for free.
func TestKeyValueCostMicroUSD(t *testing.T) {
	for _, tc := range []struct{ tier, seconds, want string }{
		// Exact from the rounded pricing.yaml rate literals, as the per-tier
		// Stripe prices were: a 730-hour pro month is $69.999999...
		{"starter", "3600", "9589.041096"},
		{"standard", "3600", "19178.082192"},
		{"pro", "2628000", "69999999.999999"},
		{"50g", "1", "199.771689"},
	} {
		got, ok := Default.KeyValueCostMicroUSD(tc.tier, tc.seconds)
		if !ok || got != tc.want {
			t.Errorf("KeyValueCostMicroUSD(%s, %s) = %q,%v want %q", tc.tier, tc.seconds, got, ok, tc.want)
		}
	}
	for _, tier := range []string{"free", "unknown"} {
		if _, ok := Default.KeyValueCostMicroUSD(tier, "3600"); ok {
			t.Errorf("%s must not bill", tier)
		}
	}
}

// SandboxSessionMicroUSD prices what the meters attributed to a session at
// the rates the bill uses, in micro-USD (w9/m178, w6/m566): one weighted
// vCPU-hour is the sheet's $0.0895, an hour of 8 GiB is $0.1064 and a used
// CPU-hour $0.084; the parts are summed exactly and rounded once, sub-micro-
// dollar usage rounds to 0, and an unpriced sheet reads 0 rather than
// guessing.
func TestSandboxSessionMicroUSD(t *testing.T) {
	for _, tc := range []struct {
		usage store.AgentSessionSandboxUsage
		want  int64
	}{
		{store.AgentSessionSandboxUsage{}, 0},
		{store.AgentSessionSandboxUsage{ComputeMilliSeconds: -5, CPUSeconds: -1, MemoryGBSeconds: -1}, 0},
		{store.AgentSessionSandboxUsage{ComputeMilliSeconds: 1}, 0},                      // $0.0000000249: below a micro-dollar
		{store.AgentSessionSandboxUsage{ComputeMilliSeconds: 40_000}, 994},               // 40 seconds of a 1-vCPU sandbox
		{store.AgentSessionSandboxUsage{ComputeMilliSeconds: 3_600_000}, 89_500},         // one vCPU-hour
		{store.AgentSessionSandboxUsage{ComputeMilliSeconds: 1_000_000_000}, 24_861_111}, // rounded to the nearest micro-dollar
		{store.AgentSessionSandboxUsage{CPUSeconds: 3600}, 84_000},                       // a used CPU-hour
		{store.AgentSessionSandboxUsage{MemoryGBSeconds: 8 * 3600}, 106_400},             // an idle hour of agent-4c-8g
		{store.AgentSessionSandboxUsage{CPUSeconds: 1}, 23},                              // $0.0000233…
		{store.AgentSessionSandboxUsage{MemoryGBSeconds: 1}, 4},                          // $0.0000036944…
		// Summed before rounding: a CPU-second alone is 23 (23.33…) and two
		// GiB-seconds 7 (7.38…), but together 31 (30.72…).
		{store.AgentSessionSandboxUsage{CPUSeconds: 1, MemoryGBSeconds: 2}, 31},
		{store.AgentSessionSandboxUsage{ComputeMilliSeconds: 1_800_000, CPUSeconds: 900, MemoryGBSeconds: 8 * 3600}, 172_150},
	} {
		if got := Default.SandboxSessionMicroUSD(tc.usage); got != tc.want {
			t.Errorf("SandboxSessionMicroUSD(%+v) = %d, want %d", tc.usage, got, tc.want)
		}
	}
	var unpriced *Sheet
	if got := unpriced.SandboxSessionMicroUSD(store.AgentSessionSandboxUsage{ComputeMilliSeconds: 3_600_000, CPUSeconds: 1}); got != 0 {
		t.Errorf("nil sheet = %d, want 0", got)
	}
	if got := (&Sheet{}).SandboxSessionMicroUSD(store.AgentSessionSandboxUsage{ComputeMilliSeconds: 3_600_000, MemoryGBSeconds: 1}); got != 0 {
		t.Errorf("unpriced sheet = %d, want 0", got)
	}
}

// w22/m314 t004 (ADR104 D8): a sandbox that ran for an hour with 2 GiB and
// used 600 CPU-seconds is billed for exactly that: $0.014 of CPU ($0.084 per
// CPU-hour used) and $0.027 of memory ($0.0133 per GiB-hour allocated), and
// nothing on the weighted meter. An idle hour of the same sandbox costs its
// memory only.
func TestEstimateSandboxBilledOnUse(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{ServiceID: "sbx-1", Kind: store.UsageKindSandboxCPUSeconds, Tier: "1c-2g", ResourceKind: store.ResourceKindSandbox, Total: 600},
		{ServiceID: "sbx-1", Kind: store.UsageKindSandboxMemoryGBSeconds, Tier: "1c-2g", ResourceKind: store.ResourceKindSandbox, Total: 2 * 3600},
	}
	est := Default.Estimate(rows)
	if len(est.Resources) != 1 || len(est.Resources[0].Charges) != 2 {
		t.Fatalf("estimate = %+v, want one sandbox with a CPU and a memory line", est)
	}
	cpu, memory := est.Resources[0].Charges[0], est.Resources[0].Charges[1]
	if cpu.Kind != store.UsageKindSandboxCPUSeconds || cpu.Unit != "CPU-hr" || cpu.RateUSD != "0.084" || cpu.Quantity != "0.167" || cpu.Unbilled {
		t.Errorf("CPU line = %+v, want 0.167 CPU-hr at $0.084", cpu)
	}
	if memory.Kind != store.UsageKindSandboxMemoryGBSeconds || memory.Unit != "GiB-hr" || memory.RateUSD != "0.0133" || memory.Quantity != "2.00" || memory.Unbilled {
		t.Errorf("memory line = %+v, want 2 GiB-hr at $0.0133", memory)
	}
	cpuUSD := Default.rateFor(store.UsageKindSandboxCPUSeconds, "1c-2g", store.ResourceKindSandbox) * 600
	memUSD := Default.rateFor(store.UsageKindSandboxMemoryGBSeconds, "1c-2g", store.ResourceKindSandbox) * 2 * 3600
	if math.Abs(cpuUSD-0.014) > 1e-9 || math.Abs(memUSD-0.0266) > 1e-9 {
		t.Errorf("CPU = $%.6f, memory = $%.6f, want $0.014 and $0.0266", cpuUSD, memUSD)
	}
	if est.TotalUSD != "0.04" || est.Resources[0].CostUSD != "0.04" {
		t.Errorf("total = %s / %s, want 0.04 (0.014 + 0.0266)", est.TotalUSD, est.Resources[0].CostUSD)
	}
	if est.SandboxUsageBilledFrom != "2026-11-18T00:00:00Z" || est.AgentSandboxUsageBilledFrom != "2026-11-26T00:00:00Z" {
		t.Errorf("SandboxUsageBilledFrom = %q, AgentSandboxUsageBilledFrom = %q, want the dated switches", est.SandboxUsageBilledFrom, est.AgentSandboxUsageBilledFrom)
	}
	// The same rates never price another resource kind's rows.
	other := Default.Estimate([]store.UsageSummaryRow{
		{ServiceID: "srv-1", Kind: store.UsageKindSandboxCPUSeconds, ResourceKind: store.ResourceKindService, Total: 3600 * 1000},
	})
	if other.TotalUSD != "0.00" || other.SandboxUsageBilledFrom != "" || other.AgentSandboxUsageBilledFrom != "" {
		t.Errorf("a non-sandbox resource was priced for sandbox CPU: %+v", other)
	}
	// An idle running hour: about no CPU, the memory in full.
	idle := Default.Estimate([]store.UsageSummaryRow{
		{ServiceID: "sbx-1", Kind: store.UsageKindSandboxCPUSeconds, ResourceKind: store.ResourceKindSandbox, Total: 16},
		{ServiceID: "sbx-1", Kind: store.UsageKindSandboxMemoryGBSeconds, ResourceKind: store.ResourceKindSandbox, Total: 2 * 3600},
	})
	if idle.TotalUSD != "0.03" {
		t.Errorf("an idle hour costs %s, want 0.03 (memory only)", idle.TotalUSD)
	}
}

// What was recorded before the switch stays a line of its own, at a rate of 0,
// beside the billed line of the same kind; it enters no total.
func TestEstimateKeepsUnbilledSandboxUsageApart(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{ServiceID: "sbx-1", Kind: store.UsageKindSandboxComputeSeconds, ResourceKind: store.ResourceKindSandbox, Total: 1211 * 3600},
		{ServiceID: "sbx-1", Kind: store.UsageKindSandboxCPUSeconds, ResourceKind: store.ResourceKindSandbox, Total: 3600, Unbilled: true},
		{ServiceID: "sbx-1", Kind: store.UsageKindSandboxCPUSeconds, ResourceKind: store.ResourceKindSandbox, Total: 3600},
	}
	est := Default.Estimate(rows)
	if est.TotalUSD != "0.19" { // $0.108 weighted + $0.084 CPU
		t.Fatalf("total = %s, want 0.19: the unbilled CPU-hour must not be charged", est.TotalUSD)
	}
	charges := est.Resources[0].Charges
	if len(charges) != 3 || !charges[1].Unbilled || charges[1].RateUSD != "0" || charges[1].CostUSD != "0.00" || charges[1].Quantity != "1.00" ||
		charges[2].Unbilled || charges[2].CostUSD != "0.08" {
		t.Fatalf("charges = %+v, want the unbilled CPU-hour at 0 and the billed one at $0.08", charges)
	}
	for _, m := range est.Meters {
		if m.Kind == store.UsageKindSandboxCPUSeconds && m.CostUSD != "0.08" {
			t.Errorf("the CPU meter total includes unbilled usage: %+v", m)
		}
	}
}

// Billed CPU and memory are exported on the weighted sandbox meter as the
// number of its units that costs the same, so the Stripe line charges what the
// estimate shows without a second and third catalog item.
func TestSandboxUsageWeightedUnitsCostTheSame(t *testing.T) {
	for _, tc := range []struct {
		kind     string
		quantity int64
	}{{store.UsageKindSandboxCPUSeconds, 600}, {store.UsageKindSandboxMemoryGBSeconds, 7200}, {store.UsageKindSandboxCPUSeconds, 1}} {
		units, ok := Default.SandboxUsageWeightedUnits(tc.kind, strconv.FormatInt(tc.quantity, 10))
		if !ok {
			t.Fatalf("%s: not convertible", tc.kind)
		}
		n, err := strconv.ParseFloat(units, 64)
		if err != nil {
			t.Fatal(err)
		}
		viaStripe := n * Default.rateFor(store.UsageKindSandboxComputeSeconds, "", store.ResourceKindSandbox)
		direct := float64(tc.quantity) * Default.rateFor(tc.kind, "", store.ResourceKindSandbox)
		if math.Abs(viaStripe-direct) > 1e-12 {
			t.Errorf("%s x %d: %s weighted units cost $%.15f, the estimate $%.15f", tc.kind, tc.quantity, units, viaStripe, direct)
		}
		if i := strings.IndexByte(units, '.'); i >= 0 && len(units)-i-1 > 12 {
			t.Errorf("%s: %q has more than the 12 decimals Stripe accepts", tc.kind, units)
		}
	}
	if _, ok := Default.SandboxUsageWeightedUnits(store.UsageKindSandboxComputeSeconds, "1"); ok {
		t.Error("the weighted meter's own kind was converted")
	}
	if _, ok := Default.SandboxUsageWeightedUnits(store.UsageKindSandboxCPUSeconds, "x"); ok {
		t.Error("a non-numeric quantity was converted")
	}
}

// The switch is one instant on an hour boundary, and the sheet refuses a
// section that could bill an hour on two models or has nothing to export on.
func TestSandboxUsageSwitchInstant(t *testing.T) {
	from := Default.SandboxUsageBilledFrom()
	if want := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC); !from.Equal(want) {
		t.Fatalf("SandboxUsageBilledFrom = %s, want %s", from, want)
	}
	if Default.SandboxUsageBilledAt(from.Add(-time.Hour)) || !Default.SandboxUsageBilledAt(from) || !Default.SandboxUsageBilledAt(from.Add(time.Hour)) {
		t.Error("an hour is billed on use exactly when it starts at or after the switch")
	}
	// w6/m566: an agent session's sandbox switches at its own instant, later.
	if agent, want := Default.AgentSandboxUsageBilledFrom(), time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC); !agent.Equal(want) {
		t.Fatalf("AgentSandboxUsageBilledFrom = %s, want %s", agent, want)
	}
	sheet := string(sheetYAML)
	edit := func(old, new string) string {
		t.Helper()
		if strings.Count(sheet, old) != 1 {
			t.Fatalf("pricing.yaml no longer has exactly one %q", old)
		}
		return strings.Replace(sheet, old, new, 1)
	}
	const instant = `billedFrom: "2026-11-18T00:00:00Z"`
	const agentInstant = `agentBilledFrom: "2026-11-26T00:00:00Z"`
	for name, raw := range map[string]string{
		"not on the hour":               edit(instant, `billedFrom: "2026-11-18T00:30:00Z"`),
		"no instant":                    edit(instant, ""),
		"agent switch not on the hour":  edit(agentInstant, `agentBilledFrom: "2026-11-26T00:00:01Z"`),
		"agent switch before the first": edit(agentInstant, `agentBilledFrom: "2026-11-17T23:00:00Z"`),
		"agent switch not an instant":   edit(agentInstant, `agentBilledFrom: "2026-11-26"`),
		"a free kind":                   edit("usdPerCPUSecond: 0.000023333333333333333333333333", "usdPerCPUSecond: 0"),
		"no weighted":                   edit("usdPerWeightedSecond: 0.000000024861111111111111111111", "usdPerWeightedSecond: 0"),
	} {
		if _, err := parseSheet([]byte(raw)); err == nil || !strings.Contains(err.Error(), "sandboxUsage") {
			t.Errorf("%s: err = %v, want the sandboxUsage section refused", name, err)
		}
	}
	// A sheet without the section never switches and prices neither kind.
	start := strings.Index(sheet, "\nsandboxUsage:\n")
	end := strings.Index(sheet, "usdPerMemoryGBSecond:")
	if start < 0 || end < start {
		t.Fatal("pricing.yaml has no sandboxUsage section")
	}
	end += strings.IndexByte(sheet[end:], '\n')
	plain, err := parseSheet([]byte(sheet[:start] + sheet[end:]))
	if err != nil {
		t.Fatal(err)
	}
	if !plain.SandboxUsageBilledFrom().IsZero() || plain.SandboxUsageBilledAt(time.Now()) || !plain.AgentSandboxUsageBilledFrom().IsZero() ||
		plain.rateFor(store.UsageKindSandboxCPUSeconds, "", store.ResourceKindSandbox) != 0 {
		t.Error("a sheet without sandboxUsage switched or priced sandbox CPU")
	}
	// Without agentBilledFrom an agent session's sandbox never switches.
	noAgent, err := parseSheet([]byte(edit(agentInstant, "")))
	if err != nil {
		t.Fatal(err)
	}
	if !noAgent.AgentSandboxUsageBilledFrom().IsZero() || !noAgent.SandboxUsageBilledFrom().Equal(from) {
		t.Error("a sheet without agentBilledFrom switched an agent session's sandbox")
	}
	if est := noAgent.Estimate([]store.UsageSummaryRow{{ServiceID: "sbx-1", Kind: store.UsageKindSandboxCPUSeconds, ResourceKind: store.ResourceKindSandbox, Total: 1}}); est.AgentSandboxUsageBilledFrom != "" {
		t.Errorf("estimate names an agent switch the sheet does not have: %q", est.AgentSandboxUsageBilledFrom)
	}
}
