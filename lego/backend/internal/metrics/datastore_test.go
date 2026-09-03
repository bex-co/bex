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
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
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

// staticDiskUsage serves a fixed disk-USED series regardless of the request.
// Capacity no longer routes through the source (w4/m91) — it is the datastore's
// logical size — so this only ever answers the used series.
func staticDiskUsage(used []MetricSeries) DiskUsageSource {
	return func(_ context.Context, _ DiskUsageRequest) ([]MetricSeries, error) {
		return used, nil
	}
}

func TestDatastoreDiskUsageForDatabaseAndKeyValue(t *testing.T) {
	used := []MetricSeries{{Labels: map[string]string{"instance": "pg-1"}, Points: []MetricPoint{{Value: 1 << 20}}}}
	svc := newService(nil, nil, sampleDatabase("pg", false), sampleKeyValue("kv"))
	svc.DiskUsage = staticDiskUsage(used)

	dbSeries, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricDisk})
	if err != nil || len(dbSeries) != 1 || dbSeries[0].Points[0].Value != 1<<20 {
		t.Fatalf("database disk usage: %v %+v", err, dbSeries)
	}
	// disk_capacity is now the datastore's logical StorageGB (the plan-default
	// size here), converted to bytes — not the physical PVC the source reports.
	kvSeries, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "kv", Metric: MetricDiskCapacity})
	wantKV := float64(tiers.Valkey.Default().StorageGB) * bytesPerGiB
	if err != nil || len(kvSeries) != 1 || kvSeries[0].Points[0].Value != wantKV {
		t.Fatalf("keyvalue disk capacity: %v %+v (want %v)", err, kvSeries, wantKV)
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
	svc.DiskUsage = staticDiskUsage([]MetricSeries{{Labels: map[string]string{"instance": "pg-1"}, Unit: unitBytes, Points: []MetricPoint{{Value: 5}}}})

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
		Namespace: "default", Resource: "pg", PVCPattern: `^pg-\d+$`,
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

// --- w4/m91: disk_capacity is the logical/billed size, not the physical PVC ---

// pgWithStorage builds a Database with an explicit plan, spec override, and
// allocated high-water so the logical-size resolution can be pinned exactly.
func pgWithStorage(name, plan string, specGB, allocatedGB int32) *appv1alpha1.Database {
	d := sampleDatabase(name, false)
	d.Spec.Plan = plan
	d.Spec.StorageGB = specGB
	d.Status.AllocatedStorageGB = allocatedGB
	return d
}

// physicalPVCSource answers any disk read with a fixed "physical PVC" capacity —
// the pre-w4/m91 kubelet_volume_stats_capacity_bytes value (a fixed 10 GiB
// Hetzner floor). Wiring it and still getting the logical size proves
// disk_capacity never reads the source: if it regressed to the kubelet series,
// every assertion below would see this bogus value instead.
func physicalPVCSource(capacityBytes float64) DiskUsageSource {
	return func(_ context.Context, _ DiskUsageRequest) ([]MetricSeries, error) {
		return []MetricSeries{{Labels: map[string]string{"instance": "pvc-0"}, Unit: unitBytes, Points: []MetricPoint{{Value: capacityBytes}}}}, nil
	}
}

func TestDiskCapacityIsLogicalSizeNotPhysicalPVC(t *testing.T) {
	const gib = float64(bytesPerGiB)
	cases := []struct {
		name    string
		db      *appv1alpha1.Database
		wantGiB float64
	}{
		{"basic-256mb reads its 1 GiB floor", pgWithStorage("pg1", "basic-256mb", 0, 0), 1},
		{"basic-1gb reads its 5 GiB floor", pgWithStorage("pg5", "basic-1gb", 0, 0), 5},
		{"spec override grows past the plan floor", pgWithStorage("pgo", "basic-256mb", 20, 0), 20},
		{"autoscaled allocated high-water wins", pgWithStorage("pga", "basic-1gb", 5, 15), 15},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(nil, nil, tc.db)
			svc.DiskUsage = physicalPVCSource(10 * gib) // bogus physical value; must be ignored
			series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
				Kind: DatastoreDatabase, Resource: tc.db.Name, Metric: MetricDiskCapacity,
			})
			if err != nil || len(series) != 1 || len(series[0].Points) != 1 {
				t.Fatalf("disk_capacity: %v %+v", err, series)
			}
			if got := series[0].Points[0].Value; got != tc.wantGiB*gib {
				t.Errorf("disk_capacity = %v bytes, want %v GiB (%v bytes) — not the 10 GiB physical PVC", got, tc.wantGiB, tc.wantGiB*gib)
			}
			if series[0].Unit != unitBytes {
				t.Errorf("unit = %q, want %q", series[0].Unit, unitBytes)
			}
		})
	}
}

// disk_capacity is config-shaped: it needs no Prometheus source (like cpu_limit).
// disk (USED), which was always accurate, still requires the kubelet source.
func TestDiskCapacityNeedsNoSourceButUsedStillDoes(t *testing.T) {
	svc := newService(nil, nil, pgWithStorage("pg", "basic-1gb", 0, 0)) // DiskUsage nil
	series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
		Kind: DatastoreDatabase, Resource: "pg", Metric: MetricDiskCapacity,
	})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 5*float64(bytesPerGiB) {
		t.Fatalf("disk_capacity without a source should be the logical size: %v %+v", err, series)
	}
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
		Kind: DatastoreDatabase, Resource: "pg", Metric: MetricDisk,
	}); err != core.ErrMetricsUnavailable {
		t.Errorf("disk (used) without a source => ErrMetricsUnavailable, got %v", err)
	}
}

// KeyValue rides the same logical-size path: capacity equals its StorageGB, not
// the physical data-<name> PVC.
func TestKeyValueDiskCapacityIsLogicalStorage(t *testing.T) {
	kv := sampleKeyValue("cache")
	kv.Spec.Plan = "free" // 1 GiB floor
	kv.Spec.StorageGB = 3 // grown past it
	svc := newService(nil, nil, kv)
	svc.DiskUsage = physicalPVCSource(10 * float64(bytesPerGiB))
	series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
		Kind: DatastoreKeyValue, Resource: "cache", Metric: MetricDiskCapacity,
	})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 3*float64(bytesPerGiB) {
		t.Fatalf("kv disk_capacity should equal the logical StorageGB: %v %+v", err, series)
	}
}

// A service's attached disk (ADR082) shares the same split, so its capacity is
// spec.disk.sizeGB (grown by the allocated high-water), not the 10 GiB PVC.
func TestServiceDiskCapacityIsLogicalSize(t *testing.T) {
	app := sampleAppWithDisk("web", &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1})
	app.Status.Disk = &appv1alpha1.DiskStatus{AllocatedSizeGB: 4} // autoscaled up
	svc := newService(nil, nil, app)
	svc.DiskUsage = physicalPVCSource(10 * float64(bytesPerGiB))
	series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
		Kind: DatastoreService, Resource: "web", Metric: MetricDiskCapacity,
	})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 4*float64(bytesPerGiB) {
		t.Fatalf("service disk_capacity should be the logical high-water, not the 10 GiB PVC: %v %+v", err, series)
	}
}

// All three API surfaces resolve disk_capacity through the same verb, so they
// must return byte-identical logical bytes (DoD: REST/GraphQL/MCP agree).
func TestDiskCapacityIdenticalAcrossRESTGraphQLMCP(t *testing.T) {
	svc := newService(nil, nil, pgWithStorage("pg", "basic-1gb", 0, 0)) // 5 GiB
	want := 5 * float64(bytesPerGiB)

	// Service verb — the shared seam every adapter calls.
	series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
		Kind: DatastoreDatabase, Resource: "pg", Metric: MetricDiskCapacity,
	})
	if err != nil || len(series) != 1 || series[0].Points[0].Value != want {
		t.Fatalf("verb disk_capacity: %v %+v", err, series)
	}

	// REST.
	var rest []renderMetricSeries
	rec := serveREST(svc, "/v1/metrics/disk-capacity?resource=pg&kind=database")
	if rec.Code != 200 {
		t.Fatalf("REST disk-capacity => %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &rest)
	if len(rest) != 1 || rest[0].Values[0].Value != want {
		t.Fatalf("REST disk_capacity = %+v, want %v", rest, want)
	}

	// GraphQL.
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ datastoreMetrics(query: {resource: "pg", name: "DISK_CAPACITY"}) { unit values { value } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	gqlFirst := res.Data.(map[string]any)["datastoreMetrics"].([]any)[0].(map[string]any)
	if gqlFirst["values"].([]any)[0].(map[string]any)["value"].(float64) != want {
		t.Errorf("GraphQL disk_capacity = %+v, want %v", gqlFirst, want)
	}

	// MCP.
	cs := mcpSession(t, svc)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_datastore_metrics", Arguments: map[string]any{
		"resource": "pg", "kind": "database", "metricTypes": []string{MetricDiskCapacity},
	}})
	if err != nil || result.IsError {
		t.Fatalf("MCP get_datastore_metrics: err %v isError %v", err, result != nil && result.IsError)
	}
	var mcpOut getMetricsResult
	b, _ := json.Marshal(result.StructuredContent)
	if err := json.Unmarshal(b, &mcpOut); err != nil {
		t.Fatalf("MCP structured content: %v", err)
	}
	if len(mcpOut.Series) != 1 || mcpOut.Series[0].Points[0].Value != want {
		t.Fatalf("MCP disk_capacity = %+v, want %v", mcpOut, want)
	}
}
