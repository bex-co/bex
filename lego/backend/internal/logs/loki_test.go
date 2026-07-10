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
			name: "label selector only",
			ns:   "default",
			q:    LogQuery{App: "web"},
			want: `{namespace="default", app="web", container="app"}`,
		},
		{
			name: "text search adds a case-insensitive line filter",
			ns:   "default",
			q:    LogQuery{App: "web", Search: "ERROR boot"},
			want: `{namespace="default", app="web", container="app"} |~ "(?i)ERROR boot"`,
		},
		{
			// A service name carrying LogQL/regex metacharacters must not break out
			// of the label matcher or inject a selector — label injection guard.
			name: "label injection is escaped",
			ns:   "default",
			q:    LogQuery{App: `web"} |= "secret`},
			want: `{namespace="default", app="web\"} |= \"secret", container="app"}`,
		},
		{
			// A regex metacharacter in the search is quoted to a literal, so `.*`
			// matches the literal text, not "anything".
			name: "search regex metachars are quoted to literals",
			ns:   "default",
			q:    LogQuery{App: "web", Search: ".*"},
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

// fakeLoki captures the last query_range request and replies with body.
type fakeLoki struct {
	srv        *httptest.Server
	lastValues url.Values
	status     int
	body       string
}

func newFakeLoki(body string) *fakeLoki {
	f := &fakeLoki{status: http.StatusOK, body: body}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.lastValues = r.URL.Query()
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
	if q := got.Get("query"); q != `{namespace="default", app="web", container="app"} |~ "(?i)hel"` {
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
