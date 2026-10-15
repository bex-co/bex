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
 *
 * Selecting a relative window samples the clock again during that same render
 * (w4/m203), so the first query after the selection ends at the selection time
 * rather than at the mount or the last tick. Leaving for a custom range and
 * coming back to the same preset is a new selection too.
 *
 * A changed `resetKey` is a new selection too (w4/306), so a caller whose query
 * changes under the same preset reads up to that change, not the last tick. A
 * custom range ignores it: its bounds never read the clock.
 */
export function useLiveRange(
  range: RangeWindow | CustomRange,
  resetKey?: string,
) {
  const custom = "startTime" in range;
  const selection = custom
    ? "custom"
    : `${range.spanSeconds}:${range.resolutionSeconds}:${resetKey ?? ""}`;
  const [clock, setClock] = useState(() => ({ selection, now: new Date() }));
  let now = clock.now;
  if (clock.selection !== selection) {
    now = new Date();
    setClock({ selection, now });
  }

  useEffect(() => {
    if (custom) return; // absolute window — nothing to slide
    const id = setInterval(
      () => setClock((previous) => ({ ...previous, now: new Date() })),
      range.resolutionSeconds * 1000,
    );
    return () => clearInterval(id);
    // `selection` restarts the cadence on a span-only change too: a tick left
    // pending would refetch again right after the fresh sample.
  }, [custom, selection, range.resolutionSeconds]);

  if (custom) {
    return {
      startTime: range.startTime,
      endTime: range.endTime,
      resolutionSeconds: range.resolutionSeconds,
    };
  }
  return resolveRange(range, now);
}
