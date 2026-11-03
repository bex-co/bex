package agentsessions

import "testing"

func TestStateAndSteerableCoverEveryPhase(t *testing.T) {
	cases := []struct {
		phase    string
		state    string
		steer    bool
		archived bool
	}{
		{PhaseCreating, StateWorking, false, false},
		{PhaseRunning, StateWorking, false, false},
		{PhaseResuming, StateWorking, false, false},
		{PhaseRedispatching, StateWorking, false, false},
		{PhaseCanceling, StateWorking, false, false},
		{PhaseCompleted, StateSucceeded, true, false},
		{PhaseCompleted, StateSucceeded, false, true}, // archived: viewable, not steerable
		{PhaseFailed, StateFailed, true, false},
		{PhaseCanceled, StateCanceled, false, false},
		{"unheard-of", StateWorking, false, false}, // unknown reads as unfinished
	}
	for _, c := range cases {
		if got := stateOf(c.phase); got != c.state {
			t.Errorf("stateOf(%q) = %q, want %q", c.phase, got, c.state)
		}
		if got := steerable(c.phase, c.archived); got != c.steer {
			t.Errorf("steerable(%q, archived=%t) = %t, want %t", c.phase, c.archived, got, c.steer)
		}
	}
}
