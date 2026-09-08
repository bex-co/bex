// ADR087 (w6/m141): the fail-closed normalizer for the per-resource action
// projections (serverActions / deployActions / databaseActions /
// keyValueActions). Same contract as capability-policy.ts: unknown action ids,
// outcomes, or blocking codes can never enable an operation, and a decision
// fetched for one resource (or workspace) answers nothing about another.
//
// Projection ids, not new verbs: each maps onto an existing execution path
// whose own guards still run at dispatch. A snapshot is never a Bearer [REDACTED]

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

// The bounded precondition vocabulary (core decision.go). Empty means ready.
export const RESOURCE_PRECONDITIONS = [
  "protected_confirmation_required",
  "suspended",
  "no_active_deploy",
  "no_active_run",
  "no_eligible_rollback_target",
  "billing_blocked",
  "unavailable",
] as const;

export type ResourcePrecondition = (typeof RESOURCE_PRECONDITIONS)[number] | "";

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

// toResourceSnapshot normalizes one projection response. Unknown action ids
// are dropped (the app never gates on them); an unrecognized outcome reads as
// "unavailable"; an unrecognized non-empty precondition on a permitted action
// reads as "unavailable" — blocked, never waived. A not-allowed decision
// carries no precondition: readiness is meaningless (and mildly disclosive)
// for an action the caller may not perform.
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

// resourceDecision is the ONLY read: a decision for this exact
// workspace+resource+action, or null. A snapshot from another workspace, for
// another resource, or without this action answers nothing — fail closed.
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

// decisionReady is the single affirmative: permitted AND nothing blocking.
export function decisionReady(
  decision: ResourceActionDecision | null,
): boolean {
  return (
    decision !== null &&
    decision.outcome === "allowed" &&
    decision.precondition === ""
  );
}

// isExecutable binds the read and the readiness check: only a decision for
// this exact workspace+resource+action that is allowed with no precondition
// executes. Everything else — null snapshot, foreign workspace/resource,
// absent or unknown action, denied, unavailable, blocked — is false.
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

// blockedPrecondition reports the blocking precondition for a bound ALLOWED
// decision, or "" when the action is ready, refused, absent, or bound to
// another workspace/resource. Denied actions are absent, never "blocked", so
// they report "" here — readiness is meaningless for them.
export function blockedPrecondition(
  snapshot: ResourceActionSnapshot | null,
  workspaceId: string | null,
  resourceId: string | null,
  action: string,
): "" | (typeof RESOURCE_PRECONDITIONS)[number] {
  if (snapshot === null || workspaceId === null || resourceId === null) {
    return "";
  }
  if (!KNOWN_ACTIONS.has(action)) return "";
  const decision = resourceDecision(
    snapshot,
    workspaceId,
    resourceId,
    action as ResourceActionId,
  );
  return decision !== null && decision.outcome === "allowed"
    ? decision.precondition
    : "";
}

// decisionDenied reports an affirmative refusal — the action must be absent,
// not explained as temporarily blocked.
export function decisionDenied(
  decision: ResourceActionDecision | null,
): boolean {
  return decision !== null && decision.outcome === "denied";
}

// ActionPresentation is the shared consumer semantics every action surface
// uses: denied, unavailable, and unbound decisions are HIDDEN (absent, never
// disabled-with-reason); a ready decision is enabled; a permitted-but-blocked
// decision stays visible with its reason. The protected-environment
// precondition presents as ready — the safe-action dialog carries the server
// phrase as the explicit second confirmation step, so the extra confirmation
// is a step, not a block.
export type ActionPresentation =
  | { kind: "hidden" }
  | { kind: "ready" }
  | {
      kind: "blocked";
      precondition: Exclude<
        (typeof RESOURCE_PRECONDITIONS)[number],
        "protected_confirmation_required"
      >;
    };

export function presentAction(
  decision: Pick<ResourceActionDecision, "outcome" | "precondition"> | null,
): ActionPresentation {
  if (decision === null || decision.outcome !== "allowed") {
    return { kind: "hidden" };
  }
  if (
    decision.precondition === "" ||
    decision.precondition === "protected_confirmation_required"
  ) {
    return { kind: "ready" };
  }
  return { kind: "blocked", precondition: decision.precondition };
}

// blockedReasonKey maps a blocking precondition to its bilingual copy key.
// Unknown codes can never reach here (normalized to "unavailable"), so the
// default is unreachable-but-closed rather than load-bearing.
export function blockedReasonKey(
  precondition: ResourceActionDecision["precondition"],
): string {
  switch (precondition) {
    case "protected_confirmation_required":
      return "resourceActions.blocked.protectedConfirmation";
    case "suspended":
      return "resourceActions.blocked.suspended";
    case "no_active_deploy":
      return "resourceActions.blocked.noActiveDeploy";
    case "no_active_run":
      return "resourceActions.blocked.noActiveRun";
    case "no_eligible_rollback_target":
      return "resourceActions.blocked.noEligibleRollbackTarget";
    case "billing_blocked":
      return "resourceActions.blocked.billingBlocked";
    case "unavailable":
      return "resourceActions.blocked.unavailable";
    case "":
      return "resourceActions.blocked.generic";
    default:
      return "resourceActions.blocked.generic";
  }
}
