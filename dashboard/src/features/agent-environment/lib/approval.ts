// Approval mode (ADR100 D3): when cloud agents pause to ask before acting.
// Shared by Settings → Agents (the workspace default) and the new-session
// composer (a per-session choice the workspace may limit).

/** The modes, least to most cautious. */
export const APPROVAL_MODES = ["autonomous", "risky", "all"] as const;

export type ApprovalMode = (typeof APPROVAL_MODES)[number];

/** Copy keys for each mode's option title and one-line description. */
export const APPROVAL_MODE_COPY: Record<
  ApprovalMode,
  { titleKey: string; descriptionKey: string }
> = {
  autonomous: {
    titleKey: "agentEnvironment.approvalAutonomousTitle",
    descriptionKey: "agentEnvironment.approvalAutonomousDescription",
  },
  risky: {
    titleKey: "agentEnvironment.approvalRiskyTitle",
    descriptionKey: "agentEnvironment.approvalRiskyDescription",
  },
  all: {
    titleKey: "agentEnvironment.approvalAllTitle",
    descriptionKey: "agentEnvironment.approvalAllDescription",
  },
};

/** A wire value as a mode; anything unknown reads as the default, autonomous. */
export function parseApprovalMode(
  value: string | null | undefined,
): ApprovalMode {
  return APPROVAL_MODES.find((mode) => mode === value) ?? "autonomous";
}

function rank(mode: ApprovalMode): number {
  return APPROVAL_MODES.indexOf(mode);
}

/**
 * Whether a session may run in `mode` under the workspace's setting: raising
 * the mode is always allowed; lowering it (or bypassing approval) only when
 * the workspace allows it. The server enforces the same rule and refuses the
 * rest with AGENT_SESSION_APPROVAL_BYPASS_FORBIDDEN.
 */
export function sessionModeAllowed(
  mode: ApprovalMode,
  workspaceMode: ApprovalMode,
  bypassAllowed: boolean,
): boolean {
  return bypassAllowed || rank(mode) >= rank(workspaceMode);
}

/**
 * The create fields a per-session choice implies. No choice (null) or the
 * workspace's own mode sends nothing, so the session follows the workspace.
 * Choosing autonomous under a stricter workspace is Devin's bypass_approval.
 */
export function sessionApprovalInput(
  choice: ApprovalMode | null,
  workspaceMode: ApprovalMode,
): { approvalMode?: ApprovalMode; bypassApproval?: boolean } {
  if (choice == null || choice === workspaceMode) return {};
  if (choice === "autonomous") return { bypassApproval: true };
  return { approvalMode: choice };
}
