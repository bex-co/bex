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

package agentsessions

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

type listAutomationsArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	Mine    bool   `json:"mine,omitempty" jsonschema:"only the automations you created"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"the previous page's nextCursor"`
	Limit   int    `json:"limit,omitempty"`
}

type automationToolArgs struct {
	OwnerID    string `json:"ownerId,omitempty"`
	Automation string `json:"automation" jsonschema:"the automation's aut- id or its name"`
}

type createAutomationArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	AgentAutomationInput
}

type updateAutomationArgs struct {
	OwnerID    string `json:"ownerId,omitempty"`
	Automation string `json:"automation" jsonschema:"the automation's aut- id or its name"`
	AgentAutomationInput
}

type listAutomationRunsArgs struct {
	OwnerID    string `json:"ownerId,omitempty"`
	Automation string `json:"automation" jsonschema:"the automation's aut- id or its name"`
	Cursor     string `json:"cursor,omitempty" jsonschema:"the previous page's nextCursor"`
	Limit      int    `json:"limit,omitempty"`
}

// registerAutomationMCP adds the ADR101 D8 agent automation tools.
func (s *Service) registerAutomationMCP(server *mcp.Server) {
	no := false
	// A verb outside get/list/create/update/delete defaults to destructive;
	// these are not, so the hints are explicit.
	safe := func(idempotent, openWorld bool) *mcp.ToolAnnotations {
		return &mcp.ToolAnnotations{IdempotentHint: idempotent, DestructiveHint: &no, OpenWorldHint: &openWorld}
	}
	mcputil.AddTool(server, &mcp.Tool{Name: "list_agent_automations", Description: "List a workspace's agent automations, newest first: a schedule plus a session template that starts a cloud agent session on each fire, as the member who created it. mine=true keeps only yours. Page with cursor (the previous nextCursor)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listAutomationsArgs) (*mcp.CallToolResult, AgentAutomationList, error) {
			out, err := s.AgentAutomations(ctx, in.OwnerID, in.Mine, in.Cursor, in.Limit)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_automation", Description: "Read one agent automation by aut- id or name: its triggers, template, next and last fire, and pause reason."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in automationToolArgs) (*mcp.CallToolResult, AgentAutomation, error) {
			out, err := s.AgentAutomation(ctx, in.OwnerID, in.Automation)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "create_agent_automation", Description: "Add an agent automation: a name, exactly one schedule trigger and a session template (task, repo, agentConfig, and maxSpendUSD when the model is platform-metered). Every fire starts a session as you. At most 50 per workspace."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createAutomationArgs) (*mcp.CallToolResult, AgentAutomation, error) {
			out, err := s.CreateAgentAutomation(ctx, in.OwnerID, in.AgentAutomationInput)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "update_agent_automation", Description: "Change an agent automation (its creator only); omitted fields keep their value, triggers and template replace the stored ones whole. Pass ifVersion to refuse the write if someone changed it since you read it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateAutomationArgs) (*mcp.CallToolResult, AgentAutomation, error) {
			out, err := s.UpdateAgentAutomation(ctx, in.OwnerID, in.Automation, in.AgentAutomationInput)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "delete_agent_automation", Description: "Delete an agent automation and its run history (its creator or a workspace admin). Sessions it already started stay."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in automationToolArgs) (*mcp.CallToolResult, deleteResult, error) {
			if err := s.DeleteAgentAutomation(ctx, in.OwnerID, in.Automation); err != nil {
				return nil, deleteResult{}, err
			}
			return nil, deleteResult{Deleted: true}, nil
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "pause_agent_automation", Description: "Stop an agent automation firing (its creator or a workspace admin). Pausing a paused automation changes nothing.", Annotations: safe(true, false)},
		func(ctx context.Context, _ *mcp.CallToolRequest, in automationToolArgs) (*mcp.CallToolResult, AgentAutomation, error) {
			out, err := s.PauseAgentAutomation(ctx, in.OwnerID, in.Automation)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "resume_agent_automation", Description: "Re-enable a paused agent automation (its creator or a workspace admin): clears the pause reason and failure count and schedules the next fire.", Annotations: safe(true, false)},
		func(ctx context.Context, _ *mcp.CallToolRequest, in automationToolArgs) (*mcp.CallToolResult, AgentAutomation, error) {
			out, err := s.ResumeAgentAutomation(ctx, in.OwnerID, in.Automation)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "run_agent_automation", Description: "Fire an agent automation now, as a normal run that starts a cloud agent session under the same authorization, overlap and spend rules as a scheduled fire. Each call is a new run.", Annotations: safe(false, true)},
		func(ctx context.Context, _ *mcp.CallToolRequest, in automationToolArgs) (*mcp.CallToolResult, AgentAutomationRun, error) {
			out, err := s.RunAgentAutomationNow(ctx, in.OwnerID, in.Automation)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "take_over_agent_automation", Description: "Make yourself (a workspace admin) the creator of an agent automation, so its fires run as you. Use it when the creator left or lost access.", Annotations: safe(true, false)},
		func(ctx context.Context, _ *mcp.CallToolRequest, in automationToolArgs) (*mcp.CallToolResult, AgentAutomation, error) {
			out, err := s.TakeOverAgentAutomation(ctx, in.OwnerID, in.Automation)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "list_agent_automation_runs", Description: "List an agent automation's runs, newest first: fire time, outcome, the session it started and any missed ticks. Page with cursor (the previous nextCursor)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listAutomationRunsArgs) (*mcp.CallToolResult, AgentAutomationRunList, error) {
			out, err := s.AgentAutomationRuns(ctx, in.OwnerID, in.Automation, in.Cursor, in.Limit)
			return nil, out, err
		})
}
