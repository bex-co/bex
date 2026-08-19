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

package github

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// mcp.go is the GitHub-connect MCP fragment: list_repos ("which repos can I
// deploy?") and get_git_connection ("is GitHub connected, and if not how does
// the human connect?"). Both are bex extensions — Render's MCP has no repo
// tools; naming follows Render's list_*/get_* convention.

type listReposResult struct {
	Repos []Repo `json:"repos"`
}

type listConnectionsResult struct {
	Connections []Connection `json:"connections"`
}

// RegisterMCP adds the git-connect tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name: "list_repos",
		Description: "List the GitHub repositories the workspace's connected GitHub App installations can deploy (private repos included; " +
			"each repo carries the accountLogin of the GitHub account it belongs to). " +
			"Use this to answer \"which of my repos can you deploy?\" before creating a service from a repo. " +
			"If it returns an empty list or a 503, GitHub is not connected — call list_git_connections for the install URL to give the human.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listReposResult, error) {
		repos, err := s.ListRepos(ctx, core.NamedWorkspace(ctx))
		return nil, listReposResult{Repos: repos}, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name: "list_git_connections",
		Description: "List the GitHub accounts/organizations this workspace has connected (account login + installation id per connection). " +
			"An empty list means GitHub is not connected — the human connects a new account from the bex dashboard Settings page; " +
			"a 503 means the GitHub App is not configured on this bex deployment.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listConnectionsResult, error) {
		conns, err := s.ListConnections(ctx, core.NamedWorkspace(ctx))
		return nil, listConnectionsResult{Connections: conns}, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name: "get_git_connection",
		Description: "DEPRECATED (use list_git_connections): report whether this workspace has connected GitHub, returning its oldest " +
			"connection only. A 503 means the GitHub App is not configured on this bex deployment.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, Connection, error) {
		conn, err := s.GetConnection(ctx, core.NamedWorkspace(ctx))
		return nil, conn, err
	})
}
