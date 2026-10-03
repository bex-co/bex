import { useMemo, useState, useSyncExternalStore } from "react";

// Cadences share a store so a table has one timer, not one timer per cell.
// Retaining the store while idle also makes StrictMode's unsubscribe/subscribe
// replay reuse the same clock. Only active stores own timers or listeners.
const clocks = new Map<number, ReturnType<typeof createClock>>();

function createClock(intervalMs: number) {
  let now = Date.now();
  let timer: ReturnType<typeof setInterval> | undefined;
  const listeners = new Set<() => void>();
  const refresh = () => {
    const next = Date.now();
    if (next === now) return;
    now = next;
    listeners.forEach((listener) => listener());
  };
  const onVisibility = () => {
    if (document.visibilityState === "visible") refresh();
  };
  return {
    getSnapshot: () => now,
    refreshIfIdle: () => {
      if (listeners.size === 0) refresh();
    },
    subscribe: (listener: () => void) => {
      listeners.add(listener);
      const age = Date.now() - now;
      if (listeners.size === 1 || age >= intervalMs || age < 0) refresh();
      if (listeners.size === 1) {
        timer = setInterval(refresh, intervalMs);
        document.addEventListener("visibilitychange", onVisibility);
        window.addEventListener("focus", refresh);
      }
      return () => {
        listeners.delete(listener);
        if (listeners.size === 0) {
          clearInterval(timer);
          timer = undefined;
          document.removeEventListener("visibilitychange", onVisibility);
          window.removeEventListener("focus", refresh);
        }
      };
    },
  };
}

/** Current epoch ms from one shared clock per cadence (one minute by default).
 * Visible-tab resume and focus refresh immediately after timer throttling.
 * Server/client text can cross a bucket boundary; callers retain their existing
 * suppressHydrationWarning on clock-derived text, as RelativeAge does. */
export function useNow(intervalMs = 60_000): number {
  const clock = useMemo(() => {
    let shared = clocks.get(intervalMs);
    if (!shared) {
      shared = createClock(intervalMs);
      clocks.set(intervalMs, shared);
    }
    shared.refreshIfIdle();
    return shared;
  }, [intervalMs]);
  // React can read the hydration snapshot repeatedly within one render.
  // Cache that pass's sample rather than returning a different millisecond.
  const [serverSnapshot] = useState(() => {
    const initial = Date.now();
    return () => initial;
  });
  return useSyncExternalStore(
    clock.subscribe,
    clock.getSnapshot,
    serverSnapshot,
  );
}
