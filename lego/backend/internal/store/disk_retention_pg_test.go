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
	"reflect"
	"strings"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPGDeletedServiceRetainsRequestedDiskBilling(t *testing.T) {
	for _, rawDelete := range []bool{false, true} {
		name := "DeleteApp"
		if rawDelete {
			name = "raw DELETE"
		}
		t.Run(name, func(t *testing.T) {
			s, ctx := newDiskTestStore(t)
			tenantID, appID := seedDiskApp(t, ctx, s)
			disk, err := s.CreateDisk(ctx, tenantID, appID, "data", "/var/data", 1)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.UpdateDisk(ctx, disk.ID, nil, nil, ptrInt32(5)); err != nil {
				t.Fatal(err)
			}
			window := time.Now().UTC().Truncate(time.Hour).Add(-2 * time.Hour)
			mid := window.Add(30 * time.Minute)
			if _, err := s.Pool.Exec(ctx, `UPDATE service_disk_sizes
				SET from_ts = CASE size_gb WHEN 1 THEN $2::timestamptz ELSE $3::timestamptz END,
				    to_ts = CASE size_gb WHEN 1 THEN $3::timestamptz ELSE NULL END
				WHERE disk_id=$1`, disk.ID, window, mid); err != nil {
				t.Fatal(err)
			}
			before, err := s.DiskUsageForWindow(ctx, window, window.Add(time.Hour))
			if err != nil || len(before) != 1 || before[0].GBSeconds != 10800 || before[0].AppID != appID {
				t.Fatalf("1→5 requested-size window = %+v, err=%v; want 10800 GB-seconds", before, err)
			}
			other, err := s.CreateApp(ctx, App{TenantID: tenantID, Name: "other", Image: "nginx:1", Tier: "starter", InitialDisk: &appv1alpha1.DiskSpec{Name: "other", MountPath: "/var/data", SizeGB: 1}})
			if err != nil {
				t.Fatal(err)
			}
			if rawDelete {
				_, err = s.Pool.Exec(ctx, `DELETE FROM apps WHERE id=$1`, appID)
			} else {
				err = s.DeleteApp(ctx, appID)
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err := s.DiskUsageForWindow(ctx, window, window.Add(time.Hour))
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("service deletion changed historical billing: before=%+v after=%+v err=%v", before, after, err)
			}
			for label, readErr := range map[string]error{
				"service": func() error { _, err := s.GetApp(ctx, appID); return err }(),
				"disk":    func() error { _, err := s.GetDisk(ctx, disk.ID); return err }(),
				"grow":    func() error { _, err := s.UpdateDisk(ctx, disk.ID, nil, nil, ptrInt32(6)); return err }(),
			} {
				if !errors.Is(readErr, ErrNotFound) {
					t.Fatalf("deleted %s = %v, want ErrNotFound", label, readErr)
				}
			}
			live, err := s.ListDisks(ctx, tenantID, "")
			if err != nil || len(live) != 1 || live[0].AppID != other.ID || live[0].SizeGB != 1 {
				t.Fatalf("live disk inventory/quota = %+v, err=%v; want only untouched sibling", live, err)
			}
			var stopped, lastPeriod time.Time
			var periods, open int
			if err := s.Pool.QueryRow(ctx, `SELECT d.deleted_at, max(p.to_ts), count(*), count(*) FILTER (WHERE p.to_ts IS NULL)
				FROM service_disks d JOIN service_disk_sizes p ON p.disk_id=d.id
				WHERE d.id=$1 GROUP BY d.deleted_at`, disk.ID).Scan(&stopped, &lastPeriod, &periods, &open); err != nil {
				t.Fatal(err)
			}
			if periods != 2 || open != 0 || !lastPeriod.Equal(stopped) {
				t.Fatalf("retained periods=%d open=%d end=%v deleted=%v", periods, open, lastPeriod, stopped)
			}
			future, err := s.DiskUsageForWindow(ctx, stopped.Add(time.Second), stopped.Add(time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range future {
				if row.AppID == appID {
					t.Fatalf("deleted service still billed in a future window: %+v", row)
				}
			}
			// A stale failed-create cleanup cannot erase a normally deleted service's history.
			if err := s.RollbackAppCreation(ctx, appID); err != nil {
				t.Fatal(err)
			}
			retained, err := s.DiskUsageForWindow(ctx, window, window.Add(time.Hour))
			if err != nil || !reflect.DeepEqual(after, retained) {
				t.Fatalf("stale rollback removed billing: %+v, err=%v", retained, err)
			}
			recreated, err := s.CreateApp(ctx, App{TenantID: tenantID, Name: "web", Image: "nginx:1", Tier: "starter", InitialDisk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1}})
			if err != nil || recreated.ID == appID {
				t.Fatalf("same-name recreation = %+v, err=%v", recreated, err)
			}
			newDisks, err := s.ListDisks(ctx, tenantID, recreated.ID)
			if err != nil || len(newDisks) != 1 || newDisks[0].ID == disk.ID || newDisks[0].SizeGB != 1 {
				t.Fatalf("replacement inherited deleted disk: %+v, err=%v", newDisks, err)
			}
			// Downgrade must refuse rather than cascade away retained financial history.
			down, err := migrationsFS.ReadFile("migrations/0175_disk_billing_retention.down.sql")
			if err != nil {
				t.Fatal(err)
			}
			tx, err := s.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(ctx, string(down))
			_ = tx.Rollback(ctx)
			if err == nil || !strings.Contains(err.Error(), "retained disk billing history") {
				t.Fatalf("downgrade = %v, want explicit financial-history refusal", err)
			}
		})
	}
}

func TestPGDiskRetentionParentAndCreationRollback(t *testing.T) {
	s, ctx := newDiskTestStore(t)
	tenantID, appID := seedDiskApp(t, ctx, s)
	if _, err := s.CreateDisk(ctx, tenantID, ids.New(ids.Service), "missing", "/var/data", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disk with absent parent = %v", err)
	}
	other, err := s.CreateTenant(ctx, "other", "pro")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateDisk(ctx, other.ID, appID, "foreign", "/var/data", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disk with wrong parent workspace = %v", err)
	}
	app, err := s.CreateApp(ctx, App{TenantID: tenantID, Name: "provisional", Image: "nginx:1", Tier: "starter", InitialDisk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RollbackAppCreation(ctx, app.ID); err != nil {
		t.Fatal(err)
	}
	var disks, periods int
	if err := s.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM service_disks), (SELECT count(*) FROM service_disk_sizes)`).Scan(&disks, &periods); err != nil || disks != 0 || periods != 0 {
		t.Fatalf("failed create left billable rows: disks=%d periods=%d err=%v", disks, periods, err)
	}
	if _, err := s.CreateApp(ctx, App{TenantID: tenantID, Name: "invalid-disk", Image: "nginx:1", Tier: "starter", InitialDisk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "relative", SizeGB: 1}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid initial disk = %v", err)
	}
	apps, err := s.ListApps(ctx)
	if err != nil || len(apps) != 1 || apps[0].ID != appID {
		t.Fatalf("creation rollbacks changed other service: %+v err=%v", apps, err)
	}
	disk, err := s.CreateDisk(ctx, tenantID, appID, "data", "/var/data", 1)
	if err != nil {
		t.Fatal(err)
	}
	// Freeze a deleting transaction's clock before a committed grow. The close
	// must use the time after its disk locks, not that old transaction start.
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT now()`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateDisk(ctx, disk.ID, nil, nil, ptrInt32(5)); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM apps WHERE id=$1`, appID); err != nil {
		t.Fatalf("delete begun before grow failed to close its period: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE service_disks SET deleted_at=NULL WHERE id=$1`, disk.ID); err == nil {
		t.Fatal("revived disk without a live parent")
	}
	if err := s.DeleteTenant(ctx, tenantID); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM service_disks WHERE tenant_id=$1`, tenantID).Scan(&disks); err != nil || disks != 0 {
		t.Fatalf("workspace FK no longer cleans disk history: disks=%d err=%v", disks, err)
	}
}

func TestPGDiskAttachmentRacesServiceDeletion(t *testing.T) {
	for _, createFirst := range []bool{false, true} {
		name := "delete first"
		if createFirst {
			name = "create first"
		}
		t.Run(name, func(t *testing.T) {
			s, _ := newDiskTestStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tenantID, appID := seedDiskApp(t, ctx, s)
			config := s.Pool.Config()
			config.MaxConns, config.MinConns = 1, 0
			writer, err := pgxpool.NewWithConfig(ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Close()
			var writerPID, holderPID int32
			if err := writer.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&writerPID); err != nil {
				t.Fatal(err)
			}
			tx, err := s.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			if createFirst {
				_, err = createDiskTx(ctx, tx, tenantID, appID, "data", "/var/data", 1)
			} else {
				_, err = tx.Exec(ctx, `DELETE FROM apps WHERE id=$1`, appID)
			}
			if err != nil {
				t.Fatal(err)
			}
			finished := make(chan error, 1)
			go func() {
				other := NewPGStore(writer)
				if createFirst {
					finished <- other.DeleteApp(ctx, appID)
				} else {
					_, err := other.CreateDisk(ctx, tenantID, appID, "data", "/var/data", 1)
					finished <- err
				}
			}()
			for {
				var blocked bool
				if err := s.Pool.QueryRow(ctx, `SELECT $2 = ANY(pg_blocking_pids($1))`, writerPID, holderPID).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				select {
				case err := <-finished:
					t.Fatalf("operation escaped parent lock: %v", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(10 * time.Millisecond):
				}
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			err = <-finished
			if (createFirst && err != nil) || (!createFirst && !errors.Is(err, ErrNotFound)) {
				t.Fatalf("racing operation = %v (create first %v)", err, createFirst)
			}
			var active, open int
			if err := s.Pool.QueryRow(ctx, `SELECT
				(SELECT count(*) FROM service_disks WHERE deleted_at IS NULL),
				(SELECT count(*) FROM service_disk_sizes WHERE to_ts IS NULL)`).Scan(&active, &open); err != nil || active != 0 || open != 0 {
				t.Fatalf("racing deletion stranded allocations: active=%d open=%d err=%v", active, open, err)
			}
		})
	}
}

func TestPGDiskAttachmentRacesWorkspaceDeletion(t *testing.T) {
	s, _ := newDiskTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tenantID, appID := seedDiskApp(t, ctx, s)
	disk, err := s.CreateDisk(ctx, tenantID, appID, "old", "/var/data", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDisk(ctx, disk.ID); err != nil {
		t.Fatal(err)
	}
	config := s.Pool.Config()
	config.MaxConns, config.MinConns = 1, 0
	writer, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	var writerPID, holderPID int32
	if err := writer.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&writerPID); err != nil {
		t.Fatal(err)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	var lockedID string
	if err := tx.QueryRow(ctx, `SELECT id FROM tenants WHERE id=$1 FOR UPDATE`, tenantID).Scan(&lockedID); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		_, err := NewPGStore(writer).CreateDisk(ctx, tenantID, appID, "new", "/var/data", 1)
		finished <- err
	}()
	for {
		var blocked bool
		if err := s.Pool.QueryRow(ctx, `SELECT $2 = ANY(pg_blocking_pids($1))`, writerPID, holderPID).Scan(&blocked); err != nil {
			t.Fatal(err)
		}
		if blocked {
			break
		}
		select {
		case err := <-finished:
			t.Fatalf("attachment escaped workspace lock: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	// A waiting attachment must not hold the App ahead of the workspace.
	if err := tx.QueryRow(ctx, `SELECT id FROM apps WHERE id=$1 FOR UPDATE NOWAIT`, appID).Scan(&lockedID); err != nil {
		t.Fatalf("attachment inverted workspace -> App lock order: %v", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tenants WHERE id=$1`, tenantID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-finished; !errors.Is(err, ErrNotFound) {
		t.Fatalf("attachment after workspace deletion = %v, want ErrNotFound (23503)", err)
	}
	var apps, disks, periods int
	if err := s.Pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM apps WHERE tenant_id=$1),
		(SELECT count(*) FROM service_disks WHERE tenant_id=$1),
		(SELECT count(*) FROM service_disk_sizes)`, tenantID).Scan(&apps, &disks, &periods); err != nil || apps != 0 || disks != 0 || periods != 0 {
		t.Fatalf("workspace deletion left rows: apps=%d disks=%d periods=%d err=%v", apps, disks, periods, err)
	}
}
