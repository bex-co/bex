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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// RecordStore keeps each sandbox's durable record (w3/m318), so an ended
// sandbox stays readable as on Render: GET answers it with status terminated
// and terminatedAt, list-sandboxes returns it under status=terminated, and a
// connect-token mint answers 409. *store.PGStore implements it; nil (store
// off) keeps none, and an ended sandbox is not found, as before.
type RecordStore interface {
	RecordSandbox(ctx context.Context, r store.SandboxRecord) error
	SetSandboxExpiry(ctx context.Context, workspaceID, sandboxID string, expiresAt *time.Time) error
	EndAbsentSandboxes(ctx context.Context, workspaceID string, live []string, at time.Time) error
	ExpireSandboxRecords(ctx context.Context, now time.Time) error
	EndedSandbox(ctx context.Context, workspaceID, sandboxID string) (store.SandboxRecord, error)
	EndedSandboxes(ctx context.Context, workspaceID string) ([]store.SandboxRecord, error)
	EndedSandboxWorkspaces(ctx context.Context, sandboxID string) ([]string, error)
}

// CodeSandboxNotRunning is the 409 a connect-token mint (and the exec and file
// transfer they authorize), suspend or resume answers for a terminated
// sandbox, and a mint for an errored one: Render's sandbox_not_running, which
// the CLI turns into "sandbox … was terminated at …" from a GET of the sandbox.
const CodeSandboxNotRunning = "sandbox_not_running"

// recordWriteTimeout bounds a record write, which never fails the verb.
const recordWriteTimeout = 5 * time.Second

func sandboxEndedError(box Sandbox) error {
	details := map[string]any{"id": box.ID, "status": string(box.Status)}
	message := fmt.Sprintf("sandbox %s errored", box.ID)
	if box.Status == StatusTerminated {
		message = fmt.Sprintf("sandbox %s was terminated", box.ID)
		if box.TerminatedAt != nil {
			at := box.TerminatedAt.UTC().Format(time.RFC3339)
			message += " at " + at
			details["terminatedAt"] = at
		}
	}
	return core.NewConflictError(CodeSandboxNotRunning, message, details)
}

// endedLive reports whether a live sandbox's reported state is one Render
// refuses a connect token for, terminated or errored. A state OpenSandbox has
// not reported yet is not a failure.
func endedLive(raw osSandbox) bool {
	status := mapOpenSandboxStatus(raw.Status.State)
	return raw.Status.State != "" && (status == StatusTerminated || status == StatusErrored)
}

// recordLive writes a created sandbox's record with its lifetime deadline,
// createdAt + timeoutSeconds (none for a sandbox without a timeout). Best
// effort, like recordEnded: a failed write only means the sandbox reads
// not-found once it ends.
func (s *Service) recordLive(ctx context.Context, ws string, box Sandbox) {
	var expiresAt *time.Time
	if box.CreatedAt != nil && box.TimeoutSeconds > 0 {
		at := box.CreatedAt.Add(time.Duration(box.TimeoutSeconds) * time.Second).UTC()
		expiresAt = &at
	}
	s.writeRecord(ctx, ws, box, nil, expiresAt)
}

// recordExpiry moves a live sandbox's recorded lifetime deadline, as a suspend
// or a resume moves its own (lifetimeDeadline), so the sweeper ends it at that
// time once OpenSandbox deletes it (w32/017). Best effort, like the other
// record writes: a sandbox whose deadline was not moved ends at the sweep.
func (s *Service) recordExpiry(ctx context.Context, raw osSandbox, expiresAt *time.Time) {
	if s.Records == nil {
		return
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordWriteTimeout)
	defer cancel()
	if err := s.Records.SetSandboxExpiry(writeCtx, raw.Metadata[metadataWorkspace], canonicalID(raw), expiresAt); err != nil {
		log.Printf("sandbox: record expiry of %s: %v", canonicalID(raw), err)
	}
}

// recordEnded records a sandbox's end now. raw is the sandbox as last read;
// its view is kept only for a sandbox created before records existed.
func (s *Service) recordEnded(ctx context.Context, ws string, raw osSandbox) {
	at := s.Now().UTC()
	s.writeRecord(ctx, ws, s.sandboxView(raw, ws), &at, nil)
}

func (s *Service) writeRecord(ctx context.Context, ws string, box Sandbox, endedAt, expiresAt *time.Time) {
	if s.Records == nil || box.ID == "" {
		return
	}
	view, err := json.Marshal(box)
	if err != nil {
		log.Printf("sandbox: encode record of %s: %v", box.ID, err)
		return
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordWriteTimeout)
	defer cancel()
	r := store.SandboxRecord{WorkspaceID: ws, SandboxID: box.ID, Owner: box.Owner, View: view, TerminatedAt: endedAt, ExpiresAt: expiresAt}
	if err := s.Records.RecordSandbox(writeCtx, r); err != nil {
		log.Printf("sandbox: write record of %s: %v", box.ID, err)
	}
}

// endedSandbox is ws's ended sandbox id as Render reads it. A sandbox with no
// ended record, or another member's that the caller may not see (the same
// owner-or-admin boundary as a live one), is not found.
func (s *Service) endedSandbox(ctx context.Context, ws, id string) (Sandbox, error) {
	if s.Records == nil {
		return Sandbox{}, sandboxNotFound(id)
	}
	r, err := s.Records.EndedSandbox(ctx, ws, id)
	if errors.Is(err, store.ErrNotFound) {
		return Sandbox{}, sandboxNotFound(id)
	}
	if err != nil {
		return Sandbox{}, err
	}
	allowed, err := s.ownerCheck(ctx, ws)(r.Owner)
	if err != nil {
		return Sandbox{}, err
	}
	if !allowed {
		return Sandbox{}, sandboxNotFound(id)
	}
	return s.endedView(r)
}

// orEnded answers a by-id read whose live lookup failed: ws's ended sandbox
// when the live sandbox was not found and an ended record exists, else err.
func (s *Service) orEnded(ctx context.Context, ws, id string, err error) (Sandbox, error) {
	if !errors.Is(err, core.ErrNotFound) {
		return Sandbox{}, err
	}
	box, endedErr := s.endedSandbox(ctx, ws, id)
	if errors.Is(endedErr, core.ErrNotFound) {
		return Sandbox{}, err
	}
	return box, endedErr
}

// endedConflict is a connect-token mint's answer when its live lookup failed:
// Render's 409 for an ended sandbox, else err.
func (s *Service) endedConflict(ctx context.Context, ws, id string, err error) error {
	box, endedErr := s.orEnded(ctx, ws, id, err)
	if endedErr != nil {
		return endedErr
	}
	return sandboxEndedError(box)
}

// endedSandboxes are ws's ended sandboxes the caller may see, less any id the
// live list still holds.
func (s *Service) endedSandboxes(ctx context.Context, ws string, live []Sandbox) ([]Sandbox, error) {
	if s.Records == nil {
		return nil, nil
	}
	records, err := s.Records.EndedSandboxes(ctx, ws)
	if err != nil {
		return nil, err
	}
	listed := make(map[string]bool, len(live))
	for _, box := range live {
		listed[box.ID] = true
	}
	visible := s.ownerCheck(ctx, ws)
	var out []Sandbox
	for _, r := range records {
		if listed[r.SandboxID] {
			continue
		}
		if ok, err := visible(r.Owner); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		box, err := s.endedView(r)
		if err != nil {
			return nil, err
		}
		out = append(out, box)
	}
	return out, nil
}

// endedView is an ended record as Render reads it. Its region is the one a
// live sandbox reads, not the one saved at create, so it agrees with its group.
func (s *Service) endedView(r store.SandboxRecord) (Sandbox, error) {
	var box Sandbox
	if err := json.Unmarshal(r.View, &box); err != nil {
		return Sandbox{}, fmt.Errorf("read sandbox record %s: %w", r.SandboxID, err)
	}
	box.Status, box.TerminatedAt, box.Region = StatusTerminated, r.TerminatedAt, s.region()
	return box, nil
}
