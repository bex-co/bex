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
      "Start a session from New session — describe a task and @-mention a repository, and the agent works it in a cloud sandbox, then opens a draft PR.",
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
  // --- Composer (prompt box) ---
  "agentSessions.promptHeading": {
    message: "What should the agent work on?",
    description: "Centered heading over the /agents prompt-box composer",
  },
  "agentSessions.taskLabel": {
    message: "Task",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message:
      "Describe a task, and @-mention a repository to scope it. Be specific — the agent works autonomously and opens a draft PR.",
    description: "Composer — task textarea placeholder",
  },
  "agentSessions.mentionButton": {
    message: "Mention a repository or session",
    description: "Composer toolbar — accessible label of the @ mention button",
  },
  "agentSessions.configButton": {
    message: "Configuration",
    description: "Composer toolbar — the Configuration popover trigger",
  },
  "agentSessions.repoNudge": {
    message: "Pick a repository with @ first.",
    description:
      "Inline nudge anchored at the @ button when submitting without a repo chip",
  },
  "agentSessions.chipRemove": {
    message: "Remove {name}",
    description: "Accessible label of a mention chip's remove button",
  },
  // --- @ mention picker ---
  "agentSessions.mentionCategoryRepos": {
    message: "Repositories",
    description: "Mention picker — the repositories category row",
  },
  "agentSessions.mentionCategoryReposDesc": {
    message: "Scope the session to a connected GitHub repository",
    description: "Mention picker — repositories category one-line description",
  },
  "agentSessions.mentionCategorySessions": {
    message: "Sessions",
    description: "Mention picker — the prior-sessions category row",
  },
  "agentSessions.mentionCategorySessionsDesc": {
    message: "Reference an earlier agent session as context",
    description: "Mention picker — sessions category one-line description",
  },
  "agentSessions.mentionNoResults": {
    message: "No matches",
    description:
      "Mention picker — empty state when the fuzzy filter matches nothing",
  },
  "agentSessions.mentionReposEmpty": {
    message: "No repositories — connect GitHub in workspace settings first.",
    description:
      "Mention picker — empty state when no installation repos exist",
  },
  "agentSessions.mentionSessionsEmpty": {
    message: "No sessions yet",
    description:
      "Mention picker — empty state when the workspace has no sessions",
  },
  "agentSessions.mentionConnected": {
    message: "Connected via GitHub App",
    description: "Mention picker readiness footer — the repo is app-connected",
  },
  "agentSessions.mentionDefaultBranch": {
    message: "Default branch: {branch}",
    description: "Mention picker readiness footer — the repo's default branch",
  },
  "agentSessions.branchLabel": {
    message: "Branch",
    description: "Composer — working branch input label",
  },
  "agentSessions.branchInvalid": {
    message: "The branch must be under bex-agent/.",
    description:
      "Composer — validation error when the branch leaves the bex-agent/* namespace",
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
  "agentSessions.agent.claude": {
    message: "Claude",
    description: "Composer — agent option: claude",
  },
  "agentSessions.agent.gemini": {
    message: "Gemini",
    description: "Composer — agent option: gemini",
  },
  "agentSessions.agent.codex": {
    message: "Codex",
    description: "Composer — agent option: codex",
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
  "agentSessions.failureReasonFallback": {
    message: "The session failed to start.",
    description:
      "Detail page — generic failure text when the session carries no specific reason",
  },
  "agentSessions.failureRetry": {
    message: "Retry",
    description:
      "Detail page — button to re-run a failed session's original task",
  },
  "agentSessions.failureRetrying": {
    message: "Retrying…",
    description:
      "Detail page — retry button label while the redispatch is in flight",
  },
  "agentSessions.provisioning": {
    message: "Starting the sandbox…",
    description:
      "Detail page — shown while a new/steered session provisions its sandbox before the conversation stream is available",
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
  "agentSessions.showEarlierMessages": {
    message: "Show {count} earlier messages",
    description:
      "Button revealing older transcript messages hidden by the render window",
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
  "agentSessions.activityWorking": {
    message: "Working…",
    description: "Activity-group summary while any tool step is still pending",
  },
  "agentSessions.activityEdited": {
    message: "Edited {count} files",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan": {
    message: "Ran {count} commands",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps": {
    message: "{count} steps",
    description: "Activity-group summary fallback — the step count",
  },
  "agentSessions.scrollToBottom": {
    message: "Scroll to latest",
    description:
      "Accessible label for the floating jump-to-bottom button in the conversation column",
  },
  // --- Full-page chat restructure (w3/m44) ---
  "agentSessions.recentSessions": {
    message: "Recent",
    description: "Sessions sidebar — heading over the recent sessions list",
  },
  "agentSessions.sidebarEmpty": {
    message: "No sessions yet",
    description: "Sessions sidebar — empty state when the workspace has none",
  },
  "agentSessions.sidebarLabel": {
    message: "Agent sessions",
    description: "Sessions sidebar — accessible landmark label",
  },
  // --- Sidebar ---
  "agentSessions.sidebarSearch": {
    message: "Search sessions",
    description: "Sessions sidebar — search toggle + input accessible label",
  },
  "agentSessions.sidebarSearchPlaceholder": {
    message: "Search sessions…",
    description: "Sessions sidebar — search input placeholder",
  },
  "agentSessions.sidebarMore": {
    message: "View all sessions",
    description: "Sessions sidebar — More action reaching the standalone list",
  },
  "agentSessions.sidebarNoMatches": {
    message: "No matching sessions",
    description:
      "Sessions sidebar — empty state when the search matches nothing",
  },
  "agentSessions.statusPhrase.prReady": {
    message: "PR is ready",
    description: "Sidebar status phrase — completed session with a draft PR",
  },
  "agentSessions.statusPhrase.working": {
    message: "Working…",
    description: "Sidebar status phrase — session still converging",
  },
  "agentSessions.menuMore": {
    message: "More actions",
    description: "Header — accessible label for the '…' overflow menu",
  },
  "agentSessions.openPr": {
    message: "Open pull request",
    description: "Header overflow menu — open the draft PR in a new tab",
  },
  "agentSessions.connect": {
    message: "Connect",
    description: "Header — trigger for the Open-in-Zed / SSH connect menu",
  },
  "agentSessions.openInZed": {
    message: "Open in Zed",
    description:
      "Connect menu — hotlink that opens the sandbox as a Zed remote project",
  },
  "agentSessions.openInZedHint": {
    message: "Opens the sandbox's /workspace over SSH. Requires the Zed editor.",
    description: "Connect menu — helper text under the Open-in-Zed action",
  },
  "agentSessions.connectSSH": {
    message: "SSH",
    description: "Connect menu — label above the copyable ssh command",
  },
  "agentSessions.sshCopy": {
    message: "Copy SSH command",
    description: "Connect menu — accessible label for the copy button",
  },
  "agentSessions.sshCopied": {
    message: "SSH command copied",
    description: "Connect menu — toast after copying the ssh command",
  },
  "agentSessions.sshCopyError": {
    message: "Couldn't copy",
    description: "Connect menu — toast when copying the ssh command fails",
  },
  "agentSessions.groupWorkedFor": {
    message: "Worked for {duration}",
    description: "Activity-group summary with a derived elapsed duration",
  },
  "agentSessions.groupWorked": {
    message: "Worked",
    description:
      "Activity-group summary when no duration could be derived (history replay)",
  },
  "agentSessions.groupThoughtFor": {
    message: "Thought for {duration}",
    description: "Reasoning-group summary with a derived elapsed duration",
  },
  "agentSessions.terminalStatus.completed": {
    message: "Session went to sleep",
    description: "Terminal transcript status line — completed session",
  },
  "agentSessions.terminalStatus.failed": {
    message: "Session ended with an error",
    description: "Terminal transcript status line — failed session",
  },
  "agentSessions.terminalStatus.canceled": {
    message: "Session was canceled",
    description: "Terminal transcript status line — canceled session",
  },
};

export default enAgentSessions;
