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

	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// spawnSandboxArgs is the MCP input for spawn_sandbox. Template selects a
// registered image; plan is the Render-compatible size; ownerId scopes the
// workspace (empty => the caller's default).
type spawnSandboxArgs struct {
	Template       string         `json:"template,omitempty"`
	Plan           string         `json:"plan,omitempty"`
	OwnerID        string         `json:"ownerId,omitempty"`
	Region         string         `json:"region,omitempty"`
	TimeoutSeconds int            `json:"timeoutSeconds,omitempty"`
	NetworkPolicy  *NetworkPolicy `json:"networkPolicy,omitempty"`
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

// execSandboxArgs is the MCP input for sandbox_exec: run one command in a
// sandbox and return its collected output + exit code.
type execSandboxArgs struct {
	ID      string `json:"id"`
	Command string `json:"command"`
	OwnerID string `json:"ownerId,omitempty"`
}

// mcpError keeps the domain's machine-readable refusal name visible in MCP's
// text error content. REST carries it in `code` and GraphQL in `extensions`;
// the MCP SDK otherwise serializes only Error(), which would erase the name.
// RegisterMCP wires the agent-facing sandbox tools (ADR042 D2 / ADR014 D3 —
// agents drive sandboxes from outside over MCP). spawn_sandbox mirrors the
// Render `ea sandbox create` intent; sandbox_exec runs a command via the same
// authorized gateway path the CLI uses, returning buffered output (no SSE dance).
func (s *Service) RegisterMCP(server *mcp.Server) {
	mcputil.AddTool(server, &mcp.Tool{Name: "spawn_sandbox", Description: "Create a hosted agent sandbox from a registered template and return its id and status."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in spawnSandboxArgs) (*mcp.CallToolResult, Sandbox, error) {
			sb, err := s.Create(ctx, CreateRequest{
				OwnerID:        in.OwnerID,
				Template:       in.Template,
				Plan:           Plan(in.Plan),
				Region:         in.Region,
				TimeoutSeconds: in.TimeoutSeconds,
				NetworkPolicy:  in.NetworkPolicy,
			})
			return nil, sb, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "list_sandboxes", Description: "List the caller's workspace's sandboxes with their statuses."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listSandboxesResult, error) {
			out, err := s.List(ctx)
			return nil, listSandboxesResult{Sandboxes: out}, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "stop_sandbox", Description: "Terminate a sandbox by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sandboxIDArgs) (*mcp.CallToolResult, stopSandboxResult, error) {
			err := s.Terminate(ctx, in.ID)
			return nil, stopSandboxResult{Stopped: err == nil}, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "sandbox_exec", Description: "Run a shell command in a sandbox and return its stdout, stderr, and exit code."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in execSandboxArgs) (*mcp.CallToolResult, ExecResult, error) {
			res, err := s.ExecBuffered(ctx, ExecRequest{OwnerID: in.OwnerID, SandboxID: in.ID, Command: in.Command})
			return nil, res, err
		})
}
