// Fail-closed capability policy (w6/m144). Unknown, loading, errored,
// unrecognized, or stale answers never authorize actions or sensitive reads.
// Role labels never substitute for a grant. Snapshots live in provider memory.

export const CAPABILITY_ACTIONS = [
  "can_view",
  "can_view_logs",
  "can_operate",
  "can_create",
  "can_view_sensitive",
  "can_manage_keys",
  "can_manage",
  "can_manage_billing",
] as const;

export type CapabilityAction = (typeof CAPABILITY_ACTIONS)[number];

const OUTCOMES = ["allowed", "denied", "unavailable"] as const;
export type CapabilityOutcome = (typeof OUTCOMES)[number];

export type CapabilityGrantInput = {
  action: string;
  outcome: string;
  reason: string | null;
};

export type CapabilitySnapshot = {
  workspaceId: string;
  role: string | null;
  /** Local receipt time, not a server membership version. Never persisted. */
  receivedAt: number;
  grants: Partial<Record<CapabilityAction, CapabilityOutcome>>;
};

/** Client freshness bound — matches RESOURCE_POLL_INTERVAL_MS. */
export const CAPABILITY_FRESHNESS_MS = 30_000;

export function snapshotIsFresh(
  snapshot: CapabilitySnapshot,
  now = Date.now(),
): boolean {
  return (
    now >= snapshot.receivedAt &&
    now - snapshot.receivedAt < CAPABILITY_FRESHNESS_MS
  );
}

export type CapabilityState =
  | { status: "checking" }
  | { status: "unavailable" }
  | { status: "ready"; snapshot: CapabilitySnapshot };

export const checkingCapabilities: CapabilityState = { status: "checking" };
export const unavailableCapabilities: CapabilityState = {
  status: "unavailable",
};

const BOOLEAN_TO_ACTION = {
  canView: "can_view",
  canViewLogs: "can_view_logs",
  canOperate: "can_operate",
  canCreate: "can_create",
  canViewSensitive: "can_view_sensitive",
  canManageKeys: "can_manage_keys",
  canManage: "can_manage",
  canManageBilling: "can_manage_billing",
} as const satisfies Record<string, CapabilityAction>;

export type CapabilityBoolean = keyof typeof BOOLEAN_TO_ACTION;

export function actionForBoolean(key: CapabilityBoolean): CapabilityAction {
  return BOOLEAN_TO_ACTION[key];
}

export const ROLE_REASON_KEYS: Record<CapabilityAction, string> = {
  can_view: "capabilities.reasonCanCreate", // unused for view-only gates today
  can_view_logs: "capabilities.reasonCanOperate",
  can_operate: "capabilities.reasonCanOperate",
  can_create: "capabilities.reasonCanCreate",
  can_view_sensitive: "capabilities.reasonCanViewSensitive",
  can_manage_keys: "capabilities.reasonCanCreate",
  can_manage: "capabilities.reasonCanManage",
  can_manage_billing: "capabilities.reasonCanManageBilling",
};

export function toSnapshot(
  workspaceId: string,
  grants: readonly CapabilityGrantInput[],
  role: string | null = null,
  receivedAt = Date.now(),
): CapabilitySnapshot {
  const known = new Set<string>(CAPABILITY_ACTIONS);
  const out: CapabilitySnapshot = {
    workspaceId,
    role,
    receivedAt,
    grants: {},
  };
  for (const grant of grants) {
    if (!known.has(grant.action)) continue;
    const outcome = (OUTCOMES as readonly string[]).includes(grant.outcome)
      ? (grant.outcome as CapabilityOutcome)
      : "unavailable";
    out.grants[grant.action as CapabilityAction] = outcome;
  }
  return out;
}

/** Affirmative only: ready + fresh + same workspace + outcome allowed. */
export function allowsAction(
  state: CapabilityState,
  workspaceId: string | null,
  action: CapabilityAction,
  now = Date.now(),
): boolean {
  return (
    state.status === "ready" &&
    snapshotIsFresh(state.snapshot, now) &&
    workspaceId !== null &&
    state.snapshot.workspaceId === workspaceId &&
    state.snapshot.grants[action] === "allowed"
  );
}

/** Confirmed role denial — never unavailable/stale/loading. */
export function confirmedDenied(
  state: CapabilityState,
  workspaceId: string | null,
  action: CapabilityAction,
): boolean {
  return (
    state.status === "ready" &&
    workspaceId !== null &&
    state.snapshot.workspaceId === workspaceId &&
    state.snapshot.grants[action] === "denied"
  );
}

export function downgradeDetected(
  previous: CapabilityState,
  next: CapabilityState,
): boolean {
  if (previous.status !== "ready" || next.status !== "ready") return false;
  if (previous.snapshot.workspaceId !== next.snapshot.workspaceId) return false;
  return CAPABILITY_ACTIONS.some(
    (action) =>
      previous.snapshot.grants[action] === "allowed" &&
      next.snapshot.grants[action] === "denied",
  );
}

/**
 * Tooltip / recovery key when an action is not allowed. Undefined while still
 * checking (disable without claiming a role change) or when allowed.
 */
export function grantReasonKey(
  state: CapabilityState,
  workspaceId: string | null,
  action: CapabilityAction,
  now = Date.now(),
): string | undefined {
  if (allowsAction(state, workspaceId, action, now)) return undefined;
  if (confirmedDenied(state, workspaceId, action)) {
    return ROLE_REASON_KEYS[action];
  }
  if (state.status === "unavailable") return "capabilities.grantUnavailable";
  if (
    state.status === "ready" &&
    workspaceId !== null &&
    state.snapshot.workspaceId === workspaceId &&
    !snapshotIsFresh(state.snapshot, now)
  ) {
    return "capabilities.grantStale";
  }
  if (
    state.status === "ready" &&
    state.snapshot.grants[action] === "unavailable"
  ) {
    return "capabilities.grantUnavailable";
  }
  return undefined;
}
