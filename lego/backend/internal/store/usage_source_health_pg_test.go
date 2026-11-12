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
	"maps"
	"os"
	"slices"
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
	for _, source := range UsageStreamSources() {
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

// w4/575: a disk deleted from a live service writes no more disk rows; closing
// its idle stream at the first hour it was gone keeps the month's coverage
// healthy once that hour is past the catch-up window, and a stream that is
// still metered stays open.
func TestCloseIdleUsageSourceStreamsKeepsCoverageHealthy(t *testing.T) {
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
	tenant, err := st.CreateTenant(ctx, "usage-idle-disk", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	for _, service := range []string{"srv-gone", "srv-kept"} {
		if err := st.UpsertUsageHourly(ctx, HourlyRow{
			WorkspaceID: tenant.ID, ResourceKind: ResourceKindService, ServiceID: service,
			Kind: UsageKindDiskGBSeconds, WindowStart: month, Quantity: 3600,
			SourceHealth: []UsageSourceObservation{{Source: UsageSourceDisk, State: UsageSourceHealthy, ExpectedFrom: month}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	gone := month.Add(time.Hour)
	if err := st.CloseIdleUsageSourceStreams(ctx, UsageKindDiskGBSeconds, UsageSourceDisk, []string{"srv-kept"}, gone); err != nil {
		t.Fatal(err)
	}
	var through *time.Time
	if err := pool.QueryRow(ctx, `SELECT expected_through FROM usage_source_streams WHERE service_id = 'srv-gone' AND workspace_id = $1`, tenant.ID).Scan(&through); err != nil || through == nil || !through.Equal(gone) {
		t.Fatalf("the deleted disk's stream ends %v (err %v), want %s", through, err, gone)
	}
	if err := pool.QueryRow(ctx, `SELECT expected_through FROM usage_source_streams WHERE service_id = 'srv-kept' AND workspace_id = $1`, tenant.ID).Scan(&through); err != nil || through != nil {
		t.Fatalf("the metered disk's stream ends %v (err %v), want it open", through, err)
	}
	// Only srv-gone's coverage is in question: drop the open stream, then read
	// the month three days on.
	if _, err := pool.Exec(ctx, `DELETE FROM usage_source_streams WHERE service_id = 'srv-kept' AND workspace_id = $1`, tenant.ID); err != nil {
		t.Fatal(err)
	}
	coverage, err := st.CurrentUsageCoverage(ctx, tenant.ID, month.Add(72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(coverage.DegradedSources, UsageSourceDisk) {
		t.Fatalf("coverage %+v names disk degraded after its disk was deleted", coverage)
	}

	// A close found later (the meter's catch-up) still ends the stream an
	// hour after its last row, not at the hour being metered.
	if err := st.UpsertUsageHourly(ctx, HourlyRow{
		WorkspaceID: tenant.ID, ResourceKind: ResourceKindService, ServiceID: "srv-late",
		Kind: UsageKindDiskGBSeconds, WindowStart: month, Quantity: 3600,
		SourceHealth: []UsageSourceObservation{{Source: UsageSourceDisk, State: UsageSourceHealthy, ExpectedFrom: month}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CloseIdleUsageSourceStreams(ctx, UsageKindDiskGBSeconds, UsageSourceDisk, nil, month.Add(10*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT expected_through FROM usage_source_streams WHERE service_id = 'srv-late' AND workspace_id = $1`, tenant.ID).Scan(&through); err != nil || through == nil || !through.Equal(gone) {
		t.Fatalf("a late close ends the stream %v (err %v), want an hour after its last row, %s", through, err, gone)
	}

	// The disk is re-added five hours on: the stream expects rows from then,
	// not across the hours it had none.
	readded := month.Add(5 * time.Hour)
	if err := st.UpsertUsageHourly(ctx, HourlyRow{
		WorkspaceID: tenant.ID, ResourceKind: ResourceKindService, ServiceID: "srv-late",
		Kind: UsageKindDiskGBSeconds, WindowStart: readded, Quantity: 3600,
		SourceHealth: []UsageSourceObservation{{Source: UsageSourceDisk, State: UsageSourceHealthy, ExpectedFrom: readded}},
	}); err != nil {
		t.Fatal(err)
	}
	var from time.Time
	if err := pool.QueryRow(ctx, `SELECT expected_from, expected_through FROM usage_source_streams WHERE service_id = 'srv-late' AND workspace_id = $1`, tenant.ID).Scan(&from, &through); err != nil || !from.Equal(readded) || through != nil {
		t.Fatalf("re-added disk stream = from %s through %v (err %v), want open from %s", from, through, err, readded)
	}
}

// w6/m334: the fleet-wide read reports, per source, the oldest hour inside the
// catch-up window with no metered reading: a missing or unavailable hour
// counts, a degraded one was metered, and an hour already past the window is
// not reported (it can no longer be metered). Dated 2001 so streams other
// tests left in the shared database are outside the window.
func TestUnmeteredUsageHolesPG(t *testing.T) {
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
	tenant, err := st.CreateTenant(ctx, "usage-unmetered", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })

	now := time.Date(2001, 1, 10, 12, 30, 0, 0, time.UTC)
	settled := usageSettledBefore(now.Truncate(time.Hour))
	record := func(service, source string, from, window time.Time, state string) {
		t.Helper()
		if err := st.RecordUsageSourceHealth(ctx, []UsageSourceRecord{{WorkspaceID: tenant.ID, ResourceKind: ResourceKindService, ServiceID: service,
			Kind: UsageKindEgressBytes, WindowStart: window, UsageSourceObservation: UsageSourceObservation{Source: source, State: state, ExpectedFrom: from}}}); err != nil {
			t.Fatal(err)
		}
	}
	// http: metered from the window's start, but 30 h ago is unavailable.
	hole := now.Truncate(time.Hour).Add(-30 * time.Hour)
	for w := settled; w.Before(now.Truncate(time.Hour)); w = w.Add(time.Hour) {
		state := UsageSourceHealthy
		if w.Equal(hole) {
			state = UsageSourceUnavailable
		}
		record("srv-unmetered-a-"+tenant.ID, UsageSourceHTTP, settled, w, state)
	}
	// websocket: every hour metered, one of them degraded.
	for w := settled; w.Before(now.Truncate(time.Hour)); w = w.Add(time.Hour) {
		state := UsageSourceHealthy
		if w.Equal(hole) {
			state = UsageSourceDegraded
		}
		record("srv-unmetered-a-"+tenant.ID, UsageSourceWebSocket, settled, w, state)
	}
	// direct: a stream expected since before the window with its last reading
	// 10 h ago, so its first unmetered in-window hour is 9 h ago.
	last := now.Truncate(time.Hour).Add(-10 * time.Hour)
	for w := settled.Add(-5 * time.Hour); !w.After(last); w = w.Add(time.Hour) {
		record("srv-unmetered-b-"+tenant.ID, UsageSourceDirect, settled.Add(-5*time.Hour), w, UsageSourceHealthy)
	}

	oldest := func(at time.Time) map[string]time.Time {
		t.Helper()
		holes, err := st.UnmeteredUsageHoles(ctx, at)
		if err != nil {
			t.Fatalf("UnmeteredUsageHoles: %v", err)
		}
		out := map[string]time.Time{}
		for _, h := range holes {
			if current, seen := out[h.Source]; !seen || h.Window.Before(current) {
				out[h.Source] = h.Window
			}
		}
		return out
	}
	got := oldest(now)
	want := map[string]time.Time{UsageSourceHTTP: hole, UsageSourceDirect: last.Add(time.Hour)}
	if !maps.EqualFunc(got, want, time.Time.Equal) {
		t.Fatalf("oldest unmetered hours = %v, want %v (websocket's degraded hour was metered)", got, want)
	}

	// A deleted App whose name a new App took over owes nothing past its own
	// last reading: closing there ends direct's hole (w6/m334).
	if err := st.CloseUsageSourceStreamsAtLastReading(ctx, UsageResourceRef{ResourceKind: ResourceKindService, ServiceID: "srv-unmetered-b-" + tenant.ID}); err != nil {
		t.Fatalf("CloseUsageSourceStreamsAtLastReading: %v", err)
	}
	var through *time.Time
	if err := pool.QueryRow(ctx, `SELECT expected_through FROM usage_source_streams WHERE service_id = $1`, "srv-unmetered-b-"+tenant.ID).Scan(&through); err != nil || through == nil || !through.Equal(last.Add(time.Hour)) {
		t.Fatalf("taken-over stream ends %v (%v), want an hour after its last reading %v", through, err, last)
	}
	if got = oldest(now); !got[UsageSourceDirect].IsZero() {
		t.Fatalf("after the close, direct's oldest unmetered hour = %v, want none", got[UsageSourceDirect])
	}

	// Once the hole is past the window it is skipped for good, not reported.
	later := now.Add(20 * time.Hour)
	if h, ok := oldest(later)[UsageSourceHTTP]; ok && !h.After(hole) {
		t.Fatalf("http hole %v reported at %v, past the catch-up window", h, later)
	}
}
