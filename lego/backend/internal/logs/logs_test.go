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

package logs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// gqlSchema builds a Query-only schema from the logs fragment; runQuery runs a
// query against it and returns the data map (failing on any GraphQL error).
func gqlSchema(svc *Service) (graphql.Schema, error) {
	return graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
}

func runQuery(t *testing.T, schema graphql.Schema, query string) map[string]any {
	t.Helper()
	res := graphql.Do(graphql.Params{Schema: schema, RequestString: query, Context: context.Background()})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql errors: %v", res.Errors)
	}
	data, _ := res.Data.(map[string]any)
	return data
}

const webInst = "web-1"

func testScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return scheme
}

func sampleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning},
	}
}

func podFor(app, name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", Labels: map[string]string{core.PodLabelApp: app},
	}}
}

// staticLogs serves canned, timestamped lines per pod.
func staticLogs(lines map[string][]string) PodLogSource {
	return func(_ context.Context, _, pod, _ string, _ int64) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(strings.Join(lines[pod], "\n"))), nil
	}
}

func staticLogStream(lines map[string][]string) PodLogStream {
	return func(_ context.Context, _, pod, _ string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(strings.Join(lines[pod], "\n"))), nil
	}
}

func newService(logs map[string][]string, objs ...client.Object) *Service {
	return &Service{
		Base:          &core.Base{Client: fakeClientWith(objs...), Namespace: "default"},
		PodLogs:       staticLogs(logs),
		PodLogsFollow: staticLogStream(logs),
	}
}

// fakeClientWith builds a fake controller-runtime client seeded with objs — the
// shared seam under newService and the Loki-source tests.
func fakeClientWith(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).Build()
}

// decodeJSON is a thin json.Unmarshal over a string, for the Loki-response tests.
func decodeJSON(s string, v any) error { return json.Unmarshal([]byte(s), v) }

// --- Logs verb (MCP read path) ---

func TestLogsAggregatesAndSorts(t *testing.T) {
	svc := newService(map[string][]string{
		webInst: {"2026-07-05T00:00:01Z hello from 1", "2026-07-05T00:00:03Z later from 1"},
		"web-2": {"2026-07-05T00:00:02Z hello from 2"},
	}, sampleApp("web"), podFor("web", webInst), podFor("web", "web-2"))

	entries, err := svc.Logs(context.Background(), "web", 0)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	want := []string{"hello from 1", "hello from 2", "later from 1"} // interleaved by timestamp
	if len(entries) != len(want) {
		t.Fatalf("want %d lines, got %d", len(want), len(entries))
	}
	for i, w := range want {
		if entries[i].Message != w {
			t.Errorf("entry %d = %q, want %q", i, entries[i].Message, w)
		}
	}
	if entries[0].Labels["instance"] != webInst || entries[0].Labels["service"] != "web" {
		t.Errorf("missing render log labels: %+v", entries[0].Labels)
	}
}

func TestLogsErrors(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(sampleApp("web")).Build()

	nolog := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
	if _, err := nolog.Logs(context.Background(), "web", 0); !errors.Is(err, core.ErrLogsUnavailable) {
		t.Errorf("nil PodLogs => ErrLogsUnavailable, got %v", err)
	}
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, PodLogs: staticLogs(nil)}
	if _, err := svc.Logs(context.Background(), "nope", 0); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown app => ErrNotFound, got %v", err)
	}
}

// NormalizeTypes (shared by all three surfaces) maps Render's repeatable `type`
// filter onto the canonical set: the `application` alias folds into `app`, "all"
// means no narrowing, and an unknown value errors instead of silently widening the
// query to every type.
func TestNormalizeTypes(t *testing.T) {
	cases := []struct {
		in      []string
		want    []string
		wantErr bool
	}{
		{nil, nil, false},             // no filter => all
		{[]string{"all"}, nil, false}, // explicit all
		{[]string{"app"}, []string{"app"}, false},
		{[]string{"application"}, []string{"app"}, false}, // alias
		{[]string{"request"}, []string{"request"}, false},
		{[]string{"build"}, []string{"build"}, false},
		{[]string{"app", "request"}, []string{"app", "request"}, false}, // both kept — a real union
		{[]string{"app", "app"}, []string{"app"}, false},                // deduped
		{[]string{"bogus"}, nil, true},                                  // unknown => error, not "all"
	}
	for _, c := range cases {
		got, err := NormalizeTypes(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("NormalizeTypes(%v) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if err == nil && !slices.Equal(got, c.want) {
			t.Errorf("NormalizeTypes(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	// The error is a bad request (400), naming the offending value.
	_, err := NormalizeTypes([]string{"bogus"})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "bogus") {
		t.Errorf("unknown type must be a bad request naming the value, got %v", err)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty(nil); got != "" {
		t.Errorf("nil => %q", got)
	}
	if got := firstNonEmpty([]string{"", "hit", "miss"}); got != "hit" {
		t.Errorf("first non-empty => %q", got)
	}
}

// --- REST logs fragment (Render envelope) ---

func serveREST(svc *Service, method, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestRESTLogsEnvelopeAndFilters(t *testing.T) {
	svc := newService(map[string][]string{
		"web-1": {"2026-07-05T00:00:01Z first boot ok", "2026-07-05T00:00:03Z GET / 200"},
		"web-2": {"2026-07-05T00:00:02Z second boot ok"},
	}, sampleApp("web"), podFor("web", "web-1"), podFor("web", "web-2"))

	var env renderLogList
	rec := serveREST(svc, "GET", "/v1/logs?resource=web")
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Logs) != 3 || env.Logs[0].Message != "first boot ok" {
		t.Fatalf("want 3 lines sorted oldest-first: %+v", env.Logs)
	}
	if env.Logs[0].ID == "" || env.Logs[0].Labels[0].Name != "type" || env.Logs[0].Labels[0].Value != LogTypeApp {
		t.Errorf("render log id/type label wrong: %+v", env.Logs[0])
	}
	if env.NextStartTime == "" || env.NextEndTime == "" {
		t.Errorf("envelope must carry cursors: %+v", env)
	}

	// text search, limit+hasMore.
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/logs?resource=web&text=get").Body.Bytes(), &env)
	if len(env.Logs) != 1 || env.Logs[0].Message != "GET / 200" {
		t.Errorf("text=get should match one line: %+v", env.Logs)
	}
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/logs?resource=web&limit=1").Body.Bytes(), &env)
	if len(env.Logs) != 1 || !env.HasMore {
		t.Errorf("limit=1 => newest line + hasMore: %+v", env)
	}
}

func TestRESTLogsTypeAndErrors(t *testing.T) {
	svc := newService(map[string][]string{"web-1": {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", "web-1"))

	// `build` and `request` both live in the durable log store; without it the API
	// says so — 503 — instead of serving a fake empty page (w7/m28).
	if got := serveREST(svc, "GET", "/v1/logs?resource=web&type=build").Code; got != 503 {
		t.Errorf("type=build without the store => 503, got %d", got)
	}
	if got := serveREST(svc, "GET", "/v1/logs?resource=web&type=request").Code; got != 503 {
		t.Errorf("type=request without the store => 503, got %d", got)
	}
	if serveREST(svc, "GET", "/v1/logs").Code != 400 {
		t.Error("missing resource => 400")
	}
	if serveREST(svc, "GET", "/v1/logs?resource=nope").Code != 404 {
		t.Error("unknown app => 404")
	}
	if serveREST(svc, "GET", "/v1/logs?resource=web&type=bogus").Code != 400 {
		t.Error("bad type => 400")
	}
	if serveREST(svc, "GET", "/v1/logs?resource=web&direction=sideways").Code != 400 {
		t.Error("bad direction => 400")
	}
}

// The honesty rule, on the surface that enforces it: in pod-log fallback mode a
// structured filter bex cannot honor is refused (503) — never accepted and
// ignored, which would answer a narrow question with unfiltered lines.
func TestRESTStructuredFiltersRefusedWithoutStore(t *testing.T) {
	svc := newService(map[string][]string{"web-1": {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", "web-1"))

	for _, filter := range []string{"level=error", "statusCode=500", "method=GET", "path=/x", "host=web.example.com"} {
		rec := serveREST(svc, "GET", "/v1/logs?resource=web&"+filter)
		if rec.Code != 503 {
			t.Errorf("%s without the store => 503, got %d (%s)", filter, rec.Code, rec.Body.String())
		}
	}
	// `instance` is the one structured filter the pod-log path CAN honor — a pod
	// name is a pod name — so it narrows rather than refusing.
	var env renderLogList
	rec := serveREST(svc, "GET", "/v1/logs?resource=web&instance=web-1")
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || rec.Code != 200 || len(env.Logs) != 1 {
		t.Errorf("instance filter should be honored on pod logs: %d %+v", rec.Code, env.Logs)
	}
	if rec = serveREST(svc, "GET", "/v1/logs?resource=web&instance=web-9"); rec.Code != 200 {
		t.Errorf("unknown instance => 200 empty, got %d", rec.Code)
	} else if _ = json.Unmarshal(rec.Body.Bytes(), &env); len(env.Logs) != 0 {
		t.Errorf("instance=web-9 matches no replica, want no lines: %+v", env.Logs)
	}
}

// The request-log split over the REST surface, end to end: `type=request` reaches
// the store's request streams and comes back as Render log lines whose labels tell
// the truth (type/method/statusCode), and `/v1/logs/values` discovers real values.
func TestRESTRequestLogsAndDiscovery(t *testing.T) {
	access := `{\"ServiceName\":\"default-web-80@kubernetes\",\"RequestMethod\":\"GET\",\"RequestPath\":\"/healthz\",\"DownstreamStatus\":200}`
	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"app":"web","type":"request","method":"GET","status":"200"}`,
		"values": `[["1751673601000000000","` + access + `"]]`,
	}))
	defer f.srv.Close()

	svc := newService(map[string][]string{"web-1": {"2026-07-05T00:00:01Z app line"}},
		sampleApp("web"), podFor("web", "web-1"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	svc.LabelValues = NewLokiLabelValuesSource(f.srv.URL, f.srv.Client())

	var env renderLogList
	rec := serveREST(svc, "GET", "/v1/logs?resource=web&type=request&statusCode=2xx&method=GET&path=/healthz")
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &env) != nil || len(env.Logs) != 1 {
		t.Fatalf("type=request => the access line, got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(env.Logs[0].Message, "/healthz") {
		t.Errorf("the access line itself must be returned: %q", env.Logs[0].Message)
	}
	labels := map[string]string{}
	for _, l := range env.Logs[0].Labels {
		labels[l.Name] = l.Value
	}
	if labels[LabelType] != LogTypeRequest || labels[LabelMethod] != "GET" || labels[LabelStatusCode] != "200" {
		t.Errorf("request line labels must be truthful: %+v", labels)
	}
	// The filters reached the store as LogQL rather than being dropped on the floor.
	if q := f.lastValues.Get("query"); !strings.Contains(q, `type="request"`) ||
		!strings.Contains(q, `method="GET"`) || !strings.Contains(q, `status=~"^(2.*)$"`) ||
		!strings.Contains(q, `request_path="/healthz"`) {
		t.Errorf("filters not pushed down as LogQL: %s", q)
	}

	// Discovery: GET /v1/logs/values?label=… returns Render's bare string array.
	f.body = `{"status":"success","data":["error","info"]}`
	rec = serveREST(svc, "GET", "/v1/logs/values?resource=web&label=level")
	var values []string
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &values) != nil || !slices.Equal(values, []string{"error", "info"}) {
		t.Errorf("label discovery => [error info], got %d %s", rec.Code, rec.Body.String())
	}
	if rec = serveREST(svc, "GET", "/v1/logs/values?resource=web"); rec.Code != 400 {
		t.Errorf("missing label => 400, got %d", rec.Code)
	}
	if rec = serveREST(svc, "GET", "/v1/logs/values?resource=web&label=bogus"); rec.Code != 400 {
		t.Errorf("unknown label => 400, got %d", rec.Code)
	}
}

// BEX_MAX_QUERY_HOURS bounds every REST log read — the historical query AND label
// discovery, which takes the same time range and would otherwise let a caller scan
// the whole store.
func TestRESTMaxQueryHoursBoundsBothReads(t *testing.T) {
	svc := newService(map[string][]string{"web-1": {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", "web-1"))
	svc.MaxQueryHours = 24

	wide := "startTime=2020-01-01T00:00:00Z&endTime=2026-01-01T00:00:00Z"
	if got := serveREST(svc, "GET", "/v1/logs?resource=web&"+wide).Code; got != 400 {
		t.Errorf("an over-wide query range => 400, got %d", got)
	}
	if got := serveREST(svc, "GET", "/v1/logs/values?resource=web&label=level&"+wide).Code; got != 400 {
		t.Errorf("discovery must honor the same window cap => 400, got %d", got)
	}
}

func TestRESTLogsUnavailableWithoutSource(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(sampleApp("web")).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}} // no PodLogs
	if serveREST(svc, "GET", "/v1/logs?resource=web").Code != 503 {
		t.Error("no source => 503")
	}
}

func TestRESTLogsSubscribeSSE(t *testing.T) {
	svc := newService(map[string][]string{
		"web-1": {"2026-07-05T00:00:01Z live one", "2026-07-05T00:00:02Z live two"},
	}, sampleApp("web"), podFor("web", "web-1"))

	rec := serveREST(svc, "GET", "/v1/logs/subscribe?resource=web")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("subscribe => 200 SSE, got %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "data: ") || !strings.Contains(body, "live one") || !strings.Contains(body, "live two") {
		t.Errorf("SSE body missing streamed lines: %q", body)
	}
}

// --- GraphQL logs fragment ---

func TestGraphQLLogs(t *testing.T) {
	svc := newService(map[string][]string{"web-1": {"2026-07-05T00:00:01Z hello"}},
		sampleApp("web"), podFor("web", "web-1"))

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web") { message type instance } }`)
	list := data["logs"].([]any)
	if len(list) != 1 {
		t.Fatalf("want 1 log, got %d", len(list))
	}
	first := list[0].(map[string]any)
	if first["message"] != "hello" || first["type"] != LogTypeApp || first["instance"] != "web-1" {
		t.Fatalf("unexpected log shape: %+v", first)
	}
}

// TestGraphQLLogsTimeWindow covers w9/m1/t002: startTime/endTime filter to the
// window exactly as REST's ?startTime=&endTime= does (both routed through the
// same LogQuery.Since/.End the pod-log fallback's keep() applies) — the
// deploy detail page's log panel depends on this to scope a query to one
// deploy's createdAt..finishedAt range.
func TestGraphQLLogsTimeWindow(t *testing.T) {
	svc := newService(map[string][]string{
		webInst: {
			"2026-07-05T00:00:01Z before the window",
			"2026-07-05T00:00:05Z inside the window",
			"2026-07-05T00:00:09Z after the window",
		},
	}, sampleApp("web"), podFor("web", webInst))

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", startTime:"2026-07-05T00:00:03Z", endTime:"2026-07-05T00:00:07Z") { message } }`)
	list := data["logs"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["message"] != "inside the window" {
		t.Fatalf("windowed logs = %+v, want exactly [inside the window]", list)
	}
}

// TestGraphQLLogsMalformedTimeErrors covers w9/m1/t002's error-shape parity:
// a malformed startTime/endTime is a resolver error naming the offending
// field, mirroring REST's `%w: startTime: …`/`%w: endTime: …` (rest.go) —
// never a silently-dropped filter.
func TestGraphQLLogsMalformedTimeErrors(t *testing.T) {
	svc := newService(map[string][]string{webInst: {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", webInst))
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	for _, tc := range []struct{ field, query string }{
		{"startTime", `{ logs(resource:"web", startTime:"not-a-time") { message } }`},
		{"endTime", `{ logs(resource:"web", endTime:"not-a-time") { message } }`},
	} {
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: tc.query, Context: context.Background()})
		if len(res.Errors) == 0 {
			t.Fatalf("malformed %s: want a resolver error, got none", tc.field)
		}
		if !strings.Contains(res.Errors[0].Message, tc.field+":") {
			t.Errorf("malformed %s error = %q, want it to name the field", tc.field, res.Errors[0].Message)
		}
	}
}

// TestGraphQLLogsMaxQueryHoursBounds covers w9/m1/t008: BEX_MAX_QUERY_HOURS
// bounds GraphQL's logs/logLabelValues exactly as it bounds REST's (rest.go's
// checkWindow, now called from both resolvers, graphql.go) — an over-wide
// window is a 400-shaped resolver error on every surface, never unbounded on
// one and capped on another.
func TestGraphQLLogsMaxQueryHoursBounds(t *testing.T) {
	svc := newService(map[string][]string{webInst: {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", webInst))
	svc.MaxQueryHours = 24
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	wide := `startTime:"2020-01-01T00:00:00Z", endTime:"2026-01-01T00:00:00Z"`
	for _, q := range []string{
		`{ logs(resource:"web", ` + wide + `) { message } }`,
		`{ logLabelValues(resource:"web", label:"level", ` + wide + `) }`,
	} {
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: q, Context: context.Background()})
		if len(res.Errors) == 0 {
			t.Errorf("%s: an over-wide window => resolver error, got none", q)
		}
	}
}

// TestMCPLogsMaxQueryHoursBounds covers w9/004: BEX_MAX_QUERY_HOURS bounds
// MCP's list_logs/list_log_label_values exactly as it bounds REST and GraphQL
// (rest.go's checkWindow) — before this, MCP was the one surface that could
// scan an unbounded startTime..endTime range.
func TestMCPLogsMaxQueryHoursBounds(t *testing.T) {
	svc := newService(map[string][]string{webInst: {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", webInst))
	svc.MaxQueryHours = 24

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })

	wide := map[string]any{
		"resource":  []string{"web"},
		"startTime": "2020-01-01T00:00:00Z",
		"endTime":   "2026-01-01T00:00:00Z",
	}
	for _, tool := range []string{"list_logs", "list_log_label_values"} {
		args := map[string]any{}
		for k, v := range wide {
			args[k] = v
		}
		if tool == "list_log_label_values" {
			args["label"] = "level"
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("%s: transport error: %v", tool, err)
		}
		if !res.IsError {
			t.Errorf("%s: an over-wide window => tool error, got success", tool)
		}
	}
}
