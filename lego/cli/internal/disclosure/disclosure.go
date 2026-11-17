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

// Package disclosure is the bex CLI's usage-telemetry disclosure (ADR113 D5).
//
// The imported Render CLI sends nothing outside CI until its one-time notice
// has been shown, and that notice names Render as the recipient. In bex the
// events go to bex-api, so the launcher owns the disclosure, without patching
// upstream:
//
//   - upstream's marker (<state>/analytics/notice-shown) is written on every
//     run, so upstream's notice never prints;
//   - bex's marker (<state>/analytics/bex-notice-shown) is the gate: a run
//     sends only in CI or once the Bex notice was shown on this machine;
//   - an undisclosed run builds upstream's sender with consent denied, and
//     prints the Bex notice after a notice-eligible command on a terminal.
//
// The rule matches Render's (render-oss/cli cmd/execution.go): CI bypasses the
// gate and shows nothing, a run without a terminal on stderr never sends, and
// the run that shows the notice sends nothing.
package disclosure

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/render-oss/cli/pkg/analytics"
	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/style"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	// bexMarker records that the Bex notice was shown on this machine.
	bexMarker = "bex-notice-shown"
	// upstreamMarker is upstream's own record (internal/analyticsnotice
	// markerFileName). Its presence keeps upstream's notice from printing.
	upstreamMarker = "notice-shown"

	// Upstream marks the trees that never draw the notice with this annotation
	// (cmd/execution.go markCommandAnalyticsNoticeIneligible); today that is
	// the hidden `analytics` group.
	noticeEligibilityAnnotation = "render.analytics.notice.eligibility"
	ineligible                  = "ineligible"

	renderDisableAnalytics = "RENDER_CLI_DISABLE_ANALYTICS"
)

// docsURL is the section of the CLI guide the notice links to.
const docsURL = "https://github.com/bex-co/bex/blob/main/docs/bex-cli.md#cli-usage-telemetry"

// The notice, line by line. The Homebrew formula's caveats
// (scripts/bex-cli-formula.sh) carry the consenting variant verbatim, and
// TestFormulaCaveatsAreTheBexNotice keeps the two equal.
const (
	collects   = "The bex CLI collects usage data on commands and performance to help improve it."
	recipient  = "The data goes to Bex, not to Render."
	collected  = "It records command names, outcomes, durations and environment signals, never arguments or values."
	notSentYet = "No data has been sent yet."
	optOutHint = "To opt out, set BEX_CLI_DISABLE_ANALYTICS=1 or DO_NOT_TRACK=1."
	disabled   = "Telemetry is disabled, so nothing is sent."
	learnMore  = "Learn more: " + docsURL
)

// lines is the notice for a user who opted out with optOut, or for a
// consenting user when optOut is empty.
func lines(optOut string) []string {
	if optOut == "" {
		return []string{collects, recipient, collected, notSentYet, "", optOutHint, learnMore}
	}
	return []string{collects, recipient, collected, disabled, "",
		"To enable telemetry, remove " + optOut + " from your environment.", learnMore}
}

// write prints the notice to w: as upstream's tinted block on a terminal,
// as plain lines anywhere else (an installer log, a pipe).
func write(w io.Writer, optOut string) error {
	text := lines(optOut)
	if f, ok := w.(*os.File); !ok || !term.IsTerminal(int(f.Fd())) {
		_, err := fmt.Fprintln(w, strings.Join(text, "\n"))
		return err
	}
	stream := command.NewStream(w)
	block := stream.Renderer().NewStyle().
		Background(style.ColorSubtleBackground).
		Foreground(style.ColorFocus).
		Padding(1, 2)
	// Wide enough for the longest line plus the horizontal padding; a
	// narrower terminal still wraps.
	widest := 0
	for _, line := range text {
		widest = max(widest, len(line))
	}
	if width := stream.Width(); width > 0 {
		block = block.Width(min(width, widest+4))
	}
	_, err := fmt.Fprintf(w, "%s\n\n", block.Render(strings.Join(text, "\n")))
	return err
}

func markerPath(name string) (string, error) {
	state, err := config.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(state, "analytics", name), nil
}

// markerExists reports whether the named marker exists. An error means it
// could not be determined, which every caller treats as "not shown".
func markerExists(name string) (bool, error) {
	path, err := markerPath(name)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func writeMarker(name string) error {
	path, err := markerPath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0o600)
}

// CanSend reports whether the gate lets this run send: in CI, or once the Bex
// notice was shown on this machine. Consent (the opt-outs) is checked
// separately.
func CanSend() bool {
	if cfg.IsCI() {
		return true
	}
	shown, err := markerExists(bexMarker)
	return err == nil && shown
}

// ensureUpstreamMarker writes upstream's marker when it is missing, so the
// imported CLI never prints Render's notice. Best effort: a failure only
// means upstream may print it once on a terminal.
func ensureUpstreamMarker() {
	if shown, err := markerExists(upstreamMarker); err == nil && !shown {
		_ = writeMarker(upstreamMarker)
	}
}

// Gate is the disclosure decision for one process.
type Gate struct {
	// optOut names the variable the user opted out with, "" when consenting.
	optOut string
	// restore undoes the consent override; nil when Begin set nothing.
	restore func()
}

// Begin applies the gate. Call it once, after bridge.Apply (which places the
// state directory and maps BEX_CLI_DISABLE_ANALYTICS) and before cmd.Execute,
// which builds upstream's sender and reads consent once while doing so.
// filled is bridge.FilledFromBex.
func Begin(filled map[string]string) *Gate {
	g := &Gate{optOut: optOutVariable(filled)}
	ensureUpstreamMarker()
	// An opted-out user sends nothing anyway. The send subprocess re-executes
	// this binary for a parent that already passed the gate.
	if g.optOut != "" || CanSend() || analytics.IsSendSubprocess() {
		return g
	}
	previous, had := os.LookupEnv(renderDisableAnalytics)
	if os.Setenv(renderDisableAnalytics, "1") != nil {
		return g
	}
	g.restore = func() {
		if had {
			_ = os.Setenv(renderDisableAnalytics, previous)
		} else {
			_ = os.Unsetenv(renderDisableAnalytics)
		}
	}
	return g
}

// optOutVariable names the variable that denies consent, as the user set it:
// the bridge maps BEX_CLI_DISABLE_ANALYTICS onto RENDER_CLI_DISABLE_ANALYTICS,
// so upstream alone would name a variable the user never set.
func optOutVariable(filled map[string]string) string {
	switch reason := analytics.ResolveConsent().OptOutReason; reason {
	case analytics.OptOutReasonNone:
		return ""
	case analytics.OptOutReasonDisableAnalyticsEnv:
		if bex := filled[renderDisableAnalytics]; bex != "" {
			return bex
		}
		return renderDisableAnalytics
	default:
		return string(reason)
	}
}

// Release restores the consent environment the process started with. Call it
// once the command is selected and before it runs (a cobra initializer):
// upstream's sender was built by then, and an override left in place would
// reach every process the command starts (the agent `bex glm` execs, an
// editor, ssh) and read there as the user's own opt-out.
func (g *Gate) Release() {
	if g.restore != nil {
		g.restore()
		g.restore = nil
	}
}

// ShowIfNeeded prints the Bex notice to stderr after an execution and records
// that it was shown, by upstream's rule: never in CI, never without a terminal
// on stderr, once per machine, and only after a notice-eligible command. args
// are the arguments cobra executed.
func (g *Gate) ShowIfNeeded(root *cobra.Command, args []string, stderr io.Writer, terminal bool) {
	if cfg.IsCI() || !terminal || !noticeEligible(root, args) {
		return
	}
	g.show(stderr)
}

// show prints the notice once per machine and records it. Like upstream, it
// prints nothing when it cannot tell whether the notice was shown, and records
// it only once it printed.
func (g *Gate) show(w io.Writer) {
	if shown, err := markerExists(bexMarker); err != nil || shown {
		return
	}
	if write(w, g.optOut) == nil {
		_ = writeMarker(bexMarker)
	}
}

// AdoptNoticeCommand makes upstream's hidden `analytics notice`, which exists
// so "installers can invoke this command after install or upgrade"
// (cmd/analyticsnotice.go), print the Bex notice instead of Render's. Its
// sibling `analytics send` stays upstream's. Unlike upstream's command it
// prints with or without a terminal, and in CI too: an installer's log is a
// disclosure a headless user can read.
//
// Upstream adds the `analytics` group while Execute sets up, after the
// launcher has wired its tree, so call this from a cobra initializer: it runs
// after the command is selected and before it runs.
func (g *Gate) AdoptNoticeCommand(root *cobra.Command) {
	group := analyticsGroup(root)
	if group == nil {
		return
	}
	for _, child := range group.Commands() {
		if child.Name() == "notice" {
			child.RunE = func(c *cobra.Command, _ []string) error {
				g.show(c.ErrOrStderr())
				return nil
			}
		}
	}
}

// analyticsGroup returns upstream's hidden `analytics` command, which exists
// only once Execute's setup got past building the API client.
func analyticsGroup(root *cobra.Command) *cobra.Command {
	for _, child := range root.Commands() {
		if child.Name() == "analytics" {
			return child
		}
	}
	return nil
}

// noticeEligible applies upstream's rule (cmd/execution.go
// commandIsAnalyticsNoticeEligible) to the command args select: every command
// except shell completion and the trees upstream annotates as ineligible.
//
// Upstream draws its notice from its post-run hook, which a setup failure
// (the API client could not be built) skips; the `analytics` group is added
// by that same setup, so its absence means setup failed.
func noticeEligible(root *cobra.Command, args []string) bool {
	if analyticsGroup(root) == nil {
		return false
	}
	target, _, _ := root.Find(args)
	if target == nil {
		return false
	}
	for c := target; c != nil; c = c.Parent() {
		if c.Annotations[noticeEligibilityAnnotation] == ineligible {
			return false
		}
		if c.Parent() == root {
			switch c.Name() {
			case "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
				return false
			}
		}
	}
	return true
}
