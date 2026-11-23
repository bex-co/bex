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

package main

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary doubles as native-init, so every case runs real processes.
func TestMain(m *testing.M) {
	if os.Getenv("BEX_NATIVE_INIT_UNDER_TEST") == "1" {
		os.Exit(run(os.Args[1:], os.Getenv("BEX_NATIVE_INIT_TEST_SECRETS"), os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// nativeInit runs native-init with no secret files mounted, unless the caller
// names a directory in BEX_NATIVE_INIT_TEST_SECRETS after it.
func nativeInit(args ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "BEX_NATIVE_INIT_UNDER_TEST=1", "QA_MARKER=kept as is",
		"BEX_NATIVE_INIT_TEST_SECRETS=/nonexistent/bex-native-init/secrets")
	return cmd
}

func exitStatus(t *testing.T, err error) int {
	t.Helper()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	}
	t.Fatalf("native-init did not run: %v", err)
	return -1
}

// The command runs exactly as PID 1 would run it: its argv, stdio, environment
// and working directory, and its exit status or 128 plus its signal.
func TestNativeInitRunsTheCommandAsIs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	notExecutable := filepath.Join(dir, "script")
	if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name           string
		args           []string
		status         int
		stdout, stderr string
	}{
		{
			"output, environment and cwd",
			[]string{"/bin/sh", "-c", `printf '%s|%s|' "$QA_MARKER" "$1"; pwd; echo oops >&2; exit 7`,
				"sh", "arg with 'quotes'"},
			7, running(`printf '%s|%s|' "$QA_MARKER" "$1"; pwd; echo oops >&2; exit 7`) + "\n" +
				"kept as is|arg with 'quotes'|" + dir + "\n", "oops\n",
		},
		{"killed by a signal", []string{"/bin/sh", "-c", "kill -KILL $$"}, 137, running("kill -KILL $$") + "\n", ""},
		{"found on PATH", []string{"sh", "-c", "exit 3"}, 3, running("exit 3") + "\n", ""},
		{"missing executable", []string{"/nonexistent/app"}, 127, "", "native-init: "},
		{"not executable", []string{notExecutable}, 126, "", "native-init: "},
		{"no command", nil, 2, "", "native-init: no command to run\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := nativeInit(tc.args...)
			cmd.Dir = dir
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if got := exitStatus(t, cmd.Run()); got != tc.status {
				t.Errorf("status = %d, want %d (stderr %q)", got, tc.status, stderr.String())
			}
			if resolved, err := filepath.EvalSymlinks(dir); err == nil {
				tc.stdout = strings.Replace(tc.stdout, dir, resolved, 1)
			}
			if stdout.String() != tc.stdout || !strings.HasPrefix(stderr.String(), tc.stderr) {
				t.Errorf("stdout %q stderr %q, want %q and prefix %q", stdout.String(), stderr.String(), tc.stdout, tc.stderr)
			}
		})
	}
}

// started runs native-init and waits for the command to print ready, so a
// signal sent next reaches the command rather than racing its launch. The
// command's line comes first, script and all.
func started(t *testing.T, script string) (*exec.Cmd, *bufio.Scanner) {
	t.Helper()
	cmd := nativeInit("/bin/sh", "-c", script)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	lines := bufio.NewScanner(out)
	want := append(strings.Split(running(script), "\n"), "ready")
	first := make(chan []string, 1)
	go func() {
		var read []string
		for len(read) < len(want) && lines.Scan() {
			read = append(read, lines.Text())
		}
		first <- read
	}()
	select {
	case read := <-first:
		if !slices.Equal(read, want) {
			t.Fatalf("command not named, then started: %q, want %q", read, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("command not named, then started, within 10s")
	}
	return cmd, lines
}

// killPidFile kills the leftover whose PID a test command recorded, which
// outlives a native-init that is not PID 1.
func killPidFile(path string) {
	if raw, err := os.ReadFile(path); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

func rest(lines *bufio.Scanner) []string {
	var out []string
	for lines.Scan() {
		out = append(out, lines.Text())
	}
	return out
}

// A command that handles SIGTERM itself gets it from native-init, exactly
// once, and its own exit ends native-init with that status.
func TestNativeInitForwardsSignalsToTheCommand(t *testing.T) {
	t.Parallel()
	cmd, lines := started(t, `trap 'echo hup' HUP; trap 'echo term; exit 0' TERM; echo ready
while :; do sleep 0.05; done`)
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	if !lines.Scan() || lines.Text() != "hup" {
		t.Fatalf("SIGHUP not forwarded: %q", lines.Text())
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	got := rest(lines)
	if status := exitStatus(t, cmd.Wait()); status != 0 || strings.Join(got, ",") != "term" {
		t.Fatalf("status %d output %q, want 0 and term", status, got)
	}
}

// The npm shape: the command dies of SIGTERM at once while the process it
// started is still draining. native-init passes the signal on and waits for
// that drain before exiting with the command's status.
func TestNativeInitWaitsForWhatAStoppedCommandLeftBehind(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("orphans are adopted only by a Linux subreaper")
	}
	app := `trap 'echo app-term; sleep 1; echo app-drained; exit 0' TERM; while :; do sleep 0.05; done`
	cmd, lines := started(t, `sh -c "`+app+`" & sleep 0.2; echo ready; wait`)
	begin := time.Now()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	got := rest(lines)
	status, took := exitStatus(t, cmd.Wait()), time.Since(begin)
	if status != 128+int(syscall.SIGTERM) || strings.Join(got, ",") != "app-term,app-drained" || took < time.Second {
		t.Fatalf("status %d after %v, output %q; want 143 after the app's 1s drain", status, took, got)
	}
}

// Without a stop request, a command exiting ends native-init at once: what it
// left running is not waited for, as when the command itself was PID 1.
func TestNativeInitExitsWithTheCommand(t *testing.T) {
	t.Parallel()
	pidFile := filepath.Join(t.TempDir(), "pid")
	cmd := nativeInit("/bin/sh", "-c", `sleep 30 & echo $! > "$1"; exit 5`, "sh", pidFile)
	t.Cleanup(func() { killPidFile(pidFile) })
	begin := time.Now()
	status := exitStatus(t, cmd.Run())
	if took := time.Since(begin); status != 5 || took > 5*time.Second {
		t.Fatalf("status %d after %v, want 5 at once", status, took)
	}
}

// A leftover that ignores the stop signal keeps native-init waiting: the
// runtime's deadline (kubelet's SIGKILL at the end of the grace period) is
// what ends it, as for any process that ignores SIGTERM.
func TestNativeInitLeavesTheDeadlineToTheRuntime(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("orphans are adopted only by a Linux subreaper")
	}
	pidFile := filepath.Join(t.TempDir(), "pid")
	cmd, _ := started(t, `sh -c "echo \$\$ > '`+pidFile+`'; trap '' TERM; while :; do sleep 0.05; done" &
sleep 0.2; echo ready; wait`)
	t.Cleanup(func() { killPidFile(pidFile) })
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		t.Fatalf("native-init exited (%v) while a leftover was still running", err)
	case <-time.After(500 * time.Millisecond):
	}
	_ = cmd.Process.Kill()
	if err := <-done; err != nil {
		t.Logf("native-init after the kill: %v", err) // expected: killed
	}
}

// running is the native instance's line for command as Render's log API
// returns it, byte for byte from ivanseibel/tretta's capture
// (04b-render-logs.json at b0f216c985; w32/m576): green in tput's form.
func running(command string) string {
	return "\x1b[32m\x1b[1m==>\x1b(B\x1b[m \x1b[1mRunning '" + command + "'\x1b(B\x1b[m"
}

// w32/m575: Render's native instance writes `==> Running '<command>'` each
// time it starts the command. native-init writes it once, before the
// command's own first output, quoted as configured: $PORT and every other
// variable unexpanded, under bash as under sh.
func TestNativeInitNamesTheCommandBeforeItsOutput(t *testing.T) {
	t.Parallel()
	const command = `echo "listening on $PORT"; echo "$(QA_MARKER)" >/dev/null 2>&1; echo done`
	for _, shell := range []string{"/bin/sh", "/bin/bash"} {
		t.Run(shell, func(t *testing.T) {
			t.Parallel()
			if _, err := os.Stat(shell); err != nil {
				t.Skipf("%s: %v", shell, err)
			}
			cmd := nativeInit(shell, "-c", command)
			cmd.Env = append(cmd.Env, "PORT=10000")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if status := exitStatus(t, cmd.Run()); status != 0 {
				t.Fatalf("status %d stderr %q", status, stderr.String())
			}
			want := running(command) + "\nlistening on 10000\ndone\n"
			if stdout.String() != want {
				t.Fatalf("stdout %q, want %q", stdout.String(), want)
			}
		})
	}
}

// Render names a command whose program bash cannot find too: the line comes
// first, then the shell's own error (Unwrenchable/ATOMIC-FIZZ-CAPS-VAULT-77-
// WASTELAND-GPS#82: `Running 'Node.js'`, then `command not found`).
func TestNativeInitNamesACommandThatCannotRun(t *testing.T) {
	t.Parallel()
	cmd := nativeInit("/bin/sh", "-c", "qa-no-such-program-m575")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if status := exitStatus(t, cmd.Run()); status != 127 || stdout.String() != running("qa-no-such-program-m575")+"\n" ||
		!strings.Contains(stderr.String(), "qa-no-such-program-m575") {
		t.Fatalf("status %d stdout %q stderr %q, want 127, the line and the shell's error",
			status, stdout.String(), stderr.String())
	}
}

// Only a `<shell> -c <command>` launch is a command Render names. A program
// started directly, a shell given a script or other flags, and a launch that
// cannot start write nothing of native-init's own.
func TestNativeInitNamesNothingForAnotherLaunch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	script := filepath.Join(dir, "script.sh")
	if err := os.WriteFile(script, []byte("echo from-script\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		args   []string
		stdout string
	}{
		{"a program run directly", []string{"/bin/echo", "-c", "direct"}, "-c direct\n"},
		{"a shell given a script", []string{"/bin/sh", script}, "from-script\n"},
		{"a shell given another flag first", []string{"/bin/sh", "-e", "-c", "echo flagged"}, "flagged\n"},
		{"a shell with no command", []string{"/bin/sh", "-c"}, ""},
		{"a missing executable", []string{"/nonexistent/sh", "-c", "echo never"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := nativeInit(tc.args...)
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			_ = cmd.Run()
			if stdout.String() != tc.stdout {
				t.Fatalf("stdout %q, want %q", stdout.String(), tc.stdout)
			}
		})
	}
}

// A launch the platform names itself, a pre-deploy step whose line the
// deploy log narrates, carries BEX_NATIVE_INIT_QUIET: native-init writes no
// line, and the command never sees the variable, whether or not it was set.
func TestNativeInitQuietLaunchNamesNothingAndHidesTheSignal(t *testing.T) {
	t.Parallel()
	const command = `printf '%s\n' "${BEX_NATIVE_INIT_QUIET-unset}"; env | grep -c BEX_NATIVE_INIT_QUIET || true`
	for _, tc := range []struct {
		name   string
		env    []string
		stdout string
	}{
		{"quiet", []string{"BEX_NATIVE_INIT_QUIET=1"}, "unset\n0\n"},
		{"quiet, empty", []string{"BEX_NATIVE_INIT_QUIET="}, "unset\n0\n"},
		{"announced", nil, running(command) + "\nunset\n0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := nativeInit("/bin/sh", "-c", command)
			cmd.Env = append(cmd.Env, tc.env...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if status := exitStatus(t, cmd.Run()); status != 0 || stdout.String() != tc.stdout {
				t.Fatalf("status %d stdout %q stderr %q, want %q", status, stdout.String(), stderr.String(), tc.stdout)
			}
		})
	}
}
