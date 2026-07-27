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
// fixed discount — 30% off compute/Postgres/KeyValue/build-minute/Postgres
// storage lines, 90% off bandwidth. It is backend-only (the operator never
// imports it) and produces the advisory estimate shown beside Stripe's real
// invoice data. The same sheet also names the Stripe catalog dimensions; Stripe
// owns authoritative rating, invoicing, and collection (ADR040).
//
// Usage: call Default.Estimate(rows) with the UsageSummaryRows from the store
// to get an EstimatedCost. An unknown tier contributes $0 (not an error).
package pricing

import (
	_ "embed"
	"fmt"
	"math"
	"sort"

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
	compute   map[string]float64 // tier ID → $/second for instance_seconds (service)
	postgres  map[string]float64 // tier ID → $/second for instance_seconds (postgres)
	keyvalue  map[string]float64 // tier ID → $/second for instance_seconds (key_value)
	bandwidth float64            // $/byte for egress_bytes
	build     float64            // $/second for build_seconds
	storage   float64            // $/GB-second for storage_gb_seconds
}

// BillableMeterNames returns the Stripe lookup/event names represented by this
// price sheet. Free/zero-rate instance tiers are omitted because no charge is
// due and therefore no Stripe meter or subscription item exists. The stable
// lexical order makes catalog validation and tests deterministic.
func (s *Sheet) BillableMeterNames() []string {
	names := make([]string, 0, len(s.compute)+len(s.postgres)+len(s.keyvalue)+3)
	appendTiers := func(resourceKind string, tiers map[string]float64) {
		for tier, rate := range tiers {
			if rate > 0 {
				names = append(names, "instance_seconds."+resourceKind+"."+tier)
			}
		}
	}
	appendTiers(store.ResourceKindService, s.compute)
	appendTiers(store.ResourceKindPostgres, s.postgres)
	appendTiers(store.ResourceKindKeyValue, s.keyvalue)
	if s.bandwidth > 0 {
		names = append(names, "egress_gib")
	}
	if s.build > 0 {
		names = append(names, "build_seconds")
	}
	if s.storage > 0 {
		names = append(names, "storage_gb_hours")
	}
	sort.Strings(names)
	return names
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
}

// MeterEstimate is one meter dimension's contribution to the estimated total.
type MeterEstimate struct {
	// Kind is "instance_seconds", "egress_bytes", "build_seconds", or
	// "storage_gb_seconds".
	Kind string `json:"kind"`
	// Tier is the plan/tier id for instance_seconds meters; empty for flat-rate
	// meters (egress_bytes, build_seconds, storage_gb_seconds).
	Tier string `json:"tier,omitempty"`
	// ResourceKind is "service", "postgres", or "key_value".
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
			CostUSD:      formatUSD(cost),
		})
	}
	return EstimatedCost{
		TotalUSD: formatUSD(totalRaw),
		Meters:   meters,
	}
}

func (s *Sheet) rateFor(kind, tier, resourceKind string) float64 {
	switch kind {
	case store.UsageKindInstanceSeconds:
		switch resourceKind {
		case store.ResourceKindService:
			return s.compute[tier]
		case store.ResourceKindPostgres:
			return s.postgres[tier]
		case store.ResourceKindKeyValue:
			return s.keyvalue[tier]
		}
	case store.UsageKindEgressBytes:
		return s.bandwidth
	case store.UsageKindBuildSeconds:
		return s.build
	case store.UsageKindStorageGBSeconds:
		if resourceKind == store.ResourceKindPostgres || resourceKind == store.ResourceKindKeyValue {
			return s.storage
		}
	}
	return 0
}

// formatUSD renders a float as a USD amount rounded to the nearest cent.
func formatUSD(v float64) string {
	return fmt.Sprintf("%.2f", math.Round(v*100)/100)
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
	} `json:"bandwidth"`
	Build struct {
		USDPerSecond float64 `json:"usdPerSecond"`
	} `json:"build"`
	Storage struct {
		USDPerGBSecond float64 `json:"usdPerGBSecond"`
	} `json:"storage"`
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
		compute:   make(map[string]float64, len(f.Compute)),
		postgres:  make(map[string]float64, len(f.Postgres)),
		keyvalue:  make(map[string]float64, len(f.KeyValue)),
		bandwidth: f.Bandwidth.USDPerByte,
		build:     f.Build.USDPerSecond,
		storage:   f.Storage.USDPerGBSecond,
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
	}
	return s, nil
}
