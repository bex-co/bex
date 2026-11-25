import { describe, expect, it } from "vitest";
import { CAPABILITY_FRESHNESS_MS } from "../capability-policy";
import {
  blockedReasonKey,
  decisionForSelectedRollback,
  decisionReady,
  gateAction,
  projectionReason,
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

describe("resource-action policy (w6/m143/t001)", () => {
  it("refuses expired decisions and receipts from a future clock", () => {
    const state = snapshot([{ action: "deploy", outcome: "allowed" }]);
    expect(
      resourceDecision(
        state,
        "tea-a",
        "srv-one",
        "deploy",
        state.receivedAt + CAPABILITY_FRESHNESS_MS - 1,
      )?.outcome,
    ).toBe("allowed");
    expect(
      resourceDecision(
        state,
        "tea-a",
        "srv-one",
        "deploy",
        state.receivedAt + CAPABILITY_FRESHNESS_MS,
      ),
    ).toBeNull();
    expect(
      resourceDecision(
        state,
        "tea-a",
        "srv-one",
        "deploy",
        state.receivedAt - 1,
      ),
    ).toBeNull();
  });
  it("readies only an allowed decision with no precondition", () => {
    const state = snapshot([
      { action: "deploy", outcome: "allowed" },
      { action: "suspend", outcome: "allowed", precondition: "suspended" },
      { action: "resume", outcome: "denied" },
    ]);
    expect(decisionReady(decide(state, "deploy"))).toBe(true);
    expect(decisionReady(decide(state, "suspend"))).toBe(false);
    expect(decisionReady(decide(state, "resume"))).toBe(false);
    expect(decide(state, "rollback")).toBe(null);
  });

  it("never reuses a decision for another resource or workspace", () => {
    const state = snapshot([{ action: "deploy", outcome: "allowed" }]);
    expect(decide(state, "deploy", "tea-a", "srv-two")).toBe(null);
    expect(decide(state, "deploy", "tea-b", "srv-one")).toBe(null);
    expect(decide(state, "deploy", null, "srv-one")).toBe(null);
  });

  it("fails closed on unknown action ids, outcomes, and preconditions", () => {
    const state = snapshot([
      { action: "deploy", outcome: "granted" },
      { action: "teleport", outcome: "allowed" },
      {
        action: "suspend",
        outcome: "allowed",
        reason: "mystery",
        precondition: "morrow",
      },
    ]);
    expect(decide(state, "deploy")?.outcome).toBe("unavailable");
    expect(decisionReady(decide(state, "deploy"))).toBe(false);
    expect(Object.keys(state.decisions).sort()).toEqual(["deploy", "suspend"]);
    expect(decide(state, "suspend")?.precondition).toBe("unavailable");
  });

  it("presents denied and unavailable distinctly for disable-with-reason", () => {
    expect(
      gateAction(
        {
          outcome: "denied",
          reason: "insufficient_permission",
          precondition: "",
        },
        "ready",
      ),
    ).toEqual({
      kind: "denied",
      reasonKey: "capabilities.actionDenied",
    });
    expect(gateAction(null, "unavailable")).toEqual({
      kind: "unavailable",
      reasonKey: "capabilities.actionUnavailable",
    });
    expect(
      gateAction(
        { outcome: "allowed", reason: "", precondition: "billing_blocked" },
        "ready",
      ),
    ).toMatchObject({ kind: "blocked", precondition: "billing_blocked" });
    expect(
      gateAction(
        {
          outcome: "allowed",
          reason: "",
          precondition: "protected_confirmation_required",
        },
        "ready",
      ),
    ).toEqual({ kind: "ready" });
  });

  it("drops service-wide no_eligible_rollback_target for a selected deploy", () => {
    const decision = decide(
      snapshot([
        {
          action: "rollback",
          outcome: "allowed",
          precondition: "no_eligible_rollback_target",
        },
      ]),
      "rollback",
    );
    expect(decisionForSelectedRollback(decision)?.precondition).toBe("");
    expect(
      decisionForSelectedRollback(
        decide(
          snapshot([
            {
              action: "rollback",
              outcome: "allowed",
              precondition: "billing_blocked",
            },
          ]),
          "rollback",
        ),
      )?.precondition,
    ).toBe("billing_blocked");
  });
});

describe("not_suspended (w4/132)", () => {
  it("is a known precondition, not normalized to unavailable", () => {
    const snapshot = toResourceSnapshot("tea-1", "srv-1", [
      {
        action: "resume",
        outcome: "allowed",
        reason: null,
        precondition: "not_suspended",
      },
    ]);
    expect(snapshot.decisions.resume?.precondition).toBe("not_suspended");
  });

  it("keeps expired known: an expired resume blocks with its upgrade reason", () => {
    const state = toResourceSnapshot("tea-1", "dpg-1", [
      {
        action: "resume",
        outcome: "allowed",
        reason: null,
        precondition: "expired",
      },
    ]);
    expect(state.decisions.resume?.precondition).toBe("expired");
    expect(
      gateAction(
        { outcome: "allowed", reason: "", precondition: "expired" },
        "ready",
      ),
    ).toMatchObject({
      kind: "blocked",
      precondition: "expired",
      reasonKey: "capabilities.blockedExpired",
    });
    expect(blockedReasonKey("expired")).toBe("capabilities.blockedExpired");
  });

  it("keeps allowance preconditions known: a suspended resume blocks with its own reason", () => {
    for (const precondition of [
      "free_hours_exhausted",
      "bandwidth_exhausted",
    ] as const) {
      const state = toResourceSnapshot("tea-1", "srv-1", [
        { action: "resume", outcome: "allowed", reason: null, precondition },
      ]);
      expect(state.decisions.resume?.precondition).toBe(precondition);
      expect(
        gateAction({ outcome: "allowed", reason: "", precondition }, "ready"),
      ).toMatchObject({ kind: "blocked", precondition });
    }
    expect(blockedReasonKey("free_hours_exhausted")).toBe(
      "capabilities.blockedFreeHoursExhausted",
    );
    expect(blockedReasonKey("bandwidth_exhausted")).toBe(
      "capabilities.blockedBandwidthExhausted",
    );
  });

  it("blocks the control with its own reason, never the suspended one", () => {
    const gate = gateAction(
      { outcome: "allowed", reason: "", precondition: "not_suspended" },
      "ready",
    );
    expect(gate.kind).toBe("blocked");
    expect(gate).toMatchObject({
      reasonKey: "capabilities.blockedNotSuspended",
    });
    // The mirror term must not collapse onto "suspended": telling a user their
    // running resource is suspended is exactly backwards.
    expect(blockedReasonKey("not_suspended")).not.toBe(
      blockedReasonKey("suspended"),
    );
  });
});

// w3/m555: one reading of a projection for a disabled control's reason.
describe("projectionReason", () => {
  const t = (key: string) => key;
  const ready = (precondition: string | null, outcome = "allowed") => ({
    status: "ready" as const,
    snapshot: snapshot([{ action: "cron_run_now", outcome, precondition }]),
  });

  it("names no reason for a ready decision", () => {
    expect(
      projectionReason(ready(null), "tea-a", "srv-one", "cron_run_now", t),
    ).toBeUndefined();
  });

  it("names the projection's state, the denial or the precondition", () => {
    const reason = (
      projection: Parameters<typeof projectionReason>[0],
      resourceId = "srv-one",
    ) => projectionReason(projection, "tea-a", resourceId, "cron_run_now", t);
    expect(reason({ status: "checking" })).toBe("capabilities.actionChecking");
    expect(reason({ status: "unavailable" })).toBe(
      "capabilities.actionUnavailable",
    );
    expect(reason(ready(null, "denied"))).toBe("capabilities.actionDenied");
    expect(reason(ready("suspended"))).toBe(blockedReasonKey("suspended"));
    // Another resource's snapshot never answers for this one.
    expect(reason(ready(null), "srv-other")).toBe(
      "capabilities.actionUnavailable",
    );
  });
});

describe("protected_environment_admin_required (w4/m578)", () => {
  it("is a known reason, kept rather than read as generic", () => {
    const state = snapshot([
      {
        action: "suspend",
        outcome: "denied",
        reason: "protected_environment_admin_required",
      },
      { action: "resume", outcome: "denied", reason: "mystery" },
    ]);
    expect(decide(state, "suspend")?.reason).toBe(
      "protected_environment_admin_required",
    );
    expect(decide(state, "resume")?.reason).toBe("authz_unavailable");
  });

  it("reads as Render's admin-only copy, and a role denial as the role copy", () => {
    expect(
      gateAction(
        {
          outcome: "denied",
          reason: "protected_environment_admin_required",
          precondition: "",
        },
        "ready",
      ),
    ).toEqual({ kind: "denied", reasonKey: "capabilities.actionAdminOnly" });
    for (const reason of [
      "insufficient_permission",
      "missing_oauth_scope",
      "authz_unavailable",
    ]) {
      expect(
        gateAction({ outcome: "denied", reason, precondition: "" }, "ready"),
      ).toEqual({ kind: "denied", reasonKey: "capabilities.actionDenied" });
    }
  });
});
