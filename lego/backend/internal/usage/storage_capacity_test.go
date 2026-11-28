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
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const gib = 1 << 30

// capacityProm is a Prometheus stub for the storage capacity meter (w4/m713):
// the exporter's up steps and the claims' summed byte-steps it answers, and
// every query it was asked. upSteps < 0 answers the up query with an empty
// vector, as Prometheus does when the exporter has no series; fail makes the
// capacity query a 503.
type capacityProm struct {
	mu        sync.Mutex
	upSteps   float64
	byteSteps float64
	fail      bool
	queries   []string
}

func (p *capacityProm) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		p.mu.Lock()
		p.queries = append(p.queries, query)
		up, bytes, fail := p.upSteps, p.byteSteps, p.fail
		p.mu.Unlock()
		value := 0.0
		switch {
		case strings.Contains(query, `up{job="kube-state-metrics"}`):
			if up < 0 {
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
				return
			}
			value = up
		case strings.Contains(query, "kube_persistentvolumeclaim_resource_requests_storage_bytes"):
			if fail {
				http.Error(w, "temporary", http.StatusServiceUnavailable)
				return
			}
			value = bytes
		case isEgressHealthQuery(query):
			value = 1
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"%v"]}]}}`, value)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (p *capacityProm) capacityQuery(t *testing.T) string {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, q := range p.queries {
		if strings.Contains(q, "kube_persistentvolumeclaim_resource_requests_storage_bytes") {
			return q
		}
	}
	t.Fatalf("no capacity query among %q", p.queries)
	return ""
}

// The capacity is read from each claim's request at every minute of the window
// and counted in the product's GB (GiB): a 10 GB database is provisioned as
// 10Gi, so a full hour reads 10 × 3,600 = 36,000, not the 10.74 decimal GB of
// its claim. An HA pair and a read replica each add their own volume, and a
// database present for half the hour pays for that half.
func TestQueryStorageCapacityGBSeconds(t *testing.T) {
	start := time.Date(2026, 12, 3, 10, 0, 0, 0, time.UTC)
	ds := datastoreEntry{ID: "dpg-db", Name: "dpg-db", Namespace: "tea-ws", Kind: store.ResourceKindPostgres, Plan: "basic-1gb"}
	for _, tt := range []struct {
		name         string
		upSteps      float64
		claimMinutes float64 // the minutes, summed over claims, each 10Gi claim existed
		want         int64
		complete     bool
	}{
		{"a full hour of one 10 GB volume", 60, 60, 36000, true},
		{"a primary, its HA standby and one read replica", 60, 3 * 60, 3 * 36000, true},
		{"created half-way through the hour", 60, 30, 18000, true},
		{"no volume yet", 60, 0, 0, true},
		{"the exporter missed ten minutes", 50, 50, 30000, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := &capacityProm{upSteps: tt.upSteps, byteSteps: tt.claimMinutes * 10 * gib}
			svc := NewService(&core.Base{}, nil, p.serve(t).URL, nil)
			got, complete, ok := svc.queryStorageCapacityGBSeconds(context.Background(), ds, start, start.Add(time.Hour))
			if !ok || got != tt.want || complete != tt.complete {
				t.Fatalf("capacity = %d complete=%v ok=%v, want %d complete=%v ok=true", got, complete, ok, tt.want, tt.complete)
			}
		})
	}

	p := &capacityProm{upSteps: 60}
	svc := NewService(&core.Base{}, nil, p.serve(t).URL, nil)
	if _, _, ok := svc.queryStorageCapacityGBSeconds(context.Background(), ds, start, start.Add(time.Hour)); !ok {
		t.Fatal("capacity query failed")
	}
	q := p.capacityQuery(t)
	// The window's own 60 steps, (start, end], a step per scrape; the claims'
	// requests, never the filesystem's capacity (a 10 GiB floor on Hetzner).
	for _, want := range []string{`namespace="tea-ws"`, `job="kube-state-metrics"`, "[3599s:60s]", "max by (persistentvolumeclaim)"} {
		if !strings.Contains(q, want) {
			t.Errorf("capacity query %q lacks %q", q, want)
		}
	}
	if strings.Contains(q, "kubelet_volume_stats") {
		t.Errorf("capacity query %q reads the volume's filesystem, not its claim", q)
	}
	pattern := regexp.MustCompile(`persistentvolumeclaim=~"([^"]+)"`).FindStringSubmatch(q)
	if pattern == nil {
		t.Fatalf("capacity query %q has no claim pattern", q)
	}
	claims := regexp.MustCompile(strings.ReplaceAll(pattern[1], `\\`, `\`))
	replica := appv1alpha1.ReadReplicaClusterName("dpg-db", "reporting")
	for name, want := range map[string]bool{
		"dpg-db-1": true, "dpg-db-2": true, replica + "-1": true, // primary, standby, a replica's instance
		"dpg-db-1-wal": false, "dpg-dbx-1": false, "dpg-db": false, "data-dpg-db-0": false,
	} {
		if got := claims.MatchString(name); got != want {
			t.Errorf("claim %q matched=%v, want %v (pattern %s)", name, got, want, pattern[1])
		}
	}
}

// An exporter that reported at no step, or a Prometheus that fails, is no
// reading at all: no row, the window is recorded unavailable and retried.
func TestStorageCapacityUnavailableWritesNoRow(t *testing.T) {
	window := time.Date(2026, 12, 3, 10, 0, 0, 0, time.UTC)
	ds := datastoreEntry{ID: "dpg-db", Name: "dpg-db", Namespace: "tea-ws", TenantID: "tea-ws", Kind: store.ResourceKindPostgres, Plan: "basic-1gb"}
	for name, p := range map[string]*capacityProm{
		"no exporter series": {upSteps: -1, byteSteps: 60 * 10 * gib},
		"exporter down":      {upSteps: 0, byteSteps: 60 * 10 * gib},
		"query failed":       {upSteps: 60, fail: true},
	} {
		t.Run(name, func(t *testing.T) {
			st := newMemUsageStore()
			svc := NewService(&core.Base{}, st, p.serve(t).URL, nil)
			if svc.processDatastoreMeterWindow(context.Background(), ds, store.UsageKindStorageCapacityGBSeconds, window) {
				t.Fatal("an unavailable capacity reading was reported metered")
			}
			if len(st.rows) != 0 {
				t.Fatalf("an unavailable capacity reading wrote %+v", st.rows)
			}
			if len(st.healthRecords) != 1 || st.healthRecords[0].Kind != store.UsageKindStorageCapacityGBSeconds ||
				st.healthRecords[0].Source != store.UsageSourceStorage || st.healthRecords[0].State != store.UsageSourceUnavailable {
				t.Fatalf("health = %+v, want one unavailable storage record", st.healthRecords)
			}
		})
	}
}

// A Postgres writes its capacity row with its plan, beside its used-bytes row;
// a Key Value writes no capacity row, since Render charges it no storage. The
// capacity stream starts at the first window the meter measures, so a meter
// added mid-month does not leave the month partial. A reading the exporter only
// partly covered is kept, degraded.
func TestDatastoreWindowWritesCapacityForPostgresOnly(t *testing.T) {
	window := time.Date(2026, 12, 3, 10, 0, 0, 0, time.UTC)
	p := &capacityProm{upSteps: 60, byteSteps: 60 * 10 * gib}
	st := newMemUsageStore()
	svc := NewService(&core.Base{}, st, p.serve(t).URL, nil)
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	pg := datastoreEntry{ID: "dpg-db", Name: "dpg-db", Namespace: "tea-ws", TenantID: "tea-ws", Kind: store.ResourceKindPostgres, Plan: "basic-1gb", CreatedAt: created}
	kv := datastoreEntry{ID: "red-kv", Name: "red-kv", Namespace: "tea-ws", TenantID: "tea-ws", Kind: store.ResourceKindKeyValue, Plan: "starter", CreatedAt: created}
	svc.processDatastoreWindow(context.Background(), pg, window)
	svc.processDatastoreWindow(context.Background(), kv, window)

	st.mu.Lock()
	row, ok := st.rows[usageKey{store.ResourceKindPostgres, "dpg-db", store.UsageKindStorageCapacityGBSeconds, "basic-1gb", window}]
	var kvCapacity []store.HourlyRow
	for k, r := range st.rows {
		if k.kind == store.UsageKindStorageCapacityGBSeconds && k.resourceKind == store.ResourceKindKeyValue {
			kvCapacity = append(kvCapacity, r)
		}
	}
	st.mu.Unlock()
	if !ok || row.Quantity != 36000 {
		t.Fatalf("Postgres capacity row = %+v present=%v, want 36,000 GB-seconds at its plan", row, ok)
	}
	if len(row.SourceHealth) != 1 || row.SourceHealth[0].Source != store.UsageSourceStorage ||
		row.SourceHealth[0].State != store.UsageSourceHealthy || !row.SourceHealth[0].ExpectedFrom.Equal(window) {
		t.Errorf("capacity health = %+v, want healthy storage expected from the window it measured", row.SourceHealth)
	}
	if len(kvCapacity) != 0 {
		t.Errorf("a Key Value wrote capacity rows %+v", kvCapacity)
	}
	if kinds := kv.meterKinds(); len(kinds) != 3 || strings.Contains(strings.Join(kinds, ","), store.UsageKindStorageCapacityGBSeconds) {
		t.Errorf("Key Value meters = %v, want no storage capacity meter", kinds)
	}

	p.mu.Lock()
	p.upSteps, p.byteSteps = 45, 45*10*gib
	p.mu.Unlock()
	next := window.Add(time.Hour)
	if !svc.processDatastoreMeterWindow(context.Background(), pg, store.UsageKindStorageCapacityGBSeconds, next) {
		t.Fatal("a partly covered reading was not written")
	}
	st.mu.Lock()
	row = st.rows[usageKey{store.ResourceKindPostgres, "dpg-db", store.UsageKindStorageCapacityGBSeconds, "basic-1gb", next}]
	st.mu.Unlock()
	if row.Quantity != 27000 || len(row.SourceHealth) != 1 || row.SourceHealth[0].State != store.UsageSourceDegraded {
		t.Errorf("partly covered capacity row = %+v, want 27,000 kept degraded", row)
	}
}

// The catch-up meters the capacity on its own cursor, so a resize Prometheus
// recorded while bex-api was down bills from the hour it happened: each window
// reads that window's claims.
func TestStorageCapacityCatchUpReadsEachWindow(t *testing.T) {
	last := time.Date(2026, 12, 3, 12, 0, 0, 0, time.UTC)
	resizedAt := last.Add(-time.Hour) // 10 GB until 11:00, 20 GB from then on
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		value := 0.0
		switch {
		case strings.Contains(query, `up{job="kube-state-metrics"}`):
			value = 60
		case strings.Contains(query, "kube_persistentvolumeclaim_resource_requests_storage_bytes"):
			var at int64 // the window's end, which the query is evaluated at
			_, _ = fmt.Sscan(r.URL.Query().Get("time"), &at)
			size := 10.0
			if time.Unix(at, 0).After(resizedAt) {
				size = 20
			}
			value = 60 * size * gib
		case isEgressHealthQuery(query):
			value = 1
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"%v"]}]}}`, value)
	}))
	defer srv.Close()
	st := newMemUsageStore()
	svc := NewService(&core.Base{}, st, srv.URL, nil)
	ds := datastoreEntry{ID: "dpg-db", Name: "dpg-db", Namespace: "tea-ws", TenantID: "tea-ws", Kind: store.ResourceKindPostgres, Plan: "basic-1gb",
		CreatedAt: last.Add(-3 * time.Hour)}
	svc.catchUpDatastoreMeterThrough(context.Background(), ds, store.UsageKindStorageCapacityGBSeconds, last)
	for window, want := range map[time.Time]int64{
		last.Add(-3 * time.Hour): 36000, last.Add(-2 * time.Hour): 36000, // before the resize
		last.Add(-time.Hour): 72000, last: 72000, // from it
	} {
		st.mu.Lock()
		row, ok := st.rows[usageKey{store.ResourceKindPostgres, "dpg-db", store.UsageKindStorageCapacityGBSeconds, "basic-1gb", window}]
		st.mu.Unlock()
		if !ok || row.Quantity != want {
			t.Errorf("window %s capacity = %+v present=%v, want %d", window.Format(time.Kitchen), row, ok, want)
		}
	}
}

// w4/m713 t006: from the switch a paid Postgres reads the same on REST,
// GraphQL and MCP: its capacity billed at $0.21/GB-month and its used bytes an
// unbilled reading at a rate of 0, and a Key Value's used bytes unbilled too,
// with the switch instant beside them. A 10 GB database with about 2 GB used,
// present all month, pays $2.10 for storage, as on Render (× 0.70).
func TestStorageCapacityAcrossAdapters(t *testing.T) {
	cutover := pricing.Default.StorageCapacityBilledFrom()
	if cutover.IsZero() {
		t.Fatal("the price sheet has no storage capacity switch")
	}
	st := newMemUsageStore()
	st.storageCapacityBilledFrom = cutover
	put := func(id, resourceKind, kind string, quantity int64) {
		t.Helper()
		if err := st.UpsertUsageHourly(context.Background(), store.HourlyRow{
			WorkspaceID: "tea-cap", ServiceID: id, ResourceKind: resourceKind,
			Kind: kind, Tier: "basic-1gb", WindowStart: cutover, Quantity: quantity,
		}); err != nil {
			t.Fatal(err)
		}
	}
	const month = 730 // hours in a pricing month, one row holding them all
	put("dpg-db", store.ResourceKindPostgres, store.UsageKindStorageCapacityGBSeconds, 36000*month)
	put("dpg-db", store.ResourceKindPostgres, store.UsageKindStorageGBSeconds, 7200*month)
	put("red-kv", store.ResourceKindKeyValue, store.UsageKindStorageGBSeconds, 3600*month)
	svc := svcWithTenant(st, "tea-cap")
	svc.Base.Clock = clockAt(cutover.Add(14 * 24 * time.Hour))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/v1/usage", nil).WithContext(ctx))
	if w.Code != http.StatusOK {
		t.Fatalf("REST: status %d: %s", w.Code, w.Body.String())
	}
	var rest usageResponse
	if err := json.NewDecoder(w.Body).Decode(&rest); err != nil {
		t.Fatal(err)
	}
	if got := rest.EstimatedCost.StorageCapacityBilledFrom; got != cutover.Format(time.RFC3339) {
		t.Errorf("REST storageCapacityBilledFrom = %q, want %s", got, cutover.Format(time.RFC3339))
	}
	type resource struct {
		id   string
		rows []usageRow
	}
	wantRows := []resource{
		{"dpg-db", []usageRow{
			{Kind: store.UsageKindStorageCapacityGBSeconds, Tier: "basic-1gb", Total: 36000 * month},
			{Kind: store.UsageKindStorageGBSeconds, Tier: "basic-1gb", Total: 7200 * month, Unbilled: true},
		}},
		{"red-kv", []usageRow{{Kind: store.UsageKindStorageGBSeconds, Tier: "basic-1gb", Total: 3600 * month, Unbilled: true}}},
	}
	var restRows []resource
	for _, s := range rest.Services {
		restRows = append(restRows, resource{s.ServiceID, s.Rows})
	}
	if !reflect.DeepEqual(restRows, wantRows) {
		t.Fatalf("REST rows = %+v, want %+v", restRows, wantRows)
	}
	type line struct {
		id, kind, unit, rate, quantity, cost string
		unbilled                             bool
	}
	wantLines := []line{
		{"dpg-db", store.UsageKindStorageCapacityGBSeconds, "GB-mo", "0.21", "10.00", "2.10", false},
		{"dpg-db", store.UsageKindStorageGBSeconds, "GB-mo", "0", "2.00", "0.00", true},
		{"red-kv", store.UsageKindStorageGBSeconds, "GB-mo", "0", "1.00", "0.00", true},
	}
	var restLines []line
	for _, r := range rest.EstimatedCost.Resources {
		for _, c := range r.Charges {
			restLines = append(restLines, line{r.ServiceID, c.Kind, c.Unit, c.RateUSD, c.Quantity, c.CostUSD, c.Unbilled})
		}
	}
	if !reflect.DeepEqual(restLines, wantLines) {
		t.Errorf("REST charge lines = %+v, want %+v", restLines, wantLines)
	}
	if rest.EstimatedCost.TotalUSD != "2.10" {
		t.Errorf("REST total = %s, want 2.10: the capacity alone", rest.EstimatedCost.TotalUSD)
	}

	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ usage {
		services { serviceId rows { kind tier total unbilled } }
		estimatedCost { totalUsd storageCapacityBilledFrom resources { serviceId charges { kind unit rateUsd quantity costUsd unbilled } } } } }`})
	if len(gql.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", gql.Errors)
	}
	usageData := gql.Data.(map[string]any)["usage"].(map[string]any)
	est := usageData["estimatedCost"].(map[string]any)
	if est["storageCapacityBilledFrom"] != rest.EstimatedCost.StorageCapacityBilledFrom || est["totalUsd"] != rest.EstimatedCost.TotalUSD {
		t.Errorf("GraphQL estimate = %v, want REST's %q and %s", est, rest.EstimatedCost.StorageCapacityBilledFrom, rest.EstimatedCost.TotalUSD)
	}
	var gqlLines []line
	for _, rawRes := range est["resources"].([]any) {
		r := rawRes.(map[string]any)
		for _, raw := range r["charges"].([]any) {
			c := raw.(map[string]any)
			gqlLines = append(gqlLines, line{r["serviceId"].(string), c["kind"].(string), c["unit"].(string), c["rateUsd"].(string),
				c["quantity"].(string), c["costUsd"].(string), c["unbilled"].(bool)})
		}
	}
	if !reflect.DeepEqual(gqlLines, wantLines) {
		t.Errorf("GraphQL charge lines = %+v, want REST's %+v", gqlLines, wantLines)
	}
	var gqlRows []resource
	for _, rawSvc := range usageData["services"].([]any) {
		s := rawSvc.(map[string]any)
		res := resource{id: s["serviceId"].(string)}
		for _, raw := range s["rows"].([]any) {
			r := raw.(map[string]any)
			res.rows = append(res.rows, usageRow{Kind: r["kind"].(string), Tier: r["tier"].(string), Total: int64(r["total"].(float64)), Unbilled: r["unbilled"].(bool)})
		}
		gqlRows = append(gqlRows, res)
	}
	if !reflect.DeepEqual(gqlRows, wantRows) {
		t.Errorf("GraphQL rows = %+v, want REST's %+v", gqlRows, wantRows)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "usage-test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "usage-test-client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_usage"})
	if err != nil || result.IsError {
		t.Fatalf("MCP get_usage: err=%v result=%+v", err, result)
	}
	rawMCP, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var viaMCP usageResponse
	if err := json.Unmarshal(rawMCP, &viaMCP); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(viaMCP.EstimatedCost, rest.EstimatedCost) || !reflect.DeepEqual(viaMCP.Services, rest.Services) {
		t.Errorf("MCP differs from REST:\nREST: %+v %+v\nMCP: %+v %+v", rest.Services, rest.EstimatedCost, viaMCP.Services, viaMCP.EstimatedCost)
	}
}
