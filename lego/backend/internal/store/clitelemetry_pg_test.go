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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestPGStoreCLITelemetryInsertAndPurge(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)

	now := time.Now().UTC()
	oldID := ids.New(ids.CLITelemetryEvent)
	freshID := ids.New(ids.CLITelemetryEvent)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM cli_telemetry_events WHERE id = ANY($1)`, []string{oldID, freshID})
		pool.Close()
	})

	tui := true
	started := now.Add(-time.Hour).Truncate(time.Second)
	insert := func(eventID string, received time.Time) {
		t.Helper()
		if err := st.InsertCLITelemetryEvent(ctx, CLITelemetryEvent{
			ID: eventID, Subject: "user-pg", WorkspaceID: "tea-pg",
			ReportedWorkspaceID: "tea-claim", Command: "services list",
			DurationMs: 7, ExitCode: 0, CompletionKind: "success",
			CLIVersion: "2.27.0", OS: "linux", Arch: "arm64",
			OutputFormat: "json", InstallationID: "install-pg",
			BexVersion:            "0.2.1",
			LaunchedFullScreenTUI: &tui,
			IsStdoutTTY:           true,
			AgentSignals:          "CLAUDECODE", CISignals: "GITHUB_ACTIONS",
			StartedAt: &started, ReceivedAt: received,
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	insert(oldID, now.AddDate(0, 0, -40))
	insert(freshID, now)

	var got CLITelemetryEvent
	if err := pool.QueryRow(ctx, `SELECT id, subject, workspace_id, reported_workspace_id, command,
		duration_ms, exit_code, completion_kind, cli_version, os, arch, output_format,
		installation_id, bex_version, launched_full_screen_tui, is_stdout_tty,
		agent_signals, ci_signals
		FROM cli_telemetry_events WHERE id = $1`, freshID).Scan(
		&got.ID, &got.Subject, &got.WorkspaceID, &got.ReportedWorkspaceID, &got.Command,
		&got.DurationMs, &got.ExitCode, &got.CompletionKind, &got.CLIVersion, &got.OS, &got.Arch,
		&got.OutputFormat, &got.InstallationID, &got.BexVersion,
		&got.LaunchedFullScreenTUI, &got.IsStdoutTTY,
		&got.AgentSignals, &got.CISignals); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.Subject != "user-pg" || got.WorkspaceID != "tea-pg" || got.Command != "services list" {
		t.Errorf("round trip = %+v", got)
	}
	// The two version axes are independent: upstream's pin and bex's own
	// release must both survive the write (w5/m94).
	if got.CLIVersion != "2.27.0" || got.BexVersion != "0.2.1" {
		t.Errorf("versions = upstream %q / bex %q, want 2.27.0 / 0.2.1", got.CLIVersion, got.BexVersion)
	}

	purged, err := st.PurgeCLITelemetryEvents(ctx, now.AddDate(0, 0, -30))
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if purged != 1 {
		t.Errorf("purged = %d, want 1 (only the 40-day-old row)", purged)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM cli_telemetry_events WHERE id = ANY($1)`, []string{oldID, freshID}).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Errorf("remaining = %d, want 1", remaining)
	}
}
