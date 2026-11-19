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
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A turn's disk outlives its sandbox (ADR047 D12, w22/m413 and w10/m491, the
// reference being Devin, whose session keeps its machine across sleeps and
// follow-ups). A turn a person stops keeps it always; a turn that failed,
// completed and idled out, or is followed by a message keeps it when its
// checkout holds work it did not deliver (uncommitted changes, commits not
// on the remote branch), at the moment its sandbox would otherwise be ended.
// The Completer, or the dispatch of the next message, reads the transcript,
// then pauses the sandbox, which commits its root filesystem, and copies the
// result to an image in the workspace's images repository: the retained
// image. The session's next turn boots a fresh sandbox from that image, with
// its own environment, secrets and egress, and the driver finds its checkout,
// commits and working-tree edits where the earlier turn left them. The driver
// still never commits or pushes (ADR090).
//
// The retained image's row (agent_session_retained_images) is its handle and
// the one place every decision is made, by compare-and-set on its state:
//
//   - the Completer saves it (saving → kept) and the next turn waits for that
//     save, up to its deadline, instead of racing it;
//   - a dispatch claims it (kept → in_use) in one statement that also checks
//     its time, so an expiry and a Steer never both win, and releases it once
//     its sandbox is up;
//   - expiry, Cancel, archive, Delete, a turn that started in another sandbox
//     and the workspace delete release it, and the purge deletes every
//     released image from the registry before its row goes, so none is left.

// DefaultRetentionTTL is how long a turn's kept disk is kept, whatever ended
// the turn (BEX_AGENT_INTERRUPT_RETENTION). MaxRetentionTTL bounds the setting
// below Devin's 14 days (docs.devin.ai release notes, 2026-10-09).
const (
	DefaultRetentionTTL = 24 * time.Hour
	MaxRetentionTTL     = 7 * 24 * time.Hour
)

// ClampRetentionTTL is the retention time the setting asks for, cut to
// MaxRetentionTTL, and whether it was cut: the one place the bound is
// applied, by the configuration (which warns) and by Retention.
func ClampRetentionTTL(ttl time.Duration) (time.Duration, bool) {
	if ttl > MaxRetentionTTL {
		return MaxRetentionTTL, true
	}
	return ttl, false
}

// Bounds of the retention flow. Vars so tests run quickly.
var (
	// retentionSaveTimeout bounds one save, and so how long the next turn
	// waits for it before it starts from a fresh sandbox.
	retentionSaveTimeout = 5 * time.Minute
	// retentionClaimLease is how long a dispatch may hold a kept image while
	// it boots the sandbox from it; a lapsed claim is let go by the sweep.
	retentionClaimLease = 15 * time.Minute
	// retentionWaitPoll is how often the next turn looks at a save it waits
	// for.
	retentionWaitPoll = time.Second
)

// retentionSaveWindow is how long after a turn settles its save may take, the
// cleanup it does afterwards included: the pause and the copy
// (retentionSaveTimeout), the end of the paused sandbox
// (sandbox.RetainEndTimeout) and the records the Completer writes once the
// save has ended (retentionSettleTimeout). It is the save's deadline, the
// time the next turn waits for it, so that the save and its cleanup always
// finish before any Steer can start fresh and the save never works on a
// session that has moved on.
func retentionSaveWindow() time.Duration {
	return retentionSaveTimeout + sandbox.RetainEndTimeout + retentionSettleTimeout
}

// retentionPurgeTimeout bounds one purge, which runs on the Completer's one
// loop and inside Cancel and archive: a registry that hangs leaves the handle
// to the next sweep and stalls nothing else. A var so tests run quickly.
var retentionPurgeTimeout = 10 * time.Second

// retentionSettleTimeout bounds the records a save writes once it has ended,
// from then, however long the save itself took. A var so tests run quickly.
var retentionSettleTimeout = 30 * time.Second

const (
	retentionPurgeBackoff    = time.Minute
	retentionPurgeMaxBackoff = time.Hour
	retentionPurgeBatch      = 5
	// maxRetentionSaves bounds the saves one replica runs at once.
	maxRetentionSaves = 4
)

// RetainedImageStore is the handle table of retained images; *store.PGStore
// implements it.
type RetainedImageStore interface {
	FinalizeAgentSessionRetaining(ctx context.Context, id, headSHA, deliveryNote string, in store.NewAgentRetainedImage) (store.AgentSession, store.TerminalTurnFact, *store.AgentRetainedImage, error)
	KeepAgentSessionRetainedImage(ctx context.Context, id, digest string, until time.Time) (bool, error)
	AgentSessionRetainedImage(ctx context.Context, sessionID string) (store.AgentRetainedImage, error)
	AgentSessionRetainedImages(ctx context.Context, sessionID string) ([]store.AgentRetainedImage, error)
	ClaimAgentSessionRetainedImage(ctx context.Context, sessionID string, now time.Time, lease time.Duration) (store.AgentRetainedImage, error)
	ClearAgentSessionRetainedSandbox(ctx context.Context, sessionID, sandboxID string) error
	UnclaimAgentSessionRetainedImage(ctx context.Context, id string) error
	FinishAgentSessionRetainedImages(ctx context.Context, sessionID, usedID string, now time.Time) error
	ReleaseAgentSessionRetainedImage(ctx context.Context, id, reason string, now time.Time) (bool, error)
	ReleaseAgentSessionRetainedImages(ctx context.Context, sessionID, reason string, now time.Time) error
	ExpireAgentSessionRetainedImages(ctx context.Context, now time.Time) (int64, error)
	ClaimAgentSessionRetainedImagePurges(ctx context.Context, now time.Time, p store.RetainedImagePurge) ([]store.AgentRetainedImage, error)
	DeleteAgentSessionRetainedImage(ctx context.Context, id string) error
	AgentSessionsSavingRetainedImage(ctx context.Context, sessionIDs []string) (map[string]bool, error)
	SetAgentSessionTurnStartNote(ctx context.Context, sessionID string, turn int, note string) error
	FinalizeAgentSessionNotRetaining(ctx context.Context, id, headSHA, deliveryNote, reason string) (store.AgentSession, store.TerminalTurnFact, error)
	RecordAgentSessionTurnCheckout(ctx context.Context, sessionID string, turn int, c store.TurnCheckout) error
	BeginAgentSessionRetention(ctx context.Context, sessionID string, turn int, sandboxID, cause string, c store.TurnCheckout, in store.NewAgentRetainedImage) (*store.AgentRetainedImage, error)
	LoseAgentSessionTurnRetention(ctx context.Context, sessionID string, turn int, cause, reason string, c *store.TurnCheckout) error
}

// RetentionSandboxes is the sandbox side of retention
// (*sandbox.AgentSessionLifecycle): reading what a checkout left undelivered,
// saving a paused sandbox's disk, deleting a retained image, and the template
// image a disk ran on.
type RetentionSandboxes interface {
	ReadSessionCheckout(ctx context.Context, workspaceID, sessionID, sandboxID string) (store.TurnCheckout, error)
	RetainAgentSessionSandbox(ctx context.Context, workspaceID, sessionID, sandboxID, imageID string) (sandbox.RetainedCommit, error)
	DeleteAgentRetainedImage(ctx context.Context, workspaceID, imageID string) error
	DeleteAgentSessionEgress(ctx context.Context, workspaceID, sessionID string) error
	AgentTemplateImage(template string) string
}

// Retention keeps turns' disks. The same pointer serves the Service, which
// continues from them and releases them, and the Completer, which saves and
// expires them. nil, or without a store or sandboxes, retains nothing and
// every path behaves as it did before retention.
type Retention struct {
	Store     RetainedImageStore
	Sandboxes RetentionSandboxes
	// TTL is how long a kept image is kept; 0 stops new retentions while the
	// ones that exist still expire, continue and are deleted.
	TTL time.Duration

	slotsOnce sync.Once
	slots     chan struct{}
}

// active reports whether retained images can be handled at all.
func (r *Retention) active() bool { return r != nil && r.Store != nil && r.Sandboxes != nil }

// retains reports whether a turn's disk is kept now.
func (r *Retention) retains() bool { return r.active() && r.TTL > 0 }

// ttl is the retention time, never longer than MaxRetentionTTL.
func (r *Retention) ttl() time.Duration {
	ttl, _ := ClampRetentionTTL(r.TTL)
	return ttl
}

func (r *Retention) saveSlots() chan struct{} {
	r.slotsOnce.Do(func() { r.slots = make(chan struct{}, maxRetentionSaves) })
	return r.slots
}

// claim readies the image a session's next turn continues from: it waits for
// a save in progress, up to the save's deadline, and then claims the kept image
// in one compare-and-set that also reads its time. It returns nil, so the turn
// starts from a fresh sandbox, when there is nothing to continue from: no
// image, one that expired, one still being saved at its deadline, or one saved
// on an older agent image than the template's now.
func (r *Retention) claim(ctx context.Context, now func() time.Time, sessionID, template string) *store.AgentRetainedImage {
	if !r.active() {
		return nil
	}
	for {
		img, err := r.Store.AgentSessionRetainedImage(ctx, sessionID)
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		if err != nil {
			log.Printf("agent-session retention: read the retained image failed (session=%s): %v", sessionID, err)
			return nil
		}
		if img.State != store.RetainedImageSaving {
			break
		}
		if !now().Before(img.SaveDeadline) {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(retentionWaitPoll):
		}
	}
	claimed, err := r.Store.ClaimAgentSessionRetainedImage(ctx, sessionID, now(), retentionClaimLease)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			log.Printf("agent-session retention: claim the retained image failed (session=%s): %v", sessionID, err)
		}
		return nil
	}
	if base := r.Sandboxes.AgentTemplateImage(template); claimed.BaseImage != base {
		// The disk carries an older driver than the template boots now.
		log.Printf("agent-session retention: the retained image ran on agent image %q, the template is now %q; starting fresh (session=%s)", claimed.BaseImage, base, sessionID)
		r.release(ctx, now, claimed.ID, store.RetainedReleaseNewAgent)
		return nil
	}
	return &claimed
}

// unclaim lets go of an image a dispatch claimed and did not use.
func (r *Retention) unclaim(id string) {
	ctx, done := context.WithTimeout(context.Background(), retentionSettleTimeout)
	defer done()
	if err := r.Store.UnclaimAgentSessionRetainedImage(ctx, id); err != nil {
		log.Printf("agent-session retention: let go of the retained image failed (id=%s): %v", id, err)
	}
}

// release marks an image for deletion; the purge deletes it.
func (r *Retention) release(ctx context.Context, now func() time.Time, id, reason string) {
	if _, err := r.Store.ReleaseAgentSessionRetainedImage(ctx, id, reason, now()); err != nil {
		log.Printf("agent-session retention: release the retained image failed (id=%s reason=%s): %v", id, reason, err)
	}
}

// finish settles the session's retained images once a turn is running in a
// sandbox: the one the turn booted from is spent, any other is stale. Both go
// to the purge, which deletes them at once.
func (r *Retention) finish(ctx context.Context, now func() time.Time, sessionID, usedID string) {
	if !r.active() {
		return
	}
	if err := r.Store.FinishAgentSessionRetainedImages(ctx, sessionID, usedID, now()); err != nil {
		log.Printf("agent-session retention: settle the retained images failed (session=%s): %v", sessionID, err)
		return
	}
	r.purge(ctx, now(), sessionID)
}

// purge deletes the released images that are due from the registry, then
// their handles; a delete that fails backs off and is tried again. With a
// session id it takes only that session's. It takes a handful at a time and
// never longer than retentionPurgeTimeout: what it leaves is the next sweep's.
func (r *Retention) purge(ctx context.Context, now time.Time, sessionID string) {
	if !r.active() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, retentionPurgeTimeout)
	defer cancel()
	rows, err := r.Store.ClaimAgentSessionRetainedImagePurges(ctx, now, store.RetainedImagePurge{
		Backoff: retentionPurgeBackoff, MaxBackoff: retentionPurgeMaxBackoff, Limit: retentionPurgeBatch, SessionID: sessionID})
	if err != nil {
		log.Printf("agent-session retention: claim retained images to delete failed: %v", err)
		return
	}
	for _, row := range rows {
		if err := r.Sandboxes.DeleteAgentRetainedImage(ctx, row.WorkspaceID, row.ID); err != nil {
			log.Printf("agent-session retention: delete retained image %s failed: %v", row.ID, err)
			continue
		}
		if err := r.Store.DeleteAgentSessionRetainedImage(ctx, row.ID); err != nil {
			log.Printf("agent-session retention: remove the handle of retained image %s failed: %v", row.ID, err)
		}
	}
}

// deleteSessionImages deletes every retained image of a session from the
// registry before the session row, which owns their handles, goes. A failure
// is returned so the Delete can be retried: an image the registry still holds
// must keep its handle.
func (r *Retention) deleteSessionImages(ctx context.Context, now func() time.Time, record store.AgentSession) error {
	if !r.active() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, retentionPurgeTimeout)
	defer cancel()
	if err := r.Store.ReleaseAgentSessionRetainedImages(ctx, record.ID, store.RetainedReleaseDeleted, now()); err != nil {
		return fmt.Errorf("release the retained images: %w", err)
	}
	rows, err := r.Store.AgentSessionRetainedImages(ctx, record.ID)
	if err != nil {
		return fmt.Errorf("list the retained images: %w", err)
	}
	for _, row := range rows {
		if err := r.Sandboxes.DeleteAgentRetainedImage(ctx, row.WorkspaceID, row.ID); err != nil {
			return fmt.Errorf("delete retained image %s: %w", row.ID, err)
		}
	}
	return nil
}

// envCheckoutNote tells the agent of a turn that follows one with
// undelivered work where its checkout comes from: restored from that turn's
// kept disk, or a fresh clone that lost the work (w10/m491). The driver puts
// it before the prompt; the same sentence, for a loss, begins the turn's
// delivery note (its start note).
const envCheckoutNote = "BEX_AGENT_CHECKOUT_NOTE"

// notRestoredNote is the sentence a turn adds to its delivery note, and tells
// its agent, when its sandbox had to start from a fresh checkout although the
// disk of turn kept was kept.
func notRestoredNote(kept int) string {
	return fmt.Sprintf("The changes kept from turn %d could not be restored%s", kept, startedFresh)
}

// startedFresh ends a note that says the turn's checkout is a fresh clone.
const startedFresh = ", so this turn started from a fresh checkout of the branch."

// restoredNote tells the agent of a turn booted from turn prev's kept disk
// that its checkout is the one prev left, with what was at stake.
func restoredNote(prev store.AgentSessionTurn) string {
	stake, plural := stakeOf(prev, "its uncommitted changes and unpushed commits")
	return fmt.Sprintf("This turn's checkout is the one turn %d left, restored from its sandbox's disk: %s %s still there, as that turn left %s.",
		prev.Turn, stake, pick(plural, "are", "is"), pick(plural, "them", "it"))
}

// lostNote is the sentence a turn adds to its delivery note, and tells its
// agent, when it started from a fresh checkout because the work turn prev
// left undelivered was lost; "" when prev lost nothing.
func lostNote(prev store.AgentSessionTurn, now time.Time) string {
	view := turnRetention(prev, now)
	if view.State != RetentionLost {
		return ""
	}
	why := ""
	if phrase := lostReasonPhrases[view.LostReason]; phrase != "" {
		why = " (" + phrase + ")"
	}
	stake, plural := stakeOf(prev, "any uncommitted changes and unpushed commits")
	return fmt.Sprintf("%s from turn %d %s lost%s%s",
		strings.ToUpper(stake[:1])+stake[1:], prev.Turn, pick(plural, "were", "was"), why, startedFresh)
}

// lostReasonPhrases say why a turn's undelivered work was lost, in the start
// note of the turn after it.
var lostReasonPhrases = map[string]string{
	RetentionLostReasonExpired:     "the time it was kept for ran out",
	RetentionLostReasonStorageFull: "the workspace's sandbox storage was full",
	RetentionLostReasonSaveFailed:  "the sandbox's disk could not be saved",
	RetentionLostReasonEditorOpen:  "an editor was open on the sandbox",
	RetentionLostReasonNotRestored: "the kept disk could not be restored",
	RetentionLostReasonSandboxLost: "the sandbox ended before its disk could be kept",
}

// stakeOf names what turn left undelivered ("the 2 unpushed commits and the
// uncommitted changes"), or unknown when its checkout was never read, and
// whether the phrase is plural.
func stakeOf(turn store.AgentSessionTurn, unknown string) (string, bool) {
	c, known := turn.Checkout()
	if !known || !c.Undelivered() {
		return unknown, true
	}
	commits := fmt.Sprintf("the %d unpushed commit%s", c.UnpushedCommits, pick(c.UnpushedCommits != 1, "s", ""))
	switch {
	case c.Dirty && c.UnpushedCommits > 0:
		return commits + " and the uncommitted changes", true
	case c.Dirty:
		return "the uncommitted changes", true
	}
	return commits, c.UnpushedCommits != 1
}

// pick picks the plural or the singular form (the CLI's twin has the same
// name).
func pick(plural bool, many, one string) string {
	if plural {
		return many
	}
	return one
}

// checkoutNote reads the turn this dispatch follows and says where its
// checkout comes from: restored from that turn's kept disk, or, for a fresh
// start that lost that turn's work, the loss. "" when there is nothing to say.
func (s *Service) checkoutNote(ctx context.Context, sessionID string, spec dispatchSpec) string {
	want := spec.turn - 1
	if spec.retained != nil {
		want = spec.retained.Turn
	}
	if want < 1 {
		return ""
	}
	turn, err := s.turnLoader(ctx, sessionID, want)()
	if err != nil || turn.Turn == 0 {
		if err != nil {
			log.Printf("agent-session dispatch: read the turn for the checkout note failed (session=%s): %v", sessionID, err)
		}
		return ""
	}
	if spec.retained != nil {
		return restoredNote(turn)
	}
	return lostNote(turn, s.Now())
}
