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
	"set_agent_session_tags",
	"upload_agent_attachment",
	"get_agent_attachment",
	"list_agent_session_attachments",
	"delete_agent_attachment",
	"list_agent_secrets",
	"set_agent_secret",
	"delete_agent_secret",
	"list_agent_skills",
	"get_agent_skill",
	"create_agent_skill",
	"update_agent_skill",
	"delete_agent_skill",
	"list_agent_skill_versions",
	"get_agent_skill_version",
	"list_agent_automations",
	"get_agent_automation",
	"create_agent_automation",
	"update_agent_automation",
	"delete_agent_automation",
	"pause_agent_automation",
	"resume_agent_automation",
	"run_agent_automation",
	"take_over_agent_automation",
	"list_agent_automation_runs",
}

type listArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	// ADR065 D3 filters + keyset pagination (cursor = the prior page's last
	// session id; a shorter/empty page signals the end).
	Archived        string   `json:"archived,omitempty"`
	Phases          []string `json:"phases,omitempty"`
	States          []string `json:"states,omitempty"`
	Tag             string   `json:"tag,omitempty"`
	Repo            string   `json:"repo,omitempty"`
	AutomationID    string   `json:"automationId,omitempty" jsonschema:"only the sessions this agent automation (aut-…) started"`
	ParentSessionID string   `json:"parentSessionId,omitempty" jsonschema:"only the direct children of this session (ags-…); read the whole tree with get_agent_session_tree"`
	Mine            bool     `json:"mine,omitempty" jsonschema:"only the sessions you started"`
	CreatedBy       []string `json:"createdBy,omitempty" jsonschema:"only the sessions these subjects started (a session's createdBy); at most 20"`
	CreatedBefore   string   `json:"createdBefore,omitempty"`
	CreatedAfter    string   `json:"createdAfter,omitempty"`
	Cursor          string   `json:"cursor,omitempty"`
	Limit           int      `json:"limit,omitempty"`
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

type setTagsArgs struct {
	ID   string   `json:"id"`
	Tags []string `json:"tags" jsonschema:"the session's new tags, replacing all current ones: at most 20, each [a-z0-9:_-]{1,64}; [] clears them"`
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
	SessionSecretsInput
	Skills []SkillInvocation `json:"skills,omitempty" jsonschema:"workspace skills (list_agent_skills) this turn must follow, at most 5, each with optional arguments for $ARGUMENTS"`
}

type agentSecretArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	Name    string `json:"name" jsonschema:"the secret's env var name"`
}

type setAgentSecretArgs struct {
	OwnerID string  `json:"ownerId,omitempty"`
	Name    string  `json:"name" jsonschema:"the env var name the agent reads: [A-Za-z_][A-Za-z0-9_]* (at most 128), not a platform-reserved name such as BEX_* or PATH"`
	Value   string  `json:"value,omitempty" jsonschema:"the secret value, 1 B to 32 KiB; write-only, never returned. Required for a new secret; omit it to change only an existing secret's scope"`
	Scope   *string `json:"scope,omitempty" jsonschema:"which sessions receive it without naming it: workspace (every session; the default for a new secret), repo (sessions on repo) or explicit (only sessions that name it in secretNames). Omit to keep an existing secret's scope"`
	Repo    string  `json:"repo,omitempty" jsonschema:"owner/name, required with scope repo"`
}

type agentSkillArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	Skill   string `json:"skill" jsonschema:"the skill's skl- id or its name"`
}

type listAgentSkillVersionsArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	Skill   string `json:"skill" jsonschema:"the skill's skl- id or its name"`
	Cursor  string `json:"cursor,omitempty" jsonschema:"the previous page's nextCursor"`
	Limit   int    `json:"limit,omitempty" jsonschema:"versions per page, default 50, at most 200"`
}

type getAgentSkillVersionArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	Skill   string `json:"skill" jsonschema:"the skill's skl- id or its name"`
	Version int    `json:"version" jsonschema:"a version number from list_agent_skill_versions"`
}

type createAgentSkillArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	AgentSkillInput
}

type updateAgentSkillArgs struct {
	OwnerID string `json:"ownerId,omitempty"`
	Skill   string `json:"skill" jsonschema:"the skill's skl- id or its name"`
	AgentSkillInput
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
	mcputil.AddTool(server, &mcp.Tool{Name: "create_agent_session", Description: "Start a cloud coding-agent session on a repository. Returns the new session in phase creating; it does not return an attach ticket (call attach_agent_session for one). Pass outputSchema (a JSON Schema) to get a machine-readable result: the agent submits one that validates against it and outcome.result carries it, or the turn fails with failureReasonCode STRUCTURED_OUTPUT_MISSING. Pass secretNames (workspace agent secrets) and secrets (inline, this session only) to give the agent API keys as env vars; values are redacted from the transcript. Pass skills to make the turn follow workspace skills (list_agent_skills)."}, createSession)
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
				Repo: in.Repo, AutomationID: in.AutomationID, ParentSessionID: in.ParentSessionID, Mine: in.Mine, CreatedBy: in.CreatedBy,
				CreatedBefore: before, CreatedAfter: after,
				Cursor: in.Cursor, Limit: in.Limit,
			})
			return nil, listResult{AgentSessions: out}, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_session", Description: "Inspect one cloud coding-agent session."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, View, error) {
			out, err := s.Get(ctx, in.ID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_session_tree", Description: "Read the whole tree of cloud coding-agent sessions a session belongs to (ADR102): pass any session id in the tree, get its root (rootSessionId) and every session below it (sessions), the root first and then by depth and creation time, each with its parentSessionId, depth, phase, state and outcome. Use it to see every child a session started in one call; list_agent_sessions with parentSessionId lists just the direct children."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idArgs) (*mcp.CallToolResult, Tree, error) {
			out, err := s.Tree(ctx, in.ID)
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
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_environment", Description: "Read a workspace's agent environment: the sandbox network tier (registries = GitHub plus the built-in package registries for the whole session, the default; custom = registries plus customHosts; full = any public host over HTTP(S) through the bex egress proxy, with model providers blocked and no raw TCP or SSH), the custom hostnames, the default model, whether the model is pinned by the platform model route, the model source (byo = the workspace's own model provider key, platform = bex-provided and billed at the provider's cost + 30%, none) with modelKeySet (the key itself is never returned), and the agent-model spend controls: the effective monthly spend cap in USD (min of the admin override or plan default and the billing trust tier's ceiling), the plan default, the override, month-to-date spend, the trust tier (T0..T4) and the next charge threshold, whether a failed or disputed payment holds the tier (trustTierLocked), whether the tier's ceiling sets the cap (agentModelSpendCapTierLimited), whether new platform-model requests are refused at the cap now (agentModelSpendCapReached: false while the next request would charge the card at the tier's threshold instead), whether reaching the next charge threshold charges the card and the workspace keeps working (agentModelChargesAtThreshold), whether an unpaid failed invoice refuses platform-model requests (billingBalanceDue), the agent profiles members can start sessions with (enabledAgents), and each profile whose model provider may train on what its sessions send with the admin acknowledgement turning it on requires (agentDataUse)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in environmentArgs) (*mcp.CallToolResult, Environment, error) {
			out, err := s.Environment(ctx, in.OwnerID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "update_agent_environment", Description: "Update a workspace's agent environment (workspace admins only). Omitted fields keep their value: networkTier registries|custom|full; customHosts replaces the list (exact public hostnames, at most 32; [] clears); defaultModel \"\" returns to the provider default and cannot differ from a pinned model. agentModelMonthlySpendCapUsd sets the monthly agent-model spend cap in USD (more than 0, at most 10x the plan default; null or resetAgentModelMonthlySpendCap: true returns to the plan default). modelApiKey (write-only) sets or rotates the workspace's own model provider key, which then wins over the bex-provided model (no token billing, no spend cap); clearModelApiKey: true removes it. enabledAgents replaces the agent profiles members can start sessions with (the others are turned off; running sessions keep running); a profile listed in agentDataUse can be enabled only with acknowledgeAgentDataUse naming it, now or earlier. The network tier and hosts apply from each session's next turn; the default model applies to sessions created afterwards; the spend cap applies within 20 seconds."},
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
	mcputil.AddTool(server, &mcp.Tool{Name: "set_agent_session_tags", Description: "Replace all tags on a cloud coding-agent session (tags filter list_agent_sessions). Pass the complete new set; [] clears them. Refused on an archived session."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setTagsArgs) (*mcp.CallToolResult, View, error) {
			if in.Tags == nil {
				return nil, View{}, errTagsRequired() // null is not [] here, as on REST
			}
			out, err := s.ReplaceTags(ctx, in.ID, in.Tags)
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
		out, err := s.Steer(ctx, SteerRequest{SessionID: in.ID, Prompt: in.Prompt, EgressAllowlist: in.EgressAllowlist, AttachmentIDs: in.AttachmentIDs, OutputSchema: in.OutputSchema, OutputRequired: in.OutputRequired, SessionSecretsInput: in.SessionSecretsInput, Skills: in.Skills})
		return nil, out, err
	}
	mcputil.AddTool(server, &mcp.Tool{Name: "send_agent_session_message", Description: "Send a follow-up message to a cloud coding-agent session: it runs a new turn, re-dispatches the sandbox on the same branch and updates the same draft PR. The session must be steerable (state succeeded or failed, not archived or canceled). Pass outputSchema to require a machine-readable result for this turn, read from outcome.result."}, sendMessage)
	mcputil.AddTool(server, &mcp.Tool{Name: "steer_agent_session", Description: "Alias of send_agent_session_message. Run a follow-up prompt turn on a cloud coding-agent session; it re-dispatches the sandbox on the same branch and updates the same draft PR."}, sendMessage)
	s.registerAttachmentMCP(server)
	s.registerSecretMCP(server)
	s.registerSkillMCP(server)
	s.registerAutomationMCP(server)
}

// registerSkillMCP adds the ADR099 D3 workspace skill tools.
func (s *Service) registerSkillMCP(server *mcp.Server) {
	mcputil.AddTool(server, &mcp.Tool{Name: "list_agent_skills", Description: "List a workspace's skills for cloud agents: reusable SKILL.md instructions (the Agent Skills format) every session receives. Shows name, description, triggers and enabled; read one with get_agent_skill. Pass names in skills to create_agent_session or send_agent_session_message to make a turn follow one."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in environmentArgs) (*mcp.CallToolResult, AgentSkillList, error) {
			out, err := s.AgentSkills(ctx, in.OwnerID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_skill", Description: "Read one workspace skill by skl- id or name, with its body and its whole SKILL.md (content)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in agentSkillArgs) (*mcp.CallToolResult, AgentSkill, error) {
			out, err := s.AgentSkill(ctx, in.OwnerID, in.Skill)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "list_agent_skill_versions", Description: "List a workspace skill's saved versions, newest first: version, short id, message, author, time and which one is current. Every save of the skill keeps a version (at most the newest 200). Read one with get_agent_skill_version; page with cursor (the previous nextCursor)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listAgentSkillVersionsArgs) (*mcp.CallToolResult, AgentSkillVersionList, error) {
			out, err := s.AgentSkillVersions(ctx, in.OwnerID, in.Skill, in.Cursor, in.Limit)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "get_agent_skill_version", Description: "Read one saved version of a workspace skill: its whole SKILL.md (content) and a unified line diff against the previous version. To restore a version, pass its content to update_agent_skill with ifVersion set to the skill's current version; that writes a new version."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in getAgentSkillVersionArgs) (*mcp.CallToolResult, AgentSkillVersion, error) {
			out, err := s.AgentSkillVersion(ctx, in.OwnerID, in.Skill, in.Version)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "create_agent_skill", Description: "Add a workspace skill (workspace admins only): name, description and body, or a whole SKILL.md as content. Skills are instructions, not secrets: every member can read them. triggers [\"user\"] applies it only when invoked. At most 100 per workspace; enabled skills total at most 1 MiB."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in createAgentSkillArgs) (*mcp.CallToolResult, AgentSkill, error) {
			out, err := s.CreateAgentSkill(ctx, in.OwnerID, in.AgentSkillInput)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "update_agent_skill", Description: "Change a workspace skill (workspace admins only); omitted fields keep their value. Pass ifVersion to refuse the write if someone changed it since you read it. Sessions use the new version from their next turn."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateAgentSkillArgs) (*mcp.CallToolResult, AgentSkill, error) {
			out, err := s.UpdateAgentSkill(ctx, in.OwnerID, in.Skill, in.AgentSkillInput)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "delete_agent_skill", Description: "Delete a workspace skill (workspace admins only)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in agentSkillArgs) (*mcp.CallToolResult, deleteResult, error) {
			if err := s.DeleteAgentSkill(ctx, in.OwnerID, in.Skill); err != nil {
				return nil, deleteResult{}, err
			}
			return nil, deleteResult{Deleted: true}, nil
		})
}

// registerSecretMCP adds the ADR098 D4 workspace agent secret tools. Values
// are write-only: no tool returns one.
func (s *Service) registerSecretMCP(server *mcp.Server) {
	mcputil.AddTool(server, &mcp.Tool{Name: "list_agent_secrets", Description: "List a workspace's agent secrets with their scopes (values are never returned). A workspace-scoped secret reaches every session and a repo-scoped one every session on its repository, as env vars of the same name; pass an explicit secret's name in secretNames to create_agent_session or send_agent_session_message to expose it."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in environmentArgs) (*mcp.CallToolResult, AgentSecretList, error) {
			out, err := s.AgentSecrets(ctx, in.OwnerID)
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "set_agent_secret", Description: "Create, rotate or rescope a workspace agent secret (workspace admins only). The value is write-only: no read returns it, and it is redacted from session transcripts. A new secret is workspace-wide unless scope says otherwise. Sessions read the new value and scope from their next turn. At most 50 per workspace."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setAgentSecretArgs) (*mcp.CallToolResult, AgentSecret, error) {
			out, err := s.SetAgentSecret(ctx, in.OwnerID, in.Name, AgentSecretInput{Value: in.Value, Scope: in.Scope, Repo: in.Repo})
			return nil, out, err
		})
	mcputil.AddTool(server, &mcp.Tool{Name: "delete_agent_secret", Description: "Delete a workspace agent secret (workspace admins only). A session that still references it fails its next turn with failureReasonCode AGENT_SESSION_SECRET_UNAVAILABLE."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in agentSecretArgs) (*mcp.CallToolResult, deleteResult, error) {
			if err := s.DeleteAgentSecret(ctx, in.OwnerID, in.Name); err != nil {
				return nil, deleteResult{}, err
			}
			return nil, deleteResult{Deleted: true}, nil
		})
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
