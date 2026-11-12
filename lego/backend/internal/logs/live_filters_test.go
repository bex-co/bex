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
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m147: a live tail takes every filter Render's /logs/subscribe takes. The
// request filters select request lines, and `level` selects app lines and
// request lines by their status class (w4/584), as the same labels narrow a
// history query's streams.

func TestLiveSubscribableRoutesFiltersToTheirProducers(t *testing.T) {
	app, request, both := []string{LogTypeApp}, []string{LogTypeRequest}, []string{LogTypeApp, LogTypeRequest}
	for _, tc := range []struct {
		name    string
		q       LogQuery
		want    []string
		refuses string // the filter a 400 must name
	}{
		{name: "no filters keeps every type", q: LogQuery{}},
		{name: "no filters keeps an explicit type", q: LogQuery{Types: app}, want: app},
		{name: "path alone follows request lines", q: LogQuery{Path: []string{"/x"}}, want: request},
		{name: "statusCode alone", q: LogQuery{StatusCode: []string{"500"}}, want: request},
		{name: "method alone", q: LogQuery{Method: []string{"GET"}}, want: request},
		{name: "host alone", q: LogQuery{Host: []string{"web.onbex.co"}}, want: request},
		{name: "request filters with type=request", q: LogQuery{Types: request, Path: []string{"/x"}}, want: request},
		{name: "request filters narrow app+request", q: LogQuery{Types: both, StatusCode: []string{"404"}}, want: request},
		{name: "level alone keeps every type", q: LogQuery{Level: []string{"error"}}},
		{name: "level with type=app", q: LogQuery{Types: app, Level: []string{"error"}}, want: app},
		{name: "level with type=request", q: LogQuery{Types: request, Level: []string{"error"}}, want: request},
		{name: "level keeps app+request", q: LogQuery{Types: both, Level: []string{"error"}}, want: both},
		{name: "level and a request filter follow request lines", q: LogQuery{Level: []string{"error"}, StatusCode: []string{"500"}}, want: request},
		{name: "type=app refuses statusCode", q: LogQuery{Types: app, StatusCode: []string{"500"}}, refuses: "statusCode"},
		{name: "type=app refuses method and path", q: LogQuery{Types: app, Method: []string{"GET"}, Path: []string{"/x"}}, refuses: "method, path"},
		{name: "type=build refuses host", q: LogQuery{Types: []string{LogTypeBuild}, Host: []string{"h"}}, refuses: "host"},
		{name: "type=build refuses level", q: LogQuery{Types: []string{LogTypeBuild}, Level: []string{"error"}}, refuses: "level"},
		{name: "a datastore keeps its own contract", q: LogQuery{App: keyValueID, Level: []string{"warning"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			routed, err := tc.q.liveSubscribable()
			if tc.refuses != "" {
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tc.refuses+" filters") {
					t.Fatalf("liveSubscribable(%+v) = %v, want a 400 naming %q", tc.q, err, tc.refuses)
				}
				return
			}
			if err != nil || !slices.Equal(routed.Types, tc.want) {
				t.Fatalf("liveSubscribable(%+v) = types %v, err %v; want %v", tc.q, routed.Types, err, tc.want)
			}
			// Routing is idempotent: a routed query routes to itself.
			again, err := routed.liveSubscribable()
			if err != nil || !slices.Equal(again.Types, routed.Types) {
				t.Fatalf("routing twice = %v, %v; want %v", again.Types, err, routed.Types)
			}
		})
	}
}

// liveFilterApp's pod writes lines at every shipper severity shape.
var liveFilterLines = map[string][]string{webInst: {
	`2026-07-05T00:00:01Z {"level":"error","msg":"json boom"}`,
	`2026-07-05T00:00:02Z {"level":"info","msg":"json fine"}`,
	`2026-07-05T00:00:03Z time=x level=error msg="logfmt boom"`,
	`2026-07-05T00:00:04Z {"level":50,"msg":"pino boom"}`,
	`2026-07-05T00:00:05Z plain words: error here`,
	`2026-07-05T00:00:06Z level=warn msg=careful`,
}}

// requestStore is a log store holding request lines, each labelled with its
// status class's level as the shipper labels them (w4/584); it applies a
// forwarded statusCode, path or level filter and records every query.
type requestStore struct {
	mu      sync.Mutex
	lines   []LogEntry
	queries []LogQuery
}

func (s *requestStore) history(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, q)
	if !q.wants(LogTypeRequest) {
		return nil, nil // the selector names no request stream
	}
	var out []LogEntry
	for _, e := range s.lines {
		if len(q.StatusCode) > 0 && !strings.Contains(e.Message, " "+q.StatusCode[0]) {
			continue
		}
		if len(q.Path) > 0 && !strings.Contains(e.Message, " "+q.Path[0]+" ") {
			continue
		}
		fields := strings.Fields(e.Message)
		level := statusLevel(fields[len(fields)-1])
		if len(q.Level) > 0 && !slices.Contains(storedLevels(q.Level), level) {
			continue
		}
		out = append(out, LogEntry{Timestamp: e.Timestamp, Message: e.Message, Labels: map[string]string{LabelType: LogTypeRequest, LabelLevel: level}})
	}
	return out, nil
}

func (s *requestStore) recorded() []LogQuery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.queries)
}

func newRequestStore() *requestStore {
	return &requestStore{lines: []LogEntry{
		{Timestamp: "2026-07-05T00:00:07Z", Message: "GET /mk-1 404"},
		{Timestamp: "2026-07-05T00:00:08Z", Message: "GET /other 500"},
		{Timestamp: "2026-07-05T00:00:09Z", Message: "POST /mk-1 500"},
	}}
}

// liveFilterTail runs one tail over liveFilterLines and the request store for
// a moment and returns what it emitted.
func liveFilterTail(t *testing.T, q LogQuery) ([]LogEntry, *scriptedStream, *requestStore) {
	t.Helper()
	stream := newScriptedStream(map[string][]string{webInst: {strings.Join(liveFilterLines[webInst], "\n")}})
	store := newRequestStore()
	svc := tailService(stream.follow, sampleApp("web"), appPodIn("web", webInst, "Running", containerRunning, 0))
	svc.History = store.history
	svc.RequestPollInterval = 5 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var emitted []LogEntry
	done := make(chan error, 1)
	go func() {
		done <- svc.FollowLogs(ctx, q, func(e LogEntry) error {
			mu.Lock()
			defer mu.Unlock()
			emitted = append(emitted, e)
			return nil
		})
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("FollowLogs(%+v): %v", q, err)
	}
	mu.Lock()
	defer mu.Unlock()
	return emitted, stream, store
}

func messages(entries []LogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Message)
	}
	slices.Sort(out)
	return out
}

func TestAppTailFiltersAppLinesByTheShipperLevel(t *testing.T) {
	appErrors := []string{`time=x level=error msg="logfmt boom"`, `{"level":"error","msg":"json boom"}`, `{"level":50,"msg":"pino boom"}`}
	for _, tc := range []struct {
		name string
		q    LogQuery
		want []string
		// polls: whether the request store is read, since a level selects
		// request lines too (w4/584).
		polls bool
	}{
		{"error", LogQuery{App: "web", Level: []string{"error"}}, append([]string{"GET /other 500", "POST /mk-1 500"}, appErrors...), true},
		{"type=app warning", LogQuery{App: "web", Types: []string{LogTypeApp}, Level: []string{"warning"}}, []string{`level=warn msg=careful`}, false},
		{"type=request warning", LogQuery{App: "web", Types: []string{LogTypeRequest}, Level: []string{"warning"}}, []string{"GET /mk-1 404"}, true},
		{"critical is the error bucket", LogQuery{App: "web", Level: []string{"critical"}}, append([]string{"GET /other 500", "POST /mk-1 500"}, appErrors...), true},
		{"unknown is the unstructured line", LogQuery{App: "web", Level: []string{"unknown"}}, []string{`plain words: error here`}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			emitted, _, store := liveFilterTail(t, tc.q)
			if got := messages(emitted); !slices.Equal(got, tc.want) {
				t.Fatalf("emitted %q, want %q", got, tc.want)
			}
			for _, e := range emitted {
				if e.Labels[LabelLevel] == "" {
					t.Errorf("line %q carries no level label", e.Message)
				}
			}
			if polled := len(store.recorded()) > 0; polled != tc.polls {
				t.Errorf("request store polled = %v, want %v", polled, tc.polls)
			}
			for _, q := range store.recorded() {
				if !slices.Equal(q.Level, tc.q.Level) {
					t.Fatalf("poll query level = %v, want the subscription's %v", q.Level, tc.q.Level)
				}
			}
		})
	}
}

func TestTypelessRequestFiltersTailRequestLinesAlone(t *testing.T) {
	for _, tc := range []struct {
		name string
		q    LogQuery
		want []string
	}{
		{"path", LogQuery{App: "web", Path: []string{"/mk-1"}}, []string{"GET /mk-1 404", "POST /mk-1 500"}},
		{"statusCode", LogQuery{App: "web", StatusCode: []string{"500"}}, []string{"GET /other 500", "POST /mk-1 500"}},
		{"type=request statusCode", LogQuery{App: "web", Types: []string{LogTypeRequest}, StatusCode: []string{"404"}}, []string{"GET /mk-1 404"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			emitted, stream, store := liveFilterTail(t, tc.q)
			if got := messages(emitted); !slices.Equal(got, tc.want) {
				t.Fatalf("emitted %q, want %q", got, tc.want)
			}
			if n := stream.openedFor(webInst); n != 0 {
				t.Errorf("a request-filter tail followed the app pod %d times", n)
			}
			for _, q := range store.recorded() {
				if !slices.Equal(q.Types, []string{LogTypeRequest}) || !slices.Equal(q.Path, tc.q.Path) || !slices.Equal(q.StatusCode, tc.q.StatusCode) {
					t.Fatalf("poll query = types %v path %v statusCode %v; want the subscription's request filters", q.Types, q.Path, q.StatusCode)
				}
			}
		})
	}
}

// With a level and a request filter, both apply to request lines, the only
// lines that carry both, as in a history query (w4/584).
func TestLevelAndRequestFiltersBothSelectRequestLines(t *testing.T) {
	emitted, _, store := liveFilterTail(t, LogQuery{App: "web", Level: []string{"error"}, StatusCode: []string{"500"}})
	want := []string{"GET /other 500", "POST /mk-1 500"}
	if got := messages(emitted); !slices.Equal(got, want) {
		t.Fatalf("emitted %q, want %q", got, want)
	}
	queries := store.recorded()
	if len(queries) == 0 {
		t.Fatal("the request store was never polled")
	}
	for _, q := range queries {
		if !slices.Equal(q.Level, []string{"error"}) || !slices.Equal(q.StatusCode, []string{"500"}) {
			t.Fatalf("poll query = level %v statusCode %v, want both of the subscription's filters", q.Level, q.StatusCode)
		}
	}
}

// The pre-slot refusals and routing hold on every subscribe transport, before
// any streaming header or WebSocket 101.
func TestSubscribeRoutesStructuredFiltersBeforeStreaming(t *testing.T) {
	newSvc := func(store bool) *Service {
		svc := newService(liveFilterLines, sampleApp("web"), podFor("web", webInst))
		if store {
			svc.History = newRequestStore().history
			svc.RequestPollInterval = 5 * time.Millisecond
		}
		return svc
	}
	for _, tc := range []struct {
		query        string
		store        bool
		status       int
		body, absent string
	}{
		{"resource=web&type=app&statusCode=500", true, http.StatusBadRequest, "statusCode filters request logs", ""},
		{"resource=web&type=build&level=error", true, http.StatusBadRequest, "level filters app and request logs", ""},
		{"resource=web&type=request&level=error", true, http.StatusOK, "GET /other 500", "GET /mk-1 404"},
		{"resource=web&path=/mk-1", false, http.StatusServiceUnavailable, "", ""},
		{"resource=web&path=/mk-1", true, http.StatusOK, "POST /mk-1 500", "json boom"},
		{"resource=web&level=error", false, http.StatusOK, "json boom", "json fine"},
	} {
		for _, accept := range []string{"text/event-stream", ""} {
			mux := http.NewServeMux()
			newSvc(tc.store).RegisterREST(mux)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?"+tc.query, nil)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			serveSubscribe(mux, rec, req)
			body := rec.Body.String()
			if rec.Code != tc.status || !strings.Contains(body, tc.body) || tc.absent != "" && strings.Contains(body, tc.absent) {
				t.Errorf("accept %q, %s (store %v) = %d %q; want %d with %q", accept, tc.query, tc.store, rec.Code, body, tc.status, tc.body)
			}
		}

		// The pinned Render CLI tails over WebSocket: a refusal is the HTTP
		// answer to the upgrade, and an accepted tail's frames carry the line.
		srv := httptest.NewServer(func() http.Handler {
			mux := http.NewServeMux()
			newSvc(tc.store).RegisterREST(mux)
			return mux
		}())
		conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?"+tc.query, nil)
		if tc.status != http.StatusOK {
			if err == nil || resp == nil || resp.StatusCode != tc.status {
				t.Errorf("WS %s: dial err %v, response %v; want HTTP %d before the upgrade", tc.query, err, resp, tc.status)
			}
		} else if err != nil {
			t.Errorf("WS %s: dial: %v", tc.query, err)
		} else {
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			for {
				_, frame, err := conn.ReadMessage()
				if err != nil {
					t.Errorf("WS %s: no frame with %q: %v", tc.query, tc.body, err)
					break
				}
				if strings.Contains(string(frame), tc.absent) {
					t.Errorf("WS %s: frame %s carries %q", tc.query, frame, tc.absent)
				}
				if strings.Contains(string(frame), tc.body) {
					if tc.query == "resource=web&level=error" && !strings.Contains(string(frame), `{"name":"level","value":"error"}`) {
						t.Errorf("WS %s: frame %s lacks its level label", tc.query, frame)
					}
					break
				}
			}
			_ = conn.Close()
		}
		srv.Close()
	}
}

// On a plan without HTTP request logs, request filters answer the plan error,
// as type=request does (w8/m138), instead of an empty request stream; `level`
// keeps streaming app lines.
func TestRequestFilterTailFollowsThePlan(t *testing.T) {
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelTenant: "tea-a"}
	svc := newService(liveFilterLines, app, podFor("web", webInst))
	svc.History = newRequestStore().history
	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "hobby", nil })

	err := svc.FollowLogs(context.Background(), LogQuery{App: "web", Path: []string{"/mk-1"}}, func(LogEntry) error { return nil })
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode {
		t.Fatalf("hobby path tail = %v, want the request-logs plan error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var got []string
	err = svc.FollowLogs(ctx, LogQuery{App: "web", Level: []string{"warning"}}, func(e LogEntry) error {
		got = append(got, e.Message)
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) || !slices.Equal(got, []string{"level=warn msg=careful"}) {
		t.Fatalf("hobby level tail = %q, %v; want the warning app line", got, err)
	}
}

// w4/585: Render emits no logs for a static site, so its request lines, which
// bex's edge records, are read on no surface: a history read of them is
// empty, discovery offers no request type, status or method, and a tail
// without type=build streams nothing.
func TestAStaticSiteHasNoRuntimeLogs(t *testing.T) {
	site := sampleApp("site")
	site.Spec.Type = appv1alpha1.TypeStaticSite
	svc := newService(liveFilterLines, site)
	svc.History = newRequestStore().history
	labels := map[string][]string{LabelType: {LogTypeRequest}, LabelStatusCode: {"500"}, LabelMethod: {"GET"}}
	svc.LabelValues = func(_ context.Context, _ string, label string, q LogQuery) ([]string, error) {
		if !q.wants(LogTypeRequest) {
			return nil, nil
		}
		return labels[label], nil
	}
	for _, q := range []LogQuery{{App: "site"}, {App: "site", Types: []string{LogTypeRequest}}} {
		entries, err := svc.QueryLogs(context.Background(), q)
		if err != nil || len(entries) != 0 {
			t.Fatalf("static site history %v = %d lines, %v; want none", q.Types, len(entries), err)
		}
	}
	for _, label := range []string{LabelType, LabelStatusCode, LabelMethod} {
		if values, err := svc.LogLabelValues(context.Background(), label, LogQuery{App: "site"}); err != nil || len(values) != 0 {
			t.Fatalf("static site %s values = %v, %v; want none", label, values, err)
		}
	}
	// A tail stays open and streams nothing, as a cron job's does between runs.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var tailed []LogEntry
	err := svc.FollowLogs(ctx, LogQuery{App: "site"}, func(e LogEntry) error {
		tailed = append(tailed, e)
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) || len(tailed) != 0 {
		t.Fatalf("static site tail = %d lines, %v; want none until the client leaves", len(tailed), err)
	}
}
