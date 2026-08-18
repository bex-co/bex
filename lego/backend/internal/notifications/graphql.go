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
		"deployStarted":   gqlutil.BoolField(func(v SettingsView) any { return v.DeployStarted }),
		"deploySucceeded": gqlutil.BoolField(func(v SettingsView) any { return v.DeploySucceeded }),
		"deployFailed":    gqlutil.BoolField(func(v SettingsView) any { return v.DeployFailed }),
	},
})

var pushDeliveryEventGQLEnum = graphql.NewEnum(graphql.EnumConfig{
	Name: "PushNotificationEvent",
	Values: graphql.EnumValueConfigMap{
		"DEPLOY_STARTED":       &graphql.EnumValueConfig{Value: string(DeliveryEventDeployStarted)},
		"DEPLOY_SUCCEEDED":     &graphql.EnumValueConfig{Value: string(DeliveryEventDeploySucceeded)},
		"DEPLOY_FAILED":        &graphql.EnumValueConfig{Value: string(DeliveryEventDeployFailed)},
		"SERVER_FAILED":        &graphql.EnumValueConfig{Value: string(DeliveryEventServerFailed)},
		"SERVER_AVAILABLE":     &graphql.EnumValueConfig{Value: string(DeliveryEventServerAvailable)},
		"SERVICE_SUSPENDED":    &graphql.EnumValueConfig{Value: string(DeliveryEventServiceSuspended)},
		"SERVICE_RESUMED":      &graphql.EnumValueConfig{Value: string(DeliveryEventServiceResumed)},
		"CRON_FAILED":          &graphql.EnumValueConfig{Value: string(DeliveryEventCronFailed)},
		"AGENT_NEEDS_DECISION": &graphql.EnumValueConfig{Value: string(DeliveryEventAgentNeedsDecision)},
		"AGENT_PR_READY":       &graphql.EnumValueConfig{Value: string(DeliveryEventAgentPRReady)},
		"AGENT_FAILED":         &graphql.EnumValueConfig{Value: string(DeliveryEventAgentFailed)},
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
		"weekdays": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushWeekdayGQLEnum))), func(v PushClockRangeView) any { return v.Weekdays }),
		"start":    gqlutil.ReqStrField(func(v PushClockRangeView) any { return v.Start }),
		"end":      gqlutil.ReqStrField(func(v PushClockRangeView) any { return v.End }),
	},
})

var pushServiceOverrideGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PushNotificationServiceOverride",
	Fields: graphql.Fields{
		"serviceId": gqlutil.ReqStrField(func(v PushServiceOverrideView) any { return v.ServiceID }),
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
		"enabled":            gqlutil.ReqBoolField(func(v PushSettingsView) any { return v.Enabled }),
		"events":             gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushDeliveryEventGQLEnum))), func(v PushSettingsView) any { return gqlDeliveryEventOutput(v.Events) }),
		"minimumUrgency":     gqlutil.Typed(graphql.NewNonNull(pushUrgencyGQLEnum), func(v PushSettingsView) any { return string(v.MinimumUrgency) }),
		"timeZone":           gqlutil.ReqStrField(func(v PushSettingsView) any { return v.TimeZone }),
		"workingHours":       gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushClockRangeGQLType))), func(v PushSettingsView) any { return v.WorkingHours }),
		"quietHours":         gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushClockRangeGQLType))), func(v PushSettingsView) any { return v.QuietHours }),
		"maxDeferralSeconds": gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(v PushSettingsView) any { return v.MaxDeferralSeconds }),
		"serviceOverrides":   gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(pushServiceOverrideGQLType))), func(v PushSettingsView) any { return v.ServiceOverrides }),
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
		"deviceId":         gqlutil.StrField(func(v DeviceSubscriptionView) any { return v.DeviceID }),
		"provider":         gqlutil.StrField(func(v DeviceSubscriptionView) any { return v.Provider }),
		"platform":         gqlutil.StrField(func(v DeviceSubscriptionView) any { return v.Platform }),
		"preferenceRef":    gqlutil.StrField(func(v DeviceSubscriptionView) any { return v.PreferenceRef }),
		"createdAt":        gqlutil.StrField(func(v DeviceSubscriptionView) any { return v.CreatedAt.Format(time.RFC3339Nano) }),
		"updatedAt":        gqlutil.StrField(func(v DeviceSubscriptionView) any { return v.UpdatedAt.Format(time.RFC3339Nano) }),
		"lastRegisteredAt": gqlutil.StrField(func(v DeviceSubscriptionView) any { return v.LastRegisteredAt.Format(time.RFC3339Nano) }),
	},
})

var pushNotificationGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PushNotification",
	Fields: graphql.Fields{
		"id":           gqlutil.ReqStrField(func(v PushNotificationView) any { return v.ID }),
		"event":        gqlutil.Typed(graphql.NewNonNull(pushDeliveryEventGQLEnum), func(v PushNotificationView) any { return v.Event }),
		"title":        gqlutil.ReqStrField(func(v PushNotificationView) any { return v.Title }),
		"body":         gqlutil.ReqStrField(func(v PushNotificationView) any { return v.Body }),
		"urgency":      gqlutil.Typed(graphql.NewNonNull(pushUrgencyGQLEnum), func(v PushNotificationView) any { return v.Urgency }),
		"resourceKind": gqlutil.ReqStrField(func(v PushNotificationView) any { return v.ResourceKind }),
		"resourceId":   gqlutil.ReqStrField(func(v PushNotificationView) any { return v.ResourceID }),
		"deepLink":     gqlutil.ReqStrField(func(v PushNotificationView) any { return v.DeepLink }),
		"occurredAt":   gqlutil.ReqStrField(func(v PushNotificationView) any { return v.OccurredAt.UTC().Format(time.RFC3339Nano) }),
		"createdAt":    gqlutil.ReqStrField(func(v PushNotificationView) any { return v.CreatedAt.UTC().Format(time.RFC3339Nano) }),
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
				"deployStarted":   gqlutil.ReqArg(graphql.Boolean),
				"deploySucceeded": gqlutil.ReqArg(graphql.Boolean),
				"deployFailed":    gqlutil.ReqArg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdateSettings(p.Context, p.Args["deployStarted"].(bool), p.Args["deploySucceeded"].(bool), p.Args["deployFailed"].(bool))
			},
		},
		"updatePushNotificationSettings": &graphql.Field{
			Type: pushSettingsGQLType,
			Args: graphql.FieldConfigArgument{
				"settings": gqlutil.ReqArg(pushSettingsGQLInput),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdatePushSettings(p.Context, gqlPushSettings(p.Args["settings"].(map[string]any)))
			},
		},
		"registerNotificationDeviceSubscription": &graphql.Field{
			Type: deviceSubscriptionGQLType,
			Args: graphql.FieldConfigArgument{
				"deviceId":  gqlutil.ReqArg(graphql.String),
				"sessionId": gqlutil.ReqArg(graphql.String),
				"provider":  gqlutil.ReqArg(graphql.String),
				"platform":  gqlutil.ReqArg(graphql.String),
				"token":     gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RegisterDeviceSubscription(p.Context, RegisterDeviceInput{
					DeviceID: p.Args["deviceId"].(string), Provider: p.Args["provider"].(string),
					SessionID: p.Args["sessionId"].(string), Platform: p.Args["platform"].(string), Token: p.Args["token"].(string),
				})
			},
		},
		"unregisterNotificationDeviceSubscription": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"deviceId": gqlutil.ReqArg(graphql.String),
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
				"id": gqlutil.ReqArg(graphql.String),
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
