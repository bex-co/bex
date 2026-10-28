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

package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// Platform-route token usage (w10/m14 t003) is recorded once per request id
// into usage_hourly, read back by the generic month-to-date verb and the
// quota total, and its ledger is purged by compaction.
func TestAgentModelUsageIsIdempotentAndMetered(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "agent-model-usage", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	at := time.Date(2026, 10, 27, 9, 40, 0, 0, time.UTC)
	const model = "deepseek/deepseek-v4.1-flash-1017"
	first := AgentModelUsage{
		RequestID: "amr-" + tenant.ID, WorkspaceID: tenant.ID, SessionID: "ags-usage", Model: model, At: at,
		InputTokens: 100, OutputTokens: 20, CacheReadTokens: 300,
	}
	for attempt, want := range []bool{true, false} {
		recorded, err := st.RecordAgentModelUsage(ctx, first)
		if err != nil || recorded != want {
			t.Fatalf("attempt %d: recorded=%v err=%v, want %v", attempt, recorded, err, want)
		}
	}
	second := first
	second.RequestID += "-2"
	second.At = at.Add(10 * time.Minute)
	second.InputTokens, second.OutputTokens, second.CacheReadTokens, second.CacheWriteTokens = 1, 2, 0, 4
	if recorded, err := st.RecordAgentModelUsage(ctx, second); err != nil || !recorded {
		t.Fatalf("second request: recorded=%v err=%v", recorded, err)
	}

	rows, err := st.UsageMonthToDate(ctx, tenant.ID, at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, r := range rows {
		if r.ResourceKind != ResourceKindAgentSession || r.ServiceID != "ags-usage" || r.Tier != model {
			t.Fatalf("unexpected row %+v", r)
		}
		got[r.Kind] = r.Total
	}
	want := map[string]int64{
		UsageKindAgentModelInputTokens: 101, UsageKindAgentModelOutputTokens: 22,
		UsageKindAgentModelCacheReadTokens: 300, UsageKindAgentModelCacheWriteTokens: 4,
	}
	for kind, total := range want {
		if got[kind] != total {
			t.Errorf("%s = %d, want %d (rows %+v)", kind, got[kind], total, rows)
		}
	}
	total, err := st.AgentModelTokensMonthToDate(ctx, tenant.ID, at)
	if err != nil || total != 427 {
		t.Fatalf("month-to-date tokens = %d, %v, want 427", total, err)
	}
	if next, err := st.AgentModelTokensMonthToDate(ctx, tenant.ID, at.AddDate(0, 1, 0)); err != nil || next != 0 {
		t.Fatalf("next month's tokens = %d, %v, want 0", next, err)
	}

	if _, err := st.CompactUsage(ctx, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	var ledger int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_model_usage_events WHERE workspace_id = $1`, tenant.ID).Scan(&ledger); err != nil || ledger != 0 {
		t.Fatalf("ledger rows after compaction = %d, %v", ledger, err)
	}
	if after, err := st.AgentModelTokensMonthToDate(ctx, tenant.ID, at); err != nil || after != 427 {
		t.Fatalf("month-to-date after compaction = %d, %v, want 427 from usage_monthly", after, err)
	}
}
