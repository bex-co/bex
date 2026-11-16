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
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// These tests run the sweep's pass over an in-memory repository and a store whose
// listings the test holds, so the pool, the deadline and the resume are proved
// without a database or a clock. The real-database sweep tests are in
// sweeper_test.go.

var _ objectstorage.SweepRepository = (*poolRepo)(nil)

// poolRepo is an in-memory SweepRepository: one live workspace per entry, each
// with one bucket in Oregon, and the usage a recorded observation leaves.
type poolRepo struct {
	mu       sync.Mutex
	order    []string // the order the store lists live workspaces: by id
	buckets  map[string]string
	observed map[string]time.Time
	objects  map[string]int64 // what a workspace's last observation counted
}

func newPoolRepo(n int) *poolRepo {
	r := &poolRepo{buckets: map[string]string{}, observed: map[string]time.Time{}, objects: map[string]int64{}}
	for range n {
		ws := ids.New(ids.Workspace)
		r.order = append(r.order, ws)
		r.buckets[ws] = ids.New(ids.ObjectBucket)
	}
	return r
}

func (r *poolRepo) ListObjectStorageWorkspaces(context.Context) ([]store.ObjectStorageWorkspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]store.ObjectStorageWorkspace, 0, len(r.order))
	for _, ws := range r.order {
		out = append(out, store.ObjectStorageWorkspace{ID: ws, LastObserved: r.observed[ws]})
	}
	return out, nil
}

func (r *poolRepo) ListObjectBuckets(_ context.Context, ws string) ([]store.ObjectBucket, error) {
	return []store.ObjectBucket{{ID: r.buckets[ws], WorkspaceID: ws, Name: "photos", Region: string(objectstorage.Oregon)}}, nil
}

func (r *poolRepo) ListObjectStorageUsage(_ context.Context, ws string) ([]store.ObjectStorageUsage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	at, ok := r.observed[ws]
	if !ok {
		return nil, nil
	}
	return []store.ObjectStorageUsage{{BucketID: r.buckets[ws], WorkspaceID: ws, Region: string(objectstorage.Oregon), Objects: r.objects[ws], ObservedAt: at}}, nil
}

func (r *poolRepo) RecordObjectStorageObservation(ctx context.Context, ws, _ string, _ []store.ObjectStorageUsage, at time.Time) error {
	if err := ctx.Err(); err != nil { // the real write fails on a dead context too
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observed[ws] = at
	return nil
}

func (r *poolRepo) recorded() map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]bool{}
	for ws := range r.observed {
		out[ws] = true
	}
	return out
}

func (r *poolRepo) RetireObjectBuckets(context.Context, string) (int64, error) { return 0, nil }
func (r *poolRepo) MarkObjectStoragePurgeAttempt(context.Context, string, time.Time) error {
	return nil
}
func (r *poolRepo) FinishObjectStoragePurge(context.Context, string) error { return nil }

// listGate stands in for one region's Wasabi. A listing answers one object of
// the workspace once hold lets it; hold nil answers at once. The embedded Store
// is nil: the sweep's usage pass reaches List only.
type listGate struct {
	objectstorage.Store
	repo *poolRepo

	mu       sync.Mutex
	inFlight int
	peak     int
	started  []string
	hold     func(ctx context.Context, workspaceID string) error
}

func (g *listGate) List(ctx context.Context, prefix, _ string, _ int) (objectstorage.ListPage, error) {
	ws := strings.TrimSuffix(prefix, "/")
	g.mu.Lock()
	g.inFlight++
	g.peak = max(g.peak, g.inFlight)
	g.started = append(g.started, ws)
	hold := g.hold
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.inFlight--
		g.mu.Unlock()
	}()
	if hold != nil {
		if err := hold(ctx, ws); err != nil {
			return objectstorage.ListPage{}, err
		}
	}
	key, err := objectstorage.ObjectKey(ws, g.repo.buckets[ws], "k")
	if err != nil {
		return objectstorage.ListPage{}, err
	}
	return objectstorage.ListPage{Objects: []objectstorage.ObjectInfo{{Key: key, Size: 10}}}, nil
}

func (g *listGate) state() (inFlight, peak int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.inFlight, g.peak
}

func (g *listGate) startedOrder() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.started...)
}

func (g *listGate) resetStarted() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.started = nil
}

// blockUntil holds every listing for which pick is true until release closes or
// the pass's context ends, as a Wasabi that has stopped answering would.
func blockUntil(release <-chan struct{}, pick func(ws string) bool) func(context.Context, string) error {
	return func(ctx context.Context, ws string) error {
		if !pick(ws) {
			return nil
		}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type poolEnv struct {
	t       *testing.T
	repo    *poolRepo
	gate    *listGate
	sweeper *objectstorage.Sweeper
	reg     *prometheus.Registry
	now     time.Time

	logMu sync.Mutex
	logs  []string
}

func newPoolEnv(t *testing.T, workspaces, workers int) *poolEnv {
	t.Helper()
	e := &poolEnv{t: t, repo: newPoolRepo(workspaces), reg: prometheus.NewRegistry(), now: testNow}
	e.gate = &listGate{repo: e.repo}
	svc := objectstorage.NewService(nil, map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: e.gate})
	e.sweeper = objectstorage.NewSweeper(svc, e.repo)
	e.sweeper.Workers = workers
	e.sweeper.MinGap = 0
	e.sweeper.Metrics = objectstorage.NewSweepMetrics(e.reg)
	e.sweeper.Now = func() time.Time { return e.now }
	e.sweeper.Logf = func(format string, args ...any) {
		e.logMu.Lock()
		defer e.logMu.Unlock()
		e.logs = append(e.logs, fmt.Sprintf(format, args...))
	}
	return e
}

func (e *poolEnv) logged(substr string) bool {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	for _, l := range e.logs {
		if strings.Contains(l, substr) {
			return true
		}
	}
	return false
}

// passInBackground starts a pass and returns the channel its result arrives on.
func (e *poolEnv) passInBackground() <-chan error {
	done := make(chan error, 1)
	go func() { done <- e.sweeper.Pass(context.Background()) }()
	return done
}

func (e *poolEnv) wait(done <-chan error) error {
	e.t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		e.t.Fatal("the pass did not end")
		return nil
	}
}

// metric reads one series of the sweep's registry: a counter or gauge value, or a
// histogram's sample count.
func (e *poolEnv) metric(name string, labels map[string]string) float64 {
	e.t.Helper()
	families, err := e.reg.Gather()
	if err != nil {
		e.t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
	series:
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if want, ok := labels[l.GetName()]; ok && want != l.GetValue() {
					continue series
				}
			}
			switch {
			case m.GetHistogram() != nil:
				return float64(m.GetHistogram().GetSampleCount())
			case m.GetCounter() != nil:
				return m.GetCounter().GetValue()
			default:
				return m.GetGauge().GetValue()
			}
		}
	}
	return 0
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestPassNeverSweepsMoreWorkspacesThanItsWorkers(t *testing.T) {
	const workers = 3
	e := newPoolEnv(t, 12, workers)
	release := make(chan struct{})
	e.gate.hold = blockUntil(release, func(string) bool { return true })

	done := e.passInBackground()
	eventually(t, "the pool to fill", func() bool { n, _ := e.gate.state(); return n == workers })
	// Give a fourth listing every chance to start before looking.
	time.Sleep(150 * time.Millisecond)
	if n, peak := e.gate.state(); n != workers || peak != workers {
		t.Fatalf("with every listing held, %d are in flight (peak %d), want exactly %d", n, peak, workers)
	}
	close(release)
	if err := e.wait(done); err != nil {
		t.Fatalf("Pass = %v", err)
	}
	if _, peak := e.gate.state(); peak != workers {
		t.Errorf("peak in flight = %d, want %d", peak, workers)
	}
	if got := len(e.repo.recorded()); got != 12 {
		t.Errorf("%d of 12 workspaces recorded", got)
	}
	if got := e.metric("bex_usage_object_storage_sweep_workspaces_total", map[string]string{"result": "swept"}); got != 12 {
		t.Errorf("workspaces_total{swept} = %v, want 12", got)
	}
	if got := e.metric("bex_usage_object_storage_sweep_pass_duration_seconds", map[string]string{"result": "complete"}); got != 1 {
		t.Errorf("pass_duration_seconds{complete} samples = %v, want 1", got)
	}
	if got := e.metric("bex_usage_object_storage_sweep_last_complete_timestamp_seconds", nil); got == 0 {
		t.Error("a pass that reached every workspace left last_complete_timestamp_seconds at 0")
	}
	if !e.logged("took") || !e.logged("12 swept") {
		t.Errorf("the pass did not log its duration and counts: %q", e.logs)
	}
}

func TestASlowWorkspaceDoesNotHoldTheOthers(t *testing.T) {
	e := newPoolEnv(t, 8, 3)
	slow := e.repo.order[0] // first in line: a sequential walk would wait on it before all others
	release := make(chan struct{})
	e.gate.hold = blockUntil(release, func(ws string) bool { return ws == slow })

	done := e.passInBackground()
	eventually(t, "the other seven workspaces to be recorded while the slow one is held", func() bool {
		rec := e.repo.recorded()
		return len(rec) == 7 && !rec[slow]
	})
	select {
	case err := <-done:
		t.Fatalf("the pass ended (%v) while the slow workspace was still held", err)
	default:
	}
	close(release)
	if err := e.wait(done); err != nil {
		t.Fatalf("Pass = %v", err)
	}
	if got := len(e.repo.recorded()); got != 8 {
		t.Errorf("%d of 8 workspaces recorded", got)
	}
}

func TestAPanickingWorkspaceFailsAloneAndTheOthersAreSwept(t *testing.T) {
	e := newPoolEnv(t, 5, 2)
	bad := e.repo.order[2]
	e.gate.hold = func(_ context.Context, ws string) error {
		if ws == bad {
			panic("a row nobody expected")
		}
		return nil
	}
	err := e.sweeper.Pass(context.Background())
	if err == nil || !strings.Contains(err.Error(), "1 of 5 workspaces failed") {
		t.Fatalf("Pass = %v, want one failure reported", err)
	}
	rec := e.repo.recorded()
	if len(rec) != 4 || rec[bad] {
		t.Errorf("recorded %d workspaces (the panicking one: %v), want the other four", len(rec), rec[bad])
	}
	if !e.logged("recovered panic") {
		t.Error("the panic was not logged")
	}
}

func TestAPassCutOffAtItsDeadlineRecordsWhatItFinished(t *testing.T) {
	e := newPoolEnv(t, 6, 2)
	e.sweeper.PassTimeout = 300 * time.Millisecond
	// The first two workspaces answer at once; Wasabi then stops answering.
	first := map[string]bool{e.repo.order[0]: true, e.repo.order[1]: true}
	e.gate.hold = blockUntil(nil, func(ws string) bool { return !first[ws] })

	start := time.Now()
	err := e.sweeper.Pass(context.Background())
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("the pass took %s past a 300ms deadline", took)
	}
	if err == nil || !strings.Contains(err.Error(), "4 of 6 workspaces not reached") {
		t.Fatalf("Pass = %v, want the four it did not reach counted", err)
	}
	rec := e.repo.recorded()
	if len(rec) != 2 || !rec[e.repo.order[0]] || !rec[e.repo.order[1]] {
		t.Errorf("recorded %v, want exactly the two that finished", rec)
	}
	if e.logged("object storage sweep: workspace ") {
		t.Errorf("a workspace cut off by the deadline was logged as a failure: %q", e.logs)
	}
	if !e.logged("2 swept, 0 fresh, 0 failed, 4 not reached") {
		t.Errorf("the summary does not count what finished and what did not: %q", e.logs)
	}
	if got := e.metric("bex_usage_object_storage_sweep_pass_duration_seconds", map[string]string{"result": "cut_off"}); got != 1 {
		t.Errorf("pass_duration_seconds{cut_off} samples = %v, want 1", got)
	}
	if got := e.metric("bex_usage_object_storage_sweep_workspaces_total", map[string]string{"result": "not_reached"}); got != 4 {
		t.Errorf("workspaces_total{not_reached} = %v, want 4", got)
	}
	if got := e.metric("bex_usage_object_storage_sweep_last_complete_timestamp_seconds", nil); got != 0 {
		t.Error("a cut-off pass counted as complete")
	}
}

func TestTheNextPassStartsWithTheWorkspacesTheLastOneDidNotReach(t *testing.T) {
	// One worker makes the order exact. Three of five workspaces are reached
	// before Wasabi stops answering on the fourth.
	e := newPoolEnv(t, 5, 1)
	e.sweeper.PassTimeout = 300 * time.Millisecond
	stalled := e.repo.order[3]
	var mu sync.Mutex
	answering := false
	e.gate.hold = func(ctx context.Context, ws string) error {
		mu.Lock()
		ok := answering
		mu.Unlock()
		if ws != stalled || ok {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if err := e.sweeper.Pass(context.Background()); err == nil || !strings.Contains(err.Error(), "2 of 5 workspaces not reached") {
		t.Fatalf("first pass = %v, want it cut off with two not reached", err)
	}
	if rec := e.repo.recorded(); len(rec) != 3 {
		t.Fatalf("the first pass recorded %d workspaces, want the three it finished", len(rec))
	}

	// The next hour: Wasabi answers again. Every workspace is due (MinGap is 0
	// here), and the pass must start with the two it never recorded, not with
	// the head of the list it already swept.
	mu.Lock()
	answering = true
	mu.Unlock()
	e.gate.resetStarted()
	e.now = e.now.Add(time.Hour)
	if err := e.sweeper.Pass(context.Background()); err != nil {
		t.Fatalf("second pass = %v", err)
	}
	want := []string{e.repo.order[3], e.repo.order[4], e.repo.order[0], e.repo.order[1], e.repo.order[2]}
	got := e.gate.startedOrder()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the second pass swept %v, want the unreached %v first, then the rest oldest first", got, want)
	}
	if rec := e.repo.recorded(); len(rec) != 5 {
		t.Errorf("after the second pass %d of 5 workspaces are recorded", len(rec))
	}
}

// exclusiveCount lets every pass through, as the advisory lock does for the one
// replica that holds it, and counts how many are inside at once.
type exclusiveCount struct {
	mu        sync.Mutex
	active    int
	peak      int
	durations []time.Duration
	errs      []error
}

func (x *exclusiveCount) RunExclusive(ctx context.Context, _ string, fn func(context.Context) error) (bool, error) {
	x.mu.Lock()
	x.active++
	x.peak = max(x.peak, x.active)
	x.mu.Unlock()
	start := time.Now()
	err := fn(ctx)
	x.mu.Lock()
	x.active--
	x.durations = append(x.durations, time.Since(start))
	x.errs = append(x.errs, err)
	x.mu.Unlock()
	return true, err
}

func TestARunCutsEachPassAtItsIntervalAndNeverOverlapsTheNext(t *testing.T) {
	e := newPoolEnv(t, 4, 2)
	// Wasabi never answers, and the pass's own timeout is the production hour
	// (the real one is 50 minutes): only the interval can end a pass.
	e.sweeper.PassTimeout = time.Hour
	e.gate.hold = blockUntil(nil, func(string) bool { return true })
	x := &exclusiveCount{}
	e.sweeper.Exclusive = x

	const interval = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		e.sweeper.Run(ctx, interval)
		close(finished)
	}()
	time.Sleep(1200 * time.Millisecond)
	cancel()
	<-finished

	x.mu.Lock()
	defer x.mu.Unlock()
	if len(x.durations) < 3 {
		t.Fatalf("%d passes ran in 1.2 s at a 100ms interval, want several", len(x.durations))
	}
	if x.peak != 1 {
		t.Errorf("%d passes were inside at once, want 1: a tick during a pass must be skipped, not overlap it", x.peak)
	}
	for i, d := range x.durations {
		if d > 600*time.Millisecond {
			t.Errorf("pass %d ran %s, past its 100ms interval", i, d)
		}
		if x.errs[i] == nil || !strings.Contains(x.errs[i].Error(), "cut off") {
			t.Errorf("pass %d ended %v, want a cut-off pass reported", i, x.errs[i])
		}
	}
}

// prometheusPath is the scrape config whose bex-api job keeps only the metric
// families its keep-list names; a family outside it is exported by the pods and
// stored by nothing (the sweep's first names were: bex_object_storage_sweep_*).
const prometheusPath = "../../../../deploy/gitops/base/prometheus.yaml"

func TestEverySweepMetricIsKeptByTheScrapeJob(t *testing.T) {
	raw, err := os.ReadFile(prometheusPath)
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^\s*regex:\s*(bex_\(billing\|\S+)\s*$`).FindSubmatch(raw)
	if found == nil {
		t.Fatalf("the bex-api keep-list (the regex that starts bex_(billing|) is not in %s: update this test with it", prometheusPath)
	}
	keep := regexp.MustCompile(`^(?:` + string(found[1]) + `)$`) // Prometheus anchors relabel regexes

	e := newPoolEnv(t, 1, 1)
	if err := e.sweeper.Pass(context.Background()); err != nil {
		t.Fatal(err)
	}
	families, err := e.reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	if len(families) != 5 {
		t.Fatalf("the sweep registered %d metric families after a pass, want 5", len(families))
	}
	for _, f := range families {
		if !keep.MatchString(f.GetName()) {
			t.Errorf("%s is not matched by the bex-api scrape job's keep-list %q: Prometheus would drop it", f.GetName(), found[1])
		}
	}
}
