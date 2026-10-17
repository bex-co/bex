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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
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

// Percentage and instance filters cannot be honored for a datastore and are
// refused by name; replica aggregation collapses its instances as it does a
// service's.
func TestDatastoreAppMetricShapingOptions(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("dpg-pg", true))
	svc.DatastoreResource = func(_ context.Context, req DatastoreResourceRequest) ([]MetricSeries, error) {
		return []MetricSeries{
			{Labels: map[string]string{"instance": "dpg-pg-1"}, Unit: unitCores, Points: []MetricPoint{{Timestamp: "2026-10-17T00:00:00Z", Value: 0.2}}},
			{Labels: map[string]string{"instance": "dpg-pg-2"}, Unit: unitCores, Points: []MetricPoint{{Timestamp: "2026-10-17T00:00:00Z", Value: 0.6}}},
		}, nil
	}
	ctx := context.Background()
	for _, q := range []MetricQuery{{Metric: MetricCPU, Percentage: true}, {Metric: MetricCPU, Instances: []string{"x"}}} {
		if _, err := svc.datastoreAppMetric(ctx, q, "dpg-pg"); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "apply to services") {
			t.Errorf("%+v = %v, want a coded refusal", q, err)
		}
	}
	series, err := svc.datastoreAppMetric(ctx, MetricQuery{Metric: MetricCPU, ReplicaAggregate: "MAX"}, "dpg-pg")
	if err != nil || len(series) != 1 || series[0].Points[0].Value != 0.6 {
		t.Fatalf("MAX across instances = %+v, %v, want one series at 0.6", series, err)
	}
}
