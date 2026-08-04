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
  // --- Detail placeholder (t004 replaces this route's body) ---
  "agentSessions.detailPlaceholderTitle": {
    message: "Session detail is coming soon",
    description:
      "Placeholder heading on the /agents/{id} route until t004 builds the detail page",
  },
  "agentSessions.detailPlaceholderBody": {
    message:
      "The live conversation, evidence, and steering controls for this session land in a follow-up. Session {id}.",
    description: "Placeholder body on the agent-session detail route",
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
};

export default enAgentSessions;
