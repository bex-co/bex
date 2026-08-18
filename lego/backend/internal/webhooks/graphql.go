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
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go mirrors the REST wire shapes (rest.go) — same fields, same
// mint-once secret rule: only createWebhookEndpoint's result ever carries
// `secret` (populated on that view alone; every read leaves it null).

var endpointGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WebhookEndpoint",
	Fields: graphql.Fields{
		"id":             gqlutil.StrField(func(v EndpointView) any { return v.ID }),
		"name":           gqlutil.StrField(func(v EndpointView) any { return v.Name }),
		"url":            gqlutil.StrField(func(v EndpointView) any { return v.URL }),
		"ownerId":        gqlutil.StrField(func(v EndpointView) any { return v.OwnerID }),
		"eventTypes":     gqlutil.StrsField(func(v EndpointView) any { return v.EventTypes }),
		"enabled":        gqlutil.BoolField(func(v EndpointView) any { return v.Enabled }),
		"disabledReason": gqlutil.StrField(func(v EndpointView) any { return v.DisabledReason }),
		// secret is non-null only on the create mutation's result — the
		// mint-once rule (see rest.go's endpointWire).
		"secret":    gqlutil.StrField(func(v EndpointView) any { return v.Secret }),
		"createdBy": gqlutil.StrField(func(v EndpointView) any { return v.CreatedBy }),
		"createdAt": gqlutil.StrField(func(v EndpointView) any { return v.CreatedAt }),
		"updatedAt": gqlutil.StrField(func(v EndpointView) any { return v.UpdatedAt }),
	},
})

var deliveryGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WebhookDelivery",
	Fields: graphql.Fields{
		"id":             gqlutil.StrField(func(v DeliveryView) any { return v.ID }),
		"eventId":        gqlutil.StrField(func(v DeliveryView) any { return v.EventID }),
		"eventType":      gqlutil.StrField(func(v DeliveryView) any { return v.EventType }),
		"serviceId":      gqlutil.StrField(func(v DeliveryView) any { return v.ServiceID }),
		"status":         gqlutil.StrField(func(v DeliveryView) any { return v.Status }),
		"attemptNumber":  gqlutil.IntField(func(v DeliveryView) any { return v.AttemptNumber }),
		"statusCode":     gqlutil.IntField(func(v DeliveryView) any { return v.StatusCode }),
		"transportError": gqlutil.StrField(func(v DeliveryView) any { return v.TransportError }),
		"responseBody":   gqlutil.StrField(func(v DeliveryView) any { return v.ResponseBody }),
		"requestBody":    gqlutil.StrField(func(v DeliveryView) any { return v.RequestBody }),
		"sentAt":         gqlutil.StrField(func(v DeliveryView) any { return v.SentAt }),
		"nextAttemptAt":  gqlutil.StrField(func(v DeliveryView) any { return v.NextAttemptAt }),
		"parentStatus":   gqlutil.StrField(func(v DeliveryView) any { return v.ParentStatus }),
		// cursor rides each item (the serviceEvents convention) — echo the last
		// one back to page.
		"cursor": gqlutil.StrField(func(v DeliveryView) any { return v.Cursor }),
	},
})

// gqlInt reads an optional int argument (absent => 0).

// GraphQLQuery returns webhookEndpoints/webhookEndpoint/webhookDeliveries +
// the webhookEventTypes vocabulary (the dashboard's picker source).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"webhookEndpoints": &graphql.Field{
			Type: graphql.NewList(endpointGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.List(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
		"webhookEndpoint": &graphql.Field{
			Type: endpointGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["id"].(string))
			},
		},
		"webhookDeliveries": &graphql.Field{
			Type: graphql.NewList(deliveryGQLType),
			Args: graphql.FieldConfigArgument{
				"endpointId": gqlutil.ReqArg(graphql.String),
				"ownerId":    gqlutil.Arg(graphql.String),
				"cursor":     gqlutil.Arg(graphql.String),
				"limit":      gqlutil.Arg(graphql.Int),
				"sentAfter":  gqlutil.Arg(graphql.String),
				"sentBefore": gqlutil.Arg(graphql.String),
				"status":     gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				sentAfter, err := core.ParseTime("sentAfter", gqlutil.Str(p.Args, "sentAfter"))
				if err != nil {
					return nil, err
				}
				sentBefore, err := core.ParseTime("sentBefore", gqlutil.Str(p.Args, "sentBefore"))
				if err != nil {
					return nil, err
				}
				return s.ListDeliveriesFiltered(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["endpointId"].(string), DeliveryFilter{
					Cursor: gqlutil.Str(p.Args, "cursor"), Limit: gqlutil.Int(p.Args, "limit"),
					SentAfter: sentAfter, SentBefore: sentBefore, Status: gqlutil.Str(p.Args, "status"),
				})
			},
		},
		"webhookEventTypes": &graphql.Field{
			Type: graphql.NewList(graphql.String),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return EventTypes, nil
			},
		},
	}
}

// GraphQLMutation returns create/delete + the enabled toggle.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createWebhookEndpoint": &graphql.Field{
			Type: endpointGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId":    gqlutil.Arg(graphql.String),
				"name":       gqlutil.Arg(graphql.String),
				"url":        gqlutil.ReqArg(graphql.String),
				"eventTypes": gqlutil.Arg(graphql.NewNonNull(graphql.NewList(graphql.String))),
				// Optional for backward-compatible GraphQL dialect evolution; omitted
				// means enabled, while false creates disabled like Render REST.
				"enabled": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				enabled := true
				if value, ok := p.Args["enabled"].(bool); ok {
					enabled = value
				}
				return s.Create(p.Context, CreateRequest{
					OwnerID: gqlutil.Str(p.Args, "ownerId"), Name: gqlutil.Str(p.Args, "name"),
					URL: p.Args["url"].(string), EventTypes: gqlutil.StringList(p.Args["eventTypes"]),
					Enabled: enabled,
				})
			},
		},
		"updateWebhookEndpoint": &graphql.Field{
			Type: endpointGQLType,
			Args: graphql.FieldConfigArgument{
				"id":         gqlutil.ReqArg(graphql.String),
				"ownerId":    gqlutil.Arg(graphql.String),
				"name":       gqlutil.Arg(graphql.String),
				"url":        gqlutil.Arg(graphql.String),
				"eventTypes": gqlutil.Arg(graphql.NewList(graphql.String)),
				"enabled":    gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				var enabledPtr *bool
				var eventTypesPtr *[]string
				var namePtr, urlPtr *string
				if v, ok := p.Args["enabled"].(bool); ok {
					enabledPtr = &v
				}
				if raw, ok := p.Args["eventTypes"]; ok {
					values := gqlutil.StringList(raw)
					eventTypesPtr = &values
				}
				if value, ok := p.Args["name"].(string); ok {
					namePtr = &value
				}
				if value, ok := p.Args["url"].(string); ok {
					urlPtr = &value
				}
				return s.Update(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["id"].(string), UpdateRequest{
					Name:       namePtr,
					URL:        urlPtr,
					EventTypes: eventTypesPtr,
					Enabled:    enabledPtr,
				})
			},
		},
		"setWebhookEndpointEnabled": &graphql.Field{
			Type: endpointGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
				"enabled": gqlutil.ReqArg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetEnabled(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["id"].(string), p.Args["enabled"].(bool))
			},
		},
		"resendWebhookDelivery": &graphql.Field{
			Type: deliveryGQLType,
			Args: graphql.FieldConfigArgument{
				"endpointId":     gqlutil.ReqArg(graphql.String),
				"attemptId":      gqlutil.ReqArg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
				"idempotencyKey": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Resend(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["endpointId"].(string),
					p.Args["attemptId"].(string), p.Args["idempotencyKey"].(string))
			},
		},
		"deleteWebhookEndpoint": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.Delete(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["id"].(string))
				return err == nil, err
			},
		},
	}
}
