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
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// runExecArgv runs execArgv's argv the way the gateway's pods/exec does, under
// this host's real /bin/sh, with the given environment and starting directory.
func runExecArgv(t *testing.T, command string, inHome bool, dir string, env ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	needSh(t)
	argv := agentExecArgv(command)
	if inHome {
		argv = execArgv(command, "")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	code := commandExitCode(t, cmd.Run())
	return out.String(), errOut.String(), code
}

func needSh(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh on this host")
	}
}

// pathWithBash is a PATH holding only a link to this host's bash, so the
// wrapper's `command -v bash` finds it and nothing else leaks in.
func pathWithBash(t *testing.T) string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash on this host")
	}
	dir := t.TempDir()
	if err := os.Symlink(bash, filepath.Join(dir, "bash")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// w32/m348: Render runs a command under `bash -c` in a new shell in the
// sandbox's home directory, so a relative upload runs by its relative path and
// bash syntax works without a `bash -c` prefix.
func TestExecRunsTheCommandUnderBashInTheHomeDirectory(t *testing.T) {
	home := fileRoundtripTempDir(t)
	if err := os.WriteFile(filepath.Join(home, "run.sh"), []byte("echo from-script\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := []string{"HOME=" + home, "PATH=" + pathWithBash(t)}

	if out, _, code := runExecArgv(t, "pwd", true, "/", env...); code != 0 || out != home+"\n" {
		t.Fatalf("pwd = %q (exit %d), want the home directory %q", out, code, home)
	}
	if out, errOut, code := runExecArgv(t, "/bin/sh run.sh", true, "/", env...); code != 0 || out != "from-script\n" {
		t.Fatalf("sh run.sh = %q, %q (exit %d), want the relative upload to run", out, errOut, code)
	}
	out, errOut, code := runExecArgv(t, `arr=(a b); echo ${arr[1]}; echo {1..3}; echo "$0"`, true, "/", env...)
	if code != 0 || out != "b\n1 2 3\nbash\n" {
		t.Fatalf("bash syntax = %q, %q (exit %d), want b, 1 2 3 and $0 bash", out, errOut, code)
	}
	// The exit code and the stdout/stderr split are the command's own.
	out, errOut, code = runExecArgv(t, `echo out; echo err >&2; exit 7`, true, "/", env...)
	if code != 7 || out != "out\n" || errOut != "err\n" {
		t.Fatalf("exit = %d, stdout %q, stderr %q; want 7, out, err", code, out, errOut)
	}
	// The command is one argument, never shell source of the wrapper: quotes,
	// `$1` and a `;` in it reach bash exactly as the caller wrote them.
	out, _, code = runExecArgv(t, `printf '%s|' "$1" 'a  b' ; echo`, true, "/", env...)
	if code != 0 || out != "|a  b|\n" {
		t.Fatalf("quoted command = %q (exit %d), want |a  b|", out, code)
	}
}

// A home directory that does not exist, or no HOME at all, starts the command
// in / rather than failing it.
func TestExecFallsBackToTheRootDirectoryWithoutAUsableHome(t *testing.T) {
	path := "PATH=" + pathWithBash(t)
	for name, env := range map[string][]string{
		"missing home": {"HOME=/nonexistent/bex-m348", path},
		"unset home":   {path},
	} {
		if out, errOut, code := runExecArgv(t, "pwd", true, fileRoundtripTempDir(t), env...); code != 0 || out != "/\n" || errOut != "" {
			t.Errorf("%s: pwd = %q, %q (exit %d), want / and no error", name, out, errOut, code)
		}
	}
}

// A root filesystem with no bash (a sandbox suspended from the Alpine base
// before w32/m348) still runs the command, through /bin/sh in $HOME.
func TestExecFallsBackToShWhenTheRootFilesystemHasNoBash(t *testing.T) {
	home := fileRoundtripTempDir(t)
	env := []string{"HOME=" + home, "PATH=" + t.TempDir()}
	out, errOut, code := runExecArgv(t, `echo "$0"; pwd; exit 4`, true, "/", env...)
	if code != 4 || out != "/bin/sh\n"+home+"\n" {
		t.Fatalf("no-bash exec = %q, %q (exit %d), want /bin/sh, the home directory and exit 4", out, errOut, code)
	}
}

// An agent-session sandbox keeps the image's working directory, the session's
// checkout, and still runs under bash (w32/m348 t004; Devin's shell starts in
// the repository).
func TestAgentSessionExecKeepsItsWorkingDirectory(t *testing.T) {
	home, workspace := fileRoundtripTempDir(t), fileRoundtripTempDir(t)
	env := []string{"HOME=" + home, "PATH=" + pathWithBash(t)}
	out, errOut, code := runExecArgv(t, `pwd; echo {1..2}`, false, workspace, env...)
	if code != 0 || out != workspace+"\n1 2\n" {
		t.Fatalf("agent exec = %q, %q (exit %d), want %q then 1 2", out, errOut, code, workspace)
	}
}
