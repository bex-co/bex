import {
  deployActionEligibility,
  isCancelableDeployStatus,
  isRollbackableDeployStatus,
} from "../deploy-action-eligibility";
import type { DeployServerGate } from "../deploy-action-types";

const ready: DeployServerGate = { outcome: "allowed", precondition: "" };

describe("deploy action eligibility", () => {
  it("keeps the status sets as target selectors for history rows", () => {
    for (const status of [
      "created",
      "queued",
      "build_in_progress",
      "pre_deploy_in_progress",
      "update_in_progress",
    ]) {
      expect(isCancelableDeployStatus(status)).toBe(true);
    }
    for (const status of [
      "build_failed",
      "canceled",
      "deactivated",
      "live",
      "pre_deploy_failed",
      "update_failed",
    ]) {
      expect(isCancelableDeployStatus(status)).toBe(false);
    }
    expect(isRollbackableDeployStatus("live")).toBe(true);
    expect(isRollbackableDeployStatus("deactivated")).toBe(true);
    expect(isRollbackableDeployStatus("update_failed")).toBe(false);
  });

  it("follows the server decision instead of client status sets", () => {
    // A ready decision allows the send even for a row the old client sets
    // would have rejected — target status no longer gates the verb.
    expect(
      deployActionEligibility({
        requestId: "confirm-cancel",
        action: "cancel",
        serviceId: "srv-one",
        server: ready,
        target: { id: "dep-one", status: "live" },
      }).allowed,
    ).toBe(true);
    expect(
      deployActionEligibility({
        requestId: "confirm-rollback",
        action: "rollback",
        serviceId: "srv-one",
        server: ready,
        target: { id: "dep-one", status: "build_failed" },
      }).allowed,
    ).toBe(true);
    expect(
      deployActionEligibility({
        requestId: "confirm-trigger",
        action: "trigger",
        serviceId: "srv-one",
        server: ready,
      }).allowed,
    ).toBe(true);
  });

  it("fails closed on denied, blocked, and missing decisions", () => {
    const trigger = (server: DeployServerGate) =>
      deployActionEligibility({
        requestId: "confirm-trigger",
        action: "trigger",
        serviceId: "srv-one",
        server,
      });
    expect(trigger(null).allowed).toBe(false);
    expect(trigger({ outcome: "unavailable", precondition: "" }).allowed).toBe(
      false,
    );
    const denied = trigger({ outcome: "denied", precondition: "" });
    expect(denied.allowed).toBe(false);
    if (!denied.allowed) {
      expect(denied.error.code).toBe("forbidden");
      expect(denied.error.refreshRequired).toBe(false);
    }
    for (const precondition of [
      "suspended",
      "no_active_deploy",
      "no_eligible_rollback_target",
      "billing_blocked",
      "unavailable",
    ] as const) {
      const blocked = trigger({ outcome: "allowed", precondition });
      expect(blocked.allowed).toBe(false);
      if (!blocked.allowed) {
        expect(blocked.error.code).toBe("conflict");
        expect(blocked.error.refreshRequired).toBe(true);
      }
    }
  });

  it("still rejects malformed identifiers without sending", () => {
    expect(
      deployActionEligibility({
        requestId: "confirm bad id",
        action: "trigger",
        serviceId: "srv-one",
        server: ready,
      }).allowed,
    ).toBe(false);
    expect(
      deployActionEligibility({
        requestId: "confirm-service",
        action: "trigger",
        serviceId: "not-a-service",
        server: ready,
      }).allowed,
    ).toBe(false);
    expect(
      deployActionEligibility({
        requestId: "confirm-bad-id",
        action: "rollback",
        serviceId: "srv-one",
        server: ready,
        target: { id: "../dep-one", status: "live" },
      }).allowed,
    ).toBe(false);
  });
});
