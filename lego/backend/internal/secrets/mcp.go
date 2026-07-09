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

package secrets

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the env-vars MCP fragment (Render's official MCP has no env-var
// tools, so these are bex extensions named after Render's env-vars REST noun).
// Every tool delegates to the same Service method REST/GraphQL call.

type serviceEnvArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name)"`
}

type envVarKeyArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name)"`
	Key       string `json:"key" jsonschema:"the environment variable name"`
}

type setEnvVarsArgs struct {
	ServiceID string       `json:"serviceId" jsonschema:"the service id (bex App name)"`
	EnvVars   []EnvVarView `json:"envVars" jsonschema:"the complete desired set of {key,value} variables (replace semantics)"`
}

type setEnvVarArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name)"`
	Key       string `json:"key" jsonschema:"the environment variable name"`
	Value     string `json:"value" jsonschema:"the value to set"`
}

type envVarsResult struct {
	EnvVars []EnvVarView `json:"envVars"`
}

type deleteEnvVarResult struct {
	Deleted bool `json:"deleted"`
}

type secretFileNameArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name)"`
	Name      string `json:"name" jsonschema:"the secret file name (mounted at /etc/secrets/<name>)"`
}

type setSecretFileArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name)"`
	Name      string `json:"name" jsonschema:"the secret file name (mounted at /etc/secrets/<name>)"`
	Content   string `json:"content" jsonschema:"the file contents"`
}

type secretFilesResult struct {
	SecretFiles []SecretFileView `json:"secretFiles"`
}

type deleteSecretFileResult struct {
	Deleted bool `json:"deleted"`
}

// RegisterMCP adds the env-var tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_env_vars",
		Description: "List a service's environment variables (key/value), sorted by key.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceEnvArgs) (*mcp.CallToolResult, envVarsResult, error) {
		vars, err := s.ListEnvVars(ctx, in.ServiceID)
		return nil, envVarsResult{EnvVars: vars}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_env_var",
		Description: "Get one environment variable of a service by key (the bare key/value).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in envVarKeyArgs) (*mcp.CallToolResult, EnvVarView, error) {
		v, err := s.GetEnvVar(ctx, in.ServiceID, in.Key)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_env_vars",
		Description: "Replace a service's whole environment-variable set (Render's replace semantics) and return the new set; the service's pods roll to pick up the change.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvVarsArgs) (*mcp.CallToolResult, envVarsResult, error) {
		vars, err := s.SetEnvVars(ctx, in.ServiceID, in.EnvVars)
		return nil, envVarsResult{EnvVars: vars}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_env_var",
		Description: "Add or update one environment variable of a service (merged into the existing set, not a replace); the service's pods roll to pick up the change.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvVarArgs) (*mcp.CallToolResult, EnvVarView, error) {
		v, err := s.SetEnvVar(ctx, in.ServiceID, in.Key, in.Value)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_env_var",
		Description: "Remove one environment variable from a service by key; the service's pods roll to pick up the change.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in envVarKeyArgs) (*mcp.CallToolResult, deleteEnvVarResult, error) {
		err := s.DeleteEnvVar(ctx, in.ServiceID, in.Key)
		return nil, deleteEnvVarResult{Deleted: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_secret_files",
		Description: "List a service's secret files (names only), sorted by name. Files are mounted read-only at /etc/secrets/<name>.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceEnvArgs) (*mcp.CallToolResult, secretFilesResult, error) {
		files, err := s.ListSecretFiles(ctx, in.ServiceID)
		return nil, secretFilesResult{SecretFiles: files}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_secret_file",
		Description: "Get one secret file of a service by name, including its contents.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in secretFileNameArgs) (*mcp.CallToolResult, SecretFileView, error) {
		f, err := s.GetSecretFile(ctx, in.ServiceID, in.Name)
		return nil, f, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_secret_file",
		Description: "Add or update one secret file of a service (mounted at /etc/secrets/<name>); the service's pods roll to pick up the change.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setSecretFileArgs) (*mcp.CallToolResult, SecretFileView, error) {
		f, err := s.SetSecretFile(ctx, in.ServiceID, in.Name, in.Content)
		return nil, f, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_secret_file",
		Description: "Remove one secret file from a service by name; the service's pods roll to pick up the change.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in secretFileNameArgs) (*mcp.CallToolResult, deleteSecretFileResult, error) {
		err := s.DeleteSecretFile(ctx, in.ServiceID, in.Name)
		return nil, deleteSecretFileResult{Deleted: err == nil}, err
	})
}
