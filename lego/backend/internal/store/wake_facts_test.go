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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m230: a free-tier wake read Deploying with Ready=False/RolloutProgressing
// until its instance was ready. With no deploy row open, a confirmed sighting
// recorded "unhealthy" from the hibernated baseline, so the wake's healthy pass
// emitted an unpaired server_available ("Service recovered"), and
// service_woken was stamped by the noticing pass, 3–15 s late.

var wakeEpoch = time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

type wakeRecorder struct {
	t  *testing.T
	st *memStore
}

func newWakeRecorder(t *testing.T) wakeRecorder {
	return wakeRecorder{t: t, st: newMemStore()}
}

func (r wakeRecorder) record(obs ObservedServiceState) []ServiceEventFact {
	r.t.Helper()
	obs.AppID = "srv-wake"
	facts, err := r.st.RecordObservedServiceState(context.Background(), obs)
	if err != nil {
		r.t.Fatalf("record %+v: %v", obs, err)
	}
	return facts
}

// sleep takes the service from a healthy baseline into Hibernated at +60s.
func (r wakeRecorder) sleep(suspended bool) {
	r.t.Helper()
	r.record(ObservedServiceState{At: wakeEpoch, ServicePhase: "Running", Availability: "healthy", AvailabilityObserved: true})
	if facts := r.record(ObservedServiceState{At: wakeEpoch.Add(time.Minute), ServicePhase: "Hibernated", Suspended: suspended, AvailabilityObserved: true}); len(facts) != 1 {
		r.t.Fatalf("sleep facts = %+v, want one", facts)
	}
}

func startingUp(at time.Duration, concrete bool) ObservedServiceState {
	return ObservedServiceState{
		At: wakeEpoch.Add(at), ServicePhase: "Deploying",
		Availability: "unhealthy", AvailabilityObserved: true,
		ReasonCode: EventReasonReadinessFailed, ConcreteFailure: concrete,
	}
}

func readyAt(transition, observed time.Duration) ObservedServiceState {
	return ObservedServiceState{
		At: wakeEpoch.Add(observed), ServicePhase: "Running",
		Availability: "healthy", AvailabilityObserved: true,
		ReadyTransitionAt: wakeEpoch.Add(transition),
	}
}

func factTypes(facts []ServiceEventFact) []ServiceEventFactType {
	out := make([]ServiceEventFactType, 0, len(facts))
	for _, fact := range facts {
		out = append(out, fact.Type)
	}
	return out
}

// TestWakeAndResumeRecordOnlyTheirLifecycleFact: two confirmed startup
// sightings, then Ready, records exactly service_woken (or service_resumed)
// stamped at the Ready transition — no server_available.
func TestWakeAndResumeRecordOnlyTheirLifecycleFact(t *testing.T) {
	for _, tc := range []struct {
		name      string
		suspended bool
		want      ServiceEventFactType
	}{
		{"idle wake", false, EventFactServiceWoken},
		{"user resume", true, EventFactServiceResumed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newWakeRecorder(t)
			r.sleep(tc.suspended)
			for _, at := range []time.Duration{120 * time.Second, 131 * time.Second} {
				if facts := r.record(startingUp(at, false)); len(facts) != 0 {
					t.Fatalf("startup sighting at +%s emitted %v", at, factTypes(facts))
				}
			}
			facts := r.record(readyAt(140*time.Second, 155*time.Second))
			if len(facts) != 1 || facts[0].Type != tc.want {
				t.Fatalf("ready facts = %v, want only %s", factTypes(facts), tc.want)
			}
			if got, want := facts[0].At, wakeEpoch.Add(140*time.Second); !got.Equal(want) {
				t.Errorf("%s at %s, want the Ready transition %s", tc.want, got, want)
			}
			if facts[0].ReasonCode != "" {
				t.Errorf("%s carries reason %q, want none", tc.want, facts[0].ReasonCode)
			}
		})
	}
}

// TestRunningServiceUnreadyWithoutDeployStillPairs is the control: the rule
// keys on the hibernated baseline, so a running service whose only instance
// goes unready with no deploy open still pages and recovers.
func TestRunningServiceUnreadyWithoutDeployStillPairs(t *testing.T) {
	r := newWakeRecorder(t)
	r.record(ObservedServiceState{At: wakeEpoch, ServicePhase: "Running", Availability: "healthy", AvailabilityObserved: true})
	var got []ServiceEventFactType
	got = append(got, factTypes(r.record(startingUp(time.Minute, false)))...)
	got = append(got, factTypes(r.record(readyAt(90*time.Second, 100*time.Second)))...)
	if len(got) != 2 || got[0] != EventFactServerFailed || got[1] != EventFactServerAvailable {
		t.Fatalf("facts = %v, want server_failed then server_available", got)
	}
}

// TestFailedWakeRecordsAPairableFailure: an instance that cannot wake is down
// to every caller, so a concrete failure during startup records server_failed,
// and the recovery that follows pairs with it.
func TestFailedWakeRecordsAPairableFailure(t *testing.T) {
	r := newWakeRecorder(t)
	r.sleep(false)
	// Ready has been False since the sleep at +60s; its transition dates the
	// sleep, so the failure is stamped by the pass that saw it.
	crashing := startingUp(120*time.Second, true)
	crashing.ReadyTransitionAt = wakeEpoch.Add(61 * time.Second)
	failed := r.record(crashing)
	if len(failed) != 1 || failed[0].Type != EventFactServerFailed || !failed[0].At.Equal(crashing.At) {
		t.Fatalf("failed-wake facts = %+v, want one server_failed at +120s", failed)
	}
	if facts := r.record(startingUp(150*time.Second, true)); len(facts) != 0 {
		t.Fatalf("re-observed failure emitted %v", factTypes(facts))
	}
	got := factTypes(r.record(readyAt(200*time.Second, 210*time.Second)))
	if len(got) != 2 || got[0] != EventFactServiceWoken || got[1] != EventFactServerAvailable {
		t.Fatalf("recovery facts = %v, want service_woken and server_available", got)
	}
}

// TestFailureSpanningASleepIsClosedByTheWake: a service that failed, then
// slept (or was suspended), keeps its failure open through the hibernation, so
// the wake that brings it back healthy records the recovery that closes it.
func TestFailureSpanningASleepIsClosedByTheWake(t *testing.T) {
	r := newWakeRecorder(t)
	r.record(ObservedServiceState{At: wakeEpoch, ServicePhase: "Running", Availability: "healthy", AvailabilityObserved: true})
	var got []ServiceEventFactType
	got = append(got, factTypes(r.record(startingUp(10*time.Second, true)))...)
	got = append(got, factTypes(r.record(ObservedServiceState{At: wakeEpoch.Add(time.Minute), ServicePhase: "Hibernated", AvailabilityObserved: true}))...)
	got = append(got, factTypes(r.record(startingUp(120*time.Second, false)))...)
	got = append(got, factTypes(r.record(readyAt(140*time.Second, 150*time.Second)))...)
	want := []ServiceEventFactType{EventFactServerFailed, EventFactServiceHibernated, EventFactServiceWoken, EventFactServerAvailable}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("facts = %v, want %v", got, want)
	}
}

// TestWokenTimestampFallsBackOutsideItsBounds: with no usable Ready
// transition — missing, before the hibernation it ends, or after the pass that
// saw it — service_woken keeps the observation time; a replay adds nothing.
func TestWokenTimestampFallsBackOutsideItsBounds(t *testing.T) {
	for name, transition := range map[string]time.Duration{
		"missing":        0,
		"before sleep":   30 * time.Second,
		"after the pass": 200 * time.Second,
	} {
		t.Run(name, func(t *testing.T) {
			r := newWakeRecorder(t)
			r.sleep(false)
			obs := readyAt(transition, 155*time.Second)
			if transition == 0 {
				obs.ReadyTransitionAt = time.Time{}
			}
			facts := r.record(obs)
			if len(facts) != 1 || !facts[0].At.Equal(obs.At) {
				t.Fatalf("facts = %+v, want one service_woken at the observation %s", facts, obs.At)
			}
			if again := r.record(obs); len(again) != 0 {
				t.Fatalf("replay emitted %v", factTypes(again))
			}
		})
	}
}

// TestObservedStateMarksConcreteFailures pins the builder half: ordinary
// startup progress is not a concrete failure; CrashLoopBackOff is.
func TestObservedStateMarksConcreteFailures(t *testing.T) {
	for reason, want := range map[string]bool{
		"RolloutProgressing": false, // the operator's reason while a wake starts up
		"CrashLoopBackOff":   true,
		"ImagePullBackOff":   true,
	} {
		obs := observedServiceStateFor("srv-wake", readyCR(appv1alpha1.PhaseDeploying, metav1.ConditionFalse, reason, time.Time{}), false)
		if obs.Availability != "unhealthy" || obs.ConcreteFailure != want {
			t.Errorf("%s: availability %q concrete %v, want unhealthy concrete %v", reason, obs.Availability, obs.ConcreteFailure, want)
		}
	}
}

// TestAServiceThatNeverServedRecordsNoRecovery is w4/350: a first instance
// that never became healthy has no availability to lose, so neither its
// unready start nor a crash loop latches a failure, and its first healthy pass
// is no "Service recovered". Once it has served, an outage pairs as before.
func TestAServiceThatNeverServedRecordsNoRecovery(t *testing.T) {
	deploying := ObservedServiceState{At: wakeEpoch, ServicePhase: "Deploying"}
	for name, first := range map[string]ObservedServiceState{
		"first deploy crash-looping after its row closed": deploying,
		"checkpoint first written mid-outage":             startingUp(0, false),
	} {
		t.Run(name, func(t *testing.T) {
			r := newWakeRecorder(t)
			var got []ServiceEventFactType
			got = append(got, factTypes(r.record(first))...)
			got = append(got, factTypes(r.record(startingUp(10*time.Second, true)))...)
			got = append(got, factTypes(r.record(startingUp(20*time.Second, true)))...)
			got = append(got, factTypes(r.record(readyAt(25*time.Second, 30*time.Second)))...)
			if len(got) != 0 {
				t.Fatalf("a service that never served recorded %v", got)
			}
			got = append(got, factTypes(r.record(startingUp(40*time.Second, true)))...)
			got = append(got, factTypes(r.record(readyAt(45*time.Second, 50*time.Second)))...)
			want := []ServiceEventFactType{EventFactServerFailed, EventFactServerAvailable}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("outage after serving = %v, want %v", got, want)
			}
		})
	}
}
