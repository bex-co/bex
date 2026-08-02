export type DeployAction = "trigger" | "cancel" | "rollback";

export type DeployTarget = {
  id: string;
  status: string;
};

type BaseDeployActionRequest = {
  /** Stable for one confirmation. Replaying it must not submit again. */
  requestId: string;
  serviceId: string;
  serviceSuspended: boolean;
};

export type TriggerDeployRequest = BaseDeployActionRequest & {
  action: "trigger";
};

export type CancelDeployRequest = BaseDeployActionRequest & {
  action: "cancel";
  /** A deploy already present in the caller's refreshed history. */
  target: DeployTarget;
};

export type RollbackDeployRequest = BaseDeployActionRequest & {
  action: "rollback";
  /** A known successful deploy selected from history, never an arbitrary image. */
  target: DeployTarget;
};

export type DeployActionRequest =
  TriggerDeployRequest | CancelDeployRequest | RollbackDeployRequest;

export type DeployMutationResult = {
  deploy: {
    id: string | null;
    status: string | null;
  } | null;
};

/**
 * The intentionally narrow mobile mutation boundary. There is nowhere to put
 * commitId, deployMode, imageUrl, a command, or any build configuration.
 */
export interface DeployActionTransport {
  trigger(serviceId: string): Promise<DeployMutationResult>;
  cancel(serviceId: string, deployId: string): Promise<DeployMutationResult>;
  rollback(serviceId: string, deployId: string): Promise<DeployMutationResult>;
}

export type DeployActionErrorCode =
  | "invalid_request"
  | "forbidden"
  | "conflict"
  | "not_found"
  | "unavailable"
  | "timeout"
  | "network"
  | "invalid_response";

export type DeployActionError = {
  code: DeployActionErrorCode;
  message: string;
  /** Unknown means the server may already have committed the action. */
  delivery: "not_sent" | "possibly_committed" | "rejected_by_server";
  refreshRequired: boolean;
  retry: "none" | "safe" | "after_refresh";
};

export type DeployActionResult =
  | {
      requestId: string;
      action: DeployAction;
      outcome: "accepted";
      deployId: string;
      status: string;
      convergence: "awaiting_refresh" | "converged";
      deduplicated: boolean;
    }
  | {
      requestId: string;
      action: DeployAction;
      outcome: "rejected";
      error: DeployActionError;
      deduplicated: boolean;
    }
  | {
      requestId: string;
      action: DeployAction;
      outcome: "unknown";
      error: DeployActionError;
      deduplicated: boolean;
    };

export type DeployActionPhase =
  "submitting" | "awaiting_refresh" | "converged" | "rejected" | "unknown";

export type DeployActionState = {
  request: DeployActionRequest;
  phase: DeployActionPhase;
  result?: DeployActionResult;
};
