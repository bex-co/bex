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

package clitelemetry

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeStore struct {
	mu     sync.Mutex
	rows   []store.CLITelemetryEvent
	retErr error
}

func (f *fakeStore) InsertCLITelemetryEvent(_ context.Context, ev store.CLITelemetryEvent) error {
	if f.retErr != nil {
		return f.retErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, ev)
	return nil
}

func newTestService() (*Service, *fakeStore) {
	st := &fakeStore{}
	return &Service{Base: &core.Base{}, Store: st}, st
}

func sampleInput() EventInput {
	tui := true
	return EventInput{
		Command:               "services list",
		DurationMs:            123,
		ExitCode:              0,
		CompletionKind:        "success",
		CLIVersion:            "2.27.0",
		OS:                    "darwin",
		Arch:                  "arm64",
		OutputFormat:          "json",
		InstallationID:        "install-1",
		LaunchedFullScreenTUI: &tui,
		IsStdoutTTY:           true,
		AgentSignals:          []string{"CLAUDECODE"},
		CISignals:             []string{"GITHUB_ACTIONS"},
		ReportedWorkspaceID:   "tea-client-claim",
		StartedAt:             "2026-09-10T04:00:00Z",
	}
}

func TestRecordPersistsAttributedRow(t *testing.T) {
	s, st := newTestService()
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-1"})

	eventID, err := s.Record(ctx, sampleInput())
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if !strings.HasPrefix(eventID, "cte-") || !id.WellFormed(eventID) {
		t.Errorf("event id = %q, want well-formed cte- id", eventID)
	}
	if len(st.rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(st.rows))
	}
	got := st.rows[0]
	if got.ID != eventID {
		t.Errorf("row id = %q, want %q", got.ID, eventID)
	}
	if got.Subject != "user-1" {
		t.Errorf("subject = %q, want reporter identity (never the client claim)", got.Subject)
	}
	if got.ReportedWorkspaceID != "tea-client-claim" {
		t.Errorf("reported workspace = %q, want the client claim kept verbatim", got.ReportedWorkspaceID)
	}
	if got.WorkspaceID == "tea-client-claim" {
		t.Errorf("workspace = client claim %q, want server-resolved attribution", got.WorkspaceID)
	}
	if got.Command != "services list" || got.DurationMs != 123 || got.ExitCode != 0 {
		t.Errorf("core fields = %+v", got)
	}
	if got.AgentSignals != "CLAUDECODE" || got.CISignals != "GITHUB_ACTIONS" {
		t.Errorf("signals = %q/%q", got.AgentSignals, got.CISignals)
	}
	if got.StartedAt == nil || got.StartedAt.UTC().Format(time.RFC3339) != "2026-09-10T04:00:00Z" {
		t.Errorf("started_at = %v", got.StartedAt)
	}
	if got.LaunchedFullScreenTUI == nil || !*got.LaunchedFullScreenTUI {
		t.Errorf("tui flag not preserved")
	}
	if got.ReceivedAt.IsZero() {
		t.Errorf("received_at not stamped")
	}
}

func TestRecordRequiresCommand(t *testing.T) {
	s, st := newTestService()
	in := sampleInput()
	in.Command = "   "
	if _, err := s.Record(context.Background(), in); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("empty command err = %v, want ErrBadRequest", err)
	}
	if len(st.rows) != 0 {
		t.Errorf("rows = %d, want 0", len(st.rows))
	}
}

func TestRecordStoreOff503s(t *testing.T) {
	s := &Service{Base: &core.Base{}}
	if _, err := s.Record(context.Background(), sampleInput()); !errors.Is(err, core.ErrCLITelemetryUnavailable) {
		t.Errorf("store-off err = %v, want ErrCLITelemetryUnavailable", err)
	}
}

func TestRecordTruncatesHostileFields(t *testing.T) {
	s, st := newTestService()
	in := sampleInput()
	in.Command = strings.Repeat("x", maxTelemetryCommandBytes+100)
	in.CLIVersion = strings.Repeat("y", maxTelemetryFieldBytes+10)
	in.AgentSignals = []string{"A", "B", "C"}
	// Pad past the cap with filler that must not survive.
	for len(in.CISignals) < maxTelemetrySignals+5 {
		in.CISignals = append(in.CISignals, "CI_FILLER")
	}
	if _, err := s.Record(context.Background(), in); err != nil {
		t.Fatalf("Record: %v", err)
	}
	got := st.rows[0]
	if len(got.Command) != maxTelemetryCommandBytes {
		t.Errorf("command len = %d, want truncated %d", len(got.Command), maxTelemetryCommandBytes)
	}
	if len(got.CLIVersion) != maxTelemetryFieldBytes {
		t.Errorf("cli_version len = %d, want truncated %d", len(got.CLIVersion), maxTelemetryFieldBytes)
	}
	if got.AgentSignals != "A,B,C" {
		t.Errorf("agent signals = %q", got.AgentSignals)
	}
	if strings.Count(got.CISignals, ",")+1 != maxTelemetrySignals {
		t.Errorf("ci signals kept = %d entries, want %d", strings.Count(got.CISignals, ",")+1, maxTelemetrySignals)
	}
}

func TestRecordBadTimestampDegradesToAbsent(t *testing.T) {
	s, st := newTestService()
	in := sampleInput()
	in.StartedAt = "not-a-time"
	if _, err := s.Record(context.Background(), in); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if st.rows[0].StartedAt != nil {
		t.Errorf("started_at = %v, want nil for unparseable input", st.rows[0].StartedAt)
	}
}
