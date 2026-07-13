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

package usage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- in-memory store stub ---

type memUsageStore struct {
	mu       sync.Mutex
	apps     []store.App
	rows     map[usageKey]store.HourlyRow
	monthly  map[monthKey]monthlyVal
	latestBy map[string]time.Time // serviceID -> latest window_start
	compacts []time.Time          // boundaries CompactUsage was called with
}

type usageKey struct {
	serviceID, kind, tier string
	windowStart           time.Time
}

type monthKey struct {
	serviceID, kind, tier string
	month                 time.Time // first of the calendar month (UTC)
}

type monthlyVal struct {
	workspaceID  string
	resourceKind string
	quantity     int64
}

func newMemUsageStore(apps ...store.App) *memUsageStore {
	return &memUsageStore{
		apps:     apps,
		rows:     map[usageKey]store.HourlyRow{},
		monthly:  map[monthKey]monthlyVal{},
		latestBy: map[string]time.Time{},
	}
}

func (m *memUsageStore) ListApps(_ context.Context) ([]store.App, error) {
	return m.apps, nil
}

func (m *memUsageStore) UpsertUsageHourly(_ context.Context, row store.HourlyRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := usageKey{row.ServiceID, row.Kind, row.Tier, row.WindowStart.UTC().Truncate(time.Hour)}
	m.rows[k] = row
	if row.WindowStart.After(m.latestBy[row.ServiceID]) {
		m.latestBy[row.ServiceID] = row.WindowStart
	}
	return nil
}

func (m *memUsageStore) LatestUsageWindow(_ context.Context, serviceID string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.latestBy[serviceID], nil
}

// UsageMonthToDate mirrors PGStore's two-table read: hourly rows in
// [monthStart, now) plus the month's usage_monthly aggregate, summed.
func (m *memUsageStore) UsageMonthToDate(_ context.Context, workspaceID string, now time.Time) ([]store.UsageSummaryRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	type summaryKey struct{ svc, kind, tier, resourceKind string }
	totals := map[summaryKey]int64{}
	for k, row := range m.rows {
		if row.WorkspaceID != workspaceID {
			continue
		}
		ws := k.windowStart.UTC()
		if ws.Before(monthStart) || !ws.Before(now) {
			continue
		}
		key := summaryKey{k.serviceID, k.kind, k.tier, row.ResourceKind}
		totals[key] += row.Quantity
	}
	for k, v := range m.monthly {
		if v.workspaceID != workspaceID || !k.month.Equal(monthStart) {
			continue
		}
		key := summaryKey{k.serviceID, k.kind, k.tier, v.resourceKind}
		totals[key] += v.quantity
	}
	out := make([]store.UsageSummaryRow, 0, len(totals))
	for key, total := range totals {
		out = append(out, store.UsageSummaryRow{
			ServiceID:    key.svc,
			Kind:         key.kind,
			Tier:         key.tier,
			ResourceKind: key.resourceKind,
			Total:        total,
		})
	}
	return out, nil
}

// CompactUsage mirrors PGStore's compaction: fold hourly rows older than
// before into the monthly map additively, then delete them. It also records
// the boundary so tests can assert what the loop asked for.
func (m *memUsageStore) CompactUsage(_ context.Context, before time.Time) (store.UsageCompaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compacts = append(m.compacts, before)
	var res store.UsageCompaction
	months := map[time.Time]bool{}
	for k, row := range m.rows {
		if !k.windowStart.Before(before) {
			continue
		}
		ws := k.windowStart.UTC()
		month := time.Date(ws.Year(), ws.Month(), 1, 0, 0, 0, 0, time.UTC)
		mk := monthKey{k.serviceID, k.kind, k.tier, month}
		m.monthly[mk] = monthlyVal{
			workspaceID:  row.WorkspaceID,
			resourceKind: row.ResourceKind,
			quantity:     m.monthly[mk].quantity + row.Quantity,
		}
		delete(m.rows, k)
		months[month] = true
		res.HourlyRows++
	}
	res.Months = int64(len(months))
	return res, nil
}

// compactCalls returns the boundaries CompactUsage was invoked with.
func (m *memUsageStore) compactCalls() []time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]time.Time(nil), m.compacts...)
}

// --- Prometheus fake ---

func fakeProm(value float64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		v := "0"
		if value > 0 {
			v = strconv.FormatFloat(value, 'f', -1, 64)
		}
		resp := map[string]any{
			"status": "success",
			"data": map[string]any{
				"result": []map[string]any{
					{"value": []any{float64(time.Now().Unix()), v}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// --- Tests ---

func TestProcessWindowUpsertIdempotent(t *testing.T) {
	app := store.App{ID: "srv-001", TenantID: "tea-001", Name: "myapp", Tier: "starter"}
	st := newMemUsageStore(app)

	// Fake Prometheus returning 2.0 pods-worth of presence.
	prom := fakeProm(2.0)
	defer prom.Close()

	svc := &Service{
		Base:     &core.Base{},
		Store:    st,
		PromBase: prom.URL,
	}

	window := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)

	// First call.
	svc.processWindow(context.Background(), app, window)
	rowsBefore := len(st.rows)

	// Second call — same window.
	svc.processWindow(context.Background(), app, window)
	rowsAfter := len(st.rows)

	if rowsAfter != rowsBefore {
		t.Errorf("idempotent upsert: row count changed from %d to %d on re-run", rowsBefore, rowsAfter)
	}
}

func TestMonthToDateBoundary(t *testing.T) {
	// Two rows: one in June, one in July. A query for July should only see the
	// July row.
	app := store.App{ID: "srv-002", TenantID: "tea-002", Name: "myapp", Tier: "free"}
	st := newMemUsageStore(app)

	june := time.Date(2026, 6, 30, 23, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 1, 1, 0, 0, 0, time.UTC)

	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-002", ServiceID: "srv-002",
		Kind: store.UsageKindInstanceSeconds, Tier: "free",
		WindowStart: june, Quantity: 3600,
	})
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-002", ServiceID: "srv-002",
		Kind: store.UsageKindInstanceSeconds, Tier: "free",
		WindowStart: july, Quantity: 7200,
	})

	nowJuly := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	rows, err := st.UsageMonthToDate(context.Background(), "tea-002", nowJuly)
	if err != nil {
		t.Fatalf("UsageMonthToDate: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Total != 7200 {
		t.Errorf("expected 7200 (July only), got %d", rows[0].Total)
	}
}

func TestMonthToDateEmptyMonth(t *testing.T) {
	app := store.App{ID: "srv-003", TenantID: "tea-003", Name: "myapp", Tier: "free"}
	st := newMemUsageStore(app)

	rows, err := st.UsageMonthToDate(context.Background(), "tea-003", time.Now().UTC())
	if err != nil {
		t.Fatalf("empty month: unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("empty month: expected 0 rows, got %d", len(rows))
	}
}

func TestCatchUpFillsMissedWindows(t *testing.T) {
	app := store.App{ID: "srv-004", TenantID: "tea-004", Name: "catchupapp", Tier: "starter"}
	st := newMemUsageStore(app)

	prom := fakeProm(1.0)
	defer prom.Close()

	svc := &Service{
		Base:     &core.Base{},
		Store:    st,
		PromBase: prom.URL,
	}

	// Simulate that the loop was last alive 3h ago: seed one row at -3h.
	past := time.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour)
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-004", ServiceID: "srv-004",
		Kind: store.UsageKindInstanceSeconds, Tier: "starter",
		WindowStart: past, Quantity: 3600,
	})

	svc.catchUp(context.Background())

	// The catch-up should have added rows for the 2 missed hours.
	total := 0
	for k := range st.rows {
		if k.serviceID == "srv-004" {
			total++
		}
	}
	// Expect the original + at least 2 new windows.
	if total < 3 {
		t.Errorf("catch-up: expected ≥3 rows (original + 2 catch-up), got %d", total)
	}
}

func TestCatchUpIsIdempotent(t *testing.T) {
	app := store.App{ID: "srv-005", TenantID: "tea-005", Name: "idempapp", Tier: "free"}
	st := newMemUsageStore(app)

	prom := fakeProm(0.5)
	defer prom.Close()

	svc := &Service{
		Base:     &core.Base{},
		Store:    st,
		PromBase: prom.URL,
	}

	svc.catchUp(context.Background())
	rowsAfterFirst := len(st.rows)

	svc.catchUp(context.Background())
	rowsAfterSecond := len(st.rows)

	if rowsAfterSecond != rowsAfterFirst {
		t.Errorf("catch-up idempotent: row count changed %d→%d on second run", rowsAfterFirst, rowsAfterSecond)
	}
}

func TestCompactBoundaryDefaultRetention(t *testing.T) {
	// Clock mid-July, default hot window (3 months) → July, June, May stay
	// hourly; the boundary is May 1 (well clear of the 48 h clamp).
	st := newMemUsageStore()
	svc := &Service{
		Base:  &core.Base{Clock: fixedClock()},
		Store: st,
	}

	svc.compact(context.Background())

	calls := st.compactCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 CompactUsage call, got %d", len(calls))
	}
	want := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !calls[0].Equal(want) {
		t.Errorf("boundary: want %v, got %v", want, calls[0])
	}
}

func TestCompactBoundaryClampedToCatchupLimit(t *testing.T) {
	// Retention 1 month with the clock just past a month boundary: the hot
	// window alone would compact everything before July 1, but the rollup's
	// catch-up can rewrite any window in the last 48 h — the boundary must be
	// clamped to now-48h so re-metered windows are never double-counted.
	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	st := newMemUsageStore()
	svc := &Service{
		Base:            &core.Base{Clock: clockAt(now)},
		Store:           st,
		RetentionMonths: 1,
	}

	svc.compact(context.Background())

	calls := st.compactCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 CompactUsage call, got %d", len(calls))
	}
	want := now.Add(-catchupLimit) // 2026-06-29 12:00, not 2026-07-01 00:00
	if !calls[0].Equal(want) {
		t.Errorf("boundary: want clamp %v, got %v", want, calls[0])
	}
}

func TestCompactFoldsOldMonthsAndPurges(t *testing.T) {
	// Rows in April (2 rows, one kind), May, and July, clock mid-July with the
	// default window (May–July hot): only April compacts; May and July hourly
	// rows are untouched. Re-running is a no-op.
	st := newMemUsageStore()
	seed := func(ws time.Time, qty int64) {
		_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
			WorkspaceID: "tea-001", ServiceID: "srv-001",
			Kind: store.UsageKindInstanceSeconds, Tier: "starter",
			WindowStart: ws, Quantity: qty,
		})
	}
	seed(time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC), 3600)
	seed(time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC), 1800)
	seed(time.Date(2026, 5, 2, 8, 0, 0, 0, time.UTC), 900)
	seed(time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC), 450)

	svc := &Service{
		Base:  &core.Base{Clock: fixedClock()},
		Store: st,
	}
	svc.compact(context.Background())

	st.mu.Lock()
	hourlyLeft, monthlyRows := len(st.rows), len(st.monthly)
	april := st.monthly[monthKey{"srv-001", store.UsageKindInstanceSeconds, "starter", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}]
	st.mu.Unlock()

	if hourlyLeft != 2 {
		t.Errorf("hourly rows left: want 2 (May + July), got %d", hourlyLeft)
	}
	if monthlyRows != 1 {
		t.Errorf("monthly rows: want 1 (April), got %d", monthlyRows)
	}
	if april.quantity != 5400 {
		t.Errorf("April aggregate: want 5400, got %d", april.quantity)
	}

	// Idempotency: a second pass finds nothing older than the boundary.
	svc.compact(context.Background())
	st.mu.Lock()
	hourlyAfter, aprilAfter := len(st.rows), st.monthly[monthKey{"srv-001", store.UsageKindInstanceSeconds, "starter", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}]
	st.mu.Unlock()
	if hourlyAfter != hourlyLeft || aprilAfter.quantity != 5400 {
		t.Errorf("re-run not a no-op: hourly %d→%d, April %d→%d", hourlyLeft, hourlyAfter, april.quantity, aprilAfter.quantity)
	}
}

func TestPeriodQueryIdenticalAcrossCompaction(t *testing.T) {
	// The boundary-crossing invariant (t005/t006): a period query for an old
	// month returns the same totals before and after that month is compacted.
	st := seedStore() // two July rows for tea-001
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: "tea-001", ServiceID: "srv-001",
		Kind: store.UsageKindInstanceSeconds, Tier: "starter",
		WindowStart: time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC), Quantity: 7200,
	})
	svc := svcWithTenant(st, "tea-001")
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})

	marchEnd := time.Date(2026, 3, 31, 23, 59, 59, 0, time.UTC)
	before, err := svc.monthToDateAt(ctx, marchEnd)
	if err != nil {
		t.Fatalf("pre-compaction query: %v", err)
	}

	svc.compact(ctx) // clock is July 15 → March is outside the hot window

	after, err := svc.monthToDateAt(ctx, marchEnd)
	if err != nil {
		t.Fatalf("post-compaction query: %v", err)
	}
	if len(before.Services) != 1 || len(after.Services) != 1 {
		t.Fatalf("services: want 1 before and after, got %d / %d", len(before.Services), len(after.Services))
	}
	br, ar := before.Services[0].Rows, after.Services[0].Rows
	if len(br) != 1 || len(ar) != 1 || br[0] != ar[0] {
		t.Errorf("totals changed across compaction: before %+v, after %+v", br, ar)
	}
	if ar[0].Total != 7200 {
		t.Errorf("March total: want 7200, got %d", ar[0].Total)
	}

	// The March hourly detail must actually be gone.
	st.mu.Lock()
	for k := range st.rows {
		if k.windowStart.Month() == time.March {
			t.Errorf("March hourly row survived compaction: %+v", k)
		}
	}
	st.mu.Unlock()
}

func TestRunTicksCompactionWithoutPrometheus(t *testing.T) {
	// With PromBase empty the loop must still run the compaction side: once at
	// startup, then on every compaction tick.
	st := newMemUsageStore()
	svc := &Service{
		Base:  &core.Base{Clock: fixedClock()},
		Store: st,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.RunWithIntervals(ctx, time.Hour, 5*time.Millisecond)
	}()

	deadline := time.After(2 * time.Second)
	for len(st.compactCalls()) < 3 {
		select {
		case <-deadline:
			cancel()
			t.Fatalf("expected ≥3 compaction passes (startup + ticks), got %d", len(st.compactCalls()))
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

func TestStoreNilReturnsUnavailable(t *testing.T) {
	svc := &Service{
		Base:  &core.Base{},
		Store: nil, // store off
	}
	_, err := svc.MonthToDate(context.Background())
	if !errors.Is(err, core.ErrUsageUnavailable) {
		t.Errorf("expected core.ErrUsageUnavailable, got %v", err)
	}
}

func TestBuildSecondsFromFakeJobs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)

	window := time.Date(2026, 7, 9, 14, 0, 0, 0, time.UTC)
	end := window.Add(time.Hour)

	start := metav1.NewTime(window.Add(5 * time.Minute))
	completion := metav1.NewTime(window.Add(12 * time.Minute))
	outsideCompletion := metav1.NewTime(window.Add(90 * time.Minute)) // outside window

	inWindow := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-myapp-abc",
			Namespace: "default",
			Labels:    map[string]string{labelBuild: "myapp"},
		},
		Status: batchv1.JobStatus{
			StartTime:      &start,
			CompletionTime: &completion,
		},
	}
	outWindow := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-myapp-xyz",
			Namespace: "default",
			Labels:    map[string]string{labelBuild: "myapp"},
		},
		Status: batchv1.JobStatus{
			StartTime:      &start,
			CompletionTime: &outsideCompletion,
		},
	}

	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(inWindow, outWindow).Build()

	svc := &Service{
		Base: &core.Base{Client: cl, Namespace: "default"},
	}

	secs := svc.queryBuildSeconds(context.Background(), "myapp", window, end)
	expected := int64(7 * 60) // 7 minutes
	if secs != expected {
		t.Errorf("build_seconds: expected %d, got %d", expected, secs)
	}
}

func TestPromInstantScalarParsesCorrectly(t *testing.T) {
	srv := fakeProm(42.5)
	defer srv.Close()

	v, err := promInstantScalar(context.Background(), nil, srv.URL, `up`, time.Now())
	if err != nil {
		t.Fatalf("promInstantScalar: %v", err)
	}
	const want = 42.5
	if v != want {
		t.Errorf("expected %v, got %v", want, v)
	}
}

// --- t008: Database/KeyValue rollup tests ---

// buildFakeClientWithDatastores creates a fake k8s client containing one
// Database CR ("mydb", plan "basic-256mb", tenant "tea-ds") and one KeyValue
// CR ("mykv", plan "starter", tenant "tea-ds").
func buildFakeClientWithDatastores(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = appv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mydb",
			Namespace: "default",
			Labels: map[string]string{
				core.LabelTenant: "tea-ds",
			},
		},
		Spec: appv1alpha1.DatabaseSpec{Plan: "basic-256mb"},
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mykv",
			Namespace: "default",
			Labels: map[string]string{
				core.LabelTenant: "tea-ds",
			},
		},
		Spec: appv1alpha1.KeyValueSpec{Plan: "starter"},
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(db, kv).Build()
}

// TestProcessDatastoreWindowUpsertInstanceSeconds verifies that
// processDatastoreWindow writes an instance_seconds row with the correct
// resource_kind, service_id (CR name), and tier (plan) for a Database.
func TestProcessDatastoreWindowUpsertInstanceSeconds(t *testing.T) {
	prom := fakeProm(1.0) // 1 pod present → 3600 instance-seconds per window
	defer prom.Close()

	st := newMemUsageStore()
	svc := &Service{
		Base:     &core.Base{},
		Store:    st,
		PromBase: prom.URL,
	}

	ds := datastoreEntry{
		ID:       "mydb",
		Name:     "mydb",
		TenantID: "tea-ds",
		Plan:     "basic-256mb",
		Kind:     store.ResourceKindPostgres,
	}
	window := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	svc.processDatastoreWindow(context.Background(), ds, window)

	k := usageKey{"mydb", store.UsageKindInstanceSeconds, "basic-256mb", window}
	st.mu.Lock()
	row, ok := st.rows[k]
	st.mu.Unlock()

	if !ok {
		t.Fatalf("expected usage_hourly row for mydb/instance_seconds, none found")
	}
	if row.ResourceKind != store.ResourceKindPostgres {
		t.Errorf("ResourceKind: want %q, got %q", store.ResourceKindPostgres, row.ResourceKind)
	}
	if row.Quantity <= 0 {
		t.Errorf("instance_seconds quantity: expected > 0, got %d", row.Quantity)
	}
}

// TestProcessDatastoreWindowNoEgressOrBuild verifies that
// processDatastoreWindow writes ONLY instance_seconds — never egress_bytes or
// build_seconds — for a Database or KeyValue resource.
func TestProcessDatastoreWindowNoEgressOrBuild(t *testing.T) {
	prom := fakeProm(1.0)
	defer prom.Close()

	st := newMemUsageStore()
	svc := &Service{
		Base:     &core.Base{},
		Store:    st,
		PromBase: prom.URL,
	}

	ds := datastoreEntry{
		ID:       "mykv",
		Name:     "mykv",
		TenantID: "tea-ds",
		Plan:     "starter",
		Kind:     store.ResourceKindKeyValue,
	}
	window := time.Date(2026, 7, 11, 6, 0, 0, 0, time.UTC)
	svc.processDatastoreWindow(context.Background(), ds, window)

	st.mu.Lock()
	defer st.mu.Unlock()
	for k := range st.rows {
		if k.serviceID == "mykv" && (k.kind == store.UsageKindEgressBytes || k.kind == store.UsageKindBuildSeconds) {
			t.Errorf("unexpected %s row for datastore mykv", k.kind)
		}
	}
}

// TestListDatastoresPicksUpCRs verifies that listDatastores finds both a
// Database and a KeyValue CR with the correct attributes (id, plan, kind,
// tenantID) and skips unlabeled CRs.
func TestListDatastoresPicksUpCRs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	// Two labeled CRs (to pick up) and one unlabeled Database (to skip).
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mydb", Namespace: "default",
			Labels: map[string]string{core.LabelTenant: "tea-ds"},
		},
		Spec: appv1alpha1.DatabaseSpec{Plan: "basic-256mb"},
	}
	unlabeledDB := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "orphan", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free"},
	}
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mykv", Namespace: "default",
			Labels: map[string]string{core.LabelTenant: "tea-ds"},
		},
		Spec: appv1alpha1.KeyValueSpec{Plan: "starter"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(db, unlabeledDB, kv).Build()

	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	entries, err := svc.listDatastores(context.Background())
	if err != nil {
		t.Fatalf("listDatastores: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 labeled entries, got %d: %+v", len(entries), entries)
	}
	byID := map[string]datastoreEntry{}
	for _, e := range entries {
		byID[e.ID] = e
	}
	dbEntry := byID["mydb"]
	if dbEntry.TenantID != "tea-ds" || dbEntry.Plan != "basic-256mb" || dbEntry.Kind != store.ResourceKindPostgres {
		t.Errorf("mydb entry wrong: %+v", dbEntry)
	}
	kvEntry := byID["mykv"]
	if kvEntry.TenantID != "tea-ds" || kvEntry.Plan != "starter" || kvEntry.Kind != store.ResourceKindKeyValue {
		t.Errorf("mykv entry wrong: %+v", kvEntry)
	}
}

// TestCatchUpCoversDatastoreWindows verifies that catchUp fills missed windows
// for Database/KeyValue CRs alongside App services.
func TestCatchUpCoversDatastoreWindows(t *testing.T) {
	cl := buildFakeClientWithDatastores(t)
	app := store.App{ID: "srv-ds", TenantID: "tea-ds", Name: "api", Tier: "starter"}
	st := newMemUsageStore(app)

	prom := fakeProm(1.0)
	defer prom.Close()

	svc := &Service{
		Base:     &core.Base{Client: cl, Namespace: "default"},
		Store:    st,
		PromBase: prom.URL,
	}

	svc.catchUp(context.Background())

	st.mu.Lock()
	var appRows, dbRows, kvRows int
	for _, row := range st.rows {
		switch row.ResourceKind {
		case store.ResourceKindService:
			appRows++
		case store.ResourceKindPostgres:
			dbRows++
		case store.ResourceKindKeyValue:
			kvRows++
		}
	}
	st.mu.Unlock()

	// At least one window per resource after catch-up.
	if appRows == 0 {
		t.Error("catchUp: no App rows written")
	}
	if dbRows == 0 {
		t.Error("catchUp: no Database rows written")
	}
	if kvRows == 0 {
		t.Error("catchUp: no KeyValue rows written")
	}
}

// TestRollupCoversDatastoreWindow verifies that rollup processes Database and
// KeyValue CRs for the window alongside Apps.
func TestRollupCoversDatastoreWindow(t *testing.T) {
	cl := buildFakeClientWithDatastores(t)
	app := store.App{ID: "srv-ds2", TenantID: "tea-ds", Name: "api2", Tier: "free"}
	st := newMemUsageStore(app)

	prom := fakeProm(1.0)
	defer prom.Close()

	svc := &Service{
		Base:     &core.Base{Client: cl, Namespace: "default"},
		Store:    st,
		PromBase: prom.URL,
	}

	// rollup operates on the window that just closed (the hour before t).
	t2 := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) // triggers window 11:00
	svc.rollup(context.Background(), t2)

	st.mu.Lock()
	var dbRows, kvRows int
	for _, row := range st.rows {
		switch row.ResourceKind {
		case store.ResourceKindPostgres:
			dbRows++
		case store.ResourceKindKeyValue:
			kvRows++
		}
	}
	st.mu.Unlock()

	if dbRows == 0 {
		t.Error("rollup: no Database rows written")
	}
	if kvRows == 0 {
		t.Error("rollup: no KeyValue rows written")
	}
}

