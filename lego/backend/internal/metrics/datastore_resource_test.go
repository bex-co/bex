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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w1/m180: Render scopes cpu, memory and their limits to Postgres and Key
// Value ids. They are the datastore's own container's series.

func recordingResourceSource(asked *[]DatastoreResourceRequest) DatastoreResourceSource {
	return func(_ context.Context, req DatastoreResourceRequest) ([]MetricSeries, error) {
		*asked = append(*asked, req)
		return []MetricSeries{{
			Labels: map[string]string{"instance": req.Resource + "-1"}, Unit: resourceUnit(req.Metric),
			Points: []MetricPoint{{Value: 0.25}},
		}}, nil
	}
}

func TestDatastoreResourceMetricsReadTheDatastoreContainer(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("dpg-pg", false), sampleKeyValue("red-kv"))
	var asked []DatastoreResourceRequest
	svc.DatastoreResource = recordingResourceSource(&asked)
	kvMemoryAsked := false
	svc.KeyValueStats = func(context.Context, KeyValueStatsRequest) ([]MetricSeries, error) {
		kvMemoryAsked = true
		return []MetricSeries{{Unit: unitBytes, Points: []MetricPoint{{Value: 4096}}}}, nil
	}

	for _, tc := range []struct {
		kind, resource, metric, container, unit string
	}{
		{DatastoreDatabase, "dpg-pg", MetricCPU, postgresContainer, unitCores},
		{DatastoreDatabase, "dpg-pg", MetricMemory, postgresContainer, unitBytes},
		{DatastoreDatabase, "dpg-pg", MetricCPULimit, postgresContainer, unitCores},
		{DatastoreDatabase, "dpg-pg", MetricMemoryLimit, postgresContainer, unitBytes},
		{DatastoreKeyValue, "red-kv", MetricCPU, keyValueContainer, unitCores},
		{DatastoreKeyValue, "red-kv", MetricCPULimit, keyValueContainer, unitCores},
		{DatastoreKeyValue, "red-kv", MetricMemoryLimit, keyValueContainer, unitBytes},
	} {
		asked = nil
		series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: tc.kind, Resource: tc.resource, Metric: tc.metric})
		if err != nil || len(series) != 1 || series[0].Unit != tc.unit {
			t.Fatalf("%s %s = %+v, %v", tc.metric, tc.resource, series, err)
		}
		if len(asked) != 1 || asked[0].Container != tc.container || asked[0].Resource != tc.resource ||
			asked[0].Namespace != "default" || asked[0].Metric != tc.metric {
			t.Errorf("%s %s asked %+v, want the %s container", tc.metric, tc.resource, asked, tc.container)
		}
	}

	// A Key Value's memory stays its used-memory series (w8/061), not the
	// container's working set.
	asked = nil
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreKeyValue, Resource: "red-kv", Metric: MetricMemory}); err != nil {
		t.Fatal(err)
	}
	if !kvMemoryAsked || len(asked) != 0 {
		t.Errorf("Key Value memory read kv_memory=%v container=%+v, want kv_memory only", kvMemoryAsked, asked)
	}

	svc.DatastoreResource = nil
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "dpg-pg", Metric: MetricCPU}); !errors.Is(err, core.ErrMetricsUnavailable) {
		t.Errorf("no source = %v, want ErrMetricsUnavailable", err)
	}
}

func TestPromDatastoreResourceQueryScopesToTheInstanceContainer(t *testing.T) {
	base := DatastoreResourceRequest{Namespace: "tea-x", Resource: "red-a.b", Container: keyValueContainer}
	for metric, want := range map[string][]string{
		MetricCPU:         {"rate(container_cpu_usage_seconds_total{", "max by (pod, container)"},
		MetricMemory:      {"container_memory_working_set_bytes{", "max by (pod, container)"},
		MetricCPULimit:    {"kube_pod_container_resource_limits{", `resource="cpu"`},
		MetricMemoryLimit: {"kube_pod_container_resource_limits{", `resource="memory"`},
	} {
		req := base
		req.Metric = metric
		q := promDatastoreResourceQuery(req)
		for _, part := range append(want, `namespace="tea-x"`, `pod=~"red-a\\.b-[0-9]+"`, `container="valkey"`) {
			if !strings.Contains(q, part) {
				t.Errorf("%s query %s lacks %s", metric, q, part)
			}
		}
	}
}

func TestGraphQLDatastoreResourceNames(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("dpg-pg", false), sampleKeyValue("red-kv"))
	var asked []DatastoreResourceRequest
	svc.DatastoreResource = recordingResourceSource(&asked)
	svc.KeyValueStats = func(context.Context, KeyValueStatsRequest) ([]MetricSeries, error) {
		return []MetricSeries{{Unit: unitBytes, Points: []MetricPoint{{Value: 4096}}}}, nil
	}
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	query := func(kind, resource, name string) map[string]any {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
			RequestString: `{ datastoreMetrics(query: {kind: "` + kind + `", resource: "` + resource + `", name: "` + name + `"}) { unit values { value } } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("%s %s: %v", name, resource, res.Errors)
		}
		return res.Data.(map[string]any)["datastoreMetrics"].([]any)[0].(map[string]any)
	}
	for _, name := range []string{"CPU", "MEMORY", "CPU_LIMIT", "MEMORY_LIMIT"} {
		asked = nil
		query("DATABASE", "dpg-pg", name)
		if len(asked) != 1 || asked[0].Container != postgresContainer {
			t.Errorf("Postgres %s asked %+v", name, asked)
		}
	}
	if got := query("KEYVALUE", "red-kv", "MEMORY"); got["values"].([]any)[0].(map[string]any)["value"].(float64) != 4096 {
		t.Errorf("Key Value MEMORY = %+v, want the used-memory series", got)
	}
}

// w34/m589: datastore cpu/memory/cpu-limit/memory-limit honor instance
// filtering, percentage normalization against DatastoreResource limit history,
// and replica aggregation (normalize -> project -> select -> aggregate).
func TestDatastoreAppMetricShapingOptions(t *testing.T) {
	const (
		pgID = "dpg-c185th5c2rvvnhbfiltg"
		kvID = "red-c185th5c2rvvnhbfiltg"
		ts1  = "2026-10-17T00:00:00Z"
		ts2  = "2026-10-17T00:01:00Z"
	)
	svc := newService(nil, nil, sampleDatabase(pgID, true), sampleKeyValue(kvID))
	svc.DatastoreResource = func(_ context.Context, req DatastoreResourceRequest) ([]MetricSeries, error) {
		switch req.Metric {
		case MetricCPU:
			return []MetricSeries{
				{Labels: map[string]string{"instance": req.Resource + "-1"}, Unit: unitCores, Points: []MetricPoint{
					{Timestamp: ts1, Value: 0.2},
					{Timestamp: ts2, Value: 0.2},
				}},
				{Labels: map[string]string{"instance": req.Resource + "-2"}, Unit: unitCores, Points: []MetricPoint{
					{Timestamp: ts1, Value: 0.6},
					{Timestamp: ts2, Value: 0.6},
				}},
			}, nil
		case MetricCPULimit:
			// Replica 1 upgrades its limit mid-window (0.5 -> 1.0 cores) while
			// replica 2 stays at 1.0 core, proving per-timestamp stepped join.
			return []MetricSeries{
				{Labels: map[string]string{"instance": req.Resource + "-1"}, Unit: unitCores, Points: []MetricPoint{
					{Timestamp: ts1, Value: 0.5},
					{Timestamp: ts2, Value: 1.0},
				}},
				{Labels: map[string]string{"instance": req.Resource + "-2"}, Unit: unitCores, Points: []MetricPoint{
					{Timestamp: ts1, Value: 1.0},
					{Timestamp: ts2, Value: 1.0},
				}},
			}, nil
		case MetricMemory:
			return []MetricSeries{
				{Labels: map[string]string{"instance": req.Resource + "-1"}, Unit: unitBytes, Points: []MetricPoint{{Timestamp: ts1, Value: float64(256 << 20)}}},
				{Labels: map[string]string{"instance": req.Resource + "-2"}, Unit: unitBytes, Points: []MetricPoint{{Timestamp: ts1, Value: float64(768 << 20)}}},
			}, nil
		case MetricMemoryLimit:
			if req.Kind == DatastoreKeyValue {
				return []MetricSeries{
					{Labels: map[string]string{"instance": req.Resource + "-0"}, Unit: unitBytes, Points: []MetricPoint{{Timestamp: ts1, Value: float64(512 << 20)}}},
				}, nil
			}
			return []MetricSeries{
				{Labels: map[string]string{"instance": req.Resource + "-1"}, Unit: unitBytes, Points: []MetricPoint{{Timestamp: ts1, Value: float64(512 << 20)}}},
				{Labels: map[string]string{"instance": req.Resource + "-2"}, Unit: unitBytes, Points: []MetricPoint{{Timestamp: ts1, Value: float64(1024 << 20)}}},
			}, nil
		default:
			return nil, nil
		}
	}
	svc.KeyValueStats = func(_ context.Context, req KeyValueStatsRequest) ([]MetricSeries, error) {
		return []MetricSeries{
			{Labels: map[string]string{"instance": req.Resource + "-0"}, Unit: unitBytes, Points: []MetricPoint{{Timestamp: ts1, Value: float64(128 << 20)}}},
		}, nil
	}
	ctx := context.Background()

	// Instance selection by raw pod name and by canonical public instance ID.
	id2 := ids.ServiceInstanceID(pgID, pgID+"-2")
	for _, sel := range []string{pgID + "-2", id2} {
		series, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: MetricCPU, Instances: []string{sel}}, pgID)
		if err != nil || len(series) != 1 || series[0].Labels["instance"] != id2 || series[0].Points[0].Value != 0.6 {
			t.Fatalf("instance=%q => %+v, %v, want only replica 2", sel, series, err)
		}
	}
	// Limit routes also honor instance filtering.
	for _, m := range []string{MetricCPULimit, MetricMemoryLimit} {
		series, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: m, Instances: []string{pgID + "-2"}}, pgID)
		if err != nil || len(series) != 1 || series[0].Labels["instance"] != id2 {
			t.Fatalf("%s instance=replica 2 => %+v, %v, want only replica 2", m, series, err)
		}
	}
	// Unknown instance ID returns empty, never broadening to all replicas.
	empty, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: MetricCPU, Instances: []string{pgID + "-does-not-exist"}}, pgID)
	if err != nil || len(empty) != 0 {
		t.Fatalf("unknown instance => %+v, %v, want empty []", empty, err)
	}

	// Percentage normalization joins per-pod, per-timestamp limit history.
	pct, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: MetricCPU, Percentage: true}, pgID)
	if err != nil || len(pct) != 2 {
		t.Fatalf("cpu percentage => %+v, %v", pct, err)
	}
	if pct[0].Unit != unitPercentage || !closeTo(pct[0].Points[0].Value, 40) || !closeTo(pct[0].Points[1].Value, 20) {
		t.Errorf("replica 1 pct = %+v, want 40%% then 20%% (stepped limit)", pct[0])
	}
	if pct[1].Unit != unitPercentage || !closeTo(pct[1].Points[0].Value, 60) || !closeTo(pct[1].Points[1].Value, 60) {
		t.Errorf("replica 2 pct = %+v, want 60%% then 60%%", pct[1])
	}

	// Normalization precedes replica aggregation: ts1 avg(40,60)=50, ts2 avg(20,60)=40.
	aggPct, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: MetricCPU, Percentage: true, ReplicaAggregate: "AVG"}, pgID)
	if err != nil || len(aggPct) != 1 || aggPct[0].Unit != unitPercentage ||
		!closeTo(aggPct[0].Points[0].Value, 50) || !closeTo(aggPct[0].Points[1].Value, 40) {
		t.Fatalf("percentage + AVG = %+v, %v, want [50, 40]", aggPct, err)
	}
	// Selection precedes aggregation: only replica 2 + AVG = [60, 60].
	selAgg, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: MetricCPU, Percentage: true, Instances: []string{pgID + "-2"}, ReplicaAggregate: "AVG"}, pgID)
	if err != nil || len(selAgg) != 1 || !closeTo(selAgg[0].Points[0].Value, 60) {
		t.Fatalf("selected percentage + AVG = %+v, %v, want 60", selAgg, err)
	}

	// Key Value memory maps to MetricKVMemory and normalizes against MetricMemoryLimit (128Mi/512Mi = 25%).
	kvPct, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: MetricMemory, Percentage: true, Instances: []string{kvID + "-0"}}, kvID)
	if err != nil || len(kvPct) != 1 || kvPct[0].Unit != unitPercentage || !closeTo(kvPct[0].Points[0].Value, 25) {
		t.Fatalf("KV memory percentage = %+v, %v, want 25%%", kvPct, err)
	}

	// Absolute replica MAX still works.
	series, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: MetricCPU, ReplicaAggregate: "MAX"}, pgID)
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 0.6 {
		t.Fatalf("MAX across instances = %+v, %v, want one series at 0.6", series, err)
	}

	// Unsupported metrics still fail with 400.
	for _, q := range []MetricQuery{
		{Metric: MetricCPULimit, Percentage: true},
		{Metric: MetricMemoryLimit, Percentage: true},
		{Metric: MetricHTTPRequests, Percentage: true},
		{Metric: MetricHTTPRequests, Instances: []string{pgID + "-1"}},
		{Metric: MetricBandwidth, Instances: []string{pgID + "-1"}},
		{Metric: MetricInstanceCount, Percentage: true},
		{Metric: MetricCPU, Instances: []string{""}},
	} {
		if _, err := svc.datastoreAppMetric(ctx, q, pgID); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%+v = %v, want ErrBadRequest", q, err)
		}
	}

	// Key Value memory percentage without DatastoreResource reports ErrMetricsUnavailable.
	svc.DatastoreResource = nil
	if _, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: MetricMemory, Percentage: true}, kvID); !errors.Is(err, core.ErrMetricsUnavailable) {
		t.Errorf("KV memory percentage without limit source = %v, want ErrMetricsUnavailable", err)
	}
}

// TestDatastorePercentageEndToEndOverHTTP exercises the production Prometheus
// sources (NewPrometheusDatastoreResourceSource + NewPrometheusKeyValueStatsSource)
// against a stub Prometheus server for Postgres CPU and Key Value memory
// percentage + instance filtering.
func TestDatastorePercentageEndToEndOverHTTP(t *testing.T) {
	const (
		pgID = "dpg-c185th5c2rvvnhbfiltg"
		kvID = "red-c185th5c2rvvnhbfiltg"
	)
	base := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC).Unix()
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		var result string
		switch {
		case strings.Contains(q, "container_cpu_usage_seconds_total"):
			result = fmt.Sprintf(`[
			  {"metric":{"pod":%q},"values":[[%d,"0.25"]]},
			  {"metric":{"pod":%q},"values":[[%d,"0.75"]]}]`, pgID+"-1", base, pgID+"-2", base)
		case strings.Contains(q, "kube_pod_container_resource_limits") && strings.Contains(q, `resource="cpu"`):
			result = fmt.Sprintf(`[
			  {"metric":{"pod":%q},"values":[[%d,"0.5"]]},
			  {"metric":{"pod":%q},"values":[[%d,"1"]]}]`, pgID+"-1", base, pgID+"-2", base)
		case strings.Contains(q, "redis_memory_used_bytes"):
			result = fmt.Sprintf(`[{"metric":{"pod":%q},"values":[[%d,"134217728"]]}]`, kvID+"-0", base)
		case strings.Contains(q, "kube_pod_container_resource_limits") && strings.Contains(q, `resource="memory"`):
			result = fmt.Sprintf(`[{"metric":{"pod":%q},"values":[[%d,"536870912"]]}]`, kvID+"-0", base)
		default:
			t.Errorf("unexpected PromQL: %q", q)
			result = `[]`
		}
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":%s}}`, result)
	}))
	defer stub.Close()

	svc := newService(nil, nil, sampleDatabase(pgID, true), sampleKeyValue(kvID))
	svc.DatastoreResource = NewPrometheusDatastoreResourceSource(stub.URL, stub.Client())
	svc.KeyValueStats = NewPrometheusKeyValueStatsSource(stub.URL, stub.Client())
	start := time.Unix(base, 0).UTC()
	end := start.Add(5 * time.Minute)

	// Postgres CPU percentage filtered to replica 2: 0.75 / 1.0 = 75%.
	pgSeries, err := svc.datastoreAppMetric(context.Background(), MetricQuery{
		Metric: MetricCPU, Percentage: true, Instances: []string{pgID + "-2"},
		Start: start, End: end, Resolution: time.Minute,
	}, pgID)
	if err != nil || len(pgSeries) != 1 || pgSeries[0].Unit != unitPercentage || !closeTo(pgSeries[0].Points[0].Value, 75) {
		t.Fatalf("Postgres CPU percentage filtered = %+v, %v, want 75%%", pgSeries, err)
	}
	if pgSeries[0].Labels["resource"] != pgID || pgSeries[0].Labels["instance"] != ids.ServiceInstanceID(pgID, pgID+"-2") {
		t.Errorf("Postgres labels = %+v, want canonical resource & instance", pgSeries[0].Labels)
	}

	// Key Value memory percentage: 128MiB / 512MiB = 25%.
	kvSeries, err := svc.datastoreAppMetric(context.Background(), MetricQuery{
		Metric: MetricMemory, Percentage: true,
		Start: start, End: end, Resolution: time.Minute,
	}, kvID)
	if err != nil || len(kvSeries) != 1 || kvSeries[0].Unit != unitPercentage || !closeTo(kvSeries[0].Points[0].Value, 25) {
		t.Fatalf("KV memory percentage = %+v, %v, want 25%%", kvSeries, err)
	}
}
