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

package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The wrapper and the probe run under this host's real /bin/sh and /proc,
// as pods/exec runs them in the sandbox (w32/m381).

func needProc(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("no /proc on this host")
	}
	needSh(t)
}

// registeredCommand is the wrapper's process for one command recorded under
// registry, started in a home directory with bash on PATH.
func registeredCommand(t *testing.T, registry, command string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	argv := execArgv(command, registry)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=" + pathWithBash(t) + ":/usr/bin:/bin"}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	return cmd, &out, &errOut
}

// commandExitCode is a finished process's exit code; any other error fails.
func commandExitCode(t *testing.T, err error) int {
	t.Helper()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	}
	t.Fatal(err)
	return -1
}

func probeRegistry(t *testing.T, dir string, ids ...string) string {
	t.Helper()
	out, err := exec.Command("/bin/sh", append([]string{"-c", commandProbe, "bex-probe", dir}, ids...)...).Output()
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	return string(out)
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(path); err == nil {
			return
		}
	}
	t.Fatalf("%s never appeared", path)
}

// The registry changes nothing the command sees: bash in the home directory,
// the stdout/stderr split and the exit code are as w32/m348 pinned them.
func TestARegisteredCommandRunsAsBeforeAndRecordsItsExit(t *testing.T) {
	needProc(t)
	dir := t.TempDir()
	cmd, out, errOut := registeredCommand(t, filepath.Join(dir, "exe-1"), `pwd; echo out; echo err >&2; arr=(a b); echo ${arr[1]}; exit 7`)
	home := strings.TrimPrefix(cmd.Env[0], "HOME=")
	before := time.Now().Unix()
	if code := commandExitCode(t, cmd.Run()); code != 7 || out.String() != home+"\nout\nb\n" || errOut.String() != "err\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "exe-1.run")); !os.IsNotExist(err) {
		t.Errorf("the ended command is still registered as running: %v", err)
	}
	got := probeRegistry(t, dir, "exe-1", "exe-unknown")
	var at int64
	if _, err := fmt.Sscanf(got, "exited exe-1 7 %d\n", &at); err != nil || at < before || at > time.Now().Unix() {
		t.Fatalf("probe = %q, want exe-1 exited 7 now", got)
	}
}

func TestARunningCommandIsInTheRegistryUntilItEnds(t *testing.T) {
	needProc(t)
	dir := t.TempDir()
	cmd, _, _ := registeredCommand(t, filepath.Join(dir, "exe-2"), `sleep 1; exit 3`)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForFile(t, filepath.Join(dir, "exe-2.run"))
	if got := probeRegistry(t, dir, "exe-2"); got != "running exe-2\n" {
		t.Fatalf("while it runs, probe = %q", got)
	}
	if code := commandExitCode(t, cmd.Wait()); code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
	if got := probeRegistry(t, dir, "exe-2"); !strings.HasPrefix(got, "exited exe-2 3 ") {
		t.Fatalf("after it ended, probe = %q", got)
	}
}

// After a resume the sandbox's PIDs start again, so a stale entry's PID may
// name another process: only the PID with the recorded start time counts.
func TestAStaleRegistryEntryIsNotRunning(t *testing.T) {
	needProc(t)
	dir := t.TempDir()
	for id, entry := range map[string]string{
		"exe-reused": strconv.Itoa(os.Getpid()) + " 1\n", // alive, but not since tick 1
		"exe-gone":   "999999999 1\n",
		"exe-blank":  strconv.Itoa(os.Getpid()) + "\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, id+".run"), []byte(entry), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := probeRegistry(t, dir, "exe-reused", "exe-gone", "exe-blank"); got != "" {
		t.Fatalf("probe = %q, want nothing running", got)
	}
}

// The command runs in the foreground, so it keeps SIGINT: a background child
// of a non-interactive shell starts with it ignored and could not trap it.
func TestARegisteredCommandKeepsSIGINT(t *testing.T) {
	needProc(t)
	dir := t.TempDir()
	cmd, out, _ := registeredCommand(t, filepath.Join(dir, "exe-4"), `trap 'echo got-int; exit 5' INT; while :; do sleep 0.05; done`)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForFile(t, filepath.Join(dir, "exe-4.run"))
	entry, err := os.ReadFile(filepath.Join(dir, "exe-4.run"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.Fields(string(entry))[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	if code := commandExitCode(t, cmd.Wait()); code != 5 || out.String() != "got-int\n" {
		t.Fatalf("exit %d, stdout %q: the command did not trap its SIGINT", code, out)
	}
}

// Render: "Background processes do not keep a sandbox active after the
// command that started them finishes".
func TestABackgroundProcessOfAFinishedCommandIsNotRunning(t *testing.T) {
	needProc(t)
	dir := t.TempDir()
	cmd, _, _ := registeredCommand(t, filepath.Join(dir, "exe-5"), `sleep 3 >/dev/null 2>&1 & echo started`)
	if code := commandExitCode(t, cmd.Run()); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := probeRegistry(t, dir, "exe-5"); !strings.HasPrefix(got, "exited exe-5 0 ") {
		t.Fatalf("probe = %q, want exe-5 ended while its background sleep runs", got)
	}
}

// A root filesystem where the registry cannot be made still runs the command.
func TestARegistryThatCannotBeMadeStillRunsTheCommand(t *testing.T) {
	needProc(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd, out, _ := registeredCommand(t, filepath.Join(blocker, "exe-6"), `echo ran; exit 4`)
	if code := commandExitCode(t, cmd.Run()); code != 4 || out.String() != "ran\n" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
}

// The exit log stays bounded: past 1 MiB it moves aside, and the probe reads
// both halves.
func TestTheExitLogRotatesAndBothHalvesAreRead(t *testing.T) {
	needProc(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "exits"), bytes.Repeat([]byte("exe-old 0 1\n"), 1<<20/12+1), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"exe-7", "exe-8"} {
		cmd, _, _ := registeredCommand(t, filepath.Join(dir, id), "exit 0")
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "exits")); err != nil || info.Size() > 64 {
		t.Fatalf("exits = %v (%v), want a fresh log after the rotation", info, err)
	}
	got := probeRegistry(t, dir, "exe-7", "exe-8")
	if !strings.Contains(got, "exited exe-7 0 ") || !strings.Contains(got, "exited exe-8 0 ") {
		t.Fatalf("probe = %q, want both commands from both halves", got)
	}
}

// An ordinary sandbox's exec names its registry entry after its record, so a
// stream that ends without the command's exit leaves the record open for the
// registry to close. An exit on the stream closes it at once, as before.
func TestAStreamWithoutTheCommandsExitLeavesItsRecordOpen(t *testing.T) {
	secret := []byte("exec-secret")
	for name, tc := range map[string]struct {
		events string
		open   bool
	}{
		"client gone":  {events: "event: output\ndata: {\"stream\":\"stdout\",\"data\":\"partial\"}\n\n", open: true},
		"command done": {events: "event: exit\ndata: {\"exit_code\":2}\n\n"},
	} {
		t.Run(name, func(t *testing.T) {
			var argv []string
			gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				claims, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), time.Now())
				if err != nil {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				argv = claims.Command
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte(tc.events))
			}))
			defer gw.Close()
			execs := &fakeExecutions{}
			svc := &Service{
				Base:       &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
				Client:     execSandboxClient(t, "id-a"),
				Exec:       &ExecConfig{Secret: secret, GatewayURL: gw.URL, Client: gw.Client()},
				Executions: execs,
			}
			rr := httptest.NewRecorder()
			if err := svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "sleep 480"}, rr, rr.Flush); err != nil {
				t.Fatal(err)
			}
			rows := execs.snapshot()
			if len(rows) != 1 {
				t.Fatalf("records = %+v", rows)
			}
			if want := commandRegistry + "/" + rows[0].ID; len(argv) < 3 || argv[len(argv)-3] != want || argv[len(argv)-1] != "sleep 480" {
				t.Fatalf("argv = %q, want the registry entry %s, then the command last", argv, want)
			}
			if open := rows[0].StoppedAt == nil; open != tc.open {
				t.Fatalf("record %+v open = %v, want %v", rows[0], open, tc.open)
			}
			if !tc.open && (rows[0].ExitCode == nil || *rows[0].ExitCode != 2) {
				t.Fatalf("record %+v, want exit 2", rows[0])
			}
		})
	}
}

// commandsFixture is the idle fixture with execution records: one sandbox,
// --idle-timeout 300, whose only command lost its client.
func commandsFixture(t *testing.T) (*idleFixture, *fakeExecutions) {
	t.Helper()
	f := newIdleFixture(t)
	f.os.add("os-1", "Running", f.now.Add(-time.Hour), "300", nil)
	execs := &fakeExecutions{rows: []store.SandboxExecution{{
		ID: "exe-1", WorkspaceID: "tea-a", SandboxID: "os-1", Type: ExecutionTypeRuns,
		Operation: ConnectOperationStream, Command: "sleep 480", StartedAt: f.now.Add(-10 * time.Minute),
	}}}
	f.svc.Executions = execs
	return f, execs
}

func (f *idleFixture) answerProbe(out string, code int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probeOut, f.probeExit = out, code
}

func (f *idleFixture) probes() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.probeAsked)
}

// The m381 DoD on the sweeper: a command whose client left keeps the sandbox
// running for as long as it runs, and the idle window then runs from its end.
func TestADisconnectedCommandKeepsItsSandboxAwakeUntilItEnds(t *testing.T) {
	f, execs := commandsFixture(t)
	f.answerProbe("running exe-1\n", 0)
	if n := f.sweeper.Sweep(context.Background()); n != 0 {
		t.Fatalf("suspended %d sandboxes with a command running", n)
	}
	if got := f.probes(); len(got) != 1 || !slices.Equal(got[0], []string{"exe-1"}) {
		t.Fatalf("probes = %v, want one about exe-1", got)
	}
	if f.activity.claimed("os-1") {
		t.Fatal("the claim outlived the decision to keep the sandbox up")
	}
	if rows := execs.snapshot(); rows[0].StoppedAt != nil || !execs.seen["exe-1"].Equal(f.now) {
		t.Fatalf("record = %+v seen %v, want open and refreshed now", rows[0], execs.seen["exe-1"])
	}

	// It ends 2 minutes before the next look: the sandbox stays up for the
	// rest of its window, and the record closes at the command's own exit.
	f.now = f.now.Add(6 * time.Minute)
	ended := f.now.Add(-2 * time.Minute)
	f.answerProbe(fmt.Sprintf("exited exe-1 0 %d\n", ended.Unix()), 0)
	if n := f.sweeper.Sweep(context.Background()); n != 0 {
		t.Fatalf("suspended %d sandboxes 2 minutes after their command ended", n)
	}
	rows := execs.snapshot()
	if rows[0].StoppedAt == nil || !rows[0].StoppedAt.Equal(ended) || rows[0].ExitCode == nil || *rows[0].ExitCode != 0 {
		t.Fatalf("record = %+v, want stopped at %v with exit 0", rows[0], ended)
	}

	// Five minutes after the command's end the sandbox is idle, and with no
	// open record nothing is probed.
	f.now = ended.Add(5*time.Minute + time.Second)
	if n := f.sweeper.Sweep(context.Background()); n != 1 {
		t.Fatalf("suspended %d sandboxes past the window after the command, want 1", n)
	}
	if got := len(f.probes()); got != 2 {
		t.Fatalf("probed %d times, want no probe once the record closed", got)
	}
}

// When the registry cannot be read, the sandbox stays up until the next sweep.
func TestAnUnreadableRegistryKeepsTheSandboxUp(t *testing.T) {
	f, _ := commandsFixture(t)
	f.answerProbe("", 1)
	if n := f.sweeper.Sweep(context.Background()); n != 0 {
		t.Fatalf("suspended %d sandboxes on an unreadable registry", n)
	}
	if f.activity.claimed("os-1") {
		t.Fatal("the claim was left behind")
	}
}

// A sandbox whose commands all reported their exit pays for no probe.
func TestASandboxWithNoOpenCommandIsNotProbed(t *testing.T) {
	f, execs := commandsFixture(t)
	execs.rows[0].StoppedAt = &f.now
	if n := f.sweeper.Sweep(context.Background()); n != 1 || len(f.probes()) != 0 {
		t.Fatalf("suspended %d with %d probes, want 1 and none", n, len(f.probes()))
	}
}

// A history read asks the registry about a record its stream stopped
// refreshing: a running command stays open, an ended one closes at its own
// exit with its exit code, and an unreadable registry ends nothing.
func TestAHistoryReadAsksTheRegistryAboutStaleRecords(t *testing.T) {
	f, _ := commandsFixture(t)
	f.now = f.now.Add(executionStaleAfter)
	read := func() Execution {
		t.Helper()
		got, err := f.svc.GetExecution(callerCtx(), "os-1", "exe-1")
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	f.answerProbe("running exe-1\n", 0)
	if got := read(); got.StoppedAt != nil {
		t.Fatalf("running command read %+v, want open", got)
	}
	f.now = f.now.Add(executionStaleAfter + time.Minute)
	f.answerProbe("", 1)
	if got := read(); got.StoppedAt != nil {
		t.Fatalf("an unreadable registry ended %+v", got)
	}
	ended := f.now.Add(-time.Minute)
	f.answerProbe(fmt.Sprintf("exited exe-1 3 %d\n", ended.Unix()), 0)
	if got := read(); got.StoppedAt == nil || !got.StoppedAt.Equal(ended) || got.ExitCode == nil || *got.ExitCode != 3 {
		t.Fatalf("ended command read %+v, want stopped at %v with exit 3", got, ended)
	}
	if last := f.activity.lastActivity("os-1"); !last.Equal(ended) {
		t.Errorf("activity = %v, want the command's end %v", last, ended)
	}
}

// A suspended sandbox's registry is not asked: its commands died with the
// pause, so their records end at their last refresh, as before.
func TestASuspendedSandboxsStaleRecordsEndAtTheirLastRefresh(t *testing.T) {
	f, _ := commandsFixture(t)
	f.os.set("os-1", func(b *fakeBox) { b.state = "Paused" })
	f.now = f.now.Add(executionStaleAfter)
	got, err := f.svc.GetExecution(callerCtx(), "os-1", "exe-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.StoppedAt == nil || got.ExitCode != nil || len(f.probes()) != 0 {
		t.Fatalf("record %+v after %d probes, want stopped without an exit and no probe", got, len(f.probes()))
	}
}

// ownStartTime is this test process's /proc start time, as the wrapper
// records its own.
func ownStartTime(t *testing.T) string {
	t.Helper()
	stat, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		t.Skip("no /proc on this host")
	}
	fields := strings.Fields(string(stat[bytes.LastIndex(stat, []byte(") "))+2:]))
	return fields[19]
}

// Between the command's exit and the wrapper logging it, the command reads as
// running because its wrapper is: never as unknown, which would end its
// record with no exit code. Once both are gone, an unlogged command is
// unknown.
func TestACommandWhoseWrapperIsStillLoggingItIsRunning(t *testing.T) {
	needProc(t)
	dir := t.TempDir()
	self := strconv.Itoa(os.Getpid()) + " " + ownStartTime(t)
	for id, entry := range map[string]string{
		"exe-ending": "999999999 1 " + self + "\n",
		"exe-killed": "999999999 1 999999998 1\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, id+".run"), []byte(entry), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := probeRegistry(t, dir, "exe-ending", "exe-killed"); got != "running exe-ending\n" {
		t.Fatalf("probe = %q, want only the command whose wrapper is alive", got)
	}
}

// The probe runs under the workspace's own subject, never the Completer's
// or the caller's: one tenant's probes cannot take another's gateway slots.
func TestTheProbeRunsUnderTheWorkspacesProbeSubject(t *testing.T) {
	f, _ := commandsFixture(t)
	f.answerProbe("running exe-1\n", 0)
	f.sweeper.Sweep(context.Background())
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.probeSubject != sandboxexec.ProbeSubjectPrefix+"tea-a" {
		t.Fatalf("probe subject = %q", f.probeSubject)
	}
}

// A record the registry does not know (its wrapper was killed, or it predates
// the registry) ends at its last refresh, and the sandbox then suspends.
func TestARecordTheRegistryDoesNotKnowEndsAndTheSandboxSuspends(t *testing.T) {
	f, execs := commandsFixture(t)
	f.answerProbe("", 0)
	if n := f.sweeper.Sweep(context.Background()); n != 1 {
		t.Fatalf("suspended %d, want the sandbox with only an unknown record", n)
	}
	if rows := execs.snapshot(); rows[0].StoppedAt == nil || !rows[0].StoppedAt.Equal(rows[0].StartedAt) || rows[0].ExitCode != nil {
		t.Fatalf("record = %+v, want stopped at its last refresh with no exit code", rows[0])
	}
}

// The tenant can write the exit log: an end in the future is held to the
// present, so it cannot pin the sandbox awake or report a stop to come.
func TestAnExitInTheFutureIsHeldToThePresent(t *testing.T) {
	f, execs := commandsFixture(t)
	f.answerProbe(fmt.Sprintf("exited exe-1 0 %d\n", f.now.Add(240*time.Hour).Unix()), 0)
	f.sweeper.Sweep(context.Background())
	if rows := execs.snapshot(); rows[0].StoppedAt == nil || !rows[0].StoppedAt.Equal(f.now) {
		t.Fatalf("record = %+v, want stopped now", rows[0])
	}
	if last := f.activity.lastActivity("os-1"); !last.Equal(f.now) {
		t.Fatalf("activity = %v, want now", last)
	}
}

// An unreadable registry keeps the sandbox up for another window, with its
// activity touched so the next probe waits that window, but only for
// probeFailureGrace after the record was last known.
func TestAnUnreadableRegistryKeepsTheSandboxUpOnlyForAWhile(t *testing.T) {
	f, execs := commandsFixture(t)
	f.answerProbe("", 1)
	if n := f.sweeper.Sweep(context.Background()); n != 0 || !f.activity.lastActivity("os-1").Equal(f.now) {
		t.Fatalf("suspended %d, activity %v: want kept up and touched now", n, f.activity.lastActivity("os-1"))
	}
	f.now = execs.snapshot()[0].StartedAt.Add(probeFailureGrace + time.Minute)
	if n := f.sweeper.Sweep(context.Background()); n != 1 {
		t.Fatalf("suspended %d past the grace, want 1", n)
	}
}
