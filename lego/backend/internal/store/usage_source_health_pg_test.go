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

func TestUsageSourceHealthTransactionsAndHealthyZero(t *testing.T) {
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
	tenant, err := st.CreateTenant(ctx, "usage-health-atomic", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	base := UsageSourceRecord{
		WorkspaceID: tenant.ID, ResourceKind: ResourceKindService, ServiceID: "srv-health",
		Kind: UsageKindEgressBytes, WindowStart: month,
		UsageSourceObservation: UsageSourceObservation{Source: UsageSourceHTTP, State: UsageSourceUnavailable, ExpectedFrom: month},
	}
	bad := base
	bad.Source = "tenant-secret-source"
	if err := st.RecordUsageSourceHealth(ctx, []UsageSourceRecord{base, bad}); err == nil {
		t.Fatal("invalid second observation unexpectedly committed")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM usage_source_streams WHERE workspace_id = $1`, tenant.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial source batch survived rollback: count=%d err=%v", count, err)
	}

	row := HourlyRow{
		WorkspaceID: tenant.ID, ResourceKind: ResourceKindService, ServiceID: "srv-health",
		Kind: UsageKindEgressBytes, WindowStart: month, Quantity: 0,
		SourceHealth: []UsageSourceObservation{
			{Source: UsageSourceHTTP, State: UsageSourceHealthy, ExpectedFrom: month},
			{Source: "invalid", State: UsageSourceHealthy, ExpectedFrom: month},
		},
	}
	if err := st.UpsertUsageHourly(ctx, row); err == nil {
		t.Fatal("invalid atomic usage+health row unexpectedly committed")
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM usage_hourly WHERE workspace_id = $1`, tenant.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("quantity survived health rollback: count=%d err=%v", count, err)
	}

	row.SourceHealth = row.SourceHealth[:1]
	if err := st.UpsertUsageHourly(ctx, row); err != nil {
		t.Fatal(err)
	}
	row.WindowStart = month.Add(time.Hour)
	if err := st.UpsertUsageHourly(ctx, row); err != nil {
		t.Fatal(err)
	}
	coverage, err := st.CurrentUsageCoverage(ctx, tenant.ID, month.Add(2*time.Hour+30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !coverage.Known || !coverage.Complete || !coverage.Through.Equal(month.Add(2*time.Hour)) {
		t.Fatalf("explicit healthy zero was not complete: %+v", coverage)
	}
}

// Every source a meter stores must pass the source CHECKs, or that meter's
// upserts fail on every window. The disk meter's 'disk' failed that way from
// ADR082 until migration 0167 (w5/m111's live replay found it in bex-api's
// logs). UsageSourceSandbox is response-only and never stored.
func TestUsageSourceChecksAcceptEveryStoredSource(t *testing.T) {
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
	tenant, err := st.CreateTenant(ctx, "usage-source-vocabulary", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })
	window := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	for _, source := range []string{
		UsageSourceInstance, UsageSourceBuild, UsageSourceStorage, UsageSourceHTTP, UsageSourceWebSocket,
		UsageSourceDirect, UsageSourcePostgres, UsageSourceKeyValue, UsageSourceDisk,
	} {
		row := HourlyRow{
			WorkspaceID: tenant.ID, ResourceKind: ResourceKindService, ServiceID: "srv-vocab-" + source,
			Kind: UsageKindDiskGBSeconds, WindowStart: window, Quantity: 1,
			SourceHealth: []UsageSourceObservation{{Source: source, State: UsageSourceHealthy}},
		}
		if err := st.UpsertUsageHourly(ctx, row); err != nil {
			t.Errorf("source %q: %v", source, err)
		}
	}
}
