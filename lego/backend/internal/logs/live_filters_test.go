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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
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
	`2026-07-05T00:00:07Z level=crit msg="disk failing"`,
	`2026-07-05T00:00:08Z {"level":"notice","msg":"heads up"}`,
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
		if len(q.Level) > 0 && !slices.Contains(q.storedLevels(), level) {
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
		// Render's eight levels select their own lines (w3/086): a 5xx
		// request line and an app error line are `error`, not `critical`.
		{"critical", LogQuery{App: "web", Level: []string{"critical"}}, []string{`level=crit msg="disk failing"`}, true},
		{"notice", LogQuery{App: "web", Level: []string{"NOTICE"}}, []string{`{"level":"notice","msg":"heads up"}`}, true},
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
	svc := newService(recentLines(liveFilterLines), app, podFor("web", webInst))
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
	// A tail stays open and streams nothing, as a cron job's does between runs,
	// including with explicit type=request or request filters (w32/102) and
	// even when no durable log store is configured.
	for _, tc := range []struct {
		name  string
		store bool
		q     LogQuery
	}{
		{"unfiltered", true, LogQuery{App: "site"}},
		{"type=request", true, LogQuery{App: "site", Types: []string{LogTypeRequest}}},
		{"type=request without store", false, LogQuery{App: "site", Types: []string{LogTypeRequest}}},
		{"statusCode filter", true, LogQuery{App: "site", StatusCode: []string{"404"}}},
		{"method filter", true, LogQuery{App: "site", Method: []string{"GET"}}},
		{"path filter", true, LogQuery{App: "site", Path: []string{"/missing"}}},
		{"host filter", true, LogQuery{App: "site", Host: []string{"site.onbex.co"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newService(liveFilterLines, site)
			if tc.store {
				s.History = newRequestStore().history
				s.RequestPollInterval = 5 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			var tailed []LogEntry
			err := s.FollowLogs(ctx, tc.q, func(e LogEntry) error {
				tailed = append(tailed, e)
				return nil
			})
			if !errors.Is(err, context.DeadlineExceeded) || len(tailed) != 0 {
				t.Fatalf("static site tail %s = %d lines, %v; want none until the client leaves", tc.name, len(tailed), err)
			}
		})
	}
}

// w32/102: a static site tail with type=request or request filters stays open
// quietly on keepalives over both WebSocket and SSE (never tripping the
// post-upgrade bad-request backstop and entering a CLI reconnect loop), and
// mixed [web, static] and project-scoped tails keep streaming the web
// service's request lines without failing on the static site.
func TestStaticSiteRequestTailStaysOpenOverWebSocketAndSSE(t *testing.T) {
	site := sampleApp("site")
	site.Spec.Type = appv1alpha1.TypeStaticSite
	web := sampleApp("web")

	newSvc := func() (*Service, *requestStore) {
		store := newRequestStore()
		svc := tailService(newScriptedStream(nil).follow, web, site, podFor("web", webInst))
		svc.History = store.history
		svc.RequestPollInterval = 5 * time.Millisecond
		return svc, store
	}

	// Single static site: WebSocket receives keepalive pings with zero frames
	// and never closes; SSE receives keepalive comments with zero event: error.
	for _, query := range []string{
		"resource=site&type=request",
		"resource=site&statusCode=404",
		"resource=site&method=GET&path=/mk-1",
	} {
		t.Run("WS "+query, func(t *testing.T) {
			svc, store := newSvc()
			srv := revalidationTestServer(svc)
			defer srv.Close()

			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?" + query
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("WS dial %s: %v (resp=%v)", query, err, resp)
			}
			defer conn.Close()

			pinged := make(chan struct{}, 1)
			conn.SetPingHandler(func(string) error {
				select {
				case pinged <- struct{}{}:
				default:
				}
				return nil
			})
			readDone := make(chan error, 1)
			go func() {
				_, payload, err := conn.ReadMessage()
				if err == nil {
					readDone <- fmt.Errorf("unexpected log frame: %s", payload)
					return
				}
				readDone <- err
			}()
			select {
			case <-pinged:
			case err := <-readDone:
				t.Fatalf("WS %s ended instead of staying open on keepalives: %v", query, err)
			case <-time.After(5 * time.Second):
				t.Fatalf("WS %s sent no keepalive ping", query)
			}
			if got := store.recorded(); len(got) != 0 {
				t.Errorf("WS %s polled the request store %d times for a static site", query, len(got))
			}
		})

		t.Run("SSE "+query, func(t *testing.T) {
			svc, store := newSvc()
			srv := revalidationTestServer(svc)
			defer srv.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/logs/subscribe?"+query, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Accept", "text/event-stream")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("SSE %s: %v", query, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("SSE %s status = %d, want 200", query, resp.StatusCode)
			}
			lineCh := make(chan string, 4)
			go func() {
				sc := bufio.NewScanner(resp.Body)
				for sc.Scan() {
					lineCh <- sc.Text()
				}
				close(lineCh)
			}()
			select {
			case line, ok := <-lineCh:
				if !ok {
					t.Fatalf("SSE %s closed instead of keeping alive", query)
				}
				if line != ": keepalive" {
					t.Fatalf("SSE %s first line = %q, want keepalive", query, line)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("SSE %s sent no keepalive", query)
			}
			if got := store.recorded(); len(got) != 0 {
				t.Errorf("SSE %s polled the request store %d times for a static site", query, len(got))
			}
		})
	}

	// Mixed [web, site] with type=request and statusCode=404: delivers web's
	// request line without failing on the static site.
	for _, query := range []string{
		"resource=web&resource=site&type=request",
		"resource=web&resource=site&statusCode=404",
	} {
		t.Run("mixed WS "+query, func(t *testing.T) {
			svc, _ := newSvc()
			srv := revalidationTestServer(svc)
			defer srv.Close()

			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?" + query
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("WS dial %s: %v", query, err)
			}
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, frame, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("WS %s read: %v", query, err)
			}
			if !strings.Contains(string(frame), "GET /mk-1 404") || strings.Contains(string(frame), `"value":"site"`) {
				t.Fatalf("WS %s frame = %s, want web's 404 request line and nothing from site", query, frame)
			}
		})
	}

	// Project-scoped tail with statusCode=404 over [web, site]: site stays open
	// quietly and does not kill the project stream.
	t.Run("project scope statusCode=404 with static site", func(t *testing.T) {
		svc, _ := newSvc()
		svc.Projects = &fakeProject{
			workspace: "tea-a",
			resources: []savedsearch.Resource{
				{ID: "web", Type: appv1alpha1.TypeWebService},
				{ID: "site", Type: appv1alpha1.TypeStaticSite},
			},
		}
		srv := revalidationTestServer(svc)
		defer srv.Close()

		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?project=prj-observed&statusCode=404"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("WS dial project scope: %v", err)
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, frame, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("WS project scope read: %v", err)
		}
		if !strings.Contains(string(frame), "GET /mk-1 404") {
			t.Fatalf("WS project scope frame = %s, want web's 404 line", frame)
		}
	})

	// Mixed type=build + type=request on a static site: request side stays idle
	// while build logs still stream cleanly.
	t.Run("static site type=build&type=request streams builds", func(t *testing.T) {
		buildPod := buildPodFor("site", "bld-site-gen-1-run", "builds", "buildkit", time.Now())
		lines := map[string][]string{
			buildPod.Name: {time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano) + " static build ok"},
		}
		svc := newService(lines, site, buildPod)
		svc.BuildNamespace = "builds"
		svc.BuildPodWaitInterval = 10 * time.Millisecond
		svc.History = newRequestStore().history
		srv := revalidationTestServer(svc)
		defer srv.Close()

		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?resource=site&type=build&type=request"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("WS dial: %v", err)
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, frame, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("WS read: %v", err)
		}
		if !strings.Contains(string(frame), "static build ok") {
			t.Fatalf("frame = %s, want the static site's build line", frame)
		}
	})
}

// w32/102: subscribeWebSocket emits the `==>` refusal Log frame and closes with
// CloseTailRefused (4000) for every post-upgrade server error (not just the
// original three sentinels), redacts unclassified internal errors, and stays
// silent on normal client disconnects.
func TestSubscribeWebSocketTerminatesEveryPostUpgradeRefusalWith4000(t *testing.T) {
	planErr := core.NewPlanFeatureError("type", "hobby", core.FeatureRequestLogs)
	for _, tc := range []struct {
		name     string
		err      error
		wantMsg  string
		forbidIn string
	}{
		{
			name:    "bad request",
			err:     fmt.Errorf("%w: invalid filter syntax", core.ErrBadRequest),
			wantMsg: "bad request: invalid filter syntax",
		},
		{
			name:    "plan feature required",
			err:     planErr,
			wantMsg: planErr.Error(),
		},
		{
			name:    "forbidden",
			err:     fmt.Errorf("%w: access revoked", core.ErrForbidden),
			wantMsg: "forbidden: access revoked",
		},
		{
			name:    "not found",
			err:     fmt.Errorf("%w: project deleted", core.ErrNotFound),
			wantMsg: "not found: project deleted",
		},
		{
			name:     "internal error redacted",
			err:      errors.New("pgx: failed connecting to postgres://secret:pass@db.internal:5432/bex"),
			wantMsg:  "internal error",
			forbidIn: "secret",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				svc.subscribeWebSocket(w, r, func(context.Context, func(LogEntry) error, func() error) error {
					return tc.err
				})
			}))
			defer srv.Close()

			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe"
			conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("WS dial: %v", err)
			}
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, payload, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("read refusal frame: %v", err)
			}
			var line renderLog
			if err := json.Unmarshal(payload, &line); err != nil {
				t.Fatalf("frame %s is not a Log: %v", payload, err)
			}
			if line.Message != "==> "+tc.wantMsg {
				t.Errorf("message = %q, want %q", line.Message, "==> "+tc.wantMsg)
			}
			if tc.forbidIn != "" && strings.Contains(string(payload), tc.forbidIn) {
				t.Errorf("frame leaked internal error: %s", payload)
			}
			_, _, err = conn.ReadMessage()
			var closeErr *websocket.CloseError
			if !errors.As(err, &closeErr) || closeErr.Code != CloseTailRefused {
				t.Fatalf("after refusal frame: err = %v, want close %d", err, CloseTailRefused)
			}
			if closeErr.Text != closeReason(errors.New(tc.wantMsg)) {
				t.Errorf("close reason = %q, want %q", closeErr.Text, closeReason(errors.New(tc.wantMsg)))
			}
			if tc.forbidIn != "" && strings.Contains(closeErr.Text, tc.forbidIn) {
				t.Errorf("close reason leaked internal error: %q", closeErr.Text)
			}
		})
	}

	// Real handler path: a project-scoped WebSocket tail whose project disappears
	// on rescope receives the `==>` refusal frame + CloseTailRefused (4000).
	t.Run("project scope rescope not found sends 4000", func(t *testing.T) {
		proj := &fakeProject{
			workspace: "tea-a",
			resources: []savedsearch.Resource{{ID: "web", Type: appv1alpha1.TypeWebService}},
		}
		svc := tailService(newScriptedStream(nil).follow, sampleApp("web"))
		svc.Projects = proj
		svc.ProjectRescopeInterval = 20 * time.Millisecond
		srv := revalidationTestServer(svc)
		defer srv.Close()

		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?project=prj-observed"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("WS dial: %v", err)
		}
		defer conn.Close()

		proj.mu.Lock()
		proj.err = core.NotFound("project")
		proj.mu.Unlock()

		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read refusal frame after rescope failure: %v", err)
		}
		var line renderLog
		if err := json.Unmarshal(payload, &line); err != nil {
			t.Fatalf("frame %s is not a Log: %v", payload, err)
		}
		if !strings.Contains(line.Message, "==> ") || !strings.Contains(line.Message, "project") {
			t.Errorf("message = %q, want ==> refusal naming project", line.Message)
		}
		_, _, err = conn.ReadMessage()
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) || closeErr.Code != CloseTailRefused {
			t.Fatalf("after rescope refusal: err = %v, want close %d", err, CloseTailRefused)
		}
	})

	// Control: normal peer disconnect ends silently without attempting a refusal
	// frame, and a write failure after peer disconnect does not crash or emit one.
	t.Run("peer disconnect stays silent", func(t *testing.T) {
		svc := &Service{}
		followDone := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			svc.subscribeWebSocket(w, r, func(ctx context.Context, emit func(LogEntry) error, _ func() error) error {
				defer close(followDone)
				<-ctx.Done()
				// Even if a follower tries to emit onto the closed socket
				// during shutdown, the write error is treated as a quiet disconnect.
				return emit(LogEntry{Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Message: "late"})
			})
		}))
		defer srv.Close()

		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe", nil)
		if err != nil {
			t.Fatalf("WS dial: %v", err)
		}
		_ = conn.Close()
		select {
		case <-followDone:
		case <-time.After(5 * time.Second):
			t.Fatal("follow did not stop after peer closed WebSocket")
		}
	})
}

// w32/102: on a Hobby workspace, request-log tails (explicit type=request or
// request filters) are refused before WebSocket upgrade / SSE 200 with HTTP 400
// PLAN_FEATURE_REQUIRED (matching GET /v1/logs) and release their slot.
// type=predeploy remains refused with HTTP 400 pre-upgrade for both web and
// static sites.
func TestSubscribeRefusesHobbyRequestLogsAndPredeployBeforeUpgrade(t *testing.T) {
	web := sampleApp("web")
	web.Labels = map[string]string{core.LabelTenant: "tea-a"}
	site := sampleApp("site")
	site.Spec.Type = appv1alpha1.TypeStaticSite
	site.Labels = map[string]string{core.LabelTenant: "tea-a"}

	newHobbySvc := func() *Service {
		svc := newService(recentLines(liveFilterLines), web, site, podFor("web", webInst))
		svc.History = newRequestStore().history
		svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "hobby", nil })
		svc.MaxSSEConns = 1
		svc.Projects = &fakeProject{
			workspace: "tea-a",
			resources: []savedsearch.Resource{{ID: "web", Type: appv1alpha1.TypeWebService}},
		}
		return svc
	}

	for _, tc := range []struct {
		query    string
		wantCode string
		wantText string
	}{
		{"resource=web&type=request", core.PlanFeatureRequiredCode, "HTTP request logs require the Pro plan"},
		{"resource=web&statusCode=404", core.PlanFeatureRequiredCode, "HTTP request logs require the Pro plan"},
		{"resource=site&type=request", core.PlanFeatureRequiredCode, "HTTP request logs require the Pro plan"},
		{"project=prj-observed&statusCode=404", core.PlanFeatureRequiredCode, "HTTP request logs require the Pro plan"},
		{"resource=web&type=predeploy", "", "has no live producer"},
		{"resource=site&type=predeploy", "", "has no live producer"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			svc := newHobbySvc()
			srv := revalidationTestServer(svc)
			defer srv.Close()

			// WebSocket upgrade must be refused with HTTP 400 before 101.
			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/v1/logs/subscribe?" + tc.query
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
			if conn != nil {
				_ = conn.Close()
			}
			if err == nil || resp == nil || resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("WS %s: err=%v resp=%v, want HTTP 400 before upgrade", tc.query, err, resp)
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body := string(bodyBytes)
			if !strings.Contains(body, tc.wantText) || (tc.wantCode != "" && !strings.Contains(body, tc.wantCode)) {
				t.Errorf("WS %s body = %s, want %q and code %q", tc.query, body, tc.wantText, tc.wantCode)
			}
			waitForSlotRelease(t, svc)

			// SSE must also answer HTTP 400 before streaming headers.
			req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/logs/subscribe?"+tc.query, nil)
			req.Header.Set("Accept", "text/event-stream")
			sseResp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("SSE %s: %v", tc.query, err)
			}
			sseBody, _ := io.ReadAll(sseResp.Body)
			_ = sseResp.Body.Close()
			if sseResp.StatusCode != http.StatusBadRequest || !strings.Contains(string(sseBody), tc.wantText) {
				t.Errorf("SSE %s = %d %s, want 400 with %q", tc.query, sseResp.StatusCode, sseBody, tc.wantText)
			}
			waitForSlotRelease(t, svc)
		})
	}
}
