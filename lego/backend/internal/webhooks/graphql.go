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
		"id":             &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EndpointView) any { return v.ID })},
		"name":           &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EndpointView) any { return v.Name })},
		"url":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EndpointView) any { return v.URL })},
		"ownerId":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EndpointView) any { return v.OwnerID })},
		"eventTypes":     &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(v EndpointView) any { return v.EventTypes })},
		"enabled":        &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v EndpointView) any { return v.Enabled })},
		"disabledReason": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EndpointView) any { return v.DisabledReason })},
		// secret is non-null only on the create mutation's result — the
		// mint-once rule (see rest.go's endpointWire).
		"secret":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EndpointView) any { return v.Secret })},
		"createdBy": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EndpointView) any { return v.CreatedBy })},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EndpointView) any { return v.CreatedAt })},
		"updatedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EndpointView) any { return v.UpdatedAt })},
	},
})

var deliveryGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WebhookDelivery",
	Fields: graphql.Fields{
		"id":              &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.ID })},
		"eventId":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.EventID })},
		"eventType":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.EventType })},
		"serviceId":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.ServiceID })},
		"status":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.Status })},
		"attemptCount":    &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.AttemptCount })},
		"lastStatusCode":  &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.LastStatusCode })},
		"lastError":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.LastError })},
		"nextAttemptAt":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.NextAttemptAt })},
		"lastAttemptedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.LastAttemptedAt })},
		"deliveredAt":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.DeliveredAt })},
		"createdAt":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.CreatedAt })},
		// cursor rides each item (the serviceEvents convention) — echo the last
		// one back to page.
		"cursor": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v DeliveryView) any { return v.Cursor })},
	},
})

// gqlStr reads an optional string argument (absent => "").
func gqlStr(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// gqlInt reads an optional int argument (absent => 0).
func gqlInt(args map[string]any, key string) int {
	if v, ok := args[key].(int); ok {
		return v
	}
	return 0
}

// GraphQLQuery returns webhookEndpoints/webhookEndpoint/webhookDeliveries +
// the webhookEventTypes vocabulary (the dashboard's picker source).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"webhookEndpoints": &graphql.Field{
			Type: graphql.NewList(endpointGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.List(p.Context, gqlStr(p.Args, "ownerId"))
			},
		},
		"webhookEndpoint": &graphql.Field{
			Type: endpointGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, gqlStr(p.Args, "ownerId"), p.Args["id"].(string))
			},
		},
		"webhookDeliveries": &graphql.Field{
			Type: graphql.NewList(deliveryGQLType),
			Args: graphql.FieldConfigArgument{
				"endpointId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"ownerId":    &graphql.ArgumentConfig{Type: graphql.String},
				"cursor":     &graphql.ArgumentConfig{Type: graphql.String},
				"limit":      &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListDeliveries(p.Context, gqlStr(p.Args, "ownerId"), p.Args["endpointId"].(string), gqlStr(p.Args, "cursor"), gqlInt(p.Args, "limit"))
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
				"ownerId":    &graphql.ArgumentConfig{Type: graphql.String},
				"name":       &graphql.ArgumentConfig{Type: graphql.String},
				"url":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"eventTypes": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.String))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Create(p.Context, CreateRequest{
					OwnerID: gqlStr(p.Args, "ownerId"), Name: gqlStr(p.Args, "name"),
					URL: p.Args["url"].(string), EventTypes: gqlutil.StringList(p.Args["eventTypes"]),
				})
			},
		},
		"setWebhookEndpointEnabled": &graphql.Field{
			Type: endpointGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
				"enabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetEnabled(p.Context, gqlStr(p.Args, "ownerId"), p.Args["id"].(string), p.Args["enabled"].(bool))
			},
		},
		"deleteWebhookEndpoint": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.Delete(p.Context, gqlStr(p.Args, "ownerId"), p.Args["id"].(string))
				return err == nil, err
			},
		},
	}
}
