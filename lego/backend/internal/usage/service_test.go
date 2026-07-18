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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/egressquery"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- in-memory store stub ---

type memUsageStore struct {
	mu       sync.Mutex
	apps     []store.App
	rows     map[usageKey]store.HourlyRow
	monthly  map[monthKey]monthlyVal
	latestBy map[string]time.Time // resourceKind/serviceID/kind -> latest window_start
	compacts []time.Time          // boundaries CompactUsage was called with
	upsert   func(store.HourlyRow) error
}

type usageKey struct {
	resourceKind          string
	serviceID, kind, tier string
	windowStart           time.Time
}

type monthKey struct {
	resourceKind          string
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
	if m.upsert != nil {
		if err := m.upsert(row); err != nil {
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	row.ResourceKind = store.NormalizeResourceKind(row.ResourceKind)
	k := usageKey{row.ResourceKind, row.ServiceID, row.Kind, row.Tier, row.WindowStart.UTC().Truncate(time.Hour)}
	m.rows[k] = row
	resourceID := row.ResourceKind + "/" + row.ServiceID + "/" + row.Kind
	if row.WindowStart.After(m.latestBy[resourceID]) {
		m.latestBy[resourceID] = row.WindowStart
	}
	return nil
}

func (m *memUsageStore) LatestUsageWindow(_ context.Context, resourceKind, serviceID, kind string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.latestBy[store.NormalizeResourceKind(resourceKind)+"/"+serviceID+"/"+kind], nil
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
		if key.kind != store.UsageKindInstanceSeconds && total == 0 {
			continue
		}
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
		mk := monthKey{k.resourceKind, k.serviceID, k.kind, k.tier, month}
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
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := "0"
		if strings.Contains(r.URL.Query().Get("query"), "healthy") || strings.Contains(r.URL.Query().Get("query"), `up{job=`) {
			v = "1"
		} else if value > 0 {
			v = strconv.FormatFloat(value, 'f', -1, 64)
		}
		resp := map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []map[string]any{
					{"value": []any{float64(time.Now().Unix()), v}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// appMeterClient returns the projected private App behind one store row. Its
// missing Ingress is a real public-egress zero, while Jobs can still be listed
// for the build meter in the same fake client.
func appMeterClient(t *testing.T, app store.App) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	projected := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name:      app.TenantID + "-" + app.Name,
		Namespace: "default",
		Labels:    map[string]string{store.LabelAppID: app.ID},
	}}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(projected).Build()
}

func publicAppMeterClient(t *testing.T, app store.App, crName, backend string, hosts ...string) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = networkingv1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	class := "traefik"
	pathType := networkingv1.PathTypePrefix
	projected := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: "default",
			Labels:    map[string]string{store.LabelAppID: app.ID},
		},
		Spec: appv1alpha1.AppSpec{Host: hosts[0], Hosts: hosts[1:]},
	}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: projected.Name, Namespace: "default"},
		Spec:       networkingv1.IngressSpec{IngressClassName: &class},
	}
	for _, host := range hosts {
		ingress.Spec.Rules = append(ingress.Spec.Rules, networkingv1.IngressRule{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{{
					Path:     "/",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: backend,
					}},
				}},
			}},
		})
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(projected, ingress).Build()
}

func TestQueryEgressBytesUsesExactRoutersForSharedBackend(t *testing.T) {
	var mu sync.Mutex
	var gotQueries []string
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		mu.Lock()
		gotQueries = append(gotQueries, query)
		mu.Unlock()
		value := "1"
		switch {
		case strings.Contains(query, "traefik_router_responses_bytes_total"):
			value = "42"
		case strings.Contains(query, "bex_websocket_egress_bytes_total"):
			value = "3"
		case strings.Contains(query, "bex_app_direct_egress_bytes_total"):
			value = "5"
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,%q]}]}}`, value)
	}))
	defer prom.Close()

	app := store.App{ID: "srv-static", TenantID: "tea-one", Name: "static"}
	cl := publicAppMeterClient(t, app, "tea-one-static", "shared-static-server", "site.onbex.co", "www.example.com")
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, nil, prom.URL, prom.Client())

	quantity, ok := svc.queryEgressBytes(context.Background(), app, time.Now().Add(-time.Hour), time.Now())
	if !ok || quantity != 50 {
		t.Fatalf("queryEgressBytes: got quantity=%d ok=%v, want 50/true", quantity, ok)
	}
	mu.Lock()
	gotQuery := strings.Join(gotQueries, "\n")
	mu.Unlock()
	for _, exact := range []string{
		"default-tea-one-static-site-onbex-co@kubernetes",
		"default-tea-one-static-www-example-com@kubernetes",
	} {
		if !strings.Contains(gotQuery, exact) {
			t.Errorf("query %q is missing exact router %q", gotQuery, exact)
		}
	}
	if !strings.Contains(gotQuery, `sum(increase(traefik_router_responses_bytes_total`) {
		t.Errorf("query does not use Prometheus reset-safe increase(): %q", gotQuery)
	}
	if strings.Contains(gotQuery, `service=~`) || strings.Contains(gotQuery, `.*`) {
		t.Errorf("query retained a broad service matcher: %q", gotQuery)
	}
}

func TestQueryEgressBytesEmptyVectorIsSuccessfulZero(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if query := r.URL.Query().Get("query"); strings.Contains(query, "healthy") || strings.Contains(query, `up{job=`) {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"1"]}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer prom.Close()
	app := store.App{ID: "srv-quiet", TenantID: "tea-one", Name: "quiet"}
	cl := publicAppMeterClient(t, app, "tea-one-quiet", "tea-one-quiet", "quiet.onbex.co")
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, nil, prom.URL, prom.Client())

	quantity, ok := svc.queryEgressBytes(context.Background(), app, time.Now().Add(-time.Hour), time.Now())
	if !ok || quantity != 0 {
		t.Fatalf("empty Prometheus vector: got quantity=%d ok=%v, want successful zero", quantity, ok)
	}
}

// The w1/m51 per-source hour gate: an unhealthy source is skipped (its
// possibly reset-inflated increase() never enters billing) while the healthy
// sources still record the hour — the old any-source-unhealthy-defers rule
// recorded nothing at all on prod (zero egress_bytes rows for July 2026,
// w1/034). Reverting to all-or-nothing deferral fails this test by name.
func TestQueryEgressBytesRecordsHealthySourcesAndSkipsUnhealthyOnes(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		value := "1"
		switch {
		case strings.Contains(query, "bex_websocket_meter_healthy"):
			value = "0" // the websocket source's health product fails this hour
		case strings.Contains(query, "increase(traefik_router_responses_bytes_total"):
			value = "1000"
		case strings.Contains(query, "increase(bex_websocket_egress_bytes_total"):
			value = "70000" // must NOT be billed — its source is unhealthy
		case strings.Contains(query, "increase(bex_app_direct_egress_bytes_total"):
			value = "500"
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,%q]}]}}`, value)
	}))
	defer prom.Close()
	app := store.App{ID: "srv-partial", TenantID: "tea-one", Name: "partial"}
	cl := publicAppMeterClient(t, app, "tea-one-partial", "tea-one-partial", "partial.onbex.co")
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, nil, prom.URL, prom.Client())

	quantity, ok := svc.queryEgressBytes(context.Background(), app, time.Now().Add(-time.Hour), time.Now())
	if !ok {
		t.Fatalf("partial source health must record the healthy remainder, not defer the hour")
	}
	if quantity != 1500 {
		t.Fatalf("healthy-source sum: got %d, want 1500 (http 1000 + direct 500, websocket skipped)", quantity)
	}
}

// A transport failure is the retryable condition — the hour still defers.
func TestQueryEgressBytesDefersTheHourOnTransportErrors(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "prometheus is down", http.StatusBadGateway)
	}))
	defer prom.Close()
	app := store.App{ID: "srv-transport", TenantID: "tea-one", Name: "transport"}
	cl := publicAppMeterClient(t, app, "tea-one-transport", "tea-one-transport", "transport.onbex.co")
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, nil, prom.URL, prom.Client())

	if quantity, ok := svc.queryEgressBytes(context.Background(), app, time.Now().Add(-time.Hour), time.Now()); ok || quantity != 0 {
		t.Fatalf("transport failure must defer the hour: quantity=%d ok=%v", quantity, ok)
	}
}

// Every source unhealthy records a successful zero — the gap-free cursor
// advances; the hour is final, never retried (its samples cannot improve).
func TestQueryEgressBytesAllSourcesUnhealthyRecordsSuccessfulZero(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		value := "1"
		if strings.Contains(query, "min_over_time(up{") {
			value = "0" // every source's up-term fails
		}
		if strings.Contains(query, "increase(") {
			value = "9999" // must not leak into the sum
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,%q]}]}}`, value)
	}))
	defer prom.Close()
	app := store.App{ID: "srv-dark", TenantID: "tea-one", Name: "dark"}
	cl := publicAppMeterClient(t, app, "tea-one-dark", "tea-one-dark", "dark.onbex.co")
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, nil, prom.URL, prom.Client())

	quantity, ok := svc.queryEgressBytes(context.Background(), app, time.Now().Add(-time.Hour), time.Now())
	if !ok || quantity != 0 {
		t.Fatalf("all-unhealthy hour must record a successful zero: quantity=%d ok=%v", quantity, ok)
	}
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

func TestProcessWindowPersistsSuccessfulZeroAnchors(t *testing.T) {
	app := store.App{ID: "srv-zero", TenantID: "tea-zero", Name: "quiet", Tier: "starter"}
	st := newMemUsageStore(app)
	prom := fakeProm(0)
	defer prom.Close()

	cl := appMeterClient(t, app)
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, st, prom.URL, prom.Client())
	window := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)

	svc.processWindow(context.Background(), app, window)

	for _, kind := range appMeterKinds {
		tier := ""
		if kind == store.UsageKindInstanceSeconds {
			tier = app.Tier
		}
		key := usageKey{store.ResourceKindService, app.ID, kind, tier, window}
		st.mu.Lock()
		row, ok := st.rows[key]
		st.mu.Unlock()
		if !ok || row.Quantity != 0 {
			t.Errorf("%s zero anchor: want present quantity=0, got %+v (present=%v)", kind, row, ok)
		}
	}

	// Zero egress/build rows are collector coverage anchors, not new API
	// entries. Preserve the existing summary behavior while instance_seconds
	// keeps its zero row for suspended/tiered services.
	rows, err := st.UsageMonthToDate(context.Background(), app.TenantID, window.Add(time.Hour))
	if err != nil {
		t.Fatalf("UsageMonthToDate: %v", err)
	}
	if len(rows) != 1 || rows[0].Kind != store.UsageKindInstanceSeconds || rows[0].Total != 0 {
		t.Fatalf("zero summary rows: want only instance_seconds=0, got %+v", rows)
	}
}

func TestProcessWindowUnavailableSourcesWriteNoFakeZero(t *testing.T) {
	app := store.App{ID: "srv-fail", TenantID: "tea-fail", Name: "unavailable", Tier: "starter"}
	st := newMemUsageStore(app)
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer prom.Close()

	// A nil Kubernetes client is unavailable, not a successful empty Job list.
	svc := NewService(&core.Base{}, st, prom.URL, prom.Client())
	svc.processWindow(context.Background(), app, time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC))

	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.rows) != 0 {
		t.Fatalf("unavailable sources wrote fake zero rows: %+v", st.rows)
	}
}

func TestAppMeterCatchUpRetriesFailedMeterWithoutBlockingHealthyMeters(t *testing.T) {
	app := store.App{ID: "srv-retry", TenantID: "tea-retry", Name: "retry", Tier: "starter"}
	st := newMemUsageStore(app)
	prom := fakeProm(0)
	defer prom.Close()

	cl := appMeterClient(t, app)
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, st, prom.URL, prom.Client())

	previous := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	for _, kind := range appMeterKinds {
		tier := ""
		if kind == store.UsageKindInstanceSeconds {
			tier = app.Tier
		}
		if err := st.UpsertUsageHourly(context.Background(), store.HourlyRow{
			WorkspaceID: app.TenantID, ServiceID: app.ID, ResourceKind: store.ResourceKindService,
			Kind: kind, Tier: tier, WindowStart: previous,
		}); err != nil {
			t.Fatalf("seed %s: %v", kind, err)
		}
	}

	var failMu sync.Mutex
	failed := false
	st.upsert = func(row store.HourlyRow) error {
		failMu.Lock()
		defer failMu.Unlock()
		if !failed && row.Kind == store.UsageKindEgressBytes && row.WindowStart.Equal(previous.Add(time.Hour)) {
			failed = true
			return errors.New("temporary store failure")
		}
		return nil
	}

	// Egress fails at 09:00. Instance/build must still advance to 09:00.
	svc.catchUpAppThrough(context.Background(), app, previous.Add(time.Hour))
	for _, kind := range []string{store.UsageKindInstanceSeconds, store.UsageKindBuildSeconds} {
		latest, err := st.LatestUsageWindow(context.Background(), store.ResourceKindService, app.ID, kind)
		if err != nil || !latest.Equal(previous.Add(time.Hour)) {
			t.Errorf("healthy %s cursor: want 09:00, got %v (err=%v)", kind, latest, err)
		}
	}
	egressLatest, _ := st.LatestUsageWindow(context.Background(), store.ResourceKindService, app.ID, store.UsageKindEgressBytes)
	if !egressLatest.Equal(previous) {
		t.Fatalf("failed egress cursor advanced: want %v, got %v", previous, egressLatest)
	}

	// The next pass retries egress 09:00 before 10:00, while healthy meters add
	// only their new 10:00 row. Idempotent keys leave a complete 3x3 grid.
	svc.catchUpAppThrough(context.Background(), app, previous.Add(2*time.Hour))
	for _, kind := range appMeterKinds {
		latest, err := st.LatestUsageWindow(context.Background(), store.ResourceKindService, app.ID, kind)
		if err != nil || !latest.Equal(previous.Add(2*time.Hour)) {
			t.Errorf("%s cursor after retry: want 10:00, got %v (err=%v)", kind, latest, err)
		}
	}
	st.mu.Lock()
	rowCount := len(st.rows)
	st.mu.Unlock()
	if rowCount != 9 {
		t.Fatalf("rows after retry: want 3 meters x 3 windows = 9, got %d", rowCount)
	}
}

func TestAppMeterCatchUpStartsAtAppCreationHour(t *testing.T) {
	created := time.Date(2026, 7, 14, 9, 30, 0, 0, time.UTC)
	app := store.App{
		ID: "srv-new", TenantID: "tea-new", Name: "new-app", Tier: "starter", CreatedAt: created,
	}
	st := newMemUsageStore(app)
	prom := fakeProm(0)
	defer prom.Close()

	cl := appMeterClient(t, app)
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, st, prom.URL, prom.Client())

	svc.catchUpAppThrough(context.Background(), app, created.Truncate(time.Hour).Add(time.Hour))

	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.rows) != 6 {
		t.Fatalf("new App rows: want 3 meters x creation/current hours = 6, got %d", len(st.rows))
	}
	for key := range st.rows {
		if key.windowStart.Before(created.Truncate(time.Hour)) {
			t.Errorf("synthetic pre-creation row: %+v", key)
		}
	}
}

func TestEgressCatchUpRecordsUnhealthyHoursAsZerosButRetriesTransportGaps(t *testing.T) {
	created := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	app := store.App{
		ID: "srv-origin", TenantID: "tea-origin", Name: "origin", Tier: "starter", CreatedAt: created,
	}
	st := newMemUsageStore(app)
	// The w1/m51 trichotomy along the cursor: health-failed hours (08:00,
	// 09:00 — every source's health product is 0) are FINAL and record
	// successful zeros (a past hour's samples never improve, w1/034);
	// measurable 10:00 records; a TRANSPORT-failed 11:00 (Prometheus 502) is
	// the retryable class and must stop the cursor before otherwise-healthy
	// 12:00 — the no-gap retry contract now applies to transport only.
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		at, _ := strconv.ParseInt(r.URL.Query().Get("time"), 10, 64)
		queryAt := time.Unix(at, 0).UTC()
		if queryAt.Equal(created.Add(4 * time.Hour)) { // the 11:00 window queries at 12:00
			http.Error(w, "prometheus is down", http.StatusBadGateway)
			return
		}
		healthy := queryAt.Equal(created.Add(3*time.Hour)) || queryAt.Equal(created.Add(5*time.Hour))
		value := "0"
		if strings.Contains(r.URL.Query().Get("query"), "healthy") || strings.Contains(r.URL.Query().Get("query"), `up{job=`) {
			if healthy {
				value = "1"
			}
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,%q]}]}}`, value)
	}))
	defer prom.Close()

	cl := publicAppMeterClient(t, app, "tea-origin-app", "tea-origin-app", "origin.onbex.co")
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, st, prom.URL, prom.Client())

	svc.catchUpAppMeterThrough(context.Background(), app, store.UsageKindEgressBytes, created.Add(2*time.Hour))
	latest, err := st.LatestUsageWindow(context.Background(), store.ResourceKindService, app.ID, store.UsageKindEgressBytes)
	if err != nil || !latest.Equal(created.Add(2*time.Hour)) {
		t.Fatalf("cursor after first pass: want 10:00, got %v (err=%v)", latest, err)
	}
	for hour := 0; hour <= 2; hour++ {
		st.mu.Lock()
		row, present := st.rows[usageKey{store.ResourceKindService, app.ID, store.UsageKindEgressBytes, "", created.Add(time.Duration(hour) * time.Hour)}]
		st.mu.Unlock()
		if !present || row.Quantity != 0 {
			t.Fatalf("hour +%dh: want a recorded zero row, got present=%v row=%+v", hour, present, row)
		}
	}

	svc.catchUpAppMeterThrough(context.Background(), app, store.UsageKindEgressBytes, created.Add(4*time.Hour))
	latest, err = st.LatestUsageWindow(context.Background(), store.ResourceKindService, app.ID, store.UsageKindEgressBytes)
	if err != nil || !latest.Equal(created.Add(2*time.Hour)) {
		t.Fatalf("transport gap advanced cursor: want 10:00, got %v (err=%v)", latest, err)
	}
	st.mu.Lock()
	_, skippedGap := st.rows[usageKey{store.ResourceKindService, app.ID, store.UsageKindEgressBytes, "", created.Add(4 * time.Hour)}]
	st.mu.Unlock()
	if skippedGap {
		t.Fatal("12:00 egress row was written across the transport-failed 11:00 window")
	}
}

func TestEgressCatchUpDoesNotSkipInitialStoreFailure(t *testing.T) {
	created := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	app := store.App{ID: "srv-store", TenantID: "tea-store", Name: "store", CreatedAt: created}
	st := newMemUsageStore(app)
	var attempted []time.Time
	st.upsert = func(row store.HourlyRow) error {
		if row.Kind == store.UsageKindEgressBytes {
			attempted = append(attempted, row.WindowStart)
			return errors.New("store unavailable")
		}
		return nil
	}
	prom := fakeProm(0)
	defer prom.Close()
	cl := appMeterClient(t, app)
	svc := NewService(&core.Base{Client: cl, Namespace: "default"}, st, prom.URL, prom.Client())

	svc.catchUpAppMeterThrough(context.Background(), app, store.UsageKindEgressBytes, created.Add(2*time.Hour))
	if len(attempted) != 1 || !attempted[0].Equal(created) {
		t.Fatalf("initial store failure was skipped: attempts=%v, want only %v", attempted, created)
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

func TestSummariseKeepsSameNameResourceKindsSeparate(t *testing.T) {
	rows := []store.UsageSummaryRow{
		{ServiceID: "shared", ResourceKind: store.ResourceKindPostgres, Kind: store.UsageKindInstanceSeconds, Tier: "free", Total: 3600},
		{ServiceID: "shared", ResourceKind: store.ResourceKindKeyValue, Kind: store.UsageKindInstanceSeconds, Tier: "free", Total: 1800},
	}

	summary := summarise("tea-shared", rows)
	if len(summary.Services) != 2 {
		t.Fatalf("same-name resources: want 2 service entries, got %+v", summary.Services)
	}
	totals := map[string]int64{}
	for _, resource := range summary.Services {
		if len(resource.Rows) != 1 {
			t.Fatalf("%s rows merged: %+v", resource.ResourceKind, resource.Rows)
		}
		totals[resource.ResourceKind] = resource.Rows[0].Total
	}
	if totals[store.ResourceKindPostgres] != 3600 || totals[store.ResourceKindKeyValue] != 1800 {
		t.Errorf("same-name totals merged or mislabeled: %+v", totals)
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
	april := st.monthly[monthKey{store.ResourceKindService, "srv-001", store.UsageKindInstanceSeconds, "starter", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}]
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
	hourlyAfter, aprilAfter := len(st.rows), st.monthly[monthKey{store.ResourceKindService, "srv-001", store.UsageKindInstanceSeconds, "starter", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}]
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
	before, err := svc.monthToDateAt(ctx, "", marchEnd)
	if err != nil {
		t.Fatalf("pre-compaction query: %v", err)
	}

	svc.compact(ctx) // clock is July 15 → March is outside the hot window

	after, err := svc.monthToDateAt(ctx, "", marchEnd)
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
	_, err := svc.MonthToDate(context.Background(), "")
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

	secs, ok := svc.queryBuildSeconds(context.Background(), "myapp", window, end)
	if !ok {
		t.Fatal("build_seconds: expected successful Job listing")
	}
	expected := int64(7 * 60) // 7 minutes
	if secs != expected {
		t.Errorf("build_seconds: expected %d, got %d", expected, secs)
	}
}

func TestSharedPromInstantParsesCorrectly(t *testing.T) {
	srv := fakeProm(42.5)
	defer srv.Close()

	v, err := egressquery.Instant(context.Background(), nil, srv.URL, `up`, time.Now())
	if err != nil {
		t.Fatalf("Instant: %v", err)
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

	k := usageKey{store.ResourceKindPostgres, "mydb", store.UsageKindInstanceSeconds, "basic-256mb", window}
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

func TestQueryStorageGBSecondsUsesPVCUsedBytes(t *testing.T) {
	tests := []struct {
		name, kind, wantPattern string
	}{
		{"postgres", store.ResourceKindPostgres, `^mydb-[0-9]+$`},
		{"key_value", store.ResourceKindKeyValue, `^data-mydb-[0-9]+$`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var query string
			prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query = r.URL.Query().Get("query")
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"2000000000"]}]}}`))
			}))
			defer prom.Close()

			svc := NewService(&core.Base{Namespace: "default"}, nil, prom.URL, prom.Client())
			start := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
			got, ok := svc.queryStorageGBSeconds(context.Background(), datastoreEntry{Name: "mydb", Kind: tt.kind}, start, start.Add(time.Hour))
			if !ok || got != 7200 { // 2 decimal GB × 3600 seconds
				t.Fatalf("storage GB-seconds: want 7200,true; got %d,%v", got, ok)
			}
			if !strings.Contains(query, "kubelet_volume_stats_used_bytes") || !strings.Contains(query, tt.wantPattern) {
				t.Errorf("unexpected storage query: %s", query)
			}
		})
	}
}

func TestProcessDatastoreWindowUpsertsStorage(t *testing.T) {
	prom := fakeProm(1_000_000_000) // 1 decimal GB average over an hour
	defer prom.Close()
	st := newMemUsageStore()
	svc := NewService(&core.Base{}, st, prom.URL, prom.Client())
	window := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	svc.processDatastoreWindow(context.Background(), datastoreEntry{
		ID: "mydb", Name: "mydb", TenantID: "tea-ds", Plan: "basic-256mb", Kind: store.ResourceKindPostgres,
	}, window)

	k := usageKey{store.ResourceKindPostgres, "mydb", store.UsageKindStorageGBSeconds, "", window}
	st.mu.Lock()
	row, ok := st.rows[k]
	st.mu.Unlock()
	if !ok || row.Quantity != 3600 {
		t.Fatalf("storage row: want 3600 GB-seconds, got %+v (present=%v)", row, ok)
	}
}

func TestPublicDatastoreEgressUsesExactProxyResource(t *testing.T) {
	var mu sync.Mutex
	var queries []string
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		mu.Lock()
		queries = append(queries, query)
		mu.Unlock()
		value := "1"
		if strings.Contains(query, "bex_pg_proxy_egress_bytes_total") {
			value = "17"
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,%q]}]}}`, value)
	}))
	defer prom.Close()
	svc := NewService(&core.Base{}, nil, prom.URL, prom.Client())
	window := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	got, ok := svc.queryDatastoreEgressBytes(context.Background(), datastoreEntry{
		ID: "db-one", Kind: store.ResourceKindPostgres, Public: true,
	}, window, window.Add(time.Hour))
	if !ok || got != 17 {
		t.Fatalf("public datastore egress = %d,%v; want 17,true", got, ok)
	}
	mu.Lock()
	query := strings.Join(queries, "\n")
	mu.Unlock()
	if !strings.Contains(query, `resource_id="db-one"`) || !strings.Contains(query, `resource_kind="postgres"`) {
		t.Fatalf("proxy query is not exact-resource attributed: %q", query)
	}
}

func TestDatastoreEgressCursorRetriesIndependently(t *testing.T) {
	var mu sync.Mutex
	failEgress := true
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if strings.Contains(query, "bex_pg_proxy_egress_bytes_total") {
			mu.Lock()
			fail := failEgress
			if failEgress {
				failEgress = false
			}
			mu.Unlock()
			if fail {
				http.Error(w, "temporary", http.StatusServiceUnavailable)
				return
			}
		}
		value := "1"
		if strings.Contains(query, "kubelet_volume_stats_used_bytes") {
			value = "1000000000"
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,%q]}]}}`, value)
	}))
	defer prom.Close()

	st := newMemUsageStore()
	svc := NewService(&core.Base{}, st, prom.URL, prom.Client())
	ds := datastoreEntry{ID: "db-one", Name: "db-one", TenantID: "tea-one", Plan: "basic", Kind: store.ResourceKindPostgres, Public: true}
	previous := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	for _, kind := range datastoreMeterKinds {
		tier := ""
		if kind == store.UsageKindInstanceSeconds {
			tier = ds.Plan
		}
		_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
			WorkspaceID: ds.TenantID, ServiceID: ds.ID, ResourceKind: ds.Kind,
			Kind: kind, Tier: tier, WindowStart: previous,
		})
	}

	svc.catchUpDatastoreThrough(context.Background(), ds, previous.Add(time.Hour))
	for _, kind := range []string{store.UsageKindInstanceSeconds, store.UsageKindStorageGBSeconds} {
		latest, err := st.LatestUsageWindow(context.Background(), ds.Kind, ds.ID, kind)
		if err != nil || !latest.Equal(previous.Add(time.Hour)) {
			t.Fatalf("healthy %s cursor did not advance: %s %v", kind, latest, err)
		}
	}
	latest, _ := st.LatestUsageWindow(context.Background(), ds.Kind, ds.ID, store.UsageKindEgressBytes)
	if !latest.Equal(previous) {
		t.Fatalf("failed egress cursor advanced to %s", latest)
	}
	svc.catchUpDatastoreThrough(context.Background(), ds, previous.Add(time.Hour))
	latest, _ = st.LatestUsageWindow(context.Background(), ds.Kind, ds.ID, store.UsageKindEgressBytes)
	if !latest.Equal(previous.Add(time.Hour)) {
		t.Fatalf("egress cursor did not retry and advance: %s", latest)
	}
}

func TestDatastoreStorageCatchUpRetriesWithoutGaps(t *testing.T) {
	var mu sync.Mutex
	storageCalls := 0
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		value := "1"
		if strings.Contains(query, "kubelet_volume_stats_used_bytes") {
			mu.Lock()
			storageCalls++
			call := storageCalls
			mu.Unlock()
			if call == 1 {
				http.Error(w, "temporary", http.StatusServiceUnavailable)
				return
			}
			value = "1000000000"
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,%q]}]}}`, value)
	}))
	defer prom.Close()

	st := newMemUsageStore()
	svc := NewService(&core.Base{}, st, prom.URL, prom.Client())
	ds := datastoreEntry{ID: "mydb", Name: "mydb", TenantID: "tea-ds", Plan: "basic-256mb", Kind: store.ResourceKindPostgres}
	previous := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	_ = st.UpsertUsageHourly(context.Background(), store.HourlyRow{
		WorkspaceID: ds.TenantID, ServiceID: ds.ID, ResourceKind: ds.Kind,
		Kind: store.UsageKindStorageGBSeconds, WindowStart: previous,
	})

	// First pass fails at 10:00 and must not advance the storage cursor.
	svc.catchUpDatastoreThrough(context.Background(), ds, previous.Add(time.Hour))
	// Next pass retries 10:00 before writing 11:00.
	svc.catchUpDatastoreThrough(context.Background(), ds, previous.Add(2*time.Hour))

	for _, window := range []time.Time{previous.Add(time.Hour), previous.Add(2 * time.Hour)} {
		key := usageKey{store.ResourceKindPostgres, "mydb", store.UsageKindStorageGBSeconds, "", window}
		st.mu.Lock()
		row, ok := st.rows[key]
		st.mu.Unlock()
		if !ok || row.Quantity != 3600 {
			t.Errorf("storage window %s: want 3600 GB-seconds, got %+v (present=%v)", window, row, ok)
		}
	}
}

// TestProcessDatastoreWindowPrivateEgressZero verifies that public-proxy egress
// is explicit N/A for a private datastore, represented by a successful zero;
// build remains service-only.
func TestProcessDatastoreWindowPrivateEgressZero(t *testing.T) {
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
	egress, found := st.rows[usageKey{store.ResourceKindKeyValue, "mykv", store.UsageKindEgressBytes, "", window}]
	if !found || egress.Quantity != 0 {
		t.Errorf("private datastore egress anchor: got %+v present=%v", egress, found)
	}
	for k := range st.rows {
		if k.serviceID == "mykv" && k.kind == store.UsageKindBuildSeconds {
			t.Errorf("unexpected build_seconds row for datastore mykv")
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
