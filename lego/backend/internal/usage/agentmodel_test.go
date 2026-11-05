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
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func TestSummariseAgentModelGroupsByModelAndTokenKind(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{ServiceID: "ags-1", ResourceKind: store.ResourceKindAgentSession, Kind: store.UsageKindAgentModelCostMicroUSD, Tier: "m/small", Total: 4_999},
		{ServiceID: "ags-2", ResourceKind: store.ResourceKindAgentSession, Kind: store.UsageKindAgentModelCostMicroUSD, Tier: "m/small", Total: 5_002},
		// Sub-cent: the estimate has no line for it, so it reads 0.00.
		{ServiceID: "ags-2", ResourceKind: store.ResourceKindAgentSession, Kind: store.UsageKindAgentModelCostMicroUSD, Tier: "m/tiny", Total: 3_000},
		{ServiceID: "ags-1", ResourceKind: store.ResourceKindAgentSession, Kind: store.UsageKindAgentModelInputTokens, Tier: "m/small", Total: 100},
		{ServiceID: "ags-2", ResourceKind: store.ResourceKindAgentSession, Kind: store.UsageKindAgentModelCacheReadTokens, Tier: "m/small", Total: 7},
		{ServiceID: "ags-3", ResourceKind: store.ResourceKindAgentSession, Kind: store.UsageKindAgentModelCostMicroUSD, Tier: "m/big", Total: 12_340_000},
		{ServiceID: "ags-3", ResourceKind: store.ResourceKindAgentSession, Kind: store.UsageKindAgentModelOutputTokens, Tier: "m/big", Total: 9},
		{ServiceID: "ags-3", ResourceKind: store.ResourceKindAgentSession, Kind: store.UsageKindAgentModelCacheWriteTokens, Tier: "m/big", Total: 3},
		// Not agent model usage: ignored.
		{ServiceID: "srv-1", ResourceKind: store.ResourceKindService, Kind: store.UsageKindInstanceSeconds, Tier: "starter", Total: 3600},
	}
	got := summariseAgentModel(rows, pricing.Default.Estimate(rows))
	if got.MonthToDateUSD != "12.35" {
		t.Fatalf("monthToDateUsd = %q, want 12.35 (12.34 + 0.01, the estimate's model lines)", got.MonthToDateUSD)
	}
	if len(got.Models) != 3 || got.Models[0].Model != "m/big" || got.Models[1].Model != "m/small" || got.Models[2].Model != "m/tiny" || got.Models[2].CostUSD != "0.00" {
		t.Fatalf("models = %+v, want m/big, m/small, then m/tiny at 0.00", got.Models)
	}
	big, small := got.Models[0], got.Models[1]
	if big.CostUSD != "12.34" || big.OutputTokens != 9 || big.CacheWriteTokens != 3 || big.InputTokens != 0 {
		t.Fatalf("m/big = %+v", big)
	}
	if small.CostUSD != "0.01" || small.InputTokens != 100 || small.CacheReadTokens != 7 {
		t.Fatalf("m/small = %+v", small)
	}
	if empty := summariseAgentModel(nil, pricing.EstimatedCost{}); empty.MonthToDateUSD != "0.00" || empty.Models == nil {
		t.Fatalf("empty agentModel = %+v, want 0.00 and []", empty)
	}
	got.Models = got.Models[:2]
	body, err := json.Marshal(toUsageResponse(Summary{AgentModel: got}).AgentModel)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"monthToDateUsd":"12.35","models":[{"model":"m/big","costUsd":"12.34","inputTokens":0,"outputTokens":9,"cacheReadTokens":0,"cacheWriteTokens":3},{"model":"m/small","costUsd":"0.01","inputTokens":100,"outputTokens":0,"cacheReadTokens":7,"cacheWriteTokens":0}]}`; string(body) != want {
		t.Fatalf("agentModel JSON = %s, want %s", body, want)
	}
}

// ADR091 D6 acceptance: the usage read's Agent model line equals the sum of
// the recorded requests' customer prices.
func TestAgentModelLineMatchesTheUsageEvents(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st := store.NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "usage-agent-model", store.PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	at := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
	events := []store.AgentModelUsage{
		{SessionID: "ags-a", Model: "m/a", At: at, InputTokens: 10, OutputTokens: 2, PriceMicroUSD: 1_234_567},
		{SessionID: "ags-a", Model: "m/a", At: at.Add(3 * time.Hour), CacheReadTokens: 50, PriceMicroUSD: 3},
		{SessionID: "ags-b", Model: "m/b", At: at.Add(26 * time.Hour), CacheWriteTokens: 4, PriceMicroUSD: 2_500_000},
		{SessionID: "ags-b", Model: "m/b", At: at.Add(27 * time.Hour), InputTokens: 1, CostSource: store.AgentModelCostUnpriced},
	}
	for i, e := range events {
		e.RequestID = tenant.ID + "-" + string(rune('a'+i))
		e.WorkspaceID = tenant.ID
		if e.CostSource == "" {
			e.CostSource = store.AgentModelCostReported
		}
		if ok, err := st.RecordAgentModelUsage(ctx, e); err != nil || !ok {
			t.Fatalf("record %d: %v %v", i, ok, err)
		}
	}
	// Compaction folds some hours into usage_monthly; the line is unchanged.
	if _, err := st.CompactUsage(ctx, at.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	rows, err := st.UsageMonthToDate(ctx, tenant.ID, at.AddDate(0, 0, 20))
	if err != nil {
		t.Fatal(err)
	}
	var eventsMicro int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(SUM(price_microusd), 0) FROM agent_model_usage_events WHERE workspace_id = $1`, tenant.ID).Scan(&eventsMicro); err != nil {
		t.Fatal(err)
	}
	got := summariseAgentModel(rows, pricing.Default.Estimate(rows))
	if want := pricing.FormatUSD(pricing.MicroToUSD(eventsMicro)); got.MonthToDateUSD != want || want != "3.73" {
		t.Fatalf("agent model line = %s, events sum to %s (%d µUSD), want 3.73", got.MonthToDateUSD, want, eventsMicro)
	}
	if len(got.Models) != 2 || got.Models[0].Model != "m/b" || got.Models[0].CostUSD != "2.50" || got.Models[0].InputTokens != 1 || got.Models[0].CacheWriteTokens != 4 ||
		got.Models[1].CostUSD != "1.23" || got.Models[1].InputTokens != 10 || got.Models[1].OutputTokens != 2 || got.Models[1].CacheReadTokens != 50 {
		t.Fatalf("breakdown = %+v", got.Models)
	}
}
