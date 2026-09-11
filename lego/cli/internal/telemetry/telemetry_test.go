package telemetry

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
)

// captureSend swaps the upstream sender for the duration of a test.
func captureSend(t *testing.T) *[]command.ExecutionResult {
	t.Helper()
	original := send
	sent := []command.ExecutionResult{}
	send = func(result command.ExecutionResult, _ io.Writer) {
		sent = append(sent, result)
	}
	t.Cleanup(func() { send = original })
	return &sent
}

func TestEmitLaunchDescribesTheLauncherNotTheAgent(t *testing.T) {
	sent := captureSend(t)
	startedAt := time.Now().Add(-250 * time.Millisecond)

	EmitLaunch("bex glm", startedAt, io.Discard)

	if len(*sent) != 1 {
		t.Fatalf("sent %d events, want exactly one", len(*sent))
	}
	got := (*sent)[0]
	if got.CommandPath != "bex glm" {
		t.Errorf("CommandPath = %q, want the matched command path", got.CommandPath)
	}
	if got.CompletionKind != command.CompletionKindSuccess {
		t.Errorf("CompletionKind = %q, want success (the launcher handed off)", got.CompletionKind)
	}
	if got.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", got.ExitCode)
	}
	if !got.AnalyticsEligible {
		t.Error("AnalyticsEligible = false; these paths exist because nothing else reports them")
	}
	// The process is about to be replaced, so it must not claim the right to
	// draw the one-time notice.
	if got.AnalyticsNoticeEligible {
		t.Error("AnalyticsNoticeEligible = true, want false before an exec")
	}
	if got.Duration <= 0 {
		t.Errorf("Duration = %v, want the elapsed launcher time", got.Duration)
	}
	if !got.StartedAt.Equal(startedAt) {
		t.Errorf("StartedAt = %v, want the caller's start instant %v", got.StartedAt, startedAt)
	}
}

func TestEmitVersionUsesTheVersionCompletionKind(t *testing.T) {
	sent := captureSend(t)

	EmitVersion("bex", time.Now(), io.Discard)

	if len(*sent) != 1 {
		t.Fatalf("sent %d events, want exactly one", len(*sent))
	}
	// A separable kind is what keeps version probes out of real command usage
	// on every panel.
	if got := (*sent)[0].CompletionKind; got != command.CompletionKindVersion {
		t.Errorf("CompletionKind = %q, want version", got)
	}
}

// TestEmitCarriesNoArguments is the privacy floor: everything after the
// provider name belongs to the launched agent (prompts, paths, flags) and must
// never reach an event.
func TestEmitCarriesNoArguments(t *testing.T) {
	sent := captureSend(t)

	EmitLaunch("bex glm", time.Now(), io.Discard)

	got := (*sent)[0]
	for _, forbidden := range []string{"--", "/", "prompt", "secret"} {
		if strings.Contains(got.CommandPath, forbidden) {
			t.Errorf("CommandPath %q contains %q; only matched command names may be emitted", got.CommandPath, forbidden)
		}
	}
	// ExecutionResult has no free-form field the launcher could fill, so the
	// command path is the whole attack surface — assert it is exactly the two
	// command words.
	if fields := strings.Fields(got.CommandPath); len(fields) != 2 {
		t.Errorf("CommandPath = %q, want exactly the binary and command names", got.CommandPath)
	}
}

// TestSendChecksConsentBeforeBuildingAClient guards a real cost, not a style
// preference. Upstream's client constructor reads the CLI config and, when a
// stored token is near expiry, performs a synchronous OAuth refresh with a
// five-second timeout. Both callers of send run where that is unaffordable —
// immediately before the process is replaced by the launched agent, and on
// `--version` — so an opted-out user must not reach it at all.
func TestSendChecksConsentBeforeBuildingAClient(t *testing.T) {
	originalConsent, originalClient := consentGranted, newAPIClient
	t.Cleanup(func() { consentGranted, newAPIClient = originalConsent, originalClient })

	built := 0
	newAPIClient = func() (*client.ClientWithResponses, error) {
		built++
		return nil, errors.New("client construction must not be reached")
	}

	consentGranted = func() bool { return false }
	send(command.ExecutionResult{CommandPath: "bex glm"}, io.Discard)
	if built != 0 {
		t.Fatalf("built %d clients while opted out, want 0", built)
	}

	consentGranted = func() bool { return true }
	send(command.ExecutionResult{CommandPath: "bex glm"}, io.Discard)
	if built != 1 {
		t.Fatalf("built %d clients while opted in, want 1", built)
	}
}
