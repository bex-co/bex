import type {
  DeployActionError,
  DeployActionRequest,
} from "./deploy-action-types";

const cancelableStatuses = new Set([
  "created",
  "queued",
  "build_in_progress",
  "pre_deploy_in_progress",
  "update_in_progress",
]);

const rollbackableStatuses = new Set(["live", "deactivated"]);
const serviceIDPattern = /^srv-[a-z0-9]+$/;
const deployIDPattern = /^dep-[a-z0-9-]+$/;
const requestIDPattern = /^[A-Za-z0-9._~-]{1,128}$/;

export function isCancelableDeployStatus(status: string): boolean {
  return cancelableStatuses.has(status);
}

export function isRollbackableDeployStatus(status: string): boolean {
  return rollbackableStatuses.has(status);
}

export type DeployActionEligibility =
  { allowed: true } | { allowed: false; error: DeployActionError };

export function deployActionEligibility(
  request: DeployActionRequest,
): DeployActionEligibility {
  if (!requestIDPattern.test(request.requestId)) {
    return denied("The deploy confirmation identifier is invalid.");
  }
  if (!serviceIDPattern.test(request.serviceId)) {
    return denied("The service identifier is invalid.");
  }
  if (
    request.action !== "trigger" &&
    !deployIDPattern.test(request.target.id)
  ) {
    return denied("The deploy identifier is invalid.");
  }
  if (
    (request.action === "trigger" || request.action === "rollback") &&
    request.serviceSuspended
  ) {
    return denied("The service is suspended.", "conflict");
  }
  if (
    request.action === "cancel" &&
    !isCancelableDeployStatus(request.target.status)
  ) {
    return denied(
      `Deploy ${request.target.id} is already ${request.target.status || "terminal"}.`,
      "conflict",
    );
  }
  if (
    request.action === "rollback" &&
    !isRollbackableDeployStatus(request.target.status)
  ) {
    return denied(
      `Deploy ${request.target.id} never reached a rollbackable state.`,
      "conflict",
    );
  }
  return { allowed: true };
}

function denied(
  message: string,
  code: DeployActionError["code"] = "invalid_request",
): DeployActionEligibility {
  return {
    allowed: false,
    error: {
      code,
      message,
      delivery: "not_sent",
      refreshRequired: code === "conflict",
      retry: code === "conflict" ? "after_refresh" : "none",
    },
  };
}
