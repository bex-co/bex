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

package usage

import (
	"math"
	"sort"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// AgentModelSummary is the period's platform-route agent model spend
// (ADR091 D6): the "Agent model" line of the Usage and Billing page, with its
// breakdown by model and token kind. Its dollars are the price sheet
// estimate's agent-session meter lines (one per model, each the sum of the
// requests' customer prices, agent_model_cost_microusd, rounded once by
// pricing), so the line and the estimate never disagree. Token counts are
// display only.
type AgentModelSummary struct {
	// MonthToDateUSD is the period's agent-model charge, to the cent: the sum
	// of the models' CostUSD.
	MonthToDateUSD string `json:"monthToDateUsd"`
	// Models is one entry per model id, highest cost first.
	Models []AgentModelBreakdown `json:"models"`
}

// AgentModelBreakdown is one model's share of the period.
type AgentModelBreakdown struct {
	Model            string `json:"model"`
	CostUSD          string `json:"costUsd"`
	InputTokens      int64  `json:"inputTokens"`
	OutputTokens     int64  `json:"outputTokens"`
	CacheReadTokens  int64  `json:"cacheReadTokens"`
	CacheWriteTokens int64  `json:"cacheWriteTokens"`
}

// summariseAgentModel folds rows' agent_session token meters per model and
// prices each model from est's agent-session cost meter lines. A model whose
// cost rounds below a cent has no estimate line and reads "0.00".
func summariseAgentModel(rows []store.UsageSummaryRow, est pricing.EstimatedCost) AgentModelSummary {
	byModel := map[string]*AgentModelBreakdown{}
	model := func(id string) *AgentModelBreakdown {
		m := byModel[id]
		if m == nil {
			m = &AgentModelBreakdown{Model: id, CostUSD: "0.00"}
			byModel[id] = m
		}
		return m
	}
	for _, r := range rows {
		if store.NormalizeResourceKind(r.ResourceKind) != store.ResourceKindAgentSession {
			continue
		}
		switch r.Kind {
		case store.UsageKindAgentModelInputTokens:
			model(r.Tier).InputTokens += r.Total
		case store.UsageKindAgentModelOutputTokens:
			model(r.Tier).OutputTokens += r.Total
		case store.UsageKindAgentModelCacheReadTokens:
			model(r.Tier).CacheReadTokens += r.Total
		case store.UsageKindAgentModelCacheWriteTokens:
			model(r.Tier).CacheWriteTokens += r.Total
		case store.UsageKindAgentModelCostMicroUSD:
			model(r.Tier)
		}
	}
	cents := map[string]int64{}
	var total int64
	for _, m := range est.Meters {
		if m.Kind != store.UsageKindAgentModelCostMicroUSD || store.NormalizeResourceKind(m.ResourceKind) != store.ResourceKindAgentSession {
			continue
		}
		c := usdCents(m.CostUSD)
		cents[m.Tier] += c
		total += c
	}
	models := make([]AgentModelBreakdown, 0, len(byModel))
	for id, m := range byModel {
		m.CostUSD = pricing.FormatUSD(float64(cents[id]) / 100)
		models = append(models, *m)
	}
	sort.Slice(models, func(i, j int) bool {
		if ci, cj := cents[models[i].Model], cents[models[j].Model]; ci != cj {
			return ci > cj
		}
		return models[i].Model < models[j].Model
	})
	return AgentModelSummary{MonthToDateUSD: pricing.FormatUSD(float64(total) / 100), Models: models}
}

// usdCents parses a pricing.FormatUSD string ("12.34") into cents.
func usdCents(usd string) int64 {
	v, err := strconv.ParseFloat(usd, 64)
	if err != nil {
		return 0
	}
	return int64(math.Round(v * 100))
}
