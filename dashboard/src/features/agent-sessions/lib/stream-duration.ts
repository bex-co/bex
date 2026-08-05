import { useEffect, useRef, useState } from "react";

// DURATIONS (w3/m44 t004 decision). The m43 v1 transcript / `UIMessageChunk`
// parts carry NO per-part timestamps — verified against the driver
// (`lego/agent-image/driver/src/{session,stream-hub}.ts`), which forwards ACP
// parts verbatim with no timing field. So we derive elapsed on the CLIENT from
// stream-arrival timing: record `Date.now()` when a group first mounts vs. when
// its last part arrives, then freeze once the group settles. This is approximate
// by design and only meaningful for a LIVE session — a replayed historical
// transcript paints all its parts in one frame, so the derived elapsed is ~0 and
// the caller falls back to a duration-less label ("Worked" / "Thought"). No
// backend change this pass; emitting real per-part timestamps from the driver is
// filed as a follow-up if the derived timing proves too coarse.

/**
 * Elapsed milliseconds a streamed group has been growing, derived from arrival
 * timing. `itemCount` is the group's current content size (steps, or reasoning
 * text length) — a change re-samples the clock; `settled` freezes the value once
 * the group is done. `Date.now()` is read only inside the effect (never during
 * render) so the hook stays render-pure.
 */
export function useStreamDuration(itemCount: number, settled: boolean): number {
  const startRef = useRef<number | null>(null);
  const frozenRef = useRef(false);
  const [elapsed, setElapsed] = useState(0);
  useEffect(() => {
    const now = Date.now();
    if (startRef.current === null) startRef.current = now;
    if (frozenRef.current) return;
    setElapsed(now - startRef.current);
    if (settled) frozenRef.current = true;
  }, [itemCount, settled]);
  return elapsed;
}

/** Approximate elapsed label ("~12s", "~3m 4s") — always prefixed "~" (derived). */
export function formatApproxDuration(ms: number): string {
  const total = Math.round(ms / 1000);
  if (total < 60) return `~${total}s`;
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return secs > 0 ? `~${mins}m ${secs}s` : `~${mins}m`;
}
