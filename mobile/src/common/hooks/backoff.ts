export interface BackoffPolicy {
  initialDelayMs: number;
  maxDelayMs: number;
  multiplier: number;
  /** Symmetric jitter as a fraction of the exponential delay (0..1). */
  jitterRatio: number;
}

export const DEFAULT_RECOVERY_BACKOFF: BackoffPolicy = {
  initialDelayMs: 500,
  maxDelayMs: 30_000,
  multiplier: 2,
  jitterRatio: 0.2,
};

export type RandomSource = () => number;

function finiteNonNegative(value: number, fallback: number): number {
  return Number.isFinite(value) && value >= 0 ? value : fallback;
}

/**
 * Computes retry delay `0` as the first backoff interval. The result is always
 * bounded by maxDelayMs, including after jitter. Randomness is injectable so
 * reconnect behavior can be tested without timing flakes.
 */
export function backoffDelayMs(
  retry: number,
  policy: BackoffPolicy = DEFAULT_RECOVERY_BACKOFF,
  random: RandomSource = Math.random,
): number {
  const initial = finiteNonNegative(policy.initialDelayMs, 0);
  const max = Math.max(initial, finiteNonNegative(policy.maxDelayMs, initial));
  const multiplier = Math.max(1, finiteNonNegative(policy.multiplier, 1));
  const jitterRatio = Math.min(1, finiteNonNegative(policy.jitterRatio, 0));
  const retryIndex = Math.max(0, Math.floor(finiteNonNegative(retry, 0)));
  const exponential = Math.min(max, initial * multiplier ** retryIndex);
  const sample = Math.min(1, Math.max(0, finiteNonNegative(random(), 0.5)));
  const jitter = exponential * jitterRatio * (sample * 2 - 1);
  return Math.round(Math.min(max, Math.max(0, exponential + jitter)));
}

export class RecoveryCanceledError extends Error {
  constructor() {
    super("recovery canceled");
    this.name = "RecoveryCanceledError";
  }
}

export function isRecoveryCanceled(error: unknown): boolean {
  return (
    error instanceof RecoveryCanceledError ||
    (error instanceof Error && error.name === "AbortError")
  );
}

export type RecoverySleep = (
  delayMs: number,
  signal: AbortSignal,
) => Promise<void>;

/** Abort-aware timer used by the production coordinator; tests inject sleep. */
export const recoverySleep: RecoverySleep = (delayMs, signal) =>
  new Promise<void>((resolve, reject) => {
    if (signal.aborted) {
      reject(new RecoveryCanceledError());
      return;
    }
    const timer = setTimeout(done, Math.max(0, delayMs));
    signal.addEventListener("abort", canceled, { once: true });

    function cleanup() {
      clearTimeout(timer);
      signal.removeEventListener("abort", canceled);
    }
    function done() {
      cleanup();
      resolve();
    }
    function canceled() {
      cleanup();
      reject(new RecoveryCanceledError());
    }
  });
