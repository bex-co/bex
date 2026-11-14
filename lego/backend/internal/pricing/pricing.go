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

// Package pricing is bex's price sheet: per-unit USD rates derived from
// Render's captured public pricing (docs/render-artifacts/pricing.md) at a
// fixed discount — 30% off workspace-plan fees, compute/Postgres/KeyValue/
// build-minute/Postgres storage lines, 90% off bandwidth. It is backend-only
// (the operator never imports it) and produces the advisory estimate shown
// beside Stripe's real invoice data. The same sheet also names the Stripe
// catalog dimensions; Stripe owns authoritative rating, invoicing, and
// collection (ADR040). Workspace plan fees are licensed monthly SKUs (not
// usage meters) and are therefore not in BillableMeterNames.
//
// Usage: call Default.Estimate(rows) with the UsageSummaryRows from the store
// to get an EstimatedCost. An unknown tier contributes $0 (not an error).
package pricing

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"math/big"
	"slices"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

//go:embed pricing.yaml
var sheetYAML []byte

// Default is the production price sheet, loaded once at package init from the
// embedded YAML. A malformed sheet panics at init — it is checked-in data
// with its own test (pricing_test.go); it must never fail silently at
// request time.
var Default = mustLoad(sheetYAML)

// Sheet maps metering dimensions to per-unit USD rates. It is immutable after
// construction.
type Sheet struct {
	compute  map[string]float64 // tier ID → $/second for instance_seconds (service)
	postgres map[string]float64 // tier ID → $/second for instance_seconds (postgres)
	keyvalue map[string]float64 // tier ID → $/second for instance_seconds (key_value)
	// keyvalueMicroUSD is each paid Key Value tier's rate in micro-USD per
	// second, exact, for the KeyValueCostMeter events.
	keyvalueMicroUSD map[string]*big.Rat
	bandwidth        float64 // $/byte for egress_bytes
	build            float64 // $/second for build_seconds
	// buildIncluded is plan → build_seconds a month the plan includes before
	// build_seconds is charged (docs/ADR030 §13).
	buildIncluded map[string]int64
	// egressIncluded is plan → egress_bytes a month the plan includes before
	// bandwidth is charged (docs/ADR030 §14).
	egressIncluded map[string]int64
	storage        float64 // $/GB-second of USED datastore volume for storage_gb_seconds
	disk           float64 // $/GB-second of PROVISIONED capacity for disk_gb_seconds
	// sandboxSnapshot is $/GB-second of a suspended sandbox's retained image for
	// sandbox_snapshot_gb_seconds.
	sandboxSnapshot float64
	sandbox         float64 // $/milli-vCPU-equivalent-second for sandbox_compute_seconds
	// sandboxUSD is the same rate, exact, for pricing one session's compute
	// (SandboxComputeMicroUSD).
	sandboxUSD *big.Rat
	workspace  map[string]float64 // plan ID → $/month licensed fee (hobby/pro/scale)
	// agentModelMarkup is the platform model route's price multiplier on
	// upstream cost (ADR091 D1): every request is priced at its provider cost
	// × markup and metered as agent_model_cost_microusd. Token meters are
	// display only.
	agentModelMarkup float64
	// agentModelSpendCap is plan → default monthly agent-model spend cap in
	// micro-USD (ADR091 D5); enterprise and unknown plans take the highest one.
	agentModelSpendCap map[string]int64
	// agentModelMaxCapMultiplier bounds an admin cap override to this × the
	// plan default.
	agentModelMaxCapMultiplier float64
	// trustTiers is the D10.1 ladder, T0 first.
	trustTiers []Tier
	// agentModelUpstream is model → list price for models bex calls
	// directly, whose provider reports no cost (agentModel.upstreamPrices).
	agentModelUpstream map[string]upstreamPrice
}

// upstreamPrice is one directly called model's list price, USD per million
// tokens; cached input is the prompt-cache read rate.
type upstreamPrice struct {
	Model       string  `json:"model"`
	Input       float64 `json:"inputUsdPerMTok"`
	CachedInput float64 `json:"cachedInputUsdPerMTok"`
	Output      float64 `json:"outputUsdPerMTok"`
}

// AgentModelUpstreamCost is the upstream USD cost of one exchange with model
// from its pricing.yaml list price, and false when the sheet lists no price
// for it. cacheWrite is not billed separately by the providers listed.
func (s *Sheet) AgentModelUpstreamCost(model string, in, out, cacheRead, cacheWrite int64) (float64, bool) {
	if s == nil {
		return 0, false
	}
	p, ok := s.agentModelUpstream[model]
	if !ok {
		return 0, false
	}
	return (float64(in+cacheWrite)*p.Input + float64(cacheRead)*p.CachedInput + float64(out)*p.Output) / tokensPerMillion, true
}

// tokensPerMillion is the display basis of the platform-route token meters:
// quantities read, like OpenRouter's list prices, per 1M tokens.
const tokensPerMillion = 1_000_000

// microUSDPerUSD is agent_model_cost_microusd's unit basis: one unit is one
// micro-USD, so the meter rates at $0.000001 per unit (ADR091 D3).
const microUSDPerUSD = 1_000_000

// Agent model markup bounds checked at load: below 1 bex would sell under
// cost; above 10 is a typo, not a price.
const (
	minAgentModelMarkup = 1
	maxAgentModelMarkup = 10
)

// AgentModelMarkup is the multiplier the platform model route applies to each
// request's upstream cost to price it (ADR091 D1, pricing.yaml
// agentModel.markup).
func (s *Sheet) AgentModelMarkup() float64 {
	if s == nil {
		return 0
	}
	return s.agentModelMarkup
}

// SandboxSnapshotMeter is the Stripe dimension a suspended sandbox's retained
// image bills on: sandbox_snapshot_gb_seconds re-based to GB-hours, like
// storage_gb_hours, at the sandboxSnapshot rate (w22/m313, ADR023).
const SandboxSnapshotMeter = "sandbox_snapshot_gb_hours"

// KeyValueCostMeter is the single Stripe dimension every paid Key Value tier
// bills on: the tier's instance seconds rated at its pricing.yaml rate, in
// micro-USD (one unit is $0.000001), like agent_model_cost_microusd (ADR091).
// One dimension instead of one per tier keeps the catalog under Stripe's 20
// items per Subscription (w5/m266 D5, ADR040).
const KeyValueCostMeter = "key_value_cost_microusd"

// KeyValueCostMicroUSD rates seconds (an integer string) of Key Value tier as
// micro-USD, rounded to six decimals; ok is false for a free or unpriced tier.
// The arithmetic is exact rational on the rate's shortest decimal spelling (its
// pricing.yaml literal), never a float product.
func (s *Sheet) KeyValueCostMicroUSD(tier, seconds string) (string, bool) {
	rate := s.keyvalueMicroUSD[tier]
	if rate == nil {
		return "", false
	}
	n, ok := new(big.Rat).SetString(seconds)
	if !ok {
		return "", false
	}
	return TrimDecimal(n.Mul(n, rate), 6), true
}

// SandboxComputeMicroUSD prices weighted milli-vCPU seconds of sandbox compute
// at the sandbox_compute_seconds rate the bill uses, in micro-USD rounded to
// the nearest unit, the unit an agent session's modelUSD is kept in (w9/m178).
// The arithmetic is exact rational on the rate float's shortest decimal; an
// unpriced sheet reads 0.
func (s *Sheet) SandboxComputeMicroUSD(milliSeconds int64) int64 {
	if s == nil || s.sandboxUSD == nil || milliSeconds <= 0 {
		return 0
	}
	micro := new(big.Rat).Mul(big.NewRat(milliSeconds, 1), s.sandboxUSD)
	micro.Mul(micro, big.NewRat(1_000_000, 1))
	n, _ := new(big.Int).SetString(micro.FloatString(0), 10)
	return n.Int64()
}

// TrimDecimal renders r rounded to places decimals without trailing zeros.
func TrimDecimal(r *big.Rat, places int) string {
	out := strings.TrimRight(strings.TrimRight(r.FloatString(places), "0"), ".")
	if out == "" || out == "-0" {
		return "0"
	}
	return out
}

// BillableMeterNames returns the Stripe lookup/event names represented by this
// price sheet. Free/zero-rate instance tiers are omitted because no charge is
// due and therefore no Stripe meter or subscription item exists. The stable
// lexical order makes catalog validation and tests deterministic.
func (s *Sheet) BillableMeterNames() []string {
	names := make([]string, 0, len(s.compute)+len(s.postgres)+len(s.keyvalue)+4)
	appendTiers := func(resourceKind string, tiers map[string]float64) {
		for tier, rate := range tiers {
			if rate > 0 {
				names = append(names, "instance_seconds."+resourceKind+"."+tier)
			}
		}
	}
	appendTiers(store.ResourceKindService, s.compute)
	appendTiers(store.ResourceKindPostgres, s.postgres)
	// Key Value is one cost-denominated dimension, not one per tier: Stripe
	// caps a Subscription at 20 items (w5/m266 D5).
	for _, rate := range s.keyvalue {
		if rate > 0 {
			names = append(names, KeyValueCostMeter)
			break
		}
	}
	if s.bandwidth > 0 {
		names = append(names, "egress_gib")
	}
	if s.build > 0 {
		names = append(names, "build_seconds")
	}
	if s.storage > 0 {
		names = append(names, "storage_gb_hours")
	}
	if s.sandbox > 0 {
		names = append(names, store.UsageKindSandboxComputeSeconds)
	}
	if s.disk > 0 {
		names = append(names, "disk_gb_hours")
	}
	if s.sandboxSnapshot > 0 {
		names = append(names, SandboxSnapshotMeter)
	}
	// Always billable: agentModel.markup is validated to [1, 10] at load.
	names = append(names, store.UsageKindAgentModelCostMicroUSD)
	sort.Strings(names)
	return names
}

// WorkspaceUSDPerMonth returns the catalog monthly fee for a workspace plan.
// listed is false for Enterprise and unknown plans (custom / no SKU). Hobby
// is listed at $0.
func (s *Sheet) WorkspaceUSDPerMonth(plan string) (usd float64, listed bool) {
	if s == nil {
		return 0, false
	}
	v, ok := s.workspace[plan]
	return v, ok
}

// EstimatedCost is the workspace-level cost estimate for a period.
type EstimatedCost struct {
	// TotalUSD is the sum of all meter costs, formatted to the nearest cent
	// (e.g. "1.23"). Always present; "0.00" for empty or all-free usage.
	TotalUSD string `json:"totalUsd"`
	// Meters breaks the total down by (kind × tier × resourceKind). Entries
	// whose rounded cost is less than $0.01 are omitted. Always a non-nil
	// slice (empty when there is no billable usage).
	Meters []MeterEstimate `json:"meters"`
	// Resources breaks the same total down by the resource that incurred it,
	// so a bill can answer "which service costs me money" — Meters aggregates
	// every service together and structurally cannot. Unlike Meters this keeps
	// every metered line, including free tiers and sub-cent costs, because it
	// is also the only place a reader sees *how much* was consumed. Always a
	// non-nil slice.
	Resources []ResourceEstimate `json:"resources"`
}

// ResourceEstimate is one resource's estimated cost for the period, with the
// per-meter charge lines behind it.
type ResourceEstimate struct {
	// ServiceID is the metered resource's id (App, Database, KeyValue, sandbox).
	ServiceID string `json:"serviceId"`
	// ServiceName is the user-facing display name. The price sheet cannot
	// resolve names — callers that have them (usage.Service) stamp it in;
	// otherwise it is empty and presenters fall back to ServiceID.
	ServiceName string `json:"serviceName,omitempty"`
	// ResourceKind is "service", "postgres", "key_value", or "sandbox".
	ResourceKind string `json:"resourceKind,omitempty"`
	// Deleted reports confirmed absence independently of name retention.
	// False means live or unknown; the price sheet does not resolve lifecycle.
	Deleted bool `json:"deleted,omitempty"`
	// CostUSD is this resource's estimated dollars, formatted to cents.
	CostUSD string `json:"costUsd"`
	// Charges are the meter lines behind CostUSD, in first-seen order.
	Charges []ChargeLine `json:"charges"`
}

// ChargeLine is one (kind × tier) meter's contribution to a resource's cost.
// Rate and Quantity are both expressed in Unit so that the arithmetic on the
// page is checkable by eye ("$0.0067/hr × 436.26 hr = $2.93"); the raw meter
// units (seconds, bytes, GB-seconds) are unreadable at human scale.
type ChargeLine struct {
	// Kind is the meter kind — "instance_seconds", "egress_bytes", etc.
	Kind string `json:"kind"`
	// Tier is the plan/tier id for instance_seconds meters; empty otherwise.
	Tier string `json:"tier,omitempty"`
	// Unit is the display unit both RateUSD and Quantity are quoted in —
	// "hr", "GB", "min", "GB-mo", or "vCPU-hr".
	Unit string `json:"unit"`
	// RateUSD is the per-Unit price, carrying enough decimals to be non-zero
	// for any priced meter ("0.0067"). "0" for a genuinely free tier.
	RateUSD string `json:"rateUsd"`
	// Quantity is the metered amount expressed in Unit ("436.26").
	Quantity string `json:"quantity"`
	// Included is the part of Quantity the workspace plan's included build
	// minutes covered, in Unit; empty when none did. CostUSD prices only the
	// rest.
	Included string `json:"included,omitempty"`
	// CostUSD is RateUSD × Quantity, formatted to cents. "0.00" is a real and
	// common value (free tiers, sub-cent usage) and is never omitted.
	CostUSD string `json:"costUsd"`
}

// MeterEstimate is one meter dimension's contribution to the estimated total.
type MeterEstimate struct {
	// Kind is "instance_seconds", "egress_bytes", "build_seconds",
	// "storage_gb_seconds", or "sandbox_compute_seconds".
	Kind string `json:"kind"`
	// Tier is the plan/tier id for instance_seconds meters; empty for flat-rate
	// meters (egress_bytes, build_seconds, storage_gb_seconds).
	Tier string `json:"tier,omitempty"`
	// ResourceKind is "service", "postgres", "key_value", or "sandbox".
	ResourceKind string `json:"resourceKind,omitempty"`
	// CostUSD is the estimated dollar cost for this meter, formatted to cents.
	CostUSD string `json:"costUsd"`
}

// Estimate computes a dollar estimate for the given usage rows by applying the
// price sheet. Rows are aggregated by (kind × tier × resourceKind) across all
// services before pricing. An unknown tier is treated as $0 (not an error).
// Meters whose rounded cost is below $0.01 are omitted from the breakdown but
// still counted toward TotalUSD. Meters is always a non-nil slice.
func (s *Sheet) Estimate(rows []store.UsageSummaryRow) EstimatedCost {
	return s.EstimateIncluding(rows, nil)
}

// Included is what a workspace plan includes a month before a meter is
// charged, by meter kind: build_seconds (Render's included pipeline minutes,
// w6/m308) and egress_bytes (its included outbound bandwidth, w6/m319).
type Included map[string]int64

// IncludedFor is what plan includes a month, keyed by every meter a plan can
// include. An unknown plan includes nothing.
func (s *Sheet) IncludedFor(plan string) Included {
	return Included{
		store.UsageKindBuildSeconds: s.buildIncluded[plan],
		store.UsageKindEgressBytes:  s.egressIncluded[plan],
	}
}

// HasAllowance reports whether a plan can include some of meter kind a month.
func HasAllowance(kind string) bool {
	_, ok := Default.IncludedFor("")[kind]
	return ok
}

// BuildCentsAbove is what seconds of build minutes above a plan's included ones
// cost, in whole cents spent: the overage a pipeline spend limit caps
// (w6/m309). A part-cent is not yet spent; the tolerance absorbs the float
// rate's representation error, so 100 minutes at $0.0035 is exactly 35.
func (s *Sheet) BuildCentsAbove(seconds, includedSeconds int64) int64 {
	if seconds <= includedSeconds {
		return 0
	}
	return int64(math.Floor(float64(seconds-includedSeconds)*s.build*100 + 1e-6))
}

// BytesPerGiB is the gigabyte bandwidth is billed, shown and included in: the
// unit of the Stripe egress_gib meter and of the estimate's "GB" line. Render
// quotes its allowance in GB (render.com/docs/outbound-bandwidth); reading
// that as GiB includes a little more, never less (docs/ADR030 §14).
const BytesPerGiB = 1 << 30

// PlanReader reads a workspace's plan; *core.Base satisfies it.
type PlanReader interface {
	WorkspacePlanOf(ctx context.Context, workspaceID string) (plan string, ok bool, err error)
}

// IncludedForWorkspace is what workspaceID's plan includes a month, the one
// answer the estimate, the Stripe emitter and the allowance sweepers share.
// Where no plan applies (none wired, the default tenant) it includes nothing.
func IncludedForWorkspace(ctx context.Context, plans PlanReader, workspaceID string) (Included, error) {
	if plans == nil {
		return Included{}, nil
	}
	plan, ok, err := plans.WorkspacePlanOf(ctx, workspaceID)
	if err != nil || !ok {
		return Included{}, err
	}
	return Default.IncludedFor(plan), nil
}

// EstimateIncluding is Estimate for a workspace whose plan includes included
// a month: those are free, so only usage above them is priced. Each allowance
// is spent on its meter's rows in order, and each resource's line records how
// much of its quantity it covered.
func (s *Sheet) EstimateIncluding(rows []store.UsageSummaryRow, included Included) EstimatedCost {
	rows, covered := applyIncluded(rows, included)
	type key struct{ kind, tier, resourceKind string }
	byKey := map[key]int64{}
	var order []key
	for _, r := range rows {
		k := key{r.Kind, r.Tier, r.ResourceKind}
		if _, seen := byKey[k]; !seen {
			order = append(order, k)
		}
		byKey[k] += r.Total
	}

	var totalRaw float64
	meters := make([]MeterEstimate, 0)
	for _, k := range order {
		qty := byKey[k]
		if qty == 0 {
			continue
		}
		rate := s.rateFor(k.kind, k.tier, k.resourceKind)
		cost := rate * float64(qty)
		totalRaw += cost
		if math.Round(cost*100) == 0 {
			continue // sub-cent contribution: skip meter entry, still counts toward total
		}
		meters = append(meters, MeterEstimate{
			Kind:         k.kind,
			Tier:         k.tier,
			ResourceKind: k.resourceKind,
			CostUSD:      FormatUSD(cost),
		})
	}
	return EstimatedCost{
		TotalUSD:  FormatUSD(totalRaw),
		Meters:    meters,
		Resources: s.resourceEstimates(rows, covered),
	}
}

// coveredKey is one resource's line of one meter kind.
type coveredKey struct{ serviceID, resourceKind, kind string }

// applyIncluded spends each allowance on its meter's rows in order. It returns
// the rows with each one's quantity cut to what is charged, and how much each
// allowance covered per resource line.
func applyIncluded(rows []store.UsageSummaryRow, included Included) ([]store.UsageSummaryRow, map[coveredKey]int64) {
	left := map[string]int64{}
	for kind, n := range included {
		if n > 0 {
			left[kind] = n
		}
	}
	if len(left) == 0 {
		return rows, nil
	}
	charged := make([]store.UsageSummaryRow, len(rows))
	covered := map[coveredKey]int64{}
	for i, r := range rows {
		charged[i] = r
		if r.Total <= 0 || left[r.Kind] <= 0 {
			continue
		}
		free := min(left[r.Kind], r.Total)
		left[r.Kind] -= free
		charged[i].Total -= free
		covered[coveredKey{r.ServiceID, r.ResourceKind, r.Kind}] += free
	}
	return charged, covered
}

// resourceEstimates breaks the same rows down by resource instead of by meter
// dimension. Rows are grouped by (serviceID × resourceKind) and, within a
// resource, by (kind × tier) — both in first-seen order, so the result is
// deterministic for a deterministic query.
func (s *Sheet) resourceEstimates(rows []store.UsageSummaryRow, covered map[coveredKey]int64) []ResourceEstimate {
	type resKey struct{ serviceID, resourceKind string }
	type lineKey struct{ kind, tier string }

	lines := map[resKey]map[lineKey]int64{}
	var resOrder []resKey
	lineOrder := map[resKey][]lineKey{}
	for _, r := range rows {
		if r.Total == 0 && covered[coveredKey{r.ServiceID, r.ResourceKind, r.Kind}] == 0 {
			continue
		}
		rk := resKey{r.ServiceID, r.ResourceKind}
		if _, seen := lines[rk]; !seen {
			lines[rk] = map[lineKey]int64{}
			resOrder = append(resOrder, rk)
		}
		lk := lineKey{r.Kind, r.Tier}
		if _, seen := lines[rk][lk]; !seen {
			lineOrder[rk] = append(lineOrder[rk], lk)
		}
		lines[rk][lk] += r.Total
	}

	out := make([]ResourceEstimate, 0, len(resOrder))
	for _, rk := range resOrder {
		var resRaw float64
		charges := make([]ChargeLine, 0, len(lineOrder[rk]))
		for _, lk := range lineOrder[rk] {
			qty := lines[rk][lk]
			rate := s.rateFor(lk.kind, lk.tier, rk.resourceKind)
			cost := rate * float64(qty)
			resRaw += cost
			unit := unitFor(lk.kind)
			line := ChargeLine{
				Kind:     lk.kind,
				Tier:     lk.tier,
				Unit:     unit.label,
				RateUSD:  formatRate(rate * unit.per),
				Quantity: formatQuantity(float64(qty) / unit.per),
				CostUSD:  FormatUSD(cost),
			}
			if free := covered[coveredKey{rk.serviceID, rk.resourceKind, lk.kind}]; free > 0 {
				// Quantity stays what was used; the plan's allowance covered
				// Included of it, and Cost prices only the rest.
				line.Quantity = formatQuantity(float64(qty+free) / unit.per)
				line.Included = formatQuantity(float64(free) / unit.per)
			}
			charges = append(charges, line)
		}
		out = append(out, ResourceEstimate{
			ServiceID:    rk.serviceID,
			ResourceKind: rk.resourceKind,
			CostUSD:      FormatUSD(resRaw),
			Charges:      charges,
		})
	}
	return out
}

// meterUnit is the human-scale unit one meter kind is billed in, and how many
// raw meter units make one of them.
type meterUnit struct {
	label string
	per   float64
}

// unitFor maps a meter kind to its display unit. The divisors mirror the
// derivations documented in pricing.yaml: 3600 s/h, 1 GiB = 2^30 bytes, 60
// s/min, one pricing month = 730 h = 2,628,000 s, and 3,600,000
// milli-vCPU-seconds per vCPU-hour.
func unitFor(kind string) meterUnit {
	switch kind {
	case store.UsageKindInstanceSeconds:
		return meterUnit{"hr", 3600}
	case store.UsageKindEgressBytes:
		return meterUnit{"GB", BytesPerGiB}
	case store.UsageKindBuildSeconds:
		return meterUnit{"min", 60}
	case store.UsageKindStorageGBSeconds, store.UsageKindDiskGBSeconds, store.UsageKindSandboxSnapshotGBSeconds:
		return meterUnit{"GB-mo", 2628000}
	case store.UsageKindSandboxMemoryGBSeconds:
		return meterUnit{"GiB-hr", 3600}
	case store.UsageKindSandboxComputeSeconds:
		return meterUnit{"vCPU-hr", 3600000}
	case store.UsageKindAgentModelInputTokens, store.UsageKindAgentModelOutputTokens,
		store.UsageKindAgentModelCacheReadTokens, store.UsageKindAgentModelCacheWriteTokens:
		return meterUnit{"1M tokens", tokensPerMillion}
	case store.UsageKindAgentModelCostMicroUSD:
		return meterUnit{"USD", microUSDPerUSD}
	}
	return meterUnit{"", 1}
}

// formatRate renders a per-unit price. Rates on this sheet span four orders of
// magnitude ($0.21/GB-mo down to $0.0067/hr), so a fixed width would either
// bury the large ones in zeros or round the small ones to "0.00" — and a rate
// rounded that hard stops multiplying out to the cost beside it, which is
// exactly the arithmetic a reader checks on a bill.
func formatRate(v float64) string {
	if v == 0 {
		return "0"
	}
	return formatDecimal(v, 4, 2)
}

// formatQuantity renders a metered amount in its display unit, at two decimals
// for everyday values and widening for fractional ones (a few MB of egress is
// real usage and must not read as zero).
func formatQuantity(v float64) string {
	if v == 0 {
		return "0"
	}
	return formatDecimal(v, 3, 2)
}

// formatDecimal renders v in fixed notation with at least minDecimals places,
// widened as far as it takes to show sig significant digits, then trimmed back
// of any zeros that merely pad them.
func formatDecimal(v float64, sig, minDecimals int) string {
	decimals := minDecimals
	if exp := int(math.Floor(math.Log10(math.Abs(v)))); exp < 0 {
		if d := sig - 1 - exp; d > decimals {
			decimals = d
		}
	}
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	for decimals > minDecimals && strings.HasSuffix(s, "0") {
		decimals--
		s = strconv.FormatFloat(v, 'f', decimals, 64)
	}
	return s
}

func (s *Sheet) rateFor(kind, tier, resourceKind string) float64 {
	switch kind {
	case store.UsageKindInstanceSeconds:
		rate, _ := s.instanceRate(tier, resourceKind)
		return rate
	case store.UsageKindEgressBytes:
		return s.bandwidth
	case store.UsageKindBuildSeconds:
		return s.build
	case store.UsageKindStorageGBSeconds:
		// A Free Postgres or Key Value includes its fixed storage at $0, as
		// Render's Free Postgres does (w4/427, ADR030).
		if tier == "free" {
			return 0
		}
		if resourceKind == store.ResourceKindPostgres || resourceKind == store.ResourceKindKeyValue {
			return s.storage
		}
	case store.UsageKindSandboxComputeSeconds:
		if resourceKind == store.ResourceKindSandbox {
			return s.sandbox
		}
	case store.UsageKindSandboxSnapshotGBSeconds:
		// The memory kind (sandbox_memory_gb_seconds) has no case: it is shown at
		// $0 until the used-CPU meter lets billing switch (w22/m314), because the
		// weighted compute charge above already includes memory.
		if resourceKind == store.ResourceKindSandbox {
			return s.sandboxSnapshot
		}
	case store.UsageKindDiskGBSeconds:
		// A service disk's flat per-GB rate: no tier dimension, because Render
		// prices a disk by size alone regardless of the plan it hangs off.
		if resourceKind == store.ResourceKindService {
			return s.disk
		}
	case store.UsageKindAgentModelCostMicroUSD:
		// The row already carries the marked-up customer price (ADR091 D1);
		// the token rows beside it are display only and rate $0.
		if resourceKind == store.ResourceKindAgentSession {
			return 1.0 / microUSDPerUSD
		}
	}
	return 0
}

// FormatUSD renders a USD amount rounded half away from zero to the cent
// ("1.23", "-0.42"). It is the one USD formatter every agent-model and usage
// read uses, so the same spend prints the same string on every surface. An
// amount that rounds to zero prints "0.00", never "-0.00".
func FormatUSD(v float64) string {
	cents := math.Round(v * 100)
	if cents == 0 {
		return "0.00"
	}
	return fmt.Sprintf("%.2f", cents/100)
}

// --- YAML loading ---

type sheetFile struct {
	Version string `json:"version"`
	Compute []struct {
		Tier         string  `json:"tier"`
		USDPerSecond float64 `json:"usdPerSecond"`
	} `json:"compute"`
	Postgres []struct {
		Tier         string  `json:"tier"`
		USDPerSecond float64 `json:"usdPerSecond"`
	} `json:"postgres"`
	KeyValue []struct {
		Tier         string  `json:"tier"`
		USDPerSecond float64 `json:"usdPerSecond"`
	} `json:"keyvalue"`
	Bandwidth struct {
		USDPerByte float64 `json:"usdPerByte"`
		IncludedGB []struct {
			Plan string `json:"plan"`
			GB   int64  `json:"gb"`
		} `json:"includedGB"`
	} `json:"bandwidth"`
	Build struct {
		USDPerSecond    float64 `json:"usdPerSecond"`
		IncludedMinutes []struct {
			Plan    string `json:"plan"`
			Minutes int64  `json:"minutes"`
		} `json:"includedMinutes"`
	} `json:"build"`
	Storage struct {
		USDPerGBSecond float64 `json:"usdPerGBSecond"`
	} `json:"storage"`
	Sandbox struct {
		USDPerWeightedSecond float64 `json:"usdPerWeightedSecond"`
	} `json:"sandbox"`
	Disk struct {
		USDPerGBSecond float64 `json:"usdPerGBSecond"`
	} `json:"disk"`
	SandboxSnapshot struct {
		USDPerGBSecond float64 `json:"usdPerGBSecond"`
	} `json:"sandboxSnapshot"`
	Workspace []struct {
		Plan        string  `json:"plan"`
		USDPerMonth float64 `json:"usdPerMonth"`
	} `json:"workspace"`
	AgentModel struct {
		Markup             float64 `json:"markup"`
		MaxCapMultiplier   float64 `json:"maxCapMultiplier"`
		MonthlySpendCapUSD []struct {
			Plan string  `json:"plan"`
			USD  float64 `json:"usd"`
		} `json:"monthlySpendCapUsd"`
		TrustTiers     []tierFile      `json:"trustTiers"`
		UpstreamPrices []upstreamPrice `json:"upstreamPrices"`
	} `json:"agentModel"`
}

func mustLoad(raw []byte) *Sheet {
	s, err := parseSheet(raw)
	if err != nil {
		panic(fmt.Sprintf("pricing: %v", err))
	}
	return s
}

func parseSheet(raw []byte) (*Sheet, error) {
	var f sheetFile
	if err := yaml.UnmarshalStrict(raw, &f); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if f.Version == "" {
		return nil, fmt.Errorf("version field is required")
	}
	s := &Sheet{
		compute:         make(map[string]float64, len(f.Compute)),
		postgres:        make(map[string]float64, len(f.Postgres)),
		keyvalue:        make(map[string]float64, len(f.KeyValue)),
		bandwidth:       f.Bandwidth.USDPerByte,
		build:           f.Build.USDPerSecond,
		storage:         f.Storage.USDPerGBSecond,
		sandbox:         f.Sandbox.USDPerWeightedSecond,
		disk:            f.Disk.USDPerGBSecond,
		sandboxSnapshot: f.SandboxSnapshot.USDPerGBSecond,
		workspace:       make(map[string]float64, len(f.Workspace)),

		agentModelMarkup: f.AgentModel.Markup,
	}
	if rate := f.Sandbox.USDPerWeightedSecond; rate > 0 {
		r, ok := new(big.Rat).SetString(strconv.FormatFloat(rate, 'f', -1, 64))
		if !ok {
			return nil, fmt.Errorf("sandbox has a non-finite rate")
		}
		s.sandboxUSD = r
	}
	if m := f.AgentModel.Markup; math.IsNaN(m) || m < minAgentModelMarkup || m > maxAgentModelMarkup {
		return nil, fmt.Errorf("agentModel.markup %v must be between %d and %d", m, minAgentModelMarkup, maxAgentModelMarkup)
	}
	if err := s.parseSpendControls(f); err != nil {
		return nil, err
	}
	s.agentModelUpstream = make(map[string]upstreamPrice, len(f.AgentModel.UpstreamPrices))
	for _, p := range f.AgentModel.UpstreamPrices {
		if _, dup := s.agentModelUpstream[p.Model]; p.Model == "" || dup {
			return nil, fmt.Errorf("agentModel.upstreamPrices: empty or duplicate model %q", p.Model)
		}
		for _, rate := range []float64{p.Input, p.CachedInput, p.Output} {
			if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
				return nil, fmt.Errorf("agentModel.upstreamPrices: %q has an invalid rate", p.Model)
			}
		}
		s.agentModelUpstream[p.Model] = p
	}
	for _, e := range f.Compute {
		if e.Tier == "" {
			return nil, fmt.Errorf("compute entry has empty tier")
		}
		s.compute[e.Tier] = e.USDPerSecond
	}
	for _, e := range f.Postgres {
		if e.Tier == "" {
			return nil, fmt.Errorf("postgres entry has empty tier")
		}
		s.postgres[e.Tier] = e.USDPerSecond
	}
	for _, e := range f.KeyValue {
		if e.Tier == "" {
			return nil, fmt.Errorf("keyvalue entry has empty tier")
		}
		s.keyvalue[e.Tier] = e.USDPerSecond
		if e.USDPerSecond > 0 {
			r, ok := new(big.Rat).SetString(strconv.FormatFloat(e.USDPerSecond, 'f', -1, 64))
			if !ok {
				return nil, fmt.Errorf("keyvalue tier %s has a non-finite rate", e.Tier)
			}
			if s.keyvalueMicroUSD == nil {
				s.keyvalueMicroUSD = map[string]*big.Rat{}
			}
			s.keyvalueMicroUSD[e.Tier] = r.Mul(r, big.NewRat(1_000_000, 1))
		}
	}
	s.buildIncluded = make(map[string]int64, len(f.Build.IncludedMinutes))
	for _, e := range f.Build.IncludedMinutes {
		if !slices.Contains(store.WorkspacePlans, e.Plan) {
			return nil, fmt.Errorf("build includedMinutes plan %q is not a workspace plan", e.Plan)
		}
		if _, dup := s.buildIncluded[e.Plan]; dup {
			return nil, fmt.Errorf("build includedMinutes lists plan %s twice", e.Plan)
		}
		if e.Minutes < 0 {
			return nil, fmt.Errorf("build includedMinutes for %s is negative", e.Plan)
		}
		s.buildIncluded[e.Plan] = e.Minutes * 60
	}
	s.egressIncluded = make(map[string]int64, len(f.Bandwidth.IncludedGB))
	for _, e := range f.Bandwidth.IncludedGB {
		if !slices.Contains(store.WorkspacePlans, e.Plan) {
			return nil, fmt.Errorf("bandwidth includedGB plan %q is not a workspace plan", e.Plan)
		}
		if _, dup := s.egressIncluded[e.Plan]; dup {
			return nil, fmt.Errorf("bandwidth includedGB lists plan %s twice", e.Plan)
		}
		if e.GB < 0 {
			return nil, fmt.Errorf("bandwidth includedGB for %s is negative", e.Plan)
		}
		s.egressIncluded[e.Plan] = e.GB * BytesPerGiB
	}
	for _, e := range f.Workspace {
		if e.Plan == "" {
			return nil, fmt.Errorf("workspace entry has empty plan")
		}
		switch e.Plan {
		case store.PlanHobby, store.PlanPro, store.PlanScale:
		default:
			return nil, fmt.Errorf("workspace plan %q is not a cataloged monthly SKU", e.Plan)
		}
		if e.USDPerMonth < 0 {
			return nil, fmt.Errorf("workspace plan %s has negative usdPerMonth", e.Plan)
		}
		s.workspace[e.Plan] = e.USDPerMonth
	}
	for _, plan := range []string{store.PlanHobby, store.PlanPro, store.PlanScale} {
		if _, ok := s.workspace[plan]; !ok {
			return nil, fmt.Errorf("workspace is missing plan %s", plan)
		}
	}
	if s.workspace[store.PlanHobby] != 0 {
		return nil, fmt.Errorf("workspace hobby usdPerMonth must be 0")
	}
	if s.workspace[store.PlanPro] <= 0 || s.workspace[store.PlanScale] <= 0 {
		return nil, fmt.Errorf("workspace pro and scale usdPerMonth must be positive")
	}
	return s, nil
}
