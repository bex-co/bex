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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The Completer's half of retention (ADR047 D12, w22/m413, w10/m491; see
// retention.go). An interrupted turn settles as completed first, so a Stop
// shows within seconds, and the save runs behind it. A turn that ended any
// other way keeps its disk when its sandbox is about to be ended and its
// checkout holds undelivered work: a failed turn's at its settle, a completed
// turn's at the idle close, and either's when the next message replaces the
// sandbox (Service.runSteerDispatch). The next turn waits for the save
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
	in, ok := c.newRetainedImage(record)
	if !ok {
		final, fact, err := c.Store.FinalizeAgentSession(ctx, record.ID, PhaseCompleted, headSHA, note, "")
		return final, fact, nil, err
	}
	return c.Retention.Store.FinalizeAgentSessionRetaining(ctx, record.ID, headSHA, note, in)
}

// newRetainedImage is the retention to begin for the session's sandbox: a new
// handle, the template image the disk runs on and the save's deadline. false
// when the session's config cannot be read, which keeps nothing.
func (c *Completer) newRetainedImage(record store.AgentSession) (store.NewAgentRetainedImage, bool) {
	config, err := decodeAgentConfig(record)
	if err != nil {
		log.Printf("agent-session retention: read the session's config failed (session=%s): %v", record.ID, err)
		return store.NewAgentRetainedImage{}, false
	}
	return store.NewAgentRetainedImage{
		ID:           id.New(id.AgentRetainedImage),
		BaseImage:    c.Retention.Sandboxes.AgentTemplateImage(strings.TrimSpace(config.Template)),
		SaveDeadline: c.now().Add(retentionSaveWindow()),
	}, true
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
// and ends the sandbox, then records the image as kept. record is the session
// as of the turn whose disk it is (its Turns and SandboxID). Whatever fails,
// the turn stays as it settled: the loss is recorded, the retention released
// and the user is not shown an error. A save made because the sandbox was
// being closed (closing) ends the sandbox when it fails, as the close would
// have; a stopped turn's leaves it to the idle grace. All of it, the cleanup
// included, ends within retentionSaveWindow of its start, before any Steer may
// start fresh.
func (c *Completer) saveRetention(ctx context.Context, record store.AgentSession, img store.AgentRetainedImage, closing bool) {
	r := c.Retention
	// The records written once the save has ended get a context of their own,
	// started then: the save may have used all of its time.
	settle := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.WithoutCancel(ctx), retentionSettleTimeout)
	}
	// lose records the loss; the sandbox is settled as ended when the save
	// ended it, else ended when the save was a close's.
	lose := func(reason string, result retentionResult, ended bool) {
		settleCtx, done := settle()
		defer done()
		c.Metrics.retention(result)
		r.release(settleCtx, c.now, img.ID, reason)
		switch {
		case ended:
			c.sandboxEnded(settleCtx, record)
		case closing:
			c.terminate(settleCtx, record)
		}
	}
	saveCtx, cancel := context.WithTimeout(ctx, retentionSaveTimeout)
	defer cancel()
	select {
	case r.saveSlots() <- struct{}{}:
		defer func() { <-r.saveSlots() }()
	case <-saveCtx.Done():
		log.Printf("agent-session retention: no save slot before the deadline (session=%s)", record.ID)
		lose(store.RetainedReleaseSaveFailed, retentionFailed, false)
		return
	}
	// The reply the agent streamed must reach the transcript before the pod
	// stops, and the next turn's continuity reads it from there.
	if !c.captureTranscript(saveCtx, record) {
		log.Printf("agent-session retention: the transcript could not be read; the disk is not kept (session=%s)", record.ID)
		lose(store.RetainedReleaseSaveFailed, retentionFailed, false)
		return
	}
	commit, err := r.Sandboxes.RetainAgentSessionSandbox(saveCtx, record.WorkspaceID, record.ID, record.SandboxID, img.ID)
	if err != nil {
		reason, result := store.RetainedReleaseSaveFailed, retentionFailed
		if errors.Is(err, sandbox.ErrAgentImageStorageLimit) {
			reason, result = store.RetainedReleaseStorageFull, retentionStorageFull
		}
		log.Printf("agent-session retention: the disk was not kept (session=%s turn=%d reason=%s): %v", record.ID, record.Turns, reason, err)
		// A sandbox the pause reached was ended: it holds nothing to reap.
		lose(reason, result, errors.Is(err, sandbox.ErrAgentSandboxPaused))
		return
	}
	settleCtx, done := settle()
	defer done()
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

// retainOnClose is the close of a settled turn's sandbox, at the point
// teardown would end it (a failed turn's at its settle, a completed turn's
// once the idle grace has passed, after the editor pin either way): when its
// checkout holds undelivered work, its disk is saved for the next message
// instead of being discarded (ADR047 D12, w10/m491). It reports whether the
// sandbox is left to a save, this one or another already running; false
// leaves it to be terminated as before.
func (c *Completer) retainOnClose(ctx context.Context, record store.AgentSession) bool {
	if !c.Retention.retains() || record.ArchivedAt != nil || record.Turns < 1 {
		return false
	}
	var cause string
	switch record.Phase {
	case PhaseFailed:
		cause = store.TurnRetentionCauseFailed
	case PhaseCompleted:
		cause = store.TurnRetentionCauseIdleClosed
	default:
		return false
	}
	checkout, _, ok := c.undelivered(ctx, record)
	if !ok || !checkout.Undelivered() {
		return false
	}
	if img := c.beginRetention(ctx, record, cause, checkout); img != nil {
		c.runRetention(func() { c.saveRetention(ctx, record, *img, true) })
		return true
	}
	// Nothing was begun for work that is there. A message may have begun the
	// next turn, whose dispatch now owns this sandbox (it saves it or tears it
	// down), or another save may be running for it, which ends it.
	if current, err := c.Store.GetAgentSession(ctx, record.ID); err == nil && current.Turns != record.Turns {
		return true
	}
	return c.savingRetention(ctx, []store.AgentSession{record})[record.ID]
}

// retainReplaced keeps the disk of the turn a message is about to replace
// (Service.runSteerDispatch, before it tears the previous sandbox down): prev
// is the session as of that turn, its Turns and its SandboxID. When the
// checkout holds undelivered work the disk is saved, inline, so the dispatch
// that follows claims it; with an editor open on the sandbox nothing is kept
// and the loss is recorded, as for a Stop (a close never meets an open
// editor: teardown waits for it). A save already running for the sandbox
// (the Completer's close of it) is the dispatch's to wait for.
func (c *Completer) retainReplaced(ctx context.Context, prev store.AgentSession) {
	if !c.Retention.retains() || prev.SandboxID == "" || prev.Turns < 1 {
		return
	}
	checkout, editorOpen, ok := c.undelivered(ctx, prev)
	switch {
	case !ok || !checkout.Undelivered():
		return
	case editorOpen:
		if err := c.Retention.Store.LoseAgentSessionTurnRetention(ctx, prev.ID, prev.Turns, store.TurnRetentionCauseReplaced, store.RetainedNotSavedEditorOpen, &checkout); err != nil {
			log.Printf("agent-session retention: record the loss failed (session=%s turn=%d): %v", prev.ID, prev.Turns, err)
		}
		return
	}
	if img := c.beginRetention(ctx, prev, store.TurnRetentionCauseReplaced, checkout); img != nil {
		c.saveRetention(ctx, prev, *img, false)
	}
}

// beginRetention begins keeping the disk of the turn record is on
// (record.Turns), whose checkout holds the undelivered work given, with
// cause: the saving row, which is the claim on pausing the sandbox. nil when
// the session moved on, a save already exists or the write failed.
func (c *Completer) beginRetention(ctx context.Context, record store.AgentSession, cause string, checkout store.TurnCheckout) *store.AgentRetainedImage {
	in, ok := c.newRetainedImage(record)
	if !ok {
		return nil
	}
	img, err := c.Retention.Store.BeginAgentSessionRetention(ctx, record.ID, record.Turns, record.SandboxID, cause, checkout, in)
	if err != nil {
		log.Printf("agent-session retention: begin the save failed (session=%s turn=%d cause=%s): %v", record.ID, record.Turns, cause, err)
		return nil
	}
	if img != nil {
		log.Printf("agent-session retention: keeping the disk (session=%s turn=%d cause=%s dirty=%t unpushed=%d)", record.ID, record.Turns, cause, checkout.Dirty, checkout.UnpushedCommits)
	}
	return img
}

// undelivered reads what the turn record is on left undelivered in its
// sandbox's checkout, and whether an editor is open on the sandbox now. The
// facts the turn recorded when it ended are believed unless they are unknown,
// an editor has connected since, which may have changed the checkout, or the
// sandbox is one resume started for an editor, whose checkout is not the
// turn's (ADR054 D8): then the checkout is read in the sandbox. ok is false
// when it cannot be told.
func (c *Completer) undelivered(ctx context.Context, record store.AgentSession) (checkout store.TurnCheckout, editorOpen, ok bool) {
	turn, found, err := c.currentTurn(ctx, record)
	if err != nil || !found {
		return store.TurnCheckout{}, false, false
	}
	open, lastEnded, err := c.Store.AgentSessionSSHActivity(ctx, record.ID, c.now().Add(-c.sshGraceTTL()))
	if err != nil {
		log.Printf("agent-session retention: ssh-activity check failed (session=%s): %v", record.ID, err)
		return store.TurnCheckout{}, false, false
	}
	recorded, known := turn.Checkout()
	edited := open || record.EditorSandbox() || (lastEnded != nil && (turn.CompletedAt == nil || lastEnded.After(*turn.CompletedAt)))
	if known && !edited {
		return recorded, false, true
	}
	readCtx, cancel := context.WithTimeout(ctx, statusReadTimeout)
	defer cancel()
	read, err := c.Retention.Sandboxes.ReadSessionCheckout(readCtx, record.WorkspaceID, record.ID, record.SandboxID)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			log.Printf("agent-session retention: read the checkout failed (session=%s sandbox=%s): %v", record.ID, record.SandboxID, err)
		}
		return store.TurnCheckout{}, open, false
	}
	return read, open, true
}

// recordCheckout records on the turn what its checkout held undelivered when
// it ended, as the driver reported it: the fact the next message and the idle
// close decide from whether to keep the disk, and the stake the surfaces show.
func (c *Completer) recordCheckout(ctx context.Context, record store.AgentSession, report statusReport) {
	checkout, ok := report.checkout()
	if !ok || !c.Retention.retains() || record.Turns < 1 {
		return
	}
	if err := c.Retention.Store.RecordAgentSessionTurnCheckout(ctx, record.ID, record.Turns, checkout); err != nil {
		// Unknown then: the decision reads the checkout in the sandbox.
		log.Printf("agent-session retention: record the turn's checkout failed (session=%s turn=%d): %v", record.ID, record.Turns, err)
	}
}

// loseWithSandbox records on a failed turn whose sandbox was gone before its
// disk could be kept that whatever it held undelivered was lost with it. What
// it held cannot be read any more, so the stake is unknown, unless the turn
// recorded its checkout clean.
func (c *Completer) loseWithSandbox(ctx context.Context, record store.AgentSession) {
	if !c.Retention.retains() || record.Turns < 1 {
		return
	}
	if turn, found, err := c.currentTurn(ctx, record); err == nil && found {
		if checkout, known := turn.Checkout(); known && !checkout.Undelivered() {
			return
		}
	}
	if err := c.Retention.Store.LoseAgentSessionTurnRetention(ctx, record.ID, record.Turns, store.TurnRetentionCauseFailed, store.RetainedNotSavedSandboxLost, nil); err != nil {
		log.Printf("agent-session retention: record the loss with the sandbox failed (session=%s turn=%d): %v", record.ID, record.Turns, err)
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
