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
	"slices"
	"testing"
	"time"
)

func coverageStream(source string, expected time.Time, windows map[time.Time]string) *usageCoverageStream {
	return &usageCoverageStream{Source: source, ExpectedFrom: expected, Windows: windows}
}

func TestAggregateUsageCoverageRequiresExplicitEvidence(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if got := aggregateUsageCoverage(nil, nil, month, month.Add(3*time.Hour), false); got.Known {
		t.Fatalf("legacy/no evidence became known: %+v", got)
	}

	streams := map[string]*usageCoverageStream{
		"instance": coverageStream(UsageSourceInstance, month, map[time.Time]string{
			month: UsageSourceHealthy, month.Add(time.Hour): UsageSourceHealthy, month.Add(2 * time.Hour): UsageSourceHealthy,
		}),
	}
	got := aggregateUsageCoverage(streams, []string{"instance"}, month, month.Add(3*time.Hour), false)
	if !got.Known || !got.Complete || !got.Through.Equal(month.Add(3*time.Hour)) || len(got.DegradedSources) != 0 {
		t.Fatalf("healthy contiguous evidence: %+v", got)
	}
}

func TestAggregateUsageCoverageStopsAtFirstGap(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	streams := map[string]*usageCoverageStream{
		"instance": coverageStream(UsageSourceInstance, month, map[time.Time]string{
			month: UsageSourceHealthy, month.Add(2 * time.Hour): UsageSourceHealthy,
		}),
		"build": coverageStream(UsageSourceBuild, month, map[time.Time]string{
			month: UsageSourceHealthy, month.Add(time.Hour): UsageSourceHealthy, month.Add(2 * time.Hour): UsageSourceHealthy,
		}),
	}
	got := aggregateUsageCoverage(streams, []string{"instance", "build"}, month, month.Add(3*time.Hour), false)
	if !got.Known || got.Complete || !got.Through.Equal(month.Add(time.Hour)) {
		t.Fatalf("internal gap was hidden by later MAX: %+v", got)
	}
}

func TestAggregateUsageCoverageDegradedAndAbsentStreamsArePartial(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	streams := map[string]*usageCoverageStream{
		"http": coverageStream(UsageSourceHTTP, month, map[time.Time]string{
			month: UsageSourceDegraded, month.Add(time.Hour): UsageSourceHealthy,
		}),
		"direct": coverageStream(UsageSourceDirect, month, map[time.Time]string{}),
	}
	got := aggregateUsageCoverage(streams, []string{"http", "direct"}, month, month.Add(2*time.Hour), false)
	if !got.Known || got.Complete || !got.Through.Equal(month) || !slices.Equal(got.DegradedSources, []string{UsageSourceHTTP}) {
		t.Fatalf("degraded/absent evidence: %+v", got)
	}
}

func TestAggregateUsageCoverageEndedStreamDoesNotDragWatermark(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	endedAt := month.Add(time.Hour)
	ended := coverageStream(UsageSourceBuild, month, map[time.Time]string{month: UsageSourceHealthy})
	ended.ExpectedThrough = &endedAt
	active := coverageStream(UsageSourceInstance, month, map[time.Time]string{
		month: UsageSourceHealthy, month.Add(time.Hour): UsageSourceHealthy, month.Add(2 * time.Hour): UsageSourceHealthy,
	})
	got := aggregateUsageCoverage(map[string]*usageCoverageStream{"ended": ended, "active": active}, []string{"ended", "active"}, month, month.Add(3*time.Hour), false)
	if !got.Complete || !got.Through.Equal(month.Add(3*time.Hour)) {
		t.Fatalf("completed ended stream dragged coverage: %+v", got)
	}

	ended.ExpectedThrough = ptrTime(month.Add(2 * time.Hour))
	got = aggregateUsageCoverage(map[string]*usageCoverageStream{"ended": ended, "active": active}, []string{"ended", "active"}, month, month.Add(3*time.Hour), false)
	if got.Complete || !got.Through.Equal(month.Add(time.Hour)) {
		t.Fatalf("uncertain deleted tail should constrain through to its first gap: %+v", got)
	}
}

func TestAggregateUsageCoverageEndedDegradationConstrainsThrough(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	endedAt := month.Add(2 * time.Hour)
	ended := coverageStream(UsageSourceHTTP, month, map[time.Time]string{
		month: UsageSourceHealthy, month.Add(time.Hour): UsageSourceDegraded,
	})
	ended.ExpectedThrough = &endedAt
	got := aggregateUsageCoverage(map[string]*usageCoverageStream{"ended": ended}, []string{"ended"}, month, month.Add(4*time.Hour), false)
	if got.Complete || !got.Through.Equal(month.Add(time.Hour)) || !slices.Equal(got.DegradedSources, []string{UsageSourceHTTP}) {
		t.Fatalf("ended degradation was hidden: %+v", got)
	}
}

func TestAggregateUsageCoverageSandboxAndBoundedDiagnostics(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	sources := []string{UsageSourceInstance, UsageSourceBuild, UsageSourceStorage, UsageSourceHTTP,
		UsageSourceWebSocket, UsageSourceDirect, UsageSourcePostgres, UsageSourceKeyValue}
	streams := map[string]*usageCoverageStream{}
	order := make([]string, 0, len(sources))
	for _, source := range sources {
		streams[source] = coverageStream(source, month, map[time.Time]string{month: UsageSourceDegraded})
		order = append(order, source)
	}
	got := aggregateUsageCoverage(streams, order, month, month.Add(time.Hour), true)
	if got.Complete {
		t.Fatalf("sandbox total was certified complete: %+v", got)
	}
	if len(got.DegradedSources) != 8 || got.DegradedSources[7] != "other" {
		t.Fatalf("degraded source bound/sentinel: %+v", got.DegradedSources)
	}
}

func TestUsageSourceVocabularyIsClosed(t *testing.T) {
	for _, source := range []string{UsageSourceInstance, UsageSourceBuild, UsageSourceStorage, UsageSourceHTTP,
		UsageSourceWebSocket, UsageSourceDirect, UsageSourcePostgres, UsageSourceKeyValue} {
		if !validUsageSource(source) {
			t.Errorf("expected valid source %q", source)
		}
	}
	for _, source := range []string{"", "sandbox", "promql{tenant=secret}"} {
		if validUsageSource(source) {
			t.Errorf("unexpected persisted source %q", source)
		}
	}
}

func ptrTime(value time.Time) *time.Time { return &value }

// TestMemStoreUsageIdempotent verifies the in-memory Store's UpsertUsageHourly
// is truly idempotent — a second upsert for the same (service, kind, tier,
// window) overwrites rather than duplicates.
func TestMemStoreUsageIdempotent(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	window := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	row := HourlyRow{
		WorkspaceID: "tea-001", ServiceID: "srv-001",
		Kind: UsageKindInstanceSeconds, Tier: "starter",
		WindowStart: window, Quantity: 100,
	}
	if err := st.UpsertUsageHourly(ctx, row); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	row.Quantity = 200
	if err := st.UpsertUsageHourly(ctx, row); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	rows, err := st.UsageMonthToDate(ctx, "tea-001", window.Add(time.Hour))
	if err != nil {
		t.Fatalf("UsageMonthToDate: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 aggregated row, got %d", len(rows))
	}
	if rows[0].Total != 200 {
		t.Errorf("expected quantity 200 after idempotent upsert, got %d", rows[0].Total)
	}
}

func TestMemStoreUsageOmitsZeroCoverageOnlyMeters(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	window := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	for _, row := range []HourlyRow{
		{WorkspaceID: "tea-zero", ServiceID: "srv-zero", Kind: UsageKindInstanceSeconds, Tier: "starter", WindowStart: window},
		{WorkspaceID: "tea-zero", ServiceID: "srv-zero", Kind: UsageKindEgressBytes, WindowStart: window},
		{WorkspaceID: "tea-zero", ServiceID: "srv-zero", Kind: UsageKindBuildSeconds, WindowStart: window},
	} {
		if err := st.UpsertUsageHourly(ctx, row); err != nil {
			t.Fatalf("upsert %s: %v", row.Kind, err)
		}
	}

	rows, err := st.UsageMonthToDate(ctx, "tea-zero", window.Add(time.Hour))
	if err != nil {
		t.Fatalf("UsageMonthToDate: %v", err)
	}
	if len(rows) != 1 || rows[0].Kind != UsageKindInstanceSeconds || rows[0].Total != 0 {
		t.Fatalf("zero summary rows: want only instance_seconds=0, got %+v", rows)
	}
}

// TestMemStoreMonthBoundary verifies that rows at the last second of one month
// and the first second of the next aggregate into their own months.
func TestMemStoreMonthBoundary(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	june := time.Date(2026, 6, 30, 23, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	base := HourlyRow{WorkspaceID: "tea-002", ServiceID: "srv-002", Kind: UsageKindEgressBytes, Tier: ""}
	base.WindowStart = june
	base.Quantity = 1000
	_ = st.UpsertUsageHourly(ctx, base)

	base.WindowStart = july
	base.Quantity = 2000
	_ = st.UpsertUsageHourly(ctx, base)

	// Query for July: should only see the July row.
	nowJuly := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	rows, err := st.UsageMonthToDate(ctx, "tea-002", nowJuly)
	if err != nil {
		t.Fatalf("UsageMonthToDate July: %v", err)
	}
	if len(rows) != 1 || rows[0].Total != 2000 {
		t.Errorf("July query: expected total 2000, got %+v", rows)
	}

	// Query for June: the now value must be within June for it to count.
	nowJune := time.Date(2026, 6, 30, 23, 59, 0, 0, time.UTC)
	rows, err = st.UsageMonthToDate(ctx, "tea-002", nowJune)
	if err != nil {
		t.Fatalf("UsageMonthToDate June: %v", err)
	}
	if len(rows) != 1 || rows[0].Total != 1000 {
		t.Errorf("June query: expected total 1000, got %+v", rows)
	}
}

// TestMemStoreLatestUsageWindow returns zero for an unknown service and the
// most recent window for a known one.
func TestMemStoreLatestUsageWindow(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	// Unknown service → zero time.
	got, err := st.LatestUsageWindow(ctx, ResourceKindService, "srv-unknown", UsageKindBuildSeconds)
	if err != nil {
		t.Fatalf("LatestUsageWindow (unknown): %v", err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time for unknown service, got %v", got)
	}

	// Insert two windows for the same service; latest should win.
	w1 := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	w2 := time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC)
	for _, w := range []time.Time{w1, w2} {
		_ = st.UpsertUsageHourly(ctx, HourlyRow{
			WorkspaceID: "tea-003", ServiceID: "srv-003",
			Kind: UsageKindBuildSeconds, Tier: "", WindowStart: w, Quantity: 60,
		})
	}
	// A newer row for another meter must not advance build_seconds' cursor.
	_ = st.UpsertUsageHourly(ctx, HourlyRow{
		WorkspaceID: "tea-003", ServiceID: "srv-003",
		Kind: UsageKindInstanceSeconds, Tier: "starter", WindowStart: w2.Add(time.Hour), Quantity: 3600,
	})

	got, err = st.LatestUsageWindow(ctx, ResourceKindService, "srv-003", UsageKindBuildSeconds)
	if err != nil {
		t.Fatalf("LatestUsageWindow: %v", err)
	}
	if !got.Equal(w2.UTC()) {
		t.Errorf("expected latest window %v, got %v", w2, got)
	}
}

func TestMemStoreUsageIdentityIncludesResourceKind(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	window := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	for _, row := range []HourlyRow{
		{WorkspaceID: "tea-identity", ServiceID: "shared", ResourceKind: ResourceKindPostgres, Kind: UsageKindInstanceSeconds, Tier: "free", WindowStart: window, Quantity: 3600},
		{WorkspaceID: "tea-identity", ServiceID: "shared", ResourceKind: ResourceKindKeyValue, Kind: UsageKindInstanceSeconds, Tier: "free", WindowStart: window, Quantity: 1800},
	} {
		if err := st.UpsertUsageHourly(ctx, row); err != nil {
			t.Fatalf("upsert %s: %v", row.ResourceKind, err)
		}
	}

	rows, err := st.UsageMonthToDate(ctx, "tea-identity", window.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("same-name resources: want 2 rows, got %+v", rows)
	}
	totals := map[string]int64{}
	for _, row := range rows {
		totals[row.ResourceKind] = row.Total
	}
	if totals[ResourceKindPostgres] != 3600 || totals[ResourceKindKeyValue] != 1800 {
		t.Errorf("same-name totals merged or overwritten: %+v", totals)
	}
}

// TestMemStoreCompactUsage verifies the in-memory Store's compaction contract
// (w8/m4): hourly rows older than the boundary fold into monthly aggregates,
// the compacted hourly detail is purged, rows at or after the boundary are
// untouched, month-to-date totals are identical before and after, and a
// re-run is a no-op.
func TestMemStoreCompactUsage(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	// Two April rows (same aggregate key), one May row, boundary May 1.
	april1 := time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC)
	april2 := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	may := time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC)
	for _, w := range []struct {
		start time.Time
		qty   int64
	}{{april1, 3600}, {april2, 1800}, {may, 900}} {
		_ = st.UpsertUsageHourly(ctx, HourlyRow{
			WorkspaceID: "tea-004", ServiceID: "srv-004",
			Kind: UsageKindInstanceSeconds, Tier: "starter",
			WindowStart: w.start, Quantity: w.qty,
		})
	}

	aprilEnd := time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC)
	preApril, err := st.UsageMonthToDate(ctx, "tea-004", aprilEnd)
	if err != nil {
		t.Fatalf("pre-compaction April query: %v", err)
	}

	boundary := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	res, err := st.CompactUsage(ctx, boundary)
	if err != nil {
		t.Fatalf("CompactUsage: %v", err)
	}
	if res.HourlyRows != 2 || res.Months != 1 {
		t.Errorf("compaction result: want 2 hourly rows / 1 month, got %+v", res)
	}
	if len(st.usage) != 1 {
		t.Errorf("hourly rows after compaction: want 1 (May), got %d", len(st.usage))
	}

	// The April total must be identical, now served from the monthly table.
	postApril, err := st.UsageMonthToDate(ctx, "tea-004", aprilEnd)
	if err != nil {
		t.Fatalf("post-compaction April query: %v", err)
	}
	if len(preApril) != 1 || len(postApril) != 1 || preApril[0] != postApril[0] {
		t.Errorf("April totals changed across compaction: before %+v, after %+v", preApril, postApril)
	}
	if postApril[0].Total != 5400 {
		t.Errorf("April total: want 5400, got %d", postApril[0].Total)
	}

	// The hot May row is untouched and still queryable.
	mayRows, err := st.UsageMonthToDate(ctx, "tea-004", may.Add(time.Hour))
	if err != nil {
		t.Fatalf("May query: %v", err)
	}
	if len(mayRows) != 1 || mayRows[0].Total != 900 {
		t.Errorf("May query: want total 900, got %+v", mayRows)
	}

	// Idempotency: nothing left older than the boundary → 0/0, totals stable.
	res, err = st.CompactUsage(ctx, boundary)
	if err != nil {
		t.Fatalf("CompactUsage re-run: %v", err)
	}
	if res.HourlyRows != 0 || res.Months != 0 {
		t.Errorf("re-run: want 0/0, got %+v", res)
	}
	again, _ := st.UsageMonthToDate(ctx, "tea-004", aprilEnd)
	if len(again) != 1 || again[0].Total != 5400 {
		t.Errorf("re-run drifted April totals: %+v", again)
	}
}

func TestMemStoreCompactStorageUsage(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	month := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for i, quantity := range []int64{3600, 7200} {
		_ = st.UpsertUsageHourly(ctx, HourlyRow{
			WorkspaceID: "tea-storage", ServiceID: "db", ResourceKind: ResourceKindPostgres,
			Kind: UsageKindStorageGBSeconds, WindowStart: month.Add(time.Duration(i) * time.Hour), Quantity: quantity,
		})
	}
	if _, err := st.CompactUsage(ctx, month.AddDate(0, 1, 0)); err != nil {
		t.Fatal(err)
	}
	rows, err := st.UsageMonthToDate(ctx, "tea-storage", month.AddDate(0, 1, 0).Add(-time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Kind != UsageKindStorageGBSeconds || rows[0].Total != 10800 {
		t.Fatalf("compacted storage total: want 10800, got %+v", rows)
	}
}

// TestMemStoreCompactUsageStraggler verifies the additive upsert: an hourly
// row that lands in an already-compacted month (in principle impossible under
// the 48 h clamp, but the contract is defensive) adds to the aggregate on the
// next pass instead of overwriting it.
func TestMemStoreCompactUsageStraggler(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	row := HourlyRow{
		WorkspaceID: "tea-005", ServiceID: "srv-005",
		Kind: UsageKindEgressBytes, Tier: "",
		WindowStart: time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC), Quantity: 1000,
	}
	_ = st.UpsertUsageHourly(ctx, row)

	boundary := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if _, err := st.CompactUsage(ctx, boundary); err != nil {
		t.Fatalf("first compaction: %v", err)
	}

	// Straggler for the same, already-compacted month.
	row.WindowStart = time.Date(2026, 4, 11, 8, 0, 0, 0, time.UTC)
	row.Quantity = 24
	_ = st.UpsertUsageHourly(ctx, row)
	if _, err := st.CompactUsage(ctx, boundary); err != nil {
		t.Fatalf("second compaction: %v", err)
	}

	rows, err := st.UsageMonthToDate(ctx, "tea-005", time.Date(2026, 4, 30, 23, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("April query: %v", err)
	}
	if len(rows) != 1 || rows[0].Total != 1024 {
		t.Errorf("straggler must add to the aggregate: want 1024, got %+v", rows)
	}
}

// TestMemStoreWorkspaceIsolation verifies that month-to-date totals for one
// workspace never include rows from another.
func TestMemStoreWorkspaceIsolation(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()

	window := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	_ = st.UpsertUsageHourly(ctx, HourlyRow{
		WorkspaceID: "tea-A", ServiceID: "srv-A",
		Kind: UsageKindInstanceSeconds, Tier: "free",
		WindowStart: window, Quantity: 500,
	})
	_ = st.UpsertUsageHourly(ctx, HourlyRow{
		WorkspaceID: "tea-B", ServiceID: "srv-B",
		Kind: UsageKindInstanceSeconds, Tier: "free",
		WindowStart: window, Quantity: 999,
	})

	now := window.Add(time.Hour)
	rows, err := st.UsageMonthToDate(ctx, "tea-A", now)
	if err != nil {
		t.Fatalf("UsageMonthToDate: %v", err)
	}
	if len(rows) != 1 || rows[0].Total != 500 {
		t.Errorf("workspace isolation: expected total 500 for tea-A, got %+v", rows)
	}
}
