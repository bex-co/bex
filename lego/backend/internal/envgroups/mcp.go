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
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the env-groups MCP fragment (Render's official MCP has no env-group
// tools, so these are bex extensions named after Render's env-groups REST noun).
// Every tool delegates to the same Service method the REST/GraphQL surfaces call.

type envGroupArgs struct {
	ID string `json:"id" jsonschema:"the env group id (evg-...)"`
}

type createEnvGroupArgs struct {
	Name string `json:"name" jsonschema:"the env group name"`
}

type setEnvGroupVarsArgs struct {
	ID      string       `json:"id" jsonschema:"the env group id (evg-...)"`
	EnvVars []EnvVarView `json:"envVars" jsonschema:"the complete desired set of {key,value} variables (replace semantics)"`
}

type setEnvGroupFileArgs struct {
	ID      string `json:"id" jsonschema:"the env group id (evg-...)"`
	Name    string `json:"name" jsonschema:"the secret file name (mounted at /etc/secrets/<name> on linked services)"`
	Content string `json:"content" jsonschema:"the file contents"`
}

type envGroupFileArgs struct {
	ID   string `json:"id" jsonschema:"the env group id (evg-...)"`
	Name string `json:"name" jsonschema:"the secret file name"`
}

type linkEnvGroupArgs struct {
	ID        string `json:"id" jsonschema:"the env group id (evg-...)"`
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name) to link/unlink"`
}

type listEnvGroupsResult struct {
	EnvGroups []EnvGroupView `json:"envGroups"`
}

type okResult struct {
	OK bool `json:"ok"`
}

// RegisterMCP adds the env-group tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_env_groups",
		Description: "List all environment groups (names, linked services, and env-var keys / secret-file names — no values).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listEnvGroupsResult, error) {
		groups, err := s.ListEnvGroups(ctx)
		return nil, listEnvGroupsResult{EnvGroups: groups}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_env_group",
		Description: "Get one environment group by id (keys/names + linked services, no values).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in envGroupArgs) (*mcp.CallToolResult, EnvGroupView, error) {
		g, err := s.GetEnvGroup(ctx, in.ID)
		return nil, g, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_env_group",
		Description: "Create an environment group with a name; add vars/files and link services afterward.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createEnvGroupArgs) (*mcp.CallToolResult, EnvGroupView, error) {
		g, err := s.CreateEnvGroup(ctx, in.Name)
		return nil, g, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_env_group",
		Description: "Delete an environment group; it is unlinked from every service first, and linked services roll.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in envGroupArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.DeleteEnvGroup(ctx, in.ID)
		return nil, okResult{OK: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_env_group_vars",
		Description: "Replace an environment group's whole env-var set (replace semantics); every linked service rolls.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvGroupVarsArgs) (*mcp.CallToolResult, okResult, error) {
		_, err := s.SetEnvGroupVars(ctx, in.ID, in.EnvVars)
		return nil, okResult{OK: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_env_group_secret_file",
		Description: "Add or update one secret file in an environment group (mounted at /etc/secrets/<name> on linked services); linked services roll.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvGroupFileArgs) (*mcp.CallToolResult, SecretFileView, error) {
		f, err := s.SetEnvGroupFile(ctx, in.ID, in.Name, in.Content)
		return nil, f, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_env_group_secret_file",
		Description: "Remove one secret file from an environment group; linked services roll.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in envGroupFileArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.DeleteEnvGroupFile(ctx, in.ID, in.Name)
		return nil, okResult{OK: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "link_env_group",
		Description: "Link an environment group to a service: the service gains the group's env vars + secret files and rolls.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in linkEnvGroupArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.LinkService(ctx, in.ID, in.ServiceID)
		return nil, okResult{OK: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unlink_env_group",
		Description: "Unlink an environment group from a service: the group's vars + files are removed from the service and it rolls.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in linkEnvGroupArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.UnlinkService(ctx, in.ID, in.ServiceID)
		return nil, okResult{OK: err == nil}, err
	})
}
