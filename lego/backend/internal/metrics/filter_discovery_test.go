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
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestNewPrometheusInstanceFilterValuesSource(t *testing.T) {
	now := fixedClock()
	start := now.Add(-2 * time.Hour)

	var gotPath string
	var gotQuery url.Values
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"status":"success","data":["web-old-11111","web-live-22222"]}`))
	}))
	defer prom.Close()

	src := NewPrometheusInstanceFilterValuesSource(prom.URL, prom.Client())

	t.Run("compute service selector and time window", func(t *testing.T) {
		pods, err := src(context.Background(), InstanceFilterValuesRequest{
			Namespace: "tea-a", Resource: "web", Start: start, End: now,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotPath != "/api/v1/label/pod/values" {
			t.Errorf("path = %q, want /api/v1/label/pod/values", gotPath)
		}
		match := gotQuery.Get("match[]")
		for _, want := range []string{
			`container_cpu_usage_seconds_total{`,
			`namespace="tea-a"`,
			`container!=""`,
			`container!="edge-cache"`,
			`container!="POD"`,
		} {
			if !strings.Contains(match, want) {
				t.Errorf("match[] = %q, want substring %q", match, want)
			}
		}
		if gotQuery.Get("start") != strconv.FormatInt(start.Unix(), 10) || gotQuery.Get("end") != strconv.FormatInt(now.Unix(), 10) {
			t.Errorf("window = %s..%s, want %d..%d", gotQuery.Get("start"), gotQuery.Get("end"), start.Unix(), now.Unix())
		}
		if !slices.Equal(pods, []string{"web-old-11111", "web-live-22222"}) {
			t.Errorf("pods = %v, want [web-old-11111 web-live-22222]", pods)
		}
	})

	t.Run("postgres selector covers primary, standby, and read replicas", func(t *testing.T) {
		if _, err := src(context.Background(), InstanceFilterValuesRequest{
			Kind: DatastoreDatabase, Namespace: "tea-a", Resource: "dpg-abc", Start: start, End: now,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		match := gotQuery.Get("match[]")
		if !strings.Contains(match, `container="postgres"`) || !strings.Contains(match, instancePods(DatastoreDatabase, "dpg-abc")) {
			t.Errorf("postgres match[] = %q, want postgres container and instancePods pattern", match)
		}
	})

	t.Run("keyvalue selector targets valkey container", func(t *testing.T) {
		if _, err := src(context.Background(), InstanceFilterValuesRequest{
			Kind: DatastoreKeyValue, Namespace: "tea-a", Resource: "red-abc", Start: start, End: now,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		match := gotQuery.Get("match[]")
		if !strings.Contains(match, `container="valkey"`) || !strings.Contains(match, instancePods(DatastoreKeyValue, "red-abc")) {
			t.Errorf("keyvalue match[] = %q, want valkey container and instancePods pattern", match)
		}
	})

	t.Run("inverted window short-circuits without hitting Prometheus", func(t *testing.T) {
		gotPath = ""
		pods, err := src(context.Background(), InstanceFilterValuesRequest{
			Namespace: "tea-a", Resource: "web", Start: now, End: start,
		})
		if err != nil || len(pods) != 0 || gotPath != "" {
			t.Fatalf("inverted window = (%v, %v, path=%q), want empty with no upstream call", pods, err, gotPath)
		}
	})
}

func TestNewLokiHTTPFilterValuesSource(t *testing.T) {
	now := fixedClock()
	start := now.Add(-time.Hour)

	var gotQuery url.Values
	loki := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{
			"status": "success",
			"data": {
				"resultType": "streams",
				"result": [
					{"stream": {"status": "200", "request_host": "b.example.com"}, "values": [["100", "b.example.com"], ["101", "b.example.com"]]},
					{"stream": {"status": "502", "request_host": "a.example.com"}, "values": [["102", "a.example.com"]]},
					{"stream": {"status": "204"}, "values": [["103", ""]]},
					{"stream": {"status": "418", "request_host": "empty.example.com"}, "values": []}
				]
			}
		}`))
	}))
	defer loki.Close()

	src := NewLokiHTTPFilterValuesSource(loki.URL, loki.Client())
	got, err := src(context.Background(), HTTPFilterValuesRequest{
		Namespace:   "tea-a",
		App:         "web",
		Start:       start,
		End:         now,
		Hosts:       []string{"a.example.com", "b.example.com"},
		StatusCodes: []string{"200", "5xx"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got.Hosts, []string{"a.example.com", "b.example.com"}) {
		t.Errorf("hosts = %v, want [a.example.com b.example.com]", got.Hosts)
	}
	if !slices.Equal(got.StatusCodes, []string{"200", "204", "502"}) {
		t.Errorf("statusCodes = %v, want [200 204 502] (empty stream ignored)", got.StatusCodes)
	}
	logql := gotQuery.Get("query")
	for _, want := range []string{
		`namespace="tea-a"`,
		`app="web"`,
		`type="request"`,
		`status=~"^(200|5..)$"`,
		`request_host="a.example.com" or request_host="b.example.com"`,
		`line_format "{{with .request_host}}{{.}}{{end}}"`,
	} {
		if !strings.Contains(logql, want) {
			t.Errorf("LogQL = %q, want substring %q", logql, want)
		}
	}
	if gotQuery.Get("direction") != "backward" || gotQuery.Get("limit") != "5000" {
		t.Errorf("params direction=%q limit=%q, want backward / 5000", gotQuery.Get("direction"), gotQuery.Get("limit"))
	}
	if gotQuery.Get("end") != strconv.FormatInt(now.Add(time.Nanosecond).UnixNano(), 10) {
		t.Errorf("end = %q, want end+1ns", gotQuery.Get("end"))
	}
}

func TestApplicationFilterDiscoveryHistoricalPodsStaticAndDatastores(t *testing.T) {
	now := fixedClock()
	start := now.Add(-3 * time.Hour)
	const (
		webID  = "srv-a0000000000000000001"
		siteID = "srv-a0000000000000000002"
		pgID   = "dpg-d0000000000000000001"
		kvID   = "red-d0000000000000000001"
	)

	web := sampleApp("web")
	web.Labels = map[string]string{core.LabelAppID: webID}
	site := sampleApp("site")
	site.Labels = map[string]string{core.LabelAppID: siteID}
	site.Spec.Type = appv1alpha1.TypeStaticSite
	pg := sampleDatabase(pgID, true)
	pg.Spec.ReadReplicas = []appv1alpha1.DatabaseReadReplica{{Name: "ro"}}
	kv := sampleKeyValue(kvID)

	// Live pods: web-live, pg primary/standby/replica + an initdb job pod (must be ignored), kv-0.
	pgPrimaryPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: pgID + "-1", Namespace: "default", Labels: map[string]string{core.PodLabelCNPGCluster: pgID},
	}}
	pgReplicaPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: appv1alpha1.ReadReplicaClusterName(pgID, "ro") + "-1", Namespace: "default",
		Labels: map[string]string{core.PodLabelCNPGCluster: appv1alpha1.ReadReplicaClusterName(pgID, "ro")},
	}}
	pgInitJobPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: pgID + "-1-initdb-xyz", Namespace: "default", Labels: map[string]string{core.PodLabelCNPGCluster: pgID},
	}}
	kvPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: kvID + "-0", Namespace: "default", Labels: map[string]string{core.PodLabelKeyValue: kvID},
	}}

	svc := newService(nil, nil, web, site, pg, kv, podFor("web", "web-abc12-live1"), pgPrimaryPod, pgReplicaPod, pgInitJobPod, kvPod)

	var promCalls []InstanceFilterValuesRequest
	svc.InstanceFilterValuesSource = func(_ context.Context, req InstanceFilterValuesRequest) ([]string, error) {
		promCalls = append(promCalls, req)
		switch req.Resource {
		case "web":
			// Terminated historical pod + live pod (deduplicated).
			return []string{"web-old99-term1", "web-abc12-live1"}, nil
		case pgID:
			// Standby pod that ran during window and was replaced.
			return []string{pgID + "-2"}, nil
		case kvID:
			return []string{kvID + "-0"}, nil
		default:
			return nil, nil
		}
	}

	// 1. Compute service returns union of historical Prometheus pod and live pod.
	qs := url.Values{
		"resource":  {webID},
		"startTime": {start.Format(time.RFC3339)},
		"endTime":   {now.Format(time.RFC3339)},
	}.Encode()
	rec := serveREST(svc, "/v1/metrics/filters/application?"+qs)
	var got []renderFilterValues
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("REST filters/application = %d %s", rec.Code, rec.Body)
	}
	wantWeb := []string{
		ids.ServiceInstanceID(webID, "web-abc12-live1"),
		ids.ServiceInstanceID(webID, "web-old99-term1"),
	}
	slices.Sort(wantWeb)
	if len(got) != 1 || got[0].Filter != "instance" || !slices.Equal(got[0].Values, wantWeb) {
		t.Fatalf("REST web instances = %+v, want %v", got, wantWeb)
	}

	// GraphQL returns the exact same set for the same window.
	gqlOut := execMetricsFiltersGQL(t, svc, `{
		metricsFilters(query: {
			filters: [{field: "RESOURCE", values: ["`+webID+`"]}],
			start: "`+start.Format(time.RFC3339)+`",
			end: "`+now.Format(time.RFC3339)+`",
			outputFilters: ["INSTANCE"]
		}) { values { field values } }
	}`)
	if !slices.Equal(gqlOut[filterFieldInstance], wantWeb) {
		t.Errorf("GraphQL web instances = %v, want %v", gqlOut[filterFieldInstance], wantWeb)
	}

	// 2. Static site still returns [] and never queries Prometheus for pods.
	promCalls = nil
	rec = serveREST(svc, "/v1/metrics/filters/application?resource="+siteID)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || len(got) != 1 || len(got[0].Values) != 0 {
		t.Fatalf("static site instances = %d %s, want empty []", rec.Code, rec.Body)
	}
	if len(promCalls) != 0 {
		t.Errorf("static site queried Prometheus instances: %+v", promCalls)
	}

	// 3. Datastores (Postgres primary + standby + read-replica, excluding initdb job; Key Value) work across multi-resource queries.
	rec = serveREST(svc, "/v1/metrics/filters/application?resource="+pgID+"&resource="+kvID)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("datastore filters/application = %d %s", rec.Code, rec.Body)
	}
	wantDS := []string{
		ids.ServiceInstanceID(pgID, pgID+"-1"),
		ids.ServiceInstanceID(pgID, pgID+"-2"),
		ids.ServiceInstanceID(pgID, appv1alpha1.ReadReplicaClusterName(pgID, "ro")+"-1"),
		ids.ServiceInstanceID(kvID, kvID+"-0"),
	}
	slices.Sort(wantDS)
	if len(got) != 1 || !slices.Equal(got[0].Values, wantDS) {
		t.Fatalf("datastore instances = %+v, want %v", got, wantDS)
	}

	// 4. Datastores on /filters/http are refused with 400 (after authorization).
	if rec := serveREST(svc, "/v1/metrics/filters/http?resource="+pgID); rec.Code != http.StatusBadRequest {
		t.Errorf("filters/http on Postgres = %d %s, want 400", rec.Code, rec.Body)
	}
	if rec := serveREST(svc, "/v1/metrics/filters/http?resource=dpg-d0000000000000000099"); rec.Code != http.StatusNotFound {
		t.Errorf("filters/http on unknown Postgres = %d %s, want 404", rec.Code, rec.Body)
	}

	// 5. Fallback when Prometheus is unwired returns live pods cleanly.
	svc.InstanceFilterValuesSource = nil
	rec = serveREST(svc, "/v1/metrics/filters/application?resource="+webID)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil ||
		len(got) != 1 || !slices.Equal(got[0].Values, []string{ids.ServiceInstanceID(webID, "web-abc12-live1")}) {
		t.Fatalf("unwired Prometheus fallback = %d %s, want live pod only", rec.Code, rec.Body)
	}
}

func TestMultiResourceCrossFilteredHTTPAndAuthz(t *testing.T) {
	now := fixedClock()
	start := now.Add(-2 * time.Hour)
	web := sampleApp("web")
	web.Labels = map[string]string{core.LabelAppID: "srv-web0000000000000001"}
	apiApp := sampleApp("api")
	apiApp.Labels = map[string]string{core.LabelAppID: "srv-api0000000000000002"}
	delApp := deleting(sampleApp("gone"))
	delApp.Labels = map[string]string{core.LabelAppID: "srv-del0000000000000003"}

	svc := newService(nil, nil, web, apiApp, delApp,
		ingressFor("web", "web", "web.example.com"),
		ingressFor("api", "api", "api.example.com"),
	)
	svc.MaxQueryHours = 24

	lokiCalls := 0
	promCalls := 0
	svc.MetricsFilterValuesSource = func(_ context.Context, req MetricsFilterValuesRequest) ([]string, error) {
		promCalls++
		if req.App == "web" {
			return []string{"200", "304"}, nil
		}
		return []string{"200", "503"}, nil
	}
	svc.HTTPFilterValuesSource = func(_ context.Context, req HTTPFilterValuesRequest) (HTTPFilterValues, error) {
		lokiCalls++
		switch req.App {
		case "web":
			if slices.Contains(req.Hosts, "api.example.com") && !slices.Contains(req.Hosts, "web.example.com") {
				return HTTPFilterValues{}, nil
			}
			if slices.Contains(req.StatusCodes, "5xx") {
				return HTTPFilterValues{}, nil
			}
			return HTTPFilterValues{Hosts: []string{"web.example.com"}, StatusCodes: []string{"200", "404"}}, nil
		case "api":
			if slices.Contains(req.Hosts, "web.example.com") && !slices.Contains(req.Hosts, "api.example.com") {
				return HTTPFilterValues{}, nil
			}
			if slices.Contains(req.StatusCodes, "5xx") {
				return HTTPFilterValues{Hosts: []string{"api.example.com"}, StatusCodes: []string{"500", "503"}}, nil
			}
			return HTTPFilterValues{Hosts: []string{"api.example.com"}, StatusCodes: []string{"200", "500"}}, nil
		default:
			return HTTPFilterValues{}, nil
		}
	}

	// 1. Multi-resource unfiltered union across REST and GraphQL.
	qs := url.Values{
		"resource":  {"web", "api"},
		"startTime": {start.Format(time.RFC3339)},
		"endTime":   {now.Format(time.RFC3339)},
	}.Encode()
	rec := serveREST(svc, "/v1/metrics/filters/http?"+qs)
	var restOut []renderFilterValues
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &restOut) != nil {
		t.Fatalf("multi-resource REST filters/http = %d %s", rec.Code, rec.Body)
	}
	wantCodes := []string{"200", "304", "404", "500", "503"}
	wantHosts := []string{"api.example.com", "web.example.com"}
	if len(restOut) != 2 || !slices.Equal(restOut[0].Values, wantCodes) || !slices.Equal(restOut[1].Values, wantHosts) {
		t.Fatalf("multi-resource REST = %+v, want codes %v hosts %v", restOut, wantCodes, wantHosts)
	}

	gqlOut := execMetricsFiltersGQL(t, svc, `{
		metricsFilters(query: {
			filters: [{field: "RESOURCE", values: ["web", "api"]}],
			start: "`+start.Format(time.RFC3339)+`",
			end: "`+now.Format(time.RFC3339)+`",
			outputFilters: ["STATUS_CODE", "HOST", "RESOURCE"]
		}) { values { field values } }
	}`)
	if !slices.Equal(gqlOut[filterFieldStatusCode], wantCodes) ||
		!slices.Equal(gqlOut[filterFieldHost], wantHosts) ||
		!slices.Equal(gqlOut[filterFieldResource], []string{"srv-web0000000000000001", "srv-api0000000000000002"}) {
		t.Fatalf("multi-resource GraphQL = %+v, want codes %v hosts %v", gqlOut, wantCodes, wantHosts)
	}

	// 2. Cross-filter by STATUS_CODE=5xx narrows hosts identically on REST and GraphQL without calling Prometheus.
	promCalls = 0
	rec = serveREST(svc, "/v1/metrics/filters/http?resource=web&service=api&statusCode=5xx")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &restOut) != nil {
		t.Fatalf("cross-filtered REST = %d %s", rec.Code, rec.Body)
	}
	if promCalls != 0 {
		t.Errorf("cross-filtered query called Prometheus %d times, want 0", promCalls)
	}
	if !slices.Equal(restOut[0].Values, []string{"500", "503"}) || !slices.Equal(restOut[1].Values, []string{"api.example.com"}) {
		t.Fatalf("cross-filtered REST = %+v, want codes [500 503] hosts [api.example.com]", restOut)
	}
	gqlCross := execMetricsFiltersGQL(t, svc, `{
		metricsFilters(query: {
			filters: [
				{field: "RESOURCE", values: ["web", "api"]},
				{field: "STATUS_CODE", values: ["5xx"]}
			],
			outputFilters: ["STATUS_CODE", "HOST"]
		}) { values { field values } }
	}`)
	if !slices.Equal(gqlCross[filterFieldStatusCode], []string{"500", "503"}) ||
		!slices.Equal(gqlCross[filterFieldHost], []string{"api.example.com"}) {
		t.Fatalf("cross-filtered GraphQL = %+v, want codes [500 503] hosts [api.example.com]", gqlCross)
	}

	// 3. Dashboard-shaped STATUS_CODE-only unfiltered call uses Prometheus and does NOT scan Loki.
	lokiCalls = 0
	promCalls = 0
	if _, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		Resources: []string{"web"}, OutputFilters: []string{filterFieldInstance, filterFieldStatusCode},
	}); err != nil {
		t.Fatalf("dashboard-shaped MetricsFilters: %v", err)
	}
	if lokiCalls != 0 || promCalls != 1 {
		t.Errorf("STATUS_CODE-only unfiltered calls: loki=%d prom=%d, want loki=0 prom=1", lokiCalls, promCalls)
	}

	// 4. Validation & fail-closed authz: bad statusCode, inverted range, MaxQueryHours, deleting/unknown resource.
	lokiCalls, promCalls = 0, 0
	if _, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		Resources: []string{"web"}, StatusCodes: []string{"999"}, OutputFilters: []string{filterFieldHost},
	}); !errors.Is(err, core.ErrBadRequest) || lokiCalls != 0 || promCalls != 0 {
		t.Errorf("invalid statusCode = (%v, loki=%d, prom=%d), want ErrBadRequest with zero source calls", err, lokiCalls, promCalls)
	}
	if _, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		Resources: []string{"web"}, Start: now, End: start, OutputFilters: []string{filterFieldStatusCode},
	}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("inverted range = %v, want ErrBadRequest", err)
	}
	if _, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		Resources: []string{"web"}, Start: now.Add(-48 * time.Hour), End: now, OutputFilters: []string{filterFieldStatusCode},
	}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("over-MaxQueryHours = %v, want ErrBadRequest", err)
	}
	// Multi-resource fails closed if ANY resource is unknown or deleting.
	if _, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		Resources: []string{"web", "nope"}, OutputFilters: []string{filterFieldStatusCode},
	}); !errors.Is(err, core.ErrNotFound) || lokiCalls != 0 || promCalls != 0 {
		t.Errorf("mixed unknown resource = (%v, loki=%d, prom=%d), want ErrNotFound with zero source calls", err, lokiCalls, promCalls)
	}
	if _, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		Resources: []string{"web", "gone"}, OutputFilters: []string{filterFieldInstance},
	}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("mixed deleting resource = %v, want ErrNotFound", err)
	}
	// Multi-resource fails closed when caller lacks permission on one resource.
	svc.Authz = denyResourceChecker{denyTenant: "ws-other"}
	other := sampleApp("other")
	other.Labels = map[string]string{core.LabelTenant: "ws-other"}
	if err := svc.Client.Create(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		Resources: []string{"web", "other"}, OutputFilters: []string{filterFieldInstance},
	}); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("unauthorized second resource = %v, want ErrForbidden", err)
	}
}

func TestMetricsFiltersAppNotBeforeClamping(t *testing.T) {
	now := fixedClock()
	created := now.Add(-30 * time.Minute)
	web := sampleApp("web")
	web.CreationTimestamp = metav1.NewTime(created)

	svc := newService(nil, nil, web, podFor("web", webInst), ingressFor("web", "web", "web.example.com"))
	var gotPromStart time.Time
	svc.InstanceFilterValuesSource = func(_ context.Context, req InstanceFilterValuesRequest) ([]string, error) {
		gotPromStart = req.Start
		return []string{"web-old"}, nil
	}

	// Start before creation clamps to AppNotBefore.
	if _, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		Resources: []string{"web"}, Start: now.Add(-2 * time.Hour), End: now, OutputFilters: []string{filterFieldInstance},
	}); err != nil {
		t.Fatalf("MetricsFilters: %v", err)
	}
	if !gotPromStart.Equal(created) {
		t.Errorf("clamped start = %s, want AppNotBefore %s", gotPromStart, created)
	}

	// Window wholly before App creation returns [] without querying Prometheus.
	gotPromStart = time.Time{}
	out, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		Resources: []string{"web"}, Start: now.Add(-3 * time.Hour), End: now.Add(-time.Hour), OutputFilters: []string{filterFieldInstance},
	})
	if err != nil || len(out) != 1 || len(out[0].Values) != 0 || !gotPromStart.IsZero() {
		t.Fatalf("pre-creation window = (%+v, %v, promStart=%v), want empty with no Prometheus call", out, err, gotPromStart)
	}
}

type denyResourceChecker struct {
	denyTenant string
}

func (c denyResourceChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	return object != "workspace:"+c.denyTenant, nil
}

func execMetricsFiltersGQL(t *testing.T, svc *Service, query string) map[string][]string {
	t.Helper()
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL %s errors: %v", query, res.Errors)
	}
	var payload struct {
		MetricsFilters struct {
			Values []struct {
				Field  string   `json:"field"`
				Values []string `json:"values"`
			} `json:"values"`
		} `json:"metricsFilters"`
	}
	if err := json.Unmarshal([]byte(mustJSON(res.Data)), &payload); err != nil {
		t.Fatalf("decode %v: %v", res.Data, err)
	}
	out := make(map[string][]string, len(payload.MetricsFilters.Values))
	for _, entry := range payload.MetricsFilters.Values {
		out[entry.Field] = entry.Values
	}
	return out
}
