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
	latestBy map[string]time.Time // serviceID -> latest window_start
}

type usageKey struct {
	serviceID, kind, tier string
	windowStart           time.Time
}

func newMemUsageStore(apps ...store.App) *memUsageStore {
	return &memUsageStore{
		apps:     apps,
		rows:     map[usageKey]store.HourlyRow{},
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

func (m *memUsageStore) UsageMonthToDate(_ context.Context, workspaceID string, now time.Time) ([]store.UsageSummaryRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	totals := map[struct{ svc, kind, tier string }]int64{}
	for k, row := range m.rows {
		if row.WorkspaceID != workspaceID {
			continue
		}
		ws := k.windowStart.UTC()
		if ws.Before(monthStart) || !ws.Before(now) {
			continue
		}
		key := struct{ svc, kind, tier string }{k.serviceID, k.kind, k.tier}
		totals[key] += row.Quantity
	}
	out := make([]store.UsageSummaryRow, 0, len(totals))
	for key, total := range totals {
		out = append(out, store.UsageSummaryRow{ServiceID: key.svc, Kind: key.kind, Tier: key.tier, Total: total})
	}
	return out, nil
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

func TestStoreNilReturnsUnavailable(t *testing.T) {
	svc := &Service{
		Base:  &core.Base{},
		Store: nil, // store off
	}
	_, err := svc.MonthToDate(context.Background())
	if !errors.Is(err, ErrUsageUnavailable) {
		t.Errorf("expected ErrUsageUnavailable, got %v", err)
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

