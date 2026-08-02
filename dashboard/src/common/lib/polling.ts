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
 * Skip a poll tick while the tab is hidden — a backgrounded dashboard
 * shouldn't keep hitting bex-api every interval. Polling resumes on the next
 * tick after the tab becomes visible again.
 */
export function skipPollWhenHidden(): boolean {
  return typeof document !== "undefined" && document.hidden;
}
