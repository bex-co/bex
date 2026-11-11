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
	"testing"
	"time"
)

func TestObservedServiceStateRecordsEachRealEdgeOnce(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	at := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	// suspended mirrors spec.suspended, which the user-driven Suspend verb is
	// the only writer of: this app's Hibernated leg below is a real suspension,
	// not free-tier idle sleep (see TestAutoHibernateFactsStayApartFromSuspend).
	record := func(offset time.Duration, phase, availability string, observed, suspended bool) []ServiceEventFact {
		facts, err := st.RecordObservedServiceState(ctx, ObservedServiceState{
			AppID: "srv-web", At: at.Add(offset), ServicePhase: phase,
			Suspended:    suspended,
			Availability: availability, AvailabilityObserved: observed,
			ReasonCode: EventReasonReadinessFailed,
		})
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return facts
	}

	if facts := record(0, "Running", "healthy", true, false); len(facts) != 0 {
		t.Fatalf("baseline emitted %+v", facts)
	}
	if facts := record(time.Second, "Deploying", "unhealthy", true, false); len(facts) != 1 || facts[0].Type != EventFactServerFailed {
		t.Fatalf("failure facts = %+v, want one server_failed", facts)
	}
	if facts := record(2*time.Second, "Deploying", "unhealthy", true, false); len(facts) != 0 {
		t.Fatalf("steady unhealthy replay emitted %+v", facts)
	}
	if facts := record(3*time.Second, "Running", "healthy", true, false); len(facts) != 1 || facts[0].Type != EventFactServerAvailable {
		t.Fatalf("recovery facts = %+v, want one server_available", facts)
	}
	if facts := record(4*time.Second, "Hibernated", "", true, true); len(facts) != 1 || facts[0].Type != EventFactServiceSuspended {
		t.Fatalf("suspend facts = %+v, want one service_suspended", facts)
	}
	if facts := record(5*time.Second, "Deploying", "", false, false); len(facts) != 0 {
		t.Fatalf("transient resume phase emitted %+v", facts)
	}
	if facts := record(6*time.Second, "Running", "healthy", true, false); len(facts) != 1 || facts[0].Type != EventFactServiceResumed {
		t.Fatalf("resume facts = %+v, want one service_resumed", facts)
	}
	if got := len(st.eventFacts); got != 4 {
		t.Fatalf("durable fact count = %d, want 4 unique edges", got)
	}
}

// A single-tick Ready=False (stale Deployment status during old-pod reaping)
// must not reach the checkpoint; a sustained outage must, one tick late; a
// recovery must pass through immediately (w3/m78 live-leg finding).
func TestDebounceUnhealthySuppressesSingleTickBlips(t *testing.T) {
	once := map[string]bool{}
	unhealthy := ObservedServiceState{AppID: "srv-web", ServicePhase: "Deploying", Availability: "unhealthy", AvailabilityObserved: true, ReasonCode: EventReasonReadinessFailed}
	healthy := ObservedServiceState{AppID: "srv-web", ServicePhase: "Running", Availability: "healthy", AvailabilityObserved: true}

	if got := debounceUnhealthy(unhealthy, once); got.AvailabilityObserved {
		t.Fatalf("first unhealthy tick recorded availability: %+v", got)
	}
	if got := debounceUnhealthy(healthy, once); !got.AvailabilityObserved || got.Availability != "healthy" {
		t.Fatalf("healthy after a blip must pass through untouched: %+v", got)
	}
	if once["srv-web"] {
		t.Fatal("blip left the app marked unhealthy-once")
	}

	if got := debounceUnhealthy(unhealthy, once); got.AvailabilityObserved {
		t.Fatalf("first tick of a real outage recorded availability: %+v", got)
	}
	got := debounceUnhealthy(unhealthy, once)
	if !got.AvailabilityObserved || got.Availability != "unhealthy" || got.ReasonCode != EventReasonReadinessFailed {
		t.Fatalf("second consecutive unhealthy tick must record: %+v", got)
	}
	if got := debounceUnhealthy(healthy, once); !got.AvailabilityObserved {
		t.Fatalf("recovery must be immediate: %+v", got)
	}
}

func TestInsertServiceEventFactIsIdempotentInProducerFake(t *testing.T) {
	st := newMemStore()
	fact := ServiceEventFact{
		SourceKey: "git:delivery:srv-web:ignored", AppID: "srv-web",
		Type: EventFactCommitIgnored, At: time.Now(), ReasonCode: EventReasonBuildFilter,
	}
	inserted, err := st.InsertServiceEventFact(context.Background(), fact)
	if err != nil || !inserted {
		t.Fatalf("first insert = (%v, %v), want (true, nil)", inserted, err)
	}
	inserted, err = st.InsertServiceEventFact(context.Background(), fact)
	if err != nil || inserted {
		t.Fatalf("retry insert = (%v, %v), want (false, nil)", inserted, err)
	}
}

// TestAutoHibernateFactsStayApartFromSuspend is the w6/m47 t002 regression: a
// free-tier service sleeping on its idle timeout observes the same Hibernated
// phase a user-driven suspension does, and the feed used to label both
// "Service suspended" — leaking a false suspend into every webhook and push
// subscriber on every routine sleep cycle. spec.suspended is what separates
// them, and the checkpoint has to remember it so the wake edge reads correctly
// too (spec.suspended is false on the way back up either way).
func TestAutoHibernateFactsStayApartFromSuspend(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	record := func(st *memStore, app string, offset time.Duration, phase string, suspended bool) []ServiceEventFact {
		t.Helper()
		facts, err := st.RecordObservedServiceState(ctx, ObservedServiceState{
			AppID: app, At: at.Add(offset), ServicePhase: phase, Suspended: suspended,
			Availability: "healthy", AvailabilityObserved: phase == "Running",
		})
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return facts
	}
	onlyFact := func(facts []ServiceEventFact, want ServiceEventFactType) {
		t.Helper()
		if len(facts) != 1 || facts[0].Type != want {
			t.Fatalf("facts = %+v, want exactly one %s", facts, want)
		}
	}

	t.Run("idle sleep and wake never borrow the suspend vocabulary", func(t *testing.T) {
		st := newMemStore()
		record(st, "srv-free", 0, "Running", false)
		onlyFact(record(st, "srv-free", time.Minute, "Hibernated", false), EventFactServiceHibernated)
		onlyFact(record(st, "srv-free", 2*time.Minute, "Running", false), EventFactServiceWoken)
	})

	t.Run("a user-driven suspend and resume keep the original vocabulary", func(t *testing.T) {
		st := newMemStore()
		record(st, "srv-paid", 0, "Running", false)
		onlyFact(record(st, "srv-paid", time.Minute, "Hibernated", true), EventFactServiceSuspended)
		// Resume passes through a transient phase with spec.suspended already
		// cleared; the checkpoint must still remember this was a suspension.
		if facts := record(st, "srv-paid", 2*time.Minute, "Deploying", false); len(facts) != 0 {
			t.Fatalf("transient resume phase emitted %+v", facts)
		}
		onlyFact(record(st, "srv-paid", 3*time.Minute, "Running", false), EventFactServiceResumed)
	})

	// w4/562: a sleeping Free service's Suspend and Resume are each recorded
	// at the action, though its phase never moves; a Resume that leaves it
	// asleep is not reported at a later request's wake, which is a wake.
	asleep := func(st *memStore, app string, offset time.Duration, suspended bool) []ServiceEventFact {
		t.Helper()
		facts, err := st.RecordObservedServiceState(ctx, ObservedServiceState{
			AppID: app, At: at.Add(offset), ServicePhase: "Hibernated", Suspended: suspended,
			Asleep: true, AvailabilityObserved: true,
		})
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return facts
	}
	t.Run("a sleeping service's suspend and resume are recorded when the owner acts", func(t *testing.T) {
		st := newMemStore()
		record(st, "srv-both", 0, "Running", false)
		onlyFact(record(st, "srv-both", time.Minute, "Hibernated", false), EventFactServiceHibernated)
		onlyFact(record(st, "srv-both", 2*time.Minute, "Hibernated", true), EventFactServiceSuspended)
		// The Resume clears the flag before the operator re-parks it: held.
		if facts := record(st, "srv-both", 3*time.Minute, "Hibernated", false); len(facts) != 0 {
			t.Fatalf("the flag fall before the operator re-parked it emitted %+v", facts)
		}
		resumed := asleep(st, "srv-both", 4*time.Minute, false)
		onlyFact(resumed, EventFactServiceResumed)
		if !resumed[0].At.Equal(at.Add(4 * time.Minute)) {
			t.Fatalf("resumed at %v, want the pass that saw it re-parked", resumed[0].At)
		}
		if facts := asleep(st, "srv-both", 5*time.Minute, false); len(facts) != 0 {
			t.Fatalf("staying asleep emitted %+v", facts)
		}
		// Days later a request wakes it: a wake, not a second resume.
		onlyFact(record(st, "srv-both", 72*time.Hour, "Running", false), EventFactServiceWoken)
	})

	t.Run("a quick undo the operator re-parks for idleness records both actions", func(t *testing.T) {
		st := newMemStore()
		record(st, "srv-undo", 0, "Running", false)
		record(st, "srv-undo", time.Second, "Running", true)
		facts := asleep(st, "srv-undo", time.Minute, false)
		if len(facts) != 2 || facts[0].Type != EventFactServiceSuspended || facts[1].Type != EventFactServiceResumed {
			t.Fatalf("facts = %+v, want service_suspended then service_resumed", facts)
		}
		onlyFact(record(st, "srv-undo", time.Hour, "Running", false), EventFactServiceWoken)
	})

	t.Run("a sleeping service the Resume wakes reports one resume, at the wake", func(t *testing.T) {
		st := newMemStore()
		record(st, "srv-wake", 0, "Running", false)
		onlyFact(record(st, "srv-wake", time.Minute, "Hibernated", false), EventFactServiceHibernated)
		onlyFact(record(st, "srv-wake", 2*time.Minute, "Hibernated", true), EventFactServiceSuspended)
		if facts := record(st, "srv-wake", 3*time.Minute, "Hibernated", false); len(facts) != 0 {
			t.Fatalf("the flag fall emitted %+v", facts)
		}
		onlyFact(record(st, "srv-wake", 4*time.Minute, "Running", false), EventFactServiceResumed)
	})

	// w4/508: a Resume clears spec.suspended before the phase leaves Hibernated.
	// A pass in that window must not erase the recorded reason, or the wake
	// reads as an idle service_woken and the resume push is lost.
	t.Run("a pass between the Resume flag flip and the wake keeps it a resume", func(t *testing.T) {
		st := newMemStore()
		record(st, "srv-race", 0, "Running", false)
		onlyFact(record(st, "srv-race", time.Minute, "Hibernated", true), EventFactServiceSuspended)
		if facts := record(st, "srv-race", 2*time.Minute, "Hibernated", false); len(facts) != 0 {
			t.Fatalf("Hibernated with suspended cleared emitted %+v", facts)
		}
		onlyFact(record(st, "srv-race", 3*time.Minute, "Running", false), EventFactServiceResumed)
	})

	// w4/527: a Resume that lands before any pass saw the service Hibernated.
	// The pass after the Suspend saw it still Running with spec.suspended set;
	// the first Hibernated pass already reads suspended=false.
	t.Run("a Resume before the Hibernated pass still records suspend and resume", func(t *testing.T) {
		st := newMemStore()
		record(st, "srv-early", 0, "Running", false)
		if facts := record(st, "srv-early", time.Second, "Running", true); len(facts) != 0 {
			t.Fatalf("the suspend flag alone emitted %+v", facts)
		}
		onlyFact(record(st, "srv-early", time.Minute, "Hibernated", false), EventFactServiceSuspended)
		onlyFact(record(st, "srv-early", 2*time.Minute, "Running", false), EventFactServiceResumed)
	})

	// w4/527 run B: Suspend and Resume both land before the service ever stops.
	t.Run("a Suspend undone before the service stops records the pair", func(t *testing.T) {
		st := newMemStore()
		record(st, "srv-quick", 0, "Running", false)
		record(st, "srv-quick", time.Second, "Running", true)
		facts := record(st, "srv-quick", 5*time.Second, "Running", false)
		if len(facts) != 2 || facts[0].Type != EventFactServiceSuspended || facts[1].Type != EventFactServiceResumed {
			t.Fatalf("facts = %+v, want service_suspended then service_resumed", facts)
		}
		// w4/555: each carries its own moment — the suspension when it was
		// first recorded, the resume when it was noticed — so the newest-first
		// feed lists the resume on top.
		if !facts[0].At.Equal(at.Add(time.Second)) || !facts[1].At.Equal(at.Add(5*time.Second)) {
			t.Fatalf("suspended at %v, resumed at %v; want %v and %v", facts[0].At, facts[1].At, at.Add(time.Second), at.Add(5*time.Second))
		}
		// The episode is closed: a later idle sleep is an idle sleep.
		onlyFact(record(st, "srv-quick", time.Hour, "Hibernated", false), EventFactServiceHibernated)
		onlyFact(record(st, "srv-quick", 2*time.Hour, "Running", false), EventFactServiceWoken)
	})

	// w4/555: a suspension first recorded by the very pass that sees the
	// Resume still lists below it, a microsecond earlier.
	t.Run("a quick suspend pair never shares one timestamp", func(t *testing.T) {
		pair := quickSuspendPair(
			ServiceEventFact{Type: EventFactServiceSuspended, At: at},
			ServiceEventFact{Type: EventFactServiceResumed, At: at},
			at)
		if !pair[0].At.Equal(at.Add(-time.Microsecond)) || !pair[1].At.Equal(at) {
			t.Fatalf("pair = %+v", pair)
		}
	})

	t.Run("an idle sleep stays an idle wake across repeated Hibernated passes", func(t *testing.T) {
		st := newMemStore()
		record(st, "srv-idle", 0, "Running", false)
		onlyFact(record(st, "srv-idle", time.Minute, "Hibernated", false), EventFactServiceHibernated)
		if facts := record(st, "srv-idle", 2*time.Minute, "Hibernated", false); len(facts) != 0 {
			t.Fatalf("steady Hibernated emitted %+v", facts)
		}
		onlyFact(record(st, "srv-idle", 3*time.Minute, "Running", false), EventFactServiceWoken)
	})
}

// TestDebouncedUnhealthySightingIsConfirmedPromptly is w4/207: a ~50s outage
// right after a first deploy was seen by one 30s pass, debounced, then seen
// healthy, and left no server_failed/server_available at all. A debounced
// first sighting now schedules one coalesced confirm pass promptly, so any
// outage longer than the confirm delay meets the second pass it needs.
func TestDebouncedUnhealthySightingIsConfirmedPromptly(t *testing.T) {
	defer func(d time.Duration) { unhealthyConfirmAfter = d }(unhealthyConfirmAfter)
	unhealthyConfirmAfter = 20 * time.Millisecond
	r := &Reconciler{kick: make(chan struct{}, 1)}
	ctx := context.Background()
	unhealthy := ObservedServiceState{AppID: "srv-web", ServicePhase: "Running", Availability: "unhealthy", AvailabilityObserved: true, ReasonCode: EventReasonReadinessFailed}
	healthy := ObservedServiceState{AppID: "srv-other", ServicePhase: "Running", Availability: "healthy", AvailabilityObserved: true}

	if got := r.guardServiceObservation(ctx, healthy); !got.AvailabilityObserved {
		t.Fatalf("healthy pass suppressed: %+v", got)
	}
	if r.confirmPending.Load() {
		t.Fatal("a healthy pass scheduled a confirmation")
	}
	if got := r.guardServiceObservation(ctx, unhealthy); got.AvailabilityObserved {
		t.Fatalf("first unhealthy sighting recorded: %+v", got)
	}
	r.guardServiceObservation(ctx, ObservedServiceState{AppID: "srv-db", Availability: "unhealthy", AvailabilityObserved: true})
	select {
	case <-r.kick:
	case <-time.After(time.Second):
		t.Fatal("debounced sighting scheduled no confirm pass")
	}
	select {
	case <-r.kick:
		t.Fatal("two sightings in one window scheduled two passes")
	case <-time.After(100 * time.Millisecond):
	}
	if got := r.guardServiceObservation(ctx, unhealthy); !got.AvailabilityObserved || got.Availability != "unhealthy" {
		t.Fatalf("the confirming pass must record the outage: %+v", got)
	}
}
