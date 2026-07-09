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
	"testing"
	"time"
)

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
	got, err := st.LatestUsageWindow(ctx, "srv-unknown")
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

	got, err = st.LatestUsageWindow(ctx, "srv-003")
	if err != nil {
		t.Fatalf("LatestUsageWindow: %v", err)
	}
	if !got.Equal(w2.UTC()) {
		t.Errorf("expected latest window %v, got %v", w2, got)
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
