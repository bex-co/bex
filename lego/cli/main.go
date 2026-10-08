// Command bex runs the upstream Render CLI against a Bex control plane.
//
// The launcher owns only process configuration. Command parsing, output, and
// API client behavior remain in github.com/render-oss/cli so that a Bex CLI
// upgrade is an explicit upstream dependency update rather than a fork.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/cli/internal/branding"
	"github.com/bex-co/bex/lego/cli/internal/bridge"
	"github.com/bex-co/bex/lego/cli/internal/code"
	"github.com/bex-co/bex/lego/cli/internal/pgtrust"
	"github.com/bex-co/bex/lego/cli/internal/telemetry"
	"github.com/bex-co/bex/lego/cli/internal/update"
	"github.com/bex-co/bex/lego/cli/internal/upgrade"
	"github.com/render-oss/cli/cmd"
	"github.com/render-oss/cli/pkg/cfg"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// bexVersion is bex's own release identity, injected by
// scripts/bex-cli-build.sh from the bex-cli/vX.Y.Z tag. It is deliberately
// separate from cfg.Version, the pinned upstream release that also names the
// User-Agent — the server-side compatibility ledger depends on that staying
// truthful.
var bexVersion = "dev"

func main() {
	if err := bridge.Apply(); err != nil {
		fmt.Fprintf(os.Stderr, "bex: configure upstream CLI: %v\n", err)
		os.Exit(1)
	}
	// Must follow Apply, which resolves the control-plane host this stamps
	// against. It covers the detached analytics sender too: that subprocess
	// re-executes this binary, so it runs main and installs the same wrapper.
	bridge.InstallControlPlaneHeaders(bexVersion)
	// The Bex-native coding commands (`bex code`, `bex glm`, …) and the
	// self-update command are additions to the imported command tree; the
	// upstream commands remain untouched.
	cmd.RootCmd.AddCommand(code.Commands()...)
	cmd.RootCmd.AddCommand(upgrade.Command(bexVersion))
	// Branding mutates exported cobra metadata (Use/help/docs) after native
	// commands attach so ungrouped Bex commands keep their help section.
	branding.Apply(cmd.RootCmd, bexVersion)
	// Brands the children upstream Execute registers after Apply before any
	// command, including __complete, runs (w2/043). The BEX_WORKSPACE name
	// resolves here too, once the complete tree can name the invoked command:
	// cobra skips initializers for --help/--version, and workspaceUse spares
	// the commands that never read the active workspace (w8/064).
	cobra.OnInitialize(func() {
		branding.Refresh(cmd.RootCmd)
		if err := bridge.ResolveWorkspaceName(workspaceUse(cmd.RootCmd, os.Args[1:])); err != nil {
			fmt.Fprintf(os.Stderr, "bex: %v\n", err)
			os.Exit(1)
		}
	})

	// Own the version path: upstream's handler compares against
	// render-oss/cli releases (const cfg.RepoURL) and would direct bex users
	// to Render's upgrade docs.
	if update.IsRootVersionRequest(os.Args[1:], cmd.RootCmd.PersistentFlags()) {
		startedAt := time.Now()
		printVersion(os.Stdout)
		// This path exits without reaching cmd.Execute(), so upstream's
		// post-run analytics hook never sees it.
		telemetry.EmitVersion(cmd.RootCmd.CommandPath(), startedAt, os.Stderr)
		os.Exit(0)
	}

	// Own the Postgres session paths the same way: a bex external connection
	// string pins sslmode=verify-full against a private CA, which upstream
	// cannot provision because it has no notion of the bex CA field.
	releaseTrust, err := provisionPostgresTrust(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "bex: %v\n", err)
		os.Exit(1)
	}

	// `bex login` gets the explicit check, the way upstream's login ended with
	// its own release banner — which the bridge suppresses because it
	// advertised render-oss/cli releases. Every other command keeps the
	// passive, TTY-only notice.
	login := loginCommand(os.Args[1:])
	ttyGate := stderrIsTTY
	if login != nil {
		ttyGate = nil
	}
	notice := startUpdateCheck(ttyGate)
	exitCode := cmd.Execute()
	releaseTrust()
	if login == nil || loginCompleted(login, exitCode) {
		printUpdateNotice(os.Stderr, notice)
	}
	os.Exit(exitCode)
}

// loginCommand returns the upstream `login` command when args invoke it.
func loginCommand(args []string) *cobra.Command {
	target, _, err := cmd.RootCmd.Find(args)
	if err != nil || target == nil || target.Name() != "login" || target.Parent() != cmd.RootCmd {
		return nil
	}
	return target
}

// loginCompleted reports whether the login run ended authenticated, as
// opposed to failing or only printing its help.
func loginCompleted(login *cobra.Command, exitCode int) bool {
	if exitCode != 0 {
		return false
	}
	help := login.Flags().Lookup("help")
	return help == nil || !help.Changed
}

// provisionPostgresTrust points a `psql`/`pgcli` invocation at its database's
// TLS server CA. It resolves the target command through cobra's own lookup, so
// which invocations count is decided by the upstream command tree rather than
// by a second parser that could disagree with it.
//
// The returned release is never nil; it removes the temporary CA once the
// delegated command has exited.
func provisionPostgresTrust(args []string) (func(), error) {
	target, rest, err := cmd.RootCmd.Find(args)
	if err != nil || target == nil || !pgtrust.IsProvisionedTool(target.Name()) {
		return func() {}, nil
	}
	// Budgeted: this read sits in front of an interactive session, so a stalled
	// control plane must not hang the terminal — it falls back to today's
	// behavior, where the delegated command reports the failure itself.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return pgtrust.Provision(ctx, pgtrust.Options{
		Tool:   target.Name(),
		Args:   rest,
		Arity:  pgtrust.ArityOf(cmd.RootCmd.PersistentFlags(), target.InheritedFlags(), target.Flags()),
		Fetch:  pgtrust.NewFetcher(),
		Lookup: os.LookupEnv,
		Setenv: os.Setenv,
		Home:   os.UserHomeDir,
	})
}

// printVersion prints bex's identity and, when permitted, the result of an
// explicit (synchronous) update check against bex's own release channel.
func printVersion(w io.Writer) {
	_, _ = fmt.Fprintf(w, "bex v%s\n", bexVersion)
	_, _ = fmt.Fprintf(w, "compatible with Render CLI v%s\n", cfg.Version)
	// The user asked, so no TTY gate — but CI stays silent and a dev build
	// has nothing to compare.
	if !update.Allowed(bexVersion, os.LookupEnv, nil) {
		return
	}
	release, ok := latestRelease()
	if !ok {
		return
	}
	if update.Newer(bexVersion, release.Version) {
		printUpgradeHint(w, release)
	} else {
		_, _ = fmt.Fprintln(w, "You are using the latest version")
	}
}

// startUpdateCheck begins the gh-style check concurrently with the command so
// most invocations never wait on the network; nil means fully gated off. A
// nil isTTY drops the interactive gate (see update.Allowed).
func startUpdateCheck(isTTY func() bool) <-chan *update.Release {
	if !update.Allowed(bexVersion, os.LookupEnv, isTTY) {
		return nil
	}
	ch := make(chan *update.Release, 1)
	go func() {
		if release, ok := latestRelease(); ok && update.Newer(bexVersion, release.Version) {
			ch <- &release
			return
		}
		ch <- nil
	}()
	return ch
}

// printUpdateNotice waits for the concurrent check only as long as one fetch
// can take, and only a cache-miss run can wait at all: Latest caches every
// outcome (success, empty, or failure) for 24h, so this budget is paid at
// most once a day and buys a durably written cache plus the notice.
func printUpdateNotice(w io.Writer, ch <-chan *update.Release) {
	if ch == nil {
		return
	}
	select {
	case release := <-ch:
		if release != nil {
			printUpgradeHint(w, *release)
		}
	case <-time.After(4 * time.Second):
	}
}

func printUpgradeHint(w io.Writer, release update.Release) {
	_, _ = fmt.Fprintf(w, "\nA new release of bex is available: v%s → v%s\n%s\nTo upgrade, run: %s\n",
		bexVersion, release.Version, release.URL, upgrade.InstructionCommand())
}

// latestRelease resolves the newest bex-cli release; ok is false when the
// check failed or a cached failure means there is nothing to report.
func latestRelease() (update.Release, bool) {
	release, err := update.NewChecker(os.LookupEnv).Latest()
	if err != nil || release.Version == "" {
		return update.Release{}, false
	}
	return release, true
}

func stderrIsTTY() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// workspaceIndependent are the command paths (below the root) that never read
// the active workspace: inventory, identity, an explicit selection, auth,
// shell completion and Bex's own self-update.
var workspaceIndependent = []string{
	"workspaces", "workspace set", "whoami", "login", "logout", "help",
	"completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd, "upgrade",
}

// typedResourceID matches Render's typed ids (srv-…, dpg-…, red-…); it mirrors
// upstream pkg/validate/id.go's per-prefix ^<prefix>-[a-z0-9]{20}$.
var typedResourceID = regexp.MustCompile(`^[a-z]{3}-[0-9a-z]{20}$`)

// workspaceUse classifies the invoked command for ResolveWorkspaceName (w8/064).
// A group command that only prints help, an explicit --workspace/-w selection
// and the commands above need no active workspace; a command whose first
// positional is a typed resource id reads that id directly. Everything else
// consumes it.
func workspaceUse(root *cobra.Command, args []string) bridge.WorkspaceUse {
	target, rest, err := root.Find(args)
	if err != nil || target == nil {
		return bridge.WorkspaceRequired
	}
	if !target.Runnable() {
		return bridge.WorkspaceUnused
	}
	path := strings.TrimPrefix(target.CommandPath(), root.Name()+" ")
	for _, independent := range workspaceIndependent {
		if path == independent || strings.HasPrefix(path, independent+" ") {
			return bridge.WorkspaceUnused
		}
	}
	if flag := target.Flags().Lookup("workspace"); flag != nil && selectsWorkspace(rest, flag.Shorthand) {
		return bridge.WorkspaceUnused
	}
	arity := pgtrust.ArityOf(root.PersistentFlags(), target.InheritedFlags(), target.Flags())
	if typedResourceID.MatchString(pgtrust.DatabaseArg(rest, arity)) {
		return bridge.WorkspaceIfResolvable
	}
	return bridge.WorkspaceRequired
}

// selectsWorkspace reports whether args pass --workspace (or its shorthand,
// attached or separate) before any `--` terminator.
func selectsWorkspace(args []string, shorthand string) bool {
	for _, arg := range args {
		switch {
		case arg == "--":
			return false
		case arg == "--workspace" || strings.HasPrefix(arg, "--workspace="):
			return true
		case shorthand != "" && strings.HasPrefix(arg, "-"+shorthand):
			return true
		}
	}
	return false
}
