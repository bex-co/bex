import type { AppStateStatus } from "react-native";
import type { MobileNetworkState } from "@/common/apollo/network-state";
import {
  backoffDelayMs,
  DEFAULT_RECOVERY_BACKOFF,
  isRecoveryCanceled,
  recoverySleep,
  type BackoffPolicy,
  type RandomSource,
  type RecoverySleep,
} from "./backoff";

export type RecoveryReason =
  "poll" | "stream" | "manual" | "connectivity" | "foreground";

export interface RecoveryEnvironment {
  connectivity: MobileNetworkState;
  appState: AppStateStatus;
}

export type RecoveryPhase =
  | "idle"
  | "waiting"
  | "running"
  | "refreshing-auth"
  | "backoff"
  | "failed"
  | "disposed";

export interface RecoverySnapshot {
  phase: RecoveryPhase;
  reason: RecoveryReason | null;
  attempt: number;
  retryInMs: number | null;
  lastSucceededAt: number | null;
  error: unknown;
}

export type RecoveryResult =
  | { status: "succeeded"; attempts: number }
  | { status: "failed"; attempts: number; error: unknown }
  | { status: "deferred"; attempts: number }
  | { status: "canceled"; attempts: number };

export interface RecoveryAttemptContext {
  signal: AbortSignal;
  reason: RecoveryReason;
  attempt: number;
}

export interface RecoveryCoordinatorOptions {
  attempt: (context: RecoveryAttemptContext) => Promise<void>;
  initialEnvironment: RecoveryEnvironment;
  refreshAuth?: (signal: AbortSignal) => Promise<void>;
  isAuthError?: (error: unknown) => boolean;
  isRetryable?: (error: unknown) => boolean;
  maxAttempts?: number;
  backoff?: BackoffPolicy;
  random?: RandomSource;
  sleep?: RecoverySleep;
  now?: () => number;
  recoverOnForeground?: boolean;
  recoverOnReconnect?: boolean;
}

type Listener = (snapshot: RecoverySnapshot) => void;

export function recoveryAvailable(environment: RecoveryEnvironment): boolean {
  return (
    environment.connectivity === "online" && environment.appState === "active"
  );
}

/** Returns only edge-triggered reasons, so duplicate native events cannot storm. */
export function recoveryReasonForTransition(
  previous: RecoveryEnvironment,
  next: RecoveryEnvironment,
): RecoveryReason | null {
  if (!recoveryAvailable(next)) return null;
  if (previous.appState !== "active" && next.appState === "active") {
    return "foreground";
  }
  if (previous.connectivity !== "online" && next.connectivity === "online") {
    return "connectivity";
  }
  return null;
}

const initialSnapshot: RecoverySnapshot = {
  phase: "idle",
  reason: null,
  attempt: 0,
  retryInMs: null,
  lastSucceededAt: null,
  error: null,
};

/**
 * A transport-neutral single-flight recovery engine. Polling consumers provide
 * a refetch attempt; SSE consumers provide an attempt that resolves once a new
 * stream opens. App/background and connectivity transitions share one gate,
 * so simultaneous native events create one recovery, never one per listener.
 */
export class RecoveryCoordinator {
  private readonly options: Required<
    Pick<
      RecoveryCoordinatorOptions,
      | "maxAttempts"
      | "backoff"
      | "random"
      | "sleep"
      | "now"
      | "recoverOnForeground"
      | "recoverOnReconnect"
    >
  > &
    RecoveryCoordinatorOptions;
  private environment: RecoveryEnvironment;
  private snapshot: RecoverySnapshot = initialSnapshot;
  private listeners = new Set<Listener>();
  private active: Promise<RecoveryResult> | null = null;
  private activeAbort: AbortController | null = null;
  private activeReason: RecoveryReason | null = null;
  private pendingReason: RecoveryReason | null = null;
  private cancelVersion = 0;
  private disposed = false;

  constructor(options: RecoveryCoordinatorOptions) {
    this.options = {
      ...options,
      maxAttempts: Math.max(1, Math.floor(options.maxAttempts ?? 5)),
      backoff: options.backoff ?? DEFAULT_RECOVERY_BACKOFF,
      random: options.random ?? Math.random,
      sleep: options.sleep ?? recoverySleep,
      now: options.now ?? Date.now,
      recoverOnForeground: options.recoverOnForeground ?? true,
      recoverOnReconnect: options.recoverOnReconnect ?? true,
    };
    this.environment = options.initialEnvironment;
  }

  getSnapshot(): RecoverySnapshot {
    return this.snapshot;
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  setEnvironment(next: RecoveryEnvironment): void {
    if (this.disposed) return;
    const previous = this.environment;
    this.environment = next;
    const transition = recoveryReasonForTransition(previous, next);

    if (!recoveryAvailable(next)) {
      if (this.active) {
        this.pendingReason = this.activeReason ?? this.pendingReason;
        this.activeAbort?.abort();
      }
      if (this.active || this.pendingReason) {
        this.publish({
          ...this.snapshot,
          phase: "waiting",
          retryInMs: null,
        });
      }
      return;
    }

    const enabledTransition =
      transition === "foreground"
        ? this.options.recoverOnForeground
        : transition === "connectivity"
          ? this.options.recoverOnReconnect
          : false;
    if (transition && enabledTransition) this.pendingReason = transition;
    if (this.pendingReason) void this.request(this.pendingReason);
  }

  request(reason: RecoveryReason): Promise<RecoveryResult> {
    if (this.disposed) {
      return Promise.resolve({ status: "canceled", attempts: 0 });
    }
    // A healthy single flight satisfies concurrent poll/SSE/manual callers.
    // Only an already-aborted flight carries work into the next availability edge.
    if (this.active && !this.activeAbort?.signal.aborted) return this.active;

    this.pendingReason = reason;
    if (!recoveryAvailable(this.environment)) {
      this.publish({
        ...this.snapshot,
        phase: "waiting",
        reason,
        retryInMs: null,
      });
      return Promise.resolve({ status: "deferred", attempts: 0 });
    }
    if (this.active) return this.active;

    this.pendingReason = null;
    this.activeReason = reason;
    const version = this.cancelVersion;
    const abort = new AbortController();
    this.activeAbort = abort;
    const run = this.run(reason, abort, version);
    this.active = run;
    void run.finally(() => {
      if (this.active !== run) return;
      this.active = null;
      this.activeAbort = null;
      this.activeReason = null;
      const pending = this.pendingReason;
      if (pending && !this.disposed && recoveryAvailable(this.environment)) {
        this.pendingReason = null;
        void this.request(pending);
      }
    });
    return run;
  }

  manualRetry(): Promise<RecoveryResult> {
    return this.request("manual");
  }

  cancel(): void {
    if (this.disposed) return;
    this.cancelVersion += 1;
    this.pendingReason = null;
    this.activeAbort?.abort();
    this.publish({
      ...initialSnapshot,
      lastSucceededAt: this.snapshot.lastSucceededAt,
    });
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    this.cancelVersion += 1;
    this.pendingReason = null;
    this.activeAbort?.abort();
    this.publish({ ...this.snapshot, phase: "disposed", retryInMs: null });
    this.listeners.clear();
  }

  private async run(
    reason: RecoveryReason,
    abort: AbortController,
    version: number,
  ): Promise<RecoveryResult> {
    let attempts = 0;
    let authRefreshed = false;
    while (attempts < this.options.maxAttempts) {
      attempts += 1;
      this.publish({
        ...this.snapshot,
        phase: "running",
        reason,
        attempt: attempts,
        retryInMs: null,
        error: null,
      });
      try {
        await this.options.attempt({
          signal: abort.signal,
          reason,
          attempt: attempts,
        });
        if (abort.signal.aborted || version !== this.cancelVersion) {
          return this.canceledOrDeferred(attempts);
        }
        this.publish({
          ...initialSnapshot,
          lastSucceededAt: this.options.now(),
        });
        return { status: "succeeded", attempts };
      } catch (caught) {
        if (
          abort.signal.aborted ||
          version !== this.cancelVersion ||
          isRecoveryCanceled(caught)
        ) {
          return this.canceledOrDeferred(attempts);
        }
        let error = caught;
        if (
          !authRefreshed &&
          this.options.refreshAuth &&
          this.options.isAuthError?.(error)
        ) {
          // One recovery run gets one serialized credential refresh. If refresh
          // itself fails, later transport retries must not hammer the issuer.
          authRefreshed = true;
          this.publish({
            ...this.snapshot,
            phase: "refreshing-auth",
            error,
            retryInMs: null,
          });
          try {
            await this.options.refreshAuth(abort.signal);
            continue;
          } catch (refreshError) {
            error = refreshError;
          }
        }

        const retryable = this.options.isRetryable?.(error) ?? true;
        if (!retryable || attempts >= this.options.maxAttempts) {
          this.publish({
            ...this.snapshot,
            phase: "failed",
            error,
            retryInMs: null,
          });
          return { status: "failed", attempts, error };
        }
        const delay = backoffDelayMs(
          attempts - 1,
          this.options.backoff,
          this.options.random,
        );
        this.publish({
          ...this.snapshot,
          phase: "backoff",
          error,
          retryInMs: delay,
        });
        try {
          await this.options.sleep(delay, abort.signal);
        } catch (sleepError) {
          if (isRecoveryCanceled(sleepError) || abort.signal.aborted) {
            return this.canceledOrDeferred(attempts);
          }
          this.publish({
            ...this.snapshot,
            phase: "failed",
            error: sleepError,
            retryInMs: null,
          });
          return { status: "failed", attempts, error: sleepError };
        }
      }
    }
    return { status: "canceled", attempts };
  }

  private canceledOrDeferred(attempts: number): RecoveryResult {
    if (this.pendingReason != null || !recoveryAvailable(this.environment)) {
      this.publish({
        ...this.snapshot,
        phase: "waiting",
        retryInMs: null,
      });
      return { status: "deferred", attempts };
    }
    if (!this.disposed) {
      this.publish({
        ...initialSnapshot,
        lastSucceededAt: this.snapshot.lastSucceededAt,
      });
    }
    return { status: "canceled", attempts };
  }

  private publish(next: RecoverySnapshot): void {
    this.snapshot = next;
    for (const listener of this.listeners) listener(next);
  }
}
