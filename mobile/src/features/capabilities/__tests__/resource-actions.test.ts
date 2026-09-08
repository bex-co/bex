import {
  blockedReasonKey,
  decisionDenied,
  decisionReady,
  presentAction,
  resourceDecision,
  toResourceSnapshot,
  type ResourceActionId,
  type ResourceActionSnapshot,
} from "../resource-actions";

const snapshot = (
  decisions: {
    action: string;
    outcome: string;
    reason?: string | null;
    precondition?: string | null;
  }[],
  workspaceId = "tea-a",
  resourceId = "srv-one",
): ResourceActionSnapshot =>
  toResourceSnapshot(
    workspaceId,
    resourceId,
    decisions.map((decision) => ({
      action: decision.action,
      outcome: decision.outcome,
      reason: decision.reason ?? null,
      precondition: decision.precondition ?? null,
    })),
  );

const decide = (
  state: ResourceActionSnapshot,
  action: ResourceActionId,
  workspaceId: string | null = "tea-a",
  resourceId = "srv-one",
) => resourceDecision(state, workspaceId, resourceId, action);

describe("resource-action policy (w6/m141/t001)", () => {
  it("readies only an allowed decision with no precondition", () => {
    const state = snapshot([
      { action: "restart", outcome: "allowed" },
      { action: "suspend", outcome: "allowed", precondition: "suspended" },
      { action: "resume", outcome: "denied" },
    ]);
    expect(decisionReady(decide(state, "restart"))).toBe(true);
    expect(decisionReady(decide(state, "suspend"))).toBe(false);
    expect(decisionReady(decide(state, "resume"))).toBe(false);
    // Absent actions are never ready.
    expect(decide(state, "rollback")).toBe(null);
    expect(decisionReady(decide(state, "rollback"))).toBe(false);
  });

  it("never reuses a decision for another resource or workspace", () => {
    const state = snapshot([{ action: "restart", outcome: "allowed" }]);
    expect(decide(state, "restart", "tea-a", "srv-two")).toBe(null);
    expect(decide(state, "restart", "tea-b", "srv-one")).toBe(null);
    expect(decide(state, "restart", null, "srv-one")).toBe(null);
    expect(resourceDecision(null, "tea-a", "srv-one", "restart")).toBe(null);
  });

  it("fails closed on unknown action ids, outcomes, and preconditions", () => {
    const state = snapshot([
      { action: "restart", outcome: "granted" },
      { action: "teleport", outcome: "allowed" },
      {
        action: "suspend",
        outcome: "allowed",
        reason: "mystery",
        precondition: "morrow",
      },
    ]);
    // Unknown outcome reads as unavailable: blocked, never a denial either.
    expect(decide(state, "restart")?.outcome).toBe("unavailable");
    expect(decisionReady(decide(state, "restart"))).toBe(false);
    expect(decisionDenied(decide(state, "restart"))).toBe(false);
    // Unknown action ids are dropped entirely.
    expect(Object.keys(state.decisions).sort()).toEqual(["restart", "suspend"]);
    // Unknown precondition falls back to a block, never ready.
    expect(decide(state, "suspend")?.precondition).toBe("unavailable");
    expect(decisionReady(decide(state, "suspend"))).toBe(false);
  });

  it("distinguishes an affirmative denial from a block", () => {
    const state = snapshot([
      { action: "resume", outcome: "denied" },
      {
        action: "suspend",
        outcome: "allowed",
        precondition: "protected_confirmation_required",
      },
    ]);
    expect(decisionDenied(decide(state, "resume"))).toBe(true);
    expect(decisionDenied(decide(state, "suspend"))).toBe(false);
  });

  it("maps every blocking precondition to copy, defaulting closed", () => {
    for (const precondition of [
      "protected_confirmation_required",
      "suspended",
      "no_active_deploy",
      "no_active_run",
      "no_eligible_rollback_target",
      "billing_blocked",
      "unavailable",
      "",
    ] as const) {
      expect(blockedReasonKey(precondition)).toContain(
        "resourceActions.blocked.",
      );
    }
  });

  it("hides refused, unanswerable, and missing decisions", () => {
    expect(presentAction(null)).toEqual({ kind: "hidden" });
    const state = snapshot([
      { action: "resume", outcome: "denied" },
      { action: "restart", outcome: "unavailable" },
    ]);
    expect(presentAction(decide(state, "resume")).kind).toBe("hidden");
    expect(presentAction(decide(state, "restart")).kind).toBe("hidden");
    expect(presentAction(decide(state, "rollback")).kind).toBe("hidden");
  });

  it("readies allowed decisions, including protected confirmation", () => {
    const state = snapshot([
      { action: "restart", outcome: "allowed" },
      {
        action: "suspend",
        outcome: "allowed",
        precondition: "protected_confirmation_required",
      },
    ]);
    // The protected phrase is a confirmation step, not a block: the option
    // stays enabled for the server-phrase round trip.
    expect(presentAction(decide(state, "restart")).kind).toBe("ready");
    expect(presentAction(decide(state, "suspend")).kind).toBe("ready");
  });

  it("blocks every other precondition with its reason intact", () => {
    const preconditions = [
      "suspended",
      "no_active_deploy",
      "no_active_run",
      "no_eligible_rollback_target",
      "billing_blocked",
      "unavailable",
    ] as const;
    for (const precondition of preconditions) {
      const only = snapshot([
        { action: "restart", outcome: "allowed", precondition },
      ]);
      const presentation = presentAction(decide(only, "restart"));
      expect(presentation.kind).toBe("blocked");
      if (presentation.kind === "blocked") {
        expect(presentation.precondition).toBe(precondition);
        expect(blockedReasonKey(presentation.precondition)).toContain(
          "resourceActions.blocked.",
        );
      }
    }
  });
});
