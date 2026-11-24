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
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type mixedHistoryFixture struct {
	mu      sync.Mutex
	entries []LogEntry
}

func (f *mixedHistoryFixture) query(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []LogEntry
	for _, e := range f.entries {
		res := e.Labels["service"]
		switch {
		case q.App != "" && res != q.App:
			continue
		case q.Database != "" && res != q.Database:
			continue
		case q.KeyValue != "" && res != q.KeyValue:
			continue
		}
		if q.App != "" && len(q.Types) > 0 && !slices.Contains(q.Types, e.Labels[LabelType]) {
			continue
		}
		if len(q.StatusCode) > 0 && !slices.Contains(q.StatusCode, e.Labels[LabelStatusCode]) {
			continue
		}
		if len(q.Method) > 0 && !slices.Contains(q.Method, e.Labels[LabelMethod]) {
			continue
		}
		if len(q.Path) > 0 && !slices.Contains(q.Path, e.Labels[LabelPath]) {
			continue
		}
		if len(q.Host) > 0 && !slices.Contains(q.Host, e.Labels[LabelHost]) {
			continue
		}
		if len(q.Build) > 0 && !slices.Contains(q.Build, e.Labels[LabelBuild]) {
			continue
		}
		out = append(out, LogEntry{
			Timestamp: e.Timestamp,
			Message:   e.Message,
			Labels:    cloneStringMap(e.Labels),
		})
	}
	return out, nil
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func newMixedResourceService(extra ...client.Object) (*Service, *fakeProject) {
	web := sampleApp("web")
	web.Status.URLs = []string{"https://web.onbex.co"}
	cron := sampleApp("nightly")
	cron.Spec.Type = appv1alpha1.TypeCronJob
	site := sampleApp("site")
	site.Spec.Type = appv1alpha1.TypeStaticSite
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: keyValueID, Namespace: "default", Labels: map[string]string{core.LabelTenant: "tea-home"}}}
	pg := sampleDatabase(postgresID)
	pg.Labels = map[string]string{core.LabelTenant: "tea-home"}
	web.Labels = map[string]string{core.LabelTenant: "tea-home"}
	cron.Labels = map[string]string{core.LabelTenant: "tea-home"}
	site.Labels = map[string]string{core.LabelTenant: "tea-home"}

	objs := []client.Object{
		web, podFor("web", "web-1"),
		cron, podFor("nightly", "nightly-1"),
		site,
		pg, databasePod(postgresID, postgresID+"-1"),
		kv, keyValuePod(keyValueID, keyValueID+"-0"),
	}
	objs = append(objs, extra...)
	svc := newService(managedTailLogs, objs...)
	hist := &mixedHistoryFixture{
		entries: []LogEntry{
			{Timestamp: "2026-07-05T00:00:01Z", Message: "web app line", Labels: map[string]string{"service": "web", LabelInstance: "web-1", LabelType: LogTypeApp, LabelLevel: "info"}},
			{Timestamp: "2026-07-05T00:00:02Z", Message: "GET /ok 200", Labels: map[string]string{"service": "web", LabelType: LogTypeRequest, LabelStatusCode: "200", LabelMethod: "GET", LabelPath: "/ok", LabelHost: "web.onbex.co", LabelLevel: "info"}},
			{Timestamp: "2026-07-05T00:00:02Z", Message: "GET /ok 200 sibling", Labels: map[string]string{"service": "web", LabelType: LogTypeRequest, LabelStatusCode: "200", LabelMethod: "GET", LabelPath: "/ok", LabelHost: "web.onbex.co", LabelLevel: "info"}},
			{Timestamp: "2026-07-05T00:00:03Z", Message: "web build line", Labels: map[string]string{"service": "web", LabelInstance: "bld-web-1", LabelType: LogTypeBuild, LabelBuild: "dep-1"}},
			{Timestamp: "2026-07-05T00:00:04Z", Message: "site build line", Labels: map[string]string{"service": "site", LabelInstance: "bld-site-1", LabelType: LogTypeBuild, LabelBuild: "dep-site"}},
			{Timestamp: "2026-07-05T00:00:05Z", Message: "pg checkpoint", Labels: map[string]string{"service": postgresID, LabelInstance: postgresID + "-1", LabelType: "postgres", LabelLevel: "info"}},
			{Timestamp: "2026-07-05T00:00:06Z", Message: "kv ready", Labels: map[string]string{"service": keyValueID, LabelInstance: keyValueID + "-0", LabelType: "key_value", LabelLevel: "warning"}},
		},
	}
	svc.History = hist.query
	svc.LabelValues = func(_ context.Context, _ string, label string, q LogQuery) ([]string, error) {
		switch {
		case q.App == "web":
			switch label {
			case LabelStatusCode:
				return []string{"200", "404"}, nil
			case LabelMethod:
				return []string{"GET", "POST"}, nil
			case LabelType:
				return []string{LogTypeApp, LogTypeRequest, LogTypeBuild}, nil
			case LabelLevel:
				return []string{"info", "error"}, nil
			case LabelInstance:
				return []string{"web-1"}, nil
			}
		case q.Database == postgresID:
			switch label {
			case LabelLevel:
				return []string{"LOG"}, nil
			case LabelInstance:
				return []string{postgresID + "-1"}, nil
			}
		case q.KeyValue == keyValueID:
			switch label {
			case LabelLevel:
				return []string{"warning"}, nil
			case LabelInstance:
				return []string{keyValueID + "-0"}, nil
			}
		}
		return nil, nil
	}
	proj := &fakeProject{
		workspace: "tea-home",
		resources: []savedsearch.Resource{
			{ID: "web", Name: "web", Type: appv1alpha1.TypeWebService, EnvironmentID: "evm-1"},
			{ID: "nightly", Name: "nightly", Type: appv1alpha1.TypeCronJob, EnvironmentID: "evm-2"},
			{ID: "site", Name: "site", Type: appv1alpha1.TypeStaticSite, EnvironmentID: "evm-2"},
			{ID: postgresID, Name: "db", Type: savedsearch.KindPostgres, EnvironmentID: "evm-2"},
			{ID: keyValueID, Name: "cache", Type: savedsearch.KindKeyValue, EnvironmentID: "evm-1"},
		},
	}
	svc.Projects = proj
	return svc, proj
}

func connectTestMCP(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(context.Background(), serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func renderLogMessages(list renderLogList) []string {
	out := make([]string, 0, len(list.Logs))
	for _, l := range list.Logs {
		out = append(out, l.Message)
	}
	return out
}

// w32/107: mixed service + datastore explicit log reads honor type, request,
// and build filters without failing with 400 or leaking datastore lines, while
// single-datastore and datastore-only reads keep datastoreFilters's 400 refusal.
func TestMixedResourceHistoryQueriesRESTAndMCP(t *testing.T) {
	svc, _ := newMixedResourceService()
	cs := connectTestMCP(t, svc)
	const window = "startTime=2026-07-05T00:00:00Z&endTime=2026-07-05T01:00:00Z"

	for _, ds := range []struct {
		id  string
		msg string
	}{
		{postgresID, "pg checkpoint"},
		{keyValueID, "kv ready"},
	} {
		cases := []struct {
			name      string
			query     string
			mcpArgs   map[string]any
			want      []string
			forbidden []string
		}{
			{
				name:      "type=app includes app and datastore process logs",
				query:     "type=app",
				mcpArgs:   map[string]any{"type": []string{"app"}},
				want:      []string{"web app line", ds.msg},
				forbidden: []string{"GET /ok 200", "web build line"},
			},
			{
				name:      "type=request skips datastore",
				query:     "type=request",
				mcpArgs:   map[string]any{"type": []string{"request"}},
				want:      []string{"GET /ok 200", "GET /ok 200 sibling"},
				forbidden: []string{ds.msg, "web app line", "web build line"},
			},
			{
				name:      "type=build skips datastore",
				query:     "type=build",
				mcpArgs:   map[string]any{"type": []string{"build"}},
				want:      []string{"web build line"},
				forbidden: []string{ds.msg, "web app line", "GET /ok 200"},
			},
			{
				name:      "type=request&type=build skips datastore without leaking",
				query:     "type=request&type=build",
				mcpArgs:   map[string]any{"type": []string{"request", "build"}},
				want:      []string{"GET /ok 200", "web build line"},
				forbidden: []string{ds.msg, "web app line"},
			},
			{
				name:      "type=app&type=build includes datastore process logs",
				query:     "type=app&type=build",
				mcpArgs:   map[string]any{"type": []string{"app", "build"}},
				want:      []string{"web app line", "web build line", ds.msg},
				forbidden: []string{"GET /ok 200"},
			},
			{
				name:      "statusCode skips datastore",
				query:     "statusCode=200",
				mcpArgs:   map[string]any{"statusCode": []string{"200"}},
				want:      []string{"GET /ok 200"},
				forbidden: []string{ds.msg},
			},
			{
				name:      "method skips datastore",
				query:     "method=GET",
				mcpArgs:   map[string]any{"method": []string{"GET"}},
				want:      []string{"GET /ok 200"},
				forbidden: []string{ds.msg},
			},
			{
				name:      "path skips datastore",
				query:     "path=/ok",
				mcpArgs:   map[string]any{"path": []string{"/ok"}},
				want:      []string{"GET /ok 200"},
				forbidden: []string{ds.msg},
			},
			{
				name:      "host skips datastore",
				query:     "host=web.onbex.co",
				mcpArgs:   map[string]any{"host": []string{"web.onbex.co"}},
				want:      []string{"GET /ok 200"},
				forbidden: []string{ds.msg},
			},
			{
				name:      "build skips datastore",
				query:     "build=dep-1",
				mcpArgs:   map[string]any{"build": []string{"dep-1"}},
				want:      []string{"web build line"},
				forbidden: []string{ds.msg},
			},
			{
				name:      "limit=1 shared-timestamp lineBeyond probe skips datastore",
				query:     "statusCode=200&limit=1",
				mcpArgs:   map[string]any{"statusCode": []string{"200"}, "limit": 1},
				want:      []string{"GET /ok 200", "GET /ok 200 sibling"},
				forbidden: []string{ds.msg},
			},
		}
		for _, tc := range cases {
			for _, order := range [][]string{{"web", ds.id}, {ds.id, "web"}} {
				rec := serveREST(svc, http.MethodGet, "/v1/logs?resource="+order[0]+"&resource="+order[1]+"&"+tc.query+"&"+window)
				if rec.Code != http.StatusOK {
					t.Fatalf("REST %v %s => %d %s", order, tc.name, rec.Code, rec.Body.String())
				}
				var got renderLogList
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("REST decode: %v", err)
				}
				msgs := renderLogMessages(got)
				for _, w := range tc.want {
					if !slices.Contains(msgs, w) {
						t.Errorf("REST %v %s: missing %q in %v", order, tc.name, w, msgs)
					}
				}
				for _, f := range tc.forbidden {
					if slices.Contains(msgs, f) {
						t.Errorf("REST %v %s: leaked %q in %v", order, tc.name, f, msgs)
					}
				}

				args := map[string]any{
					"resource":  order,
					"startTime": "2026-07-05T00:00:00Z",
					"endTime":   "2026-07-05T01:00:00Z",
				}
				for k, v := range tc.mcpArgs {
					args[k] = v
				}
				res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs", Arguments: args})
				if err != nil || res.IsError {
					t.Fatalf("MCP %v %s: err=%v res=%+v", order, tc.name, err, res)
				}
				var mcpGot renderLogList
				rendertest.DecodeMCPText(t, res, &mcpGot)
				mcpMsgs := renderLogMessages(mcpGot)
				if !slices.Equal(mcpMsgs, msgs) {
					t.Errorf("MCP %v %s = %v, want REST's %v", order, tc.name, mcpMsgs, msgs)
				}
			}
		}
	}

	// Single-datastore and datastore-only controls keep their 400 refusal.
	for _, query := range []string{
		"resource=" + postgresID + "&type=request",
		"resource=" + keyValueID + "&type=app",
		"resource=" + keyValueID + "&statusCode=200",
		"resource=" + postgresID + "&build=dep-1",
		"resource=" + postgresID + "&resource=" + keyValueID + "&statusCode=200",
		"resource=" + postgresID + "&resource=" + keyValueID + "&type=app",
	} {
		rec := serveREST(svc, http.MethodGet, "/v1/logs?"+query+"&"+window)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("control REST %s => %d %s, want 400", query, rec.Code, rec.Body.String())
		}
	}
}

// w32/107: mixed service + datastore label-value discovery skips datastores for
// service-only labels and non-datastore filters, includes them on type=app, and
// keeps single-datastore 400 refusals.
func TestMixedResourceLabelValuesRESTAndMCP(t *testing.T) {
	svc, _ := newMixedResourceService()
	cs := connectTestMCP(t, svc)
	const window = "startTime=2026-07-05T00:00:00Z&endTime=2026-07-05T01:00:00Z"

	for _, ds := range []struct {
		id    string
		level string
	}{
		{postgresID, "LOG"},
		{keyValueID, "warning"},
	} {
		cases := []struct {
			name    string
			query   string
			mcpArgs map[string]any
			want    []string
		}{
			{
				name:    "label=statusCode skips datastore",
				query:   "label=statusCode",
				mcpArgs: map[string]any{"label": "statusCode"},
				want:    []string{"200", "404"},
			},
			{
				name:    "label=method skips datastore",
				query:   "label=method",
				mcpArgs: map[string]any{"label": "method"},
				want:    []string{"GET", "POST"},
			},
			{
				name:    "label=type skips datastore",
				query:   "label=type",
				mcpArgs: map[string]any{"label": "type"},
				want:    []string{"app", "build", "request"},
			},
			{
				name:    "label=level&type=app merges app and datastore",
				query:   "label=level&type=app",
				mcpArgs: map[string]any{"label": "level", "type": []string{"app"}},
				want:    slices.Sorted(slices.Values([]string{"info", "error", ds.level})),
			},
			{
				name:    "label=level&statusCode=200 skips datastore",
				query:   "label=level&statusCode=200",
				mcpArgs: map[string]any{"label": "level", "statusCode": []string{"200"}},
				want:    []string{"error", "info"},
			},
			{
				name:    "label=instance&type=build skips datastore",
				query:   "label=instance&type=build",
				mcpArgs: map[string]any{"label": "instance", "type": []string{"build"}},
				want:    instancesOf("web", "web-1"),
			},
		}
		for _, tc := range cases {
			for _, order := range [][]string{{"web", ds.id}, {ds.id, "web"}} {
				rec := serveREST(svc, http.MethodGet, "/v1/logs/values?resource="+order[0]+"&resource="+order[1]+"&"+tc.query+"&"+window)
				if rec.Code != http.StatusOK {
					t.Fatalf("REST values %v %s => %d %s", order, tc.name, rec.Code, rec.Body.String())
				}
				var got []string
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(got, tc.want) {
					t.Errorf("REST values %v %s = %v, want %v", order, tc.name, got, tc.want)
				}

				args := map[string]any{
					"resource":  order,
					"startTime": "2026-07-05T00:00:00Z",
					"endTime":   "2026-07-05T01:00:00Z",
				}
				for k, v := range tc.mcpArgs {
					args[k] = v
				}
				res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_log_label_values", Arguments: args})
				if err != nil || res.IsError {
					t.Fatalf("MCP values %v %s: err=%v res=%+v", order, tc.name, err, res)
				}
				var mcpGot []string
				rendertest.DecodeMCPText(t, res, &mcpGot)
				if !slices.Equal(mcpGot, tc.want) {
					t.Errorf("MCP values %v %s = %v, want %v", order, tc.name, mcpGot, tc.want)
				}
			}
		}
	}

	// Single-datastore, datastore-only, and unknown-label controls keep 400.
	for _, query := range []string{
		"resource=" + keyValueID + "&label=statusCode",
		"resource=" + keyValueID + "&label=level&type=app",
		"resource=" + postgresID + "&resource=" + keyValueID + "&label=statusCode",
		"resource=" + keyValueID + "&resource=web&label=bogus",
	} {
		rec := serveREST(svc, http.MethodGet, "/v1/logs/values?"+query+"&"+window)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("control REST values %s => %d %s, want 400", query, rec.Code, rec.Body.String())
		}
	}
}

// w32/107: mixed service + datastore live tails (WS and SSE) stream both on
// type=app, stream the service alone on request/build filters without burning
// a slot on the skipped datastore, and keep single-datastore 400 refusals.
func TestMixedResourceSubscribeWSAndSSE(t *testing.T) {
	t.Run("type=app streams both service and datastore over SSE and WS", func(t *testing.T) {
		for _, ds := range []struct {
			id   string
			want string
		}{
			{postgresID, "password authentication failed"},
			{keyValueID, "Memory overcommit"},
		} {
			svc := newManagedTailService()
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?resource=web&resource="+ds.id+"&type=app", nil)
			req.Header.Set("Accept", "text/event-stream")
			serveSubscribe(mux, rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("SSE type=app %s => %d %s", ds.id, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, "from web") || !strings.Contains(body, ds.want) {
				t.Errorf("SSE type=app %s body lacks web or datastore line:\n%s", ds.id, body)
			}

			srv := httptest.NewServer(mux)
			conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web&resource="+ds.id+"&type=app&startTime=2026-07-05T00:00:00Z", nil)
			if err != nil {
				srv.Close()
				t.Fatalf("WS type=app %s dial: %v (resp=%v)", ds.id, err, resp)
			}
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			var seenWeb, seenDS bool
			for !(seenWeb && seenDS) {
				_, frame, rErr := conn.ReadMessage()
				if rErr != nil {
					break
				}
				s := string(frame)
				if strings.Contains(s, "from web") {
					seenWeb = true
				}
				if strings.Contains(s, ds.want) {
					seenDS = true
				}
			}
			_ = conn.Close()
			srv.Close()
			if !seenWeb || !seenDS {
				t.Errorf("WS type=app %s: seenWeb=%v seenDS=%v", ds.id, seenWeb, seenDS)
			}
		}
	})

	t.Run("request filter streams service alone and uses one slot", func(t *testing.T) {
		svc, _ := newMixedResourceService()
		svc.MaxSSEConns = 1
		svc.RequestPollInterval = 10 * time.Millisecond
		mux := http.NewServeMux()
		svc.RegisterREST(mux)

		// SSE with MaxSSEConns=1 succeeds because keyValueID is skipped and holds no slot.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?resource=web&resource="+keyValueID+"&statusCode=200&startTime=2026-07-05T00:00:00Z", nil)
		req.Header.Set("Accept", "text/event-stream")
		serveSubscribe(mux, rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "GET /ok 200") || strings.Contains(rec.Body.String(), "kv ready") {
			t.Fatalf("SSE mixed statusCode=200 => %d %s", rec.Code, rec.Body.String())
		}

		// WS with type=request succeeds and delivers web's request log.
		srv := httptest.NewServer(mux)
		defer srv.Close()
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web&resource="+postgresID+"&type=request&startTime=2026-07-05T00:00:00Z", nil)
		if err != nil {
			t.Fatalf("WS mixed type=request dial: %v", err)
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, frame, err := conn.ReadMessage()
		if err != nil || !strings.Contains(string(frame), "GET /ok 200") {
			t.Fatalf("WS mixed type=request frame = %s, err=%v", frame, err)
		}
	})

	t.Run("build tail streams service build alone", func(t *testing.T) {
		buildPod := buildPodFor("web", "bld-web-gen-1", "builds", "buildkit", time.Now())
		svc := newService(map[string][]string{
			buildPod.Name: {time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano) + " mixed build ok"},
		}, sampleApp("web"), buildPod, &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: keyValueID, Namespace: "default"}})
		svc.BuildNamespace = "builds"
		svc.BuildPodWaitInterval = 10 * time.Millisecond
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		srv := httptest.NewServer(mux)
		defer srv.Close()

		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web&resource="+keyValueID+"&type=build", nil)
		if err != nil {
			t.Fatalf("WS mixed type=build dial: %v", err)
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, frame, err := conn.ReadMessage()
		if err != nil || !strings.Contains(string(frame), "mixed build ok") {
			t.Fatalf("WS mixed type=build frame = %s, err=%v", frame, err)
		}
	})

	t.Run("incompatible app filter, missing store, and single-datastore controls still refuse pre-upgrade", func(t *testing.T) {
		for _, tc := range []struct {
			query      string
			wantStatus int
		}{
			{"resource=web&resource=" + keyValueID + "&type=app&statusCode=200", http.StatusBadRequest},
			{"resource=web&resource=" + keyValueID + "&type=predeploy", http.StatusBadRequest},
			{"resource=web&resource=" + keyValueID + "&method=GET", http.StatusServiceUnavailable},
			{"resource=" + keyValueID + "&type=app", http.StatusBadRequest},
			{"resource=" + keyValueID + "&statusCode=200", http.StatusBadRequest},
			{"resource=" + postgresID + "&resource=" + keyValueID + "&type=request", http.StatusBadRequest},
		} {
			svc := newManagedTailService() // no History wired -> request-only returns 503
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?"+tc.query, nil)
			req.Header.Set("Accept", "text/event-stream")
			serveSubscribe(mux, rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("subscribe %s => %d %s, want %d", tc.query, rec.Code, rec.Body.String(), tc.wantStatus)
			}
		}
	})
}

// workspaceDenyChecker denies can_view_logs on a specific workspace while
// allowing all other workspaces.
type workspaceDenyChecker struct {
	denyWorkspace string
}

func (c workspaceDenyChecker) Check(_ context.Context, _, relation, object string) (bool, error) {
	if relation == core.RelCanViewLogs && object == core.WorkspaceObject(c.denyWorkspace) {
		return false, nil
	}
	return true, nil
}

// w32/107: skipping a datastore on a mixed query must still authorize it,
// preserving 404 (missing or deleting) and 403 (forbidden) across REST, MCP,
// values, and subscribe, with the proper audit verb recorded.
func TestMixedResourceAuthorizationPreservedWhenDatastoreSkipped(t *testing.T) {
	missingPG := ids.New(ids.Postgres)
	missingKV := ids.New(ids.KeyValue)
	deletingKV := ids.New(ids.KeyValue)
	forbiddenPG := ids.New(ids.Postgres)
	forbiddenKV := ids.New(ids.KeyValue)
	delKV := deleting(&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: deletingKV, Namespace: "default", Labels: map[string]string{core.LabelTenant: "tea-home"},
	}})
	forbPG := sampleDatabase(forbiddenPG)
	forbPG.Labels = map[string]string{core.LabelTenant: "tea-other"}
	forbKV := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: forbiddenKV, Namespace: "default", Labels: map[string]string{core.LabelTenant: "tea-other"},
	}}

	newAuthSvc := func() (*Service, *auditRows) {
		svc, _ := newMixedResourceService(delKV, forbPG, forbKV)
		rows := &auditRows{}
		svc.Base.Audit = rows
		svc.Base.Workspace = homeWorkspaceOnly{}
		svc.Base.Authz = workspaceDenyChecker{denyWorkspace: "tea-other"}
		svc.MaxSSEConns = 1
		return svc, rows
	}
	identCtx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-1", Method: core.MethodSession})
	const window = "startTime=2026-07-05T00:00:00Z&endTime=2026-07-05T01:00:00Z"

	for _, tc := range []struct {
		name       string
		badID      string
		wantStatus int
		wantErr    error
	}{
		{"missing postgres", missingPG, http.StatusNotFound, core.ErrNotFound},
		{"missing key value", missingKV, http.StatusNotFound, core.ErrNotFound},
		{"deleting key value", deletingKV, http.StatusNotFound, core.ErrNotFound},
		{"forbidden postgres", forbiddenPG, http.StatusForbidden, core.ErrForbidden},
		{"forbidden key value", forbiddenKV, http.StatusForbidden, core.ErrForbidden},
	} {
		for _, order := range [][]string{{"web", tc.badID}, {tc.badID, "web"}} {
			// 1. REST GET /v1/logs with statusCode=200 and type=request
			for _, filter := range []string{"statusCode=200", "type=request", "type=build", "build=dep-1"} {
				svc, rows := newAuthSvc()
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/v1/logs?resource="+order[0]+"&resource="+order[1]+"&"+filter+"&"+window, nil).WithContext(identCtx)
				mux.ServeHTTP(rec, req)
				if rec.Code != tc.wantStatus {
					t.Errorf("%s REST /v1/logs %v %s => %d %s, want %d", tc.name, order, filter, rec.Code, rec.Body.String(), tc.wantStatus)
				}
				if tc.wantStatus == http.StatusForbidden {
					if got := rows.verbs(); !slices.Contains(got, "denied "+AuditVerbQueryLogs) {
						t.Errorf("%s REST /v1/logs audit = %v, want denied %s", tc.name, got, AuditVerbQueryLogs)
					}
				}
			}

			// 2. REST GET /v1/logs/values with label=statusCode
			{
				svc, rows := newAuthSvc()
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/v1/logs/values?resource="+order[0]+"&resource="+order[1]+"&label=statusCode&"+window, nil).WithContext(identCtx)
				mux.ServeHTTP(rec, req)
				if rec.Code != tc.wantStatus {
					t.Errorf("%s REST /v1/logs/values %v => %d %s, want %d", tc.name, order, rec.Code, rec.Body.String(), tc.wantStatus)
				}
				if tc.wantStatus == http.StatusForbidden {
					if got := rows.verbs(); !slices.Contains(got, "denied "+AuditVerbLogLabelValues) {
						t.Errorf("%s REST /v1/logs/values audit = %v, want denied %s", tc.name, got, AuditVerbLogLabelValues)
					}
				}
			}

			// 3. REST GET /v1/logs/subscribe with statusCode=200
			{
				svc, rows := newAuthSvc()
				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?resource="+order[0]+"&resource="+order[1]+"&statusCode=200", nil).WithContext(identCtx)
				req.Header.Set("Accept", "text/event-stream")
				serveSubscribe(mux, rec, req)
				if rec.Code != tc.wantStatus {
					t.Errorf("%s REST /v1/logs/subscribe %v => %d %s, want %d", tc.name, order, rec.Code, rec.Body.String(), tc.wantStatus)
				}
				if svc.sseConns.Load() != 0 {
					t.Errorf("%s subscribe leaked slot: sseConns=%d", tc.name, svc.sseConns.Load())
				}
				if tc.wantStatus == http.StatusForbidden {
					if got := rows.verbs(); !slices.Contains(got, "denied "+AuditVerbFollowLogs) {
						t.Errorf("%s subscribe audit = %v, want denied %s", tc.name, got, AuditVerbFollowLogs)
					}
				}
			}
		}

		// 4. MCP list_logs & list_log_label_values
		svc, _ := newAuthSvc()
		cs := connectTestMCP(t, svc)
		res, err := cs.CallTool(identCtx, &mcp.CallToolParams{
			Name: "list_logs",
			Arguments: map[string]any{
				"resource":   []string{"web", tc.badID},
				"statusCode": []string{"200"},
				"startTime":  "2026-07-05T00:00:00Z",
				"endTime":    "2026-07-05T01:00:00Z",
			},
		})
		if err == nil && (res == nil || !res.IsError) {
			t.Errorf("%s MCP list_logs succeeded, want error (%v)", tc.name, tc.wantErr)
		}
		res, err = cs.CallTool(identCtx, &mcp.CallToolParams{
			Name: "list_log_label_values",
			Arguments: map[string]any{
				"resource":  []string{"web", tc.badID},
				"label":     "statusCode",
				"startTime": "2026-07-05T00:00:00Z",
				"endTime":   "2026-07-05T01:00:00Z",
			},
		})
		if err == nil && (res == nil || !res.IsError) {
			t.Errorf("%s MCP list_log_label_values succeeded, want error (%v)", tc.name, tc.wantErr)
		}
		_ = errors.Is(tc.wantErr, tc.wantErr)
	}
}

// w32/107: project-scoped and environment-scoped reads drop datastores when
// request, build, or non-app multi-type filters apply across REST, GraphQL,
// MCP, and live subscribe.
func TestProjectScopedQueriesFilterDatastoresByQuery(t *testing.T) {
	svc, _ := newMixedResourceService()
	cs := connectTestMCP(t, svc)
	const window = "startTime=2026-07-05T00:00:00Z&endTime=2026-07-05T01:00:00Z"

	for _, tc := range []struct {
		name      string
		query     string
		want      []string
		forbidden []string
	}{
		{
			name:      "project with statusCode=200 (no type)",
			query:     "project=prj-observed&statusCode=200",
			want:      []string{"GET /ok 200"},
			forbidden: []string{"pg checkpoint", "kv ready", "web app line"},
		},
		{
			name:      "project + environment with statusCode=200",
			query:     "project=prj-observed&environment=evm-1&statusCode=200",
			want:      []string{"GET /ok 200"},
			forbidden: []string{"pg checkpoint", "kv ready"},
		},
		{
			name:      "project with type=request&type=build leaks zero datastore lines",
			query:     "project=prj-observed&type=request&type=build",
			want:      []string{"GET /ok 200", "web build line", "site build line"},
			forbidden: []string{"pg checkpoint", "kv ready", "web app line"},
		},
		{
			name:      "project with build=dep-1 drops datastores",
			query:     "project=prj-observed&build=dep-1",
			want:      []string{"web build line"},
			forbidden: []string{"pg checkpoint", "kv ready"},
		},
		{
			name:      "project narrowed to mixed web + keyValue with statusCode=200",
			query:     "project=prj-observed&resource=web&resource=" + keyValueID + "&statusCode=200",
			want:      []string{"GET /ok 200"},
			forbidden: []string{"kv ready"},
		},
		{
			name:      "project narrowed to mixed web + keyValue with type=app",
			query:     "project=prj-observed&resource=web&resource=" + keyValueID + "&type=app",
			want:      []string{"web app line", "kv ready"},
			forbidden: []string{"GET /ok 200"},
		},
	} {
		rec := serveREST(svc, http.MethodGet, "/v1/logs?"+tc.query+"&"+window)
		if rec.Code != http.StatusOK {
			t.Fatalf("REST %s => %d %s", tc.name, rec.Code, rec.Body.String())
		}
		var got renderLogList
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		msgs := renderLogMessages(got)
		for _, w := range tc.want {
			if !slices.Contains(msgs, w) {
				t.Errorf("REST %s: missing %q in %v", tc.name, w, msgs)
			}
		}
		for _, f := range tc.forbidden {
			if slices.Contains(msgs, f) {
				t.Errorf("REST %s: leaked %q in %v", tc.name, f, msgs)
			}
		}
	}

	// Explicit single datastore narrowed under a project still refuses with 400.
	for _, q := range []string{
		"project=prj-observed&resource=" + keyValueID + "&statusCode=200",
		"project=prj-observed&resource=" + keyValueID + "&type=request",
	} {
		rec := serveREST(svc, http.MethodGet, "/v1/logs?"+q+"&"+window)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("control REST %s => %d %s, want 400", q, rec.Code, rec.Body.String())
		}
	}

	// Project-scoped live subscribe with statusCode=200 (both whole project and
	// project narrowed to mixed web + keyValue) streams web request logs without 400.
	svc.RequestPollInterval = 10 * time.Millisecond
	for _, query := range []url.Values{
		{"project": {"prj-observed"}, "statusCode": {"200"}, "startTime": {"2026-07-05T00:00:00Z"}},
		{"project": {"prj-observed"}, "resource": {"web", keyValueID}, "statusCode": {"200"}, "startTime": {"2026-07-05T00:00:00Z"}},
	} {
		rec := subscribeProject(t, svc, query, 200*time.Millisecond)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "GET /ok 200") || strings.Contains(rec.Body.String(), "kv ready") {
			t.Errorf("subscribe %s => %d %s", query.Encode(), rec.Code, rec.Body.String())
		}
	}

	// GraphQL project+environment with statusCode=200 excludes keyValueID,
	// and bare project view with statusCode=200 does not 400.
	list, errs := runProjectQuery(t, svc, context.Background(), `project:"prj-observed", environment:"evm-1", statusCode:["200"]`)
	if errs != nil {
		t.Fatalf("GraphQL project+env statusCode=200: %v", errs)
	}
	if got := lineResources(t, list); !slices.Contains(got, "web:GET /ok 200") {
		t.Errorf("GraphQL project+env lines = %v, want web:GET /ok 200", got)
	}
	if _, errs := runProjectQuery(t, svc, context.Background(), `project:"prj-observed", statusCode:["200"]`); errs != nil {
		t.Fatalf("GraphQL bare project statusCode=200 errored: %v", errs)
	}
	if vals, errs := runProjectValues(t, svc, context.Background(), `project:"prj-observed", environment:"evm-1", label:"level", statusCode:["200"]`); errs != nil || !slices.Equal(vals, []string{"error", "info"}) {
		t.Fatalf("GraphQL project+env values = %v, errs=%v", vals, errs)
	}

	// MCP project-scoped list_logs with statusCode=200 and type=[request, build].
	for _, args := range []map[string]any{
		{"project": "prj-observed", "statusCode": []string{"200"}, "startTime": "2026-07-05T00:00:00Z", "endTime": "2026-07-05T01:00:00Z"},
		{"project": "prj-observed", "type": []string{"request", "build"}, "startTime": "2026-07-05T00:00:00Z", "endTime": "2026-07-05T01:00:00Z"},
	} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs", Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("MCP project %v: err=%v res=%+v", args, err, res)
		}
		var got renderLogList
		rendertest.DecodeMCPText(t, res, &got)
		msgs := renderLogMessages(got)
		if !slices.Contains(msgs, "GET /ok 200") || slices.Contains(msgs, "pg checkpoint") || slices.Contains(msgs, "kv ready") {
			t.Errorf("MCP project %v = %v", args, msgs)
		}
	}
}
