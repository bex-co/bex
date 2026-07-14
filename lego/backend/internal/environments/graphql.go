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

package environments

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the environments GraphQL fragment, mirroring
// internal/projects/graphql.go's shape (bex extension; Type name
// "Environment" is distinct from the pre-existing, unrelated "EnvGroup" type
// internal/envgroups registers, so the merged schema has no collision).

var environmentGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Environment",
	Fields: graphql.Fields{
		"id":                      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.ID })},
		"projectId":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.ProjectID })},
		"name":                    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.Name })},
		"ownerId":                 &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.OwnerID })},
		"createdAt":               &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.CreatedAt })},
		"serviceIds":              &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.ServiceIDs })},
		"databaseIds":             &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.DatabaseIDs })},
		"keyValueIds":             &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.KeyValueIDs })},
		"protectedStatus":         &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.ProtectedStatus })},
		"networkIsolationEnabled": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.NetworkIsolationEnabled })},
		"ipAllowList":             &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(e EnvironmentView) any { return e.IPAllowList })},
	},
})

// GraphQLQuery contributes the environments reads to the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"environments": &graphql.Field{
			Type: graphql.NewList(environmentGQLType),
			Args: graphql.FieldConfigArgument{
				"projectId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.List(p.Context, p.Args["projectId"].(string))
			},
		},
		"environment": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Get(p.Context, p.Args["id"].(string))
			},
		},
	}
}

// GraphQLMutation contributes the environment write verbs to the root Mutation.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createEnvironment": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"name":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"projectId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Create(p.Context, p.Args["projectId"].(string), p.Args["name"].(string))
			},
		},
		"renameEnvironment": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Rename(p.Context, p.Args["id"].(string), p.Args["name"].(string))
			},
		},
		"deleteEnvironment": &graphql.Field{
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				id := p.Args["id"].(string)
				if err := s.Delete(p.Context, id); err != nil {
					return nil, err
				}
				return id, nil
			},
		},
		"setEnvironmentServices": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"serviceIds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				raw, _ := p.Args["serviceIds"].([]any)
				names := make([]string, len(raw))
				for i, v := range raw {
					names[i], _ = v.(string)
				}
				return s.SetServices(p.Context, p.Args["id"].(string), names)
			},
		},
		"setEnvironmentDatabases": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"databaseIds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				raw, _ := p.Args["databaseIds"].([]any)
				names := make([]string, len(raw))
				for i, v := range raw {
					names[i], _ = v.(string)
				}
				return s.SetDatabases(p.Context, p.Args["id"].(string), names)
			},
		},
		"setEnvironmentKeyValues": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"keyValueIds": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				raw, _ := p.Args["keyValueIds"].([]any)
				names := make([]string, len(raw))
				for i, v := range raw {
					names[i], _ = v.(string)
				}
				return s.SetKeyValues(p.Context, p.Args["id"].(string), names)
			},
		},
		// setEnvironmentACL replaces the full protected-environment ACL triple
		// (w6/m19) — full-replace, matching setEnvironmentServices above.
		"setEnvironmentACL": &graphql.Field{
			Type: environmentGQLType,
			Args: graphql.FieldConfigArgument{
				"id":                      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"protectedStatus":         &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"networkIsolationEnabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
				"ipAllowList":             &graphql.ArgumentConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				raw, _ := p.Args["ipAllowList"].([]any)
				cidrs := make([]string, len(raw))
				for i, v := range raw {
					cidrs[i], _ = v.(string)
				}
				return s.SetACL(p.Context, p.Args["id"].(string), p.Args["protectedStatus"].(string), p.Args["networkIsolationEnabled"].(bool), cidrs)
			},
		},
	}
}
