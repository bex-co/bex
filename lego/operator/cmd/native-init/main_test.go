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
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary doubles as native-init, so every case runs real processes.
func TestMain(m *testing.M) {
	if os.Getenv("BEX_NATIVE_INIT_UNDER_TEST") == "1" {
		os.Exit(run(os.Args[1:], os.Stderr))
	}
	os.Exit(m.Run())
}

func nativeInit(args ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "BEX_NATIVE_INIT_UNDER_TEST=1", "QA_MARKER=kept as is")
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
			7, "kept as is|arg with 'quotes'|" + dir + "\n", "oops\n",
		},
		{"killed by a signal", []string{"/bin/sh", "-c", "kill -KILL $$"}, 137, "", ""},
		{"found on PATH", []string{"sh", "-c", "exit 3"}, 3, "", ""},
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
// signal sent next reaches the command rather than racing its launch.
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
	if !lines.Scan() || lines.Text() != "ready" {
		t.Fatalf("command did not start: %q", lines.Text())
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
	<-done
}
