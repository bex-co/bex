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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func TestStorageIntervalChunks(t *testing.T) {
	// 400 MB for one second is 0.4 GB-seconds: three one-second passes carry the
	// fraction and yield one whole unit on the third, not zero three times.
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	var remainder, total int64
	for i := range 3 {
		chunks, r := storageIntervalChunks(400_000_000, start.Add(time.Duration(i)*time.Second), 1, remainder)
		remainder = r
		for _, c := range chunks {
			total += c.quantity
		}
	}
	if total != 1 || remainder != 200_000_000 {
		t.Errorf("carried = %d with remainder %d, want 1 and 200000000", total, remainder)
	}

	// 10 GB across an hour boundary: 600 s on one side, 3,000 s on the other.
	chunks, remainder := storageIntervalChunks(10_000_000_000, start.Add(50*time.Minute), 3600, 0)
	if remainder != 0 || len(chunks) != 2 ||
		!chunks[0].window.Equal(start) || chunks[0].quantity != 10*600 ||
		!chunks[1].window.Equal(start.Add(time.Hour)) || chunks[1].quantity != 10*3000 {
		t.Errorf("hour split = %+v remainder %d", chunks, remainder)
	}
}

// TestSandboxStorageMeterPG proves the storage meter against Postgres (w22/m313):
// a polled suspended sandbox opens its object, a 10 GB image kept 24 h records
// 240 GB-hours and closing it stops accrual, replays change nothing, a registry
// outage delays a charge without losing or inventing seconds, a size read for an
// earlier period is refused after the object reopens, and a sandbox that
// vanishes while suspended closes its object.
func TestSandboxStorageMeterPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "sandbox-storage-meter", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	ws := tenant.ID

	t0 := time.Date(2026, 8, 1, 0, 0, 10, 0, time.UTC)
	observe := func(id, phase string, polled bool, at time.Time) {
		t.Helper()
		if err := s.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: ws, SandboxID: id, Phase: phase, Tier: "1c-2g", WeightMilli: 1000,
			StorageRef: "os-" + id, Polled: polled, ObservedAt: at,
		}); err != nil {
			t.Fatalf("observe %s %s at %s: %v", id, phase, at, err)
		}
	}
	stored := func(id string) int64 {
		t.Helper()
		rows, err := s.UsageMonthToDate(ctx, ws, t0.Add(30*24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		var total int64
		for _, r := range rows {
			if r.ServiceID == id && r.Kind == UsageKindSandboxSnapshotGBSeconds {
				if r.ResourceKind != ResourceKindSandbox {
					t.Errorf("storage row resource kind = %q", r.ResourceKind)
				}
				total += r.Total
			}
		}
		return total
	}
	size := func(id string, bytes int64) SandboxStorageObject {
		t.Helper()
		unsized, err := s.ClaimUnsizedSandboxStorage(ctx, t0, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range unsized {
			if o.ObjectID == id {
				if err := s.SetSandboxStorageBytes(ctx, ws, id, o.StartedAt, bytes); err != nil {
					t.Fatal(err)
				}
				return o
			}
		}
		t.Fatalf("%s is not waiting for a size: %+v", id, unsized)
		return SandboxStorageObject{}
	}

	// A request-path suspended (the pause was only accepted) and a suspending
	// poll open nothing: no image has been committed, so there is none to size.
	observe("a", "running", true, t0)
	observe("a", "suspended", false, t0.Add(time.Minute))
	observe("a", "suspending", true, t0.Add(2*time.Minute))
	if st, err := s.SandboxStorageStats(ctx, t0.Add(time.Hour)); err != nil || st.Open != 0 {
		t.Fatalf("stats before the committed image = %+v err=%v, want nothing open", st, err)
	}
	// The polled Paused state opens the object, and a replay of it changes nothing.
	opened := t0.Add(3 * time.Minute)
	observe("a", "suspended", true, opened)
	observe("a", "suspended", true, opened)
	observe("a", "suspended", true, opened.Add(30*time.Second))
	obj := size("a", 10_000_000_000)
	if !obj.StartedAt.Equal(opened) || obj.Ref != "os-a" || obj.Kind != StorageKindRootfs {
		t.Fatalf("object = %+v, want started %s ref os-a", obj, opened)
	}

	// A 10 GB image kept 24 h is 240 GB-hours; closing it stops the accrual.
	if n, err := s.AccrueSandboxStorage(ctx, opened.Add(2*time.Minute), 10); err != nil || n != 0 {
		t.Fatalf("accrual inside the staleness window advanced %d err=%v", n, err)
	}
	end := opened.Add(24 * time.Hour)
	observe("a", "running", true, end)
	for range 2 { // the second pass is a replay
		if _, err := s.AccrueSandboxStorage(ctx, end.Add(time.Hour), 10); err != nil {
			t.Fatal(err)
		}
	}
	if got := stored("a"); got != 10*24*3600 {
		t.Errorf("stored = %d GB-seconds, want %d (240 GB-hours)", got, 10*24*3600)
	}
	// Replaying the end, and the start of the same period, changes nothing.
	if err := s.EndSandboxStorage(ctx, ws, "a", end.Add(5*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.StartSandboxStorage(ctx, ws, "a", StorageKindRootfs, "os-a", opened); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccrueSandboxStorage(ctx, end.Add(48*time.Hour), 10); err != nil {
		t.Fatal(err)
	}
	if got := stored("a"); got != 10*24*3600 {
		t.Errorf("after replays stored = %d, want %d", got, 10*24*3600)
	}

	// Suspended again: the object reopens unsized. A size read for the earlier
	// period is refused; the registry being down for two hours delays the charge
	// and then covers the whole interval, neither losing nor inventing seconds.
	reopened := end.Add(26 * time.Hour)
	observe("a", "suspended", true, reopened)
	if err := s.SetSandboxStorageBytes(ctx, ws, "a", opened, 1); err != nil {
		t.Fatal(err)
	}
	if unsized, _ := s.ClaimUnsizedSandboxStorage(ctx, t0, 100); len(unsized) != 1 {
		t.Fatalf("a stale size must be refused: unsized = %+v", unsized)
	}
	if _, err := s.AccrueSandboxStorage(ctx, reopened.Add(2*time.Hour), 10); err != nil {
		t.Fatal(err)
	}
	if got := stored("a"); got != 10*24*3600 {
		t.Errorf("unsized object accrued: stored = %d", got)
	}
	size("a", 2_000_000_000)
	gone := reopened.Add(3 * time.Hour)
	// The sandbox vanished while suspended (terminated, or expired): the complete
	// list no longer holds it, which closes the object at that observation.
	if err := s.TerminateMissingSandboxMeters(ctx, ws, []string{}, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccrueSandboxStorage(ctx, gone.Add(time.Hour), 10); err != nil {
		t.Fatal(err)
	}
	if got, want := stored("a"), int64(10*24*3600+2*3*3600); got != want {
		t.Errorf("stored = %d, want %d (the first period plus 2 GB for 3 h)", got, want)
	}

	// A closed object that was never sized is never billed as zero silently: the
	// stats name it.
	observe("b", "suspended", true, t0)
	observe("b", "running", true, t0.Add(time.Hour))
	// A pause that ends within StorageUnsizedGrace may never have committed an
	// image; it is not a loss worth an alert (production QA churns these daily).
	observe("c", "suspended", true, t0)
	observe("c", "running", true, t0.Add(10*time.Second))
	st, err := s.SandboxStorageStats(ctx, end.Add(72*time.Hour))
	if err != nil || st.UnsizedClosed != 1 || st.Open != 0 || st.AccrualBacklog != 0 {
		t.Errorf("stats = %+v err=%v, want one unsized closed object (the hour-long one, not the 10 s one) and nothing open or pending", st, err)
	}

	// Snapshot objects (w32/m285) register through the same calls.
	if err := s.StartSandboxStorage(ctx, ws, "snp-1", StorageKindSnapshot, "", t0); err != nil {
		t.Fatal(err)
	}
	// A snapshot's size is registered by its owner (w32/m285), not read by the meter.
	if err := s.SetSandboxStorageBytes(ctx, ws, "snp-1", t0, 40_000_000_000); err != nil {
		t.Fatal(err)
	}
	if err := s.EndSandboxStorage(ctx, ws, "snp-1", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AccrueSandboxStorage(ctx, t0.Add(2*time.Hour), 10); err != nil {
		t.Fatal(err)
	}
	if got := stored("snp-1"); got != 40*3600 {
		t.Errorf("snapshot stored = %d GB-seconds, want %d (40 GB for an hour)", got, 40*3600)
	}
	if err := s.StartSandboxStorage(ctx, ws, "x", "disk", "", t0); err == nil {
		t.Error("an unknown storage kind must be refused")
	}
}

// TestSandboxStorageReapAndCapPG: a sandbox that is gone (terminated, or
// expired out of the complete list) queues its image for the reaper and a
// resumed one does not; reaping is marked per period; a workspace's open bytes
// are what its cap reads; and a sandbox with no storage ref is left out.
func TestSandboxStorageReapAndCapPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "sandbox-storage-reap", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	ws := tenant.ID
	t0 := time.Date(2026, 8, 1, 0, 0, 10, 0, time.UTC)
	observe := func(id, ref, phase string, at time.Time) {
		t.Helper()
		if err := s.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: ws, SandboxID: id, Phase: phase, Tier: "1c-2g", WeightMilli: 1000,
			StorageRef: ref, Polled: true, ObservedAt: at,
		}); err != nil {
			t.Fatalf("observe %s %s: %v", id, phase, err)
		}
	}
	size := func(id string, bytes int64) {
		t.Helper()
		unsized, err := s.ClaimUnsizedSandboxStorage(ctx, t0, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range unsized {
			if o.ObjectID == id {
				if err := s.SetSandboxStorageBytes(ctx, ws, id, o.StartedAt, bytes); err != nil {
					t.Fatal(err)
				}
				return
			}
		}
		t.Fatalf("%s is not waiting for a size", id)
	}

	for _, id := range []string{"terminated", "expired", "resumed", "resumed-gone", "open-a", "open-b"} {
		observe(id, "os-"+id, "suspended", t0)
	}
	size("terminated", 3_000_000_000)
	size("expired", 1_000_000_000)
	size("resumed", 2_000_000_000)
	size("resumed-gone", 1_500_000_000)
	size("open-a", 3_000_000_000)
	size("open-b", 5_000_000_000)
	observe("open-c", "os-open-c", "suspended", t0) // retained, not sized yet
	observe("agent", "", "suspended", t0)           // no storage ref: left out

	observe("terminated", "os-terminated", "terminated", t0.Add(time.Hour)) // the request path
	observe("resumed", "os-resumed", "resuming", t0.Add(time.Hour))
	// Resumed, then terminated while running: its old tags are still in the registry.
	observe("resumed-gone", "os-resumed-gone", "resuming", t0.Add(time.Hour))
	observe("resumed-gone", "os-resumed-gone", "running", t0.Add(90*time.Minute))
	observe("resumed-gone", "os-resumed-gone", "terminated", t0.Add(100*time.Minute))
	// The sandbox that expired at pause + 30 days drops out of the complete list.
	if err := s.TerminateMissingSandboxMeters(ctx, ws,
		[]string{"resumed", "open-a", "open-b", "open-c", "agent"}, t0.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	reapable, err := s.ListReapableSandboxStorage(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, o := range reapable {
		ids = append(ids, o.ObjectID+":"+o.Ref)
	}
	// The resumed sandbox that is still alive is not queued; the one that was
	// resumed and then terminated is, though it holds no storage any more.
	if len(ids) != 3 || ids[0] != "terminated:os-terminated" || ids[1] != "resumed-gone:os-resumed-gone" || ids[2] != "expired:os-expired" {
		t.Errorf("reapable = %v, want the terminated, the resumed-then-terminated and the expired image, oldest first, and not the live resumed one", ids)
	}
	if err := s.MarkSandboxStorageReaped(ctx, ws, "terminated", reapable[0].StartedAt.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.ListReapableSandboxStorage(ctx, 10); len(again) != 3 {
		t.Errorf("a mark for another period must not clear the object: %d reapable", len(again))
	}
	if err := s.MarkSandboxStorageReaped(ctx, ws, "terminated", reapable[0].StartedAt); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.ListReapableSandboxStorage(ctx, 10); len(again) != 2 {
		t.Errorf("after reaping one: %+v", again)
	}

	// The cap reads the open, sized objects: 3 GB + 5 GB, with one still unsized.
	bytes, err := s.SandboxStorageBytes(ctx, ws)
	if err != nil || bytes != 8_000_000_000 {
		t.Errorf("workspace storage = %d bytes (err %v), want 8e9: an unsized object counts for nothing", bytes, err)
	}
	stats, err := s.SandboxStorageStats(ctx, t0.Add(3*time.Hour))
	if err != nil || stats.Open != 3 || stats.ReapPending != 2 || stats.OpenBytes != 8_000_000_000 {
		t.Errorf("stats = %+v err %v, want 3 open, 2 awaiting reap, 8e9 bytes", stats, err)
	}
	if other, _ := s.SandboxStorageBytes(ctx, "tea-other"); other != 0 {
		t.Errorf("another workspace reads %d bytes", other)
	}
}

// TestSandboxStorageQueueRotatesAndPrunes: the unsized queue hands out the
// objects tried least recently first, so ones whose size cannot be read do not
// starve newer ones; and a settled, closed object past the retention window is
// pruned while one that is open, unsettled, or waiting for the reaper is not.
func TestSandboxStorageQueueRotatesAndPrunes(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "sandbox-storage-rotate", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	ws := tenant.ID
	t0 := time.Date(2026, 8, 1, 0, 0, 10, 0, time.UTC)

	for i, id := range []string{"a", "b", "c", "d"} {
		if err := s.StartSandboxStorage(ctx, ws, id, StorageKindRootfs, "os-"+id, t0.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	// A snapshot object has no registry ref to read: it never enters the queue.
	if err := s.StartSandboxStorage(ctx, ws, "snp-1", StorageKindSnapshot, "", t0); err != nil {
		t.Fatal(err)
	}
	ids := func(objs []SandboxStorageObject) string {
		var out []string
		for _, o := range objs {
			out = append(out, o.ObjectID)
		}
		return strings.Join(out, ",")
	}
	first, err := s.ClaimUnsizedSandboxStorage(ctx, t0.Add(time.Hour), 2)
	if err != nil || ids(first) != "a,b" {
		t.Fatalf("first claim = %q err %v, want the two oldest", ids(first), err)
	}
	// a and b are tried and still unsized: the next claim moves on to c and d.
	second, _ := s.ClaimUnsizedSandboxStorage(ctx, t0.Add(2*time.Hour), 2)
	if ids(second) != "c,d" {
		t.Errorf("second claim = %q, want c,d: failing objects must not hold the head of the queue", ids(second))
	}
	third, _ := s.ClaimUnsizedSandboxStorage(ctx, t0.Add(3*time.Hour), 2)
	if ids(third) != "a,b" {
		t.Errorf("third claim = %q, want the rotation back to a,b", ids(third))
	}

	// Pruning. settled: closed long ago and fully accrued; unsettled-sized: closed
	// but its seconds are not accrued yet; never-sized: closed, never sized;
	// queued: waiting for the reaper; recent: closed inside the retention window.
	mk := func(id string, bytes *int64, endAfter time.Duration, accrue, reap bool) {
		t.Helper()
		if err := s.StartSandboxStorage(ctx, ws, id, StorageKindRootfs, "os-"+id, t0); err != nil {
			t.Fatal(err)
		}
		if bytes != nil {
			if err := s.SetSandboxStorageBytes(ctx, ws, id, t0, *bytes); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.EndSandboxStorage(ctx, ws, id, t0.Add(endAfter)); err != nil {
			t.Fatal(err)
		}
		if accrue {
			if _, err := s.AccrueSandboxStorage(ctx, t0.Add(endAfter+time.Hour), 100); err != nil {
				t.Fatal(err)
			}
		}
		if reap {
			if _, err := pool.Exec(ctx, `UPDATE sandbox_storage_objects SET reap_pending = true WHERE workspace_id = $1 AND object_id = $2`, ws, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	gb := int64(1_000_000_000)
	mk("settled", &gb, time.Hour, true, false)
	mk("never-sized", nil, time.Hour, false, false)
	mk("queued", &gb, time.Hour, true, true)
	mk("recent", &gb, 40*24*time.Hour, true, false)
	// Closed long ago but its seconds were never added to usage: keep it until they are.
	mk("unsettled", &gb, time.Hour, false, false)

	now := t0.Add(StorageRetention + 2*time.Hour) // settled and never-sized are older than the window; recent is not
	n, err := s.PruneSandboxStorage(ctx, now, 100)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	rows, err := pool.Query(ctx, `SELECT object_id FROM sandbox_storage_objects WHERE workspace_id = $1 AND object_id IN ('settled','never-sized','queued','recent','unsettled','a') ORDER BY object_id`, ws)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		left = append(left, id)
	}
	rows.Close()
	if n != 2 || strings.Join(left, ",") != "a,queued,recent,unsettled" {
		t.Errorf("pruned %d, left %v: want settled and never-sized gone; open, queued, recent and unsettled kept", n, left)
	}
}
