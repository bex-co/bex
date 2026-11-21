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

package objectstorage_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

var _ objectstorage.SweepRepository = (*store.PGStore)(nil)
var _ objectstorage.PurgeRepository = (*store.PGStore)(nil)
var _ objectstorage.Exclusive = (*store.PGStore)(nil)
var _ objectstorage.UsageSink = (*store.PGStore)(nil)

// scopedRepo lists only the workspaces a test made. The sweep lists every
// workspace in the database, and the database is shared with other tests, whose
// rows a pass must not retire or purge.
type scopedRepo struct {
	objectstorage.SweepRepository
	only map[string]bool
}

func (r scopedRepo) ListObjectStorageWorkspaces(ctx context.Context) ([]store.ObjectStorageWorkspace, error) {
	all, err := r.SweepRepository.ListObjectStorageWorkspaces(ctx)
	var out []store.ObjectStorageWorkspace
	for _, w := range all {
		if r.only[w.ID] {
			out = append(out, w)
		}
	}
	return out, err
}

// ListPurgedObjectStorageWorkspaces keeps the store's order and limit over the
// test's own workspaces.
func (r scopedRepo) ListPurgedObjectStorageWorkspaces(ctx context.Context, limit int) ([]string, error) {
	all, err := r.SweepRepository.ListPurgedObjectStorageWorkspaces(ctx, 1_000_000)
	var out []string
	for _, ws := range all {
		if r.only[ws] && len(out) < limit {
			out = append(out, ws)
		}
	}
	return out, err
}

// sweepEnv is a real store and a fake Wasabi per region, a workspace, and a
// sweeper whose clock the test sets.
type sweepEnv struct {
	t         *testing.T
	PG        *store.PGStore
	Pool      *pgxpool.Pool
	WS        string
	Svc       *objectstorage.Service
	Sweeper   *objectstorage.Sweeper
	Fakes     map[objectstorage.Region]*fakes3.Server
	Now       time.Time
	Logs      []string
	scoped    map[string]bool
	logsMutex sync.Mutex
}

var testNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// newSweepEnv builds the environment. wrap, when set, wraps each region's store,
// to make a delete fail or to hide the batch delete.
func newSweepEnv(t *testing.T, name string, wrap ...func(objectstorage.Region, objectstorage.Store) objectstorage.Store) *sweepEnv {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	pg := store.NewPGStore(pool)
	tenant, err := pg.CreateTenant(ctx, name+"-"+time.Now().Format("150405.000000000"), store.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	e := &sweepEnv{t: t, PG: pg, Pool: pool, WS: tenant.ID, Now: testNow, Fakes: map[objectstorage.Region]*fakes3.Server{}, scoped: map[string]bool{tenant.ID: true}}
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM object_storage_usage WHERE workspace_id=$1`, tenant.ID)
		_, _ = pool.Exec(bg, `DELETE FROM object_multipart_uploads WHERE workspace_id=$1`, tenant.ID)
		_, _ = pool.Exec(bg, `DELETE FROM object_buckets WHERE workspace_id=$1`, tenant.ID)
		_, _ = pool.Exec(bg, `DELETE FROM object_storage_purged_workspaces WHERE workspace_id=$1`, tenant.ID)
		_, _ = pool.Exec(bg, `DELETE FROM tenants WHERE id=$1`, tenant.ID)
	})
	stores := map[objectstorage.Region]objectstorage.Store{}
	for _, region := range objectstorage.Regions {
		fake := fakes3.New(region.Bucket())
		t.Cleanup(fake.Close)
		e.Fakes[region] = fake
		var st objectstorage.Store = objectstorage.NewStore(objectstorage.Config{
			Endpoint: fake.URL, Region: region.WasabiRegion(), Bucket: region.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1,
		})
		for _, w := range wrap {
			st = w(region, st)
		}
		stores[region] = st
	}
	e.Svc = objectstorage.NewService(pg, stores)
	e.Sweeper = objectstorage.NewSweeper(e.Svc, scopedRepo{SweepRepository: pg, only: e.scoped})
	e.Sweeper.MinGap = 0
	e.Sweeper.Now = func() time.Time { return e.Now }
	e.Sweeper.Logf = func(format string, args ...any) {
		e.logsMutex.Lock()
		defer e.logsMutex.Unlock()
		e.Logs = append(e.Logs, fmt.Sprintf(format, args...))
		t.Logf(format, args...)
	}
	return e
}

func (e *sweepEnv) logged(substr string) bool {
	e.logsMutex.Lock()
	defer e.logsMutex.Unlock()
	for _, l := range e.Logs {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

// bucket creates a user bucket row of the workspace.
func (e *sweepEnv) bucket(name string, region objectstorage.Region) store.ObjectBucket {
	e.t.Helper()
	b, err := e.PG.CreateObjectBucket(context.Background(), store.ObjectBucket{WorkspaceID: e.WS, Name: name, Region: string(region)}, 100)
	if err != nil {
		e.t.Fatal(err)
	}
	return b
}

// put stores an object of size bytes in a bucket, last modified age ago.
func (e *sweepEnv) put(b store.ObjectBucket, key string, size int, age time.Duration) string {
	e.t.Helper()
	return e.putAt(e.WS, b.ID, objectstorage.Region(b.Region), key, size, age)
}

func (e *sweepEnv) putAt(ws, bucketID string, region objectstorage.Region, key string, size int, age time.Duration) string {
	e.t.Helper()
	wasabiKey, err := objectstorage.ObjectKey(ws, bucketID, key)
	if err != nil {
		e.t.Fatal(err)
	}
	e.putRaw(region, wasabiKey, size, age)
	return wasabiKey
}

// putRaw stores an object under any Wasabi key, including one no bucket owns.
func (e *sweepEnv) putRaw(region objectstorage.Region, wasabiKey string, size int, age time.Duration) {
	e.t.Helper()
	fake := e.Fakes[region]
	fake.Put(region.Bucket(), wasabiKey, make([]byte, size))
	fake.SetLastModified(region.Bucket(), wasabiKey, e.Now.Add(-age))
}

func (e *sweepEnv) store(region objectstorage.Region) objectstorage.Store {
	e.t.Helper()
	st, err := e.Svc.Store(region)
	if err != nil {
		e.t.Fatal(err)
	}
	return st
}

func (e *sweepEnv) sweep() error {
	e.t.Helper()
	_, err := e.Sweeper.SweepWorkspace(context.Background(), e.WS)
	return err
}

func (e *sweepEnv) mustSweep() {
	e.t.Helper()
	if err := e.sweep(); err != nil {
		e.t.Fatalf("sweep: %v", err)
	}
}

// usage is the workspace's usage rows by bucket id.
func (e *sweepEnv) usage() map[string]store.ObjectStorageUsage {
	e.t.Helper()
	rows, err := e.PG.ListObjectStorageUsage(context.Background(), e.WS)
	if err != nil {
		e.t.Fatal(err)
	}
	out := map[string]store.ObjectStorageUsage{}
	for _, r := range rows {
		out[r.BucketID] = r
	}
	return out
}

func (e *sweepEnv) wantUsage(id string, bytes, objects int64) {
	e.t.Helper()
	u, ok := e.usage()[id]
	if !ok || u.Bytes != bytes || u.Objects != objects || u.WorkspaceID != e.WS {
		e.t.Fatalf("usage of %s = %+v (present %v), want %d bytes in %d objects", id, u, ok, bytes, objects)
	}
}

// listRequests counts the list requests a fake saw and returns their prefixes.
func listRequests(fake *fakes3.Server) []string {
	var prefixes []string
	for _, r := range fake.Requests() {
		if r.Method == "GET" && r.Key == "" {
			prefixes = append(prefixes, r.Query.Get("prefix"))
		}
	}
	return prefixes
}

func TestSweepAttributesBytesAndObjectsToEachBucketOfAWorkspace(t *testing.T) {
	e := newSweepEnv(t, "sweep-attr")
	photos, videos := e.bucket("photos", objectstorage.Oregon), e.bucket("videos", objectstorage.Oregon)
	e.put(photos, "a.jpg", 10, time.Minute)
	e.put(photos, "dir/b.jpg", 20, time.Minute)
	e.put(photos, "dir/sub/c.jpg", 30, time.Minute)
	e.put(videos, "v.mp4", 100, time.Minute)

	e.mustSweep()

	e.wantUsage(photos.ID, 60, 3)
	e.wantUsage(videos.ID, 100, 1)
	if rows := e.usage(); len(rows) != 2 {
		t.Fatalf("usage rows = %+v, want one per bucket and no orphan row", rows)
	}
	for _, u := range e.usage() {
		if u.Region != "oregon" || !u.ObservedAt.Equal(testNow) {
			t.Errorf("usage row = %+v, want region oregon observed at %s", u, testNow)
		}
	}
	// The quota gate reads the sum of the buckets.
	if bytes, objects, _ := e.PG.ObjectStorageUsageTotals(context.Background(), e.WS, "oregon"); bytes != 160 || objects != 4 {
		t.Fatalf("totals = %d bytes, %d objects, want 160 and 4", bytes, objects)
	}
}

func TestSweepIsPerWorkspacePrefixNeverTheWholeSharedBucket(t *testing.T) {
	e := newSweepEnv(t, "sweep-prefix")
	b := e.bucket("photos", objectstorage.Oregon)
	e.put(b, "mine", 10, time.Minute)
	// Another workspace's objects share the Wasabi bucket; they are neither
	// listed for this workspace nor counted.
	other := ids.New(ids.Workspace)
	e.putAt(other, ids.New(ids.ObjectBucket), objectstorage.Oregon, "theirs", 5000, time.Minute)

	e.mustSweep()

	e.wantUsage(b.ID, 10, 1)
	if got := listRequests(e.Fakes[objectstorage.Oregon]); len(got) != 1 || got[0] != e.WS+"/" {
		t.Fatalf("list prefixes = %q, want exactly %q", got, e.WS+"/")
	}
}

func TestSweepRecordsAnEmptyBucketAsZero(t *testing.T) {
	e := newSweepEnv(t, "sweep-empty")
	b := e.bucket("empty", objectstorage.Oregon)
	e.mustSweep()
	e.wantUsage(b.ID, 0, 0)
	if rows := e.usage(); len(rows) != 1 {
		t.Fatalf("usage rows = %+v, want one zero row and no orphan marker", rows)
	}
}

func TestSweepRecordsObjectsOfNoLiveBucketUnderTheOrphanMarker(t *testing.T) {
	e := newSweepEnv(t, "sweep-orphan")
	b := e.bucket("photos", objectstorage.Oregon)
	east := e.bucket("east", objectstorage.Frankfurt)
	e.put(b, "mine", 10, time.Minute)
	// An obk- id with no row (a bucket deleted while a put was in flight), a key
	// that is not <tea>/<obk>/<key> at all, a prefix with no key under it, and an
	// object under a bucket of the workspace that lives in the other region.
	orphan := e.putAt(e.WS, ids.New(ids.ObjectBucket), objectstorage.Oregon, "x/y", 7, time.Minute)
	e.putRaw(objectstorage.Oregon, e.WS+"/stray.txt", 3, time.Minute)
	e.putRaw(objectstorage.Oregon, e.WS+"/"+ids.New(ids.ObjectBucket)+"/", 1, time.Minute)
	e.putAt(e.WS, east.ID, objectstorage.Oregon, "misplaced", 4, time.Minute)

	e.mustSweep()

	marker := objectstorage.OrphanBucketID(e.WS, objectstorage.Oregon)
	if !objectstorage.IsOrphanBucketID(marker) || objectstorage.IsOrphanBucketID(b.ID) {
		t.Fatalf("IsOrphanBucketID is wrong for %q or %q", marker, b.ID)
	}
	e.wantUsage(b.ID, 10, 1)
	e.wantUsage(east.ID, 0, 0)
	e.wantUsage(marker, 7+3+1+4, 4)
	// Orphans occupy Wasabi, so they count towards the workspace's quota.
	if bytes, objects, _ := e.PG.ObjectStorageUsageTotals(context.Background(), e.WS, "oregon"); bytes != 25 || objects != 5 {
		t.Fatalf("totals = %d bytes, %d objects, want 25 and 5 with the orphans", bytes, objects)
	}

	// A sweep does not delete them from a live workspace (a workspace delete
	// purges them with the rest of its prefix), and once they are gone the marker
	// goes with them.
	fake, oregon := e.Fakes[objectstorage.Oregon], e.store(objectstorage.Oregon)
	if _, ok := fake.Get(objectstorage.Oregon.Bucket(), orphan); !ok {
		t.Fatal("the sweep deleted an orphan object of a live workspace")
	}
	for _, k := range fake.Keys(objectstorage.Oregon.Bucket(), e.WS+"/") {
		if !strings.HasPrefix(k, e.WS+"/"+b.ID+"/") {
			if err := oregon.Delete(context.Background(), k); err != nil {
				t.Fatal(err)
			}
		}
	}
	e.mustSweep()
	if _, ok := e.usage()[marker]; ok {
		t.Fatalf("the orphan marker row stays after its objects are gone: %+v", e.usage())
	}
	e.wantUsage(b.ID, 10, 1)
}

func TestOneFailingRegionDoesNotStopTheOtherAndKeepsItsOldRows(t *testing.T) {
	e := newSweepEnv(t, "sweep-regions")
	west, east := e.bucket("west", objectstorage.Oregon), e.bucket("east", objectstorage.Frankfurt)
	e.put(west, "a", 10, time.Minute)
	e.put(east, "a", 20, time.Minute)
	e.mustSweep()
	e.wantUsage(west.ID, 10, 1)
	e.wantUsage(east.ID, 20, 1)

	// Frankfurt goes down; both regions gain an object.
	e.put(west, "b", 100, time.Minute)
	e.put(east, "b", 200, time.Minute)
	e.Fakes[objectstorage.Frankfurt].SetDown(true)
	e.Now = e.Now.Add(time.Hour)
	err := e.sweep()
	if err == nil || !strings.Contains(err.Error(), "frankfurt") {
		t.Fatalf("sweep with frankfurt down = %v, want an error naming frankfurt", err)
	}
	e.wantUsage(west.ID, 110, 2) // oregon was swept
	e.wantUsage(east.ID, 20, 1)  // frankfurt kept its old row...
	if u := e.usage()[east.ID]; !u.ObservedAt.Equal(testNow) {
		t.Fatalf("frankfurt's row was touched: observed at %s, want the old %s", u.ObservedAt, testNow)
	}

	// ...and a sweep the other way round: oregon down, frankfurt back.
	e.Fakes[objectstorage.Frankfurt].SetDown(false)
	e.Fakes[objectstorage.Oregon].SetDown(true)
	e.Now = e.Now.Add(time.Hour)
	if err := e.sweep(); err == nil || !strings.Contains(err.Error(), "oregon") {
		t.Fatalf("sweep with oregon down = %v, want an error naming oregon", err)
	}
	e.wantUsage(east.ID, 220, 2)
	e.wantUsage(west.ID, 110, 2)
}

func TestSweepListsOnlyTheRegionsAWorkspaceHasRowsIn(t *testing.T) {
	e := newSweepEnv(t, "sweep-onlyregion")
	b := e.bucket("west", objectstorage.Oregon)
	e.put(b, "a", 1, time.Minute)
	e.mustSweep()
	if n := len(e.Fakes[objectstorage.Frankfurt].Requests()); n != 0 {
		t.Fatalf("frankfurt saw %d requests for a workspace with no bucket there", n)
	}
}

func TestSweepLeavesAWorkspaceSweptRecentlyAlone(t *testing.T) {
	e := newSweepEnv(t, "sweep-gap")
	e.Sweeper.MinGap = 45 * time.Minute
	b := e.bucket("photos", objectstorage.Oregon)
	e.put(b, "a", 10, time.Minute)
	e.mustSweep()
	fake := e.Fakes[objectstorage.Oregon]
	first := len(listRequests(fake))

	// Another replica's tick, half an hour later: nothing is listed again.
	e.put(b, "b", 5, time.Minute)
	e.Now = e.Now.Add(30 * time.Minute)
	e.mustSweep()
	if got := len(listRequests(fake)); got != first {
		t.Fatalf("a workspace swept 30 minutes ago was listed again (%d list requests, want %d)", got, first)
	}
	e.wantUsage(b.ID, 10, 1)

	e.Now = e.Now.Add(20 * time.Minute) // 50 minutes after the sweep
	e.mustSweep()
	e.wantUsage(b.ID, 15, 2)
}

// exclusiveStub records the lock name and runs the function only when allowed.
type exclusiveStub struct {
	run  bool
	name string
	done chan struct{}
}

func (x *exclusiveStub) RunExclusive(ctx context.Context, name string, fn func(context.Context) error) (bool, error) {
	defer close(x.done)
	x.name = name
	if !x.run {
		return false, nil
	}
	return true, fn(ctx)
}

// runOnce starts Run, waits for its first pass to finish, and stops it.
func runOnce(t *testing.T, sw *objectstorage.Sweeper, x *exclusiveStub) {
	t.Helper()
	sw.Exclusive = x
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		sw.Run(ctx, time.Hour)
		close(finished)
	}()
	select {
	case <-x.done:
	case <-time.After(20 * time.Second):
		t.Fatal("the sweep never ran its first pass")
	}
	cancel()
	<-finished
}

func TestRunSweepsThroughTheExclusiveLockAndANonHolderDoesNothing(t *testing.T) {
	e := newSweepEnv(t, "sweep-run")
	b := e.bucket("photos", objectstorage.Oregon)
	e.put(b, "a", 10, time.Minute)
	fake := e.Fakes[objectstorage.Oregon]

	// A replica that does not get the lock lists nothing.
	runOnce(t, e.Sweeper, &exclusiveStub{run: false, done: make(chan struct{})})
	if n := len(fake.Requests()); n != 0 {
		t.Fatalf("a replica without the lock sent %d requests to Wasabi", n)
	}

	holder := &exclusiveStub{run: true, done: make(chan struct{})}
	runOnce(t, e.Sweeper, holder)
	if holder.name != "object-storage-sweep" {
		t.Fatalf("lock name = %q", holder.name)
	}
	e.wantUsage(b.ID, 10, 1)
}

func TestPassReportsFailuresAndKeepsGoing(t *testing.T) {
	e := newSweepEnv(t, "sweep-pass")
	b := e.bucket("photos", objectstorage.Oregon)
	e.put(b, "a", 10, time.Minute)
	e.Fakes[objectstorage.Oregon].SetDown(true)
	if err := e.Sweeper.Pass(context.Background()); err == nil || !strings.Contains(err.Error(), "1 of 1 workspaces failed") {
		t.Fatalf("Pass with Wasabi down = %v, want the failure reported", err)
	}
	if !e.logged(e.WS) {
		t.Fatal("the failing workspace was not logged")
	}
	e.Fakes[objectstorage.Oregon].SetDown(false)
	if err := e.Sweeper.Pass(context.Background()); err != nil {
		t.Fatalf("Pass once Wasabi is back = %v", err)
	}
	e.wantUsage(b.ID, 10, 1)
}

func TestSweepStopsListingAWorkspaceFarPastTheObjectCap(t *testing.T) {
	e := newSweepEnv(t, "sweep-cap")
	e.Sweeper.MaxListed = 1000 // one page
	b := e.bucket("photos", objectstorage.Oregon)
	for i := range 1500 {
		e.put(b, fmt.Sprintf("k%04d", i), 1, time.Minute)
	}
	e.mustSweep()
	u := e.usage()[b.ID]
	if u.Objects != 1000 || u.Bytes != 1000 {
		t.Fatalf("a listing past the cap recorded %+v, want its first 1000 as a lower bound", u)
	}
	if !e.logged("lower bound") {
		t.Fatal("the truncated listing was not logged")
	}
}

func TestSweepOfADisabledServiceDoesNothing(t *testing.T) {
	e := newSweepEnv(t, "sweep-disabled")
	b := e.bucket("photos", objectstorage.Oregon)
	e.put(b, "a", 10, time.Minute)
	off := objectstorage.NewSweeper(objectstorage.NewService(e.PG, nil), scopedRepo{SweepRepository: e.PG, only: e.scoped})
	if err := off.Pass(context.Background()); err != nil {
		t.Fatalf("Pass without a credential = %v", err)
	}
	if rows := e.usage(); len(rows) != 0 {
		t.Fatalf("a sweep with no credential wrote usage: %+v", rows)
	}
}

// The meter, end to end. 1500 bytes held for ten sweeps of 100,000 seconds each
// is 1.5 GB-seconds. One sweep alone is 0.15, which rounds to nothing, so the
// 1 that arrives is the remainder each sweep carries into the next.
func TestSweepAccruesTheStorageShadowMeterFromTheObservedBytes(t *testing.T) {
	e := newSweepEnv(t, "sweep-meter")
	b := e.bucket("photos", objectstorage.Oregon)
	e.put(b, "a", 1500, time.Minute)
	e.mustSweep() // the first observation accrues nothing
	if got := gbSeconds(t, e); got != 0 {
		t.Fatalf("the first sweep accrued %d GB-seconds", got)
	}
	for range 10 {
		e.Now = e.Now.Add(100_000 * time.Second)
		e.mustSweep()
	}
	if got := gbSeconds(t, e); got != 1 {
		t.Fatalf("1500 bytes for 1,000,000 seconds = %d GB-seconds, want 1 (1.5 rounded down)", got)
	}
	var kinds, resources, services int
	if err := e.Pool.QueryRow(context.Background(), `SELECT count(DISTINCT kind), count(DISTINCT resource_kind), count(DISTINCT service_id)
		FROM usage_hourly WHERE workspace_id = $1 AND kind LIKE 'object_storage_%'`, e.WS).Scan(&kinds, &resources, &services); err != nil {
		t.Fatal(err)
	}
	if kinds != 1 || resources != 1 || services != 1 {
		t.Fatalf("the meter wrote %d kinds, %d resource kinds and %d service ids, want one of each", kinds, resources, services)
	}
}

func gbSeconds(t *testing.T, e *sweepEnv) int64 {
	t.Helper()
	var n int64
	err := e.Pool.QueryRow(context.Background(), `SELECT COALESCE(sum(quantity), 0)::bigint FROM usage_hourly
		WHERE workspace_id = $1 AND kind = 'object_storage_gb_seconds' AND resource_kind = 'object_storage' AND service_id = $1`, e.WS).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *sweepEnv) setLifecycle(b store.ObjectBucket, rules string) {
	e.t.Helper()
	if err := e.PG.SetObjectBucketLifecycle(context.Background(), e.WS, b.ID, json.RawMessage(rules)); err != nil {
		e.t.Fatal(err)
	}
}

// keysSpy counts the sweeps of ended access keys.
type keysSpy struct{ calls atomic.Int32 }

func (k *keysSpy) SweepEnded(context.Context) (int, error) { k.calls.Add(1); return 0, nil }

func TestPassAbortsStaleUploadsRunsAbortRulesAndSweepsEndedKeysOncePerPass(t *testing.T) {
	e := newSweepEnv(t, "sweep-mp")
	keys := &keysSpy{}
	e.Sweeper.Keys = keys
	ctx := context.Background()
	stale, ruled := e.bucket("stale", objectstorage.Oregon), e.bucket("ruled", objectstorage.Oregon)
	start := func(b store.ObjectBucket, key string) string {
		id, err := e.Svc.CreateMultipartUpload(ctx, e.WS, b, key, "s3k-test", objectstorage.ObjectHeaders{})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	idle, fresh := start(stale, "idle"), start(stale, "fresh")
	tmp, keep := start(ruled, "tmp/x"), start(ruled, "keep/y")
	rules := `[{"id":"r","enabled":true,"conditions":{"prefix":"tmp/"},"abortMultipartUploadsTransition":{"condition":{"type":"Age","maxAge":172800}}}]`
	if err := e.PG.SetObjectBucketLifecycle(ctx, e.WS, ruled.ID, json.RawMessage(rules)); err != nil {
		t.Fatal(err)
	}
	age := func(id, column, interval string) {
		if _, err := e.Pool.Exec(ctx, `UPDATE object_multipart_uploads SET `+column+` = now() - $2::interval WHERE id = $1`, id, interval); err != nil {
			t.Fatal(err)
		}
	}
	age(idle, "last_part_at", "8 days")
	age(tmp, "created_at", "3 days")
	age(keep, "created_at", "3 days")
	e.Now = time.Now()
	if err := e.Sweeper.Pass(ctx); err != nil {
		t.Fatalf("pass: %v", err)
	}
	open := func(id string) bool { _, err := e.PG.GetObjectMultipartUpload(ctx, e.WS, id); return err == nil }
	if open(idle) || open(tmp) {
		t.Errorf("an upload with no part for 8 days (open: %v) or one older than its rule (open: %v) survived the pass", open(idle), open(tmp))
	}
	if !open(fresh) || !open(keep) {
		t.Errorf("a fresh upload (open: %v) or one outside every rule (open: %v) was aborted", open(fresh), open(keep))
	}
	if n := e.Fakes[objectstorage.Oregon].OpenUploads(); n != 2 {
		t.Errorf("Wasabi holds %d uploads after the pass, want the 2 that stay", n)
	}
	if n := keys.calls.Load(); n != 1 {
		t.Errorf("the pass swept ended keys %d times, want once", n)
	}
	if err := e.Sweeper.Pass(ctx); err != nil || keys.calls.Load() != 2 {
		t.Errorf("the next pass: %v, %d sweeps of keys, want 2", err, keys.calls.Load())
	}
}
