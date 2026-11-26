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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// apiKeyGrantGQLType is one permission of a key's grant at one level
// (w9/m496); the ids are core.KeyPermissionCatalog's.
var apiKeyGrantGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKeyGrant",
	Fields: graphql.Fields{
		"permission": gqlutil.ReqStrField(func(e core.KeyGrantEntry) any { return e.Permission }),
		"level":      gqlutil.ReqStrField(func(e core.KeyGrantEntry) any { return e.Level }),
	},
})

// apiKeyPermissionsGQLType is a key's grant: all, or exactly grants (empty is
// Render's "Clear all").
var apiKeyPermissionsGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKeyPermissions",
	Fields: graphql.Fields{
		"all": gqlutil.ReqBoolField(func(g core.KeyGrant) any { return g.All }),
		"grants": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(apiKeyGrantGQLType))),
			func(g core.KeyGrant) any { return g.Grants }),
	},
})

var apiKeyGrantInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "ApiKeyGrantInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"permission": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"level":      &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
	},
})

// apiKeyPermissionsInputGQLType is a requested grant: all: true (every
// permission the creator's role allows), or grants naming catalog permissions.
var apiKeyPermissionsInputGQLType = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "ApiKeyPermissionsInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"all":    &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		"grants": &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(apiKeyGrantInputGQLType))},
	},
})

// grantArg reads an ApiKeyPermissionsInput argument; nil when absent.
func grantArg(args map[string]any, name string) *core.KeyGrant {
	raw, ok := args[name].(map[string]any)
	if !ok {
		return nil
	}
	g := core.KeyGrant{All: gqlutil.Bool(raw, "all"), Grants: []core.KeyGrantEntry{}}
	list, _ := raw["grants"].([]any)
	for _, item := range list {
		entry, _ := item.(map[string]any)
		g.Grants = append(g.Grants, core.KeyGrantEntry{Permission: gqlutil.Str(entry, "permission"), Level: core.KeyLevel(gqlutil.Str(entry, "level"))})
	}
	return &g
}

var apiKeyPermissionActionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKeyPermissionAction",
	Fields: graphql.Fields{
		"id":   gqlutil.ReqStrField(func(a core.KeyActionInfo) any { return a.ID }),
		"name": gqlutil.ReqStrField(func(a core.KeyActionInfo) any { return a.Name }),
		"bex":  gqlutil.ReqBoolField(func(a core.KeyActionInfo) any { return a.Bex }),
	},
})

var apiKeyPermissionLevelGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKeyPermissionLevel",
	Fields: graphql.Fields{
		"level":        gqlutil.ReqStrField(func(l PermissionLevelInfo) any { return l.Level }),
		"name":         gqlutil.ReqStrField(func(l PermissionLevelInfo) any { return l.Name }),
		"grantable":    gqlutil.ReqBoolField(func(l PermissionLevelInfo) any { return l.Grantable }),
		"requiredRole": gqlutil.ReqStrField(func(l PermissionLevelInfo) any { return l.RequiredRole }),
		"actions": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(apiKeyPermissionActionGQLType))),
			func(l PermissionLevelInfo) any { return l.Actions }),
	},
})

var apiKeyPermissionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKeyPermission",
	Fields: graphql.Fields{
		"id":   gqlutil.ReqStrField(func(p PermissionInfo) any { return p.ID }),
		"name": gqlutil.ReqStrField(func(p PermissionInfo) any { return p.Name }),
		"levels": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(apiKeyPermissionLevelGQLType))),
			func(p PermissionInfo) any { return p.Levels }),
	},
})

// apiKeyPermissionAreaGQLType is one area of the permission picker.
var apiKeyPermissionAreaGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKeyPermissionArea",
	Fields: graphql.Fields{
		"id":   gqlutil.ReqStrField(func(a PermissionArea) any { return a.ID }),
		"name": gqlutil.ReqStrField(func(a PermissionArea) any { return a.Name }),
		"bex":  gqlutil.ReqBoolField(func(a PermissionArea) any { return a.Bex }),
		"permissions": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(apiKeyPermissionGQLType))),
			func(a PermissionArea) any { return a.Permissions }),
	},
})

// apiKeyGQLType mirrors the REST APIKey object (bex extension; Render's dashboard
// manages keys outside its public schemas). secret and key (the bearer form,
// docs/ADR012-auth.md §8e) are non-empty only in the createApiKey payload — list
// resolves them empty.
var apiKeyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ApiKey",
	Fields: graphql.Fields{
		"id":         gqlutil.StrField(func(k APIKey) any { return k.ID }),
		"name":       gqlutil.StrField(func(k APIKey) any { return k.Name }),
		"secret":     gqlutil.StrField(func(k APIKey) any { return k.Secret }),
		"key":        gqlutil.StrField(func(k APIKey) any { return k.Key }),
		"createdAt":  gqlutil.StrField(func(k APIKey) any { return k.CreatedAt }),
		"createdBy":  gqlutil.StrField(func(k APIKey) any { return k.CreatedBy }),
		"lastUsedAt": gqlutil.StrField(func(k APIKey) any { return k.LastUsedAt }),
		"permissions": gqlutil.Typed(graphql.NewNonNull(apiKeyPermissionsGQLType),
			func(k APIKey) any { return k.Permissions }),
	},
})

// provisionedOrganizationToken is provisionOrganizationAuditLogToken's
// answer, Render's { token }.
type provisionedOrganizationToken struct {
	Token string
}

var provisionedOrganizationTokenGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ProvisionedOrganizationAPIToken",
	Fields: graphql.Fields{
		"token": gqlutil.ReqStrField(func(t provisionedOrganizationToken) any { return t.Token }),
	},
})

// GraphQLQuery returns the apiKeys list query. ownerId (w6/m18) names the
// workspace to list; omitted means the caller's default workspace. The list
// is the caller's own keys; allMembers: true is an Admin's view of every key
// in the workspace (w9/m539).
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"apiKeys": &graphql.Field{
			Type: graphql.NewList(apiKeyGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId":    gqlutil.Arg(graphql.String),
				"allMembers": gqlutil.Arg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListAPIKeys(p.Context, gqlutil.Str(p.Args, "ownerId"), gqlutil.Bool(p.Args, "allMembers"))
			},
		},
		// apiKeyPermissionCatalog is the permission picker's catalog, each
		// level marked with whether the caller's role may grant it (w9/m496).
		"apiKeyPermissionCatalog": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(apiKeyPermissionAreaGQLType))),
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PermissionCatalog(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
	}
}

// GraphQLMutation returns the createApiKey / updateApiKeyPermissions /
// revokeApiKey mutations. ownerId
// (w6/m18) names the workspace createApiKey binds into / revokeApiKey's key
// must belong to; omitted means the caller's default workspace.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createApiKey": &graphql.Field{
			Type: apiKeyGQLType,
			Args: graphql.FieldConfigArgument{
				"name":    gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
				// Omitted means All: every permission the creator's role allows.
				"permissions": gqlutil.Arg(apiKeyPermissionsInputGQLType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateAPIKey(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["name"].(string), grantArg(p.Args, "permissions"))
			},
		},
		// updateApiKeyPermissions changes a key's grant (Render's gear icon).
		"updateApiKeyPermissions": &graphql.Field{
			Type: apiKeyGQLType,
			Args: graphql.FieldConfigArgument{
				"id":          gqlutil.ReqArg(graphql.String),
				"ownerId":     gqlutil.Arg(graphql.String),
				"permissions": gqlutil.ReqArg(apiKeyPermissionsInputGQLType),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdateAPIKeyPermissions(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["id"].(string), *grantArg(p.Args, "permissions"))
			},
		},
		// An organization's audit-log keys (w1/m657): Render's two
		// mutations, its argument names and its answers. The list is
		// Organization.organizationAPITokens (internal/workspaces).
		"provisionOrganizationAuditLogToken": &graphql.Field{
			Type:        graphql.NewNonNull(provisionedOrganizationTokenGQLType),
			Description: "Create an organization API key with read-only access to the audit logs of the organization and all of its workspaces (Org Owners). token, the key's bearer form, is shown only here.",
			Args: graphql.FieldConfigArgument{
				"organizationId": gqlutil.ReqArg(graphql.String),
				"label":          gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				token, err := s.ProvisionOrganizationAuditLogToken(p.Context, p.Args["organizationId"].(string), p.Args["label"].(string))
				if err != nil {
					return nil, err
				}
				return provisionedOrganizationToken{Token: token}, nil
			},
		},
		"revokeOrganizationAPIToken": &graphql.Field{
			Type:        graphql.Boolean,
			Description: "Revoke one of an organization's API keys (Org Owners); it stops at once.",
			Args: graphql.FieldConfigArgument{
				"organizationId": gqlutil.ReqArg(graphql.String),
				"tokenId":        gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.RevokeOrganizationAPIToken(p.Context, p.Args["organizationId"].(string), p.Args["tokenId"].(string))
				return err == nil, err
			},
		},
		"revokeApiKey": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				err := s.RevokeAPIKey(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["id"].(string))
				return err == nil, err
			},
		},
	}
}
