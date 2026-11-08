// How an automation's run outcomes and pauses are shown, and which actions a
// viewer may take (ADR101 D4, D9). Pure so the list, the detail page and the
// tests share one decision.

export type OutcomeBadgeVariant =
  "success" | "secondary" | "outline" | "destructive";

export interface OutcomeDisplay {
  labelKey: string;
  variant: OutcomeBadgeVariant;
}

/** ADR101 D3/D7: claimed, started, skipped_overlap, failed_create, denied. */
const OUTCOMES: Record<string, OutcomeDisplay> = {
  started: { labelKey: "agentAutomations.outcome.started", variant: "success" },
  claimed: {
    labelKey: "agentAutomations.outcome.claimed",
    variant: "secondary",
  },
  skipped_overlap: {
    labelKey: "agentAutomations.outcome.skipped_overlap",
    variant: "outline",
  },
  failed_create: {
    labelKey: "agentAutomations.outcome.failed_create",
    variant: "destructive",
  },
  denied: {
    labelKey: "agentAutomations.outcome.denied",
    variant: "destructive",
  },
};

/** An outcome the dashboard does not know renders as itself, never as a blank. */
export function outcomeDisplay(outcome: string): OutcomeDisplay & {
  known: boolean;
} {
  const known = OUTCOMES[outcome];
  return known
    ? { ...known, known: true }
    : {
        labelKey: "agentAutomations.outcome.unknown",
        variant: "outline",
        known: false,
      };
}

/** The platform's pause reasons (ADR101 D3, D4): badge text and detail explanation. */
const PAUSED_REASONS = {
  repeated_failures: {
    badge: "agentAutomations.paused.repeated_failures",
    explain: "agentAutomations.pausedExplain.repeated_failures",
  },
  creator_lost_access: {
    badge: "agentAutomations.paused.creator_lost_access",
    explain: "agentAutomations.pausedExplain.creator_lost_access",
  },
  template_invalid: {
    badge: "agentAutomations.paused.template_invalid",
    explain: "agentAutomations.pausedExplain.template_invalid",
  },
} as const;

const UNKNOWN_PAUSE = {
  badge: "agentAutomations.paused.unknown",
  explain: "agentAutomations.pausedExplain.unknown",
} as const;

function pauseCopy(reason: string | null | undefined) {
  if (!reason) return null;
  return (
    (
      PAUSED_REASONS as Record<
        string,
        (typeof PAUSED_REASONS)[keyof typeof PAUSED_REASONS]
      >
    )[reason] ?? UNKNOWN_PAUSE
  );
}

/** The locale key for a pause reason's badge; a manual pause has none. */
export function pausedReasonKey(
  reason: string | null | undefined,
): string | null {
  return pauseCopy(reason)?.badge ?? null;
}

/** The sentence explaining a platform pause on the detail page. */
export function pausedExplainKey(
  reason: string | null | undefined,
): string | null {
  return pauseCopy(reason)?.explain ?? null;
}

export type AutomationStatus = "active" | "paused" | "ended";

/** Active, paused (by a person or by the platform), or finished with no next fire. */
export function automationStatus(a: {
  enabled: boolean;
  pausedReason?: string | null;
  nextFireAt?: string | null;
}): AutomationStatus {
  if (!a.enabled || a.pausedReason) return "paused";
  return a.nextFireAt ? "active" : "ended";
}

export interface ViewerRole {
  /** Fail-closed: true only on an affirmative, fresh capability answer. */
  canCreate: boolean;
  canManage: boolean;
  /** True when the viewer's workspace identity is the automation's creator. */
  isCreator: boolean;
}

export interface AutomationActions {
  /** Only the creator changes what runs. */
  edit: boolean;
  /** Pause, resume and Run now: the creator or an admin. */
  operate: boolean;
  /** Delete: the creator or an admin. */
  remove: boolean;
  /** Only an admin takes an automation over, and not their own. */
  takeOver: boolean;
}

/** Nothing allowed: the pending state, before the viewer's role is known. */
export const NO_ACTIONS: AutomationActions = {
  edit: false,
  operate: false,
  remove: false,
  takeOver: false,
};

export function automationActions(viewer: ViewerRole): AutomationActions {
  const creator = viewer.isCreator && viewer.canCreate;
  const operate = creator || viewer.canManage;
  return {
    edit: creator,
    operate,
    remove: operate,
    takeOver: viewer.canManage && !viewer.isCreator,
  };
}

/** Whether the pause/resume control currently pauses or resumes. */
export function pauseToggleKind(a: {
  enabled: boolean;
  pausedReason?: string | null;
}): "pause" | "resume" {
  return a.enabled && !a.pausedReason ? "pause" : "resume";
}
