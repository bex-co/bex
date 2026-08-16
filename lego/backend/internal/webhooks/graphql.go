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
		"id":              gqlutil.StrField(func(v DeliveryView) any { return v.ID }),
		"eventId":         gqlutil.StrField(func(v DeliveryView) any { return v.EventID }),
		"eventType":       gqlutil.StrField(func(v DeliveryView) any { return v.EventType }),
		"serviceId":       gqlutil.StrField(func(v DeliveryView) any { return v.ServiceID }),
		"status":          gqlutil.StrField(func(v DeliveryView) any { return v.Status }),
		"attemptCount":    gqlutil.IntField(func(v DeliveryView) any { return v.AttemptCount }),
		"lastStatusCode":  gqlutil.IntField(func(v DeliveryView) any { return v.LastStatusCode }),
		"lastError":       gqlutil.StrField(func(v DeliveryView) any { return v.LastError }),
		"nextAttemptAt":   gqlutil.StrField(func(v DeliveryView) any { return v.NextAttemptAt }),
		"lastAttemptedAt": gqlutil.StrField(func(v DeliveryView) any { return v.LastAttemptedAt }),
		"deliveredAt":     gqlutil.StrField(func(v DeliveryView) any { return v.DeliveredAt }),
		"createdAt":       gqlutil.StrField(func(v DeliveryView) any { return v.CreatedAt }),
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
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListDeliveries(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["endpointId"].(string), gqlutil.Str(p.Args, "cursor"), gqlutil.Int(p.Args, "limit"))
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
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Create(p.Context, CreateRequest{
					OwnerID: gqlutil.Str(p.Args, "ownerId"), Name: gqlutil.Str(p.Args, "name"),
					URL: p.Args["url"].(string), EventTypes: gqlutil.StringList(p.Args["eventTypes"]),
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
				if v, ok := p.Args["enabled"].(bool); ok {
					enabledPtr = &v
				}
				return s.Update(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["id"].(string), UpdateRequest{
					Name:       gqlutil.Str(p.Args, "name"),
					URL:        gqlutil.Str(p.Args, "url"),
					EventTypes: gqlutil.StringList(p.Args["eventTypes"]),
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
