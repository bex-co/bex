// ADR087 (w6/m138): the fail-closed capability policy. Every decision the
// app makes from the caller's workspace permissions goes through these pure
// functions so the rules are unit-testable and cannot fork per screen.
//
// Deliberately NOT the dashboard's permissive-while-unknown hook: unknown,
// loading, errored, or unrecognized server values all read as not-allowed.
// A role label never substitutes for a grant, and nothing here persists —
// snapshots live in provider memory only.

export const CAPABILITY_ACTIONS = [
  "can_view",
  "can_view_logs",
  "can_operate",
  "can_create",
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
  grants: Partial<Record<CapabilityAction, CapabilityOutcome>>;
};

export type CapabilityState =
  | { status: "checking" }
  | { status: "unavailable" }
  | { status: "ready"; snapshot: CapabilitySnapshot };

export const checkingCapabilities: CapabilityState = { status: "checking" };
export const unavailableCapabilities: CapabilityState = {
  status: "unavailable",
};

// toSnapshot normalizes the server's grants list. Unknown action ids are
// dropped (the app never gates on them); an unrecognized outcome value on a
// KNOWN action reads as "unavailable" — the server said something this build
// doesn't understand, which must gate like "couldn't check", never like a
// permission.
export function toSnapshot(
  workspaceId: string,
  grants: readonly CapabilityGrantInput[],
): CapabilitySnapshot {
  const known = new Set<string>(CAPABILITY_ACTIONS);
  const out: CapabilitySnapshot = { workspaceId, grants: {} };
  for (const grant of grants) {
    if (!known.has(grant.action)) continue;
    const outcome = (OUTCOMES as readonly string[]).includes(grant.outcome)
      ? (grant.outcome as CapabilityOutcome)
      : "unavailable";
    out.grants[grant.action as CapabilityAction] = outcome;
  }
  return out;
}

// allowsAction is the ONLY affirmative: a ready snapshot for this workspace
// whose grant is exactly "allowed". Checking, unavailable, a snapshot from
// another workspace, a missing grant, and every other outcome are false.
export function allowsAction(
  state: CapabilityState,
  workspaceId: string | null,
  action: CapabilityAction,
): boolean {
  return (
    state.status === "ready" &&
    workspaceId !== null &&
    state.snapshot.workspaceId === workspaceId &&
    state.snapshot.grants[action] === "allowed"
  );
}

// confirmedDenied distinguishes an affirmative refusal from an unanswerable
// check — the difference between "your access doesn't cover this" copy and
// the neutral "couldn't check" state. Only a ready same-workspace snapshot
// can confirm anything.
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

// downgradeDetected reports a CONFIRMED loss: an action the previous ready
// snapshot allowed that the next ready snapshot (same workspace) no longer
// does. A transport failure produces no ready snapshot and so can never
// masquerade as a role change (ADR087: unavailable ≠ demoted).
export function downgradeDetected(
  previous: CapabilityState,
  next: CapabilityState,
): boolean {
  if (previous.status !== "ready" || next.status !== "ready") return false;
  if (previous.snapshot.workspaceId !== next.snapshot.workspaceId) return false;
  return CAPABILITY_ACTIONS.some(
    (action) =>
      previous.snapshot.grants[action] === "allowed" &&
      next.snapshot.grants[action] !== "allowed",
  );
}
