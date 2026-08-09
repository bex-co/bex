package agentsessions

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type listArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
}

type idArgs struct {
	ID string `json:"id"`
}

type steerArgs struct {
	ID              string   `json:"id"`
	Prompt          string   `json:"prompt"`
	EgressAllowlist []string `json:"egressAllowlist,omitempty"`
}

type listResult struct {
	AgentSessions []View `json:"agentSessions"`
}

func toolError(err error) error {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return fmt.Errorf("%s: %w", coded.Code, err)
	}
	return err
}

// The tool names follow the existing early-access sandbox vocabulary:
// spawn/list/get/cancel/resume, with agent_session making the bex-native
// resource explicit rather than pretending Render has a corresponding API.
func (s *Service) RegisterMCP(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{Name: "spawn_agent_session", Description: "Start a cloud coding-agent session on a repository and return its sandbox attach ticket."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in CreateRequest) (*mcp.CallToolResult, View, error) {
			out, err := s.Create(ctx, in)
			return nil, out, toolError(err)
		})
	mcp.AddTool(server, &mcp.Tool{Name: "list_agent_sessions", Description: "List cloud coding-agent sessions in a workspace."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listArgs) (*mcp.CallToolResult, listResult, error) {
			out, err := s.List(ctx, in.OwnerID)
			return nil, listResult{AgentSessions: out}, toolError(err)
		})
	mcp.AddTool(server, &mcp.Tool{Name: "get_agent_session", Description: "Inspect one cloud coding-agent session."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Get(ctx, in.ID)
			return nil, out, toolError(err)
		})
	mcp.AddTool(server, &mcp.Tool{Name: "get_agent_session_capabilities", Description: "Report a workspace's selectable agent profiles and GitHub/model-key readiness for composing a cloud coding-agent session; never returns model endpoints, templates, egress, or credentials."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listArgs) (*mcp.CallToolResult, Capabilities, error) {
			out, err := s.Capabilities(ctx, in.OwnerID)
			return nil, out, toolError(err)
		})
	mcp.AddTool(server, &mcp.Tool{Name: "resume_agent_session", Description: "Resume a suspended cloud coding-agent session and mint a fresh attach ticket."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Resume(ctx, in.ID)
			return nil, out, toolError(err)
		})
	mcp.AddTool(server, &mcp.Tool{Name: "attach_agent_session", Description: "Mint a fresh attach ticket for a started cloud coding-agent session to (re)connect to its live or replayed conversation stream, without changing its lifecycle."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.AttachTicket(ctx, in.ID)
			return nil, out, toolError(err)
		})
	mcp.AddTool(server, &mcp.Tool{Name: "cancel_agent_session", Description: "Cancel a cloud coding-agent session and terminate its sandbox."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Cancel(ctx, in.ID)
			return nil, out, toolError(err)
		})
	mcp.AddTool(server, &mcp.Tool{Name: "steer_agent_session", Description: "Run a follow-up prompt turn on a cloud coding-agent session; it re-dispatches the sandbox on the same branch and updates the same draft PR."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in steerArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Steer(ctx, SteerRequest{SessionID: in.ID, Prompt: in.Prompt, EgressAllowlist: in.EgressAllowlist})
			return nil, out, toolError(err)
		})
}
