import { useState } from "react";
import {
  isCustomRange,
  type RangeSelection,
} from "@/features/metrics/lib/range";

/**
 * A custom log window's end and whether it had already ended when it was
 * applied (w21/016). Nothing can stream into a closed window, so its live tail
 * never opens; a window ending in the future keeps tailing, bounded by its end.
 * The clock is sampled when the range is applied, not on every render. Shared
 * by the service and datastore Logs tabs so the two cannot disagree (w4/625).
 */
export function useCustomWindow(range: RangeSelection): {
  customEndMs: number | null;
  windowClosed: boolean;
} {
  const customEndMs = isCustomRange(range) ? Date.parse(range.endTime) : null;
  const rangeKey = JSON.stringify(range);
  const [appliedAt, setAppliedAt] = useState(() => ({
    key: rangeKey,
    now: Date.now(),
  }));
  let appliedNow = appliedAt.now;
  if (appliedAt.key !== rangeKey) {
    appliedNow = new Date().getTime();
    setAppliedAt({ key: rangeKey, now: appliedNow });
  }
  return {
    customEndMs,
    windowClosed: customEndMs !== null && customEndMs <= appliedNow,
  };
}
