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
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// turnRetention turns the store's record of a stopped turn's disk into what
// every surface says (w22/m413 t004).
func TestTurnRetentionReadsTheStoredRecord(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	later, earlier := now.Add(time.Hour), now.Add(-time.Hour)
	cases := []struct {
		name      string
		turn      store.AgentSessionTurn
		retention string
		until     *time.Time
		reason    string
	}{
		{"nothing recorded", store.AgentSessionTurn{Interrupted: true}, RetentionNone, nil, ""},
		{"saving", store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionSaving}, RetentionSaving, nil, ""},
		{"kept, with its time", store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionKept, RetainedUntil: &later}, RetentionKept, &later, ""},
		{"kept, but its time has passed", store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionKept, RetainedUntil: &earlier}, RetentionLost, nil, RetentionLostReasonExpired},
		{"kept, to the instant", store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionKept, RetainedUntil: &now}, RetentionLost, nil, RetentionLostReasonExpired},
		{"kept, with no time recorded", store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionKept}, RetentionNone, nil, ""},
		{"continued", store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionContinued, RetainedUntil: &later}, RetentionContinued, nil, ""},
		{"lost, no reason recorded", store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionLost}, RetentionLost, nil, ""},
		{"a turn that was not stopped reads as recorded (w10/m491)", store.AgentSessionTurn{Retention: store.TurnRetentionKept, RetainedUntil: &later, RetentionCause: store.TurnRetentionCauseFailed}, RetentionKept, &later, ""},
		{"a turn that was not stopped and kept nothing reads none", store.AgentSessionTurn{}, RetentionNone, nil, ""},
		{"an unknown record reads none", store.AgentSessionTurn{Interrupted: true, Retention: "mystery"}, RetentionNone, nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := turnRetention(c.turn, now)
			if got.State != c.retention || got.LostReason != c.reason {
				t.Fatalf("turnRetention = %+v, want %s / %q", got, c.retention, c.reason)
			}
			if (got.Until == nil) != (c.until == nil) || (got.Until != nil && !got.Until.Equal(*c.until)) {
				t.Fatalf("until = %v, want %v", got.Until, c.until)
			}
		})
	}
}

// Every cause the store records for a loss either has a name in the API or is
// one the API leaves out on purpose, so a new release reason cannot slip by
// unnamed unnoticed.
func TestEveryStoredLossReasonIsNamedOrLeftOutOnPurpose(t *testing.T) {
	// Continued is not a loss; superseded means a later turn made the disk
	// stale, and deleted ends the session, so neither tells a reader anything.
	leftOut := []string{store.RetainedReleaseContinued, store.RetainedReleaseSuperseded, store.RetainedReleaseDeleted}
	all := []string{
		store.RetainedReleaseContinued, store.RetainedReleaseSuperseded, store.RetainedReleaseExpired, store.RetainedReleaseCanceled,
		store.RetainedReleaseArchived, store.RetainedReleaseDeleted, store.RetainedReleaseSaveFailed, store.RetainedReleaseStorageFull,
		store.RetainedReleaseUnusable, store.RetainedReleaseAbandoned, store.RetainedReleaseNewAgent, store.RetainedNotSavedEditorOpen,
		store.RetainedNotSavedSandboxLost,
	}
	now := time.Now()
	for _, stored := range all {
		got := turnRetention(store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionLost, RetentionReason: stored}, now).LostReason
		_, named := retentionLostReasons[stored]
		switch {
		case named && got == "":
			t.Errorf("a loss recorded as %q is named in the table but reads no reason", stored)
		case !named && (!slices.Contains(leftOut, stored) || got != ""):
			t.Errorf("a loss recorded as %q reads %q: name it, or list it as left out on purpose", stored, got)
		}
	}
	if len(retentionLostReasons)+len(leftOut) != len(all) {
		t.Errorf("%d named + %d left out is not the %d stored reasons", len(retentionLostReasons), len(leftOut), len(all))
	}
}

// The transcript says a stopped turn's disk is being saved, which only the
// retention-aware read of the turns knows. The plain read is the one the SSH
// gateway's database role calls, and its role cannot read the table that
// derives it, so the service must ask for the aware one.
func TestTranscriptReadsTheTurnsThroughTheRetentionAwareStore(t *testing.T) {
	svc, st, _, id := steerableFixture(t)
	turn := st.turns[id][1]
	turn.Interrupted = true
	st.turns[id][1] = turn
	st.savingTurns = map[string]bool{id: true}

	page, err := svc.Transcript(caller("alice"), id, -1, 0)
	if err != nil || len(page.Turns) == 0 || page.Turns[0].Retention != RetentionSaving {
		t.Fatalf("transcript turns = %+v / %v, want the first reading saving", page.Turns, err)
	}
	plain, err := st.AgentSessionTurns(context.Background(), id)
	if err != nil || plain[0].Retention != "" {
		t.Fatalf("the plain read says %q: it must not derive what the gateway's role cannot read", plain[0].Retention)
	}
}

// w10/m491: the cause and the stake ride every state but none; a stopped turn
// recorded before the cause was reads stopped.
func TestTurnRetentionCarriesTheCauseAndTheStake(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	later := now.Add(time.Hour)
	dirty, unpushed := true, 2
	got := turnRetention(store.AgentSessionTurn{Retention: store.TurnRetentionKept, RetainedUntil: &later,
		RetentionCause: store.TurnRetentionCauseReplaced, CheckoutDirty: &dirty, UnpushedCommits: &unpushed}, now)
	if got.Cause != RetentionCauseReplaced || got.Uncommitted == nil || !*got.Uncommitted || got.UnpushedCommits == nil || *got.UnpushedCommits != 2 {
		t.Fatalf("view = %+v", got)
	}
	if got := turnRetention(store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionContinued}, now); got.Cause != RetentionCauseStopped || got.UnpushedCommits != nil {
		t.Fatalf("an older stopped turn = %+v, want stopped with no stake", got)
	}
	if got := turnRetention(store.AgentSessionTurn{RetentionCause: store.TurnRetentionCauseFailed, CheckoutDirty: &dirty, UnpushedCommits: &unpushed}, now); got.Cause != "" || got.UnpushedCommits != nil {
		t.Fatalf("a turn that kept nothing = %+v, want neither", got)
	}
	lost := turnRetention(store.AgentSessionTurn{Retention: store.TurnRetentionLost, RetentionReason: store.RetainedNotSavedSandboxLost, RetentionCause: store.TurnRetentionCauseFailed}, now)
	if lost.LostReason != RetentionLostReasonSandboxLost || lost.Cause != RetentionCauseFailed {
		t.Fatalf("a lost sandbox = %+v", lost)
	}
}

// The start note and the agent's note name what was at stake, with the verb
// agreeing, and why it was lost.
func TestCheckoutNotesNameTheStake(t *testing.T) {
	now := time.Now()
	n := func(v int) *int { return &v }
	b := func(v bool) *bool { return &v }
	for _, tc := range []struct {
		name string
		turn store.AgentSessionTurn
		lost string
		kept string
	}{
		{"commits and changes", store.AgentSessionTurn{Turn: 3, Retention: store.TurnRetentionLost, RetentionReason: store.RetainedReleaseStorageFull, CheckoutDirty: b(true), UnpushedCommits: n(2)},
			"The 2 unpushed commits and the uncommitted changes from turn 3 were lost (the workspace's sandbox storage was full), so this turn started from a fresh checkout of the branch.",
			"This turn's checkout is the one turn 3 left, restored from its sandbox's disk: the 2 unpushed commits and the uncommitted changes are still there, as that turn left them."},
		{"one commit", store.AgentSessionTurn{Turn: 1, Retention: store.TurnRetentionLost, RetentionReason: store.RetainedReleaseExpired, CheckoutDirty: b(false), UnpushedCommits: n(1)},
			"The 1 unpushed commit from turn 1 was lost (the time it was kept for ran out), so this turn started from a fresh checkout of the branch.",
			"This turn's checkout is the one turn 1 left, restored from its sandbox's disk: the 1 unpushed commit is still there, as that turn left it."},
		{"unknown, no reason", store.AgentSessionTurn{Turn: 2, Retention: store.TurnRetentionLost},
			"Any uncommitted changes and unpushed commits from turn 2 were lost, so this turn started from a fresh checkout of the branch.",
			"This turn's checkout is the one turn 2 left, restored from its sandbox's disk: its uncommitted changes and unpushed commits are still there, as that turn left them."},
	} {
		if got := lostNote(tc.turn, now); got != tc.lost {
			t.Errorf("%s lost:\n got %q\nwant %q", tc.name, got, tc.lost)
		}
		if got := restoredNote(tc.turn); got != tc.kept {
			t.Errorf("%s restored:\n got %q\nwant %q", tc.name, got, tc.kept)
		}
	}
	if got := lostNote(store.AgentSessionTurn{Turn: 1, Retention: store.TurnRetentionContinued}, now); got != "" {
		t.Errorf("a continued turn's note = %q, want none", got)
	}
}
