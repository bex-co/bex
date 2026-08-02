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
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the env-vars MCP fragment (Render's official MCP has no env-var
// tools, so these are bex extensions named after Render's env-vars REST noun).
// Every tool delegates to the same Service method REST/GraphQL call.

type serviceEnvArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name)"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"resume after this cursor (the cursor of the previous list_env_vars result); omit for the first page"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum variables to return (1-100); omit for the complete list"`
}

type envVarKeyArgs struct {
	ServiceID string `json:"serviceId" jsonschema:"the service id (bex App name)"`
	Key       string `json:"key" jsonschema:"the environment variable name"`
}

type setEnvVarsArgs struct {
	ServiceID string           `json:"serviceId" jsonschema:"the service id (bex App name)"`
	EnvVars   []envVarWriteArg `json:"envVars" jsonschema:"the complete desired set of value-or-generate variables (replace semantics)"`
}

type envVarWriteArg struct {
	Key           string `json:"key" jsonschema:"the environment variable name"`
	Value         string `json:"value,omitempty" jsonschema:"the value to set; omit when generateValue is true"`
	GenerateValue bool   `json:"generateValue,omitempty" jsonschema:"mint a base64-encoded 256-bit value server-side; cannot be combined with value"`
}

type setEnvVarArgs struct {
	ServiceID     string `json:"serviceId" jsonschema:"the service id (bex App name)"`
	Key           string `json:"key" jsonschema:"the environment variable name"`
	Value         string `json:"value,omitempty" jsonschema:"the value to set; omit when generateValue is true"`
	GenerateValue bool   `json:"generateValue,omitempty" jsonschema:"mint a base64-encoded 256-bit value server-side; cannot be combined with value"`
}

type envVarsResult struct {
	EnvVars []EnvVarView `json:"envVars"`
	Cursor  string       `json:"cursor,omitempty"`
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
	Cursor      string           `json:"cursor,omitempty"`
}

type deleteSecretFileResult struct {
	Deleted bool `json:"deleted"`
}

type patchEnvironmentArgs struct {
	ServiceID           string            `json:"serviceId" jsonschema:"the service id (bex App name)"`
	EnvVars             []EnvVarPatch     `json:"envVars,omitempty" jsonschema:"sparse environment-variable set, generate, or delete operations"`
	SecretFiles         []SecretFilePatch `json:"secretFiles,omitempty" jsonschema:"sparse secret-file set or delete operations"`
	SaveMode            SaveMode          `json:"saveMode" jsonschema:"save_only projects configuration without a rollout; deploy rolls exactly once"`
	ExpectedEnvRevision *string           `json:"expectedEnvRevision,omitempty" jsonschema:"optional opaque revision from list_env_vars or get_env_var; when set, exactly one ordinary env value update is allowed"`
}

// environmentMCPError keeps the same stable domain code visible when the MCP
// SDK serializes an error to text. REST exposes code and GraphQL exposes
// extensions.code; without this wrapper MCP would retain only Error().
func environmentMCPError(err error) error {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return fmt.Errorf("%s: %w", coded.Code, err)
	}
	return err
}

// RegisterMCP adds the env-var tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "patch_service_environment",
		Description: "Apply one sparse env-var and secret-file patch without returning secret material; save_only causes no rollout and deploy rolls the service once.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in patchEnvironmentArgs) (*mcp.CallToolResult, EnvironmentPatchResult, error) {
		result, err := s.PatchEnvironment(ctx, in.ServiceID, EnvironmentPatch{EnvVars: in.EnvVars, SecretFiles: in.SecretFiles, SaveMode: in.SaveMode, ExpectedEnvRevision: in.ExpectedEnvRevision})
		return nil, result, environmentMCPError(err)
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_env_vars",
		Description: "List a service's environment variables (key/value), sorted by key.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in serviceEnvArgs) (*mcp.CallToolResult, envVarsResult, error) {
		vars, err := s.ListEnvVarsPage(ctx, in.ServiceID, in.Cursor, in.Limit)
		result := envVarsResult{EnvVars: vars}
		if (in.Limit > 0 || in.Cursor != "") && len(vars) > 0 {
			result.Cursor = vars[len(vars)-1].Key
		}
		return nil, result, err
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
		writes := make([]EnvVarView, 0, len(in.EnvVars))
		for _, v := range in.EnvVars {
			writes = append(writes, EnvVarView{Key: v.Key, Value: v.Value, Generate: v.GenerateValue})
		}
		vars, err := s.SetEnvVars(ctx, in.ServiceID, writes)
		return nil, envVarsResult{EnvVars: vars}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_env_var",
		Description: "Add or update one environment variable of a service (merged into the existing set, not a replace); the service's pods roll to pick up the change.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvVarArgs) (*mcp.CallToolResult, EnvVarView, error) {
		v, err := s.SetEnvVar(ctx, in.ServiceID, in.Key, EnvVarWrite{Value: in.Value, GenerateValue: in.GenerateValue})
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
		files, err := s.ListSecretFilesPage(ctx, in.ServiceID, in.Cursor, in.Limit)
		result := secretFilesResult{SecretFiles: files}
		if (in.Limit > 0 || in.Cursor != "") && len(files) > 0 {
			result.Cursor = files[len(files)-1].Name
		}
		return nil, result, err
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
