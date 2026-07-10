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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
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

// narrowLogType (shared by the REST + MCP fragments) turns Render's repeatable
// `type` filter into Core's single Type: narrow only on one concrete type,
// tolerate the `application` alias, report ok=false for an unknown value.
func TestNarrowLogType(t *testing.T) {
	cases := []struct {
		in     []string
		want   string
		wantOK bool
	}{
		{nil, "", true},                                     // no filter => all
		{[]string{"all"}, "", true},                         // explicit all
		{[]string{"app"}, LogTypeApplication, true},         // Render's `app`
		{[]string{"application"}, LogTypeApplication, true}, // alias
		{[]string{"request"}, LogTypeRequest, true},
		{[]string{"build"}, LogTypeBuild, true},
		{[]string{"app", "request"}, "", true}, // several => all
		{[]string{"app", "app"}, LogTypeApplication, true},
		{[]string{"bogus"}, "", false}, // unknown => not ok
	}
	for _, c := range cases {
		got, ok := narrowLogType(c.in)
		if got != c.want || ok != c.wantOK {
			t.Errorf("narrowLogType(%v) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.wantOK)
		}
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
	if env.Logs[0].ID == "" || env.Logs[0].Labels[0].Name != "type" || env.Logs[0].Labels[0].Value != renderLogTypeApp {
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

	// request/build have no source: 200 + empty.
	for _, ty := range []string{"request", "build"} {
		rec := serveREST(svc, "GET", "/v1/logs?resource=web&type="+ty)
		var env renderLogList
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &env) != nil || len(env.Logs) != 0 {
			t.Errorf("type=%s => 200 empty, got %d %+v", ty, rec.Code, env.Logs)
		}
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
	if first["message"] != "hello" || first["type"] != renderLogTypeApp || first["instance"] != "web-1" {
		t.Fatalf("unexpected log shape: %+v", first)
	}
}
