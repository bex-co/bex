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
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/types/renderlog"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// w32/m591: for a pre-w32/m575 native image (InstanceAnnounces: false), a
// failed deploy with a recorded first exit places its single
// `==> Exited with status N` (using run 1's status) and troubleshooting line
// right after run 1's `==> Running '<command>'`, before runs 2 and 3, with
// nothing at FinishedAt.
func TestFirstExitVerdictFollowsRun1OnLegacyNativeDeploy(t *testing.T) {
	web := progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService}
	run1Start := firstAppStart
	run1Exit := firstAppStart.Add(4 * time.Second)
	run2Start := firstAppStart.Add(8 * time.Second)
	run3Start := firstAppStart.Add(22 * time.Second)
	closedAt := firstAppStart.Add(35 * time.Second)
	firstCode, lastCode := int32(1), int32(2)

	d := nativeRollout("update_failed")
	d.FinishedAt = closedAt
	d.AppStartedAt = []time.Time{run1Start, run2Start, run3Start}
	d.FirstExitAt, d.FirstExitCode = run1Exit, &firstCode
	d.ExitCode = &lastCode

	entries := progressLines(d, web)
	var msgs []string
	for _, e := range entries {
		msgs = append(msgs, e.Message)
	}
	want := []string{
		"==> Build queued",
		rolloutBuilding,
		rendersDeploying,
		rendersWebConc1,
		rendersRunning,
		rendersExited1,
		rendersTroubleshoot,
		rendersRunning,
		rendersRunning,
	}
	if !slices.Equal(msgs, want) {
		t.Fatalf("legacy native story:\n got %q\nwant %q", msgs, want)
	}
	// Verdict pair is stamped at the close of run1Exit's whole second,
	// strictly before run2Start and before FinishedAt.
	wantAt := run1Exit.Add(time.Second - time.Nanosecond).UTC().Format(time.RFC3339Nano)
	if entries[5].Timestamp != wantAt || entries[6].Timestamp != wantAt {
		t.Fatalf("verdict timestamps = (%s, %s), want %s", entries[5].Timestamp, entries[6].Timestamp, wantAt)
	}
	for _, e := range entries {
		at, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Timestamp, err)
		}
		if at.Equal(closedAt) || at.After(closedAt) {
			t.Fatalf("unexpected line at/after FinishedAt: %q at %s", e.Message, e.Timestamp)
		}
	}
}

// w32/m591: when run 1 exits and run 2 starts in the exact same whole second,
// the legacy native log still orders Running (run 1), Exited + troubleshoot,
// Running (run 2), Running (run 3) with monotonic timestamps inside FinishedAt.
func TestFirstExitVerdictOrdersWhenRun2StartsInSameWholeSecond(t *testing.T) {
	web := progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService}
	run1Start := firstAppStart
	sameSec := firstAppStart.Add(1 * time.Second)
	run3Start := firstAppStart.Add(12 * time.Second)
	closedAt := firstAppStart.Add(30 * time.Second)
	code := int32(1)

	d := nativeRollout("update_failed")
	d.FinishedAt = closedAt
	d.AppStartedAt = []time.Time{run1Start, sameSec, run3Start}
	d.FirstExitAt, d.FirstExitCode = sameSec, &code
	d.ExitCode = &code

	entries := progressLines(d, web)
	var msgs []string
	var prev time.Time
	for _, e := range entries {
		msgs = append(msgs, e.Message)
		at, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Timestamp, err)
		}
		if at.Before(prev) {
			t.Fatalf("non-monotonic timestamp: %q at %s before %s", e.Message, at, prev)
		}
		if at.After(closedAt) {
			t.Fatalf("timestamp past FinishedAt: %q at %s > %s", e.Message, at, closedAt)
		}
		prev = at
	}
	want := []string{
		"==> Build queued", rolloutBuilding, rendersDeploying, rendersWebConc1,
		rendersRunning, rendersExited1, rendersTroubleshoot, rendersRunning, rendersRunning,
	}
	if !slices.Equal(msgs, want) {
		t.Fatalf("same-second story:\n got %q\nwant %q", msgs, want)
	}
}

// w32/m591: for a new native deploy (InstanceAnnounces: true) and a Dockerfile
// deploy (StartCommand: ""), the interleaved app+build log places the verdict
// pair right after run 1's nanosecond app output (in the same second as
// FirstExitAt) and strictly before run 2's lines in the next second, with no
// Running lines in type=build and nothing at FinishedAt.
func TestFirstExitVerdictInterleavesBetweenRun1OutputAndRun2(t *testing.T) {
	run1Start := firstAppStart
	run1Exit := firstAppStart.Add(4 * time.Second) // whole second: 22:15:02Z
	run2Start := run1Exit.Add(1 * time.Second)     // next second:  22:15:03Z
	run3Start := run1Exit.Add(12 * time.Second)
	closedAt := run1Exit.Add(30 * time.Second)
	code1, code3 := int32(1), int32(3)

	for _, tc := range []struct {
		name      string
		announces bool
		startCmd  string
		appLines  []LogEntry
		wantOrder []string
	}{
		{
			name: "announcing native image", announces: true, startCmd: "npm start",
			appLines: []LogEntry{
				typedLine(run1Start.Add(120*time.Millisecond), LogTypeApp, rendersRunning),
				typedLine(run1Exit.Add(350*time.Millisecond), LogTypeApp, "Error: boom on run 1"),
				typedLine(run2Start.Add(100*time.Millisecond), LogTypeApp, rendersRunning),
				typedLine(run2Start.Add(400*time.Millisecond), LogTypeApp, "Error: boom on run 2"),
				typedLine(run3Start.Add(100*time.Millisecond), LogTypeApp, rendersRunning),
				typedLine(run3Start.Add(400*time.Millisecond), LogTypeApp, "Error: boom on run 3"),
			},
			wantOrder: []string{
				"==> Build queued", rolloutBuilding, rendersDeploying, rendersWebConc1,
				rendersRunning, "Error: boom on run 1",
				rendersExited1, rendersTroubleshoot,
				rendersRunning, "Error: boom on run 2",
				rendersRunning, "Error: boom on run 3",
			},
		},
		{
			name: "Dockerfile service", announces: false, startCmd: "",
			appLines: []LogEntry{
				typedLine(run1Exit.Add(450*time.Millisecond), LogTypeApp, "docker-failed run 1"),
				typedLine(run2Start.Add(200*time.Millisecond), LogTypeApp, "docker-failed run 2"),
				typedLine(run3Start.Add(200*time.Millisecond), LogTypeApp, "docker-failed run 3"),
			},
			wantOrder: []string{
				"==> Build queued", rolloutBuilding, rendersDeploying, rendersWebConc1,
				"docker-failed run 1",
				rendersExited1, rendersTroubleshoot,
				"docker-failed run 2",
				"docker-failed run 3",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := nativeRollout("update_failed")
			d.FinishedAt = closedAt
			d.StartCommand = tc.startCmd
			d.InstanceAnnounces = tc.announces
			d.AppStartedAt = []time.Time{run1Start, run2Start, run3Start}
			d.FirstExitAt, d.FirstExitCode = run1Exit, &code1
			d.ExitCode = &code3

			app := sampleApp("web")
			app.Spec.Repo = rolloutRepo
			svc := newService(nil, app, podFor("web", "web-1"))
			svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
				var in []LogEntry
				for _, e := range tc.appLines {
					if q.keep(e) && (len(q.Types) == 0 || slices.Contains(q.Types, e.Labels[LabelType])) {
						in = append(in, e)
					}
				}
				return in, nil
			}
			svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }

			// Mixed (interleaved) read over [CreatedAt, FinishedAt].
			mixed, err := svc.QueryLogs(context.Background(), LogQuery{
				App: "web", Types: []string{LogTypeApp, LogTypeBuild}, Since: rolloutCreated, End: closedAt,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := inOrder(mixed); !slices.Equal(got, tc.wantOrder) {
				t.Fatalf("interleaved:\n got %q\nwant %q", got, tc.wantOrder)
			}

			// type=build read carries the verdict once at run 1's exit, no Running lines, nothing at close.
			buildOnly := buildRead(t, svc, LogQuery{Since: rolloutCreated, End: closedAt})
			wantBuild := []string{"==> Build queued", rolloutBuilding, rendersDeploying, rendersWebConc1, rendersExited1, rendersTroubleshoot}
			if got := inOrder(buildOnly); !slices.Equal(got, wantBuild) {
				t.Fatalf("type=build:\n got %q\nwant %q", got, wantBuild)
			}
			for _, e := range buildOnly {
				if e.Message == rendersExited1 || e.Message == rendersTroubleshoot {
					at, _ := time.Parse(time.RFC3339Nano, e.Timestamp)
					if !at.Before(run2Start) || !at.Before(closedAt) {
						t.Fatalf("verdict %q at %s must precede run2Start %s and FinishedAt %s", e.Message, at, run2Start, closedAt)
					}
				}
			}
		})
	}
}

// w32/m591: suppression and fallback rules — never emitted while open or for
// eventually-live deploys; exit code 0 is preserved; OOMKilled, TimedOut,
// build_failed, pre_deploy_failed, canceled, and legacy update_failed rows
// without FirstExitAt keep their exact existing closing lines.
func TestFirstExitVerdictSuppressionAndFallbacks(t *testing.T) {
	web := progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService}
	built := []string{"==> Build queued", rolloutBuilding}
	exit0, exit1, exit2, exit137 := int32(0), int32(1), int32(2), int32(137)
	run1Exit := firstAppStart.Add(2 * time.Second)
	with := func(d DeployProgress, fn func(*DeployProgress)) DeployProgress { fn(&d); return d }

	for _, tc := range []struct {
		name string
		row  DeployProgress
		want []string
	}{
		{
			"update_in_progress with FirstExitAt emits no verdict",
			with(nativeRollout("update_in_progress"), func(d *DeployProgress) {
				d.FinishedAt = time.Time{}
				d.FirstExitAt, d.FirstExitCode = run1Exit, &exit1
			}),
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning}),
		},
		{
			"eventually-live deploy with FirstExitAt emits no verdict",
			with(nativeRollout("live"), func(d *DeployProgress) {
				d.FirstExitAt, d.FirstExitCode = run1Exit, &exit1
			}),
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersLive}),
		},
		{
			"deactivated deploy with FirstExitAt emits no verdict",
			with(nativeRollout("deactivated"), func(d *DeployProgress) {
				d.FirstExitAt, d.FirstExitCode = run1Exit, &exit1
			}),
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersLive}),
		},
		{
			"exit code 0 is preserved at first exit",
			with(nativeRollout("update_failed"), func(d *DeployProgress) {
				d.FirstExitAt, d.FirstExitCode = run1Exit, &exit0
				d.ExitCode = &exit1
			}),
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning,
				renderlog.PlatformFailure.Line("Exited with status 0"), rendersTroubleshoot}),
		},
		{
			"OOMKilled keeps existing closing line even if FirstExitAt was recorded",
			with(nativeRollout("update_failed"), func(d *DeployProgress) {
				d.FirstExitAt, d.FirstExitCode = run1Exit, &exit1
				d.ExitCode, d.OOMKilled = &exit137, true
				d.FailureReason = "container was killed for memory"
			}),
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, "==> Deploy failed: container was killed for memory"}),
		},
		{
			"TimedOut keeps Timed Out at FinishedAt even if FirstExitAt was recorded",
			with(nativeRollout("update_failed"), func(d *DeployProgress) {
				d.FirstExitAt, d.FirstExitCode = run1Exit, &exit1
				d.TimedOut = true
			}),
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersTimedOut, rendersTroubleshoot}),
		},
		{
			"legacy update_failed row with zero FirstExitAt keeps verdict at FinishedAt",
			with(nativeRollout("update_failed"), func(d *DeployProgress) {
				d.AppStartedAt = []time.Time{firstAppStart, firstAppStart.Add(2 * time.Second)}
				d.ExitCode = &exit2
			}),
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersRunning,
				renderlog.PlatformFailure.Line("Exited with status 2"), rendersTroubleshoot}),
		},
		{
			"canceled deploy with FirstExitAt keeps Deploy canceled",
			with(nativeRollout("canceled"), func(d *DeployProgress) {
				d.FirstExitAt, d.FirstExitCode = run1Exit, &exit1
			}),
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, "==> Deploy canceled"}),
		},
		{
			"prebuilt image update_failed places verdict at first exit",
			with(nativeRollout("update_failed"), func(d *DeployProgress) {
				d.Built, d.Commit, d.Image, d.StartCommand = false, "", "docker.io/library/busybox:1.36", ""
				d.FirstExitAt, d.FirstExitCode = run1Exit, &exit1
				d.ExitCode = &exit2
			}),
			[]string{"==> Deploy queued", "==> Deploying image docker.io/library/busybox:1.36", rendersWebConc1, rendersExited1, rendersTroubleshoot},
		},
		{
			"FinishedAt in same second as FirstExitAt clamps verdict to FinishedAt",
			with(nativeRollout("update_failed"), func(d *DeployProgress) {
				d.FirstExitAt, d.FirstExitCode = rolloutClose.Truncate(time.Second), &exit1
				d.ExitCode = &exit1
			}),
			slices.Concat(built, []string{rendersDeploying, rendersWebConc1, rendersRunning, rendersExited1, rendersTroubleshoot}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := story(tc.row, web); !slices.Equal(got, tc.want) {
				t.Fatalf("got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// w32/m591: a row first seen update_failed with FirstExitAt < FinishedAt while
// its build pod is still draining holds back the first-exit verdict until the
// build's last line, then emits it once.
func TestBuildTailHoldsFirstExitVerdictUntilTheBuildsLastLine(t *testing.T) {
	b := startBuildTail(t)
	b.print("==> Using Node 24")
	b.expect("==> Using Node 24")
	code := int32(1)
	b.update(func(d *DeployProgress) {
		d.Status, d.Built = "update_failed", true
		d.StartedAt = d.CreatedAt.Add(time.Second)
		d.RolloutStartedAt = d.CreatedAt.Add(20 * time.Second)
		d.WebConcurrencyDefault = 1
		d.InstanceAnnounces = true
		d.FirstExitAt, d.FirstExitCode = d.CreatedAt.Add(25*time.Second).Truncate(time.Second), &code
		d.ExitCode = &code
		d.FinishedAt = d.CreatedAt.Add(time.Minute)
	})
	b.expect(buildingFrom)
	b.expect(rendersDeploying)
	b.expect(rendersWebConc1)
	b.quiet()
	b.print("==> Build successful 🎉")
	b.expect("==> Build successful 🎉")
	if err := b.out.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-b.done:
		if err != nil {
			t.Fatalf("tail ended with %v, want clean end", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the tail did not end")
	}
	var rest []string
	for len(b.got) > 0 {
		rest = append(rest, <-b.got)
	}
	want := []string{rendersExited1, rendersTroubleshoot}
	if !slices.Equal(rest, want) {
		t.Fatalf("after the build's last line the tail sent %q\nwant %q", rest, want)
	}
}

// w32/m591: the live WebSocket build tail emits no Exited line while the
// deploy is update_in_progress (even after FirstExitAt is recorded), emits the
// pair once when the deploy settles update_failed, and matches history IDs.
func TestWebSocketBuildTailSendsFirstExitVerdictOnceOnUpdateFailed(t *testing.T) {
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
	row := DeployProgress{
		ID: "dep-fe", Generation: 4, Status: deployStatusBuildInProgress, Built: true, Commit: "abc1234def5678",
		CreatedAt: soon(30 * time.Minute), StartedAt: soon(31 * time.Minute),
	}
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
	ch := make(chan renderLog, 64)
	go func() {
		defer close(ch)
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var line renderLog
			if err := json.Unmarshal(payload, &line); err == nil {
				ch <- line
			}
		}
	}()
	var got []string
	ids := map[string]string{}
	readUntil := func(want string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for !slices.Contains(got, want) {
			select {
			case line, ok := <-ch:
				if !ok {
					t.Fatalf("stream closed waiting for %q after %q", want, got)
				}
				got = append(got, line.Message)
				ids[line.Message] = line.ID
			case <-deadline:
				t.Fatalf("timed out waiting for %q after %q", want, got)
			}
		}
	}
	drain := func(d time.Duration) {
		timer := time.NewTimer(d)
		defer timer.Stop()
		for {
			select {
			case line, ok := <-ch:
				if !ok {
					return
				}
				got = append(got, line.Message)
				ids[line.Message] = line.ID
			case <-timer.C:
				return
			}
		}
	}
	readUntil("==> Build successful 🎉")

	code1, code2 := int32(1), int32(2)
	update(func(d *DeployProgress) {
		d.Status = "update_in_progress"
		d.RolloutStartedAt = soon(70 * time.Minute)
		d.StartCommand = "npm start"
		d.InstanceAnnounces = true
		d.WebConcurrencyDefault = 1
		d.FirstExitAt = soon(71 * time.Minute).Truncate(time.Second)
		d.FirstExitCode = &code1
	})
	readUntil(rendersWebConc1)
	// Confirm no Exited line leaks while still update_in_progress.
	drain(100 * time.Millisecond)
	if slices.Contains(got, rendersExited1) || slices.Contains(got, rendersTroubleshoot) {
		t.Fatalf("verdict leaked while update_in_progress: %q", got)
	}

	update(func(d *DeployProgress) {
		d.Status = "update_failed"
		d.ExitCode = &code2
		d.FinishedAt = soon(72 * time.Minute)
	})
	readUntil(rendersTroubleshoot)
	// Let more polls run and verify no duplicate verdict.
	drain(150 * time.Millisecond)

	want := []string{rendersDeploying, rendersWebConc1, rendersExited1, rendersTroubleshoot}
	var sent []string
	for _, m := range got {
		if slices.Contains(want, m) {
			sent = append(sent, m)
		}
	}
	if !slices.Equal(sent, want) {
		t.Fatalf("tail sent %q, want %q once each (all %q)", sent, want, got)
	}

	history, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}, Since: opened, End: soon(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	read := map[string]string{}
	for _, e := range history {
		read[e.Message] = logID(e)
	}
	for _, m := range want {
		if read[m] == "" || read[m] != ids[m] {
			t.Errorf("%q: history id %q, tail id %q", m, read[m], ids[m])
		}
	}
}

// w32/m591: REST /v1/logs, GraphQL logs and MCP list_logs return the exact
// same message bytes (ANSI forms included), IDs and timestamps for the
// first-exit verdict pair.
func TestFirstExitVerdictReadsTheSameOnRESTGraphQLAndMCP(t *testing.T) {
	run1Start := firstAppStart
	run1Exit := firstAppStart.Add(4 * time.Second)
	run2Start := run1Exit.Add(2 * time.Second)
	closedAt := run1Exit.Add(30 * time.Second)
	code1 := int32(1)

	d := nativeRollout("update_failed")
	d.FinishedAt = closedAt
	d.InstanceAnnounces = true
	d.AppStartedAt = []time.Time{run1Start, run2Start}
	d.FirstExitAt, d.FirstExitCode = run1Exit, &code1
	d.ExitCode = &code1

	entries := []LogEntry{
		typedLine(run1Start.Add(100*time.Millisecond), LogTypeApp, rendersRunning),
		typedLine(run1Exit.Add(400*time.Millisecond), LogTypeApp, "run-1-error"),
		typedLine(run2Start.Add(100*time.Millisecond), LogTypeApp, rendersRunning),
	}
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	svc := newService(nil, app, podFor("web", "web-1"))
	svc.PodLogs, svc.PodLogsFollow = nil, nil
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		var in []LogEntry
		for _, e := range entries {
			if q.keep(e) && (len(q.Types) == 0 || slices.Contains(q.Types, e.Labels[LabelType])) {
				e.Labels = maps.Clone(e.Labels)
				in = append(in, e)
			}
		}
		return in, nil
	}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }
	start, end := rolloutCreated.Format(time.RFC3339), closedAt.Format(time.RFC3339)

	type verdictRow struct{ id, ts, msg string }
	extract := func(rows []verdictRow) []verdictRow {
		var out []verdictRow
		for _, r := range rows {
			if r.msg == rendersExited1 || r.msg == rendersTroubleshoot {
				out = append(out, r)
			}
		}
		return out
	}

	// REST
	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&type=build&startTime="+start+"&endTime="+end)
	var restEnv renderLogList
	if err := json.Unmarshal(rec.Body.Bytes(), &restEnv); err != nil {
		t.Fatalf("REST = %d %s (%v)", rec.Code, rec.Body, err)
	}
	var restRows []verdictRow
	for _, l := range restEnv.Logs {
		restRows = append(restRows, verdictRow{l.ID, l.Timestamp, l.Message})
	}
	restVerdict := extract(restRows)
	if len(restVerdict) != 2 {
		t.Fatalf("REST verdict = %+v, want 2 lines", restVerdict)
	}

	// GraphQL
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", type:"build", startTime:"`+start+`", endTime:"`+end+`") { logs { id timestamp message } } }`)
	var gqlRows []verdictRow
	for _, row := range data["logs"].(map[string]any)["logs"].([]any) {
		m := row.(map[string]any)
		gqlRows = append(gqlRows, verdictRow{m["id"].(string), m["timestamp"].(string), m["message"].(string)})
	}
	if gqlVerdict := extract(gqlRows); !slices.Equal(gqlVerdict, restVerdict) {
		t.Fatalf("GraphQL verdict %+v != REST %+v", gqlVerdict, restVerdict)
	}

	// MCP
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
	var mcpEnv renderLogList
	rendertest.DecodeMCPText(t, res, &mcpEnv)
	var mcpRows []verdictRow
	for _, l := range mcpEnv.Logs {
		mcpRows = append(mcpRows, verdictRow{l.ID, l.Timestamp, l.Message})
	}
	if mcpVerdict := extract(mcpRows); !slices.Equal(mcpVerdict, restVerdict) {
		t.Fatalf("MCP verdict %+v != REST %+v", mcpVerdict, restVerdict)
	}
}
