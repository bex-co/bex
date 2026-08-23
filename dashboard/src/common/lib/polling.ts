import { useEffect } from "react";

/**
 * Baseline cadence for keeping resource state fresh across the dashboard.
 * Resource lists and detail pages poll at this interval so state changed
 * elsewhere (the operator, a teammate, the CLI) shows up without a reload.
 * Hooks that track a fast transition (a provisioning database, a running
 * deploy) still poll faster while converging and fall back to this baseline
 * once settled.
 */
export const RESOURCE_POLL_INTERVAL_MS = 30_000;

/**
 * Fast cadence for a resource that is still converging (a provisioning
 * database, an upgrading key-value store): poll at this interval until the
 * status settles, then fall back to `RESOURCE_POLL_INTERVAL_MS`.
 */
export const CONVERGING_POLL_INTERVAL_MS = 3_000;

/**
 * Refetch a resource after a lifecycle verb, entering the fast converging
 * cadence first. A successful lifecycle mutation updates desired state before
 * the operator necessarily exposes its first converging status, so polling
 * begins immediately — an eager refetch that still says "available" cannot
 * hide the subsequent Upgrading/Provisioning transition.
 */
export function eagerRefetch(
  startPolling: (intervalMs: number) => void,
  refetch: () => Promise<unknown>,
): void {
  startPolling(CONVERGING_POLL_INTERVAL_MS);
  void refetch();
}

/**
 * Poll a resource at the converging cadence while it is still moving, and at
 * the baseline once it settles — the shape every live-resource hook in the
 * dashboard wants, so it lives here once rather than being re-derived per
 * feature. Pass Apollo's `startPolling`/`stopPolling` straight through; both
 * are stable across renders, so the timer is only ever reset when `converging`
 * actually flips.
 *
 * `enabled: false` registers no timer at all — for a secondary consumer reading
 * the same document as a designated polling owner, where a second timer would
 * drift into its own round trips instead of deduplicating.
 */
export function useConvergingPoll(
  startPolling: (intervalMs: number) => void,
  stopPolling: () => void,
  converging: boolean,
  enabled = true,
): void {
  useEffect(() => {
    if (!enabled) return;
    startPolling(
      converging ? CONVERGING_POLL_INTERVAL_MS : RESOURCE_POLL_INTERVAL_MS,
    );
    return () => stopPolling();
  }, [enabled, converging, startPolling, stopPolling]);
}

/**
 * Skip a poll tick while the tab is hidden — a backgrounded dashboard
 * shouldn't keep hitting bex-api every interval. Polling resumes on the next
 * tick after the tab becomes visible again.
 */
export function skipPollWhenHidden(): boolean {
  return typeof document !== "undefined" && document.hidden;
}
