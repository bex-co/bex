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
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's rollout lines (w32/m560), restated so the tests pin the wire text.
const (
	rendersDeploying = "==> Deploying..."
	rendersWebConc1  = "==> Setting WEB_CONCURRENCY=1 by default, based on available CPUs in the instance"
	rendersRunning   = "==> Running 'npm start'"
	rendersLive      = "==> Your service is live 🎉"
	rolloutRepo      = "https://github.com/x/y"
	rolloutBuilding  = "==> Building from https://github.com/x/y@abc1234"
)

// The fixtures' moments, from DerekSopok/EMS-Kahoot#3's Render log: the
// rollout starts at 22:14:46.27 and the start command at 22:14:58.
var (
	rolloutCreated = time.Date(2026, 11, 22, 22, 13, 30, 0, time.UTC)
	rolloutStart   = time.Date(2026, 11, 22, 22, 14, 46, 270000000, time.UTC)
	firstAppStart  = time.Date(2026, 11, 22, 22, 14, 58, 0, time.UTC)
	rolloutClose   = time.Date(2026, 11, 22, 22, 15, 2, 930000000, time.UTC)
)

// nativeRollout is a built native Node web service's deploy that closed with
// status at rolloutClose: its rollout started at rolloutStart, its start
// command at firstAppStart, and its instance got WEB_CONCURRENCY=1.
func nativeRollout(status string) DeployProgress {
	return DeployProgress{
		ID: "dep-rollout", Generation: 4, Status: status, Built: true, Commit: "abc1234def5678",
		CreatedAt: rolloutCreated, StartedAt: rolloutCreated.Add(time.Second), FinishedAt: rolloutClose,
		RolloutStartedAt: rolloutStart, AppStartedAt: []time.Time{firstAppStart},
		StartCommand: "npm start", WebConcurrencyDefault: 1,
	}
}

// story is the messages of d's platform lines, in order.
func story(d DeployProgress, pc progressContext) []string {
	var out []string
	for _, e := range progressLines(d, pc) {
		out = append(out, e.Message)
	}
	return out
}

// w32/m560: between a build's end and its verdict, Render's deploy log says
// `==> Deploying...` when the rollout starts, the WEB_CONCURRENCY default its
// instance got, and on a native runtime `==> Running '<command>'` each time
// the command starts. Each line comes only from a moment the operator
// recorded, so a deploy that never reached its rollout, a static site and a
// cron job get none of them, and a stamp after the close is not the deploy's.
func TestRolloutLinesFollowRendersLogByServiceTypeAndOutcome(t *testing.T) {
	web := progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService}
	built := []string{"==> Build queued", rolloutBuilding}
	with := func(d DeployProgress, change func(*DeployProgress)) DeployProgress {
		change(&d)
		return d
	}
	noRecord := func(d *DeployProgress) {
		d.RolloutStartedAt, d.AppStartedAt, d.StartCommand, d.WebConcurrencyDefault = time.Time{}, nil, "", 0
	}
	exited := int32(1)
	for _, c := range []struct {
		name string
		row  DeployProgress
		pc   progressContext
		want []string
	}{
		{"a native web service that went live", nativeRollout("live"), web,
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersLive})},
		{"a native private service", nativeRollout("live"),
			progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypePrivateService},
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersLive})},
		{"a native background worker, which gets no default", with(nativeRollout("live"), func(d *DeployProgress) { d.WebConcurrencyDefault = 0 }),
			progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeBackgroundWorker},
			slices.Concat(built, []string{rendersDeploying, rendersRunning, rendersLive})},
		{"a Dockerfile web service, which names no command", with(nativeRollout("live"), func(d *DeployProgress) { d.StartCommand = "" }), web,
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersLive})},
		{"a web service that sets its own WEB_CONCURRENCY", with(nativeRollout("live"), func(d *DeployProgress) { d.WebConcurrencyDefault = 0 }), web,
			slices.Concat(built, []string{rendersDeploying, rendersRunning, rendersLive})},
		{"a prebuilt image keeps its own rollout line", with(nativeRollout("live"), func(d *DeployProgress) {
			d.Built, d.Commit, d.Image, d.StartCommand = false, "", "docker.io/library/nginx:1.27", ""
		}), progressContext{serviceType: appv1alpha1.TypeWebService},
			[]string{"==> Deploy queued", "==> Deploying image docker.io/library/nginx:1.27", rendersWebConc1, rendersLive}},
		{"a static site", with(nativeRollout("live"), noRecord),
			progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeStaticSite},
			slices.Concat(built, []string{"==> Your site is live 🎉"})},
		{"a cron job", with(nativeRollout("live"), noRecord),
			progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeCronJob},
			slices.Concat(built, []string{"==> Cron job deployed — runs on schedule 🎉"})},
		{"a start command that exited, then started again", with(nativeRollout("update_failed"), func(d *DeployProgress) {
			d.ExitCode, d.AppStartedAt = &exited, []time.Time{firstAppStart, firstAppStart.Add(2 * time.Second)}
		}), web, slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersRunning,
			"==> Exited with status 1", troubleshootLine})},
		{"a rollout that ran out of its budget", with(nativeRollout("update_failed"), func(d *DeployProgress) { d.TimedOut = true }), web,
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, timedOutLine, troubleshootLine})},
		{"a build that failed", with(nativeRollout("build_failed"), noRecord), web,
			slices.Concat(built, []string{"==> Build failed"})},
		{"a deploy canceled before its rollout", with(nativeRollout("canceled"), noRecord), web,
			slices.Concat(built, []string{"==> Deploy canceled"})},
		{"a deploy canceled during its rollout", nativeRollout("canceled"), web,
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, "==> Deploy canceled"})},
		{"a rollout still running", with(nativeRollout("update_in_progress"), func(d *DeployProgress) { d.FinishedAt = time.Time{} }), web,
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning})},
		{"a row an older operator closed, with no moment recorded", with(nativeRollout("live"), noRecord), web,
			slices.Concat(built, []string{rendersLive})},
		{"a record stamped after the close", with(nativeRollout("live"), func(d *DeployProgress) { d.RolloutStartedAt = rolloutClose.Add(time.Second) }), web,
			slices.Concat(built, []string{rendersLive})},
		{"a start after the close is the service's", with(nativeRollout("live"), func(d *DeployProgress) {
			d.AppStartedAt = []time.Time{firstAppStart, rolloutClose.Add(5 * time.Second)}
		}), web, slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersLive})},
		// w32/m575: an image built since native-init names its command writes
		// the line in the instance's own stream; the narration keeps the rest.
		{"a native web service whose instance names its command", with(nativeRollout("live"), announcing), web,
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersLive})},
		{"a native private service whose instance names its command", with(nativeRollout("live"), announcing),
			progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypePrivateService},
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersLive})},
		{"a native background worker whose instance names its command", with(nativeRollout("live"), func(d *DeployProgress) {
			announcing(d)
			d.WebConcurrencyDefault = 0
		}), progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeBackgroundWorker},
			slices.Concat(built, []string{rendersDeploying, rendersLive})},
		{"an announcing start command that exited, then started again", with(nativeRollout("update_failed"), func(d *DeployProgress) {
			announcing(d)
			d.ExitCode, d.AppStartedAt = &exited, []time.Time{firstAppStart, firstAppStart.Add(2 * time.Second)}
		}), web, slices.Concat(built, []string{rendersDeploying, rendersWebConc1, "==> Exited with status 1", troubleshootLine})},
		{"an announcing rollout that ran out of its budget", with(nativeRollout("update_failed"), func(d *DeployProgress) {
			announcing(d)
			d.TimedOut = true
		}), web, slices.Concat(built, []string{rendersDeploying, rendersWebConc1, timedOutLine, troubleshootLine})},
		{"an announcing rollout still running", with(nativeRollout("update_in_progress"), func(d *DeployProgress) {
			announcing(d)
			d.FinishedAt = time.Time{}
		}), web, slices.Concat(built, []string{rendersDeploying, rendersWebConc1})},
		{"a rollback to an image built before native-init named its command", with(nativeRollout("live"), func(d *DeployProgress) {
			d.Built, d.Commit, d.Image = false, "", "zot.bex-registry.svc:5000/tea-ws/web:gen-2"
		}), web, []string{"==> Deploy queued", "==> Deploying image zot.bex-registry.svc:5000/tea-ws/web:gen-2", rendersWebConc1, rendersRunning, rendersLive}},
		{"a rollback to an image whose instance names its command", with(nativeRollout("live"), func(d *DeployProgress) {
			announcing(d)
			d.Built, d.Commit, d.Image = false, "", "zot.bex-registry.svc:5000/tea-ws/web:gen-3"
		}), web, []string{"==> Deploy queued", "==> Deploying image zot.bex-registry.svc:5000/tea-ws/web:gen-3", rendersWebConc1, rendersLive}},
	} {
		if got := story(c.row, c.pc); !slices.Equal(got, c.want) {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// announcing marks d's release as one on an image whose native-init writes
// `==> Running '<command>'` itself (w32/m575).
func announcing(d *DeployProgress) { d.InstanceAnnounces = true }

// Render prints the command as configured, $PORT unexpanded
// (frayel/devagent#62), and the lines carry the moments the operator
// recorded: the rollout's start, then each start of the app container.
func TestRolloutLinesQuoteTheCommandAtItsRecordedStarts(t *testing.T) {
	d := nativeRollout("update_failed")
	d.StartCommand = "uvicorn app.main:app --host 0.0.0.0 --port $PORT"
	second := firstAppStart.Add(3 * time.Second)
	d.AppStartedAt = []time.Time{firstAppStart, second}
	var got []string
	for _, e := range rolloutLines(d, true) {
		got = append(got, e.Timestamp+" "+e.Message)
	}
	stamp := func(t time.Time) string { return t.Format(time.RFC3339Nano) + " " }
	running := "==> Running 'uvicorn app.main:app --host 0.0.0.0 --port $PORT'"
	want := []string{
		stamp(rolloutStart) + rendersDeploying,
		stamp(rolloutStart.Add(rolloutLineStep)) + rendersWebConc1,
		stamp(firstAppStart) + running,
		stamp(second) + running,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines:\n got %q\nwant %q", got, want)
	}
}

// The kubelet stamps a container's start in whole seconds and a deploy that
// ran no build starts at its dispatch, in whole seconds too: a line whose
// moment would read before the one it follows is held just after it.
func TestRolloutLinesKeepTheirOrderWithinASecond(t *testing.T) {
	d := nativeRollout("live")
	d.RolloutStartedAt = d.StartedAt
	d.AppStartedAt = []time.Time{d.StartedAt.Truncate(time.Second)}
	lines := rolloutLines(d, true)
	if len(lines) != 3 {
		t.Fatalf("lines = %v, want three", lines)
	}
	prev := d.StartedAt
	for _, e := range lines {
		at, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil || !at.After(prev) {
			t.Fatalf("%q at %s does not follow %s: %v", e.Message, e.Timestamp, prev, lines)
		}
		prev = at
	}
}

// A native build's own last line, `==> Build successful 🎉`, is stamped by the
// pod in nanoseconds; the rollout's start, in the same second, is read after
// it: the history read orders by those moments.
func TestDeployingReadsAfterTheBuildsLastLineInTheSameSecond(t *testing.T) {
	d := nativeRollout("live")
	built := time.Date(2026, 11, 22, 22, 14, 46, 100000000, time.UTC)
	output := LogEntry{
		Timestamp: built.Format(time.RFC3339Nano), Message: "==> Build successful 🎉",
		Labels: map[string]string{LabelType: LogTypeBuild, LabelInstance: "bld-web-gen-4-a1b2c", "container": "buildkit"},
	}
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	svc := newService(nil, app)
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return []LogEntry{output}, nil }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }
	got := inOrder(buildRead(t, svc, LogQuery{Since: rolloutCreated, End: rolloutClose.Add(time.Minute)}))
	at := slices.Index(got, "==> Build successful 🎉")
	if at < 0 || at+3 >= len(got) || !slices.Equal(got[at+1:at+4], []string{rendersDeploying, rendersWebConc1, rendersRunning}) {
		t.Fatalf("log = %q, want the rollout lines right after the build's last line", got)
	}
}

// w32/m560 t006: the CLI's build tail (Render's WebSocket subscription, which
// follows every build) sends the rollout lines while the row is in its
// rollout, in order and each once, then the verdict; a history read over the
// same window returns the same lines with the same ids.
func TestWebSocketBuildTailSendsTheRolloutLinesAsTheyAreRecorded(t *testing.T) {
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
	update := func(change func(*DeployProgress)) { mu.Lock(); change(&row); mu.Unlock() }
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
	var got []string
	ids := map[string]string{}
	readUntil := func(want string) {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		for !slices.Contains(got, want) {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("waiting for %q after %q: %v", want, got, err)
			}
			var line renderLog
			if err := json.Unmarshal(payload, &line); err != nil {
				t.Fatalf("frame %s is not a Log: %v", payload, err)
			}
			got = append(got, line.Message)
			ids[line.Message] = line.ID
		}
	}
	readUntil("==> Build successful 🎉")

	update(func(d *DeployProgress) {
		d.Status, d.RolloutStartedAt, d.StartCommand, d.WebConcurrencyDefault = "update_in_progress", soon(70*time.Minute), "npm start", 1
	})
	readUntil(rendersWebConc1)
	update(func(d *DeployProgress) { d.AppStartedAt = []time.Time{soon(71 * time.Minute).Truncate(time.Second)} })
	readUntil(rendersRunning)
	update(func(d *DeployProgress) { d.Status, d.FinishedAt = "live", soon(72*time.Minute) })
	readUntil(rendersLive)
	// Let a few more passes read the same row again.
	_ = conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var line renderLog
		_ = json.Unmarshal(payload, &line)
		got = append(got, line.Message)
	}

	rollout := []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersLive}
	var sent []string
	for _, m := range got {
		if slices.Contains(rollout, m) {
			sent = append(sent, m)
		}
	}
	if !slices.Equal(sent, rollout) {
		t.Fatalf("tail sent %q, want %q in order, each once (all lines %q)", sent, rollout, got)
	}

	history, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}, Since: opened, End: soon(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	read := map[string]string{}
	for _, e := range history {
		read[e.Message] = logID(e)
	}
	for _, m := range rollout {
		if read[m] == "" || read[m] != ids[m] {
			t.Errorf("%q: history id %q, tail id %q", m, read[m], ids[m])
		}
	}
}

// w32/m575: a deploy's view that carries the instance's own lines beside its
// narration (the dashboard's deploy page, the CLI's type-less deploy tail, a
// read of app and build together) shows one `==> Running '<command>'` per
// start, whichever side writes it: the instance's, with its own labels, on
// an image whose native-init names the command; the narration's, at the
// recorded start, on an older image, whose instance writes none.
func TestADeploysAppAndBuildViewShowsOneRunningLinePerStart(t *testing.T) {
	second := firstAppStart.Add(4 * time.Second)
	instanceLine := func(at time.Time) LogEntry {
		e := typedLine(at, LogTypeApp, rendersRunning)
		e.Labels[LabelInstance], e.Labels[LabelLevel] = "web-6d9f7c8b5-x2k4p", "info"
		return e
	}
	output := []LogEntry{
		typedLine(rolloutStart.Add(-200*time.Millisecond), LogTypeBuild, "==> Build successful 🎉"),
		typedLine(firstAppStart.Add(900*time.Millisecond), LogTypeApp, "listening on 10000"),
		typedLine(second.Add(900*time.Millisecond), LogTypeApp, "listening on 10000"),
	}
	for _, tc := range []struct {
		name      string
		announces bool
		instance  []LogEntry
		wantType  string
	}{
		{"an image whose native-init names the command", true,
			[]LogEntry{instanceLine(firstAppStart.Add(150 * time.Millisecond)), instanceLine(second.Add(150 * time.Millisecond))}, LogTypeApp},
		{"an image built before it", false, nil, LogTypeBuild},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := nativeRollout("live")
			d.AppStartedAt, d.InstanceAnnounces = []time.Time{firstAppStart, second}, tc.announces
			app := sampleApp("web")
			app.Spec.Repo = rolloutRepo
			svc := newService(nil, app, podFor("web", "web-1"))
			entries := slices.Concat(output, tc.instance)
			svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
				var in []LogEntry
				for _, e := range entries {
					if q.keep(e) && (len(q.Types) == 0 || slices.Contains(q.Types, e.Labels[LabelType])) {
						in = append(in, e)
					}
				}
				return in, nil
			}
			svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }
			got, err := svc.QueryLogs(context.Background(), LogQuery{
				App: "web", Types: []string{LogTypeApp, LogTypeBuild}, Since: rolloutCreated, End: rolloutClose.Add(time.Minute),
			})
			if err != nil {
				t.Fatal(err)
			}
			var running []LogEntry
			for _, e := range got {
				if e.Message == rendersRunning {
					running = append(running, e)
				}
			}
			if len(running) != 2 {
				t.Fatalf("Running lines = %d in %q, want one per start", len(running), inOrder(got))
			}
			for i, e := range running {
				if e.Labels[LabelType] != tc.wantType {
					t.Errorf("Running line %d labels = %v, want type %s", i, e.Labels, tc.wantType)
				}
				if tc.announces && (e.Labels[LabelInstance] == "" || e.Labels[LabelLevel] != "info") {
					t.Errorf("the instance's line lost its instance or level label: %v", e.Labels)
				}
			}
			at := slices.IndexFunc(got, func(e LogEntry) bool { return e.Message == rendersRunning })
			if at < 0 || at+1 >= len(got) || got[at+1].Message != "listening on 10000" {
				t.Errorf("log = %q, want each Running line right before the command's own output", inOrder(got))
			}
		})
	}
}
