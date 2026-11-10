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
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// spawnSandboxArgs is the MCP input for spawn_sandbox. Template selects a
// registered image; plan is the Render-compatible size; ownerId scopes the
// workspace (empty => the caller's default).
type spawnSandboxArgs struct {
	Template string `json:"template,omitempty"`
	Plan     string `json:"plan,omitempty"`
	OwnerID  string `json:"ownerId,omitempty"`
	Region   string `json:"region,omitempty"`
	// TimeoutSeconds: omitted = 86400 s, 0 = no timeout (see resolveTimeout).
	TimeoutSeconds *int `json:"timeoutSeconds,omitempty"`
	// IdleTimeoutSeconds: omitted = 900 s, 0 disables idle suspension (see
	// resolveIdleTimeout).
	IdleTimeoutSeconds *int           `json:"idleTimeoutSeconds,omitempty"`
	NetworkPolicy      *NetworkPolicy `json:"networkPolicy,omitempty"`
	// Name is Render's optional, non-unique sandbox name (at most 64
	// characters, never sbx-…).
	Name string `json:"name,omitempty"`
	// Env is Render's create-time env: injected into the sandbox, never read back.
	Env map[string]string `json:"env,omitempty"`
	// SandboxGroupID is the group to create in (w22/m311); omitted = the
	// workspace's default group.
	SandboxGroupID string `json:"sandboxGroupId,omitempty"`
}

// listSandboxesArgs is the MCP input for list_sandboxes: Render's list
// `status` filter (omitted = every status but terminated).
type listSandboxesArgs struct {
	Status []string `json:"status,omitempty"`
	// Name keeps only sandboxes with exactly this name.
	Name string `json:"name,omitempty"`
	// SandboxGroupID keeps only sandboxes in this group (w22/m311).
	SandboxGroupID string `json:"sandboxGroupId,omitempty"`
}

type sandboxIDArgs struct {
	ID string `json:"id"`
}

// listExecutionsArgs is the MCP input for list_sandbox_executions: Render's
// cursor and limit (omitted = 20, at most 100).
type listExecutionsArgs struct {
	ID      string `json:"id"`
	OwnerID string `json:"ownerId,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

type listExecutionsResult struct {
	Executions []ExecutionEnvelope `json:"executions"`
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
	mcputil.AddTool(server, &mcp.Tool{Name: "spawn_sandbox", Description: fmt.Sprintf("Create a hosted agent sandbox from a registered template and return its id and status. networkPolicy is Render's: omitted means allow-all (any public destination); {type:\"deny-all\"} blocks all outbound traffic; {type:\"allow-list\", rules:[{domain}]} allows only HTTPS to the listed hostnames (a leading *. covers one subdomain level). env sets environment variables inside the sandbox (at most %d, %d KB across names and values); values are never returned. sandboxGroupId puts it in one of the workspace's sandbox groups (list_sandbox_groups); omitted, it goes in the default group, which a workspace gets when it first needs one. A group at its concurrencyLimit refuses the create with SANDBOX_GROUP_CONCURRENCY_LIMIT.", maxSandboxEnvVars, maxSandboxEnvBytes/1024)},
		func(ctx context.Context, _ *mcp.CallToolRequest, in spawnSandboxArgs) (*mcp.CallToolResult, Sandbox, error) {
			sb, err := s.Create(ctx, CreateRequest{
				OwnerID:            in.OwnerID,
				Template:           in.Template,
				Plan:               Plan(in.Plan),
				Region:             in.Region,
				TimeoutSeconds:     in.TimeoutSeconds,
				IdleTimeoutSeconds: in.IdleTimeoutSeconds,
				NetworkPolicy:      in.NetworkPolicy,
				Name:               in.Name,
				Env:                in.Env,
				SandboxGroupID:     in.SandboxGroupID,
			})
			return nil, sb, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "list_sandboxes", Description: "List the caller's workspace's sandboxes with their statuses. Optional status: a list of Render sandbox statuses to keep; omitted, terminated sandboxes are hidden. Optional sandboxGroupId keeps only that group's sandboxes."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listSandboxesArgs) (*mcp.CallToolResult, listSandboxesResult, error) {
			out, err := s.listFiltered(ctx, in.Status, in.Name, in.SandboxGroupID)
			return nil, listSandboxesResult{Sandboxes: out}, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "stop_sandbox", Description: "Terminate a sandbox by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sandboxIDArgs) (*mcp.CallToolResult, stopSandboxResult, error) {
			err := s.Terminate(ctx, in.ID)
			return nil, stopSandboxResult{Stopped: err == nil}, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "list_sandbox_executions", Description: "List a sandbox's recorded command runs and file transfers, newest first, with each one's command or path, caller, start and stop time, and exit code."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listExecutionsArgs) (*mcp.CallToolResult, listExecutionsResult, error) {
			out, err := s.ListExecutions(core.WithWorkspace(ctx, in.OwnerID), in.ID, in.Cursor, core.PageLimitOrDefault(in.Limit))
			return nil, listExecutionsResult{Executions: out}, err
		})
	s.registerGroupMCP(server)
	mcputil.AddTool(server, &mcp.Tool{Name: "sandbox_exec", Description: "Run a shell command in a sandbox and return its stdout, stderr, and exit code."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in execSandboxArgs) (*mcp.CallToolResult, ExecResult, error) {
			res, err := s.ExecBuffered(ctx, ExecRequest{OwnerID: in.OwnerID, SandboxID: in.ID, Command: in.Command})
			return nil, res, err
		})
}
