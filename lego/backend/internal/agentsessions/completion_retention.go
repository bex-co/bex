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

package agentsessions

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The Completer's half of retention (ADR047 D12, w22/m413; see retention.go).
// An interrupted turn settles as completed first, so a Stop shows within
// seconds, and the save runs behind it. The next turn waits for the save
// (Retention.claim) rather than racing it, and a sandbox whose save is in
// progress is not reaped.

// finalizeRetaining finalizes an interrupted turn as completed and, in the
// same write, begins the retention of its disk (retention.go): the session is
// steerable at the instant its saving row exists, so the next turn waits for
// the save instead of slipping in before it. The returned row is nil, and the
// session is finalized as for any turn, when retention is off, the sandbox is
// gone, the session was archived or another save is live; the sandbox then
// follows its ordinary teardown.
func (c *Completer) finalizeRetaining(ctx context.Context, record store.AgentSession, headSHA, note string) (store.AgentSession, store.TerminalTurnFact, *store.AgentRetainedImage, error) {
	r := c.Retention
	config, err := decodeAgentConfig(record)
	if err != nil {
		log.Printf("agent-session retention: read the session's config failed (session=%s): %v", record.ID, err)
		final, fact, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseCompleted, headSHA, note, "")
		return final, fact, nil, err
	}
	return r.Store.FinalizeAgentSessionRetaining(ctx, record.ID, headSHA, note, store.NewAgentRetainedImage{
		ID:           id.New(id.AgentRetainedImage),
		BaseImage:    r.Sandboxes.AgentTemplateImage(strings.TrimSpace(config.Template)),
		SaveDeadline: c.now().Add(retentionSaveWindow()),
	})
}

// runRetention runs a save off the Completer's loop, which a pause that takes
// a minute must not hold up; tests run it inline.
func (c *Completer) runRetention(fn func()) {
	if c.retainRunner != nil {
		c.retainRunner(fn)
		return
	}
	c.retainWG.Add(1)
	go func() {
		defer c.retainWG.Done()
		fn()
	}()
}

// saveRetention saves the turn's disk: it reads the transcript, which needs a
// running pod, then pauses the sandbox, copies its disk to the retained image
// and ends the sandbox, then records the image as kept. Whatever fails, the
// turn stays as it settled: the loss is recorded, the retention released and
// the user is not shown an error. All of it, the cleanup included, ends within
// retentionSaveWindow of the settle, before any Steer may start fresh.
func (c *Completer) saveRetention(ctx context.Context, record store.AgentSession, img store.AgentRetainedImage) {
	r := c.Retention
	// The records written once the save has ended get a context of their own,
	// started then: the save may have used all of its time.
	settle := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.WithoutCancel(ctx), retentionSettleTimeout)
	}
	lose := func(reason string, result retentionResult) {
		settleCtx, done := settle()
		defer done()
		c.Metrics.retention(result)
		r.release(settleCtx, c.now, img.ID, reason)
	}
	saveCtx, cancel := context.WithTimeout(ctx, retentionSaveTimeout)
	defer cancel()
	select {
	case r.saveSlots() <- struct{}{}:
		defer func() { <-r.saveSlots() }()
	case <-saveCtx.Done():
		log.Printf("agent-session retention: no save slot before the deadline (session=%s)", record.ID)
		lose(store.RetainedReleaseSaveFailed, retentionFailed)
		return
	}
	// The reply the agent streamed must reach the transcript before the pod
	// stops, and the next turn's continuity reads it from there.
	if !c.captureTranscript(saveCtx, record) {
		log.Printf("agent-session retention: the transcript could not be read; the disk is not kept (session=%s)", record.ID)
		lose(store.RetainedReleaseSaveFailed, retentionFailed)
		return
	}
	commit, err := r.Sandboxes.RetainAgentSessionSandbox(saveCtx, record.WorkspaceID, record.ID, record.SandboxID, img.ID)
	settleCtx, done := settle()
	defer done()
	if err != nil {
		reason, result := store.RetainedReleaseSaveFailed, retentionFailed
		if errors.Is(err, sandbox.ErrAgentImageStorageLimit) {
			reason, result = store.RetainedReleaseStorageFull, retentionStorageFull
		}
		log.Printf("agent-session retention: the disk was not kept (session=%s turn=%d reason=%s): %v", record.ID, record.Turns, reason, err)
		if errors.Is(err, sandbox.ErrAgentSandboxPaused) {
			// The paused sandbox was ended: it holds nothing to reap.
			c.sandboxEnded(settleCtx, record)
		}
		c.Metrics.retention(result)
		r.release(settleCtx, c.now, img.ID, reason)
		return
	}
	if commit.SandboxEnded {
		c.sandboxEnded(settleCtx, record)
	}
	until := c.now().Add(r.ttl())
	kept, err := r.Store.KeepAgentSessionRetainedImage(settleCtx, img.ID, commit.Digest, until)
	if err == nil && kept {
		c.Metrics.retention(retentionKept)
		log.Printf("agent-session retention: kept session=%s turn=%d bytes=%d until=%s", record.ID, record.Turns, commit.SizeBytes, until.Format(time.RFC3339))
		return
	}
	// The retention was released while the copy ran (a cancel, an archive, a
	// delete, a turn that started fresh), or its record could not be written:
	// the image just written is named by nothing, so it goes now.
	if err != nil {
		log.Printf("agent-session retention: record the kept image failed (session=%s): %v", record.ID, err)
		c.Metrics.retention(retentionFailed)
		r.release(settleCtx, c.now, img.ID, store.RetainedReleaseSaveFailed)
	} else {
		c.Metrics.retention(retentionSuperseded)
	}
	if err := r.Sandboxes.DeleteAgentRetainedImage(settleCtx, record.WorkspaceID, img.ID); err != nil {
		log.Printf("agent-session retention: delete the unrecorded image %s failed: %v", img.ID, err)
	}
}

// sandboxEnded settles the session once the save has ended its sandbox: the
// session's sandbox id is dropped, only while it is still that sandbox's, and
// its egress policy is deleted, as the ordinary teardown does, unless a newer
// turn has started, which is using the session's policy and has its own
// sandbox (CleanupAgentDispatches leaves a newer turn's the same way).
func (c *Completer) sandboxEnded(ctx context.Context, record store.AgentSession) {
	r := c.Retention
	if err := r.Store.ClearAgentSessionRetainedSandbox(ctx, record.ID, record.SandboxID); err != nil {
		log.Printf("agent-session retention: clear sandbox id failed (session=%s): %v", record.ID, err)
	}
	current, err := c.Store.GetAgentSession(ctx, record.ID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("agent-session retention: read the session before deleting its egress policy failed (session=%s): %v", record.ID, err)
		}
		return
	}
	if current.Turns != record.Turns {
		return
	}
	if err := r.Sandboxes.DeleteAgentSessionEgress(ctx, record.WorkspaceID, record.ID); err != nil {
		log.Printf("agent-session retention: delete the session's egress policy failed (session=%s): %v", record.ID, err)
	}
}

// reapRetainedImages applies the clock to retained images (expiry, a save
// whose Completer is gone, a claim whose dispatch died) and deletes the
// released ones from the registry.
func (c *Completer) reapRetainedImages(ctx context.Context) {
	r := c.Retention
	if !r.active() {
		return
	}
	now := c.now()
	if n, err := r.Store.ExpireAgentSessionRetainedImages(ctx, now); err != nil {
		log.Printf("agent-session retention: expire retained images failed: %v", err)
	} else if n > 0 {
		log.Printf("agent-session retention: released %d retained images", n)
	}
	r.purge(ctx, now, "")
}

// savingRetention reports which of the sessions have a save in progress: the
// idle reaper must not end a sandbox the save is pausing.
func (c *Completer) savingRetention(ctx context.Context, rows []store.AgentSession) map[string]bool {
	if !c.Retention.active() || len(rows) == 0 {
		return nil
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	saving, err := c.Retention.Store.AgentSessionsSavingRetainedImage(ctx, ids)
	if err != nil {
		// Leave every sandbox for the next tick rather than end one blind.
		log.Printf("agent-session retention: read the saves in progress failed: %v", err)
		all := make(map[string]bool, len(ids))
		for _, id := range ids {
			all[id] = true
		}
		return all
	}
	return saving
}
