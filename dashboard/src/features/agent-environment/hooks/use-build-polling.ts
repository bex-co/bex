import { useEffect, useRef } from "react";
import { BUILD_POLL_INTERVAL_MS } from "@/features/agent-environment/lib/repo-environment-builds";

/**
 * Re-reads a query every BUILD_POLL_INTERVAL_MS while a build it shows is
 * queued or building, and registers no timer otherwise (ADR111 D10). Pass
 * Apollo's `startPolling`/`stopPolling`, which are stable across renders.
 *
 * `onSettled` runs once when `active` turns false after being true: another
 * query can write the build's new status into the shared cache first, which
 * would stop this poll before it read the fields only it selects (a build's
 * log), so the caller reads them one last time.
 */
export function useBuildPolling(
  active: boolean,
  startPolling: (intervalMs: number) => void,
  stopPolling: () => void,
  onSettled?: () => void,
): void {
  const wasActive = useRef(false);
  const settled = useRef(onSettled);
  useEffect(() => {
    settled.current = onSettled;
  });
  useEffect(() => {
    if (active) {
      wasActive.current = true;
      startPolling(BUILD_POLL_INTERVAL_MS);
      return () => stopPolling();
    }
    if (wasActive.current) {
      wasActive.current = false;
      settled.current?.();
    }
  }, [active, startPolling, stopPolling]);
}
