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

package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Render's execution record vocabulary (`GET /sandboxes/{id}/execs`).
const (
	ExecutionTypeRuns  = "runs"
	ExecutionTypeFiles = "files"

	// ExecutionOperationSync is a tokenless buffered exec (MCP sandbox_exec);
	// a streamed run records ConnectOperationStream.
	ExecutionOperationSync = "sync"
)

// maxExecutionCommand bounds the command a record keeps; Render documents
// "the truncated command" without a length.
const maxExecutionCommand = 1024

// executionStopTimeout bounds the write that closes a record after its stream
// ended, whatever ended it.
const executionStopTimeout = 5 * time.Second

// executionRecordTimeout bounds the write that opens a record.
const executionRecordTimeout = 2 * time.Second

// ExecutionStore is where execution records live; *store.PGStore implements it.
type ExecutionStore interface {
	RecordSandboxExecution(ctx context.Context, e store.SandboxExecution) error
	StopSandboxExecution(ctx context.Context, workspaceID, executionID string, stoppedAt time.Time, exitCode *int) error
	StopOpenSandboxExecutions(ctx context.Context, workspaceID, sandboxID string, stoppedAt time.Time) error
	SandboxExecutions(ctx context.Context, workspaceID, sandboxID, after string, limit int) ([]store.SandboxExecution, error)
	SandboxExecution(ctx context.Context, workspaceID, sandboxID, executionID string) (store.SandboxExecution, error)
	PruneSandboxExecutions(ctx context.Context, workspaceID string, live []string) error
}

// Execution is Render's execution record.
type Execution struct {
	ID        string     `json:"id"`
	SandboxID string     `json:"sandboxId"`
	Type      string     `json:"type"`
	Operation string     `json:"operation"`
	Command   string     `json:"command,omitempty"`
	UserID    string     `json:"userId"`
	StartedAt time.Time  `json:"startedAt"`
	StoppedAt *time.Time `json:"stoppedAt,omitempty"`
	ExitCode  *int       `json:"exitCode,omitempty"`
}

// ExecutionEnvelope is one item of Render's execution list.
type ExecutionEnvelope struct {
	Execution Execution `json:"execution"`
	Cursor    string    `json:"cursor"`
}

func truncateCommand(command string) string {
	if len(command) <= maxExecutionCommand {
		return command
	}
	cut := maxExecutionCommand
	for cut > 0 && !utf8.RuneStart(command[cut]) {
		cut--
	}
	return command[:cut]
}

// recordExecution writes a started execution. History is informational: a
// store failure is logged and never refuses the command or transfer.
func (s *Service) recordExecution(ctx context.Context, e store.SandboxExecution) {
	if s.Executions == nil {
		return
	}
	e.Command = truncateCommand(e.Command)
	// Bounded: this write sits in front of a mint or a stream's first byte.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), executionRecordTimeout)
	defer cancel()
	if err := s.Executions.RecordSandboxExecution(ctx, e); err != nil {
		log.Printf("sandbox: record execution %s of %s: %v", e.ID, e.SandboxID, err)
	}
}

// stopExecution closes a record; exitCode is nil when none was reported.
func (s *Service) stopExecution(ctx context.Context, workspace, executionID string, exitCode *int) {
	if s.Executions == nil || executionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), executionStopTimeout)
	defer cancel()
	if err := s.Executions.StopSandboxExecution(ctx, workspace, executionID, s.Now(), exitCode); err != nil {
		log.Printf("sandbox: stop execution %s: %v", executionID, err)
	}
}

// executionsTarget authorizes a history read exactly as a sandbox read (Get's
// scope, relation and ownership), without Get's projection: a sandbox the
// caller cannot Get has no history they can see.
func (s *Service) executionsTarget(ctx context.Context, sandboxID string) (string, string, error) {
	ctx, err := s.scopeSandbox(ctx, "", sandboxID)
	if err != nil {
		return "", "", err
	}
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return "", "", err
	}
	if !s.enabled() || s.Executions == nil {
		return "", "", core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return "", "", err
	}
	ws := s.WorkspaceOrDefault(ctx)
	raw, err := s.ownedSandbox(ctx, key, ws, sandboxID)
	if err != nil {
		return "", "", err
	}
	return ws, canonicalID(raw), nil
}

// ListExecutions is Render's `GET /sandboxes/{id}/execs`: newest first,
// resuming after the execution id `after`, each item carrying its cursor.
func (s *Service) ListExecutions(ctx context.Context, sandboxID, after string, limit int) ([]ExecutionEnvelope, error) {
	views, err := s.listExecutionViews(ctx, sandboxID, after, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ExecutionEnvelope, len(views))
	for i, v := range views {
		out[i] = ExecutionEnvelope{Execution: v, Cursor: v.ID}
	}
	return out, nil
}

func (s *Service) listExecutionViews(ctx context.Context, sandboxID, after string, limit int) ([]Execution, error) {
	ws, box, err := s.executionsTarget(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Executions.SandboxExecutions(ctx, ws, box, after, limit)
	if err != nil {
		return nil, err
	}
	return s.executionViews(ctx, rows)
}

// GetExecution is Render's `GET /sandboxes/{id}/execs/{execId}`.
func (s *Service) GetExecution(ctx context.Context, sandboxID, executionID string) (Execution, error) {
	ws, box, err := s.executionsTarget(ctx, sandboxID)
	if err != nil {
		return Execution{}, err
	}
	row, err := s.Executions.SandboxExecution(ctx, ws, box, executionID)
	if errors.Is(err, store.ErrNotFound) {
		return Execution{}, core.NewNotFoundError("SANDBOX_EXECUTION_NOT_FOUND", "sandbox execution not found", map[string]any{"id": executionID})
	}
	if err != nil {
		return Execution{}, err
	}
	views, err := s.executionViews(ctx, []store.SandboxExecution{row})
	if err != nil {
		return Execution{}, err
	}
	return views[0], nil
}

// executionViews projects records, showing each subject as its Render usr-
// id where it has one.
func (s *Service) executionViews(ctx context.Context, rows []store.SandboxExecution) ([]Execution, error) {
	subjects := make([]string, 0, len(rows))
	for _, r := range rows {
		subjects = append(subjects, r.Subject)
	}
	slices.Sort(subjects)
	users, err := core.PublicUserIDs(ctx, s.UserIDs, slices.Compact(subjects))
	if err != nil {
		return nil, err
	}
	out := make([]Execution, len(rows))
	for i, r := range rows {
		user := r.Subject
		if id, ok := users[r.Subject]; ok {
			user = id
		}
		out[i] = Execution{
			ID: r.ID, SandboxID: r.SandboxID, Type: r.Type, Operation: r.Operation,
			Command: r.Command, UserID: user, StartedAt: r.StartedAt.UTC(),
			StoppedAt: r.StoppedAt, ExitCode: r.ExitCode,
		}
	}
	return out, nil
}

// exitSniffer passes an exec SSE stream through unchanged while reading the
// exit code from its `event: exit` frame. Only the current line is held, and
// at most maxSniffLine bytes of it.
type exitSniffer struct {
	line     []byte
	event    string
	exitCode *int
}

const maxSniffLine = 4096

func (x *exitSniffer) observe(p []byte) {
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		chunk := p
		if i >= 0 {
			chunk = p[:i]
		}
		if room := maxSniffLine - len(x.line); room > 0 {
			x.line = append(x.line, chunk[:min(len(chunk), room)]...)
		}
		if i < 0 {
			return
		}
		x.endLine()
		p = p[i+1:]
	}
}

var (
	sseEventPrefix = []byte("event: ")
	sseDataPrefix  = []byte("data: ")
)

func (x *exitSniffer) endLine() {
	line := bytes.TrimSuffix(x.line, []byte("\r"))
	defer func() { x.line = x.line[:0] }()
	switch {
	case bytes.HasPrefix(line, sseEventPrefix):
		x.event = string(line[len(sseEventPrefix):])
	case x.event == "exit" && bytes.HasPrefix(line, sseDataPrefix):
		if code := decodeExitCode(line[len(sseDataPrefix):]); code != nil {
			x.exitCode = code
		}
	}
}

// decodeExitCode reads an `exit` frame's code. The gateway emits the pinned
// Render CLI's `exit_code` (w7/m147) and, for one release, the pre-m147
// `exitCode` beside it; a reader may meet either while the gateway and
// bex-api roll out apart.
func decodeExitCode(payload []byte) *int {
	var ev struct {
		ExitCode       *int `json:"exit_code"`
		LegacyExitCode *int `json:"exitCode"`
	}
	if json.Unmarshal(payload, &ev) != nil {
		return nil
	}
	if ev.ExitCode != nil {
		return ev.ExitCode
	}
	return ev.LegacyExitCode
}
