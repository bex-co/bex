package agentsessions

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// CloudAgentsTools is the `cloud-agents` toolset (ADR094 D2): the task-shaped
// subset an orchestrator or CI bot needs to delegate coding work, served alone
// at /mcp/cloud-agents. Admin, archive and ticket tools stay on /mcp only.

var CloudAgentsTools = []string{
	"get_agent_session_capabilities",
	"create_agent_session",
	"wait_agent_sessions",
	"get_agent_session",
	"list_agent_sessions",
	"send_agent_session_message",
	"get_agent_session_transcript",
	"get_agent_session_tool_output",
	"cancel_agent_session",
	"upload_agent_attachment",
	"get_agent_attachment",
	"list_agent_session_attachments",
	"delete_agent_attachment",
}

type listArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	// ADR065 D3 filters + keyset pagination (cursor = the prior page's last
	// session id; a shorter/empty page signals the end).
	Archived      string   `json:"archived,omitempty"`
	Phases        []string `json:"phases,omitempty"`
	States        []string `json:"states,omitempty"`
	Tag           string   `json:"tag,omitempty"`
	Repo          string   `json:"repo,omitempty"`
	CreatedBefore string   `json:"createdBefore,omitempty"`
	CreatedAfter  string   `json:"createdAfter,omitempty"`
	Cursor        string   `json:"cursor,omitempty"`
	Limit         int      `json:"limit,omitempty"`
}

type capabilitiesArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
}

type environmentArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
}

type updateEnvironmentArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	EnvironmentInput
}

// UnmarshalJSON records an explicit null spend cap (clear the override),
// which the plain decode cannot tell from an omitted one.
func (a *updateEnvironmentArgs) UnmarshalJSON(data []byte) error {
	type wire updateEnvironmentArgs
	var decoded wire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*a = updateEnvironmentArgs(decoded)
	a.markSpendCapPresence(data)
	return nil
}

type idArgs struct {
	ID string `json:"id"`
}

type transcriptArgs struct {
	ID       string `json:"id"`
	AfterSeq *int64 `json:"afterSeq,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type toolOutputArgs struct {
	ID         string `json:"id"`
	Turn       int    `json:"turn"`
	ToolCallID string `json:"toolCallId"`
}

type deleteResult struct {
	Deleted bool `json:"deleted"`
}

type attachTicketArgs struct {
	ID     string `json:"id"`
	Action string `json:"action,omitempty"` // "read" or "turn", defaults to "read"
}

type steerArgs struct {
	ID              string   `json:"id"`
	Prompt          string   `json:"prompt"`
	EgressAllowlist []string `json:"egressAllowlist,omitempty"`
	AttachmentIDs   []string `json:"attachmentIds,omitempty"`
	// OutputSchema and OutputRequired: this turn's structured-output contract (ADR094 D5).
	OutputSchema   map[string]any `json:"outputSchema,omitempty" jsonschema:"a JSON Schema 2020-12 object (at most 64 KiB, no external $ref) the turn's result must satisfy; read it from outcome.result"`
	OutputRequired *bool          `json:"outputRequired,omitempty" jsonschema:"default true when outputSchema is set: a turn that ends without a final valid result fails STRUCTURED_OUTPUT_MISSING"`
}

type uploadAttachmentArgs struct {
	OwnerID       string `json:"ownerId,omitempty"`
	Name          string `json:"name" jsonschema:"the file name the user and the agent see"`
	ContentBase64 string `json:"contentBase64" jsonschema:"the file's bytes, standard base64, at most 4 MiB decoded"`
}

type listAttachmentsArgs struct {
	ID        string `json:"id" jsonschema:"the agent session id"`
	Direction string `json:"direction,omitempty" jsonschema:"input (uploaded by users) or output (shared by the agent); omit for both"`
	Turn      *int   `json:"turn,omitempty" jsonschema:"only the files of this turn"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"the previous page's last attachment id"`
	Limit     int    `json:"limit,omitempty"`
}

type listAttachmentsResult struct {
	Attachments []Attachment `json:"attachments"`
}

type listResult struct {
	AgentSessions []View `json:"agentSessions"`
}

// The tool names follow the existing early-access sandbox vocabulary:
// spawn/list/get/cancel/resume, with agent_session making the bex-native
// resource explicit rather than pretending Render has a corresponding API.
func (s *Service) RegisterMCP(server *mcp.Server) {
	// create_agent_session is the canonical name (ADR094 D2: `create_` is the
	// vocabulary every other bex create tool uses); spawn_agent_session stays
	// as an alias so existing clients keep working.
	createSession := func(ctx context.Context, _ *mcp.CallToolRequest, in CreateRequest) (*mcp.CallToolResult, View, error) {
		out, err := s.Create(ctx, in)
		return nil, out, err
	}
	mcputil.AddTool(server, &mcp.Tool{Name: "create_agent_session", Description: "Start a cloud coding-agent session on a repository. Returns the new session in phase creating; it does not return an attach ticket (call attach_agent_session for one). Pass outputSchema (a JSON Schema) to get a machine-readable result: the agent submits one that validates against it and outcome.result carries it, or the turn fails with failureReasonCode STRUCTURED_OUTPUT_MISSING."}, createSession)
	mcputil.AddTool(server, &mcp.Tool{Name: "spawn_agent_session", Description: "Alias of create_agent_session. Start a cloud coding-agent session on a repository; it does not return an attach ticket (call attach_agent_session for one)."}, createSession)
	s.registerWaitMCP(server)
	mcputil.AddTool(server, &mcp.Tool{Name: "list_agent_sessions", Description: "List cloud coding-agent sessions in a workspace: one filtered page, newest first. Defaults to the unarchived working set; archived selects false|true|all; cursor is the prior page's last session id (a shorter/empty page is the end)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listArgs) (*mcp.CallToolResult, listResult, error) {
			before, err := core.ParseTime("createdBefore", in.CreatedBefore)
			if err != nil {
				return nil, listResult{}, err
			}
			after, err := core.ParseTime("createdAfter", in.CreatedAfter)
			if err != nil {
				return nil, listResult{}, err
			}
			out, err := s.List(ctx, ListRequest{
				OwnerID: in.OwnerID, Archived: in.Archived, Phases: in.Phases, States: in.States, Tag: in.Tag,
				Repo: in.Repo, CreatedBefore: before, CreatedAfter: after,
				Cursor: in.Cursor, Limit: in.Limit,
			})
			return nil, listResult{AgentSessions: out}, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_session", Description: "Inspect one cloud coding-agent session."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Get(ctx, in.ID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_session_model_cost", Description: "Read what a cloud coding-agent session's bex-provided model requests cost: the model source (platform = bex-billed, byo = billed by the workspace's own provider), the total in USD rounded to the cent (billed at the model provider's cost + 30%) and each turn's cost, request count, input/output/cache tokens and models. A session on the workspace's own model key has no requests here; its provider bills it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, ModelCost, error) {
			out, err := s.ModelCost(ctx, in.ID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_session_capabilities", Description: "Report a workspace's selectable agent profiles and GitHub/model-key readiness for composing a cloud coding-agent session; never returns model endpoints, templates, egress, or credentials."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in capabilitiesArgs) (*mcp.CallToolResult, Capabilities, error) {
			out, err := s.Capabilities(ctx, in.OwnerID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_environment", Description: "Read a workspace's agent environment: the sandbox network tier (registries = GitHub plus the built-in package registries for the whole session, the default; custom = registries plus customHosts; full = any public host over HTTP(S) through the bex egress proxy, with model providers blocked and no raw TCP or SSH), the custom hostnames, the default model, whether the model is pinned by the platform model route, the model source (byo = the workspace's own model provider key, platform = bex-provided and billed at the provider's cost + 30%, none) with modelKeySet (the key itself is never returned), and the agent-model spend controls: the effective monthly spend cap in USD (min of the admin override or plan default and the billing trust tier's ceiling), the plan default, the override, month-to-date spend, the trust tier (T0..T4) and the next charge threshold, whether a failed or disputed payment holds the tier (trustTierLocked), whether the tier's ceiling sets the cap (agentModelSpendCapTierLimited), and whether an unpaid failed invoice refuses platform-model requests (billingBalanceDue)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in environmentArgs) (*mcp.CallToolResult, Environment, error) {
			out, err := s.Environment(ctx, in.OwnerID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "update_agent_environment", Description: "Update a workspace's agent environment (workspace admins only). Omitted fields keep their value: networkTier registries|custom|full; customHosts replaces the list (exact public hostnames, at most 32; [] clears); defaultModel \"\" returns to the provider default and cannot differ from a pinned model. agentModelMonthlySpendCapUsd sets the monthly agent-model spend cap in USD (more than 0, at most 10x the plan default; null or resetAgentModelMonthlySpendCap: true returns to the plan default). modelApiKey (write-only) sets or rotates the workspace's own model provider key, which then wins over the bex-provided model (no token billing, no spend cap); clearModelApiKey: true removes it. The network tier and hosts apply from each session's next turn; the default model applies to sessions created afterwards; the spend cap applies within 20 seconds."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateEnvironmentArgs) (*mcp.CallToolResult, Environment, error) {
			out, err := s.UpdateEnvironment(ctx, in.OwnerID, in.EnvironmentInput)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_session_transcript", Description: "Read one page of a cloud coding-agent session's durable conversation transcript, seq-ordered: each part is a completed item (a UIMessage text, reasoning or dynamic-tool part; a data snapshot; or a start/finish chunk), partial when the live stream ended inside it (a later page may carry its complete copy under a new seq with the same turn and partIndex, which supersedes it; render parts in turn, partIndex order). Works for live, completed, and archived sessions; page by echoing nextAfterSeq as afterSeq until parts is empty."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in transcriptArgs) (*mcp.CallToolResult, TranscriptPage, error) {
			afterSeq := int64(-1)
			if in.AfterSeq != nil {
				afterSeq = *in.AfterSeq
			}
			out, err := s.Transcript(ctx, in.ID, afterSeq, in.Limit)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_session_tool_output", Description: "Read the complete output of one tool call in a cloud coding-agent session. A transcript tool part flagged fullOutput: true stores only the head and tail of a long output inline (around a [bex: N chars clipped] marker); this returns the full text by field (output, errorText, terminal:<terminalId>), each capped at 1 MiB. Pass the session id, the part's turn and its toolCallId. Not found when no full copy was kept."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in toolOutputArgs) (*mcp.CallToolResult, ToolOutput, error) {
			out, err := s.ToolOutput(ctx, in.ID, in.Turn, in.ToolCallID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "archive_agent_session", Description: "Archive a cloud coding-agent session: it leaves the working-set list (still viewable; resume/steer refused until unarchived) and any still-live sandbox is reclaimed at the next reaper tick without canceling an in-flight turn."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Archive(ctx, in.ID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "unarchive_agent_session", Description: "Return an archived cloud coding-agent session to the working set, re-enabling resume/steer."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Unarchive(ctx, in.ID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "delete_agent_session", Description: "Permanently delete a finished cloud coding-agent session: its record and transcript are destroyed. Live sessions must be canceled first. Irreversible."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, deleteResult, error) {
			if err := s.Delete(ctx, in.ID); err != nil {
				return nil, deleteResult{}, err
			}
			return nil, deleteResult{Deleted: true}, nil
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "resume_agent_session", Description: "Resume a suspended cloud coding-agent session. Returns the session; it does not mint an attach ticket (call attach_agent_session for one)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Resume(ctx, in.ID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "attach_agent_session", Description: "Mint a fresh attach ticket for a started cloud coding-agent session to (re)connect to its live or replayed conversation stream, without changing its lifecycle. Action 'read' for transcript replay (GET), 'turn' for live prompt execution (POST). Requires can_create for 'turn'."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in attachTicketArgs) (*mcp.CallToolResult, View, error) {
			action := in.Action
			if action == "" {
				action = agentsessionticket.ActionRead // Default to read for safety
			}
			out, err := s.AttachTicket(ctx, in.ID, action)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "cancel_agent_session", Description: "Cancel a cloud coding-agent session and terminate its sandbox."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Cancel(ctx, in.ID)
			return nil, out, err
		})
	// send_agent_session_message is the canonical name for a follow-up turn
	// (ADR094 D2); steer_agent_session stays as an alias.
	sendMessage := func(ctx context.Context, _ *mcp.CallToolRequest, in steerArgs) (*mcp.CallToolResult, View, error) {
		out, err := s.Steer(ctx, SteerRequest{SessionID: in.ID, Prompt: in.Prompt, EgressAllowlist: in.EgressAllowlist, AttachmentIDs: in.AttachmentIDs, OutputSchema: in.OutputSchema, OutputRequired: in.OutputRequired})
		return nil, out, err
	}
	mcputil.AddTool(server, &mcp.Tool{Name: "send_agent_session_message", Description: "Send a follow-up message to a cloud coding-agent session: it runs a new turn, re-dispatches the sandbox on the same branch and updates the same draft PR. The session must be steerable (state succeeded or failed, not archived or canceled). Pass outputSchema to require a machine-readable result for this turn, read from outcome.result."}, sendMessage)
	mcputil.AddTool(server, &mcp.Tool{Name: "steer_agent_session", Description: "Alias of send_agent_session_message. Run a follow-up prompt turn on a cloud coding-agent session; it re-dispatches the sandbox on the same branch and updates the same draft PR."}, sendMessage)
	s.registerAttachmentMCP(server)
}

// registerAttachmentMCP adds the ADR095 D11 attachment tools. An upload is
// unattached until its id is passed as attachmentIds to create_agent_session
// or send_agent_session_message.
func (s *Service) registerAttachmentMCP(server *mcp.Server) {
	mcputil.AddTool(server, &mcp.Tool{
		Name:        "upload_agent_attachment",
		Description: fmt.Sprintf("Upload a file (a screenshot, log, dataset) for a cloud coding agent: pass its name and its bytes as standard base64, at most %d MiB decoded (use REST POST /v1/agent-attachments for up to %d MiB). Returns an unattached attachment; pass its id in attachmentIds to create_agent_session or send_agent_session_message within 24 hours. The type is detected from the bytes.", maxMCPUploadBytes>>20, maxAttachmentBytes>>20),
		Annotations: mcputil.DefaultAnnotations("create_"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in uploadAttachmentArgs) (*mcp.CallToolResult, Attachment, error) {
		data, err := decodeMCPUpload(in.ContentBase64)
		if err != nil {
			return nil, Attachment{}, err
		}
		out, err := s.UploadAttachment(ctx, UploadAttachmentRequest{OwnerID: in.OwnerID, Name: in.Name, Size: int64(len(data)), Body: bytes.NewReader(data)})
		return nil, out, err
	})
	mcputil.AddTool(server, &mcp.Tool{
		Name:        "get_agent_attachment",
		Description: "Read one agent attachment: its metadata and a downloadUrl valid for 15 minutes. A PNG, JPEG, GIF or WebP image up to 5 MiB is also returned as an image, and a text, JSON, YAML or CSV file up to 256 KiB as text, so you can look at it directly.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, Attachment, error) {
		out, data, err := s.AttachmentInline(ctx, in.ID)
		if err != nil || data == nil {
			return nil, out, err
		}
		return inlineAttachmentResult(out, data), out, nil
	})
	mcputil.AddTool(server, &mcp.Tool{
		Name:        "list_agent_session_attachments",
		Description: "List a cloud coding-agent session's files, oldest first: inputs the users attached and outputs the agent shared, each with a downloadUrl valid for 15 minutes. Filter by direction (input or output) and turn; page with cursor (the previous page's last id) until a shorter page.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listAttachmentsArgs) (*mcp.CallToolResult, listAttachmentsResult, error) {
		list, err := s.ListAttachments(ctx, in.ID, AttachmentListQuery{Direction: in.Direction, Turn: in.Turn, Cursor: in.Cursor, Limit: in.Limit})
		return nil, listAttachmentsResult{Attachments: list}, err
	})
	mcputil.AddTool(server, &mcp.Tool{
		Name:        "delete_agent_attachment",
		Description: "Delete one of your unattached uploads. A file attached to a session cannot be deleted on its own; it is deleted with its session.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, deleteResult, error) {
		if err := s.DeleteAttachment(ctx, in.ID); err != nil {
			return nil, deleteResult{}, err
		}
		return nil, deleteResult{Deleted: true}, nil
	})
}

// decodeMCPUpload decodes an inline upload, refusing an oversized one before
// decoding it.
func decodeMCPUpload(encoded string) ([]byte, error) {
	tooLarge := core.NewTooLargeError("AGENT_ATTACHMENT_TOO_LARGE",
		fmt.Sprintf("an MCP upload is limited to %d MiB; use REST POST /v1/agent-attachments for up to %d MiB", maxMCPUploadBytes>>20, maxAttachmentBytes>>20),
		map[string]any{"maxBytes": maxMCPUploadBytes})
	if base64.StdEncoding.DecodedLen(len(encoded)) > maxMCPUploadBytes+2 {
		return nil, tooLarge
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, core.NewBadRequestError("AGENT_ATTACHMENT_INVALID", "contentBase64 must be standard base64", map[string]any{"field": "contentBase64"})
	}
	if len(data) > maxMCPUploadBytes {
		return nil, tooLarge
	}
	return data, nil
}

// inlineAttachmentResult is the JSON text block the SDK would generate for
// out, followed by the file itself as an image or text block.
func inlineAttachmentResult(out Attachment, data []byte) *mcp.CallToolResult {
	body, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	file := mcp.Content(&mcp.TextContent{Text: string(data)})
	if strings.HasPrefix(out.MediaType, "image/") {
		file = &mcp.ImageContent{Data: data, MIMEType: out.MediaType}
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(body)}, file}}
}
