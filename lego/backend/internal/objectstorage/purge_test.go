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
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A workspace with buckets, objects and a sweep behind it, in both regions,
// beside another workspace's objects in the same Wasabi buckets.
type purgeFixture struct {
	*sweepEnv
	photos, east store.ObjectBucket
	other        string
	otherKey     string
}

func newPurgeFixture(t *testing.T, name string) *purgeFixture {
	t.Helper()
	e := newSweepEnv(t, name)
	f := &purgeFixture{sweepEnv: e}
	f.photos = e.bucket("photos", objectstorage.Oregon)
	f.east = e.bucket("east", objectstorage.Frankfurt)
	e.put(f.photos, "a.jpg", 10, time.Minute)
	e.put(f.photos, "dir/b.jpg", 20, time.Minute)
	e.put(f.east, "c.jpg", 30, time.Minute)
	// An orphan and a stray key are under the workspace's prefix, so they go too.
	e.putAt(e.WS, ids.New(ids.ObjectBucket), objectstorage.Oregon, "orphan", 5, time.Minute)
	e.putRaw(objectstorage.Frankfurt, e.WS+"/stray", 5, time.Minute)
	f.other = ids.New(ids.Workspace)
	f.otherKey = e.putAt(f.other, ids.New(ids.ObjectBucket), objectstorage.Oregon, "keep", 7, time.Minute)
	e.mustSweep() // usage rows exist, so there is something for the purge to drop
	return f
}

func (f *purgeFixture) objectsLeft() (own, others int) {
	for _, region := range objectstorage.Regions {
		fake := f.Fakes[region]
		own += len(fake.Keys(region.Bucket(), f.WS+"/"))
		others += len(fake.Keys(region.Bucket(), f.other+"/"))
	}
	return own, others
}

// rows counts the workspace's bucket rows (retired or not) and usage rows.
func (f *purgeFixture) rows() (buckets, retired, usage int) {
	f.t.Helper()
	err := f.Pool.QueryRow(context.Background(), `SELECT
		(SELECT count(*) FROM object_buckets WHERE workspace_id = $1),
		(SELECT count(*) FROM object_buckets WHERE workspace_id = $1 AND deleted_at IS NOT NULL),
		(SELECT count(*) FROM object_storage_usage WHERE workspace_id = $1)`, f.WS).Scan(&buckets, &retired, &usage)
	if err != nil {
		f.t.Fatal(err)
	}
	return buckets, retired, usage
}

func (f *purgeFixture) purger() *objectstorage.WorkspacePurger {
	return &objectstorage.WorkspacePurger{Service: f.Svc, Repo: f.PG}
}

// retiredAt is when the workspace's last bucket row was retired, by the
// database's clock, which stamped it.
func (f *purgeFixture) retiredAt() time.Time {
	f.t.Helper()
	var at time.Time
	if err := f.Pool.QueryRow(context.Background(),
		`SELECT max(deleted_at) FROM object_buckets WHERE workspace_id = $1`, f.WS).Scan(&at); err != nil {
		f.t.Fatal(err)
	}
	return at
}

// sweepAt runs a sweep pass whose clock reads at.
func (f *purgeFixture) sweepAt(at time.Time) {
	f.t.Helper()
	f.Now = at
	if err := f.Sweeper.Pass(context.Background()); err != nil {
		f.t.Fatalf("sweep at %s: %v", at, err)
	}
}

func TestWorkspaceDeletePurgesBothRegionsAndDropsTheRowsAfterTheHold(t *testing.T) {
	f := newPurgeFixture(t, "purge-delete")
	if own, _ := f.objectsLeft(); own != 5 {
		t.Fatalf("setup: %d objects under the workspace, want 5", own)
	}
	if b, _, u := f.rows(); b != 2 || u != 4 {
		t.Fatalf("setup: %d bucket rows and %d usage rows, want 2 and 4 (two buckets and an orphan marker in each region)", b, u)
	}

	// What the workspace teardown does: the tenant delete retires the rows, then
	// the post-cascade purger runs.
	ctx := context.Background()
	if err := f.PG.DeleteTenant(ctx, f.WS); err != nil {
		t.Fatal(err)
	}
	if _, retired, _ := f.rows(); retired != 2 {
		t.Fatalf("DeleteTenant left %d retired rows, want 2", retired)
	}
	for _, b := range []store.ObjectBucket{f.photos, f.east} {
		if _, err := f.PG.GetObjectBucketByID(ctx, b.ID); err == nil {
			t.Fatalf("bucket %s still resolves for a pub- host after the workspace was deleted", b.ID)
		}
	}
	if err := f.purger().PurgeWorkspace(ctx, f.WS); err != nil {
		t.Fatal(err)
	}

	own, others := f.objectsLeft()
	if own != 0 {
		t.Fatalf("%d objects remain under the deleted workspace's prefix, want none in either region", own)
	}
	if others != 1 {
		t.Fatalf("another workspace has %d objects left, want its 1 untouched", others)
	}
	// The rows stay for PurgeHold: they are how the sweep finds the prefix to
	// purge again while an upload authorized before the deletion can still land.
	if b, retired, u := f.rows(); b != 2 || retired != 2 || u != 4 {
		t.Fatalf("right after the purge %d bucket rows (%d retired) and %d usage rows remain, want 2 (2) and 4 held", b, retired, u)
	}
	deleted := f.retiredAt()
	f.sweepAt(deleted.Add(objectstorage.PurgeHold - time.Minute))
	if b, _, u := f.rows(); b != 2 || u != 4 {
		t.Fatalf("a sweep inside the hold left %d bucket rows and %d usage rows, want them all held", b, u)
	}
	f.sweepAt(deleted.Add(objectstorage.PurgeHold + time.Minute))
	if b, _, u := f.rows(); b != 0 || u != 0 {
		t.Fatalf("after the hold %d bucket rows and %d usage rows remain, want none", b, u)
	}
	if _, others := f.objectsLeft(); others != 1 {
		t.Fatalf("another workspace has %d objects left after the sweeps, want its 1 untouched", others)
	}
}

// A presigned PUT goes straight to Wasabi, which knows nothing of the deletion,
// so a URL minted before it can still store an object after the purge. Every
// sweep inside the hold removes such an object, and the sweep that drops the
// rows purges first.
func TestAPresignedPutThatLandsAfterThePurgeIsRemovedByALaterSweep(t *testing.T) {
	f := newPurgeFixture(t, "purge-late-put")
	ctx := context.Background()
	body := []byte("late bytes")
	var urls []string
	for _, key := range []string{"late/1", "late/2", "late/3"} {
		put, err := f.Svc.MintPut(ctx, f.WS, objectstorage.Oregon, key, int64(len(body)))
		if err != nil {
			t.Fatal(err)
		}
		urls = append(urls, put.URL)
	}
	if err := f.PG.DeleteTenant(ctx, f.WS); err != nil {
		t.Fatal(err)
	}
	deleted := f.retiredAt()
	if err := f.purger().PurgeWorkspace(ctx, f.WS); err != nil {
		t.Fatal(err)
	}
	if own, _ := f.objectsLeft(); own != 0 {
		t.Fatalf("%d objects remain right after the purge, want none", own)
	}
	land := func(url string) {
		t.Helper()
		upload(t, url, body)
		if own, _ := f.objectsLeft(); own != 1 {
			t.Fatalf("setup: the late upload left %d objects under the deleted workspace, want 1", own)
		}
	}

	// An upload lands after the purge: the next sweep removes it and keeps the rows.
	land(urls[0])
	f.sweepAt(deleted.Add(30 * time.Minute))
	if own, _ := f.objectsLeft(); own != 0 {
		t.Fatalf("a sweep inside the hold left %d late objects", own)
	}
	if b, _, _ := f.rows(); b == 0 {
		t.Fatal("a sweep inside the hold dropped the rows: nothing would find a later upload")
	}

	// One that lands late in the hold is removed too, and the rows still stay.
	land(urls[1])
	f.sweepAt(deleted.Add(objectstorage.PurgeHold - time.Minute))
	if own, _ := f.objectsLeft(); own != 0 {
		t.Fatalf("a sweep at the end of the hold left %d late objects", own)
	}
	if b, _, _ := f.rows(); b == 0 {
		t.Fatal("the rows were dropped before the hold ended")
	}

	// The sweep after the hold purges what landed before it, then drops the rows.
	land(urls[2])
	f.sweepAt(deleted.Add(objectstorage.PurgeHold + time.Minute))
	own, others := f.objectsLeft()
	if own != 0 || others != 1 {
		t.Fatalf("after the hold %d objects remain under the deleted workspace and %d under the other, want 0 and 1", own, others)
	}
	if b, _, u := f.rows(); b != 0 || u != 0 {
		t.Fatalf("after the hold %d bucket rows and %d usage rows remain, want none", b, u)
	}
}

// An upload slower than the hold lands after the rows that named the prefix are
// gone. The purge left its own record of the workspace, so a later sweep still
// looks under the prefix and removes it.
func TestAnUploadThatLandsAfterTheHoldIsRemovedByALaterSweep(t *testing.T) {
	f := newPurgeFixture(t, "purge-after-hold")
	ctx := context.Background()
	body := []byte("very late bytes")
	put, err := f.Svc.MintPut(ctx, f.WS, objectstorage.Frankfurt, "late/slow", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.PG.DeleteTenant(ctx, f.WS); err != nil {
		t.Fatal(err)
	}
	deleted := f.retiredAt()
	if err := f.purger().PurgeWorkspace(ctx, f.WS); err != nil {
		t.Fatal(err)
	}
	f.sweepAt(deleted.Add(objectstorage.PurgeHold + time.Minute))
	if b, _, u := f.rows(); b != 0 || u != 0 {
		t.Fatalf("setup: after the hold %d bucket rows and %d usage rows remain, want none", b, u)
	}

	upload(t, put.URL, body)
	if own, _ := f.objectsLeft(); own != 1 {
		t.Fatalf("setup: the upload left %d objects under the purged workspace, want 1", own)
	}
	f.sweepAt(deleted.Add(objectstorage.PurgeHold + time.Hour))
	own, others := f.objectsLeft()
	if own != 0 {
		t.Fatalf("a sweep after the hold left %d objects that arrived once the rows were gone", own)
	}
	if others != 1 {
		t.Fatalf("another workspace has %d objects left, want its 1 untouched", others)
	}
	if !f.logged("removed 1 objects that arrived after its purge") {
		t.Fatal("the sweep removed the late object without logging it")
	}
}

// The look under a purged prefix trusts only the purge's own record. A live
// workspace whose rows are missing (every bucket deleted, never observed) and a
// prefix no row names at all (a workspace being created, or one another
// database knows) both keep their objects, even when the record is there by
// mistake for the live one.
func TestTheLookUnderPurgedPrefixesLeavesLiveAndUnrecordedOnesAlone(t *testing.T) {
	e := newSweepEnv(t, "purge-not-recorded")
	ctx := context.Background()
	liveKey := e.putAt(e.WS, ids.New(ids.ObjectBucket), objectstorage.Oregon, "kept", 3, time.Minute)
	if _, err := e.Pool.Exec(ctx, `INSERT INTO object_storage_purged_workspaces (workspace_id, purged_at) VALUES ($1, now())`, e.WS); err != nil {
		t.Fatal(err)
	}
	unknown := ids.New(ids.Workspace)
	unknownKey := e.putAt(unknown, ids.New(ids.ObjectBucket), objectstorage.Frankfurt, "kept", 3, time.Minute)
	e.scoped[unknown] = true // the store, not the test's scope, is what leaves it alone
	t.Cleanup(func() {
		_, _ = e.Pool.Exec(context.Background(), `DELETE FROM object_storage_purged_workspaces WHERE workspace_id = $1`, unknown)
	})

	for range 2 {
		if err := e.Sweeper.Pass(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := e.Fakes[objectstorage.Oregon].Get(objectstorage.Oregon.Bucket(), liveKey); !ok {
		t.Fatal("the sweep deleted an object of a live workspace that has no rows")
	}
	if _, ok := e.Fakes[objectstorage.Frankfurt].Get(objectstorage.Frankfurt.Bucket(), unknownKey); !ok {
		t.Fatal("the sweep deleted an object under a prefix no row names")
	}
	// Neither prefix was even listed: no row of the sweep's names either one.
	for _, region := range objectstorage.Regions {
		for _, r := range e.Fakes[region].Requests() {
			if p := r.Query.Get("prefix"); r.Method == http.MethodGet && (strings.HasPrefix(p, e.WS+"/") || strings.HasPrefix(p, unknown+"/")) {
				t.Fatalf("the sweep listed %s in %s", p, region)
			}
		}
	}
}

// One pass looks under at most PurgedChecks purged prefixes in each region, and
// the next pass starts with the ones it did not reach.
func TestTheLookUnderPurgedPrefixesIsBoundedPerPassAndResumes(t *testing.T) {
	e := newSweepEnv(t, "purge-bounded")
	e.Sweeper.PurgedChecks = 2
	ctx := context.Background()
	purged := map[string]bool{}
	for range 3 {
		ws := ids.New(ids.Workspace)
		purged[ws] = true
		e.scoped[ws] = true
		if _, err := e.Pool.Exec(ctx, `INSERT INTO object_storage_purged_workspaces (workspace_id, purged_at) VALUES ($1, now())`, ws); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = e.Pool.Exec(context.Background(), `DELETE FROM object_storage_purged_workspaces WHERE workspace_id = $1`, ws)
		})
		for i := range 3 {
			e.putAt(ws, ids.New(ids.ObjectBucket), objectstorage.Oregon, fmt.Sprintf("late/%d", i), 4, time.Minute)
		}
	}
	left := func() (prefixes int) {
		for ws := range purged {
			if len(e.Fakes[objectstorage.Oregon].Keys(objectstorage.Oregon.Bucket(), ws+"/")) > 0 {
				prefixes++
			}
		}
		return prefixes
	}
	// looked counts the purged prefixes each region was listed under.
	looked := func() map[objectstorage.Region]int {
		out := map[objectstorage.Region]int{}
		for _, region := range objectstorage.Regions {
			seen := map[string]bool{}
			for _, r := range e.Fakes[region].Requests() {
				if ws, _, ok := strings.Cut(r.Query.Get("prefix"), "/"); ok && purged[ws] && !seen[ws] {
					seen[ws] = true
					out[region]++
				}
			}
		}
		return out
	}

	if err := e.Sweeper.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	if got := left(); got != 1 {
		t.Fatalf("after one pass with PurgedChecks 2, %d of the 3 purged prefixes still hold objects, want 1", got)
	}
	if got := looked(); got[objectstorage.Oregon] != 2 || got[objectstorage.Frankfurt] != 2 {
		t.Fatalf("one pass listed %v purged prefixes, want 2 in each region", got)
	}
	e.Now = e.Now.Add(time.Hour)
	if err := e.Sweeper.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	if got := left(); got != 0 {
		t.Fatalf("the next pass left %d purged prefixes holding objects, want the one the first did not reach emptied", got)
	}
}

func TestThePurgeHoldOutlastsThePresignedURLsAndTheirSlowestUpload(t *testing.T) {
	// A URL is valid for PresignTTL, and the largest object it allows takes about
	// eight hours at 1.5 Mbit/s once Wasabi has accepted the request.
	slowest := time.Duration(objectstorage.MaxObjectBytesPresigned*8/1_500_000) * time.Second
	if objectstorage.PurgeHold < objectstorage.PresignTTL+slowest {
		t.Fatalf("PurgeHold = %s, shorter than a URL's life (%s) plus a 5 GiB upload at 1.5 Mbit/s (%s)",
			objectstorage.PurgeHold, objectstorage.PresignTTL, slowest)
	}
}

func TestPurgeThatFailedOnceIsRetriedBySweepUntilItFinishes(t *testing.T) {
	f := newPurgeFixture(t, "purge-retry")
	ctx := context.Background()
	if err := f.PG.DeleteTenant(ctx, f.WS); err != nil {
		t.Fatal(err)
	}

	// Oregon is down at deletion time. The deletion is not failed...
	f.Fakes[objectstorage.Oregon].SetDown(true)
	if err := f.purger().PurgeWorkspace(ctx, f.WS); err != nil {
		t.Fatalf("a Wasabi outage failed the workspace deletion: %v", err)
	}
	// ...frankfurt, which was reachable, is purged, and the rows stay as the
	// record of what is still owed, stamped with the attempt.
	if own := len(f.Fakes[objectstorage.Frankfurt].Keys(objectstorage.Frankfurt.Bucket(), f.WS+"/")); own != 0 {
		t.Fatalf("frankfurt still holds %d objects although it was up", own)
	}
	if b, retired, _ := f.rows(); b != 2 || retired != 2 {
		t.Fatalf("%d rows, %d retired after the failed purge, want both kept and retired", b, retired)
	}
	var stamped int
	if err := f.Pool.QueryRow(ctx, `SELECT count(*) FROM object_buckets WHERE workspace_id = $1 AND purge_attempted_at IS NOT NULL`, f.WS).Scan(&stamped); err != nil || stamped != 2 {
		t.Fatalf("%d rows carry purge_attempted_at, %v; want both stamped", stamped, err)
	}

	// The hourly sweep retries, and keeps failing while Wasabi is down...
	if err := f.Sweeper.Pass(ctx); err == nil {
		t.Fatal("the sweep reported success while oregon was down")
	}
	if b, _, _ := f.rows(); b != 2 {
		t.Fatalf("the rows were dropped (%d left) before the purge finished", b)
	}
	if own, _ := f.objectsLeft(); own == 0 {
		t.Fatal("oregon's objects vanished while it was down")
	}

	// ...until it succeeds, and drops the rows once the hold is over.
	f.Fakes[objectstorage.Oregon].SetDown(false)
	f.Now = f.retiredAt().Add(objectstorage.PurgeHold + time.Minute)
	if err := f.Sweeper.Pass(ctx); err != nil {
		t.Fatalf("the retry after Wasabi came back failed: %v", err)
	}
	own, others := f.objectsLeft()
	if own != 0 || others != 1 {
		t.Fatalf("after the retry %d objects remain under the workspace and %d under the other, want 0 and 1", own, others)
	}
	if b, _, u := f.rows(); b != 0 || u != 0 {
		t.Fatalf("after the retry %d bucket rows and %d usage rows remain, want none", b, u)
	}
}

func TestSweepPurgesAWorkspaceWhoseTenantRowIsGoneWithoutRetiredRows(t *testing.T) {
	f := newPurgeFixture(t, "purge-ghost")
	ctx := context.Background()
	// The tenant row vanishes without DeleteTenant having retired the buckets: a
	// bucket created while the workspace was being deleted, or a delete from
	// before this code shipped.
	if _, err := f.Pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, f.WS); err != nil {
		t.Fatal(err)
	}
	if _, err := f.PG.GetObjectBucketByID(ctx, f.photos.ID); err != nil {
		t.Fatalf("setup: %v", err)
	}
	f.Now = time.Now()
	if err := f.Sweeper.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	if own, _ := f.objectsLeft(); own != 0 {
		t.Fatalf("%d objects of a deleted workspace remain", own)
	}
	// The sweep retired the rows itself, so the hold starts there.
	if b, retired, _ := f.rows(); b != 2 || retired != 2 {
		t.Fatalf("the sweep left %d rows, %d retired, want both retired and held", b, retired)
	}
	f.sweepAt(f.retiredAt().Add(objectstorage.PurgeHold + time.Minute))
	if b, _, u := f.rows(); b != 0 || u != 0 {
		t.Fatalf("%d bucket rows and %d usage rows of a deleted workspace remain after the hold", b, u)
	}
}

func TestPurgeBoundsItselfAndLeavesTheRestToTheSweep(t *testing.T) {
	f := newPurgeFixture(t, "purge-timeout")
	ctx := context.Background()
	if err := f.PG.DeleteTenant(ctx, f.WS); err != nil {
		t.Fatal(err)
	}
	// A purge that has no time at all is the request path giving up early: the
	// deletion still succeeds and the rows wait for the sweep.
	p := &objectstorage.WorkspacePurger{Service: f.Svc, Repo: f.PG, Timeout: time.Nanosecond}
	if err := p.PurgeWorkspace(ctx, f.WS); err != nil {
		t.Fatalf("a purge that ran out of time failed the deletion: %v", err)
	}
	if b, _, _ := f.rows(); b != 2 {
		t.Fatalf("%d rows after a purge with no time, want both kept", b)
	}
	f.Now = f.retiredAt().Add(objectstorage.PurgeHold + time.Minute)
	if err := f.Sweeper.Pass(ctx); err != nil {
		t.Fatal(err)
	}
	if b, _, _ := f.rows(); b != 0 {
		t.Fatalf("%d rows after the sweep finished the purge, want none", b)
	}
}

func TestPurgeIsANoOpWithoutACredential(t *testing.T) {
	e := newSweepEnv(t, "purge-off")
	b := e.bucket("photos", objectstorage.Oregon)
	e.put(b, "a", 1, time.Minute)
	ctx := context.Background()
	if err := e.PG.DeleteTenant(ctx, e.WS); err != nil {
		t.Fatal(err)
	}
	var nilPurger *objectstorage.WorkspacePurger
	for name, p := range map[string]*objectstorage.WorkspacePurger{
		"no service": {Repo: e.PG},
		"no stores":  {Service: objectstorage.NewService(e.PG, nil), Repo: e.PG},
		"nil":        nilPurger,
	} {
		if err := p.PurgeWorkspace(ctx, e.WS); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Nothing was touched: the rows wait, retired, for a sweep with a credential.
	if got := len(e.Fakes[objectstorage.Oregon].Keys(objectstorage.Oregon.Bucket(), e.WS+"/")); got != 1 {
		t.Fatalf("%d objects left, want the 1 untouched", got)
	}
	if _, err := e.PG.GetObjectBucketByID(ctx, b.ID); err == nil {
		t.Fatal("the bucket resolves although its workspace was deleted")
	}
}
