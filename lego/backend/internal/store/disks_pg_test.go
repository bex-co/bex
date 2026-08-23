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
	"errors"
	"testing"
	"time"
)

// The disk row is simultaneously deploy intent and the billing record, so these
// run against real Postgres: the one-live-disk index, the grow-only guard, and
// the GB-second integration are all SQL-level guarantees the fake cannot prove.

func newDiskTestStore(t *testing.T) (*PGStore, context.Context) {
	t.Helper()
	return newBillingTestStore(t)
}

func seedDiskApp(t *testing.T, ctx context.Context, s *PGStore) (tenantID, appID string) {
	t.Helper()
	tenant, err := s.CreateTenant(ctx, "disks-"+time.Now().Format("150405.000000"), "pro")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	app, err := s.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "nginx:1", Branch: "main", Port: 3000, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	return tenant.ID, app.ID
}

func TestPGDiskLifecycle(t *testing.T) {
	s, ctx := newDiskTestStore(t)
	tenantID, appID := seedDiskApp(t, ctx, s)

	disk, err := s.CreateDisk(ctx, tenantID, appID, "data", "/var/data", 10)
	if err != nil {
		t.Fatalf("CreateDisk: %v", err)
	}

	// One live disk per service, enforced by the partial unique index — a racing
	// second create loses in the database, not only in the API's check.
	if _, err := s.CreateDisk(ctx, tenantID, appID, "second", "/var/more", 10); !errors.Is(err, ErrConflict) {
		t.Fatalf("second CreateDisk = %v, want ErrConflict", err)
	}

	if _, err := s.UpdateDisk(ctx, disk.ID, nil, nil, ptrInt32(5)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("shrink = %v, want ErrInvalid", err)
	}
	grown, err := s.UpdateDisk(ctx, disk.ID, nil, nil, ptrInt32(25))
	if err != nil {
		t.Fatalf("grow: %v", err)
	}
	if grown.SizeGB != 25 {
		t.Fatalf("size after grow = %d, want 25", grown.SizeGB)
	}

	if err := s.DeleteDisk(ctx, disk.ID); err != nil {
		t.Fatalf("DeleteDisk: %v", err)
	}
	if _, err := s.GetDisk(ctx, disk.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetDisk after delete = %v, want ErrNotFound", err)
	}
	// Deleting frees the service to take a new disk.
	if _, err := s.CreateDisk(ctx, tenantID, appID, "replacement", "/var/data", 10); err != nil {
		t.Fatalf("CreateDisk after delete: %v", err)
	}
}

// The meter's whole contract: provisioned size integrated over time, with a
// grow contributing both sizes within the window it lands in.
func TestPGDiskUsageIntegratesSizePeriods(t *testing.T) {
	s, ctx := newDiskTestStore(t)
	tenantID, appID := seedDiskApp(t, ctx, s)

	disk, err := s.CreateDisk(ctx, tenantID, appID, "data", "/var/data", 10)
	if err != nil {
		t.Fatalf("CreateDisk: %v", err)
	}
	// Rewrite the lifecycle onto a fixed hour so the arithmetic is exact rather
	// than dependent on wall-clock timing.
	windowStart := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(time.Hour)
	if _, err := s.Pool.Exec(ctx, `UPDATE service_disks SET created_at = $2 WHERE id = $1`, disk.ID, windowStart); err != nil {
		t.Fatalf("backdate disk: %v", err)
	}
	if _, err := s.Pool.Exec(ctx, `DELETE FROM service_disk_sizes WHERE disk_id = $1`, disk.ID); err != nil {
		t.Fatalf("reset periods: %v", err)
	}
	// 10GB for the first half hour, 40GB for the second.
	mid := windowStart.Add(30 * time.Minute)
	if _, err := s.Pool.Exec(ctx,
		`INSERT INTO service_disk_sizes (disk_id, size_gb, from_ts, to_ts) VALUES ($1, 10, $2, $3), ($1, 40, $3, NULL)`,
		disk.ID, windowStart, mid); err != nil {
		t.Fatalf("seed periods: %v", err)
	}

	rows, err := s.DiskUsageForWindow(ctx, windowStart, windowEnd)
	if err != nil {
		t.Fatalf("DiskUsageForWindow: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one per service with a disk", rows)
	}
	// 10GB × 1800s + 40GB × 1800s = 90000 GB-seconds.
	if rows[0].GBSeconds != 90000 {
		t.Fatalf("GB-seconds = %d, want 90000 (10GB for 30m then 40GB for 30m)", rows[0].GBSeconds)
	}
	if rows[0].AppID != appID || rows[0].TenantID != tenantID {
		t.Fatalf("row = %+v, want it attributed to the owning service and workspace", rows[0])
	}

	// A window entirely before the disk existed owes nothing.
	empty, err := s.DiskUsageForWindow(ctx, windowStart.Add(-2*time.Hour), windowStart.Add(-time.Hour))
	if err != nil {
		t.Fatalf("DiskUsageForWindow (before): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("pre-creation window billed %+v", empty)
	}
}

// Deleting closes the open period, so billing stops at the deletion instant
// rather than running to the end of the window.
func TestPGDiskUsageStopsAtDeletion(t *testing.T) {
	s, ctx := newDiskTestStore(t)
	tenantID, appID := seedDiskApp(t, ctx, s)

	disk, err := s.CreateDisk(ctx, tenantID, appID, "data", "/var/data", 10)
	if err != nil {
		t.Fatalf("CreateDisk: %v", err)
	}
	windowStart := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	deletedAt := windowStart.Add(15 * time.Minute)
	if _, err := s.Pool.Exec(ctx,
		`UPDATE service_disk_sizes SET from_ts = $2, to_ts = $3 WHERE disk_id = $1`,
		disk.ID, windowStart, deletedAt); err != nil {
		t.Fatalf("close period: %v", err)
	}

	rows, err := s.DiskUsageForWindow(ctx, windowStart, windowStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("DiskUsageForWindow: %v", err)
	}
	if len(rows) != 1 || rows[0].GBSeconds != 9000 {
		t.Fatalf("rows = %+v, want 10GB × 900s = 9000 GB-seconds", rows)
	}
}

func ptrInt32(v int32) *int32 { return &v }
