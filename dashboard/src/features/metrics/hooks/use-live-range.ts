import { useEffect, useState } from "react";
import {
  resolveRange,
  type CustomRange,
  type RangeWindow,
} from "@/features/metrics/lib/range";

/**
 * Slides a range window's start/end forward with wall-clock time, so a live
 * dashboard's window keeps moving instead of freezing at first render (Apollo's
 * pollInterval alone re-fetches the *same* variables — it doesn't recompute
 * them). Ticks at the window's own resolution — a 12h/300s-bucket preset can't
 * show anything new more often than that, so there's no point refetching faster.
 * Takes only the two fields it reads, so a fixed non-preset window (the Scaling
 * page's 48h Recent Metrics) needs no fake preset id.
 *
 * A custom absolute range (w5/m56) is fixed, not relative: its start/end pass
 * straight through and the timer never arms, so the window stays put.
 */
export function useLiveRange(range: RangeWindow | CustomRange) {
  const custom = "startTime" in range;
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    if (custom) return; // absolute window — nothing to slide
    const id = setInterval(
      () => setNow(new Date()),
      range.resolutionSeconds * 1000,
    );
    return () => clearInterval(id);
  }, [custom, range.resolutionSeconds]);

  if (custom) {
    return {
      startTime: range.startTime,
      endTime: range.endTime,
      resolutionSeconds: range.resolutionSeconds,
    };
  }
  return resolveRange(range, now);
}
