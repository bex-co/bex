// Command bex runs the upstream Render CLI against a Bex control plane.
//
// The launcher owns only process configuration. Command parsing, output, and
// API client behavior remain in github.com/render-oss/cli so that a Bex CLI
// upgrade is an explicit upstream dependency update rather than a fork.
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bex-co/bex/cli/internal/bridge"
	"github.com/bex-co/bex/cli/internal/code"
	"github.com/bex-co/bex/cli/internal/update"
	"github.com/render-oss/cli/cmd"
	"github.com/render-oss/cli/pkg/cfg"
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
	// The Bex-native coding commands (`bex code`, `bex glm`, …) are additions
	// to the imported command tree; the upstream commands remain untouched.
	cmd.RootCmd.AddCommand(code.Commands()...)

	// Own the version path: upstream's handler compares against
	// render-oss/cli releases (const cfg.RepoURL) and would direct bex users
	// to Render's upgrade docs.
	if update.IsRootVersionRequest(os.Args[1:], cmd.RootCmd.PersistentFlags()) {
		printVersion(os.Stdout)
		os.Exit(0)
	}

	notice := startUpdateCheck()
	exitCode := cmd.Execute()
	printUpdateNotice(os.Stderr, notice)
	os.Exit(exitCode)
}

// printVersion prints bex's identity and, when permitted, the result of an
// explicit (synchronous) update check against bex's own release channel.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "bex v%s (Render CLI v%s compatible)\n", bexVersion, cfg.Version)
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
		fmt.Fprintln(w, "You are using the latest version")
	}
}

// startUpdateCheck begins the gh-style passive check concurrently with the
// command so most invocations never wait on the network; nil means fully
// gated off.
func startUpdateCheck() <-chan *update.Release {
	if !update.Allowed(bexVersion, os.LookupEnv, stderrIsTTY) {
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
	fmt.Fprintf(w, "\nA new release of bex is available: v%s → v%s\n%s\n", bexVersion, release.Version, release.URL)
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
