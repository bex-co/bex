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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func TestObjectStorageMetersMigrationRoundTrip(t *testing.T) {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	read := func(name string) string {
		raw, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	hasColumn := func() bool {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_name = 'object_storage_usage' AND column_name = 'gb_remainder')`).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	// accepts reports whether usage_hourly takes a row of kind, in a savepoint
	// so a refused insert does not end the transaction.
	accepts := func(kind string) bool {
		t.Helper()
		sp, err := tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = sp.Rollback(ctx) }()
		var ws string
		if err := sp.QueryRow(ctx, `INSERT INTO tenants (id, name, plan) VALUES ('tea-mig' || substr(md5(random()::text), 1, 8), 'mig' || md5(random()::text), 'hobby') RETURNING id`).Scan(&ws); err != nil {
			t.Fatal(err)
		}
		_, err = sp.Exec(ctx, `INSERT INTO usage_hourly (workspace_id, service_id, kind, resource_kind, window_start, quantity)
			VALUES ($1, $1, $2, 'object_storage', '2026-01-01', 1)`, ws, kind)
		return err == nil
	}
	exec := func(sql string) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	kinds := []string{"object_storage_gb_seconds", "object_storage_ingress_bytes", "object_storage_egress_bytes"}

	for _, k := range kinds {
		if !accepts(k) {
			t.Fatalf("0274 left usage_hourly refusing %s", k)
		}
	}
	if accepts("object_storage_cost_microusd") {
		t.Fatal("usage_hourly accepts a kind no migration added")
	}
	if !hasColumn() {
		t.Fatal("0274 left object_storage_usage without gb_remainder")
	}

	exec(read("0274_object_storage_meters.down.sql"))
	for _, k := range kinds {
		if accepts(k) {
			t.Fatalf("the down migration left usage_hourly accepting %s", k)
		}
	}
	if !accepts("sandbox_snapshot_gb_seconds") {
		t.Fatal("the down migration narrowed the kind list below 0227's")
	}
	if hasColumn() {
		t.Fatal("the down migration left gb_remainder behind")
	}

	exec(read("0274_object_storage_meters.up.sql"))
	for _, k := range kinds {
		if !accepts(k) {
			t.Fatalf("the up migration did not restore %s after a down", k)
		}
	}
	if !hasColumn() {
		t.Fatal("the up migration did not restore gb_remainder")
	}
}

func objectUsageKindTotal(t *testing.T, s *PGStore, ws, kind string) (total int64, rows int) {
	t.Helper()
	err := s.Pool.QueryRow(context.Background(), `
		SELECT COALESCE(sum(quantity), 0)::bigint, count(*)::int FROM usage_hourly
		WHERE workspace_id = $1 AND kind = $2 AND resource_kind = 'object_storage' AND service_id = $1 AND tier = ''`,
		ws, kind).Scan(&total, &rows)
	if err != nil {
		t.Fatal(err)
	}
	return total, rows
}

func objectUsageByHour(t *testing.T, s *PGStore, ws, kind string) map[string]int64 {
	t.Helper()
	rows, err := s.Pool.Query(context.Background(),
		`SELECT window_start, quantity FROM usage_hourly WHERE workspace_id = $1 AND kind = $2 AND resource_kind = 'object_storage'`, ws, kind)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int64{}
	for rows.Next() {
		var at time.Time
		var q int64
		if err := rows.Scan(&at, &q); err != nil {
			t.Fatal(err)
		}
		got[at.UTC().Format("15:04")] = q
	}
	return got
}

func TestObjectStorageObservationAccruesGBSecondsAcrossHours(t *testing.T) {
	s, ws := objectBucketStore(t, "objmeter-accrue")
	ctx := context.Background()
	a, b := newObjectBucket(t, s, ws, "bucket-a", 10), newObjectBucket(t, s, ws, "bucket-b", 10)
	const gb = int64(1_000_000_000)
	t0 := time.Date(2026, 11, 10, 10, 30, 0, 0, time.UTC)
	observe := func(at time.Time, rows ...ObjectStorageUsage) {
		t.Helper()
		for i := range rows {
			rows[i].WorkspaceID, rows[i].Region = ws, "oregon"
		}
		if err := s.RecordObjectStorageObservation(ctx, ws, "oregon", rows, at); err != nil {
			t.Fatal(err)
		}
	}

	// The first observation of a bucket accrues nothing: no time is invented
	// before it.
	observe(t0, ObjectStorageUsage{BucketID: a.ID, Bytes: 10 * gb, Objects: 1}, ObjectStorageUsage{BucketID: b.ID})
	if total, rows := objectUsageKindTotal(t, s, ws, UsageKindObjectStorageGBSeconds); total != 0 || rows != 0 {
		t.Fatalf("the first observation accrued %d GB-seconds in %d rows, want none", total, rows)
	}

	// Two hours at 10 GB, split at the UTC hour boundaries: 30 min, 1 h, 30 min.
	observe(t0.Add(2*time.Hour), ObjectStorageUsage{BucketID: a.ID, Bytes: 10 * gb, Objects: 1}, ObjectStorageUsage{BucketID: b.ID})
	want := map[string]int64{"10:00": 10 * 1800, "11:00": 10 * 3600, "12:00": 10 * 1800}
	if got := objectUsageByHour(t, s, ws, UsageKindObjectStorageGBSeconds); !equalHours(got, want) {
		t.Fatalf("after two hours at 10 GB: %v, want %v", got, want)
	}

	// The bucket grows to 20 GB over the next hour: the interval accrues at the
	// mean of the two observations, 15 GB, half of it in each hour it crosses.
	observe(t0.Add(3*time.Hour), ObjectStorageUsage{BucketID: a.ID, Bytes: 20 * gb, Objects: 2}, ObjectStorageUsage{BucketID: b.ID})
	want = map[string]int64{"10:00": 10 * 1800, "11:00": 10 * 3600, "12:00": 10*1800 + 15*1800, "13:00": 15 * 1800}
	if got := objectUsageByHour(t, s, ws, UsageKindObjectStorageGBSeconds); !equalHours(got, want) {
		t.Fatalf("after growing to 20 GB: %v, want %v", got, want)
	}

	// The row the sweep reads is the latest observation.
	if rows, err := s.ListObjectStorageUsage(ctx, ws); err != nil || len(rows) != 2 {
		t.Fatalf("usage rows = %+v, %v; want one per bucket", rows, err)
	}
	if bytes, objects, _ := s.ObjectStorageUsageTotals(ctx, ws, "oregon"); bytes != 20*gb || objects != 2 {
		t.Fatalf("totals = %d bytes, %d objects, want 20 GB, 2", bytes, objects)
	}
}

func equalHours(got, want map[string]int64) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// The sub-unit byte-seconds of one sweep carry into the next, so an hourly
// cadence cannot round a small workspace's storage down to nothing.
func TestObjectStorageObservationCarriesTheRemainderBetweenSweeps(t *testing.T) {
	s, ws := objectBucketStore(t, "objmeter-remainder")
	ctx := context.Background()
	bucket := newObjectBucket(t, s, ws, "bucket-a", 10)
	t0 := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	// 1.5 GB for one second is 1.5 GB-seconds; observed twice, one second apart,
	// the right total is 3, where rounding each sweep down on its own gives 2.
	for i := range 3 {
		err := s.RecordObjectStorageObservation(ctx, ws, "oregon",
			[]ObjectStorageUsage{{BucketID: bucket.ID, Bytes: 1_500_000_000, Objects: 1}}, t0.Add(time.Duration(i)*time.Second))
		if err != nil {
			t.Fatal(err)
		}
	}
	if total, _ := objectUsageKindTotal(t, s, ws, UsageKindObjectStorageGBSeconds); total != 3 {
		t.Fatalf("1.5 GB for 2 seconds = %d GB-seconds, want 3", total)
	}
}

func TestObjectStorageObservationDropsWhatItNoLongerSeesAndAccruesItsLastInterval(t *testing.T) {
	s, ws := objectBucketStore(t, "objmeter-vanish")
	ctx := context.Background()
	bucket := newObjectBucket(t, s, ws, "bucket-a", 10)
	orphan := "orphan:" + ws + ":oregon"
	const gb = int64(1_000_000_000)
	t0 := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	observe := func(at time.Time, rows ...ObjectStorageUsage) {
		t.Helper()
		for i := range rows {
			rows[i].WorkspaceID, rows[i].Region = ws, "oregon"
		}
		if err := s.RecordObjectStorageObservation(ctx, ws, "oregon", rows, at); err != nil {
			t.Fatal(err)
		}
	}
	observe(t0, ObjectStorageUsage{BucketID: bucket.ID}, ObjectStorageUsage{BucketID: orphan, Bytes: 4 * gb, Objects: 1})
	// The orphan is gone an hour later: it was there for half of the hour on
	// average (4 GB, then 0), and its row goes.
	observe(t0.Add(time.Hour), ObjectStorageUsage{BucketID: bucket.ID})
	if got := objectUsageByHour(t, s, ws, UsageKindObjectStorageGBSeconds); !equalHours(got, map[string]int64{"10:00": 2 * 3600}) {
		t.Fatalf("the vanished row accrued %v, want 2 GB x 3600 s in the 10:00 hour", got)
	}
	rows, _ := s.ListObjectStorageUsage(ctx, ws)
	if len(rows) != 1 || rows[0].BucketID != bucket.ID {
		t.Fatalf("rows after the orphan vanished = %+v, want only the bucket", rows)
	}
	if bytes, _, _ := s.ObjectStorageUsageTotals(ctx, ws, "oregon"); bytes != 0 {
		t.Fatalf("a vanished row still counts towards the quota: %d bytes", bytes)
	}
}

func TestObjectStorageObservationNeverAccruesBackwards(t *testing.T) {
	s, ws := objectBucketStore(t, "objmeter-clock")
	ctx := context.Background()
	bucket := newObjectBucket(t, s, ws, "bucket-a", 10)
	t0 := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	observe := func(at time.Time) {
		t.Helper()
		err := s.RecordObjectStorageObservation(ctx, ws, "oregon",
			[]ObjectStorageUsage{{BucketID: bucket.ID, WorkspaceID: ws, Region: "oregon", Bytes: 5_000_000_000, Objects: 1}}, at)
		if err != nil {
			t.Fatal(err)
		}
	}
	observe(t0)
	observe(t0.Add(time.Hour))
	before, _ := objectUsageKindTotal(t, s, ws, UsageKindObjectStorageGBSeconds)
	observe(t0) // a replica whose clock is an hour behind
	observe(t0.Add(time.Hour))
	after, _ := objectUsageKindTotal(t, s, ws, UsageKindObjectStorageGBSeconds)
	if before != 5*3600 || after != before {
		t.Fatalf("GB-seconds %d then %d after a stale observation, want %d both times", before, after, 5*3600)
	}
}

func TestObjectStorageObservationOfAGoneWorkspaceRecordsUsageRowsButNoMeter(t *testing.T) {
	s, _ := objectBucketStore(t, "objmeter-gone")
	ctx := context.Background()
	ghost := "tea-ghost" + uniqueMachineRun()
	t.Cleanup(func() { _, _ = s.Pool.Exec(ctx, `DELETE FROM object_storage_usage WHERE workspace_id=$1`, ghost) })
	t0 := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	for _, at := range []time.Time{t0, t0.Add(time.Hour)} {
		err := s.RecordObjectStorageObservation(ctx, ghost, "oregon",
			[]ObjectStorageUsage{{BucketID: "obk-ghostbucket" + ghost, WorkspaceID: ghost, Region: "oregon", Bytes: 9_000_000_000, Objects: 1}}, at)
		if err != nil {
			t.Fatalf("an observation of a deleted workspace failed on the usage_hourly foreign key: %v", err)
		}
	}
	if total, rows := objectUsageKindTotal(t, s, ghost, UsageKindObjectStorageGBSeconds); total != 0 || rows != 0 {
		t.Fatalf("a workspace with no tenant row accrued %d in %d rows", total, rows)
	}
}

func TestAddObjectStorageUsageSumsPerHourAndSkipsGoneWorkspaces(t *testing.T) {
	s, ws := objectBucketStore(t, "objmeter-add")
	ctx := context.Background()
	hour := time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)
	ghost := "tea-ghost" + uniqueMachineRun()
	err := s.AddObjectStorageUsage(ctx, []ObjectStorageUsageDelta{
		{WorkspaceID: ws, Kind: UsageKindObjectStorageIngressBytes, Window: hour.Add(17 * time.Minute), Quantity: 100},
		{WorkspaceID: ws, Kind: UsageKindObjectStorageIngressBytes, Window: hour, Quantity: 23},
		{WorkspaceID: ws, Kind: UsageKindObjectStorageEgressBytes, Window: hour, Quantity: 7},
		{WorkspaceID: ws, Kind: UsageKindObjectStorageEgressBytes, Window: hour.Add(time.Hour), Quantity: 0},
		{WorkspaceID: ghost, Kind: UsageKindObjectStorageEgressBytes, Window: hour, Quantity: 5},
	})
	if err != nil {
		t.Fatalf("a delta for a deleted workspace failed the batch: %v", err)
	}
	if total, rows := objectUsageKindTotal(t, s, ws, UsageKindObjectStorageIngressBytes); total != 123 || rows != 1 {
		t.Fatalf("ingress = %d in %d rows, want 123 in one hour row", total, rows)
	}
	if total, rows := objectUsageKindTotal(t, s, ws, UsageKindObjectStorageEgressBytes); total != 7 || rows != 1 {
		t.Fatalf("egress = %d in %d rows, want 7 in one row (a zero delta writes none)", total, rows)
	}
	if total, _ := objectUsageKindTotal(t, s, ghost, UsageKindObjectStorageEgressBytes); total != 0 {
		t.Fatalf("a deleted workspace received %d bytes of usage", total)
	}
	// Adding again adds.
	if err := s.AddObjectStorageUsage(ctx, []ObjectStorageUsageDelta{{WorkspaceID: ws, Kind: UsageKindObjectStorageEgressBytes, Window: hour, Quantity: 3}}); err != nil {
		t.Fatal(err)
	}
	if total, _ := objectUsageKindTotal(t, s, ws, UsageKindObjectStorageEgressBytes); total != 10 {
		t.Fatalf("egress after a second add = %d, want 10", total)
	}
	if err := s.AddObjectStorageUsage(ctx, []ObjectStorageUsageDelta{{WorkspaceID: ws, Kind: UsageKindEgressBytes, Window: hour, Quantity: 1}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a kind that is not an object storage meter = %v, want ErrInvalid (it would be billed as Render bandwidth)", err)
	}
}

func TestDeleteTenantRetiresObjectBucketsAndFinishPurgeDropsThem(t *testing.T) {
	s, ws := objectBucketStore(t, "objpurge-retire")
	_, other := objectBucketStore(t, "objpurge-other")
	ctx := context.Background()
	b := newObjectBucket(t, s, ws, "photos", 10)
	sys, err := s.EnsureSystemObjectBucket(ctx, ws, "render-oregon", "oregon")
	if err != nil {
		t.Fatal(err)
	}
	keep := newObjectBucket(t, s, other, "photos", 10)
	at := time.Now().UTC()
	for _, u := range []ObjectStorageUsage{
		{BucketID: b.ID, WorkspaceID: ws, Region: "oregon", Bytes: 5, Objects: 1, ObservedAt: at},
		{BucketID: "orphan:" + ws + ":oregon", WorkspaceID: ws, Region: "oregon", Bytes: 6, Objects: 1, ObservedAt: at},
		{BucketID: keep.ID, WorkspaceID: other, Region: "oregon", Bytes: 7, Objects: 1, ObservedAt: at},
	} {
		if err := s.UpsertObjectStorageUsage(ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.DeleteTenant(ctx, ws); err != nil {
		t.Fatal(err)
	}
	// The rows survive, retired: no pub- host can resolve them, the names are
	// free, and the purge still has its record of what is owed.
	for _, id := range []string{b.ID, sys.ID} {
		if _, err := s.GetObjectBucketByID(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("retired bucket %s still resolves: %v", id, err)
		}
	}
	var retired, live int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE deleted_at IS NOT NULL), count(*) FILTER (WHERE deleted_at IS NULL)
		FROM object_buckets WHERE workspace_id = $1`, ws).Scan(&retired, &live); err != nil {
		t.Fatal(err)
	}
	if retired != 2 || live != 0 {
		t.Fatalf("after DeleteTenant: %d retired and %d live rows, want 2 and 0", retired, live)
	}
	if _, err := s.GetObjectBucketByID(ctx, keep.ID); err != nil {
		t.Fatalf("another workspace's bucket was retired: %v", err)
	}

	if err := s.FinishObjectStoragePurge(ctx, ws); err != nil {
		t.Fatal(err)
	}
	var buckets, usage int
	if err := s.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM object_buckets WHERE workspace_id = $1),
		(SELECT count(*) FROM object_storage_usage WHERE workspace_id = $1)`, ws).Scan(&buckets, &usage); err != nil {
		t.Fatal(err)
	}
	if buckets != 0 || usage != 0 {
		t.Fatalf("after the purge finished: %d bucket rows and %d usage rows remain", buckets, usage)
	}
	if bytes, _, _ := s.ObjectStorageUsageTotals(ctx, other, "oregon"); bytes != 7 {
		t.Fatalf("finishing one workspace's purge touched another's usage: %d bytes, want 7", bytes)
	}
}

func TestDeleteTenantOfAMissingWorkspaceRetiresNothing(t *testing.T) {
	s, ws := objectBucketStore(t, "objpurge-missing")
	ctx := context.Background()
	b := newObjectBucket(t, s, ws, "photos", 10)
	ghost := "tea-ghost" + uniqueMachineRun()
	if err := s.DeleteTenant(ctx, ghost); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteTenant of a missing workspace = %v, want ErrNotFound", err)
	}
	if _, err := s.GetObjectBucketByID(ctx, b.ID); err != nil {
		t.Fatalf("a failed delete retired a live workspace's bucket: %v", err)
	}
}

func TestFinishObjectStoragePurgeKeepsALiveRowAndItsUsage(t *testing.T) {
	s, ws := objectBucketStore(t, "objpurge-race")
	ctx := context.Background()
	old := newObjectBucket(t, s, ws, "old", 10)
	if _, err := s.RetireObjectBuckets(ctx, ws); err != nil {
		t.Fatal(err)
	}
	// A bucket created in the race with the delete.
	racer := newObjectBucket(t, s, ws, "racer", 10)
	at := time.Now().UTC()
	for _, id := range []string{old.ID, racer.ID} {
		if err := s.UpsertObjectStorageUsage(ctx, ObjectStorageUsage{BucketID: id, WorkspaceID: ws, Region: "oregon", Bytes: 1, Objects: 1, ObservedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.FinishObjectStoragePurge(ctx, ws); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetObjectBucketByID(ctx, racer.ID); err != nil {
		t.Fatalf("a purge dropped a live bucket row: %v", err)
	}
	rows, _ := s.ListObjectStorageUsage(ctx, ws)
	if len(rows) != 2 {
		t.Fatalf("usage rows beside a live bucket = %d, want both kept until it is retired and purged", len(rows))
	}
}

func TestListObjectStorageWorkspacesFindsLiveRetiredAndGoneWorkspaces(t *testing.T) {
	s, live := objectBucketStore(t, "objlist-live")
	_, usageOnly := objectBucketStore(t, "objlist-usage")
	_, deleted := objectBucketStore(t, "objlist-deleted")
	_, never := objectBucketStore(t, "objlist-never")
	ctx := context.Background()
	gone := "tea-gone" + uniqueMachineRun()
	t.Cleanup(func() {
		_, _ = s.Pool.Exec(ctx, `DELETE FROM object_buckets WHERE workspace_id=$1`, gone)
	})

	newObjectBucket(t, s, live, "photos", 10)
	at := time.Now().UTC()
	if err := s.UpsertObjectStorageUsage(ctx, ObjectStorageUsage{BucketID: "orphan:" + usageOnly + ":oregon", WorkspaceID: usageOnly, Region: "oregon", Bytes: 1, Objects: 1, ObservedAt: at}); err != nil {
		t.Fatal(err)
	}
	newObjectBucket(t, s, deleted, "photos", 10)
	if err := s.DeleteTenant(ctx, deleted); err != nil {
		t.Fatal(err)
	}
	// Rows whose tenant is gone and were never retired.
	if _, err := s.CreateObjectBucket(ctx, ObjectBucket{WorkspaceID: gone, Name: "photos", Region: "oregon"}, 10); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListObjectStorageWorkspaces(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]ObjectStorageWorkspace{}
	order := map[string]int{}
	for i, w := range got {
		byID[w.ID] = w
		order[w.ID] = i
	}
	for id, retired := range map[string]bool{live: false, usageOnly: false, deleted: true, gone: true} {
		w, ok := byID[id]
		if !ok || w.Retired != retired {
			t.Errorf("workspace %s = %+v (listed %v), want retired=%v", id, w, ok, retired)
		}
	}
	if _, ok := byID[never]; ok {
		t.Errorf("a workspace with no object storage row is listed: the sweep would list Wasabi for nothing")
	}
	// Retired workspaces lead, the one never attempted before the one attempted.
	if order[deleted] > order[live] || order[gone] > order[usageOnly] {
		t.Errorf("a live workspace is listed before a retired one: %v", order)
	}
	if err := s.MarkObjectStoragePurgeAttempt(ctx, deleted, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ListObjectStorageWorkspaces(ctx)
	order = map[string]int{}
	for i, w := range got {
		order[w.ID] = i
	}
	if order[deleted] < order[gone] {
		t.Errorf("a purge just attempted (%d) leads one never attempted (%d): a failing purge would starve the rest", order[deleted], order[gone])
	}
}

// TestObjectStorageEgressSinceSumsEveryWorkspaceFromTheGivenHour is the number the
// month-to-date egress alert reads (ADR112 D4): all workspaces together, the
// egress kind only, nothing before the start. Other tests share the database, so
// the window is a year no other test writes to.
func TestObjectStorageEgressSinceSumsEveryWorkspaceFromTheGivenHour(t *testing.T) {
	s, ws := objectBucketStore(t, "objegress-a")
	_, other := objectBucketStore(t, "objegress-b")
	ctx := context.Background()
	monthStart := time.Date(2037, 3, 1, 0, 0, 0, 0, time.UTC)
	before, err := s.ObjectStorageEgressSince(ctx, monthStart)
	if err != nil {
		t.Fatal(err)
	}
	if before != 0 {
		t.Fatalf("the test year is not empty: %d", before)
	}
	err = s.AddObjectStorageUsage(ctx, []ObjectStorageUsageDelta{
		{WorkspaceID: ws, Kind: UsageKindObjectStorageEgressBytes, Window: monthStart.Add(-time.Hour), Quantity: 1000}, // last month
		{WorkspaceID: ws, Kind: UsageKindObjectStorageEgressBytes, Window: monthStart, Quantity: 10},                   // the first hour counts
		{WorkspaceID: ws, Kind: UsageKindObjectStorageEgressBytes, Window: monthStart.Add(30 * 24 * time.Hour), Quantity: 20},
		{WorkspaceID: other, Kind: UsageKindObjectStorageEgressBytes, Window: monthStart.Add(5 * time.Hour), Quantity: 300},
		{WorkspaceID: ws, Kind: UsageKindObjectStorageIngressBytes, Window: monthStart.Add(time.Hour), Quantity: 5000}, // not egress
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ObjectStorageEgressSince(ctx, monthStart)
	if err != nil || got != 330 {
		t.Fatalf("egress since the month's start = %d, %v; want 330 (10 + 20 + 300: both workspaces, not last month, not ingress)", got, err)
	}
	if got, _ := s.ObjectStorageEgressSince(ctx, monthStart.Add(time.Hour)); got != 320 {
		t.Errorf("one hour later = %d, want 320", got)
	}
	// A time inside an hour starts at the hour: the hour's row is not split.
	if got, _ := s.ObjectStorageEgressSince(ctx, monthStart.Add(30*time.Minute)); got != 330 {
		t.Errorf("half past the first hour = %d, want 330", got)
	}
}

// TestEgressSumHasAnIndexAndItsDownMigrationRemovesIt: the month-to-date egress
// gauge reads one kind across workspaces by window every minute on every
// replica, and no other usage_hourly index serves that (0275). The index is
// partial, so it holds only the egress rows, and the sum can be answered from it.
func TestEgressSumHasAnIndexAndItsDownMigrationRemovesIt(t *testing.T) {
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
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const name = "usage_hourly_object_storage_egress_idx"
	exists := func() bool {
		t.Helper()
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'usage_hourly' AND indexname = $1)`, name).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	read := func(file string) string {
		raw, err := migrationsFS.ReadFile("migrations/" + file)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	if !exists() {
		t.Fatal("0275 left no index for the egress sum")
	}
	var def string
	if err := tx.QueryRow(ctx, `SELECT indexdef FROM pg_indexes WHERE indexname = $1`, name).Scan(&def); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"window_start", "INCLUDE (quantity)", "object_storage_egress_bytes", "object_storage"} {
		if !strings.Contains(def, want) {
			t.Errorf("the index is %q, want it to name %q", def, want)
		}
	}
	if _, err := tx.Exec(ctx, read("0275_object_storage_egress_index.down.sql")); err != nil {
		t.Fatal(err)
	}
	if exists() {
		t.Fatal("the down migration left the index")
	}
	if _, err := tx.Exec(ctx, read("0275_object_storage_egress_index.up.sql")); err != nil || !exists() {
		t.Fatalf("the up migration did not restore the index after a down: %v", err)
	}
}
