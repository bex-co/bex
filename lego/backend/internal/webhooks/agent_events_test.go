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


package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeAgentOutcomes struct {
	reads []string
	err   error
}

func (f *fakeAgentOutcomes) AgentSessionTurnOutcome(_ context.Context, sessionID string, turn int, phase string) (string, json.RawMessage, error) {
	f.reads = append(f.reads, sessionID)
	if f.err != nil {
		return "", nil, f.err
	}
	state := "succeeded"
	if phase == "failed" {
		state = "failed"
	}
	return state, json.RawMessage(`{"state":"` + state + `","turn":` + string(rune('0'+turn)) + `}`), nil
}

func agentRow(session string, turn int, phase string, at time.Time) store.WebhookEventRow {
	return store.WebhookEventRow{
		Key: "agent:" + session + ":" + string(rune('0'+turn)) + ":" + phase, At: at, TenantID: "tea-a",
		ServiceID: session, Source: store.EventSourceAgent, Phase: phase, Turn: turn,
	}
}

// ADR094 D9: a subscribed endpoint receives one signed event per turn outcome
// whose data is {id, state, outcome} (the session id, not an event id) and
// carries no summary; an endpoint that did not ask for it, another workspace's
// endpoint, and a workspace without endpoints get nothing.
func TestAgentSessionTurnEventsReachOnlySubscribedEndpoints(t *testing.T) {
	st := newFakeWorkerStore()
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	eventAt := now.Add(-time.Minute)
	st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
	st.events = []store.WebhookEventRow{
		agentRow("ags-one", 1, "completed", eventAt),
		agentRow("ags-one", 2, "failed", eventAt.Add(time.Second)),
	}
	succeeded := endpoint("whk-ok", "tea-a", "https://a.example/ok", "whsec_x")
	succeeded.EventTypes = []string{TypeAgentSessionTurnSucceeded}
	all := endpoint("whk-all", "tea-a", "https://a.example/all", "whsec_x")
	other := endpoint("whk-deploys", "tea-a", "https://a.example/deploys", "whsec_x")
	other.EventTypes = []string{TypeDeployEnded}
	elsewhere := endpoint("whk-b", "tea-b", "https://b.example/ok", "whsec_x")
	elsewhere.EventTypes = []string{TypeAgentSessionTurnSucceeded, TypeAgentSessionTurnFailed}
	st.endpoints = []store.WebhookEndpoint{succeeded, all, other, elsewhere}
	outcomes := &fakeAgentOutcomes{}
	w := &Worker{Store: st, Clock: func() time.Time { return now }, AgentOutcomes: outcomes}
	if err := dispatchStored(w); err != nil {
		t.Fatal(err)
	}

	byEndpoint := map[string][]string{}
	for _, d := range st.queue {
		byEndpoint[d.EndpointID] = append(byEndpoint[d.EndpointID], d.EventType)
		var p struct {
			Type string         `json:"type"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal([]byte(d.Payload), &p); err != nil {
			t.Fatal(err)
		}
		outcome, _ := p.Data["outcome"].(map[string]any)
		if p.Type != d.EventType || p.Data["id"] != "ags-one" || p.Data["state"] == nil || outcome == nil || outcome["summary"] != nil || len(p.Data) != 3 {
			t.Errorf("payload = %s, want {id, state, outcome} for ags-one", d.Payload)
		}
		if d.EventID == "" || d.EventID == "ags-one" {
			t.Errorf("webhook-id = %q, want a derived event id distinct per turn event", d.EventID)
		}
	}
	for endpointID, want := range map[string][]string{
		"whk-ok":  {TypeAgentSessionTurnSucceeded},
		"whk-all": {TypeAgentSessionTurnSucceeded, TypeAgentSessionTurnFailed},
	} {
		got := byEndpoint[endpointID]
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s got %v, want %v", endpointID, got, want)
		}
	}
	for _, endpointID := range []string{"whk-deploys", "whk-b"} {
		if len(byEndpoint[endpointID]) != 0 {
			t.Errorf("%s got %v, want nothing", endpointID, byEndpoint[endpointID])
		}
	}
	if st.wmKey != "agent:ags-one:2:failed" {
		t.Errorf("watermark = %q, want the last agent row", st.wmKey)
	}
}

// Nobody subscribed: the outcome is never read, and without a reader the
// events are dropped rather than sent without an outcome.
func TestAgentSessionTurnEventsReadTheOutcomeOnlyForSubscribers(t *testing.T) {
	st := newFakeWorkerStore()
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	eventAt := now.Add(-time.Minute)
	st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
	st.events = []store.WebhookEventRow{agentRow("ags-one", 1, "completed", eventAt)}
	narrow := endpoint("whk-d", "tea-a", "https://a.example/d", "whsec_x")
	narrow.EventTypes = []string{TypeDeployEnded}
	st.endpoints = []store.WebhookEndpoint{narrow}
	outcomes := &fakeAgentOutcomes{}
	if err := dispatchStored(&Worker{Store: st, Clock: func() time.Time { return now }, AgentOutcomes: outcomes}); err != nil {
		t.Fatal(err)
	}
	if len(outcomes.reads) != 0 || len(st.queue) != 0 {
		t.Fatalf("reads=%v deliveries=%d, want neither", outcomes.reads, len(st.queue))
	}

	st = newFakeWorkerStore()
	st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
	st.events = []store.WebhookEventRow{agentRow("ags-one", 1, "completed", eventAt)}
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-all", "tea-a", "https://a.example/all", "whsec_x")}
	if err := dispatchStored(&Worker{Store: st, Clock: func() time.Time { return now }}); err != nil || len(st.queue) != 0 {
		t.Fatalf("no reader: err=%v deliveries=%d, want the event dropped", err, len(st.queue))
	}
}

// A session deleted before delivery drops its event; any other read failure
// fails the pass so the watermark stays and the event is retried.
func TestAgentSessionTurnEventOutcomeFailures(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	eventAt := now.Add(-time.Minute)
	run := func(readErr error) (*fakeWorkerStore, error) {
		st := newFakeWorkerStore()
		st.wmSeeded, st.wmAt = true, eventAt.Add(-time.Hour)
		st.events = []store.WebhookEventRow{agentRow("ags-one", 1, "failed", eventAt)}
		st.endpoints = []store.WebhookEndpoint{endpoint("whk-all", "tea-a", "https://a.example/all", "whsec_x")}
		w := &Worker{Store: st, Clock: func() time.Time { return now }, AgentOutcomes: &fakeAgentOutcomes{err: readErr}}
		return st, dispatchStored(w)
	}
	if st, err := run(store.ErrNotFound); err != nil || len(st.queue) != 0 {
		t.Fatalf("deleted session: err=%v deliveries=%d, want a clean drop", err, len(st.queue))
	}
	if st, err := run(errors.New("db down")); err == nil || len(st.queue) != 0 || st.wmKey != "" {
		t.Fatalf("read failure: err=%v deliveries=%d watermark=%q, want a failed pass that keeps the watermark", err, len(st.queue), st.wmKey)
	}
}

func TestAgentSessionEventTypesAreAdvertisedAndNeverProjectedAsFeedEvents(t *testing.T) {
	for _, name := range []string{TypeAgentSessionTurnSucceeded, TypeAgentSessionTurnFailed, TypeAgentSessionNeedsInput} {
		if !slices.Contains(EventTypes, name) {
			t.Errorf("%s is not in the subscribable vocabulary", name)
		}
		if got, err := normalizeEventTypes([]string{name}); err != nil || !slices.Equal(got, []string{name}) {
			t.Errorf("normalizeEventTypes(%s) = %v, %v", name, got, err)
		}
	}
	// The agent source has its own fan-out; the generic projector skips it.
	if eventType, _, ok := project(agentRow("ags-one", 1, "completed", time.Now())); ok {
		t.Errorf("project() claimed an agent row as %q", eventType)
	}
}
