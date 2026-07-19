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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the env-groups MCP fragment (Render's official MCP has no env-group
// tools, so these are bex extensions named after Render's env-groups REST noun).
// Every tool delegates to the same Service method the REST/GraphQL surfaces call.
// OwnerID on list/create (w6/m24) follows the shared ownerId precedence every
// workspace-scoped MCP tool uses (core.SelectedWorkspace): explicit arg > the
// session's select_workspace > the caller's default workspace.

type envGroupArgs struct {
	ID string `json:"id" jsonschema:"the env group id (evg-...)"`
}

type listEnvGroupsArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"workspace id to list; defaults to the selected or caller's default workspace"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"opaque cursor from the last group in the previous page"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum groups to return (1-100); omitted returns the complete list for compatibility"`
}

type createEnvGroupArgs struct {
	Name          string              `json:"name" jsonschema:"the env group name"`
	OwnerID       string              `json:"ownerId,omitempty" jsonschema:"workspace id to create the group in; defaults to the selected or caller's default workspace"`
	EnvironmentID string              `json:"environmentId,omitempty" jsonschema:"optional environment id (env-...) in the same workspace to assign this group to"`
	EnvVars       []CreateEnvVarInput `json:"envVars,omitempty" jsonschema:"optional initial {key,value|generateValue} variables"`
	SecretFiles   []SecretFileView    `json:"secretFiles,omitempty" jsonschema:"optional initial {name,content} secret files"`
	ServiceIDs    []string            `json:"serviceIds,omitempty" jsonschema:"optional service ids to link atomically during creation"`
}

type renameEnvGroupArgs struct {
	ID   string `json:"id" jsonschema:"the env group id (evg-...)"`
	Name string `json:"name" jsonschema:"the new env group name"`
}

type setEnvGroupVarsArgs struct {
	ID      string       `json:"id" jsonschema:"the env group id (evg-...)"`
	EnvVars []EnvVarView `json:"envVars" jsonschema:"the complete desired set of {key,value} variables (replace semantics)"`
}

type envGroupVarArgs struct {
	ID  string `json:"id" jsonschema:"the env group id (evg-...)"`
	Key string `json:"key" jsonschema:"the environment variable key"`
}

type setEnvGroupVarArgs struct {
	ID    string `json:"id" jsonschema:"the env group id (evg-...)"`
	Key   string `json:"key" jsonschema:"the environment variable key"`
	Value string `json:"value" jsonschema:"the environment variable value"`
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
		Description: "List one workspace's environment groups with cursor paging (names, linked services, and env-var keys / secret-file names — no values); Render's name, environment, and timestamp filters are REST-only.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listEnvGroupsArgs) (*mcp.CallToolResult, listEnvGroupsResult, error) {
		ownerID, err := core.SelectedWorkspace(ctx, s.Selections, req, in.OwnerID)
		if err != nil {
			return nil, listEnvGroupsResult{}, err
		}
		groups, err := s.ListEnvGroups(ctx, ownerID)
		if err == nil {
			groups = pageEnvGroups(groups, in.Cursor, in.Limit, in.Cursor != "" || in.Limit != 0)
		}
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
		Description: "Create an environment group, optionally with initial variables, secret files, and service links in one atomic operation.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createEnvGroupArgs) (*mcp.CallToolResult, EnvGroupView, error) {
		ownerID, err := core.SelectedWorkspace(ctx, s.Selections, req, in.OwnerID)
		if err != nil {
			return nil, EnvGroupView{}, err
		}
		g, err := s.CreateEnvGroup(ctx, CreateEnvGroupRequest{
			Name: in.Name, OwnerID: ownerID,
			EnvironmentID: in.EnvironmentID, EnvVars: in.EnvVars,
			SecretFiles: in.SecretFiles, ServiceIDs: in.ServiceIDs,
		})
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
		Name:        "rename_env_group",
		Description: "Rename an environment group without changing its id, contents, or service links.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in renameEnvGroupArgs) (*mcp.CallToolResult, EnvGroupView, error) {
		g, err := s.RenameEnvGroup(ctx, in.ID, in.Name)
		return nil, g, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_env_group_vars",
		Description: "Replace an environment group's whole env-var set (replace semantics); every linked service rolls.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvGroupVarsArgs) (*mcp.CallToolResult, okResult, error) {
		_, err := s.SetEnvGroupVars(ctx, in.ID, in.EnvVars)
		return nil, okResult{OK: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_env_group_var",
		Description: "Reveal one environment variable value in an environment group.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in envGroupVarArgs) (*mcp.CallToolResult, EnvVarView, error) {
		v, err := s.GetEnvGroupVar(ctx, in.ID, in.Key)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_env_group_var",
		Description: "Add or update one environment variable without replacing the group's other variables; linked services roll.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvGroupVarArgs) (*mcp.CallToolResult, EnvVarView, error) {
		v, err := s.SetEnvGroupVar(ctx, in.ID, in.Key, in.Value)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_env_group_var",
		Description: "Remove one environment variable from an environment group; linked services roll.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in envGroupVarArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.DeleteEnvGroupVar(ctx, in.ID, in.Key)
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
		Name:        "get_env_group_secret_file",
		Description: "Reveal one secret file's contents in an environment group.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in envGroupFileArgs) (*mcp.CallToolResult, SecretFileView, error) {
		f, err := s.GetEnvGroupFile(ctx, in.ID, in.Name)
		return nil, f, err
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
