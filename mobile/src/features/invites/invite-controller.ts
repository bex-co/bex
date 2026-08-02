import {
  isValidInviteSubject,
  parseInviteToken,
  type StoredInvite,
} from "./invite-token";
import type { InviteStore } from "./invite-storage";

export type AcceptedWorkspace = {
  id: string;
  name: string | null;
  role: string | null;
};

export type InviteTerminalFailure =
  | "invalid"
  | "expired"
  | "already-accepted"
  | "plan-limit"
  | "authorization"
  | "subject-changed"
  | "failed"
  | "storage";

export type InviteRetryableFailure = "transport" | "unavailable";

export type InviteFlowState =
  | { status: "loading" }
  | { status: "empty" }
  | { status: "ready" }
  | { status: "accepting" }
  | { status: "retryable"; failure: InviteRetryableFailure }
  | {
      status: "accepted";
      workspace: AcceptedWorkspace;
      refreshFailed: boolean;
    }
  | { status: "terminal"; failure: InviteTerminalFailure };

export type InviteAcceptanceClient = {
  accept(token: string, signal: AbortSignal): Promise<AcceptedWorkspace>;
};

export class InviteFlowController {
  private state: InviteFlowState = { status: "loading" };
  private pending: StoredInvite | null = null;
  private loadPromise?: Promise<void>;
  private acceptance?: Promise<void>;
  private abort?: AbortController;
  private generation = 0;

  constructor(
    private readonly store: InviteStore,
    private readonly client: InviteAcceptanceClient,
    private readonly refreshWorkspaces: () => Promise<void>,
    private readonly onState: (state: InviteFlowState) => void = () => {},
    private readonly timeoutMs = 15_000,
  ) {}

  getState(): InviteFlowState {
    return this.state;
  }

  bootstrap(subject: string | null): Promise<void> {
    return this.ensureLoaded().then(() => this.bindSubject(subject));
  }

  async capture(value: unknown, subject: string | null): Promise<boolean> {
    await this.ensureLoaded();
    const token = parseInviteToken(value);
    if (!token || !isValidInviteSubject(subject)) {
      await this.discard({ status: "terminal", failure: "invalid" });
      return false;
    }
    const invite: StoredInvite = { version: 1, token, subject };
    try {
      await this.store.save(invite);
      this.pending = invite;
      this.generation += 1;
      this.setState({ status: "ready" });
      return true;
    } catch {
      this.pending = null;
      this.setState({ status: "terminal", failure: "storage" });
      return false;
    }
  }

  accept(subject: string): Promise<void> {
    if (this.acceptance) return this.acceptance;
    const promise = this.performAccept(subject).finally(() => {
      if (this.acceptance === promise) this.acceptance = undefined;
    });
    this.acceptance = promise;
    return promise;
  }

  async clear(): Promise<void> {
    this.invalidatePending();
    this.setState({ status: "empty" });
    await this.store.clear();
  }

  private ensureLoaded(): Promise<void> {
    this.loadPromise ??= this.load().finally(() => {
      // Keep a fulfilled/rejected promise as the one bootstrap result. A
      // storage error must not cause background retry loops around a bearer.
    });
    return this.loadPromise;
  }

  private async load(): Promise<void> {
    try {
      this.pending = await this.store.load();
      this.setState(this.pending ? { status: "ready" } : { status: "empty" });
    } catch {
      this.pending = null;
      this.setState({ status: "terminal", failure: "storage" });
    }
  }

  private async bindSubject(subject: string | null): Promise<void> {
    if (!this.pending || subject === null) return;
    if (!isValidInviteSubject(subject)) {
      await this.discard({ status: "terminal", failure: "subject-changed" });
      return;
    }
    if (this.pending.subject && this.pending.subject !== subject) {
      await this.discard({ status: "terminal", failure: "subject-changed" });
      return;
    }
    if (this.pending.subject === null) {
      const bound = { ...this.pending, subject };
      try {
        await this.store.save(bound);
        this.pending = bound;
      } catch {
        await this.discard({ status: "terminal", failure: "storage" });
        return;
      }
    }
    if (
      this.state.status !== "accepting" &&
      this.state.status !== "retryable"
    ) {
      this.setState({ status: "ready" });
    }
  }

  private async performAccept(subject: string): Promise<void> {
    await this.ensureLoaded();
    await this.bindSubject(subject);
    const pending = this.pending;
    if (!pending || pending.subject !== subject) return;

    const generation = this.generation;
    const abort = new AbortController();
    this.abort = abort;
    const timer = setTimeout(() => abort.abort(), Math.max(1, this.timeoutMs));
    this.setState({ status: "accepting" });
    try {
      const workspace = await abortable(
        this.client.accept(pending.token, abort.signal),
        abort.signal,
      );
      if (!this.isCurrent(generation, abort)) return;
      await this.store.clear();
      this.pending = null;
      this.generation += 1;
      let refreshFailed = false;
      try {
        await this.refreshWorkspaces();
      } catch {
        refreshFailed = true;
      }
      this.setState({ status: "accepted", workspace, refreshFailed });
    } catch (error) {
      if (!this.isCurrent(generation, abort)) return;
      const failure = classifyInviteAcceptanceError(error);
      if (failure === "transport" || failure === "unavailable") {
        this.setState({ status: "retryable", failure });
      } else {
        await this.discard({ status: "terminal", failure });
      }
    } finally {
      clearTimeout(timer);
      if (this.abort === abort) this.abort = undefined;
    }
  }

  private isCurrent(generation: number, abort: AbortController): boolean {
    return this.generation === generation && this.abort === abort;
  }

  private async discard(state: InviteFlowState): Promise<void> {
    this.invalidatePending();
    try {
      await this.store.clear();
      this.setState(state);
    } catch {
      this.setState({ status: "terminal", failure: "storage" });
    }
  }

  private invalidatePending(): void {
    this.generation += 1;
    this.abort?.abort();
    this.abort = undefined;
    this.pending = null;
  }

  private setState(state: InviteFlowState): void {
    this.state = state;
    this.onState(state);
  }
}

export function classifyInviteAcceptanceError(
  error: unknown,
): InviteTerminalFailure | InviteRetryableFailure {
  const facts = errorFacts(error);
  if (facts.codes.has("INVITE_INVALID")) {
    return "invalid";
  }
  if (facts.codes.has("INVITE_EXPIRED")) return "expired";
  if (facts.codes.has("INVITE_ALREADY_ACCEPTED")) return "already-accepted";
  if (facts.codes.has("INVITE_PLAN_LIMIT")) return "plan-limit";
  if (
    facts.statuses.has(401) ||
    facts.statuses.has(403) ||
    facts.codes.has("UNAUTHENTICATED") ||
    facts.codes.has("FORBIDDEN")
  ) {
    return "authorization";
  }
  if (
    facts.names.has("AbortError") ||
    facts.names.has("TimeoutError") ||
    facts.statuses.has(408) ||
    facts.codes.has("TIMEOUT")
  ) {
    return "transport";
  }
  if (
    facts.names.has("TypeError") ||
    facts.statuses.has(404) ||
    facts.statuses.has(429) ||
    [...facts.statuses].some((status) => status >= 500) ||
    facts.codes.has("NETWORK_ERROR") ||
    facts.codes.has("SERVICE_UNAVAILABLE")
  ) {
    return "unavailable";
  }
  return "failed";
}

function abortable<T>(promise: Promise<T>, signal: AbortSignal): Promise<T> {
  if (signal.aborted) return Promise.reject(timeoutError());
  return new Promise<T>((resolve, reject) => {
    const abort = () => {
      signal.removeEventListener("abort", abort);
      reject(timeoutError());
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

function timeoutError(): Error {
  return Object.assign(new Error("invite acceptance outcome is unknown"), {
    name: "TimeoutError",
  });
}

function errorFacts(error: unknown): {
  codes: Set<string>;
  statuses: Set<number>;
  names: Set<string>;
} {
  const facts = {
    codes: new Set<string>(),
    statuses: new Set<number>(),
    names: new Set<string>(),
  };
  const seen = new Set<unknown>();
  const visit = (value: unknown) => {
    if (!value || typeof value !== "object" || seen.has(value)) return;
    seen.add(value);
    const record = value as Record<string, unknown>;
    if (typeof record.code === "string") {
      facts.codes.add(record.code.toUpperCase());
    }
    if (typeof record.name === "string") facts.names.add(record.name);
    for (const key of ["status", "statusCode"] as const) {
      const raw = record[key];
      if (typeof raw === "number") facts.statuses.add(raw);
      if (typeof raw === "string" && /^\d{3}$/.test(raw)) {
        facts.statuses.add(Number(raw));
      }
    }
    for (const key of ["extensions", "cause", "networkError"] as const) {
      visit(record[key]);
    }
    for (const key of ["errors", "graphQLErrors"] as const) {
      const nested = record[key];
      if (Array.isArray(nested)) nested.forEach(visit);
    }
  };
  visit(error);
  return facts;
}
