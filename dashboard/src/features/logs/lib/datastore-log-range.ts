import {
  RANGE_PRESETS,
  isCustomRange,
  resolveRange,
  type RangeSelection,
} from "@/features/metrics/lib/range";

// Shared range handling for the datastore Logs viewers (Postgres + Key Value,
// w5/030). They adopt the same m56 range ladder the service Logs/Metrics tabs
// use (via the shared RangeSelect), so their range control is a RangeSelection
// resolved to a [startTime, endTime] window — not a private 1h/6h/24h enum.

/** The datastore Logs default window — 1h, the service Logs tab's own default. */
export const DEFAULT_DATASTORE_LOG_RANGE: RangeSelection =
  RANGE_PRESETS.find((preset) => preset.id === "1h") ?? RANGE_PRESETS[0];

/** Resolve a RangeSelection to the absolute window the logs query needs: an
 *  explicit custom range, or a relative preset resolved against now. */
export function rangeWindow(range: RangeSelection): {
  startTime: string;
  endTime: string;
} {
  if (isCustomRange(range)) {
    return { startTime: range.startTime, endTime: range.endTime };
  }
  const { startTime, endTime } = resolveRange(range, new Date());
  return { startTime, endTime };
}
