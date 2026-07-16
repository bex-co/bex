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

package secrets

import (
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the env-vars GraphQL mutation fragment. The *reads* are Render
// dashboard-shaped and nested under the Service type (`service{ envVarKeys{ id
// key } }` / `service{ envVar(key){ value } }`) — those fields live in the apps
// package and reach this Service through the core.EnvVarReader seam, so the reads
// stay on the Service object exactly like Render's dashboard. The mutations are
// top-level (Render's dashboard mutation names weren't captured, so these are
// bex's own) and return a success boolean; the full objects are on the REST
// surface and the nested reads.

// envVarsFromArgs maps the GraphQL envVars input list onto []EnvVarView.
func envVarsFromArgs(raw []any) []EnvVarView {
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
		if generate, ok := m["generateValue"].(bool); ok {
			ev.Generate = generate
		}
		vars = append(vars, ev)
	}
	return vars
}

var envVarListValueGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvVarListValue",
	Fields: graphql.Fields{
		"id":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EnvVarView) any { return v.Key })},
		"key":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EnvVarView) any { return v.Key })},
		"value": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v EnvVarView) any { return v.Value })},
	},
})

var envVarWithCursorGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvVarWithCursor",
	Fields: graphql.Fields{
		"envVar": &graphql.Field{Type: envVarListValueGQLType, Resolve: gqlutil.Field(func(v envVarWithCursor) any { return v.EnvVar })},
		"cursor": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(v envVarWithCursor) any { return v.Cursor })},
	},
})

var secretFileListValueGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SecretFileListValue",
	Fields: graphql.Fields{
		"id":   &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(f SecretFileView) any { return f.Name })},
		"name": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(f SecretFileView) any { return f.Name })},
	},
})

var secretFileWithCursorGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SecretFileWithCursor",
	Fields: graphql.Fields{
		"secretFile": &graphql.Field{Type: secretFileListValueGQLType, Resolve: gqlutil.Field(func(f secretFileWithCursor) any { return f.SecretFile })},
		"cursor":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(f secretFileWithCursor) any { return f.Cursor })},
	},
})

// GraphQLQuery exposes the paged env-var and secret-file lists as the GraphQL
// twins of Render's REST envelopes. The existing nested service.envVarKeys /
// service.secretFileNames fields remain intact for old dashboard clients; new
// clients use envVars(serviceId,cursor,limit) / secretFiles(serviceId,cursor,limit).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"envVars": &graphql.Field{
			Type: graphql.NewList(envVarWithCursorGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"cursor":    &graphql.ArgumentConfig{Type: graphql.String},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				cursor, _ := p.Args["cursor"].(string)
				limit := 0
				if value, ok := p.Args["limit"].(int); ok {
					if value < 1 {
						return nil, core.ErrBadRequest
					}
					limit = value
				}
				vars, err := s.ListEnvVarsPage(p.Context, p.Args["serviceId"].(string), cursor, limit)
				if err != nil {
					return nil, err
				}
				return toEnvVarList(vars), nil
			},
		},
		"secretFiles": &graphql.Field{
			Type: graphql.NewList(secretFileWithCursorGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"cursor":    &graphql.ArgumentConfig{Type: graphql.String},
				"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				cursor, _ := p.Args["cursor"].(string)
				limit := 0
				if value, ok := p.Args["limit"].(int); ok {
					if value < 1 {
						return nil, core.ErrBadRequest
					}
					limit = value
				}
				files, err := s.ListSecretFilesPage(p.Context, p.Args["serviceId"].(string), cursor, limit)
				if err != nil {
					return nil, err
				}
				return toSecretFileList(files), nil
			},
		},
	}
}

// GraphQLMutation returns the env-var write mutations for the root to merge.
func (s *Service) GraphQLMutation() graphql.Fields {
	svcKey := graphql.FieldConfigArgument{
		"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		"key":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
	}
	return graphql.Fields{
		"setEnvVars": &graphql.Field{ // Render's replace-all
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"envVars":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(gqlutil.EnvVarInputType)))},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				raw, _ := p.Args["envVars"].([]any)
				_, err := s.SetEnvVars(p.Context, p.Args["serviceId"].(string), envVarsFromArgs(raw))
				return err == nil, err
			},
		},
		"setEnvVar": &graphql.Field{ // Render's add-or-update one
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"serviceId":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"key":           &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"value":         &graphql.ArgumentConfig{Type: graphql.String},
				"generateValue": &graphql.ArgumentConfig{Type: graphql.Boolean},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				value, _ := p.Args["value"].(string)
				generate, _ := p.Args["generateValue"].(bool)
				_, err := s.SetEnvVar(p.Context, p.Args["serviceId"].(string), p.Args["key"].(string), EnvVarWrite{Value: value, GenerateValue: generate})
				return err == nil, err
			},
		},
		"deleteEnvVar": &graphql.Field{
			Type: graphql.Boolean,
			Args: svcKey,
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteEnvVar(p.Context, p.Args["serviceId"].(string), p.Args["key"].(string))
				return err == nil, err
			},
		},
		"setSecretFile": &graphql.Field{ // add or update one secret file (merged)
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"content":   &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				content, _ := p.Args["content"].(string)
				_, err := s.SetSecretFile(p.Context, p.Args["serviceId"].(string), p.Args["name"].(string), content)
				return err == nil, err
			},
		},
		"deleteSecretFile": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"serviceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"name":      &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteSecretFile(p.Context, p.Args["serviceId"].(string), p.Args["name"].(string))
				return err == nil, err
			},
		},
	}
}
