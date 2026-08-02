import {
  isActiveCronRun,
  isTerminalCronRun,
  type CronRunSummary,
} from "./cron-history";

export type CronAction = "run" | "cancel";

export type CronActionRequest = {
  requestId: string;
  action: CronAction;
  serviceId: string;
  serviceSuspended: boolean;
  before: readonly CronRunSummary[];
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

/**
 * Suppresses replay by confirmation identity and single-flights two matching
 * confirmations. A transport ambiguity is never retried by this controller.
 */
export class CronActionController {
  private readonly requests = new Map<string, RequestRecord>();
  private readonly inFlight = new Map<string, Promise<CronActionResult>>();
  private readonly aborts = new Map<string, AbortController>();
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

    const ineligible = eligibilityError(request);
    if (ineligible) {
      const promise = Promise.resolve(rejected(request.action, ineligible));
      this.requests.set(request.requestId, { fingerprint, promise });
      return promise;
    }

    const active = this.inFlight.get(fingerprint);
    if (active) {
      const promise = active.then((result) => ({
        ...result,
        deduplicated: true,
      }));
      this.requests.set(request.requestId, { fingerprint, promise });
      return promise;
    }

    const abort = new AbortController();
    this.aborts.set(fingerprint, abort);
    const timeout = setTimeout(
      () => abort.abort(),
      Math.max(1, this.timeoutMs),
    );
    let promise: Promise<CronActionResult>;
    promise = this.perform(request, abort.signal).finally(() => {
      clearTimeout(timeout);
      if (this.inFlight.get(fingerprint) === promise) {
        this.inFlight.delete(fingerprint);
        this.aborts.delete(fingerprint);
      }
    });
    this.inFlight.set(fingerprint, promise);
    this.requests.set(request.requestId, { fingerprint, promise });
    return promise;
  }

  clear(): void {
    for (const abort of this.aborts.values()) abort.abort();
    this.requests.clear();
    this.inFlight.clear();
    this.aborts.clear();
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
      if (isDeterministicRejection(error)) {
        const refreshRequired = conflictLike(error);
        if (refreshRequired)
          this.requireAuthoritativeRefresh(request.serviceId);
        const normalizedError = conflictLike(error)
          ? Object.assign(
              new Error(errorFacts(error).text || "cron conflict"),
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

function eligibilityError(request: CronActionRequest): string | null {
  if (request.serviceSuspended && request.action === "run") {
    return "cron service is suspended";
  }
  if (request.action === "run") return null;
  if (!request.target || !isActiveCronRun(request.target.status)) {
    return "cron run is already terminal";
  }
  return null;
}

function actionFingerprint(request: CronActionRequest): string {
  return request.action === "run"
    ? `run:${request.serviceId}`
    : `cancel:${request.serviceId}:${request.target?.id ?? ""}`;
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

function errorFacts(error: unknown): { statuses: Set<number>; text: string } {
  const statuses = new Set<number>();
  const text: string[] = [];
  const seen = new Set<unknown>();
  const visit = (value: unknown) => {
    if (!value || typeof value !== "object" || seen.has(value)) return;
    seen.add(value);
    const record = value as Record<string, unknown>;
    if (typeof record.message === "string") {
      text.push(record.message.toLowerCase());
    }
    for (const candidate of [record.statusCode, record.status, record.code]) {
      if (typeof candidate === "number") statuses.add(candidate);
      if (typeof candidate === "string") {
        if (/^\d{3}$/.test(candidate)) statuses.add(Number(candidate));
        else text.push(candidate.toLowerCase());
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
  return { statuses, text: text.join(" ") };
}

function conflictLike(error: unknown): boolean {
  const { statuses, text } = errorFacts(error);
  return (
    statuses.has(404) ||
    statuses.has(409) ||
    /not found|conflict|terminal|workspace billing enforcement is active/.test(
      text,
    )
  );
}

function isDeterministicRejection(error: unknown): boolean {
  const { statuses, text } = errorFacts(error);
  return (
    statuses.has(400) ||
    statuses.has(401) ||
    statuses.has(403) ||
    statuses.has(404) ||
    statuses.has(409) ||
    /bad request|unauthorized|not authorized|forbidden|not found|conflict|terminal|suspended|workspace billing enforcement is active/.test(
      text,
    )
  );
}
