import type {
  DeployActionError,
  DeployActionRequest,
} from "./deploy-action-types";

// Target SELECTORS, not eligibility: the deployActions projection answers
// whether a cancelable/rollbackable row exists for the service, while deploy
// history names the concrete row to act on. These sets stay client-side only
// to pick that row — they never gate whether the verb may run.
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
  // Eligibility itself is the server's per-service decision, captured at
  // confirmation time: suspension, open-deploy, rollback-target, and billing
  // preconditions all live in the projection (and are rechecked by the verb
  // at dispatch). A missing or unanswerable decision fails closed.
  const server = request.server;
  if (!server) {
    return unavailable("Deploy eligibility is currently unavailable.");
  }
  if (server.outcome === "denied") {
    return {
      allowed: false,
      error: {
        code: "forbidden",
        message: "The deploy action is not permitted.",
        delivery: "not_sent",
        refreshRequired: false,
        retry: "none",
      },
    };
  }
  if (server.outcome !== "allowed") {
    return unavailable("Deploy eligibility is currently unavailable.");
  }
  if (server.precondition !== "") {
    return denied(blockedMessage(server.precondition), "conflict");
  }
  return { allowed: true };
}

function blockedMessage(precondition: string): string {
  switch (precondition) {
    case "suspended":
      return "The service is suspended.";
    case "no_active_deploy":
      return "There is no active deploy.";
    case "no_eligible_rollback_target":
      return "There is no eligible rollback target.";
    case "billing_blocked":
      return "Billing enforcement blocks this action.";
    default:
      return "The deploy action is currently unavailable.";
  }
}

function unavailable(message: string): DeployActionEligibility {
  return {
    allowed: false,
    error: {
      code: "unavailable",
      message,
      delivery: "not_sent",
      refreshRequired: true,
      retry: "after_refresh",
    },
  };
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
