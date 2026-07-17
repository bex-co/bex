import { useEffect, useState } from "react";
import { resolveRange, type RangeWindow } from "@/features/metrics/lib/range";

/**
 * Slides a range window's start/end forward with wall-clock time, so a live
 * dashboard's window keeps moving instead of freezing at first render (Apollo's
 * pollInterval alone re-fetches the *same* variables — it doesn't recompute
 * them). Ticks at the window's own resolution — a 12h/300s-bucket preset can't
 * show anything new more often than that, so there's no point refetching faster.
 * Takes only the two fields it reads, so a fixed non-preset window (the Scaling
 * page's 48h Recent Metrics) needs no fake preset id.
 */
export function useLiveRange(preset: RangeWindow) {
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
