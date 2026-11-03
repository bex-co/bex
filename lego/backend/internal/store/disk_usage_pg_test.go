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

package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestDiskUsageCrossingsPG pins w2/m255's storage on real Postgres: a crossing
// flips the disk's state once (a racing replica writes nothing), lands in the
// service's event feed with its Render details attached, and the generic fact
// writer refuses a disk-usage fact without its details row.
func TestDiskUsageCrossingsPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	ws, err := st.CreateWorkspace(ctx, "disk-usage-"+stamp, store.PlanHobby, "disk-owner-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	app, err := st.CreateApp(ctx, store.App{TenantID: ws.ID, Name: "db-" + stamp, Image: "postgres:16", Branch: "main", Port: 5432, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	disk, err := st.CreateDisk(ctx, ws.ID, app.ID, "data", "/data", 10)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	high := true
	if err := st.SetDiskUsagePending(ctx, disk.ID, &high, now); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDiskUsagePending(ctx, disk.ID, &high, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	state, err := st.GetDiskUsageState(ctx, disk.ID)
	if err != nil || state.PendingSince == nil || !state.PendingSince.Equal(now) {
		t.Fatalf("pending = %+v err=%v, want the first sighting kept", state, err)
	}

	fact := store.ServiceEventFact{
		SourceKey: "disk_usage:" + disk.ID + ":high:" + stamp, AppID: app.ID,
		Type: store.EventFactServiceDiskUsageHigh, At: now,
		DiskUsage: &store.DiskUsageDetails{DiskID: disk.ID, DiskName: "data", MountPath: "/data", UsagePercent: 85.5, ThresholdPercent: 80},
	}
	if ok, err := st.RecordDiskUsageCrossing(ctx, fact); err != nil || !ok {
		t.Fatalf("record: ok=%v err=%v", ok, err)
	}
	raced := fact
	raced.SourceKey += "-replica"
	if ok, err := st.RecordDiskUsageCrossing(ctx, raced); err != nil || ok {
		t.Fatalf("a second high crossing must write nothing: ok=%v err=%v", ok, err)
	}
	state, _ = st.GetDiskUsageState(ctx, disk.ID)
	if !state.High || state.PendingHigh != nil {
		t.Errorf("state after crossing = %+v", state)
	}

	rows, err := st.ListServiceEvents(ctx, app.ID, ws.ID, store.ServiceEventFilter{
		FactTypes: []string{string(store.EventFactServiceDiskUsageHigh)}, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].DiskUsage == nil || rows[0].DiskUsage.UsagePercent != 85.5 || rows[0].DiskUsage.DiskID != disk.ID {
		t.Fatalf("feed rows = %+v", rows)
	}

	bare := fact
	bare.SourceKey += "-bare"
	if _, err := st.InsertServiceEventFact(ctx, bare); !errors.Is(err, store.ErrInvalid) {
		t.Errorf("generic insert of a disk-usage fact: err=%v, want ErrInvalid", err)
	}
	bare.DiskUsage = nil
	if _, err := st.InsertServiceEventFact(ctx, bare); err == nil {
		t.Error("a disk-usage fact without details must be refused")
	}
}
