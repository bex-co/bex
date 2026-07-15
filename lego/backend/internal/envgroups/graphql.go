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

package envgroups

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the env-groups GraphQL fragment. Types carry distinct names
// (EnvGroupVar/EnvGroupSecretFile, not the apps feature's EnvVar/SecretFile) so
// the single merged schema has no duplicate-type collision. Group list/detail
// reads are keys/names-only; the top-level per-value queries reveal sensitive
// values on demand. Mutations return a success boolean except create/rename.

var envGroupVarGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvGroupVar",
	Fields: graphql.Fields{
		"key":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EnvVarView) any { return v.Key })},
		"value": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EnvVarView) any { return v.Value })},
	},
})

var envGroupFileGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvGroupSecretFile",
	Fields: graphql.Fields{
		"name":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(f SecretFileView) any { return f.Name })},
		"content": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(f SecretFileView) any { return f.Content })},
	},
})

var envGroupGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvGroup",
	Fields: graphql.Fields{
		"id":            &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(g EnvGroupView) any { return g.ID })},
		"name":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(g EnvGroupView) any { return g.Name })},
		"ownerId":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(g EnvGroupView) any { return g.OwnerID })},
		"environmentId": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(g EnvGroupView) any { return g.EnvironmentID })},
		"serviceLinks":  &graphql.Field{Type: graphql.NewList(graphql.String), Resolve: gqlutil.Field(func(g EnvGroupView) any { return g.ServiceLinks })},
		"envVars":       &graphql.Field{Type: graphql.NewList(envGroupVarGQLType), Resolve: gqlutil.Field(func(g EnvGroupView) any { return g.EnvVars })},
		"secretFiles":   &graphql.Field{Type: graphql.NewList(envGroupFileGQLType), Resolve: gqlutil.Field(func(g EnvGroupView) any { return g.SecretFiles })},
		"createdAt":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(g EnvGroupView) any { return g.CreatedAt })},
		"updatedAt":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(g EnvGroupView) any { return g.UpdatedAt })},
	},
})

var envGroupVarInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "EnvGroupVarInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"key":   &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

func varsFromArgs(raw []any) []EnvVarView {
	vars := make([]EnvVarView, 0, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		ev := EnvVarView{}
		if k, ok := m["key"].(string); ok {
			ev.Key = k
		}
		if v, ok := m["value"].(string); ok {
			ev.Value = v
		}
		vars = append(vars, ev)
	}
	return vars
}

// GraphQLQuery returns the env-group read fields. envGroups' ownerId (w6/m24)
// names the workspace to list; omitted means the caller's default workspace.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"envGroups": &graphql.Field{
			Type: graphql.NewList(envGroupGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListEnvGroups(p.Context, gqlStr(p.Args, "ownerId"))
			},
		},
		"envGroup": &graphql.Field{
			Type: envGroupGQLType,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetEnvGroup(p.Context, p.Args["id"].(string))
			},
		},
		"envGroupVar": &graphql.Field{
			Type: envGroupVarGQLType,
			Args: graphql.FieldConfigArgument{
				"id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"key": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetEnvGroupVar(p.Context, p.Args["id"].(string), p.Args["key"].(string))
			},
		},
		"envGroupSecretFile": &graphql.Field{
			Type: envGroupFileGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetEnvGroupFile(p.Context, p.Args["id"].(string), p.Args["name"].(string))
			},
		},
	}
}

// GraphQLMutation returns the env-group write mutations.
func (s *Service) GraphQLMutation() graphql.Fields {
	idArg := graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}}
	idService := graphql.FieldConfigArgument{
		"id":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
	}
	return graphql.Fields{
		"createEnvGroup": &graphql.Field{
			Type: envGroupGQLType,
			Args: graphql.FieldConfigArgument{
				"name":          &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"ownerId":       &graphql.ArgumentConfig{Type: graphql.String},
				"environmentId": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateEnvGroup(p.Context, gqlStr(p.Args, "ownerId"), p.Args["name"].(string), gqlStr(p.Args, "environmentId"))
			},
		},
		"renameEnvGroup": &graphql.Field{
			Type: envGroupGQLType,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RenameEnvGroup(p.Context, p.Args["id"].(string), p.Args["name"].(string))
			},
		},
		"deleteEnvGroup": &graphql.Field{
			Type: graphql.Boolean,
			Args: idArg,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteEnvGroup(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},
		"setEnvGroupVars": &graphql.Field{ // replace-all group env vars
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"envVars": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(envGroupVarInputGQLType)))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				raw, _ := p.Args["envVars"].([]any)
				_, err := s.SetEnvGroupVars(p.Context, p.Args["id"].(string), varsFromArgs(raw))
				return err == nil, err
			},
		},
		"setEnvGroupVar": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"key":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"value": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				value, _ := p.Args["value"].(string)
				_, err := s.SetEnvGroupVar(p.Context, p.Args["id"].(string), p.Args["key"].(string), value)
				return err == nil, err
			},
		},
		"deleteEnvGroupVar": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"key": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteEnvGroupVar(p.Context, p.Args["id"].(string), p.Args["key"].(string))
				return err == nil, err
			},
		},
		"setEnvGroupSecretFile": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"content": &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				content, _ := p.Args["content"].(string)
				_, err := s.SetEnvGroupFile(p.Context, p.Args["id"].(string), p.Args["name"].(string), content)
				return err == nil, err
			},
		},
		"deleteEnvGroupSecretFile": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteEnvGroupFile(p.Context, p.Args["id"].(string), p.Args["name"].(string))
				return err == nil, err
			},
		},
		"linkEnvGroup": &graphql.Field{
			Type: graphql.Boolean,
			Args: idService,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.LinkService(p.Context, p.Args["id"].(string), p.Args["serviceId"].(string))
				return err == nil, err
			},
		},
		"unlinkEnvGroup": &graphql.Field{
			Type: graphql.Boolean,
			Args: idService,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.UnlinkService(p.Context, p.Args["id"].(string), p.Args["serviceId"].(string))
				return err == nil, err
			},
		},
	}
}

// gqlStr reads an optional string arg, "" when absent — package-local per
// apikeys' own gqlStr precedent (not worth a shared gqlutil helper for a
// one-line copy).
func gqlStr(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}
