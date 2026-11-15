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
		{"a turn that was not stopped reads none whatever is recorded", store.AgentSessionTurn{Retention: store.TurnRetentionKept, RetainedUntil: &later}, RetentionNone, nil, ""},
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
	named := map[string]string{
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
	// Continued is not a loss; superseded means a later turn made the disk
	// stale, and deleted ends the session, so neither tells a reader anything.
	leftOut := []string{store.RetainedReleaseContinued, store.RetainedReleaseSuperseded, store.RetainedReleaseDeleted}
	all := []string{
		store.RetainedReleaseContinued, store.RetainedReleaseSuperseded, store.RetainedReleaseExpired, store.RetainedReleaseCanceled,
		store.RetainedReleaseArchived, store.RetainedReleaseDeleted, store.RetainedReleaseSaveFailed, store.RetainedReleaseStorageFull,
		store.RetainedReleaseUnusable, store.RetainedReleaseAbandoned, store.RetainedReleaseNewAgent, store.RetainedNotSavedEditorOpen,
	}
	now := time.Now()
	for _, stored := range all {
		got := turnRetention(store.AgentSessionTurn{Interrupted: true, Retention: store.TurnRetentionLost, RetentionReason: stored}, now).LostReason
		if want, ok := named[stored]; ok {
			if got != want {
				t.Errorf("a loss recorded as %q reads %q, want %q", stored, got, want)
			}
		} else if !slices.Contains(leftOut, stored) || got != "" {
			t.Errorf("a loss recorded as %q reads %q: name it, or list it as left out on purpose", stored, got)
		}
	}
	if len(named)+len(leftOut) != len(all) {
		t.Errorf("%d named + %d left out is not the %d stored reasons", len(named), len(leftOut), len(all))
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
