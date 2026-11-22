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
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// Render's block, byte for byte as falcon-Z/z0-auth#12 reads it through
// Render's API: the blank lines keep their space, the rule is 59 slashes, and
// the URL ends at its host.
const (
	rendersBlank = "==> "
	rendersRule  = "==> ///////////////////////////////////////////////////////////"
	rendersURL   = "==> Available at your primary URL https://z0-auth.onbex.co"
)

// rendersBlock is Render's six lines for url, the line that names it.
func rendersBlock(url string) []string {
	return []string{rendersBlank, rendersRule, rendersBlank, url, rendersBlank, rendersRule}
}

// webLive is a deploy of a web service that went live at
// https://z0-auth.onbex.co, its row's record.
func webLive() DeployProgress {
	d := nativeRollout("live")
	d.LiveURLs = []string{"https://z0-auth.onbex.co"}
	return d
}

// w32/m567: Render's deploy log follows a web service's live line with six
// lines naming its primary URL, each a line of its own a moment after the one
// before it (falcon-Z/z0-auth#12: 17.712 live, then 17.947 to 17.965).
func TestPrimaryURLBlockFollowsTheLiveLineInRendersBytes(t *testing.T) {
	if n := strings.Count(rendersRule, "/"); n != 59 {
		t.Fatalf("the test's rule has %d slashes, want Render's 59", n)
	}
	d := webLive()
	lines := progressLines(d, progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService})
	at := slices.IndexFunc(lines, func(e LogEntry) bool { return e.Message == rendersLive })
	if at < 0 || at+7 != len(lines) {
		t.Fatalf("lines = %q, want the live line then six more", inOrder(lines))
	}
	if got := inOrder(lines[at+1:]); !slices.Equal(got, rendersBlock(rendersURL)) {
		t.Fatalf("block = %q\nwant %q", got, rendersBlock(rendersURL))
	}
	ids := map[string]bool{}
	for i, e := range lines[at:] {
		if want := d.FinishedAt.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano); e.Timestamp != want {
			t.Errorf("%q at %s, want %s", e.Message, e.Timestamp, want)
		}
		if e.Labels[LabelInstance] != d.ID || e.Labels["container"] != progressContainer || e.Labels[LabelType] != LogTypeBuild {
			t.Errorf("%q labels = %v, want the live line's", e.Message, e.Labels)
		}
		if id := logID(e); ids[id] {
			t.Errorf("%q shares its id with an earlier line", e.Message)
		} else {
			ids[id] = true
		}
	}
}

// The block comes from the row's own record, so every web deploy that went
// live gets it, however it got there, and nothing else does: a failure, a
// cancel, and a row with no record (a static site, a private service, a
// worker, a cron job, a row closed before the record existed) end where they
// did. A deactivated deploy keeps the block it went live with, and a custom
// domain counts the service's other domains.
func TestPrimaryURLBlockByDeployAndOutcome(t *testing.T) {
	web := progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService}
	image := progressContext{serviceType: appv1alpha1.TypeWebService}
	with := func(change func(*DeployProgress)) DeployProgress {
		d := webLive()
		change(&d)
		return d
	}
	exited := int32(1)
	block := rendersBlock(rendersURL)
	for _, c := range []struct {
		name string
		row  DeployProgress
		pc   progressContext
		want []string // the lines after the verdict's first line
	}{
		{"a native web service", webLive(), web, block},
		{"a Dockerfile web service", with(func(d *DeployProgress) { d.StartCommand = "" }), web, block},
		{"a prebuilt image", with(func(d *DeployProgress) {
			d.Built, d.Commit, d.Image, d.StartCommand = false, "", "docker.io/library/nginx:1.27", ""
		}), image, block},
		{"a rollback", with(func(d *DeployProgress) { d.Built, d.Image = false, "registry.bex.co/web@sha256:abc" }), web, block},
		{"a deploy a newer one replaced", with(func(d *DeployProgress) { d.Status = "deactivated" }), web, block},
		{"a custom domain and the platform URL", with(func(d *DeployProgress) {
			d.LiveURLs = []string{"https://bookorbit.example", "https://z0-auth.onbex.co"}
		}), web, rendersBlock("==> Available at your primary URL https://bookorbit.example + 1 more domain")},
		{"a root domain, its www redirect and the platform URL", with(func(d *DeployProgress) {
			d.LiveURLs = []string{"https://teranga-ai.example", "https://z0-auth.onbex.co", "https://www.teranga-ai.example"}
		}), web, rendersBlock("==> Available at your primary URL https://teranga-ai.example + 2 more domains")},
		{"a row with no record", with(func(d *DeployProgress) { d.LiveURLs = nil }), web, nil},
		{"a static site, which records none", with(func(d *DeployProgress) { d.LiveURLs = nil }),
			progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeStaticSite}, nil},
		{"a start command that exited", with(func(d *DeployProgress) { d.Status, d.ExitCode = "update_failed", &exited }), web, []string{troubleshootLine}},
		{"a rollout out of its budget", with(func(d *DeployProgress) { d.Status, d.TimedOut = "update_failed", true }), web, []string{troubleshootLine}},
		{"a failed build", with(func(d *DeployProgress) { d.Status = "build_failed" }), web, nil},
		{"a failed pre-deploy", with(func(d *DeployProgress) { d.Status = "pre_deploy_failed" }), web, nil},
		{"a cancel", with(func(d *DeployProgress) { d.Status = "canceled" }), web, nil},
		{"a rollout still running", with(func(d *DeployProgress) { d.Status, d.FinishedAt = "update_in_progress", time.Time{} }), web, nil},
	} {
		lines := progressLines(c.row, c.pc)
		verdict := slices.IndexFunc(lines, func(e LogEntry) bool { return e.Timestamp == c.row.FinishedAt.Format(time.RFC3339Nano) })
		var got []string
		if verdict >= 0 {
			got = inOrder(lines[verdict+1:])
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: after the verdict %q\n got %q\nwant %q", c.name, inOrder(lines), got, c.want)
		}
	}
}

// Render writes the block straight after the live line; the initial deploy
// hook runs once the deploy is live, so its lines come after the block.
func TestPrimaryURLBlockComesBeforeTheInitialDeployHook(t *testing.T) {
	rows := hookRows()
	rows[1].LiveURLs = []string{"https://web.onbex.co"}
	svc := hookService(rows, hookRun1(appv1alpha1.PreDeploySucceeded, "", true), nil)
	got := inOrder(buildRead(t, svc, LogQuery{Deploy: "dep-first", Since: hookLive.Add(-2 * time.Minute), End: hookLive.Add(time.Minute)}))
	want := slices.Concat([]string{"==> Deploy queued", "==> Rolling out the current release", rendersLive},
		rendersBlock("==> Available at your primary URL https://web.onbex.co"), []string{hookLineRunning, hookLineSucceeded})
	if !slices.Equal(got, want) {
		t.Fatalf("first deploy's log = %q\nwant %q", got, want)
	}
}

// A window that opens inside the block keeps the block's lines after its
// start: the row's story ends with the block, not at its live line.
func TestPrimaryURLBlockSurvivesAWindowOpenedInsideIt(t *testing.T) {
	d := webLive()
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }
	got := inOrder(buildRead(t, svc, LogQuery{Since: d.FinishedAt.Add(3 * time.Millisecond), End: d.FinishedAt.Add(time.Minute)}))
	if want := rendersBlock(rendersURL)[2:]; !slices.Equal(got, want) {
		t.Fatalf("log = %q, want the block from its third line on %q", got, want)
	}
	// A window that closes inside it keeps the lines before its end.
	got = inOrder(buildRead(t, svc, LogQuery{Since: d.FinishedAt, End: d.FinishedAt.Add(2 * time.Millisecond)}))
	if want := []string{rendersLive, rendersBlank, rendersRule}; !slices.Equal(got, want) {
		t.Fatalf("log = %q, want %q", got, want)
	}
}

// A row first seen live while its build's output is still arriving keeps its
// live line and the block until the build's last line, then sends each line
// of the block once, in order.
func TestBuildTailHoldsThePrimaryURLBlockUntilTheBuildsLastLine(t *testing.T) {
	b := startBuildTail(t)
	b.print("==> Using Node 24")
	b.expect("==> Using Node 24")
	b.update(func(d *DeployProgress) {
		d.Status, d.Built = "live", true
		d.StartedAt, d.FinishedAt = d.CreatedAt.Add(time.Second), d.CreatedAt.Add(time.Minute)
		d.LiveURLs = []string{"https://z0-auth.onbex.co"}
	})
	b.expect(buildingFrom)
	b.quiet()
	b.print("==> Build successful 🎉")
	b.expect("==> Build successful 🎉")
	if err := b.out.Close(); err != nil {
		t.Fatal(err)
	}
	// The tail ends once it has sent the row's close; every line it sent is
	// in the buffered channel by then.
	select {
	case err := <-b.done:
		if err != nil {
			t.Fatalf("tail ended with %v, want a clean end", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the tail did not end")
	}
	var rest []string
	for len(b.got) > 0 {
		rest = append(rest, <-b.got)
	}
	if want := append([]string{rendersLive}, rendersBlock(rendersURL)...); !slices.Equal(rest, want) {
		t.Fatalf("after the build's last line the tail sent %q\nwant %q", rest, want)
	}
}

// The CLI's build tail (Render's WebSocket subscription, which follows every
// build) sends the block with the live line, each line once however many
// passes read the row again, with the ids a history read gives them.
func TestWebSocketBuildTailSendsThePrimaryURLBlockOnce(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }
	svc := newService(map[string][]string{
		"bld-web-gen-4-a": {soon(time.Hour).Format(time.RFC3339Nano) + " ==> Build successful 🎉"},
	}, app)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	var mu sync.Mutex
	row := DeployProgress{ID: "dep-r", Generation: 4, Status: deployStatusBuildInProgress, Built: true, Commit: "abc1234def5678",
		CreatedAt: soon(30 * time.Minute), StartedAt: soon(31 * time.Minute)}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		mu.Lock()
		defer mu.Unlock()
		return []DeployProgress{row}, nil
	}
	srv := revalidationTestServer(svc)
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource=web&type=build", nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()
	if err := svc.Client.Create(context.Background(), buildPodFor("web", "bld-web-gen-4-a", "builds", "buildkit", opened)); err != nil {
		t.Fatal(err)
	}
	var got, ids []string
	read := func(until func() bool, wait time.Duration) {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(wait))
		for !until() {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				if wait < time.Second {
					return
				}
				t.Fatalf("waiting after %q: %v", got, err)
			}
			var line renderLog
			if err := json.Unmarshal(payload, &line); err != nil {
				t.Fatalf("frame %s is not a Log: %v", payload, err)
			}
			got, ids = append(got, line.Message), append(ids, line.ID)
		}
	}
	read(func() bool { return slices.Contains(got, "==> Build successful 🎉") }, 5*time.Second)
	mu.Lock()
	row.Status, row.FinishedAt, row.LiveURLs = "live", soon(72*time.Minute), []string{"https://z0-auth.onbex.co"}
	mu.Unlock()
	read(func() bool { return len(got) > 0 && got[len(got)-1] == rendersRule && slices.Contains(got, rendersURL) }, 5*time.Second)
	// Let a few more passes read the same row again.
	read(func() bool { return false }, 150*time.Millisecond)

	at := slices.Index(got, rendersLive)
	if at < 0 || !slices.Equal(got[at+1:], rendersBlock(rendersURL)) {
		t.Fatalf("tail sent %q, want the live line then the block, each line once", got)
	}
	history, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}, Since: opened, End: soon(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	live := slices.IndexFunc(history, func(e LogEntry) bool { return e.Message == rendersLive })
	if live < 0 || live+7 > len(history) {
		t.Fatalf("history %q has no block after its live line", inOrder(history))
	}
	var read7 []string
	for _, e := range history[live : live+7] {
		read7 = append(read7, logID(e))
	}
	if !slices.Equal(read7, ids[at:]) {
		t.Fatalf("history ids %q, tail ids %q", read7, ids[at:])
	}
}

// REST /v1/logs (`bex logs --type build`), GraphQL logs (the dashboard's
// deploy log pane) and MCP list_logs read the block's six lines in order,
// with the same ids, and the JSON keeps each blank line's trailing space.
func TestPrimaryURLBlockReadsTheSameOnRESTGraphQLAndMCP(t *testing.T) {
	d := webLive()
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	svc := newService(nil, app)
	svc.PodLogs, svc.PodLogsFollow = nil, nil
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }
	start, end := d.FinishedAt.Add(-time.Second).Format(time.RFC3339), d.FinishedAt.Add(time.Second).Format(time.RFC3339)
	block := append([]string{rendersLive}, rendersBlock(rendersURL)...)
	// lines is the messages and ids from the live line on, oldest first.
	lines := func(surface string, messages, ids []string) []string {
		t.Helper()
		if len(messages) > 0 && messages[0] != rendersLive {
			slices.Reverse(messages)
			slices.Reverse(ids)
		}
		at := slices.Index(messages, rendersLive)
		if at < 0 || !slices.Equal(messages[at:], block) {
			t.Fatalf("%s reads %q, want the live line then the block", surface, messages)
		}
		return ids[at:]
	}

	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&type=build&startTime="+start+"&endTime="+end)
	var env renderLogList
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("REST = %d %s (%v)", rec.Code, rec.Body, err)
	}
	if !strings.Contains(rec.Body.String(), `"message":"==\u003e "`) {
		t.Fatalf("REST body has no blank marker with its space: %s", rec.Body)
	}
	var messages, ids []string
	for _, l := range env.Logs {
		messages, ids = append(messages, l.Message), append(ids, l.ID)
	}
	rest := lines("REST", messages, ids)

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", type:"build", startTime:"`+start+`", endTime:"`+end+`") { logs { id message } } }`)
	messages, ids = nil, nil
	for _, row := range data["logs"].(map[string]any)["logs"].([]any) {
		m := row.(map[string]any)
		messages, ids = append(messages, m["message"].(string)), append(ids, m["id"].(string))
	}
	if gql := lines("GraphQL", messages, ids); !slices.Equal(gql, rest) {
		t.Fatalf("GraphQL ids %q, REST %q", gql, rest)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web"}, "type": []string{LogTypeBuild}, "startTime": start, "endTime": end,
	}})
	if err != nil || res.IsError {
		t.Fatalf("list_logs: %v %+v", err, res)
	}
	var got renderLogList
	rendertest.DecodeMCPText(t, res, &got)
	messages, ids = nil, nil
	for _, l := range got.Logs {
		messages, ids = append(messages, l.Message), append(ids, l.ID)
	}
	if mcpIDs := lines("MCP", messages, ids); !slices.Equal(mcpIDs, rest) {
		t.Fatalf("MCP ids %q, REST %q", mcpIDs, rest)
	}
}
