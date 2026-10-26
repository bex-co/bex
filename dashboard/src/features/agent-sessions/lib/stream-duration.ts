import { useEffect, useRef, useState } from "react";

// Activity/reasoning labels prefer persisted ISO-8601 UTC `at` values so live
// delivery, terminal replay, and reconnect of the same transcript render the
// same elapsed value. Parts without a valid `at` (legacy and mixed histories)
// keep the pre-m87 client arrival fallback: `Date.now()` on first mount vs last
// growth, which collapses to ~0 on a one-frame replay and shows the
// duration-less "Worked" / "Thought" label.

/** Display cap so a pathological timestamp cannot produce an unbounded label. */
export const MAX_ELAPSED_MS = 24 * 60 * 60 * 1000;

export interface StreamDuration {
  ms: number;
  /** True when elapsed came from client arrival rather than persisted `at`. */
  approximate: boolean;
}

/**
 * Elapsed milliseconds a streamed group has been growing. `sourceTimesMs` are
 * parsed source timestamps from the group's parts; `itemCount` / `settled`
 * drive the live-clock and legacy arrival-time paths when a closed source
 * interval is not yet available. Closed source intervals are render-pure.
 */
export function useStreamDuration(
  itemCount: number,
  settled: boolean,
  sourceTimesMs: readonly number[] = [],
): StreamDuration {
  const startRef = useRef<number | null>(null);
  const frozenRef = useRef(false);
  // Closed source interval needs no clock. A single distinct start while still
  // growing samples Date.now() in an effect (cannot be render-pure).
  const sourcedClosed = elapsedMsFromSource(sourceTimesMs, true, 0);
  const needsLiveClock =
    !settled && sourcedClosed !== undefined && sourcedClosed === 0;
  // A primitive (not the array) keys the effect: callers rebuild
  // `sourceTimesMs` on every streamed chunk.
  const liveStartMs = needsLiveClock
    ? Math.min(...sourceTimesMs.filter((n) => Number.isFinite(n)))
    : undefined;
  const [duration, setDuration] = useState<StreamDuration>({
    ms: 0,
    approximate: true,
  });
  const publishedRef = useRef(duration);

  useEffect(() => {
    if (sourcedClosed !== undefined && !needsLiveClock) return;
    const now = Date.now();
    let next: StreamDuration;
    if (liveStartMs !== undefined) {
      next = {
        ms: wholeSeconds(clampElapsed(now - liveStartMs)),
        approximate: false,
      };
    } else {
      if (startRef.current === null) startRef.current = now;
      if (frozenRef.current) return;
      next = {
        ms: wholeSeconds(clampElapsed(now - startRef.current)),
        approximate: true,
      };
      if (settled) frozenRef.current = true;
    }
    // Only a changed whole-second label re-renders. Each streamed chunk is a
    // synchronous `useChat` store update; a state update queued from this
    // passive effect on every chunk stays pending behind the burst and trips
    // React's nested-update limit (#185, w10/m16 t001) on a long replay.
    const prev = publishedRef.current;
    if (prev.ms === next.ms && prev.approximate === next.approximate) return;
    publishedRef.current = next;
    setDuration(next);
  }, [itemCount, settled, sourcedClosed, needsLiveClock, liveStartMs]);

  if (sourcedClosed !== undefined && !needsLiveClock) {
    return { ms: sourcedClosed, approximate: false };
  }
  return duration;
}

function clampElapsed(ms: number): number {
  return Number.isFinite(ms) ? Math.min(Math.max(0, ms), MAX_ELAPSED_MS) : 0;
}

/** Labels show whole seconds, so the live clock only changes once a second. */
function wholeSeconds(ms: number): number {
  return Math.floor(ms / 1000) * 1000;
}

/**
 * Closed interval over valid source times. Missing timing returns `undefined`
 * (caller falls back). Out-of-order values use min/max so elapsed is never
 * negative; invalid/non-finite values are ignored; the result is clamped.
 */
export function elapsedMsFromSource(
  timestamps: readonly number[],
  settled: boolean,
  nowMs: number,
): number | undefined {
  const valid = timestamps.filter((n) => Number.isFinite(n));
  if (valid.length === 0) return undefined;
  const start = Math.min(...valid);
  const end = Math.max(...valid);
  const span = !settled && start === end ? nowMs - start : end - start;
  if (!Number.isFinite(span) || span < 0) return 0;
  return Math.min(span, MAX_ELAPSED_MS);
}

/** Exact elapsed label from persisted source timestamps ("12s", "3m 4s"). */
export function formatElapsedDuration(ms: number): string {
  const total = Math.round(Math.max(0, ms) / 1000);
  if (total < 60) return `${total}s`;
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
}

/** Approximate elapsed label ("~12s", "~3m 4s") — arrival-time fallback. */
export function formatApproxDuration(ms: number): string {
  return `~${formatElapsedDuration(ms)}`;
}

export function formatStreamDuration(duration: StreamDuration): string {
  return duration.approximate
    ? formatApproxDuration(duration.ms)
    : formatElapsedDuration(duration.ms);
}
