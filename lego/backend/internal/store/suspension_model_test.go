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

package store

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// operatorSecond is what the operator reports at one second of a scenario.
type operatorSecond struct {
	phase appv1alpha1.AppPhase
	// ready is when the latest wake completed (0: none yet), the Ready
	// transition a pass that sees Running carries.
	ready int
}

// playOperator plays a service's phase, second by second, under Suspends and
// Resumes that alternate from a Suspend at the given seconds: a Suspend stops
// the instance dPark seconds later unless a Resume comes first, and a Resume of
// a stopped service wakes it dWake seconds later unless a Suspend comes first.
func playOperator(actions []int, seconds int, dPark, dWake []int) []operatorSecond {
	out := make([]operatorSecond, seconds+1)
	phase := appv1alpha1.PhaseRunning
	parkAt, wakeAt, ready := -1, -1, 0
	next := 0
	for t := 0; t <= seconds; t++ {
		if next < len(actions) && actions[next] == t {
			if next%2 == 0 { // Suspend
				wakeAt = -1
				if phase != appv1alpha1.PhaseHibernated {
					parkAt = t + dPark[next]
				}
			} else { // Resume
				parkAt = -1
				switch phase {
				case appv1alpha1.PhaseHibernated:
					phase, wakeAt = appv1alpha1.PhaseDeploying, t+dWake[next]
				case appv1alpha1.PhaseDeploying:
					// A Suspend stopped its wake; this Resume starts it again.
					wakeAt = t + dWake[next]
				}
			}
			next++
		}
		if parkAt == t {
			phase, parkAt = appv1alpha1.PhaseHibernated, -1
		}
		if wakeAt == t {
			phase, wakeAt, ready = appv1alpha1.PhaseRunning, -1, t
		}
		out[t] = operatorSecond{phase: phase, ready: ready}
	}
	return out
}

// TestSuspensionReplayReportsEveryActionUnderAnyInterleaving drives the event
// projector with random Suspend/Resume series, an operator that stops and wakes
// the instance in between, and passes at random times that read the spec before
// they record. Each action must have its own event, in order (w21/065):
// service_suspended and service_resumed alternate, once per action. The model
// does not cover a service that sleeps from idleness, or one whose spec changes
// by anything but these two verbs.
func TestSuspensionReplayReportsEveryActionUnderAnyInterleaving(t *testing.T) {
	ctx := context.Background()
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	at := func(seconds int) time.Time { return base.Add(time.Duration(seconds) * time.Second) }
	for seed := uint64(1); seed <= 20000; seed++ {
		rng := rand.New(rand.NewPCG(seed, 65))
		n := 1 + rng.IntN(10)
		actions := make([]int, n)
		dPark, dWake := make([]int, n), make([]int, n)
		clock := 5
		for i := range actions {
			clock += 1 + rng.IntN(30)
			actions[i] = clock
			dPark[i], dWake[i] = 1+rng.IntN(12), 5+rng.IntN(36)
		}
		end := clock + 150
		operator := playOperator(actions, end, dPark, dWake)
		flagAt := func(second int) bool {
			flag := false
			for i, a := range actions {
				if a <= second {
					flag = i%2 == 0
				}
			}
			return flag
		}

		type step struct {
			second int
			log    int // index of the action to log, or -1 for a pass
			read   int // second a pass read the spec
		}
		var steps []step
		for i, a := range actions {
			steps = append(steps, step{second: a, log: i})
		}
		for p := 3 + rng.IntN(20); p > 0; p-- {
			record := 1 + rng.IntN(end-60)
			steps = append(steps, step{second: record, log: -1, read: max(record-rng.IntN(7), 0)})
		}
		// The last pass runs once everything has settled.
		steps = append(steps, step{second: end, log: -1, read: end - 1})
		// An action is logged before a pass that records at the same second.
		slices.SortStableFunc(steps, func(a, b step) int {
			if a.second != b.second {
				return a.second - b.second
			}
			return b.log - a.log
		})

		st := newMemStore()
		appID := fmt.Sprintf("srv-model-%d", seed)
		if _, err := st.RecordObservedServiceState(ctx, running(appID, at(0), at(0), false)); err != nil {
			t.Fatal(err)
		}
		var got []ServiceEventFact
		for _, s := range steps {
			if s.log >= 0 {
				if err := st.RecordServiceSuspensionChange(ctx, appID, s.log%2 == 0, at(s.second)); err != nil {
					t.Fatal(err)
				}
				continue
			}
			state := operator[s.second]
			obs := ObservedServiceState{
				AppID: appID, At: at(s.second), SpecReadAt: at(s.read), ServicePhase: string(state.phase),
				Suspended: flagAt(s.read), AvailabilityObserved: true,
			}
			if state.phase == appv1alpha1.PhaseRunning {
				obs.Availability = "healthy"
				// Half the scenarios give the pass no Ready time to place a wake by.
				if state.ready > 0 && seed%2 == 0 {
					obs.ReadyTransitionAt = at(state.ready)
				}
			}
			facts, err := st.RecordObservedServiceState(ctx, obs)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, facts...)
		}

		var lifecycle []ServiceEventFact
		for _, f := range got {
			if f.Type == EventFactServiceSuspended || f.Type == EventFactServiceResumed {
				lifecycle = append(lifecycle, f)
			}
		}
		slices.SortStableFunc(lifecycle, func(a, b ServiceEventFact) int { return a.At.Compare(b.At) })
		var types []ServiceEventFactType
		for _, f := range lifecycle {
			types = append(types, f.Type)
		}
		var wanted []ServiceEventFactType
		for i := range actions {
			wanted = append(wanted, []ServiceEventFactType{EventFactServiceSuspended, EventFactServiceResumed}[i%2])
		}
		if !slices.Equal(types, wanted) {
			var timed []string
			for _, f := range lifecycle {
				timed = append(timed, fmt.Sprintf("%s@%d", f.Type, int(f.At.Sub(base)/time.Second)))
			}
			var order []string
			for _, st := range steps {
				if st.log >= 0 {
					order = append(order, fmt.Sprintf("log(%d)@%d", st.log, st.second))
				} else {
					order = append(order, fmt.Sprintf("pass@%d(read %d,%s)", st.second, st.read, operator[st.second].phase))
				}
			}
			t.Fatalf("seed %d: actions at %v (park delays %v, wake delays %v) reported %v, want one event per action: %v\nphases: %v\nevents: %v\nsteps: %v",
				seed, actions, dPark, dWake, types, wanted, phaseRuns(operator), timed, order)
		}
	}
}

// phaseRuns summarizes the operator's phases as "phase@second" changes.
func phaseRuns(seconds []operatorSecond) []string {
	var out []string
	var last appv1alpha1.AppPhase
	for t, s := range seconds {
		if s.phase != last {
			out = append(out, fmt.Sprintf("%s@%d", s.phase, t))
			last = s.phase
		}
	}
	return out
}
