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

package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func sampleAppWithAutoscaling(name string, as *appv1alpha1.AutoscalingSpec) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Autoscaling = as
	return a
}

func int32p(v int32) *int32 { return &v }

func sampleDatabase(name string, ha bool) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Status:     appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady, HighAvailabilityEnabled: ha},
	}
}

func sampleKeyValue(name string) *appv1alpha1.KeyValue {
	return &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
	}
}

// --- t001: autoscale-target ---

func TestAutoscaleTargetPresentAndOmitted(t *testing.T) {
	svc := newService(nil, nil,
		sampleAppWithAutoscaling("web", &appv1alpha1.AutoscalingSpec{
			Enabled: true, TargetCPUPercent: int32p(70), TargetMemoryPercent: int32p(80),
		}),
		sampleApp("plain"),
		sampleAppWithAutoscaling("disabled", &appv1alpha1.AutoscalingSpec{Enabled: false, TargetCPUPercent: int32p(70)}),
		sampleAppWithAutoscaling("cpu-only", &appv1alpha1.AutoscalingSpec{Enabled: true, TargetCPUPercent: int32p(60)}),
	)

	cpu, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPUTarget})
	if err != nil || len(cpu) != 1 || cpu[0].Unit != unitPercentage || cpu[0].Points[0].Value != 70 {
		t.Fatalf("cpu_target with autoscaling enabled: %v %+v", err, cpu)
	}
	mem, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricMemoryTarget})
	if err != nil || len(mem) != 1 || mem[0].Points[0].Value != 80 {
		t.Fatalf("memory_target with autoscaling enabled: %v %+v", err, mem)
	}

	// No autoscaling configured at all => omitted, not a fake zero.
	none, err := svc.Metrics(context.Background(), MetricQuery{App: "plain", Metric: MetricCPUTarget})
	if err != nil || len(none) != 0 {
		t.Errorf("no autoscaling spec should omit cpu_target: %v %+v", err, none)
	}

	// Autoscaling configured but disabled => omitted.
	disabled, err := svc.Metrics(context.Background(), MetricQuery{App: "disabled", Metric: MetricCPUTarget})
	if err != nil || len(disabled) != 0 {
		t.Errorf("disabled autoscaling should omit cpu_target: %v %+v", err, disabled)
	}

	// CPU target configured, memory target not => memory_target omitted.
	cpuOnlyMem, err := svc.Metrics(context.Background(), MetricQuery{App: "cpu-only", Metric: MetricMemoryTarget})
	if err != nil || len(cpuOnlyMem) != 0 {
		t.Errorf("unset memory target should be omitted, not faked: %v %+v", err, cpuOnlyMem)
	}
}

// --- t002: disk usage ---

func staticDiskUsage(byPattern map[string][]MetricSeries) DiskUsageSource {
	return func(_ context.Context, req DiskUsageRequest) ([]MetricSeries, error) {
		return byPattern[req.Metric], nil
	}
}

func TestDatastoreDiskUsageForDatabaseAndKeyValue(t *testing.T) {
	used := []MetricSeries{{Labels: map[string]string{"instance": "pg-1"}, Points: []MetricPoint{{Value: 1 << 20}}}}
	capacity := []MetricSeries{{Labels: map[string]string{"instance": "pg-1"}, Points: []MetricPoint{{Value: 10 << 20}}}}
	svc := newService(nil, nil, sampleDatabase("pg", false), sampleKeyValue("kv"))
	svc.DiskUsage = staticDiskUsage(map[string][]MetricSeries{MetricDisk: used, MetricDiskCapacity: capacity})

	dbSeries, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricDisk})
	if err != nil || len(dbSeries) != 1 || dbSeries[0].Points[0].Value != 1<<20 {
		t.Fatalf("database disk usage: %v %+v", err, dbSeries)
	}
	kvSeries, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "kv", Metric: MetricDiskCapacity})
	if err != nil || len(kvSeries) != 1 || kvSeries[0].Points[0].Value != 10<<20 {
		t.Fatalf("keyvalue disk capacity: %v %+v", err, kvSeries)
	}
}

func TestDatastoreDiskUsagePVCPattern(t *testing.T) {
	if got, want := pvcPattern(DatastoreDatabase, "pg"), `^pg-\d+$`; got != want {
		t.Errorf("database pvc pattern = %q, want %q", got, want)
	}
	if got, want := pvcPattern(DatastoreKeyValue, "kv"), `^data-kv-\d+$`; got != want {
		t.Errorf("keyvalue pvc pattern = %q, want %q", got, want)
	}
}

func TestDatastoreMetricsUnavailableWithoutSource(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("pg", false))
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricDisk}); err != core.ErrMetricsUnavailable {
		t.Errorf("disk without source => ErrMetricsUnavailable, got %v", err)
	}
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "nope", Metric: MetricDisk}); err != core.ErrNotFound {
		t.Errorf("unknown database => ErrNotFound, got %v", err)
	}
}

// --- t003: DB active connections ---

func TestDatastoreDBConnectionsRealData(t *testing.T) {
	var got DBConnectionsRequest
	svc := newService(nil, nil, sampleDatabase("pg", false))
	svc.DBConnections = func(_ context.Context, req DBConnectionsRequest) ([]MetricSeries, error) {
		got = req
		return []MetricSeries{{Labels: map[string]string{"instance": "pg-1"}, Unit: unitCount, Points: []MetricPoint{{Value: 12}}}}, nil
	}

	series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricDBConnections})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 12 || series[0].Unit != unitCount {
		t.Fatalf("db_connections: %v %+v", err, series)
	}
	if got.Cluster != "pg" {
		t.Errorf("source should receive the Database name as Cluster, got %q", got.Cluster)
	}
}

func TestDatastoreDBConnectionsNotValidForKeyValue(t *testing.T) {
	svc := newService(nil, nil, sampleKeyValue("kv"))
	svc.DBConnections = func(context.Context, DBConnectionsRequest) ([]MetricSeries, error) {
		t.Fatal("db_connections source should never be called for a keyvalue resource")
		return nil, nil
	}
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "kv", Metric: MetricDBConnections}); err == nil {
		t.Error("db_connections on a keyvalue resource should error")
	}
}

// --- w5/011: Key Value memory + connections (redis_exporter) ---

func TestDatastoreKeyValueMemoryAndConnections(t *testing.T) {
	var got KeyValueStatsRequest
	svc := newService(nil, nil, sampleKeyValue("cache"))
	svc.KeyValueStats = func(_ context.Context, req KeyValueStatsRequest) ([]MetricSeries, error) {
		got = req
		unit := unitBytes
		if req.Dimension == "connections" {
			unit = unitCount
		}
		return []MetricSeries{{Labels: map[string]string{"instance": "cache-0"}, Unit: unit, Points: []MetricPoint{{Value: 4096}}}}, nil
	}

	mem, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "cache", Metric: MetricKVMemory})
	if err != nil || len(mem) != 1 || mem[0].Unit != unitBytes {
		t.Fatalf("kv_memory: %v %+v", err, mem)
	}
	if got.Dimension != "memory" || got.Resource != "cache" {
		t.Errorf("memory request = %+v", got)
	}

	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "cache", Metric: MetricKVConnections}); err != nil {
		t.Fatalf("kv_connections: %v", err)
	}
	if got.Dimension != "connections" {
		t.Errorf("connections dimension = %q", got.Dimension)
	}
}

func TestDatastoreKeyValueMetricsNotValidForDatabase(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("pg", false))
	svc.KeyValueStats = func(context.Context, KeyValueStatsRequest) ([]MetricSeries, error) {
		t.Fatal("kv stats source should never be called for a database resource")
		return nil, nil
	}
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricKVMemory}); err == nil {
		t.Error("kv_memory on a database resource should error")
	}
}

func TestDatastoreKeyValueMetricsUnavailableWithoutSource(t *testing.T) {
	svc := newService(nil, nil, sampleKeyValue("cache")) // KeyValueStats nil
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "cache", Metric: MetricKVConnections}); err != core.ErrMetricsUnavailable {
		t.Errorf("no source => ErrMetricsUnavailable, got %v", err)
	}
}

// --- t004: replication lag, gated on HighAvailabilityEnabled ---

func TestReplicationLagOmittedWithoutHA(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("pg", false)) // HighAvailabilityEnabled: false — today, always
	svc.ReplicationLag = func(context.Context, ReplicationLagRequest) ([]MetricSeries, error) {
		t.Fatal("replication-lag source should never be called pre-HA — that's exactly the fake-zero this gate avoids")
		return nil, nil
	}
	series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricReplicationLag})
	if err != nil || len(series) != 0 {
		t.Fatalf("replication_lag pre-HA should be omitted (nil, no error): %v %+v", err, series)
	}
}

func TestReplicationLagServedOnceHAEnabled(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("pg", true)) // HighAvailabilityEnabled: true
	svc.ReplicationLag = func(_ context.Context, req ReplicationLagRequest) ([]MetricSeries, error) {
		return []MetricSeries{{Unit: unitSeconds, Points: []MetricPoint{{Value: 0.4}}}}, nil
	}
	series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricReplicationLag})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 0.4 {
		t.Fatalf("replication_lag once HA is enabled: %v %+v", err, series)
	}
}

func TestReplicationLagUnavailableWhenHAButNoSource(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("pg", true))
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricReplicationLag}); err != core.ErrMetricsUnavailable {
		t.Errorf("HA on but no source => ErrMetricsUnavailable, got %v", err)
	}
}

// --- w5/m71: matchers derive from the resolved CR's metadata.name ---

// The datastore-metrics identifier contract: `resource` is the CR name (the
// typed id), and every Prometheus matcher is built from the authorized CR's
// metadata.name — never from a display name or a raw input the finder didn't
// resolve. These tests pin both halves: an id-named CR yields id-derived
// matchers, and a display-name input fails closed (an explicit error, not the
// silent empty series the dashboard's empty charts hid — w5/044).
func TestDatastoreMatchersUseResolvedCRName(t *testing.T) {
	kv := sampleKeyValue("red-cache")
	kv.Spec.Name = "sessions-cache" // display name must never reach a matcher
	var gotDisk DiskUsageRequest
	var gotStats KeyValueStatsRequest
	svc := newService(nil, nil, kv)
	svc.DiskUsage = func(_ context.Context, req DiskUsageRequest) ([]MetricSeries, error) {
		gotDisk = req
		return nil, nil
	}
	svc.KeyValueStats = func(_ context.Context, req KeyValueStatsRequest) ([]MetricSeries, error) {
		gotStats = req
		return nil, nil
	}

	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "red-cache", Metric: MetricDisk}); err != nil {
		t.Fatalf("disk by id: %v", err)
	}
	if gotDisk.Resource != "red-cache" || gotDisk.PVCPattern != `^data-red-cache-\d+$` {
		t.Errorf("disk request = %+v, want resource red-cache + id-derived pvc pattern", gotDisk)
	}
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "red-cache", Metric: MetricKVMemory}); err != nil {
		t.Fatalf("kv_memory by id: %v", err)
	}
	if gotStats.Resource != "red-cache" {
		t.Errorf("kv stats request resource = %q, want the CR name red-cache", gotStats.Resource)
	}
}

func TestDatastoreDisplayNameFailsClosed(t *testing.T) {
	kv := sampleKeyValue("red-cache")
	kv.Spec.Name = "sessions-cache"
	svc := newService(nil, nil, kv)
	svc.DiskUsage = func(context.Context, DiskUsageRequest) ([]MetricSeries, error) {
		t.Fatal("a display-name input must never reach a source — fail closed before querying")
		return nil, nil
	}
	// The original dashboard bug (w5/044) sent Spec.Name here; it must be an
	// explicit lookup error, not silently-empty series.
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "sessions-cache", Metric: MetricDisk}); err == nil {
		t.Error("display-name resource should error (not empty series)")
	}
}

// --- REST fragment ---

func TestRESTDatastoreMetrics(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("pg", false))
	svc.DiskUsage = staticDiskUsage(map[string][]MetricSeries{
		MetricDisk: {{Labels: map[string]string{"instance": "pg-1"}, Unit: unitBytes, Points: []MetricPoint{{Value: 5}}}},
	})

	var series []renderMetricSeries
	rec := serveREST(svc, "/v1/metrics/disk?resource=pg")
	if rec.Code != 200 {
		t.Fatalf("disk?resource=pg => %d, body %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &series)
	if len(series) != 1 || series[0].Values[0].Value != 5 {
		t.Fatalf("disk series: %+v", series)
	}

	if serveREST(svc, "/v1/metrics/disk").Code != 400 {
		t.Error("missing resource => 400")
	}
	if serveREST(svc, "/v1/metrics/disk?resource=nope").Code != 404 {
		t.Error("unknown database => 404")
	}
}

// --- GraphQL fragment ---

func TestGraphQLDatastoreMetrics(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("pg", false))
	svc.DBConnections = func(context.Context, DBConnectionsRequest) ([]MetricSeries, error) {
		return []MetricSeries{{Unit: unitCount, Points: []MetricPoint{{Value: 3}}}}, nil
	}
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ datastoreMetrics(query: {resource: "pg", name: "DB_CONNECTIONS"}) { unit values { value } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	first := res.Data.(map[string]any)["datastoreMetrics"].([]any)[0].(map[string]any)
	if first["unit"] != unitCount || first["values"].([]any)[0].(map[string]any)["value"].(float64) != 3 {
		t.Errorf("db connections via graphql: %+v", first)
	}
}

// --- Prometheus source builders (no live backend) ---

func TestPrometheusDBConnectionsRoundTrip(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"pod":"pg-1"},"values":[[1000000,"3"]]}]}}`))
	}))
	defer ts.Close()
	series, err := NewPrometheusDBConnectionsSource(ts.URL, ts.Client())(context.Background(), DBConnectionsRequest{
		Namespace: "default", Cluster: "pg", Start: time.Unix(1_000_000, 0), End: time.Unix(1_000_120, 0), Resolution: 60 * time.Second,
	})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 3 || series[0].Labels["instance"] != "pg-1" || series[0].Unit != unitCount {
		t.Fatalf("db connections roundtrip: %v %+v", err, series)
	}
}

func TestPrometheusReplicationLagRoundTrip(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"pod":"pg-2"},"values":[[1000000,"0.4"]]}]}}`))
	}))
	defer ts.Close()
	series, err := NewPrometheusReplicationLagSource(ts.URL, ts.Client())(context.Background(), ReplicationLagRequest{
		Namespace: "default", Cluster: "pg", Start: time.Unix(1_000_000, 0), End: time.Unix(1_000_120, 0), Resolution: 60 * time.Second,
	})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 0.4 || series[0].Unit != unitSeconds {
		t.Fatalf("replication lag roundtrip: %v %+v", err, series)
	}
}

func TestPrometheusDiskUsageRoundTrip(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"persistentvolumeclaim":"pg-1"},"values":[[1000000,"1048576"]]}]}}`))
	}))
	defer ts.Close()
	series, err := NewPrometheusDiskUsageSource(ts.URL, ts.Client())(context.Background(), DiskUsageRequest{
		Namespace: "default", Resource: "pg", PVCPattern: `^pg-\d+$`, Metric: MetricDisk,
		Start: time.Unix(1_000_000, 0), End: time.Unix(1_000_120, 0), Resolution: 60 * time.Second,
	})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 1048576 || series[0].Labels["instance"] != "pg-1" || series[0].Unit != unitBytes {
		t.Fatalf("disk usage roundtrip: %v %+v", err, series)
	}
}

func TestPrometheusKeyValueStatsRoundTrip(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"pod":"cache-0"},"values":[[1000000,"4096"]]}]}}`))
	}))
	defer ts.Close()
	series, err := NewPrometheusKeyValueStatsSource(ts.URL, ts.Client())(context.Background(), KeyValueStatsRequest{
		Namespace: "default", Resource: "cache", Dimension: "memory",
		Start: time.Unix(1_000_000, 0), End: time.Unix(1_000_120, 0), Resolution: 60 * time.Second,
	})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 4096 || series[0].Labels["instance"] != "cache-0" || series[0].Labels["resource"] != "cache" || series[0].Unit != unitBytes {
		t.Fatalf("kv stats roundtrip: %v %+v", err, series)
	}
	if _, still := series[0].Labels["pod"]; still {
		t.Errorf("raw pod label should not leak: %+v", series[0].Labels)
	}
	if !strings.Contains(gotQuery, "redis_memory_used_bytes") || !strings.Contains(gotQuery, `pod=~"cache-[0-9]+"`) {
		t.Errorf("unexpected query: %q", gotQuery)
	}
}

// --- w1/m86: a service's attached persistent disk (ADR082 D6) ---

func sampleAppWithDisk(name string, disk *appv1alpha1.DiskSpec) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Disk = disk
	return a
}

// The claim a service's disk graph reads must be the claim the operator
// created. Both sides derive it from appv1alpha1.DiskPVCName, so this asserts
// the pattern the source actually receives — including for a name long enough
// to be truncated, which is where a hand-copied second spelling of the rule
// would silently return an empty graph instead of an error.
func TestDatastoreServiceDiskQueriesTheOperatorsClaimName(t *testing.T) {
	for _, name := range []string{"web", strings.Repeat("a", 70)} {
		var got DiskUsageRequest
		svc := newService(nil, nil, sampleAppWithDisk(name, &appv1alpha1.DiskSpec{
			Name: "data", MountPath: "/var/data", SizeGB: 10,
		}))
		svc.DiskUsage = func(_ context.Context, req DiskUsageRequest) ([]MetricSeries, error) {
			got = req
			return []MetricSeries{{Labels: map[string]string{"instance": req.PVCPattern}, Unit: unitBytes, Points: []MetricPoint{{Value: 2048}}}}, nil
		}

		series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
			Kind: DatastoreService, Resource: name, Metric: MetricDisk,
		})
		if err != nil || len(series) != 1 || series[0].Points[0].Value != 2048 {
			t.Fatalf("service disk: %v %+v", err, series)
		}
		want := `^` + regexp.QuoteMeta(appv1alpha1.DiskPVCName(name)) + `$`
		if got.PVCPattern != want {
			t.Errorf("pattern for %q: got %q, want %q", name, got.PVCPattern, want)
		}
		// No ordinal suffix: a disk-bearing service is capped at one instance,
		// so there is exactly one claim (unlike a datastore's `-\d+`).
		if strings.Contains(got.PVCPattern, `\d`) {
			t.Errorf("service disk pattern should name one claim, got %q", got.PVCPattern)
		}
		if got.Resource != name {
			t.Errorf("source should receive the app name, got %q", got.Resource)
		}
	}
}

// A diskless service is not an error — the Disk tab asks for the graph before
// it knows whether a disk exists, and a 4xx there would render as a failure
// rather than "no disk yet".
func TestDatastoreServiceWithoutADiskReportsNoSeries(t *testing.T) {
	svc := newService(nil, nil, sampleAppWithDisk("web", nil))
	svc.DiskUsage = func(context.Context, DiskUsageRequest) ([]MetricSeries, error) {
		t.Fatal("a service with no disk should never reach the PVC source")
		return nil, nil
	}
	series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
		Kind: DatastoreService, Resource: "web", Metric: MetricDisk,
	})
	if err != nil || series != nil {
		t.Fatalf("diskless service => (nil, nil), got %v %+v", err, series)
	}
}

// A service has no datastore process, so the datastore-process metrics are a
// caller error rather than an empty answer.
func TestDatastoreServiceRejectsNonDiskMetrics(t *testing.T) {
	svc := newService(nil, nil, sampleAppWithDisk("web", &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}))
	for _, m := range []string{MetricDBConnections, MetricReplicationLag, MetricKVMemory, MetricKVConnections} {
		if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
			Kind: DatastoreService, Resource: "web", Metric: m,
		}); err == nil {
			t.Errorf("%s on a service resource should error", m)
		}
	}
}

// An unknown service is NotFound, not an empty graph — the same shape the
// Database path returns, so the dashboard's error handling is uniform.
func TestDatastoreServiceUnknownResourceIsNotFound(t *testing.T) {
	svc := newService(nil, nil, sampleAppWithDisk("web", nil))
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
		Kind: DatastoreService, Resource: "nope", Metric: MetricDisk,
	}); err != core.ErrNotFound {
		t.Errorf("unknown service => ErrNotFound, got %v", err)
	}
}
