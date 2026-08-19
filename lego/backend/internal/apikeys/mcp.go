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

type createAPIKeyArgs struct {
	Name string `json:"name" jsonschema:"human-readable name for the credential"`
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
		Description: "Create a machine credential (OAuth2 client) for the platform API. The secret is returned once — store it. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createAPIKeyArgs) (*mcp.CallToolResult, APIKey, error) {
		key, err := s.CreateAPIKey(ctx, core.NamedWorkspace(ctx), in.Name)
		return nil, key, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_api_keys",
		Description: "List the platform API's machine credentials (secrets never included). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listAPIKeysResult, error) {
		keys, err := s.ListAPIKeys(ctx, core.NamedWorkspace(ctx))
		return nil, listAPIKeysResult{APIKeys: keys}, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "revoke_api_key",
		Description: "Revoke a machine credential by keyId; its tokens stop working. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in revokeAPIKeyArgs) (*mcp.CallToolResult, revokeAPIKeyResult, error) {
		err := s.RevokeAPIKey(ctx, core.NamedWorkspace(ctx), in.KeyID)
		return nil, revokeAPIKeyResult{Revoked: err == nil}, err
	})
}
