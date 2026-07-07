import { useEffect, useState } from "react";
import { resolveRange, type RangePreset } from "@/features/metrics/lib/range";

/**
 * Slides a range preset's start/end forward with wall-clock time, so a live
 * dashboard's window keeps moving instead of freezing at first render (Apollo's
 * pollInterval alone re-fetches the *same* variables — it doesn't recompute
 * them). Ticks at the preset's own resolution — a 12h/300s-bucket preset can't
 * show anything new more often than that, so there's no point refetching faster.
 */
export function useLiveRange(preset: RangePreset) {
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const id = setInterval(
      () => setNow(new Date()),
      preset.resolutionSeconds * 1000,
    );
    return () => clearInterval(id);
  }, [preset.resolutionSeconds]);

  return resolveRange(preset, now);
}
