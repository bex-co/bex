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

const testModel = "deepseek/deepseek-v4.1-flash-1017"

// sheetWithAgentModel is the shipped sheet with its (final) agentModel block
// replaced.
func sheetWithAgentModel(t *testing.T, block string) (*Sheet, error) {
	t.Helper()
	i := strings.Index(string(sheetYAML), "\nagentModel:\n")
	if i < 0 {
		t.Fatal("pricing.yaml no longer ends with the agentModel block this test rewrites")
	}
	return parseSheet([]byte(string(sheetYAML[:i+1]) + "agentModel:\n" + block))
}

// spendBlock is a minimal valid spend-control block the validation cases edit.
const spendBlock = "  maxCapMultiplier: 10\n" +
	"  monthlySpendCapUsd:\n    - plan: hobby\n      usd: 20\n    - plan: pro\n      usd: 200\n    - plan: scale\n      usd: 2000\n" +
	"  trustTiers:\n    - tier: T0\n      minLifetimePaidUsd: 0\n      minPaidCycleInvoices: 0\n      capCeilingUsd: 10\n      maxConcurrentSessions: 2\n" +
	"    - tier: T1\n      minLifetimePaidUsd: 10\n      minPaidCycleInvoices: 0\n"

func agentSessionRows(model string) []store.UsageSummaryRow {
	row := func(kind string, total int64) store.UsageSummaryRow {
		return store.UsageSummaryRow{ServiceID: "ags-x", Kind: kind, Tier: model, ResourceKind: store.ResourceKindAgentSession, Total: total}
	}
	return []store.UsageSummaryRow{
		row(store.UsageKindAgentModelInputTokens, 10_000_000),
		row(store.UsageKindAgentModelOutputTokens, 2_000_000),
		row(store.UsageKindAgentModelCacheReadTokens, 50_000_000),
		row(store.UsageKindAgentModelCacheWriteTokens, 1_000_000),
		row(store.UsageKindAgentModelCostMicroUSD, 4_567_890), // $4.57 already marked up
	}
}

// ADR091 D1/D3: the shipped sheet prices agent models by cost × 1.30 through
// the one cost meter. Token lines stay visible at $0; the cost line reads
// rate 1 × quantity USD.
func TestAgentModelCostMeterIsTheOnlyBilledLine(t *testing.T) {
	if got := Default.AgentModelMarkup(); got != 1.30 {
		t.Fatalf("shipped markup = %v, want 1.30", got)
	}
	est := Default.Estimate(agentSessionRows(testModel))
	if est.TotalUSD != "4.57" {
		t.Fatalf("estimate total = %s, want 4.57 (%+v)", est.TotalUSD, est)
	}
	if len(est.Meters) != 1 || est.Meters[0].Kind != store.UsageKindAgentModelCostMicroUSD {
		t.Fatalf("estimate meters = %+v, want only the cost meter", est.Meters)
	}
	charges := est.Resources[0].Charges
	if len(charges) != 5 {
		t.Fatalf("charge lines = %+v, want 4 token lines + the cost line", charges)
	}
	for _, line := range charges[:4] {
		if line.Unit != "1M tokens" || line.RateUSD != "0" || line.CostUSD != "0.00" {
			t.Fatalf("token line = %+v, want a $0 display line", line)
		}
	}
	if line := charges[4]; line.Unit != "USD" || line.RateUSD != "1.00" || line.Quantity != "4.57" || line.CostUSD != "4.57" {
		t.Fatalf("cost line = %+v, want $1.00 × 4.57 USD = $4.57", line)
	}
	names := Default.BillableMeterNames()
	if !slices.Contains(names, store.UsageKindAgentModelCostMicroUSD) {
		t.Fatalf("BillableMeterNames missing %s: %v", store.UsageKindAgentModelCostMicroUSD, names)
	}
	for _, name := range names {
		if strings.HasPrefix(name, "agent_model_") && name != store.UsageKindAgentModelCostMicroUSD {
			t.Fatalf("token meter %s must not be billable (ADR091 D3)", name)
		}
	}
}

func TestAgentModelSheetValidation(t *testing.T) {
	if _, err := sheetWithAgentModel(t, "  markup: 1.3\n"+spendBlock); err != nil {
		t.Fatalf("valid block refused: %v", err)
	}
	for name, block := range map[string]string{
		"missing markup":      spendBlock,
		"markup below cost":   "  markup: 0.9\n" + spendBlock,
		"markup typo":         "  markup: 13\n" + spendBlock,
		"per-model prices":    "  markup: 1.3\n  models:\n    - model: m\n" + spendBlock,
		"token quota is gone": "  markup: 1.3\n  monthlyTokenQuota: []\n" + spendBlock,
		"no multiplier":       "  markup: 1.3\n" + strings.Replace(spendBlock, "  maxCapMultiplier: 10\n", "", 1),
		"unknown plan":        "  markup: 1.3\n" + strings.Replace(spendBlock, "plan: scale", "plan: gold", 1),
		"zero cap":            "  markup: 1.3\n" + strings.Replace(spendBlock, "usd: 200\n", "usd: 0\n", 1),
		"no tiers":            "  markup: 1.3\n" + spendBlock[:strings.Index(spendBlock, "  trustTiers:")],
		"T0 needs payment":    "  markup: 1.3\n" + strings.Replace(spendBlock, "minLifetimePaidUsd: 0\n", "minLifetimePaidUsd: 1\n", 1),
		"ladder not rising":   "  markup: 1.3\n" + strings.Replace(spendBlock, "minLifetimePaidUsd: 10\n", "minLifetimePaidUsd: 0\n", 1),
		"zero concurrency":    "  markup: 1.3\n" + strings.Replace(spendBlock, "maxConcurrentSessions: 2", "maxConcurrentSessions: 0", 1),
	} {
		if _, err := sheetWithAgentModel(t, block); err == nil {
			t.Errorf("%s: parse accepted an invalid agentModel block", name)
		}
	}
}
