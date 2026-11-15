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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// What a turn's transcript entry says became of the unpushed work of a turn a
// person stopped (ADR047 D12, w22/m413 t004): its disk, which the next message
// continues from while it is kept. The same words on REST, GraphQL, MCP, the
// dashboard and `bex agent`.
const (
	// RetentionNone: nothing was kept, and nothing is owed: the turn was not
	// stopped, retention is off, its sandbox was already gone or its session
	// was archived. A stopped turn whose disk could not be kept is "lost".
	RetentionNone = "none"
	// RetentionSaving: the turn is settled and its disk is being saved; the
	// answer becomes "kept" or "lost" within minutes.
	RetentionSaving = "saving"
	// RetentionKept: the disk is kept until RetainedUntil and the next message
	// starts from it.
	RetentionKept = "kept"
	// RetentionContinued: the next turn started from the kept disk.
	RetentionContinued = "continued"
	// RetentionLost: the disk was not kept or is gone; the unpushed work with it.
	RetentionLost = "lost"
)

// Why a lost disk was lost (TranscriptTurn.RetentionLostReason). A loss with
// another cause, a turn that started elsewhere before the disk was ready or the
// session's deletion, names none.
const (
	RetentionLostReasonExpired     = "expired"      // the retention time ran out
	RetentionLostReasonStorageFull = "storage_full" // the workspace's storage cap had no room for it
	RetentionLostReasonSaveFailed  = "save_failed"  // pausing, copying or recording the disk failed
	RetentionLostReasonEditorOpen  = "editor_open"  // an editor was open on the sandbox, which Stop does not end
	RetentionLostReasonCanceled    = "canceled"     // the session was canceled
	RetentionLostReasonArchived    = "archived"     // the session was archived
	RetentionLostReasonNotRestored = "not_restored" // the next turn could not start from it
)

// retentionLostReasons maps what the store records as the cause of a loss to
// the reasons the API names.
var retentionLostReasons = map[string]string{
	store.RetainedReleaseExpired:     RetentionLostReasonExpired,
	store.RetainedReleaseStorageFull: RetentionLostReasonStorageFull,
	store.RetainedReleaseSaveFailed:  RetentionLostReasonSaveFailed,
	store.RetainedReleaseAbandoned:   RetentionLostReasonSaveFailed,
	store.RetainedNotSavedEditorOpen: RetentionLostReasonEditorOpen,
	store.RetainedReleaseCanceled:    RetentionLostReasonCanceled,
	store.RetainedReleaseArchived:    RetentionLostReasonArchived,
	store.RetainedReleaseUnusable:    RetentionLostReasonNotRestored,
	store.RetainedReleaseNewAgent:    RetentionLostReasonNotRestored,
}

// turnRetentionView is what a turn's transcript entry says about its disk.
type turnRetentionView struct {
	State      string
	Until      *time.Time
	LostReason string
}

// turnRetention reads a turn's retention record at now. A disk still recorded
// as kept after its time is lost, as the next message treats it (the claim
// refuses an image past its time), without waiting for the Completer's sweep to
// say so.
func turnRetention(turn store.AgentSessionTurn, now time.Time) turnRetentionView {
	if !turn.Interrupted {
		return turnRetentionView{State: RetentionNone}
	}
	switch turn.Retention {
	case store.TurnRetentionSaving:
		return turnRetentionView{State: RetentionSaving}
	case store.TurnRetentionKept:
		if turn.RetainedUntil == nil {
			return turnRetentionView{State: RetentionNone}
		}
		if !now.Before(*turn.RetainedUntil) {
			return turnRetentionView{State: RetentionLost, LostReason: RetentionLostReasonExpired}
		}
		until := *turn.RetainedUntil
		return turnRetentionView{State: RetentionKept, Until: &until}
	case store.TurnRetentionContinued:
		return turnRetentionView{State: RetentionContinued}
	case store.TurnRetentionLost:
		return turnRetentionView{State: RetentionLost, LostReason: retentionLostReasons[turn.RetentionReason]}
	}
	return turnRetentionView{State: RetentionNone}
}
