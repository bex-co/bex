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

package apikeys

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// apiKeyGQLType mirrors the REST APIKey object (bex extension; Render's dashboard
// manages keys outside its public schemas). secret is non-empty only in the
// createApiKey payload — list resolves it empty.
var apiKeyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKey",
	Fields: graphql.Fields{
		"id":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(k APIKey) any { return k.ID })},
		"name":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(k APIKey) any { return k.Name })},
		"secret":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(k APIKey) any { return k.Secret })},
		"createdAt":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(k APIKey) any { return k.CreatedAt })},
		"createdBy":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(k APIKey) any { return k.CreatedBy })},
		"lastUsedAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(k APIKey) any { return k.LastUsedAt })},
	},
})

// GraphQLQuery returns the apiKeys list query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"apiKeys": &graphql.Field{
			Type:    graphql.NewList(apiKeyGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.ListAPIKeys(p.Context) },
		},
	}
}

// GraphQLMutation returns the createApiKey / revokeApiKey mutations.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createApiKey": &graphql.Field{
			Type: apiKeyGQLType,
			Args: graphql.FieldConfigArgument{
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateAPIKey(p.Context, p.Args["name"].(string))
			},
		},
		"revokeApiKey": &graphql.Field{
			Type: graphql.Boolean,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.RevokeAPIKey(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},
	}
}
