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

package projects

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the projects GraphQL fragment (bex extension).

var projectGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Project",
	Fields: graphql.Fields{
		"id":          gqlutil.StrField(func(p ProjectView) any { return p.ID }),
		"name":        gqlutil.StrField(func(p ProjectView) any { return p.Name }),
		"ownerId":     gqlutil.StrField(func(p ProjectView) any { return p.OwnerID }),
		"createdAt":   gqlutil.StrField(func(p ProjectView) any { return p.CreatedAt }),
		"serviceIds":  gqlutil.StrsField(func(p ProjectView) any { return p.ServiceIDs }),
		"databaseIds": gqlutil.StrsField(func(p ProjectView) any { return p.DatabaseIDs }),
		"keyValueIds": gqlutil.StrsField(func(p ProjectView) any { return p.KeyValueIDs }),
	},
})

// GraphQLQuery contributes the projects reads to the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"projects": &graphql.Field{
			Type: graphql.NewList(projectGQLType),
			Args: gqlutil.PageArgs(graphql.FieldConfigArgument{
				"ownerId": gqlutil.ReqArg(graphql.String),
			}),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				out, err := s.List(p.Context, p.Args["ownerId"].(string))
				if err != nil {
					return nil, err
				}
				return gqlutil.Page(p, out, func(project ProjectView) string {
					return project.ID
				}), nil
			},
		},
		"project": &graphql.Field{
			Type: projectGQLType,
			Args: graphql.FieldConfigArgument{
				"id": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["id"].(string))
			},
		},
	}
}

// GraphQLMutation contributes the project write verbs to the root Mutation.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createProject": &graphql.Field{
			Type: projectGQLType,
			Args: graphql.FieldConfigArgument{
				"name":    gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Create(p.Context, p.Args["ownerId"].(string), p.Args["name"].(string))
			},
		},
		"renameProject": gqlutil.ArgMutation(projectGQLType, "name", s.Rename),
		"deleteProject": &graphql.Field{
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"id": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				id := p.Args["id"].(string)
				if err := s.Delete(p.Context, id); err != nil {
					return nil, err
				}
				return id, nil
			},
		},
		"setProjectServices": &graphql.Field{
			Type: projectGQLType,
			Args: graphql.FieldConfigArgument{
				"id":         gqlutil.ReqArg(graphql.String),
				"serviceIds": gqlutil.Arg(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetServices(p.Context, p.Args["id"].(string), gqlutil.StringList(p.Args["serviceIds"]))
			},
		},
		"setProjectDatabases": &graphql.Field{
			Type: projectGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          gqlutil.ReqArg(graphql.String),
				"databaseIds": gqlutil.Arg(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetDatabases(p.Context, p.Args["id"].(string), gqlutil.StringList(p.Args["databaseIds"]))
			},
		},
		"setProjectKeyValues": &graphql.Field{
			Type: projectGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          gqlutil.ReqArg(graphql.String),
				"keyValueIds": gqlutil.Arg(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetKeyValues(p.Context, p.Args["id"].(string), gqlutil.StringList(p.Args["keyValueIds"]))
			},
		},
	}
}
