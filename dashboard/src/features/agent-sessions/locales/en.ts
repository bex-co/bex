import type { TranslationEntry } from "@/i18n";

const enAgentSessions: Record<string, TranslationEntry> = {
  // --- Page / list ---
  "agentSessions.pageTitle": {
    message: "Agent sessions",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.pageSubtitle": {
    message:
      "Assign a coding task to a cloud agent. It works in a sandbox on a bex-agent/* branch and opens a draft PR.",
    description: "Agent sessions page subtitle explaining the feature",
  },
  "agentSessions.listTitle": {
    message: "Sessions",
    description: "Agent sessions table card title",
  },
  "agentSessions.colTask": {
    message: "Task",
    description: "Agent sessions table column header — the session's prompt",
  },
  "agentSessions.colRepo": {
    message: "Repository",
    description: "Agent sessions table column header — repo/branch",
  },
  "agentSessions.colAgent": {
    message: "Agent",
    description: "Agent sessions table column header — the driver agent",
  },
  "agentSessions.colPhase": {
    message: "Phase",
    description: "Agent sessions table column header — lifecycle phase chip",
  },
  "agentSessions.colPr": {
    message: "PR",
    description: "Agent sessions table column header — draft pull request",
  },
  "agentSessions.colCreated": {
    message: "Created",
    description: "Agent sessions table column header — createdAt relative age",
  },
  "agentSessions.prBadge": {
    message: "#{number}",
    description: "Draft PR badge label — the pull-request number",
  },
  "agentSessions.errorTitle": {
    message: "Couldn't load agent sessions",
    description: "Agent sessions list error state heading",
  },
  // --- Empty state ---
  "agentSessions.emptyTitle": {
    message: "No agent sessions yet",
    description: "Agent sessions list empty-state heading",
  },
  "agentSessions.emptyBody": {
    message:
      "Start a session below — describe a task, pick a repository and an agent, and it works the task in a cloud sandbox, then opens a draft PR.",
    description: "Agent sessions list empty-state body",
  },
  // --- Phase chips ---
  "agentSessions.phase.creating": {
    message: "Creating",
    description: "Agent session phase chip — sandbox is being created",
  },
  "agentSessions.phase.running": {
    message: "Running",
    description: "Agent session phase chip — a turn is in progress",
  },
  "agentSessions.phase.resuming": {
    message: "Resuming",
    description: "Agent session phase chip — session is resuming",
  },
  "agentSessions.phase.redispatching": {
    message: "Redispatching",
    description: "Agent session phase chip — session is being redispatched",
  },
  "agentSessions.phase.completed": {
    message: "Completed",
    description: "Agent session phase chip — session finished successfully",
  },
  "agentSessions.phase.failed": {
    message: "Failed",
    description: "Agent session phase chip — session failed",
  },
  "agentSessions.phase.canceling": {
    message: "Canceling",
    description: "Agent session phase chip — cancellation in progress",
  },
  "agentSessions.phase.canceled": {
    message: "Canceled",
    description: "Agent session phase chip — session was canceled",
  },
  // --- Composer ---
  "agentSessions.composerTitle": {
    message: "Start a session",
    description: "New-session composer card title",
  },
  "agentSessions.taskLabel": {
    message: "Task",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message:
      "Describe what the agent should do. Be specific — it works autonomously and opens a draft PR.",
    description: "Composer — task textarea placeholder",
  },
  "agentSessions.taskRequired": {
    message: "Describe the task for the agent.",
    description: "Composer — validation error when the task is empty",
  },
  "agentSessions.repoLabel": {
    message: "Repository",
    description: "Composer — repository (owner/name) input label",
  },
  "agentSessions.repoPlaceholder": {
    message: "owner/name",
    description: "Composer — repository input placeholder",
  },
  "agentSessions.repoRequired": {
    message: "Enter a repository as owner/name.",
    description: "Composer — validation error when the repo is empty",
  },
  "agentSessions.repoHint": {
    message: "The GitHub repository the agent works against.",
    description: "Composer — repository field helper text",
  },
  "agentSessions.branchLabel": {
    message: "Branch",
    description: "Composer — working branch input label",
  },
  "agentSessions.branchPlaceholder": {
    message: "bex-agent/my-task",
    description: "Composer — branch input placeholder",
  },
  "agentSessions.branchRequired": {
    message: "Enter a working branch.",
    description: "Composer — validation error when the branch is empty",
  },
  "agentSessions.branchHint": {
    message:
      "The agent commits to a bex-agent/* branch and opens a draft PR from it.",
    description: "Composer — branch field helper text (bex-agent/* guidance)",
  },
  "agentSessions.agentLabel": {
    message: "Agent",
    description: "Composer — agent select label",
  },
  "agentSessions.agentClaude": {
    message: "Claude",
    description: "Composer — agent option: claude",
  },
  "agentSessions.agentGemini": {
    message: "Gemini",
    description: "Composer — agent option: gemini",
  },
  "agentSessions.agentCodex": {
    message: "Codex",
    description: "Composer — agent option: codex",
  },
  "agentSessions.advancedShow": {
    message: "Advanced",
    description: "Composer — toggle that expands the advanced fields",
  },
  "agentSessions.advancedHide": {
    message: "Hide advanced",
    description: "Composer — toggle that collapses the advanced fields",
  },
  "agentSessions.modelLabel": {
    message: "Model",
    description: "Composer (advanced) — model override input label",
  },
  "agentSessions.modelPlaceholder": {
    message: "Provider default",
    description: "Composer (advanced) — model input placeholder",
  },
  "agentSessions.modelHint": {
    message: "Optional model override for the selected agent.",
    description: "Composer (advanced) — model field helper text",
  },
  "agentSessions.modelEndpointLabel": {
    message: "Model endpoint",
    description: "Composer (advanced) — model endpoint input label",
  },
  "agentSessions.modelEndpointPlaceholder": {
    message: "https://api.example.com",
    description: "Composer (advanced) — model endpoint input placeholder",
  },
  "agentSessions.modelEndpointHint": {
    message: "Optional custom HTTPS endpoint for the model provider.",
    description: "Composer (advanced) — model endpoint field helper text",
  },
  "agentSessions.egressLabel": {
    message: "Egress allowlist",
    description: "Composer (advanced) — extra egress hostnames textarea label",
  },
  "agentSessions.egressPlaceholder": {
    message: "one hostname per line",
    description: "Composer (advanced) — egress allowlist textarea placeholder",
  },
  "agentSessions.egressHint": {
    message:
      "Extra public hostnames the sandbox may reach (up to 32), beyond the model endpoint and the built-in package registries.",
    description: "Composer (advanced) — egress allowlist field helper text",
  },
  "agentSessions.egressTooMany": {
    message: "Too many hostnames — the allowlist allows at most 32 entries.",
    description: "Composer — validation error when more than 32 egress entries",
  },
  "agentSessions.submit": {
    message: "Start session",
    description: "Composer — submit button label",
  },
  "agentSessions.submitting": {
    message: "Starting…",
    description: "Composer — submit button label while the create is in flight",
  },
  // --- Unavailable (503) + create error ---
  "agentSessions.unavailableTitle": {
    message: "Agent sessions aren't configured",
    description: "Composer — house callout title when the feature is 503",
  },
  "agentSessions.unavailableBody": {
    message:
      "This platform hasn't enabled cloud agent sessions. Ask your operator to configure the agent-session gateway.",
    description: "Composer — house callout body when the feature is 503",
  },
  "agentSessions.createErrorTitle": {
    message: "Couldn't start the session",
    description: "Composer — error alert title when the create fails",
  },
  // --- Detail page (t004) ---
  "agentSessions.detailTitle": {
    message: "Session",
    description: "Agent-session detail page document title",
  },
  "agentSessions.backToList": {
    message: "All sessions",
    description: "Detail page — back link to the /agents list",
  },
  "agentSessions.detailErrorTitle": {
    message: "Couldn't load this session",
    description: "Detail page — heading when the session read fails",
  },
  "agentSessions.conversationTitle": {
    message: "Conversation",
    description: "Detail page — the live conversation column card title",
  },
  "agentSessions.failureTitle": {
    message: "Session failed",
    description: "Detail page — heading over a failed session's reason",
  },
  // Header meta + cancel
  "agentSessions.metaDuration": {
    message: "Duration {duration}",
    description: "Detail header — elapsed session wall-clock",
  },
  "agentSessions.metaTurns": {
    message: "{turns} turns",
    description: "Detail header — number of prompt turns taken",
  },
  "agentSessions.metaDelivery": {
    message: "Delivery {mode}",
    description: "Detail header — how the last turn's sandbox was obtained",
  },
  "agentSessions.delivery.resume": {
    message: "resume",
    description: "Delivery mode — the sandbox was resumed",
  },
  "agentSessions.delivery.redispatch": {
    message: "redispatch",
    description: "Delivery mode — a new sandbox was dispatched",
  },
  "agentSessions.cancel": {
    message: "Cancel",
    description: "Detail header — cancel-session button label",
  },
  "agentSessions.cancelDisabledCanceling": {
    message: "Cancellation is already in progress.",
    description: "Detail header — tooltip on the disabled cancel button",
  },
  "agentSessions.canceling": {
    message: "Canceling…",
    description: "Detail header — confirm button label while the cancel runs",
  },
  "agentSessions.cancelSuccess": {
    message: "Canceling the session…",
    description: "Detail header — toast after the cancel is accepted",
  },
  "agentSessions.cancelConfirmTitle": {
    message: "Cancel this session?",
    description: "Cancel confirm dialog title",
  },
  "agentSessions.cancelConfirmBody": {
    message:
      "This stops the agent. Any commits already pushed to the branch and the draft PR are preserved.",
    description: "Cancel confirm dialog body — states pushed work is preserved",
  },
  "agentSessions.cancelConfirmDismiss": {
    message: "Keep running",
    description: "Cancel confirm dialog — dismiss button",
  },
  "agentSessions.cancelConfirmProceed": {
    message: "Cancel session",
    description: "Cancel confirm dialog — proceed button",
  },
  // PR card
  "agentSessions.prCardTitle": {
    message: "Pull request",
    description: "Detail page — draft-PR card title",
  },
  "agentSessions.prCardNone": {
    message:
      "No pull request yet. The agent opens a draft PR once it pushes work.",
    description: "PR card — shown before the session has opened a PR",
  },
  "agentSessions.prCardHeadSha": {
    message: "Head commit",
    description: "PR card — label for the head SHA",
  },
  // Evidence panel
  "agentSessions.evidenceTitle": {
    message: "Evidence",
    description: "Detail page — bounded evidence card title",
  },
  "agentSessions.evidenceEmpty": {
    message: "No evidence captured yet.",
    description: "Evidence panel — empty state",
  },
  "agentSessions.evidenceCommits": {
    message: "{count} commits",
    description: "Evidence panel — commit count",
  },
  "agentSessions.evidenceCommandLog": {
    message: "Command log",
    description: "Evidence panel — command log section label",
  },
  "agentSessions.evidenceTestOutput": {
    message: "Test output",
    description: "Evidence panel — test output section label",
  },
  "agentSessions.evidenceOutputTail": {
    message: "Output tail",
    description: "Evidence panel — output tail section label",
  },
  "agentSessions.evidenceChangedFiles": {
    message: "Changed files",
    description: "Evidence panel — changed-files section label",
  },
  "agentSessions.evidenceTruncated": {
    message: "Some captured output was truncated.",
    description: "Evidence panel — honest truncation note",
  },
  // Steering composer
  "agentSessions.steerTitle": {
    message: "Steer this session",
    description: "Steering composer card title",
  },
  "agentSessions.steerErrorTitle": {
    message: "Couldn't send",
    description: "Steering composer — inline error alert title",
  },
  "agentSessions.steerPlaceholderIdle": {
    message:
      "Send a follow-up instruction to start a new turn on the same branch.",
    description: "Steering composer — textarea placeholder for an idle session",
  },
  "agentSessions.steerPlaceholderLive": {
    message: "Send a message to steer the running agent.",
    description: "Steering composer — textarea placeholder for a live session",
  },
  "agentSessions.steerHintIdle": {
    message:
      "This session is idle — sending redispatches a new turn on the same branch.",
    description: "Steering composer — helper text for the redispatch route",
  },
  "agentSessions.steerHintLive": {
    message: "Your message is sent as a live turn in the conversation.",
    description: "Steering composer — helper text for the live chat route",
  },
  "agentSessions.steerDisabledCanceling": {
    message:
      "This session is being canceled. Work already pushed is preserved.",
    description: "Steering composer — disabled reason while canceling",
  },
  "agentSessions.steerDisabledCanceled": {
    message: "This session was canceled.",
    description: "Steering composer — disabled reason once canceled",
  },
  "agentSessions.steerDisabledStream": {
    message:
      "The conversation stream is unavailable, so live steering is paused.",
    description:
      "Steering composer — disabled reason when the m43 stream is down",
  },
  "agentSessions.steerDisabledInFlight": {
    message:
      "A turn is in progress. Wait for it to finish before sending another.",
    description: "Steering composer — disabled reason while a turn streams",
  },
  "agentSessions.steerSubmit": {
    message: "Send",
    description: "Steering composer — submit button label",
  },
  "agentSessions.steerSending": {
    message: "Sending…",
    description: "Steering composer — submit button label while sending",
  },
  "agentSessions.steerSuccess": {
    message: "Sent — redispatching a new turn.",
    description: "Steering composer — toast after a redispatch is accepted",
  },
  // --- Typed AGENT_SESSION_* error messages (keyed by errors.ts messageKey) ---
  "agentSessions.errors.AGENT_SESSION_INPUT_INVALID": {
    message:
      "That input isn't valid. Check the repository, branch, and task, then try again.",
    description: "Mapped message for AGENT_SESSION_INPUT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_ID_INVALID": {
    message: "That session id isn't valid.",
    description: "Mapped message for AGENT_SESSION_ID_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_FOUND": {
    message: "That session no longer exists.",
    description: "Mapped message for AGENT_SESSION_NOT_FOUND",
  },
  "agentSessions.errors.AGENT_SESSION_CONFLICT": {
    message: "This session is busy with another operation. Try again shortly.",
    description: "Mapped message for AGENT_SESSION_CONFLICT",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_STEERABLE": {
    message: "This session can't be steered in its current phase ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_STEERABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_RESUMABLE": {
    message: "This session can't be resumed in its current phase ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_RESUMABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_ATTACHABLE": {
    message: "This session can't be attached in its current phase ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_ATTACHABLE",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_IN_FLIGHT": {
    message:
      "A turn is already running. Wait for it to finish before sending another.",
    description: "Mapped message for AGENT_SESSION_TURN_IN_FLIGHT",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_ENDPOINT_INVALID": {
    message: "The model endpoint must be a valid HTTPS URL.",
    description: "Mapped message for AGENT_SESSION_MODEL_ENDPOINT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_INVALID": {
    message: 'Egress allowlist entry "{entry}" is invalid: {reason}',
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE": {
    message: "The egress allowlist can't be changed after the session starts.",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_PHASE_INVALID": {
    message:
      "The egress allowlist can't be set in this session's current phase.",
    description: "Mapped message for AGENT_SESSION_EGRESS_PHASE_INVALID",
  },

  // --- Conversation column (t002) ---
  "agentSessions.conversationLoading": {
    message: "Loading conversation…",
    description: "Placeholder while the client-only conversation column loads",
  },
  "agentSessions.conversationConnecting": {
    message: "Connecting to the session stream…",
    description: "Shown while the conversation stream replay is in flight",
  },
  "agentSessions.conversationEmpty": {
    message: "No conversation yet.",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.conversationEnded": {
    message: "Session ended.",
    description: "Footer note under a terminal session's replayed transcript",
  },
  "agentSessions.conversationUnavailable": {
    message: "The conversation stream is unavailable right now.",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.groupThought": {
    message: "Thought",
    description: "Collapsible header for the agent's reasoning group",
  },
  "agentSessions.groupPlan": {
    message: "Plan",
    description: "Collapsible header for the agent's plan/task checklist group",
  },
  "agentSessions.groupTerminal": {
    message: "Terminal",
    description: "Collapsible header for a terminal-output group",
  },
  "agentSessions.groupCommand": {
    message: "Command",
    description: "Collapsible header for a command/tool-call group",
  },
  "agentSessions.groupDiff": {
    message: "Diff",
    description: "Label for a file-diff group when no path is given",
  },
  "agentSessions.terminalNoOutput": {
    message: "(no output)",
    description: "Placeholder when a terminal group has no captured output",
  },
  "agentSessions.toolInput": {
    message: "Input",
    description: "Section label for a tool call's input",
  },
  "agentSessions.toolOutput": {
    message: "Output",
    description: "Section label for a tool call's output",
  },
  "agentSessions.toolError": {
    message: "Error",
    description: "Section label for a tool call's error text",
  },
  "agentSessions.toolStateRunning": {
    message: "Running",
    description: "Badge label for a tool call awaiting output",
  },
  "agentSessions.toolStateDone": {
    message: "Done",
    description: "Badge label for a completed tool call",
  },
  "agentSessions.toolStateError": {
    message: "Failed",
    description: "Badge label for a failed tool call",
  },
};

export default enAgentSessions;
