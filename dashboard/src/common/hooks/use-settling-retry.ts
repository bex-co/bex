import { useCallback, useEffect, useRef, useState } from "react";
import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";

/** How often a deletion refused with BILLING_SETTLING is retried. */
export const SETTLING_RETRY_INTERVAL_MS = 15_000;
/** How long after the first attempt a BILLING_SETTLING deletion keeps
 *  retrying. Stripe's meter summaries took 55–62 s to reflect exported usage
 *  in a test-mode probe (2026-11-12), and each attempt waits up to 20 s. */
export const SETTLING_RETRY_WINDOW_MS = 180_000;

/** True when bex-api refused a deletion because Stripe is still aggregating
 *  the final usage it just exported (BILLING_SETTLING, retryable). */
export function isBillingSettling(err: unknown): boolean {
  return hasGraphQLErrorCode(err, "BILLING_SETTLING");
}

export interface SettlingRetry {
  /** Runs attempt, and while it rejects with BILLING_SETTLING, runs it again
   *  every SETTLING_RETRY_INTERVAL_MS for up to SETTLING_RETRY_WINDOW_MS.
   *  Resolves with the first success; rejects with the last error otherwise
   *  (still BILLING_SETTLING when the window ran out). */
  run: <T>(attempt: () => Promise<T>) => Promise<T>;
  /** True while waiting to retry a BILLING_SETTLING refusal. */
  settling: boolean;
}

/**
 * Settle before delete (ADR091 D10.4) answers BILLING_SETTLING while Stripe
 * aggregates the workspace's final usage, which can outlast one request. The
 * workspace and account deletion flows retry through this hook and show
 * "Finalizing final usage…" meanwhile. Unmounting stops the retries.
 */
export function useSettlingRetry(): SettlingRetry {
  const [settling, setSettling] = useState(false);
  const mounted = useRef(true);
  const cancel = useRef<(() => void) | null>(null);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      cancel.current?.();
    };
  }, []);

  const run = useCallback(async <T>(attempt: () => Promise<T>) => {
    const started = Date.now();
    try {
      for (;;) {
        try {
          return await attempt();
        } catch (err) {
          const elapsed = Date.now() - started;
          if (
            !isBillingSettling(err) ||
            !mounted.current ||
            elapsed + SETTLING_RETRY_INTERVAL_MS > SETTLING_RETRY_WINDOW_MS
          ) {
            throw err;
          }
          setSettling(true);
          await new Promise<void>((resolve, reject) => {
            const timer = setTimeout(() => {
              cancel.current = null;
              resolve();
            }, SETTLING_RETRY_INTERVAL_MS);
            cancel.current = () => {
              clearTimeout(timer);
              reject(err);
            };
          });
        }
      }
    } finally {
      if (mounted.current) setSettling(false);
    }
  }, []);

  return { run, settling };
}
