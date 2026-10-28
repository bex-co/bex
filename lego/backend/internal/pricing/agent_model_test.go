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

// pricedSheet is the shipped sheet plus one priced model and a Hobby quota,
// i.e. what pricing.yaml becomes once the user sets numbers.
func pricedSheet(t *testing.T) *Sheet {
	t.Helper()
	raw := strings.Replace(string(sheetYAML), "agentModel:\n  models: []\n  monthlyTokenQuota: []\n", `agentModel:
  models:
    - model: `+testModel+`
      usdPerMillionInputTokens: 0.30
      usdPerMillionOutputTokens: 1.20
      usdPerMillionCacheReadTokens: 0.006
      usdPerMillionCacheWriteTokens: 0
  monthlyTokenQuota:
    - plan: hobby
      tokens: 1000
`, 1)
	if raw == string(sheetYAML) {
		t.Fatal("pricing.yaml no longer ends with the empty agentModel block this test rewrites")
	}
	s, err := parseSheet([]byte(raw))
	if err != nil {
		t.Fatalf("parse priced sheet: %v", err)
	}
	return s
}

func tokenRows(model string) []store.UsageSummaryRow {
	row := func(kind string, total int64) store.UsageSummaryRow {
		return store.UsageSummaryRow{ServiceID: "ags-x", Kind: kind, Tier: model, ResourceKind: store.ResourceKindAgentSession, Total: total}
	}
	return []store.UsageSummaryRow{
		row(store.UsageKindAgentModelInputTokens, 10_000_000),     // $3.00
		row(store.UsageKindAgentModelOutputTokens, 2_000_000),     // $2.40
		row(store.UsageKindAgentModelCacheReadTokens, 50_000_000), // $0.30
		row(store.UsageKindAgentModelCacheWriteTokens, 1_000_000), // $0 (unpriced kind)
	}
}

// The shipped sheet meters but never charges: no model priced, no Stripe
// lookup key, no plan quota.
func TestAgentModelTokensAreUnbilledByDefault(t *testing.T) {
	est := Default.Estimate(tokenRows(testModel))
	if est.TotalUSD != "0.00" || len(est.Meters) != 0 {
		t.Fatalf("default estimate = %+v, want $0.00 with no meter lines", est)
	}
	if len(est.Resources) != 1 || len(est.Resources[0].Charges) != 4 || est.Resources[0].Charges[0].Unit != "1M tokens" {
		t.Fatalf("default estimate must still show the metered token lines: %+v", est.Resources)
	}
	for _, name := range Default.BillableMeterNames() {
		if strings.HasPrefix(name, "agent_model_") {
			t.Fatalf("default sheet exposes billable token meter %s", name)
		}
	}
	for _, plan := range store.WorkspacePlans {
		if _, limited := Default.AgentModelMonthlyTokenQuota(plan); limited {
			t.Fatalf("default sheet caps plan %s", plan)
		}
	}
}

func TestPricedAgentModelEstimatesAndExportsMeters(t *testing.T) {
	s := pricedSheet(t)
	est := s.Estimate(tokenRows(testModel))
	if est.TotalUSD != "5.70" {
		t.Fatalf("priced estimate total = %s, want 5.70 (%+v)", est.TotalUSD, est)
	}
	if len(est.Meters) != 3 {
		t.Fatalf("priced estimate meters = %+v, want input/output/cache-read lines", est.Meters)
	}
	line := est.Resources[0].Charges[0]
	if line.RateUSD != "0.30" || line.Quantity != "10.00" || line.CostUSD != "3.00" {
		t.Fatalf("input charge line = %+v, want $0.30 × 10.00 (1M tokens) = $3.00", line)
	}
	// Another model's tokens on the same sheet stay $0.
	if other := s.Estimate(tokenRows("openai/gpt-x")); other.TotalUSD != "0.00" {
		t.Fatalf("unpriced model billed: %+v", other)
	}
	names := s.BillableMeterNames()
	for _, want := range []string{
		"agent_model_input_tokens.deepseek_deepseek-v4.1-flash-1017",
		"agent_model_output_tokens.deepseek_deepseek-v4.1-flash-1017",
		"agent_model_cache_read_tokens.deepseek_deepseek-v4.1-flash-1017",
	} {
		if !slices.Contains(names, want) {
			t.Errorf("BillableMeterNames missing %s: %v", want, names)
		}
	}
	if slices.Contains(names, "agent_model_cache_write_tokens.deepseek_deepseek-v4.1-flash-1017") {
		t.Error("a zero-priced token kind must not get a Stripe meter")
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
		"empty model":    "  models:\n    - model: \"\"\n",
		"negative price": "  models:\n    - model: m\n      usdPerMillionInputTokens: -1\n",
		"duplicate":      "  models:\n    - model: m\n    - model: m\n",
		"unknown plan":   "  monthlyTokenQuota:\n    - plan: gold\n      tokens: 5\n",
		"zero quota":     "  monthlyTokenQuota:\n    - plan: pro\n      tokens: 0\n",
	} {
		raw := strings.Replace(string(sheetYAML), "agentModel:\n  models: []\n  monthlyTokenQuota: []\n", "agentModel:\n"+block, 1)
		if _, err := parseSheet([]byte(raw)); err == nil {
			t.Errorf("%s: parse accepted an invalid agentModel block", name)
		}
	}
}

func TestAgentModelMeterNameFoldsModelPunctuation(t *testing.T) {
	if got := AgentModelMeterName(store.UsageKindAgentModelInputTokens, "Vendor/Model:free~x"); got != "agent_model_input_tokens.vendor_model_free_x" {
		t.Fatalf("meter name = %s", got)
	}
}
