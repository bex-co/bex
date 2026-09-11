// Package telemetry emits analytics events for the launcher-owned command
// paths the upstream CLI's own post-run hook can never observe.
//
// Upstream fires exactly one hook after Cobra finishes an execution
// (render-oss/cli cmd/execution.go), which covers every imported command and
// also bex's native `upgrade` — verified, not assumed. Two bex paths escape it:
//
//   - `bex code` / `bex glm` and the other provider launchers replace the
//     process with syscall.Exec, so the execution never "finishes" and the hook
//     never runs;
//   - root `--version` answers and exits before cmd.Execute() is reached.
//
// Sending is delegated to upstream's own Sender, so consent resolution
// (DO_NOT_TRACK, RENDER_CLI_DISABLE_ANALYTICS — and therefore
// BEX_CLI_DISABLE_ANALYTICS, which the bridge maps onto it), delivery backoff,
// installation identity, and the detached send subprocess all behave exactly as
// they do for an imported command. None of that policy is reimplemented here.
//
// One gate is genuinely not applied: upstream's post-run hook additionally waits
// for its one-time disclosure notice to have been shown, and that check lives in
// an unexported package inside Cobra's post-run path — which, by definition,
// neither of these paths reaches. So a machine whose very first bex invocation
// is `bex glm` or `bex --version` can report before that notice has printed.
// The divergence is deliberate and recorded in docs/runbooks/cli-analytics.md
// and docs/bex-cli.md; bex's own disclosure is the documentation, not Render's
// notice copy, which the compatibility checklist explicitly tells users not to
// act on.
package telemetry

import (
	"io"
	"time"

	"github.com/render-oss/cli/pkg/analytics"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
)

// Narrow seams so a test can prove the ordering below, which is the whole
// point of it: consent is environment-only and free, client construction is
// neither.
var (
	consentGranted = func() bool { return analytics.ResolveConsent().Granted }
	newAPIClient   = client.NewDefaultClient
)

// send is the seam tests replace; production wiring is upstream's Sender.
var send = func(result command.ExecutionResult, stderr io.Writer) {
	// Resolve consent before building anything. analytics.New would check it
	// too, but only after client.NewDefaultClient has already read the config
	// file and — when a stored token is near expiry — spent up to five seconds
	// on a synchronous OAuth refresh. Both of these paths run at a moment where
	// that is not affordable: one is the last statement before the process is
	// replaced by the launched agent, the other is `--version`. Checking first
	// (it reads only environment variables) means an opted-out user pays
	// nothing at all.
	if !consentGranted() {
		return
	}
	// The ingest endpoint is authenticated, so an unauthenticated invocation
	// has nothing it could attribute an event to. A logged-out run is therefore
	// silent by construction rather than by policy — the documented collection
	// gap, not a dropped event.
	apiClient, err := newAPIClient()
	if err != nil {
		return
	}
	analytics.New(apiClient).Send(result, stderr)
}

// EmitLaunch reports one provider launch (`bex glm`, `bex code`, …).
//
// It must be called immediately before the exec that replaces this process, and
// the sender outlives that exec: delivery itself is a detached subprocess, not
// an inline round trip.
//
// Delivery is not free, though, and the cost lands on the launch. A consenting
// user pays a config read here, and — on the day a stored token is within its
// refresh window — upstream's client construction can spend up to five seconds
// refreshing it before the agent starts. That is the price of attributing the
// launch at all; an opted-out user pays none of it (see send).
//
// The event describes bex's own work: did the launcher resolve a provider,
// prepare an isolated config directory, and hand off. Whatever the launched
// agent does afterwards is the provider's session, not a bex command, and is
// never observed here.
func EmitLaunch(commandPath string, startedAt time.Time, stderr io.Writer) {
	emit(commandPath, command.CompletionKindSuccess, startedAt, 0, stderr)
}

// EmitVersion reports a root `--version` answer.
//
// Deliberate divergence from upstream, which treats version as a non-event
// (its fast path exits with no dependencies, so no sender exists). bex reports
// it because the launcher's release identity is the thing being measured
// (w5/m94) and `completion_kind = "version"` keeps these rows separable from
// real command usage on every panel.
func EmitVersion(commandPath string, startedAt time.Time, stderr io.Writer) {
	emit(commandPath, command.CompletionKindVersion, startedAt, 0, stderr)
}

func emit(commandPath string, kind command.CompletionKind, startedAt time.Time, exitCode int, stderr io.Writer) {
	send(command.ExecutionResult{
		// Eligible by construction: these helpers are only called from the two
		// paths that are known to escape the upstream hook, so there is no
		// second emitter to double-count with.
		AnalyticsEligible: true,
		// The one-time notice belongs to Cobra's post-run path, which by
		// definition did not run here; claiming eligibility would let a
		// process that is about to be replaced try to draw it.
		AnalyticsNoticeEligible: false,
		// Matched command names only — never arguments. Everything after the
		// provider name belongs to the launched agent and stays out of the
		// event entirely.
		CommandPath:    commandPath,
		CompletionKind: kind,
		Duration:       time.Since(startedAt),
		ExitCode:       exitCode,
		StartedAt:      startedAt,
	}, stderr)
}
