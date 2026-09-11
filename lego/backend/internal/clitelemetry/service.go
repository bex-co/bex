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

// Package clitelemetry is the CLI-telemetry collection side (w5/m92): it
// ingests the `POST /v1/cli-telemetry-events` deliveries the imported Render
// CLI's own analytics sender already emits, attributing each to the
// reporter's auth identity. The transport needs no launcher work — the
// bridge repoints RENDER_HOST at bex-api, so the upstream sender already
// targets us; consent is the launcher's separate flip (t004).
//
// Attribution is server-side by construction: Subject/WorkspaceID come from
// the auth identity and Base tenant resolution, never from the client's
// claim. The payload's own current_workspace_id is kept verbatim as
// ReportedWorkspaceID for join/debug only.
package clitelemetry

import (
	"context"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TelemetryStore is the Service's seam to the control-plane store.
// *store.PGStore satisfies it; a fake backs the tests. nil => the
// control-plane store is off (BEX_CP_DB_URI unset) and verbs 503.
type TelemetryStore interface {
	InsertCLITelemetryEvent(ctx context.Context, ev store.CLITelemetryEvent) error
}

// TelemetryMetrics is the Service's seam to the origin metrics registry.
// *api.OriginMetrics satisfies it (declared here so the dependency points
// api -> clitelemetry, never back). nil => metrics off, silently skipped.
type TelemetryMetrics interface {
	ObserveTelemetry(command, completionKind, cliVersion, outputFormat string)
}

// Service ingests CLI telemetry events over the injected control-plane
// store. Always non-nil; its verbs 503 until the store is wired.
type Service struct {
	*core.Base
	Store   TelemetryStore
	Metrics TelemetryMetrics
}

// Field bounds keep one hostile delivery from bloating the table: the
// upstream sender emits short command paths and small signal lists, so
// anything larger is truncated, never rejected (ingest stays liberal —
// telemetry must not fail the CLI, and the server answers 202 either way).
const (
	maxTelemetryCommandBytes = 512
	maxTelemetryFieldBytes   = 256
	maxTelemetrySignals      = 32
	// A release tag is short by construction (`0.2.1`, `1.2.3-rc.1+build.4`);
	// anything longer is not one of ours, so the value is dropped rather than
	// truncated into a plausible-looking release that never existed.
	maxTelemetryVersionBytes = 64
)

// BexVersionHeader carries the bex launcher's own release identity out of band.
// It is a header rather than a body field because the body is Render's
// CliTelemetryEventPOSTInput and must stay diffable against it field-for-field;
// and it cannot ride User-Agent, which upstream owns that string and it must keep naming the
// pinned render-oss/cli release the compatibility ledger tracks (w5/m94). The
// launcher sets it in lego/cli/internal/bridge. The two constants are
// deliberately not shared — `cli` imports no sibling module by design — so the
// compiler cannot catch a rename. Each side instead pins the literal in its own
// test (TestBexVersionHeaderNameIsPinned here, and its twin in lego/cli), which
// is what makes a one-sided rename fail loudly instead of silently dropping the
// release axis.
const BexVersionHeader = "X-Bex-CLI-Version"

// EventInput is the validated ingest shape. JSON names mirror Render's
// CliTelemetryEventPOSTInput one-for-one (see rest.go); unknown future
// fields never reach this struct — plain DecodeJSON ignores them, which the
// unknown-field test pins against a future strict-decoding regression.
type EventInput struct {
	Command               string
	DurationMs            int64
	ExitCode              int
	CompletionKind        string
	CLIVersion            string
	OS                    string
	Arch                  string
	OutputFormat          string
	InstallationID        string
	BexVersion            string
	LaunchedFullScreenTUI *bool
	IsStdinTTY            bool
	IsStdoutTTY           bool
	IsStderrTTY           bool
	IsTermDumb            bool
	AgentSignals          []string
	CISignals             []string
	ReportedWorkspaceID   string
	StartedAt             string
}

// Record validates in, attributes it to the caller, and stores it,
// returning the minted event id. Membership-gated on the acting workspace
// (RelCanView is read-classified: allowed calls emit no audit row, so
// per-invocation ingest doesn't double the audit write rate; denials still
// record). The REST scope-class gate outside already enforced the write
// capability before this runs.
func (s *Service) Record(ctx context.Context, in EventInput) (string, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return "", err
	}
	if s.Store == nil {
		return "", core.ErrCLITelemetryUnavailable
	}
	command := strings.TrimSpace(in.Command)
	if command == "" {
		return "", core.ErrBadRequest
	}
	var startedAt *time.Time
	if t, err := time.Parse(time.RFC3339, in.StartedAt); err == nil {
		startedAt = &t
	}
	// An unparseable timestamp degrades to absent rather than failing the
	// ingest: the sender always emits RFC3339, so anything else is a foreign
	// client we still want to count.
	subject := ""
	if ident, ok := core.IdentityFrom(ctx); ok {
		subject = ident.Subject
	}
	eventID := id.New(id.CLITelemetryEvent)
	ev := store.CLITelemetryEvent{
		ID:                    eventID,
		Subject:               subject,
		WorkspaceID:           s.WorkspaceOrDefault(ctx),
		ReportedWorkspaceID:   truncate(in.ReportedWorkspaceID, maxTelemetryFieldBytes),
		Command:               truncate(command, maxTelemetryCommandBytes),
		DurationMs:            in.DurationMs,
		ExitCode:              in.ExitCode,
		CompletionKind:        truncate(in.CompletionKind, maxTelemetryFieldBytes),
		CLIVersion:            truncate(in.CLIVersion, maxTelemetryFieldBytes),
		OS:                    truncate(in.OS, maxTelemetryFieldBytes),
		Arch:                  truncate(in.Arch, maxTelemetryFieldBytes),
		OutputFormat:          truncate(in.OutputFormat, maxTelemetryFieldBytes),
		InstallationID:        truncate(in.InstallationID, maxTelemetryFieldBytes),
		BexVersion:            sanitizeVersion(in.BexVersion),
		LaunchedFullScreenTUI: in.LaunchedFullScreenTUI,
		IsStdinTTY:            in.IsStdinTTY,
		IsStdoutTTY:           in.IsStdoutTTY,
		IsStderrTTY:           in.IsStderrTTY,
		IsTermDumb:            in.IsTermDumb,
		AgentSignals:          joinSignals(in.AgentSignals),
		CISignals:             joinSignals(in.CISignals),
		StartedAt:             startedAt,
		ReceivedAt:            time.Now().UTC(),
	}
	if err := s.Store.InsertCLITelemetryEvent(ctx, ev); err != nil {
		return "", err
	}
	// Count only stored events: rejections stay on the origin request
	// counter's status axis instead of double-appearing here.
	if s.Metrics != nil {
		s.Metrics.ObserveTelemetry(ev.Command, ev.CompletionKind, ev.CLIVersion, ev.OutputFormat)
	}
	return eventID, nil
}

// sanitizeVersion accepts only a release-tag shape and otherwise stores
// nothing: rejecting the whole value, rather than stripping bad characters out
// of it, keeps a mangled input from masquerading as a real release on a
// low-cardinality dashboard axis.
//
// It deliberately guards only BexVersion, not the CLIVersion beside it. That
// asymmetry is the conservative choice, not an oversight: every body field is
// truncated-never-rejected by w5/m92's explicit "ingest stays liberal" rule, and
// tightening a shipped field would silently drop data that is stored today.
// BexVersion is new and header-borne, so it can start strict. If the upstream
// axis ever needs the same treatment, that is a change to m92's rule and needs
// its own decision.
func sanitizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > maxTelemetryVersionBytes {
		return ""
	}
	for _, r := range v {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '.', r == '-', r == '+', r == '_':
		default:
			return ""
		}
	}
	return v
}

func truncate(v string, max int) string {
	if len(v) <= max {
		return v
	}
	return v[:max]
}

// joinSignals folds the sender's allow-listed env-name lists into one
// comma-joined column value, capped so a foreign client cannot bloat the row.
// Names only — the sender never transmits values.
func joinSignals(signals []string) string {
	if len(signals) > maxTelemetrySignals {
		signals = signals[:maxTelemetrySignals]
	}
	trimmed := make([]string, 0, len(signals))
	for _, name := range signals {
		if name = truncate(strings.TrimSpace(name), maxTelemetryFieldBytes); name != "" {
			trimmed = append(trimmed, name)
		}
	}
	return strings.Join(trimmed, ",")
}
