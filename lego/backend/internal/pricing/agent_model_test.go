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

// emptyAgentModelBlock is the shipped agentModel block the sheet tests rewrite.
const emptyAgentModelBlock = "agentModel:\n  markup: 1.30\n  monthlyTokenQuota: []\n"

// sheetWithAgentModel is the shipped sheet with its agentModel block replaced.
func sheetWithAgentModel(t *testing.T, block string) (*Sheet, error) {
	t.Helper()
	raw := strings.Replace(string(sheetYAML), emptyAgentModelBlock, "agentModel:\n"+block, 1)
	if raw == string(sheetYAML) {
		t.Fatal("pricing.yaml no longer ends with the agentModel block this test rewrites")
	}
	return parseSheet([]byte(raw))
}

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
	for _, plan := range store.WorkspacePlans {
		if _, limited := Default.AgentModelMonthlyTokenQuota(plan); limited {
			t.Fatalf("default sheet caps plan %s", plan)
		}
	}
}

func TestAgentModelQuotaStillParses(t *testing.T) {
	s, err := sheetWithAgentModel(t, "  markup: 1.5\n  monthlyTokenQuota:\n    - plan: hobby\n      tokens: 1000\n")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.AgentModelMarkup() != 1.5 {
		t.Fatalf("markup = %v, want 1.5", s.AgentModelMarkup())
	}
	if quota, limited := s.AgentModelMonthlyTokenQuota(store.PlanHobby); !limited || quota != 1000 {
		t.Fatalf("hobby quota = %d/%v, want 1000", quota, limited)
	}
	if _, limited := s.AgentModelMonthlyTokenQuota(store.PlanPro); limited {
		t.Fatal("a plan without a quota entry must be unlimited")
	}
}

func TestAgentModelSheetValidation(t *testing.T) {
	for name, block := range map[string]string{
		"missing markup":    "  monthlyTokenQuota: []\n",
		"markup below cost": "  markup: 0.9\n",
		"markup typo":       "  markup: 13\n",
		"per-model prices":  "  markup: 1.3\n  models:\n    - model: m\n",
		"unknown plan":      "  markup: 1.3\n  monthlyTokenQuota:\n    - plan: gold\n      tokens: 5\n",
		"zero quota":        "  markup: 1.3\n  monthlyTokenQuota:\n    - plan: pro\n      tokens: 0\n",
	} {
		if _, err := sheetWithAgentModel(t, block); err == nil {
			t.Errorf("%s: parse accepted an invalid agentModel block", name)
		}
	}
}
