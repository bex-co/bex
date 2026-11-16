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

package agent

import (
	"fmt"
	"strings"
)

// What became of the unpushed work of a turn a person stopped (ADR047 D12,
// w22/m413): its disk, which the session's next message continues from while
// it is kept. The sentences are the dashboard's, in English: bex-api's turn
// says the state, and the client says it in words.

// A turn's retention states and, for a lost one, why: the words bex-api's
// transcript uses (the fixture and its backend twin pin both lists).
const (
	retentionNone      = "none"
	retentionSaving    = "saving"
	retentionKept      = "kept"
	retentionContinued = "continued"
	retentionLost      = "lost"
)

const (
	lostExpired     = "expired"
	lostStorageFull = "storage_full"
	lostSaveFailed  = "save_failed"
	lostEditorOpen  = "editor_open"
	lostCanceled    = "canceled"
	lostArchived    = "archived"
	lostNotRestored = "not_restored"
)

// retentionSentence is what the stopped turn t says about its unpushed work,
// or "" when it kept nothing (retention "none", or a state this CLI does not
// know: a newer server never makes it claim something it did not say).
func retentionSentence(t transcriptTurn) string {
	switch t.Retention {
	case retentionSaving:
		return "Saving your changes so the next message can continue from them."
	case retentionKept:
		if t.RetainedUntil == nil {
			return ""
		}
		return "Your changes are kept until " + zoneTime(t.RetainedUntil, nil) + "."
	case retentionContinued:
		return "The next message continued from these changes."
	case retentionLost:
		return lostSentence(t.RetentionLostReason)
	}
	return ""
}

// lostSentence says why a lost disk was lost, in the words the dashboard uses;
// a loss with no cause to name (the session was canceled or archived, or a
// reason this CLI does not know) is only said to be a loss.
func lostSentence(reason string) string {
	switch reason {
	case lostExpired:
		return "Your unpushed changes were lost when the retained sandbox expired."
	case lostStorageFull:
		return "Your unpushed changes were not kept because the workspace's sandbox storage was full."
	case lostSaveFailed:
		return "Your unpushed changes could not be saved and were lost."
	case lostEditorOpen:
		return "Your unpushed changes were not kept because an editor was open on the sandbox."
	case lostNotRestored:
		return "Your unpushed changes could not be restored, so the next message started from a fresh checkout."
	case lostCanceled, lostArchived:
		// The session ended, which its state already says.
	}
	return "Your unpushed changes were lost."
}

// stoppedNote is the line that ends a turn a person stopped: that it was, and
// what became of its unpushed work.
func stoppedNote(t transcriptTurn) string {
	return strings.TrimSpace(fmt.Sprintf("Turn %d was interrupted before it finished. %s", t.Turn, retentionSentence(t)))
}
