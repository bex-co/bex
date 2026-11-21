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

package main_test

// The bex CLI's telemetry disclosure (ADR113 D5), driven through the built
// launcher: a run sends only in CI or once the Bex notice was shown on this
// machine, the notice prints after a notice-eligible command on a terminal,
// and Render's notice never prints.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

const (
	// bexNoticeLine is a line only the Bex notice carries.
	bexNoticeLine = "The data goes to Bex, not to Render."
	// renderNoticeLine opens upstream's notice, which must never print.
	renderNoticeLine = "The Render CLI collects"
)

// analyticsState is where the launcher keeps both notice markers for home.
func analyticsState(home string) string {
	return filepath.Join(home, ".bex", "state", "analytics")
}

func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

func assertNoRenderNotice(t *testing.T, outputs ...string) {
	t.Helper()
	for _, output := range outputs {
		if strings.Contains(output, renderNoticeLine) || strings.Contains(output, "render.com/docs/cli#usage-telemetry") {
			t.Errorf("Render's telemetry notice printed:\n%s", output)
		}
	}
}

// runOnTerminal runs bex the way a person at a terminal does: stderr is a
// pseudo-terminal (where the notice may print) and stdout a pipe, so a test
// can check that stdout stays machine-readable. It returns both streams and
// fails the test when bex fails.
func runOnTerminal(t *testing.T, env []string, args ...string) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, err := tryOnTerminal(t, env, args...)
	if err != nil {
		t.Fatalf("bex %v: %v\nstdout:\n%s\nstderr:\n%s", args, err, stdout, stderr)
	}
	return stdout, stderr
}

// tryOnTerminal is runOnTerminal for a command that may fail.
func tryOnTerminal(t *testing.T, env []string, args ...string) (stdout, stderr string, runErr error) {
	t.Helper()
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("open pseudo-terminal: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	command := exec.Command(buildBex(), args...)
	command.Env = env
	var out bytes.Buffer
	command.Stdout = &out
	command.Stderr = tty
	if err := command.Start(); err != nil {
		_ = tty.Close()
		t.Fatal(err)
	}
	// Only the child holds the terminal now, so the reader below sees EOF
	// (EIO on Linux) once it exits.
	_ = tty.Close()
	var captured bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&captured, ptmx)
		close(done)
	}()
	runErr = command.Wait()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("terminal output did not drain")
	}
	// The terminal's line discipline turns "\n" into "\r\n".
	return out.String(), strings.ReplaceAll(captured.String(), "\r\n", "\n"), runErr
}

// TestUndisclosedHeadlessRunSendsNothing: with no terminal and no CI the
// notice cannot be shown, so nothing is ever sent, including by the
// launcher's own reports (`--version`, a provider launch, `bex s3`), and no Bex marker
// is written. Upstream's marker is, so Render's notice can never print.
func TestUndisclosedHeadlessRunSendsNothing(t *testing.T) {
	stub := &telemetryStub{routes: s3MintRoutes()}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()

	for _, run := range []struct {
		args  []string
		extra []string
	}{
		{[]string{"workspaces", "-o", "json"}, nil},
		{[]string{"--version"}, []string{"BEX_NO_UPDATE_NOTIFIER=1"}},
		{[]string{"glm"}, []string{
			"PATH=" + stubClaudeOnPath(t) + string(os.PathListSeparator) + os.Getenv("PATH"),
			"ZAI_API_KEY=test-provider-key"}},
		{[]string{"s3", "ls"}, s3LaunchEnv(t)},
	} {
		command := exec.Command(buildBex(), run.args...)
		command.Env = telemetryEnv(t, home, api.URL, run.extra...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("bex %v: %v\n%s", run.args, err, output)
		}
		assertNoRenderNotice(t, string(output))
		if strings.Contains(string(output), bexNoticeLine) {
			t.Errorf("bex %v printed the notice without a terminal:\n%s", run.args, output)
		}
	}

	time.Sleep(3 * time.Second)
	if got := stub.count(); got != 0 {
		t.Errorf("telemetry events = %d before any disclosure, want 0", got)
	}
	if !fileExists(t, filepath.Join(analyticsState(home), "notice-shown")) {
		t.Error("upstream's marker was not written")
	}
	if fileExists(t, filepath.Join(analyticsState(home), "bex-notice-shown")) {
		t.Error("a headless run recorded the Bex notice as shown")
	}
}

// TestGateDoesNotReachTheLaunchedAgent: the gate denies consent to the
// sender upstream builds, not to the processes a command starts. An agent
// `bex glm` hands the terminal to must not inherit an opt-out the user never
// set, or every bex it runs would read as opted out.
func TestGateDoesNotReachTheLaunchedAgent(t *testing.T) {
	dir := t.TempDir()
	seen := filepath.Join(dir, "seen")
	script := "#!/bin/sh\necho \"${RENDER_CLI_DISABLE_ANALYTICS-unset}\" > " + seen + "\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(buildBex(), "glm")
	command.Env = telemetryEnv(t, t.TempDir(), "http://127.0.0.1:1",
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "ZAI_API_KEY=test-provider-key")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex glm: %v\n%s", err, output)
	}
	got, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "unset" {
		t.Errorf("the launched agent saw RENDER_CLI_DISABLE_ANALYTICS=%s, want it unset", got)
	}
}

// TestCIBypassesTheDisclosureGate: CI sends on a clean machine, shows
// nothing and records nothing, as Render's CLI does.
func TestCIBypassesTheDisclosureGate(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()

	env := telemetryEnv(t, home, api.URL, "CI=true")
	command := exec.Command(buildBex(), "workspaces", "-o", "json")
	command.Env = env
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("bex workspaces: %v\n%s", err, output)
	}
	awaitEvents(t, stub, 1)
	assertNoRenderNotice(t, string(output))
	if strings.Contains(string(output), bexNoticeLine) {
		t.Errorf("CI printed the notice:\n%s", output)
	}
	// On a terminal too: CI never draws it.
	_, stderr := runOnTerminal(t, env, "workspaces", "-o", "json")
	if strings.Contains(stderr, bexNoticeLine) {
		t.Errorf("CI printed the notice on a terminal:\n%s", stderr)
	}
	if fileExists(t, filepath.Join(analyticsState(home), "bex-notice-shown")) {
		t.Error("CI recorded the Bex notice as shown")
	}
}

// TestFirstTerminalRunShowsTheBexNotice is the definition of done: the first
// terminal run prints the Bex notice once, on stderr, and sends nothing; the
// next run prints nothing and sends exactly one event.
func TestFirstTerminalRunShowsTheBexNotice(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()
	env := telemetryEnv(t, home, api.URL)

	stdout, stderr := runOnTerminal(t, env, "workspaces", "-o", "json")
	if got := strings.Count(stderr, bexNoticeLine); got != 1 {
		t.Fatalf("first run printed the Bex notice %d times, want 1:\n%s", got, stderr)
	}
	for _, want := range []string{"No data has been sent yet.", "BEX_CLI_DISABLE_ANALYTICS=1 or DO_NOT_TRACK=1", "docs/bex-cli.md#cli-usage-telemetry"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("notice missing %q:\n%s", want, stderr)
		}
	}
	if !json.Valid([]byte(stdout)) {
		t.Errorf("stdout is not the command's JSON alone:\n%s", stdout)
	}
	if !fileExists(t, filepath.Join(analyticsState(home), "bex-notice-shown")) {
		t.Fatal("the shown notice was not recorded")
	}
	time.Sleep(3 * time.Second)
	if got := stub.count(); got != 0 {
		t.Fatalf("the run that showed the notice sent %d events, want 0", got)
	}

	_, again := runOnTerminal(t, env, "workspaces", "-o", "json")
	if strings.Contains(again, bexNoticeLine) {
		t.Errorf("second run printed the notice again:\n%s", again)
	}
	if event := awaitEvents(t, stub, 1)[0]; event["command"] != "bex workspaces" {
		t.Errorf("command = %v, want bex workspaces", event["command"])
	}
	assertNoRenderNotice(t, stderr, again)
}

// A machine that saw Render's copy before this release has upstream's marker
// only: it sees the Bex notice once, and that run sends nothing.
func TestMachineThatSawRendersNoticeSeesBexsOnce(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()
	if err := os.MkdirAll(analyticsState(home), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(analyticsState(home), "notice-shown"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr := runOnTerminal(t, telemetryEnv(t, home, api.URL), "workspaces", "-o", "json")
	if !strings.Contains(stderr, bexNoticeLine) {
		t.Fatalf("no Bex notice for a machine that only saw Render's:\n%s", stderr)
	}
	assertNoRenderNotice(t, stderr)
	time.Sleep(3 * time.Second)
	if got := stub.count(); got != 0 {
		t.Errorf("telemetry events = %d, want 0 on the run that showed the notice", got)
	}
}

// TestOptedOutNoticeNamesTheVariableTheUserSet: the bridge maps the Bex
// opt-out onto upstream's, so upstream's copy would name a variable the user
// never set (ADR113 G2).
func TestOptedOutNoticeNamesTheVariableTheUserSet(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	for _, variable := range []string{"BEX_CLI_DISABLE_ANALYTICS", "DO_NOT_TRACK", "RENDER_CLI_DISABLE_ANALYTICS"} {
		t.Run(variable, func(t *testing.T) {
			_, stderr := runOnTerminal(t, telemetryEnv(t, t.TempDir(), api.URL, variable+"=1"), "workspaces", "-o", "json")
			for _, want := range []string{bexNoticeLine, "Telemetry is disabled, so nothing is sent.",
				"To enable telemetry, remove " + variable + " from your environment."} {
				if !strings.Contains(stderr, want) {
					t.Errorf("opted-out notice missing %q:\n%s", want, stderr)
				}
			}
			assertNoRenderNotice(t, stderr)
		})
	}
	time.Sleep(3 * time.Second)
	if got := stub.count(); got != 0 {
		t.Errorf("telemetry events = %d under opt-out, want 0", got)
	}
}

// TestCompletionAndAnalyticsNeverShowTheNotice: shell completion and the
// hidden analytics tree are notice-ineligible upstream, so they neither print
// the notice nor record it.
func TestCompletionAndAnalyticsNeverShowTheNotice(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	for _, args := range [][]string{
		{"__complete", "workspace", ""},
		{"__completeNoDesc", "workspace", ""},
		{"completion", "zsh"},
	} {
		t.Run(args[0], func(t *testing.T) {
			home := t.TempDir()
			_, stderr := runOnTerminal(t, telemetryEnv(t, home, api.URL), args...)
			if strings.Contains(stderr, bexNoticeLine) {
				t.Errorf("bex %v printed the notice:\n%s", args, stderr)
			}
			assertNoRenderNotice(t, stderr)
			if fileExists(t, filepath.Join(analyticsState(home), "bex-notice-shown")) {
				t.Errorf("bex %v recorded the notice as shown", args)
			}
		})
	}
	t.Run("analytics send", func(t *testing.T) {
		home := t.TempDir()
		// The event file does not exist, so the command fails; it must still
		// draw nothing.
		_, stderr, err := tryOnTerminal(t, telemetryEnv(t, home, api.URL), "analytics", "send", filepath.Join(home, "missing.json"))
		if err == nil {
			t.Fatal("analytics send of a missing event file succeeded")
		}
		if strings.Contains(stderr, bexNoticeLine) {
			t.Errorf("analytics send printed the notice:\n%s", stderr)
		}
		assertNoRenderNotice(t, stderr)
		if fileExists(t, filepath.Join(analyticsState(home), "bex-notice-shown")) {
			t.Error("analytics send recorded the notice as shown")
		}
	})
}

// analyticsNotice runs the installer's `bex analytics notice` without a
// terminal and returns its stderr.
func analyticsNotice(t *testing.T, env []string) string {
	t.Helper()
	command := exec.Command(buildBex(), "analytics", "notice")
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("bex analytics notice: %v\n%s", err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("bex analytics notice wrote to stdout:\n%s", stdout.String())
	}
	return stderr.String()
}

// TestAnalyticsNoticeShowsTheBexNoticeOnce: the hidden command installers run
// prints Bex's text with no terminal, records both markers, and prints
// nothing the second time; after it, a headless run sends.
func TestAnalyticsNoticeShowsTheBexNoticeOnce(t *testing.T) {
	stub := &telemetryStub{}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()
	env := telemetryEnv(t, home, api.URL)

	first := analyticsNotice(t, env)
	if !strings.Contains(first, bexNoticeLine) || !strings.Contains(first, "No data has been sent yet.") {
		t.Fatalf("bex analytics notice printed:\n%s", first)
	}
	assertNoRenderNotice(t, first)
	for _, marker := range []string{"notice-shown", "bex-notice-shown"} {
		if !fileExists(t, filepath.Join(analyticsState(home), marker)) {
			t.Errorf("%s not written", marker)
		}
	}
	if second := analyticsNotice(t, env); second != "" {
		t.Errorf("second bex analytics notice printed:\n%s", second)
	}

	command := exec.Command(buildBex(), "workspaces", "-o", "json")
	command.Env = env
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex workspaces: %v\n%s", err, output)
	}
	awaitEvents(t, stub, 1)
}

// TestFormulaCaveatsAreTheBexNotice keeps the Homebrew formula's caveats
// (scripts/bex-cli-formula.sh) equal to what `bex analytics notice` prints.
func TestFormulaCaveatsAreTheBexNotice(t *testing.T) {
	checksums := filepath.Join(t.TempDir(), "checksums.txt")
	var lines strings.Builder
	for _, platform := range []string{"darwin-arm64", "darwin-amd64", "linux-arm64", "linux-amd64"} {
		lines.WriteString(strings.Repeat("0", 64) + "  bex-0.1.0-" + platform + ".tar.gz\n")
	}
	if err := os.WriteFile(checksums, []byte(lines.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	render := exec.Command("bash", filepath.Join("..", "..", "scripts", "bex-cli-formula.sh"), checksums)
	render.Env = append(os.Environ(), "VERSION=0.1.0")
	formula, err := render.Output()
	if err != nil {
		t.Fatalf("render formula: %v", err)
	}

	// The body of `<<~'NOTICE'`, dedented as Ruby's squiggly heredoc does.
	_, body, ok := strings.Cut(string(formula), "<<~'NOTICE'\n")
	if !ok {
		t.Fatalf("formula has no caveats heredoc:\n%s", formula)
	}
	body, _, ok = strings.Cut(body, "\n    NOTICE\n")
	if !ok {
		t.Fatalf("caveats heredoc is not terminated:\n%s", formula)
	}
	var caveats []string
	for _, line := range strings.Split(body, "\n") {
		caveats = append(caveats, strings.TrimPrefix(line, "      "))
	}

	notice := analyticsNotice(t, telemetryEnv(t, t.TempDir(), "http://127.0.0.1:1"))
	if got, want := strings.Join(caveats, "\n")+"\n", notice; got != want {
		t.Errorf("formula caveats differ from bex analytics notice:\ncaveats:\n%s\nnotice:\n%s", got, want)
	}
}
