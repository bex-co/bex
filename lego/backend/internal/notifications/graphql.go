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
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the notifications GraphQL fragment the dashboard's Settings
// page consumes: `notificationSettings` query + `updateNotificationSettings`
// mutation, both scoped to the caller (no workspaceId arg — same shape as
// `usage`).

var notificationSettingsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "NotificationSettings",
	Fields: graphql.Fields{
		"deployStarted":   &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v SettingsView) any { return v.DeployStarted })},
		"deploySucceeded": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v SettingsView) any { return v.DeploySucceeded })},
		"deployFailed":    &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v SettingsView) any { return v.DeployFailed })},
	},
})

var pushDeliveryEventGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "PushNotificationEvent",
	Values: graphql.EnumValueConfigMap{
		"DEPLOY_STARTED":       &graphql.EnumValueConfig{Value: string(DeliveryEventDeployStarted)},
		"DEPLOY_SUCCEEDED":     &graphql.EnumValueConfig{Value: string(DeliveryEventDeploySucceeded)},
		"DEPLOY_FAILED":        &graphql.EnumValueConfig{Value: string(DeliveryEventDeployFailed)},
		"SERVER_FAILED":        &graphql.EnumValueConfig{Value: string(DeliveryEventServerFailed)},
		"CRON_FAILED":          &graphql.EnumValueConfig{Value: string(DeliveryEventCronFailed)},
		"USAGE_THRESHOLD":      &graphql.EnumValueConfig{Value: string(DeliveryEventUsageThreshold)},
		"AGENT_NEEDS_DECISION": &graphql.EnumValueConfig{Value: string(DeliveryEventAgentNeedsDecision)},
		"AGENT_PR_READY":       &graphql.EnumValueConfig{Value: string(DeliveryEventAgentPRReady)},
	},
})

var pushUrgencyGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "PushNotificationUrgency",
	Values: graphql.EnumValueConfigMap{
		"ROUTINE":   &graphql.EnumValueConfig{Value: string(DeliveryUrgencyRoutine)},
		"IMPORTANT": &graphql.EnumValueConfig{Value: string(DeliveryUrgencyImportant)},
		"CRITICAL":  &graphql.EnumValueConfig{Value: string(DeliveryUrgencyCritical)},
	},
})

var pushWeekdayGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "PushNotificationWeekday",
	Values: graphql.EnumValueConfigMap{
		"SUNDAY": &graphql.EnumValueConfig{Value: "sunday"}, "MONDAY": &graphql.EnumValueConfig{Value: "monday"},
		"TUESDAY": &graphql.EnumValueConfig{Value: "tuesday"}, "WEDNESDAY": &graphql.EnumValueConfig{Value: "wednesday"},
		"THURSDAY": &graphql.EnumValueConfig{Value: "thursday"}, "FRIDAY": &graphql.EnumValueConfig{Value: "friday"},
		"SATURDAY": &graphql.EnumValueConfig{Value: "saturday"},
	},
})

var pushClockRangeGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PushNotificationClockRange",
	Fields: graphql.Fields{
		"weekdays": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushWeekdayGQLEnum))), Resolve: gqlutil.Field(func(v PushClockRangeView) any { return v.Weekdays })},
		"start":    &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushClockRangeView) any { return v.Start })},
		"end":      &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushClockRangeView) any { return v.End })},
	},
})

var pushServiceOverrideGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PushNotificationServiceOverride",
	Fields: graphql.Fields{
		"serviceId": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushServiceOverrideView) any { return v.ServiceID })},
		"enabled": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v PushServiceOverrideView) any {
			if v.Enabled == nil {
				return nil
			}
			return *v.Enabled
		})},
		"events": &graphql.Field{Type: graphql.NewList(graphql.NewNonNull(pushDeliveryEventGQLEnum)), Resolve: gqlutil.Field(func(v PushServiceOverrideView) any {
			if v.Events == nil {
				return nil
			}
			return gqlDeliveryEventOutput(*v.Events)
		})},
		"minimumUrgency": &graphql.Field{Type: pushUrgencyGQLEnum, Resolve: gqlutil.Field(func(v PushServiceOverrideView) any {
			if v.MinimumUrgency == nil {
				return nil
			}
			return string(*v.MinimumUrgency)
		})},
	},
})

var pushSettingsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PushNotificationSettings",
	Fields: graphql.Fields{
		"enabled":            &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: gqlutil.Field(func(v PushSettingsView) any { return v.Enabled })},
		"events":             &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushDeliveryEventGQLEnum))), Resolve: gqlutil.Field(func(v PushSettingsView) any { return gqlDeliveryEventOutput(v.Events) })},
		"minimumUrgency":     &graphql.Field{Type: graphql.NewNonNull(pushUrgencyGQLEnum), Resolve: gqlutil.Field(func(v PushSettingsView) any { return string(v.MinimumUrgency) })},
		"timeZone":           &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushSettingsView) any { return v.TimeZone })},
		"workingHours":       &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushClockRangeGQLType))), Resolve: gqlutil.Field(func(v PushSettingsView) any { return v.WorkingHours })},
		"quietHours":         &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushClockRangeGQLType))), Resolve: gqlutil.Field(func(v PushSettingsView) any { return v.QuietHours })},
		"maxDeferralSeconds": &graphql.Field{Type: graphql.NewNonNull(graphql.Int), Resolve: gqlutil.Field(func(v PushSettingsView) any { return v.MaxDeferralSeconds })},
		"serviceOverrides":   &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushServiceOverrideGQLType))), Resolve: gqlutil.Field(func(v PushSettingsView) any { return v.ServiceOverrides })},
	},
})

var pushClockRangeGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "PushNotificationClockRangeInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"weekdays": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushWeekdayGQLEnum)))},
		"start":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"end":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

var pushServiceOverrideGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "PushNotificationServiceOverrideInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"serviceId":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"enabled":        &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		"events":         &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(pushDeliveryEventGQLEnum))},
		"minimumUrgency": &graphql.InputObjectFieldConfig{Type: pushUrgencyGQLEnum},
	},
})

var pushSettingsGQLInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "PushNotificationSettingsInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"enabled":            &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Boolean)},
		"events":             &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushDeliveryEventGQLEnum)))},
		"minimumUrgency":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(pushUrgencyGQLEnum)},
		"timeZone":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"workingHours":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushClockRangeGQLInput)))},
		"quietHours":         &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushClockRangeGQLInput)))},
		"maxDeferralSeconds": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.Int)},
		"serviceOverrides":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushServiceOverrideGQLInput)))},
	},
})

var deviceSubscriptionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "NotificationDeviceSubscription",
	Fields: graphql.Fields{
		"deviceId":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeviceSubscriptionView) any { return v.DeviceID })},
		"provider":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeviceSubscriptionView) any { return v.Provider })},
		"platform":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeviceSubscriptionView) any { return v.Platform })},
		"preferenceRef":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeviceSubscriptionView) any { return v.PreferenceRef })},
		"createdAt":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeviceSubscriptionView) any { return v.CreatedAt.Format(time.RFC3339Nano) })},
		"updatedAt":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeviceSubscriptionView) any { return v.UpdatedAt.Format(time.RFC3339Nano) })},
		"lastRegisteredAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeviceSubscriptionView) any { return v.LastRegisteredAt.Format(time.RFC3339Nano) })},
	},
})

var pushNotificationGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PushNotification",
	Fields: graphql.Fields{
		"id":           &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.ID })},
		"event":        &graphql.Field{Type: graphql.NewNonNull(pushDeliveryEventGQLEnum), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.Event })},
		"title":        &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.Title })},
		"body":         &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.Body })},
		"urgency":      &graphql.Field{Type: graphql.NewNonNull(pushUrgencyGQLEnum), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.Urgency })},
		"resourceKind": &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.ResourceKind })},
		"resourceId":   &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.ResourceID })},
		"deepLink":     &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.DeepLink })},
		"occurredAt":   &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.OccurredAt.UTC().Format(time.RFC3339Nano) })},
		"createdAt":    &graphql.Field{Type: graphql.NewNonNull(graphql.String), Resolve: gqlutil.Field(func(v PushNotificationView) any { return v.CreatedAt.UTC().Format(time.RFC3339Nano) })},
		"readAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PushNotificationView) any {
			if v.ReadAt == nil {
				return nil
			}
			return v.ReadAt.UTC().Format(time.RFC3339Nano)
		})},
	},
})

// GraphQLQuery contributes the `notificationSettings` query to the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"notificationSettings": &graphql.Field{
			Type: notificationSettingsGQLType,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetSettings(p.Context)
			},
		},
		"pushNotificationSettings": &graphql.Field{
			Type:    pushSettingsGQLType,
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.GetPushSettings(p.Context) },
		},
		"pushNotificationsAvailable": &graphql.Field{
			Type:    graphql.NewNonNull(graphql.Boolean),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.IsPushAvailable(p.Context) },
		},
		"notificationDeviceSubscriptions": &graphql.Field{
			Type: graphql.NewList(deviceSubscriptionGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListDeviceSubscriptions(p.Context)
			},
		},
		"notificationInbox": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushNotificationGQLType))),
			Args: graphql.FieldConfigArgument{
				"limit": &graphql.ArgumentConfig{Type: graphql.Int, DefaultValue: defaultNotificationInboxLimit},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListNotificationInbox(p.Context, p.Args["limit"].(int))
			},
		},
		"unreadPushNotificationCount": &graphql.Field{
			Type:    graphql.NewNonNull(graphql.Int),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.UnreadPushNotificationCount(p.Context) },
		},
	}
}

// GraphQLMutation contributes the `updateNotificationSettings` mutation.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"updateNotificationSettings": &graphql.Field{
			Type: notificationSettingsGQLType,
			Args: graphql.FieldConfigArgument{
				"deployStarted":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
				"deploySucceeded": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
				"deployFailed":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdateSettings(p.Context, p.Args["deployStarted"].(bool), p.Args["deploySucceeded"].(bool), p.Args["deployFailed"].(bool))
			},
		},
		"updatePushNotificationSettings": &graphql.Field{
			Type: pushSettingsGQLType,
			Args: graphql.FieldConfigArgument{
				"settings": &graphql.ArgumentConfig{Type: graphql.NewNonNull(pushSettingsGQLInput)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdatePushSettings(p.Context, gqlPushSettings(p.Args["settings"].(map[string]any)))
			},
		},
		"registerNotificationDeviceSubscription": &graphql.Field{
			Type: deviceSubscriptionGQLType,
			Args: graphql.FieldConfigArgument{
				"deviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"provider": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"platform": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"token":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RegisterDeviceSubscription(p.Context, RegisterDeviceInput{
					DeviceID: p.Args["deviceId"].(string), Provider: p.Args["provider"].(string),
					Platform: p.Args["platform"].(string), Token: p.Args["token"].(string),
				})
			},
		},
		"unregisterNotificationDeviceSubscription": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"deviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UnregisterDeviceSubscription(p.Context, p.Args["deviceId"].(string))
			},
		},
		"revokeNotificationDeviceSubscriptions": &graphql.Field{
			Type: graphql.Int,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				count, err := s.RevokeDeviceSubscriptions(p.Context)
				return int(count), err
			},
		},
		"markPushNotificationRead": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.MarkPushNotificationRead(p.Context, p.Args["id"].(string))
			},
		},
	}
}

func gqlPushSettings(raw map[string]any) PushSettingsView {
	view := PushSettingsView{
		Enabled: raw["enabled"].(bool), Events: gqlDeliveryEvents(raw["events"]),
		MinimumUrgency: DeliveryUrgency(raw["minimumUrgency"].(string)), TimeZone: raw["timeZone"].(string),
		WorkingHours: gqlPushClockRanges(raw["workingHours"]), QuietHours: gqlPushClockRanges(raw["quietHours"]),
		MaxDeferralSeconds: raw["maxDeferralSeconds"].(int),
	}
	for _, item := range raw["serviceOverrides"].([]any) {
		input := item.(map[string]any)
		override := PushServiceOverrideView{ServiceID: input["serviceId"].(string)}
		if value, ok := input["enabled"].(bool); ok {
			override.Enabled = &value
		}
		if events, ok := input["events"]; ok {
			value := gqlDeliveryEvents(events)
			override.Events = &value
		}
		if urgency, ok := input["minimumUrgency"].(string); ok {
			value := DeliveryUrgency(urgency)
			override.MinimumUrgency = &value
		}
		view.ServiceOverrides = append(view.ServiceOverrides, override)
	}
	return view
}

func gqlDeliveryEvents(raw any) []DeliveryEvent {
	values, _ := raw.([]any)
	out := make([]DeliveryEvent, 0, len(values))
	for _, value := range values {
		out = append(out, DeliveryEvent(value.(string)))
	}
	return out
}

func gqlDeliveryEventOutput(events []DeliveryEvent) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		out = append(out, string(event))
	}
	return out
}

func gqlPushClockRanges(raw any) []PushClockRangeView {
	values, _ := raw.([]any)
	out := make([]PushClockRangeView, 0, len(values))
	for _, value := range values {
		item := value.(map[string]any)
		rangeView := PushClockRangeView{Start: item["start"].(string), End: item["end"].(string)}
		for _, weekday := range item["weekdays"].([]any) {
			rangeView.Weekdays = append(rangeView.Weekdays, weekday.(string))
		}
		out = append(out, rangeView)
	}
	return out
}
