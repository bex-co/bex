import { safeActionOperationKey, type SafeActionIntent } from "./model";

export type SafeActionAuditStatus =
  "recorded" | "pending" | "unavailable" | "not-reported";

export interface SafeActionServerResult<Data> {
  data: Data;
  feedback?: "accepted-unverified";
  audit?: {
    status: Exclude<SafeActionAuditStatus, "not-reported">;
    eventId?: string;
  };
}

export interface SafeActionExecutionContext {
  signal: AbortSignal;
  retryIdentity: string;
  actionId: SafeActionIntent["actionId"];
  target: SafeActionIntent["target"];
}

export type SafeActionOperation<Data> = (
  context: SafeActionExecutionContext,
) => Promise<SafeActionServerResult<Data>>;

export type SafeActionFeedback =
  | "success"
  | "accepted-unverified"
  | "authorization-denied"
  | "conflict"
  | "timeout-unknown"
  | "audit-pending"
  | "audit-unavailable"
  | "failed"
  | "canceled";

export type SafeActionOutcome<Data> =
  | {
      status: "succeeded";
      feedback: "success";
      data: Data;
      retryIdentity: string;
      auditStatus: "recorded" | "not-reported";
      auditEventId?: string;
      canRetry: false;
    }
  | {
      status: "partial";
      feedback: "accepted-unverified";
      data: Data;
      retryIdentity: string;
      auditStatus: "not-reported";
      canRetry: false;
    }
  | {
      status: "partial";
      feedback: "audit-pending" | "audit-unavailable";
      data: Data;
      retryIdentity: string;
      auditStatus: "pending" | "unavailable";
      auditEventId?: string;
      canRetry: false;
    }
  | {
      status: "blocked";
      feedback: "authorization-denied" | "conflict";
      retryIdentity: string;
      error: unknown;
      canRetry: false;
    }
  | {
      status: "unknown";
      feedback: "timeout-unknown" | "audit-unavailable";
      retryIdentity: string;
      error: unknown;
      canRetry: false;
    }
  | {
      status: "failed";
      feedback: "failed";
      retryIdentity: string;
      error: unknown;
      canRetry: false;
    }
  | {
      status: "canceled";
      feedback: "canceled";
      retryIdentity: string;
      canRetry: false;
    };

interface ErrorFacts {
  names: Set<string>;
  codes: Set<string>;
  statuses: Set<number>;
  messages: Set<string>;
}

function collectErrorFacts(error: unknown): ErrorFacts {
  const facts: ErrorFacts = {
    names: new Set<string>(),
    codes: new Set<string>(),
    statuses: new Set<number>(),
    messages: new Set<string>(),
  };
  const seen = new Set<unknown>();
  const visit = (value: unknown) => {
    if (!value || typeof value !== "object" || seen.has(value)) return;
    seen.add(value);
    const record = value as Record<string, unknown>;
    if (typeof record.name === "string") facts.names.add(record.name);
    if (typeof record.message === "string") {
      facts.messages.add(record.message.toLowerCase());
    }
    if (typeof record.code === "string") {
      facts.codes.add(record.code.toUpperCase());
    }
    for (const key of ["status", "statusCode"]) {
      if (typeof record[key] === "number") facts.statuses.add(record[key]);
    }
    if (record.extensions) visit(record.extensions);
    if (record.cause) visit(record.cause);
    for (const key of ["errors", "graphQLErrors"]) {
      if (Array.isArray(record[key])) {
        for (const nested of record[key]) visit(nested);
      }
    }
  };
  visit(error);
  return facts;
}

function messageMatches(messages: Set<string>, pattern: RegExp): boolean {
  for (const message of messages) if (pattern.test(message)) return true;
  return false;
}

function codeMatches(codes: Set<string>, pattern: RegExp): boolean {
  for (const code of codes) if (pattern.test(code)) return true;
  return false;
}

export function classifySafeActionFailure(
  error: unknown,
): Exclude<
  SafeActionFeedback,
  "success" | "accepted-unverified" | "audit-pending" | "canceled"
> {
  const facts = collectErrorFacts(error);
  if (
    facts.statuses.has(401) ||
    facts.statuses.has(403) ||
    codeMatches(facts.codes, /(?:UNAUTHORIZED|FORBIDDEN|AUTHORIZATION)/) ||
    messageMatches(facts.messages, /(?:unauthorized|forbidden|not authorized)/)
  ) {
    return "authorization-denied";
  }
  if (
    facts.statuses.has(409) ||
    codeMatches(
      facts.codes,
      /(?:CONFLICT|STALE|NOT_(?:CANCELABLE|RESUMABLE))/,
    ) ||
    messageMatches(
      facts.messages,
      /(?:conflict|already terminal|state changed)/,
    )
  ) {
    return "conflict";
  }
  if (
    facts.statuses.has(408) ||
    facts.statuses.has(504) ||
    facts.names.has("AbortError") ||
    facts.names.has("TimeoutError") ||
    codeMatches(facts.codes, /TIMEOUT/) ||
    messageMatches(facts.messages, /(?:timeout|timed out|deadline exceeded)/)
  ) {
    return "timeout-unknown";
  }
  if (
    codeMatches(facts.codes, /AUDIT/) ||
    messageMatches(facts.messages, /audit.*unavailable/)
  ) {
    return "audit-unavailable";
  }
  return "failed";
}

interface ActiveExecution<Data> {
  retryIdentity: string;
  abort: AbortController;
  promise: Promise<SafeActionOutcome<Data>>;
}

/** Single-flight executor shared by every safe-action component on a screen. */
export class SafeActionExecutor {
  private active = new Map<string, ActiveExecution<unknown>>();
  private listeners = new Set<() => void>();

  isPending(intent?: SafeActionIntent | null): boolean {
    return intent ? this.active.has(safeActionOperationKey(intent)) : false;
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  execute<Data>(
    intent: SafeActionIntent,
    operation: SafeActionOperation<Data>,
  ): Promise<SafeActionOutcome<Data>> {
    if (!intent.confirmed) {
      return Promise.reject(
        new Error("safe action must be confirmed before execution"),
      );
    }
    const key = safeActionOperationKey(intent);
    const existing = this.active.get(key) as ActiveExecution<Data> | undefined;
    if (existing) return existing.promise;

    const abort = new AbortController();
    const promise = this.run(intent, operation, abort);
    this.active.set(key, {
      retryIdentity: intent.retryIdentity,
      abort,
      promise: promise as Promise<SafeActionOutcome<unknown>>,
    });
    this.notify();
    void promise.finally(() => {
      const current = this.active.get(key);
      if (current?.promise !== promise) return;
      this.active.delete(key);
      this.notify();
    });
    return promise;
  }

  cancel(intent: SafeActionIntent): void {
    this.active.get(safeActionOperationKey(intent))?.abort.abort();
  }

  cancelAll(): void {
    for (const execution of this.active.values()) execution.abort.abort();
  }

  private async run<Data>(
    intent: SafeActionIntent,
    operation: SafeActionOperation<Data>,
    abort: AbortController,
  ): Promise<SafeActionOutcome<Data>> {
    try {
      const result = await operation({
        signal: abort.signal,
        retryIdentity: intent.retryIdentity,
        actionId: intent.actionId,
        target: intent.target,
      });
      if (abort.signal.aborted) {
        return {
          status: "canceled",
          feedback: "canceled",
          retryIdentity: intent.retryIdentity,
          canRetry: false,
        };
      }
      if (result.feedback === "accepted-unverified") {
        return {
          status: "partial",
          feedback: "accepted-unverified",
          data: result.data,
          retryIdentity: intent.retryIdentity,
          auditStatus: "not-reported",
          canRetry: false,
        };
      }
      const auditStatus = result.audit?.status ?? "not-reported";
      if (auditStatus === "pending" || auditStatus === "unavailable") {
        return {
          status: "partial",
          feedback:
            auditStatus === "pending" ? "audit-pending" : "audit-unavailable",
          data: result.data,
          retryIdentity: intent.retryIdentity,
          auditStatus,
          auditEventId: result.audit?.eventId,
          canRetry: false,
        };
      }
      return {
        status: "succeeded",
        feedback: "success",
        data: result.data,
        retryIdentity: intent.retryIdentity,
        auditStatus,
        auditEventId: result.audit?.eventId,
        canRetry: false,
      };
    } catch (error) {
      if (abort.signal.aborted) {
        return {
          status: "canceled",
          feedback: "canceled",
          retryIdentity: intent.retryIdentity,
          canRetry: false,
        };
      }
      const feedback = classifySafeActionFailure(error);
      if (feedback === "authorization-denied" || feedback === "conflict") {
        return {
          status: "blocked",
          feedback,
          retryIdentity: intent.retryIdentity,
          error,
          canRetry: false,
        };
      }
      if (feedback === "timeout-unknown" || feedback === "audit-unavailable") {
        return {
          status: "unknown",
          feedback,
          retryIdentity: intent.retryIdentity,
          error,
          // The current GraphQL mutations do not accept an idempotency key.
          // A timeout may have committed, so reconciliation must precede any
          // new confirmation instead of blindly replaying the mutation.
          canRetry: false,
        };
      }
      return {
        status: "failed",
        feedback: "failed",
        retryIdentity: intent.retryIdentity,
        error,
        canRetry: false,
      };
    }
  }

  private notify(): void {
    for (const listener of this.listeners) listener();
  }
}
