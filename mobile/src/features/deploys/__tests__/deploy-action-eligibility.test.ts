import {
  deployActionEligibility,
  isCancelableDeployStatus,
  isRollbackableDeployStatus,
} from "../deploy-action-eligibility";

describe("deploy action eligibility", () => {
  it("matches the backend's exact cancel and rollback windows", () => {
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

  it("fails closed on suspended, terminal, unknown, and malformed targets", () => {
    expect(
      deployActionEligibility({
        requestId: "confirm-trigger",
        action: "trigger",
        serviceId: "srv-one",
        serviceSuspended: true,
      }).allowed,
    ).toBe(false);
    expect(
      deployActionEligibility({
        requestId: "confirm-cancel",
        action: "cancel",
        serviceId: "srv-one",
        serviceSuspended: false,
        target: { id: "dep-one", status: "live" },
      }).allowed,
    ).toBe(false);
    expect(
      deployActionEligibility({
        requestId: "confirm-rollback",
        action: "rollback",
        serviceId: "srv-one",
        serviceSuspended: false,
        target: { id: "dep-one", status: "build_failed" },
      }).allowed,
    ).toBe(false);
    expect(
      deployActionEligibility({
        requestId: "confirm-bad-id",
        action: "rollback",
        serviceId: "srv-one",
        serviceSuspended: false,
        target: { id: "../dep-one", status: "live" },
      }).allowed,
    ).toBe(false);
  });
});
