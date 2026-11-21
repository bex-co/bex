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
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// mcp.go is the api-key MCP fragment (bex extensions over Render's MCP, which
// manages keys outside its tools; naming follows Render's convention).

// grantArgs is a requested grant (w9/m496) as a tool argument: both fields
// optional, so {all: true} and {grants: [...]} are each complete.
type grantArgs struct {
	All    bool                 `json:"all,omitempty" jsonschema:"every permission your member role allows, now and as your role changes"`
	Grants []core.KeyGrantEntry `json:"grants,omitempty" jsonschema:"the permissions, each at one level, from list_api_key_permissions; empty with all false grants nothing (Render's Clear all)"`
}

func (g grantArgs) grant() core.KeyGrant {
	return core.KeyGrant{All: g.All, Grants: g.Grants}
}

type createAPIKeyArgs struct {
	Name string `json:"name" jsonschema:"human-readable name for the credential"`
	// Permissions is the key's grant; omitted means All.
	Permissions *grantArgs `json:"permissions,omitempty" jsonschema:"the key's permissions; omitted means all: every permission your role allows"`
}

type updateAPIKeyPermissionsArgs struct {
	KeyID       string    `json:"keyId" jsonschema:"the API key id (OAuth2 client_id)"`
	Permissions grantArgs `json:"permissions" jsonschema:"the key's new permissions, replacing its grant"`
}

type listAPIKeyPermissionsResult struct {
	Areas []PermissionArea `json:"areas"`
}

type listAPIKeysArgs struct {
	AllMembers bool `json:"allMembers,omitempty" jsonschema:"list every member's keys in the workspace, not only yours; a workspace Admin only"`
}

type listAPIKeysResult struct {
	APIKeys []APIKey `json:"apiKeys"`
}

type revokeAPIKeyArgs struct {
	KeyID string `json:"keyId" jsonschema:"the API key id (OAuth2 client_id)"`
}

type revokeAPIKeyResult struct {
	Revoked bool `json:"revoked"`
}

// RegisterMCP adds the api-key tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_api_key",
		Description: "Create an API key for the platform API; every member role may, and the key never does more than your role allows. `key` is returned once — store it and send it as `Authorization: Bearer`. `id` and `secret` are the same credential as an OAuth2 client, for a client_credentials exchange. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createAPIKeyArgs) (*mcp.CallToolResult, APIKey, error) {
		var grant *core.KeyGrant
		if in.Permissions != nil {
			g := in.Permissions.grant()
			grant = &g
		}
		key, err := s.CreateAPIKey(ctx, core.NamedWorkspace(ctx), in.Name, grant)
		return nil, key, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_api_key_permissions",
		Description: "Change an API key's permissions, replacing its grant, as Render's gear icon does. Only the key's creator may, within their role; the change applies on the key's next request. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateAPIKeyPermissionsArgs) (*mcp.CallToolResult, APIKey, error) {
		key, err := s.UpdateAPIKeyPermissions(ctx, core.NamedWorkspace(ctx), in.KeyID, in.Permissions.grant())
		return nil, key, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_api_key_permissions",
		Description: "List the permissions an API key can be granted (Render's areas, permissions and levels), each level marked with whether your role lets you grant it. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listAPIKeyPermissionsResult, error) {
		areas, err := s.PermissionCatalog(ctx, core.NamedWorkspace(ctx))
		return nil, listAPIKeyPermissionsResult{Areas: areas}, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_api_keys",
		Description: "List the API keys you created in the workspace (secrets never included). A workspace Admin may set allMembers to list every member's keys. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listAPIKeysArgs) (*mcp.CallToolResult, listAPIKeysResult, error) {
		keys, err := s.ListAPIKeys(ctx, core.NamedWorkspace(ctx), in.AllMembers)
		return nil, listAPIKeysResult{APIKeys: keys}, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "revoke_api_key",
		Description: "Revoke an API key by keyId; its tokens stop working. Any member revokes the keys they created; a workspace Admin revokes any key in the workspace. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in revokeAPIKeyArgs) (*mcp.CallToolResult, revokeAPIKeyResult, error) {
		err := s.RevokeAPIKey(ctx, core.NamedWorkspace(ctx), in.KeyID)
		return nil, revokeAPIKeyResult{Revoked: err == nil}, err
	})
}
