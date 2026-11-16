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

package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The hourly sweep (ADR112 D8). Every hour, for each workspace that has bucket
// rows, it lists the workspace's prefix, <tea-id>/, in each region whose last
// observation is older than its gap (a small region every pass, a large one every
// few hours at most every 72: Wasabi counts what a listing reads as egress,
// sweepbudget.go), and from that listing
//
//   - attributes bytes and objects to a bucket by the second path segment and
//     records them in object_storage_usage, which the quota gate, `bucket info`
//     and the object_storage_gb_seconds shadow meter read;
//   - deletes the objects a bucket's lifecycle rules expire (D5);
//
// and, for a deleted workspace, purges the prefix in both regions and drops its
// rows (D9, purge.go). The Wasabi bucket is shared by every workspace, so it is
// only ever listed by one workspace's prefix.

const (
	// SweepInterval is how often every replica schedules the sweep.
	SweepInterval = time.Hour

	// MaxLifecycleDeletesPerRule bounds the objects one rule deletes in one
	// sweep, so a sweep stays inside its hour (ADR112 D5).
	MaxLifecycleDeletesPerRule = 10_000

	// MaxSweepListedObjects stops a listing that has run far past the object cap
	// (MaxWorkspaceObjects): the write gate refuses a workspace that big whatever
	// the exact count, and an unbounded listing would not finish inside the hour.
	MaxSweepListedObjects = 2 * MaxWorkspaceObjects

	// sweepMinGap is how recent a region's usage must be for the sweep to skip
	// listing it, for a region of up to SweepHourlyObjects objects; a larger one
	// waits longer (regionGap). Every replica schedules the sweep and one runs it
	// at a time, so without this a second replica's tick, half an hour after the
	// first replica's, would list everything again.
	sweepMinGap = 45 * time.Minute

	// sweepRegionTimeout bounds the listing of one workspace in one region.
	sweepRegionTimeout = 30 * time.Minute

	// sweepPassTimeout bounds a whole pass; what it did not reach is picked up by
	// the next one, which starts with it (sweepOrder).
	sweepPassTimeout = 50 * time.Minute

	// sweepWorkers is how many workspaces one pass sweeps at once (ADR112 D8).
	// One workspace's listing stays sequential, so this is also the most list
	// requests in flight to Wasabi: the sweep is a client of a vendor that
	// publishes no limit for S3 requests and shares the account with the
	// platform's backups (ADR112 D1), so it takes a few slots, not a burst. The
	// production measurements behind 4 are in ADR112 D8, and when to raise it in
	// docs/runbooks/object-storage-sweep.md.
	sweepWorkers = 4

	// sweepLockName is the advisory lock that keeps the pass on one replica.
	sweepLockName = "object-storage-sweep"
)

// OrphanBucketID is the usage key under which the sweep records the objects of a
// workspace in a region that belong to no live bucket: an object whose obk- id
// has no row (a bucket deleted while a put was in flight) or a key that is not
// <tea-id>/<obk-id>/<key> at all. They occupy Wasabi, so they count towards the
// workspace's quota and its storage meter, and the purge of a deleted workspace
// removes them with the rest of its prefix. The key is not an obk- id, so it can
// never collide with a bucket's, and is unique per workspace and region because
// object_storage_usage is keyed by bucket_id alone.
func OrphanBucketID(workspaceID string, region Region) string {
	return orphanPrefix + workspaceID + ":" + string(region)
}

const orphanPrefix = "orphan:"

// IsOrphanBucketID reports whether a usage row's bucket_id is an orphan marker
// rather than a bucket.
func IsOrphanBucketID(bucketID string) bool { return strings.HasPrefix(bucketID, orphanPrefix) }

// PurgeRepository is the part of the store a purge reads and writes.
// *store.PGStore implements it.
type PurgeRepository interface {
	RetireObjectBuckets(ctx context.Context, workspaceID string) (int64, error)
	MarkObjectStoragePurgeAttempt(ctx context.Context, workspaceID string, at time.Time) error
	FinishObjectStoragePurge(ctx context.Context, workspaceID string) error
}

// SweepRepository is the part of the store the sweep reads and writes.
// *store.PGStore implements it.
type SweepRepository interface {
	PurgeRepository
	ListObjectStorageWorkspaces(ctx context.Context) ([]store.ObjectStorageWorkspace, error)
	ListObjectBuckets(ctx context.Context, workspaceID string) ([]store.ObjectBucket, error)
	ListObjectStorageUsage(ctx context.Context, workspaceID string) ([]store.ObjectStorageUsage, error)
	RecordObjectStorageObservation(ctx context.Context, workspaceID, region string, observed []store.ObjectStorageUsage, at time.Time) error
}

// Exclusive runs a function on one replica at a time. *store.PGStore does,
// with a Postgres advisory lock (billing's sweeps use it the same way).
type Exclusive interface {
	RunExclusive(ctx context.Context, name string, fn func(context.Context) error) (ran bool, err error)
}

// Sweeper is the hourly object storage sweep. NewSweeper sets production
// values; tests replace the exported fields.
type Sweeper struct {
	svc  *Service
	repo SweepRepository

	// Exclusive keeps Run's pass on one replica. Nil runs every tick.
	Exclusive Exclusive
	// LifecycleCap is the most objects one rule deletes in one pass.
	LifecycleCap int
	// MinGap is how recent a region's usage must be for a pass to leave it alone,
	// when the region held up to HourlyObjects objects. Zero sweeps every region
	// every pass.
	MinGap time.Duration
	// HourlyObjects and MaxGap scale MinGap with the region's size: one more hour
	// for each HourlyObjects objects its last observation counted, at most MaxGap
	// (sweepbudget.go). Zero turns the scaling or the ceiling off.
	HourlyObjects int64
	MaxGap        time.Duration
	// RegionTimeout bounds one workspace in one region; PassTimeout the pass.
	RegionTimeout, PassTimeout time.Duration
	// Workers is how many workspaces a pass sweeps at once; below one it is one.
	Workers int
	// Metrics records each pass and workspace; nil records nothing.
	Metrics *SweepMetrics
	// MaxListed stops a listing after about this many objects.
	MaxListed int
	Now       func() time.Time
	Logf      func(format string, args ...any)

	// listedBytes is the estimated size of every listing response this sweeper
	// has read (listResponseBytes), so a pass can log what it cost.
	listedBytes atomic.Int64
}

// NewSweeper returns the sweep over svc's regions. A service with no credential
// has no regions, and its passes do nothing.
func NewSweeper(svc *Service, repo SweepRepository) *Sweeper {
	return &Sweeper{
		svc: svc, repo: repo,
		LifecycleCap: MaxLifecycleDeletesPerRule, MinGap: sweepMinGap,
		HourlyObjects: SweepHourlyObjects, MaxGap: SweepMaxInterval - (SweepInterval - sweepMinGap),
		RegionTimeout: sweepRegionTimeout, PassTimeout: sweepPassTimeout, Workers: sweepWorkers, MaxListed: int(MaxSweepListedObjects),
		Now: time.Now, Logf: log.Printf,
	}
}

// Run schedules a pass every interval until ctx ends, the first at once. Every
// replica runs it and Exclusive lets one pass at a time through (the attachment
// sweep, which beside it purges Wasabi objects, claims rows with SKIP LOCKED
// instead, but a pass over a prefix listing has no row to claim).
//
// A pass never outlives its interval: its context ends at the interval, or at
// PassTimeout when that is sooner. core.Poll finishes a pass before it reads the
// ticker, which holds one tick, so a tick that falls inside a pass is skipped,
// never queued, and passes do not overlap on a replica; the advisory lock does
// the same across replicas.
func (sw *Sweeper) Run(ctx context.Context, interval time.Duration) {
	core.Poll(ctx, "object storage sweep", interval, func(ctx context.Context) error {
		if interval > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, interval)
			defer cancel()
		}
		if sw.Exclusive == nil {
			return sw.Pass(ctx)
		}
		_, err := sw.Exclusive.RunExclusive(ctx, sweepLockName, sw.Pass)
		return err
	})
}

// Pass sweeps every workspace that has object storage rows: a live workspace's
// usage and lifecycle, a deleted workspace's purge. Up to Workers workspaces are
// swept at once, so a workspace with millions of objects does not hold the ones
// behind it; each workspace's own listing stays sequential. One workspace
// failing, or one of its regions, does not stop the others; it is logged, and the
// pass reports how many failed.
//
// A pass that reaches PassTimeout (or its caller's deadline) stops starting
// workspaces, abandons the ones in flight (their listings are never recorded
// half-done) and returns an error that says how many it did not reach. The next
// pass starts with those: sweepOrder puts the least recently observed first.
func (sw *Sweeper) Pass(ctx context.Context) error {
	if !sw.svc.Enabled() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, sw.PassTimeout)
	defer cancel()
	started := time.Now()
	listedBefore := sw.listedBytes.Load()
	workspaces, err := sw.repo.ListObjectStorageWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}
	sweepOrder(workspaces)

	// Each worker writes its own element; the zero value is "not reached".
	outcomes := make([]workspaceOutcome, len(workspaces))
	workers := max(sw.Workers, 1)
	var group errgroup.Group
	group.SetLimit(workers)
	for i, w := range workspaces {
		if ctx.Err() != nil {
			break
		}
		group.Go(func() error {
			// Go blocks while every worker is busy, so the deadline may have
			// passed by the time this one has a place.
			if ctx.Err() == nil {
				outcomes[i] = sw.sweepOne(ctx, w)
			}
			return nil
		})
	}
	_ = group.Wait() // the workers return no error: sweepOne logs and counts

	var tally passTally
	for _, o := range outcomes {
		tally.add(o)
	}
	took := time.Since(started)
	sw.Logf("object storage sweep: pass over %d workspaces took %s with %d workers: %d swept, %d fresh, %d failed, %d not reached; listings read about %d bytes",
		len(workspaces), took.Round(time.Millisecond), workers, tally.swept, tally.fresh, tally.failed, tally.notReached, sw.listedBytes.Load()-listedBefore)
	sw.Metrics.observePass(took, tally)
	switch {
	case tally.notReached > 0:
		return fmt.Errorf("pass cut off: %d of %d workspaces not reached, the next pass starts with them (%d failed): %w",
			tally.notReached, len(workspaces), tally.failed, ctx.Err())
	case tally.failed > 0:
		return fmt.Errorf("%d of %d workspaces failed", tally.failed, len(workspaces))
	}
	return nil
}

// sweepOrder puts deleted workspaces first, in the order the store lists them
// (the purge attempted longest ago leads), then live ones least recently
// observed first, a workspace the sweep never recorded before any that it did.
// A pass cut off at its deadline leaves the workspaces it did not reach with the
// oldest observations, so the next pass starts with exactly those instead of
// sweeping the head of the list again, which an id order would.
func sweepOrder(workspaces []store.ObjectStorageWorkspace) {
	slices.SortStableFunc(workspaces, func(a, b store.ObjectStorageWorkspace) int {
		switch {
		case a.Retired != b.Retired:
			if a.Retired {
				return -1
			}
			return 1
		case a.Retired:
			return 0
		}
		return a.LastObserved.Compare(b.LastObserved)
	})
}

// workspaceOutcome is what one workspace's turn in a pass came to. The zero
// value is a workspace the pass never finished: it ended first, and the next
// pass takes it.
type workspaceOutcome int

const (
	outcomeNotReached workspaceOutcome = iota
	outcomeSwept                       // listed, or purged
	outcomeFresh                       // every region was observed within MinGap
	outcomeFailed                      // an error, a panic, or a purge that did not finish
)

// passTally counts a pass's workspaces by outcome.
type passTally struct{ swept, fresh, failed, notReached int }

func (t *passTally) add(o workspaceOutcome) {
	switch o {
	case outcomeSwept:
		t.swept++
	case outcomeFresh:
		t.fresh++
	case outcomeFailed:
		t.failed++
	case outcomeNotReached:
		t.notReached++
	}
}

// sweepOne sweeps or purges one workspace and records what it cost. A panic
// fails this workspace only: Pass runs it on a worker goroutine, where core.Poll's
// recover does not reach, and one malformed row must not take bex-api down.
func (sw *Sweeper) sweepOne(ctx context.Context, w store.ObjectStorageWorkspace) (outcome workspaceOutcome) {
	started := time.Now()
	defer func() {
		if p := recover(); p != nil {
			sw.Logf("object storage sweep: workspace %s: recovered panic: %v\n%s", w.ID, p, debug.Stack())
			outcome = outcomeFailed
		}
	}()
	listed := true
	var err error
	if w.Retired {
		// One large purge must not eat the pass and starve the live sweeps
		// behind it; what it did not finish is retried next hour.
		purgeCtx, cancelPurge := context.WithTimeout(ctx, sw.RegionTimeout)
		err = sw.PurgeWorkspace(purgeCtx, w.ID)
		cancelPurge()
	} else {
		listed, err = sw.SweepWorkspace(ctx, w.ID)
	}
	switch {
	case err != nil && ctx.Err() != nil:
		return outcomeNotReached
	case err == nil && !listed:
		return outcomeFresh
	}
	sw.Metrics.observeWorkspace(time.Since(started))
	if err != nil {
		sw.Logf("object storage sweep: workspace %s: %v", w.ID, err)
		return outcomeFailed
	}
	return outcomeSwept
}

// PurgeWorkspace purges a deleted workspace: every object under <tea-id>/ in
// every region, then its bucket and usage rows. A region that cannot be reached
// fails the purge and leaves the rows, stamped, for the next pass.
func (sw *Sweeper) PurgeWorkspace(ctx context.Context, workspaceID string) error {
	_, err := purgeWorkspace(ctx, sw.svc, sw.repo, workspaceID, sw.Now())
	return err
}

// SweepWorkspace sweeps one live workspace, region by region, and reports
// whether it listed any region. A region that fails keeps its old usage rows and
// does not stop the other. A region whose usage is newer than its gap (MinGap
// for a small one, longer for one that held many objects: regionGap) was swept
// by another replica or not long enough ago, and is left alone. Wasabi counts
// what a listing reads as egress, so the wait is what keeps a large region's
// listings inside the account's free allowance (sweepbudget.go).
func (sw *Sweeper) SweepWorkspace(ctx context.Context, workspaceID string) (listed bool, err error) {
	buckets, err := sw.repo.ListObjectBuckets(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("list buckets: %w", err)
	}
	usage, err := sw.repo.ListObjectStorageUsage(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("list usage: %w", err)
	}
	var errs []error
	for _, region := range Regions {
		st, ok := sw.svc.stores[region]
		if !ok {
			continue
		}
		var inRegion []store.ObjectBucket
		for _, b := range buckets {
			if b.Region == string(region) {
				inRegion = append(inRegion, b)
			}
		}
		lastSeen, objects, hasUsage := regionObservation(usage, region)
		if len(inRegion) == 0 && !hasUsage {
			continue // nothing of this workspace was ever recorded here
		}
		if hasUsage && sw.Now().Sub(lastSeen) < sw.regionGap(objects) {
			continue
		}
		listed = true
		if err := sw.sweepRegion(ctx, workspaceID, region, st, inRegion); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", region, err))
		}
	}
	return listed, errors.Join(errs...)
}

// regionObservation is what the usage rows of a region say: when it was last
// observed (the newest row) and how many objects the live buckets and the
// orphans held, which is what a listing of it will read.
func regionObservation(usage []store.ObjectStorageUsage, region Region) (at time.Time, objects int64, ok bool) {
	for _, u := range usage {
		if u.Region != string(region) {
			continue
		}
		objects += u.Objects
		if u.ObservedAt.After(at) {
			at, ok = u.ObservedAt, true
		}
	}
	return at, objects, ok
}

// bucketTally is what the sweep counted under one usage key.
type bucketTally struct{ bytes, objects int64 }

// sweepRegion lists the workspace's prefix in one region, page by page. Each
// page's objects are attributed to a bucket, and those a lifecycle rule expires
// are deleted before the next page is read, so an object is deleted moments after
// it was listed, not minutes. Only a listing that finished is recorded; a failure
// leaves the previous observation in place.
func (sw *Sweeper) sweepRegion(ctx context.Context, workspaceID string, region Region, st Store, buckets []store.ObjectBucket) error {
	ctx, cancel := context.WithTimeout(ctx, sw.RegionTimeout)
	defer cancel()
	now := sw.Now()
	type bucketRules struct {
		tally bucketTally
		rules *sweepLifecycle
	}
	live := make(map[string]*bucketRules, len(buckets))
	for _, b := range buckets {
		live[b.ID] = &bucketRules{rules: newSweepLifecycle(b.ID, b.Lifecycle, sw.Logf)}
	}
	var orphans bucketTally
	deleting := true // cleared by the first failed delete; the listing goes on
	prefix, after, listed := WorkspacePrefix(workspaceID), "", 0
	for {
		page, err := st.List(ctx, prefix, after, ListMaxLimit)
		if err != nil {
			return fmt.Errorf("list %s: %w", prefix, err)
		}
		read := listResponseBytes(prefix, page.Objects)
		sw.listedBytes.Add(read)
		sw.Metrics.observeListed(region, read)
		var expired []ObjectInfo
		var expiredOwner []*bucketTally
		for _, o := range page.Objects {
			_, bucketID, key, ok := ParseObjectKey(o.Key)
			b := live[bucketID]
			switch {
			case !ok || b == nil:
				orphans.bytes, orphans.objects = orphans.bytes+o.Size, orphans.objects+1
			case deleting && b.rules.claim(key, o.LastModified, now, sw.LifecycleCap):
				expired, expiredOwner = append(expired, o), append(expiredOwner, &b.tally)
			default:
				b.tally.bytes, b.tally.objects = b.tally.bytes+o.Size, b.tally.objects+1
			}
		}
		if len(expired) > 0 {
			kept, err := deleteObjects(ctx, st, expired)
			if err != nil {
				deleting = false
				sw.Logf("object storage sweep: workspace %s in %s: lifecycle delete failed, the rest wait for the next sweep: %v", workspaceID, region, err)
			}
			// An object that is still there is still stored.
			for i, o := range expired {
				if kept[o.Key] {
					expiredOwner[i].bytes, expiredOwner[i].objects = expiredOwner[i].bytes+o.Size, expiredOwner[i].objects+1
				}
			}
		}
		listed += len(page.Objects)
		if page.Next == "" {
			break
		}
		if listed >= sw.MaxListed {
			sw.Logf("object storage sweep: workspace %s in %s: stopped listing after %d objects, far past the cap of %d; the counts are a lower bound",
				workspaceID, region, listed, MaxWorkspaceObjects)
			break
		}
		after = page.Next
	}

	observed := make([]store.ObjectStorageUsage, 0, len(live)+1)
	for _, b := range buckets {
		t := live[b.ID].tally
		observed = append(observed, store.ObjectStorageUsage{BucketID: b.ID, Bytes: t.bytes, Objects: t.objects})
	}
	if orphans.objects > 0 {
		observed = append(observed, store.ObjectStorageUsage{BucketID: OrphanBucketID(workspaceID, region), Bytes: orphans.bytes, Objects: orphans.objects})
	}
	slices.SortFunc(observed, func(a, b store.ObjectStorageUsage) int { return strings.Compare(a.BucketID, b.BucketID) })
	for i := range observed {
		observed[i].WorkspaceID, observed[i].Region = workspaceID, string(region)
	}
	return sw.repo.RecordObjectStorageObservation(ctx, workspaceID, string(region), observed, sw.Now())
}

// deleteObjects deletes the listed objects in one request and returns the keys
// that are still there: all of them when the request failed, the ones S3 named
// when only some did.
func deleteObjects(ctx context.Context, st Store, objects []ObjectInfo) (kept map[string]bool, err error) {
	keys := make([]string, len(objects))
	for i, o := range objects {
		keys[i] = o.Key
	}
	failed, err := st.DeleteKeys(ctx, keys)
	kept = make(map[string]bool, len(failed))
	for _, k := range failed {
		kept[k] = true
	}
	return kept, err
}
