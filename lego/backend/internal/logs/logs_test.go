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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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

const postgresID = "dpg-c185th5c2rvvnhbfiltg"

func sampleDatabase(name string) *appv1alpha1.Database {
	return &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
}

func databasePod(database, name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", Labels: map[string]string{core.PodLabelCNPGCluster: database},
	}}
}

func podFor(app, name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", Labels: map[string]string{core.PodLabelApp: app},
	}}
}

func buildPodFor(app, name, namespace, container string, created time.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			Labels:            map[string]string{core.PodLabelBuild: app},
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: container}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  container,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
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

type homeWorkspaceOnly struct{}

func (homeWorkspaceOnly) Tenant(context.Context, core.Identity) (string, bool) {
	return "tea-home", true
}

func (homeWorkspaceOnly) IsMember(_ context.Context, _ core.Identity, tenantID string) (bool, error) {
	return tenantID == "tea-home", nil
}

type denyLogChecker struct{}

func (denyLogChecker) Check(context.Context, string, string, string) (bool, error) {
	return false, nil
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

func TestLogResourceFanoutIsBoundedAndDeduplicated(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/logs?resource=web&resource=web&resource=worker", nil)
	resources, _, err := parseLogParams(request)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(resources, []string{"web", "worker"}) {
		t.Fatalf("resources = %v, want first-seen unique values", resources)
	}

	request = httptest.NewRequest(http.MethodGet, "/v1/logs?"+strings.Repeat("resource=web&", maxLogResources+1), nil)
	if _, _, err := parseLogParams(request); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("oversized resource array error = %v, want bad request", err)
	}
}

func TestManagedPostgresLogsAcrossRESTGraphQLAndMCP(t *testing.T) {
	pod := postgresID + "-1"
	otherPod := "dpg-d185th5c2rvvnhbfiltg-1"
	svc := newService(map[string][]string{
		pod:      {"2026-07-05T00:00:01Z checkpoint complete"},
		otherPod: {"2026-07-05T00:00:02Z must not leak"},
	}, sampleDatabase(postgresID), databasePod(postgresID, pod), databasePod("dpg-d185th5c2rvvnhbfiltg", otherPod))

	// REST keeps Render's generic resource filter and returns the immutable dpg-
	// id, the Postgres type, and the CNPG instance without a database-only route.
	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+postgresID+"&text=checkpoint&instance="+pod)
	var env renderLogList
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &env) != nil || len(env.Logs) != 1 {
		t.Fatalf("REST Postgres logs => %d %s", rec.Code, rec.Body.String())
	}
	labels := map[string]string{}
	for _, label := range env.Logs[0].Labels {
		labels[label.Name] = label.Value
	}
	if labels["resource"] != postgresID || labels[LabelType] != "postgres" || labels[LabelInstance] != pod {
		t.Fatalf("REST Postgres labels = %+v", labels)
	}
	if strings.Contains(rec.Body.String(), "must not leak") {
		t.Fatal("REST Postgres query mixed another database's pod logs")
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	data := runQuery(t, schema, `{ logs(resource:"`+postgresID+`", text:"checkpoint", instance:["`+pod+`"]) { message type instance } }`)
	rows := data["logs"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["type"] != "postgres" {
		t.Fatalf("GraphQL Postgres logs = %+v", rows)
	}

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
	t.Cleanup(func() { _ = cs.Close() })
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{postgresID}, "text": []string{"checkpoint"}, "instance": []string{pod},
	}})
	if err != nil || result.IsError || !strings.Contains(result.Content[0].(*mcp.TextContent).Text, "checkpoint complete") {
		t.Fatalf("MCP Postgres logs = %+v, err=%v", result, err)
	}
}

func TestManagedPostgresLogsRejectAnotherWorkspaceOnEveryAdapter(t *testing.T) {
	database := sampleDatabase(postgresID)
	database.Labels = map[string]string{core.LabelTenant: "tea-other", core.LabelWorkspace: "tea-other"}
	svc := newService(nil, database)
	svc.Base.Workspace = homeWorkspaceOnly{}
	called := false
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) {
		called = true
		return nil, nil
	}

	requestContext := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "session"})
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs?resource="+postgresID, nil).WithContext(requestContext)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("REST cross-workspace Postgres logs => %d, want 403", rec.Code)
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: `{ logs(resource:"` + postgresID + `") { message } }`, Context: requestContext})
	if len(result.Errors) == 0 || !strings.Contains(strings.ToLower(result.Errors[0].Message), "forbidden") {
		t.Fatalf("GraphQL cross-workspace Postgres logs = %+v", result.Errors)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	// The in-memory MCP transport does not carry auth middleware context, so pin
	// the same fail-closed adapter property with a relation denial here. The
	// membership denial above already proves the cross-workspace resource gate.
	svc.Base.Authz = denyLogChecker{}
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
	t.Cleanup(func() { _ = cs.Close() })
	mcpResult, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{"resource": []string{postgresID}}})
	if err != nil || !mcpResult.IsError {
		t.Fatalf("MCP cross-workspace Postgres logs = %+v, err=%v", mcpResult, err)
	}
	if called {
		t.Fatal("cross-workspace denial happened after the log source was queried")
	}
}

func TestRESTLogsResolveTypedIDToTenantPrefixedApp(t *testing.T) {
	app := sampleApp(core.CRName("tea-a", "web"))
	app.Labels = map[string]string{
		core.LabelTenant:      "tea-a",
		core.LabelServiceName: "web",
		core.LabelAppID:       "srv-c185th5c2rvvnhbfiltg",
	}
	physicalName := app.Name
	svc := newService(map[string][]string{
		"web-1": {"2026-07-05T00:00:01Z booted"},
	}, app, podFor(physicalName, "web-1"))

	rec := serveREST(svc, "GET", "/v1/logs?resource=srv-c185th5c2rvvnhbfiltg")
	var env renderLogList
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("typed-id logs => %d: %v %s", rec.Code, err, rec.Body.String())
	}
	if len(env.Logs) != 1 || env.Logs[0].Message != "booted" {
		t.Fatalf("typed-id logs = %+v, want the tenant-prefixed App's line", env.Logs)
	}
	labels := map[string]string{}
	for _, label := range env.Logs[0].Labels {
		labels[label.Name] = label.Value
	}
	if labels["resource"] != "srv-c185th5c2rvvnhbfiltg" {
		t.Fatalf("resource label = %q, want public typed id", labels["resource"])
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

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?resource=web", nil)
	req.Header.Set("Accept", "text/event-stream")
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("subscribe => 200 SSE, got %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "data: ") || !strings.Contains(body, "live one") || !strings.Contains(body, "live two") {
		t.Errorf("SSE body missing streamed lines: %q", body)
	}
}

func TestFollowBuildLogsStreamsNewestActiveBuildPod(t *testing.T) {
	old := buildPodFor("web", "bld-web-gen-1-old", "builds", "buildkit", time.Unix(1, 0))
	newest := buildPodFor("web", "bld-web-gen-2-new", "builds", "buildkit", time.Unix(2, 0))
	svc := newService(map[string][]string{
		old.Name:    {"2026-07-05T00:00:00Z stale build"},
		newest.Name: {"2026-07-05T00:00:01Z live build one", "2026-07-05T00:00:02Z live build two"},
	}, sampleApp("web"), old, newest)
	svc.BuildNamespace = "builds"

	var entries []LogEntry
	err := svc.FollowLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}}, func(entry LogEntry) error {
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		t.Fatalf("FollowLogs(type=build): %v", err)
	}
	if len(entries) != 2 || entries[0].Message != "live build one" || entries[1].Message != "live build two" {
		t.Fatalf("build entries = %+v, want newest build's two lines", entries)
	}
	for _, entry := range entries {
		if entry.Labels[LabelType] != LogTypeBuild || entry.Labels["instance"] != newest.Name || entry.Labels["container"] != "buildkit" {
			t.Errorf("build labels = %+v", entry.Labels)
		}
	}
}

// A Pending build pod (scheduling, image pull — minutes on a cold node) is a
// build in flight: the tail must wait for it to start, not answer the terminal
// "no running build" event to a subscriber who connected the moment the deploy
// opened (the prod symptom behind w9: a deploy page that never showed a single
// build line because its one subscription died before buildkit's image arrived).
func TestFollowBuildLogsWaitsForPendingPodToStart(t *testing.T) {
	pending := buildPodFor("web", "bld-web-gen-3-slow", "builds", "buildkit", time.Unix(3, 0))
	pending.Status = corev1.PodStatus{Phase: corev1.PodPending}
	svc := newService(map[string][]string{
		pending.Name: {"2026-07-05T00:00:01Z build line one", "2026-07-05T00:00:02Z build line two"},
	}, sampleApp("web"), pending)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 5 * time.Millisecond

	// Flip the pod to Running (container up) after the tail has begun waiting.
	go func() {
		time.Sleep(25 * time.Millisecond)
		started := buildPodFor("web", pending.Name, "builds", "buildkit", time.Unix(3, 0))
		started.ResourceVersion = pending.ResourceVersion
		if err := svc.Client.Status().Update(context.Background(), started); err != nil {
			t.Errorf("flip pod to Running: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var entries []LogEntry
	err := svc.FollowLogs(ctx, LogQuery{App: "web", Types: []string{LogTypeBuild}}, func(entry LogEntry) error {
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		t.Fatalf("FollowLogs(type=build) while pod Pending->Running: %v", err)
	}
	if len(entries) != 2 || entries[0].Message != "build line one" || entries[1].Message != "build line two" {
		t.Fatalf("build entries = %+v, want the started pod's two lines", entries)
	}
}

func TestFollowBuildLogsNoActivePodIsNamedTerminalOutcome(t *testing.T) {
	completed := buildPodFor("web", "bld-web-gen-1-done", "builds", "buildkit", time.Unix(1, 0))
	completed.Status.Phase = corev1.PodSucceeded
	svc := newService(nil, sampleApp("web"), completed)
	svc.BuildNamespace = "builds"

	err := svc.FollowLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}}, func(LogEntry) error { return nil })
	if !errors.Is(err, ErrBuildNotRunning) {
		t.Fatalf("no active build => %v, want ErrBuildNotRunning", err)
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/logs/subscribe?resource=web&type=build", nil)
	req.Header.Set("Accept", "text/event-stream")
	mux.ServeHTTP(rec, req)
	if body := rec.Body.String(); !strings.Contains(body, "event: error") || !strings.Contains(body, ErrBuildNotRunning.Error()) {
		t.Fatalf("terminal SSE body = %q", body)
	}
}

// --- Platform progress lines (w1/m48) ---

// inFlightDeploy is a repo-backed deploy row mid-build: queued + building
// lines earned, no terminal line yet.
func inFlightDeploy() DeployProgress {
	return DeployProgress{
		ID:        "dep-progress-1",
		Status:    "build_in_progress",
		Commit:    "abc1234def5678",
		CreatedAt: time.Date(2026, 7, 17, 20, 16, 14, 0, time.UTC),
		StartedAt: time.Date(2026, 7, 17, 20, 16, 16, 0, time.UTC),
	}
}

func TestQueryLogsSynthesizesProgressLinesForExplicitBuildType(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	svc := newService(nil, app)
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		if !slices.Contains(q.Types, LogTypeBuild) {
			return []LogEntry{{Timestamp: "2026-07-17T20:17:00Z", Message: "app noise", Labels: map[string]string{LabelType: LogTypeApp}}}, nil
		}
		return []LogEntry{{Timestamp: "2026-07-17T20:18:20Z", Message: "#1 real build line", Labels: map[string]string{LabelType: LogTypeBuild, LabelInstance: "bld-pod"}}}, nil
	}
	failed := inFlightDeploy()
	failed.Status = "build_failed"
	failed.FinishedAt = time.Date(2026, 7, 17, 20, 18, 46, 0, time.UTC)
	svc.DeployProgress = func(_ context.Context, resource string, _ time.Time) ([]DeployProgress, error) {
		if resource != "web" {
			t.Errorf("progress source keyed by %q, want the public resource id", resource)
		}
		return []DeployProgress{failed}, nil
	}

	q := LogQuery{App: "web", Types: []string{LogTypeBuild},
		Since: time.Date(2026, 7, 17, 20, 16, 14, 0, time.UTC),
		End:   time.Date(2026, 7, 17, 20, 18, 46, 0, time.UTC)}
	entries, err := svc.QueryLogs(context.Background(), q)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	want := []string{
		"==> Build queued",
		"==> Building from https://github.com/x/y.git@abc1234",
		"#1 real build line",
		"==> Build failed",
	}
	if len(entries) != len(want) {
		t.Fatalf("entries = %+v, want %d lines", entries, len(want))
	}
	for i, w := range want {
		if entries[i].Message != w {
			t.Errorf("entry %d = %q, want %q (chronological interleave)", i, entries[i].Message, w)
		}
	}
	for _, e := range entries {
		if e.Message == "#1 real build line" {
			continue
		}
		if e.Labels[LabelType] != LogTypeBuild || e.Labels[LabelInstance] != "dep-progress-1" || e.Labels["container"] != progressContainer {
			t.Errorf("synthesized labels = %+v", e.Labels)
		}
	}

	// Determinism: a second identical read yields identical entries and ids.
	again, err := svc.QueryLogs(context.Background(), q)
	if err != nil {
		t.Fatalf("QueryLogs (second read): %v", err)
	}
	for i := range entries {
		if logID(entries[i]) != logID(again[i]) {
			t.Errorf("entry %d id drifted across reads: %q vs %q", i, logID(entries[i]), logID(again[i]))
		}
	}
}

// A query that does not explicitly ask for build logs gets no narration — the
// same condition under which the store selector excludes build streams.
func TestQueryLogsNoProgressLinesWithoutExplicitBuildType(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) {
		return []LogEntry{{Timestamp: "2026-07-17T20:17:00Z", Message: "app noise", Labels: map[string]string{LabelType: LogTypeApp}}}, nil
	}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{inFlightDeploy()}, nil
	}
	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web"})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Message, "==>") {
			t.Errorf("untyped query synthesized %q — narration must be explicit-build only", e.Message)
		}
	}
}

// Without a durable store, type=build history stays the named 503 — platform
// lines never masquerade as a successful (empty) build history.
func TestQueryLogsBuildTypeStillRefusedStorelessDespiteProgressSource(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{inFlightDeploy()}, nil
	}
	_, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}})
	if !errors.Is(err, core.ErrLogStoreUnavailable) {
		t.Fatalf("storeless type=build => ErrLogStoreUnavailable, got %v", err)
	}
}

// The build tail narrates immediately: queued/building lines arrive while the
// pod is still Pending (image pull), before any real stdout exists — the
// incident this milestone closes (a subscriber staring at silence for the
// build's whole cold start).
func TestFollowBuildLogsNarratesWhileAwaitingPendingPod(t *testing.T) {
	pending := buildPodFor("web", "bld-web-gen-9-slow", "builds", "buildkit", time.Unix(9, 0))
	pending.Status = corev1.PodStatus{Phase: corev1.PodPending}
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	svc := newService(map[string][]string{
		pending.Name: {"2026-07-17T20:18:20Z real build line"},
	}, app, pending)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 5 * time.Millisecond
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{inFlightDeploy()}, nil
	}

	go func() {
		time.Sleep(25 * time.Millisecond)
		started := buildPodFor("web", pending.Name, "builds", "buildkit", time.Unix(9, 0))
		started.ResourceVersion = pending.ResourceVersion
		if err := svc.Client.Status().Update(context.Background(), started); err != nil {
			t.Errorf("flip pod to Running: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var msgs []string
	err := svc.FollowLogs(ctx, LogQuery{App: "web", Types: []string{LogTypeBuild}}, func(e LogEntry) error {
		msgs = append(msgs, e.Message)
		return nil
	})
	if err != nil {
		t.Fatalf("FollowLogs: %v", err)
	}
	want := []string{"==> Build queued", "==> Building from https://github.com/x/y.git@abc1234", "real build line"}
	if !slices.Equal(msgs, want) {
		t.Fatalf("tail = %v, want narration before stdout, each line once: %v", msgs, want)
	}
}

// Subscribing to an already-terminal deploy whose pod output is still
// streamable catches up on every earned line exactly once, including the
// closing one.
func TestFollowBuildLogsEmitsTerminalLineOnce(t *testing.T) {
	running := buildPodFor("web", "bld-web-gen-9-live", "builds", "buildkit", time.Unix(9, 0))
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y.git"
	svc := newService(map[string][]string{
		running.Name: {"2026-07-17T20:18:20Z real build line"},
	}, app, running)
	svc.BuildNamespace = "builds"
	failed := inFlightDeploy()
	failed.Status = "build_failed"
	failed.FinishedAt = time.Date(2026, 7, 17, 20, 18, 46, 0, time.UTC)
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{failed}, nil
	}

	var msgs []string
	err := svc.FollowLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}}, func(e LogEntry) error {
		msgs = append(msgs, e.Message)
		return nil
	})
	if err != nil {
		t.Fatalf("FollowLogs: %v", err)
	}
	count := 0
	for _, m := range msgs {
		if m == "==> Build failed" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("terminal line emitted %d times in %v, want exactly once", count, msgs)
	}
	if !slices.Contains(msgs, "real build line") {
		t.Fatalf("real stdout missing from %v", msgs)
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

// TestLogsSubscribeCapSpeaksRenderErrorDialect pins w9/m39/t001: the
// GET /v1/logs/subscribe SSE-connection-cap 429 answers in the one Render error
// dialect ({error,message,id}), not a bare {"error"}, so a Render client reading
// .message on a rejected live tail sees a real reason. sseConns is pre-filled to
// the cap so the next connection is refused without a live blocking stream.
func TestLogsSubscribeCapSpeaksRenderErrorDialect(t *testing.T) {
	s := &Service{Base: &core.Base{Namespace: "default"}, MaxSSEConns: 1}
	s.sseConns.Store(1) // at the cap already

	rec := httptest.NewRecorder()
	s.logsSubscribe(rec, httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?resource=srv-x", nil))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429; body=%q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%q)", err, rec.Body.String())
	}
	if msg, ok := body["message"].(string); !ok || msg == "" {
		t.Errorf("429 body missing non-empty string `message`: %v", body)
	}
	if body["id"] != "too_many_requests" {
		t.Errorf("429 body id = %v, want too_many_requests", body["id"])
	}
	// The cap must be released on rejection, not leaked.
	if got := s.sseConns.Load(); got != 1 {
		t.Errorf("sseConns leaked: got %d, want 1", got)
	}
}

func TestSubscriptionWorkspaceCapPreservesAnotherTenantsCapacity(t *testing.T) {
	appA := sampleApp("a")
	appA.Labels = map[string]string{core.LabelTenant: "tea-a"}
	appB := sampleApp("b")
	appB.Labels = map[string]string{core.LabelTenant: "tea-b"}
	s := &Service{
		Base:                    &core.Base{Client: fakeClientWith(appA, appB), Namespace: "default"},
		MaxSSEConns:             3,
		MaxSSEConnsPerWorkspace: 2,
		MaxSSEConnsPerSubject:   0,
	}

	r1, err := s.acquireSubscription(context.Background(), LogQuery{App: "a"})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := s.acquireSubscription(context.Background(), LogQuery{App: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.acquireSubscription(context.Background(), LogQuery{App: "a"}); !errors.Is(err, errSubscriptionLimit) {
		t.Fatalf("third tenant A subscription = %v, want capacity error", err)
	}
	rb, err := s.acquireSubscription(context.Background(), LogQuery{App: "b"})
	if err != nil {
		t.Fatalf("tenant B could not use reserved capacity: %v", err)
	}
	rb()
	r2()
	r1()
	if got := s.sseConns.Load(); got != 0 {
		t.Fatalf("global subscriptions leaked: %d", got)
	}
}

// TestLogLabelValuesHostAndPathForMetricsFilters pins the discovery the metrics
// Network card's Host/Path filters reuse (w5/m58): `host` resolves from the App's
// own URLs even with no store wired (so the Host dropdown always populates),
// while `path` is deliberately NOT a discoverable label — a high-cardinality line
// field, not a Loki stream label — so the metrics UI uses a free-text Path filter
// rather than a fabricated dropdown, exactly as the Logs tab does.
func TestLogLabelValuesHostAndPathForMetricsFilters(t *testing.T) {
	app := sampleApp("web")
	app.Status.URLs = []string{"https://web.onbex.co", "https://www.example.com"}
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(app).Build()
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}} // no LabelValues source: Loki unwired

	// host resolves from the App's URLs with no store wired.
	hosts, err := svc.LogLabelValues(context.Background(), LabelHost, LogQuery{App: "web"})
	if err != nil {
		t.Fatalf("host discovery: %v", err)
	}
	if !slices.Contains(hosts, "web.onbex.co") || !slices.Contains(hosts, "www.example.com") {
		t.Errorf("host values = %v, want both of the App's hostnames", hosts)
	}

	// path is not discoverable — the verb names it rather than fabricating a dropdown.
	if _, err := svc.LogLabelValues(context.Background(), "path", LogQuery{App: "web"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("path discovery: want ErrBadRequest (not a discoverable label), got %v", err)
	}
}

// TestParseContainerLogLineTruncatesHugeMessage pins round-5 finding 17: a
// near-1MiB log record is truncated at the source so a tenant workload emitting
// huge lines cannot exhaust every downstream (count-capped) log client.
func TestParseContainerLogLineTruncatesHugeMessage(t *testing.T) {
	huge := strings.Repeat("A", maxLogMessageBytes+5000)
	entry := parseContainerLogLine("web", "web-1", core.AppContainer, LogTypeApp, "2026-08-14T00:00:00Z "+huge)
	if len(entry.Message) > maxLogMessageBytes+len(" …[truncated]") {
		t.Fatalf("message not truncated: len=%d", len(entry.Message))
	}
	if !strings.HasSuffix(entry.Message, "[truncated]") {
		t.Errorf("truncated message must carry the marker, got tail %q", entry.Message[max(0, len(entry.Message)-20):])
	}
	// A normal line is untouched.
	short := parseContainerLogLine("web", "web-1", core.AppContainer, LogTypeApp, "2026-08-14T00:00:00Z hello")
	if short.Message != "hello" {
		t.Errorf("short message = %q, want hello", short.Message)
	}
}

// --- Live-tail authorization revalidation (w4/034) ---

// freshGateChecker answers the cached admission Check from one flag and the
// authoritative CheckFresh from another — the stale-positive / fresh-deny shape
// the log-tail watchdog exists to close (an OpenFGA positive cache letting a
// revoked caller's established tail run on).
type freshGateChecker struct {
	mu         sync.Mutex
	cached     bool
	fresh      bool
	freshErr   error
	freshCalls int
}

func (c *freshGateChecker) Check(context.Context, string, string, string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cached, nil
}

func (c *freshGateChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.freshCalls++
	return c.fresh, c.freshErr
}

func (c *freshGateChecker) setFresh(allow bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fresh, c.freshErr = allow, err
}

func (c *freshGateChecker) freshCallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.freshCalls
}

// blockingStream is an io.ReadCloser whose Read blocks until ctx ends, so a
// tail stays established long enough for the revalidation watchdog to tick.
type blockingStream struct{ ctx context.Context }

func (s blockingStream) Read([]byte) (int, error) {
	<-s.ctx.Done()
	return 0, s.ctx.Err()
}

func (s blockingStream) Close() error { return nil }

func blockingLogStream() PodLogStream {
	return func(ctx context.Context, _, _, _ string) (io.ReadCloser, error) {
		return blockingStream{ctx: ctx}, nil
	}
}

// revalidationService builds a tail-capable Service with the watchdog on a
// test-fast cadence and a small global SSE cap so tests can observe slot
// release.
func revalidationService(checker *freshGateChecker, stream PodLogStream, objs ...client.Object) *Service {
	return &Service{
		Base:               &core.Base{Client: fakeClientWith(objs...), Namespace: "default", Authz: checker},
		PodLogsFollow:      stream,
		MaxSSEConns:        5,
		RevalidateInterval: 10 * time.Millisecond,
	}
}

// revalidationTestServer serves svc's REST routes behind the identity an
// auth-gate would have resolved — the Authz checker requires one in ctx.
func revalidationTestServer(svc *Service) *httptest.Server {
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(core.WithIdentity(r.Context(), core.Identity{Subject: "alice", Method: "session"})))
	}))
}

func subscribeSSE(t *testing.T, srv *httptest.Server, resource string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/logs/subscribe?resource="+resource, nil)
	if err != nil {
		t.Fatalf("build subscribe request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	return resp
}

// streamDone closes when the response body ends (the server-side stream closed)
// or errors.
func streamDone(body io.ReadCloser) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(io.Discard, body)
	}()
	return done
}

// streamLines delivers the stream's lines as they arrive and closes at EOF.
func streamLines(body io.ReadCloser) <-chan string {
	lines := make(chan string, 16)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	return lines
}

func waitForFreshCalls(t *testing.T, c *freshGateChecker, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c.freshCallCount() >= n {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("fresh re-checks = %d, want >= %d", c.freshCallCount(), n)
}

// waitForStreamEnd fails unless the stream ends within one generous bound of
// the watchdog interval (10ms cadence here; 5s absorbs slow CI).
func waitForStreamEnd(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("established tail outlived its failed re-check")
	}
}

func waitForSlotRelease(t *testing.T, svc *Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if svc.sseConns.Load() == 0 {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("SSE slot not released: %d", svc.sseConns.Load())
}

func TestLogStreamRevalidationEndsStaleAuthorizedTail(t *testing.T) {
	checker := &freshGateChecker{cached: true, fresh: true}
	svc := revalidationService(checker, blockingLogStream(), sampleApp("web"), podFor("web", "web-1"))
	srv := revalidationTestServer(svc)
	defer srv.Close()

	resp := subscribeSSE(t, srv, "web")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("subscribe status = %d, want 200 (cached admission allow)", resp.StatusCode)
	}
	done := streamDone(resp.Body)

	// The tail is established on the CACHED allow; once the watchdog's first
	// healthy re-check has ticked, revoke: the next FRESH check (uncached, so
	// the positive cache cannot mask it) must end the stream within one
	// interval and release the subscription's cap slot.
	waitForFreshCalls(t, checker, 1)
	checker.setFresh(false, nil)

	waitForStreamEnd(t, done)
	waitForSlotRelease(t, svc)
}

func TestLogStreamRevalidationFailsClosedOnCheckerError(t *testing.T) {
	checker := &freshGateChecker{cached: true, fresh: true}
	svc := revalidationService(checker, blockingLogStream(), sampleApp("web"), podFor("web", "web-1"))
	srv := revalidationTestServer(svc)
	defer srv.Close()

	resp := subscribeSSE(t, srv, "web")
	defer resp.Body.Close()
	done := streamDone(resp.Body)

	// A checker that cannot be reached is a failed check, not an allow: the
	// watchdog fails closed and ends the tail.
	waitForFreshCalls(t, checker, 1)
	checker.setFresh(false, errors.New("openfga unreachable"))

	waitForStreamEnd(t, done)
	waitForSlotRelease(t, svc)
}

func TestLogStreamRevalidationEndsTailWhenAppDeleted(t *testing.T) {
	checker := &freshGateChecker{cached: true, fresh: true}
	app := sampleApp("web")
	svc := revalidationService(checker, blockingLogStream(), app, podFor("web", "web-1"))
	srv := revalidationTestServer(svc)
	defer srv.Close()

	resp := subscribeSSE(t, srv, "web")
	defer resp.Body.Close()
	done := streamDone(resp.Body)

	// The resource disappearing ends the tail too: a stream of a gone App has
	// nothing left to be authorized for.
	waitForFreshCalls(t, checker, 1)
	if err := svc.Client.Delete(context.Background(), app); err != nil {
		t.Fatalf("delete App: %v", err)
	}

	waitForStreamEnd(t, done)
	waitForSlotRelease(t, svc)
}

func TestLogStreamRevalidationPreservesHealthyTail(t *testing.T) {
	checker := &freshGateChecker{cached: true, fresh: true}
	pr, pw := io.Pipe()
	stream := func(ctx context.Context, _, _, _ string) (io.ReadCloser, error) {
		go func() { <-ctx.Done(); _ = pr.CloseWithError(ctx.Err()) }()
		return pr, nil
	}
	svc := revalidationService(checker, stream, sampleApp("web"), podFor("web", "web-1"))
	srv := revalidationTestServer(svc)
	defer srv.Close()

	resp := subscribeSSE(t, srv, "web")
	defer resp.Body.Close()
	lines := streamLines(resp.Body)

	assertLine := func(want string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			select {
			case line, ok := <-lines:
				if !ok {
					t.Fatalf("healthy tail ended while waiting for %q", want)
				}
				if strings.Contains(line, want) {
					return
				}
			case <-deadline:
				t.Fatalf("timed out waiting for %q on a healthy tail", want)
			}
		}
	}

	if _, err := pw.Write([]byte("2026-08-19T00:00:01Z one\n")); err != nil {
		t.Fatalf("write line one: %v", err)
	}
	assertLine("one")

	// Several healthy re-checks tick by; an allowed re-check must not interrupt
	// delivery.
	waitForFreshCalls(t, checker, 3)
	if _, err := pw.Write([]byte("2026-08-19T00:00:02Z two\n")); err != nil {
		t.Fatalf("write line two: %v", err)
	}
	assertLine("two")
}

func TestLogStreamRevalidationEndsStaleAuthorizedWebSocketTail(t *testing.T) {
	checker := &freshGateChecker{cached: true, fresh: true}
	svc := revalidationService(checker, blockingLogStream(), sampleApp("web"), podFor("web", "web-1"))
	srv := revalidationTestServer(svc)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?resource=web"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()

	// Same contract as the SSE path: a fresh deny ends the established tail —
	// the server closes the connection, so the client's next read fails.
	waitForFreshCalls(t, checker, 1)
	checker.setFresh(false, nil)

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("stale-authorized WebSocket tail outlived its revocation")
	}
	waitForSlotRelease(t, svc)
}

func TestLogStreamReconnectStillAuthorizesAtAdmission(t *testing.T) {
	// Revoked everywhere: a NEW subscription is refused at admission (the
	// watchdog only governs established tails; it is not a loophole around the
	// admission gate).
	checker := &freshGateChecker{cached: false, fresh: false}
	opened := make(chan struct{}, 1)
	stream := func(ctx context.Context, _, _, _ string) (io.ReadCloser, error) {
		opened <- struct{}{}
		return blockingStream{ctx: ctx}, nil
	}
	svc := revalidationService(checker, stream, sampleApp("web"), podFor("web", "web-1"))
	srv := revalidationTestServer(svc)
	defer srv.Close()

	resp := subscribeSSE(t, srv, "web")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("reconnect after revocation = %d, want 403 at admission", resp.StatusCode)
	}
	select {
	case <-opened:
		t.Fatal("pod log stream opened despite the admission denial")
	default:
	}
}

func TestLogStreamRevalidationNegativeIntervalDisables(t *testing.T) {
	// A fresh check WOULD deny, but a negative interval restores the
	// admission-only behavior: the watchdog never ticks and the tail lives on.
	checker := &freshGateChecker{cached: true, fresh: false}
	svc := revalidationService(checker, blockingLogStream(), sampleApp("web"), podFor("web", "web-1"))
	svc.RevalidateInterval = -1
	srv := revalidationTestServer(svc)
	defer srv.Close()

	resp := subscribeSSE(t, srv, "web")
	defer resp.Body.Close()
	done := streamDone(resp.Body)

	select {
	case <-done:
		t.Fatal("tail ended with the watchdog disabled")
	case <-time.After(100 * time.Millisecond):
	}
	if got := checker.freshCallCount(); got != 0 {
		t.Errorf("fresh re-checks with the watchdog disabled = %d, want 0", got)
	}
}
