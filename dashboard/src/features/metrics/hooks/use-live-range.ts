import { useEffect, useState } from "react";
import { resolveRange, type RangePreset } from "@/features/metrics/lib/range";

const TICK_MS = 15_000;

/**
 * Slides a range preset's start/end forward with wall-clock time, so a live
 * dashboard's window keeps moving instead of freezing at first render (Apollo's
 * pollInterval alone re-fetches the *same* variables — it doesn't recompute
 * them).
 */
export function useLiveRange(preset: RangePreset) {
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), TICK_MS);
    return () => clearInterval(id);
  }, []);

  return resolveRange(preset, now);
}
