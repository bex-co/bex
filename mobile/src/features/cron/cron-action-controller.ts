import type { ResourceActionDecision } from "../capabilities/resource-actions";
import { isTerminalCronRun, type CronRunSummary } from "./cron-history";

export type CronAction = "run" | "cancel";

/**
 * The server's per-service decision for one cron verb (ADR087 serverActions
 * projection, normalized by toResourceSnapshot), captured at confirmation
 * time and bound into the request fingerprint: a changed outcome or
 * precondition cannot silently reuse an earlier confirmation. Null (no row
 * for this exact workspace+service+action) never authorizes a send.
 */
export type CronServerGate = Pick<
  ResourceActionDecision,
  "outcome" | "precondition"
> | null;

export type CronActionRequest = {
  requestId: string;
  action: CronAction;
  serviceId: string;
  /** Server eligibility for this verb, read at confirmation time. */
  server: CronServerGate;
  target?: CronRunSummary;
};

export type CronActionResult =
  | {
      outcome: "accepted";
      action: CronAction;
      runId: string;
      status: string;
      deduplicated: boolean;
    }
  | {
      outcome: "unknown" | "rejected";
      action: CronAction;
      error: unknown;
      refreshRequired: boolean;
      deduplicated: boolean;
    };

export type CronMutationResult = {
  run?: { id?: string | null; status?: string | null } | null;
};

export type CronActionTransport = {
  run: (serviceId: string, signal: AbortSignal) => Promise<CronMutationResult>;
  cancel: (
    serviceId: string,
    runId: string,
    signal: AbortSignal,
  ) => Promise<CronMutationResult>;
};

type RequestRecord = {
  fingerprint: string;
  promise: Promise<CronActionResult>;
};

type ActiveRequest = {
  promise: Promise<CronActionResult>;
  abort: AbortController;
};

/**
 * Suppresses replay by confirmation identity and single-flights two matching
 * confirmations. A transport ambiguity is never retried by this controller.
 */
export class CronActionController {
  private readonly requests = new Map<string, RequestRecord>();
  private readonly active = new Map<string, ActiveRequest>();
  private readonly refreshBlockedServices = new Set<string>();

  constructor(
    private readonly transport: CronActionTransport,
    private readonly timeoutMs = 15_000,
  ) {}

  execute(request: CronActionRequest): Promise<CronActionResult> {
    const fingerprint = actionFingerprint(request);
    const remembered = this.requests.get(request.requestId);
    if (remembered) {
      if (remembered.fingerprint === fingerprint) return remembered.promise;
      return Promise.resolve(rejected(request.action, "confirmation changed"));
    }
    if (this.refreshBlockedServices.has(request.serviceId)) {
      const promise = Promise.resolve(
        rejected(
          request.action,
          "authoritative cron history refresh required",
          true,
        ),
      );
      this.requests.set(request.requestId, { fingerprint, promise });
      return promise;
    }

    const gate = serverGateError(request);
    if (gate) {
      const promise = Promise.resolve(
        rejected(request.action, gate.message, gate.refreshRequired),
      );
      this.requests.set(request.requestId, { fingerprint, promise });
      return promise;
    }

    const active = this.active.get(fingerprint);
    if (active) {
      const promise = active.promise.then((result) => ({
        ...result,
        deduplicated: true,
      }));
      this.requests.set(request.requestId, { fingerprint, promise });
      return promise;
    }

    const abort = new AbortController();
    const timeout = setTimeout(
      () => abort.abort(),
      Math.max(1, this.timeoutMs),
    );
    let promise: Promise<CronActionResult>;
    promise = this.perform(request, abort.signal).finally(() => {
      clearTimeout(timeout);
      if (this.active.get(fingerprint)?.promise === promise) {
        this.active.delete(fingerprint);
      }
    });
    this.active.set(fingerprint, { promise, abort });
    this.requests.set(request.requestId, { fingerprint, promise });
    return promise;
  }

  clear(): void {
    for (const request of this.active.values()) request.abort.abort();
    this.requests.clear();
    this.active.clear();
    this.refreshBlockedServices.clear();
  }

  requireAuthoritativeRefresh(serviceId: string): void {
    this.refreshBlockedServices.add(serviceId);
  }

  markAuthoritativelyRefreshed(serviceId: string): void {
    this.refreshBlockedServices.delete(serviceId);
  }

  private async perform(
    request: CronActionRequest,
    signal: AbortSignal,
  ): Promise<CronActionResult> {
    try {
      const response = await abortable(
        request.action === "run"
          ? this.transport.run(request.serviceId, signal)
          : this.transport.cancel(
              request.serviceId,
              request.target!.id,
              signal,
            ),
        signal,
      );
      if (signal.aborted) {
        throw Object.assign(new Error("cron action timed out"), {
          name: "TimeoutError",
        });
      }
      const runId = response.run?.id?.trim();
      if (!runId) {
        this.requireAuthoritativeRefresh(request.serviceId);
        return {
          outcome: "unknown",
          action: request.action,
          error: new Error("cron action returned no run identifier"),
          refreshRequired: true,
          deduplicated: false,
        };
      }
      return {
        outcome: "accepted",
        action: request.action,
        runId,
        status: response.run?.status ?? "unknown",
        deduplicated: false,
      };
    } catch (error) {
      const facts = errorFacts(error);
      if (isDeterministicRejection(facts)) {
        const refreshRequired = conflictLike(facts);
        if (refreshRequired)
          this.requireAuthoritativeRefresh(request.serviceId);
        const normalizedError = refreshRequired
          ? Object.assign(
              new Error("cron action was rejected by current server state"),
              {
                statusCode: 409,
                cause: error,
              },
            )
          : error;
        return {
          outcome: "rejected",
          action: request.action,
          error: normalizedError,
          refreshRequired,
          deduplicated: false,
        };
      }
      this.requireAuthoritativeRefresh(request.serviceId);
      return {
        outcome: "unknown",
        action: request.action,
        error,
        refreshRequired: true,
        deduplicated: false,
      };
    }
  }
}

function abortable<T>(promise: Promise<T>, signal: AbortSignal): Promise<T> {
  if (signal.aborted) {
    return Promise.reject(
      Object.assign(new Error("cron action timed out"), {
        name: "TimeoutError",
      }),
    );
  }
  return new Promise<T>((resolve, reject) => {
    const abort = () => {
      signal.removeEventListener("abort", abort);
      reject(
        Object.assign(new Error("cron action timed out"), {
          name: "TimeoutError",
        }),
      );
    };
    signal.addEventListener("abort", abort, { once: true });
    promise.then(
      (value) => {
        signal.removeEventListener("abort", abort);
        resolve(value);
      },
      (error) => {
        signal.removeEventListener("abort", abort);
        reject(error);
      },
    );
  });
}

export function cronActionObserved(
  request: CronActionRequest,
  result: CronActionResult,
  after: readonly CronRunSummary[],
): boolean {
  if (result.outcome === "accepted") {
    const observed = after.find((run) => run.id === result.runId);
    return request.action === "run"
      ? Boolean(observed)
      : Boolean(observed && isTerminalCronRun(observed.status));
  }
  if (result.outcome !== "unknown") return false;
  if (request.action === "cancel") {
    const observed = after.find((run) => run.id === request.target?.id);
    return Boolean(observed && isTerminalCronRun(observed.status));
  }
  // A scheduled run can legitimately appear during this refresh window. With
  // no id from the lost mutation response, a new row cannot prove that this
  // particular manual run request committed.
  return false;
}

export async function awaitCronActionObservation(options: {
  request: CronActionRequest;
  result: CronActionResult;
  refresh: () => Promise<CronRunSummary[]>;
  wait?: () => Promise<void>;
  maxPolls?: number;
}): Promise<{ observed: boolean; runs: CronRunSummary[] }> {
  const wait =
    options.wait ??
    (() => new Promise<void>((resolve) => setTimeout(resolve, 1_500)));
  const maxPolls = Math.max(1, options.maxPolls ?? 6);
  let runs: CronRunSummary[] = [];
  for (let poll = 0; poll < maxPolls; poll += 1) {
    runs = await options.refresh();
    if (cronActionObserved(options.request, options.result, runs)) {
      return { observed: true, runs };
    }
    if (poll < maxPolls - 1) await wait();
  }
  return { observed: false, runs };
}

// The server's per-service decision gates the send: suspension, billing,
// and active-run preconditions live in the projection (and are rechecked by
// the verb at dispatch), never in parallel client status sets. History still
// names the concrete cancel target row — a cancel without one is malformed
// and never sent. A missing or unanswerable decision fails closed.
function serverGateError(request: CronActionRequest): {
  message: string;
  refreshRequired: boolean;
} | null {
  if (
    request.action === "cancel" &&
    (!request.target || !request.target.id.trim())
  ) {
    return { message: "cron run target is missing", refreshRequired: false };
  }
  const server = request.server;
  if (!server) {
    return {
      message: "cron action eligibility is currently unavailable",
      refreshRequired: true,
    };
  }
  if (server.outcome === "denied") {
    return { message: "cron action is not permitted", refreshRequired: false };
  }
  if (server.outcome !== "allowed") {
    return {
      message: "cron action eligibility is currently unavailable",
      refreshRequired: true,
    };
  }
  if (server.precondition !== "") {
    return {
      message: blockedMessage(server.precondition),
      refreshRequired: true,
    };
  }
  return null;
}

function blockedMessage(precondition: string): string {
  switch (precondition) {
    case "suspended":
      return "cron service is suspended";
    case "no_active_run":
      return "cron run is already terminal";
    case "billing_blocked":
      return "billing enforcement blocks this action";
    default:
      return "cron action is currently unavailable";
  }
}

function actionFingerprint(request: CronActionRequest): string {
  // The server gate binds the exact eligibility confirmed: a changed outcome
  // or precondition (or target) under a reused confirmation id is a different
  // action and must not replay the earlier confirmation.
  const gate = request.server
    ? `${request.server.outcome}:${request.server.precondition}`
    : "none";
  return request.action === "run"
    ? `run:${request.serviceId}:${gate}`
    : `cancel:${request.serviceId}:${request.target?.id ?? ""}:${gate}`;
}

function rejected(
  action: CronAction,
  message: string,
  refreshRequired = false,
): CronActionResult {
  return {
    outcome: "rejected",
    action,
    error: Object.assign(new Error(message), { statusCode: 409 }),
    refreshRequired,
    deduplicated: false,
  };
}

type CronErrorFacts = {
  statuses: Set<number>;
  codes: Set<string>;
};

function errorFacts(error: unknown): CronErrorFacts {
  const statuses = new Set<number>();
  const codes = new Set<string>();
  const seen = new Set<unknown>();
  const visit = (value: unknown) => {
    if (!value || typeof value !== "object" || seen.has(value)) return;
    seen.add(value);
    const record = value as Record<string, unknown>;
    for (const candidate of [record.statusCode, record.status, record.code]) {
      if (typeof candidate === "number") statuses.add(candidate);
      if (typeof candidate === "string") {
        if (/^\d{3}$/.test(candidate)) statuses.add(Number(candidate));
        else codes.add(candidate.toUpperCase());
      }
    }
    for (const nested of [
      record.cause,
      record.networkError,
      record.extensions,
    ]) {
      visit(nested);
    }
    for (const key of ["errors", "graphQLErrors"]) {
      if (Array.isArray(record[key])) {
        for (const nested of record[key]) visit(nested);
      }
    }
  };
  visit(error);
  return { statuses, codes };
}

function conflictLike({ codes }: CronErrorFacts): boolean {
  return (
    codes.has("CRON_SUSPENDED") ||
    codes.has("CRON_RUN_NOT_FOUND") ||
    codes.has("CRON_RUN_TERMINAL") ||
    codes.has("BILLING_ENFORCED")
  );
}

function isDeterministicRejection({
  statuses,
  codes,
}: CronErrorFacts): boolean {
  return (
    statuses.has(400) ||
    statuses.has(401) ||
    statuses.has(402) ||
    statuses.has(403) ||
    codes.has("BAD_USER_INPUT") ||
    codes.has("UNAUTHENTICATED") ||
    codes.has("FORBIDDEN") ||
    codes.has("CRON_SUSPENDED") ||
    codes.has("CRON_RUN_NOT_FOUND") ||
    codes.has("CRON_RUN_TERMINAL") ||
    codes.has("BILLING_ENFORCED") ||
    codes.has("PAYMENT_REQUIRED")
  );
}
