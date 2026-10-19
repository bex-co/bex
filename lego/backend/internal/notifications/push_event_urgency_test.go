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

package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestPushEventUrgencyCoversEverySelectableEvent pins the published table
// (w4/m231): one valid urgency per selectable event, in the settings' order,
// with exactly the four Routine events a default Important minimum drops.
func TestPushEventUrgencyCoversEverySelectableEvent(t *testing.T) {
	if len(pushEventUrgency) != len(orderedDeliveryEvents) {
		t.Fatalf("pushEventUrgency has %d rows, want one per selectable event (%d)", len(pushEventUrgency), len(orderedDeliveryEvents))
	}
	var routine []DeliveryEvent
	for i, event := range orderedDeliveryEvents {
		urgency, ok := pushEventUrgency[event]
		if !ok || !validDeliveryUrgency(urgency) {
			t.Fatalf("%s has urgency %q, want a valid one", event, urgency)
		}
		if got := pushEventUrgencies[i]; got != (PushEventUrgencyView{Event: event, Urgency: urgency, Sent: !unsentPushEvents[event]}) {
			t.Fatalf("pushEventUrgencies[%d] = %+v, want %s=%s sent=%v", i, got, event, urgency, !unsentPushEvents[event])
		}
		if urgency == DeliveryUrgencyRoutine {
			routine = append(routine, event)
		}
	}
	want := []DeliveryEvent{DeliveryEventDeployStarted, DeliveryEventDeploySucceeded, DeliveryEventServiceSuspended, DeliveryEventServiceResumed}
	if !reflect.DeepEqual(routine, want) {
		t.Fatalf("Routine events = %v, want %v", routine, want)
	}
}

// TestProjectedPushUrgencyIsPublished drives every projection and checks it
// pushes at the urgency the settings read publishes, so the panel's "won't be
// pushed" notice and the worker's drop decision cannot disagree. Every
// selectable event must be projected exactly when the settings read publishes
// it as sent; a new event without a projection fails here (w4/349).
func TestProjectedPushUrgencyIsPublished(t *testing.T) {
	fact := func(factType string) store.WebhookEventRow {
		return store.WebhookEventRow{Source: store.EventSourceFact, FactType: factType, ServiceName: "api"}
	}
	deploy := func(phase, status string) store.WebhookEventRow {
		return store.WebhookEventRow{Source: store.EventSourceDeploy, Phase: phase, Status: status, ServiceName: "api"}
	}
	type projection struct {
		push projectedPush
		ok   bool
	}
	project := func(row store.WebhookEventRow, factStatus string) projection {
		push, ok := projectPushEvent(row, "srv-c185th5c2rvvnhbfiltg", factStatus)
		return projection{push, ok}
	}
	agent := func(s store.AgentSession) projection {
		push, ok := projectAgentSessionPush(s)
		return projection{push, ok}
	}
	projections := []projection{
		project(deploy(store.EventPhaseStarted, ""), ""),
		project(deploy(store.EventPhaseEnded, store.DeployLive), ""),
		project(deploy(store.EventPhaseEnded, store.DeployBuildFailed), ""),
		project(fact(string(store.EventFactServerFailed)), ""),
		project(fact(string(store.EventFactServerAvailable)), ""),
		project(fact(string(store.EventFactServiceSuspended)), ""),
		project(fact(string(store.EventFactServiceResumed)), ""),
		project(fact(string(store.EventFactJobRunEnded)), store.EventStatusFailed),
		project(fact(string(store.DatastoreFactPostgresUnavailable)), ""),
		project(fact(string(store.DatastoreFactPostgresAvailable)), ""),
		project(fact(string(store.DatastoreFactKeyValueUnhealthy)), ""),
		project(fact(string(store.DatastoreFactKeyValueAvailable)), ""),
		project(fact(string(store.DatastoreFactPostgresBackupFailed)), ""),
		project(fact(string(store.DatastoreFactPostgresRestoreFailed)), ""),
		project(fact(string(store.DatastoreFactPostgresUpgradeFailed)), ""),
		agent(store.AgentSession{Phase: "completed", PRURL: "https://github.com/o/r/pull/1"}),
		agent(store.AgentSession{Phase: "completed"}),
		agent(store.AgentSession{Phase: "failed"}),
	}
	projected := map[DeliveryEvent]bool{}
	for _, p := range projections {
		if !p.ok {
			t.Fatalf("projection unexpectedly declined: %+v", p.push)
		}
		event := DeliveryEvent(p.push.event)
		if published := pushEventUrgency[event]; p.push.urgency != string(published) {
			t.Fatalf("%s pushes at %q, but the settings read publishes %q", event, p.push.urgency, published)
		}
		projected[event] = true
	}
	for _, view := range pushEventUrgencies {
		if projected[view.Event] != view.Sent {
			t.Errorf("%s: projected=%v but published sent=%v", view.Event, projected[view.Event], view.Sent)
		}
	}
}

// TestPushEventUrgenciesSurfaceParity reads the table over REST and GraphQL:
// the same rows, in the same order, under each surface's own spelling.
func TestPushEventUrgenciesSurfaceParity(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice"})
	svc := newTestService(newFakeStore(), fakeWorkspace{"alice": "tea-a"}, nil, nil)

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/notification-settings/push/event-urgencies", nil).WithContext(ctx))
	if rec.Code != http.StatusOK {
		t.Fatalf("REST status = %d: %s", rec.Code, rec.Body)
	}
	var rest []PushEventUrgencyView
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil {
		t.Fatalf("REST decode: %v", err)
	}
	if !reflect.DeepEqual(rest, pushEventUrgencies) {
		t.Fatalf("REST = %+v, want %+v", rest, pushEventUrgencies)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("GraphQL schema: %v", err)
	}
	result := graphql.Do(graphql.Params{Schema: schema, RequestString: `{ pushEventUrgencies { event urgency sent } }`, Context: ctx})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", result.Errors)
	}
	raw, _ := json.Marshal(result.Data)
	var gql struct {
		Rows []struct {
			Event, Urgency string
			Sent           bool
		} `json:"pushEventUrgencies"`
	}
	if err := json.Unmarshal(raw, &gql); err != nil {
		t.Fatalf("GraphQL decode: %v", err)
	}
	enumValue := func(enum *graphql.Enum, name string) string {
		for _, v := range enum.Values() {
			if v.Name == name {
				return v.Value.(string)
			}
		}
		return ""
	}
	if len(gql.Rows) != len(rest) {
		t.Fatalf("GraphQL has %d rows, REST %d", len(gql.Rows), len(rest))
	}
	for i, row := range gql.Rows {
		got := PushEventUrgencyView{
			Event:   DeliveryEvent(enumValue(pushDeliveryEventGQLEnum, row.Event)),
			Urgency: DeliveryUrgency(enumValue(pushUrgencyGQLEnum, row.Urgency)),
			Sent:    row.Sent,
		}
		if got != rest[i] {
			t.Fatalf("GraphQL row %d = %+v, REST %+v", i, got, rest[i])
		}
	}
}

// TestPublishedUrgencyPredictsTheDeliveryDecision feeds every selected event,
// at its published urgency, through the worker's own evaluator under each
// minimum. It drops exactly the events ranked below the minimum — the rule the
// dashboard uses to flag a selection — and delivers the rest, so the panel's
// notice and the real decision cannot disagree (w4/m231). The evaluator is
// unchanged; this pins its decisions to the published table.
func TestPublishedUrgencyPredictsTheDeliveryDecision(t *testing.T) {
	now := time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)
	// The dashboard's order (pushUrgencies), not the evaluator's urgencyRank.
	rank := map[DeliveryUrgency]int{DeliveryUrgencyRoutine: 0, DeliveryUrgencyImportant: 1, DeliveryUrgencyCritical: 2}
	for _, minimum := range []DeliveryUrgency{DeliveryUrgencyRoutine, DeliveryUrgencyImportant, DeliveryUrgencyCritical} {
		view := clonePushSettings(defaultPushSettings)
		view.Events = append([]DeliveryEvent(nil), orderedDeliveryEvents...)
		view.MinimumUrgency = minimum
		policy, err := view.deliveryPolicy()
		if err != nil {
			t.Fatalf("deliveryPolicy(): %v", err)
		}
		for _, row := range pushEventUrgencies {
			input := deliveryInput(DeliveryChannelPush, row.Event, row.Urgency)
			got, err := evaluatorAt(now).Evaluate(policy, input)
			if err != nil {
				t.Fatalf("%s under %s: Evaluate: %v", row.Event, minimum, err)
			}
			want := DeliveryReasonReady
			if rank[row.Urgency] < rank[minimum] {
				want = DeliveryReasonBelowUrgency
			}
			if got.Reason != want {
				t.Errorf("%s (%s) under minimum %s: reason %q, want %q", row.Event, row.Urgency, minimum, got.Reason, want)
			}
		}
	}
}
