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

// What a turn's transcript entry says became of the work its sandbox held
// undelivered (ADR047 D12, w22/m413 t004, w10/m491): its disk, which the next
// message continues from while it is kept. A stopped turn's disk is always
// kept; a turn that failed, idled out or was followed by a message keeps its
// disk when its checkout held uncommitted changes or unpushed commits. The
// same words on REST, GraphQL, MCP, the dashboard and `bex agent`.
const (
	// RetentionNone: nothing was kept, and nothing is owed: the turn left
	// nothing undelivered, retention is off, or its session was archived. A
	// turn whose undelivered work could not be kept is "lost".
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
	RetentionLostReasonEditorOpen  = "editor_open"  // an editor was open on the sandbox when it was stopped or replaced
	RetentionLostReasonCanceled    = "canceled"     // the session was canceled
	RetentionLostReasonArchived    = "archived"     // the session was archived
	RetentionLostReasonNotRestored = "not_restored" // the next turn could not start from it
	RetentionLostReasonSandboxLost = "sandbox_lost" // the sandbox was gone before its disk could be kept
)

// What ended the sandbox whose disk a turn kept or lost
// (TranscriptTurn.RetentionCause, w10/m491).
const (
	RetentionCauseStopped    = "stopped"     // a person stopped the turn
	RetentionCauseFailed     = "failed"      // the turn failed
	RetentionCauseReplaced   = "replaced"    // the next message replaced the sandbox
	RetentionCauseIdleClosed = "idle_closed" // the idle grace closed the sandbox
)

// retentionCauses maps what the store records as a retention's cause to the
// causes the API names: the same words.
var retentionCauses = map[string]string{
	store.TurnRetentionCauseStopped:    RetentionCauseStopped,
	store.TurnRetentionCauseFailed:     RetentionCauseFailed,
	store.TurnRetentionCauseReplaced:   RetentionCauseReplaced,
	store.TurnRetentionCauseIdleClosed: RetentionCauseIdleClosed,
}

// retentionLostReasons maps what the store records as the cause of a loss to
// the reasons the API names.
var retentionLostReasons = map[string]string{
	store.RetainedReleaseExpired:      RetentionLostReasonExpired,
	store.RetainedReleaseStorageFull:  RetentionLostReasonStorageFull,
	store.RetainedReleaseSaveFailed:   RetentionLostReasonSaveFailed,
	store.RetainedReleaseAbandoned:    RetentionLostReasonSaveFailed,
	store.RetainedNotSavedEditorOpen:  RetentionLostReasonEditorOpen,
	store.RetainedReleaseCanceled:     RetentionLostReasonCanceled,
	store.RetainedReleaseArchived:     RetentionLostReasonArchived,
	store.RetainedReleaseUnusable:     RetentionLostReasonNotRestored,
	store.RetainedReleaseNewAgent:     RetentionLostReasonNotRestored,
	store.RetainedNotSavedSandboxLost: RetentionLostReasonSandboxLost,
}

// turnRetentionView is what a turn's transcript entry says about its disk:
// its state, what ended the sandbox (Cause) and what was at stake
// (UnpushedCommits and Uncommitted, nil when unknown), the last three only for
// a state other than none.
type turnRetentionView struct {
	State           string
	Until           *time.Time
	LostReason      string
	Cause           string
	UnpushedCommits *int
	Uncommitted     *bool
}

// turnRetention reads a turn's retention record at now. A disk still recorded
// as kept after its time is lost, as the next message treats it (the claim
// refuses an image past its time), without waiting for the Completer's sweep to
// say so.
func turnRetention(turn store.AgentSessionTurn, now time.Time) turnRetentionView {
	view := turnRetentionState(turn, now)
	if view.State == RetentionNone {
		return view
	}
	view.Cause = retentionCauses[turn.RetentionCause]
	if view.Cause == "" && turn.Interrupted {
		// Recorded before the cause was (w22/m413): only a stop kept a disk.
		view.Cause = RetentionCauseStopped
	}
	if _, known := turn.Checkout(); known {
		view.UnpushedCommits, view.Uncommitted = turn.UnpushedCommits, turn.CheckoutDirty
	}
	return view
}

// turnRetentionState is turnRetention's state, time and reason.
func turnRetentionState(turn store.AgentSessionTurn, now time.Time) turnRetentionView {
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

// The descriptions of a turn's retention fields, shared by GraphQL and MCP so
// the two say the same thing (REST carries the same JSON as MCP).
const (
	retentionDescription           = "What became of the work the turn's sandbox held undelivered (its disk, ADR047 D12): none (nothing was kept and nothing is owed: the turn left nothing undelivered, retention is off, or the session was archived), saving (being saved now), kept (until retainedUntil; the next message continues from it), continued (the next turn started from it) or lost (retentionLostReason says why, when it is known). A stopped turn's disk is always kept; a turn that failed, was left idle until its sandbox closed, or was followed by a message keeps its disk when its checkout held uncommitted changes or unpushed commits."
	retainedUntilDescription       = "When the disk of a kept turn expires (RFC 3339); null for every other retention."
	retentionLostReasonDescription = "Why a lost disk was lost: expired, storage_full (the workspace's storage cap was full), save_failed, editor_open (an editor was open on the sandbox when it was stopped or replaced), canceled, archived, not_restored (the next turn could not start from it) or sandbox_lost (the sandbox was gone before its disk could be kept)."
	retentionCauseDescription      = "What ended the sandbox whose disk was kept or lost: stopped (a person stopped the turn), failed (the turn failed), replaced (the next message replaced the sandbox) or idle_closed (the sandbox was closed after the session sat idle)."
	unpushedCommitsDescription     = "How many commits the turn's checkout held that were not on the remote session branch, for a turn whose disk was kept or lost."
	uncommittedChangesDescription  = "Whether the turn's checkout held uncommitted changes, for a turn whose disk was kept or lost."
	startNoteDescription           = "Set on a turn that started from a fresh checkout although the turn before it left work undelivered: the sentence its delivery note begins with, saying that work could not be restored or was lost."
)
