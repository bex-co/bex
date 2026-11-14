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

// Package desktop hands editing off to bex desktop: `bex edit` and the
// path-shaped shorthand `bex <path>` exec the desktop's editor launcher, the
// way `bex code` execs claude. The launcher's names and locations are the
// contract in bex-desktop docs/adrs/ADR002-cli-command-bex-desktop.md.
package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bex-co/bex/lego/cli/internal/telemetry"
)

// LauncherName is the desktop editor launcher's command on PATH (ADR002).
const LauncherName = "bex-desktop"

// installHint names both ways to get the launcher onto the machine.
const installHint = "bex edit opens files in bex desktop, but no bex desktop editor launcher was found.\n" +
	"Install bex desktop from https://bex.co/download, then run \"Install CLI\" from its command palette\n" +
	"(or put `" + LauncherName + "` on your PATH)"

// nonInteractive explains the refusal; a script that really wants a GUI can
// call the launcher itself.
const nonInteractive = "bex edit opens the bex desktop editor, which needs an interactive terminal;\n" +
	"in scripts and CI run `" + LauncherName + "` directly"

// macApps are the channel builds of the desktop app, stable first (ADR002).
var macApps = []string{"Bex.app", "Bex Preview.app", "Bex Nightly.app", "Bex Dev.app"}

// system is the process boundary the command touches, injectable for tests.
type system struct {
	goos        string
	lookPath    func(string) (string, error)
	stat        func(string) (os.FileInfo, error)
	glob        func(string) ([]string, error)
	evalLinks   func(string) (string, error)
	executable  func() (string, error)
	home        func() (string, error)
	interactive func() bool
	exec        func(path string, argv []string) error
	session     func(ctx context.Context, id string) (string, error)
}

func osSystem() system {
	return system{
		goos:       runtime.GOOS,
		lookPath:   exec.LookPath,
		stat:       os.Stat,
		glob:       filepath.Glob,
		evalLinks:  filepath.EvalSymlinks,
		executable: os.Executable,
		home:       os.UserHomeDir,
		interactive: func() bool {
			return !cfg.IsCI() && (term.IsTerminal(int(os.Stdout.Fd())) || term.IsTerminal(int(os.Stderr.Fd())))
		},
		exec: func(path string, argv []string) error {
			return syscall.Exec(path, argv, os.Environ())
		},
		session: sessionSSHAddress,
	}
}

// Command is `bex edit`. Flag parsing is disabled: every argument belongs to
// the desktop launcher, which resolves paths against the working directory it
// inherits.
func Command() *cobra.Command {
	sys := osSystem()
	return &cobra.Command{
		Use:   "edit [path[:line[:col]]… | ags-<id>] [editor flags]",
		Short: "Open files, folders or an agent session's sandbox in bex desktop",
		Long: "Opens paths in the bex desktop editor by handing every argument to its\n" +
			"launcher (`bex-desktop`) unchanged: paths, path:line:col, -n/--new,\n" +
			"-a/--add, -e/--existing, -w/--wait, - for stdin, --diff OLD NEW and\n" +
			"ssh:// URLs.\n\n" +
			"An agent-session id (ags-…) opens that session's sandbox /workspace over\n" +
			"SSH, like the dashboard's Open in Zed. The sandbox must be running and you\n" +
			"need an SSH key registered with bex.\n\n" +
			"Shorthand: `bex <path>` is `bex edit <path>` when <path> exists and looks\n" +
			"like a path — `.`, `..`, or starting with ./ ../ / ~ or containing a /.\n" +
			"A bare word is always a command: `bex docs` runs docs; `bex ./docs`\n" +
			"opens the folder.",
		Example:            "  bex edit .\n  bex edit main.go:42 -n\n  bex .\n  bex edit ags-d0abc123",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
				return c.Help()
			}
			startedAt := time.Now()
			return run(c.Context(), sys, args, func() {
				telemetry.EmitLaunch(c.CommandPath(), startedAt, c.ErrOrStderr())
			})
		},
	}
}

// Available reports whether `bex edit` can open bex desktop here: an
// interactive terminal and an installed launcher.
func Available() bool {
	sys := osSystem()
	if !sys.interactive() {
		return false
	}
	_, err := locate(sys)
	return err == nil
}

// OpenSession is `bex edit <ags-id>`: it replaces this process with the
// launcher opening the session sandbox's /workspace over SSH. A caller that
// already read the session passes its sshAddress so it is not read again;
// "" looks it up.
func OpenSession(ctx context.Context, id, sshAddress string, beforeExec func()) error {
	sys := osSystem()
	if sshAddress != "" {
		sys.session = func(context.Context, string) (string, error) { return sshAddress, nil }
	}
	return run(ctx, sys, []string{id}, beforeExec)
}

// run resolves the launcher and any session target, then replaces this
// process with the launcher. beforeExec runs immediately before that
// replacement.
func run(ctx context.Context, sys system, args []string, beforeExec func()) error {
	if !sys.interactive() {
		return errors.New(nonInteractive)
	}
	launcher, err := locate(sys)
	if err != nil {
		return err
	}
	argv, err := sessionTargets(ctx, sys, args)
	if err != nil {
		return err
	}
	beforeExec()
	if err := sys.exec(launcher, append([]string{LauncherName}, argv...)); err != nil {
		return fmt.Errorf("exec %s: %w", launcher, err)
	}
	return nil
}

// locate finds the desktop editor launcher: `bex-desktop` on PATH, then the
// bundle-internal launcher of an installed app (ADR002 § Stable contract).
// It never looks up `bex` on PATH — a pre-rename desktop link and this very
// binary both answer to that name — and refuses any candidate that resolves
// to the running executable.
func locate(sys system) (string, error) {
	self := ""
	if exe, err := sys.executable(); err == nil {
		self = resolved(sys, exe)
	}
	for _, candidate := range candidates(sys) {
		info, err := sys.stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if self != "" && resolved(sys, candidate) == self {
			continue
		}
		return candidate, nil
	}
	return "", errors.New(installHint)
}

func candidates(sys system) []string {
	var out []string
	if path, err := sys.lookPath(LauncherName); err == nil {
		out = append(out, path)
	}
	home, _ := sys.home()
	switch sys.goos {
	case "darwin":
		dirs := []string{"/Applications"}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Applications"))
		}
		for _, app := range macApps {
			for _, dir := range dirs {
				// Contents/MacOS/bex is the editor itself, never the launcher.
				out = append(out, filepath.Join(dir, app, "Contents", "MacOS", "cli"))
			}
		}
	case "linux":
		if home == "" {
			break
		}
		// The stable channel first, then other channels by name; builds
		// before 0.139 shipped bin/cli.
		stable := filepath.Join(home, ".local", "bex.app")
		channels, _ := sys.glob(filepath.Join(home, ".local", "bex-*.app"))
		for _, app := range append([]string{stable}, channels...) {
			out = append(out, filepath.Join(app, "bin", "bex"), filepath.Join(app, "bin", "cli"))
		}
	}
	return out
}

func resolved(sys system, path string) string {
	if real, err := sys.evalLinks(path); err == nil {
		return real
	}
	return path
}

// sessionTargets replaces each ags-… argument that is not an existing path
// with the session sandbox's SSH workspace URL.
func sessionTargets(ctx context.Context, sys system, args []string) ([]string, error) {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = arg
		if !isSessionID(arg) {
			continue
		}
		if _, err := sys.stat(arg); err == nil {
			continue
		}
		address, err := sys.session(ctx, arg)
		if err != nil {
			return nil, err
		}
		out[i] = "ssh://" + address + "/workspace"
	}
	return out, nil
}

// isSessionID matches the typed agent-session id shape `ags-<xid>`.
func isSessionID(arg string) bool {
	rest, ok := strings.CutPrefix(arg, "ags-")
	if !ok || rest == "" {
		return false
	}
	for _, r := range rest {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
