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

package postgres

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// Render's dashboard GraphQL calls a managed Postgres a "database" (query
// database(id), databaseStatusQuery, ...) — captured live — even though its REST
// noun is "postgres". bex mirrors that split: REST /v1/postgres, GraphQL
// database* (which also matches bex's own Database CRD).
var postgresGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Database",
	Fields: graphql.Fields{
		"id":                      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.ID })},
		"name":                    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Name })},
		"plan":                    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Plan })},
		"version":                 &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Version })},
		"status":                  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Status })},
		"databaseName":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.DatabaseName })},
		"databaseUser":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.DatabaseUser })},
		"diskSizeGB":              &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(v PostgresView) any { return v.DiskSizeGB })},
		"highAvailabilityEnabled": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v PostgresView) any { return v.HighAvailabilityEnabled })},
		"suspended":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Suspended })},
		"createdAt":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.CreatedAt })},
		"externalHost":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresView) any { return v.ExternalHost })},
		"public":                  &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(v PostgresView) any { return v.Public })},
	},
})

var connectionInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "PostgresConnectionInfo",
	Fields: graphql.Fields{
		"password":                 &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.Password })},
		"internalConnectionString": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.InternalConnectionString })},
		"externalConnectionString": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.ExternalConnectionString })},
		"psqlCommand":              &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v PostgresConnectionInfo) any { return v.PSQLCommand })},
	},
})

// GraphQLQuery returns the database read fields (Render dashboard nouns).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"databases": &graphql.Field{ // list; Render lists via env, bex offers a top-level list
			Type:    graphql.NewList(postgresGQLType),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.ListPostgres(p.Context) },
		},
		"database": &graphql.Field{ // Render's dashboard query name
			Type: postgresGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetPostgres(p.Context, p.Args["id"].(string))
			},
		},
		"databaseConnectionInfo": &graphql.Field{
			Type: connectionInfoGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PostgresConnectionInfo(p.Context, p.Args["id"].(string))
			},
		},
	}
}

// GraphQLMutation returns the createDatabase / deleteDatabase mutations.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createDatabase": &graphql.Field{
			Type: postgresGQLType,
			Args: graphql.FieldConfigArgument{
				"name":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"plan":       &graphql.ArgumentConfig{Type: graphql.String},
				"version":    &graphql.ArgumentConfig{Type: graphql.String},
				"diskSizeGB": &graphql.ArgumentConfig{Type: graphql.Int},
				"public":     &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := CreatePostgresRequest{Name: p.Args["name"].(string)}
				if v, ok := p.Args["plan"].(string); ok {
					req.Plan = v
				}
				if v, ok := p.Args["version"].(string); ok {
					req.Version = v
				}
				if v, ok := p.Args["diskSizeGB"].(int); ok {
					req.DiskSizeGB = int32(v)
				}
				if v, ok := p.Args["public"].(bool); ok {
					req.Public = v
				}
				return s.CreatePostgres(p.Context, req)
			},
		},
		"deleteDatabase": &graphql.Field{
			Type: graphql.Boolean,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeletePostgres(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},
	}
}
