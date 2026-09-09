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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const mcpWorkspaceIDDescription = "The ID of the Render workspace to use. Reuse the workspaceId the user confirmed from list_workspaces."

// MCP tools that operate on the caller rather than on a workspace do not
// expose workspaceId. Every other registered tool is a workspace resource
// operation and is scoped uniformly here instead of growing adapter-specific
// ownerId or transport-session state in each feature package.
var mcpCallerScopedTools = map[string]struct{}{
	"preview_workspace_invite":     {},
	"accept_workspace_invite":      {},
	"add_ssh_key":                  {},
	"delete_ssh_key":               {},
	"get_notification_settings":    {},
	"list_ssh_keys":                {},
	"list_workspaces":              {},
	"update_notification_settings": {},
}

func mcpWorkspaceMiddleware(base *core.Base, checkScope func(context.Context, string) error) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			switch method {
			case "tools/list":
				result, err := next(ctx, method, req)
				if err != nil {
					return nil, err
				}
				listed, ok := result.(*mcp.ListToolsResult)
				if !ok {
					return result, nil
				}
				return toolsWithWorkspaceID(listed), nil
			case "tools/call":
				return mcpBindCallWorkspace(ctx, base, method, req, next, checkScope)
			default:
				return next(ctx, method, req)
			}
		}
	}
}

// mcpBindCallWorkspace binds a tool call's explicit workspaceId onto the context
// after checking the caller may act in it, and strips the argument before the
// tool sees it. A caller-scoped tool (one that answers about the caller rather
// than a workspace) passes straight through, as does any request shape that
// isn't a tool call.
func mcpBindCallWorkspace(ctx context.Context, base *core.Base, method string, req mcp.Request, next mcp.MethodHandler, checkScope func(context.Context, string) error) (mcp.Result, error) {
	call, ok := req.(*mcp.CallToolRequest)
	if !ok || call.Params == nil {
		return next(ctx, method, req)
	}
	if _, callerScoped := mcpCallerScopedTools[call.Params.Name]; callerScoped {
		if err := mcpRequireScope(ctx, checkScope, call.Params.Name); err != nil {
			return mcpCallError(ctx, err), nil
		}
		return next(ctx, method, req)
	}
	resolved, workspaceID, err := takeMCPWorkspaceID(call)
	if err != nil {
		return mcpCallError(ctx, err), nil
	}
	if workspaceID != "" {
		ctx = core.WithWorkspace(ctx, workspaceID)
		if err := base.ValidateNamedWorkspace(ctx); err != nil {
			return mcpCallError(ctx, fmt.Errorf("cannot access workspace %s: %w", workspaceID, err)), nil
		}
	}
	if err := mcpRequireScope(ctx, checkScope, call.Params.Name); err != nil {
		return mcpCallError(ctx, err), nil
	}
	return next(ctx, method, resolved)
}

func mcpRequireScope(ctx context.Context, checkScope func(context.Context, string) error, tool string) error {
	if checkScope == nil {
		return nil
	}
	if err := checkScope(ctx, tool); err != nil {
		return core.MCPError(err)
	}
	return nil
}

func toolsWithWorkspaceID(in *mcp.ListToolsResult) *mcp.ListToolsResult {
	out := *in
	out.Tools = make([]*mcp.Tool, len(in.Tools))
	for i, tool := range in.Tools {
		if tool == nil {
			continue
		}
		clone := *tool
		if _, callerScoped := mcpCallerScopedTools[clone.Name]; !callerScoped {
			clone.InputSchema = workspaceInputSchema(clone.InputSchema)
		}
		out.Tools[i] = &clone
	}
	return &out
}

func workspaceInputSchema(schema any) any {
	raw, err := json.Marshal(schema)
	if err != nil {
		return schema
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return schema
	}
	properties, _ := object["properties"].(map[string]any)
	if properties == nil {
		properties = map[string]any{}
	}
	delete(properties, "ownerId")
	delete(properties, "ownerID")
	properties["workspaceId"] = map[string]any{
		"type":        "string",
		"description": mcpWorkspaceIDDescription,
	}
	object["properties"] = properties
	if required, ok := object["required"].([]any); ok {
		filtered := slices.DeleteFunc(required, func(name any) bool {
			return name == "workspaceId" || name == "ownerId" || name == "ownerID"
		})
		if len(filtered) == 0 {
			delete(object, "required")
		} else {
			object["required"] = filtered
		}
	}
	return object
}

func takeMCPWorkspaceID(req *mcp.CallToolRequest) (*mcp.CallToolRequest, string, error) {
	var args map[string]json.RawMessage
	if len(req.Params.Arguments) != 0 {
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return req, "", fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	if args == nil {
		args = map[string]json.RawMessage{}
	}
	if _, exists := args["ownerId"]; exists {
		return req, "", errors.New("ownerId is not supported by MCP tools; use workspaceId")
	}
	if _, exists := args["ownerID"]; exists {
		return req, "", errors.New("ownerID is not supported by MCP tools; use workspaceId")
	}

	workspaceID := ""
	if raw, exists := args["workspaceId"]; exists {
		if err := json.Unmarshal(raw, &workspaceID); err != nil {
			return req, "", errors.New("workspaceId must be a string")
		}
		if strings.TrimSpace(workspaceID) == "" {
			return req, "", errors.New("workspaceId cannot be empty")
		}
		delete(args, "workspaceId")
	}

	arguments, err := json.Marshal(args)
	if err != nil {
		return req, "", fmt.Errorf("encode tool arguments: %w", err)
	}
	params := *req.Params
	params.Arguments = arguments
	resolved := *req
	resolved.Params = &params
	return &resolved, workspaceID, nil
}

// mcpCallError renders a gate's refusal as the tool-visible error result, and
// tells the telemetry middleware which CLASS of refusal it was: SetError keeps
// only the message, so a workspace/scope denial would otherwise be
// indistinguishable from a tool fault (w3/m84, httpmetrics.go).
func mcpCallError(ctx context.Context, err error) *mcp.CallToolResult {
	markToolOutcome(ctx, err)
	var result mcp.CallToolResult
	result.SetError(err)
	return &result
}
