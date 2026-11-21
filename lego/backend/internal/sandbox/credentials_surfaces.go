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

package sandbox

import (
	"context"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// GraphQL and MCP surfaces for sandbox group credentials (w2/m533, ADR122).
// Both delegate directly to Service's shared verbs so REST, GraphQL, and MCP
// enforce identical validation, write-only OpenBao custody, split RBAC,
// scoped-key permissions, and coded errors.

var sandboxGroupCredentialInjectionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxGroupCredentialInjection",
	Fields: graphql.Fields{
		"type":     gqlutil.ReqStrField(func(i GroupCredentialInjection) any { return i.Type }),
		"name":     gqlutil.OptionalStrField(func(i GroupCredentialInjection) any { return i.Name }),
		"template": gqlutil.OptionalStrField(func(i GroupCredentialInjection) any { return i.Template }),
	},
})

var sandboxGroupCredentialVersionGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxGroupCredentialVersion",
	Fields: graphql.Fields{
		"id":          gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(v GroupCredentialVersion) any { return int(v.ID) }),
		"updatedAt":   gqlutil.TimeField(func(v GroupCredentialVersion) any { return v.UpdatedAt }),
		"updatedBy":   gqlutil.ReqStrField(func(v GroupCredentialVersion) any { return v.UpdatedBy }),
		"fingerprint": gqlutil.ReqStrField(func(v GroupCredentialVersion) any { return v.Fingerprint }),
		"restoredFromVersionId": gqlutil.IntField(func(v GroupCredentialVersion) any {
			if v.RestoredFromVersionID == nil {
				return nil
			}
			return int(*v.RestoredFromVersionID)
		}),
	},
})

// sandboxGroupCredentialGQLType mirrors GroupCredential with no secret value field.
var sandboxGroupCredentialGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxGroupCredential",
	Fields: graphql.Fields{
		"id":        gqlutil.ReqStrField(func(c GroupCredential) any { return c.ID }),
		"domain":    gqlutil.ReqStrField(func(c GroupCredential) any { return c.Domain }),
		"version":   gqlutil.Typed(graphql.NewNonNull(sandboxGroupCredentialVersionGQLType), func(c GroupCredential) any { return c.Version }),
		"enabled":   gqlutil.ReqBoolField(func(c GroupCredential) any { return c.Enabled }),
		"methods":   gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(c GroupCredential) any { return c.Methods }),
		"paths":     gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(c GroupCredential) any { return c.Paths }),
		"injection": gqlutil.Typed(graphql.NewNonNull(sandboxGroupCredentialInjectionGQLType), func(c GroupCredential) any { return c.Injection }),
	},
})

var sandboxGroupCredentialInjectionInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "SandboxGroupCredentialInjectionInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"type":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"name":     &graphql.InputObjectFieldConfig{Type: graphql.String},
		"template": &graphql.InputObjectFieldConfig{Type: graphql.String},
	},
})

var createSandboxGroupCredentialInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "CreateSandboxGroupCredentialInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"id":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"domain":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"value":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		"enabled":   &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		"methods":   &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
		"paths":     &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
		"injection": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(sandboxGroupCredentialInjectionInput)},
	},
})

var updateSandboxGroupCredentialInput = graphql.NewInputObject(graphql.InputObjectConfig{
	Name: "UpdateSandboxGroupCredentialInput",
	Fields: graphql.InputObjectConfigFieldMap{
		"value":     &graphql.InputObjectFieldConfig{Type: graphql.String},
		"domain":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		"enabled":   &graphql.InputObjectFieldConfig{Type: graphql.Boolean},
		"methods":   &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
		"paths":     &graphql.InputObjectFieldConfig{Type: graphql.NewList(graphql.NewNonNull(graphql.String))},
		"injection": &graphql.InputObjectFieldConfig{Type: sandboxGroupCredentialInjectionInput},
	},
})

func credentialInjectionFromArgs(raw any) *GroupCredentialInjection {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return &GroupCredentialInjection{
		Type:     gqlutil.Str(m, "type"),
		Name:     gqlutil.Str(m, "name"),
		Template: gqlutil.Str(m, "template"),
	}
}

func (s *Service) credentialQueryFields() graphql.Fields {
	return graphql.Fields{
		"sandboxGroupCredentials": &graphql.Field{
			Type: graphql.NewList(graphql.NewNonNull(sandboxGroupCredentialGQLType)),
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListGroupCredentials(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string))
			},
		},
		"sandboxGroupCredential": &graphql.Field{
			Type: sandboxGroupCredentialGQLType,
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"id":             gqlutil.ReqArg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetGroupCredential(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), p.Args["id"].(string))
			},
		},
		"sandboxGroupCredentialVersions": &graphql.Field{
			Type: graphql.NewList(graphql.NewNonNull(sandboxGroupCredentialVersionGQLType)),
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"id":             gqlutil.ReqArg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListGroupCredentialVersions(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), p.Args["id"].(string))
			},
		},
	}
}

func (s *Service) credentialMutationFields() graphql.Fields {
	return graphql.Fields{
		"createSandboxGroupCredential": &graphql.Field{
			Type: sandboxGroupCredentialGQLType,
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"input":          gqlutil.ReqArg(createSandboxGroupCredentialInput),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				in, _ := p.Args["input"].(map[string]any)
				return s.CreateGroupCredential(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), CreateGroupCredentialRequest{
					ID:        gqlutil.Str(in, "id"),
					Domain:    gqlutil.Str(in, "domain"),
					Value:     gqlutil.Str(in, "value"),
					Enabled:   gqlutil.BoolPtr(in, "enabled"),
					Methods:   gqlutil.StringList(in["methods"]),
					Paths:     gqlutil.StringList(in["paths"]),
					Injection: credentialInjectionFromArgs(in["injection"]),
				})
			},
		},
		"updateSandboxGroupCredential": &graphql.Field{
			Type: sandboxGroupCredentialGQLType,
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"id":             gqlutil.ReqArg(graphql.String),
				"input":          gqlutil.ReqArg(updateSandboxGroupCredentialInput),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				in, _ := p.Args["input"].(map[string]any)
				req := UpdateGroupCredentialRequest{
					Value:     gqlutil.StrPtr(in, "value"),
					Domain:    gqlutil.StrPtr(in, "domain"),
					Enabled:   gqlutil.BoolPtr(in, "enabled"),
					Injection: credentialInjectionFromArgs(in["injection"]),
				}
				if raw, ok := in["methods"].([]any); ok {
					list := gqlutil.StringList(raw)
					req.Methods = &list
				}
				if raw, ok := in["paths"].([]any); ok {
					list := gqlutil.StringList(raw)
					req.Paths = &list
				}
				return s.UpdateGroupCredential(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), p.Args["id"].(string), req)
			},
		},
		"deleteSandboxGroupCredential": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"id":             gqlutil.ReqArg(graphql.String),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.DeleteGroupCredential(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), p.Args["id"].(string)); err != nil {
					return false, err
				}
				return true, nil
			},
		},
		"restoreSandboxGroupCredential": &graphql.Field{
			Type: sandboxGroupCredentialGQLType,
			Args: graphql.FieldConfigArgument{
				"sandboxGroupId": gqlutil.ReqArg(graphql.String),
				"id":             gqlutil.ReqArg(graphql.String),
				"versionId":      gqlutil.ReqArg(graphql.Int),
				"ownerId":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RestoreGroupCredential(gqlutil.OwnerScoped(p), p.Args["sandboxGroupId"].(string), p.Args["id"].(string), int64(p.Args["versionId"].(int)))
			},
		},
	}
}

type listGroupCredentialsArgs struct {
	SandboxGroupID string `json:"sandboxGroupId"`
}

type listGroupCredentialsResult struct {
	Credentials []GroupCredential `json:"credentials"`
}

type getGroupCredentialArgs struct {
	SandboxGroupID string `json:"sandboxGroupId"`
	CredentialID   string `json:"credentialId"`
}

type listGroupCredentialVersionsResult struct {
	Versions []GroupCredentialVersion `json:"versions"`
}

type createGroupCredentialArgs struct {
	SandboxGroupID string `json:"sandboxGroupId"`
	CreateGroupCredentialRequest
}

type updateGroupCredentialArgs struct {
	SandboxGroupID string `json:"sandboxGroupId"`
	CredentialID   string `json:"credentialId"`
	UpdateGroupCredentialRequest
}

type restoreGroupCredentialArgs struct {
	SandboxGroupID string `json:"sandboxGroupId"`
	CredentialID   string `json:"credentialId"`
	VersionID      int64  `json:"versionId"`
}

func (s *Service) registerCredentialMCP(server *mcp.Server) {
	readOnly := func() *mcp.ToolAnnotations {
		return &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: new(false),
			OpenWorldHint:   new(false),
		}
	}
	addTool(s, server, &mcp.Tool{
		Name:        "list_sandbox_group_credentials",
		Description: "List all credentials configured on a sandbox group, sorted lexicographically by id. Returns routing, injection settings, enabled state, and current version metadata; secret values are write-only and never returned.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listGroupCredentialsArgs) (*mcp.CallToolResult, listGroupCredentialsResult, error) {
		out, err := s.ListGroupCredentials(ctx, in.SandboxGroupID)
		return nil, listGroupCredentialsResult{Credentials: out}, err
	})

	addTool(s, server, &mcp.Tool{
		Name:        "get_sandbox_group_credential",
		Description: "Get one sandbox group credential's routing, injection settings, enabled state, and current version metadata by sandboxGroupId and credentialId. Secret values are write-only and never returned.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getGroupCredentialArgs) (*mcp.CallToolResult, GroupCredential, error) {
		cred, err := s.GetGroupCredential(ctx, in.SandboxGroupID, in.CredentialID)
		return nil, cred, err
	})

	addTool(s, server, &mcp.Tool{
		Name:        "list_sandbox_group_credential_versions",
		Description: "List retained versions of a sandbox group credential, newest version id first, including updatedAt, updatedBy, fingerprint, and restoredFromVersionId. Secret values are never returned.",
		Annotations: readOnly(),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getGroupCredentialArgs) (*mcp.CallToolResult, listGroupCredentialVersionsResult, error) {
		out, err := s.ListGroupCredentialVersions(ctx, in.SandboxGroupID, in.CredentialID)
		return nil, listGroupCredentialVersionsResult{Versions: out}, err
	})

	addTool(s, server, &mcp.Tool{
		Name:        "create_sandbox_group_credential",
		Description: "Create a write-only outbound credential on a sandbox group (workspace Admin required). id is case-sensitive ([A-Za-z0-9._~-]+, never ., .., or rnd-*). domain must be a public DNS hostname. value is write-only (up to 5 KiB UTF-8) and never returned. injection.type is header, basic, cookie, or query (name is required for header/cookie/query and forbidden for basic; template defaults to ${SECRET}). methods and paths default to matching all requests.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(false),
			OpenWorldHint:   new(true),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createGroupCredentialArgs) (*mcp.CallToolResult, GroupCredential, error) {
		cred, err := s.CreateGroupCredential(ctx, in.SandboxGroupID, in.CreateGroupCredentialRequest)
		return nil, cred, err
	})

	addTool(s, server, &mcp.Tool{
		Name:        "update_sandbox_group_credential",
		Description: "Update a sandbox group credential. Omitted fields keep their current values; providing injection replaces the entire injection object; passing empty methods or paths clears that filter. Developers may update injection, methods, paths, and enabled; changing domain or value requires a workspace Admin. Creates a new version when configuration or value changes; secret values are never returned.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(true),
			OpenWorldHint:   new(true),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateGroupCredentialArgs) (*mcp.CallToolResult, GroupCredential, error) {
		cred, err := s.UpdateGroupCredential(ctx, in.SandboxGroupID, in.CredentialID, in.UpdateGroupCredentialRequest)
		return nil, cred, err
	})

	addTool(s, server, &mcp.Tool{
		Name:        "delete_sandbox_group_credential",
		Description: "Delete a sandbox group credential and purge all of its retained versions and secret values (workspace Admin required). Idempotent: deleting an absent credential in an existing group succeeds.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(true),
			IdempotentHint:  true,
			OpenWorldHint:   new(true),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getGroupCredentialArgs) (*mcp.CallToolResult, deleteResult, error) {
		if err := s.DeleteGroupCredential(ctx, in.SandboxGroupID, in.CredentialID); err != nil {
			return nil, deleteResult{}, err
		}
		return nil, deleteResult{ID: in.CredentialID, Deleted: true}, nil
	})

	addTool(s, server, &mcp.Tool{
		Name:        "restore_sandbox_group_credential",
		Description: "Restore a retained version of a sandbox group credential by versionId as a new current version with restoredFromVersionId set (workspace Admin required). Secret values are never returned.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: new(true),
			OpenWorldHint:   new(true),
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in restoreGroupCredentialArgs) (*mcp.CallToolResult, GroupCredential, error) {
		cred, err := s.RestoreGroupCredential(ctx, in.SandboxGroupID, in.CredentialID, in.VersionID)
		return nil, cred, err
	})
}
