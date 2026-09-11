import { describe, expect, it } from "vitest";
import {
  decisionForSelectedRollback,
  decisionReady,
  gateAction,
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
    expect(gateAction({ outcome: "denied", precondition: "" }, "ready")).toEqual(
      {
        kind: "denied",
        reasonKey: "capabilities.actionDenied",
      },
    );
    expect(gateAction(null, "unavailable")).toEqual({
      kind: "unavailable",
      reasonKey: "capabilities.actionUnavailable",
    });
    expect(
      gateAction(
        { outcome: "allowed", precondition: "billing_blocked" },
        "ready",
      ),
    ).toMatchObject({ kind: "blocked", precondition: "billing_blocked" });
    expect(
      gateAction(
        { outcome: "allowed", precondition: "protected_confirmation_required" },
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
