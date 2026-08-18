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
		"id":    gqlutil.StrField(func(v EnvVarView) any { return v.Key }),
		"key":   gqlutil.StrField(func(v EnvVarView) any { return v.Key }),
		"value": gqlutil.StrField(func(v EnvVarView) any { return v.Value }),
	},
})

var envVarWithCursorGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvVarWithCursor",
	Fields: graphql.Fields{
		"envVar": gqlutil.Typed(envVarListValueGQLType, func(v envVarWithCursor) any { return v.EnvVar }),
		"cursor": gqlutil.StrField(func(v envVarWithCursor) any { return v.Cursor }),
	},
})

var secretFileListValueGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SecretFileListValue",
	Fields: graphql.Fields{
		"id":   gqlutil.StrField(func(f SecretFileView) any { return f.Name }),
		"name": gqlutil.StrField(func(f SecretFileView) any { return f.Name }),
	},
})

var secretFileWithCursorGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SecretFileWithCursor",
	Fields: graphql.Fields{
		"secretFile": gqlutil.Typed(secretFileListValueGQLType, func(f secretFileWithCursor) any { return f.SecretFile }),
		"cursor":     gqlutil.StrField(func(f secretFileWithCursor) any { return f.Cursor }),
	},
})

var envVarPatchGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "EnvironmentEnvVarPatchInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"key":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"fromKey":       &graphql.InputObjectFieldConfig{Type: graphql.String},
		"value":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"generateValue": &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		"delete":        &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
	},
})

var secretFilePatchGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "EnvironmentSecretFilePatchInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"name":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"fromName": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"content":  &graphql.InputObjectFieldConfig{Type: graphql.String},
		"delete":   &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
	},
})

var environmentPatchResultGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvironmentPatchResult",
	Fields: graphql.Fields{
		"envVarKeys":      gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(v EnvironmentPatchResult) any { return v.EnvVarKeys }),
		"secretFileNames": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(v EnvironmentPatchResult) any { return v.SecretFileNames }),
		"rolledOut":       gqlutil.ReqBoolField(func(v EnvironmentPatchResult) any { return v.RolledOut }),
	},
})

func environmentPatchFromArgs(p graphql.ResolveParams) EnvironmentPatch {
	patch := EnvironmentPatch{SaveMode: SaveMode(p.Args["saveMode"].(string))}
	if revision, ok := p.Args["expectedEnvRevision"].(string); ok {
		patch.ExpectedEnvRevision = &revision
	}
	if raw, ok := p.Args["envVars"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			write := EnvVarPatch{}
			write.Key, _ = m["key"].(string)
			write.FromKey, _ = m["fromKey"].(string)
			write.Value, _ = m["value"].(string)
			_, write.ValueSet = m["value"]
			write.GenerateValue, _ = m["generateValue"].(bool)
			write.Delete, _ = m["delete"].(bool)
			patch.EnvVars = append(patch.EnvVars, write)
		}
	}
	if raw, ok := p.Args["secretFiles"].([]any); ok {
		for _, item := range raw {
			m, _ := item.(map[string]any)
			write := SecretFilePatch{}
			write.Name, _ = m["name"].(string)
			write.FromName, _ = m["fromName"].(string)
			write.Content, _ = m["content"].(string)
			write.Delete, _ = m["delete"].(bool)
			patch.SecretFiles = append(patch.SecretFiles, write)
		}
	}
	return patch
}

// GraphQLQuery exposes the paged env-var and secret-file lists as the GraphQL
// twins of Render's REST envelopes. The existing nested service.envVarKeys /
// service.secretFileNames fields remain intact for old dashboard clients; new
// clients use envVars(serviceId,cursor,limit) / secretFiles(serviceId,cursor,limit).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"envVars": &graphql.Field{
			Type: graphql.NewList(envVarWithCursorGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"cursor":    gqlutil.Arg(graphql.String),
				"limit":     gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				cursor := gqlutil.Str(p.Args, "cursor")
				limit, err := gqlutil.PositiveLimit(p.Args)
				if err != nil {
					return nil, err
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
				"serviceId": gqlutil.ReqArg(graphql.String),
				"cursor":    gqlutil.Arg(graphql.String),
				"limit":     gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				cursor := gqlutil.Str(p.Args, "cursor")
				limit, err := gqlutil.PositiveLimit(p.Args)
				if err != nil {
					return nil, err
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
		"serviceId": gqlutil.ReqArg(graphql.String),
		"key":       gqlutil.ReqArg(graphql.String),
	}
	return graphql.Fields{
		"patchServiceEnvironment": &graphql.Field{
			Type: graphql.NewNonNull(environmentPatchResultGQLType),
			Args: graphql.FieldConfigArgument{
				"serviceId":           gqlutil.ReqArg(graphql.String),
				"envVars":             gqlutil.Arg(graphql.NewList(graphql.NewNonNull(envVarPatchGQLType))),
				"secretFiles":         gqlutil.Arg(graphql.NewList(graphql.NewNonNull(secretFilePatchGQLType))),
				"saveMode":            gqlutil.ReqArg(graphql.String),
				"expectedEnvRevision": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PatchEnvironment(p.Context, p.Args["serviceId"].(string), environmentPatchFromArgs(p))
			},
		},
		"setEnvVars": &graphql.Field{ // Render's replace-all
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"envVars":   gqlutil.Arg(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(gqlutil.EnvVarInputType)))),
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
				"serviceId":     gqlutil.ReqArg(graphql.String),
				"key":           gqlutil.ReqArg(graphql.String),
				"value":         gqlutil.Arg(graphql.String),
				"generateValue": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				value := gqlutil.Str(p.Args, "value")
				generate := gqlutil.Bool(p.Args, "generateValue")
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
				"serviceId": gqlutil.ReqArg(graphql.String),
				"name":      gqlutil.ReqArg(graphql.String),
				"content":   gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				content := gqlutil.Str(p.Args, "content")
				_, err := s.SetSecretFile(p.Context, p.Args["serviceId"].(string), p.Args["name"].(string), content)
				return err == nil, err
			},
		},
		"deleteSecretFile": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"serviceId": gqlutil.ReqArg(graphql.String),
				"name":      gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteSecretFile(p.Context, p.Args["serviceId"].(string), p.Args["name"].(string))
				return err == nil, err
			},
		},
	}
}
