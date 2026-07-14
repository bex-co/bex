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
)

// mcp.go is the GitHub-connect MCP fragment: list_repos ("which repos can I
// deploy?") and get_git_connection ("is GitHub connected, and if not how does
// the human connect?"). Both are bex extensions — Render's MCP has no repo
// tools; naming follows Render's list_*/get_* convention. OwnerID follows the
// shared ownerId precedence every workspace-scoped MCP tool uses
// (core.SelectedWorkspace, w6/m18): explicit arg > the session's
// select_workspace > the caller's default workspace.

type ownerIDArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"workspace id to query; defaults to the selected or caller's default workspace"`
}

type listReposResult struct {
	Repos []Repo `json:"repos"`
}

// RegisterMCP adds the git-connect tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_repos",
		Description: "List the GitHub repositories the connected GitHub App installation can deploy (private repos included). " +
			"Use this to answer \"which of my repos can you deploy?\" before creating a service from a repo. " +
			"If it returns an empty list or a 503, GitHub is not connected — call get_git_connection for the install URL to give the human.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ownerIDArgs) (*mcp.CallToolResult, listReposResult, error) {
		repos, err := s.ListRepos(ctx, core.SelectedWorkspace(s.Selections, req, in.OwnerID))
		return nil, listReposResult{Repos: repos}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_git_connection",
		Description: "Report whether this workspace has connected GitHub (account login + install URL). " +
			"When connected is false, give the human the returned installUrl to install the bex GitHub App and grant repos; " +
			"a 503 means the GitHub App is not configured on this bex deployment.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ownerIDArgs) (*mcp.CallToolResult, Connection, error) {
		conn, err := s.GetConnection(ctx, core.SelectedWorkspace(s.Selections, req, in.OwnerID))
		return nil, conn, err
	})
}
