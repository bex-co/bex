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

package sandbox

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// spawnSandboxArgs is the MCP input for spawn_sandbox. Template selects a
// registered image; plan is the Render-compatible size; ownerId scopes the
// workspace (empty => the caller's default).
type spawnSandboxArgs struct {
	Template string `json:"template"`
	Plan     string `json:"plan,omitempty"`
	OwnerID  string `json:"ownerId,omitempty"`
}

type sandboxIDArgs struct {
	ID string `json:"id"`
}

type listSandboxesResult struct {
	Sandboxes []Sandbox `json:"sandboxes"`
}

type stopSandboxResult struct {
	Stopped bool `json:"stopped"`
}

// RegisterMCP wires the agent-facing sandbox tools (ADR042 D2 / ADR014 D3 —
// agents drive sandboxes from outside over MCP). spawn_sandbox mirrors the
// Render `ea sandbox create` intent; sandbox_exec (the streaming run) is a
// follow-up once the exec transport lands.
func (s *Service) RegisterMCP(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "spawn_sandbox", Description: "Create a hosted agent sandbox from a registered template and return its id and status."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in spawnSandboxArgs) (*mcp.CallToolResult, Sandbox, error) {
			sb, err := s.Create(ctx, CreateRequest{OwnerID: in.OwnerID, Template: in.Template, Plan: Plan(in.Plan)})
			return nil, sb, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "list_sandboxes", Description: "List the caller's workspace's sandboxes with their statuses."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listSandboxesResult, error) {
			out, err := s.List(ctx)
			return nil, listSandboxesResult{Sandboxes: out}, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "stop_sandbox", Description: "Terminate a sandbox by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sandboxIDArgs) (*mcp.CallToolResult, stopSandboxResult, error) {
			err := s.Terminate(ctx, in.ID)
			return nil, stopSandboxResult{Stopped: err == nil}, err
		})
}
