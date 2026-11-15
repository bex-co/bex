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

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w22/017: the orphan sweep deletes the images of workspaces that no longer
// exist. What it must never do is as much the contract as what it does, so each
// test below states both.

// fakeWorkspaces answers which of the asked workspace ids exist, and records
// what it was asked.
type fakeWorkspaces struct {
	mu     sync.Mutex
	exist  map[string]bool
	err    error
	asked  []string
	rounds int
}

func (f *fakeWorkspaces) ExistingWorkspaceIDs(_ context.Context, workspaceIDs []string) (map[string]bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rounds++
	f.asked = append(f.asked, workspaceIDs...)
	if f.err != nil {
		return nil, f.err
	}
	out := map[string]bool{}
	for _, id := range workspaceIDs {
		if f.exist[id] {
			out[id] = true
		}
	}
	return out, nil
}

func (f *fakeWorkspaces) askedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := slices.Clone(f.asked)
	sort.Strings(ids)
	return slices.Compact(ids)
}

type sweepClock struct{ now time.Time }

func (c *sweepClock) Now() time.Time { return c.now }

func (c *sweepClock) advance(d time.Duration) { c.now = c.now.Add(d) }

type sweepFixture struct {
	registry *memoryRegistry
	sweeper  *OrphanSweeper
	metrics  *StorageMetrics
	clock    *sweepClock
}

func newSweepFixture(t *testing.T, workspaces WorkspaceLookup) *sweepFixture {
	t.Helper()
	registry := newMemoryRegistry(t)
	metrics := NewStorageMetrics(prometheus.NewRegistry())
	clock := &sweepClock{now: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	return &sweepFixture{
		registry: registry, metrics: metrics, clock: clock,
		sweeper: &OrphanSweeper{Reaper: registry.reaper(), Workspaces: workspaces, Metrics: metrics, Now: clock.Now},
	}
}

func (f *sweepFixture) reaped() float64 { return testutil.ToFloat64(f.metrics.orphanReaped) }

func (f *sweepFixture) failures() float64 { return testutil.ToFloat64(f.metrics.orphanFailures) }

func ns(ws string) string { return "snapshots/" + ws + "-sandbox/" }

// The sweep deletes the snapshot tags of every repository under the namespace of
// a workspace that is gone, and nothing else: not a workspace that exists, not
// a tag the controller did not write, not a repository outside the snapshot
// prefix or under a namespace that is not a workspace's. The catalog is read in
// pages, so a namespace past the first page is found.
func TestOrphanSweepDeletesOnlyTheRepositoriesOfDeletedWorkspaces(t *testing.T) {
	gone, alsoGone, live := ids.New(ids.Workspace), ids.New(ids.Workspace), ids.New(ids.Workspace)
	workspaces := &fakeWorkspaces{exist: map[string]bool{live: true}}
	f := newSweepFixture(t, workspaces)
	f.sweeper.pageSize = 2

	orphans := map[string][]string{
		ns(gone) + "os-1-sandbox":                            {"snap-gen1", "snap-gen2", "keep-me"},
		ns(gone) + "images/sbb-aaaaaaaaaaaaaaaaaaaa":         {imageTag},
		ns(alsoGone) + "os-2-sandbox":                        {"snap-gen7"},
		ns(alsoGone) + "images/ari-bbbbbbbbbbbbbbbbbbbb":     {imageTag},
		ns(live) + "os-3-sandbox":                            nil, // filled below
		ns(live) + "images/ari-cccccccccccccccccccc":         nil,
		"other/" + gone + "-sandbox/os-1-sandbox":            {"snap-gen1"}, // another registry area
		"snapshots-extra/" + gone + "-sandbox/os-1-sandbox":  {"snap-gen1"}, // prefix is a path, not a string prefix
		gone + "-sandbox/os-1-sandbox":                       {"snap-gen1"},
		"mirror/snapshots/" + gone + "-sandbox/os-1-sandbox": {"snap-gen1"}, // the prefix deeper in the path
		"snapshots/" + gone + "/os-1-sandbox":                {"snap-gen1"}, // the hosting namespace, not the sandbox one
		"snapshots/not-a-workspace-sandbox/os-1-sandbox":     {"snap-gen1"},
		"snapshots/" + gone + "-sandbox":                     {"snap-gen1"}, // a repository named like the namespace itself
		"builds/" + gone + "/app":                            {"latest"},
	}
	orphans[ns(live)+"os-3-sandbox"] = []string{"snap-gen1", "snap-gen2"}
	orphans[ns(live)+"images/ari-cccccccccccccccccccc"] = []string{imageTag}
	for repo, tags := range orphans {
		f.registry.add(repo, tags...)
	}
	before := f.registry.everyTag()

	f.sweeper.Sweep(context.Background())

	after := f.registry.everyTag()
	deleted := map[string][]string{
		ns(gone) + "os-1-sandbox":                        {"keep-me"},
		ns(gone) + "images/sbb-aaaaaaaaaaaaaaaaaaaa":     nil,
		ns(alsoGone) + "os-2-sandbox":                    nil,
		ns(alsoGone) + "images/ari-bbbbbbbbbbbbbbbbbbbb": nil,
	}
	for repo, tags := range before {
		want := tags
		if left, isOrphan := deleted[repo]; isOrphan {
			want = left
		}
		if !slices.Equal(after[repo], want) {
			t.Errorf("%s = %v, want %v (before: %v)", repo, after[repo], want, tags)
		}
	}
	if got := f.reaped(); got != 4 {
		t.Errorf("orphan repositories reaped = %v, want 4", got)
	}
	want := []string{alsoGone, gone, live}
	sort.Strings(want)
	if got := workspaces.askedIDs(); !slices.Equal(got, want) {
		t.Errorf("asked about %v, want only the three well-formed workspace namespaces %v", got, want)
	}
	// Only the controller's and the importer's tags go, each by its own tag: never a
	// digest delete, which would take every tag on the manifest.
	for _, d := range f.registry.deletes() {
		tag := d[strings.LastIndex(d, ":")+1:]
		if tag != imageTag && !snapshotTag.MatchString(tag) {
			t.Errorf("unexpected delete %s", d)
		}
	}
	if got := f.failures(); got != 0 {
		t.Errorf("failures = %v", got)
	}
}

// The prefix is the configured one (the controller's --snapshot-registry path),
// not the default: a registry that keeps its images under another path is swept
// there, and the default path is then outside it.
func TestOrphanSweepFollowsTheConfiguredPrefix(t *testing.T) {
	gone := ids.New(ids.Workspace)
	f := newSweepFixture(t, &fakeWorkspaces{})
	config := f.registry.config()
	config.Prefix = "tenant/snaps"
	f.sweeper.Reaper = &RegistryReaper{RegistryConfig: config}
	inside, outside := "tenant/snaps/"+gone+"-sandbox/os-1-sandbox", ns(gone)+"os-1-sandbox"
	f.registry.add(inside, "snap-gen1")
	f.registry.add(outside, "snap-gen1")

	f.sweeper.Sweep(context.Background())

	if len(f.registry.tags(inside)) != 0 || len(f.registry.tags(outside)) != 1 {
		t.Errorf("inside the configured prefix %v, outside it %v", f.registry.tags(inside), f.registry.tags(outside))
	}
}

// A catalog the registry cannot serve, or a workspace lookup that fails, skips
// the round without deleting anything: absence of evidence is not a deleted
// workspace. The next round, once it is due, sweeps.
func TestOrphanSweepSkipsTheRoundOnACatalogOrLookupError(t *testing.T) {
	gone := ids.New(ids.Workspace)
	repo := ns(gone) + "os-1-sandbox"
	workspaces := &fakeWorkspaces{}
	f := newSweepFixture(t, workspaces)
	f.registry.add(repo, "snap-gen1")

	f.registry.setCatalogStatus(500)
	f.sweeper.Sweep(context.Background())
	if len(f.registry.deletes()) != 0 || workspaces.rounds != 0 {
		t.Fatalf("a failed catalog read deleted %v after %d workspace lookups", f.registry.deletes(), workspaces.rounds)
	}
	if f.failures() != 1 {
		t.Errorf("failures after a catalog error = %v, want 1", f.failures())
	}

	f.registry.setCatalogStatus(0)
	workspaces.err = errors.New("database down")
	f.clock.advance(4 * time.Hour)
	f.sweeper.Sweep(context.Background())
	if len(f.registry.deletes()) != 0 {
		t.Fatalf("a failed workspace lookup deleted %v: with the lookup down every workspace looks gone", f.registry.deletes())
	}
	if f.failures() != 2 {
		t.Errorf("failures after a lookup error = %v, want 2", f.failures())
	}

	workspaces.err = nil
	f.clock.advance(4 * time.Hour)
	f.sweeper.Sweep(context.Background())
	if got := f.registry.tags(repo); len(got) != 0 {
		t.Errorf("the next due round left %v", got)
	}
}

// A round deletes the images of at most maxRepositories repositories, so a
// backlog (the eight orphans of 2026-11-15, or a thousand) is cleared over
// several rounds and never in one burst of registry calls. The bound counts
// repositories that had something to delete: the emptied ones the catalog still
// lists do not use it up.
func TestOrphanSweepIsBoundedPerRound(t *testing.T) {
	gone := ids.New(ids.Workspace)
	f := newSweepFixture(t, &fakeWorkspaces{})
	f.sweeper.maxRepositories = 2
	repos := make([]string, 5)
	for i := range repos {
		repos[i] = fmt.Sprintf("%sos-%d-sandbox", ns(gone), i)
		f.registry.add(repos[i], "snap-gen1", "snap-gen2")
	}
	remaining := func() int {
		n := 0
		for _, repo := range repos {
			if len(f.registry.tags(repo)) > 0 {
				n++
			}
		}
		return n
	}

	for round, wantLeft := range []int{3, 1, 0, 0} {
		f.sweeper.Sweep(context.Background())
		if got := remaining(); got != wantLeft {
			t.Fatalf("round %d left %d repositories with images, want %d", round+1, got, wantLeft)
		}
		f.clock.advance(f.sweeper.interval())
	}
	if f.reaped() != 5 {
		t.Errorf("reaped = %v, want 5", f.reaped())
	}
}

// A round starts at most once per interval on a replica (three hours unless set),
// so the sweep adds a few registry calls an hour and no more, however often the
// loop ticks. It is safe on several replicas: each paces itself, and a tag the
// other deleted first is already gone, which is success.
func TestOrphanSweepIsRateLimited(t *testing.T) {
	f := newSweepFixture(t, &fakeWorkspaces{})
	sweep := func() int {
		f.sweeper.Sweep(context.Background())
		return f.registry.catalogRequests()
	}
	if got := sweep(); got != 1 {
		t.Fatalf("first sweep read the catalog %d times, want 1", got)
	}
	if got := sweep(); got != 1 {
		t.Fatalf("a second sweep at once read the catalog again (%d)", got)
	}
	f.clock.advance(3*time.Hour - time.Second)
	if got := sweep(); got != 1 {
		t.Fatalf("a sweep 3h short of a second read the catalog again (%d)", got)
	}
	f.clock.advance(time.Second)
	if got := sweep(); got != 2 {
		t.Fatalf("a sweep an interval after the last read the catalog %d times, want 2", got)
	}

	// Two replicas race for the same image: the loser finds the tag gone.
	gone := ids.New(ids.Workspace)
	repo := ns(gone) + "os-1-sandbox"
	f.registry.add(repo, "snap-gen1")
	other := &OrphanSweeper{Reaper: f.registry.reaper(), Workspaces: &fakeWorkspaces{}, Metrics: f.metrics, Now: f.clock.Now}
	f.clock.advance(3 * time.Hour)
	var wg sync.WaitGroup
	for _, s := range []*OrphanSweeper{f.sweeper, other} {
		wg.Add(1)
		go func() { defer wg.Done(); s.Sweep(context.Background()) }()
	}
	wg.Wait()
	if len(f.registry.tags(repo)) != 0 || f.failures() != 0 {
		t.Errorf("two replicas sweeping together: tags %v, failures %v", f.registry.tags(repo), f.failures())
	}
}

// A repository the registry refuses to delete from is counted and the others are
// still swept; a registry that refuses everything ends the round after three
// failures in a row, so a credential without the delete action is not hammered.
func TestOrphanSweepContinuesPastAFailedRepositoryAndStopsAfterThree(t *testing.T) {
	gone := ids.New(ids.Workspace)
	f := newSweepFixture(t, &fakeWorkspaces{})
	denied, fine := ns(gone)+"os-1-sandbox", ns(gone)+"os-2-sandbox"
	f.registry.add(denied, "snap-gen1")
	f.registry.add(fine, "snap-gen1")
	f.registry.deny(denied)
	f.sweeper.Sweep(context.Background())
	if len(f.registry.tags(fine)) != 0 || len(f.registry.tags(denied)) != 1 {
		t.Errorf("tags: refused %v, other %v", f.registry.tags(denied), f.registry.tags(fine))
	}
	if f.failures() != 1 || f.reaped() != 1 {
		t.Errorf("failures %v, reaped %v, want 1 and 1", f.failures(), f.reaped())
	}

	all := newSweepFixture(t, &fakeWorkspaces{})
	for i := range 8 {
		all.registry.add(fmt.Sprintf("%sos-%d-sandbox", ns(gone), i), "snap-gen1")
	}
	all.registry.setFailDeletes(true)
	all.sweeper.Sweep(context.Background())
	if got := all.registry.deleteAttempts(); got != 3 {
		t.Errorf("a registry refusing every delete was asked %d times in a round, want 3", got)
	}
	if all.failures() != 3 {
		t.Errorf("failures = %v, want 3", all.failures())
	}
}

// An unconfigured sweeper does nothing, and a nil one is a no-op: the sweep is
// opt-in with the reaper credential, like the reaper.
func TestOrphanSweepUnconfiguredIsANoOp(t *testing.T) {
	var none *OrphanSweeper
	none.Sweep(context.Background())
	registry := newMemoryRegistry(t)
	gone := ids.New(ids.Workspace)
	registry.add(ns(gone)+"os-1-sandbox", "snap-gen1")
	(&OrphanSweeper{Workspaces: &fakeWorkspaces{}}).Sweep(context.Background())
	(&OrphanSweeper{Reaper: registry.reaper()}).Sweep(context.Background())
	if registry.catalogRequests() != 0 || len(registry.deletes()) != 0 {
		t.Errorf("an unconfigured sweeper called the registry: %d catalog reads, deletes %v", registry.catalogRequests(), registry.deletes())
	}
}

// Run ticks and the sweep paces itself: one round however many ticks pass in
// the interval, and it returns when its context ends.
func TestOrphanSweepRunSweepsOnceAndStopsWithItsContext(t *testing.T) {
	gone := ids.New(ids.Workspace)
	f := newSweepFixture(t, &fakeWorkspaces{})
	f.sweeper.Now = nil // the real clock
	f.sweeper.tick = time.Millisecond
	repo := ns(gone) + "os-1-sandbox"
	f.registry.add(repo, "snap-gen1")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); f.sweeper.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for len(f.registry.tags(repo)) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("Run never swept")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // dozens of further ticks
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return when its context ended")
	}
	if got := f.registry.catalogRequests(); got != 1 {
		t.Errorf("the catalog was read %d times, want once for the whole interval", got)
	}
}

// Against the real workspaces table: a workspace whose row exists keeps its
// images however far its teardown has got (the delete's purgers run while the
// row stands), a workspace being created keeps them too, and one whose row is gone
// loses them. This is the safety the sweep rests on.
func TestOrphanSweepAgainstTheWorkspacesTablePG(t *testing.T) {
	st := openGroupsPG(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	mk := func(label string) string {
		t.Helper()
		tenant, err := st.CreateWorkspace(ctx, fmt.Sprintf("sweep-%s-%d", label, stamp), store.PlanHobby, fmt.Sprintf("sweep-owner-%s-%d", label, stamp))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.DeleteTenant(ctx, tenant.ID) })
		return tenant.ID
	}
	live, midDelete, deleted := mk("live"), mk("mid"), mk("gone")
	creating := ids.New(ids.Workspace)
	attempt := ids.New(ids.WorkspaceCreationAttempt)
	if _, err := st.Pool.Exec(ctx, `
		INSERT INTO workspace_creation_attempts (id, workspace_id, owner_subject, name, plan, billing_email, payment_required, state, expires_at)
		VALUES ($1, $2, 'owner', $3, 'pro', 'a@example.com', false, 'prepared', now() + interval '1 hour')`,
		attempt, creating, "attempt-"+attempt); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = st.Pool.Exec(ctx, `DELETE FROM workspace_creation_attempts WHERE id = $1`, attempt) })
	if err := st.DeleteTenant(ctx, deleted); err != nil {
		t.Fatal(err)
	}
	never := ids.New(ids.Workspace) // a namespace whose workspace nothing records

	f := newSweepFixture(t, st)
	repos := map[string]bool{} // repository -> whether its images must go
	for ws, goes := range map[string]bool{live: false, midDelete: false, creating: false, deleted: true, never: true} {
		repo := ns(ws) + "os-1-sandbox"
		f.registry.add(repo, "snap-gen1", "snap-gen2")
		repos[repo] = goes
	}
	// The mid-delete workspace's purge has already run: only its row is left.
	purger := &WorkspacePurger{Reaper: f.registry.reaper(), Storage: st}
	if err := purger.PurgeWorkspace(ctx, midDelete); err != nil {
		t.Fatal(err)
	}
	f.registry.add(ns(midDelete)+"os-1-sandbox", "snap-gen3") // an image a pause committed after the purge

	f.sweeper.Sweep(ctx)

	for repo, goes := range repos {
		left := f.registry.tags(repo)
		if goes && len(left) != 0 {
			t.Errorf("%s still has %v: its workspace is gone", repo, left)
		}
		if !goes && len(left) == 0 {
			t.Errorf("%s lost its images though its workspace exists or is being created", repo)
		}
	}
}
