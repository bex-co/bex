import type { TranslationEntry } from "@/i18n";

const enAgentSessions: Record<string, TranslationEntry> = {
  // --- Page / list ---
  "agentSessions.pageTitle": {
    message: "Agent sessions",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.listTitle": {
    message: "Recent",
    description: "Agent sessions recents heading — matches the rail",
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
    message: "Couldn't load sessions.",
    description: "Agent sessions list error — one sentence, Retry beside it",
  },
  "agentSessions.retry": {
    message: "Retry",
    description: "Retries a failed agent-sessions list request",
  },
  // --- Empty state ---
  "agentSessions.emptyBody": {
    message: "Sessions you start will show up here.",
    description: "Default empty — one quiet line under the composer",
  },
  "agentSessions.emptyArchivedTitle": {
    message: "No archived sessions",
    description: "Archived-only list empty-state heading",
  },
  "agentSessions.emptyArchivedBody": {
    message: "Sessions you archive will appear here.",
    description: "Archived-only list empty-state body",
  },
  "agentSessions.emptyFilteredTitle": {
    message: "No matching sessions",
    description: "Phase-filtered list empty-state heading",
  },
  "agentSessions.emptyFilteredBody": {
    message: "Try another phase or clear the filters.",
    description: "Phase-filtered list empty-state body",
  },
  "agentSessions.clearFilters": {
    message: "Clear filters",
    description: "Clears agent-session membership and phase filters",
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
  "agentSessions.phase.hibernating": {
    message: "Hibernating",
    description: "Agent session phase chip — snapshot upload in progress",
  },
  "agentSessions.phase.hibernated": {
    message: "Hibernated",
    description: "Agent session phase chip — reclaimed to a durable snapshot",
  },
  // --- Hibernation (ADR059 D5/D6) ---
  "agentSessions.pin": {
    message: "Pin",
    description: "Button — pin a hibernated session so it never expires",
  },
  "agentSessions.unpin": {
    message: "Unpin",
    description: "Button — remove a session's never-expire pin",
  },
  "agentSessions.pinned": {
    message: "Pinned",
    description: "Badge — this session is pinned (never expires)",
  },
  "agentSessions.pinSuccess": {
    message: "Pinned — this workspace will not expire.",
    description: "Toast — pin succeeded",
  },
  "agentSessions.unpinSuccess": {
    message: "Unpinned — this workspace is back on the retention clock.",
    description: "Toast — unpin succeeded",
  },
  "agentSessions.hibernatedStorage": {
    message: "Hibernated · {size}",
    description: "Meta — hibernated state with snapshot storage size",
  },
  // --- Composer (prompt box) ---
  "agentSessions.taskLabel": {
    message: "Task",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message: "Describe a coding task…",
    description: "Composer — task editor placeholder",
  },
  "agentSessions.mentionButton": {
    message: "Mention a repository or session",
    description: "Composer toolbar — accessible label of the @ mention button",
  },
  "agentSessions.configButton": {
    message: "Advanced",
    description: "Composer toolbar — the Advanced popover trigger",
  },
  "agentSessions.addRepository": {
    message: "Add repository",
    description: "Composer toolbar — repo chip when none is selected",
  },
  "agentSessions.repoChip": {
    message: "Repository {repo}",
    description: "Composer toolbar — accessible name of the selected-repo chip",
  },
  "agentSessions.keyboardHint": {
    message: "Enter to start · Shift+Enter for a new line · @ for a repo",
    description: "Muted hint under the composer",
  },
  "agentSessions.connectGitHubTitle": {
    message: "Connect GitHub to start",
    description: "Composer callout when the workspace has no App repos",
  },
  "agentSessions.connectGitHubBody": {
    message:
      "Cloud agents need a connected GitHub repository. Connect the GitHub App for this workspace, then come back to start a session.",
    description: "Composer callout body when there are no installation repos",
  },
  "agentSessions.connectGitHub": {
    message: "Connect GitHub",
    description: "Composer CTA that starts the GitHub App install",
  },
  "agentSessions.connectGitHubSettings": {
    message: "Workspace settings",
    description: "Secondary link to /workspace/settings from the GitHub CTA",
  },
  "agentSessions.exampleFixTests": {
    message: "Fix the failing tests and open a draft PR",
    description: "First-run example prompt chip",
  },
  "agentSessions.exampleAddReadme": {
    message: "Add a README that explains how to run the project",
    description: "First-run example prompt chip",
  },
  "agentSessions.repoNudge": {
    message: "Pick a repository first.",
    description:
      "Inline nudge when submitting without a repo while repos exist",
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
  "agentSessions.metaTurn": {
    message: "{turns} turn",
    description: "Detail header — number of prompt turns taken (singular)",
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
      "This session is idle — sending a message redispatches a new turn on the same branch.",
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
  "agentSessions.steerDisabledWaitForTurn": {
    message: "Wait for the current turn to finish before sending a follow-up.",
    description:
      "Composer disabled reason while the current durable agent turn is running",
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
  "agentSessions.conversationIncomplete": {
    message: "Some assistant output could not be preserved",
    description:
      "Warning below a durable user turn whose assistant transcript is incomplete",
  },
  "agentSessions.conversationUnavailable": {
    message: "The conversation stream is unavailable right now.",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.conversationUnavailableTerminal": {
    message: "The conversation transcript is not available for this session.",
    description:
      "Terminal session — transcript unavailable rather than live stream degraded",
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
    message:
      "Opens the sandbox's /workspace over SSH. Requires the Zed editor.",
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
    description:
      "Activity-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.groupWorked": {
    message: "Worked",
    description:
      "Activity-group summary when no duration could be derived (history replay)",
  },
  "agentSessions.groupThoughtFor": {
    message: "Thought for {duration}",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
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
  "agentSessions.archive": {
    message: "Archive",
    description: "Action putting a session out of the working set (ADR065)",
  },
  "agentSessions.unarchive": {
    message: "Unarchive",
    description: "Action returning an archived session to the working set",
  },
  "agentSessions.archiveSuccess": {
    message: "Session archived",
    description: "Toast after archiving a session",
  },
  "agentSessions.undoArchive": {
    message: "Undo",
    description: "Toast action that immediately unarchives a session",
  },
  "agentSessions.unarchiveSuccess": {
    message: "Session unarchived",
    description: "Toast after unarchiving a session",
  },
  "agentSessions.archivedBadge": {
    message: "Archived",
    description: "Badge marking a session as archived (out of the working set)",
  },
  "agentSessions.sidebarArchived": {
    message: "Archived",
    description: "Sidebar link to the archived-sessions list",
  },
  "agentSessions.delete": {
    message: "Delete",
    description: "Destructive action permanently deleting a finished session",
  },
  "agentSessions.deleting": {
    message: "Deleting…",
    description: "Delete confirm button while the delete is in flight",
  },
  "agentSessions.deleteSuccess": {
    message: "Session deleted",
    description: "Toast after permanently deleting a session",
  },
  "agentSessions.deleteConfirmTitle": {
    message: "Delete this session?",
    description: "Delete confirmation dialog title",
  },
  "agentSessions.deleteConfirmBody": {
    message:
      "The session record, its conversation transcript, and any hibernation snapshot will be permanently deleted. Pushed branches and pull requests on GitHub are not affected. This cannot be undone.",
    description: "Delete confirmation dialog body",
  },
  "agentSessions.deleteConfirmDismiss": {
    message: "Keep session",
    description: "Delete confirmation dialog dismiss button",
  },
  "agentSessions.deleteConfirmProceed": {
    message: "Delete session",
    description: "Delete confirmation dialog destructive proceed button",
  },
  "agentSessions.colActions": {
    message: "Actions",
    description: "Screen-reader label of the list's trailing actions column",
  },
  "agentSessions.filterActive": {
    message: "Recent",
    description: "List membership tab — the unarchived working set",
  },
  "agentSessions.filterArchived": {
    message: "Archived",
    description: "List membership tab — archived sessions only",
  },
  "agentSessions.filterAll": {
    message: "All",
    description: "List membership tab — archived and unarchived together",
  },
  "agentSessions.loadMore": {
    message: "Load more",
    description: "Button fetching the next page of sessions",
  },
  "agentSessions.loadingMore": {
    message: "Loading…",
    description: "Load-more button while the next page is in flight",
  },
};

export default enAgentSessions;
