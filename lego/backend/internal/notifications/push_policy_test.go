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
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const testPushServiceID = "srv-c185th5c2rvvnhbfiltg"

func pushContext(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "oauth2"})
}

func TestPushSettingsDefaults(t *testing.T) {
	svc := newTestService(newFakeStore(), fakeWorkspace{"alice": "tea-a"}, nil, nil)
	got, err := svc.GetPushSettings(pushContext("alice"))
	if err != nil {
		t.Fatal(err)
	}
	want := PushSettingsView{
		Enabled: true,
		Events: []DeliveryEvent{
			DeliveryEventDeployFailed, DeliveryEventServerFailed, DeliveryEventCronFailed,
			DeliveryEventAgentPRReady, DeliveryEventAgentFailed,
		},
		MinimumUrgency: DeliveryUrgencyImportant, TimeZone: "UTC",
		WorkingHours: []PushClockRangeView{}, QuietHours: []PushClockRangeView{},
		MaxDeferralSeconds: 8 * 60 * 60, ServiceOverrides: []PushServiceOverrideView{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults = %#v, want %#v", got, want)
	}
	got.Events[0] = DeliveryEventDeployStarted
	again, _ := svc.GetPushSettings(pushContext("alice"))
	if again.Events[0] != DeliveryEventDeployFailed {
		t.Fatal("default settings slice was shared across callers")
	}
}

func TestPushSettingsRoundTripPreservesEmailAndNormalizes(t *testing.T) {
	st := newFakeStore()
	st.rows[[2]string{"tea-a", "alice"}] = store.NotificationSettings{
		ID: "ntf-fake", TenantID: "tea-a", Subject: "alice",
		DeployStarted: true, DeploySucceeded: true, DeployFailed: false,
	}
	svc := newTestService(st, fakeWorkspace{"alice": "tea-a"}, nil, nil)
	disabled := false
	events := []DeliveryEvent{DeliveryEventDeployFailed, DeliveryEventDeployFailed}
	requested := PushSettingsView{
		Enabled:        true,
		Events:         []DeliveryEvent{DeliveryEventCronFailed, DeliveryEventDeployFailed, DeliveryEventCronFailed},
		MinimumUrgency: DeliveryUrgencyRoutine, TimeZone: " America/New_York ",
		WorkingHours:       []PushClockRangeView{{Weekdays: []string{"FRIDAY", "monday", "friday"}, Start: " 09:00 ", End: "17:00"}},
		QuietHours:         []PushClockRangeView{{Weekdays: []string{"sunday"}, Start: "22:00", End: "06:00"}},
		MaxDeferralSeconds: 3600,
		ServiceOverrides: []PushServiceOverrideView{{
			ServiceID: testPushServiceID, Enabled: &disabled, Events: &events,
		}},
	}
	got, err := svc.UpdatePushSettings(pushContext("alice"), requested)
	if err != nil {
		t.Fatal(err)
	}
	if got.TimeZone != "America/New_York" || !reflect.DeepEqual(got.Events, []DeliveryEvent{DeliveryEventDeployFailed, DeliveryEventCronFailed}) {
		t.Fatalf("normalized settings = %#v", got)
	}
	if !reflect.DeepEqual(got.WorkingHours[0].Weekdays, []string{"monday", "friday"}) || got.WorkingHours[0].Start != "09:00" {
		t.Fatalf("normalized range = %#v", got.WorkingHours[0])
	}
	if got.ServiceOverrides[0].Events == nil || !reflect.DeepEqual(*got.ServiceOverrides[0].Events, []DeliveryEvent{DeliveryEventDeployFailed}) {
		t.Fatalf("normalized override = %#v", got.ServiceOverrides[0])
	}
	row := st.rows[[2]string{"tea-a", "alice"}]
	if !row.DeployStarted || !row.DeploySucceeded || row.DeployFailed {
		t.Fatalf("push update changed email preferences: %#v", row)
	}
	roundTrip, err := svc.GetPushSettings(pushContext("alice"))
	if err != nil || !reflect.DeepEqual(roundTrip, got) {
		t.Fatalf("round trip = %#v (%v), want %#v", roundTrip, err, got)
	}
}

func TestPushSettingsValidation(t *testing.T) {
	base := clonePushSettings(defaultPushSettings)
	boolValue := true
	urgency := DeliveryUrgency("urgent")
	tests := []struct {
		name   string
		mutate func(*PushSettingsView)
	}{
		{"unknown event", func(v *PushSettingsView) { v.Events = []DeliveryEvent{"marketing"} }},
		{"unknown urgency", func(v *PushSettingsView) { v.MinimumUrgency = "urgent" }},
		{"local timezone", func(v *PushSettingsView) { v.TimeZone = "Local" }},
		{"unknown timezone", func(v *PushSettingsView) { v.TimeZone = "Mars/Olympus" }},
		{"zero deferral", func(v *PushSettingsView) { v.MaxDeferralSeconds = 0 }},
		{"excessive deferral", func(v *PushSettingsView) { v.MaxDeferralSeconds = int(MaximumDeliveryDeferral/time.Second) + 1 }},
		{"bad clock", func(v *PushSettingsView) {
			v.QuietHours = []PushClockRangeView{{Weekdays: []string{"monday"}, Start: "25:00", End: "06:00"}}
		}},
		{"empty weekdays", func(v *PushSettingsView) { v.WorkingHours = []PushClockRangeView{{Start: "09:00", End: "17:00"}} }},
		{"bad service id", func(v *PushSettingsView) {
			v.ServiceOverrides = []PushServiceOverrideView{{ServiceID: "srv-not-opaque", Enabled: &boolValue}}
		}},
		{"empty override", func(v *PushSettingsView) {
			v.ServiceOverrides = []PushServiceOverrideView{{ServiceID: testPushServiceID}}
		}},
		{"bad override urgency", func(v *PushSettingsView) {
			v.ServiceOverrides = []PushServiceOverrideView{{ServiceID: testPushServiceID, MinimumUrgency: &urgency}}
		}},
	}
	svc := newTestService(newFakeStore(), fakeWorkspace{"alice": "tea-a"}, nil, nil)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := clonePushSettings(base)
			test.mutate(&input)
			_, err := svc.UpdatePushSettings(pushContext("alice"), input)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("error = %v, want ErrBadRequest", err)
			}
		})
	}
}

func TestPushSettingsAreCallerScoped(t *testing.T) {
	st := newFakeStore()
	svc := newTestService(st, fakeWorkspace{"alice": "tea-a", "bob": "tea-b"}, nil, nil)
	alice := clonePushSettings(defaultPushSettings)
	alice.Enabled = false
	if _, err := svc.UpdatePushSettings(pushContext("alice"), alice); err != nil {
		t.Fatal(err)
	}
	bob, err := svc.GetPushSettings(pushContext("bob"))
	if err != nil || !bob.Enabled {
		t.Fatalf("bob settings = %#v (%v), want independent defaults", bob, err)
	}
	if _, found := st.rows[[2]string{"tea-b", "alice"}]; found {
		t.Fatal("alice policy escaped into bob's workspace")
	}
}

func TestPushServiceOverrideIsAnExactEventFilter(t *testing.T) {
	empty := []DeliveryEvent{}
	view := clonePushSettings(defaultPushSettings)
	view.ServiceOverrides = []PushServiceOverrideView{{ServiceID: testPushServiceID, Events: &empty}}
	normalized, err := normalizePushSettings(view)
	if err != nil {
		t.Fatal(err)
	}
	policy, _ := normalized.deliveryPolicy()
	evaluator := DeliveryPolicyEvaluator{Now: func() time.Time { return time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC) }}
	baseInput := DeliveryInput{
		Channel: DeliveryChannelPush, Event: DeliveryEventDeployFailed, Urgency: DeliveryUrgencyImportant,
		WorkspaceID: "tea-a", EventWorkspaceID: "tea-a", Subject: "alice", WorkspaceRole: DeliveryRoleViewer,
	}
	baseInput.ServiceID = testPushServiceID
	decision, err := evaluator.Evaluate(policy, baseInput)
	if err != nil || decision.Reason != DeliveryReasonEventFiltered {
		t.Fatalf("overridden service = %#v (%v)", decision, err)
	}
	baseInput.ServiceID = "srv-c185th5c2rvvnhbfilta"
	decision, err = evaluator.Evaluate(policy, baseInput)
	if err != nil || decision.Disposition != DeliverySend {
		t.Fatalf("inheriting service = %#v (%v)", decision, err)
	}
}

func TestPushTimezoneAndDSTRangesProjectToEvaluator(t *testing.T) {
	view := clonePushSettings(defaultPushSettings)
	view.TimeZone = "America/Los_Angeles"
	view.WorkingHours = []PushClockRangeView{{Weekdays: []string{"sunday"}, Start: "01:30", End: "03:30"}}
	normalized, err := normalizePushSettings(view)
	if err != nil {
		t.Fatal(err)
	}
	policy, _ := normalized.deliveryPolicy()
	input := DeliveryInput{
		Channel: DeliveryChannelPush, Event: DeliveryEventDeployFailed, Urgency: DeliveryUrgencyImportant,
		WorkspaceID: "tea-a", EventWorkspaceID: "tea-a", Subject: "alice", WorkspaceRole: DeliveryRoleViewer,
	}
	for _, instant := range []time.Time{
		time.Date(2026, 3, 8, 9, 45, 0, 0, time.UTC),  // 01:45 PST
		time.Date(2026, 3, 8, 10, 15, 0, 0, time.UTC), // 03:15 PDT after the DST gap
	} {
		decision, evaluateErr := (DeliveryPolicyEvaluator{Now: func() time.Time { return instant }}).Evaluate(policy, input)
		if evaluateErr != nil || decision.Disposition != DeliverySend {
			t.Fatalf("at %s decision = %#v (%v)", instant, decision, evaluateErr)
		}
	}
}

func TestPushSettingsRESTAndGraphQLTypedRoundTrip(t *testing.T) {
	ctx := pushContext("alice")
	newService := func() *Service { return newTestService(newFakeStore(), fakeWorkspace{"alice": "tea-a"}, nil, nil) }
	body := `{"enabled":true,"events":["deploy_failed"],"minimumUrgency":"important","timeZone":"UTC","workingHours":[],"quietHours":[],"maxDeferralSeconds":3600,"serviceOverrides":[]}`
	restSvc := newService()
	mux := http.NewServeMux()
	restSvc.RegisterREST(mux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch, "/v1/notification-settings/push", strings.NewReader(body)).WithContext(ctx))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"deploy_failed"`) {
		t.Fatalf("REST = %d %s", recorder.Code, recorder.Body)
	}

	gqlSvc := newService()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: gqlSvc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: gqlSvc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `mutation {
		updatePushNotificationSettings(settings: {
			enabled: true, events: [DEPLOY_FAILED], minimumUrgency: IMPORTANT,
			timeZone: "UTC", workingHours: [{weekdays:[MONDAY],start:"09:00",end:"17:00"}],
			quietHours: [], maxDeferralSeconds: 3600,
			serviceOverrides: [{serviceId:"` + testPushServiceID + `",events:[]}]
		}) { enabled events minimumUrgency timeZone maxDeferralSeconds
			workingHours { weekdays start end } serviceOverrides { serviceId enabled events minimumUrgency } }
	}`})
	if len(result.Errors) != 0 {
		t.Fatalf("GraphQL errors: %v", result.Errors)
	}
	raw, _ := json.Marshal(result.Data)
	if strings.Contains(string(raw), "pushPolicyJson") || !strings.Contains(string(raw), `"DEPLOY_FAILED"`) || !strings.Contains(string(raw), `"MONDAY"`) {
		t.Fatalf("GraphQL typed result = %s", raw)
	}
}

// A policy written while usage_threshold was still selectable must survive the
// event's retirement: reads drop it silently, writes can no longer re-enter it.
func TestPushSettingsStoredRetiredEventIsDropped(t *testing.T) {
	st := newFakeStore()
	overrideEvents := `["deploy_failed","usage_threshold"]`
	stored := `{"enabled":true,"events":["deploy_failed","usage_threshold","cron_failed"],` +
		`"minimumUrgency":"important","timeZone":"UTC","workingHours":[],"quietHours":[],` +
		`"maxDeferralSeconds":3600,"serviceOverrides":[{"serviceId":"` + testPushServiceID + `","events":` + overrideEvents + `}]}`
	st.rows[[2]string{"tea-a", "alice"}] = store.NotificationSettings{
		ID: "ntf-fake", TenantID: "tea-a", Subject: "alice", PushPolicy: json.RawMessage(stored),
	}
	svc := newTestService(st, fakeWorkspace{"alice": "tea-a"}, nil, nil)

	got, err := svc.GetPushSettings(pushContext("alice"))
	if err != nil {
		t.Fatalf("stored policy with a retired event must still read: %v", err)
	}
	if !reflect.DeepEqual(got.Events, []DeliveryEvent{DeliveryEventDeployFailed, DeliveryEventCronFailed}) {
		t.Fatalf("events = %#v, want usage_threshold dropped", got.Events)
	}
	if got.ServiceOverrides[0].Events == nil ||
		!reflect.DeepEqual(*got.ServiceOverrides[0].Events, []DeliveryEvent{DeliveryEventDeployFailed}) {
		t.Fatalf("override events = %#v, want usage_threshold dropped", got.ServiceOverrides[0])
	}

	if _, err := storedPushDeliveryPolicy(json.RawMessage(stored)); err != nil {
		t.Fatalf("worker must still compile a policy carrying a retired event: %v", err)
	}

	write := clonePushSettings(defaultPushSettings)
	write.Events = append(write.Events, DeliveryEvent("usage_threshold"))
	if _, err := svc.UpdatePushSettings(pushContext("alice"), write); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("write error = %v, want ErrBadRequest", err)
	}
}
