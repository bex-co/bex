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
		"key":   gqlutil.StrField(func(v EnvVarView) any { return v.Key }),
		"value": gqlutil.StrField(func(v EnvVarView) any { return v.Value }),
	},
})

var envGroupFileGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvGroupSecretFile",
	Fields: graphql.Fields{
		"name":    gqlutil.StrField(func(f SecretFileView) any { return f.Name }),
		"content": gqlutil.StrField(func(f SecretFileView) any { return f.Content }),
	},
})

var envGroupGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvGroup",
	Fields: graphql.Fields{
		"id":            gqlutil.StrField(func(g EnvGroupView) any { return g.ID }),
		"name":          gqlutil.StrField(func(g EnvGroupView) any { return g.Name }),
		"ownerId":       gqlutil.StrField(func(g EnvGroupView) any { return g.OwnerID }),
		"environmentId": gqlutil.OptionalStrField(func(g EnvGroupView) any { return g.EnvironmentID }),
		"serviceLinks":  gqlutil.StrsField(func(g EnvGroupView) any { return g.ServiceLinks }),
		"envVars":       gqlutil.Typed(graphql.NewList(envGroupVarGQLType), func(g EnvGroupView) any { return g.EnvVars }),
		"secretFiles":   gqlutil.Typed(graphql.NewList(envGroupFileGQLType), func(g EnvGroupView) any { return g.SecretFiles }),
		"createdAt":     gqlutil.StrField(func(g EnvGroupView) any { return g.CreatedAt }),
		"updatedAt":     gqlutil.StrField(func(g EnvGroupView) any { return g.UpdatedAt }),
		"revision":      gqlutil.StrField(func(g EnvGroupView) any { return g.Revision }),
		"availability":  gqlutil.StrField(func(g EnvGroupView) any { return g.Availability }),
	},
})

var envGroupVarInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "EnvGroupVarInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"key":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"generateValue": &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
	},
})

var envGroupFileInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "EnvGroupSecretFileInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"name":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"content": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

var envGroupVarPatchGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "EnvGroupVarPatchInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"key":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"fromKey":       &graphql.InputObjectFieldConfig{Type: graphql.String},
		"value":         &graphql.InputObjectFieldConfig{Type: graphql.String},
		"generateValue": &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		"delete":        &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
	},
})

var envGroupFilePatchGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "EnvGroupSecretFilePatchInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"name":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"fromName": &graphql.InputObjectFieldConfig{Type: graphql.String},
		"content":  &graphql.InputObjectFieldConfig{Type: graphql.String},
		"delete":   &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
	},
})

var envGroupPatchResultGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "EnvGroupEnvironmentPatchResult",
	Fields: graphql.Fields{
		"envVarKeys":         gqlutil.StrsField(func(r EnvironmentPatchResult) any { return r.EnvVarKeys }),
		"secretFileNames":    gqlutil.StrsField(func(r EnvironmentPatchResult) any { return r.SecretFileNames }),
		"revision":           gqlutil.StrField(func(r EnvironmentPatchResult) any { return r.Revision }),
		"affectedServiceIds": gqlutil.StrsField(func(r EnvironmentPatchResult) any { return r.AffectedServiceIDs }),
		"failedServiceIds":   gqlutil.StrsField(func(r EnvironmentPatchResult) any { return r.FailedServiceIDs }),
		"rolledOut":          gqlutil.BoolField(func(r EnvironmentPatchResult) any { return r.RolledOut }),
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
			ev.ValueSet = true
		}
		if generate, ok := m["generateValue"].(bool); ok {
			ev.GenerateValue = generate
		}
		vars = append(vars, ev)
	}
	return vars
}

func createVarsFromArgs(raw any) []CreateEnvVarInput {
	items, _ := raw.([]any)
	vars := make([]CreateEnvVarInput, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		generate, _ := m["generateValue"].(bool)
		_, valueSet := m["value"]
		vars = append(vars, CreateEnvVarInput{
			Key: gqlutil.Str(m, "key"), Value: gqlutil.Str(m, "value"), ValueSet: valueSet, GenerateValue: generate,
		})
	}
	return vars
}

func createFilesFromArgs(raw any) []SecretFileView {
	items, _ := raw.([]any)
	files := make([]SecretFileView, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		files = append(files, SecretFileView{Name: gqlutil.Str(m, "name"), Content: gqlutil.Str(m, "content")})
	}
	return files
}

func envVarPatchesFromArgs(raw any) []EnvVarPatch {
	items, _ := raw.([]any)
	out := make([]EnvVarPatch, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		generate, _ := m["generateValue"].(bool)
		remove, _ := m["delete"].(bool)
		_, valueSet := m["value"]
		out = append(out, EnvVarPatch{
			Key: gqlutil.Str(m, "key"), FromKey: gqlutil.Str(m, "fromKey"),
			Value: gqlutil.Str(m, "value"), ValueSet: valueSet, GenerateValue: generate, Delete: remove,
		})
	}
	return out
}

func filePatchesFromArgs(raw any) []SecretFilePatch {
	items, _ := raw.([]any)
	out := make([]SecretFilePatch, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		remove, _ := m["delete"].(bool)
		out = append(out, SecretFilePatch{
			Name: gqlutil.Str(m, "name"), FromName: gqlutil.Str(m, "fromName"),
			Content: gqlutil.Str(m, "content"), Delete: remove,
		})
	}
	return out
}

// GraphQLQuery returns the env-group read fields. envGroups' ownerId (w6/m24)
// names the workspace to list; omitted means the caller's default workspace.
// Render's name/environment/timestamp list filters stay REST-only, matching the
// Environments dialect; GraphQL retains its native owner + paging contract.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"envGroups": &graphql.Field{
			Type: graphql.NewList(envGroupGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"cursor":  gqlutil.Arg(graphql.String),
				"limit":   gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				groups, err := s.ListEnvGroups(p.Context, gqlutil.Str(p.Args, "ownerId"))
				if err != nil {
					return nil, err
				}
				cursor, cursorSet := p.Args["cursor"].(string)
				limit, limitSet := p.Args["limit"].(int)
				return pageEnvGroups(groups, cursor, limit, cursorSet || limitSet), nil
			},
		},
		"envGroup":           gqlutil.IDVerb(envGroupGQLType, s.GetEnvGroup),
		"envGroupVar":        gqlutil.ArgMutation(envGroupVarGQLType, "key", s.GetEnvGroupVar),
		"envGroupSecretFile": gqlutil.ArgMutation(envGroupFileGQLType, "name", s.GetEnvGroupFile),
	}
}

// GraphQLMutation returns the env-group write mutations.
func (s *Service) GraphQLMutation() graphql.Fields {
	idService := graphql.FieldConfigArgument{
		"id":        gqlutil.ReqArg(graphql.String),
		"serviceId": gqlutil.ReqArg(graphql.String),
	}
	return graphql.Fields{
		"createEnvGroup": &graphql.Field{
			Type: envGroupGQLType,
			Args: graphql.FieldConfigArgument{
				"name":          gqlutil.ReqArg(graphql.String),
				"ownerId":       gqlutil.Arg(graphql.String),
				"environmentId": gqlutil.Arg(graphql.String),
				"envVars":       gqlutil.Arg(graphql.NewList(graphql.NewNonNull(envGroupVarInputGQLType))),
				"secretFiles":   gqlutil.Arg(graphql.NewList(graphql.NewNonNull(envGroupFileInputGQLType))),
				"serviceIds":    gqlutil.Arg(graphql.NewList(graphql.NewNonNull(graphql.String))),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateEnvGroup(p.Context, CreateEnvGroupRequest{
					Name:          p.Args["name"].(string),
					OwnerID:       gqlutil.Str(p.Args, "ownerId"),
					EnvironmentID: gqlutil.Str(p.Args, "environmentId"),
					EnvVars:       createVarsFromArgs(p.Args["envVars"]),
					SecretFiles:   createFilesFromArgs(p.Args["secretFiles"]),
					ServiceIDs:    gqlutil.StringList(p.Args["serviceIds"]),
				})
			},
		},
		"renameEnvGroup": gqlutil.ArgMutation(envGroupGQLType, "name", s.RenameEnvGroup),
		"moveEnvGroup":   gqlutil.ArgMutation(envGroupGQLType, "environmentId", s.MoveEnvGroup),
		"cloneEnvGroup": &graphql.Field{
			Type: envGroupGQLType,
			Args: graphql.FieldConfigArgument{
				"id":            gqlutil.ReqArg(graphql.String),
				"name":          gqlutil.ReqArg(graphql.String),
				"ownerId":       gqlutil.Arg(graphql.String),
				"environmentId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CloneEnvGroup(p.Context, p.Args["id"].(string), CloneEnvGroupRequest{
					Name: p.Args["name"].(string), OwnerID: gqlutil.Str(p.Args, "ownerId"),
					EnvironmentID: gqlutil.Str(p.Args, "environmentId"),
				})
			},
		},
		"deleteEnvGroup": &graphql.Field{
			Type: graphql.Boolean,
			Args: gqlutil.IDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteEnvGroup(p.Context, p.Args["id"].(string))
				return err == nil, err
			},
		},
		"patchEnvGroupEnvironment": &graphql.Field{
			Type: envGroupPatchResultGQLType,
			Args: graphql.FieldConfigArgument{
				"id":               gqlutil.ReqArg(graphql.String),
				"envVars":          gqlutil.Arg(graphql.NewList(graphql.NewNonNull(envGroupVarPatchGQLType))),
				"secretFiles":      gqlutil.Arg(graphql.NewList(graphql.NewNonNull(envGroupFilePatchGQLType))),
				"saveMode":         gqlutil.ReqArg(graphql.String),
				"expectedRevision": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				patch := EnvironmentPatch{
					EnvVars:     envVarPatchesFromArgs(p.Args["envVars"]),
					SecretFiles: filePatchesFromArgs(p.Args["secretFiles"]),
					SaveMode:    SaveMode(p.Args["saveMode"].(string)),
				}
				if revision, ok := p.Args["expectedRevision"].(string); ok {
					patch.ExpectedRevision = &revision
				}
				return s.PatchEnvironment(p.Context, p.Args["id"].(string), patch)
			},
		},
		"setEnvGroupVars": &graphql.Field{ // replace-all group env vars
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"envVars": gqlutil.Arg(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(envGroupVarInputGQLType)))),
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
				"id":            gqlutil.ReqArg(graphql.String),
				"key":           gqlutil.ReqArg(graphql.String),
				"value":         gqlutil.Arg(graphql.String),
				"generateValue": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				value := gqlutil.Str(p.Args, "value")
				_, valueSet := p.Args["value"]
				generate, _ := p.Args["generateValue"].(bool)
				_, err := s.SetEnvGroupVarInput(p.Context, p.Args["id"].(string), EnvVarView{
					Key: p.Args["key"].(string), Value: value, ValueSet: valueSet, GenerateValue: generate,
				})
				return err == nil, err
			},
		},
		"deleteEnvGroupVar": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":  gqlutil.ReqArg(graphql.String),
				"key": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.DeleteEnvGroupVar(p.Context, p.Args["id"].(string), p.Args["key"].(string))
				return err == nil, err
			},
		},
		"setEnvGroupSecretFile": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"name":    gqlutil.ReqArg(graphql.String),
				"content": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				content := gqlutil.Str(p.Args, "content")
				_, err := s.SetEnvGroupFile(p.Context, p.Args["id"].(string), p.Args["name"].(string), content)
				return err == nil, err
			},
		},
		"deleteEnvGroupSecretFile": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":   gqlutil.ReqArg(graphql.String),
				"name": gqlutil.ReqArg(graphql.String),
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
