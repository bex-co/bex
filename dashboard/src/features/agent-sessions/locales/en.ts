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
    message: "Add repository or session",
    description:
      "Composer toolbar — opens the repository/session mention picker",
  },
  "agentSessions.repoChip": {
    message: "Repository {repo}",
    description: "Composer toolbar — accessible name of the selected-repo chip",
  },
  "agentSessions.keyboardHint": {
    message: "Enter to start · Shift+Enter for a new line",
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
  "agentSessions.agent.muse": {
    message: "Muse Code",
    description: "Composer — agent option: muse (Meta's Muse Code)",
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
  "agentSessions.dependencyUnavailableTitle": {
    message: "Agent sessions are temporarily unavailable",
    description:
      "Composer — retryable dependency-outage title, distinct from configuration",
  },
  "agentSessions.dependencyUnavailableBody": {
    message:
      "A platform dependency is having trouble right now. Your setup is still intact; wait a moment and try again.",
    description:
      "Composer — retryable dependency-outage body that does not send the user to an operator",
  },
  "agentSessions.modelKeyMissingTitle": {
    message: "Add a model provider key first",
    description:
      "Composer pre-flight title when the workspace has no model key",
  },
  "agentSessions.modelKeyMissingBody": {
    message:
      "This workspace has no model access yet. A workspace admin can add the workspace's own model provider key under Settings → Agents, then return here.",
    description:
      "Composer pre-flight body when the workspace has neither its own model key nor the bex-provided model (ADR091 D7)",
  },
  "agentSessions.modelKeyMissingAction": {
    message: "Open Settings → Agents",
    description:
      "Composer pre-flight link to the Settings → Agents model key field (/workspace/settings#agents)",
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
  "agentSessions.untitled": {
    message: "Untitled session",
    description:
      "Last-resort session name when a session carries neither a repository nor a task prompt",
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
  "agentSessions.deliveryNoteTitle": {
    message: "Changes not fully delivered",
    description:
      "Detail page — heading over a completed session's note that the agent left work uncommitted, unpushed or without a pull request",
  },
  "agentSessions.deliveryNoteHint": {
    message:
      "Send a follow-up message asking the agent to finish committing, pushing and updating the pull request.",
    description:
      "Detail page — the next step shown under an undelivered-work note",
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
  "agentSessions.capacityFailureTitle": {
    message: "Out of sandbox capacity",
    description:
      "Detail page — heading when a session failed on the plan's sandbox limit",
  },
  "agentSessions.capacityFailureBody": {
    message:
      "This workspace has reached its plan's limit on concurrent sandboxes. Upgrade the plan to run more at once, or stop an idle session and retry.",
    description:
      "Detail page — explains a plan-limit sandbox failure and the two remedies",
  },
  "agentSessions.upgradePlan": {
    message: "Upgrade plan",
    description:
      "Detail page — CTA on a capacity failure; opens the change-plan dialog",
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
  "agentSessions.metaTurns_one": {
    message: "{count} turn",
    description: "Detail header — number of prompt turns taken (singular)",
  },
  "agentSessions.metaTurns_other": {
    message: "{count} turns",
    description: "Detail header — number of prompt turns taken (plural)",
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
      "This session is idle — sending a message continues the conversation in a fresh sandbox on the same branch.",
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
  "agentSessions.errors.AGENT_SESSION_BRANCH_IN_USE": {
    message:
      "Another session is already working on this branch. Send that session a message, or use another bex-agent/ branch.",
    description: "Mapped message for AGENT_SESSION_BRANCH_IN_USE (w5/m382)",
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
  "agentSessions.conversationNotStarted": {
    message: "The conversation will appear when the sandbox is ready.",
    description:
      "Healthy pre-stream state while a newly created session is provisioning",
  },
  "agentSessions.conversationEmpty": {
    message: "No conversation yet.",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.turnError": {
    message: "Turn {turn} failed",
    description:
      "Recorded failure of an earlier turn; subsequent turns remain readable",
  },
  "agentSessions.continuitySessionLoad": {
    message: "Agent resumed with its restored conversation state.",
    description:
      "System line when a fresh agent generation reloaded its own saved session (ADR047 D3 ladder rung 1)",
  },
  "agentSessions.continuityReprime": {
    message: "Agent restarted — context rebuilt from the session history.",
    description:
      "System line when a fresh agent generation was primed from the durable transcript (ladder rung 2)",
  },
  "agentSessions.continuityTaskRedelivery": {
    message: "Agent restarted — the original task was re-delivered.",
    description:
      "System line when no prior turn ever reached the agent, so the task was re-sent (ladder rung 3)",
  },
  "agentSessions.conversationEndedEmpty": {
    message: "This session ended before a conversation was recorded.",
    description: "Ended session with no durable transcript parts",
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
  "agentSessions.showEarlierMessages_one": {
    message: "Show {count} earlier message",
    description:
      "Button revealing older transcript messages hidden by the render window (singular)",
  },
  "agentSessions.showEarlierMessages_other": {
    message: "Show {count} earlier messages",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
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
  "agentSessions.toolStateDenied": {
    message: "Denied",
    description: "Badge label for a tool call whose execution was denied",
  },
  "agentSessions.toolStateInterrupted": {
    message: "Interrupted",
    description:
      "Badge label for a tool call the session ended before it finished",
  },
  "agentSessions.toolStateError": {
    message: "Failed",
    description: "Badge label for a failed tool call",
  },
  "agentSessions.activityEdited_one": {
    message: "Edited {count} file",
    description:
      "Activity-group summary when the turn edited files (diffs, singular)",
  },
  "agentSessions.activityEdited_other": {
    message: "Edited {count} files",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan_one": {
    message: "Ran {count} command",
    description:
      "Activity-group summary when the turn ran commands/terminals (singular)",
  },
  "agentSessions.activityRan_other": {
    message: "Ran {count} commands",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps_one": {
    message: "{count} step",
    description: "Activity-group summary fallback — the step count (singular)",
  },
  "agentSessions.activitySteps_other": {
    message: "{count} steps",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activityRead_one": {
    message: "Read {count} file",
    description: "Activity-group summary segment — file reads (singular)",
  },
  "agentSessions.activityRead_other": {
    message: "Read {count} files",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activitySearched_one": {
    message: "Searched {count} time",
    description: "Activity-group summary segment — searches (singular)",
  },
  "agentSessions.activitySearched_other": {
    message: "Searched {count} times",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySubagents_one": {
    message: "Ran {count} subagent",
    description:
      "Activity-group summary segment — delegated subagent tasks (singular)",
  },
  "agentSessions.activitySubagents_other": {
    message: "Ran {count} subagents",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activityTools_one": {
    message: "Used {count} tool",
    description:
      "Activity-group summary segment — tools of no known kind (singular)",
  },
  "agentSessions.activityTools_other": {
    message: "Used {count} tools",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityFailed_one": {
    message: "{count} failed",
    description:
      "Activity-group summary segment — failed tool calls (singular)",
  },
  "agentSessions.activityFailed_other": {
    message: "{count} failed",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.outputClipped_one": {
    message: "{formatted} character clipped",
    description:
      "Badge on a tool output the server stored head+tail-capped (singular)",
  },
  "agentSessions.outputClipped_other": {
    message: "{formatted} characters clipped",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.showFullOutput": {
    message: "Show full output",
    description:
      "Button on a clipped tool output that loads the complete text the server kept",
  },
  "agentSessions.hideFullOutput": {
    message: "Hide full output",
    description: "Button that collapses a loaded full tool output",
  },
  "agentSessions.fullOutputNotRetained": {
    message: "Full output not retained",
    description:
      "Note on a clipped tool output whose complete text the server did not keep (clipped before full copies were stored)",
  },
  "agentSessions.fullOutputLoading": {
    message: "Loading full output…",
    description: "Status while a tool call's full output loads",
  },
  "agentSessions.fullOutputError": {
    message: "Couldn't load the full output.",
    description: "Inline error when loading a tool call's full output failed",
  },
  "agentSessions.fullOutputCopy": {
    message: "Copy full output",
    description:
      "Accessible label of the copy button on a loaded full tool output",
  },
  "agentSessions.fullOutputCopied": {
    message: "Full output copied",
    description: "Toast after copying a full tool output",
  },
  "agentSessions.fullOutputCopyError": {
    message: "Couldn't copy the full output",
    description: "Toast when copying a full tool output failed",
  },
  "agentSessions.conversationPartial": {
    message: "Conversation only partially loaded.",
    description:
      "Inline row when the transcript stream failed after some of it rendered",
  },
  "agentSessions.liveStatus": {
    message: "Agent status",
    description:
      "Accessible label for the single live status line under a streaming transcript",
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
  "agentSessions.groupThoughtFor": {
    message: "Thought for {duration}",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.terminalStatus.completed": {
    message: "Agent finished",
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
  "agentSessions.tagsLabel": {
    message: "Tags",
    description: "Accessible label for a session row's tag list (w21/002)",
  },
  "agentSessions.filterByTag": {
    message: "Show sessions tagged {tag}",
    description:
      "Accessible label of a tag chip that filters the session list to that tag",
  },
  "agentSessions.filterTagged": {
    message: "Tagged {tag}",
    description: "Session list heading while narrowed to one tag",
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
      "The session record and its conversation transcript will be permanently deleted. Pushed branches and pull requests on GitHub are not affected. This cannot be undone.",
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
  // --- Attachments (ADR095) ---
  "agentSessions.attachFiles": {
    message: "Attach files",
    description:
      "Paperclip button that opens the file picker in the agent composers",
  },
  "agentSessions.attachmentRemove": {
    message: "Remove {name}",
    description: "Accessible label of the remove button on an attachment chip",
  },
  "agentSessions.attachmentTooLarge": {
    message: "Larger than {max}",
    description: "Attachment chip error when a file exceeds the per-file limit",
  },
  "agentSessions.attachmentTooMany": {
    message: "Up to {max} files per message",
    description: "Attachment chip error when a message has too many files",
  },
  "agentSessions.attachmentDropHint": {
    message: "Drop files to attach",
    description: "Overlay shown while dragging files over a composer",
  },
  "agentSessions.sharedFileLabel": {
    message: "Shared a file",
    description: "Label above a file the agent shared with share_file",
  },
  "agentSessions.filesButton": {
    message: "Files",
    description: "Session header button that opens the session files",
  },
  "agentSessions.filesTitle": {
    message: "Session files",
    description: "Title of the session files sheet",
  },
  "agentSessions.filesDescription": {
    message: "Files you attached and files the agent shared in this session.",
    description: "Description under the session files sheet title",
  },
  "agentSessions.filesEmpty": {
    message:
      "No files yet. Attach files to a message, or ask the agent to share one.",
    description: "Empty state of the session files sheet",
  },
  "agentSessions.filesError": {
    message: "Could not load the session's files.",
    description: "Error state of the session files sheet",
  },
  "agentSessions.filesColumnName": {
    message: "Name",
    description: "Files table column: file name",
  },
  "agentSessions.filesColumnFrom": {
    message: "From",
    description: "Files table column: who added the file",
  },
  "agentSessions.filesColumnTurn": {
    message: "Turn",
    description: "Files table column: turn number",
  },
  "agentSessions.filesColumnSize": {
    message: "Size",
    description: "Files table column: file size",
  },
  "agentSessions.filesFromInput": {
    message: "You",
    description: "Files table: a file the user attached",
  },
  "agentSessions.filesFromOutput": {
    message: "Agent",
    description: "Files table: a file the agent shared",
  },
  "agentSessions.filesLoadMore": {
    message: "Load more",
    description: "Files table: load the next page",
  },
  "agentSessions.attachmentsBlockSend": {
    message: "Remove the files that failed to upload to send",
    description: "Hint when failed attachment chips block sending",
  },
  "agentSessions.attachmentOpen": {
    message: "Open {name}",
    description:
      "Accessible label of an attachment thumbnail that opens the lightbox",
  },
  "agentSessions.attachmentDownload": {
    message: "Download {name}",
    description: "Accessible label of an attachment download card",
  },
  "agentSessions.attachmentUnavailable": {
    message: "Unavailable",
    description:
      "Shown on an attachment whose download link could not be loaded",
  },
  "agentSessions.attachmentUploading": {
    message: "Uploading, {percent}%",
    description: "Screen-reader status of an attachment chip while uploading",
  },
  "agentSessions.attachmentReady": {
    message: "Uploaded",
    description:
      "Screen-reader status of an attachment chip once its upload landed",
  },
  "agentSessions.attachmentHttpError": {
    message: "Upload failed (HTTP {status})",
    description:
      "Attachment chip error when the server answered without a message",
  },
  "agentSessions.attachmentNetworkError": {
    message: "Upload failed: network error",
    description:
      "Attachment chip error when the upload could not reach the server",
  },
  "agentSessions.attachmentsUploading": {
    message: "Waiting for files to finish uploading…",
    description: "Composer hint while attachment uploads block sending",
  },
  "agentSessions.spendWarningTitle": {
    message: "Agent model spend is at {percent}% of this month's cap",
    description: "Composer warning at 80% of the spend cap (ADR091 D6)",
  },
  "agentSessions.spendWarningBody": {
    message: "{used} of {cap} used. New model requests stop at the cap.",
    description: "Composer warning body",
  },
  "agentSessions.spendWarningBodyCharge": {
    message:
      "{used} of {cap} used. At {cap} the card on file is charged and sessions keep running; new model requests stop only if the charge fails.",
    description:
      "Composer warning body when the cap is a new workspace's charge point (ADR091 D10.2a)",
  },
  "agentSessions.spendReachedTitle.cap": {
    message: "Monthly agent model spend cap reached",
    description: "Composer refusal title: the workspace's own cap",
  },
  "agentSessions.spendReachedTitle.newWorkspace": {
    message: "New-workspace spend limit reached",
    description: "Composer refusal title: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedTitle.locked": {
    message: "Agent model spend is on hold",
    description: "Composer refusal title: a payment hold",
  },
  "agentSessions.spendReachedBody.cap": {
    message:
      "This workspace has used {used} of its {cap} monthly cap, so new sessions can't start. A workspace admin can raise the cap, or it resets on the 1st (UTC).",
    description: "Composer refusal body: the workspace's own cap",
  },
  "agentSessions.spendReachedBody.capMax": {
    message:
      "This workspace has used {used} of its {cap} monthly cap, the most it can be set to, so new sessions can't start. The cap resets on the 1st (UTC); contact support if you need a higher limit.",
    description:
      "Composer refusal body: the workspace's own cap binds at its maximum, so no admin can raise it",
  },
  "agentSessions.spendReachedBody.newWorkspace": {
    message:
      "New workspaces can spend up to {cap} on the bex-provided model until their first successful payment. The limit rises after that, or resets on the 1st (UTC).",
    description: "Composer refusal body: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedBody.locked": {
    message:
      "A failed or disputed payment limits this workspace's agent model spend to {cap}. Settle the outstanding payment in Billing, then start a new session.",
    description: "Composer refusal body: a payment hold",
  },
  "agentSessions.spendIncreaseCap": {
    message: "Increase cap",
    description: "Admin action linking to Settings \u2192 Agents",
  },
  "agentSessions.spendGoToBilling": {
    message: "Go to Billing",
    description: "Link to the Billing page",
  },
  "agentSessions.spendBalanceDueTitle": {
    message: "A payment failed",
    description: "Composer refusal title: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendBalanceDueBody": {
    message:
      "Agent sessions on the bex-provided model are paused until the outstanding invoice is paid.",
    description: "Composer refusal body: BILLING_BALANCE_DUE",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT": {
    message:
      "New workspaces can run {limit} agent sessions on the bex-provided model at once until their first successful payment. Wait for a running session to finish or stop one, then try again.",
    description: "Mapped message for AGENT_SESSION_TRUST_LIMIT (ADR091 D10.1)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT_locked": {
    message:
      "After a failed or disputed payment, this workspace can run {limit} agent sessions on the bex-provided model at once. Settle the outstanding payment in Billing to lift the limit.",
    description:
      "Mapped message for AGENT_SESSION_TRUST_LIMIT while a payment hold applies",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_INVALID": {
    message: "The spend limit must be more than $0 and at most $100,000.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_INVALID (ADR094 D8)",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE": {
    message:
      "A spend limit only applies to sessions on the bex-provided model. This workspace uses its own model key, so clear the limit or switch the workspace to the bex model in Settings → Agents.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE (ADR094 D8)",
  },
  "agentSessions.spendLimitLabel": {
    message: "Spend limit (USD)",
    description:
      "Composer — label/aria-label of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitPlaceholder": {
    message: "Spend limit",
    description:
      "Composer — placeholder of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitHint": {
    message:
      "Optional. The session stops once its bex-model spend reaches this amount (more than $0, at most $100,000).",
    description: "Composer — help text under the spend limit input",
  },
  "agentSessions.spendLimitInvalid": {
    message:
      "Enter an amount more than $0 and at most $100,000, or leave it blank.",
    description: "Composer — inline error for an out-of-range spend limit",
  },
  "agentSessions.failureSessionSpendLimitReached": {
    message:
      "This session stopped because its model spend reached the session limit of {limit}. Start a new session with a higher limit to continue.",
    description:
      "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED when the session's own maxSpendUSD ceiling was hit",
  },
  "agentSessions.spendNote": {
    message: "Model spend {spent}",
    description:
      "Session page — the session's model spend so far (outcome.cost)",
  },
  "agentSessions.spendNoteWithLimit": {
    message: "Model spend {spent} of the {limit} session limit",
    description:
      "Session page — model spend so far beside the session's spend limit",
  },
  "agentSessions.costNote": {
    message: "Cost {total} · model {model}, compute {compute}",
    description:
      "Session page — the session's total cost so far (outcome.cost.totalUSD) with its model and sandbox compute parts",
  },
  "agentSessions.costNoteWithLimit": {
    message:
      "Cost {total} · model {model} of the {limit} session limit, compute {compute}",
    description:
      "Session page — total cost with its parts; the session spend limit caps the model part",
  },
  "agentSessions.resultTitle": {
    message: "Result",
    description:
      "Session page — heading over the turn's final structured result",
  },
  "agentSessions.resultDraftTitle": {
    message: "Draft result",
    description:
      "Session page — heading over the agent's not-yet-final structured result while the turn runs",
  },
  "agentSessions.resultDraftHint": {
    message: "Not final yet. The agent may still change it.",
    description: "Session page — note under a draft structured result",
  },
  "agentSessions.resultCopy": {
    message: "Copy JSON",
    description: "Session page — button copying the structured result",
  },
  "agentSessions.resultCopied": {
    message: "Copied",
    description: "Session page — toast after copying the structured result",
  },
  "agentSessions.failureSpendCapReached": {
    message:
      "This workspace reached its agent model spend limit for the month. An admin can raise the cap in Settings → Agents, or it resets on the 1st (UTC).",
    description: "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED",
  },
  "agentSessions.failureBalanceDue": {
    message:
      "A payment for this workspace failed. Pay the outstanding invoice in Billing, then start a new session.",
    description: "Failure callout for BILLING_BALANCE_DUE",
  },
  "agentSessions.costTotal": {
    message: "{amount} model cost",
    description: "Session header: the session's model cost (ADR091 D6)",
  },
  "agentSessions.costTotalAll": {
    message: "{amount} total cost",
    description:
      "Session header: the session's total cost so far, model plus sandbox compute (outcome.cost.totalUSD, w9/m178)",
  },
  "agentSessions.costBreakdown": {
    message: "Model {model} · compute {compute}",
    description:
      "Session header cost popover: the model and sandbox compute parts of the total",
  },
  "agentSessions.costByo": {
    message: "Billed by your provider",
    description: "Session header: a BYO-key session has no bex model cost",
  },
  "agentSessions.costTitle": {
    message: "Model cost by turn",
    description: "Heading of the per-turn cost popover",
  },
  "agentSessions.costTurn": {
    message: "Turn {turn}",
    description: "One turn's row in the cost popover",
  },
  "agentSessions.costSetup": {
    message: "Before the first turn",
    description: "Requests recorded before the first turn",
  },
  "agentSessions.costTurnTokens": {
    message: "{input} input · {output} output · {cache} cache tokens",
    description: "A turn's token counts",
  },
  "agentSessions.costPricing": {
    message: "Billed at the model provider's cost + 30%.",
    description: "Pricing statement in the cost popover",
  },
  "agentSessions.secretsButton": {
    message: "Secrets",
    description: "Composer toolbar: the Secrets button label/tooltip (ADR098)",
  },
  "agentSessions.secretsButtonCount_one": {
    message: "Secrets ({count} attached)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsButtonCount_other": {
    message: "Secrets ({count} attached)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsTitle": {
    message: "Secrets for this session",
    description: "Composer Secrets popover heading",
  },
  "agentSessions.secretsDescription": {
    message:
      "The agent reads each one as an environment variable. Values are never shown again and are redacted from the transcript.",
    description: "Composer Secrets popover description",
  },
  "agentSessions.secretsWorkspace": {
    message: "Workspace secrets",
    description:
      "Composer Secrets popover: workspace agent secrets list heading",
  },
  "agentSessions.secretsWorkspaceLoadFailed": {
    message: "Couldn't load the workspace's agent secrets.",
    description:
      "Composer secrets panel error when the workspace secret list fails to load",
  },
  "agentSessions.secretsWorkspaceEmpty": {
    message: "No workspace secrets yet.",
    description: "Composer Secrets popover: no workspace agent secrets",
  },
  "agentSessions.secretsManage": {
    message: "Manage in Settings",
    description: "Composer Secrets popover: link to Settings \u2192 Agents",
  },
  "agentSessions.secretsInline": {
    message: "Only for this session",
    description: "Composer Secrets popover: inline secrets heading",
  },
  "agentSessions.secretsRemove": {
    message: "Remove {name}",
    description:
      "Composer Secrets popover: remove an inline secret (accessible label)",
  },
  "agentSessions.secretsNamePlaceholder": {
    message: "NAME",
    description: "Composer Secrets popover: inline secret name placeholder",
  },
  "agentSessions.secretsNameLabel": {
    message: "Secret name",
    description: "Composer Secrets popover: inline secret name field label",
  },
  "agentSessions.secretsValuePlaceholder": {
    message: "Value",
    description: "Composer Secrets popover: inline secret value placeholder",
  },
  "agentSessions.secretsValueLabel": {
    message: "Secret value",
    description: "Composer Secrets popover: inline secret value field label",
  },
  "agentSessions.secretsAdd": {
    message: "Add",
    description: "Composer Secrets popover: add an inline secret",
  },
  "agentSessions.secretsHint": {
    message: "Deleted with the session.",
    description: "Composer Secrets popover: inline secret lifetime hint",
  },
  "agentSessions.secretsFull": {
    message: "A session takes at most 20 secrets.",
    description: "Composer Secrets popover: session secret limit reached",
  },
  "agentSessions.secretsDuplicate": {
    message: "This session already has a secret with that name.",
    description: "Composer Secrets popover: duplicate inline secret name",
  },
  "agentSessions.secretsScopeRepo": {
    message: "This repository",
    description:
      "Composer Secrets popover: hint beside a secret scoped to the chosen repository, included",
  },
  "agentSessions.secretsScopeOtherRepo": {
    message: "Only on {repo}",
    description:
      "Composer Secrets popover: hint beside a secret scoped to another repository, not available here",
  },
  "agentSessions.appliedSecretsTitle": {
    message: "Secrets in the latest turn",
    description:
      "Session header: heading of the applied-secrets popover (names and sources, never values)",
  },
  "agentSessions.appliedSecretSource.inline": {
    message: "this session",
    description:
      "Applied secret source: an inline secret given to this session only",
  },
  "agentSessions.appliedSecretSource.explicit": {
    message: "selected",
    description: "Applied secret source: a workspace secret the session picked",
  },
  "agentSessions.appliedSecretSource.repo": {
    message: "repository",
    description:
      "Applied secret source: a workspace secret scoped to the session's repository",
  },
  "agentSessions.appliedSecretSource.workspace": {
    message: "workspace",
    description:
      "Applied secret source: a workspace secret every session receives",
  },
  "agentSessions.metaSecrets_one": {
    message: "{count} secret",
    description:
      "Detail header \u2014 number of attached secrets (singular, ADR098)",
  },
  "agentSessions.metaSecrets_other": {
    message: "{count} secrets",
    description:
      "Detail header \u2014 number of attached secrets (plural, ADR098)",
  },
  "agentSessions.skillsButton": {
    message: "Skills",
    description: "Composer toolbar: the Skills button label/tooltip (ADR099)",
  },
  "agentSessions.skillsButtonCount_one": {
    message: "Skills ({count} chosen)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsButtonCount_other": {
    message: "Skills ({count} chosen)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsTitle": {
    message: "Skills for this turn",
    description: "Composer Skills popover heading",
  },
  "agentSessions.skillsDescription": {
    message:
      "The agent follows each chosen skill. Enabled skills it may also apply on its own.",
    description: "Composer Skills popover description",
  },
  "agentSessions.skillsLoadFailed": {
    message: "Couldn't load the workspace skills.",
    description: "Composer Skills popover: list failed",
  },
  "agentSessions.skillsEmpty": {
    message: "No skills in this workspace yet.",
    description: "Composer Skills popover: none exist",
  },
  "agentSessions.skillsManage": {
    message: "Manage skills",
    description: "Composer Skills popover link to Settings → Agents",
  },
  "agentSessions.skillsArgumentsLabel": {
    message: "Arguments for {name}",
    description: "Composer Skills popover: argument field label",
  },
  "agentSessions.skillsArgumentsPlaceholder": {
    message: "Arguments (optional)",
    description: "Composer Skills popover: argument field placeholder",
  },
  "agentSessions.skillsArgumentsInvalid": {
    message: "Arguments are one line, up to 4 KiB.",
    description: "Composer Skills popover: invalid arguments",
  },
  "agentSessions.skillsFull": {
    message: "A turn can use at most 5 skills.",
    description: "Composer Skills popover: limit reached",
  },
  "agentSessions.skillsHint": {
    message: "Arguments fill $ARGUMENTS in the skill.",
    description: "Composer Skills popover hint",
  },
  "agentSessions.skillsRemove": {
    message: "Remove {name}",
    description: "Accessible label of a chosen skill chip's remove button",
  },
  "agentSessions.turnSkills": {
    message: "Skills used in this turn",
    description: "Accessible label of a transcript turn's skill pills",
  },
  "agentSessions.turnOrigins": {
    message: "What steered this turn",
    description:
      "Accessible label of the lines above a transcript turn that PR feedback started (w9/m344)",
  },
  "agentSessions.turnOriginReview": {
    message: "Steered by a review from @{author}",
    description:
      "A turn started by a pull-request review; {author} is the reviewer's GitHub login. Links to the review",
  },
  "agentSessions.turnOriginReviewAnonymous": {
    message: "Steered by a PR review",
    description:
      "A turn started by a pull-request review whose author is unknown. Links to the review",
  },
  "agentSessions.turnOriginComment": {
    message: "Steered by a PR comment from @{author}",
    description:
      "A turn started by a pull-request comment; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginCommentAnonymous": {
    message: "Steered by a PR comment",
    description:
      "A turn started by a pull-request comment whose author is unknown. Links to the comment",
  },
  "agentSessions.turnOriginCiFailure": {
    message: "Steered by a failed CI run",
    description:
      "A turn started by a failed CI run on the pull request. Links to the failed checks",
  },
  "agentSessions.turnOriginsStart": {
    message: "What started this session",
    description:
      "Accessible label of the line above a session's first turn when a /bex comment on a pull request started it (w9/m348)",
  },
  "agentSessions.turnOriginStartComment": {
    message: "Started from @{author}'s comment",
    description:
      "The first turn of a session a /bex pull-request comment started; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginStartCommentAnonymous": {
    message: "Started from a PR comment",
    description:
      "The first turn of a session a /bex pull-request comment started, when the commenter is unknown. Links to the comment",
  },
  "agentSessions.lineage.childSessions": {
    message: "Child sessions",
    description:
      "Heading of the block listing sessions the agent spawned, in the parent session's transcript",
  },
  "agentSessions.lineage.childCardTitle": {
    message: "Created a session",
    description:
      "Small label above a child session's title on its card in the parent's transcript",
  },
  "agentSessions.lineage.agentsPanel": {
    message: "Agents",
    description:
      "Heading of the session page's right-hand panel listing the sessions below this one",
  },
  "agentSessions.lineage.viewSession": {
    message: "View session",
    description: "Link on an Agents panel row that opens that child session",
  },
  "agentSessions.lineage.parentSession": {
    message: "Parent session",
    description:
      "Link in the Agents panel of a child session that opens its parent",
  },
  "agentSessions.lineage.collapsePanel": {
    message: "Collapse the Agents panel",
    description:
      "Accessible label of the button that collapses the Agents panel",
  },
  "agentSessions.lineage.expandPanel": {
    message: "Expand the Agents panel",
    description: "Accessible label of the button that expands the Agents panel",
  },
  "agentSessions.lineage.collapseChildren": {
    message: "Hide child sessions",
    description:
      "Accessible label of a sidebar row's button that collapses its nested child sessions",
  },
  "agentSessions.lineage.expandChildren": {
    message: "Show child sessions",
    description:
      "Accessible label of a sidebar row's button that expands its nested child sessions",
  },
  "agentSessions.lineage.status.working": {
    message: "Working",
    description:
      "Status group and badge of a child session that is still running",
  },
  "agentSessions.lineage.status.done": {
    message: "Done",
    description:
      "Status group and badge of a child session that finished successfully",
  },
  "agentSessions.lineage.status.failed": {
    message: "Failed",
    description: "Status group and badge of a child session that failed",
  },
  "agentSessions.lineage.status.canceled": {
    message: "Canceled",
    description: "Status group and badge of a child session that was canceled",
  },
  "agentSessions.sidebarMine": {
    message: "Mine",
    description:
      "Sessions sidebar — show only the sessions you started (Devin's default)",
  },
  "agentSessions.sidebarEveryone": {
    message: "Everyone",
    description: "Sessions sidebar — show every member's sessions",
  },
  "agentSessions.sidebarOwnerFilter": {
    message: "Whose sessions",
    description:
      "Sessions sidebar — accessible name of the Mine/Everyone toggle",
  },
  "agentSessions.sidebarEmptyMine": {
    message: "No sessions started by you. Older sessions are under Everyone.",
    description:
      "Sessions sidebar — empty state under Mine. Sessions from before 2026-11-08 record no starter, so they appear only under Everyone",
  },
  "agentSessions.startedByPerson": {
    message: "Started by {name}",
    description: "Session header and list rows — who started the session",
  },
  "agentSessions.startedForPerson": {
    message: "for {name}",
    description:
      "Session header — after 'Started by automation X': the member the automation runs as",
  },
  "agentSessions.editTags": {
    message: "Edit tags",
    description: "Session header menu — open the tag editor",
  },
  "agentSessions.editTagsTitle": {
    message: "Edit tags",
    description: "Tag editor dialog title",
  },
  "agentSessions.editTagsDescription": {
    message:
      "Tags replace the session's current tags. Separate them with commas or spaces; leave empty to clear.",
    description: "Tag editor dialog description",
  },
  "agentSessions.editTagsHint": {
    message:
      "Up to 20 tags: lowercase letters, digits, colon, underscore or hyphen.",
    description: "Tag editor dialog — the tag rules",
  },
  "agentSessions.editTagsSave": {
    message: "Save tags",
    description: "Tag editor dialog — save button",
  },
  "agentSessions.editTagsSaved": {
    message: "Tags saved",
    description: "Toast after the tags were replaced",
  },
};

export default enAgentSessions;
