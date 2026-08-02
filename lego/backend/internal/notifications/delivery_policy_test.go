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
	"errors"
	"testing"
	"time"
)

func boolPointer(value bool) *bool { return &value }

func urgencyPointer(value DeliveryUrgency) *DeliveryUrgency { return &value }

func policyAt(zone string) DeliveryPolicy {
	return DeliveryPolicy{
		Channels: map[DeliveryChannel]DeliveryChannelPolicy{
			DeliveryChannelEmail: {
				Enabled: true,
				Events: map[DeliveryEvent]bool{
					DeliveryEventDeployFailed:    true,
					DeliveryEventDeploySucceeded: false,
				},
				MinimumUrgency: DeliveryUrgencyRoutine,
			},
			DeliveryChannelPush: {
				Enabled: true,
				Events: map[DeliveryEvent]bool{
					DeliveryEventDeployFailed: true,
					DeliveryEventServerFailed: true,
				},
				MinimumUrgency: DeliveryUrgencyImportant,
			},
		},
		Schedule: DeliverySchedule{TimeZone: zone, MaxDeferral: 24 * time.Hour},
	}
}

func deliveryInput(channel DeliveryChannel, event DeliveryEvent, urgency DeliveryUrgency) DeliveryInput {
	return DeliveryInput{
		Channel: channel, Event: event, Urgency: urgency,
		WorkspaceID: "tea-one", EventWorkspaceID: "tea-one", Subject: "user-one",
		WorkspaceRole: DeliveryRoleDeveloper, ServiceID: "srv-one",
	}
}

func evaluatorAt(now time.Time) DeliveryPolicyEvaluator {
	return DeliveryPolicyEvaluator{Now: func() time.Time { return now }}
}

func TestDeliveryPolicyDurableEventVocabulary(t *testing.T) {
	want := []DeliveryEvent{
		"deploy_started", "deploy_succeeded", "deploy_failed", "server_failed", "cron_failed",
	}
	got := []DeliveryEvent{
		DeliveryEventDeployStarted, DeliveryEventDeploySucceeded, DeliveryEventDeployFailed,
		DeliveryEventServerFailed, DeliveryEventCronFailed,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDeliveryPolicyChannelEventUrgencyAndServiceOverrides(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		mutate func(*DeliveryPolicy, *DeliveryInput)
		want   DeliveryReason
	}{
		{
			name: "channel disabled even for critical",
			mutate: func(policy *DeliveryPolicy, input *DeliveryInput) {
				policy.Channels[DeliveryChannelPush] = DeliveryChannelPolicy{
					Enabled: false, Events: map[DeliveryEvent]bool{DeliveryEventDeployFailed: true}, MinimumUrgency: DeliveryUrgencyRoutine,
				}
				input.Urgency = DeliveryUrgencyCritical
			},
			want: DeliveryReasonChannelDisabled,
		},
		{
			name: "channel event filter",
			mutate: func(_ *DeliveryPolicy, input *DeliveryInput) {
				input.Event = DeliveryEventDeploySucceeded
			},
			want: DeliveryReasonEventFiltered,
		},
		{
			name: "channel minimum urgency",
			mutate: func(_ *DeliveryPolicy, input *DeliveryInput) {
				input.Urgency = DeliveryUrgencyRoutine
			},
			want: DeliveryReasonBelowUrgency,
		},
		{
			name: "service disables inherited channel",
			mutate: func(policy *DeliveryPolicy, _ *DeliveryInput) {
				policy.Services = map[string]DeliveryServiceOverride{"srv-one": {Channels: map[DeliveryChannel]DeliveryChannelOverride{
					DeliveryChannelPush: {Enabled: boolPointer(false)},
				}}}
			},
			want: DeliveryReasonChannelDisabled,
		},
		{
			name: "service disables one inherited event",
			mutate: func(policy *DeliveryPolicy, _ *DeliveryInput) {
				policy.Services = map[string]DeliveryServiceOverride{"srv-one": {Channels: map[DeliveryChannel]DeliveryChannelOverride{
					DeliveryChannelPush: {Events: map[DeliveryEvent]bool{DeliveryEventDeployFailed: false}},
				}}}
			},
			want: DeliveryReasonEventFiltered,
		},
		{
			name: "service enables event without changing member base",
			mutate: func(policy *DeliveryPolicy, input *DeliveryInput) {
				input.Event = DeliveryEventDeploySucceeded
				policy.Services = map[string]DeliveryServiceOverride{"srv-one": {Channels: map[DeliveryChannel]DeliveryChannelOverride{
					DeliveryChannelPush: {
						Events:         map[DeliveryEvent]bool{DeliveryEventDeploySucceeded: true},
						MinimumUrgency: urgencyPointer(DeliveryUrgencyRoutine),
					},
				}}}
			},
			want: DeliveryReasonReady,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := policyAt("UTC")
			input := deliveryInput(DeliveryChannelPush, DeliveryEventDeployFailed, DeliveryUrgencyImportant)
			tc.mutate(&policy, &input)
			got, err := evaluatorAt(now).Evaluate(policy, input)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got.Reason != tc.want {
				t.Fatalf("reason = %q, want %q (decision %+v)", got.Reason, tc.want, got)
			}
		})
	}
}

func TestDeliveryPolicyWorkingAndQuietHoursBoundaries(t *testing.T) {
	policy := policyAt("America/Los_Angeles")
	policy.Schedule.WorkingHours = []DeliveryClockRange{{
		Weekdays: []time.Weekday{time.Monday}, Start: "09:00", End: "17:00",
	}}
	policy.Schedule.QuietHours = []DeliveryClockRange{{
		Weekdays: []time.Weekday{time.Monday}, Start: "12:00", End: "13:00",
	}}
	input := deliveryInput(DeliveryChannelPush, DeliveryEventDeployFailed, DeliveryUrgencyImportant)
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("working start is inclusive", func(t *testing.T) {
		now := time.Date(2026, time.August, 3, 9, 0, 0, 0, location)
		got, err := evaluatorAt(now).Evaluate(policy, input)
		if err != nil || got.Disposition != DeliverySend || !got.DeliverAt.Equal(now) {
			t.Fatalf("decision = %+v, error = %v", got, err)
		}
	})
	t.Run("quiet start defers to its exclusive end", func(t *testing.T) {
		now := time.Date(2026, time.August, 3, 12, 0, 20, 0, location)
		got, err := evaluatorAt(now).Evaluate(policy, input)
		want := time.Date(2026, time.August, 3, 13, 0, 0, 0, location)
		if err != nil || got.Disposition != DeliveryDefer || !got.DeliverAt.Equal(want) {
			t.Fatalf("decision = %+v, want deferred to %s, error = %v", got, want, err)
		}
	})
	t.Run("working end defers to next working day within bound", func(t *testing.T) {
		policy := policy
		policy.Schedule.MaxDeferral = 7 * 24 * time.Hour
		now := time.Date(2026, time.August, 3, 17, 0, 0, 0, location)
		got, err := evaluatorAt(now).Evaluate(policy, input)
		want := time.Date(2026, time.August, 10, 9, 0, 0, 0, location)
		if err != nil || got.Disposition != DeliveryDefer || !got.DeliverAt.Equal(want) {
			t.Fatalf("decision = %+v, want deferred to %s, error = %v", got, want, err)
		}
	})
}

func TestDeliveryPolicyDSTGapAndFoldUseRealInstants(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	input := deliveryInput(DeliveryChannelPush, DeliveryEventDeployFailed, DeliveryUrgencyImportant)

	t.Run("spring gap selects first real minute in working range", func(t *testing.T) {
		policy := policyAt("America/Los_Angeles")
		policy.Schedule.WorkingHours = []DeliveryClockRange{{
			Weekdays: []time.Weekday{time.Sunday}, Start: "02:30", End: "03:30",
		}}
		now := time.Date(2026, time.March, 8, 1, 59, 30, 0, location)
		got, err := evaluatorAt(now).Evaluate(policy, input)
		want := time.Date(2026, time.March, 8, 3, 0, 0, 0, location)
		if err != nil || got.Disposition != DeliveryDefer || !got.DeliverAt.Equal(want) {
			t.Fatalf("decision = %+v, want %s, error = %v", got, want, err)
		}
	})

	t.Run("fall fold remains quiet across both one oclock hours", func(t *testing.T) {
		policy := policyAt("America/Los_Angeles")
		policy.Schedule.QuietHours = []DeliveryClockRange{{
			Weekdays: []time.Weekday{time.Sunday}, Start: "01:00", End: "02:00",
		}}
		// Construct the first 01:15 occurrence by its unambiguous UTC instant.
		now := time.Date(2026, time.November, 1, 8, 15, 0, 0, time.UTC)
		got, err := evaluatorAt(now).Evaluate(policy, input)
		want := time.Date(2026, time.November, 1, 10, 0, 0, 0, time.UTC)
		if err != nil || got.Disposition != DeliveryDefer || !got.DeliverAt.Equal(want) {
			t.Fatalf("decision = %+v, want %s (%s), error = %v", got, want, want.In(location), err)
		}
	})
}

func TestDeliveryPolicyTimezoneChangesApplyWithoutStaleLocation(t *testing.T) {
	now := time.Date(2026, time.August, 3, 7, 30, 0, 0, time.UTC)
	input := deliveryInput(DeliveryChannelPush, DeliveryEventDeployFailed, DeliveryUrgencyImportant)
	quiet := []DeliveryClockRange{{
		Weekdays: []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday},
		Start:    "00:00", End: "02:00",
	}}

	losAngeles := policyAt("America/Los_Angeles")
	losAngeles.Schedule.QuietHours = quiet
	gotLA, err := evaluatorAt(now).Evaluate(losAngeles, input)
	if err != nil || gotLA.Disposition != DeliveryDefer {
		t.Fatalf("Los Angeles decision = %+v, error = %v", gotLA, err)
	}
	utc := policyAt("UTC")
	utc.Schedule.QuietHours = quiet
	gotUTC, err := evaluatorAt(now).Evaluate(utc, input)
	if err != nil || gotUTC.Disposition != DeliverySend {
		t.Fatalf("UTC decision = %+v, error = %v", gotUTC, err)
	}
}

func TestDeliveryPolicyCriticalBypassesScheduleButNotPreferences(t *testing.T) {
	now := time.Date(2026, time.August, 3, 23, 0, 0, 0, time.UTC)
	policy := policyAt("UTC")
	policy.Schedule.QuietHours = []DeliveryClockRange{{
		Weekdays: []time.Weekday{time.Monday}, Start: "22:00", End: "08:00",
	}}
	input := deliveryInput(DeliveryChannelPush, DeliveryEventServerFailed, DeliveryUrgencyCritical)
	got, err := evaluatorAt(now).Evaluate(policy, input)
	if err != nil || got.Disposition != DeliverySend || got.Reason != DeliveryReasonCriticalBypass || !got.BypassedSchedule {
		t.Fatalf("critical decision = %+v, error = %v", got, err)
	}
	if got.OSCriticalAlert {
		t.Fatal("policy must not claim an OS critical-alert entitlement")
	}

	channel := policy.Channels[DeliveryChannelPush]
	channel.Enabled = false
	policy.Channels[DeliveryChannelPush] = channel
	got, err = evaluatorAt(now).Evaluate(policy, input)
	if err != nil || got.Reason != DeliveryReasonChannelDisabled {
		t.Fatalf("disabled critical decision = %+v, error = %v", got, err)
	}
}

func TestDeliveryPolicyWorkspaceRoleInputs(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	policy := policyAt("UTC")
	input := deliveryInput(DeliveryChannelPush, DeliveryEventDeployFailed, DeliveryUrgencyImportant)

	input.WorkspaceRole = DeliveryRoleViewer
	got, err := evaluatorAt(now).Evaluate(policy, input)
	if err != nil || got.Disposition != DeliverySend {
		t.Fatalf("existing all-member semantics = %+v, error = %v", got, err)
	}
	input.EligibleRoles = []DeliveryWorkspaceRole{DeliveryRoleDeveloper, DeliveryRoleAdmin}
	got, err = evaluatorAt(now).Evaluate(policy, input)
	if err != nil || got.Reason != DeliveryReasonRoleIneligible {
		t.Fatalf("restricted role decision = %+v, error = %v", got, err)
	}
	input.EligibleRoles = []DeliveryWorkspaceRole{}
	got, err = evaluatorAt(now).Evaluate(policy, input)
	if err != nil || got.Reason != DeliveryReasonRoleIneligible {
		t.Fatalf("explicit empty role set decision = %+v, error = %v", got, err)
	}
	input.EligibleRoles = []DeliveryWorkspaceRole{DeliveryRoleDeveloper, DeliveryRoleAdmin}
	input.WorkspaceRole = DeliveryRoleAdmin
	got, err = evaluatorAt(now).Evaluate(policy, input)
	if err != nil || got.Disposition != DeliverySend {
		t.Fatalf("eligible role decision = %+v, error = %v", got, err)
	}
	input.EventWorkspaceID = "tea-other"
	got, err = evaluatorAt(now).Evaluate(policy, input)
	if err != nil || got.Reason != DeliveryReasonWorkspaceMismatch {
		t.Fatalf("cross-workspace decision = %+v, error = %v", got, err)
	}
}

func TestDeliveryPolicyBoundsDeferralAndFailsClosed(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	input := deliveryInput(DeliveryChannelPush, DeliveryEventDeployFailed, DeliveryUrgencyImportant)

	t.Run("no allowed instant in bound drops", func(t *testing.T) {
		policy := policyAt("UTC")
		policy.Schedule.WorkingHours = []DeliveryClockRange{{
			Weekdays: []time.Weekday{time.Tuesday}, Start: "09:00", End: "10:00",
		}}
		policy.Schedule.MaxDeferral = 30 * time.Minute
		got, err := evaluatorAt(now).Evaluate(policy, input)
		if err != nil || got.Disposition != DeliveryDrop || got.Reason != DeliveryReasonDeferralLimit || !got.DeliverAt.IsZero() {
			t.Fatalf("decision = %+v, error = %v", got, err)
		}
	})

	cases := []struct {
		name   string
		mutate func(*DeliveryPolicy, *DeliveryInput, *DeliveryPolicyEvaluator)
	}{
		{"nil clock", func(_ *DeliveryPolicy, _ *DeliveryInput, evaluator *DeliveryPolicyEvaluator) { evaluator.Now = nil }},
		{"bad timezone", func(policy *DeliveryPolicy, _ *DeliveryInput, _ *DeliveryPolicyEvaluator) {
			policy.Schedule.TimeZone = "PST8-ish"
		}},
		{"local timezone alias", func(policy *DeliveryPolicy, _ *DeliveryInput, _ *DeliveryPolicyEvaluator) {
			policy.Schedule.TimeZone = "Local"
		}},
		{"bad clock", func(policy *DeliveryPolicy, _ *DeliveryInput, _ *DeliveryPolicyEvaluator) {
			policy.Schedule.QuietHours = []DeliveryClockRange{{Weekdays: []time.Weekday{time.Monday}, Start: "25:00", End: "08:00"}}
		}},
		{"equal clock boundary", func(policy *DeliveryPolicy, _ *DeliveryInput, _ *DeliveryPolicyEvaluator) {
			policy.Schedule.QuietHours = []DeliveryClockRange{{Weekdays: []time.Weekday{time.Monday}, Start: "08:00", End: "08:00"}}
		}},
		{"unbounded deferral", func(policy *DeliveryPolicy, _ *DeliveryInput, _ *DeliveryPolicyEvaluator) {
			policy.Schedule.MaxDeferral = MaximumDeliveryDeferral + time.Minute
		}},
		{"unknown stored event", func(policy *DeliveryPolicy, _ *DeliveryInput, _ *DeliveryPolicyEvaluator) {
			policy.Channels[DeliveryChannelPush].Events[DeliveryEvent("future_event")] = true
		}},
		{"unknown input event", func(_ *DeliveryPolicy, input *DeliveryInput, _ *DeliveryPolicyEvaluator) {
			input.Event = DeliveryEvent("future_event")
		}},
		{"unknown role", func(_ *DeliveryPolicy, input *DeliveryInput, _ *DeliveryPolicyEvaluator) {
			input.WorkspaceRole = DeliveryWorkspaceRole("owner")
		}},
		{"missing identity", func(_ *DeliveryPolicy, input *DeliveryInput, _ *DeliveryPolicyEvaluator) { input.Subject = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := policyAt("UTC")
			input := input
			evaluator := evaluatorAt(now)
			tc.mutate(&policy, &input, &evaluator)
			got, err := evaluator.Evaluate(policy, input)
			if !errors.Is(err, ErrInvalidDeliveryPolicy) {
				t.Fatalf("error = %v, want ErrInvalidDeliveryPolicy", err)
			}
			if got.Disposition != DeliveryDrop || got.Reason != DeliveryReasonInvalidPolicy || !got.DeliverAt.IsZero() {
				t.Fatalf("fail-closed decision = %+v", got)
			}
		})
	}
}
