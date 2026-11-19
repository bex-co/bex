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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w5/194 against real Postgres, as production met it on 2026-11-17 06:00Z: a
// cut-off pass committed one disk's row of a window and not the other's, and
// the next pass metered the window after it whole. The store reports the
// missing row as the disk source's hole, and the next hourly pass fills it.
func TestDiskMeterFillsAHoleBehindANewerWindowPG(t *testing.T) {
	st, pool, ctx := newCronMinimumStore(t)
	tenant, err := st.CreateTenant(ctx, "disk-hole", "pro")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	var apps []string
	for _, disk := range []struct {
		name string
		gb   int32
	}{{"big", 15}, {"small", 10}} {
		app, err := st.CreateApp(ctx, store.App{TenantID: tenant.ID, Name: disk.name, Image: "nginx:1", Branch: "main", Port: 3000, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatalf("CreateApp: %v", err)
		}
		created, err := st.CreateDisk(ctx, tenant.ID, app.ID, "data", "/var/data", disk.gb)
		if err != nil {
			t.Fatalf("CreateDisk: %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE service_disk_sizes SET from_ts = $2 WHERE disk_id = $1`, created.ID, hourWindow(-6)); err != nil {
			t.Fatalf("backdate disk: %v", err)
		}
		apps = append(apps, app.ID)
	}
	big, small := apps[0], apps[1]
	before, hole, after, next := hourWindow(-4), hourWindow(-3), hourWindow(-2), hourWindow(-1)
	svc := &Service{Base: &core.Base{}, Store: st}

	svc.catchUpDisksThrough(ctx, before)
	// The cut-off pass: the big disk's row alone, with no idle close.
	if err := st.UpsertUsageHourly(ctx, store.HourlyRow{
		WorkspaceID: tenant.ID, ResourceKind: store.ResourceKindService, ServiceID: big,
		Kind: store.UsageKindDiskGBSeconds, WindowStart: hole, Quantity: 15 * 3600,
		SourceHealth: []store.UsageSourceObservation{{Source: store.UsageSourceDisk, State: store.UsageSourceHealthy}},
	}); err != nil {
		t.Fatalf("UpsertUsageHourly: %v", err)
	}
	if err := svc.meterDiskWindow(ctx, after); err != nil {
		t.Fatalf("meterDiskWindow: %v", err)
	}
	if got := diskHoles(t, st, after); len(got) != 1 || got[0].ServiceID != small || !got[0].Window.Equal(hole) {
		t.Fatalf("disk holes = %+v, want the small disk's at %s", got, hole)
	}

	svc.catchUpDisksThrough(ctx, next)

	quantities := map[string]map[time.Time]int64{}
	rows, err := pool.Query(ctx, `SELECT service_id, window_start, quantity FROM usage_hourly WHERE kind = $1 AND window_start >= $2`,
		store.UsageKindDiskGBSeconds, before)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var service string
		var window time.Time
		var quantity int64
		if err := rows.Scan(&service, &window, &quantity); err != nil {
			t.Fatal(err)
		}
		if quantities[service] == nil {
			quantities[service] = map[time.Time]int64{}
		}
		quantities[service][window.UTC()] = quantity
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for service, gb := range map[string]int64{big: 15, small: 10} {
		for _, window := range []time.Time{before, hole, after, next} {
			if got := quantities[service][window]; got != gb*3600 {
				t.Fatalf("%s at %s = %d GB-seconds, want %d", service, window, got, gb*3600)
			}
		}
	}
	if got := diskHoles(t, st, next); len(got) != 0 {
		t.Fatalf("disk holes after the pass = %+v, want none", got)
	}
}

// diskHoles is the disk source's holes through window, as the unmetered-hour
// gauge reads them.
func diskHoles(t *testing.T, st *store.PGStore, window time.Time) []store.UsageHole {
	t.Helper()
	holes, err := st.UnmeteredUsageHoles(t.Context(), window.Add(time.Hour))
	if err != nil {
		t.Fatalf("UnmeteredUsageHoles: %v", err)
	}
	var disk []store.UsageHole
	for _, hole := range holes {
		if hole.Source == store.UsageSourceDisk {
			disk = append(disk, hole)
		}
	}
	return disk
}
