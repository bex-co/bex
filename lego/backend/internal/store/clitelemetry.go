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

package store

import (
	"context"
	"time"
)

// CLITelemetryEvent is one ingested `POST /v1/cli-telemetry-events` delivery
// (w5/m92). Subject/WorkspaceID are the reporter's auth attribution, resolved
// server-side and never taken from the client's claim; ReportedWorkspaceID
// keeps the CLI's own current_workspace_id for join/debug. The remaining
// fields mirror Render's CliTelemetryEventPOSTInput JSON shape one-for-one so
// the imported CLI's sender needs no translation. BexVersion is the exception:
// it is bex's own launcher release, carried by a header rather than the body
// precisely so that shape stays diffable against upstream (w5/m94), and is
// empty for an unmodified upstream CLI or any pre-m94 build.
type CLITelemetryEvent struct {
	ID                    string
	Subject               string
	WorkspaceID           string
	ReportedWorkspaceID   string
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
	AgentSignals          string
	CISignals             string
	StartedAt             *time.Time
	ReceivedAt            time.Time
}

func (s *PGStore) InsertCLITelemetryEvent(ctx context.Context, ev CLITelemetryEvent) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO cli_telemetry_events
			(id, subject, workspace_id, reported_workspace_id, command,
			 duration_ms, exit_code, completion_kind, cli_version, os, arch,
			 output_format, installation_id, bex_version,
			 launched_full_screen_tui,
			 is_stdin_tty, is_stdout_tty, is_stderr_tty, is_term_dumb,
			 agent_signals, ci_signals, started_at, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20, $21, $22, $23)`,
		ev.ID, ev.Subject, ev.WorkspaceID, ev.ReportedWorkspaceID, ev.Command,
		ev.DurationMs, ev.ExitCode, ev.CompletionKind, ev.CLIVersion, ev.OS, ev.Arch,
		ev.OutputFormat, ev.InstallationID, ev.BexVersion, ev.LaunchedFullScreenTUI,
		ev.IsStdinTTY, ev.IsStdoutTTY, ev.IsStderrTTY, ev.IsTermDumb,
		ev.AgentSignals, ev.CISignals, ev.StartedAt, ev.ReceivedAt)
	return err
}

// PurgeCLITelemetryEvents applies the platform retention window to ingested
// CLI telemetry: stable installation UUIDs must not become an indefinitely
// retained trail. It runs in the audit service's scheduled sweep beside
// PurgeAuditEvents/PurgeSSHSessions (w5/m92).
func (s *PGStore) PurgeCLITelemetryEvents(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM cli_telemetry_events WHERE received_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
