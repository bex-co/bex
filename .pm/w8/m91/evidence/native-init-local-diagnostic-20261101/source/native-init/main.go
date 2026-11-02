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

// Command native-init is the entrypoint of a native Node service image
// (w8/m88): it runs the image's start command as its one child and forwards
// every signal it receives to it.
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
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// forwarded are the signals a container runtime or operator sends a service.
var forwarded = []os.Signal{
	syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP,
	syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGWINCH,
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(argv []string, stderr io.Writer) int {
	if len(argv) == 0 {
		_, _ = fmt.Fprintln(stderr, "native-init: no command to run")
		return 2
	}
	// PID 1 adopts orphans already; anywhere else they are claimed.
	asInit := os.Getpid() == 1
	if !asInit {
		becomeSubreaper()
	}
	signals := make(chan os.Signal, 32)
	signal.Notify(signals, append(forwarded, syscall.SIGCHLD)...)
	child, err := start(argv, asInit)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "native-init: %v\n", err)
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, exec.ErrNotFound) {
			return 127
		}
		return 126
	}
	return supervise(child, asInit, signals)
}

// start launches the command with the inherited stdio, environment and
// working directory. As PID 1 the child keeps native-init's process group,
// and so the terminal's foreground; otherwise it leads its own, which is how
// native-init later reaches what it leaves behind.
func start(argv []string, asInit bool) (int, error) {
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return 0, err
	}
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
