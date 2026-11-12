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
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/m159 t001: Render's `type` is a list that includes build, and the pinned
// CLI sends `--type request,build` as type=request&type=build. bex answered
// 400 "log type build must be requested on its own", because one store
// selector cannot union build streams with app and request streams.

func typedLine(at time.Time, typ, msg string) LogEntry {
	e := lineAt(at, msg)
	e.Labels[LabelType] = typ
	return e
}

// mixedStore serves typed lines the way the store selects them: one type
// list per read, never build beside another type (lokiTypeMatcher's
// invariant), recording each read's types.
func mixedStore(t *testing.T, entries ...LogEntry) (*Service, func() [][]string) {
	t.Helper()
	svc := newService(nil, sampleApp("web"), podFor("web", "web-1"))
	var mu sync.Mutex
	var reads [][]string
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		mu.Lock()
		reads = append(reads, slices.Clone(q.Types))
		mu.Unlock()
		if len(q.Types) > 1 && slices.Contains(q.Types, LogTypeBuild) {
			t.Errorf("the store was asked for build beside other types: %v", q.Types)
		}
		var in []LogEntry
		for _, e := range entries {
			if q.keep(e) && (len(q.Types) == 0 || slices.Contains(q.Types, e.Labels[LabelType])) {
				in = append(in, e)
			}
		}
		return q.capToLimit(in), nil
	}
	return svc, func() [][]string { mu.Lock(); defer mu.Unlock(); return slices.Clone(reads) }
}

func TestBuildLogsCombineWithOtherTypes(t *testing.T) {
	base := time.Date(2026, 11, 12, 7, 15, 0, 0, time.UTC)
	svc, reads := mixedStore(t,
		typedLine(base.Add(1*time.Second), LogTypeRequest, "GET / 200"),
		typedLine(base.Add(2*time.Second), LogTypeBuild, "==> Running build command 'true'..."),
		typedLine(base.Add(3*time.Second), LogTypeApp, "listening"),
		typedLine(base.Add(4*time.Second), LogTypeBuild, "==> Build successful"),
		typedLine(base.Add(5*time.Second), LogTypeRequest, "GET /health 200"),
	)
	for _, tc := range []struct {
		types     []string
		direction string
		limit     int
		want      string
		more      bool
	}{
		{[]string{LogTypeRequest, LogTypeBuild}, DirectionForward, 100,
			"[GET / 200 ==> Running build command 'true'... ==> Build successful GET /health 200]", false},
		{[]string{LogTypeApp, LogTypeRequest, LogTypeBuild}, DirectionForward, 100,
			"[GET / 200 ==> Running build command 'true'... listening ==> Build successful GET /health 200]", false},
		// The limit is a total across both reads; backward keeps the newest.
		{[]string{LogTypeApp, LogTypeBuild}, DirectionBackward, 2,
			"[==> Build successful listening]", true},
		{[]string{LogTypeApp, LogTypeBuild}, DirectionForward, 2,
			"[==> Running build command 'true'... listening]", true},
	} {
		name := fmt.Sprintf("%s/%s/%d", strings.Join(tc.types, "+"), tc.direction, tc.limit)
		t.Run(name, func(t *testing.T) {
			page, err := svc.queryLogPage(context.Background(), []string{"web"}, LogQuery{
				Types: tc.types, Direction: tc.direction, Limit: int64(tc.limit),
				Since: base, End: base.Add(time.Minute),
			})
			if err != nil {
				t.Fatalf("a %v read: %v", tc.types, err)
			}
			var got []string
			for _, e := range page.Entries {
				got = append(got, e.Message)
			}
			if fmt.Sprint(got) != tc.want || page.HasMore != tc.more {
				t.Errorf("lines = %v (more %v), want %s (more %v)", got, page.HasMore, tc.want, tc.more)
			}
		})
	}
	// Every read the store saw was build alone or the other types alone.
	for _, r := range reads() {
		if slices.Contains(r, LogTypeBuild) && len(r) > 1 {
			t.Errorf("store read %v mixed build with other types", r)
		}
	}

	// Pre-deploy logs stay their own source.
	if err := (LogQuery{App: "web", Types: []string{LogTypePreDeploy, LogTypeApp}}).validate(); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("predeploy beside app: %v, want a bad request", err)
	}
}

// One part whose scan ran out of time bounds the merged page: the other part's
// lines past its edge are dropped, so the next page, which resumes at the
// edge, repeats none of them.
func TestBuildLogsCombinedWithAPartialScanStopAtItsEdge(t *testing.T) {
	base := time.Date(2026, 11, 12, 7, 15, 0, 0, time.UTC)
	edge := base.Add(30 * time.Second)
	svc := newService(nil, sampleApp("web"), podFor("web", "web-1"))
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		if slices.Contains(q.Types, LogTypeBuild) {
			// Forward: covered [start, edge).
			return nil, &ScanIncompleteError{Entries: []LogEntry{typedLine(base.Add(10*time.Second), LogTypeBuild, "build-early")}, ScannedTo: edge}
		}
		return []LogEntry{
			typedLine(base.Add(20*time.Second), LogTypeApp, "app-before-edge"),
			typedLine(base.Add(40*time.Second), LogTypeApp, "app-after-edge"),
		}, nil
	}
	page, err := svc.queryLogPage(context.Background(), []string{"web"}, LogQuery{
		Types: []string{LogTypeApp, LogTypeBuild}, Direction: DirectionForward, Limit: 100,
		Since: base, End: base.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range page.Entries {
		got = append(got, e.Message)
	}
	if fmt.Sprint(got) != "[build-early app-before-edge]" || !page.HasMore || page.NextStartTime != edge.Format(time.RFC3339Nano) {
		t.Errorf("page = %v more=%v next=%s, want the lines before the edge and the next page starting at %s", got, page.HasMore, page.NextStartTime, edge)
	}
}

// The pinned CLI's `bex deploys create` streams the deploy through a
// type-less WebSocket tail opened at the trigger (pkg/tui/flows/log.go:
// TailLogsFlow). Render's subscription carries build lines there too, so the
// build shows while it runs, then the new instance's own lines (w8/m159).
// The dashboard's SSE tail with no type keeps app and request lines only.
func TestTypelessWebSocketTailCarriesBuildAndAppLines(t *testing.T) {
	soon := func(d time.Duration) string { return time.Now().Add(d).UTC().Format(time.RFC3339Nano) }
	// Every pod's lines are known up front: the fake stream reads the map
	// from other goroutines. The build pod itself appears later.
	lines := map[string][]string{
		"web-1":              {soon(time.Hour+time.Second) + " app listening"},
		"bld-web-gen-2-next": {soon(time.Hour) + " ==> Running build command 'true'..."},
	}
	svc := newService(lines, sampleApp("web"), podFor("web", "web-1"))
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond
	srv := revalidationTestServer(svc)
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web", nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()
	time.Sleep(50 * time.Millisecond)
	build := buildPodFor("web", "bld-web-gen-2-next", "builds", "buildkit", time.Now())
	if err := svc.Client.Create(context.Background(), build); err != nil {
		t.Fatalf("start the build: %v", err)
	}
	got := map[string]string{}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for len(got) < 2 {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read after %v: %v", got, err)
		}
		var line renderLog
		if err := json.Unmarshal(payload, &line); err != nil {
			t.Fatalf("frame %s is not a Log: %v", payload, err)
		}
		for _, l := range line.Labels {
			if l.Name == "type" {
				got[l.Value] = line.Message
			}
		}
	}
	if got[LogTypeBuild] != "==> Running build command 'true'..." || got[LogTypeApp] != "app listening" {
		t.Errorf("lines by type = %v, want the build line and the app line", got)
	}
}

// Label discovery and the instance lookup reach the store with a type list
// too, and must split a build mix as the history read does (review of
// w8/m159): the store selector cannot union build with app and request.
func TestLabelValuesForABuildMixAreTheUnionOfTheStoreReads(t *testing.T) {
	svc, _ := mixedStore(t)
	var mu sync.Mutex
	var reads [][]string
	svc.LabelValues = func(_ context.Context, _ string, label string, q LogQuery) ([]string, error) {
		mu.Lock()
		reads = append(reads, slices.Clone(q.Types))
		mu.Unlock()
		if len(q.Types) > 1 && slices.Contains(q.Types, LogTypeBuild) {
			t.Errorf("label discovery asked the store for build beside other types: %v", q.Types)
		}
		if label == LabelType {
			return slices.Clone(q.Types), nil
		}
		return []string{"web-1"}, nil
	}
	got, err := svc.LogLabelValues(context.Background(), LabelType, LogQuery{App: "web", Types: []string{LogTypeRequest, LogTypeBuild}})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != "[build request]" {
		t.Errorf("type values = %v, want the union of the build read and the request read", got)
	}
	if len(reads) != 2 {
		t.Errorf("store reads = %v, want two", reads)
	}
}

// A static site has no request lines (w4/585): its request+build read is the
// build read alone, not a read of app streams.
func TestAStaticSitesRequestAndBuildReadIsItsBuildRead(t *testing.T) {
	site := sampleApp("site")
	site.Spec.Type = "static_site"
	base := time.Date(2026, 11, 12, 7, 15, 0, 0, time.UTC)
	svc := newService(nil, site)
	var reads [][]string
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		reads = append(reads, slices.Clone(q.Types))
		return []LogEntry{typedLine(base.Add(time.Second), LogTypeBuild, "built")}, nil
	}
	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "site", Types: []string{LogTypeRequest, LogTypeBuild}, Since: base, End: base.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(reads) != "[[build]]" || len(entries) != 1 {
		t.Errorf("store reads = %v, lines = %d; want one build read and its line", reads, len(entries))
	}
}

// The CLI's build tail with the deploy rows wired (review of w8/m159): it
// replays nothing of a deploy that finished before it opened, narrates each
// platform line once while it reads the same build pod again on every pass,
// and follows a second build after the first.
func TestWebSocketBuildTailNarratesEachBuildOnce(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/x/y"
	opened := time.Now()
	soon := func(d time.Duration) string { return opened.Add(d).UTC().Format(time.RFC3339Nano) }
	lines := map[string][]string{
		"bld-web-gen-2-a": {soon(time.Hour) + " first build line"},
		"bld-web-gen-3-b": {soon(2*time.Hour) + " second build line"},
	}
	svc := newService(lines, app)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond
	var mu sync.Mutex
	rows := []DeployProgress{{ID: "dep-old", Status: "live", Built: true, Commit: "0000000aaaa",
		CreatedAt: opened.Add(-time.Hour), StartedAt: opened.Add(-time.Hour), FinishedAt: opened.Add(-50 * time.Minute)}}
	setRows := func(r ...DeployProgress) { mu.Lock(); rows = r; mu.Unlock() }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(rows), nil
	}
	srv := revalidationTestServer(svc)
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web&type=build", nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()
	var got, keys []string
	readUntil := func(want string) {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		for !slices.Contains(got, want) {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("waiting for %q after %v: %v", want, got, err)
			}
			var line renderLog
			if err := json.Unmarshal(payload, &line); err != nil {
				t.Fatalf("frame %s is not a Log: %v", payload, err)
			}
			got = append(got, line.Message)
			keys = append(keys, line.Timestamp+" "+line.Message)
		}
	}

	first := DeployProgress{ID: "dep-a", Status: "build_in_progress", Built: true, Commit: "aaaaaaa1111",
		CreatedAt: opened.Add(30 * time.Minute), StartedAt: opened.Add(31 * time.Minute)}
	setRows(first, rows[0])
	if err := svc.Client.Create(context.Background(), buildPodFor("web", "bld-web-gen-2-a", "builds", "buildkit", opened)); err != nil {
		t.Fatal(err)
	}
	readUntil("first build line")
	first.Status, first.FinishedAt = "live", opened.Add(90*time.Minute)
	setRows(first, rows[0])
	readUntil("==> Your service is live 🎉")

	second := DeployProgress{ID: "dep-b", Status: "build_in_progress", Built: true, Commit: "bbbbbbb2222",
		CreatedAt: opened.Add(100 * time.Minute), StartedAt: opened.Add(101 * time.Minute)}
	setRows(second, first, rows[0])
	if err := svc.Client.Create(context.Background(), buildPodFor("web", "bld-web-gen-3-b", "builds", "buildkit", opened.Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	readUntil("second build line")
	// Let a few more passes read the same pods again.
	_ = conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var line renderLog
		_ = json.Unmarshal(payload, &line)
		got = append(got, line.Message)
		keys = append(keys, line.Timestamp+" "+line.Message)
	}

	// A line is its time and its text: two deploys each say "Build queued".
	sentTimes := map[string]int{}
	for _, k := range keys {
		sentTimes[k]++
	}
	for k, n := range sentTimes {
		if n > 1 {
			t.Errorf("%q was sent %d times; lines: %v", k, n, got)
		}
	}
	counts := map[string]int{}
	for _, m := range got {
		counts[m]++
	}
	for _, m := range got {
		if strings.Contains(m, "0000000") {
			t.Errorf("the deploy that finished before the tail opened was replayed: %q", m)
		}
	}
	if counts["==> Building from https://github.com/x/y@aaaaaaa"] != 1 || counts["==> Building from https://github.com/x/y@bbbbbbb"] != 1 {
		t.Errorf("each build's narration once, got %v", got)
	}
}
