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

// GraphQLQuery contributes the `notificationSettings` query to the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"notificationSettings": &graphql.Field{
			Type: notificationSettingsGQLType,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetSettings(p.Context)
			},
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
	}
}
