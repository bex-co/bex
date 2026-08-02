import { deployActionEligibility } from "./deploy-action-eligibility";
import { mapDeployActionError } from "./deploy-action-errors";
import type {
  DeployActionRequest,
  DeployActionResult,
  DeployActionState,
  DeployActionTransport,
  DeployMutationResult,
} from "./deploy-action-types";

type Listener = (state: DeployActionState) => void;

type RequestRecord = {
  fingerprint: string;
  promise: Promise<DeployActionResult>;
  state: DeployActionState;
};

const terminalStatuses = new Set([
  "build_failed",
  "canceled",
  "deactivated",
  "live",
  "pre_deploy_failed",
  "update_failed",
]);

/**
 * One controller per authenticated workspace. It never automatically retries a
 * mutation: a lost response may already have committed on the server. Stable
 * confirmation ids and an action fingerprint suppress replay and double taps.
 */
export class DeployActionController {
  private records = new Map<string, RequestRecord>();
  private inFlight = new Map<string, Promise<DeployActionResult>>();
  private listeners = new Set<Listener>();

  constructor(
    private readonly transport: DeployActionTransport,
    private readonly maxRememberedRequests = 100,
  ) {}

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  state(requestId: string): DeployActionState | undefined {
    return this.records.get(requestId)?.state;
  }

  execute(request: DeployActionRequest): Promise<DeployActionResult> {
    const fingerprint = actionFingerprint(request);
    const remembered = this.records.get(request.requestId);
    if (remembered) {
      if (remembered.fingerprint === fingerprint) return remembered.promise;
      return Promise.resolve(
        this.rememberImmediateRejection(
          request,
          "A deploy confirmation identifier was reused for a different action.",
        ),
      );
    }

    const eligibility = deployActionEligibility(request);
    if (!eligibility.allowed) {
      const result: DeployActionResult = {
        requestId: request.requestId,
        action: request.action,
        outcome: "rejected",
        error: eligibility.error,
        deduplicated: false,
      };
      return this.remember(
        request,
        fingerprint,
        Promise.resolve(result),
        result,
      );
    }

    const matching = this.inFlight.get(fingerprint);
    if (matching) {
      const promise = matching.then((result) => ({
        ...result,
        requestId: request.requestId,
        deduplicated: true,
      }));
      return this.remember(
        request,
        fingerprint,
        promise,
        undefined,
        "submitting",
      );
    }

    let promise: Promise<DeployActionResult>;
    promise = this.perform(request).finally(() => {
      if (this.inFlight.get(fingerprint) === promise) {
        this.inFlight.delete(fingerprint);
      }
      this.prune();
    });
    this.inFlight.set(fingerprint, promise);
    return this.remember(
      request,
      fingerprint,
      promise,
      undefined,
      "submitting",
    );
  }

  /** Called after deploy-history refresh observes the accepted deploy. */
  markConverged(
    requestId: string,
    status: string,
  ): DeployActionState | undefined {
    const record = this.records.get(requestId);
    const result = record?.state.result;
    if (!record || !result || result.outcome !== "accepted")
      return record?.state;
    const nextResult: DeployActionResult = {
      ...result,
      status,
      convergence: terminalStatuses.has(status)
        ? "converged"
        : "awaiting_refresh",
    };
    record.state = {
      ...record.state,
      phase:
        nextResult.convergence === "converged"
          ? "converged"
          : "awaiting_refresh",
      result: nextResult,
    };
    this.emit(record.state);
    return record.state;
  }

  clear(): void {
    this.records.clear();
    this.inFlight.clear();
  }

  private async perform(
    request: DeployActionRequest,
  ): Promise<DeployActionResult> {
    let response: DeployMutationResult;
    try {
      if (request.action === "trigger") {
        response = await this.transport.trigger(request.serviceId);
      } else if (request.action === "cancel") {
        response = await this.transport.cancel(
          request.serviceId,
          request.target.id,
        );
      } else {
        response = await this.transport.rollback(
          request.serviceId,
          request.target.id,
        );
      }
    } catch (error) {
      const mapped = mapDeployActionError(error);
      return {
        requestId: request.requestId,
        action: request.action,
        outcome:
          mapped.delivery === "possibly_committed" ? "unknown" : "rejected",
        error: mapped,
        deduplicated: false,
      };
    }

    const id = response.deploy?.id;
    if (!id) {
      return {
        requestId: request.requestId,
        action: request.action,
        outcome: "unknown",
        error: {
          code: "invalid_response",
          message:
            "The server accepted the action but returned no deploy identifier.",
          delivery: "possibly_committed",
          refreshRequired: true,
          retry: "after_refresh",
        },
        deduplicated: false,
      };
    }
    const status = response.deploy?.status ?? "unknown";
    return {
      requestId: request.requestId,
      action: request.action,
      outcome: "accepted",
      deployId: id,
      status,
      convergence: terminalStatuses.has(status)
        ? "converged"
        : "awaiting_refresh",
      deduplicated: false,
    };
  }

  private remember(
    request: DeployActionRequest,
    fingerprint: string,
    promise: Promise<DeployActionResult>,
    immediate?: DeployActionResult,
    initialPhase: DeployActionState["phase"] = immediateResultPhase(immediate),
  ): Promise<DeployActionResult> {
    const record: RequestRecord = {
      fingerprint,
      promise,
      state: { request, phase: initialPhase, result: immediate },
    };
    this.records.set(request.requestId, record);
    this.emit(record.state);
    void promise.then((result) => {
      // An identity/workspace boundary may clear the controller while the
      // server mutation is still resolving. Never publish that stale result.
      if (this.records.get(request.requestId) !== record) return;
      record.state = {
        request,
        phase: resultPhase(result),
        result,
      };
      this.emit(record.state);
    });
    return promise;
  }

  private rememberImmediateRejection(
    request: DeployActionRequest,
    message: string,
  ): DeployActionResult {
    return {
      requestId: request.requestId,
      action: request.action,
      outcome: "rejected",
      error: {
        code: "invalid_request",
        message,
        delivery: "not_sent",
        refreshRequired: false,
        retry: "none",
      },
      deduplicated: false,
    };
  }

  private prune(): void {
    if (this.records.size <= this.maxRememberedRequests) return;
    for (const [requestId, record] of this.records) {
      if (record.state.phase !== "submitting") this.records.delete(requestId);
      if (this.records.size <= this.maxRememberedRequests) return;
    }
  }

  private emit(state: DeployActionState): void {
    for (const listener of this.listeners) listener(state);
  }
}

function actionFingerprint(request: DeployActionRequest): string {
  return request.action === "trigger"
    ? `${request.action}:${request.serviceId}`
    : `${request.action}:${request.serviceId}:${request.target.id}`;
}

function immediateResultPhase(
  result?: DeployActionResult,
): DeployActionState["phase"] {
  return result ? resultPhase(result) : "submitting";
}

function resultPhase(result: DeployActionResult): DeployActionState["phase"] {
  if (result.outcome === "unknown") return "unknown";
  if (result.outcome === "rejected") return "rejected";
  return result.convergence === "converged" ? "converged" : "awaiting_refresh";
}
