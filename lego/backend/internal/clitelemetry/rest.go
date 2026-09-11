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
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// telemetryEventRequest is POST /v1/cli-telemetry-events' body. JSON names
// mirror Render's CliTelemetryEventPOSTInput one-for-one (upstream
// pkg/client/clitelemetry) — the imported CLI's sender needs no translation,
// and a field-by-field diff against that struct is the t006 parity check.
// Render's embedded public spec does not list this path, so the OpenAPI gate
// passes it through WITHOUT strict decoding: unknown future fields are
// ignored by plain DecodeJSON, and the unknown-field test below pins that
// forward-compat against a future spec addition.
type telemetryEventRequest struct {
	AgentSignals          []string `json:"agent_signals"`
	Arch                  string   `json:"arch"`
	CISignals             []string `json:"ci_signals"`
	CLIVersion            string   `json:"cli_version"`
	Command               string   `json:"command"`
	CompletionKind        string   `json:"completion_kind"`
	CurrentWorkspaceID    string   `json:"current_workspace_id"`
	DurationMs            int64    `json:"duration_ms"`
	ExitCode              int      `json:"exit_code"`
	InstallationID        string   `json:"installation_id"`
	LaunchedFullScreenTUI *bool    `json:"launched_full_screen_tui"`
	OS                    string   `json:"os"`
	OutputFormat          string   `json:"output_format"`
	StartedAt             string   `json:"started_at"`
	IsStdinTTY            bool     `json:"is_stdin_tty"`
	IsStdoutTTY           bool     `json:"is_stdout_tty"`
	IsStderrTTY           bool     `json:"is_stderr_tty"`
	IsTermDumb            bool     `json:"is_term_dumb"`
}

// RegisterREST mounts the telemetry ingest surface: one POST. The upstream
// sender fires and forgets (best-effort, detached subprocess), so the verb
// answers 202 on store and maps every failure to the shared error dialect —
// a rejection never breaks the CLI, it just stops that one event.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/cli-telemetry-events", func(w http.ResponseWriter, r *http.Request) {
		var req telemetryEventRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, core.ErrBadRequest)
			return
		}
		eventID, err := s.Record(r.Context(), EventInput{
			Command:               req.Command,
			DurationMs:            req.DurationMs,
			ExitCode:              req.ExitCode,
			CompletionKind:        req.CompletionKind,
			CLIVersion:            req.CLIVersion,
			OS:                    req.OS,
			Arch:                  req.Arch,
			OutputFormat:          req.OutputFormat,
			InstallationID:        req.InstallationID,
			BexVersion:            r.Header.Get(BexVersionHeader),
			LaunchedFullScreenTUI: req.LaunchedFullScreenTUI,
			IsStdinTTY:            req.IsStdinTTY,
			IsStdoutTTY:           req.IsStdoutTTY,
			IsStderrTTY:           req.IsStderrTTY,
			IsTermDumb:            req.IsTermDumb,
			AgentSignals:          req.AgentSignals,
			CISignals:             req.CISignals,
			ReportedWorkspaceID:   req.CurrentWorkspaceID,
			StartedAt:             req.StartedAt,
		})
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusAccepted, map[string]string{"id": eventID})
	})
}
