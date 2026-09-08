import {
  allowsAction,
  checkingCapabilities,
  confirmedDenied,
  downgradeDetected,
  toSnapshot,
  unavailableCapabilities,
  type CapabilityState,
} from "../capability-policy";

const ready = (
  grants: Record<string, string>,
  workspaceId = "tea-a",
): CapabilityState => ({
  status: "ready",
  snapshot: toSnapshot(
    workspaceId,
    Object.entries(grants).map(([action, outcome]) => ({
      action,
      outcome,
      reason: null,
    })),
  ),
});

describe("capability policy (ADR087)", () => {
  it("allows only a confirmed allowed grant in the same workspace", () => {
    const state = ready({ can_operate: "allowed", can_create: "denied" });
    expect(allowsAction(state, "tea-a", "can_operate")).toBe(true);
    expect(allowsAction(state, "tea-a", "can_create")).toBe(false);
    // Another workspace's snapshot answers nothing about this one.
    expect(allowsAction(state, "tea-b", "can_operate")).toBe(false);
    expect(allowsAction(state, null, "can_operate")).toBe(false);
    // A missing grant is not an allowance.
    expect(allowsAction(state, "tea-a", "can_view_logs")).toBe(false);
  });

  it("fails closed while checking or unavailable — never permissive-while-unknown", () => {
    for (const state of [checkingCapabilities, unavailableCapabilities]) {
      expect(allowsAction(state, "tea-a", "can_view")).toBe(false);
      expect(allowsAction(state, "tea-a", "can_operate")).toBe(false);
      expect(confirmedDenied(state, "tea-a", "can_operate")).toBe(false);
    }
  });

  it("fails closed on unknown server vocabulary", () => {
    const state = ready({
      can_operate: "granted", // unrecognized outcome on a known action
      can_view: "allowed",
    });
    expect(allowsAction(state, "tea-a", "can_operate")).toBe(false);
    // Unknown outcome is NOT a confirmed denial either: it must present as
    // "couldn't check", never as "your role forbids this".
    expect(confirmedDenied(state, "tea-a", "can_operate")).toBe(false);
    // Unknown action ids are dropped entirely.
    const snapshot = toSnapshot("tea-a", [
      { action: "can_teleport", outcome: "allowed", reason: null },
    ]);
    expect(Object.keys(snapshot.grants)).toEqual([]);
  });

  it("distinguishes a confirmed denial from an unanswerable check", () => {
    const denied = ready({ can_operate: "denied" });
    const outage = ready({ can_operate: "unavailable" });
    expect(confirmedDenied(denied, "tea-a", "can_operate")).toBe(true);
    expect(confirmedDenied(outage, "tea-a", "can_operate")).toBe(false);
    expect(allowsAction(outage, "tea-a", "can_operate")).toBe(false);
  });

  it("detects a downgrade only between ready same-workspace snapshots", () => {
    const held = ready({ can_operate: "allowed" });
    const revoked = ready({ can_operate: "denied" });
    const outage = ready({ can_operate: "unavailable" });
    expect(downgradeDetected(held, revoked)).toBe(true);
    // Unavailable stops work but does not prove a permission change.
    expect(downgradeDetected(held, outage)).toBe(false);
    expect(downgradeDetected(held, ready({}))).toBe(false);
    // A transport failure produces no ready snapshot: never a role change.
    expect(downgradeDetected(held, unavailableCapabilities)).toBe(false);
    expect(downgradeDetected(held, checkingCapabilities)).toBe(false);
    // Cross-workspace comparisons are meaningless.
    expect(
      downgradeDetected(held, ready({ can_operate: "denied" }, "tea-b")),
    ).toBe(false);
    // An upgrade is not a downgrade.
    expect(downgradeDetected(revoked, held)).toBe(false);
  });

  it("expires affirmative access at 30 seconds without trusting clock rollback", () => {
    const state: CapabilityState = {
      status: "ready",
      snapshot: toSnapshot(
        "tea-a",
        [
          { action: "can_view", outcome: "allowed", reason: null },
          { action: "can_operate", outcome: "allowed", reason: null },
        ],
        10_000,
      ),
    };
    for (const action of ["can_view", "can_operate"] as const) {
      expect(allowsAction(state, "tea-a", action, 39_999)).toBe(true);
      expect(allowsAction(state, "tea-a", action, 40_000)).toBe(false);
      expect(allowsAction(state, "tea-a", action, 9_999)).toBe(false);
    }
  });
});
