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
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/104: synthetic deploy progress and initial-deploy-hook lines carry no
// level, statusCode, method, host, or path labels, so any positive filter on
// those labels must return zero synthetic lines across REST, GraphQL, and MCP
// while negated filters (`!=` / `!~`) keep them to match Loki's selectors.

func progressFilterFixture(t *testing.T) (*Service, time.Time, time.Time) {
	t.Helper()
	base := time.Date(2026, 11, 23, 22, 50, 0, 0, time.UTC)
	created := base.Add(10 * time.Second)
	started := base.Add(15 * time.Second)
	finished := base.Add(40 * time.Second)

	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	app.Spec.InitialDeployHook = "npm run seed"
	app.Status.InitialDeployHook = &appv1alpha1.PreDeployStatus{
		Generation: 4,
		Status:     appv1alpha1.PreDeploySucceeded,
		StartedAt:  base.Add(42 * time.Second).Format(time.RFC3339Nano),
		FinishedAt: base.Add(45 * time.Second).Format(time.RFC3339Nano),
	}

	reqRecord, _ := json.Marshal(map[string]any{
		"RequestHost":     "bogus.example",
		"RequestPath":     "/never-hit",
		"request_Rndr-Id": "req-404",
	})
	f := newFrontendLoki(t,
		fakeStream{
			labels: map[string]string{"namespace": "default", "app": "web", "type": LogTypeBuild, "pod": buildPod(4, "aaaaa")},
			stamps: []int64{base.Add(20 * time.Second).UnixNano()},
			text:   func(int64) string { return "real build output" },
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "app": "web", "container": "app", "type": LogTypeApp, "pod": "web-1", "level": "critical"},
			stamps: []int64{base.Add(25 * time.Second).UnixNano()},
			text:   func(int64) string { return "app critical boom" },
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "app": "web", "type": LogTypeRequest, "method": "POST", "status": "404", "level": "warning"},
			stamps: []int64{base.Add(30 * time.Second).UnixNano()},
			text:   func(int64) string { return string(reqRecord) },
		},
	)

	row := DeployProgress{
		ID:         "dep-filter",
		Generation: 4,
		Status:     "live",
		Built:      true,
		Commit:     "abc1234def5678",
		CreatedAt:  created,
		StartedAt:  started,
		FinishedAt: finished,
		LiveURLs:   []string{"https://web.onbex.co"},
	}
	svc := newService(nil, app, podFor("web", "web-1"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{row}, nil
	}
	return svc, base, base.Add(time.Minute)
}

func TestSynthesizeProgressExcludesPositiveLevelAndRequestFilters(t *testing.T) {
	svc, start, end := progressFilterFixture(t)
	read := func(t *testing.T, q LogQuery) []LogEntry {
		t.Helper()
		q.App, q.Since, q.End, q.Limit, q.Direction = "web", start, end, 100, DirectionForward
		entries, err := svc.QueryLogs(context.Background(), q)
		if err != nil {
			t.Fatalf("QueryLogs(%+v): %v", q, err)
		}
		return entries
	}
	messages := func(entries []LogEntry) []string {
		out := make([]string, len(entries))
		for i, e := range entries {
			out[i] = e.Message
		}
		return out
	}

	// Positive level and HTTP request filters exclude every synthetic build
	// progress and hook line on type=build and on build-filtered reads.
	for _, tc := range []struct {
		name string
		q    LogQuery
	}{
		{"type=build level=critical", LogQuery{Types: []string{LogTypeBuild}, Level: []string{"critical"}}},
		{"type=build level=error", LogQuery{Types: []string{LogTypeBuild}, Level: []string{"error"}}},
		{"type=build level=unknown", LogQuery{Types: []string{LogTypeBuild}, Level: []string{"unknown"}}},
		{"type=build statusCode=404", LogQuery{Types: []string{LogTypeBuild}, StatusCode: []string{"404"}}},
		{"type=build statusCode=4xx", LogQuery{Types: []string{LogTypeBuild}, StatusCode: []string{"4xx"}}},
		{"type=build method=POST", LogQuery{Types: []string{LogTypeBuild}, Method: []string{"POST"}}},
		{"type=build host=bogus.example", LogQuery{Types: []string{LogTypeBuild}, Host: []string{"bogus.example"}}},
		{"type=build path=/never-hit", LogQuery{Types: []string{LogTypeBuild}, Path: []string{"/never-hit"}}},
		{"build=dep-filter level=error", LogQuery{Build: []string{"dep-filter"}, Level: []string{"error"}}},
		{"build=dep-filter statusCode=404", LogQuery{Build: []string{"dep-filter"}, StatusCode: []string{"404"}}},
		{"build=dep-filter method=POST", LogQuery{Build: []string{"dep-filter"}, Method: []string{"POST"}}},
		{"build=dep-filter host=bogus.example", LogQuery{Build: []string{"dep-filter"}, Host: []string{"bogus.example"}}},
		{"build=dep-filter path=/never-hit", LogQuery{Build: []string{"dep-filter"}, Path: []string{"/never-hit"}}},
		{"negated level + positive statusCode", LogQuery{Types: []string{LogTypeBuild}, Level: []string{"error"}, StatusCode: []string{"404"}, negated: []string{LabelLevel}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := read(t, tc.q); len(got) != 0 {
				t.Fatalf("got %d entries %q, want 0", len(got), messages(got))
			}
		})
	}

	// Mixed types: matching app/request lines still return, with zero synthetic
	// build progress or hook lines mixed in.
	for _, tc := range []struct {
		name      string
		q         LogQuery
		wantCount int
		wantType  string
		wantSub   string
	}{
		{"build+app level=critical", LogQuery{Types: []string{LogTypeBuild, LogTypeApp}, Level: []string{"critical"}}, 1, LogTypeApp, "app critical boom"},
		{"build+app level=alert (no match)", LogQuery{Types: []string{LogTypeBuild, LogTypeApp}, Level: []string{"alert"}}, 0, "", ""},
		{"build+request statusCode=404", LogQuery{Types: []string{LogTypeBuild, LogTypeRequest}, StatusCode: []string{"404"}}, 1, LogTypeRequest, `requestID="req-404"`},
		{"build+request method=POST", LogQuery{Types: []string{LogTypeBuild, LogTypeRequest}, Method: []string{"POST"}}, 1, LogTypeRequest, `requestID="req-404"`},
		{"build+request host=bogus.example", LogQuery{Types: []string{LogTypeBuild, LogTypeRequest}, Host: []string{"bogus.example"}}, 1, LogTypeRequest, `requestID="req-404"`},
		{"build+request path=/never-hit", LogQuery{Types: []string{LogTypeBuild, LogTypeRequest}, Path: []string{"/never-hit"}}, 1, LogTypeRequest, `requestID="req-404"`},
		{"build+request statusCode=503 (no match)", LogQuery{Types: []string{LogTypeBuild, LogTypeRequest}, StatusCode: []string{"503"}}, 0, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := read(t, tc.q)
			if len(got) != tc.wantCount {
				t.Fatalf("got %d entries %q, want %d", len(got), messages(got), tc.wantCount)
			}
			for _, e := range got {
				if e.Labels[LabelType] != tc.wantType || !strings.Contains(e.Message, tc.wantSub) {
					t.Errorf("entry = type=%q msg=%q, want type=%q containing %q", e.Labels[LabelType], e.Message, tc.wantType, tc.wantSub)
				}
			}
		})
	}

	// Negated filters keep unlabeled synthetic build progress and hook lines,
	// matching Loki's `!=` / `!~` evaluation on unlabeled build streams.
	for _, tc := range []struct {
		name string
		q    LogQuery
	}{
		{"negated level", LogQuery{Types: []string{LogTypeBuild}, Level: []string{"error"}, negated: []string{LabelLevel}}},
		{"negated statusCode", LogQuery{Types: []string{LogTypeBuild}, StatusCode: []string{"200"}, negated: []string{LabelStatusCode}}},
		{"negated method", LogQuery{Types: []string{LogTypeBuild}, Method: []string{"GET"}, negated: []string{LabelMethod}}},
		{"negated host", LogQuery{Types: []string{LogTypeBuild}, Host: []string{"other.example"}, negated: []string{LabelHost}}},
		{"negated path", LogQuery{Types: []string{LogTypeBuild}, Path: []string{"/healthz"}, negated: []string{LabelPath}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msgs := messages(read(t, tc.q))
			for _, want := range []string{"==> Build queued", rendersLive, hookLineRunning, hookLineSucceeded} {
				if !slices.Contains(msgs, want) {
					t.Errorf("negated read missing %q; got %q", want, msgs)
				}
			}
		})
	}

	// Controls: text, deploy, build, and unfiltered type=build still synthesize
	// and filter progress and hook lines normally.
	t.Run("controls keep progress and hook lines", func(t *testing.T) {
		unfiltered := messages(read(t, LogQuery{Types: []string{LogTypeBuild}}))
		for _, want := range []string{"==> Build queued", "real build output", rendersLive, hookLineRunning, hookLineSucceeded} {
			if !slices.Contains(unfiltered, want) {
				t.Errorf("unfiltered type=build missing %q; got %q", want, unfiltered)
			}
		}
		textFiltered := messages(read(t, LogQuery{Types: []string{LogTypeBuild}, Search: []string{"Your service is live"}}))
		if !slices.Equal(textFiltered, []string{rendersLive}) {
			t.Errorf("text-filtered type=build = %q, want [%q]", textFiltered, rendersLive)
		}
		deployFiltered := messages(read(t, LogQuery{Types: []string{LogTypeBuild}, Deploy: "dep-filter"}))
		if !slices.Contains(deployFiltered, rendersLive) || !slices.Contains(deployFiltered, hookLineSucceeded) {
			t.Errorf("deploy-filtered type=build = %q, want progress and hook lines", deployFiltered)
		}
		buildFiltered := messages(read(t, LogQuery{Build: []string{"dep-filter"}}))
		if !slices.Contains(buildFiltered, rendersLive) || !slices.Contains(buildFiltered, "real build output") {
			t.Errorf("build-filtered read = %q, want progress and build pod lines", buildFiltered)
		}
	})
}

func TestBuildFilterExcludesProgressLinesAcrossRESTGraphQLAndMCP(t *testing.T) {
	svc, start, end := progressFilterFixture(t)
	startStr, endStr := start.Format(time.RFC3339), end.Format(time.RFC3339)

	// REST GET /v1/logs
	restURL := func(extra string) string {
		return "/v1/logs?resource=web&startTime=" + startStr + "&endTime=" + endStr + "&limit=20&" + extra
	}
	for _, extra := range []string{
		"type=build&statusCode=404",
		"type=build&method=POST",
		"type=build&host=bogus.example",
		"type=build&path=/never-hit",
		"type=build&level=critical",
		"type=build&level=unknown",
		"build=dep-filter&level=error",
		"build=dep-filter&statusCode=404",
	} {
		env := decodeLogList(t, serveREST(svc, http.MethodGet, restURL(extra)))
		if len(env.Logs) != 0 || env.HasMore {
			t.Errorf("REST %s = %d logs (hasMore=%v), want 0 (hasMore=false)", extra, len(env.Logs), env.HasMore)
		}
	}
	if mixedApp := decodeLogList(t, serveREST(svc, http.MethodGet, restURL("type=build&type=app&level=critical"))); len(mixedApp.Logs) != 1 || mixedApp.Logs[0].Message != "app critical boom" {
		t.Errorf("REST type=build&type=app&level=critical = %+v, want only app critical boom", mixedApp.Logs)
	}
	if mixedReq := decodeLogList(t, serveREST(svc, http.MethodGet, restURL("type=build&type=request&statusCode=404"))); len(mixedReq.Logs) != 1 || !strings.Contains(mixedReq.Logs[0].Message, `requestID="req-404"`) {
		t.Errorf("REST type=build&type=request&statusCode=404 = %+v, want only req-404", mixedReq.Logs)
	}
	if ctrl := decodeLogList(t, serveREST(svc, http.MethodGet, restURL("type=build"))); len(ctrl.Logs) == 0 || !slices.ContainsFunc(ctrl.Logs, func(l renderLog) bool { return l.Message == rendersLive }) {
		t.Errorf("REST control type=build = %+v, want progress lines including live", ctrl.Logs)
	}

	// GraphQL logs
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	gqlLogs := func(args string) ([]any, bool) {
		t.Helper()
		data := runQuery(t, schema, `{ logs(resource:"web", startTime:"`+startStr+`", endTime:"`+endStr+`", limit:20, `+args+`) { hasMore logs { message type } } }`)
		page := data["logs"].(map[string]any)
		return page["logs"].([]any), page["hasMore"].(bool)
	}
	for _, args := range []string{
		`type:"build", statusCode:["404"]`,
		`type:"build", method:["POST"]`,
		`type:"build", host:["bogus.example"]`,
		`type:"build", path:["/never-hit"]`,
		`type:"build", level:["critical"]`,
		`type:"build", level:["unknown"]`,
		`build:["dep-filter"], statusCode:["404"]`,
	} {
		if rows, more := gqlLogs(args); len(rows) != 0 || more {
			t.Errorf("GraphQL %s = %d logs (hasMore=%v), want 0 (hasMore=false)", args, len(rows), more)
		}
	}
	if rows, _ := gqlLogs(`type:"build"`); len(rows) == 0 {
		t.Errorf("GraphQL control type=build returned 0 logs")
	}

	// MCP list_logs
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(mcpSrv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpSrv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	callMCP := func(extra map[string]any) renderLogList {
		t.Helper()
		args := map[string]any{
			"resource":  []string{"web"},
			"startTime": startStr,
			"endTime":   endStr,
			"limit":     20,
		}
		for k, v := range extra {
			args[k] = v
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("list_logs(%v): %v %+v", extra, err, res)
		}
		var out renderLogList
		rendertest.DecodeMCPText(t, res, &out)
		return out
	}
	for _, extra := range []map[string]any{
		{"type": []string{LogTypeBuild}, "statusCode": []string{"404"}},
		{"type": []string{LogTypeBuild}, "method": []string{"POST"}},
		{"type": []string{LogTypeBuild}, "host": []string{"bogus.example"}},
		{"type": []string{LogTypeBuild}, "path": []string{"/never-hit"}},
		{"type": []string{LogTypeBuild}, "level": []string{"critical"}},
		{"build": []string{"dep-filter"}, "level": []string{"error"}},
	} {
		if got := callMCP(extra); len(got.Logs) != 0 || got.HasMore {
			t.Errorf("MCP %v = %d logs (hasMore=%v), want 0 (hasMore=false)", extra, len(got.Logs), got.HasMore)
		}
	}
	if got := callMCP(map[string]any{"type": []string{LogTypeBuild, LogTypeApp}, "level": []string{"critical"}}); len(got.Logs) != 1 || got.Logs[0].Message != "app critical boom" {
		t.Errorf("MCP build+app level=critical = %+v, want only app critical boom", got.Logs)
	}
	if got := callMCP(map[string]any{"type": []string{LogTypeBuild, LogTypeRequest}, "path": []string{"/never-hit"}}); len(got.Logs) != 1 || !strings.Contains(got.Logs[0].Message, `requestID="req-404"`) {
		t.Errorf("MCP build+request path=/never-hit = %+v, want only req-404", got.Logs)
	}
	if ctrl := callMCP(map[string]any{"type": []string{LogTypeBuild}}); len(ctrl.Logs) == 0 || !slices.ContainsFunc(ctrl.Logs, func(l renderLog) bool { return l.Message == rendersLive }) {
		t.Errorf("MCP control type=build = %+v, want progress lines including live", ctrl.Logs)
	}
}
