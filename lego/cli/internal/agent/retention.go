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

// What became of the work a turn's sandbox held undelivered (ADR047 D12,
// w22/m413, w10/m491): its disk, which the session's next message continues
// from while it is kept. A stopped turn keeps it always; a turn that failed,
// idled out or was followed by a message keeps it when its checkout held
// uncommitted changes or unpushed commits. The sentences are the dashboard's,
// in English: bex-api's turn says the state, and the client says it in words.

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
	lostSandboxLost = "sandbox_lost"
)

// stake names what the turn left undelivered, as the API counted it: "2
// unpushed commits and uncommitted changes", and whether that is plural.
// Unknown names fallback.
func stake(t transcriptTurn, fallback string) (string, bool) {
	if t.UnpushedCommits == nil || t.UncommittedChanges == nil || (*t.UnpushedCommits <= 0 && !*t.UncommittedChanges) {
		return fallback, true
	}
	n, dirty := *t.UnpushedCommits, *t.UncommittedChanges
	commits := fmt.Sprintf("%d unpushed commit%s", n, pick(n != 1, "s", ""))
	switch {
	case n > 0 && dirty:
		return commits + " and uncommitted changes", true
	case dirty:
		return "uncommitted changes", true
	}
	return commits, n != 1
}

// retentionSentence is what the turn t says about the work its sandbox held
// undelivered, or "" when it kept nothing (retention "none", or a state this
// CLI does not know: a newer server never makes it claim something it did
// not say).
func retentionSentence(t transcriptTurn) string {
	switch t.Retention {
	case retentionSaving:
		return "Saving your changes so the next message can continue from them."
	case retentionKept:
		if t.RetainedUntil == nil {
			return ""
		}
		what, plural := stake(t, "changes")
		return "Your " + what + " " + pick(plural, "are", "is") + " kept until " + zoneTime(t.RetainedUntil, nil) + "."
	case retentionContinued:
		return "The next message continued from these changes."
	case retentionLost:
		return lostSentence(t)
	}
	return ""
}

// lostSentence says why a lost disk was lost, in the words the dashboard uses;
// a loss with no cause to name (the session was canceled or archived, or a
// reason this CLI does not know) is only said to be a loss.
func lostSentence(t transcriptTurn) string {
	what, plural := stake(t, "unpushed changes")
	were := pick(plural, "were", "was")
	switch t.RetentionLostReason {
	case lostExpired:
		return "Your " + what + " " + were + " lost when the retained sandbox expired."
	case lostStorageFull:
		return "Your " + what + " " + were + " not kept because the workspace's sandbox storage was full."
	case lostSaveFailed:
		return "Your " + what + " could not be saved and " + were + " lost."
	case lostEditorOpen:
		return "Your " + what + " " + were + " not kept because an editor was open on the sandbox."
	case lostNotRestored:
		return "Your " + what + " could not be restored, so the next message started from a fresh checkout."
	case lostSandboxLost:
		return "Your " + what + " " + were + " lost when the turn failed and its sandbox was gone."
	case lostCanceled, lostArchived:
		// The session ended, which its state already says.
	}
	return "Your " + what + " " + were + " lost."
}

func pick(plural bool, many, one string) string {
	if plural {
		return many
	}
	return one
}

// turnNote is the line that ends a noted turn: that a person stopped it, when
// one did, and what became of the work its sandbox held undelivered.
func turnNote(t transcriptTurn) string {
	if t.Interrupted {
		return strings.TrimSpace(fmt.Sprintf("Turn %d was interrupted before it finished. %s", t.Turn, retentionSentence(t)))
	}
	return strings.TrimSpace(fmt.Sprintf("Turn %d: %s", t.Turn, retentionSentence(t)))
}
