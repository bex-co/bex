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

package keyvalue

import (
	"context"
	"fmt"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
)

// KeyValueInstanceType is the display-shaped projection of one lego/types/tiers
// Valkey tier — the create dialog's plan-picker source, the managed key-value
// sibling of postgres.DatabaseInstanceType. Render's dashboard hardcodes its own
// Key Value plan list (no public REST/MCP equivalent to mirror byte-for-byte),
// so this is a bex extension, documented as such — never a fourth hardcoded copy
// of the ladder the tiers catalog collapsed. ID is the KeyValue CRD's spec.plan
// spelling (what createKeyValue accepts), matching the other fields.
type KeyValueInstanceType struct {
	ID        string
	Name      string
	CPU       string
	Memory    string
	StorageGB int32
	// MonthlyUSD is the always-on monthly price from pricing.yaml, "" for an
	// unlisted tier (w4/156, the postgres sibling's rule).
	MonthlyUSD string
	// Available is false for a paused tier, which UnavailableReason explains
	// (compute's InstanceType precedent).
	Available         bool
	UnavailableReason string
}

// InstanceTypeUnavailableCode is the stable code for a request selecting a
// paused Key Value tier — the same code a paused compute tier answers.
const InstanceTypeUnavailableCode = "INSTANCE_TYPE_UNAVAILABLE"

// pausedPlanMessage is the customer-facing sentence for a paused tier, "" when
// the tier is selectable.
func pausedPlanMessage(plan string) string {
	if !tiers.Valkey.Paused(plan) {
		return ""
	}
	selectable := tiers.Valkey.SelectableIDs()
	return fmt.Sprintf("Key Value plan %q is temporarily unavailable; choose %s or a smaller plan.", kvTierDisplayName(plan), kvTierDisplayName(selectable[len(selectable)-1]))
}

// ErrPausedPlan refuses a new selection of a paused tier on every surface (API
// and Blueprint); a store already on it (current) may re-send its own plan.
func ErrPausedPlan(plan, current string) error {
	if plan == current {
		return nil
	}
	if msg := pausedPlanMessage(plan); msg != "" {
		return core.NewBadRequestError(InstanceTypeUnavailableCode, msg, map[string]any{"plan": plan, "field": "plan"})
	}
	return nil
}

// InstanceTypes lists every plan in the shared Valkey catalog, in ladder order —
// read from lego/types/tiers, never a hardcoded copy here (the same rule
// apps.InstanceTypes and postgres.InstanceTypes follow).
func (s *Service) InstanceTypes(ctx context.Context) ([]KeyValueInstanceType, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	ids := tiers.Valkey.IDs()
	out := make([]KeyValueInstanceType, len(ids))
	for i, id := range ids {
		t, _ := tiers.Valkey.ByID(id)
		monthlyUSD, _ := pricing.Default.InstanceMonthlyUSD(id, store.ResourceKindKeyValue)
		reason := pausedPlanMessage(id)
		out[i] = KeyValueInstanceType{
			Available:         reason == "",
			UnavailableReason: reason,
			MonthlyUSD:        monthlyUSD,
			ID:                t.ID,
			Name:              kvTierDisplayName(id),
			CPU:               t.CPU,
			Memory:            t.Memory,
			StorageGB:         t.StorageGB,
		}
	}
	return out, nil
}

// kvTierDisplayName is a Valkey plan id for display: Render's names, e.g.
// "standard" -> "Standard", "pro_plus" -> "Pro Plus", "3g" -> "3 GB".
func kvTierDisplayName(id string) string {
	if size, ok := strings.CutSuffix(id, "g"); ok && size != "" && strings.Trim(size, "0123456789") == "" {
		return size + " GB"
	}
	words := strings.Split(id, "_")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
