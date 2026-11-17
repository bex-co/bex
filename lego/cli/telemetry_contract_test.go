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

// ADR113 D7, the coverage contract, against the built launcher: every
// Bex-native command that replaces the process stores exactly one event whose
// command path carries no argument, and the outcomes `bex agent` reports by
// exit code arrive as explicit_exit with that code, not as CLI faults. The
// list these launch tests answer to is processReplacers in
// telemetry_coverage_test.go.

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/cli/internal/desktop"
)

// contractSession is the agent session the launch and outcome tests read.
const contractSession = "ags-d0abc123def456ghij78"

// contractSandbox is that session's running sandbox.
const contractSandbox = contractSession + "@ssh.example.test"

// sessionAnswer is bex-api's answer for contractSession in state.
func sessionAnswer(state string) string {
	return `{"id":"` + contractSession + `","ownerId":"tea-bex","state":"` + state + `",` +
		`"failureReasonCode":"STRUCTURED_OUTPUT_MISSING","failureReason":"no result",` +
		`"sshAddress":"` + contractSandbox + `","agentConfig":{"agent":"claude","task":"t"}}`
}

// recordingProgram puts a program called name on a fresh PATH entry. It
// writes its arguments to the returned file and exits 0, so a test can tell
// that bex replaced itself with it, and with what.
func recordingProgram(t *testing.T, name string) (pathEntry, argsFile string) {
	t.Helper()
	pathEntry = t.TempDir()
	argsFile = filepath.Join(t.TempDir(), "args")
	script := "#!/bin/sh\necho \"$@\" > " + argsFile + "\n"
	if err := os.WriteFile(filepath.Join(pathEntry, name), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return pathEntry, argsFile
}

// launchedWith is what the program recordingProgram installed was run with;
// it fails the test when bex never replaced itself with that program.
func launchedWith(t *testing.T, argsFile string) string {
	t.Helper()
	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("bex did not replace itself with the launched program: %v", err)
	}
	return strings.TrimSpace(string(got))
}

// launchEnv is telemetryEnv for a launch, with pathEntry ahead of PATH. The
// update check stays off the network: on a terminal it would otherwise run.
func launchEnv(t *testing.T, home, apiURL, pathEntry string) []string {
	t.Helper()
	return telemetryEnv(t, home, apiURL,
		"PATH="+pathEntry+string(os.PathListSeparator)+os.Getenv("PATH"),
		"BEX_NO_UPDATE_NOTIFIER=1")
}

// raw is every captured telemetry body, undecoded.
func (s *telemetryStub) raw() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.bodies...)
}

// assertOneLaunchEvent: the launch stored exactly one event, a success of
// command, and nothing typed after the command names reached any body.
func assertOneLaunchEvent(t *testing.T, stub *telemetryStub, command string, typed ...string) {
	t.Helper()
	event := awaitEvents(t, stub, 1)[0]
	if event["command"] != command {
		t.Errorf("command = %v, want %s", event["command"], command)
	}
	if event["completion_kind"] != "success" || event["exit_code"] != float64(0) {
		t.Errorf("completion_kind %v, exit_code %v; want success, 0", event["completion_kind"], event["exit_code"])
	}
	for _, body := range stub.raw() {
		for _, argument := range typed {
			if strings.Contains(body, argument) {
				t.Errorf("the event carries the argument %q: %s", argument, body)
			}
		}
	}
}

// TestBexEditLaunchEmitsExactlyOneEvent: `bex edit <path>` and its shorthand
// `bex <path>` replace bex with the desktop launcher, so upstream's hook never
// runs; the launch reports itself once, as `bex edit`, without the path.
func TestBexEditLaunchEmitsExactlyOneEvent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "launch-contract-notes.md")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, run := range []struct {
		name string
		args []string
	}{
		{"edit", []string{"edit", file + ":42"}},
		{"shorthand", []string{file}},
	} {
		t.Run(run.name, func(t *testing.T) {
			t.Parallel()
			stub := &telemetryStub{}
			api := httptest.NewServer(stub.handler())
			t.Cleanup(api.Close)
			home := t.TempDir()
			seedNoticeMarker(t, home)
			bin, argsFile := recordingProgram(t, desktop.LauncherName)

			// bex edit opens a GUI only from a terminal.
			if _, stderr, err := tryOnTerminal(t, launchEnv(t, home, api.URL, bin), run.args...); err != nil {
				t.Fatalf("bex %v: %v\n%s", run.args, err, stderr)
			}
			if got, want := launchedWith(t, argsFile), run.args[len(run.args)-1]; got != want {
				t.Errorf("launcher ran with %q, want %q", got, want)
			}
			assertOneLaunchEvent(t, stub, "bex edit", "launch-contract-notes", dir)
		})
	}
}

// TestBexAgentOpenLaunchEmitsExactlyOneEvent: with the desktop installed and
// the sandbox up, `bex agent open` replaces bex with the desktop launcher
// opening the sandbox. The dashboard fallback returns through Cobra instead,
// so the launcher's own record is what proves this is the replacing path.
func TestBexAgentOpenLaunchEmitsExactlyOneEvent(t *testing.T) {
	t.Parallel()
	stub := &telemetryStub{routes: map[string]string{"/v1/agent-sessions/" + contractSession: sessionAnswer("working")}}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()
	seedNoticeMarker(t, home)
	bin, argsFile := recordingProgram(t, desktop.LauncherName)

	stdout, stderr, err := tryOnTerminal(t, launchEnv(t, home, api.URL, bin), "agent", "open", contractSession)
	if err != nil {
		t.Fatalf("bex agent open: %v\nstdout %s\nstderr %s", err, stdout, stderr)
	}
	if got, want := launchedWith(t, argsFile), "ssh://"+contractSandbox+"/workspace"; got != want {
		t.Errorf("launcher ran with %q, want %q", got, want)
	}
	assertOneLaunchEvent(t, stub, "bex agent open", contractSession)
}

// TestBexAgentSSHLaunchEmitsExactlyOneEvent: `bex agent ssh` replaces bex with
// the user's ssh; the launch reports itself once, without the session or the
// arguments that went to ssh.
func TestBexAgentSSHLaunchEmitsExactlyOneEvent(t *testing.T) {
	t.Parallel()
	stub := &telemetryStub{routes: map[string]string{"/v1/agent-sessions/" + contractSession: sessionAnswer("working")}}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()
	seedNoticeMarker(t, home)
	bin, argsFile := recordingProgram(t, "ssh")

	command := exec.Command(buildBex(), "agent", "ssh", contractSession, "--", "-p", "2222", "contract-remote-command")
	command.Env = launchEnv(t, home, api.URL, bin)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex agent ssh: %v\n%s", err, output)
	}
	if got, want := launchedWith(t, argsFile), "-p 2222 "+contractSandbox+" contract-remote-command"; got != want {
		t.Errorf("ssh ran with %q, want %q", got, want)
	}
	assertOneLaunchEvent(t, stub, "bex agent ssh", contractSession, "contract-remote-command")
}

// TestBexAgentOutcomesArriveAsExplicitExit pins the data the board's agent
// outcome panels read (ADR113 D7): a failed turn, a canceled session and an
// automation run that started no session are outcomes the command reports by
// its exit code, stored as explicit_exit with that code, never as a CLI fault.
func TestBexAgentOutcomesArriveAsExplicitExit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		route   string
		answer  string
		args    []string
		command string
		exit    int
	}{
		{
			name: "failed turn", route: "/v1/agent-sessions/" + contractSession, answer: sessionAnswer("failed"),
			args: []string{"agent", "wait", contractSession}, command: "bex agent wait", exit: 3,
		},
		{
			name: "canceled session", route: "/v1/agent-sessions/" + contractSession, answer: sessionAnswer("canceled"),
			args: []string{"agent", "wait", contractSession}, command: "bex agent wait", exit: 4,
		},
		{
			name: "automation run started no session", route: "/v1/agent-automations/triage/run",
			answer: `{"automationId":"aut-d0abc123def456ghij78","fireAt":"2026-11-16T10:00:00Z","outcome":"skipped_overlap"}`,
			args:   []string{"agent", "automations", "run", "triage"}, command: "bex agent automations run", exit: 6,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stub := &telemetryStub{routes: map[string]string{tc.route: tc.answer}}
			api := httptest.NewServer(stub.handler())
			t.Cleanup(api.Close)
			home := t.TempDir()
			seedNoticeMarker(t, home)

			command := exec.Command(buildBex(), tc.args...)
			command.Env = telemetryEnv(t, home, api.URL, "BEX_NO_UPDATE_NOTIFIER=1", "BEX_WORKSPACE=tea-bex")
			output, _ := command.CombinedOutput()
			if code := command.ProcessState.ExitCode(); code != tc.exit {
				t.Fatalf("bex %v exited %d, want %d\n%s", tc.args, code, tc.exit, output)
			}

			event := awaitEvents(t, stub, 1)[0]
			if event["command"] != tc.command {
				t.Errorf("command = %v, want %s", event["command"], tc.command)
			}
			if event["completion_kind"] != "explicit_exit" {
				t.Errorf("completion_kind = %v, want explicit_exit", event["completion_kind"])
			}
			if event["exit_code"] != float64(tc.exit) {
				t.Errorf("exit_code = %v, want %d", event["exit_code"], tc.exit)
			}
		})
	}
}
