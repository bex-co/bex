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
	record := func(offset time.Duration, phase, availability string, observed bool) []ServiceEventFact {
		facts, err := st.RecordObservedServiceState(ctx, ObservedServiceState{
			AppID: "srv-web", At: at.Add(offset), ServicePhase: phase,
			Availability: availability, AvailabilityObserved: observed,
			ReasonCode: EventReasonReadinessFailed,
		})
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return facts
	}

	if facts := record(0, "Running", "healthy", true); len(facts) != 0 {
		t.Fatalf("baseline emitted %+v", facts)
	}
	if facts := record(time.Second, "Deploying", "unhealthy", true); len(facts) != 1 || facts[0].Type != EventFactServerFailed {
		t.Fatalf("failure facts = %+v, want one server_failed", facts)
	}
	if facts := record(2*time.Second, "Deploying", "unhealthy", true); len(facts) != 0 {
		t.Fatalf("steady unhealthy replay emitted %+v", facts)
	}
	if facts := record(3*time.Second, "Running", "healthy", true); len(facts) != 1 || facts[0].Type != EventFactServerAvailable {
		t.Fatalf("recovery facts = %+v, want one server_available", facts)
	}
	if facts := record(4*time.Second, "Hibernated", "", true); len(facts) != 1 || facts[0].Type != EventFactServiceSuspended {
		t.Fatalf("suspend facts = %+v, want one service_suspended", facts)
	}
	if facts := record(5*time.Second, "Deploying", "", false); len(facts) != 0 {
		t.Fatalf("transient resume phase emitted %+v", facts)
	}
	if facts := record(6*time.Second, "Running", "healthy", true); len(facts) != 1 || facts[0].Type != EventFactServiceResumed {
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
