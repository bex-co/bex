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
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/105: synthetic deploy progress and initial-deploy-hook lines emit a
// projected instance ID derived from the deploy ID (`dep-...`), so
// translateInstanceFilter must include deploy IDs as reverse-lookup candidates
// on build reads, LogLabelValues(instance) includes in-window deploy IDs on
// build reads for consistency, and live tails honor them.

type progressInstanceFixture struct {
	svc           *Service
	start         time.Time
	end           time.Time
	buildPodName  string
	dep1          string
	dep2          string
	depOutOfRange string
	progressCalls *atomic.Int32
}

func newProgressInstanceFixture(t *testing.T) progressInstanceFixture {
	t.Helper()
	base := time.Date(2026, 11, 23, 22, 50, 0, 0, time.UTC)
	buildPodName := buildPod(4, "aaaaa")

	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	app.Spec.InitialDeployHook = "npm run seed"
	app.Status.InitialDeployHook = &appv1alpha1.PreDeployStatus{
		Generation: 4,
		Status:     appv1alpha1.PreDeploySucceeded,
		StartedAt:  base.Add(22 * time.Second).Format(time.RFC3339Nano),
		FinishedAt: base.Add(24 * time.Second).Format(time.RFC3339Nano),
	}

	f := newFrontendLoki(t,
		fakeStream{
			labels: map[string]string{"namespace": "default", "app": "web", "type": LogTypeBuild, "pod": buildPodName},
			stamps: []int64{base.Add(12 * time.Second).UnixNano()},
			text:   func(int64) string { return "real buildkit output" },
		},
		fakeStream{
			labels: map[string]string{"namespace": "default", "app": "web", "container": "app", "type": LogTypeApp, "pod": "web-1", "level": "info"},
			stamps: []int64{base.Add(30 * time.Second).UnixNano()},
			text:   func(int64) string { return "app listening on 10000" },
		},
	)

	rows := []DeployProgress{
		{
			ID:         "dep-second",
			Generation: 5,
			Status:     "live",
			Image:      "docker.io/library/nginx:1.27-alpine",
			CreatedAt:  base.Add(40 * time.Second),
			StartedAt:  base.Add(42 * time.Second),
			FinishedAt: base.Add(50 * time.Second),
			LiveURLs:   []string{"https://web.onbex.co"},
		},
		{
			ID:         "dep-first",
			Generation: 4,
			Status:     "deactivated",
			Built:      true,
			Commit:     "abc1234def5678",
			CreatedAt:  base.Add(5 * time.Second),
			StartedAt:  base.Add(10 * time.Second),
			FinishedAt: base.Add(20 * time.Second),
			LiveURLs:   []string{"https://web.onbex.co"},
		},
		{
			ID:         "dep-ancient",
			Generation: 1,
			Status:     "deactivated",
			Image:      "docker.io/library/nginx:1.26-alpine",
			CreatedAt:  base.Add(-2 * time.Hour),
			StartedAt:  base.Add(-2*time.Hour + 5*time.Second),
			FinishedAt: base.Add(-2*time.Hour + 15*time.Second),
		},
	}

	svc := newService(nil, app, podFor("web", "web-1"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	svc.LabelValues = func(_ context.Context, _ string, label string, q LogQuery) ([]string, error) {
		if label != LabelInstance {
			return nil, nil
		}
		if slices.Contains(q.Types, LogTypeBuild) {
			return []string{buildPodName}, nil
		}
		return []string{"web-1"}, nil
	}
	var calls atomic.Int32
	svc.DeployProgress = func(_ context.Context, _ string, end time.Time) ([]DeployProgress, error) {
		calls.Add(1)
		if end.IsZero() {
			return slices.Clone(rows), nil
		}
		var out []DeployProgress
		for _, r := range rows {
			if !r.CreatedAt.After(end) {
				out = append(out, r)
			}
		}
		return out, nil
	}
	return progressInstanceFixture{
		svc:           svc,
		start:         base,
		end:           base.Add(time.Minute),
		buildPodName:  buildPodName,
		dep1:          "dep-first",
		dep2:          "dep-second",
		depOutOfRange: "dep-ancient",
		progressCalls: &calls,
	}
}

func TestTranslateInstanceFilterResolvesDeployProgressIDs(t *testing.T) {
	fx := newProgressInstanceFixture(t)
	read := func(t *testing.T, q LogQuery) []LogEntry {
		t.Helper()
		q.App, q.Since, q.End, q.Limit, q.Direction = "web", fx.start, fx.end, 100, DirectionForward
		entries, err := fx.svc.QueryLogs(context.Background(), q)
		if err != nil {
			t.Fatalf("QueryLogs(%+v): %v", q, err)
		}
		return entries
	}
	msgs := func(entries []LogEntry) []string {
		out := make([]string, len(entries))
		for i, e := range entries {
			out[i] = e.Message
		}
		return out
	}

	// Step 1: read type=build and extract the emitted instance IDs for both
	// deploys' synthetic progress lines (exact CLI round-trip repro).
	allBuild := read(t, LogQuery{Types: []string{LogTypeBuild}})
	var emittedDep1, emittedDep2 string
	for _, e := range allBuild {
		if e.Labels[LabelBuild] == fx.dep1 && e.Message == rendersLive {
			emittedDep1 = e.Labels[LabelInstance]
		}
		if e.Labels[LabelBuild] == fx.dep2 && e.Message == rendersLive {
			emittedDep2 = e.Labels[LabelInstance]
		}
	}
	wantDep1 := projectLogInstance("web", fx.dep1)
	wantDep2 := projectLogInstance("web", fx.dep2)
	if emittedDep1 != wantDep1 || emittedDep2 != wantDep2 {
		t.Fatalf("emitted deploy instances = (%q, %q), want (%q, %q)", emittedDep1, emittedDep2, wantDep1, wantDep2)
	}

	// Step 2: filtering type=build by dep1's emitted instance ID returns only
	// dep1's progress and hook lines — excluding dep2 and the BuildKit pod.
	t.Run("positive deploy instance filter", func(t *testing.T) {
		got := read(t, LogQuery{Types: []string{LogTypeBuild}, Instance: []string{emittedDep1}})
		if len(got) == 0 {
			t.Fatalf("expected dep1 progress lines for instance=%q, got 0", emittedDep1)
		}
		for _, e := range got {
			if e.Labels[LabelInstance] != emittedDep1 || e.Labels[LabelBuild] != fx.dep1 {
				t.Errorf("unexpected entry %q labels=%v; want only dep1 (%s)", e.Message, e.Labels, emittedDep1)
			}
		}
		gotMsgs := msgs(got)
		for _, want := range []string{"==> Build queued", rendersLive, hookLineRunning, hookLineSucceeded} {
			if !slices.Contains(gotMsgs, want) {
				t.Errorf("missing %q in %q", want, gotMsgs)
			}
		}
	})

	// Combined build + instance filters intersect accurately.
	t.Run("build filter with deploy instance", func(t *testing.T) {
		same := read(t, LogQuery{Build: []string{fx.dep1}, Instance: []string{emittedDep1}})
		if len(same) == 0 || !slices.Contains(msgs(same), rendersLive) {
			t.Fatalf("build=dep1 instance=dep1 = %q, want dep1 progress lines", msgs(same))
		}
		mismatch := read(t, LogQuery{Build: []string{fx.dep2}, Instance: []string{emittedDep1}})
		if len(mismatch) != 0 {
			t.Fatalf("build=dep2 instance=dep1 = %q, want 0 lines", msgs(mismatch))
		}
	})

	// Mixed types: build+app with deploy instance returns only dep1 progress
	// lines; with App pod instance returns only App lines.
	t.Run("mixed build and app types", func(t *testing.T) {
		depOnly := read(t, LogQuery{Types: []string{LogTypeBuild, LogTypeApp}, Instance: []string{emittedDep1}})
		if len(depOnly) == 0 || !slices.Contains(msgs(depOnly), rendersLive) {
			t.Fatalf("mixed build+app instance=dep1 = %q, want dep1 progress lines", msgs(depOnly))
		}
		for _, e := range depOnly {
			if e.Labels[LabelType] != LogTypeBuild || e.Labels[LabelInstance] != emittedDep1 {
				t.Errorf("mixed deploy query leaked %q labels=%v", e.Message, e.Labels)
			}
		}
		appInst := projectLogInstance("web", "web-1")
		appOnly := read(t, LogQuery{Types: []string{LogTypeBuild, LogTypeApp}, Instance: []string{appInst}})
		if len(appOnly) != 1 || appOnly[0].Message != "app listening on 10000" {
			t.Fatalf("mixed build+app instance=web-1 = %+v, want only app line", appOnly)
		}
	})

	// Negated instance filter excludes dep1's progress & hook lines while
	// preserving dep2's progress lines and real BuildKit pod lines.
	t.Run("negated deploy instance filter", func(t *testing.T) {
		got := read(t, LogQuery{
			Types:    []string{LogTypeBuild},
			Instance: []string{emittedDep1},
			negated:  []string{LabelInstance},
		})
		if len(got) == 0 {
			t.Fatal("negated deploy instance returned 0 lines")
		}
		var sawDep2, sawBuildPod bool
		for _, e := range got {
			if e.Labels[LabelInstance] == emittedDep1 || e.Labels[LabelBuild] == fx.dep1 && e.Message == hookLineSucceeded {
				t.Errorf("negated instance=%s leaked dep1 entry %q labels=%v", emittedDep1, e.Message, e.Labels)
			}
			if e.Labels[LabelInstance] == emittedDep2 && e.Message == rendersLive {
				sawDep2 = true
			}
			if e.Labels[LabelInstance] == projectLogInstance("web", fx.buildPodName) && e.Message == "real buildkit output" {
				sawBuildPod = true
			}
		}
		if !sawDep2 || !sawBuildPod {
			t.Errorf("negated instance=%s missing dep2 (%v) or buildkit pod (%v); got %q", emittedDep1, sawDep2, sawBuildPod, msgs(got))
		}
	})

	// Controls: unknown instance, real BuildKit pod, App pod (no DeployProgress
	// call), and best-effort degradation when DeployProgress errors.
	t.Run("controls", func(t *testing.T) {
		unknown := projectLogInstance("web", "nonexistent")
		if got := read(t, LogQuery{Types: []string{LogTypeBuild}, Instance: []string{unknown}}); len(got) != 0 {
			t.Errorf("unknown instance = %q, want 0 lines", msgs(got))
		}

		buildPodInst := projectLogInstance("web", fx.buildPodName)
		buildPodOnly := read(t, LogQuery{Types: []string{LogTypeBuild}, Instance: []string{buildPodInst}})
		if len(buildPodOnly) != 1 || buildPodOnly[0].Message != "real buildkit output" {
			t.Errorf("buildkit pod instance = %+v, want only real buildkit output", buildPodOnly)
		}

		appInst := projectLogInstance("web", "web-1")
		before := fx.progressCalls.Load()
		appOnly := read(t, LogQuery{Types: []string{LogTypeApp}, Instance: []string{appInst}})
		untypedApp := read(t, LogQuery{Instance: []string{appInst}})
		if len(appOnly) != 1 || len(untypedApp) != 1 {
			t.Fatalf("app instance reads = (%d, %d), want (1, 1)", len(appOnly), len(untypedApp))
		}
		if after := fx.progressCalls.Load(); after != before {
			t.Errorf("app/untyped instance reads called DeployProgress %d times, want 0", after-before)
		}

		// Best-effort: a DeployProgress error must not break real build-pod filtering.
		orig := fx.svc.DeployProgress
		fx.svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
			return nil, errors.New("db temporarily unavailable")
		}
		t.Cleanup(func() { fx.svc.DeployProgress = orig })
		degraded := read(t, LogQuery{Types: []string{LogTypeBuild}, Instance: []string{buildPodInst}})
		if len(degraded) != 1 || degraded[0].Message != "real buildkit output" {
			t.Errorf("degraded DeployProgress build-pod read = %+v, want real buildkit output", degraded)
		}
	})
}

func TestDeployProgressInstanceFilterAcrossRESTGraphQLAndMCP(t *testing.T) {
	fx := newProgressInstanceFixture(t)
	startStr, endStr := fx.start.Format(time.RFC3339), fx.end.Format(time.RFC3339)

	// Extract dep-first's emitted instance ID via REST first (mirrors CLI).
	unfiltered := decodeLogList(t, serveREST(fx.svc, http.MethodGet,
		"/v1/logs?resource=web&type=build&startTime="+startStr+"&endTime="+endStr+"&limit=50"))
	var emittedDep1 string
	for _, l := range unfiltered.Logs {
		if l.Message != rendersLive {
			continue
		}
		var buildVal, instVal string
		for _, lb := range l.Labels {
			if lb.Name == LabelBuild {
				buildVal = lb.Value
			}
			if lb.Name == LabelInstance {
				instVal = lb.Value
			}
		}
		if buildVal == fx.dep1 {
			emittedDep1 = instVal
			break
		}
	}
	if emittedDep1 == "" {
		t.Fatalf("REST unfiltered type=build did not emit live line for %s: %+v", fx.dep1, unfiltered.Logs)
	}
	unknown := projectLogInstance("web", "nonexistent")

	// REST GET /v1/logs
	t.Run("REST", func(t *testing.T) {
		got := decodeLogList(t, serveREST(fx.svc, http.MethodGet,
			"/v1/logs?resource=web&type=build&instance="+emittedDep1+"&startTime="+startStr+"&endTime="+endStr+"&limit=50"))
		if len(got.Logs) == 0 || !slices.ContainsFunc(got.Logs, func(l renderLog) bool { return l.Message == rendersLive }) {
			t.Fatalf("REST instance=%s = %+v, want dep1 progress lines", emittedDep1, got.Logs)
		}
		for _, l := range got.Logs {
			if !slices.ContainsFunc(l.Labels, func(lb renderLabel) bool { return lb.Name == LabelInstance && lb.Value == emittedDep1 }) {
				t.Errorf("REST entry %q labels=%+v missing instance=%s", l.Message, l.Labels, emittedDep1)
			}
		}
		empty := decodeLogList(t, serveREST(fx.svc, http.MethodGet,
			"/v1/logs?resource=web&type=build&instance="+unknown+"&startTime="+startStr+"&endTime="+endStr+"&limit=50"))
		if len(empty.Logs) != 0 || empty.HasMore {
			t.Errorf("REST unknown instance = %d logs (hasMore=%v), want 0", len(empty.Logs), empty.HasMore)
		}
	})

	// GraphQL logs
	t.Run("GraphQL", func(t *testing.T) {
		schema, err := gqlSchema(fx.svc)
		if err != nil {
			t.Fatalf("schema: %v", err)
		}
		data := runQuery(t, schema, `{ logs(resource:"web", type:"build", instance:["`+emittedDep1+`"], startTime:"`+startStr+`", endTime:"`+endStr+`", limit:50) { hasMore logs { message instance build } } }`)
		page := data["logs"].(map[string]any)
		rows := page["logs"].([]any)
		if len(rows) == 0 {
			t.Fatalf("GraphQL instance=%s returned 0 logs", emittedDep1)
		}
		sawLive := false
		for _, raw := range rows {
			row := raw.(map[string]any)
			if row["instance"] != emittedDep1 || row["build"] != fx.dep1 {
				t.Errorf("GraphQL row %+v want instance=%s build=%s", row, emittedDep1, fx.dep1)
			}
			if row["message"] == rendersLive {
				sawLive = true
			}
		}
		if !sawLive {
			t.Errorf("GraphQL instance=%s missing live line: %+v", emittedDep1, rows)
		}
		emptyData := runQuery(t, schema, `{ logs(resource:"web", type:"build", instance:["`+unknown+`"], startTime:"`+startStr+`", endTime:"`+endStr+`", limit:50) { hasMore logs { message } } }`)
		emptyPage := emptyData["logs"].(map[string]any)
		if len(emptyPage["logs"].([]any)) != 0 || emptyPage["hasMore"].(bool) {
			t.Errorf("GraphQL unknown instance = %+v, want empty", emptyPage)
		}
	})

	// MCP list_logs
	t.Run("MCP", func(t *testing.T) {
		mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		fx.svc.RegisterMCP(mcpSrv)
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
		call := func(inst string) renderLogList {
			t.Helper()
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{
				Name: "list_logs",
				Arguments: map[string]any{
					"resource":  []string{"web"},
					"type":      []string{LogTypeBuild},
					"instance":  []string{inst},
					"startTime": startStr,
					"endTime":   endStr,
					"limit":     50,
				},
			})
			if err != nil || res.IsError {
				t.Fatalf("list_logs(%s): %v %+v", inst, err, res)
			}
			var out renderLogList
			rendertest.DecodeMCPText(t, res, &out)
			return out
		}
		got := call(emittedDep1)
		if len(got.Logs) == 0 || !slices.ContainsFunc(got.Logs, func(l renderLog) bool { return l.Message == rendersLive }) {
			t.Fatalf("MCP instance=%s = %+v, want dep1 progress lines", emittedDep1, got.Logs)
		}
		for _, l := range got.Logs {
			if !slices.ContainsFunc(l.Labels, func(lb renderLabel) bool { return lb.Name == LabelInstance && lb.Value == emittedDep1 }) {
				t.Errorf("MCP entry %q labels=%+v missing instance=%s", l.Message, l.Labels, emittedDep1)
			}
		}
		if empty := call(unknown); len(empty.Logs) != 0 || empty.HasMore {
			t.Errorf("MCP unknown instance = %d logs (hasMore=%v), want 0", len(empty.Logs), empty.HasMore)
		}
	})
}

func TestLiveTailFollowsDeployProgressInstance(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	opened := time.Now().UTC().Truncate(time.Second)
	soon := func(d time.Duration) string { return opened.Add(d).Format(time.RFC3339Nano) }
	depInst := projectLogInstance("web", "dep-live")
	appInst := projectLogInstance("web", "web-1")

	lines := map[string][]string{
		"web-1":           {soon(2*time.Second) + " app stdout line"},
		"bld-web-gen-2-a": {soon(3*time.Second) + " buildkit stdout line"},
	}
	svc := newService(lines, app, podFor("web", "web-1"))
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond
	svc.TailRelistInterval = 20 * time.Millisecond
	var mu sync.Mutex
	row := DeployProgress{
		ID:         "dep-live",
		Generation: 2,
		Status:     "live",
		Built:      true,
		Commit:     "aaaaaaa1111",
		CreatedAt:  opened.Add(time.Second),
		StartedAt:  opened.Add(2 * time.Second),
		FinishedAt: opened.Add(4 * time.Second),
		LiveURLs:   []string{"https://web.onbex.co"},
	}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		mu.Lock()
		defer mu.Unlock()
		return []DeployProgress{row}, nil
	}
	srv := revalidationTestServer(svc)
	defer srv.Close()

	dialRead := func(t *testing.T, query, wantMsg string) []renderLog {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?"+query, nil)
		if err != nil {
			t.Fatalf("WS dial %s: %v", query, err)
		}
		defer conn.Close()
		var got []renderLog
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("waiting for %q on %s after %d lines: %v", wantMsg, query, len(got), err)
			}
			var line renderLog
			if err := json.Unmarshal(payload, &line); err != nil {
				t.Fatalf("frame %s: %v", payload, err)
			}
			got = append(got, line)
			if line.Message == wantMsg {
				return got
			}
		}
	}

	startParam := opened.Format(time.RFC3339)

	// 1. type=build&instance=<projected-dep-id> streams the deploy's progress lines.
	t.Run("type=build deploy instance", func(t *testing.T) {
		got := dialRead(t, "resource=web&type=build&instance="+depInst+"&startTime="+startParam, rendersLive)
		for _, l := range got {
			if !slices.ContainsFunc(l.Labels, func(lb renderLabel) bool { return lb.Name == LabelInstance && lb.Value == depInst }) {
				t.Errorf("build tail leaked entry %q labels=%+v; want instance=%s", l.Message, l.Labels, depInst)
			}
		}
	})

	// 2. Type-less WebSocket tail (followPodsAndBuilds) with deploy instance
	// streams the deploy's progress lines and excludes App pod output.
	t.Run("typeless tail with deploy instance", func(t *testing.T) {
		got := dialRead(t, "resource=web&instance="+depInst+"&startTime="+startParam, rendersLive)
		for _, l := range got {
			if strings.Contains(l.Message, "app stdout line") {
				t.Errorf("typeless deploy-instance tail leaked app line: %+v", l)
			}
		}
	})

	// 3. Control: type-less WebSocket tail with App pod instance streams App
	// pod output and skips deploy progress lines.
	t.Run("typeless tail with app pod instance", func(t *testing.T) {
		got := dialRead(t, "resource=web&instance="+appInst+"&startTime="+startParam, "app stdout line")
		for _, l := range got {
			if strings.HasPrefix(l.Message, "==>") || !slices.ContainsFunc(l.Labels, func(lb renderLabel) bool { return lb.Name == LabelInstance && lb.Value == appInst }) {
				t.Errorf("app-pod tail leaked non-app entry %q labels=%+v", l.Message, l.Labels)
			}
		}
	})
}

// LogLabelValues(instance) for type=build (and build-filtered) reads includes
// projected in-window deploy IDs alongside Loki pod instances, so every
// instance value emitted by QueryLogs(type=build) is both discoverable and
// round-trip filterable — while untyped / type=app discovery stays Loki-only.
func TestLogLabelValuesIncludesInWindowDeployInstancesForBuild(t *testing.T) {
	fx := newProgressInstanceFixture(t)
	ctx := context.Background()

	wantBuildPod := ids.ServiceInstanceID("web", fx.buildPodName)
	wantDep1 := ids.ServiceInstanceID("web", fx.dep1)
	wantDep2 := ids.ServiceInstanceID("web", fx.dep2)
	wantAncient := ids.ServiceInstanceID("web", fx.depOutOfRange)
	wantAppPod := ids.ServiceInstanceID("web", "web-1")

	buildVals, err := fx.svc.LogLabelValues(ctx, LabelInstance, LogQuery{
		App: "web", Types: []string{LogTypeBuild}, Since: fx.start, End: fx.end,
	})
	if err != nil {
		t.Fatalf("LogLabelValues(build): %v", err)
	}
	for _, want := range []string{wantBuildPod, wantDep1, wantDep2} {
		if !slices.Contains(buildVals, want) {
			t.Errorf("build instance values %v missing %q", buildVals, want)
		}
	}
	if slices.Contains(buildVals, wantAncient) {
		t.Errorf("build instance values %v included out-of-window deploy %q", buildVals, wantAncient)
	}

	// Every discovered type=build instance round-trips through QueryLogs.
	for _, inst := range buildVals {
		entries, err := fx.svc.QueryLogs(ctx, LogQuery{
			App: "web", Types: []string{LogTypeBuild}, Instance: []string{inst},
			Since: fx.start, End: fx.end, Limit: 50,
		})
		if err != nil || len(entries) == 0 {
			t.Fatalf("round-trip QueryLogs(instance=%s) = %d entries, err=%v; want >0", inst, len(entries), err)
		}
		for _, e := range entries {
			if e.Labels[LabelInstance] != inst {
				t.Errorf("round-trip entry %q has instance=%q, want %q", e.Message, e.Labels[LabelInstance], inst)
			}
		}
	}

	// Build-filtered discovery includes only that build's deploy ID.
	scopedVals, err := fx.svc.LogLabelValues(ctx, LabelInstance, LogQuery{
		App: "web", Build: []string{fx.dep1}, Since: fx.start, End: fx.end,
	})
	if err != nil {
		t.Fatalf("LogLabelValues(build=dep1): %v", err)
	}
	if !slices.Contains(scopedVals, wantDep1) || slices.Contains(scopedVals, wantDep2) {
		t.Errorf("build-scoped instance values = %v, want %q and not %q", scopedVals, wantDep1, wantDep2)
	}

	// Positive request/level filters exclude synthetic deploy IDs (w32/104).
	filteredVals, err := fx.svc.LogLabelValues(ctx, LabelInstance, LogQuery{
		App: "web", Types: []string{LogTypeBuild}, StatusCode: []string{"404"}, Since: fx.start, End: fx.end,
	})
	if err != nil {
		t.Fatalf("LogLabelValues(build+statusCode): %v", err)
	}
	if slices.Contains(filteredVals, wantDep1) || slices.Contains(filteredVals, wantDep2) {
		t.Errorf("positive-filtered build instance values = %v, want no deploy IDs", filteredVals)
	}

	// Untyped and type=app discovery stay Loki-only (no deploy IDs, no DeployProgress call).
	before := fx.progressCalls.Load()
	for _, types := range [][]string{nil, {LogTypeApp}} {
		vals, err := fx.svc.LogLabelValues(ctx, LabelInstance, LogQuery{
			App: "web", Types: types, Since: fx.start, End: fx.end,
		})
		if err != nil {
			t.Fatalf("LogLabelValues(%v): %v", types, err)
		}
		if !slices.Equal(vals, []string{wantAppPod}) {
			t.Errorf("LogLabelValues(%v) = %v, want [%s]", types, vals, wantAppPod)
		}
	}
	if after := fx.progressCalls.Load(); after != before {
		t.Errorf("non-build LogLabelValues called DeployProgress %d times, want 0", after-before)
	}
}
