// Fail-closed normalizer for per-resource action projections (serverActions /
// deployActions). Unknown action ids, outcomes, or blocking codes can never
// enable an operation, and a decision fetched for one resource (or workspace)
// answers nothing about another (ADR087 / w6/m143).

export const RESOURCE_ACTION_IDS = [
  "restart",
  "suspend",
  "resume",
  "deploy",
  "cancel_deploy",
  "rollback",
  "cron_run_now",
  "cron_cancel_run",
] as const;

export type ResourceActionId = (typeof RESOURCE_ACTION_IDS)[number];

export const RESOURCE_PRECONDITIONS = [
  "protected_confirmation_required",
  "suspended",
  "no_active_deploy",
  "no_active_run",
  "no_eligible_rollback_target",
  "billing_blocked",
  "unavailable",
] as const;

export type ResourcePrecondition =
  | (typeof RESOURCE_PRECONDITIONS)[number]
  | "";

const OUTCOMES = ["allowed", "denied", "unavailable"] as const;
export type ResourceOutcome = (typeof OUTCOMES)[number];

export type ResourceActionInput = {
  action: string;
  outcome: string;
  reason: string | null;
  precondition: string | null;
};

export type ResourceActionDecision = {
  action: ResourceActionId;
  outcome: ResourceOutcome;
  /** Empty when allowed. Unknown server reasons arrive as generic. */
  reason: string;
  /** Empty means ready. Unknown server codes arrive as "unavailable". */
  precondition: "" | (typeof RESOURCE_PRECONDITIONS)[number];
};

export type ResourceActionSnapshot = {
  workspaceId: string;
  resourceId: string;
  receivedAt: number;
  decisions: Partial<Record<ResourceActionId, ResourceActionDecision>>;
};

const KNOWN_ACTIONS = new Set<string>(RESOURCE_ACTION_IDS);
const KNOWN_PRECONDITIONS = new Set<string>(RESOURCE_PRECONDITIONS);
const KNOWN_REASONS = new Set([
  "missing_oauth_scope",
  "insufficient_permission",
  "authz_unavailable",
]);

/** Normalize one projection response. Unknown ids drop; unknown outcomes /
 *  preconditions fail closed as unavailable — never waive. */
export function toResourceSnapshot(
  workspaceId: string,
  resourceId: string,
  rows: readonly ResourceActionInput[],
  receivedAt = Date.now(),
): ResourceActionSnapshot {
  const decisions: ResourceActionSnapshot["decisions"] = {};
  for (const row of rows) {
    if (!KNOWN_ACTIONS.has(row.action)) continue;
    const action = row.action as ResourceActionId;
    const outcome = (OUTCOMES as readonly string[]).includes(row.outcome)
      ? (row.outcome as ResourceOutcome)
      : "unavailable";
    const rawReason = row.reason?.trim() ?? "";
    const reason =
      outcome === "allowed"
        ? ""
        : KNOWN_REASONS.has(rawReason)
          ? rawReason
          : "authz_unavailable";
    let precondition: ResourceActionDecision["precondition"] = "";
    if (outcome === "allowed") {
      const raw = row.precondition?.trim() ?? "";
      if (raw === "") precondition = "";
      else if (KNOWN_PRECONDITIONS.has(raw)) {
        precondition = raw as (typeof RESOURCE_PRECONDITIONS)[number];
      } else {
        precondition = "unavailable";
      }
    }
    decisions[action] = { action, outcome, reason, precondition };
  }
  return { workspaceId, resourceId, receivedAt, decisions };
}

/** Bound read: decision for this exact workspace+resource+action, or null. */
export function resourceDecision(
  snapshot: ResourceActionSnapshot | null,
  workspaceId: string | null,
  resourceId: string,
  action: ResourceActionId,
): ResourceActionDecision | null {
  if (!snapshot || workspaceId === null) return null;
  if (snapshot.workspaceId !== workspaceId) return null;
  if (snapshot.resourceId !== resourceId) return null;
  return snapshot.decisions[action] ?? null;
}

export function decisionReady(
  decision: ResourceActionDecision | null,
): boolean {
  return (
    decision !== null &&
    decision.outcome === "allowed" &&
    decision.precondition === ""
  );
}

/** Permitted AND nothing blocking for this exact target. Fail closed. */
export function isExecutable(
  snapshot: ResourceActionSnapshot | null,
  workspaceId: string | null,
  resourceId: string | null,
  action: string,
): boolean {
  if (snapshot === null || workspaceId === null || resourceId === null) {
    return false;
  }
  if (!KNOWN_ACTIONS.has(action)) return false;
  return decisionReady(
    resourceDecision(
      snapshot,
      workspaceId,
      resourceId,
      action as ResourceActionId,
    ),
  );
}

export function decisionDenied(
  decision: ResourceActionDecision | null,
): boolean {
  return decision !== null && decision.outcome === "denied";
}

/**
 * Dashboard presentation: disable-with-reason (never hide). Denied /
 * unavailable / unbound stay visible with copy; ready enables; blocked keeps
 * the control with its precondition. protected_confirmation_required presents
 * as ready — the protected dialog is the second confirmation step.
 */
export type ActionGate =
  | { kind: "checking" }
  | { kind: "ready" }
  | { kind: "denied"; reasonKey: string }
  | { kind: "unavailable"; reasonKey: string }
  | {
      kind: "blocked";
      precondition: Exclude<
        (typeof RESOURCE_PRECONDITIONS)[number],
        "protected_confirmation_required"
      >;
      reasonKey: string;
    };

export function gateAction(
  decision: Pick<ResourceActionDecision, "outcome" | "precondition"> | null,
  status: "checking" | "unavailable" | "ready",
): ActionGate {
  if (status === "checking") return { kind: "checking" };
  if (status === "unavailable" || decision === null) {
    return {
      kind: "unavailable",
      reasonKey: "capabilities.actionUnavailable",
    };
  }
  if (decision.outcome === "denied") {
    return {
      kind: "denied",
      reasonKey: "capabilities.actionDenied",
    };
  }
  if (decision.outcome !== "allowed") {
    return {
      kind: "unavailable",
      reasonKey: "capabilities.actionUnavailable",
    };
  }
  if (
    decision.precondition === "" ||
    decision.precondition === "protected_confirmation_required"
  ) {
    return { kind: "ready" };
  }
  return {
    kind: "blocked",
    precondition: decision.precondition,
    reasonKey: blockedReasonKey(decision.precondition),
  };
}

/** Translation key for a blocking precondition. Unknown codes never reach
 *  here (normalized to unavailable). */
export function blockedReasonKey(
  precondition: ResourceActionDecision["precondition"],
): string {
  switch (precondition) {
    case "protected_confirmation_required":
      return "capabilities.blockedProtectedConfirmation";
    case "suspended":
      return "capabilities.blockedSuspended";
    case "no_active_deploy":
      return "capabilities.blockedNoActiveDeploy";
    case "no_active_run":
      return "capabilities.blockedNoActiveRun";
    case "no_eligible_rollback_target":
      return "capabilities.blockedNoEligibleRollbackTarget";
    case "billing_blocked":
      return "capabilities.blockedBillingBlocked";
    case "unavailable":
      return "capabilities.blockedUnavailable";
    case "":
      return "capabilities.blockedGeneric";
    default:
      return "capabilities.blockedGeneric";
  }
}

/** Reason string for a gate, or undefined when the control should enable. */
export function gateReason(
  gate: ActionGate,
  t: (key: string) => string,
): string | undefined {
  switch (gate.kind) {
    case "ready":
      return undefined;
    case "checking":
      return t("capabilities.actionChecking");
    case "denied":
    case "unavailable":
    case "blocked":
      return t(gate.reasonKey);
  }
}

/**
 * Service-wide `no_eligible_rollback_target` must not disable a selected
 * deploy that itself passes the execute-path eligibility predicate. Drop that
 * precondition when composing a selected-target rollback gate (m143 t003).
 */
export function decisionForSelectedRollback(
  decision: ResourceActionDecision | null,
): ResourceActionDecision | null {
  if (decision === null) return null;
  if (decision.precondition !== "no_eligible_rollback_target") return decision;
  return { ...decision, precondition: "" };
}
