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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// activityMetrics are Render's database-activity and disk graphs (w7/m430).
var activityMetrics = []string{
	MetricNetworkActivity, MetricTransactionVolume, MetricLockDelayedQueries,
	MetricDiskActivity, MetricDiskOperations,
}

// activityService wires recording stubs for both new sources over a Postgres,
// a Key Value and a service with a disk, and returns what each source saw.
func activityService(t *testing.T, objs ...client.Object) (*Service, *[]DatastoreActivityRequest, *[]LockWaitsRequest) {
	t.Helper()
	svc := newService(nil, nil, objs...)
	var activity []DatastoreActivityRequest
	var locks []LockWaitsRequest
	svc.DatastoreActivity = func(_ context.Context, req DatastoreActivityRequest) ([]MetricSeries, error) {
		activity = append(activity, req)
		return []MetricSeries{{
			Labels: map[string]string{"resource": req.Resource, "instance": req.Resource + "-1", LabelDirection: directionRead},
			Unit:   unitBytes, Points: []MetricPoint{{Timestamp: "2026-11-16T00:00:00Z", Value: 7}},
		}}, nil
	}
	svc.LockWaits = func(_ context.Context, req LockWaitsRequest) ([]MetricSeries, error) {
		locks = append(locks, req)
		return []MetricSeries{{
			Labels: map[string]string{"resource": req.Database, "instance": req.Database + "-1"},
			Unit:   unitCount, Points: []MetricPoint{{Timestamp: "2026-11-16T00:00:00Z", Value: 1}},
		}}, nil
	}
	return svc, &activity, &locks
}

func diskService(name string) *appv1alpha1.App {
	return sampleAppWithDisk(name, &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10})
}

// TestActivityMetricsServedPerKind pins Render's Metrics page: Postgres has all
// five graphs, a Key Value Disk Activity, a service with a disk Disk Activity
// and Disk Operations. Every other pair is refused by name before any source
// is read, and the served ones reach their source with the resolved names.
func TestActivityMetricsServedPerKind(t *testing.T) {
	served := map[string][]string{
		DatastoreDatabase: activityMetrics,
		DatastoreKeyValue: {MetricDiskActivity},
		DatastoreService:  {MetricDiskActivity, MetricDiskOperations},
	}
	resources := map[string]string{DatastoreDatabase: "pg", DatastoreKeyValue: "cache", DatastoreService: "web"}
	created := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	app := diskService("web")
	app.CreationTimestamp.Time = created
	for kind, resource := range resources {
		for _, metric := range activityMetrics {
			t.Run(kind+"/"+metric, func(t *testing.T) {
				svc, activity, locks := activityService(t, sampleDatabase("pg", false), sampleKeyValue("cache"), app)
				series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: kind, Resource: resource, Metric: metric})
				if !slices.Contains(served[kind], metric) {
					if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), metric) {
						t.Fatalf("%s on a %s: err %v, want a 400 naming the metric", metric, kind, err)
					}
					if len(*activity)+len(*locks) != 0 {
						t.Fatalf("a refused metric reached a source: %+v %+v", *activity, *locks)
					}
					return
				}
				if err != nil || len(series) != 1 {
					t.Fatalf("%s on a %s: %v %+v", metric, kind, err, series)
				}
				if metric == MetricLockDelayedQueries {
					if got := (*locks)[0]; got.Namespace != "default" || got.Database != "pg" {
						t.Errorf("lock waits request = %+v", got)
					}
					return
				}
				got := (*activity)[0]
				if got.Kind != kind || got.Namespace != "default" || got.Resource != resource || got.Metric != metric {
					t.Errorf("activity request = %+v", got)
				}
				// Only a service's pods are matched by name across App
				// generations, so only it carries the creation floor.
				if wantFloor := kind == DatastoreService; got.NotBefore.IsZero() == wantFloor {
					t.Errorf("NotBefore = %v for a %s", got.NotBefore, kind)
				}
				if series[0].Labels[LabelDirection] != directionRead {
					t.Errorf("the direction label must survive the verb: %v", series[0].Labels)
				}
			})
		}
	}
}

// A service without a disk answers its disk graphs empty, as its usage graph
// does (the Disk tab asks before it knows), and never reads a source.
func TestActivityOnADisklessServiceIsEmpty(t *testing.T) {
	svc, activity, _ := activityService(t, sampleAppWithDisk("web", nil))
	for _, metric := range []string{MetricDiskActivity, MetricDiskOperations} {
		series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreService, Resource: "web", Metric: metric})
		if err != nil || series != nil {
			t.Fatalf("%s on a diskless service = %v %+v, want (nil, nil)", metric, err, series)
		}
	}
	if len(*activity) != 0 {
		t.Fatalf("a diskless service reached the source: %+v", *activity)
	}
}

// Without Prometheus the activity graphs are METRICS_UNAVAILABLE; without the
// log store Lock-Delayed Queries says so (LOG_STORE_UNAVAILABLE, 503 on REST)
// rather than charting a zero.
func TestActivityMetricsWithoutTheirSource(t *testing.T) {
	svc := newService(nil, nil, sampleDatabase("pg", false))
	for _, metric := range []string{MetricNetworkActivity, MetricTransactionVolume, MetricDiskActivity, MetricDiskOperations} {
		if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: metric}); err != core.ErrMetricsUnavailable {
			t.Errorf("%s without Prometheus = %v, want ErrMetricsUnavailable", metric, err)
		}
	}
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: MetricLockDelayedQueries}); err != core.ErrLogStoreUnavailable {
		t.Errorf("lock_delayed_queries without the log store = %v, want ErrLogStoreUnavailable", err)
	}
	rec := serveREST(svc, "/v1/metrics/lock-delayed-queries?resource=pg")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "LOG_STORE_UNAVAILABLE") {
		t.Errorf("REST lock-delayed-queries without the log store = %d %s", rec.Code, rec.Body.String())
	}
	// Authorization still comes first: an unknown database is a 404.
	if rec := serveREST(svc, "/v1/metrics/lock-delayed-queries?resource=nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown database = %d, want 404", rec.Code)
	}
}

// activitySurfaces reads one metric on REST, GraphQL and MCP and returns each
// surface's series as (labels, values) pairs, or the error each answered.
type surfaceRead struct {
	series []string
	err    string
}

func readActivitySurfaces(t *testing.T, svc *Service, kind, resource, metric, rest, gql string) map[string]surfaceRead {
	t.Helper()
	out := map[string]surfaceRead{}
	flatten := func(labels map[string]string, values []float64) string {
		delete(labels, LabelMetric) // MCP names the metric on each series
		b, _ := json.Marshal(struct {
			L map[string]string
			V []float64
		}{labels, values})
		return string(b)
	}

	rec := serveREST(svc, "/v1/metrics/"+rest+"?resource="+resource+"&kind="+kind)
	if rec.Code != http.StatusOK {
		out["rest"] = surfaceRead{err: rec.Body.String()}
	} else {
		var series []renderMetricSeries
		if err := json.Unmarshal(rec.Body.Bytes(), &series); err != nil || series == nil {
			t.Fatalf("REST body %q: %v", rec.Body.String(), err)
		}
		r := surfaceRead{series: []string{}}
		for _, s := range series {
			labels := map[string]string{}
			for _, l := range s.Labels {
				labels[l.Field] = l.Value
			}
			var values []float64
			for _, v := range s.Values {
				values = append(values, v.Value)
			}
			r.series = append(r.series, flatten(labels, values))
		}
		out["rest"] = r
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: fmt.Sprintf(
		`{ datastoreMetrics(query: {kind: %q, resource: %q, name: %q}) { labels { field value } values { value } } }`, kind, resource, gql)})
	if len(res.Errors) > 0 {
		out["graphql"] = surfaceRead{err: res.Errors[0].Message}
	} else {
		r := surfaceRead{series: []string{}}
		for _, raw := range res.Data.(map[string]any)["datastoreMetrics"].([]any) {
			s := raw.(map[string]any)
			// GraphQL names these two by Render's MetricQueryFilter (w3/087,
			// TestGraphQLMetricLabelNames); read back in REST's names to compare.
			restName := map[string]string{"RESOURCE": LabelResource, "INSTANCE": "instance"}
			labels := map[string]string{}
			for _, l := range s["labels"].([]any) {
				field := l.(map[string]any)["field"].(string)
				if name, ok := restName[field]; ok {
					field = name
				}
				labels[field] = l.(map[string]any)["value"].(string)
			}
			var values []float64
			for _, v := range s["values"].([]any) {
				values = append(values, v.(map[string]any)["value"].(float64))
			}
			r.series = append(r.series, flatten(labels, values))
		}
		out["graphql"] = r
	}

	result, err := mcpSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_datastore_metrics", Arguments: map[string]any{
		"resourceId": resource, "kind": kind, "metricTypes": []string{metric},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		b, _ := json.Marshal(result.Content)
		out["mcp"] = surfaceRead{err: string(b)}
	} else {
		var got datastoreMetricsResult
		b, _ := json.Marshal(result.StructuredContent)
		if err := json.Unmarshal(b, &got); err != nil || got.Series == nil {
			t.Fatalf("MCP structured content %s: %v", b, err)
		}
		r := surfaceRead{series: []string{}}
		for _, s := range got.Series {
			if s.Labels[LabelMetric] != metric {
				t.Errorf("MCP series does not name its metric: %v", s.Labels)
			}
			var values []float64
			for _, p := range s.Points {
				values = append(values, p.Value)
			}
			r.series = append(r.series, flatten(s.Labels, values))
		}
		out["mcp"] = r
	}
	return out
}

// TestActivityMetricsAgreeAcrossSurfaces: REST, GraphQL datastoreMetrics and
// MCP get_datastore_metrics return the same series (labels, direction, values)
// for a populated and an empty window, an empty window is [] everywhere, and a
// metric the kind does not have is refused by name on all three.
func TestActivityMetricsAgreeAcrossSurfaces(t *testing.T) {
	two := []MetricSeries{
		{Labels: map[string]string{"resource": "pg", "instance": "pg-1", LabelDirection: directionRead}, Unit: unitBytes, Points: []MetricPoint{{Timestamp: "2026-11-16T00:00:00Z", Value: 1024}, {Timestamp: "2026-11-16T00:01:00Z", Value: 0}}},
		{Labels: map[string]string{"resource": "pg", "instance": "pg-1", LabelDirection: directionWrite}, Unit: unitBytes, Points: []MetricPoint{{Timestamp: "2026-11-16T00:00:00Z", Value: 2048}, {Timestamp: "2026-11-16T00:01:00Z", Value: 4096}}},
	}
	cases := []struct {
		kind, resource, metric, rest, gql string
		answer                            []MetricSeries
	}{
		{DatastoreDatabase, "pg", MetricNetworkActivity, "network-activity", "NETWORK_ACTIVITY", two},
		{DatastoreDatabase, "pg", MetricTransactionVolume, "transaction-volume", "TRANSACTION_VOLUME", nil},
		{DatastoreDatabase, "pg", MetricLockDelayedQueries, "lock-delayed-queries", "LOCK_DELAYED_QUERIES", two[:1]},
		{DatastoreKeyValue, "cache", MetricDiskActivity, "disk-activity", "DISK_ACTIVITY", two},
		{DatastoreService, "web", MetricDiskOperations, "disk-operations", "DISK_OPERATIONS", nil},
	}
	for _, tc := range cases {
		t.Run(tc.metric, func(t *testing.T) {
			svc := newService(nil, nil, sampleDatabase("pg", false), sampleKeyValue("cache"), diskService("web"))
			answer := func() []MetricSeries {
				out := make([]MetricSeries, 0, len(tc.answer))
				for _, s := range tc.answer {
					labels := map[string]string{}
					for k, v := range s.Labels {
						labels[k] = v
					}
					out = append(out, MetricSeries{Labels: labels, Unit: s.Unit, Points: slices.Clone(s.Points)})
				}
				if len(out) == 0 {
					return nil // a source answering no series at all
				}
				return out
			}
			svc.DatastoreActivity = func(context.Context, DatastoreActivityRequest) ([]MetricSeries, error) { return answer(), nil }
			svc.LockWaits = func(context.Context, LockWaitsRequest) ([]MetricSeries, error) { return answer(), nil }

			reads := readActivitySurfaces(t, svc, tc.kind, tc.resource, tc.metric, tc.rest, tc.gql)
			for surface, r := range reads {
				if r.err != "" {
					t.Fatalf("%s: %s", surface, r.err)
				}
				if len(r.series) != len(tc.answer) {
					t.Errorf("%s: %d series, want %d: %v", surface, len(r.series), len(tc.answer), r.series)
				}
			}
			if !reflect.DeepEqual(reads["rest"].series, reads["graphql"].series) || !reflect.DeepEqual(reads["rest"].series, reads["mcp"].series) {
				t.Errorf("surfaces disagree:\nREST    %v\nGraphQL %v\nMCP     %v", reads["rest"].series, reads["graphql"].series, reads["mcp"].series)
			}
		})
	}

	// Refused by name on every surface, before any source is read.
	svc, activity, locks := activityService(t, sampleKeyValue("cache"), diskService("web"))
	for _, tc := range []struct{ kind, resource, metric, rest, gql string }{
		{DatastoreKeyValue, "cache", MetricTransactionVolume, "transaction-volume", "TRANSACTION_VOLUME"},
		{DatastoreKeyValue, "cache", MetricDiskOperations, "disk-operations", "DISK_OPERATIONS"},
		{DatastoreService, "web", MetricNetworkActivity, "network-activity", "NETWORK_ACTIVITY"},
		{DatastoreService, "web", MetricLockDelayedQueries, "lock-delayed-queries", "LOCK_DELAYED_QUERIES"},
	} {
		for surface, r := range readActivitySurfaces(t, svc, tc.kind, tc.resource, tc.metric, tc.rest, tc.gql) {
			if !strings.Contains(r.err, tc.metric) {
				t.Errorf("%s %s on a %s: %+v, want a refusal naming the metric", surface, tc.metric, tc.kind, r)
			}
		}
	}
	if len(*activity)+len(*locks) != 0 {
		t.Fatalf("a refused read reached a source: %+v %+v", *activity, *locks)
	}
	// REST answers the refusal as a 400.
	if rec := serveREST(svc, "/v1/metrics/disk-operations?resource=cache&kind=keyvalue"); rec.Code != http.StatusBadRequest {
		t.Errorf("REST disk-operations on a Key Value = %d, want 400", rec.Code)
	}
}

// TestActivityMetricsStayInTheirWorkspace: a datastore in another workspace is
// denied before any source runs, on the new graphs as on the old ones.
func TestActivityMetricsStayInTheirWorkspace(t *testing.T) {
	db := sampleDatabase("pg", false)
	db.Labels = map[string]string{core.LabelTenant: "tea-other"}
	svc, activity, locks := activityService(t, db)
	svc.Authz = ownWorkspaceOnly("workspace:tea-mine")
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "u", Method: core.MethodSession})
	for _, metric := range []string{MetricNetworkActivity, MetricTransactionVolume, MetricLockDelayedQueries, MetricDiskActivity, MetricDiskOperations} {
		if _, err := svc.DatastoreMetrics(ctx, DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "pg", Metric: metric}); !errors.Is(err, core.ErrForbidden) && !errors.Is(err, core.ErrNotFound) {
			t.Errorf("%s on another workspace's database = %v, want denied", metric, err)
		}
	}
	if len(*activity)+len(*locks) != 0 {
		t.Fatalf("a denied read reached a source: %+v %+v", *activity, *locks)
	}
}

// ownWorkspaceOnly allows every relation on one workspace object and nothing
// else: the caller is a member of that workspace only.
type ownWorkspaceOnly string

func (o ownWorkspaceOnly) Check(_ context.Context, _, _, object string) (bool, error) {
	return object == string(o), nil
}

// --- The queries, run by Prometheus's and Loki's own engines ---

// TestActivityQueriesOnPrometheus evaluates the production PromQL for each
// graph in Prometheus's engine (`promtool test rules`) over series shaped
// like production's (read 2026-11-16): a Postgres pod with its WAL sidecar,
// a LUKS service disk charged to both its dm and sd devices, the boot disk,
// a recreated pod's stale series, and pods that are not the resource's.
func TestActivityQueriesOnPrometheus(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		testenv.Skip(t, "promtool not on PATH")
	}
	// Counters sampled every 15s (cAdvisor) or 60s (CNPG), each growing by a
	// fixed amount per sample, so a 60s bucket holds 4 or 1 increments.
	counter := func(perSample float64, samples int) string {
		return fmt.Sprintf("0+%sx%d", strconv.FormatFloat(perSample, 'f', -1, 64), samples)
	}
	type in struct{ series, values string }
	cadvisor := []in{
		// Postgres pg-1: data volume on sdb, boot disk on sda.
		{`container_fs_writes_bytes_total{namespace="ns",pod="pg-1",container="postgres",device="/dev/sdb",id="a"}`, counter(100, 80)},
		{`container_fs_writes_bytes_total{namespace="ns",pod="pg-1",container="plugin-barman-cloud",device="/dev/sdb",id="b"}`, counter(10, 80)},
		{`container_fs_writes_bytes_total{namespace="ns",pod="pg-1",container="postgres",device="/dev/sda",id="a"}`, counter(5000, 80)},
		{`container_fs_reads_bytes_total{namespace="ns",pod="pg-1",container="postgres",device="/dev/sdb",id="a"}`, counter(25, 80)},
		{`container_fs_writes_total{namespace="ns",pod="pg-1",container="postgres",device="/dev/sdb",id="a"}`, counter(3, 80)},
		{`container_fs_reads_total{namespace="ns",pod="pg-1",container="postgres",device="/dev/sdb",id="a"}`, counter(1, 80)},
		// Another resource's pods: a database whose name extends ours, and
		// a different namespace.
		{`container_fs_writes_bytes_total{namespace="ns",pod="pg-extra-1",container="postgres",device="/dev/sdc",id="c"}`, counter(999, 80)},
		{`container_fs_writes_bytes_total{namespace="other",pod="pg-1",container="postgres",device="/dev/sdb",id="d"}`, counter(999, 80)},
		// Service web's LUKS disk: the same writes on dm-0 and on sdd.
		{`container_fs_writes_bytes_total{namespace="ns",pod="web-6b7f9c-x2k4p",container="app",device="/dev/dm-0",id="e"}`, counter(40, 80)},
		{`container_fs_writes_bytes_total{namespace="ns",pod="web-6b7f9c-x2k4p",container="app",device="/dev/sdd",id="e"}`, counter(40, 80)},
		{`container_fs_writes_bytes_total{namespace="ns",pod="web-6b7f9c-x2k4p",container="app",device="/dev/sda",id="e"}`, counter(7000, 80)},
		// A service of another name in the namespace.
		{`container_fs_writes_bytes_total{namespace="ns",pod="web-api-6b7f9c-x2k4p",container="app",device="/dev/sde",id="f"}`, counter(999, 80)},
		// Postgres network, on the pod-level row.
		{`container_network_receive_bytes_total{namespace="ns",pod="pg-1",container="",interface="eth0",id="p"}`, counter(300, 80)},
		{`container_network_transmit_bytes_total{namespace="ns",pod="pg-1",container="",interface="eth0",id="p"}`, counter(600, 80)},
		// A read replica of pg is a CNPG cluster of its own (w6/m467).
		{`container_network_receive_bytes_total{namespace="ns",pod="pg-r-ca978112ca-1",container="",interface="eth0",id="r"}`, counter(30, 80)},
		{`container_network_transmit_bytes_total{namespace="ns",pod="pg-r-ca978112ca-1",container="",interface="eth0",id="r"}`, counter(60, 80)},
	}
	cnpg := []in{
		{`cnpg_pg_stat_database_xact_commit{namespace="ns",cnpg_io_cluster="pg",pod="pg-1",datname="pg_app"}`, counter(40, 20)},
		{`cnpg_pg_stat_database_xact_rollback{namespace="ns",cnpg_io_cluster="pg",pod="pg-1",datname="pg_app"}`, counter(2, 20)},
		{`cnpg_pg_stat_database_xact_commit{namespace="ns",cnpg_io_cluster="pg",pod="pg-1",datname="postgres"}`, counter(500, 20)},
		{`cnpg_pg_stat_database_xact_commit{namespace="ns",cnpg_io_cluster="pg",pod="pg-1",datname="template1"}`, counter(500, 20)},
		{`cnpg_pg_stat_database_xact_commit{namespace="ns",cnpg_io_cluster="pg-extra",pod="pg-extra-1",datname="pg_app"}`, counter(999, 20)},
		{`cnpg_pg_stat_database_xact_commit{namespace="ns",cnpg_io_cluster="pg-r-ca978112ca",pod="pg-r-ca978112ca-1",datname="pg_app"}`, counter(7, 20)},
	}
	q := func(kind, resource, metric string, step time.Duration) string {
		query, _ := promDatastoreActivityQuery(DatastoreActivityRequest{Kind: kind, Namespace: "ns", Resource: resource, Metric: metric, Resolution: step})
		return query
	}
	type check struct {
		expr, at string
		want     []string // exp_samples, label set + value
	}
	minute := time.Minute
	checks := []check{
		// 60s bucket = 4 cAdvisor samples: (100+10) per sample on the data
		// volume, sda excluded; reads 25 per sample.
		{q(DatastoreDatabase, "pg", MetricDiskActivity, minute), "10m", []string{
			`{direction="write",pod="pg-1"} 440`, `{direction="read",pod="pg-1"} 100`}},
		{q(DatastoreDatabase, "pg", MetricDiskOperations, minute), "10m", []string{
			`{direction="write",pod="pg-1"} 12`, `{direction="read",pod="pg-1"} 4`}},
		// LUKS counted once, boot disk excluded, web-api's pod not matched.
		{q(DatastoreService, "web", MetricDiskActivity, minute), "10m", []string{`{direction="write",pod="web-6b7f9c-x2k4p"} 160`}},
		// A 5-minute bucket is increase() over it: 20 samples.
		{q(DatastoreService, "web", MetricDiskActivity, 5*minute), "10m", []string{`{direction="write",pod="web-6b7f9c-x2k4p"} 800`}},
		{q(DatastoreDatabase, "pg", MetricNetworkActivity, minute), "10m", []string{
			`{direction="read",pod="pg-1"} 1200`, `{direction="write",pod="pg-1"} 2400`,
			`{direction="read",pod="pg-r-ca978112ca-1"} 120`, `{direction="write",pod="pg-r-ca978112ca-1"} 240`}},
		// 42 tenant transactions a minute; postgres and template1 excluded.
		// A 60s step over a 60s scrape spreads the 240s rate over the bucket.
		// The read replica's own 7 are its own instance's.
		{q(DatastoreDatabase, "pg", MetricTransactionVolume, minute), "10m", []string{`{pod="pg-1"} 42`, `{pod="pg-r-ca978112ca-1"} 7`}},
		{q(DatastoreDatabase, "pg", MetricTransactionVolume, 10*minute), "15m", []string{`{pod="pg-1"} 420`, `{pod="pg-r-ca978112ca-1"} 70`}},
	}
	var b strings.Builder
	b.WriteString("rule_files: []\nevaluation_interval: 15s\ntests:\n")
	for _, group := range []struct {
		interval string
		series   []in
	}{{"15s", cadvisor}, {"60s", cnpg}} {
		fmt.Fprintf(&b, "  - interval: %s\n    input_series:\n", group.interval)
		for _, s := range group.series {
			fmt.Fprintf(&b, "      - series: '%s'\n        values: '%s'\n", s.series, s.values)
		}
		b.WriteString("    promql_expr_test:\n")
		for _, c := range checks {
			uses := strings.Contains(c.expr, "cnpg_") == (group.interval == "60s")
			if !uses {
				continue
			}
			// Rounded: a rate times its window is exact only up to float error.
			fmt.Fprintf(&b, "      - expr: 'round(%s, 0.001)'\n        eval_time: %s\n        exp_samples:\n", strings.ReplaceAll(c.expr, "'", "''"), c.at)
			for _, w := range c.want {
				i := strings.LastIndex(w, " ")
				fmt.Fprintf(&b, "          - labels: '%s'\n            value: %s\n", w[:i], w[i+1:])
			}
		}
	}
	runPromtoolTests(t, promtool, b.String())
}

// TestPrometheusActivitySourceShapesSeries: the source keeps resource,
// instance and direction, drops every other label, and stamps the unit.
func TestPrometheusActivitySourceShapesSeries(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"pod":"pg-1","direction":"write","device":"/dev/sdb"},"values":[[1000000,"440"]]}]}}`))
	}))
	defer ts.Close()
	series, err := NewPrometheusDatastoreActivitySource(ts.URL, ts.Client())(context.Background(), DatastoreActivityRequest{
		Kind: DatastoreDatabase, Namespace: "ns", Resource: "pg", Metric: MetricDiskOperations,
		Start: time.Unix(1_000_000, 0), End: time.Unix(1_000_120, 0), Resolution: time.Minute,
	})
	want := map[string]string{"resource": "pg", "instance": "pg-1", LabelDirection: directionWrite}
	if err != nil || len(series) != 1 || !reflect.DeepEqual(series[0].Labels, want) || series[0].Unit != unitCount {
		t.Fatalf("activity source: %v %+v", err, series)
	}
	if !strings.Contains(gotQuery, "container_fs_writes_total") {
		t.Errorf("disk_operations queried %q", gotQuery)
	}
}

// TestLockWaitsOnLoki counts real log_lock_waits lines, captured from CNPG
// 1.30.1 on PostgreSQL 18.4 and rendered as the log shipper renders them, in
// the pinned Loki (BEX_TEST_LOKI_URL): a three-second wait counts once, in the
// bucket where it was granted; its "still waiting" line, a wait still in
// progress, a sub-second wait under a lowered deadlock_timeout, a tenant's
// own look-alike text, and another database's wait do not count.
func TestLockWaitsOnLoki(t *testing.T) {
	base := os.Getenv("BEX_TEST_LOKI_URL")
	if base == "" {
		testenv.Skip(t, "BEX_TEST_LOKI_URL not set")
	}
	base = strings.TrimRight(base, "/")
	ns := fmt.Sprintf("m430-%d", time.Now().UnixNano())
	bucket := time.Now().Add(-5 * time.Minute).Truncate(time.Minute)
	type line struct {
		at       time.Duration
		database string
		text     string
	}
	pg := func(pid int, sev, msg string) string {
		return fmt.Sprintf("2026-11-17 01:37:06.399 UTC [%d] %s:  %s", pid, sev, msg)
	}
	lines := []line{
		// The completed three-second wait: two lines, counted once.
		{10 * time.Second, "pg", pg(228, "LOG", "process 228 still waiting for ShareLock on transaction 761 after 1000.191 ms DETAIL:  Process holding the lock: 226. Wait queue: 228.")},
		{12 * time.Second, "pg", pg(228, "LOG", "process 228 acquired ShareLock on transaction 761 after 2500.040 ms")},
		// Still waiting when the window ends: no acquired line yet.
		{70 * time.Second, "pg", pg(301, "LOG", "process 301 still waiting for ExclusiveLock on tuple (0,1) of relation 16385 of database 5 after 1000.022 ms DETAIL:  Process holding the lock: 300. Wait queue: 301.")},
		// A deadlock_timeout lowered to 200ms by a legacy parameter.
		{20 * time.Second, "pg", pg(410, "LOG", "process 410 acquired ShareLock on transaction 900 after 812.503 ms")},
		// A tenant's RAISE NOTICE quoting the shape, and an ordinary line.
		{25 * time.Second, "pg", pg(415, "NOTICE", "process 1 acquired ShareLock on transaction 1 after 5000.000 ms")},
		{30 * time.Second, "pg", pg(416, "LOG", "checkpoint complete: wrote 3 buffers")},
		// The same wait in another database.
		{12 * time.Second, "pg-other", pg(228, "LOG", "process 228 acquired ShareLock on transaction 761 after 2500.040 ms")},
	}
	type stream struct {
		Stream map[string]string `json:"stream"`
		Values [][2]string       `json:"values"`
	}
	var streams []stream
	for _, l := range lines {
		streams = append(streams, stream{
			Stream: map[string]string{"namespace": ns, "database": l.database, "type": "postgres", "pod": l.database + "-1", "container": "postgres", "level": "info"},
			Values: [][2]string{{strconv.FormatInt(bucket.Add(l.at).UnixNano(), 10), l.text}},
		})
	}
	body, _ := json.Marshal(map[string]any{"streams": streams})
	hc := &http.Client{Timeout: 10 * time.Second}
	resp, err := hc.Post(base+"/loki/api/v1/push", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("push: HTTP %d", resp.StatusCode)
	}

	source := NewLokiLockWaitsSource(base, hc)
	read := func(database string) []MetricSeries {
		t.Helper()
		series, err := source(context.Background(), LockWaitsRequest{
			Namespace: ns, Database: database, Start: bucket, End: bucket.Add(2 * time.Minute), Resolution: time.Minute,
		})
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		return series
	}
	// Pushed lines become queryable once the ingester has them.
	var series []MetricSeries
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(500 * time.Millisecond) {
		if series = read("pg"); len(series) > 0 || time.Now().After(deadline) {
			break
		}
	}
	if len(series) != 1 || series[0].Labels["instance"] != "pg-1" || series[0].Labels["resource"] != "pg" || series[0].Unit != unitCount {
		t.Fatalf("lock waits = %+v, want one pg-1 series", series)
	}
	var total float64
	counted := map[string]float64{}
	for _, p := range series[0].Points {
		total += p.Value
		counted[p.Timestamp] += p.Value
	}
	if total != 1 {
		t.Fatalf("lock-delayed queries = %v over the window (%v), want exactly the one completed wait", total, counted)
	}
	// The bucket that holds the acquired line, (bucket, bucket+60s].
	if got := counted[bucket.Add(time.Minute).UTC().Format(time.RFC3339)]; got != 1 {
		t.Errorf("the wait counted in %v, want the bucket ending %s", counted, bucket.Add(time.Minute).UTC().Format(time.RFC3339))
	}
	if other := read("pg-other"); len(other) != 1 || other[0].Labels["instance"] != "pg-other-1" {
		t.Errorf("another database's own wait = %+v", other)
	}
}
