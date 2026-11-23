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

	"github.com/bex-co/bex/lego/types/renderlog"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// Render's pre-deploy lines (w32/m572, w32/091), restated so the tests pin the
// wire text, for a step whose command is `npx prisma migrate deploy`.
const (
	rendersStepStart    = "==> Starting pre-deploy: npx prisma migrate deploy"
	rendersStepRunning  = "\x1b[32m\x1b[1m==>\x1b(B\x1b[m \x1b[1mRunning 'npx prisma migrate deploy'\x1b(B\x1b[m"
	rendersStepComplete = "==> Pre-deploy complete!"
	rendersStepFailed   = "==> Pre-deploy has failed"
	rendersStepTimedOut = "==> Pre-deploy timed out"
	rendersExited128    = "\x1b[0;31m\x1b[1m==> Exited with status 128\x1b[0m"
	stepCommand         = "npx prisma migrate deploy"
)

// The step's moments, from Unwrenchable/ATOMIC-FIZZ-CAPS-VAULT-77-WASTELAND-GPS#82's
// Render log: the build ends at 11:29:11.25, the step starts at 11:29:14.99,
// its container at 11:29:27 (the kubelet's whole second of 27.17), the step
// fails at 11:29:31.49, and the deploy closes at 11:29:32.47.
var (
	stepBuildEnd  = time.Date(2026, 11, 22, 11, 29, 11, 250000000, time.UTC)
	stepLaunch    = time.Date(2026, 11, 22, 11, 29, 14, 990000000, time.UTC)
	stepContainer = time.Date(2026, 11, 22, 11, 29, 27, 0, time.UTC)
	stepSettled   = time.Date(2026, 11, 22, 11, 29, 31, 490000000, time.UTC)
	stepClose     = time.Date(2026, 11, 22, 11, 29, 32, 470000000, time.UTC)
)

// stepDeploy is a built native web service's deploy whose pre-deploy step
// failed with status 1, its row's record.
func stepDeploy() DeployProgress {
	code := int32(1)
	return DeployProgress{
		ID: "dep-step", Generation: 3, Status: deployStatusPreDeployFailed, Built: true, Commit: "abc1234def5678",
		CreatedAt: stepBuildEnd.Add(-90 * time.Second), StartedAt: stepBuildEnd.Add(-80 * time.Second), FinishedAt: stepClose,
		FailureReason: "the pre-deploy command exited with code 1; check the pre-deploy logs",
		PreDeploy: PreDeployRun{Status: "failed", StartedAt: stepLaunch, ContainerStartedAt: stepContainer, FinishedAt: stepSettled,
			Command: stepCommand, Native: true, ExitCode: &code},
	}
}

// stepPassed is stepDeploy whose step passed, then rolled out and went live.
func stepPassed() DeployProgress {
	d := stepDeploy()
	zero := int32(0)
	d.Status, d.FailureReason, d.FinishedAt = "live", "", stepSettled.Add(20*time.Second)
	d.PreDeploy.Status, d.PreDeploy.ExitCode = preDeploySucceeded, &zero
	d.RolloutStartedAt, d.AppStartedAt = stepSettled.Add(50*time.Millisecond), []time.Time{stepSettled.Add(9 * time.Second).Truncate(time.Second)}
	d.StartCommand, d.WebConcurrencyDefault = "npm start", 1
	return d
}

// w32/m572: Render's log narrates the pre-deploy step between the build's end
// and the rollout: `==> Starting pre-deploy: <command>`, `==> Running
// '<command>'` on a native runtime only (ClipFarmVB/ClipFarm#173 and
// ganjasan/mapsurvey#72 are Docker steps with no such line), then `==>
// Pre-deploy complete!`, or `==> Pre-deploy has failed`, `==> Exited with
// status N` and the troubleshooting line. Failures Render's logs do not show
// keep bex's own line, and every line comes from a moment the operator
// recorded.
func TestPreDeployLinesFollowRendersLogByOutcomeLaunchAndServiceType(t *testing.T) {
	web := progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService}
	built := []string{"==> Build queued", rolloutBuilding}
	with := func(d DeployProgress, change func(*DeployProgress)) DeployProgress {
		change(&d)
		return d
	}
	failedWith := func(reason string, change func(*PreDeployRun)) DeployProgress {
		return with(stepDeploy(), func(d *DeployProgress) {
			d.FailureReason = reason
			change(&d.PreDeploy)
		})
	}
	code := func(c int32) *int32 { return &c }
	exited := []string{rendersStepFailed, rendersExited1, rendersTroubleshoot}
	for _, c := range []struct {
		name string
		row  DeployProgress
		pc   progressContext
		want []string
	}{
		{"a native step that exited 1", stepDeploy(), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepRunning}, exited)},
		{"a native step that exited 127", failedWith("the pre-deploy command exited with code 127; check the pre-deploy logs",
			func(r *PreDeployRun) { r.ExitCode = code(127) }), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepRunning, rendersStepFailed, renderlog.PlatformFailure.Line("Exited with status 127"), rendersTroubleshoot})},
		{"a Dockerfile step, which gets no Running line", failedWith(stepDeploy().FailureReason, func(r *PreDeployRun) { r.Native = false }), web,
			slices.Concat(built, []string{rendersStepStart}, exited)},
		{"a prebuilt image's step", with(stepDeploy(), func(d *DeployProgress) {
			d.Built, d.Commit, d.Image, d.PreDeploy.Native = false, "", "docker.io/library/node:22", false
		}), progressContext{serviceType: appv1alpha1.TypeWebService},
			slices.Concat([]string{"==> Deploy queued", "==> Deploying image docker.io/library/node:22", rendersStepStart}, exited)},
		{"a private service's step", stepDeploy(), progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypePrivateService},
			slices.Concat(built, []string{rendersStepStart, rendersStepRunning}, exited)},
		{"a background worker's step", stepDeploy(), progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeBackgroundWorker},
			slices.Concat(built, []string{rendersStepStart, rendersStepRunning}, exited)},
		{"a step that passed, then the rollout", stepPassed(), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepRunning, rendersStepComplete,
				rendersDeploying, rendersWebConc1, rendersRunning, rendersLive})},
		{"a passing step whose rollout still runs", with(stepPassed(), func(d *DeployProgress) {
			d.Status, d.FinishedAt, d.RolloutStartedAt, d.AppStartedAt = "update_in_progress", time.Time{}, time.Time{}, nil
		}), web, slices.Concat(built, []string{rendersStepStart, rendersStepRunning, rendersStepComplete})},
		{"a step still running", with(stepDeploy(), func(d *DeployProgress) {
			d.Status, d.FinishedAt, d.FailureReason = "pre_deploy_in_progress", time.Time{}, ""
			d.PreDeploy.Status, d.PreDeploy.FinishedAt, d.PreDeploy.ExitCode = "running", time.Time{}, nil
		}), web, slices.Concat(built, []string{rendersStepStart, rendersStepRunning})},
		{"a step whose container has not started", with(stepDeploy(), func(d *DeployProgress) {
			d.Status, d.FinishedAt, d.FailureReason = "pre_deploy_in_progress", time.Time{}, ""
			d.PreDeploy = PreDeployRun{Status: "running", StartedAt: stepLaunch, Command: stepCommand, Native: true}
		}), web, slices.Concat(built, []string{rendersStepStart})},
		{"a step killed for memory keeps bex's line", failedWith("the pre-deploy command ran out of memory (exit code 137); check the pre-deploy logs",
			func(r *PreDeployRun) { r.ExitCode, r.OOMKilled = code(137), true }), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepRunning,
				"==> Pre-deploy failed: the pre-deploy command ran out of memory (exit code 137); check the pre-deploy logs"})},
		{"a native step stopped at its 30-minute limit closes with Render's timed out line", failedWith("the pre-deploy command did not finish within its 30-minute window; check the pre-deploy logs",
			func(r *PreDeployRun) { r.ExitCode = nil }), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepRunning, rendersStepTimedOut})},
		{"a Docker step stopped at its 30-minute limit closes with Render's timed out line", failedWith("the pre-deploy command did not finish within its 30-minute window; check the pre-deploy logs",
			func(r *PreDeployRun) { r.Native, r.ExitCode = false, nil }), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepTimedOut})},
		{"a step closed by the control-plane backstop keeps bex's line", failedWith(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployTimedOut),
			func(r *PreDeployRun) { r.FinishedAt, r.ExitCode = time.Time{}, nil }), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepRunning,
				"==> Pre-deploy failed: " + appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployTimedOut)})},
		{"an older-operator row stopped at its window keeps bex's line", failedWith("the pre-deploy command did not finish within its 30-minute window; check the pre-deploy logs",
			func(r *PreDeployRun) { *r = PreDeployRun{Status: "failed"} }), web,
			slices.Concat(built, []string{"==> Pre-deploy failed: the pre-deploy command did not finish within its 30-minute window; check the pre-deploy logs"})},
		{"an image that could not be pulled keeps bex's line", failedWith("image pull is failing: not found; the pre-deploy command never ran",
			func(r *PreDeployRun) { r.ContainerStartedAt, r.ExitCode = time.Time{}, nil }), web,
			slices.Concat(built, []string{rendersStepStart, "==> Pre-deploy failed: image pull is failing: not found; the pre-deploy command never ran"})},
		{"a program the runtime could not launch closes with status 128", failedWith(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployCouldNotLaunch),
			func(r *PreDeployRun) { r.ContainerStartedAt, r.ExitCode = time.Time{}, nil }), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepFailed, rendersExited128, rendersTroubleshoot})},
		{"an image step whose named program does not exist closes with status 128", with(stepDeploy(), func(d *DeployProgress) {
			d.Built, d.Commit, d.Image = false, "", "docker.io/library/node:22"
			d.FailureReason = appv1alpha1.PreDeployCouldNotStartPrefix + `the program "release" does not exist in this image` + appv1alpha1.PreDeployNeverRanSuffix
			d.PreDeploy.Command, d.PreDeploy.Native, d.PreDeploy.ContainerStartedAt, d.PreDeploy.ExitCode = "release", false, time.Time{}, nil
		}), progressContext{serviceType: appv1alpha1.TypeWebService},
			[]string{"==> Deploy queued", "==> Deploying image docker.io/library/node:22", "==> Starting pre-deploy: release",
				rendersStepFailed, rendersExited128, rendersTroubleshoot}},
		{"a program that is not executable closes with status 128", failedWith(
			appv1alpha1.PreDeployCouldNotStartPrefix+`the program "./migrate" is not executable in this image`+appv1alpha1.PreDeployNeverRanSuffix,
			func(r *PreDeployRun) { r.Native, r.ContainerStartedAt, r.ExitCode = false, time.Time{}, nil }), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepFailed, rendersExited128, rendersTroubleshoot})},
		{"a program the runtime could not execute closes with status 128", failedWith(
			appv1alpha1.PreDeployCouldNotStartPrefix+`the container runtime could not execute "./migrate"`+appv1alpha1.PreDeployNeverRanSuffix,
			func(r *PreDeployRun) { r.Native, r.ContainerStartedAt, r.ExitCode = false, time.Time{}, nil }), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepFailed, rendersExited128, rendersTroubleshoot})},
		{"an older-operator row that could not launch keeps bex's line", failedWith(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployCouldNotLaunch),
			func(r *PreDeployRun) { *r = PreDeployRun{Status: "failed"} }), web,
			slices.Concat(built, []string{"==> Pre-deploy failed: " + appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployCouldNotLaunch)})},
		{"no build minutes left: refused before it launched", failedWith(appv1alpha1.BuildMinutesExhaustedMessage,
			func(r *PreDeployRun) { *r = PreDeployRun{Status: "failed", FinishedAt: stepSettled} }), web,
			slices.Concat(built, []string{"==> Pre-deploy failed: " + appv1alpha1.BuildMinutesExhaustedMessage})},
		{"an infrastructure fault before it launched", failedWith("the pre-deploy command could not be started: the platform failed",
			func(r *PreDeployRun) { *r = PreDeployRun{Status: "failed", FinishedAt: stepSettled} }), web,
			slices.Concat(built, []string{"==> Pre-deploy failed: the pre-deploy command could not be started: the platform failed"})},
		{"a step that exited 0 yet failed keeps bex's line", failedWith("the pre-deploy command failed; check the pre-deploy logs",
			func(r *PreDeployRun) { r.ExitCode = code(0) }), web,
			slices.Concat(built, []string{rendersStepStart, rendersStepRunning, "==> Pre-deploy failed: the pre-deploy command failed; check the pre-deploy logs"})},
		{"a deploy canceled while its step ran", with(stepDeploy(), func(d *DeployProgress) {
			d.Status, d.FailureReason = "canceled", ""
			d.PreDeploy.Status, d.PreDeploy.FinishedAt, d.PreDeploy.ExitCode = "canceled", time.Time{}, nil
		}), web, slices.Concat(built, []string{rendersStepStart, rendersStepRunning, "==> Deploy canceled"})},
		{"a row an older operator recorded keeps bex's line", failedWith(stepDeploy().FailureReason,
			func(r *PreDeployRun) { *r = PreDeployRun{Status: "failed"} }), web,
			slices.Concat(built, []string{"==> Pre-deploy failed: the pre-deploy command exited with code 1; check the pre-deploy logs"})},
		{"a static site runs no step", with(nativeRollout("live"), func(d *DeployProgress) {
			d.RolloutStartedAt, d.AppStartedAt, d.StartCommand, d.WebConcurrencyDefault = time.Time{}, nil, "", 0
		}), progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeStaticSite},
			slices.Concat(built, []string{"==> Your site is live 🎉"})},
		{"a cron job runs no step", with(nativeRollout("live"), func(d *DeployProgress) {
			d.RolloutStartedAt, d.AppStartedAt, d.StartCommand, d.WebConcurrencyDefault = time.Time{}, nil, "", 0
		}), progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeCronJob},
			slices.Concat(built, []string{"==> Cron job deployed — runs on schedule 🎉"})},
	} {
		if got := story(c.row, c.pc); !slices.Equal(got, c.want) {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// Each line carries the moment the operator recorded for it: the launch, the
// container's start, and the moment the verdict was settled; the deploy's own
// close carries its verdict.
func TestPreDeployLinesCarryTheRecordedMoments(t *testing.T) {
	stamp := func(t time.Time) string { return t.Format(time.RFC3339Nano) + " " }
	var got []string
	for _, e := range progressLines(stepDeploy(), progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService}) {
		got = append(got, e.Timestamp+" "+e.Message)
	}
	want := []string{
		stamp(stepLaunch) + rendersStepStart,
		stamp(stepContainer) + rendersStepRunning,
		stamp(stepSettled) + rendersStepFailed,
		stamp(stepClose) + rendersExited1,
		stamp(stepClose) + rendersTroubleshoot,
	}
	if !slices.Equal(got[2:], want) {
		t.Fatalf("lines:\n got %q\nwant %q", got[2:], want)
	}

	// w32/091: a step stopped at its 30-minute limit closes with `==> Pre-deploy
	// timed out` at the step's end (stepSettled), with nothing at stepClose.
	timedOut := stepDeploy()
	timedOut.FailureReason = appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployWindowExceeded,
		appv1alpha1.DeployReasonParam{Name: "minutes", Value: "30"})
	timedOut.PreDeploy.ExitCode = nil
	got = nil
	for _, e := range progressLines(timedOut, progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService}) {
		got = append(got, e.Timestamp+" "+e.Message)
	}
	wantTimedOut := []string{
		stamp(stepLaunch) + rendersStepStart,
		stamp(stepContainer) + rendersStepRunning,
		stamp(stepSettled) + rendersStepTimedOut,
	}
	if !slices.Equal(got[2:], wantTimedOut) {
		t.Fatalf("timed out lines:\n got %q\nwant %q", got[2:], wantTimedOut)
	}

	// w32/091: a program the runtime could not launch writes no Running line,
	// stamps `==> Pre-deploy has failed` at stepSettled, and closes with
	// `==> Exited with status 128` and the troubleshooting line at stepClose.
	launch := stepDeploy()
	launch.FailureReason = appv1alpha1.PreDeployCouldNotStartPrefix + `the program "release" does not exist in this image` + appv1alpha1.PreDeployNeverRanSuffix
	launch.PreDeploy.Command, launch.PreDeploy.Native, launch.PreDeploy.ContainerStartedAt, launch.PreDeploy.ExitCode = "release", false, time.Time{}, nil
	got = nil
	for _, e := range progressLines(launch, progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService}) {
		got = append(got, e.Timestamp+" "+e.Message)
	}
	wantLaunch := []string{
		stamp(stepLaunch) + "==> Starting pre-deploy: release",
		stamp(stepSettled) + rendersStepFailed,
		stamp(stepClose) + rendersExited128,
		stamp(stepClose) + rendersTroubleshoot,
	}
	if !slices.Equal(got[2:], wantLaunch) {
		t.Fatalf("launch failure lines:\n got %q\nwant %q", got[2:], wantLaunch)
	}
}

// A container that starts in the step's first second, whose whole-second
// start reads before the launch, and a verdict the operator's clock stamped
// a hair after the row's close keep their order and stay inside the deploy.
func TestPreDeployLinesKeepTheirOrderInsideTheDeploy(t *testing.T) {
	d := stepDeploy()
	d.PreDeploy.ContainerStartedAt = stepLaunch.Truncate(time.Second)
	d.PreDeploy.FinishedAt = stepClose.Add(3 * time.Millisecond)
	lines := progressLines(d, progressContext{repo: rolloutRepo, serviceType: appv1alpha1.TypeWebService})
	var prev time.Time
	var order []string
	for _, e := range lines {
		at, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil {
			t.Fatal(err)
		}
		if at.Before(prev) || at.After(stepClose) {
			t.Fatalf("%q at %s: out of order or past the close %s: %v", e.Message, e.Timestamp, stepClose, lines)
		}
		prev = at
		order = append(order, e.Message)
	}
	if !slices.Equal(order[2:], []string{rendersStepStart, rendersStepRunning, rendersStepFailed, rendersExited1, rendersTroubleshoot}) {
		t.Fatalf("order = %q", order)
	}
	if running := lines[3].Timestamp; running != stepLaunch.Add(rolloutLineStep).Format(time.RFC3339Nano) {
		t.Fatalf("Running at %s, want held just after the launch", running)
	}
}

// stepOutput is the step's own output as the shipper files it: a build line
// of the step's Job pod, stamped by the pod in nanoseconds.
func stepOutput(at time.Time, msg string) LogEntry {
	return LogEntry{Timestamp: at.Format(time.RFC3339Nano), Message: msg,
		Labels: map[string]string{LabelType: LogTypeBuild, LabelInstance: "predeploy-web-gen-3-x7k2p", "container": "predeploy"}}
}

// The history read orders the lines by their moments around the build's and
// the step's own output: the start after the build's last line, the command's
// output after Running, the verdict after the output's last line, and for a
// passing step `==> Pre-deploy complete!` before `==> Deploying...`.
func TestPreDeployLinesReadAroundTheBuildAndTheStepsOutput(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	read := func(d DeployProgress, output ...LogEntry) []string {
		t.Helper()
		svc := newService(nil, app)
		svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return output, nil }
		svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }
		got := inOrder(buildRead(t, svc, LogQuery{Since: d.CreatedAt, End: d.FinishedAt.Add(time.Minute)}))
		at := slices.Index(got, "==> Build successful 🎉")
		if at < 0 {
			t.Fatalf("log = %q, no build line", got)
		}
		return got[at:]
	}
	// The build's last line in the launch's own second, and the command's
	// output in its container's first second and just before the verdict.
	buildEnd := stepOutput(stepLaunch.Add(-200*time.Millisecond), "==> Build successful 🎉")
	buildEnd.Labels[LabelInstance], buildEnd.Labels["container"] = "bld-web-gen-3-a1b2c", "buildkit"
	first := stepOutput(stepContainer.Add(170*time.Millisecond), "Prisma schema loaded from prisma/schema.prisma")
	last := stepOutput(stepSettled.Add(-400*time.Millisecond), "Error: P3009 migrate found failed migrations")

	if got, want := read(stepDeploy(), buildEnd, first, last), []string{"==> Build successful 🎉", rendersStepStart, rendersStepRunning,
		first.Message, last.Message, rendersStepFailed, rendersExited1, rendersTroubleshoot}; !slices.Equal(got, want) {
		t.Fatalf("a failed step reads\n %q\nwant %q", got, want)
	}
	if strings.Contains(strings.Join(read(stepDeploy(), buildEnd, first, last), "\n"), rendersDeploying) {
		t.Fatal("a failed step's deploy never rolls out")
	}

	passed := stepPassed()
	done := stepOutput(stepSettled.Add(-400*time.Millisecond), "All migrations have been successfully applied.")
	if got, want := read(passed, buildEnd, first, done), []string{"==> Build successful 🎉", rendersStepStart, rendersStepRunning,
		first.Message, done.Message, rendersStepComplete, rendersDeploying, rendersWebConc1, rendersRunning, rendersLive}; !slices.Equal(got, want) {
		t.Fatalf("a passing step reads\n %q\nwant %q", got, want)
	}

	// w32/091: a step stopped at its 30-minute limit puts `==> Pre-deploy timed
	// out` right after the command's last output line, with nothing after it
	// (MalaysiaKaki/astrail-webmcp#10).
	timedOut := stepDeploy()
	timedOut.FailureReason = appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployWindowExceeded,
		appv1alpha1.DeployReasonParam{Name: "minutes", Value: "30"})
	timedOut.PreDeploy.Native, timedOut.PreDeploy.ExitCode = false, nil
	timedOut.PreDeploy.FinishedAt = stepLaunch.Add(30 * time.Minute)
	timedOut.FinishedAt = timedOut.PreDeploy.FinishedAt.Add(time.Second)
	gateOK := stepOutput(stepLaunch.Add(40*time.Second), "schema gate OK")
	if got, want := read(timedOut, buildEnd, gateOK), []string{"==> Build successful 🎉", rendersStepStart, gateOK.Message, rendersStepTimedOut}; !slices.Equal(got, want) {
		t.Fatalf("a timed-out step reads\n %q\nwant %q", got, want)
	}
}

// w32/m572 t006: the CLI's build tail (Render's WebSocket subscription, which
// follows every build) sends the step's lines once each is recorded, in order
// and each once, then the verdict; a history read over the same window gives
// them the same ids.
func TestWebSocketBuildTailSendsThePreDeployLinesAsTheyAreRecorded(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }
	svc := newService(map[string][]string{
		"bld-web-gen-3-a": {soon(time.Hour).Format(time.RFC3339Nano) + " ==> Build successful 🎉"},
	}, app)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	var mu sync.Mutex
	row := DeployProgress{ID: "dep-s", Generation: 3, Status: deployStatusBuildInProgress, Built: true, Commit: "abc1234def5678",
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
	if err := svc.Client.Create(context.Background(), buildPodFor("web", "bld-web-gen-3-a", "builds", "buildkit", opened)); err != nil {
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
		d.Status = "pre_deploy_in_progress"
		d.PreDeploy = PreDeployRun{Status: "running", StartedAt: soon(70 * time.Minute), Command: stepCommand, Native: true}
	})
	readUntil(rendersStepStart)
	update(func(d *DeployProgress) { d.PreDeploy.ContainerStartedAt = soon(71 * time.Minute).Truncate(time.Second) })
	readUntil(rendersStepRunning)
	code := int32(1)
	update(func(d *DeployProgress) {
		d.PreDeploy.Status, d.PreDeploy.FinishedAt, d.PreDeploy.ExitCode = "failed", soon(72*time.Minute), &code
		d.Status, d.FinishedAt = deployStatusPreDeployFailed, soon(72*time.Minute+time.Second)
	})
	readUntil(rendersTroubleshoot)
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

	step := []string{rendersStepStart, rendersStepRunning, rendersStepFailed, rendersExited1, rendersTroubleshoot}
	var sent []string
	for _, m := range got {
		if slices.Contains(step, m) {
			sent = append(sent, m)
		}
	}
	if !slices.Equal(sent, step) {
		t.Fatalf("tail sent %q, want %q in order, each once (all lines %q)", sent, step, got)
	}

	history, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeBuild}, Since: opened, End: soon(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	read := map[string]string{}
	for _, e := range history {
		read[e.Message] = logID(e)
	}
	for _, m := range step {
		if read[m] == "" || read[m] != ids[m] {
			t.Errorf("%q: history id %q, tail id %q", m, read[m], ids[m])
		}
	}
}

// REST /v1/logs (`bex logs --type build`), GraphQL logs (the dashboard's
// deploy log pane) and MCP list_logs read the step's lines in order, with the
// same ids.
func TestPreDeployLinesReadTheSameOnRESTGraphQLAndMCP(t *testing.T) {
	d := stepDeploy()
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	svc := newService(nil, app)
	svc.PodLogs, svc.PodLogsFollow = nil, nil
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }
	start, end := stepLaunch.Add(-time.Second).Format(time.RFC3339), stepClose.Add(time.Second).Format(time.RFC3339)
	step := []string{rendersStepStart, rendersStepRunning, rendersStepFailed, rendersExited1, rendersTroubleshoot}
	// lines is the step's ids, oldest first, once its messages read as step.
	lines := func(surface string, messages, ids []string) []string {
		t.Helper()
		if len(messages) > 0 && messages[0] != rendersStepStart {
			slices.Reverse(messages)
			slices.Reverse(ids)
		}
		if !slices.Equal(messages, step) {
			t.Fatalf("%s reads %q, want %q", surface, messages, step)
		}
		return ids
	}

	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&type=build&startTime="+start+"&endTime="+end)
	var env renderLogList
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("REST = %d %s (%v)", rec.Code, rec.Body, err)
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
