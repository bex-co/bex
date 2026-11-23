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

// Command native-init is the entrypoint of every native service image
// (w8/m88, w8/290): it runs the image's start command as its one child and
// forwards every signal it receives to it.
//
// Being PID 1 is the first reason. Linux delivers a signal from outside the
// container to PID 1 only when it has a handler for it, so a start command
// running as PID 1 with no SIGTERM handler, a shell running a script that
// does not exec the app, or a handler-less app, never stopped on SIGTERM.
//
// What it adds over running the command as PID 1 is the end of a shutdown.
// `npm start` runs its script under `sh -c`; on SIGTERM npm forwards the
// signal to that shell, which dies of it, and npm exits at once — orphaning
// the app before it ever saw the signal, so the container ended mid-request.
// When the command exits after a termination signal, native-init passes the
// signal on to every process it left behind and waits for them, so the app
// drains within the pod's grace period, which kubelet still enforces.
//
// A process that already received the signal from the command before it
// exited gets it a second time; npm, Yarn and pnpm do not forward to the app
// itself, and a directly launched app is the command, signalled once.
//
// When the command exits on its own, native-init exits with its status at
// once; as PID 1 that ends whatever it left running, exactly as the container
// always has. The status is the command's own, or 128 plus the signal that
// killed it.
//
// Before the command starts, native-init links each secret file mounted at
// /etc/secrets from the working directory, the service root (linkSecretFiles),
// then names the command on stdout as Render's native launcher does
// (announce).
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/bex-co/bex/lego/types/renderlog"
)

// forwarded are the signals a container runtime or operator sends a service.
var forwarded = []os.Signal{
	syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP,
	syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGWINCH,
}

// quietEnv is build.NativeInitQuietEnv: set on a launch whose command the
// platform names itself, a pre-deploy step, whose `==> Running '<command>'`
// the deploy log narrates (w32/m572, w32/m575). native-init then writes no
// line, and removes the variable from the command's environment either way.
const quietEnv = "BEX_NATIVE_INIT_QUIET"

func main() {
	os.Exit(run(os.Args[1:], secretFilesDir, os.Stdout, os.Stderr))
}

func run(argv []string, secrets string, stdout, stderr io.Writer) int {
	if len(argv) == 0 {
		_, _ = fmt.Fprintln(stderr, "native-init: no command to run")
		return 2
	}
	_, quiet := os.LookupEnv(quietEnv)
	_ = os.Unsetenv(quietEnv)
	linkSecretFiles(secrets, ".", stderr)
	// PID 1 adopts orphans already; anywhere else they are claimed.
	asInit := os.Getpid() == 1
	if !asInit {
		becomeSubreaper()
	}
	signals := make(chan os.Signal, 32)
	signal.Notify(signals, append(forwarded, syscall.SIGCHLD)...)
	path, err := exec.LookPath(argv[0])
	child := 0
	if err == nil {
		if !quiet {
			announce(argv, stdout)
		}
		child, err = start(path, argv, asInit)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "native-init: %v\n", err)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, exec.ErrNotFound) {
			return 127
		}
		return 126
	}
	return supervise(child, asInit, signals)
}

// announce writes Render's `==> Running '<command>'` for a command run as
// `<shell> -c <command>`, as a native image runs a service's start command, a
// cron run, a pre-deploy step and the initial deploy hook. Render's native
// instance writes it each time it starts the command, at a deploy, after a
// crash and at a free instance's spin-up (ivanseibel/tretta's log capture,
// DerekSopok/EMS-Kahoot#3), in green as that capture of Render's log API has
// it (renderlog.Instance, w32/m576). The command is quoted as configured,
// $PORT unexpanded, and the line is one write before the command starts, so
// its own output never splits it. Any other argument shape names nothing, and
// a shell that cannot be found starts nothing to name.
func announce(argv []string, stdout io.Writer) {
	if len(argv) < 3 || argv[1] != "-c" {
		return
	}
	if shell := filepath.Base(argv[0]); shell != "bash" && shell != "sh" {
		return
	}
	_, _ = io.WriteString(stdout, renderlog.Instance.Line("Running '"+argv[2]+"'")+"\n")
}

// start launches the command, found at path, with the inherited stdio,
// environment and working directory. As PID 1 the child keeps native-init's
// process group, and so the terminal's foreground; otherwise it leads its
// own, which is how native-init later reaches what it leaves behind.
func start(path string, argv []string, asInit bool) (int, error) {
	return syscall.ForkExec(path, argv, &syscall.ProcAttr{
		Env:   os.Environ(),
		Files: []uintptr{0, 1, 2},
		Sys:   &syscall.SysProcAttr{Setpgid: !asInit},
	})
}

// supervise forwards signals to the child and reaps every process that
// becomes native-init's, returning the child's status once nothing is left to
// wait for — or at once when the child exits without being asked to stop.
// Every signal is followed by a reap, so a SIGCHLD the full channel dropped
// is still caught on the next delivery.
func supervise(child int, asInit bool, signals <-chan os.Signal) int {
	status, exited := 0, false
	var stopping syscall.Signal
	leftovers := func(sig syscall.Signal) {
		if asInit {
			_ = syscall.Kill(-1, sig) // everything in the container but native-init
		} else {
			_ = syscall.Kill(-child, sig)
		}
	}
	for sig := range signals {
		if s, ok := sig.(syscall.Signal); ok && s != syscall.SIGCHLD {
			if stopping == 0 && (s == syscall.SIGTERM || s == syscall.SIGINT || s == syscall.SIGQUIT) {
				stopping = s
			}
			if exited {
				leftovers(s)
			} else {
				_ = syscall.Kill(child, s)
			}
		}
		for {
			var ws syscall.WaitStatus
			pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
			if errors.Is(err, syscall.ECHILD) && exited {
				return status
			}
			if pid <= 0 {
				break
			}
			if pid != child {
				continue
			}
			status, exited = exitCode(ws), true
			if stopping == 0 {
				return status
			}
			leftovers(stopping)
		}
	}
	return status
}

func exitCode(ws syscall.WaitStatus) int {
	if ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return ws.ExitStatus()
}
