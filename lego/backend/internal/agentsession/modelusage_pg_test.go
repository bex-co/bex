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

package agentsession

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// ADR091 acceptance: a platform-route exchange reporting $0.001 is priced at
// the 1.30 markup — price_microusd 1300, cost_source reported — and meters
// 1300 µUSD into agent_model_cost_microusd; a replay adds nothing.
func TestPGModelUsageRecordsReportedCostAtMarkup(t *testing.T) {
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
	defer pool.Close()
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, "model-cost-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pg.DeleteTenant(ctx, tenant.ID) }()
	session, err := pg.CreateAgentSession(ctx, store.AgentSession{
		WorkspaceID: tenant.ID, Repo: "octo/repo", Branch: "bex-agent/task-1",
		AgentConfig: []byte(`{"agent":"claude","task":"x"}`), InitialPrompt: "x",
	})
	if err != nil {
		t.Fatal(err)
	}

	at := time.Now().UTC()
	rec := &ModelUsageRecorder{Store: pg, Now: func() time.Time { return at }, Markup: 1.3}
	cost := 0.001
	rep := ModelUsageReport{
		RequestID: id.New(id.AgentModelRequest), SessionID: session.ID, Workspace: tenant.ID,
		Model: DefaultDevModel, InputTokens: 100, OutputTokens: 20, UpstreamCostUSD: &cost,
	}
	for attempt, wantDuplicate := range []bool{false, true} {
		out, err := rec.Record(ctx, rep)
		if err != nil || !out.Acknowledged || out.Duplicate != wantDuplicate {
			t.Fatalf("attempt %d: %+v, %v (want duplicate=%v)", attempt, out, err, wantDuplicate)
		}
	}

	var upstream, price int64
	var source string
	if err := pool.QueryRow(ctx, `SELECT upstream_cost_microusd, cost_source, price_microusd
		FROM agent_model_usage_events WHERE request_id = $1`, rep.RequestID).Scan(&upstream, &source, &price); err != nil {
		t.Fatal(err)
	}
	if upstream != 1000 || source != store.AgentModelCostReported || price != 1300 {
		t.Fatalf("event = upstream %d / %s / price %d, want 1000 / reported / 1300", upstream, source, price)
	}
	var metered int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(SUM(quantity), 0) FROM usage_hourly
		WHERE workspace_id = $1 AND kind = $2 AND resource_kind = $3`,
		tenant.ID, store.UsageKindAgentModelCostMicroUSD, store.ResourceKindAgentSession).Scan(&metered); err != nil {
		t.Fatal(err)
	}
	if metered != 1300 {
		t.Fatalf("agent_model_cost_microusd = %d, want 1300 once (replay adds nothing)", metered)
	}
}
