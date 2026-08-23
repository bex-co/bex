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
	"math"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// MonthSeconds is one pricing month (730 h), the same constant the sheet's
// per-second rates were derived from (pricing.yaml).
const MonthSeconds = 2_628_000

// Variable-cost reasons: resources whose real cost depends on runtime behavior
// are listed but excluded from the monthly total, mirroring Render's blueprint
// estimate ("Excluding scaling and cron jobs").
const (
	VariableAutoscaling   = "autoscaling"    // autoscaler decides the instance count
	VariableMultiInstance = "multi_instance" // fixed numInstances > 1; the line prices one instance
	VariableCron          = "cron"           // billed per-second only while runs execute
)

// MonthlyResource is one declared (not yet metered) resource to estimate — the
// forward-looking counterpart of a store.UsageSummaryRow, fed from a Blueprint
// preview's parsed plans.
type MonthlyResource struct {
	// Name is the resource's blueprint name, used to label line items.
	Name string
	// ResourceKind is store.ResourceKindService, store.ResourceKindPostgres,
	// or store.ResourceKindKeyValue.
	ResourceKind string
	// Tier is the plan/tier id priced by the sheet. Free or unknown tiers
	// contribute nothing (consistent with rateFor's $0 policy).
	Tier string
	// TierLabel is the human display name for Tier ("pro-plus" → "Pro Plus");
	// empty falls back to Tier. Display naming lives with the caller — this
	// package prices, it does not name.
	TierLabel string
	// Instances is the fixed instance count (0 means 1). A count above one is
	// reported as a variable cost; the line still prices a single instance.
	Instances int
	// Autoscaling marks a service whose instance count the autoscaler owns.
	Autoscaling bool
	// Cron marks a cron service: listed as variable, never priced into the
	// total (it only bills per-second while runs execute).
	Cron bool
	// StorageGB is the provisioned volume size. Datastore kinds price it at the
	// used-storage rate; a service prices its attached DISK at the disk rate
	// (docs/ADR082-persistent-disks.md D8). The caller resolves the plan's floor when
	// the blueprint omits an explicit size.
	StorageGB int32
	// HighAvailability adds a standby line at the same price (ratio 1.0).
	HighAvailability bool
	// ReadReplicas adds one line per replica, each priced like the primary.
	ReadReplicas int
}

// MonthlyEstimate is the always-on monthly projection for a set of declared
// resources — what a Blueprint costs per month if everything runs continuously.
// Storage is priced on provisioned size, an upper bound of the metered
// used-bytes rate the invoice applies.
type MonthlyEstimate struct {
	// TotalUSD sums the line items, formatted to cents ("0.00" when all-free).
	TotalUSD string `json:"totalUsd"`
	// Lines are the priced rows; free/unknown tiers are filtered out. Always
	// a non-nil slice.
	Lines []MonthlyLine `json:"lines"`
	// Variable lists resources whose cost depends on runtime behavior,
	// excluded from TotalUSD. Always a non-nil slice.
	Variable []VariableCost `json:"variable"`
}

// MonthlyLine is one priced resource row.
type MonthlyLine struct {
	// Name is the blueprint resource name, suffixed for derived rows
	// ("db (standby)", "db (replica)").
	Name string `json:"name"`
	// ResourceKind is store.ResourceKindService / Postgres / KeyValue.
	ResourceKind string `json:"resourceKind"`
	// Tier is the priced plan/tier id.
	Tier string `json:"tier"`
	// TierLabel is the display name for Tier (falls back to Tier).
	TierLabel string `json:"tierLabel"`
	// MonthlyUSD is InstanceUSD + StorageUSD, formatted to cents.
	MonthlyUSD string `json:"monthlyUsd"`
	// InstanceUSD is the compute component, formatted to cents.
	InstanceUSD string `json:"instanceUsd"`
	// StorageUSD is the provisioned-storage component (datastores), formatted
	// to cents; omitted when zero.
	StorageUSD string `json:"storageUsd,omitempty"`
	// StorageGB is the provisioned size StorageUSD was computed from.
	StorageGB int32 `json:"storageGb,omitempty"`
}

// VariableCost is a resource listed but excluded from the monthly total.
type VariableCost struct {
	Name string `json:"name"`
	// Reason is VariableAutoscaling, VariableMultiInstance, or VariableCron.
	Reason string `json:"reason"`
}

// MonthlyEstimate projects declared resources to an always-on monthly cost.
// Free and unknown tiers produce no line (rateFor's $0 policy); cron and
// autoscaled/multi-instance resources surface in Variable instead of (or, for
// scaling, alongside) their base line.
func (s *Sheet) MonthlyEstimate(resources []MonthlyResource) MonthlyEstimate {
	var total float64
	lines := make([]MonthlyLine, 0, len(resources))
	variable := make([]VariableCost, 0)
	for _, r := range resources {
		switch {
		case r.Cron:
			variable = append(variable, VariableCost{Name: r.Name, Reason: VariableCron})
			continue
		case r.Autoscaling:
			variable = append(variable, VariableCost{Name: r.Name, Reason: VariableAutoscaling})
		case r.Instances > 1:
			variable = append(variable, VariableCost{Name: r.Name, Reason: VariableMultiInstance})
		}

		instanceUSD := s.rateFor(store.UsageKindInstanceSeconds, r.Tier, r.ResourceKind) * MonthSeconds
		if math.Round(instanceUSD*100) == 0 {
			// Free (or unknown) instance tier: the whole resource contributes
			// nothing — including its storage floor, matching Render's
			// blueprint estimate, which filters $0 plans out entirely.
			continue
		}
		var storageUSD float64
		switch {
		case r.ResourceKind == store.ResourceKindPostgres || r.ResourceKind == store.ResourceKindKeyValue:
			storageUSD = float64(r.StorageGB) * s.storage * MonthSeconds
		case r.ResourceKind == store.ResourceKindService:
			// A service's storage is its attached disk, priced at the disk rate
			// on PROVISIONED capacity — the one estimate line that equals its
			// invoice exactly, since both sides bill what was reserved
			// (docs/ADR082-persistent-disks.md D8).
			storageUSD = float64(r.StorageGB) * s.disk * MonthSeconds
		}
		lineUSD := instanceUSD + storageUSD

		label := r.TierLabel
		if label == "" {
			label = r.Tier
		}
		addLine := func(name string) {
			total += lineUSD
			line := MonthlyLine{
				Name:         name,
				ResourceKind: r.ResourceKind,
				Tier:         r.Tier,
				TierLabel:    label,
				MonthlyUSD:   formatUSD(lineUSD),
				InstanceUSD:  formatUSD(instanceUSD),
			}
			if math.Round(storageUSD*100) != 0 {
				line.StorageUSD = formatUSD(storageUSD)
				line.StorageGB = r.StorageGB
			}
			lines = append(lines, line)
		}
		addLine(r.Name)
		if r.HighAvailability {
			addLine(r.Name + " (standby)")
		}
		for i := 0; i < r.ReadReplicas; i++ {
			addLine(r.Name + " (replica)")
		}
	}
	return MonthlyEstimate{
		TotalUSD: formatUSD(total),
		Lines:    lines,
		Variable: variable,
	}
}
