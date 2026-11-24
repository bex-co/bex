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
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/types/renderlog"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's platform lines as Render's log API returns them (w32/m576), copied
// byte for byte from ivanseibel/tretta's capture (04b-render-logs.json at
// b0f216c985), kush252/Sentry-AI's (investigation_data.jsonl at 13d4e291cc)
// and Ishan2036924/upmyrank-poc's (smoke_v0207_1_r2_063029.json).
// rendersDeploying, rendersWebConc1, rendersRunning and rendersLive are in
// progress_rollout_test.go.
const (
	rendersURLBlank     = "\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1m\x1b[0m"
	rendersURLRule      = "\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1m///////////////////////////////////////////////////////////\x1b[0m"
	rendersExited1      = "\x1b[0;31m\x1b[1m==> Exited with status 1\x1b[0m"
	rendersTimedOut     = "\x1b[0;31m\x1b[1m==> Timed Out\x1b[0m"
	rendersTroubleshoot = "\x1b[0;34m\x1b[1m==> \x1b[0m\x1b[1mCommon ways to troubleshoot your deploy: https://render.com/docs/troubleshooting-deploys\x1b[0m"
	rendersPortTimeout  = "\x1b[0;31m\x1b[1m==> Port scan timeout reached, no open ports detected. Bind your service to at least one port. " +
		"If you don't need to receive traffic on any port, create a background worker instead.\x1b[0m"
	rendersPortDocs         = "\x1b[0;31m\x1b[1m==> Docs on specifying a port: https://render.com/docs/web-services#port-binding\x1b[0m"
	rendersDetectedPort     = "\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1mDetected service running on port 8080\x1b[0m"
	rendersDetectedPortDocs = "\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1mDocs on specifying a port: https://render.com/docs/web-services#port-binding\x1b[0m"
)

// rendersURLLine is the block's URL line for url, green as the live line.
func rendersURLLine(url string) string {
	return "\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1mAvailable at your primary URL " + url + "\x1b[0m"
}

// rendersRunningLine is the native instance's line for command (tretta).
func rendersRunningLine(command string) string {
	return "\x1b[32m\x1b[1m==>\x1b(B\x1b[m \x1b[1mRunning '" + command + "'\x1b(B\x1b[m"
}

// shown is each message as a terminal shows it.
func shown(entries []LogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, displayText(e.Message))
	}
	return out
}

// formStory is every line the fixtures below narrate, a deploy of each kind
// and outcome, so that a line added to progressLines or hookLines has to be
// given its form here.
func formStory() []LogEntry {
	t0 := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	code := func(c int32) *int32 { return &c }
	repo := progressContext{repo: "https://github.com/bex-co/bex", branch: "main", serviceType: appv1alpha1.TypeWebService}
	queued := DeployProgress{ID: "dep-queued", Status: deployStatusQueued, CreatedAt: at(0)}
	waiting := repo
	waiting.wait = buildWait{Reason: appv1alpha1.ReasonBuildQueued}
	oidc := repo
	oidc.oidc = oidcRefusal{generation: 3, lines: []string{oidcRefusalLine([]string{"AWS_ROLE_ARN"})}}
	live := DeployProgress{
		ID: "dep-live", Status: "live", Generation: 3, Commit: "90aff00645fe42f6", Built: true, DetectedHTTPPort: 8080,
		CreatedAt: at(0), StartedAt: at(1), RolloutStartedAt: at(40), AppStartedAt: []time.Time{at(50)},
		StartCommand: "node main.js", WebConcurrencyDefault: 1, FinishedAt: at(60),
		LiveURLs:  []string{"https://hello.onbex.co", "https://www.hello.example"},
		PreDeploy: PreDeployRun{Status: preDeploySucceeded, StartedAt: at(20), ContainerStartedAt: at(25), FinishedAt: at(30), Command: "npm run migrate", Native: true},
	}
	scanning := DeployProgress{ID: "dep-scan", Status: "update_failed", Image: "docker.io/library/busybox:1.36",
		CreatedAt: at(0), StartedAt: at(1), PortScanSince: at(10), FinishedAt: at(10 + 15*60), TimedOut: true}
	localhost := scanning
	localhost.ID, localhost.PortScanFound, localhost.PortScanFoundAt = "dep-local", appv1alpha1.PortScanFoundLocalhost, at(10)
	noHTTP := scanning
	noHTTP.ID, noHTTP.PortScanFound, noHTTP.PortScanFoundAt = "dep-nohttp", appv1alpha1.PortScanFoundNoHTTP, at(10)
	envPort := scanning
	envPort.ID, envPort.PortScanFound, envPort.PortScanFoundAt = "dep-envport", appv1alpha1.PortScanFoundEnvPort(8080), at(10)
	exited := DeployProgress{ID: "dep-exited", Status: "update_failed", Image: "docker.io/library/node:22",
		CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(30), ExitCode: code(1)}
	stepExited := DeployProgress{ID: "dep-step", Status: deployStatusPreDeployFailed, Built: true,
		CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(40),
		PreDeploy: PreDeployRun{Status: "failed", StartedAt: at(20), FinishedAt: at(39), Command: "exit 3", ExitCode: code(3)}}
	stepTimedOut := DeployProgress{ID: "dep-step-timeout", Status: deployStatusPreDeployFailed, Built: true,
		CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(40),
		FailureReason: appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployWindowExceeded,
			appv1alpha1.DeployReasonParam{Name: "minutes", Value: "30"}),
		PreDeploy: PreDeployRun{Status: "failed", StartedAt: at(20), FinishedAt: at(39), Command: "sleep 3600"}}
	stepStartError := DeployProgress{ID: "dep-step-launch", Status: deployStatusPreDeployFailed, Built: true,
		CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(40),
		FailureReason: appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployCouldNotLaunch),
		PreDeploy:     PreDeployRun{Status: "failed", StartedAt: at(20), FinishedAt: at(39), Command: "release"}}
	buildFailed := DeployProgress{ID: "dep-build", Status: "build_failed", Built: true, CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(30),
		FailureReason: appv1alpha1.FormatBuildFailed("build command", "npm ERR! missing script: build")}
	publish := buildFailed
	publish.ID, publish.FailureReason = "dep-publish", appv1alpha1.FormatBuildFailed(appv1alpha1.PublishStep, "Publish directory dist does not exist!")
	reasons := DeployProgress{ID: "dep-reason", Status: "update_failed", Image: "docker.io/library/node:22", CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(30),
		FailureReason: "the image could not be pulled\n==> retry later"}
	var out []LogEntry
	for _, c := range []struct {
		d  DeployProgress
		pc progressContext
	}{
		{queued, waiting},
		{queued, progressContext{}},
		{live, oidc},
		{DeployProgress{ID: "dep-rollback", Status: "live", Image: "registry/app@sha256:1", CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(5)}, repo},
		{DeployProgress{ID: "dep-rollout", Status: "canceled", CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(5)}, repo},
		{DeployProgress{ID: "dep-site", Status: "live", CreatedAt: at(0), FinishedAt: at(5)}, progressContext{serviceType: appv1alpha1.TypeStaticSite}},
		{DeployProgress{ID: "dep-cron", Status: "live", CreatedAt: at(0), FinishedAt: at(5)}, progressContext{serviceType: appv1alpha1.TypeCronJob}},
		{DeployProgress{ID: "dep-bds-ok", Status: "live", Built: true, CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(5)}, progressContext{repo: "https://github.com/bex-co/bex", branch: "main", serviceType: appv1alpha1.TypeBuildSource}},
		{DeployProgress{ID: "dep-bds-cancel", Status: "canceled", CreatedAt: at(0), FinishedAt: at(5)}, progressContext{repo: "https://github.com/bex-co/bex", branch: "main", serviceType: appv1alpha1.TypeBuildSource}},
		{DeployProgress{ID: "dep-old-image", Status: "update_in_progress", Image: "registry/app@sha256:2", CreatedAt: at(0), StartedAt: at(1),
			RolloutStartedAt: at(2), AppStartedAt: []time.Time{at(3)}, StartCommand: "gunicorn app:app"}, progressContext{}},
		{scanning, progressContext{}},
		{localhost, progressContext{}},
		{noHTTP, progressContext{}},
		{envPort, progressContext{}},
		{exited, progressContext{}},
		{stepExited, repo},
		{stepTimedOut, repo},
		{stepStartError, repo},
		{buildFailed, repo},
		{publish, repo},
		{reasons, progressContext{}},
		{DeployProgress{ID: "dep-step-oom", Status: deployStatusPreDeployFailed, CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(9),
			FailureReason: "the pre-deploy command ran out of memory"}, progressContext{}},
		{DeployProgress{ID: "dep-build-infra", Status: "build_failed", CreatedAt: at(0), StartedAt: at(1), FinishedAt: at(9)}, repo},
	} {
		out = append(out, progressLines(c.d, c.pc)...)
	}
	hookLive := live
	hookLive.ID = "dep-hook"
	out = append(out, hookLines(hookLive, hookRun{generation: 3, status: appv1alpha1.PreDeployFailed, message: "exit 1", startedAt: at(61), finishedAt: at(70)})...)
	return append(out, hookLines(hookLive, hookRun{generation: 3, status: appv1alpha1.PreDeploySucceeded, startedAt: at(61), finishedAt: at(70)})...)
}

// TestEveryPlatformLineCarriesItsForm is t001's table for the control plane
// (w32/m576): a line Render's log API shows carries Render's form, and every
// other line, bex's own and Render's lines no capture shows with their codes
// (w32/085), stays plain. Each line shows the words it showed before.
func TestEveryPlatformLineCarriesItsForm(t *testing.T) {
	type line struct {
		shown string
		form  renderlog.Form
	}
	step, success, failure, instance, plain := renderlog.PlatformStep, renderlog.PlatformSuccess, renderlog.PlatformFailure, renderlog.Instance, renderlog.Plain
	want := []line{
		{"==> Build queued", plain},
		{"==> Deploy queued", plain},
		{waitLineNeutral, plain},
		{"==> Building from https://github.com/bex-co/bex@90aff00", plain},
		{"==> Building from https://github.com/bex-co/bex@main", plain},
		{"==> Deploying image registry/app@sha256:1", plain},
		{"==> Deploying image registry/app@sha256:2", plain},
		{"==> Deploying image docker.io/library/busybox:1.36", plain},
		{"==> Deploying image docker.io/library/node:22", plain},
		{"==> Rolling out the current release", plain},
		{oidcRefusalLine([]string{"AWS_ROLE_ARN"}), plain},
		{"==> Starting pre-deploy: npm run migrate", plain},
		{"==> Running 'npm run migrate'", instance},
		{preDeployCompleteLine, plain},
		{"==> Starting pre-deploy: exit 3", plain},
		{"==> Starting pre-deploy: sleep 3600", plain},
		{"==> Starting pre-deploy: release", plain},
		{preDeployFailedLine, plain},
		{preDeployTimedOutLine, plain},
		{deployingLine, step},
		{"==> Setting WEB_CONCURRENCY=1 by default, based on available CPUs in the instance", step},
		{"==> Running 'node main.js'", instance},
		{"==> Running 'gunicorn app:app'", instance},
		{portScanLine, plain},
		{portScanLocalhostLine, plain},
		{portScanNoHTTPLine, plain},
		{portScanTimeoutLine, failure},
		{portScanLocalhostTimeoutLine, plain},
		{portScanNoHTTPTimeoutLine, plain},
		{fmt.Sprintf(portScanEnvPortLine, 8080), plain},
		{fmt.Sprintf(portScanEnvPortTimeoutLine, 8080, 8080), plain},
		{"==> Detected service running on port 8080", success},
		{"==> Your service is live 🎉", success},
		{"==> Your site is live 🎉", plain},
		{"==> Cron job deployed — runs on schedule 🎉", plain},
		{"==> Build succeeded 🎉", plain},
		{"==> Build canceled", plain},
		{primaryURLBlank, success},
		{primaryURLRule, success},
		{"==> Available at your primary URL https://hello.onbex.co + 1 more domain", success},
		{"==> Exited with status 1", failure},
		{"==> Exited with status 3", failure},
		{"==> Exited with status 128", failure},
		{timedOutLine, failure},
		{buildFailedLine, plain},
		{"==> Publish directory dist does not exist!", plain},
		{troubleshootLine, step},
		{"==> Deploy failed: the image could not be pulled", plain},
		{"==> retry later", plain},
		{"==> Pre-deploy failed: the pre-deploy command ran out of memory", plain},
		{"==> Build failed", plain},
		{"==> Deploy canceled", plain},
		{hookLineRunning, plain},
		{hookLineFailed + ": exit 1", plain},
		{hookLineRetry, plain},
		{hookLineSucceeded, plain},
	}
	written := map[string]string{} // shown text → message
	for _, e := range formStory() {
		written[displayText(e.Message)] = e.Message
	}
	// The port docs line is written in three forms: plain after a scan line,
	// red after the plain wording's timeout line, and green after port
	// detection, which TestRendersPlatformLinesByteForByte checks.
	delete(written, portScanDocsLine)
	for _, w := range want {
		msg, ok := written[w.shown]
		if !ok {
			t.Errorf("the fixtures narrate no %q", w.shown)
			continue
		}
		delete(written, w.shown)
		if text := strings.TrimPrefix(w.shown, "==> "); msg != w.form.Line(text) {
			t.Errorf("%q is written %q, want %q", w.shown, msg, w.form.Line(text))
		}
	}
	for shownText, msg := range written {
		t.Errorf("no form chosen for %q (written %q): add it to this table", shownText, msg)
	}
}

// TestRendersPlatformLinesByteForByte pins the lines Render's captures show
// against the captured bytes, not the forms the code shares.
func TestRendersPlatformLinesByteForByte(t *testing.T) {
	messages := map[string]bool{}
	for _, e := range formStory() {
		messages[e.Message] = true
	}
	for _, capture := range []string{
		rendersDeploying, rendersWebConc1, rendersLive, rendersURLBlank, rendersURLRule, rendersTimedOut,
		rendersTroubleshoot, rendersPortTimeout, rendersPortDocs, rendersDetectedPort, rendersDetectedPortDocs,
		rendersExited1, rendersRunningLine("node main.js"),
		rendersURLLine("https://hello.onbex.co + 1 more domain"),
	} {
		if !messages[capture] {
			t.Errorf("no narrated line is Render's %q", capture)
		}
	}
	// The port docs line appears in three forms: plain after a scan line, red
	// after the plain timeout, and green after Detected service running on port.
	var docs []string
	for _, e := range formStory() {
		if displayText(e.Message) == portScanDocsLine {
			docs = append(docs, e.Message)
		}
	}
	if !slices.Contains(docs, rendersPortDocs) || !slices.Contains(docs, rendersDetectedPortDocs) || !slices.Contains(docs, portScanDocsLine) {
		t.Errorf("the port docs lines are %q, want plain after a scan line, %q after the timeout, and %q after port detection", docs, rendersPortDocs, rendersDetectedPortDocs)
	}
}

// A reason that quotes one of bex's coloured lines reads as its words, with
// one marker (w8/369's rule for plain markers).
func TestUnmarkedReadsAColouredLineByItsWords(t *testing.T) {
	for in, want := range map[string]string{
		renderlog.BuildRuntime.Line("Using Node.js version 22.23.1 via environment variable NODE_VERSION"): "Using Node.js version 22.23.1 via environment variable NODE_VERSION",
		renderlog.BuildShell.Line("Running build command 'npm ci'..."):                                     "Running build command 'npm ci'...",
		"  " + renderlog.PlatformFailure.Line("Exited with status 1") + " ":                                "Exited with status 1",
		"==> ==> Installing Bundler 2.5.6":                                                                 "Installing Bundler 2.5.6",
		"\x1b[31mnpm ERR!\x1b[0m missing script: build":                                                    "\x1b[31mnpm ERR!\x1b[0m missing script: build",
	} {
		if got := unmarked(in); got != want {
			t.Errorf("unmarked(%q) = %q, want %q", in, got, want)
		}
	}
	lines := terminalLines("build_failed", appv1alpha1.TypeWebService,
		"build failed (exit 1)\n"+renderlog.BuildShell.Line("Running build command 'npm ci'...")+"\nnpm ERR! missing script")
	if want := []string{"==> Build failed: build failed (exit 1)", "==> Running build command 'npm ci'...", "==> npm ERR! missing script"}; !slices.Equal(lines, want) {
		t.Errorf("terminalLines = %q, want %q", lines, want)
	}
}

// TestColouredLinesShowTheirWordsInEveryRead is t002's rule: each coloured
// line shows the words it showed before, and a display-mode search finds it
// by those words, while the raw default matches the stored bytes, as for any
// coloured app line (w4/m189).
func TestColouredLinesShowTheirWordsInEveryRead(t *testing.T) {
	entries := formStory()
	if got := fmt.Sprint(shown(entries)); strings.Contains(got, "\x1b") {
		t.Fatalf("a shown line keeps an escape: %s", got)
	}
	matches := func(mode, text string) []string {
		q := LogQuery{Search: []string{text}, TextMode: mode}.normalized()
		var out []string
		for _, e := range entries {
			if q.keep(e) {
				out = append(out, e.Message)
			}
		}
		return out
	}
	if got := matches(TextModeDisplay, "==> Deploying..."); !slices.Equal(got, []string{rendersDeploying}) {
		t.Errorf("display search for ==> Deploying... found %q", got)
	}
	if got := matches(TextModeRaw, "==> Deploying..."); len(got) != 0 {
		t.Errorf("raw search for ==> Deploying... found %q: the stored bytes carry codes between the marker and the words", got)
	}
	if got := matches(TextModeRaw, "Deploying..."); !slices.Equal(got, []string{rendersDeploying}) {
		t.Errorf("raw search for Deploying... found %q", got)
	}
	if got := matches(TextModeRaw, "==> Build queued"); len(got) == 0 {
		t.Error("raw search no longer finds a plain line")
	}
}

// The same rule on REST GET /v1/logs, the CLI's `bex logs --type build
// --text`: a display-mode search finds the coloured line by the words it
// shows; the raw default does not, as for any coloured app line, and finds it
// by its words alone.
func TestRESTTextSearchFindsAColouredLineByItsWords(t *testing.T) {
	d := webLive()
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	svc := newService(nil, app)
	svc.PodLogs, svc.PodLogsFollow = nil, nil
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{d}, nil }
	window := "&startTime=" + d.CreatedAt.Add(-time.Second).Format(time.RFC3339) + "&endTime=" + d.FinishedAt.Add(time.Second).Format(time.RFC3339)
	search := func(query string) []string {
		t.Helper()
		rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&type=build"+window+query)
		var env renderLogList
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("REST %s = %d %s (%v)", query, rec.Code, rec.Body, err)
		}
		var out []string
		for _, l := range env.Logs {
			out = append(out, l.Message)
		}
		return out
	}
	phrase := url.QueryEscape("==> Deploying...")
	if got := search("&textMode=display&text=" + phrase); !slices.Equal(got, []string{rendersDeploying}) {
		t.Errorf("display search = %q, want Render's %q", got, rendersDeploying)
	}
	if got := search("&text=" + phrase); len(got) != 0 {
		t.Errorf("raw search = %q, want none: the stored bytes carry codes between the marker and the words", got)
	}
	if got := search("&text=Deploying..."); !slices.Equal(got, []string{rendersDeploying}) {
		t.Errorf("raw search for the words = %q, want %q", got, rendersDeploying)
	}
}
