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
)

// mcp.go is the api-key MCP fragment (bex extensions over Render's MCP, which
// manages keys outside its tools; naming follows Render's convention). OwnerID
// on each args struct follows the shared ownerId precedence every
// workspace-scoped MCP tool uses (core.SelectedWorkspace, w6/m18): explicit
// arg > the session's select_workspace > the caller's default workspace.

type createAPIKeyArgs struct {
	Name    string `json:"name" jsonschema:"human-readable name for the credential"`
	OwnerID string `json:"ownerId,omitempty" jsonschema:"workspace id to bind the key to; defaults to the selected or caller's default workspace"`
}

type listAPIKeysArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"workspace id to list; defaults to the selected or caller's default workspace"`
}

type listAPIKeysResult struct {
	APIKeys []APIKey `json:"apiKeys"`
}

type revokeAPIKeyArgs struct {
	KeyID   string `json:"keyId" jsonschema:"the API key id (OAuth2 client_id)"`
	OwnerID string `json:"ownerId,omitempty" jsonschema:"the key's workspace id; defaults to the selected or caller's default workspace"`
}

type revokeAPIKeyResult struct {
	Revoked bool `json:"revoked"`
}

// RegisterMCP adds the api-key tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_api_key",
		Description: "Create a machine credential (OAuth2 client) for the platform API. The secret is returned once — store it. bex extension over Render's MCP.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createAPIKeyArgs) (*mcp.CallToolResult, APIKey, error) {
		ownerID, err := core.SelectedWorkspace(ctx, s.Selections, req, in.OwnerID)
		if err != nil {
			return nil, APIKey{}, err
		}
		key, err := s.CreateAPIKey(ctx, ownerID, in.Name)
		return nil, key, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_api_keys",
		Description: "List the platform API's machine credentials (secrets never included). bex extension over Render's MCP.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listAPIKeysArgs) (*mcp.CallToolResult, listAPIKeysResult, error) {
		ownerID, err := core.SelectedWorkspace(ctx, s.Selections, req, in.OwnerID)
		if err != nil {
			return nil, listAPIKeysResult{}, err
		}
		keys, err := s.ListAPIKeys(ctx, ownerID)
		return nil, listAPIKeysResult{APIKeys: keys}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "revoke_api_key",
		Description: "Revoke a machine credential by keyId; its tokens stop working. bex extension over Render's MCP.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in revokeAPIKeyArgs) (*mcp.CallToolResult, revokeAPIKeyResult, error) {
		ownerID, err := core.SelectedWorkspace(ctx, s.Selections, req, in.OwnerID)
		if err != nil {
			return nil, revokeAPIKeyResult{}, err
		}
		err = s.RevokeAPIKey(ctx, ownerID, in.KeyID)
		return nil, revokeAPIKeyResult{Revoked: err == nil}, err
	})
}
