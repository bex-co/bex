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

type queuedMessageArgs struct {
	ID        string `json:"id" jsonschema:"the agent session id"`
	MessageID string `json:"messageId" jsonschema:"the queued message's id (aqm-…), from the queuedMessage a send returned or list_agent_session_queued_messages"`
}

type editQueuedMessageArgs struct {
	queuedMessageArgs
	Prompt string `json:"prompt" jsonschema:"the message's new text; the turn then runs as you"`
}

type moveQueuedMessageArgs struct {
	queuedMessageArgs
	Position int `json:"position" jsonschema:"the place in the queue, counted from 1; 1 runs next and a number past the end puts it last"`
}

// registerQueueMCP adds the ADR047 D12 message queue and Stop turn tools.
func (s *Service) registerQueueMCP(server *mcp.Server) {
	mcputil.AddTool(server, &mcp.Tool{Name: "list_agent_session_queued_messages", Description: "List the messages queued on a cloud coding-agent session while its turn ran, next to run first, each with its id, place, author and, when its last attempt to run was refused (a spend or live-sandbox limit), blockedReason. A message runs as its own turn when the turn before it settles."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, QueuedMessageList, error) {
			out, err := s.QueuedMessages(ctx, in.ID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "edit_agent_session_queued_message", Description: "Replace a queued message's text. The turn then runs as you, so you need the access a send needs. AGENT_QUEUED_MESSAGE_DISPATCHED when it already ran, was dropped or is being started."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in editQueuedMessageArgs) (*mcp.CallToolResult, QueuedMessage, error) {
			out, err := s.EditQueuedMessage(ctx, in.ID, in.MessageID, in.Prompt)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "delete_agent_session_queued_message", Description: "Remove a queued message so it never runs. AGENT_QUEUED_MESSAGE_DISPATCHED when it already ran, was dropped or is being started."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in queuedMessageArgs) (*mcp.CallToolResult, deleteResult, error) {
			if err := s.DeleteQueuedMessage(ctx, in.ID, in.MessageID); err != nil {
				return nil, deleteResult{}, err
			}
			return nil, deleteResult{Deleted: true}, nil
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "move_agent_session_queued_message", Description: "Move a queued message to a place in the queue (1 runs next; past the end is last) and get the queue back. To run a message now: interrupt_agent_session, then move it to 1."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in moveQueuedMessageArgs) (*mcp.CallToolResult, QueuedMessageList, error) {
			out, err := s.MoveQueuedMessage(ctx, in.ID, in.MessageID, in.Position)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "interrupt_agent_session", Description: "Stop the running turn of a cloud coding-agent session and nothing else: the session settles within seconds as succeeded with the turn interrupted (turns[].interrupted), stays steerable, keeps what the agent already streamed and pushed, and keeps its queued messages, the first of which then runs. Unlike cancel_agent_session, which ends the session. AGENT_SESSION_NO_TURN_RUNNING when the session is idle; AGENT_SESSION_TURN_STARTING while the turn's sandbox is still starting."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Interrupt(ctx, in.ID)
			return nil, out, err
		})
}
