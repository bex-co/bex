// Fail-closed normalizer for per-resource action projections (serverActions /
// deployActions / databaseActions / keyValueActions / envGroupActions). Unknown action ids,
// outcomes, or blocking codes can never enable an operation, and a decision
// fetched for one resource (or workspace) answers nothing about another
// (ADR087 / w6/m143).

import { snapshotIsFresh } from "./capability-policy";

export const RESOURCE_ACTION_IDS = [
  "restart",
  "suspend",
  "resume",
  "deploy",
  "cancel_deploy",
  "rollback",
  "cron_run_now",
  "cron_cancel_run",
  // The verbs a protected environment keeps to Admins (w4/m594): a
  // datastore's connection-info reveal and IP rules, an environment group's
  // values, their changes and its delete. A service, a Postgres and a Key
  // Value report delete too (w4/724).
  "view_connection_info",
  "update_ip_allow_list",
  "view_env_vars",
  "update_env_vars",
  "delete",
  // Opening a service's shell (serverActions, w4/726) and turning a web
  // service's maintenance mode on or off (w4/725), Admin-only in a protected
  // environment too.
  "shell",
  "toggle_maintenance_mode",
] as const;

export type ResourceActionId = (typeof RESOURCE_ACTION_IDS)[number];

export const RESOURCE_PRECONDITIONS = [
  "protected_confirmation_required",
  "suspended",
  // The mirror of "suspended": resume has nothing to do on a running resource.
  // Distinct from "suspended" on purpose — rendering "this is suspended" as the
  // reason Resume is unavailable would be exactly backwards (w4/132).
  "not_suspended",
  "no_active_deploy",
  "no_active_run",
  "no_eligible_rollback_target",
  "billing_blocked",
  // The free plan expired: the resource stays suspended until upgraded.
  "expired",
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
  // A restricted verb (suspend, resume, shell, the maintenance toggle,
  // delete; w4/m594's rows) on a member of a protected environment, refused because
  // the caller is not a workspace Admin (w4/m578): Render's "This action is
  // limited to admins."
  "protected_environment_admin_required",
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
  now = Date.now(),
): ResourceActionDecision | null {
  if (!snapshot || workspaceId === null) return null;
  if (snapshot.workspaceId !== workspaceId) return null;
  if (snapshot.resourceId !== resourceId) return null;
  if (!snapshotIsFresh(snapshot, now)) return null;
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
  decision: Pick<
    ResourceActionDecision,
    "outcome" | "reason" | "precondition"
  > | null,
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
      reasonKey:
        decision.reason === "protected_environment_admin_required"
          ? "capabilities.actionAdminOnly"
          : "capabilities.actionDenied",
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
    case "not_suspended":
      return "capabilities.blockedNotSuspended";
    case "no_active_deploy":
      return "capabilities.blockedNoActiveDeploy";
    case "no_active_run":
      return "capabilities.blockedNoActiveRun";
    case "no_eligible_rollback_target":
      return "capabilities.blockedNoEligibleRollbackTarget";
    case "billing_blocked":
      return "capabilities.blockedBillingBlocked";
    case "expired":
      return "capabilities.blockedExpired";
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
 * Why a resource's action projection refuses `action` right now, as the
 * reason a disabled control shows, or undefined when it may run: the
 * projection still checking, unavailable, the decision denied, or a blocking
 * precondition.
 */
export function projectionReason(
  projection:
    | { status: "checking" | "unavailable" }
    | { status: "ready"; snapshot: ResourceActionSnapshot },
  workspaceId: string | null,
  resourceId: string,
  action: ResourceActionId,
  t: (key: string) => string,
): string | undefined {
  return gateReason(
    gateAction(
      projection.status === "ready"
        ? resourceDecision(projection.snapshot, workspaceId, resourceId, action)
        : null,
      projection.status,
    ),
    t,
  );
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
