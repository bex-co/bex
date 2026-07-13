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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// --- LogQL builder (pure) ---

func TestLokiQueryFor(t *testing.T) {
	cases := []struct {
		name string
		ns   string
		q    LogQuery
		want string
	}{
		{
			// No type filter => every type: the App's container streams (container
			// "app") UNION the request streams (no container label at all).
			name: "no type filter unions app + request streams",
			ns:   "default",
			q:    LogQuery{App: "web"},
			want: `{namespace="default", app="web", container=~"app|"}`,
		},
		{
			name: "type=app selects the App container's streams",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeApp}},
			want: `{namespace="default", app="web", container="app"}`,
		},
		{
			name: "type=request selects the access-log streams",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeRequest}},
			want: `{namespace="default", app="web", type="request"}`,
		},
		{
			name: "text search adds a case-insensitive line filter",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeApp}, Search: "ERROR boot"},
			want: `{namespace="default", app="web", container="app"} |~ "(?i)ERROR boot"`,
		},
		{
			// Bounded fields are label matchers — the cheap, indexed half.
			name: "level / instance are label matchers on app streams",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeApp}, Level: []string{"error"}, Instance: []string{"web-1"}},
			want: `{namespace="default", app="web", container="app", level="error", pod="web-1"}`,
		},
		{
			// statusCode is the store's `status` label; a class shorthand becomes an
			// anchored regex, so 4xx matches 404 and not 200.
			name: "statusCode class + method are label matchers on request streams",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeRequest}, StatusCode: []string{"4xx"}, Method: []string{"GET"}},
			want: `{namespace="default", app="web", type="request", method="GET", status=~"^(4.*)$"}`,
		},
		{
			// Several values for one filter is an OR (Render's array semantics).
			name: "multiple values for one filter OR together",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeApp}, Level: []string{"error", "warn"}},
			want: `{namespace="default", app="web", container="app", level=~"^(error|warn)$"}`,
		},
		{
			// path/host are NEVER labels (unbounded — the cardinality budget): they
			// are parsed out of the JSON access line at query time.
			name: "path and host are line filters over the access line, not labels",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeRequest}, Path: []string{"/healthz"}, Host: []string{"web.onbex.co"}},
			want: `{namespace="default", app="web", type="request"} | json request_path="RequestPath", request_host="RequestHost" | request_path="/healthz" | request_host="web.onbex.co"`,
		},
		{
			name: "a wildcard value becomes an anchored regex, other metachars stay literal",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeRequest}, Path: []string{"/api/*"}},
			want: `{namespace="default", app="web", type="request"} | json request_path="RequestPath", request_host="RequestHost" | request_path=~"^(/api/.*)$"`,
		},
		{
			// A service name carrying LogQL/regex metacharacters must not break out
			// of the label matcher or inject a selector — label injection guard.
			name: "label injection is escaped",
			ns:   "default",
			q:    LogQuery{App: `web"} |= "secret`, Types: []string{LogTypeApp}},
			want: `{namespace="default", app="web\"} |= \"secret", container="app"}`,
		},
		{
			// The same guard on a caller-supplied FILTER value: a quote can't close
			// the matcher, and a regex metacharacter stays a literal.
			name: "filter-value injection is escaped",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeApp}, Level: []string{`error"} |= "secret`}},
			want: `{namespace="default", app="web", container="app", level="error\"} |= \"secret"}`,
		},
		{
			name: "filter-value regex metachars are quoted in the regex branch",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeRequest}, Method: []string{"GET", "PO.T"}},
			want: `{namespace="default", app="web", type="request", method=~"^(GET|PO\\.T)$"}`,
		},
		{
			// A regex metacharacter in the search is quoted to a literal, so `.*`
			// matches the literal text, not "anything".
			name: "search regex metachars are quoted to literals",
			ns:   "default",
			q:    LogQuery{App: "web", Types: []string{LogTypeApp}, Search: ".*"},
			want: `{namespace="default", app="web", container="app"} |~ "(?i)\\.\\*"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lokiQueryFor(c.ns, c.q); got != c.want {
				t.Errorf("lokiQueryFor:\n got %s\nwant %s", got, c.want)
			}
		})
	}
}

// --- Range + limit resolution (pure) ---

func TestLokiRange(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	since := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC)

	// No bounds => [now-lookback, now].
	s, e := lokiRange(LogQuery{}, now)
	if !e.Equal(now) || !s.Equal(now.Add(-lokiLookback)) {
		t.Errorf("empty query: got [%s, %s], want [%s, %s]", s, e, now.Add(-lokiLookback), now)
	}
	// Explicit bounds win over the defaults; now is ignored.
	s, e = lokiRange(LogQuery{Since: since, End: end}, now)
	if !s.Equal(since) || !e.Equal(end) {
		t.Errorf("explicit bounds: got [%s, %s], want [%s, %s]", s, e, since, end)
	}
	// Only End set => start defaults off End, not now.
	s, _ = lokiRange(LogQuery{End: end}, now)
	if !s.Equal(end.Add(-lokiLookback)) {
		t.Errorf("end-only start: got %s, want %s", s, end.Add(-lokiLookback))
	}
}

func TestLokiLimit(t *testing.T) {
	if got := lokiLimit(LogQuery{Limit: 0}); got != defaultLogTail {
		t.Errorf("zero limit => %d, want default %d", got, defaultLogTail)
	}
	if got := lokiLimit(LogQuery{Limit: -5}); got != defaultLogTail {
		t.Errorf("negative limit => %d, want default %d", got, defaultLogTail)
	}
	if got := lokiLimit(LogQuery{Limit: 20}); got != 20 {
		t.Errorf("explicit limit => %d, want 20", got)
	}
}

// --- Stream parser (pure) ---

// lokiResp builds a query_range JSON body from stream label sets + [ns,line] pairs.
func lokiResp(streams ...map[string]any) string {
	results := make([]string, 0, len(streams))
	for _, st := range streams {
		labels := st["stream"].(string)
		values := st["values"].(string)
		results = append(results, fmt.Sprintf(`{"stream":%s,"values":%s}`, labels, values))
	}
	return fmt.Sprintf(`{"status":"success","data":{"resultType":"streams","result":[%s]}}`,
		strings.Join(results, ","))
}

func TestParseLokiStreams(t *testing.T) {
	// Two streams (two pods); parser interleaves oldest-first across them.
	body := lokiResp(
		map[string]any{
			"stream": `{"app":"web","pod":"web-1","container":"app"}`,
			"values": `[["1751673603000000000","later from 1"],["1751673601000000000","hello from 1"]]`,
		},
		map[string]any{
			"stream": `{"app":"web","pod":"web-2","container":"app"}`,
			"values": `[["1751673602000000000","hello from 2"]]`,
		},
	)
	var lr lokiRangeResponse
	if err := decodeJSON(body, &lr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	entries, err := parseLokiStreams(lr, LogQuery{App: "web", Limit: 100})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"hello from 1", "hello from 2", "later from 1"}
	if len(entries) != len(want) {
		t.Fatalf("want %d entries, got %d: %+v", len(want), len(entries), entries)
	}
	for i, w := range want {
		if entries[i].Message != w {
			t.Errorf("entry %d = %q, want %q", i, entries[i].Message, w)
		}
	}
	// Labels land in the pod-log path's shape so toRenderLog/logID are identical.
	if entries[0].Labels["service"] != "web" || entries[0].Labels["instance"] != "web-1" || entries[0].Labels["container"] != "app" {
		t.Errorf("render labels wrong: %+v", entries[0].Labels)
	}
	// Timestamps are RFC3339Nano UTC, matching parseLogLine.
	if _, err := time.Parse(time.RFC3339Nano, entries[0].Timestamp); err != nil {
		t.Errorf("timestamp not RFC3339Nano: %q", entries[0].Timestamp)
	}
}

func TestParseLokiStreamsCapsToLimit(t *testing.T) {
	body := lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-1","container":"app"}`,
		"values": `[["1751673603000000000","c"],["1751673602000000000","b"],["1751673601000000000","a"]]`,
	})
	var lr lokiRangeResponse
	_ = decodeJSON(body, &lr)
	entries, err := parseLokiStreams(lr, LogQuery{App: "web", Limit: 2})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Newest 2 kept, oldest-first: b, c.
	if len(entries) != 2 || entries[0].Message != "b" || entries[1].Message != "c" {
		t.Fatalf("want newest 2 [b c], got %+v", entries)
	}
}

func TestParseLokiStreamsDropsBadAndErrors(t *testing.T) {
	// A non-numeric timestamp drops just that line.
	body := lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-1","container":"app"}`,
		"values": `[["not-a-number","junk"],["1751673601000000000","good"]]`,
	})
	var lr lokiRangeResponse
	_ = decodeJSON(body, &lr)
	entries, err := parseLokiStreams(lr, LogQuery{App: "web", Limit: 100})
	if err != nil || len(entries) != 1 || entries[0].Message != "good" {
		t.Fatalf("bad timestamp should drop one line: %+v (err %v)", entries, err)
	}
	// A non-success status is surfaced honestly, not swallowed.
	if _, err := parseLokiStreams(lokiRangeResponse{Status: "error"}, LogQuery{}); err == nil {
		t.Error("status=error should surface an error")
	}
}

// --- End-to-end over a fake Loki HTTP server ---

// fakeLoki captures the last request (path + query) and replies with body.
type fakeLoki struct {
	srv        *httptest.Server
	lastValues url.Values
	lastPath   string
	status     int
	body       string
}

func newFakeLoki(body string) *fakeLoki {
	f := &fakeLoki{status: http.StatusOK, body: body}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.lastValues = r.URL.Query()
		f.lastPath = r.URL.Path
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.body))
	}))
	return f
}

func TestLokiSourceRequestAndDecode(t *testing.T) {
	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-1","container":"app"}`,
		"values": `[["1751673601000000000","hello"]]`,
	}))
	defer f.srv.Close()

	src := NewLokiSource(f.srv.URL, f.srv.Client())
	since := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC)
	entries, err := src(context.Background(), "default", LogQuery{App: "web", Search: "hel", Since: since, End: end, Limit: 50})
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if len(entries) != 1 || entries[0].Message != "hello" {
		t.Fatalf("want one decoded entry: %+v", entries)
	}

	// The request pushed the filters/bounds/limit to Loki (real time range).
	got := f.lastValues
	if q := got.Get("query"); q != `{namespace="default", app="web", container=~"app|"} |~ "(?i)hel"` {
		t.Errorf("query pushed to loki wrong: %s", q)
	}
	if got.Get("direction") != "backward" {
		t.Errorf("direction = %q, want backward", got.Get("direction"))
	}
	if got.Get("limit") != "50" {
		t.Errorf("limit = %q, want 50", got.Get("limit"))
	}
	if s, _ := strconv.ParseInt(got.Get("start"), 10, 64); s != since.UnixNano() {
		t.Errorf("start = %q, want %d", got.Get("start"), since.UnixNano())
	}
	if e, _ := strconv.ParseInt(got.Get("end"), 10, 64); e != end.UnixNano() {
		t.Errorf("end = %q, want %d", got.Get("end"), end.UnixNano())
	}
}

func TestLokiSourceDownSurfacesError(t *testing.T) {
	// Loki returning 5xx is surfaced (no silent empty page), like Prometheus.
	f := newFakeLoki("boom")
	f.status = http.StatusServiceUnavailable
	defer f.srv.Close()
	src := NewLokiSource(f.srv.URL, f.srv.Client())
	if _, err := src(context.Background(), "default", LogQuery{App: "web", Limit: 20}); err == nil {
		t.Error("loki 503 should surface an error")
	}

	// Unreachable endpoint also errors rather than hanging or faking data.
	src2 := NewLokiSource("http://127.0.0.1:0", &http.Client{Timeout: time.Second})
	if _, err := src2(context.Background(), "default", LogQuery{App: "web", Limit: 20}); err == nil {
		t.Error("unreachable loki should surface an error")
	}
}

// --- Service routing: History set => QueryLogs reads Loki, not pods ---

func TestQueryLogsRoutesToHistory(t *testing.T) {
	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-9","container":"app"}`,
		"values": `[["1751673601000000000","from loki history"]]`,
	}))
	defer f.srv.Close()

	// PodLogs is deliberately a different line: proving the read came from Loki.
	svc := newService(map[string][]string{webInst: {"2026-07-05T00:00:01Z from live pod"}},
		sampleApp("web"), podFor("web", webInst))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())

	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web"})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(entries) != 1 || entries[0].Message != "from loki history" {
		t.Fatalf("QueryLogs should read Loki history, got %+v", entries)
	}
	// Limit still clamps to Render's default (20) via normalized() before the call.
	if got := f.lastValues.Get("limit"); got != strconv.Itoa(defaultLogLimit) {
		t.Errorf("limit not clamped to default: %q", got)
	}
}

func TestLogsAvailableWithHistoryOnly(t *testing.T) {
	// History wired but PodLogs nil: the read verbs are available (not 503) and
	// serve from Loki — a Loki-only deployment is valid.
	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-1","container":"app"}`,
		"values": `[["1751673601000000000","hi"]]`,
	}))
	defer f.srv.Close()
	cl := fakeClientWith(sampleApp("web"), podFor("web", "web-1"))
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, History: NewLokiSource(f.srv.URL, f.srv.Client())}

	if _, err := svc.QueryLogs(context.Background(), LogQuery{App: "web"}); err != nil {
		t.Errorf("QueryLogs with History-only should succeed, got %v", err)
	}
	if _, err := svc.Logs(context.Background(), "web", 0); err != nil {
		t.Errorf("Logs with History-only should succeed, got %v", err)
	}
}

func TestLogsUnavailableWithNeitherSource(t *testing.T) {
	cl := fakeClientWith(sampleApp("web"))
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}} // no History, no PodLogs
	if _, err := svc.QueryLogs(context.Background(), LogQuery{App: "web"}); !errors.Is(err, core.ErrLogsUnavailable) {
		t.Errorf("neither source => ErrLogsUnavailable, got %v", err)
	}
}

// --- Cardinality guard ---

// `path` (and `host`) are unbounded per request: promoting either to a Loki label
// would multiply streams per App without bound — a cardinality incident, and the
// exact regression this test exists to catch. They must appear only after the
// selector's closing brace, as line filters.
func TestPathAndHostNeverBecomeLabels(t *testing.T) {
	q := LogQuery{
		App:   "web",
		Types: []string{LogTypeRequest},
		Path:  []string{"/a/very/specific/path"},
		Host:  []string{"web.onbex.co"},
	}
	selector := lokiSelectorFor("default", q)
	for _, unbounded := range []string{"/a/very/specific/path", "web.onbex.co", "request_path", "request_host"} {
		if strings.Contains(selector, unbounded) {
			t.Errorf("%q must not appear in the stream selector (cardinality): %s", unbounded, selector)
		}
	}
	// …but the full query does filter on them, after the selector.
	full := lokiQueryFor("default", q)
	body := strings.TrimPrefix(full, selector)
	if !strings.Contains(body, "/a/very/specific/path") || !strings.Contains(body, "web.onbex.co") {
		t.Errorf("path/host must still be honored as line filters: %s", full)
	}
	// The discovery API is selector-only, so it can never carry them either.
	if strings.Contains(lokiSelectorFor("default", q), "|") {
		t.Errorf("the label-values selector must carry no line pipeline: %s", selector)
	}
}

// The `Nxx` class shorthand belongs to statusCode ALONE. It used to be applied to
// every filter's values, which silently turned a path (or host, or method) that
// merely looked like a status class into a wildcard — `path=/4xx` matching `/4any`.
func TestStatusClassShorthandIsStatusCodeOnly(t *testing.T) {
	q := LogQuery{App: "web", Types: []string{LogTypeRequest}, StatusCode: []string{"4xx"}, Path: []string{"4xx"}}
	got := lokiQueryFor("default", q)
	if !strings.Contains(got, `status=~"^(4.*)$"`) {
		t.Errorf("statusCode=4xx should expand to the class wildcard: %s", got)
	}
	if !strings.Contains(got, `request_path="4xx"`) {
		t.Errorf("a path of \"4xx\" is a literal path, not a status class: %s", got)
	}
}

// --- Direction: which end of the window `limit` keeps ---

func TestDirectionSelectsWhichLinesSurvive(t *testing.T) {
	body := lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-1","container":"app"}`,
		"values": `[["1751673603000000000","c"],["1751673602000000000","b"],["1751673601000000000","a"]]`,
	})
	var lr lokiRangeResponse
	_ = decodeJSON(body, &lr)

	// forward => the OLDEST 2 of the window; backward (default) => the newest 2.
	fwd, err := parseLokiStreams(lr, LogQuery{App: "web", Limit: 2, Direction: DirectionForward})
	if err != nil || len(fwd) != 2 || fwd[0].Message != "a" || fwd[1].Message != "b" {
		t.Fatalf("forward should keep the oldest 2 [a b], got %+v (err %v)", fwd, err)
	}
	back, err := parseLokiStreams(lr, LogQuery{App: "web", Limit: 2, Direction: DirectionBackward})
	if err != nil || len(back) != 2 || back[0].Message != "b" || back[1].Message != "c" {
		t.Fatalf("backward should keep the newest 2 [b c], got %+v (err %v)", back, err)
	}
	// Either way the slice stays oldest-first — direction picks lines, not order.
	if fwd[0].Timestamp > fwd[1].Timestamp || back[0].Timestamp > back[1].Timestamp {
		t.Error("entries must stay oldest-first regardless of direction")
	}
}

// --- Request-line labels: the split is truthful on the wire ---

func TestRequestStreamLabelsRenderTruthfully(t *testing.T) {
	body := lokiResp(map[string]any{
		"stream": `{"app":"web","type":"request","method":"GET","status":"200"}`,
		"values": `[["1751673601000000000","{\"RequestPath\":\"/\",\"DownstreamStatus\":200}"]]`,
	})
	var lr lokiRangeResponse
	_ = decodeJSON(body, &lr)
	entries, err := parseLokiStreams(lr, LogQuery{App: "web", Limit: 10})
	if err != nil || len(entries) != 1 {
		t.Fatalf("parse: %v %+v", err, entries)
	}
	got := entries[0].Labels
	if got[LabelType] != LogTypeRequest || got[LabelMethod] != "GET" || got[LabelStatusCode] != "200" {
		t.Errorf("request line must carry type/method/statusCode: %+v", got)
	}
	// A request line came from the edge, not a replica: no instance/container is
	// invented for it.
	if _, ok := got["instance"]; ok {
		t.Errorf("request line must not claim an instance: %+v", got)
	}
	if _, ok := got["container"]; ok {
		t.Errorf("request line must not claim a container: %+v", got)
	}
	// The REST projection reports the truthful type, not a hardcoded "app".
	if rl := toRenderLog(entries[0]); rl.Labels[0].Name != LabelType || rl.Labels[0].Value != LogTypeRequest {
		t.Errorf("REST log line type label wrong: %+v", rl.Labels)
	}
}

// --- Label-value discovery ---

func TestLogLabelValuesScopedToTheService(t *testing.T) {
	f := newFakeLoki(`{"status":"success","data":["error","info","unknown"]}`)
	defer f.srv.Close()

	svc := &Service{
		Base:        &core.Base{Client: fakeClientWith(sampleApp("web"), sampleApp("other")), Namespace: "default"},
		History:     NewLokiSource(f.srv.URL, f.srv.Client()),
		LabelValues: NewLokiLabelValuesSource(f.srv.URL, f.srv.Client()),
	}

	values, err := svc.LogLabelValues(context.Background(), LabelLevel, LogQuery{App: "web"})
	if err != nil {
		t.Fatalf("LogLabelValues: %v", err)
	}
	if !slices.Equal(values, []string{"error", "info", "unknown"}) {
		t.Errorf("values = %v", values)
	}
	// TENANCY: the label-values call is scoped to the requested App's streams, so
	// service B's pods/levels can never surface in service A's discovery. An
	// unscoped query here would enumerate the whole store.
	q := f.lastValues.Get("query")
	if !strings.Contains(q, `app="web"`) || !strings.Contains(q, `namespace="default"`) {
		t.Errorf("discovery must be scoped to the App's streams, got %q", q)
	}
	if strings.Contains(q, "other") {
		t.Errorf("another service must never appear in the selector: %q", q)
	}

	// Render's label names map onto the store's: statusCode -> status, instance -> pod.
	if _, err := svc.LogLabelValues(context.Background(), LabelStatusCode, LogQuery{App: "web"}); err != nil {
		t.Fatalf("statusCode discovery: %v", err)
	}
	if path := f.lastPath; !strings.Contains(path, "/label/status/values") {
		t.Errorf("statusCode must discover the store's `status` label, got %s", path)
	}
	if _, err := svc.LogLabelValues(context.Background(), LabelInstance, LogQuery{App: "web"}); err != nil {
		t.Fatalf("instance discovery: %v", err)
	}
	if path := f.lastPath; !strings.Contains(path, "/label/pod/values") {
		t.Errorf("instance must discover the store's `pod` label, got %s", path)
	}

	// An unknown label is a bad request naming it — not an empty list, which would
	// read as "this service has no such values".
	if _, err := svc.LogLabelValues(context.Background(), "bogus", LogQuery{App: "web"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown label => ErrBadRequest, got %v", err)
	}
}

func TestLogLabelValuesHostComesFromTheApp(t *testing.T) {
	app := sampleApp("web")
	app.Status.URLs = []string{"https://web.onbex.co/"}
	// No LabelValues source wired (no store): `host` still resolves — it is not a
	// stream label, it comes from the App itself — while a store-backed label is
	// refused honestly rather than answered with an empty list.
	svc := &Service{Base: &core.Base{Client: fakeClientWith(app), Namespace: "default"}}

	hosts, err := svc.LogLabelValues(context.Background(), LabelHost, LogQuery{App: "web"})
	if err != nil || !slices.Equal(hosts, []string{"web.onbex.co"}) {
		t.Errorf("host values = %v (err %v)", hosts, err)
	}
	if _, err := svc.LogLabelValues(context.Background(), LabelLevel, LogQuery{App: "web"}); !errors.Is(err, core.ErrLogStoreUnavailable) {
		t.Errorf("level discovery without the store => ErrLogStoreUnavailable, got %v", err)
	}
}

// --- Fallback mode: refuse what the pod-log path cannot honor ---

func TestFallbackRefusesStoreOnlyFilters(t *testing.T) {
	svc := newService(map[string][]string{webInst: {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", webInst))

	for name, q := range map[string]LogQuery{
		"level":      {App: "web", Level: []string{"error"}},
		"statusCode": {App: "web", StatusCode: []string{"500"}},
		"method":     {App: "web", Method: []string{"GET"}},
		"path":       {App: "web", Path: []string{"/x"}},
		"host":       {App: "web", Host: []string{"web.onbex.co"}},
		"request":    {App: "web", Types: []string{LogTypeRequest}},
	} {
		if _, err := svc.QueryLogs(context.Background(), q); !errors.Is(err, core.ErrLogStoreUnavailable) {
			t.Errorf("%s without the store => ErrLogStoreUnavailable, got %v", name, err)
		}
		// The live tail refuses the same set — but as a BAD REQUEST, not "the store
		// is missing": the tail reads pod logs even on a cluster that has Loki, so
		// blaming the store would be a lie there. The refusal is the transport's.
		if err := svc.FollowLogs(context.Background(), q, func(LogEntry) error { return nil }); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s on the tail => ErrBadRequest, got %v", name, err)
		}
	}

	// …and that holds with the store wired, where ErrLogStoreUnavailable would be
	// flatly untrue.
	withStore := newService(map[string][]string{webInst: {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", webInst))
	withStore.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	err := withStore.FollowLogs(context.Background(), LogQuery{App: "web", Level: []string{"error"}}, func(LogEntry) error { return nil })
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("tail + level WITH the store wired => ErrBadRequest (transport limit), got %v", err)
	}

	// A build-only query is empty by design (no backend anywhere), not an error.
	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}})
	if err != nil || len(entries) != 0 {
		t.Errorf("type=build => empty, got %+v (err %v)", entries, err)
	}
}
