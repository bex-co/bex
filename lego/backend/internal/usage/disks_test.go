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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func diskMeterService(st *memUsageStore, promBase string) *Service {
	return &Service{Base: &core.Base{}, Store: st, PromBase: promBase}
}

func hourWindow(offset int) time.Time {
	return time.Now().UTC().Truncate(time.Hour).Add(time.Duration(offset) * time.Hour)
}

// The meter's defining property: it reads control-plane rows, so it keeps
// billing provisioned volumes while Prometheus (and the whole app cluster) is
// unreachable. Every other meter is structurally off in that state.
func TestDiskMeterRunsWithoutPrometheus(t *testing.T) {
	window := hourWindow(-1)
	st := &memUsageStore{
		rows:     map[usageKey]store.HourlyRow{},
		latestBy: map[string]time.Time{},
		diskRows: map[time.Time][]store.DiskUsageRow{
			window: {{TenantID: "tea-1", AppID: "srv-1", GBSeconds: 36000}},
		},
	}
	svc := diskMeterService(st, "") // no Prometheus

	svc.catchUpDisksThrough(context.Background(), window)

	row, ok := st.rows[usageKey{
		resourceKind: store.ResourceKindService, serviceID: "srv-1",
		kind: store.UsageKindDiskGBSeconds, windowStart: window,
	}]
	if !ok {
		t.Fatalf("no disk row written for %s; rows=%+v", window, st.rows)
	}
	if row.Quantity != 36000 {
		t.Fatalf("quantity = %d, want the 36000 GB-seconds the store reported", row.Quantity)
	}
	if row.Tier != "" {
		t.Fatalf("tier = %q, want empty — disks price per GB regardless of plan", row.Tier)
	}
	if row.WorkspaceID != "tea-1" {
		t.Fatalf("workspace = %q, want the owning workspace", row.WorkspaceID)
	}
}

// A restart must backfill every window it missed, oldest first, rather than
// jumping to the newest and leaving an unbilled hole.
func TestDiskMeterBackfillsMissedWindows(t *testing.T) {
	first, second, third := hourWindow(-3), hourWindow(-2), hourWindow(-1)
	st := &memUsageStore{
		rows:     map[usageKey]store.HourlyRow{},
		latestBy: map[string]time.Time{},
		diskRows: map[time.Time][]store.DiskUsageRow{
			first:  {{TenantID: "tea-1", AppID: "srv-1", GBSeconds: 100}},
			second: {{TenantID: "tea-1", AppID: "srv-1", GBSeconds: 200}},
			third:  {{TenantID: "tea-1", AppID: "srv-1", GBSeconds: 300}},
		},
	}
	svc := diskMeterService(st, "")

	// Seed the cursor at `first`, as though the process died after that window.
	svc.catchUpDisksThrough(context.Background(), first)
	svc.catchUpDisksThrough(context.Background(), third)

	for _, tc := range []struct {
		window time.Time
		want   int64
	}{{first, 100}, {second, 200}, {third, 300}} {
		row, ok := st.rows[usageKey{
			resourceKind: store.ResourceKindService, serviceID: "srv-1",
			kind: store.UsageKindDiskGBSeconds, windowStart: tc.window,
		}]
		if !ok || row.Quantity != tc.want {
			t.Fatalf("window %s = %+v (present=%v), want %d GB-seconds", tc.window, row, ok, tc.want)
		}
	}
}

// Re-running a window must not double-bill: the arithmetic is over committed
// rows, so the same window always yields the same quantity.
func TestDiskMeterIsIdempotentPerWindow(t *testing.T) {
	window := hourWindow(-1)
	writes := 0
	st := &memUsageStore{
		rows:     map[usageKey]store.HourlyRow{},
		latestBy: map[string]time.Time{},
		diskRows: map[time.Time][]store.DiskUsageRow{
			window: {{TenantID: "tea-1", AppID: "srv-1", GBSeconds: 36000}},
		},
	}
	st.upsert = func(row store.HourlyRow) error {
		if row.Kind == store.UsageKindDiskGBSeconds {
			writes++
		}
		return nil
	}
	svc := diskMeterService(st, "")

	svc.catchUpDisksThrough(context.Background(), window)
	first := writes
	svc.catchUpDisksThrough(context.Background(), window)

	if writes != first {
		t.Fatalf("re-running the same window wrote again (%d then %d); the cursor should have skipped it", first, writes)
	}
}

// A service with no disk owes nothing — an empty result must not write a zero
// row, which would claim evidence of usage that does not exist.
func TestDiskMeterWritesNothingWithoutDisks(t *testing.T) {
	window := hourWindow(-1)
	st := &memUsageStore{
		rows:     map[usageKey]store.HourlyRow{},
		latestBy: map[string]time.Time{},
		diskRows: map[time.Time][]store.DiskUsageRow{},
	}
	svc := diskMeterService(st, "")

	svc.catchUpDisksThrough(context.Background(), window)

	if len(st.rows) != 0 {
		t.Fatalf("rows = %+v, want none for a workspace with no disks", st.rows)
	}
}
