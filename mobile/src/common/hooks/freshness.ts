import { useEffect, useMemo, useState } from "react";

export type FreshnessStatus = "not-loaded" | "fresh" | "stale";

export interface FreshnessSnapshot {
  status: FreshnessStatus;
  /** Translation-friendly semantic label; no user-visible copy lives here. */
  label: "freshness.notLoaded" | "freshness.current" | "freshness.stale";
  ageMs: number | null;
  staleAt: number | null;
}

export function freshnessSnapshot(
  updatedAt: number | Date | null | undefined,
  staleAfterMs: number,
  now = Date.now(),
): FreshnessSnapshot {
  if (updatedAt == null) {
    return {
      status: "not-loaded",
      label: "freshness.notLoaded",
      ageMs: null,
      staleAt: null,
    };
  }
  const updated = updatedAt instanceof Date ? updatedAt.getTime() : updatedAt;
  if (!Number.isFinite(updated)) {
    return {
      status: "not-loaded",
      label: "freshness.notLoaded",
      ageMs: null,
      staleAt: null,
    };
  }
  const threshold = Math.max(0, staleAfterMs);
  const ageMs = Math.max(0, now - updated);
  const staleAt = updated + threshold;
  const stale = ageMs >= threshold;
  return {
    status: stale ? "stale" : "fresh",
    label: stale ? "freshness.stale" : "freshness.current",
    ageMs,
    staleAt,
  };
}

export interface UseFreshnessOptions {
  staleAfterMs: number;
  /** Caps age-label refresh work after the exact stale boundary is crossed. */
  tickMs?: number;
  now?: () => number;
}

/**
 * Tracks an observation from fresh to stale at the exact threshold, then keeps
 * ageMs reasonably current. Feature screens decide how to translate/render the
 * semantic label and may use the same primitive for polling or SSE snapshots.
 */
export function useFreshness(
  updatedAt: number | Date | null | undefined,
  options: UseFreshnessOptions,
): FreshnessSnapshot {
  const { staleAfterMs, tickMs = 30_000, now = Date.now } = options;
  const [clock, setClock] = useState(now);
  const updated = updatedAt instanceof Date ? updatedAt.getTime() : updatedAt;
  const snapshot = useMemo(
    () => freshnessSnapshot(updated, staleAfterMs, clock),
    [clock, staleAfterMs, updated],
  );

  useEffect(() => {
    const nextBoundary =
      snapshot.staleAt == null ? tickMs : Math.max(1, snapshot.staleAt - clock);
    const delay = Math.max(1, Math.min(tickMs, nextBoundary));
    const timer = setTimeout(() => setClock(now()), delay);
    return () => clearTimeout(timer);
  }, [clock, now, snapshot.staleAt, tickMs]);

  return snapshot;
}
